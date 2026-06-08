package exam

import (
	"context"
	"encoding/json"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lifei6671/papermind/server/internal/service/pagination"
	"github.com/lifei6671/papermind/server/internal/service/permission"
)

// ManagementDetailInput 是管理端考试详情的统一读取入口参数。
// 后续 detail、overview、candidate、results、logs 接口都应先通过它获得同一份权限裁剪结果。
type ManagementDetailInput struct {
	Permission permission.PermissionContext // 当前登录主体的权限上下文，必须由 API 层从 session 和空间成员关系派生。
	TenantID   uint64                       // 考试所属租户 ID。
	ExamID     uint64                       // 管理端要访问的考试 ID。
}

// ManagementPermissions 表示考试详情页各 tab 和操作按钮的后端权限结果。
// 前端只能用它控制显隐和禁用，最终写操作仍必须由对应 API 再做一次后端校验。
type ManagementPermissions struct {
	CanViewDetail       bool // 是否允许进入管理端考试详情页。
	CanViewOverview     bool // 是否允许查看考试概览。
	CanViewPaper        bool // 是否允许查看试卷预览。
	CanViewCandidates   bool // 是否允许查看考生管理列表。
	CanManageCandidates bool // 是否允许导入考生或重发邀请。
	CanViewResults      bool // 是否允许查看成绩管理。
	CanExportResults    bool // 是否允许导出成绩；教师首版不开放。
	CanPublishResults   bool // 是否允许发布成绩。
	CanUpdateSettings   bool // 是否允许修改考试设置。
	CanViewLogs         bool // 是否允许查看操作日志。
}

// ManagementDetail 是管理端考试详情的聚合权限上下文。
// 这里不提前装载每个 tab 的大列表数据，只保留后续查询必须复用的考试、投放目标和授权空间范围。
type ManagementDetail struct {
	Exam            Exam                  // 考试基础记录。
	Targets         []Target              // 原始投放目标，供基础信息和考生范围解释使用。
	TargetSpaceIDs  []uint64              // 投放目标展开后的当前有效空间范围。
	AllowedSpaceIDs []uint64              // 当前管理者在本场考试内实际可见的空间范围。
	Permissions     ManagementPermissions // 当前管理者可用的页面和操作权限。
}

// OverviewCandidateStats 是考试概览上方考生状态卡片的数据。
type OverviewCandidateStats struct {
	Planned    int // 当前授权范围内计划应考人数。
	Joined     int // 当前授权范围内已开始作答人数。
	Submitted  int // 当前授权范围内已交卷人数。
	InProgress int // 当前授权范围内正在作答人数。
}

// OverviewQuestionTypeStat 是考试概览题型分布表的一行。
type OverviewQuestionTypeStat struct {
	QuestionType  string // 题型编码。
	QuestionCount int    // 该题型题量。
	TotalScore    string // 该题型总分。
}

// OverviewActivity 是考试概览近期动态列表的一行。
type OverviewActivity struct {
	OperationType   string // 操作类型。
	OperationTitle  string // 操作标题。
	OperationDetail string // 操作摘要。
	CreatedAt       int64  // 操作时间，Unix 毫秒时间戳。
}

// ExamOverviewData 是考试概览 tab 的后端聚合数据。
type ExamOverviewData struct {
	Detail           ManagementDetail           // 复用详情权限上下文，API 层可继续使用其标题区和权限。
	CandidateStats   OverviewCandidateStats     // 考生进度统计。
	QuestionTypes    []OverviewQuestionTypeStat // 题型分布。
	RecentActivities []OverviewActivity         // 近期动态；没有日志时返回空数组。
}

// PaperPreviewOption 是管理端试卷预览里的选项。
// 注意这里不包含 is_correct，避免普通管理端预览泄露标准答案。
type PaperPreviewOption struct {
	ID      uint64 // 选项 ID。
	Key     string // 选项标签。
	Content string // 选项内容。
}

// PaperPreviewQuestion 是管理端试卷预览题目。
// CorrectText 和 CorrectOptionIDs 不会出现在该对象中，答案只保留在作答快照源内部使用。
type PaperPreviewQuestion struct {
	SectionID    uint64               // 大题 ID。
	SectionName  string               // 大题名称。
	QuestionID   uint64               // 原题 ID。
	QuestionType string               // 题型编码。
	Title        string               // 题干。
	Score        string               // 题目分值。
	BlankCount   int                  // 填空题空位数。
	SortOrder    int                  // 过滤后的题目序号，从 1 开始。
	Options      []PaperPreviewOption // 选项列表。
}

// PaperPreviewSection 是试卷结构侧栏的大题摘要。
type PaperPreviewSection struct {
	SectionID     uint64 // 大题 ID。
	SectionName   string // 大题名称。
	QuestionType  string // 题型编码。
	QuestionCount int    // 大题题量。
	TotalScore    string // 大题总分。
}

// PaperPreviewInput 是试卷预览接口的查询条件。
type PaperPreviewInput struct {
	ManagementDetailInput
	QuestionType string // 题型筛选；空字符串表示全部题型。
	Page         int    // 页码，从 1 开始。
	PageSize     int    // 每页数量。
}

// ExamPaperPreviewData 是试卷预览 tab 的后端数据。
type ExamPaperPreviewData struct {
	Detail   ManagementDetail       // 复用详情权限上下文。
	Sections []PaperPreviewSection  // 试卷结构。
	Items    []PaperPreviewQuestion // 当前页题目。
	Page     int                    // 当前页码。
	PageSize int                    // 每页数量。
	Total    int                    // 过滤后的题目总数。
}

// CandidateListInput 是考生管理 tab 的查询条件。
type CandidateListInput struct {
	ManagementDetailInput
	Keyword  string // 姓名、用户名或空间名搜索关键字。
	Status   string // 状态筛选；空字符串表示不过滤。
	Page     int    // 页码，从 1 开始。
	PageSize int    // 每页数量。
}

// CandidateListData 是考生管理 tab 的真实分页数据。
type CandidateListData struct {
	Detail   ManagementDetail // 复用详情权限上下文。
	Items    []ExamCandidate  // 当前页应考考生。
	Page     int              // 当前页码。
	PageSize int              // 每页数量。
	Total    int64            // 当前条件下的总数。
}

// ResultSummaryStats 是成绩管理统计卡片的数据。
type ResultSummaryStats struct {
	Submitted         int    // 已提交作答数量。
	AverageScore      string // 平均分。
	HighestScore      string // 最高分。
	PassRate          string // 及格率，带百分号。
	PendingSubjective int    // 仍有主观题待阅的作答数量。
}

// ResultScoreDistribution 表示一个分数段的人数。
type ResultScoreDistribution struct {
	Label string // 分数段标签，例如 80-89。
	Count int    // 该分数段人数。
}

// ResultQuestionTypeRate 表示题型平均得分率。
type ResultQuestionTypeRate struct {
	QuestionType      string // 题型编码。
	QuestionTypeLabel string // 题型中文名称。
	AverageRate       int    // 平均得分率百分比，不带百分号。
}

// ResultSummaryData 是成绩管理摘要接口的返回对象。
type ResultSummaryData struct {
	Detail            ManagementDetail          // 复用详情权限上下文。
	Stats             ResultSummaryStats        // 统计卡片。
	ScoreDistribution []ResultScoreDistribution // 分数段分布。
	QuestionTypeRates []ResultQuestionTypeRate  // 题型得分率。
}

// ResultListInput 是成绩管理列表查询条件。
type ResultListInput struct {
	ManagementDetailInput
	Keyword  string // 姓名、用户名或空间名搜索关键字。
	Status   string // 成绩状态筛选；空字符串表示不过滤。
	Page     int    // 页码，从 1 开始。
	PageSize int    // 每页数量。
}

// ExamResult 表示成绩管理列表的一行。
type ExamResult struct {
	Rank            uint64 // 基于授权范围内全量排序得到的排名。
	AttemptID       uint64 // 作答 ID。
	UserID          uint64 // 考生用户 ID。
	Username        string // 登录名或学号。
	RealName        string // 考生姓名。
	SpaceID         uint64 // 展示用空间 ID。
	SpaceName       string // 展示用空间名称。
	ObjectiveScore  string // 客观题得分。
	SubjectiveScore string // 主观题得分。
	TotalScore      string // 总分。
	Status          string // 成绩状态。
	SubmittedAt     int64  // 提交时间。
}

// ResultListData 是成绩管理列表分页数据。
type ResultListData struct {
	Detail   ManagementDetail // 复用详情权限上下文。
	Items    []ExamResult     // 当前页成绩。
	Page     int              // 当前页码。
	PageSize int              // 每页数量。
	Total    int64            // 当前条件下总数。
}

// AnswerSheetInput 是管理端答卷详情读取条件。
// ExamID 和 AttemptID 必须同时参与查询，避免仅凭 attempt_id 跨考试读取答卷。
type AnswerSheetInput struct {
	ManagementDetailInput
	AttemptID uint64 // 作答 ID。
}

// AnswerSheetAttempt 是答卷详情顶部的考生和作答摘要。
type AnswerSheetAttempt struct {
	AttemptID       uint64 // 作答 ID。
	ExamID          uint64 // 考试 ID。
	UserID          uint64 // 考生用户 ID。
	Username        string // 登录名或学号。
	RealName        string // 考生姓名。
	ObjectiveScore  string // 客观题得分。
	SubjectiveScore string // 主观题得分。
	TotalScore      string // 总分。
	SubmittedAt     int64  // 提交时间。
}

// AnswerSheetItem 是答卷详情中的单题记录。
// 快照字段来自作答时固化的数据，不能回查当前题库，避免考试后改题影响历史答卷。
type AnswerSheetItem struct {
	AttemptQuestionID     uint64 // 考生题目快照 ID。
	SectionID             uint64 // 大题 ID。
	QuestionID            uint64 // 题目 ID。
	SortOrder             int    // 作答题号。
	SectionSnapshot       string // 大题快照 JSON。
	QuestionSnapshot      string // 题目快照 JSON。
	QuestionType          string // 题型编码。
	QuestionTitle         string // 题干标题，便于前端列表展示。
	OptionSnapshot        string // 选项快照 JSON。
	CorrectAnswerSnapshot string // 标准答案快照 JSON，仅管理端答卷详情返回。
	Score                 string // 题目满分。
	AnswerContent         string // 学生答案。
	AnswerScore           string // 本题得分。
	GradingStatus         string // 阅卷状态。
	GraderComment         string // 阅卷评语。
	GradedBy              uint64 // 阅卷人用户 ID。
	GradedAt              *int64 // 阅卷时间。
}

// AnswerSheetData 是管理端答卷详情接口返回对象。
type AnswerSheetData struct {
	Detail  ManagementDetail   // 复用详情权限上下文。
	Attempt AnswerSheetAttempt // 作答摘要。
	Items   []AnswerSheetItem  // 按 sort_order 排序的题目和答案。
}

// ManagementSettingsInput 是考试设置页首版写接口的输入。
// 当前只允许修改成绩发布配置，发布范围、时长和公平性配置必须由 handler 拒绝或后续单独设计。
type ManagementSettingsInput struct {
	ManagementDetailInput
	PublishMode      string // 成绩发布模式。
	ScorePublishTime *int64 // 统一成绩发布时间；nil 表示不设置定时发布时间。
}

// ManagementSettingsData 是考试设置更新后的返回对象。
type ManagementSettingsData struct {
	Detail ManagementDetail // 复用详情权限上下文，便于前端继续使用最新按钮权限。
	Exam   Exam             // 更新后的考试基础记录。
}

// OperationLogListInput 是操作日志 tab 的分页读取输入。
// OperationType 为空表示读取全部类型，非空时由仓储层精确匹配操作类型。
type OperationLogListInput struct {
	ManagementDetailInput
	OperationType string // 操作类型筛选。
	Page          int    // 页码。
	PageSize      int    // 每页数量。
}

// OperationLogListData 是操作日志 tab 的分页返回数据。
type OperationLogListData struct {
	Detail   ManagementDetail // 复用详情权限上下文，便于前端控制日志 tab 可见性。
	Items    []OperationLog   // 当前页操作日志。
	Page     int              // 页码。
	PageSize int              // 每页数量。
	Total    int64            // 授权范围内符合条件的日志总数。
}

// ImportCandidatesInput 是考生导入写接口的输入。
// UserIDs 是要追加为用户直投目标的考生用户 ID，service 会先去重，repository 再按权限范围过滤有效学生。
type ImportCandidatesInput struct {
	ManagementDetailInput
	UserIDs []uint64 // 本次导入的考生用户 ID。
}

// ImportCandidatesResult 表示导入考生后的写入统计。
type ImportCandidatesResult struct {
	Detail        ManagementDetail // 复用详情权限上下文，便于 API 返回最新权限。
	ImportedCount int              // 本次真实新增的用户直投目标数量。
	SkippedCount  int              // 重复、已存在或无效考生数量。
}

// ResendInvitationsInput 是重发邀请码写接口的输入。
// UserIDs 只表示前端本次选中的目标用户，service 和 repository 都必须再次按应考名单与授权空间过滤。
type ResendInvitationsInput struct {
	ManagementDetailInput
	UserIDs []uint64 // 本次需要重发邀请码的考生用户 ID。
}

// ResendInvitationsResult 表示重发邀请码后的处理统计。
// 当前系统还没有短信或邮件通道，因此接口返回考试邀请码，并把“重发”记录为管理端审计操作。
type ResendInvitationsResult struct {
	Detail       ManagementDetail // 复用详情权限上下文，便于 API 返回最新权限。
	SentCount    int              // 本次通过应考名单与权限校验的目标人数。
	SkippedCount int              // 重复、无效、非应考或不在授权范围内的目标人数。
	InviteCode   string           // 考试邀请码，供前端提示或复制。
}

// AttemptOverviewStats 是仓储层返回的作答状态聚合结果。
type AttemptOverviewStats struct {
	Joined     int // 已开始作答人数。
	Submitted  int // 已交卷人数。
	InProgress int // 正在作答人数。
}

// ImportCandidateTargetsInput 是仓储层导入用户直投目标的事务输入。
type ImportCandidateTargetsInput struct {
	TenantID        uint64       // 所属租户 ID。
	ExamID          uint64       // 考试 ID。
	UserIDs         []uint64     // 已去重后的考生用户 ID。
	AllowedSpaceIDs []uint64     // 当前管理者在本场考试内可管理的空间范围。
	Log             OperationLog // 与导入目标同事务写入的操作日志。
}

// ImportCandidateTargetsResult 是仓储层导入后的结果摘要。
type ImportCandidateTargetsResult struct {
	ImportedTargets []Target // 本次真实新增的用户直投目标。
	ImportedCount   int      // 本次真实新增数量。
	SkippedCount    int      // 已存在、无效或不在授权空间内的用户数量。
}

// ResendInvitationsRepositoryInput 是仓储层重发邀请码的事务输入。
// 仓储层负责用 UserIDs 和 AllowedSpaceIDs 求交集，防止 API 层或 service 层漏过滤导致越权记录。
type ResendInvitationsRepositoryInput struct {
	TenantID        uint64       // 所属租户 ID。
	ExamID          uint64       // 考试 ID。
	UserIDs         []uint64     // 已去重后的目标考生用户 ID。
	AllowedSpaceIDs []uint64     // 当前管理者在本场考试内可管理的空间范围。
	Log             OperationLog // 与本次重发动作一起写入的操作日志。
}

// ResendInvitationsRepositoryResult 是仓储层重发邀请码后的结果摘要。
type ResendInvitationsRepositoryResult struct {
	SentUserIDs  []uint64 // 通过应考名单与授权范围过滤的用户 ID。
	SentCount    int      // 通过校验的人数。
	SkippedCount int      // 非应考、越权或无效用户数量。
}

// ResultSummaryRepositoryInput 是仓储层成绩摘要查询条件。
type ResultSummaryRepositoryInput struct {
	TenantID   uint64   // 所属租户 ID。
	ExamID     uint64   // 考试 ID。
	SpaceIDs   []uint64 // 当前管理者可见空间范围。
	PassScore  float64  // 及格分数线。
	TotalScore float64  // 试卷总分，用于仓储层生成与试卷满分匹配的分数段。
}

// ResultSummaryRepositoryResult 是仓储层已聚合的成绩摘要。
type ResultSummaryRepositoryResult struct {
	Submitted         int                       // 已交卷数量。
	AverageScore      string                    // 平均分。
	HighestScore      string                    // 最高分。
	Passed            int                       // 达到及格线人数。
	PendingSubjective int                       // 有待阅主观题的作答数量。
	ScoreDistribution []ResultScoreDistribution // 基于真实已交卷成绩聚合的分数段人数。
	QuestionTypeRates []ResultQuestionTypeRate  // 基于答案明细聚合的题型得分率。
}

// ListExamResultsInput 是仓储层成绩列表查询条件。
type ListExamResultsInput struct {
	TenantID uint64   // 所属租户 ID。
	ExamID   uint64   // 考试 ID。
	SpaceIDs []uint64 // 当前管理者可见空间范围。
	Keyword  string   // 姓名、用户名或空间名搜索关键字。
	Status   string   // 成绩状态筛选。
	Page     int      // 页码。
	PageSize int      // 每页数量。
}

// AnswerSheetRepositoryInput 是仓储层答卷详情查询条件。
type AnswerSheetRepositoryInput struct {
	TenantID  uint64   // 所属租户 ID。
	ExamID    uint64   // 考试 ID，必须和 AttemptID 同时命中。
	AttemptID uint64   // 作答 ID。
	SpaceIDs  []uint64 // 当前管理者可见空间范围。
}

// AnswerSheetRepositoryResult 是仓储层返回的答卷详情。
type AnswerSheetRepositoryResult struct {
	Attempt AnswerSheetAttempt // 作答摘要。
	Items   []AnswerSheetItem  // 单题答卷明细。
}

// UpdateManagementSettingsRepositoryInput 是仓储层考试设置写入条件。
// Log 必须和考试发布配置更新放在同一事务，保证设置变更具备可审计性。
type UpdateManagementSettingsRepositoryInput struct {
	TenantID         uint64       // 所属租户 ID。
	ExamID           uint64       // 考试 ID。
	PublishMode      string       // 成绩发布模式。
	ScorePublishTime *int64       // 统一成绩发布时间。
	Log              OperationLog // 本次设置变更的操作日志。
}

// ManagementDetailRepository 抽象管理端详情页依赖的数据读取能力。
// 该接口刻意保持很窄，避免详情权限 service 直接感知题目、成绩、日志等具体 tab 查询细节。
type ManagementDetailRepository interface {
	GetExam(ctx context.Context, tenantID uint64, examID uint64) (Exam, error)
	ListTargets(ctx context.Context, tenantID uint64, examID uint64) ([]Target, error)
	ExamTargetSpaceIDs(ctx context.Context, tenantID uint64, examID uint64) ([]uint64, error)
	CountExamCandidates(ctx context.Context, tenantID uint64, examID uint64, spaceIDs []uint64) (int, error)
	CountExamAttemptStats(ctx context.Context, tenantID uint64, examID uint64, spaceIDs []uint64) (AttemptOverviewStats, error)
	ListExamCandidates(ctx context.Context, input ListExamCandidatesInput) (pagination.Result[ExamCandidate], error)
	ImportCandidateTargets(ctx context.Context, input ImportCandidateTargetsInput) (ImportCandidateTargetsResult, error)
	ResendInvitations(ctx context.Context, input ResendInvitationsRepositoryInput) (ResendInvitationsRepositoryResult, error)
	SummarizeExamResults(ctx context.Context, input ResultSummaryRepositoryInput) (ResultSummaryRepositoryResult, error)
	ListExamResults(ctx context.Context, input ListExamResultsInput) (pagination.Result[ExamResult], error)
	GetAnswerSheet(ctx context.Context, input AnswerSheetRepositoryInput) (AnswerSheetRepositoryResult, error)
	UpdateManagementSettings(ctx context.Context, input UpdateManagementSettingsRepositoryInput) (Exam, error)
	ListOperationLogs(ctx context.Context, input ListOperationLogsInput) (pagination.Result[OperationLog], error)
	ListFixedSnapshotQuestions(ctx context.Context, tenantID uint64, examID uint64) ([]SnapshotSourceQuestion, error)
	ListFrozenLiveSnapshotQuestions(ctx context.Context, tenantID uint64, examID uint64) ([]SnapshotSourceQuestion, error)
}

// ManagementDetailServiceOptions 是管理端详情 service 的依赖配置。
type ManagementDetailServiceOptions struct {
	Repo              ManagementDetailRepository
	PermissionChecker permission.PermissionChecker
	Now               func() int64
}

// ManagementDetailService 统一计算考试详情页的管理范围和按钮权限。
type ManagementDetailService struct {
	repo              ManagementDetailRepository
	permissionChecker permission.PermissionChecker
	now               func() int64
}

// NewManagementDetailService 创建管理端考试详情 service。
func NewManagementDetailService(options ManagementDetailServiceOptions) *ManagementDetailService {
	checker := options.PermissionChecker
	if checker == nil {
		checker = permission.NewFixedRoleChecker()
	}
	now := options.Now
	if now == nil {
		now = func() int64 { return time.Now().UnixMilli() }
	}
	return &ManagementDetailService{
		repo:              options.Repo,
		permissionChecker: checker,
		now:               now,
	}
}

// GetDetail 读取考试详情聚合上下文，并按当前管理者权限裁剪可见空间范围。
func (s *ManagementDetailService) GetDetail(ctx context.Context, input ManagementDetailInput) (ManagementDetail, error) {
	if !canEnterManagementDetail(input.Permission, input.TenantID) {
		return ManagementDetail{}, permission.ErrForbidden
	}
	exam, err := s.repo.GetExam(ctx, input.TenantID, input.ExamID)
	if err != nil {
		return ManagementDetail{}, err
	}
	targets, err := s.repo.ListTargets(ctx, input.TenantID, input.ExamID)
	if err != nil {
		return ManagementDetail{}, err
	}
	targetSpaceIDs, err := s.repo.ExamTargetSpaceIDs(ctx, input.TenantID, input.ExamID)
	if err != nil {
		return ManagementDetail{}, err
	}
	allowedSpaceIDs := s.allowedManagementSpaceIDs(input.Permission, exam.ID, targetSpaceIDs)
	if len(allowedSpaceIDs) == 0 && input.Permission.Role != permission.RoleTenantAdmin {
		return ManagementDetail{}, permission.ErrForbidden
	}
	permissions := s.managementPermissions(input.Permission, exam.ID, allowedSpaceIDs)
	if !permissions.CanViewDetail {
		return ManagementDetail{}, permission.ErrForbidden
	}
	return ManagementDetail{
		Exam:            exam,
		Targets:         cloneTargets(targets),
		TargetSpaceIDs:  cloneUint64s(targetSpaceIDs),
		AllowedSpaceIDs: allowedSpaceIDs,
		Permissions:     permissions,
	}, nil
}

// GetOverview 返回考试概览数据，并复用详情权限裁剪范围。
func (s *ManagementDetailService) GetOverview(ctx context.Context, input ManagementDetailInput) (ExamOverviewData, error) {
	detail, err := s.GetDetail(ctx, input)
	if err != nil {
		return ExamOverviewData{}, err
	}
	planned, err := s.repo.CountExamCandidates(ctx, input.TenantID, input.ExamID, detail.AllowedSpaceIDs)
	if err != nil {
		return ExamOverviewData{}, err
	}
	attemptStats, err := s.repo.CountExamAttemptStats(ctx, input.TenantID, input.ExamID, detail.AllowedSpaceIDs)
	if err != nil {
		return ExamOverviewData{}, err
	}
	sources, err := s.previewSources(ctx, detail.Exam)
	if err != nil {
		return ExamOverviewData{}, err
	}
	activities, err := s.recentActivities(ctx, input.Permission, input.TenantID, input.ExamID, detail.AllowedSpaceIDs)
	if err != nil {
		return ExamOverviewData{}, err
	}
	return ExamOverviewData{
		Detail: detail,
		CandidateStats: OverviewCandidateStats{
			Planned:    planned,
			Joined:     attemptStats.Joined,
			Submitted:  attemptStats.Submitted,
			InProgress: attemptStats.InProgress,
		},
		QuestionTypes:    questionTypeStatsFromSources(sources),
		RecentActivities: activities,
	}, nil
}

// GetPaperPreview 返回管理端试卷预览数据，并确保不携带正确答案。
func (s *ManagementDetailService) GetPaperPreview(ctx context.Context, input PaperPreviewInput) (ExamPaperPreviewData, error) {
	detail, err := s.GetDetail(ctx, input.ManagementDetailInput)
	if err != nil {
		return ExamPaperPreviewData{}, err
	}
	sources, err := s.previewSources(ctx, detail.Exam)
	if err != nil {
		return ExamPaperPreviewData{}, err
	}
	sections := previewSectionsFromSources(sources)
	filtered := filterPreviewSources(sources, input.QuestionType)
	page := pagination.Normalize(pagination.Input{Page: input.Page, PageSize: input.PageSize})
	start := pagination.Offset(page)
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + page.PageSize
	if end > len(filtered) {
		end = len(filtered)
	}
	return ExamPaperPreviewData{
		Detail:   detail,
		Sections: sections,
		Items:    previewQuestionsFromSources(filtered[start:end], start),
		Page:     page.Page,
		PageSize: page.PageSize,
		Total:    len(filtered),
	}, nil
}

// ListCandidates 返回当前管理者授权范围内的应考考生分页列表。
func (s *ManagementDetailService) ListCandidates(ctx context.Context, input CandidateListInput) (CandidateListData, error) {
	detail, err := s.GetDetail(ctx, input.ManagementDetailInput)
	if err != nil {
		return CandidateListData{}, err
	}
	if !detail.Permissions.CanViewCandidates {
		return CandidateListData{}, permission.ErrForbidden
	}
	result, err := s.repo.ListExamCandidates(ctx, ListExamCandidatesInput{
		TenantID:       input.TenantID,
		ExamID:         input.ExamID,
		SpaceIDs:       detail.AllowedSpaceIDs,
		Keyword:        input.Keyword,
		Status:         input.Status,
		ResultStrategy: detail.Exam.ResultStrategy,
		Page:           input.Page,
		PageSize:       input.PageSize,
	})
	if err != nil {
		return CandidateListData{}, err
	}
	return CandidateListData{
		Detail:   detail,
		Items:    result.Items,
		Page:     result.Page,
		PageSize: result.PageSize,
		Total:    result.Total,
	}, nil
}

// GetResultsSummary 返回成绩管理摘要统计和图表数据。
func (s *ManagementDetailService) GetResultsSummary(ctx context.Context, input ManagementDetailInput) (ResultSummaryData, error) {
	detail, err := s.GetDetail(ctx, input)
	if err != nil {
		return ResultSummaryData{}, err
	}
	if !detail.Permissions.CanViewResults {
		return ResultSummaryData{}, permission.ErrForbidden
	}
	sources, err := s.previewSources(ctx, detail.Exam)
	if err != nil {
		return ResultSummaryData{}, err
	}
	totalScore := totalScoreFromSources(sources)
	passScore := totalScore * 0.6
	summary, err := s.repo.SummarizeExamResults(ctx, ResultSummaryRepositoryInput{
		TenantID:   input.TenantID,
		ExamID:     input.ExamID,
		SpaceIDs:   detail.AllowedSpaceIDs,
		PassScore:  passScore,
		TotalScore: totalScore,
	})
	if err != nil {
		return ResultSummaryData{}, err
	}
	return ResultSummaryData{
		Detail: detail,
		Stats: ResultSummaryStats{
			Submitted:         summary.Submitted,
			AverageScore:      summary.AverageScore,
			HighestScore:      summary.HighestScore,
			PassRate:          formatPassRate(summary.Passed, summary.Submitted),
			PendingSubjective: summary.PendingSubjective,
		},
		ScoreDistribution: summary.ScoreDistribution,
		QuestionTypeRates: summary.QuestionTypeRates,
	}, nil
}

// ListResults 返回当前管理者授权范围内的成绩分页列表。
func (s *ManagementDetailService) ListResults(ctx context.Context, input ResultListInput) (ResultListData, error) {
	detail, err := s.GetDetail(ctx, input.ManagementDetailInput)
	if err != nil {
		return ResultListData{}, err
	}
	if !detail.Permissions.CanViewResults {
		return ResultListData{}, permission.ErrForbidden
	}
	result, err := s.repo.ListExamResults(ctx, ListExamResultsInput{
		TenantID: input.TenantID,
		ExamID:   input.ExamID,
		SpaceIDs: detail.AllowedSpaceIDs,
		Keyword:  input.Keyword,
		Status:   input.Status,
		Page:     input.Page,
		PageSize: input.PageSize,
	})
	if err != nil {
		return ResultListData{}, err
	}
	return ResultListData{
		Detail:   detail,
		Items:    result.Items,
		Page:     result.Page,
		PageSize: result.PageSize,
		Total:    result.Total,
	}, nil
}

// GetAnswerSheet 返回当前管理者授权范围内的答卷详情。
// 答卷详情属于成绩管理的深层数据，必须复用成绩查看权限，并由仓储按 examID + attemptID 双重定位。
func (s *ManagementDetailService) GetAnswerSheet(ctx context.Context, input AnswerSheetInput) (AnswerSheetData, error) {
	detail, err := s.GetDetail(ctx, input.ManagementDetailInput)
	if err != nil {
		return AnswerSheetData{}, err
	}
	if !detail.Permissions.CanViewResults {
		return AnswerSheetData{}, permission.ErrForbidden
	}
	result, err := s.repo.GetAnswerSheet(ctx, AnswerSheetRepositoryInput{
		TenantID:  input.TenantID,
		ExamID:    input.ExamID,
		AttemptID: input.AttemptID,
		SpaceIDs:  detail.AllowedSpaceIDs,
	})
	if err != nil {
		return AnswerSheetData{}, err
	}
	return AnswerSheetData{
		Detail:  detail,
		Attempt: result.Attempt,
		Items:   result.Items,
	}, nil
}

// UpdateSettings 更新考试详情页的设置项。
// 首版只开放成绩发布配置，教师即使可以查看成绩也不能修改设置。
func (s *ManagementDetailService) UpdateSettings(ctx context.Context, input ManagementSettingsInput) (ManagementSettingsData, error) {
	detail, err := s.GetDetail(ctx, input.ManagementDetailInput)
	if err != nil {
		return ManagementSettingsData{}, err
	}
	if !detail.Permissions.CanUpdateSettings {
		return ManagementSettingsData{}, permission.ErrForbidden
	}
	log := OperationLog{
		TenantID:        input.TenantID,
		ExamID:          input.ExamID,
		OperationType:   OperationTypeUpdateSettings,
		OperationTitle:  "修改考试设置",
		OperationDetail: "修改成绩发布配置为 " + input.PublishMode,
		ActorID:         input.Permission.UserID,
		ActorType:       input.Permission.SubjectType,
		ActorRole:       input.Permission.Role,
		CreatedBy:       input.Permission.UserID,
		CreatedByType:   input.Permission.SubjectType,
	}
	exam, err := s.repo.UpdateManagementSettings(ctx, UpdateManagementSettingsRepositoryInput{
		TenantID:         input.TenantID,
		ExamID:           input.ExamID,
		PublishMode:      input.PublishMode,
		ScorePublishTime: input.ScorePublishTime,
		Log:              log,
	})
	if err != nil {
		return ManagementSettingsData{}, err
	}
	detail.Exam = exam
	return ManagementSettingsData{Detail: detail, Exam: exam}, nil
}

// ListLogs 返回当前管理者授权范围内的操作日志。
// 租户管理员可以整场考试分页读取；空间管理员和教师必须先按授权空间查询再合并分页，避免看到其它空间的管理动作。
func (s *ManagementDetailService) ListLogs(ctx context.Context, input OperationLogListInput) (OperationLogListData, error) {
	detail, err := s.GetDetail(ctx, input.ManagementDetailInput)
	if err != nil {
		return OperationLogListData{}, err
	}
	if !detail.Permissions.CanViewLogs {
		return OperationLogListData{}, permission.ErrForbidden
	}
	page := pagination.Normalize(pagination.Input{Page: input.Page, PageSize: input.PageSize})
	if input.Permission.Role == permission.RoleTenantAdmin {
		offset := pagination.Offset(page)
		fetchSize := offset + page.PageSize
		items := make([]OperationLog, 0, fetchSize)
		seenPageGroups := make(map[string]struct{}, fetchSize)
		seenAllGroups := make(map[string]struct{}, fetchSize)
		repositoryPage := 1
		for {
			result, err := s.repo.ListOperationLogs(ctx, ListOperationLogsInput{
				TenantID:      input.TenantID,
				ExamID:        input.ExamID,
				OperationType: input.OperationType,
				Page:          repositoryPage,
				PageSize:      pagination.MaxPageSize,
			})
			if err != nil {
				return OperationLogListData{}, err
			}
			if len(result.Items) == 0 {
				break
			}
			for _, log := range result.Items {
				groupKey := operationLogGroupKey(log)
				seenAllGroups[groupKey] = struct{}{}
				if len(items) >= fetchSize {
					continue
				}
				if _, ok := seenPageGroups[groupKey]; ok {
					continue
				}
				seenPageGroups[groupKey] = struct{}{}
				items = append(items, log)
			}
			if int64(repositoryPage*pagination.MaxPageSize) >= result.Total {
				break
			}
			repositoryPage++
		}
		if offset > len(items) {
			offset = len(items)
		}
		end := offset + page.PageSize
		if end > len(items) {
			end = len(items)
		}
		return OperationLogListData{
			Detail:   detail,
			Items:    items[offset:end],
			Page:     page.Page,
			PageSize: page.PageSize,
			Total:    int64(len(seenAllGroups)),
		}, nil
	}
	if len(detail.AllowedSpaceIDs) == 0 {
		return OperationLogListData{Detail: detail, Items: []OperationLog{}, Page: page.Page, PageSize: page.PageSize}, nil
	}
	// 仓储当前只支持单个 space_id 精确过滤；这里先汇总授权空间内全量候选，再统一去重分页，避免跨空间重复组把较新的唯一日志挤出当前页。
	offset := pagination.Offset(page)
	logs, err := s.listScopedOperationLogs(ctx, input.TenantID, input.ExamID, input.OperationType, detail.AllowedSpaceIDs)
	if err != nil {
		return OperationLogListData{}, err
	}
	sortOperationLogsDesc(logs)
	logs = dedupeOperationLogsByGroup(logs)
	total := int64(len(logs))
	if offset > len(logs) {
		offset = len(logs)
	}
	end := offset + page.PageSize
	if end > len(logs) {
		end = len(logs)
	}
	return OperationLogListData{
		Detail:   detail,
		Items:    logs[offset:end],
		Page:     page.Page,
		PageSize: page.PageSize,
		Total:    total,
	}, nil
}

// ImportCandidates 把授权范围内的启用学生追加为用户直投目标。
// 已开考后名单会影响考试公平性，因此只允许在开考前导入；重复用户会计入跳过数量而不是报错。
func (s *ManagementDetailService) ImportCandidates(ctx context.Context, input ImportCandidatesInput) (ImportCandidatesResult, error) {
	detail, err := s.GetDetail(ctx, input.ManagementDetailInput)
	if err != nil {
		return ImportCandidatesResult{}, err
	}
	if !detail.Permissions.CanManageCandidates {
		return ImportCandidatesResult{}, permission.ErrForbidden
	}
	if detail.Exam.StartTime > 0 && s.now() >= detail.Exam.StartTime {
		return ImportCandidatesResult{}, ErrExamAlreadyStarted
	}
	userIDs, duplicateCount := uniqueUint64sWithDuplicateCount(input.UserIDs)
	if len(userIDs) == 0 {
		return ImportCandidatesResult{Detail: detail, SkippedCount: duplicateCount}, nil
	}
	log := OperationLog{
		TenantID:        input.TenantID,
		ExamID:          input.ExamID,
		OperationType:   OperationTypeImportCandidates,
		OperationTitle:  "导入考生",
		OperationDetail: "导入 " + strconv.Itoa(len(userIDs)) + " 名考生",
		ActorID:         input.Permission.UserID,
		ActorType:       input.Permission.SubjectType,
		ActorRole:       input.Permission.Role,
		CreatedBy:       input.Permission.UserID,
		CreatedByType:   input.Permission.SubjectType,
	}
	result, err := s.repo.ImportCandidateTargets(ctx, ImportCandidateTargetsInput{
		TenantID:        input.TenantID,
		ExamID:          input.ExamID,
		UserIDs:         userIDs,
		AllowedSpaceIDs: detail.AllowedSpaceIDs,
		Log:             log,
	})
	if err != nil {
		return ImportCandidatesResult{}, err
	}
	return ImportCandidatesResult{
		Detail:        detail,
		ImportedCount: result.ImportedCount,
		SkippedCount:  result.SkippedCount + duplicateCount,
	}, nil
}

// ResendInvitations 对当前授权范围内的应考用户重发考试邀请码。
// 当前版本没有外部通知通道，重发动作的后端含义是校验可发送目标、返回邀请码并写入 send_invite 审计日志。
func (s *ManagementDetailService) ResendInvitations(ctx context.Context, input ResendInvitationsInput) (ResendInvitationsResult, error) {
	detail, err := s.GetDetail(ctx, input.ManagementDetailInput)
	if err != nil {
		return ResendInvitationsResult{}, err
	}
	if !detail.Permissions.CanManageCandidates {
		return ResendInvitationsResult{}, permission.ErrForbidden
	}
	if detail.Exam.StartTime > 0 && s.now() >= detail.Exam.StartTime {
		return ResendInvitationsResult{}, ErrExamAlreadyStarted
	}
	userIDs, duplicateCount := uniqueUint64sWithDuplicateCount(input.UserIDs)
	if len(userIDs) == 0 {
		return ResendInvitationsResult{Detail: detail, SkippedCount: duplicateCount, InviteCode: detail.Exam.InviteCode}, nil
	}
	log := OperationLog{
		TenantID:        input.TenantID,
		ExamID:          input.ExamID,
		OperationType:   OperationTypeSendInvite,
		OperationTitle:  "重发邀请码",
		OperationDetail: "重发 " + strconv.Itoa(len(userIDs)) + " 名考生邀请码",
		ActorID:         input.Permission.UserID,
		ActorType:       input.Permission.SubjectType,
		ActorRole:       input.Permission.Role,
		CreatedBy:       input.Permission.UserID,
		CreatedByType:   input.Permission.SubjectType,
	}
	result, err := s.repo.ResendInvitations(ctx, ResendInvitationsRepositoryInput{
		TenantID:        input.TenantID,
		ExamID:          input.ExamID,
		UserIDs:         userIDs,
		AllowedSpaceIDs: detail.AllowedSpaceIDs,
		Log:             log,
	})
	if err != nil {
		return ResendInvitationsResult{}, err
	}
	return ResendInvitationsResult{
		Detail:       detail,
		SentCount:    result.SentCount,
		SkippedCount: result.SkippedCount + duplicateCount,
		InviteCode:   detail.Exam.InviteCode,
	}, nil
}

func (s *ManagementDetailService) previewSources(ctx context.Context, exam Exam) ([]SnapshotSourceQuestion, error) {
	if exam.BuildMode == BuildModeRuleLive {
		return s.repo.ListFrozenLiveSnapshotQuestions(ctx, exam.TenantID, exam.ID)
	}
	return s.repo.ListFixedSnapshotQuestions(ctx, exam.TenantID, exam.ID)
}

func (s *ManagementDetailService) recentActivities(ctx context.Context, permissionContext permission.PermissionContext, tenantID uint64, examID uint64, allowedSpaceIDs []uint64) ([]OverviewActivity, error) {
	// 租户管理员可以查看整场考试日志；空间管理员和教师必须按授权空间精确过滤，避免近期动态泄露其它空间操作。
	if permissionContext.Role == permission.RoleTenantAdmin {
		logs := make([]OperationLog, 0, 4)
		seenGroups := make(map[string]struct{}, 4)
		repositoryPage := 1
		for len(logs) < 4 {
			result, err := s.repo.ListOperationLogs(ctx, ListOperationLogsInput{
				TenantID: tenantID,
				ExamID:   examID,
				Page:     repositoryPage,
				PageSize: pagination.MaxPageSize,
			})
			if err != nil {
				return nil, err
			}
			if len(result.Items) == 0 {
				break
			}
			for _, log := range result.Items {
				groupKey := operationLogGroupKey(log)
				if _, ok := seenGroups[groupKey]; ok {
					continue
				}
				seenGroups[groupKey] = struct{}{}
				logs = append(logs, log)
				if len(logs) == 4 {
					break
				}
			}
			if int64(repositoryPage*pagination.MaxPageSize) >= result.Total {
				break
			}
			repositoryPage++
		}
		return overviewActivitiesFromLogs(logs), nil
	}
	if len(allowedSpaceIDs) == 0 {
		return []OverviewActivity{}, nil
	}
	logs, err := s.listScopedOperationLogs(ctx, tenantID, examID, "", allowedSpaceIDs)
	if err != nil {
		return nil, err
	}
	sortOperationLogsDesc(logs)
	logs = dedupeOperationLogsByGroup(logs)
	if len(logs) > 4 {
		logs = logs[:4]
	}
	return overviewActivitiesFromLogs(logs), nil
}

func (s *ManagementDetailService) listScopedOperationLogs(ctx context.Context, tenantID uint64, examID uint64, operationType string, allowedSpaceIDs []uint64) ([]OperationLog, error) {
	logs := make([]OperationLog, 0, len(allowedSpaceIDs)*pagination.MaxPageSize)
	for _, spaceID := range allowedSpaceIDs {
		scopeSpaceID := spaceID
		repositoryPage := 1
		for {
			result, err := s.repo.ListOperationLogs(ctx, ListOperationLogsInput{
				TenantID:      tenantID,
				ExamID:        examID,
				SpaceID:       &scopeSpaceID,
				OperationType: operationType,
				Page:          repositoryPage,
				PageSize:      pagination.MaxPageSize,
			})
			if err != nil {
				return nil, err
			}
			if len(result.Items) == 0 {
				break
			}
			logs = append(logs, result.Items...)
			if int64(repositoryPage*pagination.MaxPageSize) >= result.Total {
				break
			}
			repositoryPage++
		}
	}
	return logs, nil
}

// sortOperationLogsDesc 保持操作日志统一按最新时间和更大 ID 优先展示。
func sortOperationLogsDesc(logs []OperationLog) {
	sort.SliceStable(logs, func(left, right int) bool {
		if logs[left].CreatedAt == logs[right].CreatedAt {
			return logs[left].ID > logs[right].ID
		}
		return logs[left].CreatedAt > logs[right].CreatedAt
	})
}

func dedupeOperationLogsByGroup(logs []OperationLog) []OperationLog {
	seen := make(map[string]struct{}, len(logs))
	items := make([]OperationLog, 0, len(logs))
	for _, log := range logs {
		key := operationLogGroupKey(log)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		items = append(items, log)
	}
	return items
}

func operationLogGroupKey(log OperationLog) string {
	var payload struct {
		OperationGroupID string `json:"operation_group_id"`
	}
	if strings.TrimSpace(log.ExtJSON) != "" && json.Unmarshal([]byte(log.ExtJSON), &payload) == nil && payload.OperationGroupID != "" {
		return payload.OperationGroupID
	}
	return strconv.FormatUint(log.ID, 10)
}

// overviewActivitiesFromLogs 将审计日志转换成概览动态，避免概览层暴露日志表的全部字段。
func overviewActivitiesFromLogs(logs []OperationLog) []OverviewActivity {
	activities := make([]OverviewActivity, 0, len(logs))
	for _, item := range logs {
		activities = append(activities, OverviewActivity{
			OperationType:   item.OperationType,
			OperationTitle:  item.OperationTitle,
			OperationDetail: item.OperationDetail,
			CreatedAt:       item.CreatedAt,
		})
	}
	return activities
}

func questionTypeStatsFromSources(sources []SnapshotSourceQuestion) []OverviewQuestionTypeStat {
	order := make([]string, 0)
	stats := map[string]*OverviewQuestionTypeStat{}
	for _, source := range sources {
		stat, ok := stats[source.QuestionType]
		if !ok {
			order = append(order, source.QuestionType)
			stat = &OverviewQuestionTypeStat{QuestionType: source.QuestionType}
			stats[source.QuestionType] = stat
		}
		stat.QuestionCount++
		stat.TotalScore = addScoreStrings(stat.TotalScore, source.Score)
	}
	items := make([]OverviewQuestionTypeStat, 0, len(order))
	for _, questionType := range order {
		items = append(items, *stats[questionType])
	}
	return items
}

func previewSectionsFromSources(sources []SnapshotSourceQuestion) []PaperPreviewSection {
	order := make([]uint64, 0)
	sections := map[uint64]*PaperPreviewSection{}
	for _, source := range sources {
		section, ok := sections[source.SectionID]
		if !ok {
			order = append(order, source.SectionID)
			section = &PaperPreviewSection{
				SectionID:    source.SectionID,
				SectionName:  source.SectionName,
				QuestionType: source.QuestionType,
			}
			sections[source.SectionID] = section
		}
		section.QuestionCount++
		section.TotalScore = addScoreStrings(section.TotalScore, source.Score)
	}
	items := make([]PaperPreviewSection, 0, len(order))
	for _, sectionID := range order {
		items = append(items, *sections[sectionID])
	}
	return items
}

func filterPreviewSources(sources []SnapshotSourceQuestion, questionType string) []SnapshotSourceQuestion {
	if questionType == "" {
		return sources
	}
	filtered := make([]SnapshotSourceQuestion, 0, len(sources))
	for _, source := range sources {
		if source.QuestionType == questionType {
			filtered = append(filtered, source)
		}
	}
	return filtered
}

func previewQuestionsFromSources(sources []SnapshotSourceQuestion, offset int) []PaperPreviewQuestion {
	items := make([]PaperPreviewQuestion, 0, len(sources))
	for index, source := range sources {
		options := make([]PaperPreviewOption, 0, len(source.Options))
		for _, option := range source.Options {
			options = append(options, PaperPreviewOption{
				ID:      option.ID,
				Key:     option.Key,
				Content: option.Content,
			})
		}
		items = append(items, PaperPreviewQuestion{
			SectionID:    source.SectionID,
			SectionName:  source.SectionName,
			QuestionID:   source.QuestionID,
			QuestionType: source.QuestionType,
			Title:        source.Title,
			Score:        source.Score,
			BlankCount:   source.BlankCount,
			SortOrder:    offset + index + 1,
			Options:      options,
		})
	}
	return items
}

func addScoreStrings(left string, right string) string {
	leftScore, err := parseGradingScore(left)
	if err != nil {
		return left
	}
	rightScore, err := parseGradingScore(right)
	if err != nil {
		return left
	}
	return formatGradingScore(leftScore + rightScore)
}

func totalScoreFromSources(sources []SnapshotSourceQuestion) float64 {
	total := 0.0
	for _, source := range sources {
		score, err := parseGradingScore(source.Score)
		if err != nil {
			continue
		}
		total += score
	}
	return total
}

func formatPassRate(passed int, submitted int) string {
	if submitted == 0 {
		return "0%"
	}
	rate := int(math.Round(float64(passed) / float64(submitted) * 100))
	return strconv.Itoa(rate) + "%"
}

// canEnterManagementDetail 先做管理端入口级校验，避免学生或跨租户主体触发后续数据读取。
func canEnterManagementDetail(ctx permission.PermissionContext, tenantID uint64) bool {
	if ctx.SubjectType != permission.SubjectTenantUser || ctx.TenantID != tenantID {
		return false
	}
	if ctx.Role == permission.RoleTenantAdmin || ctx.Role == permission.RoleSpaceAdmin || ctx.Role == permission.RoleTeacher {
		return true
	}
	for _, role := range ctx.SpaceMemberships {
		if role == permission.RoleSpaceAdmin || role == permission.RoleTeacher {
			return true
		}
	}
	return false
}

// allowedManagementSpaceIDs 将考试投放空间和当前管理者空间授权求交集。
// 租户管理员不需要空间交集，直接保留整场考试当前有效投放空间。
func (s *ManagementDetailService) allowedManagementSpaceIDs(ctx permission.PermissionContext, examID uint64, targetSpaceIDs []uint64) []uint64 {
	if ctx.Role == permission.RoleTenantAdmin {
		return cloneUint64s(targetSpaceIDs)
	}
	allowed := make([]uint64, 0, len(targetSpaceIDs))
	for _, spaceID := range uniqueSortedUint64s(targetSpaceIDs) {
		if err := s.permissionChecker.CanViewExamResults(permissionWithExamScope(ctx, examID, spaceID), examID); err == nil {
			allowed = append(allowed, spaceID)
		}
	}
	return allowed
}

// managementPermissions 基于已裁剪的空间范围生成前端按钮权限。
// 候选人管理允许 scoped space admin 操作；考试设置和成绩发布属于整场考试的全局写入，只开放给租户管理员。
func (s *ManagementDetailService) managementPermissions(ctx permission.PermissionContext, examID uint64, allowedSpaceIDs []uint64) ManagementPermissions {
	canViewResults := false
	canExportResults := false
	for _, spaceID := range allowedSpaceIDs {
		scoped := permissionWithExamScope(ctx, examID, spaceID)
		if s.permissionChecker.CanViewExamResults(scoped, examID) == nil {
			canViewResults = true
		}
		if s.permissionChecker.CanExportExamResults(scoped, examID) == nil {
			canExportResults = true
		}
	}
	if ctx.Role == permission.RoleTenantAdmin && len(allowedSpaceIDs) == 0 {
		scoped := permissionWithExamScope(ctx, examID, 0)
		canViewResults = s.permissionChecker.CanViewExamResults(scoped, examID) == nil
		canExportResults = s.permissionChecker.CanExportExamResults(scoped, examID) == nil
	}
	canManageCandidates := ctx.Role == permission.RoleTenantAdmin || canExportResults
	canWriteGlobalSettings := ctx.Role == permission.RoleTenantAdmin
	return ManagementPermissions{
		CanViewDetail:       canViewResults,
		CanViewOverview:     canViewResults,
		CanViewPaper:        canViewResults,
		CanViewCandidates:   canViewResults,
		CanManageCandidates: canManageCandidates,
		CanViewResults:      canViewResults,
		CanExportResults:    canExportResults,
		CanPublishResults:   canWriteGlobalSettings,
		CanUpdateSettings:   canWriteGlobalSettings,
		CanViewLogs:         canViewResults,
	}
}

// cloneTargets 返回目标切片副本，避免调用方修改聚合结果时污染仓储返回值。
func cloneTargets(items []Target) []Target {
	next := make([]Target, len(items))
	copy(next, items)
	return next
}

// cloneUint64s 返回去重排序后的 ID 副本，保证后续分页和权限判断顺序稳定。
func cloneUint64s(items []uint64) []uint64 {
	next := make([]uint64, len(items))
	copy(next, items)
	return uniqueSortedUint64s(next)
}

// uniqueUint64sWithDuplicateCount 保留用户输入的首次出现顺序，同时统计重复或空 ID。
// 导入考生需要向用户解释跳过数量，但不应因为一个重复 ID 让整批导入失败。
func uniqueUint64sWithDuplicateCount(items []uint64) ([]uint64, int) {
	seen := make(map[uint64]struct{}, len(items))
	unique := make([]uint64, 0, len(items))
	skipped := 0
	for _, item := range items {
		if item == 0 {
			skipped++
			continue
		}
		if _, ok := seen[item]; ok {
			skipped++
			continue
		}
		seen[item] = struct{}{}
		unique = append(unique, item)
	}
	return unique, skipped
}

// uniqueSortedUint64s 在原切片上排序去重，调用方必须确保传入的是可修改副本。
func uniqueSortedUint64s(items []uint64) []uint64 {
	if len(items) == 0 {
		return []uint64{}
	}
	sort.Slice(items, func(i int, j int) bool { return items[i] < items[j] })
	write := 0
	var previous uint64
	for index, item := range items {
		if index > 0 && item == previous {
			continue
		}
		items[write] = item
		write++
		previous = item
	}
	return items[:write]
}

package exam

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"time"

	"github.com/lifei6671/papermind/server/internal/service/pagination"
	"github.com/lifei6671/papermind/server/library/constant"
)

const (
	// StatusDraft 表示考试草稿状态。
	StatusDraft = constant.ExamStatusDraft
	// StatusPublished 表示考试已发布。
	StatusPublished = constant.ExamStatusPublished
	// StatusClosed 表示考试已提前结束。
	StatusClosed = constant.ExamStatusClosed
	// StatusDisabled 表示考试已禁用。
	StatusDisabled = constant.ExamStatusDisabled

	// BuildModeManual 表示手动固化组卷。
	BuildModeManual = constant.BuildModeManual
	// BuildModeRuleFixed 表示规则生成后固化组卷。
	BuildModeRuleFixed = constant.BuildModeRuleFixed
	// BuildModeRuleLive 表示实时抽题组卷。
	BuildModeRuleLive = constant.BuildModeRuleLive

	// ResultStrategyLatest 表示多次作答取最近一次成绩。
	ResultStrategyLatest = "latest"
	// ResultStrategyHighest 表示多次作答取最高成绩。
	ResultStrategyHighest = "highest"

	// PublishModeImmediateScore 表示提交后立即出分。
	PublishModeImmediateScore = "immediate_score"
	// PublishModeManualPublish 表示教师阅卷后发布成绩。
	PublishModeManualPublish = "manual_publish"

	// TargetTypeSpace 表示发布给空间。
	TargetTypeSpace = "space"
	// TargetTypeUser 表示发布给指定用户。
	TargetTypeUser = "user"

	// AttemptStatusInProgress 表示作答进行中。
	AttemptStatusInProgress = constant.AttemptStatusInProgress
	// AttemptStatusSubmitted 表示作答已提交。
	AttemptStatusSubmitted = constant.AttemptStatusSubmitted

	// CandidateStatusNotStarted 表示应考考生尚未开始作答。
	CandidateStatusNotStarted = "not_started"
	// CandidateStatusInProgress 表示应考考生存在未提交的进行中作答。
	CandidateStatusInProgress = "in_progress"
	// CandidateStatusSubmitted 表示应考考生已经存在可计入成绩的已提交作答。
	CandidateStatusSubmitted = "submitted"

	// OperationTypePublishExam 表示发布考试的管理端操作。
	OperationTypePublishExam = "publish_exam"
	// OperationTypeSendInvite 表示发送或重发考试邀请码的管理端操作。
	OperationTypeSendInvite = "send_invite"
	// OperationTypeImportCandidates 表示导入应考考生的管理端操作。
	OperationTypeImportCandidates = "import_candidates"
	// OperationTypeExportResults 表示导出考试成绩的管理端操作。
	OperationTypeExportResults = "export_results"
	// OperationTypePublishResults 表示发布考试成绩的管理端操作。
	OperationTypePublishResults = "publish_results"
	// OperationTypeUpdateSettings 表示修改考试设置的管理端操作。
	OperationTypeUpdateSettings = "update_settings"
	// OperationTypeGradeAnswer 表示人工阅卷的管理端操作。
	OperationTypeGradeAnswer = "grade_answer"
	// OperationTypeSystemEvent 表示系统自动补充的管理端操作日志。
	OperationTypeSystemEvent = "system_event"
)

var (
	ErrDurationExceedsExamWindow        = errors.New("duration exceeds exam window")
	ErrShortTextCannotRepeatAttempt     = errors.New("short text exam cannot repeat attempt")
	ErrShortTextCannotImmediateScore    = errors.New("short text exam cannot immediate score")
	ErrPaperNotEnabled                  = errors.New("paper not enabled")
	ErrDuplicateExamTarget              = errors.New("duplicate exam target")
	ErrLoginRequiredForInvite           = errors.New("login or register required for invite")
	ErrExamNotEligible                  = errors.New("exam not eligible")
	ErrExamNotStarted                   = errors.New("exam not started")
	ErrExamEnded                        = errors.New("exam ended")
	ErrExamAlreadyStarted               = errors.New("exam already started")
	ErrExamTargetRequired               = errors.New("exam target required")
	ErrInvalidExamStatus                = errors.New("invalid exam status")
	ErrAttemptNotFound                  = errors.New("attempt not found")
	ErrMaxAttemptsReached               = errors.New("max attempts reached")
	ErrAttemptUniqueConflict            = errors.New("attempt unique conflict")
	ErrExamTokenAttemptMismatch         = errors.New("exam token attempt mismatch")
	ErrExamTokenInvalid                 = errors.New("exam token invalid")
	ErrInviteCodeCollision              = errors.New("invite code collision")
	ErrRuleLiveQuestionPoolInsufficient = errors.New("rule live question pool insufficient")
	ErrAnswerDeadlineExceeded           = errors.New("answer deadline exceeded")
	ErrAttemptAlreadySubmitted          = errors.New("attempt already submitted")
)

const (
	millisPerMinute           int64 = 60 * 1000
	defaultTokenBufferMinutes       = 5
)

type Exam struct {
	ID               uint64 // 考试主键 ID。
	TenantID         uint64 // 所属租户 ID。
	PaperID          uint64 // 关联试卷 ID。
	Name             string // 考试名称。
	StartTime        int64  // 考试开始时间，Unix 毫秒时间戳。
	EndTime          int64  // 考试结束时间，Unix 毫秒时间戳。
	DurationMinutes  int    // 单次作答时长，单位分钟。
	MaxAttempts      int    // 每名考生最多作答次数。
	ResultStrategy   string // 多次作答成绩策略。
	PublishMode      string // 成绩发布模式。
	ScorePublishTime *int64 // 统一成绩公布时间。
	InviteCode       string // 考试邀请码。
	Status           string // 考试状态。
	BuildMode        string // 冗余的试卷组卷模式，便于快照生成。
	CreatedBy        uint64 // 创建人用户 ID，用于空间角色限制只能操作自己创建的考试。
	CreatedByType    string // 创建人主体类型。
	Targets          []Target
	TargetType       string // 列表和兼容响应使用的首个发布目标类型。
	TargetID         uint64 // 列表和兼容响应使用的首个发布目标 ID。
}

type Paper struct {
	ID                uint64 // 试卷主键 ID。
	BuildMode         string // 组卷方式。
	Status            string // 试卷状态。
	ContainsShortText bool   // 是否包含简答题。
}

type Target struct {
	TenantID      uint64   // 所属租户 ID。
	ExamID        uint64   // 考试 ID。
	TargetType    string   // 发布目标类型：space / user。
	TargetID      uint64   // 发布目标 ID。
	ScopeSpaceIDs []uint64 // 用户直投目标在发布时命中的空间范围；为空时回退到当前有效成员空间。
}

// OperationLog 表示管理端考试操作审计记录。
// service 和 API 层使用该对象传递日志，避免直接依赖数据库 DO 和 JSON 字段实现。
type OperationLog struct {
	ID              uint64  // 操作日志主键 ID。
	TenantID        uint64  // 所属租户 ID。
	ExamID          uint64  // 考试 ID。
	OperationType   string  // 操作类型，例如 publish_exam / send_invite。
	OperationTitle  string  // 操作标题，用于操作日志列表展示。
	OperationDetail string  // 操作详情摘要，避免前端拼接审计文案。
	ActorID         uint64  // 操作人用户 ID，由后端 session 派生。
	ActorType       string  // 操作人主体类型，例如 tenant_user / system。
	ActorRole       string  // 操作发生时的租户级角色快照。
	SpaceID         *uint64 // 操作关联空间；nil 表示租户级操作。
	CreatedAt       int64   // 操作日志创建时间，Unix 毫秒时间戳。
	CreatedBy       uint64  // 创建人主体 ID，通常与 ActorID 一致。
	CreatedByType   string  // 创建人主体类型，通常与 ActorType 一致。
	ExtJSON         string  // JSON 扩展字段，用于保存 operation_group_id 等元数据。
}

type LivePoolItem struct {
	SectionID  uint64 // 大题 ID。
	RuleID     uint64 // 规则 ID。
	QuestionID uint64 // 题目 ID。
}

type Attempt struct {
	ID                 uint64 // 作答主键 ID。
	TenantID           uint64 // 所属租户 ID。
	ExamID             uint64 // 考试 ID。
	UserID             uint64 // 考生用户 ID。
	AttemptNo          int    // 第几次作答。
	Status             string // 作答状态。
	StartedAt          int64  // 开始作答时间。
	SubmittedAt        *int64 // 提交时间，nil 表示尚未提交。
	ExamTokenHash      string // exam token 哈希。
	ExamTokenExpiresAt int64  // exam token 过期时间。
	Version            int64  // 乐观锁版本。
}

type AttemptQuestion struct {
	ID                    uint64 // 考生题目快照 ID。
	TenantID              uint64 // 所属租户 ID。
	AttemptID             uint64 // 作答 ID。
	SectionID             uint64 // 原始大题 ID。
	QuestionID            uint64 // 原始题目 ID。
	SectionSnapshot       string // 大题快照 JSON。
	SortOrder             int    // 全局题号。
	Score                 string // 题目分值。
	QuestionSnapshot      string // 题干快照 JSON。
	OptionSnapshot        string // 选项快照 JSON。
	CorrectAnswerSnapshot string // 正确答案快照 JSON。
}

type SnapshotSourceQuestion struct {
	SectionID        uint64 // 大题 ID。
	SectionName      string // 大题名称。
	Instructions     string // 作答说明。
	QuestionID       uint64 // 题目 ID。
	QuestionType     string // 题型，判分时必须使用快照值避免题库变更影响考试。
	Title            string // 题干。
	Score            string // 分值。
	Options          []SnapshotSourceOption
	BlankCount       int      // 填空题空位数量，供考试前端渲染多个输入框。
	OptionIDs        []uint64 // 最终展示选项 ID 顺序。
	CorrectOptionIDs []uint64 // 正确选项 ID。
	CorrectText      string   // 填空或简答正确答案。
}

type SnapshotSourceOption struct {
	ID      uint64 // 选项 ID，答题保存和客观题判分都以 ID 为准。
	Key     string // 原始选项标签，例如 A/B/C。
	Content string // 选项展示内容。
}

type CreateDraftInput struct {
	TenantID uint64 // 所属租户 ID。
	PaperID  uint64 // 关联试卷 ID。
	Name     string // 考试名称。
}

type PublishInput struct {
	TenantID         uint64 // 所属租户 ID。
	ExamID           uint64 // 考试 ID。
	PaperID          uint64 // 关联试卷 ID。
	StartTime        int64  // 开始时间。
	EndTime          int64  // 结束时间。
	DurationMinutes  int    // 单次作答时长。
	MaxAttempts      int    // 最大作答次数。
	ResultStrategy   string // 成绩策略。
	PublishMode      string // 成绩发布模式。
	ScorePublishTime *int64 // 成绩公布时间。
}

// PublishWithTargetInput 用于一次性创建已发布考试并写入投放目标。
// Targets 是新版本多目标字段；TargetType 和 TargetID 只用于兼容旧单目标请求。
type PublishWithTargetInput struct {
	TenantID         uint64   // 所属租户 ID。
	PaperID          uint64   // 关联试卷 ID。
	Name             string   // 考试名称。
	TargetType       string   // 发布目标类型。
	TargetID         uint64   // 发布目标 ID。
	Targets          []Target // 发布目标列表；非空时优先于 TargetType 和 TargetID。
	StartTime        int64    // 开始时间。
	EndTime          int64    // 结束时间。
	DurationMinutes  int      // 单次作答时长。
	MaxAttempts      int      // 最大作答次数。
	ResultStrategy   string   // 成绩策略。
	PublishMode      string   // 成绩发布模式。
	ScorePublishTime *int64   // 成绩公布时间。
	Status           string   // 创建后的考试状态：draft / published；为空时兼容为 published。
	ActorID          uint64   // 发布人用户 ID，由 API 层从 session 派生。
	ActorType        string   // 发布人主体类型，由 API 层从 session 派生。
	ActorRole        string   // 发布人角色快照，由 API 层从 session 派生。
}

// CreateWithTargetInput 用于一次性创建考试和投放目标。
// 该类型复用发布请求字段，但允许前端明确选择 draft 或 published。
type CreateWithTargetInput = PublishWithTargetInput

type UpdateStatusInput struct {
	TenantID  uint64 // 所属租户 ID。
	ExamID    uint64 // 考试 ID。
	Status    string // 目标状态：published / closed / disabled。
	ActorID   uint64 // 操作人用户 ID，由 API 层从 session 派生。
	ActorType string // 操作人主体类型，由 API 层从 session 派生。
	ActorRole string // 操作人角色快照，由 API 层从 session 派生。
}

type UpdateDraftWithTargetInput struct {
	TenantID         uint64   // 所属租户 ID。
	ExamID           uint64   // 草稿考试 ID。
	PaperID          uint64   // 关联试卷 ID。
	Name             string   // 考试名称。
	TargetType       string   // 发布目标类型。
	TargetID         uint64   // 发布目标 ID。
	Targets          []Target // 发布目标列表；非空时优先于 TargetType 和 TargetID。
	StartTime        int64    // 开始时间。
	EndTime          int64    // 结束时间。
	DurationMinutes  int      // 单次作答时长。
	MaxAttempts      int      // 最大作答次数。
	ResultStrategy   string   // 成绩策略。
	PublishMode      string   // 成绩发布模式。
	ScorePublishTime *int64   // 成绩公布时间。
	ActorID          uint64   // 操作人用户 ID，由 API 层从 session 派生。
	ActorType        string   // 操作人主体类型，由 API 层从 session 派生。
	ActorRole        string   // 操作人角色快照，由 API 层从 session 派生。
}

type UpdateExamStatusRepositoryInput struct {
	TenantID       uint64       // 所属租户 ID。
	ExamID         uint64       // 考试 ID。
	Status         string       // 目标状态。
	ExpectedStatus string       // 期望源状态；为空表示不做源状态条件更新。
	InviteCode     string       // 发布草稿时生成的邀请码；为空表示不变更。
	EndTime        *int64       // 提前结束时写入当前结束时间；nil 表示不变更。
	Log            OperationLog // 管理端操作日志。
}

type UpdateDraftWithTargetsRepositoryInput struct {
	Exam    Exam          // 更新后的草稿考试基础配置。
	Targets []Target      // 更新后的投放目标。
	Log     *OperationLog // 操作日志；nil 表示不写日志。
}

type UpdateScorePublishConfigInput struct {
	TenantID         uint64 // 所属租户 ID。
	ExamID           uint64 // 考试 ID。
	PublishMode      string // 成绩发布模式。
	ScorePublishTime *int64 // 成绩公布时间。
	ActorID          uint64 // 发布成绩的用户 ID，由 API 层从 session 派生。
	ActorType        string // 发布成绩的主体类型，由 API 层从 session 派生。
	ActorRole        string // 发布成绩的角色快照，由 API 层从 session 派生。
}

type UpdateScorePublishConfigRepositoryInput struct {
	TenantID         uint64        // 所属租户 ID。
	ExamID           uint64        // 考试 ID。
	PublishMode      string        // 成绩发布模式。
	ScorePublishTime *int64        // 成绩公布时间。
	Log              *OperationLog // 操作日志；nil 表示不写日志。
}

type AddTargetInput struct {
	TenantID   uint64 // 所属租户 ID。
	ExamID     uint64 // 考试 ID。
	TargetType string // 发布目标类型。
	TargetID   uint64 // 发布目标 ID。
}

type ResolveInviteInput struct {
	InviteCode string // 考试邀请码。
	UserID     uint64 // 当前登录用户 ID，0 表示未登录。
}

type StartInput struct {
	TenantID uint64 // 所属租户 ID。
	ExamID   uint64 // 考试 ID。
	UserID   uint64 // 考生用户 ID。
}

type StartResult struct {
	Attempt   Attempt // 作答记录。
	ExamToken string  // 明文 exam token，只返回给考生端。
}

type GenerateSnapshotInput struct {
	TenantID  uint64 // 所属租户 ID。
	ExamID    uint64 // 考试 ID。
	AttemptID uint64 // 作答 ID。
}

type ListInput struct {
	TenantID uint64  // 所属租户 ID。
	SpaceID  *uint64 // 可见空间范围；nil 表示租户管理员全租户范围。
	PaperID  *uint64 // 关联试卷过滤；nil 表示不过滤试卷。
	Search   string  // 考试名称、试卷名称、邀请码或目标空间名称搜索关键字。
	Page     int     // 页码，从 1 开始。
	PageSize int     // 每页数量。
}

// ListOperationLogsInput 表示管理端操作日志分页查询条件。
// SpaceID 为空时读取整场考试日志，非空时只读取该空间精确关联的日志。
type ListOperationLogsInput struct {
	TenantID      uint64  // 所属租户 ID。
	ExamID        uint64  // 考试 ID。
	SpaceID       *uint64 // 可见空间范围；nil 表示租户管理员全考试范围。
	OperationType string  // 操作类型筛选，空字符串表示不过滤。
	Page          int     // 页码，从 1 开始。
	PageSize      int     // 每页数量。
}

// CandidateSourceTarget 表示应考考生来自哪类考试投放目标。
// 同一考生可能同时来自空间投放和用户直投，前端据此解释名单来源，避免重复显示多行。
type CandidateSourceTarget struct {
	TargetType string // 来源目标类型：space / user。
	TargetID   uint64 // 来源目标 ID。
	SpaceID    uint64 // 来源关联空间 ID；用户直投时表示该用户当前命中的授权空间。
	SpaceName  string // 来源关联空间名称。
}

// ExamCandidate 表示考生管理 tab 的一行应考考生。
// attempt 相关字段只返回摘要，答卷详情必须通过后续答卷接口按权限单独读取。
type ExamCandidate struct {
	UserID           uint64                  // 考生用户 ID。
	Username         string                  // 登录名或学号。
	RealName         string                  // 考生姓名。
	SpaceID          uint64                  // 展示用空间 ID，多空间命中时取排序最靠前的授权空间。
	SpaceName        string                  // 展示用空间名称。
	Status           string                  // 考试状态：not_started / in_progress / submitted。
	StartedAt        *int64                  // 当前进行中作答开始时间，或结果作答开始时间。
	SubmittedAt      *int64                  // 结果作答提交时间。
	TotalScore       string                  // 结果作答总分，未提交时为空。
	AttemptCount     int                     // 当前授权范围内该考生的作答次数。
	CurrentAttemptID *uint64                 // 最新进行中作答 ID；没有进行中作答时为空。
	ResultAttemptID  *uint64                 // 按 result_strategy 选出的结果作答 ID；未提交时为空。
	SourceTargets    []CandidateSourceTarget // 去重后的名单来源。
}

// ListExamCandidatesInput 表示管理端考生列表查询条件。
// SpaceIDs 必须是详情 service 裁剪后的授权空间范围，仓储层据此做最终数据过滤。
type ListExamCandidatesInput struct {
	TenantID       uint64   // 所属租户 ID。
	ExamID         uint64   // 考试 ID。
	SpaceIDs       []uint64 // 当前管理者可见的考试投放空间范围。
	Keyword        string   // 姓名、用户名或空间名搜索关键字。
	Status         string   // 状态筛选；空字符串表示不过滤。
	ResultStrategy string   // 多次作答成绩策略。
	Page           int      // 页码，从 1 开始。
	PageSize       int      // 每页数量。
}

type Repository interface {
	ListExams(ctx context.Context, input ListInput) (pagination.Result[Exam], error)
	CreateExam(ctx context.Context, exam Exam) (Exam, error)
	GetPaper(ctx context.Context, tenantID uint64, paperID uint64) (Paper, error)
	InviteCodeExists(ctx context.Context, tenantID uint64, code string) (bool, error)
	ListRuleLiveCandidates(ctx context.Context, tenantID uint64, paperID uint64) ([]LivePoolItem, error)
	PublishExamAndFreezeLivePool(ctx context.Context, exam Exam, pool []LivePoolItem, expectedStatus string, log *OperationLog) (Exam, error)
	CreatePublishedExamWithTarget(ctx context.Context, exam Exam, pool []LivePoolItem, target Target, log *OperationLog) (Exam, error)
	CreatePublishedExamWithTargets(ctx context.Context, exam Exam, pool []LivePoolItem, targets []Target, log *OperationLog) (Exam, error)
	UpdateDraftWithTargets(ctx context.Context, input UpdateDraftWithTargetsRepositoryInput) (Exam, error)
	UpdateExamStatus(ctx context.Context, input UpdateExamStatusRepositoryInput) (Exam, error)
	TargetExists(ctx context.Context, tenantID uint64, examID uint64, targetType string, targetID uint64) (bool, error)
	AddTarget(ctx context.Context, target Target) error
	FindExamByInviteCode(ctx context.Context, inviteCode string) (Exam, error)
	GetExam(ctx context.Context, tenantID uint64, examID uint64) (Exam, error)
	IsEligible(ctx context.Context, tenantID uint64, examID uint64, userID uint64) (bool, error)
	FindInProgressAttempt(ctx context.Context, tenantID uint64, examID uint64, userID uint64) (Attempt, error)
	CountAttempts(ctx context.Context, tenantID uint64, examID uint64, userID uint64) (int, error)
	CreateAttempt(ctx context.Context, attempt Attempt) (Attempt, error)
	UpdateAttemptToken(ctx context.Context, attempt Attempt) (Attempt, error)
	FindAttemptByTokenHash(ctx context.Context, tokenHash string) (Attempt, error)
	ListFixedSnapshotQuestions(ctx context.Context, tenantID uint64, examID uint64) ([]SnapshotSourceQuestion, error)
	ListFrozenLiveSnapshotQuestions(ctx context.Context, tenantID uint64, examID uint64) ([]SnapshotSourceQuestion, error)
	SaveAttemptQuestions(ctx context.Context, questions []AttemptQuestion) ([]AttemptQuestion, error)
	UpdateScorePublishConfig(ctx context.Context, input UpdateScorePublishConfigRepositoryInput) (Exam, error)
}

type CodeGenerator interface {
	NextCode() (string, error)
}

type TokenIssuer interface {
	IssueToken() (string, error)
}

type ServiceOptions struct {
	Repo                   Repository
	CodeGenerator          CodeGenerator
	TokenIssuer            TokenIssuer
	Now                    func() int64
	ExamTokenBufferMinutes int
}

type Service struct {
	repo              Repository
	codeGenerator     CodeGenerator
	tokenIssuer       TokenIssuer
	now               func() int64
	tokenBufferMillis int64
}

func NewService(options ServiceOptions) *Service {
	now := options.Now
	if now == nil {
		now = func() int64 { return time.Now().UnixMilli() }
	}
	tokenIssuer := options.TokenIssuer
	if tokenIssuer == nil {
		tokenIssuer = randomTokenIssuer{}
	}
	tokenBufferMinutes := options.ExamTokenBufferMinutes
	if tokenBufferMinutes <= 0 {
		tokenBufferMinutes = defaultTokenBufferMinutes
	}
	return &Service{
		repo:              options.Repo,
		codeGenerator:     options.CodeGenerator,
		tokenIssuer:       tokenIssuer,
		now:               now,
		tokenBufferMillis: int64(tokenBufferMinutes) * millisPerMinute,
	}
}

func (s *Service) List(ctx context.Context, input ListInput) (pagination.Result[Exam], error) {
	return s.repo.ListExams(ctx, input)
}

func (s *Service) GetExam(ctx context.Context, tenantID uint64, examID uint64) (Exam, error) {
	return s.repo.GetExam(ctx, tenantID, examID)
}

func (s *Service) CreateDraft(ctx context.Context, input CreateDraftInput) (Exam, error) {
	return s.repo.CreateExam(ctx, Exam{
		TenantID:       input.TenantID,
		PaperID:        input.PaperID,
		Name:           input.Name,
		MaxAttempts:    1,
		ResultStrategy: ResultStrategyLatest,
		Status:         StatusDraft,
	})
}

func (s *Service) Publish(ctx context.Context, input PublishInput) (Exam, error) {
	exam, pool, err := s.buildPublishedExam(ctx, publishSettingsInput{
		ExamID:           input.ExamID,
		TenantID:         input.TenantID,
		PaperID:          input.PaperID,
		StartTime:        input.StartTime,
		EndTime:          input.EndTime,
		DurationMinutes:  input.DurationMinutes,
		MaxAttempts:      input.MaxAttempts,
		ResultStrategy:   input.ResultStrategy,
		PublishMode:      input.PublishMode,
		ScorePublishTime: input.ScorePublishTime,
	})
	if err != nil {
		return Exam{}, err
	}
	return s.repo.PublishExamAndFreezeLivePool(ctx, exam, pool, "", nil)
}

func (s *Service) PublishWithTarget(ctx context.Context, input PublishWithTargetInput) (Exam, error) {
	input.Status = StatusPublished
	return s.CreateWithTarget(ctx, input)
}

func (s *Service) CreateWithTarget(ctx context.Context, input CreateWithTargetInput) (Exam, error) {
	targets, err := normalizePublishTargets(input)
	if err != nil {
		return Exam{}, err
	}
	status := input.Status
	if status == "" {
		status = StatusPublished
	}
	if status == StatusDraft {
		exam, err := s.buildDraftExamWithSettings(ctx, publishSettingsInput{
			TenantID:         input.TenantID,
			PaperID:          input.PaperID,
			Name:             input.Name,
			StartTime:        input.StartTime,
			EndTime:          input.EndTime,
			DurationMinutes:  input.DurationMinutes,
			MaxAttempts:      input.MaxAttempts,
			ResultStrategy:   input.ResultStrategy,
			PublishMode:      input.PublishMode,
			ScorePublishTime: input.ScorePublishTime,
		})
		if err != nil {
			return Exam{}, err
		}
		exam.CreatedBy = input.ActorID
		exam.CreatedByType = input.ActorType
		created, err := s.repo.CreatePublishedExamWithTargets(ctx, exam, nil, targets, nil)
		if err != nil {
			return Exam{}, err
		}
		return hideDraftInviteCode(created), nil
	}
	if status != StatusPublished {
		return Exam{}, ErrInvalidExamStatus
	}
	exam, pool, err := s.buildPublishedExam(ctx, publishSettingsInput{
		TenantID:         input.TenantID,
		PaperID:          input.PaperID,
		Name:             input.Name,
		StartTime:        input.StartTime,
		EndTime:          input.EndTime,
		DurationMinutes:  input.DurationMinutes,
		MaxAttempts:      input.MaxAttempts,
		ResultStrategy:   input.ResultStrategy,
		PublishMode:      input.PublishMode,
		ScorePublishTime: input.ScorePublishTime,
	})
	if err != nil {
		return Exam{}, err
	}
	exam.CreatedBy = input.ActorID
	exam.CreatedByType = input.ActorType
	log := OperationLog{
		TenantID:        input.TenantID,
		OperationType:   OperationTypePublishExam,
		OperationTitle:  "发布考试",
		OperationDetail: "发布考试到 " + strconv.Itoa(len(targets)) + " 个目标",
		ActorID:         input.ActorID,
		ActorType:       input.ActorType,
		ActorRole:       input.ActorRole,
		CreatedAt:       s.now(),
		CreatedBy:       input.ActorID,
		CreatedByType:   input.ActorType,
	}
	return s.repo.CreatePublishedExamWithTargets(ctx, exam, pool, targets, &log)
}

func (s *Service) UpdateDraftWithTarget(ctx context.Context, input UpdateDraftWithTargetInput) (Exam, error) {
	targets, err := normalizePublishTargets(PublishWithTargetInput{
		TargetType: input.TargetType,
		TargetID:   input.TargetID,
		Targets:    input.Targets,
	})
	if err != nil {
		return Exam{}, err
	}
	exam, _, err := s.buildPublishableExam(ctx, publishSettingsInput{
		ExamID:           input.ExamID,
		TenantID:         input.TenantID,
		PaperID:          input.PaperID,
		Name:             input.Name,
		StartTime:        input.StartTime,
		EndTime:          input.EndTime,
		DurationMinutes:  input.DurationMinutes,
		MaxAttempts:      input.MaxAttempts,
		ResultStrategy:   input.ResultStrategy,
		PublishMode:      input.PublishMode,
		ScorePublishTime: input.ScorePublishTime,
	})
	if err != nil {
		return Exam{}, err
	}
	exam.ID = input.ExamID
	exam.Status = StatusDraft
	log := OperationLog{
		TenantID:        input.TenantID,
		ExamID:          input.ExamID,
		OperationType:   OperationTypeUpdateSettings,
		OperationTitle:  "修改草稿考试",
		OperationDetail: "修改草稿考试配置",
		ActorID:         input.ActorID,
		ActorType:       input.ActorType,
		ActorRole:       input.ActorRole,
		CreatedAt:       s.now(),
		CreatedBy:       input.ActorID,
		CreatedByType:   input.ActorType,
	}
	updated, err := s.repo.UpdateDraftWithTargets(ctx, UpdateDraftWithTargetsRepositoryInput{
		Exam:    exam,
		Targets: targets,
		Log:     &log,
	})
	if err != nil {
		return Exam{}, err
	}
	return hideDraftInviteCode(updated), nil
}

func (s *Service) UpdateStatus(ctx context.Context, input UpdateStatusInput) (Exam, error) {
	switch input.Status {
	case StatusPublished:
		exam, err := s.repo.GetExam(ctx, input.TenantID, input.ExamID)
		if err != nil {
			return Exam{}, err
		}
		if exam.Status == StatusPublished {
			return exam, nil
		}
		if exam.Status != StatusDraft {
			return Exam{}, ErrInvalidExamStatus
		}
		if len(exam.Targets) == 0 {
			return Exam{}, ErrExamTargetRequired
		}
		published, pool, err := s.buildPublishedExam(ctx, publishSettingsInput{
			ExamID:           exam.ID,
			TenantID:         exam.TenantID,
			PaperID:          exam.PaperID,
			Name:             exam.Name,
			StartTime:        exam.StartTime,
			EndTime:          exam.EndTime,
			DurationMinutes:  exam.DurationMinutes,
			MaxAttempts:      exam.MaxAttempts,
			ResultStrategy:   exam.ResultStrategy,
			PublishMode:      exam.PublishMode,
			ScorePublishTime: exam.ScorePublishTime,
		})
		if err != nil {
			return Exam{}, err
		}
		log := OperationLog{
			TenantID:        input.TenantID,
			ExamID:          input.ExamID,
			OperationType:   OperationTypePublishExam,
			OperationTitle:  "发布考试",
			OperationDetail: "发布草稿考试",
			ActorID:         input.ActorID,
			ActorType:       input.ActorType,
			ActorRole:       input.ActorRole,
			CreatedAt:       s.now(),
			CreatedBy:       input.ActorID,
			CreatedByType:   input.ActorType,
		}
		return s.repo.PublishExamAndFreezeLivePool(ctx, published, pool, StatusDraft, &log)
	case StatusClosed:
		exam, err := s.repo.GetExam(ctx, input.TenantID, input.ExamID)
		if err != nil {
			return Exam{}, err
		}
		if exam.Status != StatusPublished {
			return Exam{}, ErrInvalidExamStatus
		}
		endTime := s.now()
		return s.repo.UpdateExamStatus(ctx, UpdateExamStatusRepositoryInput{
			TenantID:       input.TenantID,
			ExamID:         input.ExamID,
			Status:         StatusClosed,
			ExpectedStatus: StatusPublished,
			EndTime:        &endTime,
			Log:            s.statusOperationLog(input, "提前结束考试"),
		})
	case StatusDisabled:
		exam, err := s.repo.GetExam(ctx, input.TenantID, input.ExamID)
		if err != nil {
			return Exam{}, err
		}
		if exam.Status == StatusDisabled {
			return exam, nil
		}
		if exam.Status != StatusPublished {
			return Exam{}, ErrInvalidExamStatus
		}
		return s.repo.UpdateExamStatus(ctx, UpdateExamStatusRepositoryInput{
			TenantID:       input.TenantID,
			ExamID:         input.ExamID,
			Status:         StatusDisabled,
			ExpectedStatus: StatusPublished,
			Log:            s.statusOperationLog(input, "禁用考试"),
		})
	default:
		return Exam{}, ErrInvalidExamStatus
	}
}

// normalizePublishTargets 归一化发布目标，保证 service 之后只处理去重后的多目标列表。
// 旧客户端仍可继续传 TargetType 和 TargetID，新客户端传 Targets 时会完全覆盖旧字段。
func normalizePublishTargets(input PublishWithTargetInput) ([]Target, error) {
	candidates := input.Targets
	if len(candidates) == 0 && input.TargetType != "" && input.TargetID != 0 {
		candidates = []Target{{TargetType: input.TargetType, TargetID: input.TargetID}}
	}
	if len(candidates) == 0 {
		return nil, ErrExamTargetRequired
	}

	seen := make(map[string]struct{}, len(candidates))
	targets := make([]Target, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.TargetType == "" || candidate.TargetID == 0 {
			return nil, ErrExamTargetRequired
		}
		key := candidate.TargetType + ":" + strconv.FormatUint(candidate.TargetID, 10)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		targets = append(targets, Target{
			TenantID:      input.TenantID,
			TargetType:    candidate.TargetType,
			TargetID:      candidate.TargetID,
			ScopeSpaceIDs: append([]uint64(nil), candidate.ScopeSpaceIDs...),
		})
	}
	return targets, nil
}

func (s *Service) statusOperationLog(input UpdateStatusInput, title string) OperationLog {
	return OperationLog{
		TenantID:        input.TenantID,
		ExamID:          input.ExamID,
		OperationType:   OperationTypeUpdateSettings,
		OperationTitle:  title,
		OperationDetail: title,
		ActorID:         input.ActorID,
		ActorType:       input.ActorType,
		ActorRole:       input.ActorRole,
		CreatedAt:       s.now(),
		CreatedBy:       input.ActorID,
		CreatedByType:   input.ActorType,
	}
}

type publishSettingsInput struct {
	ExamID           uint64
	TenantID         uint64
	PaperID          uint64
	Name             string
	StartTime        int64
	EndTime          int64
	DurationMinutes  int
	MaxAttempts      int
	ResultStrategy   string
	PublishMode      string
	ScorePublishTime *int64
}

func (s *Service) buildPublishedExam(ctx context.Context, input publishSettingsInput) (Exam, []LivePoolItem, error) {
	exam, paper, err := s.buildPublishableExam(ctx, input)
	if err != nil {
		return Exam{}, nil, err
	}
	inviteCode, err := s.nextUniqueInviteCode(ctx, input.TenantID)
	if err != nil {
		return Exam{}, nil, err
	}
	exam.InviteCode = inviteCode
	exam.Status = StatusPublished
	var pool []LivePoolItem
	if paper.BuildMode == BuildModeRuleLive {
		pool, err = s.repo.ListRuleLiveCandidates(ctx, input.TenantID, input.PaperID)
		if err != nil {
			return Exam{}, nil, err
		}
	}
	return exam, pool, nil
}

func (s *Service) buildDraftExamWithSettings(ctx context.Context, input publishSettingsInput) (Exam, error) {
	exam, _, err := s.buildPublishableExam(ctx, input)
	if err != nil {
		return Exam{}, err
	}
	inviteCode, err := s.nextDraftInviteCode(ctx, input.TenantID)
	if err != nil {
		return Exam{}, err
	}
	// 数据库对 invite_code 仍有全局唯一约束；草稿使用内部占位码入库，对外保持空邀请码语义。
	exam.InviteCode = inviteCode
	exam.Status = StatusDraft
	return exam, nil
}

func hideDraftInviteCode(exam Exam) Exam {
	if exam.Status == StatusDraft {
		exam.InviteCode = ""
	}
	return exam
}

func (s *Service) buildPublishableExam(ctx context.Context, input publishSettingsInput) (Exam, Paper, error) {
	if int64(input.DurationMinutes)*millisPerMinute > input.EndTime-input.StartTime {
		return Exam{}, Paper{}, ErrDurationExceedsExamWindow
	}
	paper, err := s.repo.GetPaper(ctx, input.TenantID, input.PaperID)
	if err != nil {
		return Exam{}, Paper{}, err
	}
	if paper.Status != constant.PaperStatusEnabled {
		return Exam{}, Paper{}, ErrPaperNotEnabled
	}
	if paper.ContainsShortText && input.MaxAttempts != 1 {
		return Exam{}, Paper{}, ErrShortTextCannotRepeatAttempt
	}
	if paper.ContainsShortText && input.PublishMode == PublishModeImmediateScore {
		return Exam{}, Paper{}, ErrShortTextCannotImmediateScore
	}
	exam := Exam{
		ID:               input.ExamID,
		TenantID:         input.TenantID,
		PaperID:          input.PaperID,
		Name:             input.Name,
		StartTime:        input.StartTime,
		EndTime:          input.EndTime,
		DurationMinutes:  input.DurationMinutes,
		MaxAttempts:      input.MaxAttempts,
		ResultStrategy:   input.ResultStrategy,
		PublishMode:      input.PublishMode,
		ScorePublishTime: input.ScorePublishTime,
		BuildMode:        paper.BuildMode,
	}
	return exam, paper, nil
}

func (s *Service) AddTarget(ctx context.Context, input AddTargetInput) error {
	exists, err := s.repo.TargetExists(ctx, input.TenantID, input.ExamID, input.TargetType, input.TargetID)
	if err != nil {
		return err
	}
	if exists {
		return ErrDuplicateExamTarget
	}
	return s.repo.AddTarget(ctx, Target{
		TenantID:   input.TenantID,
		ExamID:     input.ExamID,
		TargetType: input.TargetType,
		TargetID:   input.TargetID,
	})
}

func (s *Service) ResolveInvite(ctx context.Context, input ResolveInviteInput) (Exam, error) {
	if input.UserID == 0 {
		return Exam{}, ErrLoginRequiredForInvite
	}
	exam, err := s.repo.FindExamByInviteCode(ctx, input.InviteCode)
	if err != nil {
		return Exam{}, err
	}
	if exam.Status != StatusPublished {
		return Exam{}, ErrExamNotEligible
	}
	return exam, nil
}

func (s *Service) StartExam(ctx context.Context, input StartInput) (StartResult, error) {
	exam, err := s.repo.GetExam(ctx, input.TenantID, input.ExamID)
	if err != nil {
		return StartResult{}, err
	}
	if exam.Status != StatusPublished {
		return StartResult{}, ErrExamNotEligible
	}
	eligible, err := s.repo.IsEligible(ctx, input.TenantID, input.ExamID, input.UserID)
	if err != nil {
		return StartResult{}, err
	}
	if !eligible {
		return StartResult{}, ErrExamNotEligible
	}
	now := s.now()
	if now < exam.StartTime {
		return StartResult{}, ErrExamNotStarted
	}
	if now > exam.EndTime {
		return StartResult{}, ErrExamEnded
	}
	if attempt, err := s.repo.FindInProgressAttempt(ctx, input.TenantID, input.ExamID, input.UserID); err == nil {
		return s.refreshAttemptToken(ctx, attempt, exam)
	} else if !errors.Is(err, ErrAttemptNotFound) {
		return StartResult{}, err
	}
	count, err := s.repo.CountAttempts(ctx, input.TenantID, input.ExamID, input.UserID)
	if err != nil {
		return StartResult{}, err
	}
	if exam.MaxAttempts > 0 && count >= exam.MaxAttempts {
		return StartResult{}, ErrMaxAttemptsReached
	}
	token, err := s.tokenIssuer.IssueToken()
	if err != nil {
		return StartResult{}, err
	}
	attempt := Attempt{
		TenantID:           input.TenantID,
		ExamID:             input.ExamID,
		UserID:             input.UserID,
		AttemptNo:          count + 1,
		Status:             AttemptStatusInProgress,
		StartedAt:          now,
		ExamTokenHash:      s.HashExamToken(token),
		ExamTokenExpiresAt: answerDeadline(now, exam) + s.tokenBufferMillis,
	}
	attempt, err = s.repo.CreateAttempt(ctx, attempt)
	if errors.Is(err, ErrAttemptUniqueConflict) {
		existing, findErr := s.repo.FindInProgressAttempt(ctx, input.TenantID, input.ExamID, input.UserID)
		if findErr != nil {
			return StartResult{}, findErr
		}
		return s.refreshAttemptToken(ctx, existing, exam)
	}
	if err != nil {
		return StartResult{}, err
	}
	return StartResult{Attempt: attempt, ExamToken: token}, nil
}

func (s *Service) refreshAttemptToken(ctx context.Context, attempt Attempt, exam Exam) (StartResult, error) {
	token, err := s.tokenIssuer.IssueToken()
	if err != nil {
		return StartResult{}, err
	}
	attempt.ExamTokenHash = s.HashExamToken(token)
	attempt.ExamTokenExpiresAt = answerDeadline(attempt.StartedAt, exam) + s.tokenBufferMillis
	updated, err := s.repo.UpdateAttemptToken(ctx, attempt)
	if err != nil {
		return StartResult{}, err
	}
	return StartResult{Attempt: updated, ExamToken: token}, nil
}

func (s *Service) GenerateAttemptSnapshots(ctx context.Context, input GenerateSnapshotInput) ([]AttemptQuestion, error) {
	exam, err := s.repo.GetExam(ctx, input.TenantID, input.ExamID)
	if err != nil {
		return nil, err
	}
	var sources []SnapshotSourceQuestion
	if exam.BuildMode == BuildModeRuleLive {
		sources, err = s.repo.ListFrozenLiveSnapshotQuestions(ctx, input.TenantID, input.ExamID)
	} else {
		sources, err = s.repo.ListFixedSnapshotQuestions(ctx, input.TenantID, input.ExamID)
	}
	if err != nil {
		return nil, err
	}
	snapshots := make([]AttemptQuestion, 0, len(sources))
	for index, source := range sources {
		snapshots = append(snapshots, AttemptQuestion{
			TenantID:              input.TenantID,
			AttemptID:             input.AttemptID,
			SectionID:             source.SectionID,
			QuestionID:            source.QuestionID,
			SectionSnapshot:       mustJSON(map[string]any{"name": source.SectionName, "instructions": source.Instructions}),
			SortOrder:             index + 1,
			Score:                 source.Score,
			QuestionSnapshot:      mustJSON(map[string]any{"title": source.Title, "type": source.QuestionType, "blank_count": source.BlankCount}),
			OptionSnapshot:        mustJSON(map[string]any{"option_ids": source.OptionIDs, "options": source.Options}),
			CorrectAnswerSnapshot: mustJSON(map[string]any{"option_ids": source.CorrectOptionIDs, "text": source.CorrectText}),
		})
	}
	return snapshots, nil
}

func (s *Service) SaveAttemptQuestions(ctx context.Context, questions []AttemptQuestion) ([]AttemptQuestion, error) {
	return s.repo.SaveAttemptQuestions(ctx, questions)
}

func (s *Service) UpdateScorePublishConfig(ctx context.Context, input UpdateScorePublishConfigInput) (Exam, error) {
	log := OperationLog{
		TenantID:        input.TenantID,
		ExamID:          input.ExamID,
		OperationType:   OperationTypePublishResults,
		OperationTitle:  "发布成绩",
		OperationDetail: "更新成绩发布配置",
		ActorID:         input.ActorID,
		ActorType:       input.ActorType,
		ActorRole:       input.ActorRole,
		CreatedAt:       s.now(),
		CreatedBy:       input.ActorID,
		CreatedByType:   input.ActorType,
	}
	return s.repo.UpdateScorePublishConfig(ctx, UpdateScorePublishConfigRepositoryInput{
		TenantID:         input.TenantID,
		ExamID:           input.ExamID,
		PublishMode:      input.PublishMode,
		ScorePublishTime: input.ScorePublishTime,
		Log:              &log,
	})
}

func (s *Service) ValidateExamToken(ctx context.Context, token string, attemptID uint64) (Attempt, error) {
	attempt, err := s.repo.FindAttemptByTokenHash(ctx, HashExamToken(token))
	if err != nil {
		return Attempt{}, ErrExamTokenInvalid
	}
	if attempt.ID != attemptID {
		return Attempt{}, ErrExamTokenAttemptMismatch
	}
	if attempt.Status != AttemptStatusInProgress || attempt.ExamTokenExpiresAt < s.now() {
		return Attempt{}, ErrExamTokenInvalid
	}
	return attempt, nil
}

func (s *Service) HashExamToken(token string) string {
	return HashExamToken(token)
}

func HashExamToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *Service) nextUniqueInviteCode(ctx context.Context, tenantID uint64) (string, error) {
	generator := s.codeGenerator
	if generator == nil {
		generator = randomCodeGenerator{}
	}
	for range 8 {
		code, err := generator.NextCode()
		if err != nil {
			return "", err
		}
		exists, err := s.repo.InviteCodeExists(ctx, tenantID, code)
		if err != nil {
			return "", err
		}
		if !exists {
			return code, nil
		}
	}
	return "", ErrInviteCodeCollision
}

func (s *Service) nextDraftInviteCode(ctx context.Context, tenantID uint64) (string, error) {
	for range 8 {
		buffer := make([]byte, 16)
		if _, err := io.ReadFull(rand.Reader, buffer); err != nil {
			return "", err
		}
		code := "__draft__:" + hex.EncodeToString(buffer)
		exists, err := s.repo.InviteCodeExists(ctx, tenantID, code)
		if err != nil {
			return "", err
		}
		if !exists {
			return code, nil
		}
	}
	return "", ErrInviteCodeCollision
}

func answerDeadline(startedAt int64, exam Exam) int64 {
	durationDeadline := startedAt + int64(exam.DurationMinutes)*millisPerMinute
	if durationDeadline < exam.EndTime {
		return durationDeadline
	}
	return exam.EndTime
}

func mustJSON(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}

type randomTokenIssuer struct{}

func (randomTokenIssuer) IssueToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

type randomCodeGenerator struct{}

func (randomCodeGenerator) NextCode() (string, error) {
	buffer := make([]byte, 6)
	if _, err := io.ReadFull(rand.Reader, buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

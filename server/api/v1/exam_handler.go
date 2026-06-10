package v1

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	serviceexam "github.com/lifei6671/papermind/server/internal/service/exam"
	"github.com/lifei6671/papermind/server/internal/service/permission"
	servicespace "github.com/lifei6671/papermind/server/internal/service/space"
	servicetenantuser "github.com/lifei6671/papermind/server/internal/service/tenantuser"
	"github.com/lifei6671/papermind/server/library/code"
	"github.com/lifei6671/papermind/server/library/constant"
	"github.com/lifei6671/papermind/server/library/response"
	"gorm.io/gorm"
)

// examHandler 承载考试发布、考试入口、阅卷和成绩相关的 HTTP 编排。
//
// handler 只负责请求解析、认证上下文转换和响应映射；考试状态流转、
// token 校验、阅卷和成绩规则都继续下沉到 service 层。
type examHandler struct {
	service              *serviceexam.Service
	management           *serviceexam.ManagementDetailService
	taking               *serviceexam.TakingService
	review               *serviceexam.ReviewService
	export               *serviceexam.ExportService
	result               *serviceexam.ResultService
	papers               paperScopeFinder
	targets              examTargetFinder
	members              spaceMemberFinder
	tenantUsers          *servicetenantuser.Service
	sessionMaxAgeSeconds int
	now                  func() int64
}

// spaceMemberFinder 抽象空间成员查询能力，供权限上下文动态反查空间角色。
//
// 注意：space_admin 不能来自 session role，必须从当前数据库成员关系读取。
type spaceMemberFinder interface {
	FindMember(ctx context.Context, tenantID uint64, spaceID uint64, userID uint64) (servicespace.Member, error)
	ListEffectiveMembershipsForUser(ctx context.Context, tenantID uint64, userID uint64) ([]servicespace.Member, error)
	SpaceExists(ctx context.Context, tenantID uint64, spaceID uint64) (bool, error)
}

type examTargetFinder interface {
	ListTargets(ctx context.Context, tenantID uint64, examID uint64) ([]serviceexam.Target, error)
}

// publishExamTargetRequest 是发布考试时单个投放目标的 HTTP 请求结构。
//
// 多目标发布只允许显式投放到空间或用户，后续权限校验会逐个目标确认当前
// 操作者是否有权把考试投放到对应范围。
type publishExamTargetRequest struct {
	TargetType    string   `json:"target_type"`
	TargetID      uint64   `json:"target_id"`
	ScopeSpaceIDs []uint64 `json:"scope_space_ids"`
}

type publishExamRequest struct {
	TenantID         uint64                     `json:"tenant_id"`
	PaperID          uint64                     `json:"paper_id"`
	Name             string                     `json:"name"`
	TargetType       string                     `json:"target_type"`
	TargetID         uint64                     `json:"target_id"`
	Targets          []publishExamTargetRequest `json:"targets"`
	StartTime        int64                      `json:"start_time"`
	EndTime          int64                      `json:"end_time"`
	DurationMinutes  int                        `json:"duration_minutes"`
	MaxAttempts      int                        `json:"max_attempts"`
	ResultStrategy   string                     `json:"result_strategy"`
	PublishMode      string                     `json:"publish_mode"`
	ScorePublishTime *int64                     `json:"score_publish_time"`
	Status           string                     `json:"status"`
}

type examStatusRequest struct {
	TenantID uint64 `json:"tenant_id"`
	Status   string `json:"status"`
}

type resolveExamInviteRequest struct {
	InviteCode string `json:"invite_code"`
	UserID     uint64 `json:"user_id"`
}

type startAttemptRequest struct {
	TenantID uint64 `json:"tenant_id"`
	UserID   uint64 `json:"user_id"`
}

type saveAnswerRequest struct {
	TenantID     uint64   `json:"tenant_id"`
	ExamToken    string   `json:"exam_token"`
	QuestionType string   `json:"question_type"`
	OptionIDs    []uint64 `json:"option_ids"`
	Text         string   `json:"text"`
}

type submitAttemptRequest struct {
	TenantID  uint64 `json:"tenant_id"`
	ExamToken string `json:"exam_token"`
	EventType string `json:"event_type"`
}

type recordExamEventRequest struct {
	TenantID  uint64 `json:"tenant_id"`
	ExamToken string `json:"exam_token"`
	EventType string `json:"event_type"`
	Payload   string `json:"payload"`
}

type importCandidatesRequest struct {
	TenantID uint64   `json:"tenant_id"`
	SpaceID  uint64   `json:"space_id"`
	UserIDs  []uint64 `json:"user_ids"`
}

type resendInvitationsRequest struct {
	TenantID uint64   `json:"tenant_id"`
	SpaceID  uint64   `json:"space_id"`
	UserIDs  []uint64 `json:"user_ids"`
}

type actorPermissionRequest struct {
	ActorID   uint64 `json:"actor_id"`
	ActorRole string `json:"actor_role"`
	SpaceID   uint64 `json:"space_id"`
}

type gradeShortTextRequest struct {
	actorPermissionRequest
	TenantID      uint64 `json:"tenant_id"`
	ExamID        uint64 `json:"exam_id"`
	AnswerVersion int64  `json:"answer_version"`
	Score         string `json:"score"`
	Comment       string `json:"comment"`
}

type saveResultPublishConfigRequest struct {
	actorPermissionRequest
	TenantID         uint64 `json:"tenant_id"`
	ExamID           uint64 `json:"exam_id"`
	PublishMode      string `json:"publish_mode"`
	ScorePublishTime *int64 `json:"score_publish_time"`
}

type managementSettingsRequest struct {
	TenantID         uint64 `json:"tenant_id"`
	SpaceID          uint64 `json:"space_id"`
	PublishMode      string `json:"publish_mode"`
	ScorePublishTime *int64 `json:"score_publish_time"`
}

type exportResultsRequest struct {
	actorPermissionRequest
	TenantID uint64 `json:"tenant_id"`
	ExamID   uint64 `json:"exam_id"`
}

type examResponse struct {
	ID               uint64                         `json:"id"`
	TenantID         uint64                         `json:"tenant_id"`
	PaperID          uint64                         `json:"paper_id"`
	Name             string                         `json:"name"`
	StartTime        int64                          `json:"start_time"`
	EndTime          int64                          `json:"end_time"`
	DurationMinutes  int                            `json:"duration_minutes"`
	MaxAttempts      int                            `json:"max_attempts"`
	ResultStrategy   string                         `json:"result_strategy"`
	PublishMode      string                         `json:"publish_mode"`
	ScorePublishTime *int64                         `json:"score_publish_time"`
	InviteCode       string                         `json:"invite_code"`
	Status           string                         `json:"status"`
	CreatedBy        uint64                         `json:"created_by"`
	TargetType       string                         `json:"target_type,omitempty"`
	TargetID         uint64                         `json:"target_id,omitempty"`
	Targets          []examManagementTargetResponse `json:"targets,omitempty"`
}

type examManagementTargetResponse struct {
	TargetType    string   `json:"target_type"`
	TargetID      uint64   `json:"target_id"`
	ScopeSpaceIDs []uint64 `json:"scope_space_ids,omitempty"`
}

type examManagementPermissionsResponse struct {
	CanViewDetail       bool `json:"can_view_detail"`
	CanViewOverview     bool `json:"can_view_overview"`
	CanViewPaper        bool `json:"can_view_paper"`
	CanViewCandidates   bool `json:"can_view_candidates"`
	CanManageCandidates bool `json:"can_manage_candidates"`
	CanViewResults      bool `json:"can_view_results"`
	CanExportResults    bool `json:"can_export_results"`
	CanPublishResults   bool `json:"can_publish_results"`
	CanUpdateSettings   bool `json:"can_update_settings"`
	CanViewLogs         bool `json:"can_view_logs"`
}

type examManagementDetailResponse struct {
	Exam            examResponse                      `json:"exam"`
	Targets         []examManagementTargetResponse    `json:"targets"`
	TargetSpaceIDs  []uint64                          `json:"target_space_ids"`
	AllowedSpaceIDs []uint64                          `json:"allowed_space_ids"`
	Permissions     examManagementPermissionsResponse `json:"permissions"`
}

// examOverviewCandidateStatsResponse 是考试概览中的考生进度统计。
type examOverviewCandidateStatsResponse struct {
	Planned    int `json:"planned"`
	Joined     int `json:"joined"`
	Submitted  int `json:"submitted"`
	InProgress int `json:"in_progress"`
}

// examOverviewQuestionTypeResponse 是考试概览中的题型分布。
type examOverviewQuestionTypeResponse struct {
	QuestionType  string `json:"question_type"`
	QuestionCount int    `json:"question_count"`
	TotalScore    string `json:"total_score"`
}

// examOverviewActivityResponse 是考试概览中的近期动态。
type examOverviewActivityResponse struct {
	OperationType   string `json:"operation_type"`
	OperationTitle  string `json:"operation_title"`
	OperationDetail string `json:"operation_detail"`
	CreatedAt       int64  `json:"created_at"`
}

// examOverviewResponse 是考试概览接口响应。
type examOverviewResponse struct {
	Exam             examResponse                       `json:"exam"`
	CandidateStats   examOverviewCandidateStatsResponse `json:"candidate_stats"`
	QuestionTypes    []examOverviewQuestionTypeResponse `json:"question_types"`
	RecentActivities []examOverviewActivityResponse     `json:"recent_activities"`
	Permissions      examManagementPermissionsResponse  `json:"permissions"`
}

// examPaperPreviewOptionResponse 是试卷预览题目选项，不包含正确答案标记。
type examPaperPreviewOptionResponse struct {
	ID      uint64 `json:"id"`
	Key     string `json:"key"`
	Content string `json:"content"`
}

// examPaperPreviewQuestionResponse 是试卷预览题目，不包含标准答案。
type examPaperPreviewQuestionResponse struct {
	SectionID    uint64                           `json:"section_id"`
	SectionName  string                           `json:"section_name"`
	QuestionID   uint64                           `json:"question_id"`
	QuestionType string                           `json:"question_type"`
	Title        string                           `json:"title"`
	Score        string                           `json:"score"`
	BlankCount   int                              `json:"blank_count"`
	SortOrder    int                              `json:"sort_order"`
	Options      []examPaperPreviewOptionResponse `json:"options"`
}

// examPaperPreviewSectionResponse 是试卷预览左侧结构摘要。
type examPaperPreviewSectionResponse struct {
	SectionID     uint64 `json:"section_id"`
	SectionName   string `json:"section_name"`
	QuestionType  string `json:"question_type"`
	QuestionCount int    `json:"question_count"`
	TotalScore    string `json:"total_score"`
}

// examPaperPreviewResponse 是试卷预览接口响应。
type examPaperPreviewResponse struct {
	Exam        examResponse                       `json:"exam"`
	Sections    []examPaperPreviewSectionResponse  `json:"sections"`
	Items       []examPaperPreviewQuestionResponse `json:"items"`
	Page        int                                `json:"page"`
	PageSize    int                                `json:"page_size"`
	Total       int                                `json:"total"`
	Permissions examManagementPermissionsResponse  `json:"permissions"`
}

type examCandidateSourceTargetResponse struct {
	TargetType string `json:"target_type"` // 来源目标类型：space / user。
	TargetID   uint64 `json:"target_id"`   // 来源目标 ID。
	SpaceID    uint64 `json:"space_id"`    // 来源命中的授权空间 ID。
	SpaceName  string `json:"space_name"`  // 来源命中的授权空间名称。
}

type examCandidateResponse struct {
	UserID           uint64                              `json:"user_id"`            // 考生用户 ID。
	Username         string                              `json:"username"`           // 登录名或学号。
	RealName         string                              `json:"real_name"`          // 考生姓名。
	SpaceID          uint64                              `json:"space_id"`           // 展示用空间 ID。
	SpaceName        string                              `json:"space_name"`         // 展示用空间名称。
	Status           string                              `json:"status"`             // 考试状态：not_started / in_progress / submitted。
	StartedAt        *int64                              `json:"started_at"`         // 当前作答或结果作答开始时间。
	SubmittedAt      *int64                              `json:"submitted_at"`       // 结果作答提交时间。
	TotalScore       string                              `json:"total_score"`        // 结果作答总分。
	AttemptCount     int                                 `json:"attempt_count"`      // 该考生作答次数。
	CurrentAttemptID *uint64                             `json:"current_attempt_id"` // 最新进行中作答 ID。
	ResultAttemptID  *uint64                             `json:"result_attempt_id"`  // 按成绩策略选出的结果作答 ID。
	SourceTargets    []examCandidateSourceTargetResponse `json:"source_targets"`     // 去重后的名单来源。
}

// examCandidateListResponse 是考生管理 tab 的分页响应。
// 权限结果随列表返回，前端据此控制导入和重发邀请按钮，但写接口仍必须再次后端校验。
type examCandidateListResponse struct {
	Exam        examResponse                      `json:"exam"`
	Items       []examCandidateResponse           `json:"items"`
	Page        int                               `json:"page"`
	PageSize    int                               `json:"page_size"`
	Total       int64                             `json:"total"`
	Permissions examManagementPermissionsResponse `json:"permissions"`
}

// examCandidateImportResponse 是导入考生接口响应。
// imported_count 表示真实新增的用户直投目标数量，skipped_count 表示重复、无效或越权目标数量。
type examCandidateImportResponse struct {
	ImportedCount int                               `json:"imported_count"`
	SkippedCount  int                               `json:"skipped_count"`
	Permissions   examManagementPermissionsResponse `json:"permissions"`
}

// examInvitationResendResponse 是重发邀请码接口响应。
// invite_code 由后端返回，当前版本无短信/邮件通道，前端可据此提示或复制给目标考生。
type examInvitationResendResponse struct {
	SentCount    int                               `json:"sent_count"`
	SkippedCount int                               `json:"skipped_count"`
	InviteCode   string                            `json:"invite_code"`
	Permissions  examManagementPermissionsResponse `json:"permissions"`
}

// examResultSummaryStatsResponse 是成绩管理摘要统计卡片响应。
type examResultSummaryStatsResponse struct {
	Submitted         int    `json:"submitted"`
	AverageScore      string `json:"average_score"`
	HighestScore      string `json:"highest_score"`
	PassRate          string `json:"pass_rate"`
	PendingSubjective int    `json:"pending_subjective"`
}

// examResultScoreDistributionResponse 是成绩管理分数段分布响应。
type examResultScoreDistributionResponse struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

// examResultQuestionTypeRateResponse 是成绩管理题型平均得分率响应。
type examResultQuestionTypeRateResponse struct {
	QuestionType      string `json:"question_type"`
	QuestionTypeLabel string `json:"question_type_label"`
	AverageRate       int    `json:"average_rate"`
}

// examResultSummaryResponse 是成绩管理摘要接口响应。
type examResultSummaryResponse struct {
	Exam              examResponse                          `json:"exam"`
	Stats             examResultSummaryStatsResponse        `json:"stats"`
	ScoreDistribution []examResultScoreDistributionResponse `json:"score_distribution"`
	QuestionTypeRates []examResultQuestionTypeRateResponse  `json:"question_type_rates"`
	Permissions       examManagementPermissionsResponse     `json:"permissions"`
}

// examManagementResultResponse 是成绩管理列表单行响应。
type examManagementResultResponse struct {
	Rank            uint64 `json:"rank"`
	AttemptID       uint64 `json:"attempt_id"`
	UserID          uint64 `json:"user_id"`
	Username        string `json:"username"`
	RealName        string `json:"real_name"`
	SpaceID         uint64 `json:"space_id"`
	SpaceName       string `json:"space_name"`
	ObjectiveScore  string `json:"objective_score"`
	SubjectiveScore string `json:"subjective_score"`
	TotalScore      string `json:"total_score"`
	Status          string `json:"status"`
	SubmittedAt     int64  `json:"submitted_at"`
}

// examManagementResultListResponse 是成绩管理列表分页响应。
type examManagementResultListResponse struct {
	Exam        examResponse                      `json:"exam"`
	Items       []examManagementResultResponse    `json:"items"`
	Page        int                               `json:"page"`
	PageSize    int                               `json:"page_size"`
	Total       int64                             `json:"total"`
	Permissions examManagementPermissionsResponse `json:"permissions"`
}

// examAnswerSheetAttemptResponse 是管理端答卷详情的作答摘要。
type examAnswerSheetAttemptResponse struct {
	AttemptID       uint64 `json:"attempt_id"`
	ExamID          uint64 `json:"exam_id"`
	UserID          uint64 `json:"user_id"`
	Username        string `json:"username"`
	RealName        string `json:"real_name"`
	ObjectiveScore  string `json:"objective_score"`
	SubjectiveScore string `json:"subjective_score"`
	TotalScore      string `json:"total_score"`
	SubmittedAt     int64  `json:"submitted_at"`
}

// examAnswerSheetItemResponse 是管理端答卷详情的单题记录。
type examAnswerSheetItemResponse struct {
	AttemptQuestionID     uint64 `json:"attempt_question_id"`
	SectionID             uint64 `json:"section_id"`
	QuestionID            uint64 `json:"question_id"`
	SortOrder             int    `json:"sort_order"`
	SectionSnapshot       string `json:"section_snapshot"`
	QuestionSnapshot      string `json:"question_snapshot"`
	QuestionType          string `json:"question_type"`
	QuestionTitle         string `json:"question_title"`
	OptionSnapshot        string `json:"option_snapshot"`
	CorrectAnswerSnapshot string `json:"correct_answer_snapshot"`
	Score                 string `json:"score"`
	AnswerContent         string `json:"answer_content"`
	AnswerScore           string `json:"answer_score"`
	GradingStatus         string `json:"grading_status"`
	GraderComment         string `json:"grader_comment"`
	GradedBy              uint64 `json:"graded_by"`
	GradedAt              *int64 `json:"graded_at"`
}

// examAnswerSheetResponse 是管理端答卷详情接口响应。
type examAnswerSheetResponse struct {
	Exam        examResponse                      `json:"exam"`
	Attempt     examAnswerSheetAttemptResponse    `json:"attempt"`
	Items       []examAnswerSheetItemResponse     `json:"items"`
	Permissions examManagementPermissionsResponse `json:"permissions"`
}

type examOperationLogResponse struct {
	ID               uint64  `json:"id"`
	OperationType    string  `json:"operation_type"`
	OperationTitle   string  `json:"operation_title"`
	OperationDetail  string  `json:"operation_detail"`
	ActorID          uint64  `json:"actor_id"`
	ActorType        string  `json:"actor_type"`
	ActorRole        string  `json:"actor_role"`
	OperationGroupID string  `json:"operation_group_id"`
	SpaceID          *uint64 `json:"space_id"`
	CreatedAt        int64   `json:"created_at"`
}

type examOperationLogListResponse struct {
	Exam        examResponse                      `json:"exam"`
	Items       []examOperationLogResponse        `json:"items"`
	Page        int                               `json:"page"`
	PageSize    int                               `json:"page_size"`
	Total       int64                             `json:"total"`
	Permissions examManagementPermissionsResponse `json:"permissions"`
}

type examInviteResponse = examResponse

type startAttemptResponse struct {
	Attempt   attemptResponse           `json:"attempt"`
	ExamToken string                    `json:"exam_token"`
	Questions []attemptQuestionResponse `json:"questions"`
}

type attemptResponse struct {
	ID             uint64 `json:"id"`
	ExamID         uint64 `json:"exam_id"`
	UserID         uint64 `json:"user_id"`
	AttemptNo      int    `json:"attempt_no"`
	Status         string `json:"status"`
	StartedAt      int64  `json:"started_at"`
	AnswerDeadline int64  `json:"answer_deadline"`
}

type attemptQuestionResponse struct {
	ID        uint64                   `json:"id"`
	SortOrder int                      `json:"sort_order"`
	Section   sectionSnapshotResponse  `json:"section"`
	Question  questionSnapshotResponse `json:"question"`
	Options   []optionSnapshotResponse `json:"options"`
	Score     string                   `json:"score"`
}

type sectionSnapshotResponse struct {
	Name         string `json:"name"`
	Instructions string `json:"instructions"`
}

type questionSnapshotResponse struct {
	Title      string `json:"title"`
	Type       string `json:"type"`
	BlankCount int    `json:"blank_count,omitempty"`
}

type optionSnapshotResponse struct {
	ID      uint64 `json:"id"`
	Key     string `json:"key"`
	Content string `json:"content"`
}

type answerSavedResponse struct {
	Saved     bool  `json:"saved"`
	UpdatedAt int64 `json:"updated_at"`
}

type submitAttemptResponse struct {
	Submitted bool `json:"submitted"`
}

type examEventResponse struct {
	Recorded bool `json:"recorded"`
}

type pendingReviewResponse struct {
	AttemptID             uint64 `json:"attempt_id"`
	AttemptQuestionID     uint64 `json:"attempt_question_id"`
	StudentName           string `json:"student_name"`
	SpaceName             string `json:"space_name"`
	ExamName              string `json:"exam_name"`
	QuestionTitle         string `json:"question_title"`
	AnswerContent         string `json:"answer_content"`
	SubmittedAt           int64  `json:"submitted_at"`
	MaxScore              string `json:"max_score"`
	AnswerVersion         int64  `json:"answer_version"`
	PendingShortTextCount int    `json:"pending_short_text_count"`
	Status                string `json:"status"`
}

type pendingReviewListResponse struct {
	Items []pendingReviewResponse `json:"items"`
}

type gradeShortTextResponse struct {
	Graded bool `json:"graded"`
}

type resultResponse struct {
	ID              uint64 `json:"id"`
	StudentName     string `json:"student_name"`
	SpaceName       string `json:"space_name"`
	AttemptNo       int    `json:"attempt_no"`
	ObjectiveScore  string `json:"objective_score"`
	SubjectiveScore string `json:"subjective_score"`
	TotalScore      string `json:"total_score"`
	SubmittedAt     int64  `json:"submitted_at"`
	Status          string `json:"status"`
}

type resultListResponse struct {
	Items []resultResponse `json:"items"`
}

type resultPublishConfigResponse struct {
	Saved bool         `json:"saved"`
	Exam  examResponse `json:"exam"`
}

type resultExportResponse struct {
	FilePath string `json:"file_path"`
	FileURL  string `json:"file_url"`
	RowCount int    `json:"row_count"`
}

type visibleResultResponse struct {
	AttemptID       uint64 `json:"attempt_id"`
	ExamID          uint64 `json:"exam_id"`
	AttemptNo       int    `json:"attempt_no"`
	ObjectiveScore  string `json:"objective_score"`
	SubjectiveScore string `json:"subjective_score"`
	TotalScore      string `json:"total_score"`
	AnalysisVisible bool   `json:"analysis_visible"`
}

type examListResponse struct {
	Items    []examResponse `json:"items"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
	Total    int64          `json:"total"`
}

func (h examHandler) list(c *gin.Context) {
	tenantID, err := readUintQuery(c, "tenant_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id 必须是正整数"))
		return
	}
	if !h.authorizeExamBusiness(c, tenantID) {
		return
	}
	spaceID, err := readOptionalUintQuery(c, "space_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "space_id 必须是正整数"))
		return
	}
	paperID, err := readOptionalUintQuery(c, "paper_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "paper_id 必须是正整数"))
		return
	}
	if _, err := permissionContextForResourceScope(c, tenantID, spaceID, h.members, h.tenantUsers); err != nil {
		writePermissionOrInternalError(c, err, "构建考试权限上下文失败")
		return
	}
	page, pageSize, err := readPaginationQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "page 和 page_size 必须是正整数"))
		return
	}
	result, err := h.service.List(c.Request.Context(), serviceexam.ListInput{
		TenantID: tenantID,
		SpaceID:  spaceID,
		PaperID:  paperID,
		Search:   c.Query("search"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "读取考试列表失败"))
		return
	}
	items := make([]examResponse, 0, len(result.Items))
	for _, exam := range result.Items {
		items = append(items, examToResponse(exam))
	}
	paged := response.Page(items, result.Page, result.PageSize, result.Total)
	c.JSON(http.StatusOK, response.OK(examListResponse{
		Items:    paged.Items,
		Page:     paged.Page,
		PageSize: paged.PageSize,
		Total:    paged.Total,
	}))
}

func (h examHandler) getManagementDetail(c *gin.Context) {
	examID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "考试 ID 必须是正整数"))
		return
	}
	tenantID, err := readUintQuery(c, "tenant_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id 必须是正整数"))
		return
	}
	if !h.authorizeExamBusiness(c, tenantID) {
		return
	}
	spaceID, err := readOptionalUintQueryValue(c, "space_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "space_id 必须是正整数"))
		return
	}
	permissionContext, err := h.managementPermissionContext(c, tenantID, spaceID)
	if err != nil {
		writePermissionOrInternalError(c, err, "构建考试详情权限上下文失败")
		return
	}
	detail, err := h.management.GetDetail(c.Request.Context(), serviceexam.ManagementDetailInput{
		Permission: permissionContext,
		TenantID:   tenantID,
		ExamID:     examID,
	})
	if err != nil {
		writeExamManagementDetailError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(examManagementDetailToResponse(detail)))
}

func (h examHandler) getOverview(c *gin.Context) {
	examID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "考试 ID 必须是正整数"))
		return
	}
	tenantID, err := readUintQuery(c, "tenant_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id 必须是正整数"))
		return
	}
	if !h.authorizeExamBusiness(c, tenantID) {
		return
	}
	spaceID, err := readOptionalUintQueryValue(c, "space_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "space_id 必须是正整数"))
		return
	}
	permissionContext, err := h.managementPermissionContext(c, tenantID, spaceID)
	if err != nil {
		writePermissionOrInternalError(c, err, "构建考试概览权限上下文失败")
		return
	}
	overview, err := h.management.GetOverview(c.Request.Context(), serviceexam.ManagementDetailInput{
		Permission: permissionContext,
		TenantID:   tenantID,
		ExamID:     examID,
	})
	if err != nil {
		writeExamManagementDetailError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(examOverviewToResponse(overview)))
}

func (h examHandler) getPaperPreview(c *gin.Context) {
	examID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "考试 ID 必须是正整数"))
		return
	}
	tenantID, err := readUintQuery(c, "tenant_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id 必须是正整数"))
		return
	}
	if !h.authorizeExamBusiness(c, tenantID) {
		return
	}
	spaceID, err := readOptionalUintQueryValue(c, "space_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "space_id 必须是正整数"))
		return
	}
	page, pageSize, err := readPaginationQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "page 和 page_size 必须是正整数"))
		return
	}
	permissionContext, err := h.managementPermissionContext(c, tenantID, spaceID)
	if err != nil {
		writePermissionOrInternalError(c, err, "构建试卷预览权限上下文失败")
		return
	}
	preview, err := h.management.GetPaperPreview(c.Request.Context(), serviceexam.PaperPreviewInput{
		ManagementDetailInput: serviceexam.ManagementDetailInput{
			Permission: permissionContext,
			TenantID:   tenantID,
			ExamID:     examID,
		},
		QuestionType: c.Query("type"),
		Page:         page,
		PageSize:     pageSize,
	})
	if err != nil {
		writeExamManagementDetailError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(examPaperPreviewToResponse(preview)))
}

func (h examHandler) listCandidates(c *gin.Context) {
	// 考生管理列表必须复用详情页权限上下文，不能信任前端传入空间范围。
	examID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "考试 ID 必须是正整数"))
		return
	}
	tenantID, err := readUintQuery(c, "tenant_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id 必须是正整数"))
		return
	}
	if !h.authorizeExamBusiness(c, tenantID) {
		return
	}
	spaceID, err := readOptionalUintQueryValue(c, "space_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "space_id 必须是正整数"))
		return
	}
	page, pageSize, err := readPaginationQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "page 和 page_size 必须是正整数"))
		return
	}
	permissionContext, err := h.managementPermissionContext(c, tenantID, spaceID)
	if err != nil {
		writePermissionOrInternalError(c, err, "构建考生管理权限上下文失败")
		return
	}
	candidates, err := h.management.ListCandidates(c.Request.Context(), serviceexam.CandidateListInput{
		ManagementDetailInput: serviceexam.ManagementDetailInput{
			Permission: permissionContext,
			TenantID:   tenantID,
			ExamID:     examID,
		},
		Keyword:  c.Query("keyword"),
		Status:   c.Query("status"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		writeExamManagementDetailError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(examCandidatesToResponse(candidates)))
}

func (h examHandler) getResultsSummary(c *gin.Context) {
	// 成绩摘要必须复用详情页权限裁剪，避免 space_admin / teacher 越权看到整场成绩。
	examID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "考试 ID 必须是正整数"))
		return
	}
	tenantID, err := readUintQuery(c, "tenant_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id 必须是正整数"))
		return
	}
	if !h.authorizeExamBusiness(c, tenantID) {
		return
	}
	spaceID, err := readOptionalUintQueryValue(c, "space_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "space_id 必须是正整数"))
		return
	}
	permissionContext, err := h.managementPermissionContext(c, tenantID, spaceID)
	if err != nil {
		writePermissionOrInternalError(c, err, "构建成绩摘要权限上下文失败")
		return
	}
	summary, err := h.management.GetResultsSummary(c.Request.Context(), serviceexam.ManagementDetailInput{
		Permission: permissionContext,
		TenantID:   tenantID,
		ExamID:     examID,
	})
	if err != nil {
		writeExamManagementDetailError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(examResultsSummaryToResponse(summary)))
}

func (h examHandler) listManagementResults(c *gin.Context) {
	// 成绩列表的排名在 repository 基于全量排序和分页 offset 生成，handler 只负责入参和响应映射。
	examID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "考试 ID 必须是正整数"))
		return
	}
	tenantID, err := readUintQuery(c, "tenant_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id 必须是正整数"))
		return
	}
	if !h.authorizeExamBusiness(c, tenantID) {
		return
	}
	spaceID, err := readOptionalUintQueryValue(c, "space_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "space_id 必须是正整数"))
		return
	}
	page, pageSize, err := readPaginationQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "page 和 page_size 必须是正整数"))
		return
	}
	permissionContext, err := h.managementPermissionContext(c, tenantID, spaceID)
	if err != nil {
		writePermissionOrInternalError(c, err, "构建成绩列表权限上下文失败")
		return
	}
	results, err := h.management.ListResults(c.Request.Context(), serviceexam.ResultListInput{
		ManagementDetailInput: serviceexam.ManagementDetailInput{
			Permission: permissionContext,
			TenantID:   tenantID,
			ExamID:     examID,
		},
		Keyword:  c.Query("keyword"),
		Status:   c.Query("status"),
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		writeExamManagementDetailError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(examManagementResultsToResponse(results)))
}

func (h examHandler) getManagementAnswerSheet(c *gin.Context) {
	// 管理端答卷详情必须同时解析考试 ID 和作答 ID，避免仅凭 attempt_id 跨考试读取答卷。
	examID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "考试 ID 必须是正整数"))
		return
	}
	attemptID, err := readUintParam(c, "attempt_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "作答 ID 必须是正整数"))
		return
	}
	tenantID, err := readUintQuery(c, "tenant_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id 必须是正整数"))
		return
	}
	if !h.authorizeExamBusiness(c, tenantID) {
		return
	}
	spaceID, err := readOptionalUintQueryValue(c, "space_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "space_id 必须是正整数"))
		return
	}
	permissionContext, err := h.managementPermissionContext(c, tenantID, spaceID)
	if err != nil {
		writePermissionOrInternalError(c, err, "构建答卷详情权限上下文失败")
		return
	}
	answerSheet, err := h.management.GetAnswerSheet(c.Request.Context(), serviceexam.AnswerSheetInput{
		ManagementDetailInput: serviceexam.ManagementDetailInput{
			Permission: permissionContext,
			TenantID:   tenantID,
			ExamID:     examID,
		},
		AttemptID: attemptID,
	})
	if err != nil {
		writeExamManagementDetailError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(examAnswerSheetToResponse(answerSheet)))
}

func (h examHandler) listManagementOperationLogs(c *gin.Context) {
	// 操作日志是管理端审计数据，必须复用详情页权限裁剪，不能按前端传入空间直接放行。
	examID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "考试 ID 必须是正整数"))
		return
	}
	tenantID, err := readUintQuery(c, "tenant_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id 必须是正整数"))
		return
	}
	if !h.authorizeExamBusiness(c, tenantID) {
		return
	}
	spaceID, err := readOptionalUintQueryValue(c, "space_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "space_id 必须是正整数"))
		return
	}
	page, pageSize, err := readPaginationQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "page 和 page_size 必须是正整数"))
		return
	}
	permissionContext, err := h.managementPermissionContext(c, tenantID, spaceID)
	if err != nil {
		writePermissionOrInternalError(c, err, "构建操作日志权限上下文失败")
		return
	}
	logs, err := h.management.ListLogs(c.Request.Context(), serviceexam.OperationLogListInput{
		ManagementDetailInput: serviceexam.ManagementDetailInput{
			Permission: permissionContext,
			TenantID:   tenantID,
			ExamID:     examID,
		},
		OperationType: c.Query("operation_type"),
		Page:          page,
		PageSize:      pageSize,
	})
	if err != nil {
		writeExamManagementDetailError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(examOperationLogsToResponse(logs)))
}

func (h examHandler) updateManagementSettings(c *gin.Context) {
	// 设置页首版只允许修改成绩发布配置；使用严格 JSON 解析可以阻止考试时长等公平性字段混入请求。
	examID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "考试 ID 必须是正整数"))
		return
	}
	var request managementSettingsRequest
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "请求体只能包含 tenant_id、space_id、publish_mode 和 score_publish_time"))
		return
	}
	if request.TenantID == 0 || request.PublishMode == "" {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id 和 publish_mode 不能为空"))
		return
	}
	if !validPublishMode(request.PublishMode) {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "publish_mode 只能是 immediate_score 或 manual_publish"))
		return
	}
	if !h.authorizeExamBusiness(c, request.TenantID) {
		return
	}
	permissionContext, err := h.managementPermissionContext(c, request.TenantID, request.SpaceID)
	if err != nil {
		writePermissionOrInternalError(c, err, "构建考试设置权限上下文失败")
		return
	}
	updated, err := h.management.UpdateSettings(c.Request.Context(), serviceexam.ManagementSettingsInput{
		ManagementDetailInput: serviceexam.ManagementDetailInput{
			Permission: permissionContext,
			TenantID:   request.TenantID,
			ExamID:     examID,
		},
		PublishMode:      request.PublishMode,
		ScorePublishTime: request.ScorePublishTime,
	})
	if err != nil {
		writeExamManagementDetailError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(resultPublishConfigResponse{Saved: true, Exam: examToResponse(updated.Exam)}))
}

func (h examHandler) importCandidates(c *gin.Context) {
	// 导入考生会改变考试应考范围，必须走 POST 并在 service 层再次校验管理权限和开考状态。
	examID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "考试 ID 必须是正整数"))
		return
	}
	var request importCandidatesRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "请求体不是合法 JSON"))
		return
	}
	if request.TenantID == 0 {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id 必须是正整数"))
		return
	}
	if len(request.UserIDs) == 0 {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "user_ids 不能为空"))
		return
	}
	if !h.authorizeExamBusiness(c, request.TenantID) {
		return
	}
	permissionContext, err := h.managementWritePermissionContext(c, request.TenantID, request.SpaceID)
	if err != nil {
		writePermissionContextError(c, err, "构建考生导入权限上下文失败")
		return
	}
	result, err := h.management.ImportCandidates(c.Request.Context(), serviceexam.ImportCandidatesInput{
		ManagementDetailInput: serviceexam.ManagementDetailInput{
			Permission: permissionContext,
			TenantID:   request.TenantID,
			ExamID:     examID,
		},
		UserIDs: request.UserIDs,
	})
	if err != nil {
		writeExamManagementWriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(examImportCandidatesToResponse(result)))
}

func (h examHandler) resendInvitations(c *gin.Context) {
	// 重发邀请码只允许作用于真实应考名单，handler 仅解析请求，权限和名单过滤统一交给 service / repository。
	examID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "考试 ID 必须是正整数"))
		return
	}
	var request resendInvitationsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "请求体不是合法 JSON"))
		return
	}
	if request.TenantID == 0 {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id 必须是正整数"))
		return
	}
	if len(request.UserIDs) == 0 {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "user_ids 不能为空"))
		return
	}
	if !h.authorizeExamBusiness(c, request.TenantID) {
		return
	}
	permissionContext, err := h.managementWritePermissionContext(c, request.TenantID, request.SpaceID)
	if err != nil {
		writePermissionContextError(c, err, "构建重发邀请码权限上下文失败")
		return
	}
	result, err := h.management.ResendInvitations(c.Request.Context(), serviceexam.ResendInvitationsInput{
		ManagementDetailInput: serviceexam.ManagementDetailInput{
			Permission: permissionContext,
			TenantID:   request.TenantID,
			ExamID:     examID,
		},
		UserIDs: request.UserIDs,
	})
	if err != nil {
		writeExamManagementWriteError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(examResendInvitationsToResponse(result)))
}

func (h examHandler) publish(c *gin.Context) {
	var request publishExamRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "请求体不是合法 JSON"))
		return
	}
	if err := request.validate(); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, err.Error()))
		return
	}
	if !h.authorizeExamBusiness(c, request.TenantID) {
		return
	}
	principal, err := h.liveTenantPrincipal(c, request.TenantID)
	if err != nil {
		writePermissionContextError(c, err, "读取当前发布人失败")
		return
	}
	targets, err := request.normalizedTargets()
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, err.Error()))
		return
	}
	serviceTargets := make([]serviceexam.Target, 0, len(targets))
	for _, target := range targets {
		scopeSpaceIDs, ok := h.authorizePublishScope(c, request.TenantID, request.PaperID, target.TargetType, target.TargetID, target.ScopeSpaceIDs)
		if !ok {
			return
		}
		serviceTargets = append(serviceTargets, serviceexam.Target{
			TenantID:      request.TenantID,
			TargetType:    target.TargetType,
			TargetID:      target.TargetID,
			ScopeSpaceIDs: scopeSpaceIDs,
		})
	}
	created, err := h.service.CreateWithTarget(c.Request.Context(), serviceexam.CreateWithTargetInput{
		TenantID:         request.TenantID,
		PaperID:          request.PaperID,
		Name:             request.Name,
		Targets:          serviceTargets,
		StartTime:        request.StartTime,
		EndTime:          request.EndTime,
		DurationMinutes:  request.DurationMinutes,
		MaxAttempts:      request.MaxAttempts,
		ResultStrategy:   request.ResultStrategy,
		PublishMode:      request.PublishMode,
		ScorePublishTime: request.ScorePublishTime,
		Status:           request.normalizedStatus(),
		ActorID:          principal.UserID,
		ActorType:        principal.SubjectType,
		ActorRole:        principal.Role,
	})
	if err != nil {
		writeExamServiceError(c, err)
		return
	}
	result := examToResponse(created)
	result.TargetType = targets[0].TargetType
	result.TargetID = targets[0].TargetID
	c.JSON(http.StatusOK, response.OK(result))
}

func (h examHandler) updateDraft(c *gin.Context) {
	examID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "考试 ID 必须是正整数"))
		return
	}
	var request publishExamRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "请求体不是合法 JSON"))
		return
	}
	if err := request.validate(); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, err.Error()))
		return
	}
	if request.Status != "" && request.Status != serviceexam.StatusDraft {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "草稿更新接口只能保存 draft 状态"))
		return
	}
	if !h.authorizeExamBusiness(c, request.TenantID) {
		return
	}
	principal, err := h.liveTenantPrincipal(c, request.TenantID)
	if err != nil {
		writePermissionContextError(c, err, "读取当前操作人失败")
		return
	}
	if !h.authorizeExamStatusUpdate(c, request.TenantID, examID, principal.UserID) {
		return
	}
	targets, err := request.normalizedTargets()
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, err.Error()))
		return
	}
	serviceTargets := make([]serviceexam.Target, 0, len(targets))
	for _, target := range targets {
		scopeSpaceIDs, ok := h.authorizePublishScope(c, request.TenantID, request.PaperID, target.TargetType, target.TargetID, target.ScopeSpaceIDs)
		if !ok {
			return
		}
		serviceTargets = append(serviceTargets, serviceexam.Target{
			TenantID:      request.TenantID,
			TargetType:    target.TargetType,
			TargetID:      target.TargetID,
			ScopeSpaceIDs: scopeSpaceIDs,
		})
	}
	updated, err := h.service.UpdateDraftWithTarget(c.Request.Context(), serviceexam.UpdateDraftWithTargetInput{
		TenantID:         request.TenantID,
		ExamID:           examID,
		PaperID:          request.PaperID,
		Name:             request.Name,
		Targets:          serviceTargets,
		StartTime:        request.StartTime,
		EndTime:          request.EndTime,
		DurationMinutes:  request.DurationMinutes,
		MaxAttempts:      request.MaxAttempts,
		ResultStrategy:   request.ResultStrategy,
		PublishMode:      request.PublishMode,
		ScorePublishTime: request.ScorePublishTime,
		ActorID:          principal.UserID,
		ActorType:        principal.SubjectType,
		ActorRole:        principal.Role,
	})
	if err != nil {
		writeExamServiceError(c, err)
		return
	}
	result := examToResponse(updated)
	result.TargetType = targets[0].TargetType
	result.TargetID = targets[0].TargetID
	c.JSON(http.StatusOK, response.OK(result))
}

func (h examHandler) updateStatus(c *gin.Context) {
	examID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "考试 ID 必须是正整数"))
		return
	}
	var request examStatusRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "请求体不是合法 JSON"))
		return
	}
	if err := request.validate(); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, err.Error()))
		return
	}
	if !h.authorizeExamBusiness(c, request.TenantID) {
		return
	}
	principal, err := h.liveTenantPrincipal(c, request.TenantID)
	if err != nil {
		writePermissionContextError(c, err, "读取当前操作人失败")
		return
	}
	if !h.authorizeExamStatusUpdate(c, request.TenantID, examID, principal.UserID) {
		return
	}
	updated, err := h.service.UpdateStatus(c.Request.Context(), serviceexam.UpdateStatusInput{
		TenantID:  request.TenantID,
		ExamID:    examID,
		Status:    request.Status,
		ActorID:   principal.UserID,
		ActorType: principal.SubjectType,
		ActorRole: principal.Role,
	})
	if err != nil {
		writeExamServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(examToResponse(updated)))
}

func (h examHandler) authorizeExamStatusUpdate(c *gin.Context, tenantID uint64, examID uint64, actorID uint64) bool {
	permissionContext, err := h.managementPermissionContext(c, tenantID, 0)
	if err != nil {
		writePermissionContextError(c, err, "构建考试状态权限上下文失败")
		return false
	}
	detail, err := h.management.GetDetail(c.Request.Context(), serviceexam.ManagementDetailInput{
		Permission: permissionContext,
		TenantID:   tenantID,
		ExamID:     examID,
	})
	if err != nil {
		if errors.Is(err, permission.ErrForbidden) {
			writePermissionOrInternalError(c, err, "无权更新考试状态")
			return false
		}
		writeExamServiceError(c, err)
		return false
	}
	if detail.Permissions.CanUpdateSettings {
		return true
	}
	if detail.Exam.CreatedBy == actorID && examTargetSpacesCoveredByAllowedSpaces(detail.TargetSpaceIDs, detail.AllowedSpaceIDs) {
		return true
	}
	writePermissionOrInternalError(c, permission.ErrForbidden, "无权更新考试状态")
	return false
}

func (h examHandler) authorizeExamBusiness(c *gin.Context, tenantID uint64) bool {
	principal, err := h.liveTenantPrincipal(c, tenantID)
	if err != nil {
		writePermissionContextError(c, err, "读取当前用户权限失败")
		return false
	}
	if principal.Role == permission.RoleTenantAdmin || principal.Role == permission.RoleTeacher {
		return true
	}
	allowed, err := hasExamBusinessMembership(c, h.members, tenantID, principal.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "读取当前用户空间授权失败"))
		return false
	}
	if allowed {
		return true
	}
	writePermissionOrInternalError(c, permission.ErrForbidden, "无权执行当前操作")
	return false
}

func (h examHandler) authorizePublishScope(c *gin.Context, tenantID uint64, paperID uint64, targetType string, targetID uint64, requestedScopeSpaceIDs []uint64) ([]uint64, bool) {
	spaceID, err := h.papers.GetPaperSpaceID(c.Request.Context(), tenantID, paperID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "读取考试试卷权限范围失败"))
		return nil, false
	}
	if spaceID != nil {
		permissionContext, err := permissionContextForResourceScope(c, tenantID, spaceID, h.members, h.tenantUsers)
		if err != nil {
			writePermissionOrInternalError(c, err, "构建考试发布权限上下文失败")
			return nil, false
		}
		permissionContext.PaperScope = map[uint64]uint64{paperID: *spaceID}
		if err := permission.NewFixedRoleChecker().CanPublishExam(permissionContext, paperID); err != nil {
			writePermissionOrInternalError(c, err, "校验考试发布权限失败")
			return nil, false
		}
	}
	switch targetType {
	case serviceexam.TargetTypeSpace:
		if !h.authorizePublishSpaceTarget(c, tenantID, targetID) {
			return nil, false
		}
		return nil, true
	case serviceexam.TargetTypeUser:
		return h.authorizePublishUserTarget(c, tenantID, targetID, spaceID, requestedScopeSpaceIDs)
	default:
		writePermissionOrInternalError(c, permission.ErrForbidden, "校验考试发布目标失败")
		return nil, false
	}
}

func (h examHandler) authorizePublishSpaceTarget(c *gin.Context, tenantID uint64, spaceID uint64) bool {
	exists, err := h.members.SpaceExists(c.Request.Context(), tenantID, spaceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "读取考试发布目标失败"))
		return false
	}
	if !exists {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, servicespace.ErrSpaceNotFound.Error()))
		return false
	}
	permissionContext, err := permissionContextForResourceScope(c, tenantID, &spaceID, h.members, h.tenantUsers)
	if err != nil {
		writePermissionOrInternalError(c, err, "构建考试发布目标权限上下文失败")
		return false
	}
	if permissionContext.Role == permission.RoleTenantAdmin {
		return true
	}
	role := permissionContext.SpaceMemberships[spaceID]
	if role == permission.RoleSpaceAdmin || role == permission.RoleTeacher {
		return true
	}
	writePermissionOrInternalError(c, permission.ErrForbidden, "校验考试发布目标失败")
	return false
}

func (h examHandler) authorizePublishUserTarget(c *gin.Context, tenantID uint64, userID uint64, currentSpaceID *uint64, requestedScopeSpaceIDs []uint64) ([]uint64, bool) {
	principal, ok := currentAuthPrincipal(c)
	if !ok {
		writePermissionOrInternalError(c, permission.ErrForbidden, "校验考试发布目标失败")
		return nil, false
	}
	requestedScopeSpaceIDs = uniqueUint64s(requestedScopeSpaceIDs)
	requestedScopeSet := make(map[uint64]struct{}, len(requestedScopeSpaceIDs))
	for _, spaceID := range requestedScopeSpaceIDs {
		if spaceID == 0 {
			writePermissionOrInternalError(c, permission.ErrForbidden, "校验考试发布目标失败")
			return nil, false
		}
		requestedScopeSet[spaceID] = struct{}{}
	}
	if principal.Role != permission.RoleTenantAdmin && currentSpaceID == nil && len(requestedScopeSpaceIDs) == 0 {
		writePermissionOrInternalError(c, permission.ErrForbidden, "校验考试发布目标失败")
		return nil, false
	}
	user, err := h.tenantUsers.Get(c.Request.Context(), tenantID, userID)
	if errors.Is(err, servicetenantuser.ErrUserNotFound) {
		writePermissionOrInternalError(c, permission.ErrForbidden, "校验考试发布目标失败")
		return nil, false
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "读取考试发布目标失败"))
		return nil, false
	}
	if user.Status != servicetenantuser.StatusEnabled || user.Role != servicetenantuser.RoleStudent {
		writePermissionOrInternalError(c, permission.ErrForbidden, "校验考试发布目标失败")
		return nil, false
	}
	memberships, err := h.members.ListEffectiveMembershipsForUser(c.Request.Context(), tenantID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "读取考试发布目标范围失败"))
		return nil, false
	}
	scopeSpaceIDs := make([]uint64, 0, len(memberships))
	for _, membership := range memberships {
		if membership.Role != servicespace.RoleStudent {
			continue
		}
		if currentSpaceID != nil && membership.SpaceID != *currentSpaceID {
			continue
		}
		if len(requestedScopeSet) > 0 {
			if _, ok := requestedScopeSet[membership.SpaceID]; !ok {
				continue
			}
		}
		if h.actorCanPublishToSpace(c, tenantID, membership.SpaceID) {
			scopeSpaceIDs = append(scopeSpaceIDs, membership.SpaceID)
		}
	}
	if len(requestedScopeSpaceIDs) > 0 && len(uniqueUint64s(scopeSpaceIDs)) != len(requestedScopeSpaceIDs) {
		writePermissionOrInternalError(c, permission.ErrForbidden, "校验考试发布目标失败")
		return nil, false
	}
	if len(scopeSpaceIDs) > 0 {
		return uniqueUint64s(scopeSpaceIDs), true
	}
	writePermissionOrInternalError(c, permission.ErrForbidden, "校验考试发布目标失败")
	return nil, false
}

func (h examHandler) actorCanPublishToSpace(c *gin.Context, tenantID uint64, spaceID uint64) bool {
	permissionContext, err := permissionContextForResourceScope(c, tenantID, &spaceID, h.members, h.tenantUsers)
	if err != nil {
		return false
	}
	if permissionContext.Role == permission.RoleTenantAdmin {
		return true
	}
	role := permissionContext.SpaceMemberships[spaceID]
	return role == permission.RoleSpaceAdmin || role == permission.RoleTeacher
}

func uniqueUint64s(values []uint64) []uint64 {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[uint64]struct{}, len(values))
	unique := make([]uint64, 0, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}
	return unique
}

func examTargetSpacesCoveredByAllowedSpaces(targetSpaceIDs []uint64, allowedSpaceIDs []uint64) bool {
	targetSpaceIDs = uniqueUint64s(targetSpaceIDs)
	allowedSpaceIDs = uniqueUint64s(allowedSpaceIDs)
	if len(targetSpaceIDs) == 0 || len(allowedSpaceIDs) == 0 {
		return false
	}
	allowed := make(map[uint64]struct{}, len(allowedSpaceIDs))
	for _, spaceID := range allowedSpaceIDs {
		allowed[spaceID] = struct{}{}
	}
	for _, spaceID := range targetSpaceIDs {
		if _, ok := allowed[spaceID]; !ok {
			return false
		}
	}
	return true
}

func (h examHandler) resolveInvite(c *gin.Context) {
	var request resolveExamInviteRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "请求体不是合法 JSON"))
		return
	}
	if err := request.validate(); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, err.Error()))
		return
	}
	principal, ok := currentAuthPrincipal(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, response.Fail(code.InvalidParam, "请先登录或注册后进入考试"))
		return
	}
	if principal.SubjectType != permission.SubjectTenantUser || principal.TenantID == 0 {
		c.JSON(http.StatusForbidden, response.Fail(code.InvalidParam, "只有租户用户可以进入考试"))
		return
	}
	// 邀请码入口先解析考试基础信息；真正创建 attempt 和 exam token 放在答题页 API 中处理。
	exam, err := h.service.ResolveInvite(c.Request.Context(), serviceexam.ResolveInviteInput{
		InviteCode: request.InviteCode,
		UserID:     principal.UserID,
	})
	if err != nil {
		writeExamServiceError(c, err)
		return
	}
	if principal.TenantID != exam.TenantID {
		c.JSON(http.StatusForbidden, response.Fail(code.InvalidParam, "当前登录用户不属于该考试租户"))
		return
	}
	if err := saveExamEntrySession(c, exam, principal.UserID, defaultSessionMaxAgeSeconds(h.sessionMaxAgeSeconds)); err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "保存考试入口会话失败"))
		return
	}
	c.JSON(http.StatusOK, response.OK(examToResponse(exam)))
}

func saveExamEntrySession(c *gin.Context, exam serviceexam.Exam, userID uint64, maxAgeSeconds int) error {
	session := sessions.Default(c)
	session.Set(examEntrySessionTenantIDKey, exam.TenantID)
	session.Set(examEntrySessionExamIDKey, exam.ID)
	session.Set(examEntrySessionUserIDKey, userID)
	session.Options(sessions.Options{
		Path:     "/",
		MaxAge:   maxAgeSeconds,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return session.Save()
}

func sessionUint64(value any) (uint64, bool) {
	switch v := value.(type) {
	case uint64:
		return v, v > 0
	case uint:
		return uint64(v), v > 0
	case int:
		if v <= 0 {
			return 0, false
		}
		return uint64(v), true
	case int64:
		if v <= 0 {
			return 0, false
		}
		return uint64(v), true
	default:
		return 0, false
	}
}

func examEntrySessionUser(c *gin.Context, tenantID uint64, examID uint64) (uint64, bool) {
	session := sessions.Default(c)
	sessionTenantID, ok := sessionUint64(session.Get(examEntrySessionTenantIDKey))
	if !ok || sessionTenantID != tenantID {
		return 0, false
	}
	sessionExamID, ok := sessionUint64(session.Get(examEntrySessionExamIDKey))
	if !ok || sessionExamID != examID {
		return 0, false
	}
	userID, ok := sessionUint64(session.Get(examEntrySessionUserIDKey))
	if !ok || userID == 0 {
		return 0, false
	}
	return userID, true
}

func (h examHandler) startAttempt(c *gin.Context) {
	examID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "考试 ID 必须是正整数"))
		return
	}
	var request startAttemptRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "请求体不是合法 JSON"))
		return
	}
	if request.TenantID == 0 {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id 必须是正整数"))
		return
	}
	userID, ok := examEntrySessionUser(c, request.TenantID, examID)
	if !ok {
		c.JSON(http.StatusUnauthorized, response.Fail(code.InvalidParam, "请先通过邀请码进入考试"))
		return
	}
	if request.UserID != 0 && request.UserID != userID {
		c.JSON(http.StatusForbidden, response.Fail(code.InvalidParam, "考试入口身份不匹配"))
		return
	}
	started, err := h.service.StartExam(c.Request.Context(), serviceexam.StartInput{
		TenantID: request.TenantID,
		ExamID:   examID,
		UserID:   userID,
	})
	if err != nil {
		writeExamServiceError(c, err)
		return
	}
	// 开考成功后立即固化本次题目快照，后续保存答案只引用 attempt_question_id。
	snapshots, err := h.service.GenerateAttemptSnapshots(c.Request.Context(), serviceexam.GenerateSnapshotInput{
		TenantID:  request.TenantID,
		ExamID:    examID,
		AttemptID: started.Attempt.ID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "生成作答题目失败"))
		return
	}
	snapshots, err = h.service.SaveAttemptQuestions(c.Request.Context(), snapshots)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "保存作答题目失败"))
		return
	}
	exam, err := h.service.GetExam(c.Request.Context(), request.TenantID, examID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "读取考试信息失败"))
		return
	}
	questions, err := attemptQuestionsToResponse(snapshots)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "读取作答题目失败"))
		return
	}
	c.JSON(http.StatusOK, response.OK(startAttemptResponse{
		Attempt:   attemptToResponse(started.Attempt, exam),
		ExamToken: started.ExamToken,
		Questions: questions,
	}))
}

func (h examHandler) saveAnswer(c *gin.Context) {
	attemptID, err := readUintParam(c, "attempt_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "作答 ID 必须是正整数"))
		return
	}
	attemptQuestionID, err := readUintParam(c, "attempt_question_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "作答题目 ID 必须是正整数"))
		return
	}
	var request saveAnswerRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "请求体不是合法 JSON"))
		return
	}
	if request.TenantID == 0 || request.ExamToken == "" || request.QuestionType == "" {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id、exam_token 和 question_type 不能为空"))
		return
	}
	err = h.taking.SaveAnswer(c.Request.Context(), serviceexam.SaveAnswerInput{
		TenantID:          request.TenantID,
		AttemptID:         attemptID,
		AttemptQuestionID: attemptQuestionID,
		ExamToken:         request.ExamToken,
		QuestionType:      request.QuestionType,
		OptionIDs:         request.OptionIDs,
		Text:              request.Text,
	})
	if err != nil {
		writeTakingServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(answerSavedResponse{Saved: true, UpdatedAt: h.now()}))
}

func (h examHandler) submitAttempt(c *gin.Context) {
	attemptID, err := readUintParam(c, "attempt_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "作答 ID 必须是正整数"))
		return
	}
	var request submitAttemptRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "请求体不是合法 JSON"))
		return
	}
	if request.TenantID == 0 || request.ExamToken == "" {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id 和 exam_token 不能为空"))
		return
	}
	eventType := request.EventType
	if eventType == "" {
		eventType = serviceexam.EventTypeSubmit
	}
	if eventType != serviceexam.EventTypeSubmit && eventType != serviceexam.EventTypeAutoSubmit {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "event_type 只能是 submit 或 auto_submit"))
		return
	}
	if err := h.taking.Submit(c.Request.Context(), serviceexam.SubmitInput{
		TenantID:  request.TenantID,
		AttemptID: attemptID,
		ExamToken: request.ExamToken,
		EventType: eventType,
	}); err != nil {
		writeTakingServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(submitAttemptResponse{Submitted: true}))
}

func (h examHandler) recordEvent(c *gin.Context) {
	attemptID, err := readUintParam(c, "attempt_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "作答 ID 必须是正整数"))
		return
	}
	var request recordExamEventRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "请求体不是合法 JSON"))
		return
	}
	if request.TenantID == 0 || request.ExamToken == "" || request.EventType == "" {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id、exam_token 和 event_type 不能为空"))
		return
	}
	if !isRecordableExamEvent(request.EventType) {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "event_type 只能是 blur 或 focus"))
		return
	}
	if request.Payload == "" {
		request.Payload = "{}"
	}
	if err := h.taking.RecordEvent(c.Request.Context(), serviceexam.RecordEventInput{
		TenantID:  request.TenantID,
		AttemptID: attemptID,
		ExamToken: request.ExamToken,
		EventType: request.EventType,
		Payload:   request.Payload,
	}); err != nil {
		writeTakingServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(examEventResponse{Recorded: true}))
}

func isRecordableExamEvent(eventType string) bool {
	return eventType == serviceexam.EventTypeBlur || eventType == serviceexam.EventTypeFocus
}

func (h examHandler) getExamEntryResult(c *gin.Context) {
	attemptID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "成绩 ID 必须是正整数"))
		return
	}
	principal, ok := currentAuthPrincipal(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, response.Fail(code.InvalidParam, "请先登录"))
		return
	}
	if principal.SubjectType != permission.SubjectTenantUser || principal.TenantID == 0 {
		c.JSON(http.StatusForbidden, response.Fail(code.InvalidParam, "只有租户用户可以查看考试成绩"))
		return
	}
	principal, err = h.liveTenantPrincipal(c, principal.TenantID)
	if err != nil {
		writePermissionContextError(c, err, "读取当前用户权限失败")
		return
	}
	permissionContext := permission.PermissionContext{
		SubjectType: principal.SubjectType,
		UserID:      principal.UserID,
		TenantID:    principal.TenantID,
		Role:        principal.Role,
	}
	currentVisible, err := h.result.GetVisibleResult(c.Request.Context(), serviceexam.ResultQueryInput{
		Permission: permissionContext,
		TenantID:   principal.TenantID,
		AttemptID:  attemptID,
	})
	if err != nil {
		writeVisibleResultError(c, err)
		return
	}
	exam, err := h.service.GetExam(c.Request.Context(), principal.TenantID, currentVisible.ExamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "读取考试信息失败"))
		return
	}
	visible, err := h.result.SelectVisibleResult(c.Request.Context(), serviceexam.SelectResultInput{
		Permission:     permissionContext,
		TenantID:       principal.TenantID,
		ExamID:         currentVisible.ExamID,
		ResultStrategy: exam.ResultStrategy,
	})
	if err != nil {
		writeVisibleResultError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(visibleResultToResponse(visible)))
}

func (h examHandler) listPendingReviews(c *gin.Context) {
	tenantID, err := readUintQuery(c, "tenant_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id 必须是正整数"))
		return
	}
	examID, err := readUintQuery(c, "exam_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "exam_id 必须是正整数"))
		return
	}
	permissionContext, err := h.readActorPermissionQuery(c, tenantID)
	if err != nil {
		writePermissionContextError(c, err, "读取待阅卷列表失败")
		return
	}
	items, err := h.review.ListPendingAttempts(c.Request.Context(), serviceexam.ListPendingAttemptsInput{
		Permission: permissionContext,
		TenantID:   tenantID,
		ExamID:     examID,
		Keyword:    c.Query("search"),
	})
	if err != nil {
		writePermissionOrInternalError(c, err, "读取待阅卷列表失败")
		return
	}
	responses := make([]pendingReviewResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, pendingReviewToResponse(item))
	}
	c.JSON(http.StatusOK, response.OK(pendingReviewListResponse{Items: responses}))
}

func (h examHandler) gradeShortText(c *gin.Context) {
	attemptID, err := readUintParam(c, "attempt_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "作答 ID 必须是正整数"))
		return
	}
	attemptQuestionID, err := readUintParam(c, "attempt_question_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "作答题目 ID 必须是正整数"))
		return
	}
	var request gradeShortTextRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "请求体不是合法 JSON"))
		return
	}
	if request.TenantID == 0 || request.ExamID == 0 || request.AnswerVersion == 0 || request.Score == "" {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id、exam_id、answer_version 和 score 不能为空"))
		return
	}
	permissionContext, err := h.permissionContextFromSession(c, request.TenantID, request.SpaceID)
	if err != nil {
		writePermissionContextError(c, err, "保存阅卷结果失败")
		return
	}
	err = h.review.GradeShortText(c.Request.Context(), serviceexam.GradeShortTextInput{
		Permission:        permissionContext,
		TenantID:          request.TenantID,
		ExamID:            request.ExamID,
		AttemptID:         attemptID,
		AttemptQuestionID: attemptQuestionID,
		AnswerVersion:     request.AnswerVersion,
		Score:             request.Score,
		Comment:           request.Comment,
	})
	if err != nil {
		if errors.Is(err, serviceexam.ErrAnswerVersionConflict) {
			c.JSON(http.StatusConflict, response.Fail(code.InvalidParam, "答案已被其他阅卷人更新，请刷新后重试"))
			return
		}
		if errors.Is(err, serviceexam.ErrInvalidGradeScore) {
			c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "score 必须是不超过题目满分的非负数字"))
			return
		}
		writePermissionOrInternalError(c, err, "保存阅卷结果失败")
		return
	}
	c.JSON(http.StatusOK, response.OK(gradeShortTextResponse{Graded: true}))
}

func (h examHandler) listResults(c *gin.Context) {
	tenantID, err := readUintQuery(c, "tenant_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id 必须是正整数"))
		return
	}
	examID, err := readUintQuery(c, "exam_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "exam_id 必须是正整数"))
		return
	}
	permissionContext, err := h.readActorPermissionQuery(c, tenantID)
	if err != nil {
		writePermissionContextError(c, err, "读取成绩列表失败")
		return
	}
	rows, err := h.export.ListExamScores(c.Request.Context(), serviceexam.ListExamScoresInput{
		Permission: permissionContext,
		TenantID:   tenantID,
		ExamID:     examID,
	})
	if err != nil {
		writePermissionOrInternalError(c, err, "读取成绩列表失败")
		return
	}
	items := make([]resultResponse, 0, len(rows))
	for index, row := range rows {
		items = append(items, scoreRowToResponse(uint64(index+1), row))
	}
	c.JSON(http.StatusOK, response.OK(resultListResponse{Items: items}))
}

func (h examHandler) saveResultPublishConfig(c *gin.Context) {
	var request saveResultPublishConfigRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "请求体不是合法 JSON"))
		return
	}
	if request.TenantID == 0 || request.ExamID == 0 || request.PublishMode == "" {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id、exam_id 和 publish_mode 不能为空"))
		return
	}
	if !validPublishMode(request.PublishMode) {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "publish_mode 只能是 immediate_score 或 manual_publish"))
		return
	}
	permissionContext, err := h.permissionContextForResultPublishConfig(c, request.TenantID, request.ExamID)
	if err != nil {
		writePermissionContextError(c, err, "保存成绩发布配置失败")
		return
	}
	// 旧成绩发布配置接口继续保留兼容入口，但写权限必须与考试详情页保持一致，
	// 只能由租户管理员统一调整，避免空间管理员或教师绕过新的管理边界回写配置。
	if permissionContext.Role != permission.RoleTenantAdmin {
		writePermissionOrInternalError(c, permission.ErrForbidden, "保存成绩发布配置失败")
		return
	}
	principal, err := h.liveTenantPrincipal(c, request.TenantID)
	if err != nil {
		writePermissionContextError(c, err, "读取当前发布人失败")
		return
	}
	// 成绩可见性由后端统一以 publish_mode 和 score_publish_time 判断，前端只提交配置。
	exam, err := h.service.UpdateScorePublishConfig(c.Request.Context(), serviceexam.UpdateScorePublishConfigInput{
		TenantID:         request.TenantID,
		ExamID:           request.ExamID,
		PublishMode:      request.PublishMode,
		ScorePublishTime: request.ScorePublishTime,
		ActorID:          principal.UserID,
		ActorType:        principal.SubjectType,
		ActorRole:        principal.Role,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "保存成绩发布配置失败"))
		return
	}
	c.JSON(http.StatusOK, response.OK(resultPublishConfigResponse{Saved: true, Exam: examToResponse(exam)}))
}

func (h examHandler) exportResults(c *gin.Context) {
	var request exportResultsRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "请求体不是合法 JSON"))
		return
	}
	if request.TenantID == 0 || request.ExamID == 0 {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id 和 exam_id 不能为空"))
		return
	}
	permissionContext, err := h.permissionContextFromSession(c, request.TenantID, request.SpaceID)
	if err != nil {
		writePermissionContextError(c, err, "导出成绩失败")
		return
	}
	result, err := h.export.ExportExamScores(c.Request.Context(), serviceexam.ExportExamScoresInput{
		Permission: permissionContext,
		TenantID:   request.TenantID,
		ExamID:     request.ExamID,
	})
	if err != nil {
		writePermissionOrInternalError(c, err, "导出成绩失败")
		return
	}
	fileName := filepath.Base(result.FilePath)
	c.JSON(http.StatusOK, response.OK(resultExportResponse{
		FilePath: fileName,
		FileURL:  exportDownloadURL(fileName, request.TenantID, request.ExamID, request.SpaceID),
		RowCount: result.RowCount,
	}))
}

func (h examHandler) downloadExport(c *gin.Context) {
	fileName := c.Param("file_name")
	tenantID, err := readUintQuery(c, "tenant_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id 必须是正整数"))
		return
	}
	examID, err := readUintQuery(c, "exam_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "exam_id 必须是正整数"))
		return
	}
	spaceID, err := readOptionalUintQueryValue(c, "space_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "space_id 必须是正整数"))
		return
	}
	permissionContext, err := h.permissionContextFromSession(c, tenantID, spaceID)
	if err != nil {
		writePermissionContextError(c, err, "下载成绩导出失败")
		return
	}
	permissionContext = permissionContextWithExamScope(permissionContext, examID, spaceID)
	if err := permission.NewFixedRoleChecker().CanExportExamResults(permissionContext, examID); err != nil {
		writePermissionOrInternalError(c, err, "下载成绩导出失败")
		return
	}
	filePath, err := h.export.ResolveExportFile(fileName, examID, permissionContext)
	if errors.Is(err, serviceexam.ErrInvalidExportFile) {
		c.JSON(http.StatusNotFound, response.Fail(code.InvalidParam, "导出文件不存在"))
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "读取导出文件失败"))
		return
	}
	c.FileAttachment(filePath, fileName)
}

func exportDownloadURL(fileName string, tenantID uint64, examID uint64, spaceID uint64) string {
	query := url.Values{}
	query.Set("tenant_id", strconv.FormatUint(tenantID, 10))
	query.Set("exam_id", strconv.FormatUint(examID, 10))
	if spaceID != 0 {
		query.Set("space_id", strconv.FormatUint(spaceID, 10))
	}
	return "/api/v1/results/export-files/" + url.PathEscape(fileName) + "?" + query.Encode()
}

func validResultStrategy(strategy string) bool {
	return strategy == serviceexam.ResultStrategyLatest || strategy == serviceexam.ResultStrategyHighest
}

func validPublishMode(mode string) bool {
	return mode == serviceexam.PublishModeImmediateScore || mode == serviceexam.PublishModeManualPublish
}

func validCreateExamStatus(status string) bool {
	return status == "" || status == serviceexam.StatusDraft || status == serviceexam.StatusPublished
}

func validUpdateExamStatus(status string) bool {
	return status == serviceexam.StatusPublished || status == serviceexam.StatusClosed || status == serviceexam.StatusDisabled
}

func (r publishExamRequest) validate() error {
	if r.TenantID == 0 {
		return errors.New("tenant_id 必须是正整数")
	}
	if r.PaperID == 0 {
		return errors.New("paper_id 必须是正整数")
	}
	if r.Name == "" {
		return errors.New("name 不能为空")
	}
	if _, err := r.normalizedTargets(); err != nil {
		return err
	}
	if r.DurationMinutes <= 0 {
		return errors.New("duration_minutes 必须是正整数")
	}
	if r.MaxAttempts < 0 {
		return errors.New("max_attempts 必须是 0 或正整数")
	}
	if r.ResultStrategy == "" {
		return errors.New("result_strategy 不能为空")
	}
	if !validResultStrategy(r.ResultStrategy) {
		return errors.New("result_strategy 只能是 latest 或 highest")
	}
	if r.PublishMode == "" {
		return errors.New("publish_mode 不能为空")
	}
	if !validPublishMode(r.PublishMode) {
		return errors.New("publish_mode 只能是 immediate_score 或 manual_publish")
	}
	if r.StartTime <= 0 || r.EndTime <= r.StartTime {
		return errors.New("考试时间范围不合法")
	}
	if !validCreateExamStatus(r.Status) {
		return errors.New("status 只能是 draft 或 published")
	}
	return nil
}

func (r publishExamRequest) normalizedStatus() string {
	if r.Status == "" {
		return serviceexam.StatusPublished
	}
	return r.Status
}

func (r examStatusRequest) validate() error {
	if r.TenantID == 0 {
		return errors.New("tenant_id 必须是正整数")
	}
	if !validUpdateExamStatus(r.Status) {
		return errors.New("status 只能是 published、closed 或 disabled")
	}
	return nil
}

// normalizedTargets 将旧版单目标字段和新版 targets 数组统一成同一种结构。
//
// 这里保留旧字段是为了让现有前端和测试在后端升级期间继续可用；当 targets
// 非空时以新版数组为准，并在进入 service 前去重，避免重复目标触发数据库唯一约束。
func (r publishExamRequest) normalizedTargets() ([]publishExamTargetRequest, error) {
	targets := r.Targets
	if len(targets) == 0 && (r.TargetType != "" || r.TargetID != 0) {
		targets = []publishExamTargetRequest{{
			TargetType: r.TargetType,
			TargetID:   r.TargetID,
		}}
	}
	if len(targets) == 0 {
		return nil, errors.New("targets 不能为空")
	}

	normalized := make([]publishExamTargetRequest, 0, len(targets))
	seen := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		if target.TargetType != serviceexam.TargetTypeSpace && target.TargetType != serviceexam.TargetTypeUser {
			return nil, errors.New("target_type 只能是 space 或 user")
		}
		if target.TargetID == 0 {
			return nil, errors.New("target_id 必须是正整数")
		}
		scopeSpaceIDs := uniqueUint64s(target.ScopeSpaceIDs)
		for _, spaceID := range scopeSpaceIDs {
			if spaceID == 0 {
				return nil, errors.New("scope_space_ids 必须是正整数")
			}
		}
		key := target.TargetType + ":" + strconv.FormatUint(target.TargetID, 10)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		target.ScopeSpaceIDs = scopeSpaceIDs
		normalized = append(normalized, target)
	}
	return normalized, nil
}

func (r resolveExamInviteRequest) validate() error {
	if r.InviteCode == "" {
		return errors.New("invite_code 不能为空")
	}
	return nil
}

func writeExamServiceError(c *gin.Context, err error) {
	if errors.Is(err, serviceexam.ErrDurationExceedsExamWindow) ||
		errors.Is(err, serviceexam.ErrShortTextCannotRepeatAttempt) ||
		errors.Is(err, serviceexam.ErrShortTextCannotImmediateScore) ||
		errors.Is(err, serviceexam.ErrPaperNotEnabled) ||
		errors.Is(err, serviceexam.ErrDuplicateExamTarget) ||
		errors.Is(err, serviceexam.ErrExamTargetRequired) ||
		errors.Is(err, serviceexam.ErrInvalidExamStatus) ||
		errors.Is(err, serviceexam.ErrRuleLiveQuestionPoolInsufficient) ||
		errors.Is(err, serviceexam.ErrLoginRequiredForInvite) ||
		errors.Is(err, serviceexam.ErrMaxAttemptsReached) ||
		errors.Is(err, serviceexam.ErrExamNotStarted) ||
		errors.Is(err, serviceexam.ErrExamEnded) ||
		errors.Is(err, serviceexam.ErrExamNotEligible) {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, err.Error()))
		return
	}
	c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "考试操作失败"))
}

func writeTakingServiceError(c *gin.Context, err error) {
	if errors.Is(err, serviceexam.ErrExamTokenInvalid) ||
		errors.Is(err, serviceexam.ErrExamTokenAttemptMismatch) ||
		errors.Is(err, serviceexam.ErrAnswerDeadlineExceeded) ||
		errors.Is(err, serviceexam.ErrAttemptAlreadySubmitted) {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, err.Error()))
		return
	}
	c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "作答操作失败"))
}

func writePermissionContextError(c *gin.Context, err error, fallback string) {
	if errors.Is(err, permission.ErrForbidden) {
		writePermissionOrInternalError(c, err, fallback)
		return
	}
	c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, err.Error()))
}

func writeVisibleResultError(c *gin.Context, err error) {
	if errors.Is(err, serviceexam.ErrResultNotVisible) {
		c.JSON(http.StatusForbidden, response.Fail(code.InvalidParam, "成绩暂未公布或无权查看"))
		return
	}
	if errors.Is(err, serviceexam.ErrResultNotFound) || errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, response.Fail(code.InvalidParam, "成绩不存在"))
		return
	}
	c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "读取成绩失败"))
}

func writeExamManagementDetailError(c *gin.Context, err error) {
	if errors.Is(err, permission.ErrForbidden) {
		c.JSON(http.StatusForbidden, response.Fail(code.InvalidParam, "无权查看当前考试"))
		return
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, response.Fail(code.InvalidParam, "考试不存在或已删除"))
		return
	}
	c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "读取考试详情失败"))
}

func writeExamManagementWriteError(c *gin.Context, err error) {
	if errors.Is(err, serviceexam.ErrExamAlreadyStarted) {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "考试已开始，不能导入考生"))
		return
	}
	if errors.Is(err, permission.ErrForbidden) {
		c.JSON(http.StatusForbidden, response.Fail(code.InvalidParam, "无权执行当前操作"))
		return
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, response.Fail(code.InvalidParam, "考试不存在或已删除"))
		return
	}
	c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "考试管理写操作失败"))
}

func (h examHandler) permissionContextForResultPublishConfig(c *gin.Context, tenantID uint64, examID uint64) (permission.PermissionContext, error) {
	principal, ok := currentAuthPrincipal(c)
	if !ok {
		return permission.PermissionContext{}, errors.New("请先登录")
	}
	if principal.SubjectType != permission.SubjectTenantUser || principal.TenantID != tenantID {
		return permission.PermissionContext{}, permission.ErrForbidden
	}
	if principal.Role == permission.RoleTenantAdmin {
		ctx, err := h.permissionContextFromSession(c, tenantID, 0)
		if err != nil {
			return permission.PermissionContext{}, err
		}
		return permissionContextWithExamScope(ctx, examID, 0), nil
	}

	spaceIDs, err := h.resultPublishTargetSpaceIDs(c.Request.Context(), tenantID, examID)
	if err != nil {
		return permission.PermissionContext{}, err
	}
	for _, spaceID := range spaceIDs {
		ctx, err := h.permissionContextFromSession(c, tenantID, spaceID)
		if err != nil {
			if errors.Is(err, permission.ErrForbidden) {
				continue
			}
			return permission.PermissionContext{}, err
		}
		ctx = permissionContextWithExamScope(ctx, examID, spaceID)
		if err := permission.NewFixedRoleChecker().CanViewExamResults(ctx, examID); err == nil {
			return ctx, nil
		}
	}
	return permission.PermissionContext{}, permission.ErrForbidden
}

func (h examHandler) resultPublishTargetSpaceIDs(ctx context.Context, tenantID uint64, examID uint64) ([]uint64, error) {
	targets, err := h.targets.ListTargets(ctx, tenantID, examID)
	if err != nil {
		return nil, err
	}
	seen := map[uint64]struct{}{}
	spaceIDs := make([]uint64, 0, len(targets))
	appendSpaceID := func(spaceID uint64) {
		if spaceID == 0 {
			return
		}
		if _, ok := seen[spaceID]; ok {
			return
		}
		seen[spaceID] = struct{}{}
		spaceIDs = append(spaceIDs, spaceID)
	}
	for _, target := range targets {
		switch target.TargetType {
		case serviceexam.TargetTypeSpace:
			appendSpaceID(target.TargetID)
		case serviceexam.TargetTypeUser:
			memberships, err := h.members.ListEffectiveMembershipsForUser(ctx, tenantID, target.TargetID)
			if err != nil {
				return nil, err
			}
			for _, membership := range memberships {
				if membership.Status == servicespace.StatusEnabled {
					appendSpaceID(membership.SpaceID)
				}
			}
		}
	}
	return spaceIDs, nil
}

func (h examHandler) readActorPermissionQuery(c *gin.Context, tenantID uint64) (permission.PermissionContext, error) {
	spaceID, err := readOptionalUintQueryValue(c, "space_id")
	if err != nil {
		return permission.PermissionContext{}, errors.New("space_id 必须是正整数")
	}
	return h.permissionContextFromSession(c, tenantID, spaceID)
}

func permissionContextWithExamScope(ctx permission.PermissionContext, examID uint64, spaceID uint64) permission.PermissionContext {
	next := ctx
	next.ExamScope = clonePermissionScope(ctx.ExamScope)
	next.ExamScope[examID] = spaceID
	return next
}

func clonePermissionScope(scope map[uint64]uint64) map[uint64]uint64 {
	next := make(map[uint64]uint64, len(scope)+1)
	for key, value := range scope {
		next[key] = value
	}
	return next
}

func readOptionalUintQueryValue(c *gin.Context, key string) (uint64, error) {
	raw := c.Query(key)
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || value == 0 {
		return 0, errors.New("invalid query")
	}
	return value, nil
}

func (h examHandler) permissionContextFromSession(c *gin.Context, tenantID uint64, spaceID uint64) (permission.PermissionContext, error) {
	principal, ok := currentAuthPrincipal(c)
	if !ok {
		return permission.PermissionContext{}, errors.New("请先登录")
	}
	ctx := permission.PermissionContext{
		SubjectType:      principal.SubjectType,
		UserID:           principal.UserID,
		TenantID:         tenantID,
		Role:             principal.Role,
		SpaceMemberships: map[uint64]string{},
		ExamScope:        map[uint64]uint64{},
		AttemptScope:     map[uint64]uint64{},
	}
	if principal.SubjectType == permission.SubjectPlatformUser {
		return permission.PermissionContext{}, permission.ErrForbidden
	}
	principal, err := h.liveTenantPrincipal(c, tenantID)
	if err != nil {
		return permission.PermissionContext{}, err
	}
	ctx.UserID = principal.UserID
	ctx.Role = principal.Role
	switch principal.Role {
	case permission.RoleTenantAdmin:
		return ctx, nil
	case permission.RoleTeacher:
		return h.permissionContextFromSpaceMembership(c, ctx, tenantID, spaceID, principal.UserID)
	case permission.RoleStudent:
		return h.permissionContextFromSpaceMembership(c, ctx, tenantID, spaceID, principal.UserID)
	default:
		return permission.PermissionContext{}, permission.ErrForbidden
	}
}

func (h examHandler) managementPermissionContext(c *gin.Context, tenantID uint64, spaceID uint64) (permission.PermissionContext, error) {
	// 管理端显式带 space_id 时，教师/空间管理员只能保留该空间身份，避免详情页把同租户其它空间一并放进权限上下文。
	if spaceID > 0 {
		return h.permissionContextFromSession(c, tenantID, spaceID)
	}
	principal, err := h.liveTenantPrincipal(c, tenantID)
	if err != nil {
		return permission.PermissionContext{}, err
	}
	ctx := permission.PermissionContext{
		SubjectType:      principal.SubjectType,
		UserID:           principal.UserID,
		TenantID:         tenantID,
		Role:             principal.Role,
		SpaceMemberships: map[uint64]string{},
		ExamScope:        map[uint64]uint64{},
		AttemptScope:     map[uint64]uint64{},
	}
	if principal.Role == permission.RoleTenantAdmin {
		return ctx, nil
	}
	memberships, err := h.members.ListEffectiveMembershipsForUser(c.Request.Context(), tenantID, principal.UserID)
	if err != nil {
		return permission.PermissionContext{}, err
	}
	// 详情页需要按考试目标自动裁剪多个空间，因此这里一次性注入当前用户全部启用空间身份。
	for _, membership := range memberships {
		if membership.Status == servicespace.StatusEnabled {
			ctx.SpaceMemberships[membership.SpaceID] = membership.Role
		}
	}
	return ctx, nil
}

func (h examHandler) managementWritePermissionContext(c *gin.Context, tenantID uint64, spaceID uint64) (permission.PermissionContext, error) {
	principal, err := h.liveTenantPrincipal(c, tenantID)
	if err != nil {
		return permission.PermissionContext{}, err
	}
	// 写接口不能在“展开全部空间身份”的模式下执行；非租户管理员必须显式声明当前操作空间。
	if principal.Role != permission.RoleTenantAdmin && spaceID == 0 {
		return permission.PermissionContext{}, errors.New("space_id 必须是正整数")
	}
	return h.managementPermissionContext(c, tenantID, spaceID)
}

func (h examHandler) liveTenantPrincipal(c *gin.Context, tenantID uint64) (AuthPrincipal, error) {
	principal, ok := currentAuthPrincipal(c)
	if !ok {
		return AuthPrincipal{}, errors.New("请先登录")
	}
	if principal.SubjectType != permission.SubjectTenantUser || principal.TenantID != tenantID {
		return AuthPrincipal{}, permission.ErrForbidden
	}
	if h.tenantUsers == nil {
		return principal, nil
	}
	user, err := h.tenantUsers.Get(c.Request.Context(), tenantID, principal.UserID)
	if errors.Is(err, servicetenantuser.ErrUserNotFound) {
		return AuthPrincipal{}, permission.ErrForbidden
	}
	if err != nil {
		return AuthPrincipal{}, err
	}
	if user.Status != servicetenantuser.StatusEnabled || user.Role != principal.Role {
		return AuthPrincipal{}, permission.ErrForbidden
	}
	return principal, nil
}

func (h examHandler) permissionContextFromSpaceMembership(c *gin.Context, ctx permission.PermissionContext, tenantID uint64, spaceID uint64, userID uint64) (permission.PermissionContext, error) {
	if spaceID == 0 {
		return permission.PermissionContext{}, errors.New("space_id 必须是正整数")
	}
	member, err := h.members.FindMember(c.Request.Context(), tenantID, spaceID, userID)
	if errors.Is(err, servicespace.ErrMemberNotFound) {
		return permission.PermissionContext{}, permission.ErrForbidden
	}
	if err != nil {
		return permission.PermissionContext{}, err
	}
	if member.Status != servicespace.StatusEnabled {
		return permission.PermissionContext{}, permission.ErrForbidden
	}
	ctx.SpaceMemberships[spaceID] = member.Role
	return ctx, nil
}

func examToResponse(exam serviceexam.Exam) examResponse {
	targets := make([]examManagementTargetResponse, 0, len(exam.Targets))
	for _, target := range exam.Targets {
		targets = append(targets, examManagementTargetResponse{
			TargetType:    target.TargetType,
			TargetID:      target.TargetID,
			ScopeSpaceIDs: target.ScopeSpaceIDs,
		})
	}
	return examResponse{
		ID:               exam.ID,
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
		InviteCode:       examInviteCodeForResponse(exam),
		Status:           exam.Status,
		CreatedBy:        exam.CreatedBy,
		TargetType:       exam.TargetType,
		TargetID:         exam.TargetID,
		Targets:          targets,
	}
}

func examInviteCodeForResponse(exam serviceexam.Exam) string {
	if exam.Status == serviceexam.StatusDraft {
		return ""
	}
	return exam.InviteCode
}

func examManagementDetailToResponse(detail serviceexam.ManagementDetail) examManagementDetailResponse {
	targets := make([]examManagementTargetResponse, 0, len(detail.Targets))
	for _, target := range detail.Targets {
		targets = append(targets, examManagementTargetResponse{
			TargetType:    target.TargetType,
			TargetID:      target.TargetID,
			ScopeSpaceIDs: target.ScopeSpaceIDs,
		})
	}
	return examManagementDetailResponse{
		Exam:            examToResponse(detail.Exam),
		Targets:         targets,
		TargetSpaceIDs:  detail.TargetSpaceIDs,
		AllowedSpaceIDs: detail.AllowedSpaceIDs,
		Permissions:     managementPermissionsToResponse(detail.Permissions),
	}
}

func examOverviewToResponse(overview serviceexam.ExamOverviewData) examOverviewResponse {
	questionTypes := make([]examOverviewQuestionTypeResponse, 0, len(overview.QuestionTypes))
	for _, item := range overview.QuestionTypes {
		questionTypes = append(questionTypes, examOverviewQuestionTypeResponse{
			QuestionType:  item.QuestionType,
			QuestionCount: item.QuestionCount,
			TotalScore:    item.TotalScore,
		})
	}
	activities := make([]examOverviewActivityResponse, 0, len(overview.RecentActivities))
	for _, item := range overview.RecentActivities {
		activities = append(activities, examOverviewActivityResponse{
			OperationType:   item.OperationType,
			OperationTitle:  item.OperationTitle,
			OperationDetail: item.OperationDetail,
			CreatedAt:       item.CreatedAt,
		})
	}
	return examOverviewResponse{
		Exam: examToResponse(overview.Detail.Exam),
		CandidateStats: examOverviewCandidateStatsResponse{
			Planned:    overview.CandidateStats.Planned,
			Joined:     overview.CandidateStats.Joined,
			Submitted:  overview.CandidateStats.Submitted,
			InProgress: overview.CandidateStats.InProgress,
		},
		QuestionTypes:    questionTypes,
		RecentActivities: activities,
		Permissions:      managementPermissionsToResponse(overview.Detail.Permissions),
	}
}

func examPaperPreviewToResponse(preview serviceexam.ExamPaperPreviewData) examPaperPreviewResponse {
	sections := make([]examPaperPreviewSectionResponse, 0, len(preview.Sections))
	for _, section := range preview.Sections {
		sections = append(sections, examPaperPreviewSectionResponse{
			SectionID:     section.SectionID,
			SectionName:   section.SectionName,
			QuestionType:  section.QuestionType,
			QuestionCount: section.QuestionCount,
			TotalScore:    section.TotalScore,
		})
	}
	items := make([]examPaperPreviewQuestionResponse, 0, len(preview.Items))
	for _, item := range preview.Items {
		options := make([]examPaperPreviewOptionResponse, 0, len(item.Options))
		for _, option := range item.Options {
			options = append(options, examPaperPreviewOptionResponse{
				ID:      option.ID,
				Key:     option.Key,
				Content: option.Content,
			})
		}
		items = append(items, examPaperPreviewQuestionResponse{
			SectionID:    item.SectionID,
			SectionName:  item.SectionName,
			QuestionID:   item.QuestionID,
			QuestionType: item.QuestionType,
			Title:        item.Title,
			Score:        item.Score,
			BlankCount:   item.BlankCount,
			SortOrder:    item.SortOrder,
			Options:      options,
		})
	}
	return examPaperPreviewResponse{
		Exam:        examToResponse(preview.Detail.Exam),
		Sections:    sections,
		Items:       items,
		Page:        preview.Page,
		PageSize:    preview.PageSize,
		Total:       preview.Total,
		Permissions: managementPermissionsToResponse(preview.Detail.Permissions),
	}
}

// examCandidatesToResponse 将 service 层考生分页结果转换为 HTTP 响应，避免 handler 泄露内部权限上下文结构。
func examCandidatesToResponse(candidates serviceexam.CandidateListData) examCandidateListResponse {
	items := make([]examCandidateResponse, 0, len(candidates.Items))
	for _, item := range candidates.Items {
		sources := make([]examCandidateSourceTargetResponse, 0, len(item.SourceTargets))
		for _, source := range item.SourceTargets {
			sources = append(sources, examCandidateSourceTargetResponse{
				TargetType: source.TargetType,
				TargetID:   source.TargetID,
				SpaceID:    source.SpaceID,
				SpaceName:  source.SpaceName,
			})
		}
		items = append(items, examCandidateResponse{
			UserID:           item.UserID,
			Username:         item.Username,
			RealName:         item.RealName,
			SpaceID:          item.SpaceID,
			SpaceName:        item.SpaceName,
			Status:           item.Status,
			StartedAt:        item.StartedAt,
			SubmittedAt:      item.SubmittedAt,
			TotalScore:       item.TotalScore,
			AttemptCount:     item.AttemptCount,
			CurrentAttemptID: item.CurrentAttemptID,
			ResultAttemptID:  item.ResultAttemptID,
			SourceTargets:    sources,
		})
	}
	return examCandidateListResponse{
		Exam:        examToResponse(candidates.Detail.Exam),
		Items:       items,
		Page:        candidates.Page,
		PageSize:    candidates.PageSize,
		Total:       candidates.Total,
		Permissions: managementPermissionsToResponse(candidates.Detail.Permissions),
	}
}

// examResultsSummaryToResponse 将成绩摘要 service 数据转换为前端约定的 snake_case JSON。
func examResultsSummaryToResponse(summary serviceexam.ResultSummaryData) examResultSummaryResponse {
	scoreDistribution := make([]examResultScoreDistributionResponse, 0, len(summary.ScoreDistribution))
	for _, item := range summary.ScoreDistribution {
		scoreDistribution = append(scoreDistribution, examResultScoreDistributionResponse{
			Label: item.Label,
			Count: item.Count,
		})
	}
	questionTypeRates := make([]examResultQuestionTypeRateResponse, 0, len(summary.QuestionTypeRates))
	for _, item := range summary.QuestionTypeRates {
		questionTypeRates = append(questionTypeRates, examResultQuestionTypeRateResponse{
			QuestionType:      item.QuestionType,
			QuestionTypeLabel: item.QuestionTypeLabel,
			AverageRate:       item.AverageRate,
		})
	}
	return examResultSummaryResponse{
		Exam: examToResponse(summary.Detail.Exam),
		Stats: examResultSummaryStatsResponse{
			Submitted:         summary.Stats.Submitted,
			AverageScore:      summary.Stats.AverageScore,
			HighestScore:      summary.Stats.HighestScore,
			PassRate:          summary.Stats.PassRate,
			PendingSubjective: summary.Stats.PendingSubjective,
		},
		ScoreDistribution: scoreDistribution,
		QuestionTypeRates: questionTypeRates,
		Permissions:       managementPermissionsToResponse(summary.Detail.Permissions),
	}
}

// examManagementResultsToResponse 将成绩管理分页列表转换为 HTTP 响应。
func examManagementResultsToResponse(results serviceexam.ResultListData) examManagementResultListResponse {
	items := make([]examManagementResultResponse, 0, len(results.Items))
	for _, item := range results.Items {
		items = append(items, examManagementResultResponse{
			Rank:            item.Rank,
			AttemptID:       item.AttemptID,
			UserID:          item.UserID,
			Username:        item.Username,
			RealName:        item.RealName,
			SpaceID:         item.SpaceID,
			SpaceName:       item.SpaceName,
			ObjectiveScore:  item.ObjectiveScore,
			SubjectiveScore: item.SubjectiveScore,
			TotalScore:      item.TotalScore,
			Status:          item.Status,
			SubmittedAt:     item.SubmittedAt,
		})
	}
	return examManagementResultListResponse{
		Exam:        examToResponse(results.Detail.Exam),
		Items:       items,
		Page:        results.Page,
		PageSize:    results.PageSize,
		Total:       results.Total,
		Permissions: managementPermissionsToResponse(results.Detail.Permissions),
	}
}

// examAnswerSheetToResponse 将管理端答卷详情转换为 HTTP 响应。
// 标准答案和学生答案只在该管理端深层接口返回，普通试卷预览仍不暴露答案。
func examAnswerSheetToResponse(answerSheet serviceexam.AnswerSheetData) examAnswerSheetResponse {
	items := make([]examAnswerSheetItemResponse, 0, len(answerSheet.Items))
	for _, item := range answerSheet.Items {
		items = append(items, examAnswerSheetItemResponse{
			AttemptQuestionID:     item.AttemptQuestionID,
			SectionID:             item.SectionID,
			QuestionID:            item.QuestionID,
			SortOrder:             item.SortOrder,
			SectionSnapshot:       item.SectionSnapshot,
			QuestionSnapshot:      item.QuestionSnapshot,
			QuestionType:          item.QuestionType,
			QuestionTitle:         item.QuestionTitle,
			OptionSnapshot:        item.OptionSnapshot,
			CorrectAnswerSnapshot: item.CorrectAnswerSnapshot,
			Score:                 item.Score,
			AnswerContent:         item.AnswerContent,
			AnswerScore:           item.AnswerScore,
			GradingStatus:         item.GradingStatus,
			GraderComment:         item.GraderComment,
			GradedBy:              item.GradedBy,
			GradedAt:              item.GradedAt,
		})
	}
	return examAnswerSheetResponse{
		Exam: examToResponse(answerSheet.Detail.Exam),
		Attempt: examAnswerSheetAttemptResponse{
			AttemptID:       answerSheet.Attempt.AttemptID,
			ExamID:          answerSheet.Attempt.ExamID,
			UserID:          answerSheet.Attempt.UserID,
			Username:        answerSheet.Attempt.Username,
			RealName:        answerSheet.Attempt.RealName,
			ObjectiveScore:  answerSheet.Attempt.ObjectiveScore,
			SubjectiveScore: answerSheet.Attempt.SubjectiveScore,
			TotalScore:      answerSheet.Attempt.TotalScore,
			SubmittedAt:     answerSheet.Attempt.SubmittedAt,
		},
		Items:       items,
		Permissions: managementPermissionsToResponse(answerSheet.Detail.Permissions),
	}
}

// examOperationLogsToResponse 将操作日志分页结果转换为前端可消费的审计列表。
// 日志只返回展示和追溯所需字段，不透出 ext_json 等内部审计扩展数据。
func examOperationLogsToResponse(logs serviceexam.OperationLogListData) examOperationLogListResponse {
	items := make([]examOperationLogResponse, 0, len(logs.Items))
	for _, item := range logs.Items {
		items = append(items, examOperationLogResponse{
			ID:               item.ID,
			OperationType:    item.OperationType,
			OperationTitle:   item.OperationTitle,
			OperationDetail:  item.OperationDetail,
			ActorID:          item.ActorID,
			ActorType:        item.ActorType,
			ActorRole:        item.ActorRole,
			OperationGroupID: operationGroupIDFromExtJSON(item.ExtJSON),
			SpaceID:          item.SpaceID,
			CreatedAt:        item.CreatedAt,
		})
	}
	return examOperationLogListResponse{
		Exam:        examToResponse(logs.Detail.Exam),
		Items:       items,
		Page:        logs.Page,
		PageSize:    logs.PageSize,
		Total:       logs.Total,
		Permissions: managementPermissionsToResponse(logs.Detail.Permissions),
	}
}

// operationGroupIDFromExtJSON 只从审计扩展字段中提取可用于前端聚合的组 ID。
// 完整 ext_json 可能继续承载内部审计元数据，不能直接透出给前端。
func operationGroupIDFromExtJSON(extJSON string) string {
	var payload struct {
		OperationGroupID string `json:"operation_group_id"`
	}
	if err := json.Unmarshal([]byte(extJSON), &payload); err != nil {
		return ""
	}
	return payload.OperationGroupID
}

func examImportCandidatesToResponse(result serviceexam.ImportCandidatesResult) examCandidateImportResponse {
	return examCandidateImportResponse{
		ImportedCount: result.ImportedCount,
		SkippedCount:  result.SkippedCount,
		Permissions:   managementPermissionsToResponse(result.Detail.Permissions),
	}
}

func examResendInvitationsToResponse(result serviceexam.ResendInvitationsResult) examInvitationResendResponse {
	return examInvitationResendResponse{
		SentCount:    result.SentCount,
		SkippedCount: result.SkippedCount,
		InviteCode:   result.InviteCode,
		Permissions:  managementPermissionsToResponse(result.Detail.Permissions),
	}
}

func managementPermissionsToResponse(permissions serviceexam.ManagementPermissions) examManagementPermissionsResponse {
	return examManagementPermissionsResponse{
		CanViewDetail:       permissions.CanViewDetail,
		CanViewOverview:     permissions.CanViewOverview,
		CanViewPaper:        permissions.CanViewPaper,
		CanViewCandidates:   permissions.CanViewCandidates,
		CanManageCandidates: permissions.CanManageCandidates,
		CanViewResults:      permissions.CanViewResults,
		CanExportResults:    permissions.CanExportResults,
		CanPublishResults:   permissions.CanPublishResults,
		CanUpdateSettings:   permissions.CanUpdateSettings,
		CanViewLogs:         permissions.CanViewLogs,
	}
}

func pendingReviewToResponse(item serviceexam.PendingAttempt) pendingReviewResponse {
	return pendingReviewResponse{
		AttemptID:             item.AttemptID,
		AttemptQuestionID:     item.AttemptQuestionID,
		StudentName:           item.StudentName,
		SpaceName:             item.SpaceName,
		ExamName:              item.ExamName,
		QuestionTitle:         item.QuestionTitle,
		AnswerContent:         item.AnswerContent,
		SubmittedAt:           item.SubmittedAt,
		MaxScore:              item.MaxScore,
		AnswerVersion:         item.AnswerVersion,
		PendingShortTextCount: item.PendingShortTextCount,
		Status:                constant.GradingStatusPending,
	}
}

func scoreRowToResponse(id uint64, row serviceexam.ScoreExportRow) resultResponse {
	return resultResponse{
		ID:              id,
		StudentName:     row.StudentName,
		SpaceName:       row.SpaceName,
		AttemptNo:       row.AttemptNo,
		ObjectiveScore:  row.ObjectiveScore,
		SubjectiveScore: row.SubjectiveScore,
		TotalScore:      row.TotalScore,
		SubmittedAt:     row.SubmittedAt,
		Status:          "可发布",
	}
}

func visibleResultToResponse(result serviceexam.VisibleResult) visibleResultResponse {
	return visibleResultResponse{
		AttemptID:       result.AttemptID,
		ExamID:          result.ExamID,
		AttemptNo:       result.AttemptNo,
		ObjectiveScore:  result.ObjectiveScore,
		SubjectiveScore: result.SubjectiveScore,
		TotalScore:      result.TotalScore,
		AnalysisVisible: result.AnalysisVisible,
	}
}

func attemptToResponse(attempt serviceexam.Attempt, exam serviceexam.Exam) attemptResponse {
	return attemptResponse{
		ID:             attempt.ID,
		ExamID:         attempt.ExamID,
		UserID:         attempt.UserID,
		AttemptNo:      attempt.AttemptNo,
		Status:         attempt.Status,
		StartedAt:      attempt.StartedAt,
		AnswerDeadline: answerDeadlineForResponse(attempt.StartedAt, exam),
	}
}

func answerDeadlineForResponse(startedAt int64, exam serviceexam.Exam) int64 {
	durationDeadline := startedAt + int64(exam.DurationMinutes)*60_000
	if durationDeadline < exam.EndTime {
		return durationDeadline
	}
	return exam.EndTime
}

func attemptQuestionsToResponse(questions []serviceexam.AttemptQuestion) ([]attemptQuestionResponse, error) {
	items := make([]attemptQuestionResponse, 0, len(questions))
	for _, question := range questions {
		item, err := attemptQuestionToResponse(question)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func attemptQuestionToResponse(question serviceexam.AttemptQuestion) (attemptQuestionResponse, error) {
	section := sectionSnapshotResponse{}
	if err := json.Unmarshal([]byte(question.SectionSnapshot), &section); err != nil {
		return attemptQuestionResponse{}, err
	}
	questionSnapshot := questionSnapshotResponse{}
	if err := json.Unmarshal([]byte(question.QuestionSnapshot), &questionSnapshot); err != nil {
		return attemptQuestionResponse{}, err
	}
	var optionSnapshot struct {
		Options []optionSnapshotResponse `json:"options"`
	}
	if err := json.Unmarshal([]byte(question.OptionSnapshot), &optionSnapshot); err != nil {
		return attemptQuestionResponse{}, err
	}
	return attemptQuestionResponse{
		ID:        question.ID,
		SortOrder: question.SortOrder,
		Section:   section,
		Question:  questionSnapshot,
		Options:   optionSnapshot.Options,
		Score:     question.Score,
	}, nil
}

package v1

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

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
	service     *serviceexam.Service
	taking      *serviceexam.TakingService
	review      *serviceexam.ReviewService
	export      *serviceexam.ExportService
	result      *serviceexam.ResultService
	papers      paperScopeFinder
	targets     examTargetFinder
	members     spaceMemberFinder
	tenantUsers *servicetenantuser.Service
	now         func() int64
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

type publishExamRequest struct {
	TenantID         uint64 `json:"tenant_id"`
	PaperID          uint64 `json:"paper_id"`
	Name             string `json:"name"`
	TargetType       string `json:"target_type"`
	TargetID         uint64 `json:"target_id"`
	StartTime        int64  `json:"start_time"`
	EndTime          int64  `json:"end_time"`
	DurationMinutes  int    `json:"duration_minutes"`
	MaxAttempts      int    `json:"max_attempts"`
	ResultStrategy   string `json:"result_strategy"`
	PublishMode      string `json:"publish_mode"`
	ScorePublishTime *int64 `json:"score_publish_time"`
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

type exportResultsRequest struct {
	actorPermissionRequest
	TenantID uint64 `json:"tenant_id"`
	ExamID   uint64 `json:"exam_id"`
}

type examResponse struct {
	ID               uint64 `json:"id"`
	TenantID         uint64 `json:"tenant_id"`
	PaperID          uint64 `json:"paper_id"`
	Name             string `json:"name"`
	StartTime        int64  `json:"start_time"`
	EndTime          int64  `json:"end_time"`
	DurationMinutes  int    `json:"duration_minutes"`
	MaxAttempts      int    `json:"max_attempts"`
	ResultStrategy   string `json:"result_strategy"`
	PublishMode      string `json:"publish_mode"`
	ScorePublishTime *int64 `json:"score_publish_time"`
	InviteCode       string `json:"invite_code"`
	Status           string `json:"status"`
	TargetType       string `json:"target_type,omitempty"`
	TargetID         uint64 `json:"target_id,omitempty"`
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
	Title string `json:"title"`
	Type  string `json:"type"`
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
	if !authorizeExamBusiness(c, tenantID, h.members) {
		return
	}
	page, pageSize, err := readPaginationQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "page 和 page_size 必须是正整数"))
		return
	}
	result, err := h.service.List(c.Request.Context(), serviceexam.ListInput{TenantID: tenantID, Page: page, PageSize: pageSize})
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
	if !authorizeExamBusiness(c, request.TenantID, h.members) {
		return
	}
	if !h.authorizePublishScope(c, request.TenantID, request.PaperID, request.TargetType, request.TargetID) {
		return
	}
	draft, err := h.service.CreateDraft(c.Request.Context(), serviceexam.CreateDraftInput{
		TenantID: request.TenantID,
		PaperID:  request.PaperID,
		Name:     request.Name,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "创建考试草稿失败"))
		return
	}
	published, err := h.service.Publish(c.Request.Context(), serviceexam.PublishInput{
		TenantID:         request.TenantID,
		ExamID:           draft.ID,
		PaperID:          request.PaperID,
		StartTime:        request.StartTime,
		EndTime:          request.EndTime,
		DurationMinutes:  request.DurationMinutes,
		MaxAttempts:      request.MaxAttempts,
		ResultStrategy:   request.ResultStrategy,
		PublishMode:      request.PublishMode,
		ScorePublishTime: request.ScorePublishTime,
	})
	if err != nil {
		writeExamServiceError(c, err)
		return
	}
	if err := h.service.AddTarget(c.Request.Context(), serviceexam.AddTargetInput{
		TenantID:   request.TenantID,
		ExamID:     published.ID,
		TargetType: request.TargetType,
		TargetID:   request.TargetID,
	}); err != nil {
		writeExamServiceError(c, err)
		return
	}
	result := examToResponse(published)
	result.TargetType = request.TargetType
	result.TargetID = request.TargetID
	c.JSON(http.StatusOK, response.OK(result))
}

func (h examHandler) authorizePublishScope(c *gin.Context, tenantID uint64, paperID uint64, targetType string, targetID uint64) bool {
	spaceID, err := h.papers.GetPaperSpaceID(c.Request.Context(), tenantID, paperID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "读取考试试卷权限范围失败"))
		return false
	}
	permissionContext, err := permissionContextForResourceScope(c, tenantID, spaceID, h.members)
	if err != nil {
		writePermissionOrInternalError(c, err, "构建考试发布权限上下文失败")
		return false
	}
	if spaceID == nil {
		if permissionContext.Role != permission.RoleTenantAdmin {
			writePermissionOrInternalError(c, permission.ErrForbidden, "公共试卷仅租户管理员可发布")
			return false
		}
	} else {
		permissionContext.PaperScope = map[uint64]uint64{paperID: *spaceID}
		if err := permission.NewFixedRoleChecker().CanPublishExam(permissionContext, paperID); err != nil {
			writePermissionOrInternalError(c, err, "校验考试发布权限失败")
			return false
		}
	}
	switch targetType {
	case serviceexam.TargetTypeSpace:
		return h.authorizePublishSpaceTarget(c, tenantID, targetID)
	case serviceexam.TargetTypeUser:
		return h.authorizePublishUserTarget(c, tenantID, targetID)
	default:
		writePermissionOrInternalError(c, permission.ErrForbidden, "校验考试发布目标失败")
		return false
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
	permissionContext, err := permissionContextForResourceScope(c, tenantID, &spaceID, h.members)
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

func (h examHandler) authorizePublishUserTarget(c *gin.Context, tenantID uint64, userID uint64) bool {
	principal, ok := currentAuthPrincipal(c)
	if !ok {
		writePermissionOrInternalError(c, permission.ErrForbidden, "校验考试发布目标失败")
		return false
	}
	user, err := h.tenantUsers.Get(c.Request.Context(), tenantID, userID)
	if errors.Is(err, servicetenantuser.ErrUserNotFound) {
		writePermissionOrInternalError(c, permission.ErrForbidden, "校验考试发布目标失败")
		return false
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "读取考试发布目标失败"))
		return false
	}
	if user.Status != servicetenantuser.StatusEnabled || user.Role != servicetenantuser.RoleStudent {
		writePermissionOrInternalError(c, permission.ErrForbidden, "校验考试发布目标失败")
		return false
	}
	if principal.SubjectType == permission.SubjectTenantUser && principal.TenantID == tenantID && principal.Role == permission.RoleTenantAdmin {
		return true
	}
	memberships, err := h.members.ListEffectiveMembershipsForUser(c.Request.Context(), tenantID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "读取考试发布目标范围失败"))
		return false
	}
	for _, membership := range memberships {
		if h.actorCanPublishToSpace(c, tenantID, membership.SpaceID) {
			return true
		}
	}
	writePermissionOrInternalError(c, permission.ErrForbidden, "校验考试发布目标失败")
	return false
}

func (h examHandler) actorCanPublishToSpace(c *gin.Context, tenantID uint64, spaceID uint64) bool {
	permissionContext, err := permissionContextForResourceScope(c, tenantID, &spaceID, h.members)
	if err != nil {
		return false
	}
	if permissionContext.Role == permission.RoleTenantAdmin {
		return true
	}
	role := permissionContext.SpaceMemberships[spaceID]
	return role == permission.RoleSpaceAdmin || role == permission.RoleTeacher
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
	if err := saveExamEntrySession(c, exam, principal.UserID); err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "保存考试入口会话失败"))
		return
	}
	c.JSON(http.StatusOK, response.OK(examToResponse(exam)))
}

func saveExamEntrySession(c *gin.Context, exam serviceexam.Exam, userID uint64) error {
	session := sessions.Default(c)
	session.Set(examEntrySessionTenantIDKey, exam.TenantID)
	session.Set(examEntrySessionExamIDKey, exam.ID)
	session.Set(examEntrySessionUserIDKey, userID)
	session.Options(sessions.Options{
		Path:     "/",
		MaxAge:   int(24 * time.Hour / time.Second),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return session.Save()
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
	permissionContext := permission.PermissionContext{
		SubjectType: principal.SubjectType,
		UserID:      principal.UserID,
		TenantID:    principal.TenantID,
		Role:        principal.Role,
	}
	visible, err := h.result.GetVisibleResult(c.Request.Context(), serviceexam.ResultQueryInput{
		Permission: permissionContext,
		TenantID:   principal.TenantID,
		AttemptID:  attemptID,
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
	permissionContext, err := h.permissionContextForResultPublishConfig(c, request.TenantID, request.ExamID)
	if err != nil {
		writePermissionContextError(c, err, "保存成绩发布配置失败")
		return
	}
	if err := permission.NewFixedRoleChecker().CanViewExamResults(permissionContext, request.ExamID); err != nil {
		writePermissionOrInternalError(c, err, "保存成绩发布配置失败")
		return
	}
	// 成绩可见性由后端统一以 publish_mode 和 score_publish_time 判断，前端只提交配置。
	exam, err := h.service.UpdateScorePublishConfig(c.Request.Context(), serviceexam.UpdateScorePublishConfigInput{
		TenantID:         request.TenantID,
		ExamID:           request.ExamID,
		PublishMode:      request.PublishMode,
		ScorePublishTime: request.ScorePublishTime,
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
	c.JSON(http.StatusOK, response.OK(resultExportResponse{FilePath: result.FilePath, RowCount: result.RowCount}))
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
	if r.TargetType != serviceexam.TargetTypeSpace && r.TargetType != serviceexam.TargetTypeUser {
		return errors.New("target_type 只能是 space 或 user")
	}
	if r.TargetID == 0 {
		return errors.New("target_id 必须是正整数")
	}
	if r.DurationMinutes <= 0 {
		return errors.New("duration_minutes 必须是正整数")
	}
	if r.MaxAttempts <= 0 {
		return errors.New("max_attempts 必须是正整数")
	}
	if r.ResultStrategy == "" {
		return errors.New("result_strategy 不能为空")
	}
	if r.PublishMode == "" {
		return errors.New("publish_mode 不能为空")
	}
	if r.StartTime <= 0 || r.EndTime <= r.StartTime {
		return errors.New("考试时间范围不合法")
	}
	return nil
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
		errors.Is(err, serviceexam.ErrDuplicateExamTarget) ||
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
	if principal.SubjectType != permission.SubjectTenantUser || principal.TenantID != tenantID {
		return permission.PermissionContext{}, permission.ErrForbidden
	}
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
		InviteCode:       exam.InviteCode,
		Status:           exam.Status,
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

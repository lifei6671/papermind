package v1

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/lifei6671/papermind/server/api/middleware"
	dbdao "github.com/lifei6671/papermind/server/internal/dao/db"
	serviceexam "github.com/lifei6671/papermind/server/internal/service/exam"
	"github.com/lifei6671/papermind/server/internal/service/pagination"
	servicepaper "github.com/lifei6671/papermind/server/internal/service/paper"
	"github.com/lifei6671/papermind/server/internal/service/permission"
	serviceplatformuser "github.com/lifei6671/papermind/server/internal/service/platformuser"
	servicequestion "github.com/lifei6671/papermind/server/internal/service/question"
	servicespace "github.com/lifei6671/papermind/server/internal/service/space"
	servicetenant "github.com/lifei6671/papermind/server/internal/service/tenant"
	servicetenantuser "github.com/lifei6671/papermind/server/internal/service/tenantuser"
	"github.com/lifei6671/papermind/server/internal/storage"
	"github.com/lifei6671/papermind/server/library/code"
	"github.com/lifei6671/papermind/server/library/constant"
	"github.com/lifei6671/papermind/server/library/crypto"
	"github.com/lifei6671/papermind/server/library/response"
	"gorm.io/gorm"
)

const (
	examEntrySessionTenantIDKey = "exam_entry_tenant_id"
	examEntrySessionExamIDKey   = "exam_entry_exam_id"
	examEntrySessionUserIDKey   = "exam_entry_user_id"
)

type RouterOptions struct {
	DB                   *gorm.DB
	Now                  func() int64
	CodeGenerator        serviceexam.CodeGenerator
	AllowRegisterDefault bool
	AuthSessionStore     sessions.Store
	AuthSessionProvider  string
	AuthSessionSecret    string
	AuthSessionKeyPrefix string
	AuthSessionRedisAddr string
	AuthSessionRedisUser string
	AuthSessionRedisPass string
	AuthSessionRedisDB   int
	AuthSessionTTL       int
	ExportDir            string
	UploadDir            string
	UploadStore          storage.ObjectStore
	PasswordMinLength    int
}

func NewRouter(options RouterOptions) *gin.Engine {
	platformUserRepository := dbdao.NewPlatformUserRepository(options.DB, dbdao.PlatformUserRepositoryOptions{Now: options.Now})
	platformUserService := serviceplatformuser.NewService(serviceplatformuser.ServiceOptions{
		Repo: platformUserRepository,
		Now:  options.Now,
	})
	examRepository := dbdao.NewExamRepository(options.DB, dbdao.ExamRepositoryOptions{Now: options.Now})
	examService := serviceexam.NewService(serviceexam.ServiceOptions{
		Repo:          examRepository,
		Now:           options.Now,
		CodeGenerator: options.CodeGenerator,
	})
	takingService := serviceexam.NewTakingService(serviceexam.TakingServiceOptions{
		Repo: examRepository,
		Now:  options.Now,
	})
	takingService.StartEventConsumer(context.Background())
	reviewService := serviceexam.NewReviewService(serviceexam.ReviewServiceOptions{
		Repo:              examRepository,
		PermissionChecker: permission.NewFixedRoleChecker(),
		Now:               options.Now,
	})
	exportService := serviceexam.NewExportService(serviceexam.ExportServiceOptions{
		Repo:              examRepository,
		PermissionChecker: permission.NewFixedRoleChecker(),
		ExportDir:         defaultExportDir(options.ExportDir),
		Now:               options.Now,
	})
	resultService := serviceexam.NewResultService(serviceexam.ResultServiceOptions{
		Repo:              examRepository,
		PermissionChecker: permission.NewFixedRoleChecker(),
		Now:               options.Now,
	})
	tenantRepository := dbdao.NewTenantRepository(options.DB, dbdao.TenantRepositoryOptions{Now: options.Now})
	tenantService := servicetenant.NewService(servicetenant.ServiceOptions{
		Repo:                 tenantRepository,
		CodeGenerator:        options.CodeGenerator,
		AllowRegisterDefault: options.AllowRegisterDefault,
	})
	spaceRepository := dbdao.NewSpaceRepository(options.DB, dbdao.SpaceRepositoryOptions{Now: options.Now})
	spaceService := servicespace.NewService(servicespace.ServiceOptions{Repo: spaceRepository})
	questionRepository := dbdao.NewQuestionRepository(options.DB, dbdao.QuestionRepositoryOptions{Now: options.Now})
	questionService := servicequestion.NewQuestionService(servicequestion.QuestionServiceOptions{Repo: questionRepository})
	paperRepository := dbdao.NewPaperRepository(options.DB, dbdao.PaperRepositoryOptions{Now: options.Now})
	paperService := servicepaper.NewService(servicepaper.ServiceOptions{Repo: paperRepository})
	userRepository := dbdao.NewTenantUserRepository(options.DB, dbdao.TenantUserRepositoryOptions{Now: options.Now})
	userService := servicetenantuser.NewService(servicetenantuser.ServiceOptions{
		Repo:                       userRepository,
		SpaceAdminInvariantChecker: spaceRepository,
		Now:                        options.Now,
	})
	examHandler := examHandler{service: examService, taking: takingService, review: reviewService, export: exportService, result: resultService, members: spaceRepository, now: defaultRouterNow(options.Now)}
	tenantHandler := tenantHandler{service: tenantService}
	spaceHandler := spaceHandler{service: spaceService, members: spaceRepository}
	userHandler := userHandler{service: userService, passwordMinLength: options.PasswordMinLength}
	questionHandler := questionHandler{service: questionService, members: spaceRepository}
	paperHandler := paperHandler{service: paperService, papers: paperRepository, members: spaceRepository}
	authHandler := authHandler{platformUsers: platformUserService, tenantUsers: userService, sessionMaxAgeSeconds: options.AuthSessionTTL, passwordMinLength: options.PasswordMinLength}
	uploadDir := defaultUploadDir(options.UploadDir)
	uploadHandler := uploadHandler{store: defaultUploadStore(options.UploadStore, uploadDir), now: time.Now}
	sessionStore := defaultAuthSessionStore(options)

	router := gin.New()
	router.Use(middleware.RequestID(), middleware.RequestLogger(), middleware.Recovery())
	router.Use(bearerSessionCookieMiddleware(authSessionName), sessions.Sessions(authSessionName, sessionStore), authContextMiddleware())
	router.StaticFS("/uploads", gin.Dir(uploadDir, false))
	api := router.Group("/api/v1")
	api.POST("/auth/platform/login", authHandler.platformLogin)
	api.POST("/auth/tenant/login", authHandler.tenantLogin)
	api.POST("/auth/tenant/register", authHandler.tenantRegister)
	api.GET("/profile", requireAuthPrincipalMiddleware(), authHandler.getProfile)
	api.POST("/profile", requireAuthPrincipalMiddleware(), authHandler.updateProfile)
	api.GET("/exams", requireExamBusinessPrincipalMiddleware(), examHandler.list)
	api.POST("/exams", requireExamBusinessPrincipalMiddleware(), examHandler.publish)
	api.POST("/exams/invite/resolve", examHandler.resolveInvite)
	api.POST("/exams/:id/attempts/start", examHandler.startAttempt)
	api.POST("/exam-attempts/:attempt_id/answers/:attempt_question_id", examHandler.saveAnswer)
	api.POST("/exam-attempts/:attempt_id/submit", examHandler.submitAttempt)
	api.POST("/exam-attempts/:attempt_id/events", examHandler.recordEvent)
	api.POST("/exam-entry/invite/resolve", examHandler.resolveInvite)
	api.POST("/exam-entry/exams/:id/attempts/start", examHandler.startAttempt)
	api.POST("/exam-entry/attempts/:attempt_id/answers/:attempt_question_id", examHandler.saveAnswer)
	api.POST("/exam-entry/attempts/:attempt_id/submit", examHandler.submitAttempt)
	api.POST("/exam-entry/attempts/:attempt_id/events", examHandler.recordEvent)
	api.GET("/exam-entry/results/:id", requireAuthPrincipalMiddleware(), examHandler.getExamEntryResult)
	api.GET("/grading/pending", requireExamBusinessPrincipalMiddleware(), examHandler.listPendingReviews)
	api.POST("/exam-attempts/:attempt_id/questions/:attempt_question_id/grade", requireExamBusinessPrincipalMiddleware(), examHandler.gradeShortText)
	api.GET("/results", requireExamBusinessPrincipalMiddleware(), examHandler.listResults)
	api.POST("/results/publish-config", requireExamBusinessPrincipalMiddleware(), examHandler.saveResultPublishConfig)
	api.POST("/results/export", requireExamBusinessPrincipalMiddleware(), examHandler.exportResults)
	api.GET("/tenants", requirePlatformPrincipalMiddleware(), tenantHandler.list)
	api.POST("/tenants", requirePlatformPrincipalMiddleware(), tenantHandler.create)
	api.POST("/tenants/:id/profile", requirePlatformPrincipalMiddleware(), tenantHandler.updateProfile)
	api.POST("/tenants/:id/reset-code", requirePlatformPrincipalMiddleware(), tenantHandler.resetCode)
	api.POST("/tenants/:id/register-setting", requirePlatformPrincipalMiddleware(), tenantHandler.updateRegisterSetting)
	api.POST("/uploads", requireTenantAdminOrPlatformPrincipalMiddleware(), uploadHandler.create)
	api.GET("/spaces", requireTenantAdminOrPlatformPrincipalMiddleware(), spaceHandler.list)
	api.POST("/spaces", requireTenantAdminOrPlatformPrincipalMiddleware(), spaceHandler.create)
	api.GET("/spaces/:id/members", requireAuthPrincipalMiddleware(), spaceHandler.listMembers)
	api.POST("/spaces/:id/members", requireAuthPrincipalMiddleware(), spaceHandler.addMember)
	api.PUT("/spaces/:id/members/:user_id", requireAuthPrincipalMiddleware(), spaceHandler.updateMember)
	api.DELETE("/spaces/:id/members/:user_id", requireAuthPrincipalMiddleware(), spaceHandler.removeMember)
	api.GET("/users", requireTenantAdminOrPlatformPrincipalMiddleware(), userHandler.list)
	api.POST("/users", requireTenantAdminOrPlatformPrincipalMiddleware(), userHandler.create)
	api.POST("/users/:id/disable", requireTenantAdminOrPlatformPrincipalMiddleware(), userHandler.disable)
	api.GET("/questions", requireExamBusinessPrincipalMiddleware(), questionHandler.list)
	api.POST("/questions", requireExamBusinessPrincipalMiddleware(), questionHandler.create)
	api.POST("/questions/import", requireExamBusinessPrincipalMiddleware(), questionHandler.importQuestions)
	api.GET("/papers", requireExamBusinessPrincipalMiddleware(), paperHandler.list)
	api.GET("/papers/:id/rules", requireExamBusinessPrincipalMiddleware(), paperHandler.listRules)
	api.POST("/papers/:id/rule-fixed/generate", requireExamBusinessPrincipalMiddleware(), paperHandler.generateRuleFixed)
	api.POST("/papers/:id/rule-live/precheck", requireExamBusinessPrincipalMiddleware(), paperHandler.precheckRuleLive)
	api.GET("/papers/:id/sections", requireExamBusinessPrincipalMiddleware(), paperHandler.listSections)
	api.POST("/papers/:id/sections", requireExamBusinessPrincipalMiddleware(), paperHandler.createSection)
	api.POST("/papers/:id/sections/:section_id/questions", requireExamBusinessPrincipalMiddleware(), paperHandler.addManualQuestion)
	api.POST("/papers/:id/sections/:section_id/rules", requireExamBusinessPrincipalMiddleware(), paperHandler.createRule)
	return router
}

func defaultExportDir(exportDir string) string {
	if exportDir != "" {
		return exportDir
	}
	return "server/data/exports"
}

func defaultUploadDir(uploadDir string) string {
	if uploadDir != "" {
		return uploadDir
	}
	return "server/data/uploads"
}

func defaultUploadStore(store storage.ObjectStore, uploadDir string) storage.ObjectStore {
	if store != nil {
		return store
	}
	return storage.NewLocalStore(storage.LocalStoreOptions{RootDir: uploadDir, PublicBaseURL: "/uploads"})
}

func defaultAuthSessionStore(options RouterOptions) sessions.Store {
	if options.AuthSessionStore != nil {
		return options.AuthSessionStore
	}
	store, err := NewSessionStore(SessionStoreOptions{
		Provider: options.AuthSessionProvider,
		Secret:   options.AuthSessionSecret,
		Redis: RedisSessionStoreOptions{
			Addr:      options.AuthSessionRedisAddr,
			Username:  options.AuthSessionRedisUser,
			Password:  options.AuthSessionRedisPass,
			DB:        options.AuthSessionRedisDB,
			KeyPrefix: options.AuthSessionKeyPrefix,
		},
	})
	if err != nil {
		panic(err)
	}
	return store
}

func defaultRouterNow(now func() int64) func() int64 {
	if now != nil {
		return now
	}
	return func() int64 { return time.Now().UnixMilli() }
}

type examHandler struct {
	service *serviceexam.Service
	taking  *serviceexam.TakingService
	review  *serviceexam.ReviewService
	export  *serviceexam.ExportService
	result  *serviceexam.ResultService
	members spaceMemberFinder
	now     func() int64
}

type spaceMemberFinder interface {
	FindMember(ctx context.Context, tenantID uint64, spaceID uint64, userID uint64) (servicespace.Member, error)
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
	if !authorizeExamBusiness(c, tenantID) {
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
	if !authorizeExamBusiness(c, request.TenantID) {
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
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, err.Error()))
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
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, err.Error()))
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
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, err.Error()))
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
	permissionContext, err := h.permissionContextFromSession(c, request.TenantID, request.SpaceID)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, err.Error()))
		return
	}
	if err := permission.NewFixedRoleChecker().CanViewExamResults(permissionContextWithExamScope(permissionContext, request.ExamID, request.SpaceID), request.ExamID); err != nil {
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
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, err.Error()))
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
		if spaceID == 0 {
			return permission.PermissionContext{}, errors.New("space_id 必须是正整数")
		}
		member, err := h.members.FindMember(c.Request.Context(), tenantID, spaceID, principal.UserID)
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
	default:
		return permission.PermissionContext{}, permission.ErrForbidden
	}
	return ctx, nil
}

func writePermissionOrInternalError(c *gin.Context, err error, fallback string) {
	if errors.Is(err, permission.ErrForbidden) {
		c.JSON(http.StatusForbidden, response.Fail(code.InvalidParam, "无权执行当前操作"))
		return
	}
	c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, fallback))
}

func authorizeTenantManagement(c *gin.Context, tenantID uint64) bool {
	principal, ok := currentAuthPrincipal(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, response.Fail(code.InvalidParam, "请先登录"))
		return false
	}
	if principal.SubjectType == permission.SubjectPlatformUser {
		return true
	}
	if principal.SubjectType == permission.SubjectTenantUser &&
		principal.Role == permission.RoleTenantAdmin &&
		principal.TenantID == tenantID {
		return true
	}
	c.JSON(http.StatusForbidden, response.Fail(code.InvalidParam, "无权执行当前操作"))
	return false
}

func authorizeExamBusiness(c *gin.Context, tenantID uint64) bool {
	principal, ok := currentAuthPrincipal(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, response.Fail(code.InvalidParam, "请先登录"))
		return false
	}
	if principal.SubjectType == permission.SubjectTenantUser &&
		principal.TenantID == tenantID &&
		(principal.Role == permission.RoleTenantAdmin || principal.Role == permission.RoleTeacher) {
		return true
	}
	c.JSON(http.StatusForbidden, response.Fail(code.InvalidParam, "无权执行当前操作"))
	return false
}

func permissionContextForResourceScope(c *gin.Context, tenantID uint64, spaceID *uint64, members spaceMemberFinder) (permission.PermissionContext, error) {
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
	}
	if principal.SubjectType != permission.SubjectTenantUser || principal.TenantID != tenantID {
		return permission.PermissionContext{}, permission.ErrForbidden
	}
	if principal.Role == permission.RoleTenantAdmin {
		return ctx, nil
	}
	if spaceID == nil {
		return permission.PermissionContext{}, permission.ErrForbidden
	}
	member, err := members.FindMember(c.Request.Context(), tenantID, *spaceID, principal.UserID)
	if errors.Is(err, servicespace.ErrMemberNotFound) {
		return permission.PermissionContext{}, permission.ErrForbidden
	}
	if err != nil {
		return permission.PermissionContext{}, err
	}
	if member.Status != servicespace.StatusEnabled {
		return permission.PermissionContext{}, permission.ErrForbidden
	}
	ctx.SpaceMemberships[*spaceID] = member.Role
	return ctx, nil
}

func readUintQuery(c *gin.Context, key string) (uint64, error) {
	raw := c.Query(key)
	if raw == "" {
		return 0, errors.New("missing query")
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || value == 0 {
		return 0, errors.New("invalid query")
	}
	return value, nil
}

func readPaginationQuery(c *gin.Context) (int, int, error) {
	page, err := readPositiveIntQuery(c, "page", pagination.DefaultPage)
	if err != nil {
		return 0, 0, err
	}
	pageSize, err := readPositiveIntQuery(c, "page_size", pagination.DefaultPageSize)
	if err != nil {
		return 0, 0, err
	}
	if pageSize > pagination.MaxPageSize {
		pageSize = pagination.MaxPageSize
	}
	return page, pageSize, nil
}

func readPositiveIntQuery(c *gin.Context, key string, defaultValue int) (int, error) {
	raw := c.Query(key)
	if raw == "" {
		return defaultValue, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, errors.New("invalid query")
	}
	return value, nil
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

type tenantHandler struct {
	service *servicetenant.Service
}

type createTenantRequest struct {
	Name          string `json:"name"`
	LogoURL       string `json:"logo_url"`
	Description   string `json:"description"`
	AllowRegister *bool  `json:"allow_register"`
	AdminUsername string `json:"admin_username"`
	AdminRealName string `json:"admin_real_name"`
	AdminPhone    string `json:"admin_phone"`
	AdminEmail    string `json:"admin_email"`
	AdminPassword string `json:"admin_password"`
}

type updateTenantProfileRequest struct {
	Name        string `json:"name"`
	LogoURL     string `json:"logo_url"`
	Description string `json:"description"`
}

type updateTenantRegisterSettingRequest struct {
	AllowRegister bool `json:"allow_register"`
}

type tenantResponse struct {
	ID            uint64 `json:"id"`
	Name          string `json:"name"`
	LogoURL       string `json:"logo_url"`
	Description   string `json:"description"`
	TenantCode    string `json:"tenant_code"`
	AllowRegister bool   `json:"allow_register"`
	Status        string `json:"status"`
}

type tenantListResponse struct {
	Items    []tenantResponse `json:"items"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
	Total    int64            `json:"total"`
}

func (h tenantHandler) list(c *gin.Context) {
	page, pageSize, err := readPaginationQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "page 和 page_size 必须是正整数"))
		return
	}
	result, err := h.service.List(c.Request.Context(), servicetenant.ListInput{
		Page:     page,
		PageSize: pageSize,
		Keyword:  c.Query("keyword"),
	})
	if err != nil {
		writeTenantInternalError(c, "读取租户列表失败", err)
		return
	}
	items := make([]tenantResponse, 0, len(result.Items))
	for _, tenant := range result.Items {
		items = append(items, tenantToResponse(tenant))
	}
	paged := response.Page(items, result.Page, result.PageSize, result.Total)
	c.JSON(http.StatusOK, response.OK(tenantListResponse{
		Items:    paged.Items,
		Page:     paged.Page,
		PageSize: paged.PageSize,
		Total:    paged.Total,
	}))
}

func (h tenantHandler) create(c *gin.Context) {
	actorID, ok := requirePlatformActor(c)
	if !ok {
		return
	}
	var request createTenantRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "请求体不是合法 JSON"))
		return
	}
	if request.Name == "" {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "name 不能为空"))
		return
	}
	if request.AdminUsername == "" || request.AdminRealName == "" || request.AdminPassword == "" {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "首个租户管理员用户名、姓名和初始密码不能为空"))
		return
	}
	passwordHash, err := crypto.HashPassword(request.AdminPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "生成首个租户管理员密码失败"))
		return
	}
	tenant, err := h.service.Create(c.Request.Context(), servicetenant.CreateInput{
		Name:          request.Name,
		LogoURL:       request.LogoURL,
		Description:   request.Description,
		AllowRegister: request.AllowRegister,
		ActorID:       actorID,
		InitialAdmin: servicetenant.InitialAdmin{
			Username:     request.AdminUsername,
			RealName:     request.AdminRealName,
			Phone:        request.AdminPhone,
			Email:        request.AdminEmail,
			PasswordHash: passwordHash,
		},
	})
	if err != nil {
		writeTenantInternalError(c, "创建租户失败", err)
		return
	}
	c.JSON(http.StatusOK, response.OK(tenantToResponse(tenant)))
}

func (h tenantHandler) updateProfile(c *gin.Context) {
	actorID, ok := requirePlatformActor(c)
	if !ok {
		return
	}
	tenantID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "租户 ID 必须是正整数"))
		return
	}
	var request updateTenantProfileRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "请求体不是合法 JSON"))
		return
	}
	// 租户资料用于平台列表展示，名称、描述和 Logo 必须一起返回最新快照。
	tenant, err := h.service.UpdateProfile(c.Request.Context(), servicetenant.UpdateProfileInput{
		TenantID:    tenantID,
		Name:        request.Name,
		LogoURL:     request.LogoURL,
		Description: request.Description,
		ActorID:     actorID,
	})
	if err != nil {
		writeTenantInternalError(c, "更新租户资料失败", err)
		return
	}
	c.JSON(http.StatusOK, response.OK(tenantToResponse(tenant)))
}

func (h tenantHandler) resetCode(c *gin.Context) {
	actorID, ok := requirePlatformActor(c)
	if !ok {
		return
	}
	tenantID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "租户 ID 必须是正整数"))
		return
	}
	tenant, err := h.service.ResetTenantCode(c.Request.Context(), servicetenant.ResetTenantCodeInput{
		TenantID: tenantID,
		ActorID:  actorID,
	})
	if err != nil {
		writeTenantInternalError(c, "重置租户码失败", err)
		return
	}
	c.JSON(http.StatusOK, response.OK(tenantToResponse(tenant)))
}

func (h tenantHandler) updateRegisterSetting(c *gin.Context) {
	actorID, ok := requirePlatformActor(c)
	if !ok {
		return
	}
	tenantID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "租户 ID 必须是正整数"))
		return
	}
	var request updateTenantRegisterSettingRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "请求体不是合法 JSON"))
		return
	}
	// 注册开关只控制该租户是否允许自注册，不改变租户启停状态。
	tenant, err := h.service.UpdateRegisterSetting(c.Request.Context(), servicetenant.UpdateRegisterSettingInput{
		TenantID:      tenantID,
		AllowRegister: request.AllowRegister,
		ActorID:       actorID,
	})
	if err != nil {
		writeTenantInternalError(c, "更新租户注册设置失败", err)
		return
	}
	c.JSON(http.StatusOK, response.OK(tenantToResponse(tenant)))
}

func writeTenantInternalError(c *gin.Context, message string, err error) {
	slog.Error(
		"tenant api failed",
		"request_id", c.GetString(middleware.RequestIDKey),
		"method", c.Request.Method,
		"path", c.Request.URL.Path,
		"message", message,
		"error", err,
	)
	c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, message))
}

func tenantToResponse(tenant servicetenant.Tenant) tenantResponse {
	return tenantResponse{
		ID:            tenant.ID,
		Name:          tenant.Name,
		LogoURL:       tenant.LogoURL,
		Description:   tenant.Description,
		TenantCode:    tenant.TenantCode,
		AllowRegister: tenant.AllowRegister,
		Status:        tenant.Status,
	}
}

type spaceMemberLister interface {
	ListMemberNames(ctx context.Context, tenantID uint64, spaceID uint64) ([]dbdao.SpaceMemberName, error)
	FindMember(ctx context.Context, tenantID uint64, spaceID uint64, userID uint64) (servicespace.Member, error)
}

type spaceHandler struct {
	service *servicespace.Service
	members spaceMemberLister
}

type createSpaceRequest struct {
	TenantID     uint64   `json:"tenant_id"`
	Name         string   `json:"name"`
	LogoURL      string   `json:"logo_url"`
	Description  string   `json:"description"`
	Type         string   `json:"type"`
	AdminUserIDs []uint64 `json:"admin_user_ids"`
}

type spaceResponse struct {
	ID          uint64                `json:"id"`
	TenantID    uint64                `json:"tenant_id"`
	Name        string                `json:"name"`
	LogoURL     string                `json:"logo_url"`
	Description string                `json:"description"`
	Type        string                `json:"type"`
	Status      string                `json:"status"`
	Members     []spaceMemberResponse `json:"members"`
}

type spaceMemberResponse struct {
	ID     uint64 `json:"id"`
	UserID uint64 `json:"user_id"`
	Name   string `json:"name"`
	Role   string `json:"role"`
	Status string `json:"status"`
}

type addSpaceMemberRequest struct {
	TenantID uint64 `json:"tenant_id"`
	UserID   uint64 `json:"user_id"`
	Role     string `json:"role"`
}

type updateSpaceMemberRequest struct {
	TenantID uint64 `json:"tenant_id"`
	Role     string `json:"role"`
	Status   string `json:"status"`
}

type spaceListResponse struct {
	Items    []spaceResponse `json:"items"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
	Total    int64           `json:"total"`
}

func (h spaceHandler) list(c *gin.Context) {
	tenantID, err := readUintQuery(c, "tenant_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id 必须是正整数"))
		return
	}
	if !authorizeTenantManagement(c, tenantID) {
		return
	}
	page, pageSize, err := readPaginationQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "page 和 page_size 必须是正整数"))
		return
	}
	result, err := h.service.List(c.Request.Context(), servicespace.ListInput{TenantID: tenantID, Page: page, PageSize: pageSize})
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "读取空间列表失败"))
		return
	}
	items := make([]spaceResponse, 0, len(result.Items))
	for _, space := range result.Items {
		item, err := h.spaceToResponse(c.Request.Context(), space)
		if err != nil {
			c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "读取空间成员失败"))
			return
		}
		items = append(items, item)
	}
	paged := response.Page(items, result.Page, result.PageSize, result.Total)
	c.JSON(http.StatusOK, response.OK(spaceListResponse{
		Items:    paged.Items,
		Page:     paged.Page,
		PageSize: paged.PageSize,
		Total:    paged.Total,
	}))
}

func (h spaceHandler) create(c *gin.Context) {
	var request createSpaceRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "请求体不是合法 JSON"))
		return
	}
	if err := request.validate(); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, err.Error()))
		return
	}
	if !authorizeTenantManagement(c, request.TenantID) {
		return
	}
	space, err := h.service.Create(c.Request.Context(), servicespace.CreateInput{
		TenantID:     request.TenantID,
		Name:         request.Name,
		LogoURL:      request.LogoURL,
		Description:  request.Description,
		Type:         request.Type,
		AdminUserIDs: request.AdminUserIDs,
	})
	if err != nil {
		writeSpaceServiceError(c, err)
		return
	}
	result, err := h.spaceToResponse(c.Request.Context(), space)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "读取空间成员失败"))
		return
	}
	c.JSON(http.StatusOK, response.OK(result))
}

func (h spaceHandler) listMembers(c *gin.Context) {
	spaceID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "space id 必须是正整数"))
		return
	}
	tenantID, err := readUintQuery(c, "tenant_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id 必须是正整数"))
		return
	}
	if !h.authorizeSpaceMembers(c, tenantID, spaceID) {
		return
	}
	members, err := h.members.ListMemberNames(c.Request.Context(), tenantID, spaceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "读取空间成员失败"))
		return
	}
	c.JSON(http.StatusOK, response.OK(spaceMemberNamesToResponse(members)))
}

func (h spaceHandler) addMember(c *gin.Context) {
	spaceID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "space id 必须是正整数"))
		return
	}
	var request addSpaceMemberRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "请求体不是合法 JSON"))
		return
	}
	if request.TenantID == 0 || request.UserID == 0 || request.Role == "" {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id、user_id 和 role 不能为空"))
		return
	}
	if !h.authorizeSpaceMembers(c, request.TenantID, spaceID) {
		return
	}
	member, err := h.service.JoinMember(c.Request.Context(), servicespace.JoinMemberInput{
		TenantID: request.TenantID,
		SpaceID:  spaceID,
		UserID:   request.UserID,
		Role:     request.Role,
	})
	if err != nil {
		writeSpaceServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(spaceMemberToResponse(member, "")))
}

func (h spaceHandler) updateMember(c *gin.Context) {
	spaceID, userID, ok := readSpaceMemberParams(c)
	if !ok {
		return
	}
	var request updateSpaceMemberRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "请求体不是合法 JSON"))
		return
	}
	if request.TenantID == 0 {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id 必须是正整数"))
		return
	}
	if !h.authorizeSpaceMembers(c, request.TenantID, spaceID) {
		return
	}
	if request.Role != "" {
		if err := h.service.ChangeMemberRole(c.Request.Context(), servicespace.ChangeRoleInput{TenantID: request.TenantID, SpaceID: spaceID, UserID: userID, Role: request.Role}); err != nil {
			writeSpaceServiceError(c, err)
			return
		}
	}
	if request.Status == servicespace.StatusDisabled {
		if err := h.service.DisableMember(c.Request.Context(), servicespace.MemberActionInput{TenantID: request.TenantID, SpaceID: spaceID, UserID: userID}); err != nil {
			writeSpaceServiceError(c, err)
			return
		}
	}
	member, err := h.members.FindMember(c.Request.Context(), request.TenantID, spaceID, userID)
	if err != nil {
		writeSpaceServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(spaceMemberToResponse(member, "")))
}

func (h spaceHandler) removeMember(c *gin.Context) {
	spaceID, userID, ok := readSpaceMemberParams(c)
	if !ok {
		return
	}
	tenantID, err := readUintQuery(c, "tenant_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id 必须是正整数"))
		return
	}
	if !h.authorizeSpaceMembers(c, tenantID, spaceID) {
		return
	}
	if err := h.service.RemoveMember(c.Request.Context(), servicespace.MemberActionInput{TenantID: tenantID, SpaceID: spaceID, UserID: userID}); err != nil {
		writeSpaceServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(gin.H{"removed": true}))
}

func (h spaceHandler) authorizeSpaceMembers(c *gin.Context, tenantID uint64, spaceID uint64) bool {
	principal, ok := currentAuthPrincipal(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, response.Fail(code.InvalidParam, "请先登录"))
		return false
	}
	if principal.SubjectType != permission.SubjectTenantUser || principal.TenantID != tenantID {
		c.JSON(http.StatusForbidden, response.Fail(code.InvalidParam, "无权执行当前操作"))
		return false
	}
	if principal.Role == permission.RoleTenantAdmin {
		return true
	}
	member, err := h.members.FindMember(c.Request.Context(), tenantID, spaceID, principal.UserID)
	if err != nil || member.Status != servicespace.StatusEnabled || member.Role != servicespace.RoleSpaceAdmin {
		c.JSON(http.StatusForbidden, response.Fail(code.InvalidParam, "无权执行当前操作"))
		return false
	}
	return true
}

func readSpaceMemberParams(c *gin.Context) (uint64, uint64, bool) {
	spaceID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "space id 必须是正整数"))
		return 0, 0, false
	}
	userID, err := readUintParam(c, "user_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "user_id 必须是正整数"))
		return 0, 0, false
	}
	return spaceID, userID, true
}

func (r createSpaceRequest) validate() error {
	if r.TenantID == 0 {
		return errors.New("tenant_id 必须是正整数")
	}
	if r.Name == "" {
		return errors.New("name 不能为空")
	}
	if r.Type == "" {
		return errors.New("type 不能为空")
	}
	if len(r.AdminUserIDs) == 0 {
		return errors.New("admin_user_ids 不能为空")
	}
	return nil
}

func (h spaceHandler) spaceToResponse(ctx context.Context, space servicespace.Space) (spaceResponse, error) {
	members, err := h.members.ListMemberNames(ctx, space.TenantID, space.ID)
	if err != nil {
		return spaceResponse{}, err
	}
	result := spaceResponse{
		ID:          space.ID,
		TenantID:    space.TenantID,
		Name:        space.Name,
		LogoURL:     space.LogoURL,
		Description: space.Description,
		Type:        space.Type,
		Status:      space.Status,
		Members:     make([]spaceMemberResponse, 0, len(members)),
	}
	for _, member := range members {
		result.Members = append(result.Members, spaceMemberNameToResponse(member))
	}
	return result, nil
}

func spaceMemberNamesToResponse(members []dbdao.SpaceMemberName) []spaceMemberResponse {
	items := make([]spaceMemberResponse, 0, len(members))
	for _, member := range members {
		items = append(items, spaceMemberNameToResponse(member))
	}
	return items
}

func spaceMemberNameToResponse(member dbdao.SpaceMemberName) spaceMemberResponse {
	return spaceMemberResponse{
		ID:     member.ID,
		UserID: member.UserID,
		Name:   displaySpaceMemberName(member.UserID, member.Name),
		Role:   member.Role,
		Status: member.Status,
	}
}

func spaceMemberToResponse(member servicespace.Member, name string) spaceMemberResponse {
	return spaceMemberResponse{
		ID:     member.ID,
		UserID: member.UserID,
		Name:   displaySpaceMemberName(member.UserID, name),
		Role:   member.Role,
		Status: member.Status,
	}
}

func displaySpaceMemberName(userID uint64, name string) string {
	if name != "" {
		return name
	}
	return "用户 " + strconv.FormatUint(userID, 10)
}

func writeSpaceServiceError(c *gin.Context, err error) {
	if errors.Is(err, servicespace.ErrSpaceAdminRequired) ||
		errors.Is(err, servicespace.ErrCannotLoseLastSpaceAdmin) ||
		errors.Is(err, servicespace.ErrMemberNotFound) {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, err.Error()))
		return
	}
	c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "空间操作失败"))
}

type userHandler struct {
	service           *servicetenantuser.Service
	passwordMinLength int
}

type createUserRequest struct {
	TenantID  uint64 `json:"tenant_id"`
	Username  string `json:"username"`
	RealName  string `json:"real_name"`
	AvatarURL string `json:"avatar_url"`
	Password  string `json:"password"`
	Role      string `json:"role"`
}

type disableUserRequest struct {
	TenantID uint64 `json:"tenant_id"`
}

type userResponse struct {
	ID        uint64 `json:"id"`
	TenantID  uint64 `json:"tenant_id"`
	Username  string `json:"username"`
	RealName  string `json:"real_name"`
	AvatarURL string `json:"avatar_url"`
	Role      string `json:"role"`
	Status    string `json:"status"`
}

type userListResponse struct {
	Items    []userResponse `json:"items"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
	Total    int64          `json:"total"`
}

func (h userHandler) list(c *gin.Context) {
	tenantID, err := readUintQuery(c, "tenant_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id 必须是正整数"))
		return
	}
	if !authorizeTenantManagement(c, tenantID) {
		return
	}
	page, pageSize, err := readPaginationQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "page 和 page_size 必须是正整数"))
		return
	}
	result, err := h.service.List(c.Request.Context(), servicetenantuser.ListInput{TenantID: tenantID, Page: page, PageSize: pageSize})
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "读取用户列表失败"))
		return
	}
	items := make([]userResponse, 0, len(result.Items))
	for _, user := range result.Items {
		items = append(items, userToResponse(user))
	}
	paged := response.Page(items, result.Page, result.PageSize, result.Total)
	c.JSON(http.StatusOK, response.OK(userListResponse{
		Items:    paged.Items,
		Page:     paged.Page,
		PageSize: paged.PageSize,
		Total:    paged.Total,
	}))
}

func (h userHandler) create(c *gin.Context) {
	var request createUserRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "请求体不是合法 JSON"))
		return
	}
	if err := request.validate(h.passwordMinLength); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, err.Error()))
		return
	}
	if !authorizeTenantManagement(c, request.TenantID) {
		return
	}
	passwordHash, err := crypto.HashPassword(request.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "生成密码失败"))
		return
	}
	user, err := h.service.Create(c.Request.Context(), servicetenantuser.CreateInput{
		TenantID:     request.TenantID,
		Username:     request.Username,
		RealName:     request.RealName,
		AvatarURL:    request.AvatarURL,
		PasswordHash: passwordHash,
		Role:         request.Role,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "创建用户失败"))
		return
	}
	c.JSON(http.StatusOK, response.OK(userToResponse(user)))
}

func (h userHandler) disable(c *gin.Context) {
	userID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "用户 ID 必须是正整数"))
		return
	}
	var request disableUserRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "请求体不是合法 JSON"))
		return
	}
	if request.TenantID == 0 {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id 必须是正整数"))
		return
	}
	if !authorizeTenantManagement(c, request.TenantID) {
		return
	}
	principal, ok := currentAuthPrincipal(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, response.Fail(code.InvalidParam, "请先登录"))
		return
	}
	if err := h.service.Disable(c.Request.Context(), servicetenantuser.DisableInput{
		TenantID: request.TenantID,
		ActorID:  principal.UserID,
		TargetID: userID,
	}); err != nil {
		writeUserServiceError(c, err)
		return
	}
	user, err := h.service.Get(c.Request.Context(), request.TenantID, userID)
	if err != nil {
		writeUserServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(userToResponse(user)))
}

func (r createUserRequest) validate(passwordMinLength int) error {
	if r.TenantID == 0 {
		return errors.New("tenant_id 必须是正整数")
	}
	if r.Username == "" {
		return errors.New("username 不能为空")
	}
	if r.RealName == "" {
		return errors.New("real_name 不能为空")
	}
	if r.Password == "" {
		return errors.New("password 不能为空")
	}
	if err := validatePasswordMinLength(r.Password, passwordMinLength); err != nil {
		return err
	}
	if r.Role != servicetenantuser.RoleTenantAdmin && r.Role != servicetenantuser.RoleTeacher && r.Role != servicetenantuser.RoleStudent {
		return errors.New("role 只能是 tenant_admin、teacher 或 student")
	}
	return nil
}

func userToResponse(user servicetenantuser.User) userResponse {
	return userResponse{
		ID:        user.ID,
		TenantID:  user.TenantID,
		Username:  user.Username,
		RealName:  user.RealName,
		AvatarURL: user.AvatarURL,
		Role:      user.Role,
		Status:    user.Status,
	}
}

func writeUserServiceError(c *gin.Context, err error) {
	if errors.Is(err, servicetenantuser.ErrCannotDisableSelf) ||
		errors.Is(err, servicetenantuser.ErrCannotLoseLastTenantAdmin) ||
		errors.Is(err, servicespace.ErrCannotLoseLastSpaceAdmin) {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, err.Error()))
		return
	}
	if errors.Is(err, servicetenantuser.ErrUserNotFound) {
		c.JSON(http.StatusNotFound, response.Fail(code.InvalidParam, err.Error()))
		return
	}
	c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "用户操作失败"))
}

func readUintParam(c *gin.Context, key string) (uint64, error) {
	raw := c.Param(key)
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || value == 0 {
		return 0, errors.New("invalid param")
	}
	return value, nil
}

type questionHandler struct {
	service *servicequestion.QuestionService
	members spaceMemberFinder
}

type createQuestionRequest struct {
	TenantID     uint64                  `json:"tenant_id"`
	SpaceID      *uint64                 `json:"space_id"`
	Type         string                  `json:"type"`
	Difficulty   string                  `json:"difficulty"`
	Title        string                  `json:"title"`
	Analysis     string                  `json:"analysis"`
	ScoreDefault string                  `json:"score_default"`
	Tags         []string                `json:"tags"`
	Options      []questionOptionRequest `json:"options"`
}

type questionOptionRequest struct {
	OptionKey    string `json:"option_key"`
	Content      string `json:"content"`
	IsCorrect    bool   `json:"is_correct"`
	IsDistractor bool   `json:"is_distractor"`
}

type questionResponse struct {
	ID           uint64                   `json:"id"`
	TenantID     uint64                   `json:"tenant_id"`
	SpaceID      *uint64                  `json:"space_id,omitempty"`
	Type         string                   `json:"type"`
	Difficulty   string                   `json:"difficulty"`
	Title        string                   `json:"title"`
	Analysis     string                   `json:"analysis"`
	ScoreDefault string                   `json:"score_default"`
	Status       string                   `json:"status"`
	Tag          string                   `json:"tag"`
	Tags         []string                 `json:"tags"`
	Options      []questionOptionResponse `json:"options"`
}

type questionOptionResponse struct {
	ID           uint64 `json:"id"`
	OptionKey    string `json:"option_key"`
	Content      string `json:"content"`
	IsCorrect    bool   `json:"is_correct"`
	IsDistractor bool   `json:"is_distractor"`
}

type questionListResponse struct {
	Items    []questionResponse `json:"items"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
	Total    int64              `json:"total"`
}

func (h questionHandler) list(c *gin.Context) {
	tenantID, err := readUintQuery(c, "tenant_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id 必须是正整数"))
		return
	}
	if !authorizeExamBusiness(c, tenantID) {
		return
	}
	spaceID, err := readOptionalUintQuery(c, "space_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "space_id 必须是正整数"))
		return
	}
	page, pageSize, err := readPaginationQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "page 和 page_size 必须是正整数"))
		return
	}
	result, err := h.service.ListVisibleQuestions(c.Request.Context(), servicequestion.ListQuestionsInput{
		TenantID: tenantID,
		SpaceID:  spaceID,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "读取题目列表失败"))
		return
	}
	items := make([]questionResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, questionToResponse(item))
	}
	paged := response.Page(items, result.Page, result.PageSize, result.Total)
	c.JSON(http.StatusOK, response.OK(questionListResponse{
		Items:    paged.Items,
		Page:     paged.Page,
		PageSize: paged.PageSize,
		Total:    paged.Total,
	}))
}

func (h questionHandler) create(c *gin.Context) {
	var request createQuestionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "请求体不是合法 JSON"))
		return
	}
	if err := request.validate(); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, err.Error()))
		return
	}
	if !authorizeExamBusiness(c, request.TenantID) {
		return
	}
	permissionContext, err := permissionContextForResourceScope(c, request.TenantID, request.SpaceID, h.members)
	if err != nil {
		writePermissionOrInternalError(c, err, "构建题库权限上下文失败")
		return
	}
	created, err := h.service.CreateQuestion(c.Request.Context(), servicequestion.CreateQuestionInput{
		Permission:   permissionContext,
		TenantID:     request.TenantID,
		SpaceID:      request.SpaceID,
		Type:         request.Type,
		Difficulty:   request.Difficulty,
		Title:        request.Title,
		Analysis:     request.Analysis,
		ScoreDefault: request.ScoreDefault,
		Options:      request.toServiceOptions(),
		Tags:         request.Tags,
	})
	if err != nil {
		writeQuestionServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(questionToResponse(created)))
}

func (r createQuestionRequest) validate() error {
	if r.TenantID == 0 {
		return errors.New("tenant_id 必须是正整数")
	}
	if r.Title == "" {
		return errors.New("title 不能为空")
	}
	if r.Type == "" {
		return errors.New("type 不能为空")
	}
	if r.Difficulty == "" {
		return errors.New("difficulty 不能为空")
	}
	return nil
}

func (r createQuestionRequest) toServiceOptions() []servicequestion.QuestionOptionInput {
	options := make([]servicequestion.QuestionOptionInput, 0, len(r.Options))
	for _, option := range r.Options {
		options = append(options, servicequestion.QuestionOptionInput{
			OptionKey:    option.OptionKey,
			Content:      option.Content,
			IsCorrect:    option.IsCorrect,
			IsDistractor: option.IsDistractor,
		})
	}
	return options
}

func questionToResponse(item servicequestion.Question) questionResponse {
	options := make([]questionOptionResponse, 0, len(item.Options))
	for _, option := range item.Options {
		options = append(options, questionOptionResponse{
			ID:           option.ID,
			OptionKey:    option.OptionKey,
			Content:      option.Content,
			IsCorrect:    option.IsCorrect,
			IsDistractor: option.IsDistractor,
		})
	}
	tag := ""
	if len(item.Tags) > 0 {
		tag = item.Tags[0]
	}
	return questionResponse{
		ID:           item.ID,
		TenantID:     item.TenantID,
		SpaceID:      item.SpaceID,
		Type:         item.Type,
		Difficulty:   item.Difficulty,
		Title:        item.Title,
		Analysis:     item.Analysis,
		ScoreDefault: item.ScoreDefault,
		Status:       item.Status,
		Tag:          tag,
		Tags:         item.Tags,
		Options:      options,
	}
}

func writeQuestionServiceError(c *gin.Context, err error) {
	if errors.Is(err, permission.ErrForbidden) {
		c.JSON(http.StatusForbidden, response.Fail(code.InvalidParam, "无权执行当前操作"))
		return
	}
	if errors.Is(err, servicequestion.ErrChoiceQuestionNeedsCorrectAnswer) ||
		errors.Is(err, servicequestion.ErrSingleQuestionOnlyOneCorrectAnswer) ||
		errors.Is(err, servicequestion.ErrFillBlankOnlySupportsSingleBlank) {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, err.Error()))
		return
	}
	c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "题目操作失败"))
}

func readOptionalUintQuery(c *gin.Context, key string) (*uint64, error) {
	raw := c.Query(key)
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || value == 0 {
		return nil, errors.New("invalid query")
	}
	return &value, nil
}

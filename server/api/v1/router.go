package v1

import (
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	apimiddleware "github.com/lifei6671/papermind/server/api/middleware"
	apirouter "github.com/lifei6671/papermind/server/api/router"
	serviceexam "github.com/lifei6671/papermind/server/internal/service/exam"
)

const (
	examEntrySessionTenantIDKey = "exam_entry_tenant_id"
	examEntrySessionExamIDKey   = "exam_entry_exam_id"
	examEntrySessionUserIDKey   = "exam_entry_user_id"
	examEntryContextKey         = "exam_entry_context"
)

// RouterOptions 是 v1 路由的启动配置别名，实际定义位于核心 router 包。
type RouterOptions = apirouter.Options

// NewRouter 保留 v1 包原有入口，便于现有测试和调用方平滑迁移。
func NewRouter(options RouterOptions) *gin.Engine {
	return apirouter.New(options, RegisterRoutes, AuthMiddlewares(options)...)
}

// AuthMiddlewares 返回 v1 API 需要的登录态中间件链。
func AuthMiddlewares(options RouterOptions) []gin.HandlerFunc {
	sessionStore := defaultAuthSessionStore(options)
	return []gin.HandlerFunc{
		apimiddleware.BearerSessionCookie(authSessionName),
		sessions.Sessions(authSessionName, sessionStore),
		apimiddleware.AuthContext(),
	}
}

// RegisterRoutes 只负责注册 /api/v1 下的版本路由和对应 handler。
func RegisterRoutes(api *gin.RouterGroup, deps apirouter.Dependencies) {
	managementDetail := serviceexam.NewManagementDetailService(serviceexam.ManagementDetailServiceOptions{Repo: deps.ExamRepository, Now: deps.Now})
	examHandler := examHandler{service: deps.Exams, management: managementDetail, taking: deps.Taking, review: deps.Review, export: deps.Export, result: deps.Results, papers: deps.PaperRepository, targets: deps.ExamRepository, members: deps.SpaceRepository, tenantUsers: deps.TenantUsers, sessionMaxAgeSeconds: deps.AuthSessionTTL, now: deps.Now}
	tenantHandler := tenantHandler{service: deps.Tenants, spaces: deps.Spaces, users: deps.TenantUsers, members: deps.SpaceRepository, passwordMinLength: deps.PasswordMinLength}
	spaceHandler := spaceHandler{service: deps.Spaces, users: deps.TenantUsers, members: deps.SpaceRepository}
	userHandler := userHandler{service: deps.TenantUsers, passwordMinLength: deps.PasswordMinLength}
	questionHandler := questionHandler{service: deps.Questions, members: deps.SpaceRepository, users: deps.TenantUsers, importJobs: newQuestionImportJobStore()}
	paperHandler := paperHandler{service: deps.Papers, papers: deps.PaperRepository, members: deps.SpaceRepository, users: deps.TenantUsers}
	authHandler := authHandler{platformUsers: deps.PlatformUsers, tenantUsers: deps.TenantUsers, spaces: deps.Spaces, sessionMaxAgeSeconds: deps.AuthSessionTTL, passwordMinLength: deps.PasswordMinLength}
	uploadHandler := uploadHandler{store: deps.UploadStore, now: time.Now}

	api.POST("/auth/platform/login", authHandler.platformLogin)
	api.POST("/auth/tenant/login", authHandler.tenantLogin)
	api.POST("/auth/tenant/select-space", apimiddleware.RequireAuthPrincipal(), authHandler.selectTenantSpace)
	api.POST("/auth/tenant/register", authHandler.tenantRegister)
	api.POST("/auth/logout", authHandler.logout)
	api.GET("/profile", apimiddleware.RequireAuthPrincipal(), authHandler.getProfile)
	api.POST("/profile", apimiddleware.RequireAuthPrincipal(), authHandler.updateProfile)
	api.POST("/profile/password", apimiddleware.RequireAuthPrincipal(), authHandler.updatePassword)
	api.GET("/profile/spaces", apimiddleware.RequireAuthPrincipal(), authHandler.listProfileSpaces)
	tenant := api.Group("/tenant")
	tenant.GET("/profile/spaces", apimiddleware.RequireAuthPrincipal(), authHandler.listProfileSpaces)
	api.GET("/exams", apimiddleware.RequireExamBusinessPrincipal(), examHandler.list)
	api.POST("/exams", apimiddleware.RequireExamBusinessPrincipal(), examHandler.publish)
	api.POST("/exams/:id/draft", apimiddleware.RequireExamBusinessPrincipal(), examHandler.updateDraft)
	api.POST("/exams/:id/status", apimiddleware.RequireExamBusinessPrincipal(), examHandler.updateStatus)
	api.GET("/exams/:id/detail", apimiddleware.RequireExamBusinessPrincipal(), examHandler.getManagementDetail)
	api.GET("/exams/:id/overview", apimiddleware.RequireExamBusinessPrincipal(), examHandler.getOverview)
	api.GET("/exams/:id/paper-preview", apimiddleware.RequireExamBusinessPrincipal(), examHandler.getPaperPreview)
	api.GET("/exams/:id/candidates", apimiddleware.RequireExamBusinessPrincipal(), examHandler.listCandidates)
	api.GET("/exams/:id/results/summary", apimiddleware.RequireExamBusinessPrincipal(), examHandler.getResultsSummary)
	api.GET("/exams/:id/results", apimiddleware.RequireExamBusinessPrincipal(), examHandler.listManagementResults)
	api.GET("/exams/:id/attempts/:attempt_id/answer-sheet", apimiddleware.RequireExamBusinessPrincipal(), examHandler.getManagementAnswerSheet)
	api.GET("/exams/:id/logs", apimiddleware.RequireExamBusinessPrincipal(), examHandler.listManagementOperationLogs)
	api.POST("/exams/:id/settings", apimiddleware.RequireExamBusinessPrincipal(), examHandler.updateManagementSettings)
	api.POST("/exams/:id/candidates/import", apimiddleware.RequireExamBusinessPrincipal(), examHandler.importCandidates)
	api.POST("/exams/:id/invitations/resend", apimiddleware.RequireExamBusinessPrincipal(), examHandler.resendInvitations)
	api.POST("/exams/invite/resolve", examHandler.resolveInvite)
	api.POST("/exams/:id/attempts/start", examHandler.startAttempt)
	api.POST("/exam-entry/invite/resolve", examHandler.resolveInvite)
	api.POST("/exam-entry/exams/:id/attempts/start", examHandler.startAttempt)
	api.POST("/exam-entry/attempts/:attempt_id/answers/:attempt_question_id", examEntryTokenMiddleware(deps.Taking), examHandler.saveAnswer)
	api.POST("/exam-entry/attempts/:attempt_id/submit", examEntryTokenMiddleware(deps.Taking), examHandler.submitAttempt)
	api.POST("/exam-entry/attempts/:attempt_id/events", examEntryTokenMiddleware(deps.Taking), examHandler.recordEvent)
	api.GET("/exam-entry/results/:id", apimiddleware.RequireAuthPrincipal(), examHandler.getExamEntryResult)
	api.GET("/grading/pending", apimiddleware.RequireExamBusinessPrincipal(), examHandler.listPendingReviews)
	api.POST("/exam-attempts/:attempt_id/questions/:attempt_question_id/grade", apimiddleware.RequireExamBusinessPrincipal(), examHandler.gradeShortText)
	api.GET("/results", apimiddleware.RequireExamBusinessPrincipal(), examHandler.listResults)
	api.POST("/results/publish-config", apimiddleware.RequireExamBusinessPrincipal(), examHandler.saveResultPublishConfig)
	api.POST("/results/export", apimiddleware.RequireExamBusinessPrincipal(), examHandler.exportResults)
	api.GET("/results/export-files/:file_name", apimiddleware.RequireExamBusinessPrincipal(), examHandler.downloadExport)
	platformOnly := apimiddleware.RequireLivePlatformPrincipal(deps.PlatformUsers)
	api.GET("/tenants", platformOnly, tenantHandler.list)
	api.POST("/tenants", platformOnly, tenantHandler.create)
	api.POST("/tenants/:id/profile", platformOnly, tenantHandler.updateProfile)
	api.POST("/tenants/:id/reset-code", platformOnly, tenantHandler.resetCode)
	api.POST("/tenants/:id/register-setting", platformOnly, tenantHandler.updateRegisterSetting)
	api.GET("/tenants/:id/spaces", platformOnly, tenantHandler.listTenantSpaces)
	api.GET("/tenants/:id/users", platformOnly, tenantHandler.listTenantUsers)
	api.POST("/uploads", apimiddleware.RequireLiveTenantAdminOrPlatformPrincipal(deps.PlatformUsers, deps.TenantUsers), uploadHandler.create)
	api.GET("/spaces", apimiddleware.RequireAuthPrincipal(), spaceHandler.list)
	api.POST("/spaces", apimiddleware.RequireAuthPrincipal(), spaceHandler.create)
	api.PUT("/spaces/:id", apimiddleware.RequireAuthPrincipal(), spaceHandler.updateProfile)
	api.POST("/spaces/:id/disable", apimiddleware.RequireAuthPrincipal(), spaceHandler.disable)
	api.DELETE("/spaces/:id", apimiddleware.RequireAuthPrincipal(), spaceHandler.delete)
	api.GET("/spaces/:id/members", apimiddleware.RequireAuthPrincipal(), spaceHandler.listMembers)
	api.POST("/spaces/:id/members", apimiddleware.RequireAuthPrincipal(), spaceHandler.addMember)
	api.PUT("/spaces/:id/members/:user_id", apimiddleware.RequireAuthPrincipal(), spaceHandler.updateMember)
	api.DELETE("/spaces/:id/members/:user_id", apimiddleware.RequireAuthPrincipal(), spaceHandler.removeMember)
	tenant.GET("/spaces", apimiddleware.RequireAuthPrincipal(), spaceHandler.list)
	tenant.POST("/spaces", apimiddleware.RequireAuthPrincipal(), spaceHandler.create)
	tenant.PUT("/spaces/:id", apimiddleware.RequireAuthPrincipal(), spaceHandler.updateProfile)
	tenant.POST("/spaces/:id/disable", apimiddleware.RequireAuthPrincipal(), spaceHandler.disable)
	tenant.DELETE("/spaces/:id", apimiddleware.RequireAuthPrincipal(), spaceHandler.delete)
	tenant.GET("/spaces/:id/members", apimiddleware.RequireAuthPrincipal(), spaceHandler.listMembers)
	tenant.POST("/spaces/:id/members", apimiddleware.RequireAuthPrincipal(), spaceHandler.addMember)
	tenant.PUT("/spaces/:id/members/:user_id", apimiddleware.RequireAuthPrincipal(), spaceHandler.updateMember)
	tenant.DELETE("/spaces/:id/members/:user_id", apimiddleware.RequireAuthPrincipal(), spaceHandler.removeMember)
	api.GET("/users", apimiddleware.RequireAuthPrincipal(), userHandler.list)
	api.POST("/users", apimiddleware.RequireAuthPrincipal(), userHandler.create)
	api.POST("/users/import", apimiddleware.RequireAuthPrincipal(), userHandler.importUsers)
	api.POST("/users/:id/disable", apimiddleware.RequireAuthPrincipal(), userHandler.disable)
	api.POST("/users/:id/enable", apimiddleware.RequireAuthPrincipal(), userHandler.enable)
	api.PUT("/users/:id/profile", apimiddleware.RequireAuthPrincipal(), userHandler.updateProfile)
	api.DELETE("/users/:id", apimiddleware.RequireAuthPrincipal(), userHandler.delete)
	api.PUT("/users/:id/role", apimiddleware.RequireAuthPrincipal(), userHandler.updateRole)
	tenant.GET("/users", apimiddleware.RequireAuthPrincipal(), userHandler.list)
	tenant.POST("/users", apimiddleware.RequireAuthPrincipal(), userHandler.create)
	tenant.POST("/users/import", apimiddleware.RequireAuthPrincipal(), userHandler.importUsers)
	tenant.POST("/users/:id/disable", apimiddleware.RequireAuthPrincipal(), userHandler.disable)
	tenant.POST("/users/:id/enable", apimiddleware.RequireAuthPrincipal(), userHandler.enable)
	tenant.PUT("/users/:id/profile", apimiddleware.RequireAuthPrincipal(), userHandler.updateProfile)
	tenant.DELETE("/users/:id", apimiddleware.RequireAuthPrincipal(), userHandler.delete)
	tenant.PUT("/users/:id/role", apimiddleware.RequireAuthPrincipal(), userHandler.updateRole)
	api.GET("/questions", apimiddleware.RequireExamBusinessPrincipal(), questionHandler.list)
	api.GET("/questions/tags", apimiddleware.RequireExamBusinessPrincipal(), questionHandler.listTags)
	api.POST("/questions/availability-counts", apimiddleware.RequireExamBusinessPrincipal(), questionHandler.countAvailability)
	api.POST("/questions", apimiddleware.RequireExamBusinessPrincipal(), questionHandler.create)
	api.GET("/questions/:id", apimiddleware.RequireExamBusinessPrincipal(), questionHandler.get)
	api.PUT("/questions/:id", apimiddleware.RequireExamBusinessPrincipal(), questionHandler.update)
	api.POST("/questions/:id/disable", apimiddleware.RequireExamBusinessPrincipal(), questionHandler.disable)
	api.POST("/questions/:id/enable", apimiddleware.RequireExamBusinessPrincipal(), questionHandler.enable)
	api.DELETE("/questions/:id", apimiddleware.RequireExamBusinessPrincipal(), questionHandler.delete)
	api.POST("/questions/import", apimiddleware.RequireExamBusinessPrincipal(), questionHandler.importQuestions)
	api.POST("/questions/import/jobs", apimiddleware.RequireExamBusinessPrincipal(), questionHandler.startImportQuestionsJob)
	api.GET("/questions/import/jobs/:job_id/events", apimiddleware.RequireExamBusinessPrincipal(), questionHandler.streamImportQuestionJobEvents)
	api.GET("/papers", apimiddleware.RequireExamBusinessPrincipal(), paperHandler.list)
	api.GET("/papers/publish-candidates", apimiddleware.RequireExamBusinessPrincipal(), paperHandler.listPublishCandidates)
	api.GET("/papers/:id", apimiddleware.RequireExamBusinessPrincipal(), paperHandler.get)
	api.POST("/papers", apimiddleware.RequireExamBusinessPrincipal(), paperHandler.create)
	api.PUT("/papers/:id", apimiddleware.RequireExamBusinessPrincipal(), paperHandler.update)
	api.DELETE("/papers/:id", apimiddleware.RequireExamBusinessPrincipal(), paperHandler.delete)
	api.POST("/papers/:id/enable", apimiddleware.RequireExamBusinessPrincipal(), paperHandler.enable)
	api.POST("/papers/:id/disable", apimiddleware.RequireExamBusinessPrincipal(), paperHandler.disable)
	api.PUT("/papers/:id/mode", apimiddleware.RequireExamBusinessPrincipal(), paperHandler.updateBuildMode)
	api.GET("/papers/:id/rules", apimiddleware.RequireExamBusinessPrincipal(), paperHandler.listRules)
	api.PUT("/papers/:id/rules/:rule_id", apimiddleware.RequireExamBusinessPrincipal(), paperHandler.updateRule)
	api.DELETE("/papers/:id/rules/:rule_id", apimiddleware.RequireExamBusinessPrincipal(), paperHandler.deleteRule)
	api.POST("/papers/:id/rule-fixed/generate", apimiddleware.RequireExamBusinessPrincipal(), paperHandler.generateRuleFixed)
	api.POST("/papers/:id/rule-live/precheck", apimiddleware.RequireExamBusinessPrincipal(), paperHandler.precheckRuleLive)
	api.GET("/papers/:id/sections", apimiddleware.RequireExamBusinessPrincipal(), paperHandler.listSections)
	api.GET("/papers/:id/questions", apimiddleware.RequireExamBusinessPrincipal(), paperHandler.listSectionQuestions)
	api.POST("/papers/:id/sections", apimiddleware.RequireExamBusinessPrincipal(), paperHandler.createSection)
	api.PUT("/papers/:id/sections/reorder", apimiddleware.RequireExamBusinessPrincipal(), paperHandler.reorderSections)
	api.DELETE("/papers/:id/sections/:section_id", apimiddleware.RequireExamBusinessPrincipal(), paperHandler.deleteSection)
	api.POST("/papers/:id/sections/:section_id/questions", apimiddleware.RequireExamBusinessPrincipal(), paperHandler.addManualQuestion)
	api.PUT("/papers/:id/sections/:section_id/questions/:question_id", apimiddleware.RequireExamBusinessPrincipal(), paperHandler.updateSectionQuestion)
	api.DELETE("/papers/:id/sections/:section_id/questions/:question_id", apimiddleware.RequireExamBusinessPrincipal(), paperHandler.deleteSectionQuestion)
	api.POST("/papers/:id/sections/:section_id/questions/:question_id/replace", apimiddleware.RequireExamBusinessPrincipal(), paperHandler.replaceSectionQuestion)
	api.POST("/papers/:id/sections/:section_id/rules", apimiddleware.RequireExamBusinessPrincipal(), paperHandler.createRule)
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

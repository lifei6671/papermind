package v1

import (
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	apirouter "github.com/lifei6671/papermind/server/api/router"
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
		bearerSessionCookieMiddleware(authSessionName),
		sessions.Sessions(authSessionName, sessionStore),
		authContextMiddleware(),
	}
}

// RegisterRoutes 只负责注册 /api/v1 下的版本路由和对应 handler。
func RegisterRoutes(api *gin.RouterGroup, deps apirouter.Dependencies) {
	examHandler := examHandler{service: deps.Exams, taking: deps.Taking, review: deps.Review, export: deps.Export, result: deps.Results, papers: deps.PaperRepository, members: deps.SpaceRepository, now: deps.Now}
	tenantHandler := tenantHandler{service: deps.Tenants}
	spaceHandler := spaceHandler{service: deps.Spaces, members: deps.SpaceRepository}
	userHandler := userHandler{service: deps.TenantUsers, passwordMinLength: deps.PasswordMinLength}
	questionHandler := questionHandler{service: deps.Questions, members: deps.SpaceRepository}
	paperHandler := paperHandler{service: deps.Papers, papers: deps.PaperRepository, members: deps.SpaceRepository}
	authHandler := authHandler{platformUsers: deps.PlatformUsers, tenantUsers: deps.TenantUsers, spaces: deps.Spaces, sessionMaxAgeSeconds: deps.AuthSessionTTL, passwordMinLength: deps.PasswordMinLength}
	uploadHandler := uploadHandler{store: deps.UploadStore, now: time.Now}

	api.POST("/auth/platform/login", authHandler.platformLogin)
	api.POST("/auth/tenant/login", authHandler.tenantLogin)
	api.POST("/auth/tenant/register", authHandler.tenantRegister)
	api.GET("/profile", requireAuthPrincipalMiddleware(), authHandler.getProfile)
	api.POST("/profile", requireAuthPrincipalMiddleware(), authHandler.updateProfile)
	api.GET("/profile/spaces", requireAuthPrincipalMiddleware(), authHandler.listProfileSpaces)
	api.GET("/exams", requireExamBusinessPrincipalMiddleware(), examHandler.list)
	api.POST("/exams", requireExamBusinessPrincipalMiddleware(), examHandler.publish)
	api.POST("/exams/invite/resolve", examHandler.resolveInvite)
	api.POST("/exams/:id/attempts/start", examHandler.startAttempt)
	api.POST("/exam-attempts/:attempt_id/answers/:attempt_question_id", examHandler.saveAnswer)
	api.POST("/exam-attempts/:attempt_id/submit", examHandler.submitAttempt)
	api.POST("/exam-attempts/:attempt_id/events", examHandler.recordEvent)
	api.POST("/exam-entry/invite/resolve", examHandler.resolveInvite)
	api.POST("/exam-entry/exams/:id/attempts/start", examHandler.startAttempt)
	api.POST("/exam-entry/attempts/:attempt_id/answers/:attempt_question_id", examEntryTokenMiddleware(deps.Taking), examHandler.saveAnswer)
	api.POST("/exam-entry/attempts/:attempt_id/submit", examEntryTokenMiddleware(deps.Taking), examHandler.submitAttempt)
	api.POST("/exam-entry/attempts/:attempt_id/events", examEntryTokenMiddleware(deps.Taking), examHandler.recordEvent)
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

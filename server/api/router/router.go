package router

import (
	"context"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/lifei6671/papermind/server/api/middleware"
	dbdao "github.com/lifei6671/papermind/server/internal/dao/db"
	serviceexam "github.com/lifei6671/papermind/server/internal/service/exam"
	servicepaper "github.com/lifei6671/papermind/server/internal/service/paper"
	"github.com/lifei6671/papermind/server/internal/service/permission"
	serviceplatformuser "github.com/lifei6671/papermind/server/internal/service/platformuser"
	servicequestion "github.com/lifei6671/papermind/server/internal/service/question"
	servicespace "github.com/lifei6671/papermind/server/internal/service/space"
	servicetenant "github.com/lifei6671/papermind/server/internal/service/tenant"
	servicetenantuser "github.com/lifei6671/papermind/server/internal/service/tenantuser"
	"github.com/lifei6671/papermind/server/internal/storage"
	"gorm.io/gorm"
)

// Options 汇总 HTTP 路由启动所需的外部配置。
//
// router 包只负责装配基础设施依赖和 Gin 路由骨架，具体版本 API 的
// handler 注册由调用方通过 RegisterFunc 注入，避免核心路由包反向依赖 v1。
type Options struct {
	DB                     *gorm.DB
	Now                    func() int64
	CodeGenerator          serviceexam.CodeGenerator
	AllowRegisterDefault   bool
	AuthSessionStore       sessions.Store
	AuthSessionProvider    string
	AuthSessionSecret      string
	AuthSessionKeyPrefix   string
	AuthSessionRedisAddr   string
	AuthSessionRedisUser   string
	AuthSessionRedisPass   string
	AuthSessionRedisDB     int
	AuthSessionTTL         int
	ExportDir              string
	UploadDir              string
	UploadStore            storage.ObjectStore
	PasswordMinLength      int
	ExamTokenBufferMinutes int
}

// Dependencies 是版本路由注册时需要使用的后端服务和仓储能力。
//
// 这里保留仓储字段，是因为部分 HTTP 权限边界需要反查资源真实空间；
// 数据库读写仍由 dao/db 层承载，handler 只依赖这些显式接口。
type Dependencies struct {
	PlatformUsers          *serviceplatformuser.Service
	TenantUsers            *servicetenantuser.Service
	Spaces                 *servicespace.Service
	Tenants                *servicetenant.Service
	Exams                  *serviceexam.Service
	Taking                 *serviceexam.TakingService
	Review                 *serviceexam.ReviewService
	Export                 *serviceexam.ExportService
	Results                *serviceexam.ResultService
	Questions              *servicequestion.QuestionService
	Papers                 *servicepaper.Service
	PaperRepository        *dbdao.PaperRepository
	SpaceRepository        *dbdao.SpaceRepository
	UploadStore            storage.ObjectStore
	UploadDir              string
	Now                    func() int64
	PasswordMinLength      int
	AuthSessionTTL         int
	ExamTokenBufferMinutes int
}

// RegisterFunc 将具体 API 版本的路由注册到指定分组。
type RegisterFunc func(api *gin.RouterGroup, deps Dependencies)

// New 创建完整 Gin Engine，并把 /api/v1 的业务路由注册委托给 register。
func New(options Options, register RegisterFunc, authMiddlewares ...gin.HandlerFunc) *gin.Engine {
	deps := buildDependencies(options)

	engine := gin.New()
	engine.Use(middleware.RequestID(), middleware.RequestLogger(), middleware.Recovery())
	if len(authMiddlewares) > 0 {
		engine.Use(authMiddlewares...)
	}
	engine.StaticFS("/uploads", gin.Dir(deps.UploadDir, false))

	api := engine.Group("/api/v1")
	register(api, deps)
	return engine
}

func buildDependencies(options Options) Dependencies {
	platformUserRepository := dbdao.NewPlatformUserRepository(options.DB, dbdao.PlatformUserRepositoryOptions{Now: options.Now})
	platformUserService := serviceplatformuser.NewService(serviceplatformuser.ServiceOptions{
		Repo: platformUserRepository,
		Now:  options.Now,
	})

	examRepository := dbdao.NewExamRepository(options.DB, dbdao.ExamRepositoryOptions{Now: options.Now})
	examService := serviceexam.NewService(serviceexam.ServiceOptions{
		Repo:                   examRepository,
		Now:                    options.Now,
		CodeGenerator:          options.CodeGenerator,
		ExamTokenBufferMinutes: options.ExamTokenBufferMinutes,
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

	uploadDir := defaultUploadDir(options.UploadDir)
	return Dependencies{
		PlatformUsers:          platformUserService,
		TenantUsers:            userService,
		Spaces:                 spaceService,
		Tenants:                tenantService,
		Exams:                  examService,
		Taking:                 takingService,
		Review:                 reviewService,
		Export:                 exportService,
		Results:                resultService,
		Questions:              questionService,
		Papers:                 paperService,
		PaperRepository:        paperRepository,
		SpaceRepository:        spaceRepository,
		UploadStore:            defaultUploadStore(options.UploadStore, uploadDir),
		UploadDir:              uploadDir,
		Now:                    defaultNow(options.Now),
		PasswordMinLength:      options.PasswordMinLength,
		AuthSessionTTL:         options.AuthSessionTTL,
		ExamTokenBufferMinutes: options.ExamTokenBufferMinutes,
	}
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

func defaultNow(now func() int64) func() int64 {
	if now != nil {
		return now
	}
	return func() int64 { return time.Now().UnixMilli() }
}

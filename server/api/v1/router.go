package v1

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/papermind/server/api/middleware"
	dbdao "github.com/lifei6671/papermind/server/internal/dao/db"
	serviceexam "github.com/lifei6671/papermind/server/internal/service/exam"
	"github.com/lifei6671/papermind/server/internal/service/pagination"
	servicequestion "github.com/lifei6671/papermind/server/internal/service/question"
	servicespace "github.com/lifei6671/papermind/server/internal/service/space"
	servicetenant "github.com/lifei6671/papermind/server/internal/service/tenant"
	servicetenantuser "github.com/lifei6671/papermind/server/internal/service/tenantuser"
	"github.com/lifei6671/papermind/server/library/code"
	"github.com/lifei6671/papermind/server/library/response"
	"gorm.io/gorm"
)

type RouterOptions struct {
	DB                   *gorm.DB
	Now                  func() int64
	CodeGenerator        serviceexam.CodeGenerator
	AllowRegisterDefault bool
}

func NewRouter(options RouterOptions) *gin.Engine {
	examRepository := dbdao.NewExamRepository(options.DB, dbdao.ExamRepositoryOptions{Now: options.Now})
	examService := serviceexam.NewService(serviceexam.ServiceOptions{
		Repo:          examRepository,
		Now:           options.Now,
		CodeGenerator: options.CodeGenerator,
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
	userRepository := dbdao.NewTenantUserRepository(options.DB, dbdao.TenantUserRepositoryOptions{Now: options.Now})
	userService := servicetenantuser.NewService(servicetenantuser.ServiceOptions{
		Repo:                       userRepository,
		SpaceAdminInvariantChecker: spaceRepository,
		Now:                        options.Now,
	})
	examHandler := examHandler{service: examService}
	tenantHandler := tenantHandler{service: tenantService}
	spaceHandler := spaceHandler{service: spaceService, members: spaceRepository}
	userHandler := userHandler{service: userService}
	questionHandler := questionHandler{service: questionService}

	router := gin.New()
	router.Use(middleware.RequestID(), middleware.Recovery())
	api := router.Group("/api/v1")
	api.GET("/exams", examHandler.list)
	api.POST("/exams", examHandler.publish)
	api.GET("/tenants", tenantHandler.list)
	api.POST("/tenants", tenantHandler.create)
	api.GET("/spaces", spaceHandler.list)
	api.POST("/spaces", spaceHandler.create)
	api.GET("/users", userHandler.list)
	api.POST("/users", userHandler.create)
	api.POST("/users/:id/disable", userHandler.disable)
	api.GET("/questions", questionHandler.list)
	api.POST("/questions", questionHandler.create)
	return router
}

type examHandler struct {
	service *serviceexam.Service
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

func writeExamServiceError(c *gin.Context, err error) {
	if errors.Is(err, serviceexam.ErrDurationExceedsExamWindow) ||
		errors.Is(err, serviceexam.ErrShortTextCannotRepeatAttempt) ||
		errors.Is(err, serviceexam.ErrShortTextCannotImmediateScore) ||
		errors.Is(err, serviceexam.ErrDuplicateExamTarget) {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, err.Error()))
		return
	}
	c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "考试操作失败"))
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

type tenantHandler struct {
	service *servicetenant.Service
}

type createTenantRequest struct {
	Name        string `json:"name"`
	LogoURL     string `json:"logo_url"`
	Description string `json:"description"`
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
	result, err := h.service.List(c.Request.Context(), servicetenant.ListInput{Page: page, PageSize: pageSize})
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "读取租户列表失败"))
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
	var request createTenantRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "请求体不是合法 JSON"))
		return
	}
	if request.Name == "" {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "name 不能为空"))
		return
	}
	tenant, err := h.service.Create(c.Request.Context(), servicetenant.CreateInput{
		Name:        request.Name,
		LogoURL:     request.LogoURL,
		Description: request.Description,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "创建租户失败"))
		return
	}
	c.JSON(http.StatusOK, response.OK(tenantToResponse(tenant)))
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
		name := member.Name
		if name == "" {
			name = "用户 " + strconv.FormatUint(member.UserID, 10)
		}
		result.Members = append(result.Members, spaceMemberResponse{
			ID:     member.ID,
			UserID: member.UserID,
			Name:   name,
			Role:   member.Role,
			Status: member.Status,
		})
	}
	return result, nil
}

func writeSpaceServiceError(c *gin.Context, err error) {
	if errors.Is(err, servicespace.ErrSpaceAdminRequired) {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, err.Error()))
		return
	}
	c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "空间操作失败"))
}

type userHandler struct {
	service *servicetenantuser.Service
}

type createUserRequest struct {
	TenantID  uint64 `json:"tenant_id"`
	Username  string `json:"username"`
	RealName  string `json:"real_name"`
	AvatarURL string `json:"avatar_url"`
	Role      string `json:"role"`
}

type disableUserRequest struct {
	TenantID uint64 `json:"tenant_id"`
	ActorID  uint64 `json:"actor_id"`
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
	if err := request.validate(); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, err.Error()))
		return
	}
	user, err := h.service.Create(c.Request.Context(), servicetenantuser.CreateInput{
		TenantID:     request.TenantID,
		Username:     request.Username,
		RealName:     request.RealName,
		AvatarURL:    request.AvatarURL,
		PasswordHash: "temporary-password-hash",
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
	if err := h.service.Disable(c.Request.Context(), servicetenantuser.DisableInput{
		TenantID: request.TenantID,
		ActorID:  request.ActorID,
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

func (r createUserRequest) validate() error {
	if r.TenantID == 0 {
		return errors.New("tenant_id 必须是正整数")
	}
	if r.Username == "" {
		return errors.New("username 不能为空")
	}
	if r.RealName == "" {
		return errors.New("real_name 不能为空")
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
	created, err := h.service.CreateQuestion(c.Request.Context(), servicequestion.CreateQuestionInput{
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

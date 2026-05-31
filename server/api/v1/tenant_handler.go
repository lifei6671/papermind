package v1

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/papermind/server/api/middleware"
	dbdao "github.com/lifei6671/papermind/server/internal/dao/db"
	servicespace "github.com/lifei6671/papermind/server/internal/service/space"
	servicetenant "github.com/lifei6671/papermind/server/internal/service/tenant"
	servicetenantuser "github.com/lifei6671/papermind/server/internal/service/tenantuser"
	"github.com/lifei6671/papermind/server/library/code"
	"github.com/lifei6671/papermind/server/library/crypto"
	"github.com/lifei6671/papermind/server/library/response"
)

// tenantHandler 处理平台管理员视角的租户治理接口。
//
// 这些接口只能由 platform_admin 访问，租户用户不能通过构造 tenant_id
// 进入平台治理能力。
type tenantHandler struct {
	service           *servicetenant.Service
	spaces            *servicespace.Service
	users             *servicetenantuser.Service
	members           *dbdao.SpaceRepository
	passwordMinLength int
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

func (h tenantHandler) listTenantSpaces(c *gin.Context) {
	if _, ok := requirePlatformActor(c); !ok {
		return
	}
	tenantID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "租户 ID 必须是正整数"))
		return
	}
	page, pageSize, err := readPaginationQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "page 和 page_size 必须是正整数"))
		return
	}
	result, err := h.spaces.List(c.Request.Context(), servicespace.ListInput{TenantID: tenantID, Page: page, PageSize: pageSize})
	if err != nil {
		writeTenantInternalError(c, "读取租户空间列表失败", err)
		return
	}
	items := make([]spaceResponse, 0, len(result.Items))
	for _, space := range result.Items {
		item, err := (spaceHandler{members: h.members}).spaceToResponse(c.Request.Context(), space)
		if err != nil {
			writeTenantInternalError(c, "读取租户空间成员失败", err)
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

func (h tenantHandler) listTenantUsers(c *gin.Context) {
	if _, ok := requirePlatformActor(c); !ok {
		return
	}
	tenantID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "租户 ID 必须是正整数"))
		return
	}
	page, pageSize, err := readPaginationQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "page 和 page_size 必须是正整数"))
		return
	}
	result, err := h.users.List(c.Request.Context(), servicetenantuser.ListInput{TenantID: tenantID, Page: page, PageSize: pageSize})
	if err != nil {
		writeTenantInternalError(c, "读取租户用户列表失败", err)
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
	if err := validatePasswordMinLength(request.AdminPassword, h.passwordMinLength); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, err.Error()))
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

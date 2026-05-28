package v1

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	servicespace "github.com/lifei6671/papermind/server/internal/service/space"
	servicetenantuser "github.com/lifei6671/papermind/server/internal/service/tenantuser"
	"github.com/lifei6671/papermind/server/library/code"
	"github.com/lifei6671/papermind/server/library/crypto"
	"github.com/lifei6671/papermind/server/library/response"
)

// userHandler 处理租户用户管理接口。
//
// 创建教师不会自动写入 space_members；教师必须通过空间成员入口分配后
// 才能获得对应空间的考试业务权限。
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

package v1

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/papermind/server/internal/service/permission"
	serviceplatformuser "github.com/lifei6671/papermind/server/internal/service/platformuser"
	servicespace "github.com/lifei6671/papermind/server/internal/service/space"
	servicetenantuser "github.com/lifei6671/papermind/server/internal/service/tenantuser"
	"github.com/lifei6671/papermind/server/library/code"
	"github.com/lifei6671/papermind/server/library/crypto"
	"github.com/lifei6671/papermind/server/library/response"
)

const platformAdminRole = permission.RolePlatformAdmin

type authHandler struct {
	platformUsers        *serviceplatformuser.Service
	tenantUsers          *servicetenantuser.Service
	spaces               *servicespace.Service
	sessionMaxAgeSeconds int
	passwordMinLength    int
}

type platformLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type tenantLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type tenantSpaceSelectRequest struct {
	TenantID uint64 `json:"tenant_id"`
	SpaceID  uint64 `json:"space_id"`
}

type tenantRegisterRequest struct {
	TenantCode string `json:"tenant_code"`
	Username   string `json:"username"`
	RealName   string `json:"real_name"`
	Password   string `json:"password"`
	Phone      string `json:"phone"`
	Email      string `json:"email"`
}

type authSessionResponse struct {
	User authUserResponse `json:"user"`
}

type authUserResponse struct {
	UserID      uint64 `json:"user_id"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	TenantID    uint64 `json:"tenant_id,omitempty"`
}

type profileRequest struct {
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url"`
	Phone       string `json:"phone"`
	Email       string `json:"email"`
}

type profileResponse struct {
	UserID      uint64 `json:"user_id"`
	TenantID    uint64 `json:"tenant_id,omitempty"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url"`
	Phone       string `json:"phone"`
	Email       string `json:"email"`
	Role        string `json:"role"`
	SubjectType string `json:"subject_type"`
}

type profileSpaceResponse struct {
	ID         uint64 `json:"id"`          // 空间成员记录 ID；租户管理员入口没有成员记录时为 0。
	TenantID   uint64 `json:"tenant_id"`   // 成员关系所属租户，前端只能把它作为当前 session 租户范围使用。
	TenantName string `json:"tenant_name"` // 租户名称，用于登录后空间选择页展示。
	SpaceID    uint64 `json:"space_id"`    // 当前用户可进入且启用的空间 ID，用于后续业务页面携带 space_id。
	SpaceName  string `json:"space_name"`  // 空间名称，用于登录后空间选择页展示。
	Role       string `json:"role"`        // 当前用户进入该空间时的身份来源：tenant_admin / space_admin / teacher / student。
	Status     string `json:"status"`      // 成员关系状态；本接口只返回 enabled，保留字段便于前端统一展示。
}

type profileSpaceListResponse struct {
	Items []profileSpaceResponse `json:"items"`
}

func (h authHandler) getProfile(c *gin.Context) {
	principal, ok := currentAuthPrincipal(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, response.Fail(code.InvalidParam, "请先登录"))
		return
	}
	if principal.SubjectType == permission.SubjectPlatformUser {
		user, err := h.livePlatformProfileUser(c, principal.UserID)
		if err != nil {
			writeProfileError(c, err)
			return
		}
		c.JSON(http.StatusOK, response.OK(platformProfileToResponse(user)))
		return
	}
	user, err := h.liveTenantProfileUser(c, principal)
	if err != nil {
		writeProfileError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(tenantProfileToResponse(user)))
}

func (h authHandler) listProfileSpaces(c *gin.Context) {
	principal, ok := currentAuthPrincipal(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, response.Fail(code.InvalidParam, "请先登录"))
		return
	}
	// 平台管理员没有租户空间身份，不能把平台主体伪装成空间管理员或教师来驱动租户业务菜单。
	if principal.SubjectType != permission.SubjectTenantUser {
		c.JSON(http.StatusForbidden, response.Fail(code.InvalidParam, "平台管理员没有租户空间授权"))
		return
	}

	// 通用租户账号登录后没有 tenant_id，入口页需要先列出所有可进入空间；
	// 已绑定租户的 session 仍复用同一查询，再按当前租户收窄前端菜单范围。
	members, err := h.spaces.ListEntryMembershipsForUser(c.Request.Context(), principal.UserID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "读取当前用户空间授权失败"))
		return
	}
	items := make([]profileSpaceResponse, 0, len(members))
	for _, member := range members {
		if principal.TenantID != 0 && member.TenantID != principal.TenantID {
			continue
		}
		items = append(items, profileSpaceToResponse(member))
	}
	c.JSON(http.StatusOK, response.OK(profileSpaceListResponse{Items: items}))
}

func (h authHandler) updateProfile(c *gin.Context) {
	principal, ok := currentAuthPrincipal(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, response.Fail(code.InvalidParam, "请先登录"))
		return
	}
	var request profileRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "请求体不是合法 JSON"))
		return
	}
	displayName := strings.TrimSpace(request.DisplayName)
	if displayName == "" {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "display_name 不能为空"))
		return
	}
	avatarURL := strings.TrimSpace(request.AvatarURL)
	phone := strings.TrimSpace(request.Phone)
	email := strings.TrimSpace(request.Email)
	if principal.SubjectType == permission.SubjectPlatformUser {
		if _, err := h.livePlatformProfileUser(c, principal.UserID); err != nil {
			writeProfileError(c, err)
			return
		}
		user, err := h.platformUsers.UpdateProfile(c.Request.Context(), serviceplatformuser.UpdateProfileInput{
			UserID:      principal.UserID,
			DisplayName: displayName,
			AvatarURL:   avatarURL,
			Phone:       phone,
			Email:       email,
		})
		if err != nil {
			writeProfileError(c, err)
			return
		}
		c.JSON(http.StatusOK, response.OK(platformProfileToResponse(user)))
		return
	}
	if _, err := h.liveTenantProfileUser(c, principal); err != nil {
		writeProfileError(c, err)
		return
	}
	user, err := h.tenantUsers.UpdateProfile(c.Request.Context(), servicetenantuser.UpdateProfileInput{
		TenantID:    principal.TenantID,
		UserID:      principal.UserID,
		DisplayName: displayName,
		AvatarURL:   avatarURL,
		Phone:       phone,
		Email:       email,
	})
	if err != nil {
		writeProfileError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(tenantProfileToResponse(user)))
}

func (h authHandler) livePlatformProfileUser(c *gin.Context, userID uint64) (serviceplatformuser.PlatformUser, error) {
	user, err := h.platformUsers.Get(c.Request.Context(), userID)
	if err != nil {
		return serviceplatformuser.PlatformUser{}, err
	}
	if user.Status != serviceplatformuser.StatusEnabled {
		return serviceplatformuser.PlatformUser{}, permission.ErrForbidden
	}
	return user, nil
}

func (h authHandler) liveTenantProfileUser(c *gin.Context, principal AuthPrincipal) (servicetenantuser.User, error) {
	if principal.SubjectType != permission.SubjectTenantUser {
		return servicetenantuser.User{}, permission.ErrForbidden
	}
	if principal.TenantID == 0 {
		user, err := h.tenantUsers.GetGlobal(c.Request.Context(), principal.UserID)
		if err != nil {
			return servicetenantuser.User{}, err
		}
		if user.Status != servicetenantuser.StatusEnabled {
			return servicetenantuser.User{}, permission.ErrForbidden
		}
		return user, nil
	}
	user, err := h.tenantUsers.Get(c.Request.Context(), principal.TenantID, principal.UserID)
	if err != nil {
		return servicetenantuser.User{}, err
	}
	if user.Status != servicetenantuser.StatusEnabled || user.Role != principal.Role {
		return servicetenantuser.User{}, permission.ErrForbidden
	}
	return user, nil
}

func (h authHandler) platformLogin(c *gin.Context) {
	var request platformLoginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "请求体不是合法 JSON"))
		return
	}
	if request.Username == "" || request.Password == "" {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "账号和密码不能为空"))
		return
	}

	user, err := h.platformUsers.Login(c.Request.Context(), serviceplatformuser.LoginInput{
		Username: request.Username,
		Password: request.Password,
		IP:       c.ClientIP(),
	})
	if err != nil {
		writePlatformLoginError(c, err)
		return
	}
	principal := AuthPrincipal{
		SubjectType: permission.SubjectPlatformUser,
		UserID:      user.ID,
		Role:        platformAdminRole,
	}
	if _, err := saveAuthPrincipalSession(c, principal, defaultSessionMaxAgeSeconds(h.sessionMaxAgeSeconds)); err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "保存登录会话失败"))
		return
	}

	// 平台管理员登录成功后只返回前端会话所需身份，不把密码哈希、审计字段暴露给浏览器。
	c.JSON(http.StatusOK, response.OK(authSessionResponse{
		User: authUserResponse{
			UserID:      user.ID,
			DisplayName: user.Username,
			Role:        platformAdminRole,
		},
	}))
}

func (h authHandler) tenantLogin(c *gin.Context) {
	var request tenantLoginRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "请求体不是合法 JSON"))
		return
	}
	if request.Username == "" || request.Password == "" {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "账号和密码不能为空"))
		return
	}

	user, err := h.tenantUsers.Login(c.Request.Context(), servicetenantuser.LoginInput{
		Username: request.Username,
		Password: request.Password,
		IP:       c.ClientIP(),
	})
	if err != nil {
		writeTenantLoginError(c, err)
		return
	}
	principal := AuthPrincipal{
		SubjectType: permission.SubjectTenantUser,
		UserID:      user.ID,
		Role:        servicetenantuser.RoleTenantUser,
	}
	if _, err := saveAuthPrincipalSession(c, principal, defaultSessionMaxAgeSeconds(h.sessionMaxAgeSeconds)); err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "保存登录会话失败"))
		return
	}
	c.JSON(http.StatusOK, response.OK(authSessionResponse{
		User: authUserResponse{
			UserID:      user.ID,
			DisplayName: user.RealName,
			Role:        servicetenantuser.RoleTenantUser,
		},
	}))
}

func (h authHandler) selectTenantSpace(c *gin.Context) {
	principal, ok := currentAuthPrincipal(c)
	if !ok || principal.SubjectType != permission.SubjectTenantUser {
		c.JSON(http.StatusUnauthorized, response.Fail(code.InvalidParam, "请先登录租户用户账号"))
		return
	}
	var request tenantSpaceSelectRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "请求体不是合法 JSON"))
		return
	}
	if request.TenantID == 0 {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id 必须是正整数"))
		return
	}

	// 进入空间前先校验全局账号在目标租户的启用成员关系，之后才把 tenant_id 写入 session。
	user, err := h.tenantUsers.Get(c.Request.Context(), request.TenantID, principal.UserID)
	if err != nil {
		writeTenantSpaceSelectError(c, err)
		return
	}
	if user.Status != servicetenantuser.StatusEnabled {
		c.JSON(http.StatusForbidden, response.Fail(code.InvalidParam, "当前租户账号已禁用"))
		return
	}
	if user.Role == servicetenantuser.RoleTenantAdmin {
		if request.SpaceID != 0 {
			exists, err := h.spaces.SpaceExists(c.Request.Context(), request.TenantID, request.SpaceID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "校验目标空间失败"))
				return
			}
			if !exists {
				c.JSON(http.StatusForbidden, response.Fail(code.InvalidParam, "无权进入该空间"))
				return
			}
		}
	} else if request.SpaceID == 0 || !hasSelectedSpace(c.Request.Context(), h.spaces, request.TenantID, request.SpaceID, principal.UserID) {
		c.JSON(http.StatusForbidden, response.Fail(code.InvalidParam, "无权进入该空间"))
		return
	}

	nextPrincipal := AuthPrincipal{
		SubjectType: permission.SubjectTenantUser,
		UserID:      user.ID,
		TenantID:    request.TenantID,
		Role:        user.Role,
	}
	if _, err := saveAuthPrincipalSession(c, nextPrincipal, defaultSessionMaxAgeSeconds(h.sessionMaxAgeSeconds)); err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "保存租户空间会话失败"))
		return
	}
	c.JSON(http.StatusOK, response.OK(authSessionResponse{
		User: authUserResponse{
			UserID:      user.ID,
			DisplayName: user.RealName,
			Role:        user.Role,
			TenantID:    request.TenantID,
		},
	}))
}

func (h authHandler) tenantRegister(c *gin.Context) {
	var request tenantRegisterRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "请求体不是合法 JSON"))
		return
	}
	if request.TenantCode == "" || request.Username == "" || request.RealName == "" || request.Password == "" {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "租户码、账号、姓名和密码不能为空"))
		return
	}
	if err := validatePasswordMinLength(request.Password, h.passwordMinLength); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, err.Error()))
		return
	}
	passwordHash, err := crypto.HashPassword(request.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "生成密码哈希失败"))
		return
	}

	// 租户码自注册只创建学生账号，不自动加入空间，后续空间归属由租户管理员维护。
	user, err := h.tenantUsers.RegisterWithTenantCode(c.Request.Context(), servicetenantuser.RegisterInput{
		TenantCode:   request.TenantCode,
		Username:     request.Username,
		RealName:     request.RealName,
		PasswordHash: passwordHash,
		Phone:        request.Phone,
		Email:        request.Email,
	})
	if err != nil {
		writeTenantRegisterError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(userToResponse(user)))
}

func profileSpaceToResponse(member servicespace.Member) profileSpaceResponse {
	return profileSpaceResponse{
		ID:         member.ID,
		TenantID:   member.TenantID,
		TenantName: member.TenantName,
		SpaceID:    member.SpaceID,
		SpaceName:  member.SpaceName,
		Role:       member.Role,
		Status:     member.Status,
	}
}

func hasSelectedSpace(ctx context.Context, spaces *servicespace.Service, tenantID uint64, spaceID uint64, userID uint64) bool {
	members, err := spaces.ListEffectiveMembershipsForUser(ctx, tenantID, userID)
	if err != nil {
		return false
	}
	for _, member := range members {
		if member.SpaceID == spaceID {
			return true
		}
	}
	return false
}

func platformProfileToResponse(user serviceplatformuser.PlatformUser) profileResponse {
	return profileResponse{
		UserID:      user.ID,
		DisplayName: user.Username,
		AvatarURL:   user.AvatarURL,
		Phone:       user.Phone,
		Email:       user.Email,
		Role:        platformAdminRole,
		SubjectType: permission.SubjectPlatformUser,
	}
}

func tenantProfileToResponse(user servicetenantuser.User) profileResponse {
	return profileResponse{
		UserID:      user.ID,
		TenantID:    user.TenantID,
		DisplayName: user.RealName,
		AvatarURL:   user.AvatarURL,
		Phone:       user.Phone,
		Email:       user.Email,
		Role:        user.Role,
		SubjectType: permission.SubjectTenantUser,
	}
}

func writeProfileError(c *gin.Context, err error) {
	if errors.Is(err, permission.ErrForbidden) {
		c.JSON(http.StatusForbidden, response.Fail(code.InvalidParam, "无权执行当前操作"))
		return
	}
	if errors.Is(err, serviceplatformuser.ErrDisplayNameRequired) ||
		errors.Is(err, servicetenantuser.ErrDisplayNameRequired) {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "display_name 不能为空"))
		return
	}
	if errors.Is(err, serviceplatformuser.ErrPlatformUserNotFound) ||
		errors.Is(err, servicetenantuser.ErrUserNotFound) {
		c.JSON(http.StatusNotFound, response.Fail(code.InvalidParam, "当前用户不存在"))
		return
	}
	c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "个人资料操作失败"))
}

func validatePasswordMinLength(password string, passwordMinLength int) error {
	minLength := normalizedPasswordMinLength(passwordMinLength)
	if len([]rune(password)) < minLength {
		return fmt.Errorf("密码至少需要 %d 位", minLength)
	}
	return nil
}

func normalizedPasswordMinLength(passwordMinLength int) int {
	if passwordMinLength > 0 {
		return passwordMinLength
	}
	return 1
}

func writePlatformLoginError(c *gin.Context, err error) {
	if errors.Is(err, serviceplatformuser.ErrInvalidCredential) ||
		errors.Is(err, serviceplatformuser.ErrPlatformUserNotFound) {
		c.JSON(http.StatusUnauthorized, response.Fail(code.InvalidParam, "账号或密码不正确"))
		return
	}
	if errors.Is(err, serviceplatformuser.ErrPlatformUserDisabled) {
		c.JSON(http.StatusForbidden, response.Fail(code.InvalidParam, "平台管理员账号已禁用"))
		return
	}
	c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "平台管理员登录失败"))
}

func writeTenantLoginError(c *gin.Context, err error) {
	if errors.Is(err, servicetenantuser.ErrInvalidCredential) ||
		errors.Is(err, servicetenantuser.ErrUserNotFound) {
		c.JSON(http.StatusUnauthorized, response.Fail(code.InvalidParam, "账号或密码不正确"))
		return
	}
	if errors.Is(err, servicetenantuser.ErrUserDisabled) {
		c.JSON(http.StatusForbidden, response.Fail(code.InvalidParam, "租户用户账号已禁用"))
		return
	}
	c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "租户用户登录失败"))
}

func writeTenantSpaceSelectError(c *gin.Context, err error) {
	if errors.Is(err, servicetenantuser.ErrUserNotFound) {
		c.JSON(http.StatusForbidden, response.Fail(code.InvalidParam, "无权进入该空间"))
		return
	}
	c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "选择租户空间失败"))
}

func writeTenantRegisterError(c *gin.Context, err error) {
	if errors.Is(err, servicetenantuser.ErrTenantNotFound) {
		c.JSON(http.StatusNotFound, response.Fail(code.InvalidParam, "租户码不存在"))
		return
	}
	if errors.Is(err, servicetenantuser.ErrTenantDisabled) {
		c.JSON(http.StatusForbidden, response.Fail(code.InvalidParam, "租户已禁用"))
		return
	}
	if errors.Is(err, servicetenantuser.ErrRegisterNotAllowed) {
		c.JSON(http.StatusForbidden, response.Fail(code.InvalidParam, "当前租户不允许自注册"))
		return
	}
	c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "租户用户注册失败"))
}

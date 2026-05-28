package v1

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/papermind/server/internal/service/permission"
	serviceplatformuser "github.com/lifei6671/papermind/server/internal/service/platformuser"
	servicetenantuser "github.com/lifei6671/papermind/server/internal/service/tenantuser"
	"github.com/lifei6671/papermind/server/library/code"
	"github.com/lifei6671/papermind/server/library/crypto"
	"github.com/lifei6671/papermind/server/library/response"
)

const platformAdminRole = permission.RolePlatformAdmin

type authHandler struct {
	platformUsers        *serviceplatformuser.Service
	tenantUsers          *servicetenantuser.Service
	sessionMaxAgeSeconds int
	passwordMinLength    int
}

type platformLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type tenantLoginRequest struct {
	TenantID uint64 `json:"tenant_id"`
	Username string `json:"username"`
	Password string `json:"password"`
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
	AccessToken  string           `json:"access_token"`
	RefreshToken string           `json:"refresh_token"`
	User         authUserResponse `json:"user"`
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

func (h authHandler) getProfile(c *gin.Context) {
	principal, ok := currentAuthPrincipal(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, response.Fail(code.InvalidParam, "请先登录"))
		return
	}
	if principal.SubjectType == permission.SubjectPlatformUser {
		user, err := h.platformUsers.Get(c.Request.Context(), principal.UserID)
		if err != nil {
			writeProfileError(c, err)
			return
		}
		c.JSON(http.StatusOK, response.OK(platformProfileToResponse(user)))
		return
	}
	user, err := h.tenantUsers.Get(c.Request.Context(), principal.TenantID, principal.UserID)
	if err != nil {
		writeProfileError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(tenantProfileToResponse(user)))
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
	accessToken, err := saveAuthPrincipalSession(c, principal, defaultSessionMaxAgeSeconds(h.sessionMaxAgeSeconds))
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "保存登录会话失败"))
		return
	}

	// 平台管理员登录成功后只返回前端会话所需身份，不把密码哈希、审计字段暴露给浏览器。
	c.JSON(http.StatusOK, response.OK(authSessionResponse{
		AccessToken:  accessToken,
		RefreshToken: accessToken,
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
	if request.TenantID == 0 || request.Username == "" || request.Password == "" {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "租户 ID、账号和密码不能为空"))
		return
	}

	user, err := h.tenantUsers.Login(c.Request.Context(), servicetenantuser.LoginInput{
		TenantID: request.TenantID,
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
		TenantID:    user.TenantID,
		Role:        user.Role,
	}
	accessToken, err := saveAuthPrincipalSession(c, principal, defaultSessionMaxAgeSeconds(h.sessionMaxAgeSeconds))
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "保存登录会话失败"))
		return
	}
	c.JSON(http.StatusOK, response.OK(authSessionResponse{
		AccessToken:  accessToken,
		RefreshToken: accessToken,
		User: authUserResponse{
			UserID:      user.ID,
			DisplayName: user.RealName,
			Role:        user.Role,
			TenantID:    user.TenantID,
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

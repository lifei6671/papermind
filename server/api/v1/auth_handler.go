package v1

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	serviceplatformuser "github.com/lifei6671/papermind/server/internal/service/platformuser"
	"github.com/lifei6671/papermind/server/library/code"
	"github.com/lifei6671/papermind/server/library/response"
)

const platformAdminRole = "platform_admin"

type AuthTokenIssuer interface {
	IssueAccessToken() (string, error)
	IssueRefreshToken() (string, error)
}

type authHandler struct {
	platformUsers *serviceplatformuser.Service
	tokenIssuer   AuthTokenIssuer
}

type platformLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
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
	accessToken, err := h.tokenIssuer.IssueAccessToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "签发登录凭证失败"))
		return
	}
	refreshToken, err := h.tokenIssuer.IssueRefreshToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "签发登录凭证失败"))
		return
	}

	// 平台管理员登录成功后只返回前端会话所需身份，不把密码哈希、审计字段暴露给浏览器。
	c.JSON(http.StatusOK, response.OK(authSessionResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User: authUserResponse{
			UserID:      user.ID,
			DisplayName: user.Username,
			Role:        platformAdminRole,
		},
	}))
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

func defaultAuthTokenIssuer(issuer AuthTokenIssuer) AuthTokenIssuer {
	if issuer != nil {
		return issuer
	}
	return randomAuthTokenIssuer{}
}

type randomAuthTokenIssuer struct{}

func (randomAuthTokenIssuer) IssueAccessToken() (string, error) {
	return issueOpaqueToken()
}

func (randomAuthTokenIssuer) IssueRefreshToken() (string, error) {
	return issueOpaqueToken()
}

func issueOpaqueToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

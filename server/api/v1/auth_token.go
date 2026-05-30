package v1

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/memstore"
	redisstore "github.com/gin-contrib/sessions/redis"
	"github.com/gin-gonic/gin"
	"github.com/lifei6671/papermind/server/internal/service/permission"
	"github.com/lifei6671/papermind/server/library/code"
	"github.com/lifei6671/papermind/server/library/response"
)

const (
	authPrincipalGinKey = "auth_principal"
	authSessionName     = "papermind.sid"

	authSessionSubjectTypeKey = "actor_type"
	authSessionUserIDKey      = "actor_id"
	authSessionTenantIDKey    = "tenant_id"
	authSessionRoleKey        = "role"
)

var (
	errAuthSessionProviderUnsupported = errors.New("auth session provider unsupported")
	errAuthSessionCookieMissing       = errors.New("auth session cookie missing")
)

type authPrincipalContextKey struct{}

type AuthPrincipal struct {
	SubjectType string `json:"subject_type"`
	UserID      uint64 `json:"user_id"`
	TenantID    uint64 `json:"tenant_id,omitempty"`
	Role        string `json:"role,omitempty"`
}

type SessionStoreOptions struct {
	Provider string
	Secret   string
	Redis    RedisSessionStoreOptions
}

type RedisSessionStoreOptions struct {
	Addr      string
	Username  string
	Password  string
	DB        int
	KeyPrefix string
}

func NewSessionStore(options SessionStoreOptions) (sessions.Store, error) {
	provider := strings.ToLower(strings.TrimSpace(options.Provider))
	if provider == "" {
		provider = "memory"
	}
	secret, err := sessionSecret(options.Secret)
	if err != nil {
		return nil, err
	}
	switch provider {
	case "memory":
		return memstore.NewStore([]byte(secret)), nil
	case "redis":
		return newRedisSessionStore(secret, options.Redis)
	default:
		return nil, fmt.Errorf("%w: %s", errAuthSessionProviderUnsupported, provider)
	}
}

func sessionSecret(value string) (string, error) {
	secret := strings.TrimSpace(value)
	if secret == "" {
		generated, err := randomSessionSecret()
		if err != nil {
			return "", err
		}
		secret = generated
	}
	return secret, nil
}

func newRedisSessionStore(secret string, options RedisSessionStoreOptions) (sessions.Store, error) {
	addr := strings.TrimSpace(options.Addr)
	if addr == "" {
		return nil, errors.New("auth session redis addr is empty")
	}
	store, err := redisstore.NewStoreWithDB(
		10,
		"tcp",
		addr,
		strings.TrimSpace(options.Username),
		options.Password,
		strconv.Itoa(options.DB),
		[]byte(secret),
	)
	if err != nil {
		return nil, err
	}
	keyPrefix := strings.TrimSpace(options.KeyPrefix)
	if keyPrefix != "" {
		if err := redisstore.SetKeyPrefix(store, keyPrefix); err != nil {
			return nil, err
		}
	}
	return store, nil
}

func randomSessionSecret() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate session secret: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

func bearerSessionCookieMiddleware(sessionName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := strings.TrimSpace(c.GetHeader("Authorization"))
		if header == "" {
			c.Next()
			return
		}
		token, ok := bearerToken(header)
		if ok {
			replaceRequestCookie(c.Request, sessionName, token)
		}
		c.Next()
	}
}

func replaceRequestCookie(request *http.Request, name string, value string) {
	cookies := request.Cookies()
	request.Header.Del("Cookie")
	for _, cookie := range cookies {
		if cookie.Name != name {
			request.AddCookie(cookie)
		}
	}
	// 兼容已持有签名 session 值的调用方，进入 Gin session middleware 前统一映射成 cookie。
	request.AddCookie(&http.Cookie{Name: name, Value: value})
}

func authContextMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, ok := authPrincipalFromGinSession(c)
		if ok {
			c.Set(authPrincipalGinKey, principal)
			// 业务 service 后续可以直接从标准 context 中取当前主体，不依赖 Gin 类型。
			ctx := context.WithValue(c.Request.Context(), authPrincipalContextKey{}, principal)
			c.Request = c.Request.WithContext(ctx)
		}
		c.Next()
	}
}

func saveAuthPrincipalSession(c *gin.Context, principal AuthPrincipal, maxAgeSeconds int) (string, error) {
	session := sessions.Default(c)
	clearExamEntrySession(session)
	// 平台登录 session 是平台侧审计主体来源，不能让后续写接口再信任请求体里的 actor_id。
	session.Set(authSessionSubjectTypeKey, principal.SubjectType)
	session.Set(authSessionUserIDKey, principal.UserID)
	session.Set(authSessionTenantIDKey, principal.TenantID)
	session.Set(authSessionRoleKey, principal.Role)
	session.Options(sessions.Options{
		Path:     "/",
		MaxAge:   maxAgeSeconds,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	if err := session.Save(); err != nil {
		return "", err
	}
	token, ok := responseCookieValue(c.Writer.Header(), authSessionName)
	if !ok {
		return "", errAuthSessionCookieMissing
	}
	return token, nil
}

func clearExamEntrySession(session sessions.Session) {
	session.Delete(examEntrySessionTenantIDKey)
	session.Delete(examEntrySessionExamIDKey)
	session.Delete(examEntrySessionUserIDKey)
}

func responseCookieValue(header http.Header, name string) (string, bool) {
	resp := http.Response{Header: header}
	for _, cookie := range resp.Cookies() {
		if cookie.Name == name && cookie.Value != "" {
			return cookie.Value, true
		}
	}
	return "", false
}

func authPrincipalFromGinSession(c *gin.Context) (AuthPrincipal, bool) {
	session := sessions.Default(c)
	subjectType, ok := session.Get(authSessionSubjectTypeKey).(string)
	if !ok || subjectType == "" {
		return AuthPrincipal{}, false
	}
	userID, ok := sessionUint64(session.Get(authSessionUserIDKey))
	if !ok || userID == 0 {
		return AuthPrincipal{}, false
	}
	tenantID, _ := sessionUint64(session.Get(authSessionTenantIDKey))
	role, _ := session.Get(authSessionRoleKey).(string)
	return AuthPrincipal{
		SubjectType: subjectType,
		UserID:      userID,
		TenantID:    tenantID,
		Role:        role,
	}, true
}

func sessionUint64(value any) (uint64, bool) {
	switch v := value.(type) {
	case uint64:
		return v, true
	case uint:
		return uint64(v), true
	case int:
		if v < 0 {
			return 0, false
		}
		return uint64(v), true
	case int64:
		if v < 0 {
			return 0, false
		}
		return uint64(v), true
	case float64:
		if v < 0 {
			return 0, false
		}
		return uint64(v), true
	default:
		return 0, false
	}
}

func bearerToken(header string) (string, bool) {
	const prefix = "Bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", false
	}
	token := strings.TrimSpace(header[len(prefix):])
	return token, token != ""
}

func authPrincipalFromContext(ctx context.Context) (AuthPrincipal, bool) {
	principal, ok := ctx.Value(authPrincipalContextKey{}).(AuthPrincipal)
	return principal, ok
}

func requirePlatformActor(c *gin.Context) (uint64, bool) {
	if value, ok := c.Get(authPrincipalGinKey); ok {
		if principal, ok := value.(AuthPrincipal); ok &&
			principal.SubjectType == permission.SubjectPlatformUser &&
			principal.UserID != 0 {
			return principal.UserID, true
		}
	}
	if principal, ok := authPrincipalFromContext(c.Request.Context()); ok &&
		principal.SubjectType == permission.SubjectPlatformUser &&
		principal.UserID != 0 {
		return principal.UserID, true
	}
	c.JSON(http.StatusUnauthorized, response.Fail(code.InvalidParam, "请先登录平台管理员账号"))
	return 0, false
}

func requireAuthPrincipalMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := currentAuthPrincipal(c); !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, response.Fail(code.InvalidParam, "请先登录"))
			return
		}
		c.Next()
	}
}

func requirePlatformPrincipalMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, ok := currentAuthPrincipal(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, response.Fail(code.InvalidParam, "请先登录平台管理员账号"))
			return
		}
		if principal.SubjectType != permission.SubjectPlatformUser {
			c.AbortWithStatusJSON(http.StatusForbidden, response.Fail(code.InvalidParam, "仅平台管理员可操作"))
			return
		}
		c.Next()
	}
}

func requireTenantAdminOrPlatformPrincipalMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, ok := currentAuthPrincipal(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, response.Fail(code.InvalidParam, "请先登录"))
			return
		}
		if principal.SubjectType == permission.SubjectPlatformUser && principal.Role == permission.RolePlatformAdmin {
			c.Next()
			return
		}
		if principal.SubjectType == permission.SubjectTenantUser && principal.Role == permission.RoleTenantAdmin {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusForbidden, response.Fail(code.InvalidParam, "无权执行当前操作"))
	}
}

func requireExamBusinessPrincipalMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, ok := currentAuthPrincipal(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, response.Fail(code.InvalidParam, "请先登录"))
			return
		}
		if principal.SubjectType == permission.SubjectTenantUser &&
			principal.TenantID != 0 &&
			validTenantExamSessionRole(principal.Role) {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusForbidden, response.Fail(code.InvalidParam, "无权执行当前操作"))
	}
}

func validTenantExamSessionRole(role string) bool {
	return role == permission.RoleTenantAdmin || role == permission.RoleTeacher || role == permission.RoleStudent
}

func currentAuthPrincipal(c *gin.Context) (AuthPrincipal, bool) {
	if value, ok := c.Get(authPrincipalGinKey); ok {
		if principal, ok := value.(AuthPrincipal); ok && principal.UserID != 0 {
			return principal, true
		}
	}
	if principal, ok := authPrincipalFromContext(c.Request.Context()); ok && principal.UserID != 0 {
		return principal, true
	}
	return AuthPrincipal{}, false
}

func defaultSessionMaxAgeSeconds(value int) int {
	if value > 0 {
		return value
	}
	return int(7 * 24 * time.Hour / time.Second)
}

package v1

import (
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

	apimiddleware "github.com/lifei6671/papermind/server/api/middleware"
	"github.com/lifei6671/papermind/server/internal/service/permission"
	"github.com/lifei6671/papermind/server/library/code"
	"github.com/lifei6671/papermind/server/library/response"
)

const authSessionName = "papermind.sid"

var (
	errAuthSessionProviderUnsupported = errors.New("auth session provider unsupported")
	errAuthSessionCookieMissing       = errors.New("auth session cookie missing")
)

type AuthPrincipal = apimiddleware.AuthPrincipal

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

func saveAuthPrincipalSession(c *gin.Context, principal AuthPrincipal, maxAgeSeconds int) (string, error) {
	session := sessions.Default(c)
	clearExamEntrySession(session)
	// 平台登录 session 是平台侧审计主体来源，不能让后续写接口再信任请求体里的 actor_id。
	session.Set(apimiddleware.AuthSessionSubjectTypeKey, principal.SubjectType)
	session.Set(apimiddleware.AuthSessionUserIDKey, principal.UserID)
	session.Set(apimiddleware.AuthSessionTenantIDKey, principal.TenantID)
	session.Set(apimiddleware.AuthSessionRoleKey, principal.Role)
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

func requirePlatformActor(c *gin.Context) (uint64, bool) {
	principal, ok := currentAuthPrincipal(c)
	if ok && principal.SubjectType == permission.SubjectPlatformUser && principal.UserID != 0 {
		return principal.UserID, true
	}
	c.JSON(http.StatusUnauthorized, response.Fail(code.InvalidParam, "请先登录平台管理员账号"))
	return 0, false
}

func currentAuthPrincipal(c *gin.Context) (AuthPrincipal, bool) {
	return apimiddleware.CurrentAuthPrincipal(c)
}

func defaultSessionMaxAgeSeconds(value int) int {
	if value > 0 {
		return value
	}
	return int(7 * 24 * time.Hour / time.Second)
}

package v1

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/lifei6671/papermind/server/internal/service/permission"
)

func TestGinSessionPrincipalCanBeReadFromBearerToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store, err := NewSessionStore(SessionStoreOptions{Provider: "memory", Secret: "test-session-secret-32-bytes-long"})
	if err != nil {
		t.Fatalf("NewSessionStore() error = %v", err)
	}
	router := gin.New()
	router.Use(bearerSessionCookieMiddleware(authSessionName), sessions.Sessions(authSessionName, store), authContextMiddleware())
	router.POST("/login", func(c *gin.Context) {
		token, err := saveAuthPrincipalSession(c, AuthPrincipal{
			SubjectType: permission.SubjectPlatformUser,
			UserID:      7,
			Role:        platformAdminRole,
		}, 3600)
		if err != nil {
			t.Fatalf("saveAuthPrincipalSession() error = %v", err)
		}
		c.String(http.StatusOK, token)
	})
	router.POST("/protected", func(c *gin.Context) {
		actorID, ok := requirePlatformActor(c)
		if !ok {
			return
		}
		c.JSON(http.StatusOK, gin.H{"actor_id": actorID})
	})

	loginRecorder := httptest.NewRecorder()
	router.ServeHTTP(loginRecorder, httptest.NewRequest(http.MethodPost, "/login", nil))
	if loginRecorder.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", loginRecorder.Code, loginRecorder.Body.String())
	}

	protectedRecorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/protected", nil)
	request.AddCookie(&http.Cookie{Name: authSessionName, Value: "stale-cookie"})
	request.Header.Set("Authorization", "Bearer "+loginRecorder.Body.String())
	router.ServeHTTP(protectedRecorder, request)
	if protectedRecorder.Code != http.StatusOK {
		t.Fatalf("protected status = %d, body = %s", protectedRecorder.Code, protectedRecorder.Body.String())
	}
}

func TestNewSessionStoreSupportsRedisProvider(t *testing.T) {
	store, err := NewSessionStore(SessionStoreOptions{
		Provider: "redis",
		Secret:   "test-session-secret-32-bytes-long",
		Redis: RedisSessionStoreOptions{
			Addr:      "127.0.0.1:6379",
			Username:  "",
			Password:  "",
			DB:        2,
			KeyPrefix: "papermind:test:session",
		},
	})
	if err != nil {
		if strings.Contains(err.Error(), "connectex") || strings.Contains(err.Error(), "connection refused") {
			t.Skipf("redis is not available for provider smoke test: %v", err)
		}
		t.Fatalf("NewSessionStore() error = %v", err)
	}
	if store == nil {
		t.Fatalf("NewSessionStore() returned nil store")
	}
}

func TestNewSessionStoreRejectsUnsupportedProvider(t *testing.T) {
	_, err := NewSessionStore(SessionStoreOptions{Provider: "unknown"})
	if !errors.Is(err, errAuthSessionProviderUnsupported) {
		t.Fatalf("NewSessionStore() error = %v, want errAuthSessionProviderUnsupported", err)
	}
}

func TestPermissionContextFromSessionRequiresAuthenticatedPrincipal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	_, err := (examHandler{}).permissionContextFromSession(c, 10, 0)
	if err == nil {
		t.Fatalf("expected missing authenticated principal to be rejected")
	}
}

func TestExamBusinessMiddlewareRejectsSpaceAdminSessionRole(t *testing.T) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(authPrincipalGinKey, AuthPrincipal{
		SubjectType: permission.SubjectTenantUser,
		UserID:      20,
		TenantID:    10,
		Role:        permission.RoleSpaceAdmin,
	})

	requireExamBusinessPrincipalMiddleware()(c)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("space_admin session role should be forbidden, got status = %d", recorder.Code)
	}
}

func TestExamBusinessMiddlewareAllowsTenantLevelExamRoles(t *testing.T) {
	for _, role := range []string{permission.RoleTenantAdmin, permission.RoleTeacher} {
		t.Run(role, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Set(authPrincipalGinKey, AuthPrincipal{
				SubjectType: permission.SubjectTenantUser,
				UserID:      20,
				TenantID:    10,
				Role:        role,
			})

			requireExamBusinessPrincipalMiddleware()(c)

			if c.IsAborted() {
				t.Fatalf("%s should pass exam business middleware", role)
			}
		})
	}
}

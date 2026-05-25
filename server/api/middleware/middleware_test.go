package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/papermind/server/bootstrap/migration"
)

func TestRequestIDAddsHeaderAndContextValue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID())
	router.GET("/ping", func(c *gin.Context) {
		if c.GetString(RequestIDKey) == "" {
			t.Fatalf("request id missing from context")
		}
		c.String(http.StatusOK, "ok")
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/ping", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if recorder.Header().Get(RequestIDHeader) == "" {
		t.Fatalf("request id missing from response header")
	}
}

func TestRequestIDKeepsIncomingHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID())
	router.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, c.GetString(RequestIDKey))
	})

	request := httptest.NewRequest(http.MethodGet, "/ping", nil)
	request.Header.Set(RequestIDHeader, "req-123")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Header().Get(RequestIDHeader) != "req-123" {
		t.Fatalf("response request id = %q", recorder.Header().Get(RequestIDHeader))
	}
	if recorder.Body.String() != "req-123" {
		t.Fatalf("body = %q", recorder.Body.String())
	}
}

func TestRecoveryReturnsUnifiedErrorResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID(), Recovery())
	router.GET("/panic", func(c *gin.Context) {
		panic("boom")
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/panic", nil))

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", recorder.Code)
	}
	if recorder.Header().Get(RequestIDHeader) == "" {
		t.Fatalf("request id missing from response header")
	}
	if recorder.Body.String() == "" {
		t.Fatalf("response body is empty")
	}
}

func TestRequestLoggerDoesNotChangeSuccessfulResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID(), RequestLogger())
	router.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusCreated, "created")
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/ping", nil))

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d", recorder.Code)
	}
	if recorder.Body.String() != "created" {
		t.Fatalf("body = %q", recorder.Body.String())
	}
}

func TestMigrationGuardReturnsMiddlePageForBrowserRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(MigrationGuard(staticMigrationStatus{state: migration.StateRunning}))
	router.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("Accept", "text/html")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", recorder.Code)
	}
	if recorder.Header().Get("Retry-After") != "5" {
		t.Fatalf("Retry-After = %q", recorder.Header().Get("Retry-After"))
	}
	if recorder.Body.String() == "" {
		t.Fatalf("migration page body is empty")
	}
}

func TestMigrationGuardReturnsJSONForAPIRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(MigrationGuard(staticMigrationStatus{state: migration.StateRunning}))
	router.GET("/api/v1/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/ping", nil)
	request.Header.Set("Accept", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", recorder.Code)
	}
	if recorder.Body.String() == "" {
		t.Fatalf("migration json body is empty")
	}
}

func TestMigrationGuardAllowsRequestsAfterMigration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(MigrationGuard(staticMigrationStatus{state: migration.StateDone}))
	router.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/ping", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if recorder.Body.String() != "ok" {
		t.Fatalf("body = %q", recorder.Body.String())
	}
}

type staticMigrationStatus struct {
	state migration.State
}

func (s staticMigrationStatus) Status() migration.Status {
	return migration.Status{State: s.state}
}

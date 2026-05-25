package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
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

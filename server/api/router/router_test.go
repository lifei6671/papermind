package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestNewAppliesConfiguredCORSOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const allowedOrigin = "https://app.example.com"

	engine := New(Options{CORSOrigins: []string{allowedOrigin}}, func(api *gin.RouterGroup, deps Dependencies) {
		api.GET("/ping", func(c *gin.Context) {
			c.Status(http.StatusNoContent)
		})
	})

	request := httptest.NewRequest(http.MethodGet, "/api/v1/ping", nil)
	request.Header.Set("Origin", allowedOrigin)
	response := httptest.NewRecorder()

	engine.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("response.Code = %d, want %d", response.Code, http.StatusNoContent)
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != allowedOrigin {
		t.Fatalf("Access-Control-Allow-Origin = %q, want %q", got, allowedOrigin)
	}
}

func TestNewHandlesConfiguredCORSPreflight(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const allowedOrigin = "https://app.example.com"

	engine := New(Options{CORSOrigins: []string{allowedOrigin}}, func(api *gin.RouterGroup, deps Dependencies) {
		api.POST("/papers", func(c *gin.Context) {
			c.Status(http.StatusNoContent)
		})
	})

	request := httptest.NewRequest(http.MethodOptions, "/api/v1/papers", nil)
	request.Header.Set("Origin", allowedOrigin)
	request.Header.Set("Access-Control-Request-Method", http.MethodPost)
	response := httptest.NewRecorder()

	engine.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("response.Code = %d, want %d", response.Code, http.StatusNoContent)
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != allowedOrigin {
		t.Fatalf("Access-Control-Allow-Origin = %q, want %q", got, allowedOrigin)
	}
	if got := response.Header().Get("Access-Control-Allow-Methods"); got == "" {
		t.Fatalf("Access-Control-Allow-Methods is empty")
	}
}

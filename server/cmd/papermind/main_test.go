package main

import (
	"net/http"
	"testing"

	"gorm.io/gorm"

	"github.com/lifei6671/papermind/server/library/config"
)

func TestBuildHTTPServerUsesConfiguredPort(t *testing.T) {
	server := buildHTTPServer(&config.Config{
		App: config.AppConfig{HTTPPort: 18080},
	}, http.NewServeMux())

	if server.Addr != ":18080" {
		t.Fatalf("server.Addr = %q, want :18080", server.Addr)
	}
	if server.Handler == nil {
		t.Fatalf("server.Handler is nil")
	}
}

func TestBuildHTTPServerUsesDefaultPort(t *testing.T) {
	server := buildHTTPServer(&config.Config{}, http.NewServeMux())

	if server.Addr != ":9080" {
		t.Fatalf("server.Addr = %q, want :9080", server.Addr)
	}
}

func TestRouterOptionsFromConfigPassesSecurityDefaults(t *testing.T) {
	options := routerOptionsFromConfig(&config.Config{
		Security: config.SecurityConfig{
			AllowRegisterDefault: true,
			PasswordMinLength:    12,
		},
		Auth: config.AuthConfig{Session: config.SessionConfig{
			Provider: "memory",
			Secret:   "test-secret",
		}},
	}, &gorm.DB{})

	if !options.AllowRegisterDefault {
		t.Fatal("AllowRegisterDefault was not passed from config")
	}
	if options.PasswordMinLength != 12 {
		t.Fatalf("PasswordMinLength = %d, want 12", options.PasswordMinLength)
	}
}

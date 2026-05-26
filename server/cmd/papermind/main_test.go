package main

import (
	"net/http"
	"testing"

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

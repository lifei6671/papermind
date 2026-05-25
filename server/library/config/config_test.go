package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadReadsYAMLConfig(t *testing.T) {
	configPath := writeConfig(t, `
app:
  name: papermind-test
  env: test
  http_port: 18080
  public_url: http://127.0.0.1:18080
database:
  driver: sqlite
  dsn: file:test.db?_foreign_keys=on
  max_open_conns: 3
  max_idle_conns: 2
auth:
  access_token_ttl: 7200
  refresh_token_ttl: 604800
  exam_token_buffer_minutes: 30
storage:
  temp_dir: /tmp/papermind/tmp
  import_dir: /tmp/papermind/imports
  export_dir: /tmp/papermind/exports
security:
  allow_register_default: true
  password_min_length: 10
  cors_origins:
    - http://localhost:5173
    - http://127.0.0.1:5173
`)

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.App.Name != "papermind-test" {
		t.Fatalf("App.Name = %q", cfg.App.Name)
	}
	if cfg.App.HTTPPort != 18080 {
		t.Fatalf("App.HTTPPort = %d", cfg.App.HTTPPort)
	}
	if cfg.Database.MaxOpenConns != 3 {
		t.Fatalf("Database.MaxOpenConns = %d", cfg.Database.MaxOpenConns)
	}
	if cfg.Auth.ExamTokenBufferMinutes != 30 {
		t.Fatalf("Auth.ExamTokenBufferMinutes = %d", cfg.Auth.ExamTokenBufferMinutes)
	}
	if !cfg.Security.AllowRegisterDefault {
		t.Fatalf("Security.AllowRegisterDefault = false")
	}
	if len(cfg.Security.CORSOrigins) != 2 {
		t.Fatalf("Security.CORSOrigins length = %d", len(cfg.Security.CORSOrigins))
	}
}

func TestLoadAppliesEnvironmentOverrides(t *testing.T) {
	configPath := writeConfig(t, `
app:
  name: papermind
  env: dev
  http_port: 8080
  public_url: http://localhost:8080
database:
  driver: sqlite
  dsn: file:dev.db
  max_open_conns: 1
  max_idle_conns: 1
auth:
  access_token_ttl: 7200
  refresh_token_ttl: 604800
  exam_token_buffer_minutes: 30
storage:
  temp_dir: /tmp/papermind/tmp
  import_dir: /tmp/papermind/imports
  export_dir: /tmp/papermind/exports
security:
  allow_register_default: false
  password_min_length: 8
  cors_origins:
    - http://localhost:5173
`)
	t.Setenv("PAPERMIND_DATABASE_DSN", "postgres://user:pass@127.0.0.1:5432/papermind")
	t.Setenv("PAPERMIND_AUTH_ACCESS_TOKEN_TTL", "3600")
	t.Setenv("PAPERMIND_SECURITY_CORS_ORIGINS", "http://localhost:3000, http://127.0.0.1:3000")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Database.DSN != "postgres://user:pass@127.0.0.1:5432/papermind" {
		t.Fatalf("Database.DSN = %q", cfg.Database.DSN)
	}
	if cfg.Auth.AccessTokenTTL != 3600 {
		t.Fatalf("Auth.AccessTokenTTL = %d", cfg.Auth.AccessTokenTTL)
	}
	if got := strings.Join(cfg.Security.CORSOrigins, ","); got != "http://localhost:3000,http://127.0.0.1:3000" {
		t.Fatalf("Security.CORSOrigins = %q", got)
	}
}

func TestValidateStorageDirsRequiresExistingWritableDirectories(t *testing.T) {
	root := t.TempDir()
	tempDir := filepath.Join(root, "tmp")
	importDir := filepath.Join(root, "imports")
	exportDir := filepath.Join(root, "exports")
	for _, dir := range []string{tempDir, importDir, exportDir} {
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatalf("Mkdir(%q) error = %v", dir, err)
		}
	}

	cfg := &Config{
		Storage: StorageConfig{
			TempDir:   tempDir,
			ImportDir: importDir,
			ExportDir: exportDir,
		},
	}
	if err := ValidateStorageDirs(cfg); err != nil {
		t.Fatalf("ValidateStorageDirs() error = %v", err)
	}

	cfg.Storage.ExportDir = filepath.Join(root, "missing")
	if err := ValidateStorageDirs(cfg); err == nil {
		t.Fatalf("ValidateStorageDirs() error = nil, want missing directory error")
	}
}

func TestExampleConfigLoads(t *testing.T) {
	cfg, err := Load("../../conf/app.example.yaml")
	if err != nil {
		t.Fatalf("Load(example) error = %v", err)
	}
	if cfg.App.Name == "" {
		t.Fatalf("App.Name is empty")
	}
	if cfg.Database.Driver == "" {
		t.Fatalf("Database.Driver is empty")
	}
}

func writeConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "app.yaml")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

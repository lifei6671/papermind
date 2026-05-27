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
  session:
    provider: memory
    secret: yaml-session-secret
    ttl: 604800
    key_prefix: papermind:session
    cleanup_interval: 300
    redis:
      addr: 127.0.0.1:6379
      username: default
      password: example-password
      db: 1
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
	if cfg.Auth.Session.Provider != "memory" ||
		cfg.Auth.Session.Secret != "yaml-session-secret" ||
		cfg.Auth.Session.Redis.DB != 1 {
		t.Fatalf("Auth.Session = %#v", cfg.Auth.Session)
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
  session:
    provider: memory
    secret: ""
    ttl: 604800
    key_prefix: papermind:session
    cleanup_interval: 300
    redis:
      addr: 127.0.0.1:6379
      db: 0
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
	t.Setenv("PAPERMIND_AUTH_SESSION_PROVIDER", "redis")
	t.Setenv("PAPERMIND_AUTH_SESSION_SECRET", "env-session-secret")
	t.Setenv("PAPERMIND_AUTH_SESSION_TTL", "86400")
	t.Setenv("PAPERMIND_AUTH_SESSION_KEY_PREFIX", "pm:test:session")
	t.Setenv("PAPERMIND_AUTH_SESSION_CLEANUP_INTERVAL", "60")
	t.Setenv("PAPERMIND_AUTH_SESSION_REDIS_ADDR", "127.0.0.1:6380")
	t.Setenv("PAPERMIND_AUTH_SESSION_REDIS_USERNAME", "session-user")
	t.Setenv("PAPERMIND_AUTH_SESSION_REDIS_PASSWORD", "session-pass")
	t.Setenv("PAPERMIND_AUTH_SESSION_REDIS_DB", "2")
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
	if cfg.Auth.Session.Provider != "redis" ||
		cfg.Auth.Session.Secret != "env-session-secret" ||
		cfg.Auth.Session.TTL != 86400 ||
		cfg.Auth.Session.KeyPrefix != "pm:test:session" ||
		cfg.Auth.Session.CleanupInterval != 60 ||
		cfg.Auth.Session.Redis.Addr != "127.0.0.1:6380" ||
		cfg.Auth.Session.Redis.Username != "session-user" ||
		cfg.Auth.Session.Redis.Password != "session-pass" ||
		cfg.Auth.Session.Redis.DB != 2 {
		t.Fatalf("Auth.Session = %#v", cfg.Auth.Session)
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
	} else if !strings.Contains(err.Error(), "storage.export_dir") {
		t.Fatalf("ValidateStorageDirs() error = %q, want storage.export_dir", err.Error())
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

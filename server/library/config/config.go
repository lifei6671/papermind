package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
)

type Config struct {
	App      AppConfig      `yaml:"app"`
	Database DatabaseConfig `yaml:"database"`
	Auth     AuthConfig     `yaml:"auth"`
	Storage  StorageConfig  `yaml:"storage"`
	Security SecurityConfig `yaml:"security"`
}

type AppConfig struct {
	Name      string `yaml:"name"`
	Env       string `yaml:"env"`
	HTTPPort  int    `yaml:"http_port"`
	PublicURL string `yaml:"public_url"`
}

type DatabaseConfig struct {
	Driver       string `yaml:"driver"`
	DSN          string `yaml:"dsn"`
	MaxOpenConns int    `yaml:"max_open_conns"`
	MaxIdleConns int    `yaml:"max_idle_conns"`
}

type AuthConfig struct {
	AccessTokenTTL         int           `yaml:"access_token_ttl"`
	RefreshTokenTTL        int           `yaml:"refresh_token_ttl"`
	ExamTokenBufferMinutes int           `yaml:"exam_token_buffer_minutes"`
	Session                SessionConfig `yaml:"session"`
}

type SessionConfig struct {
	Provider        string             `yaml:"provider"`
	Secret          string             `yaml:"secret"`
	TTL             int                `yaml:"ttl"`
	KeyPrefix       string             `yaml:"key_prefix"`
	CleanupInterval int                `yaml:"cleanup_interval"`
	Redis           RedisSessionConfig `yaml:"redis"`
}

type RedisSessionConfig struct {
	Addr     string `yaml:"addr"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

type StorageConfig struct {
	TempDir   string `yaml:"temp_dir"`
	ImportDir string `yaml:"import_dir"`
	ExportDir string `yaml:"export_dir"`
}

type SecurityConfig struct {
	AllowRegisterDefault bool     `yaml:"allow_register_default"`
	PasswordMinLength    int      `yaml:"password_min_length"`
	CORSOrigins          []string `yaml:"cors_origins"`
}

func Load(path string) (*Config, error) {
	if path == "" {
		return nil, fmt.Errorf("config path is empty")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}
	if err := applyEnvironmentOverrides(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func LoadAndValidate(path string) (*Config, error) {
	cfg, err := Load(path)
	if err != nil {
		return nil, err
	}
	if err := ValidateStorageDirs(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func ValidateStorageDirs(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	// 启动阶段必须确认运行期目录可写，避免导入、导出和临时文件在业务请求中才失败。
	dirs := map[string]string{
		"storage.temp_dir":   cfg.Storage.TempDir,
		"storage.import_dir": cfg.Storage.ImportDir,
		"storage.export_dir": cfg.Storage.ExportDir,
	}
	for name, dir := range dirs {
		if err := validateWritableDir(name, dir); err != nil {
			return err
		}
	}
	return nil
}

func applyEnvironmentOverrides(cfg *Config) error {
	overrideString("PAPERMIND_APP_NAME", &cfg.App.Name)
	overrideString("PAPERMIND_APP_ENV", &cfg.App.Env)
	if err := overrideInt("PAPERMIND_APP_HTTP_PORT", &cfg.App.HTTPPort); err != nil {
		return err
	}
	overrideString("PAPERMIND_APP_PUBLIC_URL", &cfg.App.PublicURL)

	overrideString("PAPERMIND_DATABASE_DRIVER", &cfg.Database.Driver)
	overrideString("PAPERMIND_DATABASE_DSN", &cfg.Database.DSN)
	if err := overrideInt("PAPERMIND_DATABASE_MAX_OPEN_CONNS", &cfg.Database.MaxOpenConns); err != nil {
		return err
	}
	if err := overrideInt("PAPERMIND_DATABASE_MAX_IDLE_CONNS", &cfg.Database.MaxIdleConns); err != nil {
		return err
	}

	if err := overrideInt("PAPERMIND_AUTH_ACCESS_TOKEN_TTL", &cfg.Auth.AccessTokenTTL); err != nil {
		return err
	}
	if err := overrideInt("PAPERMIND_AUTH_REFRESH_TOKEN_TTL", &cfg.Auth.RefreshTokenTTL); err != nil {
		return err
	}
	if err := overrideInt("PAPERMIND_AUTH_EXAM_TOKEN_BUFFER_MINUTES", &cfg.Auth.ExamTokenBufferMinutes); err != nil {
		return err
	}
	overrideString("PAPERMIND_AUTH_SESSION_PROVIDER", &cfg.Auth.Session.Provider)
	overrideString("PAPERMIND_AUTH_SESSION_SECRET", &cfg.Auth.Session.Secret)
	if err := overrideInt("PAPERMIND_AUTH_SESSION_TTL", &cfg.Auth.Session.TTL); err != nil {
		return err
	}
	overrideString("PAPERMIND_AUTH_SESSION_KEY_PREFIX", &cfg.Auth.Session.KeyPrefix)
	if err := overrideInt("PAPERMIND_AUTH_SESSION_CLEANUP_INTERVAL", &cfg.Auth.Session.CleanupInterval); err != nil {
		return err
	}
	overrideString("PAPERMIND_AUTH_SESSION_REDIS_ADDR", &cfg.Auth.Session.Redis.Addr)
	overrideString("PAPERMIND_AUTH_SESSION_REDIS_USERNAME", &cfg.Auth.Session.Redis.Username)
	overrideString("PAPERMIND_AUTH_SESSION_REDIS_PASSWORD", &cfg.Auth.Session.Redis.Password)
	if err := overrideInt("PAPERMIND_AUTH_SESSION_REDIS_DB", &cfg.Auth.Session.Redis.DB); err != nil {
		return err
	}

	overrideString("PAPERMIND_STORAGE_TEMP_DIR", &cfg.Storage.TempDir)
	overrideString("PAPERMIND_STORAGE_IMPORT_DIR", &cfg.Storage.ImportDir)
	overrideString("PAPERMIND_STORAGE_EXPORT_DIR", &cfg.Storage.ExportDir)

	if err := overrideBool("PAPERMIND_SECURITY_ALLOW_REGISTER_DEFAULT", &cfg.Security.AllowRegisterDefault); err != nil {
		return err
	}
	if err := overrideInt("PAPERMIND_SECURITY_PASSWORD_MIN_LENGTH", &cfg.Security.PasswordMinLength); err != nil {
		return err
	}
	overrideStringSlice("PAPERMIND_SECURITY_CORS_ORIGINS", &cfg.Security.CORSOrigins)
	return nil
}

func overrideString(key string, target *string) {
	if value, ok := os.LookupEnv(key); ok {
		*target = value
	}
}

func overrideStringSlice(key string, target *[]string) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return
	}

	// 多源 CORS 配置用逗号分隔，裁剪空白后丢弃空项，避免环境变量格式差异污染配置。
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			items = append(items, part)
		}
	}
	*target = items
}

func overrideInt(key string, target *int) error {
	value, ok := os.LookupEnv(key)
	if !ok {
		return nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("parse env %s as int: %w", key, err)
	}
	*target = parsed
	return nil
}

func overrideBool(key string, target *bool) error {
	value, ok := os.LookupEnv(key)
	if !ok {
		return nil
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fmt.Errorf("parse env %s as bool: %w", key, err)
	}
	*target = parsed
	return nil
}

func validateWritableDir(name string, dir string) error {
	if dir == "" {
		return fmt.Errorf("%s is empty", name)
	}

	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("%s is not accessible: %w", name, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory: %s", name, dir)
	}

	file, err := os.CreateTemp(dir, ".papermind-write-check-*")
	if err != nil {
		return fmt.Errorf("%s is not writable: %w", name, err)
	}
	fileName := file.Name()
	if err := file.Close(); err != nil {
		return fmt.Errorf("%s write check close failed: %w", name, err)
	}
	if err := os.Remove(fileName); err != nil {
		return fmt.Errorf("%s write check cleanup failed: %w", name, err)
	}
	return nil
}

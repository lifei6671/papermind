package db

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"

	"github.com/lifei6671/papermind/server/library/config"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var (
	ErrNotInitialized     = errors.New("database is not initialized")
	ErrAlreadyInitialized = errors.New("database is already initialized")
	singletonMu           sync.RWMutex
	singleton             *gorm.DB
)

type Options struct {
	Logger       *slog.Logger
	MigrationDir string
}

func Init(cfg config.DatabaseConfig) (*gorm.DB, error) {
	return InitWithOptions(cfg, Options{})
}

func InitWithOptions(cfg config.DatabaseConfig, opts Options) (*gorm.DB, error) {
	singletonMu.Lock()
	defer singletonMu.Unlock()
	if singleton != nil {
		return nil, ErrAlreadyInitialized
	}

	// 数据库连接是进程级基础设施，只在服务启动时初始化一次，运行期 DAO 统一复用同一个连接池。
	gormDB, err := openWithOptions(cfg, opts)
	if err != nil {
		return nil, err
	}
	singleton = gormDB
	return singleton, nil
}

func Get() (*gorm.DB, error) {
	singletonMu.RLock()
	defer singletonMu.RUnlock()
	if singleton == nil {
		return nil, ErrNotInitialized
	}
	return singleton, nil
}

func Close() error {
	singletonMu.Lock()
	defer singletonMu.Unlock()
	if singleton == nil {
		return nil
	}

	sqlDB, err := singleton.DB()
	if err != nil {
		return fmt.Errorf("get sql db: %w", err)
	}
	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("close database: %w", err)
	}
	singleton = nil
	return nil
}

func Open(cfg config.DatabaseConfig) (*gorm.DB, error) {
	return OpenWithOptions(cfg, Options{})
}

func OpenWithOptions(cfg config.DatabaseConfig, opts Options) (*gorm.DB, error) {
	return openWithOptions(cfg, opts)
}

func openWithOptions(cfg config.DatabaseConfig, opts Options) (*gorm.DB, error) {
	driver := strings.ToLower(strings.TrimSpace(cfg.Driver))
	if driver == "" {
		return nil, fmt.Errorf("database driver is empty")
	}
	if cfg.DSN == "" {
		return nil, fmt.Errorf("database dsn is empty")
	}

	if driver == "sqlite" {
		if err := validateSQLiteDSN(cfg.DSN); err != nil {
			return nil, err
		}
	}

	dialector, err := dialectorFor(driver, cfg.DSN)
	if err != nil {
		return nil, err
	}

	gormDB, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, fmt.Errorf("get sql db: %w", err)
	}

	maxOpenConns, maxIdleConns := poolSettings(driver, cfg)
	if maxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(maxOpenConns)
	}
	if maxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(maxIdleConns)
	}

	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.Info(
		"database initialized",
		"driver", driver,
		"max_open_conns", maxOpenConns,
		"max_idle_conns", maxIdleConns,
		"migration_dir", opts.MigrationDir,
	)

	return gormDB, nil
}

func dialectorFor(driver string, dsn string) (gorm.Dialector, error) {
	switch driver {
	case "postgres", "postgresql":
		return postgres.Open(dsn), nil
	case "mysql":
		return mysql.Open(dsn), nil
	case "sqlite":
		return sqlite.Open(dsn), nil
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", driver)
	}
}

func poolSettings(driver string, cfg config.DatabaseConfig) (int, int) {
	maxOpenConns := cfg.MaxOpenConns
	maxIdleConns := cfg.MaxIdleConns
	if driver == "sqlite" && maxOpenConns <= 0 {
		maxOpenConns = 1
	}
	return maxOpenConns, maxIdleConns
}

func validateSQLiteDSN(dsn string) error {
	queryStart := strings.Index(dsn, "?")
	if queryStart < 0 {
		return fmt.Errorf("sqlite dsn must include _foreign_keys=on, _journal_mode=WAL and _busy_timeout")
	}

	values, err := url.ParseQuery(dsn[queryStart+1:])
	if err != nil {
		return fmt.Errorf("parse sqlite dsn query: %w", err)
	}
	if !hasQueryValue(values, "_foreign_keys", "on") {
		return fmt.Errorf("sqlite dsn must include _foreign_keys=on")
	}
	if !hasQueryValue(values, "_journal_mode", "WAL") {
		return fmt.Errorf("sqlite dsn must include _journal_mode=WAL")
	}
	if values.Get("_busy_timeout") == "" {
		return fmt.Errorf("sqlite dsn must include _busy_timeout")
	}
	return nil
}

func hasQueryValue(values url.Values, key string, expected string) bool {
	return strings.EqualFold(values.Get(key), expected)
}

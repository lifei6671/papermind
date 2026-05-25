package db

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/lifei6671/papermind/server/library/config"
)

func TestOpenRejectsUnsupportedDriver(t *testing.T) {
	_, err := Open(config.DatabaseConfig{Driver: "oracle"})
	if err == nil {
		t.Fatalf("Open() error = nil, want unsupported driver error")
	}
}

func TestOpenSQLiteRequiresRequiredDSNOptions(t *testing.T) {
	_, err := Open(config.DatabaseConfig{
		Driver: "sqlite",
		DSN:    filepath.Join(t.TempDir(), "papermind.db"),
	})
	if err == nil {
		t.Fatalf("Open() error = nil, want sqlite dsn option error")
	}
}

func TestOpenSQLiteAppliesDefaultPool(t *testing.T) {
	gormDB, err := Open(config.DatabaseConfig{
		Driver: "sqlite",
		DSN:    sqliteDSN(t),
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		t.Fatalf("DB() error = %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	if sqlDB.Stats().MaxOpenConnections != 1 {
		t.Fatalf("MaxOpenConnections = %d", sqlDB.Stats().MaxOpenConnections)
	}
}

func TestOpenSQLiteAllowsConfiguredSmallPool(t *testing.T) {
	gormDB, err := Open(config.DatabaseConfig{
		Driver:       "sqlite",
		DSN:          sqliteDSN(t),
		MaxOpenConns: 7,
		MaxIdleConns: 3,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	sqlDB, err := gormDB.DB()
	if err != nil {
		t.Fatalf("DB() error = %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	if sqlDB.Stats().MaxOpenConnections != 7 {
		t.Fatalf("MaxOpenConnections = %d", sqlDB.Stats().MaxOpenConnections)
	}
}

func TestDialectorSupportsPostgresMySQLAndSQLite(t *testing.T) {
	tests := []struct {
		name   string
		driver string
		dsn    string
	}{
		{
			name:   "postgres",
			driver: "postgres",
			dsn:    "host=127.0.0.1 user=papermind password=papermind dbname=papermind port=5432 sslmode=disable",
		},
		{
			name:   "mysql",
			driver: "mysql",
			dsn:    "papermind:papermind@tcp(127.0.0.1:3306)/papermind?parseTime=true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dialector, err := dialectorFor(tt.driver, tt.dsn)
			if err != nil {
				t.Fatalf("dialectorFor() error = %v", err)
			}
			if dialector == nil {
				t.Fatalf("dialectorFor() = nil")
			}
		})
	}
}

func TestPoolSettingsUseConfiguredValuesForPostgresAndMySQL(t *testing.T) {
	for _, driver := range []string{"postgres", "mysql"} {
		t.Run(driver, func(t *testing.T) {
			maxOpen, maxIdle := poolSettings(driver, config.DatabaseConfig{
				MaxOpenConns: 12,
				MaxIdleConns: 4,
			})

			if maxOpen != 12 {
				t.Fatalf("maxOpen = %d", maxOpen)
			}
			if maxIdle != 4 {
				t.Fatalf("maxIdle = %d", maxIdle)
			}
		})
	}
}

func TestGetReturnsErrorBeforeInit(t *testing.T) {
	resetForTest(t)

	_, err := Get()
	if !errors.Is(err, ErrNotInitialized) {
		t.Fatalf("Get() error = %v, want ErrNotInitialized", err)
	}
}

func TestInitStoresSingletonInstance(t *testing.T) {
	resetForTest(t)

	initialized, err := Init(config.DatabaseConfig{
		Driver: "sqlite",
		DSN:    sqliteDSN(t),
	})
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	t.Cleanup(func() {
		_ = Close()
	})

	got, err := Get()
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got != initialized {
		t.Fatalf("Get() returned different instance")
	}
}

func TestInitRejectsRepeatedInitialization(t *testing.T) {
	resetForTest(t)

	if _, err := Init(config.DatabaseConfig{Driver: "sqlite", DSN: sqliteDSN(t)}); err != nil {
		t.Fatalf("first Init() error = %v", err)
	}
	t.Cleanup(func() {
		_ = Close()
	})
	if _, err := Init(config.DatabaseConfig{Driver: "sqlite", DSN: sqliteDSN(t)}); !errors.Is(err, ErrAlreadyInitialized) {
		t.Fatalf("second Init() error = %v, want ErrAlreadyInitialized", err)
	}
}

func TestCloseClearsSingleton(t *testing.T) {
	resetForTest(t)

	if _, err := Init(config.DatabaseConfig{Driver: "sqlite", DSN: sqliteDSN(t)}); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	if err := Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	_, err := Get()
	if !errors.Is(err, ErrNotInitialized) {
		t.Fatalf("Get() error = %v, want ErrNotInitialized", err)
	}
}

func resetForTest(t *testing.T) {
	t.Helper()

	singletonMu.Lock()
	defer singletonMu.Unlock()
	if singleton != nil {
		sqlDB, err := singleton.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	}
	singleton = nil
	t.Cleanup(func() {
		singletonMu.Lock()
		defer singletonMu.Unlock()
		if singleton != nil {
			sqlDB, err := singleton.DB()
			if err == nil {
				_ = sqlDB.Close()
			}
		}
		singleton = nil
	})
}

func sqliteDSN(t *testing.T) string {
	t.Helper()

	path := filepath.ToSlash(filepath.Join(t.TempDir(), "papermind.db"))
	return "file:" + path + "?_foreign_keys=on&_journal_mode=WAL&_busy_timeout=5000"
}

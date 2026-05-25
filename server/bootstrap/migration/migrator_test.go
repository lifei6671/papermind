package migration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lifei6671/papermind/server/internal/dao/db"
	"github.com/lifei6671/papermind/server/library/config"
	"gorm.io/gorm"
)

func TestRunExecutesSQLFilesByVersion(t *testing.T) {
	gormDB := openSQLiteForMigrationTest(t)
	dir := t.TempDir()
	writeMigration(t, dir, "002_second.sql", "INSERT INTO migration_order (id, name) VALUES (2, 'second');")
	writeMigration(t, dir, "001_first.sql", "CREATE TABLE migration_order (id INTEGER PRIMARY KEY, name TEXT NOT NULL); INSERT INTO migration_order (id, name) VALUES (1, 'first');")

	if err := Run(gormDB, dir); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var names []string
	if err := gormDB.Raw("SELECT name FROM migration_order ORDER BY id").Scan(&names).Error; err != nil {
		t.Fatalf("query migration_order error = %v", err)
	}
	if got := names[0] + "," + names[1]; got != "first,second" {
		t.Fatalf("migration order = %q", got)
	}
}

func TestRunSkipsAlreadyAppliedVersions(t *testing.T) {
	gormDB := openSQLiteForMigrationTest(t)
	dir := t.TempDir()
	writeMigration(t, dir, "001_create.sql", "CREATE TABLE once_table (id INTEGER PRIMARY KEY); INSERT INTO once_table (id) VALUES (1);")

	if err := Run(gormDB, dir); err != nil {
		t.Fatalf("first Run() error = %v", err)
	}
	if err := Run(gormDB, dir); err != nil {
		t.Fatalf("second Run() error = %v", err)
	}

	var count int64
	if err := gormDB.Raw("SELECT COUNT(*) FROM once_table").Scan(&count).Error; err != nil {
		t.Fatalf("query once_table count error = %v", err)
	}
	if count != 1 {
		t.Fatalf("count = %d", count)
	}
}

func TestRunStopsWhenMigrationFails(t *testing.T) {
	gormDB := openSQLiteForMigrationTest(t)
	dir := t.TempDir()
	writeMigration(t, dir, "001_good.sql", "CREATE TABLE failed_stop (id INTEGER PRIMARY KEY);")
	writeMigration(t, dir, "002_bad.sql", "INSERT INTO missing_table (id) VALUES (1);")
	writeMigration(t, dir, "003_never.sql", "CREATE TABLE should_not_exist (id INTEGER PRIMARY KEY);")

	if err := Run(gormDB, dir); err == nil {
		t.Fatalf("Run() error = nil, want failed migration error")
	}

	if gormDB.Migrator().HasTable("should_not_exist") {
		t.Fatalf("migration after failed file was executed")
	}
}

func TestDirForDriverSelectsDatabaseMigrationDirectory(t *testing.T) {
	tests := []struct {
		driver string
		want   string
	}{
		{driver: "sqlite", want: filepath.Join("data", "migrations", "sqlite")},
		{driver: "mysql", want: filepath.Join("data", "migrations", "mysql")},
		{driver: "postgres", want: filepath.Join("data", "migrations", "postgres")},
		{driver: "postgresql", want: filepath.Join("data", "migrations", "postgres")},
	}

	for _, tt := range tests {
		t.Run(tt.driver, func(t *testing.T) {
			got, err := DirForDriver("data/migrations", tt.driver)
			if err != nil {
				t.Fatalf("DirForDriver() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("DirForDriver() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDirForDriverRejectsUnsupportedDriver(t *testing.T) {
	if _, err := DirForDriver("data/migrations", "oracle"); err == nil {
		t.Fatalf("DirForDriver() error = nil, want unsupported driver error")
	}
}

func openSQLiteForMigrationTest(t *testing.T) *gorm.DB {
	t.Helper()

	gormDB, err := db.Open(config.DatabaseConfig{
		Driver: "sqlite",
		DSN:    "file:" + filepath.ToSlash(filepath.Join(t.TempDir(), "migration.db")) + "?_foreign_keys=on&_journal_mode=WAL&_busy_timeout=5000",
	})
	if err != nil {
		t.Fatalf("open sqlite error = %v", err)
	}
	sqlDB, err := gormDB.DB()
	if err != nil {
		t.Fatalf("DB() error = %v", err)
	}
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})
	return gormDB
}

func writeMigration(t *testing.T, dir string, name string, content string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatalf("write migration %s error = %v", name, err)
	}
}

package adminseed

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/lifei6671/papermind/server/bootstrap/migration"
	dbmodel "github.com/lifei6671/papermind/server/internal/dao/db"
	"github.com/lifei6671/papermind/server/library/config"
	"github.com/lifei6671/papermind/server/library/crypto"
	"gorm.io/gorm"
)

func TestRunCreatesDefaultPlatformAdminAfterMigration(t *testing.T) {
	gormDB, closeDB := openSQLiteForAdminSeedTest(t)
	defer closeDB()

	if err := migration.Run(gormDB, filepath.Join("..", "..", "data", "migrations", "sqlite")); err != nil {
		t.Fatalf("Run(sqlite migrations) error = %v", err)
	}
	if err := Run(context.Background(), gormDB); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	var admin dbmodel.PlatformUserDO
	if err := gormDB.Where("username = ?", "admin").First(&admin).Error; err != nil {
		t.Fatalf("query default admin error = %v", err)
	}
	if admin.Status != "enabled" {
		t.Fatalf("default admin status = %q, want enabled", admin.Status)
	}
	if admin.Email != "admin@iminho.me" {
		t.Fatalf("default admin email = %q, want admin@iminho.me", admin.Email)
	}
	if admin.PasswordHash == "admin123" || !crypto.VerifyPassword(admin.PasswordHash, "admin123") {
		t.Fatalf("default admin password hash does not verify admin123")
	}

	if err := Run(context.Background(), gormDB); err != nil {
		t.Fatalf("second Run() error = %v", err)
	}
	var count int64
	if err := gormDB.Model(&dbmodel.PlatformUserDO{}).Where("username = ?", "admin").Count(&count).Error; err != nil {
		t.Fatalf("count default admin error = %v", err)
	}
	if count != 1 {
		t.Fatalf("default admin count = %d, want 1", count)
	}
}

func openSQLiteForAdminSeedTest(t *testing.T) (*gorm.DB, func()) {
	t.Helper()

	gormDB, err := dbmodel.Open(config.DatabaseConfig{
		Driver: "sqlite",
		DSN:    "file:" + filepath.ToSlash(filepath.Join(t.TempDir(), "admin-seed.db")) + "?_foreign_keys=on&_journal_mode=WAL&_busy_timeout=5000",
	})
	if err != nil {
		t.Fatalf("open sqlite error = %v", err)
	}
	sqlDB, err := gormDB.DB()
	if err != nil {
		t.Fatalf("DB() error = %v", err)
	}
	return gormDB, func() {
		_ = sqlDB.Close()
	}
}

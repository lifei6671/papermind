package migration

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

const schemaMigrationsTable = "schema_migrations"

type fileMigration struct {
	Version int64
	Name    string
	Path    string
}

func Run(gormDB *gorm.DB, dir string) error {
	if gormDB == nil {
		return fmt.Errorf("database is nil")
	}
	if dir == "" {
		return fmt.Errorf("migration dir is empty")
	}

	// 迁移版本表先于业务迁移创建，用于保证迁移脚本可以重复执行且不会重复应用。
	if err := ensureSchemaMigrationsTable(gormDB); err != nil {
		return err
	}

	migrations, err := loadMigrationFiles(dir)
	if err != nil {
		return err
	}

	for _, migration := range migrations {
		applied, err := isApplied(gormDB, migration.Version)
		if err != nil {
			return err
		}
		if applied {
			slog.Info("migration skipped", "version", migration.Version, "name", migration.Name)
			continue
		}

		// 单个迁移文件失败时立即返回错误，启动流程据此停止，避免服务运行在半迁移状态。
		if err := applyMigration(gormDB, migration); err != nil {
			return err
		}
	}
	return nil
}

func RunForDriver(gormDB *gorm.DB, migrationsRoot string, driver string) error {
	dir, err := DirForDriver(migrationsRoot, driver)
	if err != nil {
		return err
	}
	return Run(gormDB, dir)
}

func DirForDriver(migrationsRoot string, driver string) (string, error) {
	if migrationsRoot == "" {
		return "", fmt.Errorf("migration root is empty")
	}

	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "postgres", "postgresql":
		return filepath.Join(migrationsRoot, "postgres"), nil
	case "mysql":
		return filepath.Join(migrationsRoot, "mysql"), nil
	case "sqlite":
		return filepath.Join(migrationsRoot, "sqlite"), nil
	default:
		return "", fmt.Errorf("unsupported migration driver: %s", driver)
	}
}

func ensureSchemaMigrationsTable(gormDB *gorm.DB) error {
	sql := `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version BIGINT PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    applied_at BIGINT NOT NULL
)`
	if err := gormDB.Exec(sql).Error; err != nil {
		return fmt.Errorf("ensure schema migrations table: %w", err)
	}
	return nil
}

func loadMigrationFiles(dir string) ([]fileMigration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read migration dir: %w", err)
	}

	migrations := make([]fileMigration, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		version, err := parseVersion(entry.Name())
		if err != nil {
			return nil, err
		}
		migrations = append(migrations, fileMigration{
			Version: version,
			Name:    entry.Name(),
			Path:    filepath.Join(dir, entry.Name()),
		})
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})
	return migrations, nil
}

func parseVersion(name string) (int64, error) {
	versionPart := name
	if index := strings.Index(name, "_"); index >= 0 {
		versionPart = name[:index]
	} else if index := strings.Index(name, "."); index >= 0 {
		versionPart = name[:index]
	}

	version, err := strconv.ParseInt(versionPart, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse migration version from %s: %w", name, err)
	}
	return version, nil
}

func isApplied(gormDB *gorm.DB, version int64) (bool, error) {
	var count int64
	if err := gormDB.Raw("SELECT COUNT(*) FROM "+schemaMigrationsTable+" WHERE version = ?", version).Scan(&count).Error; err != nil {
		return false, fmt.Errorf("query migration version %d: %w", version, err)
	}
	return count > 0, nil
}

func applyMigration(gormDB *gorm.DB, migration fileMigration) error {
	content, err := os.ReadFile(migration.Path)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", migration.Name, err)
	}

	return gormDB.Transaction(func(tx *gorm.DB) error {
		if err := execStatements(tx, string(content)); err != nil {
			return fmt.Errorf("apply migration %s: %w", migration.Name, err)
		}
		if err := tx.Exec(
			"INSERT INTO "+schemaMigrationsTable+" (version, name, applied_at) VALUES (?, ?, ?)",
			migration.Version,
			migration.Name,
			time.Now().UnixMilli(),
		).Error; err != nil {
			return fmt.Errorf("record migration %s: %w", migration.Name, err)
		}
		slog.Info("migration applied", "version", migration.Version, "name", migration.Name)
		return nil
	})
}

func execStatements(gormDB *gorm.DB, content string) error {
	// 迁移文件按分号拆分执行，首版迁移 SQL 禁止在字符串字面量中嵌入未转义分号。
	for _, statement := range strings.Split(content, ";") {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}
		if err := gormDB.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}

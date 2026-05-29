package migration

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/lifei6671/papermind/server/internal/dao/db"
	"github.com/lifei6671/papermind/server/library/config"
	"gorm.io/gorm"
)

func TestSQLiteTenantSpaceMigrationCreatesTablesAndIndexes(t *testing.T) {
	gormDB, closeDB := openSQLiteForActualMigrationTest(t)
	defer closeDB()

	migrationDir := filepath.Join("..", "..", "data", "migrations", "sqlite")
	if err := Run(gormDB, migrationDir); err != nil {
		t.Fatalf("Run(sqlite migrations) error = %v", err)
	}

	for _, table := range []string{"tenants", "spaces", "space_members", "space_configs", "users", "tenant_user_memberships"} {
		if !gormDB.Migrator().HasTable(table) {
			t.Fatalf("table %s missing", table)
		}
	}
	assertSQLiteColumnExists(t, gormDB, "tenants", "logo_url")
	assertSQLiteColumnExists(t, gormDB, "tenants", "description")
	assertSQLiteColumnExists(t, gormDB, "tenants", "created_by_type")
	assertSQLiteColumnExists(t, gormDB, "tenants", "updated_by_type")
	assertSQLiteColumnExists(t, gormDB, "spaces", "logo_url")
	assertSQLiteColumnExists(t, gormDB, "spaces", "description")
	assertSQLiteColumnMissing(t, gormDB, "users", "tenant_id")
	assertSQLiteColumnExists(t, gormDB, "tenant_user_memberships", "status")
	assertSQLiteColumnExists(t, gormDB, "tenant_user_memberships", "created_by_type")
	assertSQLiteColumnExists(t, gormDB, "tenant_user_memberships", "updated_by_type")
	assertSQLiteColumnExists(t, gormDB, "question_tags", "created_by_type")
	assertSQLiteColumnExists(t, gormDB, "exam_events", "created_by_type")
	assertSQLiteUniqueIndexContains(t, gormDB, "tenants", []string{"tenant_code", "deleted_at"})
	assertSQLiteUniqueIndexContains(t, gormDB, "space_members", []string{"tenant_id", "space_id", "user_id", "deleted_at"})
	assertSQLiteUniqueIndexContains(t, gormDB, "space_configs", []string{"tenant_id", "space_id", "config_key"})
	assertSQLiteUniqueIndexContains(t, gormDB, "users", []string{"username", "deleted_at"})
	assertSQLiteUniqueIndexContains(t, gormDB, "tenant_user_memberships", []string{"tenant_id", "user_id"})
}

func TestSQLiteUsersAllowMultipleEmptyOptionalContacts(t *testing.T) {
	gormDB, closeDB := openSQLiteForActualMigrationTest(t)
	defer closeDB()

	migrationDir := filepath.Join("..", "..", "data", "migrations", "sqlite")
	if err := Run(gormDB, migrationDir); err != nil {
		t.Fatalf("Run(sqlite migrations) error = %v", err)
	}

	for _, id := range []int{1, 2} {
		if err := gormDB.Exec(`
			INSERT INTO users (
				id, username, real_name, phone, email, password_hash, status,
				created_at, updated_at, ext_json
			) VALUES (?, ?, ?, '', '', 'hash', 'enabled', 1000, 1000, '{}')
		`, id, "user"+strconv.Itoa(id), "用户"+strconv.Itoa(id)).Error; err != nil {
			t.Fatalf("insert user %d with empty contacts: %v", id, err)
		}
	}
}

func openSQLiteForActualMigrationTest(t *testing.T) (*gorm.DB, func()) {
	t.Helper()

	gormDB, err := db.Open(config.DatabaseConfig{
		Driver: "sqlite",
		DSN:    "file:" + filepath.ToSlash(filepath.Join(t.TempDir(), "actual-migration.db")) + "?_foreign_keys=on&_journal_mode=WAL&_busy_timeout=5000",
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

func assertSQLiteColumnExists(t *testing.T, gormDB *gorm.DB, table string, column string) {
	t.Helper()

	if !sqliteColumnExists(t, gormDB, table, column) {
		t.Fatalf("column %s.%s missing", table, column)
	}
}

func assertSQLiteColumnMissing(t *testing.T, gormDB *gorm.DB, table string, column string) {
	t.Helper()

	if sqliteColumnExists(t, gormDB, table, column) {
		t.Fatalf("column %s.%s should not exist", table, column)
	}
}

func sqliteColumnExists(t *testing.T, gormDB *gorm.DB, table string, column string) bool {
	t.Helper()

	rows, err := gormDB.Raw("PRAGMA table_info(" + table + ")").Rows()
	if err != nil {
		t.Fatalf("PRAGMA table_info(%s) error = %v", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			t.Fatalf("scan table_info(%s) error = %v", table, err)
		}
		if name == column {
			return true
		}
	}
	return false
}

func assertSQLiteUniqueIndexContains(t *testing.T, gormDB *gorm.DB, table string, columns []string) {
	t.Helper()

	rows, err := gormDB.Raw("PRAGMA index_list(" + table + ")").Rows()
	if err != nil {
		t.Fatalf("PRAGMA index_list(%s) error = %v", table, err)
	}
	indexNames := make([]string, 0)
	for rows.Next() {
		var seq int
		var name string
		var unique int
		var origin string
		var partial int
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			t.Fatalf("scan index_list(%s) error = %v", table, err)
		}
		if unique != 1 {
			continue
		}
		indexNames = append(indexNames, name)
	}
	if err := rows.Close(); err != nil {
		t.Fatalf("close index_list(%s) rows error = %v", table, err)
	}

	// SQLite 单连接模式下不能在 index_list rows 未关闭时继续执行 index_info，否则会等待同一个连接释放。
	for _, name := range indexNames {
		if sqliteIndexColumnsMatch(t, gormDB, name, columns) {
			return
		}
	}
	t.Fatalf("unique index on %s(%s) missing", table, strings.Join(columns, ","))
}

func sqliteIndexColumnsMatch(t *testing.T, gormDB *gorm.DB, indexName string, columns []string) bool {
	t.Helper()

	rows, err := gormDB.Raw("PRAGMA index_info(" + indexName + ")").Rows()
	if err != nil {
		t.Fatalf("PRAGMA index_info(%s) error = %v", indexName, err)
	}
	defer rows.Close()
	got := make([]string, 0, len(columns))
	for rows.Next() {
		var seqno, cid int
		var name string
		if err := rows.Scan(&seqno, &cid, &name); err != nil {
			t.Fatalf("scan index_info(%s) error = %v", indexName, err)
		}
		got = append(got, name)
	}
	return strings.Join(got, ",") == strings.Join(columns, ",")
}

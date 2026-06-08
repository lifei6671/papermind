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

func TestSQLiteExamManagementDetailMigrationCreatesOperationLogTableAndIndexes(t *testing.T) {
	gormDB, closeDB := openSQLiteForActualMigrationTest(t)
	defer closeDB()

	migrationDir := filepath.Join("..", "..", "data", "migrations", "sqlite")
	if err := Run(gormDB, migrationDir); err != nil {
		t.Fatalf("Run(sqlite migrations) error = %v", err)
	}

	if !gormDB.Migrator().HasTable("exam_operation_logs") {
		t.Fatalf("table exam_operation_logs missing")
	}
	for _, column := range []string{
		"tenant_id",
		"exam_id",
		"operation_type",
		"operation_title",
		"operation_detail",
		"actor_id",
		"actor_type",
		"actor_role",
		"space_id",
		"created_at",
		"created_by",
		"created_by_type",
		"ext_json",
	} {
		assertSQLiteColumnExists(t, gormDB, "exam_operation_logs", column)
	}
	assertSQLiteIndexColumns(t, gormDB, "idx_exam_attempts_exam_status_submitted", []string{"tenant_id", "exam_id", "status", "submitted_at"})
	assertSQLiteIndexColumns(t, gormDB, "idx_exam_attempts_exam_user", []string{"tenant_id", "exam_id", "user_id"})
	assertSQLiteIndexColumns(t, gormDB, "idx_exam_attempts_exam_id", []string{"tenant_id", "exam_id", "id"})
	assertSQLiteIndexColumns(t, gormDB, "idx_exam_attempts_exam_score_rank", []string{"tenant_id", "exam_id", "total_score", "submitted_at", "id"})
	assertSQLiteIndexColumns(t, gormDB, "idx_exam_answers_attempt_grading", []string{"tenant_id", "attempt_id", "grading_status"})
	assertSQLiteIndexColumns(t, gormDB, "idx_exam_operation_logs_exam_time", []string{"tenant_id", "exam_id", "created_at", "id"})
}

func TestSQLiteExamTargetScopeSpacesMigrationBackfillsHistoricalUserTargets(t *testing.T) {
	gormDB, closeDB := openSQLiteForActualMigrationTest(t)
	defer closeDB()

	migrationDir := filepath.Join("..", "..", "data", "migrations", "sqlite")
	migrations, err := loadMigrationFiles(migrationDir)
	if err != nil {
		t.Fatalf("loadMigrationFiles() error = %v", err)
	}
	if err := ensureSchemaMigrationsTable(gormDB); err != nil {
		t.Fatalf("ensure schema migrations: %v", err)
	}
	for _, migration := range migrations {
		if migration.Version > 2 {
			break
		}
		if err := applyMigration(gormDB, migration); err != nil {
			t.Fatalf("apply migration %d: %v", migration.Version, err)
		}
	}

	if err := gormDB.Exec(`
		INSERT INTO papers (
			id, tenant_id, name, description, total_score, build_mode, status,
			created_at, updated_at, ext_json
		) VALUES (100, 10, '历史试卷', '', 100, 'manual', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed historical paper: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO exams (
			id, tenant_id, paper_id, name, start_time, end_time, duration_minutes,
			max_attempts, result_strategy, publish_mode, invite_code, status,
			created_at, updated_at, ext_json
		) VALUES (900, 10, 100, '历史考试', 1000, 2000, 60, 1, 'latest', 'manual_publish', 'HISTORY', 'published', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed historical exam: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO exam_targets (
			id, tenant_id, exam_id, target_type, target_id, created_at, created_by,
			created_by_type, ext_json
		) VALUES
			(1, 10, 900, 'user', 21, 1000, 0, 'system', '{"space_ids":[301,302,301]}'),
			(2, 10, 900, 'user', 22, 1000, 0, 'system', '{}')
	`).Error; err != nil {
		t.Fatalf("seed historical targets: %v", err)
	}

	for _, migration := range migrations {
		if migration.Version != 3 {
			continue
		}
		if err := applyMigration(gormDB, migration); err != nil {
			t.Fatalf("apply migration %d: %v", migration.Version, err)
		}
	}

	assertSQLiteExamTargetScopeSpaces(t, gormDB, 1, []uint64{301, 302})
	assertSQLiteExamTargetScopeSpaces(t, gormDB, 2, nil)
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

func assertSQLiteIndexColumns(t *testing.T, gormDB *gorm.DB, indexName string, columns []string) {
	t.Helper()

	if !sqliteIndexColumnsMatch(t, gormDB, indexName, columns) {
		t.Fatalf("index %s columns should be %s", indexName, strings.Join(columns, ","))
	}
}

func assertSQLiteExamTargetScopeSpaces(t *testing.T, gormDB *gorm.DB, examTargetID uint64, want []uint64) {
	t.Helper()

	var got []uint64
	if err := gormDB.Table("exam_target_scope_spaces").
		Select("space_id").
		Where("exam_target_id = ?", examTargetID).
		Order("space_id ASC").
		Scan(&got).Error; err != nil {
		t.Fatalf("query exam target scope spaces: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("expected scope spaces %#v, got %#v", want, got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("expected scope spaces %#v, got %#v", want, got)
		}
	}
}

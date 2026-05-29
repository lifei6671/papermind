package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditActorTypeColumnsAreInInitialSchema(t *testing.T) {
	tests := []struct {
		name               string
		path               string
		createTypeFragment string
		updateTypeFragment string
	}{
		{
			name:               "postgres",
			path:               filepath.Join("..", "..", "data", "migrations", "postgres", "001_tenant_space.sql"),
			createTypeFragment: "created_by_type VARCHAR(32) NOT NULL DEFAULT 'system'",
			updateTypeFragment: "updated_by_type VARCHAR(32) NOT NULL DEFAULT 'system'",
		},
		{
			name:               "mysql",
			path:               filepath.Join("..", "..", "data", "migrations", "mysql", "001_tenant_space.sql"),
			createTypeFragment: "created_by_type VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT '创建人主体类型：platform_user / tenant_user / system'",
			updateTypeFragment: "updated_by_type VARCHAR(32) NOT NULL DEFAULT 'system' COMMENT '更新人主体类型：platform_user / tenant_user / system'",
		},
		{
			name:               "sqlite",
			path:               filepath.Join("..", "..", "data", "migrations", "sqlite", "001_tenant_space.sql"),
			createTypeFragment: "created_by_type TEXT NOT NULL DEFAULT 'system'",
			updateTypeFragment: "updated_by_type TEXT NOT NULL DEFAULT 'system'",
		},
	}

	normalTables := []string{
		"tenants",
		"spaces",
		"space_members",
		"space_configs",
		"platform_users",
		"platform_configs",
		"users",
		"tenant_user_memberships",
		"questions",
		"question_options",
		"tags",
		"papers",
		"paper_sections",
		"paper_section_questions",
		"paper_section_rules",
		"exams",
		"exam_attempts",
		"exam_attempt_questions",
		"exam_answers",
	}
	createOnlyTables := []string{
		"question_tags",
		"exam_targets",
		"exam_live_question_pools",
		"exam_events",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initial := readMigrationSQL(t, tt.path)
			for _, table := range normalTables {
				assertContains(t, initial, "CREATE TABLE IF NOT EXISTS "+table)
				assertContains(t, initial, tt.createTypeFragment)
				assertContains(t, initial, tt.updateTypeFragment)
			}
			for _, table := range createOnlyTables {
				assertContains(t, initial, "CREATE TABLE IF NOT EXISTS "+table)
				assertContains(t, initial, tt.createTypeFragment)
			}
		})
	}
}

func readMigrationSQL(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	return string(content)
}

func assertContains(t *testing.T, text string, want string) {
	t.Helper()

	if !strings.Contains(text, want) {
		t.Fatalf("missing %q", want)
	}
}

func assertNotContains(t *testing.T, text string, unwanted string) {
	t.Helper()

	if strings.Contains(text, unwanted) {
		t.Fatalf("unexpected %q", unwanted)
	}
}

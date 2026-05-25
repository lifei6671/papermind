package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestQuestionSchemaMigrationContainsRequiredTablesAndConstraints(t *testing.T) {
	tests := []struct {
		name string
		path string
		want []string
	}{
		{
			name: "postgres",
			path: filepath.Join("..", "..", "data", "migrations", "postgres", "001_tenant_space.sql"),
			want: []string{
				"CREATE TABLE IF NOT EXISTS questions",
				"type VARCHAR(32) NOT NULL",
				"analysis TEXT NOT NULL DEFAULT ''",
				"CREATE TABLE IF NOT EXISTS question_options",
				"option_key VARCHAR(32) NOT NULL",
				"sort_order INTEGER NOT NULL",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_question_options_key ON question_options (tenant_id, question_id, option_key)",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_question_options_sort_order ON question_options (tenant_id, question_id, sort_order)",
				"CREATE TABLE IF NOT EXISTS tags",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_tags_name_deleted_at ON tags (tenant_id, name, deleted_at)",
				"CREATE TABLE IF NOT EXISTS question_tags",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_question_tags_tag ON question_tags (tenant_id, question_id, tag_id)",
			},
		},
		{
			name: "mysql",
			path: filepath.Join("..", "..", "data", "migrations", "mysql", "001_tenant_space.sql"),
			want: []string{
				"CREATE TABLE IF NOT EXISTS questions",
				"type VARCHAR(32) NOT NULL COMMENT '题型：single / multiple / judge / fill_blank / short_text'",
				"analysis TEXT NOT NULL COMMENT '题目解析，出题人可选填'",
				"CREATE TABLE IF NOT EXISTS question_options",
				"option_key VARCHAR(32) NOT NULL COMMENT '出题编辑时的原始展示标签，例如 A / B / C / D，不参与判分'",
				"sort_order INT NOT NULL COMMENT '选项原始排序'",
				"UNIQUE KEY uk_question_options_key (tenant_id, question_id, option_key)",
				"UNIQUE KEY uk_question_options_sort_order (tenant_id, question_id, sort_order)",
				"CREATE TABLE IF NOT EXISTS tags",
				"UNIQUE KEY uk_tags_name_deleted_at (tenant_id, name, deleted_at)",
				"CREATE TABLE IF NOT EXISTS question_tags",
				"UNIQUE KEY uk_question_tags_tag (tenant_id, question_id, tag_id)",
			},
		},
		{
			name: "sqlite",
			path: filepath.Join("..", "..", "data", "migrations", "sqlite", "001_tenant_space.sql"),
			want: []string{
				"CREATE TABLE IF NOT EXISTS questions",
				"type TEXT NOT NULL",
				"analysis TEXT NOT NULL DEFAULT ''",
				"CREATE TABLE IF NOT EXISTS question_options",
				"option_key TEXT NOT NULL",
				"sort_order INTEGER NOT NULL",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_question_options_key ON question_options (tenant_id, question_id, option_key)",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_question_options_sort_order ON question_options (tenant_id, question_id, sort_order)",
				"CREATE TABLE IF NOT EXISTS tags",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_tags_name_deleted_at ON tags (tenant_id, name, deleted_at)",
				"CREATE TABLE IF NOT EXISTS question_tags",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_question_tags_tag ON question_tags (tenant_id, question_id, tag_id)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := os.ReadFile(tt.path)
			if err != nil {
				t.Fatalf("ReadFile() error = %v", err)
			}

			sql := string(content)
			for _, want := range tt.want {
				if !strings.Contains(sql, want) {
					t.Fatalf("%s missing %q", tt.path, want)
				}
			}
		})
	}
}

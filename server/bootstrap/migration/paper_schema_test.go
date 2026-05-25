package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPaperSchemaMigrationContainsRequiredTablesAndConstraints(t *testing.T) {
	tests := []struct {
		name string
		path string
		want []string
	}{
		{
			name: "postgres",
			path: filepath.Join("..", "..", "data", "migrations", "postgres", "001_tenant_space.sql"),
			want: []string{
				"CREATE TABLE IF NOT EXISTS papers",
				"build_mode VARCHAR(32) NOT NULL",
				"show_analysis BOOLEAN NOT NULL DEFAULT FALSE",
				"CREATE TABLE IF NOT EXISTS paper_sections",
				"deleted_at BIGINT NOT NULL DEFAULT 0",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_paper_sections_sort_order ON paper_sections (tenant_id, paper_id, sort_order)",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_paper_sections_id_paper ON paper_sections (tenant_id, id, paper_id)",
				"CREATE TABLE IF NOT EXISTS paper_section_questions",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_paper_section_questions_question ON paper_section_questions (tenant_id, paper_id, question_id)",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_paper_section_questions_sort_order ON paper_section_questions (tenant_id, section_id, sort_order)",
				"CONSTRAINT fk_paper_section_questions_section FOREIGN KEY (tenant_id, section_id, paper_id) REFERENCES paper_sections (tenant_id, id, paper_id)",
				"CREATE TABLE IF NOT EXISTS paper_section_rules",
				"difficulty VARCHAR(32)",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_paper_section_rules_sort_order ON paper_section_rules (tenant_id, section_id, sort_order)",
				"CONSTRAINT fk_paper_section_rules_section FOREIGN KEY (tenant_id, section_id, paper_id) REFERENCES paper_sections (tenant_id, id, paper_id)",
			},
		},
		{
			name: "mysql",
			path: filepath.Join("..", "..", "data", "migrations", "mysql", "001_tenant_space.sql"),
			want: []string{
				"CREATE TABLE IF NOT EXISTS papers",
				"build_mode VARCHAR(32) NOT NULL COMMENT '组卷方式：manual / rule_fixed / rule_live'",
				"show_analysis TINYINT(1) NOT NULL DEFAULT 0 COMMENT '成绩可见后是否向考生展示题目解析'",
				"CREATE TABLE IF NOT EXISTS paper_sections",
				"deleted_at BIGINT NOT NULL DEFAULT 0 COMMENT '软删除时间，0 表示未删除'",
				"UNIQUE KEY uk_paper_sections_sort_order (tenant_id, paper_id, sort_order)",
				"UNIQUE KEY uk_paper_sections_id_paper (tenant_id, id, paper_id)",
				"CREATE TABLE IF NOT EXISTS paper_section_questions",
				"UNIQUE KEY uk_paper_section_questions_question (tenant_id, paper_id, question_id)",
				"UNIQUE KEY uk_paper_section_questions_sort_order (tenant_id, section_id, sort_order)",
				"CONSTRAINT fk_paper_section_questions_section FOREIGN KEY (tenant_id, section_id, paper_id) REFERENCES paper_sections (tenant_id, id, paper_id)",
				"CREATE TABLE IF NOT EXISTS paper_section_rules",
				"difficulty VARCHAR(32) NULL COMMENT '抽题难度条件，NULL 表示不限难度'",
				"UNIQUE KEY uk_paper_section_rules_sort_order (tenant_id, section_id, sort_order)",
				"CONSTRAINT fk_paper_section_rules_section FOREIGN KEY (tenant_id, section_id, paper_id) REFERENCES paper_sections (tenant_id, id, paper_id)",
			},
		},
		{
			name: "sqlite",
			path: filepath.Join("..", "..", "data", "migrations", "sqlite", "001_tenant_space.sql"),
			want: []string{
				"CREATE TABLE IF NOT EXISTS papers",
				"build_mode TEXT NOT NULL",
				"show_analysis INTEGER NOT NULL DEFAULT 0",
				"CREATE TABLE IF NOT EXISTS paper_sections",
				"deleted_at INTEGER NOT NULL DEFAULT 0",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_paper_sections_sort_order ON paper_sections (tenant_id, paper_id, sort_order)",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_paper_sections_id_paper ON paper_sections (tenant_id, id, paper_id)",
				"CREATE TABLE IF NOT EXISTS paper_section_questions",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_paper_section_questions_question ON paper_section_questions (tenant_id, paper_id, question_id)",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_paper_section_questions_sort_order ON paper_section_questions (tenant_id, section_id, sort_order)",
				"FOREIGN KEY (tenant_id, section_id, paper_id) REFERENCES paper_sections (tenant_id, id, paper_id)",
				"CREATE TABLE IF NOT EXISTS paper_section_rules",
				"difficulty TEXT",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_paper_section_rules_sort_order ON paper_section_rules (tenant_id, section_id, sort_order)",
				"FOREIGN KEY (tenant_id, section_id, paper_id) REFERENCES paper_sections (tenant_id, id, paper_id)",
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

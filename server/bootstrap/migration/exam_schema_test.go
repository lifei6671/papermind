package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExamSchemaMigrationContainsRequiredTablesAndConstraints(t *testing.T) {
	tests := []struct {
		name string
		path string
		want []string
	}{
		{
			name: "postgres",
			path: filepath.Join("..", "..", "data", "migrations", "postgres", "001_tenant_space.sql"),
			want: []string{
				"CREATE TABLE IF NOT EXISTS exams",
				"max_attempts INTEGER NOT NULL DEFAULT 1",
				"result_strategy VARCHAR(32) NOT NULL DEFAULT 'latest'",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_exams_invite_code ON exams (invite_code)",
				"CREATE TABLE IF NOT EXISTS exam_targets",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_exam_targets_target ON exam_targets (tenant_id, exam_id, target_type, target_id)",
				"CREATE TABLE IF NOT EXISTS exam_live_question_pools",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_exam_live_question_pools_question ON exam_live_question_pools (tenant_id, exam_id, section_id, rule_id, question_id)",
				"CREATE TABLE IF NOT EXISTS exam_attempts",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_exam_attempts_attempt_no ON exam_attempts (tenant_id, exam_id, user_id, attempt_no)",
				"CREATE INDEX IF NOT EXISTS idx_exam_attempts_token_status ON exam_attempts (exam_token_hash, status)",
				"CREATE TABLE IF NOT EXISTS exam_attempt_questions",
				"section_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb",
				"sort_order INTEGER NOT NULL",
				"CREATE TABLE IF NOT EXISTS exam_answers",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_exam_answers_attempt_question ON exam_answers (tenant_id, attempt_id, attempt_question_id)",
				"CREATE TABLE IF NOT EXISTS exam_events",
			},
		},
		{
			name: "mysql",
			path: filepath.Join("..", "..", "data", "migrations", "mysql", "001_tenant_space.sql"),
			want: []string{
				"CREATE TABLE IF NOT EXISTS exams",
				"max_attempts INT NOT NULL DEFAULT 1 COMMENT '每名考生最多作答次数'",
				"result_strategy VARCHAR(32) NOT NULL DEFAULT 'latest' COMMENT '多次作答成绩策略：latest / highest'",
				"UNIQUE KEY uk_exams_invite_code (invite_code)",
				"CREATE TABLE IF NOT EXISTS exam_targets",
				"UNIQUE KEY uk_exam_targets_target (tenant_id, exam_id, target_type, target_id)",
				"CREATE TABLE IF NOT EXISTS exam_live_question_pools",
				"UNIQUE KEY uk_exam_live_question_pools_question (tenant_id, exam_id, section_id, rule_id, question_id)",
				"CREATE TABLE IF NOT EXISTS exam_attempts",
				"UNIQUE KEY uk_exam_attempts_attempt_no (tenant_id, exam_id, user_id, attempt_no)",
				"KEY idx_exam_attempts_token_status (exam_token_hash, status)",
				"CREATE TABLE IF NOT EXISTS exam_attempt_questions",
				"section_snapshot JSON NOT NULL COMMENT '大题快照 JSON，包含大题名称和作答说明'",
				"sort_order INT NOT NULL COMMENT '该考生看到的全局题号，从 1 连续递增'",
				"CREATE TABLE IF NOT EXISTS exam_answers",
				"UNIQUE KEY uk_exam_answers_attempt_question (tenant_id, attempt_id, attempt_question_id)",
				"CREATE TABLE IF NOT EXISTS exam_events",
			},
		},
		{
			name: "sqlite",
			path: filepath.Join("..", "..", "data", "migrations", "sqlite", "001_tenant_space.sql"),
			want: []string{
				"CREATE TABLE IF NOT EXISTS exams",
				"max_attempts INTEGER NOT NULL DEFAULT 1",
				"result_strategy TEXT NOT NULL DEFAULT 'latest'",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_exams_invite_code ON exams (invite_code)",
				"CREATE TABLE IF NOT EXISTS exam_targets",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_exam_targets_target ON exam_targets (tenant_id, exam_id, target_type, target_id)",
				"CREATE TABLE IF NOT EXISTS exam_live_question_pools",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_exam_live_question_pools_question ON exam_live_question_pools (tenant_id, exam_id, section_id, rule_id, question_id)",
				"CREATE TABLE IF NOT EXISTS exam_attempts",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_exam_attempts_attempt_no ON exam_attempts (tenant_id, exam_id, user_id, attempt_no)",
				"CREATE INDEX IF NOT EXISTS idx_exam_attempts_token_status ON exam_attempts (exam_token_hash, status)",
				"CREATE TABLE IF NOT EXISTS exam_attempt_questions",
				"section_snapshot TEXT NOT NULL DEFAULT '{}'",
				"sort_order INTEGER NOT NULL",
				"CREATE TABLE IF NOT EXISTS exam_answers",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_exam_answers_attempt_question ON exam_answers (tenant_id, attempt_id, attempt_question_id)",
				"CREATE TABLE IF NOT EXISTS exam_events",
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

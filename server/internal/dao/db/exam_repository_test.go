package db

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	serviceexam "github.com/lifei6671/papermind/server/internal/service/exam"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestExamRepositoryInviteCodeExistsChecksAllTenants(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	repo := NewExamRepository(gormDB, ExamRepositoryOptions{Now: func() int64 { return 1000 }})

	if err := gormDB.Exec(`
		INSERT INTO papers (
			id, tenant_id, name, description, total_score, build_mode, status,
			created_at, updated_at, ext_json
		) VALUES
			(100, 10, '租户 10 试卷', '', 100, 'manual', 'enabled', 1000, 1000, '{}'),
			(200, 20, '租户 20 试卷', '', 100, 'manual', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed papers: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO exams (
			id, tenant_id, paper_id, name, start_time, end_time, duration_minutes,
			max_attempts, result_strategy, publish_mode, invite_code, status,
			created_at, updated_at, ext_json
		) VALUES (1, 10, 100, '租户 10 考试', 1000, 2000, 30, 1, 'latest', 'manual_publish', 'PM2026', 'published', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed exam: %v", err)
	}

	exists, err := repo.InviteCodeExists(t.Context(), 20, "PM2026")
	if err != nil {
		t.Fatalf("InviteCodeExists returned error: %v", err)
	}
	if !exists {
		t.Fatalf("expected invite code collision across tenants to be detected")
	}
}

func TestExamRepositoryRuleLiveFreezeAndSnapshot(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	repo := NewExamRepository(gormDB, ExamRepositoryOptions{Now: func() int64 { return 1000 }})
	seedRuleLiveRepositoryData(t, gormDB)

	candidates, err := repo.ListRuleLiveCandidates(t.Context(), 10, 100)
	if err != nil {
		t.Fatalf("ListRuleLiveCandidates returned error: %v", err)
	}
	if len(candidates) != 1 || candidates[0].SectionID != 300 || candidates[0].RuleID != 400 || candidates[0].QuestionID != 500 {
		t.Fatalf("unexpected candidates: %#v", candidates)
	}

	exam, err := repo.PublishExamAndFreezeLivePool(t.Context(), serviceExamForRuleLive(), candidates)
	if err != nil {
		t.Fatalf("PublishExamAndFreezeLivePool returned error: %v", err)
	}
	if exam.InviteCode != "LIVE2026" || exam.Status != "published" {
		t.Fatalf("unexpected published exam: %#v", exam)
	}

	snapshots, err := repo.ListFrozenLiveSnapshotQuestions(t.Context(), 10, 900)
	if err != nil {
		t.Fatalf("ListFrozenLiveSnapshotQuestions returned error: %v", err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("expected one frozen snapshot source, got %#v", snapshots)
	}
	got := snapshots[0]
	if got.SectionID != 300 || got.QuestionID != 500 || got.Title != "rule live 题干" || got.Score != "5" {
		t.Fatalf("unexpected snapshot source: %#v", got)
	}
	if len(got.Options) != 2 || got.Options[0].ID != 501 || len(got.CorrectOptionIDs) != 1 || got.CorrectOptionIDs[0] != 501 {
		t.Fatalf("unexpected snapshot options: %#v", got)
	}
}

func TestExamRepositoryRuleLiveCandidatesRejectsCrossRuleDuplicatePool(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	repo := NewExamRepository(gormDB, ExamRepositoryOptions{Now: func() int64 { return 1000 }})
	seedRuleLiveRepositoryData(t, gormDB)

	tagFilter, err := json.Marshal([]uint64{700})
	if err != nil {
		t.Fatalf("marshal tag filter: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO paper_section_rules (
			id, tenant_id, section_id, paper_id, sort_order, difficulty, tag_filter, question_count, score_per_question, shuffle_options,
			created_at, updated_at, ext_json
		) VALUES (401, 10, 300, 100, 2, 'easy', ?, 1, 5, false, 1000, 1000, '{}')
	`, string(tagFilter)).Error; err != nil {
		t.Fatalf("seed duplicate live rule: %v", err)
	}

	_, err = repo.ListRuleLiveCandidates(t.Context(), 10, 100)
	if !errors.Is(err, serviceexam.ErrRuleLiveQuestionPoolInsufficient) {
		t.Fatalf("expected insufficient pool after cross-rule dedupe, got %v", err)
	}
}

func openExamRepositoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	gormDB, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared&_foreign_keys=on"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlPath := filepath.Join("..", "..", "..", "data", "migrations", "sqlite", "001_tenant_space.sql")
	sqlBytes, err := os.ReadFile(sqlPath)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if err := gormDB.Exec(string(sqlBytes)).Error; err != nil {
		t.Fatalf("apply migration: %v", err)
	}
	return gormDB
}

func serviceExamForRuleLive() serviceexam.Exam {
	return serviceexam.Exam{
		ID:              900,
		TenantID:        10,
		PaperID:         100,
		StartTime:       1000,
		EndTime:         2000,
		DurationMinutes: 10,
		MaxAttempts:     1,
		ResultStrategy:  serviceexam.ResultStrategyLatest,
		PublishMode:     serviceexam.PublishModeManualPublish,
		InviteCode:      "LIVE2026",
		Status:          serviceexam.StatusPublished,
		BuildMode:       serviceexam.BuildModeRuleLive,
	}
}

func seedRuleLiveRepositoryData(t *testing.T, gormDB *gorm.DB) {
	t.Helper()

	tagFilter, err := json.Marshal([]uint64{700})
	if err != nil {
		t.Fatalf("marshal tag filter: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO papers (
			id, tenant_id, name, description, total_score, build_mode, status,
			created_at, updated_at, ext_json
		) VALUES (100, 10, 'rule live 试卷', '', 5, 'rule_live', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed paper: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO exams (
			id, tenant_id, paper_id, name, start_time, end_time, duration_minutes,
			max_attempts, result_strategy, publish_mode, invite_code, status,
			created_at, updated_at, ext_json
		) VALUES (900, 10, 100, 'rule live 考试', 0, 0, 0, 1, 'latest', 'manual_publish', '', 'draft', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed exam: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO paper_sections (
			id, tenant_id, paper_id, sort_order, name, question_type, instructions, total_score, question_count,
			created_at, updated_at, ext_json
		) VALUES (300, 10, 100, 1, '一、实时单选', 'single', '从题池抽题', 5, 1, 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed section: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO paper_section_rules (
			id, tenant_id, section_id, paper_id, sort_order, difficulty, tag_filter, question_count, score_per_question, shuffle_options,
			created_at, updated_at, ext_json
		) VALUES (400, 10, 300, 100, 1, 'easy', ?, 1, 5, false, 1000, 1000, '{}')
	`, string(tagFilter)).Error; err != nil {
		t.Fatalf("seed rule: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO questions (
			id, tenant_id, type, difficulty, title, analysis, score_default, shuffle_options, status,
			created_at, updated_at, ext_json
		) VALUES (500, 10, 'single', 'easy', 'rule live 题干', '解析', 5, false, 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed question: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO question_options (
			id, tenant_id, question_id, option_key, sort_order, content, is_correct, is_distractor,
			created_at, updated_at, ext_json
		) VALUES
			(501, 10, 500, 'A', 1, '正确', true, false, 1000, 1000, '{}'),
			(502, 10, 500, 'B', 2, '错误', false, true, 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed options: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO tags (
			id, tenant_id, name, created_at, updated_at, ext_json
		) VALUES (700, 10, '必修', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed tag: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO question_tags (
			id, tenant_id, question_id, tag_id, created_at, ext_json
		) VALUES (800, 10, 500, 700, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed question tag: %v", err)
	}
}

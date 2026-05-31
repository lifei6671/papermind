package db

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lifei6671/papermind/server/bootstrap/migration"
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

func TestExamRepositoryFixedSnapshotCarriesFillBlankStandardAnswer(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	repo := NewExamRepository(gormDB, ExamRepositoryOptions{Now: func() int64 { return 1000 }})

	if err := gormDB.Exec(`
		INSERT INTO papers (
			id, tenant_id, name, description, total_score, build_mode, status,
			created_at, updated_at, ext_json
		) VALUES (100, 10, '固定填空试卷', '', 3, 'manual', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed paper: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO exams (
			id, tenant_id, paper_id, name, start_time, end_time, duration_minutes,
			max_attempts, result_strategy, publish_mode, invite_code, status,
			created_at, updated_at, ext_json
		) VALUES (900, 10, 100, '固定填空考试', 0, 0, 0, 1, 'latest', 'manual_publish', '', 'published', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed exam: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO paper_sections (
			id, tenant_id, paper_id, sort_order, name, question_type, instructions, total_score, question_count,
			created_at, updated_at, ext_json
		) VALUES (300, 10, 100, 1, '一、填空题', 'fill_blank', '填写准确答案', 3, 1, 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed section: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO questions (
			id, tenant_id, type, difficulty, title, analysis, score_default, shuffle_options, standard_answer, status,
			created_at, updated_at, ext_json
		) VALUES (500, 10, 'fill_blank', 'easy', 'Go 的包管理文件是 ____。', '', 3, false, 'go.mod', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed fill blank question: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO paper_section_questions (
			id, tenant_id, section_id, paper_id, question_id, sort_order, score, created_at, updated_at, ext_json
		) VALUES (600, 10, 300, 100, 500, 1, 3, 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed section question: %v", err)
	}

	snapshots, err := repo.ListFixedSnapshotQuestions(t.Context(), 10, 900)
	if err != nil {
		t.Fatalf("ListFixedSnapshotQuestions returned error: %v", err)
	}
	if len(snapshots) != 1 {
		t.Fatalf("expected one snapshot source, got %#v", snapshots)
	}
	if snapshots[0].CorrectText != "go.mod" {
		t.Fatalf("expected fill blank standard answer in snapshot source, got %#v", snapshots[0])
	}
}

func TestExamRepositoryReviewAndScoreRowsDoNotDuplicateMultiSpaceStudent(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	repo := NewExamRepository(gormDB, ExamRepositoryOptions{Now: func() int64 { return 1000 }})
	seedMultiSpaceAttemptData(t, gormDB)

	pending, err := repo.ListPendingAttempts(t.Context(), 10, 900)
	if err != nil {
		t.Fatalf("ListPendingAttempts returned error: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected one pending answer, got %#v", pending)
	}
	if pending[0].PendingShortTextCount != 1 {
		t.Fatalf("expected one pending short text count, got %d", pending[0].PendingShortTextCount)
	}
	if pending[0].SpaceID != 302 {
		t.Fatalf("expected pending review to use exam target space 302, got %#v", pending[0])
	}

	rows, err := repo.ListScoreExportRows(t.Context(), 10, 900)
	if err != nil {
		t.Fatalf("ListScoreExportRows returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected one score export row, got %#v", rows)
	}
	if rows[0].SpaceID != 302 {
		t.Fatalf("expected score export to use exam target space 302, got %#v", rows[0])
	}

	spaces, err := repo.AttemptSpaceIDs(t.Context(), 10, 700)
	if err != nil {
		t.Fatalf("AttemptSpaceIDs returned error: %v", err)
	}
	if len(spaces) != 1 || spaces[0] != 302 {
		t.Fatalf("expected attempt spaces from exam target only, got %#v", spaces)
	}
}

func TestExamRepositoryReviewAndScoreRowsResolveUserTargetSpaces(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	repo := NewExamRepository(gormDB, ExamRepositoryOptions{Now: func() int64 { return 1000 }})
	seedMultiSpaceAttemptData(t, gormDB)
	if err := gormDB.Exec(`
		UPDATE exam_targets
		SET target_type = 'user', target_id = 21
		WHERE id = 1
	`).Error; err != nil {
		t.Fatalf("switch exam target to user: %v", err)
	}

	pending, err := repo.ListPendingAttempts(t.Context(), 10, 900)
	if err != nil {
		t.Fatalf("ListPendingAttempts returned error: %v", err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected one pending answer, got %#v", pending)
	}
	assertUint64s(t, pending[0].SpaceIDs, []uint64{301, 302})
	if pending[0].SpaceID != 301 {
		t.Fatalf("expected pending review display space 301, got %#v", pending[0])
	}

	rows, err := repo.ListScoreExportRows(t.Context(), 10, 900)
	if err != nil {
		t.Fatalf("ListScoreExportRows returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected one score export row, got %#v", rows)
	}
	assertUint64s(t, rows[0].SpaceIDs, []uint64{301, 302})
	if rows[0].SpaceID != 301 {
		t.Fatalf("expected score export display space 301, got %#v", rows[0])
	}

	spaces, err := repo.AttemptSpaceIDs(t.Context(), 10, 700)
	if err != nil {
		t.Fatalf("AttemptSpaceIDs returned error: %v", err)
	}
	assertUint64s(t, spaces, []uint64{301, 302})
}

func TestExamRepositoryRejectsAnswerUpsertAfterAttemptSubmitted(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	repo := NewExamRepository(gormDB, ExamRepositoryOptions{Now: func() int64 { return 2000 }})
	seedMultiSpaceAttemptData(t, gormDB)

	err := repo.UpsertAnswer(t.Context(), serviceexam.Answer{
		TenantID:          10,
		AttemptID:         700,
		AttemptQuestionID: 800,
		AnswerContent:     "提交后的覆盖答案",
		UpdatedBy:         21,
	})
	if !errors.Is(err, serviceexam.ErrAttemptAlreadySubmitted) {
		t.Fatalf("expected ErrAttemptAlreadySubmitted, got %v", err)
	}

	var answerContent string
	if err := gormDB.Table("exam_answers").
		Select("answer_content").
		Where("tenant_id = ? AND attempt_id = ? AND attempt_question_id = ?", 10, 700, 800).
		Scan(&answerContent).Error; err != nil {
		t.Fatalf("query answer content: %v", err)
	}
	if answerContent != "答案" {
		t.Fatalf("submitted attempt answer should not be overwritten, got %q", answerContent)
	}
}

func TestExamRepositoryCreateAttemptMapsUniqueConflict(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	repo := NewExamRepository(gormDB, ExamRepositoryOptions{Now: func() int64 { return 1000 }})
	seedMultiSpaceAttemptData(t, gormDB)

	_, err := repo.CreateAttempt(t.Context(), serviceexam.Attempt{
		TenantID:  10,
		ExamID:    900,
		UserID:    21,
		AttemptNo: 1,
		Status:    serviceexam.AttemptStatusInProgress,
		StartedAt: 1600,
	})
	if !errors.Is(err, serviceexam.ErrAttemptUniqueConflict) {
		t.Fatalf("expected ErrAttemptUniqueConflict, got %v", err)
	}
}

func openExamRepositoryTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	gormDB, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared&_foreign_keys=on"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	migrationDir := filepath.Join("..", "..", "..", "data", "migrations", "sqlite")
	if err := migration.Run(gormDB, migrationDir); err != nil {
		t.Fatalf("run migration: %v", err)
	}
	return gormDB
}

func assertUint64s(t *testing.T, got []uint64, want []uint64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("expected %#v, got %#v", want, got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("expected %#v, got %#v", want, got)
		}
	}
}

func seedMultiSpaceAttemptData(t *testing.T, gormDB *gorm.DB) {
	t.Helper()

	if err := gormDB.Exec(`
		INSERT INTO spaces (
			id, tenant_id, name, created_at, updated_at, ext_json
		) VALUES
			(301, 10, '高一一班', 1000, 1000, '{}'),
			(302, 10, '语文培优班', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed spaces: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO users (
			id, username, real_name, phone, email, password_hash, status,
			created_at, updated_at, ext_json
		) VALUES (21, 'student21', '多空间考生', '13800000021', 'student21@example.test', 'hash', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO tenant_user_memberships (
			id, tenant_id, user_id, role, status, created_at, updated_at, ext_json
		) VALUES (21, 10, 21, 'student', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed tenant user membership: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO space_members (
			id, tenant_id, space_id, user_id, role_in_space, status, created_at, updated_at, ext_json
		) VALUES
			(1, 10, 301, 21, 'student', 'enabled', 1000, 1000, '{}'),
			(2, 10, 302, 21, 'student', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed space members: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO exams (
			id, tenant_id, paper_id, name, start_time, end_time, duration_minutes,
			max_attempts, result_strategy, publish_mode, invite_code, status,
			created_at, updated_at, ext_json
		) VALUES (900, 10, 100, '多空间考试', 1000, 2000, 30, 1, 'latest', 'manual_publish', 'MULTI2026', 'published', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed exam: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO exam_targets (
			id, tenant_id, exam_id, target_type, target_id, created_at, ext_json
		) VALUES (1, 10, 900, 'space', 302, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed exam target: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO exam_attempts (
			id, tenant_id, exam_id, user_id, attempt_no, status, started_at, submitted_at,
			exam_token_hash, exam_token_expires_at, objective_score, subjective_score, total_score,
			created_at, updated_at, ext_json
		) VALUES (700, 10, 900, 21, 1, 'submitted', 1000, 1500, 'token-hash', 3000, 6, 0, 6, 1000, 1500, '{}')
	`).Error; err != nil {
		t.Fatalf("seed attempt: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO exam_attempt_questions (
			id, tenant_id, attempt_id, section_id, question_id, section_snapshot, sort_order, score,
			question_snapshot, option_snapshot, correct_answer_snapshot, created_at, updated_at, ext_json
		) VALUES (800, 10, 700, 300, 500, '{}', 1, 10, '{"title":"简答题"}', '[]', '{}', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed attempt question: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO exam_answers (
			id, tenant_id, attempt_id, attempt_question_id, answer_content, score, grading_status,
			created_at, updated_at, ext_json
		) VALUES (900, 10, 700, 800, '答案', 0, 'pending', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed answer: %v", err)
	}
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

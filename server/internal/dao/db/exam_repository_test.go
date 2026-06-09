package db

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
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

	exam, err := repo.PublishExamAndFreezeLivePool(t.Context(), serviceExamForRuleLive(), candidates, "", nil)
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

func TestExamRepositoryCreatePublishedExamWithTargetsWritesAllTargets(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	repo := NewExamRepository(gormDB, ExamRepositoryOptions{Now: func() int64 { return 1000 }})
	if err := gormDB.Exec(`
		INSERT INTO papers (
			id, tenant_id, name, description, total_score, build_mode, status,
			created_at, updated_at, ext_json
		) VALUES (100, 10, '多目标试卷', '', 100, 'manual', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed paper: %v", err)
	}

	created, err := repo.CreatePublishedExamWithTargets(t.Context(), serviceexam.Exam{
		TenantID:        10,
		PaperID:         100,
		Name:            "多目标考试",
		StartTime:       1000,
		EndTime:         2000,
		DurationMinutes: 30,
		MaxAttempts:     1,
		ResultStrategy:  serviceexam.ResultStrategyLatest,
		PublishMode:     serviceexam.PublishModeManualPublish,
		InviteCode:      "MULTI2026",
		Status:          serviceexam.StatusPublished,
	}, nil, []serviceexam.Target{
		{TenantID: 10, TargetType: serviceexam.TargetTypeSpace, TargetID: 301},
		{TenantID: 10, TargetType: serviceexam.TargetTypeUser, TargetID: 21, ScopeSpaceIDs: []uint64{301}},
	}, nil)
	if err != nil {
		t.Fatalf("CreatePublishedExamWithTargets returned error: %v", err)
	}
	if created.ID == 0 || created.BuildMode != serviceexam.BuildModeManual {
		t.Fatalf("expected created exam with paper build mode, got %#v", created)
	}

	targets, err := repo.ListTargets(t.Context(), 10, created.ID)
	if err != nil {
		t.Fatalf("ListTargets returned error: %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("expected two targets, got %#v", targets)
	}
	if targets[0].ExamID != created.ID || targets[1].ExamID != created.ID {
		t.Fatalf("expected targets bound to created exam, got %#v", targets)
	}
	if targets[0].TargetType != serviceexam.TargetTypeSpace || targets[0].TargetID != 301 ||
		targets[1].TargetType != serviceexam.TargetTypeUser || targets[1].TargetID != 21 {
		t.Fatalf("expected targets to keep stable creation order, got %#v", targets)
	}
	var userTargetID uint64
	var userTargetExtJSON string
	if err := gormDB.Table("exam_targets").
		Select("id, ext_json").
		Where("tenant_id = ? AND exam_id = ? AND target_type = ? AND target_id = ?", 10, created.ID, serviceexam.TargetTypeUser, 21).
		Row().
		Scan(&userTargetID, &userTargetExtJSON); err != nil {
		t.Fatalf("query user target ext json: %v", err)
	}
	if userTargetExtJSON != `{}` {
		t.Fatalf("expected user target ext_json to stay empty, got %s", userTargetExtJSON)
	}
	assertExamTargetScopeSpaces(t, gormDB, 10, created.ID, userTargetID, []uint64{301})
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

	pending, err := repo.ListPendingAttempts(t.Context(), 10, 900, "")
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
	pending, err = repo.ListPendingAttempts(t.Context(), 10, 900, "语文培优班")
	if err != nil {
		t.Fatalf("ListPendingAttempts with keyword returned error: %v", err)
	}
	if len(pending) != 1 || pending[0].SpaceID != 302 {
		t.Fatalf("expected keyword search to match target space without duplicating rows, got %#v", pending)
	}
	pending, err = repo.ListPendingAttempts(t.Context(), 10, 900, "不存在")
	if err != nil {
		t.Fatalf("ListPendingAttempts with missing keyword returned error: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected missing keyword to return no pending rows, got %#v", pending)
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

func TestExamRepositoryReviewAndResultRowsExcludeAttemptAfterStudentRoleRemoved(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	repo := NewExamRepository(gormDB, ExamRepositoryOptions{Now: func() int64 { return 1000 }})
	seedMultiSpaceAttemptData(t, gormDB)
	if err := gormDB.Exec(`
		UPDATE space_members
		SET role_in_space = 'teacher'
		WHERE tenant_id = 10 AND user_id = 21 AND space_id = 302
	`).Error; err != nil {
		t.Fatalf("switch space role: %v", err)
	}
	if err := gormDB.Exec(`
		UPDATE tenant_user_memberships
		SET role = 'teacher'
		WHERE tenant_id = 10 AND user_id = 21
	`).Error; err != nil {
		t.Fatalf("switch tenant role: %v", err)
	}

	pending, err := repo.ListPendingAttempts(t.Context(), 10, 900, "语文培优班")
	if err != nil {
		t.Fatalf("ListPendingAttempts returned error: %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected role-changed attempt to leave pending review scope, got %#v", pending)
	}
	rows, err := repo.ListScoreExportRows(t.Context(), 10, 900)
	if err != nil {
		t.Fatalf("ListScoreExportRows returned error: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected role-changed attempt to leave score export scope, got %#v", rows)
	}
	spaces, err := repo.AttemptSpaceIDs(t.Context(), 10, 700)
	if err != nil {
		t.Fatalf("AttemptSpaceIDs returned error: %v", err)
	}
	if len(spaces) != 0 {
		t.Fatalf("expected role-changed attempt to have no review target spaces, got %#v", spaces)
	}
	results, err := repo.ListExamResults(t.Context(), serviceexam.ListExamResultsInput{
		TenantID: 10,
		ExamID:   900,
		SpaceIDs: []uint64{302},
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("ListExamResults returned error: %v", err)
	}
	if results.Total != 0 || len(results.Items) != 0 {
		t.Fatalf("expected role-changed attempt to leave management result scope, got %#v", results)
	}
	_, err = repo.GetAnswerSheet(t.Context(), serviceexam.AnswerSheetRepositoryInput{
		TenantID:  10,
		ExamID:    900,
		AttemptID: 700,
		SpaceIDs:  []uint64{302},
	})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected role-changed answer sheet to be hidden, got %v", err)
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

	pending, err := repo.ListPendingAttempts(t.Context(), 10, 900, "")
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

func TestExamRepositoryHistoricalTargetSpacesFallbackAfterMembershipRemoved(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	repo := NewExamRepository(gormDB, ExamRepositoryOptions{Now: func() int64 { return 1000 }})
	seedMultiSpaceAttemptData(t, gormDB)

	if err := gormDB.Exec(`
		DELETE FROM space_members
		WHERE tenant_id = 10 AND user_id = 21 AND space_id = 302
	`).Error; err != nil {
		t.Fatalf("delete target space membership: %v", err)
	}

	rows, err := repo.ListScoreExportRows(t.Context(), 10, 900)
	if err != nil {
		t.Fatalf("ListScoreExportRows returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected one score export row, got %#v", rows)
	}
	assertUint64s(t, rows[0].SpaceIDs, []uint64{302})
	if rows[0].SpaceID != 302 {
		t.Fatalf("expected historical score export display space 302, got %#v", rows[0])
	}

	spaces, err := repo.AttemptSpaceIDs(t.Context(), 10, 700)
	if err != nil {
		t.Fatalf("AttemptSpaceIDs returned error: %v", err)
	}
	assertUint64s(t, spaces, []uint64{302})
}

func TestExamRepositoryExamTargetSpaceIDsExpandsUserTargetByCurrentEnabledMemberships(t *testing.T) {
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

	spaces, err := repo.ExamTargetSpaceIDs(t.Context(), 10, 900)
	if err != nil {
		t.Fatalf("ExamTargetSpaceIDs returned error: %v", err)
	}
	assertUint64s(t, spaces, []uint64{301, 302})

	if err := gormDB.Exec(`
		UPDATE space_members
		SET status = 'disabled'
		WHERE tenant_id = 10 AND user_id = 21 AND space_id = 302
	`).Error; err != nil {
		t.Fatalf("disable one user target membership: %v", err)
	}

	spaces, err = repo.ExamTargetSpaceIDs(t.Context(), 10, 900)
	if err != nil {
		t.Fatalf("ExamTargetSpaceIDs after membership disabled returned error: %v", err)
	}
	assertUint64s(t, spaces, []uint64{301})
}

func TestExamRepositoryExamTargetSpaceIDsExcludesUserTargetWithoutStudentRole(t *testing.T) {
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
	if err := gormDB.Exec(`
		UPDATE space_members
		SET role_in_space = 'teacher'
		WHERE tenant_id = 10 AND user_id = 21 AND space_id = 302
	`).Error; err != nil {
		t.Fatalf("switch user space role: %v", err)
	}
	if err := gormDB.Exec(`
		UPDATE tenant_user_memberships
		SET role = 'teacher'
		WHERE tenant_id = 10 AND user_id = 21
	`).Error; err != nil {
		t.Fatalf("switch tenant user role: %v", err)
	}

	spaces, err := repo.ExamTargetSpaceIDs(t.Context(), 10, 900)
	if err != nil {
		t.Fatalf("ExamTargetSpaceIDs returned error: %v", err)
	}
	assertUint64s(t, spaces, []uint64{})
}

func TestExamRepositoryUserTargetPersistsScopedSpacesInsteadOfLeakingCurrentMemberships(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	repo := NewExamRepository(gormDB, ExamRepositoryOptions{Now: func() int64 { return 1000 }})
	seedMultiSpaceAttemptData(t, gormDB)
	if err := gormDB.Exec(`
		UPDATE exam_targets
		SET target_type = 'user', target_id = 21, ext_json = '{}'
		WHERE id = 1
	`).Error; err != nil {
		t.Fatalf("scope user target to one space: %v", err)
	}
	seedExamTargetScopeSpaces(t, gormDB, 10, 900, 1, 301)

	spaces, err := repo.ExamTargetSpaceIDs(t.Context(), 10, 900)
	if err != nil {
		t.Fatalf("ExamTargetSpaceIDs returned error: %v", err)
	}
	assertUint64s(t, spaces, []uint64{301})

	candidates, err := repo.CountExamCandidates(t.Context(), 10, 900, []uint64{302})
	if err != nil {
		t.Fatalf("CountExamCandidates returned error: %v", err)
	}
	if candidates != 0 {
		t.Fatalf("expected scoped user target to stay out of unrelated space 302, got %d", candidates)
	}

	result, err := repo.ListExamCandidates(t.Context(), serviceexam.ListExamCandidatesInput{
		TenantID:       10,
		ExamID:         900,
		SpaceIDs:       []uint64{302},
		ResultStrategy: serviceexam.ResultStrategyLatest,
		Page:           1,
		PageSize:       20,
	})
	if err != nil {
		t.Fatalf("ListExamCandidates returned error: %v", err)
	}
	if result.Total != 0 || len(result.Items) != 0 {
		t.Fatalf("expected scoped user target to be hidden from unrelated space 302, got %#v", result)
	}

	spaces, err = repo.AttemptSpaceIDs(t.Context(), 10, 700)
	if err != nil {
		t.Fatalf("AttemptSpaceIDs returned error: %v", err)
	}
	assertUint64s(t, spaces, []uint64{301})

	list, err := repo.ListExams(t.Context(), serviceexam.ListInput{
		TenantID: 10,
		SpaceID:  uint64Ptr(302),
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("ListExams returned error: %v", err)
	}
	if list.Total != 0 || len(list.Items) != 0 {
		t.Fatalf("expected scoped user target exam to stay out of unrelated space 302 list, got %#v", list)
	}

	stats, err := repo.CountExamAttemptStats(t.Context(), 10, 900, []uint64{302})
	if err != nil {
		t.Fatalf("CountExamAttemptStats returned error: %v", err)
	}
	if stats.Joined != 0 || stats.Submitted != 0 || stats.InProgress != 0 {
		t.Fatalf("expected scoped user target attempt stats to stay out of unrelated space 302, got %#v", stats)
	}

	if err := gormDB.Exec(`
		DELETE FROM space_members
		WHERE tenant_id = 10 AND user_id = 21 AND space_id = 301
	`).Error; err != nil {
		t.Fatalf("delete scoped student space membership: %v", err)
	}

	eligible, err := repo.IsEligible(t.Context(), 10, 900, 21)
	if err != nil {
		t.Fatalf("IsEligible returned error: %v", err)
	}
	if eligible {
		t.Fatalf("expected scoped user target without matching scoped membership to be ineligible")
	}

	results, err := repo.ListExamResults(t.Context(), serviceexam.ListExamResultsInput{
		TenantID: 10,
		ExamID:   900,
		SpaceIDs: []uint64{302},
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("ListExamResults returned error: %v", err)
	}
	if results.Total != 0 || len(results.Items) != 0 {
		t.Fatalf("expected scoped user target results to stay out of unrelated space 302, got %#v", results)
	}

	results, err = repo.ListExamResults(t.Context(), serviceexam.ListExamResultsInput{
		TenantID: 10,
		ExamID:   900,
		SpaceIDs: []uint64{302},
		Keyword:  "语文培优班",
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("ListExamResults keyword search returned error: %v", err)
	}
	if results.Total != 0 || len(results.Items) != 0 {
		t.Fatalf("expected scoped user target keyword search to stay out of unrelated space 302, got %#v", results)
	}

	_, err = repo.GetAnswerSheet(t.Context(), serviceexam.AnswerSheetRepositoryInput{
		TenantID:  10,
		ExamID:    900,
		AttemptID: 700,
		SpaceIDs:  []uint64{302},
	})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected scoped user target answer sheet to stay out of unrelated space 302, got %v", err)
	}
}

func TestExamRepositoryListExamsScopesReturnedTargetsToRequestedSpace(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	repo := NewExamRepository(gormDB, ExamRepositoryOptions{Now: func() int64 { return 1000 }})
	seedMultiSpaceAttemptData(t, gormDB)
	if err := gormDB.Exec(`
		UPDATE exam_targets
		SET target_type = 'user', target_id = 21, ext_json = '{}'
		WHERE id = 1
	`).Error; err != nil {
		t.Fatalf("scope visible user target to 301: %v", err)
	}
	seedExamTargetScopeSpaces(t, gormDB, 10, 900, 1, 301)
	if err := gormDB.Exec(`
		INSERT INTO exam_targets (
			id, tenant_id, exam_id, target_type, target_id, created_at, ext_json
		) VALUES (2, 10, 900, 'space', 302, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed unrelated space target: %v", err)
	}

	list, err := repo.ListExams(t.Context(), serviceexam.ListInput{
		TenantID: 10,
		SpaceID:  uint64Ptr(301),
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("ListExams returned error: %v", err)
	}
	if list.Total != 1 || len(list.Items) != 1 {
		t.Fatalf("expected scoped user target exam to stay visible in space 301, got %#v", list)
	}
	if len(list.Items[0].Targets) != 1 || list.Items[0].Targets[0].TargetType != serviceexam.TargetTypeUser || list.Items[0].Targets[0].TargetID != 21 {
		t.Fatalf("expected scoped list to hide unrelated space 302 target, got %#v", list.Items[0].Targets)
	}
}

func TestExamRepositoryOverviewStatsIgnoreDisabledTargetSpaces(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	repo := NewExamRepository(gormDB, ExamRepositoryOptions{Now: func() int64 { return 1000 }})
	seedMultiSpaceAttemptData(t, gormDB)

	candidates, err := repo.CountExamCandidates(t.Context(), 10, 900, []uint64{302})
	if err != nil {
		t.Fatalf("CountExamCandidates returned error: %v", err)
	}
	if candidates != 1 {
		t.Fatalf("expected one candidate before disabling space, got %d", candidates)
	}

	stats, err := repo.CountExamAttemptStats(t.Context(), 10, 900, []uint64{302})
	if err != nil {
		t.Fatalf("CountExamAttemptStats returned error: %v", err)
	}
	if stats.Joined != 1 || stats.Submitted != 1 || stats.InProgress != 0 {
		t.Fatalf("expected submitted stats before disabling space, got %#v", stats)
	}

	if err := gormDB.Exec(`
		UPDATE spaces
		SET status = 'disabled'
		WHERE tenant_id = 10 AND id = 302
	`).Error; err != nil {
		t.Fatalf("disable target space: %v", err)
	}

	candidates, err = repo.CountExamCandidates(t.Context(), 10, 900, []uint64{302})
	if err != nil {
		t.Fatalf("CountExamCandidates after disabling space returned error: %v", err)
	}
	if candidates != 0 {
		t.Fatalf("disabled target space should not contribute candidates, got %d", candidates)
	}

	stats, err = repo.CountExamAttemptStats(t.Context(), 10, 900, []uint64{302})
	if err != nil {
		t.Fatalf("CountExamAttemptStats after disabling space returned error: %v", err)
	}
	if stats.Joined != 0 || stats.Submitted != 0 || stats.InProgress != 0 {
		t.Fatalf("disabled target space should not contribute attempt stats, got %#v", stats)
	}
}

func TestExamRepositoryCountExamCandidatesRequiresStudentSpaceRole(t *testing.T) {
	t.Run("space target", func(t *testing.T) {
		gormDB := openExamRepositoryTestDB(t)
		repo := NewExamRepository(gormDB, ExamRepositoryOptions{Now: func() int64 { return 1000 }})
		seedMultiSpaceAttemptData(t, gormDB)
		if err := gormDB.Exec(`
			UPDATE space_members
			SET role_in_space = 'teacher'
			WHERE tenant_id = 10 AND user_id = 21 AND space_id = 302
		`).Error; err != nil {
			t.Fatalf("switch target space role: %v", err)
		}

		candidates, err := repo.CountExamCandidates(t.Context(), 10, 900, []uint64{302})
		if err != nil {
			t.Fatalf("CountExamCandidates returned error: %v", err)
		}
		if candidates != 0 {
			t.Fatalf("non-student space role should not contribute candidates, got %d", candidates)
		}
		stats, err := repo.CountExamAttemptStats(t.Context(), 10, 900, []uint64{302})
		if err != nil {
			t.Fatalf("CountExamAttemptStats returned error: %v", err)
		}
		if stats.Joined != 0 || stats.Submitted != 0 || stats.InProgress != 0 {
			t.Fatalf("non-student space role should not contribute attempt stats, got %#v", stats)
		}
	})

	t.Run("user target", func(t *testing.T) {
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
		if err := gormDB.Exec(`
			UPDATE space_members
			SET role_in_space = 'teacher'
			WHERE tenant_id = 10 AND user_id = 21 AND space_id = 302
		`).Error; err != nil {
			t.Fatalf("switch target space role: %v", err)
		}

		candidates, err := repo.CountExamCandidates(t.Context(), 10, 900, []uint64{302})
		if err != nil {
			t.Fatalf("CountExamCandidates returned error: %v", err)
		}
		if candidates != 0 {
			t.Fatalf("non-student scoped user role should not contribute candidates, got %d", candidates)
		}
		stats, err := repo.CountExamAttemptStats(t.Context(), 10, 900, []uint64{302})
		if err != nil {
			t.Fatalf("CountExamAttemptStats returned error: %v", err)
		}
		if stats.Joined != 0 || stats.Submitted != 0 || stats.InProgress != 0 {
			t.Fatalf("non-student scoped user role should not contribute attempt stats, got %#v", stats)
		}
	})
}

func TestExamRepositoryCountExamAttemptStatsUsesSingleStatusPerCandidate(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	repo := NewExamRepository(gormDB, ExamRepositoryOptions{Now: func() int64 { return 1000 }})
	seedMultiSpaceAttemptData(t, gormDB)

	if err := gormDB.Exec(`
		INSERT INTO exam_attempts (
			id, tenant_id, exam_id, user_id, attempt_no, status, started_at, submitted_at,
			exam_token_hash, exam_token_expires_at, objective_score, subjective_score, total_score,
			created_at, updated_at, ext_json
		) VALUES (701, 10, 900, 21, 2, 'in_progress', 1600, NULL, 'token-hash-2', 3000, 0, 0, 0, 1600, 1600, '{}')
	`).Error; err != nil {
		t.Fatalf("seed in-progress retake: %v", err)
	}

	stats, err := repo.CountExamAttemptStats(t.Context(), 10, 900, []uint64{302})
	if err != nil {
		t.Fatalf("CountExamAttemptStats returned error: %v", err)
	}
	if stats.Joined != 1 || stats.Submitted != 1 || stats.InProgress != 0 {
		t.Fatalf("expected retake candidate to stay submitted-only, got %#v", stats)
	}
}

func TestExamRepositoryListExamResultsKeepsHistoricalSingleTargetSpaceAfterMembershipRemoved(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	repo := NewExamRepository(gormDB, ExamRepositoryOptions{Now: func() int64 { return 1000 }})
	seedMultiSpaceAttemptData(t, gormDB)

	if err := gormDB.Exec(`
		DELETE FROM space_members
		WHERE tenant_id = 10 AND user_id = 21 AND space_id = 302
	`).Error; err != nil {
		t.Fatalf("delete target space membership: %v", err)
	}

	results, err := repo.ListExamResults(t.Context(), serviceexam.ListExamResultsInput{
		TenantID: 10,
		ExamID:   900,
		SpaceIDs: []uint64{302},
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("ListExamResults returned error: %v", err)
	}
	if results.Total != 1 || len(results.Items) != 1 {
		t.Fatalf("expected one historical result row, got %#v", results)
	}
	if results.Items[0].SpaceID != 302 || results.Items[0].SpaceName != "语文培优班" {
		t.Fatalf("expected historical result to keep target space 302, got %#v", results.Items[0])
	}
}

func TestExamRepositoryListExamResultsDoesNotFallbackMultiSpaceExamToRequestedSpace(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	repo := NewExamRepository(gormDB, ExamRepositoryOptions{Now: func() int64 { return 1000 }})
	seedMultiSpaceAttemptData(t, gormDB)

	if err := gormDB.Exec(`
		INSERT INTO exam_targets (
			id, tenant_id, exam_id, target_type, target_id, created_at, ext_json
		) VALUES (2, 10, 900, 'space', 301, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed second space target: %v", err)
	}
	if err := gormDB.Exec(`
		DELETE FROM space_members
		WHERE tenant_id = 10 AND user_id = 21
	`).Error; err != nil {
		t.Fatalf("delete historical memberships: %v", err)
	}

	results, err := repo.ListExamResults(t.Context(), serviceexam.ListExamResultsInput{
		TenantID: 10,
		ExamID:   900,
		SpaceIDs: []uint64{302},
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("ListExamResults returned error: %v", err)
	}
	if results.Total != 0 || len(results.Items) != 0 {
		t.Fatalf("multi-space historical fallback should not leak into requested space 302, got %#v", results)
	}
}

func TestExamRepositoryListExamResultsDisplaysAuthorizedTargetSpace(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	repo := NewExamRepository(gormDB, ExamRepositoryOptions{Now: func() int64 { return 1000 }})
	seedMultiSpaceAttemptData(t, gormDB)

	if err := gormDB.Exec(`
		INSERT INTO exam_targets (
			id, tenant_id, exam_id, target_type, target_id, created_at, ext_json
		) VALUES (2, 10, 900, 'space', 301, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed second space target: %v", err)
	}

	results, err := repo.ListExamResults(t.Context(), serviceexam.ListExamResultsInput{
		TenantID: 10,
		ExamID:   900,
		SpaceIDs: []uint64{302},
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("ListExamResults returned error: %v", err)
	}
	if results.Total != 1 || len(results.Items) != 1 {
		t.Fatalf("expected one authorized result row, got %#v", results)
	}
	if results.Items[0].SpaceID != 302 || results.Items[0].SpaceName != "语文培优班" {
		t.Fatalf("expected result display space to stay in authorized space 302, got %#v", results.Items[0])
	}
}

func TestExamRepositoryListExamResultsDoesNotFallbackMixedTargetsToSingleSpace(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	repo := NewExamRepository(gormDB, ExamRepositoryOptions{Now: func() int64 { return 1000 }})
	seedMultiSpaceAttemptData(t, gormDB)

	if err := gormDB.Exec(`
		INSERT INTO users (
			id, username, real_name, phone, email, password_hash, status,
			created_at, updated_at, ext_json
		) VALUES (22, 'student22', '用户直投考生', '13800000022', 'student22@example.test', 'hash', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed mixed target user: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO tenant_user_memberships (
			id, tenant_id, user_id, role, status, created_at, updated_at, ext_json
		) VALUES (22, 10, 22, 'student', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed mixed target tenant membership: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO space_members (
			id, tenant_id, space_id, user_id, role_in_space, status, created_at, updated_at, ext_json
		) VALUES (22, 10, 301, 22, 'student', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed mixed target membership: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO exam_targets (
			id, tenant_id, exam_id, target_type, target_id, created_at, ext_json
		) VALUES (2, 10, 900, 'user', 22, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed mixed user target: %v", err)
	}
	seedExamTargetScopeSpaces(t, gormDB, 10, 900, 2, 301)
	if err := gormDB.Exec(`
		INSERT INTO exam_attempts (
			id, tenant_id, exam_id, user_id, attempt_no, status, started_at, submitted_at,
			exam_token_hash, exam_token_expires_at, objective_score, subjective_score, total_score,
			created_at, updated_at, ext_json
		) VALUES (701, 10, 900, 22, 1, 'submitted', 1000, 1600, 'token-hash-701', 3000, 5, 0, 5, 1000, 1600, '{}')
	`).Error; err != nil {
		t.Fatalf("seed mixed target attempt: %v", err)
	}
	if err := gormDB.Exec(`
		DELETE FROM space_members
		WHERE tenant_id = 10 AND user_id = 22 AND space_id = 301
	`).Error; err != nil {
		t.Fatalf("delete mixed target membership: %v", err)
	}

	results, err := repo.ListExamResults(t.Context(), serviceexam.ListExamResultsInput{
		TenantID: 10,
		ExamID:   900,
		SpaceIDs: []uint64{302},
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("ListExamResults returned error: %v", err)
	}
	if results.Total != 1 || len(results.Items) != 1 {
		t.Fatalf("mixed targets should not leak user-target history into space 302, got %#v", results)
	}
	if results.Items[0].UserID != 21 {
		t.Fatalf("expected only original space-target candidate to remain visible, got %#v", results.Items)
	}
}

func TestExamRepositoryAttemptSpaceIDsDoesNotFallbackMixedTargetsToSingleSpace(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	repo := NewExamRepository(gormDB, ExamRepositoryOptions{Now: func() int64 { return 1000 }})
	seedMultiSpaceAttemptData(t, gormDB)

	if err := gormDB.Exec(`
		INSERT INTO users (
			id, username, real_name, phone, email, password_hash, status,
			created_at, updated_at, ext_json
		) VALUES (22, 'student22', '用户直投考生', '13800000022', 'student22@example.test', 'hash', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed mixed target user: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO tenant_user_memberships (
			id, tenant_id, user_id, role, status, created_at, updated_at, ext_json
		) VALUES (22, 10, 22, 'student', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed mixed target tenant membership: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO space_members (
			id, tenant_id, space_id, user_id, role_in_space, status, created_at, updated_at, ext_json
		) VALUES (22, 10, 301, 22, 'student', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed mixed target membership: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO exam_targets (
			id, tenant_id, exam_id, target_type, target_id, created_at, ext_json
		) VALUES (2, 10, 900, 'user', 22, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed mixed user target: %v", err)
	}
	seedExamTargetScopeSpaces(t, gormDB, 10, 900, 2, 301)
	if err := gormDB.Exec(`
		INSERT INTO exam_attempts (
			id, tenant_id, exam_id, user_id, attempt_no, status, started_at, submitted_at,
			exam_token_hash, exam_token_expires_at, objective_score, subjective_score, total_score,
			created_at, updated_at, ext_json
		) VALUES (701, 10, 900, 22, 1, 'submitted', 1000, 1600, 'token-hash-701', 3000, 5, 0, 5, 1000, 1600, '{}')
	`).Error; err != nil {
		t.Fatalf("seed mixed target attempt: %v", err)
	}
	if err := gormDB.Exec(`
		DELETE FROM space_members
		WHERE tenant_id = 10 AND user_id = 22 AND space_id = 301
	`).Error; err != nil {
		t.Fatalf("delete mixed target membership: %v", err)
	}

	spaces, err := repo.AttemptSpaceIDs(t.Context(), 10, 701)
	if err != nil {
		t.Fatalf("AttemptSpaceIDs returned error: %v", err)
	}
	if len(spaces) != 0 {
		t.Fatalf("mixed targets should not fallback user-target history to space target, got %#v", spaces)
	}
}

func TestExamRepositoryListExamResultsSearchesSpaceName(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	repo := NewExamRepository(gormDB, ExamRepositoryOptions{Now: func() int64 { return 1000 }})
	seedMultiSpaceAttemptData(t, gormDB)

	results, err := repo.ListExamResults(t.Context(), serviceexam.ListExamResultsInput{
		TenantID: 10,
		ExamID:   900,
		SpaceIDs: []uint64{302},
		Keyword:  "语文培优班",
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("ListExamResults returned error: %v", err)
	}
	if results.Total != 1 || len(results.Items) != 1 {
		t.Fatalf("expected space name keyword to match one result row, got %#v", results)
	}
}

func TestExamRepositoryListExamResultsSearchesSpaceNameOnlyInAuthorizedScope(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	repo := NewExamRepository(gormDB, ExamRepositoryOptions{Now: func() int64 { return 1000 }})
	seedMultiSpaceAttemptData(t, gormDB)
	if err := gormDB.Exec(`
		INSERT INTO exam_targets (
			id, tenant_id, exam_id, target_type, target_id, created_at, ext_json
		) VALUES (2, 10, 900, 'space', 301, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed unauthorized space target: %v", err)
	}

	results, err := repo.ListExamResults(t.Context(), serviceexam.ListExamResultsInput{
		TenantID: 10,
		ExamID:   900,
		SpaceIDs: []uint64{302},
		Keyword:  "高一一班",
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("ListExamResults returned error: %v", err)
	}
	if results.Total != 0 || len(results.Items) != 0 {
		t.Fatalf("expected unauthorized space name keyword to stay out of scoped results, got %#v", results)
	}
}

func TestExamRepositoryListExamCandidatesDedupesMixedTargets(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	repo := NewExamRepository(gormDB, ExamRepositoryOptions{Now: func() int64 { return 1000 }})
	seedMultiSpaceAttemptData(t, gormDB)

	if err := gormDB.Exec(`
		INSERT INTO exam_targets (
			id, tenant_id, exam_id, target_type, target_id, created_at, ext_json
		) VALUES (2, 10, 900, 'user', 21, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed direct user target: %v", err)
	}

	result, err := repo.ListExamCandidates(t.Context(), serviceexam.ListExamCandidatesInput{
		TenantID:       10,
		ExamID:         900,
		SpaceIDs:       []uint64{302},
		ResultStrategy: serviceexam.ResultStrategyLatest,
		Page:           1,
		PageSize:       20,
	})
	if err != nil {
		t.Fatalf("ListExamCandidates returned error: %v", err)
	}
	if result.Total != 1 || len(result.Items) != 1 {
		t.Fatalf("expected one deduped candidate, got total=%d items=%#v", result.Total, result.Items)
	}
	item := result.Items[0]
	if item.UserID != 21 || item.RealName != "多空间考生" || item.Status != serviceexam.CandidateStatusSubmitted {
		t.Fatalf("unexpected candidate identity or status: %#v", item)
	}
	if item.AttemptCount != 1 || item.ResultAttemptID == nil || *item.ResultAttemptID != 700 || item.CurrentAttemptID != nil {
		t.Fatalf("unexpected attempt summary: %#v", item)
	}
	if len(item.SourceTargets) != 2 {
		t.Fatalf("expected space and direct user source targets, got %#v", item.SourceTargets)
	}
}

func TestExamRepositoryListExamCandidatesIgnoresDisabledUsers(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	repo := NewExamRepository(gormDB, ExamRepositoryOptions{Now: func() int64 { return 1000 }})
	seedMultiSpaceAttemptData(t, gormDB)

	if err := gormDB.Exec(`
		UPDATE users
		SET status = 'disabled'
		WHERE id = 21
	`).Error; err != nil {
		t.Fatalf("disable candidate user: %v", err)
	}

	result, err := repo.ListExamCandidates(t.Context(), serviceexam.ListExamCandidatesInput{
		TenantID:       10,
		ExamID:         900,
		SpaceIDs:       []uint64{302},
		ResultStrategy: serviceexam.ResultStrategyLatest,
		Page:           1,
		PageSize:       20,
	})
	if err != nil {
		t.Fatalf("ListExamCandidates after disabling user returned error: %v", err)
	}
	if result.Total != 0 || len(result.Items) != 0 {
		t.Fatalf("disabled candidate should be hidden, got total=%d items=%#v", result.Total, result.Items)
	}
}

func TestExamRepositoryListExamCandidatesReturnsNotStartedInProgressAndSubmittedStatuses(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	repo := NewExamRepository(gormDB, ExamRepositoryOptions{Now: func() int64 { return 1000 }})
	seedMultiSpaceAttemptData(t, gormDB)

	if err := gormDB.Exec(`
		INSERT INTO users (
			id, username, real_name, phone, email, password_hash, status,
			created_at, updated_at, ext_json
		) VALUES
			(22, 'student22', '未开始考生', '13800000022', 'student22@example.test', 'hash', 'enabled', 1000, 1000, '{}'),
			(23, 'student23', '进行中考生', '13800000023', 'student23@example.test', 'hash', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed status users: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO tenant_user_memberships (
			id, tenant_id, user_id, role, status, created_at, updated_at, ext_json
		) VALUES
			(22, 10, 22, 'student', 'enabled', 1000, 1000, '{}'),
			(23, 10, 23, 'student', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed status tenant memberships: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO space_members (
			id, tenant_id, space_id, user_id, role_in_space, status, created_at, updated_at, ext_json
		) VALUES
			(3, 10, 302, 22, 'student', 'enabled', 1000, 1000, '{}'),
			(4, 10, 302, 23, 'student', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed status space memberships: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO exam_attempts (
			id, tenant_id, exam_id, user_id, attempt_no, status, started_at, submitted_at,
			exam_token_hash, exam_token_expires_at, objective_score, subjective_score, total_score,
			created_at, updated_at, ext_json
		) VALUES (701, 10, 900, 23, 1, 'in_progress', 1200, NULL, 'token-hash-progress', 3000, 0, 0, 0, 1200, 1200, '{}')
	`).Error; err != nil {
		t.Fatalf("seed in progress attempt: %v", err)
	}

	result, err := repo.ListExamCandidates(t.Context(), serviceexam.ListExamCandidatesInput{
		TenantID:       10,
		ExamID:         900,
		SpaceIDs:       []uint64{302},
		ResultStrategy: serviceexam.ResultStrategyLatest,
		Page:           1,
		PageSize:       20,
	})
	if err != nil {
		t.Fatalf("ListExamCandidates returned error: %v", err)
	}
	statusByUser := make(map[uint64]string, len(result.Items))
	for _, item := range result.Items {
		statusByUser[item.UserID] = item.Status
	}
	expected := map[uint64]string{
		21: serviceexam.CandidateStatusSubmitted,
		22: serviceexam.CandidateStatusNotStarted,
		23: serviceexam.CandidateStatusInProgress,
	}
	if !reflect.DeepEqual(statusByUser, expected) {
		t.Fatalf("unexpected candidate statuses: got %#v want %#v", statusByUser, expected)
	}
}

func TestExamRepositoryListExamCandidatesFiltersStatusBeforePagination(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	repo := NewExamRepository(gormDB, ExamRepositoryOptions{Now: func() int64 { return 1000 }})
	seedMultiSpaceAttemptData(t, gormDB)

	if err := gormDB.Exec(`
		INSERT INTO users (
			id, username, real_name, phone, email, password_hash, status,
			created_at, updated_at, ext_json
		) VALUES (22, 'student22', '未开始考生', '13800000022', 'student22@example.test', 'hash', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed not started user: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO tenant_user_memberships (
			id, tenant_id, user_id, role, status, created_at, updated_at, ext_json
		) VALUES (22, 10, 22, 'student', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed not started tenant membership: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO space_members (
			id, tenant_id, space_id, user_id, role_in_space, status, created_at, updated_at, ext_json
		) VALUES (3, 10, 302, 22, 'student', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed not started space membership: %v", err)
	}

	result, err := repo.ListExamCandidates(t.Context(), serviceexam.ListExamCandidatesInput{
		TenantID:       10,
		ExamID:         900,
		SpaceIDs:       []uint64{302},
		Status:         serviceexam.CandidateStatusNotStarted,
		ResultStrategy: serviceexam.ResultStrategyLatest,
		Page:           1,
		PageSize:       1,
	})
	if err != nil {
		t.Fatalf("ListExamCandidates status filter returned error: %v", err)
	}
	if result.Total != 1 || len(result.Items) != 1 || result.Items[0].UserID != 22 {
		t.Fatalf("status filter must apply before pagination, got total=%d items=%#v", result.Total, result.Items)
	}
}

func TestExamRepositoryListExamCandidatesSelectsHighestResultAttempt(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	repo := NewExamRepository(gormDB, ExamRepositoryOptions{Now: func() int64 { return 1000 }})
	seedMultiSpaceAttemptData(t, gormDB)

	if err := gormDB.Exec(`
		INSERT INTO exam_attempts (
			id, tenant_id, exam_id, user_id, attempt_no, status, started_at, submitted_at,
			exam_token_hash, exam_token_expires_at, objective_score, subjective_score, total_score,
			created_at, updated_at, ext_json
		) VALUES (701, 10, 900, 21, 2, 'submitted', 1100, 1400, 'token-hash-2', 3000, 8, 0, 8, 1100, 1400, '{}')
	`).Error; err != nil {
		t.Fatalf("seed higher score attempt: %v", err)
	}

	result, err := repo.ListExamCandidates(t.Context(), serviceexam.ListExamCandidatesInput{
		TenantID:       10,
		ExamID:         900,
		SpaceIDs:       []uint64{302},
		ResultStrategy: serviceexam.ResultStrategyHighest,
		Page:           1,
		PageSize:       20,
	})
	if err != nil {
		t.Fatalf("ListExamCandidates highest strategy returned error: %v", err)
	}
	if len(result.Items) != 1 || result.Items[0].ResultAttemptID == nil || *result.Items[0].ResultAttemptID != 701 {
		t.Fatalf("highest strategy should choose attempt 701, got %#v", result.Items)
	}
}

func TestExamRepositoryImportCandidateTargetsFiltersAndWritesLog(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	repo := NewExamRepository(gormDB, ExamRepositoryOptions{Now: func() int64 { return 1800 }})
	seedMultiSpaceAttemptData(t, gormDB)

	if err := gormDB.Exec(`
		INSERT INTO users (
			id, username, real_name, phone, email, password_hash, status,
			created_at, updated_at, ext_json
		) VALUES
			(22, 'student22', '可导入考生', '13800000022', 'student22@example.test', 'hash', 'enabled', 1000, 1000, '{}'),
			(23, 'student23', '非授权空间考生', '13800000023', 'student23@example.test', 'hash', 'enabled', 1000, 1000, '{}'),
			(24, 'student24', '禁用考生', '13800000024', 'student24@example.test', 'hash', 'disabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed import users: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO tenant_user_memberships (
			id, tenant_id, user_id, role, status, created_at, updated_at, ext_json
		) VALUES
			(22, 10, 22, 'student', 'enabled', 1000, 1000, '{}'),
			(23, 10, 23, 'student', 'enabled', 1000, 1000, '{}'),
			(24, 10, 24, 'student', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed import tenant memberships: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO space_members (
			id, tenant_id, space_id, user_id, role_in_space, status, created_at, updated_at, ext_json
		) VALUES
			(3, 10, 302, 22, 'student', 'enabled', 1000, 1000, '{}'),
			(4, 10, 301, 23, 'student', 'enabled', 1000, 1000, '{}'),
			(5, 10, 302, 24, 'student', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed import space memberships: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO exam_targets (
			id, tenant_id, exam_id, target_type, target_id, created_at, ext_json
		) VALUES (2, 10, 900, 'user', 21, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed existing user target: %v", err)
	}

	result, err := repo.ImportCandidateTargets(t.Context(), serviceexam.ImportCandidateTargetsInput{
		TenantID:        10,
		ExamID:          900,
		UserIDs:         []uint64{21, 22, 23, 24},
		AllowedSpaceIDs: []uint64{301, 302},
		Log: serviceexam.OperationLog{
			TenantID:        10,
			ExamID:          900,
			OperationType:   serviceexam.OperationTypeImportCandidates,
			OperationTitle:  "导入考生",
			OperationDetail: "导入 4 名考生",
			ActorID:         11,
			ActorType:       AuditActorTenantUser,
			ActorRole:       "tenant_admin",
		},
	})
	if err != nil {
		t.Fatalf("ImportCandidateTargets returned error: %v", err)
	}
	if result.ImportedCount != 2 || result.SkippedCount != 2 {
		t.Fatalf("expected two imported and two skipped, got %#v", result)
	}
	if len(result.ImportedTargets) != 2 || result.ImportedTargets[0].TargetID != 22 || result.ImportedTargets[1].TargetID != 23 {
		t.Fatalf("expected users 22 and 23 imported, got %#v", result.ImportedTargets)
	}
	var targets []struct {
		TargetType string
		TargetID   uint64
	}
	if err := gormDB.Table("exam_targets").
		Select("target_type, target_id").
		Where("tenant_id = ? AND exam_id = ? AND target_type = ?", 10, 900, serviceexam.TargetTypeUser).
		Order("target_id ASC").
		Scan(&targets).Error; err != nil {
		t.Fatalf("query imported targets: %v", err)
	}
	if len(targets) != 3 || targets[0].TargetID != 21 || targets[1].TargetID != 22 || targets[2].TargetID != 23 {
		t.Fatalf("expected existing user 21 and imported users 22/23 only, got %#v", targets)
	}
	logs, err := repo.ListOperationLogs(t.Context(), serviceexam.ListOperationLogsInput{
		TenantID:      10,
		ExamID:        900,
		OperationType: serviceexam.OperationTypeImportCandidates,
		Page:          1,
		PageSize:      10,
	})
	if err != nil {
		t.Fatalf("ListOperationLogs returned error: %v", err)
	}
	if logs.Total != 2 || logs.Items[0].OperationDetail != "导入 4 名考生" || logs.Items[1].OperationDetail != "导入 4 名考生" {
		t.Fatalf("expected import operation log, got %#v", logs)
	}
	assertOperationLogsShareGroupAndSpaces(t, logs.Items, []uint64{301, 302})
}

func TestExamRepositoryImportCandidateTargetsSkipsLogWhenNothingImported(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	repo := NewExamRepository(gormDB, ExamRepositoryOptions{Now: func() int64 { return 1800 }})
	seedMultiSpaceAttemptData(t, gormDB)

	result, err := repo.ImportCandidateTargets(t.Context(), serviceexam.ImportCandidateTargetsInput{
		TenantID:        10,
		ExamID:          900,
		UserIDs:         []uint64{999},
		AllowedSpaceIDs: []uint64{302},
		Log: serviceexam.OperationLog{
			TenantID:        10,
			ExamID:          900,
			OperationType:   serviceexam.OperationTypeImportCandidates,
			OperationTitle:  "导入考生",
			OperationDetail: "导入 1 名考生",
			ActorID:         11,
			ActorType:       AuditActorTenantUser,
			ActorRole:       "space_admin",
		},
	})
	if err != nil {
		t.Fatalf("ImportCandidateTargets returned error: %v", err)
	}
	if result.ImportedCount != 0 || result.SkippedCount != 1 {
		t.Fatalf("expected one skipped and no imported target, got %#v", result)
	}
	assertOperationLogCount(t, gormDB, 10, 900, serviceexam.OperationTypeImportCandidates, 0)
}

func TestExamRepositoryResendInvitationsFiltersCandidatesAndWritesLog(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	repo := NewExamRepository(gormDB, ExamRepositoryOptions{Now: func() int64 { return 1900 }})
	seedMultiSpaceAttemptData(t, gormDB)

	if err := gormDB.Exec(`
		INSERT INTO users (
			id, username, real_name, phone, email, password_hash, status,
			created_at, updated_at, ext_json
		) VALUES
			(22, 'student22', '授权应考生', '13800000022', 'student22@example.test', 'hash', 'enabled', 1000, 1000, '{}'),
			(23, 'student23', '非授权空间考生', '13800000023', 'student23@example.test', 'hash', 'enabled', 1000, 1000, '{}'),
			(24, 'student24', '禁用应考生', '13800000024', 'student24@example.test', 'hash', 'disabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed resend users: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO tenant_user_memberships (
			id, tenant_id, user_id, role, status, created_at, updated_at, ext_json
		) VALUES
			(22, 10, 22, 'student', 'enabled', 1000, 1000, '{}'),
			(23, 10, 23, 'student', 'enabled', 1000, 1000, '{}'),
			(24, 10, 24, 'student', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed resend tenant memberships: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO space_members (
			id, tenant_id, space_id, user_id, role_in_space, status, created_at, updated_at, ext_json
		) VALUES
			(3, 10, 302, 22, 'student', 'enabled', 1000, 1000, '{}'),
			(4, 10, 301, 23, 'student', 'enabled', 1000, 1000, '{}'),
			(5, 10, 302, 24, 'student', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed resend space memberships: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO exam_targets (
			id, tenant_id, exam_id, target_type, target_id, created_at, ext_json
		) VALUES (3, 10, 900, 'space', 301, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed resend second exam target: %v", err)
	}

	result, err := repo.ResendInvitations(t.Context(), serviceexam.ResendInvitationsRepositoryInput{
		TenantID:        10,
		ExamID:          900,
		UserIDs:         []uint64{21, 22, 23, 24},
		AllowedSpaceIDs: []uint64{301, 302},
		Log: serviceexam.OperationLog{
			TenantID:        10,
			ExamID:          900,
			OperationType:   serviceexam.OperationTypeSendInvite,
			OperationTitle:  "重发邀请码",
			OperationDetail: "重发 4 名考生邀请码",
			ActorID:         11,
			ActorType:       AuditActorTenantUser,
			ActorRole:       "tenant_admin",
		},
	})
	if err != nil {
		t.Fatalf("ResendInvitations returned error: %v", err)
	}
	if result.SentCount != 3 || result.SkippedCount != 1 {
		t.Fatalf("expected three sent and one skipped, got %#v", result)
	}
	assertUint64s(t, result.SentUserIDs, []uint64{21, 22, 23})
	logs, err := repo.ListOperationLogs(t.Context(), serviceexam.ListOperationLogsInput{
		TenantID:      10,
		ExamID:        900,
		OperationType: serviceexam.OperationTypeSendInvite,
		Page:          1,
		PageSize:      10,
	})
	if err != nil {
		t.Fatalf("ListOperationLogs returned error: %v", err)
	}
	if logs.Total != 2 || logs.Items[0].OperationDetail != "重发 4 名考生邀请码" || logs.Items[1].OperationDetail != "重发 4 名考生邀请码" {
		t.Fatalf("expected send_invite operation log, got %#v", logs)
	}
	assertOperationLogsShareGroupAndSpaces(t, logs.Items, []uint64{301, 302})
}

func TestExamRepositoryResendInvitationsSkipsLogWhenNothingSent(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	repo := NewExamRepository(gormDB, ExamRepositoryOptions{Now: func() int64 { return 1900 }})
	seedMultiSpaceAttemptData(t, gormDB)

	result, err := repo.ResendInvitations(t.Context(), serviceexam.ResendInvitationsRepositoryInput{
		TenantID:        10,
		ExamID:          900,
		UserIDs:         []uint64{999},
		AllowedSpaceIDs: []uint64{302},
		Log: serviceexam.OperationLog{
			TenantID:        10,
			ExamID:          900,
			OperationType:   serviceexam.OperationTypeSendInvite,
			OperationTitle:  "重发邀请码",
			OperationDetail: "重发 1 名考生邀请码",
			ActorID:         11,
			ActorType:       AuditActorTenantUser,
			ActorRole:       "space_admin",
		},
	})
	if err != nil {
		t.Fatalf("ResendInvitations returned error: %v", err)
	}
	if result.SentCount != 0 || result.SkippedCount != 1 {
		t.Fatalf("expected one skipped and no sent target, got %#v", result)
	}
	assertOperationLogCount(t, gormDB, 10, 900, serviceexam.OperationTypeSendInvite, 0)
}

func TestExamRepositoryIsEligibleRejectsUserTargetWithoutActiveStudentSpace(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	repo := NewExamRepository(gormDB, ExamRepositoryOptions{Now: func() int64 { return 1000 }})
	seedMultiSpaceAttemptData(t, gormDB)

	if err := gormDB.Exec(`
		UPDATE exam_targets
		SET target_type = 'user', target_id = 21
		WHERE tenant_id = 10 AND exam_id = 900
	`).Error; err != nil {
		t.Fatalf("switch exam target to user: %v", err)
	}
	if err := gormDB.Exec(`
		DELETE FROM space_members
		WHERE tenant_id = 10 AND user_id = 21
	`).Error; err != nil {
		t.Fatalf("delete student space memberships: %v", err)
	}

	eligible, err := repo.IsEligible(t.Context(), 10, 900, 21)
	if err != nil {
		t.Fatalf("IsEligible returned error: %v", err)
	}
	if eligible {
		t.Fatalf("expected direct user target without active student space membership to be ineligible")
	}
}

func TestExamRepositoryUpdateManagementSettingsWritesScopedLogs(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	repo := NewExamRepository(gormDB, ExamRepositoryOptions{Now: func() int64 { return 2100 }})
	seedMultiSpaceAttemptData(t, gormDB)

	updated, err := repo.UpdateManagementSettings(t.Context(), serviceexam.UpdateManagementSettingsRepositoryInput{
		TenantID:         10,
		ExamID:           900,
		PublishMode:      serviceexam.PublishModeImmediateScore,
		ScorePublishTime: nil,
		Log: serviceexam.OperationLog{
			TenantID:        10,
			ExamID:          900,
			OperationType:   serviceexam.OperationTypeUpdateSettings,
			OperationTitle:  "修改考试设置",
			OperationDetail: "修改成绩发布配置",
			ActorID:         11,
			ActorType:       AuditActorTenantUser,
			ActorRole:       "space_admin",
		},
	})
	if err != nil {
		t.Fatalf("UpdateManagementSettings returned error: %v", err)
	}
	if updated.PublishMode != serviceexam.PublishModeImmediateScore {
		t.Fatalf("expected publish mode to be updated, got %#v", updated)
	}

	spaceID := uint64(302)
	logs, err := repo.ListOperationLogs(t.Context(), serviceexam.ListOperationLogsInput{
		TenantID:      10,
		ExamID:        900,
		SpaceID:       &spaceID,
		OperationType: serviceexam.OperationTypeUpdateSettings,
		Page:          1,
		PageSize:      10,
	})
	if err != nil {
		t.Fatalf("ListOperationLogs returned error: %v", err)
	}
	if logs.Total != 1 || len(logs.Items) != 1 {
		t.Fatalf("expected one scoped settings log, got %#v", logs)
	}
	if logs.Items[0].SpaceID == nil || *logs.Items[0].SpaceID != 302 {
		t.Fatalf("expected settings log scoped to target space 302, got %#v", logs.Items[0])
	}
}

func TestExamRepositoryUpdateManagementSettingsPreservesScopedUserTargetLogs(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	repo := NewExamRepository(gormDB, ExamRepositoryOptions{Now: func() int64 { return 2100 }})
	seedMultiSpaceAttemptData(t, gormDB)

	var targetID uint64
	if err := gormDB.Table("exam_targets").
		Where("tenant_id = ? AND exam_id = ?", 10, 900).
		Updates(map[string]any{
			"target_type": serviceexam.TargetTypeUser,
			"target_id":   21,
			"ext_json":    "{}",
		}).Error; err != nil {
		t.Fatalf("scope settings target to one user: %v", err)
	}
	if err := gormDB.Table("exam_targets").
		Select("id").
		Where("tenant_id = ? AND exam_id = ? AND target_type = ? AND target_id = ?", 10, 900, serviceexam.TargetTypeUser, 21).
		Scan(&targetID).Error; err != nil {
		t.Fatalf("query scoped target id: %v", err)
	}
	if err := gormDB.Table("exam_target_scope_spaces").Create(map[string]any{
		"tenant_id":      10,
		"exam_id":        900,
		"exam_target_id": targetID,
		"space_id":       301,
		"created_at":     int64(2000),
		"ext_json":       "{}",
	}).Error; err != nil {
		t.Fatalf("insert target scope space: %v", err)
	}

	updated, err := repo.UpdateManagementSettings(t.Context(), serviceexam.UpdateManagementSettingsRepositoryInput{
		TenantID:         10,
		ExamID:           900,
		PublishMode:      serviceexam.PublishModeImmediateScore,
		ScorePublishTime: nil,
		Log: serviceexam.OperationLog{
			TenantID:        10,
			ExamID:          900,
			OperationType:   serviceexam.OperationTypeUpdateSettings,
			OperationTitle:  "修改考试设置",
			OperationDetail: "修改成绩发布配置",
			ActorID:         11,
			ActorType:       AuditActorTenantUser,
			ActorRole:       "space_admin",
		},
	})
	if err != nil {
		t.Fatalf("UpdateManagementSettings returned error: %v", err)
	}
	if updated.PublishMode != serviceexam.PublishModeImmediateScore {
		t.Fatalf("expected publish mode to be updated, got %#v", updated)
	}

	space301 := uint64(301)
	logs301, err := repo.ListOperationLogs(t.Context(), serviceexam.ListOperationLogsInput{
		TenantID:      10,
		ExamID:        900,
		SpaceID:       &space301,
		OperationType: serviceexam.OperationTypeUpdateSettings,
		Page:          1,
		PageSize:      10,
	})
	if err != nil {
		t.Fatalf("ListOperationLogs for scoped space returned error: %v", err)
	}
	if logs301.Total != 1 || len(logs301.Items) != 1 {
		t.Fatalf("expected one scoped settings log in space 301, got %#v", logs301)
	}

	space302 := uint64(302)
	logs302, err := repo.ListOperationLogs(t.Context(), serviceexam.ListOperationLogsInput{
		TenantID:      10,
		ExamID:        900,
		SpaceID:       &space302,
		OperationType: serviceexam.OperationTypeUpdateSettings,
		Page:          1,
		PageSize:      10,
	})
	if err != nil {
		t.Fatalf("ListOperationLogs for unrelated space returned error: %v", err)
	}
	if logs302.Total != 0 || len(logs302.Items) != 0 {
		t.Fatalf("expected scoped user target settings log to stay out of space 302, got %#v", logs302)
	}
}

func TestExamRepositoryUpdateScorePublishConfigRejectsMissingExamBeforeWritingLog(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	repo := NewExamRepository(gormDB, ExamRepositoryOptions{Now: func() int64 { return 2100 }})

	_, err := repo.UpdateScorePublishConfig(
		t.Context(),
		serviceexam.UpdateScorePublishConfigRepositoryInput{
			TenantID:         10,
			ExamID:           999,
			PublishMode:      serviceexam.PublishModeImmediateScore,
			ScorePublishTime: nil,
			Log: &serviceexam.OperationLog{
				TenantID:        10,
				ExamID:          999,
				OperationType:   serviceexam.OperationTypePublishResults,
				OperationTitle:  "发布成绩",
				OperationDetail: "发布成绩给考生",
				ActorID:         11,
				ActorType:       AuditActorTenantUser,
				ActorRole:       "tenant_admin",
			},
		},
	)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected missing exam to return record not found, got %v", err)
	}
	assertOperationLogCount(t, gormDB, 10, 999, serviceexam.OperationTypePublishResults, 0)
}

func TestExamRepositoryUpdateManagementSettingsRejectsMissingExamBeforeWritingLog(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	repo := NewExamRepository(gormDB, ExamRepositoryOptions{Now: func() int64 { return 2100 }})

	_, err := repo.UpdateManagementSettings(t.Context(), serviceexam.UpdateManagementSettingsRepositoryInput{
		TenantID:         10,
		ExamID:           999,
		PublishMode:      serviceexam.PublishModeImmediateScore,
		ScorePublishTime: nil,
		Log: serviceexam.OperationLog{
			TenantID:        10,
			ExamID:          999,
			OperationType:   serviceexam.OperationTypeUpdateSettings,
			OperationTitle:  "修改考试设置",
			OperationDetail: "修改成绩发布配置",
			ActorID:         11,
			ActorType:       AuditActorTenantUser,
			ActorRole:       "tenant_admin",
		},
	})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected missing exam to return record not found, got %v", err)
	}
	assertOperationLogCount(t, gormDB, 10, 999, serviceexam.OperationTypeUpdateSettings, 0)
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

func TestExamRepositoryGradeShortTextRejectsMismatchedExamIDBeforeWritingLog(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	repo := NewExamRepository(gormDB, ExamRepositoryOptions{Now: func() int64 { return 2000 }})
	seedMultiSpaceAttemptData(t, gormDB)

	err := repo.GradeShortTextAndRecalculate(t.Context(), serviceexam.ShortTextGrade{
		TenantID:          10,
		ExamID:            901,
		AttemptID:         700,
		AttemptQuestionID: 800,
		AnswerVersion:     1,
		Score:             "4",
		GradedBy:          11,
		GraderType:        AuditActorTenantUser,
		GraderRole:        "teacher",
		SpaceIDs:          []uint64{302},
	})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected mismatched exam id to be rejected, got %v", err)
	}

	logs, err := repo.ListOperationLogs(t.Context(), serviceexam.ListOperationLogsInput{
		TenantID:      10,
		ExamID:        901,
		OperationType: serviceexam.OperationTypeGradeAnswer,
		Page:          1,
		PageSize:      10,
	})
	if err != nil {
		t.Fatalf("ListOperationLogs returned error: %v", err)
	}
	if logs.Total != 0 || len(logs.Items) != 0 {
		t.Fatalf("mismatched exam id should not write grading log, got %#v", logs)
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

func TestExamRepositoryAppendAndListOperationLogs(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	repo := NewExamRepository(gormDB, ExamRepositoryOptions{Now: func() int64 { return 1800 }})
	spaceID := uint64(301)
	otherSpaceID := uint64(302)
	seedMultiSpaceAttemptData(t, gormDB)

	logs := []serviceexam.OperationLog{
		{
			TenantID:        10,
			ExamID:          900,
			OperationType:   serviceexam.OperationTypePublishExam,
			OperationTitle:  "发布考试",
			OperationDetail: "发布到高一一班",
			ActorID:         11,
			ActorType:       AuditActorTenantUser,
			ActorRole:       "tenant_admin",
			SpaceID:         &spaceID,
			ExtJSON:         `{"operation_group_id":"group-1"}`,
		},
		{
			TenantID:        10,
			ExamID:          900,
			OperationType:   serviceexam.OperationTypeSendInvite,
			OperationTitle:  "发送邀请码",
			OperationDetail: "发送给语文培优班",
			ActorID:         12,
			ActorType:       AuditActorTenantUser,
			ActorRole:       "space_admin",
			SpaceID:         &otherSpaceID,
			ExtJSON:         `{}`,
		},
		{
			TenantID:        10,
			ExamID:          901,
			OperationType:   serviceexam.OperationTypePublishExam,
			OperationTitle:  "其它考试",
			OperationDetail: "不应出现在考试 900",
			ActorID:         11,
			ActorType:       AuditActorTenantUser,
			ActorRole:       "tenant_admin",
			ExtJSON:         `{}`,
		},
	}
	for _, log := range logs {
		if err := repo.AppendOperationLog(t.Context(), log); err != nil {
			t.Fatalf("AppendOperationLog returned error: %v", err)
		}
	}

	allLogs, err := repo.ListOperationLogs(t.Context(), serviceexam.ListOperationLogsInput{TenantID: 10, ExamID: 900, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListOperationLogs returned error: %v", err)
	}
	if allLogs.Total != 2 || len(allLogs.Items) != 2 {
		t.Fatalf("expected two logs for exam 900, got %#v", allLogs)
	}
	if allLogs.Items[0].OperationType != serviceexam.OperationTypeSendInvite || allLogs.Items[1].OperationType != serviceexam.OperationTypePublishExam {
		t.Fatalf("expected logs ordered by created_at desc and id desc, got %#v", allLogs.Items)
	}
	if allLogs.Items[1].ExtJSON != `{"operation_group_id":"group-1"}` {
		t.Fatalf("expected ext json to round trip, got %s", allLogs.Items[1].ExtJSON)
	}

	spaceLogs, err := repo.ListOperationLogs(t.Context(), serviceexam.ListOperationLogsInput{TenantID: 10, ExamID: 900, SpaceID: &spaceID, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListOperationLogs with space returned error: %v", err)
	}
	if spaceLogs.Total != 1 || len(spaceLogs.Items) != 1 || spaceLogs.Items[0].SpaceID == nil || *spaceLogs.Items[0].SpaceID != spaceID {
		t.Fatalf("expected only exact space logs, got %#v", spaceLogs)
	}

	filteredLogs, err := repo.ListOperationLogs(t.Context(), serviceexam.ListOperationLogsInput{
		TenantID:      10,
		ExamID:        900,
		OperationType: serviceexam.OperationTypePublishExam,
		Page:          1,
		PageSize:      10,
	})
	if err != nil {
		t.Fatalf("ListOperationLogs with type returned error: %v", err)
	}
	if filteredLogs.Total != 1 || filteredLogs.Items[0].OperationTitle != "发布考试" {
		t.Fatalf("expected only publish_exam log, got %#v", filteredLogs)
	}
}

func TestOperationGroupExtJSONGeneratesDistinctGroupIDsWithinSameMillisecond(t *testing.T) {
	log := serviceexam.OperationLog{
		TenantID:      10,
		ExamID:        900,
		OperationType: serviceexam.OperationTypeImportCandidates,
		ActorID:       11,
	}

	first := operationGroupExtJSON(log, 1800)
	second := operationGroupExtJSON(log, 1800)

	var firstPayload map[string]string
	if err := json.Unmarshal([]byte(first), &firstPayload); err != nil {
		t.Fatalf("unmarshal first ext json: %v", err)
	}
	var secondPayload map[string]string
	if err := json.Unmarshal([]byte(second), &secondPayload); err != nil {
		t.Fatalf("unmarshal second ext json: %v", err)
	}
	if firstPayload["operation_group_id"] == "" || secondPayload["operation_group_id"] == "" {
		t.Fatalf("expected operation_group_id in both payloads, got %q and %q", first, second)
	}
	if firstPayload["operation_group_id"] == secondPayload["operation_group_id"] {
		t.Fatalf("expected distinct operation_group_id values within same millisecond, got %q", firstPayload["operation_group_id"])
	}
}

func TestOperationGroupExtJSONDoesNotDependOnProcessLocalSequence(t *testing.T) {
	log := serviceexam.OperationLog{
		TenantID:      10,
		ExamID:        900,
		OperationType: serviceexam.OperationTypeImportCandidates,
		ActorID:       11,
	}

	operationGroupSequence = 0
	first := operationGroupExtJSON(log, 1800)
	// 模拟另一个新进程从相同的本地 sequence 起点生成同一业务操作的 group id。
	operationGroupSequence = 0
	second := operationGroupExtJSON(log, 1800)

	var firstPayload map[string]string
	if err := json.Unmarshal([]byte(first), &firstPayload); err != nil {
		t.Fatalf("unmarshal first ext json: %v", err)
	}
	var secondPayload map[string]string
	if err := json.Unmarshal([]byte(second), &secondPayload); err != nil {
		t.Fatalf("unmarshal second ext json: %v", err)
	}
	if firstPayload["operation_group_id"] == secondPayload["operation_group_id"] {
		t.Fatalf("expected operation_group_id to remain distinct across simulated process restarts, got %q", firstPayload["operation_group_id"])
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

func seedExamTargetScopeSpaces(t *testing.T, gormDB *gorm.DB, tenantID uint64, examID uint64, examTargetID uint64, spaceIDs ...uint64) {
	t.Helper()

	for _, spaceID := range spaceIDs {
		if err := gormDB.Table("exam_target_scope_spaces").Create(map[string]any{
			"tenant_id":      tenantID,
			"exam_id":        examID,
			"exam_target_id": examTargetID,
			"space_id":       spaceID,
			"created_at":     int64(1000),
			"ext_json":       "{}",
		}).Error; err != nil {
			t.Fatalf("seed exam target scope space %d: %v", spaceID, err)
		}
	}
}

func assertExamTargetScopeSpaces(t *testing.T, gormDB *gorm.DB, tenantID uint64, examID uint64, examTargetID uint64, want []uint64) {
	t.Helper()

	var got []uint64
	if err := gormDB.Table("exam_target_scope_spaces").
		Select("space_id").
		Where("tenant_id = ? AND exam_id = ? AND exam_target_id = ?", tenantID, examID, examTargetID).
		Order("space_id ASC").
		Scan(&got).Error; err != nil {
		t.Fatalf("query exam target scope spaces: %v", err)
	}
	assertUint64s(t, got, want)
}

func assertOperationLogCount(t *testing.T, gormDB *gorm.DB, tenantID uint64, examID uint64, operationType string, want int64) {
	t.Helper()

	var got int64
	if err := gormDB.Table("exam_operation_logs").
		Where("tenant_id = ? AND exam_id = ? AND operation_type = ?", tenantID, examID, operationType).
		Count(&got).Error; err != nil {
		t.Fatalf("count operation logs: %v", err)
	}
	if got != want {
		t.Fatalf("expected %d operation logs, got %d", want, got)
	}
}

func assertOperationLogsShareGroupAndSpaces(t *testing.T, logs []serviceexam.OperationLog, wantSpaces []uint64) {
	t.Helper()
	if len(logs) != len(wantSpaces) {
		t.Fatalf("expected %d logs for spaces %#v, got %#v", len(wantSpaces), wantSpaces, logs)
	}
	seenSpaces := make(map[uint64]struct{}, len(logs))
	var groupID string
	for _, log := range logs {
		if log.SpaceID == nil {
			t.Fatalf("expected scoped operation log, got nil space in %#v", log)
		}
		seenSpaces[*log.SpaceID] = struct{}{}
		var ext map[string]string
		if err := json.Unmarshal([]byte(log.ExtJSON), &ext); err != nil {
			t.Fatalf("operation log ext_json should be valid JSON, got %q: %v", log.ExtJSON, err)
		}
		currentGroupID := ext["operation_group_id"]
		if currentGroupID == "" {
			t.Fatalf("expected operation_group_id in ext_json, got %q", log.ExtJSON)
		}
		if groupID == "" {
			groupID = currentGroupID
			continue
		}
		if currentGroupID != groupID {
			t.Fatalf("expected same operation_group_id %q, got %q in %#v", groupID, currentGroupID, logs)
		}
	}
	for _, spaceID := range wantSpaces {
		if _, ok := seenSpaces[spaceID]; !ok {
			t.Fatalf("expected log scoped to space %d, got %#v", spaceID, logs)
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
		INSERT INTO papers (
			id, tenant_id, name, description, total_score, build_mode, status,
			created_at, updated_at, ext_json
		) VALUES (100, 10, '多空间试卷', '', 10, 'manual', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed paper: %v", err)
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

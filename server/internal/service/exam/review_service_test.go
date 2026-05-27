package exam

import (
	"context"
	"errors"
	"testing"

	"github.com/lifei6671/papermind/server/internal/service/permission"
)

func TestReviewServiceListsPendingAttemptsByAttempt(t *testing.T) {
	repo := newFakeReviewRepository()
	repo.pendingAttempts = []PendingAttempt{
		{AttemptID: 1001, ExamID: 10, UserID: 201, StudentName: "张三", SpaceID: 301, PendingShortTextCount: 2, SubmittedAt: fixedUnixMilli},
		{AttemptID: 1002, ExamID: 10, UserID: 202, StudentName: "李四", SpaceID: 301, PendingShortTextCount: 1, SubmittedAt: fixedUnixMilli + minuteMillis},
	}
	svc := NewReviewService(ReviewServiceOptions{Repo: repo, PermissionChecker: permission.NewFixedRoleChecker(), Now: fixedNow})

	items, err := svc.ListPendingAttempts(context.Background(), ListPendingAttemptsInput{
		Permission: teacherPermissionContext(),
		TenantID:   10,
		ExamID:     10,
	})
	if err != nil {
		t.Fatalf("ListPendingAttempts returned error: %v", err)
	}
	if len(items) != 2 || items[0].AttemptID != 1001 || items[1].AttemptID != 1002 {
		t.Fatalf("expected pending attempts grouped by attempt, got %#v", items)
	}
}

func TestReviewServiceFiltersPendingAttemptsByGradePermission(t *testing.T) {
	repo := newFakeReviewRepository()
	repo.pendingAttempts = []PendingAttempt{
		{AttemptID: 1001, ExamID: 10, UserID: 201, StudentName: "张三", SpaceID: 301, PendingShortTextCount: 2, SubmittedAt: fixedUnixMilli},
		{AttemptID: 1002, ExamID: 10, UserID: 202, StudentName: "李四", SpaceID: 302, PendingShortTextCount: 1, SubmittedAt: fixedUnixMilli + minuteMillis},
	}
	limited := teacherPermissionContext()
	svc := NewReviewService(ReviewServiceOptions{Repo: repo, PermissionChecker: permission.NewFixedRoleChecker(), Now: fixedNow})

	items, err := svc.ListPendingAttempts(context.Background(), ListPendingAttemptsInput{
		Permission: limited,
		TenantID:   10,
		ExamID:     10,
	})
	if err != nil {
		t.Fatalf("ListPendingAttempts returned error: %v", err)
	}
	if len(items) != 1 || items[0].AttemptID != 1001 {
		t.Fatalf("expected only gradable attempts, got %#v", items)
	}
}

func TestReviewServiceGradesShortTextWithVersionAndRecalculatesScores(t *testing.T) {
	repo := newFakeReviewRepository()
	repo.attemptSpaces = map[uint64]uint64{1001: 301}
	svc := NewReviewService(ReviewServiceOptions{Repo: repo, PermissionChecker: permission.NewFixedRoleChecker(), Now: fixedNow})

	err := svc.GradeShortText(context.Background(), GradeShortTextInput{
		Permission:        teacherPermissionContext(),
		TenantID:          10,
		AttemptID:         1001,
		AttemptQuestionID: 9001,
		AnswerVersion:     7,
		Score:             "4.5",
		Comment:           "要点完整",
	})
	if err != nil {
		t.Fatalf("GradeShortText returned error: %v", err)
	}
	if repo.graded.AttemptQuestionID != 9001 || repo.graded.AnswerVersion != 7 || repo.graded.Score != "4.5" {
		t.Fatalf("expected versioned short text grade saved, got %#v", repo.graded)
	}
	if repo.graded.GradedBy != 501 || repo.graded.GradedAt != fixedUnixMilli || repo.graded.Comment != "要点完整" {
		t.Fatalf("expected grader metadata saved, got %#v", repo.graded)
	}
	if !repo.recalculated {
		t.Fatal("expected subjective score and total score recalculated in grading transaction")
	}
}

func TestReviewServiceRejectsClientClaimedAttemptScope(t *testing.T) {
	repo := newFakeReviewRepository()
	repo.attemptSpaces = map[uint64]uint64{1001: 302}
	svc := NewReviewService(ReviewServiceOptions{Repo: repo, PermissionChecker: permission.NewFixedRoleChecker(), Now: fixedNow})

	err := svc.GradeShortText(context.Background(), GradeShortTextInput{
		Permission:        teacherPermissionContext(),
		TenantID:          10,
		AttemptID:         1001,
		AttemptQuestionID: 9001,
		AnswerVersion:     7,
		Score:             "4",
	})
	if !errors.Is(err, permission.ErrForbidden) {
		t.Fatalf("expected permission denied for mismatched attempt space, got %v", err)
	}
	if repo.graded.AttemptQuestionID != 0 {
		t.Fatalf("repository should not be called without permission, got %#v", repo.graded)
	}
}

func TestReviewServiceRejectsStaleShortTextGradeVersion(t *testing.T) {
	repo := newFakeReviewRepository()
	repo.attemptSpaces = map[uint64]uint64{1001: 301}
	repo.gradeErr = ErrAnswerVersionConflict
	svc := NewReviewService(ReviewServiceOptions{Repo: repo, PermissionChecker: permission.NewFixedRoleChecker(), Now: fixedNow})

	err := svc.GradeShortText(context.Background(), GradeShortTextInput{
		Permission:        teacherPermissionContext(),
		TenantID:          10,
		AttemptID:         1001,
		AttemptQuestionID: 9001,
		AnswerVersion:     6,
		Score:             "4",
	})
	if !errors.Is(err, ErrAnswerVersionConflict) {
		t.Fatalf("expected ErrAnswerVersionConflict, got %v", err)
	}
}

func TestReviewServiceUsesPermissionCheckerForGrading(t *testing.T) {
	repo := newFakeReviewRepository()
	svc := NewReviewService(ReviewServiceOptions{Repo: repo, PermissionChecker: permission.NewFixedRoleChecker(), Now: fixedNow})

	err := svc.GradeShortText(context.Background(), GradeShortTextInput{
		Permission:        permission.PermissionContext{SubjectType: permission.SubjectTenantUser, UserID: 502, TenantID: 10},
		TenantID:          10,
		AttemptID:         1001,
		AttemptQuestionID: 9001,
		AnswerVersion:     7,
		Score:             "4",
	})
	if !errors.Is(err, permission.ErrForbidden) {
		t.Fatalf("expected permission denied, got %v", err)
	}
	if repo.graded.AttemptQuestionID != 0 {
		t.Fatalf("repository should not be called without permission, got %#v", repo.graded)
	}
}

func newFakeReviewRepository() *fakeReviewRepository {
	return &fakeReviewRepository{}
}

func teacherPermissionContext() permission.PermissionContext {
	return permission.PermissionContext{
		SubjectType: permission.SubjectTenantUser,
		UserID:      501,
		TenantID:    10,
		SpaceRoles:  map[uint64]string{301: permission.RoleTeacher},
		AttemptScope: map[uint64]uint64{
			1001: 301,
			1002: 301,
		},
	}
}

type fakeReviewRepository struct {
	pendingAttempts []PendingAttempt
	attemptSpaces   map[uint64]uint64
	graded          ShortTextGrade
	recalculated    bool
	gradeErr        error
}

func (r *fakeReviewRepository) ListPendingAttempts(ctx context.Context, tenantID uint64, examID uint64) ([]PendingAttempt, error) {
	return append([]PendingAttempt(nil), r.pendingAttempts...), nil
}

func (r *fakeReviewRepository) AttemptSpaceIDs(ctx context.Context, tenantID uint64, attemptID uint64) ([]uint64, error) {
	return []uint64{r.attemptSpaces[attemptID]}, nil
}

func (r *fakeReviewRepository) GradeShortTextAndRecalculate(ctx context.Context, grade ShortTextGrade) error {
	if r.gradeErr != nil {
		return r.gradeErr
	}
	r.graded = grade
	r.recalculated = true
	return nil
}

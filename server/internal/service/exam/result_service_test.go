package exam

import (
	"context"
	"errors"
	"testing"

	"github.com/lifei6671/papermind/server/internal/service/permission"
)

func TestResultServiceShowsImmediateScoreForObjectiveExam(t *testing.T) {
	repo := newFakeResultRepository()
	repo.result = ResultSnapshot{
		AttemptID:       1001,
		ExamID:          20,
		UserID:          30,
		AttemptNo:       1,
		TotalScore:      "8",
		ObjectiveScore:  "8",
		SubjectiveScore: "0",
		PublishMode:     PublishModeImmediateScore,
		ShowAnalysis:    true,
	}
	svc := NewResultService(ResultServiceOptions{Repo: repo, PermissionChecker: permission.NewFixedRoleChecker(), Now: fixedNow})

	result, err := svc.GetVisibleResult(context.Background(), ResultQueryInput{Permission: studentPermissionContext(), TenantID: 10, AttemptID: 1001})
	if err != nil {
		t.Fatalf("GetVisibleResult returned error: %v", err)
	}
	if result.TotalScore != "8" || !result.AnalysisVisible {
		t.Fatalf("expected immediate objective score with analysis, got %#v", result)
	}
}

func TestResultServiceHidesManualPublishResultBeforePublishTime(t *testing.T) {
	publishAt := fixedUnixMilli + minuteMillis
	repo := newFakeResultRepository()
	repo.result = ResultSnapshot{
		AttemptID:        1001,
		ExamID:           20,
		UserID:           30,
		AttemptNo:        1,
		TotalScore:       "8",
		PublishMode:      PublishModeManualPublish,
		ScorePublishTime: &publishAt,
	}
	svc := NewResultService(ResultServiceOptions{Repo: repo, PermissionChecker: permission.NewFixedRoleChecker(), Now: fixedNow})

	_, err := svc.GetVisibleResult(context.Background(), ResultQueryInput{Permission: studentPermissionContext(), TenantID: 10, AttemptID: 1001})
	if !errors.Is(err, ErrResultNotVisible) {
		t.Fatalf("expected ErrResultNotVisible before publish time, got %v", err)
	}
}

func TestResultServiceShowsManualPublishResultAtPublishTimeAndHonorsAnalysisSwitch(t *testing.T) {
	publishAt := fixedUnixMilli
	repo := newFakeResultRepository()
	repo.result = ResultSnapshot{
		AttemptID:        1001,
		ExamID:           20,
		UserID:           30,
		AttemptNo:        1,
		TotalScore:       "8",
		PublishMode:      PublishModeManualPublish,
		ScorePublishTime: &publishAt,
		ShowAnalysis:     false,
	}
	svc := NewResultService(ResultServiceOptions{Repo: repo, PermissionChecker: permission.NewFixedRoleChecker(), Now: fixedNow})

	result, err := svc.GetVisibleResult(context.Background(), ResultQueryInput{Permission: studentPermissionContext(), TenantID: 10, AttemptID: 1001})
	if err != nil {
		t.Fatalf("GetVisibleResult returned error: %v", err)
	}
	if result.AnalysisVisible {
		t.Fatalf("show_analysis=false should hide analysis, got %#v", result)
	}
}

func TestResultServiceShowsOwnResultForSpaceStudentMembership(t *testing.T) {
	repo := newFakeResultRepository()
	repo.result = ResultSnapshot{
		AttemptID:   1001,
		ExamID:      20,
		UserID:      30,
		AttemptNo:   1,
		TotalScore:  "8",
		PublishMode: PublishModeImmediateScore,
	}
	svc := NewResultService(ResultServiceOptions{Repo: repo, PermissionChecker: permission.NewFixedRoleChecker(), Now: fixedNow})
	permissionContext := permission.PermissionContext{
		SubjectType:      permission.SubjectTenantUser,
		UserID:           30,
		TenantID:         10,
		Role:             permission.RoleTeacher,
		SpaceMemberships: map[uint64]string{301: permission.RoleStudent},
	}

	result, err := svc.GetVisibleResult(context.Background(), ResultQueryInput{Permission: permissionContext, TenantID: 10, AttemptID: 1001})
	if err != nil {
		t.Fatalf("space student should view own published result: %v", err)
	}
	if result.AttemptID != 1001 || result.TotalScore != "8" {
		t.Fatalf("expected own visible result, got %#v", result)
	}
}

func TestResultServiceSelectsLatestAndHighestAttempt(t *testing.T) {
	repo := newFakeResultRepository()
	repo.results = []ResultSnapshot{
		{AttemptID: 1001, ExamID: 20, UserID: 30, AttemptNo: 1, TotalScore: "7", PublishMode: PublishModeImmediateScore},
		{AttemptID: 1002, ExamID: 20, UserID: 30, AttemptNo: 2, TotalScore: "9", PublishMode: PublishModeImmediateScore},
		{AttemptID: 1003, ExamID: 20, UserID: 30, AttemptNo: 3, TotalScore: "8", PublishMode: PublishModeImmediateScore},
	}
	svc := NewResultService(ResultServiceOptions{Repo: repo, PermissionChecker: permission.NewFixedRoleChecker(), Now: fixedNow})

	latest, err := svc.SelectVisibleResult(context.Background(), SelectResultInput{Permission: studentPermissionContext(), TenantID: 10, ExamID: 20, ResultStrategy: ResultStrategyLatest})
	if err != nil {
		t.Fatalf("SelectVisibleResult latest returned error: %v", err)
	}
	if latest.AttemptID != 1003 {
		t.Fatalf("expected latest attempt 1003, got %#v", latest)
	}

	highest, err := svc.SelectVisibleResult(context.Background(), SelectResultInput{Permission: studentPermissionContext(), TenantID: 10, ExamID: 20, ResultStrategy: ResultStrategyHighest})
	if err != nil {
		t.Fatalf("SelectVisibleResult highest returned error: %v", err)
	}
	if highest.AttemptID != 1002 {
		t.Fatalf("expected highest attempt 1002, got %#v", highest)
	}
}

func TestResultServiceRejectsViewingAnotherStudentsResult(t *testing.T) {
	repo := newFakeResultRepository()
	repo.result = ResultSnapshot{
		AttemptID:   1001,
		ExamID:      20,
		UserID:      99,
		AttemptNo:   1,
		TotalScore:  "8",
		PublishMode: PublishModeImmediateScore,
	}
	svc := NewResultService(ResultServiceOptions{Repo: repo, PermissionChecker: permission.NewFixedRoleChecker(), Now: fixedNow})

	_, err := svc.GetVisibleResult(context.Background(), ResultQueryInput{Permission: studentPermissionContext(), TenantID: 10, AttemptID: 1001})
	if !errors.Is(err, ErrResultNotVisible) {
		t.Fatalf("expected ErrResultNotVisible for another student's result, got %v", err)
	}
}

func studentPermissionContext() permission.PermissionContext {
	return permission.PermissionContext{
		SubjectType: permission.SubjectTenantUser,
		UserID:      30,
		TenantID:    10,
		Role:        permission.RoleStudent,
		ExamScope:   map[uint64]uint64{20: 301},
	}
}

func newFakeResultRepository() *fakeResultRepository {
	return &fakeResultRepository{}
}

type fakeResultRepository struct {
	result  ResultSnapshot
	results []ResultSnapshot
}

func (r *fakeResultRepository) GetResultSnapshot(ctx context.Context, tenantID uint64, attemptID uint64) (ResultSnapshot, error) {
	return r.result, nil
}

func (r *fakeResultRepository) ListUserResultSnapshots(ctx context.Context, tenantID uint64, examID uint64, userID uint64) ([]ResultSnapshot, error) {
	return append([]ResultSnapshot(nil), r.results...), nil
}

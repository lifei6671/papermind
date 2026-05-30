package exam

import (
	"context"
	"encoding/csv"
	"errors"
	"github.com/lifei6671/papermind/server/internal/service/permission"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportServiceWritesExamScoresWithRequiredFields(t *testing.T) {
	exportDir := t.TempDir()
	repo := &fakeExportRepository{
		rows: []ScoreExportRow{
			{
				StudentName:     "张三",
				SpaceID:         301,
				SpaceName:       "高一一班",
				AttemptNo:       2,
				ObjectiveScore:  "8",
				SubjectiveScore: "4",
				TotalScore:      "12",
				SubmittedAt:     fixedUnixMilli,
			},
		},
	}
	svc := NewExportService(ExportServiceOptions{Repo: repo, PermissionChecker: permission.NewFixedRoleChecker(), ExportDir: exportDir, Now: fixedNow})

	result, err := svc.ExportExamScores(context.Background(), ExportExamScoresInput{Permission: exportPermissionContext(), TenantID: 10, ExamID: 20})
	if err != nil {
		t.Fatalf("ExportExamScores returned error: %v", err)
	}
	if !strings.HasPrefix(result.FilePath, exportDir) {
		t.Fatalf("expected export file under configured dir, got %q", result.FilePath)
	}
	if !strings.HasPrefix(filepath.Base(result.FilePath), "tenant-10-exam-20-scores-space-301-") {
		t.Fatalf("expected export file name scoped by tenant, got %q", filepath.Base(result.FilePath))
	}
	data, err := os.ReadFile(result.FilePath)
	if err != nil {
		t.Fatalf("ReadFile exported CSV returned error: %v", err)
	}
	content := string(data)
	for _, want := range []string{"考生姓名", "空间名称", "attempt 次数", "客观题分", "主观题分", "总分", "提交时间", "张三", "高一一班", "12"} {
		if !strings.Contains(content, want) {
			t.Fatalf("expected exported CSV to contain %q, got:\n%s", want, content)
		}
	}
	if result.RowCount != 1 {
		t.Fatalf("expected one exported row, got %d", result.RowCount)
	}
}

func TestExportServiceCreatesConfiguredExportDir(t *testing.T) {
	root := t.TempDir()
	exportDir := filepath.Join(root, "nested", "exports")
	repo := &fakeExportRepository{}
	svc := NewExportService(ExportServiceOptions{Repo: repo, PermissionChecker: permission.NewFixedRoleChecker(), ExportDir: exportDir, Now: fixedNow})

	result, err := svc.ExportExamScores(context.Background(), ExportExamScoresInput{Permission: exportPermissionContext(), TenantID: 10, ExamID: 20})
	if err != nil {
		t.Fatalf("ExportExamScores returned error: %v", err)
	}
	if _, err := os.Stat(result.FilePath); err != nil {
		t.Fatalf("expected export file created: %v", err)
	}
}
func TestExportServiceRejectsExportFileFromDifferentTenant(t *testing.T) {
	exportDir := t.TempDir()
	fileName := "exam-20-scores-tenant-123456.csv"
	if err := os.WriteFile(filepath.Join(exportDir, fileName), []byte("考生姓名\n张三\n"), 0o644); err != nil {
		t.Fatalf("write fixture export file: %v", err)
	}
	svc := NewExportService(ExportServiceOptions{Repo: &fakeExportRepository{}, PermissionChecker: permission.NewFixedRoleChecker(), ExportDir: exportDir, Now: fixedNow})

	_, err := svc.ResolveExportFile(fileName, 20, permission.PermissionContext{
		SubjectType: permission.SubjectTenantUser,
		UserID:      601,
		TenantID:    11,
		Role:        permission.RoleTenantAdmin,
	})

	if !errors.Is(err, ErrInvalidExportFile) {
		t.Fatalf("expected cross-tenant export file rejected, got %v", err)
	}
}

func TestExportServiceFiltersRowsByActualSpaceScope(t *testing.T) {
	repo := &fakeExportRepository{
		rows: []ScoreExportRow{
			{StudentName: "张三", SpaceID: 301, SpaceName: "一班", TotalScore: "10"},
			{StudentName: "李四", SpaceID: 302, SpaceName: "二班", TotalScore: "12"},
		},
	}
	svc := NewExportService(ExportServiceOptions{Repo: repo, PermissionChecker: permission.NewFixedRoleChecker(), ExportDir: t.TempDir(), Now: fixedNow})

	rows, err := svc.ListExamScores(context.Background(), ListExamScoresInput{
		Permission: teacherScorePermissionContext(),
		TenantID:   10,
		ExamID:     20,
	})
	if err != nil {
		t.Fatalf("ListExamScores returned error: %v", err)
	}
	if len(rows) != 1 || rows[0].StudentName != "张三" {
		t.Fatalf("expected only rows in actual allowed space, got %#v", rows)
	}
}

func TestExportServiceReturnsEmptyRowsWithoutExamScope(t *testing.T) {
	repo := &fakeExportRepository{}
	svc := NewExportService(ExportServiceOptions{Repo: repo, PermissionChecker: permission.NewFixedRoleChecker(), ExportDir: t.TempDir(), Now: fixedNow})

	rows, err := svc.ListExamScores(context.Background(), ListExamScoresInput{
		Permission: permission.PermissionContext{
			SubjectType:      permission.SubjectTenantUser,
			UserID:           601,
			TenantID:         10,
			Role:             permission.RoleTeacher,
			SpaceMemberships: map[uint64]string{301: permission.RoleTeacher},
		},
		TenantID: 10,
		ExamID:   20,
	})
	if err != nil {
		t.Fatalf("ListExamScores returned error: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected empty score rows, got %#v", rows)
	}
}

func TestExportServiceRejectsTeacherExportEvenWhenTeacherCanViewScores(t *testing.T) {
	repo := &fakeExportRepository{
		rows: []ScoreExportRow{{StudentName: "张三", SpaceID: 301, SpaceName: "一班", TotalScore: "10"}},
	}
	svc := NewExportService(ExportServiceOptions{Repo: repo, PermissionChecker: permission.NewFixedRoleChecker(), ExportDir: t.TempDir(), Now: fixedNow})

	rows, err := svc.ListExamScores(context.Background(), ListExamScoresInput{
		Permission: teacherScorePermissionContext(),
		TenantID:   10,
		ExamID:     20,
	})
	if err != nil || len(rows) != 1 {
		t.Fatalf("teacher should view scoped score rows, rows = %#v, err = %v", rows, err)
	}
	_, err = svc.ExportExamScores(context.Background(), ExportExamScoresInput{
		Permission: teacherScorePermissionContext(),
		TenantID:   10,
		ExamID:     20,
	})
	if !errors.Is(err, permission.ErrForbidden) {
		t.Fatalf("teacher should not export scores, got %v", err)
	}
}

func TestExportServiceRejectsUnauthorizedExport(t *testing.T) {
	repo := &fakeExportRepository{}
	svc := NewExportService(ExportServiceOptions{Repo: repo, PermissionChecker: permission.NewFixedRoleChecker(), ExportDir: t.TempDir(), Now: fixedNow})

	_, err := svc.ExportExamScores(context.Background(), ExportExamScoresInput{
		Permission: permission.PermissionContext{SubjectType: permission.SubjectTenantUser, UserID: 601, TenantID: 10},
		TenantID:   10,
		ExamID:     20,
	})
	if !errors.Is(err, permission.ErrForbidden) {
		t.Fatalf("expected permission denied, got %v", err)
	}
	if repo.called {
		t.Fatal("repository should not be called when export permission is denied")
	}
}

func TestWriteScoreExportCSVReturnsFlushErrors(t *testing.T) {
	err := writeScoreExportCSV(csv.NewWriter(failingWriter{}), []ScoreExportRow{{StudentName: "张三"}})
	if !errors.Is(err, errFailingWriter) {
		t.Fatalf("expected flush error returned, got %v", err)
	}
}

func exportPermissionContext() permission.PermissionContext {
	return permission.PermissionContext{
		SubjectType:      permission.SubjectTenantUser,
		UserID:           601,
		TenantID:         10,
		Role:             permission.RoleTeacher,
		SpaceMemberships: map[uint64]string{301: permission.RoleSpaceAdmin},
		ExamScope:        map[uint64]uint64{20: 301},
	}
}

func teacherScorePermissionContext() permission.PermissionContext {
	return permission.PermissionContext{
		SubjectType:      permission.SubjectTenantUser,
		UserID:           601,
		TenantID:         10,
		Role:             permission.RoleTeacher,
		SpaceMemberships: map[uint64]string{301: permission.RoleTeacher},
		ExamScope:        map[uint64]uint64{20: 301},
	}
}

var errFailingWriter = errors.New("writer failed")

type failingWriter struct{}

func (failingWriter) Write(p []byte) (int, error) {
	return 0, errFailingWriter
}

type fakeExportRepository struct {
	rows   []ScoreExportRow
	called bool
}

func (r *fakeExportRepository) ListScoreExportRows(ctx context.Context, tenantID uint64, examID uint64) ([]ScoreExportRow, error) {
	r.called = true
	return append([]ScoreExportRow(nil), r.rows...), nil
}

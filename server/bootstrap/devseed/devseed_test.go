package devseed

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/lifei6671/papermind/server/bootstrap/migration"
	dbmodel "github.com/lifei6671/papermind/server/internal/dao/db"
	serviceexam "github.com/lifei6671/papermind/server/internal/service/exam"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const fixedSeedNow int64 = 1_780_000_000_000

func TestRunForDriverSeedsSQLiteDevDataForE2E(t *testing.T) {
	gormDB := openDevSeedTestDB(t)

	if err := runForDriver(gormDB, "sqlite", "dev", func() int64 { return fixedSeedNow }); err != nil {
		t.Fatalf("runForDriver returned error: %v", err)
	}

	assertCount(t, gormDB, "tenants", 1)
	assertCount(t, gormDB, "users", 3)
	assertCount(t, gormDB, "spaces", 2)
	assertCount(t, gormDB, "questions", 2)
	assertCount(t, gormDB, "paper_section_questions", 2)
	assertCount(t, gormDB, "exam_targets", 1)
	assertDemoQuestionAudit(t, gormDB)

	var tenant struct {
		TenantCode    string
		AllowRegister bool
	}
	if err := gormDB.Table("tenants").
		Select("tenant_code, allow_register").
		Where("id = ?", demoTenantID).
		Scan(&tenant).Error; err != nil {
		t.Fatalf("query tenant: %v", err)
	}
	if tenant.TenantCode != "PM-QT01" || !tenant.AllowRegister {
		t.Fatalf("unexpected demo tenant: %#v", tenant)
	}

	repo := dbmodel.NewExamRepository(gormDB, dbmodel.ExamRepositoryOptions{
		Now: func() int64 { return fixedSeedNow + time.Minute.Milliseconds() },
	})
	service := serviceexam.NewService(serviceexam.ServiceOptions{
		Repo:        repo,
		TokenIssuer: fixedTokenIssuer{token: "exam-token"},
		Now:         func() int64 { return fixedSeedNow + time.Minute.Milliseconds() },
	})

	started, err := service.StartExam(context.Background(), serviceexam.StartInput{
		TenantID: demoTenantID,
		ExamID:   demoExamID,
		UserID:   demoStudentID,
	})
	if err != nil {
		t.Fatalf("StartExam returned error: %v", err)
	}

	snapshots, err := service.GenerateAttemptSnapshots(context.Background(), serviceexam.GenerateSnapshotInput{
		TenantID:  demoTenantID,
		ExamID:    demoExamID,
		AttemptID: started.Attempt.ID,
	})
	if err != nil {
		t.Fatalf("GenerateAttemptSnapshots returned error: %v", err)
	}
	if len(snapshots) != 2 {
		t.Fatalf("expected 2 attempt snapshots, got %d", len(snapshots))
	}
}

func TestRunForDriverIsIdempotentAndRefreshesExamWindow(t *testing.T) {
	gormDB := openDevSeedTestDB(t)

	if err := runForDriver(gormDB, "sqlite", "dev", func() int64 { return fixedSeedNow }); err != nil {
		t.Fatalf("first runForDriver returned error: %v", err)
	}
	if err := gormDB.Model(&dbmodel.QuestionDO{}).
		Where("tenant_id = ? AND id IN ?", demoTenantID, []uint64{demoSingleQuestionID, demoShortQuestionID}).
		Updates(map[string]any{"created_by": 0, "created_by_type": "system", "updated_by": 0, "updated_by_type": "system"}).Error; err != nil {
		t.Fatalf("clear seeded question audit: %v", err)
	}
	if err := runForDriver(gormDB, "sqlite", "dev", func() int64 { return fixedSeedNow + time.Hour.Milliseconds() }); err != nil {
		t.Fatalf("second runForDriver returned error: %v", err)
	}

	assertCount(t, gormDB, "users", 3)
	assertCount(t, gormDB, "tenant_user_memberships", 3)
	assertCount(t, gormDB, "exam_targets", 1)

	var exam struct {
		StartTime int64
		EndTime   int64
	}
	if err := gormDB.Table("exams").
		Select("start_time, end_time").
		Where("id = ?", demoExamID).
		Scan(&exam).Error; err != nil {
		t.Fatalf("query exam: %v", err)
	}
	if exam.StartTime != fixedSeedNow || exam.EndTime != fixedSeedNow+time.Hour.Milliseconds()+sevenDaysMillis {
		t.Fatalf("exam window was not refreshed by second seed run: %#v", exam)
	}
	assertDemoQuestionAudit(t, gormDB)
}

func TestRunForDriverSkipsOutsideSQLiteDev(t *testing.T) {
	for _, item := range []struct {
		name   string
		driver string
		env    string
	}{
		{name: "prod sqlite", driver: "sqlite", env: "prod"},
		{name: "dev mysql", driver: "mysql", env: "dev"},
	} {
		t.Run(item.name, func(t *testing.T) {
			gormDB := openDevSeedTestDB(t)

			if err := runForDriver(gormDB, item.driver, item.env, func() int64 { return fixedSeedNow }); err != nil {
				t.Fatalf("runForDriver returned error: %v", err)
			}

			assertCount(t, gormDB, "tenants", 0)
		})
	}
}

func openDevSeedTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gormDB, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_foreign_keys=on"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	migrationsRoot := filepath.Join("..", "..", "data", "migrations")
	if err := migration.RunForDriver(gormDB, migrationsRoot, "sqlite"); err != nil {
		t.Fatalf("run migration: %v", err)
	}
	return gormDB
}

func assertCount(t *testing.T, gormDB *gorm.DB, table string, want int64) {
	t.Helper()

	var got int64
	if err := gormDB.Table(table).Count(&got).Error; err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	if got != want {
		t.Fatalf("count %s = %d, want %d", table, got, want)
	}
}

func assertDemoQuestionAudit(t *testing.T, gormDB *gorm.DB) {
	t.Helper()

	var count int64
	if err := gormDB.Table("questions").
		Where("tenant_id = ?", demoTenantID).
		Where("id IN ?", []uint64{demoSingleQuestionID, demoShortQuestionID}).
		Where("created_by = ?", demoTeacherID).
		Where("created_by_type = ?", dbmodel.AuditActorTenantUser).
		Where("updated_by = ?", demoTeacherID).
		Where("updated_by_type = ?", dbmodel.AuditActorTenantUser).
		Count(&count).Error; err != nil {
		t.Fatalf("query demo question audit: %v", err)
	}
	if count != 2 {
		t.Fatalf("demo questions should be attributed to teacher01, got %d rows", count)
	}
}

type fixedTokenIssuer struct {
	token string
}

func (i fixedTokenIssuer) IssueToken() (string, error) {
	return i.token, nil
}

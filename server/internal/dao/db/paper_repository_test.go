package db

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lifei6671/papermind/server/bootstrap/migration"
	servicepaper "github.com/lifei6671/papermind/server/internal/service/paper"
	servicequestion "github.com/lifei6671/papermind/server/internal/service/question"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPaperRepositoryExcludesDisabledSpacePapers(t *testing.T) {
	gormDB := openPaperRepositoryTestDB(t)
	repo := NewPaperRepository(gormDB, PaperRepositoryOptions{Now: func() int64 { return 1000 }})
	questionRepo := NewQuestionRepository(gormDB, QuestionRepositoryOptions{Now: func() int64 { return 1000 }})

	enabledSpaceID := uint64(301)
	disabledSpaceID := uint64(302)
	seedPaperSpace(t, gormDB, enabledSpaceID, "enabled")
	seedPaperSpace(t, gormDB, disabledSpaceID, "enabled")

	publicPaper := createPaperForTest(t, repo, servicepaper.Paper{
		TenantID: 10,
		SpaceID:  nil,
		Name:     "公共试卷",
	})
	enabledPaper := createPaperForTest(t, repo, servicepaper.Paper{
		TenantID: 10,
		SpaceID:  &enabledSpaceID,
		Name:     "启用空间试卷",
	})
	disabledPaper := createPaperForTest(t, repo, servicepaper.Paper{
		TenantID: 10,
		SpaceID:  &disabledSpaceID,
		Name:     "禁用空间试卷",
	})
	if err := gormDB.Exec("UPDATE spaces SET status = 'disabled' WHERE tenant_id = 10 AND id = ?", disabledSpaceID).Error; err != nil {
		t.Fatalf("disable paper space: %v", err)
	}

	list, err := repo.ListPapers(t.Context(), servicepaper.ListPapersInput{TenantID: 10})
	if err != nil {
		t.Fatalf("ListPapers returned error: %v", err)
	}
	gotNames := make([]string, 0, len(list.Items))
	for _, item := range list.Items {
		gotNames = append(gotNames, item.Name)
	}
	if strings.Join(gotNames, ",") != "启用空间试卷,公共试卷" {
		t.Fatalf("paper names = %#v, want enabled space and public papers only", gotNames)
	}

	if _, err := repo.GetPaper(t.Context(), 10, disabledPaper.ID); !errors.Is(err, servicepaper.ErrPaperNotFound) {
		t.Fatalf("GetPaper disabled space err = %v, want ErrPaperNotFound", err)
	}
	if _, err := repo.GetPaperSpaceID(t.Context(), 10, disabledPaper.ID); !errors.Is(err, servicepaper.ErrPaperNotFound) {
		t.Fatalf("GetPaperSpaceID disabled space err = %v, want ErrPaperNotFound", err)
	}
	if _, err := repo.UpdatePaperStatus(t.Context(), 10, disabledPaper.ID, servicepaper.StatusDisabled, 99); !errors.Is(err, servicepaper.ErrPaperNotFound) {
		t.Fatalf("UpdatePaperStatus disabled space err = %v, want ErrPaperNotFound", err)
	}

	publicQuestion, err := questionRepo.CreateQuestion(t.Context(), servicequestion.Question{
		TenantID:     10,
		Type:         servicequestion.QuestionTypeShortText,
		Difficulty:   servicequestion.DifficultyMedium,
		Title:        "公共题",
		ScoreDefault: "2",
		Status:       servicequestion.QuestionStatusEnabled,
	}, nil, nil)
	if err != nil {
		t.Fatalf("CreateQuestion returned error: %v", err)
	}
	if usable, err := repo.QuestionUsableForPaper(t.Context(), 10, disabledPaper.ID, publicQuestion.ID); err != nil || usable {
		t.Fatalf("QuestionUsableForPaper disabled paper usable=%v err=%v, want false nil", usable, err)
	}
	if usable, err := repo.QuestionUsableForScope(t.Context(), 10, &disabledSpaceID, publicQuestion.ID); err != nil || usable {
		t.Fatalf("QuestionUsableForScope disabled space usable=%v err=%v, want false nil", usable, err)
	}
	if _, err := repo.GetPaper(t.Context(), 10, publicPaper.ID); err != nil {
		t.Fatalf("GetPaper public paper returned error: %v", err)
	}
	if _, err := repo.GetPaper(t.Context(), 10, enabledPaper.ID); err != nil {
		t.Fatalf("GetPaper enabled space paper returned error: %v", err)
	}
}

func TestPaperRepositorySearchesBeforePaginating(t *testing.T) {
	gormDB := openPaperRepositoryTestDB(t)
	repo := NewPaperRepository(gormDB, PaperRepositoryOptions{Now: func() int64 { return 1000 }})

	createPaperForTest(t, repo, servicepaper.Paper{TenantID: 10, Name: "高一语文月考"})
	createPaperForTest(t, repo, servicepaper.Paper{TenantID: 10, Name: "高一数学月考"})
	createPaperForTest(t, repo, servicepaper.Paper{TenantID: 10, Name: "高一英语周测"})

	list, err := repo.ListPapers(t.Context(), servicepaper.ListPapersInput{
		TenantID: 10,
		Page:     1,
		PageSize: 1,
		Search:   "月考",
	})
	if err != nil {
		t.Fatalf("ListPapers returned error: %v", err)
	}
	if list.Total != 2 || len(list.Items) != 1 {
		t.Fatalf("expected filtered total=2 and one paged item, got %#v", list)
	}
	if list.Items[0].Name != "高一数学月考" {
		t.Fatalf("expected newest matching paper first, got %#v", list.Items[0])
	}
}

func createPaperForTest(t *testing.T, repo *PaperRepository, paper servicepaper.Paper) servicepaper.Paper {
	t.Helper()

	if paper.DurationMinutes == 0 {
		paper.DurationMinutes = 90
	}
	if paper.BuildMode == "" {
		paper.BuildMode = servicepaper.BuildModeManual
	}
	if paper.Status == "" {
		paper.Status = servicepaper.StatusEnabled
	}
	created, err := repo.CreatePaper(t.Context(), paper)
	if err != nil {
		t.Fatalf("CreatePaper(%s) returned error: %v", paper.Name, err)
	}
	return created
}

func openPaperRepositoryTestDB(t *testing.T) *gorm.DB {
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

func seedPaperSpace(t *testing.T, gormDB *gorm.DB, spaceID uint64, status string) {
	t.Helper()

	if err := gormDB.Exec(`
		INSERT INTO spaces (
			id, tenant_id, name, logo_url, description, type, status,
			created_at, updated_at, ext_json
		) VALUES (?, 10, '试卷测试空间', '', '', 'class', ?, 1000, 1000, '{}')
	`, spaceID, status).Error; err != nil {
		t.Fatalf("seed paper space %d: %v", spaceID, err)
	}
}

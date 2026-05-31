package db

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/lifei6671/papermind/server/bootstrap/migration"
	servicequestion "github.com/lifei6671/papermind/server/internal/service/question"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestQuestionRepositoryPersistsStandardAndReferenceAnswers(t *testing.T) {
	gormDB := openQuestionRepositoryTestDB(t)
	repo := NewQuestionRepository(gormDB, QuestionRepositoryOptions{Now: func() int64 { return 1000 }})

	created, err := repo.CreateQuestion(t.Context(), servicequestion.Question{
		TenantID:        10,
		Type:            servicequestion.QuestionTypeFillBlank,
		Difficulty:      servicequestion.DifficultyEasy,
		Title:           "Go 的包管理文件是 ____。",
		ScoreDefault:    "3",
		StandardAnswer:  "go.mod",
		ReferenceAnswer: "用于人工复核的参考说明",
		Status:          servicequestion.QuestionStatusEnabled,
	}, nil, nil)
	if err != nil {
		t.Fatalf("CreateQuestion returned error: %v", err)
	}
	if created.StandardAnswer != "go.mod" || created.ReferenceAnswer != "用于人工复核的参考说明" {
		t.Fatalf("expected created question to keep answers, got %#v", created)
	}

	result, err := repo.ListVisibleQuestions(t.Context(), servicequestion.ListQuestionsInput{TenantID: 10, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListVisibleQuestions returned error: %v", err)
	}
	if len(result.Items) != 1 {
		t.Fatalf("expected one question, got %#v", result.Items)
	}
	if result.Items[0].StandardAnswer != "go.mod" || result.Items[0].ReferenceAnswer != "用于人工复核的参考说明" {
		t.Fatalf("expected listed question to keep answers, got %#v", result.Items[0])
	}
}

func openQuestionRepositoryTestDB(t *testing.T) *gorm.DB {
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

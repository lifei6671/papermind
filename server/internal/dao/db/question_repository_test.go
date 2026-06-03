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

func TestQuestionRepositoryListsVisibleQuestionsByCreatedAtDesc(t *testing.T) {
	gormDB := openQuestionRepositoryTestDB(t)
	now := int64(1000)
	repo := NewQuestionRepository(gormDB, QuestionRepositoryOptions{Now: func() int64 { return now }})

	for _, item := range []struct {
		title string
		now   int64
	}{
		{title: "最早创建的题目", now: 1000},
		{title: "最新创建的题目", now: 3000},
		{title: "中间创建的题目", now: 2000},
	} {
		now = item.now
		if _, err := repo.CreateQuestion(t.Context(), servicequestion.Question{
			TenantID:     10,
			Type:         servicequestion.QuestionTypeShortText,
			Difficulty:   servicequestion.DifficultyMedium,
			Title:        item.title,
			ScoreDefault: "2",
			Status:       servicequestion.QuestionStatusEnabled,
		}, nil, nil); err != nil {
			t.Fatalf("CreateQuestion(%s) returned error: %v", item.title, err)
		}
	}

	result, err := repo.ListVisibleQuestions(t.Context(), servicequestion.ListQuestionsInput{TenantID: 10, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListVisibleQuestions returned error: %v", err)
	}
	got := make([]string, 0, len(result.Items))
	for _, item := range result.Items {
		got = append(got, item.Title)
	}
	want := []string{"最新创建的题目", "中间创建的题目", "最早创建的题目"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("question order = %#v, want %#v", got, want)
	}
}

func TestQuestionRepositoryListWithoutSpaceIncludesTenantAndSpaceQuestions(t *testing.T) {
	gormDB := openQuestionRepositoryTestDB(t)
	now := int64(1000)
	repo := NewQuestionRepository(gormDB, QuestionRepositoryOptions{Now: func() int64 { return now }})
	spaceID := uint64(301)

	for _, item := range []struct {
		title   string
		spaceID *uint64
		now     int64
	}{
		{title: "教师上传的空间题", spaceID: &spaceID, now: 2000},
		{title: "租户公共题", spaceID: nil, now: 1000},
	} {
		now = item.now
		if _, err := repo.CreateQuestion(t.Context(), servicequestion.Question{
			TenantID:     10,
			SpaceID:      item.spaceID,
			Type:         servicequestion.QuestionTypeShortText,
			Difficulty:   servicequestion.DifficultyMedium,
			Title:        item.title,
			ScoreDefault: "2",
			Status:       servicequestion.QuestionStatusEnabled,
		}, nil, nil); err != nil {
			t.Fatalf("CreateQuestion(%s) returned error: %v", item.title, err)
		}
	}

	result, err := repo.ListVisibleQuestions(t.Context(), servicequestion.ListQuestionsInput{TenantID: 10, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListVisibleQuestions returned error: %v", err)
	}
	got := make([]string, 0, len(result.Items))
	for _, item := range result.Items {
		got = append(got, item.Title)
	}
	want := []string{"教师上传的空间题", "租户公共题"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("question titles = %#v, want %#v", got, want)
	}
	if result.Total != 2 {
		t.Fatalf("question total = %d, want 2", result.Total)
	}
}

func TestQuestionRepositoryUpdateQuestionCanMoveToPublicScope(t *testing.T) {
	gormDB := openQuestionRepositoryTestDB(t)
	repo := NewQuestionRepository(gormDB, QuestionRepositoryOptions{Now: func() int64 { return 1000 }})
	spaceID := uint64(301)

	created, err := repo.CreateQuestion(t.Context(), servicequestion.Question{
		TenantID:     10,
		SpaceID:      &spaceID,
		Type:         servicequestion.QuestionTypeSingle,
		Difficulty:   servicequestion.DifficultyMedium,
		Title:        "空间题",
		ScoreDefault: "2",
		Status:       servicequestion.QuestionStatusEnabled,
	}, []servicequestion.QuestionOption{
		{OptionKey: "A", Content: "对", IsCorrect: true},
		{OptionKey: "B", Content: "错"},
	}, []string{"函数"})
	if err != nil {
		t.Fatalf("CreateQuestion returned error: %v", err)
	}

	updated, err := repo.UpdateQuestion(t.Context(), servicequestion.Question{
		ID:           created.ID,
		TenantID:     10,
		SpaceID:      nil,
		Type:         servicequestion.QuestionTypeSingle,
		Difficulty:   servicequestion.DifficultyHard,
		Title:        "公共题",
		ScoreDefault: "3",
		Status:       servicequestion.QuestionStatusEnabled,
		CreatedBy:    9,
	}, []servicequestion.QuestionOption{
		{OptionKey: "A", Content: "对", IsCorrect: true},
		{OptionKey: "B", Content: "错"},
	}, []string{"公共"})
	if err != nil {
		t.Fatalf("UpdateQuestion returned error: %v", err)
	}
	if updated.SpaceID != nil {
		t.Fatalf("expected updated question to move to public scope, got %#v", updated.SpaceID)
	}
}

func TestQuestionRepositorySearchesVisibleQuestionsAcrossTagsAndOptions(t *testing.T) {
	gormDB := openQuestionRepositoryTestDB(t)
	now := int64(1000)
	repo := NewQuestionRepository(gormDB, QuestionRepositoryOptions{Now: func() int64 { return now }})

	for _, item := range []struct {
		title   string
		options []servicequestion.QuestionOption
		tags    []string
		now     int64
	}{
		{
			title: "常规函数题",
			options: []servicequestion.QuestionOption{
				{OptionKey: "A", SortOrder: 1, Content: "一次函数", IsCorrect: true},
				{OptionKey: "B", SortOrder: 2, Content: "二次函数", IsDistractor: true},
			},
			tags: []string{"函数"},
			now:  1000,
		},
		{
			title: "阅读理解题",
			options: []servicequestion.QuestionOption{
				{OptionKey: "A", SortOrder: 1, Content: "普通段落", IsCorrect: true},
				{OptionKey: "B", SortOrder: 2, Content: "压轴选项", IsDistractor: true},
			},
			tags: []string{"阅读理解", "压轴题"},
			now:  2000,
		},
	} {
		now = item.now
		if _, err := repo.CreateQuestion(t.Context(), servicequestion.Question{
			TenantID:     10,
			Type:         servicequestion.QuestionTypeSingle,
			Difficulty:   servicequestion.DifficultyMedium,
			Title:        item.title,
			ScoreDefault: "2",
			Status:       servicequestion.QuestionStatusEnabled,
		}, item.options, item.tags); err != nil {
			t.Fatalf("CreateQuestion(%s) returned error: %v", item.title, err)
		}
	}

	result, err := repo.ListVisibleQuestions(t.Context(), servicequestion.ListQuestionsInput{
		TenantID: 10,
		Page:     1,
		PageSize: 10,
		Search:   "压轴",
	})
	if err != nil {
		t.Fatalf("ListVisibleQuestions returned error: %v", err)
	}
	if result.Total != 1 || len(result.Items) != 1 {
		t.Fatalf("expected one search result, got total=%d items=%#v", result.Total, result.Items)
	}
	if result.Items[0].Title != "阅读理解题" {
		t.Fatalf("expected search to match tag or option content, got %#v", result.Items[0])
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

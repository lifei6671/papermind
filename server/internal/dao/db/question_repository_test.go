package db

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/lifei6671/papermind/server/bootstrap/migration"
	servicequestion "github.com/lifei6671/papermind/server/internal/service/question"
	servicespace "github.com/lifei6671/papermind/server/internal/service/space"
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
	seedQuestionSpace(t, gormDB, spaceID, "enabled")

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

func TestQuestionRepositoryPublicScopeFiltersBeforePaginating(t *testing.T) {
	gormDB := openQuestionRepositoryTestDB(t)
	now := int64(1000)
	repo := NewQuestionRepository(gormDB, QuestionRepositoryOptions{Now: func() int64 { return now }})
	spaceID := uint64(301)
	seedQuestionSpace(t, gormDB, spaceID, "enabled")

	for _, item := range []struct {
		title   string
		spaceID *uint64
		now     int64
	}{
		{title: "空间题不应进入公共候选池", spaceID: &spaceID, now: 3000},
		{title: "公共题第二页", spaceID: nil, now: 2000},
		{title: "公共题第一页", spaceID: nil, now: 1000},
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

	result, err := repo.ListVisibleQuestions(t.Context(), servicequestion.ListQuestionsInput{
		TenantID: 10,
		Scope:    "public",
		Page:     1,
		PageSize: 1,
	})
	if err != nil {
		t.Fatalf("ListVisibleQuestions returned error: %v", err)
	}
	if result.Total != 2 || len(result.Items) != 1 {
		t.Fatalf("expected two public questions before pagination, got total=%d items=%#v", result.Total, result.Items)
	}
	if result.Items[0].Title != "公共题第二页" {
		t.Fatalf("expected newest public question first, got %#v", result.Items[0])
	}
}

func TestQuestionRepositoryListVisibleQuestionsExcludesDisabledSpaceQuestions(t *testing.T) {
	gormDB := openQuestionRepositoryTestDB(t)
	seedQuestionSpace(t, gormDB, 301, "disabled")
	seedQuestionRow(t, gormDB, 10, 301, "禁用空间题目", 2000)
	now := int64(1000)
	repo := NewQuestionRepository(gormDB, QuestionRepositoryOptions{Now: func() int64 { return now }})

	for _, item := range []struct {
		title   string
		spaceID *uint64
		now     int64
	}{
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
	if result.Total != 1 || len(result.Items) != 1 {
		t.Fatalf("expected only public question, got total=%d items=%#v", result.Total, result.Items)
	}
	if result.Items[0].Title != "租户公共题" {
		t.Fatalf("expected disabled space question to be hidden, got %#v", result.Items[0])
	}
}

func TestQuestionRepositoryUpdateQuestionCanMoveToPublicScope(t *testing.T) {
	gormDB := openQuestionRepositoryTestDB(t)
	repo := NewQuestionRepository(gormDB, QuestionRepositoryOptions{Now: func() int64 { return 1000 }})
	spaceID := uint64(301)
	seedQuestionSpace(t, gormDB, spaceID, "enabled")

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

func TestQuestionRepositoryAppliesStructuredFiltersBeforePaginating(t *testing.T) {
	gormDB := openQuestionRepositoryTestDB(t)
	repo := NewQuestionRepository(gormDB, QuestionRepositoryOptions{Now: func() int64 { return 1000 }})

	for _, item := range []struct {
		title        string
		questionType string
		difficulty   string
		status       string
		tags         []string
	}{
		{title: "函数基础题", questionType: servicequestion.QuestionTypeSingle, difficulty: servicequestion.DifficultyMedium, status: servicequestion.QuestionStatusEnabled, tags: []string{"函数"}},
		{title: "函数进阶题", questionType: servicequestion.QuestionTypeSingle, difficulty: servicequestion.DifficultyMedium, status: servicequestion.QuestionStatusEnabled, tags: []string{"函数"}},
		{title: "函数草稿题", questionType: servicequestion.QuestionTypeSingle, difficulty: servicequestion.DifficultyMedium, status: servicequestion.QuestionStatusDraft, tags: []string{"函数"}},
		{title: "阅读题", questionType: servicequestion.QuestionTypeShortText, difficulty: servicequestion.DifficultyMedium, status: servicequestion.QuestionStatusEnabled, tags: []string{"阅读"}},
	} {
		if _, err := repo.CreateQuestion(t.Context(), servicequestion.Question{
			TenantID:     10,
			Type:         item.questionType,
			Difficulty:   item.difficulty,
			Title:        item.title,
			ScoreDefault: "2",
			Status:       item.status,
		}, nil, item.tags); err != nil {
			t.Fatalf("CreateQuestion(%s) returned error: %v", item.title, err)
		}
	}

	result, err := repo.ListVisibleQuestions(t.Context(), servicequestion.ListQuestionsInput{
		TenantID:   10,
		Page:       1,
		PageSize:   1,
		Type:       servicequestion.QuestionTypeSingle,
		Difficulty: servicequestion.DifficultyMedium,
		Tag:        "函数",
		Status:     servicequestion.QuestionStatusEnabled,
	})
	if err != nil {
		t.Fatalf("ListVisibleQuestions returned error: %v", err)
	}
	if result.Total != 2 || len(result.Items) != 1 {
		t.Fatalf("expected filtered total=2 and one paged item, got %#v", result)
	}
	if result.Items[0].Title != "函数进阶题" {
		t.Fatalf("expected newest matching question first, got %#v", result.Items[0])
	}
}

func TestQuestionRepositoryCountsAvailableQuestionsByTypeInDatabase(t *testing.T) {
	gormDB := openQuestionRepositoryTestDB(t)
	spaceID := uint64(301)
	seedQuestionSpace(t, gormDB, spaceID, servicespace.StatusEnabled)
	repo := NewQuestionRepository(gormDB, QuestionRepositoryOptions{Now: func() int64 { return 1000 }})

	createdIDs := make(map[string]uint64)
	for _, item := range []struct {
		key          string
		title        string
		spaceID      *uint64
		questionType string
		tags         []string
	}{
		{key: "blocked", title: "已选单选题", spaceID: &spaceID, questionType: servicequestion.QuestionTypeSingle, tags: []string{"函数"}},
		{key: "single", title: "可用单选题", spaceID: &spaceID, questionType: servicequestion.QuestionTypeSingle, tags: []string{"函数"}},
		{key: "public", title: "公共单选题", spaceID: nil, questionType: servicequestion.QuestionTypeSingle, tags: []string{"函数"}},
		{key: "judge", title: "可用判断题", spaceID: &spaceID, questionType: servicequestion.QuestionTypeJudge, tags: []string{"函数"}},
		{key: "other-tag", title: "其他标签题", spaceID: &spaceID, questionType: servicequestion.QuestionTypeSingle, tags: []string{"阅读"}},
	} {
		created, err := repo.CreateQuestion(t.Context(), servicequestion.Question{
			TenantID:     10,
			SpaceID:      item.spaceID,
			Type:         item.questionType,
			Difficulty:   servicequestion.DifficultyMedium,
			Title:        item.title,
			ScoreDefault: "2",
			Status:       servicequestion.QuestionStatusEnabled,
		}, nil, item.tags)
		if err != nil {
			t.Fatalf("CreateQuestion(%s) returned error: %v", item.title, err)
		}
		createdIDs[item.key] = created.ID
	}

	counts, err := repo.CountVisibleQuestionsByType(t.Context(), servicequestion.QuestionAvailabilityInput{
		TenantID:           10,
		SpaceID:            &spaceID,
		Status:             servicequestion.QuestionStatusEnabled,
		Tags:               []string{"函数"},
		ExcludeQuestionIDs: []uint64{createdIDs["blocked"]},
	})
	if err != nil {
		t.Fatalf("CountVisibleQuestionsByType returned error: %v", err)
	}
	if counts[servicequestion.QuestionTypeSingle] != 2 {
		t.Fatalf("expected two available single questions, got %#v", counts)
	}
	if counts[servicequestion.QuestionTypeJudge] != 1 {
		t.Fatalf("expected one available judge question, got %#v", counts)
	}
}

func TestQuestionRepositoryListsVisibleTagsByScope(t *testing.T) {
	gormDB := openQuestionRepositoryTestDB(t)
	spaceID := uint64(301)
	disabledSpaceID := uint64(302)
	seedQuestionSpace(t, gormDB, spaceID, servicespace.StatusEnabled)
	seedQuestionSpace(t, gormDB, disabledSpaceID, servicespace.StatusDisabled)
	repo := NewQuestionRepository(gormDB, QuestionRepositoryOptions{Now: func() int64 { return 1000 }})

	for _, item := range []struct {
		title   string
		spaceID *uint64
		tags    []string
	}{
		{title: "空间函数题", spaceID: &spaceID, tags: []string{"函数", "高一"}},
		{title: "公共函数题", spaceID: nil, tags: []string{"函数压轴"}},
	} {
		if _, err := repo.CreateQuestion(t.Context(), servicequestion.Question{
			TenantID:     10,
			SpaceID:      item.spaceID,
			Type:         servicequestion.QuestionTypeSingle,
			Difficulty:   servicequestion.DifficultyMedium,
			Title:        item.title,
			ScoreDefault: "2",
			Status:       servicequestion.QuestionStatusEnabled,
		}, nil, item.tags); err != nil {
			t.Fatalf("CreateQuestion(%s) returned error: %v", item.title, err)
		}
	}
	if err := gormDB.Exec(`
		INSERT INTO questions (
			id, tenant_id, space_id, type, difficulty, title, score_default, status,
			created_at, created_by_type, updated_at, updated_by_type, version, ext_json
		) VALUES (999, 10, ?, ?, ?, '禁用空间题', '2', ?, 1000, ?, 1000, ?, 1, '{}')
	`,
		disabledSpaceID,
		servicequestion.QuestionTypeSingle,
		servicequestion.DifficultyMedium,
		servicequestion.QuestionStatusEnabled,
		AuditActorTenantUser,
		AuditActorTenantUser,
	).Error; err != nil {
		t.Fatalf("seed disabled space question: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO tags (
			id, tenant_id, name, created_at, updated_at, ext_json
		) VALUES (999, 10, '隐藏标签', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed hidden tag: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO question_tags (
			tenant_id, question_id, tag_id, created_at, ext_json
		) VALUES (10, 999, 999, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed hidden question tag: %v", err)
	}

	tags, err := repo.ListVisibleQuestionTags(t.Context(), servicequestion.QuestionTagListInput{
		TenantID: 10,
		SpaceID:  &spaceID,
		Status:   servicequestion.QuestionStatusEnabled,
		Search:   "函数",
	})
	if err != nil {
		t.Fatalf("ListVisibleQuestionTags returned error: %v", err)
	}
	assertStrings(t, tags, []string{"函数", "函数压轴"})
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

func seedQuestionSpace(t *testing.T, gormDB *gorm.DB, spaceID uint64, status string) {
	t.Helper()
	if err := gormDB.Exec(`
		INSERT INTO spaces (
			id, tenant_id, name, logo_url, description, type, status,
			created_at, updated_at, ext_json
		) VALUES (?, 10, '题库测试空间', '', '', 'class', ?, 1000, 1000, '{}')
	`, spaceID, status).Error; err != nil {
		t.Fatalf("seed question space %d: %v", spaceID, err)
	}
}

func seedQuestionRow(t *testing.T, gormDB *gorm.DB, tenantID uint64, spaceID uint64, title string, createdAt int64) {
	t.Helper()
	if err := gormDB.Exec(`
		INSERT INTO questions (
			tenant_id, space_id, type, difficulty, title, score_default, status,
			created_at, created_by_type, updated_at, updated_by_type, version, ext_json
		) VALUES (?, ?, ?, ?, ?, '2', ?, ?, ?, ?, ?, 1, '{}')
	`,
		tenantID,
		spaceID,
		servicequestion.QuestionTypeShortText,
		servicequestion.DifficultyMedium,
		title,
		servicequestion.QuestionStatusEnabled,
		createdAt,
		AuditActorTenantUser,
		createdAt,
		AuditActorTenantUser,
	).Error; err != nil {
		t.Fatalf("seed question row %q: %v", title, err)
	}
}

func assertStrings(t *testing.T, got []string, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
}

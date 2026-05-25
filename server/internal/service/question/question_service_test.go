package question

import (
	"context"
	"errors"
	"testing"
)

func TestCreateQuestionSupportsAllTypesAndBaseFields(t *testing.T) {
	repo := &fakeQuestionRepository{}
	svc := NewQuestionService(QuestionServiceOptions{Repo: repo})
	spaceID := uint64(100)

	cases := []struct {
		name  string
		input CreateQuestionInput
	}{
		{
			name: "single",
			input: CreateQuestionInput{
				TenantID:     10,
				SpaceID:      &spaceID,
				Type:         QuestionTypeSingle,
				Difficulty:   DifficultyEasy,
				Title:        "1+1 等于几？",
				Analysis:     "",
				ScoreDefault: "2.5",
				Options: []QuestionOptionInput{
					{OptionKey: "A", Content: "2", IsCorrect: true},
					{OptionKey: "B", Content: "3", IsDistractor: true},
				},
			},
		},
		{
			name: "multiple",
			input: CreateQuestionInput{
				TenantID:     10,
				Type:         QuestionTypeMultiple,
				Difficulty:   DifficultyMedium,
				Title:        "哪些是偶数？",
				Analysis:     "能被 2 整除。",
				ScoreDefault: "4",
				Options: []QuestionOptionInput{
					{OptionKey: "A", Content: "2", IsCorrect: true},
					{OptionKey: "B", Content: "4", IsCorrect: true},
					{OptionKey: "C", Content: "5", IsDistractor: true},
				},
			},
		},
		{
			name: "judge",
			input: CreateQuestionInput{
				TenantID:       10,
				Type:           QuestionTypeJudge,
				Difficulty:     DifficultyEasy,
				Title:          "地球是圆的。",
				ScoreDefault:   "1",
				StandardAnswer: "true",
			},
		},
		{
			name: "fill blank",
			input: CreateQuestionInput{
				TenantID:       10,
				Type:           QuestionTypeFillBlank,
				Difficulty:     DifficultyHard,
				Title:          "Go 的包管理文件是 ____。",
				ScoreDefault:   "3",
				StandardAnswer: "go.mod",
			},
		},
		{
			name: "short text",
			input: CreateQuestionInput{
				TenantID:        10,
				Type:            QuestionTypeShortText,
				Difficulty:      DifficultyMedium,
				Title:           "简述事务隔离的意义。",
				ScoreDefault:    "8",
				ReferenceAnswer: "避免并发读写互相污染。",
			},
		},
	}

	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			created, err := svc.CreateQuestion(context.Background(), item.input)
			if err != nil {
				t.Fatalf("CreateQuestion returned error: %v", err)
			}
			if created.ID == 0 {
				t.Fatalf("expected created question ID")
			}
			if repo.createdQuestion.Type != item.input.Type {
				t.Fatalf("expected type %q, got %q", item.input.Type, repo.createdQuestion.Type)
			}
			if repo.createdQuestion.Analysis != item.input.Analysis {
				t.Fatalf("expected optional analysis %q, got %q", item.input.Analysis, repo.createdQuestion.Analysis)
			}
			if repo.createdQuestion.Difficulty != item.input.Difficulty {
				t.Fatalf("expected difficulty %q, got %q", item.input.Difficulty, repo.createdQuestion.Difficulty)
			}
			if repo.createdQuestion.ScoreDefault != item.input.ScoreDefault {
				t.Fatalf("expected score %q, got %q", item.input.ScoreDefault, repo.createdQuestion.ScoreDefault)
			}
			if item.name == "single" && repo.createdQuestion.SpaceID == nil {
				t.Fatalf("expected space question to keep space ID")
			}
			if item.name == "multiple" && repo.createdQuestion.SpaceID != nil {
				t.Fatalf("expected public question to use nil space ID")
			}
		})
	}
}

func TestListVisibleQuestionsUsesPublicAndSpaceScope(t *testing.T) {
	spaceID := uint64(100)
	repo := &fakeQuestionRepository{
		visibleQuestions: []Question{
			{ID: 1, TenantID: 10, SpaceID: nil, Title: "公共题"},
			{ID: 2, TenantID: 10, SpaceID: &spaceID, Title: "空间题"},
		},
	}
	svc := NewQuestionService(QuestionServiceOptions{Repo: repo})

	questions, err := svc.ListVisibleQuestions(context.Background(), 10, &spaceID)
	if err != nil {
		t.Fatalf("ListVisibleQuestions returned error: %v", err)
	}
	if !repo.listVisibleCalled {
		t.Fatalf("expected visibility repository query to be called")
	}
	if len(questions) != 2 {
		t.Fatalf("expected public and space questions, got %#v", questions)
	}
}

func TestChoiceQuestionValidation(t *testing.T) {
	svc := NewQuestionService(QuestionServiceOptions{Repo: &fakeQuestionRepository{}})

	_, err := svc.CreateQuestion(context.Background(), CreateQuestionInput{
		TenantID: 10,
		Type:     QuestionTypeSingle,
		Title:    "没有正确答案",
		Options: []QuestionOptionInput{
			{OptionKey: "A", Content: "错"},
		},
	})
	if !errors.Is(err, ErrChoiceQuestionNeedsCorrectAnswer) {
		t.Fatalf("expected ErrChoiceQuestionNeedsCorrectAnswer, got %v", err)
	}

	_, err = svc.CreateQuestion(context.Background(), CreateQuestionInput{
		TenantID: 10,
		Type:     QuestionTypeSingle,
		Title:    "两个正确答案",
		Options: []QuestionOptionInput{
			{OptionKey: "A", Content: "对", IsCorrect: true},
			{OptionKey: "B", Content: "也对", IsCorrect: true},
		},
	})
	if !errors.Is(err, ErrSingleQuestionOnlyOneCorrectAnswer) {
		t.Fatalf("expected ErrSingleQuestionOnlyOneCorrectAnswer, got %v", err)
	}

	_, err = svc.CreateQuestion(context.Background(), CreateQuestionInput{
		TenantID: 10,
		Type:     QuestionTypeMultiple,
		Title:    "多个正确答案",
		Options: []QuestionOptionInput{
			{OptionKey: "A", Content: "对", IsCorrect: true},
			{OptionKey: "B", Content: "也对", IsCorrect: true},
		},
	})
	if err != nil {
		t.Fatalf("expected multiple choice to allow multiple correct answers, got %v", err)
	}
}

func TestChoiceOptionKeyIsNotUsedForGradingAndIDsAreNormalized(t *testing.T) {
	svc := NewQuestionService(QuestionServiceOptions{Repo: &fakeQuestionRepository{}})
	snapshot := ChoiceSnapshot{
		Options: []ChoiceSnapshotOption{
			{ID: 30, OptionKey: "A", IsCorrect: true},
			{ID: 10, OptionKey: "B", IsCorrect: true},
			{ID: 20, OptionKey: "C", IsCorrect: false},
		},
	}

	if !svc.GradeChoiceAnswer(snapshot, []uint64{10, 30}) {
		t.Fatalf("expected choice grading to use normalized option IDs")
	}
	if svc.GradeChoiceAnswer(snapshot, []uint64{20, 30}) {
		t.Fatalf("expected wrong option IDs to fail grading")
	}
	if !svc.GradeChoiceSnapshotAnswer(snapshot, []ChoiceSnapshotAnswer{{OptionID: 30, OptionKey: "Z"}, {OptionID: 10, OptionKey: "Y"}}) {
		t.Fatalf("expected snapshot grading to ignore option_key")
	}
}

func TestChoiceDisplayCountAndDistractorArePersisted(t *testing.T) {
	repo := &fakeQuestionRepository{}
	svc := NewQuestionService(QuestionServiceOptions{Repo: repo})
	displayCount := 4

	_, err := svc.CreateQuestion(context.Background(), CreateQuestionInput{
		TenantID:           10,
		Type:               QuestionTypeSingle,
		Title:              "选项展示数量",
		ChoiceDisplayCount: &displayCount,
		Options: []QuestionOptionInput{
			{OptionKey: "A", Content: "对", IsCorrect: true},
			{OptionKey: "B", Content: "干扰项", IsDistractor: true},
		},
	})
	if err != nil {
		t.Fatalf("CreateQuestion returned error: %v", err)
	}
	if repo.createdQuestion.ChoiceDisplayCount == nil || *repo.createdQuestion.ChoiceDisplayCount != 4 {
		t.Fatalf("expected choice display count 4, got %#v", repo.createdQuestion.ChoiceDisplayCount)
	}
	if len(repo.createdOptions) != 2 || !repo.createdOptions[1].IsDistractor {
		t.Fatalf("expected distractor option persisted, got %#v", repo.createdOptions)
	}
}

func TestReplaceOptionsUsesFullReplacementInTransaction(t *testing.T) {
	repo := &fakeQuestionRepository{}
	svc := NewQuestionService(QuestionServiceOptions{Repo: repo})

	err := svc.ReplaceOptions(context.Background(), ReplaceOptionsInput{
		TenantID:   10,
		QuestionID: 100,
		Options: []QuestionOptionInput{
			{OptionKey: "A", Content: "新正确答案", IsCorrect: true},
			{OptionKey: "B", Content: "新干扰项", IsDistractor: true},
		},
	})
	if err != nil {
		t.Fatalf("ReplaceOptions returned error: %v", err)
	}
	if !repo.replaceOptionsInTransaction {
		t.Fatalf("expected full option replacement in one repository transaction")
	}
	if repo.replacedQuestionID != 100 || len(repo.replacedOptions) != 2 {
		t.Fatalf("expected replaced options for question 100, got question=%d options=%#v", repo.replacedQuestionID, repo.replacedOptions)
	}
}

func TestFillBlankAndShortTextRules(t *testing.T) {
	svc := NewQuestionService(QuestionServiceOptions{Repo: &fakeQuestionRepository{}})

	_, err := svc.CreateQuestion(context.Background(), CreateQuestionInput{
		TenantID:       10,
		Type:           QuestionTypeFillBlank,
		Title:          "多空题",
		ScoreDefault:   "3",
		StandardAnswer: "A|B",
		BlankCount:     2,
	})
	if !errors.Is(err, ErrFillBlankOnlySupportsSingleBlank) {
		t.Fatalf("expected ErrFillBlankOnlySupportsSingleBlank, got %v", err)
	}
	if !svc.GradeFillBlankAnswer(" go.mod ", "go.mod") {
		t.Fatalf("expected fill blank grading to trim answer before exact match")
	}
	if svc.GradeFillBlankAnswer("go.sum", "go.mod") {
		t.Fatalf("expected different fill blank answer to fail")
	}

	created, err := svc.CreateQuestion(context.Background(), CreateQuestionInput{
		TenantID:        10,
		Type:            QuestionTypeShortText,
		Title:           "简答",
		ScoreDefault:    "5",
		ReferenceAnswer: "参考答案",
	})
	if err != nil {
		t.Fatalf("CreateQuestion short text returned error: %v", err)
	}
	if created.GradingMode != GradingModeManual {
		t.Fatalf("expected short text manual grading, got %q", created.GradingMode)
	}
}

func TestImportTemplateAndImportRows(t *testing.T) {
	repo := &fakeQuestionRepository{}
	svc := NewQuestionService(QuestionServiceOptions{Repo: repo})

	headers := svc.ImportTemplateHeaders()
	wantHeaders := []string{"type", "title", "options", "correct_answer", "analysis", "difficulty", "tags"}
	for i, want := range wantHeaders {
		if headers[i] != want {
			t.Fatalf("expected header %d to be %q, got %q", i, want, headers[i])
		}
	}

	result, err := svc.ImportQuestions(context.Background(), ImportQuestionsInput{
		TenantID: 10,
		Rows: []ImportRow{
			{RowNumber: 2, Type: QuestionTypeSingle, Title: "合法题", Options: "A.对|B.错", CorrectAnswer: "A", Difficulty: DifficultyEasy, Tags: "数学,基础"},
			{RowNumber: 3, Type: QuestionTypeSingle, Title: "错误题", Options: "A.错|B.也错", CorrectAnswer: "", Difficulty: DifficultyEasy},
		},
	})
	if err != nil {
		t.Fatalf("ImportQuestions returned error: %v", err)
	}
	if result.SuccessCount != 1 {
		t.Fatalf("expected one imported question, got %d", result.SuccessCount)
	}
	if len(result.Errors) != 1 || result.Errors[0].RowNumber != 3 {
		t.Fatalf("expected row 3 import error, got %#v", result.Errors)
	}
	if len(repo.importedQuestions) != 1 || repo.importedQuestions[0].Title != "合法题" {
		t.Fatalf("expected valid question written, got %#v", repo.importedQuestions)
	}
	if len(repo.importedTags[0]) != 2 || repo.importedTags[0][0] != "数学" || repo.importedTags[0][1] != "基础" {
		t.Fatalf("expected imported tags, got %#v", repo.importedTags)
	}
}

type fakeQuestionRepository struct {
	createdQuestion Question
	createdOptions  []QuestionOption

	visibleQuestions  []Question
	listVisibleCalled bool

	replaceOptionsInTransaction bool
	replacedQuestionID          uint64
	replacedOptions             []QuestionOption

	importedQuestions []Question
	importedOptions   [][]QuestionOption
	importedTags      [][]string
}

func (r *fakeQuestionRepository) CreateQuestion(ctx context.Context, item Question, options []QuestionOption, tags []string) (Question, error) {
	item.ID = uint64(len(r.importedQuestions) + 1)
	r.createdQuestion = item
	r.createdOptions = options
	r.importedQuestions = append(r.importedQuestions, item)
	r.importedOptions = append(r.importedOptions, options)
	r.importedTags = append(r.importedTags, tags)
	return item, nil
}

func (r *fakeQuestionRepository) ListVisibleQuestions(ctx context.Context, tenantID uint64, spaceID *uint64) ([]Question, error) {
	r.listVisibleCalled = true
	return r.visibleQuestions, nil
}

func (r *fakeQuestionRepository) ReplaceOptionsInTransaction(ctx context.Context, tenantID uint64, questionID uint64, options []QuestionOption) error {
	r.replaceOptionsInTransaction = true
	r.replacedQuestionID = questionID
	r.replacedOptions = options
	return nil
}

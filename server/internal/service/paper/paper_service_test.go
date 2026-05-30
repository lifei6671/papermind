package paper

import (
	"context"
	"errors"
	"testing"

	"github.com/lifei6671/papermind/server/library/constant"
)

func TestSectionLifecycleUsesStableOrderAndCascadingDelete(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewService(ServiceOptions{Repo: repo})

	section, err := svc.CreateSection(context.Background(), CreateSectionInput{
		TenantID:     10,
		PaperID:      100,
		SortOrder:    1,
		Name:         "一、单选题",
		QuestionType: constant.QuestionTypeSingle,
		Instructions: "请选择一个正确答案",
	})
	if err != nil {
		t.Fatalf("CreateSection returned error: %v", err)
	}
	if section.ID != 1 {
		t.Fatalf("expected section ID 1, got %d", section.ID)
	}
	if !repo.checkedSectionSortOrder {
		t.Fatalf("expected unique section sort order check")
	}
	if repo.createdSection.Name != "一、单选题" || repo.createdSection.Instructions != "请选择一个正确答案" {
		t.Fatalf("expected section name and instructions saved, got %#v", repo.createdSection)
	}

	if err := svc.UpdateSection(context.Background(), UpdateSectionInput{
		TenantID:     10,
		SectionID:    1,
		Name:         "一、选择题",
		QuestionType: constant.QuestionTypeMultiple,
		Instructions: "请选择全部正确答案",
	}); err != nil {
		t.Fatalf("UpdateSection returned error: %v", err)
	}
	if repo.updatedSectionName != "一、选择题" || repo.updatedQuestionType != constant.QuestionTypeMultiple || repo.updatedInstructions != "请选择全部正确答案" {
		t.Fatalf("expected section fields updated, got name=%q type=%q instructions=%q", repo.updatedSectionName, repo.updatedQuestionType, repo.updatedInstructions)
	}

	if err := svc.ReorderSections(context.Background(), 10, 100, []SectionOrder{{SectionID: 1, SortOrder: 2}}); err != nil {
		t.Fatalf("ReorderSections returned error: %v", err)
	}
	if len(repo.sectionOrders) != 1 || repo.sectionOrders[0].SortOrder != 2 {
		t.Fatalf("expected section order update, got %#v", repo.sectionOrders)
	}

	if err := svc.DeleteSection(context.Background(), 10, 1); err != nil {
		t.Fatalf("DeleteSection returned error: %v", err)
	}
	if !repo.deletedSectionInTransaction || repo.deletedSectionID != 1 {
		t.Fatalf("expected section soft delete with child hard delete in one transaction")
	}

	if _, err := svc.ListSections(context.Background(), 10, 100); err != nil {
		t.Fatalf("ListSections returned error: %v", err)
	}
	if !repo.listActiveSectionsCalled {
		t.Fatalf("expected active section query to filter deleted_at = 0")
	}
}

func TestManualPaperAddQuestionPreventsDuplicatesAndRecalculates(t *testing.T) {
	repo := &fakeRepository{
		duplicatePaperQuestion: true,
	}
	svc := NewService(ServiceOptions{Repo: repo})

	paper, err := svc.CreateManualPaper(context.Background(), CreatePaperInput{
		TenantID:         10,
		Name:             "期中考试",
		ShuffleQuestions: true,
		ShowAnalysis:     true,
	})
	if err != nil {
		t.Fatalf("CreateManualPaper returned error: %v", err)
	}
	if paper.BuildMode != BuildModeManual || paper.Status != StatusDraft {
		t.Fatalf("expected manual draft paper, got %#v", paper)
	}

	err = svc.AddManualQuestion(context.Background(), AddSectionQuestionInput{
		TenantID:   10,
		PaperID:    100,
		SectionID:  200,
		QuestionID: 300,
		SortOrder:  1,
		Score:      "2.5",
	})
	if !errors.Is(err, ErrDuplicatePaperQuestion) {
		t.Fatalf("expected ErrDuplicatePaperQuestion, got %v", err)
	}

	repo.duplicatePaperQuestion = false
	repo.questionUsableForPaper = true
	shuffle := false
	err = svc.AddManualQuestion(context.Background(), AddSectionQuestionInput{
		TenantID:       10,
		PaperID:        100,
		SectionID:      200,
		QuestionID:     300,
		SortOrder:      2,
		Score:          "2.5",
		ShuffleOptions: &shuffle,
	})
	if err != nil {
		t.Fatalf("AddManualQuestion returned error: %v", err)
	}
	if !repo.addQuestionAndRecalculateInTransaction {
		t.Fatalf("expected add question and recalculation in one transaction")
	}
	if repo.addedQuestion.SortOrder != 2 || repo.addedQuestion.Score != "2.5" || repo.addedQuestion.ShuffleOptions == nil || *repo.addedQuestion.ShuffleOptions {
		t.Fatalf("expected question sort, score and nullable shuffle override saved, got %#v", repo.addedQuestion)
	}
	if repo.sectionTotalScore != "2.5" || repo.paperTotalScore != "2.5" {
		t.Fatalf("expected totals recalculated to 2.5, got section=%q paper=%q", repo.sectionTotalScore, repo.paperTotalScore)
	}
}

func TestRuleFixedGenerateAndReviewBehavesLikeManualAfterGeneration(t *testing.T) {
	repo := &fakeRepository{
		ruleMatches: map[uint64][]uint64{
			1: {300, 301},
		},
	}
	svc := NewService(ServiceOptions{Repo: repo})
	tagFilter := []uint64{7, 8}
	difficulty := "medium"

	rule, err := svc.ConfigureRule(context.Background(), ConfigureRuleInput{
		TenantID:         10,
		PaperID:          100,
		SectionID:        200,
		SortOrder:        1,
		Difficulty:       &difficulty,
		TagIDs:           tagFilter,
		QuestionCount:    2,
		ScorePerQuestion: "3",
		ShuffleOptions:   nil,
	})
	if err != nil {
		t.Fatalf("ConfigureRule returned error: %v", err)
	}
	if rule.TagFilter != "[7,8]" {
		t.Fatalf("expected stable JSON tag filter [7,8], got %q", rule.TagFilter)
	}

	noDifficultyRule, err := svc.ConfigureRule(context.Background(), ConfigureRuleInput{
		TenantID:         10,
		PaperID:          100,
		SectionID:        200,
		SortOrder:        2,
		Difficulty:       nil,
		TagIDs:           []uint64{8, 7},
		QuestionCount:    1,
		ScorePerQuestion: "1",
	})
	if err != nil {
		t.Fatalf("ConfigureRule without difficulty returned error: %v", err)
	}
	if noDifficultyRule.Difficulty != nil || noDifficultyRule.TagFilter != "[7,8]" {
		t.Fatalf("expected nil difficulty and stable tag filter, got %#v", noDifficultyRule)
	}

	if err := svc.GenerateRuleFixed(context.Background(), 10, 100); err != nil {
		t.Fatalf("GenerateRuleFixed returned error: %v", err)
	}
	if !repo.generatedFixedInTransaction || len(repo.generatedQuestions) != 2 {
		t.Fatalf("expected generated fixed questions in transaction, got %#v", repo.generatedQuestions)
	}

	if err := svc.ReplaceGeneratedQuestion(context.Background(), ReplaceGeneratedQuestionInput{
		TenantID:      10,
		PaperID:       100,
		SectionID:     200,
		OldQuestionID: 300,
		NewQuestionID: 399,
		Score:         "4",
		SortOrder:     1,
	}); err != nil {
		t.Fatalf("ReplaceGeneratedQuestion returned error: %v", err)
	}
	if repo.replacedOldQuestionID != 300 || repo.replacedNewQuestionID != 399 {
		t.Fatalf("expected generated question replacement, got old=%d new=%d", repo.replacedOldQuestionID, repo.replacedNewQuestionID)
	}

	if err := svc.AdjustGeneratedQuestion(context.Background(), AdjustGeneratedQuestionInput{
		TenantID:   10,
		PaperID:    100,
		SectionID:  200,
		QuestionID: 399,
		SortOrder:  3,
		Score:      "5",
	}); err != nil {
		t.Fatalf("AdjustGeneratedQuestion returned error: %v", err)
	}
	if repo.adjustedQuestionID != 399 || repo.adjustedSortOrder != 3 || repo.adjustedScore != "5" {
		t.Fatalf("expected adjusted generated question, got question=%d order=%d score=%q", repo.adjustedQuestionID, repo.adjustedSortOrder, repo.adjustedScore)
	}
}

func TestRuleLivePrecheckDedupeAndFreezePool(t *testing.T) {
	repo := &fakeRepository{
		ruleMatches: map[uint64][]uint64{
			1: {300, 301},
			2: {301, 302},
			3: {400},
		},
	}
	svc := NewService(ServiceOptions{Repo: repo})

	result, err := svc.PrecheckRuleLive(context.Background(), 10, 100, []Rule{
		{ID: 1, QuestionCount: 2},
		{ID: 2, QuestionCount: 1},
	})
	if err != nil {
		t.Fatalf("PrecheckRuleLive returned error: %v", err)
	}
	if len(result.CandidateQuestionIDs) != 3 {
		t.Fatalf("expected deduped candidates [300 301 302], got %#v", result.CandidateQuestionIDs)
	}

	_, err = svc.PrecheckRuleLive(context.Background(), 10, 100, []Rule{
		{ID: 3, QuestionCount: 2},
	})
	if !errors.Is(err, ErrQuestionPoolInsufficient) {
		t.Fatalf("expected ErrQuestionPoolInsufficient, got %v", err)
	}

	if err := svc.FreezeLiveQuestionPool(context.Background(), 10, 900, result); err != nil {
		t.Fatalf("FreezeLiveQuestionPool returned error: %v", err)
	}
	if !repo.frozenLivePool || len(repo.frozenQuestionIDs) != 3 {
		t.Fatalf("expected frozen live question pool, got %#v", repo.frozenQuestionIDs)
	}

	repo.ruleMatches[1] = []uint64{999}
	if got := repo.FrozenQuestionIDs(); len(got) != 3 || got[0] != 300 {
		t.Fatalf("expected frozen pool unaffected by later question bank changes, got %#v", got)
	}

	err = svc.UpdateRuleLiveRule(context.Background(), UpdateRuleInput{
		TenantID:   10,
		PaperID:    100,
		ExamID:     900,
		RuleID:     1,
		ExamFrozen: true,
	})
	if !errors.Is(err, ErrExamMustBeWithdrawnBeforeRuleChange) {
		t.Fatalf("expected ErrExamMustBeWithdrawnBeforeRuleChange, got %v", err)
	}
}

func TestAggregateRecalculationByBuildMode(t *testing.T) {
	repo := &fakeRepository{
		manualSectionQuestions: []SectionQuestion{
			{SectionID: 200, Score: "2.5"},
			{SectionID: 200, Score: "3.5"},
		},
		ruleLiveRules: []Rule{
			{ID: 1, SectionID: 201, QuestionCount: 2, ScorePerQuestion: "4"},
			{ID: 2, SectionID: 201, QuestionCount: 1, ScorePerQuestion: "3"},
		},
	}
	svc := NewService(ServiceOptions{Repo: repo})

	if err := svc.RecalculatePaper(context.Background(), 10, 100, BuildModeManual); err != nil {
		t.Fatalf("RecalculatePaper manual returned error: %v", err)
	}
	if !repo.recalculatedInTransaction || repo.sectionTotalScore != "6" || repo.sectionQuestionCount != 2 || repo.paperTotalScore != "6" {
		t.Fatalf("expected manual totals section=6 count=2 paper=6, got section=%q count=%d paper=%q", repo.sectionTotalScore, repo.sectionQuestionCount, repo.paperTotalScore)
	}

	if err := svc.RecalculatePaper(context.Background(), 10, 100, BuildModeRuleLive); err != nil {
		t.Fatalf("RecalculatePaper rule_live returned error: %v", err)
	}
	if repo.sectionTotalScore != "11" || repo.sectionQuestionCount != 3 || repo.paperTotalScore != "11" {
		t.Fatalf("expected rule_live totals section=11 count=3 paper=11, got section=%q count=%d paper=%q", repo.sectionTotalScore, repo.sectionQuestionCount, repo.paperTotalScore)
	}
}

type fakeRepository struct {
	checkedSectionSortOrder     bool
	createdSection              Section
	updatedSectionName          string
	updatedQuestionType         string
	updatedInstructions         string
	sectionOrders               []SectionOrder
	deletedSectionInTransaction bool
	deletedSectionID            uint64
	listActiveSectionsCalled    bool

	createdPaper Paper

	duplicatePaperQuestion                 bool
	questionUsableForPaper                 bool
	addQuestionAndRecalculateInTransaction bool
	addedQuestion                          SectionQuestion

	ruleMatches                 map[uint64][]uint64
	configuredRule              Rule
	generatedFixedInTransaction bool
	generatedQuestions          []SectionQuestion
	replacedOldQuestionID       uint64
	replacedNewQuestionID       uint64
	adjustedQuestionID          uint64
	adjustedSortOrder           int
	adjustedScore               string

	frozenLivePool    bool
	frozenQuestionIDs []uint64

	manualSectionQuestions    []SectionQuestion
	ruleLiveRules             []Rule
	recalculatedInTransaction bool
	sectionTotalScore         string
	sectionQuestionCount      int
	paperTotalScore           string
}

func (r *fakeRepository) SectionSortOrderExists(ctx context.Context, tenantID uint64, paperID uint64, sortOrder int) (bool, error) {
	r.checkedSectionSortOrder = true
	return false, nil
}

func (r *fakeRepository) ListPapers(ctx context.Context, input ListPapersInput) ([]Paper, error) {
	return nil, nil
}

func (r *fakeRepository) CreateSection(ctx context.Context, section Section) (Section, error) {
	section.ID = 1
	r.createdSection = section
	return section, nil
}

func (r *fakeRepository) UpdateSection(ctx context.Context, input UpdateSectionInput) error {
	r.updatedSectionName = input.Name
	r.updatedQuestionType = input.QuestionType
	r.updatedInstructions = input.Instructions
	return nil
}

func (r *fakeRepository) ReorderSections(ctx context.Context, tenantID uint64, paperID uint64, orders []SectionOrder) error {
	r.sectionOrders = orders
	return nil
}

func (r *fakeRepository) DeleteSectionCascade(ctx context.Context, tenantID uint64, sectionID uint64) error {
	r.deletedSectionInTransaction = true
	r.deletedSectionID = sectionID
	return nil
}

func (r *fakeRepository) ListActiveSections(ctx context.Context, tenantID uint64, paperID uint64) ([]Section, error) {
	r.listActiveSectionsCalled = true
	return []Section{{ID: 1, TenantID: tenantID, PaperID: paperID}}, nil
}

func (r *fakeRepository) CreatePaper(ctx context.Context, paper Paper) (Paper, error) {
	paper.ID = 100
	r.createdPaper = paper
	return paper, nil
}

func (r *fakeRepository) DeletePaper(ctx context.Context, tenantID uint64, paperID uint64) error {
	return nil
}

func (r *fakeRepository) PaperQuestionExists(ctx context.Context, tenantID uint64, paperID uint64, questionID uint64) (bool, error) {
	return r.duplicatePaperQuestion, nil
}

func (r *fakeRepository) QuestionUsableForPaper(ctx context.Context, tenantID uint64, paperID uint64, questionID uint64) (bool, error) {
	return r.questionUsableForPaper, nil
}

func (r *fakeRepository) AddSectionQuestionAndRecalculate(ctx context.Context, question SectionQuestion) error {
	r.addQuestionAndRecalculateInTransaction = true
	r.addedQuestion = question
	r.sectionTotalScore = question.Score
	r.sectionQuestionCount = 1
	r.paperTotalScore = question.Score
	return nil
}

func (r *fakeRepository) CreateRule(ctx context.Context, rule Rule) (Rule, error) {
	rule.ID = uint64(rule.SortOrder)
	r.configuredRule = rule
	return rule, nil
}

func (r *fakeRepository) MatchQuestionsForRule(ctx context.Context, tenantID uint64, paperID uint64, rule Rule) ([]uint64, error) {
	return r.ruleMatches[rule.ID], nil
}

func (r *fakeRepository) ListRules(ctx context.Context, tenantID uint64, paperID uint64) ([]Rule, error) {
	rules := make([]Rule, 0, len(r.ruleMatches))
	for id := range r.ruleMatches {
		rules = append(rules, Rule{ID: id, SectionID: 200, PaperID: paperID, QuestionCount: len(r.ruleMatches[id]), ScorePerQuestion: "3"})
	}
	return rules, nil
}

func (r *fakeRepository) GenerateFixedQuestionsAndRecalculate(ctx context.Context, tenantID uint64, paperID uint64, questions []SectionQuestion) error {
	r.generatedFixedInTransaction = true
	r.generatedQuestions = questions
	return nil
}

func (r *fakeRepository) ReplaceGeneratedQuestionAndRecalculate(ctx context.Context, input ReplaceGeneratedQuestionInput) error {
	r.replacedOldQuestionID = input.OldQuestionID
	r.replacedNewQuestionID = input.NewQuestionID
	return nil
}

func (r *fakeRepository) AdjustGeneratedQuestionAndRecalculate(ctx context.Context, input AdjustGeneratedQuestionInput) error {
	r.adjustedQuestionID = input.QuestionID
	r.adjustedSortOrder = input.SortOrder
	r.adjustedScore = input.Score
	return nil
}

func (r *fakeRepository) FreezeLivePools(ctx context.Context, tenantID uint64, examID uint64, questionIDs []uint64) error {
	r.frozenLivePool = true
	r.frozenQuestionIDs = append([]uint64(nil), questionIDs...)
	return nil
}

func (r *fakeRepository) FrozenQuestionIDs() []uint64 {
	return append([]uint64(nil), r.frozenQuestionIDs...)
}

func (r *fakeRepository) UpdateRule(ctx context.Context, input UpdateRuleInput) error {
	return nil
}

func (r *fakeRepository) ListSectionQuestions(ctx context.Context, tenantID uint64, paperID uint64) ([]SectionQuestion, error) {
	return r.manualSectionQuestions, nil
}

func (r *fakeRepository) ListRuleLiveRules(ctx context.Context, tenantID uint64, paperID uint64) ([]Rule, error) {
	return r.ruleLiveRules, nil
}

func (r *fakeRepository) SaveAggregates(ctx context.Context, tenantID uint64, paperID uint64, sections []SectionAggregate, paperTotalScore string) error {
	r.recalculatedInTransaction = true
	if len(sections) > 0 {
		r.sectionTotalScore = sections[0].TotalScore
		r.sectionQuestionCount = sections[0].QuestionCount
	}
	r.paperTotalScore = paperTotalScore
	return nil
}

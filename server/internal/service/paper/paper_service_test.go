package paper

import (
	"context"
	"errors"
	"testing"

	"github.com/lifei6671/papermind/server/internal/service/pagination"
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

	if err := svc.DeleteSection(context.Background(), 10, 100, 1); err != nil {
		t.Fatalf("DeleteSection returned error: %v", err)
	}
	if !repo.deletedSectionInTransaction || repo.deletedSectionPaperID != 100 || repo.deletedSectionID != 1 {
		t.Fatalf("expected section soft delete with child hard delete in one transaction")
	}

	if _, err := svc.ListSections(context.Background(), 10, 100); err != nil {
		t.Fatalf("ListSections returned error: %v", err)
	}
	if !repo.listActiveSectionsCalled {
		t.Fatalf("expected active section query to filter deleted_at = 0")
	}
}

func TestSectionWritesRejectPublishedRuleLiveExam(t *testing.T) {
	repo := &fakeRepository{ruleChangeLocked: true}
	svc := NewService(ServiceOptions{Repo: repo})

	if _, err := svc.CreateSection(context.Background(), CreateSectionInput{
		TenantID:     10,
		PaperID:      100,
		SortOrder:    1,
		Name:         "一、选择题",
		QuestionType: constant.QuestionTypeSingle,
	}); !errors.Is(err, ErrExamMustBeWithdrawnBeforeRuleChange) {
		t.Fatalf("CreateSection expected ErrExamMustBeWithdrawnBeforeRuleChange, got %v", err)
	}
	if err := svc.ReorderSections(context.Background(), 10, 100, []SectionOrder{{SectionID: 1, SortOrder: 2}}); !errors.Is(err, ErrExamMustBeWithdrawnBeforeRuleChange) {
		t.Fatalf("ReorderSections expected ErrExamMustBeWithdrawnBeforeRuleChange, got %v", err)
	}
	if err := svc.DeleteSection(context.Background(), 10, 100, 1); !errors.Is(err, ErrExamMustBeWithdrawnBeforeRuleChange) {
		t.Fatalf("DeleteSection expected ErrExamMustBeWithdrawnBeforeRuleChange, got %v", err)
	}
	if repo.checkedSectionSortOrder || len(repo.sectionOrders) > 0 || repo.deletedSectionInTransaction {
		t.Fatalf("expected published rule_live section writes to stop before repository writes, repo=%#v", repo)
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
		DurationMinutes:  150,
		GradeLevel:       "高一",
		ShuffleQuestions: true,
		ShowAnalysis:     true,
		ActorID:          501,
	})
	if err != nil {
		t.Fatalf("CreateManualPaper returned error: %v", err)
	}
	if paper.BuildMode != BuildModeManual || paper.Status != StatusDraft {
		t.Fatalf("expected manual draft paper, got %#v", paper)
	}
	if repo.createdPaper.CreatedBy != 501 {
		t.Fatalf("expected created paper actor to be persisted, got %#v", repo.createdPaper)
	}
	if repo.createdPaper.DurationMinutes != 150 {
		t.Fatalf("expected created paper duration to be persisted, got %#v", repo.createdPaper)
	}
	if repo.createdPaper.GradeLevel != "高一" {
		t.Fatalf("expected created paper grade level to be persisted, got %#v", repo.createdPaper)
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

func TestUpdatePaperCanMoveUnreferencedPaperScope(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewService(ServiceOptions{Repo: repo})
	spaceID := uint64(301)

	updated, err := svc.UpdatePaper(context.Background(), UpdatePaperInput{
		TenantID:      10,
		PaperID:       100,
		TargetSpaceID: &spaceID,
		ChangeSpace:   true,
		Name:          "空间试卷",
		Description:   "迁移到空间",
		GradeLevel:    "高二",
		ActorID:       501,
	})
	if err != nil {
		t.Fatalf("UpdatePaper returned error: %v", err)
	}
	if updated.SpaceID == nil || *updated.SpaceID != spaceID {
		t.Fatalf("expected paper moved to space %d, got %#v", spaceID, updated.SpaceID)
	}
	if repo.updatedPaper.TargetSpaceID == nil || *repo.updatedPaper.TargetSpaceID != spaceID || !repo.updatedPaper.ChangeSpace {
		t.Fatalf("expected repository to receive paper scope move, got %#v", repo.updatedPaper)
	}
}

func TestUpdatePaperRejectsReferencedPaperScopeMove(t *testing.T) {
	repo := &fakeRepository{paperReferenced: true}
	svc := NewService(ServiceOptions{Repo: repo})
	spaceID := uint64(301)

	_, err := svc.UpdatePaper(context.Background(), UpdatePaperInput{
		TenantID:      10,
		PaperID:       100,
		TargetSpaceID: &spaceID,
		ChangeSpace:   true,
		Name:          "已引用试卷",
		GradeLevel:    "高二",
		ActorID:       501,
	})
	if !errors.Is(err, ErrPaperInUse) {
		t.Fatalf("expected referenced paper space move rejected, got %v", err)
	}
}

func TestUpdatePaperRejectsScopeMoveWhenExistingQuestionFallsOutOfScope(t *testing.T) {
	repo := &fakeRepository{
		currentPaperSpaceID: &[]uint64{301}[0],
		manualSectionQuestions: []SectionQuestion{
			{TenantID: 10, PaperID: 100, SectionID: 11, QuestionID: 2001, SortOrder: 1, Score: "5"},
		},
		questionUsableForScope: map[uint64]bool{
			2001: false,
		},
	}
	svc := NewService(ServiceOptions{Repo: repo})

	_, err := svc.UpdatePaper(context.Background(), UpdatePaperInput{
		TenantID:      10,
		PaperID:       100,
		TargetSpaceID: nil,
		ChangeSpace:   true,
		Name:          "迁移到公共试卷",
		GradeLevel:    "高二",
		ActorID:       501,
	})
	if !errors.Is(err, ErrQuestionOutOfScope) {
		t.Fatalf("expected out-of-scope question to block paper scope move, got %v", err)
	}
}

func TestRuleLivePaperRejectsManualQuestionWrites(t *testing.T) {
	repo := &fakeRepository{
		currentBuildMode:       BuildModeRuleLive,
		questionUsableForPaper: true,
	}
	svc := NewService(ServiceOptions{Repo: repo})

	err := svc.AddManualQuestion(context.Background(), AddSectionQuestionInput{
		TenantID:   10,
		PaperID:    100,
		SectionID:  200,
		QuestionID: 300,
		SortOrder:  1,
		Score:      "2",
	})
	if !errors.Is(err, ErrRuleLiveManualQuestionChange) {
		t.Fatalf("AddManualQuestion expected ErrRuleLiveManualQuestionChange, got %v", err)
	}

	err = svc.UpdateSectionQuestion(context.Background(), UpdateSectionQuestionInput{
		TenantID:   10,
		PaperID:    100,
		SectionID:  200,
		QuestionID: 300,
		SortOrder:  2,
		Score:      "3",
	})
	if !errors.Is(err, ErrRuleLiveManualQuestionChange) {
		t.Fatalf("UpdateSectionQuestion expected ErrRuleLiveManualQuestionChange, got %v", err)
	}

	err = svc.DeleteSectionQuestion(context.Background(), 10, 100, 200, 300)
	if !errors.Is(err, ErrRuleLiveManualQuestionChange) {
		t.Fatalf("DeleteSectionQuestion expected ErrRuleLiveManualQuestionChange, got %v", err)
	}

	err = svc.ReplaceGeneratedQuestion(context.Background(), ReplaceGeneratedQuestionInput{
		TenantID:      10,
		PaperID:       100,
		SectionID:     200,
		OldQuestionID: 300,
		NewQuestionID: 301,
		SortOrder:     1,
		Score:         "2",
	})
	if !errors.Is(err, ErrRuleLiveManualQuestionChange) {
		t.Fatalf("ReplaceGeneratedQuestion expected ErrRuleLiveManualQuestionChange, got %v", err)
	}

	if repo.addQuestionAndRecalculateInTransaction || repo.adjustedQuestionID != 0 || repo.replacedNewQuestionID != 0 || repo.deletedSectionQuestionID != 0 {
		t.Fatalf("expected live manual writes to stop before repository writes, repo=%#v", repo)
	}
}

func TestRuleFixedGenerateAndReviewBehavesLikeManualAfterGeneration(t *testing.T) {
	repo := &fakeRepository{
		questionUsableForPaper: true,
		ruleMatches: map[uint64][]MatchedQuestion{
			1: matchedQuestionsWithScores(
				MatchedQuestion{ID: 300, ScoreDefault: "2"},
				MatchedQuestion{ID: 301, ScoreDefault: "9"},
			),
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

	if err := svc.GenerateRuleFixed(context.Background(), 10, 100, nil); err != nil {
		t.Fatalf("GenerateRuleFixed returned error: %v", err)
	}
	if !repo.generatedFixedInTransaction || len(repo.generatedQuestions) != 2 {
		t.Fatalf("expected generated fixed questions in transaction, got %#v", repo.generatedQuestions)
	}
	if repo.generatedFixedBuildMode != BuildModeRuleFixed {
		t.Fatalf("expected generated fixed build mode %q, got %q", BuildModeRuleFixed, repo.generatedFixedBuildMode)
	}
	if repo.generatedQuestions[0].Score != "2" || repo.generatedQuestions[1].Score != "9" {
		t.Fatalf("expected generated scores from original question score_default, got %#v", repo.generatedQuestions)
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

func TestGenerateRuleFixedRejectsPublishedRuleLiveExam(t *testing.T) {
	repo := &fakeRepository{
		ruleChangeLocked: true,
		ruleMatches: map[uint64][]MatchedQuestion{
			1: matchedQuestions(300, 301),
		},
	}
	svc := NewService(ServiceOptions{Repo: repo})

	err := svc.GenerateRuleFixed(context.Background(), 10, 100, nil)
	if !errors.Is(err, ErrExamMustBeWithdrawnBeforeRuleChange) {
		t.Fatalf("expected ErrExamMustBeWithdrawnBeforeRuleChange, got %v", err)
	}
	if repo.generatedFixedInTransaction {
		t.Fatalf("expected fixed-question generation not to start when published rule_live exam exists")
	}
}

func TestGenerateRuleFixedExcludesRecentExamAndDuplicateQuestions(t *testing.T) {
	repo := &fakeRepository{
		ruleMatches: map[uint64][]MatchedQuestion{
			1: matchedQuestions(101, 102, 103),
			2: matchedQuestions(102, 104),
		},
		rules: []Rule{
			{
				ID:                         1,
				SectionID:                  200,
				QuestionCount:              2,
				ScorePerQuestion:           "3",
				ExcludeRecentExamQuestions: true,
				ExcludeUsedQuestions:       true,
			},
			{
				ID:                   2,
				SectionID:            201,
				QuestionCount:        1,
				ScorePerQuestion:     "5",
				ExcludeUsedQuestions: true,
			},
		},
		recentExamQuestionIDs: map[uint64]bool{101: true},
	}
	svc := NewService(ServiceOptions{Repo: repo})

	if err := svc.GenerateRuleFixed(context.Background(), 10, 100, nil); err != nil {
		t.Fatalf("GenerateRuleFixed returned error: %v", err)
	}
	got := make([]uint64, 0, len(repo.generatedQuestions))
	for _, question := range repo.generatedQuestions {
		got = append(got, question.QuestionID)
	}
	want := []uint64{102, 103, 104}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("expected generated questions %v, got %v", want, got)
		}
	}
}

func TestGenerateRuleFixedExcludesBlockedQuestions(t *testing.T) {
	repo := &fakeRepository{
		ruleMatches: map[uint64][]MatchedQuestion{
			1: matchedQuestions(101, 102, 103),
		},
		rules: []Rule{
			{
				ID:               1,
				SectionID:        200,
				QuestionCount:    2,
				ScorePerQuestion: "3",
			},
		},
	}
	svc := NewService(ServiceOptions{Repo: repo})

	if err := svc.GenerateRuleFixed(context.Background(), 10, 100, []uint64{101}); err != nil {
		t.Fatalf("GenerateRuleFixed returned error: %v", err)
	}
	got := make([]uint64, 0, len(repo.generatedQuestions))
	for _, question := range repo.generatedQuestions {
		got = append(got, question.QuestionID)
	}
	want := []uint64{102, 103}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("expected blocked question 101 to be excluded, got %v", got)
		}
	}
}

func TestGenerateRuleFixedFallsBackWhenDifficultyBucketIsInsufficient(t *testing.T) {
	repo := &fakeRepository{
		ruleMatches: map[uint64][]MatchedQuestion{
			1: matchedQuestions(101, 102, 103),
		},
		ruleDifficultyMatches: map[string][]MatchedQuestion{
			"easy":   matchedQuestions(101),
			"medium": matchedQuestions(102),
			"hard":   {},
		},
		rules: []Rule{
			{
				ID:               1,
				SectionID:        200,
				QuestionCount:    3,
				ScorePerQuestion: "3",
				DifficultyPercentages: DifficultyPercentages{
					Easy:   30,
					Medium: 50,
					Hard:   20,
				},
			},
		},
	}
	svc := NewService(ServiceOptions{Repo: repo})

	if err := svc.GenerateRuleFixed(context.Background(), 10, 100, nil); err != nil {
		t.Fatalf("GenerateRuleFixed returned error: %v", err)
	}
	got := make([]uint64, 0, len(repo.generatedQuestions))
	for _, question := range repo.generatedQuestions {
		got = append(got, question.QuestionID)
	}
	want := []uint64{101, 102, 103}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("expected difficulty shortage to fall back to remaining candidates %v, got %v", want, got)
		}
	}
}

func TestRuleLivePrecheckDedupeAndFreezePool(t *testing.T) {
	repo := &fakeRepository{
		ruleMatches: map[uint64][]MatchedQuestion{
			1: matchedQuestions(300, 301),
			2: matchedQuestions(301, 302),
			3: matchedQuestions(400),
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

	repo.ruleMatches[1] = matchedQuestions(999)
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

func TestRuleChangesRejectPublishedRuleLiveExam(t *testing.T) {
	repo := &fakeRepository{ruleChangeLocked: true}
	svc := NewService(ServiceOptions{Repo: repo})

	_, err := svc.ConfigureRule(context.Background(), ConfigureRuleInput{
		TenantID:         10,
		PaperID:          100,
		SectionID:        200,
		SortOrder:        1,
		TagIDs:           []uint64{1},
		QuestionCount:    2,
		ScorePerQuestion: "5",
	})
	if !errors.Is(err, ErrExamMustBeWithdrawnBeforeRuleChange) {
		t.Fatalf("ConfigureRule expected ErrExamMustBeWithdrawnBeforeRuleChange, got %v", err)
	}

	err = svc.UpdateRuleLiveRule(context.Background(), UpdateRuleInput{
		TenantID:         10,
		PaperID:          100,
		RuleID:           300,
		SectionID:        200,
		SortOrder:        1,
		TagIDs:           []uint64{1},
		QuestionCount:    2,
		ScorePerQuestion: "5",
	})
	if !errors.Is(err, ErrExamMustBeWithdrawnBeforeRuleChange) {
		t.Fatalf("UpdateRuleLiveRule expected ErrExamMustBeWithdrawnBeforeRuleChange, got %v", err)
	}

	err = svc.DeleteRule(context.Background(), 10, 100, 300)
	if !errors.Is(err, ErrExamMustBeWithdrawnBeforeRuleChange) {
		t.Fatalf("DeleteRule expected ErrExamMustBeWithdrawnBeforeRuleChange, got %v", err)
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

func TestUpdateBuildModeRejectsPublishedRuleLiveExam(t *testing.T) {
	repo := &fakeRepository{ruleChangeLocked: true}
	svc := NewService(ServiceOptions{Repo: repo})

	_, err := svc.UpdateBuildMode(context.Background(), UpdateBuildModeInput{
		TenantID:  10,
		PaperID:   100,
		BuildMode: BuildModeRuleFixed,
	})
	if !errors.Is(err, ErrExamMustBeWithdrawnBeforeRuleChange) {
		t.Fatalf("expected ErrExamMustBeWithdrawnBeforeRuleChange, got %v", err)
	}
	if repo.updatedBuildMode {
		t.Fatalf("expected build mode update not to hit repository when rule_live exam is published")
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
	deletedSectionPaperID       uint64
	deletedSectionID            uint64
	listActiveSectionsCalled    bool
	currentBuildMode            string

	createdPaper        Paper
	updatedPaper        UpdatePaperInput
	updatedStatus       string
	updatedBy           uint64
	paperReferenced     bool
	currentPaperSpaceID *uint64

	duplicatePaperQuestion                 bool
	questionUsableForPaper                 bool
	questionUsableForScope                 map[uint64]bool
	addQuestionAndRecalculateInTransaction bool
	addedQuestion                          SectionQuestion
	deletedSectionQuestionID               uint64

	ruleMatches                 map[uint64][]MatchedQuestion
	ruleDifficultyMatches       map[string][]MatchedQuestion
	rules                       []Rule
	recentExamQuestionIDs       map[uint64]bool
	configuredRule              Rule
	generatedFixedInTransaction bool
	generatedFixedBuildMode     string
	generatedQuestions          []SectionQuestion
	replacedOldQuestionID       uint64
	replacedNewQuestionID       uint64
	adjustedQuestionID          uint64
	adjustedSortOrder           int
	adjustedScore               string

	frozenLivePool    bool
	frozenQuestionIDs []uint64
	ruleChangeLocked  bool
	updatedBuildMode  bool

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

func (r *fakeRepository) ListPapers(ctx context.Context, input ListPapersInput) (pagination.Result[Paper], error) {
	return pagination.Result[Paper]{}, nil
}

func (r *fakeRepository) GetPaper(ctx context.Context, tenantID uint64, paperID uint64) (Paper, error) {
	buildMode := r.currentBuildMode
	if buildMode == "" {
		buildMode = BuildModeManual
	}
	return Paper{ID: paperID, TenantID: tenantID, SpaceID: r.currentPaperSpaceID, BuildMode: buildMode}, nil
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

func (r *fakeRepository) DeleteSectionCascade(ctx context.Context, tenantID uint64, paperID uint64, sectionID uint64) error {
	r.deletedSectionInTransaction = true
	r.deletedSectionPaperID = paperID
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

func (r *fakeRepository) UpdatePaper(ctx context.Context, input UpdatePaperInput) (Paper, error) {
	r.updatedPaper = input
	paper := Paper{ID: input.PaperID, TenantID: input.TenantID, SpaceID: input.TargetSpaceID, Name: input.Name, Description: input.Description}
	if input.DurationMinutes != nil {
		paper.DurationMinutes = *input.DurationMinutes
	}
	return paper, nil
}

func (r *fakeRepository) UpdatePaperStatus(ctx context.Context, tenantID uint64, paperID uint64, status string, actorID uint64) (Paper, error) {
	r.updatedStatus = status
	r.updatedBy = actorID
	return Paper{ID: paperID, TenantID: tenantID, Status: status}, nil
}

func (r *fakeRepository) DeletePaper(ctx context.Context, tenantID uint64, paperID uint64) error {
	return nil
}

func (r *fakeRepository) PaperReferenced(ctx context.Context, tenantID uint64, paperID uint64) (bool, error) {
	return r.paperReferenced, nil
}

func (r *fakeRepository) PaperQuestionExists(ctx context.Context, tenantID uint64, paperID uint64, questionID uint64) (bool, error) {
	return r.duplicatePaperQuestion, nil
}

func (r *fakeRepository) QuestionUsableForPaper(ctx context.Context, tenantID uint64, paperID uint64, questionID uint64) (bool, error) {
	return r.questionUsableForPaper, nil
}

func (r *fakeRepository) QuestionUsableForScope(ctx context.Context, tenantID uint64, spaceID *uint64, questionID uint64) (bool, error) {
	if r.questionUsableForScope == nil {
		return true, nil
	}
	return r.questionUsableForScope[questionID], nil
}

func (r *fakeRepository) AddSectionQuestionAndRecalculate(ctx context.Context, question SectionQuestion) error {
	r.addQuestionAndRecalculateInTransaction = true
	r.addedQuestion = question
	r.sectionTotalScore = question.Score
	r.sectionQuestionCount = 1
	r.paperTotalScore = question.Score
	return nil
}

func (r *fakeRepository) UpdateSectionQuestionAndRecalculate(ctx context.Context, input UpdateSectionQuestionInput) error {
	r.adjustedQuestionID = input.QuestionID
	r.adjustedSortOrder = input.SortOrder
	r.adjustedScore = input.Score
	return nil
}

func (r *fakeRepository) DeleteSectionQuestionAndRecalculate(ctx context.Context, tenantID uint64, paperID uint64, sectionID uint64, questionID uint64) error {
	r.deletedSectionQuestionID = questionID
	return nil
}

func (r *fakeRepository) CreateRule(ctx context.Context, rule Rule) (Rule, error) {
	rule.ID = uint64(rule.SortOrder)
	r.configuredRule = rule
	return rule, nil
}

func (r *fakeRepository) CreateRuleAndRecalculate(ctx context.Context, rule Rule, buildMode string) (Rule, error) {
	rule.ID = uint64(rule.SortOrder)
	r.configuredRule = rule
	if buildMode == BuildModeRuleLive {
		r.recalculatedInTransaction = true
	}
	return rule, nil
}

func (r *fakeRepository) MatchQuestionsForRule(ctx context.Context, tenantID uint64, paperID uint64, rule Rule) ([]MatchedQuestion, error) {
	if rule.Difficulty != nil && r.ruleDifficultyMatches != nil {
		return r.ruleDifficultyMatches[*rule.Difficulty], nil
	}
	return r.ruleMatches[rule.ID], nil
}

func (r *fakeRepository) RecentExamQuestionIDs(ctx context.Context, tenantID uint64, paperID uint64, limit int) (map[uint64]bool, error) {
	return r.recentExamQuestionIDs, nil
}

func matchedQuestions(ids ...uint64) []MatchedQuestion {
	questions := make([]MatchedQuestion, 0, len(ids))
	for _, id := range ids {
		questions = append(questions, MatchedQuestion{ID: id, ScoreDefault: "3"})
	}
	return questions
}

func matchedQuestionsWithScores(questions ...MatchedQuestion) []MatchedQuestion {
	return questions
}

func (r *fakeRepository) TagIDsByNames(ctx context.Context, tenantID uint64, names []string) ([]uint64, error) {
	ids := make([]uint64, 0, len(names))
	for index := range names {
		ids = append(ids, uint64(index+1))
	}
	return ids, nil
}

func (r *fakeRepository) ListRules(ctx context.Context, tenantID uint64, paperID uint64) ([]Rule, error) {
	if len(r.rules) > 0 {
		return r.rules, nil
	}
	rules := make([]Rule, 0, len(r.ruleMatches))
	for id := range r.ruleMatches {
		rules = append(rules, Rule{ID: id, SectionID: 200, PaperID: paperID, QuestionCount: len(r.ruleMatches[id]), ScorePerQuestion: "3"})
	}
	return rules, nil
}

func (r *fakeRepository) GenerateFixedQuestionsAndRecalculate(ctx context.Context, tenantID uint64, paperID uint64, buildMode string, questions []SectionQuestion) error {
	r.generatedFixedInTransaction = true
	r.generatedFixedBuildMode = buildMode
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

func (r *fakeRepository) HasPublishedRuleLiveExam(ctx context.Context, tenantID uint64, paperID uint64) (bool, error) {
	return r.ruleChangeLocked, nil
}

func (r *fakeRepository) FrozenQuestionIDs() []uint64 {
	return append([]uint64(nil), r.frozenQuestionIDs...)
}

func (r *fakeRepository) UpdateRule(ctx context.Context, input UpdateRuleInput) error {
	return nil
}

func (r *fakeRepository) UpdateRuleAndRecalculate(ctx context.Context, input UpdateRuleInput, buildMode string) error {
	if buildMode == BuildModeRuleLive {
		r.recalculatedInTransaction = true
	}
	return nil
}

func (r *fakeRepository) DeleteRule(ctx context.Context, tenantID uint64, paperID uint64, ruleID uint64) error {
	return nil
}

func (r *fakeRepository) DeleteRuleAndRecalculate(ctx context.Context, tenantID uint64, paperID uint64, ruleID uint64, buildMode string) error {
	if buildMode == BuildModeRuleLive {
		r.recalculatedInTransaction = true
	}
	return nil
}

func (r *fakeRepository) UpdateBuildMode(ctx context.Context, tenantID uint64, paperID uint64, buildMode string) error {
	r.updatedBuildMode = true
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

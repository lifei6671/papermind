package paper

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"

	"github.com/lifei6671/papermind/server/library/constant"
)

const (
	// BuildModeManual 表示手动固化组卷。
	BuildModeManual = constant.BuildModeManual
	// BuildModeRuleFixed 表示规则生成后固化组卷。
	BuildModeRuleFixed = constant.BuildModeRuleFixed
	// BuildModeRuleLive 表示考试开始时实时抽题。
	BuildModeRuleLive = constant.BuildModeRuleLive

	// StatusDraft 表示试卷草稿状态。
	StatusDraft = constant.PaperStatusDraft
)

var (
	ErrDuplicatePaperQuestion              = errors.New("duplicate paper question")
	ErrQuestionPoolInsufficient            = errors.New("question pool insufficient")
	ErrExamMustBeWithdrawnBeforeRuleChange = errors.New("exam must be withdrawn before rule change")
	ErrQuestionOutOfScope                  = errors.New("question out of paper scope")
)

type Paper struct {
	ID               uint64  // 试卷主键 ID。
	TenantID         uint64  // 所属租户 ID。
	SpaceID          *uint64 // 所属空间 ID，nil 表示租户公共试卷。
	Name             string  // 试卷名称。
	Description      string  // 试卷说明。
	TotalScore       string  // 试卷总分。
	BuildMode        string  // 组卷方式：manual / rule_fixed / rule_live。
	ShuffleQuestions bool    // 是否对每个考生随机题目顺序。
	ShowAnalysis     bool    // 成绩可见后是否向考生展示题目解析。
	Status           string  // 试卷状态。
}

type Section struct {
	ID            uint64 // 试卷大题主键 ID。
	TenantID      uint64 // 所属租户 ID。
	PaperID       uint64 // 试卷 ID。
	SortOrder     int    // 大题排序。
	Name          string // 大题名称。
	QuestionType  string // 大题题型。
	Instructions  string // 大题作答说明。
	TotalScore    string // 大题小计分。
	QuestionCount int    // 大题题目数量。
}

type SectionQuestion struct {
	TenantID       uint64 // 所属租户 ID。
	SectionID      uint64 // 大题 ID。
	PaperID        uint64 // 试卷 ID。
	QuestionID     uint64 // 题目 ID。
	SortOrder      int    // 题目在大题中的排序。
	Score          string // 该题在本试卷中的分值。
	ShuffleOptions *bool  // 是否随机选项，nil 表示回退题库默认值。
}

type Rule struct {
	ID               uint64  // 大题抽题规则主键 ID。
	TenantID         uint64  // 所属租户 ID。
	SectionID        uint64  // 大题 ID。
	PaperID          uint64  // 试卷 ID。
	SortOrder        int     // 规则在大题内的排序。
	Difficulty       *string // 抽题难度条件，nil 表示不限难度。
	TagFilter        string  // 标签过滤条件，JSON 数组字符串。
	QuestionCount    int     // 该规则抽题数量。
	ScorePerQuestion string  // 该规则下每题分值。
	ShuffleOptions   *bool   // 是否随机选项，nil 表示回退题库默认值。
}

type SectionOrder struct {
	SectionID uint64 // 大题 ID。
	SortOrder int    // 新排序。
}

type CreateSectionInput struct {
	TenantID     uint64 // 所属租户 ID。
	PaperID      uint64 // 试卷 ID。
	SortOrder    int    // 大题排序。
	Name         string // 大题名称。
	QuestionType string // 大题题型。
	Instructions string // 大题作答说明。
}

type UpdateSectionInput struct {
	TenantID     uint64 // 所属租户 ID。
	SectionID    uint64 // 大题 ID。
	Name         string // 大题名称。
	QuestionType string // 大题题型。
	Instructions string // 大题作答说明。
}

type CreatePaperInput struct {
	TenantID         uint64  // 所属租户 ID。
	SpaceID          *uint64 // 所属空间 ID。
	Name             string  // 试卷名称。
	Description      string  // 试卷说明。
	ShuffleQuestions bool    // 是否随机题目顺序。
	ShowAnalysis     bool    // 是否展示解析。
}

type AddSectionQuestionInput struct {
	TenantID       uint64 // 所属租户 ID。
	PaperID        uint64 // 试卷 ID。
	SectionID      uint64 // 大题 ID。
	QuestionID     uint64 // 题目 ID。
	SortOrder      int    // 题目在大题中的排序。
	Score          string // 分值覆盖。
	ShuffleOptions *bool  // 可空选项随机设置。
}

type ConfigureRuleInput struct {
	TenantID         uint64   // 所属租户 ID。
	PaperID          uint64   // 试卷 ID。
	SectionID        uint64   // 大题 ID。
	SortOrder        int      // 规则排序。
	Difficulty       *string  // 抽题难度，nil 表示不限。
	TagIDs           []uint64 // 标签过滤条件。
	QuestionCount    int      // 抽题数量。
	ScorePerQuestion string   // 每题分值。
	ShuffleOptions   *bool    // 可空选项随机设置。
}

type ReplaceGeneratedQuestionInput struct {
	TenantID      uint64 // 所属租户 ID。
	PaperID       uint64 // 试卷 ID。
	SectionID     uint64 // 大题 ID。
	OldQuestionID uint64 // 原题目 ID。
	NewQuestionID uint64 // 替换后题目 ID。
	Score         string // 新分值。
	SortOrder     int    // 新排序。
}

type AdjustGeneratedQuestionInput struct {
	TenantID   uint64 // 所属租户 ID。
	PaperID    uint64 // 试卷 ID。
	SectionID  uint64 // 大题 ID。
	QuestionID uint64 // 题目 ID。
	SortOrder  int    // 新排序。
	Score      string // 新分值。
}

type LivePrecheckResult struct {
	CandidateQuestionIDs []uint64 // 去重后的候选题池。
}

type UpdateRuleInput struct {
	TenantID   uint64 // 所属租户 ID。
	PaperID    uint64 // 试卷 ID。
	ExamID     uint64 // 考试 ID。
	RuleID     uint64 // 规则 ID。
	ExamFrozen bool   // 考试是否已冻结题池。
}

type SectionAggregate struct {
	SectionID     uint64 // 大题 ID。
	TotalScore    string // 大题小计分。
	QuestionCount int    // 题目数量。
}

type Repository interface {
	ListPapers(ctx context.Context, tenantID uint64) ([]Paper, error)
	SectionSortOrderExists(ctx context.Context, tenantID uint64, paperID uint64, sortOrder int) (bool, error)
	CreateSection(ctx context.Context, section Section) (Section, error)
	UpdateSection(ctx context.Context, input UpdateSectionInput) error
	ReorderSections(ctx context.Context, tenantID uint64, paperID uint64, orders []SectionOrder) error
	DeleteSectionCascade(ctx context.Context, tenantID uint64, sectionID uint64) error
	ListActiveSections(ctx context.Context, tenantID uint64, paperID uint64) ([]Section, error)
	CreatePaper(ctx context.Context, paper Paper) (Paper, error)
	PaperQuestionExists(ctx context.Context, tenantID uint64, paperID uint64, questionID uint64) (bool, error)
	QuestionUsableForPaper(ctx context.Context, tenantID uint64, paperID uint64, questionID uint64) (bool, error)
	AddSectionQuestionAndRecalculate(ctx context.Context, question SectionQuestion) error
	CreateRule(ctx context.Context, rule Rule) (Rule, error)
	MatchQuestionsForRule(ctx context.Context, tenantID uint64, paperID uint64, rule Rule) ([]uint64, error)
	ListRules(ctx context.Context, tenantID uint64, paperID uint64) ([]Rule, error)
	GenerateFixedQuestionsAndRecalculate(ctx context.Context, tenantID uint64, paperID uint64, questions []SectionQuestion) error
	ReplaceGeneratedQuestionAndRecalculate(ctx context.Context, input ReplaceGeneratedQuestionInput) error
	AdjustGeneratedQuestionAndRecalculate(ctx context.Context, input AdjustGeneratedQuestionInput) error
	FreezeLivePools(ctx context.Context, tenantID uint64, examID uint64, questionIDs []uint64) error
	UpdateRule(ctx context.Context, input UpdateRuleInput) error
	ListSectionQuestions(ctx context.Context, tenantID uint64, paperID uint64) ([]SectionQuestion, error)
	ListRuleLiveRules(ctx context.Context, tenantID uint64, paperID uint64) ([]Rule, error)
	SaveAggregates(ctx context.Context, tenantID uint64, paperID uint64, sections []SectionAggregate, paperTotalScore string) error
}

type ServiceOptions struct {
	Repo Repository
}

type Service struct {
	repo Repository
}

func NewService(options ServiceOptions) *Service {
	return &Service{repo: options.Repo}
}

func (s *Service) ListPapers(ctx context.Context, tenantID uint64) ([]Paper, error) {
	return s.repo.ListPapers(ctx, tenantID)
}

func (s *Service) CreateSection(ctx context.Context, input CreateSectionInput) (Section, error) {
	if _, err := s.repo.SectionSortOrderExists(ctx, input.TenantID, input.PaperID, input.SortOrder); err != nil {
		return Section{}, err
	}
	return s.repo.CreateSection(ctx, Section{
		TenantID:     input.TenantID,
		PaperID:      input.PaperID,
		SortOrder:    input.SortOrder,
		Name:         input.Name,
		QuestionType: input.QuestionType,
		Instructions: input.Instructions,
		TotalScore:   "0",
	})
}

func (s *Service) UpdateSection(ctx context.Context, input UpdateSectionInput) error {
	return s.repo.UpdateSection(ctx, input)
}

func (s *Service) ReorderSections(ctx context.Context, tenantID uint64, paperID uint64, orders []SectionOrder) error {
	return s.repo.ReorderSections(ctx, tenantID, paperID, orders)
}

func (s *Service) DeleteSection(ctx context.Context, tenantID uint64, sectionID uint64) error {
	return s.repo.DeleteSectionCascade(ctx, tenantID, sectionID)
}

func (s *Service) ListSections(ctx context.Context, tenantID uint64, paperID uint64) ([]Section, error) {
	return s.repo.ListActiveSections(ctx, tenantID, paperID)
}

func (s *Service) ListSectionQuestions(ctx context.Context, tenantID uint64, paperID uint64) ([]SectionQuestion, error) {
	return s.repo.ListSectionQuestions(ctx, tenantID, paperID)
}

func (s *Service) ListRules(ctx context.Context, tenantID uint64, paperID uint64) ([]Rule, error) {
	return s.repo.ListRules(ctx, tenantID, paperID)
}

func (s *Service) CreateManualPaper(ctx context.Context, input CreatePaperInput) (Paper, error) {
	return s.repo.CreatePaper(ctx, Paper{
		TenantID:         input.TenantID,
		SpaceID:          input.SpaceID,
		Name:             input.Name,
		Description:      input.Description,
		TotalScore:       "0",
		BuildMode:        BuildModeManual,
		ShuffleQuestions: input.ShuffleQuestions,
		ShowAnalysis:     input.ShowAnalysis,
		Status:           StatusDraft,
	})
}

func (s *Service) AddManualQuestion(ctx context.Context, input AddSectionQuestionInput) error {
	exists, err := s.repo.PaperQuestionExists(ctx, input.TenantID, input.PaperID, input.QuestionID)
	if err != nil {
		return err
	}
	if exists {
		return ErrDuplicatePaperQuestion
	}
	usable, err := s.repo.QuestionUsableForPaper(ctx, input.TenantID, input.PaperID, input.QuestionID)
	if err != nil {
		return err
	}
	if !usable {
		return ErrQuestionOutOfScope
	}
	return s.repo.AddSectionQuestionAndRecalculate(ctx, SectionQuestion{
		TenantID:       input.TenantID,
		PaperID:        input.PaperID,
		SectionID:      input.SectionID,
		QuestionID:     input.QuestionID,
		SortOrder:      input.SortOrder,
		Score:          input.Score,
		ShuffleOptions: input.ShuffleOptions,
	})
}

func (s *Service) ConfigureRule(ctx context.Context, input ConfigureRuleInput) (Rule, error) {
	return s.repo.CreateRule(ctx, Rule{
		TenantID:         input.TenantID,
		PaperID:          input.PaperID,
		SectionID:        input.SectionID,
		SortOrder:        input.SortOrder,
		Difficulty:       input.Difficulty,
		TagFilter:        tagFilterJSON(input.TagIDs),
		QuestionCount:    input.QuestionCount,
		ScorePerQuestion: input.ScorePerQuestion,
		ShuffleOptions:   input.ShuffleOptions,
	})
}

func (s *Service) GenerateRuleFixed(ctx context.Context, tenantID uint64, paperID uint64) error {
	rules, err := s.repo.ListRules(ctx, tenantID, paperID)
	if err != nil {
		return err
	}
	questions := make([]SectionQuestion, 0)
	sort.Slice(rules, func(i int, j int) bool {
		return rules[i].SortOrder < rules[j].SortOrder
	})
	for _, rule := range rules {
		matched, err := s.repo.MatchQuestionsForRule(ctx, tenantID, paperID, rule)
		if err != nil {
			return err
		}
		if len(matched) < rule.QuestionCount {
			return ErrQuestionPoolInsufficient
		}
		for index := 0; index < rule.QuestionCount; index++ {
			questions = append(questions, SectionQuestion{
				TenantID:       tenantID,
				PaperID:        paperID,
				SectionID:      rule.SectionID,
				QuestionID:     matched[index],
				SortOrder:      len(questions) + 1,
				Score:          rule.ScorePerQuestion,
				ShuffleOptions: rule.ShuffleOptions,
			})
		}
	}
	return s.repo.GenerateFixedQuestionsAndRecalculate(ctx, tenantID, paperID, questions)
}

func (s *Service) ReplaceGeneratedQuestion(ctx context.Context, input ReplaceGeneratedQuestionInput) error {
	return s.repo.ReplaceGeneratedQuestionAndRecalculate(ctx, input)
}

func (s *Service) AdjustGeneratedQuestion(ctx context.Context, input AdjustGeneratedQuestionInput) error {
	return s.repo.AdjustGeneratedQuestionAndRecalculate(ctx, input)
}

func (s *Service) PrecheckRuleLive(ctx context.Context, tenantID uint64, paperID uint64, rules []Rule) (LivePrecheckResult, error) {
	seen := map[uint64]bool{}
	candidates := make([]uint64, 0)
	totalRequired := 0
	for _, rule := range rules {
		totalRequired += rule.QuestionCount
		matched, err := s.repo.MatchQuestionsForRule(ctx, tenantID, paperID, rule)
		if err != nil {
			return LivePrecheckResult{}, err
		}
		for _, questionID := range matched {
			if seen[questionID] {
				continue
			}
			seen[questionID] = true
			candidates = append(candidates, questionID)
		}
	}
	if len(candidates) < totalRequired {
		return LivePrecheckResult{}, ErrQuestionPoolInsufficient
	}
	return LivePrecheckResult{CandidateQuestionIDs: candidates}, nil
}

func (s *Service) FreezeLiveQuestionPool(ctx context.Context, tenantID uint64, examID uint64, result LivePrecheckResult) error {
	return s.repo.FreezeLivePools(ctx, tenantID, examID, result.CandidateQuestionIDs)
}

func (s *Service) UpdateRuleLiveRule(ctx context.Context, input UpdateRuleInput) error {
	if input.ExamFrozen {
		return ErrExamMustBeWithdrawnBeforeRuleChange
	}
	return s.repo.UpdateRule(ctx, input)
}

func (s *Service) RecalculatePaper(ctx context.Context, tenantID uint64, paperID uint64, buildMode string) error {
	switch buildMode {
	case BuildModeRuleLive:
		rules, err := s.repo.ListRuleLiveRules(ctx, tenantID, paperID)
		if err != nil {
			return err
		}
		sections, total := aggregateRules(rules)
		return s.repo.SaveAggregates(ctx, tenantID, paperID, sections, total)
	default:
		questions, err := s.repo.ListSectionQuestions(ctx, tenantID, paperID)
		if err != nil {
			return err
		}
		sections, total := aggregateQuestions(questions)
		return s.repo.SaveAggregates(ctx, tenantID, paperID, sections, total)
	}
}

func tagFilterJSON(tagIDs []uint64) string {
	sorted := append([]uint64(nil), tagIDs...)
	sort.Slice(sorted, func(i int, j int) bool {
		return sorted[i] < sorted[j]
	})
	data, _ := json.Marshal(sorted)
	return string(data)
}

func aggregateQuestions(questions []SectionQuestion) ([]SectionAggregate, string) {
	bySection := map[uint64]SectionAggregate{}
	paperTotal := 0.0
	for _, question := range questions {
		score := mustScore(question.Score)
		agg := bySection[question.SectionID]
		agg.SectionID = question.SectionID
		agg.TotalScore = formatScore(mustScore(agg.TotalScore) + score)
		agg.QuestionCount++
		bySection[question.SectionID] = agg
		paperTotal += score
	}
	return sortedAggregates(bySection), formatScore(paperTotal)
}

func aggregateRules(rules []Rule) ([]SectionAggregate, string) {
	bySection := map[uint64]SectionAggregate{}
	paperTotal := 0.0
	for _, rule := range rules {
		total := float64(rule.QuestionCount) * mustScore(rule.ScorePerQuestion)
		agg := bySection[rule.SectionID]
		agg.SectionID = rule.SectionID
		agg.TotalScore = formatScore(mustScore(agg.TotalScore) + total)
		agg.QuestionCount += rule.QuestionCount
		bySection[rule.SectionID] = agg
		paperTotal += total
	}
	return sortedAggregates(bySection), formatScore(paperTotal)
}

func sortedAggregates(items map[uint64]SectionAggregate) []SectionAggregate {
	sections := make([]SectionAggregate, 0, len(items))
	for _, item := range items {
		sections = append(sections, item)
	}
	sort.Slice(sections, func(i int, j int) bool {
		return sections[i].SectionID < sections[j].SectionID
	})
	return sections
}

func mustScore(value string) float64 {
	if strings.TrimSpace(value) == "" {
		return 0
	}
	parsed, _ := strconv.ParseFloat(value, 64)
	return parsed
}

func formatScore(value float64) string {
	text := strconv.FormatFloat(value, 'f', 6, 64)
	text = strings.TrimRight(text, "0")
	text = strings.TrimRight(text, ".")
	if text == "" {
		return "0"
	}
	return text
}

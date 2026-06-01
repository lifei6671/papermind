package question

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/lifei6671/papermind/server/internal/service/pagination"
	"github.com/lifei6671/papermind/server/internal/service/permission"
	"github.com/lifei6671/papermind/server/library/constant"
)

const (
	// QuestionTypeSingle 表示单选题。
	QuestionTypeSingle = constant.QuestionTypeSingle
	// QuestionTypeMultiple 表示多选题。
	QuestionTypeMultiple = constant.QuestionTypeMultiple
	// QuestionTypeJudge 表示判断题。
	QuestionTypeJudge = constant.QuestionTypeJudge
	// QuestionTypeFillBlank 表示填空题。
	QuestionTypeFillBlank = constant.QuestionTypeFillBlank
	// QuestionTypeShortText 表示简答题。
	QuestionTypeShortText = constant.QuestionTypeShortText

	// DifficultyEasy 表示简单难度。
	DifficultyEasy = "easy"
	// DifficultyMedium 表示中等难度。
	DifficultyMedium = "medium"
	// DifficultyHard 表示困难难度。
	DifficultyHard = "hard"

	// QuestionStatusEnabled 表示题目可用。
	QuestionStatusEnabled = constant.QuestionStatusEnabled
	// QuestionStatusDraft 表示题目草稿，必须手动启用后才可参与组卷。
	QuestionStatusDraft = constant.QuestionStatusDraft
	// QuestionStatusDisabled 表示题目被禁用。
	QuestionStatusDisabled = "disabled"

	// GradingModeManual 表示人工阅卷。
	GradingModeManual = constant.GradingModeManual
)

var (
	ErrChoiceQuestionNeedsCorrectAnswer   = errors.New("choice question needs at least one correct answer")
	ErrSingleQuestionOnlyOneCorrectAnswer = errors.New("single choice question only allows one correct answer")
	ErrFillBlankNeedsStandardAnswer       = errors.New("fill blank needs standard answer")
	ErrUnsupportedQuestionType            = errors.New("unsupported question type")
	ErrUnsupportedDifficulty              = errors.New("unsupported difficulty")
	ErrUnsupportedQuestionStatus          = errors.New("unsupported question status")
	ErrQuestionNotFound                   = errors.New("question not found")
	ErrQuestionReferenced                 = errors.New("question referenced")
)

type Question struct {
	ID                 uint64  // 题目主键 ID。
	TenantID           uint64  // 所属租户 ID。
	SpaceID            *uint64 // 所属空间 ID，nil 表示租户公共题库。
	Type               string  // 题型：single / multiple / judge / fill_blank / short_text。
	Difficulty         string  // 难度：easy / medium / hard。
	Title              string  // 题干内容。
	Analysis           string  // 题目解析，可选。
	ScoreDefault       string  // 默认分值。
	ChoiceDisplayCount *int    // 选择题展示选项数量，nil 表示不限制。
	ShuffleOptions     bool    // 题库默认选项随机设置。
	StandardAnswer     string  // 填空题标准答案或判断题标准答案。
	ReferenceAnswer    string  // 简答题参考答案。
	GradingMode        string  // 阅卷方式。
	Status             string  // 题目状态。
	CreatedAt          int64   // 出题时间，Unix 毫秒时间戳。
	CreatedBy          uint64  // 出题人用户 ID。
	AuthorName         string  // 出题人账号。
	AuthorRole         string  // 出题人在租户内的身份角色。
	Options            []QuestionOption
	Tags               []string
}

type QuestionOption struct {
	ID           uint64 // 题目选项主键 ID。
	OptionKey    string // 出题编辑时的原始展示标签，不参与判分。
	SortOrder    int    // 选项原始排序。
	Content      string // 选项内容。
	IsCorrect    bool   // 是否为正确答案。
	IsDistractor bool   // 是否可作为随机补位干扰项。
}

type CreateQuestionInput struct {
	Permission         permission.PermissionContext // 当前写入人权限上下文。
	TenantID           uint64                       // 所属租户 ID。
	SpaceID            *uint64                      // 所属空间 ID，nil 表示租户公共题库。
	Type               string                       // 题型。
	Difficulty         string                       // 难度。
	Title              string                       // 题干内容。
	Analysis           string                       // 题目解析，可选。
	ScoreDefault       string                       // 默认分值。
	ChoiceDisplayCount *int                         // 选择题展示选项数量。
	ShuffleOptions     bool                         // 题库默认选项随机设置。
	Options            []QuestionOptionInput        // 选择题选项。
	StandardAnswer     string                       // 填空题标准答案或判断题答案。
	ReferenceAnswer    string                       // 简答题参考答案。
	BlankCount         int                          // 填空数量，仅用于前端/导入阶段表达题目有几个空。
	Tags               []string                     // 题目标签名称。
}

type ListQuestionsInput struct {
	TenantID uint64
	SpaceID  *uint64
	Page     int
	PageSize int
}

type GetQuestionInput struct {
	Permission permission.PermissionContext
	TenantID   uint64
	QuestionID uint64
}

type UpdateQuestionInput struct {
	Permission         permission.PermissionContext // 当前写入人权限上下文。
	TenantID           uint64                       // 所属租户 ID。
	QuestionID         uint64                       // 待更新题目 ID。
	Type               string                       // 题型。
	Difficulty         string                       // 难度。
	Title              string                       // 题干内容。
	Analysis           string                       // 题目解析，可选。
	ScoreDefault       string                       // 默认分值。
	ChoiceDisplayCount *int                         // 选择题展示选项数量。
	ShuffleOptions     bool                         // 题库默认选项随机设置。
	Options            []QuestionOptionInput        // 选择题选项。
	StandardAnswer     string                       // 填空题标准答案或判断题答案。
	ReferenceAnswer    string                       // 简答题参考答案。
	BlankCount         int                          // 填空数量，仅用于前端/导入阶段表达题目有几个空。
	Tags               []string                     // 题目标签名称。
}

type UpdateQuestionStatusInput struct {
	Permission permission.PermissionContext
	TenantID   uint64
	QuestionID uint64
	Status     string
}

type DeleteQuestionInput struct {
	Permission permission.PermissionContext
	TenantID   uint64
	QuestionID uint64
}

type QuestionOptionInput struct {
	OptionKey    string // 出题编辑时的原始展示标签。
	Content      string // 选项内容。
	IsCorrect    bool   // 是否为正确答案。
	IsDistractor bool   // 是否可作为随机补位干扰项。
}

type ReplaceOptionsInput struct {
	TenantID   uint64                // 所属租户 ID。
	QuestionID uint64                // 题目 ID。
	Options    []QuestionOptionInput // 新选项完整列表。
}

type ChoiceSnapshot struct {
	Options []ChoiceSnapshotOption // 考试快照中的选项。
}

type ChoiceSnapshotOption struct {
	ID        uint64 // 快照选项 ID。
	OptionKey string // 展示标签，不参与判分。
	IsCorrect bool   // 是否正确答案。
}

type ChoiceSnapshotAnswer struct {
	OptionID  uint64 // 作答选择的快照选项 ID。
	OptionKey string // 前端展示标签，不参与判分。
}

type ImportQuestionsInput struct {
	Permission permission.PermissionContext // 当前导入人权限上下文。
	TenantID   uint64                       // 所属租户 ID。
	SpaceID    *uint64                      // 所属空间 ID。
	Rows       []ImportRow                  // 导入数据行。
}

type ImportRow struct {
	RowNumber     int    // 原始表格行号。
	Type          string // 题型。
	Title         string // 题干。
	Options       string // 选项，格式示例：A.对|B.错。
	CorrectAnswer string // 正确答案，选择题填写 option_key，多个用逗号分隔。
	Analysis      string // 解析。
	Difficulty    string // 难度。
	Tags          string // 标签，逗号分隔。
}

type ImportResult struct {
	SuccessCount int           // 成功导入行数。
	Errors       []ImportError // 行级错误。
}

type ImportError struct {
	RowNumber int    // 原始表格行号。
	Reason    string // 失败原因。
}

type QuestionRepository interface {
	CreateQuestion(ctx context.Context, item Question, options []QuestionOption, tags []string) (Question, error)
	GetQuestion(ctx context.Context, tenantID uint64, questionID uint64) (Question, error)
	ListVisibleQuestions(ctx context.Context, input ListQuestionsInput) (pagination.Result[Question], error)
	UpdateQuestion(ctx context.Context, item Question, options []QuestionOption, tags []string) (Question, error)
	UpdateQuestionStatus(ctx context.Context, tenantID uint64, questionID uint64, status string, actorID uint64) (Question, error)
	DeleteQuestion(ctx context.Context, tenantID uint64, questionID uint64, actorID uint64) error
	QuestionReferenced(ctx context.Context, tenantID uint64, questionID uint64) (bool, error)
	ReplaceOptionsInTransaction(ctx context.Context, tenantID uint64, questionID uint64, options []QuestionOption) error
}

type QuestionServiceOptions struct {
	Repo QuestionRepository
}

type QuestionService struct {
	repo QuestionRepository
}

func NewQuestionService(options QuestionServiceOptions) *QuestionService {
	return &QuestionService{repo: options.Repo}
}

func (s *QuestionService) CreateQuestion(ctx context.Context, input CreateQuestionInput) (Question, error) {
	if err := canWriteQuestionScope(input.Permission, input.TenantID, input.SpaceID); err != nil {
		return Question{}, err
	}
	options := toQuestionOptions(input.Options)
	if err := validateQuestionInput(input, options); err != nil {
		return Question{}, err
	}
	item := Question{
		TenantID:           input.TenantID,
		SpaceID:            input.SpaceID,
		Type:               input.Type,
		Difficulty:         input.Difficulty,
		Title:              input.Title,
		Analysis:           input.Analysis,
		ScoreDefault:       input.ScoreDefault,
		ChoiceDisplayCount: input.ChoiceDisplayCount,
		ShuffleOptions:     input.ShuffleOptions,
		StandardAnswer:     input.StandardAnswer,
		ReferenceAnswer:    input.ReferenceAnswer,
		Status:             QuestionStatusDraft,
		CreatedBy:          input.Permission.UserID,
	}
	if input.Type == QuestionTypeShortText {
		item.GradingMode = GradingModeManual
	}
	return s.repo.CreateQuestion(ctx, item, options, input.Tags)
}

func (s *QuestionService) GetQuestion(ctx context.Context, input GetQuestionInput) (Question, error) {
	item, err := s.repo.GetQuestion(ctx, input.TenantID, input.QuestionID)
	if err != nil {
		return Question{}, err
	}
	if err := canWriteQuestionScope(input.Permission, item.TenantID, item.SpaceID); err != nil {
		return Question{}, err
	}
	return item, nil
}

func (s *QuestionService) ListVisibleQuestions(ctx context.Context, input ListQuestionsInput) (pagination.Result[Question], error) {
	return s.repo.ListVisibleQuestions(ctx, input)
}

func (s *QuestionService) UpdateQuestion(ctx context.Context, input UpdateQuestionInput) (Question, error) {
	existing, err := s.repo.GetQuestion(ctx, input.TenantID, input.QuestionID)
	if err != nil {
		return Question{}, err
	}
	if err := canWriteQuestionScope(input.Permission, existing.TenantID, existing.SpaceID); err != nil {
		return Question{}, err
	}
	referenced, err := s.repo.QuestionReferenced(ctx, input.TenantID, input.QuestionID)
	if err != nil {
		return Question{}, err
	}
	if referenced {
		return Question{}, ErrQuestionReferenced
	}
	options := toQuestionOptions(input.Options)
	createInput := CreateQuestionInput{
		Permission:         input.Permission,
		TenantID:           input.TenantID,
		SpaceID:            existing.SpaceID,
		Type:               input.Type,
		Difficulty:         input.Difficulty,
		Title:              input.Title,
		Analysis:           input.Analysis,
		ScoreDefault:       input.ScoreDefault,
		ChoiceDisplayCount: input.ChoiceDisplayCount,
		ShuffleOptions:     input.ShuffleOptions,
		Options:            input.Options,
		StandardAnswer:     input.StandardAnswer,
		ReferenceAnswer:    input.ReferenceAnswer,
		BlankCount:         input.BlankCount,
		Tags:               input.Tags,
	}
	if err := validateQuestionInput(createInput, options); err != nil {
		return Question{}, err
	}
	item := Question{
		ID:                 input.QuestionID,
		TenantID:           input.TenantID,
		SpaceID:            existing.SpaceID,
		Type:               input.Type,
		Difficulty:         input.Difficulty,
		Title:              input.Title,
		Analysis:           input.Analysis,
		ScoreDefault:       input.ScoreDefault,
		ChoiceDisplayCount: input.ChoiceDisplayCount,
		ShuffleOptions:     input.ShuffleOptions,
		StandardAnswer:     input.StandardAnswer,
		ReferenceAnswer:    input.ReferenceAnswer,
		Status:             existing.Status,
		CreatedBy:          input.Permission.UserID,
	}
	if input.Type == QuestionTypeShortText {
		item.GradingMode = GradingModeManual
	}
	return s.repo.UpdateQuestion(ctx, item, options, input.Tags)
}

func (s *QuestionService) UpdateQuestionStatus(ctx context.Context, input UpdateQuestionStatusInput) (Question, error) {
	if input.Status != QuestionStatusEnabled && input.Status != QuestionStatusDisabled {
		return Question{}, ErrUnsupportedQuestionStatus
	}
	item, err := s.repo.GetQuestion(ctx, input.TenantID, input.QuestionID)
	if err != nil {
		return Question{}, err
	}
	if err := canWriteQuestionScope(input.Permission, item.TenantID, item.SpaceID); err != nil {
		return Question{}, err
	}
	return s.repo.UpdateQuestionStatus(ctx, input.TenantID, input.QuestionID, input.Status, input.Permission.UserID)
}

func (s *QuestionService) DeleteQuestion(ctx context.Context, input DeleteQuestionInput) error {
	item, err := s.repo.GetQuestion(ctx, input.TenantID, input.QuestionID)
	if err != nil {
		return err
	}
	if err := canWriteQuestionScope(input.Permission, item.TenantID, item.SpaceID); err != nil {
		return err
	}
	referenced, err := s.repo.QuestionReferenced(ctx, input.TenantID, input.QuestionID)
	if err != nil {
		return err
	}
	if referenced {
		return ErrQuestionReferenced
	}
	return s.repo.DeleteQuestion(ctx, input.TenantID, input.QuestionID, input.Permission.UserID)
}

func (s *QuestionService) GradeChoiceAnswer(snapshot ChoiceSnapshot, selectedIDs []uint64) bool {
	return sameIDs(correctOptionIDs(snapshot), normalizeIDs(selectedIDs))
}

func (s *QuestionService) GradeChoiceSnapshotAnswer(snapshot ChoiceSnapshot, answers []ChoiceSnapshotAnswer) bool {
	selectedIDs := make([]uint64, 0, len(answers))
	for _, answer := range answers {
		selectedIDs = append(selectedIDs, answer.OptionID)
	}
	return s.GradeChoiceAnswer(snapshot, selectedIDs)
}

func (s *QuestionService) ReplaceOptions(ctx context.Context, input ReplaceOptionsInput) error {
	options := toQuestionOptions(input.Options)
	if err := validateChoiceOptions(QuestionTypeMultiple, options); err != nil {
		return err
	}
	return s.repo.ReplaceOptionsInTransaction(ctx, input.TenantID, input.QuestionID, options)
}

func (s *QuestionService) GradeFillBlankAnswer(answer string, standardAnswer string) bool {
	left, err := parseFillBlankAnswers(answer)
	if err != nil {
		return false
	}
	right, err := parseFillBlankAnswers(standardAnswer)
	if err != nil {
		return false
	}
	return sameStrings(left, right)
}

func (s *QuestionService) ImportTemplateHeaders() []string {
	return []string{"type", "title", "options", "correct_answer", "analysis", "difficulty", "tags"}
}

func (s *QuestionService) ImportQuestions(ctx context.Context, input ImportQuestionsInput) (ImportResult, error) {
	if err := canWriteQuestionScope(input.Permission, input.TenantID, input.SpaceID); err != nil {
		return ImportResult{}, err
	}
	result := ImportResult{}
	for _, row := range input.Rows {
		questionInput := row.toCreateQuestionInput(input.TenantID, input.SpaceID)
		questionInput.Permission = input.Permission
		if _, err := s.CreateQuestion(ctx, questionInput); err != nil {
			result.Errors = append(result.Errors, ImportError{
				RowNumber: row.RowNumber,
				Reason:    err.Error(),
			})
			continue
		}
		result.SuccessCount++
	}
	return result, nil
}

func canWriteQuestionScope(ctx permission.PermissionContext, tenantID uint64, spaceID *uint64) error {
	if ctx.SubjectType != permission.SubjectTenantUser || ctx.TenantID != tenantID {
		return permission.ErrForbidden
	}
	if ctx.Role == permission.RoleTenantAdmin {
		return nil
	}
	if spaceID == nil {
		return permission.ErrForbidden
	}
	role := ctx.SpaceMemberships[*spaceID]
	if role == permission.RoleSpaceAdmin || role == permission.RoleTeacher {
		return nil
	}
	return permission.ErrForbidden
}

func validateQuestionInput(input CreateQuestionInput, options []QuestionOption) error {
	if input.Difficulty != "" && !supportedDifficulty(input.Difficulty) {
		return ErrUnsupportedDifficulty
	}
	switch input.Type {
	case QuestionTypeSingle:
		return validateChoiceOptions(QuestionTypeSingle, options)
	case QuestionTypeMultiple:
		return validateChoiceOptions(QuestionTypeMultiple, options)
	case QuestionTypeJudge:
		return nil
	case QuestionTypeFillBlank:
		answers, err := parseFillBlankAnswers(input.StandardAnswer)
		if err != nil || len(answers) == 0 {
			return ErrFillBlankNeedsStandardAnswer
		}
		return nil
	case QuestionTypeShortText:
		return nil
	default:
		return ErrUnsupportedQuestionType
	}
}

func supportedDifficulty(difficulty string) bool {
	switch difficulty {
	case DifficultyEasy, DifficultyMedium, DifficultyHard:
		return true
	default:
		return false
	}
}

func validateChoiceOptions(questionType string, options []QuestionOption) error {
	correctCount := 0
	for _, option := range options {
		if option.IsCorrect {
			correctCount++
		}
	}
	if correctCount == 0 {
		return ErrChoiceQuestionNeedsCorrectAnswer
	}
	if questionType == QuestionTypeSingle && correctCount != 1 {
		return ErrSingleQuestionOnlyOneCorrectAnswer
	}
	return nil
}

func toQuestionOptions(inputs []QuestionOptionInput) []QuestionOption {
	options := make([]QuestionOption, 0, len(inputs))
	for index, input := range inputs {
		options = append(options, QuestionOption{
			OptionKey:    input.OptionKey,
			SortOrder:    index + 1,
			Content:      input.Content,
			IsCorrect:    input.IsCorrect,
			IsDistractor: input.IsDistractor,
		})
	}
	return options
}

func correctOptionIDs(snapshot ChoiceSnapshot) []uint64 {
	ids := make([]uint64, 0, len(snapshot.Options))
	for _, option := range snapshot.Options {
		if option.IsCorrect {
			ids = append(ids, option.ID)
		}
	}
	return normalizeIDs(ids)
}

func normalizeIDs(ids []uint64) []uint64 {
	normalized := append([]uint64(nil), ids...)
	sort.Slice(normalized, func(i int, j int) bool {
		return normalized[i] < normalized[j]
	})
	return normalized
}

func sameIDs(left []uint64, right []uint64) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func (row ImportRow) toCreateQuestionInput(tenantID uint64, spaceID *uint64) CreateQuestionInput {
	options := parseImportOptions(row.Options, row.CorrectAnswer)
	standardAnswer := ""
	if row.Type == QuestionTypeFillBlank {
		standardAnswer = strings.TrimSpace(row.CorrectAnswer)
	}
	return CreateQuestionInput{
		TenantID:       tenantID,
		SpaceID:        spaceID,
		Type:           row.Type,
		Difficulty:     row.Difficulty,
		Title:          row.Title,
		Analysis:       row.Analysis,
		ScoreDefault:   "1",
		Options:        options,
		StandardAnswer: standardAnswer,
		Tags:           splitCSV(row.Tags),
	}
}

func parseImportOptions(rawOptions string, rawCorrect string) []QuestionOptionInput {
	correctKeys := map[string]bool{}
	for _, key := range splitCSV(rawCorrect) {
		correctKeys[key] = true
	}
	parts := strings.Split(rawOptions, "|")
	options := make([]QuestionOptionInput, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		key, content, ok := strings.Cut(part, ".")
		if !ok {
			key = part
			content = part
		}
		key = strings.TrimSpace(key)
		options = append(options, QuestionOptionInput{
			OptionKey:    key,
			Content:      strings.TrimSpace(content),
			IsCorrect:    correctKeys[key],
			IsDistractor: !correctKeys[key],
		})
	}
	return options
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			items = append(items, part)
		}
	}
	return items
}

func parseFillBlankAnswers(raw string) ([]string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	if !strings.HasPrefix(trimmed, "[") {
		return []string{trimmed}, nil
	}
	var items []string
	if err := json.Unmarshal([]byte(trimmed), &items); err != nil {
		return nil, err
	}
	normalized := make([]string, 0, len(items))
	for _, item := range items {
		value := strings.TrimSpace(item)
		if value == "" {
			return nil, ErrFillBlankNeedsStandardAnswer
		}
		normalized = append(normalized, value)
	}
	return normalized, nil
}

func sameStrings(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

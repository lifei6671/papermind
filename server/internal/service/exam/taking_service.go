package exam

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	// QuestionTypeSingle 表示单选题。
	QuestionTypeSingle = "single"
	// QuestionTypeMultiple 表示多选题。
	QuestionTypeMultiple = "multiple"
	// QuestionTypeJudge 表示判断题。
	QuestionTypeJudge = "judge"
	// QuestionTypeFillBlank 表示填空题。
	QuestionTypeFillBlank = "fill_blank"
	// QuestionTypeShortText 表示简答题。
	QuestionTypeShortText = "short_text"

	// EventTypeBlur 表示考生窗口失焦事件。
	EventTypeBlur = "blur"
	// EventTypeFocus 表示考生窗口聚焦事件。
	EventTypeFocus = "focus"
	// EventTypeSubmit 表示手动提交事件。
	EventTypeSubmit = "submit"
	// EventTypeAutoSubmit 表示自动交卷事件。
	EventTypeAutoSubmit = "auto_submit"
)

const (
	answerTransportGraceMillis int64 = 5 * 1000
	eventThrottleMillis        int64 = 1000
	defaultEventBufferSize           = 64
)

const (
	// GradingStatusAuto 表示客观题已经由系统自动判分。
	GradingStatusAuto = "auto"
	// GradingStatusPending 表示主观题等待教师人工阅卷。
	GradingStatusPending = "pending"
)

var (
	ErrUnsupportedQuestionType = errors.New("unsupported question type")
	ErrInvalidQuestionScore    = errors.New("invalid question score")
)

type Answer struct {
	TenantID          uint64 // 所属租户 ID。
	AttemptID         uint64 // 作答 ID。
	AttemptQuestionID uint64 // 考生题目快照 ID。
	AnswerContent     string // 答案内容，选择题保存选项 ID 或 JSON 数组。
	UpdatedAt         int64  // 更新时间，Unix 毫秒时间戳。
	UpdatedBy         uint64 // 更新人用户 ID。
}

type AnswerForGrading struct {
	AttemptQuestionID     uint64 // 考生题目快照 ID。
	QuestionType          string // 题型：single / multiple / judge / fill_blank / short_text。
	QuestionScore         string // 当前题目分值。
	AnswerContent         string // 考生答案内容。
	CorrectAnswerSnapshot string // 正确答案快照 JSON。
}

type AnswerGradingResult struct {
	AttemptQuestionID uint64 // 考生题目快照 ID。
	Score             string // 本题得分。
	GradingStatus     string // 阅卷状态：auto / pending。
}

type ObjectiveGradingFunc func(items []AnswerForGrading) ([]AnswerGradingResult, string, error)

type ExamEvent struct {
	TenantID  uint64 // 所属租户 ID。
	AttemptID uint64 // 作答 ID。
	EventType string // 事件类型。
	EventTime int64  // 事件发生时间，Unix 毫秒时间戳。
	Payload   string // 事件扩展信息 JSON。
}

type SaveAnswerInput struct {
	TenantID          uint64   // 所属租户 ID。
	AttemptID         uint64   // 作答 ID。
	AttemptQuestionID uint64   // 考生题目快照 ID。
	ExamToken         string   // 考试过程不透明 token。
	QuestionType      string   // 题型。
	OptionIDs         []uint64 // 选择题选项 ID。
	Text              string   // 填空题或简答题文本。
}

type SubmitInput struct {
	TenantID  uint64 // 所属租户 ID。
	AttemptID uint64 // 作答 ID。
	ExamToken string // 考试过程不透明 token。
	EventType string // submit / auto_submit。
}

type RecordEventInput struct {
	TenantID  uint64 // 所属租户 ID。
	AttemptID uint64 // 作答 ID。
	ExamToken string // 考试过程不透明 token。
	EventType string // 事件类型。
	Payload   string // 事件扩展信息 JSON。
}

type TakingRepository interface {
	FindAttemptByTokenHash(ctx context.Context, tokenHash string) (Attempt, error)
	GetExam(ctx context.Context, tenantID uint64, examID uint64) (Exam, error)
	UpsertAnswer(ctx context.Context, answer Answer) error
	// 提交事务内先锁定作答状态，再读取本次答案快照并调用 grader，最后原子写入分数和提交事件。
	SubmitAttemptAndGradeObjectiveQuestions(ctx context.Context, tenantID uint64, attemptID uint64, version int64, submittedAt int64, status string, grader ObjectiveGradingFunc, event ExamEvent) (int64, error)
	AppendEvent(ctx context.Context, event ExamEvent) error
}

type TakingLogger interface {
	Warn(message string, fields map[string]any)
}

type TakingServiceOptions struct {
	Repo            TakingRepository
	Now             func() int64
	EventBufferSize int
	Logger          TakingLogger
}

type TakingService struct {
	repo        TakingRepository
	now         func() int64
	events      chan ExamEvent
	logger      TakingLogger
	lastEventAt map[eventThrottleKey]int64
}

func NewTakingService(options TakingServiceOptions) *TakingService {
	now := options.Now
	if now == nil {
		now = func() int64 { return time.Now().UnixMilli() }
	}
	bufferSize := options.EventBufferSize
	if bufferSize <= 0 {
		bufferSize = defaultEventBufferSize
	}
	return &TakingService{
		repo:        options.Repo,
		now:         now,
		events:      make(chan ExamEvent, bufferSize),
		logger:      options.Logger,
		lastEventAt: make(map[eventThrottleKey]int64),
	}
}

func (s *TakingService) SaveAnswer(ctx context.Context, input SaveAnswerInput) error {
	attempt, exam, err := s.validateTakingToken(ctx, input.TenantID, input.AttemptID, input.ExamToken)
	if err != nil {
		return err
	}
	if attempt.Status != AttemptStatusInProgress {
		return ErrAttemptAlreadySubmitted
	}
	if s.now() > answerDeadline(attempt.StartedAt, exam)+answerTransportGraceMillis {
		return ErrAnswerDeadlineExceeded
	}
	return s.repo.UpsertAnswer(ctx, Answer{
		TenantID:          input.TenantID,
		AttemptID:         input.AttemptID,
		AttemptQuestionID: input.AttemptQuestionID,
		AnswerContent:     normalizeAnswer(input),
		UpdatedAt:         s.now(),
		UpdatedBy:         attempt.UserID,
	})
}

func (s *TakingService) Submit(ctx context.Context, input SubmitInput) error {
	attempt, _, err := s.validateTakingToken(ctx, input.TenantID, input.AttemptID, input.ExamToken)
	if err != nil {
		return err
	}
	if attempt.Status != AttemptStatusInProgress {
		return ErrAttemptAlreadySubmitted
	}
	event := ExamEvent{
		TenantID:  input.TenantID,
		AttemptID: input.AttemptID,
		EventType: input.EventType,
		EventTime: s.now(),
	}
	_, err = s.repo.SubmitAttemptAndGradeObjectiveQuestions(ctx, input.TenantID, input.AttemptID, attempt.Version, s.now(), AttemptStatusSubmitted, s.gradeObjectiveAnswers, event)
	return err
}

func (s *TakingService) RecordEvent(ctx context.Context, input RecordEventInput) error {
	if _, _, err := s.validateTakingToken(ctx, input.TenantID, input.AttemptID, input.ExamToken); err != nil {
		return err
	}
	event := ExamEvent{
		TenantID:  input.TenantID,
		AttemptID: input.AttemptID,
		EventType: input.EventType,
		EventTime: s.now(),
		Payload:   input.Payload,
	}
	if isCriticalEvent(input.EventType) {
		return s.repo.AppendEvent(ctx, event)
	}
	if s.shouldThrottle(event) {
		return nil
	}
	select {
	case s.events <- event:
		return nil
	default:
		if s.logger != nil {
			s.logger.Warn("drop non-critical exam event", map[string]any{
				"tenant_id":  input.TenantID,
				"attempt_id": input.AttemptID,
				"event_type": input.EventType,
			})
		}
		return nil
	}
}

func (s *TakingService) DrainEvents(ctx context.Context, limit int) (int, error) {
	drained := 0
	for limit <= 0 || drained < limit {
		select {
		case event := <-s.events:
			if err := s.repo.AppendEvent(ctx, event); err != nil {
				return drained, err
			}
			drained++
		default:
			return drained, nil
		}
	}
	return drained, nil
}

func (s *TakingService) StartEventConsumer(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-s.events:
				if err := s.repo.AppendEvent(ctx, event); err != nil && s.logger != nil {
					s.logger.Warn("append exam event failed", map[string]any{
						"tenant_id":  event.TenantID,
						"attempt_id": event.AttemptID,
						"event_type": event.EventType,
					})
				}
			}
		}
	}()
}

func (s *TakingService) PendingEventCount() int {
	return len(s.events)
}

func (s *TakingService) validateTakingToken(ctx context.Context, tenantID uint64, attemptID uint64, token string) (Attempt, Exam, error) {
	attempt, err := s.repo.FindAttemptByTokenHash(ctx, HashExamToken(token))
	if err != nil {
		return Attempt{}, Exam{}, ErrExamTokenInvalid
	}
	if attempt.TenantID != tenantID || attempt.ID != attemptID {
		return Attempt{}, Exam{}, ErrExamTokenAttemptMismatch
	}
	if attempt.ExamTokenExpiresAt < s.now() {
		return Attempt{}, Exam{}, ErrExamTokenInvalid
	}
	exam, err := s.repo.GetExam(ctx, tenantID, attempt.ExamID)
	if err != nil {
		return Attempt{}, Exam{}, err
	}
	return attempt, exam, nil
}

func (s *TakingService) shouldThrottle(event ExamEvent) bool {
	if event.EventType != EventTypeBlur && event.EventType != EventTypeFocus {
		return false
	}
	key := eventThrottleKey{tenantID: event.TenantID, attemptID: event.AttemptID, eventType: event.EventType}
	lastAt, exists := s.lastEventAt[key]
	if exists && event.EventTime-lastAt < eventThrottleMillis {
		return true
	}
	s.lastEventAt[key] = event.EventTime
	return false
}

func normalizeAnswer(input SaveAnswerInput) string {
	switch input.QuestionType {
	case QuestionTypeSingle, QuestionTypeJudge:
		if len(input.OptionIDs) == 0 {
			return ""
		}
		data, _ := json.Marshal(input.OptionIDs[0])
		return string(data)
	case QuestionTypeMultiple:
		optionIDs := append([]uint64(nil), input.OptionIDs...)
		sort.Slice(optionIDs, func(i, j int) bool { return optionIDs[i] < optionIDs[j] })
		data, _ := json.Marshal(optionIDs)
		return string(data)
	default:
		return input.Text
	}
}

func (s *TakingService) gradeObjectiveAnswers(items []AnswerForGrading) ([]AnswerGradingResult, string, error) {
	grades := make([]AnswerGradingResult, 0, len(items))
	objectiveScore := 0.0
	for _, item := range items {
		// 自动判分只处理客观题和首版单空填空题，简答题保留给人工阅卷流程。
		result, score, err := gradeAnswer(item)
		if err != nil {
			return nil, "", err
		}
		grades = append(grades, result)
		objectiveScore += score
	}
	return grades, formatGradingScore(objectiveScore), nil
}

func gradeAnswer(item AnswerForGrading) (AnswerGradingResult, float64, error) {
	result := AnswerGradingResult{
		AttemptQuestionID: item.AttemptQuestionID,
		Score:             "0",
		GradingStatus:     GradingStatusAuto,
	}
	correct := false
	var err error
	if item.QuestionType == QuestionTypeShortText {
		result.GradingStatus = GradingStatusPending
		return result, 0, nil
	}
	snapshot, err := parseCorrectAnswerSnapshot(item.CorrectAnswerSnapshot)
	if err != nil {
		return AnswerGradingResult{}, 0, err
	}
	switch item.QuestionType {
	case QuestionTypeSingle, QuestionTypeJudge:
		correct, err = gradeSingleOptionAnswer(item.AnswerContent, snapshot.OptionIDs)
	case QuestionTypeMultiple:
		correct, err = gradeMultipleOptionAnswer(item.AnswerContent, snapshot.OptionIDs)
	case QuestionTypeFillBlank:
		correct = strings.TrimSpace(item.AnswerContent) == strings.TrimSpace(snapshot.Text)
	default:
		return AnswerGradingResult{}, 0, ErrUnsupportedQuestionType
	}
	if err != nil {
		return AnswerGradingResult{}, 0, err
	}
	score, err := parseGradingScore(item.QuestionScore)
	if err != nil {
		return AnswerGradingResult{}, 0, err
	}
	if !correct {
		return result, 0, nil
	}
	result.Score = formatGradingScore(score)
	return result, score, nil
}

func gradeSingleOptionAnswer(answer string, correctOptionIDs []uint64) (bool, error) {
	if strings.TrimSpace(answer) == "" {
		return false, nil
	}
	var selectedID uint64
	if err := json.Unmarshal([]byte(answer), &selectedID); err != nil {
		return false, err
	}
	return sameUint64Set([]uint64{selectedID}, correctOptionIDs), nil
}

func gradeMultipleOptionAnswer(answer string, correctOptionIDs []uint64) (bool, error) {
	if strings.TrimSpace(answer) == "" {
		return false, nil
	}
	var selectedIDs []uint64
	if err := json.Unmarshal([]byte(answer), &selectedIDs); err != nil {
		return false, err
	}
	// 多选题必须反序列化为数组后排序比较，避免 JSON 字符串空格或顺序差异影响判分。
	return sameUint64Set(selectedIDs, correctOptionIDs), nil
}

func parseCorrectAnswerSnapshot(value string) (correctAnswerSnapshot, error) {
	var snapshot correctAnswerSnapshot
	if err := json.Unmarshal([]byte(value), &snapshot); err != nil {
		return correctAnswerSnapshot{}, err
	}
	return snapshot, nil
}

func sameUint64Set(left []uint64, right []uint64) bool {
	left = sortedUint64s(left)
	right = sortedUint64s(right)
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

func sortedUint64s(values []uint64) []uint64 {
	sorted := append([]uint64(nil), values...)
	sort.Slice(sorted, func(i int, j int) bool {
		return sorted[i] < sorted[j]
	})
	return sorted
}

func parseGradingScore(value string) (float64, error) {
	if strings.TrimSpace(value) == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, ErrInvalidQuestionScore
	}
	return parsed, nil
}

func formatGradingScore(value float64) string {
	text := strconv.FormatFloat(value, 'f', 6, 64)
	text = strings.TrimRight(text, "0")
	text = strings.TrimRight(text, ".")
	if text == "" {
		return "0"
	}
	return text
}

func isCriticalEvent(eventType string) bool {
	return eventType == EventTypeSubmit || eventType == EventTypeAutoSubmit
}

type correctAnswerSnapshot struct {
	OptionIDs []uint64 `json:"option_ids"`
	Text      string   `json:"text"`
}

type eventThrottleKey struct {
	tenantID  uint64
	attemptID uint64
	eventType string
}

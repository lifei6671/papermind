package exam

import (
	"context"
	"encoding/json"
	"sort"
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

type Answer struct {
	TenantID          uint64 // 所属租户 ID。
	AttemptID         uint64 // 作答 ID。
	AttemptQuestionID uint64 // 考生题目快照 ID。
	AnswerContent     string // 答案内容，选择题保存选项 ID 或 JSON 数组。
	UpdatedAt         int64  // 更新时间，Unix 毫秒时间戳。
	UpdatedBy         uint64 // 更新人用户 ID。
}

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
	SubmitAttempt(ctx context.Context, tenantID uint64, attemptID uint64, version int64, submittedAt int64, status string) (int64, error)
	GradeObjectiveQuestions(ctx context.Context, tenantID uint64, attemptID uint64) error
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
	rowsAffected, err := s.repo.SubmitAttempt(ctx, input.TenantID, input.AttemptID, attempt.Version, s.now(), AttemptStatusSubmitted)
	if err != nil {
		return err
	}
	if rowsAffected > 0 {
		if err := s.repo.GradeObjectiveQuestions(ctx, input.TenantID, input.AttemptID); err != nil {
			return err
		}
	}
	return s.repo.AppendEvent(ctx, ExamEvent{
		TenantID:  input.TenantID,
		AttemptID: input.AttemptID,
		EventType: input.EventType,
		EventTime: s.now(),
	})
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

func isCriticalEvent(eventType string) bool {
	return eventType == EventTypeSubmit || eventType == EventTypeAutoSubmit
}

type eventThrottleKey struct {
	tenantID  uint64
	attemptID uint64
	eventType string
}

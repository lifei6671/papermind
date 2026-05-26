package exam

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSaveAnswerValidatesExamTokenDeadlineAndNormalizesAnswers(t *testing.T) {
	repo := newFakeTakingRepository()
	repo.exam = Exam{ID: 1, TenantID: 10, EndTime: fixedUnixMilli + 60*minuteMillis, DurationMinutes: 30}
	repo.attempt = Attempt{
		ID:                 99,
		TenantID:           10,
		ExamID:             1,
		UserID:             20,
		Status:             AttemptStatusInProgress,
		StartedAt:          fixedUnixMilli,
		ExamTokenHash:      HashExamToken("exam-token"),
		ExamTokenExpiresAt: fixedUnixMilli + 35*minuteMillis,
	}
	svc := NewTakingService(TakingServiceOptions{Repo: repo, Now: repo.currentTime})

	err := svc.SaveAnswer(context.Background(), SaveAnswerInput{
		TenantID:          10,
		AttemptID:         99,
		AttemptQuestionID: 9001,
		ExamToken:         "exam-token",
		QuestionType:      QuestionTypeMultiple,
		OptionIDs:         []uint64{7, 3, 5},
	})
	if err != nil {
		t.Fatalf("SaveAnswer returned error: %v", err)
	}
	if repo.savedAnswer.AnswerContent != "[3,5,7]" {
		t.Fatalf("expected multiple choice answer sorted JSON, got %q", repo.savedAnswer.AnswerContent)
	}

	repo.now = fixedUnixMilli + 30*minuteMillis + answerTransportGraceMillis
	err = svc.SaveAnswer(context.Background(), SaveAnswerInput{
		TenantID:          10,
		AttemptID:         99,
		AttemptQuestionID: 9002,
		ExamToken:         "exam-token",
		QuestionType:      QuestionTypeFillBlank,
		Text:              "go.mod",
	})
	if err != nil {
		t.Fatalf("SaveAnswer should allow transport grace: %v", err)
	}

	repo.now = fixedUnixMilli + 30*minuteMillis + answerTransportGraceMillis + 1
	err = svc.SaveAnswer(context.Background(), SaveAnswerInput{
		TenantID:          10,
		AttemptID:         99,
		AttemptQuestionID: 9003,
		ExamToken:         "exam-token",
		QuestionType:      QuestionTypeShortText,
		Text:              "answer",
	})
	if !errors.Is(err, ErrAnswerDeadlineExceeded) {
		t.Fatalf("expected ErrAnswerDeadlineExceeded, got %v", err)
	}

	repo.now = fixedUnixMilli
	repo.attempt.Status = AttemptStatusSubmitted
	err = svc.SaveAnswer(context.Background(), SaveAnswerInput{
		TenantID:          10,
		AttemptID:         99,
		AttemptQuestionID: 9004,
		ExamToken:         "exam-token",
		QuestionType:      QuestionTypeSingle,
		OptionIDs:         []uint64{1},
	})
	if !errors.Is(err, ErrAttemptAlreadySubmitted) {
		t.Fatalf("expected ErrAttemptAlreadySubmitted, got %v", err)
	}
}

func TestSubmitAttemptUsesVersionedIdempotentUpdateAndWritesCriticalEvent(t *testing.T) {
	repo := newFakeTakingRepository()
	repo.exam = Exam{ID: 1, TenantID: 10, EndTime: fixedUnixMilli + 60*minuteMillis, DurationMinutes: 30}
	repo.attempt = Attempt{
		ID:                 99,
		TenantID:           10,
		ExamID:             1,
		UserID:             20,
		Status:             AttemptStatusInProgress,
		StartedAt:          fixedUnixMilli,
		Version:            3,
		ExamTokenHash:      HashExamToken("exam-token"),
		ExamTokenExpiresAt: fixedUnixMilli + 35*minuteMillis,
	}
	svc := NewTakingService(TakingServiceOptions{Repo: repo, Now: fixedNow})

	err := svc.Submit(context.Background(), SubmitInput{
		TenantID:  10,
		AttemptID: 99,
		ExamToken: "exam-token",
		EventType: EventTypeSubmit,
	})
	if err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}
	if repo.submittedStatus != AttemptStatusSubmitted || repo.submittedVersion != 3 {
		t.Fatalf("expected versioned status update, got status=%q version=%d", repo.submittedStatus, repo.submittedVersion)
	}
	if repo.gradeAttemptID != 99 {
		t.Fatalf("expected objective grading triggered, got %d", repo.gradeAttemptID)
	}
	if len(repo.events) != 1 || repo.events[0].EventType != EventTypeSubmit {
		t.Fatalf("expected critical submit event written, got %#v", repo.events)
	}

	repo.updateRowsAffected = 0
	err = svc.Submit(context.Background(), SubmitInput{
		TenantID:  10,
		AttemptID: 99,
		ExamToken: "exam-token",
		EventType: EventTypeAutoSubmit,
	})
	if err != nil {
		t.Fatalf("Submit should treat concurrent status update as idempotent success: %v", err)
	}
	if len(repo.events) != 2 || repo.events[1].EventType != EventTypeAutoSubmit {
		t.Fatalf("expected auto_submit event still written, got %#v", repo.events)
	}
	if repo.savedGradingCount != 1 {
		t.Fatalf("idempotent submit should not save objective grades again, got %d", repo.savedGradingCount)
	}
}

func TestSubmitDoesNotMarkAttemptSubmittedWhenObjectiveGradingCannotBePrepared(t *testing.T) {
	repo := newFakeTakingRepository()
	repo.exam = Exam{ID: 1, TenantID: 10, EndTime: fixedUnixMilli + 60*minuteMillis, DurationMinutes: 30}
	repo.attempt = Attempt{
		ID:                 99,
		TenantID:           10,
		ExamID:             1,
		UserID:             20,
		Status:             AttemptStatusInProgress,
		StartedAt:          fixedUnixMilli,
		Version:            3,
		ExamTokenHash:      HashExamToken("exam-token"),
		ExamTokenExpiresAt: fixedUnixMilli + 35*minuteMillis,
	}
	repo.gradingItems = []AnswerForGrading{
		{
			AttemptQuestionID:     1001,
			QuestionType:          QuestionTypeSingle,
			QuestionScore:         "not-a-score",
			AnswerContent:         "101",
			CorrectAnswerSnapshot: `{"option_ids":[101],"text":""}`,
		},
	}
	svc := NewTakingService(TakingServiceOptions{Repo: repo, Now: fixedNow})

	err := svc.Submit(context.Background(), SubmitInput{
		TenantID:  10,
		AttemptID: 99,
		ExamToken: "exam-token",
		EventType: EventTypeSubmit,
	})
	if !errors.Is(err, ErrInvalidQuestionScore) {
		t.Fatalf("expected grading preparation error, got %v", err)
	}
	if repo.submittedStatus != "" {
		t.Fatalf("attempt should not be marked submitted before grading is prepared, got %q", repo.submittedStatus)
	}
	if len(repo.events) != 0 {
		t.Fatalf("submit event should not be written when submit is not persisted, got %#v", repo.events)
	}
}

func TestExamEventsThrottleDropNonCriticalAndKeepCriticalReliable(t *testing.T) {
	repo := newFakeTakingRepository()
	repo.exam = Exam{ID: 1, TenantID: 10, EndTime: fixedUnixMilli + 60*minuteMillis, DurationMinutes: 30}
	repo.attempt = Attempt{
		ID:                 99,
		TenantID:           10,
		ExamID:             1,
		Status:             AttemptStatusInProgress,
		StartedAt:          fixedUnixMilli,
		ExamTokenHash:      HashExamToken("exam-token"),
		ExamTokenExpiresAt: fixedUnixMilli + 35*minuteMillis,
	}
	logger := &fakeTakingLogger{}
	svc := NewTakingService(TakingServiceOptions{Repo: repo, Now: repo.currentTime, EventBufferSize: 1, Logger: logger})

	err := svc.RecordEvent(context.Background(), RecordEventInput{TenantID: 10, AttemptID: 99, ExamToken: "exam-token", EventType: EventTypeBlur})
	if err != nil {
		t.Fatalf("RecordEvent blur returned error: %v", err)
	}
	repo.now += 500
	if err := svc.RecordEvent(context.Background(), RecordEventInput{TenantID: 10, AttemptID: 99, ExamToken: "exam-token", EventType: EventTypeBlur}); err != nil {
		t.Fatalf("RecordEvent throttled blur returned error: %v", err)
	}
	if queued := svc.PendingEventCount(); queued != 1 {
		t.Fatalf("expected throttled duplicate not queued, got %d", queued)
	}

	repo.now += eventThrottleMillis + 1
	if err := svc.RecordEvent(context.Background(), RecordEventInput{TenantID: 10, AttemptID: 99, ExamToken: "exam-token", EventType: EventTypeFocus}); err != nil {
		t.Fatalf("RecordEvent full queue focus returned error: %v", err)
	}
	if logger.warnCount != 1 {
		t.Fatalf("expected non-critical event drop warning, got %d", logger.warnCount)
	}

	if _, err := svc.DrainEvents(context.Background(), 10); err != nil {
		t.Fatalf("DrainEvents returned error: %v", err)
	}
	if len(repo.events) != 1 || repo.events[0].EventType != EventTypeBlur {
		t.Fatalf("expected async consumer to persist queued blur event, got %#v", repo.events)
	}

	if err := svc.RecordEvent(context.Background(), RecordEventInput{TenantID: 10, AttemptID: 99, ExamToken: "exam-token", EventType: EventTypeSubmit}); err != nil {
		t.Fatalf("critical submit event should not be dropped: %v", err)
	}
	if len(repo.events) != 2 || repo.events[1].EventType != EventTypeSubmit {
		t.Fatalf("expected critical submit event written synchronously, got %#v", repo.events)
	}
}

func TestStartEventConsumerPersistsQueuedEventsAsynchronously(t *testing.T) {
	repo := newFakeTakingRepository()
	repo.eventAppended = make(chan struct{}, 1)
	repo.exam = Exam{ID: 1, TenantID: 10, EndTime: fixedUnixMilli + 60*minuteMillis, DurationMinutes: 30}
	repo.attempt = Attempt{
		ID:                 99,
		TenantID:           10,
		ExamID:             1,
		Status:             AttemptStatusInProgress,
		StartedAt:          fixedUnixMilli,
		ExamTokenHash:      HashExamToken("exam-token"),
		ExamTokenExpiresAt: fixedUnixMilli + 35*minuteMillis,
	}
	svc := NewTakingService(TakingServiceOptions{Repo: repo, Now: repo.currentTime, EventBufferSize: 1})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svc.StartEventConsumer(ctx)
	if err := svc.RecordEvent(context.Background(), RecordEventInput{TenantID: 10, AttemptID: 99, ExamToken: "exam-token", EventType: EventTypeFocus}); err != nil {
		t.Fatalf("RecordEvent focus returned error: %v", err)
	}

	select {
	case <-repo.eventAppended:
	case <-time.After(time.Second):
		t.Fatal("expected started consumer to persist queued event")
	}
	if len(repo.events) != 1 || repo.events[0].EventType != EventTypeFocus {
		t.Fatalf("expected focus event persisted by consumer, got %#v", repo.events)
	}
}

func TestGradeObjectiveQuestionsAppliesSnapshotRulesAndLeavesShortTextPending(t *testing.T) {
	repo := newFakeTakingRepository()
	repo.gradingItems = []AnswerForGrading{
		{
			AttemptQuestionID:     1001,
			QuestionType:          QuestionTypeSingle,
			QuestionScore:         "2",
			AnswerContent:         "101",
			CorrectAnswerSnapshot: `{"option_ids":[101],"text":""}`,
		},
		{
			AttemptQuestionID:     1002,
			QuestionType:          QuestionTypeJudge,
			QuestionScore:         "1",
			AnswerContent:         "202",
			CorrectAnswerSnapshot: `{"option_ids":[202],"text":""}`,
		},
		{
			AttemptQuestionID:     1003,
			QuestionType:          QuestionTypeMultiple,
			QuestionScore:         "3",
			AnswerContent:         "[104, 101]",
			CorrectAnswerSnapshot: `{"option_ids":[101,104],"text":""}`,
		},
		{
			AttemptQuestionID:     1004,
			QuestionType:          QuestionTypeFillBlank,
			QuestionScore:         "4",
			AnswerContent:         " go.mod ",
			CorrectAnswerSnapshot: `{"option_ids":[],"text":"go.mod"}`,
		},
		{
			AttemptQuestionID:     1005,
			QuestionType:          QuestionTypeShortText,
			QuestionScore:         "5",
			AnswerContent:         "简答题需要老师人工阅卷",
			CorrectAnswerSnapshot: "",
		},
		{
			AttemptQuestionID:     1006,
			QuestionType:          QuestionTypeMultiple,
			QuestionScore:         "3",
			AnswerContent:         "[101]",
			CorrectAnswerSnapshot: `{"option_ids":[101,104],"text":""}`,
		},
	}
	svc := NewTakingService(TakingServiceOptions{Repo: repo, Now: fixedNow})

	grades, objectiveScore, err := svc.gradeObjectiveAnswers(repo.gradingItems)
	if err != nil {
		t.Fatalf("gradeObjectiveAnswers returned error: %v", err)
	}
	repo.savedGrades = grades
	repo.savedObjectiveScore = objectiveScore

	want := []AnswerGradingResult{
		{AttemptQuestionID: 1001, Score: "2", GradingStatus: GradingStatusAuto},
		{AttemptQuestionID: 1002, Score: "1", GradingStatus: GradingStatusAuto},
		{AttemptQuestionID: 1003, Score: "3", GradingStatus: GradingStatusAuto},
		{AttemptQuestionID: 1004, Score: "4", GradingStatus: GradingStatusAuto},
		{AttemptQuestionID: 1005, Score: "0", GradingStatus: GradingStatusPending},
		{AttemptQuestionID: 1006, Score: "0", GradingStatus: GradingStatusAuto},
	}
	if len(repo.savedGrades) != len(want) {
		t.Fatalf("expected %d grading results, got %#v", len(want), repo.savedGrades)
	}
	for index := range want {
		if repo.savedGrades[index] != want[index] {
			t.Fatalf("grading result %d mismatch: want %#v got %#v", index, want[index], repo.savedGrades[index])
		}
	}
	if repo.savedObjectiveScore != "10" {
		t.Fatalf("expected objective score 10, got %q", repo.savedObjectiveScore)
	}
}

func TestGradeObjectiveQuestionsFailsFastWhenMatchedScoreIsInvalid(t *testing.T) {
	repo := newFakeTakingRepository()
	svc := NewTakingService(TakingServiceOptions{Repo: repo, Now: fixedNow})

	_, _, err := svc.gradeObjectiveAnswers([]AnswerForGrading{
		{
			AttemptQuestionID:     1001,
			QuestionType:          QuestionTypeSingle,
			QuestionScore:         "not-a-score",
			AnswerContent:         "101",
			CorrectAnswerSnapshot: `{"option_ids":[101],"text":""}`,
		},
	})
	if !errors.Is(err, ErrInvalidQuestionScore) {
		t.Fatalf("expected ErrInvalidQuestionScore for correct answer, got %v", err)
	}

	_, _, err = svc.gradeObjectiveAnswers([]AnswerForGrading{
		{
			AttemptQuestionID:     1002,
			QuestionType:          QuestionTypeSingle,
			QuestionScore:         "not-a-score",
			AnswerContent:         "102",
			CorrectAnswerSnapshot: `{"option_ids":[101],"text":""}`,
		},
	})
	if !errors.Is(err, ErrInvalidQuestionScore) {
		t.Fatalf("expected ErrInvalidQuestionScore for wrong answer, got %v", err)
	}
}

func newFakeTakingRepository() *fakeTakingRepository {
	return &fakeTakingRepository{
		now:                fixedUnixMilli,
		updateRowsAffected: 1,
	}
}

type fakeTakingRepository struct {
	now int64

	exam    Exam
	attempt Attempt

	savedAnswer Answer

	submittedStatus     string
	submittedVersion    int64
	updateRowsAffected  int64
	gradeAttemptID      uint64
	gradingItems        []AnswerForGrading
	savedGrades         []AnswerGradingResult
	savedObjectiveScore string
	savedGradingCount   int64

	events []ExamEvent

	eventAppended chan struct{}
}

func (r *fakeTakingRepository) currentTime() int64 {
	return r.now
}

func (r *fakeTakingRepository) FindAttemptByTokenHash(ctx context.Context, tokenHash string) (Attempt, error) {
	if tokenHash != r.attempt.ExamTokenHash {
		return Attempt{}, ErrAttemptNotFound
	}
	return r.attempt, nil
}

func (r *fakeTakingRepository) GetExam(ctx context.Context, tenantID uint64, examID uint64) (Exam, error) {
	return r.exam, nil
}

func (r *fakeTakingRepository) UpsertAnswer(ctx context.Context, answer Answer) error {
	r.savedAnswer = answer
	return nil
}

func (r *fakeTakingRepository) SubmitAttemptAndGradeObjectiveQuestions(ctx context.Context, tenantID uint64, attemptID uint64, version int64, submittedAt int64, status string, grader ObjectiveGradingFunc, event ExamEvent) (int64, error) {
	r.submittedStatus = status
	r.submittedVersion = version
	r.gradeAttemptID = attemptID
	if r.updateRowsAffected > 0 {
		grades, objectiveScore, err := grader(r.gradingItems)
		if err != nil {
			r.submittedStatus = ""
			r.submittedVersion = 0
			return 0, err
		}
		r.savedGrades = grades
		r.savedObjectiveScore = objectiveScore
		r.savedGradingCount++
	}
	r.events = append(r.events, event)
	return r.updateRowsAffected, nil
}

func (r *fakeTakingRepository) AppendEvent(ctx context.Context, event ExamEvent) error {
	r.events = append(r.events, event)
	if r.eventAppended != nil {
		select {
		case r.eventAppended <- struct{}{}:
		default:
		}
	}
	return nil
}

type fakeTakingLogger struct {
	warnCount int
}

func (l *fakeTakingLogger) Warn(message string, fields map[string]any) {
	l.warnCount++
}

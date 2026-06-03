package exam

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/lifei6671/papermind/server/internal/service/pagination"
	"github.com/lifei6671/papermind/server/library/constant"
)

func TestCreateDraftAndPublishExamValidatesSettingsAndFreezesRuleLivePool(t *testing.T) {
	repo := &fakeRepository{
		inviteCodes: map[string]bool{"DUPLICATE": true},
		papers: map[uint64]Paper{
			100: {ID: 100, BuildMode: BuildModeRuleLive, Status: constant.PaperStatusEnabled},
			101: {ID: 101, BuildMode: BuildModeManual, ContainsShortText: true, Status: constant.PaperStatusEnabled},
		},
		liveCandidates: []LivePoolItem{
			{SectionID: 10, RuleID: 1, QuestionID: 300},
		},
	}
	generator := &fakeCodeGenerator{codes: []string{"DUPLICATE", "INVITE001"}}
	svc := NewService(ServiceOptions{
		Repo:          repo,
		CodeGenerator: generator,
		TokenIssuer:   fakeTokenIssuer{token: "exam-token"},
		Now:           fixedNow,
	})

	draft, err := svc.CreateDraft(context.Background(), CreateDraftInput{
		TenantID: 10,
		PaperID:  100,
		Name:     "期中考试",
	})
	if err != nil {
		t.Fatalf("CreateDraft returned error: %v", err)
	}
	if draft.Status != StatusDraft || draft.MaxAttempts != 1 || draft.ResultStrategy != ResultStrategyLatest {
		t.Fatalf("expected draft defaults, got %#v", draft)
	}

	_, err = svc.Publish(context.Background(), PublishInput{
		TenantID:        10,
		ExamID:          1,
		PaperID:         100,
		StartTime:       fixedUnixMilli,
		EndTime:         fixedUnixMilli + 30*minuteMillis,
		DurationMinutes: 60,
		MaxAttempts:     1,
		ResultStrategy:  ResultStrategyLatest,
		PublishMode:     PublishModeImmediateScore,
	})
	if !errors.Is(err, ErrDurationExceedsExamWindow) {
		t.Fatalf("expected ErrDurationExceedsExamWindow, got %v", err)
	}

	_, err = svc.Publish(context.Background(), PublishInput{
		TenantID:        10,
		ExamID:          2,
		PaperID:         101,
		StartTime:       fixedUnixMilli,
		EndTime:         fixedUnixMilli + 120*minuteMillis,
		DurationMinutes: 60,
		MaxAttempts:     2,
		ResultStrategy:  ResultStrategyHighest,
		PublishMode:     PublishModeManualPublish,
	})
	if !errors.Is(err, ErrShortTextCannotRepeatAttempt) {
		t.Fatalf("expected ErrShortTextCannotRepeatAttempt, got %v", err)
	}

	_, err = svc.Publish(context.Background(), PublishInput{
		TenantID:        10,
		ExamID:          3,
		PaperID:         101,
		StartTime:       fixedUnixMilli,
		EndTime:         fixedUnixMilli + 120*minuteMillis,
		DurationMinutes: 60,
		MaxAttempts:     1,
		ResultStrategy:  ResultStrategyLatest,
		PublishMode:     PublishModeImmediateScore,
	})
	if !errors.Is(err, ErrShortTextCannotImmediateScore) {
		t.Fatalf("expected ErrShortTextCannotImmediateScore, got %v", err)
	}

	scorePublishTime := fixedUnixMilli + 90*minuteMillis
	published, err := svc.Publish(context.Background(), PublishInput{
		TenantID:         10,
		ExamID:           1,
		PaperID:          100,
		StartTime:        fixedUnixMilli,
		EndTime:          fixedUnixMilli + 120*minuteMillis,
		DurationMinutes:  60,
		MaxAttempts:      1,
		ResultStrategy:   ResultStrategyLatest,
		PublishMode:      PublishModeManualPublish,
		ScorePublishTime: &scorePublishTime,
	})
	if err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}
	if published.InviteCode != "INVITE001" {
		t.Fatalf("expected unique invite code INVITE001, got %q", published.InviteCode)
	}
	if published.Status != StatusPublished || published.ScorePublishTime == nil || *published.ScorePublishTime != scorePublishTime {
		t.Fatalf("expected published exam with score publish time, got %#v", published)
	}
	if !repo.frozeLivePool || len(repo.frozenPool) != 1 {
		t.Fatalf("expected rule_live pool frozen, got %#v", repo.frozenPool)
	}
}

func TestPublishRejectsPaperThatIsNotEnabled(t *testing.T) {
	repo := &fakeRepository{
		papers: map[uint64]Paper{
			100: {ID: 100, BuildMode: BuildModeManual, Status: constant.PaperStatusDisabled},
			101: {ID: 101, BuildMode: BuildModeManual, Status: constant.PaperStatusDraft},
		},
	}
	svc := NewService(ServiceOptions{
		Repo:          repo,
		CodeGenerator: &fakeCodeGenerator{codes: []string{"INVITE001"}},
		TokenIssuer:   fakeTokenIssuer{token: "exam-token"},
		Now:           fixedNow,
	})

	for _, paperID := range []uint64{100, 101} {
		_, err := svc.Publish(context.Background(), PublishInput{
		TenantID:        10,
		ExamID:          1,
		PaperID:         paperID,
		StartTime:       fixedUnixMilli,
		EndTime:         fixedUnixMilli + 120*minuteMillis,
		DurationMinutes: 60,
		MaxAttempts:     1,
		ResultStrategy:  ResultStrategyLatest,
		PublishMode:     PublishModeManualPublish,
	})
		if !errors.Is(err, ErrPaperNotEnabled) {
			t.Fatalf("paper %d: expected ErrPaperNotEnabled, got %v", paperID, err)
		}
	}
	if repo.updatedExam.ID != 0 {
		t.Fatalf("expected non-enabled paper to stop publish before persistence, got %#v", repo.updatedExam)
	}
}

func TestTargetsRejectDuplicatesAndInviteRequiresLogin(t *testing.T) {
	repo := &fakeRepository{
		targets: map[targetKey]bool{
			{tenantID: 10, examID: 1, targetType: TargetTypeSpace, targetID: 100}: true,
		},
		examsByInvite: map[string]Exam{
			"INVITE001": {ID: 1, TenantID: 10, Status: StatusPublished},
		},
	}
	svc := NewService(ServiceOptions{Repo: repo, Now: fixedNow})

	err := svc.AddTarget(context.Background(), AddTargetInput{
		TenantID:   10,
		ExamID:     1,
		TargetType: TargetTypeSpace,
		TargetID:   100,
	})
	if !errors.Is(err, ErrDuplicateExamTarget) {
		t.Fatalf("expected ErrDuplicateExamTarget, got %v", err)
	}

	if err := svc.AddTarget(context.Background(), AddTargetInput{TenantID: 10, ExamID: 1, TargetType: TargetTypeUser, TargetID: 20}); err != nil {
		t.Fatalf("AddTarget user returned error: %v", err)
	}
	if repo.addedTarget.TargetType != TargetTypeUser || repo.addedTarget.TargetID != 20 {
		t.Fatalf("expected user target added, got %#v", repo.addedTarget)
	}

	_, err = svc.ResolveInvite(context.Background(), ResolveInviteInput{InviteCode: "INVITE001"})
	if !errors.Is(err, ErrLoginRequiredForInvite) {
		t.Fatalf("expected ErrLoginRequiredForInvite, got %v", err)
	}

	exam, err := svc.ResolveInvite(context.Background(), ResolveInviteInput{InviteCode: "INVITE001", UserID: 20})
	if err != nil {
		t.Fatalf("ResolveInvite returned error: %v", err)
	}
	if exam.ID != 1 {
		t.Fatalf("expected exam ID 1, got %d", exam.ID)
	}
}

func TestStartExamIsIdempotentAndIssuesOpaqueAttemptToken(t *testing.T) {
	existingTokenHash := HashExamToken("existing-token")
	repo := &fakeRepository{
		exams: map[uint64]Exam{
			1: {ID: 1, TenantID: 10, PaperID: 100, StartTime: fixedUnixMilli - minuteMillis, EndTime: fixedUnixMilli + 60*minuteMillis, DurationMinutes: 30, MaxAttempts: 2, Status: StatusPublished},
		},
		eligible: true,
		existingInProgress: &Attempt{
			ID:                 99,
			TenantID:           10,
			ExamID:             1,
			UserID:             20,
			AttemptNo:          1,
			Status:             AttemptStatusInProgress,
			ExamTokenHash:      existingTokenHash,
			ExamTokenExpiresAt: fixedUnixMilli + 30*minuteMillis + defaultTokenBufferMillis,
		},
	}
	svc := NewService(ServiceOptions{
		Repo:        repo,
		TokenIssuer: fakeTokenIssuer{token: "exam-token"},
		Now:         fixedNow,
	})

	started, err := svc.StartExam(context.Background(), StartInput{TenantID: 10, ExamID: 1, UserID: 20})
	if err != nil {
		t.Fatalf("StartExam returned error: %v", err)
	}
	if started.Attempt.ID != 99 || repo.createdAttempt.ID != 0 {
		t.Fatalf("expected existing in_progress attempt, got started=%#v created=%#v", started.Attempt, repo.createdAttempt)
	}
	if started.ExamToken != "exam-token" {
		t.Fatalf("expected existing attempt to receive a fresh token, got %q", started.ExamToken)
	}
	if started.Attempt.ExamTokenHash != HashExamToken("exam-token") || started.Attempt.ExamTokenHash == existingTokenHash || repo.updateAttemptTokenCount != 1 {
		t.Fatalf("expected existing attempt token to be refreshed, got started=%#v updates=%d", started, repo.updateAttemptTokenCount)
	}

	repo.existingInProgress = nil
	repo.attemptCount = 2
	_, err = svc.StartExam(context.Background(), StartInput{TenantID: 10, ExamID: 1, UserID: 20})
	if !errors.Is(err, ErrMaxAttemptsReached) {
		t.Fatalf("expected ErrMaxAttemptsReached, got %v", err)
	}

	repo.attemptCount = 1
	repo.createdAttempt = Attempt{}
	started, err = svc.StartExam(context.Background(), StartInput{TenantID: 10, ExamID: 1, UserID: 20})
	if err != nil {
		t.Fatalf("StartExam create returned error: %v", err)
	}
	if started.ExamToken != "exam-token" {
		t.Fatalf("expected opaque token returned, got %q", started.ExamToken)
	}
	if repo.createdAttempt.AttemptNo != 2 {
		t.Fatalf("expected next attempt no 2, got %d", repo.createdAttempt.AttemptNo)
	}
	if repo.createdAttempt.ExamTokenHash == "" || repo.createdAttempt.ExamTokenHash == "exam-token" {
		t.Fatalf("expected only token hash saved, got %q", repo.createdAttempt.ExamTokenHash)
	}
	if repo.createdAttempt.ExamTokenExpiresAt != fixedUnixMilli+30*minuteMillis+defaultTokenBufferMillis {
		t.Fatalf("unexpected token expiry: %d", repo.createdAttempt.ExamTokenExpiresAt)
	}

	repo.conflictOnCreate = true
	repo.existingInProgress = nil
	repo.conflictCreated = false
	started, err = svc.StartExam(context.Background(), StartInput{TenantID: 10, ExamID: 1, UserID: 20})
	if err != nil {
		t.Fatalf("StartExam conflict returned error: %v", err)
	}
	if started.Attempt.ID != 100 || repo.retriedAfterConflict != 1 {
		t.Fatalf("expected conflict to query existing in_progress only, got attempt=%#v retries=%d", started.Attempt, repo.retriedAfterConflict)
	}
	if started.ExamToken != "exam-token" || repo.updateAttemptTokenCount != 2 {
		t.Fatalf("expected conflict path to renew token, got token=%q updates=%d", started.ExamToken, repo.updateAttemptTokenCount)
	}
}

func TestStartExamUsesConfiguredTokenBuffer(t *testing.T) {
	repo := &fakeRepository{
		exams: map[uint64]Exam{
			1: {ID: 1, TenantID: 10, PaperID: 100, StartTime: fixedUnixMilli - minuteMillis, EndTime: fixedUnixMilli + 60*minuteMillis, DurationMinutes: 30, MaxAttempts: 1, Status: StatusPublished},
		},
		eligible: true,
	}
	svc := NewService(ServiceOptions{
		Repo:                   repo,
		TokenIssuer:            fakeTokenIssuer{token: "exam-token"},
		Now:                    fixedNow,
		ExamTokenBufferMinutes: 30,
	})

	_, err := svc.StartExam(context.Background(), StartInput{TenantID: 10, ExamID: 1, UserID: 20})
	if err != nil {
		t.Fatalf("StartExam returned error: %v", err)
	}

	wantExpiresAt := fixedUnixMilli + 30*minuteMillis + 30*minuteMillis
	if repo.createdAttempt.ExamTokenExpiresAt != wantExpiresAt {
		t.Fatalf("ExamTokenExpiresAt = %d, want %d", repo.createdAttempt.ExamTokenExpiresAt, wantExpiresAt)
	}
}

func TestStartExamValidatesQualificationAndTime(t *testing.T) {
	repo := &fakeRepository{
		exams: map[uint64]Exam{
			1: {ID: 1, TenantID: 10, StartTime: fixedUnixMilli + minuteMillis, EndTime: fixedUnixMilli + 60*minuteMillis, DurationMinutes: 30, MaxAttempts: 1, Status: StatusPublished},
			2: {ID: 2, TenantID: 10, StartTime: fixedUnixMilli - 60*minuteMillis, EndTime: fixedUnixMilli - minuteMillis, DurationMinutes: 30, MaxAttempts: 1, Status: StatusPublished},
		},
		eligible: false,
	}
	svc := NewService(ServiceOptions{Repo: repo, TokenIssuer: fakeTokenIssuer{token: "exam-token"}, Now: fixedNow})

	_, err := svc.StartExam(context.Background(), StartInput{TenantID: 10, ExamID: 1, UserID: 20})
	if !errors.Is(err, ErrExamNotEligible) {
		t.Fatalf("expected ErrExamNotEligible, got %v", err)
	}

	repo.eligible = true
	_, err = svc.StartExam(context.Background(), StartInput{TenantID: 10, ExamID: 1, UserID: 20})
	if !errors.Is(err, ErrExamNotStarted) {
		t.Fatalf("expected ErrExamNotStarted, got %v", err)
	}

	_, err = svc.StartExam(context.Background(), StartInput{TenantID: 10, ExamID: 2, UserID: 20})
	if !errors.Is(err, ErrExamEnded) {
		t.Fatalf("expected ErrExamEnded, got %v", err)
	}
}

func TestGenerateAttemptSnapshotsUseSourceAndKeepImmutablePayload(t *testing.T) {
	repo := &fakeRepository{
		exams: map[uint64]Exam{
			1: {ID: 1, TenantID: 10, PaperID: 100, BuildMode: BuildModeManual},
			2: {ID: 2, TenantID: 10, PaperID: 101, BuildMode: BuildModeRuleFixed},
			3: {ID: 3, TenantID: 10, PaperID: 102, BuildMode: BuildModeRuleLive},
		},
		fixedQuestions: []SnapshotSourceQuestion{
			{SectionID: 10, SectionName: "一、单选题", QuestionID: 300, QuestionType: QuestionTypeSingle, Title: "1+1=?", Score: "2", OptionIDs: []uint64{2, 1}, CorrectOptionIDs: []uint64{1}},
			{SectionID: 11, SectionName: "二、填空题", QuestionID: 301, QuestionType: QuestionTypeFillBlank, Title: "Go mod file", Score: "3", CorrectText: "go.mod"},
		},
		liveQuestions: []SnapshotSourceQuestion{
			{SectionID: 12, SectionName: "实时题", QuestionID: 400, QuestionType: QuestionTypeSingle, Title: "live", Score: "5", OptionIDs: []uint64{9, 8}, CorrectOptionIDs: []uint64{8}},
		},
	}
	svc := NewService(ServiceOptions{Repo: repo, Now: fixedNow})

	for _, examID := range []uint64{1, 2} {
		snapshots, err := svc.GenerateAttemptSnapshots(context.Background(), GenerateSnapshotInput{TenantID: 10, ExamID: examID, AttemptID: 900})
		if err != nil {
			t.Fatalf("GenerateAttemptSnapshots fixed exam %d returned error: %v", examID, err)
		}
		if !repo.usedFixedQuestions {
			t.Fatalf("expected manual/rule_fixed to use fixed paper section questions")
		}
		if len(snapshots) != 2 || snapshots[0].SortOrder != 1 || snapshots[1].SortOrder != 2 {
			t.Fatalf("expected global continuous sort order, got %#v", snapshots)
		}
		if snapshots[0].SectionSnapshot == "" || snapshots[0].QuestionSnapshot == "" || snapshots[0].OptionSnapshot == "" || snapshots[0].CorrectAnswerSnapshot == "" {
			t.Fatalf("expected all snapshot payloads generated, got %#v", snapshots[0])
		}
		if snapshots[0].OptionSnapshot == "[A,B]" {
			t.Fatalf("expected option snapshot to store option IDs, not frontend labels")
		}
	}

	repo.usedFixedQuestions = false
	snapshots, err := svc.GenerateAttemptSnapshots(context.Background(), GenerateSnapshotInput{TenantID: 10, ExamID: 3, AttemptID: 901})
	if err != nil {
		t.Fatalf("GenerateAttemptSnapshots rule_live returned error: %v", err)
	}
	if !repo.usedFrozenPool || repo.usedFixedQuestions {
		t.Fatalf("expected rule_live to use frozen pool only")
	}
	if len(snapshots) != 1 || snapshots[0].QuestionID != 400 {
		t.Fatalf("expected live snapshot from frozen pool, got %#v", snapshots)
	}
	var questionSnapshot struct {
		Title string `json:"title"`
		Type  string `json:"type"`
	}
	if err := json.Unmarshal([]byte(snapshots[0].QuestionSnapshot), &questionSnapshot); err != nil {
		t.Fatalf("question snapshot should be valid JSON: %v", err)
	}
	if questionSnapshot.Type != QuestionTypeSingle {
		t.Fatalf("expected question type frozen into snapshot, got %#v", questionSnapshot)
	}
	before := snapshots[0].QuestionSnapshot
	repo.liveQuestions[0].Title = "changed"
	if snapshots[0].QuestionSnapshot != before {
		t.Fatalf("expected generated snapshot to be immutable after question bank change")
	}
}

func TestExamTokenScopeOnlyCurrentAttempt(t *testing.T) {
	repo := &fakeRepository{
		attemptsByTokenHash: map[string]Attempt{},
	}
	svc := NewService(ServiceOptions{Repo: repo, TokenIssuer: fakeTokenIssuer{token: "exam-token"}, Now: fixedNow})
	hash := svc.HashExamToken("exam-token")
	repo.attemptsByTokenHash[hash] = Attempt{ID: 99, TenantID: 10, ExamID: 1, UserID: 20, Status: AttemptStatusInProgress, ExamTokenHash: hash, ExamTokenExpiresAt: fixedUnixMilli + minuteMillis}

	attempt, err := svc.ValidateExamToken(context.Background(), "exam-token", 99)
	if err != nil {
		t.Fatalf("ValidateExamToken returned error: %v", err)
	}
	if attempt.ID != 99 {
		t.Fatalf("expected attempt 99, got %d", attempt.ID)
	}

	_, err = svc.ValidateExamToken(context.Background(), "exam-token", 100)
	if !errors.Is(err, ErrExamTokenAttemptMismatch) {
		t.Fatalf("expected ErrExamTokenAttemptMismatch, got %v", err)
	}
}

const (
	fixedUnixMilli           int64 = 1767225600000
	minuteMillis             int64 = 60 * 1000
	defaultTokenBufferMillis int64 = 5 * minuteMillis
)

func fixedNow() int64 {
	return fixedUnixMilli
}

type fakeRepository struct {
	papers         map[uint64]Paper
	exams          map[uint64]Exam
	inviteCodes    map[string]bool
	liveCandidates []LivePoolItem
	frozeLivePool  bool
	frozenPool     []LivePoolItem
	updatedExam    Exam

	targets       map[targetKey]bool
	addedTarget   Target
	examsByInvite map[string]Exam

	eligible                bool
	existingInProgress      *Attempt
	attemptCount            int
	createdAttempt          Attempt
	updateAttemptTokenCount int
	conflictOnCreate        bool
	conflictCreated         bool
	retriedAfterConflict    int
	attemptsByTokenHash     map[string]Attempt

	fixedQuestions     []SnapshotSourceQuestion
	liveQuestions      []SnapshotSourceQuestion
	usedFixedQuestions bool
	usedFrozenPool     bool
}

func (r *fakeRepository) ListExams(ctx context.Context, input ListInput) (pagination.Result[Exam], error) {
	exams := make([]Exam, 0, len(r.exams))
	for _, exam := range r.exams {
		if exam.TenantID == input.TenantID {
			exams = append(exams, exam)
		}
	}
	page := pagination.Normalize(pagination.Input{Page: input.Page, PageSize: input.PageSize})
	return pagination.Result[Exam]{
		Items:    exams,
		Page:     page.Page,
		PageSize: page.PageSize,
		Total:    int64(len(exams)),
	}, nil
}

func (r *fakeRepository) CreateExam(ctx context.Context, exam Exam) (Exam, error) {
	exam.ID = 1
	return exam, nil
}

func (r *fakeRepository) GetPaper(ctx context.Context, tenantID uint64, paperID uint64) (Paper, error) {
	return r.papers[paperID], nil
}

func (r *fakeRepository) InviteCodeExists(ctx context.Context, tenantID uint64, code string) (bool, error) {
	return r.inviteCodes[code], nil
}

func (r *fakeRepository) ListRuleLiveCandidates(ctx context.Context, tenantID uint64, paperID uint64) ([]LivePoolItem, error) {
	return r.liveCandidates, nil
}

func (r *fakeRepository) PublishExamAndFreezeLivePool(ctx context.Context, exam Exam, pool []LivePoolItem) (Exam, error) {
	r.updatedExam = exam
	r.frozeLivePool = len(pool) > 0
	r.frozenPool = append([]LivePoolItem(nil), pool...)
	return exam, nil
}

func (r *fakeRepository) CreatePublishedExamWithTarget(ctx context.Context, exam Exam, pool []LivePoolItem, target Target) (Exam, error) {
	if exam.ID == 0 {
		exam.ID = 1
	}
	target.ExamID = exam.ID
	r.updatedExam = exam
	r.frozeLivePool = len(pool) > 0
	r.frozenPool = append([]LivePoolItem(nil), pool...)
	r.addedTarget = target
	return exam, nil
}

func (r *fakeRepository) TargetExists(ctx context.Context, tenantID uint64, examID uint64, targetType string, targetID uint64) (bool, error) {
	return r.targets[targetKey{tenantID: tenantID, examID: examID, targetType: targetType, targetID: targetID}], nil
}

func (r *fakeRepository) AddTarget(ctx context.Context, target Target) error {
	r.addedTarget = target
	return nil
}

func (r *fakeRepository) FindExamByInviteCode(ctx context.Context, inviteCode string) (Exam, error) {
	return r.examsByInvite[inviteCode], nil
}

func (r *fakeRepository) GetExam(ctx context.Context, tenantID uint64, examID uint64) (Exam, error) {
	return r.exams[examID], nil
}

func (r *fakeRepository) IsEligible(ctx context.Context, tenantID uint64, examID uint64, userID uint64) (bool, error) {
	return r.eligible, nil
}

func (r *fakeRepository) FindInProgressAttempt(ctx context.Context, tenantID uint64, examID uint64, userID uint64) (Attempt, error) {
	if r.conflictCreated {
		r.retriedAfterConflict++
	}
	if r.existingInProgress == nil {
		return Attempt{}, ErrAttemptNotFound
	}
	return *r.existingInProgress, nil
}

func (r *fakeRepository) CountAttempts(ctx context.Context, tenantID uint64, examID uint64, userID uint64) (int, error) {
	return r.attemptCount, nil
}

func (r *fakeRepository) CreateAttempt(ctx context.Context, attempt Attempt) (Attempt, error) {
	if r.conflictOnCreate {
		r.conflictCreated = true
		r.existingInProgress = &Attempt{ID: 100, TenantID: attempt.TenantID, ExamID: attempt.ExamID, UserID: attempt.UserID, AttemptNo: attempt.AttemptNo, Status: AttemptStatusInProgress}
		return Attempt{}, ErrAttemptUniqueConflict
	}
	attempt.ID = 101
	r.createdAttempt = attempt
	return attempt, nil
}

func (r *fakeRepository) UpdateAttemptToken(ctx context.Context, attempt Attempt) (Attempt, error) {
	r.updateAttemptTokenCount++
	if r.existingInProgress != nil && r.existingInProgress.ID == attempt.ID {
		*r.existingInProgress = attempt
	}
	return attempt, nil
}

func (r *fakeRepository) FindAttemptByTokenHash(ctx context.Context, tokenHash string) (Attempt, error) {
	attempt, ok := r.attemptsByTokenHash[tokenHash]
	if !ok {
		return Attempt{}, ErrAttemptNotFound
	}
	return attempt, nil
}

func (r *fakeRepository) ListFixedSnapshotQuestions(ctx context.Context, tenantID uint64, examID uint64) ([]SnapshotSourceQuestion, error) {
	r.usedFixedQuestions = true
	return append([]SnapshotSourceQuestion(nil), r.fixedQuestions...), nil
}

func (r *fakeRepository) ListFrozenLiveSnapshotQuestions(ctx context.Context, tenantID uint64, examID uint64) ([]SnapshotSourceQuestion, error) {
	r.usedFrozenPool = true
	return append([]SnapshotSourceQuestion(nil), r.liveQuestions...), nil
}

func (r *fakeRepository) SaveAttemptQuestions(ctx context.Context, questions []AttemptQuestion) ([]AttemptQuestion, error) {
	return append([]AttemptQuestion(nil), questions...), nil
}

func (r *fakeRepository) UpdateScorePublishConfig(ctx context.Context, tenantID uint64, examID uint64, publishMode string, scorePublishTime *int64) (Exam, error) {
	exam := r.exams[examID]
	exam.PublishMode = publishMode
	exam.ScorePublishTime = scorePublishTime
	r.exams[examID] = exam
	return exam, nil
}

type fakeCodeGenerator struct {
	codes []string
	next  int
}

func (g *fakeCodeGenerator) NextCode() (string, error) {
	code := g.codes[g.next]
	g.next++
	return code, nil
}

type fakeTokenIssuer struct {
	token string
}

func (i fakeTokenIssuer) IssueToken() (string, error) {
	return i.token, nil
}

type targetKey struct {
	tenantID   uint64
	examID     uint64
	targetType string
	targetID   uint64
}

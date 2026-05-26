package exam

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"time"
)

const (
	// StatusDraft 表示考试草稿状态。
	StatusDraft = "draft"
	// StatusPublished 表示考试已发布。
	StatusPublished = "published"

	// BuildModeManual 表示手动固化组卷。
	BuildModeManual = "manual"
	// BuildModeRuleFixed 表示规则生成后固化组卷。
	BuildModeRuleFixed = "rule_fixed"
	// BuildModeRuleLive 表示实时抽题组卷。
	BuildModeRuleLive = "rule_live"

	// ResultStrategyLatest 表示多次作答取最近一次成绩。
	ResultStrategyLatest = "latest"
	// ResultStrategyHighest 表示多次作答取最高成绩。
	ResultStrategyHighest = "highest"

	// PublishModeImmediateScore 表示提交后立即出分。
	PublishModeImmediateScore = "immediate_score"
	// PublishModeManualPublish 表示教师阅卷后发布成绩。
	PublishModeManualPublish = "manual_publish"

	// TargetTypeSpace 表示发布给空间。
	TargetTypeSpace = "space"
	// TargetTypeUser 表示发布给指定用户。
	TargetTypeUser = "user"

	// AttemptStatusInProgress 表示作答进行中。
	AttemptStatusInProgress = "in_progress"
	// AttemptStatusSubmitted 表示作答已提交。
	AttemptStatusSubmitted = "submitted"
)

var (
	ErrDurationExceedsExamWindow     = errors.New("duration exceeds exam window")
	ErrShortTextCannotRepeatAttempt  = errors.New("short text exam cannot repeat attempt")
	ErrShortTextCannotImmediateScore = errors.New("short text exam cannot immediate score")
	ErrDuplicateExamTarget           = errors.New("duplicate exam target")
	ErrLoginRequiredForInvite        = errors.New("login or register required for invite")
	ErrExamNotEligible               = errors.New("exam not eligible")
	ErrExamNotStarted                = errors.New("exam not started")
	ErrExamEnded                     = errors.New("exam ended")
	ErrAttemptNotFound               = errors.New("attempt not found")
	ErrMaxAttemptsReached            = errors.New("max attempts reached")
	ErrAttemptUniqueConflict         = errors.New("attempt unique conflict")
	ErrExamTokenAttemptMismatch      = errors.New("exam token attempt mismatch")
	ErrExamTokenInvalid              = errors.New("exam token invalid")
	ErrInviteCodeCollision           = errors.New("invite code collision")
	ErrAnswerDeadlineExceeded        = errors.New("answer deadline exceeded")
	ErrAttemptAlreadySubmitted       = errors.New("attempt already submitted")
)

const (
	millisPerMinute   int64 = 60 * 1000
	tokenBufferMillis       = 5 * millisPerMinute
)

type Exam struct {
	ID               uint64 // 考试主键 ID。
	TenantID         uint64 // 所属租户 ID。
	PaperID          uint64 // 关联试卷 ID。
	Name             string // 考试名称。
	StartTime        int64  // 考试开始时间，Unix 毫秒时间戳。
	EndTime          int64  // 考试结束时间，Unix 毫秒时间戳。
	DurationMinutes  int    // 单次作答时长，单位分钟。
	MaxAttempts      int    // 每名考生最多作答次数。
	ResultStrategy   string // 多次作答成绩策略。
	PublishMode      string // 成绩发布模式。
	ScorePublishTime *int64 // 统一成绩公布时间。
	InviteCode       string // 考试邀请码。
	Status           string // 考试状态。
	BuildMode        string // 冗余的试卷组卷模式，便于快照生成。
}

type Paper struct {
	ID                uint64 // 试卷主键 ID。
	BuildMode         string // 组卷方式。
	ContainsShortText bool   // 是否包含简答题。
}

type Target struct {
	TenantID   uint64 // 所属租户 ID。
	ExamID     uint64 // 考试 ID。
	TargetType string // 发布目标类型：space / user。
	TargetID   uint64 // 发布目标 ID。
}

type LivePoolItem struct {
	SectionID  uint64 // 大题 ID。
	RuleID     uint64 // 规则 ID。
	QuestionID uint64 // 题目 ID。
}

type Attempt struct {
	ID                 uint64 // 作答主键 ID。
	TenantID           uint64 // 所属租户 ID。
	ExamID             uint64 // 考试 ID。
	UserID             uint64 // 考生用户 ID。
	AttemptNo          int    // 第几次作答。
	Status             string // 作答状态。
	StartedAt          int64  // 开始作答时间。
	SubmittedAt        *int64 // 提交时间，nil 表示尚未提交。
	ExamTokenHash      string // exam token 哈希。
	ExamTokenExpiresAt int64  // exam token 过期时间。
	Version            int64  // 乐观锁版本。
}

type AttemptQuestion struct {
	TenantID              uint64 // 所属租户 ID。
	AttemptID             uint64 // 作答 ID。
	SectionID             uint64 // 原始大题 ID。
	QuestionID            uint64 // 原始题目 ID。
	SectionSnapshot       string // 大题快照 JSON。
	SortOrder             int    // 全局题号。
	Score                 string // 题目分值。
	QuestionSnapshot      string // 题干快照 JSON。
	OptionSnapshot        string // 选项快照 JSON。
	CorrectAnswerSnapshot string // 正确答案快照 JSON。
}

type SnapshotSourceQuestion struct {
	SectionID        uint64   // 大题 ID。
	SectionName      string   // 大题名称。
	Instructions     string   // 作答说明。
	QuestionID       uint64   // 题目 ID。
	QuestionType     string   // 题型，判分时必须使用快照值避免题库变更影响考试。
	Title            string   // 题干。
	Score            string   // 分值。
	OptionIDs        []uint64 // 最终展示选项 ID 顺序。
	CorrectOptionIDs []uint64 // 正确选项 ID。
	CorrectText      string   // 填空或简答正确答案。
}

type CreateDraftInput struct {
	TenantID uint64 // 所属租户 ID。
	PaperID  uint64 // 关联试卷 ID。
	Name     string // 考试名称。
}

type PublishInput struct {
	TenantID         uint64 // 所属租户 ID。
	ExamID           uint64 // 考试 ID。
	PaperID          uint64 // 关联试卷 ID。
	StartTime        int64  // 开始时间。
	EndTime          int64  // 结束时间。
	DurationMinutes  int    // 单次作答时长。
	MaxAttempts      int    // 最大作答次数。
	ResultStrategy   string // 成绩策略。
	PublishMode      string // 成绩发布模式。
	ScorePublishTime *int64 // 成绩公布时间。
}

type AddTargetInput struct {
	TenantID   uint64 // 所属租户 ID。
	ExamID     uint64 // 考试 ID。
	TargetType string // 发布目标类型。
	TargetID   uint64 // 发布目标 ID。
}

type ResolveInviteInput struct {
	InviteCode string // 考试邀请码。
	UserID     uint64 // 当前登录用户 ID，0 表示未登录。
}

type StartInput struct {
	TenantID uint64 // 所属租户 ID。
	ExamID   uint64 // 考试 ID。
	UserID   uint64 // 考生用户 ID。
}

type StartResult struct {
	Attempt   Attempt // 作答记录。
	ExamToken string  // 明文 exam token，只返回给考生端。
}

type GenerateSnapshotInput struct {
	TenantID  uint64 // 所属租户 ID。
	ExamID    uint64 // 考试 ID。
	AttemptID uint64 // 作答 ID。
}

type Repository interface {
	CreateExam(ctx context.Context, exam Exam) (Exam, error)
	GetPaper(ctx context.Context, tenantID uint64, paperID uint64) (Paper, error)
	InviteCodeExists(ctx context.Context, tenantID uint64, code string) (bool, error)
	ListRuleLiveCandidates(ctx context.Context, tenantID uint64, paperID uint64) ([]LivePoolItem, error)
	PublishExamAndFreezeLivePool(ctx context.Context, exam Exam, pool []LivePoolItem) (Exam, error)
	TargetExists(ctx context.Context, tenantID uint64, examID uint64, targetType string, targetID uint64) (bool, error)
	AddTarget(ctx context.Context, target Target) error
	FindExamByInviteCode(ctx context.Context, inviteCode string) (Exam, error)
	GetExam(ctx context.Context, tenantID uint64, examID uint64) (Exam, error)
	IsEligible(ctx context.Context, tenantID uint64, examID uint64, userID uint64) (bool, error)
	FindInProgressAttempt(ctx context.Context, tenantID uint64, examID uint64, userID uint64) (Attempt, error)
	CountAttempts(ctx context.Context, tenantID uint64, examID uint64, userID uint64) (int, error)
	CreateAttempt(ctx context.Context, attempt Attempt) (Attempt, error)
	FindAttemptByTokenHash(ctx context.Context, tokenHash string) (Attempt, error)
	ListFixedSnapshotQuestions(ctx context.Context, tenantID uint64, examID uint64) ([]SnapshotSourceQuestion, error)
	ListFrozenLiveSnapshotQuestions(ctx context.Context, tenantID uint64, examID uint64) ([]SnapshotSourceQuestion, error)
}

type CodeGenerator interface {
	NextCode() (string, error)
}

type TokenIssuer interface {
	IssueToken() (string, error)
}

type ServiceOptions struct {
	Repo          Repository
	CodeGenerator CodeGenerator
	TokenIssuer   TokenIssuer
	Now           func() int64
}

type Service struct {
	repo          Repository
	codeGenerator CodeGenerator
	tokenIssuer   TokenIssuer
	now           func() int64
}

func NewService(options ServiceOptions) *Service {
	now := options.Now
	if now == nil {
		now = func() int64 { return time.Now().UnixMilli() }
	}
	tokenIssuer := options.TokenIssuer
	if tokenIssuer == nil {
		tokenIssuer = randomTokenIssuer{}
	}
	return &Service{
		repo:          options.Repo,
		codeGenerator: options.CodeGenerator,
		tokenIssuer:   tokenIssuer,
		now:           now,
	}
}

func (s *Service) CreateDraft(ctx context.Context, input CreateDraftInput) (Exam, error) {
	return s.repo.CreateExam(ctx, Exam{
		TenantID:       input.TenantID,
		PaperID:        input.PaperID,
		Name:           input.Name,
		MaxAttempts:    1,
		ResultStrategy: ResultStrategyLatest,
		Status:         StatusDraft,
	})
}

func (s *Service) Publish(ctx context.Context, input PublishInput) (Exam, error) {
	if int64(input.DurationMinutes)*millisPerMinute > input.EndTime-input.StartTime {
		return Exam{}, ErrDurationExceedsExamWindow
	}
	paper, err := s.repo.GetPaper(ctx, input.TenantID, input.PaperID)
	if err != nil {
		return Exam{}, err
	}
	if paper.ContainsShortText && input.MaxAttempts > 1 {
		return Exam{}, ErrShortTextCannotRepeatAttempt
	}
	if paper.ContainsShortText && input.PublishMode == PublishModeImmediateScore {
		return Exam{}, ErrShortTextCannotImmediateScore
	}
	inviteCode, err := s.nextUniqueInviteCode(ctx, input.TenantID)
	if err != nil {
		return Exam{}, err
	}
	exam := Exam{
		ID:               input.ExamID,
		TenantID:         input.TenantID,
		PaperID:          input.PaperID,
		StartTime:        input.StartTime,
		EndTime:          input.EndTime,
		DurationMinutes:  input.DurationMinutes,
		MaxAttempts:      input.MaxAttempts,
		ResultStrategy:   input.ResultStrategy,
		PublishMode:      input.PublishMode,
		ScorePublishTime: input.ScorePublishTime,
		InviteCode:       inviteCode,
		Status:           StatusPublished,
		BuildMode:        paper.BuildMode,
	}
	var pool []LivePoolItem
	if paper.BuildMode == BuildModeRuleLive {
		pool, err = s.repo.ListRuleLiveCandidates(ctx, input.TenantID, input.PaperID)
		if err != nil {
			return Exam{}, err
		}
	}
	return s.repo.PublishExamAndFreezeLivePool(ctx, exam, pool)
}

func (s *Service) AddTarget(ctx context.Context, input AddTargetInput) error {
	exists, err := s.repo.TargetExists(ctx, input.TenantID, input.ExamID, input.TargetType, input.TargetID)
	if err != nil {
		return err
	}
	if exists {
		return ErrDuplicateExamTarget
	}
	return s.repo.AddTarget(ctx, Target{
		TenantID:   input.TenantID,
		ExamID:     input.ExamID,
		TargetType: input.TargetType,
		TargetID:   input.TargetID,
	})
}

func (s *Service) ResolveInvite(ctx context.Context, input ResolveInviteInput) (Exam, error) {
	if input.UserID == 0 {
		return Exam{}, ErrLoginRequiredForInvite
	}
	return s.repo.FindExamByInviteCode(ctx, input.InviteCode)
}

func (s *Service) StartExam(ctx context.Context, input StartInput) (StartResult, error) {
	exam, err := s.repo.GetExam(ctx, input.TenantID, input.ExamID)
	if err != nil {
		return StartResult{}, err
	}
	eligible, err := s.repo.IsEligible(ctx, input.TenantID, input.ExamID, input.UserID)
	if err != nil {
		return StartResult{}, err
	}
	if !eligible {
		return StartResult{}, ErrExamNotEligible
	}
	now := s.now()
	if now < exam.StartTime {
		return StartResult{}, ErrExamNotStarted
	}
	if now > exam.EndTime {
		return StartResult{}, ErrExamEnded
	}
	if attempt, err := s.repo.FindInProgressAttempt(ctx, input.TenantID, input.ExamID, input.UserID); err == nil {
		return StartResult{Attempt: attempt}, nil
	} else if !errors.Is(err, ErrAttemptNotFound) {
		return StartResult{}, err
	}
	count, err := s.repo.CountAttempts(ctx, input.TenantID, input.ExamID, input.UserID)
	if err != nil {
		return StartResult{}, err
	}
	if count >= exam.MaxAttempts {
		return StartResult{}, ErrMaxAttemptsReached
	}
	token, err := s.tokenIssuer.IssueToken()
	if err != nil {
		return StartResult{}, err
	}
	attempt := Attempt{
		TenantID:           input.TenantID,
		ExamID:             input.ExamID,
		UserID:             input.UserID,
		AttemptNo:          count + 1,
		Status:             AttemptStatusInProgress,
		StartedAt:          now,
		ExamTokenHash:      s.HashExamToken(token),
		ExamTokenExpiresAt: answerDeadline(now, exam) + tokenBufferMillis,
	}
	attempt, err = s.repo.CreateAttempt(ctx, attempt)
	if errors.Is(err, ErrAttemptUniqueConflict) {
		existing, findErr := s.repo.FindInProgressAttempt(ctx, input.TenantID, input.ExamID, input.UserID)
		if findErr != nil {
			return StartResult{}, findErr
		}
		return StartResult{Attempt: existing}, nil
	}
	if err != nil {
		return StartResult{}, err
	}
	return StartResult{Attempt: attempt, ExamToken: token}, nil
}

func (s *Service) GenerateAttemptSnapshots(ctx context.Context, input GenerateSnapshotInput) ([]AttemptQuestion, error) {
	exam, err := s.repo.GetExam(ctx, input.TenantID, input.ExamID)
	if err != nil {
		return nil, err
	}
	var sources []SnapshotSourceQuestion
	if exam.BuildMode == BuildModeRuleLive {
		sources, err = s.repo.ListFrozenLiveSnapshotQuestions(ctx, input.TenantID, input.ExamID)
	} else {
		sources, err = s.repo.ListFixedSnapshotQuestions(ctx, input.TenantID, input.ExamID)
	}
	if err != nil {
		return nil, err
	}
	snapshots := make([]AttemptQuestion, 0, len(sources))
	for index, source := range sources {
		snapshots = append(snapshots, AttemptQuestion{
			TenantID:              input.TenantID,
			AttemptID:             input.AttemptID,
			SectionID:             source.SectionID,
			QuestionID:            source.QuestionID,
			SectionSnapshot:       mustJSON(map[string]any{"name": source.SectionName, "instructions": source.Instructions}),
			SortOrder:             index + 1,
			Score:                 source.Score,
			QuestionSnapshot:      mustJSON(map[string]any{"title": source.Title, "type": source.QuestionType}),
			OptionSnapshot:        mustJSON(map[string]any{"option_ids": source.OptionIDs}),
			CorrectAnswerSnapshot: mustJSON(map[string]any{"option_ids": source.CorrectOptionIDs, "text": source.CorrectText}),
		})
	}
	return snapshots, nil
}

func (s *Service) ValidateExamToken(ctx context.Context, token string, attemptID uint64) (Attempt, error) {
	attempt, err := s.repo.FindAttemptByTokenHash(ctx, HashExamToken(token))
	if err != nil {
		return Attempt{}, ErrExamTokenInvalid
	}
	if attempt.ID != attemptID {
		return Attempt{}, ErrExamTokenAttemptMismatch
	}
	if attempt.Status != AttemptStatusInProgress || attempt.ExamTokenExpiresAt < s.now() {
		return Attempt{}, ErrExamTokenInvalid
	}
	return attempt, nil
}

func (s *Service) HashExamToken(token string) string {
	return HashExamToken(token)
}

func HashExamToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *Service) nextUniqueInviteCode(ctx context.Context, tenantID uint64) (string, error) {
	generator := s.codeGenerator
	if generator == nil {
		generator = randomCodeGenerator{}
	}
	for range 8 {
		code, err := generator.NextCode()
		if err != nil {
			return "", err
		}
		exists, err := s.repo.InviteCodeExists(ctx, tenantID, code)
		if err != nil {
			return "", err
		}
		if !exists {
			return code, nil
		}
	}
	return "", ErrInviteCodeCollision
}

func answerDeadline(startedAt int64, exam Exam) int64 {
	durationDeadline := startedAt + int64(exam.DurationMinutes)*millisPerMinute
	if durationDeadline < exam.EndTime {
		return durationDeadline
	}
	return exam.EndTime
}

func mustJSON(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}

type randomTokenIssuer struct{}

func (randomTokenIssuer) IssueToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

type randomCodeGenerator struct{}

func (randomCodeGenerator) NextCode() (string, error) {
	buffer := make([]byte, 6)
	if _, err := io.ReadFull(rand.Reader, buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

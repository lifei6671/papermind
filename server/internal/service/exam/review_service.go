package exam

import (
	"context"
	"errors"
	"time"

	"github.com/lifei6671/papermind/server/internal/service/permission"
)

var (
	ErrAnswerVersionConflict = errors.New("answer version conflict")
)

type PendingAttempt struct {
	AttemptID             uint64 // 作答 ID。
	AttemptQuestionID     uint64 // 待阅卷题目快照 ID。
	ExamID                uint64 // 考试 ID。
	UserID                uint64 // 考生用户 ID。
	StudentName           string // 考生姓名。
	SpaceID               uint64 // 考生所属空间 ID，用于权限范围判断。
	SpaceName             string // 考生所属空间名称。
	ExamName              string // 考试名称。
	QuestionTitle         string // 简答题题干。
	AnswerContent         string // 考生作答内容。
	MaxScore              string // 简答题满分。
	AnswerVersion         int64  // 答案版本号，用于乐观锁。
	PendingShortTextCount int    // 待阅卷简答题数量。
	SubmittedAt           int64  // 提交时间，Unix 毫秒时间戳。
}

type ShortTextGrade struct {
	TenantID          uint64 // 所属租户 ID。
	AttemptID         uint64 // 作答 ID。
	AttemptQuestionID uint64 // 考生题目快照 ID。
	AnswerVersion     int64  // 答案版本号，用于乐观锁。
	Score             string // 简答题得分。
	Comment           string // 阅卷评语。
	GradedBy          uint64 // 阅卷人用户 ID。
	GradedAt          int64  // 阅卷时间，Unix 毫秒时间戳。
}

type ListPendingAttemptsInput struct {
	Permission permission.PermissionContext // 当前阅卷人权限上下文。
	TenantID   uint64                       // 所属租户 ID。
	ExamID     uint64                       // 考试 ID。
}

type GradeShortTextInput struct {
	Permission        permission.PermissionContext // 当前阅卷人权限上下文。
	TenantID          uint64                       // 所属租户 ID。
	AttemptID         uint64                       // 作答 ID。
	AttemptQuestionID uint64                       // 考生题目快照 ID。
	AnswerVersion     int64                        // 答案版本号。
	Score             string                       // 简答题得分。
	Comment           string                       // 阅卷评语。
}

type ReviewRepository interface {
	ListPendingAttempts(ctx context.Context, tenantID uint64, examID uint64) ([]PendingAttempt, error)
	GradeShortTextAndRecalculate(ctx context.Context, grade ShortTextGrade) error
}

type ReviewServiceOptions struct {
	Repo              ReviewRepository
	PermissionChecker permission.PermissionChecker
	Now               func() int64
}

type ReviewService struct {
	repo              ReviewRepository
	permissionChecker permission.PermissionChecker
	now               func() int64
}

func NewReviewService(options ReviewServiceOptions) *ReviewService {
	now := options.Now
	if now == nil {
		now = func() int64 { return time.Now().UnixMilli() }
	}
	return &ReviewService{
		repo:              options.Repo,
		permissionChecker: options.PermissionChecker,
		now:               now,
	}
}

func (s *ReviewService) ListPendingAttempts(ctx context.Context, input ListPendingAttemptsInput) ([]PendingAttempt, error) {
	items, err := s.repo.ListPendingAttempts(ctx, input.TenantID, input.ExamID)
	if err != nil {
		return nil, err
	}
	if err := s.permissionChecker.CanGradeExam(input.Permission, input.ExamID); err == nil {
		return items, nil
	}
	allowed := make([]PendingAttempt, 0, len(items))
	for _, item := range items {
		if err := s.permissionChecker.CanGradeAttempt(input.Permission, item.AttemptID); err == nil {
			allowed = append(allowed, item)
		}
	}
	return allowed, nil
}

func (s *ReviewService) GradeShortText(ctx context.Context, input GradeShortTextInput) error {
	if err := s.permissionChecker.CanGradeAttempt(input.Permission, input.AttemptID); err != nil {
		return err
	}
	// 简答题阅卷和主观题/总分重算必须由仓储在同一事务内完成，避免成绩短暂不一致。
	return s.repo.GradeShortTextAndRecalculate(ctx, ShortTextGrade{
		TenantID:          input.TenantID,
		AttemptID:         input.AttemptID,
		AttemptQuestionID: input.AttemptQuestionID,
		AnswerVersion:     input.AnswerVersion,
		Score:             input.Score,
		Comment:           input.Comment,
		GradedBy:          input.Permission.UserID,
		GradedAt:          s.now(),
	})
}

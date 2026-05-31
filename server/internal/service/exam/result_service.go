package exam

import (
	"context"
	"errors"
	"time"

	"github.com/lifei6671/papermind/server/internal/service/permission"
)

var (
	ErrResultNotVisible = errors.New("result not visible")
	ErrResultNotFound   = errors.New("result not found")
)

type ResultSnapshot struct {
	AttemptID        uint64 // 作答 ID。
	ExamID           uint64 // 考试 ID。
	UserID           uint64 // 考生用户 ID。
	AttemptNo        int    // 作答次数。
	TotalScore       string // 总分。
	ObjectiveScore   string // 客观题分。
	SubjectiveScore  string // 主观题分。
	PublishMode      string // 成绩发布模式。
	ScorePublishTime *int64 // 统一公布时间。
	ShowAnalysis     bool   // 试卷是否允许展示解析。
}

type VisibleResult struct {
	ResultSnapshot
	AnalysisVisible bool // 成绩可见且试卷允许展示解析时为 true。
}

type ResultQueryInput struct {
	Permission permission.PermissionContext // 当前查看人权限上下文。
	TenantID   uint64                       // 所属租户 ID。
	AttemptID  uint64                       // 作答 ID。
}

type SelectResultInput struct {
	Permission     permission.PermissionContext // 当前查看人权限上下文。
	TenantID       uint64                       // 所属租户 ID。
	ExamID         uint64                       // 考试 ID。
	ResultStrategy string                       // latest / highest。
}

type ResultRepository interface {
	GetResultSnapshot(ctx context.Context, tenantID uint64, attemptID uint64) (ResultSnapshot, error)
	ListUserResultSnapshots(ctx context.Context, tenantID uint64, examID uint64, userID uint64) ([]ResultSnapshot, error)
}

type ResultServiceOptions struct {
	Repo              ResultRepository
	PermissionChecker permission.PermissionChecker
	Now               func() int64
}

type ResultService struct {
	repo              ResultRepository
	permissionChecker permission.PermissionChecker
	now               func() int64
}

func NewResultService(options ResultServiceOptions) *ResultService {
	now := options.Now
	if now == nil {
		now = func() int64 { return time.Now().UnixMilli() }
	}
	return &ResultService{repo: options.Repo, permissionChecker: options.PermissionChecker, now: now}
}

func (s *ResultService) GetVisibleResult(ctx context.Context, input ResultQueryInput) (VisibleResult, error) {
	snapshot, err := s.repo.GetResultSnapshot(ctx, input.TenantID, input.AttemptID)
	if err != nil {
		return VisibleResult{}, err
	}
	if !s.canViewResult(input.Permission, snapshot) {
		return VisibleResult{}, ErrResultNotVisible
	}
	return s.visibleResult(snapshot)
}

func (s *ResultService) SelectVisibleResult(ctx context.Context, input SelectResultInput) (VisibleResult, error) {
	items, err := s.repo.ListUserResultSnapshots(ctx, input.TenantID, input.ExamID, input.Permission.UserID)
	if err != nil {
		return VisibleResult{}, err
	}
	visibleItems := make([]VisibleResult, 0, len(items))
	for _, item := range items {
		if !s.canViewResult(input.Permission, item) {
			continue
		}
		visible, err := s.visibleResult(item)
		if errors.Is(err, ErrResultNotVisible) {
			continue
		}
		if err != nil {
			return VisibleResult{}, err
		}
		visibleItems = append(visibleItems, visible)
	}
	if len(visibleItems) == 0 {
		return VisibleResult{}, ErrResultNotFound
	}
	if input.ResultStrategy == ResultStrategyHighest {
		return highestResult(visibleItems), nil
	}
	return latestResult(visibleItems), nil
}

func (s *ResultService) canViewResult(ctx permission.PermissionContext, snapshot ResultSnapshot) bool {
	// 查分入口先以作答本人为边界；空间学生可能不是租户级 student 角色。
	if ctx.SubjectType != permission.SubjectTenantUser || ctx.UserID != snapshot.UserID {
		return false
	}
	return s.permissionChecker.CanViewOwnResult(ctx, snapshot.AttemptID) == nil
}

func (s *ResultService) visibleResult(snapshot ResultSnapshot) (VisibleResult, error) {
	if snapshot.PublishMode == PublishModeManualPublish {
		if snapshot.ScorePublishTime == nil || s.now() < *snapshot.ScorePublishTime {
			return VisibleResult{}, ErrResultNotVisible
		}
	}
	result := VisibleResult{ResultSnapshot: snapshot}
	result.AnalysisVisible = snapshot.ShowAnalysis
	return result, nil
}

func latestResult(items []VisibleResult) VisibleResult {
	latest := items[0]
	for _, item := range items[1:] {
		if item.AttemptNo > latest.AttemptNo {
			latest = item
		}
	}
	return latest
}

func highestResult(items []VisibleResult) VisibleResult {
	highest := items[0]
	for _, item := range items[1:] {
		itemScore, err := parseGradingScore(item.TotalScore)
		if err != nil {
			continue
		}
		highestScore, err := parseGradingScore(highest.TotalScore)
		if err != nil || itemScore > highestScore {
			highest = item
		}
	}
	return highest
}

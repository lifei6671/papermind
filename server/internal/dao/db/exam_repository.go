package db

import (
	"context"
	"errors"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"

	serviceexam "github.com/lifei6671/papermind/server/internal/service/exam"
	"github.com/lifei6671/papermind/server/internal/service/pagination"
)

type ExamRepository struct {
	db  *gorm.DB
	now func() int64
}

type ExamRepositoryOptions struct {
	Now func() int64
}

func NewExamRepository(gormDB *gorm.DB, options ExamRepositoryOptions) *ExamRepository {
	now := options.Now
	if now == nil {
		now = func() int64 { return time.Now().UnixMilli() }
	}
	return &ExamRepository{db: gormDB, now: now}
}

func (r *ExamRepository) ListExams(ctx context.Context, tenantID uint64, page pagination.Input) (pagination.Result[serviceexam.Exam], error) {
	page = pagination.Normalize(page)
	query := r.db.WithContext(ctx).Model(&ExamDO{}).
		Where(ExamColumns.TenantID+" = ?", tenantID).
		Where(ExamColumns.DeletedAt+" = ?", 0)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return pagination.Result[serviceexam.Exam]{}, err
	}
	var rows []ExamDO
	if err := query.
		Order(ExamColumns.ID + " ASC").
		Limit(page.PageSize).
		Offset(pagination.Offset(page)).
		Find(&rows).Error; err != nil {
		return pagination.Result[serviceexam.Exam]{}, err
	}
	exams := make([]serviceexam.Exam, 0, len(rows))
	for _, row := range rows {
		exams = append(exams, examFromDO(row, ""))
	}
	return pagination.Result[serviceexam.Exam]{
		Items:    exams,
		Page:     page.Page,
		PageSize: page.PageSize,
		Total:    total,
	}, nil
}

func (r *ExamRepository) CreateExam(ctx context.Context, exam serviceexam.Exam) (serviceexam.Exam, error) {
	now := r.now()
	row := ExamDO{
		BaseFields: BaseFields{
			CreatedAt: now,
			UpdatedAt: now,
			Version:   1,
			ExtJSON:   datatypes.JSON("{}"),
		},
		TenantID:       exam.TenantID,
		PaperID:        exam.PaperID,
		Name:           exam.Name,
		MaxAttempts:    exam.MaxAttempts,
		ResultStrategy: exam.ResultStrategy,
		Status:         exam.Status,
	}
	if row.MaxAttempts == 0 {
		row.MaxAttempts = 1
	}
	if row.ResultStrategy == "" {
		row.ResultStrategy = serviceexam.ResultStrategyLatest
	}
	if row.PublishMode == "" {
		row.PublishMode = serviceexam.PublishModeManualPublish
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return serviceexam.Exam{}, err
	}
	return examFromDO(row, ""), nil
}

func (r *ExamRepository) GetPaper(ctx context.Context, tenantID uint64, paperID uint64) (serviceexam.Paper, error) {
	var row PaperDO
	if err := r.db.WithContext(ctx).
		Where(PaperColumns.TenantID+" = ?", tenantID).
		Where(PaperColumns.ID+" = ?", paperID).
		Where(PaperColumns.DeletedAt+" = ?", 0).
		First(&row).Error; err != nil {
		return serviceexam.Paper{}, err
	}
	var shortTextCount int64
	if err := r.db.WithContext(ctx).Table(PaperSectionDO{}.TableName()).
		Where(PaperSectionColumns.TenantID+" = ?", tenantID).
		Where(PaperSectionColumns.DeletedAt+" = ?", 0).
		Where(PaperSectionColumns.QuestionType+" = ?", "short_text").
		Where(PaperSectionColumns.PaperID+" = ?", paperID).
		Count(&shortTextCount).Error; err != nil {
		return serviceexam.Paper{}, err
	}
	return serviceexam.Paper{
		ID:                row.ID,
		BuildMode:         row.BuildMode,
		ContainsShortText: shortTextCount > 0,
	}, nil
}

func (r *ExamRepository) InviteCodeExists(ctx context.Context, tenantID uint64, code string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&ExamDO{}).
		Where(ExamColumns.TenantID+" = ?", tenantID).
		Where(ExamColumns.InviteCode+" = ?", code).
		Where(ExamColumns.DeletedAt+" = ?", 0).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *ExamRepository) ListRuleLiveCandidates(ctx context.Context, tenantID uint64, paperID uint64) ([]serviceexam.LivePoolItem, error) {
	return []serviceexam.LivePoolItem{}, nil
}

func (r *ExamRepository) PublishExamAndFreezeLivePool(ctx context.Context, exam serviceexam.Exam, pool []serviceexam.LivePoolItem) (serviceexam.Exam, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{
			ExamColumns.StartTime:        exam.StartTime,
			ExamColumns.EndTime:          exam.EndTime,
			ExamColumns.DurationMinutes:  exam.DurationMinutes,
			ExamColumns.MaxAttempts:      exam.MaxAttempts,
			ExamColumns.ResultStrategy:   exam.ResultStrategy,
			ExamColumns.PublishMode:      exam.PublishMode,
			ExamColumns.ScorePublishTime: exam.ScorePublishTime,
			ExamColumns.InviteCode:       exam.InviteCode,
			ExamColumns.Status:           exam.Status,
			BaseColumns.UpdatedAt:        r.now(),
			BaseColumns.Version:          gorm.Expr(BaseColumns.Version + " + 1"),
		}
		result := tx.Model(&ExamDO{}).
			Where(ExamColumns.TenantID+" = ?", exam.TenantID).
			Where(ExamColumns.ID+" = ?", exam.ID).
			Where(ExamColumns.DeletedAt+" = ?", 0).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		for _, item := range pool {
			row := ExamLiveQuestionPoolDO{
				RelationFields: RelationFields{
					CreatedAt: r.now(),
					ExtJSON:   datatypes.JSON([]byte("{}")),
				},
				TenantID:   exam.TenantID,
				ExamID:     exam.ID,
				SectionID:  item.SectionID,
				RuleID:     item.RuleID,
				QuestionID: item.QuestionID,
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return serviceexam.Exam{}, err
	}
	return r.GetExam(ctx, exam.TenantID, exam.ID)
}

func (r *ExamRepository) TargetExists(ctx context.Context, tenantID uint64, examID uint64, targetType string, targetID uint64) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&ExamTargetDO{}).
		Where(ExamTargetColumns.TenantID+" = ?", tenantID).
		Where(ExamTargetColumns.ExamID+" = ?", examID).
		Where(ExamTargetColumns.TargetType+" = ?", targetType).
		Where(ExamTargetColumns.TargetID+" = ?", targetID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *ExamRepository) AddTarget(ctx context.Context, target serviceexam.Target) error {
	row := ExamTargetDO{
		RelationFields: RelationFields{
			CreatedAt: r.now(),
			ExtJSON:   datatypes.JSON([]byte("{}")),
		},
		TenantID:   target.TenantID,
		ExamID:     target.ExamID,
		TargetType: target.TargetType,
		TargetID:   target.TargetID,
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *ExamRepository) FindExamByInviteCode(ctx context.Context, inviteCode string) (serviceexam.Exam, error) {
	var row ExamDO
	if err := r.db.WithContext(ctx).
		Where(ExamColumns.InviteCode+" = ?", inviteCode).
		Where(ExamColumns.DeletedAt+" = ?", 0).
		First(&row).Error; err != nil {
		return serviceexam.Exam{}, err
	}
	return examFromDO(row, ""), nil
}

func (r *ExamRepository) GetExam(ctx context.Context, tenantID uint64, examID uint64) (serviceexam.Exam, error) {
	var row ExamDO
	if err := r.db.WithContext(ctx).
		Where(ExamColumns.TenantID+" = ?", tenantID).
		Where(ExamColumns.ID+" = ?", examID).
		Where(ExamColumns.DeletedAt+" = ?", 0).
		First(&row).Error; err != nil {
		return serviceexam.Exam{}, err
	}
	paper, err := r.GetPaper(ctx, tenantID, row.PaperID)
	if err != nil {
		return serviceexam.Exam{}, err
	}
	return examFromDO(row, paper.BuildMode), nil
}

func (r *ExamRepository) IsEligible(ctx context.Context, tenantID uint64, examID uint64, userID uint64) (bool, error) {
	return true, nil
}

func (r *ExamRepository) FindInProgressAttempt(ctx context.Context, tenantID uint64, examID uint64, userID uint64) (serviceexam.Attempt, error) {
	var row ExamAttemptDO
	err := r.db.WithContext(ctx).
		Where(ExamAttemptColumns.TenantID+" = ?", tenantID).
		Where(ExamAttemptColumns.ExamID+" = ?", examID).
		Where(ExamAttemptColumns.UserID+" = ?", userID).
		Where(ExamAttemptColumns.Status+" = ?", serviceexam.AttemptStatusInProgress).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return serviceexam.Attempt{}, serviceexam.ErrAttemptNotFound
	}
	if err != nil {
		return serviceexam.Attempt{}, err
	}
	return attemptFromDO(row), nil
}

func (r *ExamRepository) CountAttempts(ctx context.Context, tenantID uint64, examID uint64, userID uint64) (int, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&ExamAttemptDO{}).
		Where(ExamAttemptColumns.TenantID+" = ?", tenantID).
		Where(ExamAttemptColumns.ExamID+" = ?", examID).
		Where(ExamAttemptColumns.UserID+" = ?", userID).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return int(count), nil
}

func (r *ExamRepository) CreateAttempt(ctx context.Context, attempt serviceexam.Attempt) (serviceexam.Attempt, error) {
	now := r.now()
	row := ExamAttemptDO{
		BaseFields: BaseFields{
			CreatedAt: now,
			UpdatedAt: now,
			Version:   1,
			ExtJSON:   datatypes.JSON([]byte("{}")),
		},
		TenantID:           attempt.TenantID,
		ExamID:             attempt.ExamID,
		UserID:             attempt.UserID,
		AttemptNo:          attempt.AttemptNo,
		Status:             attempt.Status,
		StartedAt:          attempt.StartedAt,
		SubmittedAt:        attempt.SubmittedAt,
		ExamTokenHash:      attempt.ExamTokenHash,
		ExamTokenExpiresAt: attempt.ExamTokenExpiresAt,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return serviceexam.Attempt{}, err
	}
	return attemptFromDO(row), nil
}

func (r *ExamRepository) FindAttemptByTokenHash(ctx context.Context, tokenHash string) (serviceexam.Attempt, error) {
	var row ExamAttemptDO
	if err := r.db.WithContext(ctx).
		Where(ExamAttemptColumns.ExamTokenHash+" = ?", tokenHash).
		First(&row).Error; err != nil {
		return serviceexam.Attempt{}, err
	}
	return attemptFromDO(row), nil
}

func (r *ExamRepository) ListFixedSnapshotQuestions(ctx context.Context, tenantID uint64, examID uint64) ([]serviceexam.SnapshotSourceQuestion, error) {
	return []serviceexam.SnapshotSourceQuestion{}, nil
}

func (r *ExamRepository) ListFrozenLiveSnapshotQuestions(ctx context.Context, tenantID uint64, examID uint64) ([]serviceexam.SnapshotSourceQuestion, error) {
	return []serviceexam.SnapshotSourceQuestion{}, nil
}

func examFromDO(row ExamDO, buildMode string) serviceexam.Exam {
	return serviceexam.Exam{
		ID:               row.ID,
		TenantID:         row.TenantID,
		PaperID:          row.PaperID,
		Name:             row.Name,
		StartTime:        row.StartTime,
		EndTime:          row.EndTime,
		DurationMinutes:  row.DurationMinutes,
		MaxAttempts:      row.MaxAttempts,
		ResultStrategy:   row.ResultStrategy,
		PublishMode:      row.PublishMode,
		ScorePublishTime: row.ScorePublishTime,
		InviteCode:       row.InviteCode,
		Status:           row.Status,
		BuildMode:        buildMode,
	}
}

func attemptFromDO(row ExamAttemptDO) serviceexam.Attempt {
	return serviceexam.Attempt{
		ID:                 row.ID,
		TenantID:           row.TenantID,
		ExamID:             row.ExamID,
		UserID:             row.UserID,
		AttemptNo:          row.AttemptNo,
		Status:             row.Status,
		StartedAt:          row.StartedAt,
		SubmittedAt:        row.SubmittedAt,
		ExamTokenHash:      row.ExamTokenHash,
		ExamTokenExpiresAt: row.ExamTokenExpiresAt,
		Version:            row.Version,
	}
}

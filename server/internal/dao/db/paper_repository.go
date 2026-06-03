package db

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	servicepaper "github.com/lifei6671/papermind/server/internal/service/paper"
	"github.com/lifei6671/papermind/server/library/constant"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/plugin/soft_delete"
)

type PaperRepository struct {
	db  *gorm.DB
	now func() int64
}

type PaperRepositoryOptions struct {
	Now func() int64
}

type paperRuleConfigJSON struct {
	QuestionScope              string                             `json:"question_scope"`
	TagNames                   []string                           `json:"tag_names"`
	DifficultyPercentages      servicepaper.DifficultyPercentages `json:"difficulty_percentages"`
	PrioritizeQuality          bool                               `json:"prioritize_quality"`
	ExcludeRecentExamQuestions bool                               `json:"exclude_recent_exam_questions"`
	ExcludeUsedQuestions       bool                               `json:"exclude_used_questions"`
}

type paperListRow struct {
	PaperDO
	CreatorName string `gorm:"column:creator_name"`
}

func NewPaperRepository(gormDB *gorm.DB, options PaperRepositoryOptions) *PaperRepository {
	now := options.Now
	if now == nil {
		now = func() int64 { return time.Now().UnixMilli() }
	}
	return &PaperRepository{db: gormDB, now: now}
}

func (r *PaperRepository) ListPapers(ctx context.Context, input servicepaper.ListPapersInput) ([]servicepaper.Paper, error) {
	query := r.db.WithContext(ctx).Table(PaperDO{}.TableName()+" AS papers").
		Select(r.paperSelectColumns()).
		Joins("LEFT JOIN users AS users ON users.id = papers."+BaseColumns.CreatedBy+" AND papers."+BaseColumns.CreatedByType+" = ? AND users.deleted_at = 0", AuditActorTenantUser).
		Where("papers."+PaperColumns.TenantID+" = ?", input.TenantID).
		Where("papers."+PaperColumns.DeletedAt+" = ?", 0)
	if input.SpaceID != nil {
		query = query.Where(r.db.Where("papers."+PaperColumns.SpaceID+" IS NULL").Or("papers."+PaperColumns.SpaceID+" = ?", *input.SpaceID))
	}
	var rows []paperListRow
	if err := query.
		Order("papers." + BaseColumns.CreatedAt + " DESC").
		Order("papers." + PaperColumns.ID + " DESC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]servicepaper.Paper, 0, len(rows))
	for _, row := range rows {
		items = append(items, paperFromListRow(row))
	}
	return items, nil
}

func (r *PaperRepository) GetPaper(ctx context.Context, tenantID uint64, paperID uint64) (servicepaper.Paper, error) {
	row, err := r.findPaperRow(ctx, tenantID, paperID)
	if err != nil {
		return servicepaper.Paper{}, err
	}
	return paperFromListRow(row), nil
}

func (r *PaperRepository) HasPublishedRuleLiveExam(ctx context.Context, tenantID uint64, paperID uint64) (bool, error) {
	var total int64
	err := r.db.WithContext(ctx).
		Table(ExamDO{}.TableName()+" AS exams").
		Joins(
			"JOIN "+ExamLiveQuestionPoolDO{}.TableName()+" AS pools ON pools."+ExamLiveQuestionPoolColumns.TenantID+
				" = exams."+ExamColumns.TenantID+" AND pools."+ExamLiveQuestionPoolColumns.ExamID+" = exams."+ExamColumns.ID,
		).
		Where("exams."+ExamColumns.TenantID+" = ?", tenantID).
		Where("exams."+ExamColumns.PaperID+" = ?", paperID).
		Where("exams."+ExamColumns.Status+" = ?", constant.ExamStatusPublished).
		Where("exams."+ExamColumns.DeletedAt+" = ?", 0).
		Count(&total).Error
	return total > 0, err
}

func (r *PaperRepository) GetPaperSpaceID(ctx context.Context, tenantID uint64, paperID uint64) (*uint64, error) {
	var row PaperDO
	err := r.db.WithContext(ctx).
		Select(PaperColumns.SpaceID).
		Where(PaperColumns.TenantID+" = ?", tenantID).
		Where(PaperColumns.ID+" = ?", paperID).
		Where(PaperColumns.DeletedAt+" = ?", 0).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, servicepaper.ErrPaperNotFound
	}
	if err != nil {
		return nil, err
	}
	return row.SpaceID, nil
}

func (r *PaperRepository) SectionSortOrderExists(ctx context.Context, tenantID uint64, paperID uint64, sortOrder int) (bool, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&PaperSectionDO{}).
		Where(PaperSectionColumns.TenantID+" = ?", tenantID).
		Where(PaperSectionColumns.PaperID+" = ?", paperID).
		Where(PaperSectionColumns.SortOrder+" = ?", sortOrder).
		Where(PaperSectionColumns.DeletedAt+" = ?", 0).
		Count(&total).Error
	return total > 0, err
}

func (r *PaperRepository) CreateSection(ctx context.Context, section servicepaper.Section) (servicepaper.Section, error) {
	now := r.now()
	row := PaperSectionDO{
		BaseFields: BaseFields{
			CreatedAt:     now,
			CreatedByType: AuditActorTenantUser,
			UpdatedAt:     now,
			UpdatedByType: AuditActorTenantUser,
			Version:       1,
			ExtJSON:       datatypes.JSON("{}"),
		},
		TenantID:      section.TenantID,
		PaperID:       section.PaperID,
		SortOrder:     section.SortOrder,
		Name:          section.Name,
		QuestionType:  section.QuestionType,
		Instructions:  section.Instructions,
		TotalScore:    section.TotalScore,
		QuestionCount: section.QuestionCount,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return servicepaper.Section{}, err
	}
	return sectionFromDO(row), nil
}

func (r *PaperRepository) UpdateSection(ctx context.Context, input servicepaper.UpdateSectionInput) error {
	return r.db.WithContext(ctx).Model(&PaperSectionDO{}).
		Where(PaperSectionColumns.TenantID+" = ?", input.TenantID).
		Where(PaperSectionColumns.ID+" = ?", input.SectionID).
		Where(PaperSectionColumns.DeletedAt+" = ?", 0).
		Updates(map[string]any{
			PaperSectionColumns.Name:         input.Name,
			PaperSectionColumns.QuestionType: input.QuestionType,
			PaperSectionColumns.Instructions: input.Instructions,
			BaseColumns.UpdatedAt:            r.now(),
			BaseColumns.UpdatedByType:        AuditActorTenantUser,
			BaseColumns.Version:              gorm.Expr(BaseColumns.Version + " + 1"),
		}).Error
}

func (r *PaperRepository) ReorderSections(ctx context.Context, tenantID uint64, paperID uint64, orders []servicepaper.SectionOrder) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if len(orders) == 0 {
			return nil
		}
		var maxSortOrder int
		if err := tx.Model(&PaperSectionDO{}).
			Where(PaperSectionColumns.TenantID+" = ?", tenantID).
			Where(PaperSectionColumns.PaperID+" = ?", paperID).
			Where(PaperSectionColumns.DeletedAt+" = ?", 0).
			Select("COALESCE(MAX(" + PaperSectionColumns.SortOrder + "), 0)").
			Scan(&maxSortOrder).Error; err != nil {
			return err
		}
		// 先把待重排大题搬到当前排序区间之外，避免唯一索引 `(tenant_id, paper_id, sort_order)` 在交换顺序时冲突。
		for index, order := range orders {
			result := tx.Model(&PaperSectionDO{}).
				Where(PaperSectionColumns.TenantID+" = ?", tenantID).
				Where(PaperSectionColumns.PaperID+" = ?", paperID).
				Where(PaperSectionColumns.ID+" = ?", order.SectionID).
				Where(PaperSectionColumns.DeletedAt+" = ?", 0).
				Update(PaperSectionColumns.SortOrder, maxSortOrder+index+1)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return servicepaper.ErrPaperNotFound
			}
		}
		for _, order := range orders {
			result := tx.Model(&PaperSectionDO{}).
				Where(PaperSectionColumns.TenantID+" = ?", tenantID).
				Where(PaperSectionColumns.PaperID+" = ?", paperID).
				Where(PaperSectionColumns.ID+" = ?", order.SectionID).
				Where(PaperSectionColumns.DeletedAt+" = ?", 0).
				Update(PaperSectionColumns.SortOrder, order.SortOrder)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return servicepaper.ErrPaperNotFound
			}
		}
		return nil
	})
}

func (r *PaperRepository) DeleteSectionCascade(ctx context.Context, tenantID uint64, paperID uint64, sectionID uint64) error {
	now := r.now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var maxSortOrder int
		if err := tx.Unscoped().Model(&PaperSectionDO{}).
			Where(PaperSectionColumns.TenantID+" = ?", tenantID).
			Where(PaperSectionColumns.PaperID+" = ?", paperID).
			Select("COALESCE(MAX(" + PaperSectionColumns.SortOrder + "), 0)").
			Scan(&maxSortOrder).Error; err != nil {
			return err
		}
		result := tx.Model(&PaperSectionDO{}).
			Where(PaperSectionColumns.TenantID+" = ?", tenantID).
			Where(PaperSectionColumns.PaperID+" = ?", paperID).
			Where(PaperSectionColumns.ID+" = ?", sectionID).
			Where(PaperSectionColumns.DeletedAt+" = ?", 0).
			Updates(map[string]any{
				PaperSectionColumns.SortOrder: maxSortOrder + 1,
				PaperSectionColumns.DeletedAt: now,
				BaseColumns.UpdatedAt:         now,
				BaseColumns.UpdatedByType:     AuditActorTenantUser,
				BaseColumns.Version:           gorm.Expr(BaseColumns.Version + " + 1"),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return servicepaper.ErrPaperNotFound
		}
		if err := tx.Where(PaperSectionQuestionColumns.TenantID+" = ?", tenantID).
			Where(PaperSectionQuestionColumns.PaperID+" = ?", paperID).
			Where(PaperSectionQuestionColumns.SectionID+" = ?", sectionID).
			Delete(&PaperSectionQuestionDO{}).Error; err != nil {
			return err
		}
		if err := tx.Where(PaperSectionRuleColumns.TenantID+" = ?", tenantID).
			Where(PaperSectionRuleColumns.PaperID+" = ?", paperID).
			Where(PaperSectionRuleColumns.SectionID+" = ?", sectionID).
			Delete(&PaperSectionRuleDO{}).Error; err != nil {
			return err
		}

		var remaining []PaperSectionDO
		if err := tx.Where(PaperSectionColumns.TenantID+" = ?", tenantID).
			Where(PaperSectionColumns.PaperID+" = ?", paperID).
			Where(PaperSectionColumns.DeletedAt+" = ?", 0).
			Order(PaperSectionColumns.SortOrder + " ASC").
			Find(&remaining).Error; err != nil {
			return err
		}
		for index, section := range remaining {
			if err := tx.Model(&PaperSectionDO{}).
				Where(PaperSectionColumns.TenantID+" = ?", tenantID).
				Where(PaperSectionColumns.PaperID+" = ?", paperID).
				Where(PaperSectionColumns.ID+" = ?", section.ID).
				Update(PaperSectionColumns.SortOrder, maxSortOrder+index+2).Error; err != nil {
				return err
			}
		}
		var deletedSections []PaperSectionDO
		if err := tx.Unscoped().
			Where(PaperSectionColumns.TenantID+" = ?", tenantID).
			Where(PaperSectionColumns.PaperID+" = ?", paperID).
			Where(PaperSectionColumns.DeletedAt+" <> ?", 0).
			Order(PaperSectionColumns.SortOrder + " ASC").
			Find(&deletedSections).Error; err != nil {
			return err
		}
		for index, section := range deletedSections {
			if err := tx.Unscoped().Model(&PaperSectionDO{}).
				Where(PaperSectionColumns.TenantID+" = ?", tenantID).
				Where(PaperSectionColumns.PaperID+" = ?", paperID).
				Where(PaperSectionColumns.ID+" = ?", section.ID).
				Updates(map[string]any{
					PaperSectionColumns.SortOrder: maxSortOrder + len(remaining) + index + 2,
					BaseColumns.UpdatedAt:         now,
					BaseColumns.UpdatedByType:     AuditActorTenantUser,
					BaseColumns.Version:           gorm.Expr(BaseColumns.Version + " + 1"),
				}).Error; err != nil {
				return err
			}
		}
		for index, section := range remaining {
			if err := tx.Model(&PaperSectionDO{}).
				Where(PaperSectionColumns.TenantID+" = ?", tenantID).
				Where(PaperSectionColumns.PaperID+" = ?", paperID).
				Where(PaperSectionColumns.ID+" = ?", section.ID).
				Updates(map[string]any{
					PaperSectionColumns.SortOrder: index + 1,
					BaseColumns.UpdatedAt:         now,
					BaseColumns.UpdatedByType:     AuditActorTenantUser,
					BaseColumns.Version:           gorm.Expr(BaseColumns.Version + " + 1"),
				}).Error; err != nil {
				return err
			}
		}
		return r.recalculateQuestionAggregates(tx, tenantID, paperID, now)
	})
}

func (r *PaperRepository) ListActiveSections(ctx context.Context, tenantID uint64, paperID uint64) ([]servicepaper.Section, error) {
	var rows []PaperSectionDO
	if err := r.db.WithContext(ctx).
		Where(PaperSectionColumns.TenantID+" = ?", tenantID).
		Where(PaperSectionColumns.PaperID+" = ?", paperID).
		Where(PaperSectionColumns.DeletedAt+" = ?", 0).
		Order(PaperSectionColumns.SortOrder + " ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]servicepaper.Section, 0, len(rows))
	for _, row := range rows {
		items = append(items, sectionFromDO(row))
	}
	return items, nil
}

func (r *PaperRepository) CreatePaper(ctx context.Context, paper servicepaper.Paper) (servicepaper.Paper, error) {
	now := r.now()
	row := PaperDO{
		BaseFields: BaseFields{
			CreatedAt:     now,
			CreatedBy:     paper.CreatedBy,
			CreatedByType: AuditActorTenantUser,
			UpdatedAt:     now,
			UpdatedBy:     paper.CreatedBy,
			UpdatedByType: AuditActorTenantUser,
			Version:       1,
			ExtJSON:       datatypes.JSON("{}"),
		},
		TenantID:         paper.TenantID,
		SpaceID:          paper.SpaceID,
		Name:             paper.Name,
		Description:      paper.Description,
		DurationMinutes:  paper.DurationMinutes,
		GradeLevel:       paper.GradeLevel,
		TotalScore:       paper.TotalScore,
		BuildMode:        paper.BuildMode,
		ShuffleQuestions: paper.ShuffleQuestions,
		ShowAnalysis:     paper.ShowAnalysis,
		Status:           paper.Status,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return servicepaper.Paper{}, err
	}
	return r.GetPaper(ctx, paper.TenantID, row.ID)
}

func (r *PaperRepository) UpdatePaper(ctx context.Context, input servicepaper.UpdatePaperInput) (servicepaper.Paper, error) {
	updates := map[string]any{
		PaperColumns.Name:         input.Name,
		PaperColumns.Description:  input.Description,
		PaperColumns.GradeLevel:   input.GradeLevel,
		BaseColumns.UpdatedAt:     r.now(),
		BaseColumns.UpdatedBy:     input.ActorID,
		BaseColumns.UpdatedByType: AuditActorTenantUser,
		BaseColumns.Version:       gorm.Expr(BaseColumns.Version + " + 1"),
	}
	if input.DurationMinutes != nil {
		updates[PaperColumns.DurationMinutes] = *input.DurationMinutes
	}
	if input.ChangeSpace {
		updates[PaperColumns.SpaceID] = input.TargetSpaceID
	}
	result := r.db.WithContext(ctx).Model(&PaperDO{}).
		Where(PaperColumns.TenantID+" = ?", input.TenantID).
		Where(PaperColumns.ID+" = ?", input.PaperID).
		Where(PaperColumns.DeletedAt+" = ?", 0).
		Updates(updates)
	if result.Error != nil {
		return servicepaper.Paper{}, result.Error
	}
	if result.RowsAffected == 0 {
		return servicepaper.Paper{}, servicepaper.ErrPaperNotFound
	}
	return r.GetPaper(ctx, input.TenantID, input.PaperID)
}

func (r *PaperRepository) UpdatePaperStatus(ctx context.Context, tenantID uint64, paperID uint64, status string, actorID uint64) (servicepaper.Paper, error) {
	result := r.db.WithContext(ctx).Model(&PaperDO{}).
		Where(PaperColumns.TenantID+" = ?", tenantID).
		Where(PaperColumns.ID+" = ?", paperID).
		Where(PaperColumns.DeletedAt+" = ?", 0).
		Updates(map[string]any{
			PaperColumns.Status:       status,
			BaseColumns.UpdatedAt:     r.now(),
			BaseColumns.UpdatedBy:     actorID,
			BaseColumns.UpdatedByType: AuditActorTenantUser,
			BaseColumns.Version:       gorm.Expr(BaseColumns.Version + " + 1"),
		})
	if result.Error != nil {
		return servicepaper.Paper{}, result.Error
	}
	if result.RowsAffected == 0 {
		return servicepaper.Paper{}, servicepaper.ErrPaperNotFound
	}
	return r.GetPaper(ctx, tenantID, paperID)
}

func (r *PaperRepository) DeletePaper(ctx context.Context, tenantID uint64, paperID uint64) error {
	now := r.now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		referenced, err := r.paperReferencedTx(ctx, tx, tenantID, paperID)
		if err != nil {
			return err
		}
		if referenced {
			return servicepaper.ErrPaperInUse
		}
		// 删除试卷先软删除主表，随后清理组卷关系；已删除试卷不会再参与列表、发布和权限反查。
		result := tx.Model(&PaperDO{}).
			Where(PaperColumns.TenantID+" = ?", tenantID).
			Where(PaperColumns.ID+" = ?", paperID).
			Where(PaperColumns.DeletedAt+" = ?", 0).
			Updates(map[string]any{
				PaperColumns.DeletedAt:    soft_delete.DeletedAt(now),
				BaseColumns.UpdatedAt:     now,
				BaseColumns.UpdatedByType: AuditActorTenantUser,
				BaseColumns.Version:       gorm.Expr(BaseColumns.Version + " + 1"),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return servicepaper.ErrPaperNotFound
		}
		if err := tx.Model(&PaperSectionDO{}).
			Where(PaperSectionColumns.TenantID+" = ?", tenantID).
			Where(PaperSectionColumns.PaperID+" = ?", paperID).
			Where(PaperSectionColumns.DeletedAt+" = ?", 0).
			Updates(map[string]any{
				PaperSectionColumns.DeletedAt: soft_delete.DeletedAt(now),
				BaseColumns.UpdatedAt:         now,
				BaseColumns.UpdatedByType:     AuditActorTenantUser,
				BaseColumns.Version:           gorm.Expr(BaseColumns.Version + " + 1"),
			}).Error; err != nil {
			return err
		}
		if err := tx.Where(PaperSectionQuestionColumns.TenantID+" = ?", tenantID).
			Where(PaperSectionQuestionColumns.PaperID+" = ?", paperID).
			Delete(&PaperSectionQuestionDO{}).Error; err != nil {
			return err
		}
		return tx.Where(PaperSectionRuleColumns.TenantID+" = ?", tenantID).
			Where(PaperSectionRuleColumns.PaperID+" = ?", paperID).
			Delete(&PaperSectionRuleDO{}).Error
	})
}

func (r *PaperRepository) PaperReferenced(ctx context.Context, tenantID uint64, paperID uint64) (bool, error) {
	return r.paperReferencedTx(ctx, r.db.WithContext(ctx), tenantID, paperID)
}

func (r *PaperRepository) paperReferencedTx(ctx context.Context, tx *gorm.DB, tenantID uint64, paperID uint64) (bool, error) {
	var examCount int64
	if err := tx.WithContext(ctx).Model(&ExamDO{}).
		Where(ExamColumns.TenantID+" = ?", tenantID).
		Where(ExamColumns.PaperID+" = ?", paperID).
		Where(ExamColumns.DeletedAt+" = ?", 0).
		Count(&examCount).Error; err != nil {
		return false, err
	}
	return examCount > 0, nil
}

func (r *PaperRepository) PaperQuestionExists(ctx context.Context, tenantID uint64, paperID uint64, questionID uint64) (bool, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&PaperSectionQuestionDO{}).
		Where(PaperSectionQuestionColumns.TenantID+" = ?", tenantID).
		Where(PaperSectionQuestionColumns.PaperID+" = ?", paperID).
		Where(PaperSectionQuestionColumns.QuestionID+" = ?", questionID).
		Count(&total).Error
	return total > 0, err
}

func (r *PaperRepository) QuestionUsableForPaper(ctx context.Context, tenantID uint64, paperID uint64, questionID uint64) (bool, error) {
	var total int64
	err := r.db.WithContext(ctx).Table(QuestionDO{}.TableName()+" AS q").
		Joins("JOIN "+PaperDO{}.TableName()+" AS p ON p."+PaperColumns.TenantID+" = q."+QuestionColumns.TenantID+" AND p."+PaperColumns.ID+" = ? AND p."+PaperColumns.DeletedAt+" = 0", paperID).
		Where("q."+QuestionColumns.TenantID+" = ?", tenantID).
		Where("q."+QuestionColumns.ID+" = ?", questionID).
		Where("q."+QuestionColumns.Status+" = ?", "enabled").
		Where("q."+QuestionColumns.DeletedAt+" = ?", 0).
		Where(r.db.Where("q." + QuestionColumns.SpaceID + " IS NULL").Or("q." + QuestionColumns.SpaceID + " = p." + PaperColumns.SpaceID)).
		Count(&total).Error
	return total > 0, err
}

func (r *PaperRepository) QuestionUsableForScope(ctx context.Context, tenantID uint64, spaceID *uint64, questionID uint64) (bool, error) {
	var total int64
	query := r.db.WithContext(ctx).Table(QuestionDO{}.TableName()+" AS q").
		Where("q."+QuestionColumns.TenantID+" = ?", tenantID).
		Where("q."+QuestionColumns.ID+" = ?", questionID).
		Where("q."+QuestionColumns.Status+" = ?", "enabled").
		Where("q."+QuestionColumns.DeletedAt+" = ?", 0)
	if spaceID == nil {
		query = query.Where("q." + QuestionColumns.SpaceID + " IS NULL")
	} else {
		query = query.Where(r.db.Where("q." + QuestionColumns.SpaceID + " IS NULL").Or("q." + QuestionColumns.SpaceID + " = ?", *spaceID))
	}
	if err := query.Count(&total).Error; err != nil {
		return false, err
	}
	return total > 0, nil
}

func (r *PaperRepository) AddSectionQuestionAndRecalculate(ctx context.Context, question servicepaper.SectionQuestion) error {
	now := r.now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		row := PaperSectionQuestionDO{
			BaseFields: BaseFields{
				CreatedAt:     now,
				CreatedByType: AuditActorTenantUser,
				UpdatedAt:     now,
				UpdatedByType: AuditActorTenantUser,
				Version:       1,
				ExtJSON:       datatypes.JSON("{}"),
			},
			TenantID:       question.TenantID,
			SectionID:      question.SectionID,
			PaperID:        question.PaperID,
			QuestionID:     question.QuestionID,
			SortOrder:      question.SortOrder,
			Score:          question.Score,
			ShuffleOptions: question.ShuffleOptions,
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		var sectionAggregate struct {
			TotalScore    float64
			QuestionCount int
		}
		if err := tx.Model(&PaperSectionQuestionDO{}).
			Select("COALESCE(SUM(score), 0) AS total_score, COUNT(*) AS question_count").
			Where(PaperSectionQuestionColumns.TenantID+" = ?", question.TenantID).
			Where(PaperSectionQuestionColumns.PaperID+" = ?", question.PaperID).
			Where(PaperSectionQuestionColumns.SectionID+" = ?", question.SectionID).
			Scan(&sectionAggregate).Error; err != nil {
			return err
		}
		if err := tx.Model(&PaperSectionDO{}).
			Where(PaperSectionColumns.TenantID+" = ?", question.TenantID).
			Where(PaperSectionColumns.PaperID+" = ?", question.PaperID).
			Where(PaperSectionColumns.ID+" = ?", question.SectionID).
			Updates(map[string]any{
				PaperSectionColumns.TotalScore:    formatRepositoryScore(sectionAggregate.TotalScore),
				PaperSectionColumns.QuestionCount: sectionAggregate.QuestionCount,
				BaseColumns.UpdatedAt:             now,
				BaseColumns.UpdatedByType:         AuditActorTenantUser,
				BaseColumns.Version:               gorm.Expr(BaseColumns.Version + " + 1"),
			}).Error; err != nil {
			return err
		}
		var paperTotal float64
		if err := tx.Model(&PaperSectionDO{}).
			Select("COALESCE(SUM(total_score), 0)").
			Where(PaperSectionColumns.TenantID+" = ?", question.TenantID).
			Where(PaperSectionColumns.PaperID+" = ?", question.PaperID).
			Where(PaperSectionColumns.DeletedAt+" = ?", 0).
			Scan(&paperTotal).Error; err != nil {
			return err
		}
		return tx.Model(&PaperDO{}).
			Where(PaperColumns.TenantID+" = ?", question.TenantID).
			Where(PaperColumns.ID+" = ?", question.PaperID).
			Where(PaperColumns.DeletedAt+" = ?", 0).
			Updates(map[string]any{
				PaperColumns.TotalScore:   formatRepositoryScore(paperTotal),
				BaseColumns.UpdatedAt:     now,
				BaseColumns.UpdatedByType: AuditActorTenantUser,
				BaseColumns.Version:       gorm.Expr(BaseColumns.Version + " + 1"),
			}).Error
	})
}

func (r *PaperRepository) UpdateSectionQuestionAndRecalculate(ctx context.Context, input servicepaper.UpdateSectionQuestionInput) error {
	now := r.now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []PaperSectionQuestionDO
		if err := tx.Where(PaperSectionQuestionColumns.TenantID+" = ?", input.TenantID).
			Where(PaperSectionQuestionColumns.PaperID+" = ?", input.PaperID).
			Where(PaperSectionQuestionColumns.SectionID+" = ?", input.SectionID).
			Order(PaperSectionQuestionColumns.SortOrder + " ASC").
			Find(&rows).Error; err != nil {
			return err
		}
		targetIndex := -1
		for index := range rows {
			if rows[index].QuestionID == input.QuestionID {
				targetIndex = index
				rows[index].Score = input.Score
				break
			}
		}
		if targetIndex < 0 {
			return servicepaper.ErrPaperNotFound
		}
		target := rows[targetIndex]
		reordered := make([]PaperSectionQuestionDO, 0, len(rows))
		reordered = append(reordered, rows[:targetIndex]...)
		reordered = append(reordered, rows[targetIndex+1:]...)
		insertIndex := input.SortOrder - 1
		if insertIndex < 0 {
			insertIndex = 0
		}
		if insertIndex > len(reordered) {
			insertIndex = len(reordered)
		}
		reordered = append(reordered, PaperSectionQuestionDO{})
		copy(reordered[insertIndex+1:], reordered[insertIndex:])
		reordered[insertIndex] = target
		maxSortOrder := 0
		for _, row := range rows {
			if row.SortOrder > maxSortOrder {
				maxSortOrder = row.SortOrder
			}
		}
		for index, row := range reordered {
			result := tx.Model(&PaperSectionQuestionDO{}).
				Where(PaperSectionQuestionColumns.TenantID+" = ?", input.TenantID).
				Where(PaperSectionQuestionColumns.PaperID+" = ?", input.PaperID).
				Where(PaperSectionQuestionColumns.SectionID+" = ?", input.SectionID).
				Where(PaperSectionQuestionColumns.QuestionID+" = ?", row.QuestionID).
				Updates(map[string]any{
					PaperSectionQuestionColumns.SortOrder: maxSortOrder + index + 1,
					BaseColumns.UpdatedAt:                 now,
					BaseColumns.UpdatedByType:             AuditActorTenantUser,
					BaseColumns.Version:                   gorm.Expr(BaseColumns.Version + " + 1"),
				})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return servicepaper.ErrPaperNotFound
			}
		}
		for index, row := range reordered {
			updates := map[string]any{
				PaperSectionQuestionColumns.SortOrder: index + 1,
				BaseColumns.UpdatedAt:                 now,
				BaseColumns.UpdatedByType:             AuditActorTenantUser,
				BaseColumns.Version:                   gorm.Expr(BaseColumns.Version + " + 1"),
			}
			if row.QuestionID == input.QuestionID {
				updates[PaperSectionQuestionColumns.Score] = input.Score
			}
			result := tx.Model(&PaperSectionQuestionDO{}).
				Where(PaperSectionQuestionColumns.TenantID+" = ?", input.TenantID).
				Where(PaperSectionQuestionColumns.PaperID+" = ?", input.PaperID).
				Where(PaperSectionQuestionColumns.SectionID+" = ?", input.SectionID).
				Where(PaperSectionQuestionColumns.QuestionID+" = ?", row.QuestionID).
				Updates(updates)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return servicepaper.ErrPaperNotFound
			}
		}
		return r.recalculateQuestionAggregates(tx, input.TenantID, input.PaperID, now)
	})
}

func (r *PaperRepository) DeleteSectionQuestionAndRecalculate(ctx context.Context, tenantID uint64, paperID uint64, sectionID uint64, questionID uint64) error {
	now := r.now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Where(PaperSectionQuestionColumns.TenantID+" = ?", tenantID).
			Where(PaperSectionQuestionColumns.PaperID+" = ?", paperID).
			Where(PaperSectionQuestionColumns.SectionID+" = ?", sectionID).
			Where(PaperSectionQuestionColumns.QuestionID+" = ?", questionID).
			Delete(&PaperSectionQuestionDO{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return servicepaper.ErrPaperNotFound
		}
		return r.recalculateQuestionAggregates(tx, tenantID, paperID, now)
	})
}

func (r *PaperRepository) CreateRule(ctx context.Context, rule servicepaper.Rule) (servicepaper.Rule, error) {
	now := r.now()
	return r.createRule(r.db.WithContext(ctx), rule, now)
}

func (r *PaperRepository) CreateRuleAndRecalculate(ctx context.Context, rule servicepaper.Rule, buildMode string) (servicepaper.Rule, error) {
	now := r.now()
	var created servicepaper.Rule
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		next, err := r.createRule(tx, rule, now)
		if err != nil {
			return err
		}
		created = next
		return r.recalculateRuleAggregates(tx, rule.TenantID, rule.PaperID, buildMode, now)
	})
	if err != nil {
		return servicepaper.Rule{}, err
	}
	return created, nil
}

func (r *PaperRepository) createRule(tx *gorm.DB, rule servicepaper.Rule, now int64) (servicepaper.Rule, error) {
	row := PaperSectionRuleDO{
		BaseFields: BaseFields{
			CreatedAt:     now,
			CreatedByType: AuditActorTenantUser,
			UpdatedAt:     now,
			UpdatedByType: AuditActorTenantUser,
			Version:       1,
			ExtJSON:       datatypes.JSON(mustPaperRuleConfigJSON(rule)),
		},
		TenantID:         rule.TenantID,
		SectionID:        rule.SectionID,
		PaperID:          rule.PaperID,
		SortOrder:        rule.SortOrder,
		Difficulty:       rule.Difficulty,
		TagFilter:        rule.TagFilter,
		QuestionCount:    rule.QuestionCount,
		ScorePerQuestion: rule.ScorePerQuestion,
		ShuffleOptions:   rule.ShuffleOptions,
	}
	if err := tx.Create(&row).Error; err != nil {
		return servicepaper.Rule{}, err
	}
	return ruleFromDO(row), nil
}

func (r *PaperRepository) MatchQuestionsForRule(ctx context.Context, tenantID uint64, paperID uint64, rule servicepaper.Rule) ([]servicepaper.MatchedQuestion, error) {
	var section PaperSectionDO
	if err := r.db.WithContext(ctx).
		Where(PaperSectionColumns.TenantID+" = ?", tenantID).
		Where(PaperSectionColumns.PaperID+" = ?", paperID).
		Where(PaperSectionColumns.ID+" = ?", rule.SectionID).
		Where(PaperSectionColumns.DeletedAt+" = ?", 0).
		First(&section).Error; err != nil {
		return nil, err
	}

	tagIDs := tagIDsFromFilter(rule.TagFilter)
	query := r.db.WithContext(ctx).Table(QuestionDO{}.TableName()+" AS q").
		Select("q."+QuestionColumns.ID+" AS id, q."+QuestionColumns.ScoreDefault+" AS score_default").
		Joins("JOIN "+PaperDO{}.TableName()+" AS p ON p."+PaperColumns.TenantID+" = q."+QuestionColumns.TenantID+" AND p."+PaperColumns.ID+" = ? AND p."+PaperColumns.DeletedAt+" = 0", paperID).
		Where("q."+QuestionColumns.TenantID+" = ?", tenantID).
		Where("q."+QuestionColumns.Type+" = ?", section.QuestionType).
		Where("q."+QuestionColumns.Status+" = ?", "enabled").
		Where("q."+QuestionColumns.DeletedAt+" = ?", 0).
		Where(r.db.Where("q." + QuestionColumns.SpaceID + " IS NULL").Or("q." + QuestionColumns.SpaceID + " = p." + PaperColumns.SpaceID))
	if rule.Difficulty != nil {
		query = query.Where("q."+QuestionColumns.Difficulty+" = ?", *rule.Difficulty)
	}
	if len(tagIDs) > 0 {
		// 规则标签需要全部命中；这里按题目聚合后校验命中的不同 tag_id 数量，避免一题只命中部分标签也被抽中。
		query = query.Joins("JOIN "+QuestionTagDO{}.TableName()+" AS qt ON qt."+QuestionTagColumns.TenantID+" = q."+QuestionColumns.TenantID+" AND qt."+QuestionTagColumns.QuestionID+" = q."+QuestionColumns.ID).
			Where("qt."+QuestionTagColumns.TagID+" IN ?", tagIDs).
			Group("q."+QuestionColumns.ID+", q."+QuestionColumns.ScoreDefault).
			Having("COUNT(DISTINCT qt."+QuestionTagColumns.TagID+") = ?", len(tagIDs))
	}

	var questions []servicepaper.MatchedQuestion
	orderExpr := "q." + QuestionColumns.ID + " ASC"
	if rule.PrioritizeQuality {
		orderExpr = "q." + QuestionColumns.QualityScore + " DESC, q." + QuestionColumns.ID + " ASC"
	}
	if err := query.Order(orderExpr).Scan(&questions).Error; err != nil {
		return nil, err
	}
	return questions, nil
}

func (r *PaperRepository) RecentExamQuestionIDs(ctx context.Context, tenantID uint64, paperID uint64, limit int) (map[uint64]bool, error) {
	if limit <= 0 {
		return map[uint64]bool{}, nil
	}
	var paperIDs []uint64
	if err := r.db.WithContext(ctx).Table(ExamDO{}.TableName()+" AS e").
		Select("e."+ExamColumns.PaperID).
		Joins("JOIN "+PaperDO{}.TableName()+" AS ep ON ep."+PaperColumns.TenantID+" = e."+ExamColumns.TenantID+" AND ep."+PaperColumns.ID+" = e."+ExamColumns.PaperID+" AND ep."+PaperColumns.DeletedAt+" = 0").
		Joins("JOIN "+PaperDO{}.TableName()+" AS current_paper ON current_paper."+PaperColumns.TenantID+" = e."+ExamColumns.TenantID+" AND current_paper."+PaperColumns.ID+" = ? AND current_paper."+PaperColumns.DeletedAt+" = 0", paperID).
		Where("e."+ExamColumns.TenantID+" = ?", tenantID).
		Where("e."+ExamColumns.PaperID+" <> ?", paperID).
		Where("e."+ExamColumns.Status+" = ?", constant.ExamStatusPublished).
		Where("e."+ExamColumns.DeletedAt+" = ?", 0).
		Where("COALESCE(current_paper." + PaperColumns.SpaceID + ", 0) = COALESCE(ep." + PaperColumns.SpaceID + ", 0)").
		Order("e." + BaseColumns.CreatedAt + " DESC").
		Limit(limit).
		Scan(&paperIDs).Error; err != nil {
		return nil, err
	}
	if len(paperIDs) == 0 {
		return map[uint64]bool{}, nil
	}
	var questionIDs []uint64
	if err := r.db.WithContext(ctx).Table(PaperSectionQuestionDO{}.TableName()).
		Select(PaperSectionQuestionColumns.QuestionID).
		Where(PaperSectionQuestionColumns.TenantID+" = ?", tenantID).
		Where(PaperSectionQuestionColumns.PaperID+" IN ?", paperIDs).
		Scan(&questionIDs).Error; err != nil {
		return nil, err
	}
	excluded := make(map[uint64]bool, len(questionIDs))
	for _, questionID := range questionIDs {
		excluded[questionID] = true
	}
	return excluded, nil
}

func (r *PaperRepository) TagIDsByNames(ctx context.Context, tenantID uint64, names []string) ([]uint64, error) {
	if len(names) == 0 {
		return []uint64{}, nil
	}
	var rows []struct {
		ID uint64 `gorm:"column:id"`
	}
	if err := r.db.WithContext(ctx).Table(TagDO{}.TableName()).
		Select(TagColumns.ID).
		Where(TagColumns.TenantID+" = ?", tenantID).
		Where(TagColumns.Name+" IN ?", names).
		Where(TagColumns.DeletedAt+" = ?", 0).
		Order(TagColumns.ID + " ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	tagIDs := make([]uint64, 0, len(rows))
	for _, row := range rows {
		tagIDs = append(tagIDs, row.ID)
	}
	return tagIDs, nil
}

func (r *PaperRepository) ListRules(ctx context.Context, tenantID uint64, paperID uint64) ([]servicepaper.Rule, error) {
	var rows []PaperSectionRuleDO
	if err := r.db.WithContext(ctx).
		Where(PaperSectionRuleColumns.TenantID+" = ?", tenantID).
		Where(PaperSectionRuleColumns.PaperID+" = ?", paperID).
		Order(PaperSectionRuleColumns.SortOrder + " ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]servicepaper.Rule, 0, len(rows))
	for _, row := range rows {
		items = append(items, ruleFromDO(row))
	}
	return items, nil
}

func (r *PaperRepository) GenerateFixedQuestionsAndRecalculate(ctx context.Context, tenantID uint64, paperID uint64, buildMode string, questions []servicepaper.SectionQuestion) error {
	now := r.now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where(PaperSectionQuestionColumns.TenantID+" = ?", tenantID).
			Where(PaperSectionQuestionColumns.PaperID+" = ?", paperID).
			Delete(&PaperSectionQuestionDO{}).Error; err != nil {
			return err
		}
		for _, question := range questions {
			// rule_fixed 生成结果落入固化题目表，后续审题、替换和考试快照都以该表为准。
			row := PaperSectionQuestionDO{
				BaseFields: BaseFields{
					CreatedAt:     now,
					CreatedByType: AuditActorTenantUser,
					UpdatedAt:     now,
					UpdatedByType: AuditActorTenantUser,
					Version:       1,
					ExtJSON:       datatypes.JSON("{}"),
				},
				TenantID:       question.TenantID,
				SectionID:      question.SectionID,
				PaperID:        question.PaperID,
				QuestionID:     question.QuestionID,
				SortOrder:      question.SortOrder,
				Score:          question.Score,
				ShuffleOptions: question.ShuffleOptions,
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		if err := r.recalculateQuestionAggregates(tx, tenantID, paperID, now); err != nil {
			return err
		}
		result := tx.Model(&PaperDO{}).
			Where(PaperColumns.TenantID+" = ?", tenantID).
			Where(PaperColumns.ID+" = ?", paperID).
			Where(PaperColumns.DeletedAt+" = ?", 0).
			Updates(map[string]any{
				PaperColumns.BuildMode:    buildMode,
				BaseColumns.UpdatedAt:     now,
				BaseColumns.UpdatedByType: AuditActorTenantUser,
				BaseColumns.Version:       gorm.Expr(BaseColumns.Version + " + 1"),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return servicepaper.ErrPaperNotFound
		}
		return nil
	})
}

func (r *PaperRepository) ReplaceGeneratedQuestionAndRecalculate(ctx context.Context, input servicepaper.ReplaceGeneratedQuestionInput) error {
	now := r.now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&PaperSectionQuestionDO{}).
			Where(PaperSectionQuestionColumns.TenantID+" = ?", input.TenantID).
			Where(PaperSectionQuestionColumns.PaperID+" = ?", input.PaperID).
			Where(PaperSectionQuestionColumns.SectionID+" = ?", input.SectionID).
			Where(PaperSectionQuestionColumns.QuestionID+" = ?", input.OldQuestionID).
			Updates(map[string]any{
				PaperSectionQuestionColumns.QuestionID: input.NewQuestionID,
				PaperSectionQuestionColumns.SortOrder:  input.SortOrder,
				PaperSectionQuestionColumns.Score:      input.Score,
				BaseColumns.UpdatedAt:                  now,
				BaseColumns.UpdatedByType:              AuditActorTenantUser,
				BaseColumns.Version:                    gorm.Expr(BaseColumns.Version + " + 1"),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return servicepaper.ErrPaperNotFound
		}
		return r.recalculateQuestionAggregates(tx, input.TenantID, input.PaperID, now)
	})
}

func (r *PaperRepository) AdjustGeneratedQuestionAndRecalculate(ctx context.Context, input servicepaper.AdjustGeneratedQuestionInput) error {
	return r.UpdateSectionQuestionAndRecalculate(ctx, servicepaper.UpdateSectionQuestionInput{
		TenantID:   input.TenantID,
		PaperID:    input.PaperID,
		SectionID:  input.SectionID,
		QuestionID: input.QuestionID,
		SortOrder:  input.SortOrder,
		Score:      input.Score,
	})
}

func (r *PaperRepository) FreezeLivePools(ctx context.Context, tenantID uint64, examID uint64, questionIDs []uint64) error {
	return errors.New("rule_live freeze is handled by exam repository")
}

func (r *PaperRepository) UpdateRule(ctx context.Context, input servicepaper.UpdateRuleInput) error {
	now := r.now()
	return r.updateRule(r.db.WithContext(ctx), input, now)
}

func (r *PaperRepository) UpdateRuleAndRecalculate(ctx context.Context, input servicepaper.UpdateRuleInput, buildMode string) error {
	now := r.now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := r.updateRule(tx, input, now); err != nil {
			return err
		}
		return r.recalculateRuleAggregates(tx, input.TenantID, input.PaperID, buildMode, now)
	})
}

func (r *PaperRepository) updateRule(tx *gorm.DB, input servicepaper.UpdateRuleInput, now int64) error {
	result := tx.Model(&PaperSectionRuleDO{}).
		Where(PaperSectionRuleColumns.TenantID+" = ?", input.TenantID).
		Where(PaperSectionRuleColumns.PaperID+" = ?", input.PaperID).
		Where(PaperSectionRuleColumns.ID+" = ?", input.RuleID).
		Updates(map[string]any{
			PaperSectionRuleColumns.SectionID:        input.SectionID,
			PaperSectionRuleColumns.SortOrder:        input.SortOrder,
			PaperSectionRuleColumns.Difficulty:       input.Difficulty,
			PaperSectionRuleColumns.TagFilter:        mustTagFilterJSON(input.TagIDs),
			PaperSectionRuleColumns.QuestionCount:    input.QuestionCount,
			PaperSectionRuleColumns.ScorePerQuestion: input.ScorePerQuestion,
			PaperSectionRuleColumns.ShuffleOptions:   input.ShuffleOptions,
			BaseColumns.ExtJSON:                      mustPaperRuleConfigJSONFromUpdate(input),
			BaseColumns.UpdatedAt:                    now,
			BaseColumns.UpdatedByType:                AuditActorTenantUser,
			BaseColumns.Version:                      gorm.Expr(BaseColumns.Version + " + 1"),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return servicepaper.ErrPaperNotFound
	}
	return nil
}

func (r *PaperRepository) DeleteRule(ctx context.Context, tenantID uint64, paperID uint64, ruleID uint64) error {
	return r.deleteRule(r.db.WithContext(ctx), tenantID, paperID, ruleID)
}

func (r *PaperRepository) DeleteRuleAndRecalculate(ctx context.Context, tenantID uint64, paperID uint64, ruleID uint64, buildMode string) error {
	now := r.now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := r.deleteRule(tx, tenantID, paperID, ruleID); err != nil {
			return err
		}
		return r.recalculateRuleAggregates(tx, tenantID, paperID, buildMode, now)
	})
}

func (r *PaperRepository) deleteRule(tx *gorm.DB, tenantID uint64, paperID uint64, ruleID uint64) error {
	result := tx.
		Where(PaperSectionRuleColumns.TenantID+" = ?", tenantID).
		Where(PaperSectionRuleColumns.PaperID+" = ?", paperID).
		Where(PaperSectionRuleColumns.ID+" = ?", ruleID).
		Delete(&PaperSectionRuleDO{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return servicepaper.ErrPaperNotFound
	}
	return nil
}

func (r *PaperRepository) UpdateBuildMode(ctx context.Context, tenantID uint64, paperID uint64, buildMode string) error {
	now := r.now()
	result := r.db.WithContext(ctx).Model(&PaperDO{}).
		Where(PaperColumns.TenantID+" = ?", tenantID).
		Where(PaperColumns.ID+" = ?", paperID).
		Where(PaperColumns.DeletedAt+" = ?", 0).
		Updates(map[string]any{
			PaperColumns.BuildMode:    buildMode,
			BaseColumns.UpdatedAt:     now,
			BaseColumns.UpdatedByType: AuditActorTenantUser,
			BaseColumns.Version:       gorm.Expr(BaseColumns.Version + " + 1"),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return servicepaper.ErrPaperNotFound
	}
	return nil
}

func (r *PaperRepository) ListSectionQuestions(ctx context.Context, tenantID uint64, paperID uint64) ([]servicepaper.SectionQuestion, error) {
	var rows []PaperSectionQuestionDO
	if err := r.db.WithContext(ctx).
		Where(PaperSectionQuestionColumns.TenantID+" = ?", tenantID).
		Where(PaperSectionQuestionColumns.PaperID+" = ?", paperID).
		Order(PaperSectionQuestionColumns.SortOrder + " ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]servicepaper.SectionQuestion, 0, len(rows))
	for _, row := range rows {
		items = append(items, sectionQuestionFromDO(row))
	}
	return items, nil
}

func (r *PaperRepository) ListRuleLiveRules(ctx context.Context, tenantID uint64, paperID uint64) ([]servicepaper.Rule, error) {
	return r.ListRules(ctx, tenantID, paperID)
}

func (r *PaperRepository) SaveAggregates(ctx context.Context, tenantID uint64, paperID uint64, sections []servicepaper.SectionAggregate, paperTotalScore string) error {
	now := r.now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return r.saveAggregates(tx, tenantID, paperID, sections, paperTotalScore, now)
	})
}

func paperFromDO(row PaperDO) servicepaper.Paper {
	return servicepaper.Paper{
		ID:               row.ID,
		TenantID:         row.TenantID,
		SpaceID:          row.SpaceID,
		Name:             row.Name,
		Description:      row.Description,
		DurationMinutes:  row.DurationMinutes,
		GradeLevel:       row.GradeLevel,
		TotalScore:       row.TotalScore,
		BuildMode:        row.BuildMode,
		ShuffleQuestions: row.ShuffleQuestions,
		ShowAnalysis:     row.ShowAnalysis,
		Status:           row.Status,
		CreatedAt:        row.CreatedAt,
		CreatedBy:        row.CreatedBy,
	}
}

func paperFromListRow(row paperListRow) servicepaper.Paper {
	item := paperFromDO(row.PaperDO)
	item.CreatorName = row.CreatorName
	return item
}

func (r *PaperRepository) findPaperRow(ctx context.Context, tenantID uint64, paperID uint64) (paperListRow, error) {
	var row paperListRow
	err := r.db.WithContext(ctx).Table(PaperDO{}.TableName()+" AS papers").
		Select(r.paperSelectColumns()).
		Joins("LEFT JOIN users AS users ON users.id = papers."+BaseColumns.CreatedBy+" AND papers."+BaseColumns.CreatedByType+" = ? AND users.deleted_at = 0", AuditActorTenantUser).
		Where("papers."+PaperColumns.TenantID+" = ?", tenantID).
		Where("papers."+PaperColumns.ID+" = ?", paperID).
		Where("papers."+PaperColumns.DeletedAt+" = ?", 0).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return paperListRow{}, servicepaper.ErrPaperNotFound
	}
	if err != nil {
		return paperListRow{}, err
	}
	return row, nil
}

func (r *PaperRepository) paperSelectColumns() string {
	return "papers.*, users.username AS creator_name"
}

func sectionFromDO(row PaperSectionDO) servicepaper.Section {
	return servicepaper.Section{
		ID:            row.ID,
		TenantID:      row.TenantID,
		PaperID:       row.PaperID,
		SortOrder:     row.SortOrder,
		Name:          row.Name,
		QuestionType:  row.QuestionType,
		Instructions:  row.Instructions,
		TotalScore:    row.TotalScore,
		QuestionCount: row.QuestionCount,
	}
}

func sectionQuestionFromDO(row PaperSectionQuestionDO) servicepaper.SectionQuestion {
	return servicepaper.SectionQuestion{
		TenantID:       row.TenantID,
		SectionID:      row.SectionID,
		PaperID:        row.PaperID,
		QuestionID:     row.QuestionID,
		SortOrder:      row.SortOrder,
		Score:          row.Score,
		ShuffleOptions: row.ShuffleOptions,
	}
}

func ruleFromDO(row PaperSectionRuleDO) servicepaper.Rule {
	config := paperRuleConfigFromJSON(row.ExtJSON, row.TagFilter)
	return servicepaper.Rule{
		ID:                         row.ID,
		TenantID:                   row.TenantID,
		SectionID:                  row.SectionID,
		PaperID:                    row.PaperID,
		SortOrder:                  row.SortOrder,
		Difficulty:                 row.Difficulty,
		TagFilter:                  row.TagFilter,
		TagNames:                   config.TagNames,
		QuestionScope:              config.QuestionScope,
		DifficultyPercentages:      config.DifficultyPercentages,
		QuestionCount:              row.QuestionCount,
		ScorePerQuestion:           row.ScorePerQuestion,
		ShuffleOptions:             row.ShuffleOptions,
		PrioritizeQuality:          config.PrioritizeQuality,
		ExcludeRecentExamQuestions: config.ExcludeRecentExamQuestions,
		ExcludeUsedQuestions:       config.ExcludeUsedQuestions,
	}
}

func mustPaperRuleConfigJSON(rule servicepaper.Rule) []byte {
	data, _ := json.Marshal(paperRuleConfigJSON{
		QuestionScope:              rule.QuestionScope,
		TagNames:                   rule.TagNames,
		DifficultyPercentages:      rule.DifficultyPercentages,
		PrioritizeQuality:          rule.PrioritizeQuality,
		ExcludeRecentExamQuestions: rule.ExcludeRecentExamQuestions,
		ExcludeUsedQuestions:       rule.ExcludeUsedQuestions,
	})
	return data
}

func mustPaperRuleConfigJSONFromUpdate(input servicepaper.UpdateRuleInput) []byte {
	return mustPaperRuleConfigJSON(servicepaper.Rule{
		QuestionScope:              input.QuestionScope,
		TagNames:                   input.TagNames,
		DifficultyPercentages:      input.DifficultyPercentages,
		PrioritizeQuality:          input.PrioritizeQuality,
		ExcludeRecentExamQuestions: input.ExcludeRecentExamQuestions,
		ExcludeUsedQuestions:       input.ExcludeUsedQuestions,
	})
}

func paperRuleConfigFromJSON(raw datatypes.JSON, tagFilter string) paperRuleConfigJSON {
	config := paperRuleConfigJSON{QuestionScope: "space_all", ExcludeUsedQuestions: true}
	if len(tagIDsFromFilter(tagFilter)) > 0 {
		config.QuestionScope = "tag_filter"
	}
	if len(raw) == 0 {
		return config
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		return config
	}
	if config.QuestionScope == "" {
		config.QuestionScope = "space_all"
		if len(tagIDsFromFilter(tagFilter)) > 0 {
			config.QuestionScope = "tag_filter"
		}
	}
	return config
}

func (r *PaperRepository) recalculateQuestionAggregates(tx *gorm.DB, tenantID uint64, paperID uint64, now int64) error {
	if err := tx.Model(&PaperSectionDO{}).
		Where(PaperSectionColumns.TenantID+" = ?", tenantID).
		Where(PaperSectionColumns.PaperID+" = ?", paperID).
		Where(PaperSectionColumns.DeletedAt+" = ?", 0).
		Updates(map[string]any{
			PaperSectionColumns.TotalScore:    "0",
			PaperSectionColumns.QuestionCount: 0,
			BaseColumns.UpdatedAt:             now,
			BaseColumns.UpdatedByType:         AuditActorTenantUser,
			BaseColumns.Version:               gorm.Expr(BaseColumns.Version + " + 1"),
		}).Error; err != nil {
		return err
	}

	var aggregates []struct {
		SectionID     uint64
		TotalScore    float64
		QuestionCount int
	}
	if err := tx.Model(&PaperSectionQuestionDO{}).
		Select("section_id AS section_id, COALESCE(SUM(score), 0) AS total_score, COUNT(*) AS question_count").
		Where(PaperSectionQuestionColumns.TenantID+" = ?", tenantID).
		Where(PaperSectionQuestionColumns.PaperID+" = ?", paperID).
		Group(PaperSectionQuestionColumns.SectionID).
		Scan(&aggregates).Error; err != nil {
		return err
	}
	for _, aggregate := range aggregates {
		if err := tx.Model(&PaperSectionDO{}).
			Where(PaperSectionColumns.TenantID+" = ?", tenantID).
			Where(PaperSectionColumns.PaperID+" = ?", paperID).
			Where(PaperSectionColumns.ID+" = ?", aggregate.SectionID).
			Updates(map[string]any{
				PaperSectionColumns.TotalScore:    formatRepositoryScore(aggregate.TotalScore),
				PaperSectionColumns.QuestionCount: aggregate.QuestionCount,
				BaseColumns.UpdatedAt:             now,
				BaseColumns.UpdatedByType:         AuditActorTenantUser,
				BaseColumns.Version:               gorm.Expr(BaseColumns.Version + " + 1"),
			}).Error; err != nil {
			return err
		}
	}

	var paperTotal float64
	if err := tx.Model(&PaperSectionDO{}).
		Select("COALESCE(SUM(total_score), 0)").
		Where(PaperSectionColumns.TenantID+" = ?", tenantID).
		Where(PaperSectionColumns.PaperID+" = ?", paperID).
		Where(PaperSectionColumns.DeletedAt+" = ?", 0).
		Scan(&paperTotal).Error; err != nil {
		return err
	}
	return tx.Model(&PaperDO{}).
		Where(PaperColumns.TenantID+" = ?", tenantID).
		Where(PaperColumns.ID+" = ?", paperID).
		Where(PaperColumns.DeletedAt+" = ?", 0).
		Updates(map[string]any{
			PaperColumns.TotalScore:   formatRepositoryScore(paperTotal),
			BaseColumns.UpdatedAt:     now,
			BaseColumns.UpdatedByType: AuditActorTenantUser,
			BaseColumns.Version:       gorm.Expr(BaseColumns.Version + " + 1"),
		}).Error
}

func (r *PaperRepository) recalculateRuleAggregates(tx *gorm.DB, tenantID uint64, paperID uint64, buildMode string, now int64) error {
	if buildMode != servicepaper.BuildModeRuleLive {
		return nil
	}
	var aggregates []struct {
		SectionID     uint64
		TotalScore    float64
		QuestionCount int
	}
	if err := tx.Model(&PaperSectionRuleDO{}).
		Select("section_id AS section_id, COALESCE(SUM(question_count * score_per_question), 0) AS total_score, COALESCE(SUM(question_count), 0) AS question_count").
		Where(PaperSectionRuleColumns.TenantID+" = ?", tenantID).
		Where(PaperSectionRuleColumns.PaperID+" = ?", paperID).
		Group(PaperSectionRuleColumns.SectionID).
		Scan(&aggregates).Error; err != nil {
		return err
	}
	sections := make([]servicepaper.SectionAggregate, 0, len(aggregates))
	totalScore := 0.0
	for _, aggregate := range aggregates {
		totalScore += aggregate.TotalScore
		sections = append(sections, servicepaper.SectionAggregate{
			SectionID:     aggregate.SectionID,
			TotalScore:    formatRepositoryScore(aggregate.TotalScore),
			QuestionCount: aggregate.QuestionCount,
		})
	}
	return r.saveAggregates(tx, tenantID, paperID, sections, formatRepositoryScore(totalScore), now)
}

func (r *PaperRepository) saveAggregates(tx *gorm.DB, tenantID uint64, paperID uint64, sections []servicepaper.SectionAggregate, paperTotalScore string, now int64) error {
	if err := tx.Model(&PaperSectionDO{}).
		Where(PaperSectionColumns.TenantID+" = ?", tenantID).
		Where(PaperSectionColumns.PaperID+" = ?", paperID).
		Where(PaperSectionColumns.DeletedAt+" = ?", 0).
		Updates(map[string]any{
			PaperSectionColumns.TotalScore:    "0",
			PaperSectionColumns.QuestionCount: 0,
			BaseColumns.UpdatedAt:             now,
			BaseColumns.UpdatedByType:         AuditActorTenantUser,
			BaseColumns.Version:               gorm.Expr(BaseColumns.Version + " + 1"),
		}).Error; err != nil {
		return err
	}
	for _, section := range sections {
		result := tx.Model(&PaperSectionDO{}).
			Where(PaperSectionColumns.TenantID+" = ?", tenantID).
			Where(PaperSectionColumns.PaperID+" = ?", paperID).
			Where(PaperSectionColumns.ID+" = ?", section.SectionID).
			Updates(map[string]any{
				PaperSectionColumns.TotalScore:    section.TotalScore,
				PaperSectionColumns.QuestionCount: section.QuestionCount,
				BaseColumns.UpdatedAt:             now,
				BaseColumns.UpdatedByType:         AuditActorTenantUser,
				BaseColumns.Version:               gorm.Expr(BaseColumns.Version + " + 1"),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return servicepaper.ErrPaperNotFound
		}
	}
	result := tx.Model(&PaperDO{}).
		Where(PaperColumns.TenantID+" = ?", tenantID).
		Where(PaperColumns.ID+" = ?", paperID).
		Where(PaperColumns.DeletedAt+" = ?", 0).
		Updates(map[string]any{
			PaperColumns.TotalScore:   paperTotalScore,
			BaseColumns.UpdatedAt:     now,
			BaseColumns.UpdatedByType: AuditActorTenantUser,
			BaseColumns.Version:       gorm.Expr(BaseColumns.Version + " + 1"),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return servicepaper.ErrPaperNotFound
	}
	return nil
}

func tagIDsFromFilter(tagFilter string) []uint64 {
	var tagIDs []uint64
	if err := json.Unmarshal([]byte(tagFilter), &tagIDs); err != nil {
		return []uint64{}
	}
	return tagIDs
}

func mustTagFilterJSON(tagIDs []uint64) string {
	sorted := append([]uint64(nil), tagIDs...)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j] < sorted[i] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	data, _ := json.Marshal(sorted)
	return string(data)
}

func formatRepositoryScore(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

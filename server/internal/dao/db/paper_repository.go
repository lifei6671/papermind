package db

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	servicepaper "github.com/lifei6671/papermind/server/internal/service/paper"
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

func NewPaperRepository(gormDB *gorm.DB, options PaperRepositoryOptions) *PaperRepository {
	now := options.Now
	if now == nil {
		now = func() int64 { return time.Now().UnixMilli() }
	}
	return &PaperRepository{db: gormDB, now: now}
}

func (r *PaperRepository) ListPapers(ctx context.Context, tenantID uint64) ([]servicepaper.Paper, error) {
	var rows []PaperDO
	if err := r.db.WithContext(ctx).
		Where(PaperColumns.TenantID+" = ?", tenantID).
		Where(PaperColumns.DeletedAt+" = ?", 0).
		Order(PaperColumns.ID + " ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]servicepaper.Paper, 0, len(rows))
	for _, row := range rows {
		items = append(items, paperFromDO(row))
	}
	return items, nil
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
		for _, order := range orders {
			if err := tx.Model(&PaperSectionDO{}).
				Where(PaperSectionColumns.TenantID+" = ?", tenantID).
				Where(PaperSectionColumns.PaperID+" = ?", paperID).
				Where(PaperSectionColumns.ID+" = ?", order.SectionID).
				Where(PaperSectionColumns.DeletedAt+" = ?", 0).
				Update(PaperSectionColumns.SortOrder, order.SortOrder).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *PaperRepository) DeleteSectionCascade(ctx context.Context, tenantID uint64, sectionID uint64) error {
	now := r.now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&PaperSectionDO{}).
			Where(PaperSectionColumns.TenantID+" = ?", tenantID).
			Where(PaperSectionColumns.ID+" = ?", sectionID).
			Update(PaperSectionColumns.DeletedAt, now).Error; err != nil {
			return err
		}
		if err := tx.Where(PaperSectionQuestionColumns.TenantID+" = ?", tenantID).
			Where(PaperSectionQuestionColumns.SectionID+" = ?", sectionID).
			Delete(&PaperSectionQuestionDO{}).Error; err != nil {
			return err
		}
		return tx.Where(PaperSectionRuleColumns.TenantID+" = ?", tenantID).
			Where(PaperSectionRuleColumns.SectionID+" = ?", sectionID).
			Delete(&PaperSectionRuleDO{}).Error
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
			CreatedByType: AuditActorTenantUser,
			UpdatedAt:     now,
			UpdatedByType: AuditActorTenantUser,
			Version:       1,
			ExtJSON:       datatypes.JSON("{}"),
		},
		TenantID:         paper.TenantID,
		SpaceID:          paper.SpaceID,
		Name:             paper.Name,
		Description:      paper.Description,
		TotalScore:       paper.TotalScore,
		BuildMode:        paper.BuildMode,
		ShuffleQuestions: paper.ShuffleQuestions,
		ShowAnalysis:     paper.ShowAnalysis,
		Status:           paper.Status,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return servicepaper.Paper{}, err
	}
	return paperFromDO(row), nil
}

func (r *PaperRepository) DeletePaper(ctx context.Context, tenantID uint64, paperID uint64) error {
	now := r.now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var examCount int64
		if err := tx.Model(&ExamDO{}).
			Where(ExamColumns.TenantID+" = ?", tenantID).
			Where(ExamColumns.PaperID+" = ?", paperID).
			Where(ExamColumns.DeletedAt+" = ?", 0).
			Count(&examCount).Error; err != nil {
			return err
		}
		if examCount > 0 {
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

func (r *PaperRepository) CreateRule(ctx context.Context, rule servicepaper.Rule) (servicepaper.Rule, error) {
	now := r.now()
	row := PaperSectionRuleDO{
		BaseFields: BaseFields{
			CreatedAt:     now,
			CreatedByType: AuditActorTenantUser,
			UpdatedAt:     now,
			UpdatedByType: AuditActorTenantUser,
			Version:       1,
			ExtJSON:       datatypes.JSON("{}"),
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
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return servicepaper.Rule{}, err
	}
	return ruleFromDO(row), nil
}

func (r *PaperRepository) MatchQuestionsForRule(ctx context.Context, tenantID uint64, paperID uint64, rule servicepaper.Rule) ([]uint64, error) {
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
		Select("q."+QuestionColumns.ID).
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
			Group("q."+QuestionColumns.ID).
			Having("COUNT(DISTINCT qt."+QuestionTagColumns.TagID+") = ?", len(tagIDs))
	}

	var ids []uint64
	if err := query.Order("q." + QuestionColumns.ID + " ASC").Scan(&ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
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

func (r *PaperRepository) GenerateFixedQuestionsAndRecalculate(ctx context.Context, tenantID uint64, paperID uint64, questions []servicepaper.SectionQuestion) error {
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
		return r.recalculateQuestionAggregates(tx, tenantID, paperID, now)
	})
}

func (r *PaperRepository) ReplaceGeneratedQuestionAndRecalculate(ctx context.Context, input servicepaper.ReplaceGeneratedQuestionInput) error {
	return errors.New("rule_fixed review is not connected to HTTP yet")
}

func (r *PaperRepository) AdjustGeneratedQuestionAndRecalculate(ctx context.Context, input servicepaper.AdjustGeneratedQuestionInput) error {
	return errors.New("rule_fixed review is not connected to HTTP yet")
}

func (r *PaperRepository) FreezeLivePools(ctx context.Context, tenantID uint64, examID uint64, questionIDs []uint64) error {
	return errors.New("rule_live freeze is handled by exam repository")
}

func (r *PaperRepository) UpdateRule(ctx context.Context, input servicepaper.UpdateRuleInput) error {
	return errors.New("rule_live update is not connected to HTTP yet")
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
			if err := tx.Model(&PaperSectionDO{}).
				Where(PaperSectionColumns.TenantID+" = ?", tenantID).
				Where(PaperSectionColumns.PaperID+" = ?", paperID).
				Where(PaperSectionColumns.ID+" = ?", section.SectionID).
				Updates(map[string]any{
					PaperSectionColumns.TotalScore:    section.TotalScore,
					PaperSectionColumns.QuestionCount: section.QuestionCount,
					BaseColumns.UpdatedAt:             now,
					BaseColumns.UpdatedByType:         AuditActorTenantUser,
					BaseColumns.Version:               gorm.Expr(BaseColumns.Version + " + 1"),
				}).Error; err != nil {
				return err
			}
		}
		return tx.Model(&PaperDO{}).
			Where(PaperColumns.TenantID+" = ?", tenantID).
			Where(PaperColumns.ID+" = ?", paperID).
			Where(PaperColumns.DeletedAt+" = ?", 0).
			Updates(map[string]any{
				PaperColumns.TotalScore:   paperTotalScore,
				BaseColumns.UpdatedAt:     now,
				BaseColumns.UpdatedByType: AuditActorTenantUser,
				BaseColumns.Version:       gorm.Expr(BaseColumns.Version + " + 1"),
			}).Error
	})
}

func paperFromDO(row PaperDO) servicepaper.Paper {
	return servicepaper.Paper{
		ID:               row.ID,
		TenantID:         row.TenantID,
		SpaceID:          row.SpaceID,
		Name:             row.Name,
		Description:      row.Description,
		TotalScore:       row.TotalScore,
		BuildMode:        row.BuildMode,
		ShuffleQuestions: row.ShuffleQuestions,
		ShowAnalysis:     row.ShowAnalysis,
		Status:           row.Status,
	}
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
	return servicepaper.Rule{
		ID:               row.ID,
		TenantID:         row.TenantID,
		SectionID:        row.SectionID,
		PaperID:          row.PaperID,
		SortOrder:        row.SortOrder,
		Difficulty:       row.Difficulty,
		TagFilter:        row.TagFilter,
		QuestionCount:    row.QuestionCount,
		ScorePerQuestion: row.ScorePerQuestion,
		ShuffleOptions:   row.ShuffleOptions,
	}
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

func tagIDsFromFilter(tagFilter string) []uint64 {
	var tagIDs []uint64
	if err := json.Unmarshal([]byte(tagFilter), &tagIDs); err != nil {
		return []uint64{}
	}
	return tagIDs
}

func formatRepositoryScore(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

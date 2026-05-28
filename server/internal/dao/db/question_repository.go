package db

import (
	"context"
	"errors"
	"time"

	"github.com/lifei6671/papermind/server/internal/service/pagination"
	servicequestion "github.com/lifei6671/papermind/server/internal/service/question"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type QuestionRepository struct {
	db  *gorm.DB
	now func() int64
}

type QuestionRepositoryOptions struct {
	Now func() int64
}

func NewQuestionRepository(gormDB *gorm.DB, options QuestionRepositoryOptions) *QuestionRepository {
	now := options.Now
	if now == nil {
		now = func() int64 { return time.Now().UnixMilli() }
	}
	return &QuestionRepository{db: gormDB, now: now}
}

func (r *QuestionRepository) CreateQuestion(ctx context.Context, item servicequestion.Question, options []servicequestion.QuestionOption, tags []string) (servicequestion.Question, error) {
	var created servicequestion.Question
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := r.now()
		row := QuestionDO{
			BaseFields: BaseFields{
				CreatedAt:     now,
				CreatedByType: AuditActorTenantUser,
				UpdatedAt:     now,
				UpdatedByType: AuditActorTenantUser,
				Version:       1,
				ExtJSON:       datatypes.JSON("{}"),
			},
			TenantID:           item.TenantID,
			SpaceID:            item.SpaceID,
			Type:               item.Type,
			Difficulty:         item.Difficulty,
			Title:              item.Title,
			Analysis:           item.Analysis,
			ScoreDefault:       item.ScoreDefault,
			ChoiceDisplayCount: item.ChoiceDisplayCount,
			ShuffleOptions:     item.ShuffleOptions,
			Status:             item.Status,
		}
		if row.Difficulty == "" {
			row.Difficulty = servicequestion.DifficultyMedium
		}
		if row.ScoreDefault == "" {
			row.ScoreDefault = "0"
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		created = questionFromDO(row)

		createdOptions, err := r.createOptions(ctx, tx, row.TenantID, row.ID, options)
		if err != nil {
			return err
		}
		created.Options = createdOptions

		createdTags, err := r.bindTags(ctx, tx, row.TenantID, row.ID, tags)
		if err != nil {
			return err
		}
		created.Tags = createdTags
		return nil
	})
	if err != nil {
		return servicequestion.Question{}, err
	}
	return created, nil
}

func (r *QuestionRepository) ListVisibleQuestions(ctx context.Context, input servicequestion.ListQuestionsInput) (pagination.Result[servicequestion.Question], error) {
	page := pagination.Normalize(pagination.Input{Page: input.Page, PageSize: input.PageSize})
	var rows []QuestionDO
	query := r.db.WithContext(ctx).
		Model(&QuestionDO{}).
		Where(QuestionColumns.TenantID+" = ?", input.TenantID).
		Where(QuestionColumns.DeletedAt+" = ?", 0)
	if input.SpaceID == nil {
		query = query.Where(QuestionColumns.SpaceID + " IS NULL")
	} else {
		query = query.Where(r.db.Where(QuestionColumns.SpaceID+" IS NULL").Or(QuestionColumns.SpaceID+" = ?", *input.SpaceID))
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return pagination.Result[servicequestion.Question]{}, err
	}
	if err := query.Order(QuestionColumns.ID + " ASC").
		Limit(page.PageSize).
		Offset(pagination.Offset(page)).
		Find(&rows).Error; err != nil {
		return pagination.Result[servicequestion.Question]{}, err
	}
	questions := make([]servicequestion.Question, 0, len(rows))
	for _, row := range rows {
		item := questionFromDO(row)
		options, err := r.listOptions(ctx, row.TenantID, row.ID)
		if err != nil {
			return pagination.Result[servicequestion.Question]{}, err
		}
		tags, err := r.listTags(ctx, row.TenantID, row.ID)
		if err != nil {
			return pagination.Result[servicequestion.Question]{}, err
		}
		item.Options = options
		item.Tags = tags
		questions = append(questions, item)
	}
	return pagination.Result[servicequestion.Question]{
		Items:    questions,
		Page:     page.Page,
		PageSize: page.PageSize,
		Total:    total,
	}, nil
}

func (r *QuestionRepository) ReplaceOptionsInTransaction(ctx context.Context, tenantID uint64, questionID uint64, options []servicequestion.QuestionOption) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where(QuestionOptionColumns.TenantID+" = ?", tenantID).
			Where(QuestionOptionColumns.QuestionID+" = ?", questionID).
			Delete(&QuestionOptionDO{}).Error; err != nil {
			return err
		}
		_, err := r.createOptions(ctx, tx, tenantID, questionID, options)
		return err
	})
}

func (r *QuestionRepository) createOptions(ctx context.Context, tx *gorm.DB, tenantID uint64, questionID uint64, options []servicequestion.QuestionOption) ([]servicequestion.QuestionOption, error) {
	now := r.now()
	created := make([]servicequestion.QuestionOption, 0, len(options))
	for index, option := range options {
		row := QuestionOptionDO{
			BaseFields: BaseFields{
				CreatedAt:     now,
				CreatedByType: AuditActorTenantUser,
				UpdatedAt:     now,
				UpdatedByType: AuditActorTenantUser,
				Version:       1,
				ExtJSON:       datatypes.JSON("{}"),
			},
			TenantID:     tenantID,
			QuestionID:   questionID,
			OptionKey:    option.OptionKey,
			SortOrder:    index + 1,
			Content:      option.Content,
			IsCorrect:    option.IsCorrect,
			IsDistractor: option.IsDistractor,
		}
		if row.OptionKey == "" {
			row.OptionKey = optionKeyByIndex(index)
		}
		if err := tx.WithContext(ctx).Create(&row).Error; err != nil {
			return nil, err
		}
		created = append(created, optionFromDO(row))
	}
	return created, nil
}

func (r *QuestionRepository) bindTags(ctx context.Context, tx *gorm.DB, tenantID uint64, questionID uint64, tags []string) ([]string, error) {
	created := make([]string, 0, len(tags))
	for _, name := range tags {
		if name == "" {
			continue
		}
		tag, err := r.findOrCreateTag(ctx, tx, tenantID, name)
		if err != nil {
			return nil, err
		}
		relation := QuestionTagDO{
			RelationFields: RelationFields{
				CreatedAt:     r.now(),
				CreatedByType: AuditActorTenantUser,
				ExtJSON:       datatypes.JSON("{}"),
			},
			TenantID:   tenantID,
			QuestionID: questionID,
			TagID:      tag.ID,
		}
		if err := tx.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&relation).Error; err != nil {
			return nil, err
		}
		created = append(created, tag.Name)
	}
	return created, nil
}

func (r *QuestionRepository) findOrCreateTag(ctx context.Context, tx *gorm.DB, tenantID uint64, name string) (TagDO, error) {
	var tag TagDO
	err := tx.WithContext(ctx).
		Where(TagColumns.TenantID+" = ?", tenantID).
		Where(TagColumns.Name+" = ?", name).
		Where(TagColumns.DeletedAt+" = ?", 0).
		First(&tag).Error
	if err == nil {
		return tag, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return TagDO{}, err
	}
	now := r.now()
	tag = TagDO{
		BaseFields: BaseFields{
			CreatedAt:     now,
			CreatedByType: AuditActorTenantUser,
			UpdatedAt:     now,
			UpdatedByType: AuditActorTenantUser,
			Version:       1,
			ExtJSON:       datatypes.JSON("{}"),
		},
		TenantID: tenantID,
		Name:     name,
	}
	if err := tx.WithContext(ctx).Create(&tag).Error; err != nil {
		return TagDO{}, err
	}
	return tag, nil
}

func (r *QuestionRepository) listOptions(ctx context.Context, tenantID uint64, questionID uint64) ([]servicequestion.QuestionOption, error) {
	var rows []QuestionOptionDO
	if err := r.db.WithContext(ctx).
		Where(QuestionOptionColumns.TenantID+" = ?", tenantID).
		Where(QuestionOptionColumns.QuestionID+" = ?", questionID).
		Order(QuestionOptionColumns.SortOrder + " ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	options := make([]servicequestion.QuestionOption, 0, len(rows))
	for _, row := range rows {
		options = append(options, optionFromDO(row))
	}
	return options, nil
}

func (r *QuestionRepository) listTags(ctx context.Context, tenantID uint64, questionID uint64) ([]string, error) {
	var rows []struct {
		Name string `gorm:"column:name"`
	}
	err := r.db.WithContext(ctx).Table(QuestionTagDO{}.TableName()+" AS qt").
		Select("t.name").
		Joins("INNER JOIN tags AS t ON t.tenant_id = qt.tenant_id AND t.id = qt.tag_id AND t.deleted_at = 0").
		Where("qt.tenant_id = ?", tenantID).
		Where("qt.question_id = ?", questionID).
		Order("qt.id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	tags := make([]string, 0, len(rows))
	for _, row := range rows {
		tags = append(tags, row.Name)
	}
	return tags, nil
}

func questionFromDO(row QuestionDO) servicequestion.Question {
	return servicequestion.Question{
		ID:                 row.ID,
		TenantID:           row.TenantID,
		SpaceID:            row.SpaceID,
		Type:               row.Type,
		Difficulty:         row.Difficulty,
		Title:              row.Title,
		Analysis:           row.Analysis,
		ScoreDefault:       row.ScoreDefault,
		ChoiceDisplayCount: row.ChoiceDisplayCount,
		ShuffleOptions:     row.ShuffleOptions,
		Status:             row.Status,
	}
}

func optionFromDO(row QuestionOptionDO) servicequestion.QuestionOption {
	return servicequestion.QuestionOption{
		ID:           row.ID,
		OptionKey:    row.OptionKey,
		SortOrder:    row.SortOrder,
		Content:      row.Content,
		IsCorrect:    row.IsCorrect,
		IsDistractor: row.IsDistractor,
	}
}

func optionKeyByIndex(index int) string {
	if index >= 0 && index < 26 {
		return string(rune('A' + index))
	}
	return "Z"
}

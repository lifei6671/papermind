package db

import (
	"context"
	"errors"
	"strings"
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

type questionListRow struct {
	QuestionDO
	AuthorName string `gorm:"column:author_name"`
	AuthorRole string `gorm:"column:author_role"`
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
				CreatedBy:     item.CreatedBy,
				CreatedByType: AuditActorTenantUser,
				UpdatedAt:     now,
				UpdatedBy:     item.CreatedBy,
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
			StandardAnswer:     item.StandardAnswer,
			ReferenceAnswer:    item.ReferenceAnswer,
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
	return r.GetQuestion(ctx, created.TenantID, created.ID)
}

func (r *QuestionRepository) GetQuestion(ctx context.Context, tenantID uint64, questionID uint64) (servicequestion.Question, error) {
	row, err := r.findQuestionRow(ctx, tenantID, questionID)
	if err != nil {
		return servicequestion.Question{}, err
	}
	item := questionFromListRow(row)
	options, err := r.listOptions(ctx, tenantID, questionID)
	if err != nil {
		return servicequestion.Question{}, err
	}
	tags, err := r.listTags(ctx, tenantID, questionID)
	if err != nil {
		return servicequestion.Question{}, err
	}
	item.Options = options
	item.Tags = tags
	return item, nil
}

func (r *QuestionRepository) ListVisibleQuestions(ctx context.Context, input servicequestion.ListQuestionsInput) (pagination.Result[servicequestion.Question], error) {
	page := pagination.Normalize(pagination.Input{Page: input.Page, PageSize: input.PageSize})
	query := r.db.WithContext(ctx).
		Table(QuestionDO{}.TableName()+" AS questions").
		Joins("LEFT JOIN users AS users ON users.id = questions."+BaseColumns.CreatedBy+" AND questions."+BaseColumns.CreatedByType+" = ? AND users.deleted_at = 0", AuditActorTenantUser).
		Joins("LEFT JOIN tenant_user_memberships AS tum ON tum.tenant_id = questions.tenant_id AND tum.user_id = questions."+BaseColumns.CreatedBy).
		Where("questions."+QuestionColumns.TenantID+" = ?", input.TenantID).
		Where("questions."+QuestionColumns.DeletedAt+" = ?", 0)
	if input.SpaceID == nil {
		query = query.Where("questions." + QuestionColumns.SpaceID + " IS NULL")
	} else {
		query = query.Where(r.db.Where("questions."+QuestionColumns.SpaceID+" IS NULL").Or("questions."+QuestionColumns.SpaceID+" = ?", *input.SpaceID))
	}
	query = r.applyQuestionSearch(query, input.Search)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return pagination.Result[servicequestion.Question]{}, err
	}
	var rows []questionListRow
	if err := query.Select(r.questionSelectColumns()).
		Order("questions." + BaseColumns.CreatedAt + " DESC").
		Order("questions." + QuestionColumns.ID + " DESC").
		Limit(page.PageSize).
		Offset(pagination.Offset(page)).
		Scan(&rows).Error; err != nil {
		return pagination.Result[servicequestion.Question]{}, err
	}
	questions := make([]servicequestion.Question, 0, len(rows))
	for _, row := range rows {
		item := questionFromListRow(row)
		options, err := r.listOptions(ctx, row.QuestionDO.TenantID, row.QuestionDO.ID)
		if err != nil {
			return pagination.Result[servicequestion.Question]{}, err
		}
		tags, err := r.listTags(ctx, row.QuestionDO.TenantID, row.QuestionDO.ID)
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

func (r *QuestionRepository) applyQuestionSearch(query *gorm.DB, search string) *gorm.DB {
	keyword := strings.TrimSpace(search)
	if keyword == "" {
		return query
	}
	pattern := "%" + strings.ToLower(keyword) + "%"
	condition := r.db.Where("LOWER(questions."+QuestionColumns.Title+") LIKE ?", pattern).
		Or("LOWER(questions."+QuestionColumns.Analysis+") LIKE ?", pattern).
		Or("LOWER(questions."+QuestionColumns.Difficulty+") LIKE ?", pattern).
		Or("LOWER(questions."+QuestionColumns.Type+") LIKE ?", pattern).
		Or("LOWER(questions."+QuestionColumns.Status+") LIKE ?", pattern).
		Or("LOWER(users.username) LIKE ?", pattern).
		Or("LOWER(tum.role) LIKE ?", pattern).
		Or(
			`EXISTS (
					SELECT 1
					FROM question_options AS qo
					WHERE qo.tenant_id = questions.tenant_id
						AND qo.question_id = questions.id
						AND LOWER(qo.content) LIKE ?
				)`,
			pattern,
		).
		Or(
			`EXISTS (
					SELECT 1
					FROM question_tags AS qt
					JOIN tags AS tags ON tags.tenant_id = qt.tenant_id AND tags.id = qt.tag_id AND tags.deleted_at = 0
					WHERE qt.tenant_id = questions.tenant_id
						AND qt.question_id = questions.id
						AND LOWER(tags.name) LIKE ?
				)`,
			pattern,
		)
	return query.Where(condition)
}

func (r *QuestionRepository) UpdateQuestion(ctx context.Context, item servicequestion.Question, options []servicequestion.QuestionOption, tags []string) (servicequestion.Question, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{
			QuestionColumns.Type:               item.Type,
			QuestionColumns.Difficulty:         item.Difficulty,
			QuestionColumns.Title:              item.Title,
			QuestionColumns.Analysis:           item.Analysis,
			QuestionColumns.StandardAnswer:     item.StandardAnswer,
			QuestionColumns.ReferenceAnswer:    item.ReferenceAnswer,
			QuestionColumns.ScoreDefault:       item.ScoreDefault,
			QuestionColumns.ChoiceDisplayCount: item.ChoiceDisplayCount,
			QuestionColumns.ShuffleOptions:     item.ShuffleOptions,
			BaseColumns.UpdatedAt:              r.now(),
			BaseColumns.UpdatedBy:              item.CreatedBy,
			BaseColumns.UpdatedByType:          AuditActorTenantUser,
			BaseColumns.Version:                gorm.Expr(BaseColumns.Version + " + 1"),
		}
		result := tx.Model(&QuestionDO{}).
			Where(QuestionColumns.TenantID+" = ?", item.TenantID).
			Where(QuestionColumns.ID+" = ?", item.ID).
			Where(QuestionColumns.DeletedAt+" = ?", 0).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return servicequestion.ErrQuestionNotFound
		}
		if err := tx.Where(QuestionOptionColumns.TenantID+" = ?", item.TenantID).
			Where(QuestionOptionColumns.QuestionID+" = ?", item.ID).
			Delete(&QuestionOptionDO{}).Error; err != nil {
			return err
		}
		if _, err := r.createOptions(ctx, tx, item.TenantID, item.ID, options); err != nil {
			return err
		}
		if err := tx.Where(QuestionTagColumns.TenantID+" = ?", item.TenantID).
			Where(QuestionTagColumns.QuestionID+" = ?", item.ID).
			Delete(&QuestionTagDO{}).Error; err != nil {
			return err
		}
		_, err := r.bindTags(ctx, tx, item.TenantID, item.ID, tags)
		return err
	})
	if err != nil {
		return servicequestion.Question{}, err
	}
	return r.GetQuestion(ctx, item.TenantID, item.ID)
}

func (r *QuestionRepository) UpdateQuestionStatus(ctx context.Context, tenantID uint64, questionID uint64, status string, actorID uint64) (servicequestion.Question, error) {
	result := r.db.WithContext(ctx).Model(&QuestionDO{}).
		Where(QuestionColumns.TenantID+" = ?", tenantID).
		Where(QuestionColumns.ID+" = ?", questionID).
		Where(QuestionColumns.DeletedAt+" = ?", 0).
		Updates(map[string]any{
			QuestionColumns.Status:    status,
			BaseColumns.UpdatedAt:     r.now(),
			BaseColumns.UpdatedBy:     actorID,
			BaseColumns.UpdatedByType: AuditActorTenantUser,
			BaseColumns.Version:       gorm.Expr(BaseColumns.Version + " + 1"),
		})
	if result.Error != nil {
		return servicequestion.Question{}, result.Error
	}
	if result.RowsAffected == 0 {
		return servicequestion.Question{}, servicequestion.ErrQuestionNotFound
	}
	return r.GetQuestion(ctx, tenantID, questionID)
}

func (r *QuestionRepository) DeleteQuestion(ctx context.Context, tenantID uint64, questionID uint64, actorID uint64) error {
	result := r.db.WithContext(ctx).Model(&QuestionDO{}).
		Where(QuestionColumns.TenantID+" = ?", tenantID).
		Where(QuestionColumns.ID+" = ?", questionID).
		Where(QuestionColumns.DeletedAt+" = ?", 0).
		Updates(map[string]any{
			QuestionColumns.DeletedAt: r.now(),
			BaseColumns.UpdatedAt:     r.now(),
			BaseColumns.UpdatedBy:     actorID,
			BaseColumns.UpdatedByType: AuditActorTenantUser,
			BaseColumns.Version:       gorm.Expr(BaseColumns.Version + " + 1"),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return servicequestion.ErrQuestionNotFound
	}
	return nil
}

func (r *QuestionRepository) QuestionReferenced(ctx context.Context, tenantID uint64, questionID uint64) (bool, error) {
	tables := []string{"paper_section_questions", "exam_live_question_pools", "exam_attempt_questions"}
	for _, tableName := range tables {
		var count int64
		err := r.db.WithContext(ctx).Table(tableName).
			Where("tenant_id = ?", tenantID).
			Where("question_id = ?", questionID).
			Count(&count).Error
		if err != nil {
			return false, err
		}
		if count > 0 {
			return true, nil
		}
	}
	return false, nil
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
		StandardAnswer:     row.StandardAnswer,
		ReferenceAnswer:    row.ReferenceAnswer,
		ScoreDefault:       row.ScoreDefault,
		ChoiceDisplayCount: row.ChoiceDisplayCount,
		ShuffleOptions:     row.ShuffleOptions,
		Status:             row.Status,
		CreatedAt:          row.CreatedAt,
		CreatedBy:          row.CreatedBy,
	}
}

func questionFromListRow(row questionListRow) servicequestion.Question {
	item := questionFromDO(row.QuestionDO)
	item.AuthorName = row.AuthorName
	item.AuthorRole = row.AuthorRole
	return item
}

func (r *QuestionRepository) findQuestionRow(ctx context.Context, tenantID uint64, questionID uint64) (questionListRow, error) {
	var row questionListRow
	err := r.db.WithContext(ctx).Table(QuestionDO{}.TableName()+" AS questions").
		Select(r.questionSelectColumns()).
		Joins("LEFT JOIN users AS users ON users.id = questions."+BaseColumns.CreatedBy+" AND questions."+BaseColumns.CreatedByType+" = ? AND users.deleted_at = 0", AuditActorTenantUser).
		Joins("LEFT JOIN tenant_user_memberships AS tum ON tum.tenant_id = questions.tenant_id AND tum.user_id = questions."+BaseColumns.CreatedBy).
		Where("questions."+QuestionColumns.TenantID+" = ?", tenantID).
		Where("questions."+QuestionColumns.ID+" = ?", questionID).
		Where("questions."+QuestionColumns.DeletedAt+" = ?", 0).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return questionListRow{}, servicequestion.ErrQuestionNotFound
	}
	if err != nil {
		return questionListRow{}, err
	}
	return row, nil
}

func (r *QuestionRepository) questionSelectColumns() string {
	return "questions.*, users.username AS author_name, tum.role AS author_role"
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

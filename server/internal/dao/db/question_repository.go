package db

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/lifei6671/papermind/server/internal/service/pagination"
	servicequestion "github.com/lifei6671/papermind/server/internal/service/question"
	servicespace "github.com/lifei6671/papermind/server/internal/service/space"
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
			QualityScore:       item.QualityScore,
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
	query := r.visibleQuestionQuery(ctx, input.TenantID, input.SpaceID, input.Scope)
	query = r.applyQuestionSearch(query, input.Search)
	query = r.applyQuestionFilters(query, input)
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

func (r *QuestionRepository) ListVisibleQuestionTags(ctx context.Context, input servicequestion.QuestionTagListInput) ([]string, error) {
	query := r.visibleQuestionQuery(ctx, input.TenantID, input.SpaceID, input.Scope).
		Joins("JOIN question_tags AS qt ON qt.tenant_id = questions.tenant_id AND qt.question_id = questions.id").
		Joins("JOIN tags AS tags ON tags.tenant_id = qt.tenant_id AND tags.id = qt.tag_id AND tags.deleted_at = 0")
	query = r.applyQuestionFilters(query, servicequestion.ListQuestionsInput{Status: input.Status})
	if keyword := strings.TrimSpace(input.Search); keyword != "" {
		query = query.Where("LOWER(tags.name) LIKE ?", "%"+strings.ToLower(keyword)+"%")
	}
	var tags []string
	if err := query.Distinct("tags.name").Order("tags.name ASC").Pluck("tags.name", &tags).Error; err != nil {
		return nil, err
	}
	return tags, nil
}

func (r *QuestionRepository) CountVisibleQuestionsByType(ctx context.Context, input servicequestion.QuestionAvailabilityInput) (map[string]int64, error) {
	query := r.visibleQuestionQuery(ctx, input.TenantID, input.SpaceID, input.Scope)
	query = r.applyQuestionFilters(query, servicequestion.ListQuestionsInput{Status: input.Status})
	query = r.applyQuestionRequiredTags(query, input.Tags)
	if len(input.ExcludeQuestionIDs) > 0 {
		query = query.Where("questions."+QuestionColumns.ID+" NOT IN ?", input.ExcludeQuestionIDs)
	}
	var rows []struct {
		Type  string
		Count int64
	}
	if err := query.Select("questions." + QuestionColumns.Type + " AS type, COUNT(*) AS count").
		Group("questions." + QuestionColumns.Type).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	counts := make(map[string]int64, len(rows))
	for _, row := range rows {
		counts[row.Type] = row.Count
	}
	return counts, nil
}

func (r *QuestionRepository) visibleQuestionQuery(ctx context.Context, tenantID uint64, spaceID *uint64, scope string) *gorm.DB {
	query := r.db.WithContext(ctx).
		Table(QuestionDO{}.TableName()+" AS questions").
		Joins("LEFT JOIN users AS users ON users.id = questions."+BaseColumns.CreatedBy+" AND questions."+BaseColumns.CreatedByType+" = ? AND users.deleted_at = 0", AuditActorTenantUser).
		Joins("LEFT JOIN tenant_user_memberships AS tum ON tum.tenant_id = questions.tenant_id AND tum.user_id = questions."+BaseColumns.CreatedBy).
		Where("questions."+QuestionColumns.TenantID+" = ?", tenantID).
		Where("questions."+QuestionColumns.DeletedAt+" = ?", 0)
	query = r.applyEnabledQuestionSpace(query, "questions")
	if strings.TrimSpace(scope) == "public" {
		query = query.Where("questions." + QuestionColumns.SpaceID + " IS NULL")
	} else if spaceID != nil {
		query = query.Where(r.db.Where("questions."+QuestionColumns.SpaceID+" IS NULL").Or("questions."+QuestionColumns.SpaceID+" = ?", *spaceID))
	}
	return query
}

func (r *QuestionRepository) applyQuestionFilters(query *gorm.DB, input servicequestion.ListQuestionsInput) *gorm.DB {
	if questionType := strings.TrimSpace(input.Type); questionType != "" {
		query = query.Where("questions."+QuestionColumns.Type+" = ?", questionType)
	}
	if difficulty := strings.TrimSpace(input.Difficulty); difficulty != "" {
		query = query.Where("questions."+QuestionColumns.Difficulty+" = ?", difficulty)
	}
	if status := normalizeQuestionStatusFilter(input.Status); status != "" {
		query = query.Where("questions."+QuestionColumns.Status+" = ?", status)
	}
	if tag := strings.TrimSpace(input.Tag); tag != "" {
		query = query.Where(
			`EXISTS (
				SELECT 1
				FROM question_tags AS qt
				JOIN tags AS tags ON tags.tenant_id = qt.tenant_id and tags.id = qt.tag_id and tags.deleted_at = 0
				WHERE qt.tenant_id = questions.tenant_id
					and qt.question_id = questions.id
					and LOWER(tags.name) = ?
			)`,
			strings.ToLower(tag),
		)
	}
	return query
}

func (r *QuestionRepository) applyQuestionRequiredTags(query *gorm.DB, tags []string) *gorm.DB {
	for _, tag := range normalizeQuestionTagFilters(tags) {
		query = query.Where(
			`EXISTS (
				SELECT 1
				FROM question_tags AS qt
				JOIN tags AS tags ON tags.tenant_id = qt.tenant_id and tags.id = qt.tag_id and tags.deleted_at = 0
				WHERE qt.tenant_id = questions.tenant_id
					and qt.question_id = questions.id
					and LOWER(tags.name) = ?
			)`,
			strings.ToLower(tag),
		)
	}
	return query
}

func normalizeQuestionTagFilters(tags []string) []string {
	seen := make(map[string]struct{}, len(tags))
	result := make([]string, 0, len(tags))
	for _, tag := range tags {
		normalized := strings.TrimSpace(tag)
		if normalized == "" {
			continue
		}
		key := strings.ToLower(normalized)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func normalizeQuestionStatusFilter(status string) string {
	switch strings.TrimSpace(status) {
	case "":
		return ""
	case "ready":
		return servicequestion.QuestionStatusEnabled
	default:
		return strings.TrimSpace(status)
	}
}

func (r *QuestionRepository) QuestionTitleExists(ctx context.Context, tenantID uint64, spaceID *uint64, title string) (bool, error) {
	query := r.db.WithContext(ctx).
		Table(QuestionDO{}.TableName()).
		Where(QuestionColumns.TenantID+" = ?", tenantID).
		Where(QuestionColumns.DeletedAt+" = ?", 0).
		Where(QuestionColumns.Title+" = ?", title)
	query = r.applyEnabledQuestionSpace(query, QuestionDO{}.TableName())
	if spaceID == nil {
		query = query.Where(QuestionColumns.SpaceID + " IS NULL")
	} else {
		query = query.Where(r.db.Where(QuestionColumns.SpaceID+" IS NULL").Or(QuestionColumns.SpaceID+" = ?", *spaceID))
	}
	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *QuestionRepository) applyEnabledQuestionSpace(query *gorm.DB, questionTable string) *gorm.DB {
	spaceIDColumn := questionTable + "." + QuestionColumns.SpaceID
	tenantIDColumn := questionTable + "." + QuestionColumns.TenantID
	return query.Where(
		r.db.Where(spaceIDColumn+" IS NULL").Or(
			`EXISTS (
				SELECT 1
				FROM spaces
				WHERE spaces.tenant_id = `+tenantIDColumn+`
					and spaces.id = `+spaceIDColumn+`
					and spaces.status = ?
					and spaces.deleted_at = 0
			)`,
			servicespace.StatusEnabled,
		),
	)
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
			QuestionColumns.SpaceID:            item.SpaceID,
			QuestionColumns.Type:               item.Type,
			QuestionColumns.Difficulty:         item.Difficulty,
			QuestionColumns.Title:              item.Title,
			QuestionColumns.Analysis:           item.Analysis,
			QuestionColumns.StandardAnswer:     item.StandardAnswer,
			QuestionColumns.ReferenceAnswer:    item.ReferenceAnswer,
			QuestionColumns.ScoreDefault:       item.ScoreDefault,
			QuestionColumns.QualityScore:       item.QualityScore,
			QuestionColumns.ChoiceDisplayCount: item.ChoiceDisplayCount,
			QuestionColumns.ShuffleOptions:     item.ShuffleOptions,
			BaseColumns.UpdatedAt:              r.now(),
			BaseColumns.UpdatedBy:              item.CreatedBy,
			BaseColumns.UpdatedByType:          AuditActorTenantUser,
			BaseColumns.Version:                gorm.Expr(BaseColumns.Version + " + 1"),
		}
		query := tx.Model(&QuestionDO{}).
			Where(QuestionColumns.TenantID+" = ?", item.TenantID).
			Where(QuestionColumns.ID+" = ?", item.ID).
			Where(QuestionColumns.DeletedAt+" = ?", 0)
		query = r.applyEnabledQuestionSpace(query, QuestionDO{}.TableName())
		result := query.Updates(updates)
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
	query := r.db.WithContext(ctx).Model(&QuestionDO{}).
		Where(QuestionColumns.TenantID+" = ?", tenantID).
		Where(QuestionColumns.ID+" = ?", questionID).
		Where(QuestionColumns.DeletedAt+" = ?", 0)
	query = r.applyEnabledQuestionSpace(query, QuestionDO{}.TableName())
	result := query.Updates(map[string]any{
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
	query := r.db.WithContext(ctx).Model(&QuestionDO{}).
		Where(QuestionColumns.TenantID+" = ?", tenantID).
		Where(QuestionColumns.ID+" = ?", questionID).
		Where(QuestionColumns.DeletedAt+" = ?", 0)
	query = r.applyEnabledQuestionSpace(query, QuestionDO{}.TableName())
	result := query.Updates(map[string]any{
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
		QualityScore:       row.QualityScore,
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
	query := r.db.WithContext(ctx).Table(QuestionDO{}.TableName()+" AS questions").
		Select(r.questionSelectColumns()).
		Joins("LEFT JOIN users AS users ON users.id = questions."+BaseColumns.CreatedBy+" AND questions."+BaseColumns.CreatedByType+" = ? AND users.deleted_at = 0", AuditActorTenantUser).
		Joins("LEFT JOIN tenant_user_memberships AS tum ON tum.tenant_id = questions.tenant_id AND tum.user_id = questions."+BaseColumns.CreatedBy).
		Where("questions."+QuestionColumns.TenantID+" = ?", tenantID).
		Where("questions."+QuestionColumns.ID+" = ?", questionID).
		Where("questions."+QuestionColumns.DeletedAt+" = ?", 0)
	query = r.applyEnabledQuestionSpace(query, "questions")
	err := query.First(&row).Error
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

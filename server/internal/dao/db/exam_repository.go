package db

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	serviceexam "github.com/lifei6671/papermind/server/internal/service/exam"
	"github.com/lifei6671/papermind/server/internal/service/pagination"
	"github.com/lifei6671/papermind/server/library/constant"
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
		Where(PaperSectionColumns.QuestionType+" = ?", constant.QuestionTypeShortText).
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
		Where(ExamColumns.InviteCode+" = ?", code).
		Where(ExamColumns.DeletedAt+" = ?", 0).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *ExamRepository) ListRuleLiveCandidates(ctx context.Context, tenantID uint64, paperID uint64) ([]serviceexam.LivePoolItem, error) {
	var rules []PaperSectionRuleDO
	if err := r.db.WithContext(ctx).
		Where(PaperSectionRuleColumns.TenantID+" = ?", tenantID).
		Where(PaperSectionRuleColumns.PaperID+" = ?", paperID).
		Order(PaperSectionRuleColumns.SortOrder + " ASC").
		Find(&rules).Error; err != nil {
		return nil, err
	}
	items := make([]serviceexam.LivePoolItem, 0)
	usedQuestionIDs := make(map[uint64]struct{})
	for _, rule := range rules {
		questionIDs, err := r.matchRuleLiveQuestionIDs(ctx, tenantID, rule)
		if err != nil {
			return nil, err
		}
		selected := 0
		for _, questionID := range questionIDs {
			if _, exists := usedQuestionIDs[questionID]; exists {
				continue
			}
			items = append(items, serviceexam.LivePoolItem{
				SectionID:  rule.SectionID,
				RuleID:     rule.ID,
				QuestionID: questionID,
			})
			usedQuestionIDs[questionID] = struct{}{}
			selected++
			if selected == rule.QuestionCount {
				break
			}
		}
		if selected < rule.QuestionCount {
			return nil, serviceexam.ErrRuleLiveQuestionPoolInsufficient
		}
	}
	return items, nil
}

func (r *ExamRepository) matchRuleLiveQuestionIDs(ctx context.Context, tenantID uint64, rule PaperSectionRuleDO) ([]uint64, error) {
	var section PaperSectionDO
	if err := r.db.WithContext(ctx).
		Where(PaperSectionColumns.TenantID+" = ?", tenantID).
		Where(PaperSectionColumns.ID+" = ?", rule.SectionID).
		Where(PaperSectionColumns.PaperID+" = ?", rule.PaperID).
		Where(PaperSectionColumns.DeletedAt+" = ?", 0).
		First(&section).Error; err != nil {
		return nil, err
	}

	tagIDs := tagIDsFromFilter(rule.TagFilter)
	query := r.db.WithContext(ctx).Table(QuestionDO{}.TableName()+" AS questions").
		Select("questions."+QuestionColumns.ID).
		Where("questions."+QuestionColumns.TenantID+" = ?", tenantID).
		Where("questions."+QuestionColumns.Type+" = ?", section.QuestionType).
		Where("questions."+QuestionColumns.Status+" = ?", "enabled").
		Where("questions."+QuestionColumns.DeletedAt+" = ?", 0)
	if rule.Difficulty != nil {
		query = query.Where("questions."+QuestionColumns.Difficulty+" = ?", *rule.Difficulty)
	}
	if len(tagIDs) > 0 {
		query = query.Joins("JOIN "+QuestionTagDO{}.TableName()+" AS question_tags ON question_tags."+QuestionTagColumns.TenantID+" = questions."+QuestionColumns.TenantID+" AND question_tags."+QuestionTagColumns.QuestionID+" = questions."+QuestionColumns.ID).
			Where("question_tags."+QuestionTagColumns.TagID+" IN ?", tagIDs).
			Group("questions."+QuestionColumns.ID).
			Having("COUNT(DISTINCT question_tags."+QuestionTagColumns.TagID+") = ?", len(tagIDs))
	}
	var ids []uint64
	if err := query.Order("questions." + QuestionColumns.ID + " ASC").Scan(&ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
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
	var directCount int64
	if err := r.db.WithContext(ctx).Model(&ExamTargetDO{}).
		Joins("JOIN users ON users."+UserColumns.TenantID+" = exam_targets."+ExamTargetColumns.TenantID+
			" AND users."+UserColumns.ID+" = exam_targets."+ExamTargetColumns.TargetID+
			" AND users."+UserColumns.ID+" = ?"+
			" AND users."+UserColumns.Status+" = ?"+
			" AND users."+UserColumns.DeletedAt+" = ?", userID, "enabled", 0).
		Joins("JOIN user_roles ON user_roles."+UserRoleColumns.TenantID+" = users."+UserColumns.TenantID+
			" AND user_roles."+UserRoleColumns.UserID+" = users."+UserColumns.ID+
			" AND user_roles."+UserRoleColumns.Role+" = ?", "student").
		Where("exam_targets."+ExamTargetColumns.TenantID+" = ?", tenantID).
		Where("exam_targets."+ExamTargetColumns.ExamID+" = ?", examID).
		Where("exam_targets."+ExamTargetColumns.TargetType+" = ?", serviceexam.TargetTypeUser).
		Where("exam_targets."+ExamTargetColumns.TargetID+" = ?", userID).
		Count(&directCount).Error; err != nil {
		return false, err
	}
	if directCount > 0 {
		return true, nil
	}

	var spaceCount int64
	if err := r.db.WithContext(ctx).Model(&ExamTargetDO{}).
		Joins("JOIN space_members ON space_members."+SpaceMemberColumns.TenantID+" = exam_targets."+ExamTargetColumns.TenantID+
			" AND space_members."+SpaceMemberColumns.SpaceID+" = exam_targets."+ExamTargetColumns.TargetID+
			" AND space_members."+SpaceMemberColumns.UserID+" = ?"+
			" AND space_members."+SpaceMemberColumns.RoleInSpace+" = ?"+
			" AND space_members."+SpaceMemberColumns.Status+" = ?"+
			" AND space_members."+SpaceMemberColumns.DeletedAt+" = ?", userID, "student", "enabled", 0).
		Joins("JOIN users ON users."+UserColumns.TenantID+" = exam_targets."+ExamTargetColumns.TenantID+
			" AND users."+UserColumns.ID+" = space_members."+SpaceMemberColumns.UserID+
			" AND users."+UserColumns.Status+" = ?"+
			" AND users."+UserColumns.DeletedAt+" = ?", "enabled", 0).
		Where("exam_targets."+ExamTargetColumns.TenantID+" = ?", tenantID).
		Where("exam_targets."+ExamTargetColumns.ExamID+" = ?", examID).
		Where("exam_targets."+ExamTargetColumns.TargetType+" = ?", serviceexam.TargetTypeSpace).
		Count(&spaceCount).Error; err != nil {
		return false, err
	}
	return spaceCount > 0, nil
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

func (r *ExamRepository) UpdateAttemptToken(ctx context.Context, attempt serviceexam.Attempt) (serviceexam.Attempt, error) {
	now := r.now()
	if err := r.db.WithContext(ctx).Model(&ExamAttemptDO{}).
		Where(ExamAttemptColumns.TenantID+" = ?", attempt.TenantID).
		Where(ExamAttemptColumns.ID+" = ?", attempt.ID).
		Updates(map[string]any{
			ExamAttemptColumns.ExamTokenHash:      attempt.ExamTokenHash,
			ExamAttemptColumns.ExamTokenExpiresAt: attempt.ExamTokenExpiresAt,
			BaseColumns.UpdatedAt:                 now,
			BaseColumns.Version:                   gorm.Expr(BaseColumns.Version + " + 1"),
		}).Error; err != nil {
		return serviceexam.Attempt{}, err
	}
	return r.FindAttemptByTokenHash(ctx, attempt.ExamTokenHash)
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
	var rows []struct {
		SectionID    uint64
		SectionName  string
		Instructions string
		QuestionID   uint64
		QuestionType string
		Title        string
		Score        string
		SortOrder    int
	}
	if err := r.db.WithContext(ctx).
		Table("exams AS e").
		Select("s.id AS section_id, s.name AS section_name, s.instructions AS instructions, q.id AS question_id, q.type AS question_type, q.title AS title, psq.score AS score, psq.sort_order AS sort_order").
		Joins("JOIN paper_section_questions AS psq ON psq.tenant_id = e.tenant_id AND psq.paper_id = e.paper_id").
		Joins("JOIN paper_sections AS s ON s.tenant_id = psq.tenant_id AND s.id = psq.section_id AND s.deleted_at = 0").
		Joins("JOIN questions AS q ON q.tenant_id = psq.tenant_id AND q.id = psq.question_id AND q.deleted_at = 0").
		Where("e.tenant_id = ?", tenantID).
		Where("e.id = ?", examID).
		Order("s.sort_order ASC, psq.sort_order ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	items := make([]serviceexam.SnapshotSourceQuestion, 0, len(rows))
	for _, row := range rows {
		options, err := r.listSnapshotOptions(ctx, tenantID, row.QuestionID)
		if err != nil {
			return nil, err
		}
		items = append(items, serviceexam.SnapshotSourceQuestion{
			SectionID:        row.SectionID,
			SectionName:      row.SectionName,
			Instructions:     row.Instructions,
			QuestionID:       row.QuestionID,
			QuestionType:     row.QuestionType,
			Title:            row.Title,
			Score:            row.Score,
			Options:          options.options,
			OptionIDs:        options.optionIDs,
			CorrectOptionIDs: options.correctOptionIDs,
		})
	}
	return items, nil
}

func (r *ExamRepository) ListFrozenLiveSnapshotQuestions(ctx context.Context, tenantID uint64, examID uint64) ([]serviceexam.SnapshotSourceQuestion, error) {
	var rows []struct {
		SectionID    uint64
		SectionName  string
		Instructions string
		QuestionID   uint64
		QuestionType string
		Title        string
		Score        string
	}
	if err := r.db.WithContext(ctx).
		Table(ExamLiveQuestionPoolDO{}.TableName()+" AS pool").
		Select("sections.id AS section_id, sections.name AS section_name, sections.instructions AS instructions, questions.id AS question_id, questions.type AS question_type, questions.title AS title, rules.score_per_question AS score").
		Joins("JOIN "+PaperSectionRuleDO{}.TableName()+" AS rules ON rules.tenant_id = pool.tenant_id AND rules.id = pool.rule_id").
		Joins("JOIN "+PaperSectionDO{}.TableName()+" AS sections ON sections.tenant_id = pool.tenant_id AND sections.id = pool.section_id AND sections.deleted_at = 0").
		Joins("JOIN "+QuestionDO{}.TableName()+" AS questions ON questions.tenant_id = pool.tenant_id AND questions.id = pool.question_id AND questions.deleted_at = 0").
		Where("pool."+ExamLiveQuestionPoolColumns.TenantID+" = ?", tenantID).
		Where("pool."+ExamLiveQuestionPoolColumns.ExamID+" = ?", examID).
		Order("sections." + PaperSectionColumns.SortOrder + " ASC, rules." + PaperSectionRuleColumns.SortOrder + " ASC, pool." + RelationColumns.ID + " ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]serviceexam.SnapshotSourceQuestion, 0, len(rows))
	for _, row := range rows {
		options, err := r.listSnapshotOptions(ctx, tenantID, row.QuestionID)
		if err != nil {
			return nil, err
		}
		items = append(items, serviceexam.SnapshotSourceQuestion{
			SectionID:        row.SectionID,
			SectionName:      row.SectionName,
			Instructions:     row.Instructions,
			QuestionID:       row.QuestionID,
			QuestionType:     row.QuestionType,
			Title:            row.Title,
			Score:            row.Score,
			Options:          options.options,
			OptionIDs:        options.optionIDs,
			CorrectOptionIDs: options.correctOptionIDs,
		})
	}
	return items, nil
}

func (r *ExamRepository) SaveAttemptQuestions(ctx context.Context, questions []serviceexam.AttemptQuestion) ([]serviceexam.AttemptQuestion, error) {
	if len(questions) == 0 {
		return []serviceexam.AttemptQuestion{}, nil
	}
	existing, err := r.listAttemptQuestions(ctx, questions[0].TenantID, questions[0].AttemptID)
	if err != nil {
		return nil, err
	}
	if len(existing) > 0 {
		return existing, nil
	}
	now := r.now()
	rows := make([]ExamAttemptQuestionDO, 0, len(questions))
	for _, question := range questions {
		rows = append(rows, ExamAttemptQuestionDO{
			BaseFields: BaseFields{
				CreatedAt: now,
				UpdatedAt: now,
				Version:   1,
				ExtJSON:   datatypes.JSON([]byte("{}")),
			},
			TenantID:              question.TenantID,
			AttemptID:             question.AttemptID,
			SectionID:             question.SectionID,
			QuestionID:            question.QuestionID,
			SectionSnapshot:       datatypes.JSON([]byte(question.SectionSnapshot)),
			SortOrder:             question.SortOrder,
			Score:                 question.Score,
			QuestionSnapshot:      datatypes.JSON([]byte(question.QuestionSnapshot)),
			OptionSnapshot:        datatypes.JSON([]byte(question.OptionSnapshot)),
			CorrectAnswerSnapshot: datatypes.JSON([]byte(question.CorrectAnswerSnapshot)),
		})
	}
	if err := r.db.WithContext(ctx).Create(&rows).Error; err != nil {
		return nil, err
	}
	saved := append([]serviceexam.AttemptQuestion(nil), questions...)
	for index := range saved {
		saved[index].ID = rows[index].ID
	}
	return saved, nil
}

func (r *ExamRepository) ListPendingAttempts(ctx context.Context, tenantID uint64, examID uint64) ([]serviceexam.PendingAttempt, error) {
	var rows []pendingReviewRow
	if err := r.db.WithContext(ctx).Table("exam_answers AS answers").
		Select(`
			attempts.id AS attempt_id,
			attempt_questions.id AS attempt_question_id,
			attempts.exam_id AS exam_id,
			attempts.user_id AS user_id,
			COALESCE(users.real_name, users.username) AS student_name,
			COALESCE(spaces.id, 0) AS space_id,
			COALESCE(spaces.name, '') AS space_name,
			exams.name AS exam_name,
			attempt_questions.question_snapshot AS question_snapshot,
			answers.answer_content AS answer_content,
			attempt_questions.score AS max_score,
			answers.version AS answer_version,
			attempts.submitted_at AS submitted_at
		`).
		Joins("JOIN exam_attempt_questions AS attempt_questions ON attempt_questions.tenant_id = answers.tenant_id AND attempt_questions.attempt_id = answers.attempt_id AND attempt_questions.id = answers.attempt_question_id").
		Joins("JOIN exam_attempts AS attempts ON attempts.tenant_id = answers.tenant_id AND attempts.id = answers.attempt_id").
		Joins("JOIN exams ON exams.tenant_id = attempts.tenant_id AND exams.id = attempts.exam_id AND exams.deleted_at = 0").
		Joins("JOIN users ON users.tenant_id = attempts.tenant_id AND users.id = attempts.user_id AND users.deleted_at = 0").
		Joins("LEFT JOIN space_members AS members ON members.tenant_id = attempts.tenant_id AND members.user_id = attempts.user_id AND members.status = 'enabled' AND members.deleted_at = 0").
		Joins("LEFT JOIN spaces ON spaces.tenant_id = members.tenant_id AND spaces.id = members.space_id AND spaces.deleted_at = 0").
		Where("answers.tenant_id = ?", tenantID).
		Where("attempts.exam_id = ?", examID).
		Where("answers.grading_status = ?", constant.GradingStatusPending).
		Order("attempts.submitted_at ASC, attempt_questions.sort_order ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	countByAttempt := make(map[uint64]int, len(rows))
	for _, row := range rows {
		countByAttempt[row.AttemptID]++
	}
	items := make([]serviceexam.PendingAttempt, 0, len(rows))
	for _, row := range rows {
		title, err := parseQuestionSnapshotTitle(row.QuestionSnapshot)
		if err != nil {
			return nil, err
		}
		items = append(items, serviceexam.PendingAttempt{
			AttemptID:             row.AttemptID,
			AttemptQuestionID:     row.AttemptQuestionID,
			ExamID:                row.ExamID,
			UserID:                row.UserID,
			StudentName:           row.StudentName,
			SpaceID:               row.SpaceID,
			SpaceName:             row.SpaceName,
			ExamName:              row.ExamName,
			QuestionTitle:         title,
			AnswerContent:         row.AnswerContent,
			MaxScore:              row.MaxScore,
			AnswerVersion:         row.AnswerVersion,
			PendingShortTextCount: countByAttempt[row.AttemptID],
			SubmittedAt:           row.SubmittedAtValue(),
		})
	}
	return items, nil
}

func (r *ExamRepository) AttemptSpaceIDs(ctx context.Context, tenantID uint64, attemptID uint64) ([]uint64, error) {
	var rows []struct {
		SpaceID uint64
	}
	result := r.db.WithContext(ctx).Table("exam_attempts AS attempts").
		Distinct("COALESCE(members.space_id, 0) AS space_id").
		Joins("LEFT JOIN space_members AS members ON members.tenant_id = attempts.tenant_id AND members.user_id = attempts.user_id AND members.status = 'enabled' AND members.deleted_at = 0").
		Where("attempts.tenant_id = ?", tenantID).
		Where("attempts.id = ?", attemptID).
		Scan(&rows)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, serviceexam.ErrAttemptNotFound
	}
	spaces := make([]uint64, 0, len(rows))
	for _, row := range rows {
		spaces = append(spaces, row.SpaceID)
	}
	return spaces, nil
}

func (r *ExamRepository) GradeShortTextAndRecalculate(ctx context.Context, grade serviceexam.ShortTextGrade) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := grade.GradedAt
		// 简答题评分使用答案 version 乐观锁，避免两个教师同时覆盖同一题评分。
		result := tx.Model(&ExamAnswerDO{}).
			Where(ExamAnswerColumns.TenantID+" = ?", grade.TenantID).
			Where(ExamAnswerColumns.AttemptID+" = ?", grade.AttemptID).
			Where(ExamAnswerColumns.AttemptQuestionID+" = ?", grade.AttemptQuestionID).
			Where(BaseColumns.Version+" = ?", grade.AnswerVersion).
			Updates(map[string]any{
				ExamAnswerColumns.Score:         grade.Score,
				ExamAnswerColumns.GradingStatus: constant.GradingStatusGraded,
				ExamAnswerColumns.GradedBy:      grade.GradedBy,
				ExamAnswerColumns.GradedAt:      &now,
				ExamAnswerColumns.GraderComment: grade.Comment,
				BaseColumns.UpdatedAt:           now,
				BaseColumns.Version:             gorm.Expr(BaseColumns.Version + " + 1"),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return serviceexam.ErrAnswerVersionConflict
		}

		subjectiveScore, err := sumGradedSubjectiveScore(tx, grade.TenantID, grade.AttemptID)
		if err != nil {
			return err
		}
		var attempt ExamAttemptDO
		if err := tx.Where(ExamAttemptColumns.TenantID+" = ?", grade.TenantID).
			Where(ExamAttemptColumns.ID+" = ?", grade.AttemptID).
			First(&attempt).Error; err != nil {
			return err
		}
		objectiveScore, err := parseScoreString(attempt.ObjectiveScore)
		if err != nil {
			return err
		}
		return tx.Model(&ExamAttemptDO{}).
			Where(ExamAttemptColumns.TenantID+" = ?", grade.TenantID).
			Where(ExamAttemptColumns.ID+" = ?", grade.AttemptID).
			Updates(map[string]any{
				ExamAttemptColumns.SubjectiveScore: formatScoreString(subjectiveScore),
				ExamAttemptColumns.TotalScore:      formatScoreString(objectiveScore + subjectiveScore),
				BaseColumns.UpdatedAt:              now,
				BaseColumns.Version:                gorm.Expr(BaseColumns.Version + " + 1"),
			}).Error
	})
}

func (r *ExamRepository) ListScoreExportRows(ctx context.Context, tenantID uint64, examID uint64) ([]serviceexam.ScoreExportRow, error) {
	var rows []scoreExportRow
	if err := r.db.WithContext(ctx).Table("exam_attempts AS attempts").
		Select(`
			COALESCE(users.real_name, users.username) AS student_name,
			COALESCE(spaces.id, 0) AS space_id,
			COALESCE(spaces.name, '') AS space_name,
			attempts.attempt_no AS attempt_no,
			attempts.objective_score AS objective_score,
			attempts.subjective_score AS subjective_score,
			attempts.total_score AS total_score,
			attempts.submitted_at AS submitted_at
		`).
		Joins("JOIN users ON users.tenant_id = attempts.tenant_id AND users.id = attempts.user_id AND users.deleted_at = 0").
		Joins("LEFT JOIN space_members AS members ON members.tenant_id = attempts.tenant_id AND members.user_id = attempts.user_id AND members.status = 'enabled' AND members.deleted_at = 0").
		Joins("LEFT JOIN spaces ON spaces.tenant_id = members.tenant_id AND spaces.id = members.space_id AND spaces.deleted_at = 0").
		Where("attempts.tenant_id = ?", tenantID).
		Where("attempts.exam_id = ?", examID).
		Where("attempts.submitted_at IS NOT NULL").
		Order("attempts.submitted_at ASC, attempts.id ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]serviceexam.ScoreExportRow, 0, len(rows))
	for _, row := range rows {
		items = append(items, serviceexam.ScoreExportRow{
			StudentName:     row.StudentName,
			SpaceID:         row.SpaceID,
			SpaceName:       row.SpaceName,
			AttemptNo:       row.AttemptNo,
			ObjectiveScore:  row.ObjectiveScore,
			SubjectiveScore: row.SubjectiveScore,
			TotalScore:      row.TotalScore,
			SubmittedAt:     row.SubmittedAtValue(),
		})
	}
	return items, nil
}

func (r *ExamRepository) UpdateScorePublishConfig(ctx context.Context, tenantID uint64, examID uint64, publishMode string, scorePublishTime *int64) (serviceexam.Exam, error) {
	if err := r.db.WithContext(ctx).Model(&ExamDO{}).
		Where(ExamColumns.TenantID+" = ?", tenantID).
		Where(ExamColumns.ID+" = ?", examID).
		Where(ExamColumns.DeletedAt+" = ?", 0).
		Updates(map[string]any{
			ExamColumns.PublishMode:      publishMode,
			ExamColumns.ScorePublishTime: scorePublishTime,
			BaseColumns.UpdatedAt:        r.now(),
			BaseColumns.Version:          gorm.Expr(BaseColumns.Version + " + 1"),
		}).Error; err != nil {
		return serviceexam.Exam{}, err
	}
	return r.GetExam(ctx, tenantID, examID)
}

func (r *ExamRepository) listAttemptQuestions(ctx context.Context, tenantID uint64, attemptID uint64) ([]serviceexam.AttemptQuestion, error) {
	var rows []ExamAttemptQuestionDO
	if err := r.db.WithContext(ctx).
		Where(ExamAttemptQuestionColumns.TenantID+" = ?", tenantID).
		Where(ExamAttemptQuestionColumns.AttemptID+" = ?", attemptID).
		Order(ExamAttemptQuestionColumns.SortOrder + " ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]serviceexam.AttemptQuestion, 0, len(rows))
	for _, row := range rows {
		items = append(items, attemptQuestionFromDO(row))
	}
	return items, nil
}

func (r *ExamRepository) UpsertAnswer(ctx context.Context, answer serviceexam.Answer) error {
	now := r.now()
	row := ExamAnswerDO{
		BaseFields: BaseFields{
			CreatedAt: now,
			UpdatedAt: now,
			Version:   1,
			ExtJSON:   datatypes.JSON([]byte("{}")),
		},
		TenantID:          answer.TenantID,
		AttemptID:         answer.AttemptID,
		AttemptQuestionID: answer.AttemptQuestionID,
		AnswerContent:     answer.AnswerContent,
		GradingStatus:     serviceexam.GradingStatusPending,
	}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: ExamAnswerColumns.TenantID},
			{Name: ExamAnswerColumns.AttemptID},
			{Name: ExamAnswerColumns.AttemptQuestionID},
		},
		DoUpdates: clause.Assignments(map[string]any{
			ExamAnswerColumns.AnswerContent: answer.AnswerContent,
			BaseColumns.UpdatedAt:           now,
			BaseColumns.Version:             gorm.Expr(BaseColumns.Version + " + 1"),
		}),
	}).Create(&row).Error
}

func (r *ExamRepository) SubmitAttemptAndGradeObjectiveQuestions(ctx context.Context, tenantID uint64, attemptID uint64, version int64, submittedAt int64, status string, grader serviceexam.ObjectiveGradingFunc, event serviceexam.ExamEvent) (int64, error) {
	var affected int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&ExamAttemptDO{}).
			Where(ExamAttemptColumns.TenantID+" = ?", tenantID).
			Where(ExamAttemptColumns.ID+" = ?", attemptID).
			Where(BaseColumns.Version+" = ?", version).
			Where(ExamAttemptColumns.Status+" = ?", serviceexam.AttemptStatusInProgress).
			Updates(map[string]any{
				ExamAttemptColumns.Status:      status,
				ExamAttemptColumns.SubmittedAt: submittedAt,
				BaseColumns.UpdatedAt:          submittedAt,
				BaseColumns.Version:            gorm.Expr(BaseColumns.Version + " + 1"),
			})
		if result.Error != nil {
			return result.Error
		}
		affected = result.RowsAffected
		if affected == 0 {
			return nil
		}

		items, err := r.listAnswersForGrading(tx, tenantID, attemptID)
		if err != nil {
			return err
		}
		grades, objectiveScore, err := grader(items)
		if err != nil {
			return err
		}
		if err := r.saveAnswerGrades(tx, tenantID, attemptID, submittedAt, grades); err != nil {
			return err
		}
		if err := tx.Model(&ExamAttemptDO{}).
			Where(ExamAttemptColumns.TenantID+" = ?", tenantID).
			Where(ExamAttemptColumns.ID+" = ?", attemptID).
			Updates(map[string]any{
				ExamAttemptColumns.ObjectiveScore: objectiveScore,
				ExamAttemptColumns.TotalScore:     objectiveScore,
				BaseColumns.UpdatedAt:             submittedAt,
			}).Error; err != nil {
			return err
		}
		return appendEventWithDB(tx, event)
	})
	return affected, err
}

func (r *ExamRepository) AppendEvent(ctx context.Context, event serviceexam.ExamEvent) error {
	return appendEventWithDB(r.db.WithContext(ctx), event)
}

type snapshotOptions struct {
	options          []serviceexam.SnapshotSourceOption
	optionIDs        []uint64
	correctOptionIDs []uint64
}

func (r *ExamRepository) listSnapshotOptions(ctx context.Context, tenantID uint64, questionID uint64) (snapshotOptions, error) {
	var rows []QuestionOptionDO
	if err := r.db.WithContext(ctx).
		Where(QuestionOptionColumns.TenantID+" = ?", tenantID).
		Where(QuestionOptionColumns.QuestionID+" = ?", questionID).
		Order(QuestionOptionColumns.SortOrder + " ASC").
		Find(&rows).Error; err != nil {
		return snapshotOptions{}, err
	}
	result := snapshotOptions{
		options:          make([]serviceexam.SnapshotSourceOption, 0, len(rows)),
		optionIDs:        make([]uint64, 0, len(rows)),
		correctOptionIDs: make([]uint64, 0, len(rows)),
	}
	for _, row := range rows {
		result.options = append(result.options, serviceexam.SnapshotSourceOption{
			ID:      row.ID,
			Key:     row.OptionKey,
			Content: row.Content,
		})
		result.optionIDs = append(result.optionIDs, row.ID)
		if row.IsCorrect {
			result.correctOptionIDs = append(result.correctOptionIDs, row.ID)
		}
	}
	return result, nil
}

func (r *ExamRepository) listAnswersForGrading(tx *gorm.DB, tenantID uint64, attemptID uint64) ([]serviceexam.AnswerForGrading, error) {
	var questionRows []ExamAttemptQuestionDO
	if err := tx.
		Where(ExamAttemptQuestionColumns.TenantID+" = ?", tenantID).
		Where(ExamAttemptQuestionColumns.AttemptID+" = ?", attemptID).
		Order(ExamAttemptQuestionColumns.SortOrder + " ASC").
		Find(&questionRows).Error; err != nil {
		return nil, err
	}
	var answerRows []ExamAnswerDO
	if err := tx.
		Where(ExamAnswerColumns.TenantID+" = ?", tenantID).
		Where(ExamAnswerColumns.AttemptID+" = ?", attemptID).
		Find(&answerRows).Error; err != nil {
		return nil, err
	}
	answers := make(map[uint64]string, len(answerRows))
	for _, row := range answerRows {
		answers[row.AttemptQuestionID] = row.AnswerContent
	}
	items := make([]serviceexam.AnswerForGrading, 0, len(questionRows))
	for _, row := range questionRows {
		questionType, err := parseQuestionSnapshotType(row.QuestionSnapshot)
		if err != nil {
			return nil, err
		}
		items = append(items, serviceexam.AnswerForGrading{
			AttemptQuestionID:     row.ID,
			QuestionType:          questionType,
			QuestionScore:         row.Score,
			AnswerContent:         answers[row.ID],
			CorrectAnswerSnapshot: string(row.CorrectAnswerSnapshot),
		})
	}
	return items, nil
}

func (r *ExamRepository) saveAnswerGrades(tx *gorm.DB, tenantID uint64, attemptID uint64, now int64, grades []serviceexam.AnswerGradingResult) error {
	for _, grade := range grades {
		row := ExamAnswerDO{
			BaseFields: BaseFields{
				CreatedAt: now,
				UpdatedAt: now,
				Version:   1,
				ExtJSON:   datatypes.JSON([]byte("{}")),
			},
			TenantID:          tenantID,
			AttemptID:         attemptID,
			AttemptQuestionID: grade.AttemptQuestionID,
			Score:             grade.Score,
			GradingStatus:     grade.GradingStatus,
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: ExamAnswerColumns.TenantID},
				{Name: ExamAnswerColumns.AttemptID},
				{Name: ExamAnswerColumns.AttemptQuestionID},
			},
			DoUpdates: clause.Assignments(map[string]any{
				ExamAnswerColumns.Score:         grade.Score,
				ExamAnswerColumns.GradingStatus: grade.GradingStatus,
				BaseColumns.UpdatedAt:           now,
				BaseColumns.Version:             gorm.Expr(BaseColumns.Version + " + 1"),
			}),
		}).Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

func appendEventWithDB(tx *gorm.DB, event serviceexam.ExamEvent) error {
	payload := event.Payload
	if payload == "" {
		payload = "{}"
	}
	return tx.Create(&ExamEventDO{
		EventFields: EventFields{
			CreatedAt: event.EventTime,
			ExtJSON:   datatypes.JSON([]byte("{}")),
		},
		TenantID:  event.TenantID,
		AttemptID: event.AttemptID,
		EventType: event.EventType,
		EventTime: event.EventTime,
		Payload:   datatypes.JSON([]byte(payload)),
	}).Error
}

func parseQuestionSnapshotType(raw datatypes.JSON) (string, error) {
	var snapshot struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return "", err
	}
	return snapshot.Type, nil
}

func parseQuestionSnapshotTitle(raw string) (string, error) {
	var snapshot struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal([]byte(raw), &snapshot); err != nil {
		return "", err
	}
	return snapshot.Title, nil
}

func sumGradedSubjectiveScore(tx *gorm.DB, tenantID uint64, attemptID uint64) (float64, error) {
	var rows []ExamAnswerDO
	if err := tx.Where(ExamAnswerColumns.TenantID+" = ?", tenantID).
		Where(ExamAnswerColumns.AttemptID+" = ?", attemptID).
		Where(ExamAnswerColumns.GradingStatus+" = ?", constant.GradingStatusGraded).
		Find(&rows).Error; err != nil {
		return 0, err
	}
	var total float64
	for _, row := range rows {
		score, err := parseScoreString(row.Score)
		if err != nil {
			return 0, err
		}
		total += score
	}
	return total, nil
}

func parseScoreString(value string) (float64, error) {
	if strings.TrimSpace(value) == "" {
		return 0, nil
	}
	return strconv.ParseFloat(value, 64)
}

func formatScoreString(value float64) string {
	text := strconv.FormatFloat(value, 'f', 6, 64)
	text = strings.TrimRight(text, "0")
	text = strings.TrimRight(text, ".")
	if text == "" {
		return "0"
	}
	return text
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

func attemptQuestionFromDO(row ExamAttemptQuestionDO) serviceexam.AttemptQuestion {
	return serviceexam.AttemptQuestion{
		ID:                    row.ID,
		TenantID:              row.TenantID,
		AttemptID:             row.AttemptID,
		SectionID:             row.SectionID,
		QuestionID:            row.QuestionID,
		SectionSnapshot:       string(row.SectionSnapshot),
		SortOrder:             row.SortOrder,
		Score:                 row.Score,
		QuestionSnapshot:      string(row.QuestionSnapshot),
		OptionSnapshot:        string(row.OptionSnapshot),
		CorrectAnswerSnapshot: string(row.CorrectAnswerSnapshot),
	}
}

type pendingReviewRow struct {
	AttemptID         uint64
	AttemptQuestionID uint64
	ExamID            uint64
	UserID            uint64
	StudentName       string
	SpaceID           uint64
	SpaceName         string
	ExamName          string
	QuestionSnapshot  string
	AnswerContent     string
	MaxScore          string
	AnswerVersion     int64
	SubmittedAt       *int64
}

func (r pendingReviewRow) SubmittedAtValue() int64 {
	if r.SubmittedAt == nil {
		return 0
	}
	return *r.SubmittedAt
}

type scoreExportRow struct {
	StudentName     string
	SpaceID         uint64
	SpaceName       string
	AttemptNo       int
	ObjectiveScore  string
	SubjectiveScore string
	TotalScore      string
	SubmittedAt     *int64
}

func (r scoreExportRow) SubmittedAtValue() int64 {
	if r.SubmittedAt == nil {
		return 0
	}
	return *r.SubmittedAt
}

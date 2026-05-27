package db

import (
	"context"
	"encoding/json"
	"errors"
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
	return []serviceexam.SnapshotSourceQuestion{}, nil
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

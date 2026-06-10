package db

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mattn/go-sqlite3"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/plugin/soft_delete"

	serviceexam "github.com/lifei6671/papermind/server/internal/service/exam"
	"github.com/lifei6671/papermind/server/internal/service/pagination"
	servicespace "github.com/lifei6671/papermind/server/internal/service/space"
	servicetenantuser "github.com/lifei6671/papermind/server/internal/service/tenantuser"
	"github.com/lifei6671/papermind/server/library/constant"
)

// ExamRepository 负责考试模块的数据库持久化边界。
// 这里集中处理考试发布、考试目标、作答快照、答案、阅卷和成绩导出等数据读写，
// service 层只关心业务对象，具体表结构和多表关联都收敛在仓储层。
type ExamRepository struct {
	db  *gorm.DB
	now func() int64
}

var operationGroupSequence uint64

// ExamRepositoryOptions 提供仓储层可替换的运行时依赖。
// Now 主要用于测试中固定毫秒时间戳，避免考试发布、作答和评分用例受真实时间影响。
type ExamRepositoryOptions struct {
	Now func() int64
}

// NewExamRepository 创建考试仓储。
// 未传入 Now 时使用当前 Unix 毫秒时间，确保写入数据库的时间字段保持同一单位。
func NewExamRepository(gormDB *gorm.DB, options ExamRepositoryOptions) *ExamRepository {
	now := options.Now
	if now == nil {
		now = func() int64 { return time.Now().UnixMilli() }
	}
	return &ExamRepository{db: gormDB, now: now}
}

// ListExams 按租户分页列出未删除的考试。
// 查询先统计总数再读取当前页，返回值保留分页参数，方便 API 层直接构造列表响应。
func (r *ExamRepository) ListExams(ctx context.Context, input serviceexam.ListInput) (pagination.Result[serviceexam.Exam], error) {
	page := pagination.Normalize(pagination.Input{Page: input.Page, PageSize: input.PageSize})
	query := r.db.WithContext(ctx).Model(&ExamDO{}).
		Where(ExamColumns.TenantID+" = ?", input.TenantID).
		Where(ExamColumns.DeletedAt+" = ?", 0)
	if input.SpaceID != nil {
		query = query.Where(fmt.Sprintf(`
				EXISTS (
				SELECT 1 FROM exam_targets AS et
				WHERE et.tenant_id = exams.tenant_id
					and et.exam_id = exams.id
					and (
						(et.target_type = ? and et.target_id = ?)
						or (et.target_type = ? and EXISTS (
							SELECT 1 FROM space_members AS sm
							JOIN tenant_user_memberships AS tum
								ON tum.tenant_id = sm.tenant_id
								and tum.user_id = sm.user_id
								and tum.status = ?
							JOIN users AS u
								ON u.id = sm.user_id
								and u.status = ?
								and u.deleted_at = 0
							WHERE sm.tenant_id = et.tenant_id
								and sm.space_id = ?
								and sm.user_id = et.target_id
								and sm.status = ?
								and sm.deleted_at = 0
								and %s
						))
					)
			)
			`, r.userTargetScopedSpacePredicate("et", "sm.space_id")), serviceexam.TargetTypeSpace, *input.SpaceID, serviceexam.TargetTypeUser, servicetenantuser.StatusEnabled, servicetenantuser.StatusEnabled, *input.SpaceID, servicespace.StatusEnabled)
	}
	if input.PaperID != nil {
		query = query.Where(ExamColumns.PaperID+" = ?", *input.PaperID)
	}
	query = r.applyExamListSearch(query, input)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return pagination.Result[serviceexam.Exam]{}, err
	}
	var rows []ExamDO
	if err := query.
		Order(ExamColumns.ID + " DESC").
		Limit(page.PageSize).
		Offset(pagination.Offset(page)).
		Find(&rows).Error; err != nil {
		return pagination.Result[serviceexam.Exam]{}, err
	}
	paperNames, err := r.examPaperNames(ctx, input.TenantID, rows)
	if err != nil {
		return pagination.Result[serviceexam.Exam]{}, err
	}
	targetGroups := make(map[uint64][]serviceexam.Target, len(rows))
	if len(rows) > 0 {
		examIDs := make([]uint64, 0, len(rows))
		for _, row := range rows {
			examIDs = append(examIDs, row.ID)
		}
		var targetRows []ExamTargetDO
		targetQuery := r.db.WithContext(ctx).
			Where(ExamTargetColumns.TenantID+" = ?", input.TenantID).
			Where(ExamTargetColumns.ExamID+" IN ?", examIDs)
		if input.SpaceID != nil {
			targetQuery = targetQuery.Where(fmt.Sprintf(`
				(
					exam_targets.target_type = ? and exam_targets.target_id = ?
				)
				or (
					exam_targets.target_type = ? and EXISTS (
						SELECT 1 FROM space_members AS sm
						JOIN tenant_user_memberships AS tum
							ON tum.tenant_id = sm.tenant_id
							and tum.user_id = sm.user_id
							and tum.status = ?
						JOIN users AS u
							ON u.id = sm.user_id
							and u.status = ?
							and u.deleted_at = 0
						WHERE sm.tenant_id = exam_targets.tenant_id
							and sm.space_id = ?
							and sm.user_id = exam_targets.target_id
							and sm.status = ?
							and sm.deleted_at = 0
							and %s
					)
				)
			`, r.userTargetScopedSpacePredicate("exam_targets", "sm.space_id")),
				serviceexam.TargetTypeSpace, *input.SpaceID,
				serviceexam.TargetTypeUser, servicetenantuser.StatusEnabled, servicetenantuser.StatusEnabled, *input.SpaceID, servicespace.StatusEnabled,
			)
		}
		if err := targetQuery.
			Order(ExamTargetColumns.ExamID + " ASC").
			Order(ExamTargetColumns.ID + " ASC").
			Find(&targetRows).Error; err != nil {
			return pagination.Result[serviceexam.Exam]{}, err
		}
		for _, row := range targetRows {
			targetGroups[row.ExamID] = append(targetGroups[row.ExamID], serviceexam.Target{
				TenantID:   row.TenantID,
				ExamID:     row.ExamID,
				TargetType: row.TargetType,
				TargetID:   row.TargetID,
			})
		}
	}
	exams := make([]serviceexam.Exam, 0, len(rows))
	for _, row := range rows {
		exam := examFromDO(row, "")
		exam.PaperName = paperNames[row.PaperID]
		if targets, ok := targetGroups[row.ID]; ok {
			exam.Targets = append([]serviceexam.Target(nil), targets...)
			exam.TargetType = targets[0].TargetType
			exam.TargetID = targets[0].TargetID
		}
		exams = append(exams, exam)
	}
	return pagination.Result[serviceexam.Exam]{
		Items:    exams,
		Page:     page.Page,
		PageSize: page.PageSize,
		Total:    total,
	}, nil
}

type examPaperNameRow struct {
	ID   uint64
	Name string
}

func (r *ExamRepository) examPaperNames(ctx context.Context, tenantID uint64, exams []ExamDO) (map[uint64]string, error) {
	paperIDs := make([]uint64, 0, len(exams))
	for _, exam := range exams {
		paperIDs = append(paperIDs, exam.PaperID)
	}
	paperIDs = uniqueSortedOperationSpaceIDs(paperIDs)
	names := make(map[uint64]string, len(paperIDs))
	if len(paperIDs) == 0 {
		return names, nil
	}
	var rows []examPaperNameRow
	if err := r.db.WithContext(ctx).Model(&PaperDO{}).
		Select(PaperColumns.ID+", "+PaperColumns.Name).
		Where(PaperColumns.TenantID+" = ?", tenantID).
		Where(PaperColumns.ID+" IN ?", paperIDs).
		Where(PaperColumns.DeletedAt+" = ?", 0).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		names[row.ID] = row.Name
	}
	return names, nil
}

func (r *ExamRepository) applyExamListSearch(query *gorm.DB, input serviceexam.ListInput) *gorm.DB {
	keyword := strings.TrimSpace(input.Search)
	if keyword == "" {
		return query
	}
	pattern := likeIgnoreCasePattern(keyword)
	condition := LikeIgnoreCase(r.db, "exams."+ExamColumns.Name, keyword).
		Or(LikeIgnoreCase(r.db, "exams."+ExamColumns.InviteCode, keyword)).
		Or(`EXISTS (
			SELECT 1 FROM papers AS p
			WHERE p.tenant_id = exams.tenant_id
				and p.id = exams.paper_id
				and p.deleted_at = 0
				and `+likeIgnoreCaseClause(r.db, "p.name")+`
		)`, pattern)
	if input.SpaceID != nil {
		condition = condition.Or(`EXISTS (
			SELECT 1 FROM spaces AS visible_space
			WHERE visible_space.tenant_id = exams.tenant_id
				and visible_space.id = ?
				and visible_space.status = ?
				and visible_space.deleted_at = 0
				and `+likeIgnoreCaseClause(r.db, "visible_space.name")+`
		)`, *input.SpaceID, servicespace.StatusEnabled, pattern)
	} else {
		condition = condition.Or(`EXISTS (
			SELECT 1 FROM exam_targets AS et_search
			LEFT JOIN spaces AS target_space
				ON target_space.tenant_id = et_search.tenant_id
				and target_space.id = et_search.target_id
				and et_search.target_type = ?
				and target_space.status = ?
				and target_space.deleted_at = 0
			LEFT JOIN exam_target_scope_spaces AS etss_search
				ON etss_search.tenant_id = et_search.tenant_id
				and etss_search.exam_target_id = et_search.id
			LEFT JOIN spaces AS scoped_space
				ON scoped_space.tenant_id = etss_search.tenant_id
				and scoped_space.id = etss_search.space_id
				and scoped_space.status = ?
				and scoped_space.deleted_at = 0
			WHERE et_search.tenant_id = exams.tenant_id
				and et_search.exam_id = exams.id
				and (
					`+likeIgnoreCaseClause(r.db, "target_space.name")+`
					or `+likeIgnoreCaseClause(r.db, "scoped_space.name")+`
				)
		)`, serviceexam.TargetTypeSpace, servicespace.StatusEnabled, servicespace.StatusEnabled, pattern, pattern)
	}
	return query.Where(condition)
}

// CreateExam 创建考试草稿的基础记录。
// 未指定作答次数、成绩策略和成绩发布模式时写入业务默认值，保证后续发布流程有稳定初始状态。
func (r *ExamRepository) CreateExam(ctx context.Context, exam serviceexam.Exam) (serviceexam.Exam, error) {
	now := r.now()
	row := ExamDO{
		BaseFields: BaseFields{
			CreatedAt:     now,
			CreatedBy:     exam.CreatedBy,
			CreatedByType: AuditActorTenantUser,
			UpdatedAt:     now,
			UpdatedBy:     exam.CreatedBy,
			UpdatedByType: AuditActorTenantUser,
			Version:       1,
			ExtJSON:       datatypes.JSON("{}"),
		},
		TenantID:         exam.TenantID,
		PaperID:          exam.PaperID,
		Name:             exam.Name,
		StartTime:        exam.StartTime,
		EndTime:          exam.EndTime,
		DurationMinutes:  exam.DurationMinutes,
		MaxAttempts:      exam.MaxAttempts,
		ResultStrategy:   exam.ResultStrategy,
		PublishMode:      exam.PublishMode,
		ScorePublishTime: exam.ScorePublishTime,
		InviteCode:       exam.InviteCode,
		Status:           exam.Status,
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

// GetPaper 读取考试发布需要的试卷摘要。
// ContainsShortText 由小节题型实时统计，用于判断考试是否需要人工阅卷。
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
		Status:            row.Status,
		ContainsShortText: shortTextCount > 0,
	}, nil
}

// InviteCodeExists 检查邀请码是否已被未删除考试占用。
// 邀请码面向考生入口使用，因此按全局唯一处理，不把 tenantID 作为过滤条件。
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

// ListRuleLiveCandidates 按动态组卷规则生成发布时的候选题池。
// 每条规则按 sort_order 顺序抽题，同一次考试发布中同一道题只会被使用一次。
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
		// 动态组卷先拿到满足当前规则的全部题目，再在内存里排除前序规则已占用的题目。
		// 这样可以避免同一张试卷的多个规则抽中同一道题，保证冻结题池不重复。
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
			// 去重之后数量不足时直接失败，发布流程不会落入部分冻结的不可用状态。
			return nil, serviceexam.ErrRuleLiveQuestionPoolInsufficient
		}
	}
	return items, nil
}

// matchRuleLiveQuestionIDs 查出满足单条动态组卷规则的题目 ID。
// 题型来自规则所属小节，难度和标签来自规则本身，结果按题目 ID 稳定排序。
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
		// 标签过滤要求题目同时拥有规则指定的全部标签。
		// JOIN 后用 GROUP/HAVING 统计不同标签数量，避免 IN 查询退化为“命中任意标签”。
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

// PublishExamAndFreezeLivePool 发布考试并冻结动态组卷题池。
// 考试状态和冻结题目写在同一个事务内，防止考试已发布但题池缺失或只写入一部分。
func (r *ExamRepository) PublishExamAndFreezeLivePool(ctx context.Context, exam serviceexam.Exam, pool []serviceexam.LivePoolItem, expectedStatus string, log *serviceexam.OperationLog) (serviceexam.Exam, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 发布时写入所有会影响考生可见性和成绩发布的配置，并递增考试版本。
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
			BaseColumns.UpdatedByType:    AuditActorTenantUser,
			BaseColumns.Version:          gorm.Expr(BaseColumns.Version + " + 1"),
		}
		query := tx.Model(&ExamDO{}).
			Where(ExamColumns.TenantID+" = ?", exam.TenantID).
			Where(ExamColumns.ID+" = ?", exam.ID).
			Where(ExamColumns.DeletedAt+" = ?", 0)
		if expectedStatus != "" {
			query = query.Where(ExamColumns.Status+" = ?", expectedStatus)
		}
		result := query.Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return r.examStatusPreconditionError(tx, exam.TenantID, exam.ID, expectedStatus)
		}
		for _, item := range pool {
			// 动态组卷结果在发布瞬间固化到 exam_live_question_pools。
			// 后续题库内容或规则变更不会改变已经发布考试的出题范围。
			row := ExamLiveQuestionPoolDO{
				RelationFields: RelationFields{
					CreatedAt:     r.now(),
					CreatedByType: AuditActorTenantUser,
					ExtJSON:       datatypes.JSON([]byte("{}")),
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
		if log != nil {
			publishLog := *log
			publishLog.TenantID = exam.TenantID
			publishLog.ExamID = exam.ID
			targets, err := r.listTargetsInDB(tx, exam.TenantID, exam.ID)
			if err != nil {
				return err
			}
			spaceIDs, err := r.operationTargetSpaceIDsInTx(tx, exam.TenantID, targets)
			if err != nil {
				return err
			}
			if err := r.appendOperationLogsForSpacesInTx(tx, publishLog, spaceIDs); err != nil {
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

// CreatePublishedExamWithTarget 创建已发布考试，并在同一个事务内写入动态题池和首个投放目标。
// API 的“发布考试”入口使用该方法，避免校验或目标写入失败后留下草稿或无目标考试。
func (r *ExamRepository) CreatePublishedExamWithTarget(ctx context.Context, exam serviceexam.Exam, pool []serviceexam.LivePoolItem, target serviceexam.Target, log *serviceexam.OperationLog) (serviceexam.Exam, error) {
	return r.CreatePublishedExamWithTargets(ctx, exam, pool, []serviceexam.Target{target}, log)
}

// CreatePublishedExamWithTargets 创建已发布考试，并在同一个事务内写入动态题池和多个投放目标。
// 任一目标或题池写入失败都会回滚整场考试，避免出现“已发布但没有完整投放范围”的中间状态。
func (r *ExamRepository) CreatePublishedExamWithTargets(ctx context.Context, exam serviceexam.Exam, pool []serviceexam.LivePoolItem, targets []serviceexam.Target, log *serviceexam.OperationLog) (serviceexam.Exam, error) {
	var createdID uint64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := r.now()
		row := ExamDO{
			BaseFields: BaseFields{
				CreatedAt:     now,
				CreatedBy:     exam.CreatedBy,
				CreatedByType: AuditActorTenantUser,
				UpdatedAt:     now,
				UpdatedBy:     exam.CreatedBy,
				UpdatedByType: AuditActorTenantUser,
				Version:       1,
				ExtJSON:       datatypes.JSON([]byte("{}")),
			},
			TenantID:         exam.TenantID,
			PaperID:          exam.PaperID,
			Name:             exam.Name,
			StartTime:        exam.StartTime,
			EndTime:          exam.EndTime,
			DurationMinutes:  exam.DurationMinutes,
			MaxAttempts:      exam.MaxAttempts,
			ResultStrategy:   exam.ResultStrategy,
			PublishMode:      exam.PublishMode,
			ScorePublishTime: exam.ScorePublishTime,
			InviteCode:       exam.InviteCode,
			Status:           exam.Status,
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		createdID = row.ID
		for _, item := range pool {
			poolRow := ExamLiveQuestionPoolDO{
				RelationFields: RelationFields{
					CreatedAt:     now,
					CreatedByType: AuditActorTenantUser,
					ExtJSON:       datatypes.JSON([]byte("{}")),
				},
				TenantID:   exam.TenantID,
				ExamID:     row.ID,
				SectionID:  item.SectionID,
				RuleID:     item.RuleID,
				QuestionID: item.QuestionID,
			}
			if err := tx.Create(&poolRow).Error; err != nil {
				return err
			}
		}
		for _, target := range targets {
			targetRow := ExamTargetDO{
				RelationFields: RelationFields{
					CreatedAt:     now,
					CreatedByType: AuditActorTenantUser,
					ExtJSON:       datatypes.JSON([]byte("{}")),
				},
				TenantID:   target.TenantID,
				ExamID:     row.ID,
				TargetType: target.TargetType,
				TargetID:   target.TargetID,
			}
			if err := tx.Create(&targetRow).Error; err != nil {
				return err
			}
			if err := r.createTargetScopeSpacesInTx(tx, targetRow, target, now); err != nil {
				return err
			}
		}
		if log != nil {
			publishLog := *log
			publishLog.TenantID = exam.TenantID
			publishLog.ExamID = row.ID
			spaceIDs, err := r.operationTargetSpaceIDsInTx(tx, exam.TenantID, targets)
			if err != nil {
				return err
			}
			// 发布日志必须和考试、冻结题池、投放目标在同一事务提交，避免管理端审计缺失。
			if err := r.appendOperationLogsForSpacesInTx(tx, publishLog, spaceIDs); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return serviceexam.Exam{}, err
	}
	return r.GetExam(ctx, exam.TenantID, createdID)
}

// UpdateDraftWithTargets 更新草稿考试配置，并重建投放目标。
// 该方法只允许更新 draft 状态，避免已发布考试的公平性字段被绕过修改。
func (r *ExamRepository) UpdateDraftWithTargets(ctx context.Context, input serviceexam.UpdateDraftWithTargetsRepositoryInput) (serviceexam.Exam, error) {
	exam := input.Exam
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := r.now()
		result := tx.Model(&ExamDO{}).
			Where(ExamColumns.TenantID+" = ?", exam.TenantID).
			Where(ExamColumns.ID+" = ?", exam.ID).
			Where(ExamColumns.Status+" = ?", serviceexam.StatusDraft).
			Where(ExamColumns.DeletedAt+" = ?", 0).
			Updates(map[string]any{
				ExamColumns.PaperID:          exam.PaperID,
				ExamColumns.Name:             exam.Name,
				ExamColumns.StartTime:        exam.StartTime,
				ExamColumns.EndTime:          exam.EndTime,
				ExamColumns.DurationMinutes:  exam.DurationMinutes,
				ExamColumns.MaxAttempts:      exam.MaxAttempts,
				ExamColumns.ResultStrategy:   exam.ResultStrategy,
				ExamColumns.PublishMode:      exam.PublishMode,
				ExamColumns.ScorePublishTime: exam.ScorePublishTime,
				BaseColumns.UpdatedAt:        now,
				BaseColumns.UpdatedByType:    AuditActorTenantUser,
				BaseColumns.Version:          gorm.Expr(BaseColumns.Version + " + 1"),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return r.examStatusPreconditionError(tx, exam.TenantID, exam.ID, serviceexam.StatusDraft)
		}
		if err := tx.Where(ExamTargetScopeSpaceColumns.TenantID+" = ?", exam.TenantID).
			Where(ExamTargetScopeSpaceColumns.ExamID+" = ?", exam.ID).
			Delete(&ExamTargetScopeSpaceDO{}).Error; err != nil {
			return err
		}
		if err := tx.Where(ExamTargetColumns.TenantID+" = ?", exam.TenantID).
			Where(ExamTargetColumns.ExamID+" = ?", exam.ID).
			Delete(&ExamTargetDO{}).Error; err != nil {
			return err
		}
		for _, target := range input.Targets {
			targetRow := ExamTargetDO{
				RelationFields: RelationFields{
					CreatedAt:     now,
					CreatedByType: AuditActorTenantUser,
					ExtJSON:       datatypes.JSON([]byte("{}")),
				},
				TenantID:   exam.TenantID,
				ExamID:     exam.ID,
				TargetType: target.TargetType,
				TargetID:   target.TargetID,
			}
			if err := tx.Create(&targetRow).Error; err != nil {
				return err
			}
			if err := r.createTargetScopeSpacesInTx(tx, targetRow, target, now); err != nil {
				return err
			}
		}
		if input.Log != nil {
			log := *input.Log
			log.TenantID = exam.TenantID
			log.ExamID = exam.ID
			spaceIDs, err := r.operationTargetSpaceIDsInTx(tx, exam.TenantID, input.Targets)
			if err != nil {
				return err
			}
			return r.appendOperationLogsForSpacesInTx(tx, log, spaceIDs)
		}
		return nil
	})
	if err != nil {
		return serviceexam.Exam{}, err
	}
	return r.GetExam(ctx, exam.TenantID, exam.ID)
}

// TargetExists 判断考试目标是否已经存在。
// 发布流程用它避免重复添加同一个用户或空间目标。
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

// AddTarget 为考试添加一个投放目标。
// targetType 区分直投用户和投放空间，targetID 的含义由 targetType 决定。
func (r *ExamRepository) AddTarget(ctx context.Context, target serviceexam.Target) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := r.now()
		row := ExamTargetDO{
			RelationFields: RelationFields{
				CreatedAt:     now,
				CreatedByType: AuditActorTenantUser,
				ExtJSON:       datatypes.JSON([]byte("{}")),
			},
			TenantID:   target.TenantID,
			ExamID:     target.ExamID,
			TargetType: target.TargetType,
			TargetID:   target.TargetID,
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		return r.createTargetScopeSpacesInTx(tx, row, target, now)
	})
}

// ImportCandidateTargets 批量把授权范围内的启用学生追加为用户直投目标。
// 目标写入和操作日志写入必须处于同一事务，避免导入成功但审计日志缺失。
func (r *ExamRepository) ImportCandidateTargets(ctx context.Context, input serviceexam.ImportCandidateTargetsInput) (serviceexam.ImportCandidateTargetsResult, error) {
	result := serviceexam.ImportCandidateTargetsResult{SkippedCount: len(input.UserIDs)}
	if len(input.UserIDs) == 0 {
		return result, nil
	}
	if len(input.AllowedSpaceIDs) == 0 {
		return result, nil
	}

	validUserIDs, err := r.validImportCandidateUserIDs(ctx, input.TenantID, input.UserIDs, input.AllowedSpaceIDs)
	if err != nil {
		return serviceexam.ImportCandidateTargetsResult{}, err
	}
	if len(validUserIDs) == 0 {
		return result, nil
	}
	existingUserIDs, err := r.existingExamUserTargetIDs(ctx, input.TenantID, input.ExamID, validUserIDs)
	if err != nil {
		return serviceexam.ImportCandidateTargetsResult{}, err
	}

	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := r.now()
		validSet := uint64Set(validUserIDs)
		existingSet := uint64Set(existingUserIDs)
		imported := make([]serviceexam.Target, 0, len(validUserIDs))
		for _, userID := range input.UserIDs {
			if _, ok := validSet[userID]; !ok {
				continue
			}
			if _, ok := existingSet[userID]; ok {
				continue
			}
			scopeSpaceIDs, err := r.operationUserSpaceIDsInTx(tx, input.TenantID, []uint64{userID}, input.AllowedSpaceIDs)
			if err != nil {
				return err
			}
			row := ExamTargetDO{
				RelationFields: RelationFields{
					CreatedAt:     now,
					CreatedByType: AuditActorTenantUser,
					ExtJSON:       datatypes.JSON([]byte("{}")),
				},
				TenantID:   input.TenantID,
				ExamID:     input.ExamID,
				TargetType: serviceexam.TargetTypeUser,
				TargetID:   userID,
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
			target := serviceexam.Target{
				TenantID:      input.TenantID,
				ExamID:        input.ExamID,
				TargetType:    serviceexam.TargetTypeUser,
				TargetID:      userID,
				ScopeSpaceIDs: scopeSpaceIDs,
			}
			if err := r.createTargetScopeSpacesInTx(tx, row, target, now); err != nil {
				return err
			}
			existingSet[userID] = struct{}{}
			imported = append(imported, target)
		}
		importedUserIDs := targetUserIDs(imported)
		result.ImportedTargets = imported
		result.ImportedCount = len(imported)
		result.SkippedCount = len(input.UserIDs) - result.ImportedCount
		if result.ImportedCount == 0 {
			return nil
		}
		spaceIDs, err := r.operationUserSpaceIDsInTx(tx, input.TenantID, importedUserIDs, input.AllowedSpaceIDs)
		if err != nil {
			return err
		}
		if err := r.appendOperationLogsForSpacesInTx(tx, input.Log, spaceIDs); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return serviceexam.ImportCandidateTargetsResult{}, err
	}
	return result, nil
}

// validImportCandidateUserIDs 返回既在授权空间内、又是当前租户启用学生的用户 ID。
// 查询只返回去重 ID，实际插入顺序仍按调用方传入顺序处理，保证结果稳定可解释。
func (r *ExamRepository) validImportCandidateUserIDs(ctx context.Context, tenantID uint64, userIDs []uint64, allowedSpaceIDs []uint64) ([]uint64, error) {
	var ids []uint64
	if err := r.db.WithContext(ctx).Table("space_members AS members").
		Select("DISTINCT members.user_id").
		Joins("JOIN spaces ON spaces.tenant_id = members.tenant_id AND spaces.id = members.space_id AND spaces.status = ? AND spaces.deleted_at = 0", servicespace.StatusEnabled).
		Joins("JOIN users ON users.id = members.user_id AND users.status = ? AND users.deleted_at = 0", servicetenantuser.StatusEnabled).
		Joins("JOIN tenant_user_memberships AS tum ON tum.tenant_id = members.tenant_id AND tum.user_id = members.user_id AND tum.role = ? AND tum.status = ?", constant.RoleStudent, servicetenantuser.StatusEnabled).
		Where("members.tenant_id = ?", tenantID).
		Where("members.user_id IN ?", userIDs).
		Where("members.space_id IN ?", allowedSpaceIDs).
		Where("members.role_in_space = ?", constant.RoleStudent).
		Where("members.status = ?", servicespace.StatusEnabled).
		Where("members.deleted_at = ?", 0).
		Order("members.user_id ASC").
		Scan(&ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

// existingExamUserTargetIDs 返回考试下已经存在的用户直投目标，避免重复写入唯一目标。
func (r *ExamRepository) existingExamUserTargetIDs(ctx context.Context, tenantID uint64, examID uint64, userIDs []uint64) ([]uint64, error) {
	var ids []uint64
	if err := r.db.WithContext(ctx).Model(&ExamTargetDO{}).
		Select(ExamTargetColumns.TargetID).
		Where(ExamTargetColumns.TenantID+" = ?", tenantID).
		Where(ExamTargetColumns.ExamID+" = ?", examID).
		Where(ExamTargetColumns.TargetType+" = ?", serviceexam.TargetTypeUser).
		Where(ExamTargetColumns.TargetID+" IN ?", userIDs).
		Order(ExamTargetColumns.TargetID + " ASC").
		Scan(&ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

// ResendInvitations 校验本次重发目标是否仍属于当前授权范围内的应考名单，并写入操作日志。
// 当前系统没有外部通知表或短信/邮件通道，因此仓储层只负责“可发送目标过滤 + 审计落库”的原子性。
func (r *ExamRepository) ResendInvitations(ctx context.Context, input serviceexam.ResendInvitationsRepositoryInput) (serviceexam.ResendInvitationsRepositoryResult, error) {
	result := serviceexam.ResendInvitationsRepositoryResult{SkippedCount: len(input.UserIDs)}
	if len(input.UserIDs) == 0 {
		return result, nil
	}
	if len(input.AllowedSpaceIDs) == 0 {
		return result, nil
	}

	validUserIDs, err := r.validInvitationCandidateUserIDs(ctx, input.TenantID, input.ExamID, input.UserIDs, input.AllowedSpaceIDs)
	if err != nil {
		return serviceexam.ResendInvitationsRepositoryResult{}, err
	}
	validSet := uint64Set(validUserIDs)
	sentUserIDs := make([]uint64, 0, len(validUserIDs))
	seenSent := make(map[uint64]struct{}, len(validUserIDs))
	for _, userID := range input.UserIDs {
		if _, ok := validSet[userID]; !ok {
			continue
		}
		if _, ok := seenSent[userID]; ok {
			continue
		}
		seenSent[userID] = struct{}{}
		sentUserIDs = append(sentUserIDs, userID)
	}
	if len(sentUserIDs) == 0 {
		return result, nil
	}

	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		spaceIDs, err := r.operationUserSpaceIDsInTx(tx, input.TenantID, sentUserIDs, input.AllowedSpaceIDs)
		if err != nil {
			return err
		}
		return r.appendOperationLogsForSpacesInTx(tx, input.Log, spaceIDs)
	})
	if err != nil {
		return serviceexam.ResendInvitationsRepositoryResult{}, err
	}
	result.SentUserIDs = sentUserIDs
	result.SentCount = len(sentUserIDs)
	result.SkippedCount = len(input.UserIDs) - result.SentCount
	return result, nil
}

// validInvitationCandidateUserIDs 返回输入用户中真实属于该考试、且命中授权空间的应考用户。
// 查询复用考生列表的 CTE，确保空间投放和用户直投的口径与考生管理 tab 完全一致。
func (r *ExamRepository) validInvitationCandidateUserIDs(ctx context.Context, tenantID uint64, examID uint64, userIDs []uint64, allowedSpaceIDs []uint64) ([]uint64, error) {
	var ids []uint64
	sql := r.examCandidateExpandedSQL(`
		SELECT DISTINCT candidate_id
		FROM expanded_candidates
		WHERE candidate_id IN ?
		ORDER BY candidate_id ASC
	`)
	args := append(examCandidateExpandedArgs(tenantID, examID, allowedSpaceIDs), userIDs)
	if err := r.db.WithContext(ctx).Raw(sql, args...).Scan(&ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

// uint64Set 把 ID 列表转换为集合，供批量过滤使用。
func uint64Set(values []uint64) map[uint64]struct{} {
	items := make(map[uint64]struct{}, len(values))
	for _, value := range values {
		items[value] = struct{}{}
	}
	return items
}

// FindExamByInviteCode 通过邀请码读取考试。
// 考生入口没有 tenant 上下文，因此只按邀请码和未删除状态定位考试。
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

// GetExam 读取单场考试并补齐试卷组卷模式。
// service 层需要 BuildMode 判断固定组卷或动态组卷的后续作答快照来源。
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
	exam := examFromDO(row, paper.BuildMode)
	targets, err := r.ListTargets(ctx, tenantID, examID)
	if err != nil {
		return serviceexam.Exam{}, err
	}
	if len(targets) > 0 {
		exam.Targets = append([]serviceexam.Target(nil), targets...)
		exam.TargetType = targets[0].TargetType
		exam.TargetID = targets[0].TargetID
	}
	return exam, nil
}

// ListTargets 读取考试已配置的投放目标，用于从真实资源范围重建管理权限。
func (r *ExamRepository) ListTargets(ctx context.Context, tenantID uint64, examID uint64) ([]serviceexam.Target, error) {
	return r.listTargetsInDB(r.db.WithContext(ctx), tenantID, examID)
}

func (r *ExamRepository) listTargetsInDB(db *gorm.DB, tenantID uint64, examID uint64) ([]serviceexam.Target, error) {
	var rows []ExamTargetDO
	if err := db.
		Where(ExamTargetColumns.TenantID+" = ?", tenantID).
		Where(ExamTargetColumns.ExamID+" = ?", examID).
		Order(ExamTargetColumns.ID + " ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	targets := make([]serviceexam.Target, 0, len(rows))
	targetRowIDs := make([]uint64, 0, len(rows))
	targetIndex := make(map[uint64]int, len(rows))
	for _, row := range rows {
		targetIndex[row.ID] = len(targets)
		targetRowIDs = append(targetRowIDs, row.ID)
		targets = append(targets, serviceexam.Target{
			TenantID:   row.TenantID,
			ExamID:     row.ExamID,
			TargetType: row.TargetType,
			TargetID:   row.TargetID,
		})
	}
	if err := r.fillTargetScopeSpaceIDs(db, tenantID, targetRowIDs, targetIndex, targets); err != nil {
		return nil, err
	}
	return targets, nil
}

func (r *ExamRepository) fillTargetScopeSpaceIDs(db *gorm.DB, tenantID uint64, targetRowIDs []uint64, targetIndex map[uint64]int, targets []serviceexam.Target) error {
	if len(targetRowIDs) == 0 {
		return nil
	}
	var rows []ExamTargetScopeSpaceDO
	if err := db.
		Where(ExamTargetScopeSpaceColumns.TenantID+" = ?", tenantID).
		Where(ExamTargetScopeSpaceColumns.ExamTargetID+" IN ?", targetRowIDs).
		Order(ExamTargetScopeSpaceColumns.ExamTargetID + " ASC, " + ExamTargetScopeSpaceColumns.SpaceID + " ASC").
		Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		index, ok := targetIndex[row.ExamTargetID]
		if !ok {
			continue
		}
		if targets[index].TargetType != serviceexam.TargetTypeUser {
			continue
		}
		targets[index].ScopeSpaceIDs = append(targets[index].ScopeSpaceIDs, row.SpaceID)
	}
	for index := range targets {
		targets[index].ScopeSpaceIDs = uniqueSortedOperationSpaceIDs(targets[index].ScopeSpaceIDs)
	}
	return nil
}

// AppendOperationLog 追加写入管理端操作日志。
// 操作人字段必须由 service 层从 session 派生后传入，仓储层只负责持久化并补齐创建字段。
func (r *ExamRepository) AppendOperationLog(ctx context.Context, log serviceexam.OperationLog) error {
	return r.appendOperationLogInTx(r.db.WithContext(ctx), log)
}

// appendOperationLogInTx 在给定事务或连接上追加操作日志。
// 导入考生等关键写操作会传入事务对象，确保业务数据和审计日志一起提交或一起回滚。
func (r *ExamRepository) appendOperationLogInTx(tx *gorm.DB, log serviceexam.OperationLog) error {
	now := r.now()
	extJSON := log.ExtJSON
	if strings.TrimSpace(extJSON) == "" {
		extJSON = "{}"
	}
	createdByType := log.CreatedByType
	if createdByType == "" {
		createdByType = log.ActorType
	}
	createdBy := log.CreatedBy
	if createdBy == 0 {
		createdBy = log.ActorID
	}
	row := ExamOperationLogDO{
		EventFields: EventFields{
			CreatedAt:     now,
			CreatedBy:     createdBy,
			CreatedByType: createdByType,
			ExtJSON:       datatypes.JSON([]byte(extJSON)),
		},
		TenantID:        log.TenantID,
		ExamID:          log.ExamID,
		OperationType:   log.OperationType,
		OperationTitle:  log.OperationTitle,
		OperationDetail: log.OperationDetail,
		ActorID:         log.ActorID,
		ActorType:       log.ActorType,
		ActorRole:       log.ActorRole,
		SpaceID:         log.SpaceID,
	}
	return tx.Create(&row).Error
}

// appendOperationLogsForSpacesInTx 按受影响空间拆分同一次管理操作。
// 租户管理员需要通过 operation_group_id 聚合同一次操作；空间角色只能按自己的 space_id 精确看到对应日志。
func (r *ExamRepository) appendOperationLogsForSpacesInTx(tx *gorm.DB, log serviceexam.OperationLog, spaceIDs []uint64) error {
	scopedSpaceIDs := uniqueSortedOperationSpaceIDs(spaceIDs)
	if len(scopedSpaceIDs) == 0 {
		return r.appendOperationLogInTx(tx, log)
	}
	groupExtJSON := operationGroupExtJSON(log, r.now())
	for _, spaceID := range scopedSpaceIDs {
		scopedSpaceID := spaceID
		scopedLog := log
		scopedLog.SpaceID = &scopedSpaceID
		scopedLog.ExtJSON = groupExtJSON
		if err := r.appendOperationLogInTx(tx, scopedLog); err != nil {
			return err
		}
	}
	return nil
}

// operationGroupExtJSON 在日志扩展字段中补齐 operation_group_id。
// 这里保留调用方已有扩展字段，只在缺少组 ID 时写入当前操作生成的稳定审计组。
func operationGroupExtJSON(log serviceexam.OperationLog, now int64) string {
	ext := map[string]any{}
	if strings.TrimSpace(log.ExtJSON) != "" {
		_ = json.Unmarshal([]byte(log.ExtJSON), &ext)
	}
	if groupID, ok := ext["operation_group_id"].(string); !ok || groupID == "" {
		ext["operation_group_id"] = newOperationGroupID(log, now)
	}
	payload, err := json.Marshal(ext)
	if err != nil {
		return `{}`
	}
	return string(payload)
}

func newOperationGroupID(log serviceexam.OperationLog, now int64) string {
	var entropy [16]byte
	if _, err := rand.Read(entropy[:]); err == nil {
		return fmt.Sprintf("%s-%d-%d-%d-%d-%x", log.OperationType, log.TenantID, log.ExamID, log.ActorID, now, entropy)
	}
	sequence := atomic.AddUint64(&operationGroupSequence, 1)
	return fmt.Sprintf("%s-%d-%d-%d-%d-%d", log.OperationType, log.TenantID, log.ExamID, log.ActorID, now, sequence)
}

func (r *ExamRepository) createTargetScopeSpacesInTx(tx *gorm.DB, targetRow ExamTargetDO, target serviceexam.Target, now int64) error {
	scopeSpaceIDs := targetScopeSpaceIDsForStorage(target)
	for _, spaceID := range scopeSpaceIDs {
		scopeRow := ExamTargetScopeSpaceDO{
			RelationFields: RelationFields{
				CreatedAt:     now,
				CreatedByType: AuditActorTenantUser,
				ExtJSON:       datatypes.JSON([]byte("{}")),
			},
			TenantID:     targetRow.TenantID,
			ExamID:       targetRow.ExamID,
			ExamTargetID: targetRow.ID,
			SpaceID:      spaceID,
		}
		if err := tx.Create(&scopeRow).Error; err != nil {
			return err
		}
	}
	return nil
}

func targetScopeSpaceIDsForStorage(target serviceexam.Target) []uint64 {
	switch target.TargetType {
	case serviceexam.TargetTypeSpace:
		if target.TargetID == 0 {
			return nil
		}
		return []uint64{target.TargetID}
	case serviceexam.TargetTypeUser:
		return uniqueSortedOperationSpaceIDs(target.ScopeSpaceIDs)
	default:
		return nil
	}
}

// operationTargetSpaceIDsInTx 把发布目标展开为可见空间范围。
// 空间目标直接使用空间 ID；用户直投目标按用户当前启用空间展开，满足操作日志空间过滤规则。
func (r *ExamRepository) operationTargetSpaceIDsInTx(tx *gorm.DB, tenantID uint64, targets []serviceexam.Target) ([]uint64, error) {
	spaceIDs := make([]uint64, 0, len(targets))
	for _, target := range targets {
		switch target.TargetType {
		case serviceexam.TargetTypeSpace:
			spaceIDs = append(spaceIDs, target.TargetID)
		case serviceexam.TargetTypeUser:
			userSpaceIDs, err := r.operationUserSpaceIDsInTx(tx, tenantID, []uint64{target.TargetID}, target.ScopeSpaceIDs)
			if err != nil {
				return nil, err
			}
			spaceIDs = append(spaceIDs, userSpaceIDs...)
		}
	}
	return uniqueSortedOperationSpaceIDs(spaceIDs), nil
}

// operationUserSpaceIDsInTx 返回用户当前所在的启用空间。
// allowedSpaceIDs 非空时会进一步收窄到当前管理者授权范围，避免越权写入其它空间可见日志。
func (r *ExamRepository) operationUserSpaceIDsInTx(tx *gorm.DB, tenantID uint64, userIDs []uint64, allowedSpaceIDs []uint64) ([]uint64, error) {
	userIDs = uniqueSortedOperationSpaceIDs(userIDs)
	if len(userIDs) == 0 {
		return nil, nil
	}
	query := tx.Table("space_members AS members").
		Select("DISTINCT members.space_id").
		Joins("JOIN spaces ON spaces.tenant_id = members.tenant_id AND spaces.id = members.space_id AND spaces.status = ? AND spaces.deleted_at = 0", servicespace.StatusEnabled).
		Joins("JOIN users ON users.id = members.user_id AND users.status = ? AND users.deleted_at = 0", servicetenantuser.StatusEnabled).
		Joins("JOIN tenant_user_memberships AS tum ON tum.tenant_id = members.tenant_id AND tum.user_id = members.user_id AND tum.role = ? AND tum.status = ?", constant.RoleStudent, servicetenantuser.StatusEnabled).
		Where("members.tenant_id = ?", tenantID).
		Where("members.user_id IN ?", userIDs).
		Where("members.role_in_space = ?", constant.RoleStudent).
		Where("members.status = ?", servicespace.StatusEnabled).
		Where("members.deleted_at = ?", 0)
	if len(allowedSpaceIDs) > 0 {
		query = query.Where("members.space_id IN ?", uniqueSortedOperationSpaceIDs(allowedSpaceIDs))
	}
	var spaceIDs []uint64
	if err := query.Order("members.space_id ASC").Scan(&spaceIDs).Error; err != nil {
		return nil, err
	}
	return uniqueSortedOperationSpaceIDs(spaceIDs), nil
}

func targetUserIDs(targets []serviceexam.Target) []uint64 {
	userIDs := make([]uint64, 0, len(targets))
	for _, target := range targets {
		if target.TargetType == serviceexam.TargetTypeUser {
			userIDs = append(userIDs, target.TargetID)
		}
	}
	return userIDs
}

func uniqueSortedOperationSpaceIDs(values []uint64) []uint64 {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[uint64]struct{}, len(values))
	items := make([]uint64, 0, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		items = append(items, value)
	}
	sort.Slice(items, func(left, right int) bool {
		return items[left] < items[right]
	})
	return items
}

// ListOperationLogs 按考试分页读取管理端操作日志。
// space_id 过滤采用精确匹配，避免空间管理员或教师看到租户级或其它空间的审计记录。
func (r *ExamRepository) ListOperationLogs(ctx context.Context, input serviceexam.ListOperationLogsInput) (pagination.Result[serviceexam.OperationLog], error) {
	page := pagination.Normalize(pagination.Input{Page: input.Page, PageSize: input.PageSize})
	query := r.db.WithContext(ctx).Model(&ExamOperationLogDO{}).
		Where(ExamOperationLogColumns.TenantID+" = ?", input.TenantID).
		Where(ExamOperationLogColumns.ExamID+" = ?", input.ExamID)
	if input.SpaceID != nil {
		query = query.Where(ExamOperationLogColumns.SpaceID+" = ?", *input.SpaceID)
	}
	if input.OperationType != "" {
		query = query.Where(ExamOperationLogColumns.OperationType+" = ?", input.OperationType)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return pagination.Result[serviceexam.OperationLog]{}, err
	}
	var rows []ExamOperationLogDO
	if err := query.
		Order(EventColumns.CreatedAt + " DESC").
		Order(EventColumns.ID + " DESC").
		Limit(page.PageSize).
		Offset(pagination.Offset(page)).
		Find(&rows).Error; err != nil {
		return pagination.Result[serviceexam.OperationLog]{}, err
	}
	logs := make([]serviceexam.OperationLog, 0, len(rows))
	for _, row := range rows {
		logs = append(logs, operationLogFromDO(row))
	}
	return pagination.Result[serviceexam.OperationLog]{
		Items:    logs,
		Page:     page.Page,
		PageSize: page.PageSize,
		Total:    total,
	}, nil
}

// ExamTargetSpaceIDs 读取考试目标覆盖到的有效空间，用于无成绩时仍按真实考试范围鉴权。
func (r *ExamRepository) ExamTargetSpaceIDs(ctx context.Context, tenantID uint64, examID uint64) ([]uint64, error) {
	targets, err := r.ListTargets(ctx, tenantID, examID)
	if err != nil {
		return nil, err
	}
	return r.targetSpaceIDsForTargets(ctx, tenantID, targets)
}

func (r *ExamRepository) targetSpaceIDsForTargets(ctx context.Context, tenantID uint64, targets []serviceexam.Target) ([]uint64, error) {
	spaceIDs := make([]uint64, 0, len(targets))
	for _, target := range targets {
		switch target.TargetType {
		case serviceexam.TargetTypeSpace:
			spaceIDs = append(spaceIDs, target.TargetID)
		case serviceexam.TargetTypeUser:
			userSpaceIDs, err := r.operationUserSpaceIDsInTx(r.db.WithContext(ctx), tenantID, []uint64{target.TargetID}, target.ScopeSpaceIDs)
			if err != nil {
				return nil, err
			}
			spaceIDs = append(spaceIDs, userSpaceIDs...)
		}
	}
	return uniqueSortedOperationSpaceIDs(spaceIDs), nil
}

// CountExamCandidates 按考试目标和授权空间统计当前有效应考人数。
// 空间投放从启用目标空间成员展开，用户直投按该用户当前启用空间成员关系与授权空间求交集。
func (r *ExamRepository) CountExamCandidates(ctx context.Context, tenantID uint64, examID uint64, spaceIDs []uint64) (int, error) {
	if len(spaceIDs) == 0 {
		return 0, nil
	}
	enabledSpaceIDs, err := r.enabledSpaceIDs(ctx, tenantID, spaceIDs)
	if err != nil {
		return 0, err
	}
	if len(enabledSpaceIDs) == 0 {
		return 0, nil
	}
	candidateIDs := make(map[uint64]struct{})
	if err := r.collectSpaceTargetCandidateIDs(ctx, tenantID, examID, enabledSpaceIDs, candidateIDs); err != nil {
		return 0, err
	}
	if err := r.collectUserTargetCandidateIDs(ctx, tenantID, examID, enabledSpaceIDs, candidateIDs); err != nil {
		return 0, err
	}
	return len(candidateIDs), nil
}

const examCandidateCountBatchSize = 500

type examCandidateMemberRow struct {
	ID      uint64
	UserID  uint64
	SpaceID uint64
}

type examCandidateTargetRow struct {
	ID       uint64
	TargetID uint64
}

type examCandidateScopeRow struct {
	ExamTargetID uint64
	SpaceID      uint64
}

func (r *ExamRepository) enabledSpaceIDs(ctx context.Context, tenantID uint64, spaceIDs []uint64) ([]uint64, error) {
	ids := uniqueSortedOperationSpaceIDs(spaceIDs)
	if len(ids) == 0 {
		return nil, nil
	}
	var enabled []uint64
	if err := r.db.WithContext(ctx).Model(&SpaceDO{}).
		Where(SpaceColumns.TenantID+" = ?", tenantID).
		Where(SpaceColumns.ID+" IN ?", ids).
		Where(SpaceColumns.Status+" = ?", servicespace.StatusEnabled).
		Where(SpaceColumns.DeletedAt+" = ?", 0).
		Order(SpaceColumns.ID+" ASC").
		Pluck(SpaceColumns.ID, &enabled).Error; err != nil {
		return nil, err
	}
	return enabled, nil
}

func (r *ExamRepository) collectSpaceTargetCandidateIDs(ctx context.Context, tenantID uint64, examID uint64, enabledSpaceIDs []uint64, candidateIDs map[uint64]struct{}) error {
	var lastTargetID uint64
	for {
		var targets []examCandidateTargetRow
		if err := r.db.WithContext(ctx).Model(&ExamTargetDO{}).
			Select(ExamTargetColumns.ID+", "+ExamTargetColumns.TargetID).
			Where(ExamTargetColumns.TenantID+" = ?", tenantID).
			Where(ExamTargetColumns.ExamID+" = ?", examID).
			Where(ExamTargetColumns.TargetType+" = ?", serviceexam.TargetTypeSpace).
			Where(ExamTargetColumns.TargetID+" IN ?", enabledSpaceIDs).
			Where(ExamTargetColumns.ID+" > ?", lastTargetID).
			Order(ExamTargetColumns.ID + " ASC").
			Limit(examCandidateCountBatchSize).
			Find(&targets).Error; err != nil {
			return err
		}
		if len(targets) == 0 {
			return nil
		}
		lastTargetID = targets[len(targets)-1].ID
		targetSpaceIDs := make([]uint64, 0, len(targets))
		for _, target := range targets {
			targetSpaceIDs = append(targetSpaceIDs, target.TargetID)
		}
		if err := r.collectSpaceTargetCandidateBatch(ctx, tenantID, targetSpaceIDs, candidateIDs); err != nil {
			return err
		}
		if len(targets) < examCandidateCountBatchSize {
			return nil
		}
	}
}

func (r *ExamRepository) collectSpaceTargetCandidateBatch(ctx context.Context, tenantID uint64, targetSpaceIDs []uint64, candidateIDs map[uint64]struct{}) error {
	var lastMemberID uint64
	for {
		var rows []examCandidateMemberRow
		if err := r.db.WithContext(ctx).Model(&SpaceMemberDO{}).
			Select(SpaceMemberColumns.ID+", "+SpaceMemberColumns.UserID).
			Where(SpaceMemberColumns.TenantID+" = ?", tenantID).
			Where(SpaceMemberColumns.SpaceID+" IN ?", targetSpaceIDs).
			Where(SpaceMemberColumns.RoleInSpace+" = ?", constant.RoleStudent).
			Where(SpaceMemberColumns.Status+" = ?", servicespace.StatusEnabled).
			Where(SpaceMemberColumns.DeletedAt+" = ?", 0).
			Where(SpaceMemberColumns.ID+" > ?", lastMemberID).
			Order(SpaceMemberColumns.ID + " ASC").
			Limit(examCandidateCountBatchSize).
			Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		lastMemberID = rows[len(rows)-1].ID
		userIDs := make([]uint64, 0, len(rows))
		for _, row := range rows {
			userIDs = append(userIDs, row.UserID)
		}
		validUserIDs, err := r.validEnabledTenantStudentUserSet(ctx, tenantID, userIDs)
		if err != nil {
			return err
		}
		for _, row := range rows {
			if _, ok := validUserIDs[row.UserID]; ok {
				candidateIDs[row.UserID] = struct{}{}
			}
		}
		if len(rows) < examCandidateCountBatchSize {
			return nil
		}
	}
}

func (r *ExamRepository) collectUserTargetCandidateIDs(ctx context.Context, tenantID uint64, examID uint64, enabledSpaceIDs []uint64, candidateIDs map[uint64]struct{}) error {
	enabledSpaceSet := uint64Set(enabledSpaceIDs)
	var lastTargetID uint64
	for {
		var targets []examCandidateTargetRow
		if err := r.db.WithContext(ctx).Model(&ExamTargetDO{}).
			Select(ExamTargetColumns.ID+", "+ExamTargetColumns.TargetID).
			Where(ExamTargetColumns.TenantID+" = ?", tenantID).
			Where(ExamTargetColumns.ExamID+" = ?", examID).
			Where(ExamTargetColumns.TargetType+" = ?", serviceexam.TargetTypeUser).
			Where(ExamTargetColumns.ID+" > ?", lastTargetID).
			Order(ExamTargetColumns.ID + " ASC").
			Limit(examCandidateCountBatchSize).
			Find(&targets).Error; err != nil {
			return err
		}
		if len(targets) == 0 {
			return nil
		}
		lastTargetID = targets[len(targets)-1].ID
		if err := r.collectUserTargetCandidateBatch(ctx, tenantID, examID, enabledSpaceIDs, enabledSpaceSet, targets, candidateIDs); err != nil {
			return err
		}
		if len(targets) < examCandidateCountBatchSize {
			return nil
		}
	}
}

func (r *ExamRepository) collectUserTargetCandidateBatch(ctx context.Context, tenantID uint64, examID uint64, enabledSpaceIDs []uint64, enabledSpaceSet map[uint64]struct{}, targets []examCandidateTargetRow, candidateIDs map[uint64]struct{}) error {
	targetRowIDs := make([]uint64, 0, len(targets))
	userIDs := make([]uint64, 0, len(targets))
	targetsByUser := make(map[uint64][]examCandidateTargetRow, len(targets))
	for _, target := range targets {
		targetRowIDs = append(targetRowIDs, target.ID)
		userIDs = append(userIDs, target.TargetID)
		targetsByUser[target.TargetID] = append(targetsByUser[target.TargetID], target)
	}
	scopedSpacesByTargetID, err := r.userTargetScopeSpacesByTargetID(ctx, tenantID, examID, targetRowIDs)
	if err != nil {
		return err
	}
	validUserIDs, err := r.validEnabledTenantStudentUserSet(ctx, tenantID, userIDs)
	if err != nil {
		return err
	}
	if len(validUserIDs) == 0 {
		return nil
	}
	var lastMemberID uint64
	for {
		var members []examCandidateMemberRow
		if err := r.db.WithContext(ctx).Model(&SpaceMemberDO{}).
			Select(SpaceMemberColumns.ID+", "+SpaceMemberColumns.UserID+", "+SpaceMemberColumns.SpaceID).
			Where(SpaceMemberColumns.TenantID+" = ?", tenantID).
			Where(SpaceMemberColumns.UserID+" IN ?", userIDs).
			Where(SpaceMemberColumns.SpaceID+" IN ?", enabledSpaceIDs).
			Where(SpaceMemberColumns.RoleInSpace+" = ?", constant.RoleStudent).
			Where(SpaceMemberColumns.Status+" = ?", servicespace.StatusEnabled).
			Where(SpaceMemberColumns.DeletedAt+" = ?", 0).
			Where(SpaceMemberColumns.ID+" > ?", lastMemberID).
			Order(SpaceMemberColumns.ID + " ASC").
			Limit(examCandidateCountBatchSize).
			Find(&members).Error; err != nil {
			return err
		}
		if len(members) == 0 {
			return nil
		}
		lastMemberID = members[len(members)-1].ID
		for _, member := range members {
			if _, ok := validUserIDs[member.UserID]; !ok {
				continue
			}
			for _, target := range targetsByUser[member.UserID] {
				if userTargetAllowsSpace(target.ID, member.SpaceID, enabledSpaceSet, scopedSpacesByTargetID) {
					candidateIDs[member.UserID] = struct{}{}
					break
				}
			}
		}
		if len(members) < examCandidateCountBatchSize {
			return nil
		}
	}
}

func (r *ExamRepository) userTargetScopeSpacesByTargetID(ctx context.Context, tenantID uint64, examID uint64, targetRowIDs []uint64) (map[uint64]map[uint64]struct{}, error) {
	result := make(map[uint64]map[uint64]struct{})
	if len(targetRowIDs) == 0 {
		return result, nil
	}
	var rows []examCandidateScopeRow
	if err := r.db.WithContext(ctx).Model(&ExamTargetScopeSpaceDO{}).
		Select(ExamTargetScopeSpaceColumns.ExamTargetID+", "+ExamTargetScopeSpaceColumns.SpaceID).
		Where(ExamTargetScopeSpaceColumns.TenantID+" = ?", tenantID).
		Where(ExamTargetScopeSpaceColumns.ExamID+" = ?", examID).
		Where(ExamTargetScopeSpaceColumns.ExamTargetID+" IN ?", targetRowIDs).
		Order(ExamTargetScopeSpaceColumns.ExamTargetID + " ASC, " + ExamTargetScopeSpaceColumns.SpaceID + " ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		if result[row.ExamTargetID] == nil {
			result[row.ExamTargetID] = make(map[uint64]struct{})
		}
		result[row.ExamTargetID][row.SpaceID] = struct{}{}
	}
	return result, nil
}

func (r *ExamRepository) validEnabledTenantStudentUserSet(ctx context.Context, tenantID uint64, userIDs []uint64) (map[uint64]struct{}, error) {
	ids := uniqueSortedOperationSpaceIDs(userIDs)
	if len(ids) == 0 {
		return map[uint64]struct{}{}, nil
	}
	var enabledUserIDs []uint64
	if err := r.db.WithContext(ctx).Model(&UserDO{}).
		Where(UserColumns.ID+" IN ?", ids).
		Where(UserColumns.Status+" = ?", servicetenantuser.StatusEnabled).
		Where(UserColumns.DeletedAt+" = ?", 0).
		Pluck(UserColumns.ID, &enabledUserIDs).Error; err != nil {
		return nil, err
	}
	if len(enabledUserIDs) == 0 {
		return map[uint64]struct{}{}, nil
	}
	var tenantStudentIDs []uint64
	if err := r.db.WithContext(ctx).Model(&UserRoleDO{}).
		Where(UserRoleColumns.TenantID+" = ?", tenantID).
		Where(UserRoleColumns.UserID+" IN ?", enabledUserIDs).
		Where(UserRoleColumns.Role+" = ?", constant.RoleStudent).
		Where(UserRoleColumns.Status+" = ?", servicetenantuser.StatusEnabled).
		Pluck(UserRoleColumns.UserID, &tenantStudentIDs).Error; err != nil {
		return nil, err
	}
	return uint64Set(tenantStudentIDs), nil
}

func userTargetAllowsSpace(targetID uint64, spaceID uint64, enabledSpaceSet map[uint64]struct{}, scopedSpacesByTargetID map[uint64]map[uint64]struct{}) bool {
	if _, ok := enabledSpaceSet[spaceID]; !ok {
		return false
	}
	scopedSpaces, hasScope := scopedSpacesByTargetID[targetID]
	if !hasScope {
		return true
	}
	_, ok := scopedSpaces[spaceID]
	return ok
}

// ListExamCandidates 动态展开考试目标形成当前页应考名单。
// 查询先按授权空间求出候选 user_id 页，再补充来源和作答摘要，避免大考试一次性加载全量考生。
func (r *ExamRepository) ListExamCandidates(ctx context.Context, input serviceexam.ListExamCandidatesInput) (pagination.Result[serviceexam.ExamCandidate], error) {
	page := pagination.Normalize(pagination.Input{Page: input.Page, PageSize: input.PageSize})
	if len(input.SpaceIDs) == 0 {
		return pagination.Result[serviceexam.ExamCandidate]{Items: []serviceexam.ExamCandidate{}, Page: page.Page, PageSize: page.PageSize}, nil
	}
	keyword := strings.TrimSpace(input.Keyword)
	var total int64
	countSQL := r.examCandidateExpandedSQL(`
		SELECT COUNT(*)
		FROM (
			SELECT candidate_id
			FROM expanded_candidates
			LEFT JOIN candidate_attempt_status ON candidate_attempt_status.user_id = expanded_candidates.candidate_id
			WHERE (? = ''
				OR username LIKE ?
				OR real_name LIKE ?
				OR space_name LIKE ?)
			GROUP BY candidate_id
			HAVING (? = '' OR ? = CASE
				WHEN MAX(COALESCE(has_submitted, 0)) = 1 THEN 'submitted'
				WHEN MAX(COALESCE(has_in_progress, 0)) = 1 THEN 'in_progress'
				ELSE 'not_started'
			END)
		) AS filtered_candidates
	`)
	keywordLike := "%" + keyword + "%"
	searchArgs := []any{keyword, keywordLike, keywordLike, keywordLike}
	filterArgs := append(searchArgs, input.Status, input.Status)
	if err := r.db.WithContext(ctx).Raw(countSQL, examCandidateExpandedArgs(input.TenantID, input.ExamID, input.SpaceIDs, filterArgs...)...).Scan(&total).Error; err != nil {
		return pagination.Result[serviceexam.ExamCandidate]{}, err
	}
	if total == 0 {
		return pagination.Result[serviceexam.ExamCandidate]{Items: []serviceexam.ExamCandidate{}, Page: page.Page, PageSize: page.PageSize, Total: total}, nil
	}

	var rows []examCandidatePageRow
	pageSQL := r.examCandidateExpandedSQL(`
		SELECT candidate_id AS user_id,
			MIN(username) AS username,
			MIN(real_name) AS real_name,
			MIN(source_space_id) AS space_id,
			MIN(space_name) AS space_name
		FROM expanded_candidates
		LEFT JOIN candidate_attempt_status ON candidate_attempt_status.user_id = expanded_candidates.candidate_id
		WHERE (? = ''
			OR username LIKE ?
			OR real_name LIKE ?
			OR space_name LIKE ?)
		GROUP BY candidate_id
		HAVING (? = '' OR ? = CASE
			WHEN MAX(COALESCE(has_submitted, 0)) = 1 THEN 'submitted'
			WHEN MAX(COALESCE(has_in_progress, 0)) = 1 THEN 'in_progress'
			ELSE 'not_started'
		END)
		ORDER BY MIN(real_name) ASC, candidate_id ASC
		LIMIT ? OFFSET ?
	`)
	pageArgs := append(examCandidateExpandedArgs(input.TenantID, input.ExamID, input.SpaceIDs, filterArgs...), page.PageSize, pagination.Offset(page))
	if err := r.db.WithContext(ctx).Raw(pageSQL, pageArgs...).Scan(&rows).Error; err != nil {
		return pagination.Result[serviceexam.ExamCandidate]{}, err
	}
	items := make([]serviceexam.ExamCandidate, 0, len(rows))
	userIDs := make([]uint64, 0, len(rows))
	for _, row := range rows {
		userIDs = append(userIDs, row.UserID)
		items = append(items, serviceexam.ExamCandidate{
			UserID:    row.UserID,
			Username:  row.Username,
			RealName:  row.RealName,
			SpaceID:   row.SpaceID,
			SpaceName: row.SpaceName,
			Status:    serviceexam.CandidateStatusNotStarted,
		})
	}
	sources, err := r.examCandidateSources(ctx, input.TenantID, input.ExamID, input.SpaceIDs, userIDs)
	if err != nil {
		return pagination.Result[serviceexam.ExamCandidate]{}, err
	}
	attempts, err := r.examCandidateAttempts(ctx, input.TenantID, input.ExamID, userIDs)
	if err != nil {
		return pagination.Result[serviceexam.ExamCandidate]{}, err
	}
	for index := range items {
		items[index].SourceTargets = sources[items[index].UserID]
		applyCandidateAttemptSummary(&items[index], attempts[items[index].UserID], input.ResultStrategy)
	}
	return pagination.Result[serviceexam.ExamCandidate]{Items: items, Page: page.Page, PageSize: page.PageSize, Total: total}, nil
}

func (r *ExamRepository) examCandidateExpandedSQL(selectSQL string) string {
	return examCandidateExpandedSQL(r.userTargetScopedSpacePredicate("targets", "members.space_id"), selectSQL)
}

type examCandidatePageRow struct {
	UserID    uint64 // 考生用户 ID。
	Username  string // 登录名或学号。
	RealName  string // 考生姓名。
	SpaceID   uint64 // 展示用空间 ID。
	SpaceName string // 展示用空间名称。
}

type examCandidateSourceRow struct {
	CandidateID   uint64 // 考生用户 ID。
	TargetType    string // 来源目标类型。
	TargetID      uint64 // 来源目标 ID。
	SourceSpaceID uint64 // 来源命中的空间 ID。
	SpaceName     string // 来源命中的空间名称。
}

type examCandidateAttemptRow struct {
	ID          uint64 // 作答 ID。
	UserID      uint64 // 考生用户 ID。
	Status      string // 作答状态。
	StartedAt   int64  // 开始作答时间。
	SubmittedAt *int64 // 提交时间。
	TotalScore  string // 总分。
	AttemptNo   int    // 第几次作答。
}

// examCandidateExpandedSQL 构建考生动态展开 CTE，保证 count、分页和来源查询使用同一套有效性条件。
func examCandidateExpandedSQL(userTargetScopePredicate string, selectSQL string) string {
	return fmt.Sprintf(`
		WITH expanded_candidates AS (
			SELECT members.user_id AS candidate_id,
				users.username AS username,
				users.real_name AS real_name,
				targets.target_type AS target_type,
				targets.target_id AS target_id,
				members.space_id AS source_space_id,
				spaces.name AS space_name
			FROM exam_targets AS targets
			JOIN spaces ON spaces.tenant_id = targets.tenant_id
				AND spaces.id = targets.target_id
				AND spaces.status = ?
				AND spaces.deleted_at = 0
			JOIN space_members AS members ON members.tenant_id = targets.tenant_id
				AND members.space_id = targets.target_id
				AND members.space_id IN ?
				AND members.role_in_space = ?
				AND members.status = ?
				AND members.deleted_at = 0
			JOIN users ON users.id = members.user_id
				AND users.status = ?
				AND users.deleted_at = 0
			JOIN tenant_user_memberships AS tum ON tum.tenant_id = members.tenant_id
				AND tum.user_id = members.user_id
				AND tum.role = ?
				AND tum.status = ?
			WHERE targets.tenant_id = ?
				AND targets.exam_id = ?
				AND targets.target_type = ?
			UNION ALL
			SELECT targets.target_id AS candidate_id,
				users.username AS username,
				users.real_name AS real_name,
				targets.target_type AS target_type,
				targets.target_id AS target_id,
				members.space_id AS source_space_id,
				spaces.name AS space_name
			FROM exam_targets AS targets
			JOIN users ON users.id = targets.target_id
				AND users.status = ?
				AND users.deleted_at = 0
			JOIN tenant_user_memberships AS tum ON tum.tenant_id = targets.tenant_id
				AND tum.user_id = targets.target_id
				AND tum.role = ?
				AND tum.status = ?
			JOIN space_members AS members ON members.tenant_id = targets.tenant_id
				AND members.user_id = targets.target_id
				AND members.space_id IN ?
				AND members.role_in_space = ?
				AND members.status = ?
				AND members.deleted_at = 0
				AND %s
			JOIN spaces ON spaces.tenant_id = members.tenant_id
				AND spaces.id = members.space_id
				AND spaces.status = ?
				AND spaces.deleted_at = 0
			WHERE targets.tenant_id = ?
				AND targets.exam_id = ?
				AND targets.target_type = ?
		),
		candidate_attempt_status AS (
			SELECT user_id,
				MAX(CASE WHEN submitted_at IS NOT NULL OR status = ? THEN 1 ELSE 0 END) AS has_submitted,
				MAX(CASE WHEN submitted_at IS NULL AND status = ? THEN 1 ELSE 0 END) AS has_in_progress
			FROM exam_attempts
			WHERE tenant_id = ?
				AND exam_id = ?
			GROUP BY user_id
		)
	`, userTargetScopePredicate) + selectSQL
}

// examCandidateExpandedArgs 返回 expanded_candidates 和 candidate_attempt_status 两个 CTE 的公共参数。
func examCandidateExpandedArgs(tenantID uint64, examID uint64, spaceIDs []uint64, extra ...any) []any {
	args := []any{
		servicespace.StatusEnabled, spaceIDs, constant.RoleStudent, servicespace.StatusEnabled, servicetenantuser.StatusEnabled, constant.RoleStudent, servicetenantuser.StatusEnabled,
		tenantID, examID, serviceexam.TargetTypeSpace,
		servicetenantuser.StatusEnabled, constant.RoleStudent, servicetenantuser.StatusEnabled, spaceIDs, constant.RoleStudent, servicespace.StatusEnabled, servicespace.StatusEnabled,
		tenantID, examID, serviceexam.TargetTypeUser,
		serviceexam.AttemptStatusSubmitted, serviceexam.AttemptStatusInProgress, tenantID, examID,
	}
	return append(args, extra...)
}

// userTargetScopedSpacePredicate 让用户直投优先命中发布时持久化的 scoped spaces。
// 历史目标若未写入映射表，则继续回退到用户当前有效成员空间。
func (r *ExamRepository) userTargetScopedSpacePredicate(targetAlias string, memberSpaceColumn string) string {
	return fmt.Sprintf(`(
		NOT EXISTS (
			SELECT 1
			FROM exam_target_scope_spaces AS target_scope_absence
			WHERE target_scope_absence.tenant_id = %s.tenant_id
				AND target_scope_absence.exam_id = %s.exam_id
				AND target_scope_absence.exam_target_id = %s.id
		)
		OR EXISTS (
			SELECT 1
			FROM exam_target_scope_spaces AS target_scope_match
			WHERE target_scope_match.tenant_id = %s.tenant_id
				AND target_scope_match.exam_id = %s.exam_id
				AND target_scope_match.exam_target_id = %s.id
				AND target_scope_match.space_id = %s
		)
	)`, targetAlias, targetAlias, targetAlias, targetAlias, targetAlias, targetAlias, memberSpaceColumn)
}

// examCandidateSources 返回当前页考生的来源摘要，空间投放和用户直投都保留，供前端解释去重来源。
func (r *ExamRepository) examCandidateSources(ctx context.Context, tenantID uint64, examID uint64, spaceIDs []uint64, userIDs []uint64) (map[uint64][]serviceexam.CandidateSourceTarget, error) {
	var rows []examCandidateSourceRow
	sql := r.examCandidateExpandedSQL(`
		SELECT DISTINCT candidate_id,
			target_type,
			target_id,
			source_space_id,
			space_name
		FROM expanded_candidates
		WHERE candidate_id IN ?
		ORDER BY candidate_id ASC, target_type ASC, target_id ASC, source_space_id ASC
	`)
	args := append(examCandidateExpandedArgs(tenantID, examID, spaceIDs), userIDs)
	if err := r.db.WithContext(ctx).Raw(sql, args...).Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[uint64][]serviceexam.CandidateSourceTarget, len(userIDs))
	for _, row := range rows {
		result[row.CandidateID] = append(result[row.CandidateID], serviceexam.CandidateSourceTarget{
			TargetType: row.TargetType,
			TargetID:   row.TargetID,
			SpaceID:    row.SourceSpaceID,
			SpaceName:  row.SpaceName,
		})
	}
	return result, nil
}

// examCandidateAttempts 批量读取当前页考生作答记录，后续在内存中按 result_strategy 选择展示作答。
func (r *ExamRepository) examCandidateAttempts(ctx context.Context, tenantID uint64, examID uint64, userIDs []uint64) (map[uint64][]examCandidateAttemptRow, error) {
	var rows []examCandidateAttemptRow
	if err := r.db.WithContext(ctx).Table("exam_attempts").
		Select("id, user_id, status, started_at, submitted_at, total_score, attempt_no").
		Where("tenant_id = ?", tenantID).
		Where("exam_id = ?", examID).
		Where("user_id IN ?", userIDs).
		Order("user_id ASC, id ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[uint64][]examCandidateAttemptRow, len(userIDs))
	for _, row := range rows {
		result[row.UserID] = append(result[row.UserID], row)
	}
	return result, nil
}

// applyCandidateAttemptSummary 根据作答记录生成考生管理行的状态和结果作答摘要。
func applyCandidateAttemptSummary(candidate *serviceexam.ExamCandidate, attempts []examCandidateAttemptRow, resultStrategy string) {
	candidate.AttemptCount = len(attempts)
	var current *examCandidateAttemptRow
	var result *examCandidateAttemptRow
	for index := range attempts {
		attempt := &attempts[index]
		if attempt.SubmittedAt == nil && attempt.Status == serviceexam.AttemptStatusInProgress {
			if current == nil || attempt.ID > current.ID {
				current = attempt
			}
			continue
		}
		if attempt.SubmittedAt != nil || attempt.Status == serviceexam.AttemptStatusSubmitted {
			if result == nil || candidateResultAttemptLess(*result, *attempt, resultStrategy) {
				result = attempt
			}
		}
	}
	if current != nil {
		candidate.Status = serviceexam.CandidateStatusInProgress
		candidate.CurrentAttemptID = uint64Ptr(current.ID)
		candidate.StartedAt = int64Ptr(current.StartedAt)
	}
	if result != nil {
		candidate.Status = serviceexam.CandidateStatusSubmitted
		candidate.ResultAttemptID = uint64Ptr(result.ID)
		candidate.CurrentAttemptID = nil
		candidate.StartedAt = int64Ptr(result.StartedAt)
		candidate.SubmittedAt = result.SubmittedAt
		candidate.TotalScore = result.TotalScore
	}
}

// candidateResultAttemptLess 判断 right 是否比 left 更适合作为最终结果作答。
func candidateResultAttemptLess(left examCandidateAttemptRow, right examCandidateAttemptRow, resultStrategy string) bool {
	if resultStrategy == serviceexam.ResultStrategyHighest {
		leftScore, _ := strconv.ParseFloat(left.TotalScore, 64)
		rightScore, _ := strconv.ParseFloat(right.TotalScore, 64)
		if leftScore != rightScore {
			return leftScore < rightScore
		}
	}
	leftSubmittedAt := int64(0)
	if left.SubmittedAt != nil {
		leftSubmittedAt = *left.SubmittedAt
	}
	rightSubmittedAt := int64(0)
	if right.SubmittedAt != nil {
		rightSubmittedAt = *right.SubmittedAt
	}
	if leftSubmittedAt != rightSubmittedAt {
		return leftSubmittedAt < rightSubmittedAt
	}
	return left.ID < right.ID
}

// uint64Ptr 返回 uint64 指针，避免在循环里直接取临时变量地址。
func uint64Ptr(value uint64) *uint64 {
	next := value
	return &next
}

// int64Ptr 返回 int64 指针，避免在循环里直接取临时变量地址。
func int64Ptr(value int64) *int64 {
	next := value
	return &next
}

// CountExamAttemptStats 按授权且仍启用的空间统计已开始、进行中和已交卷人数。
// 统计使用 distinct user_id，避免同一考生多次作答时把人数放大。
func (r *ExamRepository) CountExamAttemptStats(ctx context.Context, tenantID uint64, examID uint64, spaceIDs []uint64) (serviceexam.AttemptOverviewStats, error) {
	if len(spaceIDs) == 0 {
		return serviceexam.AttemptOverviewStats{}, nil
	}
	userTargetScopePredicate := r.userTargetScopedSpacePredicate("targets", "members.space_id")
	var rows []struct {
		UserID      uint64
		Status      string
		SubmittedAt *int64
	}
	if err := r.db.WithContext(ctx).Table("exam_attempts AS attempts").
		Select("DISTINCT attempts.user_id AS user_id, attempts.status AS status, attempts.submitted_at AS submitted_at").
		Joins("JOIN exam_targets AS targets ON targets.tenant_id = attempts.tenant_id AND targets.exam_id = attempts.exam_id").
		Joins(fmt.Sprintf(`
					JOIN space_members AS members ON members.tenant_id = attempts.tenant_id
						AND members.user_id = attempts.user_id
						AND members.space_id IN ?
						AND members.role_in_space = ?
						AND members.status = ?
						AND members.deleted_at = 0
					AND (
						(targets.target_type = ? AND targets.target_id = members.space_id)
						OR (targets.target_type = ? AND targets.target_id = attempts.user_id AND %s)
					)
				`, userTargetScopePredicate), spaceIDs, constant.RoleStudent, servicespace.StatusEnabled, serviceexam.TargetTypeSpace, serviceexam.TargetTypeUser).
		Joins("JOIN spaces ON spaces.tenant_id = members.tenant_id AND spaces.id = members.space_id AND spaces.status = ? AND spaces.deleted_at = 0", servicespace.StatusEnabled).
		Joins("JOIN users ON users.id = attempts.user_id AND users.status = ? AND users.deleted_at = 0", servicetenantuser.StatusEnabled).
		Joins("JOIN tenant_user_memberships AS tum ON tum.tenant_id = attempts.tenant_id AND tum.user_id = attempts.user_id AND tum.role = ? AND tum.status = ?", constant.RoleStudent, servicetenantuser.StatusEnabled).
		Where("attempts.tenant_id = ?", tenantID).
		Where("attempts.exam_id = ?", examID).
		Scan(&rows).Error; err != nil {
		return serviceexam.AttemptOverviewStats{}, err
	}
	joined := map[uint64]struct{}{}
	submitted := map[uint64]struct{}{}
	inProgress := map[uint64]struct{}{}
	for _, row := range rows {
		joined[row.UserID] = struct{}{}
		if row.SubmittedAt != nil || row.Status == serviceexam.AttemptStatusSubmitted {
			submitted[row.UserID] = struct{}{}
			delete(inProgress, row.UserID)
			continue
		}
		if row.Status == serviceexam.AttemptStatusInProgress {
			if _, ok := submitted[row.UserID]; !ok {
				inProgress[row.UserID] = struct{}{}
			}
		}
	}
	return serviceexam.AttemptOverviewStats{
		Joined:     len(joined),
		Submitted:  len(submitted),
		InProgress: len(inProgress),
	}, nil
}

// IsEligible 判断用户是否具备参加考试的资格。
// 资格来源包括考试直投给用户，以及考试投放到用户所在的有效学生空间。
func (r *ExamRepository) IsEligible(ctx context.Context, tenantID uint64, examID uint64, userID uint64) (bool, error) {
	var directCount int64
	// 直投用户除了启用学生身份，还必须仍属于至少一个启用学生空间，和管理端考生展开口径保持一致。
	if err := r.db.WithContext(ctx).Model(&ExamTargetDO{}).
		Joins("JOIN users ON users."+UserColumns.ID+" = exam_targets."+ExamTargetColumns.TargetID+
			" AND users."+UserColumns.ID+" = ?"+
			" AND users."+UserColumns.Status+" = ?"+
			" AND users."+UserColumns.DeletedAt+" = ?", userID, "enabled", 0).
		Joins("JOIN tenant_user_memberships AS tum ON tum."+UserRoleColumns.TenantID+" = exam_targets."+ExamTargetColumns.TenantID+
			" AND tum."+UserRoleColumns.UserID+" = users."+UserColumns.ID+
			" AND tum."+UserRoleColumns.Role+" = ?"+
			" AND tum."+UserRoleColumns.Status+" = ?", "student", "enabled").
		Joins("JOIN space_members ON space_members."+SpaceMemberColumns.TenantID+" = exam_targets."+ExamTargetColumns.TenantID+
			" AND space_members."+SpaceMemberColumns.UserID+" = users."+UserColumns.ID+
			" AND space_members."+SpaceMemberColumns.RoleInSpace+" = ?"+
			" AND space_members."+SpaceMemberColumns.Status+" = ?"+
			" AND space_members."+SpaceMemberColumns.DeletedAt+" = ?", "student", "enabled", 0).
		Joins("JOIN spaces ON spaces."+SpaceColumns.TenantID+" = space_members."+SpaceMemberColumns.TenantID+
			" AND spaces."+SpaceColumns.ID+" = space_members."+SpaceMemberColumns.SpaceID+
			" AND spaces."+SpaceColumns.Status+" = ?"+
			" AND spaces."+SpaceColumns.DeletedAt+" = ?", "enabled", 0).
		Where("exam_targets."+ExamTargetColumns.TenantID+" = ?", tenantID).
		Where("exam_targets."+ExamTargetColumns.ExamID+" = ?", examID).
		Where("exam_targets."+ExamTargetColumns.TargetType+" = ?", serviceexam.TargetTypeUser).
		Where("exam_targets."+ExamTargetColumns.TargetID+" = ?", userID).
		Where(r.userTargetScopedSpacePredicate("exam_targets", "space_members."+SpaceMemberColumns.SpaceID)).
		Count(&directCount).Error; err != nil {
		return false, err
	}
	if directCount > 0 {
		return true, nil
	}

	var spaceCount int64
	// 空间投放按用户当前有效空间成员关系判断，用户和成员关系都必须处于启用状态。
	if err := r.db.WithContext(ctx).Model(&ExamTargetDO{}).
		Joins("JOIN space_members ON space_members."+SpaceMemberColumns.TenantID+" = exam_targets."+ExamTargetColumns.TenantID+
			" AND space_members."+SpaceMemberColumns.SpaceID+" = exam_targets."+ExamTargetColumns.TargetID+
			" AND space_members."+SpaceMemberColumns.UserID+" = ?"+
			" AND space_members."+SpaceMemberColumns.RoleInSpace+" = ?"+
			" AND space_members."+SpaceMemberColumns.Status+" = ?"+
			" AND space_members."+SpaceMemberColumns.DeletedAt+" = ?", userID, "student", "enabled", 0).
		Joins("JOIN users ON users."+UserColumns.ID+" = space_members."+SpaceMemberColumns.UserID+
			" AND users."+UserColumns.Status+" = ?"+
			" AND users."+UserColumns.DeletedAt+" = ?", "enabled", 0).
		Joins("JOIN tenant_user_memberships AS tum ON tum."+UserRoleColumns.TenantID+" = exam_targets."+ExamTargetColumns.TenantID+
			" AND tum."+UserRoleColumns.UserID+" = users."+UserColumns.ID+
			" AND tum."+UserRoleColumns.Status+" = ?", "enabled").
		Where("exam_targets."+ExamTargetColumns.TenantID+" = ?", tenantID).
		Where("exam_targets."+ExamTargetColumns.ExamID+" = ?", examID).
		Where("exam_targets."+ExamTargetColumns.TargetType+" = ?", serviceexam.TargetTypeSpace).
		Count(&spaceCount).Error; err != nil {
		return false, err
	}
	return spaceCount > 0, nil
}

// FindInProgressAttempt 查找考生未提交的作答记录。
// 未找到时转换为考试服务层的 ErrAttemptNotFound，让上层按业务语义决定是否新建作答。
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

// CountAttempts 统计考生在一场考试中的历史作答次数。
// 开考前用它和考试的 MaxAttempts 一起判断是否还能继续参加考试。
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

// CreateAttempt 创建一次考试作答。
// 这里只落作答主表，题目快照会在 SaveAttemptQuestions 中单独持久化，便于开考流程分步复用。
func (r *ExamRepository) CreateAttempt(ctx context.Context, attempt serviceexam.Attempt) (serviceexam.Attempt, error) {
	now := r.now()
	row := ExamAttemptDO{
		BaseFields: BaseFields{
			CreatedAt:     now,
			CreatedByType: AuditActorTenantUser,
			UpdatedAt:     now,
			UpdatedByType: AuditActorTenantUser,
			Version:       1,
			ExtJSON:       datatypes.JSON([]byte("{}")),
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
		if isUniqueConstraintError(err) {
			return serviceexam.Attempt{}, serviceexam.ErrAttemptUniqueConflict
		}
		return serviceexam.Attempt{}, err
	}
	return attemptFromDO(row), nil
}

// UpdateAttemptToken 更新作答令牌摘要及过期时间。
// 写入后按 token hash 读回最新作答，确保返回对象包含数据库侧递增后的版本等字段。
func (r *ExamRepository) UpdateAttemptToken(ctx context.Context, attempt serviceexam.Attempt) (serviceexam.Attempt, error) {
	now := r.now()
	if err := r.db.WithContext(ctx).Model(&ExamAttemptDO{}).
		Where(ExamAttemptColumns.TenantID+" = ?", attempt.TenantID).
		Where(ExamAttemptColumns.ID+" = ?", attempt.ID).
		Updates(map[string]any{
			ExamAttemptColumns.ExamTokenHash:      attempt.ExamTokenHash,
			ExamAttemptColumns.ExamTokenExpiresAt: attempt.ExamTokenExpiresAt,
			BaseColumns.UpdatedAt:                 now,
			BaseColumns.UpdatedByType:             AuditActorTenantUser,
			BaseColumns.Version:                   gorm.Expr(BaseColumns.Version + " + 1"),
		}).Error; err != nil {
		return serviceexam.Attempt{}, err
	}
	return r.FindAttemptByTokenHash(ctx, attempt.ExamTokenHash)
}

// FindAttemptByTokenHash 通过作答令牌摘要定位作答。
// 考生提交答案时只携带令牌，仓储层据此恢复 tenant、exam 和 attempt 上下文。
func (r *ExamRepository) FindAttemptByTokenHash(ctx context.Context, tokenHash string) (serviceexam.Attempt, error) {
	var row ExamAttemptDO
	if err := r.db.WithContext(ctx).
		Where(ExamAttemptColumns.ExamTokenHash+" = ?", tokenHash).
		First(&row).Error; err != nil {
		return serviceexam.Attempt{}, err
	}
	return attemptFromDO(row), nil
}

// ListFixedSnapshotQuestions 为固定组卷考试读取作答快照来源。
// 固定组卷直接使用试卷小节题目表的当前配置，按小节顺序和题目顺序生成考生看到的题目列表。
func (r *ExamRepository) ListFixedSnapshotQuestions(ctx context.Context, tenantID uint64, examID uint64) ([]serviceexam.SnapshotSourceQuestion, error) {
	var rows []struct {
		SectionID    uint64
		SectionName  string
		Instructions string
		QuestionID   uint64
		QuestionType string
		Title        string
		CorrectText  string
		Score        string
		SortOrder    int
	}
	if err := r.db.WithContext(ctx).
		Table("exams AS e").
		Select("s.id AS section_id, s.name AS section_name, s.instructions AS instructions, q.id AS question_id, q.type AS question_type, q.title AS title, q.standard_answer AS correct_text, psq.score AS score, psq.sort_order AS sort_order").
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
		// 选项和正确答案也进入作答快照，考试开始后即使题库选项变化也不影响本次作答判分。
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
			CorrectText:      row.CorrectText,
			Score:            row.Score,
			Options:          options.options,
			BlankCount:       fillBlankCount(row.QuestionType, row.CorrectText),
			OptionIDs:        options.optionIDs,
			CorrectOptionIDs: options.correctOptionIDs,
		})
	}
	return items, nil
}

// ListFrozenLiveSnapshotQuestions 为动态组卷考试读取发布时冻结的作答快照来源。
// 动态组卷不再实时读取规则命中的题目，而是使用发布事务写入的冻结题池，保证所有考生题目一致。
func (r *ExamRepository) ListFrozenLiveSnapshotQuestions(ctx context.Context, tenantID uint64, examID uint64) ([]serviceexam.SnapshotSourceQuestion, error) {
	var rows []struct {
		SectionID    uint64
		SectionName  string
		Instructions string
		QuestionID   uint64
		QuestionType string
		Title        string
		CorrectText  string
		Score        string
	}
	if err := r.db.WithContext(ctx).
		Table(ExamLiveQuestionPoolDO{}.TableName()+" AS pool").
		Select("sections.id AS section_id, sections.name AS section_name, sections.instructions AS instructions, questions.id AS question_id, questions.type AS question_type, questions.title AS title, questions.standard_answer AS correct_text, rules.score_per_question AS score").
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
		// 冻结池只保存题目归属，题目内容和选项在开考时一起写入 attempt_questions 快照。
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
			CorrectText:      row.CorrectText,
			Score:            row.Score,
			Options:          options.options,
			BlankCount:       fillBlankCount(row.QuestionType, row.CorrectText),
			OptionIDs:        options.optionIDs,
			CorrectOptionIDs: options.correctOptionIDs,
		})
	}
	return items, nil
}

// SaveAttemptQuestions 保存一次作答对应的题目快照。
// 如果同一 attempt 已经存在快照，直接返回现有数据，使开考接口在重试时保持幂等。
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
		// 小节、题干、选项和正确答案都保存为 JSON 快照，后续题库编辑不会影响本次作答展示和判分。
		rows = append(rows, ExamAttemptQuestionDO{
			BaseFields: BaseFields{
				CreatedAt:     now,
				CreatedByType: AuditActorTenantUser,
				UpdatedAt:     now,
				UpdatedByType: AuditActorTenantUser,
				Version:       1,
				ExtJSON:       datatypes.JSON([]byte("{}")),
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

// ListPendingAttempts 列出一场考试中待人工评分的简答题答案。
// 每一行对应一个待阅答案，并携带考生、空间、考试、题目快照和答案版本信息。
func (r *ExamRepository) ListPendingAttempts(ctx context.Context, tenantID uint64, examID uint64, keyword string) ([]serviceexam.PendingAttempt, error) {
	var rows []pendingReviewRow
	// 待阅答案主查询不直接关联空间成员，避免多空间考生把同一答案放大为多行。
	// 本次考试实际命中的投放空间会在结果组装阶段按 attempt 单独读取。
	query := r.db.WithContext(ctx).Table("exam_answers AS answers").
		Select(`
				attempts.id AS attempt_id,
				attempt_questions.id AS attempt_question_id,
			attempts.exam_id AS exam_id,
			attempts.user_id AS user_id,
			COALESCE(users.real_name, users.username) AS student_name,
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
		Joins("JOIN users ON users.id = attempts.user_id AND users.deleted_at = 0").
		Where("answers.tenant_id = ?", tenantID).
		Where("attempts.exam_id = ?", examID).
		Where("answers.grading_status = ?", constant.GradingStatusPending)
	query = r.applyPendingAttemptSearch(query, keyword)
	if err := query.
		Order("attempts.submitted_at ASC, attempt_questions.sort_order ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	countByAttempt := make(map[uint64]int, len(rows))
	for _, row := range rows {
		// PendingShortTextCount 表示同一次作答还剩多少简答题待阅，便于阅卷列表展示进度。
		countByAttempt[row.AttemptID]++
	}
	items := make([]serviceexam.PendingAttempt, 0, len(rows))
	spaceCache := make(map[uint64][]attemptTargetSpaceRow)
	for _, row := range rows {
		// 题目标题从作答快照读取，而不是从题库表回查，避免题库改名影响历史阅卷记录。
		title, err := parseQuestionSnapshotTitle(row.QuestionSnapshot)
		if err != nil {
			return nil, err
		}
		spaces, err := r.cachedAttemptTargetSpaces(ctx, tenantID, row.AttemptID, spaceCache)
		if err != nil {
			return nil, err
		}
		if len(spaces) == 0 {
			continue
		}
		spaceID, spaceName := firstAttemptTargetSpace(spaces)
		items = append(items, serviceexam.PendingAttempt{
			AttemptID:             row.AttemptID,
			AttemptQuestionID:     row.AttemptQuestionID,
			ExamID:                row.ExamID,
			UserID:                row.UserID,
			StudentName:           row.StudentName,
			SpaceID:               spaceID,
			SpaceName:             spaceName,
			SpaceIDs:              targetSpaceIDs(spaces),
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

func (r *ExamRepository) applyPendingAttemptSearch(query *gorm.DB, keyword string) *gorm.DB {
	value := strings.TrimSpace(keyword)
	if value == "" {
		return query
	}
	pattern := likeIgnoreCasePattern(value)
	userTargetScopePredicate := r.userTargetScopedSpacePredicate("targets", "members.space_id")
	condition := LikeIgnoreCase(r.db, "COALESCE(users.real_name, users.username)", value).
		Or(LikeIgnoreCase(r.db, "users.username", value)).
		Or(LikeIgnoreCase(r.db, "exams.name", value)).
		Or(LikeIgnoreCase(r.db, "attempt_questions.question_snapshot", value)).
		Or(LikeIgnoreCase(r.db, "answers.answer_content", value)).
		Or(fmt.Sprintf(`
					EXISTS (
						SELECT 1
						FROM exam_targets AS targets
					JOIN space_members AS members
						ON members.tenant_id = attempts.tenant_id
						AND members.user_id = attempts.user_id
						AND members.status = ?
						AND members.role_in_space = ?
						AND members.deleted_at = 0
						AND (
							(targets.target_type = ? AND members.space_id = targets.target_id)
							OR (targets.target_type = ? AND targets.target_id = attempts.user_id AND %s)
						)
					JOIN tenant_user_memberships AS tum
						ON tum.tenant_id = members.tenant_id
						AND tum.user_id = members.user_id
						AND tum.role = ?
						AND tum.status = ?
					JOIN spaces
						ON spaces.tenant_id = members.tenant_id
						AND spaces.id = members.space_id
						AND spaces.status = ?
						AND spaces.deleted_at = 0
					WHERE targets.tenant_id = attempts.tenant_id
						AND targets.exam_id = attempts.exam_id
						AND %s
					)
				`, userTargetScopePredicate, likeIgnoreCaseClause(r.db, "spaces.name")), servicespace.StatusEnabled, constant.RoleStudent, serviceexam.TargetTypeSpace, serviceexam.TargetTypeUser, constant.RoleStudent, servicetenantuser.StatusEnabled, servicespace.StatusEnabled, pattern)
	return query.Where(condition)
}

// AttemptSpaceIDs 读取作答考生在本次考试中实际命中的投放空间。
// 阅卷权限检查只看考试目标空间，避免考生其他空间的教师越权查看答案。
func (r *ExamRepository) AttemptSpaceIDs(ctx context.Context, tenantID uint64, attemptID uint64) ([]uint64, error) {
	exists, err := r.attemptExists(ctx, tenantID, attemptID)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, serviceexam.ErrAttemptNotFound
	}
	spaces, err := r.listAttemptTargetSpaces(ctx, tenantID, attemptID)
	if err != nil {
		return nil, err
	}
	return targetSpaceIDs(spaces), nil
}

func (r *ExamRepository) attemptExists(ctx context.Context, tenantID uint64, attemptID uint64) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&ExamAttemptDO{}).
		Where(ExamAttemptColumns.TenantID+" = ?", tenantID).
		Where(ExamAttemptColumns.ID+" = ?", attemptID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

type attemptTargetSpaceRow struct {
	SpaceID   uint64
	SpaceName string
}

// cachedAttemptTargetSpaces 缓存同一批结果中重复出现的 attempt 空间归属查询。
// 同一次作答可能有多道待阅题，缓存可以避免为每一道题重复查询考试目标空间。
func (r *ExamRepository) cachedAttemptTargetSpaces(ctx context.Context, tenantID uint64, attemptID uint64, cache map[uint64][]attemptTargetSpaceRow) ([]attemptTargetSpaceRow, error) {
	if spaces, ok := cache[attemptID]; ok {
		return spaces, nil
	}
	spaces, err := r.listAttemptTargetSpaces(ctx, tenantID, attemptID)
	if err != nil {
		return nil, err
	}
	cache[attemptID] = spaces
	return spaces, nil
}

// listAttemptTargetSpaces 查找作答实际命中的考试投放空间。
// 空间投放只命中目标空间；用户直投优先命中发布时持久化的 scoped spaces。
func (r *ExamRepository) listAttemptTargetSpaces(ctx context.Context, tenantID uint64, attemptID uint64) ([]attemptTargetSpaceRow, error) {
	var rows []attemptTargetSpaceRow
	userTargetScopePredicate := r.userTargetScopedSpacePredicate("targets", "members.space_id")
	if err := r.db.WithContext(ctx).Table("exam_attempts AS attempts").
		Select("DISTINCT spaces.id AS space_id, spaces.name AS space_name").
		Joins("JOIN exam_targets AS targets ON targets.tenant_id = attempts.tenant_id AND targets.exam_id = attempts.exam_id").
		Joins(fmt.Sprintf(`
			JOIN space_members AS members
				ON members.tenant_id = attempts.tenant_id
				AND members.user_id = attempts.user_id
				AND members.status = ?
				AND members.role_in_space = ?
				AND members.deleted_at = 0
				AND (
					(targets.target_type = ? AND members.space_id = targets.target_id)
					OR (targets.target_type = ? AND targets.target_id = attempts.user_id AND %s)
				)
		`, userTargetScopePredicate), servicespace.StatusEnabled, constant.RoleStudent, serviceexam.TargetTypeSpace, serviceexam.TargetTypeUser).
		Joins("JOIN spaces ON spaces.tenant_id = members.tenant_id AND spaces.id = members.space_id AND spaces.status = ? AND spaces.deleted_at = 0", servicespace.StatusEnabled).
		Joins("JOIN users ON users.id = members.user_id AND users.status = ? AND users.deleted_at = 0", servicetenantuser.StatusEnabled).
		Joins("JOIN tenant_user_memberships AS tum ON tum.tenant_id = members.tenant_id AND tum.user_id = members.user_id AND tum.role = ? AND tum.status = ?", constant.RoleStudent, servicetenantuser.StatusEnabled).
		Where("attempts.tenant_id = ?", tenantID).
		Where("attempts.id = ?", attemptID).
		Order("spaces.id ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) > 0 {
		return rows, nil
	}
	// 历史已交卷记录不能因为考生后来退空间就失去展示空间。
	// 这里只允许“整场考试只有一个显式空间目标且不存在任何用户直投目标”时回退，
	// 避免混合投放下把用户直投考生错误归到某个空间目标。
	return r.listAttemptSingleTargetSpaceFallback(ctx, tenantID, attemptID)
}

func (r *ExamRepository) listAttemptSingleTargetSpaceFallback(ctx context.Context, tenantID uint64, attemptID uint64) ([]attemptTargetSpaceRow, error) {
	var rows []attemptTargetSpaceRow
	if err := r.db.WithContext(ctx).Table("exam_attempts AS attempts").
		Select("DISTINCT spaces.id AS space_id, spaces.name AS space_name").
		Joins(`
			JOIN exam_targets AS targets ON targets.tenant_id = attempts.tenant_id
				AND targets.exam_id = attempts.exam_id
				AND targets.target_type = ?
		`, serviceexam.TargetTypeSpace).
		Joins("JOIN spaces ON spaces.tenant_id = targets.tenant_id AND spaces.id = targets.target_id AND spaces.status = ? AND spaces.deleted_at = 0", servicespace.StatusEnabled).
		Joins("JOIN tenant_user_memberships AS tum ON tum.tenant_id = attempts.tenant_id AND tum.user_id = attempts.user_id AND tum.role = ? AND tum.status = ?", constant.RoleStudent, servicetenantuser.StatusEnabled).
		Where("attempts.tenant_id = ?", tenantID).
		Where("attempts.id = ?", attemptID).
		Where(`
			NOT EXISTS (
				SELECT 1
				FROM exam_targets AS user_targets
				WHERE user_targets.tenant_id = attempts.tenant_id
					and user_targets.exam_id = attempts.exam_id
					and user_targets.target_type = ?
			)
		`, serviceexam.TargetTypeUser).
		Where(`
			NOT EXISTS (
				SELECT 1
				FROM space_members AS current_members
				WHERE current_members.tenant_id = attempts.tenant_id
					and current_members.user_id = attempts.user_id
					and current_members.space_id = targets.target_id
					and current_members.status = ?
					and current_members.deleted_at = 0
					and current_members.role_in_space <> ?
			)
		`, servicespace.StatusEnabled, constant.RoleStudent).
		Order("spaces.id ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) != 1 {
		return nil, nil
	}
	return rows, nil
}

func firstAttemptTargetSpace(spaces []attemptTargetSpaceRow) (uint64, string) {
	if len(spaces) == 0 {
		return 0, ""
	}
	return spaces[0].SpaceID, spaces[0].SpaceName
}

func firstAttemptTargetSpaceInScope(spaces []attemptTargetSpaceRow, scopeSpaceIDs []uint64) (uint64, string) {
	if len(scopeSpaceIDs) == 0 {
		return firstAttemptTargetSpace(spaces)
	}
	scope := make(map[uint64]struct{}, len(scopeSpaceIDs))
	for _, spaceID := range scopeSpaceIDs {
		scope[spaceID] = struct{}{}
	}
	for _, space := range spaces {
		if _, ok := scope[space.SpaceID]; ok {
			return space.SpaceID, space.SpaceName
		}
	}
	return 0, ""
}

func targetSpaceIDs(spaces []attemptTargetSpaceRow) []uint64 {
	ids := make([]uint64, 0, len(spaces))
	for _, space := range spaces {
		ids = append(ids, space.SpaceID)
	}
	return ids
}

func targetSpaceNames(spaces []attemptTargetSpaceRow) map[uint64]string {
	names := make(map[uint64]string, len(spaces))
	for _, space := range spaces {
		names[space.SpaceID] = space.SpaceName
	}
	return names
}

// GradeShortTextAndRecalculate 保存简答题人工评分并重算总分。
// 人工评分和 attempt 分数回写放在同一个事务里，避免答案已评分但总分仍是旧值。
func (r *ExamRepository) GradeShortTextAndRecalculate(ctx context.Context, grade serviceexam.ShortTextGrade) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := grade.GradedAt
		gradeScore, err := parseScoreString(grade.Score)
		if err != nil || math.IsNaN(gradeScore) || math.IsInf(gradeScore, 0) || gradeScore < 0 {
			return serviceexam.ErrInvalidGradeScore
		}
		var attemptQuestion ExamAttemptQuestionDO
		if err := tx.Where(ExamAttemptQuestionColumns.TenantID+" = ?", grade.TenantID).
			Where(ExamAttemptQuestionColumns.AttemptID+" = ?", grade.AttemptID).
			Where(ExamAttemptQuestionColumns.ID+" = ?", grade.AttemptQuestionID).
			First(&attemptQuestion).Error; err != nil {
			return err
		}
		maxScore, err := parseScoreString(attemptQuestion.Score)
		if err != nil || math.IsNaN(maxScore) || math.IsInf(maxScore, 0) || maxScore < 0 || gradeScore > maxScore {
			return serviceexam.ErrInvalidGradeScore
		}
		var attempt ExamAttemptDO
		if err := tx.Where(ExamAttemptColumns.TenantID+" = ?", grade.TenantID).
			Where(ExamAttemptColumns.ID+" = ?", grade.AttemptID).
			First(&attempt).Error; err != nil {
			return err
		}
		if grade.ExamID != 0 && grade.ExamID != attempt.ExamID {
			return gorm.ErrRecordNotFound
		}

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
				BaseColumns.UpdatedByType:       AuditActorTenantUser,
				BaseColumns.Version:             gorm.Expr(BaseColumns.Version + " + 1"),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return serviceexam.ErrAnswerVersionConflict
		}

		// 主观题总分只统计已经人工评分完成的答案，仍处于 pending 的简答题不提前计入总分。
		subjectiveScore, err := sumGradedSubjectiveScore(tx, grade.TenantID, grade.AttemptID)
		if err != nil {
			return err
		}
		objectiveScore, err := parseScoreString(attempt.ObjectiveScore)
		if err != nil {
			return err
		}
		// 总分由提交时的客观题分加上当前已完成评分的主观题分组成。
		if err := tx.Model(&ExamAttemptDO{}).
			Where(ExamAttemptColumns.TenantID+" = ?", grade.TenantID).
			Where(ExamAttemptColumns.ID+" = ?", grade.AttemptID).
			Updates(map[string]any{
				ExamAttemptColumns.SubjectiveScore: formatScoreString(subjectiveScore),
				ExamAttemptColumns.TotalScore:      formatScoreString(objectiveScore + subjectiveScore),
				BaseColumns.UpdatedAt:              now,
				BaseColumns.UpdatedByType:          AuditActorTenantUser,
				BaseColumns.Version:                gorm.Expr(BaseColumns.Version + " + 1"),
			}).Error; err != nil {
			return err
		}
		if grade.ExamID == 0 {
			return nil
		}
		// 阅卷日志必须和答案评分、总分重算同事务提交，避免成绩变化缺少可追溯记录。
		return r.appendOperationLogsForSpacesInTx(tx, serviceexam.OperationLog{
			TenantID:        grade.TenantID,
			ExamID:          attempt.ExamID,
			OperationType:   serviceexam.OperationTypeGradeAnswer,
			OperationTitle:  "人工阅卷",
			OperationDetail: "保存主观题评分",
			ActorID:         grade.GradedBy,
			ActorType:       grade.GraderType,
			ActorRole:       grade.GraderRole,
			CreatedAt:       grade.GradedAt,
			CreatedBy:       grade.GradedBy,
			CreatedByType:   grade.GraderType,
		}, grade.SpaceIDs)
	})
}

// ListScoreExportRows 读取成绩导出所需的已提交作答记录。
// 只导出 submitted_at 非空的作答，避免未提交或异常中断的作答进入成绩文件。
func (r *ExamRepository) ListScoreExportRows(ctx context.Context, tenantID uint64, examID uint64) ([]serviceexam.ScoreExportRow, error) {
	var rows []scoreExportRow
	// 成绩主查询只读取 attempt 和用户信息，避免空间成员关系把成绩行放大。
	// 展示空间和权限空间随后按考试实际投放目标补齐。
	if err := r.db.WithContext(ctx).Table("exam_attempts AS attempts").
		Select(`
			attempts.id AS attempt_id,
			COALESCE(users.real_name, users.username) AS student_name,
			attempts.attempt_no AS attempt_no,
			attempts.objective_score AS objective_score,
			attempts.subjective_score AS subjective_score,
			attempts.total_score AS total_score,
			attempts.submitted_at AS submitted_at
		`).
		Joins("JOIN users ON users.id = attempts.user_id AND users.deleted_at = 0").
		Where("attempts.tenant_id = ?", tenantID).
		Where("attempts.exam_id = ?", examID).
		Where("attempts.submitted_at IS NOT NULL").
		Order("attempts.submitted_at ASC, attempts.id ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]serviceexam.ScoreExportRow, 0, len(rows))
	spaceCache := make(map[uint64][]attemptTargetSpaceRow)
	for _, row := range rows {
		spaces, err := r.cachedAttemptTargetSpaces(ctx, tenantID, row.AttemptID, spaceCache)
		if err != nil {
			return nil, err
		}
		if len(spaces) == 0 {
			continue
		}
		spaceID, spaceName := firstAttemptTargetSpace(spaces)
		items = append(items, serviceexam.ScoreExportRow{
			StudentName:     row.StudentName,
			SpaceID:         spaceID,
			SpaceName:       spaceName,
			SpaceIDs:        targetSpaceIDs(spaces),
			SpaceNames:      targetSpaceNames(spaces),
			AttemptNo:       row.AttemptNo,
			ObjectiveScore:  row.ObjectiveScore,
			SubjectiveScore: row.SubjectiveScore,
			TotalScore:      row.TotalScore,
			SubmittedAt:     row.SubmittedAtValue(),
		})
	}
	return items, nil
}

// SummarizeExamResults 聚合管理端成绩摘要。
// 聚合在数据库侧完成，避免前端或 service 拉全量成绩后再计算统计卡片。
func (r *ExamRepository) SummarizeExamResults(ctx context.Context, input serviceexam.ResultSummaryRepositoryInput) (serviceexam.ResultSummaryRepositoryResult, error) {
	var row examResultSummaryRow
	if err := r.managementResultBaseQuery(ctx, input.TenantID, input.ExamID, input.SpaceIDs).
		Select(`
			COUNT(*) AS submitted,
			COALESCE(AVG(CAST(attempts.total_score AS REAL)), 0) AS average_score,
			COALESCE(MAX(CAST(attempts.total_score AS REAL)), 0) AS highest_score,
			COALESCE(SUM(CASE WHEN CAST(attempts.total_score AS REAL) >= ? THEN 1 ELSE 0 END), 0) AS passed
		`, input.PassScore).
		Scan(&row).Error; err != nil {
		return serviceexam.ResultSummaryRepositoryResult{}, err
	}
	var pendingSubjective int64
	if err := r.managementResultBaseQuery(ctx, input.TenantID, input.ExamID, input.SpaceIDs).
		Where(`
			EXISTS (
					SELECT 1 FROM exam_answers AS answers
					WHERE answers.tenant_id = attempts.tenant_id
						and answers.attempt_id = attempts.id
						and answers.grading_status = ?
				)
			`, constant.GradingStatusPending).
		Count(&pendingSubjective).Error; err != nil {
		return serviceexam.ResultSummaryRepositoryResult{}, err
	}
	scoreDistribution, err := r.summarizeResultScoreDistribution(ctx, input)
	if err != nil {
		return serviceexam.ResultSummaryRepositoryResult{}, err
	}
	questionTypeRates, err := r.summarizeResultQuestionTypeRates(ctx, input)
	if err != nil {
		return serviceexam.ResultSummaryRepositoryResult{}, err
	}
	return serviceexam.ResultSummaryRepositoryResult{
		Submitted:         int(row.Submitted),
		AverageScore:      formatExamRepositoryScore(row.AverageScore),
		HighestScore:      formatExamRepositoryScore(row.HighestScore),
		Passed:            int(row.Passed),
		PendingSubjective: int(pendingSubjective),
		ScoreDistribution: scoreDistribution,
		QuestionTypeRates: questionTypeRates,
	}, nil
}

// summarizeResultScoreDistribution 按授权范围内真实已交卷成绩聚合分数段。
// 这里读取的是 attempt.total_score，不依赖前端页内数据，避免分页列表缺行导致图表统计失真。
func (r *ExamRepository) summarizeResultScoreDistribution(ctx context.Context, input serviceexam.ResultSummaryRepositoryInput) ([]serviceexam.ResultScoreDistribution, error) {
	var rows []managementResultScoreRow
	if err := r.managementResultBaseQuery(ctx, input.TenantID, input.ExamID, input.SpaceIDs).
		Select("CAST(attempts.total_score AS REAL) AS total_score").
		Order("CAST(attempts.total_score AS REAL) ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []serviceexam.ResultScoreDistribution{}, nil
	}
	distribution := make([]serviceexam.ResultScoreDistribution, 0, len(rows))
	indexByLabel := make(map[string]int, len(rows))
	for _, row := range rows {
		label := resultScoreBucketLabel(row.TotalScore, input.TotalScore)
		index, ok := indexByLabel[label]
		if !ok {
			indexByLabel[label] = len(distribution)
			distribution = append(distribution, serviceexam.ResultScoreDistribution{Label: label})
			index = len(distribution) - 1
		}
		distribution[index].Count++
	}
	return distribution, nil
}

// summarizeResultQuestionTypeRates 按题目快照和答案得分计算题型平均得分率。
// 题型来自 exam_attempt_questions.question_snapshot，确保考试后题库类型被编辑也不会影响历史统计。
func (r *ExamRepository) summarizeResultQuestionTypeRates(ctx context.Context, input serviceexam.ResultSummaryRepositoryInput) ([]serviceexam.ResultQuestionTypeRate, error) {
	var rows []managementResultQuestionScoreRow
	submittedAttempts := r.managementResultBaseQuery(ctx, input.TenantID, input.ExamID, input.SpaceIDs).
		Select("attempts.id AS id")
	if err := r.db.WithContext(ctx).Table("exam_attempt_questions AS attempt_questions").
		Select(`
			attempt_questions.question_snapshot AS question_snapshot,
			attempt_questions.score AS max_score,
			COALESCE(answers.score, '') AS earned_score
		`).
		Joins("JOIN (?) AS submitted_attempts ON submitted_attempts.id = attempt_questions.attempt_id", submittedAttempts).
		Joins(`
			LEFT JOIN exam_answers AS answers ON answers.tenant_id = attempt_questions.tenant_id
				AND answers.attempt_id = attempt_questions.attempt_id
				AND answers.attempt_question_id = attempt_questions.id
		`).
		Where("attempt_questions.tenant_id = ?", input.TenantID).
		Order("attempt_questions.sort_order ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	type aggregate struct {
		earned   float64
		possible float64
	}
	order := make([]string, 0)
	aggregates := make(map[string]*aggregate)
	for _, row := range rows {
		questionType, err := parseQuestionSnapshotType(datatypes.JSON(row.QuestionSnapshot))
		if err != nil {
			return nil, err
		}
		if _, ok := aggregates[questionType]; !ok {
			order = append(order, questionType)
			aggregates[questionType] = &aggregate{}
		}
		possible, err := parseScoreString(row.MaxScore)
		if err != nil {
			return nil, err
		}
		earned, err := parseScoreString(row.EarnedScore)
		if err != nil {
			return nil, err
		}
		aggregates[questionType].possible += possible
		aggregates[questionType].earned += earned
	}
	items := make([]serviceexam.ResultQuestionTypeRate, 0, len(order))
	for _, questionType := range order {
		item := aggregates[questionType]
		averageRate := 0
		if item.possible > 0 {
			averageRate = int(math.Round(item.earned / item.possible * 100))
		}
		items = append(items, serviceexam.ResultQuestionTypeRate{
			QuestionType:      questionType,
			QuestionTypeLabel: resultQuestionTypeLabel(questionType),
			AverageRate:       averageRate,
		})
	}
	return items, nil
}

// ListExamResults 按授权范围分页读取成绩管理列表。
// 排名基于同一授权范围下的稳定排序和分页 offset 生成，不能在 handler 里用当前页下标重置。
func (r *ExamRepository) ListExamResults(ctx context.Context, input serviceexam.ListExamResultsInput) (pagination.Result[serviceexam.ExamResult], error) {
	page := pagination.Normalize(pagination.Input{Page: input.Page, PageSize: input.PageSize})
	exam, err := r.GetExam(ctx, input.TenantID, input.ExamID)
	if err != nil {
		return pagination.Result[serviceexam.ExamResult]{}, err
	}
	resultsVisible := examResultsVisibleForManagement(exam, r.now())
	query := r.managementResultBaseQuery(ctx, input.TenantID, input.ExamID, input.SpaceIDs)
	if strings.TrimSpace(input.Keyword) != "" {
		keyword := "%" + strings.TrimSpace(input.Keyword) + "%"
		userTargetScopePredicate := r.userTargetScopedSpacePredicate("targets", "members.space_id")
		query = query.Where(fmt.Sprintf(`
			(
				users.username LIKE ?
				OR users.real_name LIKE ?
				OR EXISTS (
					SELECT 1
					FROM exam_targets AS targets
					JOIN space_members AS members
							ON members.tenant_id = attempts.tenant_id
							and members.user_id = attempts.user_id
							and members.space_id IN ?
							and members.status = ?
							and members.role_in_space = ?
							and members.deleted_at = 0
							and (
								(targets.target_type = ? and members.space_id = targets.target_id)
								OR (targets.target_type = ? and targets.target_id = attempts.user_id and %s)
							)
					JOIN tenant_user_memberships AS tum
							ON tum.tenant_id = members.tenant_id
							and tum.user_id = members.user_id
							and tum.role = ?
							and tum.status = ?
					JOIN spaces
							ON spaces.tenant_id = members.tenant_id
							and spaces.id = members.space_id
							and spaces.status = ?
							and spaces.deleted_at = 0
					WHERE targets.tenant_id = attempts.tenant_id
						and targets.exam_id = attempts.exam_id
						and spaces.name LIKE ?
				)
			)
		`, userTargetScopePredicate), keyword, keyword, input.SpaceIDs, servicespace.StatusEnabled, constant.RoleStudent, serviceexam.TargetTypeSpace, serviceexam.TargetTypeUser, constant.RoleStudent, servicetenantuser.StatusEnabled, servicespace.StatusEnabled, keyword)
	}
	switch input.Status {
	case "pending_review":
		query = query.Where(`
			EXISTS (
					SELECT 1 FROM exam_answers AS answers
					WHERE answers.tenant_id = attempts.tenant_id
						and answers.attempt_id = attempts.id
						and answers.grading_status = ?
				)
			`, constant.GradingStatusPending)
	case "published":
		query = query.Where(`
			NOT EXISTS (
					SELECT 1 FROM exam_answers AS answers
					WHERE answers.tenant_id = attempts.tenant_id
						and answers.attempt_id = attempts.id
						and answers.grading_status = ?
				)
			`, constant.GradingStatusPending)
		if !resultsVisible {
			query = query.Where("1 = 0")
		}
	case "pending_publish":
		query = query.Where(`
			NOT EXISTS (
					SELECT 1 FROM exam_answers AS answers
					WHERE answers.tenant_id = attempts.tenant_id
						and answers.attempt_id = attempts.id
						and answers.grading_status = ?
				)
			`, constant.GradingStatusPending)
		if resultsVisible {
			query = query.Where("1 = 0")
		}
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return pagination.Result[serviceexam.ExamResult]{}, err
	}
	var rows []managementResultRow
	if err := query.
		Select(`
			attempts.id AS attempt_id,
			attempts.user_id AS user_id,
			users.username AS username,
			users.real_name AS real_name,
			attempts.objective_score AS objective_score,
			attempts.subjective_score AS subjective_score,
			attempts.total_score AS total_score,
			attempts.submitted_at AS submitted_at,
			(
				SELECT COUNT(1) FROM exam_answers AS answers
				WHERE answers.tenant_id = attempts.tenant_id
					AND answers.attempt_id = attempts.id
					AND answers.grading_status = ?
			) AS pending_subjective_count
		`, constant.GradingStatusPending).
		Order("CAST(attempts.total_score AS REAL) DESC").
		Order("attempts.submitted_at ASC").
		Order("attempts.id ASC").
		Limit(page.PageSize).
		Offset(pagination.Offset(page)).
		Scan(&rows).Error; err != nil {
		return pagination.Result[serviceexam.ExamResult]{}, err
	}
	items := make([]serviceexam.ExamResult, 0, len(rows))
	spaceCache := make(map[uint64][]attemptTargetSpaceRow)
	offset := pagination.Offset(page)
	for index, row := range rows {
		spaces, err := r.cachedAttemptTargetSpaces(ctx, input.TenantID, row.AttemptID, spaceCache)
		if err != nil {
			return pagination.Result[serviceexam.ExamResult]{}, err
		}
		spaceID, spaceName := firstAttemptTargetSpaceInScope(spaces, input.SpaceIDs)
		status := "published"
		if row.PendingSubjectiveCount > 0 {
			status = "pending_review"
		} else if !resultsVisible {
			status = "pending_publish"
		}
		items = append(items, serviceexam.ExamResult{
			Rank:            uint64(offset + index + 1),
			AttemptID:       row.AttemptID,
			UserID:          row.UserID,
			Username:        row.Username,
			RealName:        row.RealName,
			SpaceID:         spaceID,
			SpaceName:       spaceName,
			ObjectiveScore:  row.ObjectiveScore,
			SubjectiveScore: row.SubjectiveScore,
			TotalScore:      row.TotalScore,
			Status:          status,
			SubmittedAt:     row.SubmittedAtValue(),
		})
	}
	return pagination.Result[serviceexam.ExamResult]{
		Items:    items,
		Page:     page.Page,
		PageSize: page.PageSize,
		Total:    total,
	}, nil
}

func examResultsVisibleForManagement(exam serviceexam.Exam, now int64) bool {
	if exam.PublishMode != serviceexam.PublishModeManualPublish {
		return true
	}
	return exam.ScorePublishTime != nil && now >= *exam.ScorePublishTime
}

// GetAnswerSheet 按 examID + attemptID 双重定位管理端答卷详情。
// 基础作答查询复用成绩列表授权范围，避免空间管理员或教师跨空间读取其它考生答卷。
func (r *ExamRepository) GetAnswerSheet(ctx context.Context, input serviceexam.AnswerSheetRepositoryInput) (serviceexam.AnswerSheetRepositoryResult, error) {
	var attemptRow managementAnswerSheetAttemptRow
	if err := r.managementResultBaseQuery(ctx, input.TenantID, input.ExamID, input.SpaceIDs).
		Where("attempts.id = ?", input.AttemptID).
		Select(`
			attempts.id AS attempt_id,
			attempts.exam_id AS exam_id,
			attempts.user_id AS user_id,
			users.username AS username,
			users.real_name AS real_name,
			attempts.objective_score AS objective_score,
			attempts.subjective_score AS subjective_score,
			attempts.total_score AS total_score,
			attempts.submitted_at AS submitted_at
		`).
		Scan(&attemptRow).Error; err != nil {
		return serviceexam.AnswerSheetRepositoryResult{}, err
	}
	if attemptRow.AttemptID == 0 {
		return serviceexam.AnswerSheetRepositoryResult{}, gorm.ErrRecordNotFound
	}
	items, err := r.listManagementAnswerSheetItems(ctx, input.TenantID, input.AttemptID)
	if err != nil {
		return serviceexam.AnswerSheetRepositoryResult{}, err
	}
	return serviceexam.AnswerSheetRepositoryResult{
		Attempt: serviceexam.AnswerSheetAttempt{
			AttemptID:       attemptRow.AttemptID,
			ExamID:          attemptRow.ExamID,
			UserID:          attemptRow.UserID,
			Username:        attemptRow.Username,
			RealName:        attemptRow.RealName,
			ObjectiveScore:  attemptRow.ObjectiveScore,
			SubjectiveScore: attemptRow.SubjectiveScore,
			TotalScore:      attemptRow.TotalScore,
			SubmittedAt:     attemptRow.SubmittedAtValue(),
		},
		Items: items,
	}, nil
}

// listManagementAnswerSheetItems 读取单次作答的题目快照和答案。
// 答卷详情必须展示历史快照，因此不回查 questions 或 question_options 当前状态。
func (r *ExamRepository) listManagementAnswerSheetItems(ctx context.Context, tenantID uint64, attemptID uint64) ([]serviceexam.AnswerSheetItem, error) {
	var rows []managementAnswerSheetItemRow
	if err := r.db.WithContext(ctx).Table("exam_attempt_questions AS attempt_questions").
		Select(`
			attempt_questions.id AS attempt_question_id,
			attempt_questions.section_id AS section_id,
			attempt_questions.question_id AS question_id,
			attempt_questions.sort_order AS sort_order,
			attempt_questions.section_snapshot AS section_snapshot,
			attempt_questions.question_snapshot AS question_snapshot,
			attempt_questions.option_snapshot AS option_snapshot,
			attempt_questions.correct_answer_snapshot AS correct_answer_snapshot,
			attempt_questions.score AS score,
			COALESCE(answers.answer_content, '') AS answer_content,
			COALESCE(answers.score, '') AS answer_score,
			COALESCE(answers.grading_status, '') AS grading_status,
			COALESCE(answers.grader_comment, '') AS grader_comment,
			COALESCE(answers.graded_by, 0) AS graded_by,
			answers.graded_at AS graded_at
		`).
		Joins(`
			LEFT JOIN exam_answers AS answers ON answers.tenant_id = attempt_questions.tenant_id
				AND answers.attempt_id = attempt_questions.attempt_id
				AND answers.attempt_question_id = attempt_questions.id
		`).
		Where("attempt_questions.tenant_id = ?", tenantID).
		Where("attempt_questions.attempt_id = ?", attemptID).
		Order("attempt_questions.sort_order ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]serviceexam.AnswerSheetItem, 0, len(rows))
	for _, row := range rows {
		questionType, err := parseQuestionSnapshotType(datatypes.JSON(row.QuestionSnapshot))
		if err != nil {
			return nil, err
		}
		questionTitle, err := parseQuestionSnapshotTitle(row.QuestionSnapshot)
		if err != nil {
			return nil, err
		}
		items = append(items, serviceexam.AnswerSheetItem{
			AttemptQuestionID:     row.AttemptQuestionID,
			SectionID:             row.SectionID,
			QuestionID:            row.QuestionID,
			SortOrder:             row.SortOrder,
			SectionSnapshot:       row.SectionSnapshot,
			QuestionSnapshot:      row.QuestionSnapshot,
			QuestionType:          questionType,
			QuestionTitle:         questionTitle,
			OptionSnapshot:        row.OptionSnapshot,
			CorrectAnswerSnapshot: row.CorrectAnswerSnapshot,
			Score:                 row.Score,
			AnswerContent:         row.AnswerContent,
			AnswerScore:           row.AnswerScore,
			GradingStatus:         row.GradingStatus,
			GraderComment:         row.GraderComment,
			GradedBy:              row.GradedBy,
			GradedAt:              row.GradedAt,
		})
	}
	return items, nil
}

func (r *ExamRepository) managementResultBaseQuery(ctx context.Context, tenantID uint64, examID uint64, spaceIDs []uint64) *gorm.DB {
	query := r.db.WithContext(ctx).Table("exam_attempts AS attempts").
		Joins("JOIN users ON users.id = attempts.user_id AND users.deleted_at = 0").
		Where("attempts.tenant_id = ?", tenantID).
		Where("attempts.exam_id = ?", examID).
		Where("attempts.submitted_at IS NOT NULL")
	if len(spaceIDs) > 0 {
		query = query.Where(fmt.Sprintf(`
			EXISTS (
				SELECT 1
				FROM exam_targets AS targets
				JOIN space_members AS members
						ON members.tenant_id = attempts.tenant_id
						and members.user_id = attempts.user_id
						and members.status = ?
						and members.role_in_space = ?
						and members.deleted_at = 0
						and (
							(targets.target_type = ? and members.space_id = targets.target_id)
							or (targets.target_type = ? and targets.target_id = attempts.user_id and %s)
						)
					JOIN tenant_user_memberships AS tum
						ON tum.tenant_id = members.tenant_id
						and tum.user_id = members.user_id
						and tum.role = ?
						and tum.status = ?
					JOIN spaces
						ON spaces.tenant_id = members.tenant_id
						and spaces.id = members.space_id
						and spaces.status = ?
						and spaces.deleted_at = 0
					WHERE targets.tenant_id = attempts.tenant_id
						and targets.exam_id = attempts.exam_id
						and members.space_id IN ?
				)
				OR (
					NOT EXISTS (
						SELECT 1
						FROM exam_targets AS targets
						JOIN space_members AS members
								ON members.tenant_id = attempts.tenant_id
								and members.user_id = attempts.user_id
								and members.status = ?
								and members.role_in_space = ?
								and members.deleted_at = 0
								and (
									(targets.target_type = ? and members.space_id = targets.target_id)
									or (targets.target_type = ? and targets.target_id = attempts.user_id and %s)
								)
						JOIN tenant_user_memberships AS tum
								ON tum.tenant_id = members.tenant_id
								and tum.user_id = members.user_id
								and tum.role = ?
								and tum.status = ?
						JOIN spaces
								ON spaces.tenant_id = members.tenant_id
								and spaces.id = members.space_id
								and spaces.status = ?
								and spaces.deleted_at = 0
						WHERE targets.tenant_id = attempts.tenant_id
							and targets.exam_id = attempts.exam_id
					)
					and NOT EXISTS (
						SELECT 1
						FROM exam_targets AS user_targets
						WHERE user_targets.tenant_id = attempts.tenant_id
							and user_targets.exam_id = attempts.exam_id
							and user_targets.target_type = ?
					)
						and EXISTS (
							SELECT 1
							FROM tenant_user_memberships AS fallback_tum
							WHERE fallback_tum.tenant_id = attempts.tenant_id
								and fallback_tum.user_id = attempts.user_id
								and fallback_tum.role = ?
								and fallback_tum.status = ?
						)
						and (
							SELECT COUNT(DISTINCT targets.target_id)
							FROM exam_targets AS targets
							JOIN spaces
									ON spaces.tenant_id = targets.tenant_id
									and spaces.id = targets.target_id
									and spaces.status = ?
									and spaces.deleted_at = 0
							WHERE targets.tenant_id = attempts.tenant_id
								and targets.exam_id = attempts.exam_id
								and targets.target_type = ?
						) = 1
						and EXISTS (
							SELECT 1
							FROM exam_targets AS targets
							JOIN spaces
									ON spaces.tenant_id = targets.tenant_id
									and spaces.id = targets.target_id
									and spaces.status = ?
									and spaces.deleted_at = 0
							WHERE targets.tenant_id = attempts.tenant_id
								and targets.exam_id = attempts.exam_id
								and targets.target_type = ?
								and targets.target_id IN ?
						)
						and NOT EXISTS (
							SELECT 1
							FROM exam_targets AS targets
							JOIN space_members AS current_members
									ON current_members.tenant_id = targets.tenant_id
									and current_members.space_id = targets.target_id
									and current_members.user_id = attempts.user_id
									and current_members.status = ?
									and current_members.deleted_at = 0
									and current_members.role_in_space <> ?
							WHERE targets.tenant_id = attempts.tenant_id
								and targets.exam_id = attempts.exam_id
								and targets.target_type = ?
						)
					)
					`,
			r.userTargetScopedSpacePredicate("targets", "members.space_id"),
			r.userTargetScopedSpacePredicate("targets", "members.space_id")),
			servicespace.StatusEnabled, constant.RoleStudent, serviceexam.TargetTypeSpace, serviceexam.TargetTypeUser, constant.RoleStudent, servicetenantuser.StatusEnabled, servicespace.StatusEnabled, spaceIDs,
			servicespace.StatusEnabled, constant.RoleStudent, serviceexam.TargetTypeSpace, serviceexam.TargetTypeUser, constant.RoleStudent, servicetenantuser.StatusEnabled, servicespace.StatusEnabled,
			serviceexam.TargetTypeUser, constant.RoleStudent, servicetenantuser.StatusEnabled,
			servicespace.StatusEnabled, serviceexam.TargetTypeSpace,
			servicespace.StatusEnabled, serviceexam.TargetTypeSpace, spaceIDs,
			servicespace.StatusEnabled, constant.RoleStudent, serviceexam.TargetTypeSpace,
		)
	}
	return query
}

// resultScoreBucketLabel 根据试卷总分生成成绩图表的分数段标签。
// 低总分试卷如果继续使用 10 分段会全部挤在 0-9，因此按真实得分聚合；高总分试卷保持常见成绩段。
func resultScoreBucketLabel(score float64, totalScore float64) string {
	if totalScore <= 10 {
		return formatExamRepositoryScore(score)
	}
	if totalScore > 100 {
		switch {
		case score < 60:
			return "0-59"
		case score < 70:
			return "60-69"
		case score < 80:
			return "70-79"
		case score < 90:
			return "80-89"
		case score < 100:
			return "90-99"
		default:
			return "100-" + formatExamRepositoryScore(totalScore)
		}
	}
	bucketStart := int(score/10) * 10
	bucketEnd := bucketStart + 9
	if totalScore > 0 && float64(bucketEnd) > totalScore {
		bucketEnd = int(math.Ceil(totalScore))
	}
	return strconv.Itoa(bucketStart) + "-" + strconv.Itoa(bucketEnd)
}

// resultQuestionTypeLabel 返回成绩管理图表展示用的题型中文名。
// DAO 已经按快照解析题型，这里只做稳定展示映射，不参与权限或评分判断。
func resultQuestionTypeLabel(questionType string) string {
	switch questionType {
	case constant.QuestionTypeSingle:
		return "单选题"
	case constant.QuestionTypeMultiple:
		return "多选题"
	case constant.QuestionTypeJudge:
		return "判断题"
	case constant.QuestionTypeFillBlank:
		return "填空题"
	case constant.QuestionTypeShortText:
		return "解答题"
	default:
		return questionType
	}
}

func formatExamRepositoryScore(score float64) string {
	if math.Abs(score-math.Round(score)) < 0.0000001 {
		return strconv.FormatInt(int64(math.Round(score)), 10)
	}
	return strings.TrimRight(strings.TrimRight(strconv.FormatFloat(score, 'f', 2, 64), "0"), ".")
}

// GetResultSnapshot 读取单次作答成绩，并带出考试发布策略和试卷解析开关。
// 学生查分入口以 attempt 为成绩标识，避免在没有独立 result 表时引入额外状态。
func (r *ExamRepository) GetResultSnapshot(ctx context.Context, tenantID uint64, attemptID uint64) (serviceexam.ResultSnapshot, error) {
	var row struct {
		AttemptID        uint64
		ExamID           uint64
		UserID           uint64
		AttemptNo        int
		TotalScore       string
		ObjectiveScore   string
		SubjectiveScore  string
		PublishMode      string
		ScorePublishTime *int64
		ShowAnalysis     bool
	}
	err := r.db.WithContext(ctx).Table("exam_attempts AS attempts").
		Select(`
			attempts.id AS attempt_id,
			attempts.exam_id AS exam_id,
			attempts.user_id AS user_id,
			attempts.attempt_no AS attempt_no,
			attempts.total_score AS total_score,
			attempts.objective_score AS objective_score,
			attempts.subjective_score AS subjective_score,
			exams.publish_mode AS publish_mode,
			exams.score_publish_time AS score_publish_time,
			papers.show_analysis AS show_analysis
		`).
		Joins("JOIN exams ON exams.tenant_id = attempts.tenant_id AND exams.id = attempts.exam_id AND exams.deleted_at = 0").
		Joins("JOIN papers ON papers.tenant_id = exams.tenant_id AND papers.id = exams.paper_id AND papers.deleted_at = 0").
		Where("attempts.tenant_id = ?", tenantID).
		Where("attempts.id = ?", attemptID).
		Where("attempts.submitted_at IS NOT NULL").
		Scan(&row).Error
	if err != nil {
		return serviceexam.ResultSnapshot{}, err
	}
	if row.AttemptID == 0 {
		return serviceexam.ResultSnapshot{}, gorm.ErrRecordNotFound
	}
	return serviceexam.ResultSnapshot{
		AttemptID:        row.AttemptID,
		ExamID:           row.ExamID,
		UserID:           row.UserID,
		AttemptNo:        row.AttemptNo,
		TotalScore:       row.TotalScore,
		ObjectiveScore:   row.ObjectiveScore,
		SubjectiveScore:  row.SubjectiveScore,
		PublishMode:      row.PublishMode,
		ScorePublishTime: row.ScorePublishTime,
		ShowAnalysis:     row.ShowAnalysis,
	}, nil
}

// ListUserResultSnapshots 读取某个考生在同一考试下的所有已提交成绩，用于 latest/highest 策略选择。
func (r *ExamRepository) ListUserResultSnapshots(ctx context.Context, tenantID uint64, examID uint64, userID uint64) ([]serviceexam.ResultSnapshot, error) {
	var rows []struct {
		AttemptID        uint64
		ExamID           uint64
		UserID           uint64
		AttemptNo        int
		TotalScore       string
		ObjectiveScore   string
		SubjectiveScore  string
		PublishMode      string
		ScorePublishTime *int64
		ShowAnalysis     bool
	}
	if err := r.db.WithContext(ctx).Table("exam_attempts AS attempts").
		Select(`
			attempts.id AS attempt_id,
			attempts.exam_id AS exam_id,
			attempts.user_id AS user_id,
			attempts.attempt_no AS attempt_no,
			attempts.total_score AS total_score,
			attempts.objective_score AS objective_score,
			attempts.subjective_score AS subjective_score,
			exams.publish_mode AS publish_mode,
			exams.score_publish_time AS score_publish_time,
			papers.show_analysis AS show_analysis
		`).
		Joins("JOIN exams ON exams.tenant_id = attempts.tenant_id AND exams.id = attempts.exam_id AND exams.deleted_at = 0").
		Joins("JOIN papers ON papers.tenant_id = exams.tenant_id AND papers.id = exams.paper_id AND papers.deleted_at = 0").
		Where("attempts.tenant_id = ?", tenantID).
		Where("attempts.exam_id = ?", examID).
		Where("attempts.user_id = ?", userID).
		Where("attempts.submitted_at IS NOT NULL").
		Order("attempts.attempt_no ASC, attempts.id ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]serviceexam.ResultSnapshot, 0, len(rows))
	for _, row := range rows {
		items = append(items, serviceexam.ResultSnapshot{
			AttemptID:        row.AttemptID,
			ExamID:           row.ExamID,
			UserID:           row.UserID,
			AttemptNo:        row.AttemptNo,
			TotalScore:       row.TotalScore,
			ObjectiveScore:   row.ObjectiveScore,
			SubjectiveScore:  row.SubjectiveScore,
			PublishMode:      row.PublishMode,
			ScorePublishTime: row.ScorePublishTime,
			ShowAnalysis:     row.ShowAnalysis,
		})
	}
	return items, nil
}

// UpdateScorePublishConfig 更新考试成绩发布方式。
// 发布配置和操作日志必须同事务写入，避免成绩已可见但管理端日志缺失。
func (r *ExamRepository) UpdateScorePublishConfig(ctx context.Context, input serviceexam.UpdateScorePublishConfigRepositoryInput) (serviceexam.Exam, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&ExamDO{}).
			Where(ExamColumns.TenantID+" = ?", input.TenantID).
			Where(ExamColumns.ID+" = ?", input.ExamID).
			Where(ExamColumns.DeletedAt+" = ?", 0).
			Updates(map[string]any{
				ExamColumns.PublishMode:      input.PublishMode,
				ExamColumns.ScorePublishTime: input.ScorePublishTime,
				BaseColumns.UpdatedAt:        r.now(),
				BaseColumns.UpdatedByType:    AuditActorTenantUser,
				BaseColumns.Version:          gorm.Expr(BaseColumns.Version + " + 1"),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		if input.Log == nil {
			return nil
		}
		publishLog := *input.Log
		publishLog.TenantID = input.TenantID
		publishLog.ExamID = input.ExamID
		targets, err := r.listTargetsInDB(tx, input.TenantID, input.ExamID)
		if err != nil {
			return err
		}
		spaceIDs, err := r.operationTargetSpaceIDsInTx(tx, input.TenantID, targets)
		if err != nil {
			return err
		}
		return r.appendOperationLogsForSpacesInTx(tx, publishLog, spaceIDs)
	})
	if err != nil {
		return serviceexam.Exam{}, err
	}
	return r.GetExam(ctx, input.TenantID, input.ExamID)
}

// UpdateExamStatus 更新考试状态，并在同一事务中写入管理端操作日志。
func (r *ExamRepository) UpdateExamStatus(ctx context.Context, input serviceexam.UpdateExamStatusRepositoryInput) (serviceexam.Exam, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{
			ExamColumns.Status:        input.Status,
			BaseColumns.UpdatedAt:     r.now(),
			BaseColumns.UpdatedByType: AuditActorTenantUser,
			BaseColumns.Version:       gorm.Expr(BaseColumns.Version + " + 1"),
		}
		if input.InviteCode != "" {
			updates[ExamColumns.InviteCode] = input.InviteCode
		}
		if input.EndTime != nil {
			updates[ExamColumns.EndTime] = *input.EndTime
		}
		query := tx.Model(&ExamDO{}).
			Where(ExamColumns.TenantID+" = ?", input.TenantID).
			Where(ExamColumns.ID+" = ?", input.ExamID).
			Where(ExamColumns.DeletedAt+" = ?", 0)
		if input.ExpectedStatus != "" {
			query = query.Where(ExamColumns.Status+" = ?", input.ExpectedStatus)
		}
		result := query.Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return r.examStatusPreconditionError(tx, input.TenantID, input.ExamID, input.ExpectedStatus)
		}
		if input.Log.OperationType == "" {
			return nil
		}
		log := input.Log
		log.TenantID = input.TenantID
		log.ExamID = input.ExamID
		targets, err := r.listTargetsInDB(tx, input.TenantID, input.ExamID)
		if err != nil {
			return err
		}
		spaceIDs, err := r.operationTargetSpaceIDsInTx(tx, input.TenantID, targets)
		if err != nil {
			return err
		}
		return r.appendOperationLogsForSpacesInTx(tx, log, spaceIDs)
	})
	if err != nil {
		return serviceexam.Exam{}, err
	}
	return r.GetExam(ctx, input.TenantID, input.ExamID)
}

// DeleteDraftExam 软删除草稿考试。
// 已发布、已结束或已禁用考试必须保留考试记录和作答链路，不能通过删除接口移除。
func (r *ExamRepository) DeleteDraftExam(ctx context.Context, input serviceexam.DeleteDraftInput) error {
	now := r.now()
	result := r.db.WithContext(ctx).Model(&ExamDO{}).
		Where(ExamColumns.TenantID+" = ?", input.TenantID).
		Where(ExamColumns.ID+" = ?", input.ExamID).
		Where(ExamColumns.Status+" = ?", serviceexam.StatusDraft).
		Where(ExamColumns.DeletedAt+" = ?", 0).
		Updates(map[string]any{
			ExamColumns.DeletedAt:     soft_delete.DeletedAt(now),
			BaseColumns.UpdatedAt:     now,
			BaseColumns.UpdatedByType: AuditActorTenantUser,
			BaseColumns.Version:       gorm.Expr(BaseColumns.Version + " + 1"),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return r.examStatusPreconditionError(r.db.WithContext(ctx), input.TenantID, input.ExamID, serviceexam.StatusDraft)
	}
	return nil
}

func (r *ExamRepository) examStatusPreconditionError(tx *gorm.DB, tenantID uint64, examID uint64, expectedStatus string) error {
	if expectedStatus == "" {
		return gorm.ErrRecordNotFound
	}
	var count int64
	if err := tx.Model(&ExamDO{}).
		Where(ExamColumns.TenantID+" = ?", tenantID).
		Where(ExamColumns.ID+" = ?", examID).
		Where(ExamColumns.DeletedAt+" = ?", 0).
		Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return gorm.ErrRecordNotFound
	}
	return serviceexam.ErrInvalidExamStatus
}

// AppendExportOperationLog 写入成绩导出审计日志。
// 导出文件属于外部副作用；调用方会在文件生成成功后调用本方法，并在日志失败时向外传播错误。
func (r *ExamRepository) AppendExportOperationLog(ctx context.Context, log serviceexam.OperationLog, spaceIDs []uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return r.appendOperationLogsForSpacesInTx(tx, log, spaceIDs)
	})
}

// UpdateManagementSettings 更新考试详情页设置，并在同一事务中写入管理端操作日志。
// 当前设置接口只负责成绩发布配置；其它影响公平性的字段不在该方法入参中，避免被前端绕过修改。
func (r *ExamRepository) UpdateManagementSettings(ctx context.Context, input serviceexam.UpdateManagementSettingsRepositoryInput) (serviceexam.Exam, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&ExamDO{}).
			Where(ExamColumns.TenantID+" = ?", input.TenantID).
			Where(ExamColumns.ID+" = ?", input.ExamID).
			Where(ExamColumns.DeletedAt+" = ?", 0).
			Updates(map[string]any{
				ExamColumns.PublishMode:      input.PublishMode,
				ExamColumns.ScorePublishTime: input.ScorePublishTime,
				BaseColumns.UpdatedAt:        r.now(),
				BaseColumns.UpdatedByType:    AuditActorTenantUser,
				BaseColumns.Version:          gorm.Expr(BaseColumns.Version + " + 1"),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		targets, err := r.listTargetsInDB(tx, input.TenantID, input.ExamID)
		if err != nil {
			return err
		}
		spaceIDs, err := r.operationTargetSpaceIDsInTx(tx, input.TenantID, targets)
		if err != nil {
			return err
		}
		return r.appendOperationLogsForSpacesInTx(tx, input.Log, spaceIDs)
	})
	if err != nil {
		return serviceexam.Exam{}, err
	}
	return r.GetExam(ctx, input.TenantID, input.ExamID)
}

// GetAttemptQuestion 读取一次作答中的单题快照。
// 保存答案时以这里的题型快照为准，防止客户端伪造 question_type 影响答案序列化和后续判分。
func (r *ExamRepository) GetAttemptQuestion(ctx context.Context, tenantID uint64, attemptID uint64, attemptQuestionID uint64) (serviceexam.AttemptQuestion, error) {
	var row ExamAttemptQuestionDO
	err := r.db.WithContext(ctx).
		Where(ExamAttemptQuestionColumns.TenantID+" = ?", tenantID).
		Where(ExamAttemptQuestionColumns.AttemptID+" = ?", attemptID).
		Where(ExamAttemptQuestionColumns.ID+" = ?", attemptQuestionID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return serviceexam.AttemptQuestion{}, serviceexam.ErrAttemptNotFound
	}
	if err != nil {
		return serviceexam.AttemptQuestion{}, err
	}
	return attemptQuestionFromDO(row), nil
}

// listAttemptQuestions 按作答题目顺序读取已保存的快照。
// 开考重试、答题页加载和幂等保存都会复用这份快照数据。
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

// UpsertAnswer 保存或更新考生答案。
// 自动保存场景会多次写同一道题，按 tenant、attempt 和 attempt_question 做唯一冲突更新。
func (r *ExamRepository) UpsertAnswer(ctx context.Context, answer serviceexam.Answer) error {
	now := r.now()
	row := ExamAnswerDO{
		BaseFields: BaseFields{
			CreatedAt:     now,
			CreatedBy:     answer.UpdatedBy,
			CreatedByType: AuditActorTenantUser,
			UpdatedAt:     now,
			UpdatedBy:     answer.UpdatedBy,
			UpdatedByType: AuditActorTenantUser,
			Version:       1,
			ExtJSON:       datatypes.JSON([]byte("{}")),
		},
		TenantID:          answer.TenantID,
		AttemptID:         answer.AttemptID,
		AttemptQuestionID: answer.AttemptQuestionID,
		AnswerContent:     answer.AnswerContent,
		GradingStatus:     serviceexam.GradingStatusPending,
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var attempt ExamAttemptDO
		// 自动保存和提交可能并发到达。这里锁住 attempt 行并重新校验状态，
		// 防止提交完成后的滞后保存继续覆盖答案内容。
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where(ExamAttemptColumns.TenantID+" = ?", answer.TenantID).
			Where(ExamAttemptColumns.ID+" = ?", answer.AttemptID).
			First(&attempt).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return serviceexam.ErrAttemptNotFound
			}
			return err
		}
		if attempt.Status != serviceexam.AttemptStatusInProgress {
			return serviceexam.ErrAttemptAlreadySubmitted
		}

		// 更新答案内容时同步递增 version，人工阅卷会用该版本做乐观锁，避免评分覆盖新答案。
		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: ExamAnswerColumns.TenantID},
				{Name: ExamAnswerColumns.AttemptID},
				{Name: ExamAnswerColumns.AttemptQuestionID},
			},
			DoUpdates: clause.Assignments(map[string]any{
				ExamAnswerColumns.AnswerContent: answer.AnswerContent,
				BaseColumns.UpdatedAt:           now,
				BaseColumns.UpdatedBy:           answer.UpdatedBy,
				BaseColumns.UpdatedByType:       AuditActorTenantUser,
				BaseColumns.Version:             gorm.Expr(BaseColumns.Version + " + 1"),
			}),
		}).Create(&row).Error
	})
}

// SubmitAttemptAndGradeObjectiveQuestions 提交作答并完成客观题自动判分。
// 提交状态更新、答案判分、分数回写和提交事件写入在同一事务内完成。
func (r *ExamRepository) SubmitAttemptAndGradeObjectiveQuestions(ctx context.Context, input serviceexam.SubmitAttemptAndGradeInput) (int64, error) {
	var affected int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 作答提交使用 attempt version 和 in_progress 状态做乐观锁。
		// 重复提交或页面持有旧版本时不会继续判分，也不会重复写提交事件。
		result := tx.Model(&ExamAttemptDO{}).
			Where(ExamAttemptColumns.TenantID+" = ?", input.TenantID).
			Where(ExamAttemptColumns.ID+" = ?", input.AttemptID).
			Where(BaseColumns.Version+" = ?", input.Version).
			Where(ExamAttemptColumns.Status+" = ?", serviceexam.AttemptStatusInProgress).
			Updates(map[string]any{
				ExamAttemptColumns.Status:      input.Status,
				ExamAttemptColumns.SubmittedAt: input.SubmittedAt,
				BaseColumns.UpdatedAt:          input.SubmittedAt,
				BaseColumns.UpdatedByType:      AuditActorTenantUser,
				BaseColumns.Version:            gorm.Expr(BaseColumns.Version + " + 1"),
			})
		if result.Error != nil {
			return result.Error
		}
		affected = result.RowsAffected
		if affected == 0 {
			return nil
		}

		// 自动判分读取的是 attempt_questions 中的题目快照和正确答案快照，保证按开考时内容评分。
		items, err := r.listAnswersForGrading(tx, input.TenantID, input.AttemptID)
		if err != nil {
			return err
		}
		grades, objectiveScore, err := input.Grader(items)
		if err != nil {
			return err
		}
		if err := r.saveAnswerGrades(tx, input.TenantID, input.AttemptID, input.SubmittedAt, grades); err != nil {
			return err
		}
		// 提交时总分先等于客观题分；后续简答题人工评分完成后会再叠加主观题分。
		if err := tx.Model(&ExamAttemptDO{}).
			Where(ExamAttemptColumns.TenantID+" = ?", input.TenantID).
			Where(ExamAttemptColumns.ID+" = ?", input.AttemptID).
			Updates(map[string]any{
				ExamAttemptColumns.ObjectiveScore: objectiveScore,
				ExamAttemptColumns.TotalScore:     objectiveScore,
				BaseColumns.UpdatedAt:             input.SubmittedAt,
				BaseColumns.UpdatedByType:         AuditActorTenantUser,
			}).Error; err != nil {
			return err
		}
		return appendEventWithDB(tx, input.Event)
	})
	return affected, err
}

// AppendEvent 追加考试事件。
// 事件用于记录提交等关键动作，便于后续审计或排查考生作答链路。
func (r *ExamRepository) AppendEvent(ctx context.Context, event serviceexam.ExamEvent) error {
	return appendEventWithDB(r.db.WithContext(ctx), event)
}

// snapshotOptions 汇总一道题在作答快照中需要保存的选项信息。
// optionIDs 用于展示顺序，correctOptionIDs 用于客观题判分。
type snapshotOptions struct {
	options          []serviceexam.SnapshotSourceOption
	optionIDs        []uint64
	correctOptionIDs []uint64
}

// listSnapshotOptions 按选项顺序读取题目的展示选项和正确选项。
// 返回值会被写入作答快照，确保后续题库选项调整不影响已开考的作答。
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

// listAnswersForGrading 组装自动判分输入。
// 未作答的题目在答案 map 中没有记录，会以空字符串交给 grader，保持“未答即空答”的判分语义。
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
		// 题型从题目快照解析，判分逻辑不依赖当前题库记录，避免考试后编辑题库改变历史评分。
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

// saveAnswerGrades 写入自动判分结果。
// 客观题如果已有自动保存答案则更新分数和状态；未作答题也会插入一行评分记录。
func (r *ExamRepository) saveAnswerGrades(tx *gorm.DB, tenantID uint64, attemptID uint64, now int64, grades []serviceexam.AnswerGradingResult) error {
	for _, grade := range grades {
		row := ExamAnswerDO{
			BaseFields: BaseFields{
				CreatedAt:     now,
				CreatedByType: AuditActorTenantUser,
				UpdatedAt:     now,
				UpdatedByType: AuditActorTenantUser,
				Version:       1,
				ExtJSON:       datatypes.JSON([]byte("{}")),
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
				BaseColumns.UpdatedByType:       AuditActorTenantUser,
				BaseColumns.Version:             gorm.Expr(BaseColumns.Version + " + 1"),
			}),
		}).Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

// appendEventWithDB 使用传入的 DB 或事务写入考试事件。
// 空 payload 统一落为 {}，保证事件表中的 JSON 字段始终是合法对象。
func appendEventWithDB(tx *gorm.DB, event serviceexam.ExamEvent) error {
	payload := event.Payload
	if payload == "" {
		payload = "{}"
	}
	return tx.Create(&ExamEventDO{
		EventFields: EventFields{
			CreatedAt:     event.EventTime,
			CreatedByType: AuditActorTenantUser,
			ExtJSON:       datatypes.JSON([]byte("{}")),
		},
		TenantID:  event.TenantID,
		AttemptID: event.AttemptID,
		EventType: event.EventType,
		EventTime: event.EventTime,
		Payload:   datatypes.JSON([]byte(payload)),
	}).Error
}

// parseQuestionSnapshotType 从题目快照中解析题型。
// 自动判分只需要题型字段，不回查 questions 表，确保按作答时的题目定义评分。
func parseQuestionSnapshotType(raw datatypes.JSON) (string, error) {
	var snapshot struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return "", err
	}
	return snapshot.Type, nil
}

// parseQuestionSnapshotTitle 从题目快照中解析题目标题。
// 阅卷列表展示历史题目标题，避免题库标题修改后改变教师看到的待阅记录。
func parseQuestionSnapshotTitle(raw string) (string, error) {
	var snapshot struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal([]byte(raw), &snapshot); err != nil {
		return "", err
	}
	return snapshot.Title, nil
}

func fillBlankCount(questionType string, correctText string) int {
	if questionType != constant.QuestionTypeFillBlank {
		return 0
	}
	trimmed := strings.TrimSpace(correctText)
	if trimmed == "" {
		return 1
	}
	if !strings.HasPrefix(trimmed, "[") {
		return 1
	}
	var answers []string
	if err := json.Unmarshal([]byte(trimmed), &answers); err != nil || len(answers) == 0 {
		return 1
	}
	return len(answers)
}

// sumGradedSubjectiveScore 汇总一次作答中已完成人工评分的主观题分数。
// 仍处于待阅状态的答案不会计入 subjective_score，避免总分提前包含未确认分数。
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

// parseScoreString 将数据库中的分数字符串转换为浮点数。
// 空字符串表示该分数尚未产生，在重算总分时按 0 处理。
func parseScoreString(value string) (float64, error) {
	if strings.TrimSpace(value) == "" {
		return 0, nil
	}
	return strconv.ParseFloat(value, 64)
}

// formatScoreString 将计算后的分数写回数据库使用的字符串格式。
// 保留最多 6 位小数并裁剪无意义的尾零，避免成绩导出出现 10.000000 这类展示值。
func formatScoreString(value float64) string {
	text := strconv.FormatFloat(value, 'f', 6, 64)
	text = strings.TrimRight(text, "0")
	text = strings.TrimRight(text, ".")
	if text == "" {
		return "0"
	}
	return text
}

func isUniqueConstraintError(err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) {
		return sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique ||
			sqliteErr.ExtendedCode == sqlite3.ErrConstraintPrimaryKey
	}
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		return mysqlErr.Number == 1062
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

// examFromDO 将考试表记录转换为服务层考试对象。
// buildMode 来自试卷表，调用方按需传入，避免列表查询为每条考试额外关联试卷。
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
		CreatedBy:        row.CreatedBy,
		CreatedByType:    row.CreatedByType,
	}
}

// operationLogFromDO 将操作日志表记录转换为服务层对象。
// JSON 扩展字段以字符串传出，避免 service 和 API 层依赖 GORM 的 datatypes.JSON 类型。
func operationLogFromDO(row ExamOperationLogDO) serviceexam.OperationLog {
	return serviceexam.OperationLog{
		ID:              row.ID,
		TenantID:        row.TenantID,
		ExamID:          row.ExamID,
		OperationType:   row.OperationType,
		OperationTitle:  row.OperationTitle,
		OperationDetail: row.OperationDetail,
		ActorID:         row.ActorID,
		ActorType:       row.ActorType,
		ActorRole:       row.ActorRole,
		SpaceID:         row.SpaceID,
		CreatedAt:       row.CreatedAt,
		CreatedBy:       row.CreatedBy,
		CreatedByType:   row.CreatedByType,
		ExtJSON:         string(row.ExtJSON),
	}
}

// attemptFromDO 将作答表记录转换为服务层作答对象。
// Version 会暴露给提交接口，用于防止重复提交或旧页面覆盖新状态。
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

// attemptQuestionFromDO 将作答题目快照转换为服务层对象。
// JSON 快照在服务层以字符串传递，避免仓储层泄露数据库 JSON 类型。
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

// pendingReviewRow 是待阅列表 SQL 的扫描结构。
// SubmittedAt 使用指针承接数据库 NULL，再由 SubmittedAtValue 转成服务层默认值。
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

// SubmittedAtValue 将可空提交时间转换为服务层使用的 int64。
// 理论上待阅答案来自已提交作答，但这里保留 NULL 兼容，避免扫描层直接解引用空指针。
func (r pendingReviewRow) SubmittedAtValue() int64 {
	if r.SubmittedAt == nil {
		return 0
	}
	return *r.SubmittedAt
}

// scoreExportRow 是成绩导出 SQL 的扫描结构。
// 该结构只服务导出查询，避免把聚合字段暴露到通用 DO。
type scoreExportRow struct {
	AttemptID       uint64
	StudentName     string
	SpaceID         uint64
	SpaceName       string
	AttemptNo       int
	ObjectiveScore  string
	SubjectiveScore string
	TotalScore      string
	SubmittedAt     *int64
}

type examResultSummaryRow struct {
	Submitted    int64
	AverageScore float64
	HighestScore float64
	Passed       int64
}

type managementResultRow struct {
	AttemptID              uint64
	UserID                 uint64
	Username               string
	RealName               string
	ObjectiveScore         string
	SubjectiveScore        string
	TotalScore             string
	SubmittedAt            *int64
	PendingSubjectiveCount int64
}

type managementResultScoreRow struct {
	TotalScore float64
}

type managementResultQuestionScoreRow struct {
	QuestionSnapshot string
	MaxScore         string
	EarnedScore      string
}

type managementAnswerSheetAttemptRow struct {
	AttemptID       uint64
	ExamID          uint64
	UserID          uint64
	Username        string
	RealName        string
	ObjectiveScore  string
	SubjectiveScore string
	TotalScore      string
	SubmittedAt     *int64
}

type managementAnswerSheetItemRow struct {
	AttemptQuestionID     uint64
	SectionID             uint64
	QuestionID            uint64
	SortOrder             int
	SectionSnapshot       string
	QuestionSnapshot      string
	OptionSnapshot        string
	CorrectAnswerSnapshot string
	Score                 string
	AnswerContent         string
	AnswerScore           string
	GradingStatus         string
	GraderComment         string
	GradedBy              uint64
	GradedAt              *int64
}

// SubmittedAtValue 将答卷详情作答摘要中的可空提交时间转换为服务层默认值。
// 答卷详情入口只允许已提交作答，这里保留空值安全以兼容历史异常数据。
func (r managementAnswerSheetAttemptRow) SubmittedAtValue() int64 {
	if r.SubmittedAt == nil {
		return 0
	}
	return *r.SubmittedAt
}

// SubmittedAtValue 将成绩管理列表中的可空提交时间转换为服务层默认值。
// 列表查询已过滤 submitted_at 非空，这里保留空值安全以兼容历史异常数据。
func (r managementResultRow) SubmittedAtValue() int64 {
	if r.SubmittedAt == nil {
		return 0
	}
	return *r.SubmittedAt
}

// SubmittedAtValue 将成绩导出中的可空提交时间转换为服务层默认值。
// 导出查询已经过滤 submitted_at 非空，这里主要用于保持扫描结构的空值安全。
func (r scoreExportRow) SubmittedAtValue() int64 {
	if r.SubmittedAt == nil {
		return 0
	}
	return *r.SubmittedAt
}

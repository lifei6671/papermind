package db

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/mattn/go-sqlite3"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

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
		query = query.Where(`
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
						))
					)
			)
		`, serviceexam.TargetTypeSpace, *input.SpaceID, serviceexam.TargetTypeUser, servicetenantuser.StatusEnabled, servicetenantuser.StatusEnabled, *input.SpaceID, servicespace.StatusEnabled)
	}
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

// CreateExam 创建考试草稿的基础记录。
// 未指定作答次数、成绩策略和成绩发布模式时写入业务默认值，保证后续发布流程有稳定初始状态。
func (r *ExamRepository) CreateExam(ctx context.Context, exam serviceexam.Exam) (serviceexam.Exam, error) {
	now := r.now()
	row := ExamDO{
		BaseFields: BaseFields{
			CreatedAt:     now,
			CreatedByType: AuditActorTenantUser,
			UpdatedAt:     now,
			UpdatedByType: AuditActorTenantUser,
			Version:       1,
			ExtJSON:       datatypes.JSON("{}"),
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
func (r *ExamRepository) PublishExamAndFreezeLivePool(ctx context.Context, exam serviceexam.Exam, pool []serviceexam.LivePoolItem) (serviceexam.Exam, error) {
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
		return nil
	})
	if err != nil {
		return serviceexam.Exam{}, err
	}
	return r.GetExam(ctx, exam.TenantID, exam.ID)
}

// CreatePublishedExamWithTarget 创建已发布考试，并在同一个事务内写入动态题池和首个投放目标。
// API 的“发布考试”入口使用该方法，避免校验或目标写入失败后留下草稿或无目标考试。
func (r *ExamRepository) CreatePublishedExamWithTarget(ctx context.Context, exam serviceexam.Exam, pool []serviceexam.LivePoolItem, target serviceexam.Target) (serviceexam.Exam, error) {
	var createdID uint64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := r.now()
		row := ExamDO{
			BaseFields: BaseFields{
				CreatedAt:     now,
				CreatedByType: AuditActorTenantUser,
				UpdatedAt:     now,
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
		return tx.Create(&targetRow).Error
	})
	if err != nil {
		return serviceexam.Exam{}, err
	}
	return r.GetExam(ctx, exam.TenantID, createdID)
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
	row := ExamTargetDO{
		RelationFields: RelationFields{
			CreatedAt:     r.now(),
			CreatedByType: AuditActorTenantUser,
			ExtJSON:       datatypes.JSON([]byte("{}")),
		},
		TenantID:   target.TenantID,
		ExamID:     target.ExamID,
		TargetType: target.TargetType,
		TargetID:   target.TargetID,
	}
	return r.db.WithContext(ctx).Create(&row).Error
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
	return examFromDO(row, paper.BuildMode), nil
}

// ListTargets 读取考试已配置的投放目标，用于从真实资源范围重建管理权限。
func (r *ExamRepository) ListTargets(ctx context.Context, tenantID uint64, examID uint64) ([]serviceexam.Target, error) {
	var rows []ExamTargetDO
	if err := r.db.WithContext(ctx).
		Where(ExamTargetColumns.TenantID+" = ?", tenantID).
		Where(ExamTargetColumns.ExamID+" = ?", examID).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	targets := make([]serviceexam.Target, 0, len(rows))
	for _, row := range rows {
		targets = append(targets, serviceexam.Target{
			TenantID:   row.TenantID,
			ExamID:     row.ExamID,
			TargetType: row.TargetType,
			TargetID:   row.TargetID,
		})
	}
	return targets, nil
}

// ExamTargetSpaceIDs 读取考试目标覆盖到的有效空间，用于无成绩时仍按真实考试范围鉴权。
func (r *ExamRepository) ExamTargetSpaceIDs(ctx context.Context, tenantID uint64, examID uint64) ([]uint64, error) {
	var rows []attemptTargetSpaceRow
	if err := r.db.WithContext(ctx).Raw(`
		SELECT DISTINCT spaces.id AS space_id, spaces.name AS space_name
		FROM exam_targets AS targets
		JOIN spaces ON spaces.tenant_id = targets.tenant_id
			AND spaces.id = targets.target_id
			AND spaces.status = ?
			AND spaces.deleted_at = 0
		WHERE targets.tenant_id = ?
			AND targets.exam_id = ?
			AND targets.target_type = ?
		UNION
		SELECT DISTINCT spaces.id AS space_id, spaces.name AS space_name
		FROM exam_targets AS targets
		JOIN space_members AS members ON members.tenant_id = targets.tenant_id
			AND members.user_id = targets.target_id
			AND members.status = ?
			AND members.deleted_at = 0
		JOIN spaces ON spaces.tenant_id = members.tenant_id
			AND spaces.id = members.space_id
			AND spaces.status = ?
			AND spaces.deleted_at = 0
		JOIN users ON users.id = members.user_id
			AND users.status = ?
			AND users.deleted_at = 0
		JOIN tenant_user_memberships AS tum ON tum.tenant_id = members.tenant_id
			AND tum.user_id = members.user_id
			AND tum.status = ?
		WHERE targets.tenant_id = ?
			AND targets.exam_id = ?
			AND targets.target_type = ?
		ORDER BY space_id ASC
	`,
		servicespace.StatusEnabled, tenantID, examID, serviceexam.TargetTypeSpace,
		servicespace.StatusEnabled, servicespace.StatusEnabled, servicetenantuser.StatusEnabled, servicetenantuser.StatusEnabled,
		tenantID, examID, serviceexam.TargetTypeUser,
	).Scan(&rows).Error; err != nil {
		return nil, err
	}
	return targetSpaceIDs(rows), nil
}

// IsEligible 判断用户是否具备参加考试的资格。
// 资格来源包括考试直投给用户，以及考试投放到用户所在的有效学生空间。
func (r *ExamRepository) IsEligible(ctx context.Context, tenantID uint64, examID uint64, userID uint64) (bool, error) {
	var directCount int64
	// 直投用户必须仍是启用状态，并且拥有 student 角色，避免被禁用或非学生用户进入考试。
	if err := r.db.WithContext(ctx).Model(&ExamTargetDO{}).
		Joins("JOIN users ON users."+UserColumns.ID+" = exam_targets."+ExamTargetColumns.TargetID+
			" AND users."+UserColumns.ID+" = ?"+
			" AND users."+UserColumns.Status+" = ?"+
			" AND users."+UserColumns.DeletedAt+" = ?", userID, "enabled", 0).
		Joins("JOIN tenant_user_memberships AS tum ON tum."+UserRoleColumns.TenantID+" = exam_targets."+ExamTargetColumns.TenantID+
			" AND tum."+UserRoleColumns.UserID+" = users."+UserColumns.ID+
			" AND tum."+UserRoleColumns.Role+" = ?"+
			" AND tum."+UserRoleColumns.Status+" = ?", "student", "enabled").
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
func (r *ExamRepository) ListPendingAttempts(ctx context.Context, tenantID uint64, examID uint64) ([]serviceexam.PendingAttempt, error) {
	var rows []pendingReviewRow
	// 待阅答案主查询不直接关联空间成员，避免多空间考生把同一答案放大为多行。
	// 本次考试实际命中的投放空间会在结果组装阶段按 attempt 单独读取。
	if err := r.db.WithContext(ctx).Table("exam_answers AS answers").
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
		Where("answers.grading_status = ?", constant.GradingStatusPending).
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
// 空间投放只命中目标空间；用户直投则映射到考生当前有效空间成员关系。
func (r *ExamRepository) listAttemptTargetSpaces(ctx context.Context, tenantID uint64, attemptID uint64) ([]attemptTargetSpaceRow, error) {
	var rows []attemptTargetSpaceRow
	if err := r.db.WithContext(ctx).Table("exam_attempts AS attempts").
		Select("DISTINCT spaces.id AS space_id, spaces.name AS space_name").
		Joins("JOIN exam_targets AS targets ON targets.tenant_id = attempts.tenant_id AND targets.exam_id = attempts.exam_id").
		Joins(`
			JOIN space_members AS members
				ON members.tenant_id = attempts.tenant_id
				AND members.user_id = attempts.user_id
				AND members.status = ?
				AND members.deleted_at = 0
				AND (
					(targets.target_type = ? AND members.space_id = targets.target_id)
					OR (targets.target_type = ? AND targets.target_id = attempts.user_id)
				)
		`, servicespace.StatusEnabled, serviceexam.TargetTypeSpace, serviceexam.TargetTypeUser).
		Joins("JOIN spaces ON spaces.tenant_id = members.tenant_id AND spaces.id = members.space_id AND spaces.status = ? AND spaces.deleted_at = 0", servicespace.StatusEnabled).
		Joins("JOIN users ON users.id = members.user_id AND users.status = ? AND users.deleted_at = 0", servicetenantuser.StatusEnabled).
		Joins("JOIN tenant_user_memberships AS tum ON tum.tenant_id = members.tenant_id AND tum.user_id = members.user_id AND tum.status = ?", servicetenantuser.StatusEnabled).
		Where("attempts.tenant_id = ?", tenantID).
		Where("attempts.id = ?", attemptID).
		Order("spaces.id ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func firstAttemptTargetSpace(spaces []attemptTargetSpaceRow) (uint64, string) {
	if len(spaces) == 0 {
		return 0, ""
	}
	return spaces[0].SpaceID, spaces[0].SpaceName
}

func targetSpaceIDs(spaces []attemptTargetSpaceRow) []uint64 {
	ids := make([]uint64, 0, len(spaces))
	for _, space := range spaces {
		ids = append(ids, space.SpaceID)
	}
	return ids
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
		// 总分由提交时的客观题分加上当前已完成评分的主观题分组成。
		return tx.Model(&ExamAttemptDO{}).
			Where(ExamAttemptColumns.TenantID+" = ?", grade.TenantID).
			Where(ExamAttemptColumns.ID+" = ?", grade.AttemptID).
			Updates(map[string]any{
				ExamAttemptColumns.SubjectiveScore: formatScoreString(subjectiveScore),
				ExamAttemptColumns.TotalScore:      formatScoreString(objectiveScore + subjectiveScore),
				BaseColumns.UpdatedAt:              now,
				BaseColumns.UpdatedByType:          AuditActorTenantUser,
				BaseColumns.Version:                gorm.Expr(BaseColumns.Version + " + 1"),
			}).Error
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
		spaceID, spaceName := firstAttemptTargetSpace(spaces)
		items = append(items, serviceexam.ScoreExportRow{
			StudentName:     row.StudentName,
			SpaceID:         spaceID,
			SpaceName:       spaceName,
			SpaceIDs:        targetSpaceIDs(spaces),
			AttemptNo:       row.AttemptNo,
			ObjectiveScore:  row.ObjectiveScore,
			SubjectiveScore: row.SubjectiveScore,
			TotalScore:      row.TotalScore,
			SubmittedAt:     row.SubmittedAtValue(),
		})
	}
	return items, nil
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
// 修改后重新读取考试，确保调用方拿到包含最新发布配置和试卷组卷模式的业务对象。
func (r *ExamRepository) UpdateScorePublishConfig(ctx context.Context, tenantID uint64, examID uint64, publishMode string, scorePublishTime *int64) (serviceexam.Exam, error) {
	if err := r.db.WithContext(ctx).Model(&ExamDO{}).
		Where(ExamColumns.TenantID+" = ?", tenantID).
		Where(ExamColumns.ID+" = ?", examID).
		Where(ExamColumns.DeletedAt+" = ?", 0).
		Updates(map[string]any{
			ExamColumns.PublishMode:      publishMode,
			ExamColumns.ScorePublishTime: scorePublishTime,
			BaseColumns.UpdatedAt:        r.now(),
			BaseColumns.UpdatedByType:    AuditActorTenantUser,
			BaseColumns.Version:          gorm.Expr(BaseColumns.Version + " + 1"),
		}).Error; err != nil {
		return serviceexam.Exam{}, err
	}
	return r.GetExam(ctx, tenantID, examID)
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
func (r *ExamRepository) SubmitAttemptAndGradeObjectiveQuestions(ctx context.Context, tenantID uint64, attemptID uint64, version int64, submittedAt int64, status string, grader serviceexam.ObjectiveGradingFunc, event serviceexam.ExamEvent) (int64, error) {
	var affected int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 作答提交使用 attempt version 和 in_progress 状态做乐观锁。
		// 重复提交或页面持有旧版本时不会继续判分，也不会重复写提交事件。
		result := tx.Model(&ExamAttemptDO{}).
			Where(ExamAttemptColumns.TenantID+" = ?", tenantID).
			Where(ExamAttemptColumns.ID+" = ?", attemptID).
			Where(BaseColumns.Version+" = ?", version).
			Where(ExamAttemptColumns.Status+" = ?", serviceexam.AttemptStatusInProgress).
			Updates(map[string]any{
				ExamAttemptColumns.Status:      status,
				ExamAttemptColumns.SubmittedAt: submittedAt,
				BaseColumns.UpdatedAt:          submittedAt,
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
		// 提交时总分先等于客观题分；后续简答题人工评分完成后会再叠加主观题分。
		if err := tx.Model(&ExamAttemptDO{}).
			Where(ExamAttemptColumns.TenantID+" = ?", tenantID).
			Where(ExamAttemptColumns.ID+" = ?", attemptID).
			Updates(map[string]any{
				ExamAttemptColumns.ObjectiveScore: objectiveScore,
				ExamAttemptColumns.TotalScore:     objectiveScore,
				BaseColumns.UpdatedAt:             submittedAt,
				BaseColumns.UpdatedByType:         AuditActorTenantUser,
			}).Error; err != nil {
			return err
		}
		return appendEventWithDB(tx, event)
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

// SubmittedAtValue 将成绩导出中的可空提交时间转换为服务层默认值。
// 导出查询已经过滤 submitted_at 非空，这里主要用于保持扫描结构的空值安全。
func (r scoreExportRow) SubmittedAtValue() int64 {
	if r.SubmittedAt == nil {
		return 0
	}
	return *r.SubmittedAt
}

package devseed

import (
	"fmt"
	"strings"
	"time"

	dbmodel "github.com/lifei6671/papermind/server/internal/dao/db"
	"github.com/lifei6671/papermind/server/library/constant"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	demoPlatformAdminID  uint64 = 1
	demoTenantID         uint64 = 10
	demoTenantAdminID    uint64 = 1
	demoStudentID        uint64 = 20
	demoTeacherID        uint64 = 21
	demoSpaceID          uint64 = 100
	demoClassSpaceID     uint64 = 200
	demoPaperID          uint64 = 100
	demoSingleSectionID  uint64 = 1001
	demoShortSectionID   uint64 = 1002
	demoSingleQuestionID uint64 = 2001
	demoShortQuestionID  uint64 = 2002
	demoExamID           uint64 = 1

	sevenDaysMillis int64 = int64(7 * 24 * time.Hour / time.Millisecond)
)

// RunForDriver 只为本地 SQLite 开发环境补齐前端端到端联调所需的最小演示数据。
func RunForDriver(gormDB *gorm.DB, driver string, appEnv string) error {
	return runForDriver(gormDB, driver, appEnv, func() int64 { return time.Now().UnixMilli() })
}

func runForDriver(gormDB *gorm.DB, driver string, appEnv string, now func() int64) error {
	if gormDB == nil {
		return fmt.Errorf("database is nil")
	}
	if !isSQLiteDev(driver, appEnv) {
		return nil
	}
	if now == nil {
		now = func() int64 { return time.Now().UnixMilli() }
	}

	seedNow := now()
	return gormDB.Transaction(func(tx *gorm.DB) error {
		if err := seedIdentityAndSpace(tx, seedNow); err != nil {
			return err
		}
		if err := seedQuestionPaperAndExam(tx, seedNow); err != nil {
			return err
		}
		return refreshDemoExamWindow(tx, seedNow)
	})
}

func isSQLiteDev(driver string, appEnv string) bool {
	normalizedDriver := strings.ToLower(strings.TrimSpace(driver))
	normalizedEnv := strings.ToLower(strings.TrimSpace(appEnv))
	if normalizedDriver != "sqlite" {
		return false
	}
	return normalizedEnv == "dev" || normalizedEnv == "development" || normalizedEnv == "local"
}

func seedIdentityAndSpace(tx *gorm.DB, now int64) error {
	// 前端默认以平台管理员 1、租户 10、考生 20 进入页面，本地种子数据固定这些 ID。
	if err := insertIgnore(tx, []dbmodel.PlatformUserDO{{
		BaseFields:   baseFields(demoPlatformAdminID, now),
		Username:     "admin",
		Phone:        "13800000001",
		Email:        "admin@example.test",
		PasswordHash: "papermind123",
		Status:       "enabled",
	}}); err != nil {
		return fmt.Errorf("seed platform user: %w", err)
	}

	if err := insertIgnore(tx, []dbmodel.TenantDO{{
		BaseFields:    baseFields(demoTenantID, now),
		Name:          "青藤一中",
		Description:   "本地端到端联调用演示租户",
		TenantCode:    "PM-QT01",
		AllowRegister: true,
		Status:        "enabled",
	}}); err != nil {
		return fmt.Errorf("seed tenant: %w", err)
	}

	if err := insertIgnore(tx, []dbmodel.UserDO{
		{
			BaseFields:   baseFields(demoTenantAdminID, now),
			TenantID:     demoTenantID,
			Username:     "tenant.admin",
			RealName:     "租户管理员",
			Phone:        "13800000010",
			Email:        "tenant-admin@example.test",
			PasswordHash: "papermind123",
			Status:       "enabled",
		},
		{
			BaseFields:   baseFields(demoStudentID, now),
			TenantID:     demoTenantID,
			Username:     "student01",
			RealName:     "张同学",
			Phone:        "13800000020",
			Email:        "student01@example.test",
			PasswordHash: "papermind123",
			Status:       "enabled",
		},
		{
			BaseFields:   baseFields(demoTeacherID, now),
			TenantID:     demoTenantID,
			Username:     "teacher01",
			RealName:     "李老师",
			Phone:        "13800000021",
			Email:        "teacher01@example.test",
			PasswordHash: "papermind123",
			Status:       "enabled",
		},
	}); err != nil {
		return fmt.Errorf("seed tenant users: %w", err)
	}

	if err := insertIgnore(tx, []dbmodel.UserRoleDO{
		{BaseFields: baseFields(1, now), TenantID: demoTenantID, UserID: demoTenantAdminID, Role: "tenant_admin"},
		{BaseFields: baseFields(2, now), TenantID: demoTenantID, UserID: demoStudentID, Role: "student"},
		{BaseFields: baseFields(3, now), TenantID: demoTenantID, UserID: demoTeacherID, Role: "teacher"},
	}); err != nil {
		return fmt.Errorf("seed user roles: %w", err)
	}

	if err := insertIgnore(tx, []dbmodel.SpaceDO{
		{
			BaseFields:  baseFields(demoSpaceID, now),
			TenantID:    demoTenantID,
			Name:        "高一全年级",
			Description: "本地联调考试发布范围",
			Type:        "class",
			Status:      "enabled",
		},
		{
			BaseFields:  baseFields(demoClassSpaceID, now),
			TenantID:    demoTenantID,
			Name:        "高一 1 班",
			Description: "本地联调班级空间",
			Type:        "class",
			Status:      "enabled",
		},
	}); err != nil {
		return fmt.Errorf("seed spaces: %w", err)
	}

	if err := insertIgnore(tx, []dbmodel.SpaceMemberDO{
		{BaseFields: baseFields(1, now), TenantID: demoTenantID, SpaceID: demoSpaceID, UserID: demoTenantAdminID, RoleInSpace: "space_admin", Status: "enabled"},
		{BaseFields: baseFields(2, now), TenantID: demoTenantID, SpaceID: demoSpaceID, UserID: demoStudentID, RoleInSpace: "student", Status: "enabled"},
		{BaseFields: baseFields(3, now), TenantID: demoTenantID, SpaceID: demoSpaceID, UserID: demoTeacherID, RoleInSpace: "teacher", Status: "enabled"},
		{BaseFields: baseFields(4, now), TenantID: demoTenantID, SpaceID: demoClassSpaceID, UserID: demoStudentID, RoleInSpace: "student", Status: "enabled"},
	}); err != nil {
		return fmt.Errorf("seed space members: %w", err)
	}
	return nil
}

func seedQuestionPaperAndExam(tx *gorm.DB, now int64) error {
	if err := insertIgnore(tx, []dbmodel.TagDO{
		{BaseFields: baseFields(1, now), TenantID: demoTenantID, Name: "语文"},
		{BaseFields: baseFields(2, now), TenantID: demoTenantID, Name: "阅读理解"},
	}); err != nil {
		return fmt.Errorf("seed tags: %w", err)
	}

	if err := insertIgnore(tx, []dbmodel.QuestionDO{
		{
			BaseFields:     baseFields(demoSingleQuestionID, now),
			TenantID:       demoTenantID,
			Type:           constant.QuestionTypeSingle,
			Difficulty:     "easy",
			Title:          "《岳阳楼记》中“先天下之忧而忧”体现的核心精神是？",
			Analysis:       "该句体现了先忧后乐的责任意识。",
			ScoreDefault:   "2",
			ShuffleOptions: false,
			Status:         constant.QuestionStatusEnabled,
		},
		{
			BaseFields:     baseFields(demoShortQuestionID, now),
			TenantID:       demoTenantID,
			Type:           constant.QuestionTypeShortText,
			Difficulty:     "medium",
			Title:          "请简述范仲淹“先忧后乐”思想的现实意义。",
			Analysis:       "可从责任意识、公共精神和个人担当展开。",
			ScoreDefault:   "10",
			ShuffleOptions: false,
			Status:         constant.QuestionStatusEnabled,
		},
	}); err != nil {
		return fmt.Errorf("seed questions: %w", err)
	}

	if err := insertIgnore(tx, []dbmodel.QuestionOptionDO{
		{BaseFields: baseFields(1, now), TenantID: demoTenantID, QuestionID: demoSingleQuestionID, OptionKey: "A", SortOrder: 1, Content: "责任担当", IsCorrect: true},
		{BaseFields: baseFields(2, now), TenantID: demoTenantID, QuestionID: demoSingleQuestionID, OptionKey: "B", SortOrder: 2, Content: "隐逸避世", IsDistractor: true},
		{BaseFields: baseFields(3, now), TenantID: demoTenantID, QuestionID: demoSingleQuestionID, OptionKey: "C", SortOrder: 3, Content: "及时行乐", IsDistractor: true},
		{BaseFields: baseFields(4, now), TenantID: demoTenantID, QuestionID: demoSingleQuestionID, OptionKey: "D", SortOrder: 4, Content: "功名至上", IsDistractor: true},
	}); err != nil {
		return fmt.Errorf("seed question options: %w", err)
	}

	if err := insertIgnore(tx, []dbmodel.QuestionTagDO{
		{RelationFields: relationFields(1, now), TenantID: demoTenantID, QuestionID: demoSingleQuestionID, TagID: 1},
		{RelationFields: relationFields(2, now), TenantID: demoTenantID, QuestionID: demoShortQuestionID, TagID: 2},
	}); err != nil {
		return fmt.Errorf("seed question tags: %w", err)
	}

	if err := insertIgnore(tx, []dbmodel.PaperDO{{
		BaseFields:   baseFields(demoPaperID, now),
		TenantID:     demoTenantID,
		Name:         "高一语文月考试卷",
		Description:  "覆盖客观题作答和主观题阅卷的本地联调试卷",
		TotalScore:   "12",
		BuildMode:    constant.BuildModeManual,
		ShowAnalysis: true,
		Status:       "enabled",
	}}); err != nil {
		return fmt.Errorf("seed paper: %w", err)
	}

	if err := insertIgnore(tx, []dbmodel.PaperSectionDO{
		{BaseFields: baseFields(demoSingleSectionID, now), TenantID: demoTenantID, PaperID: demoPaperID, SortOrder: 1, Name: "一、单项选择题", QuestionType: constant.QuestionTypeSingle, Instructions: "每题 2 分", TotalScore: "2", QuestionCount: 1},
		{BaseFields: baseFields(demoShortSectionID, now), TenantID: demoTenantID, PaperID: demoPaperID, SortOrder: 2, Name: "二、简答题", QuestionType: constant.QuestionTypeShortText, Instructions: "结合文本作答", TotalScore: "10", QuestionCount: 1},
	}); err != nil {
		return fmt.Errorf("seed paper sections: %w", err)
	}

	if err := insertIgnore(tx, []dbmodel.PaperSectionQuestionDO{
		{BaseFields: baseFields(1, now), TenantID: demoTenantID, SectionID: demoSingleSectionID, PaperID: demoPaperID, QuestionID: demoSingleQuestionID, SortOrder: 1, Score: "2"},
		{BaseFields: baseFields(2, now), TenantID: demoTenantID, SectionID: demoShortSectionID, PaperID: demoPaperID, QuestionID: demoShortQuestionID, SortOrder: 1, Score: "10"},
	}); err != nil {
		return fmt.Errorf("seed paper questions: %w", err)
	}

	if err := insertIgnore(tx, []dbmodel.ExamDO{{
		BaseFields:      baseFields(demoExamID, now),
		TenantID:        demoTenantID,
		PaperID:         demoPaperID,
		Name:            "高一语文期中考试",
		StartTime:       now - time.Hour.Milliseconds(),
		EndTime:         now + sevenDaysMillis,
		DurationMinutes: 120,
		MaxAttempts:     1,
		ResultStrategy:  "latest",
		PublishMode:     "manual_publish",
		InviteCode:      "PM2026",
		Status:          constant.ExamStatusPublished,
	}}); err != nil {
		return fmt.Errorf("seed exam: %w", err)
	}

	if err := insertIgnore(tx, []dbmodel.ExamTargetDO{{
		RelationFields: relationFields(1, now),
		TenantID:       demoTenantID,
		ExamID:         demoExamID,
		TargetType:     "space",
		TargetID:       demoSpaceID,
	}}); err != nil {
		return fmt.Errorf("seed exam target: %w", err)
	}
	return nil
}

func refreshDemoExamWindow(tx *gorm.DB, now int64) error {
	// 开发库可能长期复用，默认考试每次启动都滚动到当前时间窗口内。
	return tx.Model(&dbmodel.ExamDO{}).
		Where("tenant_id = ?", demoTenantID).
		Where("id = ?", demoExamID).
		Updates(map[string]any{
			"start_time": now - time.Hour.Milliseconds(),
			"end_time":   now + sevenDaysMillis,
			"updated_at": now,
		}).Error
}

func insertIgnore[T any](tx *gorm.DB, rows []T) error {
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error
}

func baseFields(id uint64, now int64) dbmodel.BaseFields {
	return dbmodel.BaseFields{
		ID:        id,
		CreatedAt: now,
		UpdatedAt: now,
		Version:   1,
		ExtJSON:   datatypes.JSON([]byte("{}")),
	}
}

func relationFields(id uint64, now int64) dbmodel.RelationFields {
	return dbmodel.RelationFields{
		ID:        id,
		CreatedAt: now,
		ExtJSON:   datatypes.JSON([]byte("{}")),
	}
}

package db

import (
	"context"
	"errors"
	"testing"

	servicespace "github.com/lifei6671/papermind/server/internal/service/space"
	servicetenantuser "github.com/lifei6671/papermind/server/internal/service/tenantuser"
	"gorm.io/gorm"
)

func TestTenantUserRepositoryListsUsersByCreatedAtDesc(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	if err := gormDB.Exec(`
		INSERT INTO tenants (
			id, name, logo_url, description, tenant_code, allow_register, status,
			created_at, updated_at, ext_json
		) VALUES (10, '青藤一中', '', '', 'PM-QT01', 1, 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO users (
			id, username, real_name, phone, email, password_hash, status,
			created_at, updated_at, ext_json
		) VALUES
			(20, 'old.user', '旧用户', '13800000020', 'old@example.test', 'hash', 'enabled', 5000, 5000, '{}'),
			(21, 'new.user', '新用户', '13800000021', 'new@example.test', 'hash', 'enabled', 1000, 1000, '{}'),
			(22, 'same.time', '同时间用户', '13800000022', 'same@example.test', 'hash', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed users: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO tenant_user_memberships (
			id, tenant_id, user_id, role, status, created_at, updated_at, ext_json
		) VALUES
			(20, 10, 20, 'teacher', 'enabled', 1000, 1000, '{}'),
			(21, 10, 21, 'teacher', 'enabled', 3000, 3000, '{}'),
			(22, 10, 22, 'teacher', 'enabled', 3000, 3000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed memberships: %v", err)
	}

	repo := NewTenantUserRepository(gormDB, TenantUserRepositoryOptions{})
	result, err := repo.ListUsers(context.Background(), servicetenantuser.ListInput{TenantID: 10, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListUsers returned error: %v", err)
	}
	got := make([]uint64, 0, len(result.Items))
	for _, user := range result.Items {
		got = append(got, user.ID)
	}
	assertUint64s(t, got, []uint64{22, 21, 20})
}

func TestTenantUserRepositorySearchesBeforePaginating(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	if err := gormDB.Exec(`
		INSERT INTO tenants (
			id, name, logo_url, description, tenant_code, allow_register, status,
			created_at, updated_at, ext_json
		) VALUES (10, '青藤一中', '', '', 'PM-QT01', 1, 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO users (
			id, username, real_name, phone, email, password_hash, status,
			created_at, updated_at, ext_json
		) VALUES
			(20, 'math.teacher', '数学教师', '13800000020', 'math@example.test', 'hash', 'enabled', 1000, 1000, '{}'),
			(21, 'english.teacher', '英语教师', '13800000021', 'english@example.test', 'hash', 'enabled', 2000, 2000, '{}'),
			(22, 'math.student', '数学学生', '13800000022', 'student@example.test', 'hash', 'enabled', 3000, 3000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed users: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO tenant_user_memberships (
			id, tenant_id, user_id, role, status, created_at, updated_at, ext_json
		) VALUES
			(20, 10, 20, 'teacher', 'enabled', 1000, 1000, '{}'),
			(21, 10, 21, 'teacher', 'enabled', 2000, 2000, '{}'),
			(22, 10, 22, 'student', 'enabled', 3000, 3000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed memberships: %v", err)
	}

	repo := NewTenantUserRepository(gormDB, TenantUserRepositoryOptions{})
	result, err := repo.ListUsers(context.Background(), servicetenantuser.ListInput{TenantID: 10, Page: 1, PageSize: 1, Search: "math"})
	if err != nil {
		t.Fatalf("ListUsers returned error: %v", err)
	}
	if result.Total != 2 || len(result.Items) != 1 {
		t.Fatalf("expected filtered total=2 and one paged item, got %#v", result)
	}
	if result.Items[0].Username != "math.student" {
		t.Fatalf("expected newest matching user first, got %#v", result.Items[0])
	}
}

func TestTenantUserRepositorySearchesLocalizedRoleAndStatusBeforePaginating(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	if err := gormDB.Exec(`
		INSERT INTO tenants (
			id, name, logo_url, description, tenant_code, allow_register, status,
			created_at, updated_at, ext_json
		) VALUES (10, '青藤一中', '', '', 'PM-QT01', 1, 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO users (
			id, username, real_name, phone, email, password_hash, status,
			created_at, updated_at, ext_json
		) VALUES
			(20, 'teacher.enabled', '李老师', '13800000020', 'teacher@example.test', 'hash', 'enabled', 1000, 1000, '{}'),
			(21, 'student.enabled', '张同学', '13800000021', 'student@example.test', 'hash', 'enabled', 2000, 2000, '{}'),
			(22, 'student.disabled', '王同学', '13800000022', 'disabled@example.test', 'hash', 'disabled', 3000, 3000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed users: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO tenant_user_memberships (
			id, tenant_id, user_id, role, status, created_at, updated_at, ext_json
		) VALUES
			(20, 10, 20, 'teacher', 'enabled', 1000, 1000, '{}'),
			(21, 10, 21, 'student', 'enabled', 2000, 2000, '{}'),
			(22, 10, 22, 'student', 'enabled', 3000, 3000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed memberships: %v", err)
	}

	repo := NewTenantUserRepository(gormDB, TenantUserRepositoryOptions{})
	studentResult, err := repo.ListUsers(context.Background(), servicetenantuser.ListInput{TenantID: 10, Page: 1, PageSize: 10, Search: "学生"})
	if err != nil {
		t.Fatalf("ListUsers by localized role returned error: %v", err)
	}
	assertUint64s(t, tenantUserIDs(studentResult.Items), []uint64{22, 21})

	disabledResult, err := repo.ListUsers(context.Background(), servicetenantuser.ListInput{TenantID: 10, Page: 1, PageSize: 10, Search: "禁用"})
	if err != nil {
		t.Fatalf("ListUsers by localized status returned error: %v", err)
	}
	assertUint64s(t, tenantUserIDs(disabledResult.Items), []uint64{22})
}

func TestTenantUserRepositoryFiltersRoleAndStatusBeforePaginating(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	if err := gormDB.Exec(`
		INSERT INTO tenants (
			id, name, logo_url, description, tenant_code, allow_register, status,
			created_at, updated_at, ext_json
		) VALUES (10, '青藤一中', '', '', 'PM-QT01', 1, 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO users (
			id, username, real_name, phone, email, password_hash, status,
			created_at, updated_at, ext_json
		) VALUES
			(20, 'math.teacher', '数学教师', '13800000020', 'teacher@example.test', 'hash', 'enabled', 1000, 1000, '{}'),
			(21, 'math.student.enabled', '数学学生', '13800000021', 'student@example.test', 'hash', 'enabled', 2000, 2000, '{}'),
			(22, 'math.student.disabled', '禁用数学学生', '13800000022', 'disabled@example.test', 'hash', 'disabled', 3000, 3000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed users: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO tenant_user_memberships (
			id, tenant_id, user_id, role, status, created_at, updated_at, ext_json
		) VALUES
			(20, 10, 20, 'teacher', 'enabled', 1000, 1000, '{}'),
			(21, 10, 21, 'student', 'enabled', 2000, 2000, '{}'),
			(22, 10, 22, 'student', 'enabled', 3000, 3000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed memberships: %v", err)
	}

	repo := NewTenantUserRepository(gormDB, TenantUserRepositoryOptions{})
	result, err := repo.ListUsers(context.Background(), servicetenantuser.ListInput{
		TenantID: 10,
		Page:     1,
		PageSize: 1,
		Search:   "math",
		Role:     servicetenantuser.RoleStudent,
		Status:   servicetenantuser.StatusEnabled,
	})
	if err != nil {
		t.Fatalf("ListUsers returned error: %v", err)
	}
	if result.Total != 1 || len(result.Items) != 1 {
		t.Fatalf("expected role/status filtered total=1 and one item, got %#v", result)
	}
	if result.Items[0].ID != 21 {
		t.Fatalf("expected enabled student 21, got %#v", result.Items[0])
	}
}

func TestTenantUserRepositoryExcludesSpaceMembersBeforePaginating(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	if err := gormDB.Exec(`
		INSERT INTO tenants (
			id, name, logo_url, description, tenant_code, allow_register, status,
			created_at, updated_at, ext_json
		) VALUES (10, '青藤一中', '', '', 'PM-QT01', 1, 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO spaces (
			id, tenant_id, name, logo_url, description, type, status,
			created_at, updated_at, ext_json
		) VALUES (301, 10, '高一 1 班', '', '', 'class', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed space: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO users (
			id, username, real_name, phone, email, password_hash, status,
			created_at, updated_at, ext_json
		) VALUES
			(20, 'student.in.space', '空间内学生', '13800000020', 'in@example.test', 'hash', 'enabled', 1000, 1000, '{}'),
			(21, 'student.available.1', '可添加学生一', '13800000021', 'available1@example.test', 'hash', 'enabled', 2000, 2000, '{}'),
			(22, 'student.available.2', '可添加学生二', '13800000022', 'available2@example.test', 'hash', 'enabled', 3000, 3000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed users: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO tenant_user_memberships (
			id, tenant_id, user_id, role, status, created_at, updated_at, ext_json
		) VALUES
			(20, 10, 20, 'student', 'enabled', 1000, 1000, '{}'),
			(21, 10, 21, 'student', 'enabled', 2000, 2000, '{}'),
			(22, 10, 22, 'student', 'enabled', 3000, 3000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed memberships: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO space_members (
			id, tenant_id, space_id, user_id, role_in_space, status,
			created_at, updated_at, ext_json
		) VALUES (20, 10, 301, 20, 'student', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed space member: %v", err)
	}

	repo := NewTenantUserRepository(gormDB, TenantUserRepositoryOptions{})
	result, err := repo.ListUsers(context.Background(), servicetenantuser.ListInput{
		TenantID:       10,
		Page:           1,
		PageSize:       1,
		Search:         "student",
		Role:           servicetenantuser.RoleStudent,
		Status:         servicetenantuser.StatusEnabled,
		ExcludeSpaceID: 301,
	})
	if err != nil {
		t.Fatalf("ListUsers returned error: %v", err)
	}
	if result.Total != 2 || len(result.Items) != 1 {
		t.Fatalf("expected filtered total=2 and one paged item, got %#v", result)
	}
	if result.Items[0].ID != 22 {
		t.Fatalf("expected newest available student first, got %#v", result.Items[0])
	}
}

func TestTenantUserRepositoryUpdateProfileDoesNotTouchOtherTenantUser(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	if err := gormDB.Exec(`
		INSERT INTO tenants (
			id, name, logo_url, description, tenant_code, allow_register, status,
			created_at, updated_at, ext_json
		) VALUES
			(10, '青藤一中', '', '', 'PM-QT01', 1, 'enabled', 1000, 1000, '{}'),
			(11, '松云二中', '', '', 'PM-SY02', 1, 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed tenants: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO users (
			id, username, real_name, phone, email, password_hash, status,
			created_at, updated_at, ext_json
		) VALUES (30, 'other.tenant', '外租户用户', '13800000030', 'other@example.test', 'hash', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed other tenant user: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO tenant_user_memberships (
			id, tenant_id, user_id, role, status, created_at, updated_at, ext_json
		) VALUES (30, 11, 30, 'teacher', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed other tenant membership: %v", err)
	}
	repo := NewTenantUserRepository(gormDB, TenantUserRepositoryOptions{Now: func() int64 { return 2000 }})

	_, err := repo.UpdateProfile(context.Background(), servicetenantuser.UpdateProfileInput{
		TenantID:    10,
		UserID:      30,
		DisplayName: "被越权修改",
		Phone:       "13900000030",
		Email:       "hacked@example.test",
	})
	if !errors.Is(err, servicetenantuser.ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound for other tenant user, got %v", err)
	}

	var user struct {
		RealName string
		Phone    string
		Email    string
	}
	if err := gormDB.Table("users").
		Select("real_name, phone, email").
		Where("id = ?", 30).
		Scan(&user).Error; err != nil {
		t.Fatalf("query other tenant user: %v", err)
	}
	if user.RealName != "外租户用户" || user.Phone != "13800000030" || user.Email != "other@example.test" {
		t.Fatalf("other tenant user should stay unchanged, got %#v", user)
	}
}

func TestTenantUserRepositoryRejectsDisablingLastTenantAdmin(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	seedTenantAdminInvariantData(t, gormDB, false)
	repo := NewTenantUserRepository(gormDB, TenantUserRepositoryOptions{Now: func() int64 { return 2000 }})

	err := repo.UpdateStatus(context.Background(), 10, 20, servicetenantuser.StatusDisabled)
	if !errors.Is(err, servicetenantuser.ErrCannotLoseLastTenantAdmin) {
		t.Fatalf("expected ErrCannotLoseLastTenantAdmin, got %v", err)
	}
	var status string
	if err := gormDB.Table("tenant_user_memberships").Select("status").Where("tenant_id = ? AND user_id = ?", 10, 20).Scan(&status).Error; err != nil {
		t.Fatalf("query user status: %v", err)
	}
	if status != servicetenantuser.StatusEnabled {
		t.Fatalf("last tenant admin disable should rollback, got status %q", status)
	}
}

func TestTenantUserRepositoryAllowsDisablingNonLastTenantAdmin(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	seedTenantAdminInvariantData(t, gormDB, true)
	repo := NewTenantUserRepository(gormDB, TenantUserRepositoryOptions{Now: func() int64 { return 2000 }})

	if err := repo.UpdateStatus(context.Background(), 10, 20, servicetenantuser.StatusDisabled); err != nil {
		t.Fatalf("UpdateStatus returned error: %v", err)
	}
	var status string
	if err := gormDB.Table("tenant_user_memberships").Select("status").Where("tenant_id = ? AND user_id = ?", 10, 20).Scan(&status).Error; err != nil {
		t.Fatalf("query user status: %v", err)
	}
	if status != servicetenantuser.StatusDisabled {
		t.Fatalf("expected target tenant admin disabled, got %q", status)
	}
}

func TestTenantUserRepositoryRejectsDisablingMissingUser(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	seedTenantAdminInvariantData(t, gormDB, true)
	repo := NewTenantUserRepository(gormDB, TenantUserRepositoryOptions{Now: func() int64 { return 2000 }})

	err := repo.UpdateStatus(context.Background(), 10, 404, servicetenantuser.StatusDisabled)
	if !errors.Is(err, servicetenantuser.ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestTenantUserRepositoryRejectsDeletingLastTenantAdmin(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	seedTenantAdminInvariantData(t, gormDB, false)
	repo := NewTenantUserRepository(gormDB, TenantUserRepositoryOptions{Now: func() int64 { return 2000 }})

	err := repo.DeleteUser(context.Background(), 10, 20)
	if !errors.Is(err, servicetenantuser.ErrCannotLoseLastTenantAdmin) {
		t.Fatalf("expected ErrCannotLoseLastTenantAdmin, got %v", err)
	}
	var membershipCount int64
	if err := gormDB.Table("tenant_user_memberships").Where("tenant_id = ? AND user_id = ?", 10, 20).Count(&membershipCount).Error; err != nil {
		t.Fatalf("query user membership: %v", err)
	}
	if membershipCount != 1 {
		t.Fatalf("last tenant admin delete should rollback, got membership count %d", membershipCount)
	}
}

func TestTenantUserRepositoryAllowsDeletingNonLastTenantAdmin(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	seedTenantAdminInvariantData(t, gormDB, true)
	repo := NewTenantUserRepository(gormDB, TenantUserRepositoryOptions{Now: func() int64 { return 2000 }})

	if err := repo.DeleteUser(context.Background(), 10, 20); err != nil {
		t.Fatalf("DeleteUser returned error: %v", err)
	}
	var membershipCount int64
	if err := gormDB.Table("tenant_user_memberships").Where("tenant_id = ? AND user_id = ?", 10, 20).Count(&membershipCount).Error; err != nil {
		t.Fatalf("query user membership: %v", err)
	}
	if membershipCount != 0 {
		t.Fatalf("expected target tenant admin membership removed, got %d", membershipCount)
	}
}

func TestTenantUserRepositoryRejectsChangingLastTenantAdminRole(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	seedTenantAdminInvariantData(t, gormDB, false)
	repo := NewTenantUserRepository(gormDB, TenantUserRepositoryOptions{Now: func() int64 { return 2000 }})

	err := repo.UpdateRole(context.Background(), 10, 20, servicetenantuser.RoleTeacher)
	if !errors.Is(err, servicetenantuser.ErrCannotLoseLastTenantAdmin) {
		t.Fatalf("expected ErrCannotLoseLastTenantAdmin, got %v", err)
	}
	var role string
	if err := gormDB.Table("tenant_user_memberships").Select("role").Where("tenant_id = ? AND user_id = ?", 10, 20).Scan(&role).Error; err != nil {
		t.Fatalf("query user role: %v", err)
	}
	if role != servicetenantuser.RoleTenantAdmin {
		t.Fatalf("last tenant admin role change should rollback, got role %q", role)
	}
}

func TestTenantUserRepositoryAllowsChangingNonLastTenantAdminRole(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	seedTenantAdminInvariantData(t, gormDB, true)
	repo := NewTenantUserRepository(gormDB, TenantUserRepositoryOptions{Now: func() int64 { return 2000 }})

	if err := repo.UpdateRole(context.Background(), 10, 20, servicetenantuser.RoleTeacher); err != nil {
		t.Fatalf("UpdateRole returned error: %v", err)
	}
	var role string
	if err := gormDB.Table("tenant_user_memberships").Select("role").Where("tenant_id = ? AND user_id = ?", 10, 20).Scan(&role).Error; err != nil {
		t.Fatalf("query user role: %v", err)
	}
	if role != servicetenantuser.RoleTeacher {
		t.Fatalf("expected target role teacher, got %q", role)
	}
}

func TestTenantUserRepositoryAllowsPromotingTeacherToTenantAdmin(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	seedTenantAdminInvariantData(t, gormDB, false)
	repo := NewTenantUserRepository(gormDB, TenantUserRepositoryOptions{Now: func() int64 { return 2000 }})

	if err := repo.UpdateRole(context.Background(), 10, 21, servicetenantuser.RoleTenantAdmin); err != nil {
		t.Fatalf("UpdateRole returned error: %v", err)
	}
	var role string
	if err := gormDB.Table("tenant_user_memberships").Select("role").Where("tenant_id = ? AND user_id = ?", 10, 21).Scan(&role).Error; err != nil {
		t.Fatalf("query user role: %v", err)
	}
	if role != servicetenantuser.RoleTenantAdmin {
		t.Fatalf("expected promoted role tenant_admin, got %q", role)
	}
}

func TestTenantUserRepositoryRejectsImportWhenRoleCoverageLosesLastTenantAdmin(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	seedTenantAdminInvariantData(t, gormDB, false)
	repo := NewTenantUserRepository(gormDB, TenantUserRepositoryOptions{Now: func() int64 { return 2000 }})

	_, err := repo.ImportUsers(context.Background(), servicetenantuser.ImportUsersRepositoryInput{
		TenantID: 10,
		ActorID:  22,
		Rows: []servicetenantuser.ImportUsersRepositoryRow{
			{Username: "admin.one", RealName: "管理员一", PasswordHash: "hash-updated", Role: servicetenantuser.RoleTeacher},
			{Username: "new.student", RealName: "新学生", PasswordHash: "hash-new", Role: servicetenantuser.RoleStudent},
		},
	})
	if !errors.Is(err, servicetenantuser.ErrCannotLoseLastTenantAdmin) {
		t.Fatalf("expected ErrCannotLoseLastTenantAdmin, got %v", err)
	}
	var role string
	if err := gormDB.Table("tenant_user_memberships").Select("role").Where("tenant_id = ? AND user_id = ?", 10, 20).Scan(&role).Error; err != nil {
		t.Fatalf("query user role: %v", err)
	}
	if role != servicetenantuser.RoleTenantAdmin {
		t.Fatalf("batch import should rollback downgraded admin role, got %q", role)
	}
	var count int64
	if err := gormDB.Table("tenant_user_memberships AS tum").
		Joins("JOIN users ON users.id = tum.user_id").
		Where("tum.tenant_id = ? AND users.username = ?", 10, "new.student").
		Count(&count).Error; err != nil {
		t.Fatalf("count new user: %v", err)
	}
	if count != 0 {
		t.Fatalf("batch import should rollback created users, got count %d", count)
	}
}

func TestTenantUserRepositoryImportsUsersAndAllowsRoleSwap(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	seedTenantAdminInvariantData(t, gormDB, false)
	repo := NewTenantUserRepository(gormDB, TenantUserRepositoryOptions{Now: func() int64 { return 2000 }})

	result, err := repo.ImportUsers(context.Background(), servicetenantuser.ImportUsersRepositoryInput{
		TenantID: 10,
		ActorID:  22,
		Rows: []servicetenantuser.ImportUsersRepositoryRow{
			{Username: "admin.one", RealName: "管理员一", PasswordHash: "hash-updated", Role: servicetenantuser.RoleTeacher},
			{Username: "teacher.one", RealName: "教师一", PasswordHash: "hash-teacher", Role: servicetenantuser.RoleTenantAdmin},
			{Username: "new.student", RealName: "新学生", PasswordHash: "hash-new", Role: servicetenantuser.RoleStudent},
		},
	})
	if err != nil {
		t.Fatalf("ImportUsers returned error: %v", err)
	}
	if result.SuccessCount != 3 {
		t.Fatalf("expected success count 3, got %d", result.SuccessCount)
	}
	var adminOneRole string
	if err := gormDB.Table("tenant_user_memberships").Select("role").Where("tenant_id = ? AND user_id = ?", 10, 20).Scan(&adminOneRole).Error; err != nil {
		t.Fatalf("query admin role: %v", err)
	}
	if adminOneRole != servicetenantuser.RoleTeacher {
		t.Fatalf("expected admin.one downgraded to teacher, got %q", adminOneRole)
	}
	var teacherOneRole string
	if err := gormDB.Table("tenant_user_memberships").Select("role").Where("tenant_id = ? AND user_id = ?", 10, 21).Scan(&teacherOneRole).Error; err != nil {
		t.Fatalf("query teacher role: %v", err)
	}
	if teacherOneRole != servicetenantuser.RoleTenantAdmin {
		t.Fatalf("expected teacher.one promoted to tenant_admin, got %q", teacherOneRole)
	}
	var newUserRole string
	if err := gormDB.Table("users").
		Select("tum.role").
		Joins("JOIN tenant_user_memberships AS tum ON tum.user_id = users.id").
		Where("tum.tenant_id = ? AND users.username = ?", 10, "new.student").
		Scan(&newUserRole).Error; err != nil {
		t.Fatalf("query new user role: %v", err)
	}
	if newUserRole != servicetenantuser.RoleStudent {
		t.Fatalf("expected new student role, got %q", newUserRole)
	}
}

func TestTenantUserRepositoryRejectsDisablingUserWhenSpaceLosesLastAdmin(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	seedTenantAdminInvariantData(t, gormDB, true)
	if err := gormDB.Exec(`
		INSERT INTO spaces (
			id, tenant_id, name, logo_url, description, type, status,
			created_at, updated_at, ext_json
		) VALUES (301, 10, '高一一班', '', '', 'class', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed space: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO space_members (
			id, tenant_id, space_id, user_id, role_in_space, status,
			created_at, updated_at, ext_json
		) VALUES (20, 10, 301, 20, 'space_admin', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed space member: %v", err)
	}
	repo := NewTenantUserRepository(gormDB, TenantUserRepositoryOptions{Now: func() int64 { return 2000 }})

	err := repo.UpdateStatus(context.Background(), 10, 20, servicetenantuser.StatusDisabled)
	if !errors.Is(err, servicespace.ErrCannotLoseLastSpaceAdmin) {
		t.Fatalf("expected ErrCannotLoseLastSpaceAdmin, got %v", err)
	}
	var status string
	if err := gormDB.Table("tenant_user_memberships").Select("status").Where("tenant_id = ? AND user_id = ?", 10, 20).Scan(&status).Error; err != nil {
		t.Fatalf("query user status: %v", err)
	}
	if status != servicetenantuser.StatusEnabled {
		t.Fatalf("space admin invariant should rollback user disable, got status %q", status)
	}
}

func TestTenantUserRepositoryRejectsDeletingUserWhenSpaceLosesLastAdmin(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	seedTenantAdminInvariantData(t, gormDB, true)
	if err := gormDB.Exec(`
		INSERT INTO spaces (
			id, tenant_id, name, logo_url, description, type, status,
			created_at, updated_at, ext_json
		) VALUES (301, 10, '高一一班', '', '', 'class', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed space: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO space_members (
			id, tenant_id, space_id, user_id, role_in_space, status,
			created_at, updated_at, ext_json
		) VALUES (20, 10, 301, 20, 'space_admin', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed space member: %v", err)
	}
	repo := NewTenantUserRepository(gormDB, TenantUserRepositoryOptions{Now: func() int64 { return 2000 }})

	err := repo.DeleteUser(context.Background(), 10, 20)
	if !errors.Is(err, servicespace.ErrCannotLoseLastSpaceAdmin) {
		t.Fatalf("expected ErrCannotLoseLastSpaceAdmin, got %v", err)
	}
	var membershipCount int64
	if err := gormDB.Table("tenant_user_memberships").Where("tenant_id = ? AND user_id = ?", 10, 20).Count(&membershipCount).Error; err != nil {
		t.Fatalf("query user membership: %v", err)
	}
	if membershipCount != 1 {
		t.Fatalf("space admin invariant should rollback user delete, got membership count %d", membershipCount)
	}
}

func TestTenantUserRepositoryAllowsDisablingUserWhenOnlyDisabledSpaceLosesAdmin(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	seedTenantAdminInvariantData(t, gormDB, true)
	if err := gormDB.Exec(`
		INSERT INTO spaces (
			id, tenant_id, name, logo_url, description, type, status,
			created_at, updated_at, ext_json
		) VALUES (301, 10, '高一一班', '', '', 'class', 'disabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed disabled space: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO space_members (
			id, tenant_id, space_id, user_id, role_in_space, status,
			created_at, updated_at, ext_json
		) VALUES (20, 10, 301, 20, 'space_admin', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed space member: %v", err)
	}
	repo := NewTenantUserRepository(gormDB, TenantUserRepositoryOptions{Now: func() int64 { return 2000 }})

	if err := repo.UpdateStatus(context.Background(), 10, 20, servicetenantuser.StatusDisabled); err != nil {
		t.Fatalf("UpdateStatus returned error: %v", err)
	}
	var status string
	if err := gormDB.Table("tenant_user_memberships").Select("status").Where("tenant_id = ? AND user_id = ?", 10, 20).Scan(&status).Error; err != nil {
		t.Fatalf("query user status: %v", err)
	}
	if status != servicetenantuser.StatusDisabled {
		t.Fatalf("expected user disabled when only disabled space loses admin, got %q", status)
	}
}

func TestTenantUserRepositoryBuildDisableImpactReturnsAdminSpaceIDs(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	seedTenantAdminInvariantData(t, gormDB, true)
	if err := gormDB.Exec(`
		INSERT INTO spaces (
			id, tenant_id, name, logo_url, description, type, status,
			created_at, updated_at, ext_json
		) VALUES
			(301, 10, '高一一班', '', '', 'class', 'enabled', 1000, 1000, '{}'),
			(302, 10, '高一二班', '', '', 'class', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed spaces: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO space_members (
			id, tenant_id, space_id, user_id, role_in_space, status,
			created_at, updated_at, ext_json
		) VALUES
			(20, 10, 301, 20, 'space_admin', 'enabled', 1000, 1000, '{}'),
			(21, 10, 302, 20, 'teacher', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed space members: %v", err)
	}
	repo := NewTenantUserRepository(gormDB, TenantUserRepositoryOptions{Now: func() int64 { return 2000 }})

	impact, err := repo.BuildDisableImpact(context.Background(), 10, 20)
	if err != nil {
		t.Fatalf("BuildDisableImpact returned error: %v", err)
	}
	if len(impact.AdminSpaceIDs) != 1 || impact.AdminSpaceIDs[0] != 301 {
		t.Fatalf("expected admin space 301, got %#v", impact.AdminSpaceIDs)
	}
}

func TestTenantUserRepositoryInvariantQueriesLockAdminRows(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	repo := NewTenantUserRepository(gormDB, TenantUserRepositoryOptions{})

	tenantAdminQuery := repo.tenantAdminLockQuery(context.Background(), gormDB, 10)
	assertUpdateLockingClause(t, tenantAdminQuery.Statement.Clauses["FOR"])

	spaceAdminQuery := repo.spaceAdminLockQuery(context.Background(), gormDB, 10, []uint64{301})
	assertUpdateLockingClause(t, spaceAdminQuery.Statement.Clauses["FOR"])
}

func seedTenantAdminInvariantData(t *testing.T, gormDB interface {
	Exec(sql string, values ...any) *gorm.DB
}, withAnotherAdmin bool) {
	t.Helper()
	if err := gormDB.Exec(`
		INSERT INTO tenants (
			id, name, logo_url, description, tenant_code, allow_register, status,
			created_at, updated_at, ext_json
		) VALUES (10, '青藤一中', '', '', 'PM-QT01', 1, 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO users (
			id, username, real_name, phone, email, password_hash, status,
			created_at, updated_at, ext_json
		) VALUES
			(20, 'admin.one', '管理员一', '13800000020', 'admin20@example.test', 'hash', 'enabled', 1000, 1000, '{}'),
			(21, 'teacher.one', '教师一', '13800000021', 'teacher21@example.test', 'hash', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed users: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO tenant_user_memberships (
			id, tenant_id, user_id, role, status, created_at, updated_at, ext_json
		) VALUES
			(20, 10, 20, 'tenant_admin', 'enabled', 1000, 1000, '{}'),
			(21, 10, 21, 'teacher', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed user roles: %v", err)
	}
	if !withAnotherAdmin {
		return
	}
	if err := gormDB.Exec(`
		INSERT INTO users (
			id, username, real_name, phone, email, password_hash, status,
			created_at, updated_at, ext_json
		) VALUES (22, 'admin.two', '管理员二', '13800000022', 'admin22@example.test', 'hash', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed second admin user: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO tenant_user_memberships (
			id, tenant_id, user_id, role, status, created_at, updated_at, ext_json
		) VALUES (22, 10, 22, 'tenant_admin', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed second admin role: %v", err)
	}
}

func tenantUserIDs(users []servicetenantuser.User) []uint64 {
	ids := make([]uint64, 0, len(users))
	for _, user := range users {
		ids = append(ids, user.ID)
	}
	return ids
}

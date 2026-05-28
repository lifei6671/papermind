package db

import (
	"context"
	"errors"
	"testing"

	servicespace "github.com/lifei6671/papermind/server/internal/service/space"
	servicetenantuser "github.com/lifei6671/papermind/server/internal/service/tenantuser"
	"gorm.io/gorm"
)

func TestTenantUserRepositoryRejectsDisablingLastTenantAdmin(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	seedTenantAdminInvariantData(t, gormDB, false)
	repo := NewTenantUserRepository(gormDB, TenantUserRepositoryOptions{Now: func() int64 { return 2000 }})

	err := repo.UpdateStatus(context.Background(), 10, 20, servicetenantuser.StatusDisabled)
	if !errors.Is(err, servicetenantuser.ErrCannotLoseLastTenantAdmin) {
		t.Fatalf("expected ErrCannotLoseLastTenantAdmin, got %v", err)
	}
	var status string
	if err := gormDB.Table("users").Select("status").Where("tenant_id = ? AND id = ?", 10, 20).Scan(&status).Error; err != nil {
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
	if err := gormDB.Table("users").Select("status").Where("tenant_id = ? AND id = ?", 10, 20).Scan(&status).Error; err != nil {
		t.Fatalf("query user status: %v", err)
	}
	if status != servicetenantuser.StatusDisabled {
		t.Fatalf("expected target tenant admin disabled, got %q", status)
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
	if err := gormDB.Table("users").Select("status").Where("tenant_id = ? AND id = ?", 10, 20).Scan(&status).Error; err != nil {
		t.Fatalf("query user status: %v", err)
	}
	if status != servicetenantuser.StatusEnabled {
		t.Fatalf("space admin invariant should rollback user disable, got status %q", status)
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
	if err := gormDB.Table("users").Select("status").Where("tenant_id = ? AND id = ?", 10, 20).Scan(&status).Error; err != nil {
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
			id, tenant_id, username, real_name, phone, email, password_hash, status,
			created_at, updated_at, ext_json
		) VALUES
			(20, 10, 'admin.one', '管理员一', '13800000020', 'admin20@example.test', 'hash', 'enabled', 1000, 1000, '{}'),
			(21, 10, 'teacher.one', '教师一', '13800000021', 'teacher21@example.test', 'hash', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed users: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO user_roles (
			id, tenant_id, user_id, role, created_at, updated_at, ext_json
		) VALUES
			(20, 10, 20, 'tenant_admin', 1000, 1000, '{}'),
			(21, 10, 21, 'teacher', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed user roles: %v", err)
	}
	if !withAnotherAdmin {
		return
	}
	if err := gormDB.Exec(`
		INSERT INTO users (
			id, tenant_id, username, real_name, phone, email, password_hash, status,
			created_at, updated_at, ext_json
		) VALUES (22, 10, 'admin.two', '管理员二', '13800000022', 'admin22@example.test', 'hash', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed second admin user: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO user_roles (
			id, tenant_id, user_id, role, created_at, updated_at, ext_json
		) VALUES (22, 10, 22, 'tenant_admin', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed second admin role: %v", err)
	}
}

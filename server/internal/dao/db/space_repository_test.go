package db

import (
	"context"
	"errors"
	"testing"

	"github.com/lifei6671/papermind/server/internal/service/pagination"
	servicespace "github.com/lifei6671/papermind/server/internal/service/space"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func TestSpaceRepositoryRejectsLosingLastSpaceAdminAfterMemberChanges(t *testing.T) {
	for _, tc := range []struct {
		name string
		act  func(*SpaceRepository) error
	}{
		{name: "disable", act: func(repo *SpaceRepository) error {
			return repo.DisableMember(context.Background(), 10, 301, 20)
		}},
		{name: "remove", act: func(repo *SpaceRepository) error {
			return repo.RemoveMember(context.Background(), 10, 301, 20)
		}},
		{name: "downgrade", act: func(repo *SpaceRepository) error {
			return repo.UpdateMemberRole(context.Background(), 10, 301, 20, servicespace.RoleTeacher)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gormDB := openExamRepositoryTestDB(t)
			seedSpaceAdminInvariantData(t, gormDB, false)
			repo := NewSpaceRepository(gormDB, SpaceRepositoryOptions{Now: func() int64 { return 2000 }})

			err := tc.act(repo)
			if !errors.Is(err, servicespace.ErrCannotLoseLastSpaceAdmin) {
				t.Fatalf("expected ErrCannotLoseLastSpaceAdmin, got %v", err)
			}
			var member struct {
				Role   string
				Status string
			}
			if err := gormDB.Table("space_members").
				Select("role_in_space AS role, status").
				Where("tenant_id = ? AND space_id = ? AND user_id = ? AND deleted_at = 0", 10, 301, 20).
				Scan(&member).Error; err != nil {
				t.Fatalf("query member after rollback: %v", err)
			}
			if member.Role != servicespace.RoleSpaceAdmin || member.Status != servicespace.StatusEnabled {
				t.Fatalf("member change should rollback, got %#v", member)
			}
		})
	}
}

func TestSpaceRepositoryAllowsChangingNonLastSpaceAdmin(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	seedSpaceAdminInvariantData(t, gormDB, true)
	repo := NewSpaceRepository(gormDB, SpaceRepositoryOptions{Now: func() int64 { return 2000 }})

	if err := repo.UpdateMemberRole(context.Background(), 10, 301, 20, servicespace.RoleTeacher); err != nil {
		t.Fatalf("UpdateMemberRole returned error: %v", err)
	}
	var role string
	if err := gormDB.Table("space_members").
		Select("role_in_space").
		Where("tenant_id = ? AND space_id = ? AND user_id = ?", 10, 301, 20).
		Scan(&role).Error; err != nil {
		t.Fatalf("query role: %v", err)
	}
	if role != servicespace.RoleTeacher {
		t.Fatalf("expected downgraded role, got %q", role)
	}
}

func TestSpaceRepositoryRejectsMissingMemberChanges(t *testing.T) {
	for _, tc := range []struct {
		name string
		act  func(*SpaceRepository) error
	}{
		{name: "disable", act: func(repo *SpaceRepository) error {
			return repo.DisableMember(context.Background(), 10, 301, 404)
		}},
		{name: "remove", act: func(repo *SpaceRepository) error {
			return repo.RemoveMember(context.Background(), 10, 301, 404)
		}},
		{name: "role", act: func(repo *SpaceRepository) error {
			return repo.UpdateMemberRole(context.Background(), 10, 301, 23, servicespace.RoleTeacher)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gormDB := openExamRepositoryTestDB(t)
			seedSpaceAdminInvariantData(t, gormDB, true)
			if err := gormDB.Exec(`
				INSERT INTO users (
					id, tenant_id, username, real_name, phone, email, password_hash, status,
					created_at, updated_at, ext_json
				) VALUES (23, 10, 'teacher.not.member', '非成员教师', '13800000023', 'teacher23@example.test', 'hash', 'enabled', 1000, 1000, '{}')
			`).Error; err != nil {
				t.Fatalf("seed non-member user: %v", err)
			}
			repo := NewSpaceRepository(gormDB, SpaceRepositoryOptions{Now: func() int64 { return 2000 }})

			err := tc.act(repo)
			if !errors.Is(err, servicespace.ErrMemberNotFound) {
				t.Fatalf("expected ErrMemberNotFound, got %v", err)
			}
		})
	}
}

func TestSpaceRepositoryAllowsLosingLastSpaceAdminInDisabledSpace(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	seedSpaceAdminInvariantData(t, gormDB, false)
	if err := gormDB.Table("spaces").
		Where("tenant_id = ? AND id = ?", 10, 301).
		Update("status", servicespace.StatusDisabled).Error; err != nil {
		t.Fatalf("disable space: %v", err)
	}
	repo := NewSpaceRepository(gormDB, SpaceRepositoryOptions{Now: func() int64 { return 2000 }})

	if err := repo.DisableMember(context.Background(), 10, 301, 20); err != nil {
		t.Fatalf("DisableMember returned error: %v", err)
	}
	var status string
	if err := gormDB.Table("space_members").
		Select("status").
		Where("tenant_id = ? AND space_id = ? AND user_id = ?", 10, 301, 20).
		Scan(&status).Error; err != nil {
		t.Fatalf("query member status: %v", err)
	}
	if status != servicespace.StatusDisabled {
		t.Fatalf("expected member disabled in disabled space, got %q", status)
	}
}

func TestSpaceRepositoryUpdateAndDeleteSpaceProfile(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	seedSpaceAdminInvariantData(t, gormDB, true)
	repo := NewSpaceRepository(gormDB, SpaceRepositoryOptions{Now: func() int64 { return 2000 }})

	updated, err := repo.UpdateSpaceProfile(context.Background(), servicespace.UpdateProfileInput{
		TenantID:    10,
		SpaceID:     301,
		Name:        "高一二班",
		LogoURL:     "logos/class2.png",
		Description: "期中考试空间",
		Type:        "class",
	})
	if err != nil {
		t.Fatalf("UpdateSpaceProfile returned error: %v", err)
	}
	if updated.Name != "高一二班" || updated.LogoURL != "logos/class2.png" || updated.Description != "期中考试空间" {
		t.Fatalf("unexpected updated space: %#v", updated)
	}

	// 删除空间保留成员审计记录，但空间本身不再出现在有效列表。
	if err := repo.DeleteSpace(context.Background(), 10, 301); err != nil {
		t.Fatalf("DeleteSpace returned error: %v", err)
	}
	result, err := repo.ListSpaces(context.Background(), 10, pagination.Input{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListSpaces returned error: %v", err)
	}
	if result.Total != 0 || len(result.Items) != 0 {
		t.Fatalf("expected deleted space hidden from list, got %#v", result)
	}
}

func TestSpaceRepositoryListEffectiveMembershipsForUserFiltersInactiveRows(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	seedUserMembershipListData(t, gormDB)
	repo := NewSpaceRepository(gormDB, SpaceRepositoryOptions{Now: func() int64 { return 2000 }})

	members, err := repo.ListEffectiveMembershipsForUser(context.Background(), 10, 21)
	if err != nil {
		t.Fatalf("ListEffectiveMembershipsForUser returned error: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("expected 2 effective memberships, got %d: %#v", len(members), members)
	}
	if members[0].SpaceID != 301 || members[0].Role != servicespace.RoleTeacher {
		t.Fatalf("unexpected first membership: %#v", members[0])
	}
	if members[1].SpaceID != 302 || members[1].Role != servicespace.RoleSpaceAdmin {
		t.Fatalf("unexpected second membership: %#v", members[1])
	}
}

func TestSpaceRepositoryEffectiveMembershipQueriesIgnoreInactiveSpaces(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	seedUserMembershipListData(t, gormDB)
	if err := gormDB.Exec(`
		UPDATE spaces
		SET status = 'disabled'
		WHERE tenant_id = 10 AND id = 301
	`).Error; err != nil {
		t.Fatalf("disable space: %v", err)
	}
	if err := gormDB.Exec(`
		UPDATE spaces
		SET deleted_at = 1700000000000
		WHERE tenant_id = 10 AND id = 302
	`).Error; err != nil {
		t.Fatalf("delete space: %v", err)
	}
	repo := NewSpaceRepository(gormDB, SpaceRepositoryOptions{Now: func() int64 { return 2000 }})

	members, err := repo.ListEffectiveMembershipsForUser(context.Background(), 10, 21)
	if err != nil {
		t.Fatalf("ListEffectiveMembershipsForUser returned error: %v", err)
	}
	if len(members) != 0 {
		t.Fatalf("expected inactive spaces hidden from memberships, got %#v", members)
	}
	_, err = repo.FindMember(context.Background(), 10, 301, 21)
	if !errors.Is(err, servicespace.ErrMemberNotFound) {
		t.Fatalf("expected disabled space member lookup to miss, got %v", err)
	}
}

func TestSpaceRepositoryEffectiveMembershipQueriesIgnoreInactiveUsers(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	seedUserMembershipListData(t, gormDB)
	if err := gormDB.Table("users").
		Where("tenant_id = ? AND id = ?", 10, 21).
		Update("status", "disabled").Error; err != nil {
		t.Fatalf("disable user: %v", err)
	}
	repo := NewSpaceRepository(gormDB, SpaceRepositoryOptions{Now: func() int64 { return 2000 }})

	members, err := repo.ListEffectiveMembershipsForUser(context.Background(), 10, 21)
	if err != nil {
		t.Fatalf("ListEffectiveMembershipsForUser returned error: %v", err)
	}
	if len(members) != 0 {
		t.Fatalf("expected disabled users hidden from memberships, got %#v", members)
	}
	_, err = repo.FindMember(context.Background(), 10, 301, 21)
	if !errors.Is(err, servicespace.ErrMemberNotFound) {
		t.Fatalf("expected disabled user member lookup to miss, got %v", err)
	}
}

func TestSpaceRepositoryCountsOnlyEnabledUsersAsSpaceAdmins(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	seedSpaceAdminInvariantData(t, gormDB, true)
	if err := gormDB.Table("users").
		Where("tenant_id = ? AND id = ?", 10, 22).
		Update("status", "disabled").Error; err != nil {
		t.Fatalf("disable second admin user: %v", err)
	}
	repo := NewSpaceRepository(gormDB, SpaceRepositoryOptions{Now: func() int64 { return 2000 }})

	err := repo.UpdateMemberRole(context.Background(), 10, 301, 20, servicespace.RoleTeacher)
	if !errors.Is(err, servicespace.ErrCannotLoseLastSpaceAdmin) {
		t.Fatalf("expected disabled user not to count as space admin, got %v", err)
	}
}

func TestSpaceRepositoryInvariantQueryLocksSpaceAdminRows(t *testing.T) {
	gormDB := openExamRepositoryTestDB(t)
	repo := NewSpaceRepository(gormDB, SpaceRepositoryOptions{})

	query := repo.spaceAdminLockQuery(context.Background(), gormDB, 10, 301)
	assertUpdateLockingClause(t, query.Statement.Clauses["FOR"])
}

func assertUpdateLockingClause(t *testing.T, lockClause clause.Clause) {
	t.Helper()
	locking, ok := lockClause.Expression.(clause.Locking)
	if !ok {
		t.Fatalf("expected locking clause, got %#v", lockClause.Expression)
	}
	if locking.Strength != "UPDATE" {
		t.Fatalf("expected UPDATE lock, got %q", locking.Strength)
	}
}

func seedSpaceAdminInvariantData(t *testing.T, gormDB interface {
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
		INSERT INTO spaces (
			id, tenant_id, name, logo_url, description, type, status,
			created_at, updated_at, ext_json
		) VALUES (301, 10, '高一一班', '', '', 'class', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed space: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO users (
			id, tenant_id, username, real_name, phone, email, password_hash, status,
			created_at, updated_at, ext_json
		) VALUES
			(20, 10, 'space.admin.one', '空间管理员一', '13800000020', 'admin20@example.test', 'hash', 'enabled', 1000, 1000, '{}'),
			(21, 10, 'teacher.one', '教师一', '13800000021', 'teacher21@example.test', 'hash', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed users: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO space_members (
			id, tenant_id, space_id, user_id, role_in_space, status,
			created_at, updated_at, ext_json
		) VALUES
			(20, 10, 301, 20, 'space_admin', 'enabled', 1000, 1000, '{}'),
			(21, 10, 301, 21, 'teacher', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed space members: %v", err)
	}
	if !withAnotherAdmin {
		return
	}
	if err := gormDB.Exec(`
		INSERT INTO users (
			id, tenant_id, username, real_name, phone, email, password_hash, status,
			created_at, updated_at, ext_json
		) VALUES (22, 10, 'space.admin.two', '空间管理员二', '13800000022', 'admin22@example.test', 'hash', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed second space admin user: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO space_members (
			id, tenant_id, space_id, user_id, role_in_space, status,
			created_at, updated_at, ext_json
		) VALUES (22, 10, 301, 22, 'space_admin', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed second space admin member: %v", err)
	}
}

func seedUserMembershipListData(t *testing.T, gormDB interface {
	Exec(sql string, values ...any) *gorm.DB
}) {
	t.Helper()
	if err := gormDB.Exec(`
		INSERT INTO tenants (
			id, name, logo_url, description, tenant_code, allow_register, status,
			created_at, updated_at, ext_json
		) VALUES
			(10, '青藤一中', '', '', 'PM-QT01', 1, 'enabled', 1000, 1000, '{}'),
			(11, '远山中学', '', '', 'PM-YS01', 1, 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed tenants: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO spaces (
			id, tenant_id, name, logo_url, description, type, status,
			created_at, updated_at, ext_json
		) VALUES
			(301, 10, '高一一班', '', '', 'class', 'enabled', 1000, 1000, '{}'),
			(302, 10, '高一二班', '', '', 'class', 'enabled', 1000, 1000, '{}'),
			(303, 10, '高一三班', '', '', 'class', 'enabled', 1000, 1000, '{}'),
			(304, 10, '高一四班', '', '', 'class', 'enabled', 1000, 1000, '{}'),
			(401, 11, '远山一班', '', '', 'class', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed spaces: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO users (
			id, tenant_id, username, real_name, phone, email, password_hash, status,
			created_at, updated_at, ext_json
		) VALUES
			(21, 10, 'teacher.one', '教师一', '13800000021', 'teacher21@example.test', 'hash', 'enabled', 1000, 1000, '{}'),
			(22, 11, 'teacher.other', '其他教师', '13800000022', 'teacher22@example.test', 'hash', 'enabled', 1000, 1000, '{}')
	`).Error; err != nil {
		t.Fatalf("seed users: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO space_members (
			id, tenant_id, space_id, user_id, role_in_space, status,
			created_at, updated_at, deleted_at, ext_json
		) VALUES
			(31, 10, 301, 21, 'teacher', 'enabled', 1000, 1000, 0, '{}'),
			(32, 10, 302, 21, 'space_admin', 'enabled', 1000, 1000, 0, '{}'),
			(33, 10, 303, 21, 'teacher', 'disabled', 1000, 1000, 0, '{}'),
			(34, 10, 304, 21, 'teacher', 'enabled', 1000, 1000, 1700000000000, '{}'),
			(41, 11, 401, 22, 'teacher', 'enabled', 1000, 1000, 0, '{}')
	`).Error; err != nil {
		t.Fatalf("seed space members: %v", err)
	}
}

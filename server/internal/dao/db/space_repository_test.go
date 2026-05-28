package db

import (
	"context"
	"errors"
	"testing"

	servicespace "github.com/lifei6671/papermind/server/internal/service/space"
	"gorm.io/gorm"
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

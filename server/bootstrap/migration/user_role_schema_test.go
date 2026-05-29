package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUserRoleSchemaMigrationContainsRequiredTablesAndConstraints(t *testing.T) {
	tests := []struct {
		name string
		path string
		want []string
	}{
		{
			name: "postgres",
			path: filepath.Join("..", "..", "data", "migrations", "postgres", "001_tenant_space.sql"),
			want: []string{
				"CREATE TABLE IF NOT EXISTS platform_users",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_platform_users_username_deleted_at ON platform_users (username, deleted_at)",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_platform_users_phone_deleted_at ON platform_users (phone, deleted_at)",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_platform_users_email_deleted_at ON platform_users (email, deleted_at)",
				"CREATE TABLE IF NOT EXISTS platform_configs",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_platform_configs_config_key ON platform_configs (config_key)",
				"CREATE TABLE IF NOT EXISTS users",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_users_username_deleted_at ON users (username, deleted_at)",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_users_phone_deleted_at ON users (phone, deleted_at)",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_users_email_deleted_at ON users (email, deleted_at)",
				"CREATE TABLE IF NOT EXISTS tenant_user_memberships",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_tenant_user_memberships_user ON tenant_user_memberships (tenant_id, user_id)",
			},
		},
		{
			name: "mysql",
			path: filepath.Join("..", "..", "data", "migrations", "mysql", "001_tenant_space.sql"),
			want: []string{
				"CREATE TABLE IF NOT EXISTS platform_users",
				"UNIQUE KEY uk_platform_users_username_deleted_at (username, deleted_at)",
				"UNIQUE KEY uk_platform_users_phone_deleted_at (phone, deleted_at)",
				"UNIQUE KEY uk_platform_users_email_deleted_at (email, deleted_at)",
				"CREATE TABLE IF NOT EXISTS platform_configs",
				"UNIQUE KEY uk_platform_configs_config_key (config_key)",
				"CREATE TABLE IF NOT EXISTS users",
				"UNIQUE KEY uk_users_username_deleted_at (username, deleted_at)",
				"UNIQUE KEY uk_users_phone_deleted_at (phone, deleted_at)",
				"UNIQUE KEY uk_users_email_deleted_at (email, deleted_at)",
				"CREATE TABLE IF NOT EXISTS tenant_user_memberships",
				"UNIQUE KEY uk_tenant_user_memberships_user (tenant_id, user_id)",
			},
		},
		{
			name: "sqlite",
			path: filepath.Join("..", "..", "data", "migrations", "sqlite", "001_tenant_space.sql"),
			want: []string{
				"CREATE TABLE IF NOT EXISTS platform_users",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_platform_users_username_deleted_at ON platform_users (username, deleted_at)",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_platform_users_phone_deleted_at ON platform_users (phone, deleted_at)",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_platform_users_email_deleted_at ON platform_users (email, deleted_at)",
				"CREATE TABLE IF NOT EXISTS platform_configs",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_platform_configs_config_key ON platform_configs (config_key)",
				"CREATE TABLE IF NOT EXISTS users",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_users_username_deleted_at ON users (username, deleted_at)",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_users_phone_deleted_at ON users (phone, deleted_at)",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_users_email_deleted_at ON users (email, deleted_at)",
				"CREATE TABLE IF NOT EXISTS tenant_user_memberships",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_tenant_user_memberships_user ON tenant_user_memberships (tenant_id, user_id)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := os.ReadFile(tt.path)
			if err != nil {
				t.Fatalf("ReadFile() error = %v", err)
			}

			sql := string(content)
			for _, want := range tt.want {
				if !strings.Contains(sql, want) {
					t.Fatalf("%s missing %q", tt.path, want)
				}
			}
		})
	}
}

func TestTenantUserMembershipSingleRoleConstraintDoesNotRequireAppendMigration(t *testing.T) {
	tests := []struct {
		name string
		path string
		want []string
	}{
		{
			name: "postgres",
			path: filepath.Join("..", "..", "data", "migrations", "postgres", "001_tenant_space.sql"),
			want: []string{
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_tenant_user_memberships_user ON tenant_user_memberships (tenant_id, user_id)",
			},
		},
		{
			name: "mysql",
			path: filepath.Join("..", "..", "data", "migrations", "mysql", "001_tenant_space.sql"),
			want: []string{
				"UNIQUE KEY uk_tenant_user_memberships_user (tenant_id, user_id)",
			},
		},
		{
			name: "sqlite",
			path: filepath.Join("..", "..", "data", "migrations", "sqlite", "001_tenant_space.sql"),
			want: []string{
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_tenant_user_memberships_user ON tenant_user_memberships (tenant_id, user_id)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content, err := os.ReadFile(tt.path)
			if err != nil {
				t.Fatalf("ReadFile() error = %v", err)
			}
			sql := string(content)
			for _, want := range tt.want {
				if !strings.Contains(sql, want) {
					t.Fatalf("%s missing %q", tt.path, want)
				}
			}
		})
	}
}

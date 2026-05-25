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
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_users_username_deleted_at ON users (tenant_id, username, deleted_at)",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_users_phone_deleted_at ON users (tenant_id, phone, deleted_at)",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_users_email_deleted_at ON users (tenant_id, email, deleted_at)",
				"CREATE TABLE IF NOT EXISTS user_roles",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_user_roles_role ON user_roles (tenant_id, user_id, role)",
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
				"UNIQUE KEY uk_users_username_deleted_at (tenant_id, username, deleted_at)",
				"UNIQUE KEY uk_users_phone_deleted_at (tenant_id, phone, deleted_at)",
				"UNIQUE KEY uk_users_email_deleted_at (tenant_id, email, deleted_at)",
				"CREATE TABLE IF NOT EXISTS user_roles",
				"UNIQUE KEY uk_user_roles_role (tenant_id, user_id, role)",
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
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_users_username_deleted_at ON users (tenant_id, username, deleted_at)",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_users_phone_deleted_at ON users (tenant_id, phone, deleted_at)",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_users_email_deleted_at ON users (tenant_id, email, deleted_at)",
				"CREATE TABLE IF NOT EXISTS user_roles",
				"CREATE UNIQUE INDEX IF NOT EXISTS uk_user_roles_role ON user_roles (tenant_id, user_id, role)",
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

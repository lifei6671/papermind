package db

import (
	"reflect"
	"strings"
	"testing"
)

func TestUserRoleTablesUseExpectedNames(t *testing.T) {
	tests := []struct {
		name string
		do   interface{ TableName() string }
		want string
	}{
		{name: "platform user", do: PlatformUserDO{}, want: "platform_users"},
		{name: "platform config", do: PlatformConfigDO{}, want: "platform_configs"},
		{name: "tenant user", do: UserDO{}, want: "users"},
		{name: "tenant user membership", do: UserRoleDO{}, want: "tenant_user_memberships"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.do.TableName(); got != tt.want {
				t.Fatalf("TableName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUserRoleBusinessFieldsUseColumnMappings(t *testing.T) {
	tests := []struct {
		model any
		names []string
	}{
		{model: PlatformUserDO{}, names: []string{"Username", "AvatarURL", "Phone", "Email", "PasswordHash", "LastLoginIP", "LastLoginAt", "Status"}},
		{model: PlatformConfigDO{}, names: []string{"ConfigKey", "ConfigValue", "ValueType", "Description"}},
		{model: UserDO{}, names: []string{"Username", "RealName", "AvatarURL", "Phone", "Email", "PasswordHash", "LastLoginIP", "LastLoginAt", "Status"}},
		{model: UserRoleDO{}, names: []string{"TenantID", "UserID", "Role", "Status"}},
	}

	for _, tt := range tests {
		modelType := reflect.TypeOf(tt.model)
		for _, name := range tt.names {
			field, ok := modelType.FieldByName(name)
			if !ok {
				t.Fatalf("%s missing %s", modelType.Name(), name)
			}
			if !strings.Contains(field.Tag.Get("gorm"), "column:") {
				t.Fatalf("%s.%s gorm tag = %q", modelType.Name(), name, field.Tag.Get("gorm"))
			}
		}
	}
}

func TestUserRoleColumnMappings(t *testing.T) {
	if PlatformUserColumns.AvatarURL != "avatar_url" {
		t.Fatalf("PlatformUserColumns.AvatarURL = %q", PlatformUserColumns.AvatarURL)
	}
	if PlatformUserColumns.LastLoginIP != "last_login_ip" {
		t.Fatalf("PlatformUserColumns.LastLoginIP = %q", PlatformUserColumns.LastLoginIP)
	}
	if PlatformUserColumns.LastLoginAt != "last_login_at" {
		t.Fatalf("PlatformUserColumns.LastLoginAt = %q", PlatformUserColumns.LastLoginAt)
	}
	if PlatformConfigColumns.ConfigKey != "config_key" {
		t.Fatalf("PlatformConfigColumns.ConfigKey = %q", PlatformConfigColumns.ConfigKey)
	}
	if UserColumns.RealName != "real_name" {
		t.Fatalf("UserColumns.RealName = %q", UserColumns.RealName)
	}
	if UserColumns.DeletedAt != "deleted_at" {
		t.Fatalf("UserColumns.DeletedAt = %q", UserColumns.DeletedAt)
	}
	if UserRoleColumns.Role != "role" {
		t.Fatalf("UserRoleColumns.Role = %q", UserRoleColumns.Role)
	}
	if UserRoleColumns.Status != "status" {
		t.Fatalf("UserRoleColumns.Status = %q", UserRoleColumns.Status)
	}
}

func TestUserRoleSoftDeleteBoundaries(t *testing.T) {
	assertHasEmbeddedField(t, reflect.TypeOf(PlatformUserDO{}), "SoftDeleteFields")
	assertHasEmbeddedField(t, reflect.TypeOf(UserDO{}), "SoftDeleteFields")
	assertMissingField(t, reflect.TypeOf(PlatformConfigDO{}), "DeletedAt")
	assertMissingField(t, reflect.TypeOf(UserRoleDO{}), "DeletedAt")

	// 租户用户关系需要乐观锁字段，后续角色或成员状态变更可以按版本号避免覆盖。
	assertHasEmbeddedField(t, reflect.TypeOf(UserRoleDO{}), "BaseFields")
}

func assertHasEmbeddedField(t *testing.T, modelType reflect.Type, fieldName string) {
	t.Helper()

	field, ok := modelType.FieldByName(fieldName)
	if !ok {
		t.Fatalf("%s missing embedded %s", modelType.Name(), fieldName)
	}
	if !field.Anonymous {
		t.Fatalf("%s.%s should be embedded", modelType.Name(), fieldName)
	}
}

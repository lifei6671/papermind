package db

import (
	"reflect"
	"strings"
	"testing"
)

func TestTenantSpaceTablesUseExpectedNames(t *testing.T) {
	tests := []struct {
		name string
		do   interface{ TableName() string }
		want string
	}{
		{name: "tenant", do: TenantDO{}, want: "tenants"},
		{name: "space", do: SpaceDO{}, want: "spaces"},
		{name: "space member", do: SpaceMemberDO{}, want: "space_members"},
		{name: "space config", do: SpaceConfigDO{}, want: "space_configs"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.do.TableName(); got != tt.want {
				t.Fatalf("TableName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTenantSpaceBusinessFieldsUseColumnMappings(t *testing.T) {
	tests := []struct {
		model any
		names []string
	}{
		{model: TenantDO{}, names: []string{"Name", "LogoURL", "Description", "TenantCode", "AllowRegister", "Status"}},
		{model: SpaceDO{}, names: []string{"TenantID", "Name", "LogoURL", "Description", "Type", "Status"}},
		{model: SpaceMemberDO{}, names: []string{"TenantID", "SpaceID", "UserID", "RoleInSpace", "Status"}},
		{model: SpaceConfigDO{}, names: []string{"TenantID", "SpaceID", "ConfigKey", "ConfigValue", "ValueType", "Description"}},
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

func TestTenantSpaceColumnMappings(t *testing.T) {
	if TenantColumns.TenantCode != "tenant_code" {
		t.Fatalf("TenantColumns.TenantCode = %q", TenantColumns.TenantCode)
	}
	if TenantColumns.LogoURL != "logo_url" {
		t.Fatalf("TenantColumns.LogoURL = %q", TenantColumns.LogoURL)
	}
	if SpaceColumns.Description != "description" {
		t.Fatalf("SpaceColumns.Description = %q", SpaceColumns.Description)
	}
	if SpaceMemberColumns.RoleInSpace != "role_in_space" {
		t.Fatalf("SpaceMemberColumns.RoleInSpace = %q", SpaceMemberColumns.RoleInSpace)
	}
	if SpaceConfigColumns.ConfigKey != "config_key" {
		t.Fatalf("SpaceConfigColumns.ConfigKey = %q", SpaceConfigColumns.ConfigKey)
	}
}

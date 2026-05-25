package config

import (
	"context"
	"errors"
	"testing"
)

func TestReadPlatformConfigConvertsValueByType(t *testing.T) {
	repo := &fakeRepository{
		platformConfigs: map[string]ConfigItem{
			"site_name":               {Key: "site_name", Value: "PaperMind", ValueType: ValueTypeString},
			"security.allow_register": {Key: "security.allow_register", Value: "true", ValueType: ValueTypeBool},
			"exam.default_duration":   {Key: "exam.default_duration", Value: "90", ValueType: ValueTypeNumber},
			"exam.export_options":     {Key: "exam.export_options", Value: `{"withAnswer":true}`, ValueType: ValueTypeJSON},
		},
	}
	svc := NewService(ServiceOptions{Repo: repo})

	stringValue, err := svc.ReadPlatformConfig(context.Background(), "site_name")
	if err != nil {
		t.Fatalf("ReadPlatformConfig string returned error: %v", err)
	}
	if stringValue.TypedValue != "PaperMind" {
		t.Fatalf("expected string PaperMind, got %#v", stringValue.TypedValue)
	}

	boolValue, err := svc.ReadPlatformConfig(context.Background(), "security.allow_register")
	if err != nil {
		t.Fatalf("ReadPlatformConfig bool returned error: %v", err)
	}
	if boolValue.TypedValue != true {
		t.Fatalf("expected bool true, got %#v", boolValue.TypedValue)
	}

	numberValue, err := svc.ReadPlatformConfig(context.Background(), "exam.default_duration")
	if err != nil {
		t.Fatalf("ReadPlatformConfig number returned error: %v", err)
	}
	if numberValue.TypedValue != float64(90) {
		t.Fatalf("expected number 90, got %#v", numberValue.TypedValue)
	}

	jsonValue, err := svc.ReadPlatformConfig(context.Background(), "exam.export_options")
	if err != nil {
		t.Fatalf("ReadPlatformConfig json returned error: %v", err)
	}
	jsonMap, ok := jsonValue.TypedValue.(map[string]any)
	if !ok || jsonMap["withAnswer"] != true {
		t.Fatalf("expected json map with withAnswer=true, got %#v", jsonValue.TypedValue)
	}
}

func TestReadPlatformConfigFailsFastWhenConversionFails(t *testing.T) {
	repo := &fakeRepository{
		platformConfigs: map[string]ConfigItem{
			"exam.default_duration": {Key: "exam.default_duration", Value: "not-a-number", ValueType: ValueTypeNumber},
		},
	}
	svc := NewService(ServiceOptions{Repo: repo})

	_, err := svc.ReadPlatformConfig(context.Background(), "exam.default_duration")
	if !errors.Is(err, ErrInvalidConfigValue) {
		t.Fatalf("expected ErrInvalidConfigValue, got %v", err)
	}
}

func TestWritePlatformConfigRejectsProductionSecrets(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewService(ServiceOptions{Repo: repo})

	err := svc.WritePlatformConfig(context.Background(), WritePlatformConfigInput{
		Key:         "jwt.secret",
		Value:       "secret-value",
		ValueType:   ValueTypeString,
		Description: "JWT 密钥",
	})
	if !errors.Is(err, ErrSensitivePlatformConfig) {
		t.Fatalf("expected ErrSensitivePlatformConfig, got %v", err)
	}
	if repo.writtenPlatform.Key != "" {
		t.Fatalf("expected sensitive config not to be written, got %#v", repo.writtenPlatform)
	}
}

func TestWritePlatformConfigValidatesAndPersistsValue(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewService(ServiceOptions{Repo: repo})

	err := svc.WritePlatformConfig(context.Background(), WritePlatformConfigInput{
		Key:         "security.allow_register_default",
		Value:       "false",
		ValueType:   ValueTypeBool,
		Description: "是否默认允许自注册",
	})
	if err != nil {
		t.Fatalf("WritePlatformConfig returned error: %v", err)
	}
	if repo.writtenPlatform.Key != "security.allow_register_default" {
		t.Fatalf("expected platform config key saved, got %q", repo.writtenPlatform.Key)
	}
	if repo.writtenPlatform.ValueType != ValueTypeBool {
		t.Fatalf("expected bool value type, got %q", repo.writtenPlatform.ValueType)
	}
	if repo.writtenPlatform.Description != "是否默认允许自注册" {
		t.Fatalf("expected description saved, got %q", repo.writtenPlatform.Description)
	}
}

func TestReadAndWriteSpaceConfigUseTenantAndSpaceScope(t *testing.T) {
	repo := &fakeRepository{
		spaceConfigs: map[spaceConfigKey]ConfigItem{
			{tenantID: 10, spaceID: 100, key: "exam.default_duration"}: {
				TenantID:  10,
				SpaceID:   100,
				Key:       "exam.default_duration",
				Value:     "60",
				ValueType: ValueTypeNumber,
			},
		},
	}
	svc := NewService(ServiceOptions{Repo: repo})

	read, err := svc.ReadSpaceConfig(context.Background(), 10, 100, "exam.default_duration")
	if err != nil {
		t.Fatalf("ReadSpaceConfig returned error: %v", err)
	}
	if read.TypedValue != float64(60) {
		t.Fatalf("expected number 60, got %#v", read.TypedValue)
	}

	err = svc.WriteSpaceConfig(context.Background(), WriteSpaceConfigInput{
		TenantID:    10,
		SpaceID:     100,
		Key:         "exam.shuffle",
		Value:       "true",
		ValueType:   ValueTypeBool,
		Description: "是否打乱题目",
	})
	if err != nil {
		t.Fatalf("WriteSpaceConfig returned error: %v", err)
	}
	if repo.writtenSpace.TenantID != 10 || repo.writtenSpace.SpaceID != 100 || repo.writtenSpace.Key != "exam.shuffle" {
		t.Fatalf("expected scoped space config saved, got %#v", repo.writtenSpace)
	}
}

func TestDeleteConfigUsesHardDeleteRepositoryMethods(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewService(ServiceOptions{Repo: repo})

	if err := svc.DeletePlatformConfig(context.Background(), "security.allow_register_default"); err != nil {
		t.Fatalf("DeletePlatformConfig returned error: %v", err)
	}
	if repo.deletedPlatformKey != "security.allow_register_default" {
		t.Fatalf("expected platform config hard delete key, got %q", repo.deletedPlatformKey)
	}

	if err := svc.DeleteSpaceConfig(context.Background(), 10, 100, "exam.shuffle"); err != nil {
		t.Fatalf("DeleteSpaceConfig returned error: %v", err)
	}
	if repo.deletedSpaceTenantID != 10 || repo.deletedSpaceSpaceID != 100 || repo.deletedSpaceKey != "exam.shuffle" {
		t.Fatalf("expected space config hard delete scope, got tenant=%d space=%d key=%q", repo.deletedSpaceTenantID, repo.deletedSpaceSpaceID, repo.deletedSpaceKey)
	}
}

type fakeRepository struct {
	platformConfigs map[string]ConfigItem
	spaceConfigs    map[spaceConfigKey]ConfigItem

	writtenPlatform ConfigItem
	writtenSpace    ConfigItem

	deletedPlatformKey string

	deletedSpaceTenantID uint64
	deletedSpaceSpaceID  uint64
	deletedSpaceKey      string
}

func (r *fakeRepository) FindPlatformConfig(ctx context.Context, key string) (ConfigItem, error) {
	item, ok := r.platformConfigs[key]
	if !ok {
		return ConfigItem{}, ErrConfigNotFound
	}
	return item, nil
}

func (r *fakeRepository) UpsertPlatformConfig(ctx context.Context, item ConfigItem) error {
	r.writtenPlatform = item
	return nil
}

func (r *fakeRepository) DeletePlatformConfig(ctx context.Context, key string) error {
	r.deletedPlatformKey = key
	return nil
}

func (r *fakeRepository) FindSpaceConfig(ctx context.Context, tenantID uint64, spaceID uint64, key string) (ConfigItem, error) {
	item, ok := r.spaceConfigs[spaceConfigKey{tenantID: tenantID, spaceID: spaceID, key: key}]
	if !ok {
		return ConfigItem{}, ErrConfigNotFound
	}
	return item, nil
}

func (r *fakeRepository) UpsertSpaceConfig(ctx context.Context, item ConfigItem) error {
	r.writtenSpace = item
	return nil
}

func (r *fakeRepository) DeleteSpaceConfig(ctx context.Context, tenantID uint64, spaceID uint64, key string) error {
	r.deletedSpaceTenantID = tenantID
	r.deletedSpaceSpaceID = spaceID
	r.deletedSpaceKey = key
	return nil
}

type spaceConfigKey struct {
	tenantID uint64
	spaceID  uint64
	key      string
}

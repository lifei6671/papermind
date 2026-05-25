package config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	// ValueTypeString 表示配置值按字符串读取。
	ValueTypeString = "string"
	// ValueTypeNumber 表示配置值按数字读取。
	ValueTypeNumber = "number"
	// ValueTypeBool 表示配置值按布尔读取。
	ValueTypeBool = "bool"
	// ValueTypeJSON 表示配置值按 JSON 读取。
	ValueTypeJSON = "json"
)

var (
	ErrConfigNotFound          = errors.New("config not found")
	ErrInvalidConfigValue      = errors.New("invalid config value")
	ErrUnsupportedValueType    = errors.New("unsupported config value type")
	ErrSensitivePlatformConfig = errors.New("sensitive config cannot be stored in platform config")
)

type ConfigItem struct {
	TenantID    uint64 // 所属租户 ID，平台配置为 0。
	SpaceID     uint64 // 空间 ID，平台配置为 0。
	Key         string // 配置键。
	Value       string // 配置值，按字符串保存。
	ValueType   string // 配置值类型：string / number / bool / json。
	Description string // 配置说明。
	TypedValue  any    // 按 value_type 转换后的配置值。
}

type WritePlatformConfigInput struct {
	Key         string // 配置键。
	Value       string // 配置值，按字符串保存。
	ValueType   string // 配置值类型：string / number / bool / json。
	Description string // 配置说明。
}

type WriteSpaceConfigInput struct {
	TenantID    uint64 // 所属租户 ID。
	SpaceID     uint64 // 空间 ID。
	Key         string // 配置键。
	Value       string // 配置值，按字符串保存。
	ValueType   string // 配置值类型：string / number / bool / json。
	Description string // 配置说明。
}

type Repository interface {
	FindPlatformConfig(ctx context.Context, key string) (ConfigItem, error)
	UpsertPlatformConfig(ctx context.Context, item ConfigItem) error
	DeletePlatformConfig(ctx context.Context, key string) error
	FindSpaceConfig(ctx context.Context, tenantID uint64, spaceID uint64, key string) (ConfigItem, error)
	UpsertSpaceConfig(ctx context.Context, item ConfigItem) error
	DeleteSpaceConfig(ctx context.Context, tenantID uint64, spaceID uint64, key string) error
}

type ServiceOptions struct {
	Repo Repository
}

type Service struct {
	repo Repository
}

func NewService(options ServiceOptions) *Service {
	return &Service{repo: options.Repo}
}

func (s *Service) ReadPlatformConfig(ctx context.Context, key string) (ConfigItem, error) {
	item, err := s.repo.FindPlatformConfig(ctx, key)
	if err != nil {
		return ConfigItem{}, err
	}
	return withTypedValue(item)
}

func (s *Service) WritePlatformConfig(ctx context.Context, input WritePlatformConfigInput) error {
	if isSensitivePlatformConfigKey(input.Key) {
		return ErrSensitivePlatformConfig
	}
	item := ConfigItem{
		Key:         input.Key,
		Value:       input.Value,
		ValueType:   input.ValueType,
		Description: input.Description,
	}
	if _, err := withTypedValue(item); err != nil {
		return err
	}
	return s.repo.UpsertPlatformConfig(ctx, item)
}

func (s *Service) DeletePlatformConfig(ctx context.Context, key string) error {
	return s.repo.DeletePlatformConfig(ctx, key)
}

func (s *Service) ReadSpaceConfig(ctx context.Context, tenantID uint64, spaceID uint64, key string) (ConfigItem, error) {
	item, err := s.repo.FindSpaceConfig(ctx, tenantID, spaceID, key)
	if err != nil {
		return ConfigItem{}, err
	}
	return withTypedValue(item)
}

func (s *Service) WriteSpaceConfig(ctx context.Context, input WriteSpaceConfigInput) error {
	item := ConfigItem{
		TenantID:    input.TenantID,
		SpaceID:     input.SpaceID,
		Key:         input.Key,
		Value:       input.Value,
		ValueType:   input.ValueType,
		Description: input.Description,
	}
	if _, err := withTypedValue(item); err != nil {
		return err
	}
	return s.repo.UpsertSpaceConfig(ctx, item)
}

func (s *Service) DeleteSpaceConfig(ctx context.Context, tenantID uint64, spaceID uint64, key string) error {
	return s.repo.DeleteSpaceConfig(ctx, tenantID, spaceID, key)
}

func withTypedValue(item ConfigItem) (ConfigItem, error) {
	value, err := convertValue(item.Value, item.ValueType)
	if err != nil {
		return ConfigItem{}, err
	}
	item.TypedValue = value
	return item, nil
}

func convertValue(raw string, valueType string) (any, error) {
	switch valueType {
	case ValueTypeString:
		return raw, nil
	case ValueTypeNumber:
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, fmt.Errorf("%w: %s", ErrInvalidConfigValue, err.Error())
		}
		return value, nil
	case ValueTypeBool:
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, fmt.Errorf("%w: %s", ErrInvalidConfigValue, err.Error())
		}
		return value, nil
	case ValueTypeJSON:
		var value any
		if err := json.Unmarshal([]byte(raw), &value); err != nil {
			return nil, fmt.Errorf("%w: %s", ErrInvalidConfigValue, err.Error())
		}
		return value, nil
	default:
		return nil, ErrUnsupportedValueType
	}
}

func isSensitivePlatformConfigKey(key string) bool {
	lowerKey := strings.ToLower(key)
	for _, marker := range []string{"secret", "password", "token", "dsn", "database", "private_key", "access_key"} {
		if strings.Contains(lowerKey, marker) {
			return true
		}
	}
	return false
}

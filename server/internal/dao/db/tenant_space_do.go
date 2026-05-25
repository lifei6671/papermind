package db

type TenantDO struct {
	BaseFields
	SoftDeleteFields
	Name          string `gorm:"column:name"`
	LogoURL       string `gorm:"column:logo_url"`
	Description   string `gorm:"column:description"`
	TenantCode    string `gorm:"column:tenant_code"`
	AllowRegister bool   `gorm:"column:allow_register"`
	Status        string `gorm:"column:status"`
}

func (TenantDO) TableName() string {
	return "tenants"
}

// TenantColumns 与 TenantDO 放在同一个文件，租户仓储查询必须引用这里的列名，避免硬编码字段。
var TenantColumns = struct {
	ID            string
	Name          string
	LogoURL       string
	Description   string
	TenantCode    string
	AllowRegister string
	Status        string
	DeletedAt     string
}{
	ID:            BaseColumns.ID,
	Name:          "name",
	LogoURL:       "logo_url",
	Description:   "description",
	TenantCode:    "tenant_code",
	AllowRegister: "allow_register",
	Status:        "status",
	DeletedAt:     SoftDeleteColumns.DeletedAt,
}

type SpaceDO struct {
	BaseFields
	SoftDeleteFields
	TenantID    uint64 `gorm:"column:tenant_id"`
	Name        string `gorm:"column:name"`
	LogoURL     string `gorm:"column:logo_url"`
	Description string `gorm:"column:description"`
	Type        string `gorm:"column:type"`
	Status      string `gorm:"column:status"`
}

func (SpaceDO) TableName() string {
	return "spaces"
}

// SpaceColumns 与 SpaceDO 同文件维护，空间可见性和租户隔离查询统一引用这些列名。
var SpaceColumns = struct {
	ID          string
	TenantID    string
	Name        string
	LogoURL     string
	Description string
	Type        string
	Status      string
	DeletedAt   string
}{
	ID:          BaseColumns.ID,
	TenantID:    "tenant_id",
	Name:        "name",
	LogoURL:     "logo_url",
	Description: "description",
	Type:        "type",
	Status:      "status",
	DeletedAt:   SoftDeleteColumns.DeletedAt,
}

type SpaceMemberDO struct {
	BaseFields
	SoftDeleteFields
	TenantID    uint64 `gorm:"column:tenant_id"`
	SpaceID     uint64 `gorm:"column:space_id"`
	UserID      uint64 `gorm:"column:user_id"`
	RoleInSpace string `gorm:"column:role_in_space"`
	Status      string `gorm:"column:status"`
}

func (SpaceMemberDO) TableName() string {
	return "space_members"
}

// SpaceMemberColumns 与 SpaceMemberDO 同文件维护，成员有效性查询必须同时使用 status 和 deleted_at。
var SpaceMemberColumns = struct {
	ID          string
	TenantID    string
	SpaceID     string
	UserID      string
	RoleInSpace string
	Status      string
	DeletedAt   string
}{
	ID:          BaseColumns.ID,
	TenantID:    "tenant_id",
	SpaceID:     "space_id",
	UserID:      "user_id",
	RoleInSpace: "role_in_space",
	Status:      "status",
	DeletedAt:   SoftDeleteColumns.DeletedAt,
}

type SpaceConfigDO struct {
	BaseFields
	TenantID    uint64 `gorm:"column:tenant_id"`
	SpaceID     uint64 `gorm:"column:space_id"`
	ConfigKey   string `gorm:"column:config_key"`
	ConfigValue string `gorm:"column:config_value"`
	ValueType   string `gorm:"column:value_type"`
	Description string `gorm:"column:description"`
}

func (SpaceConfigDO) TableName() string {
	return "space_configs"
}

// SpaceConfigColumns 与 SpaceConfigDO 同文件维护，配置项删除采用硬删除，不包含 deleted_at。
var SpaceConfigColumns = struct {
	ID          string
	TenantID    string
	SpaceID     string
	ConfigKey   string
	ConfigValue string
	ValueType   string
	Description string
}{
	ID:          BaseColumns.ID,
	TenantID:    "tenant_id",
	SpaceID:     "space_id",
	ConfigKey:   "config_key",
	ConfigValue: "config_value",
	ValueType:   "value_type",
	Description: "description",
}

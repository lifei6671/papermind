package db

type TenantDO struct {
	BaseFields              // 普通业务表公共字段。
	SoftDeleteFields        // 软删除字段。
	Name             string `gorm:"column:name"`           // 租户名称，例如学校、企业、培训机构。
	LogoURL          string `gorm:"column:logo_url"`       // 企业或机构 Logo 地址。
	Description      string `gorm:"column:description"`    // 企业或机构描述。
	TenantCode       string `gorm:"column:tenant_code"`    // 租户码，用于注册归属。
	AllowRegister    bool   `gorm:"column:allow_register"` // 是否允许该租户用户自注册。
	Status           string `gorm:"column:status"`         // 租户状态：enabled / disabled。
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
	BaseFields              // 普通业务表公共字段。
	SoftDeleteFields        // 软删除字段。
	TenantID         uint64 `gorm:"column:tenant_id"`   // 所属租户 ID。
	Name             string `gorm:"column:name"`        // 空间名称，例如班级、专业、课程、培训项目。
	LogoURL          string `gorm:"column:logo_url"`    // 空间 Logo 地址，可为空。
	Description      string `gorm:"column:description"` // 空间描述，可为空。
	Type             string `gorm:"column:type"`        // 空间类型：class / major / course / training / custom。
	Status           string `gorm:"column:status"`      // 空间状态：enabled / disabled。
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
	BaseFields              // 普通业务表公共字段。
	SoftDeleteFields        // 软删除字段。
	TenantID         uint64 `gorm:"column:tenant_id"`     // 所属租户 ID。
	SpaceID          uint64 `gorm:"column:space_id"`      // 空间 ID。
	UserID           uint64 `gorm:"column:user_id"`       // 租户用户 ID。
	RoleInSpace      string `gorm:"column:role_in_space"` // 空间内角色：space_admin / teacher / student。
	Status           string `gorm:"column:status"`        // 空间成员状态：enabled / disabled。
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
	BaseFields         // 普通业务表公共字段。
	TenantID    uint64 `gorm:"column:tenant_id"`    // 所属租户 ID。
	SpaceID     uint64 `gorm:"column:space_id"`     // 空间 ID。
	ConfigKey   string `gorm:"column:config_key"`   // 配置键，例如 default_exam_duration。
	ConfigValue string `gorm:"column:config_value"` // 配置值，按字符串保存。
	ValueType   string `gorm:"column:value_type"`   // 配置值类型：string / number / bool / json。
	Description string `gorm:"column:description"`  // 配置说明。
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

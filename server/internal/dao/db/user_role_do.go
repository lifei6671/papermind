package db

type PlatformUserDO struct {
	BaseFields              // 普通业务表公共字段。
	SoftDeleteFields        // 软删除字段。
	Username         string `gorm:"column:username"`      // 平台管理员登录名。
	AvatarURL        string `gorm:"column:avatar_url"`    // 用户头像地址。
	Phone            string `gorm:"column:phone"`         // 手机号，可用于登录或找回账号。
	Email            string `gorm:"column:email"`         // 邮箱，可用于登录或通知。
	PasswordHash     string `gorm:"column:password_hash"` // 密码哈希。
	LastLoginIP      string `gorm:"column:last_login_ip"` // 最后登录 IP。
	LastLoginAt      int64  `gorm:"column:last_login_at"` // 最后登录时间，Unix 毫秒时间戳。
	Status           string `gorm:"column:status"`        // 平台管理员状态：enabled / disabled。
}

func (PlatformUserDO) TableName() string {
	return "platform_users"
}

// PlatformUserColumns 与 PlatformUserDO 同文件维护，平台账号登录和禁用校验统一引用这些列名。
var PlatformUserColumns = struct {
	ID           string
	Username     string
	AvatarURL    string
	Phone        string
	Email        string
	PasswordHash string
	LastLoginIP  string
	LastLoginAt  string
	Status       string
	DeletedAt    string
}{
	ID:           BaseColumns.ID,
	Username:     "username",
	AvatarURL:    "avatar_url",
	Phone:        "phone",
	Email:        "email",
	PasswordHash: "password_hash",
	LastLoginIP:  "last_login_ip",
	LastLoginAt:  "last_login_at",
	Status:       "status",
	DeletedAt:    SoftDeleteColumns.DeletedAt,
}

type PlatformConfigDO struct {
	BaseFields         // 普通业务表公共字段。
	ConfigKey   string `gorm:"column:config_key"`   // 配置键，例如 allow_register_default。
	ConfigValue string `gorm:"column:config_value"` // 配置值，按字符串保存。
	ValueType   string `gorm:"column:value_type"`   // 配置值类型：string / number / bool / json。
	Description string `gorm:"column:description"`  // 配置说明。
}

func (PlatformConfigDO) TableName() string {
	return "platform_configs"
}

// PlatformConfigColumns 与 PlatformConfigDO 同文件维护，平台配置采用硬删除，不包含 deleted_at。
var PlatformConfigColumns = struct {
	ID          string
	ConfigKey   string
	ConfigValue string
	ValueType   string
	Description string
}{
	ID:          BaseColumns.ID,
	ConfigKey:   "config_key",
	ConfigValue: "config_value",
	ValueType:   "value_type",
	Description: "description",
}

type UserDO struct {
	BaseFields              // 普通业务表公共字段。
	SoftDeleteFields        // 软删除字段。
	TenantID         uint64 `gorm:"column:tenant_id"`     // 所属租户 ID。
	Username         string `gorm:"column:username"`      // 租户内登录名。
	RealName         string `gorm:"column:real_name"`     // 真实姓名，用于阅卷、成绩单和导出。
	AvatarURL        string `gorm:"column:avatar_url"`    // 用户头像地址。
	Phone            string `gorm:"column:phone"`         // 手机号，可用于登录或通知。
	Email            string `gorm:"column:email"`         // 邮箱，可用于登录或通知。
	PasswordHash     string `gorm:"column:password_hash"` // 密码哈希。
	LastLoginIP      string `gorm:"column:last_login_ip"` // 最后登录 IP。
	LastLoginAt      int64  `gorm:"column:last_login_at"` // 最后登录时间，Unix 毫秒时间戳。
	Status           string `gorm:"column:status"`        // 用户状态：enabled / disabled。
}

func (UserDO) TableName() string {
	return "users"
}

// UserColumns 与 UserDO 同文件维护，租户用户查询必须带 tenant_id，避免跨租户数据泄漏。
var UserColumns = struct {
	ID           string
	TenantID     string
	Username     string
	RealName     string
	AvatarURL    string
	Phone        string
	Email        string
	PasswordHash string
	LastLoginIP  string
	LastLoginAt  string
	Status       string
	DeletedAt    string
}{
	ID:           BaseColumns.ID,
	TenantID:     "tenant_id",
	Username:     "username",
	RealName:     "real_name",
	AvatarURL:    "avatar_url",
	Phone:        "phone",
	Email:        "email",
	PasswordHash: "password_hash",
	LastLoginIP:  "last_login_ip",
	LastLoginAt:  "last_login_at",
	Status:       "status",
	DeletedAt:    SoftDeleteColumns.DeletedAt,
}

type UserRoleDO struct {
	BaseFields        // 普通业务表公共字段。
	TenantID   uint64 `gorm:"column:tenant_id"` // 所属租户 ID。
	UserID     uint64 `gorm:"column:user_id"`   // 租户用户 ID。
	Role       string `gorm:"column:role"`      // 用户角色：tenant_admin / teacher / student。
}

func (UserRoleDO) TableName() string {
	return "user_roles"
}

// UserRoleColumns 与 UserRoleDO 同文件维护，固定角色权限判断统一引用 role 字段。
var UserRoleColumns = struct {
	ID       string
	TenantID string
	UserID   string
	Role     string
}{
	ID:       BaseColumns.ID,
	TenantID: "tenant_id",
	UserID:   "user_id",
	Role:     "role",
}

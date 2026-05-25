package db

import (
	"gorm.io/datatypes"
	"gorm.io/plugin/soft_delete"
)

// BaseFields 是普通业务表的公共字段集合。
// 业务表默认继承创建、更新、乐观锁和扩展 JSON 字段，追加日志表和纯关系表如需豁免必须在具体实体中显式声明。
type BaseFields struct {
	ID        uint64         `gorm:"column:id;primaryKey"` // 主键 ID。
	CreatedAt int64          `gorm:"column:created_at"`    // 创建时间，Unix 毫秒时间戳。
	CreatedBy uint64         `gorm:"column:created_by"`    // 创建人用户 ID。
	UpdatedAt int64          `gorm:"column:updated_at"`    // 更新时间，Unix 毫秒时间戳。
	UpdatedBy uint64         `gorm:"column:updated_by"`    // 更新人用户 ID。
	Version   int64          `gorm:"column:version"`       // 数据版本号，用于乐观锁。
	ExtJSON   datatypes.JSON `gorm:"column:ext_json"`      // JSON 扩展字段，保存非主流程元数据。
}

// BaseColumns 与 BaseFields 放在同一个文件，Repository 拼接字段时只能引用这里的列名，避免散落硬编码。
var BaseColumns = struct {
	ID        string
	CreatedAt string
	CreatedBy string
	UpdatedAt string
	UpdatedBy string
	Version   string
	ExtJSON   string
}{
	ID:        "id",
	CreatedAt: "created_at",
	CreatedBy: "created_by",
	UpdatedAt: "updated_at",
	UpdatedBy: "updated_by",
	Version:   "version",
	ExtJSON:   "ext_json",
}

// SoftDeleteFields 只嵌入需要软删除的核心主表。
// 使用 Unix 毫秒时间戳而不是 NULL，保证 PostgreSQL、MySQL 和 SQLite 的唯一索引行为一致。
type SoftDeleteFields struct {
	DeletedAt soft_delete.DeletedAt `gorm:"column:deleted_at;softDelete:milli;default:0"` // 软删除时间，0 表示未删除。
}

// SoftDeleteColumns 与 SoftDeleteFields 同文件维护，软删除过滤和唯一索引声明统一引用该字段名。
var SoftDeleteColumns = struct {
	DeletedAt string
}{
	DeletedAt: "deleted_at",
}

// RelationFields 用于 question_tags、exam_targets 等纯关系表。
// 这类表只通过新增和删除维护关系，不存在业务更新动作，因此不放 updated_at、updated_by 和 version。
type RelationFields struct {
	ID        uint64         `gorm:"column:id;primaryKey"` // 关系记录主键 ID。
	CreatedAt int64          `gorm:"column:created_at"`    // 创建时间，Unix 毫秒时间戳。
	CreatedBy uint64         `gorm:"column:created_by"`    // 创建人用户 ID。
	ExtJSON   datatypes.JSON `gorm:"column:ext_json"`      // JSON 扩展字段，保存非主流程元数据。
}

// RelationColumns 与 RelationFields 同文件维护，关系表 Repository 只能引用这些列名做字段选择和条件拼接。
var RelationColumns = struct {
	ID        string
	CreatedAt string
	CreatedBy string
	ExtJSON   string
}{
	ID:        "id",
	CreatedAt: "created_at",
	CreatedBy: "created_by",
	ExtJSON:   "ext_json",
}

// EventFields 用于 exam_events 等追加写日志表。
// 事件写入后不可修改，省略 updated_at、updated_by 和 version，避免误导维护者把事件当作可更新记录。
type EventFields struct {
	ID        uint64         `gorm:"column:id;primaryKey"` // 事件记录主键 ID。
	CreatedAt int64          `gorm:"column:created_at"`    // 创建时间，Unix 毫秒时间戳。
	CreatedBy uint64         `gorm:"column:created_by"`    // 创建人用户 ID。
	ExtJSON   datatypes.JSON `gorm:"column:ext_json"`      // JSON 扩展字段，保存非主流程元数据。
}

// EventColumns 与 EventFields 同文件维护，日志查询只引用这里的列名，保持字段拼接一致。
var EventColumns = struct {
	ID        string
	CreatedAt string
	CreatedBy string
	ExtJSON   string
}{
	ID:        "id",
	CreatedAt: "created_at",
	CreatedBy: "created_by",
	ExtJSON:   "ext_json",
}

package db

import (
	"fmt"

	"gorm.io/gorm"
)

// LikeIgnoreCase 按当前数据库方言生成大小写不敏感的模糊匹配条件。
// PostgreSQL 使用 ILIKE；MySQL 依赖建表默认大小写不敏感 collation；SQLite 的 LIKE 默认对 ASCII 不敏感。
func LikeIgnoreCase(db *gorm.DB, column, value string) *gorm.DB {
	return db.Where(likeIgnoreCaseClause(db, column), likeIgnoreCasePattern(value))
}

func likeIgnoreCaseClause(db *gorm.DB, column string) string {
	switch db.Dialector.Name() {
	case "postgres":
		return fmt.Sprintf("%s ILIKE ?", column)
	case "mysql", "sqlite":
		return fmt.Sprintf("%s LIKE ?", column)
	default:
		return fmt.Sprintf("LOWER(%s) LIKE LOWER(?)", column)
	}
}

func likeIgnoreCasePattern(value string) string {
	return "%" + value + "%"
}

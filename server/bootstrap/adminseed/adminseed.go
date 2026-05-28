package adminseed

import (
	"context"
	"fmt"

	dbmodel "github.com/lifei6671/papermind/server/internal/dao/db"
	serviceplatformuser "github.com/lifei6671/papermind/server/internal/service/platformuser"
	"github.com/lifei6671/papermind/server/library/crypto"
	"gorm.io/gorm"
)

const (
	defaultPlatformAdminUsername = "admin"
	defaultPlatformAdminEmail    = "admin@iminho.me"
	defaultPlatformAdminPassword = "admin123"
)

// Run 在平台管理员表为空时创建内置管理员，保证全新数据库迁移后可直接登录后台。
func Run(ctx context.Context, gormDB *gorm.DB) error {
	if gormDB == nil {
		return fmt.Errorf("database is nil")
	}
	passwordHash, err := crypto.HashPassword(defaultPlatformAdminPassword)
	if err != nil {
		return fmt.Errorf("hash default platform admin password: %w", err)
	}

	service := serviceplatformuser.NewService(serviceplatformuser.ServiceOptions{
		Repo: dbmodel.NewPlatformUserRepository(gormDB, dbmodel.PlatformUserRepositoryOptions{}),
	})
	if _, err := service.SeedAdmin(ctx, serviceplatformuser.SeedAdminInput{
		Username:     defaultPlatformAdminUsername,
		Email:        defaultPlatformAdminEmail,
		PasswordHash: passwordHash,
	}); err != nil {
		return fmt.Errorf("seed default platform admin: %w", err)
	}
	return nil
}

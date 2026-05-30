package adminseed

import (
	"context"
	"fmt"
	"strings"

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

// Run 在开发环境创建默认平台管理员，供测试和本地联调用。
func Run(ctx context.Context, gormDB *gorm.DB) error {
	return RunForEnv(ctx, gormDB, "dev")
}

// RunForEnv 只在开发、本地和测试环境创建公开默认管理员，避免生产空库暴露固定高权限账号。
func RunForEnv(ctx context.Context, gormDB *gorm.DB, appEnv string) error {
	if gormDB == nil {
		return fmt.Errorf("database is nil")
	}
	if !allowsDefaultPlatformAdminSeed(appEnv) {
		return nil
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

func allowsDefaultPlatformAdminSeed(appEnv string) bool {
	switch strings.ToLower(strings.TrimSpace(appEnv)) {
	case "dev", "development", "local", "test":
		return true
	default:
		return false
	}
}

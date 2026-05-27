package db

import (
	"context"
	"errors"
	"time"

	serviceplatformuser "github.com/lifei6671/papermind/server/internal/service/platformuser"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type PlatformUserRepository struct {
	db  *gorm.DB
	now func() int64
}

type PlatformUserRepositoryOptions struct {
	Now func() int64
}

func NewPlatformUserRepository(gormDB *gorm.DB, options PlatformUserRepositoryOptions) *PlatformUserRepository {
	now := options.Now
	if now == nil {
		now = func() int64 { return time.Now().UnixMilli() }
	}
	return &PlatformUserRepository{db: gormDB, now: now}
}

func (r *PlatformUserRepository) HasAny(ctx context.Context) (bool, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&PlatformUserDO{}).
		Where(PlatformUserColumns.DeletedAt+" = ?", 0).
		Count(&total).Error; err != nil {
		return false, err
	}
	return total > 0, nil
}

func (r *PlatformUserRepository) Create(ctx context.Context, user serviceplatformuser.PlatformUser) (serviceplatformuser.PlatformUser, error) {
	now := r.now()
	row := PlatformUserDO{
		BaseFields: BaseFields{
			CreatedAt: now,
			UpdatedAt: now,
			Version:   1,
			ExtJSON:   datatypes.JSON("{}"),
		},
		Username:     user.Username,
		AvatarURL:    user.AvatarURL,
		Phone:        generatedPlatformUserPhone(user.Username),
		Email:        generatedPlatformUserEmail(user.Username),
		PasswordHash: user.PasswordHash,
		Status:       user.Status,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return serviceplatformuser.PlatformUser{}, err
	}
	return platformUserFromDO(row), nil
}

func (r *PlatformUserRepository) FindByUsername(ctx context.Context, username string) (serviceplatformuser.PlatformUser, error) {
	var row PlatformUserDO
	err := r.db.WithContext(ctx).
		Where(PlatformUserColumns.Username+" = ?", username).
		Where(PlatformUserColumns.DeletedAt+" = ?", 0).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return serviceplatformuser.PlatformUser{}, serviceplatformuser.ErrPlatformUserNotFound
	}
	if err != nil {
		return serviceplatformuser.PlatformUser{}, err
	}
	return platformUserFromDO(row), nil
}

func (r *PlatformUserRepository) FindByID(ctx context.Context, userID uint64) (serviceplatformuser.PlatformUser, error) {
	var row PlatformUserDO
	err := r.db.WithContext(ctx).
		Where(PlatformUserColumns.ID+" = ?", userID).
		Where(PlatformUserColumns.DeletedAt+" = ?", 0).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return serviceplatformuser.PlatformUser{}, serviceplatformuser.ErrPlatformUserNotFound
	}
	if err != nil {
		return serviceplatformuser.PlatformUser{}, err
	}
	return platformUserFromDO(row), nil
}

func (r *PlatformUserRepository) CountEnabledAdmins(ctx context.Context) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).Model(&PlatformUserDO{}).
		Where(PlatformUserColumns.Status+" = ?", serviceplatformuser.StatusEnabled).
		Where(PlatformUserColumns.DeletedAt+" = ?", 0).
		Count(&total).Error
	return total, err
}

func (r *PlatformUserRepository) UpdateLoginAudit(ctx context.Context, userID uint64, ip string, at int64) error {
	return r.db.WithContext(ctx).Model(&PlatformUserDO{}).
		Where(PlatformUserColumns.ID+" = ?", userID).
		Where(PlatformUserColumns.DeletedAt+" = ?", 0).
		Updates(map[string]any{
			PlatformUserColumns.LastLoginIP: ip,
			PlatformUserColumns.LastLoginAt: at,
			BaseColumns.UpdatedAt:           r.now(),
			BaseColumns.Version:             gorm.Expr(BaseColumns.Version + " + 1"),
		}).Error
}

func (r *PlatformUserRepository) UpdateStatus(ctx context.Context, userID uint64, status string) error {
	return r.db.WithContext(ctx).Model(&PlatformUserDO{}).
		Where(PlatformUserColumns.ID+" = ?", userID).
		Where(PlatformUserColumns.DeletedAt+" = ?", 0).
		Updates(map[string]any{
			PlatformUserColumns.Status: status,
			BaseColumns.UpdatedAt:      r.now(),
			BaseColumns.Version:        gorm.Expr(BaseColumns.Version + " + 1"),
		}).Error
}

func (r *PlatformUserRepository) UpdateAvatarURL(ctx context.Context, userID uint64, url string) error {
	return r.db.WithContext(ctx).Model(&PlatformUserDO{}).
		Where(PlatformUserColumns.ID+" = ?", userID).
		Where(PlatformUserColumns.DeletedAt+" = ?", 0).
		Updates(map[string]any{
			PlatformUserColumns.AvatarURL: url,
			BaseColumns.UpdatedAt:         r.now(),
			BaseColumns.Version:           gorm.Expr(BaseColumns.Version + " + 1"),
		}).Error
}

func platformUserFromDO(row PlatformUserDO) serviceplatformuser.PlatformUser {
	return serviceplatformuser.PlatformUser{
		ID:           row.ID,
		Username:     row.Username,
		AvatarURL:    row.AvatarURL,
		PasswordHash: row.PasswordHash,
		LastLoginIP:  row.LastLoginIP,
		LastLoginAt:  row.LastLoginAt,
		Status:       row.Status,
	}
}

func generatedPlatformUserPhone(username string) string {
	return "pm-platform-" + username
}

func generatedPlatformUserEmail(username string) string {
	return username + "@papermind.local"
}

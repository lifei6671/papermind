package db

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/lifei6671/papermind/server/internal/service/pagination"
	servicetenantuser "github.com/lifei6671/papermind/server/internal/service/tenantuser"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type TenantUserRepository struct {
	db  *gorm.DB
	now func() int64
}

type TenantUserRepositoryOptions struct {
	Now func() int64
}

func NewTenantUserRepository(gormDB *gorm.DB, options TenantUserRepositoryOptions) *TenantUserRepository {
	now := options.Now
	if now == nil {
		now = func() int64 { return time.Now().UnixMilli() }
	}
	return &TenantUserRepository{db: gormDB, now: now}
}

func (r *TenantUserRepository) ListUsers(ctx context.Context, tenantID uint64, page pagination.Input) (pagination.Result[servicetenantuser.User], error) {
	page = pagination.Normalize(page)
	query := r.db.WithContext(ctx).Model(&UserDO{}).
		Where(UserColumns.TenantID+" = ?", tenantID).
		Where(UserColumns.DeletedAt+" = ?", 0)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return pagination.Result[servicetenantuser.User]{}, err
	}
	var rows []UserDO
	if err := query.
		Order(UserColumns.ID + " ASC").
		Limit(page.PageSize).
		Offset(pagination.Offset(page)).
		Find(&rows).Error; err != nil {
		return pagination.Result[servicetenantuser.User]{}, err
	}
	roles, err := r.listRolesByUserIDs(ctx, tenantID, userIDsFromRows(rows))
	if err != nil {
		return pagination.Result[servicetenantuser.User]{}, err
	}
	users := make([]servicetenantuser.User, 0, len(rows))
	for _, row := range rows {
		users = append(users, tenantUserFromDO(row, roleOrStudent(roles[row.ID])))
	}
	return pagination.Result[servicetenantuser.User]{
		Items:    users,
		Page:     page.Page,
		PageSize: page.PageSize,
		Total:    total,
	}, nil
}

func (r *TenantUserRepository) FindUserByID(ctx context.Context, tenantID uint64, userID uint64) (servicetenantuser.User, error) {
	var row UserDO
	err := r.db.WithContext(ctx).
		Where(UserColumns.TenantID+" = ?", tenantID).
		Where(UserColumns.ID+" = ?", userID).
		Where(UserColumns.DeletedAt+" = ?", 0).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return servicetenantuser.User{}, servicetenantuser.ErrUserNotFound
	}
	if err != nil {
		return servicetenantuser.User{}, err
	}
	role, err := r.findRoleByUserID(ctx, tenantID, userID)
	if err != nil {
		return servicetenantuser.User{}, err
	}
	return tenantUserFromDO(row, roleOrStudent(role)), nil
}

func (r *TenantUserRepository) FindTenantByCode(ctx context.Context, tenantCode string) (servicetenantuser.Tenant, error) {
	var row TenantDO
	err := r.db.WithContext(ctx).
		Where(TenantColumns.TenantCode+" = ?", tenantCode).
		Where(TenantColumns.DeletedAt+" = ?", 0).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return servicetenantuser.Tenant{}, servicetenantuser.ErrTenantNotFound
	}
	if err != nil {
		return servicetenantuser.Tenant{}, err
	}
	return servicetenantuser.Tenant{
		ID:            row.ID,
		TenantCode:    row.TenantCode,
		AllowRegister: row.AllowRegister,
		Status:        row.Status,
	}, nil
}

func (r *TenantUserRepository) CreateUser(ctx context.Context, user servicetenantuser.User) (servicetenantuser.User, error) {
	now := r.now()
	row := UserDO{
		BaseFields: BaseFields{
			CreatedAt: now,
			UpdatedAt: now,
			Version:   1,
			ExtJSON:   datatypes.JSON("{}"),
		},
		TenantID:     user.TenantID,
		Username:     user.Username,
		RealName:     user.RealName,
		AvatarURL:    user.AvatarURL,
		Phone:        user.Phone,
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
		Status:       user.Status,
	}
	if row.Phone == "" {
		row.Phone = generatedUserPhone(user.TenantID, user.Username)
	}
	if row.Email == "" {
		row.Email = generatedUserEmail(user.TenantID, user.Username)
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return servicetenantuser.User{}, err
	}
	return tenantUserFromDO(row, user.Role), nil
}

func (r *TenantUserRepository) CreateUserRole(ctx context.Context, tenantID uint64, userID uint64, role string) error {
	now := r.now()
	row := UserRoleDO{
		BaseFields: BaseFields{
			CreatedAt: now,
			UpdatedAt: now,
			Version:   1,
			ExtJSON:   datatypes.JSON("{}"),
		},
		TenantID: tenantID,
		UserID:   userID,
		Role:     role,
	}
	return r.db.WithContext(ctx).Create(&row).Error
}

func (r *TenantUserRepository) FindUserByUsername(ctx context.Context, tenantID uint64, username string) (servicetenantuser.User, error) {
	var row UserDO
	err := r.db.WithContext(ctx).
		Where(UserColumns.TenantID+" = ?", tenantID).
		Where(UserColumns.Username+" = ?", username).
		Where(UserColumns.DeletedAt+" = ?", 0).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return servicetenantuser.User{}, servicetenantuser.ErrUserNotFound
	}
	if err != nil {
		return servicetenantuser.User{}, err
	}
	return tenantUserFromDO(row, ""), nil
}

func (r *TenantUserRepository) UpdateLoginAudit(ctx context.Context, tenantID uint64, userID uint64, ip string, at int64) error {
	return r.db.WithContext(ctx).Model(&UserDO{}).
		Where(UserColumns.TenantID+" = ?", tenantID).
		Where(UserColumns.ID+" = ?", userID).
		Where(UserColumns.DeletedAt+" = ?", 0).
		Updates(map[string]any{
			UserColumns.LastLoginIP: ip,
			UserColumns.LastLoginAt: at,
			BaseColumns.UpdatedAt:   r.now(),
			BaseColumns.Version:     gorm.Expr(BaseColumns.Version + " + 1"),
		}).Error
}

func (r *TenantUserRepository) UpdateAvatarURL(ctx context.Context, tenantID uint64, userID uint64, url string) error {
	return r.db.WithContext(ctx).Model(&UserDO{}).
		Where(UserColumns.TenantID+" = ?", tenantID).
		Where(UserColumns.ID+" = ?", userID).
		Where(UserColumns.DeletedAt+" = ?", 0).
		Updates(map[string]any{
			UserColumns.AvatarURL: url,
			BaseColumns.UpdatedAt: r.now(),
			BaseColumns.Version:   gorm.Expr(BaseColumns.Version + " + 1"),
		}).Error
}

func (r *TenantUserRepository) BuildDisableImpact(ctx context.Context, tenantID uint64, userID uint64) (servicetenantuser.DisableImpact, error) {
	return servicetenantuser.DisableImpact{
		LoseLogin:       true,
		LoseExamAccess:  true,
		LoseGradeAccess: true,
	}, nil
}

func (r *TenantUserRepository) UpdateStatus(ctx context.Context, tenantID uint64, userID uint64, status string) error {
	return r.db.WithContext(ctx).Model(&UserDO{}).
		Where(UserColumns.TenantID+" = ?", tenantID).
		Where(UserColumns.ID+" = ?", userID).
		Where(UserColumns.DeletedAt+" = ?", 0).
		Updates(map[string]any{
			UserColumns.Status:    status,
			BaseColumns.UpdatedAt: r.now(),
			BaseColumns.Version:   gorm.Expr(BaseColumns.Version + " + 1"),
		}).Error
}

func tenantUserFromDO(row UserDO, role string) servicetenantuser.User {
	return servicetenantuser.User{
		ID:           row.ID,
		TenantID:     row.TenantID,
		Username:     row.Username,
		RealName:     row.RealName,
		AvatarURL:    row.AvatarURL,
		Phone:        row.Phone,
		Email:        row.Email,
		PasswordHash: row.PasswordHash,
		LastLoginIP:  row.LastLoginIP,
		LastLoginAt:  row.LastLoginAt,
		Role:         role,
		Status:       row.Status,
	}
}

func (r *TenantUserRepository) listRolesByUserIDs(ctx context.Context, tenantID uint64, userIDs []uint64) (map[uint64]string, error) {
	if len(userIDs) == 0 {
		return map[uint64]string{}, nil
	}
	var rows []UserRoleDO
	if err := r.db.WithContext(ctx).
		Where(UserRoleColumns.TenantID+" = ?", tenantID).
		Where(UserRoleColumns.UserID+" IN ?", userIDs).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	roles := make(map[uint64]string, len(rows))
	for _, row := range rows {
		roles[row.UserID] = row.Role
	}
	return roles, nil
}

func (r *TenantUserRepository) findRoleByUserID(ctx context.Context, tenantID uint64, userID uint64) (string, error) {
	var row UserRoleDO
	err := r.db.WithContext(ctx).
		Where(UserRoleColumns.TenantID+" = ?", tenantID).
		Where(UserRoleColumns.UserID+" = ?", userID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return row.Role, nil
}

func userIDsFromRows(rows []UserDO) []uint64 {
	ids := make([]uint64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return ids
}

func roleOrStudent(role string) string {
	if role == "" {
		return servicetenantuser.RoleStudent
	}
	return role
}

func generatedUserPhone(tenantID uint64, username string) string {
	return "pm-" + strconv.FormatUint(tenantID, 10) + "-" + username
}

func generatedUserEmail(tenantID uint64, username string) string {
	return username + "-" + strconv.FormatUint(tenantID, 10) + "@papermind.local"
}

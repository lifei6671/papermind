package db

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/lifei6671/papermind/server/internal/service/pagination"
	servicespace "github.com/lifei6671/papermind/server/internal/service/space"
	servicetenantuser "github.com/lifei6671/papermind/server/internal/service/tenantuser"
	"github.com/lifei6671/papermind/server/library/constant"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TenantUserRepository struct {
	db  *gorm.DB
	now func() int64
}

type TenantUserRepositoryOptions struct {
	Now func() int64
}

type tenantUserRow struct {
	ID                  uint64
	TenantID            uint64
	Username            string
	RealName            string
	AvatarURL           string
	Phone               string
	Email               string
	PasswordHash        string
	ForcePasswordChange bool
	LastLoginIP         string
	LastLoginAt         int64
	CreatedAt           int64
	UpdatedAt           int64
	Role                string
	MembershipStatus    string
	AccountStatus       string
}

var tenantUserRoleSearchLabels = map[string]string{
	servicetenantuser.RoleTenantAdmin: "租户管理员",
	servicetenantuser.RoleTeacher:     "教师",
	servicetenantuser.RoleStudent:     "学生",
}

var tenantUserStatusSearchLabels = map[string]string{
	servicetenantuser.StatusEnabled:  "启用",
	servicetenantuser.StatusDisabled: "禁用",
}

func NewTenantUserRepository(gormDB *gorm.DB, options TenantUserRepositoryOptions) *TenantUserRepository {
	now := options.Now
	if now == nil {
		now = func() int64 { return time.Now().UnixMilli() }
	}
	return &TenantUserRepository{db: gormDB, now: now}
}

func (r *TenantUserRepository) ListUsers(ctx context.Context, input servicetenantuser.ListInput) (pagination.Result[servicetenantuser.User], error) {
	page := pagination.Normalize(pagination.Input{Page: input.Page, PageSize: input.PageSize})
	query := r.tenantUserQuery(ctx).Where("tum."+UserRoleColumns.TenantID+" = ?", input.TenantID)
	query = r.applyTenantUserFilters(query, input.Role, input.Status)
	query = r.applyTenantUserSpaceExclusion(query, input.TenantID, input.ExcludeSpaceID)
	query = r.applyTenantUserSearch(query, input.Search)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return pagination.Result[servicetenantuser.User]{}, err
	}
	var rows []tenantUserRow
	if err := query.
		Select(r.tenantUserSelectColumns()).
		Order("tum." + BaseColumns.CreatedAt + " DESC").
		Order("tum." + UserRoleColumns.ID + " DESC").
		Limit(page.PageSize).
		Offset(pagination.Offset(page)).
		Scan(&rows).Error; err != nil {
		return pagination.Result[servicetenantuser.User]{}, err
	}
	users := make([]servicetenantuser.User, 0, len(rows))
	for _, row := range rows {
		users = append(users, tenantUserFromRow(row))
	}
	return pagination.Result[servicetenantuser.User]{
		Items:    users,
		Page:     page.Page,
		PageSize: page.PageSize,
		Total:    total,
	}, nil
}

func (r *TenantUserRepository) applyTenantUserSpaceExclusion(query *gorm.DB, tenantID uint64, excludeSpaceID uint64) *gorm.DB {
	if excludeSpaceID == 0 {
		return query
	}
	return query.Where(
		`NOT EXISTS (
				SELECT 1
				FROM space_members AS sm
				WHERE sm.tenant_id = ?
					and sm.space_id = ?
					and sm.user_id = users.id
					and sm.deleted_at = 0
			)`,
		tenantID,
		excludeSpaceID,
	)
}

func (r *TenantUserRepository) applyTenantUserFilters(query *gorm.DB, role string, status string) *gorm.DB {
	if normalizedRole := strings.TrimSpace(role); normalizedRole != "" {
		if !validTenantUserRoleFilter(normalizedRole) {
			return query.Where("1 = 0")
		}
		query = query.Where("tum."+UserRoleColumns.Role+" = ?", normalizedRole)
	}
	if normalizedStatus := strings.TrimSpace(status); normalizedStatus != "" {
		query = query.Where(r.tenantUserStatusCondition(normalizedStatus))
	}
	return query
}

func (r *TenantUserRepository) applyTenantUserSearch(query *gorm.DB, search string) *gorm.DB {
	keyword := strings.TrimSpace(search)
	if keyword == "" {
		return query
	}
	pattern := "%" + strings.ToLower(keyword) + "%"
	condition := r.db.Where("LOWER(users."+UserColumns.Username+") LIKE ?", pattern).
		Or("LOWER(users."+UserColumns.RealName+") LIKE ?", pattern).
		Or("LOWER(users."+UserColumns.Phone+") LIKE ?", pattern).
		Or("LOWER(users."+UserColumns.Email+") LIKE ?", pattern).
		Or("LOWER(tum."+UserRoleColumns.Role+") LIKE ?", pattern).
		Or("LOWER(tum."+UserRoleColumns.Status+") LIKE ?", pattern).
		Or("LOWER(users."+UserColumns.Status+") LIKE ?", pattern)
	for _, role := range localizedEnumMatches(keyword, tenantUserRoleSearchLabels) {
		condition = condition.Or("tum."+UserRoleColumns.Role+" = ?", role)
	}
	for _, status := range localizedEnumMatches(keyword, tenantUserStatusSearchLabels) {
		condition = condition.Or(r.tenantUserStatusCondition(status))
	}
	return query.Where(condition)
}

func (r *TenantUserRepository) tenantUserStatusCondition(status string) *gorm.DB {
	switch strings.TrimSpace(status) {
	case servicetenantuser.StatusEnabled:
		return r.db.Where("tum."+UserRoleColumns.Status+" = ?", servicetenantuser.StatusEnabled).
			Where("users."+UserColumns.Status+" = ?", servicetenantuser.StatusEnabled)
	case servicetenantuser.StatusDisabled:
		return r.db.Where("tum."+UserRoleColumns.Status+" = ?", servicetenantuser.StatusDisabled).
			Or("users."+UserColumns.Status+" = ?", servicetenantuser.StatusDisabled)
	default:
		return r.db.Where("1 = 0")
	}
}

func validTenantUserRoleFilter(role string) bool {
	switch role {
	case servicetenantuser.RoleTenantAdmin, servicetenantuser.RoleTeacher, servicetenantuser.RoleStudent:
		return true
	default:
		return false
	}
}

func (r *TenantUserRepository) FindUserByID(ctx context.Context, tenantID uint64, userID uint64) (servicetenantuser.User, error) {
	var row tenantUserRow
	err := r.tenantUserQuery(ctx).
		Select(r.tenantUserSelectColumns()).
		Where("tum."+UserRoleColumns.TenantID+" = ?", tenantID).
		Where("users."+UserColumns.ID+" = ?", userID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return servicetenantuser.User{}, servicetenantuser.ErrUserNotFound
	}
	if err != nil {
		return servicetenantuser.User{}, err
	}
	return tenantUserFromRow(row), nil
}

func (r *TenantUserRepository) FindGlobalUserByID(ctx context.Context, userID uint64) (servicetenantuser.User, error) {
	var row UserDO
	err := r.db.WithContext(ctx).Model(&UserDO{}).
		Where(UserColumns.ID+" = ?", userID).
		Where(UserColumns.DeletedAt+" = ?", 0).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return servicetenantuser.User{}, servicetenantuser.ErrUserNotFound
	}
	if err != nil {
		return servicetenantuser.User{}, err
	}
	return tenantUserFromGlobal(row, 0, servicetenantuser.RoleTenantUser, row.Status), nil
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
	return r.createUser(ctx, r.db, user)
}

func (r *TenantUserRepository) CreateUserRole(ctx context.Context, tenantID uint64, userID uint64, role string) error {
	return r.createUserRole(ctx, r.db, tenantID, userID, role)
}

func (r *TenantUserRepository) CreateUserWithRole(ctx context.Context, user servicetenantuser.User, role string) (servicetenantuser.User, error) {
	var created servicetenantuser.User
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		user.Role = role
		nextUser, err := r.createUser(ctx, tx, user)
		if err != nil {
			return err
		}
		if err := r.createUserRole(ctx, tx, user.TenantID, nextUser.ID, role); err != nil {
			return err
		}
		created, err = r.findUserByIDWithDB(ctx, tx, user.TenantID, nextUser.ID)
		return err
	})
	if err != nil {
		return servicetenantuser.User{}, err
	}
	return created, nil
}

func (r *TenantUserRepository) createUser(ctx context.Context, gormDB *gorm.DB, user servicetenantuser.User) (servicetenantuser.User, error) {
	existing, err := r.findGlobalUserByUsername(ctx, gormDB, user.Username, true)
	if err == nil {
		return tenantUserFromGlobal(existing, user.TenantID, user.Role, servicetenantuser.StatusEnabled), nil
	}
	if !errors.Is(err, servicetenantuser.ErrUserNotFound) {
		return servicetenantuser.User{}, err
	}
	now := r.now()
	row := UserDO{
		BaseFields: BaseFields{
			CreatedAt:     now,
			CreatedByType: AuditActorTenantUser,
			UpdatedAt:     now,
			UpdatedByType: AuditActorTenantUser,
			Version:       1,
			ExtJSON:       datatypes.JSON("{}"),
		},
		Username:            user.Username,
		RealName:            user.RealName,
		AvatarURL:           user.AvatarURL,
		Phone:               user.Phone,
		Email:               user.Email,
		PasswordHash:        user.PasswordHash,
		ForcePasswordChange: user.ForcePasswordChange,
		Status:              servicetenantuser.StatusEnabled,
	}
	if row.Phone == "" {
		row.Phone = generatedUserPhone(user.TenantID, user.Username)
	}
	if row.Email == "" {
		row.Email = generatedUserEmail(user.TenantID, user.Username)
	}
	if err := gormDB.WithContext(ctx).Create(&row).Error; err != nil {
		return servicetenantuser.User{}, err
	}
	return tenantUserFromGlobal(row, user.TenantID, user.Role, servicetenantuser.StatusEnabled), nil
}

func (r *TenantUserRepository) createUserRole(ctx context.Context, gormDB *gorm.DB, tenantID uint64, userID uint64, role string) error {
	now := r.now()
	row := UserRoleDO{
		BaseFields: BaseFields{
			CreatedAt:     now,
			CreatedByType: AuditActorTenantUser,
			UpdatedAt:     now,
			UpdatedByType: AuditActorTenantUser,
			Version:       1,
			ExtJSON:       datatypes.JSON("{}"),
		},
		TenantID: tenantID,
		UserID:   userID,
		Role:     role,
		Status:   servicetenantuser.StatusEnabled,
	}
	return gormDB.WithContext(ctx).Create(&row).Error
}

func (r *TenantUserRepository) FindUserByUsername(ctx context.Context, tenantID uint64, username string) (servicetenantuser.User, error) {
	var row tenantUserRow
	err := r.tenantUserQuery(ctx).
		Select(r.tenantUserSelectColumns()).
		Where("tum."+UserRoleColumns.TenantID+" = ?", tenantID).
		Where("users."+UserColumns.Username+" = ?", username).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return servicetenantuser.User{}, servicetenantuser.ErrUserNotFound
	}
	if err != nil {
		return servicetenantuser.User{}, err
	}
	return tenantUserFromRow(row), nil
}

func (r *TenantUserRepository) FindGlobalUserByUsername(ctx context.Context, username string) (servicetenantuser.User, error) {
	row, err := r.findGlobalUserByUsername(ctx, r.db, username, false)
	if err != nil {
		return servicetenantuser.User{}, err
	}
	return tenantUserFromGlobal(row, 0, "", row.Status), nil
}

func (r *TenantUserRepository) UpdateLoginAudit(ctx context.Context, tenantID uint64, userID uint64, ip string, at int64) error {
	return r.db.WithContext(ctx).Model(&UserDO{}).
		Where(UserColumns.ID+" = ?", userID).
		Where(UserColumns.DeletedAt+" = ?", 0).
		Updates(map[string]any{
			UserColumns.LastLoginIP:   ip,
			UserColumns.LastLoginAt:   at,
			BaseColumns.UpdatedAt:     r.now(),
			BaseColumns.UpdatedByType: AuditActorTenantUser,
			BaseColumns.Version:       gorm.Expr(BaseColumns.Version + " + 1"),
		}).Error
}

func (r *TenantUserRepository) UpdateProfile(ctx context.Context, input servicetenantuser.UpdateProfileInput) (servicetenantuser.User, error) {
	query := r.db.WithContext(ctx).Model(&UserDO{}).
		Where(UserColumns.ID+" = ?", input.UserID).
		Where(UserColumns.DeletedAt+" = ?", 0)
	if input.TenantID != 0 {
		query = query.Where(`
			EXISTS (
				SELECT 1
				FROM tenant_user_memberships AS tum
				WHERE tum.tenant_id = ?
					and tum.user_id = users.id
			)
		`, input.TenantID)
	}
	result := query.Updates(map[string]any{
		UserColumns.RealName:      input.DisplayName,
		UserColumns.AvatarURL:     input.AvatarURL,
		UserColumns.Phone:         input.Phone,
		UserColumns.Email:         input.Email,
		BaseColumns.UpdatedAt:     r.now(),
		BaseColumns.UpdatedByType: AuditActorTenantUser,
		BaseColumns.Version:       gorm.Expr(BaseColumns.Version + " + 1"),
	})
	if result.Error != nil {
		return servicetenantuser.User{}, result.Error
	}
	if input.TenantID == 0 {
		return r.FindGlobalUserByID(ctx, input.UserID)
	}
	return r.FindUserByID(ctx, input.TenantID, input.UserID)
}

func (r *TenantUserRepository) UpdatePassword(ctx context.Context, userID uint64, passwordHash string, forcePasswordChange bool) (servicetenantuser.User, error) {
	result := r.db.WithContext(ctx).Model(&UserDO{}).
		Where(UserColumns.ID+" = ?", userID).
		Where(UserColumns.DeletedAt+" = ?", 0).
		Updates(map[string]any{
			UserColumns.PasswordHash:        passwordHash,
			UserColumns.ForcePasswordChange: forcePasswordChange,
			BaseColumns.UpdatedAt:           r.now(),
			BaseColumns.UpdatedByType:       AuditActorTenantUser,
			BaseColumns.Version:             gorm.Expr(BaseColumns.Version + " + 1"),
		})
	if result.Error != nil {
		return servicetenantuser.User{}, result.Error
	}
	if result.RowsAffected == 0 {
		return servicetenantuser.User{}, servicetenantuser.ErrUserNotFound
	}
	return r.FindGlobalUserByID(ctx, userID)
}

func (r *TenantUserRepository) UpdateAvatarURL(ctx context.Context, tenantID uint64, userID uint64, url string) error {
	return r.db.WithContext(ctx).Model(&UserDO{}).
		Where(UserColumns.ID+" = ?", userID).
		Where(UserColumns.DeletedAt+" = ?", 0).
		Updates(map[string]any{
			UserColumns.AvatarURL:     url,
			BaseColumns.UpdatedAt:     r.now(),
			BaseColumns.UpdatedByType: AuditActorTenantUser,
			BaseColumns.Version:       gorm.Expr(BaseColumns.Version + " + 1"),
		}).Error
}

func (r *TenantUserRepository) BuildDisableImpact(ctx context.Context, tenantID uint64, userID uint64) (servicetenantuser.DisableImpact, error) {
	var adminSpaceIDs []uint64
	if err := r.db.WithContext(ctx).Model(&SpaceMemberDO{}).
		Where(SpaceMemberColumns.TenantID+" = ?", tenantID).
		Where(SpaceMemberColumns.UserID+" = ?", userID).
		Where(SpaceMemberColumns.RoleInSpace+" = ?", servicespace.RoleSpaceAdmin).
		Where(SpaceMemberColumns.Status+" = ?", servicespace.StatusEnabled).
		Where(SpaceMemberColumns.DeletedAt+" = ?", 0).
		Pluck(SpaceMemberColumns.SpaceID, &adminSpaceIDs).Error; err != nil {
		return servicetenantuser.DisableImpact{}, err
	}
	return servicetenantuser.DisableImpact{
		LoseLogin:       true,
		LoseExamAccess:  true,
		LoseGradeAccess: true,
		AdminSpaceIDs:   adminSpaceIDs,
	}, nil
}

func (r *TenantUserRepository) UpdateStatus(ctx context.Context, tenantID uint64, userID uint64, status string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if status == servicetenantuser.StatusDisabled {
			if err := r.lockTenantAdminRows(ctx, tx, tenantID); err != nil {
				return err
			}
			if err := r.lockSpaceAdminRowsForUserDisable(ctx, tx, tenantID, userID); err != nil {
				return err
			}
		}
		result := tx.Model(&UserRoleDO{}).
			Where(UserRoleColumns.TenantID+" = ?", tenantID).
			Where(UserRoleColumns.UserID+" = ?", userID).
			Updates(map[string]any{
				UserRoleColumns.Status:    status,
				BaseColumns.UpdatedAt:     r.now(),
				BaseColumns.UpdatedByType: AuditActorTenantUser,
				BaseColumns.Version:       gorm.Expr(BaseColumns.Version + " + 1"),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return servicetenantuser.ErrUserNotFound
		}
		if status == servicetenantuser.StatusDisabled {
			if err := r.validateTenantAdminInvariant(ctx, tx, tenantID); err != nil {
				return err
			}
			if err := r.validateSpaceAdminInvariantAfterUserStatusChange(ctx, tx, tenantID, userID); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *TenantUserRepository) DeleteUser(ctx context.Context, tenantID uint64, userID uint64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := r.lockTenantAdminRows(ctx, tx, tenantID); err != nil {
			return err
		}
		if err := r.lockSpaceAdminRowsForUserDisable(ctx, tx, tenantID, userID); err != nil {
			return err
		}
		result := tx.Where(UserRoleColumns.TenantID+" = ?", tenantID).
			Where(UserRoleColumns.UserID+" = ?", userID).
			Delete(&UserRoleDO{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return servicetenantuser.ErrUserNotFound
		}
		// 租户用户删除只移除当前租户关系，全局账号保留；空间管理员不变式按移除后的租户关系校验。
		if err := r.validateTenantAdminInvariant(ctx, tx, tenantID); err != nil {
			return err
		}
		return r.validateSpaceAdminInvariantAfterUserStatusChange(ctx, tx, tenantID, userID)
	})
}

func (r *TenantUserRepository) UpdateRole(ctx context.Context, tenantID uint64, userID uint64, role string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := r.lockTenantAdminRows(ctx, tx, tenantID); err != nil {
			return err
		}
		if err := r.lockUserRow(ctx, tx, tenantID, userID); err != nil {
			return err
		}
		result := tx.Model(&UserRoleDO{}).
			Where(UserRoleColumns.TenantID+" = ?", tenantID).
			Where(UserRoleColumns.UserID+" = ?", userID).
			Updates(map[string]any{
				UserRoleColumns.Role:      role,
				BaseColumns.UpdatedAt:     r.now(),
				BaseColumns.UpdatedByType: AuditActorTenantUser,
				BaseColumns.Version:       gorm.Expr(BaseColumns.Version + " + 1"),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return servicetenantuser.ErrUserNotFound
		}
		// 租户级角色变更会让 tenant_admin 身份立即生效或失效，必须按变更后状态校验。
		return r.validateTenantAdminInvariant(ctx, tx, tenantID)
	})
}

func (r *TenantUserRepository) ImportUsers(ctx context.Context, input servicetenantuser.ImportUsersRepositoryInput) (servicetenantuser.ImportUsersResult, error) {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := r.lockTenantAdminRows(ctx, tx, input.TenantID); err != nil {
			return err
		}
		for _, row := range input.Rows {
			user, err := r.findUserByUsernameForUpdate(ctx, tx, row.Username)
			if errors.Is(err, servicetenantuser.ErrUserNotFound) {
				if err := r.createImportedUser(ctx, tx, input.TenantID, row); err != nil {
					return err
				}
				continue
			}
			if err != nil {
				return err
			}
			currentRole, err := r.findRoleByUserIDWithDB(ctx, tx, input.TenantID, user.ID)
			if err != nil {
				return err
			}
			if user.ID == input.ActorID && currentRole != row.Role {
				return servicetenantuser.ErrCannotChangeSelfRole
			}
			if currentRole == "" {
				if err := r.createUserRole(ctx, tx, input.TenantID, user.ID, row.Role); err != nil {
					return err
				}
			}
			if err := r.updateImportedUser(ctx, tx, user.ID, input.TenantID, row); err != nil {
				return err
			}
		}
		// 批量导入允许一次提交内完成租户管理员交接，最终状态仍必须保留至少一个启用管理员。
		return r.validateTenantAdminInvariant(ctx, tx, input.TenantID)
	})
	if err != nil {
		return servicetenantuser.ImportUsersResult{}, err
	}
	return servicetenantuser.ImportUsersResult{SuccessCount: len(input.Rows)}, nil
}

func (r *TenantUserRepository) createImportedUser(ctx context.Context, tx *gorm.DB, tenantID uint64, row servicetenantuser.ImportUsersRepositoryRow) error {
	user, err := r.createUser(ctx, tx, servicetenantuser.User{
		TenantID:     tenantID,
		Username:     row.Username,
		RealName:     row.RealName,
		AvatarURL:    row.AvatarURL,
		PasswordHash: row.PasswordHash,
		Role:         row.Role,
		Status:       servicetenantuser.StatusEnabled,
	})
	if err != nil {
		return err
	}
	return r.createUserRole(ctx, tx, tenantID, user.ID, row.Role)
}

func (r *TenantUserRepository) updateImportedUser(ctx context.Context, tx *gorm.DB, userID uint64, tenantID uint64, row servicetenantuser.ImportUsersRepositoryRow) error {
	now := r.now()
	userResult := tx.WithContext(ctx).Model(&UserDO{}).
		Where(UserColumns.ID+" = ?", userID).
		Where(UserColumns.DeletedAt+" = ?", 0).
		Updates(map[string]any{
			UserColumns.RealName:      row.RealName,
			UserColumns.AvatarURL:     row.AvatarURL,
			UserColumns.PasswordHash:  row.PasswordHash,
			BaseColumns.UpdatedAt:     now,
			BaseColumns.UpdatedByType: AuditActorTenantUser,
			BaseColumns.Version:       gorm.Expr(BaseColumns.Version + " + 1"),
		})
	if userResult.Error != nil {
		return userResult.Error
	}
	if userResult.RowsAffected == 0 {
		return servicetenantuser.ErrUserNotFound
	}
	roleResult := tx.WithContext(ctx).Model(&UserRoleDO{}).
		Where(UserRoleColumns.TenantID+" = ?", tenantID).
		Where(UserRoleColumns.UserID+" = ?", userID).
		Updates(map[string]any{
			UserRoleColumns.Role:      row.Role,
			UserRoleColumns.Status:    servicetenantuser.StatusEnabled,
			BaseColumns.UpdatedAt:     now,
			BaseColumns.UpdatedByType: AuditActorTenantUser,
			BaseColumns.Version:       gorm.Expr(BaseColumns.Version + " + 1"),
		})
	if roleResult.Error != nil {
		return roleResult.Error
	}
	if roleResult.RowsAffected == 0 {
		return servicetenantuser.ErrUserNotFound
	}
	return nil
}

func (r *TenantUserRepository) findUserByUsernameForUpdate(ctx context.Context, tx *gorm.DB, username string) (servicetenantuser.User, error) {
	row, err := r.findGlobalUserByUsername(ctx, tx.Clauses(clause.Locking{Strength: "UPDATE"}), username, false)
	if err != nil {
		return servicetenantuser.User{}, err
	}
	return tenantUserFromGlobal(row, 0, "", row.Status), nil
}

func (r *TenantUserRepository) lockUserRow(ctx context.Context, tx *gorm.DB, tenantID uint64, userID uint64) error {
	var row UserRoleDO
	err := tx.WithContext(ctx).Model(&UserRoleDO{}).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where(UserRoleColumns.TenantID+" = ?", tenantID).
		Where(UserRoleColumns.UserID+" = ?", userID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return servicetenantuser.ErrUserNotFound
	}
	return err
}

func (r *TenantUserRepository) lockTenantAdminRows(ctx context.Context, tx *gorm.DB, tenantID uint64) error {
	var rows []UserRoleDO
	return r.tenantAdminLockQuery(ctx, tx, tenantID).Find(&rows).Error
}

func (r *TenantUserRepository) tenantAdminLockQuery(ctx context.Context, gormDB *gorm.DB, tenantID uint64) *gorm.DB {
	return gormDB.WithContext(ctx).Model(&UserRoleDO{}).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where(UserRoleColumns.TenantID+" = ?", tenantID).
		Where(UserRoleColumns.Role+" = ?", constant.RoleTenantAdmin).
		Where(UserRoleColumns.Status+" = ?", servicetenantuser.StatusEnabled)
}

func (r *TenantUserRepository) lockSpaceAdminRowsForUserDisable(ctx context.Context, tx *gorm.DB, tenantID uint64, userID uint64) error {
	var spaceIDs []uint64
	enabledSpaceIDs := tx.WithContext(ctx).Model(&SpaceDO{}).
		Select(SpaceColumns.ID).
		Where(SpaceColumns.TenantID+" = ?", tenantID).
		Where(SpaceColumns.Status+" = ?", servicespace.StatusEnabled).
		Where(SpaceColumns.DeletedAt+" = ?", 0)
	if err := tx.WithContext(ctx).Model(&SpaceMemberDO{}).
		Where(SpaceMemberColumns.TenantID+" = ?", tenantID).
		Where(SpaceMemberColumns.UserID+" = ?", userID).
		Where(SpaceMemberColumns.RoleInSpace+" = ?", servicespace.RoleSpaceAdmin).
		Where(SpaceMemberColumns.Status+" = ?", servicespace.StatusEnabled).
		Where(SpaceMemberColumns.DeletedAt+" = ?", 0).
		Where(SpaceMemberColumns.SpaceID+" IN (?)", enabledSpaceIDs).
		Pluck(SpaceMemberColumns.SpaceID, &spaceIDs).Error; err != nil {
		return err
	}
	if len(spaceIDs) == 0 {
		return nil
	}
	var rows []SpaceMemberDO
	return r.spaceAdminLockQuery(ctx, tx, tenantID, spaceIDs).Find(&rows).Error
}

func (r *TenantUserRepository) spaceAdminLockQuery(ctx context.Context, gormDB *gorm.DB, tenantID uint64, spaceIDs []uint64) *gorm.DB {
	enabledSpaceIDs := gormDB.WithContext(ctx).Model(&SpaceDO{}).
		Select(SpaceColumns.ID).
		Where(SpaceColumns.TenantID+" = ?", tenantID).
		Where(SpaceColumns.Status+" = ?", servicespace.StatusEnabled).
		Where(SpaceColumns.DeletedAt+" = ?", 0)
	return gormDB.WithContext(ctx).Model(&SpaceMemberDO{}).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where(SpaceMemberColumns.TenantID+" = ?", tenantID).
		Where(SpaceMemberColumns.SpaceID+" IN ?", spaceIDs).
		Where(SpaceMemberColumns.SpaceID+" IN (?)", enabledSpaceIDs).
		Where(SpaceMemberColumns.RoleInSpace+" = ?", servicespace.RoleSpaceAdmin).
		Where(SpaceMemberColumns.Status+" = ?", servicespace.StatusEnabled).
		Where(SpaceMemberColumns.DeletedAt+" = ?", 0)
}

func (r *TenantUserRepository) validateTenantAdminInvariant(ctx context.Context, tx *gorm.DB, tenantID uint64) error {
	var count int64
	err := tx.WithContext(ctx).Table("tenant_user_memberships AS tum").
		Joins("JOIN users ON users.id = tum.user_id AND users.status = ? AND users.deleted_at = 0", servicetenantuser.StatusEnabled).
		Where("tum."+UserRoleColumns.TenantID+" = ?", tenantID).
		Where("tum."+UserRoleColumns.Role+" = ?", constant.RoleTenantAdmin).
		Where("tum."+UserRoleColumns.Status+" = ?", servicetenantuser.StatusEnabled).
		Count(&count).Error
	if err != nil {
		return err
	}
	if count == 0 {
		return servicetenantuser.ErrCannotLoseLastTenantAdmin
	}
	return nil
}

func (r *TenantUserRepository) validateSpaceAdminInvariantAfterUserStatusChange(ctx context.Context, tx *gorm.DB, tenantID uint64, userID uint64) error {
	var memberRows []SpaceMemberDO
	enabledSpaceIDs := tx.WithContext(ctx).Model(&SpaceDO{}).
		Select(SpaceColumns.ID).
		Where(SpaceColumns.TenantID+" = ?", tenantID).
		Where(SpaceColumns.Status+" = ?", servicespace.StatusEnabled).
		Where(SpaceColumns.DeletedAt+" = ?", 0)
	if err := tx.WithContext(ctx).Model(&SpaceMemberDO{}).
		Where(SpaceMemberColumns.TenantID+" = ?", tenantID).
		Where(SpaceMemberColumns.UserID+" = ?", userID).
		Where(SpaceMemberColumns.RoleInSpace+" = ?", servicespace.RoleSpaceAdmin).
		Where(SpaceMemberColumns.Status+" = ?", servicespace.StatusEnabled).
		Where(SpaceMemberColumns.DeletedAt+" = ?", 0).
		Where(SpaceMemberColumns.SpaceID+" IN (?)", enabledSpaceIDs).
		Find(&memberRows).Error; err != nil {
		return err
	}
	enabledUserIDs := tx.WithContext(ctx).Table("tenant_user_memberships AS tum").
		Select("tum."+UserRoleColumns.UserID).
		Joins("JOIN users ON users.id = tum.user_id AND users.status = ? AND users.deleted_at = 0", servicetenantuser.StatusEnabled).
		Where("tum."+UserRoleColumns.TenantID+" = ?", tenantID).
		Where("tum."+UserRoleColumns.Status+" = ?", servicetenantuser.StatusEnabled)
	for _, member := range memberRows {
		var count int64
		err := tx.WithContext(ctx).Model(&SpaceMemberDO{}).
			Where(SpaceMemberColumns.TenantID+" = ?", tenantID).
			Where(SpaceMemberColumns.SpaceID+" = ?", member.SpaceID).
			Where(SpaceMemberColumns.RoleInSpace+" = ?", servicespace.RoleSpaceAdmin).
			Where(SpaceMemberColumns.Status+" = ?", servicespace.StatusEnabled).
			Where(SpaceMemberColumns.UserID+" IN (?)", enabledUserIDs).
			Where(SpaceMemberColumns.DeletedAt+" = ?", 0).
			Count(&count).Error
		if err != nil {
			return err
		}
		if count == 0 {
			return servicespace.ErrCannotLoseLastSpaceAdmin
		}
	}
	return nil
}

func (r *TenantUserRepository) tenantUserQuery(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).Table("tenant_user_memberships AS tum").
		Joins("JOIN users ON users.id = tum.user_id AND users.deleted_at = 0")
}

func (r *TenantUserRepository) tenantUserSelectColumns() string {
	return "users.id, tum.tenant_id, users.username, users.real_name, users.avatar_url, users.phone, users.email, " +
		"users.password_hash, users.force_password_change, users.last_login_ip, users.last_login_at, users.created_at, users.updated_at, " +
		"tum.role, tum.status AS membership_status, users.status AS account_status"
}

func (r *TenantUserRepository) findUserByIDWithDB(ctx context.Context, gormDB *gorm.DB, tenantID uint64, userID uint64) (servicetenantuser.User, error) {
	var row tenantUserRow
	err := gormDB.WithContext(ctx).Table("tenant_user_memberships AS tum").
		Joins("JOIN users ON users.id = tum.user_id AND users.deleted_at = 0").
		Select(r.tenantUserSelectColumns()).
		Where("tum."+UserRoleColumns.TenantID+" = ?", tenantID).
		Where("users."+UserColumns.ID+" = ?", userID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return servicetenantuser.User{}, servicetenantuser.ErrUserNotFound
	}
	if err != nil {
		return servicetenantuser.User{}, err
	}
	return tenantUserFromRow(row), nil
}

func (r *TenantUserRepository) findGlobalUserByUsername(ctx context.Context, gormDB *gorm.DB, username string, forUpdate bool) (UserDO, error) {
	var row UserDO
	query := gormDB.WithContext(ctx).Model(&UserDO{}).
		Where(UserColumns.Username+" = ?", username).
		Where(UserColumns.DeletedAt+" = ?", 0)
	if forUpdate {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	err := query.First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return UserDO{}, servicetenantuser.ErrUserNotFound
	}
	if err != nil {
		return UserDO{}, err
	}
	return row, nil
}

func tenantUserFromRow(row tenantUserRow) servicetenantuser.User {
	return servicetenantuser.User{
		ID:                  row.ID,
		TenantID:            row.TenantID,
		Username:            row.Username,
		RealName:            row.RealName,
		AvatarURL:           row.AvatarURL,
		Phone:               row.Phone,
		Email:               row.Email,
		PasswordHash:        row.PasswordHash,
		ForcePasswordChange: row.ForcePasswordChange,
		LastLoginIP:         row.LastLoginIP,
		LastLoginAt:         row.LastLoginAt,
		CreatedAt:           row.CreatedAt,
		UpdatedAt:           row.UpdatedAt,
		Role:                roleOrStudent(row.Role),
		Status:              tenantMembershipStatus(row.MembershipStatus, row.AccountStatus),
	}
}

func tenantUserFromGlobal(row UserDO, tenantID uint64, role string, status string) servicetenantuser.User {
	return servicetenantuser.User{
		ID:                  row.ID,
		TenantID:            tenantID,
		Username:            row.Username,
		RealName:            row.RealName,
		AvatarURL:           row.AvatarURL,
		Phone:               row.Phone,
		Email:               row.Email,
		PasswordHash:        row.PasswordHash,
		ForcePasswordChange: row.ForcePasswordChange,
		LastLoginIP:         row.LastLoginIP,
		LastLoginAt:         row.LastLoginAt,
		CreatedAt:           row.CreatedAt,
		UpdatedAt:           row.UpdatedAt,
		Role:                role,
		Status:              status,
	}
}

func (r *TenantUserRepository) findRoleByUserID(ctx context.Context, tenantID uint64, userID uint64) (string, error) {
	return r.findRoleByUserIDWithDB(ctx, r.db, tenantID, userID)
}

func (r *TenantUserRepository) findRoleByUserIDWithDB(ctx context.Context, gormDB *gorm.DB, tenantID uint64, userID uint64) (string, error) {
	var row UserRoleDO
	err := gormDB.WithContext(ctx).
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

func roleOrStudent(role string) string {
	if role == "" {
		return servicetenantuser.RoleStudent
	}
	return role
}

func tenantMembershipStatus(membershipStatus string, accountStatus string) string {
	if accountStatus != servicetenantuser.StatusEnabled {
		return servicetenantuser.StatusDisabled
	}
	if membershipStatus == "" {
		return servicetenantuser.StatusEnabled
	}
	return membershipStatus
}

func generatedUserPhone(tenantID uint64, username string) string {
	return "pm-" + strconv.FormatUint(tenantID, 10) + "-" + username
}

func generatedUserEmail(tenantID uint64, username string) string {
	return username + "-" + strconv.FormatUint(tenantID, 10) + "@papermind.local"
}

package db

import (
	"context"
	"errors"
	"strconv"
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
		if err := r.createUserRole(ctx, tx, nextUser.TenantID, nextUser.ID, role); err != nil {
			return err
		}
		created = nextUser
		created.Role = role
		return nil
	})
	if err != nil {
		return servicetenantuser.User{}, err
	}
	return created, nil
}

func (r *TenantUserRepository) createUser(ctx context.Context, gormDB *gorm.DB, user servicetenantuser.User) (servicetenantuser.User, error) {
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
	if err := gormDB.WithContext(ctx).Create(&row).Error; err != nil {
		return servicetenantuser.User{}, err
	}
	return tenantUserFromDO(row, user.Role), nil
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
	}
	return gormDB.WithContext(ctx).Create(&row).Error
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
	role, err := r.findRoleByUserID(ctx, tenantID, row.ID)
	if err != nil {
		return servicetenantuser.User{}, err
	}
	return tenantUserFromDO(row, roleOrStudent(role)), nil
}

func (r *TenantUserRepository) UpdateLoginAudit(ctx context.Context, tenantID uint64, userID uint64, ip string, at int64) error {
	return r.db.WithContext(ctx).Model(&UserDO{}).
		Where(UserColumns.TenantID+" = ?", tenantID).
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
	err := r.db.WithContext(ctx).Model(&UserDO{}).
		Where(UserColumns.TenantID+" = ?", input.TenantID).
		Where(UserColumns.ID+" = ?", input.UserID).
		Where(UserColumns.DeletedAt+" = ?", 0).
		Updates(map[string]any{
			UserColumns.RealName:      input.DisplayName,
			UserColumns.AvatarURL:     input.AvatarURL,
			UserColumns.Phone:         input.Phone,
			UserColumns.Email:         input.Email,
			BaseColumns.UpdatedAt:     r.now(),
			BaseColumns.UpdatedByType: AuditActorTenantUser,
			BaseColumns.Version:       gorm.Expr(BaseColumns.Version + " + 1"),
		}).Error
	if err != nil {
		return servicetenantuser.User{}, err
	}
	return r.FindUserByID(ctx, input.TenantID, input.UserID)
}

func (r *TenantUserRepository) UpdateAvatarURL(ctx context.Context, tenantID uint64, userID uint64, url string) error {
	return r.db.WithContext(ctx).Model(&UserDO{}).
		Where(UserColumns.TenantID+" = ?", tenantID).
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
		result := tx.Model(&UserDO{}).
			Where(UserColumns.TenantID+" = ?", tenantID).
			Where(UserColumns.ID+" = ?", userID).
			Where(UserColumns.DeletedAt+" = ?", 0).
			Updates(map[string]any{
				UserColumns.Status:        status,
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
		now := r.now()
		result := tx.Model(&UserDO{}).
			Where(UserColumns.TenantID+" = ?", tenantID).
			Where(UserColumns.ID+" = ?", userID).
			Where(UserColumns.DeletedAt+" = ?", 0).
			Updates(map[string]any{
				UserColumns.DeletedAt:     now,
				BaseColumns.UpdatedAt:     now,
				BaseColumns.UpdatedByType: AuditActorTenantUser,
				BaseColumns.Version:       gorm.Expr(BaseColumns.Version + " + 1"),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return servicetenantuser.ErrUserNotFound
		}
		// 用户删除会让租户角色和空间管理员身份同时失效，两个不变式必须基于删除后的状态校验。
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
			user, err := r.findUserByUsernameForUpdate(ctx, tx, input.TenantID, row.Username)
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
		Where(UserColumns.TenantID+" = ?", tenantID).
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

func (r *TenantUserRepository) findUserByUsernameForUpdate(ctx context.Context, tx *gorm.DB, tenantID uint64, username string) (servicetenantuser.User, error) {
	var row UserDO
	err := tx.WithContext(ctx).Model(&UserDO{}).
		Clauses(clause.Locking{Strength: "UPDATE"}).
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

func (r *TenantUserRepository) lockUserRow(ctx context.Context, tx *gorm.DB, tenantID uint64, userID uint64) error {
	var row UserDO
	err := tx.WithContext(ctx).Model(&UserDO{}).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where(UserColumns.TenantID+" = ?", tenantID).
		Where(UserColumns.ID+" = ?", userID).
		Where(UserColumns.DeletedAt+" = ?", 0).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return servicetenantuser.ErrUserNotFound
	}
	return err
}

func (r *TenantUserRepository) lockTenantAdminRows(ctx context.Context, tx *gorm.DB, tenantID uint64) error {
	var rows []UserDO
	return r.tenantAdminLockQuery(ctx, tx, tenantID).Find(&rows).Error
}

func (r *TenantUserRepository) tenantAdminLockQuery(ctx context.Context, gormDB *gorm.DB, tenantID uint64) *gorm.DB {
	adminUserIDs := gormDB.WithContext(ctx).Model(&UserRoleDO{}).
		Select(UserRoleColumns.UserID).
		Where(UserRoleColumns.TenantID+" = ?", tenantID).
		Where(UserRoleColumns.Role+" = ?", constant.RoleTenantAdmin)
	return gormDB.WithContext(ctx).Model(&UserDO{}).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where(UserColumns.TenantID+" = ?", tenantID).
		Where(UserColumns.ID+" IN (?)", adminUserIDs).
		Where(UserColumns.Status+" = ?", servicetenantuser.StatusEnabled).
		Where(UserColumns.DeletedAt+" = ?", 0)
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
	adminUserIDs := tx.WithContext(ctx).Model(&UserRoleDO{}).
		Select(UserRoleColumns.UserID).
		Where(UserRoleColumns.TenantID+" = ?", tenantID).
		Where(UserRoleColumns.Role+" = ?", constant.RoleTenantAdmin)
	err := tx.WithContext(ctx).Model(&UserDO{}).
		Where(UserColumns.TenantID+" = ?", tenantID).
		Where(UserColumns.ID+" IN (?)", adminUserIDs).
		Where(UserColumns.Status+" = ?", servicetenantuser.StatusEnabled).
		Where(UserColumns.DeletedAt+" = ?", 0).
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
	enabledUserIDs := tx.WithContext(ctx).Model(&UserDO{}).
		Select(UserColumns.ID).
		Where(UserColumns.TenantID+" = ?", tenantID).
		Where(UserColumns.Status+" = ?", servicetenantuser.StatusEnabled).
		Where(UserColumns.DeletedAt+" = ?", 0)
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

package db

import (
	"context"
	"errors"
	"time"

	"github.com/lifei6671/papermind/server/internal/service/pagination"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/plugin/soft_delete"

	servicespace "github.com/lifei6671/papermind/server/internal/service/space"
	servicetenantuser "github.com/lifei6671/papermind/server/internal/service/tenantuser"
)

type SpaceRepository struct {
	db  *gorm.DB
	now func() int64
}

type SpaceRepositoryOptions struct {
	Now func() int64
}

func NewSpaceRepository(gormDB *gorm.DB, options SpaceRepositoryOptions) *SpaceRepository {
	now := options.Now
	if now == nil {
		now = func() int64 { return time.Now().UnixMilli() }
	}
	return &SpaceRepository{db: gormDB, now: now}
}

func (r *SpaceRepository) ListSpaces(ctx context.Context, tenantID uint64, page pagination.Input) (pagination.Result[servicespace.Space], error) {
	page = pagination.Normalize(page)
	query := r.db.WithContext(ctx).Model(&SpaceDO{}).
		Where(SpaceColumns.TenantID+" = ?", tenantID).
		Where(SpaceColumns.DeletedAt+" = ?", 0)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return pagination.Result[servicespace.Space]{}, err
	}
	var rows []SpaceDO
	if err := query.
		Order(SpaceColumns.ID + " ASC").
		Limit(page.PageSize).
		Offset(pagination.Offset(page)).
		Find(&rows).Error; err != nil {
		return pagination.Result[servicespace.Space]{}, err
	}
	spaces := make([]servicespace.Space, 0, len(rows))
	for _, row := range rows {
		spaces = append(spaces, spaceFromDO(row))
	}
	return pagination.Result[servicespace.Space]{
		Items:    spaces,
		Page:     page.Page,
		PageSize: page.PageSize,
		Total:    total,
	}, nil
}

func (r *SpaceRepository) CreateSpace(ctx context.Context, space servicespace.Space, adminUserIDs []uint64) (servicespace.Space, error) {
	var created servicespace.Space
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := r.now()
		row := SpaceDO{
			BaseFields: BaseFields{
				CreatedAt:     now,
				CreatedByType: AuditActorTenantUser,
				UpdatedAt:     now,
				UpdatedByType: AuditActorTenantUser,
				Version:       1,
				ExtJSON:       datatypes.JSON("{}"),
			},
			TenantID:    space.TenantID,
			Name:        space.Name,
			LogoURL:     space.LogoURL,
			Description: space.Description,
			Type:        space.Type,
			Status:      space.Status,
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		created = spaceFromDO(row)
		for _, userID := range adminUserIDs {
			if err := r.validateEnabledTenantUser(ctx, tx, space.TenantID, userID); err != nil {
				return err
			}
			member := SpaceMemberDO{
				BaseFields: BaseFields{
					CreatedAt:     now,
					CreatedByType: AuditActorTenantUser,
					UpdatedAt:     now,
					UpdatedByType: AuditActorTenantUser,
					Version:       1,
					ExtJSON:       datatypes.JSON("{}"),
				},
				TenantID:    space.TenantID,
				SpaceID:     row.ID,
				UserID:      userID,
				RoleInSpace: servicespace.RoleSpaceAdmin,
				Status:      servicespace.StatusEnabled,
			}
			if err := tx.Create(&member).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return servicespace.Space{}, err
	}
	return created, nil
}

func (r *SpaceRepository) UpdateSpaceProfile(ctx context.Context, input servicespace.UpdateProfileInput) (servicespace.Space, error) {
	updates := map[string]any{
		SpaceColumns.Name:         input.Name,
		SpaceColumns.LogoURL:      input.LogoURL,
		SpaceColumns.Description:  input.Description,
		SpaceColumns.Type:         input.Type,
		BaseColumns.UpdatedAt:     r.now(),
		BaseColumns.UpdatedByType: AuditActorTenantUser,
		BaseColumns.Version:       gorm.Expr(BaseColumns.Version + " + 1"),
	}
	// 空间资料由租户管理员维护，更新后立即返回最新快照供管理端刷新列表。
	result := r.db.WithContext(ctx).Model(&SpaceDO{}).
		Where(SpaceColumns.TenantID+" = ?", input.TenantID).
		Where(SpaceColumns.ID+" = ?", input.SpaceID).
		Where(SpaceColumns.DeletedAt+" = ?", 0).
		Updates(updates)
	if result.Error != nil {
		return servicespace.Space{}, result.Error
	}
	if result.RowsAffected == 0 {
		return servicespace.Space{}, servicespace.ErrSpaceNotFound
	}
	return r.findSpace(ctx, input.TenantID, input.SpaceID)
}

func (r *SpaceRepository) DeleteSpace(ctx context.Context, tenantID uint64, spaceID uint64) error {
	// 删除空间只软删除空间资料；成员记录保留审计轨迹，但已删除空间不会再被有效授权查询命中。
	result := r.db.WithContext(ctx).Model(&SpaceDO{}).
		Where(SpaceColumns.TenantID+" = ?", tenantID).
		Where(SpaceColumns.ID+" = ?", spaceID).
		Where(SpaceColumns.DeletedAt+" = ?", 0).
		Updates(map[string]any{
			SpaceColumns.DeletedAt:    soft_delete.DeletedAt(r.now()),
			BaseColumns.UpdatedAt:     r.now(),
			BaseColumns.UpdatedByType: AuditActorTenantUser,
			BaseColumns.Version:       gorm.Expr(BaseColumns.Version + " + 1"),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return servicespace.ErrSpaceNotFound
	}
	return nil
}

func (r *SpaceRepository) findSpace(ctx context.Context, tenantID uint64, spaceID uint64) (servicespace.Space, error) {
	var row SpaceDO
	err := r.db.WithContext(ctx).
		Where(SpaceColumns.TenantID+" = ?", tenantID).
		Where(SpaceColumns.ID+" = ?", spaceID).
		Where(SpaceColumns.DeletedAt+" = ?", 0).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return servicespace.Space{}, servicespace.ErrSpaceNotFound
	}
	if err != nil {
		return servicespace.Space{}, err
	}
	return spaceFromDO(row), nil
}

func (r *SpaceRepository) AddMember(ctx context.Context, member servicespace.Member) (servicespace.Member, error) {
	var created SpaceMemberDO
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := r.validateEnabledSpace(ctx, tx, member.TenantID, member.SpaceID); err != nil {
			return err
		}
		if err := r.validateEnabledTenantUser(ctx, tx, member.TenantID, member.UserID); err != nil {
			return err
		}
		now := r.now()
		row := SpaceMemberDO{
			BaseFields: BaseFields{
				CreatedAt:     now,
				CreatedByType: AuditActorTenantUser,
				UpdatedAt:     now,
				UpdatedByType: AuditActorTenantUser,
				Version:       1,
				ExtJSON:       datatypes.JSON([]byte("{}")),
			},
			TenantID:    member.TenantID,
			SpaceID:     member.SpaceID,
			UserID:      member.UserID,
			RoleInSpace: member.Role,
			Status:      member.Status,
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		created = row
		return nil
	})
	if err != nil {
		return servicespace.Member{}, err
	}
	return memberFromDO(created), nil
}

func (r *SpaceRepository) ListEffectiveMembers(ctx context.Context, tenantID uint64, spaceID uint64) ([]servicespace.Member, error) {
	var rows []SpaceMemberDO
	if err := r.db.WithContext(ctx).
		Model(&SpaceMemberDO{}).
		Joins("JOIN spaces ON spaces.tenant_id = space_members.tenant_id AND spaces.id = space_members.space_id").
		Joins("JOIN users ON users.tenant_id = space_members.tenant_id AND users.id = space_members.user_id").
		Where("space_members."+SpaceMemberColumns.TenantID+" = ?", tenantID).
		Where("space_members."+SpaceMemberColumns.SpaceID+" = ?", spaceID).
		Where("space_members."+SpaceMemberColumns.Status+" = ?", servicespace.StatusEnabled).
		Where("space_members."+SpaceMemberColumns.DeletedAt+" = ?", 0).
		Where("spaces."+SpaceColumns.Status+" = ?", servicespace.StatusEnabled).
		Where("spaces."+SpaceColumns.DeletedAt+" = ?", 0).
		Where("users."+UserColumns.Status+" = ?", servicetenantuser.StatusEnabled).
		Where("users."+UserColumns.DeletedAt+" = ?", 0).
		Order("space_members." + SpaceMemberColumns.ID + " ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	members := make([]servicespace.Member, 0, len(rows))
	for _, row := range rows {
		members = append(members, memberFromDO(row))
	}
	return members, nil
}

func (r *SpaceRepository) ListEffectiveMembershipsForUser(ctx context.Context, tenantID uint64, userID uint64) ([]servicespace.Member, error) {
	var rows []SpaceMemberDO
	if err := r.db.WithContext(ctx).
		Model(&SpaceMemberDO{}).
		Joins("JOIN spaces ON spaces.tenant_id = space_members.tenant_id AND spaces.id = space_members.space_id").
		Joins("JOIN users ON users.tenant_id = space_members.tenant_id AND users.id = space_members.user_id").
		Where("space_members."+SpaceMemberColumns.TenantID+" = ?", tenantID).
		Where("space_members."+SpaceMemberColumns.UserID+" = ?", userID).
		Where("space_members."+SpaceMemberColumns.Status+" = ?", servicespace.StatusEnabled).
		Where("space_members."+SpaceMemberColumns.DeletedAt+" = ?", 0).
		Where("spaces."+SpaceColumns.Status+" = ?", servicespace.StatusEnabled).
		Where("spaces."+SpaceColumns.DeletedAt+" = ?", 0).
		Where("users."+UserColumns.Status+" = ?", servicetenantuser.StatusEnabled).
		Where("users."+UserColumns.DeletedAt+" = ?", 0).
		Order("space_members." + SpaceMemberColumns.SpaceID + " ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	members := make([]servicespace.Member, 0, len(rows))
	for _, row := range rows {
		members = append(members, memberFromDO(row))
	}
	return members, nil
}

func (r *SpaceRepository) FindMember(ctx context.Context, tenantID uint64, spaceID uint64, userID uint64) (servicespace.Member, error) {
	var row SpaceMemberDO
	err := r.db.WithContext(ctx).
		Model(&SpaceMemberDO{}).
		Joins("JOIN spaces ON spaces.tenant_id = space_members.tenant_id AND spaces.id = space_members.space_id").
		Joins("JOIN users ON users.tenant_id = space_members.tenant_id AND users.id = space_members.user_id").
		Where("space_members."+SpaceMemberColumns.TenantID+" = ?", tenantID).
		Where("space_members."+SpaceMemberColumns.SpaceID+" = ?", spaceID).
		Where("space_members."+SpaceMemberColumns.UserID+" = ?", userID).
		Where("space_members."+SpaceMemberColumns.DeletedAt+" = ?", 0).
		Where("spaces."+SpaceColumns.Status+" = ?", servicespace.StatusEnabled).
		Where("spaces."+SpaceColumns.DeletedAt+" = ?", 0).
		Where("users."+UserColumns.Status+" = ?", servicetenantuser.StatusEnabled).
		Where("users."+UserColumns.DeletedAt+" = ?", 0).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return servicespace.Member{}, servicespace.ErrMemberNotFound
	}
	if err != nil {
		return servicespace.Member{}, err
	}
	return memberFromDO(row), nil
}

func (r *SpaceRepository) CountEnabledSpaceAdmins(ctx context.Context, tenantID uint64, spaceID uint64) (int64, error) {
	return r.countEnabledSpaceAdmins(ctx, r.db, tenantID, spaceID)
}

func (r *SpaceRepository) countEnabledSpaceAdmins(ctx context.Context, gormDB *gorm.DB, tenantID uint64, spaceID uint64) (int64, error) {
	var count int64
	err := gormDB.WithContext(ctx).Model(&SpaceMemberDO{}).
		Joins("JOIN spaces ON spaces.tenant_id = space_members.tenant_id AND spaces.id = space_members.space_id").
		Joins("JOIN users ON users.tenant_id = space_members.tenant_id AND users.id = space_members.user_id").
		Where("space_members."+SpaceMemberColumns.TenantID+" = ?", tenantID).
		Where("space_members."+SpaceMemberColumns.SpaceID+" = ?", spaceID).
		Where("space_members."+SpaceMemberColumns.RoleInSpace+" = ?", servicespace.RoleSpaceAdmin).
		Where("space_members."+SpaceMemberColumns.Status+" = ?", servicespace.StatusEnabled).
		Where("space_members."+SpaceMemberColumns.DeletedAt+" = ?", 0).
		Where("spaces.status = ?", servicespace.StatusEnabled).
		Where("spaces.deleted_at = ?", 0).
		Where("users.status = ?", servicetenantuser.StatusEnabled).
		Where("users.deleted_at = ?", 0).
		Count(&count).Error
	return count, err
}

func (r *SpaceRepository) ValidateBeforeDisableTenantUser(ctx context.Context, tenantID uint64, userID uint64) error {
	var rows []SpaceMemberDO
	enabledSpaceIDs := r.db.WithContext(ctx).Model(&SpaceDO{}).
		Select(SpaceColumns.ID).
		Where(SpaceColumns.TenantID+" = ?", tenantID).
		Where(SpaceColumns.Status+" = ?", servicespace.StatusEnabled).
		Where(SpaceColumns.DeletedAt+" = ?", 0)
	if err := r.db.WithContext(ctx).
		Where(SpaceMemberColumns.TenantID+" = ?", tenantID).
		Where(SpaceMemberColumns.UserID+" = ?", userID).
		Where(SpaceMemberColumns.RoleInSpace+" = ?", servicespace.RoleSpaceAdmin).
		Where(SpaceMemberColumns.Status+" = ?", servicespace.StatusEnabled).
		Where(SpaceMemberColumns.DeletedAt+" = ?", 0).
		Where(SpaceMemberColumns.SpaceID+" IN (?)", enabledSpaceIDs).
		Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		count, err := r.CountEnabledSpaceAdmins(ctx, tenantID, row.SpaceID)
		if err != nil {
			return err
		}
		if count <= 1 {
			return servicespace.ErrCannotLoseLastSpaceAdmin
		}
	}
	return nil
}

func (r *SpaceRepository) DisableMember(ctx context.Context, tenantID uint64, spaceID uint64, userID uint64) error {
	return r.updateMember(ctx, tenantID, spaceID, userID, map[string]any{
		SpaceMemberColumns.Status: servicespace.StatusDisabled,
	})
}

func (r *SpaceRepository) RemoveMember(ctx context.Context, tenantID uint64, spaceID uint64, userID uint64) error {
	return r.updateMember(ctx, tenantID, spaceID, userID, map[string]any{
		SpaceMemberColumns.DeletedAt: soft_delete.DeletedAt(r.now()),
	})
}

func (r *SpaceRepository) UpdateMemberRole(ctx context.Context, tenantID uint64, spaceID uint64, userID uint64, role string) error {
	return r.updateMember(ctx, tenantID, spaceID, userID, map[string]any{
		SpaceMemberColumns.RoleInSpace: role,
	})
}

func (r *SpaceRepository) updateMember(ctx context.Context, tenantID uint64, spaceID uint64, userID uint64, updates map[string]any) error {
	updates[BaseColumns.UpdatedAt] = r.now()
	updates[BaseColumns.UpdatedByType] = AuditActorTenantUser
	updates[BaseColumns.Version] = gorm.Expr(BaseColumns.Version + " + 1")
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := r.lockSpaceAdminRows(ctx, tx, tenantID, spaceID); err != nil {
			return err
		}
		if _, ok := updates[SpaceMemberColumns.RoleInSpace]; ok {
			if err := r.validateEnabledTenantUser(ctx, tx, tenantID, userID); err != nil {
				return err
			}
		}
		result := tx.Model(&SpaceMemberDO{}).
			Where(SpaceMemberColumns.TenantID+" = ?", tenantID).
			Where(SpaceMemberColumns.SpaceID+" = ?", spaceID).
			Where(SpaceMemberColumns.UserID+" = ?", userID).
			Where(SpaceMemberColumns.DeletedAt+" = ?", 0).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return servicespace.ErrMemberNotFound
		}
		enabled, err := r.spaceEnabled(ctx, tx, tenantID, spaceID)
		if err != nil {
			return err
		}
		if !enabled {
			return nil
		}
		count, err := r.countEnabledSpaceAdmins(ctx, tx, tenantID, spaceID)
		if err != nil {
			return err
		}
		if count == 0 {
			return servicespace.ErrCannotLoseLastSpaceAdmin
		}
		return nil
	})
}

func (r *SpaceRepository) lockSpaceAdminRows(ctx context.Context, tx *gorm.DB, tenantID uint64, spaceID uint64) error {
	var rows []SpaceMemberDO
	return r.spaceAdminLockQuery(ctx, tx, tenantID, spaceID).Find(&rows).Error
}

func (r *SpaceRepository) spaceAdminLockQuery(ctx context.Context, gormDB *gorm.DB, tenantID uint64, spaceID uint64) *gorm.DB {
	enabledSpaceIDs := gormDB.WithContext(ctx).Model(&SpaceDO{}).
		Select(SpaceColumns.ID).
		Where(SpaceColumns.TenantID+" = ?", tenantID).
		Where(SpaceColumns.Status+" = ?", servicespace.StatusEnabled).
		Where(SpaceColumns.DeletedAt+" = ?", 0)
	return gormDB.WithContext(ctx).Model(&SpaceMemberDO{}).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where(SpaceMemberColumns.TenantID+" = ?", tenantID).
		Where(SpaceMemberColumns.SpaceID+" = ?", spaceID).
		Where(SpaceMemberColumns.SpaceID+" IN (?)", enabledSpaceIDs).
		Where(SpaceMemberColumns.RoleInSpace+" = ?", servicespace.RoleSpaceAdmin).
		Where(SpaceMemberColumns.Status+" = ?", servicespace.StatusEnabled).
		Where(SpaceMemberColumns.DeletedAt+" = ?", 0)
}

func (r *SpaceRepository) spaceEnabled(ctx context.Context, gormDB *gorm.DB, tenantID uint64, spaceID uint64) (bool, error) {
	var count int64
	err := gormDB.WithContext(ctx).Model(&SpaceDO{}).
		Where(SpaceColumns.TenantID+" = ?", tenantID).
		Where(SpaceColumns.ID+" = ?", spaceID).
		Where(SpaceColumns.Status+" = ?", servicespace.StatusEnabled).
		Where(SpaceColumns.DeletedAt+" = ?", 0).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *SpaceRepository) validateEnabledSpace(ctx context.Context, gormDB *gorm.DB, tenantID uint64, spaceID uint64) error {
	enabled, err := r.spaceEnabled(ctx, gormDB, tenantID, spaceID)
	if err != nil {
		return err
	}
	if !enabled {
		return servicespace.ErrSpaceNotFound
	}
	return nil
}

func (r *SpaceRepository) SpaceExists(ctx context.Context, tenantID uint64, spaceID uint64) (bool, error) {
	return r.spaceEnabled(ctx, r.db, tenantID, spaceID)
}

func (r *SpaceRepository) validateEnabledTenantUser(ctx context.Context, gormDB *gorm.DB, tenantID uint64, userID uint64) error {
	var count int64
	err := gormDB.WithContext(ctx).Model(&UserDO{}).
		Where(UserColumns.TenantID+" = ?", tenantID).
		Where(UserColumns.ID+" = ?", userID).
		Where(UserColumns.Status+" = ?", servicetenantuser.StatusEnabled).
		Where(UserColumns.DeletedAt+" = ?", 0).
		Count(&count).Error
	if err != nil {
		return err
	}
	if count == 0 {
		return servicespace.ErrMemberUserUnavailable
	}
	return nil
}

func (r *SpaceRepository) ListMemberNames(ctx context.Context, tenantID uint64, spaceID uint64) ([]SpaceMemberName, error) {
	var rows []SpaceMemberName
	err := r.db.WithContext(ctx).Table(SpaceMemberDO{}.TableName()+" AS sm").
		Select("sm.id, sm.tenant_id, sm.space_id, sm.user_id, u.real_name AS name, sm.role_in_space AS role, sm.status").
		Joins("LEFT JOIN users AS u ON u.tenant_id = sm.tenant_id AND u.id = sm.user_id AND u.deleted_at = 0").
		Where("sm.tenant_id = ?", tenantID).
		Where("sm.space_id = ?", spaceID).
		Where("sm.deleted_at = ?", 0).
		Order("sm.id ASC").
		Scan(&rows).Error
	return rows, err
}

type SpaceMemberName struct {
	ID       uint64 `gorm:"column:id"`
	TenantID uint64 `gorm:"column:tenant_id"`
	SpaceID  uint64 `gorm:"column:space_id"`
	UserID   uint64 `gorm:"column:user_id"`
	Name     string `gorm:"column:name"`
	Role     string `gorm:"column:role"`
	Status   string `gorm:"column:status"`
}

func spaceFromDO(row SpaceDO) servicespace.Space {
	return servicespace.Space{
		ID:          row.ID,
		TenantID:    row.TenantID,
		Name:        row.Name,
		LogoURL:     row.LogoURL,
		Description: row.Description,
		Type:        row.Type,
		Status:      row.Status,
	}
}

func memberFromDO(row SpaceMemberDO) servicespace.Member {
	return servicespace.Member{
		ID:       row.ID,
		TenantID: row.TenantID,
		SpaceID:  row.SpaceID,
		UserID:   row.UserID,
		Role:     row.RoleInSpace,
		Status:   row.Status,
	}
}

package db

import (
	"context"
	"errors"
	"time"

	"github.com/lifei6671/papermind/server/internal/service/pagination"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/plugin/soft_delete"

	servicespace "github.com/lifei6671/papermind/server/internal/service/space"
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
				CreatedAt: now,
				UpdatedAt: now,
				Version:   1,
				ExtJSON:   datatypes.JSON("{}"),
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
			member := SpaceMemberDO{
				BaseFields: BaseFields{
					CreatedAt: now,
					UpdatedAt: now,
					Version:   1,
					ExtJSON:   datatypes.JSON("{}"),
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

func (r *SpaceRepository) AddMember(ctx context.Context, member servicespace.Member) (servicespace.Member, error) {
	now := r.now()
	row := SpaceMemberDO{
		BaseFields: BaseFields{
			CreatedAt: now,
			UpdatedAt: now,
			Version:   1,
			ExtJSON:   datatypes.JSON([]byte("{}")),
		},
		TenantID:    member.TenantID,
		SpaceID:     member.SpaceID,
		UserID:      member.UserID,
		RoleInSpace: member.Role,
		Status:      member.Status,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return servicespace.Member{}, err
	}
	return memberFromDO(row), nil
}

func (r *SpaceRepository) ListEffectiveMembers(ctx context.Context, tenantID uint64, spaceID uint64) ([]servicespace.Member, error) {
	var rows []SpaceMemberDO
	if err := r.db.WithContext(ctx).
		Where(SpaceMemberColumns.TenantID+" = ?", tenantID).
		Where(SpaceMemberColumns.SpaceID+" = ?", spaceID).
		Where(SpaceMemberColumns.Status+" = ?", servicespace.StatusEnabled).
		Where(SpaceMemberColumns.DeletedAt+" = ?", 0).
		Order(SpaceMemberColumns.ID + " ASC").
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
		Where(SpaceMemberColumns.TenantID+" = ?", tenantID).
		Where(SpaceMemberColumns.SpaceID+" = ?", spaceID).
		Where(SpaceMemberColumns.UserID+" = ?", userID).
		Where(SpaceMemberColumns.DeletedAt+" = ?", 0).
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
	var count int64
	err := r.db.WithContext(ctx).Model(&SpaceMemberDO{}).
		Where(SpaceMemberColumns.TenantID+" = ?", tenantID).
		Where(SpaceMemberColumns.SpaceID+" = ?", spaceID).
		Where(SpaceMemberColumns.RoleInSpace+" = ?", servicespace.RoleSpaceAdmin).
		Where(SpaceMemberColumns.Status+" = ?", servicespace.StatusEnabled).
		Where(SpaceMemberColumns.DeletedAt+" = ?", 0).
		Count(&count).Error
	return count, err
}

func (r *SpaceRepository) ValidateBeforeDisableTenantUser(ctx context.Context, tenantID uint64, userID uint64) error {
	var rows []SpaceMemberDO
	if err := r.db.WithContext(ctx).
		Where(SpaceMemberColumns.TenantID+" = ?", tenantID).
		Where(SpaceMemberColumns.UserID+" = ?", userID).
		Where(SpaceMemberColumns.RoleInSpace+" = ?", servicespace.RoleSpaceAdmin).
		Where(SpaceMemberColumns.Status+" = ?", servicespace.StatusEnabled).
		Where(SpaceMemberColumns.DeletedAt+" = ?", 0).
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
	updates[BaseColumns.Version] = gorm.Expr(BaseColumns.Version + " + 1")
	return r.db.WithContext(ctx).Model(&SpaceMemberDO{}).
		Where(SpaceMemberColumns.TenantID+" = ?", tenantID).
		Where(SpaceMemberColumns.SpaceID+" = ?", spaceID).
		Where(SpaceMemberColumns.UserID+" = ?", userID).
		Where(SpaceMemberColumns.DeletedAt+" = ?", 0).
		Updates(updates).Error
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

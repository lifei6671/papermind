package db

import (
	"context"
	"strings"
	"time"

	"github.com/lifei6671/papermind/server/internal/service/pagination"
	servicetenant "github.com/lifei6671/papermind/server/internal/service/tenant"
	"github.com/lifei6671/papermind/server/library/constant"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type TenantRepository struct {
	db  *gorm.DB
	now func() int64
}

type TenantRepositoryOptions struct {
	Now func() int64
}

func NewTenantRepository(gormDB *gorm.DB, options TenantRepositoryOptions) *TenantRepository {
	now := options.Now
	if now == nil {
		now = func() int64 { return time.Now().UnixMilli() }
	}
	return &TenantRepository{db: gormDB, now: now}
}

func (r *TenantRepository) List(ctx context.Context, input servicetenant.ListInput) (pagination.Result[servicetenant.Tenant], error) {
	page := pagination.Normalize(pagination.Input{Page: input.Page, PageSize: input.PageSize})
	query := r.db.WithContext(ctx).Model(&TenantDO{}).
		Where(TenantColumns.DeletedAt+" = ?", 0)
	if keyword := strings.TrimSpace(input.Keyword); keyword != "" {
		likePattern := "%" + keyword + "%"
		// 租户检索只覆盖列表可见字段，避免隐藏字段命中后前端无法解释结果来源。
		query = query.Where(
			r.db.Where(TenantColumns.Name+" LIKE ?", likePattern).
				Or(TenantColumns.Description+" LIKE ?", likePattern).
				Or(TenantColumns.TenantCode+" LIKE ?", likePattern),
		)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return pagination.Result[servicetenant.Tenant]{}, err
	}
	var rows []TenantDO
	if err := query.
		Order(TenantColumns.ID + " DESC").
		Limit(page.PageSize).
		Offset(pagination.Offset(page)).
		Find(&rows).Error; err != nil {
		return pagination.Result[servicetenant.Tenant]{}, err
	}
	tenants := make([]servicetenant.Tenant, 0, len(rows))
	for _, row := range rows {
		tenants = append(tenants, tenantFromDO(row))
	}
	return pagination.Result[servicetenant.Tenant]{
		Items:    tenants,
		Page:     page.Page,
		PageSize: page.PageSize,
		Total:    total,
	}, nil
}

func (r *TenantRepository) CreateWithAdmin(ctx context.Context, tenant servicetenant.Tenant, admin servicetenant.InitialAdmin) (servicetenant.Tenant, error) {
	now := r.now()
	row := TenantDO{
		BaseFields: BaseFields{
			CreatedAt:     now,
			CreatedBy:     tenant.CreatedBy,
			CreatedByType: AuditActorPlatformUser,
			UpdatedAt:     now,
			UpdatedBy:     tenant.UpdatedBy,
			UpdatedByType: AuditActorPlatformUser,
			Version:       1,
			ExtJSON:       datatypes.JSON([]byte("{}")),
		},
		Name:          tenant.Name,
		LogoURL:       tenant.LogoURL,
		Description:   tenant.Description,
		TenantCode:    tenant.TenantCode,
		AllowRegister: tenant.AllowRegister,
		Status:        tenant.Status,
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		userRow := UserDO{
			BaseFields: BaseFields{
				CreatedAt:     now,
				CreatedBy:     tenant.CreatedBy,
				CreatedByType: AuditActorPlatformUser,
				UpdatedAt:     now,
				UpdatedBy:     tenant.UpdatedBy,
				UpdatedByType: AuditActorPlatformUser,
				Version:       1,
				ExtJSON:       datatypes.JSON([]byte("{}")),
			},
			Username:     admin.Username,
			RealName:     admin.RealName,
			Phone:        admin.Phone,
			Email:        admin.Email,
			PasswordHash: admin.PasswordHash,
			Status:       servicetenant.StatusEnabled,
		}
		if err := tx.Create(&userRow).Error; err != nil {
			return err
		}
		roleRow := UserRoleDO{
			BaseFields: BaseFields{
				CreatedAt:     now,
				CreatedBy:     tenant.CreatedBy,
				CreatedByType: AuditActorPlatformUser,
				UpdatedAt:     now,
				UpdatedBy:     tenant.UpdatedBy,
				UpdatedByType: AuditActorPlatformUser,
				Version:       1,
				ExtJSON:       datatypes.JSON([]byte("{}")),
			},
			TenantID: row.ID,
			UserID:   userRow.ID,
			Role:     constant.RoleTenantAdmin,
			Status:   servicetenant.StatusEnabled,
		}
		return tx.Create(&roleRow).Error
	})
	if err != nil {
		return servicetenant.Tenant{}, err
	}
	return tenantFromDO(row), nil
}

func (r *TenantRepository) FindByID(ctx context.Context, tenantID uint64) (servicetenant.Tenant, error) {
	var row TenantDO
	if err := r.db.WithContext(ctx).
		Where(TenantColumns.ID+" = ?", tenantID).
		Where(TenantColumns.DeletedAt+" = ?", 0).
		First(&row).Error; err != nil {
		return servicetenant.Tenant{}, err
	}
	return tenantFromDO(row), nil
}

func (r *TenantRepository) TenantCodeExists(ctx context.Context, tenantCode string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&TenantDO{}).
		Where(TenantColumns.TenantCode+" = ?", tenantCode).
		Where(TenantColumns.DeletedAt+" = ?", 0).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *TenantRepository) UpdateTenantCode(ctx context.Context, tenantID uint64, tenantCode string, actorID uint64) (servicetenant.Tenant, error) {
	if err := r.db.WithContext(ctx).Model(&TenantDO{}).
		Where(TenantColumns.ID+" = ?", tenantID).
		Where(TenantColumns.DeletedAt+" = ?", 0).
		Updates(map[string]any{
			TenantColumns.TenantCode:  tenantCode,
			BaseColumns.UpdatedAt:     r.now(),
			BaseColumns.UpdatedBy:     actorID,
			BaseColumns.UpdatedByType: AuditActorPlatformUser,
			BaseColumns.Version:       gorm.Expr(BaseColumns.Version + " + 1"),
		}).Error; err != nil {
		return servicetenant.Tenant{}, err
	}
	return r.FindByID(ctx, tenantID)
}

func (r *TenantRepository) UpdateProfile(ctx context.Context, input servicetenant.UpdateProfileInput) (servicetenant.Tenant, error) {
	if err := r.db.WithContext(ctx).Model(&TenantDO{}).
		Where(TenantColumns.ID+" = ?", input.TenantID).
		Where(TenantColumns.DeletedAt+" = ?", 0).
		Updates(map[string]any{
			TenantColumns.Name:        input.Name,
			TenantColumns.LogoURL:     input.LogoURL,
			TenantColumns.Description: input.Description,
			BaseColumns.UpdatedAt:     r.now(),
			BaseColumns.UpdatedBy:     input.ActorID,
			BaseColumns.UpdatedByType: AuditActorPlatformUser,
			BaseColumns.Version:       gorm.Expr(BaseColumns.Version + " + 1"),
		}).Error; err != nil {
		return servicetenant.Tenant{}, err
	}
	return r.FindByID(ctx, input.TenantID)
}

func (r *TenantRepository) UpdateAllowRegister(ctx context.Context, tenantID uint64, allowRegister bool, actorID uint64) (servicetenant.Tenant, error) {
	if err := r.db.WithContext(ctx).Model(&TenantDO{}).
		Where(TenantColumns.ID+" = ?", tenantID).
		Where(TenantColumns.DeletedAt+" = ?", 0).
		Updates(map[string]any{
			TenantColumns.AllowRegister: allowRegister,
			BaseColumns.UpdatedAt:       r.now(),
			BaseColumns.UpdatedBy:       actorID,
			BaseColumns.UpdatedByType:   AuditActorPlatformUser,
			BaseColumns.Version:         gorm.Expr(BaseColumns.Version + " + 1"),
		}).Error; err != nil {
		return servicetenant.Tenant{}, err
	}
	return r.FindByID(ctx, tenantID)
}

func (r *TenantRepository) UpdateStatus(ctx context.Context, tenantID uint64, status string, actorID uint64) error {
	return r.db.WithContext(ctx).Model(&TenantDO{}).
		Where(TenantColumns.ID+" = ?", tenantID).
		Where(TenantColumns.DeletedAt+" = ?", 0).
		Updates(map[string]any{
			TenantColumns.Status:      status,
			BaseColumns.UpdatedAt:     r.now(),
			BaseColumns.UpdatedBy:     actorID,
			BaseColumns.UpdatedByType: AuditActorPlatformUser,
			BaseColumns.Version:       gorm.Expr(BaseColumns.Version + " + 1"),
		}).Error
}

func tenantFromDO(row TenantDO) servicetenant.Tenant {
	return servicetenant.Tenant{
		ID:            row.ID,
		Name:          row.Name,
		LogoURL:       row.LogoURL,
		Description:   row.Description,
		TenantCode:    row.TenantCode,
		AllowRegister: row.AllowRegister,
		Status:        row.Status,
		CreatedBy:     row.CreatedBy,
		UpdatedBy:     row.UpdatedBy,
	}
}

package db

import (
	"context"
	"time"

	"github.com/lifei6671/papermind/server/internal/service/pagination"
	servicetenant "github.com/lifei6671/papermind/server/internal/service/tenant"
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

func (r *TenantRepository) List(ctx context.Context, page pagination.Input) (pagination.Result[servicetenant.Tenant], error) {
	page = pagination.Normalize(page)
	query := r.db.WithContext(ctx).Model(&TenantDO{}).
		Where(TenantColumns.DeletedAt+" = ?", 0)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return pagination.Result[servicetenant.Tenant]{}, err
	}
	var rows []TenantDO
	if err := query.
		Order(TenantColumns.ID + " ASC").
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

func (r *TenantRepository) Create(ctx context.Context, tenant servicetenant.Tenant) (servicetenant.Tenant, error) {
	now := r.now()
	row := TenantDO{
		BaseFields: BaseFields{
			CreatedAt: now,
			UpdatedAt: now,
			Version:   1,
			ExtJSON:   datatypes.JSON([]byte("{}")),
		},
		Name:          tenant.Name,
		LogoURL:       tenant.LogoURL,
		Description:   tenant.Description,
		TenantCode:    tenant.TenantCode,
		AllowRegister: tenant.AllowRegister,
		Status:        tenant.Status,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
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

func (r *TenantRepository) UpdateTenantCode(ctx context.Context, tenantID uint64, tenantCode string) (servicetenant.Tenant, error) {
	if err := r.db.WithContext(ctx).Model(&TenantDO{}).
		Where(TenantColumns.ID+" = ?", tenantID).
		Where(TenantColumns.DeletedAt+" = ?", 0).
		Updates(map[string]any{
			TenantColumns.TenantCode: tenantCode,
			BaseColumns.UpdatedAt:    r.now(),
			BaseColumns.Version:      gorm.Expr(BaseColumns.Version + " + 1"),
		}).Error; err != nil {
		return servicetenant.Tenant{}, err
	}
	return r.FindByID(ctx, tenantID)
}

func (r *TenantRepository) UpdateStatus(ctx context.Context, tenantID uint64, status string) error {
	return r.db.WithContext(ctx).Model(&TenantDO{}).
		Where(TenantColumns.ID+" = ?", tenantID).
		Where(TenantColumns.DeletedAt+" = ?", 0).
		Updates(map[string]any{
			TenantColumns.Status:  status,
			BaseColumns.UpdatedAt: r.now(),
			BaseColumns.Version:   gorm.Expr(BaseColumns.Version + " + 1"),
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
	}
}

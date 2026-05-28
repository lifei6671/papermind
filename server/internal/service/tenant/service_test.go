package tenant

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lifei6671/papermind/server/internal/service/pagination"
)

func TestCreateTenantGeneratesUniqueCodeAndInheritsDefaultRegisterSetting(t *testing.T) {
	repo := &fakeRepository{
		existingCodes: map[string]bool{
			"DUPLICATE": true,
		},
	}
	generator := &fakeCodeGenerator{codes: []string{"DUPLICATE", "TENANT001"}}
	svc := NewService(ServiceOptions{
		Repo:                 repo,
		CodeGenerator:        generator,
		AllowRegisterDefault: true,
	})

	created, err := svc.Create(context.Background(), CreateInput{
		Name:        "第一中学",
		LogoURL:     "logos/tenant.png",
		Description: "面向校内考试的租户",
		ActorID:     99,
		InitialAdmin: InitialAdmin{
			Username:     "tenant.admin",
			RealName:     "租户管理员",
			Phone:        "13800000000",
			Email:        "admin@example.test",
			PasswordHash: "hashed-password",
		},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if created.ID != 1 {
		t.Fatalf("expected created tenant ID 1, got %d", created.ID)
	}
	if repo.created.TenantCode != "TENANT001" {
		t.Fatalf("expected unique tenant code TENANT001, got %q", repo.created.TenantCode)
	}
	if !repo.created.AllowRegister {
		t.Fatalf("expected allow_register to inherit true default")
	}
	if repo.created.LogoURL != "logos/tenant.png" {
		t.Fatalf("expected logo URL saved, got %q", repo.created.LogoURL)
	}
	if repo.created.Description != "面向校内考试的租户" {
		t.Fatalf("expected description saved, got %q", repo.created.Description)
	}
	if repo.created.Status != StatusEnabled {
		t.Fatalf("expected tenant status enabled, got %q", repo.created.Status)
	}
	if repo.created.CreatedBy != 99 || repo.created.UpdatedBy != 99 {
		t.Fatalf("expected tenant audit user 99, got created_by=%d updated_by=%d", repo.created.CreatedBy, repo.created.UpdatedBy)
	}
	if repo.createdAdmin.Username != "tenant.admin" || repo.createdAdmin.PasswordHash != "hashed-password" {
		t.Fatalf("expected first tenant admin saved with tenant, got %#v", repo.createdAdmin)
	}
}

func TestCreateTenantCanOverrideRegisterSetting(t *testing.T) {
	repo := &fakeRepository{}
	generator := &fakeCodeGenerator{codes: []string{"TENANT001"}}
	allowRegister := false
	svc := NewService(ServiceOptions{
		Repo:                 repo,
		CodeGenerator:        generator,
		AllowRegisterDefault: true,
	})

	created, err := svc.Create(context.Background(), CreateInput{
		Name:          "第一中学",
		AllowRegister: &allowRegister,
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if created.AllowRegister {
		t.Fatalf("expected explicit allow_register=false, got %#v", created)
	}
}

func TestCreateTenantFailsWhenCodeGeneratorCannotProduceUniqueCode(t *testing.T) {
	repo := &fakeRepository{
		existingCodes: map[string]bool{
			"DUPLICATE": true,
		},
	}
	generator := &fakeCodeGenerator{codes: []string{"DUPLICATE", "DUPLICATE"}}
	svc := NewService(ServiceOptions{
		Repo:                    repo,
		CodeGenerator:           generator,
		MaxCodeGenerateAttempts: 2,
	})

	_, err := svc.Create(context.Background(), CreateInput{Name: "第一中学"})
	if !errors.Is(err, ErrTenantCodeCollision) {
		t.Fatalf("expected ErrTenantCodeCollision, got %v", err)
	}
	if repo.created.Name != "" {
		t.Fatalf("expected no tenant created, got %#v", repo.created)
	}
}

func TestGetTenantCodeReturnsCurrentCode(t *testing.T) {
	repo := &fakeRepository{
		tenantsByID: map[uint64]Tenant{
			12: {ID: 12, TenantCode: "TENANT001"},
		},
	}
	svc := NewService(ServiceOptions{Repo: repo})

	code, err := svc.GetTenantCode(context.Background(), 12)
	if err != nil {
		t.Fatalf("GetTenantCode returned error: %v", err)
	}
	if code != "TENANT001" {
		t.Fatalf("expected TENANT001, got %q", code)
	}
}

func TestResetTenantCodeGeneratesNewUniqueCode(t *testing.T) {
	repo := &fakeRepository{
		tenantsByID: map[uint64]Tenant{
			12: {ID: 12, TenantCode: "OLD001"},
		},
		existingCodes: map[string]bool{
			"OLD001": true,
		},
	}
	generator := &fakeCodeGenerator{codes: []string{"OLD001", "NEW001"}}
	svc := NewService(ServiceOptions{
		Repo:          repo,
		CodeGenerator: generator,
	})

	updated, err := svc.ResetTenantCode(context.Background(), ResetTenantCodeInput{TenantID: 12, ActorID: 99})
	if err != nil {
		t.Fatalf("ResetTenantCode returned error: %v", err)
	}
	if updated.TenantCode != "NEW001" {
		t.Fatalf("expected NEW001, got %q", updated.TenantCode)
	}
	if repo.updatedCodeTenantID != 12 || repo.updatedCode != "NEW001" {
		t.Fatalf("expected code update tenant=12 code=NEW001, got tenant=%d code=%q", repo.updatedCodeTenantID, repo.updatedCode)
	}
	if repo.updatedCodeActorID != 99 {
		t.Fatalf("expected code update actor 99, got %d", repo.updatedCodeActorID)
	}
}

func TestResetTenantCodeUsesDefaultGeneratorWhenNotConfigured(t *testing.T) {
	repo := &fakeRepository{
		tenantsByID: map[uint64]Tenant{
			12: {ID: 12, TenantCode: "OLD001"},
		},
	}
	svc := NewService(ServiceOptions{Repo: repo})

	updated, err := svc.ResetTenantCode(context.Background(), ResetTenantCodeInput{TenantID: 12})
	if err != nil {
		t.Fatalf("ResetTenantCode returned error: %v", err)
	}
	if updated.TenantCode == "" || updated.TenantCode == "OLD001" {
		t.Fatalf("expected generated tenant code, got %#v", updated)
	}
}

func TestEnableAndDisableTenantUpdateStatus(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewService(ServiceOptions{Repo: repo})

	if err := svc.Enable(context.Background(), UpdateStatusInput{TenantID: 12, ActorID: 99}); err != nil {
		t.Fatalf("Enable returned error: %v", err)
	}
	if repo.updatedStatusTenantID != 12 || repo.updatedStatus != StatusEnabled {
		t.Fatalf("expected enabled status update, got tenant=%d status=%q", repo.updatedStatusTenantID, repo.updatedStatus)
	}
	if repo.updatedStatusActorID != 99 {
		t.Fatalf("expected enabled status actor 99, got %d", repo.updatedStatusActorID)
	}

	if err := svc.Disable(context.Background(), UpdateStatusInput{TenantID: 12, ActorID: 88}); err != nil {
		t.Fatalf("Disable returned error: %v", err)
	}
	if repo.updatedStatusTenantID != 12 || repo.updatedStatus != StatusDisabled {
		t.Fatalf("expected disabled status update, got tenant=%d status=%q", repo.updatedStatusTenantID, repo.updatedStatus)
	}
	if repo.updatedStatusActorID != 88 {
		t.Fatalf("expected disabled status actor 88, got %d", repo.updatedStatusActorID)
	}
}

func TestUpdateProfileReturnsUpdatedTenant(t *testing.T) {
	repo := &fakeRepository{
		tenantsByID: map[uint64]Tenant{
			12: {ID: 12, Name: "旧租户", LogoURL: "old.png", Description: "旧描述", TenantCode: "TENANT001", AllowRegister: true},
		},
	}
	svc := NewService(ServiceOptions{Repo: repo})

	updated, err := svc.UpdateProfile(context.Background(), UpdateProfileInput{
		TenantID:    12,
		Name:        "新租户",
		LogoURL:     "/uploads/new.webp",
		Description: "新描述",
		ActorID:     99,
	})
	if err != nil {
		t.Fatalf("UpdateProfile returned error: %v", err)
	}
	if updated.Name != "新租户" || updated.LogoURL != "/uploads/new.webp" || updated.Description != "新描述" {
		t.Fatalf("expected updated profile, got %#v", updated)
	}
	if repo.updatedProfile.TenantID != 12 || repo.updatedProfile.Name != "新租户" || repo.updatedProfile.LogoURL != "/uploads/new.webp" || repo.updatedProfile.Description != "新描述" {
		t.Fatalf("expected profile update tenant=12, got %#v", repo.updatedProfile)
	}
	if repo.updatedProfile.ActorID != 99 {
		t.Fatalf("expected profile update actor 99, got %d", repo.updatedProfile.ActorID)
	}
}

func TestUpdateRegisterSettingReturnsUpdatedTenant(t *testing.T) {
	repo := &fakeRepository{
		tenantsByID: map[uint64]Tenant{
			12: {ID: 12, TenantCode: "TENANT001", AllowRegister: true},
		},
	}
	svc := NewService(ServiceOptions{Repo: repo})

	updated, err := svc.UpdateRegisterSetting(context.Background(), UpdateRegisterSettingInput{
		TenantID:      12,
		AllowRegister: false,
		ActorID:       99,
	})
	if err != nil {
		t.Fatalf("UpdateRegisterSetting returned error: %v", err)
	}
	if updated.AllowRegister {
		t.Fatalf("expected registration disabled, got %#v", updated)
	}
	if repo.updatedRegisterTenantID != 12 || repo.updatedAllowRegister {
		t.Fatalf("expected register update tenant=12 allow=false, got tenant=%d allow=%v", repo.updatedRegisterTenantID, repo.updatedAllowRegister)
	}
	if repo.updatedRegisterActorID != 99 {
		t.Fatalf("expected register update actor 99, got %d", repo.updatedRegisterActorID)
	}
}

func TestRandomCodeGeneratorReturnsTenantCode(t *testing.T) {
	generator := RandomCodeGenerator{reader: strings.NewReader("123456789")}

	code, err := generator.NextCode()
	if err != nil {
		t.Fatalf("NextCode returned error: %v", err)
	}
	if len(code) != 10 {
		t.Fatalf("expected tenant code length 10, got %d", len(code))
	}
	if !strings.HasPrefix(code, "T") {
		t.Fatalf("expected tenant code to start with T, got %q", code)
	}
	const allowedCodeChars = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"
	for _, item := range code[1:] {
		if !strings.ContainsRune(allowedCodeChars, item) {
			t.Fatalf("expected tenant code to use allowed characters, got %q in %q", item, code)
		}
	}
}

type fakeRepository struct {
	created      Tenant
	createdAdmin InitialAdmin

	tenantsByID   map[uint64]Tenant
	existingCodes map[string]bool

	updatedCodeTenantID uint64
	updatedCode         string
	updatedCodeActorID  uint64

	updatedStatusTenantID uint64
	updatedStatus         string
	updatedStatusActorID  uint64

	updatedProfile UpdateProfileInput

	updatedRegisterTenantID uint64
	updatedAllowRegister    bool
	updatedRegisterActorID  uint64
}

func (r *fakeRepository) List(ctx context.Context, input ListInput) (pagination.Result[Tenant], error) {
	tenants := make([]Tenant, 0, len(r.tenantsByID))
	for _, tenant := range r.tenantsByID {
		tenants = append(tenants, tenant)
	}
	page := pagination.Normalize(pagination.Input{Page: input.Page, PageSize: input.PageSize})
	return pagination.Result[Tenant]{
		Items:    tenants,
		Page:     page.Page,
		PageSize: page.PageSize,
		Total:    int64(len(tenants)),
	}, nil
}

func (r *fakeRepository) CreateWithAdmin(ctx context.Context, tenant Tenant, admin InitialAdmin) (Tenant, error) {
	tenant.ID = 1
	r.created = tenant
	r.createdAdmin = admin
	return tenant, nil
}

func (r *fakeRepository) FindByID(ctx context.Context, tenantID uint64) (Tenant, error) {
	tenant, ok := r.tenantsByID[tenantID]
	if !ok {
		return Tenant{}, ErrTenantNotFound
	}
	return tenant, nil
}

func (r *fakeRepository) TenantCodeExists(ctx context.Context, code string) (bool, error) {
	return r.existingCodes[code], nil
}

func (r *fakeRepository) UpdateTenantCode(ctx context.Context, tenantID uint64, code string, actorID uint64) (Tenant, error) {
	r.updatedCodeTenantID = tenantID
	r.updatedCode = code
	r.updatedCodeActorID = actorID
	tenant := r.tenantsByID[tenantID]
	tenant.TenantCode = code
	return tenant, nil
}

func (r *fakeRepository) UpdateStatus(ctx context.Context, tenantID uint64, status string, actorID uint64) error {
	r.updatedStatusTenantID = tenantID
	r.updatedStatus = status
	r.updatedStatusActorID = actorID
	return nil
}

func (r *fakeRepository) UpdateProfile(ctx context.Context, input UpdateProfileInput) (Tenant, error) {
	r.updatedProfile = input
	tenant := r.tenantsByID[input.TenantID]
	tenant.Name = input.Name
	tenant.LogoURL = input.LogoURL
	tenant.Description = input.Description
	return tenant, nil
}

func (r *fakeRepository) UpdateAllowRegister(ctx context.Context, tenantID uint64, allowRegister bool, actorID uint64) (Tenant, error) {
	r.updatedRegisterTenantID = tenantID
	r.updatedAllowRegister = allowRegister
	r.updatedRegisterActorID = actorID
	tenant := r.tenantsByID[tenantID]
	tenant.AllowRegister = allowRegister
	return tenant, nil
}

type fakeCodeGenerator struct {
	codes []string
	next  int
}

func (g *fakeCodeGenerator) NextCode() (string, error) {
	code := g.codes[g.next]
	g.next++
	return code, nil
}

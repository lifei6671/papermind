package tenant

import (
	"context"
	"errors"
	"strings"
	"testing"
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

	updated, err := svc.ResetTenantCode(context.Background(), 12)
	if err != nil {
		t.Fatalf("ResetTenantCode returned error: %v", err)
	}
	if updated.TenantCode != "NEW001" {
		t.Fatalf("expected NEW001, got %q", updated.TenantCode)
	}
	if repo.updatedCodeTenantID != 12 || repo.updatedCode != "NEW001" {
		t.Fatalf("expected code update tenant=12 code=NEW001, got tenant=%d code=%q", repo.updatedCodeTenantID, repo.updatedCode)
	}
}

func TestEnableAndDisableTenantUpdateStatus(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewService(ServiceOptions{Repo: repo})

	if err := svc.Enable(context.Background(), 12); err != nil {
		t.Fatalf("Enable returned error: %v", err)
	}
	if repo.updatedStatusTenantID != 12 || repo.updatedStatus != StatusEnabled {
		t.Fatalf("expected enabled status update, got tenant=%d status=%q", repo.updatedStatusTenantID, repo.updatedStatus)
	}

	if err := svc.Disable(context.Background(), 12); err != nil {
		t.Fatalf("Disable returned error: %v", err)
	}
	if repo.updatedStatusTenantID != 12 || repo.updatedStatus != StatusDisabled {
		t.Fatalf("expected disabled status update, got tenant=%d status=%q", repo.updatedStatusTenantID, repo.updatedStatus)
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
	created Tenant

	tenantsByID   map[uint64]Tenant
	existingCodes map[string]bool

	updatedCodeTenantID uint64
	updatedCode         string

	updatedStatusTenantID uint64
	updatedStatus         string
}

func (r *fakeRepository) Create(ctx context.Context, tenant Tenant) (Tenant, error) {
	tenant.ID = 1
	r.created = tenant
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

func (r *fakeRepository) UpdateTenantCode(ctx context.Context, tenantID uint64, code string) (Tenant, error) {
	r.updatedCodeTenantID = tenantID
	r.updatedCode = code
	tenant := r.tenantsByID[tenantID]
	tenant.TenantCode = code
	return tenant, nil
}

func (r *fakeRepository) UpdateStatus(ctx context.Context, tenantID uint64, status string) error {
	r.updatedStatusTenantID = tenantID
	r.updatedStatus = status
	return nil
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

package tenant

import (
	"context"
	"crypto/rand"
	"errors"
	"io"

	"github.com/lifei6671/papermind/server/internal/service/pagination"
)

const (
	// StatusEnabled 表示租户可正常使用。
	StatusEnabled = "enabled"
	// StatusDisabled 表示租户已被禁用。
	StatusDisabled = "disabled"
)

var (
	ErrTenantNotFound       = errors.New("tenant not found")
	ErrTenantCodeCollision  = errors.New("tenant code collision")
	ErrTenantCodeGenerator  = errors.New("tenant code generator is required")
	ErrTenantRepositoryMiss = errors.New("tenant repository is required")
)

type Tenant struct {
	ID            uint64 // 租户主键 ID。
	Name          string // 租户名称，例如学校、企业、培训机构。
	LogoURL       string // 企业或机构 Logo 地址。
	Description   string // 企业或机构描述。
	TenantCode    string // 租户码，用于专属注册链接和手动注册归属。
	AllowRegister bool   // 是否允许该租户用户自注册。
	Status        string // 租户状态：enabled / disabled。
}

type CreateInput struct {
	Name        string // 租户名称。
	LogoURL     string // 企业或机构 Logo 地址。
	Description string // 企业或机构描述。
}

type ListInput struct {
	Page     int
	PageSize int
}

type Repository interface {
	List(ctx context.Context, page pagination.Input) (pagination.Result[Tenant], error)
	Create(ctx context.Context, tenant Tenant) (Tenant, error)
	FindByID(ctx context.Context, tenantID uint64) (Tenant, error)
	TenantCodeExists(ctx context.Context, code string) (bool, error)
	UpdateTenantCode(ctx context.Context, tenantID uint64, code string) (Tenant, error)
	UpdateStatus(ctx context.Context, tenantID uint64, status string) error
}

type CodeGenerator interface {
	NextCode() (string, error)
}

type RandomCodeGenerator struct {
	reader io.Reader
}

func NewRandomCodeGenerator() CodeGenerator {
	return RandomCodeGenerator{reader: rand.Reader}
}

func (g RandomCodeGenerator) NextCode() (string, error) {
	reader := g.reader
	if reader == nil {
		reader = rand.Reader
	}
	buffer := make([]byte, 9)
	if _, err := io.ReadFull(reader, buffer); err != nil {
		return "", err
	}

	const alphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"
	code := make([]byte, 0, 10)
	code = append(code, 'T')
	for _, item := range buffer {
		code = append(code, alphabet[int(item)%len(alphabet)])
	}
	return string(code), nil
}

type ServiceOptions struct {
	Repo                    Repository
	CodeGenerator           CodeGenerator
	AllowRegisterDefault    bool
	MaxCodeGenerateAttempts int
}

type Service struct {
	repo                    Repository
	codeGenerator           CodeGenerator
	allowRegisterDefault    bool
	maxCodeGenerateAttempts int
}

func NewService(options ServiceOptions) *Service {
	attempts := options.MaxCodeGenerateAttempts
	if attempts == 0 {
		attempts = 8
	}
	return &Service{
		repo:                    options.Repo,
		codeGenerator:           options.CodeGenerator,
		allowRegisterDefault:    options.AllowRegisterDefault,
		maxCodeGenerateAttempts: attempts,
	}
}

func (s *Service) List(ctx context.Context, input ListInput) (pagination.Result[Tenant], error) {
	return s.repo.List(ctx, pagination.Input{Page: input.Page, PageSize: input.PageSize})
}

func (s *Service) Create(ctx context.Context, input CreateInput) (Tenant, error) {
	code, err := s.nextUniqueCode(ctx)
	if err != nil {
		return Tenant{}, err
	}
	return s.repo.Create(ctx, Tenant{
		Name:          input.Name,
		LogoURL:       input.LogoURL,
		Description:   input.Description,
		TenantCode:    code,
		AllowRegister: s.allowRegisterDefault,
		Status:        StatusEnabled,
	})
}

func (s *Service) GetTenantCode(ctx context.Context, tenantID uint64) (string, error) {
	tenant, err := s.repo.FindByID(ctx, tenantID)
	if err != nil {
		return "", err
	}
	return tenant.TenantCode, nil
}

func (s *Service) ResetTenantCode(ctx context.Context, tenantID uint64) (Tenant, error) {
	if _, err := s.repo.FindByID(ctx, tenantID); err != nil {
		return Tenant{}, err
	}
	code, err := s.nextUniqueCode(ctx)
	if err != nil {
		return Tenant{}, err
	}
	return s.repo.UpdateTenantCode(ctx, tenantID, code)
}

func (s *Service) Enable(ctx context.Context, tenantID uint64) error {
	return s.repo.UpdateStatus(ctx, tenantID, StatusEnabled)
}

func (s *Service) Disable(ctx context.Context, tenantID uint64) error {
	return s.repo.UpdateStatus(ctx, tenantID, StatusDisabled)
}

func (s *Service) nextUniqueCode(ctx context.Context) (string, error) {
	if s.repo == nil {
		return "", ErrTenantRepositoryMiss
	}
	if s.codeGenerator == nil {
		return "", ErrTenantCodeGenerator
	}
	for range s.maxCodeGenerateAttempts {
		code, err := s.codeGenerator.NextCode()
		if err != nil {
			return "", err
		}
		exists, err := s.repo.TenantCodeExists(ctx, code)
		if err != nil {
			return "", err
		}
		if !exists {
			return code, nil
		}
	}
	return "", ErrTenantCodeCollision
}

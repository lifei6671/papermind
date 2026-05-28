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
	CreatedBy     uint64 // 创建租户的平台管理员用户 ID。
	UpdatedBy     uint64 // 最近更新租户的平台管理员用户 ID。
}

type InitialAdmin struct {
	Username     string // 首个租户管理员登录名。
	RealName     string // 首个租户管理员真实姓名。
	Phone        string // 首个租户管理员手机号。
	Email        string // 首个租户管理员邮箱。
	PasswordHash string // 首个租户管理员密码哈希。
}

type CreateInput struct {
	Name          string // 租户名称。
	LogoURL       string // 企业或机构 Logo 地址。
	Description   string // 企业或机构描述。
	AllowRegister *bool  // 是否允许该租户用户自注册；为空时继承平台默认值。
	ActorID       uint64 // 执行创建操作的平台管理员用户 ID。
	InitialAdmin  InitialAdmin
}

type ListInput struct {
	Page     int
	PageSize int
	Keyword  string
}

type UpdateProfileInput struct {
	TenantID    uint64
	Name        string
	LogoURL     string
	Description string
	ActorID     uint64
}

type ResetTenantCodeInput struct {
	TenantID uint64
	ActorID  uint64
}

type UpdateRegisterSettingInput struct {
	TenantID      uint64
	AllowRegister bool
	ActorID       uint64
}

type UpdateStatusInput struct {
	TenantID uint64
	ActorID  uint64
}

type Repository interface {
	List(ctx context.Context, input ListInput) (pagination.Result[Tenant], error)
	CreateWithAdmin(ctx context.Context, tenant Tenant, admin InitialAdmin) (Tenant, error)
	FindByID(ctx context.Context, tenantID uint64) (Tenant, error)
	TenantCodeExists(ctx context.Context, code string) (bool, error)
	UpdateTenantCode(ctx context.Context, tenantID uint64, code string, actorID uint64) (Tenant, error)
	UpdateProfile(ctx context.Context, input UpdateProfileInput) (Tenant, error)
	UpdateAllowRegister(ctx context.Context, tenantID uint64, allowRegister bool, actorID uint64) (Tenant, error)
	UpdateStatus(ctx context.Context, tenantID uint64, status string, actorID uint64) error
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
	codeGenerator := options.CodeGenerator
	if codeGenerator == nil {
		codeGenerator = NewRandomCodeGenerator()
	}
	return &Service{
		repo:                    options.Repo,
		codeGenerator:           codeGenerator,
		allowRegisterDefault:    options.AllowRegisterDefault,
		maxCodeGenerateAttempts: attempts,
	}
}

func (s *Service) List(ctx context.Context, input ListInput) (pagination.Result[Tenant], error) {
	return s.repo.List(ctx, input)
}

func (s *Service) Create(ctx context.Context, input CreateInput) (Tenant, error) {
	code, err := s.nextUniqueCode(ctx)
	if err != nil {
		return Tenant{}, err
	}
	allowRegister := s.allowRegisterDefault
	if input.AllowRegister != nil {
		allowRegister = *input.AllowRegister
	}
	return s.repo.CreateWithAdmin(ctx, Tenant{
		Name:          input.Name,
		LogoURL:       input.LogoURL,
		Description:   input.Description,
		TenantCode:    code,
		AllowRegister: allowRegister,
		Status:        StatusEnabled,
		CreatedBy:     input.ActorID,
		UpdatedBy:     input.ActorID,
	}, input.InitialAdmin)
}

func (s *Service) GetTenantCode(ctx context.Context, tenantID uint64) (string, error) {
	tenant, err := s.repo.FindByID(ctx, tenantID)
	if err != nil {
		return "", err
	}
	return tenant.TenantCode, nil
}

func (s *Service) ResetTenantCode(ctx context.Context, input ResetTenantCodeInput) (Tenant, error) {
	if _, err := s.repo.FindByID(ctx, input.TenantID); err != nil {
		return Tenant{}, err
	}
	code, err := s.nextUniqueCode(ctx)
	if err != nil {
		return Tenant{}, err
	}
	return s.repo.UpdateTenantCode(ctx, input.TenantID, code, input.ActorID)
}

func (s *Service) UpdateProfile(ctx context.Context, input UpdateProfileInput) (Tenant, error) {
	return s.repo.UpdateProfile(ctx, input)
}

func (s *Service) UpdateRegisterSetting(ctx context.Context, input UpdateRegisterSettingInput) (Tenant, error) {
	return s.repo.UpdateAllowRegister(ctx, input.TenantID, input.AllowRegister, input.ActorID)
}

func (s *Service) Enable(ctx context.Context, input UpdateStatusInput) error {
	return s.repo.UpdateStatus(ctx, input.TenantID, StatusEnabled, input.ActorID)
}

func (s *Service) Disable(ctx context.Context, input UpdateStatusInput) error {
	return s.repo.UpdateStatus(ctx, input.TenantID, StatusDisabled, input.ActorID)
}

func (s *Service) nextUniqueCode(ctx context.Context) (string, error) {
	if s.repo == nil {
		return "", ErrTenantRepositoryMiss
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

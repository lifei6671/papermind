package space

import (
	"context"
	"errors"

	"github.com/lifei6671/papermind/server/internal/service/pagination"
)

const (
	// StatusEnabled 表示空间或成员可正常使用。
	StatusEnabled = "enabled"
	// StatusDisabled 表示空间或成员已禁用。
	StatusDisabled = "disabled"

	// RoleSpaceAdmin 表示空间管理员。
	RoleSpaceAdmin = "space_admin"
	// RoleTeacher 表示教师。
	RoleTeacher = "teacher"
	// RoleStudent 表示学生。
	RoleStudent = "student"
)

var (
	ErrSpaceAdminRequired       = errors.New("space admin is required")
	ErrCannotLoseLastSpaceAdmin = errors.New("cannot lose last enabled space admin")
	ErrMemberNotFound           = errors.New("space member not found")
	ErrSpaceNotFound            = errors.New("space not found")
	ErrInvalidMemberRole        = errors.New("invalid space member role")
	ErrMemberUserUnavailable    = errors.New("space member user is unavailable")
)

type Space struct {
	ID          uint64 // 空间主键 ID。
	TenantID    uint64 // 所属租户 ID。
	Name        string // 空间名称，例如班级、专业、课程、培训项目。
	LogoURL     string // 空间 Logo 地址，可为空。
	Description string // 空间描述，可为空。
	Type        string // 空间类型：class / major / course / training / custom。
	Status      string // 空间状态：enabled / disabled。
}

type Member struct {
	ID       uint64 // 空间成员主键 ID。
	TenantID uint64 // 所属租户 ID。
	SpaceID  uint64 // 空间 ID。
	UserID   uint64 // 租户用户 ID。
	Role     string // 空间内角色：space_admin / teacher / student。
	Status   string // 空间成员状态：enabled / disabled。
}

type CreateInput struct {
	TenantID     uint64   // 所属租户 ID。
	Name         string   // 空间名称。
	LogoURL      string   // 空间 Logo 地址，可为空。
	Description  string   // 空间描述，可为空。
	Type         string   // 空间类型。
	AdminUserIDs []uint64 // 初始空间管理员用户 ID。
}

type ListInput struct {
	TenantID uint64
	Page     int
	PageSize int
}

type UpdateProfileInput struct {
	TenantID    uint64 // 所属租户 ID。
	SpaceID     uint64 // 空间 ID。
	Name        string // 空间名称。
	LogoURL     string // 空间 Logo 地址，可为空。
	Description string // 空间描述，可为空。
	Type        string // 空间类型。
}

type DeleteInput struct {
	TenantID uint64 // 所属租户 ID。
	SpaceID  uint64 // 空间 ID。
}

type JoinMemberInput struct {
	TenantID uint64 // 所属租户 ID。
	SpaceID  uint64 // 空间 ID。
	UserID   uint64 // 租户用户 ID。
	Role     string // 空间内角色。
}

type MemberActionInput struct {
	TenantID uint64 // 所属租户 ID。
	SpaceID  uint64 // 空间 ID。
	UserID   uint64 // 租户用户 ID。
}

type ChangeRoleInput struct {
	TenantID uint64 // 所属租户 ID。
	SpaceID  uint64 // 空间 ID。
	UserID   uint64 // 租户用户 ID。
	Role     string // 目标空间内角色。
}

type Repository interface {
	ListSpaces(ctx context.Context, tenantID uint64, page pagination.Input) (pagination.Result[Space], error)
	CreateSpace(ctx context.Context, space Space, adminUserIDs []uint64) (Space, error)
	UpdateSpaceProfile(ctx context.Context, input UpdateProfileInput) (Space, error)
	DeleteSpace(ctx context.Context, tenantID uint64, spaceID uint64) error
	AddMember(ctx context.Context, member Member) (Member, error)
	ListEffectiveMembers(ctx context.Context, tenantID uint64, spaceID uint64) ([]Member, error)
	ListEffectiveMembershipsForUser(ctx context.Context, tenantID uint64, userID uint64) ([]Member, error)
	FindMember(ctx context.Context, tenantID uint64, spaceID uint64, userID uint64) (Member, error)
	CountEnabledSpaceAdmins(ctx context.Context, tenantID uint64, spaceID uint64) (int64, error)
	DisableMember(ctx context.Context, tenantID uint64, spaceID uint64, userID uint64) error
	RemoveMember(ctx context.Context, tenantID uint64, spaceID uint64, userID uint64) error
	UpdateMemberRole(ctx context.Context, tenantID uint64, spaceID uint64, userID uint64, role string) error
}

type ServiceOptions struct {
	Repo Repository
}

type Service struct {
	repo Repository
}

func NewService(options ServiceOptions) *Service {
	return &Service{repo: options.Repo}
}

func (s *Service) List(ctx context.Context, input ListInput) (pagination.Result[Space], error) {
	return s.repo.ListSpaces(ctx, input.TenantID, pagination.Input{Page: input.Page, PageSize: input.PageSize})
}

func (s *Service) Create(ctx context.Context, input CreateInput) (Space, error) {
	if len(input.AdminUserIDs) == 0 {
		return Space{}, ErrSpaceAdminRequired
	}
	return s.repo.CreateSpace(ctx, Space{
		TenantID:    input.TenantID,
		Name:        input.Name,
		LogoURL:     input.LogoURL,
		Description: input.Description,
		Type:        input.Type,
		Status:      StatusEnabled,
	}, input.AdminUserIDs)
}

func (s *Service) UpdateProfile(ctx context.Context, input UpdateProfileInput) (Space, error) {
	return s.repo.UpdateSpaceProfile(ctx, input)
}

func (s *Service) Delete(ctx context.Context, input DeleteInput) error {
	return s.repo.DeleteSpace(ctx, input.TenantID, input.SpaceID)
}

func (s *Service) JoinMember(ctx context.Context, input JoinMemberInput) (Member, error) {
	if !validMemberRole(input.Role) {
		return Member{}, ErrInvalidMemberRole
	}
	return s.repo.AddMember(ctx, Member{
		TenantID: input.TenantID,
		SpaceID:  input.SpaceID,
		UserID:   input.UserID,
		Role:     input.Role,
		Status:   StatusEnabled,
	})
}

func (s *Service) ListEffectiveMembers(ctx context.Context, tenantID uint64, spaceID uint64) ([]Member, error) {
	return s.repo.ListEffectiveMembers(ctx, tenantID, spaceID)
}

func (s *Service) ListEffectiveMembershipsForUser(ctx context.Context, tenantID uint64, userID uint64) ([]Member, error) {
	return s.repo.ListEffectiveMembershipsForUser(ctx, tenantID, userID)
}

func (s *Service) DisableMember(ctx context.Context, input MemberActionInput) error {
	if err := s.ValidateSpaceAdminInvariant(ctx, input.TenantID, input.SpaceID, input.UserID); err != nil {
		return err
	}
	return s.repo.DisableMember(ctx, input.TenantID, input.SpaceID, input.UserID)
}

func (s *Service) RemoveMember(ctx context.Context, input MemberActionInput) error {
	if err := s.ValidateSpaceAdminInvariant(ctx, input.TenantID, input.SpaceID, input.UserID); err != nil {
		return err
	}
	return s.repo.RemoveMember(ctx, input.TenantID, input.SpaceID, input.UserID)
}

func (s *Service) ChangeMemberRole(ctx context.Context, input ChangeRoleInput) error {
	if !validMemberRole(input.Role) {
		return ErrInvalidMemberRole
	}
	if input.Role != RoleSpaceAdmin {
		if err := s.ValidateSpaceAdminInvariant(ctx, input.TenantID, input.SpaceID, input.UserID); err != nil {
			return err
		}
	}
	return s.repo.UpdateMemberRole(ctx, input.TenantID, input.SpaceID, input.UserID, input.Role)
}

func (s *Service) ValidateSpaceAdminInvariant(ctx context.Context, tenantID uint64, spaceID uint64, userID uint64) error {
	member, err := s.repo.FindMember(ctx, tenantID, spaceID, userID)
	if err != nil {
		return err
	}
	if member.Role != RoleSpaceAdmin || member.Status != StatusEnabled {
		return nil
	}
	count, err := s.repo.CountEnabledSpaceAdmins(ctx, tenantID, spaceID)
	if err != nil {
		return err
	}
	if count == 1 {
		return ErrCannotLoseLastSpaceAdmin
	}
	return nil
}

func validMemberRole(role string) bool {
	return role == RoleSpaceAdmin || role == RoleTeacher || role == RoleStudent
}

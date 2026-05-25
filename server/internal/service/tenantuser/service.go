package tenantuser

import (
	"context"
	"errors"
	"time"
)

const (
	// StatusEnabled 表示租户或用户可正常使用。
	StatusEnabled = "enabled"
	// StatusDisabled 表示租户或用户已被禁用。
	StatusDisabled = "disabled"

	// LoginFailureInvalidCredential 表示登录名或密码不正确。
	LoginFailureInvalidCredential = "invalid_credential"
	// LoginFailureDisabled 表示账号已被禁用。
	LoginFailureDisabled = "disabled"
)

var (
	ErrTenantNotFound        = errors.New("tenant not found")
	ErrTenantDisabled        = errors.New("tenant disabled")
	ErrRegisterNotAllowed    = errors.New("tenant register not allowed")
	ErrUserNotFound          = errors.New("tenant user not found")
	ErrInvalidCredential     = errors.New("invalid tenant user credential")
	ErrUserDisabled          = errors.New("tenant user disabled")
	ErrUnsupportedAvatarType = errors.New("unsupported avatar content type")
	ErrAvatarTooLarge        = errors.New("avatar file too large")
	ErrCannotDisableSelf     = errors.New("cannot disable self")
)

type Tenant struct {
	ID            uint64 // 租户主键 ID。
	TenantCode    string // 租户码，用于专属注册链接和手动注册归属。
	AllowRegister bool   // 是否允许该租户用户自注册。
	Status        string // 租户状态：enabled / disabled。
}

type User struct {
	ID           uint64 // 租户用户主键 ID。
	TenantID     uint64 // 所属租户 ID。
	Username     string // 租户内登录名。
	RealName     string // 真实姓名，用于阅卷、成绩单和导出。
	AvatarURL    string // 用户头像地址。
	Phone        string // 手机号，可用于登录或通知。
	Email        string // 邮箱，可用于登录或通知。
	PasswordHash string // 密码哈希。
	LastLoginIP  string // 最后登录 IP。
	LastLoginAt  int64  // 最后登录时间，Unix 毫秒时间戳。
	Status       string // 用户状态：enabled / disabled。
}

type RegisterInput struct {
	TenantCode   string // 租户码，来自注册链接或手动输入。
	Username     string // 租户内登录名。
	RealName     string // 真实姓名。
	PasswordHash string // 密码哈希。
	Phone        string // 手机号。
	Email        string // 邮箱。
}

type LoginInput struct {
	TenantID uint64 // 所属租户 ID。
	Username string // 租户内登录名。
	Password string // 登录明文密码。
	IP       string // 登录来源 IP。
}

type UploadAvatarInput struct {
	TenantID    uint64 // 所属租户 ID。
	UserID      uint64 // 租户用户 ID。
	FileName    string // 上传文件名。
	ContentType string // 上传文件 MIME 类型。
	Size        int64  // 上传文件大小，单位字节。
}

type AvatarObjectInput struct {
	TenantID    uint64 // 所属租户 ID。
	UserID      uint64 // 租户用户 ID。
	FileName    string // 上传文件名。
	ContentType string // 上传文件 MIME 类型。
	Size        int64  // 上传文件大小，单位字节。
}

type PreviewDisableInput struct {
	TenantID uint64 // 所属租户 ID。
	UserID   uint64 // 将被禁用的租户用户 ID。
}

type DisableInput struct {
	TenantID uint64 // 所属租户 ID。
	ActorID  uint64 // 执行禁用操作的用户 ID。
	TargetID uint64 // 将被禁用的用户 ID。
}

type DisableImpact struct {
	LoseLogin       bool     // 禁用后失去登录能力。
	LoseExamAccess  bool     // 禁用后失去考试能力。
	LoseGradeAccess bool     // 禁用后失去阅卷能力。
	AdminSpaceIDs   []uint64 // 该用户作为空间管理员的空间 ID。
}

type Repository interface {
	FindTenantByCode(ctx context.Context, tenantCode string) (Tenant, error)
	CreateUser(ctx context.Context, user User) (User, error)
	FindUserByUsername(ctx context.Context, tenantID uint64, username string) (User, error)
	UpdateLoginAudit(ctx context.Context, tenantID uint64, userID uint64, ip string, at int64) error
	UpdateAvatarURL(ctx context.Context, tenantID uint64, userID uint64, url string) error
	BuildDisableImpact(ctx context.Context, tenantID uint64, userID uint64) (DisableImpact, error)
	UpdateStatus(ctx context.Context, tenantID uint64, userID uint64, status string) error
}

type PasswordVerifier interface {
	Verify(hash string, password string) bool
}

type SecurityLogger interface {
	LoginFailed(ctx context.Context, tenantID uint64, username string, ip string, reason string) error
}

type AvatarStorage interface {
	SaveTenantUserAvatar(ctx context.Context, input AvatarObjectInput) (string, error)
}

type SpaceAdminInvariantChecker interface {
	ValidateBeforeDisableTenantUser(ctx context.Context, tenantID uint64, userID uint64) error
}

type ServiceOptions struct {
	Repo                       Repository
	PasswordVerifier           PasswordVerifier
	SecurityLogger             SecurityLogger
	AvatarStorage              AvatarStorage
	SpaceAdminInvariantChecker SpaceAdminInvariantChecker
	MaxAvatarSize              int64
	AllowedAvatarContentTypes  []string
	Now                        func() int64
}

type Service struct {
	repo                       Repository
	passwordVerifier           PasswordVerifier
	securityLogger             SecurityLogger
	avatarStorage              AvatarStorage
	spaceAdminInvariantChecker SpaceAdminInvariantChecker
	maxAvatarSize              int64
	allowedAvatarContentTypes  []string
	now                        func() int64
}

func NewService(options ServiceOptions) *Service {
	now := options.Now
	if now == nil {
		now = func() int64 { return time.Now().UnixMilli() }
	}
	return &Service{
		repo:                       options.Repo,
		passwordVerifier:           options.PasswordVerifier,
		securityLogger:             options.SecurityLogger,
		avatarStorage:              options.AvatarStorage,
		spaceAdminInvariantChecker: options.SpaceAdminInvariantChecker,
		maxAvatarSize:              options.MaxAvatarSize,
		allowedAvatarContentTypes:  options.AllowedAvatarContentTypes,
		now:                        now,
	}
}

func (s *Service) RegisterWithTenantCode(ctx context.Context, input RegisterInput) (User, error) {
	return s.register(ctx, input)
}

func (s *Service) RegisterWithTenantLink(ctx context.Context, input RegisterInput) (User, error) {
	return s.register(ctx, input)
}

func (s *Service) Login(ctx context.Context, input LoginInput) (User, error) {
	user, err := s.repo.FindUserByUsername(ctx, input.TenantID, input.Username)
	if err != nil {
		if logErr := s.securityLogger.LoginFailed(ctx, input.TenantID, input.Username, input.IP, LoginFailureInvalidCredential); logErr != nil {
			return User{}, logErr
		}
		return User{}, ErrInvalidCredential
	}
	if user.Status != StatusEnabled {
		if logErr := s.securityLogger.LoginFailed(ctx, input.TenantID, input.Username, input.IP, LoginFailureDisabled); logErr != nil {
			return User{}, logErr
		}
		return User{}, ErrUserDisabled
	}
	if !s.passwordVerifier.Verify(user.PasswordHash, input.Password) {
		if logErr := s.securityLogger.LoginFailed(ctx, input.TenantID, input.Username, input.IP, LoginFailureInvalidCredential); logErr != nil {
			return User{}, logErr
		}
		return User{}, ErrInvalidCredential
	}
	now := s.now()
	if err := s.repo.UpdateLoginAudit(ctx, input.TenantID, user.ID, input.IP, now); err != nil {
		return User{}, err
	}
	user.LastLoginIP = input.IP
	user.LastLoginAt = now
	return user, nil
}

func (s *Service) UploadAvatar(ctx context.Context, input UploadAvatarInput) (string, error) {
	if !s.isAllowedAvatarContentType(input.ContentType) {
		return "", ErrUnsupportedAvatarType
	}
	if input.Size > s.maxAvatarSize {
		return "", ErrAvatarTooLarge
	}
	url, err := s.avatarStorage.SaveTenantUserAvatar(ctx, AvatarObjectInput{
		TenantID:    input.TenantID,
		UserID:      input.UserID,
		FileName:    input.FileName,
		ContentType: input.ContentType,
		Size:        input.Size,
	})
	if err != nil {
		return "", err
	}
	if err := s.repo.UpdateAvatarURL(ctx, input.TenantID, input.UserID, url); err != nil {
		return "", err
	}
	return url, nil
}

func (s *Service) PreviewDisable(ctx context.Context, input PreviewDisableInput) (DisableImpact, error) {
	return s.repo.BuildDisableImpact(ctx, input.TenantID, input.UserID)
}

func (s *Service) Disable(ctx context.Context, input DisableInput) error {
	if input.ActorID == input.TargetID {
		return ErrCannotDisableSelf
	}
	if err := s.spaceAdminInvariantChecker.ValidateBeforeDisableTenantUser(ctx, input.TenantID, input.TargetID); err != nil {
		return err
	}
	return s.repo.UpdateStatus(ctx, input.TenantID, input.TargetID, StatusDisabled)
}

func (s *Service) register(ctx context.Context, input RegisterInput) (User, error) {
	tenant, err := s.repo.FindTenantByCode(ctx, input.TenantCode)
	if err != nil {
		return User{}, err
	}
	if tenant.Status != StatusEnabled {
		return User{}, ErrTenantDisabled
	}
	if !tenant.AllowRegister {
		return User{}, ErrRegisterNotAllowed
	}
	return s.repo.CreateUser(ctx, User{
		TenantID:     tenant.ID,
		Username:     input.Username,
		RealName:     input.RealName,
		PasswordHash: input.PasswordHash,
		Phone:        input.Phone,
		Email:        input.Email,
		Status:       StatusEnabled,
	})
}

func (s *Service) isAllowedAvatarContentType(contentType string) bool {
	for _, allowed := range s.allowedAvatarContentTypes {
		if contentType == allowed {
			return true
		}
	}
	return false
}

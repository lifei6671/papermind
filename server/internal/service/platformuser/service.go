package platformuser

import (
	"context"
	"crypto/subtle"
	"errors"
	"time"
)

const (
	// StatusEnabled 表示平台管理员账号可登录、可操作。
	StatusEnabled = "enabled"
	// StatusDisabled 表示平台管理员账号已禁用。
	StatusDisabled = "disabled"

	// LoginFailureInvalidCredential 表示登录名或密码不正确。
	LoginFailureInvalidCredential = "invalid_credential"
	// LoginFailureDisabled 表示账号已被禁用。
	LoginFailureDisabled = "disabled"
)

var (
	ErrInvalidCredential      = errors.New("invalid platform user credential")
	ErrPlatformUserDisabled   = errors.New("platform user disabled")
	ErrPlatformUserNotFound   = errors.New("platform user not found")
	ErrCannotDisableSelf      = errors.New("cannot disable self")
	ErrCannotDisableLastAdmin = errors.New("cannot disable last enabled platform admin")
	ErrUnsupportedAvatarType  = errors.New("unsupported avatar content type")
	ErrAvatarTooLarge         = errors.New("avatar file too large")
)

type PlatformUser struct {
	ID           uint64 // 平台管理员主键 ID。
	Username     string // 平台管理员登录名。
	AvatarURL    string // 用户头像地址。
	PasswordHash string // 密码哈希。
	LastLoginIP  string // 最后登录 IP。
	LastLoginAt  int64  // 最后登录时间，Unix 毫秒时间戳。
	Status       string // 平台管理员状态：enabled / disabled。
}

type SeedAdminInput struct {
	Username     string // 初始平台管理员登录名。
	PasswordHash string // 初始平台管理员密码哈希。
}

type LoginInput struct {
	Username string // 平台管理员登录名。
	Password string // 登录明文密码。
	IP       string // 登录来源 IP。
}

type DisableInput struct {
	ActorID  uint64 // 执行禁用操作的平台管理员 ID。
	TargetID uint64 // 被禁用的平台管理员 ID。
}

type UploadAvatarInput struct {
	UserID      uint64 // 平台管理员 ID。
	FileName    string // 上传文件名。
	ContentType string // 上传文件 MIME 类型。
	Size        int64  // 上传文件大小，单位字节。
}

type AvatarObjectInput struct {
	UserID      uint64 // 平台管理员 ID。
	FileName    string // 上传文件名。
	ContentType string // 上传文件 MIME 类型。
	Size        int64  // 上传文件大小，单位字节。
}

type Repository interface {
	HasAny(ctx context.Context) (bool, error)
	Create(ctx context.Context, user PlatformUser) (PlatformUser, error)
	FindByUsername(ctx context.Context, username string) (PlatformUser, error)
	FindByID(ctx context.Context, userID uint64) (PlatformUser, error)
	CountEnabledAdmins(ctx context.Context) (int64, error)
	UpdateLoginAudit(ctx context.Context, userID uint64, ip string, at int64) error
	UpdateStatus(ctx context.Context, userID uint64, status string) error
	UpdateAvatarURL(ctx context.Context, userID uint64, url string) error
}

type PasswordVerifier interface {
	Verify(hash string, password string) bool
}

type SecurityLogger interface {
	LoginFailed(ctx context.Context, username string, ip string, reason string) error
}

type AvatarStorage interface {
	SavePlatformUserAvatar(ctx context.Context, input AvatarObjectInput) (string, error)
}

type ServiceOptions struct {
	Repo                      Repository
	PasswordVerifier          PasswordVerifier
	SecurityLogger            SecurityLogger
	AvatarStorage             AvatarStorage
	MaxAvatarSize             int64
	AllowedAvatarContentTypes []string
	Now                       func() int64
}

type Service struct {
	repo                      Repository
	passwordVerifier          PasswordVerifier
	securityLogger            SecurityLogger
	avatarStorage             AvatarStorage
	maxAvatarSize             int64
	allowedAvatarContentTypes []string
	now                       func() int64
}

func NewService(options ServiceOptions) *Service {
	now := options.Now
	if now == nil {
		now = func() int64 { return time.Now().UnixMilli() }
	}
	passwordVerifier := options.PasswordVerifier
	if passwordVerifier == nil {
		passwordVerifier = plainPasswordVerifier{}
	}
	securityLogger := options.SecurityLogger
	if securityLogger == nil {
		securityLogger = noopSecurityLogger{}
	}
	return &Service{
		repo:                      options.Repo,
		passwordVerifier:          passwordVerifier,
		securityLogger:            securityLogger,
		avatarStorage:             options.AvatarStorage,
		maxAvatarSize:             options.MaxAvatarSize,
		allowedAvatarContentTypes: options.AllowedAvatarContentTypes,
		now:                       now,
	}
}

func (s *Service) SeedAdmin(ctx context.Context, input SeedAdminInput) (PlatformUser, error) {
	exists, err := s.repo.HasAny(ctx)
	if err != nil {
		return PlatformUser{}, err
	}
	if exists {
		return PlatformUser{}, nil
	}
	return s.repo.Create(ctx, PlatformUser{
		Username:     input.Username,
		PasswordHash: input.PasswordHash,
		Status:       StatusEnabled,
	})
}

func (s *Service) Login(ctx context.Context, input LoginInput) (PlatformUser, error) {
	user, err := s.repo.FindByUsername(ctx, input.Username)
	if err != nil {
		if logErr := s.securityLogger.LoginFailed(ctx, input.Username, input.IP, LoginFailureInvalidCredential); logErr != nil {
			return PlatformUser{}, logErr
		}
		return PlatformUser{}, ErrInvalidCredential
	}
	if user.Status != StatusEnabled {
		if logErr := s.securityLogger.LoginFailed(ctx, input.Username, input.IP, LoginFailureDisabled); logErr != nil {
			return PlatformUser{}, logErr
		}
		return PlatformUser{}, ErrPlatformUserDisabled
	}
	if !s.passwordVerifier.Verify(user.PasswordHash, input.Password) {
		if logErr := s.securityLogger.LoginFailed(ctx, input.Username, input.IP, LoginFailureInvalidCredential); logErr != nil {
			return PlatformUser{}, logErr
		}
		return PlatformUser{}, ErrInvalidCredential
	}
	now := s.now()
	if err := s.repo.UpdateLoginAudit(ctx, user.ID, input.IP, now); err != nil {
		return PlatformUser{}, err
	}
	user.LastLoginIP = input.IP
	user.LastLoginAt = now
	return user, nil
}

func (s *Service) Disable(ctx context.Context, input DisableInput) error {
	if input.ActorID == input.TargetID {
		return ErrCannotDisableSelf
	}
	target, err := s.repo.FindByID(ctx, input.TargetID)
	if err != nil {
		return err
	}
	if target.Status == StatusEnabled {
		count, err := s.repo.CountEnabledAdmins(ctx)
		if err != nil {
			return err
		}
		if count <= 1 {
			return ErrCannotDisableLastAdmin
		}
	}
	return s.repo.UpdateStatus(ctx, input.TargetID, StatusDisabled)
}

func (s *Service) UploadAvatar(ctx context.Context, input UploadAvatarInput) (string, error) {
	if !s.isAllowedAvatarContentType(input.ContentType) {
		return "", ErrUnsupportedAvatarType
	}
	if input.Size > s.maxAvatarSize {
		return "", ErrAvatarTooLarge
	}
	url, err := s.avatarStorage.SavePlatformUserAvatar(ctx, AvatarObjectInput{
		UserID:      input.UserID,
		FileName:    input.FileName,
		ContentType: input.ContentType,
		Size:        input.Size,
	})
	if err != nil {
		return "", err
	}
	if err := s.repo.UpdateAvatarURL(ctx, input.UserID, url); err != nil {
		return "", err
	}
	return url, nil
}

func (s *Service) isAllowedAvatarContentType(contentType string) bool {
	for _, allowed := range s.allowedAvatarContentTypes {
		if contentType == allowed {
			return true
		}
	}
	return false
}

type plainPasswordVerifier struct{}

func (plainPasswordVerifier) Verify(hash string, password string) bool {
	// 首版 seed 只保存开发环境口令字面量；这里用常量时间比较，后续接入专用哈希器时替换该默认实现。
	return subtle.ConstantTimeCompare([]byte(hash), []byte(password)) == 1
}

type noopSecurityLogger struct{}

func (noopSecurityLogger) LoginFailed(ctx context.Context, username string, ip string, reason string) error {
	return nil
}

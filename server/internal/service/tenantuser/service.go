package tenantuser

import (
	"context"
	"errors"
	"time"

	"github.com/lifei6671/papermind/server/internal/service/pagination"
	"github.com/lifei6671/papermind/server/library/crypto"
)

const (
	// StatusEnabled 表示租户或用户可正常使用。
	StatusEnabled = "enabled"
	// StatusDisabled 表示租户或用户已被禁用。
	StatusDisabled = "disabled"

	// RoleTenantAdmin 表示租户管理员。
	RoleTenantAdmin = "tenant_admin"
	// RoleTenantUser 表示已登录但尚未进入具体租户上下文的全局租户账号。
	RoleTenantUser = "tenant_user"
	// RoleTeacher 表示教师。
	RoleTeacher = "teacher"
	// RoleStudent 表示学生。
	RoleStudent = "student"

	// LoginFailureInvalidCredential 表示登录名或密码不正确。
	LoginFailureInvalidCredential = "invalid_credential"
	// LoginFailureDisabled 表示账号已被禁用。
	LoginFailureDisabled = "disabled"
)

var (
	ErrTenantNotFound            = errors.New("tenant not found")
	ErrTenantDisabled            = errors.New("tenant disabled")
	ErrRegisterNotAllowed        = errors.New("tenant register not allowed")
	ErrUserNotFound              = errors.New("tenant user not found")
	ErrDisplayNameRequired       = errors.New("display name required")
	ErrInvalidCredential         = errors.New("invalid tenant user credential")
	ErrUserDisabled              = errors.New("tenant user disabled")
	ErrUnsupportedAvatarType     = errors.New("unsupported avatar content type")
	ErrAvatarTooLarge            = errors.New("avatar file too large")
	ErrCannotDisableSelf         = errors.New("cannot disable self")
	ErrCannotChangeSelfRole      = errors.New("cannot change self role")
	ErrInvalidRole               = errors.New("invalid tenant user role")
	ErrInvalidImportRow          = errors.New("invalid tenant user import row")
	ErrCannotLoseLastTenantAdmin = errors.New("cannot lose last enabled tenant admin")
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
	Role         string // 租户固定角色：tenant_admin / teacher / student。
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

type CreateInput struct {
	TenantID     uint64 // 所属租户 ID。
	Username     string // 租户内登录名。
	RealName     string // 真实姓名。
	AvatarURL    string // 用户头像地址。
	PasswordHash string // 密码哈希。
	Phone        string // 手机号。
	Email        string // 邮箱。
	Role         string // 租户固定角色。
}

type ListInput struct {
	TenantID uint64
	Page     int
	PageSize int
}

type LoginInput struct {
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

type EnableInput struct {
	TenantID uint64 // 所属租户 ID。
	ActorID  uint64 // 执行启用操作的用户 ID。
	TargetID uint64 // 将被启用的用户 ID。
}

type DeleteInput struct {
	TenantID uint64 // 所属租户 ID。
	ActorID  uint64 // 执行删除操作的用户 ID。
	TargetID uint64 // 将被删除的用户 ID。
}

type UpdateRoleInput struct {
	TenantID uint64 // 所属租户 ID。
	ActorID  uint64 // 执行角色变更的用户 ID。
	TargetID uint64 // 将被变更角色的用户 ID。
	Role     string // 新租户固定角色。
}

type ImportUsersInput struct {
	TenantID uint64          // 所属租户 ID。
	ActorID  uint64          // 执行批量导入的租户用户 ID。
	Rows     []ImportUserRow // 导入用户行。
}

type ImportUserRow struct {
	Username  string // 租户内登录名。
	RealName  string // 真实姓名。
	AvatarURL string // 用户头像地址。
	Password  string // 初始或覆盖密码明文。
	Role      string // 租户固定角色。
}

type ImportUsersRepositoryInput struct {
	TenantID uint64                     // 所属租户 ID。
	ActorID  uint64                     // 执行批量导入的租户用户 ID。
	Rows     []ImportUsersRepositoryRow // 已完成密码哈希的导入用户行。
}

type ImportUsersRepositoryRow struct {
	Username     string // 租户内登录名。
	RealName     string // 真实姓名。
	AvatarURL    string // 用户头像地址。
	PasswordHash string // 密码哈希。
	Role         string // 租户固定角色。
}

type ImportUsersResult struct {
	SuccessCount int // 成功导入行数。
}

type UpdateProfileInput struct {
	TenantID    uint64 // 所属租户 ID。
	UserID      uint64 // 当前租户用户 ID。
	DisplayName string // 真实姓名。
	AvatarURL   string // 用户头像地址。
	Phone       string // 手机号。
	Email       string // 邮箱。
}

type DisableImpact struct {
	LoseLogin       bool     // 禁用后失去登录能力。
	LoseExamAccess  bool     // 禁用后失去考试能力。
	LoseGradeAccess bool     // 禁用后失去阅卷能力。
	AdminSpaceIDs   []uint64 // 该用户作为空间管理员的空间 ID。
}

type Repository interface {
	ListUsers(ctx context.Context, tenantID uint64, page pagination.Input) (pagination.Result[User], error)
	FindUserByID(ctx context.Context, tenantID uint64, userID uint64) (User, error)
	FindGlobalUserByID(ctx context.Context, userID uint64) (User, error)
	FindTenantByCode(ctx context.Context, tenantCode string) (Tenant, error)
	CreateUser(ctx context.Context, user User) (User, error)
	CreateUserRole(ctx context.Context, tenantID uint64, userID uint64, role string) error
	CreateUserWithRole(ctx context.Context, user User, role string) (User, error)
	FindUserByUsername(ctx context.Context, tenantID uint64, username string) (User, error)
	FindGlobalUserByUsername(ctx context.Context, username string) (User, error)
	UpdateLoginAudit(ctx context.Context, tenantID uint64, userID uint64, ip string, at int64) error
	UpdateProfile(ctx context.Context, input UpdateProfileInput) (User, error)
	UpdateAvatarURL(ctx context.Context, tenantID uint64, userID uint64, url string) error
	BuildDisableImpact(ctx context.Context, tenantID uint64, userID uint64) (DisableImpact, error)
	UpdateStatus(ctx context.Context, tenantID uint64, userID uint64, status string) error
	DeleteUser(ctx context.Context, tenantID uint64, userID uint64) error
	UpdateRole(ctx context.Context, tenantID uint64, userID uint64, role string) error
	ImportUsers(ctx context.Context, input ImportUsersRepositoryInput) (ImportUsersResult, error)
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
	passwordVerifier := options.PasswordVerifier
	if passwordVerifier == nil {
		passwordVerifier = defaultPasswordVerifier{}
	}
	securityLogger := options.SecurityLogger
	if securityLogger == nil {
		securityLogger = noopSecurityLogger{}
	}
	return &Service{
		repo:                       options.Repo,
		passwordVerifier:           passwordVerifier,
		securityLogger:             securityLogger,
		avatarStorage:              options.AvatarStorage,
		spaceAdminInvariantChecker: options.SpaceAdminInvariantChecker,
		maxAvatarSize:              options.MaxAvatarSize,
		allowedAvatarContentTypes:  options.AllowedAvatarContentTypes,
		now:                        now,
	}
}

func (s *Service) List(ctx context.Context, input ListInput) (pagination.Result[User], error) {
	return s.repo.ListUsers(ctx, input.TenantID, pagination.Input{Page: input.Page, PageSize: input.PageSize})
}

func (s *Service) Get(ctx context.Context, tenantID uint64, userID uint64) (User, error) {
	return s.repo.FindUserByID(ctx, tenantID, userID)
}

func (s *Service) GetGlobal(ctx context.Context, userID uint64) (User, error) {
	return s.repo.FindGlobalUserByID(ctx, userID)
}

func (s *Service) Create(ctx context.Context, input CreateInput) (User, error) {
	role := input.Role
	if role == "" {
		role = RoleStudent
	}
	user, err := s.repo.CreateUserWithRole(ctx, User{
		TenantID:     input.TenantID,
		Username:     input.Username,
		RealName:     input.RealName,
		AvatarURL:    input.AvatarURL,
		PasswordHash: input.PasswordHash,
		Phone:        input.Phone,
		Email:        input.Email,
		Role:         role,
		Status:       StatusEnabled,
	}, role)
	if err != nil {
		return User{}, err
	}
	user.Role = role
	return user, nil
}

func (s *Service) RegisterWithTenantCode(ctx context.Context, input RegisterInput) (User, error) {
	return s.register(ctx, input)
}

func (s *Service) RegisterWithTenantLink(ctx context.Context, input RegisterInput) (User, error) {
	return s.register(ctx, input)
}

func (s *Service) Login(ctx context.Context, input LoginInput) (User, error) {
	user, err := s.repo.FindGlobalUserByUsername(ctx, input.Username)
	if err != nil {
		if logErr := s.securityLogger.LoginFailed(ctx, 0, input.Username, input.IP, LoginFailureInvalidCredential); logErr != nil {
			return User{}, logErr
		}
		return User{}, ErrInvalidCredential
	}
	if user.Status != StatusEnabled {
		if logErr := s.securityLogger.LoginFailed(ctx, 0, input.Username, input.IP, LoginFailureDisabled); logErr != nil {
			return User{}, logErr
		}
		return User{}, ErrUserDisabled
	}
	if !s.passwordVerifier.Verify(user.PasswordHash, input.Password) {
		if logErr := s.securityLogger.LoginFailed(ctx, 0, input.Username, input.IP, LoginFailureInvalidCredential); logErr != nil {
			return User{}, logErr
		}
		return User{}, ErrInvalidCredential
	}
	now := s.now()
	if err := s.repo.UpdateLoginAudit(ctx, 0, user.ID, input.IP, now); err != nil {
		return User{}, err
	}
	user.LastLoginIP = input.IP
	user.LastLoginAt = now
	user.Role = RoleTenantUser
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

func (s *Service) Enable(ctx context.Context, input EnableInput) error {
	return s.repo.UpdateStatus(ctx, input.TenantID, input.TargetID, StatusEnabled)
}

func (s *Service) Delete(ctx context.Context, input DeleteInput) error {
	if input.ActorID == input.TargetID {
		return ErrCannotDisableSelf
	}
	if err := s.spaceAdminInvariantChecker.ValidateBeforeDisableTenantUser(ctx, input.TenantID, input.TargetID); err != nil {
		return err
	}
	return s.repo.DeleteUser(ctx, input.TenantID, input.TargetID)
}

func (s *Service) UpdateRole(ctx context.Context, input UpdateRoleInput) error {
	if input.ActorID == input.TargetID {
		return ErrCannotChangeSelfRole
	}
	if !validTenantUserRole(input.Role) {
		return ErrInvalidRole
	}
	return s.repo.UpdateRole(ctx, input.TenantID, input.TargetID, input.Role)
}

func (s *Service) ImportUsers(ctx context.Context, input ImportUsersInput) (ImportUsersResult, error) {
	rows := make([]ImportUsersRepositoryRow, 0, len(input.Rows))
	for _, row := range input.Rows {
		if row.Username == "" || row.RealName == "" || row.Password == "" {
			return ImportUsersResult{}, ErrInvalidImportRow
		}
		if !validTenantUserRole(row.Role) {
			return ImportUsersResult{}, ErrInvalidRole
		}
		passwordHash, err := crypto.HashPassword(row.Password)
		if err != nil {
			return ImportUsersResult{}, err
		}
		rows = append(rows, ImportUsersRepositoryRow{
			Username:     row.Username,
			RealName:     row.RealName,
			AvatarURL:    row.AvatarURL,
			PasswordHash: passwordHash,
			Role:         row.Role,
		})
	}
	return s.repo.ImportUsers(ctx, ImportUsersRepositoryInput{
		TenantID: input.TenantID,
		ActorID:  input.ActorID,
		Rows:     rows,
	})
}

func (s *Service) UpdateProfile(ctx context.Context, input UpdateProfileInput) (User, error) {
	if input.DisplayName == "" {
		return User{}, ErrDisplayNameRequired
	}
	return s.repo.UpdateProfile(ctx, input)
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
	user, err := s.repo.CreateUserWithRole(ctx, User{
		TenantID:     tenant.ID,
		Username:     input.Username,
		RealName:     input.RealName,
		PasswordHash: input.PasswordHash,
		Phone:        input.Phone,
		Email:        input.Email,
		Role:         RoleStudent,
		Status:       StatusEnabled,
	}, RoleStudent)
	if err != nil {
		return User{}, err
	}
	user.Role = RoleStudent
	return user, nil
}

func (s *Service) isAllowedAvatarContentType(contentType string) bool {
	for _, allowed := range s.allowedAvatarContentTypes {
		if contentType == allowed {
			return true
		}
	}
	return false
}

func validTenantUserRole(role string) bool {
	return role == RoleTenantAdmin || role == RoleTeacher || role == RoleStudent
}

type defaultPasswordVerifier struct{}

func (defaultPasswordVerifier) Verify(hash string, password string) bool {
	return crypto.VerifyPassword(hash, password)
}

type noopSecurityLogger struct{}

func (noopSecurityLogger) LoginFailed(ctx context.Context, tenantID uint64, username string, ip string, reason string) error {
	return nil
}

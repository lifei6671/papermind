package tenantuser

import (
	"context"
	"errors"
	"testing"

	"github.com/lifei6671/papermind/server/internal/service/pagination"
)

func TestRegisterWithTenantCodeCreatesEnabledUserWithoutSpaceMembership(t *testing.T) {
	repo := &fakeRepository{
		tenantsByCode: map[string]Tenant{
			"TENANT001": {ID: 10, TenantCode: "TENANT001", AllowRegister: true, Status: StatusEnabled},
		},
	}
	svc := NewService(ServiceOptions{Repo: repo, Now: fixedNow})

	user, err := svc.RegisterWithTenantCode(context.Background(), RegisterInput{
		TenantCode:   "TENANT001",
		Username:     "student01",
		RealName:     "张三",
		PasswordHash: "hashed-password",
		Phone:        "13800000000",
		Email:        "student01@example.com",
	})
	if err != nil {
		t.Fatalf("RegisterWithTenantCode returned error: %v", err)
	}
	if user.ID != 1 {
		t.Fatalf("expected user ID 1, got %d", user.ID)
	}
	if repo.created.TenantID != 10 {
		t.Fatalf("expected tenant ID 10, got %d", repo.created.TenantID)
	}
	if repo.created.Status != StatusEnabled {
		t.Fatalf("expected enabled status, got %q", repo.created.Status)
	}
	if repo.created.RealName != "张三" {
		t.Fatalf("expected real name saved, got %q", repo.created.RealName)
	}
	if repo.createdRole != RoleStudent {
		t.Fatalf("expected student role saved, got %q", repo.createdRole)
	}
	if repo.addedSpaceMembership {
		t.Fatalf("expected self-registered user not to be added to any space")
	}
}

func TestRegisterWithTenantLinkUsesTenantCode(t *testing.T) {
	repo := &fakeRepository{
		tenantsByCode: map[string]Tenant{
			"TENANT001": {ID: 10, TenantCode: "TENANT001", AllowRegister: true, Status: StatusEnabled},
		},
	}
	svc := NewService(ServiceOptions{Repo: repo, Now: fixedNow})

	user, err := svc.RegisterWithTenantLink(context.Background(), RegisterInput{
		TenantCode:   "TENANT001",
		Username:     "student01",
		PasswordHash: "hashed-password",
	})
	if err != nil {
		t.Fatalf("RegisterWithTenantLink returned error: %v", err)
	}
	if user.TenantID != 10 {
		t.Fatalf("expected tenant ID 10, got %d", user.TenantID)
	}
}

func TestRegisterRejectsDisabledRegistration(t *testing.T) {
	repo := &fakeRepository{
		tenantsByCode: map[string]Tenant{
			"TENANT001": {ID: 10, TenantCode: "TENANT001", AllowRegister: false, Status: StatusEnabled},
		},
	}
	svc := NewService(ServiceOptions{Repo: repo, Now: fixedNow})

	_, err := svc.RegisterWithTenantCode(context.Background(), RegisterInput{
		TenantCode:   "TENANT001",
		Username:     "student01",
		PasswordHash: "hashed-password",
	})
	if !errors.Is(err, ErrRegisterNotAllowed) {
		t.Fatalf("expected ErrRegisterNotAllowed, got %v", err)
	}
	if repo.created.Username != "" {
		t.Fatalf("expected no user created, got %#v", repo.created)
	}
}

func TestLoginUpdatesLastLoginAuditOnSuccess(t *testing.T) {
	repo := &fakeRepository{
		usersByTenantAndUsername: map[userLookupKey]User{
			{tenantID: 10, username: "student01"}: {
				ID:           20,
				TenantID:     10,
				Username:     "student01",
				PasswordHash: "hashed-password",
				Status:       StatusEnabled,
			},
		},
	}
	verifier := fakePasswordVerifier{match: true}
	logger := &fakeSecurityLogger{}
	svc := NewService(ServiceOptions{
		Repo:             repo,
		PasswordVerifier: verifier,
		SecurityLogger:   logger,
		Now:              fixedNow,
	})

	user, err := svc.Login(context.Background(), LoginInput{
		TenantID: 10,
		Username: "student01",
		Password: "plain-password",
		IP:       "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}
	if user.ID != 20 {
		t.Fatalf("expected user ID 20, got %d", user.ID)
	}
	if repo.auditUserID != 20 {
		t.Fatalf("expected audit user ID 20, got %d", repo.auditUserID)
	}
	if repo.auditIP != "127.0.0.1" {
		t.Fatalf("expected audit IP 127.0.0.1, got %q", repo.auditIP)
	}
	if repo.auditAt != fixedUnixMilli {
		t.Fatalf("expected audit at %d, got %d", fixedUnixMilli, repo.auditAt)
	}
	if len(logger.failed) != 0 {
		t.Fatalf("expected no failed login log, got %#v", logger.failed)
	}
}

func TestLoginFailureWritesSecurityLogAndSkipsAuditUpdate(t *testing.T) {
	repo := &fakeRepository{
		usersByTenantAndUsername: map[userLookupKey]User{
			{tenantID: 10, username: "student01"}: {
				ID:           20,
				TenantID:     10,
				Username:     "student01",
				PasswordHash: "hashed-password",
				Status:       StatusEnabled,
			},
		},
	}
	verifier := fakePasswordVerifier{match: false}
	logger := &fakeSecurityLogger{}
	svc := NewService(ServiceOptions{
		Repo:             repo,
		PasswordVerifier: verifier,
		SecurityLogger:   logger,
		Now:              fixedNow,
	})

	_, err := svc.Login(context.Background(), LoginInput{
		TenantID: 10,
		Username: "student01",
		Password: "wrong-password",
		IP:       "127.0.0.1",
	})
	if !errors.Is(err, ErrInvalidCredential) {
		t.Fatalf("expected ErrInvalidCredential, got %v", err)
	}
	if repo.auditUserID != 0 || repo.auditAt != 0 || repo.auditIP != "" {
		t.Fatalf("expected no audit update, got userID=%d ip=%q at=%d", repo.auditUserID, repo.auditIP, repo.auditAt)
	}
	if len(logger.failed) != 1 {
		t.Fatalf("expected one failed login log, got %d", len(logger.failed))
	}
	if logger.failed[0].Reason != LoginFailureInvalidCredential {
		t.Fatalf("expected invalid credential reason, got %q", logger.failed[0].Reason)
	}
}

func TestUploadAvatarValidatesTypeAndSizeBeforeSaving(t *testing.T) {
	storage := &fakeAvatarStorage{}
	repo := &fakeRepository{}
	svc := NewService(ServiceOptions{
		Repo:          repo,
		AvatarStorage: storage,
		MaxAvatarSize: 1024,
		AllowedAvatarContentTypes: []string{
			"image/png",
			"image/jpeg",
		},
		Now: fixedNow,
	})

	_, err := svc.UploadAvatar(context.Background(), UploadAvatarInput{
		TenantID:    10,
		UserID:      20,
		FileName:    "avatar.gif",
		ContentType: "image/gif",
		Size:        512,
	})
	if !errors.Is(err, ErrUnsupportedAvatarType) {
		t.Fatalf("expected ErrUnsupportedAvatarType, got %v", err)
	}

	_, err = svc.UploadAvatar(context.Background(), UploadAvatarInput{
		TenantID:    10,
		UserID:      20,
		FileName:    "avatar.png",
		ContentType: "image/png",
		Size:        2048,
	})
	if !errors.Is(err, ErrAvatarTooLarge) {
		t.Fatalf("expected ErrAvatarTooLarge, got %v", err)
	}
	if storage.savedUserID != 0 {
		t.Fatalf("expected invalid avatar not saved, got user ID %d", storage.savedUserID)
	}
}

func TestUploadAvatarSavesAndUpdatesAvatarURL(t *testing.T) {
	storage := &fakeAvatarStorage{returnURL: "avatars/tenant/20.png"}
	repo := &fakeRepository{}
	svc := NewService(ServiceOptions{
		Repo:          repo,
		AvatarStorage: storage,
		MaxAvatarSize: 1024,
		AllowedAvatarContentTypes: []string{
			"image/png",
		},
		Now: fixedNow,
	})

	url, err := svc.UploadAvatar(context.Background(), UploadAvatarInput{
		TenantID:    10,
		UserID:      20,
		FileName:    "avatar.png",
		ContentType: "image/png",
		Size:        512,
	})
	if err != nil {
		t.Fatalf("UploadAvatar returned error: %v", err)
	}
	if url != "avatars/tenant/20.png" {
		t.Fatalf("expected avatar URL avatars/tenant/20.png, got %q", url)
	}
	if storage.savedTenantID != 10 || storage.savedUserID != 20 {
		t.Fatalf("expected saved tenant=10 user=20, got tenant=%d user=%d", storage.savedTenantID, storage.savedUserID)
	}
	if repo.avatarTenantID != 10 || repo.avatarUserID != 20 || repo.avatarURL != "avatars/tenant/20.png" {
		t.Fatalf("expected avatar URL update, got tenant=%d user=%d url=%q", repo.avatarTenantID, repo.avatarUserID, repo.avatarURL)
	}
}

func TestPreviewDisableReturnsImpactBeforeStatusChange(t *testing.T) {
	expected := DisableImpact{
		LoseLogin:       true,
		LoseExamAccess:  true,
		LoseGradeAccess: true,
		AdminSpaceIDs:   []uint64{100, 101},
	}
	repo := &fakeRepository{impact: expected}
	svc := NewService(ServiceOptions{Repo: repo, Now: fixedNow})

	impact, err := svc.PreviewDisable(context.Background(), PreviewDisableInput{
		TenantID: 10,
		UserID:   20,
	})
	if err != nil {
		t.Fatalf("PreviewDisable returned error: %v", err)
	}
	if !impact.LoseLogin || !impact.LoseExamAccess || !impact.LoseGradeAccess {
		t.Fatalf("expected disable impact flags, got %#v", impact)
	}
	if len(impact.AdminSpaceIDs) != 2 || impact.AdminSpaceIDs[0] != 100 || impact.AdminSpaceIDs[1] != 101 {
		t.Fatalf("expected admin space IDs [100 101], got %#v", impact.AdminSpaceIDs)
	}
	if repo.updatedStatus != "" {
		t.Fatalf("expected preview not to update status, got %q", repo.updatedStatus)
	}
}

func TestDisableRejectsSelf(t *testing.T) {
	svc := NewService(ServiceOptions{Repo: &fakeRepository{}, Now: fixedNow})

	err := svc.Disable(context.Background(), DisableInput{
		TenantID: 10,
		ActorID:  20,
		TargetID: 20,
	})
	if !errors.Is(err, ErrCannotDisableSelf) {
		t.Fatalf("expected ErrCannotDisableSelf, got %v", err)
	}
}

func TestDisableValidatesSpaceAdminInvariantBeforeStatusChange(t *testing.T) {
	repo := &fakeRepository{}
	checker := &fakeSpaceAdminInvariantChecker{}
	svc := NewService(ServiceOptions{
		Repo:                       repo,
		SpaceAdminInvariantChecker: checker,
		Now:                        fixedNow,
	})

	err := svc.Disable(context.Background(), DisableInput{
		TenantID: 10,
		ActorID:  99,
		TargetID: 20,
	})
	if err != nil {
		t.Fatalf("Disable returned error: %v", err)
	}
	if checker.checkedTenantID != 10 || checker.checkedUserID != 20 {
		t.Fatalf("expected invariant check tenant=10 user=20, got tenant=%d user=%d", checker.checkedTenantID, checker.checkedUserID)
	}
	if repo.updatedStatusTenantID != 10 || repo.updatedStatusUserID != 20 || repo.updatedStatus != StatusDisabled {
		t.Fatalf("expected disabled status update, got tenant=%d user=%d status=%q", repo.updatedStatusTenantID, repo.updatedStatusUserID, repo.updatedStatus)
	}
}

const fixedUnixMilli int64 = 1767225600123

func fixedNow() int64 {
	return fixedUnixMilli
}

type fakeRepository struct {
	tenantsByCode map[string]Tenant
	created       User
	createdRole   string

	addedSpaceMembership bool

	usersByTenantAndUsername map[userLookupKey]User

	auditTenantID uint64
	auditUserID   uint64
	auditIP       string
	auditAt       int64

	avatarTenantID uint64
	avatarUserID   uint64
	avatarURL      string

	impact DisableImpact

	updatedStatusTenantID uint64
	updatedStatusUserID   uint64
	updatedStatus         string

	profileInput UpdateProfileInput
}

func (r *fakeRepository) ListUsers(ctx context.Context, tenantID uint64, page pagination.Input) (pagination.Result[User], error) {
	page = pagination.Normalize(page)
	return pagination.Result[User]{
		Items:    nil,
		Page:     page.Page,
		PageSize: page.PageSize,
		Total:    0,
	}, nil
}

func (r *fakeRepository) FindUserByID(ctx context.Context, tenantID uint64, userID uint64) (User, error) {
	return User{}, ErrUserNotFound
}

func (r *fakeRepository) FindTenantByCode(ctx context.Context, tenantCode string) (Tenant, error) {
	tenant, ok := r.tenantsByCode[tenantCode]
	if !ok {
		return Tenant{}, ErrTenantNotFound
	}
	return tenant, nil
}

func (r *fakeRepository) CreateUser(ctx context.Context, user User) (User, error) {
	user.ID = 1
	r.created = user
	return user, nil
}

func (r *fakeRepository) CreateUserRole(ctx context.Context, tenantID uint64, userID uint64, role string) error {
	r.createdRole = role
	return nil
}

func (r *fakeRepository) CreateUserWithRole(ctx context.Context, user User, role string) (User, error) {
	created, err := r.CreateUser(ctx, user)
	if err != nil {
		return User{}, err
	}
	if err := r.CreateUserRole(ctx, created.TenantID, created.ID, role); err != nil {
		return User{}, err
	}
	created.Role = role
	return created, nil
}

func (r *fakeRepository) FindUserByUsername(ctx context.Context, tenantID uint64, username string) (User, error) {
	user, ok := r.usersByTenantAndUsername[userLookupKey{tenantID: tenantID, username: username}]
	if !ok {
		return User{}, ErrUserNotFound
	}
	return user, nil
}

func (r *fakeRepository) UpdateLoginAudit(ctx context.Context, tenantID uint64, userID uint64, ip string, at int64) error {
	r.auditTenantID = tenantID
	r.auditUserID = userID
	r.auditIP = ip
	r.auditAt = at
	return nil
}

func (r *fakeRepository) UpdateAvatarURL(ctx context.Context, tenantID uint64, userID uint64, url string) error {
	r.avatarTenantID = tenantID
	r.avatarUserID = userID
	r.avatarURL = url
	return nil
}

func (r *fakeRepository) BuildDisableImpact(ctx context.Context, tenantID uint64, userID uint64) (DisableImpact, error) {
	return r.impact, nil
}

func (r *fakeRepository) UpdateStatus(ctx context.Context, tenantID uint64, userID uint64, status string) error {
	r.updatedStatusTenantID = tenantID
	r.updatedStatusUserID = userID
	r.updatedStatus = status
	return nil
}

func (r *fakeRepository) UpdateProfile(ctx context.Context, input UpdateProfileInput) (User, error) {
	r.profileInput = input
	return User{
		ID:        input.UserID,
		TenantID:  input.TenantID,
		RealName:  input.DisplayName,
		AvatarURL: input.AvatarURL,
		Phone:     input.Phone,
		Email:     input.Email,
		Status:    StatusEnabled,
	}, nil
}

type userLookupKey struct {
	tenantID uint64
	username string
}

type fakePasswordVerifier struct {
	match bool
}

func (v fakePasswordVerifier) Verify(hash string, password string) bool {
	return v.match
}

type failedLogin struct {
	TenantID uint64
	Username string
	IP       string
	Reason   string
}

type fakeSecurityLogger struct {
	failed []failedLogin
}

func (l *fakeSecurityLogger) LoginFailed(ctx context.Context, tenantID uint64, username string, ip string, reason string) error {
	l.failed = append(l.failed, failedLogin{
		TenantID: tenantID,
		Username: username,
		IP:       ip,
		Reason:   reason,
	})
	return nil
}

type fakeAvatarStorage struct {
	returnURL     string
	savedTenantID uint64
	savedUserID   uint64
}

func (s *fakeAvatarStorage) SaveTenantUserAvatar(ctx context.Context, input AvatarObjectInput) (string, error) {
	s.savedTenantID = input.TenantID
	s.savedUserID = input.UserID
	return s.returnURL, nil
}

type fakeSpaceAdminInvariantChecker struct {
	checkedTenantID uint64
	checkedUserID   uint64
}

func (c *fakeSpaceAdminInvariantChecker) ValidateBeforeDisableTenantUser(ctx context.Context, tenantID uint64, userID uint64) error {
	c.checkedTenantID = tenantID
	c.checkedUserID = userID
	return nil
}

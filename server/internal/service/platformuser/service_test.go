package platformuser

import (
	"context"
	"errors"
	"testing"
)

func TestSeedAdminCreatesInitialPlatformAdmin(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewService(ServiceOptions{
		Repo: repo,
		Now:  fixedNow,
	})

	user, err := svc.SeedAdmin(context.Background(), SeedAdminInput{
		Username:     "admin",
		Email:        "admin@iminho.me",
		PasswordHash: "hashed-password",
	})
	if err != nil {
		t.Fatalf("SeedAdmin returned error: %v", err)
	}
	if user.ID != 1 {
		t.Fatalf("expected seeded admin ID 1, got %d", user.ID)
	}
	if repo.created.Username != "admin" {
		t.Fatalf("expected username admin, got %q", repo.created.Username)
	}
	if repo.created.Status != StatusEnabled {
		t.Fatalf("expected enabled status, got %q", repo.created.Status)
	}
	if repo.created.Email != "admin@iminho.me" {
		t.Fatalf("expected email admin@iminho.me, got %q", repo.created.Email)
	}
}

func TestSeedAdminSkipsWhenPlatformAdminAlreadyExists(t *testing.T) {
	repo := &fakeRepository{existingAny: true}
	svc := NewService(ServiceOptions{
		Repo: repo,
		Now:  fixedNow,
	})

	user, err := svc.SeedAdmin(context.Background(), SeedAdminInput{
		Username:     "admin",
		PasswordHash: "hashed-password",
	})
	if err != nil {
		t.Fatalf("SeedAdmin returned error: %v", err)
	}
	if user != (PlatformUser{}) {
		t.Fatalf("expected empty user when seed skipped, got %#v", user)
	}
	if repo.created.Username != "" {
		t.Fatalf("expected no created user, got %#v", repo.created)
	}
}

func TestLoginUpdatesLastLoginAuditOnSuccess(t *testing.T) {
	repo := &fakeRepository{
		usersByUsername: map[string]PlatformUser{
			"admin": {
				ID:           12,
				Username:     "admin",
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
		Username: "admin",
		Password: "plain-password",
		IP:       "127.0.0.1",
	})
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}
	if user.ID != 12 {
		t.Fatalf("expected user ID 12, got %d", user.ID)
	}
	if repo.auditUserID != 12 {
		t.Fatalf("expected audit user ID 12, got %d", repo.auditUserID)
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
		usersByUsername: map[string]PlatformUser{
			"admin": {
				ID:           12,
				Username:     "admin",
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
		Username: "admin",
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

func TestDisableRejectsSelf(t *testing.T) {
	svc := NewService(ServiceOptions{Repo: &fakeRepository{}, Now: fixedNow})

	err := svc.Disable(context.Background(), DisableInput{
		ActorID:  10,
		TargetID: 10,
	})
	if !errors.Is(err, ErrCannotDisableSelf) {
		t.Fatalf("expected ErrCannotDisableSelf, got %v", err)
	}
}

func TestDisableRejectsLastEnabledPlatformAdmin(t *testing.T) {
	repo := &fakeRepository{
		usersByID: map[uint64]PlatformUser{
			20: {ID: 20, Status: StatusEnabled},
		},
		enabledAdmins: 1,
	}
	svc := NewService(ServiceOptions{Repo: repo, Now: fixedNow})

	err := svc.Disable(context.Background(), DisableInput{
		ActorID:  10,
		TargetID: 20,
	})
	if !errors.Is(err, ErrCannotDisableLastAdmin) {
		t.Fatalf("expected ErrCannotDisableLastAdmin, got %v", err)
	}
	if repo.updatedStatus != "" {
		t.Fatalf("expected no status update, got %q", repo.updatedStatus)
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
		UserID:      12,
		FileName:    "avatar.gif",
		ContentType: "image/gif",
		Size:        512,
	})
	if !errors.Is(err, ErrUnsupportedAvatarType) {
		t.Fatalf("expected ErrUnsupportedAvatarType, got %v", err)
	}

	_, err = svc.UploadAvatar(context.Background(), UploadAvatarInput{
		UserID:      12,
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
	storage := &fakeAvatarStorage{returnURL: "avatars/12.png"}
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
		UserID:      12,
		FileName:    "avatar.png",
		ContentType: "image/png",
		Size:        512,
	})
	if err != nil {
		t.Fatalf("UploadAvatar returned error: %v", err)
	}
	if url != "avatars/12.png" {
		t.Fatalf("expected avatar URL avatars/12.png, got %q", url)
	}
	if storage.savedUserID != 12 {
		t.Fatalf("expected saved user ID 12, got %d", storage.savedUserID)
	}
	if repo.avatarUserID != 12 || repo.avatarURL != "avatars/12.png" {
		t.Fatalf("expected avatar URL update, got userID=%d url=%q", repo.avatarUserID, repo.avatarURL)
	}
}

const fixedUnixMilli int64 = 1767225600123

func fixedNow() int64 {
	return fixedUnixMilli
}

type fakeRepository struct {
	existingAny bool
	created     PlatformUser

	usersByUsername map[string]PlatformUser
	usersByID       map[uint64]PlatformUser
	enabledAdmins   int64

	auditUserID uint64
	auditIP     string
	auditAt     int64

	updatedUserID uint64
	updatedStatus string

	avatarUserID uint64
	avatarURL    string
}

func (r *fakeRepository) HasAny(ctx context.Context) (bool, error) {
	return r.existingAny, nil
}

func (r *fakeRepository) Create(ctx context.Context, user PlatformUser) (PlatformUser, error) {
	user.ID = 1
	r.created = user
	return user, nil
}

func (r *fakeRepository) FindByUsername(ctx context.Context, username string) (PlatformUser, error) {
	user, ok := r.usersByUsername[username]
	if !ok {
		return PlatformUser{}, ErrPlatformUserNotFound
	}
	return user, nil
}

func (r *fakeRepository) FindByID(ctx context.Context, userID uint64) (PlatformUser, error) {
	user, ok := r.usersByID[userID]
	if !ok {
		return PlatformUser{}, ErrPlatformUserNotFound
	}
	return user, nil
}

func (r *fakeRepository) CountEnabledAdmins(ctx context.Context) (int64, error) {
	return r.enabledAdmins, nil
}

func (r *fakeRepository) UpdateLoginAudit(ctx context.Context, userID uint64, ip string, at int64) error {
	r.auditUserID = userID
	r.auditIP = ip
	r.auditAt = at
	return nil
}

func (r *fakeRepository) UpdateStatus(ctx context.Context, userID uint64, status string) error {
	r.updatedUserID = userID
	r.updatedStatus = status
	return nil
}

func (r *fakeRepository) UpdateAvatarURL(ctx context.Context, userID uint64, url string) error {
	r.avatarUserID = userID
	r.avatarURL = url
	return nil
}

type fakePasswordVerifier struct {
	match bool
}

func (v fakePasswordVerifier) Verify(hash string, password string) bool {
	return v.match
}

type failedLogin struct {
	Username string
	IP       string
	Reason   string
}

type fakeSecurityLogger struct {
	failed []failedLogin
}

func (l *fakeSecurityLogger) LoginFailed(ctx context.Context, username string, ip string, reason string) error {
	l.failed = append(l.failed, failedLogin{
		Username: username,
		IP:       ip,
		Reason:   reason,
	})
	return nil
}

type fakeAvatarStorage struct {
	returnURL   string
	savedUserID uint64
}

func (s *fakeAvatarStorage) SavePlatformUserAvatar(ctx context.Context, input AvatarObjectInput) (string, error) {
	s.savedUserID = input.UserID
	return s.returnURL, nil
}

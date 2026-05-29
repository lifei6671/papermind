package space

import (
	"context"
	"errors"
	"testing"

	"github.com/lifei6671/papermind/server/internal/service/pagination"
)

func TestCreateSpaceRequiresAtLeastOneSpaceAdmin(t *testing.T) {
	svc := NewService(ServiceOptions{Repo: &fakeRepository{}})

	_, err := svc.Create(context.Background(), CreateInput{
		TenantID: 10,
		Name:     "高一一班",
		Type:     "class",
	})
	if !errors.Is(err, ErrSpaceAdminRequired) {
		t.Fatalf("expected ErrSpaceAdminRequired, got %v", err)
	}
}

func TestCreateSpaceSavesOptionalLogoAndDescription(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewService(ServiceOptions{Repo: repo})

	created, err := svc.Create(context.Background(), CreateInput{
		TenantID:     10,
		Name:         "高一一班",
		Type:         "class",
		LogoURL:      "logos/space.png",
		Description:  "高一年级一班空间",
		AdminUserIDs: []uint64{20},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if created.ID != 1 {
		t.Fatalf("expected space ID 1, got %d", created.ID)
	}
	if repo.createdSpace.LogoURL != "logos/space.png" {
		t.Fatalf("expected logo URL saved, got %q", repo.createdSpace.LogoURL)
	}
	if repo.createdSpace.Description != "高一年级一班空间" {
		t.Fatalf("expected description saved, got %q", repo.createdSpace.Description)
	}
	if repo.createdSpace.Status != StatusEnabled {
		t.Fatalf("expected enabled status, got %q", repo.createdSpace.Status)
	}
	if len(repo.createdAdminUserIDs) != 1 || repo.createdAdminUserIDs[0] != 20 {
		t.Fatalf("expected admin user IDs [20], got %#v", repo.createdAdminUserIDs)
	}
}

func TestCreateSpaceAllowsEmptyLogoAndDescription(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewService(ServiceOptions{Repo: repo})

	_, err := svc.Create(context.Background(), CreateInput{
		TenantID:     10,
		Name:         "公共题库空间",
		Type:         "custom",
		AdminUserIDs: []uint64{20},
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if repo.createdSpace.LogoURL != "" || repo.createdSpace.Description != "" {
		t.Fatalf("expected empty optional fields, got logo=%q description=%q", repo.createdSpace.LogoURL, repo.createdSpace.Description)
	}
}

func TestUpdateProfileSavesSpaceProfileFields(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewService(ServiceOptions{Repo: repo})

	updated, err := svc.UpdateProfile(context.Background(), UpdateProfileInput{
		TenantID:    10,
		SpaceID:     100,
		Name:        "高一二班",
		LogoURL:     "logos/class2.png",
		Description: "高一二班考试空间",
		Type:        "class",
	})
	if err != nil {
		t.Fatalf("UpdateProfile returned error: %v", err)
	}
	if updated.Name != "高一二班" || updated.LogoURL != "logos/class2.png" {
		t.Fatalf("unexpected updated space: %#v", updated)
	}
	if repo.updatedProfile.SpaceID != 100 || repo.updatedProfile.Description != "高一二班考试空间" {
		t.Fatalf("expected repository update input, got %#v", repo.updatedProfile)
	}
}

func TestDeleteSpaceUsesTenantAndSpaceScope(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewService(ServiceOptions{Repo: repo})

	if err := svc.Delete(context.Background(), DeleteInput{TenantID: 10, SpaceID: 100}); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if repo.deletedTenantID != 10 || repo.deletedSpaceID != 100 {
		t.Fatalf("expected delete tenant=10 space=100, got tenant=%d space=%d", repo.deletedTenantID, repo.deletedSpaceID)
	}
}

func TestJoinMemberCreatesEnabledMember(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewService(ServiceOptions{Repo: repo})

	member, err := svc.JoinMember(context.Background(), JoinMemberInput{
		TenantID: 10,
		SpaceID:  100,
		UserID:   20,
		Role:     RoleTeacher,
	})
	if err != nil {
		t.Fatalf("JoinMember returned error: %v", err)
	}
	if member.ID != 1 {
		t.Fatalf("expected member ID 1, got %d", member.ID)
	}
	if repo.addedMember.Status != StatusEnabled {
		t.Fatalf("expected enabled member, got %q", repo.addedMember.Status)
	}
	if repo.addedMember.Role != RoleTeacher {
		t.Fatalf("expected teacher role, got %q", repo.addedMember.Role)
	}
}

func TestJoinMemberRejectsInvalidRole(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewService(ServiceOptions{Repo: repo})

	err := func() error {
		_, err := svc.JoinMember(context.Background(), JoinMemberInput{
			TenantID: 10,
			SpaceID:  100,
			UserID:   20,
			Role:     "owner",
		})
		return err
	}()
	if !errors.Is(err, ErrInvalidMemberRole) {
		t.Fatalf("expected ErrInvalidMemberRole, got %v", err)
	}
	if repo.addedMember.UserID != 0 {
		t.Fatalf("expected invalid role not to reach repository, got %#v", repo.addedMember)
	}
}

func TestChangeMemberRoleRejectsInvalidRole(t *testing.T) {
	repo := &fakeRepository{}
	svc := NewService(ServiceOptions{Repo: repo})

	err := svc.ChangeMemberRole(context.Background(), ChangeRoleInput{
		TenantID: 10,
		SpaceID:  100,
		UserID:   20,
		Role:     "owner",
	})
	if !errors.Is(err, ErrInvalidMemberRole) {
		t.Fatalf("expected ErrInvalidMemberRole, got %v", err)
	}
	if repo.changedRole != "" {
		t.Fatalf("expected invalid role not to reach repository, got %q", repo.changedRole)
	}
}

func TestListEffectiveMembersUsesEnabledAndNotDeletedRepositoryQuery(t *testing.T) {
	repo := &fakeRepository{
		effectiveMembers: []Member{
			{TenantID: 10, SpaceID: 100, UserID: 20, Role: RoleTeacher, Status: StatusEnabled},
		},
	}
	svc := NewService(ServiceOptions{Repo: repo})

	members, err := svc.ListEffectiveMembers(context.Background(), 10, 100)
	if err != nil {
		t.Fatalf("ListEffectiveMembers returned error: %v", err)
	}
	if !repo.listEffectiveCalled {
		t.Fatalf("expected effective member repository query to be called")
	}
	if len(members) != 1 || members[0].UserID != 20 {
		t.Fatalf("expected effective member user 20, got %#v", members)
	}
}

func TestListEffectiveMembershipsForUserUsesTenantAndUserScope(t *testing.T) {
	repo := &fakeRepository{
		effectiveMembershipsForUser: []Member{
			{TenantID: 10, SpaceID: 301, UserID: 21, Role: RoleTeacher, Status: StatusEnabled},
			{TenantID: 10, SpaceID: 302, UserID: 21, Role: RoleSpaceAdmin, Status: StatusEnabled},
		},
	}
	svc := NewService(ServiceOptions{Repo: repo})

	members, err := svc.ListEffectiveMembershipsForUser(context.Background(), 10, 21)
	if err != nil {
		t.Fatalf("ListEffectiveMembershipsForUser returned error: %v", err)
	}
	if repo.listMembershipTenantID != 10 || repo.listMembershipUserID != 21 {
		t.Fatalf("expected tenant 10 user 21, got tenant=%d user=%d", repo.listMembershipTenantID, repo.listMembershipUserID)
	}
	if len(members) != 2 || members[1].Role != RoleSpaceAdmin {
		t.Fatalf("expected effective memberships for user, got %#v", members)
	}
}

func TestDisableMemberRejectsLastEnabledSpaceAdmin(t *testing.T) {
	repo := &fakeRepository{
		membersByKey: map[memberKey]Member{
			{tenantID: 10, spaceID: 100, userID: 20}: {TenantID: 10, SpaceID: 100, UserID: 20, Role: RoleSpaceAdmin, Status: StatusEnabled},
		},
		enabledSpaceAdminCount: 1,
	}
	svc := NewService(ServiceOptions{Repo: repo})

	err := svc.DisableMember(context.Background(), MemberActionInput{
		TenantID: 10,
		SpaceID:  100,
		UserID:   20,
	})
	if !errors.Is(err, ErrCannotLoseLastSpaceAdmin) {
		t.Fatalf("expected ErrCannotLoseLastSpaceAdmin, got %v", err)
	}
	if repo.disabledUserID != 0 {
		t.Fatalf("expected no disable update, got user ID %d", repo.disabledUserID)
	}
}

func TestRemoveMemberRejectsLastEnabledSpaceAdmin(t *testing.T) {
	repo := &fakeRepository{
		membersByKey: map[memberKey]Member{
			{tenantID: 10, spaceID: 100, userID: 20}: {TenantID: 10, SpaceID: 100, UserID: 20, Role: RoleSpaceAdmin, Status: StatusEnabled},
		},
		enabledSpaceAdminCount: 1,
	}
	svc := NewService(ServiceOptions{Repo: repo})

	err := svc.RemoveMember(context.Background(), MemberActionInput{
		TenantID: 10,
		SpaceID:  100,
		UserID:   20,
	})
	if !errors.Is(err, ErrCannotLoseLastSpaceAdmin) {
		t.Fatalf("expected ErrCannotLoseLastSpaceAdmin, got %v", err)
	}
	if repo.removedUserID != 0 {
		t.Fatalf("expected no remove update, got user ID %d", repo.removedUserID)
	}
}

func TestChangeMemberRoleRejectsDowngradingLastEnabledSpaceAdmin(t *testing.T) {
	repo := &fakeRepository{
		membersByKey: map[memberKey]Member{
			{tenantID: 10, spaceID: 100, userID: 20}: {TenantID: 10, SpaceID: 100, UserID: 20, Role: RoleSpaceAdmin, Status: StatusEnabled},
		},
		enabledSpaceAdminCount: 1,
	}
	svc := NewService(ServiceOptions{Repo: repo})

	err := svc.ChangeMemberRole(context.Background(), ChangeRoleInput{
		TenantID: 10,
		SpaceID:  100,
		UserID:   20,
		Role:     RoleTeacher,
	})
	if !errors.Is(err, ErrCannotLoseLastSpaceAdmin) {
		t.Fatalf("expected ErrCannotLoseLastSpaceAdmin, got %v", err)
	}
	if repo.changedRole != "" {
		t.Fatalf("expected no role update, got %q", repo.changedRole)
	}
}

func TestDisableMemberAllowsLastAdminWhenSpaceIsNotEnabled(t *testing.T) {
	repo := &fakeRepository{
		membersByKey: map[memberKey]Member{
			{tenantID: 10, spaceID: 100, userID: 20}: {TenantID: 10, SpaceID: 100, UserID: 20, Role: RoleSpaceAdmin, Status: StatusEnabled},
		},
		enabledSpaceAdminCount: 0,
	}
	svc := NewService(ServiceOptions{Repo: repo})

	if err := svc.DisableMember(context.Background(), MemberActionInput{TenantID: 10, SpaceID: 100, UserID: 20}); err != nil {
		t.Fatalf("DisableMember returned error: %v", err)
	}
	if repo.disabledUserID != 20 {
		t.Fatalf("expected disabled user 20, got %d", repo.disabledUserID)
	}
}

func TestMemberOperationsCallUnifiedSpaceAdminInvariant(t *testing.T) {
	repo := &fakeRepository{
		membersByKey: map[memberKey]Member{
			{tenantID: 10, spaceID: 100, userID: 20}: {TenantID: 10, SpaceID: 100, UserID: 20, Role: RoleSpaceAdmin, Status: StatusEnabled},
		},
		enabledSpaceAdminCount: 2,
	}
	svc := NewService(ServiceOptions{Repo: repo})

	if err := svc.DisableMember(context.Background(), MemberActionInput{TenantID: 10, SpaceID: 100, UserID: 20}); err != nil {
		t.Fatalf("DisableMember returned error: %v", err)
	}
	if repo.invariantChecks != 1 || repo.disabledUserID != 20 {
		t.Fatalf("expected disable to call invariant once and update user 20, checks=%d disabled=%d", repo.invariantChecks, repo.disabledUserID)
	}

	if err := svc.RemoveMember(context.Background(), MemberActionInput{TenantID: 10, SpaceID: 100, UserID: 20}); err != nil {
		t.Fatalf("RemoveMember returned error: %v", err)
	}
	if repo.invariantChecks != 2 || repo.removedUserID != 20 {
		t.Fatalf("expected remove to call invariant twice and update user 20, checks=%d removed=%d", repo.invariantChecks, repo.removedUserID)
	}

	if err := svc.ChangeMemberRole(context.Background(), ChangeRoleInput{TenantID: 10, SpaceID: 100, UserID: 20, Role: RoleTeacher}); err != nil {
		t.Fatalf("ChangeMemberRole returned error: %v", err)
	}
	if repo.invariantChecks != 3 || repo.changedRole != RoleTeacher {
		t.Fatalf("expected role change to call invariant three times and update role, checks=%d role=%q", repo.invariantChecks, repo.changedRole)
	}
}

type fakeRepository struct {
	createdSpace        Space
	createdAdminUserIDs []uint64

	updatedProfile  UpdateProfileInput
	deletedTenantID uint64
	deletedSpaceID  uint64

	addedMember Member

	effectiveMembers    []Member
	listEffectiveCalled bool

	effectiveMembershipsForUser []Member
	listMembershipTenantID      uint64
	listMembershipUserID        uint64

	membersByKey            map[memberKey]Member
	enabledSpaceAdminCount  int64
	invariantChecks         int
	disabledUserID          uint64
	removedUserID           uint64
	changedRole             string
	changedRoleUserID       uint64
	changedRoleUpdatedSpace uint64
}

func (r *fakeRepository) ListSpaces(ctx context.Context, tenantID uint64, page pagination.Input) (pagination.Result[Space], error) {
	page = pagination.Normalize(page)
	items := []Space{r.createdSpace}
	return pagination.Result[Space]{
		Items:    items,
		Page:     page.Page,
		PageSize: page.PageSize,
		Total:    int64(len(items)),
	}, nil
}

func (r *fakeRepository) CreateSpace(ctx context.Context, space Space, adminUserIDs []uint64) (Space, error) {
	space.ID = 1
	r.createdSpace = space
	r.createdAdminUserIDs = adminUserIDs
	return space, nil
}

func (r *fakeRepository) UpdateSpaceProfile(ctx context.Context, input UpdateProfileInput) (Space, error) {
	r.updatedProfile = input
	return Space{
		ID:          input.SpaceID,
		TenantID:    input.TenantID,
		Name:        input.Name,
		LogoURL:     input.LogoURL,
		Description: input.Description,
		Type:        input.Type,
		Status:      StatusEnabled,
	}, nil
}

func (r *fakeRepository) DeleteSpace(ctx context.Context, tenantID uint64, spaceID uint64) error {
	r.deletedTenantID = tenantID
	r.deletedSpaceID = spaceID
	return nil
}

func (r *fakeRepository) AddMember(ctx context.Context, member Member) (Member, error) {
	member.ID = 1
	r.addedMember = member
	return member, nil
}

func (r *fakeRepository) ListEffectiveMembers(ctx context.Context, tenantID uint64, spaceID uint64) ([]Member, error) {
	r.listEffectiveCalled = true
	return r.effectiveMembers, nil
}

func (r *fakeRepository) ListEffectiveMembershipsForUser(ctx context.Context, tenantID uint64, userID uint64) ([]Member, error) {
	r.listMembershipTenantID = tenantID
	r.listMembershipUserID = userID
	return r.effectiveMembershipsForUser, nil
}

func (r *fakeRepository) FindMember(ctx context.Context, tenantID uint64, spaceID uint64, userID uint64) (Member, error) {
	member, ok := r.membersByKey[memberKey{tenantID: tenantID, spaceID: spaceID, userID: userID}]
	if !ok {
		return Member{}, ErrMemberNotFound
	}
	return member, nil
}

func (r *fakeRepository) CountEnabledSpaceAdmins(ctx context.Context, tenantID uint64, spaceID uint64) (int64, error) {
	r.invariantChecks++
	return r.enabledSpaceAdminCount, nil
}

func (r *fakeRepository) DisableMember(ctx context.Context, tenantID uint64, spaceID uint64, userID uint64) error {
	r.disabledUserID = userID
	return nil
}

func (r *fakeRepository) RemoveMember(ctx context.Context, tenantID uint64, spaceID uint64, userID uint64) error {
	r.removedUserID = userID
	return nil
}

func (r *fakeRepository) UpdateMemberRole(ctx context.Context, tenantID uint64, spaceID uint64, userID uint64, role string) error {
	r.changedRoleUpdatedSpace = spaceID
	r.changedRoleUserID = userID
	r.changedRole = role
	return nil
}

type memberKey struct {
	tenantID uint64
	spaceID  uint64
	userID   uint64
}

package permission

import "testing"

func TestPermissionContextFromActorCopiesOnlyPrimaryRole(t *testing.T) {
	ctx := PermissionContextFromActor(ActorContext{
		SubjectType: SubjectTenantUser,
		UserID:      20,
		TenantID:    10,
		Role:        RoleTeacher,
	})

	if ctx.Role != RoleTeacher || ctx.hasSpaceRole(100, RoleSpaceAdmin) {
		t.Fatalf("permission context should copy tenant role only, got %#v", ctx)
	}
}

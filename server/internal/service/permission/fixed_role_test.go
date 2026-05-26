package permission

import "testing"

func TestFixedRoleCheckerManagesTenantByPlatformOrTenantAdmin(t *testing.T) {
	checker := NewFixedRoleChecker()

	if err := checker.CanManageTenant(PermissionContext{SubjectType: SubjectPlatformUser}, 99); err != nil {
		t.Fatalf("platform user should manage any tenant: %v", err)
	}
	if err := checker.CanManageTenant(PermissionContext{SubjectType: SubjectTenantUser, TenantID: 7, TenantRoles: []string{RoleTenantAdmin}}, 7); err != nil {
		t.Fatalf("tenant admin should manage own tenant: %v", err)
	}
	if err := checker.CanManageTenant(PermissionContext{SubjectType: SubjectTenantUser, TenantID: 7, TenantRoles: []string{RoleTenantAdmin}}, 8); err == nil {
		t.Fatalf("tenant admin should not manage another tenant")
	}
}

func TestFixedRoleCheckerManagesSpaceByTenantAdminOrSpaceAdmin(t *testing.T) {
	checker := NewFixedRoleChecker()

	tenantAdmin := PermissionContext{SubjectType: SubjectTenantUser, TenantID: 7, TenantRoles: []string{RoleTenantAdmin}}
	if err := checker.CanManageSpace(tenantAdmin, 100); err != nil {
		t.Fatalf("tenant admin should manage tenant spaces: %v", err)
	}

	spaceAdmin := PermissionContext{SubjectType: SubjectTenantUser, TenantID: 7, SpaceRoles: map[uint64]string{100: RoleSpaceAdmin}}
	if err := checker.CanManageSpace(spaceAdmin, 100); err != nil {
		t.Fatalf("space admin should manage own space: %v", err)
	}
	if err := checker.CanManageSpace(spaceAdmin, 101); err == nil {
		t.Fatalf("space admin should not manage another space")
	}
}

func TestFixedRoleCheckerUsesResourceScopeForQuestionsPapersAttemptsAndExams(t *testing.T) {
	checker := NewFixedRoleChecker()

	teacher := PermissionContext{
		SubjectType:   SubjectTenantUser,
		TenantID:      7,
		SpaceRoles:    map[uint64]string{100: RoleTeacher},
		QuestionScope: map[uint64]uint64{200: 100},
		PaperScope:    map[uint64]uint64{300: 100},
		AttemptScope:  map[uint64]uint64{400: 100},
		ExamScope:     map[uint64]uint64{500: 100},
	}

	if err := checker.CanManageQuestion(teacher, 200); err != nil {
		t.Fatalf("teacher should manage scoped question: %v", err)
	}
	if err := checker.CanPublishExam(teacher, 300); err != nil {
		t.Fatalf("teacher should publish scoped paper: %v", err)
	}
	if err := checker.CanGradeAttempt(teacher, 400); err != nil {
		t.Fatalf("teacher should grade scoped attempt: %v", err)
	}
	if err := checker.CanGradeExam(teacher, 500); err != nil {
		t.Fatalf("teacher should grade scoped exam: %v", err)
	}
	if err := checker.CanTakeExam(teacher, 500); err == nil {
		t.Fatalf("teacher without student role should not take exam")
	}

	student := PermissionContext{
		SubjectType: SubjectTenantUser,
		TenantID:    7,
		SpaceRoles:  map[uint64]string{100: RoleStudent},
		ExamScope:   map[uint64]uint64{500: 100},
	}
	if err := checker.CanTakeExam(student, 500); err != nil {
		t.Fatalf("student should take scoped exam: %v", err)
	}
}

func TestFixedRoleCheckerRejectsMissingOrUnknownScope(t *testing.T) {
	checker := NewFixedRoleChecker()

	ctx := PermissionContext{SubjectType: SubjectTenantUser, TenantID: 7, SpaceRoles: map[uint64]string{100: RoleTeacher}}
	if err := checker.CanManageQuestion(ctx, 200); err == nil {
		t.Fatalf("question without scope should be denied")
	}
	if err := checker.CanTakeExam(PermissionContext{SubjectType: SubjectPlatformUser}, 500); err == nil {
		t.Fatalf("platform user should not take tenant exam")
	}
}

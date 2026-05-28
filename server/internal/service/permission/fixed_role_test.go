package permission

import "testing"

func TestFixedRoleCheckerManagesTenantByPlatformOrTenantAdmin(t *testing.T) {
	checker := NewFixedRoleChecker()

	if err := checker.CanManageTenantLifecycle(PermissionContext{SubjectType: SubjectPlatformUser, Role: RolePlatformAdmin}, 99); err != nil {
		t.Fatalf("platform admin should manage tenant lifecycle: %v", err)
	}
	if err := checker.CanManageTenantBusiness(PermissionContext{SubjectType: SubjectTenantUser, TenantID: 7, Role: RoleTenantAdmin}, 7); err != nil {
		t.Fatalf("tenant admin should manage own tenant business: %v", err)
	}
	if err := checker.CanManageTenantLifecycle(PermissionContext{SubjectType: SubjectTenantUser, TenantID: 7, Role: RoleTenantAdmin}, 7); err == nil {
		t.Fatalf("tenant admin should not manage tenant lifecycle")
	}
	if err := checker.CanManageTenantBusiness(PermissionContext{SubjectType: SubjectTenantUser, TenantID: 7, Role: RoleTenantAdmin}, 8); err == nil {
		t.Fatalf("tenant admin should not manage another tenant business")
	}
}

func TestFixedRoleCheckerSplitsSpaceProfileAndMemberManagement(t *testing.T) {
	checker := NewFixedRoleChecker()

	tenantAdmin := PermissionContext{SubjectType: SubjectTenantUser, TenantID: 7, Role: RoleTenantAdmin}
	if err := checker.CanManageSpaceProfile(tenantAdmin, 100); err != nil {
		t.Fatalf("tenant admin should manage tenant space profile: %v", err)
	}

	spaceAdmin := PermissionContext{SubjectType: SubjectTenantUser, TenantID: 7, Role: RoleTeacher, SpaceMemberships: map[uint64]string{100: RoleSpaceAdmin}}
	if err := checker.CanManageSpaceProfile(spaceAdmin, 100); err == nil {
		t.Fatalf("space admin should not manage space profile")
	}
	if err := checker.CanManageSpaceMembers(spaceAdmin, 100); err != nil {
		t.Fatalf("space admin should manage own space members: %v", err)
	}
	if err := checker.CanManageSpaceMembers(spaceAdmin, 101); err == nil {
		t.Fatalf("space admin should not manage another space members")
	}
}

func TestFixedRoleCheckerUsesResourceScopeForQuestionsPapersAttemptsAndExams(t *testing.T) {
	checker := NewFixedRoleChecker()

	teacher := PermissionContext{
		SubjectType:      SubjectTenantUser,
		TenantID:         7,
		Role:             RoleTeacher,
		SpaceMemberships: map[uint64]string{100: RoleTeacher},
		QuestionScope:    map[uint64]uint64{200: 100},
		PaperScope:       map[uint64]uint64{300: 100},
		AttemptScope:     map[uint64]uint64{400: 100},
		ExamScope:        map[uint64]uint64{500: 100},
	}

	if err := checker.CanManageQuestion(teacher, 200); err != nil {
		t.Fatalf("teacher should manage scoped question: %v", err)
	}
	if err := checker.CanManagePaper(teacher, 300); err != nil {
		t.Fatalf("teacher should manage scoped paper: %v", err)
	}
	if err := checker.CanPublishExam(teacher, 300); err != nil {
		t.Fatalf("teacher should publish scoped paper: %v", err)
	}
	if err := checker.CanGradeAttempt(teacher, 400); err != nil {
		t.Fatalf("teacher should grade scoped attempt: %v", err)
	}
	if err := checker.CanViewExamResults(teacher, 500); err != nil {
		t.Fatalf("teacher should view scoped exam results: %v", err)
	}
	if err := checker.CanExportExamResults(teacher, 500); err == nil {
		t.Fatalf("teacher should not export exam results")
	}
	if err := checker.CanTakeExam(teacher, 500); err == nil {
		t.Fatalf("teacher without student role should not take exam")
	}

	student := PermissionContext{
		SubjectType: SubjectTenantUser,
		TenantID:    7,
		Role:        RoleStudent,
		ExamScope:   map[uint64]uint64{500: 100},
	}
	if err := checker.CanTakeExam(student, 500); err != nil {
		t.Fatalf("student should take scoped exam: %v", err)
	}
}

func TestFixedRoleCheckerRejectsMissingOrUnknownScope(t *testing.T) {
	checker := NewFixedRoleChecker()

	ctx := PermissionContext{SubjectType: SubjectTenantUser, TenantID: 7, Role: RoleTeacher, SpaceMemberships: map[uint64]string{100: RoleTeacher}}
	if err := checker.CanManageQuestion(ctx, 200); err == nil {
		t.Fatalf("question without scope should be denied")
	}
	if err := checker.CanTakeExam(PermissionContext{SubjectType: SubjectPlatformUser}, 500); err == nil {
		t.Fatalf("platform user should not take tenant exam")
	}
}

func TestFixedRoleCheckerAllowsTenantAdminWithoutSpaceMembership(t *testing.T) {
	checker := NewFixedRoleChecker()
	ctx := PermissionContext{
		SubjectType:   SubjectTenantUser,
		TenantID:      7,
		Role:          RoleTenantAdmin,
		QuestionScope: map[uint64]uint64{200: 100},
		PaperScope:    map[uint64]uint64{300: 100},
		AttemptScope:  map[uint64]uint64{400: 100},
		ExamScope:     map[uint64]uint64{500: 100},
	}

	if err := checker.CanManageQuestion(ctx, 200); err != nil {
		t.Fatalf("tenant admin should manage tenant question without space membership: %v", err)
	}
	if err := checker.CanManagePaper(ctx, 300); err != nil {
		t.Fatalf("tenant admin should manage tenant paper without space membership: %v", err)
	}
	if err := checker.CanGradeAttempt(ctx, 400); err != nil {
		t.Fatalf("tenant admin should grade tenant attempt without space membership: %v", err)
	}
	if err := checker.CanExportExamResults(ctx, 500); err != nil {
		t.Fatalf("tenant admin should export tenant exam results without space membership: %v", err)
	}
}

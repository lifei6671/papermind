package permission

import "errors"

var ErrForbidden = errors.New("permission denied")

type FixedRoleChecker struct{}

func NewFixedRoleChecker() PermissionChecker {
	return FixedRoleChecker{}
}

func (FixedRoleChecker) CanManageTenantLifecycle(ctx PermissionContext, tenantID uint64) error {
	if ctx.SubjectType == SubjectPlatformUser && ctx.Role == RolePlatformAdmin {
		return nil
	}
	return ErrForbidden
}

func (FixedRoleChecker) CanManageTenantBusiness(ctx PermissionContext, tenantID uint64) error {
	return canManageTenant(ctx, tenantID)
}

func (FixedRoleChecker) CanManageTenantUsers(ctx PermissionContext, tenantID uint64) error {
	return canManageTenant(ctx, tenantID)
}

func (FixedRoleChecker) CanManageSpaceProfile(ctx PermissionContext, spaceID uint64) error {
	if ctx.SubjectType == SubjectTenantUser && ctx.hasTenantRole(RoleTenantAdmin) {
		return nil
	}
	return ErrForbidden
}

func (FixedRoleChecker) CanManageSpaceMembers(ctx PermissionContext, spaceID uint64) error {
	if ctx.SubjectType != SubjectTenantUser {
		return ErrForbidden
	}
	if ctx.hasTenantRole(RoleTenantAdmin) || ctx.hasSpaceRole(spaceID, RoleSpaceAdmin) {
		return nil
	}
	return ErrForbidden
}

func (FixedRoleChecker) CanManageQuestion(ctx PermissionContext, questionID uint64) error {
	spaceID, ok := ctx.QuestionScope[questionID]
	if !ok {
		return ErrForbidden
	}
	return canManageSpaceResource(ctx, spaceID)
}

func (FixedRoleChecker) CanManagePaper(ctx PermissionContext, paperID uint64) error {
	spaceID, ok := ctx.PaperScope[paperID]
	if !ok {
		return ErrForbidden
	}
	return canManageSpaceResource(ctx, spaceID)
}

func (FixedRoleChecker) CanPublishExam(ctx PermissionContext, paperID uint64) error {
	spaceID, ok := ctx.PaperScope[paperID]
	if !ok {
		return ErrForbidden
	}
	return canManageSpaceResource(ctx, spaceID)
}

func (FixedRoleChecker) CanGradeAttempt(ctx PermissionContext, attemptID uint64) error {
	spaceID, ok := ctx.AttemptScope[attemptID]
	if !ok {
		return ErrForbidden
	}
	return canManageSpaceResource(ctx, spaceID)
}

func (FixedRoleChecker) CanViewExamResults(ctx PermissionContext, examID uint64) error {
	spaceID, ok := ctx.ExamScope[examID]
	if !ok {
		return ErrForbidden
	}
	return canManageSpaceResource(ctx, spaceID)
}

func (FixedRoleChecker) CanExportExamResults(ctx PermissionContext, examID uint64) error {
	spaceID, ok := ctx.ExamScope[examID]
	if !ok {
		return ErrForbidden
	}
	if ctx.hasTenantRole(RoleTenantAdmin) || ctx.hasSpaceRole(spaceID, RoleSpaceAdmin) {
		return nil
	}
	return ErrForbidden
}

func (FixedRoleChecker) CanViewOwnResult(ctx PermissionContext, resultID uint64) error {
	// 是否为本人由 ResultService 基于 attempt 快照校验；这里仅拒绝非租户主体。
	if ctx.SubjectType == SubjectTenantUser {
		return nil
	}
	return ErrForbidden
}

func (FixedRoleChecker) CanTakeExam(ctx PermissionContext, examID uint64) error {
	if ctx.SubjectType != SubjectTenantUser {
		return ErrForbidden
	}
	if _, ok := ctx.ExamScope[examID]; !ok {
		return ErrForbidden
	}
	if ctx.hasTenantRole(RoleStudent) {
		return nil
	}
	return ErrForbidden
}

func canManageTenant(ctx PermissionContext, tenantID uint64) error {
	if ctx.SubjectType == SubjectTenantUser && ctx.TenantID == tenantID && ctx.hasTenantRole(RoleTenantAdmin) {
		return nil
	}
	return ErrForbidden
}

func canManageSpaceResource(ctx PermissionContext, spaceID uint64) error {
	if ctx.SubjectType != SubjectTenantUser {
		return ErrForbidden
	}
	if ctx.hasTenantRole(RoleTenantAdmin) || ctx.hasSpaceRole(spaceID, RoleSpaceAdmin, RoleTeacher) {
		return nil
	}
	return ErrForbidden
}

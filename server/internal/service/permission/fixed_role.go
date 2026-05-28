package permission

import "errors"

var ErrForbidden = errors.New("permission denied")

type FixedRoleChecker struct{}

func NewFixedRoleChecker() PermissionChecker {
	return FixedRoleChecker{}
}

func (FixedRoleChecker) CanManageTenant(ctx PermissionContext, tenantID uint64) error {
	if ctx.SubjectType == SubjectPlatformUser {
		return nil
	}
	if ctx.SubjectType == SubjectTenantUser && ctx.TenantID == tenantID && ctx.hasTenantRole(RoleTenantAdmin) {
		return nil
	}
	return ErrForbidden
}

func (FixedRoleChecker) CanManageSpace(ctx PermissionContext, spaceID uint64) error {
	if ctx.SubjectType == SubjectPlatformUser {
		return nil
	}
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

func (FixedRoleChecker) CanPublishExam(ctx PermissionContext, paperID uint64) error {
	spaceID, ok := ctx.PaperScope[paperID]
	if !ok {
		return ErrForbidden
	}
	return canManageSpaceResource(ctx, spaceID)
}

func (FixedRoleChecker) CanGradeExam(ctx PermissionContext, examID uint64) error {
	spaceID, ok := ctx.ExamScope[examID]
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

func (FixedRoleChecker) CanTakeExam(ctx PermissionContext, examID uint64) error {
	if ctx.SubjectType != SubjectTenantUser {
		return ErrForbidden
	}
	spaceID, ok := ctx.ExamScope[examID]
	if !ok {
		return ErrForbidden
	}
	if ctx.hasTenantRole(RoleStudent) || ctx.hasSpaceRole(spaceID, RoleStudent) {
		return nil
	}
	return ErrForbidden
}

func canManageSpaceResource(ctx PermissionContext, spaceID uint64) error {
	if ctx.SubjectType != SubjectTenantUser {
		return ErrForbidden
	}
	if ctx.hasSpaceRole(spaceID, RoleSpaceAdmin, RoleTeacher) {
		return nil
	}
	return ErrForbidden
}

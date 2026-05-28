package permission

// PermissionChecker 是 service 层唯一依赖的权限判断入口。
// 业务 service 不直接判断角色字符串，后续替换 RBAC 时只需要替换该接口实现。
type PermissionChecker interface {
	CanManageTenantLifecycle(ctx PermissionContext, tenantID uint64) error
	CanManageTenantBusiness(ctx PermissionContext, tenantID uint64) error
	CanManageTenantUsers(ctx PermissionContext, tenantID uint64) error
	CanManageSpaceProfile(ctx PermissionContext, spaceID uint64) error
	CanManageSpaceMembers(ctx PermissionContext, spaceID uint64) error
	CanManageQuestion(ctx PermissionContext, questionID uint64) error
	CanManagePaper(ctx PermissionContext, paperID uint64) error
	CanPublishExam(ctx PermissionContext, paperID uint64) error
	CanGradeAttempt(ctx PermissionContext, attemptID uint64) error
	CanViewExamResults(ctx PermissionContext, examID uint64) error
	CanExportExamResults(ctx PermissionContext, examID uint64) error
	CanViewOwnResult(ctx PermissionContext, resultID uint64) error
	CanTakeExam(ctx PermissionContext, examID uint64) error
}

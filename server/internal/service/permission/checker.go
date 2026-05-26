package permission

// PermissionChecker 是 service 层唯一依赖的权限判断入口。
// 业务 service 不直接判断角色字符串，后续替换 RBAC 时只需要替换该接口实现。
type PermissionChecker interface {
	CanManageTenant(ctx PermissionContext, tenantID uint64) error
	CanManageSpace(ctx PermissionContext, spaceID uint64) error
	CanManageQuestion(ctx PermissionContext, questionID uint64) error
	CanPublishExam(ctx PermissionContext, paperID uint64) error
	CanGradeExam(ctx PermissionContext, examID uint64) error
	CanGradeAttempt(ctx PermissionContext, attemptID uint64) error
	CanTakeExam(ctx PermissionContext, examID uint64) error
}

package permission

const (
	SubjectPlatformUser = "platform_user"
	SubjectTenantUser   = "tenant_user"

	RoleTenantAdmin = "tenant_admin"
	RoleSpaceAdmin  = "space_admin"
	RoleTeacher     = "teacher"
	RoleStudent     = "student"
)

// PermissionContext 是认证解析后传给 service 层的权限上下文。
// 首版固定角色实现只依赖这里的角色和资源归属快照，避免在权限层直接访问 GORM。
type PermissionContext struct {
	SubjectType string // 主体类型：platform_user / tenant_user。
	UserID      uint64 // 当前登录用户 ID。
	TenantID    uint64 // 当前租户用户所属租户 ID，平台用户为 0。

	TenantRoles []string          // 租户级角色：tenant_admin / teacher / student。
	SpaceRoles  map[uint64]string // 空间内角色，key 为空间 ID，value 为 space_admin / teacher / student。

	QuestionScope map[uint64]uint64 // 题目到空间 ID 的授权快照，key 为 questionID。
	PaperScope    map[uint64]uint64 // 试卷到空间 ID 的授权快照，key 为 paperID。
	AttemptScope  map[uint64]uint64 // 作答到空间 ID 的授权快照，key 为 attemptID。
	ExamScope     map[uint64]uint64 // 考试到空间 ID 的授权快照，key 为 examID。
}

func (ctx PermissionContext) hasTenantRole(role string) bool {
	for _, item := range ctx.TenantRoles {
		if item == role {
			return true
		}
	}
	return false
}

func (ctx PermissionContext) hasAnyTenantRole(roles ...string) bool {
	for _, role := range roles {
		if ctx.hasTenantRole(role) {
			return true
		}
	}
	return false
}

func (ctx PermissionContext) hasSpaceRole(spaceID uint64, roles ...string) bool {
	if ctx.SpaceRoles == nil {
		return false
	}
	current, ok := ctx.SpaceRoles[spaceID]
	if !ok {
		return false
	}
	for _, role := range roles {
		if current == role {
			return true
		}
	}
	return false
}

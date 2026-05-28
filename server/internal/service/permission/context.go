package permission

import "github.com/lifei6671/papermind/server/library/constant"

const (
	SubjectPlatformUser = constant.SubjectPlatformUser
	SubjectTenantUser   = constant.SubjectTenantUser

	RolePlatformAdmin = constant.RolePlatformAdmin
	RoleTenantAdmin   = constant.RoleTenantAdmin
	RoleSpaceAdmin    = constant.RoleSpaceAdmin
	RoleTeacher       = constant.RoleTeacher
	RoleStudent       = constant.RoleStudent
)

// ActorContext 是认证层解析出的统一主体身份。
// Role 只保存平台角色或租户级角色，空间管理员身份必须来自空间成员授权查询。
type ActorContext struct {
	SubjectType string
	UserID      uint64
	TenantID    uint64
	Role        string
}

func PermissionContextFromActor(actor ActorContext) PermissionContext {
	return PermissionContext{
		SubjectType: actor.SubjectType,
		UserID:      actor.UserID,
		TenantID:    actor.TenantID,
		Role:        actor.Role,
	}
}

// PermissionContext 是认证解析后传给 service 层的权限上下文。
// 首版固定角色实现只依赖租户级角色和资源归属快照，避免在权限层直接访问 GORM。
type PermissionContext struct {
	SubjectType string // 主体类型：platform_user / tenant_user。
	UserID      uint64 // 当前登录用户 ID。
	TenantID    uint64 // 当前租户用户所属租户 ID，平台用户为 0。
	Role        string // 平台角色或租户级角色：platform_admin / tenant_admin / teacher / student。

	SpaceMemberships map[uint64]string // 空间授权快照，key 为空间 ID，value 来自 space_members.role_in_space。

	QuestionScope map[uint64]uint64 // 题目到空间 ID 的授权快照，key 为 questionID。
	PaperScope    map[uint64]uint64 // 试卷到空间 ID 的授权快照，key 为 paperID。
	AttemptScope  map[uint64]uint64 // 作答到空间 ID 的授权快照，key 为 attemptID。
	ExamScope     map[uint64]uint64 // 考试到空间 ID 的授权快照，key 为 examID。
}

func (ctx PermissionContext) hasTenantRole(role string) bool {
	return ctx.SubjectType == SubjectTenantUser && ctx.Role == role
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
	if ctx.SpaceMemberships == nil {
		return false
	}
	current, ok := ctx.SpaceMemberships[spaceID]
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

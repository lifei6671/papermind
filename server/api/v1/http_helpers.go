package v1

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/papermind/server/internal/service/pagination"
	"github.com/lifei6671/papermind/server/internal/service/permission"
	servicespace "github.com/lifei6671/papermind/server/internal/service/space"
	servicetenantuser "github.com/lifei6671/papermind/server/internal/service/tenantuser"
	"github.com/lifei6671/papermind/server/library/code"
	"github.com/lifei6671/papermind/server/library/response"
)

func writePermissionOrInternalError(c *gin.Context, err error, fallback string) {
	if errors.Is(err, permission.ErrForbidden) {
		c.JSON(http.StatusForbidden, response.Fail(code.InvalidParam, "无权执行当前操作"))
		return
	}
	c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, fallback))
}

func currentTenantAdminPrincipal(c *gin.Context) (AuthPrincipal, bool) {
	principal, ok := currentAuthPrincipal(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, response.Fail(code.InvalidParam, "请先登录"))
		return AuthPrincipal{}, false
	}
	// 租户管理接口只能绑定当前租户管理员会话，平台账号不能借 tenant_id 进入租户业务。
	if principal.SubjectType == permission.SubjectTenantUser &&
		principal.Role == permission.RoleTenantAdmin &&
		principal.TenantID != 0 {
		return principal, true
	}
	c.JSON(http.StatusForbidden, response.Fail(code.InvalidParam, "无权执行当前操作"))
	return AuthPrincipal{}, false
}

func currentLiveTenantAdminPrincipal(c *gin.Context, users *servicetenantuser.Service) (AuthPrincipal, bool) {
	principal, ok := currentTenantAdminPrincipal(c)
	if !ok {
		return AuthPrincipal{}, false
	}
	user, err := users.Get(c.Request.Context(), principal.TenantID, principal.UserID)
	if errors.Is(err, servicetenantuser.ErrUserNotFound) {
		c.JSON(http.StatusForbidden, response.Fail(code.InvalidParam, "无权执行当前操作"))
		return AuthPrincipal{}, false
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "读取当前账号权限失败"))
		return AuthPrincipal{}, false
	}
	// 租户管理写入口必须实时读取当前用户状态和角色，避免旧 session 在降权、禁用或删除后继续生效。
	if user.Status != servicetenantuser.StatusEnabled || user.Role != servicetenantuser.RoleTenantAdmin {
		c.JSON(http.StatusForbidden, response.Fail(code.InvalidParam, "无权执行当前操作"))
		return AuthPrincipal{}, false
	}
	return principal, true
}

// authorizeExamBusiness 校验当前主体是否能进入租户考试业务接口。
//
// 这里只做管理端粗粒度入口判断，具体空间范围和资源归属仍由后续
// PermissionContext 或 service 反查。空间管理员身份不写入 session，
// 需要从当前启用的空间成员关系反查后放行。
func authorizeExamBusiness(c *gin.Context, tenantID uint64, members spaceMemberFinder) bool {
	principal, ok := currentAuthPrincipal(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, response.Fail(code.InvalidParam, "请先登录"))
		return false
	}
	if principal.SubjectType == permission.SubjectTenantUser && principal.TenantID == tenantID {
		if principal.Role == permission.RoleTenantAdmin || principal.Role == permission.RoleTeacher {
			return true
		}
		allowed, err := hasExamBusinessMembership(c, members, tenantID, principal.UserID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "读取当前用户空间授权失败"))
			return false
		}
		if allowed {
			return true
		}
	}
	c.JSON(http.StatusForbidden, response.Fail(code.InvalidParam, "无权执行当前操作"))
	return false
}

func hasExamBusinessMembership(c *gin.Context, members spaceMemberFinder, tenantID uint64, userID uint64) (bool, error) {
	if members == nil {
		return false, nil
	}
	memberships, err := members.ListEffectiveMembershipsForUser(c.Request.Context(), tenantID, userID)
	if err != nil {
		return false, err
	}
	for _, member := range memberships {
		if member.Status == servicespace.StatusEnabled &&
			(member.Role == servicespace.RoleSpaceAdmin || member.Role == servicespace.RoleTeacher) {
			return true, nil
		}
	}
	return false, nil
}

// permissionContextForResourceScope 从登录态和真实资源空间重建权限上下文。
//
// 该 helper 是题库、试卷、考试发布等写接口的关键边界：space_admin /
// teacher 的空间身份必须从 space_members 反查，不能来自 session role。
func liveTenantPrincipalFromSession(c *gin.Context, tenantID uint64, users *servicetenantuser.Service) (AuthPrincipal, error) {
	principal, ok := currentAuthPrincipal(c)
	if !ok {
		return AuthPrincipal{}, errors.New("请先登录")
	}
	if principal.SubjectType != permission.SubjectTenantUser || principal.TenantID != tenantID {
		return AuthPrincipal{}, permission.ErrForbidden
	}
	if users == nil {
		return principal, nil
	}
	user, err := users.Get(c.Request.Context(), tenantID, principal.UserID)
	if errors.Is(err, servicetenantuser.ErrUserNotFound) {
		return AuthPrincipal{}, permission.ErrForbidden
	}
	if err != nil {
		return AuthPrincipal{}, err
	}
	// 管理端权限入口必须实时读取租户用户状态和角色，避免旧 session 在降权、禁用或删除后继续生效。
	if user.Status != servicetenantuser.StatusEnabled || user.Role != principal.Role {
		return AuthPrincipal{}, permission.ErrForbidden
	}
	return principal, nil
}

func permissionContextForResourceScope(c *gin.Context, tenantID uint64, spaceID *uint64, members spaceMemberFinder, users *servicetenantuser.Service) (permission.PermissionContext, error) {
	principal, err := liveTenantPrincipalFromSession(c, tenantID, users)
	if err != nil {
		return permission.PermissionContext{}, err
	}
	ctx := permission.PermissionContext{
		SubjectType:      principal.SubjectType,
		UserID:           principal.UserID,
		TenantID:         tenantID,
		Role:             principal.Role,
		SpaceMemberships: map[uint64]string{},
	}
	if principal.Role == permission.RoleTenantAdmin {
		return ctx, nil
	}
	if spaceID == nil {
		return permission.PermissionContext{}, permission.ErrForbidden
	}
	member, err := members.FindMember(c.Request.Context(), tenantID, *spaceID, principal.UserID)
	if errors.Is(err, servicespace.ErrMemberNotFound) {
		return permission.PermissionContext{}, permission.ErrForbidden
	}
	if err != nil {
		return permission.PermissionContext{}, err
	}
	if member.Status != servicespace.StatusEnabled {
		return permission.PermissionContext{}, permission.ErrForbidden
	}
	ctx.SpaceMemberships[*spaceID] = member.Role
	return ctx, nil
}

func readUintQuery(c *gin.Context, key string) (uint64, error) {
	raw := c.Query(key)
	if raw == "" {
		return 0, errors.New("missing query")
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || value == 0 {
		return 0, errors.New("invalid query")
	}
	return value, nil
}

func readPaginationQuery(c *gin.Context) (int, int, error) {
	page, err := readPositiveIntQuery(c, "page", pagination.DefaultPage)
	if err != nil {
		return 0, 0, err
	}
	pageSize, err := readPositiveIntQuery(c, "page_size", pagination.DefaultPageSize)
	if err != nil {
		return 0, 0, err
	}
	if pageSize > pagination.MaxPageSize {
		pageSize = pagination.MaxPageSize
	}
	return page, pageSize, nil
}

func readPositiveIntQuery(c *gin.Context, key string, defaultValue int) (int, error) {
	raw := c.Query(key)
	if raw == "" {
		return defaultValue, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, errors.New("invalid query")
	}
	return value, nil
}

func readUintParam(c *gin.Context, key string) (uint64, error) {
	raw := c.Param(key)
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || value == 0 {
		return 0, errors.New("invalid param")
	}
	return value, nil
}

func readOptionalUintQuery(c *gin.Context, key string) (*uint64, error) {
	raw := c.Query(key)
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || value == 0 {
		return nil, errors.New("invalid query")
	}
	return &value, nil
}

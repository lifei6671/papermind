package v1

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/papermind/server/internal/service/pagination"
	"github.com/lifei6671/papermind/server/internal/service/permission"
	servicespace "github.com/lifei6671/papermind/server/internal/service/space"
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

// authorizeTenantManagement 校验当前主体是否能管理指定租户。
//
// 平台管理员可进入平台侧租户治理；租户侧管理必须是当前租户的 tenant_admin。
func authorizeTenantManagement(c *gin.Context, tenantID uint64) bool {
	principal, ok := currentAuthPrincipal(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, response.Fail(code.InvalidParam, "请先登录"))
		return false
	}
	if principal.SubjectType == permission.SubjectPlatformUser {
		return true
	}
	if principal.SubjectType == permission.SubjectTenantUser &&
		principal.Role == permission.RoleTenantAdmin &&
		principal.TenantID == tenantID {
		return true
	}
	c.JSON(http.StatusForbidden, response.Fail(code.InvalidParam, "无权执行当前操作"))
	return false
}

// authorizeExamBusiness 校验当前主体是否能进入租户考试业务接口。
//
// 这里只做管理端粗粒度入口判断，具体空间范围和资源归属仍由后续
// PermissionContext 或 service 反查。
func authorizeExamBusiness(c *gin.Context, tenantID uint64) bool {
	principal, ok := currentAuthPrincipal(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, response.Fail(code.InvalidParam, "请先登录"))
		return false
	}
	if principal.SubjectType == permission.SubjectTenantUser &&
		principal.TenantID == tenantID &&
		(principal.Role == permission.RoleTenantAdmin || principal.Role == permission.RoleTeacher) {
		return true
	}
	c.JSON(http.StatusForbidden, response.Fail(code.InvalidParam, "无权执行当前操作"))
	return false
}

// permissionContextForResourceScope 从登录态和真实资源空间重建权限上下文。
//
// 该 helper 是题库、试卷、考试发布等写接口的关键边界：space_admin /
// teacher 的空间身份必须从 space_members 反查，不能来自 session role。
func permissionContextForResourceScope(c *gin.Context, tenantID uint64, spaceID *uint64, members spaceMemberFinder) (permission.PermissionContext, error) {
	principal, ok := currentAuthPrincipal(c)
	if !ok {
		return permission.PermissionContext{}, errors.New("请先登录")
	}
	ctx := permission.PermissionContext{
		SubjectType:      principal.SubjectType,
		UserID:           principal.UserID,
		TenantID:         tenantID,
		Role:             principal.Role,
		SpaceMemberships: map[uint64]string{},
	}
	if principal.SubjectType != permission.SubjectTenantUser || principal.TenantID != tenantID {
		return permission.PermissionContext{}, permission.ErrForbidden
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

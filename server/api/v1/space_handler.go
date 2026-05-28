package v1

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	dbdao "github.com/lifei6671/papermind/server/internal/dao/db"
	"github.com/lifei6671/papermind/server/internal/service/permission"
	servicespace "github.com/lifei6671/papermind/server/internal/service/space"
	"github.com/lifei6671/papermind/server/library/code"
	"github.com/lifei6671/papermind/server/library/response"
)

type spaceMemberLister interface {
	ListMemberNames(ctx context.Context, tenantID uint64, spaceID uint64) ([]dbdao.SpaceMemberName, error)
	FindMember(ctx context.Context, tenantID uint64, spaceID uint64, userID uint64) (servicespace.Member, error)
}

// spaceHandler 处理租户空间和空间成员接口。
//
// 空间资料管理只允许 tenant_admin；成员管理会在 handler 层按真实空间成员
// 关系校验 tenant_admin 或当前空间 space_admin。
type spaceHandler struct {
	service *servicespace.Service
	members spaceMemberLister
}

type createSpaceRequest struct {
	TenantID     uint64   `json:"tenant_id"`
	Name         string   `json:"name"`
	LogoURL      string   `json:"logo_url"`
	Description  string   `json:"description"`
	Type         string   `json:"type"`
	AdminUserIDs []uint64 `json:"admin_user_ids"`
}

type spaceResponse struct {
	ID          uint64                `json:"id"`
	TenantID    uint64                `json:"tenant_id"`
	Name        string                `json:"name"`
	LogoURL     string                `json:"logo_url"`
	Description string                `json:"description"`
	Type        string                `json:"type"`
	Status      string                `json:"status"`
	Members     []spaceMemberResponse `json:"members"`
}

type spaceMemberResponse struct {
	ID     uint64 `json:"id"`
	UserID uint64 `json:"user_id"`
	Name   string `json:"name"`
	Role   string `json:"role"`
	Status string `json:"status"`
}

type addSpaceMemberRequest struct {
	TenantID uint64 `json:"tenant_id"`
	UserID   uint64 `json:"user_id"`
	Role     string `json:"role"`
}

type updateSpaceMemberRequest struct {
	TenantID uint64 `json:"tenant_id"`
	Role     string `json:"role"`
	Status   string `json:"status"`
}

type spaceListResponse struct {
	Items    []spaceResponse `json:"items"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
	Total    int64           `json:"total"`
}

func (h spaceHandler) list(c *gin.Context) {
	tenantID, err := readUintQuery(c, "tenant_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id 必须是正整数"))
		return
	}
	if !authorizeTenantManagement(c, tenantID) {
		return
	}
	page, pageSize, err := readPaginationQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "page 和 page_size 必须是正整数"))
		return
	}
	result, err := h.service.List(c.Request.Context(), servicespace.ListInput{TenantID: tenantID, Page: page, PageSize: pageSize})
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "读取空间列表失败"))
		return
	}
	items := make([]spaceResponse, 0, len(result.Items))
	for _, space := range result.Items {
		item, err := h.spaceToResponse(c.Request.Context(), space)
		if err != nil {
			c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "读取空间成员失败"))
			return
		}
		items = append(items, item)
	}
	paged := response.Page(items, result.Page, result.PageSize, result.Total)
	c.JSON(http.StatusOK, response.OK(spaceListResponse{
		Items:    paged.Items,
		Page:     paged.Page,
		PageSize: paged.PageSize,
		Total:    paged.Total,
	}))
}

func (h spaceHandler) create(c *gin.Context) {
	var request createSpaceRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "请求体不是合法 JSON"))
		return
	}
	if err := request.validate(); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, err.Error()))
		return
	}
	if !authorizeTenantManagement(c, request.TenantID) {
		return
	}
	space, err := h.service.Create(c.Request.Context(), servicespace.CreateInput{
		TenantID:     request.TenantID,
		Name:         request.Name,
		LogoURL:      request.LogoURL,
		Description:  request.Description,
		Type:         request.Type,
		AdminUserIDs: request.AdminUserIDs,
	})
	if err != nil {
		writeSpaceServiceError(c, err)
		return
	}
	result, err := h.spaceToResponse(c.Request.Context(), space)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "读取空间成员失败"))
		return
	}
	c.JSON(http.StatusOK, response.OK(result))
}

func (h spaceHandler) listMembers(c *gin.Context) {
	spaceID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "space id 必须是正整数"))
		return
	}
	tenantID, err := readUintQuery(c, "tenant_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id 必须是正整数"))
		return
	}
	if !h.authorizeSpaceMembers(c, tenantID, spaceID) {
		return
	}
	members, err := h.members.ListMemberNames(c.Request.Context(), tenantID, spaceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "读取空间成员失败"))
		return
	}
	c.JSON(http.StatusOK, response.OK(spaceMemberNamesToResponse(members)))
}

func (h spaceHandler) addMember(c *gin.Context) {
	spaceID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "space id 必须是正整数"))
		return
	}
	var request addSpaceMemberRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "请求体不是合法 JSON"))
		return
	}
	if request.TenantID == 0 || request.UserID == 0 || request.Role == "" {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id、user_id 和 role 不能为空"))
		return
	}
	if !h.authorizeSpaceMembers(c, request.TenantID, spaceID) {
		return
	}
	member, err := h.service.JoinMember(c.Request.Context(), servicespace.JoinMemberInput{
		TenantID: request.TenantID,
		SpaceID:  spaceID,
		UserID:   request.UserID,
		Role:     request.Role,
	})
	if err != nil {
		writeSpaceServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(spaceMemberToResponse(member, "")))
}

func (h spaceHandler) updateMember(c *gin.Context) {
	spaceID, userID, ok := readSpaceMemberParams(c)
	if !ok {
		return
	}
	var request updateSpaceMemberRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "请求体不是合法 JSON"))
		return
	}
	if request.TenantID == 0 {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id 必须是正整数"))
		return
	}
	if !h.authorizeSpaceMembers(c, request.TenantID, spaceID) {
		return
	}
	if request.Role != "" {
		if err := h.service.ChangeMemberRole(c.Request.Context(), servicespace.ChangeRoleInput{TenantID: request.TenantID, SpaceID: spaceID, UserID: userID, Role: request.Role}); err != nil {
			writeSpaceServiceError(c, err)
			return
		}
	}
	if request.Status == servicespace.StatusDisabled {
		if err := h.service.DisableMember(c.Request.Context(), servicespace.MemberActionInput{TenantID: request.TenantID, SpaceID: spaceID, UserID: userID}); err != nil {
			writeSpaceServiceError(c, err)
			return
		}
	}
	member, err := h.members.FindMember(c.Request.Context(), request.TenantID, spaceID, userID)
	if err != nil {
		writeSpaceServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(spaceMemberToResponse(member, "")))
}

func (h spaceHandler) removeMember(c *gin.Context) {
	spaceID, userID, ok := readSpaceMemberParams(c)
	if !ok {
		return
	}
	tenantID, err := readUintQuery(c, "tenant_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id 必须是正整数"))
		return
	}
	if !h.authorizeSpaceMembers(c, tenantID, spaceID) {
		return
	}
	if err := h.service.RemoveMember(c.Request.Context(), servicespace.MemberActionInput{TenantID: tenantID, SpaceID: spaceID, UserID: userID}); err != nil {
		writeSpaceServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(gin.H{"removed": true}))
}

func (h spaceHandler) authorizeSpaceMembers(c *gin.Context, tenantID uint64, spaceID uint64) bool {
	principal, ok := currentAuthPrincipal(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, response.Fail(code.InvalidParam, "请先登录"))
		return false
	}
	if principal.SubjectType != permission.SubjectTenantUser || principal.TenantID != tenantID {
		c.JSON(http.StatusForbidden, response.Fail(code.InvalidParam, "无权执行当前操作"))
		return false
	}
	if principal.Role == permission.RoleTenantAdmin {
		return true
	}
	member, err := h.members.FindMember(c.Request.Context(), tenantID, spaceID, principal.UserID)
	if err != nil || member.Status != servicespace.StatusEnabled || member.Role != servicespace.RoleSpaceAdmin {
		c.JSON(http.StatusForbidden, response.Fail(code.InvalidParam, "无权执行当前操作"))
		return false
	}
	return true
}

func readSpaceMemberParams(c *gin.Context) (uint64, uint64, bool) {
	spaceID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "space id 必须是正整数"))
		return 0, 0, false
	}
	userID, err := readUintParam(c, "user_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "user_id 必须是正整数"))
		return 0, 0, false
	}
	return spaceID, userID, true
}

func (r createSpaceRequest) validate() error {
	if r.TenantID == 0 {
		return errors.New("tenant_id 必须是正整数")
	}
	if r.Name == "" {
		return errors.New("name 不能为空")
	}
	if r.Type == "" {
		return errors.New("type 不能为空")
	}
	if len(r.AdminUserIDs) == 0 {
		return errors.New("admin_user_ids 不能为空")
	}
	return nil
}

func (h spaceHandler) spaceToResponse(ctx context.Context, space servicespace.Space) (spaceResponse, error) {
	members, err := h.members.ListMemberNames(ctx, space.TenantID, space.ID)
	if err != nil {
		return spaceResponse{}, err
	}
	result := spaceResponse{
		ID:          space.ID,
		TenantID:    space.TenantID,
		Name:        space.Name,
		LogoURL:     space.LogoURL,
		Description: space.Description,
		Type:        space.Type,
		Status:      space.Status,
		Members:     make([]spaceMemberResponse, 0, len(members)),
	}
	for _, member := range members {
		result.Members = append(result.Members, spaceMemberNameToResponse(member))
	}
	return result, nil
}

func spaceMemberNamesToResponse(members []dbdao.SpaceMemberName) []spaceMemberResponse {
	items := make([]spaceMemberResponse, 0, len(members))
	for _, member := range members {
		items = append(items, spaceMemberNameToResponse(member))
	}
	return items
}

func spaceMemberNameToResponse(member dbdao.SpaceMemberName) spaceMemberResponse {
	return spaceMemberResponse{
		ID:     member.ID,
		UserID: member.UserID,
		Name:   displaySpaceMemberName(member.UserID, member.Name),
		Role:   member.Role,
		Status: member.Status,
	}
}

func spaceMemberToResponse(member servicespace.Member, name string) spaceMemberResponse {
	return spaceMemberResponse{
		ID:     member.ID,
		UserID: member.UserID,
		Name:   displaySpaceMemberName(member.UserID, name),
		Role:   member.Role,
		Status: member.Status,
	}
}

func displaySpaceMemberName(userID uint64, name string) string {
	if name != "" {
		return name
	}
	return "用户 " + strconv.FormatUint(userID, 10)
}

func writeSpaceServiceError(c *gin.Context, err error) {
	if errors.Is(err, servicespace.ErrSpaceAdminRequired) ||
		errors.Is(err, servicespace.ErrCannotLoseLastSpaceAdmin) ||
		errors.Is(err, servicespace.ErrMemberNotFound) {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, err.Error()))
		return
	}
	c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "空间操作失败"))
}

package v1

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/papermind/server/internal/service/permission"
	servicequestion "github.com/lifei6671/papermind/server/internal/service/question"
	servicetenantuser "github.com/lifei6671/papermind/server/internal/service/tenantuser"
	"github.com/lifei6671/papermind/server/library/code"
	"github.com/lifei6671/papermind/server/library/response"
)

// questionHandler 处理题库读写接口。
//
// 公共题库和空间题库的写权限必须通过 permissionContextForResourceScope
// 从真实资源范围重建，不能信任前端传入的空间范围。
type questionHandler struct {
	service *servicequestion.QuestionService
	members spaceMemberFinder
	users   *servicetenantuser.Service
}

type createQuestionRequest struct {
	TenantID        uint64                  `json:"tenant_id"`
	SpaceID         *uint64                 `json:"space_id"`
	Type            string                  `json:"type"`
	Difficulty      string                  `json:"difficulty"`
	Title           string                  `json:"title"`
	Analysis        string                  `json:"analysis"`
	StandardAnswer  string                  `json:"standard_answer"`
	ReferenceAnswer string                  `json:"reference_answer"`
	BlankCount      int                     `json:"blank_count"`
	ScoreDefault    string                  `json:"score_default"`
	Tags            []string                `json:"tags"`
	Options         []questionOptionRequest `json:"options"`
}

type questionOptionRequest struct {
	OptionKey    string `json:"option_key"`
	Content      string `json:"content"`
	IsCorrect    bool   `json:"is_correct"`
	IsDistractor bool   `json:"is_distractor"`
}

type questionResponse struct {
	ID              uint64                   `json:"id"`
	TenantID        uint64                   `json:"tenant_id"`
	SpaceID         *uint64                  `json:"space_id,omitempty"`
	Type            string                   `json:"type"`
	Difficulty      string                   `json:"difficulty"`
	Title           string                   `json:"title"`
	Analysis        string                   `json:"analysis"`
	StandardAnswer  string                   `json:"standard_answer,omitempty"`
	ReferenceAnswer string                   `json:"reference_answer,omitempty"`
	ScoreDefault    string                   `json:"score_default"`
	Status          string                   `json:"status"`
	AuthorName      string                   `json:"author_name"`
	AuthorRole      string                   `json:"author_role"`
	CreatedAt       int64                    `json:"created_at"`
	Tag             string                   `json:"tag"`
	Tags            []string                 `json:"tags"`
	Options         []questionOptionResponse `json:"options"`
}

type questionTenantRequest struct {
	TenantID uint64 `json:"tenant_id"`
}

type questionOptionResponse struct {
	ID           uint64 `json:"id"`
	OptionKey    string `json:"option_key"`
	Content      string `json:"content"`
	IsCorrect    bool   `json:"is_correct"`
	IsDistractor bool   `json:"is_distractor"`
}

type questionListResponse struct {
	Items    []questionResponse `json:"items"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
	Total    int64              `json:"total"`
}

func (h questionHandler) list(c *gin.Context) {
	tenantID, err := readUintQuery(c, "tenant_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id 必须是正整数"))
		return
	}
	if !authorizeExamBusiness(c, tenantID, h.members) {
		return
	}
	spaceID, err := readOptionalUintQuery(c, "space_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "space_id 必须是正整数"))
		return
	}
	if _, err := permissionContextForResourceScope(c, tenantID, spaceID, h.members, h.users); err != nil {
		writePermissionOrInternalError(c, err, "构建题库权限上下文失败")
		return
	}
	page, pageSize, err := readPaginationQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "page 和 page_size 必须是正整数"))
		return
	}
	result, err := h.service.ListVisibleQuestions(c.Request.Context(), servicequestion.ListQuestionsInput{
		TenantID: tenantID,
		SpaceID:  spaceID,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "读取题目列表失败"))
		return
	}
	items := make([]questionResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, questionToResponse(item))
	}
	paged := response.Page(items, result.Page, result.PageSize, result.Total)
	c.JSON(http.StatusOK, response.OK(questionListResponse{
		Items:    paged.Items,
		Page:     paged.Page,
		PageSize: paged.PageSize,
		Total:    paged.Total,
	}))
}

func (h questionHandler) create(c *gin.Context) {
	var request createQuestionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "请求体不是合法 JSON"))
		return
	}
	if err := request.validate(); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, err.Error()))
		return
	}
	if !authorizeExamBusiness(c, request.TenantID, h.members) {
		return
	}
	permissionContext, err := permissionContextForResourceScope(c, request.TenantID, request.SpaceID, h.members, h.users)
	if err != nil {
		writePermissionOrInternalError(c, err, "构建题库权限上下文失败")
		return
	}
	created, err := h.service.CreateQuestion(c.Request.Context(), servicequestion.CreateQuestionInput{
		Permission:      permissionContext,
		TenantID:        request.TenantID,
		SpaceID:         request.SpaceID,
		Type:            request.Type,
		Difficulty:      request.Difficulty,
		Title:           request.Title,
		Analysis:        request.Analysis,
		StandardAnswer:  request.StandardAnswer,
		ReferenceAnswer: request.ReferenceAnswer,
		BlankCount:      request.BlankCount,
		ScoreDefault:    request.ScoreDefault,
		Options:         request.toServiceOptions(),
		Tags:            request.Tags,
	})
	if err != nil {
		writeQuestionServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(questionToResponse(created)))
}

func (h questionHandler) get(c *gin.Context) {
	questionID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "题目 ID 必须是正整数"))
		return
	}
	tenantID, err := readUintQuery(c, "tenant_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id 必须是正整数"))
		return
	}
	if !authorizeExamBusiness(c, tenantID, h.members) {
		return
	}
	permissionContext, err := h.permissionContextForQuestionOperation(c, tenantID)
	if err != nil {
		writePermissionOrInternalError(c, err, "构建题库权限上下文失败")
		return
	}
	item, err := h.service.GetQuestion(c.Request.Context(), servicequestion.GetQuestionInput{
		Permission: permissionContext,
		TenantID:   tenantID,
		QuestionID: questionID,
	})
	if err != nil {
		writeQuestionServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(questionToResponse(item)))
}

func (h questionHandler) update(c *gin.Context) {
	questionID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "题目 ID 必须是正整数"))
		return
	}
	var request createQuestionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "请求体不是合法 JSON"))
		return
	}
	if err := request.validate(); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, err.Error()))
		return
	}
	if !authorizeExamBusiness(c, request.TenantID, h.members) {
		return
	}
	permissionContext, err := h.permissionContextForQuestionOperation(c, request.TenantID)
	if err != nil {
		writePermissionOrInternalError(c, err, "构建题库权限上下文失败")
		return
	}
	updated, err := h.service.UpdateQuestion(c.Request.Context(), servicequestion.UpdateQuestionInput{
		Permission:      permissionContext,
		TenantID:        request.TenantID,
		QuestionID:      questionID,
		Type:            request.Type,
		Difficulty:      request.Difficulty,
		Title:           request.Title,
		Analysis:        request.Analysis,
		StandardAnswer:  request.StandardAnswer,
		ReferenceAnswer: request.ReferenceAnswer,
		BlankCount:      request.BlankCount,
		ScoreDefault:    request.ScoreDefault,
		Options:         request.toServiceOptions(),
		Tags:            request.Tags,
	})
	if err != nil {
		writeQuestionServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(questionToResponse(updated)))
}

func (h questionHandler) disable(c *gin.Context) {
	h.updateStatus(c, servicequestion.QuestionStatusDisabled)
}

func (h questionHandler) enable(c *gin.Context) {
	h.updateStatus(c, servicequestion.QuestionStatusEnabled)
}

func (h questionHandler) updateStatus(c *gin.Context, status string) {
	questionID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "题目 ID 必须是正整数"))
		return
	}
	var request questionTenantRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "请求体不是合法 JSON"))
		return
	}
	if request.TenantID == 0 {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id 必须是正整数"))
		return
	}
	if !authorizeExamBusiness(c, request.TenantID, h.members) {
		return
	}
	permissionContext, err := h.permissionContextForQuestionOperation(c, request.TenantID)
	if err != nil {
		writePermissionOrInternalError(c, err, "构建题库权限上下文失败")
		return
	}
	updated, err := h.service.UpdateQuestionStatus(c.Request.Context(), servicequestion.UpdateQuestionStatusInput{
		Permission: permissionContext,
		TenantID:   request.TenantID,
		QuestionID: questionID,
		Status:     status,
	})
	if err != nil {
		writeQuestionServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(questionToResponse(updated)))
}

func (h questionHandler) delete(c *gin.Context) {
	questionID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "题目 ID 必须是正整数"))
		return
	}
	var request questionTenantRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "请求体不是合法 JSON"))
		return
	}
	if request.TenantID == 0 {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id 必须是正整数"))
		return
	}
	if !authorizeExamBusiness(c, request.TenantID, h.members) {
		return
	}
	permissionContext, err := h.permissionContextForQuestionOperation(c, request.TenantID)
	if err != nil {
		writePermissionOrInternalError(c, err, "构建题库权限上下文失败")
		return
	}
	if err := h.service.DeleteQuestion(c.Request.Context(), servicequestion.DeleteQuestionInput{
		Permission: permissionContext,
		TenantID:   request.TenantID,
		QuestionID: questionID,
	}); err != nil {
		writeQuestionServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(nil))
}

func (h questionHandler) permissionContextForQuestionOperation(c *gin.Context, tenantID uint64) (permission.PermissionContext, error) {
	principal, err := liveTenantPrincipalFromSession(c, tenantID, h.users)
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
	if principal.Role == permission.RoleTenantAdmin || h.members == nil {
		return ctx, nil
	}
	memberships, err := h.members.ListEffectiveMembershipsForUser(c.Request.Context(), tenantID, principal.UserID)
	if err != nil {
		return permission.PermissionContext{}, err
	}
	for _, member := range memberships {
		ctx.SpaceMemberships[member.SpaceID] = member.Role
	}
	return ctx, nil
}

func (r createQuestionRequest) validate() error {
	if r.TenantID == 0 {
		return errors.New("tenant_id 必须是正整数")
	}
	if r.Title == "" {
		return errors.New("title 不能为空")
	}
	if r.Type == "" {
		return errors.New("type 不能为空")
	}
	if r.Difficulty == "" {
		return errors.New("difficulty 不能为空")
	}
	return nil
}

func (r createQuestionRequest) toServiceOptions() []servicequestion.QuestionOptionInput {
	options := make([]servicequestion.QuestionOptionInput, 0, len(r.Options))
	for _, option := range r.Options {
		options = append(options, servicequestion.QuestionOptionInput{
			OptionKey:    option.OptionKey,
			Content:      option.Content,
			IsCorrect:    option.IsCorrect,
			IsDistractor: option.IsDistractor,
		})
	}
	return options
}

func questionToResponse(item servicequestion.Question) questionResponse {
	options := make([]questionOptionResponse, 0, len(item.Options))
	for _, option := range item.Options {
		options = append(options, questionOptionResponse{
			ID:           option.ID,
			OptionKey:    option.OptionKey,
			Content:      option.Content,
			IsCorrect:    option.IsCorrect,
			IsDistractor: option.IsDistractor,
		})
	}
	tag := ""
	if len(item.Tags) > 0 {
		tag = item.Tags[0]
	}
	return questionResponse{
		ID:              item.ID,
		TenantID:        item.TenantID,
		SpaceID:         item.SpaceID,
		Type:            item.Type,
		Difficulty:      item.Difficulty,
		Title:           item.Title,
		Analysis:        item.Analysis,
		StandardAnswer:  item.StandardAnswer,
		ReferenceAnswer: item.ReferenceAnswer,
		ScoreDefault:    item.ScoreDefault,
		Status:          item.Status,
		AuthorName:      item.AuthorName,
		AuthorRole:      item.AuthorRole,
		CreatedAt:       item.CreatedAt,
		Tag:             tag,
		Tags:            item.Tags,
		Options:         options,
	}
}

func writeQuestionServiceError(c *gin.Context, err error) {
	if errors.Is(err, permission.ErrForbidden) {
		c.JSON(http.StatusForbidden, response.Fail(code.InvalidParam, "无权执行当前操作"))
		return
	}
	if errors.Is(err, servicequestion.ErrQuestionNotFound) {
		c.JSON(http.StatusNotFound, response.Fail(code.InvalidParam, "题目不存在"))
		return
	}
	if errors.Is(err, servicequestion.ErrQuestionReferenced) {
		c.JSON(http.StatusConflict, response.Fail(code.InvalidParam, "题目已被试卷或考试引用，不能编辑或删除"))
		return
	}
	if errors.Is(err, servicequestion.ErrChoiceQuestionNeedsCorrectAnswer) ||
		errors.Is(err, servicequestion.ErrSingleQuestionOnlyOneCorrectAnswer) ||
		errors.Is(err, servicequestion.ErrUnsupportedQuestionType) ||
		errors.Is(err, servicequestion.ErrUnsupportedDifficulty) ||
		errors.Is(err, servicequestion.ErrUnsupportedQuestionStatus) ||
		errors.Is(err, servicequestion.ErrFillBlankNeedsStandardAnswer) {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, err.Error()))
		return
	}
	c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "题目操作失败"))
}

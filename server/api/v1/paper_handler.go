package v1

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	servicepaper "github.com/lifei6671/papermind/server/internal/service/paper"
	"github.com/lifei6671/papermind/server/internal/service/permission"
	servicespace "github.com/lifei6671/papermind/server/internal/service/space"
	servicetenantuser "github.com/lifei6671/papermind/server/internal/service/tenantuser"
	"github.com/lifei6671/papermind/server/library/code"
	"github.com/lifei6671/papermind/server/library/response"
)

type paperHandler struct {
	service *servicepaper.Service
	papers  paperScopeFinder
	members spaceMemberFinder
	users   *servicetenantuser.Service
}

type paperScopeFinder interface {
	GetPaperSpaceID(ctx context.Context, tenantID uint64, paperID uint64) (*uint64, error)
}

type paperResponse struct {
	ID               uint64  `json:"id"`
	TenantID         uint64  `json:"tenant_id"`
	SpaceID          *uint64 `json:"space_id,omitempty"`
	Name             string  `json:"name"`
	Description      string  `json:"description"`
	DurationMinutes  int     `json:"duration_minutes"`
	GradeLevel       string  `json:"grade_level"`
	TotalScore       string  `json:"total_score"`
	BuildMode        string  `json:"build_mode"`
	ShuffleQuestions bool    `json:"shuffle_questions"`
	ShowAnalysis     bool    `json:"show_analysis"`
	Status           string  `json:"status"`
	CreatedAt        int64   `json:"created_at"`
	CreatorName      string  `json:"creator_name"`
}

type paperListResponse struct {
	Items []paperResponse `json:"items"`
}

type paperSectionResponse struct {
	ID            uint64 `json:"id"`
	TenantID      uint64 `json:"tenant_id"`
	PaperID       uint64 `json:"paper_id"`
	SortOrder     int    `json:"sort_order"`
	Name          string `json:"name"`
	QuestionType  string `json:"question_type"`
	Instructions  string `json:"instructions"`
	TotalScore    string `json:"total_score"`
	QuestionCount int    `json:"question_count"`
}

type paperSectionListResponse struct {
	Items []paperSectionResponse `json:"items"`
}

type paperSectionQuestionResponse struct {
	TenantID   uint64 `json:"tenant_id"`
	PaperID    uint64 `json:"paper_id"`
	SectionID  uint64 `json:"section_id"`
	QuestionID uint64 `json:"question_id"`
	SortOrder  int    `json:"sort_order"`
	Score      string `json:"score"`
}

type paperSectionQuestionListResponse struct {
	Items []paperSectionQuestionResponse `json:"items"`
}

type paperRuleResponse struct {
	ID                         uint64                             `json:"id"`
	TenantID                   uint64                             `json:"tenant_id"`
	PaperID                    uint64                             `json:"paper_id"`
	SectionID                  uint64                             `json:"section_id"`
	SortOrder                  int                                `json:"sort_order"`
	Difficulty                 *string                            `json:"difficulty"`
	TagIDs                     []uint64                           `json:"tag_ids"`
	TagNames                   []string                           `json:"tag_names"`
	QuestionScope              string                             `json:"question_scope"`
	DifficultyPercentages      servicepaper.DifficultyPercentages `json:"difficulty_percentages"`
	QuestionCount              int                                `json:"question_count"`
	ScorePerQuestion           string                             `json:"score_per_question"`
	ShuffleOptions             *bool                              `json:"shuffle_options"`
	PrioritizeQuality          bool                               `json:"prioritize_quality"`
	ExcludeRecentExamQuestions bool                               `json:"exclude_recent_exam_questions"`
	ExcludeUsedQuestions       bool                               `json:"exclude_used_questions"`
}

type paperRuleListResponse struct {
	Items []paperRuleResponse `json:"items"`
}

type paperRuleFixedGenerateResponse struct {
	PaperID   uint64 `json:"paper_id"`
	Generated bool   `json:"generated"`
}

type paperRuleLivePrecheckResponse struct {
	CandidateQuestionIDs []uint64 `json:"candidate_question_ids"`
	CandidateCount       int      `json:"candidate_count"`
}

type createPaperSectionRequest struct {
	TenantID     uint64 `json:"tenant_id"`
	Name         string `json:"name"`
	QuestionType string `json:"question_type"`
	Instructions string `json:"instructions"`
}

type paperSectionOrderRequest struct {
	SectionID uint64 `json:"section_id"`
	SortOrder int    `json:"sort_order"`
}

type reorderPaperSectionsRequest struct {
	TenantID uint64                     `json:"tenant_id"`
	Orders   []paperSectionOrderRequest `json:"orders"`
}

type createPaperRequest struct {
	TenantID         uint64  `json:"tenant_id"`
	SpaceID          *uint64 `json:"space_id"`
	Name             string  `json:"name"`
	Description      string  `json:"description"`
	DurationMinutes  *int    `json:"duration_minutes"`
	GradeLevel       string  `json:"grade_level"`
	ShuffleQuestions bool    `json:"shuffle_questions"`
	ShowAnalysis     bool    `json:"show_analysis"`
}

type updatePaperRequest struct {
	TenantID        uint64 `json:"tenant_id"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	DurationMinutes *int   `json:"duration_minutes"`
	GradeLevel      string `json:"grade_level"`
}

type paperTenantRequest struct {
	TenantID uint64 `json:"tenant_id"`
}

type addPaperSectionQuestionRequest struct {
	TenantID   uint64 `json:"tenant_id"`
	QuestionID uint64 `json:"question_id"`
	Score      string `json:"score"`
}

type createPaperRuleRequest struct {
	TenantID                   uint64                             `json:"tenant_id"`
	SortOrder                  int                                `json:"sort_order"`
	Difficulty                 *string                            `json:"difficulty"`
	TagIDs                     []uint64                           `json:"tag_ids"`
	TagNames                   []string                           `json:"tag_names"`
	QuestionScope              string                             `json:"question_scope"`
	DifficultyPercentages      servicepaper.DifficultyPercentages `json:"difficulty_percentages"`
	QuestionCount              int                                `json:"question_count"`
	ScorePerQuestion           string                             `json:"score_per_question"`
	ShuffleOptions             *bool                              `json:"shuffle_options"`
	PrioritizeQuality          bool                               `json:"prioritize_quality"`
	ExcludeRecentExamQuestions bool                               `json:"exclude_recent_exam_questions"`
	ExcludeUsedQuestions       bool                               `json:"exclude_used_questions"`
}

type paperRuleActionRequest struct {
	TenantID uint64 `json:"tenant_id"`
}

type updatePaperSectionQuestionRequest struct {
	TenantID  uint64 `json:"tenant_id"`
	SortOrder int    `json:"sort_order"`
	Score     string `json:"score"`
}

type replacePaperSectionQuestionRequest struct {
	TenantID      uint64 `json:"tenant_id"`
	NewQuestionID uint64 `json:"new_question_id"`
	SortOrder     int    `json:"sort_order"`
	Score         string `json:"score"`
}

type updatePaperRuleRequest struct {
	TenantID                   uint64                             `json:"tenant_id"`
	SectionID                  uint64                             `json:"section_id"`
	SortOrder                  int                                `json:"sort_order"`
	Difficulty                 *string                            `json:"difficulty"`
	TagIDs                     []uint64                           `json:"tag_ids"`
	TagNames                   []string                           `json:"tag_names"`
	QuestionScope              string                             `json:"question_scope"`
	DifficultyPercentages      servicepaper.DifficultyPercentages `json:"difficulty_percentages"`
	QuestionCount              int                                `json:"question_count"`
	ScorePerQuestion           string                             `json:"score_per_question"`
	ShuffleOptions             *bool                              `json:"shuffle_options"`
	PrioritizeQuality          bool                               `json:"prioritize_quality"`
	ExcludeRecentExamQuestions bool                               `json:"exclude_recent_exam_questions"`
	ExcludeUsedQuestions       bool                               `json:"exclude_used_questions"`
}

type updatePaperBuildModeRequest struct {
	TenantID  uint64 `json:"tenant_id"`
	BuildMode string `json:"build_mode"`
}

func (h paperHandler) list(c *gin.Context) {
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
		writePermissionOrInternalError(c, err, "构建试卷权限上下文失败")
		return
	}
	papers, err := h.service.ListPapers(c.Request.Context(), servicepaper.ListPapersInput{TenantID: tenantID, SpaceID: spaceID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "读取试卷列表失败"))
		return
	}
	items := make([]paperResponse, 0, len(papers))
	for _, paper := range papers {
		items = append(items, paperToResponse(paper))
	}
	c.JSON(http.StatusOK, response.OK(paperListResponse{Items: items}))
}

func (h paperHandler) create(c *gin.Context) {
	var request createPaperRequest
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
	if !h.authorizePaperCreate(c, request.TenantID, request.SpaceID) {
		return
	}
	principal, err := liveTenantPrincipalFromSession(c, request.TenantID, h.users)
	if err != nil {
		writePermissionOrInternalError(c, err, "构建试卷写权限上下文失败")
		return
	}
	// 创建接口只生成草稿试卷壳；大题、选题和规则仍由后续组卷接口维护。
	durationMinutes := 120
	if request.DurationMinutes != nil {
		durationMinutes = *request.DurationMinutes
	}
	paper, err := h.service.CreateManualPaper(c.Request.Context(), servicepaper.CreatePaperInput{
		TenantID:         request.TenantID,
		SpaceID:          request.SpaceID,
		Name:             request.Name,
		Description:      request.Description,
		DurationMinutes:  durationMinutes,
		GradeLevel:       request.GradeLevel,
		ShuffleQuestions: request.ShuffleQuestions,
		ShowAnalysis:     request.ShowAnalysis,
		ActorID:          principal.UserID,
	})
	if err != nil {
		writePaperServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(paperToResponse(paper)))
}

func (h paperHandler) update(c *gin.Context) {
	paperID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "试卷 ID 必须是正整数"))
		return
	}
	var request updatePaperRequest
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
	if !h.authorizePaperWrite(c, request.TenantID, paperID) {
		return
	}
	principal, err := liveTenantPrincipalFromSession(c, request.TenantID, h.users)
	if err != nil {
		writePermissionOrInternalError(c, err, "构建试卷写权限上下文失败")
		return
	}
	paper, err := h.service.UpdatePaper(c.Request.Context(), servicepaper.UpdatePaperInput{
		TenantID:        request.TenantID,
		PaperID:         paperID,
		Name:            request.Name,
		Description:     request.Description,
		DurationMinutes: request.DurationMinutes,
		GradeLevel:      request.GradeLevel,
		ActorID:         principal.UserID,
	})
	if err != nil {
		writePaperServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(paperToResponse(paper)))
}

func (h paperHandler) delete(c *gin.Context) {
	paperID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "试卷 ID 必须是正整数"))
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
	if !h.authorizePaperWrite(c, tenantID, paperID) {
		return
	}
	if err := h.service.DeletePaper(c.Request.Context(), tenantID, paperID); err != nil {
		writePaperServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(gin.H{"deleted": true}))
}

func (h paperHandler) disable(c *gin.Context) {
	h.updateStatus(c, servicepaper.StatusDisabled)
}

func (h paperHandler) enable(c *gin.Context) {
	h.updateStatus(c, servicepaper.StatusEnabled)
}

func (h paperHandler) updateStatus(c *gin.Context, status string) {
	paperID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "试卷 ID 必须是正整数"))
		return
	}
	var request paperTenantRequest
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
	if !h.authorizePaperWrite(c, request.TenantID, paperID) {
		return
	}
	principal, err := liveTenantPrincipalFromSession(c, request.TenantID, h.users)
	if err != nil {
		writePermissionOrInternalError(c, err, "构建试卷写权限上下文失败")
		return
	}
	paper, err := h.service.UpdatePaperStatus(c.Request.Context(), servicepaper.UpdatePaperStatusInput{
		TenantID: request.TenantID,
		PaperID:  paperID,
		Status:   status,
		ActorID:  principal.UserID,
	})
	if err != nil {
		writePaperServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(paperToResponse(paper)))
}

func (h paperHandler) listSections(c *gin.Context) {
	paperID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "试卷 ID 必须是正整数"))
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
	if !h.authorizePaperRead(c, tenantID, paperID) {
		return
	}
	sections, err := h.service.ListSections(c.Request.Context(), tenantID, paperID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "读取试卷大题失败"))
		return
	}
	items := make([]paperSectionResponse, 0, len(sections))
	for _, section := range sections {
		items = append(items, sectionToResponse(section))
	}
	c.JSON(http.StatusOK, response.OK(paperSectionListResponse{Items: items}))
}

func (h paperHandler) createSection(c *gin.Context) {
	paperID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "试卷 ID 必须是正整数"))
		return
	}
	var request createPaperSectionRequest
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
	if !h.authorizePaperWrite(c, request.TenantID, paperID) {
		return
	}
	sections, err := h.service.ListSections(c.Request.Context(), request.TenantID, paperID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "读取试卷大题失败"))
		return
	}
	// 新增大题默认追加到当前试卷大题末尾，避免前端重复维护 sort_order。
	section, err := h.service.CreateSection(c.Request.Context(), servicepaper.CreateSectionInput{
		TenantID:     request.TenantID,
		PaperID:      paperID,
		SortOrder:    len(sections) + 1,
		Name:         request.Name,
		QuestionType: request.QuestionType,
		Instructions: request.Instructions,
	})
	if err != nil {
		if errors.Is(err, servicepaper.ErrExamMustBeWithdrawnBeforeRuleChange) {
			c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "创建试卷大题失败"))
		return
	}
	c.JSON(http.StatusOK, response.OK(sectionToResponse(section)))
}

func (h paperHandler) reorderSections(c *gin.Context) {
	paperID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "试卷 ID 必须是正整数"))
		return
	}
	var request reorderPaperSectionsRequest
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
	if !h.authorizePaperWrite(c, request.TenantID, paperID) {
		return
	}
	orders := make([]servicepaper.SectionOrder, 0, len(request.Orders))
	for _, item := range request.Orders {
		orders = append(orders, servicepaper.SectionOrder{
			SectionID: item.SectionID,
			SortOrder: item.SortOrder,
		})
	}
	if err := h.service.ReorderSections(c.Request.Context(), request.TenantID, paperID, orders); err != nil {
		if errors.Is(err, servicepaper.ErrExamMustBeWithdrawnBeforeRuleChange) {
			c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, err.Error()))
			return
		}
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "重排试卷大题失败"))
		return
	}
	c.JSON(http.StatusOK, response.OK(nil))
}

func (h paperHandler) addManualQuestion(c *gin.Context) {
	paperID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "试卷 ID 必须是正整数"))
		return
	}
	sectionID, err := readUintParam(c, "section_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "大题 ID 必须是正整数"))
		return
	}
	var request addPaperSectionQuestionRequest
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
	if !h.authorizePaperWrite(c, request.TenantID, paperID) {
		return
	}
	questions, err := h.service.ListSectionQuestions(c.Request.Context(), request.TenantID, paperID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "读取试卷题目失败"))
		return
	}
	question := servicepaper.AddSectionQuestionInput{
		TenantID:   request.TenantID,
		PaperID:    paperID,
		SectionID:  sectionID,
		QuestionID: request.QuestionID,
		SortOrder:  nextSectionQuestionSortOrder(questions, sectionID),
		Score:      request.Score,
	}
	// 手动选题写入 paper_section_questions 后，由 service/repository 同事务重算大题和试卷总分。
	if err := h.service.AddManualQuestion(c.Request.Context(), question); err != nil {
		writePaperServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(sectionQuestionToResponse(servicepaper.SectionQuestion{
		TenantID:   question.TenantID,
		PaperID:    question.PaperID,
		SectionID:  question.SectionID,
		QuestionID: question.QuestionID,
		SortOrder:  question.SortOrder,
		Score:      question.Score,
	})))
}

func (h paperHandler) listSectionQuestions(c *gin.Context) {
	paperID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "试卷 ID 必须是正整数"))
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
	if !h.authorizePaperRead(c, tenantID, paperID) {
		return
	}
	questions, err := h.service.ListSectionQuestions(c.Request.Context(), tenantID, paperID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "读取试卷题目失败"))
		return
	}
	items := make([]paperSectionQuestionResponse, 0, len(questions))
	for _, question := range questions {
		items = append(items, sectionQuestionToResponse(question))
	}
	c.JSON(http.StatusOK, response.OK(paperSectionQuestionListResponse{Items: items}))
}

func (h paperHandler) updateSectionQuestion(c *gin.Context) {
	paperID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "试卷 ID 必须是正整数"))
		return
	}
	sectionID, err := readUintParam(c, "section_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "大题 ID 必须是正整数"))
		return
	}
	questionID, err := readUintParam(c, "question_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "题目 ID 必须是正整数"))
		return
	}
	var request updatePaperSectionQuestionRequest
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
	if !h.authorizePaperWrite(c, request.TenantID, paperID) {
		return
	}
	if err := h.service.UpdateSectionQuestion(c.Request.Context(), servicepaper.UpdateSectionQuestionInput{
		TenantID:   request.TenantID,
		PaperID:    paperID,
		SectionID:  sectionID,
		QuestionID: questionID,
		SortOrder:  request.SortOrder,
		Score:      request.Score,
	}); err != nil {
		writePaperServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(sectionQuestionToResponse(servicepaper.SectionQuestion{
		TenantID:   request.TenantID,
		PaperID:    paperID,
		SectionID:  sectionID,
		QuestionID: questionID,
		SortOrder:  request.SortOrder,
		Score:      request.Score,
	})))
}

func (h paperHandler) replaceSectionQuestion(c *gin.Context) {
	paperID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "试卷 ID 必须是正整数"))
		return
	}
	sectionID, err := readUintParam(c, "section_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "大题 ID 必须是正整数"))
		return
	}
	questionID, err := readUintParam(c, "question_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "题目 ID 必须是正整数"))
		return
	}
	var request replacePaperSectionQuestionRequest
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
	if !h.authorizePaperWrite(c, request.TenantID, paperID) {
		return
	}
	if err := h.service.ReplaceGeneratedQuestion(c.Request.Context(), servicepaper.ReplaceGeneratedQuestionInput{
		TenantID:      request.TenantID,
		PaperID:       paperID,
		SectionID:     sectionID,
		OldQuestionID: questionID,
		NewQuestionID: request.NewQuestionID,
		SortOrder:     request.SortOrder,
		Score:         request.Score,
	}); err != nil {
		writePaperServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(sectionQuestionToResponse(servicepaper.SectionQuestion{
		TenantID:   request.TenantID,
		PaperID:    paperID,
		SectionID:  sectionID,
		QuestionID: request.NewQuestionID,
		SortOrder:  request.SortOrder,
		Score:      request.Score,
	})))
}

func (h paperHandler) deleteSectionQuestion(c *gin.Context) {
	paperID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "试卷 ID 必须是正整数"))
		return
	}
	sectionID, err := readUintParam(c, "section_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "大题 ID 必须是正整数"))
		return
	}
	questionID, err := readUintParam(c, "question_id")
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
	if !h.authorizePaperWrite(c, tenantID, paperID) {
		return
	}
	if err := h.service.DeleteSectionQuestion(c.Request.Context(), tenantID, paperID, sectionID, questionID); err != nil {
		writePaperServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(gin.H{"deleted": true}))
}

func (h paperHandler) listRules(c *gin.Context) {
	paperID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "试卷 ID 必须是正整数"))
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
	if !h.authorizePaperRead(c, tenantID, paperID) {
		return
	}
	rules, err := h.service.ListRules(c.Request.Context(), tenantID, paperID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "读取组卷规则失败"))
		return
	}
	items := make([]paperRuleResponse, 0, len(rules))
	for _, rule := range rules {
		items = append(items, ruleToResponse(rule))
	}
	c.JSON(http.StatusOK, response.OK(paperRuleListResponse{Items: items}))
}

func (h paperHandler) createRule(c *gin.Context) {
	paperID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "试卷 ID 必须是正整数"))
		return
	}
	sectionID, err := readUintParam(c, "section_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "大题 ID 必须是正整数"))
		return
	}
	var request createPaperRuleRequest
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
	if !h.authorizePaperWrite(c, request.TenantID, paperID) {
		return
	}
	// 组卷规则持久化为 paper_section_rules，tag_ids 在 service 层稳定排序为 JSON，便于后续规则生成和预检查复用同一来源。
	rule, err := h.service.ConfigureRule(c.Request.Context(), servicepaper.ConfigureRuleInput{
		TenantID:                   request.TenantID,
		PaperID:                    paperID,
		SectionID:                  sectionID,
		SortOrder:                  request.SortOrder,
		Difficulty:                 request.Difficulty,
		TagIDs:                     request.TagIDs,
		TagNames:                   request.TagNames,
		QuestionScope:              request.QuestionScope,
		DifficultyPercentages:      request.DifficultyPercentages,
		QuestionCount:              request.QuestionCount,
		ScorePerQuestion:           request.ScorePerQuestion,
		ShuffleOptions:             request.ShuffleOptions,
		PrioritizeQuality:          request.PrioritizeQuality,
		ExcludeRecentExamQuestions: request.ExcludeRecentExamQuestions,
		ExcludeUsedQuestions:       request.ExcludeUsedQuestions,
	})
	if err != nil {
		writePaperServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(ruleToResponse(rule)))
}

func (h paperHandler) generateRuleFixed(c *gin.Context) {
	paperID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "试卷 ID 必须是正整数"))
		return
	}
	var request paperRuleActionRequest
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
	if !h.authorizePaperWrite(c, request.TenantID, paperID) {
		return
	}
	if err := h.service.GenerateRuleFixed(c.Request.Context(), request.TenantID, paperID); err != nil {
		writePaperServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(paperRuleFixedGenerateResponse{PaperID: paperID, Generated: true}))
}

func (h paperHandler) precheckRuleLive(c *gin.Context) {
	paperID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "试卷 ID 必须是正整数"))
		return
	}
	var request paperRuleActionRequest
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
	if !h.authorizePaperWrite(c, request.TenantID, paperID) {
		return
	}
	rules, err := h.service.ListRules(c.Request.Context(), request.TenantID, paperID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "读取组卷规则失败"))
		return
	}
	result, err := h.service.PrecheckRuleLive(c.Request.Context(), request.TenantID, paperID, rules)
	if err != nil {
		writePaperServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(paperRuleLivePrecheckResponse{
		CandidateQuestionIDs: result.CandidateQuestionIDs,
		CandidateCount:       len(result.CandidateQuestionIDs),
	}))
}

func (h paperHandler) updateRule(c *gin.Context) {
	paperID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "试卷 ID 必须是正整数"))
		return
	}
	ruleID, err := readUintParam(c, "rule_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "规则 ID 必须是正整数"))
		return
	}
	var request updatePaperRuleRequest
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
	if !h.authorizePaperWrite(c, request.TenantID, paperID) {
		return
	}
	if err := h.service.UpdateRuleLiveRule(c.Request.Context(), servicepaper.UpdateRuleInput{
		TenantID:                   request.TenantID,
		PaperID:                    paperID,
		RuleID:                     ruleID,
		SectionID:                  request.SectionID,
		SortOrder:                  request.SortOrder,
		Difficulty:                 request.Difficulty,
		TagIDs:                     request.TagIDs,
		TagNames:                   request.TagNames,
		QuestionScope:              request.QuestionScope,
		DifficultyPercentages:      request.DifficultyPercentages,
		QuestionCount:              request.QuestionCount,
		ScorePerQuestion:           request.ScorePerQuestion,
		ShuffleOptions:             request.ShuffleOptions,
		PrioritizeQuality:          request.PrioritizeQuality,
		ExcludeRecentExamQuestions: request.ExcludeRecentExamQuestions,
		ExcludeUsedQuestions:       request.ExcludeUsedQuestions,
	}); err != nil {
		writePaperServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(ruleToResponse(servicepaper.Rule{
		ID:                         ruleID,
		TenantID:                   request.TenantID,
		PaperID:                    paperID,
		SectionID:                  request.SectionID,
		SortOrder:                  request.SortOrder,
		Difficulty:                 request.Difficulty,
		TagFilter:                  servicepaperTagFilterJSON(request.TagIDs),
		TagNames:                   request.TagNames,
		QuestionScope:              request.QuestionScope,
		DifficultyPercentages:      request.DifficultyPercentages,
		QuestionCount:              request.QuestionCount,
		ScorePerQuestion:           request.ScorePerQuestion,
		ShuffleOptions:             request.ShuffleOptions,
		PrioritizeQuality:          request.PrioritizeQuality,
		ExcludeRecentExamQuestions: request.ExcludeRecentExamQuestions,
		ExcludeUsedQuestions:       request.ExcludeUsedQuestions,
	})))
}

func (h paperHandler) deleteRule(c *gin.Context) {
	paperID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "试卷 ID 必须是正整数"))
		return
	}
	ruleID, err := readUintParam(c, "rule_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "规则 ID 必须是正整数"))
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
	if !h.authorizePaperWrite(c, tenantID, paperID) {
		return
	}
	if err := h.service.DeleteRule(c.Request.Context(), tenantID, paperID, ruleID); err != nil {
		writePaperServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(gin.H{"deleted": true}))
}

func (h paperHandler) updateBuildMode(c *gin.Context) {
	paperID, err := readUintParam(c, "id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "试卷 ID 必须是正整数"))
		return
	}
	var request updatePaperBuildModeRequest
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
	if !h.authorizePaperWrite(c, request.TenantID, paperID) {
		return
	}
	paper, err := h.service.UpdateBuildMode(c.Request.Context(), servicepaper.UpdateBuildModeInput{
		TenantID:  request.TenantID,
		PaperID:   paperID,
		BuildMode: request.BuildMode,
	})
	if err != nil {
		writePaperServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, response.OK(paperToResponse(paper)))
}

func (r createPaperSectionRequest) validate() error {
	if r.TenantID == 0 {
		return errors.New("tenant_id 必须是正整数")
	}
	if r.Name == "" {
		return errors.New("name 不能为空")
	}
	if r.QuestionType == "" {
		return errors.New("question_type 不能为空")
	}
	return nil
}

func (r reorderPaperSectionsRequest) validate() error {
	if r.TenantID == 0 {
		return errors.New("tenant_id 必须是正整数")
	}
	if len(r.Orders) == 0 {
		return errors.New("orders 不能为空")
	}
	for _, item := range r.Orders {
		if item.SectionID == 0 {
			return errors.New("section_id 必须是正整数")
		}
		if item.SortOrder <= 0 {
			return errors.New("sort_order 必须是正整数")
		}
	}
	return nil
}

func (r createPaperRequest) validate() error {
	if r.TenantID == 0 {
		return errors.New("tenant_id 必须是正整数")
	}
	if r.SpaceID != nil && *r.SpaceID == 0 {
		return errors.New("space_id 必须是正整数")
	}
	if r.Name == "" {
		return errors.New("name 不能为空")
	}
	if r.DurationMinutes != nil && *r.DurationMinutes <= 0 {
		return errors.New("duration_minutes 必须是正整数")
	}
	return nil
}

func (r updatePaperRequest) validate() error {
	if r.TenantID == 0 {
		return errors.New("tenant_id 必须是正整数")
	}
	if r.Name == "" {
		return errors.New("name 不能为空")
	}
	if r.DurationMinutes != nil && *r.DurationMinutes <= 0 {
		return errors.New("duration_minutes 必须是正整数")
	}
	return nil
}

func (r addPaperSectionQuestionRequest) validate() error {
	if r.TenantID == 0 {
		return errors.New("tenant_id 必须是正整数")
	}
	if r.QuestionID == 0 {
		return errors.New("question_id 必须是正整数")
	}
	if r.Score == "" {
		return errors.New("score 不能为空")
	}
	if err := validateNonNegativeScore("score", r.Score); err != nil {
		return err
	}
	return nil
}

func (r createPaperRuleRequest) validate() error {
	if r.TenantID == 0 {
		return errors.New("tenant_id 必须是正整数")
	}
	if r.SortOrder <= 0 {
		return errors.New("sort_order 必须是正整数")
	}
	if r.QuestionCount <= 0 {
		return errors.New("question_count 必须是正整数")
	}
	if r.ScorePerQuestion == "" {
		return errors.New("score_per_question 不能为空")
	}
	if err := validateNonNegativeScore("score_per_question", r.ScorePerQuestion); err != nil {
		return err
	}
	return validateDifficultyPercentages(r.DifficultyPercentages)
}

func validateDifficultyPercentages(percentages servicepaper.DifficultyPercentages) error {
	if percentages.Easy < 0 || percentages.Medium < 0 || percentages.Hard < 0 {
		return errors.New("difficulty_percentages 不能包含负数")
	}
	total := percentages.Easy + percentages.Medium + percentages.Hard
	if total != 0 && total != 100 {
		return errors.New("difficulty_percentages 总和必须为 100")
	}
	return nil
}

func validateNonNegativeScore(field string, value string) error {
	parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) || parsed < 0 {
		return errors.New(field + " 必须是非负数字")
	}
	return nil
}

func (r paperRuleActionRequest) validate() error {
	if r.TenantID == 0 {
		return errors.New("tenant_id 必须是正整数")
	}
	return nil
}

func (r paperTenantRequest) validate() error {
	if r.TenantID == 0 {
		return errors.New("tenant_id 必须是正整数")
	}
	return nil
}

func (r updatePaperSectionQuestionRequest) validate() error {
	if r.TenantID == 0 {
		return errors.New("tenant_id 必须是正整数")
	}
	if r.SortOrder <= 0 {
		return errors.New("sort_order 必须是正整数")
	}
	if r.Score == "" {
		return errors.New("score 不能为空")
	}
	return validateNonNegativeScore("score", r.Score)
}

func (r replacePaperSectionQuestionRequest) validate() error {
	if r.TenantID == 0 {
		return errors.New("tenant_id 必须是正整数")
	}
	if r.NewQuestionID == 0 {
		return errors.New("new_question_id 必须是正整数")
	}
	if r.SortOrder <= 0 {
		return errors.New("sort_order 必须是正整数")
	}
	if r.Score == "" {
		return errors.New("score 不能为空")
	}
	return validateNonNegativeScore("score", r.Score)
}

func (r updatePaperRuleRequest) validate() error {
	if r.TenantID == 0 {
		return errors.New("tenant_id 必须是正整数")
	}
	if r.SectionID == 0 {
		return errors.New("section_id 必须是正整数")
	}
	if r.SortOrder <= 0 {
		return errors.New("sort_order 必须是正整数")
	}
	if r.QuestionCount <= 0 {
		return errors.New("question_count 必须是正整数")
	}
	if r.ScorePerQuestion == "" {
		return errors.New("score_per_question 不能为空")
	}
	if err := validateNonNegativeScore("score_per_question", r.ScorePerQuestion); err != nil {
		return err
	}
	return validateDifficultyPercentages(r.DifficultyPercentages)
}

func (r updatePaperBuildModeRequest) validate() error {
	if r.TenantID == 0 {
		return errors.New("tenant_id 必须是正整数")
	}
	switch r.BuildMode {
	case servicepaper.BuildModeManual, servicepaper.BuildModeRuleFixed, servicepaper.BuildModeRuleLive:
		return nil
	default:
		return errors.New("build_mode 不合法")
	}
}

func (h paperHandler) authorizePaperRead(c *gin.Context, tenantID uint64, paperID uint64) bool {
	spaceID, err := h.papers.GetPaperSpaceID(c.Request.Context(), tenantID, paperID)
	if err != nil {
		if errors.Is(err, servicepaper.ErrPaperNotFound) {
			writePaperServiceError(c, err)
			return false
		}
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "读取试卷权限范围失败"))
		return false
	}
	principal, err := liveTenantPrincipalFromSession(c, tenantID, h.users)
	if err != nil {
		writePermissionOrInternalError(c, err, "校验试卷读权限失败")
		return false
	}
	if principal.Role == permission.RoleTenantAdmin {
		return true
	}
	if spaceID == nil {
		return true
	}
	permissionContext, err := permissionContextForResourceScope(c, tenantID, spaceID, h.members, h.users)
	if err != nil {
		writePermissionOrInternalError(c, err, "构建试卷权限上下文失败")
		return false
	}
	permissionContext.PaperScope = map[uint64]uint64{paperID: *spaceID}
	if err := permission.NewFixedRoleChecker().CanManagePaper(permissionContext, paperID); err != nil {
		writePermissionOrInternalError(c, err, "校验试卷读权限失败")
		return false
	}
	return true
}

func (h paperHandler) authorizePaperWrite(c *gin.Context, tenantID uint64, paperID uint64) bool {
	spaceID, err := h.papers.GetPaperSpaceID(c.Request.Context(), tenantID, paperID)
	if err != nil {
		if errors.Is(err, servicepaper.ErrPaperNotFound) {
			writePaperServiceError(c, err)
			return false
		}
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "读取试卷权限范围失败"))
		return false
	}
	permissionContext, err := permissionContextForResourceScope(c, tenantID, spaceID, h.members, h.users)
	if err != nil {
		writePermissionOrInternalError(c, err, "构建试卷权限上下文失败")
		return false
	}
	if spaceID == nil {
		if permissionContext.Role != permission.RoleTenantAdmin {
			writePermissionOrInternalError(c, permission.ErrForbidden, "公共试卷仅租户管理员可维护")
			return false
		}
		return true
	}
	permissionContext.PaperScope = map[uint64]uint64{paperID: *spaceID}
	if err := permission.NewFixedRoleChecker().CanManagePaper(permissionContext, paperID); err != nil {
		writePermissionOrInternalError(c, err, "校验试卷写权限失败")
		return false
	}
	return true
}

func (h paperHandler) authorizePaperCreate(c *gin.Context, tenantID uint64, spaceID *uint64) bool {
	permissionContext, err := permissionContextForResourceScope(c, tenantID, spaceID, h.members, h.users)
	if err != nil {
		writePermissionOrInternalError(c, err, "构建试卷权限上下文失败")
		return false
	}
	if spaceID == nil {
		if permissionContext.Role != permission.RoleTenantAdmin {
			writePermissionOrInternalError(c, permission.ErrForbidden, "公共试卷仅租户管理员可维护")
			return false
		}
		return true
	}
	exists, err := h.members.SpaceExists(c.Request.Context(), tenantID, *spaceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "读取空间权限范围失败"))
		return false
	}
	if !exists {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, servicespace.ErrSpaceNotFound.Error()))
		return false
	}
	// 新建空间试卷还没有 paper_id，使用临时作用域复用统一的试卷管理权限校验。
	permissionContext.PaperScope = map[uint64]uint64{0: *spaceID}
	if err := permission.NewFixedRoleChecker().CanManagePaper(permissionContext, 0); err != nil {
		writePermissionOrInternalError(c, err, "校验试卷写权限失败")
		return false
	}
	return true
}

func writePaperServiceError(c *gin.Context, err error) {
	if errors.Is(err, permission.ErrForbidden) {
		c.JSON(http.StatusForbidden, response.Fail(code.InvalidParam, "无权执行当前操作"))
		return
	}
	if errors.Is(err, servicepaper.ErrPaperNotFound) {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, err.Error()))
		return
	}
	if errors.Is(err, servicepaper.ErrPaperInUse) {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, err.Error()))
		return
	}
	if errors.Is(err, servicepaper.ErrDuplicatePaperQuestion) {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, err.Error()))
		return
	}
	if errors.Is(err, servicepaper.ErrQuestionOutOfScope) {
		c.JSON(http.StatusForbidden, response.Fail(code.InvalidParam, "题目不属于当前试卷可用范围"))
		return
	}
	if errors.Is(err, servicepaper.ErrQuestionPoolInsufficient) {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, err.Error()))
		return
	}
	if errors.Is(err, servicepaper.ErrExamMustBeWithdrawnBeforeRuleChange) {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, err.Error()))
		return
	}
	if errors.Is(err, servicepaper.ErrRuleLiveManualQuestionChange) {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, err.Error()))
		return
	}
	c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "试卷操作失败"))
}

func nextSectionQuestionSortOrder(questions []servicepaper.SectionQuestion, sectionID uint64) int {
	maxSortOrder := 0
	for _, question := range questions {
		if question.SectionID == sectionID && question.SortOrder > maxSortOrder {
			maxSortOrder = question.SortOrder
		}
	}
	return maxSortOrder + 1
}

func paperToResponse(paper servicepaper.Paper) paperResponse {
	return paperResponse{
		ID:               paper.ID,
		TenantID:         paper.TenantID,
		SpaceID:          paper.SpaceID,
		Name:             paper.Name,
		Description:      paper.Description,
		DurationMinutes:  paper.DurationMinutes,
		GradeLevel:       paper.GradeLevel,
		TotalScore:       paper.TotalScore,
		BuildMode:        paper.BuildMode,
		ShuffleQuestions: paper.ShuffleQuestions,
		ShowAnalysis:     paper.ShowAnalysis,
		Status:           paper.Status,
		CreatedAt:        paper.CreatedAt,
		CreatorName:      paper.CreatorName,
	}
}

func sectionQuestionToResponse(question servicepaper.SectionQuestion) paperSectionQuestionResponse {
	return paperSectionQuestionResponse{
		TenantID:   question.TenantID,
		PaperID:    question.PaperID,
		SectionID:  question.SectionID,
		QuestionID: question.QuestionID,
		SortOrder:  question.SortOrder,
		Score:      question.Score,
	}
}

func sectionToResponse(section servicepaper.Section) paperSectionResponse {
	return paperSectionResponse{
		ID:            section.ID,
		TenantID:      section.TenantID,
		PaperID:       section.PaperID,
		SortOrder:     section.SortOrder,
		Name:          section.Name,
		QuestionType:  section.QuestionType,
		Instructions:  section.Instructions,
		TotalScore:    section.TotalScore,
		QuestionCount: section.QuestionCount,
	}
}

func ruleToResponse(rule servicepaper.Rule) paperRuleResponse {
	return paperRuleResponse{
		ID:                         rule.ID,
		TenantID:                   rule.TenantID,
		PaperID:                    rule.PaperID,
		SectionID:                  rule.SectionID,
		SortOrder:                  rule.SortOrder,
		Difficulty:                 rule.Difficulty,
		TagIDs:                     tagIDsFromRule(rule),
		TagNames:                   rule.TagNames,
		QuestionScope:              rule.QuestionScope,
		DifficultyPercentages:      rule.DifficultyPercentages,
		QuestionCount:              rule.QuestionCount,
		ScorePerQuestion:           rule.ScorePerQuestion,
		ShuffleOptions:             rule.ShuffleOptions,
		PrioritizeQuality:          rule.PrioritizeQuality,
		ExcludeRecentExamQuestions: rule.ExcludeRecentExamQuestions,
		ExcludeUsedQuestions:       rule.ExcludeUsedQuestions,
	}
}

func tagIDsFromRule(rule servicepaper.Rule) []uint64 {
	var tagIDs []uint64
	if err := json.Unmarshal([]byte(rule.TagFilter), &tagIDs); err != nil {
		return []uint64{}
	}
	return tagIDs
}

func servicepaperTagFilterJSON(tagIDs []uint64) string {
	sorted := append([]uint64(nil), tagIDs...)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j] < sorted[i] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	data, _ := json.Marshal(sorted)
	return string(data)
}

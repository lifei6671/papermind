package v1

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	servicepaper "github.com/lifei6671/papermind/server/internal/service/paper"
	"github.com/lifei6671/papermind/server/internal/service/permission"
	servicespace "github.com/lifei6671/papermind/server/internal/service/space"
	"github.com/lifei6671/papermind/server/library/code"
	"github.com/lifei6671/papermind/server/library/response"
)

type paperHandler struct {
	service *servicepaper.Service
	papers  paperScopeFinder
	members spaceMemberFinder
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
	TotalScore       string  `json:"total_score"`
	BuildMode        string  `json:"build_mode"`
	ShuffleQuestions bool    `json:"shuffle_questions"`
	ShowAnalysis     bool    `json:"show_analysis"`
	Status           string  `json:"status"`
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

type paperRuleResponse struct {
	ID               uint64   `json:"id"`
	TenantID         uint64   `json:"tenant_id"`
	PaperID          uint64   `json:"paper_id"`
	SectionID        uint64   `json:"section_id"`
	SortOrder        int      `json:"sort_order"`
	Difficulty       *string  `json:"difficulty"`
	TagIDs           []uint64 `json:"tag_ids"`
	QuestionCount    int      `json:"question_count"`
	ScorePerQuestion string   `json:"score_per_question"`
	ShuffleOptions   *bool    `json:"shuffle_options"`
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

type createPaperRequest struct {
	TenantID         uint64  `json:"tenant_id"`
	SpaceID          *uint64 `json:"space_id"`
	Name             string  `json:"name"`
	Description      string  `json:"description"`
	ShuffleQuestions bool    `json:"shuffle_questions"`
	ShowAnalysis     bool    `json:"show_analysis"`
}

type addPaperSectionQuestionRequest struct {
	TenantID   uint64 `json:"tenant_id"`
	QuestionID uint64 `json:"question_id"`
	Score      string `json:"score"`
}

type createPaperRuleRequest struct {
	TenantID         uint64   `json:"tenant_id"`
	SortOrder        int      `json:"sort_order"`
	Difficulty       *string  `json:"difficulty"`
	TagIDs           []uint64 `json:"tag_ids"`
	QuestionCount    int      `json:"question_count"`
	ScorePerQuestion string   `json:"score_per_question"`
	ShuffleOptions   *bool    `json:"shuffle_options"`
}

type paperRuleActionRequest struct {
	TenantID uint64 `json:"tenant_id"`
}

func (h paperHandler) list(c *gin.Context) {
	tenantID, err := readUintQuery(c, "tenant_id")
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id 必须是正整数"))
		return
	}
	if !authorizeExamBusiness(c, tenantID) {
		return
	}
	papers, err := h.service.ListPapers(c.Request.Context(), tenantID)
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
	if !authorizeExamBusiness(c, request.TenantID) {
		return
	}
	if !h.authorizePaperCreate(c, request.TenantID, request.SpaceID) {
		return
	}
	// 创建接口只生成草稿试卷壳；大题、选题和规则仍由后续组卷接口维护。
	paper, err := h.service.CreateManualPaper(c.Request.Context(), servicepaper.CreatePaperInput{
		TenantID:         request.TenantID,
		SpaceID:          request.SpaceID,
		Name:             request.Name,
		Description:      request.Description,
		ShuffleQuestions: request.ShuffleQuestions,
		ShowAnalysis:     request.ShowAnalysis,
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
	if !authorizeExamBusiness(c, tenantID) {
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
	if !authorizeExamBusiness(c, tenantID) {
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
	if !authorizeExamBusiness(c, request.TenantID) {
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
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "创建试卷大题失败"))
		return
	}
	c.JSON(http.StatusOK, response.OK(sectionToResponse(section)))
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
	if !authorizeExamBusiness(c, request.TenantID) {
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
	if !authorizeExamBusiness(c, tenantID) {
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
	if !authorizeExamBusiness(c, request.TenantID) {
		return
	}
	if !h.authorizePaperWrite(c, request.TenantID, paperID) {
		return
	}
	// 组卷规则持久化为 paper_section_rules，tag_ids 在 service 层稳定排序为 JSON，便于后续规则生成和预检查复用同一来源。
	rule, err := h.service.ConfigureRule(c.Request.Context(), servicepaper.ConfigureRuleInput{
		TenantID:         request.TenantID,
		PaperID:          paperID,
		SectionID:        sectionID,
		SortOrder:        request.SortOrder,
		Difficulty:       request.Difficulty,
		TagIDs:           request.TagIDs,
		QuestionCount:    request.QuestionCount,
		ScorePerQuestion: request.ScorePerQuestion,
		ShuffleOptions:   request.ShuffleOptions,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "保存组卷规则失败"))
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
	if !authorizeExamBusiness(c, request.TenantID) {
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
	if !authorizeExamBusiness(c, request.TenantID) {
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
	return nil
}

func (r paperRuleActionRequest) validate() error {
	if r.TenantID == 0 {
		return errors.New("tenant_id 必须是正整数")
	}
	return nil
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
	permissionContext, err := permissionContextForResourceScope(c, tenantID, spaceID, h.members)
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
	permissionContext, err := permissionContextForResourceScope(c, tenantID, spaceID, h.members)
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
		TotalScore:       paper.TotalScore,
		BuildMode:        paper.BuildMode,
		ShuffleQuestions: paper.ShuffleQuestions,
		ShowAnalysis:     paper.ShowAnalysis,
		Status:           paper.Status,
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
		ID:               rule.ID,
		TenantID:         rule.TenantID,
		PaperID:          rule.PaperID,
		SectionID:        rule.SectionID,
		SortOrder:        rule.SortOrder,
		Difficulty:       rule.Difficulty,
		TagIDs:           tagIDsFromRule(rule),
		QuestionCount:    rule.QuestionCount,
		ScorePerQuestion: rule.ScorePerQuestion,
		ShuffleOptions:   rule.ShuffleOptions,
	}
}

func tagIDsFromRule(rule servicepaper.Rule) []uint64 {
	var tagIDs []uint64
	if err := json.Unmarshal([]byte(rule.TagFilter), &tagIDs); err != nil {
		return []uint64{}
	}
	return tagIDs
}

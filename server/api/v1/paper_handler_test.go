package v1

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/papermind/server/library/constant"
	"gorm.io/gorm"
)

func TestPaperAPIRoutesListSectionsAndAddManualQuestionWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedPaperAPITestData(t, gormDB)
	seedExamBusinessLoginAPITestData(t, gormDB)
	seedPaperTeacherSpaceAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "teacher.exam", "papermind123")

	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, authorizedRequest(http.MethodGet, "/api/v1/papers?tenant_id=10&space_id=301", nil, authHeader))
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listRecorder.Code, listRecorder.Body.String())
	}
	listBody := decodeExamAPIResponse[paperListResponse](t, listRecorder.Body.Bytes())
	if len(listBody.Data.Items) != 1 || listBody.Data.Items[0].Name != "高一语文月考试卷" {
		t.Fatalf("unexpected paper list: %#v", listBody.Data.Items)
	}
	if listBody.Data.Items[0].CreatorName != "teacher.exam" || listBody.Data.Items[0].CreatedAt != fixedAPINow {
		t.Fatalf("expected creator metadata in paper list, got %#v", listBody.Data.Items[0])
	}

	sectionsRecorder := httptest.NewRecorder()
	router.ServeHTTP(sectionsRecorder, authorizedRequest(http.MethodGet, "/api/v1/papers/100/sections?tenant_id=10", nil, authHeader))
	if sectionsRecorder.Code != http.StatusOK {
		t.Fatalf("sections status = %d, body = %s", sectionsRecorder.Code, sectionsRecorder.Body.String())
	}
	sectionsBody := decodeExamAPIResponse[paperSectionListResponse](t, sectionsRecorder.Body.Bytes())
	if len(sectionsBody.Data.Items) != 1 || sectionsBody.Data.Items[0].Name != "一、现代文阅读" {
		t.Fatalf("unexpected sections: %#v", sectionsBody.Data.Items)
	}

	createSectionPayload := []byte(fmt.Sprintf(`{
		"tenant_id": 10,
		"name": "二、语言文字运用",
		"question_type": "%s",
		"instructions": "请完成语言文字基础题"
	}`, constant.QuestionTypeSingle))
	createSectionRecorder := httptest.NewRecorder()
	router.ServeHTTP(createSectionRecorder, authorizedRequest(http.MethodPost, "/api/v1/papers/100/sections", createSectionPayload, authHeader))
	if createSectionRecorder.Code != http.StatusOK {
		t.Fatalf("create section status = %d, body = %s", createSectionRecorder.Code, createSectionRecorder.Body.String())
	}
	createSectionBody := decodeExamAPIResponse[paperSectionResponse](t, createSectionRecorder.Body.Bytes())
	if createSectionBody.Data.SortOrder != 2 || createSectionBody.Data.TotalScore != "0" {
		t.Fatalf("unexpected created section: %#v", createSectionBody.Data)
	}

	addQuestionPayload := []byte(`{
		"tenant_id": 10,
		"question_id": 101,
		"score": "4"
	}`)
	addQuestionRecorder := httptest.NewRecorder()
	router.ServeHTTP(addQuestionRecorder, authorizedRequest(http.MethodPost, "/api/v1/papers/100/sections/1/questions", addQuestionPayload, authHeader))
	if addQuestionRecorder.Code != http.StatusOK {
		t.Fatalf("add question status = %d, body = %s", addQuestionRecorder.Code, addQuestionRecorder.Body.String())
	}
	addQuestionBody := decodeExamAPIResponse[paperSectionQuestionResponse](t, addQuestionRecorder.Body.Bytes())
	if addQuestionBody.Data.QuestionID != 101 || addQuestionBody.Data.SortOrder != 1 {
		t.Fatalf("unexpected added question response: %#v", addQuestionBody.Data)
	}

	var paperTotal string
	if err := gormDB.Table("papers").Select("total_score").
		Where("tenant_id = ?", 10).
		Where("id = ?", 100).
		Scan(&paperTotal).Error; err != nil {
		t.Fatalf("read paper total score: %v", err)
	}
	if paperTotal != "4" {
		t.Fatalf("expected paper total score recalculated to 4, got %q", paperTotal)
	}
}

func TestPaperAPIRoutesUpdateEnableAndDisableWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedPaperAPITestData(t, gormDB)
	seedExamBusinessLoginAPITestData(t, gormDB)
	seedPaperTeacherSpaceAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "teacher.exam", "papermind123")

	updateRecorder := httptest.NewRecorder()
	router.ServeHTTP(updateRecorder, authorizedRequest(http.MethodPut, "/api/v1/papers/100", []byte(`{
		"tenant_id": 10,
		"name": "高一语文期末试卷",
		"description": "文学阅读与语言基础",
		"duration_minutes": 150,
		"grade_level": "高二"
	}`), authHeader))
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("update paper status = %d, body = %s", updateRecorder.Code, updateRecorder.Body.String())
	}
	updateBody := decodeExamAPIResponse[paperResponse](t, updateRecorder.Body.Bytes())
	if updateBody.Data.Name != "高一语文期末试卷" || updateBody.Data.Description != "文学阅读与语言基础" || updateBody.Data.DurationMinutes != 150 || updateBody.Data.GradeLevel != "高二" {
		t.Fatalf("unexpected updated paper response: %#v", updateBody.Data)
	}

	disableRecorder := httptest.NewRecorder()
	router.ServeHTTP(disableRecorder, authorizedRequest(http.MethodPost, "/api/v1/papers/100/disable", []byte(`{
		"tenant_id": 10
	}`), authHeader))
	if disableRecorder.Code != http.StatusOK {
		t.Fatalf("disable paper status = %d, body = %s", disableRecorder.Code, disableRecorder.Body.String())
	}
	disableBody := decodeExamAPIResponse[paperResponse](t, disableRecorder.Body.Bytes())
	if disableBody.Data.Status != "disabled" {
		t.Fatalf("expected disabled paper response, got %#v", disableBody.Data)
	}

	enableRecorder := httptest.NewRecorder()
	router.ServeHTTP(enableRecorder, authorizedRequest(http.MethodPost, "/api/v1/papers/100/enable", []byte(`{
		"tenant_id": 10
	}`), authHeader))
	if enableRecorder.Code != http.StatusOK {
		t.Fatalf("enable paper status = %d, body = %s", enableRecorder.Code, enableRecorder.Body.String())
	}
	enableBody := decodeExamAPIResponse[paperResponse](t, enableRecorder.Body.Bytes())
	if enableBody.Data.Status != "enabled" {
		t.Fatalf("expected enabled paper response, got %#v", enableBody.Data)
	}

	var saved struct {
		Name            string `gorm:"column:name"`
		Status          string `gorm:"column:status"`
		DurationMinutes int    `gorm:"column:duration_minutes"`
		GradeLevel      string `gorm:"column:grade_level"`
		UpdatedBy       uint64 `gorm:"column:updated_by"`
	}
	if err := gormDB.Table("papers").
		Select("name, status, duration_minutes, grade_level, updated_by").
		Where("tenant_id = ? AND id = ?", 10, 100).
		Scan(&saved).Error; err != nil {
		t.Fatalf("read updated paper row: %v", err)
	}
	if saved.Name != "高一语文期末试卷" || saved.Status != "enabled" || saved.DurationMinutes != 150 || saved.GradeLevel != "高二" || saved.UpdatedBy != 501 {
		t.Fatalf("unexpected persisted paper row: %#v", saved)
	}
}

func TestPaperRuleAPIRoutesConfigureGenerateAndPrecheckWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedPaperAPITestData(t, gormDB)
	seedPaperRuleAPITestData(t, gormDB)
	seedExamBusinessLoginAPITestData(t, gormDB)
	seedPaperTeacherSpaceAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "teacher.exam", "papermind123")

	createRulePayload := []byte(`{
		"tenant_id": 10,
		"sort_order": 1,
		"difficulty": "easy",
		"tag_ids": [1],
		"question_count": 1,
		"score_per_question": "4"
	}`)
	createRuleRecorder := httptest.NewRecorder()
	router.ServeHTTP(createRuleRecorder, authorizedRequest(http.MethodPost, "/api/v1/papers/100/sections/1/rules", createRulePayload, authHeader))
	if createRuleRecorder.Code != http.StatusOK {
		t.Fatalf("create rule status = %d, body = %s", createRuleRecorder.Code, createRuleRecorder.Body.String())
	}
	createRuleBody := decodeExamAPIResponse[paperRuleResponse](t, createRuleRecorder.Body.Bytes())
	if createRuleBody.Data.SectionID != 1 || createRuleBody.Data.QuestionCount != 1 || len(createRuleBody.Data.TagIDs) != 1 || createRuleBody.Data.TagIDs[0] != 1 {
		t.Fatalf("unexpected created rule: %#v", createRuleBody.Data)
	}

	listRulesRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRulesRecorder, authorizedRequest(http.MethodGet, "/api/v1/papers/100/rules?tenant_id=10", nil, authHeader))
	if listRulesRecorder.Code != http.StatusOK {
		t.Fatalf("list rules status = %d, body = %s", listRulesRecorder.Code, listRulesRecorder.Body.String())
	}
	listRulesBody := decodeExamAPIResponse[paperRuleListResponse](t, listRulesRecorder.Body.Bytes())
	if len(listRulesBody.Data.Items) != 1 || listRulesBody.Data.Items[0].ID != createRuleBody.Data.ID {
		t.Fatalf("unexpected rule list: %#v", listRulesBody.Data.Items)
	}

	generatePayload := []byte(`{"tenant_id": 10}`)
	generateRecorder := httptest.NewRecorder()
	router.ServeHTTP(generateRecorder, authorizedRequest(http.MethodPost, "/api/v1/papers/100/rule-fixed/generate", generatePayload, authHeader))
	if generateRecorder.Code != http.StatusOK {
		t.Fatalf("generate rule_fixed status = %d, body = %s", generateRecorder.Code, generateRecorder.Body.String())
	}
	generateBody := decodeExamAPIResponse[paperRuleFixedGenerateResponse](t, generateRecorder.Body.Bytes())
	if !generateBody.Data.Generated || generateBody.Data.PaperID != 100 {
		t.Fatalf("unexpected generate response: %#v", generateBody.Data)
	}

	var generatedQuestionID uint64
	if err := gormDB.Table("paper_section_questions").Select("question_id").
		Where("tenant_id = ?", 10).
		Where("paper_id = ?", 100).
		Scan(&generatedQuestionID).Error; err != nil {
		t.Fatalf("read generated question: %v", err)
	}
	if generatedQuestionID != 101 {
		t.Fatalf("expected rule_fixed to generate question 101, got %d", generatedQuestionID)
	}

	precheckRecorder := httptest.NewRecorder()
	router.ServeHTTP(precheckRecorder, authorizedRequest(http.MethodPost, "/api/v1/papers/100/rule-live/precheck", generatePayload, authHeader))
	if precheckRecorder.Code != http.StatusOK {
		t.Fatalf("precheck rule_live status = %d, body = %s", precheckRecorder.Code, precheckRecorder.Body.String())
	}
	precheckBody := decodeExamAPIResponse[paperRuleLivePrecheckResponse](t, precheckRecorder.Body.Bytes())
	if precheckBody.Data.CandidateCount != 1 || len(precheckBody.Data.CandidateQuestionIDs) != 1 || precheckBody.Data.CandidateQuestionIDs[0] != 101 {
		t.Fatalf("unexpected precheck response: %#v", precheckBody.Data)
	}
}

func TestPaperRuleFixedGenerateInsufficientPoolReturnsFriendlyMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedPaperAPITestData(t, gormDB)
	seedPaperRuleAPITestData(t, gormDB)
	seedExamBusinessLoginAPITestData(t, gormDB)
	seedPaperTeacherSpaceAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "teacher.exam", "papermind123")

	createRuleRecorder := httptest.NewRecorder()
	router.ServeHTTP(createRuleRecorder, authorizedRequest(http.MethodPost, "/api/v1/papers/100/sections/1/rules", []byte(`{
		"tenant_id": 10,
		"sort_order": 1,
		"difficulty": "easy",
		"tag_ids": [1],
		"question_count": 99,
		"score_per_question": "4"
	}`), authHeader))
	if createRuleRecorder.Code != http.StatusOK {
		t.Fatalf("create rule status = %d, body = %s", createRuleRecorder.Code, createRuleRecorder.Body.String())
	}

	generateRecorder := httptest.NewRecorder()
	router.ServeHTTP(generateRecorder, authorizedRequest(http.MethodPost, "/api/v1/papers/100/rule-fixed/generate", []byte(`{
		"tenant_id": 10
	}`), authHeader))
	if generateRecorder.Code != http.StatusBadRequest {
		t.Fatalf("generate insufficient rule_fixed status = %d, body = %s", generateRecorder.Code, generateRecorder.Body.String())
	}
	body := generateRecorder.Body.String()
	if !strings.Contains(body, "题库题量不足") || strings.Contains(body, "question pool insufficient") {
		t.Fatalf("expected friendly insufficient pool message, body = %s", body)
	}
}

func TestPaperAssemblyWorkspaceRoutesManageSectionQuestionsWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedPaperAPITestData(t, gormDB)
	seedPaperRuleAPITestData(t, gormDB)
	seedPaperAssemblyQuestionWorkspaceAPITestData(t, gormDB)
	seedExamBusinessLoginAPITestData(t, gormDB)
	seedPaperTeacherSpaceAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "teacher.exam", "papermind123")

	addQuestionRecorder := httptest.NewRecorder()
	router.ServeHTTP(addQuestionRecorder, authorizedRequest(http.MethodPost, "/api/v1/papers/100/sections/1/questions", []byte(`{
		"tenant_id": 10,
		"question_id": 101,
		"score": "4"
	}`), authHeader))
	if addQuestionRecorder.Code != http.StatusOK {
		t.Fatalf("add question status = %d, body = %s", addQuestionRecorder.Code, addQuestionRecorder.Body.String())
	}

	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, authorizedRequest(http.MethodGet, "/api/v1/papers/100/questions?tenant_id=10", nil, authHeader))
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list questions status = %d, body = %s", listRecorder.Code, listRecorder.Body.String())
	}
	listBody := decodeExamAPIResponse[paperSectionQuestionListResponse](t, listRecorder.Body.Bytes())
	if len(listBody.Data.Items) != 1 || listBody.Data.Items[0].QuestionID != 101 || listBody.Data.Items[0].Score != "4" {
		t.Fatalf("unexpected paper questions: %#v", listBody.Data.Items)
	}

	updateRecorder := httptest.NewRecorder()
	router.ServeHTTP(updateRecorder, authorizedRequest(http.MethodPut, "/api/v1/papers/100/sections/1/questions/101", []byte(`{
		"tenant_id": 10,
		"sort_order": 2,
		"score": "7"
	}`), authHeader))
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("update question status = %d, body = %s", updateRecorder.Code, updateRecorder.Body.String())
	}
	updateBody := decodeExamAPIResponse[paperSectionQuestionResponse](t, updateRecorder.Body.Bytes())
	if updateBody.Data.SortOrder != 2 || updateBody.Data.Score != "7" {
		t.Fatalf("unexpected updated question response: %#v", updateBody.Data)
	}

	replaceRecorder := httptest.NewRecorder()
	router.ServeHTTP(replaceRecorder, authorizedRequest(http.MethodPost, "/api/v1/papers/100/sections/1/questions/101/replace", []byte(`{
		"tenant_id": 10,
		"new_question_id": 103,
		"sort_order": 1,
		"score": "6"
	}`), authHeader))
	if replaceRecorder.Code != http.StatusOK {
		t.Fatalf("replace question status = %d, body = %s", replaceRecorder.Code, replaceRecorder.Body.String())
	}
	replaceBody := decodeExamAPIResponse[paperSectionQuestionResponse](t, replaceRecorder.Body.Bytes())
	if replaceBody.Data.QuestionID != 103 || replaceBody.Data.SortOrder != 1 || replaceBody.Data.Score != "6" {
		t.Fatalf("unexpected replaced question response: %#v", replaceBody.Data)
	}

	deleteRecorder := httptest.NewRecorder()
	router.ServeHTTP(deleteRecorder, authorizedRequest(http.MethodDelete, "/api/v1/papers/100/sections/1/questions/103?tenant_id=10", nil, authHeader))
	if deleteRecorder.Code != http.StatusOK {
		t.Fatalf("delete question status = %d, body = %s", deleteRecorder.Code, deleteRecorder.Body.String())
	}

	var remaining int64
	if err := gormDB.Table("paper_section_questions").
		Where("tenant_id = ?", 10).
		Where("paper_id = ?", 100).
		Count(&remaining).Error; err != nil {
		t.Fatalf("read remaining section questions: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("expected section question to be deleted, got remaining=%d", remaining)
	}
}

func TestPaperAssemblyWorkspaceRoutesReorderSectionQuestionsWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedPaperAPITestData(t, gormDB)
	seedPaperAssemblyQuestionWorkspaceAPITestData(t, gormDB)
	seedExamBusinessLoginAPITestData(t, gormDB)
	seedPaperTeacherSpaceAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "teacher.exam", "papermind123")

	for _, questionID := range []int{101, 103} {
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, authorizedRequest(http.MethodPost, "/api/v1/papers/100/sections/1/questions", []byte(fmt.Sprintf(`{
			"tenant_id": 10,
			"question_id": %d,
			"score": "4"
		}`, questionID)), authHeader))
		if recorder.Code != http.StatusOK {
			t.Fatalf("add question %d status = %d, body = %s", questionID, recorder.Code, recorder.Body.String())
		}
	}

	reorderRecorder := httptest.NewRecorder()
	router.ServeHTTP(reorderRecorder, authorizedRequest(http.MethodPut, "/api/v1/papers/100/sections/1/questions/103", []byte(`{
		"tenant_id": 10,
		"sort_order": 1,
		"score": "4"
	}`), authHeader))
	if reorderRecorder.Code != http.StatusOK {
		t.Fatalf("reorder section question status = %d, body = %s", reorderRecorder.Code, reorderRecorder.Body.String())
	}

	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, authorizedRequest(http.MethodGet, "/api/v1/papers/100/questions?tenant_id=10", nil, authHeader))
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list reordered questions status = %d, body = %s", listRecorder.Code, listRecorder.Body.String())
	}
	listBody := decodeExamAPIResponse[paperSectionQuestionListResponse](t, listRecorder.Body.Bytes())
	if len(listBody.Data.Items) != 2 {
		t.Fatalf("unexpected reordered question count: %#v", listBody.Data.Items)
	}
	if listBody.Data.Items[0].QuestionID != 103 || listBody.Data.Items[0].SortOrder != 1 {
		t.Fatalf("expected question 103 ordered first, got %#v", listBody.Data.Items)
	}
	if listBody.Data.Items[1].QuestionID != 101 || listBody.Data.Items[1].SortOrder != 2 {
		t.Fatalf("expected question 101 ordered second, got %#v", listBody.Data.Items)
	}
}

func TestPaperAssemblyWorkspaceRoutesReorderSectionsWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedPaperAPITestData(t, gormDB)
	seedExamBusinessLoginAPITestData(t, gormDB)
	seedPaperTeacherSpaceAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "teacher.exam", "papermind123")

	createSectionRecorder := httptest.NewRecorder()
	router.ServeHTTP(createSectionRecorder, authorizedRequest(http.MethodPost, "/api/v1/papers/100/sections", []byte(fmt.Sprintf(`{
		"tenant_id": 10,
		"name": "二、语言文字运用",
		"question_type": "%s",
		"instructions": "请完成语言文字基础题"
	}`, constant.QuestionTypeSingle)), authHeader))
	if createSectionRecorder.Code != http.StatusOK {
		t.Fatalf("create section status = %d, body = %s", createSectionRecorder.Code, createSectionRecorder.Body.String())
	}
	createSectionBody := decodeExamAPIResponse[paperSectionResponse](t, createSectionRecorder.Body.Bytes())

	reorderRecorder := httptest.NewRecorder()
	router.ServeHTTP(reorderRecorder, authorizedRequest(http.MethodPut, "/api/v1/papers/100/sections/reorder", []byte(fmt.Sprintf(`{
		"tenant_id": 10,
		"orders": [
			{"section_id": %d, "sort_order": 1},
			{"section_id": 1, "sort_order": 2}
		]
	}`, createSectionBody.Data.ID)), authHeader))
	if reorderRecorder.Code != http.StatusOK {
		t.Fatalf("reorder sections status = %d, body = %s", reorderRecorder.Code, reorderRecorder.Body.String())
	}

	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, authorizedRequest(http.MethodGet, "/api/v1/papers/100/sections?tenant_id=10", nil, authHeader))
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list sections status = %d, body = %s", listRecorder.Code, listRecorder.Body.String())
	}
	listBody := decodeExamAPIResponse[paperSectionListResponse](t, listRecorder.Body.Bytes())
	if len(listBody.Data.Items) != 2 {
		t.Fatalf("unexpected section count after reorder: %#v", listBody.Data.Items)
	}
	if listBody.Data.Items[0].ID != createSectionBody.Data.ID || listBody.Data.Items[0].SortOrder != 1 {
		t.Fatalf("expected new section ordered first, got %#v", listBody.Data.Items)
	}
	if listBody.Data.Items[1].ID != 1 || listBody.Data.Items[1].SortOrder != 2 {
		t.Fatalf("expected original section ordered second, got %#v", listBody.Data.Items)
	}

	if err := gormDB.Exec(`
		INSERT INTO paper_section_rules (
			id, tenant_id, section_id, paper_id, sort_order, question_count, score_per_question,
			created_at, created_by_type, updated_at, updated_by_type, ext_json
		) VALUES (9901, 10, ?, 100, 1, 2, '5', ?, 'tenant_user', ?, 'tenant_user', '{}')
	`, createSectionBody.Data.ID, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed section rule: %v", err)
	}
	deleteSectionRecorder := httptest.NewRecorder()
	router.ServeHTTP(deleteSectionRecorder, authorizedRequest(
		http.MethodDelete,
		fmt.Sprintf("/api/v1/papers/100/sections/%d?tenant_id=10", createSectionBody.Data.ID),
		nil,
		authHeader,
	))
	if deleteSectionRecorder.Code != http.StatusOK {
		t.Fatalf("delete section status = %d, body = %s", deleteSectionRecorder.Code, deleteSectionRecorder.Body.String())
	}
	listAfterDeleteRecorder := httptest.NewRecorder()
	router.ServeHTTP(listAfterDeleteRecorder, authorizedRequest(http.MethodGet, "/api/v1/papers/100/sections?tenant_id=10", nil, authHeader))
	if listAfterDeleteRecorder.Code != http.StatusOK {
		t.Fatalf("list sections after delete status = %d, body = %s", listAfterDeleteRecorder.Code, listAfterDeleteRecorder.Body.String())
	}
	listAfterDeleteBody := decodeExamAPIResponse[paperSectionListResponse](t, listAfterDeleteRecorder.Body.Bytes())
	if len(listAfterDeleteBody.Data.Items) != 1 {
		t.Fatalf("unexpected section count after delete: %#v", listAfterDeleteBody.Data.Items)
	}
	if listAfterDeleteBody.Data.Items[0].ID != 1 || listAfterDeleteBody.Data.Items[0].SortOrder != 1 {
		t.Fatalf("expected remaining section compacted to first, got %#v", listAfterDeleteBody.Data.Items)
	}
	var deletedRuleCount int64
	if err := gormDB.Table("paper_section_rules").
		Where("tenant_id = ? AND section_id = ?", 10, createSectionBody.Data.ID).
		Count(&deletedRuleCount).Error; err != nil {
		t.Fatalf("count deleted rules: %v", err)
	}
	if deletedRuleCount != 0 {
		t.Fatalf("expected deleted section rules to be removed, got %d", deletedRuleCount)
	}

	missingRecorder := httptest.NewRecorder()
	router.ServeHTTP(missingRecorder, authorizedRequest(http.MethodPut, "/api/v1/papers/100/sections/reorder", []byte(`{
		"tenant_id": 10,
		"orders": [
			{"section_id": 9999, "sort_order": 1}
		]
	}`), authHeader))
	if missingRecorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected missing section reorder to fail, got status = %d, body = %s", missingRecorder.Code, missingRecorder.Body.String())
	}
}

func TestPaperDeleteSectionCompactsWhenSoftDeletedSectionKeepsSortOrder(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedPaperAPITestData(t, gormDB)
	seedExamBusinessLoginAPITestData(t, gormDB)
	seedPaperTeacherSpaceAPITestData(t, gormDB)
	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "teacher.exam", "papermind123")

	if err := gormDB.Exec(`
		UPDATE paper_sections
		SET sort_order = 2
		WHERE tenant_id = 10 AND paper_id = 100 AND id = 1
	`).Error; err != nil {
		t.Fatalf("move active section out of historical deleted slot: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO paper_sections (
			id, tenant_id, paper_id, sort_order, name, question_type, instructions,
			total_score, question_count, created_at, updated_at, ext_json, deleted_at
		) VALUES (?, 10, 100, 1, '历史删除大题', ?, '', 0, 0, ?, ?, '{}', ?)
	`, 9902, constant.QuestionTypeSingle, fixedAPINow, fixedAPINow, fixedAPINow+1).Error; err != nil {
		t.Fatalf("seed historical deleted section: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO paper_sections (
			id, tenant_id, paper_id, sort_order, name, question_type, instructions,
			total_score, question_count, created_at, updated_at, ext_json
		) VALUES (?, 10, 100, 3, '待删除大题', ?, '', 0, 0, ?, ?, '{}')
	`, 9903, constant.QuestionTypeSingle, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed active section to delete: %v", err)
	}

	deleteRecorder := httptest.NewRecorder()
	router.ServeHTTP(deleteRecorder, authorizedRequest(http.MethodDelete, "/api/v1/papers/100/sections/9903?tenant_id=10", nil, authHeader))
	if deleteRecorder.Code != http.StatusOK {
		t.Fatalf("delete section with historical sort slot status = %d, body = %s", deleteRecorder.Code, deleteRecorder.Body.String())
	}

	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, authorizedRequest(http.MethodGet, "/api/v1/papers/100/sections?tenant_id=10", nil, authHeader))
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list sections after compact status = %d, body = %s", listRecorder.Code, listRecorder.Body.String())
	}
	body := decodeExamAPIResponse[paperSectionListResponse](t, listRecorder.Body.Bytes())
	if len(body.Data.Items) != 1 || body.Data.Items[0].ID != 1 || body.Data.Items[0].SortOrder != 1 {
		t.Fatalf("expected remaining active section compacted despite historical deleted slot, got %#v", body.Data.Items)
	}
}

func TestPaperDeleteSectionAvoidsSoftDeletedSectionAtTemporarySortOrder(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedPaperAPITestData(t, gormDB)
	seedExamBusinessLoginAPITestData(t, gormDB)
	seedPaperTeacherSpaceAPITestData(t, gormDB)
	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "teacher.exam", "papermind123")

	if err := gormDB.Exec(`
		INSERT INTO paper_sections (
			id, tenant_id, paper_id, sort_order, name, question_type, instructions,
			total_score, question_count, created_at, updated_at, ext_json
		) VALUES (?, 10, 100, 2, '待删除大题', ?, '', 0, 0, ?, ?, '{}')
	`, 9904, constant.QuestionTypeSingle, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed active section to delete: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO paper_sections (
			id, tenant_id, paper_id, sort_order, name, question_type, instructions,
			total_score, question_count, created_at, updated_at, ext_json, deleted_at
		) VALUES (?, 10, 100, 3, '历史删除大题', ?, '', 0, 0, ?, ?, '{}', ?)
	`, 9905, constant.QuestionTypeSingle, fixedAPINow, fixedAPINow, fixedAPINow+1).Error; err != nil {
		t.Fatalf("seed historical deleted section at temporary sort order: %v", err)
	}

	deleteRecorder := httptest.NewRecorder()
	router.ServeHTTP(deleteRecorder, authorizedRequest(http.MethodDelete, "/api/v1/papers/100/sections/9904?tenant_id=10", nil, authHeader))
	if deleteRecorder.Code != http.StatusOK {
		t.Fatalf("delete section with occupied temporary sort order status = %d, body = %s", deleteRecorder.Code, deleteRecorder.Body.String())
	}

	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, authorizedRequest(http.MethodGet, "/api/v1/papers/100/sections?tenant_id=10", nil, authHeader))
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list sections after delete status = %d, body = %s", listRecorder.Code, listRecorder.Body.String())
	}
	body := decodeExamAPIResponse[paperSectionListResponse](t, listRecorder.Body.Bytes())
	if len(body.Data.Items) != 1 || body.Data.Items[0].ID != 1 || body.Data.Items[0].SortOrder != 1 {
		t.Fatalf("expected active section compacted after avoiding occupied temporary slot, got %#v", body.Data.Items)
	}
}

func TestPaperAssemblyWorkspaceRoutesManageRulesAndBuildModeWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedPaperAPITestData(t, gormDB)
	seedPaperRuleAPITestData(t, gormDB)
	seedExamBusinessLoginAPITestData(t, gormDB)
	seedPaperTeacherSpaceAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "teacher.exam", "papermind123")

	createRuleRecorder := httptest.NewRecorder()
	router.ServeHTTP(createRuleRecorder, authorizedRequest(http.MethodPost, "/api/v1/papers/100/sections/1/rules", []byte(`{
		"tenant_id": 10,
		"sort_order": 1,
		"difficulty": "easy",
		"tag_ids": [1],
		"question_count": 1,
		"score_per_question": "4"
	}`), authHeader))
	if createRuleRecorder.Code != http.StatusOK {
		t.Fatalf("create rule status = %d, body = %s", createRuleRecorder.Code, createRuleRecorder.Body.String())
	}
	createRuleBody := decodeExamAPIResponse[paperRuleResponse](t, createRuleRecorder.Body.Bytes())

	updateRuleRecorder := httptest.NewRecorder()
	router.ServeHTTP(updateRuleRecorder, authorizedRequest(http.MethodPut, fmt.Sprintf("/api/v1/papers/100/rules/%d", createRuleBody.Data.ID), []byte(`{
		"tenant_id": 10,
		"section_id": 1,
		"sort_order": 2,
		"difficulty": "medium",
		"tag_ids": [1],
		"question_count": 3,
		"score_per_question": "5"
	}`), authHeader))
	if updateRuleRecorder.Code != http.StatusOK {
		t.Fatalf("update rule status = %d, body = %s", updateRuleRecorder.Code, updateRuleRecorder.Body.String())
	}
	updateRuleBody := decodeExamAPIResponse[paperRuleResponse](t, updateRuleRecorder.Body.Bytes())
	if updateRuleBody.Data.SortOrder != 2 || updateRuleBody.Data.QuestionCount != 3 || updateRuleBody.Data.ScorePerQuestion != "5" {
		t.Fatalf("unexpected updated rule response: %#v", updateRuleBody.Data)
	}

	switchModeRecorder := httptest.NewRecorder()
	router.ServeHTTP(switchModeRecorder, authorizedRequest(http.MethodPut, "/api/v1/papers/100/mode", []byte(`{
		"tenant_id": 10,
		"build_mode": "rule_live"
	}`), authHeader))
	if switchModeRecorder.Code != http.StatusOK {
		t.Fatalf("switch build mode status = %d, body = %s", switchModeRecorder.Code, switchModeRecorder.Body.String())
	}
	switchModeBody := decodeExamAPIResponse[paperResponse](t, switchModeRecorder.Body.Bytes())
	if switchModeBody.Data.BuildMode != constant.BuildModeRuleLive || switchModeBody.Data.TotalScore != "15" {
		t.Fatalf("unexpected switched paper response: %#v", switchModeBody.Data)
	}

	deleteRuleRecorder := httptest.NewRecorder()
	router.ServeHTTP(deleteRuleRecorder, authorizedRequest(http.MethodDelete, fmt.Sprintf("/api/v1/papers/100/rules/%d?tenant_id=10", createRuleBody.Data.ID), nil, authHeader))
	if deleteRuleRecorder.Code != http.StatusOK {
		t.Fatalf("delete rule status = %d, body = %s", deleteRuleRecorder.Code, deleteRuleRecorder.Body.String())
	}

	var remaining int64
	if err := gormDB.Table("paper_section_rules").
		Where("tenant_id = ?", 10).
		Where("paper_id = ?", 100).
		Count(&remaining).Error; err != nil {
		t.Fatalf("read remaining rules: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("expected rule to be deleted, got remaining=%d", remaining)
	}
}

func TestPaperAssemblyWorkspaceRoutesRejectRuleChangesForPublishedRuleLiveExam(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedPaperAPITestData(t, gormDB)
	seedPaperRuleAPITestData(t, gormDB)
	seedExamBusinessLoginAPITestData(t, gormDB)
	seedPaperTeacherSpaceAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "teacher.exam", "papermind123")

	if err := gormDB.Exec(`
		INSERT INTO exams (
			id, tenant_id, paper_id, name, start_time, end_time, duration_minutes,
			max_attempts, result_strategy, publish_mode, invite_code, status,
			created_at, updated_at, ext_json
		) VALUES (9900, 10, 100, '已发布实时考试', 0, 0, 0, 1, 'latest', 'manual_publish', 'LIVE9900', 'published', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed published exam: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO exam_live_question_pools (
			id, tenant_id, exam_id, section_id, rule_id, question_id,
			created_at, created_by_type, ext_json
		) VALUES (88001, 10, 9900, 1, 201, 101, ?, 'tenant_user', '{}')
	`, fixedAPINow).Error; err != nil {
		t.Fatalf("seed published live pool: %v", err)
	}

	reorderRecorder := httptest.NewRecorder()
	router.ServeHTTP(reorderRecorder, authorizedRequest(http.MethodPut, "/api/v1/papers/100/sections/reorder", []byte(`{
		"tenant_id": 10,
		"orders": [
			{"section_id": 1, "sort_order": 1}
		]
	}`), authHeader))
	if reorderRecorder.Code != http.StatusBadRequest {
		t.Fatalf("reorder sections during published live exam status = %d, body = %s", reorderRecorder.Code, reorderRecorder.Body.String())
	}

	createSectionRecorder := httptest.NewRecorder()
	router.ServeHTTP(createSectionRecorder, authorizedRequest(http.MethodPost, "/api/v1/papers/100/sections", []byte(`{
		"tenant_id": 10,
		"name": "二、阅读题",
		"question_type": "single",
		"instructions": "每题 5 分"
	}`), authHeader))
	if createSectionRecorder.Code != http.StatusBadRequest {
		t.Fatalf("create section during published live exam status = %d, body = %s", createSectionRecorder.Code, createSectionRecorder.Body.String())
	}

	createRuleRecorder := httptest.NewRecorder()
	router.ServeHTTP(createRuleRecorder, authorizedRequest(http.MethodPost, "/api/v1/papers/100/sections/1/rules", []byte(`{
		"tenant_id": 10,
		"sort_order": 3,
		"difficulty": "easy",
		"tag_ids": [1],
		"question_count": 1,
		"score_per_question": "4"
	}`), authHeader))
	if createRuleRecorder.Code != http.StatusBadRequest {
		t.Fatalf("create rule during published live exam status = %d, body = %s", createRuleRecorder.Code, createRuleRecorder.Body.String())
	}

	updateRuleRecorder := httptest.NewRecorder()
	router.ServeHTTP(updateRuleRecorder, authorizedRequest(http.MethodPut, "/api/v1/papers/100/rules/201", []byte(`{
		"tenant_id": 10,
		"section_id": 1,
		"sort_order": 2,
		"difficulty": "medium",
		"tag_ids": [1],
		"question_count": 3,
		"score_per_question": "5"
	}`), authHeader))
	if updateRuleRecorder.Code != http.StatusBadRequest {
		t.Fatalf("update rule during published live exam status = %d, body = %s", updateRuleRecorder.Code, updateRuleRecorder.Body.String())
	}

	deleteRuleRecorder := httptest.NewRecorder()
	router.ServeHTTP(deleteRuleRecorder, authorizedRequest(http.MethodDelete, "/api/v1/papers/100/rules/201?tenant_id=10", nil, authHeader))
	if deleteRuleRecorder.Code != http.StatusBadRequest {
		t.Fatalf("delete rule during published live exam status = %d, body = %s", deleteRuleRecorder.Code, deleteRuleRecorder.Body.String())
	}

	switchModeRecorder := httptest.NewRecorder()
	router.ServeHTTP(switchModeRecorder, authorizedRequest(http.MethodPut, "/api/v1/papers/100/mode", []byte(`{
		"tenant_id": 10,
		"build_mode": "manual"
	}`), authHeader))
	if switchModeRecorder.Code != http.StatusBadRequest {
		t.Fatalf("switch build mode during published live exam status = %d, body = %s", switchModeRecorder.Code, switchModeRecorder.Body.String())
	}

	generateRecorder := httptest.NewRecorder()
	router.ServeHTTP(generateRecorder, authorizedRequest(http.MethodPost, "/api/v1/papers/100/rule-fixed/generate", []byte(`{
		"tenant_id": 10
	}`), authHeader))
	if generateRecorder.Code != http.StatusBadRequest {
		t.Fatalf("generate rule_fixed during published live exam status = %d, body = %s", generateRecorder.Code, generateRecorder.Body.String())
	}

	var generatedQuestionCount int64
	if err := gormDB.Table("paper_section_questions").
		Where("tenant_id = ? AND paper_id = ?", 10, 100).
		Count(&generatedQuestionCount).Error; err != nil {
		t.Fatalf("count generated fixed questions after rejected generate: %v", err)
	}
	if generatedQuestionCount != 0 {
		t.Fatalf("expected no fixed questions generated after rejection, got %d", generatedQuestionCount)
	}

	var buildMode string
	if err := gormDB.Table("papers").
		Select("build_mode").
		Where("tenant_id = ? AND id = ?", 10, 100).
		Scan(&buildMode).Error; err != nil {
		t.Fatalf("read paper build mode after rejected generate: %v", err)
	}
	if buildMode != constant.BuildModeManual {
		t.Fatalf("expected paper build mode to stay manual after rejected generate, got %q", buildMode)
	}
}

func TestPaperAPIRoutesRejectInvalidScoresWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedPaperAPITestData(t, gormDB)
	seedExamBusinessLoginAPITestData(t, gormDB)
	seedPaperTeacherSpaceAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "teacher.exam", "papermind123")

	for _, tc := range []struct {
		name   string
		target string
		body   []byte
	}{
		{
			name:   "manual question score",
			target: "/api/v1/papers/100/sections/1/questions",
			body: []byte(`{
				"tenant_id": 10,
				"question_id": 101,
				"score": "not-a-score"
			}`),
		},
		{
			name:   "rule score",
			target: "/api/v1/papers/100/sections/1/rules",
			body: []byte(`{
				"tenant_id": 10,
				"sort_order": 1,
				"difficulty": "easy",
				"tag_ids": [1],
				"question_count": 1,
				"score_per_question": "-1"
			}`),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, authorizedRequest(http.MethodPost, tc.target, tc.body, authHeader))
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("expected invalid score to be rejected, got status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestTeacherCannotReadOtherSpacePaperDetailsWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedPaperAPITestData(t, gormDB)
	seedExamBusinessLoginAPITestData(t, gormDB)
	seedPaperTeacherSpaceAPITestData(t, gormDB)
	seedOtherSpacePaperDetailsAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "teacher.exam", "papermind123")

	for _, tc := range []struct {
		name   string
		method string
		target string
	}{
		{name: "sections", method: http.MethodGet, target: "/api/v1/papers/201/sections?tenant_id=10"},
		{name: "rules", method: http.MethodGet, target: "/api/v1/papers/201/rules?tenant_id=10"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, authorizedRequest(tc.method, tc.target, nil, authHeader))
			if recorder.Code != http.StatusForbidden {
				t.Fatalf("expected other-space paper detail read to be forbidden, got status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestTenantAdminCanCreateAndDeletePublicPaperWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedSpaceAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

	createRecorder := httptest.NewRecorder()
	router.ServeHTTP(createRecorder, authorizedRequest(http.MethodPost, "/api/v1/papers", []byte(`{
		"tenant_id": 10,
		"name": "租户公共试卷",
		"description": "租户管理员维护的公共试卷",
		"shuffle_questions": true,
		"show_analysis": true
	}`), authHeader))
	if createRecorder.Code != http.StatusOK {
		t.Fatalf("create public paper status = %d, body = %s", createRecorder.Code, createRecorder.Body.String())
	}
	createBody := decodeExamAPIResponse[paperResponse](t, createRecorder.Body.Bytes())
	if createBody.Data.SpaceID != nil || createBody.Data.BuildMode != constant.BuildModeManual || createBody.Data.Status != constant.PaperStatusDraft {
		t.Fatalf("unexpected created public paper: %#v", createBody.Data)
	}
	if createBody.Data.CreatorName != "tenant.admin" || createBody.Data.CreatedAt != fixedAPINow {
		t.Fatalf("expected created public paper creator metadata, got %#v", createBody.Data)
	}

	deleteRecorder := httptest.NewRecorder()
	router.ServeHTTP(deleteRecorder, authorizedRequest(http.MethodDelete, fmt.Sprintf("/api/v1/papers/%d?tenant_id=10", createBody.Data.ID), nil, authHeader))
	if deleteRecorder.Code != http.StatusOK {
		t.Fatalf("delete public paper status = %d, body = %s", deleteRecorder.Code, deleteRecorder.Body.String())
	}

	var activeCount int64
	if err := gormDB.Table("papers").
		Where("tenant_id = ? AND id = ? AND deleted_at = 0", 10, createBody.Data.ID).
		Count(&activeCount).Error; err != nil {
		t.Fatalf("count active public paper: %v", err)
	}
	if activeCount != 0 {
		t.Fatalf("expected public paper to be soft deleted, got active count %d", activeCount)
	}

	var createdBy uint64
	if err := gormDB.Table("papers").
		Select("created_by").
		Where("tenant_id = ? AND id = ?", 10, createBody.Data.ID).
		Scan(&createdBy).Error; err != nil {
		t.Fatalf("read created public paper creator: %v", err)
	}
	if createdBy != 99 {
		t.Fatalf("expected created public paper created_by=99, got %d", createdBy)
	}
}

func TestTenantAdminCannotCreatePaperForMissingSpaceWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedSpaceAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodPost, "/api/v1/papers", []byte(`{
		"tenant_id": 10,
		"space_id": 404,
		"name": "无效空间试卷"
	}`), authHeader))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected missing space paper create to be rejected, got status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestTeacherCannotWritePublicPaper(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedPaperAPITestData(t, gormDB)
	seedExamBusinessLoginAPITestData(t, gormDB)

	if err := gormDB.Exec(`
		INSERT INTO papers (
			id, tenant_id, space_id, name, description, total_score, build_mode, status,
			created_at, updated_at, ext_json
		) VALUES (?, ?, NULL, ?, '', 0, ?, 'draft', ?, ?, '{}')
	`, 200, 10, "租户公共试卷", constant.BuildModeManual, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed public paper: %v", err)
	}

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "teacher.exam", "papermind123")
	payload := []byte(fmt.Sprintf(`{
		"tenant_id": 10,
		"name": "公共试卷大题",
		"question_type": "%s",
		"instructions": "教师不能维护租户公共试卷"
	}`, constant.QuestionTypeSingle))

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodPost, "/api/v1/papers/200/sections", payload, authHeader))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected teacher public paper write to be forbidden, got status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestTeacherCannotCreateOrDeletePublicPaperWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedPaperAPITestData(t, gormDB)
	seedExamBusinessLoginAPITestData(t, gormDB)

	if err := gormDB.Exec(`
		INSERT INTO papers (
			id, tenant_id, space_id, name, description, total_score, build_mode, status,
			created_at, updated_at, ext_json
		) VALUES (?, ?, NULL, ?, '', 0, ?, 'draft', ?, ?, '{}')
	`, 200, 10, "租户公共试卷", constant.BuildModeManual, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed public paper: %v", err)
	}

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "teacher.exam", "papermind123")

	createRecorder := httptest.NewRecorder()
	router.ServeHTTP(createRecorder, authorizedRequest(http.MethodPost, "/api/v1/papers", []byte(`{
		"tenant_id": 10,
		"name": "教师越权公共试卷"
	}`), authHeader))
	if createRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected teacher public paper create to be forbidden, got status = %d, body = %s", createRecorder.Code, createRecorder.Body.String())
	}

	deleteRecorder := httptest.NewRecorder()
	router.ServeHTTP(deleteRecorder, authorizedRequest(http.MethodDelete, "/api/v1/papers/200?tenant_id=10", nil, authHeader))
	if deleteRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected teacher public paper delete to be forbidden, got status = %d, body = %s", deleteRecorder.Code, deleteRecorder.Body.String())
	}

	var activeCount int64
	if err := gormDB.Table("papers").
		Where("tenant_id = ? AND id = ? AND deleted_at = 0", 10, 200).
		Count(&activeCount).Error; err != nil {
		t.Fatalf("count public paper after forbidden delete: %v", err)
	}
	if activeCount != 1 {
		t.Fatalf("expected forbidden delete to keep public paper active, got %d", activeCount)
	}
}

func TestDisabledTenantAdminCannotReadPublicPaperWithStaleSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedSpaceAPITestData(t, gormDB)

	if err := gormDB.Exec(`
		INSERT INTO papers (
			id, tenant_id, space_id, name, description, total_score, build_mode, status,
			created_at, updated_at, ext_json
		) VALUES (?, ?, NULL, ?, '', 0, ?, 'draft', ?, ?, '{}')
	`, 200, 10, "租户公共试卷", constant.BuildModeManual, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed public paper: %v", err)
	}

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")
	if err := gormDB.Table("tenant_user_memberships").
		Where("tenant_id = ? AND user_id = ?", 10, 99).
		Update("status", "disabled").Error; err != nil {
		t.Fatalf("disable tenant admin after login: %v", err)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodGet, "/api/v1/papers/200/sections?tenant_id=10", nil, authHeader))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected disabled tenant admin stale session to be forbidden, got status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestCannotDeletePaperReferencedByExamWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedSpaceAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

	deleteRecorder := httptest.NewRecorder()
	router.ServeHTTP(deleteRecorder, authorizedRequest(http.MethodDelete, "/api/v1/papers/100?tenant_id=10", nil, authHeader))
	if deleteRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected referenced paper delete to be rejected, got status = %d, body = %s", deleteRecorder.Code, deleteRecorder.Body.String())
	}

	var activeCount int64
	if err := gormDB.Table("papers").
		Where("tenant_id = ? AND id = ? AND deleted_at = 0", 10, 100).
		Count(&activeCount).Error; err != nil {
		t.Fatalf("count referenced paper after rejected delete: %v", err)
	}
	if activeCount != 1 {
		t.Fatalf("expected referenced paper to stay active, got %d", activeCount)
	}
}

func TestDeleteMissingPaperReturnsBusinessErrorWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedSpaceAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodDelete, "/api/v1/papers/404?tenant_id=10", nil, authHeader))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected missing paper delete to return business error, got status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestPaperAPIRoutesRejectOtherSpaceQuestionsWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedPaperAPITestData(t, gormDB)
	seedPaperRuleAPITestData(t, gormDB)
	seedOtherSpaceQuestionAPITestData(t, gormDB)
	seedExamBusinessLoginAPITestData(t, gormDB)
	seedPaperTeacherSpaceAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "teacher.exam", "papermind123")

	addQuestionRecorder := httptest.NewRecorder()
	router.ServeHTTP(addQuestionRecorder, authorizedRequest(http.MethodPost, "/api/v1/papers/100/sections/1/questions", []byte(`{
		"tenant_id": 10,
		"question_id": 102,
		"score": "4"
	}`), authHeader))
	if addQuestionRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected other-space manual question to be forbidden, got status = %d, body = %s", addQuestionRecorder.Code, addQuestionRecorder.Body.String())
	}
	var addedCount int64
	if err := gormDB.Table("paper_section_questions").
		Where("tenant_id = ? AND paper_id = ? AND question_id = ?", 10, 100, 102).
		Count(&addedCount).Error; err != nil {
		t.Fatalf("count other-space section question: %v", err)
	}
	if addedCount != 0 {
		t.Fatalf("expected other-space question not to be added, got %d", addedCount)
	}

	createRulePayload := []byte(`{
		"tenant_id": 10,
		"sort_order": 1,
		"difficulty": "easy",
		"tag_ids": [2],
		"question_count": 1,
		"score_per_question": "4"
	}`)
	createRuleRecorder := httptest.NewRecorder()
	router.ServeHTTP(createRuleRecorder, authorizedRequest(http.MethodPost, "/api/v1/papers/100/sections/1/rules", createRulePayload, authHeader))
	if createRuleRecorder.Code != http.StatusOK {
		t.Fatalf("create rule status = %d, body = %s", createRuleRecorder.Code, createRuleRecorder.Body.String())
	}

	precheckRecorder := httptest.NewRecorder()
	router.ServeHTTP(precheckRecorder, authorizedRequest(http.MethodPost, "/api/v1/papers/100/rule-live/precheck", []byte(`{
		"tenant_id": 10
	}`), authHeader))
	if precheckRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected other-space rule candidates to be rejected, got status = %d, body = %s", precheckRecorder.Code, precheckRecorder.Body.String())
	}
}

func seedPaperAPITestData(t *testing.T, gormDB *gorm.DB) {
	t.Helper()

	if err := gormDB.Exec(`
		INSERT INTO papers (
			id, tenant_id, space_id, name, description, total_score, build_mode, status,
			created_at, created_by, created_by_type, updated_at, updated_by, updated_by_type, ext_json
		) VALUES (?, ?, ?, ?, '', 0, ?, 'draft', ?, ?, 'tenant_user', ?, ?, 'tenant_user', '{}')
	`, 100, 10, 301, "高一语文月考试卷", constant.BuildModeManual, fixedAPINow, 501, fixedAPINow, 501).Error; err != nil {
		t.Fatalf("seed paper: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO paper_sections (
			id, tenant_id, paper_id, sort_order, name, question_type, instructions,
			total_score, question_count, created_at, updated_at, ext_json
		) VALUES (?, ?, ?, 1, ?, ?, '', 0, 0, ?, ?, '{}')
	`, 1, 10, 100, "一、现代文阅读", constant.QuestionTypeSingle, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed section: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO questions (
			id, tenant_id, type, difficulty, title, analysis, score_default, status,
			created_at, updated_at, ext_json
		) VALUES
			(101, 10, ?, 'easy', '病句辨析题', '识别语序不当。', 4, 'enabled', ?, ?, '{}')
	`, constant.QuestionTypeSingle, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed question: %v", err)
	}
}

func seedPaperTeacherSpaceAPITestData(t *testing.T, gormDB *gorm.DB) {
	t.Helper()
	if err := gormDB.Exec(`
		INSERT INTO spaces (
			id, tenant_id, name, type, status, created_at, updated_at, ext_json
		) VALUES (301, 10, '高一 1 班', 'class', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed paper space: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO space_members (
			id, tenant_id, space_id, user_id, role_in_space, status, created_at, updated_at, ext_json
		) VALUES (301, 10, 301, 501, 'teacher', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed paper teacher member: %v", err)
	}
}

func seedPaperRuleAPITestData(t *testing.T, gormDB *gorm.DB) {
	t.Helper()

	if err := gormDB.Exec(`
		INSERT INTO tags (
			id, tenant_id, name, created_at, updated_at, ext_json
		) VALUES
			(1, 10, '阅读理解', ?, ?, '{}'),
			(2, 10, '语言文字', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed tags: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO question_tags (
			tenant_id, question_id, tag_id, created_at, ext_json
		) VALUES
			(10, 101, 1, ?, '{}')
	`, fixedAPINow).Error; err != nil {
		t.Fatalf("seed question tags: %v", err)
	}
}

func seedPaperAssemblyQuestionWorkspaceAPITestData(t *testing.T, gormDB *gorm.DB) {
	t.Helper()

	if err := gormDB.Exec(`
		INSERT INTO questions (
			id, tenant_id, type, difficulty, title, analysis, score_default, status,
			created_at, updated_at, ext_json
		) VALUES
			(103, 10, ?, 'medium', '现代文阅读替换题', '用于 rule_fixed 替题。', 6, 'enabled', ?, ?, '{}')
	`, constant.QuestionTypeSingle, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed replacement question: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO question_tags (
			tenant_id, question_id, tag_id, created_at, ext_json
		) VALUES
			(10, 103, 1, ?, '{}')
	`, fixedAPINow).Error; err != nil {
		t.Fatalf("seed replacement question tag: %v", err)
	}
}

func seedOtherSpaceQuestionAPITestData(t *testing.T, gormDB *gorm.DB) {
	t.Helper()

	if err := gormDB.Exec(`
		INSERT INTO spaces (
			id, tenant_id, name, type, status, created_at, updated_at, ext_json
		) VALUES (302, 10, '高一 2 班', 'class', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed other question space: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO questions (
			id, tenant_id, space_id, type, difficulty, title, analysis, score_default, status,
			created_at, updated_at, ext_json
		) VALUES (102, 10, 302, ?, 'easy', '其他空间题', '不应被当前空间试卷引用。', 4, 'enabled', ?, ?, '{}')
	`, constant.QuestionTypeSingle, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed other-space question: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO question_tags (
			tenant_id, question_id, tag_id, created_at, ext_json
		) VALUES (10, 102, 2, ?, '{}')
	`, fixedAPINow).Error; err != nil {
		t.Fatalf("seed other-space question tag: %v", err)
	}
}

func seedOtherSpacePaperDetailsAPITestData(t *testing.T, gormDB *gorm.DB) {
	t.Helper()

	if err := gormDB.Exec(`
		INSERT INTO spaces (
			id, tenant_id, name, type, status, created_at, updated_at, ext_json
		) VALUES (302, 10, '高一 2 班', 'class', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed other paper space: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO papers (
			id, tenant_id, space_id, name, description, total_score, build_mode, status,
			created_at, updated_at, ext_json
		) VALUES (?, ?, ?, ?, '', 0, ?, 'draft', ?, ?, '{}')
	`, 201, 10, 302, "其他空间试卷", constant.BuildModeManual, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed other-space paper: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO paper_sections (
			id, tenant_id, paper_id, sort_order, name, question_type, instructions,
			total_score, question_count, created_at, updated_at, ext_json
		) VALUES (?, ?, ?, 1, ?, ?, '', 0, 0, ?, ?, '{}')
	`, 201, 10, 201, "其他空间大题", constant.QuestionTypeSingle, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed other-space section: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO paper_section_rules (
			id, tenant_id, paper_id, section_id, sort_order, difficulty, tag_filter,
			question_count, score_per_question, created_at, updated_at, ext_json
		) VALUES (?, ?, ?, ?, 1, 'easy', '[]', 1, 4, ?, ?, '{}')
	`, 201, 10, 201, 201, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed other-space paper rule: %v", err)
	}
}

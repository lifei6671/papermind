package v1

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TestPaperAPIRoutesListSectionsAndAddManualQuestionWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedPaperAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})

	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, httptest.NewRequest(http.MethodGet, "/api/v1/papers?tenant_id=10", nil))
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listRecorder.Code, listRecorder.Body.String())
	}
	listBody := decodeExamAPIResponse[paperListResponse](t, listRecorder.Body.Bytes())
	if len(listBody.Data.Items) != 1 || listBody.Data.Items[0].Name != "高一语文月考试卷" {
		t.Fatalf("unexpected paper list: %#v", listBody.Data.Items)
	}

	sectionsRecorder := httptest.NewRecorder()
	router.ServeHTTP(sectionsRecorder, httptest.NewRequest(http.MethodGet, "/api/v1/papers/100/sections?tenant_id=10", nil))
	if sectionsRecorder.Code != http.StatusOK {
		t.Fatalf("sections status = %d, body = %s", sectionsRecorder.Code, sectionsRecorder.Body.String())
	}
	sectionsBody := decodeExamAPIResponse[paperSectionListResponse](t, sectionsRecorder.Body.Bytes())
	if len(sectionsBody.Data.Items) != 1 || sectionsBody.Data.Items[0].Name != "一、现代文阅读" {
		t.Fatalf("unexpected sections: %#v", sectionsBody.Data.Items)
	}

	createSectionPayload := []byte(`{
		"tenant_id": 10,
		"name": "二、语言文字运用",
		"question_type": "single",
		"instructions": "请完成语言文字基础题"
	}`)
	createSectionRecorder := httptest.NewRecorder()
	router.ServeHTTP(createSectionRecorder, httptest.NewRequest(http.MethodPost, "/api/v1/papers/100/sections", bytes.NewReader(createSectionPayload)))
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
	router.ServeHTTP(addQuestionRecorder, httptest.NewRequest(http.MethodPost, "/api/v1/papers/100/sections/1/questions", bytes.NewReader(addQuestionPayload)))
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

func TestPaperRuleAPIRoutesConfigureGenerateAndPrecheckWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedPaperAPITestData(t, gormDB)
	seedPaperRuleAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})

	createRulePayload := []byte(`{
		"tenant_id": 10,
		"sort_order": 1,
		"difficulty": "easy",
		"tag_ids": [1],
		"question_count": 1,
		"score_per_question": "4"
	}`)
	createRuleRecorder := httptest.NewRecorder()
	router.ServeHTTP(createRuleRecorder, httptest.NewRequest(http.MethodPost, "/api/v1/papers/100/sections/1/rules", bytes.NewReader(createRulePayload)))
	if createRuleRecorder.Code != http.StatusOK {
		t.Fatalf("create rule status = %d, body = %s", createRuleRecorder.Code, createRuleRecorder.Body.String())
	}
	createRuleBody := decodeExamAPIResponse[paperRuleResponse](t, createRuleRecorder.Body.Bytes())
	if createRuleBody.Data.SectionID != 1 || createRuleBody.Data.QuestionCount != 1 || len(createRuleBody.Data.TagIDs) != 1 || createRuleBody.Data.TagIDs[0] != 1 {
		t.Fatalf("unexpected created rule: %#v", createRuleBody.Data)
	}

	listRulesRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRulesRecorder, httptest.NewRequest(http.MethodGet, "/api/v1/papers/100/rules?tenant_id=10", nil))
	if listRulesRecorder.Code != http.StatusOK {
		t.Fatalf("list rules status = %d, body = %s", listRulesRecorder.Code, listRulesRecorder.Body.String())
	}
	listRulesBody := decodeExamAPIResponse[paperRuleListResponse](t, listRulesRecorder.Body.Bytes())
	if len(listRulesBody.Data.Items) != 1 || listRulesBody.Data.Items[0].ID != createRuleBody.Data.ID {
		t.Fatalf("unexpected rule list: %#v", listRulesBody.Data.Items)
	}

	generatePayload := []byte(`{"tenant_id": 10}`)
	generateRecorder := httptest.NewRecorder()
	router.ServeHTTP(generateRecorder, httptest.NewRequest(http.MethodPost, "/api/v1/papers/100/rule-fixed/generate", bytes.NewReader(generatePayload)))
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
	router.ServeHTTP(precheckRecorder, httptest.NewRequest(http.MethodPost, "/api/v1/papers/100/rule-live/precheck", bytes.NewReader(generatePayload)))
	if precheckRecorder.Code != http.StatusOK {
		t.Fatalf("precheck rule_live status = %d, body = %s", precheckRecorder.Code, precheckRecorder.Body.String())
	}
	precheckBody := decodeExamAPIResponse[paperRuleLivePrecheckResponse](t, precheckRecorder.Body.Bytes())
	if precheckBody.Data.CandidateCount != 1 || len(precheckBody.Data.CandidateQuestionIDs) != 1 || precheckBody.Data.CandidateQuestionIDs[0] != 101 {
		t.Fatalf("unexpected precheck response: %#v", precheckBody.Data)
	}
}

func seedPaperAPITestData(t *testing.T, gormDB *gorm.DB) {
	t.Helper()

	if err := gormDB.Exec(`
		INSERT INTO papers (
			id, tenant_id, name, description, total_score, build_mode, status,
			created_at, updated_at, ext_json
		) VALUES (?, ?, ?, '', 0, ?, 'draft', ?, ?, '{}')
	`, 100, 10, "高一语文月考试卷", "manual", fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed paper: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO paper_sections (
			id, tenant_id, paper_id, sort_order, name, question_type, instructions,
			total_score, question_count, created_at, updated_at, ext_json
		) VALUES (?, ?, ?, 1, ?, 'single', '', 0, 0, ?, ?, '{}')
	`, 1, 10, 100, "一、现代文阅读", fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed section: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO questions (
			id, tenant_id, type, difficulty, title, analysis, score_default, status,
			created_at, updated_at, ext_json
		) VALUES
			(101, 10, 'single', 'easy', '病句辨析题', '识别语序不当。', 4, 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed question: %v", err)
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

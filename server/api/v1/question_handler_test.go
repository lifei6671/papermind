package v1

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/papermind/server/library/constant"
	"gorm.io/gorm"
)

func TestQuestionAPIRoutesListAndCreateWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedQuestionAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})

	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, httptest.NewRequest(http.MethodGet, "/api/v1/questions?tenant_id=10", nil))
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listRecorder.Code, listRecorder.Body.String())
	}
	listBody := decodeExamAPIResponse[questionListResponse](t, listRecorder.Body.Bytes())
	if len(listBody.Data.Items) != 1 {
		t.Fatalf("expected one seeded question, got %#v", listBody.Data.Items)
	}
	seeded := listBody.Data.Items[0]
	if seeded.Title != "现代文阅读主旨题" || seeded.Tag != "阅读理解" {
		t.Fatalf("unexpected seeded question: %#v", seeded)
	}
	if len(seeded.Options) != 2 || seeded.Options[0].Content != "把握中心句" {
		t.Fatalf("expected seeded options in response, got %#v", seeded.Options)
	}

	payload := []byte(fmt.Sprintf(`{
		"tenant_id": 10,
		"type": "%s",
		"difficulty": "medium",
		"title": "函数单调性判断",
		"analysis": "一次函数斜率为正时单调递增。",
		"score_default": "2",
		"tags": ["函数"],
		"options": [
			{"option_key": "A", "content": "y = x", "is_correct": true},
			{"option_key": "B", "content": "y = -x", "is_distractor": true}
		]
	}`, constant.QuestionTypeSingle))
	createRecorder := httptest.NewRecorder()
	router.ServeHTTP(createRecorder, httptest.NewRequest(http.MethodPost, "/api/v1/questions", bytes.NewReader(payload)))
	if createRecorder.Code != http.StatusOK {
		t.Fatalf("create status = %d, body = %s", createRecorder.Code, createRecorder.Body.String())
	}
	createBody := decodeExamAPIResponse[questionResponse](t, createRecorder.Body.Bytes())
	if createBody.Data.Title != "函数单调性判断" || createBody.Data.Tag != "函数" {
		t.Fatalf("unexpected created question response: %#v", createBody.Data)
	}
	if len(createBody.Data.Options) != 2 || createBody.Data.Options[0].Content != "y = x" {
		t.Fatalf("expected created options in response, got %#v", createBody.Data.Options)
	}
}

func TestQuestionImportAPIRouteParsesCSVAndReturnsRowErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})

	requestBody, contentType := buildQuestionImportMultipart(t, fmt.Sprintf(`type,title,options,correct_answer,analysis,difficulty,tags
%s,函数单调性判断,A.y = x|B.y = -x,A,一次函数斜率为正时单调递增。,medium,函数
%s,无正确选项,A.正确|B.错误,,缺少正确答案。,medium,基础
`, constant.QuestionTypeSingle, constant.QuestionTypeSingle))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/questions/import", requestBody)
	request.Header.Set("Content-Type", contentType)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("import status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	body := decodeExamAPIResponse[importQuestionsResponse](t, recorder.Body.Bytes())
	if body.Data.SuccessCount != 1 {
		t.Fatalf("expected one imported row, got %#v", body.Data)
	}
	if len(body.Data.Errors) != 1 || body.Data.Errors[0].RowNumber != 3 {
		t.Fatalf("expected row 3 error, got %#v", body.Data.Errors)
	}

	var importedCount int64
	if err := gormDB.Table("questions").
		Where("tenant_id = ?", 10).
		Where("title = ?", "函数单调性判断").
		Count(&importedCount).Error; err != nil {
		t.Fatalf("count imported questions: %v", err)
	}
	if importedCount != 1 {
		t.Fatalf("expected imported question persisted, got %d", importedCount)
	}
}

func seedQuestionAPITestData(t *testing.T, gormDB *gorm.DB) {
	t.Helper()

	if err := gormDB.Exec(`
		INSERT INTO questions (
			id, tenant_id, type, difficulty, title, analysis, score_default, status,
			created_at, updated_at, ext_json
		) VALUES
			(100, 10, ?, 'easy', '现代文阅读主旨题', '定位中心句并排除以偏概全选项。', 2, 'enabled', ?, ?, '{}')
	`, constant.QuestionTypeSingle, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed question: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO question_options (
			id, tenant_id, question_id, option_key, sort_order, content, is_correct, is_distractor,
			created_at, updated_at, ext_json
		) VALUES
			(1, 10, 100, 'A', 1, '把握中心句', 1, 0, ?, ?, '{}'),
			(2, 10, 100, 'B', 2, '复述细节', 0, 1, ?, ?, '{}')
	`, fixedAPINow, fixedAPINow, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed options: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO tags (
			id, tenant_id, name, created_at, updated_at, ext_json
		) VALUES
			(1, 10, '阅读理解', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed tag: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO question_tags (
			id, tenant_id, question_id, tag_id, created_at, ext_json
		) VALUES
			(1, 10, 100, 1, ?, '{}')
	`, fixedAPINow).Error; err != nil {
		t.Fatalf("seed question tag: %v", err)
	}
}

func buildQuestionImportMultipart(t *testing.T, csvContent string) (*bytes.Buffer, string) {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("tenant_id", "10"); err != nil {
		t.Fatalf("write tenant field: %v", err)
	}
	part, err := writer.CreateFormFile("file", "questions.csv")
	if err != nil {
		t.Fatalf("create import file field: %v", err)
	}
	if _, err := part.Write([]byte(csvContent)); err != nil {
		t.Fatalf("write import file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	return body, writer.FormDataContentType()
}

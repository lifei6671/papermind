package v1

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
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

	payload := []byte(`{
		"tenant_id": 10,
		"type": "single",
		"difficulty": "medium",
		"title": "函数单调性判断",
		"analysis": "一次函数斜率为正时单调递增。",
		"score_default": "2",
		"tags": ["函数"],
		"options": [
			{"option_key": "A", "content": "y = x", "is_correct": true},
			{"option_key": "B", "content": "y = -x", "is_distractor": true}
		]
	}`)
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

func seedQuestionAPITestData(t *testing.T, gormDB *gorm.DB) {
	t.Helper()

	if err := gormDB.Exec(`
		INSERT INTO questions (
			id, tenant_id, type, difficulty, title, analysis, score_default, status,
			created_at, updated_at, ext_json
		) VALUES
			(100, 10, 'single', 'easy', '现代文阅读主旨题', '定位中心句并排除以偏概全选项。', 2, 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
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

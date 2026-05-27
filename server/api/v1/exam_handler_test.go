package v1

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const fixedAPINow int64 = 1_779_792_000_000

func TestExamAPIRoutesListAndPublishWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:            gormDB,
		Now:           func() int64 { return fixedAPINow },
		CodeGenerator: fixedCodeGenerator{code: "PM2027"},
	})

	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, httptest.NewRequest(http.MethodGet, "/api/v1/exams?tenant_id=10", nil))
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listRecorder.Code, listRecorder.Body.String())
	}
	listBody := decodeExamAPIResponse[examListResponse](t, listRecorder.Body.Bytes())
	if len(listBody.Data.Items) != 1 {
		t.Fatalf("expected one seeded exam, got %#v", listBody.Data.Items)
	}
	if listBody.Data.Items[0].Name != "高一语文期中考试" || listBody.Data.Items[0].InviteCode != "PM2026" {
		t.Fatalf("unexpected seeded exam response: %#v", listBody.Data.Items[0])
	}

	payload := []byte(`{
		"tenant_id": 10,
		"paper_id": 100,
		"name": "高一语文月考试卷",
		"target_type": "space",
		"target_id": 200,
		"start_time": 1772269200000,
		"end_time": 1772276400000,
		"duration_minutes": 120,
		"max_attempts": 1,
		"result_strategy": "latest",
		"publish_mode": "manual_publish"
	}`)
	publishRecorder := httptest.NewRecorder()
	router.ServeHTTP(publishRecorder, httptest.NewRequest(http.MethodPost, "/api/v1/exams", bytes.NewReader(payload)))
	if publishRecorder.Code != http.StatusOK {
		t.Fatalf("publish status = %d, body = %s", publishRecorder.Code, publishRecorder.Body.String())
	}
	publishBody := decodeExamAPIResponse[examResponse](t, publishRecorder.Body.Bytes())
	if publishBody.Data.Status != "published" || publishBody.Data.InviteCode != "PM2027" {
		t.Fatalf("unexpected published exam response: %#v", publishBody.Data)
	}
	if publishBody.Data.TargetType != "space" || publishBody.Data.TargetID != 200 {
		t.Fatalf("expected target in response, got %#v", publishBody.Data)
	}

	var targetCount int64
	if err := gormDB.Table("exam_targets").
		Where("tenant_id = ?", 10).
		Where("exam_id = ?", publishBody.Data.ID).
		Count(&targetCount).Error; err != nil {
		t.Fatalf("count targets: %v", err)
	}
	if targetCount != 1 {
		t.Fatalf("expected one target row, got %d", targetCount)
	}
}

func TestExamEntryResolveInviteWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})

	payload := []byte(`{
		"invite_code": "PM2026",
		"user_id": 20
	}`)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/exams/invite/resolve", bytes.NewReader(payload)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("resolve invite status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	body := decodeExamAPIResponse[examInviteResponse](t, recorder.Body.Bytes())
	if body.Data.ID != 1 || body.Data.InviteCode != "PM2026" || body.Data.Name != "高一语文期中考试" {
		t.Fatalf("unexpected invite response: %#v", body.Data)
	}

	anonymousRecorder := httptest.NewRecorder()
	router.ServeHTTP(anonymousRecorder, httptest.NewRequest(http.MethodPost, "/api/v1/exams/invite/resolve", bytes.NewReader([]byte(`{
		"invite_code": "PM2026",
		"user_id": 0
	}`))))
	if anonymousRecorder.Code != http.StatusBadRequest {
		t.Fatalf("anonymous resolve status = %d, body = %s", anonymousRecorder.Code, anonymousRecorder.Body.String())
	}
}

func openExamAPITestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	gormDB, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared&_foreign_keys=on"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	sqlPath := filepath.Join("..", "..", "data", "migrations", "sqlite", "001_tenant_space.sql")
	sqlBytes, err := os.ReadFile(sqlPath)
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	if err := gormDB.Exec(string(sqlBytes)).Error; err != nil {
		t.Fatalf("apply migration: %v", err)
	}
	return gormDB
}

func seedExamAPITestData(t *testing.T, gormDB *gorm.DB) {
	t.Helper()

	if err := gormDB.Exec(`
		INSERT INTO papers (
			id, tenant_id, name, description, total_score, build_mode, status,
			created_at, updated_at, ext_json
		) VALUES (?, ?, ?, '', 100, ?, 'enabled', ?, ?, '{}')
	`, 100, 10, "高一语文月考试卷", "manual", fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed paper: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO exams (
			id, tenant_id, paper_id, name, start_time, end_time, duration_minutes,
			max_attempts, result_strategy, publish_mode, invite_code, status,
			created_at, updated_at, ext_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, 1, 'latest', 'manual_publish', ?, 'published', ?, ?, '{}')
	`, 1, 10, 100, "高一语文期中考试", fixedAPINow, fixedAPINow+7_200_000, 120, "PM2026", fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed exam: %v", err)
	}
}

type apiBodyForTest[T any] struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    T      `json:"data"`
}

func decodeExamAPIResponse[T any](t *testing.T, data []byte) apiBodyForTest[T] {
	t.Helper()

	var body apiBodyForTest[T]
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("decode response %s: %v", string(data), err)
	}
	if body.Code != 0 {
		t.Fatalf("response code = %d, message = %s", body.Code, body.Message)
	}
	return body
}

type fixedCodeGenerator struct {
	code string
}

func (g fixedCodeGenerator) NextCode() (string, error) {
	return g.code, nil
}

package v1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/papermind/server/library/constant"
	"github.com/lifei6671/papermind/server/library/crypto"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const fixedAPINow int64 = 1_779_792_000_000

func TestExamAPIRoutesListAndPublishWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedExamBusinessLoginAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:            gormDB,
		Now:           func() int64 { return fixedAPINow },
		CodeGenerator: fixedCodeGenerator{code: "PM2027"},
	})
	authHeader := tenantAuthHeader(t, router, 10, "teacher.exam", "papermind123")

	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, authorizedRequest(http.MethodGet, "/api/v1/exams?tenant_id=10", nil, authHeader))
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
	router.ServeHTTP(publishRecorder, authorizedRequest(http.MethodPost, "/api/v1/exams", payload, authHeader))
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

func TestPlatformAdminCannotAccessExamBusinessAPIs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedPlatformLoginAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := platformAuthHeader(t, router)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodGet, "/api/v1/exams?tenant_id=10", nil, authHeader))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("platform exam list status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestExamEntryResolveInviteWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedTakingAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})

	anonymousRecorder := httptest.NewRecorder()
	router.ServeHTTP(anonymousRecorder, httptest.NewRequest(http.MethodPost, "/api/v1/exams/invite/resolve", bytes.NewReader([]byte(`{
		"invite_code": "PM2026"
	}`))))
	if anonymousRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous resolve status = %d, body = %s", anonymousRecorder.Code, anonymousRecorder.Body.String())
	}

	authHeader := tenantAuthHeader(t, router, 10, "student20", "papermind123")
	payload := []byte(`{"invite_code": "PM2026", "user_id": 21}`)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodPost, "/api/v1/exams/invite/resolve", payload, authHeader))
	if recorder.Code != http.StatusOK {
		t.Fatalf("resolve invite status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	body := decodeExamAPIResponse[examInviteResponse](t, recorder.Body.Bytes())
	if body.Data.ID != 1 || body.Data.InviteCode != "PM2026" || body.Data.Name != "高一语文期中考试" {
		t.Fatalf("unexpected invite response: %#v", body.Data)
	}
}

func TestStudentTakingAPIRoutesStartSaveAndSubmitWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedTakingAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "student20", "papermind123")

	anonymousStartRecorder := httptest.NewRecorder()
	router.ServeHTTP(anonymousStartRecorder, httptest.NewRequest(http.MethodPost, "/api/v1/exams/1/attempts/start", bytes.NewReader([]byte(`{
		"tenant_id": 10,
		"user_id": 20
	}`))))
	if anonymousStartRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous start status = %d, body = %s", anonymousStartRecorder.Code, anonymousStartRecorder.Body.String())
	}

	resolveRecorder := httptest.NewRecorder()
	router.ServeHTTP(resolveRecorder, authorizedRequest(http.MethodPost, "/api/v1/exams/invite/resolve", []byte(`{
		"invite_code": "PM2026"
	}`), authHeader))
	if resolveRecorder.Code != http.StatusOK {
		t.Fatalf("resolve invite before start status = %d, body = %s", resolveRecorder.Code, resolveRecorder.Body.String())
	}

	startRecorder := httptest.NewRecorder()
	startRequest := httptest.NewRequest(http.MethodPost, "/api/v1/exams/1/attempts/start", bytes.NewReader([]byte(`{
		"tenant_id": 10
	}`)))
	for _, cookie := range resolveRecorder.Result().Cookies() {
		startRequest.AddCookie(cookie)
	}
	router.ServeHTTP(startRecorder, startRequest)
	if startRecorder.Code != http.StatusOK {
		t.Fatalf("start status = %d, body = %s", startRecorder.Code, startRecorder.Body.String())
	}
	startBody := decodeExamAPIResponse[startAttemptResponse](t, startRecorder.Body.Bytes())
	if startBody.Data.Attempt.ID == 0 || startBody.Data.ExamToken == "" || len(startBody.Data.Questions) != 1 {
		t.Fatalf("unexpected start response: %#v", startBody.Data)
	}
	question := startBody.Data.Questions[0]
	if question.Question.Title != "服务端题干" || len(question.Options) != 2 || question.Options[0].ID != 101 {
		t.Fatalf("unexpected question snapshot: %#v", question)
	}

	saveRecorder := httptest.NewRecorder()
	savePayload := fmt.Sprintf(`{
		"tenant_id": 10,
		"exam_token": "%s",
		"question_type": "%s",
		"option_ids": [101]
	}`, startBody.Data.ExamToken, constant.QuestionTypeSingle)
	router.ServeHTTP(saveRecorder, httptest.NewRequest(http.MethodPost, "/api/v1/exam-attempts/"+
		strconv.FormatUint(startBody.Data.Attempt.ID, 10)+"/answers/"+strconv.FormatUint(question.ID, 10), bytes.NewReader([]byte(savePayload))))
	if saveRecorder.Code != http.StatusOK {
		t.Fatalf("save status = %d, body = %s", saveRecorder.Code, saveRecorder.Body.String())
	}
	var answerRow struct {
		AnswerContent string
	}
	if err := gormDB.Table("exam_answers").
		Select("answer_content").
		Where("tenant_id = ? AND attempt_id = ? AND attempt_question_id = ?", 10, startBody.Data.Attempt.ID, question.ID).
		Scan(&answerRow).Error; err != nil {
		t.Fatalf("query answer: %v", err)
	}
	if answerRow.AnswerContent != "101" {
		t.Fatalf("expected normalized answer 101, got %q", answerRow.AnswerContent)
	}

	submitRecorder := httptest.NewRecorder()
	router.ServeHTTP(submitRecorder, httptest.NewRequest(http.MethodPost, "/api/v1/exam-attempts/"+
		strconv.FormatUint(startBody.Data.Attempt.ID, 10)+"/submit", bytes.NewReader([]byte(`{
		"tenant_id": 10,
		"exam_token": "`+startBody.Data.ExamToken+`"
	}`))))
	if submitRecorder.Code != http.StatusOK {
		t.Fatalf("submit status = %d, body = %s", submitRecorder.Code, submitRecorder.Body.String())
	}
	var attemptRow struct {
		Status         string
		ObjectiveScore string
	}
	if err := gormDB.Table("exam_attempts").
		Select("status, objective_score").
		Where("id = ?", startBody.Data.Attempt.ID).
		Scan(&attemptRow).Error; err != nil {
		t.Fatalf("query attempt: %v", err)
	}
	if attemptRow.Status != constant.AttemptStatusSubmitted || attemptRow.ObjectiveScore != "2" {
		t.Fatalf("unexpected submitted attempt: %#v", attemptRow)
	}
}

func TestStudentTakingAPIRoutesPersistNonCriticalEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedTakingAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "student20", "papermind123")
	resolveRecorder := httptest.NewRecorder()
	router.ServeHTTP(resolveRecorder, authorizedRequest(http.MethodPost, "/api/v1/exams/invite/resolve", []byte(`{
		"invite_code": "PM2026"
	}`), authHeader))
	if resolveRecorder.Code != http.StatusOK {
		t.Fatalf("resolve invite status = %d, body = %s", resolveRecorder.Code, resolveRecorder.Body.String())
	}
	startRecorder := httptest.NewRecorder()
	startRequest := httptest.NewRequest(http.MethodPost, "/api/v1/exams/1/attempts/start", bytes.NewReader([]byte(`{"tenant_id":10}`)))
	for _, cookie := range resolveRecorder.Result().Cookies() {
		startRequest.AddCookie(cookie)
	}
	router.ServeHTTP(startRecorder, startRequest)
	if startRecorder.Code != http.StatusOK {
		t.Fatalf("start status = %d, body = %s", startRecorder.Code, startRecorder.Body.String())
	}
	startBody := decodeExamAPIResponse[startAttemptResponse](t, startRecorder.Body.Bytes())

	eventRecorder := httptest.NewRecorder()
	router.ServeHTTP(eventRecorder, httptest.NewRequest(http.MethodPost, "/api/v1/exam-attempts/"+
		strconv.FormatUint(startBody.Data.Attempt.ID, 10)+"/events", bytes.NewReader([]byte(`{
		"tenant_id": 10,
		"exam_token": "`+startBody.Data.ExamToken+`",
		"event_type": "blur",
		"payload": "{\"reason\":\"switch_tab\"}"
	}`))))
	if eventRecorder.Code != http.StatusOK {
		t.Fatalf("event status = %d, body = %s", eventRecorder.Code, eventRecorder.Body.String())
	}
	var eventCount int64
	for attempt := 0; attempt < 20; attempt++ {
		if err := gormDB.Table("exam_events").
			Where("tenant_id = ? AND attempt_id = ? AND event_type = ?", 10, startBody.Data.Attempt.ID, constant.ExamEventTypeBlur).
			Count(&eventCount).Error; err != nil {
			t.Fatalf("count event: %v", err)
		}
		if eventCount == 1 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if eventCount != 1 {
		t.Fatalf("expected non-critical event to be persisted, got %d", eventCount)
	}
}

func TestStudentTakingAPIRoutesRejectsUserOutsideExamTargets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedTakingAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "student21", "papermind123")

	resolveRecorder := httptest.NewRecorder()
	router.ServeHTTP(resolveRecorder, authorizedRequest(http.MethodPost, "/api/v1/exams/invite/resolve", []byte(`{
		"invite_code": "PM2026"
	}`), authHeader))
	if resolveRecorder.Code != http.StatusOK {
		t.Fatalf("resolve invite status = %d, body = %s", resolveRecorder.Code, resolveRecorder.Body.String())
	}

	startRecorder := httptest.NewRecorder()
	startRequest := httptest.NewRequest(http.MethodPost, "/api/v1/exams/1/attempts/start", bytes.NewReader([]byte(`{
		"tenant_id": 10
	}`)))
	for _, cookie := range resolveRecorder.Result().Cookies() {
		startRequest.AddCookie(cookie)
	}
	router.ServeHTTP(startRecorder, startRequest)
	if startRecorder.Code != http.StatusBadRequest {
		t.Fatalf("outside target start status = %d, body = %s", startRecorder.Code, startRecorder.Body.String())
	}
}

func TestReviewAndResultAPIRoutesWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedReviewResultAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:        gormDB,
		Now:       func() int64 { return fixedAPINow },
		ExportDir: t.TempDir(),
	})
	authHeader := tenantAuthHeader(t, router, 10, "teacher.li", "papermind123")

	listReviewRecorder := httptest.NewRecorder()
	router.ServeHTTP(listReviewRecorder, authorizedRequest(http.MethodGet, "/api/v1/grading/pending?tenant_id=10&exam_id=1&space_id=301", nil, authHeader))
	if listReviewRecorder.Code != http.StatusOK {
		t.Fatalf("list review status = %d, body = %s", listReviewRecorder.Code, listReviewRecorder.Body.String())
	}
	reviewBody := decodeExamAPIResponse[pendingReviewListResponse](t, listReviewRecorder.Body.Bytes())
	if len(reviewBody.Data.Items) != 1 {
		t.Fatalf("expected one pending review row, got %#v", reviewBody.Data.Items)
	}
	pending := reviewBody.Data.Items[0]
	if pending.StudentName != "张三" || pending.QuestionTitle != "岳阳楼记思想内涵" || pending.AnswerContent == "" || pending.AnswerVersion != 7 {
		t.Fatalf("unexpected pending review response: %#v", pending)
	}

	gradeRecorder := httptest.NewRecorder()
	router.ServeHTTP(gradeRecorder, authorizedRequest(http.MethodPost, "/api/v1/exam-attempts/900/questions/901/grade", []byte(`{
		"tenant_id": 10,
		"exam_id": 1,
		"space_id": 301,
		"answer_version": 7,
		"score": "4.5",
		"comment": "要点完整"
	}`), authHeader))
	if gradeRecorder.Code != http.StatusOK {
		t.Fatalf("grade status = %d, body = %s", gradeRecorder.Code, gradeRecorder.Body.String())
	}
	var gradedAnswer struct {
		Score         string
		GradingStatus string
		GraderComment string
	}
	if err := gormDB.Table("exam_answers").
		Select("score, grading_status, grader_comment").
		Where("tenant_id = ? AND attempt_id = ? AND attempt_question_id = ?", 10, 900, 901).
		Scan(&gradedAnswer).Error; err != nil {
		t.Fatalf("query graded answer: %v", err)
	}
	if gradedAnswer.Score != "4.5" || gradedAnswer.GradingStatus != "graded" || gradedAnswer.GraderComment != "要点完整" {
		t.Fatalf("unexpected graded answer: %#v", gradedAnswer)
	}
	var scoredAttempt struct {
		SubjectiveScore string
		TotalScore      string
	}
	if err := gormDB.Table("exam_attempts").
		Select("subjective_score, total_score").
		Where("id = ?", 900).
		Scan(&scoredAttempt).Error; err != nil {
		t.Fatalf("query scored attempt: %v", err)
	}
	if scoredAttempt.SubjectiveScore != "4.5" || scoredAttempt.TotalScore != "6.5" {
		t.Fatalf("unexpected recalculated attempt score: %#v", scoredAttempt)
	}

	configRecorder := httptest.NewRecorder()
	router.ServeHTTP(configRecorder, authorizedRequest(http.MethodPost, "/api/v1/results/publish-config", []byte(`{
		"tenant_id": 10,
		"exam_id": 1,
		"space_id": 301,
		"publish_mode": "manual_publish",
		"score_publish_time": 1779795600000
	}`), authHeader))
	if configRecorder.Code != http.StatusOK {
		t.Fatalf("publish config status = %d, body = %s", configRecorder.Code, configRecorder.Body.String())
	}

	listResultsRecorder := httptest.NewRecorder()
	router.ServeHTTP(listResultsRecorder, authorizedRequest(http.MethodGet, "/api/v1/results?tenant_id=10&exam_id=1&space_id=301", nil, authHeader))
	if listResultsRecorder.Code != http.StatusOK {
		t.Fatalf("list results status = %d, body = %s", listResultsRecorder.Code, listResultsRecorder.Body.String())
	}
	resultsBody := decodeExamAPIResponse[resultListResponse](t, listResultsRecorder.Body.Bytes())
	if len(resultsBody.Data.Items) != 1 || resultsBody.Data.Items[0].StudentName != "张三" || resultsBody.Data.Items[0].TotalScore != "6.5" {
		t.Fatalf("unexpected result list response: %#v", resultsBody.Data.Items)
	}

	exportRecorder := httptest.NewRecorder()
	router.ServeHTTP(exportRecorder, authorizedRequest(http.MethodPost, "/api/v1/results/export", []byte(`{
		"tenant_id": 10,
		"exam_id": 1,
		"space_id": 301
	}`), authHeader))
	if exportRecorder.Code != http.StatusOK {
		t.Fatalf("export status = %d, body = %s", exportRecorder.Code, exportRecorder.Body.String())
	}
	exportBody := decodeExamAPIResponse[resultExportResponse](t, exportRecorder.Body.Bytes())
	if exportBody.Data.RowCount != 1 || !strings.Contains(exportBody.Data.FilePath, "exam-1-scores") {
		t.Fatalf("unexpected export response: %#v", exportBody.Data)
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
	`, 100, 10, "高一语文月考试卷", constant.BuildModeManual, fixedAPINow, fixedAPINow).Error; err != nil {
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

func seedExamBusinessLoginAPITestData(t *testing.T, gormDB *gorm.DB) {
	t.Helper()
	passwordHash, err := crypto.HashPassword("papermind123")
	if err != nil {
		t.Fatalf("hash exam business password: %v", err)
	}

	if err := gormDB.Exec(`
		INSERT INTO users (
			id, tenant_id, username, real_name, phone, email, password_hash, status,
			created_at, updated_at, ext_json
		) VALUES (501, 10, 'teacher.exam', '考试教师', '13800000501', 'teacher.exam@example.test', ?, 'enabled', ?, ?, '{}')
	`, passwordHash, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed exam business user: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO user_roles (
			id, tenant_id, user_id, role, created_at, updated_at, ext_json
		) VALUES (501, 10, 501, 'teacher', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed exam business role: %v", err)
	}
}

func seedTakingAPITestData(t *testing.T, gormDB *gorm.DB) {
	t.Helper()
	passwordHash, err := crypto.HashPassword("papermind123")
	if err != nil {
		t.Fatalf("hash tenant password: %v", err)
	}

	if err := gormDB.Exec(`
		INSERT INTO users (
			id, tenant_id, username, real_name, phone, email, password_hash, status,
			created_at, updated_at, ext_json
		) VALUES
			(20, 10, 'student20', '目标考生', '13800000020', 'student20@example.test', ?, 'enabled', ?, ?, '{}'),
			(21, 10, 'student21', '非目标考生', '13800000021', 'student21@example.test', ?, 'enabled', ?, ?, '{}')
	`, passwordHash, fixedAPINow, fixedAPINow, passwordHash, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed taking users: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO user_roles (
			id, tenant_id, user_id, role, created_at, updated_at, ext_json
		) VALUES
			(20, 10, 20, 'student', ?, ?, '{}'),
			(21, 10, 21, 'student', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed taking user roles: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO exam_targets (
			id, tenant_id, exam_id, target_type, target_id, created_at, ext_json
		) VALUES (20, 10, 1, 'user', 20, ?, '{}')
	`, fixedAPINow).Error; err != nil {
		t.Fatalf("seed taking exam target: %v", err)
	}

	if err := gormDB.Exec(`
		INSERT INTO paper_sections (
			id, tenant_id, paper_id, sort_order, name, question_type, instructions, total_score, question_count,
			created_at, updated_at, ext_json
		) VALUES (1, 10, 100, 1, '一、单项选择题', ?, '每题 2 分', 2, 1, ?, ?, '{}')
	`, constant.QuestionTypeSingle, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed paper section: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO questions (
			id, tenant_id, type, difficulty, title, analysis, score_default, shuffle_options, status,
			created_at, updated_at, ext_json
		) VALUES (1001, 10, ?, 'easy', '服务端题干', '服务端解析', 2, FALSE, 'enabled', ?, ?, '{}')
	`, constant.QuestionTypeSingle, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed question: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO question_options (
			id, tenant_id, question_id, option_key, sort_order, content, is_correct, is_distractor,
			created_at, updated_at, ext_json
		) VALUES
			(101, 10, 1001, 'A', 1, '正确选项', TRUE, FALSE, ?, ?, '{}'),
			(102, 10, 1001, 'B', 2, '干扰项', FALSE, TRUE, ?, ?, '{}')
	`, fixedAPINow, fixedAPINow, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed options: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO paper_section_questions (
			id, tenant_id, section_id, paper_id, question_id, sort_order, score,
			created_at, updated_at, ext_json
		) VALUES (1, 10, 1, 100, 1001, 1, 2, ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed paper question: %v", err)
	}
}

func seedReviewResultAPITestData(t *testing.T, gormDB *gorm.DB) {
	t.Helper()
	passwordHash, err := crypto.HashPassword("papermind123")
	if err != nil {
		t.Fatalf("hash review password: %v", err)
	}

	if err := gormDB.Exec(`
		INSERT INTO spaces (
			id, tenant_id, name, type, status, created_at, updated_at, ext_json
		) VALUES (301, 10, '高一 1 班', 'class', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed space: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO users (
			id, tenant_id, username, real_name, phone, email, password_hash, status,
			created_at, updated_at, ext_json
		) VALUES
			(501, 10, 'teacher.li', '李老师', '13800000501', 'teacher501@example.test', ?, 'enabled', ?, ?, '{}'),
			(601, 10, 'student.zhang', '张三', '13800000601', 'student601@example.test', 'hash', 'enabled', ?, ?, '{}')
	`, passwordHash, fixedAPINow, fixedAPINow, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed users: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO user_roles (
			id, tenant_id, user_id, role, created_at, updated_at, ext_json
		) VALUES (501, 10, 501, 'teacher', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed review user role: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO space_members (
			id, tenant_id, space_id, user_id, role_in_space, status, created_at, updated_at, ext_json
		) VALUES
			(701, 10, 301, 501, 'teacher', 'enabled', ?, ?, '{}'),
			(702, 10, 301, 601, 'student', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed space members: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO exam_targets (
			id, tenant_id, exam_id, target_type, target_id, created_at, ext_json
		) VALUES (801, 10, 1, 'space', 301, ?, '{}')
	`, fixedAPINow).Error; err != nil {
		t.Fatalf("seed exam target: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO exam_attempts (
			id, tenant_id, exam_id, user_id, attempt_no, status, started_at, submitted_at,
			exam_token_hash, exam_token_expires_at, objective_score, subjective_score, total_score,
			created_at, updated_at, version, ext_json
		) VALUES (900, 10, 1, 601, 1, 'submitted', ?, ?, 'hash', ?, 2, 0, 2, ?, ?, 3, '{}')
	`, fixedAPINow-3_600_000, fixedAPINow, fixedAPINow+3_600_000, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed attempt: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO exam_attempt_questions (
			id, tenant_id, attempt_id, section_id, question_id, section_snapshot, sort_order, score,
			question_snapshot, option_snapshot, correct_answer_snapshot, created_at, updated_at, version, ext_json
		) VALUES (
			901, 10, 900, 1, 1001, '{"name":"四、简答题","instructions":"结合文本作答"}', 1, 10,
			'{"title":"岳阳楼记思想内涵","type":"short_text"}', '[]', '{"text":"先忧后乐"}', ?, ?, 1, '{}'
		)
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed attempt question: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO exam_answers (
			id, tenant_id, attempt_id, attempt_question_id, answer_content, score, grading_status,
			created_at, updated_at, version, ext_json
		) VALUES (902, 10, 900, 901, '先忧后乐体现了责任意识。', 0, 'pending', ?, ?, 7, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed answer: %v", err)
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

func tenantAuthHeader(t *testing.T, router *gin.Engine, tenantID uint64, username string, password string) string {
	t.Helper()

	loginRecorder := httptest.NewRecorder()
	body := fmt.Sprintf(`{"tenant_id":%d,"username":%q,"password":%q}`, tenantID, username, password)
	router.ServeHTTP(loginRecorder, httptest.NewRequest(
		http.MethodPost,
		"/api/v1/auth/tenant/login",
		bytes.NewReader([]byte(body)),
	))
	if loginRecorder.Code != http.StatusOK {
		t.Fatalf("tenant login status = %d, body = %s", loginRecorder.Code, loginRecorder.Body.String())
	}
	responseBody := decodeExamAPIResponse[authSessionResponse](t, loginRecorder.Body.Bytes())
	return "Bearer " + responseBody.Data.AccessToken
}

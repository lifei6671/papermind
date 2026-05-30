package v1

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/papermind/server/bootstrap/migration"
	serviceexam "github.com/lifei6671/papermind/server/internal/service/exam"
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
	seedSpaceAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:            gormDB,
		Now:           func() int64 { return fixedAPINow },
		CodeGenerator: fixedCodeGenerator{code: "PM2027"},
	})
	authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

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
		"target_id": 100,
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
	if publishBody.Data.TargetType != "space" || publishBody.Data.TargetID != 100 {
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

func TestExamPublishRejectsOtherSpacePaperOrTargetWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamBusinessLoginAPITestData(t, gormDB)
	seedPaperTeacherSpaceAPITestData(t, gormDB)
	seedPaperAPITestData(t, gormDB)
	seedOtherSpaceExamPublishAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:            gormDB,
		Now:           func() int64 { return fixedAPINow },
		CodeGenerator: fixedCodeGenerator{code: "PM2028"},
	})
	authHeader := tenantAuthHeader(t, router, 10, "teacher.exam", "papermind123")

	for _, tc := range []struct {
		name    string
		payload []byte
	}{
		{
			name: "other-space paper",
			payload: []byte(`{
				"tenant_id": 10,
				"paper_id": 201,
				"name": "跨空间试卷发布",
				"target_type": "space",
				"target_id": 301,
				"start_time": 1772269200000,
				"end_time": 1772276400000,
				"duration_minutes": 120,
				"max_attempts": 1,
				"result_strategy": "latest",
				"publish_mode": "manual_publish"
			}`),
		},
		{
			name: "other-space target",
			payload: []byte(`{
				"tenant_id": 10,
				"paper_id": 100,
				"name": "跨空间目标发布",
				"target_type": "space",
				"target_id": 302,
				"start_time": 1772269200000,
				"end_time": 1772276400000,
				"duration_minutes": 120,
				"max_attempts": 1,
				"result_strategy": "latest",
				"publish_mode": "manual_publish"
			}`),
		},
		{
			name: "other-space user target",
			payload: []byte(`{
				"tenant_id": 10,
				"paper_id": 100,
				"name": "跨空间用户目标发布",
				"target_type": "user",
				"target_id": 602,
				"start_time": 1772269200000,
				"end_time": 1772276400000,
				"duration_minutes": 120,
				"max_attempts": 1,
				"result_strategy": "latest",
				"publish_mode": "manual_publish"
			}`),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, authorizedRequest(http.MethodPost, "/api/v1/exams", tc.payload, authHeader))
			if recorder.Code != http.StatusForbidden {
				t.Fatalf("expected publish forbidden, got status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
		})
	}

	var examCount int64
	if err := gormDB.Table("exams").Where("tenant_id = ?", 10).Count(&examCount).Error; err != nil {
		t.Fatalf("count exams: %v", err)
	}
	if examCount != 0 {
		t.Fatalf("expected forbidden publish not to create draft exams, got %d", examCount)
	}
}

func TestTenantAdminPublishRejectsMissingTargetWithSQLite(t *testing.T) {
	for _, tc := range []struct {
		name    string
		payload []byte
	}{
		{
			name: "missing space target",
			payload: []byte(`{
				"tenant_id": 10,
				"paper_id": 100,
				"name": "不存在空间目标发布",
				"target_type": "space",
				"target_id": 999,
				"start_time": 1772269200000,
				"end_time": 1772276400000,
				"duration_minutes": 120,
				"max_attempts": 1,
				"result_strategy": "latest",
				"publish_mode": "manual_publish"
			}`),
		},
		{
			name: "missing user target",
			payload: []byte(`{
				"tenant_id": 10,
				"paper_id": 100,
				"name": "不存在用户目标发布",
				"target_type": "user",
				"target_id": 999,
				"start_time": 1772269200000,
				"end_time": 1772276400000,
				"duration_minutes": 120,
				"max_attempts": 1,
				"result_strategy": "latest",
				"publish_mode": "manual_publish"
			}`),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			gormDB := openExamAPITestDB(t)
			seedExamAPITestData(t, gormDB)
			seedSpaceAPITestData(t, gormDB)

			router := NewRouter(RouterOptions{
				DB:            gormDB,
				Now:           func() int64 { return fixedAPINow },
				CodeGenerator: fixedCodeGenerator{code: "PM3029"},
			})
			authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, authorizedRequest(http.MethodPost, "/api/v1/exams", tc.payload, authHeader))
			if recorder.Code != http.StatusForbidden && recorder.Code != http.StatusBadRequest {
				t.Fatalf("expected missing target to be rejected, got status = %d, body = %s", recorder.Code, recorder.Body.String())
			}

			var examCount int64
			if err := gormDB.Table("exams").Where("tenant_id = ? AND id <> ?", 10, 1).Count(&examCount).Error; err != nil {
				t.Fatalf("count created exams: %v", err)
			}
			if examCount != 0 {
				t.Fatalf("missing target publish should not create draft exams, got %d", examCount)
			}
		})
	}
}

func TestExamPublishFailureDoesNotLeaveDraftWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedSpaceAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:            gormDB,
		Now:           func() int64 { return fixedAPINow },
		CodeGenerator: fixedCodeGenerator{code: "PM3029"},
	})
	authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodPost, "/api/v1/exams", []byte(`{
		"tenant_id": 10,
		"paper_id": 100,
		"name": "失败发布不应留草稿",
		"target_type": "space",
		"target_id": 100,
		"start_time": 1772269200000,
		"end_time": 1772272800000,
		"duration_minutes": 120,
		"max_attempts": 1,
		"result_strategy": "latest",
		"publish_mode": "manual_publish"
	}`), authHeader))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid publish to fail, got status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var createdCount int64
	if err := gormDB.Table("exams").
		Where("tenant_id = ? AND name = ?", 10, "失败发布不应留草稿").
		Count(&createdCount).Error; err != nil {
		t.Fatalf("count failed publish exams: %v", err)
	}
	if createdCount != 0 {
		t.Fatalf("failed publish should not leave draft exams, got %d", createdCount)
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

	for _, tc := range []struct {
		name   string
		method string
		target string
		body   []byte
	}{
		{name: "exam list", method: http.MethodGet, target: "/api/v1/exams?tenant_id=10"},
		{name: "exam publish", method: http.MethodPost, target: "/api/v1/exams", body: []byte(`{
			"tenant_id": 10,
			"paper_id": 100,
			"name": "平台越权发布",
			"target_type": "space",
			"target_id": 200,
			"start_time": 1772269200000,
			"end_time": 1772276400000,
			"duration_minutes": 120,
			"max_attempts": 1,
			"result_strategy": "latest",
			"publish_mode": "manual_publish"
		}`)},
		{name: "question create", method: http.MethodPost, target: "/api/v1/questions", body: []byte(`{
			"tenant_id": 10,
			"type": "single",
			"difficulty": "easy",
			"title": "平台越权题目",
			"score_default": "4",
			"options": [
				{"option_key":"A","content":"正确","is_correct":true},
				{"option_key":"B","content":"错误","is_correct":false}
			]
		}`)},
		{name: "paper section create", method: http.MethodPost, target: "/api/v1/papers/100/sections", body: []byte(`{
			"tenant_id": 10,
			"sort_order": 1,
			"name": "平台越权大题",
			"question_type": "single"
		}`)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, authorizedRequest(tc.method, tc.target, tc.body, authHeader))
			if recorder.Code != http.StatusForbidden {
				t.Fatalf("platform %s status = %d, body = %s", tc.name, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestTenantBusinessAPIsRejectWrongTenantOrStudentRole(t *testing.T) {
	t.Run("wrong tenant id", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		gormDB := openExamAPITestDB(t)
		seedExamAPITestData(t, gormDB)
		seedExamBusinessLoginAPITestData(t, gormDB)

		router := NewRouter(RouterOptions{
			DB:            gormDB,
			Now:           func() int64 { return fixedAPINow },
			CodeGenerator: fixedCodeGenerator{code: "PM2029"},
		})
		authHeader := tenantAuthHeader(t, router, 10, "teacher.exam", "papermind123")

		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, authorizedRequest(http.MethodPost, "/api/v1/exams", []byte(`{
			"tenant_id": 11,
			"paper_id": 100,
			"name": "跨租户发布",
			"target_type": "space",
			"target_id": 200,
			"start_time": 1772269200000,
			"end_time": 1772276400000,
			"duration_minutes": 120,
			"max_attempts": 1,
			"result_strategy": "latest",
			"publish_mode": "manual_publish"
		}`), authHeader))
		if recorder.Code != http.StatusForbidden {
			t.Fatalf("wrong tenant publish status = %d, body = %s", recorder.Code, recorder.Body.String())
		}
	})

	t.Run("student role", func(t *testing.T) {
		gin.SetMode(gin.TestMode)
		gormDB := openExamAPITestDB(t)
		seedExamAPITestData(t, gormDB)
		seedTakingAPITestData(t, gormDB)

		router := NewRouter(RouterOptions{
			DB:  gormDB,
			Now: func() int64 { return fixedAPINow },
		})
		authHeader := tenantAuthHeader(t, router, 10, "student20", "papermind123")

		for _, tc := range []struct {
			name   string
			method string
			target string
		}{
			{name: "exam management", method: http.MethodGet, target: "/api/v1/exams?tenant_id=10"},
			{name: "grading", method: http.MethodGet, target: "/api/v1/grading/pending?tenant_id=10&exam_id=1&space_id=301"},
			{name: "results", method: http.MethodGet, target: "/api/v1/results?tenant_id=10&exam_id=1&space_id=301"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, authorizedRequest(tc.method, tc.target, nil, authHeader))
				if recorder.Code != http.StatusForbidden {
					t.Fatalf("student %s status = %d, body = %s", tc.name, recorder.Code, recorder.Body.String())
				}
			})
		}
	})
}

func TestSpaceAdminMembershipCanEnterExamBusinessAPIs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamBusinessLoginAPITestData(t, gormDB)
	seedPaperTeacherSpaceAPITestData(t, gormDB)
	seedStudentSpaceAdminExamBusinessTestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "student.space.admin", "papermind123")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodGet, "/api/v1/questions?tenant_id=10&space_id=301", nil, authHeader))
	if recorder.Code != http.StatusOK {
		t.Fatalf("space admin exam business status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestExamBusinessListAPIsRejectUnauthorizedSpaceScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedExamBusinessLoginAPITestData(t, gormDB)
	seedPaperTeacherSpaceAPITestData(t, gormDB)
	seedPaperRuleAPITestData(t, gormDB)
	seedOtherSpaceQuestionAPITestData(t, gormDB)
	seedOtherSpaceOnlyExamForPublishConfigTest(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "teacher.exam", "papermind123")

	for _, tc := range []struct {
		name   string
		method string
		path   string
	}{
		{name: "questions", method: http.MethodGet, path: "/api/v1/questions?tenant_id=10&space_id=302"},
		{name: "papers", method: http.MethodGet, path: "/api/v1/papers?tenant_id=10&space_id=302"},
		{name: "exams", method: http.MethodGet, path: "/api/v1/exams?tenant_id=10&space_id=302"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, authorizedRequest(tc.method, tc.path, nil, authHeader))
			if recorder.Code != http.StatusForbidden {
				t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
		})
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

func TestExamEntrySessionIsClearedWhenTenantUserLogsInAgain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedTakingAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	student20AuthHeader := tenantAuthHeader(t, router, 10, "student20", "papermind123")

	resolveRecorder := httptest.NewRecorder()
	router.ServeHTTP(resolveRecorder, authorizedRequest(http.MethodPost, "/api/v1/exam-entry/invite/resolve", []byte(`{
		"invite_code": "PM2026"
	}`), student20AuthHeader))
	if resolveRecorder.Code != http.StatusOK {
		t.Fatalf("resolve invite status = %d, body = %s", resolveRecorder.Code, resolveRecorder.Body.String())
	}

	loginRecorder := httptest.NewRecorder()
	loginRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tenant/login", bytes.NewReader([]byte(`{
		"username": "student21",
		"password": "papermind123"
	}`)))
	for _, cookie := range resolveRecorder.Result().Cookies() {
		loginRequest.AddCookie(cookie)
	}
	router.ServeHTTP(loginRecorder, loginRequest)
	if loginRecorder.Code != http.StatusOK {
		t.Fatalf("second login status = %d, body = %s", loginRecorder.Code, loginRecorder.Body.String())
	}

	startRecorder := httptest.NewRecorder()
	startRequest := httptest.NewRequest(http.MethodPost, "/api/v1/exam-entry/exams/1/attempts/start", bytes.NewReader([]byte(`{
		"tenant_id": 10
	}`)))
	for _, cookie := range loginRecorder.Result().Cookies() {
		startRequest.AddCookie(cookie)
	}
	router.ServeHTTP(startRecorder, startRequest)
	if startRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected second login to clear stale exam entry session, got status = %d, body = %s", startRecorder.Code, startRecorder.Body.String())
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
	router.ServeHTTP(saveRecorder, httptest.NewRequest(http.MethodPost, "/api/v1/exam-entry/attempts/"+
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
	router.ServeHTTP(submitRecorder, httptest.NewRequest(http.MethodPost, "/api/v1/exam-entry/attempts/"+
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

func TestLegacyExamAttemptWriteRoutesAreNotRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedTakingAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	legacyRoutes := []struct {
		name string
		path string
	}{
		{name: "save answer", path: "/api/v1/exam-attempts/1/answers/1"},
		{name: "submit", path: "/api/v1/exam-attempts/1/submit"},
		{name: "event", path: "/api/v1/exam-attempts/1/events"},
	}
	for _, route := range legacyRoutes {
		t.Run(route.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, route.path, bytes.NewReader([]byte(`{"tenant_id":10,"exam_token":"token","question_type":"single","event_type":"blur"}`))))
			if recorder.Code != http.StatusNotFound {
				t.Fatalf("expected legacy route %s to be removed, got status = %d, body = %s", route.path, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestExamEntryResolveKeepsConfiguredAuthSessionTTL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedTakingAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:             gormDB,
		Now:            func() int64 { return fixedAPINow },
		AuthSessionTTL: 7200,
	})
	authHeader := tenantAuthHeader(t, router, 10, "student20", "papermind123")

	resolveRecorder := httptest.NewRecorder()
	router.ServeHTTP(resolveRecorder, authorizedRequest(http.MethodPost, "/api/v1/exam-entry/invite/resolve", []byte(`{
		"invite_code": "PM2026"
	}`), authHeader))
	if resolveRecorder.Code != http.StatusOK {
		t.Fatalf("resolve invite status = %d, body = %s", resolveRecorder.Code, resolveRecorder.Body.String())
	}
	cookies := resolveRecorder.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("expected resolving invite to persist exam entry session")
	}
	if cookies[0].MaxAge != 7200 {
		t.Fatalf("expected auth session TTL to stay 7200 seconds, got %d", cookies[0].MaxAge)
	}
}

func TestLoginSessionCannotWriteExamEntryWithoutExamToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedTakingAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "student20", "papermind123")

	for _, tc := range []struct {
		name    string
		path    string
		payload []byte
	}{
		{
			name: "save answer",
			path: "/api/v1/exam-entry/attempts/1/answers/1",
			payload: []byte(`{
				"tenant_id": 10,
				"question_type": "single"
			}`),
		},
		{
			name: "submit",
			path: "/api/v1/exam-entry/attempts/1/submit",
			payload: []byte(`{
				"tenant_id": 10
			}`),
		},
		{
			name: "event",
			path: "/api/v1/exam-entry/attempts/1/events",
			payload: []byte(`{
				"tenant_id": 10,
				"event_type": "blur"
			}`),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, authorizedRequest(http.MethodPost, tc.path, tc.payload, authHeader))
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("expected missing exam_token to be rejected, got status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestExamTokenCannotAccessManagementAPIs(t *testing.T) {
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
	router.ServeHTTP(resolveRecorder, authorizedRequest(http.MethodPost, "/api/v1/exam-entry/invite/resolve", []byte(`{
		"invite_code": "PM2026"
	}`), authHeader))
	if resolveRecorder.Code != http.StatusOK {
		t.Fatalf("resolve invite status = %d, body = %s", resolveRecorder.Code, resolveRecorder.Body.String())
	}
	startRecorder := httptest.NewRecorder()
	startRequest := httptest.NewRequest(http.MethodPost, "/api/v1/exam-entry/exams/1/attempts/start", bytes.NewReader([]byte(`{
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
	examTokenHeader := "Bearer " + startBody.Data.ExamToken

	for _, tc := range []struct {
		name string
		path string
	}{
		{name: "profile", path: "/api/v1/profile"},
		{name: "platform tenants", path: "/api/v1/tenants"},
		{name: "tenant spaces", path: "/api/v1/spaces?tenant_id=10"},
		{name: "tenant users", path: "/api/v1/users?tenant_id=10"},
		{name: "questions", path: "/api/v1/questions?tenant_id=10"},
		{name: "papers", path: "/api/v1/papers?tenant_id=10"},
		{name: "grading", path: "/api/v1/grading/pending?tenant_id=10&exam_id=1&space_id=301"},
		{name: "results", path: "/api/v1/results?tenant_id=10&exam_id=1&space_id=301"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, authorizedRequest(http.MethodGet, tc.path, nil, examTokenHeader))
			if recorder.Code != http.StatusUnauthorized && recorder.Code != http.StatusForbidden {
				t.Fatalf("expected exam token to be rejected, got status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestExamEntryStartReturnsOpaqueTokenAndStoresOnlyHash(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedTakingAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "student20", "papermind123")
	started := startExamEntryAttemptForTest(t, router, authHeader)

	decodedToken, err := base64.RawURLEncoding.DecodeString(started.ExamToken)
	if err != nil {
		t.Fatalf("exam token should be raw URL base64: %v", err)
	}
	if len(decodedToken) != 32 {
		t.Fatalf("expected 32 random token bytes, got %d", len(decodedToken))
	}

	var row struct {
		ExamTokenHash string
	}
	if err := gormDB.Table("exam_attempts").
		Select("exam_token_hash").
		Where("tenant_id = ? AND id = ?", 10, started.Attempt.ID).
		Scan(&row).Error; err != nil {
		t.Fatalf("query attempt token hash: %v", err)
	}
	if row.ExamTokenHash == "" || row.ExamTokenHash == started.ExamToken {
		t.Fatalf("expected database to store only token hash, got %q", row.ExamTokenHash)
	}
	if row.ExamTokenHash != serviceexam.HashExamToken(started.ExamToken) {
		t.Fatalf("stored token hash does not match returned token")
	}

	restarted := startExamEntryAttemptForTest(t, router, authHeader)
	if restarted.Attempt.ID != started.Attempt.ID {
		t.Fatalf("expected repeated start to reuse attempt %d, got %d", started.Attempt.ID, restarted.Attempt.ID)
	}
	if restarted.ExamToken == "" || restarted.ExamToken == started.ExamToken {
		t.Fatalf("expected repeated start to renew exam token, got %q", restarted.ExamToken)
	}
	var restartedRow struct {
		ExamTokenHash string
	}
	if err := gormDB.Table("exam_attempts").
		Select("exam_token_hash").
		Where("tenant_id = ? AND id = ?", 10, started.Attempt.ID).
		Scan(&restartedRow).Error; err != nil {
		t.Fatalf("query restarted attempt token hash: %v", err)
	}
	if restartedRow.ExamTokenHash == row.ExamTokenHash || restartedRow.ExamTokenHash != serviceexam.HashExamToken(restarted.ExamToken) {
		t.Fatalf("expected repeated start to update token hash, before=%q after=%q", row.ExamTokenHash, restartedRow.ExamTokenHash)
	}
}

func TestExamEntryWriteAPIsRejectExpiredTokenSubmittedAttemptAndExpiredAnswerWindow(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(t *testing.T, gormDB *gorm.DB, now *int64, started startAttemptResponse)
	}{
		{
			name: "expired token",
			mutate: func(t *testing.T, gormDB *gorm.DB, now *int64, started startAttemptResponse) {
				*now = fixedAPINow + 126*60_000
			},
		},
		{
			name: "submitted attempt",
			mutate: func(t *testing.T, gormDB *gorm.DB, now *int64, started startAttemptResponse) {
				if err := gormDB.Table("exam_attempts").
					Where("tenant_id = ? AND id = ?", 10, started.Attempt.ID).
					Updates(map[string]any{
						"status":       constant.AttemptStatusSubmitted,
						"submitted_at": fixedAPINow,
					}).Error; err != nil {
					t.Fatalf("mark attempt submitted: %v", err)
				}
			},
		},
		{
			name: "expired answer window",
			mutate: func(t *testing.T, gormDB *gorm.DB, now *int64, started startAttemptResponse) {
				*now = fixedAPINow + 120*60_000 + 5_001
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			gormDB := openExamAPITestDB(t)
			seedExamAPITestData(t, gormDB)
			seedTakingAPITestData(t, gormDB)

			now := fixedAPINow
			router := NewRouter(RouterOptions{
				DB:  gormDB,
				Now: func() int64 { return now },
			})
			authHeader := tenantAuthHeader(t, router, 10, "student20", "papermind123")
			started := startExamEntryAttemptForTest(t, router, authHeader)
			if len(started.Questions) != 1 {
				t.Fatalf("expected one started question, got %#v", started.Questions)
			}
			tc.mutate(t, gormDB, &now, started)

			attemptID := strconv.FormatUint(started.Attempt.ID, 10)
			questionID := strconv.FormatUint(started.Questions[0].ID, 10)
			for _, write := range []struct {
				name    string
				path    string
				payload []byte
			}{
				{
					name: "save answer",
					path: "/api/v1/exam-entry/attempts/" + attemptID + "/answers/" + questionID,
					payload: []byte(fmt.Sprintf(`{
						"tenant_id": 10,
						"exam_token": %q,
						"question_type": %q,
						"option_ids": [101]
					}`, started.ExamToken, constant.QuestionTypeSingle)),
				},
				{
					name: "submit",
					path: "/api/v1/exam-entry/attempts/" + attemptID + "/submit",
					payload: []byte(fmt.Sprintf(`{
						"tenant_id": 10,
						"exam_token": %q
					}`, started.ExamToken)),
				},
				{
					name: "event",
					path: "/api/v1/exam-entry/attempts/" + attemptID + "/events",
					payload: []byte(fmt.Sprintf(`{
						"tenant_id": 10,
						"exam_token": %q,
						"event_type": "blur"
					}`, started.ExamToken)),
				},
			} {
				t.Run(write.name, func(t *testing.T) {
					recorder := httptest.NewRecorder()
					router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, write.path, bytes.NewReader(write.payload)))
					if recorder.Code != http.StatusBadRequest {
						t.Fatalf("expected write to be rejected, got status = %d, body = %s", recorder.Code, recorder.Body.String())
					}
				})
			}
		})
	}
}

func TestExamEntryWriteAPIsUseExamTokenWithoutManagementSession(t *testing.T) {
	for _, tc := range []struct {
		name  string
		build func(started startAttemptResponse) (string, []byte)
	}{
		{
			name: "save answer",
			build: func(started startAttemptResponse) (string, []byte) {
				attemptID := strconv.FormatUint(started.Attempt.ID, 10)
				questionID := strconv.FormatUint(started.Questions[0].ID, 10)
				return "/api/v1/exam-entry/attempts/" + attemptID + "/answers/" + questionID, []byte(fmt.Sprintf(`{
					"tenant_id": 10,
					"exam_token": %q,
					"question_type": %q,
					"option_ids": [101]
				}`, started.ExamToken, constant.QuestionTypeSingle))
			},
		},
		{
			name: "record event",
			build: func(started startAttemptResponse) (string, []byte) {
				attemptID := strconv.FormatUint(started.Attempt.ID, 10)
				return "/api/v1/exam-entry/attempts/" + attemptID + "/events", []byte(fmt.Sprintf(`{
					"tenant_id": 10,
					"exam_token": %q,
					"event_type": "blur"
				}`, started.ExamToken))
			},
		},
		{
			name: "submit",
			build: func(started startAttemptResponse) (string, []byte) {
				attemptID := strconv.FormatUint(started.Attempt.ID, 10)
				return "/api/v1/exam-entry/attempts/" + attemptID + "/submit", []byte(fmt.Sprintf(`{
					"tenant_id": 10,
					"exam_token": %q
				}`, started.ExamToken))
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			gormDB := openExamAPITestDB(t)
			seedExamAPITestData(t, gormDB)
			seedTakingAPITestData(t, gormDB)

			router := NewRouter(RouterOptions{
				DB:  gormDB,
				Now: func() int64 { return fixedAPINow },
			})
			authHeader := tenantAuthHeader(t, router, 10, "student20", "papermind123")
			started := startExamEntryAttemptForTest(t, router, authHeader)
			if len(started.Questions) != 1 {
				t.Fatalf("expected one started question, got %#v", started.Questions)
			}
			path, payload := tc.build(started)

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload)))
			if recorder.Code != http.StatusOK {
				t.Fatalf("expected exam_token-only request to succeed, got status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestExamEntryMiddlewareValidatesTokenBeforeHandlerPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedTakingAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "student20", "papermind123")
	started := startExamEntryAttemptForTest(t, router, authHeader)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(
		http.MethodPost,
		"/api/v1/exam-entry/attempts/"+strconv.FormatUint(started.Attempt.ID, 10)+"/answers/"+strconv.FormatUint(started.Questions[0].ID, 10),
		bytes.NewReader([]byte(`{
			"tenant_id": 10,
			"exam_token": "invalid-token"
		}`)),
	))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid token to be rejected, got status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var body apiBodyForTest[any]
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response %s: %v", recorder.Body.String(), err)
	}
	if body.Message != "exam token invalid" {
		t.Fatalf("expected middleware token validation before handler payload validation, got message = %q", body.Message)
	}
}

func TestExamEntryMiddlewareRejectsOversizedBodyBeforeTokenValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedTakingAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	body := []byte(`{"tenant_id":10,"exam_token":"invalid-token","padding":"` + strings.Repeat("x", 70*1024) + `"}`)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(
		http.MethodPost,
		"/api/v1/exam-entry/attempts/1/submit",
		bytes.NewReader(body),
	))

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected oversized body rejected before token validation, got status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestExamEntrySubmitRejectsNonSubmitEventType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedTakingAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "student20", "papermind123")
	started := startExamEntryAttemptForTest(t, router, authHeader)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(
		http.MethodPost,
		"/api/v1/exam-entry/attempts/"+strconv.FormatUint(started.Attempt.ID, 10)+"/submit",
		bytes.NewReader([]byte(fmt.Sprintf(`{
			"tenant_id": 10,
			"exam_token": %q,
			"event_type": "blur"
		}`, started.ExamToken))),
	))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid submit event type rejected, got status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var status string
	if err := gormDB.Table("exam_attempts").
		Select("status").
		Where("tenant_id = ? AND id = ?", 10, started.Attempt.ID).
		Scan(&status).Error; err != nil {
		t.Fatalf("query attempt status: %v", err)
	}
	if status != constant.AttemptStatusInProgress {
		t.Fatalf("invalid submit event should not submit attempt, got status %q", status)
	}
}

func TestExamEntryRecordEventRejectsSubmitEventType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedTakingAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "student20", "papermind123")
	started := startExamEntryAttemptForTest(t, router, authHeader)

	recorder := httptest.NewRecorder()
	body := fmt.Sprintf(`{"tenant_id":10,"exam_token":%q,"event_type":"submit","payload":"{}"}`, started.ExamToken)
	router.ServeHTTP(recorder, httptest.NewRequest(
		http.MethodPost,
		"/api/v1/exam-entry/attempts/"+strconv.FormatUint(started.Attempt.ID, 10)+"/events",
		bytes.NewReader([]byte(body)),
	))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected submit event type rejected from events endpoint, got status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var submitEventCount int64
	if err := gormDB.Table("exam_events").
		Where("tenant_id = ? AND attempt_id = ? AND event_type = ?", 10, started.Attempt.ID, constant.ExamEventTypeSubmit).
		Count(&submitEventCount).Error; err != nil {
		t.Fatalf("count submit event: %v", err)
	}
	if submitEventCount != 0 {
		t.Fatalf("expected no submit event from events endpoint, got %d", submitEventCount)
	}
}

func TestExamEntryMiddlewareRejectsTenantAndAttemptMismatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedTakingAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "student20", "papermind123")
	started := startExamEntryAttemptForTest(t, router, authHeader)

	for _, tc := range []struct {
		name      string
		attemptID uint64
		tenantID  uint64
	}{
		{name: "tenant mismatch", attemptID: started.Attempt.ID, tenantID: 20},
		{name: "attempt mismatch", attemptID: started.Attempt.ID + 999, tenantID: 10},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(
				http.MethodPost,
				"/api/v1/exam-entry/attempts/"+strconv.FormatUint(tc.attemptID, 10)+"/answers/"+strconv.FormatUint(started.Questions[0].ID, 10),
				bytes.NewReader([]byte(fmt.Sprintf(`{
					"tenant_id": %d,
					"exam_token": %q,
					"question_type": %q,
					"option_ids": [101]
				}`, tc.tenantID, started.ExamToken, constant.QuestionTypeSingle))),
			))
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("expected mismatch to be rejected, got status = %d, body = %s", recorder.Code, recorder.Body.String())
			}

			var answerCount int64
			if err := gormDB.Table("exam_answers").
				Where("tenant_id = ? AND attempt_id = ?", 10, started.Attempt.ID).
				Count(&answerCount).Error; err != nil {
				t.Fatalf("count answers after rejected write: %v", err)
			}
			if answerCount != 0 {
				t.Fatalf("expected rejected request not to save answers, got %d", answerCount)
			}
		})
	}
}

func TestExamEntryMiddlewareDerivesExamAndUserFromToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedTakingAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "student20", "papermind123")
	started := startExamEntryAttemptForTest(t, router, authHeader)

	// exam-entry 写入口的 exam_id / user_id 由 exam_token 绑定的 attempt 派生，
	// 客户端额外传入的同名字段不能改变保存答案的真实考生身份。
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(
		http.MethodPost,
		"/api/v1/exam-entry/attempts/"+strconv.FormatUint(started.Attempt.ID, 10)+"/answers/"+strconv.FormatUint(started.Questions[0].ID, 10),
		bytes.NewReader([]byte(fmt.Sprintf(`{
			"tenant_id": 10,
			"exam_id": 999,
			"user_id": 999,
			"exam_token": %q,
			"question_type": %q,
			"option_ids": [101]
		}`, started.ExamToken, constant.QuestionTypeSingle))),
	))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected token-scoped save to succeed, got status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var saved struct {
		TenantID  uint64 `gorm:"column:tenant_id"`
		AttemptID uint64 `gorm:"column:attempt_id"`
		UpdatedBy uint64 `gorm:"column:updated_by"`
	}
	if err := gormDB.Table("exam_answers").
		Select("tenant_id", "attempt_id", "updated_by").
		Where("tenant_id = ? AND attempt_id = ? AND attempt_question_id = ?", 10, started.Attempt.ID, started.Questions[0].ID).
		Scan(&saved).Error; err != nil {
		t.Fatalf("query saved answer: %v", err)
	}
	if saved.TenantID != 10 || saved.AttemptID != started.Attempt.ID || saved.UpdatedBy != started.Attempt.UserID {
		t.Fatalf("expected answer scope to come from token-bound attempt, got %#v", saved)
	}
}

func TestExamEntryResultVisibleForOwnerOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedTakingAPITestData(t, gormDB)
	seedVisibleResultAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	ownerAuthHeader := tenantAuthHeader(t, router, 10, "student20", "papermind123")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodGet, "/api/v1/exam-entry/results/700", nil, ownerAuthHeader))
	if recorder.Code != http.StatusOK {
		t.Fatalf("owner result status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	body := decodeExamAPIResponse[visibleResultResponse](t, recorder.Body.Bytes())
	if body.Data.AttemptID != 700 || body.Data.TotalScore != "8" || !body.Data.AnalysisVisible {
		t.Fatalf("unexpected visible result response: %#v", body.Data)
	}

	otherAuthHeader := tenantAuthHeader(t, router, 10, "student21", "papermind123")
	forbiddenRecorder := httptest.NewRecorder()
	router.ServeHTTP(forbiddenRecorder, authorizedRequest(http.MethodGet, "/api/v1/exam-entry/results/700", nil, otherAuthHeader))
	if forbiddenRecorder.Code != http.StatusForbidden {
		t.Fatalf("other student result status = %d, body = %s", forbiddenRecorder.Code, forbiddenRecorder.Body.String())
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
	router.ServeHTTP(eventRecorder, httptest.NewRequest(http.MethodPost, "/api/v1/exam-entry/attempts/"+
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
	if exportRecorder.Code != http.StatusForbidden {
		t.Fatalf("export status = %d, body = %s", exportRecorder.Code, exportRecorder.Body.String())
	}
}

func TestReviewAndResultAPIRoutesRejectForgedSpaceIDWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedReviewResultAPITestData(t, gormDB)
	seedOtherSpaceReviewResultAPITestData(t, gormDB)

	if err := gormDB.Table("space_members").
		Where("tenant_id = ? AND space_id = ? AND user_id = ?", 10, 301, 501).
		Update("role_in_space", "space_admin").Error; err != nil {
		t.Fatalf("promote teacher to space admin: %v", err)
	}

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
	if len(reviewBody.Data.Items) != 1 || reviewBody.Data.Items[0].AttemptID != 900 || reviewBody.Data.Items[0].SpaceName != "高一 1 班" {
		t.Fatalf("expected only authorized-space pending review row, got %#v", reviewBody.Data.Items)
	}

	gradeRecorder := httptest.NewRecorder()
	router.ServeHTTP(gradeRecorder, authorizedRequest(http.MethodPost, "/api/v1/exam-attempts/910/questions/911/grade", []byte(`{
		"tenant_id": 10,
		"exam_id": 1,
		"space_id": 301,
		"answer_version": 11,
		"score": "9",
		"comment": "伪造空间"
	}`), authHeader))
	if gradeRecorder.Code != http.StatusForbidden {
		t.Fatalf("grade forged space status = %d, body = %s", gradeRecorder.Code, gradeRecorder.Body.String())
	}
	var otherSpaceAnswer struct {
		Score         string
		GradingStatus string
		GraderComment string
	}
	if err := gormDB.Table("exam_answers").
		Select("score, grading_status, grader_comment").
		Where("tenant_id = ? AND attempt_id = ? AND attempt_question_id = ?", 10, 910, 911).
		Scan(&otherSpaceAnswer).Error; err != nil {
		t.Fatalf("query other-space answer: %v", err)
	}
	if otherSpaceAnswer.Score != "0" || otherSpaceAnswer.GradingStatus != "pending" || otherSpaceAnswer.GraderComment != "" {
		t.Fatalf("expected other-space answer to remain unmodified, got %#v", otherSpaceAnswer)
	}

	listResultsRecorder := httptest.NewRecorder()
	router.ServeHTTP(listResultsRecorder, authorizedRequest(http.MethodGet, "/api/v1/results?tenant_id=10&exam_id=1&space_id=301", nil, authHeader))
	if listResultsRecorder.Code != http.StatusOK {
		t.Fatalf("list results status = %d, body = %s", listResultsRecorder.Code, listResultsRecorder.Body.String())
	}
	resultsBody := decodeExamAPIResponse[resultListResponse](t, listResultsRecorder.Body.Bytes())
	if len(resultsBody.Data.Items) != 1 || resultsBody.Data.Items[0].StudentName != "张三" || resultsBody.Data.Items[0].SpaceName != "高一 1 班" {
		t.Fatalf("expected only authorized-space result row, got %#v", resultsBody.Data.Items)
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
	if exportBody.Data.RowCount != 1 {
		t.Fatalf("expected export to include only authorized-space row, got %d", exportBody.Data.RowCount)
	}
	var exportRaw map[string]any
	if err := json.Unmarshal(exportRecorder.Body.Bytes(), &exportRaw); err != nil {
		t.Fatalf("decode export raw response: %v", err)
	}
	exportData, ok := exportRaw["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected export data object, got %#v", exportRaw["data"])
	}
	fileURL, _ := exportData["file_url"].(string)
	if !strings.HasPrefix(fileURL, "/api/v1/results/export-files/") {
		t.Fatalf("expected export file_url to be an API download URL, got %q", fileURL)
	}
	filePath, _ := exportData["file_path"].(string)
	if filepath.IsAbs(filePath) || strings.Contains(filePath, "server/data/exports") {
		t.Fatalf("expected export file_path not to expose server filesystem path, got %q", filePath)
	}

	downloadRecorder := httptest.NewRecorder()
	router.ServeHTTP(downloadRecorder, authorizedRequest(http.MethodGet, fileURL, nil, authHeader))
	if downloadRecorder.Code != http.StatusOK {
		t.Fatalf("download export status = %d, body = %s", downloadRecorder.Code, downloadRecorder.Body.String())
	}
	if disposition := downloadRecorder.Header().Get("Content-Disposition"); !strings.Contains(disposition, filePath) {
		t.Fatalf("expected download filename %q in content disposition, got %q", filePath, disposition)
	}
}

func TestResultPublishConfigRejectsForgedSpaceIDWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedReviewResultAPITestData(t, gormDB)
	seedOtherSpaceReviewResultAPITestData(t, gormDB)
	seedOtherSpaceOnlyExamForPublishConfigTest(t, gormDB)

	if err := gormDB.Table("space_members").
		Where("tenant_id = ? AND space_id = ? AND user_id = ?", 10, 301, 501).
		Update("role_in_space", "space_admin").Error; err != nil {
		t.Fatalf("promote teacher to space admin: %v", err)
	}

	router := NewRouter(RouterOptions{
		DB:        gormDB,
		Now:       func() int64 { return fixedAPINow },
		ExportDir: t.TempDir(),
	})
	authHeader := tenantAuthHeader(t, router, 10, "teacher.li", "papermind123")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodPost, "/api/v1/results/publish-config", []byte(`{
		"tenant_id": 10,
		"exam_id": 2,
		"space_id": 301,
		"publish_mode": "manual_publish",
		"score_publish_time": 1779795600000
	}`), authHeader))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("forged publish config status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var exam struct {
		PublishMode      string
		ScorePublishTime *int64
	}
	if err := gormDB.Table("exams").
		Select("publish_mode, score_publish_time").
		Where("tenant_id = ? AND id = ?", 10, 2).
		Scan(&exam).Error; err != nil {
		t.Fatalf("query forged publish exam: %v", err)
	}
	if exam.PublishMode != "immediate_score" || exam.ScorePublishTime != nil {
		t.Fatalf("forged publish config should not update exam, got %#v", exam)
	}
}

func openExamAPITestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	gormDB, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared&_foreign_keys=on"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	migrationDir := filepath.Join("..", "..", "data", "migrations", "sqlite")
	if err := migration.Run(gormDB, migrationDir); err != nil {
		t.Fatalf("run migration: %v", err)
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
		INSERT OR IGNORE INTO tenants (
			id, name, logo_url, description, tenant_code, allow_register, status,
			created_at, updated_at, version, ext_json
		) VALUES (10, '青藤一中', '', '考试业务测试租户', 'PM-QT01', true, 'enabled', ?, ?, 1, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed exam business tenant: %v", err)
	}

	if err := gormDB.Exec(`
		INSERT INTO users (
			id, username, real_name, phone, email, password_hash, status,
			created_at, updated_at, ext_json
		) VALUES (501, 'teacher.exam', '考试教师', '13800000501', 'teacher.exam@example.test', ?, 'enabled', ?, ?, '{}')
	`, passwordHash, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed exam business user: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO tenant_user_memberships (
			id, tenant_id, user_id, role, status, created_at, updated_at, ext_json
		) VALUES (501, 10, 501, 'teacher', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed exam business role: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO spaces (
			id, tenant_id, name, type, status, created_at, updated_at, ext_json
		) VALUES (300, 10, '考试业务默认空间', 'class', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed exam business space: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO space_members (
			id, tenant_id, space_id, user_id, role_in_space, status, created_at, updated_at, ext_json
		) VALUES (300, 10, 300, 501, 'teacher', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed exam business space member: %v", err)
	}
}

func seedStudentSpaceAdminExamBusinessTestData(t *testing.T, gormDB *gorm.DB) {
	t.Helper()
	passwordHash, err := crypto.HashPassword("papermind123")
	if err != nil {
		t.Fatalf("hash student space admin password: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO users (
			id, username, real_name, phone, email, password_hash, status,
			created_at, updated_at, ext_json
		) VALUES (502, 'student.space.admin', '学生空间管理员', '13800000502', 'student.space.admin@example.test', ?, 'enabled', ?, ?, '{}')
	`, passwordHash, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed student space admin user: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO tenant_user_memberships (
			id, tenant_id, user_id, role, status, created_at, updated_at, ext_json
		) VALUES (502, 10, 502, 'student', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed student space admin tenant role: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO space_members (
			id, tenant_id, space_id, user_id, role_in_space, status, created_at, updated_at, ext_json
		) VALUES (502, 10, 301, 502, 'space_admin', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed student space admin member: %v", err)
	}
}

func seedTakingAPITestData(t *testing.T, gormDB *gorm.DB) {
	t.Helper()
	passwordHash, err := crypto.HashPassword("papermind123")
	if err != nil {
		t.Fatalf("hash tenant password: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT OR IGNORE INTO tenants (
			id, name, logo_url, description, tenant_code, allow_register, status,
			created_at, updated_at, version, ext_json
		) VALUES (10, '青藤一中', '', '考试接口测试租户', 'PM-QT01', true, 'enabled', ?, ?, 1, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed taking tenant: %v", err)
	}

	if err := gormDB.Exec(`
		INSERT INTO users (
			id, username, real_name, phone, email, password_hash, status,
			created_at, updated_at, ext_json
		) VALUES
			(20, 'student20', '目标考生', '13800000020', 'student20@example.test', ?, 'enabled', ?, ?, '{}'),
			(21, 'student21', '非目标考生', '13800000021', 'student21@example.test', ?, 'enabled', ?, ?, '{}')
	`, passwordHash, fixedAPINow, fixedAPINow, passwordHash, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed taking users: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO tenant_user_memberships (
			id, tenant_id, user_id, role, status, created_at, updated_at, ext_json
		) VALUES
			(20, 10, 20, 'student', 'enabled', ?, ?, '{}'),
			(21, 10, 21, 'student', 'enabled', ?, ?, '{}')
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
		INSERT OR IGNORE INTO spaces (
			id, tenant_id, name, type, status, created_at, updated_at, ext_json
		) VALUES (100, 10, '考试入口空间', 'class', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed taking space: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT OR IGNORE INTO space_members (
			id, tenant_id, space_id, user_id, role_in_space, status, created_at, updated_at, ext_json
		) VALUES
			(120, 10, 100, 20, 'student', 'enabled', ?, ?, '{}'),
			(121, 10, 100, 21, 'student', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed taking space members: %v", err)
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
		INSERT OR IGNORE INTO tenants (
			id, name, logo_url, description, tenant_code, allow_register, status,
			created_at, updated_at, version, ext_json
		) VALUES (10, '青藤一中', '', '阅卷结果测试租户', 'PM-QT01', true, 'enabled', ?, ?, 1, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed review tenant: %v", err)
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
			id, username, real_name, phone, email, password_hash, status,
			created_at, updated_at, ext_json
		) VALUES
			(501, 'teacher.li', '李老师', '13800000501', 'teacher501@example.test', ?, 'enabled', ?, ?, '{}'),
			(601, 'student.zhang', '张三', '13800000601', 'student601@example.test', 'hash', 'enabled', ?, ?, '{}')
	`, passwordHash, fixedAPINow, fixedAPINow, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed users: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO tenant_user_memberships (
			id, tenant_id, user_id, role, status, created_at, updated_at, ext_json
		) VALUES
			(501, 10, 501, 'teacher', 'enabled', ?, ?, '{}'),
			(601, 10, 601, 'student', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow, fixedAPINow, fixedAPINow).Error; err != nil {
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

func seedOtherSpaceExamPublishAPITestData(t *testing.T, gormDB *gorm.DB) {
	t.Helper()

	if err := gormDB.Exec(`
		INSERT INTO spaces (
			id, tenant_id, name, type, status, created_at, updated_at, ext_json
		) VALUES (302, 10, '高一 2 班', 'class', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed other publish space: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO papers (
			id, tenant_id, space_id, name, description, total_score, build_mode, status,
			created_at, updated_at, ext_json
		) VALUES (201, 10, 302, '其他空间试卷', '', 0, ?, 'draft', ?, ?, '{}')
	`, constant.BuildModeManual, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed other-space publish paper: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO users (
			id, username, real_name, phone, email, password_hash, status,
			created_at, updated_at, ext_json
		) VALUES (602, 'student.other.publish', '其他空间学生', '13800000602', 'student602.publish@example.test', 'hash', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed other-space publish student: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO tenant_user_memberships (
			id, tenant_id, user_id, role, status, created_at, updated_at, ext_json
		) VALUES (602, 10, 602, 'student', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed other-space publish student membership: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO space_members (
			id, tenant_id, space_id, user_id, role_in_space, status, created_at, updated_at, ext_json
		) VALUES (302, 10, 302, 602, 'student', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed other-space publish student member: %v", err)
	}
}

func seedOtherSpaceReviewResultAPITestData(t *testing.T, gormDB *gorm.DB) {
	t.Helper()

	if err := gormDB.Exec(`
		INSERT INTO spaces (
			id, tenant_id, name, type, status, created_at, updated_at, ext_json
		) VALUES (302, 10, '高一 2 班', 'class', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed other space: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO users (
			id, username, real_name, phone, email, password_hash, status,
			created_at, updated_at, ext_json
		) VALUES (602, 'student.li', '李四', '13800000602', 'student602@example.test', 'hash', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed other-space student: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO tenant_user_memberships (
			id, tenant_id, user_id, role, status, created_at, updated_at, ext_json
		) VALUES (602, 10, 602, 'student', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed other-space student membership: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO space_members (
			id, tenant_id, space_id, user_id, role_in_space, status, created_at, updated_at, ext_json
		) VALUES (703, 10, 302, 602, 'student', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed other-space member: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO exam_targets (
			id, tenant_id, exam_id, target_type, target_id, created_at, ext_json
		) VALUES (802, 10, 1, 'space', 302, ?, '{}')
	`, fixedAPINow).Error; err != nil {
		t.Fatalf("seed other-space exam target: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO exam_attempts (
			id, tenant_id, exam_id, user_id, attempt_no, status, started_at, submitted_at,
			exam_token_hash, exam_token_expires_at, objective_score, subjective_score, total_score,
			created_at, updated_at, version, ext_json
		) VALUES (910, 10, 1, 602, 1, 'submitted', ?, ?, 'hash-other', ?, 3, 0, 3, ?, ?, 5, '{}')
	`, fixedAPINow-3_600_000, fixedAPINow, fixedAPINow+3_600_000, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed other-space attempt: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO exam_attempt_questions (
			id, tenant_id, attempt_id, section_id, question_id, section_snapshot, sort_order, score,
			question_snapshot, option_snapshot, correct_answer_snapshot, created_at, updated_at, version, ext_json
		) VALUES (
			911, 10, 910, 1, 1001, '{"name":"四、简答题","instructions":"结合文本作答"}', 1, 10,
			'{"title":"空间隔离题","type":"short_text"}', '[]', '{"text":"空间隔离"}', ?, ?, 1, '{}'
		)
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed other-space attempt question: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO exam_answers (
			id, tenant_id, attempt_id, attempt_question_id, answer_content, score, grading_status,
			created_at, updated_at, version, ext_json
		) VALUES (912, 10, 910, 911, '这是另一个空间的答案。', 0, 'pending', ?, ?, 11, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed other-space answer: %v", err)
	}
}

func seedOtherSpaceOnlyExamForPublishConfigTest(t *testing.T, gormDB *gorm.DB) {
	t.Helper()
	if err := gormDB.Exec(`
		INSERT INTO exams (
			id, tenant_id, paper_id, name, start_time, end_time, duration_minutes,
			max_attempts, result_strategy, publish_mode, invite_code, status,
			created_at, updated_at, ext_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, 1, 'latest', 'immediate_score', ?, 'published', ?, ?, '{}')
	`, 2, 10, 100, "高一 2 班单独考试", fixedAPINow, fixedAPINow+7_200_000, 120, "PM3020", fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed other-space only exam: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO exam_targets (
			id, tenant_id, exam_id, target_type, target_id, created_at, ext_json
		) VALUES (803, 10, 2, 'space', 302, ?, '{}')
	`, fixedAPINow).Error; err != nil {
		t.Fatalf("seed other-space only exam target: %v", err)
	}
}

func seedVisibleResultAPITestData(t *testing.T, gormDB *gorm.DB) {
	t.Helper()
	publishTime := fixedAPINow - 60_000
	if err := gormDB.Exec(`
		UPDATE papers
		SET show_analysis = TRUE
		WHERE tenant_id = 10 AND id = 100
	`).Error; err != nil {
		t.Fatalf("enable paper analysis: %v", err)
	}
	if err := gormDB.Exec(`
		UPDATE exams
		SET score_publish_time = ?
		WHERE tenant_id = 10 AND id = 1
	`, publishTime).Error; err != nil {
		t.Fatalf("publish exam score: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO exam_attempts (
			id, tenant_id, exam_id, user_id, attempt_no, status, started_at, submitted_at,
			exam_token_hash, exam_token_expires_at, objective_score, subjective_score, total_score,
			created_at, updated_at, version, ext_json
		) VALUES (700, 10, 1, 20, 1, 'submitted', ?, ?, 'hash-result', ?, 6, 2, 8, ?, ?, 1, '{}')
	`, fixedAPINow-3_600_000, fixedAPINow-60_000, fixedAPINow+3_600_000, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed visible result attempt: %v", err)
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

func startExamEntryAttemptForTest(t *testing.T, router *gin.Engine, authHeader string) startAttemptResponse {
	t.Helper()

	resolveRecorder := httptest.NewRecorder()
	router.ServeHTTP(resolveRecorder, authorizedRequest(http.MethodPost, "/api/v1/exam-entry/invite/resolve", []byte(`{
		"invite_code": "PM2026"
	}`), authHeader))
	if resolveRecorder.Code != http.StatusOK {
		t.Fatalf("resolve invite status = %d, body = %s", resolveRecorder.Code, resolveRecorder.Body.String())
	}

	startRecorder := httptest.NewRecorder()
	startRequest := httptest.NewRequest(http.MethodPost, "/api/v1/exam-entry/exams/1/attempts/start", bytes.NewReader([]byte(`{
		"tenant_id": 10
	}`)))
	for _, cookie := range resolveRecorder.Result().Cookies() {
		startRequest.AddCookie(cookie)
	}
	router.ServeHTTP(startRecorder, startRequest)
	if startRecorder.Code != http.StatusOK {
		t.Fatalf("start status = %d, body = %s", startRecorder.Code, startRecorder.Body.String())
	}
	return decodeExamAPIResponse[startAttemptResponse](t, startRecorder.Body.Bytes()).Data
}

type fixedCodeGenerator struct {
	code string
}

func (g fixedCodeGenerator) NextCode() (string, error) {
	return g.code, nil
}

func tenantAuthHeader(t *testing.T, router *gin.Engine, tenantID uint64, username string, password string) string {
	t.Helper()

	authHeader := tenantLoginAuthHeader(t, router, username, password)
	token := strings.TrimPrefix(authHeader, "Bearer ")
	profileRecorder := httptest.NewRecorder()
	router.ServeHTTP(profileRecorder, authorizedRequest(http.MethodGet, "/api/v1/profile/spaces", nil, authHeader))
	if profileRecorder.Code != http.StatusOK {
		t.Fatalf("profile spaces status = %d, body = %s", profileRecorder.Code, profileRecorder.Body.String())
	}
	spacesBody := decodeExamAPIResponse[profileSpaceListResponse](t, profileRecorder.Body.Bytes())
	for _, item := range spacesBody.Data.Items {
		if item.TenantID != tenantID {
			continue
		}
		selectRecorder := httptest.NewRecorder()
		selectBody := fmt.Sprintf(`{"tenant_id":%d,"space_id":%d}`, tenantID, item.SpaceID)
		router.ServeHTTP(selectRecorder, authorizedRequest(
			http.MethodPost,
			"/api/v1/auth/tenant/select-space",
			[]byte(selectBody),
			authHeader,
		))
		if selectRecorder.Code != http.StatusOK {
			t.Fatalf("select tenant space status = %d, body = %s", selectRecorder.Code, selectRecorder.Body.String())
		}
		return authHeaderFromCookies(t, selectRecorder)
	}
	selectRecorder := httptest.NewRecorder()
	selectBody := fmt.Sprintf(`{"tenant_id":%d}`, tenantID)
	router.ServeHTTP(selectRecorder, authorizedRequest(
		http.MethodPost,
		"/api/v1/auth/tenant/select-space",
		[]byte(selectBody),
		"Bearer "+token,
	))
	if selectRecorder.Code != http.StatusOK {
		t.Fatalf("tenant %d has no selectable space for %s and tenant-only select failed: status = %d, body = %s", tenantID, username, selectRecorder.Code, selectRecorder.Body.String())
	}
	return authHeaderFromCookies(t, selectRecorder)
}

func tenantLoginAuthHeader(t *testing.T, router *gin.Engine, username string, password string) string {
	t.Helper()

	loginRecorder := httptest.NewRecorder()
	body := fmt.Sprintf(`{"username":%q,"password":%q}`, username, password)
	router.ServeHTTP(loginRecorder, httptest.NewRequest(
		http.MethodPost,
		"/api/v1/auth/tenant/login",
		bytes.NewReader([]byte(body)),
	))
	if loginRecorder.Code != http.StatusOK {
		t.Fatalf("tenant login status = %d, body = %s", loginRecorder.Code, loginRecorder.Body.String())
	}
	return authHeaderFromCookies(t, loginRecorder)
}

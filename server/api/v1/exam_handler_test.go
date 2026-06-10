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

func TestPublishExamRequestAllowsUnlimitedMaxAttempts(t *testing.T) {
	request := publishExamRequest{
		TenantID:        10,
		PaperID:         100,
		Name:            "不限次数考试",
		TargetType:      serviceexam.TargetTypeSpace,
		TargetID:        301,
		StartTime:       fixedAPINow,
		EndTime:         fixedAPINow + 120*60*1000,
		DurationMinutes: 120,
		MaxAttempts:     0,
		ResultStrategy:  serviceexam.ResultStrategyLatest,
		PublishMode:     serviceexam.PublishModeManualPublish,
		Status:          serviceexam.StatusDraft,
	}
	if err := request.validate(); err != nil {
		t.Fatalf("expected unlimited max attempts to pass validation, got %v", err)
	}

	request.MaxAttempts = -1
	if err := request.validate(); err == nil {
		t.Fatal("expected negative max attempts to fail validation")
	}
}

func TestExamAPIRoutesListAndPublishWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedSpaceAPITestData(t, gormDB)
	seedExamDetailTarget(t, gormDB, 1, serviceexam.TargetTypeSpace, 100)

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
	if listBody.Data.Items[0].TargetType != "space" || listBody.Data.Items[0].TargetID != 100 {
		t.Fatalf("expected seeded exam list to keep target metadata, got %#v", listBody.Data.Items[0])
	}
	if len(listBody.Data.Items[0].Targets) != 1 ||
		listBody.Data.Items[0].Targets[0].TargetType != "space" ||
		listBody.Data.Items[0].Targets[0].TargetID != 100 {
		t.Fatalf("expected seeded exam list to keep full targets, got %#v", listBody.Data.Items[0].Targets)
	}

	filterRecorder := httptest.NewRecorder()
	router.ServeHTTP(filterRecorder, authorizedRequest(http.MethodGet, "/api/v1/exams?tenant_id=10&paper_id=999", nil, authHeader))
	if filterRecorder.Code != http.StatusOK {
		t.Fatalf("filtered list status = %d, body = %s", filterRecorder.Code, filterRecorder.Body.String())
	}
	filterBody := decodeExamAPIResponse[examListResponse](t, filterRecorder.Body.Bytes())
	if filterBody.Data.Total != 0 || len(filterBody.Data.Items) != 0 {
		t.Fatalf("expected paper_id filter to remove seeded exam, got %#v", filterBody.Data)
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

func TestExamAPIStatusLifecycleWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedSpaceAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:            gormDB,
		Now:           func() int64 { return fixedAPINow },
		CodeGenerator: fixedCodeGenerator{code: "PMSTAT"},
	})
	authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

	createRecorder := httptest.NewRecorder()
	router.ServeHTTP(createRecorder, authorizedRequest(http.MethodPost, "/api/v1/exams", []byte(`{
		"tenant_id": 10,
		"paper_id": 100,
		"name": "草稿考试",
		"target_type": "space",
		"target_id": 100,
		"start_time": 1772269200000,
		"end_time": 1772276400000,
		"duration_minutes": 120,
		"max_attempts": 1,
		"result_strategy": "latest",
		"publish_mode": "manual_publish",
		"status": "draft"
	}`), authHeader))
	if createRecorder.Code != http.StatusOK {
		t.Fatalf("create draft status = %d, body = %s", createRecorder.Code, createRecorder.Body.String())
	}
	createBody := decodeExamAPIResponse[examResponse](t, createRecorder.Body.Bytes())
	if createBody.Data.Status != serviceexam.StatusDraft || createBody.Data.InviteCode != "" {
		t.Fatalf("expected draft without invite code, got %#v", createBody.Data)
	}

	secondCreateRecorder := httptest.NewRecorder()
	router.ServeHTTP(secondCreateRecorder, authorizedRequest(http.MethodPost, "/api/v1/exams", []byte(`{
		"tenant_id": 10,
		"paper_id": 100,
		"name": "第二个草稿考试",
		"target_type": "space",
		"target_id": 100,
		"start_time": 1772269200000,
		"end_time": 1772276400000,
		"duration_minutes": 120,
		"max_attempts": 1,
		"result_strategy": "latest",
		"publish_mode": "manual_publish",
		"status": "draft"
	}`), authHeader))
	if secondCreateRecorder.Code != http.StatusOK {
		t.Fatalf("create second draft status = %d, body = %s", secondCreateRecorder.Code, secondCreateRecorder.Body.String())
	}
	secondCreateBody := decodeExamAPIResponse[examResponse](t, secondCreateRecorder.Body.Bytes())
	if secondCreateBody.Data.Status != serviceexam.StatusDraft || secondCreateBody.Data.InviteCode != "" {
		t.Fatalf("expected second draft without invite code, got %#v", secondCreateBody.Data)
	}

	publishRecorder := httptest.NewRecorder()
	router.ServeHTTP(publishRecorder, authorizedRequest(http.MethodPost, fmt.Sprintf("/api/v1/exams/%d/status", createBody.Data.ID), []byte(`{
		"tenant_id": 10,
		"status": "published"
	}`), authHeader))
	if publishRecorder.Code != http.StatusOK {
		t.Fatalf("publish draft status = %d, body = %s", publishRecorder.Code, publishRecorder.Body.String())
	}
	publishBody := decodeExamAPIResponse[examResponse](t, publishRecorder.Body.Bytes())
	if publishBody.Data.Status != serviceexam.StatusPublished || publishBody.Data.InviteCode != "PMSTAT" {
		t.Fatalf("expected draft to publish with invite code, got %#v", publishBody.Data)
	}

	closeRecorder := httptest.NewRecorder()
	router.ServeHTTP(closeRecorder, authorizedRequest(http.MethodPost, fmt.Sprintf("/api/v1/exams/%d/status", createBody.Data.ID), []byte(`{
		"tenant_id": 10,
		"status": "closed"
	}`), authHeader))
	if closeRecorder.Code != http.StatusOK {
		t.Fatalf("close exam status = %d, body = %s", closeRecorder.Code, closeRecorder.Body.String())
	}
	closeBody := decodeExamAPIResponse[examResponse](t, closeRecorder.Body.Bytes())
	if closeBody.Data.Status != serviceexam.StatusClosed || closeBody.Data.EndTime != fixedAPINow {
		t.Fatalf("expected closed exam with end_time now, got %#v", closeBody.Data)
	}
}

func TestExamAPIStatusUpdateAllowsTeacherInExamScopeWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedSpaceAPITestData(t, gormDB)
	seedExamDetailTarget(t, gormDB, 1, serviceexam.TargetTypeSpace, 100)
	passwordHash, err := crypto.HashPassword("papermind123")
	if err != nil {
		t.Fatalf("hash scoped teacher password: %v", err)
	}
	if err := gormDB.Table("users").Where("username = ?", "teacher_li").Update("password_hash", passwordHash).Error; err != nil {
		t.Fatalf("update scoped teacher password: %v", err)
	}

	router := NewRouter(RouterOptions{
		DB:            gormDB,
		Now:           func() int64 { return fixedAPINow },
		CodeGenerator: &sequenceCodeGenerator{codes: []string{"PMTEACH", "PMDISA"}},
	})
	authHeader := tenantAuthHeader(t, router, 10, "teacher_li", "papermind123")

	createRecorder := httptest.NewRecorder()
	router.ServeHTTP(createRecorder, authorizedRequest(http.MethodPost, "/api/v1/exams", []byte(`{
		"tenant_id": 10,
		"paper_id": 100,
		"name": "教师草稿考试",
		"target_type": "space",
		"target_id": 100,
		"start_time": 1772269200000,
		"end_time": 1772276400000,
		"duration_minutes": 120,
		"max_attempts": 1,
		"result_strategy": "latest",
		"publish_mode": "manual_publish",
		"status": "draft"
	}`), authHeader))
	if createRecorder.Code != http.StatusOK {
		t.Fatalf("teacher create draft status = %d, body = %s", createRecorder.Code, createRecorder.Body.String())
	}
	createBody := decodeExamAPIResponse[examResponse](t, createRecorder.Body.Bytes())
	if createBody.Data.Status != serviceexam.StatusDraft || createBody.Data.InviteCode != "" {
		t.Fatalf("expected teacher draft without invite code, got %#v", createBody.Data)
	}

	publishRecorder := httptest.NewRecorder()
	router.ServeHTTP(publishRecorder, authorizedRequest(http.MethodPost, fmt.Sprintf("/api/v1/exams/%d/status", createBody.Data.ID), []byte(`{
		"tenant_id": 10,
		"status": "published"
	}`), authHeader))
	if publishRecorder.Code != http.StatusOK {
		t.Fatalf("teacher publish draft status = %d, body = %s", publishRecorder.Code, publishRecorder.Body.String())
	}
	publishBody := decodeExamAPIResponse[examResponse](t, publishRecorder.Body.Bytes())
	if publishBody.Data.Status != serviceexam.StatusPublished || publishBody.Data.InviteCode != "PMTEACH" {
		t.Fatalf("expected teacher draft publish with invite code, got %#v", publishBody.Data)
	}

	closeRecorder := httptest.NewRecorder()
	router.ServeHTTP(closeRecorder, authorizedRequest(http.MethodPost, fmt.Sprintf("/api/v1/exams/%d/status", createBody.Data.ID), []byte(`{
		"tenant_id": 10,
		"status": "closed"
	}`), authHeader))
	if closeRecorder.Code != http.StatusOK {
		t.Fatalf("teacher close exam status = %d, body = %s", closeRecorder.Code, closeRecorder.Body.String())
	}
	closeBody := decodeExamAPIResponse[examResponse](t, closeRecorder.Body.Bytes())
	if closeBody.Data.Status != serviceexam.StatusClosed || closeBody.Data.EndTime != fixedAPINow {
		t.Fatalf("expected teacher closed exam with end_time now, got %#v", closeBody.Data)
	}

	disableCreateRecorder := httptest.NewRecorder()
	router.ServeHTTP(disableCreateRecorder, authorizedRequest(http.MethodPost, "/api/v1/exams", []byte(`{
		"tenant_id": 10,
		"paper_id": 100,
		"name": "教师可禁用考试",
		"target_type": "space",
		"target_id": 100,
		"start_time": 1772269200000,
		"end_time": 1772276400000,
		"duration_minutes": 120,
		"max_attempts": 1,
		"result_strategy": "latest",
		"publish_mode": "manual_publish",
		"status": "published"
	}`), authHeader))
	if disableCreateRecorder.Code != http.StatusOK {
		t.Fatalf("teacher create disable candidate status = %d, body = %s", disableCreateRecorder.Code, disableCreateRecorder.Body.String())
	}
	disableCreateBody := decodeExamAPIResponse[examResponse](t, disableCreateRecorder.Body.Bytes())

	disableRecorder := httptest.NewRecorder()
	router.ServeHTTP(disableRecorder, authorizedRequest(http.MethodPost, fmt.Sprintf("/api/v1/exams/%d/status", disableCreateBody.Data.ID), []byte(`{
		"tenant_id": 10,
		"status": "disabled"
	}`), authHeader))
	if disableRecorder.Code != http.StatusOK {
		t.Fatalf("teacher disable exam status = %d, body = %s", disableRecorder.Code, disableRecorder.Body.String())
	}
	disableBody := decodeExamAPIResponse[examResponse](t, disableRecorder.Body.Bytes())
	if disableBody.Data.Status != serviceexam.StatusDisabled {
		t.Fatalf("expected teacher disabled exam, got %#v", disableBody.Data)
	}
}

func TestExamAPIStatusUpdateRejectsTeacherWhoDidNotCreateExam(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedSpaceAPITestData(t, gormDB)
	seedExamDetailTarget(t, gormDB, 1, serviceexam.TargetTypeSpace, 100)
	passwordHash, err := crypto.HashPassword("papermind123")
	if err != nil {
		t.Fatalf("hash peer teacher password: %v", err)
	}
	for _, username := range []string{"teacher_li", "teacher_zhao"} {
		if err := gormDB.Table("users").Where("username = ?", username).Update("password_hash", passwordHash).Error; err != nil {
			t.Fatalf("update %s password: %v", username, err)
		}
	}
	if err := gormDB.Exec(`
		INSERT INTO space_members (
			id, tenant_id, space_id, user_id, role_in_space, status,
			created_at, updated_at, ext_json
		) VALUES (120, 10, 100, 22, 'teacher', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed peer teacher member: %v", err)
	}

	router := NewRouter(RouterOptions{
		DB:            gormDB,
		Now:           func() int64 { return fixedAPINow },
		CodeGenerator: fixedCodeGenerator{code: "PMOWNER"},
	})
	creatorHeader := tenantAuthHeader(t, router, 10, "teacher_li", "papermind123")
	peerHeader := tenantAuthHeader(t, router, 10, "teacher_zhao", "papermind123")

	createRecorder := httptest.NewRecorder()
	router.ServeHTTP(createRecorder, authorizedRequest(http.MethodPost, "/api/v1/exams", []byte(`{
		"tenant_id": 10,
		"paper_id": 100,
		"name": "教师自己发布考试",
		"target_type": "space",
		"target_id": 100,
		"start_time": 1772269200000,
		"end_time": 1772276400000,
		"duration_minutes": 120,
		"max_attempts": 1,
		"result_strategy": "latest",
		"publish_mode": "manual_publish",
		"status": "draft"
	}`), creatorHeader))
	if createRecorder.Code != http.StatusOK {
		t.Fatalf("creator create draft status = %d, body = %s", createRecorder.Code, createRecorder.Body.String())
	}
	createBody := decodeExamAPIResponse[examResponse](t, createRecorder.Body.Bytes())

	peerRecorder := httptest.NewRecorder()
	router.ServeHTTP(peerRecorder, authorizedRequest(http.MethodPost, fmt.Sprintf("/api/v1/exams/%d/status", createBody.Data.ID), []byte(`{
		"tenant_id": 10,
		"status": "published"
	}`), peerHeader))
	if peerRecorder.Code != http.StatusForbidden {
		t.Fatalf("peer teacher status = %d, body = %s", peerRecorder.Code, peerRecorder.Body.String())
	}
}

func TestExamAPITeacherCanUpdateCurrentSpaceDraftWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedSpaceAPITestData(t, gormDB)
	passwordHash, err := crypto.HashPassword("papermind123")
	if err != nil {
		t.Fatalf("hash draft update teacher password: %v", err)
	}
	if err := gormDB.Table("users").Where("username = ?", "teacher_li").Update("password_hash", passwordHash).Error; err != nil {
		t.Fatalf("update draft update teacher password: %v", err)
	}

	router := NewRouter(RouterOptions{
		DB:            gormDB,
		Now:           func() int64 { return fixedAPINow },
		CodeGenerator: fixedCodeGenerator{code: "PMDRAFT"},
	})
	authHeader := tenantAuthHeader(t, router, 10, "teacher_li", "papermind123")

	createRecorder := httptest.NewRecorder()
	router.ServeHTTP(createRecorder, authorizedRequest(http.MethodPost, "/api/v1/exams", []byte(`{
		"tenant_id": 10,
		"paper_id": 100,
		"name": "待调整草稿",
		"target_type": "space",
		"target_id": 100,
		"start_time": 1772269200000,
		"end_time": 1772276400000,
		"duration_minutes": 120,
		"max_attempts": 1,
		"result_strategy": "latest",
		"publish_mode": "manual_publish",
		"status": "draft"
	}`), authHeader))
	if createRecorder.Code != http.StatusOK {
		t.Fatalf("teacher create editable draft status = %d, body = %s", createRecorder.Code, createRecorder.Body.String())
	}
	createBody := decodeExamAPIResponse[examResponse](t, createRecorder.Body.Bytes())

	updateRecorder := httptest.NewRecorder()
	router.ServeHTTP(updateRecorder, authorizedRequest(http.MethodPost, fmt.Sprintf("/api/v1/exams/%d/draft", createBody.Data.ID), []byte(`{
		"tenant_id": 10,
		"paper_id": 100,
		"name": "调整后的教师草稿",
		"target_type": "space",
		"target_id": 100,
		"start_time": 1772355600000,
		"end_time": 1772362800000,
		"duration_minutes": 90,
		"max_attempts": 2,
		"result_strategy": "highest",
		"publish_mode": "immediate_score",
		"score_publish_time": null,
		"status": "draft"
	}`), authHeader))
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("teacher update draft status = %d, body = %s", updateRecorder.Code, updateRecorder.Body.String())
	}
	updateBody := decodeExamAPIResponse[examResponse](t, updateRecorder.Body.Bytes())
	if updateBody.Data.Status != serviceexam.StatusDraft || updateBody.Data.InviteCode != "" {
		t.Fatalf("expected updated draft without invite code, got %#v", updateBody.Data)
	}
	if updateBody.Data.Name != "调整后的教师草稿" || updateBody.Data.DurationMinutes != 90 || updateBody.Data.MaxAttempts != 2 ||
		updateBody.Data.ResultStrategy != serviceexam.ResultStrategyHighest || updateBody.Data.PublishMode != serviceexam.PublishModeImmediateScore {
		t.Fatalf("expected draft settings updated, got %#v", updateBody.Data)
	}

	publishRecorder := httptest.NewRecorder()
	router.ServeHTTP(publishRecorder, authorizedRequest(http.MethodPost, fmt.Sprintf("/api/v1/exams/%d/status", createBody.Data.ID), []byte(`{
		"tenant_id": 10,
		"status": "published"
	}`), authHeader))
	if publishRecorder.Code != http.StatusOK {
		t.Fatalf("publish updated draft status = %d, body = %s", publishRecorder.Code, publishRecorder.Body.String())
	}
	publishBody := decodeExamAPIResponse[examResponse](t, publishRecorder.Body.Bytes())
	if publishBody.Data.Name != "调整后的教师草稿" || publishBody.Data.Status != serviceexam.StatusPublished || publishBody.Data.InviteCode != "PMDRAFT" {
		t.Fatalf("expected updated draft to publish, got %#v", publishBody.Data)
	}
}

func TestExamAPIStatusUpdateRejectsTeacherOutsideExamScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedSpaceAPITestData(t, gormDB)
	seedExamDetailTarget(t, gormDB, 1, serviceexam.TargetTypeSpace, 100)
	passwordHash, err := crypto.HashPassword("papermind123")
	if err != nil {
		t.Fatalf("hash teacher password: %v", err)
	}
	if err := gormDB.Table("users").
		Where("username = ?", "teacher_zhao").
		Update("password_hash", passwordHash).Error; err != nil {
		t.Fatalf("update teacher password: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO spaces (
			id, tenant_id, name, logo_url, description, type, status,
			created_at, updated_at, ext_json
		) VALUES (110, 10, '高一 2 班', '', '', 'class', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed other teacher space: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO space_members (
			id, tenant_id, space_id, user_id, role_in_space, status,
			created_at, updated_at, ext_json
		) VALUES (110, 10, 110, 22, 'teacher', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed out-of-scope teacher member: %v", err)
	}

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "teacher_zhao", "papermind123")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodPost, "/api/v1/exams/1/status", []byte(`{
		"tenant_id": 10,
		"status": "closed"
	}`), authHeader))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected out-of-scope teacher status update 403, got status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestExamAPIDetailReturnsManagementPermissionsWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedSpaceAPITestData(t, gormDB)
	seedExamDetailTarget(t, gormDB, 1, serviceexam.TargetTypeSpace, 100)

	router := NewRouter(RouterOptions{DB: gormDB, Now: func() int64 { return fixedAPINow }})
	authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodGet, "/api/v1/exams/1/detail?tenant_id=10", nil, authHeader))
	if recorder.Code != http.StatusOK {
		t.Fatalf("detail status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	body := decodeExamAPIResponse[examManagementDetailResponse](t, recorder.Body.Bytes())
	if body.Data.Exam.ID != 1 || body.Data.Exam.Name != "高一语文期中考试" {
		t.Fatalf("unexpected detail exam: %#v", body.Data.Exam)
	}
	if len(body.Data.Targets) != 1 || body.Data.Targets[0].TargetType != serviceexam.TargetTypeSpace || body.Data.Targets[0].TargetID != 100 {
		t.Fatalf("expected original target returned, got %#v", body.Data.Targets)
	}
	if len(body.Data.AllowedSpaceIDs) != 1 || body.Data.AllowedSpaceIDs[0] != 100 {
		t.Fatalf("expected tenant admin allowed space 100, got %#v", body.Data.AllowedSpaceIDs)
	}
	if !body.Data.Permissions.CanViewDetail || !body.Data.Permissions.CanExportResults || !body.Data.Permissions.CanUpdateSettings {
		t.Fatalf("expected tenant admin permissions, got %#v", body.Data.Permissions)
	}

	missingRecorder := httptest.NewRecorder()
	router.ServeHTTP(missingRecorder, authorizedRequest(http.MethodGet, "/api/v1/exams/999/detail?tenant_id=10", nil, authHeader))
	if missingRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected missing exam detail 404, got status = %d, body = %s", missingRecorder.Code, missingRecorder.Body.String())
	}
}

func TestExamAPIDetailRespectsRequestedSpaceScopeForTeacher(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedSpaceAPITestData(t, gormDB)
	if err := gormDB.Exec(`
		INSERT INTO spaces (
			id, tenant_id, name, logo_url, description, type, status, created_at, updated_at, ext_json
		) VALUES (110, 10, '高一 2 班', '', '', 'class', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed secondary detail space: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO space_members (
			id, tenant_id, space_id, user_id, role_in_space, status, created_at, updated_at, ext_json
		) VALUES (110, 10, 110, 20, 'teacher', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed secondary teacher membership: %v", err)
	}
	seedExamDetailTarget(t, gormDB, 1, serviceexam.TargetTypeSpace, 100)
	seedExamDetailTarget(t, gormDB, 1, serviceexam.TargetTypeSpace, 110)
	passwordHash, err := crypto.HashPassword("papermind123")
	if err != nil {
		t.Fatalf("hash teacher password: %v", err)
	}
	if err := gormDB.Table("users").Where("username = ?", "teacher_li").Update("password_hash", passwordHash).Error; err != nil {
		t.Fatalf("update teacher password: %v", err)
	}

	router := NewRouter(RouterOptions{DB: gormDB, Now: func() int64 { return fixedAPINow }})
	authHeader := tenantAuthHeader(t, router, 10, "teacher_li", "papermind123")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodGet, "/api/v1/exams/1/detail?tenant_id=10&space_id=100", nil, authHeader))
	if recorder.Code != http.StatusOK {
		t.Fatalf("detail scoped status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	body := decodeExamAPIResponse[examManagementDetailResponse](t, recorder.Body.Bytes())
	if len(body.Data.AllowedSpaceIDs) != 1 || body.Data.AllowedSpaceIDs[0] != 100 {
		t.Fatalf("expected teacher detail to stay scoped to requested space 100, got %#v", body.Data.AllowedSpaceIDs)
	}
}

func TestExamAPIListScopesTargetsToRequestedSpaceForTeacher(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedSpaceAPITestData(t, gormDB)
	if err := gormDB.Exec(`
		INSERT INTO spaces (
			id, tenant_id, name, logo_url, description, type, status, created_at, updated_at, ext_json
		) VALUES (110, 10, '高一 2 班', '', '', 'class', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed secondary list space: %v", err)
	}
	seedExamDetailTarget(t, gormDB, 1, serviceexam.TargetTypeSpace, 100)
	seedExamDetailTarget(t, gormDB, 1, serviceexam.TargetTypeSpace, 110)
	passwordHash, err := crypto.HashPassword("papermind123")
	if err != nil {
		t.Fatalf("hash teacher password: %v", err)
	}
	if err := gormDB.Table("users").Where("username = ?", "teacher_li").Update("password_hash", passwordHash).Error; err != nil {
		t.Fatalf("update teacher password: %v", err)
	}

	router := NewRouter(RouterOptions{DB: gormDB, Now: func() int64 { return fixedAPINow }})
	authHeader := tenantAuthHeader(t, router, 10, "teacher_li", "papermind123")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodGet, "/api/v1/exams?tenant_id=10&space_id=100", nil, authHeader))
	if recorder.Code != http.StatusOK {
		t.Fatalf("scoped list status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	type scopedExamListResponse struct {
		Items []examResponse `json:"items"`
	}
	body := decodeExamAPIResponse[scopedExamListResponse](t, recorder.Body.Bytes())
	if len(body.Data.Items) != 1 {
		t.Fatalf("expected one visible exam, got %#v", body.Data.Items)
	}
	if len(body.Data.Items[0].Targets) != 1 || body.Data.Items[0].Targets[0].TargetType != serviceexam.TargetTypeSpace || body.Data.Items[0].Targets[0].TargetID != 100 {
		t.Fatalf("expected scoped exam list to hide target in space 110, got %#v", body.Data.Items[0].Targets)
	}
}

func TestExamAPIDetailRejectsStudentManagementAccess(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedSpaceAPITestData(t, gormDB)
	seedExamDetailTarget(t, gormDB, 1, serviceexam.TargetTypeSpace, 100)
	passwordHash, err := crypto.HashPassword("papermind123")
	if err != nil {
		t.Fatalf("hash student password: %v", err)
	}
	if err := gormDB.Table("users").Where("id = ?", 21).Update("password_hash", passwordHash).Error; err != nil {
		t.Fatalf("update student password: %v", err)
	}

	router := NewRouter(RouterOptions{DB: gormDB, Now: func() int64 { return fixedAPINow }})
	authHeader := tenantAuthHeader(t, router, 10, "student_zhang", "papermind123")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodGet, "/api/v1/exams/1/detail?tenant_id=10", nil, authHeader))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected student detail forbidden, got status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestExamAPIOverviewAndPaperPreviewWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedSpaceAPITestData(t, gormDB)
	seedExamDetailTarget(t, gormDB, 1, serviceexam.TargetTypeSpace, 100)
	seedExamOverviewPreviewData(t, gormDB)

	router := NewRouter(RouterOptions{DB: gormDB, Now: func() int64 { return fixedAPINow }})
	authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

	overviewRecorder := httptest.NewRecorder()
	router.ServeHTTP(overviewRecorder, authorizedRequest(http.MethodGet, "/api/v1/exams/1/overview?tenant_id=10", nil, authHeader))
	if overviewRecorder.Code != http.StatusOK {
		t.Fatalf("overview status = %d, body = %s", overviewRecorder.Code, overviewRecorder.Body.String())
	}
	overviewBody := decodeExamAPIResponse[examOverviewResponse](t, overviewRecorder.Body.Bytes())
	if overviewBody.Data.CandidateStats.Planned != 1 || overviewBody.Data.CandidateStats.Submitted != 1 || overviewBody.Data.CandidateStats.InProgress != 0 {
		t.Fatalf("unexpected overview candidate stats: %#v", overviewBody.Data.CandidateStats)
	}
	if len(overviewBody.Data.QuestionTypes) != 1 || overviewBody.Data.QuestionTypes[0].QuestionType != constant.QuestionTypeSingle || overviewBody.Data.QuestionTypes[0].QuestionCount != 1 {
		t.Fatalf("unexpected overview question distribution: %#v", overviewBody.Data.QuestionTypes)
	}

	previewRecorder := httptest.NewRecorder()
	router.ServeHTTP(previewRecorder, authorizedRequest(http.MethodGet, "/api/v1/exams/1/paper-preview?tenant_id=10&type=single&page=1&page_size=1", nil, authHeader))
	if previewRecorder.Code != http.StatusOK {
		t.Fatalf("paper preview status = %d, body = %s", previewRecorder.Code, previewRecorder.Body.String())
	}
	previewBody := decodeExamAPIResponse[examPaperPreviewResponse](t, previewRecorder.Body.Bytes())
	if previewBody.Data.Total != 1 || len(previewBody.Data.Items) != 1 || previewBody.Data.Items[0].Title != "服务端预览题干" {
		t.Fatalf("unexpected paper preview response: %#v", previewBody.Data)
	}
	if strings.Contains(previewRecorder.Body.String(), "correct") || strings.Contains(previewRecorder.Body.String(), "standard_answer") {
		t.Fatalf("paper preview must not expose correct answers, body = %s", previewRecorder.Body.String())
	}

	candidatesRecorder := httptest.NewRecorder()
	router.ServeHTTP(candidatesRecorder, authorizedRequest(http.MethodGet, "/api/v1/exams/1/candidates?tenant_id=10&page=1&page_size=20", nil, authHeader))
	if candidatesRecorder.Code != http.StatusOK {
		t.Fatalf("candidates status = %d, body = %s", candidatesRecorder.Code, candidatesRecorder.Body.String())
	}
	candidatesBody := decodeExamAPIResponse[examCandidateListResponse](t, candidatesRecorder.Body.Bytes())
	if candidatesBody.Data.Total != 1 || len(candidatesBody.Data.Items) != 1 {
		t.Fatalf("unexpected candidates response: %#v", candidatesBody.Data)
	}
	candidate := candidatesBody.Data.Items[0]
	if candidate.UserID != 21 || candidate.Status != serviceexam.CandidateStatusSubmitted || candidate.AttemptCount != 1 {
		t.Fatalf("unexpected candidate row: %#v", candidate)
	}
	if candidate.ResultAttemptID == nil || *candidate.ResultAttemptID != 1801 {
		t.Fatalf("expected result attempt 1801, got %#v", candidate.ResultAttemptID)
	}
}

func TestExamAPIManagementResultsWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedSpaceAPITestData(t, gormDB)
	seedExamDetailTarget(t, gormDB, 1, serviceexam.TargetTypeSpace, 100)
	seedExamOverviewPreviewData(t, gormDB)
	seedExamManagementLowScoreResultData(t, gormDB)

	router := NewRouter(RouterOptions{DB: gormDB, Now: func() int64 { return fixedAPINow }})
	authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

	type resultSummaryResponse struct {
		Stats struct {
			Submitted         int    `json:"submitted"`
			AverageScore      string `json:"average_score"`
			HighestScore      string `json:"highest_score"`
			PassRate          string `json:"pass_rate"`
			PendingSubjective int    `json:"pending_subjective"`
		} `json:"stats"`
		ScoreDistribution []struct {
			Label string `json:"label"`
			Count int    `json:"count"`
		} `json:"score_distribution"`
		QuestionTypeRates []struct {
			QuestionType      string `json:"question_type"`
			QuestionTypeLabel string `json:"question_type_label"`
			AverageRate       int    `json:"average_rate"`
		} `json:"question_type_rates"`
	}
	type managementResultListResponse struct {
		Items []struct {
			Rank            uint64 `json:"rank"`
			AttemptID       uint64 `json:"attempt_id"`
			UserID          uint64 `json:"user_id"`
			Username        string `json:"username"`
			RealName        string `json:"real_name"`
			SpaceID         uint64 `json:"space_id"`
			SpaceName       string `json:"space_name"`
			ObjectiveScore  string `json:"objective_score"`
			SubjectiveScore string `json:"subjective_score"`
			TotalScore      string `json:"total_score"`
			Status          string `json:"status"`
			SubmittedAt     int64  `json:"submitted_at"`
		} `json:"items"`
		Page     int   `json:"page"`
		PageSize int   `json:"page_size"`
		Total    int64 `json:"total"`
	}

	summaryRecorder := httptest.NewRecorder()
	router.ServeHTTP(summaryRecorder, authorizedRequest(http.MethodGet, "/api/v1/exams/1/results/summary?tenant_id=10", nil, authHeader))
	if summaryRecorder.Code != http.StatusOK {
		t.Fatalf("results summary status = %d, body = %s", summaryRecorder.Code, summaryRecorder.Body.String())
	}
	summaryBody := decodeExamAPIResponse[resultSummaryResponse](t, summaryRecorder.Body.Bytes())
	if summaryBody.Data.Stats.Submitted != 2 || summaryBody.Data.Stats.AverageScore != "1" || summaryBody.Data.Stats.HighestScore != "2" || summaryBody.Data.Stats.PassRate != "50%" {
		t.Fatalf("unexpected result summary stats: %#v", summaryBody.Data.Stats)
	}
	if len(summaryBody.Data.ScoreDistribution) != 2 || summaryBody.Data.ScoreDistribution[0].Label != "0" || summaryBody.Data.ScoreDistribution[0].Count != 1 || summaryBody.Data.ScoreDistribution[1].Label != "2" || summaryBody.Data.ScoreDistribution[1].Count != 1 {
		t.Fatalf("unexpected result score distribution: %#v", summaryBody.Data.ScoreDistribution)
	}
	if len(summaryBody.Data.QuestionTypeRates) != 1 || summaryBody.Data.QuestionTypeRates[0].QuestionType != constant.QuestionTypeSingle || summaryBody.Data.QuestionTypeRates[0].AverageRate != 50 {
		t.Fatalf("unexpected result question type rates: %#v", summaryBody.Data.QuestionTypeRates)
	}

	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, authorizedRequest(http.MethodGet, "/api/v1/exams/1/results?tenant_id=10&page=1&page_size=20", nil, authHeader))
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("management results status = %d, body = %s", listRecorder.Code, listRecorder.Body.String())
	}
	listBody := decodeExamAPIResponse[managementResultListResponse](t, listRecorder.Body.Bytes())
	if listBody.Data.Total != 2 || len(listBody.Data.Items) != 2 {
		t.Fatalf("unexpected management results response: %#v", listBody.Data)
	}
	result := listBody.Data.Items[0]
	if result.Rank != 1 || result.AttemptID != 1801 || result.UserID != 21 || result.TotalScore != "2" || result.Status != "pending_publish" {
		t.Fatalf("unexpected management result row: %#v", result)
	}

	pendingPublishRecorder := httptest.NewRecorder()
	router.ServeHTTP(pendingPublishRecorder, authorizedRequest(http.MethodGet, "/api/v1/exams/1/results?tenant_id=10&status=pending_publish&page=1&page_size=20", nil, authHeader))
	if pendingPublishRecorder.Code != http.StatusOK {
		t.Fatalf("pending publish results status = %d, body = %s", pendingPublishRecorder.Code, pendingPublishRecorder.Body.String())
	}
	pendingPublishBody := decodeExamAPIResponse[managementResultListResponse](t, pendingPublishRecorder.Body.Bytes())
	if pendingPublishBody.Data.Total != 2 || len(pendingPublishBody.Data.Items) != 2 || pendingPublishBody.Data.Items[0].Status != "pending_publish" || pendingPublishBody.Data.Items[1].Status != "pending_publish" {
		t.Fatalf("expected pending publish filter to return manual-publish result, got %#v", pendingPublishBody.Data)
	}

	publishedRecorder := httptest.NewRecorder()
	router.ServeHTTP(publishedRecorder, authorizedRequest(http.MethodGet, "/api/v1/exams/1/results?tenant_id=10&status=published&page=1&page_size=20", nil, authHeader))
	if publishedRecorder.Code != http.StatusOK {
		t.Fatalf("published results status = %d, body = %s", publishedRecorder.Code, publishedRecorder.Body.String())
	}
	publishedBody := decodeExamAPIResponse[managementResultListResponse](t, publishedRecorder.Body.Bytes())
	if publishedBody.Data.Total != 0 || len(publishedBody.Data.Items) != 0 {
		t.Fatalf("expected manual-publish exam to hide results from published filter, got %#v", publishedBody.Data)
	}

	seedExamManagementResultRankingData(t, gormDB)
	secondPageRecorder := httptest.NewRecorder()
	router.ServeHTTP(secondPageRecorder, authorizedRequest(http.MethodGet, "/api/v1/exams/1/results?tenant_id=10&page=2&page_size=1", nil, authHeader))
	if secondPageRecorder.Code != http.StatusOK {
		t.Fatalf("management results second page status = %d, body = %s", secondPageRecorder.Code, secondPageRecorder.Body.String())
	}
	secondPageBody := decodeExamAPIResponse[managementResultListResponse](t, secondPageRecorder.Body.Bytes())
	if secondPageBody.Data.Total != 3 || len(secondPageBody.Data.Items) != 1 {
		t.Fatalf("unexpected second page management results response: %#v", secondPageBody.Data)
	}
	if secondPageBody.Data.Items[0].Rank != 2 || secondPageBody.Data.Items[0].AttemptID != 1801 {
		t.Fatalf("expected second page to keep global rank 2 for attempt 1801, got %#v", secondPageBody.Data.Items[0])
	}
}

func TestExamAPIManagementResultsEmptyWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedSpaceAPITestData(t, gormDB)
	seedExamDetailTarget(t, gormDB, 1, serviceexam.TargetTypeSpace, 100)

	router := NewRouter(RouterOptions{DB: gormDB, Now: func() int64 { return fixedAPINow }})
	authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

	type emptyResultSummaryResponse struct {
		Stats struct {
			Submitted int `json:"submitted"`
		} `json:"stats"`
		ScoreDistribution []struct {
			Label string `json:"label"`
			Count int    `json:"count"`
		} `json:"score_distribution"`
		QuestionTypeRates []struct {
			QuestionType string `json:"question_type"`
		} `json:"question_type_rates"`
	}
	type emptyManagementResultListResponse struct {
		Items []struct {
			AttemptID uint64 `json:"attempt_id"`
		} `json:"items"`
		Total int64 `json:"total"`
	}

	summaryRecorder := httptest.NewRecorder()
	router.ServeHTTP(summaryRecorder, authorizedRequest(http.MethodGet, "/api/v1/exams/1/results/summary?tenant_id=10", nil, authHeader))
	if summaryRecorder.Code != http.StatusOK {
		t.Fatalf("empty results summary status = %d, body = %s", summaryRecorder.Code, summaryRecorder.Body.String())
	}
	summaryBody := decodeExamAPIResponse[emptyResultSummaryResponse](t, summaryRecorder.Body.Bytes())
	if summaryBody.Data.Stats.Submitted != 0 || len(summaryBody.Data.ScoreDistribution) != 0 || len(summaryBody.Data.QuestionTypeRates) != 0 {
		t.Fatalf("expected empty result summary, got %#v", summaryBody.Data)
	}

	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, authorizedRequest(http.MethodGet, "/api/v1/exams/1/results?tenant_id=10&page=1&page_size=20", nil, authHeader))
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("empty management results status = %d, body = %s", listRecorder.Code, listRecorder.Body.String())
	}
	listBody := decodeExamAPIResponse[emptyManagementResultListResponse](t, listRecorder.Body.Bytes())
	if listBody.Data.Total != 0 || len(listBody.Data.Items) != 0 {
		t.Fatalf("expected empty management results, got %#v", listBody.Data)
	}
}

func TestExamAPIManagementAnswerSheetWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedSpaceAPITestData(t, gormDB)
	seedExamDetailTarget(t, gormDB, 1, serviceexam.TargetTypeSpace, 100)
	seedExamOverviewPreviewData(t, gormDB)

	router := NewRouter(RouterOptions{DB: gormDB, Now: func() int64 { return fixedAPINow }})
	authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

	type answerSheetResponse struct {
		Attempt struct {
			AttemptID      uint64 `json:"attempt_id"`
			ExamID         uint64 `json:"exam_id"`
			UserID         uint64 `json:"user_id"`
			Username       string `json:"username"`
			RealName       string `json:"real_name"`
			TotalScore     string `json:"total_score"`
			ObjectiveScore string `json:"objective_score"`
			SubmittedAt    int64  `json:"submitted_at"`
		} `json:"attempt"`
		Items []struct {
			AttemptQuestionID     uint64 `json:"attempt_question_id"`
			QuestionType          string `json:"question_type"`
			QuestionTitle         string `json:"question_title"`
			Score                 string `json:"score"`
			AnswerContent         string `json:"answer_content"`
			AnswerScore           string `json:"answer_score"`
			GradingStatus         string `json:"grading_status"`
			CorrectAnswerSnapshot string `json:"correct_answer_snapshot"`
		} `json:"items"`
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodGet, "/api/v1/exams/1/attempts/1801/answer-sheet?tenant_id=10", nil, authHeader))
	if recorder.Code != http.StatusOK {
		t.Fatalf("answer sheet status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	body := decodeExamAPIResponse[answerSheetResponse](t, recorder.Body.Bytes())
	if body.Data.Attempt.AttemptID != 1801 || body.Data.Attempt.ExamID != 1 || body.Data.Attempt.UserID != 21 || body.Data.Attempt.TotalScore != "2" {
		t.Fatalf("unexpected answer sheet attempt: %#v", body.Data.Attempt)
	}
	if len(body.Data.Items) != 1 {
		t.Fatalf("expected one answer sheet item, got %#v", body.Data.Items)
	}
	item := body.Data.Items[0]
	if item.AttemptQuestionID != 1901 || item.QuestionType != constant.QuestionTypeSingle || item.QuestionTitle != "服务端预览题干" {
		t.Fatalf("unexpected answer sheet item question: %#v", item)
	}
	if item.AnswerContent != "A" || item.AnswerScore != "2" || item.GradingStatus != constant.GradingStatusAuto || item.CorrectAnswerSnapshot == "" {
		t.Fatalf("expected answer, score, grading status and correct answer snapshot, got %#v", item)
	}

	if err := gormDB.Exec(`
		INSERT INTO exams (
			id, tenant_id, paper_id, name, start_time, end_time, duration_minutes,
			max_attempts, result_strategy, publish_mode, invite_code, status,
			created_at, updated_at, ext_json
		) VALUES (2, 10, 100, '跨考试', ?, ?, 120, 1, 'latest', 'manual_publish', 'PM-CROSS', 'published', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow+7_200_000, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed cross exam: %v", err)
	}
	seedExamDetailTarget(t, gormDB, 2, serviceexam.TargetTypeSpace, 100)
	crossRecorder := httptest.NewRecorder()
	router.ServeHTTP(crossRecorder, authorizedRequest(http.MethodGet, "/api/v1/exams/2/attempts/1801/answer-sheet?tenant_id=10", nil, authHeader))
	if crossRecorder.Code != http.StatusNotFound {
		t.Fatalf("expected cross-exam answer sheet 404, status = %d, body = %s", crossRecorder.Code, crossRecorder.Body.String())
	}
}

func TestExamAPIManagementSettingsWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedSpaceAPITestData(t, gormDB)
	seedExamDetailTarget(t, gormDB, 1, serviceexam.TargetTypeSpace, 100)

	router := NewRouter(RouterOptions{DB: gormDB, Now: func() int64 { return fixedAPINow - 60_000 }})
	authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodPost, "/api/v1/exams/1/settings", []byte(`{
		"tenant_id": 10,
		"publish_mode": "manual_publish",
		"score_publish_time": 1779795600000
	}`), authHeader))
	if recorder.Code != http.StatusOK {
		t.Fatalf("management settings status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var exam struct {
		PublishMode      string
		ScorePublishTime *int64
		DurationMinutes  int
	}
	if err := gormDB.Table("exams").
		Select("publish_mode, score_publish_time, duration_minutes").
		Where("tenant_id = ? AND id = ?", 10, 1).
		Scan(&exam).Error; err != nil {
		t.Fatalf("query updated management settings exam: %v", err)
	}
	if exam.PublishMode != serviceexam.PublishModeManualPublish || exam.ScorePublishTime == nil || *exam.ScorePublishTime != 1779795600000 || exam.DurationMinutes != 120 {
		t.Fatalf("unexpected updated settings exam: %#v", exam)
	}

	var logCount int64
	if err := gormDB.Table("exam_operation_logs").
		Where("tenant_id = ? AND exam_id = ? AND operation_type = ?", 10, 1, serviceexam.OperationTypeUpdateSettings).
		Count(&logCount).Error; err != nil {
		t.Fatalf("count update_settings operation logs: %v", err)
	}
	if logCount != 1 {
		t.Fatalf("expected one update_settings operation log, got %d", logCount)
	}

	unsupportedRecorder := httptest.NewRecorder()
	router.ServeHTTP(unsupportedRecorder, authorizedRequest(http.MethodPost, "/api/v1/exams/1/settings", []byte(`{
		"tenant_id": 10,
		"publish_mode": "manual_publish",
		"duration_minutes": 45
	}`), authHeader))
	if unsupportedRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected unsupported settings field rejected, got status = %d, body = %s", unsupportedRecorder.Code, unsupportedRecorder.Body.String())
	}

	passwordHash, err := crypto.HashPassword("papermind123")
	if err != nil {
		t.Fatalf("hash teacher settings password: %v", err)
	}
	if err := gormDB.Table("users").
		Where("username = ?", "teacher_li").
		Update("password_hash", passwordHash).Error; err != nil {
		t.Fatalf("update teacher settings password: %v", err)
	}
	if err := gormDB.Table("space_members").
		Where("tenant_id = ? AND space_id = ? AND user_id = ?", 10, 100, 20).
		Update("role_in_space", "teacher").Error; err != nil {
		t.Fatalf("demote settings teacher member: %v", err)
	}

	teacherAuthHeader := tenantAuthHeader(t, router, 10, "teacher_li", "papermind123")
	teacherRecorder := httptest.NewRecorder()
	router.ServeHTTP(teacherRecorder, authorizedRequest(http.MethodPost, "/api/v1/exams/1/settings", []byte(`{
		"tenant_id": 10,
		"space_id": 100,
		"publish_mode": "immediate_score"
	}`), teacherAuthHeader))
	if teacherRecorder.Code != http.StatusForbidden {
		t.Fatalf("teacher settings status = %d, body = %s", teacherRecorder.Code, teacherRecorder.Body.String())
	}
}

func TestExamAPIManagementOperationLogsWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedSpaceAPITestData(t, gormDB)
	seedExamDetailTarget(t, gormDB, 1, serviceexam.TargetTypeSpace, 100)
	seedExamOperationLogAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{DB: gormDB, Now: func() int64 { return fixedAPINow }})
	authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

	type operationLogListResponse struct {
		Items []struct {
			ID               uint64  `json:"id"`
			OperationType    string  `json:"operation_type"`
			OperationTitle   string  `json:"operation_title"`
			OperationDetail  string  `json:"operation_detail"`
			ActorID          uint64  `json:"actor_id"`
			ActorRole        string  `json:"actor_role"`
			OperationGroupID string  `json:"operation_group_id"`
			SpaceID          *uint64 `json:"space_id"`
			CreatedAt        int64   `json:"created_at"`
		} `json:"items"`
		Page     int   `json:"page"`
		PageSize int   `json:"page_size"`
		Total    int64 `json:"total"`
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodGet, "/api/v1/exams/1/logs?tenant_id=10&page=1&page_size=10", nil, authHeader))
	if recorder.Code != http.StatusOK {
		t.Fatalf("operation logs status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	body := decodeExamAPIResponse[operationLogListResponse](t, recorder.Body.Bytes())
	if body.Data.Total != 3 || len(body.Data.Items) != 3 {
		t.Fatalf("expected tenant admin to see all logs, got %#v", body.Data)
	}
	if body.Data.Items[0].OperationType != serviceexam.OperationTypeUpdateSettings || body.Data.Items[1].OperationType != serviceexam.OperationTypeSendInvite {
		t.Fatalf("expected logs ordered by created_at desc, got %#v", body.Data.Items)
	}
	if body.Data.Items[1].OperationGroupID != "seed-group-1" {
		t.Fatalf("expected operation group id from ext_json, got %#v", body.Data.Items[1])
	}

	filterRecorder := httptest.NewRecorder()
	router.ServeHTTP(filterRecorder, authorizedRequest(http.MethodGet, "/api/v1/exams/1/logs?tenant_id=10&operation_type=send_invite&page=1&page_size=10", nil, authHeader))
	if filterRecorder.Code != http.StatusOK {
		t.Fatalf("filtered operation logs status = %d, body = %s", filterRecorder.Code, filterRecorder.Body.String())
	}
	filterBody := decodeExamAPIResponse[operationLogListResponse](t, filterRecorder.Body.Bytes())
	if filterBody.Data.Total != 1 || filterBody.Data.Items[0].OperationType != serviceexam.OperationTypeSendInvite {
		t.Fatalf("expected only send_invite log, got %#v", filterBody.Data)
	}

	passwordHash, err := crypto.HashPassword("papermind123")
	if err != nil {
		t.Fatalf("hash operation log scope password: %v", err)
	}
	if err := gormDB.Table("users").
		Where("username = ?", "teacher_li").
		Update("password_hash", passwordHash).Error; err != nil {
		t.Fatalf("update operation log scope password: %v", err)
	}
	spaceAdminHeader := tenantAuthHeader(t, router, 10, "teacher_li", "papermind123")
	spaceRecorder := httptest.NewRecorder()
	router.ServeHTTP(spaceRecorder, authorizedRequest(http.MethodGet, "/api/v1/exams/1/logs?tenant_id=10&page=1&page_size=10", nil, spaceAdminHeader))
	if spaceRecorder.Code != http.StatusOK {
		t.Fatalf("space operation logs status = %d, body = %s", spaceRecorder.Code, spaceRecorder.Body.String())
	}
	spaceBody := decodeExamAPIResponse[operationLogListResponse](t, spaceRecorder.Body.Bytes())
	if spaceBody.Data.Total != 1 || len(spaceBody.Data.Items) != 1 || spaceBody.Data.Items[0].SpaceID == nil || *spaceBody.Data.Items[0].SpaceID != 100 {
		t.Fatalf("expected space manager to see only space 100 logs, got %#v", spaceBody.Data)
	}
}

func TestExamAPIImportCandidatesWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedSpaceAPITestData(t, gormDB)
	seedExamDetailTarget(t, gormDB, 1, serviceexam.TargetTypeSpace, 100)
	seedImportCandidateAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{DB: gormDB, Now: func() int64 { return fixedAPINow - 60_000 }})
	authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

	payload := []byte(`{"tenant_id":10,"user_ids":[31,32,33]}`)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodPost, "/api/v1/exams/1/candidates/import", payload, authHeader))
	if recorder.Code != http.StatusOK {
		t.Fatalf("import candidates status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	body := decodeExamAPIResponse[examCandidateImportResponse](t, recorder.Body.Bytes())
	if body.Data.ImportedCount != 1 || body.Data.SkippedCount != 2 {
		t.Fatalf("expected one imported and two skipped, got %#v", body.Data)
	}
	if !body.Data.Permissions.CanManageCandidates {
		t.Fatalf("tenant admin should keep manage permission, got %#v", body.Data.Permissions)
	}
	var importedTargets []uint64
	if err := gormDB.Table("exam_targets").
		Select("target_id").
		Where("tenant_id = ? AND exam_id = ? AND target_type = ?", 10, 1, serviceexam.TargetTypeUser).
		Order("target_id ASC").
		Scan(&importedTargets).Error; err != nil {
		t.Fatalf("query imported candidate targets: %v", err)
	}
	if len(importedTargets) != 1 || importedTargets[0] != 31 {
		t.Fatalf("expected only user 31 imported, got %#v", importedTargets)
	}
	var logCount int64
	if err := gormDB.Table("exam_operation_logs").
		Where("tenant_id = ? AND exam_id = ? AND operation_type = ?", 10, 1, serviceexam.OperationTypeImportCandidates).
		Count(&logCount).Error; err != nil {
		t.Fatalf("count import operation logs: %v", err)
	}
	if logCount != 1 {
		t.Fatalf("expected one import operation log, got %d", logCount)
	}

	startedRouter := NewRouter(RouterOptions{DB: gormDB, Now: func() int64 { return fixedAPINow + 1 }})
	startedAuthHeader := tenantAuthHeader(t, startedRouter, 10, "tenant.admin", "papermind123")
	startedRecorder := httptest.NewRecorder()
	startedRouter.ServeHTTP(startedRecorder, authorizedRequest(http.MethodPost, "/api/v1/exams/1/candidates/import", payload, startedAuthHeader))
	if startedRecorder.Code != http.StatusBadRequest || !strings.Contains(startedRecorder.Body.String(), "考试已开始") {
		t.Fatalf("expected started exam import to return clear 400, status = %d, body = %s", startedRecorder.Code, startedRecorder.Body.String())
	}
}

func TestExamAPIResendInvitationsWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedSpaceAPITestData(t, gormDB)
	seedExamDetailTarget(t, gormDB, 1, serviceexam.TargetTypeSpace, 100)
	seedImportCandidateAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{DB: gormDB, Now: func() int64 { return fixedAPINow - 60_000 }})
	authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

	payload := []byte(`{"tenant_id":10,"user_ids":[31,32,33]}`)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodPost, "/api/v1/exams/1/invitations/resend", payload, authHeader))
	if recorder.Code != http.StatusOK {
		t.Fatalf("resend invitations status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	body := decodeExamAPIResponse[examInvitationResendResponse](t, recorder.Body.Bytes())
	if body.Data.SentCount != 1 || body.Data.SkippedCount != 2 || body.Data.InviteCode == "" {
		t.Fatalf("expected one sent, two skipped and invite code, got %#v", body.Data)
	}
	if !body.Data.Permissions.CanManageCandidates {
		t.Fatalf("tenant admin should keep manage permission, got %#v", body.Data.Permissions)
	}
	var logCount int64
	if err := gormDB.Table("exam_operation_logs").
		Where("tenant_id = ? AND exam_id = ? AND operation_type = ?", 10, 1, serviceexam.OperationTypeSendInvite).
		Count(&logCount).Error; err != nil {
		t.Fatalf("count send_invite operation logs: %v", err)
	}
	if logCount != 1 {
		t.Fatalf("expected one send_invite operation log, got %d", logCount)
	}

	startedRouter := NewRouter(RouterOptions{DB: gormDB, Now: func() int64 { return fixedAPINow + 1 }})
	startedAuthHeader := tenantAuthHeader(t, startedRouter, 10, "tenant.admin", "papermind123")
	startedRecorder := httptest.NewRecorder()
	startedRouter.ServeHTTP(startedRecorder, authorizedRequest(http.MethodPost, "/api/v1/exams/1/invitations/resend", payload, startedAuthHeader))
	if startedRecorder.Code != http.StatusBadRequest || !strings.Contains(startedRecorder.Body.String(), "考试已开始") {
		t.Fatalf("expected started exam resend to return clear 400, status = %d, body = %s", startedRecorder.Code, startedRecorder.Body.String())
	}
}

func TestExamAPICandidateWritesRespectSpaceAdminAndTeacherScopeWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedSpaceAPITestData(t, gormDB)
	seedExamDetailTarget(t, gormDB, 1, serviceexam.TargetTypeSpace, 100)
	seedImportCandidateAPITestData(t, gormDB)
	passwordHash, err := crypto.HashPassword("papermind123")
	if err != nil {
		t.Fatalf("hash candidate write scope password: %v", err)
	}
	if err := gormDB.Table("users").
		Where("username IN ?", []string{"teacher_li", "teacher_zhao"}).
		Update("password_hash", passwordHash).Error; err != nil {
		t.Fatalf("update candidate write scope passwords: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO space_members (
			id, tenant_id, space_id, user_id, role_in_space, status, created_at, updated_at, ext_json
		) VALUES (3, 10, 100, 22, 'teacher', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed candidate write scope teacher member: %v", err)
	}

	router := NewRouter(RouterOptions{DB: gormDB, Now: func() int64 { return fixedAPINow - 60_000 }})
	spaceAdminHeader := tenantAuthHeader(t, router, 10, "teacher_li", "papermind123")

	payload := []byte(`{"tenant_id":10,"space_id":100,"user_ids":[31,32]}`)
	importRecorder := httptest.NewRecorder()
	router.ServeHTTP(importRecorder, authorizedRequest(http.MethodPost, "/api/v1/exams/1/candidates/import", payload, spaceAdminHeader))
	if importRecorder.Code != http.StatusOK {
		t.Fatalf("space admin import status = %d, body = %s", importRecorder.Code, importRecorder.Body.String())
	}
	importBody := decodeExamAPIResponse[examCandidateImportResponse](t, importRecorder.Body.Bytes())
	if importBody.Data.ImportedCount != 1 || importBody.Data.SkippedCount != 1 {
		t.Fatalf("expected space admin to import only authorized-space candidate, got %#v", importBody.Data)
	}

	var importedTargets []uint64
	if err := gormDB.Table("exam_targets").
		Select("target_id").
		Where("tenant_id = ? AND exam_id = ? AND target_type = ?", 10, 1, serviceexam.TargetTypeUser).
		Order("target_id ASC").
		Scan(&importedTargets).Error; err != nil {
		t.Fatalf("query space admin imported targets: %v", err)
	}
	if len(importedTargets) != 1 || importedTargets[0] != 31 {
		t.Fatalf("expected only authorized-space user 31 imported, got %#v", importedTargets)
	}

	resendRecorder := httptest.NewRecorder()
	router.ServeHTTP(resendRecorder, authorizedRequest(http.MethodPost, "/api/v1/exams/1/invitations/resend", payload, spaceAdminHeader))
	if resendRecorder.Code != http.StatusOK {
		t.Fatalf("space admin resend status = %d, body = %s", resendRecorder.Code, resendRecorder.Body.String())
	}
	resendBody := decodeExamAPIResponse[examInvitationResendResponse](t, resendRecorder.Body.Bytes())
	if resendBody.Data.SentCount != 1 || resendBody.Data.SkippedCount != 1 {
		t.Fatalf("expected space admin to resend only authorized-space candidate, got %#v", resendBody.Data)
	}

	teacherHeader := tenantAuthHeader(t, router, 10, "teacher_zhao", "papermind123")
	teacherImportRecorder := httptest.NewRecorder()
	router.ServeHTTP(teacherImportRecorder, authorizedRequest(http.MethodPost, "/api/v1/exams/1/candidates/import", payload, teacherHeader))
	if teacherImportRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected teacher import to be forbidden, status = %d, body = %s", teacherImportRecorder.Code, teacherImportRecorder.Body.String())
	}
	teacherResendRecorder := httptest.NewRecorder()
	router.ServeHTTP(teacherResendRecorder, authorizedRequest(http.MethodPost, "/api/v1/exams/1/invitations/resend", payload, teacherHeader))
	if teacherResendRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected teacher resend to be forbidden, status = %d, body = %s", teacherResendRecorder.Code, teacherResendRecorder.Body.String())
	}
}

func TestExamAPICandidateWritesRespectRequestedSpaceScopeForMultiSpaceAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedSpaceAPITestData(t, gormDB)
	seedExamDetailTarget(t, gormDB, 1, serviceexam.TargetTypeSpace, 100)
	seedExamDetailTarget(t, gormDB, 1, serviceexam.TargetTypeSpace, 200)
	seedImportCandidateAPITestData(t, gormDB)
	passwordHash, err := crypto.HashPassword("papermind123")
	if err != nil {
		t.Fatalf("hash scoped candidate write password: %v", err)
	}
	if err := gormDB.Table("users").
		Where("username = ?", "teacher_li").
		Update("password_hash", passwordHash).Error; err != nil {
		t.Fatalf("update scoped candidate write password: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO space_members (
			id, tenant_id, space_id, user_id, role_in_space, status, created_at, updated_at, ext_json
		) VALUES (4, 10, 200, 20, 'space_admin', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed second scoped admin membership: %v", err)
	}

	router := NewRouter(RouterOptions{DB: gormDB, Now: func() int64 { return fixedAPINow - 60_000 }})
	spaceAdminHeader := tenantAuthHeader(t, router, 10, "teacher_li", "papermind123")

	missingSpaceRecorder := httptest.NewRecorder()
	router.ServeHTTP(missingSpaceRecorder, authorizedRequest(http.MethodPost, "/api/v1/exams/1/candidates/import", []byte(`{"tenant_id":10,"user_ids":[31,32]}`), spaceAdminHeader))
	if missingSpaceRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected multi-space admin import without space_id rejected, status = %d, body = %s", missingSpaceRecorder.Code, missingSpaceRecorder.Body.String())
	}

	missingResendRecorder := httptest.NewRecorder()
	router.ServeHTTP(missingResendRecorder, authorizedRequest(http.MethodPost, "/api/v1/exams/1/invitations/resend", []byte(`{"tenant_id":10,"user_ids":[31,32]}`), spaceAdminHeader))
	if missingResendRecorder.Code != http.StatusBadRequest {
		t.Fatalf("expected multi-space admin resend without space_id rejected, status = %d, body = %s", missingResendRecorder.Code, missingResendRecorder.Body.String())
	}

	importPayload := []byte(`{"tenant_id":10,"space_id":100,"user_ids":[31,32]}`)
	importRecorder := httptest.NewRecorder()
	router.ServeHTTP(importRecorder, authorizedRequest(http.MethodPost, "/api/v1/exams/1/candidates/import", importPayload, spaceAdminHeader))
	if importRecorder.Code != http.StatusOK {
		t.Fatalf("scoped space admin import status = %d, body = %s", importRecorder.Code, importRecorder.Body.String())
	}
	importBody := decodeExamAPIResponse[examCandidateImportResponse](t, importRecorder.Body.Bytes())
	if importBody.Data.ImportedCount != 1 || importBody.Data.SkippedCount != 1 {
		t.Fatalf("expected scoped import to keep only requested space candidate, got %#v", importBody.Data)
	}

	var importedTargets []uint64
	if err := gormDB.Table("exam_targets").
		Select("target_id").
		Where("tenant_id = ? AND exam_id = ? AND target_type = ?", 10, 1, serviceexam.TargetTypeUser).
		Order("target_id ASC").
		Scan(&importedTargets).Error; err != nil {
		t.Fatalf("query scoped imported targets: %v", err)
	}
	if len(importedTargets) != 1 || importedTargets[0] != 31 {
		t.Fatalf("expected scoped import to avoid space 200 user, got %#v", importedTargets)
	}

	resendPayload := []byte(`{"tenant_id":10,"space_id":100,"user_ids":[31,32]}`)
	resendRecorder := httptest.NewRecorder()
	router.ServeHTTP(resendRecorder, authorizedRequest(http.MethodPost, "/api/v1/exams/1/invitations/resend", resendPayload, spaceAdminHeader))
	if resendRecorder.Code != http.StatusOK {
		t.Fatalf("scoped space admin resend status = %d, body = %s", resendRecorder.Code, resendRecorder.Body.String())
	}
	resendBody := decodeExamAPIResponse[examInvitationResendResponse](t, resendRecorder.Body.Bytes())
	if resendBody.Data.SentCount != 1 || resendBody.Data.SkippedCount != 1 {
		t.Fatalf("expected scoped resend to keep only requested space candidate, got %#v", resendBody.Data)
	}
}

func TestExamAPIPublishWithMultipleTargetsWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedSpaceAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:            gormDB,
		Now:           func() int64 { return fixedAPINow },
		CodeGenerator: fixedCodeGenerator{code: "PM2030"},
	})
	authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

	payload := []byte(`{
		"tenant_id": 10,
		"paper_id": 100,
		"name": "高一语文多目标月考",
		"targets": [
			{"target_type": "space", "target_id": 100},
			{"target_type": "user", "target_id": 21}
		],
		"start_time": 1772269200000,
		"end_time": 1772276400000,
		"duration_minutes": 120,
		"max_attempts": 1,
		"result_strategy": "latest",
		"publish_mode": "manual_publish"
	}`)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodPost, "/api/v1/exams", payload, authHeader))
	if recorder.Code != http.StatusOK {
		t.Fatalf("multi-target publish status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	body := decodeExamAPIResponse[examResponse](t, recorder.Body.Bytes())
	if body.Data.Status != "published" || body.Data.InviteCode != "PM2030" {
		t.Fatalf("unexpected multi-target published exam response: %#v", body.Data)
	}
	if body.Data.TargetType != "space" || body.Data.TargetID != 100 {
		t.Fatalf("expected first target kept for compatibility, got %#v", body.Data)
	}
	if len(body.Data.Targets) != 2 ||
		body.Data.Targets[0].TargetType != "space" || body.Data.Targets[0].TargetID != 100 ||
		body.Data.Targets[1].TargetType != "user" || body.Data.Targets[1].TargetID != 21 {
		t.Fatalf("expected multi-target publish response to keep full targets, got %#v", body.Data.Targets)
	}

	var targets []struct {
		TargetType string
		TargetID   uint64
	}
	if err := gormDB.Table("exam_targets").
		Select("target_type, target_id").
		Where("tenant_id = ? AND exam_id = ?", 10, body.Data.ID).
		Order("id ASC").
		Scan(&targets).Error; err != nil {
		t.Fatalf("query multi-target rows: %v", err)
	}
	if len(targets) != 2 {
		t.Fatalf("expected two target rows, got %#v", targets)
	}
	if targets[0].TargetType != "space" || targets[0].TargetID != 100 ||
		targets[1].TargetType != "user" || targets[1].TargetID != 21 {
		t.Fatalf("unexpected multi-target rows: %#v", targets)
	}

	var logCount int64
	if err := gormDB.Table("exam_operation_logs").
		Where("tenant_id = ? AND exam_id = ? AND operation_type = ?", 10, body.Data.ID, serviceexam.OperationTypePublishExam).
		Count(&logCount).Error; err != nil {
		t.Fatalf("count publish_exam operation logs: %v", err)
	}
	if logCount != 1 {
		t.Fatalf("expected one publish_exam operation log, got %d", logCount)
	}
}

func TestExamPublishUserTargetPersistsAuthorizedSpacesOnlyForTeacher(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamBusinessLoginAPITestData(t, gormDB)
	seedPaperTeacherSpaceAPITestData(t, gormDB)
	seedPaperAPITestData(t, gormDB)
	if err := gormDB.Table("papers").
		Where("tenant_id = ? AND id = ?", 10, 100).
		Update("status", "enabled").Error; err != nil {
		t.Fatalf("enable publish paper: %v", err)
	}

	if err := gormDB.Exec(`
		INSERT INTO users (
			id, username, real_name, phone, email, password_hash, status,
			created_at, updated_at, ext_json
		) VALUES (601, 'student.publish.scope', '跨班考生', '13800000601', 'student601.scope@example.test', 'hash', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed scoped publish student: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO tenant_user_memberships (
			id, tenant_id, user_id, role, status, created_at, updated_at, ext_json
		) VALUES (601, 10, 601, 'student', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed scoped publish student role: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO spaces (
			id, tenant_id, name, type, status, created_at, updated_at, ext_json
		) VALUES (302, 10, '高一 2 班', 'class', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed second publish space: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO space_members (
			id, tenant_id, space_id, user_id, role_in_space, status, created_at, updated_at, ext_json
		) VALUES
			(701, 10, 302, 501, 'teacher', 'enabled', ?, ?, '{}'),
			(702, 10, 301, 601, 'student', 'enabled', ?, ?, '{}'),
			(703, 10, 302, 601, 'student', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow, fixedAPINow, fixedAPINow, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed cross-space student member: %v", err)
	}

	router := NewRouter(RouterOptions{
		DB:            gormDB,
		Now:           func() int64 { return fixedAPINow },
		CodeGenerator: fixedCodeGenerator{code: "PM2031"},
	})
	authHeader := tenantAuthHeader(t, router, 10, "teacher.exam", "papermind123")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodPost, "/api/v1/exams", []byte(`{
		"tenant_id": 10,
		"paper_id": 100,
		"name": "教师定向考生发布",
		"target_type": "user",
		"target_id": 601,
		"start_time": 1772269200000,
		"end_time": 1772276400000,
		"duration_minutes": 120,
		"max_attempts": 1,
		"result_strategy": "latest",
		"publish_mode": "manual_publish"
	}`), authHeader))
	if recorder.Code != http.StatusOK {
		t.Fatalf("teacher scoped publish status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	body := decodeExamAPIResponse[examResponse](t, recorder.Body.Bytes())

	var extJSON string
	var targetID uint64
	if err := gormDB.Table("exam_targets").
		Select("id, ext_json").
		Where("tenant_id = ? AND exam_id = ? AND target_type = ? AND target_id = ?", 10, body.Data.ID, "user", 601).
		Row().
		Scan(&targetID, &extJSON); err != nil {
		t.Fatalf("query teacher scoped target: %v", err)
	}
	if extJSON != `{}` {
		t.Fatalf("expected teacher publish target ext_json to stay empty, got %s", extJSON)
	}
	var scopedSpaceIDs []uint64
	if err := gormDB.Table("exam_target_scope_spaces").
		Select("space_id").
		Where("tenant_id = ? AND exam_id = ? AND exam_target_id = ?", 10, body.Data.ID, targetID).
		Order("space_id ASC").
		Scan(&scopedSpaceIDs).Error; err != nil {
		t.Fatalf("query teacher scoped target spaces: %v", err)
	}
	if len(scopedSpaceIDs) != 1 || scopedSpaceIDs[0] != 301 {
		t.Fatalf("expected teacher publish to keep only authorized space in scope table, got %#v", scopedSpaceIDs)
	}
}

func TestExamPublishAllowsTeacherToUsePublicPaperForCurrentSpaceWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamBusinessLoginAPITestData(t, gormDB)
	seedPaperTeacherSpaceAPITestData(t, gormDB)
	seedPublicPublishPaperAPITestData(t, gormDB, 700)

	router := NewRouter(RouterOptions{
		DB:            gormDB,
		Now:           func() int64 { return fixedAPINow },
		CodeGenerator: fixedCodeGenerator{code: "PM7030"},
	})
	authHeader := tenantAuthHeader(t, router, 10, "teacher.exam", "papermind123")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodPost, "/api/v1/exams", []byte(`{
		"tenant_id": 10,
		"paper_id": 700,
		"name": "公共试卷当前空间考试",
		"target_type": "space",
		"target_id": 301,
		"start_time": 1772269200000,
		"end_time": 1772276400000,
		"duration_minutes": 120,
		"max_attempts": 1,
		"result_strategy": "latest",
		"publish_mode": "manual_publish",
		"status": "published"
	}`), authHeader))
	if recorder.Code != http.StatusOK {
		t.Fatalf("teacher public paper space publish status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	body := decodeExamAPIResponse[examResponse](t, recorder.Body.Bytes())
	if body.Data.PaperID != 700 || body.Data.TargetType != "space" || body.Data.TargetID != 301 {
		t.Fatalf("unexpected public paper publish response: %#v", body.Data)
	}

	var scopedSpaceIDs []uint64
	if err := gormDB.Table("exam_target_scope_spaces").
		Select("space_id").
		Where("tenant_id = ? AND exam_id = ?", 10, body.Data.ID).
		Order("space_id ASC").
		Scan(&scopedSpaceIDs).Error; err != nil {
		t.Fatalf("query public paper target spaces: %v", err)
	}
	if len(scopedSpaceIDs) != 1 || scopedSpaceIDs[0] != 301 {
		t.Fatalf("expected public paper publish to scope current space only, got %#v", scopedSpaceIDs)
	}
}

func TestExamPublishPublicPaperUserTargetKeepsTeacherCurrentSpaceOnlyWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamBusinessLoginAPITestData(t, gormDB)
	seedPaperTeacherSpaceAPITestData(t, gormDB)
	seedPublicPublishPaperAPITestData(t, gormDB, 701)

	if err := gormDB.Exec(`
		INSERT INTO users (
			id, username, real_name, phone, email, password_hash, status,
			created_at, updated_at, ext_json
		) VALUES (601, 'student.public.scope', '公共试卷跨班学生', '13800000601', 'student601.public@example.test', 'hash', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed public paper student: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO tenant_user_memberships (
			id, tenant_id, user_id, role, status, created_at, updated_at, ext_json
		) VALUES (601, 10, 601, 'student', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed public paper student role: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO spaces (
			id, tenant_id, name, type, status, created_at, updated_at, ext_json
		) VALUES (302, 10, '高一 2 班', 'class', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed public paper second space: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO space_members (
			id, tenant_id, space_id, user_id, role_in_space, status, created_at, updated_at, ext_json
		) VALUES
			(701, 10, 302, 501, 'teacher', 'enabled', ?, ?, '{}'),
			(702, 10, 301, 601, 'student', 'enabled', ?, ?, '{}'),
			(703, 10, 302, 601, 'student', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow, fixedAPINow, fixedAPINow, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed public paper cross-space student member: %v", err)
	}

	router := NewRouter(RouterOptions{
		DB:            gormDB,
		Now:           func() int64 { return fixedAPINow },
		CodeGenerator: fixedCodeGenerator{code: "PM7031"},
	})
	authHeader := tenantAuthHeader(t, router, 10, "teacher.exam", "papermind123")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodPost, "/api/v1/exams", []byte(`{
		"tenant_id": 10,
		"paper_id": 701,
		"name": "公共试卷指定学生发布",
		"target_type": "user",
		"target_id": 601,
		"targets": [{"target_type": "user", "target_id": 601, "scope_space_ids": [301]}],
		"start_time": 1772269200000,
		"end_time": 1772276400000,
		"duration_minutes": 120,
		"max_attempts": 1,
		"result_strategy": "latest",
		"publish_mode": "manual_publish",
		"status": "published"
	}`), authHeader))
	if recorder.Code != http.StatusOK {
		t.Fatalf("teacher public paper user publish status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	body := decodeExamAPIResponse[examResponse](t, recorder.Body.Bytes())

	var scopedSpaceIDs []uint64
	if err := gormDB.Table("exam_target_scope_spaces").
		Select("space_id").
		Where("tenant_id = ? AND exam_id = ?", 10, body.Data.ID).
		Order("space_id ASC").
		Scan(&scopedSpaceIDs).Error; err != nil {
		t.Fatalf("query public paper user target spaces: %v", err)
	}
	if len(scopedSpaceIDs) != 1 || scopedSpaceIDs[0] != 301 {
		t.Fatalf("expected public paper user target to keep teacher current space only, got %#v", scopedSpaceIDs)
	}
}

func TestExamPublishRejectsUserTargetWithoutStudentSpaceRoleWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamBusinessLoginAPITestData(t, gormDB)
	seedPaperTeacherSpaceAPITestData(t, gormDB)
	seedPaperAPITestData(t, gormDB)
	if err := gormDB.Table("papers").
		Where("tenant_id = ? AND id = ?", 10, 100).
		Update("status", "enabled").Error; err != nil {
		t.Fatalf("enable publish paper: %v", err)
	}

	if err := gormDB.Exec(`
		INSERT INTO users (
			id, username, real_name, phone, email, password_hash, status,
			created_at, updated_at, ext_json
		) VALUES (601, 'student.publish.teacher.role', '非学生空间角色考生', '13800000601', 'student601.teacher@example.test', 'hash', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed non-student space-role user: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO tenant_user_memberships (
			id, tenant_id, user_id, role, status, created_at, updated_at, ext_json
		) VALUES (601, 10, 601, 'student', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed tenant student role: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO space_members (
			id, tenant_id, space_id, user_id, role_in_space, status, created_at, updated_at, ext_json
		) VALUES (702, 10, 301, 601, 'teacher', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed non-student space role: %v", err)
	}

	router := NewRouter(RouterOptions{
		DB:            gormDB,
		Now:           func() int64 { return fixedAPINow },
		CodeGenerator: fixedCodeGenerator{code: "PM2032"},
	})
	authHeader := tenantAuthHeader(t, router, 10, "teacher.exam", "papermind123")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodPost, "/api/v1/exams", []byte(`{
		"tenant_id": 10,
		"paper_id": 100,
		"name": "非学生空间角色定向发布",
		"target_type": "user",
		"target_id": 601,
		"start_time": 1772269200000,
		"end_time": 1772276400000,
		"duration_minutes": 120,
		"max_attempts": 1,
		"result_strategy": "latest",
		"publish_mode": "manual_publish"
	}`), authHeader))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected non-student space role user target forbidden, got status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestTenantAdminPublishRejectsUserTargetWithoutStudentSpaceRoleWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedSpaceAPITestData(t, gormDB)

	if err := gormDB.Exec(`
		INSERT INTO users (
			id, username, real_name, phone, email, password_hash, status,
			created_at, updated_at, ext_json
		) VALUES (601, 'student.publish.admin.teacher.role', '非学生空间角色考生', '13800000601', 'student601.admin.teacher@example.test', 'hash', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed tenant-admin target user: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO tenant_user_memberships (
			id, tenant_id, user_id, role, status, created_at, updated_at, ext_json
		) VALUES (601, 10, 601, 'student', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed tenant-admin target tenant role: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO space_members (
			id, tenant_id, space_id, user_id, role_in_space, status, created_at, updated_at, ext_json
		) VALUES (702, 10, 100, 601, 'teacher', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed tenant-admin target space role: %v", err)
	}

	router := NewRouter(RouterOptions{
		DB:            gormDB,
		Now:           func() int64 { return fixedAPINow },
		CodeGenerator: fixedCodeGenerator{code: "PM2033"},
	})
	authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodPost, "/api/v1/exams", []byte(`{
		"tenant_id": 10,
		"paper_id": 100,
		"name": "租户管理员非学生空间角色定向发布",
		"target_type": "user",
		"target_id": 601,
		"start_time": 1772269200000,
		"end_time": 1772276400000,
		"duration_minutes": 120,
		"max_attempts": 1,
		"result_strategy": "latest",
		"publish_mode": "manual_publish"
	}`), authHeader))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected tenant admin non-student space role user target forbidden, got status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestExamPublishRejectsUnsupportedResultStrategyAndPublishMode(t *testing.T) {
	for _, tc := range []struct {
		name           string
		resultStrategy string
		publishMode    string
	}{
		{name: "unsupported result strategy", resultStrategy: "lowest", publishMode: serviceexam.PublishModeManualPublish},
		{name: "unsupported publish mode", resultStrategy: serviceexam.ResultStrategyLatest, publishMode: "manual-pubish"},
	} {
		t.Run(tc.name, func(t *testing.T) {
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
			payload := []byte(fmt.Sprintf(`{
				"tenant_id": 10,
				"paper_id": 100,
				"name": "非法发布策略考试",
				"target_type": "space",
				"target_id": 100,
				"start_time": 1772269200000,
				"end_time": 1772276400000,
				"duration_minutes": 120,
				"max_attempts": 1,
				"result_strategy": %q,
				"publish_mode": %q
			}`, tc.resultStrategy, tc.publishMode))

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, authorizedRequest(http.MethodPost, "/api/v1/exams", payload, authHeader))
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("expected invalid publish settings rejected, got status = %d, body = %s", recorder.Code, recorder.Body.String())
			}

			var createdCount int64
			if err := gormDB.Table("exams").
				Where("tenant_id = ? AND name = ?", 10, "非法发布策略考试").
				Count(&createdCount).Error; err != nil {
				t.Fatalf("count invalid publish exams: %v", err)
			}
			if createdCount != 0 {
				t.Fatalf("expected invalid publish settings not to create exam, got %d", createdCount)
			}
		})
	}
}

func TestExamBusinessRoutesRejectDisabledTenantAdminSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedSpaceAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")
	if err := gormDB.Table("tenant_user_memberships").
		Where("tenant_id = ? AND user_id = ?", 10, 99).
		Update("status", "disabled").Error; err != nil {
		t.Fatalf("disable tenant admin: %v", err)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodGet, "/api/v1/exams?tenant_id=10", nil, authHeader))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected disabled tenant admin rejected, got status = %d, body = %s", recorder.Code, recorder.Body.String())
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
		{
			name: "mixed allowed and forbidden targets",
			payload: []byte(`{
				"tenant_id": 10,
				"paper_id": 100,
				"name": "混合越权目标发布",
				"targets": [
					{"target_type": "space", "target_id": 301},
					{"target_type": "space", "target_id": 302}
				],
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

func TestExamEntryResultUsesExamResultStrategy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedTakingAPITestData(t, gormDB)
	seedVisibleResultAPITestData(t, gormDB)
	seedHighestStrategyVisibleResultAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "student20", "papermind123")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodGet, "/api/v1/exam-entry/results/701", nil, authHeader))
	if recorder.Code != http.StatusOK {
		t.Fatalf("result status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	body := decodeExamAPIResponse[visibleResultResponse](t, recorder.Body.Bytes())
	if body.Data.AttemptID != 700 || body.Data.TotalScore != "8" {
		t.Fatalf("expected highest strategy to return attempt 700 score 8, got %#v", body.Data)
	}
}

func TestExamEntryResultRejectsDisabledStudentSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedTakingAPITestData(t, gormDB)
	seedVisibleResultAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "student20", "papermind123")
	if err := gormDB.Table("tenant_user_memberships").
		Where("tenant_id = ? AND user_id = ?", 10, 20).
		Update("status", "disabled").Error; err != nil {
		t.Fatalf("disable tenant user after login: %v", err)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodGet, "/api/v1/exam-entry/results/700", nil, authHeader))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected disabled student result access to be forbidden, got status = %d, body = %s", recorder.Code, recorder.Body.String())
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
	seedSpaceAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:        gormDB,
		Now:       func() int64 { return fixedAPINow },
		ExportDir: t.TempDir(),
	})
	authHeader := tenantAuthHeader(t, router, 10, "teacher.li", "papermind123")
	tenantAdminHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

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
	var gradeAnswerLogCount int64
	if err := gormDB.Table("exam_operation_logs").
		Where("tenant_id = ? AND exam_id = ? AND operation_type = ?", 10, 1, serviceexam.OperationTypeGradeAnswer).
		Count(&gradeAnswerLogCount).Error; err != nil {
		t.Fatalf("count grade_answer operation logs: %v", err)
	}
	if gradeAnswerLogCount != 1 {
		t.Fatalf("expected one grade_answer operation log, got %d", gradeAnswerLogCount)
	}

	configRecorder := httptest.NewRecorder()
	router.ServeHTTP(configRecorder, authorizedRequest(http.MethodPost, "/api/v1/results/publish-config", []byte(`{
		"tenant_id": 10,
		"exam_id": 1,
		"space_id": 301,
		"publish_mode": "manual_publish",
		"score_publish_time": 1779795600000
	}`), tenantAdminHeader))
	if configRecorder.Code != http.StatusOK {
		t.Fatalf("publish config status = %d, body = %s", configRecorder.Code, configRecorder.Body.String())
	}
	var publishResultsLogCount int64
	if err := gormDB.Table("exam_operation_logs").
		Where("tenant_id = ? AND exam_id = ? AND operation_type = ?", 10, 1, serviceexam.OperationTypePublishResults).
		Count(&publishResultsLogCount).Error; err != nil {
		t.Fatalf("count publish_results operation logs: %v", err)
	}
	if publishResultsLogCount != 1 {
		t.Fatalf("expected one publish_results operation log, got %d", publishResultsLogCount)
	}
	var publishResultsSpaceID *uint64
	if err := gormDB.Table("exam_operation_logs").
		Select("space_id").
		Where("tenant_id = ? AND exam_id = ? AND operation_type = ?", 10, 1, serviceexam.OperationTypePublishResults).
		Scan(&publishResultsSpaceID).Error; err != nil {
		t.Fatalf("query publish_results operation log space: %v", err)
	}
	if publishResultsSpaceID == nil || *publishResultsSpaceID != 301 {
		t.Fatalf("expected publish_results log scoped to space 301, got %#v", publishResultsSpaceID)
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

func TestGradeShortTextRejectsScoreOutsideQuestionMax(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedReviewResultAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "teacher.li", "papermind123")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodPost, "/api/v1/exam-attempts/900/questions/901/grade", []byte("{\n"+
		"\t\"tenant_id\": 10,\n"+
		"\t\"exam_id\": 1,\n"+
		"\t\"space_id\": 301,\n"+
		"\t\"answer_version\": 7,\n"+
		"\t\"score\": \"10.1\",\n"+
		"\t\"comment\": \"score over max\"\n"+
		"}"), authHeader))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected score over max to be rejected, got status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var answer struct {
		Score         string
		GradingStatus string
	}
	if err := gormDB.Table("exam_answers").
		Select("score, grading_status").
		Where("tenant_id = ? AND attempt_id = ? AND attempt_question_id = ?", 10, 900, 901).
		Scan(&answer).Error; err != nil {
		t.Fatalf("query answer after invalid grade: %v", err)
	}
	if answer.Score != "0" || answer.GradingStatus != "pending" {
		t.Fatalf("expected invalid grade not to modify answer, got %#v", answer)
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
	var exportResultsLogCount int64
	if err := gormDB.Table("exam_operation_logs").
		Where("tenant_id = ? AND exam_id = ? AND operation_type = ? AND space_id = ?", 10, 1, serviceexam.OperationTypeExportResults, 301).
		Count(&exportResultsLogCount).Error; err != nil {
		t.Fatalf("count export_results operation logs: %v", err)
	}
	if exportResultsLogCount != 1 {
		t.Fatalf("expected one export_results operation log for space 301, got %d", exportResultsLogCount)
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

func TestResultPublishConfigRejectsSpaceAdminWriteWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedReviewResultAPITestData(t, gormDB)

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
		"exam_id": 1,
		"space_id": 301,
		"publish_mode": "manual_publish",
		"score_publish_time": 1779795600000
	}`), authHeader))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("space admin publish config status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var exam struct {
		PublishMode      string
		ScorePublishTime *int64
	}
	if err := gormDB.Table("exams").
		Select("publish_mode, score_publish_time").
		Where("tenant_id = ? AND id = ?", 10, 1).
		Scan(&exam).Error; err != nil {
		t.Fatalf("query space admin publish exam: %v", err)
	}
	if exam.PublishMode != "manual_publish" || exam.ScorePublishTime != nil {
		t.Fatalf("space admin publish config should not update exam, got %#v", exam)
	}
}

func TestResultPublishConfigRejectsUnsupportedPublishModeWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)
	seedSpaceAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodPost, "/api/v1/results/publish-config", []byte(`{
		"tenant_id": 10,
		"exam_id": 1,
		"publish_mode": "manual-pubish",
		"score_publish_time": 1779795600000
	}`), authHeader))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid publish mode rejected, got status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var exam struct {
		PublishMode      string
		ScorePublishTime *int64
	}
	if err := gormDB.Table("exams").
		Select("publish_mode, score_publish_time").
		Where("tenant_id = ? AND id = ?", 10, 1).
		Scan(&exam).Error; err != nil {
		t.Fatalf("query invalid publish exam: %v", err)
	}
	if exam.PublishMode == "manual-pubish" || exam.ScorePublishTime != nil {
		t.Fatalf("invalid publish config should not update exam, got %#v", exam)
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

func seedExamDetailTarget(t *testing.T, gormDB *gorm.DB, examID uint64, targetType string, targetID uint64) {
	t.Helper()
	if err := gormDB.Exec(`
		INSERT INTO exam_targets (
			tenant_id, exam_id, target_type, target_id, created_at, ext_json
		) VALUES (?, ?, ?, ?, ?, '{}')
	`, 10, examID, targetType, targetID, fixedAPINow).Error; err != nil {
		t.Fatalf("seed exam detail target: %v", err)
	}
}

func seedExamOverviewPreviewData(t *testing.T, gormDB *gorm.DB) {
	t.Helper()
	if err := gormDB.Exec(`
		INSERT INTO paper_sections (
			id, tenant_id, paper_id, sort_order, name, question_type, instructions, total_score, question_count,
			created_at, updated_at, ext_json
		) VALUES (501, 10, 100, 1, '一、单项选择题', ?, '每题 2 分', 2, 1, ?, ?, '{}')
	`, constant.QuestionTypeSingle, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed overview section: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO questions (
			id, tenant_id, type, difficulty, title, analysis, score_default, shuffle_options, standard_answer, status,
			created_at, updated_at, ext_json
		) VALUES (1501, 10, ?, 'easy', '服务端预览题干', '不应出现在预览接口', 2, false, 'A', 'enabled', ?, ?, '{}')
	`, constant.QuestionTypeSingle, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed overview question: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO question_options (
			id, tenant_id, question_id, option_key, sort_order, content, is_correct, is_distractor,
			created_at, updated_at, ext_json
		) VALUES
			(1601, 10, 1501, 'A', 1, '正确选项', TRUE, FALSE, ?, ?, '{}'),
			(1602, 10, 1501, 'B', 2, '干扰项', FALSE, TRUE, ?, ?, '{}')
	`, fixedAPINow, fixedAPINow, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed overview options: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO paper_section_questions (
			id, tenant_id, section_id, paper_id, question_id, sort_order, score, created_at, updated_at, ext_json
		) VALUES (1701, 10, 501, 100, 1501, 1, 2, ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed overview paper question: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO exam_attempts (
			id, tenant_id, exam_id, user_id, attempt_no, status, started_at, submitted_at,
			exam_token_hash, exam_token_expires_at, objective_score, subjective_score, total_score,
			created_at, updated_at, version, ext_json
		) VALUES (1801, 10, 1, 21, 1, 'submitted', ?, ?, 'overview-token', ?, 2, 0, 2, ?, ?, 1, '{}')
	`, fixedAPINow-3_600_000, fixedAPINow, fixedAPINow+3_600_000, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed overview attempt: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO exam_attempt_questions (
			id, tenant_id, attempt_id, section_id, question_id, section_snapshot, sort_order, score,
			question_snapshot, option_snapshot, correct_answer_snapshot, created_at, updated_at, ext_json
		) VALUES (1901, 10, 1801, 501, 1501, '{"name":"一、单项选择题"}', 1, 2, '{"title":"服务端预览题干","type":"single"}', '[]', '{"correct":"A"}', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed overview attempt questions: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO exam_answers (
			id, tenant_id, attempt_id, attempt_question_id, answer_content, score, grading_status,
			created_at, updated_at, ext_json
		) VALUES (2001, 10, 1801, 1901, 'A', 2, ?, ?, ?, '{}')
	`, constant.GradingStatusAuto, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed overview answers: %v", err)
	}
}

func seedExamManagementLowScoreResultData(t *testing.T, gormDB *gorm.DB) {
	t.Helper()
	if err := gormDB.Exec(`
		INSERT INTO users (
			id, username, real_name, phone, email, password_hash, status,
			created_at, updated_at, ext_json
		) VALUES (23, 'student_low', '低分考生', '13800000023', 'student.low@example.test', 'hash', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed low score student: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO tenant_user_memberships (
			id, tenant_id, user_id, role, status, created_at, updated_at, ext_json
		) VALUES (23, 10, 23, 'student', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed low score student role: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO space_members (
			id, tenant_id, space_id, user_id, role_in_space, status, created_at, updated_at, ext_json
		) VALUES (23, 10, 100, 23, 'student', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed low score student space member: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO exam_attempts (
			id, tenant_id, exam_id, user_id, attempt_no, status, started_at, submitted_at,
			exam_token_hash, exam_token_expires_at, objective_score, subjective_score, total_score,
			created_at, updated_at, version, ext_json
		) VALUES (1803, 10, 1, 23, 1, 'submitted', ?, ?, 'overview-low-token', ?, 0, 0, 0, ?, ?, 1, '{}')
	`, fixedAPINow-3_600_000, fixedAPINow-2_000, fixedAPINow+3_600_000, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed low-score overview attempt: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO exam_attempt_questions (
			id, tenant_id, attempt_id, section_id, question_id, section_snapshot, sort_order, score,
			question_snapshot, option_snapshot, correct_answer_snapshot, created_at, updated_at, ext_json
		) VALUES (1902, 10, 1803, 501, 1501, '{"name":"一、单项选择题"}', 1, 2, '{"title":"服务端预览题干","type":"single"}', '[]', '{"correct":"A"}', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed low-score overview attempt question: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO exam_answers (
			id, tenant_id, attempt_id, attempt_question_id, answer_content, score, grading_status,
			created_at, updated_at, ext_json
		) VALUES (2002, 10, 1803, 1902, 'B', 0, ?, ?, ?, '{}')
	`, constant.GradingStatusAuto, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed low-score overview answer: %v", err)
	}
}

func seedExamManagementResultRankingData(t *testing.T, gormDB *gorm.DB) {
	t.Helper()
	if err := gormDB.Exec(`
		INSERT INTO users (
			id, username, real_name, phone, email, password_hash, status,
			created_at, updated_at, ext_json
		) VALUES (31, 'student31', '高分考生', '13800000031', 'student31@example.test', 'hash', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed result ranking user: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO tenant_user_memberships (
			id, tenant_id, user_id, role, status, created_at, updated_at, ext_json
		) VALUES (31, 10, 31, 'student', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed result ranking tenant membership: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO space_members (
			id, tenant_id, space_id, user_id, role_in_space, status,
			created_at, updated_at, ext_json
		) VALUES (31, 10, 100, 31, 'student', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed result ranking space member: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO exam_attempts (
			id, tenant_id, exam_id, user_id, attempt_no, status, started_at, submitted_at,
			exam_token_hash, exam_token_expires_at, objective_score, subjective_score, total_score,
			created_at, updated_at, version, ext_json
		) VALUES (1802, 10, 1, 31, 1, 'submitted', ?, ?, 'ranking-token', ?, 3, 0, 3, ?, ?, 1, '{}')
	`, fixedAPINow-3_600_000, fixedAPINow-1_000, fixedAPINow+3_600_000, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed result ranking attempt: %v", err)
	}
}

func seedImportCandidateAPITestData(t *testing.T, gormDB *gorm.DB) {
	t.Helper()
	if err := gormDB.Exec(`
		INSERT INTO users (
			id, username, real_name, phone, email, password_hash, status,
			created_at, updated_at, ext_json
		) VALUES
			(31, 'student31', '可导入考生', '13800000031', 'student31@example.test', 'hash', 'enabled', ?, ?, '{}'),
			(32, 'student32', '非授权空间考生', '13800000032', 'student32@example.test', 'hash', 'enabled', ?, ?, '{}'),
			(33, 'student33', '禁用考生', '13800000033', 'student33@example.test', 'hash', 'disabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow, fixedAPINow, fixedAPINow, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed import users: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO tenant_user_memberships (
			id, tenant_id, user_id, role, status, created_at, updated_at, ext_json
		) VALUES
			(31, 10, 31, 'student', 'enabled', ?, ?, '{}'),
			(32, 10, 32, 'student', 'enabled', ?, ?, '{}'),
			(33, 10, 33, 'student', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow, fixedAPINow, fixedAPINow, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed import tenant memberships: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO spaces (
			id, tenant_id, name, logo_url, description, type, status,
			created_at, updated_at, ext_json
		) VALUES (200, 10, '未授权班级', '', '', 'class', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed import unauthorized space: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO space_members (
			id, tenant_id, space_id, user_id, role_in_space, status, created_at, updated_at, ext_json
		) VALUES
			(310, 10, 100, 31, 'student', 'enabled', ?, ?, '{}'),
			(320, 10, 200, 32, 'student', 'enabled', ?, ?, '{}'),
			(330, 10, 100, 33, 'student', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow, fixedAPINow, fixedAPINow, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed import space memberships: %v", err)
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

func seedPublicPublishPaperAPITestData(t *testing.T, gormDB *gorm.DB, paperID uint64) {
	t.Helper()
	if err := gormDB.Exec(`
		INSERT INTO papers (
			id, tenant_id, space_id, name, description, total_score, build_mode, status,
			created_at, created_by, created_by_type, updated_at, updated_by, updated_by_type, ext_json
		) VALUES (?, 10, NULL, '租户公共发布试卷', '', 100, ?, 'enabled', ?, 501, 'tenant_user', ?, 501, 'tenant_user', '{}')
	`, paperID, constant.BuildModeManual, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed public publish paper: %v", err)
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

func seedExamOperationLogAPITestData(t *testing.T, gormDB *gorm.DB) {
	t.Helper()
	if err := gormDB.Exec(`
		INSERT INTO exam_operation_logs (
			id, tenant_id, exam_id, operation_type, operation_title, operation_detail,
			actor_id, actor_type, actor_role, space_id, created_at, created_by, created_by_type, ext_json
		) VALUES
			(3001, 10, 1, ?, '导入考生', '导入 1 名考生', 99, 'tenant_user', 'tenant_admin', NULL, ?, 99, 'tenant_user', '{}'),
			(3002, 10, 1, ?, '重发邀请码', '重发 1 名考生邀请码', 20, 'tenant_user', 'space_admin', 100, ?, 20, 'tenant_user', '{"operation_group_id":"seed-group-1"}'),
			(3003, 10, 1, ?, '修改考试设置', '修改成绩发布配置', 99, 'tenant_user', 'tenant_admin', 200, ?, 99, 'tenant_user', '{}')
	`, serviceexam.OperationTypeImportCandidates, fixedAPINow+1_000, serviceexam.OperationTypeSendInvite, fixedAPINow+2_000, serviceexam.OperationTypeUpdateSettings, fixedAPINow+3_000).Error; err != nil {
		t.Fatalf("seed operation logs: %v", err)
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

func seedHighestStrategyVisibleResultAPITestData(t *testing.T, gormDB *gorm.DB) {
	t.Helper()
	if err := gormDB.Exec(`
		UPDATE exams
		SET max_attempts = 2, result_strategy = ?
		WHERE tenant_id = 10 AND id = 1
	`, serviceexam.ResultStrategyHighest).Error; err != nil {
		t.Fatalf("set highest result strategy: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO exam_attempts (
			id, tenant_id, exam_id, user_id, attempt_no, status, started_at, submitted_at,
			exam_token_hash, exam_token_expires_at, objective_score, subjective_score, total_score,
			created_at, updated_at, version, ext_json
	) VALUES (701, 10, 1, 20, 2, 'submitted', ?, ?, 'hash-result-low', ?, 5, 0, 5, ?, ?, 1, '{}')
	`, fixedAPINow-1_800_000, fixedAPINow-30_000, fixedAPINow+3_600_000, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed lower visible result attempt: %v", err)
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

type sequenceCodeGenerator struct {
	codes []string
	index int
}

func (g *sequenceCodeGenerator) NextCode() (string, error) {
	if len(g.codes) == 0 {
		return "", nil
	}
	index := g.index
	if index >= len(g.codes) {
		index = len(g.codes) - 1
	}
	g.index++
	return g.codes[index], nil
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

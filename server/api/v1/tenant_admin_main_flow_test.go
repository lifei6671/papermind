package v1

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/papermind/server/library/constant"
)

func TestTenantAdminMainFlowCreatesSpaceAndAssignsTeacherWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedPlatformLoginAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:            gormDB,
		Now:           func() int64 { return fixedAPINow },
		CodeGenerator: fixedCodeGenerator{code: "PM-MAIN"},
	})
	platformHeader := platformAuthHeader(t, router)

	createTenantRecorder := httptest.NewRecorder()
	router.ServeHTTP(createTenantRecorder, authorizedRequest(http.MethodPost, "/api/v1/tenants", []byte(`{
		"name": "主链路学校",
		"logo_url": "main.png",
		"description": "P8.3 主链路联调租户",
		"allow_register": false,
		"admin_username": "main.admin",
		"admin_real_name": "主链路管理员",
		"admin_phone": "13800009999",
		"admin_email": "main-admin@example.test",
		"admin_password": "main-admin-secure-123"
	}`), platformHeader))
	if createTenantRecorder.Code != http.StatusOK {
		t.Fatalf("create tenant status = %d, body = %s", createTenantRecorder.Code, createTenantRecorder.Body.String())
	}
	createTenantBody := decodeExamAPIResponse[tenantResponse](t, createTenantRecorder.Body.Bytes())
	tenantID := createTenantBody.Data.ID

	var adminRow struct {
		ID     uint64 `gorm:"column:id"`
		Role   string `gorm:"column:role"`
		Status string `gorm:"column:status"`
	}
	if err := gormDB.Table("users").
		Select("users.id, users.status, tum.role").
		Joins("JOIN tenant_user_memberships AS tum ON tum.user_id = users.id").
		Where("tum.tenant_id = ? AND users.username = ?", tenantID, "main.admin").
		First(&adminRow).Error; err != nil {
		t.Fatalf("query first tenant admin: %v", err)
	}
	if adminRow.Role != "tenant_admin" || adminRow.Status != "enabled" {
		t.Fatalf("expected created tenant to initialize first admin, got %#v", adminRow)
	}

	adminHeader := tenantAuthHeader(t, router, tenantID, "main.admin", "main-admin-secure-123")
	createSpaceRecorder := httptest.NewRecorder()
	router.ServeHTTP(createSpaceRecorder, authorizedRequest(http.MethodPost, "/api/v1/spaces", []byte(fmt.Sprintf(`{
		"tenant_id": 999,
		"name": "主链路班级",
		"logo_url": "class-main.png",
		"description": "首个租户管理员创建的空间",
		"type": "class",
		"admin_user_ids": [%d]
	}`, adminRow.ID)), adminHeader))
	if createSpaceRecorder.Code != http.StatusOK {
		t.Fatalf("create space status = %d, body = %s", createSpaceRecorder.Code, createSpaceRecorder.Body.String())
	}
	createSpaceBody := decodeExamAPIResponse[spaceResponse](t, createSpaceRecorder.Body.Bytes())
	if createSpaceBody.Data.TenantID != tenantID || createSpaceBody.Data.Name != "主链路班级" {
		t.Fatalf("unexpected created space: %#v", createSpaceBody.Data)
	}

	createTeacherRecorder := httptest.NewRecorder()
	router.ServeHTTP(createTeacherRecorder, authorizedRequest(http.MethodPost, "/api/v1/users", []byte(`{
		"tenant_id": 999,
		"username": "main.teacher",
		"real_name": "主链路教师",
		"password": "main-teacher-secure-123",
		"role": "teacher"
	}`), adminHeader))
	if createTeacherRecorder.Code != http.StatusOK {
		t.Fatalf("create teacher status = %d, body = %s", createTeacherRecorder.Code, createTeacherRecorder.Body.String())
	}
	createTeacherBody := decodeExamAPIResponse[userResponse](t, createTeacherRecorder.Body.Bytes())
	if createTeacherBody.Data.TenantID != tenantID || createTeacherBody.Data.Role != "teacher" {
		t.Fatalf("unexpected created teacher: %#v", createTeacherBody.Data)
	}

	teacherHeader := tenantLoginAuthHeader(t, router, "main.teacher", "main-teacher-secure-123")
	beforeRecorder := httptest.NewRecorder()
	router.ServeHTTP(beforeRecorder, authorizedRequest(http.MethodGet, "/api/v1/profile/spaces", nil, teacherHeader))
	if beforeRecorder.Code != http.StatusOK {
		t.Fatalf("profile spaces before assignment status = %d, body = %s", beforeRecorder.Code, beforeRecorder.Body.String())
	}
	beforeBody := decodeExamAPIResponse[profileSpaceListResponse](t, beforeRecorder.Body.Bytes())
	if len(beforeBody.Data.Items) != 0 {
		t.Fatalf("expected teacher to start without space memberships, got %#v", beforeBody.Data.Items)
	}

	assignRecorder := httptest.NewRecorder()
	router.ServeHTTP(assignRecorder, authorizedRequest(http.MethodPost, "/api/v1/spaces/"+fmt.Sprint(createSpaceBody.Data.ID)+"/members", []byte(fmt.Sprintf(`{
		"tenant_id": 999,
		"user_id": %d,
		"role": "teacher"
	}`, createTeacherBody.Data.ID)), adminHeader))
	if assignRecorder.Code != http.StatusOK {
		t.Fatalf("assign teacher status = %d, body = %s", assignRecorder.Code, assignRecorder.Body.String())
	}

	afterRecorder := httptest.NewRecorder()
	teacherHeader = tenantAuthHeader(t, router, tenantID, "main.teacher", "main-teacher-secure-123")
	router.ServeHTTP(afterRecorder, authorizedRequest(http.MethodGet, "/api/v1/profile/spaces", nil, teacherHeader))
	if afterRecorder.Code != http.StatusOK {
		t.Fatalf("profile spaces after assignment status = %d, body = %s", afterRecorder.Code, afterRecorder.Body.String())
	}
	afterBody := decodeExamAPIResponse[profileSpaceListResponse](t, afterRecorder.Body.Bytes())
	if len(afterBody.Data.Items) != 1 {
		t.Fatalf("expected teacher to gain one profile space, got %#v", afterBody.Data.Items)
	}
	item := afterBody.Data.Items[0]
	if item.TenantID != tenantID || item.SpaceID != createSpaceBody.Data.ID || item.Role != "teacher" || item.Status != "enabled" {
		t.Fatalf("unexpected assigned profile space: %#v", item)
	}

	createPublicQuestionRecorder := httptest.NewRecorder()
	router.ServeHTTP(createPublicQuestionRecorder, authorizedRequest(http.MethodPost, "/api/v1/questions", []byte(fmt.Sprintf(`{
		"tenant_id": %d,
		"type": %q,
		"difficulty": "easy",
		"title": "主链路公共题",
		"analysis": "租户管理员维护公共题库。",
		"score_default": "2",
		"tags": ["公共题库"],
		"options": [
			{"option_key": "A", "content": "公共题正确选项", "is_correct": true},
			{"option_key": "B", "content": "公共题干扰选项", "is_distractor": true}
		]
	}`, tenantID, constant.QuestionTypeSingle)), adminHeader))
	if createPublicQuestionRecorder.Code != http.StatusOK {
		t.Fatalf("create public question status = %d, body = %s", createPublicQuestionRecorder.Code, createPublicQuestionRecorder.Body.String())
	}
	publicQuestionBody := decodeExamAPIResponse[questionResponse](t, createPublicQuestionRecorder.Body.Bytes())
	if publicQuestionBody.Data.SpaceID != nil {
		t.Fatalf("expected tenant admin public question without space scope, got %#v", publicQuestionBody.Data)
	}

	forbiddenPublicQuestionRecorder := httptest.NewRecorder()
	router.ServeHTTP(forbiddenPublicQuestionRecorder, authorizedRequest(http.MethodPost, "/api/v1/questions", []byte(fmt.Sprintf(`{
		"tenant_id": %d,
		"type": %q,
		"difficulty": "easy",
		"title": "教师越权公共题",
		"analysis": "教师不能写公共题库。",
		"score_default": "2",
		"options": [
			{"option_key": "A", "content": "正确", "is_correct": true},
			{"option_key": "B", "content": "错误", "is_distractor": true}
		]
	}`, tenantID, constant.QuestionTypeSingle)), teacherHeader))
	if forbiddenPublicQuestionRecorder.Code != http.StatusForbidden {
		t.Fatalf("expected teacher public question write to be forbidden, got status = %d, body = %s", forbiddenPublicQuestionRecorder.Code, forbiddenPublicQuestionRecorder.Body.String())
	}

	createSpaceQuestionRecorder := httptest.NewRecorder()
	router.ServeHTTP(createSpaceQuestionRecorder, authorizedRequest(http.MethodPost, "/api/v1/questions", []byte(fmt.Sprintf(`{
		"tenant_id": %d,
		"space_id": %d,
		"type": %q,
		"difficulty": "easy",
		"title": "主链路空间题",
		"analysis": "教师维护授权空间题库。",
		"score_default": "2",
		"tags": ["空间题库"],
		"options": [
			{"option_key": "A", "content": "空间题正确选项", "is_correct": true},
			{"option_key": "B", "content": "空间题干扰选项", "is_distractor": true}
		]
	}`, tenantID, createSpaceBody.Data.ID, constant.QuestionTypeSingle)), teacherHeader))
	if createSpaceQuestionRecorder.Code != http.StatusOK {
		t.Fatalf("create space question status = %d, body = %s", createSpaceQuestionRecorder.Code, createSpaceQuestionRecorder.Body.String())
	}
	spaceQuestionBody := decodeExamAPIResponse[questionResponse](t, createSpaceQuestionRecorder.Body.Bytes())
	if spaceQuestionBody.Data.SpaceID == nil || *spaceQuestionBody.Data.SpaceID != createSpaceBody.Data.ID {
		t.Fatalf("expected teacher question to stay in authorized space, got %#v", spaceQuestionBody.Data)
	}

	createShortTextQuestionRecorder := httptest.NewRecorder()
	router.ServeHTTP(createShortTextQuestionRecorder, authorizedRequest(http.MethodPost, "/api/v1/questions", []byte(fmt.Sprintf(`{
		"tenant_id": %d,
		"space_id": %d,
		"type": %q,
		"difficulty": "medium",
		"title": "主链路简答题",
		"analysis": "教师按授权空间阅卷。",
		"score_default": "8",
		"tags": ["空间题库", "人工阅卷"]
	}`, tenantID, createSpaceBody.Data.ID, constant.QuestionTypeShortText)), teacherHeader))
	if createShortTextQuestionRecorder.Code != http.StatusOK {
		t.Fatalf("create short text question status = %d, body = %s", createShortTextQuestionRecorder.Code, createShortTextQuestionRecorder.Body.String())
	}
	shortTextQuestionBody := decodeExamAPIResponse[questionResponse](t, createShortTextQuestionRecorder.Body.Bytes())
	if shortTextQuestionBody.Data.SpaceID == nil || *shortTextQuestionBody.Data.SpaceID != createSpaceBody.Data.ID {
		t.Fatalf("expected short text question to stay in authorized space, got %#v", shortTextQuestionBody.Data)
	}

	paperID := uint64(8801)
	if err := gormDB.Exec(`
		INSERT INTO papers (
			id, tenant_id, space_id, name, description, total_score, build_mode, status,
			created_at, updated_at, ext_json
		) VALUES (?, ?, ?, '主链路空间试卷', '', 0, ?, 'draft', ?, ?, '{}')
	`, paperID, tenantID, createSpaceBody.Data.ID, constant.BuildModeManual, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed main flow paper: %v", err)
	}

	createSectionRecorder := httptest.NewRecorder()
	router.ServeHTTP(createSectionRecorder, authorizedRequest(http.MethodPost, fmt.Sprintf("/api/v1/papers/%d/sections", paperID), []byte(fmt.Sprintf(`{
		"tenant_id": %d,
		"name": "一、主链路单选",
		"question_type": %q,
		"instructions": "完成主链路空间题"
	}`, tenantID, constant.QuestionTypeSingle)), teacherHeader))
	if createSectionRecorder.Code != http.StatusOK {
		t.Fatalf("create paper section status = %d, body = %s", createSectionRecorder.Code, createSectionRecorder.Body.String())
	}
	createSectionBody := decodeExamAPIResponse[paperSectionResponse](t, createSectionRecorder.Body.Bytes())

	addQuestionRecorder := httptest.NewRecorder()
	router.ServeHTTP(addQuestionRecorder, authorizedRequest(
		http.MethodPost,
		fmt.Sprintf("/api/v1/papers/%d/sections/%d/questions", paperID, createSectionBody.Data.ID),
		[]byte(fmt.Sprintf(`{
			"tenant_id": %d,
			"question_id": %d,
			"score": "2"
		}`, tenantID, spaceQuestionBody.Data.ID)),
		teacherHeader,
	))
	if addQuestionRecorder.Code != http.StatusOK {
		t.Fatalf("add paper question status = %d, body = %s", addQuestionRecorder.Code, addQuestionRecorder.Body.String())
	}

	createShortTextSectionRecorder := httptest.NewRecorder()
	router.ServeHTTP(createShortTextSectionRecorder, authorizedRequest(http.MethodPost, fmt.Sprintf("/api/v1/papers/%d/sections", paperID), []byte(fmt.Sprintf(`{
		"tenant_id": %d,
		"name": "二、主链路简答",
		"question_type": %q,
		"instructions": "完成主链路人工阅卷题"
	}`, tenantID, constant.QuestionTypeShortText)), teacherHeader))
	if createShortTextSectionRecorder.Code != http.StatusOK {
		t.Fatalf("create short text section status = %d, body = %s", createShortTextSectionRecorder.Code, createShortTextSectionRecorder.Body.String())
	}
	createShortTextSectionBody := decodeExamAPIResponse[paperSectionResponse](t, createShortTextSectionRecorder.Body.Bytes())

	addShortTextQuestionRecorder := httptest.NewRecorder()
	router.ServeHTTP(addShortTextQuestionRecorder, authorizedRequest(
		http.MethodPost,
		fmt.Sprintf("/api/v1/papers/%d/sections/%d/questions", paperID, createShortTextSectionBody.Data.ID),
		[]byte(fmt.Sprintf(`{
			"tenant_id": %d,
			"question_id": %d,
			"score": "8"
		}`, tenantID, shortTextQuestionBody.Data.ID)),
		teacherHeader,
	))
	if addShortTextQuestionRecorder.Code != http.StatusOK {
		t.Fatalf("add short text paper question status = %d, body = %s", addShortTextQuestionRecorder.Code, addShortTextQuestionRecorder.Body.String())
	}

	publishRecorder := httptest.NewRecorder()
	router.ServeHTTP(publishRecorder, authorizedRequest(http.MethodPost, "/api/v1/exams", []byte(fmt.Sprintf(`{
		"tenant_id": %d,
		"paper_id": %d,
		"name": "主链路空间考试",
		"target_type": "space",
		"target_id": %d,
		"start_time": %d,
		"end_time": %d,
		"duration_minutes": 60,
		"max_attempts": 1,
		"result_strategy": "latest",
		"publish_mode": "manual_publish"
	}`, tenantID, paperID, createSpaceBody.Data.ID, fixedAPINow-60_000, fixedAPINow+120*60_000)), teacherHeader))
	if publishRecorder.Code != http.StatusOK {
		t.Fatalf("publish exam status = %d, body = %s", publishRecorder.Code, publishRecorder.Body.String())
	}
	publishBody := decodeExamAPIResponse[examResponse](t, publishRecorder.Body.Bytes())
	if publishBody.Data.Status != "published" || publishBody.Data.TargetID != createSpaceBody.Data.ID {
		t.Fatalf("unexpected published exam: %#v", publishBody.Data)
	}

	createStudentRecorder := httptest.NewRecorder()
	router.ServeHTTP(createStudentRecorder, authorizedRequest(http.MethodPost, "/api/v1/users", []byte(`{
		"tenant_id": 999,
		"username": "main.student",
		"real_name": "主链路学生",
		"password": "main-student-secure-123",
		"role": "student"
	}`), adminHeader))
	if createStudentRecorder.Code != http.StatusOK {
		t.Fatalf("create student status = %d, body = %s", createStudentRecorder.Code, createStudentRecorder.Body.String())
	}
	createStudentBody := decodeExamAPIResponse[userResponse](t, createStudentRecorder.Body.Bytes())

	assignStudentRecorder := httptest.NewRecorder()
	router.ServeHTTP(assignStudentRecorder, authorizedRequest(http.MethodPost, "/api/v1/spaces/"+fmt.Sprint(createSpaceBody.Data.ID)+"/members", []byte(fmt.Sprintf(`{
		"tenant_id": 999,
		"user_id": %d,
		"role": "student"
	}`, createStudentBody.Data.ID)), adminHeader))
	if assignStudentRecorder.Code != http.StatusOK {
		t.Fatalf("assign student status = %d, body = %s", assignStudentRecorder.Code, assignStudentRecorder.Body.String())
	}

	studentHeader := tenantAuthHeader(t, router, tenantID, "main.student", "main-student-secure-123")
	resolveRecorder := httptest.NewRecorder()
	router.ServeHTTP(resolveRecorder, authorizedRequest(http.MethodPost, "/api/v1/exam-entry/invite/resolve", []byte(fmt.Sprintf(`{
		"invite_code": %q
	}`, publishBody.Data.InviteCode)), studentHeader))
	if resolveRecorder.Code != http.StatusOK {
		t.Fatalf("resolve invite status = %d, body = %s", resolveRecorder.Code, resolveRecorder.Body.String())
	}

	startRecorder := httptest.NewRecorder()
	startRequest := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/exam-entry/exams/%d/attempts/start", publishBody.Data.ID), bytes.NewReader([]byte(fmt.Sprintf(`{
		"tenant_id": %d
	}`, tenantID))))
	for _, cookie := range resolveRecorder.Result().Cookies() {
		startRequest.AddCookie(cookie)
	}
	router.ServeHTTP(startRecorder, startRequest)
	if startRecorder.Code != http.StatusOK {
		t.Fatalf("start exam status = %d, body = %s", startRecorder.Code, startRecorder.Body.String())
	}
	startBody := decodeExamAPIResponse[startAttemptResponse](t, startRecorder.Body.Bytes())
	if startBody.Data.ExamToken == "" || len(startBody.Data.Questions) != 2 {
		t.Fatalf("unexpected start response: %#v", startBody.Data)
	}
	var singleQuestion attemptQuestionResponse
	var shortTextQuestion attemptQuestionResponse
	for _, question := range startBody.Data.Questions {
		switch question.Question.Type {
		case constant.QuestionTypeSingle:
			singleQuestion = question
		case constant.QuestionTypeShortText:
			shortTextQuestion = question
		}
	}
	if len(singleQuestion.Options) == 0 || shortTextQuestion.ID == 0 {
		t.Fatalf("expected started single and short text questions, got %#v", startBody.Data.Questions)
	}

	saveSingleRecorder := httptest.NewRecorder()
	router.ServeHTTP(saveSingleRecorder, httptest.NewRequest(
		http.MethodPost,
		fmt.Sprintf("/api/v1/exam-entry/attempts/%d/answers/%d", startBody.Data.Attempt.ID, singleQuestion.ID),
		bytes.NewReader([]byte(fmt.Sprintf(`{
			"tenant_id": %d,
			"exam_token": %q,
			"question_type": %q,
			"option_ids": [%d]
		}`, tenantID, startBody.Data.ExamToken, constant.QuestionTypeSingle, singleQuestion.Options[0].ID))),
	))
	if saveSingleRecorder.Code != http.StatusOK {
		t.Fatalf("save single answer status = %d, body = %s", saveSingleRecorder.Code, saveSingleRecorder.Body.String())
	}

	saveShortTextRecorder := httptest.NewRecorder()
	router.ServeHTTP(saveShortTextRecorder, httptest.NewRequest(
		http.MethodPost,
		fmt.Sprintf("/api/v1/exam-entry/attempts/%d/answers/%d", startBody.Data.Attempt.ID, shortTextQuestion.ID),
		bytes.NewReader([]byte(fmt.Sprintf(`{
			"tenant_id": %d,
			"exam_token": %q,
			"question_type": %q,
			"text": "主链路学生简答"
		}`, tenantID, startBody.Data.ExamToken, constant.QuestionTypeShortText))),
	))
	if saveShortTextRecorder.Code != http.StatusOK {
		t.Fatalf("save short text answer status = %d, body = %s", saveShortTextRecorder.Code, saveShortTextRecorder.Body.String())
	}

	submitRecorder := httptest.NewRecorder()
	router.ServeHTTP(submitRecorder, httptest.NewRequest(
		http.MethodPost,
		fmt.Sprintf("/api/v1/exam-entry/attempts/%d/submit", startBody.Data.Attempt.ID),
		bytes.NewReader([]byte(fmt.Sprintf(`{
			"tenant_id": %d,
			"exam_token": %q
		}`, tenantID, startBody.Data.ExamToken))),
	))
	if submitRecorder.Code != http.StatusOK {
		t.Fatalf("submit exam status = %d, body = %s", submitRecorder.Code, submitRecorder.Body.String())
	}
	var submitted struct {
		Status         string `gorm:"column:status"`
		ObjectiveScore string `gorm:"column:objective_score"`
	}
	if err := gormDB.Table("exam_attempts").
		Select("status", "objective_score").
		Where("tenant_id = ? AND id = ?", tenantID, startBody.Data.Attempt.ID).
		Scan(&submitted).Error; err != nil {
		t.Fatalf("query submitted attempt: %v", err)
	}
	if submitted.Status != constant.AttemptStatusSubmitted || submitted.ObjectiveScore != "2" {
		t.Fatalf("unexpected submitted attempt: %#v", submitted)
	}

	listReviewRecorder := httptest.NewRecorder()
	router.ServeHTTP(listReviewRecorder, authorizedRequest(http.MethodGet, fmt.Sprintf(
		"/api/v1/grading/pending?tenant_id=%d&exam_id=%d&space_id=%d",
		tenantID,
		publishBody.Data.ID,
		createSpaceBody.Data.ID,
	), nil, teacherHeader))
	if listReviewRecorder.Code != http.StatusOK {
		t.Fatalf("list review status = %d, body = %s", listReviewRecorder.Code, listReviewRecorder.Body.String())
	}
	reviewBody := decodeExamAPIResponse[pendingReviewListResponse](t, listReviewRecorder.Body.Bytes())
	if len(reviewBody.Data.Items) != 1 {
		t.Fatalf("expected one pending short text row, got %#v", reviewBody.Data.Items)
	}
	pending := reviewBody.Data.Items[0]
	if pending.AttemptID != startBody.Data.Attempt.ID || pending.QuestionTitle != "主链路简答题" || pending.AnswerContent != "主链路学生简答" {
		t.Fatalf("unexpected pending review response: %#v", pending)
	}

	gradeRecorder := httptest.NewRecorder()
	router.ServeHTTP(gradeRecorder, authorizedRequest(http.MethodPost, fmt.Sprintf(
		"/api/v1/exam-attempts/%d/questions/%d/grade",
		pending.AttemptID,
		pending.AttemptQuestionID,
	), []byte(fmt.Sprintf(`{
		"tenant_id": %d,
		"exam_id": %d,
		"space_id": %d,
		"answer_version": %d,
		"score": "8",
		"comment": "主链路阅卷通过"
	}`, tenantID, publishBody.Data.ID, createSpaceBody.Data.ID, pending.AnswerVersion)), teacherHeader))
	if gradeRecorder.Code != http.StatusOK {
		t.Fatalf("grade status = %d, body = %s", gradeRecorder.Code, gradeRecorder.Body.String())
	}

	publishScoreTime := fixedAPINow - 1_000
	configRecorder := httptest.NewRecorder()
	router.ServeHTTP(configRecorder, authorizedRequest(http.MethodPost, "/api/v1/results/publish-config", []byte(fmt.Sprintf(`{
		"tenant_id": %d,
		"exam_id": %d,
		"space_id": %d,
		"publish_mode": "manual_publish",
		"score_publish_time": %d
	}`, tenantID, publishBody.Data.ID, createSpaceBody.Data.ID, publishScoreTime)), teacherHeader))
	if configRecorder.Code != http.StatusOK {
		t.Fatalf("publish config status = %d, body = %s", configRecorder.Code, configRecorder.Body.String())
	}

	studentResultRecorder := httptest.NewRecorder()
	router.ServeHTTP(studentResultRecorder, authorizedRequest(http.MethodGet, fmt.Sprintf(
		"/api/v1/exam-entry/results/%d",
		startBody.Data.Attempt.ID,
	), nil, studentHeader))
	if studentResultRecorder.Code != http.StatusOK {
		t.Fatalf("student result status = %d, body = %s", studentResultRecorder.Code, studentResultRecorder.Body.String())
	}
	studentResultBody := decodeExamAPIResponse[visibleResultResponse](t, studentResultRecorder.Body.Bytes())
	if studentResultBody.Data.TotalScore != "10" || studentResultBody.Data.ObjectiveScore != "2" || studentResultBody.Data.SubjectiveScore != "8" {
		t.Fatalf("unexpected student result response: %#v", studentResultBody.Data)
	}

	listResultsRecorder := httptest.NewRecorder()
	router.ServeHTTP(listResultsRecorder, authorizedRequest(http.MethodGet, fmt.Sprintf(
		"/api/v1/results?tenant_id=%d&exam_id=%d&space_id=%d",
		tenantID,
		publishBody.Data.ID,
		createSpaceBody.Data.ID,
	), nil, teacherHeader))
	if listResultsRecorder.Code != http.StatusOK {
		t.Fatalf("list results status = %d, body = %s", listResultsRecorder.Code, listResultsRecorder.Body.String())
	}
	resultsBody := decodeExamAPIResponse[resultListResponse](t, listResultsRecorder.Body.Bytes())
	if len(resultsBody.Data.Items) != 1 || resultsBody.Data.Items[0].StudentName != "主链路学生" || resultsBody.Data.Items[0].TotalScore != "10" {
		t.Fatalf("unexpected result list response: %#v", resultsBody.Data.Items)
	}

	exportRecorder := httptest.NewRecorder()
	router.ServeHTTP(exportRecorder, authorizedRequest(http.MethodPost, "/api/v1/results/export", []byte(fmt.Sprintf(`{
		"tenant_id": %d,
		"exam_id": %d,
		"space_id": %d
	}`, tenantID, publishBody.Data.ID, createSpaceBody.Data.ID)), teacherHeader))
	if exportRecorder.Code != http.StatusForbidden {
		t.Fatalf("teacher export status = %d, body = %s", exportRecorder.Code, exportRecorder.Body.String())
	}
}

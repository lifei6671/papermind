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
	seedSpaceAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, authorizedRequest(http.MethodGet, "/api/v1/questions?tenant_id=10", nil, authHeader))
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
	router.ServeHTTP(createRecorder, authorizedRequest(http.MethodPost, "/api/v1/questions", payload, authHeader))
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
	seedSpaceAPITestData(t, gormDB)
	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

	requestBody, contentType := buildQuestionImportMultipart(t, fmt.Sprintf(`type,title,options,correct_answer,analysis,difficulty,tags
%s,函数单调性判断,A.y = x|B.y = -x,A,一次函数斜率为正时单调递增。,medium,函数
%s,无正确选项,A.正确|B.错误,,缺少正确答案。,medium,基础
`, constant.QuestionTypeSingle, constant.QuestionTypeSingle))
	recorder := httptest.NewRecorder()
	request := authorizedRequest(http.MethodPost, "/api/v1/questions/import", requestBody.Bytes(), authHeader)
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

func TestTeacherCannotCreateQuestionInInactiveSpace(t *testing.T) {
	for _, tc := range []struct {
		name   string
		update string
	}{
		{name: "disabled space", update: "status = 'disabled'"},
		{name: "deleted space", update: "deleted_at = 1700000000000"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			gormDB := openExamAPITestDB(t)
			seedSpaceAPITestData(t, gormDB)
			seedProfileSpacesAPITestData(t, gormDB)

			router := NewRouter(RouterOptions{
				DB:  gormDB,
				Now: func() int64 { return fixedAPINow },
			})
			authHeader := tenantAuthHeader(t, router, 10, "teacher_li", "papermind123")
			if err := gormDB.Exec("UPDATE spaces SET "+tc.update+" WHERE tenant_id = ? AND id = ?", 10, 100).Error; err != nil {
				t.Fatalf("mark space inactive: %v", err)
			}
			payload := []byte(fmt.Sprintf(`{
				"tenant_id": 10,
				"space_id": 100,
				"type": "%s",
				"difficulty": "medium",
				"title": "空间失效后不可写题",
				"analysis": "空间不可用时拒绝题库写入。",
				"score_default": "2",
				"tags": ["权限"],
				"options": [
					{"option_key": "A", "content": "拒绝", "is_correct": true},
					{"option_key": "B", "content": "允许", "is_distractor": true}
				]
			}`, constant.QuestionTypeSingle))

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, authorizedRequest(http.MethodPost, "/api/v1/questions", payload, authHeader))
			if recorder.Code != http.StatusForbidden {
				t.Fatalf("expected inactive space create to be forbidden, got status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestTenantAdminCannotCreateQuestionForMissingSpace(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedSpaceAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")
	payload := []byte(fmt.Sprintf(`{
		"tenant_id": 10,
		"space_id": 404,
		"type": "%s",
		"difficulty": "medium",
		"title": "无效空间题目",
		"analysis": "空间不存在时拒绝题库写入。",
		"score_default": "2",
		"tags": ["权限"],
		"options": [
			{"option_key": "A", "content": "拒绝", "is_correct": true},
			{"option_key": "B", "content": "允许", "is_distractor": true}
		]
	}`, constant.QuestionTypeSingle))

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodPost, "/api/v1/questions", payload, authHeader))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected missing space question create to be rejected, got status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	var createdCount int64
	if err := gormDB.Table("questions").
		Where("tenant_id = ? AND title = ?", 10, "无效空间题目").
		Count(&createdCount).Error; err != nil {
		t.Fatalf("count invalid-space question: %v", err)
	}
	if createdCount != 0 {
		t.Fatalf("expected invalid-space question not to persist, got %d", createdCount)
	}
}

func TestCreateQuestionRejectsUnsupportedTypeAndDifficultyWithBadRequest(t *testing.T) {
	for _, tc := range []struct {
		name       string
		title      string
		questionTy string
		difficulty string
	}{
		{name: "unsupported type", title: "未知题型", questionTy: "essay", difficulty: "medium"},
		{name: "unsupported difficulty", title: "未知难度", questionTy: constant.QuestionTypeSingle, difficulty: "impossible"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			gormDB := openExamAPITestDB(t)
			seedSpaceAPITestData(t, gormDB)

			router := NewRouter(RouterOptions{
				DB:  gormDB,
				Now: func() int64 { return fixedAPINow },
			})
			authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")
			payload := []byte(fmt.Sprintf(`{
				"tenant_id": 10,
				"type": %q,
				"difficulty": %q,
				"title": %q,
				"analysis": "非法题目输入应返回参数错误。",
				"score_default": "2",
				"tags": ["参数"],
				"options": [
					{"option_key": "A", "content": "选项 A", "is_correct": true},
					{"option_key": "B", "content": "选项 B", "is_distractor": true}
				]
			}`, tc.questionTy, tc.difficulty, tc.title))

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, authorizedRequest(http.MethodPost, "/api/v1/questions", payload, authHeader))
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("expected unsupported question input to be bad request, got status = %d, body = %s", recorder.Code, recorder.Body.String())
			}

			var createdCount int64
			if err := gormDB.Table("questions").
				Where("tenant_id = ? AND title = ?", 10, tc.title).
				Count(&createdCount).Error; err != nil {
				t.Fatalf("count invalid question: %v", err)
			}
			if createdCount != 0 {
				t.Fatalf("expected invalid question not to persist, got %d", createdCount)
			}
		})
	}
}

func TestDisabledTeacherCannotCreateQuestionWithStaleSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedSpaceAPITestData(t, gormDB)
	seedProfileSpacesAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "teacher_li", "papermind123")
	if err := gormDB.Table("tenant_user_memberships").
		Where("tenant_id = ? AND user_id = ?", 10, 20).
		Update("status", "disabled").Error; err != nil {
		t.Fatalf("disable teacher after login: %v", err)
	}
	payload := []byte(fmt.Sprintf(`{
		"tenant_id": 10,
		"space_id": 100,
		"type": "%s",
		"difficulty": "medium",
		"title": "禁用用户旧会话不可写题",
		"analysis": "用户禁用后空间授权立即失效。",
		"score_default": "2",
		"tags": ["权限"],
		"options": [
			{"option_key": "A", "content": "拒绝", "is_correct": true},
			{"option_key": "B", "content": "允许", "is_distractor": true}
		]
	}`, constant.QuestionTypeSingle))

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodPost, "/api/v1/questions", payload, authHeader))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected disabled user stale session to be forbidden, got status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestDisabledTenantAdminCannotCreateQuestionWithStaleSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedSpaceAPITestData(t, gormDB)

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
	payload := []byte(fmt.Sprintf(`{
		"tenant_id": 10,
		"type": "%s",
		"difficulty": "medium",
		"title": "禁用管理员旧会话不可写题",
		"analysis": "租户管理员禁用后公共题库授权立即失效。",
		"score_default": "2",
		"tags": ["权限"],
		"options": [
			{"option_key": "A", "content": "拒绝", "is_correct": true},
			{"option_key": "B", "content": "允许", "is_distractor": true}
		]
	}`, constant.QuestionTypeSingle))

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodPost, "/api/v1/questions", payload, authHeader))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected disabled tenant admin stale session to be forbidden, got status = %d, body = %s", recorder.Code, recorder.Body.String())
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

package v1

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/papermind/server/library/crypto"
	"gorm.io/gorm"
)

func TestPlatformLoginAPIRouteSavesSessionAndAuditWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedPlatformLoginAPITestData(t, gormDB)
	seedTenantAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:             gormDB,
		Now:            func() int64 { return fixedAPINow },
		AuthSessionTTL: 7200,
	})

	payload := []byte(`{
		"username": "admin",
		"password": "papermind123"
	}`)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/platform/login", bytes.NewReader(payload))
	request.RemoteAddr = "203.0.113.10:4321"
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	body := decodeExamAPIResponse[authSessionResponse](t, recorder.Body.Bytes())
	if body.Data.AccessToken == "" || body.Data.RefreshToken != body.Data.AccessToken {
		t.Fatalf("unexpected token response: %#v", body.Data)
	}
	if body.Data.User.UserID != 1 || body.Data.User.DisplayName != "admin" || body.Data.User.Role != "platform_admin" {
		t.Fatalf("unexpected user response: %#v", body.Data.User)
	}
	if len(recorder.Result().Cookies()) == 0 {
		t.Fatalf("expected login to set session cookie")
	}
	if recorder.Result().Cookies()[0].Value != body.Data.AccessToken {
		t.Fatalf("access token should match session cookie value")
	}
	updateRecorder := httptest.NewRecorder()
	router.ServeHTTP(updateRecorder, authorizedRequest(
		http.MethodPost,
		"/api/v1/tenants/1/profile",
		[]byte(`{"name":"青藤实验中学","description":"登录 session 审计","logo_url":"qingteng.webp"}`),
		"Bearer "+body.Data.AccessToken,
	))
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("profile status = %d, body = %s", updateRecorder.Code, updateRecorder.Body.String())
	}
	assertTenantUpdatedBy(t, gormDB, 1, 1)

	var row struct {
		LastLoginIP string
		LastLoginAt int64
	}
	if err := gormDB.Table("platform_users").
		Select("last_login_ip, last_login_at").
		Where("id = ?", 1).
		Scan(&row).Error; err != nil {
		t.Fatalf("query login audit: %v", err)
	}
	if row.LastLoginIP != "203.0.113.10" || row.LastLoginAt != fixedAPINow {
		t.Fatalf("unexpected login audit: %#v", row)
	}
}

func TestPlatformLoginAPIRouteRejectsInvalidCredential(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedPlatformLoginAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})

	payload := []byte(`{
		"username": "admin",
		"password": "wrong-password"
	}`)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/auth/platform/login", bytes.NewReader(payload)))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("login status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !bytes.Contains(recorder.Body.Bytes(), []byte("账号或密码不正确")) {
		t.Fatalf("expected invalid credential message, body = %s", recorder.Body.String())
	}
}

func TestProfileAPIRoutesReadAndUpdatePlatformUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedPlatformLoginAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := platformAuthHeader(t, router)

	readRecorder := httptest.NewRecorder()
	router.ServeHTTP(readRecorder, authorizedRequest(http.MethodGet, "/api/v1/profile", nil, authHeader))
	if readRecorder.Code != http.StatusOK {
		t.Fatalf("read profile status = %d, body = %s", readRecorder.Code, readRecorder.Body.String())
	}
	readBody := decodeExamAPIResponse[profileResponse](t, readRecorder.Body.Bytes())
	if readBody.Data.DisplayName != "admin" || readBody.Data.Phone != "admin-phone" || readBody.Data.Email != "admin@example.test" {
		t.Fatalf("unexpected read profile response: %#v", readBody.Data)
	}

	updateRecorder := httptest.NewRecorder()
	router.ServeHTTP(updateRecorder, authorizedRequest(http.MethodPost, "/api/v1/profile", []byte(`{
		"display_name": "平台负责人",
		"avatar_url": "/uploads/avatars/admin.png",
		"phone": "13800000001",
		"email": "owner@example.test"
	}`), authHeader))
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("update profile status = %d, body = %s", updateRecorder.Code, updateRecorder.Body.String())
	}
	updateBody := decodeExamAPIResponse[profileResponse](t, updateRecorder.Body.Bytes())
	if updateBody.Data.DisplayName != "admin" || updateBody.Data.AvatarURL != "/uploads/avatars/admin.png" ||
		updateBody.Data.Phone != "13800000001" || updateBody.Data.Email != "owner@example.test" {
		t.Fatalf("unexpected updated profile response: %#v", updateBody.Data)
	}
}

func TestProfileAPIRoutesReadAndUpdateTenantUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedTenantRegisterAPITestData(t, gormDB)
	seedTakingAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "student20", "papermind123")

	updateRecorder := httptest.NewRecorder()
	router.ServeHTTP(updateRecorder, authorizedRequest(http.MethodPost, "/api/v1/profile", []byte(`{
		"display_name": "张同学更新",
		"avatar_url": "/uploads/avatars/student20.png",
		"phone": "13800002020",
		"email": "student20-new@example.test"
	}`), authHeader))
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("update tenant profile status = %d, body = %s", updateRecorder.Code, updateRecorder.Body.String())
	}
	body := decodeExamAPIResponse[profileResponse](t, updateRecorder.Body.Bytes())
	if body.Data.DisplayName != "张同学更新" || body.Data.TenantID != 10 || body.Data.Role != "student" ||
		body.Data.Phone != "13800002020" || body.Data.Email != "student20-new@example.test" {
		t.Fatalf("unexpected tenant profile response: %#v", body.Data)
	}
}

func TestTenantRegisterAPIRouteCreatesStudentByTenantCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedTenantRegisterAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})

	payload := []byte(`{
		"tenant_code": "PM-QT01",
		"username": "student01",
		"real_name": "张同学",
		"password": "papermind123",
		"phone": "13800002001",
		"email": "student01@example.test"
	}`)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/auth/tenant/register", bytes.NewReader(payload)))
	if recorder.Code != http.StatusOK {
		t.Fatalf("register status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	body := decodeExamAPIResponse[userResponse](t, recorder.Body.Bytes())
	if body.Data.TenantID != 10 || body.Data.Username != "student01" || body.Data.RealName != "张同学" {
		t.Fatalf("unexpected register response: %#v", body.Data)
	}
	if body.Data.Role != "student" || body.Data.Status != "enabled" {
		t.Fatalf("unexpected registered role/status: %#v", body.Data)
	}

	var row struct {
		PasswordHash string
		Role         string
	}
	if err := gormDB.Table("users").
		Select("users.password_hash, user_roles.role").
		Joins("JOIN user_roles ON user_roles.tenant_id = users.tenant_id AND user_roles.user_id = users.id").
		Where("users.tenant_id = ? AND users.username = ?", 10, "student01").
		Scan(&row).Error; err != nil {
		t.Fatalf("query registered user: %v", err)
	}
	if row.PasswordHash == "papermind123" || !crypto.VerifyPassword(row.PasswordHash, "papermind123") || row.Role != "student" {
		t.Fatalf("unexpected registered persistence: %#v", row)
	}
}

func TestTenantRegisterAPIRouteRejectsShortPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedTenantRegisterAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:                gormDB,
		Now:               func() int64 { return fixedAPINow },
		PasswordMinLength: 8,
	})

	payload := []byte(`{
		"tenant_code": "PM-QT01",
		"username": "student01",
		"real_name": "张同学",
		"password": "1234567"
	}`)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/auth/tenant/register", bytes.NewReader(payload)))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("register status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !bytes.Contains(recorder.Body.Bytes(), []byte("密码至少需要 8 位")) {
		t.Fatalf("expected password length message, body = %s", recorder.Body.String())
	}
}

func TestProtectedWriteAPIRoutesRejectAnonymousRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:            gormDB,
		Now:           func() int64 { return fixedAPINow },
		CodeGenerator: fixedCodeGenerator{code: "PM2027"},
	})

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
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/api/v1/exams", bytes.NewReader(payload)))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous publish status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestProtectedReadAPIRoutesRejectAnonymousRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedExamAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})

	cases := []struct {
		name   string
		method string
		target string
		body   []byte
	}{
		{name: "exams", method: http.MethodGet, target: "/api/v1/exams?tenant_id=10"},
		{name: "spaces", method: http.MethodGet, target: "/api/v1/spaces?tenant_id=10"},
		{name: "users", method: http.MethodGet, target: "/api/v1/users?tenant_id=10"},
		{name: "questions", method: http.MethodGet, target: "/api/v1/questions?tenant_id=10"},
		{name: "papers", method: http.MethodGet, target: "/api/v1/papers?tenant_id=10"},
		{name: "paper rules", method: http.MethodGet, target: "/api/v1/papers/100/rules?tenant_id=10"},
		{name: "paper sections", method: http.MethodGet, target: "/api/v1/papers/100/sections?tenant_id=10"},
		{name: "rule live precheck", method: http.MethodPost, target: "/api/v1/papers/100/rule-live/precheck", body: []byte(`{"tenant_id":10}`)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(tc.method, tc.target, bytes.NewReader(tc.body)))
			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("anonymous %s status = %d, body = %s", tc.name, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func seedPlatformLoginAPITestData(t *testing.T, gormDB *gorm.DB) {
	t.Helper()
	passwordHash, err := crypto.HashPassword("papermind123")
	if err != nil {
		t.Fatalf("hash platform password: %v", err)
	}

	if err := gormDB.Exec(`
		INSERT INTO platform_users (
			id, username, avatar_url, phone, email, password_hash, last_login_ip, last_login_at, status,
			created_at, updated_at, ext_json
		) VALUES (?, ?, '', ?, ?, ?, '', 0, 'enabled', ?, ?, '{}')
	`, 1, "admin", "admin-phone", "admin@example.test", passwordHash, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed platform user: %v", err)
	}
}

func seedTenantRegisterAPITestData(t *testing.T, gormDB *gorm.DB) {
	t.Helper()

	if err := gormDB.Exec(`
		INSERT INTO tenants (
			id, name, logo_url, description, tenant_code, allow_register, status,
			created_at, updated_at, version, ext_json
		) VALUES (?, ?, '', ?, ?, true, 'enabled', ?, ?, 1, '{}')
	`, 10, "青藤一中", "允许学生通过租户码自注册", "PM-QT01", fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed register tenant: %v", err)
	}
}

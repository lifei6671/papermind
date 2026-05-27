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

func TestUserAPIRoutesListCreateAndDisableWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedUserAPITestData(t, gormDB)
	seedPlatformLoginAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := platformAuthHeader(t, router)

	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, authorizedRequest(http.MethodGet, "/api/v1/users?tenant_id=10&page=1&page_size=1", nil, authHeader))
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listRecorder.Code, listRecorder.Body.String())
	}
	listBody := decodeExamAPIResponse[userListResponse](t, listRecorder.Body.Bytes())
	if len(listBody.Data.Items) != 1 || listBody.Data.Total != 3 || listBody.Data.PageSize != 1 {
		t.Fatalf("expected first page of three seeded users, got %#v", listBody.Data)
	}
	if listBody.Data.Items[0].RealName != "李老师" || listBody.Data.Items[0].Role != "teacher" {
		t.Fatalf("unexpected first user: %#v", listBody.Data.Items[0])
	}

	payload := []byte(`{
		"tenant_id": 10,
		"username": "wang.student",
		"real_name": "王同学",
		"avatar_url": "wang.png",
		"password": "wang-secure-123",
		"role": "student"
	}`)
	createRecorder := httptest.NewRecorder()
	router.ServeHTTP(createRecorder, authorizedRequest(http.MethodPost, "/api/v1/users", payload, authHeader))
	if createRecorder.Code != http.StatusOK {
		t.Fatalf("create status = %d, body = %s", createRecorder.Code, createRecorder.Body.String())
	}
	createBody := decodeExamAPIResponse[userResponse](t, createRecorder.Body.Bytes())
	if createBody.Data.RealName != "王同学" || createBody.Data.Role != "student" || createBody.Data.AvatarURL != "wang.png" {
		t.Fatalf("unexpected created user: %#v", createBody.Data)
	}

	temporaryLoginRecorder := httptest.NewRecorder()
	router.ServeHTTP(temporaryLoginRecorder, httptest.NewRequest(
		http.MethodPost,
		"/api/v1/auth/tenant/login",
		bytes.NewReader([]byte(`{"tenant_id":10,"username":"wang.student","password":"temporary-password"}`)),
	))
	if temporaryLoginRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("temporary password login status = %d, body = %s", temporaryLoginRecorder.Code, temporaryLoginRecorder.Body.String())
	}
	realPasswordLoginRecorder := httptest.NewRecorder()
	router.ServeHTTP(realPasswordLoginRecorder, httptest.NewRequest(
		http.MethodPost,
		"/api/v1/auth/tenant/login",
		bytes.NewReader([]byte(`{"tenant_id":10,"username":"wang.student","password":"wang-secure-123"}`)),
	))
	if realPasswordLoginRecorder.Code != http.StatusOK {
		t.Fatalf("created user login status = %d, body = %s", realPasswordLoginRecorder.Code, realPasswordLoginRecorder.Body.String())
	}

	disableRecorder := httptest.NewRecorder()
	router.ServeHTTP(disableRecorder, authorizedRequest(http.MethodPost, "/api/v1/users/20/disable", []byte(`{"tenant_id":10,"actor_id":99}`), authHeader))
	if disableRecorder.Code != http.StatusOK {
		t.Fatalf("disable status = %d, body = %s", disableRecorder.Code, disableRecorder.Body.String())
	}
	disableBody := decodeExamAPIResponse[userResponse](t, disableRecorder.Body.Bytes())
	if disableBody.Data.Status != "disabled" {
		t.Fatalf("expected disabled user, got %#v", disableBody.Data)
	}
}

func TestTenantStudentCannotCreateUsers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedUserAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "zhang.student", "papermind123")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodPost, "/api/v1/users", []byte(`{
		"tenant_id": 10,
		"username": "forbidden.student",
		"real_name": "越权学生",
		"avatar_url": "",
		"password": "student-secure-123",
		"role": "student"
	}`), authHeader))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden create for student, got status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestTenantAdminCanManageOnlyOwnTenantUsers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedUserAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

	ownTenantRecorder := httptest.NewRecorder()
	router.ServeHTTP(ownTenantRecorder, authorizedRequest(http.MethodGet, "/api/v1/users?tenant_id=10", nil, authHeader))
	if ownTenantRecorder.Code != http.StatusOK {
		t.Fatalf("own tenant list status = %d, body = %s", ownTenantRecorder.Code, ownTenantRecorder.Body.String())
	}

	otherTenantRecorder := httptest.NewRecorder()
	router.ServeHTTP(otherTenantRecorder, authorizedRequest(http.MethodGet, "/api/v1/users?tenant_id=20", nil, authHeader))
	if otherTenantRecorder.Code != http.StatusForbidden {
		t.Fatalf("other tenant list status = %d, body = %s", otherTenantRecorder.Code, otherTenantRecorder.Body.String())
	}
}

func TestCreateUserRejectsShortPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedUserAPITestData(t, gormDB)
	seedPlatformLoginAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:                gormDB,
		Now:               func() int64 { return fixedAPINow },
		PasswordMinLength: 8,
	})
	authHeader := platformAuthHeader(t, router)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodPost, "/api/v1/users", []byte(`{
		"tenant_id": 10,
		"username": "new.teacher",
		"real_name": "新老师",
		"password": "1234567",
		"role": "teacher"
	}`), authHeader))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("create user status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !bytes.Contains(recorder.Body.Bytes(), []byte("密码至少需要 8 位")) {
		t.Fatalf("expected password length message, body = %s", recorder.Body.String())
	}
}

func seedUserAPITestData(t *testing.T, gormDB *gorm.DB) {
	t.Helper()

	passwordHash, err := crypto.HashPassword("papermind123")
	if err != nil {
		t.Fatalf("hash tenant user password: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO users (
			id, tenant_id, username, real_name, avatar_url, phone, email, password_hash, status,
			created_at, updated_at, ext_json
		) VALUES
			(20, 10, 'li.teacher', '李老师', 'li.png', '13800001020', 'li-user@example.test', ?, 'enabled', ?, ?, '{}'),
			(21, 10, 'zhang.student', '张同学', 'zhang.png', '13800001021', 'zhang-user@example.test', ?, 'enabled', ?, ?, '{}'),
			(99, 10, 'tenant.admin', '租户管理员', 'admin.png', '13800001099', 'admin-user@example.test', ?, 'enabled', ?, ?, '{}')
	`, passwordHash, fixedAPINow, fixedAPINow, passwordHash, fixedAPINow, fixedAPINow, passwordHash, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed users: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO user_roles (
			id, tenant_id, user_id, role, created_at, updated_at, ext_json
		) VALUES
			(1, 10, 20, 'teacher', ?, ?, '{}'),
			(2, 10, 21, 'student', ?, ?, '{}'),
			(3, 10, 99, 'tenant_admin', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow, fixedAPINow, fixedAPINow, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed user roles: %v", err)
	}
}

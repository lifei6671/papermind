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

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

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

	deleteRecorder := httptest.NewRecorder()
	router.ServeHTTP(deleteRecorder, authorizedRequest(http.MethodDelete, "/api/v1/users/21?tenant_id=10", nil, authHeader))
	if deleteRecorder.Code != http.StatusOK {
		t.Fatalf("delete status = %d, body = %s", deleteRecorder.Code, deleteRecorder.Body.String())
	}
	var deletedAt int64
	if err := gormDB.Table("users").Select("deleted_at").Where("tenant_id = ? AND id = ?", 10, 21).Scan(&deletedAt).Error; err != nil {
		t.Fatalf("query deleted user: %v", err)
	}
	if deletedAt == 0 {
		t.Fatalf("expected deleted user to be soft deleted")
	}

	roleRecorder := httptest.NewRecorder()
	router.ServeHTTP(roleRecorder, authorizedRequest(http.MethodPut, "/api/v1/users/20/role", []byte(`{"tenant_id":10,"role":"student"}`), authHeader))
	if roleRecorder.Code != http.StatusOK {
		t.Fatalf("role update status = %d, body = %s", roleRecorder.Code, roleRecorder.Body.String())
	}
	roleBody := decodeExamAPIResponse[userResponse](t, roleRecorder.Body.Bytes())
	if roleBody.Data.Role != "student" || roleBody.Data.TenantID != 10 {
		t.Fatalf("expected updated role student in session tenant, got %#v", roleBody.Data)
	}

	importRecorder := httptest.NewRecorder()
	router.ServeHTTP(importRecorder, authorizedRequest(http.MethodPost, "/api/v1/users/import", []byte(`{
		"tenant_id": 10,
		"users": [
			{"username": "import.teacher", "real_name": "导入教师", "password": "import-teacher-123", "role": "teacher"},
			{"username": "import.student", "real_name": "导入学生", "password": "import-student-123", "role": "student"}
		]
	}`), authHeader))
	if importRecorder.Code != http.StatusOK {
		t.Fatalf("import users status = %d, body = %s", importRecorder.Code, importRecorder.Body.String())
	}
	importBody := decodeExamAPIResponse[importUsersResponse](t, importRecorder.Body.Bytes())
	if importBody.Data.SuccessCount != 2 {
		t.Fatalf("expected import success count 2, got %#v", importBody.Data)
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

func TestPlatformUserCannotEnterTenantUserRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedPlatformLoginAPITestData(t, gormDB)
	seedUserAPITestData(t, gormDB)

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
		{name: "list users", method: http.MethodGet, target: "/api/v1/users?tenant_id=10"},
		{name: "create user", method: http.MethodPost, target: "/api/v1/users", body: []byte(`{
			"tenant_id": 10,
			"username": "platform.created",
			"real_name": "平台越权创建",
			"password": "platform-secure-123",
			"role": "student"
		}`)},
		{name: "disable user", method: http.MethodPost, target: "/api/v1/users/20/disable", body: []byte(`{"tenant_id":10}`)},
		{name: "delete user", method: http.MethodDelete, target: "/api/v1/users/20?tenant_id=10"},
		{name: "update role", method: http.MethodPut, target: "/api/v1/users/20/role", body: []byte(`{"tenant_id":10,"role":"student"}`)},
		{name: "import users", method: http.MethodPost, target: "/api/v1/users/import", body: []byte(`{"tenant_id":10,"users":[{"username":"x","real_name":"x","password":"import-secure-123","role":"student"}]}`)},
		{name: "tenant-prefixed list users", method: http.MethodGet, target: "/api/v1/tenant/users?tenant_id=10"},
		{name: "tenant-prefixed create user", method: http.MethodPost, target: "/api/v1/tenant/users", body: []byte(`{
			"tenant_id": 10,
			"username": "platform.tenant.created",
			"real_name": "平台越权创建",
			"password": "platform-secure-123",
			"role": "student"
		}`)},
		{name: "tenant-prefixed disable user", method: http.MethodPost, target: "/api/v1/tenant/users/20/disable", body: []byte(`{"tenant_id":10}`)},
		{name: "tenant-prefixed delete user", method: http.MethodDelete, target: "/api/v1/tenant/users/20?tenant_id=10"},
		{name: "tenant-prefixed update role", method: http.MethodPut, target: "/api/v1/tenant/users/20/role", body: []byte(`{"tenant_id":10,"role":"student"}`)},
		{name: "tenant-prefixed import users", method: http.MethodPost, target: "/api/v1/tenant/users/import", body: []byte(`{"tenant_id":10,"users":[{"username":"tenant.x","real_name":"x","password":"import-secure-123","role":"student"}]}`)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, authorizedRequest(tc.method, tc.target, tc.body, authHeader))
			if recorder.Code != http.StatusForbidden {
				t.Fatalf("%s status = %d, body = %s", tc.name, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestCreateTeacherDoesNotAutoJoinSpaceMembers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedUserAPITestData(t, gormDB)
	if err := gormDB.Exec(`
		INSERT INTO spaces (
			id, tenant_id, name, logo_url, description, type, status,
			created_at, updated_at, ext_json
		) VALUES (301, 10, '高一一班', '', '', 'class', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed space: %v", err)
	}

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodPost, "/api/v1/users", []byte(`{
		"tenant_id": 10,
		"username": "new.teacher",
		"real_name": "新教师",
		"password": "new-teacher-secure-123",
		"role": "teacher"
	}`), authHeader))
	if recorder.Code != http.StatusOK {
		t.Fatalf("create teacher status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	createBody := decodeExamAPIResponse[userResponse](t, recorder.Body.Bytes())

	var memberCount int64
	if err := gormDB.Table("space_members").
		Where("tenant_id = ? AND user_id = ?", 10, createBody.Data.ID).
		Count(&memberCount).Error; err != nil {
		t.Fatalf("count created teacher space members: %v", err)
	}
	if memberCount != 0 {
		t.Fatalf("expected created teacher to have no space memberships, got %d", memberCount)
	}
}

func TestTenantAdminUserRoutesUseSessionTenant(t *testing.T) {
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
	if otherTenantRecorder.Code != http.StatusOK {
		t.Fatalf("other tenant list status = %d, body = %s", otherTenantRecorder.Code, otherTenantRecorder.Body.String())
	}
	otherTenantBody := decodeExamAPIResponse[userListResponse](t, otherTenantRecorder.Body.Bytes())
	if otherTenantBody.Data.Total != 3 || otherTenantBody.Data.Items[0].TenantID != 10 {
		t.Fatalf("expected list to use session tenant, got %#v", otherTenantBody.Data)
	}

	createRecorder := httptest.NewRecorder()
	router.ServeHTTP(createRecorder, authorizedRequest(http.MethodPost, "/api/v1/users", []byte(`{
		"tenant_id": 20,
		"username": "session.tenant.student",
		"real_name": "会话租户学生",
		"password": "session-tenant-123",
		"role": "student"
	}`), authHeader))
	if createRecorder.Code != http.StatusOK {
		t.Fatalf("create with forged tenant status = %d, body = %s", createRecorder.Code, createRecorder.Body.String())
	}
	createBody := decodeExamAPIResponse[userResponse](t, createRecorder.Body.Bytes())
	if createBody.Data.TenantID != 10 {
		t.Fatalf("expected created user to use session tenant, got %#v", createBody.Data)
	}

	disableRecorder := httptest.NewRecorder()
	router.ServeHTTP(disableRecorder, authorizedRequest(http.MethodPost, "/api/v1/users/20/disable", []byte(`{"tenant_id":20}`), authHeader))
	if disableRecorder.Code != http.StatusOK {
		t.Fatalf("disable with forged tenant status = %d, body = %s", disableRecorder.Code, disableRecorder.Body.String())
	}
	disableBody := decodeExamAPIResponse[userResponse](t, disableRecorder.Body.Bytes())
	if disableBody.Data.TenantID != 10 || disableBody.Data.Status != "disabled" {
		t.Fatalf("expected disable to use session tenant, got %#v", disableBody.Data)
	}

	roleRecorder := httptest.NewRecorder()
	router.ServeHTTP(roleRecorder, authorizedRequest(http.MethodPut, "/api/v1/users/21/role", []byte(`{"tenant_id":20,"role":"teacher"}`), authHeader))
	if roleRecorder.Code != http.StatusOK {
		t.Fatalf("role update with forged tenant status = %d, body = %s", roleRecorder.Code, roleRecorder.Body.String())
	}
	roleBody := decodeExamAPIResponse[userResponse](t, roleRecorder.Body.Bytes())
	if roleBody.Data.TenantID != 10 || roleBody.Data.Role != "teacher" {
		t.Fatalf("expected role update to use session tenant, got %#v", roleBody.Data)
	}

	importRecorder := httptest.NewRecorder()
	router.ServeHTTP(importRecorder, authorizedRequest(http.MethodPost, "/api/v1/users/import", []byte(`{
		"tenant_id": 20,
		"users": [{"username": "session.imported", "real_name": "会话导入", "password": "session-import-123", "role": "student"}]
	}`), authHeader))
	if importRecorder.Code != http.StatusOK {
		t.Fatalf("import with forged tenant status = %d, body = %s", importRecorder.Code, importRecorder.Body.String())
	}
	var importedTenantID uint64
	if err := gormDB.Table("users").Select("tenant_id").Where("username = ?", "session.imported").Scan(&importedTenantID).Error; err != nil {
		t.Fatalf("query imported tenant: %v", err)
	}
	if importedTenantID != 10 {
		t.Fatalf("expected import to use session tenant 10, got %d", importedTenantID)
	}

	tenantPrefixedRecorder := httptest.NewRecorder()
	router.ServeHTTP(tenantPrefixedRecorder, authorizedRequest(http.MethodPost, "/api/v1/tenant/users/import", []byte(`{
		"tenant_id": 20,
		"users": [{"username": "tenant.prefixed.imported", "real_name": "租户前缀导入", "password": "tenant-prefixed-123", "role": "student"}]
	}`), authHeader))
	if tenantPrefixedRecorder.Code != http.StatusOK {
		t.Fatalf("tenant-prefixed import with forged tenant status = %d, body = %s", tenantPrefixedRecorder.Code, tenantPrefixedRecorder.Body.String())
	}
	var tenantPrefixedTenantID uint64
	if err := gormDB.Table("users").Select("tenant_id").Where("username = ?", "tenant.prefixed.imported").Scan(&tenantPrefixedTenantID).Error; err != nil {
		t.Fatalf("query tenant-prefixed imported tenant: %v", err)
	}
	if tenantPrefixedTenantID != 10 {
		t.Fatalf("expected tenant-prefixed import to use session tenant 10, got %d", tenantPrefixedTenantID)
	}
}

func TestTenantAdminCannotDisableSelfByForgingActorID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedUserAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodPost, "/api/v1/users/99/disable", []byte(`{"tenant_id":10,"actor_id":20}`), authHeader))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected self-disable to be rejected, got status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestTenantAdminCannotDeleteSelf(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedUserAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodDelete, "/api/v1/users/99?tenant_id=10", nil, authHeader))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected self-delete to be rejected, got status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestTenantAdminDeleteMissingUserReturnsNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedUserAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodDelete, "/api/v1/tenant/users/404?tenant_id=10", nil, authHeader))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected missing user delete to return not found, got status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestTenantAdminCannotChangeOwnRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedUserAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodPut, "/api/v1/users/99/role", []byte(`{"tenant_id":10,"role":"teacher"}`), authHeader))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected self role change to be rejected, got status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestTenantAdminRoutesRejectStaleSessionAfterPrivilegeLoss(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate string
	}{
		{name: "role downgraded", mutate: "UPDATE user_roles SET role = 'teacher' WHERE tenant_id = 10 AND user_id = 99"},
		{name: "user disabled", mutate: "UPDATE users SET status = 'disabled' WHERE tenant_id = 10 AND id = 99"},
		{name: "user deleted", mutate: "UPDATE users SET deleted_at = 1700000000000 WHERE tenant_id = 10 AND id = 99"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			gormDB := openExamAPITestDB(t)
			seedUserAPITestData(t, gormDB)

			router := NewRouter(RouterOptions{
				DB:  gormDB,
				Now: func() int64 { return fixedAPINow },
			})
			authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")
			if err := gormDB.Exec(tc.mutate).Error; err != nil {
				t.Fatalf("mutate acting tenant admin: %v", err)
			}

			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, authorizedRequest(http.MethodPost, "/api/v1/tenant/users", []byte(`{
				"username": "stale.session.user",
				"real_name": "旧会话用户",
				"password": "stale-session-123",
				"role": "student"
			}`), authHeader))
			if recorder.Code != http.StatusForbidden {
				t.Fatalf("expected stale tenant admin session to be rejected, got status = %d, body = %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestCannotDisableLastTenantAdminWithStaleActorSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedUserAPITestData(t, gormDB)
	passwordHash, err := crypto.HashPassword("papermind123")
	if err != nil {
		t.Fatalf("hash second admin password: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO users (
			id, tenant_id, username, real_name, avatar_url, phone, email, password_hash, status,
			created_at, updated_at, ext_json
		) VALUES (100, 10, 'last.admin', '最后管理员', 'last.png', '13800001100', 'last-admin@example.test', ?, 'enabled', ?, ?, '{}')
	`, passwordHash, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed second admin user: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO user_roles (
			id, tenant_id, user_id, role, created_at, updated_at, ext_json
		) VALUES (4, 10, 100, 'tenant_admin', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed second admin role: %v", err)
	}

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")
	if err := gormDB.Table("users").
		Where("tenant_id = ? AND id = ?", 10, 99).
		Update("status", "disabled").Error; err != nil {
		t.Fatalf("disable acting admin after login: %v", err)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodPost, "/api/v1/users/100/disable", []byte(`{"tenant_id":10}`), authHeader))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected stale actor session to be rejected before disabling last tenant admin, got status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestCannotChangeLastTenantAdminRoleWithStaleActorSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedUserAPITestData(t, gormDB)
	passwordHash, err := crypto.HashPassword("papermind123")
	if err != nil {
		t.Fatalf("hash second admin password: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO users (
			id, tenant_id, username, real_name, avatar_url, phone, email, password_hash, status,
			created_at, updated_at, ext_json
		) VALUES (100, 10, 'last.admin', '最后管理员', 'last.png', '13800001100', 'last-admin@example.test', ?, 'enabled', ?, ?, '{}')
	`, passwordHash, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed second admin user: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO user_roles (
			id, tenant_id, user_id, role, created_at, updated_at, ext_json
		) VALUES (4, 10, 100, 'tenant_admin', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed second admin role: %v", err)
	}

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")
	if err := gormDB.Table("users").
		Where("tenant_id = ? AND id = ?", 10, 99).
		Update("status", "disabled").Error; err != nil {
		t.Fatalf("disable acting admin after login: %v", err)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodPut, "/api/v1/users/100/role", []byte(`{"tenant_id":10,"role":"teacher"}`), authHeader))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected stale actor session to be rejected before changing last tenant admin role, got status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestCannotImportUsersWhenActorSessionIsStale(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedUserAPITestData(t, gormDB)
	passwordHash, err := crypto.HashPassword("papermind123")
	if err != nil {
		t.Fatalf("hash second admin password: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO users (
			id, tenant_id, username, real_name, avatar_url, phone, email, password_hash, status,
			created_at, updated_at, ext_json
		) VALUES (100, 10, 'last.admin', '最后管理员', 'last.png', '13800001100', 'last-admin@example.test', ?, 'enabled', ?, ?, '{}')
	`, passwordHash, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed second admin user: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO user_roles (
			id, tenant_id, user_id, role, created_at, updated_at, ext_json
		) VALUES (4, 10, 100, 'tenant_admin', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed second admin role: %v", err)
	}

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")
	if err := gormDB.Table("users").
		Where("tenant_id = ? AND id = ?", 10, 99).
		Update("status", "disabled").Error; err != nil {
		t.Fatalf("disable acting admin after login: %v", err)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodPost, "/api/v1/users/import", []byte(`{
		"tenant_id": 10,
		"users": [
			{"username": "last.admin", "real_name": "最后管理员", "password": "last-admin-123", "role": "teacher"},
			{"username": "import.student", "real_name": "导入学生", "password": "import-student-123", "role": "student"}
		]
	}`), authHeader))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected stale actor session to be rejected before import role coverage, got status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestCreateUserRejectsShortPassword(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedUserAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:                gormDB,
		Now:               func() int64 { return fixedAPINow },
		PasswordMinLength: 8,
	})
	authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

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

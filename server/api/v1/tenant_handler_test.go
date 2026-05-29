package v1

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TestTenantAPIRoutesListAndCreateWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedPlatformLoginAPITestData(t, gormDB)
	seedTenantAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:                   gormDB,
		Now:                  func() int64 { return fixedAPINow },
		CodeGenerator:        fixedCodeGenerator{code: "PM-XH01"},
		AllowRegisterDefault: true,
	})
	authHeader := platformAuthHeader(t, router)

	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, authorizedRequest(http.MethodGet, "/api/v1/tenants", nil, authHeader))
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listRecorder.Code, listRecorder.Body.String())
	}
	listBody := decodeExamAPIResponse[tenantListResponse](t, listRecorder.Body.Bytes())
	if len(listBody.Data.Items) != 1 {
		t.Fatalf("expected one seeded tenant, got %#v", listBody.Data.Items)
	}
	if listBody.Data.Items[0].Name != "青藤一中" || listBody.Data.Items[0].TenantCode != "PM-QT01" {
		t.Fatalf("unexpected seeded tenant response: %#v", listBody.Data.Items[0])
	}

	payload := []byte(`{
		"name": "星海大学",
		"logo_url": "xinghai.png",
		"description": "面向公共课和企业培训的考试空间",
		"allow_register": false,
		"admin_username": "xinghai.admin",
		"admin_real_name": "星海管理员",
		"admin_phone": "13800001000",
		"admin_email": "admin@xinghai.example",
		"admin_password": "admin-secure-123"
	}`)
	createRecorder := httptest.NewRecorder()
	router.ServeHTTP(createRecorder, authorizedRequest(http.MethodPost, "/api/v1/tenants", payload, authHeader))
	if createRecorder.Code != http.StatusOK {
		t.Fatalf("create status = %d, body = %s", createRecorder.Code, createRecorder.Body.String())
	}
	createBody := decodeExamAPIResponse[tenantResponse](t, createRecorder.Body.Bytes())
	if createBody.Data.Name != "星海大学" || createBody.Data.TenantCode != "PM-XH01" {
		t.Fatalf("unexpected created tenant response: %#v", createBody.Data)
	}
	if createBody.Data.AllowRegister {
		t.Fatalf("expected allow_register overridden by request")
	}
	var createdAudit struct {
		CreatedBy uint64
		UpdatedBy uint64
	}
	if err := gormDB.Table("tenants").
		Select("created_by, updated_by").
		Where("id = ?", createBody.Data.ID).
		First(&createdAudit).Error; err != nil {
		t.Fatalf("query created tenant audit: %v", err)
	}
	if createdAudit.CreatedBy != 1 || createdAudit.UpdatedBy != 1 {
		t.Fatalf("expected created_by and updated_by to be login user 1, got %#v", createdAudit)
	}
	var adminRow struct {
		ID       uint64
		Username string
		RealName string
		Role     string
		Status   string
	}
	if err := gormDB.Table("users").
		Select("users.id, users.username, users.real_name, users.status, user_roles.role").
		Joins("JOIN user_roles ON user_roles.tenant_id = users.tenant_id AND user_roles.user_id = users.id").
		Where("users.tenant_id = ? AND users.username = ?", createBody.Data.ID, "xinghai.admin").
		First(&adminRow).Error; err != nil {
		t.Fatalf("query first tenant admin: %v", err)
	}
	if adminRow.RealName != "星海管理员" || adminRow.Role != "tenant_admin" || adminRow.Status != "enabled" {
		t.Fatalf("expected enabled first tenant admin role, got %#v", adminRow)
	}
	var memberCount int64
	if err := gormDB.Table("space_members").Where("tenant_id = ? AND user_id = ?", createBody.Data.ID, adminRow.ID).Count(&memberCount).Error; err != nil {
		t.Fatalf("query first tenant admin space member count: %v", err)
	}
	if memberCount != 0 {
		t.Fatalf("first tenant admin should not be added to any space, got %d memberships", memberCount)
	}
}

func TestTenantAPIListFiltersByKeywordWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedPlatformLoginAPITestData(t, gormDB)
	seedTenantAPITestData(t, gormDB)
	seedSecondTenantAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := platformAuthHeader(t, router)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodGet, "/api/v1/tenants?keyword=知行", nil, authHeader))
	if recorder.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	body := decodeExamAPIResponse[tenantListResponse](t, recorder.Body.Bytes())
	if body.Data.Total != 1 || len(body.Data.Items) != 1 {
		t.Fatalf("expected one filtered tenant, got total=%d items=%#v", body.Data.Total, body.Data.Items)
	}
	if body.Data.Items[0].Name != "知行培训" || body.Data.Items[0].TenantCode != "PM-ZX01" {
		t.Fatalf("unexpected filtered tenant: %#v", body.Data.Items[0])
	}
}

func TestTenantAPIListsTenantResourcesForPlatformOverviewWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedPlatformLoginAPITestData(t, gormDB)
	seedSpaceAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := platformAuthHeader(t, router)

	spacesRecorder := httptest.NewRecorder()
	router.ServeHTTP(spacesRecorder, authorizedRequest(http.MethodGet, "/api/v1/tenants/10/spaces", nil, authHeader))
	if spacesRecorder.Code != http.StatusOK {
		t.Fatalf("platform tenant spaces status = %d, body = %s", spacesRecorder.Code, spacesRecorder.Body.String())
	}
	spacesBody := decodeExamAPIResponse[spaceListResponse](t, spacesRecorder.Body.Bytes())
	if spacesBody.Data.Total != 1 || spacesBody.Data.Items[0].TenantID != 10 || spacesBody.Data.Items[0].Name != "高一 1 班" {
		t.Fatalf("unexpected platform tenant spaces response: %#v", spacesBody.Data)
	}

	usersRecorder := httptest.NewRecorder()
	router.ServeHTTP(usersRecorder, authorizedRequest(http.MethodGet, "/api/v1/tenants/10/users", nil, authHeader))
	if usersRecorder.Code != http.StatusOK {
		t.Fatalf("platform tenant users status = %d, body = %s", usersRecorder.Code, usersRecorder.Body.String())
	}
	usersBody := decodeExamAPIResponse[userListResponse](t, usersRecorder.Body.Bytes())
	if usersBody.Data.Total != 4 || usersBody.Data.Items[0].TenantID != 10 {
		t.Fatalf("unexpected platform tenant users response: %#v", usersBody.Data)
	}
}

func TestTenantManagementRoutesRejectAnonymousRequests(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedTenantAPITestData(t, gormDB)

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
		{name: "list", method: http.MethodGet, target: "/api/v1/tenants"},
		{name: "create", method: http.MethodPost, target: "/api/v1/tenants", body: []byte(`{"name":"星海大学"}`)},
		{name: "profile", method: http.MethodPost, target: "/api/v1/tenants/1/profile", body: []byte(`{"name":"青藤一中"}`)},
		{name: "reset code", method: http.MethodPost, target: "/api/v1/tenants/1/reset-code"},
		{name: "register setting", method: http.MethodPost, target: "/api/v1/tenants/1/register-setting", body: []byte(`{"allow_register":false}`)},
		{name: "tenant spaces", method: http.MethodGet, target: "/api/v1/tenants/1/spaces"},
		{name: "tenant users", method: http.MethodGet, target: "/api/v1/tenants/1/users"},
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

func TestTenantManagementRoutesRejectTenantUsers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedTenantAPITestData(t, gormDB)
	seedSpaceAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

	cases := []struct {
		name   string
		method string
		target string
		body   []byte
	}{
		{name: "list", method: http.MethodGet, target: "/api/v1/tenants"},
		{name: "create", method: http.MethodPost, target: "/api/v1/tenants", body: []byte(`{
			"name": "越权租户",
			"admin_username": "cross.admin",
			"admin_real_name": "越权管理员",
			"admin_password": "admin-secure-123"
		}`)},
		{name: "profile", method: http.MethodPost, target: "/api/v1/tenants/1/profile", body: []byte(`{"name":"越权修改"}`)},
		{name: "reset code", method: http.MethodPost, target: "/api/v1/tenants/1/reset-code"},
		{name: "register setting", method: http.MethodPost, target: "/api/v1/tenants/1/register-setting", body: []byte(`{"allow_register":false}`)},
		{name: "tenant spaces", method: http.MethodGet, target: "/api/v1/tenants/1/spaces"},
		{name: "tenant users", method: http.MethodGet, target: "/api/v1/tenants/1/users"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, authorizedRequest(tc.method, tc.target, tc.body, authHeader))
			if recorder.Code != http.StatusForbidden {
				t.Fatalf("tenant user %s status = %d, body = %s", tc.name, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestTenantAPIRoutesUpdateTenantOperationsWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedPlatformLoginAPITestData(t, gormDB)
	seedTenantAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:            gormDB,
		Now:           func() int64 { return fixedAPINow + 1000 },
		CodeGenerator: fixedCodeGenerator{code: "PM-QT02"},
	})
	authHeader := platformAuthHeader(t, router)

	descriptionRecorder := httptest.NewRecorder()
	router.ServeHTTP(descriptionRecorder, authorizedRequest(
		http.MethodPost,
		"/api/v1/tenants/1/profile",
		[]byte(`{"name":"青藤实验中学","description":"统一管理期中、期末和补测","logo_url":"/uploads/tenant-logos/qingteng.webp"}`),
		authHeader,
	))
	if descriptionRecorder.Code != http.StatusOK {
		t.Fatalf("profile status = %d, body = %s", descriptionRecorder.Code, descriptionRecorder.Body.String())
	}
	descriptionBody := decodeExamAPIResponse[tenantResponse](t, descriptionRecorder.Body.Bytes())
	if descriptionBody.Data.Name != "青藤实验中学" ||
		descriptionBody.Data.Description != "统一管理期中、期末和补测" ||
		descriptionBody.Data.LogoURL != "/uploads/tenant-logos/qingteng.webp" {
		t.Fatalf("unexpected profile response: %#v", descriptionBody.Data)
	}
	assertTenantUpdatedBy(t, gormDB, 1, 1)

	resetRecorder := httptest.NewRecorder()
	router.ServeHTTP(resetRecorder, authorizedRequest(http.MethodPost, "/api/v1/tenants/1/reset-code", nil, authHeader))
	if resetRecorder.Code != http.StatusOK {
		t.Fatalf("reset code status = %d, body = %s", resetRecorder.Code, resetRecorder.Body.String())
	}
	resetBody := decodeExamAPIResponse[tenantResponse](t, resetRecorder.Body.Bytes())
	if resetBody.Data.TenantCode != "PM-QT02" {
		t.Fatalf("unexpected reset tenant code response: %#v", resetBody.Data)
	}
	assertTenantUpdatedBy(t, gormDB, 1, 1)

	disableRecorder := httptest.NewRecorder()
	router.ServeHTTP(disableRecorder, authorizedRequest(
		http.MethodPost,
		"/api/v1/tenants/1/register-setting",
		[]byte(`{"allow_register":false}`),
		authHeader,
	))
	if disableRecorder.Code != http.StatusOK {
		t.Fatalf("disable register status = %d, body = %s", disableRecorder.Code, disableRecorder.Body.String())
	}
	disableBody := decodeExamAPIResponse[tenantResponse](t, disableRecorder.Body.Bytes())
	if disableBody.Data.AllowRegister {
		t.Fatalf("expected registration disabled response: %#v", disableBody.Data)
	}
	assertTenantUpdatedBy(t, gormDB, 1, 1)

	enableRecorder := httptest.NewRecorder()
	router.ServeHTTP(enableRecorder, authorizedRequest(
		http.MethodPost,
		"/api/v1/tenants/1/register-setting",
		[]byte(`{"allow_register":true}`),
		authHeader,
	))
	if enableRecorder.Code != http.StatusOK {
		t.Fatalf("enable register status = %d, body = %s", enableRecorder.Code, enableRecorder.Body.String())
	}
	enableBody := decodeExamAPIResponse[tenantResponse](t, enableRecorder.Body.Bytes())
	if !enableBody.Data.AllowRegister {
		t.Fatalf("expected registration enabled response: %#v", enableBody.Data)
	}
	assertTenantUpdatedBy(t, gormDB, 1, 1)
}

func TestTenantAPIResetTenantCodeUsesDefaultGeneratorWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedPlatformLoginAPITestData(t, gormDB)
	seedTenantAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow + 1000 },
	})
	authHeader := platformAuthHeader(t, router)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodPost, "/api/v1/tenants/1/reset-code", nil, authHeader))
	if recorder.Code != http.StatusOK {
		t.Fatalf("reset code status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	body := decodeExamAPIResponse[tenantResponse](t, recorder.Body.Bytes())
	if body.Data.TenantCode == "" || body.Data.TenantCode == "PM-QT01" {
		t.Fatalf("expected generated tenant code, got %#v", body.Data)
	}
}

func platformAuthHeader(t *testing.T, router *gin.Engine) string {
	t.Helper()

	loginRecorder := httptest.NewRecorder()
	router.ServeHTTP(loginRecorder, httptest.NewRequest(
		http.MethodPost,
		"/api/v1/auth/platform/login",
		bytes.NewReader([]byte(`{"username":"admin","password":"papermind123"}`)),
	))
	if loginRecorder.Code != http.StatusOK {
		t.Fatalf("platform login status = %d, body = %s", loginRecorder.Code, loginRecorder.Body.String())
	}
	body := decodeExamAPIResponse[authSessionResponse](t, loginRecorder.Body.Bytes())
	return "Bearer " + body.Data.AccessToken
}

func authorizedRequest(method string, target string, body []byte, authHeader string) *http.Request {
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader(body)
	}
	request := httptest.NewRequest(method, target, reader)
	request.Header.Set("Authorization", authHeader)
	return request
}

func seedTenantAPITestData(t *testing.T, gormDB *gorm.DB) {
	t.Helper()

	if err := gormDB.Exec(`
		INSERT INTO tenants (
			id, name, logo_url, description, tenant_code, allow_register, status,
			created_at, updated_at, ext_json
		) VALUES (?, ?, ?, ?, ?, 1, 'enabled', ?, ?, '{}')
	`, 1, "青藤一中", "qingteng.png", "统一管理月考、联考和补测", "PM-QT01", fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
}

func seedSecondTenantAPITestData(t *testing.T, gormDB *gorm.DB) {
	t.Helper()

	if err := gormDB.Exec(`
		INSERT INTO tenants (
			id, name, logo_url, description, tenant_code, allow_register, status,
			created_at, updated_at, ext_json
		) VALUES (?, ?, ?, ?, ?, 0, 'enabled', ?, ?, '{}')
	`, 2, "知行培训", "zhixing.png", "企业知识课堂和阶段测评", "PM-ZX01", fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed second tenant: %v", err)
	}
}

func assertTenantUpdatedBy(t *testing.T, gormDB *gorm.DB, tenantID uint64, want uint64) {
	t.Helper()

	var updatedBy uint64
	if err := gormDB.Table("tenants").
		Select("updated_by").
		Where("id = ?", tenantID).
		Scan(&updatedBy).Error; err != nil {
		t.Fatalf("query tenant updated_by: %v", err)
	}
	if updatedBy != want {
		t.Fatalf("expected tenant %d updated_by %d, got %d", tenantID, want, updatedBy)
	}
}

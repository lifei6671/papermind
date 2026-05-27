package v1

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TestPlatformLoginAPIRouteSavesSessionAndAuditWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedPlatformLoginAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:              gormDB,
		Now:             func() int64 { return fixedAPINow },
		AuthTokenIssuer: fixedAuthTokenIssuer{accessToken: "platform-access-token", refreshToken: "platform-refresh-token"},
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
	if body.Data.AccessToken != "platform-access-token" || body.Data.RefreshToken != "platform-refresh-token" {
		t.Fatalf("unexpected token response: %#v", body.Data)
	}
	if body.Data.User.UserID != 1 || body.Data.User.DisplayName != "admin" || body.Data.User.Role != "platform_admin" {
		t.Fatalf("unexpected user response: %#v", body.Data.User)
	}

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
		DB:              gormDB,
		Now:             func() int64 { return fixedAPINow },
		AuthTokenIssuer: fixedAuthTokenIssuer{accessToken: "unused-access", refreshToken: "unused-refresh"},
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
	if row.PasswordHash != "papermind123" || row.Role != "student" {
		t.Fatalf("unexpected registered persistence: %#v", row)
	}
}

func seedPlatformLoginAPITestData(t *testing.T, gormDB *gorm.DB) {
	t.Helper()

	if err := gormDB.Exec(`
		INSERT INTO platform_users (
			id, username, avatar_url, phone, email, password_hash, last_login_ip, last_login_at, status,
			created_at, updated_at, ext_json
		) VALUES (?, ?, '', ?, ?, ?, '', 0, 'enabled', ?, ?, '{}')
	`, 1, "admin", "admin-phone", "admin@example.test", "papermind123", fixedAPINow, fixedAPINow).Error; err != nil {
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

type fixedAuthTokenIssuer struct {
	accessToken  string
	refreshToken string
}

func (i fixedAuthTokenIssuer) IssueAccessToken() (string, error) {
	return i.accessToken, nil
}

func (i fixedAuthTokenIssuer) IssueRefreshToken() (string, error) {
	return i.refreshToken, nil
}

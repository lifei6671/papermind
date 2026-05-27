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

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
	seedTenantAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:                   gormDB,
		Now:                  func() int64 { return fixedAPINow },
		CodeGenerator:        fixedCodeGenerator{code: "PM-XH01"},
		AllowRegisterDefault: true,
	})

	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, httptest.NewRequest(http.MethodGet, "/api/v1/tenants", nil))
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
		"description": "面向公共课和企业培训的考试空间"
	}`)
	createRecorder := httptest.NewRecorder()
	router.ServeHTTP(createRecorder, httptest.NewRequest(http.MethodPost, "/api/v1/tenants", bytes.NewReader(payload)))
	if createRecorder.Code != http.StatusOK {
		t.Fatalf("create status = %d, body = %s", createRecorder.Code, createRecorder.Body.String())
	}
	createBody := decodeExamAPIResponse[tenantResponse](t, createRecorder.Body.Bytes())
	if createBody.Data.Name != "星海大学" || createBody.Data.TenantCode != "PM-XH01" {
		t.Fatalf("unexpected created tenant response: %#v", createBody.Data)
	}
	if !createBody.Data.AllowRegister {
		t.Fatalf("expected allow_register inherited from platform default")
	}
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

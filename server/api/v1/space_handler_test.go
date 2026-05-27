package v1

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TestSpaceAPIRoutesListAndCreateWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedSpaceAPITestData(t, gormDB)
	seedPlatformLoginAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := platformAuthHeader(t, router)

	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, authorizedRequest(http.MethodGet, "/api/v1/spaces?tenant_id=10", nil, authHeader))
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listRecorder.Code, listRecorder.Body.String())
	}
	listBody := decodeExamAPIResponse[spaceListResponse](t, listRecorder.Body.Bytes())
	if len(listBody.Data.Items) != 1 {
		t.Fatalf("expected one seeded space, got %#v", listBody.Data.Items)
	}
	space := listBody.Data.Items[0]
	if space.Name != "高一 1 班" || len(space.Members) != 2 {
		t.Fatalf("unexpected seeded space response: %#v", space)
	}
	if space.Members[0].Name != "李老师" || space.Members[0].Role != "space_admin" {
		t.Fatalf("expected admin member from users join, got %#v", space.Members)
	}

	payload := []byte(`{
		"tenant_id": 10,
		"name": "高一 3 班",
		"logo_url": "class3.png",
		"description": "面向月考与补测的学生空间",
		"type": "class",
		"admin_user_ids": [22]
	}`)
	createRecorder := httptest.NewRecorder()
	router.ServeHTTP(createRecorder, authorizedRequest(http.MethodPost, "/api/v1/spaces", payload, authHeader))
	if createRecorder.Code != http.StatusOK {
		t.Fatalf("create status = %d, body = %s", createRecorder.Code, createRecorder.Body.String())
	}
	createBody := decodeExamAPIResponse[spaceResponse](t, createRecorder.Body.Bytes())
	if createBody.Data.Name != "高一 3 班" || createBody.Data.LogoURL != "class3.png" {
		t.Fatalf("unexpected created space response: %#v", createBody.Data)
	}
	if len(createBody.Data.Members) != 1 || createBody.Data.Members[0].Name != "赵老师" {
		t.Fatalf("expected created admin member, got %#v", createBody.Data.Members)
	}
}

func seedSpaceAPITestData(t *testing.T, gormDB *gorm.DB) {
	t.Helper()

	if err := gormDB.Exec(`
		INSERT INTO users (
			id, tenant_id, username, real_name, phone, email, password_hash, status,
			created_at, updated_at, ext_json
		) VALUES
			(20, 10, 'teacher_li', '李老师', '13800000020', 'li@example.test', 'hash', 'enabled', ?, ?, '{}'),
			(21, 10, 'student_zhang', '张同学', '13800000021', 'zhang@example.test', 'hash', 'enabled', ?, ?, '{}'),
			(22, 10, 'teacher_zhao', '赵老师', '13800000022', 'zhao@example.test', 'hash', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow, fixedAPINow, fixedAPINow, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed users: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO spaces (
			id, tenant_id, name, logo_url, description, type, status,
			created_at, updated_at, ext_json
		) VALUES (?, ?, ?, ?, ?, 'class', 'enabled', ?, ?, '{}')
	`, 100, 10, "高一 1 班", "class1.png", "高一语文月考主空间", fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed space: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO space_members (
			id, tenant_id, space_id, user_id, role_in_space, status,
			created_at, updated_at, ext_json
		) VALUES
			(1, 10, 100, 20, 'space_admin', 'enabled', ?, ?, '{}'),
			(2, 10, 100, 21, 'student', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed space members: %v", err)
	}
}

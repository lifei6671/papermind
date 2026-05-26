package v1

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
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

	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, httptest.NewRequest(http.MethodGet, "/api/v1/users?tenant_id=10&page=1&page_size=1", nil))
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listRecorder.Code, listRecorder.Body.String())
	}
	listBody := decodeExamAPIResponse[userListResponse](t, listRecorder.Body.Bytes())
	if len(listBody.Data.Items) != 1 || listBody.Data.Total != 2 || listBody.Data.PageSize != 1 {
		t.Fatalf("expected first page of two seeded users, got %#v", listBody.Data)
	}
	if listBody.Data.Items[0].RealName != "李老师" || listBody.Data.Items[0].Role != "teacher" {
		t.Fatalf("unexpected first user: %#v", listBody.Data.Items[0])
	}

	payload := []byte(`{
		"tenant_id": 10,
		"username": "wang.student",
		"real_name": "王同学",
		"avatar_url": "wang.png",
		"role": "student"
	}`)
	createRecorder := httptest.NewRecorder()
	router.ServeHTTP(createRecorder, httptest.NewRequest(http.MethodPost, "/api/v1/users", bytes.NewReader(payload)))
	if createRecorder.Code != http.StatusOK {
		t.Fatalf("create status = %d, body = %s", createRecorder.Code, createRecorder.Body.String())
	}
	createBody := decodeExamAPIResponse[userResponse](t, createRecorder.Body.Bytes())
	if createBody.Data.RealName != "王同学" || createBody.Data.Role != "student" || createBody.Data.AvatarURL != "wang.png" {
		t.Fatalf("unexpected created user: %#v", createBody.Data)
	}

	disableRecorder := httptest.NewRecorder()
	router.ServeHTTP(disableRecorder, httptest.NewRequest(http.MethodPost, "/api/v1/users/20/disable", bytes.NewReader([]byte(`{"tenant_id":10,"actor_id":99}`))))
	if disableRecorder.Code != http.StatusOK {
		t.Fatalf("disable status = %d, body = %s", disableRecorder.Code, disableRecorder.Body.String())
	}
	disableBody := decodeExamAPIResponse[userResponse](t, disableRecorder.Body.Bytes())
	if disableBody.Data.Status != "disabled" {
		t.Fatalf("expected disabled user, got %#v", disableBody.Data)
	}
}

func seedUserAPITestData(t *testing.T, gormDB *gorm.DB) {
	t.Helper()

	if err := gormDB.Exec(`
		INSERT INTO users (
			id, tenant_id, username, real_name, avatar_url, phone, email, password_hash, status,
			created_at, updated_at, ext_json
		) VALUES
			(20, 10, 'li.teacher', '李老师', 'li.png', '13800001020', 'li-user@example.test', 'hash', 'enabled', ?, ?, '{}'),
			(21, 10, 'zhang.student', '张同学', 'zhang.png', '13800001021', 'zhang-user@example.test', 'hash', 'disabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed users: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO user_roles (
			id, tenant_id, user_id, role, created_at, updated_at, ext_json
		) VALUES
			(1, 10, 20, 'teacher', ?, ?, '{}'),
			(2, 10, 21, 'student', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed user roles: %v", err)
	}
}

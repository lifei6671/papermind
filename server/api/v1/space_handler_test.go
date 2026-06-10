package v1

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/papermind/server/library/crypto"
	"gorm.io/gorm"
)

func TestSpaceAPIRoutesListAndCreateWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedSpaceAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

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
	if space.Members[0].Username != "teacher_li" || space.Members[0].Phone != "13800000020" || space.Members[0].Email != "li@example.test" {
		t.Fatalf("expected member user details from users join, got %#v", space.Members[0])
	}
	if space.Members[0].CreatedAt != fixedAPINow || space.Members[0].RegisterMethod != "tenant_account" {
		t.Fatalf("expected member registration metadata, got %#v", space.Members[0])
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

func TestPlatformUserCannotEnterTenantSpaceRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedPlatformLoginAPITestData(t, gormDB)
	seedSpaceAPITestData(t, gormDB)

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
		{name: "list spaces", method: http.MethodGet, target: "/api/v1/spaces?tenant_id=10"},
		{name: "create space", method: http.MethodPost, target: "/api/v1/spaces", body: []byte(`{
			"tenant_id": 10,
			"name": "平台越权空间",
			"type": "class",
			"admin_user_ids": [22]
		}`)},
		{name: "update space", method: http.MethodPut, target: "/api/v1/spaces/100", body: []byte(`{
			"tenant_id": 10,
			"name": "平台越权更新",
			"type": "class"
		}`)},
		{name: "disable space", method: http.MethodPost, target: "/api/v1/spaces/100/disable", body: []byte(`{"tenant_id":10}`)},
		{name: "delete space", method: http.MethodDelete, target: "/api/v1/spaces/100?tenant_id=10"},
		{name: "tenant-prefixed list spaces", method: http.MethodGet, target: "/api/v1/tenant/spaces?tenant_id=10"},
		{name: "tenant-prefixed create space", method: http.MethodPost, target: "/api/v1/tenant/spaces", body: []byte(`{
			"tenant_id": 10,
			"name": "平台越权空间",
			"type": "class",
			"admin_user_ids": [22]
		}`)},
		{name: "tenant-prefixed update space", method: http.MethodPut, target: "/api/v1/tenant/spaces/100", body: []byte(`{
			"tenant_id": 10,
			"name": "平台越权更新",
			"type": "class"
		}`)},
		{name: "tenant-prefixed disable space", method: http.MethodPost, target: "/api/v1/tenant/spaces/100/disable", body: []byte(`{"tenant_id":10}`)},
		{name: "tenant-prefixed delete space", method: http.MethodDelete, target: "/api/v1/tenant/spaces/100?tenant_id=10"},
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

func TestTenantAdminSpaceRoutesUseSessionTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedSpaceAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, authorizedRequest(http.MethodGet, "/api/v1/spaces?tenant_id=20", nil, authHeader))
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list with forged tenant status = %d, body = %s", listRecorder.Code, listRecorder.Body.String())
	}
	listBody := decodeExamAPIResponse[spaceListResponse](t, listRecorder.Body.Bytes())
	if len(listBody.Data.Items) != 1 || listBody.Data.Items[0].TenantID != 10 {
		t.Fatalf("expected list to use session tenant, got %#v", listBody.Data)
	}

	createRecorder := httptest.NewRecorder()
	router.ServeHTTP(createRecorder, authorizedRequest(http.MethodPost, "/api/v1/spaces", []byte(`{
		"tenant_id": 20,
		"name": "会话租户空间",
		"type": "class",
		"admin_user_ids": [22]
	}`), authHeader))
	if createRecorder.Code != http.StatusOK {
		t.Fatalf("create with forged tenant status = %d, body = %s", createRecorder.Code, createRecorder.Body.String())
	}
	createBody := decodeExamAPIResponse[spaceResponse](t, createRecorder.Body.Bytes())
	if createBody.Data.TenantID != 10 {
		t.Fatalf("expected created space to use session tenant, got %#v", createBody.Data)
	}

	updateRecorder := httptest.NewRecorder()
	router.ServeHTTP(updateRecorder, authorizedRequest(http.MethodPut, "/api/v1/spaces/100", []byte(`{
		"tenant_id": 20,
		"name": "会话租户更新",
		"type": "class"
	}`), authHeader))
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("update with forged tenant status = %d, body = %s", updateRecorder.Code, updateRecorder.Body.String())
	}
	updateBody := decodeExamAPIResponse[spaceResponse](t, updateRecorder.Body.Bytes())
	if updateBody.Data.TenantID != 10 || updateBody.Data.Name != "会话租户更新" {
		t.Fatalf("expected updated space to use session tenant, got %#v", updateBody.Data)
	}

	deleteRecorder := httptest.NewRecorder()
	router.ServeHTTP(deleteRecorder, authorizedRequest(http.MethodDelete, "/api/v1/spaces/100?tenant_id=20", nil, authHeader))
	if deleteRecorder.Code != http.StatusOK {
		t.Fatalf("delete with forged tenant status = %d, body = %s", deleteRecorder.Code, deleteRecorder.Body.String())
	}
	var deletedAt int64
	if err := gormDB.Table("spaces").
		Select("deleted_at").
		Where("tenant_id = ? AND id = ?", 10, 100).
		Scan(&deletedAt).Error; err != nil {
		t.Fatalf("query deleted space: %v", err)
	}
	if deletedAt == 0 {
		t.Fatalf("expected delete to use session tenant")
	}

	tenantPrefixedCreate := httptest.NewRecorder()
	router.ServeHTTP(tenantPrefixedCreate, authorizedRequest(http.MethodPost, "/api/v1/tenant/spaces", []byte(`{
		"tenant_id": 20,
		"name": "租户前缀空间",
		"type": "class",
		"admin_user_ids": [22]
	}`), authHeader))
	if tenantPrefixedCreate.Code != http.StatusOK {
		t.Fatalf("tenant-prefixed create with forged tenant status = %d, body = %s", tenantPrefixedCreate.Code, tenantPrefixedCreate.Body.String())
	}
	tenantPrefixedBody := decodeExamAPIResponse[spaceResponse](t, tenantPrefixedCreate.Body.Bytes())
	if tenantPrefixedBody.Data.TenantID != 10 {
		t.Fatalf("expected tenant-prefixed created space to use session tenant, got %#v", tenantPrefixedBody.Data)
	}
}

func TestSpaceProfileRoutesRequireTenantAdminWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedSpaceAPITestData(t, gormDB)
	seedProfileSpacesAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	adminAuthHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")
	spaceAdminAuthHeader := tenantAuthHeader(t, router, 10, "teacher_li", "papermind123")

	// 空间资料属于租户级管理能力，当前空间 space_admin 不能修改名称、Logo、描述或类型。
	forbiddenUpdate := httptest.NewRecorder()
	router.ServeHTTP(forbiddenUpdate, authorizedRequest(http.MethodPut, "/api/v1/spaces/100", []byte(`{
		"tenant_id": 10,
		"name": "高一 2 班",
		"logo_url": "class2.png",
		"description": "空间管理员不应能改资料",
		"type": "class"
	}`), spaceAdminAuthHeader))
	if forbiddenUpdate.Code != http.StatusForbidden {
		t.Fatalf("space admin update status = %d, body = %s", forbiddenUpdate.Code, forbiddenUpdate.Body.String())
	}

	updateRecorder := httptest.NewRecorder()
	router.ServeHTTP(updateRecorder, authorizedRequest(http.MethodPut, "/api/v1/spaces/100", []byte(`{
		"tenant_id": 10,
		"name": "高一 2 班",
		"logo_url": "class2.png",
		"description": "面向期中考试的学生空间",
		"type": "class"
	}`), adminAuthHeader))
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("tenant admin update status = %d, body = %s", updateRecorder.Code, updateRecorder.Body.String())
	}
	updateBody := decodeExamAPIResponse[spaceResponse](t, updateRecorder.Body.Bytes())
	if updateBody.Data.Name != "高一 2 班" || updateBody.Data.Description != "面向期中考试的学生空间" {
		t.Fatalf("unexpected updated space response: %#v", updateBody.Data)
	}

	// 删除空间同样只能由租户管理员触发，避免空间管理员把自己所在空间移除。
	forbiddenDelete := httptest.NewRecorder()
	router.ServeHTTP(forbiddenDelete, authorizedRequest(http.MethodDelete, "/api/v1/spaces/100?tenant_id=10", nil, spaceAdminAuthHeader))
	if forbiddenDelete.Code != http.StatusForbidden {
		t.Fatalf("space admin delete status = %d, body = %s", forbiddenDelete.Code, forbiddenDelete.Body.String())
	}

	forbiddenDisable := httptest.NewRecorder()
	router.ServeHTTP(forbiddenDisable, authorizedRequest(http.MethodPost, "/api/v1/spaces/100/disable", []byte(`{"tenant_id":10}`), spaceAdminAuthHeader))
	if forbiddenDisable.Code != http.StatusForbidden {
		t.Fatalf("space admin disable status = %d, body = %s", forbiddenDisable.Code, forbiddenDisable.Body.String())
	}

	disableRecorder := httptest.NewRecorder()
	router.ServeHTTP(disableRecorder, authorizedRequest(http.MethodPost, "/api/v1/spaces/100/disable", []byte(`{"tenant_id":10}`), adminAuthHeader))
	if disableRecorder.Code != http.StatusOK {
		t.Fatalf("tenant admin disable status = %d, body = %s", disableRecorder.Code, disableRecorder.Body.String())
	}
	disableBody := decodeExamAPIResponse[spaceResponse](t, disableRecorder.Body.Bytes())
	if disableBody.Data.Status != "disabled" {
		t.Fatalf("expected disabled space response, got %#v", disableBody.Data)
	}
	var disabledStatus string
	if err := gormDB.Table("spaces").
		Select("status").
		Where("tenant_id = ? AND id = ?", 10, 100).
		Scan(&disabledStatus).Error; err != nil {
		t.Fatalf("query disabled space status: %v", err)
	}
	if disabledStatus != "disabled" {
		t.Fatalf("expected disabled space in database, got %q", disabledStatus)
	}

	deleteRecorder := httptest.NewRecorder()
	router.ServeHTTP(deleteRecorder, authorizedRequest(http.MethodDelete, "/api/v1/spaces/100?tenant_id=10", nil, adminAuthHeader))
	if deleteRecorder.Code != http.StatusOK {
		t.Fatalf("tenant admin delete status = %d, body = %s", deleteRecorder.Code, deleteRecorder.Body.String())
	}
}

func TestSpaceMemberRoutesAllowCurrentSpaceAdminOnlyWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedSpaceAPITestData(t, gormDB)
	seedProfileSpacesAPITestData(t, gormDB)
	seedSpaceMemberPermissionAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	spaceAdminAuthHeader := tenantAuthHeader(t, router, 10, "teacher_li", "papermind123")

	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, authorizedRequest(http.MethodGet, "/api/v1/spaces/100/members?tenant_id=10", nil, spaceAdminAuthHeader))
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("space admin list own members status = %d, body = %s", listRecorder.Code, listRecorder.Body.String())
	}
	listBody := decodeExamAPIResponse[spaceMemberListResponse](t, listRecorder.Body.Bytes())
	if listBody.Data.Total != 2 || len(listBody.Data.Items) != 2 {
		t.Fatalf("expected own space members, got %#v", listBody.Data)
	}
	if listBody.Data.Items[1].Username != "student_zhang" || listBody.Data.Items[1].Email != "zhang@example.test" {
		t.Fatalf("expected list members to include user detail fields, got %#v", listBody.Data.Items[1])
	}

	addRecorder := httptest.NewRecorder()
	router.ServeHTTP(addRecorder, authorizedRequest(http.MethodPost, "/api/v1/spaces/100/members", []byte(`{
		"tenant_id": 10,
		"user_id": 22,
		"role": "teacher"
	}`), spaceAdminAuthHeader))
	if addRecorder.Code != http.StatusOK {
		t.Fatalf("space admin add own member status = %d, body = %s", addRecorder.Code, addRecorder.Body.String())
	}
	addBody := decodeExamAPIResponse[spaceMemberResponse](t, addRecorder.Body.Bytes())
	if addBody.Data.UserID != 22 || addBody.Data.Role != "teacher" {
		t.Fatalf("unexpected added member response: %#v", addBody.Data)
	}

	updateRecorder := httptest.NewRecorder()
	router.ServeHTTP(updateRecorder, authorizedRequest(http.MethodPut, "/api/v1/spaces/100/members/21", []byte(`{
		"tenant_id": 10,
		"role": "teacher"
	}`), spaceAdminAuthHeader))
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("space admin update own member status = %d, body = %s", updateRecorder.Code, updateRecorder.Body.String())
	}
	updateBody := decodeExamAPIResponse[spaceMemberResponse](t, updateRecorder.Body.Bytes())
	if updateBody.Data.UserID != 21 || updateBody.Data.Role != "teacher" {
		t.Fatalf("unexpected updated member response: %#v", updateBody.Data)
	}

	removeRecorder := httptest.NewRecorder()
	router.ServeHTTP(removeRecorder, authorizedRequest(http.MethodDelete, "/api/v1/spaces/100/members/21?tenant_id=10", nil, spaceAdminAuthHeader))
	if removeRecorder.Code != http.StatusOK {
		t.Fatalf("space admin remove own member status = %d, body = %s", removeRecorder.Code, removeRecorder.Body.String())
	}
	var removedAt int64
	if err := gormDB.Table("space_members").
		Select("deleted_at").
		Where("tenant_id = ? AND space_id = ? AND user_id = ?", 10, 100, 21).
		Scan(&removedAt).Error; err != nil {
		t.Fatalf("query removed member: %v", err)
	}
	if removedAt == 0 {
		t.Fatalf("expected removed member to be soft deleted")
	}

	for _, tc := range []struct {
		name   string
		method string
		target string
		body   []byte
	}{
		{name: "list other space", method: http.MethodGet, target: "/api/v1/spaces/110/members?tenant_id=10"},
		{name: "add other space", method: http.MethodPost, target: "/api/v1/spaces/110/members", body: []byte(`{"tenant_id":10,"user_id":21,"role":"student"}`)},
		{name: "update other space", method: http.MethodPut, target: "/api/v1/spaces/110/members/22", body: []byte(`{"tenant_id":10,"role":"teacher"}`)},
		{name: "remove other space", method: http.MethodDelete, target: "/api/v1/spaces/110/members/22?tenant_id=10"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, authorizedRequest(tc.method, tc.target, tc.body, spaceAdminAuthHeader))
			if recorder.Code != http.StatusForbidden {
				t.Fatalf("%s status = %d, body = %s", tc.name, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestSpaceTeacherCanListCurrentSpaceStudentCandidatesOnlyWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedSpaceAPITestData(t, gormDB)
	seedProfileSpacesAPITestData(t, gormDB)
	seedSpaceMemberPermissionAPITestData(t, gormDB)

	if err := gormDB.Table("space_members").
		Where("tenant_id = ? AND space_id = ? AND user_id = ?", 10, 100, 20).
		Update("role_in_space", "teacher").Error; err != nil {
		t.Fatalf("demote seeded space admin to teacher: %v", err)
	}

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	teacherAuthHeader := tenantAuthHeader(t, router, 10, "teacher_li", "papermind123")

	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, authorizedRequest(http.MethodGet, "/api/v1/spaces/100/members?tenant_id=10&role=student&status=enabled&search=张", nil, teacherAuthHeader))
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("teacher list current space student candidates status = %d, body = %s", listRecorder.Code, listRecorder.Body.String())
	}
	listBody := decodeExamAPIResponse[spaceMemberListResponse](t, listRecorder.Body.Bytes())
	if listBody.Data.Total != 1 || len(listBody.Data.Items) != 1 {
		t.Fatalf("expected one student candidate, got %#v", listBody.Data)
	}
	if listBody.Data.Items[0].UserID != 21 || listBody.Data.Items[0].Role != "student" || listBody.Data.Items[0].Status != "enabled" {
		t.Fatalf("expected enabled student candidate only, got %#v", listBody.Data.Items[0])
	}

	for _, tc := range []struct {
		name   string
		method string
		target string
		body   []byte
	}{
		{name: "list teachers in own space", method: http.MethodGet, target: "/api/v1/spaces/100/members?tenant_id=10&role=teacher&status=enabled"},
		{name: "list students without enabled status", method: http.MethodGet, target: "/api/v1/spaces/100/members?tenant_id=10&role=student"},
		{name: "list other space students", method: http.MethodGet, target: "/api/v1/spaces/110/members?tenant_id=10&role=student&status=enabled"},
		{name: "add own space member", method: http.MethodPost, target: "/api/v1/spaces/100/members", body: []byte(`{"tenant_id":10,"user_id":22,"role":"student"}`)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, authorizedRequest(tc.method, tc.target, tc.body, teacherAuthHeader))
			if recorder.Code != http.StatusForbidden {
				t.Fatalf("%s status = %d, body = %s", tc.name, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestSpaceMemberRoutesKeepLastSpaceAdminWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedSpaceAPITestData(t, gormDB)
	seedProfileSpacesAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	spaceAdminAuthHeader := tenantAuthHeader(t, router, 10, "teacher_li", "papermind123")

	for _, tc := range []struct {
		name   string
		method string
		target string
		body   []byte
	}{
		{name: "disable last admin", method: http.MethodPut, target: "/api/v1/spaces/100/members/20", body: []byte(`{"tenant_id":10,"status":"disabled"}`)},
		{name: "downgrade last admin", method: http.MethodPut, target: "/api/v1/spaces/100/members/20", body: []byte(`{"tenant_id":10,"role":"teacher"}`)},
		{name: "remove last admin", method: http.MethodDelete, target: "/api/v1/spaces/100/members/20?tenant_id=10"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, authorizedRequest(tc.method, tc.target, tc.body, spaceAdminAuthHeader))
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("%s status = %d, body = %s", tc.name, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestTenantAdminCanManageSpaceMembersWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedSpaceAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	adminAuthHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, authorizedRequest(http.MethodGet, "/api/v1/spaces/100/members?tenant_id=10", nil, adminAuthHeader))
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("tenant admin list members status = %d, body = %s", listRecorder.Code, listRecorder.Body.String())
	}
	listBody := decodeExamAPIResponse[spaceMemberListResponse](t, listRecorder.Body.Bytes())
	if listBody.Data.Total != 2 || len(listBody.Data.Items) != 2 || listBody.Data.Items[0].Role != "space_admin" {
		t.Fatalf("unexpected tenant admin member list: %#v", listBody.Data)
	}

	addRecorder := httptest.NewRecorder()
	router.ServeHTTP(addRecorder, authorizedRequest(http.MethodPost, "/api/v1/spaces/100/members", []byte(`{
		"tenant_id": 10,
		"user_id": 22,
		"role": "space_admin"
	}`), adminAuthHeader))
	if addRecorder.Code != http.StatusOK {
		t.Fatalf("tenant admin add member status = %d, body = %s", addRecorder.Code, addRecorder.Body.String())
	}
	addBody := decodeExamAPIResponse[spaceMemberResponse](t, addRecorder.Body.Bytes())
	if addBody.Data.UserID != 22 || addBody.Data.Role != "space_admin" {
		t.Fatalf("unexpected tenant admin add response: %#v", addBody.Data)
	}

	updateRecorder := httptest.NewRecorder()
	router.ServeHTTP(updateRecorder, authorizedRequest(http.MethodPut, "/api/v1/spaces/100/members/22", []byte(`{
		"tenant_id": 10,
		"role": "teacher"
	}`), adminAuthHeader))
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("tenant admin update member status = %d, body = %s", updateRecorder.Code, updateRecorder.Body.String())
	}
	updateBody := decodeExamAPIResponse[spaceMemberResponse](t, updateRecorder.Body.Bytes())
	if updateBody.Data.UserID != 22 || updateBody.Data.Role != "teacher" {
		t.Fatalf("unexpected tenant admin update response: %#v", updateBody.Data)
	}

	disableRecorder := httptest.NewRecorder()
	router.ServeHTTP(disableRecorder, authorizedRequest(http.MethodPut, "/api/v1/spaces/100/members/22", []byte(`{
		"tenant_id": 10,
		"status": "disabled"
	}`), adminAuthHeader))
	if disableRecorder.Code != http.StatusOK {
		t.Fatalf("tenant admin disable member status = %d, body = %s", disableRecorder.Code, disableRecorder.Body.String())
	}
	disableBody := decodeExamAPIResponse[spaceMemberResponse](t, disableRecorder.Body.Bytes())
	if disableBody.Data.UserID != 22 || disableBody.Data.Status != "disabled" {
		t.Fatalf("unexpected tenant admin disable response: %#v", disableBody.Data)
	}

	enableRecorder := httptest.NewRecorder()
	router.ServeHTTP(enableRecorder, authorizedRequest(http.MethodPut, "/api/v1/spaces/100/members/22", []byte(`{
		"tenant_id": 10,
		"status": "enabled"
	}`), adminAuthHeader))
	if enableRecorder.Code != http.StatusOK {
		t.Fatalf("tenant admin enable member status = %d, body = %s", enableRecorder.Code, enableRecorder.Body.String())
	}
	enableBody := decodeExamAPIResponse[spaceMemberResponse](t, enableRecorder.Body.Bytes())
	if enableBody.Data.UserID != 22 || enableBody.Data.Status != "enabled" {
		t.Fatalf("unexpected tenant admin enable response: %#v", enableBody.Data)
	}

	removeRecorder := httptest.NewRecorder()
	router.ServeHTTP(removeRecorder, authorizedRequest(http.MethodDelete, "/api/v1/spaces/100/members/22?tenant_id=10", nil, adminAuthHeader))
	if removeRecorder.Code != http.StatusOK {
		t.Fatalf("tenant admin remove member status = %d, body = %s", removeRecorder.Code, removeRecorder.Body.String())
	}
	var removedAt int64
	if err := gormDB.Table("space_members").
		Select("deleted_at").
		Where("tenant_id = ? AND space_id = ? AND user_id = ?", 10, 100, 22).
		Scan(&removedAt).Error; err != nil {
		t.Fatalf("query removed member: %v", err)
	}
	if removedAt == 0 {
		t.Fatalf("expected tenant admin removed member to be soft deleted")
	}
}

func TestSpaceMemberRoutesRejectRemovingMissingMember(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedSpaceAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	adminAuthHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodDelete, "/api/v1/tenant/spaces/100/members/404?tenant_id=10", nil, adminAuthHeader))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected missing member removal to be rejected, got status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestSpaceMemberRoutesRejectInvalidRoleAndUnavailableUsers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedSpaceAPITestData(t, gormDB)
	if err := gormDB.Table("tenant_user_memberships").
		Where("tenant_id = ? AND user_id IN ?", 10, []uint64{21, 22}).
		Update("status", "disabled").Error; err != nil {
		t.Fatalf("disable target user: %v", err)
	}

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	adminAuthHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

	for _, tc := range []struct {
		name   string
		method string
		target string
		body   []byte
	}{
		{name: "add invalid role", method: http.MethodPost, target: "/api/v1/tenant/spaces/100/members", body: []byte(`{"user_id":22,"role":"owner"}`)},
		{name: "update invalid role", method: http.MethodPut, target: "/api/v1/tenant/spaces/100/members/21", body: []byte(`{"role":"owner"}`)},
		{name: "update disabled member role", method: http.MethodPut, target: "/api/v1/tenant/spaces/100/members/21", body: []byte(`{"role":"teacher"}`)},
		{name: "add disabled user", method: http.MethodPost, target: "/api/v1/tenant/spaces/100/members", body: []byte(`{"user_id":22,"role":"teacher"}`)},
		{name: "create space with disabled admin", method: http.MethodPost, target: "/api/v1/tenant/spaces", body: []byte(`{
			"name": "不可用管理员空间",
			"type": "class",
			"admin_user_ids": [22]
		}`)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, authorizedRequest(tc.method, tc.target, tc.body, adminAuthHeader))
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("%s status = %d, body = %s", tc.name, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestSpaceMemberRoutesUseSessionTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedSpaceAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	adminAuthHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

	listRecorder := httptest.NewRecorder()
	router.ServeHTTP(listRecorder, authorizedRequest(http.MethodGet, "/api/v1/spaces/100/members?tenant_id=20", nil, adminAuthHeader))
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list members with forged tenant status = %d, body = %s", listRecorder.Code, listRecorder.Body.String())
	}
	listBody := decodeExamAPIResponse[spaceMemberListResponse](t, listRecorder.Body.Bytes())
	if listBody.Data.Total != 2 || len(listBody.Data.Items) != 2 || listBody.Data.Items[0].UserID != 20 {
		t.Fatalf("expected member list to use session tenant, got %#v", listBody.Data)
	}

	addRecorder := httptest.NewRecorder()
	router.ServeHTTP(addRecorder, authorizedRequest(http.MethodPost, "/api/v1/spaces/100/members", []byte(`{
		"tenant_id": 20,
		"user_id": 22,
		"role": "teacher"
	}`), adminAuthHeader))
	if addRecorder.Code != http.StatusOK {
		t.Fatalf("add member with forged tenant status = %d, body = %s", addRecorder.Code, addRecorder.Body.String())
	}
	addBody := decodeExamAPIResponse[spaceMemberResponse](t, addRecorder.Body.Bytes())
	if addBody.Data.UserID != 22 || addBody.Data.Role != "teacher" {
		t.Fatalf("expected add member to use session tenant, got %#v", addBody.Data)
	}

	updateRecorder := httptest.NewRecorder()
	router.ServeHTTP(updateRecorder, authorizedRequest(http.MethodPut, "/api/v1/spaces/100/members/22", []byte(`{
		"tenant_id": 20,
		"role": "space_admin"
	}`), adminAuthHeader))
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("update member with forged tenant status = %d, body = %s", updateRecorder.Code, updateRecorder.Body.String())
	}
	updateBody := decodeExamAPIResponse[spaceMemberResponse](t, updateRecorder.Body.Bytes())
	if updateBody.Data.UserID != 22 || updateBody.Data.Role != "space_admin" {
		t.Fatalf("expected update member to use session tenant, got %#v", updateBody.Data)
	}

	removeRecorder := httptest.NewRecorder()
	router.ServeHTTP(removeRecorder, authorizedRequest(http.MethodDelete, "/api/v1/spaces/100/members/22?tenant_id=20", nil, adminAuthHeader))
	if removeRecorder.Code != http.StatusOK {
		t.Fatalf("remove member with forged tenant status = %d, body = %s", removeRecorder.Code, removeRecorder.Body.String())
	}
	var deletedAt int64
	if err := gormDB.Table("space_members").
		Select("deleted_at").
		Where("tenant_id = ? AND space_id = ? AND user_id = ?", 10, 100, 22).
		Scan(&deletedAt).Error; err != nil {
		t.Fatalf("query removed member: %v", err)
	}
	if deletedAt == 0 {
		t.Fatalf("expected remove member to use session tenant")
	}
}

func TestSpaceMemberRoutesEnableDisabledMemberWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedSpaceAPITestData(t, gormDB)
	if err := gormDB.Table("space_members").
		Where("tenant_id = ? AND space_id = ? AND user_id = ?", 10, 100, 21).
		Update("status", "disabled").Error; err != nil {
		t.Fatalf("disable member fixture: %v", err)
	}

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	adminAuthHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodPut, "/api/v1/spaces/100/members/21", []byte(`{
		"tenant_id": 10,
		"status": "enabled"
	}`), adminAuthHeader))
	if recorder.Code != http.StatusOK {
		t.Fatalf("enable member status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	body := decodeExamAPIResponse[spaceMemberResponse](t, recorder.Body.Bytes())
	if body.Data.UserID != 21 || body.Data.Status != "enabled" {
		t.Fatalf("expected enabled member response, got %#v", body.Data)
	}

	var status string
	if err := gormDB.Table("space_members").
		Select("status").
		Where("tenant_id = ? AND space_id = ? AND user_id = ?", 10, 100, 21).
		Scan(&status).Error; err != nil {
		t.Fatalf("query member status: %v", err)
	}
	if status != "enabled" {
		t.Fatalf("expected member status enabled, got %q", status)
	}
}

func TestSpaceMemberRoutesEnableInactiveTenantUserMemberWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedSpaceAPITestData(t, gormDB)
	if err := gormDB.Table("tenant_user_memberships").
		Where("tenant_id = ? AND user_id = ?", 10, 21).
		Update("status", "disabled").Error; err != nil {
		t.Fatalf("disable tenant user fixture: %v", err)
	}
	if err := gormDB.Table("space_members").
		Where("tenant_id = ? AND space_id = ? AND user_id = ?", 10, 100, 21).
		Update("status", "disabled").Error; err != nil {
		t.Fatalf("disable member fixture: %v", err)
	}

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	adminAuthHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, authorizedRequest(http.MethodPut, "/api/v1/tenant/spaces/100/members/21", []byte(`{
		"tenant_id": 10,
		"status": "enabled"
	}`), adminAuthHeader))
	if recorder.Code != http.StatusOK {
		t.Fatalf("enable inactive tenant user member status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	body := decodeExamAPIResponse[spaceMemberResponse](t, recorder.Body.Bytes())
	if body.Data.UserID != 21 || body.Data.Status != "enabled" || body.Data.Username != "student_zhang" {
		t.Fatalf("expected enabled inactive tenant user member response, got %#v", body.Data)
	}

	var status string
	if err := gormDB.Table("space_members").
		Select("status").
		Where("tenant_id = ? AND space_id = ? AND user_id = ?", 10, 100, 21).
		Scan(&status).Error; err != nil {
		t.Fatalf("query member status: %v", err)
	}
	if status != "enabled" {
		t.Fatalf("expected member status enabled, got %q", status)
	}
}

func TestTenantAdminCannotBreakLastSpaceAdminInvariantWithSQLite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedSpaceAPITestData(t, gormDB)

	router := NewRouter(RouterOptions{
		DB:  gormDB,
		Now: func() int64 { return fixedAPINow },
	})
	adminAuthHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

	for _, tc := range []struct {
		name   string
		method string
		target string
		body   []byte
	}{
		{name: "disable last admin", method: http.MethodPut, target: "/api/v1/spaces/100/members/20", body: []byte(`{"tenant_id":10,"status":"disabled"}`)},
		{name: "downgrade last admin", method: http.MethodPut, target: "/api/v1/spaces/100/members/20", body: []byte(`{"tenant_id":10,"role":"teacher"}`)},
		{name: "remove last admin", method: http.MethodDelete, target: "/api/v1/spaces/100/members/20?tenant_id=10"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, authorizedRequest(tc.method, tc.target, tc.body, adminAuthHeader))
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("%s status = %d, body = %s", tc.name, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func seedSpaceAPITestData(t *testing.T, gormDB *gorm.DB) {
	t.Helper()

	passwordHash, err := crypto.HashPassword("papermind123")
	if err != nil {
		t.Fatalf("hash tenant admin password: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT OR IGNORE INTO tenants (
			id, name, logo_url, description, tenant_code, allow_register, status,
			created_at, updated_at, version, ext_json
		) VALUES (10, '青藤一中', '', '空间接口测试租户', 'PM-QT01', true, 'enabled', ?, ?, 1, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed space API tenant: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO users (
			id, username, real_name, phone, email, password_hash, status,
			created_at, updated_at, ext_json
		) VALUES
			(20, 'teacher_li', '李老师', '13800000020', 'li@example.test', 'hash', 'enabled', ?, ?, '{}'),
			(21, 'student_zhang', '张同学', '13800000021', 'zhang@example.test', 'hash', 'enabled', ?, ?, '{}'),
			(22, 'teacher_zhao', '赵老师', '13800000022', 'zhao@example.test', 'hash', 'enabled', ?, ?, '{}'),
			(99, 'tenant.admin', '租户管理员', '13800000099', 'admin@example.test', ?, 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow, fixedAPINow, fixedAPINow, fixedAPINow, fixedAPINow, passwordHash, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed users: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO tenant_user_memberships (
			id, tenant_id, user_id, role, status, created_at, updated_at, ext_json
		) VALUES
			(20, 10, 20, 'teacher', 'enabled', ?, ?, '{}'),
			(21, 10, 21, 'student', 'enabled', ?, ?, '{}'),
			(22, 10, 22, 'teacher', 'enabled', ?, ?, '{}'),
			(99, 10, 99, 'tenant_admin', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow, fixedAPINow, fixedAPINow, fixedAPINow, fixedAPINow, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed tenant admin role: %v", err)
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

func seedSpaceMemberPermissionAPITestData(t *testing.T, gormDB *gorm.DB) {
	t.Helper()
	if err := gormDB.Exec(`
		INSERT INTO spaces (
			id, tenant_id, name, logo_url, description, type, status,
			created_at, updated_at, ext_json
		) VALUES (110, 10, '高一 10 班', '', '', 'class', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed other space: %v", err)
	}
	if err := gormDB.Exec(`
		INSERT INTO space_members (
			id, tenant_id, space_id, user_id, role_in_space, status,
			created_at, updated_at, ext_json
		) VALUES (110, 10, 110, 22, 'space_admin', 'enabled', ?, ?, '{}')
	`, fixedAPINow, fixedAPINow).Error; err != nil {
		t.Fatalf("seed other space admin: %v", err)
	}
}

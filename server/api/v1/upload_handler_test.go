package v1

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestUploadAPIRouteStoresMultipartFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	uploadDir := t.TempDir()
	gormDB := openExamAPITestDB(t)
	seedTenantAPITestData(t, gormDB)
	seedUserAPITestData(t, gormDB)
	router := NewRouter(RouterOptions{
		DB:        gormDB,
		UploadDir: uploadDir,
	})
	authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

	pngContent := "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"
	requestBody, contentType := buildUploadMultipart(t, "tenant-logos", "logo.png", "image/png", pngContent)
	recorder := httptest.NewRecorder()
	request := authorizedRequest(http.MethodPost, "/api/v1/uploads", requestBody.Bytes(), authHeader)
	request.Header.Set("Content-Type", contentType)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("upload status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	body := decodeExamAPIResponse[uploadResponse](t, recorder.Body.Bytes())
	expectedKey := regexp.MustCompile(`^tenant-logos/\d{8}/\d{14}_[a-f0-9]{16}_[a-f0-9]{16}\.png$`)
	if !expectedKey.MatchString(body.Data.Key) || body.Data.URL != "/uploads/"+body.Data.Key {
		t.Fatalf("unexpected upload response: %#v", body.Data)
	}
	if !strings.HasSuffix(body.Data.FileName, ".png") || body.Data.ContentType != "image/png" || body.Data.Size != int64(len(pngContent)) {
		t.Fatalf("unexpected upload metadata: %#v", body.Data)
	}
	content, err := os.ReadFile(filepath.Join(uploadDir, filepath.FromSlash(body.Data.Key)))
	if err != nil {
		t.Fatalf("read uploaded file: %v", err)
	}
	if string(content) != pngContent {
		t.Fatalf("uploaded content = %q", string(content))
	}
}

func TestPlatformAdminCanUploadTenantLogo(t *testing.T) {
	gin.SetMode(gin.TestMode)
	uploadDir := t.TempDir()
	gormDB := openExamAPITestDB(t)
	seedPlatformLoginAPITestData(t, gormDB)
	router := NewRouter(RouterOptions{
		DB:        gormDB,
		UploadDir: uploadDir,
	})
	authHeader := platformAuthHeader(t, router)

	requestBody, contentType := buildUploadMultipart(t, "tenant-logos", "logo.png", "image/png", "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	recorder := httptest.NewRecorder()
	request := authorizedRequest(http.MethodPost, "/api/v1/uploads", requestBody.Bytes(), authHeader)
	request.Header.Set("Content-Type", contentType)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("platform upload status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestUploadAPIRouteRejectsDisabledTenantAdminSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	uploadDir := t.TempDir()
	gormDB := openExamAPITestDB(t)
	seedTenantAPITestData(t, gormDB)
	seedUserAPITestData(t, gormDB)
	router := NewRouter(RouterOptions{
		DB:        gormDB,
		UploadDir: uploadDir,
	})
	authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")
	if err := gormDB.Table("tenant_user_memberships").
		Where("tenant_id = ? AND user_id = ?", 10, 99).
		Update("status", "disabled").Error; err != nil {
		t.Fatalf("disable tenant admin after login: %v", err)
	}

	requestBody, contentType := buildUploadMultipart(t, "tenant-logos", "logo.png", "image/png", "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	recorder := httptest.NewRecorder()
	request := authorizedRequest(http.MethodPost, "/api/v1/uploads", requestBody.Bytes(), authHeader)
	request.Header.Set("Content-Type", contentType)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected disabled tenant admin upload to be forbidden, got status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestUploadAPIRouteRejectsDisabledPlatformAdminSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	uploadDir := t.TempDir()
	gormDB := openExamAPITestDB(t)
	seedPlatformLoginAPITestData(t, gormDB)
	router := NewRouter(RouterOptions{
		DB:        gormDB,
		UploadDir: uploadDir,
	})
	authHeader := platformAuthHeader(t, router)
	if err := gormDB.Table("platform_users").
		Where("id = ?", 1).
		Update("status", "disabled").Error; err != nil {
		t.Fatalf("disable platform admin after login: %v", err)
	}

	requestBody, contentType := buildUploadMultipart(t, "tenant-logos", "logo.png", "image/png", "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	recorder := httptest.NewRecorder()
	request := authorizedRequest(http.MethodPost, "/api/v1/uploads", requestBody.Bytes(), authHeader)
	request.Header.Set("Content-Type", contentType)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected disabled platform admin upload to be forbidden, got status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestBuildUploadObjectKeyUsesTimestampMD5AndExtension(t *testing.T) {
	now := time.Date(2026, 5, 27, 16, 8, 9, 0, time.Local)
	hash := md5.Sum([]byte("logo"))
	expectedPrefix := "tenant-logos/20260527/20260527160809_" + hex.EncodeToString(hash[:])[:16] + "_"
	expectedSuffix := ".webp"

	key, err := buildUploadObjectKey("tenant-logos", "logo.PNG", "image/webp", []byte("logo"), now)
	if err != nil {
		t.Fatalf("build object key: %v", err)
	}
	if !strings.HasPrefix(key, expectedPrefix) || !strings.HasSuffix(key, expectedSuffix) {
		t.Fatalf("key = %q, want prefix %q and suffix %q", key, expectedPrefix, expectedSuffix)
	}
	randomPart := strings.TrimSuffix(strings.TrimPrefix(key, expectedPrefix), expectedSuffix)
	if !regexp.MustCompile(`^[a-f0-9]{16}$`).MatchString(randomPart) {
		t.Fatalf("random suffix = %q, want 16 lowercase hex chars", randomPart)
	}
}

func TestBuildUploadObjectKeyAvoidsSameSecondContentCollision(t *testing.T) {
	now := time.Date(2026, 5, 27, 16, 8, 9, 0, time.Local)
	firstKey, err := buildUploadObjectKey("tenant-logos", "logo.png", "image/png", []byte("logo"), now)
	if err != nil {
		t.Fatalf("build first object key: %v", err)
	}
	secondKey, err := buildUploadObjectKey("tenant-logos", "logo.png", "image/png", []byte("logo"), now)
	if err != nil {
		t.Fatalf("build second object key: %v", err)
	}
	if firstKey == secondKey {
		t.Fatalf("expected repeated upload keys to differ, both = %q", firstKey)
	}
}

func TestUploadAPIRouteRejectsMissingFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedTenantAPITestData(t, gormDB)
	seedUserAPITestData(t, gormDB)
	router := NewRouter(RouterOptions{
		DB:        gormDB,
		UploadDir: t.TempDir(),
	})
	authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

	recorder := httptest.NewRecorder()
	request := authorizedRequest(http.MethodPost, "/api/v1/uploads", nil, authHeader)
	request.Header.Set("Content-Type", "multipart/form-data")
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("missing file status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestUploadAPIRouteRejectsOversizedFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedTenantAPITestData(t, gormDB)
	seedUserAPITestData(t, gormDB)
	router := NewRouter(RouterOptions{
		DB:        gormDB,
		UploadDir: t.TempDir(),
	})
	authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

	requestBody, contentType := buildUploadMultipart(t, "tenant-logos", "large.png", "image/png", strings.Repeat("x", 10*1024*1024+1))
	recorder := httptest.NewRecorder()
	request := authorizedRequest(http.MethodPost, "/api/v1/uploads", requestBody.Bytes(), authHeader)
	request.Header.Set("Content-Type", contentType)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized upload status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestUploadAPIRouteRejectsSpoofedImageContentType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gormDB := openExamAPITestDB(t)
	seedTenantAPITestData(t, gormDB)
	seedUserAPITestData(t, gormDB)
	router := NewRouter(RouterOptions{
		DB:        gormDB,
		UploadDir: t.TempDir(),
	})
	authHeader := tenantAuthHeader(t, router, 10, "tenant.admin", "papermind123")

	requestBody, contentType := buildUploadMultipart(t, "tenant-logos", "attack.html", "image/png", "<!doctype html><script>alert(1)</script>")
	recorder := httptest.NewRecorder()
	request := authorizedRequest(http.MethodPost, "/api/v1/uploads", requestBody.Bytes(), authHeader)
	request.Header.Set("Content-Type", contentType)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("spoofed upload status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

func buildUploadMultipart(t *testing.T, category string, fileName string, contentType string, fileContent string) (*bytes.Buffer, string) {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("category", category); err != nil {
		t.Fatalf("write category field: %v", err)
	}
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", `form-data; name="file"; filename="`+fileName+`"`)
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatalf("create upload file field: %v", err)
	}
	if _, err := part.Write([]byte(fileContent)); err != nil {
		t.Fatalf("write upload file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	return body, writer.FormDataContentType()
}

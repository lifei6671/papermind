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
	router := NewRouter(RouterOptions{
		DB:        openExamAPITestDB(t),
		UploadDir: uploadDir,
	})

	requestBody, contentType := buildUploadMultipart(t, "tenant-logos", "logo.png", "image/png", "logo")
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/uploads", requestBody)
	request.Header.Set("Content-Type", contentType)
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("upload status = %d, body = %s", recorder.Code, recorder.Body.String())
	}

	body := decodeExamAPIResponse[uploadResponse](t, recorder.Body.Bytes())
	expectedKey := regexp.MustCompile(`^tenant-logos/\d{8}/\d{14}_[a-f0-9]{16}\.png$`)
	if !expectedKey.MatchString(body.Data.Key) || body.Data.URL != "/uploads/"+body.Data.Key {
		t.Fatalf("unexpected upload response: %#v", body.Data)
	}
	if !strings.HasSuffix(body.Data.FileName, ".png") || body.Data.ContentType != "image/png" || body.Data.Size != 4 {
		t.Fatalf("unexpected upload metadata: %#v", body.Data)
	}
	content, err := os.ReadFile(filepath.Join(uploadDir, filepath.FromSlash(body.Data.Key)))
	if err != nil {
		t.Fatalf("read uploaded file: %v", err)
	}
	if string(content) != "logo" {
		t.Fatalf("uploaded content = %q", string(content))
	}
}

func TestBuildUploadObjectKeyUsesTimestampMD5AndExtension(t *testing.T) {
	now := time.Date(2026, 5, 27, 16, 8, 9, 0, time.Local)
	hash := md5.Sum([]byte("logo"))
	expected := "tenant-logos/20260527/20260527160809_" + hex.EncodeToString(hash[:])[:16] + ".webp"

	key, err := buildUploadObjectKey("tenant-logos", "logo.PNG", "image/webp", []byte("logo"), now)
	if err != nil {
		t.Fatalf("build object key: %v", err)
	}
	if key != expected {
		t.Fatalf("key = %q, want %q", key, expected)
	}
}

func TestUploadAPIRouteRejectsMissingFile(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := NewRouter(RouterOptions{
		DB:        openExamAPITestDB(t),
		UploadDir: t.TempDir(),
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/uploads", bytes.NewReader(nil))
	request.Header.Set("Content-Type", "multipart/form-data")
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("missing file status = %d, body = %s", recorder.Code, recorder.Body.String())
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

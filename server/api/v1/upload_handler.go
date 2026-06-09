package v1

import (
	"bytes"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/papermind/server/internal/storage"
	"github.com/lifei6671/papermind/server/library/code"
	"github.com/lifei6671/papermind/server/library/response"
)

type uploadHandler struct {
	store storage.ObjectStore
	now   func() time.Time
}

const maxUploadFileBytes = 10 * 1024 * 1024

type uploadResponse struct {
	Key         string `json:"key"`
	URL         string `json:"url"`
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
}

type uploadObjectKeyInput struct {
	Category    string
	FileName    string
	ContentType string
	FileContent []byte
	Now         time.Time
}

func (h uploadHandler) create(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxUploadFileBytes)
	fileHeader, err := c.FormFile("file")
	if err != nil {
		if strings.Contains(err.Error(), "request body too large") {
			c.JSON(http.StatusRequestEntityTooLarge, response.Fail(code.InvalidParam, "上传文件不能超过 10MB"))
			return
		}
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "file 不能为空"))
		return
	}
	category, err := normalizeUploadCategory(c.PostForm("category"))
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, err.Error()))
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "读取上传文件失败"))
		return
	}
	defer file.Close()
	fileContent, err := io.ReadAll(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "读取上传文件失败"))
		return
	}

	contentType := detectUploadContentType(fileContent)
	if !isAllowedUploadContentType(contentType) {
		c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "不支持的上传文件类型"))
		return
	}
	objectKey, err := buildUploadObjectKey(uploadObjectKeyInput{
		Category:    category,
		FileName:    fileHeader.Filename,
		ContentType: contentType,
		FileContent: fileContent,
		Now:         h.now(),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "生成上传文件名失败"))
		return
	}

	result, err := h.store.PutObject(c.Request.Context(), storage.PutObjectInput{
		Key:         objectKey,
		FileName:    filepath.Base(objectKey),
		ContentType: contentType,
		Size:        int64(len(fileContent)),
		Body:        bytes.NewReader(fileContent),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Fail(code.InternalError, "保存上传文件失败"))
		return
	}
	c.JSON(http.StatusOK, response.OK(uploadToResponse(result)))
}

func isAllowedUploadContentType(contentType string) bool {
	switch contentType {
	case "image/png", "image/jpeg", "image/webp":
		return true
	default:
		return false
	}
}

func uploadToResponse(result storage.PutObjectResult) uploadResponse {
	return uploadResponse{
		Key:         result.Key,
		URL:         result.URL,
		FileName:    result.FileName,
		ContentType: result.ContentType,
		Size:        result.Size,
	}
}

func normalizeUploadCategory(raw string) (string, error) {
	if raw == "" {
		return "general", nil
	}
	for _, char := range raw {
		if (char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') ||
			char == '-' || char == '_' {
			continue
		}
		return "", errors.New("category 只能包含字母、数字、横线或下划线")
	}
	return strings.ToLower(raw), nil
}

func buildUploadObjectKey(input uploadObjectKeyInput) (string, error) {
	fileTime := input.Now.Format("20060102150405")
	fileHash := md5.Sum(input.FileContent)
	extension := detectUploadExtension(input.FileName, input.ContentType)
	uniqueSuffix, err := randomUploadSuffix()
	if err != nil {
		return "", err
	}
	return input.Category + "/" + input.Now.Format("20060102") + "/" + fileTime + "_" + hex.EncodeToString(fileHash[:])[:16] + "_" + uniqueSuffix + extension, nil
}

func randomUploadSuffix() (string, error) {
	buffer := make([]byte, 8)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

func detectUploadContentType(fileContent []byte) string {
	return http.DetectContentType(fileContent)
}

func detectUploadExtension(fileName string, contentType string) string {
	switch contentType {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/webp":
		return ".webp"
	}
	extension := strings.ToLower(filepath.Ext(fileName))
	if extension != "" {
		return extension
	}
	return ".bin"
}

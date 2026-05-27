package v1

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"io"
	"mime"
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

type uploadResponse struct {
	Key         string `json:"key"`
	URL         string `json:"url"`
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
}

func (h uploadHandler) create(c *gin.Context) {
	fileHeader, err := c.FormFile("file")
	if err != nil {
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

	contentType := detectUploadContentType(fileHeader.Header.Get("Content-Type"), fileHeader.Filename)
	objectKey, err := buildUploadObjectKey(category, fileHeader.Filename, contentType, fileContent, h.now())
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

func buildUploadObjectKey(category string, fileName string, contentType string, fileContent []byte, now time.Time) (string, error) {
	fileTime := now.Format("20060102150405")
	fileHash := md5.Sum(fileContent)
	extension := detectUploadExtension(fileName, contentType)
	return category + "/" + now.Format("20060102") + "/" + fileTime + "_" + hex.EncodeToString(fileHash[:])[:16] + extension, nil
}

func detectUploadContentType(headerValue string, fileName string) string {
	if headerValue != "" {
		return headerValue
	}
	if contentType := mime.TypeByExtension(filepath.Ext(fileName)); contentType != "" {
		return contentType
	}
	return "application/octet-stream"
}

func detectUploadExtension(fileName string, contentType string) string {
	if contentType == "image/webp" {
		return ".webp"
	}
	extension := strings.ToLower(filepath.Ext(fileName))
	if extension != "" {
		return extension
	}
	return ".bin"
}

package v1

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	serviceexam "github.com/lifei6671/papermind/server/internal/service/exam"
	"github.com/lifei6671/papermind/server/library/code"
	"github.com/lifei6671/papermind/server/library/response"
)

type examEntryTokenPayload struct {
	TenantID  uint64 `json:"tenant_id"`
	ExamToken string `json:"exam_token"`
}

// examEntryTokenMiddleware 在进入答题写 handler 前校验一次性考试 token。
//
// 这里故意不复用后台登录 session：exam_token 只能生成 ExamEntryContext，
// 不能升级成租户管理端身份，也不能访问 profile、题库、试卷、阅卷或成绩管理接口。
func examEntryTokenMiddleware(taking *serviceexam.TakingService) gin.HandlerFunc {
	return func(c *gin.Context) {
		attemptID, err := readUintParam(c, "attempt_id")
		if err != nil {
			c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "作答 ID 必须是正整数"))
			c.Abort()
			return
		}
		rawBody, err := c.GetRawData()
		if err != nil {
			c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "读取请求体失败"))
			c.Abort()
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(rawBody))
		var payload examEntryTokenPayload
		if err := json.Unmarshal(rawBody, &payload); err != nil {
			c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "请求体不是合法 JSON"))
			c.Abort()
			return
		}
		if payload.TenantID == 0 || payload.ExamToken == "" {
			c.JSON(http.StatusBadRequest, response.Fail(code.InvalidParam, "tenant_id 和 exam_token 不能为空"))
			c.Abort()
			return
		}
		entryContext, err := taking.ValidateExamEntryToken(c.Request.Context(), payload.TenantID, attemptID, payload.ExamToken)
		if err != nil {
			writeTakingServiceError(c, err)
			c.Abort()
			return
		}
		c.Set(examEntryContextKey, entryContext)
		c.Next()
	}
}

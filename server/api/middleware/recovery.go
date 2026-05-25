package middleware

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/papermind/server/library/code"
	"github.com/lifei6671/papermind/server/library/response"
)

func Recovery() gin.HandlerFunc {
	return gin.CustomRecovery(func(c *gin.Context, recovered any) {
		slog.Error(
			"panic recovered",
			"request_id", c.GetString(RequestIDKey),
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"panic", recovered,
		)
		c.AbortWithStatusJSON(
			http.StatusInternalServerError,
			response.Fail(code.InternalError, code.Message(code.InternalError)),
		)
	})
}

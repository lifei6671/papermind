package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lifei6671/papermind/server/bootstrap/migration"
)

type MigrationStatusProvider interface {
	Status() migration.Status
}

func MigrationGuard(provider MigrationStatusProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		if provider == nil {
			c.Next()
			return
		}

		status := provider.Status()
		if status.State != migration.StateRunning {
			c.Next()
			return
		}

		// 数据库迁移期间业务接口不可用。浏览器页面返回中间页，API 请求返回稳定 JSON，前端可据此展示升级状态并轮询。
		c.Header("Retry-After", "5")
		if wantsHTML(c.GetHeader("Accept")) {
			c.Data(http.StatusServiceUnavailable, "text/html; charset=utf-8", []byte(migrationHTML))
			c.Abort()
			return
		}
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{
			"code":    http.StatusServiceUnavailable,
			"message": "系统升级中，请稍后重试",
			"data": gin.H{
				"state": status.State,
			},
		})
	}
}

func wantsHTML(accept string) bool {
	return strings.Contains(strings.ToLower(accept), "text/html")
}

const migrationHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta http-equiv="refresh" content="5">
  <title>系统升级中</title>
  <style>
    body { margin: 0; font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; background: #f6f7f9; color: #1f2937; }
    main { min-height: 100vh; display: grid; place-items: center; padding: 24px; box-sizing: border-box; }
    section { max-width: 520px; width: 100%; background: #fff; border: 1px solid #e5e7eb; border-radius: 8px; padding: 28px; box-shadow: 0 12px 36px rgba(15, 23, 42, .08); }
    h1 { margin: 0 0 12px; font-size: 22px; line-height: 1.35; }
    p { margin: 0; line-height: 1.7; color: #4b5563; }
  </style>
</head>
<body>
  <main>
    <section>
      <h1>系统升级中</h1>
      <p>数据库正在迁移，页面会自动刷新。请稍后继续使用。</p>
    </section>
  </main>
</body>
</html>`

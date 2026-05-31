package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"

	"github.com/lifei6671/papermind/server/internal/service/permission"
	serviceplatformuser "github.com/lifei6671/papermind/server/internal/service/platformuser"
	servicetenantuser "github.com/lifei6671/papermind/server/internal/service/tenantuser"
	"github.com/lifei6671/papermind/server/library/code"
	"github.com/lifei6671/papermind/server/library/response"
)

const (
	// AuthPrincipalGinKey 是 Gin Context 中保存当前认证主体的键。
	AuthPrincipalGinKey = "auth_principal"

	// AuthSessionSubjectTypeKey 记录当前 session 的主体类型，例如平台用户或租户用户。
	AuthSessionSubjectTypeKey = "actor_type"
	// AuthSessionUserIDKey 记录当前 session 的用户 ID。
	AuthSessionUserIDKey = "actor_id"
	// AuthSessionTenantIDKey 记录当前 session 绑定的租户 ID；平台用户和未选租户时为 0。
	AuthSessionTenantIDKey = "tenant_id"
	// AuthSessionRoleKey 记录当前 session 的角色快照，具体接口仍可按需刷新数据库状态。
	AuthSessionRoleKey = "role"
)

type authPrincipalContextKey struct{}

// AuthPrincipal 表示从认证 session 解析出的当前请求主体。
type AuthPrincipal struct {
	SubjectType string `json:"subject_type"`        // 主体类型：平台用户或租户用户。
	UserID      uint64 `json:"user_id"`             // 当前主体的用户 ID。
	TenantID    uint64 `json:"tenant_id,omitempty"` // 当前租户 ID；平台用户或未进入租户时为空。
	Role        string `json:"role,omitempty"`      // 当前 session 中的角色快照。
}

// BearerSessionCookie 将 Authorization Bearer 中的 session 值映射为请求 cookie。
//
// 浏览器默认走 HttpOnly cookie；测试或兼容调用方可通过 Bearer 传入同一签名 session。
func BearerSessionCookie(sessionName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := strings.TrimSpace(c.GetHeader("Authorization"))
		if header == "" {
			c.Next()
			return
		}
		token, ok := bearerToken(header)
		if ok {
			replaceRequestCookie(c.Request, sessionName, token)
		}
		c.Next()
	}
}

// AuthContext 从 Gin session 中解析认证主体，并写入 Gin Context 与标准 context。
//
// handler 可通过 CurrentAuthPrincipal 读取，service 可通过 PrincipalFromContext 读取。
func AuthContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, ok := PrincipalFromGinSession(c)
		if ok {
			c.Set(AuthPrincipalGinKey, principal)
			ctx := context.WithValue(c.Request.Context(), authPrincipalContextKey{}, principal)
			c.Request = c.Request.WithContext(ctx)
		}
		c.Next()
	}
}

// PrincipalFromGinSession 从 Gin session 的基础字段中还原认证主体。
//
// session 使用多个基础字段存储，避免直接序列化结构体带来的跨 store 兼容问题。
func PrincipalFromGinSession(c *gin.Context) (AuthPrincipal, bool) {
	session := sessions.Default(c)
	subjectType, ok := session.Get(AuthSessionSubjectTypeKey).(string)
	if !ok || subjectType == "" {
		return AuthPrincipal{}, false
	}
	userID, err := cast.ToE[uint64](session.Get(AuthSessionUserIDKey))
	if err != nil || userID == 0 {
		return AuthPrincipal{}, false
	}
	tenantID, _ := cast.ToE[uint64](session.Get(AuthSessionTenantIDKey))
	role, _ := session.Get(AuthSessionRoleKey).(string)
	return AuthPrincipal{
		SubjectType: subjectType,
		UserID:      userID,
		TenantID:    tenantID,
		Role:        role,
	}, true
}

// PrincipalFromContext 从标准 context 中读取认证主体。
func PrincipalFromContext(ctx context.Context) (AuthPrincipal, bool) {
	principal, ok := ctx.Value(authPrincipalContextKey{}).(AuthPrincipal)
	return principal, ok
}

// CurrentAuthPrincipal 返回当前请求主体，优先读取 Gin Context，再回退到标准 context。
func CurrentAuthPrincipal(c *gin.Context) (AuthPrincipal, bool) {
	if value, ok := c.Get(AuthPrincipalGinKey); ok {
		if principal, ok := value.(AuthPrincipal); ok && principal.UserID != 0 {
			return principal, true
		}
	}
	if principal, ok := PrincipalFromContext(c.Request.Context()); ok && principal.UserID != 0 {
		return principal, true
	}
	return AuthPrincipal{}, false
}

// RequireAuthPrincipal 要求请求已经登录任意有效主体。
func RequireAuthPrincipal() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := CurrentAuthPrincipal(c); !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, response.Fail(code.InvalidParam, "请先登录"))
			return
		}
		c.Next()
	}
}

// RequirePlatformPrincipal 要求当前请求来自平台用户 session。
func RequirePlatformPrincipal() gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, ok := CurrentAuthPrincipal(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, response.Fail(code.InvalidParam, "请先登录平台管理员账号"))
			return
		}
		if principal.SubjectType != permission.SubjectPlatformUser {
			c.AbortWithStatusJSON(http.StatusForbidden, response.Fail(code.InvalidParam, "仅平台管理员可操作"))
			return
		}
		c.Next()
	}
}

// RequireLivePlatformPrincipal 要求当前请求来自仍处于启用状态的平台管理员。
//
// 该中间件会重新读取平台用户状态，避免账号禁用后旧 session 继续访问平台接口。
func RequireLivePlatformPrincipal(platformUsers *serviceplatformuser.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, ok := CurrentAuthPrincipal(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, response.Fail(code.InvalidParam, "请先登录平台管理员账号"))
			return
		}
		if principal.SubjectType != permission.SubjectPlatformUser {
			c.AbortWithStatusJSON(http.StatusForbidden, response.Fail(code.InvalidParam, "仅平台管理员可操作"))
			return
		}
		if platformUsers != nil {
			user, err := platformUsers.Get(c.Request.Context(), principal.UserID)
			if err != nil || user.Status != serviceplatformuser.StatusEnabled {
				c.AbortWithStatusJSON(http.StatusForbidden, response.Fail(code.InvalidParam, "平台管理员账号已禁用"))
				return
			}
		}
		c.Next()
	}
}

// RequireLiveTenantAdminOrPlatformPrincipal 要求当前请求来自启用的平台管理员或租户管理员。
//
// 上传等跨域入口既允许平台管理员，也允许当前租户的启用租户管理员。
func RequireLiveTenantAdminOrPlatformPrincipal(platformUsers *serviceplatformuser.Service, tenantUsers *servicetenantuser.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, ok := CurrentAuthPrincipal(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, response.Fail(code.InvalidParam, "请先登录"))
			return
		}
		if principal.SubjectType == permission.SubjectPlatformUser && principal.Role == permission.RolePlatformAdmin {
			if platformUsers != nil {
				user, err := platformUsers.Get(c.Request.Context(), principal.UserID)
				if err != nil || user.Status != serviceplatformuser.StatusEnabled {
					c.AbortWithStatusJSON(http.StatusForbidden, response.Fail(code.InvalidParam, "无权执行当前操作"))
					return
				}
			}
			c.Next()
			return
		}
		if principal.SubjectType == permission.SubjectTenantUser && principal.Role == permission.RoleTenantAdmin && principal.TenantID != 0 {
			if tenantUsers != nil {
				user, err := tenantUsers.Get(c.Request.Context(), principal.TenantID, principal.UserID)
				if err != nil || user.Status != servicetenantuser.StatusEnabled || user.Role != servicetenantuser.RoleTenantAdmin {
					c.AbortWithStatusJSON(http.StatusForbidden, response.Fail(code.InvalidParam, "无权执行当前操作"))
					return
				}
			}
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusForbidden, response.Fail(code.InvalidParam, "无权执行当前操作"))
	}
}

// RequireExamBusinessPrincipal 要求当前请求来自已进入租户的考试业务主体。
//
// 这里只做 session 角色快照的粗筛；具体空间、禁用状态和业务范围由 handler/service 再校验。
func RequireExamBusinessPrincipal() gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, ok := CurrentAuthPrincipal(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, response.Fail(code.InvalidParam, "请先登录"))
			return
		}
		if principal.SubjectType == permission.SubjectTenantUser &&
			principal.TenantID != 0 &&
			validTenantExamSessionRole(principal.Role) {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusForbidden, response.Fail(code.InvalidParam, "无权执行当前操作"))
	}
}

// replaceRequestCookie 用 Bearer session 覆盖同名 cookie，避免旧 cookie 影响本次认证。
func replaceRequestCookie(request *http.Request, name string, value string) {
	cookies := request.Cookies()
	request.Header.Del("Cookie")
	for _, cookie := range cookies {
		if cookie.Name != name {
			request.AddCookie(cookie)
		}
	}
	// 兼容已持有签名 session 值的调用方，进入 Gin session middleware 前统一映射成 cookie。
	request.AddCookie(&http.Cookie{Name: name, Value: value})
}

// bearerToken 解析 Authorization: Bearer <token> 形式的 session token。
func bearerToken(header string) (string, bool) {
	const prefix = "Bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", false
	}
	token := strings.TrimSpace(header[len(prefix):])
	return token, token != ""
}

// validTenantExamSessionRole 判断 session 角色是否可能访问考试业务入口。
func validTenantExamSessionRole(role string) bool {
	return role == permission.RoleTenantAdmin || role == permission.RoleTeacher || role == permission.RoleStudent
}

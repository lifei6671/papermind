# PaperMind 租户管理员权限模型 P0 审计记录

> 对应清单：`docs/2025-05-28-papermind-tenant-admin-permission-execution-checklist.md`
>
> 审计时间：2026-05-28
>
> 审计范围：P0.1 冲突扫描、P0.2 任务边界确认。本文只记录改造前基线，不表示冲突已经修复。

## 1. 本轮结论

- 旧 `user_roles` 多角色唯一约束仍在三种迁移和迁移测试中存在，后续由 P1.1 修复。
- 后端 `PermissionContext`、`PermissionChecker` 和部分考试服务仍保留旧权限模型痕迹，后续由 P2/P3/P6 修复。
- 前端仍把 `space_admin` 当作 session/menu role 使用，后续由 P7.1 修复。
- `devseed` 会把演示租户管理员写入演示空间成员，但正式创建租户流程当前只创建租户记录，不会自动创建默认空间或空间成员；后续由 P4.1 补齐首个租户管理员事务创建，并清理 seed 冲突。
- `/api/v1/exam-entry/results/:id` 学生查分入口当前未发现路由实现，学生查分边界后续由 P5.4/P7.3 补齐。
- 当前未提交文件只有文档改动，未发现生产配置、密钥或私有配置进入本轮变更。

## 2. P0.1 冲突扫描

### 2.1 `user_roles` 仍是三列唯一约束

当前迁移仍允许同一租户同一用户拥有多条不同租户级角色记录，与首版单角色模型冲突。

- `server/data/migrations/postgres/001_tenant_space.sql:262`
- `server/data/migrations/mysql/001_tenant_space.sql:152`
- `server/data/migrations/sqlite/001_tenant_space.sql:166`
- `server/bootstrap/migration/user_role_schema_test.go:31`
- `server/bootstrap/migration/user_role_schema_test.go:49`
- `server/bootstrap/migration/user_role_schema_test.go:67`

后续关联：P1.1 需要把索引或约束改为 `(tenant_id, user_id)`，并同步迁移测试。

### 2.2 后端权限服务仍保留旧接口和复数角色上下文

旧接口和旧上下文不只是文档引用，生产代码仍有调用或依赖。

- `server/internal/service/permission/checker.go:6`：`CanManageTenant`
- `server/internal/service/permission/checker.go:7`：`CanManageSpace`
- `server/internal/service/permission/checker.go:10`：`CanGradeExam`
- `server/internal/service/permission/context.go:20`：`TenantRoles []string`
- `server/internal/service/permission/context.go:21`：`SpaceRoles map[uint64]string`
- `server/api/v1/router.go:872`：生产路径调用 `CanGradeExam`
- `server/api/v1/router.go:1030`：构造 `PermissionContext.SpaceRoles`
- `server/api/v1/router.go:1055`：从空间成员关系写入 `ctx.SpaceRoles`
- `server/internal/service/exam/export_service.go:106`：导出成绩时调用 `CanGradeExam`
- `server/internal/service/exam/review_service.go:172`：阅卷列表直接遍历 `ctx.SpaceRoles`

后续关联：P2.1 收敛认证上下文；P3.1 替换接口签名；P6.2/P6.3 按资源真实归属重做成绩、阅卷授权。

### 2.3 前端仍把 `space_admin` 当作 session/menu role

菜单和直接路由访问当前仍直接依赖 `session.user.role`。

- `web/src/app/routes.tsx:47`：`examBusinessRoles = ["space_admin", "teacher"]`
- `web/src/app/routes.tsx:92`：空间管理菜单依赖 `menuRoles: ["space_admin"]`
- `web/src/app/routes.tsx:107`：用户管理菜单依赖 `menuRoles: ["space_admin"]`
- `web/src/app/routes.tsx:185`：菜单可见性直接判断 `route.menuRoles.includes(role)`
- `web/src/app/routes.tsx:190`：`routeActorRole` 接受 `space_admin`
- `web/src/layouts/AdminShell/AdminShell.tsx:23`：侧边栏按 `session.user.role` 过滤菜单
- `web/src/layouts/AdminShell/AdminShell.test.tsx:49`
- `web/src/layouts/AdminShell/AdminShell.test.tsx:103`

`SpaceManagementPage`、`TenantManagementPage`、`spaces` API 测试中的 `space_admin` 多数是空间成员角色，后续 P7.1 处理时需要保留 `role_in_space` 语义，不能一刀切删除。

### 2.4 `tenant_admin` 自动写入 `space_members` 的现状

正式租户创建流程当前没有自动创建默认空间，也没有写入 `space_members`。

- `server/api/v1/router.go:1328`：租户创建 handler 只调用 `tenantService.Create`
- `server/internal/service/tenant/service.go:153`
- `server/internal/service/tenant/service.go:162`
- `server/internal/dao/db/tenant_repository.go:68`

但 `devseed` 会创建演示空间，并把演示租户管理员写入演示空间成员。

- `server/bootstrap/devseed/devseed.go:135`：演示租户管理员写入 `user_roles`
- `server/bootstrap/devseed/devseed.go:142`：创建演示空间
- `server/bootstrap/devseed/devseed.go:164`：演示租户管理员写入 `space_members`，角色为 `space_admin`

后续关联：P4.1 补齐正式租户创建时首个 `tenant_admin` 的同事务创建；P4.1 同时清理 devseed 与首版规则的冲突。

### 2.5 学生查分入口尚未拆分

当前未发现 `/api/v1/exam-entry/results/:id` 路由。现有成绩接口仍集中在管理端结果接口。

- `server/api/v1/router.go:135`：`GET /api/v1/results`
- `server/api/v1/router.go:136`：`POST /api/v1/results/publish-config`
- `server/api/v1/router.go:137`：`POST /api/v1/results/export`
- `web/src/api/results.ts:69`：前端成绩列表调用 `/api/v1/results`
- `web/src/api/results.ts:84`：前端成绩导出调用 `/api/v1/results/export`

后续关联：P5.4 新增学生查分入口；P7.3 前端学生查分改走 `/api/v1/exam-entry/results/:id`。

## 3. P0.2 任务边界确认

首版边界已按权限模型文档确认：

- 只做单租户级角色，不实现多角色叠加。
- 不做复杂 RBAC 权限配置 UI。
- 不做平台管理员 impersonation。
- session 主动 revoke 是增强项，不阻塞首版；关键写接口必须实时从数据库重建权限。
- 公共题库和公共试卷首版只允许 `tenant_admin` 写入。

对应依据：

- `docs/2025-05-28-papermind-tenant-admin-permission-model.md:9`
- `docs/2025-05-28-papermind-tenant-admin-permission-model.md:17`
- `docs/2025-05-28-papermind-tenant-admin-permission-model.md:27`
- `docs/2025-05-28-papermind-tenant-admin-permission-model.md:769`
- `docs/2025-05-28-papermind-tenant-admin-permission-model.md:863`
- `docs/2025-05-28-papermind-tenant-admin-permission-model.md:980`
- `docs/2025-05-28-papermind-tenant-admin-permission-model.md:983`

## 4. 后续推进顺序

P0 已完成只读审计和边界冻结。下一步进入 P1 时应先处理数据库约束和迁移测试，不应同时启动 P2+ 的认证上下文改造，避免数据库语义和代码语义交叉漂移。

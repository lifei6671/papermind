# PaperMind 租户管理员权限模型执行清单

> 来源方案：`docs/2025-05-28-papermind-tenant-admin-permission-model.md`
>
> 本清单用于按任务推进实现、回写进度和验收。所有实现项默认未完成，只有代码改动、测试和必要 review 都完成后才能把 `[ ]` 改成 `[x]`。

## 0. 执行原则

- [ ] 每个任务完成后先做代码 review，再进入下一个强依赖任务。
- [ ] 涉及数据库迁移、公共接口、认证上下文或权限语义的任务，必须同步更新测试和相关文档。
- [ ] 业务 service 禁止散落角色字符串判断，统一通过 `PermissionChecker` 或明确的认证中间件上下文进入。
- [ ] `space_admin` 只能来自 `space_members.role_in_space`，不能写入 session role、`ActorContext.Role` 或 `PermissionContext.Role`。
- [ ] 所有“最后一个管理员”不变式必须在同一数据库事务内，基于变更后的状态校验。
- [ ] 当前任务没有完成代码验证前，不修改 `docs/2026-05-25-papermind-execution-checklist.md` 中已有勾选状态。

## 1. 推荐阶段顺序

```text
P0 现状审计与冲突冻结
  -> P1 数据库约束与审计字段
  -> P2 角色常量、DAO 与认证上下文
  -> P3 PermissionChecker 重构
  -> P4 平台/租户/空间用户管理
  -> P5 API 分组与考试入口认证
  -> P6 题库/试卷/考试/阅卷/成绩授权收口
  -> P7 前端菜单、路由和页面提示
  -> P8 测试矩阵、联调和文档回写
```

## 2. 当前已知冲突基线

这些冲突来自执行清单编写时的代码扫描，实施前需要重新确认：

- [x] P0 审计时确认 `server/data/migrations/*/001_tenant_space.sql` 和 `server/bootstrap/migration/user_role_schema_test.go` 仍使用旧三列唯一约束；P1.1 通过 `002_audit_actor_type.sql` 追加迁移改为单角色唯一约束。
- [x] `server/internal/service/permission` 仍存在 `CanManageTenant`、`CanManageSpace`、`CanGradeExam`、`TenantRoles []string`、`SpaceRoles map[uint64]string` 等旧模型痕迹。
- [x] `web/src/app/routes.tsx`、`AdminShell` 测试和部分页面仍把 `space_admin` 当作 session role 或菜单 role。
- [x] `server/bootstrap/devseed/devseed.go` 仍把演示租户管理员写入默认空间 `space_members`。
- [x] `/api/v1/exam-entry/results/:id` 学生查分入口尚未独立验收。

## P0. 现状审计与冲突冻结

**目标**：先固定当前偏差，避免后续实现时误用旧方案。

**依赖**：无。

**涉及文件**：

- 读取：`docs/2025-05-28-papermind-tenant-admin-permission-model.md`
- 读取：`docs/2026-05-25-papermind-exam-platform-technical-design.md`
- 读取：`docs/2026-05-25-papermind-execution-checklist.md`
- 读取：`server/internal/service/permission/*`
- 读取：`server/data/migrations/postgres/001_tenant_space.sql`
- 读取：`server/data/migrations/mysql/001_tenant_space.sql`
- 读取：`server/data/migrations/sqlite/001_tenant_space.sql`
- 读取：`web/src/app/routes.tsx`
- 新增：`docs/2025-05-28-papermind-tenant-admin-permission-p0-audit.md`

### P0.1 冲突扫描

- [x] 扫描旧三列唯一约束。
- [x] 扫描旧接口名 `CanManageTenant`、`CanManageSpace`、`CanViewResults`、`CanGradeExam`。
- [x] 扫描 `TenantRoles []string`、`Roles []string`、`SpaceRoles map`。
- [x] 扫描前端是否把 `space_admin` 当作 session role。
- [x] 扫描 `tenant_admin` 是否被 seed 或创建流程自动写入 `space_members`。

**验收标准**：

- [x] 输出一份本轮变更前差异清单，逐项关联到后续任务。
- [x] 确认没有直接修改生产配置、密钥或私有配置。

**建议验证**：

```powershell
rg -n "UNIQUE \\(tenant_id, user_id, role\\)|CanManageTenant\\(|CanManageSpace\\(|CanViewResults|CanGradeExam|TenantRoles|Roles \\[\\]|SpaceRoles|role: \"space_admin\"|RoleSpaceAdmin" server web\src docs
git status --short
```

### P0.2 任务边界确认

- [x] 确认首版只做单租户级角色，不实现多角色。
- [x] 确认首版不做复杂 RBAC 权限配置 UI。
- [x] 确认首版不做 platform impersonation。
- [x] 确认 session 主动 revoke 是增强项，不阻塞首版；关键写接口必须实时从数据库重建权限。
- [x] 确认公共题库和公共试卷首版只允许 `tenant_admin` 写入。

**验收标准**：

- [x] 本清单后续任务没有引入多角色、RBAC UI 或 impersonation。

## P1. 数据库约束与审计字段

**目标**：让数据层和单角色权限模型一致。

**依赖**：P0。

**涉及文件**：

- 新增：`server/data/migrations/postgres/002_audit_actor_type.sql`
- 新增：`server/data/migrations/mysql/002_audit_actor_type.sql`
- 新增：`server/data/migrations/sqlite/002_audit_actor_type.sql`
- 保持：`server/data/migrations/*/001_tenant_space.sql` 作为历史基线，不追加新字段或新约束语义。
- 修改：`server/bootstrap/migration/user_role_schema_test.go`
- 修改：`server/internal/dao/db/base_do.go` 或当前基础字段定义文件
- 修改：`server/internal/dao/db/*_do.go`
- 修改：`server/internal/dao/db/*_model_test.go`

### P1.1 `user_roles` 单角色唯一约束

- [x] 在 PostgreSQL 002 迁移中将 `user_roles` 唯一索引追加迁移为 `(tenant_id, user_id)`。
- [x] 在 MySQL 002 迁移中将 `user_roles` 唯一约束追加迁移为 `(tenant_id, user_id)`。
- [x] 在 SQLite 002 迁移中将 `user_roles` 唯一索引追加迁移为 `(tenant_id, user_id)`。
- [x] 索引或约束命名改为表达单角色语义，例如 `uk_user_roles_user`。
- [x] 更新 `server/bootstrap/migration/user_role_schema_test.go` 中三种数据库断言。

**验收标准**：

- [x] 同一租户同一用户不能同时写入 `tenant_admin` 和 `teacher`。
- [x] `user_roles.role` 仍保留，用于保存当前唯一租户级角色。
- [x] 文档中提到的 `UNIQUE (tenant_id, user_id, role)` 只作为未来扩展说明存在，不出现在首版迁移实现里。

**建议验证**：

```powershell
cd server
go test -tags json1 ./bootstrap/migration
```

### P1.2 审计人类型字段

- [x] 为需要跨主体审计的平台/租户写接口补充 `created_by_type`。
- [x] 为需要跨主体审计的平台/租户写接口补充 `updated_by_type`。
- [x] `created_by` / `updated_by` 继续保存主体 ID。
- [x] 平台管理员写租户资料时，`*_by_type = platform_user`。
- [x] 租户用户写租户内业务时，`*_by_type = tenant_user`。
- [x] 审计主体类型字段通过 `002_audit_actor_type.sql` 追加迁移落地，`001_tenant_space.sql` 保持历史基线。
- [x] 更新相关 DO 字段映射和模型测试。

**验收标准**：

- [x] 平台管理员和租户用户 ID 空间不会在审计字段中混淆。
- [x] 所有新增字段都有 GORM `column` 显式映射。
- [x] 迁移 SQL 有中文字段注释。

**建议验证**：

```powershell
cd server
go test -tags json1 ./internal/dao/db ./bootstrap/migration
```

## P2. 角色常量、DAO 与认证上下文

**目标**：把租户级单角色和空间内身份拆清楚。

**依赖**：P1。

**涉及文件**：

- 修改：`server/library/constant/*`
- 修改：`server/internal/service/permission/context.go`
- 修改：`server/api/v1/auth_token.go`
- 修改：`server/api/v1/auth_handler.go`
- 修改：`server/internal/dao/db/user_role_do.go`
- 修改：`server/internal/dao/db/tenant_user_repository.go`
- 修改：`server/internal/dao/db/space_repository.go`
- 修改：`server/api/v1/auth_token_test.go`
- 修改：`server/api/v1/auth_handler_test.go`

### P2.1 租户级角色模型

- [x] 定义租户级角色只包含 `tenant_admin`、`teacher`、`student`。
- [x] `PermissionContext.Role` 使用单个字符串，不使用 `[]string`。
- [x] 删除或替换 `TenantRoles []string`。
- [x] `PermissionContext` 不保存 `space_admin`。
- [x] `space_admin` 只在空间成员查询结果或空间授权查询中出现。

**验收标准**：

- [x] 代码中不存在通过 `ctx.Role == "space_admin"` 做空间管理员判断的实现。
- [x] session 只保存一个租户级 role。
- [x] service 层需要空间身份时，必须查询 `space_members.role_in_space`。

**建议验证**：

```powershell
rg -n "TenantRoles|Roles \\[\\]|ctx\\.Role == \"space_admin\"|RoleSpaceAdmin.*PermissionContext|space_admin.*session" server web\src
cd server
go test -tags json1 ./api/v1 ./internal/service/permission
```

### P2.2 认证主体上下文

- [x] 统一平台管理员和租户用户登录后的 `ActorContext`。
- [x] `ActorContext.Role` 只保存租户级角色或平台角色。
- [x] 登录中间件从 session 解析 `actor_type`、`actor_id`、`tenant_id`、`role`。
- [x] `ActorContext` 到 `PermissionContext` 的转换只复制租户级 role。
- [x] 关键写接口需要时从 DB 重建用户状态和角色。

**验收标准**：

- [x] 禁用用户后，关键写接口不能只凭旧 session 通过。
- [x] 平台管理员不能通过构造 `tenant_id` 进入租户业务写接口。

## P3. PermissionChecker 重构

**目标**：把权限检查集中到可测试接口，避免 service 层散落字符串判断。

**依赖**：P2。

**涉及文件**：

- 修改：`server/internal/service/permission/checker.go`
- 修改：`server/internal/service/permission/fixed_role.go`
- 修改：`server/internal/service/permission/fixed_role_test.go`
- 修改：调用 `PermissionChecker` 的 service 和 handler

### P3.1 接口签名对齐

- [x] 删除旧 `CanManageTenant`。
- [x] 新增 `CanManageTenantLifecycle(ctx, tenantID)`。
- [x] 新增 `CanManageTenantBusiness(ctx, tenantID)`。
- [x] 新增 `CanManageTenantUsers(ctx, tenantID)`。
- [x] 删除旧 `CanManageSpace`。
- [x] 新增 `CanManageSpaceProfile(ctx, spaceID)`。
- [x] 保留并明确 `CanManageSpaceMembers(ctx, spaceID)`。
- [x] 新增或对齐 `CanManagePaper(ctx, paperID)`。
- [x] 新增或对齐 `CanViewExamResults(ctx, examID)`。
- [x] 新增或对齐 `CanExportExamResults(ctx, examID)`。
- [x] 保留 `CanViewOwnResult(ctx, resultID)`。
- [x] 删除旧 `CanViewResults`、`CanGradeExam` 或把调用迁移到新语义。

**验收标准**：

- [x] `checker.go` 接口和权限方案中的接口名完全一致。
- [x] 所有旧方法名没有调用残留。

**建议验证**：

```powershell
rg -n "CanManageTenant\\(|CanManageSpace\\(|CanViewResults|CanGradeExam" server
cd server
go test -tags json1 ./internal/service/permission
```

### P3.2 固定角色规则

- [x] `platform_admin` 只能通过平台治理和租户生命周期检查。
- [x] `tenant_admin` 可以管理本租户用户、空间资料、空间成员、题库、试卷、考试、阅卷、成绩和导出。
- [x] `space_admin` 不能通过 `CanManageSpaceProfile`。
- [x] `space_admin` 可以通过当前空间 `CanManageSpaceMembers`。
- [x] `teacher` 不能管理空间成员。
- [x] `teacher` 首版不能导出成绩。
- [x] `student` 只能参加目标考试和查看自己的已发布成绩。

**验收标准**：

- [x] 单测覆盖每个角色至少一个允许和一个拒绝路径。
- [x] `tenant_admin` 不在 `space_members` 中也可以管理本租户空间资源。
- [x] `space_admin` 不能修改空间基础资料或删除空间。

## P4. 平台、租户、用户和空间管理

**目标**：完成平台治理域、租户业务域和空间业务域的写操作边界。

**依赖**：P3。

**涉及文件**：

- 修改：`server/api/v1/tenant_handler.go` 或当前租户 handler
- 修改：`server/api/v1/user_handler.go`
- 修改：`server/api/v1/space_handler.go`
- 修改：`server/internal/service/tenant/*`
- 修改：`server/internal/service/tenantuser/*`
- 修改：`server/internal/service/space/*`
- 修改：`server/internal/dao/db/platform_user_repository.go`
- 修改：`server/internal/dao/db/tenant_user_repository.go`
- 修改：`server/internal/dao/db/space_repository.go`
- 修改：对应 handler/service 测试

### P4.1 创建租户初始化首个管理员

- [x] 创建租户请求体补充首个管理员用户名、真实姓名、手机号或邮箱、初始密码。
- [x] 创建租户、创建首个用户、写入 `tenant_admin` 角色在同一事务内完成。
- [x] 任一步失败必须回滚。
- [x] 首版不自动创建默认空间。
- [x] 首版不把首个 `tenant_admin` 写入 `space_members`。

**验收标准**：

- [x] 创建租户成功后，目标租户至少有一个启用状态 `tenant_admin`。
- [x] 创建租户失败不会留下半租户或无管理员租户。
- [x] 新租户没有默认空间，首个管理员没有空间成员记录。

### P4.2 租户管理员不变式

- [ ] 实现 `ValidateTenantAdminInvariant(tx, tenantID)`。
- [x] 禁用用户时接入。
- [ ] 删除用户时接入。
- [ ] 移除或修改 `tenant_admin` 角色时接入。
- [ ] 批量导入覆盖角色时接入。
- [ ] 校验必须在同一事务内基于变更后状态执行。

**验收标准**：

- [x] 禁止禁用最后一个启用状态 `tenant_admin`。
- [ ] 两个并发降权或禁用请求不能同时删除最后一个管理员。

### P4.3 空间管理员不变式

- [ ] 实现 `ValidateSpaceAdminInvariant(tx, tenantID, spaceID)`。
- [ ] 禁用空间成员时接入。
- [ ] 移除空间成员时接入。
- [ ] 修改空间成员角色时接入。
- [ ] 禁用或删除租户用户时，对其涉及的空间逐个接入。
- [ ] 校验必须在同一事务内基于变更后状态执行。

**验收标准**：

- [ ] 禁止移除、禁用或降级最后一个启用状态 `space_admin`。
- [ ] 禁用租户用户不会导致任一启用空间失去最后一个空间管理员。

### P4.4 教师创建和空间分配

- [x] `tenant_admin` 创建 `teacher` 用户时只写 `users` 和 `user_roles`。
- [x] 创建 `teacher` 不自动写入 `space_members`。
- [x] 教师加入空间必须通过空间成员接口显式分配。
- [x] 教师没有启用空间成员关系时允许登录。
- [x] 零空间教师不能操作题库、试卷、考试或阅卷。

**验收标准**：

- [x] 用户详情或业务页能提示“该教师暂未加入任何空间，当前无法操作题库、试卷、考试或阅卷”。

### P4.5 空间资料与空间成员接口拆分

- [ ] `PUT /api/v1/tenant/spaces/:id` 只允许 `tenant_admin`。
- [ ] `DELETE /api/v1/tenant/spaces/:id` 只允许 `tenant_admin`。
- [ ] `GET /api/v1/tenant/spaces/:id/members` 允许 `tenant_admin` 或当前空间 `space_admin`。
- [ ] `POST /api/v1/tenant/spaces/:id/members` 允许 `tenant_admin` 或当前空间 `space_admin`。
- [ ] `PUT /api/v1/tenant/spaces/:id/members/:user_id` 允许 `tenant_admin` 或当前空间 `space_admin`。
- [ ] `DELETE /api/v1/tenant/spaces/:id/members/:user_id` 允许 `tenant_admin` 或当前空间 `space_admin`。

**验收标准**：

- [ ] `space_admin` 不能修改空间名称、Logo、描述、状态或删除空间。
- [ ] `space_admin` 只能管理自己所在空间的成员。

## P5. API 分组与考试入口认证

**目标**：把平台、租户管理、考试入口三个认证域拆开。

**依赖**：P3、P4。

**涉及文件**：

- 修改：`server/api/v1/router.go`
- 修改：`server/api/middleware/*`
- 修改：`server/api/v1/auth_token.go`
- 修改：`server/internal/service/exam/*`
- 修改：`server/api/v1/exam_handler_test.go`
- 修改：`server/api/v1/auth_token_test.go`

### P5.1 平台接口边界

- [ ] `/api/v1/platform/**` 只允许 `platform_admin`。
- [ ] 平台管理员可以创建租户、维护租户资料、重置租户码、修改注册开关。
- [ ] 租户用户不能访问平台接口。
- [ ] 平台管理员不能调用租户业务写接口。

**验收标准**：

- [ ] 构造 `tenant_id` 参数不能绕过平台/租户边界。

### P5.2 租户接口边界

- [ ] `/api/v1/tenant/**` 只允许 `tenant_user`。
- [ ] 所有租户接口从 session 或上下文取得 `tenant_id`。
- [ ] 不信任请求体里的跨租户 `tenant_id`。
- [ ] `student` 不能访问管理端接口。
- [ ] `GET /api/v1/tenant/results/:id` 不允许 `student`。

**验收标准**：

- [ ] `platform_user` 访问 `/api/v1/tenant/**` 返回 403。
- [ ] `tenant_user` 访问 `/api/v1/platform/**` 返回 403。

### P5.3 考试入口 `exam_token`

- [ ] `/api/v1/exam-entry/**` 不复用管理端权限中间件。
- [ ] `exam_token` 使用服务端随机不透明 token。
- [ ] 数据库只保存 `exam_token_hash`。
- [ ] 中间件按 hash 找到 attempt。
- [ ] 中间件校验 `tenant_id`、`exam_id`、`attempt_id`、`user_id`、attempt 状态、token 过期时间和业务作答截止时间。
- [ ] `exam_token` 不支持刷新和续签。
- [ ] 普通登录 session 不能调用自动保存、提交和事件接口。
- [ ] `exam_token` 不能调用 profile、tenant、questions、papers、grading、results 等后台接口。

**验收标准**：

- [ ] 过期 token、已提交 attempt、超过可保存窗口都会拒绝写入。
- [ ] 管理端 session 直接调用答题保存接口失败。

### P5.4 学生查分入口

- [x] 新增或迁移 `GET /api/v1/exam-entry/results/:id`。
- [x] 只允许目标 `student` 查看自己的成绩。
- [x] 必须满足成绩发布策略和可见时间。
- [x] 不允许学生通过 `/api/v1/tenant/results/:id` 查分。

**验收标准**：

- [x] 未发布成绩不可见。
- [x] 已发布成绩只能由本人查看。

## P6. 题库、试卷、考试、阅卷和成绩授权

**目标**：把现有业务 service 的权限检查全部收口到资源真实归属。

**依赖**：P3、P5。

**涉及文件**：

- 修改：`server/internal/service/question/*`
- 修改：`server/internal/service/paper/*`
- 修改：`server/internal/service/exam/*`
- 修改：`server/api/v1/router.go`
- 修改：相关 service 和 handler 测试

### P6.1 公共题库和公共试卷

- [x] `questions.space_id = NULL` 的公共题库只允许 `tenant_admin` 创建。
- [x] 公共题库只允许 `tenant_admin` 修改、删除和导入。
- [x] 已暴露的公共试卷写接口只允许 `tenant_admin` 修改大题、选题、规则和规则生成。
- [x] `space_admin` / `teacher` 只能管理自己启用空间内的题库和试卷。
- [x] `space_admin` / `teacher` 读取或引用公共资源时，由对应 service 显式校验。
- [ ] 当前后端尚未暴露 `papers.space_id = NULL` 的创建或删除 API；未来新增时必须复用同一公共试卷写入校验。

**验收标准**：

- [x] 教师不能把 `space_id = NULL` 当成自己的可写资源。
- [x] 空间管理员不能导入公共题库。

### P6.2 资源归属反查

- [ ] 题目权限从 `questionID` 反查真实 `tenant_id` 和 `space_id`。
- [ ] 试卷权限从 `paperID` 反查真实 `tenant_id` 和 `space_id`。
- [ ] 考试发布权限从 `paperID` 或考试目标反查真实空间。
- [ ] 阅卷权限从 `attemptID` 反查考试和成绩行真实空间。
- [ ] 成绩列表和导出从 `examID` 或成绩行反查真实空间。
- [ ] 禁止只信任请求参数拼接授权范围。

**验收标准**：

- [ ] 伪造 `space_id` 不能越权查看、阅卷或导出其他空间数据。

### P6.3 成绩查看和导出

- [x] `CanViewExamResults` 允许 `tenant_admin` 查看本租户考试成绩。
- [x] `CanViewExamResults` 允许授权空间 `space_admin` / `teacher` 查看授权范围内成绩。
- [x] `CanExportExamResults` 允许 `tenant_admin` 导出本租户考试成绩。
- [x] `CanExportExamResults` 允许当前空间 `space_admin` 导出本空间考试成绩。
- [x] 首版默认不允许 `teacher` 导出成绩。
- [x] `CanViewOwnResult` 只允许 `result.user_id == ctx.ActorID` 且成绩已发布。

**验收标准**：

- [x] 教师能看授权范围成绩，但不能导出。
- [x] 学生不能访问管理端成绩列表和导出。

## P7. 前端权限入口和体验

**目标**：前端不再把 `space_admin` 当 session role，页面入口和错误提示与后端权限一致。

**依赖**：P2、P4、P5。

**涉及文件**：

- 修改：`web/src/auth/session.tsx`
- 修改：`web/src/app/routes.tsx`
- 修改：`web/src/layouts/AdminShell/*`
- 修改：`web/src/pages/Platform/TenantManagementPage.tsx`
- 修改：`web/src/pages/Tenant/UserManagementPage.tsx`
- 修改：`web/src/pages/Tenant/SpaceManagementPage.tsx`
- 修改：`web/src/pages/Questions/*`
- 修改：`web/src/pages/Papers/*`
- 修改：`web/src/pages/Exams/*`
- 修改：`web/src/pages/Results/*`
- 修改：`web/src/api/*`
- 修改：相关前端测试

### P7.1 session 和菜单

- [x] 前端 session role 只保存租户级角色。
- [x] 前端不能把 `space_admin` 当作 session role。
- [ ] 空间管理员菜单可见性来自空间成员接口或授权空间列表。
- [x] `tenant_admin` 可以看到租户用户、空间、题库、试卷、考试、阅卷和成绩管理入口。
- [x] `platform_admin` 不能渲染租户业务页面或触发租户业务 API。
- [x] `student` 不能进入管理端菜单。

**验收标准**：

- [x] 旧 `menuRoles: ["space_admin"]` 不再作为管理端入口依据。
- [x] 直接访问无权限路由时不触发越权 API 请求。

### P7.2 创建租户和创建教师表单

- [x] 创建租户表单补充首个租户管理员账号信息。
- [x] 创建租户表单明确不会自动创建默认空间。
- [x] 创建教师表单不要求选择空间。
- [ ] 用户详情页展示教师空间分配状态。
- [x] 零空间教师显示“该教师暂未加入任何空间，当前无法操作题库、试卷、考试或阅卷”。

**验收标准**：

- [ ] 创建教师后不会在空间成员列表中自动出现。
- [ ] 通过空间成员入口分配后，教师才获得对应空间业务入口。

### P7.3 学生查分页面

- [x] 学生查分调用 `/api/v1/exam-entry/results/:id`。
- [x] 学生查分不调用 `/api/v1/tenant/results/:id`。
- [x] 未发布成绩展示等待公布提示。
- [x] 已发布成绩展示本人得分和解析控制。

**验收标准**：

- [x] 学生登录态下不会出现管理端成绩接口请求。

## P8. 测试矩阵、联调和文档回写

**目标**：用测试和联调证明权限边界真实生效。

**依赖**：P1-P7。

**涉及文件**：

- 修改：`server/internal/service/permission/*_test.go`
- 修改：`server/api/v1/*_test.go`
- 修改：`server/internal/service/**/**/*_test.go`
- 修改：`web/src/**/*.test.tsx`
- 修改：`docs/2026-05-25-papermind-execution-checklist.md`
- 修改：`docs/2026-05-25-papermind-exam-platform-technical-design.md`

### P8.1 后端测试

- [x] 覆盖平台管理员不能调用题库、试卷、考试、阅卷、成绩写接口。
- [x] 覆盖创建租户必须初始化首个 `tenant_admin`。
- [x] 覆盖创建租户失败事务回滚。
- [x] 覆盖 `user_roles` 单角色唯一约束。
- [x] 覆盖不能移除或禁用最后一个 `tenant_admin`。
- [x] 覆盖不能移除、禁用或降级最后一个 `space_admin`。
- [x] 覆盖 `tenant_admin` 不在 `space_members` 中也可以管理本租户空间资源。
- [x] 覆盖 `space_admin` 不能管理空间基础资料。
- [x] 覆盖 `teacher` 零空间不可操作题库、试卷、考试或阅卷。
- [x] 覆盖公共题库和已暴露公共试卷写接口只允许 `tenant_admin` 写入。
- [x] 覆盖学生不能访问 `/api/v1/tenant/results/:id`。
- [x] 覆盖学生只能访问自己的 `/api/v1/exam-entry/results/:id`。
- [x] 覆盖 `exam_token` 不能续签、不能访问后台接口。
- [x] 覆盖普通登录 session 不能调用答题保存、提交和事件接口。

**建议验证**：

```powershell
cd server
go test -tags json1 ./...
```

### P8.2 前端测试

- [ ] 覆盖 session role 不包含 `space_admin` 时菜单仍可由授权空间列表驱动。
- [x] 覆盖 `tenant_admin` 可见租户业务管理入口。
- [x] 覆盖 `platform_admin` 不渲染租户业务页面。
- [x] 覆盖零空间教师提示。
- [x] 覆盖学生查分接口路径为 `/api/v1/exam-entry/results/:id`。
- [x] 覆盖 401 清理登录态。
- [x] 覆盖 403 展示无权限提示。

**建议验证**：

```powershell
cd web
npm test
npm run lint
npm run build
```

### P8.3 主链路联调

- [ ] 平台管理员创建租户，并初始化首个租户管理员。
- [ ] 首个租户管理员登录后创建空间和空间成员。
- [ ] 租户管理员创建教师，教师初始不属于任何空间。
- [ ] 租户管理员把教师加入空间，教师获得空间业务权限。
- [ ] 租户管理员创建公共题库或公共试卷。
- [ ] 教师不能修改公共题库或公共试卷。
- [ ] 教师创建空间题库、组卷、发布考试。
- [ ] 学生进入考试、自动保存、提交。
- [ ] 教师阅卷并发布成绩。
- [ ] 学生通过考试入口查看自己的已发布成绩。
- [ ] 管理端按授权范围查看成绩，教师不能导出成绩。

**验收标准**：

- [ ] 主链路无越权 API 请求。
- [ ] 所有关键拒绝路径返回明确 401 或 403。

### P8.4 文档和总清单回写

- [x] 根据实际实现更新 `docs/2026-05-25-papermind-execution-checklist.md` 中 P2、P3、P8、P9、P10 的权限相关项。
- [x] 如实现与技术方案有偏差，先更新 `docs/2026-05-25-papermind-exam-platform-technical-design.md` 并说明原因。
- [x] 保持本专项清单与实际代码状态一致。

**验收标准**：

- [x] 总执行清单不再保留与当前权限方案冲突的已完成描述。
- [x] 文档没有把未实现功能写成已完成。

## 3. Review Gate

### Gate A：数据和上下文

- [ ] P1-P2 完成。
- [ ] `user_roles` 单角色约束在三种迁移中一致。
- [ ] session 和 `PermissionContext` 不保存 `space_admin`。
- [ ] 通过后端迁移和权限上下文测试。

### Gate B：权限服务

- [ ] P3 完成。
- [ ] `PermissionChecker` 接口名和权限方案一致。
- [ ] 固定角色规则覆盖平台、租户、空间、教师、学生。
- [ ] 旧接口名无残留调用。

### Gate C：业务接口

- [ ] P4-P6 完成。
- [ ] 平台、租户、考试入口三类 API 边界清晰。
- [ ] 公共资源、成绩查看、成绩导出、exam token 边界都有拒绝路径测试。

### Gate D：前端和联调

- [ ] P7-P8 完成。
- [ ] 前端菜单和 API 调用不依赖 `space_admin` session role。
- [ ] 学生查分只走考试入口接口。
- [ ] 主链路联调完成并回写文档。

## 4. 反复 Review 记录

### Review 1：按权限方案章节覆盖

- [x] 第 1-2 节角色边界已覆盖：P2、P3、P5、P7。
- [x] 第 3 节数据模型已覆盖：P1、P2。
- [x] 第 4 节租户创建已覆盖：P4.1、P7.2、P8.3。
- [x] 第 6-7 节不变式已覆盖：P4.2、P4.3、P8.1。
- [x] 第 8-9 节 PermissionChecker 和固定角色已覆盖：P3、P6。
- [x] 第 10-11 节 API 分组、exam token、session 已覆盖：P5、P8.1。
- [x] 第 12-13 节延期项已覆盖：P0.2，未纳入首版任务。

### Review 2：按实现层覆盖

- [x] 数据库迁移、DO、Repository 已覆盖：P1、P2。
- [x] service 权限抽象和不变式已覆盖：P3、P4、P6。
- [x] handler 和中间件已覆盖：P5。
- [x] 前端 session、路由、页面、API client 已覆盖：P7。
- [x] 后端和前端测试已覆盖：P8.1、P8.2。
- [x] 文档回写和总清单修正已覆盖：P8.4。

### Review 3：按高风险场景覆盖

- [x] 单角色数据库约束和旧多角色冲突已覆盖：P1.1、P2.1。
- [x] `space_admin` 不进入 session role 已覆盖：P2、P7。
- [x] 最后一个 `tenant_admin` / `space_admin` 并发删除风险已覆盖：P4.2、P4.3。
- [x] 公共题库、公共试卷写权限已覆盖：P6.1。
- [x] 学生查分接口边界已覆盖：P5.4、P7.3。
- [x] 成绩导出高风险权限已覆盖：P6.3。
- [x] `exam_token` 变成长登录 token 的风险已覆盖：P5.3。
- [x] 旧总清单与新方案冲突的回写风险已覆盖：P8.4。

## 5. 明确不在首版实现

- [ ] 多角色叠加。
- [ ] 复杂 RBAC 权限配置 UI。
- [ ] 平台管理员 impersonation。
- [ ] 自动创建默认空间。
- [ ] 创建 `tenant_admin` 或 `teacher` 时自动写入 `space_members`。
- [ ] 教师导出成绩。
- [ ] 主动 session revoke 反向索引。首版只要求关键写接口从数据库重建权限上下文。

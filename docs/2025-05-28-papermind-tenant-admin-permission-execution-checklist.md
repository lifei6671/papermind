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

- [x] P0 审计时确认 `server/data/migrations/*/001_tenant_space.sql` 和 `server/bootstrap/migration/user_role_schema_test.go` 曾存在旧三列唯一约束描述；2026-05-29 已确认本项目仍是新项目初始化建库阶段，本次权限需求不新增 `002_audit_actor_type.sql` 或其他 `002_*` 迁移脚本，最终结构直接落在 001 初始建库脚本。
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
- [x] 确认公共题库允许本租户 `tenant_admin` 与具备启用空间成员关系的 `teacher` 写入；公共试卷内容只允许 `tenant_admin` 写入，目标空间 `space_admin` 仅可把未引用公共试卷归属到自己授权空间。

**验收标准**：

- [x] 本清单后续任务没有引入多角色、RBAC UI 或 impersonation。

## P1. 数据库约束与审计字段

**目标**：让数据层和单角色权限模型一致。

**依赖**：P0。

**涉及文件**：

- 保持：`server/data/migrations/*/001_tenant_space.sql` 作为新项目首版初始化建库脚本，直接包含当前最终字段和约束。
- 不新增：`server/data/migrations/*/002_audit_actor_type.sql` 或其他 `002_*` 脚本；本项目当前无历史生产库升级诉求，本次需求不做数据库迁移动作。
- 修改：`server/bootstrap/migration/user_role_schema_test.go`
- 修改：`server/internal/dao/db/base_do.go` 或当前基础字段定义文件
- 修改：`server/internal/dao/db/*_do.go`
- 修改：`server/internal/dao/db/*_model_test.go`

### P1.1 `tenant_user_memberships` 单角色唯一约束

- [x] 在 PostgreSQL 001 初始化脚本中直接使用 `tenant_user_memberships (tenant_id, user_id)` 唯一索引。
- [x] 在 MySQL 001 初始化脚本中直接使用 `tenant_user_memberships (tenant_id, user_id)` 唯一约束。
- [x] 在 SQLite 001 初始化脚本中直接使用 `tenant_user_memberships (tenant_id, user_id)` 唯一索引。
- [x] 索引或约束命名改为表达单角色语义，例如 `uk_tenant_user_memberships_user`。
- [x] 更新 `server/bootstrap/migration/user_role_schema_test.go` 中三种数据库断言。
- [x] `users` 改为租户侧通用账号表，不再包含 `tenant_id`；用户和租户的归属关系由 `tenant_user_memberships` 维护。

**验收标准**：

- [x] 同一租户同一用户不能同时写入 `tenant_admin` 和 `teacher`。
- [x] `tenant_user_memberships.role` 用于保存当前唯一租户级角色，`tenant_user_memberships.status` 用于保存该租户成员关系是否启用。
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
- [x] 审计主体类型字段直接通过 `001_tenant_space.sql` 初始化建库落地；本项目当前不需要追加迁移动作。
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

- [x] 实现 `ValidateTenantAdminInvariant(tx, tenantID)`。
- [x] 禁用用户时接入。
- [x] 删除用户时接入。
- [x] 移除或修改 `tenant_admin` 角色时接入。
- [x] 批量导入覆盖角色时接入。
- [x] 校验必须在同一事务内基于变更后状态执行。

> 2026-05-28 进度：禁用用户路径已在 `TenantUserRepository.UpdateStatus`
> 的同一 GORM 事务内先更新用户状态，再基于变更后状态执行租户管理员和空间管理员不变式校验；
> 对应 DAO 测试已覆盖禁止禁用最后一个启用 `tenant_admin`、允许禁用非最后一个管理员以及失败回滚。
> 删除用户、租户级角色变更、批量角色覆盖入口当前尚未形成稳定 API / service 契约，暂不为勾选清单主动扩展公共接口。
>
> 2026-05-29 进度：禁用用户事务已在变更前锁定本租户启用 `tenant_admin`
> 候选用户行，并锁定目标用户涉及的启用空间内 `space_admin` 成员行；
> DAO 测试已覆盖锁定查询具备 `FOR UPDATE` 语义，避免两个并发禁用请求同时通过最后管理员计数。
> 租户级角色降权入口仍不存在，未来新增时必须复用同一事务锁定策略。
>
> 2026-05-29 进度：当前实现位于 `TenantUserRepository.UpdateStatus`
> 事务内的 `validateTenantAdminInvariant(ctx, tx, tenantID)`，并与行锁、状态更新和回滚一起验证。
>
> 2026-05-29 进度：已新增 `DELETE /api/v1/users/:id`，接口层从当前
> `tenant_admin` session 派生 `tenant_id` 和 `actor_id`，禁止自删；
> `TenantUserRepository.DeleteUser` 在同一事务内先软删除目标用户，再调用
> `validateTenantAdminInvariant(ctx, tx, tenantID)` 基于删除后的状态校验，
> 校验失败会回滚 `users.deleted_at`。
>
> 2026-05-29 进度：已新增 `PUT /api/v1/users/:id/role`，首版单角色模型下
> “移除 `tenant_admin`” 表达为把目标用户改为 `teacher` 或 `student`。
> 接口层从当前 `tenant_admin` session 派生租户和操作者，拒绝修改自己的角色；
> `TenantUserRepository.UpdateRole` 在同一事务内先更新 `tenant_user_memberships.role`，
> 再调用 `validateTenantAdminInvariant(ctx, tx, tenantID)` 基于角色变更后的状态校验，
> 校验失败会回滚角色更新。
>
> 2026-05-29 进度：已新增 `POST /api/v1/users/import`，当前范围为后端
> JSON 批量导入，不包含文件解析。接口层从当前 `tenant_admin` session 派生
> `tenant_id` 和 `actor_id`，忽略请求体伪造的 `tenant_id`；service 负责校验
> 每行用户名、真实姓名、密码和租户级角色，并统一写入密码哈希；
> `TenantUserRepository.ImportUsers` 以租户内 `username` 作为覆盖键，在同一事务内
> 先锁定启用 `tenant_admin` 关系行，再批量创建或覆盖用户资料、密码和 `tenant_user_memberships.role`，
> 最后调用 `validateTenantAdminInvariant(ctx, tx, tenantID)`。若批量覆盖会移除最后一个
> 启用 `tenant_admin`，整批导入回滚。
>
> 2026-05-29 进度：`TenantUserRepository.DeleteUser` 已检查软删除更新行数；
> 删除不存在或已删除用户时返回 `ErrUserNotFound`，API 返回 404，不再把空更新误报为成功。

**验收标准**：

- [x] 禁止禁用最后一个启用状态 `tenant_admin`。
- [x] 两个并发降权或禁用请求不能同时删除最后一个管理员。

### P4.3 空间管理员不变式

- [x] 实现 `ValidateSpaceAdminInvariant(tx, tenantID, spaceID)`。
- [x] 禁用空间成员时接入。
- [x] 移除空间成员时接入。
- [x] 修改空间成员角色时接入。
- [x] 禁用或删除租户用户时，对其涉及的空间逐个接入。
- [x] 校验必须在同一事务内基于变更后状态执行。

> 2026-05-29 进度：空间成员禁用、移除和角色变更事务已在变更前锁定目标空间内
> 启用 `space_admin` 成员行；禁用租户用户事务也会锁定目标用户涉及空间内的
> `space_admin` 成员行。DAO 测试已覆盖锁定查询具备 `FOR UPDATE` 语义，
> 用于串行化并发禁用、移除或降级最后空间管理员的风险窗口。
>
> 2026-05-29 进度：空间成员写入口通过 `SpaceRepository.updateMember`
> 事务内的 `validateSpaceAdminInvariantAfterMemberChange(ctx, tx, tenantID, spaceID)`
> 校验目标空间；禁用租户用户时通过
> `validateSpaceAdminInvariantAfterUserStatusChange(ctx, tx, tenantID, userID)`
> 逐个校验该用户涉及的启用空间。删除租户用户时复用同一事务校验，基于
> 目标用户已软删除后的有效用户集合逐个检查其管理的启用空间。
>
> 2026-05-29 进度：有效空间成员查询、当前空间成员反查和个人授权空间列表
> 已同时过滤 `spaces.status = enabled` 与 `spaces.deleted_at = 0`；
> 也会过滤 `users.status = enabled` 与 `users.deleted_at = 0`。空间管理员计数
> 已 JOIN 启用且未删除的 `users`，禁用或删除用户不再被计入最后一个
> `space_admin` 不变式，旧 session 也不能继续获得空间写权限。

**验收标准**：

- [x] 禁止移除、禁用或降级最后一个启用状态 `space_admin`。
- [x] 禁用租户用户不会导致任一启用空间失去最后一个空间管理员。

### P4.4 教师创建和空间分配

- [x] `tenant_admin` 创建 `teacher` 用户时只写全局 `users` 和 `tenant_user_memberships`。
- [x] 创建 `teacher` 不自动写入 `space_members`。
- [x] 教师加入空间必须通过空间成员接口显式分配。
- [x] 教师没有启用空间成员关系时允许登录。
- [x] 零空间教师不能操作题库、试卷、考试或阅卷。

**验收标准**：

- [x] 用户详情或业务页能提示“该教师暂未加入任何空间，当前无法操作题库、试卷、考试或阅卷”。

### P4.5 空间资料与空间成员接口拆分

- [x] `PUT /api/v1/tenant/spaces/:id` 只允许 `tenant_admin`。
- [x] `DELETE /api/v1/tenant/spaces/:id` 只允许 `tenant_admin`。
- [x] `GET /api/v1/tenant/spaces/:id/members` 允许 `tenant_admin` 或当前空间 `space_admin`。
- [x] `POST /api/v1/tenant/spaces/:id/members` 允许 `tenant_admin` 或当前空间 `space_admin`。
- [x] `PUT /api/v1/tenant/spaces/:id/members/:user_id` 允许 `tenant_admin` 或当前空间 `space_admin`。
- [x] `DELETE /api/v1/tenant/spaces/:id/members/:user_id` 允许 `tenant_admin` 或当前空间 `space_admin`。

> 2026-05-29 进度：已补齐空间资料更新和软删除接口；接口层只允许本租户 `tenant_admin`，
> 当前空间 `space_admin` 调用更新或删除空间资料均返回 403。
> 对应 service、DAO 和 API 测试已覆盖资料字段更新、软删除后列表不可见、
> 以及 `space_admin` 不能修改或删除空间资料。
>
> 2026-05-29 进度：已补 API 测试覆盖 `tenant_admin` 读取、新增、更新和移除空间成员；
> 当前空间 `space_admin` 可读取、新增、更新和移除本空间成员；
> `space_admin` 跨空间管理成员返回 403；`tenant_admin` 和 `space_admin`
> 都不能禁用、降级或移除最后一个启用 `space_admin`。
>
> 2026-05-29 进度：空间资料和空间成员接口已补充 `/api/v1/tenant/spaces...`
> 路由；旧 `/api/v1/spaces...` 路由暂保留为兼容入口，前端租户侧 API client 已切到
> `/api/v1/tenant/spaces...`。
>
> 2026-05-29 进度：新增空间成员和修改空间成员角色会校验空间内角色枚举；
> 新建空间初始管理员、添加空间成员和修改成员角色都会校验目标用户属于当前租户、
> 启用且未删除，避免把不可用用户写入有效空间授权。

**验收标准**：

- [x] `space_admin` 不能修改空间名称、Logo、描述、状态或删除空间。
- [x] `space_admin` 只能管理自己所在空间的成员。

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

- [x] `/api/v1/platform/**` 只允许 `platform_admin`。
  - 2026-05-28：当前平台治理接口实际落在 `/api/v1/tenants*`，已由 `requirePlatformPrincipalMiddleware` 保护；接口测试覆盖租户用户访问租户列表、创建、资料维护、租户码重置和注册开关都返回 403。
- [x] 平台管理员可以创建租户、维护租户资料、重置租户码、修改注册开关。
- [x] 租户用户不能访问平台接口。
- [x] 平台管理员不能调用租户业务写接口。
  - 2026-05-28：接口测试覆盖平台管理员访问考试列表、发布考试、创建题目、创建试卷大题都被拒绝。
  - 2026-05-29：平台管理员查看租户资源抽屉改走平台侧只读接口
    `GET /api/v1/tenants/:id/spaces` 和 `GET /api/v1/tenants/:id/users`；
    租户用户访问这些平台治理接口返回 403，平台抽屉不再复用租户侧
    `/api/v1/tenant/**` client。

**验收标准**：

- [x] 构造 `tenant_id` 参数不能绕过平台/租户边界。

### P5.2 租户接口边界

- [x] `/api/v1/tenant/**` 只允许 `tenant_user`。
- [x] 所有租户接口从 session 或上下文取得 `tenant_id`。
  - 2026-05-29：用户管理 `/api/v1/users` 和空间管理 `/api/v1/spaces`
    曾作为租户侧兼容入口保留；当前真实租户路由已迁移到 `/api/v1/tenant/**`，
    后端从登录态 `principal.TenantID` 派生租户。列表、创建、禁用、删除、
    角色修改、批量导入用户、空间资料增删改查、空间成员读取、新增、更新和移除
    均忽略 query/body 中伪造的跨租户 `tenant_id`。API 测试已覆盖平台管理员访问
    users/spaces 返回 403，以及租户管理员构造其他租户 `tenant_id` 时仍操作当前
    session 租户。
  - 2026-05-29：后端已新增 `/api/v1/tenant/profile/spaces`、
    `/api/v1/tenant/users...`、`/api/v1/tenant/spaces...` 路由；
    平台账号访问这些租户前缀接口返回 403，租户账号仍从 session 派生
    `tenant_id` 并忽略 query/body 中伪造的跨租户 `tenant_id`。
    前端租户侧 user/space/profile spaces API client 已切到 `/api/v1/tenant/**`；
    平台管理员直接访问 `/spaces`、`/users` 等租户业务后台路由时回到概览页，
    不触发租户接口请求。
  - 2026-05-29：租户通用账号登录不再提交 `tenant_id`，登录后生成未绑定租户空间的
    `tenant_user` session；前端跳转 `/tenant-entry` 展示可进入的租户和空间，
    用户选择后调用 `/api/v1/auth/tenant/select-space`，后端再把 session 绑定到目标
    `tenant_id`、租户级角色和可用空间范围。
- [x] 不信任请求体里的跨租户 `tenant_id`。
  - 2026-05-28：接口测试覆盖 `teacher` 使用本租户 session 构造其他租户 `tenant_id` 发布考试时返回 403。
  - 2026-05-29：接口测试补充覆盖 `tenant_admin` 在 users/spaces
    管理入口构造其他租户 `tenant_id` 时，后端不使用请求中的租户 ID。
- [x] `student` 不能访问管理端接口。
- [x] `GET /api/v1/tenant/results/:id` 不允许 `student`。
  - 2026-05-28：当前管理端成绩接口实际为 `/api/v1/results`；接口测试覆盖学生访问考试管理、待阅卷和管理端成绩列表都返回 403。

**验收标准**：

- [x] `platform_user` 访问 `/api/v1/tenant/**` 返回 403。
- [x] `tenant_user` 访问 `/api/v1/platform/**` 返回 403。

### P5.3 考试入口 `exam_token`

- [x] `/api/v1/exam-entry/**` 不复用管理端权限中间件。
- [x] `exam_token` 使用服务端随机不透明 token。
- [x] 数据库只保存 `exam_token_hash`。
- [x] 中间件按 hash 找到 attempt。
- [x] 中间件校验 `tenant_id`、`exam_id`、`attempt_id`、`user_id`、attempt 状态、token 过期时间和业务作答截止时间。
  - 2026-05-28：写入口 middleware 已先按 `exam_token` hash 找到 attempt，并校验 `tenant_id`、`attempt_id`、attempt 状态、token 过期时间和业务作答截止时间；`exam_id`、`user_id` 当前由 attempt 派生到 `ExamEntryContext`，尚未作为请求级字段比对。
  - 2026-05-29：保持现有写入口协议，不新增 `exam_id` / `user_id`
    请求字段；middleware 继续用 `exam_token` 绑定的 attempt 派生
    `ExamEntryContext.ExamID` 和 `ExamEntryContext.UserID`，并通过 API 测试覆盖
    tenant mismatch、attempt mismatch 均被拒绝，客户端额外传入伪造
    `exam_id` / `user_id` 不能改变答案保存的真实 `updated_by`。
    若未来要求请求体逐字段比对 `exam_id` / `user_id`，需要先确认公共 API 变更。
- [x] `exam_token` 过期后不续期；重复开考会为同一个 in-progress attempt 续发新的明文 token 并替换 hash。
- [x] 普通登录 session 不能调用自动保存、提交和事件接口。
- [x] `exam_token` 不能调用 profile、tenant、questions、papers、grading、results 等后台接口。

**验收标准**：

- [x] 过期 token、已提交 attempt、超过可保存窗口都会拒绝写入。
- [x] 管理端 session 直接调用答题保存接口失败。

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

- [x] `questions.space_id = NULL` 的公共题库允许本租户 `tenant_admin` 或具备启用空间成员关系的 `teacher` 创建。
- [x] 公共题库允许本租户 `tenant_admin` 或具备启用空间成员关系的 `teacher` 修改、删除和导入。
- [x] 已暴露的公共试卷内容写接口只允许 `tenant_admin` 修改大题、选题、规则和规则生成；目标空间 `space_admin` 仅可把未被考试引用的公共试卷归属到自己授权空间。
- [x] `space_admin` / `teacher` 只能管理自己启用空间内的题库和试卷。
- [x] `space_admin` / `teacher` 读取或引用公共资源时，由对应 service 显式校验。
- [x] `papers.space_id = NULL` 的创建和删除 API 已复用公共试卷写入校验。
  - 2026-05-29：新增 `POST /api/v1/papers` 和 `DELETE /api/v1/papers/:id`。
    创建时 `space_id = NULL` 的公共试卷只允许本租户 `tenant_admin`；
    删除时从 `paper_id` 反查真实 `papers.space_id` 后复用既有写权限校验。
    删除试卷会先拒绝已被未删除考试引用的试卷；未被考试引用时使用软删除并清理组卷关系表。
    本轮不新增数据库迁移动作。

**验收标准**：

- [x] 教师可以维护租户公共题库，但不能把 `space_id = NULL` 的公共试卷当成自己的可写资源。
- [x] 空间管理员不能导入公共题库。

### P6.2 资源归属反查

- [x] 题目权限从 `questionID` 反查真实 `tenant_id` 和 `space_id`。
- [x] 试卷权限从 `paperID` 反查真实 `tenant_id` 和 `space_id`。
  - 2026-05-28：手动组卷新增 `QuestionUsableForPaper`，由 DAO 反查 `papers.space_id` 和 `questions.space_id`；规则组卷候选题查询同步按试卷真实空间过滤，接口测试覆盖其他空间题不能被手动加入或规则抽中。
- [x] 考试发布权限从 `paperID` 或考试目标反查真实空间。
  - 2026-05-28：考试发布在创建草稿前反查 `papers.space_id`，并按 `target_type` 反查投放空间或目标用户有效空间成员关系；接口测试覆盖跨空间试卷、跨空间投放空间、跨空间目标用户都会被拒绝，且拒绝时不创建草稿。
- [x] 阅卷权限从 `attemptID` 反查考试和成绩行真实空间。
- [x] 成绩列表和导出从 `examID` 或成绩行反查真实空间。
  - 2026-05-28：新增 `TestReviewAndResultAPIRoutesRejectForgedSpaceIDWithSQLite` 覆盖管理端 HTTP 入口伪造已授权 `space_id` 时，待阅卷列表、阅卷写入、成绩列表和成绩导出都按 attempt / 成绩行真实空间过滤。
- [x] 禁止只信任请求参数拼接授权范围。

**验收标准**：

- [x] 伪造 `space_id` 不能越权查看、阅卷或导出其他空间数据。
- [x] 教师不能发布其他空间试卷，也不能把考试投放到无权限空间或无权限空间内用户。

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
- [x] 空间管理员菜单可见性来自空间成员接口或授权空间列表。
  - 2026-05-28：租户登录已拉取授权空间并写入 `session.profileSpaces`，`AdminShell` 已覆盖由授权空间驱动菜单；本轮新增真实 `/space-members` 空间成员入口，菜单由启用的 `profileSpaces.role = space_admin` 授权驱动，并通过现有空间成员接口读取成员列表，不暴露空间创建或空间资料编辑能力。
  - 2026-05-29：`/space-members` 已对接后端空间成员 API。页面按授权空间调用
    `GET /api/v1/tenant/spaces/:id/members` 读取成员，并通过同组接口完成添加成员、
    修改空间身份、启用/禁用和移除，不再维护前端 mock 成员数据。
  - 2026-05-29：租户管理员不再展示独立“空间成员”菜单，避免和“空间管理”
    下的成员管理入口重复；租户管理员继续从“空间管理 > 成员管理”维护空间成员。
    独立 `/space-members` 菜单只由启用的 `profileSpaces.role = space_admin` 授权驱动，
    供空间管理员管理自己授权空间内的成员。
  - 2026-05-29：租户登录页已移除租户 ID 输入框，租户账号先进入 `/tenant-entry`
    选择租户空间；`session.profileSpaces` 由登录后列表和选择空间后的最新 profile spaces
    同步，学生选择空间后进入考试入口，其他租户角色进入后台。
  - 2026-05-29：`/tenant-entry` 已按角色展示不同入口。`tenant_admin` 的多个空间授权会
    折叠成单个租户后台入口，并且选择时不传 `space_id`；`space_admin`、`teacher`
    和 `student` 继续按具体空间展示空间管理、教学业务或考试入口。
  - 2026-05-29：租户后台侧边栏不再显示平台管理员文案；租户用户显示当前租户身份和
    租户名称，主按钮从“回到概览”改为“切换租户”，点击后回到 `/tenant-entry`。
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
- [x] 用户详情页展示教师空间分配状态。
- [x] 零空间教师显示“该教师暂未加入任何空间，当前无法操作题库、试卷、考试或阅卷”。

**验收标准**：

- [x] 创建教师后不会在空间成员列表中自动出现。
- [x] 通过空间成员入口分配后，教师才获得对应空间业务入口。

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
- [x] 覆盖 `tenant_user_memberships` 单角色唯一约束。
- [x] 覆盖不能移除或禁用最后一个 `tenant_admin`。
- [x] 覆盖不能移除、禁用或降级最后一个 `space_admin`。
- [x] 覆盖 `tenant_admin` 不在 `space_members` 中也可以管理本租户空间资源。
- [x] 覆盖 `space_admin` 不能管理空间基础资料。
- [x] 覆盖 `teacher` 零空间不可操作题库、试卷、考试或阅卷。
- [x] 覆盖公共题库允许 `tenant_admin` / 具备启用空间成员关系的 `teacher` 写入，已暴露公共试卷内容写接口只允许 `tenant_admin` 写入；目标空间 `space_admin` 归属未引用公共试卷的例外路径有单独用例。
- [x] 覆盖学生不能访问 `/api/v1/tenant/results/:id`。
- [x] 覆盖学生只能访问自己的 `/api/v1/exam-entry/results/:id`。
- [x] 覆盖 `exam_token` 过期后不续期、重复开考续发 token、不能访问后台接口。
- [x] 覆盖普通登录 session 不能调用答题保存、提交和事件接口。
- [x] 覆盖 `exam_token` 返回不透明 token，数据库只保存 hash。
- [x] 覆盖过期 token、已提交 attempt、超过可保存窗口都会拒绝答题写入。
- [x] 覆盖 `/api/v1/exam-entry` 答题写入口可仅凭 `exam_token` 调用，不依赖后台 session。
- [x] 覆盖 `/api/v1/exam-entry` 写入口 middleware 在 handler 参数校验前先校验 `exam_token`。
- [x] 覆盖创建 `teacher` 后不会自动写入 `space_members`。
- [x] 覆盖伪造 `space_id` 不能越权查看、阅卷或导出其他空间数据。
- [x] 覆盖手动组卷和规则组卷不能引用其他空间题目。
- [x] 覆盖考试发布不能引用其他空间试卷或投放到无权限目标，且拒绝时不创建草稿。
- [x] 覆盖平台治理接口拒绝租户用户，租户业务接口拒绝平台用户、跨租户 `tenant_id` 和学生角色。
- [x] 覆盖 `teacher` 通过空间成员入口分配后，才会从 `/api/v1/tenant/profile/spaces` 获得授权空间。

**建议验证**：

```powershell
cd server
go test -tags json1 ./...
```

### P8.2 前端测试

- [x] 覆盖 session role 不包含 `space_admin` 时菜单仍可由授权空间列表驱动。
- [x] 覆盖真实空间成员入口只由授权空间列表中的 `space_admin` 驱动，并对接成员读取、添加、角色/状态更新和移除 API。
- [x] 覆盖 `tenant_admin` 不展示独立“空间成员”菜单，空间成员维护从“空间管理”的成员管理入口进入。
- [x] 覆盖 `tenant_admin` 可见租户业务管理入口。
- [x] 覆盖 `platform_admin` 不渲染租户业务页面。
- [x] 覆盖租户用户不展示平台治理菜单，直接访问平台治理路由不触发平台页 API。
- [x] 覆盖零空间教师提示。
- [x] 覆盖用户详情展示教师空间分配状态。
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

- [x] 平台管理员创建租户，并初始化首个租户管理员。
- [x] 首个租户管理员登录后创建空间和空间成员。
- [x] 租户管理员创建教师，教师初始不属于任何空间。
- [x] 租户管理员把教师加入空间，教师获得空间业务权限。
  - 2026-05-29：新增后端 API 主链路测试
    `TestTenantAdminMainFlowCreatesSpaceAndAssignsTeacherWithSQLite`，
    覆盖平台管理员创建租户和首个 `tenant_admin`，首个管理员登录后创建空间，
    创建 `teacher` 后其 `/api/v1/tenant/profile/spaces` 初始为空，
    再通过空间成员入口分配后获得启用空间授权。
- [x] 租户管理员创建公共题库或公共试卷。
- [x] 教师可以修改公共题库，不能修改公共试卷。
- [x] 教师创建空间题库、组卷、发布考试。
  - 2026-05-29：`TestTenantAdminMainFlowCreatesSpaceAndAssignsTeacherWithSQLite`
    扩展覆盖租户管理员创建公共题库，教师具备启用空间成员关系时可写公共题库；
    教师在授权空间创建空间题库、创建试卷大题、加入空间题目并发布空间考试。
    早前主链路测试先用数据库预置空间试卷；后续已补齐公开试卷创建/删除 API，
    公共试卷创建和删除权限由 `paper_handler_test.go` 覆盖。
- [x] 学生进入考试、自动保存、提交。
  - 2026-05-29：同一后端 API 主链路测试继续覆盖租户管理员创建学生并加入考试空间；
    学生通过 `/api/v1/exam-entry/invite/resolve` 解析邀请码，开始考试后使用
    `exam_token` 保存答案并提交，断言 attempt 进入 `submitted` 且客观题得分写入。
- [x] 教师阅卷并发布成绩。
  - 2026-05-29：同一后端 API 主链路测试新增简答题，覆盖教师读取待阅卷列表、
    使用答案版本完成评分，并保存 manual publish 成绩发布时间。
- [x] 学生通过考试入口查看自己的已发布成绩。
  - 2026-05-29：同一测试在成绩发布时间生效后，使用学生登录态访问
    `/api/v1/exam-entry/results/:id`，断言客观分、主观分和总分可见。
- [x] 管理端按授权范围查看成绩，教师不能导出成绩。
  - 2026-05-29：同一测试覆盖教师按授权空间读取成绩列表成功，同时调用
    `/api/v1/results/export` 返回 403，避免教师越权导出成绩。

**验收标准**：

- [x] 主链路无越权 API 请求。
  - 2026-05-29：主链路测试内所有管理端、教师端、学生端请求均使用对应登录态；
    租户与空间来源通过服务端权限上下文校验，未依赖伪造请求字段放行。
- [x] 所有关键拒绝路径返回明确 401 或 403。
  - 2026-05-29：当前主链路验收覆盖教师无启用空间成员关系时写公共题库 403，以及教师导出成绩 403；
    其他权限拒绝路径由 P5/P6/P8.1 的专项 API 测试继续覆盖。

### P8.4 文档和总清单回写

- [x] 根据实际实现更新 `docs/2026-05-25-papermind-execution-checklist.md` 中 P2、P3、P8、P9、P10 的权限相关项。
- [x] 如实现与技术方案有偏差，先更新 `docs/2026-05-25-papermind-exam-platform-technical-design.md` 并说明原因。
- [x] 保持本专项清单与实际代码状态一致。

**验收标准**：

- [x] 总执行清单不再保留与当前权限方案冲突的已完成描述。
- [x] 文档没有把未实现功能写成已完成。

## 3. Review Gate

### Gate A：数据和上下文

- [x] P1-P2 完成。
- [x] `tenant_user_memberships` 单角色约束在三种初始化 SQL 中一致。
- [x] session 和 `PermissionContext` 不保存 `space_admin`。
- [x] 通过后端迁移和权限上下文测试。
  - 2026-05-29：已确认本项目仍按新项目初始化建库处理，本次权限需求不新增
    `002_audit_actor_type.sql` 或其他 `002_*` 迁移脚本；三种数据库的
    `001_tenant_space.sql` 直接包含 `created_by_type`、`updated_by_type`、
    全局 `users`、`tenant_user_memberships` 和
    `uk_tenant_user_memberships_user (tenant_id, user_id)`。验证命令：
    `go test -tags json1 ./...`，结果通过。

### Gate B：权限服务

- [x] P3 完成。
- [x] `PermissionChecker` 接口名和权限方案一致。
- [x] 固定角色规则覆盖平台、租户、空间、教师、学生。
- [x] 旧接口名无残留调用。
  - 2026-05-29：`server/internal/service/permission/checker.go`
    已统一为 `CanManageTenantLifecycle`、`CanManageTenantBusiness`、
    `CanManageTenantUsers`、`CanManageSpaceProfile`、`CanManageSpaceMembers`、
    `CanManageQuestion`、`CanManagePaper`、`CanPublishExam`、
    `CanGradeAttempt`、`CanViewExamResults`、`CanExportExamResults`、
    `CanViewOwnResult` 和 `CanTakeExam`。
    固定角色测试覆盖平台管理员、租户管理员、空间管理员、教师和学生。
    `rg` 检查确认旧接口名 `CanManageTenant`、`CanManageSpace`、
    `CanGradeExam`、`CanViewResults`、`TenantRoles`、`SpaceRoles`
    无残留调用；验证命令：`go test -tags json1 ./internal/service/permission`。

### Gate C：业务接口

- [x] P4-P6 完成。
- [x] 平台、租户、考试入口三类 API 边界清晰。
- [x] 公共资源、成绩查看、成绩导出、exam token 边界都有拒绝路径测试。
  - 2026-05-29：已完成现有公开接口范围内的用户禁用、空间成员、题库、
    试卷组卷、考试、阅卷、成绩查看、成绩导出和 exam token 拒绝路径验证。
    用户批量导入已按现有 `/api/v1/users/import` 和
    `/api/v1/tenant/users/import` 契约完成；`/api/v1/tenant/**`
    租户用户、空间和当前授权空间路由迁移已完成。
  - 2026-05-29：公共试卷创建/删除 API 已补齐。目标测试覆盖
    `tenant_admin` 可创建并软删除公共试卷，`teacher` 创建或删除公共试卷返回 403；
    已被考试引用的试卷删除会返回业务错误并保持试卷有效。
    该项只新增 API / service / repository 逻辑，仍不需要数据库迁移动作。
  - 2026-05-29：评审发现的有效空间授权缺口已修复。禁用或软删除空间不会再出现在
    `/api/v1/tenant/profile/spaces`，也不能继续授权教师写空间题库；空间成员写入口
    拒绝无效角色和不可用目标用户，删除不存在用户返回 404。

### Gate D：前端和联调

- [x] P7-P8 完成。
- [x] 前端菜单和 API 调用不依赖 `space_admin` session role。
- [x] 学生查分只走考试入口接口。
- [x] 主链路联调完成并回写文档。
  - 2026-05-29：前端 `routeVisibleForRole`、`AdminShell`、登录页和学生查分
    API 测试覆盖 `space_admin` 只来自 `profileSpaces`，不写入 session role；
    学生查分 API 固定调用 `/api/v1/exam-entry/results/:id`。
    后端主链路测试已覆盖平台建租户、租户管理员建空间和成员、教师发布考试、
    学生答题提交、教师阅卷发布成绩、学生查分和教师导出成绩 403。
    验证命令：`npm test`、`npm run lint`、`npm run build`、
    `go test -tags json1 ./...`，结果均通过。

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

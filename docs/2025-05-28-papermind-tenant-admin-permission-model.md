# PaperMind 租户、平台管理员、租户管理员权限方案

> 执行推进清单见：`docs/2025-05-28-papermind-tenant-admin-permission-execution-checklist.md`。

## 1. 设计目标

本文件是对《PaperMind 在线考试平台技术方案》中权限模型、用户角色、API 分组和认证上下文部分的细化与修订。当本文与原技术方案存在冲突时，以本文为准。

首版采用固定角色模型，不实现复杂 RBAC，也不支持多角色叠加。

同一个租户用户首版只能拥有一个租户级角色：

```text
tenant_admin / teacher / student 三选一。
```

空间内身份仍通过 `space_members.role_in_space` 单独表达。租户级角色和空间内身份不是多角色系统，不允许把同一个用户同时配置成 `tenant_admin` 和 `teacher` 来组合权限。

首版明确采用单租户级角色模型：

```text
- tenant_user_memberships 使用 UNIQUE (tenant_id, user_id)。
- ActorContext.Role 只保存一个租户级角色。
- PermissionContext.Role 只保存一个租户级角色。
- space_admin 不属于租户级角色，不写入 ActorContext.Role。
- space_admin 只能通过 space_members.role_in_space 动态判断。
- 后续如果要支持多角色，再改为 UNIQUE (tenant_id, user_id, role)，并同步改造认证 session、DAO、菜单和 PermissionChecker。
```

角色分为两类主体：

```text
平台主体：
- platform_admin

租户主体：
- tenant_admin
- teacher
- student

空间内身份：
- space_admin
- teacher
- student
```

核心原则：

```text
platform_admin 管平台生命周期，不默认进入租户业务。
tenant_admin 管租户内业务，是租户内最高业务角色。
space_admin 管指定空间内业务。
teacher 执行出题、组卷、发布考试、阅卷等教学动作。
student 参加考试、查看自己成绩。
```

---

## 2. 权限边界

### 2.1 平台管理员 platform_admin

平台管理员只拥有平台治理权限。

允许：

```text
- 创建租户
- 启用 / 禁用租户
- 重置租户码
- 控制租户注册开关
- 创建首个租户管理员
- 重置租户管理员账号
- 禁用异常租户管理员账号
- 查看租户基础概览
- 管理平台配置
- 管理平台管理员账号
```

不默认允许：

```text
- 创建题目
- 修改题库
- 创建试卷
- 发布考试
- 阅卷
- 修改成绩
- 导出租户成绩明细
- 删除租户业务数据
```

平台管理员不是租户成员，不写入 `users` 表，不写入 `tenant_user_memberships` 表，不写入 `space_members` 表。

---

### 2.2 租户管理员 tenant_admin

租户管理员是租户内最高业务角色。

允许：

```text
- 管理租户用户
- 创建 / 管理空间
- 分配空间管理员
- 分配教师
- 分配学生
- 管理租户公共题库
- 管理租户公共试卷
- 管理租户内考试
- 查看租户内成绩
- 导出租户内成绩
```

租户管理员可以覆盖空间管理员和教师的业务能力，但必须受 `tenant_id` 限制。

---

### 2.3 空间管理员 space_admin

空间管理员是空间内最高业务角色。

允许：

```text
- 管理本空间成员
- 管理本空间题库
- 管理本空间试卷
- 管理本空间考试
- 查看本空间成绩
- 管理本空间教师和学生
```

不能：

```text
- 管理其他空间
- 管理租户级配置
- 管理平台级配置
- 管理其他租户
```

---

### 2.4 教师 teacher

教师是教学执行角色。

允许：

```text
- 在授权空间内创建题目
- 在授权空间内组卷
- 在授权空间内发布考试
- 阅卷
- 查看自己权限范围内的成绩
```

不能：

```text
- 管理租户配置
- 管理平台配置
- 管理空间管理员
- 管理其他空间数据
```

---

### 2.5 学生 student

允许：

```text
- 查看自己可参加的考试
- 开始考试
- 自动保存答案
- 提交答卷
- 查看自己成绩
```

不能访问管理端业务接口。

---

## 3. 数据模型建议

### 3.1 平台管理员表

保留现有设计：

```text
platform_users
├── id
├── username
├── phone
├── email
├── password_hash
├── status
├── created_at
├── updated_at
├── deleted_at
└── ext_json
```

说明：

```text
platform_users 不带 tenant_id。
platform_admin 不进入 users 表。
platform_admin 不直接绑定 tenant_user_memberships。
```

---

### 3.2 租户表

```text
tenants
├── id
├── name
├── tenant_code
├── allow_register
├── status
├── created_at
├── created_by
├── created_by_type
├── updated_at
├── updated_by
├── updated_by_type
├── deleted_at
├── version
└── ext_json
```

建议新增：

```text
created_by_type
updated_by_type
```

原因：现有表已经有 `created_by` 和 `updated_by`，继续用这两个字段保存操作者 ID。本次只新增两个类型字段，用来区分这个 ID 来自 `platform_users`、`users` 还是系统任务。如果只存 `created_by = 1`，后续无法判断这个 1 来自 `platform_users.id` 还是 `users.id`。

枚举值：

```text
actor_type:
- platform_user
- tenant_user
- system
```

---

### 3.3 租户用户表

```text
users
├── id
├── username
├── real_name
├── phone
├── email
├── password_hash
├── status
├── created_at
├── created_by
├── created_by_type
├── updated_at
├── updated_by
├── updated_by_type
├── deleted_at
├── version
└── ext_json
```

`users` 是租户侧通用账号表，不再直接绑定某一个租户。同一个账号可以通过 `tenant_user_memberships` 成为多个租户的用户。

租户管理员本质上也是租户用户，只是在某个租户的成员关系中拥有 `tenant_admin` 角色。

---

### 3.4 租户用户关系表

```text
tenant_user_memberships
├── id
├── tenant_id
├── user_id
├── role
├── status
├── created_at
├── created_by
├── created_by_type
├── updated_at
├── updated_by
├── updated_by_type
├── version
└── ext_json
```

`role` 取值：

```text
tenant_admin
teacher
student
```

唯一约束：

```sql
UNIQUE (tenant_id, user_id)
```

首版只支持单角色，`tenant_user_memberships` 中同一个 `tenant_id + user_id` 只能存在一条租户成员关系。`role` 保存该用户在当前租户内的唯一租户级角色，`status` 保存该成员关系是否启用；禁用某个租户成员关系不等于禁用全局用户账号。后续如果要支持多角色，再把唯一约束调整为 `UNIQUE (tenant_id, user_id, role)`，并同步改造认证 session、DAO、前端菜单和 `PermissionContext`。

---

### 3.5 空间成员表

```text
space_members
├── id
├── tenant_id
├── space_id
├── user_id
├── role_in_space
├── status
├── created_at
├── created_by
├── created_by_type
├── updated_at
├── updated_by
├── updated_by_type
├── deleted_at
├── version
└── ext_json
```

`role_in_space` 取值：

```text
space_admin
teacher
student
```

唯一约束：

```sql
UNIQUE (tenant_id, space_id, user_id, deleted_at)
```

---

## 4. 租户创建流程

平台管理员创建租户时，必须同时初始化首个租户管理员。

### 4.1 API

```http
POST /api/v1/platform/tenants
```

请求体：

```json
{
  "tenant_name": "示例学校",
  "allow_register": false,
  "admin_username": "admin",
  "admin_real_name": "租户管理员",
  "admin_phone": "13800000000",
  "admin_email": "admin@example.com",
  "admin_password": "InitialPassword123"
}
```

### 4.2 Service 流程

```text
1. 校验当前操作者必须是 platform_admin
2. 创建 tenant
3. 生成全局唯一 tenant_code
4. 创建或复用全局租户用户 users
5. 给该用户写入 tenant_user_memberships: tenant_admin
6. 不自动创建默认空间
7. 不把首个 tenant_admin 写入 space_members
8. 提交事务
```

首版创建租户时不自动创建默认空间，也不自动把 `tenant_admin` 写入 `space_members`。`tenant_admin` 通过租户级权限管理所有空间资源；`space_members` 只表达空间内身份，不表达 `tenant_admin` 的全局权限。

如果后续需要自动创建默认空间，必须作为显式配置项引入，例如：

```yaml
create_default_space_on_tenant_create: false
```

伪代码：

```go
func CreateTenantWithAdmin(ctx, req) error {
    actor := ctx.Actor

    if actor.Type != ActorPlatformUser {
        return ErrForbidden
    }

    return db.Transaction(func(tx *gorm.DB) error {
        tenant := TenantDO{
            Name:          req.TenantName,
            TenantCode:    GenerateTenantCode(),
            AllowRegister: req.AllowRegister,
            Status:        "enabled",
            CreatedByType: "platform_user",
            CreatedBy:     actor.ID,
        }

        if err := tenantRepo.Create(tx, tenant); err != nil {
            return err
        }

        adminUser := UserDO{
            TenantID:      tenant.ID,
            Username:      req.AdminUsername,
            RealName:      req.AdminRealName,
            Phone:         req.AdminPhone,
            Email:         req.AdminEmail,
            PasswordHash:  HashPassword(req.AdminPassword),
            Status:        "enabled",
            CreatedByType: "platform_user",
            CreatedBy:     actor.ID,
        }

        if err := userRepo.Create(tx, adminUser); err != nil {
            return err
        }

        role := UserRoleDO{
            TenantID:      tenant.ID,
            UserID:        adminUser.ID,
            Role:          "tenant_admin",
            CreatedByType: "platform_user",
            CreatedBy:     actor.ID,
        }

        if err := userRoleRepo.Create(tx, role); err != nil {
            return err
        }

        return nil
    })
}
```

---

## 5. 平台管理员和租户管理员的关联方式

两者不需要直接关系表。

不建议首版建立：

```text
platform_admin_tenants
```

正确关系是：

```text
platform_users.id
  → tenants.created_by
  → tenants.created_by_type = platform_user

users.id
  → tenant_user_memberships.user_id
  → role = tenant_admin

tenants.id
  → tenant_user_memberships.tenant_id
```

也就是：

```text
平台管理员创建租户。
租户拥有租户用户。
租户用户通过 tenant_user_memberships 成为 tenant_admin。
```

---

## 6. 租户管理员不变式

每个启用状态租户必须至少保留一个启用状态的 `tenant_admin`。

以下操作必须校验该不变式：

```text
- 禁用租户管理员账号
- 删除租户管理员账号
- 移除 tenant_admin 角色
- 禁用租户用户
- 批量导入覆盖角色
```

Service 层统一提供函数：

```go
func ValidateTenantAdminInvariant(ctx context.Context, tenantID uint64) error
```

不变式校验必须在同一数据库事务内执行，并基于即将提交后的状态校验：

```text
1. 开启事务。
2. 执行目标变更。
3. 在同一事务内查询剩余启用状态 tenant_admin 数量。
4. 数量不足则回滚。
5. 数量满足才提交。
```

语义：

```text
查询 tenant_id 下：
- users.status = enabled
- users.deleted_at = 0
- tenant_user_memberships.role = tenant_admin
- tenant_user_memberships.status = enabled

如果数量 < 1，则返回错误。
```

错误信息：

```text
租户至少需要保留一个启用状态的租户管理员
```

---

## 7. 空间管理员不变式

每个启用状态空间必须至少保留一个启用状态的 `space_admin`。

以下操作必须校验：

```text
- 禁用空间成员
- 移除空间成员
- 修改空间成员角色
- 禁用租户用户
- 删除租户用户
```

Service 层统一提供函数：

```go
func ValidateSpaceAdminInvariant(ctx context.Context, tenantID uint64, spaceID uint64) error
```

不变式校验必须在同一数据库事务内执行，并基于即将提交后的状态校验。不能先校验再更新，否则两个并发操作可能同时通过校验，最终删除或禁用最后一个空间管理员。

标准流程：

```text
1. 开启事务。
2. 执行禁用、移除或角色变更。
3. 在同一事务内查询剩余启用状态 space_admin 数量。
4. 数量不足则回滚。
5. 数量满足才提交。
```

错误信息：

```text
空间至少需要保留一个启用状态的空间管理员
```

---

## 8. PermissionChecker 设计

首版不要在 API 层散落角色判断。

统一通过：

```go
type PermissionChecker interface {
    CanManagePlatform(ctx PermissionContext) error
    CanManageTenantLifecycle(ctx PermissionContext, tenantID uint64) error
    CanManageTenantBusiness(ctx PermissionContext, tenantID uint64) error
    CanManageTenantUsers(ctx PermissionContext, tenantID uint64) error
    CanManageSpaceProfile(ctx PermissionContext, spaceID uint64) error
    CanManageSpaceMembers(ctx PermissionContext, spaceID uint64) error
    CanManageQuestion(ctx PermissionContext, questionID uint64) error
    CanManagePaper(ctx PermissionContext, paperID uint64) error
    CanPublishExam(ctx PermissionContext, paperID uint64) error
    CanGradeAttempt(ctx PermissionContext, attemptID uint64) error
    CanViewExamResults(ctx PermissionContext, examID uint64) error
    CanExportExamResults(ctx PermissionContext, examID uint64) error
    CanViewOwnResult(ctx PermissionContext, resultID uint64) error
    CanTakeExam(ctx PermissionContext, examID uint64) error
}
```

权限上下文：

```go
type PermissionContext struct {
    ActorType string // platform_user / tenant_user
    ActorID   uint64
    TenantID  uint64
    Role      string // 首版单租户级角色：platform_admin / tenant_admin / teacher / student
}
```

说明：

```text
现有实现中的 AuthPrincipal 可以作为登录态来源，但不要长期保留 AuthPrincipal、ActorContext、PermissionContext 三套平行模型。
落地时应由认证中间件把 session 解析成统一 ActorContext，再转换成 service 层使用的 PermissionContext。
```

接口语义必须覆盖所有高频业务操作。用户管理、空间成员管理、试卷管理、成绩查看和学生查看自己成绩不能在 service 中散落角色字符串判断。

`CanManageTenantLifecycle` 表示平台侧租户生命周期管理，包括创建租户、启停租户、重置租户码、注册开关和租户基础资料。只有 `platform_admin` 可通过。

`CanManageTenantBusiness` 表示租户内业务管理，包括用户、空间、题库、试卷、考试、阅卷和成绩。只有目标租户内 `tenant_admin` 可通过。

`CanManageSpaceProfile` 表示空间基础资料管理，包括修改空间名称、Logo、描述和状态。首版只有本租户 `tenant_admin` 可通过，空间管理员不能修改空间基础资料或删除空间。

`CanManageSpaceMembers` 表示空间成员管理，包括查看、添加、移除和修改空间成员。首版本租户 `tenant_admin` 或当前空间 `space_admin` 可通过。

---

## 9. 固定角色权限规则

### 9.1 platform_admin

```go
CanManagePlatform = true
CanManageTenantLifecycle = true
CanManageTenantBusiness = false

CanManageQuestion = false
CanManagePaper = false
CanPublishExam = false
CanGradeAttempt = false
CanTakeExam = false
```

平台管理员不直接通过租户业务权限。

---

### 9.2 tenant_admin

```go
CanManageTenantUsers = true
CanManageTenantBusiness = true
CanManageSpaceProfile = true
CanManageSpaceMembers = true
CanManageQuestion = true within tenant
CanManagePaper = true within tenant
CanPublishExam = true within tenant
CanGradeAttempt = true within tenant
CanViewExamResults = true within tenant
CanExportExamResults = true within tenant
```

---

### 9.3 space_admin

```go
CanManageSpaceProfile = false
CanManageSpaceMembers = true only joined space
CanManageQuestion = true only joined space
CanManagePaper = true only joined space
CanPublishExam = true only joined space
CanGradeAttempt = true only joined space
CanViewExamResults = true only joined space
CanExportExamResults = true only joined space
```

---

### 9.4 teacher

```go
CanManageQuestion = true only authorized space
CanManagePaper = true only authorized space
CanPublishExam = true only authorized space
CanGradeAttempt = true only authorized space
CanViewExamResults = true only authorized space
CanExportExamResults = false by default
```

---

### 9.5 student

```go
CanTakeExam = true only targeted exam
CanViewOwnResult = true
```

### 9.6 成绩权限边界

```text
CanViewExamResults:
  tenant_admin:
    exam.tenant_id == ctx.TenantID。
  space_admin / teacher:
    exam 所属试卷、考试发布范围或成绩归属空间必须命中已加入且启用的授权空间。
  student:
    不允许查看考试成绩列表。

CanExportExamResults:
  tenant_admin:
    允许导出本租户考试成绩。
  space_admin:
    允许导出本空间考试成绩。
  teacher:
    首版默认不允许导出；后续如需开放，必须单独配置和审计。
  student:
    不允许导出成绩。

CanViewOwnResult:
  result.user_id == ctx.ActorID。
  必须满足成绩发布策略和可见时间。
```

### 9.7 空间权限实现路径

`tenant_admin` 不强制写入 `space_members`。因此空间资源权限必须显式支持两条路径：

```text
tenant_admin:
  只校验目标资源归属 tenant_id == ctx.TenantID。

space_admin / teacher:
  必须校验 space_members 存在、tenant_id 匹配、status = enabled，并且 role_in_space 匹配授权动作。
```

如果实现 `CanManageSpaceProfile(ctx, spaceID)`、`CanManageSpaceMembers(ctx, spaceID)`、`CanManageQuestion(ctx, questionID)`、`CanManagePaper(ctx, paperID)`、`CanPublishExam(ctx, paperID)`、`CanGradeAttempt(ctx, attemptID)`、`CanViewExamResults(ctx, examID)` 或 `CanExportExamResults(ctx, examID)`，必须先从资源反查真实 `tenant_id` 和 `space_id`，不能信任请求参数拼接授权范围。

实现时禁止通过 `ctx.Role == "space_admin"` 判断空间管理员。正确路径只有两类：

```text
ctx.Role == "tenant_admin"
  表示租户级管理员，可覆盖本租户空间资源。

spaceMember.RoleInSpace == "space_admin"
  表示当前用户在指定空间内具备空间管理员身份。
```

### 9.8 公共资源权限规则

```text
questions.space_id = NULL 表示租户公共题库。
papers.space_id = NULL 表示租户公共试卷。
```

首版公共题库和公共试卷只允许 `tenant_admin` 创建、修改、删除和导入。`space_admin` / `teacher` 只能管理自己已加入且启用空间内的题库和试卷。

`space_admin` / `teacher` 可以在组卷、发布考试等流程中读取公共题库和公共试卷，但是否允许引用公共资源必须由对应业务 service 显式校验，不能把公共资源视为任意教师可写资源。

### 9.9 教师零空间状态

`teacher` 是租户级角色标签，实际业务范围来自 `space_members`。教师没有加入任何启用空间时是合法状态，但该教师没有题库、试卷、考试、阅卷等实际操作权限。

`tenant_admin` 创建 `teacher` 用户时，只创建租户用户和租户级 `teacher` 角色，不自动加入任何空间。教师加入空间必须通过空间成员接口显式分配；空间成员写入必须校验目标空间启用且未删除；新增成员和修改空间身份时，目标用户还必须属于当前租户且启用未删除，并且空间内角色只能是 `space_admin`、`teacher` 或 `student`。启用已有空间成员只恢复成员关系状态，允许租户用户账号暂时停用，但有效授权查询仍必须过滤未启用账号。

创建或编辑教师时，首版不强制绑定空间；前端用户详情和空间分配入口必须提示：

```text
该教师暂未加入任何空间，当前无法操作题库、试卷、考试或阅卷。
```

---

## 10. API 分组边界

平台接口：

```text
/api/v1/platform/**
```

仅 `platform_admin` 可访问。

包括：

```text
POST   /api/v1/platform/tenants
GET    /api/v1/platform/tenants
POST   /api/v1/platform/tenants/:id/disable
POST   /api/v1/platform/tenants/:id/enable
POST   /api/v1/platform/tenants/:id/reset-code
POST   /api/v1/platform/tenants/:id/admins
POST   /api/v1/platform/tenant-admins/:id/reset-password
```

租户接口：

```text
/api/v1/tenant/**
```

仅 `tenant_user` 可访问，并且必须带 `tenant_id` 上下文。

考试端接口：

```text
/api/v1/exam-entry/**
```

仅目标考生或持有有效考试会话的考生可访问，不归入管理端租户接口。

### 10.1 本期 API 权限矩阵

本期按目标权限模型改造。路径迁移可以分阶段落地，但权限语义必须先按下表收口。

| 当前代码路径 | 目标分组 | 允许主体 | 说明 |
| --- | --- | --- | --- |
| `POST /api/v1/auth/platform/login` | `/api/v1/auth/platform/login` | 匿名 | 平台管理员登录。 |
| `POST /api/v1/auth/tenant/login` | `/api/v1/auth/tenant/login` | 匿名 | 租户用户使用通用账号登录，不提交租户 ID，登录后进入租户空间选择页。 |
| `POST /api/v1/auth/tenant/select-space` | `/api/v1/auth/tenant/select-space` | 已登录 `tenant_user` | 从当前账号可进入的租户和空间中选择目标入口，成功后把 session 绑定到目标租户和角色。 |
| `POST /api/v1/auth/logout` | `/api/v1/auth/logout` | 匿名或已登录主体 | 清除当前 session，并下发过期的 HttpOnly session cookie。 |
| `GET/POST /api/v1/profile` | `/api/v1/profile` | 已登录主体 | 只操作当前账号资料。 |
| `GET /api/v1/tenants` | `/api/v1/platform/tenants` | `platform_admin` | 平台侧租户列表。 |
| `POST /api/v1/tenants` | `/api/v1/platform/tenants` | `platform_admin` | 创建租户并初始化首个 `tenant_admin`。 |
| `POST /api/v1/tenants/:id/profile` | `/api/v1/platform/tenants/:id/profile` | `platform_admin` | 维护租户基础资料。 |
| `POST /api/v1/tenants/:id/reset-code` | `/api/v1/platform/tenants/:id/reset-code` | `platform_admin` | 重置租户码。 |
| `POST /api/v1/tenants/:id/register-setting` | `/api/v1/platform/tenants/:id/register-setting` | `platform_admin` | 控制租户注册开关。 |
| `GET /api/v1/spaces` | `GET /api/v1/tenant/spaces` | `tenant_admin` | 租户管理员查看本租户全部空间。 |
| `POST /api/v1/spaces` | `POST /api/v1/tenant/spaces` | `tenant_admin` | 租户管理员创建本租户空间。 |
| `PUT /api/v1/spaces/:id` | `PUT /api/v1/tenant/spaces/:id` | `tenant_admin` | 租户管理员维护空间基础信息。 |
| `DELETE /api/v1/spaces/:id` | `DELETE /api/v1/tenant/spaces/:id` | `tenant_admin` | 租户管理员删除或停用空间。 |
| `GET /api/v1/spaces/:id/members` | `GET /api/v1/tenant/spaces/:id/members` | `tenant_admin` / 当前空间 `space_admin` | 查看空间成员必须校验目标空间归属和空间内身份。 |
| `POST /api/v1/spaces/:id/members` | `POST /api/v1/tenant/spaces/:id/members` | `tenant_admin` / 当前空间 `space_admin` | 添加空间成员必须校验目标空间归属和空间内身份。 |
| `PUT /api/v1/spaces/:id/members/:user_id` | `PUT /api/v1/tenant/spaces/:id/members/:user_id` | `tenant_admin` / 当前空间 `space_admin` | 修改空间成员角色或状态前必须校验空间管理员不变式。 |
| `DELETE /api/v1/spaces/:id/members/:user_id` | `DELETE /api/v1/tenant/spaces/:id/members/:user_id` | `tenant_admin` / 当前空间 `space_admin` | 移除空间成员前必须校验空间管理员不变式。 |
| `GET/POST /api/v1/users` | `/api/v1/tenant/users` | `tenant_admin` | 租户管理员管理本租户用户。 |
| `POST /api/v1/users/:id/disable` | `/api/v1/tenant/users/:id/disable` | `tenant_admin` | 禁用用户前校验租户管理员和空间管理员不变式。 |
| `POST /api/v1/users/:id/enable` | `/api/v1/tenant/users/:id/enable` | `tenant_admin` | 启用已禁用的租户用户，恢复其租户成员状态。 |
| `POST /api/v1/uploads` | `/api/v1/uploads` | `platform_admin` / `tenant_admin` | 当前通用上传接口同时承载平台租户 Logo 和租户内头像、空间 Logo；平台管理员只用于平台租户管理上传，租户用户按业务用途收口。 |
| `GET/POST /api/v1/questions` | `/api/v1/tenant/questions` | `tenant_admin` / `space_admin` / `teacher` | 题库操作必须受租户或授权空间限制。 |
| `POST /api/v1/questions/import` | `/api/v1/tenant/questions/import` | `tenant_admin` / `space_admin` / `teacher` | 导入题目按题库权限校验。 |
| `/api/v1/papers/**` | `/api/v1/tenant/papers/**` | `tenant_admin` / `space_admin` / `teacher` | 试卷、章节和组卷规则按租户或授权空间校验。 |
| `GET/POST /api/v1/exams` | `/api/v1/tenant/exams` | `tenant_admin` / `space_admin` / `teacher` | 考试列表和发布按租户或授权空间校验。 |
| `/api/v1/grading/**` | `/api/v1/tenant/grading/**` | `tenant_admin` / `space_admin` / `teacher` | 阅卷操作按考试归属和授权空间校验。 |
| `GET /api/v1/results/exams/:id` | `GET /api/v1/tenant/results/exams/:id` | `tenant_admin` / `space_admin` / `teacher` | 管理端查看范围内考试成绩列表，学生不走此接口。 |
| `GET /api/v1/results/exams/:id/export` | `GET /api/v1/tenant/results/exams/:id/export` | `tenant_admin` / `space_admin` | 成绩导出是高风险操作，首版默认不开放给教师。 |
| `GET /api/v1/results/:id` | `GET /api/v1/tenant/results/:id` | `tenant_admin` / `space_admin` / `teacher` | 管理端按授权范围查看成绩详情，学生不走此接口。 |
| `/api/v1/exams/invite/resolve` | `/api/v1/exam-entry/invite/resolve` | 已登录 `student` | 考试入口独立于管理端分组。 |
| `/api/v1/exams/:id/attempts/start` | `/api/v1/exam-entry/exams/:id/attempts/start` | 目标 `student` | 必须校验考试目标和当前学生身份。 |
| `/api/v1/exam-attempts/**` | `/api/v1/exam-entry/attempts/**` | 持有有效考试会话的 `student` | 答题、提交和事件记录不进入管理端权限分组。 |
| `GET /api/v1/results/:id` | `GET /api/v1/exam-entry/results/:id` | 作答本人 | 考生查看自己的已发布成绩，必须满足发布策略和可见时间。 |

平台管理员查看租户空间、用户等信息时，只能走平台侧只读概览接口。首版不允许平台管理员直接调用租户业务写接口，也不做 impersonation。

学生成绩查看不走 `/api/v1/tenant/results/:id`。管理端成绩详情和学生查分必须拆成两个权限入口：

```text
GET /api/v1/tenant/results/:id:
  仅用于管理端成绩详情。
  允许 tenant_admin / 授权空间 space_admin / 授权空间 teacher。
  不允许 student。

GET /api/v1/exam-entry/results/:id:
  用于考生查看自己的已发布成绩。
  只允许作答本人；空间投放考试中，作答本人可以是空间内 student 身份，而不要求租户级角色必须为 student。
  必须满足成绩发布策略和可见时间。
```

### 10.2 考试入口认证上下文

`/api/v1/exam-entry/**` 不使用平台/租户管理端权限中间件。考试过程使用独立 `exam_token`，中间件负责校验 token 并构造考试入口上下文。

```go
type ExamEntryContext struct {
    ActorType string // 固定为 tenant_user。
    Role      string // 固定为 student。
    TenantID  uint64
    UserID    uint64
    ExamID    uint64
    AttemptID uint64
}
```

校验规则：

```text
- exam_token 使用服务端随机不透明 token，数据库只保存 hash。
- 中间件必须按 hash 找到未过期、未提交或仍允许保存的 attempt。
- token 校验必须同时校验 tenant_id、exam_id、attempt_id、user_id 和作答截止时间。
- exam_token 只能访问当前 attempt 的答题、提交和事件接口。
- exam_token 过期后不续期；重复调用开考接口复用已有 in-progress attempt，但必须续发新的明文 exam_token 并更新 exam_attempts.exam_token_hash。
- 普通登录 session 不能调用自动保存、提交答卷和考试事件接口。
- exam_token 不能调用 profile、tenant、questions、papers、grading、results 等后台接口。
- exam_token 过期、attempt 已提交、考试已超过可保存窗口时必须拒绝写入。
- 用户被禁用或不再属于考试目标时，新的开考请求必须拒绝；已签发 exam_token 的自动保存按 attempt 状态和作答截止时间收口。
```

`ExamEntryContext` 可以在进入 service 前转换成 `PermissionContext`，但不能把 `exam_token` 当作普通后台 session 使用。

---

## 11. 认证上下文

登录后必须区分主体类型。

```go
type ActorContext struct {
    ActorType string // platform_user / tenant_user
    ActorID   uint64
    TenantID  uint64 // platform_user 为空或 0
    Role      string // 首版单角色
}
```

租户通用账号登录后先形成未绑定租户空间的 `tenant_user` session：

```json
{
  "actor_type": "tenant_user",
  "actor_id": 1001,
  "tenant_id": 0,
  "role": "tenant_user"
}
```

该 session 只能访问个人资料、可进入租户空间列表和空间选择接口。用户在 `/tenant-entry` 选择目标租户空间后，后端重新写入带 `tenant_id` 和租户级角色的 session，再进入对应管理端或考试入口。

平台管理员登录：

```json
{
  "actor_type": "platform_user",
  "actor_id": 1,
  "tenant_id": 0,
  "role": "platform_admin"
}
```

租户管理员登录：

```json
{
  "actor_type": "tenant_user",
  "actor_id": 1001,
  "tenant_id": 10,
  "role": "tenant_admin"
}
```

教师登录：

```json
{
  "actor_type": "tenant_user",
  "actor_id": 1002,
  "tenant_id": 10,
  "role": "teacher"
}
```

角色变更后，已登录 session 中的角色快照可能过期。首版按最低要求先保证关键接口实时失效，主动 revoke 作为增强项：

```text
首版最低要求：
- 租户、用户、空间、题库、试卷、考试、阅卷、成绩发布、成绩导出和上传等关键写接口必须实时从数据库重建角色和状态。
- 用户被禁用后，关键写接口必须立即失效。
- session 主动 revoke 如果当前 provider 暂不支持，不阻塞首版权限主线。

增强要求：
- memory provider 增加 actor_type + tenant_id + user_id -> session_key 反向索引。
- redis provider 增加 actor_type + tenant_id + user_id -> session_key set，用于批量删除。
```

即使后续支持主动 revoke，关键写接口仍必须以数据库中的当前用户状态和角色为准。

---

## 12. 首版不做的事情

首版不实现：

```text
- 复杂 RBAC
- 菜单级权限
- 按钮级权限
- 平台管理员 impersonation
- 租户管理员审批流
- 平台管理员直接进入租户后台
- 平台管理员直接修改成绩
- 平台管理员直接阅卷
```

但保留扩展口：

```text
- PermissionChecker 抽象
- created_by_type / updated_by_type
- 后续可增加 impersonation_sessions 表
- 后续可把 `role` 扩展为 `roles[]`
```

---

## 13. 后续可扩展 impersonation

首版不做，但设计时不要堵死。

后续可增加：

```text
impersonation_sessions
├── id
├── platform_user_id
├── tenant_id
├── target_user_id
├── reason
├── started_at
├── ended_at
├── status
└── ext_json
```

代入模式下，审计字段仍然要记录真实操作者：

```text
actor_type = platform_user
actor_id = 平台管理员 ID
impersonated_tenant_id = 目标租户 ID
impersonated_user_id = 被代入用户 ID
```

---

## 14. Codex 开发任务拆分

### Task 1：调整审计字段

为核心表增加：

```text
created_by_type
updated_by_type
```

保留现有：

```text
created_by
updated_by
```

`created_by` / `updated_by` 继续保存操作者 ID，新增的两个类型字段负责解释 ID 来源。

当前项目仍处于新项目初始化建库阶段，不存在历史生产库升级诉求；审计主体类型、全局 `users` 表、`tenant_user_memberships` 租户成员关系表和单角色唯一约束直接落在 `001_tenant_space.sql` 初始建库脚本中。本次需求不新增 `002_audit_actor_type.sql`，也不新增其他 `002_*` 数据库迁移脚本，不执行追加迁移动作。

至少覆盖：

```text
tenants
users
tenant_user_memberships
spaces
space_members
questions
papers
exams
```

如果为了首版简化，也可以先只覆盖：

```text
tenants
users
tenant_user_memberships
space_members
```

---

### Task 2：统一认证上下文

目标：

```text
不要新增多套平行上下文。
将现有 session 主体统一收口为 ActorContext，再转换为 PermissionContext。
```

定义：

```go
type ActorType string

const (
    ActorPlatformUser ActorType = "platform_user"
    ActorTenantUser   ActorType = "tenant_user"
    ActorSystem       ActorType = "system"
)

type ActorContext struct {
    ActorType ActorType
    ActorID   uint64
    TenantID  uint64
    Role      string
}
```

当前实现中的 `subject_type` 可以在迁移期映射为 `actor_type`，但新接口响应和文档统一使用 `actor_type`。

---

### Task 3：实现创建租户并初始化管理员

新增接口：

```text
POST /api/v1/platform/tenants
```

实现：

```text
CreateTenantWithAdmin
```

要求：

```text
- 必须 platform_admin 调用
- tenant 和 admin user 在同一事务创建
- admin user 自动授予 tenant_admin
- tenant_code 全局唯一
- 失败整体回滚
```

---

### Task 4：实现租户管理员不变式

新增：

```go
ValidateTenantAdminInvariant(tenantID uint64) error
```

接入：

```text
- 禁用用户
- 删除用户
- 移除 tenant_admin 角色
```

---

### Task 5：实现空间管理员不变式

新增：

```go
ValidateSpaceAdminInvariant(tenantID uint64, spaceID uint64) error
```

接入：

```text
- 禁用空间成员
- 移除空间成员
- 修改空间成员角色
- 禁用租户用户
```

---

### Task 6：实现 FixedRolePermissionChecker

新增：

```text
server/internal/service/permission/fixed_role.go
```

实现：

```go
type FixedRolePermissionChecker struct {}
```

规则：

```text
platform_admin 只能管理平台和租户生命周期
tenant_admin 可管理本租户内所有业务
space_admin 可管理本空间业务
teacher 可管理授权空间内教学业务
student 只能考试和看自己成绩
```

---

### Task 7：接口分组中间件

平台接口：

```go
RequireActorType(platform_user)
RequireRole(platform_admin)
```

租户接口：

```go
RequireActorType(tenant_user)
RequireTenantContext()
```

禁止：

```text
platform_user 直接访问 /api/v1/tenant/**
tenant_user 访问 /api/v1/platform
非目标考生访问 /api/v1/exam-entry/**
```

---

### Task 8：前端入口和会话改造

前端需要同步：

```text
- 登录态增加 actorType，保留单个 role。
- 平台后台只允许 platform_admin 进入。
- 租户登录页不再要求输入租户 ID，登录成功后跳转 `/tenant-entry`。
- `/tenant-entry` 读取当前账号可进入的租户和空间，调用 `/api/v1/auth/tenant/select-space` 后再进入目标空间。
- 租户后台只允许已选择租户空间后的 tenant_user 进入。
- tenant_admin 菜单必须包含租户用户、空间、题库、试卷、考试、阅卷、成绩。
- 平台管理员不能通过 tenant_id 参数直接进入租户业务页。
- 创建租户表单必须补充首个租户管理员账号信息。
- 教师没有加入任何空间时，用户详情和业务页必须提示无授权空间。
- 角色变更或禁用用户后，本地登录态必须清理；后端返回 401/403 时前端必须刷新登录态。
```

---

## 15. 必须补的测试用例

### 平台管理员相关

```text
- platform_admin 可以创建租户
- 创建租户时必须创建首个 tenant_admin
- 创建租户和 tenant_admin 任一步失败必须回滚
- platform_admin 不能调用题库创建接口
- platform_admin 不能调用阅卷接口
- platform_admin 不能调用成绩修改接口
```

### 租户管理员相关

```text
- tenant_admin 可以创建空间
- tenant_admin 可以创建教师
- tenant_admin 创建 teacher 时不自动写入 space_members
- tenant_admin 可以分配 space_admin
- tenant_admin 可以管理本租户题库
- tenant_admin 可以管理租户公共题库和公共试卷
- tenant_admin 可以管理本租户试卷、考试、阅卷和成绩
- tenant_admin 不在 space_members 中也可以管理本租户空间资源
- tenant_admin 不能访问其他租户数据
- 禁止移除最后一个 tenant_admin
- 禁止禁用最后一个 tenant_admin
- 同一个租户用户不能同时拥有多个租户级角色
- 角色变更后，关键写接口必须从数据库重建权限上下文，不能继续信任旧 session 角色快照
```

### 空间管理员相关

```text
- space_admin 可以管理本空间成员
- space_admin 不能修改空间基础资料或删除空间
- space_admin 不能管理其他空间成员
- space_admin 不能创建、修改、删除或导入租户公共题库和公共试卷
- 禁止移除最后一个 space_admin
- 禁止禁用最后一个 space_admin
```

### 教师相关

```text
- teacher 可以管理授权空间题库
- teacher 不能创建、修改、删除或导入租户公共题库和公共试卷
- teacher 不能管理空间成员
- teacher 不能管理其他空间题库
- teacher 未加入任何启用空间时不能操作题库、试卷、考试或阅卷
```

### 学生相关

```text
- student 可以参加目标考试
- student 不能访问管理端接口
- student 只能通过 /api/v1/exam-entry/results/:id 查看自己的已发布成绩
- exam_token 只能访问当前 attempt 的答题、提交和事件接口
- exam_token 过期后不续期；重复开考会为同一个 in-progress attempt 续发新的明文 token 并替换 hash
- 普通登录 session 不能调用答题、提交和事件接口
- exam_token 过期、attempt 已提交或超过作答截止时间后必须拒绝写入
```

### API 分组相关

```text
- platform_admin 可以访问 /api/v1/platform/**
- tenant_user 不能访问 /api/v1/platform/**
- platform_user 不能访问 /api/v1/tenant/**
- student 不能访问 /api/v1/tenant/results/:id
- tenant_admin 可以访问本租户管理端业务接口
- space_admin / teacher 只能访问授权空间内业务资源
- 考试入口接口不被管理端分组误拦截
- PermissionChecker 覆盖用户管理、空间成员管理、试卷管理、成绩查看和学生查看自己成绩
```

---

## 16. 最终结论

首版实现成下面这个模型即可：

```text
platform_admin
  ├── 创建租户
  ├── 初始化首个 tenant_admin
  └── 管平台治理，不直接管租户业务

tenant_admin
  ├── 管租户用户
  ├── 管空间
  ├── 管题库
  ├── 管试卷
  ├── 管考试
  └── 管成绩

space_admin
  └── 管指定空间内的人、题、卷、考、成绩

teacher
  └── 做出题、组卷、考试、阅卷

student
  └── 考试和查自己成绩
```

最重要的实现边界：

```text
平台管理员和租户管理员不通过角色继承关联。
平台管理员通过创建 tenant 和初始化 tenant_admin 形成治理关系。
租户管理员通过全局 users + tenant_user_memberships 归属于租户。
业务权限统一走 PermissionChecker。
平台接口和租户接口必须物理分组隔离。
```

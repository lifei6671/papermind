# PaperMind 在线考试平台技术方案

## 1. 目标与边界

PaperMind 首版定位为小而美的在线 SaaS 考试平台，覆盖初高中、中专、大学、专业领域考试，以及企业知识课堂的临时考试。

首版优先完成考试业务闭环：

- 多租户管理
- 用户注册、导入与空间分配
- 在线出题与题库导入
- 手动组卷与规则组卷
- 考试发布与邀请码
- 考生级随机抽题与选项随机
- 答题自动保存与提交锁定
- 客观题自动判分
- 简答题人工阅卷
- 成绩发布与导出

首版并发目标：

- 支持单场考试约 100 名考生同时在线作答。
- 正式多人考试推荐使用 PostgreSQL 或 MySQL。
- SQLite 单机版仅用于演示、本地开发和低并发小规模考试，不作为 100 人正式考试的推荐部署方式。

首版不做大而全的平台能力：

- 摄像头监考
- 人脸识别
- 锁屏客户端
- 录屏
- AI 判卷
- 复杂 RBAC
- 多租户独立数据库
- Kubernetes 部署
- 题目评论或讨论区
- 在线直播课堂

## 2. 技术栈

首版采用前后端分离的模块化单体架构：

```text
React Web 管理端 / 考试端
             ↓
        REST API v1
             ↓
Go + Gin 模块化单体
             ↓
GORM Repository
             ↓
PostgreSQL / MySQL / SQLite
```

技术选型：

- 后端：Go + Gin
- 前端：React + Vite + TypeScript
- ORM：GORM
- 数据库驱动：`gorm.io/driver/sqlite`、`gorm.io/driver/mysql`、`gorm.io/driver/postgres`
- JSON 字段：`gorm.io/datatypes`
- 配置：YAML
- 数据库：PostgreSQL 优先，兼容 MySQL，支持 SQLite 单机版
- API：REST API，统一使用 `/api/v1`

后续小程序端复用同一套业务 API，不单独拆一套考试业务接口。

## 3. 目录结构

项目目录建议：

```text
papermind
├── server
│   ├── api
│   │   ├── router
│   │   ├── middleware
│   │   ├── request
│   │   ├── response
│   │   └── v1
│   ├── bootstrap
│   ├── cmd
│   │   └── papermind
│   ├── conf
│   ├── data
│   │   ├── migrations
│   │   ├── seeds
│   │   ├── sqlite
│   │   ├── imports
│   │   └── exports
│   ├── internal
│   │   ├── service
│   │   ├── dao
│   │   │   ├── db
│   │   │   └── external
│   │   ├── model
│   │   ├── dto
│   │   └── job
│   ├── library
│   │   ├── code
│   │   ├── constant
│   │   ├── logger
│   │   ├── config
│   │   ├── validator
│   │   ├── response
│   │   ├── crypto
│   │   └── xerr
│   ├── mock
│   ├── script
│   └── tests
├── web
├── deployments
│   ├── docker-compose
│   └── sqlite-single-node
└── docs
    ├── specs
    ├── api
    └── database
```

职责边界：

- `server/api`：所有路由注册、入参解析、调用 service、返回前端数据，类似 controller 层。
- `server/bootstrap`：启动预加载、数据库初始化、配置加载、日志初始化、默认数据初始化。
- `server/cmd`：程序 main 入口。
- `server/conf`：开发环境 YAML 配置模板。
- `server/data`：数据库迁移 SQL、字典文件、SQLite 数据库文件、导入导出临时文件。
- `server/internal/service`：服务层接口与业务实现，禁止直接写 SQL，禁止直接操作数据库。
- `server/internal/dao`：数据库仓储、实体映射、第三方接口调用。
- `server/internal/dao/db`：GORM 实体、字段映射、Repository、事务封装。
- `server/internal/dao/external`：短信、对象存储、微信登录等第三方接口。
- `server/library`：工具类、错误码、常量、统一响应、日志、配置结构。
- `server/mock`：接口 mock 和测试替身。
- `server/script`：启动、迁移、构建、导入模板生成等脚本。
- `server/tests`：集成测试和 API 测试。

仓库不提供 `conf_online` 生产配置目录，避免生产隐私配置被误提交。生产配置通过环境变量、Docker secret 或部署平台密钥管理能力注入。

## 4. 后端分层规范

后端按以下调用链组织：

```text
api
  → service
  → dao/db 或 dao/external
```

分层规则：

- `api` 只处理 HTTP 相关逻辑。
- `service` 处理业务规则、权限判断、事务编排。
- `service` 层禁止直接写 SQL，禁止直接使用 GORM。
- `dao/db` 负责所有数据库实体映射、字段映射和仓储方法。
- `dao/external` 负责第三方接口调用。
- `library` 不放具体业务逻辑。

权限校验优先放在 `service` 层。HTTP 中间件只负责解析登录态、租户上下文、请求 ID 和基础鉴权信息。

首版虽然不实现复杂 RBAC，但必须提供权限判断抽象层。`service` 层不能直接散落角色判断，应统一依赖 `PermissionChecker` 之类的权限接口。首版实现可以基于固定角色和空间成员关系判断，后续可替换为 RBAC 权限点、菜单权限、数据范围等实现。

建议目录：

```text
server/internal/service/permission
├── checker.go          # 权限判断接口
├── fixed_role.go       # 首版固定角色实现
└── context.go          # 权限上下文
```

接口语义示例：

```go
type PermissionChecker interface {
    CanManageTenant(ctx PermissionContext, tenantID uint64) error
    CanManageSpace(ctx PermissionContext, spaceID uint64) error
    CanManageQuestion(ctx PermissionContext, questionID uint64) error
    CanPublishExam(ctx PermissionContext, paperID uint64) error
    CanGradeAttempt(ctx PermissionContext, attemptID uint64) error
    CanTakeExam(ctx PermissionContext, examID uint64) error
}
```

权限抽象规则：

- API 层不做业务权限判断，只做认证和上下文解析。
- Service 层调用 `PermissionChecker`，不直接判断角色字符串。
- 首版 `fixed_role` 实现使用平台管理员、租户管理员、空间管理员、教师、考生固定角色。
- 后续 RBAC 扩展时，优先替换 `PermissionChecker` 实现，不改业务 service 调用方式。

## 5. 数据库规范

### 5.1 数据库支持策略

首版数据库支持策略：

- PostgreSQL：生产推荐，第一优先级验证。
- SQLite：单机版、演示环境、本地轻量部署，第一阶段一起验证；不推荐承载 100 人正式在线考试。
- MySQL：兼容目标，第二阶段补充完整验证。

GORM 数据库驱动固定使用：

- `gorm.io/driver/sqlite`
- `gorm.io/driver/mysql`
- `gorm.io/driver/postgres`

JSON 字段统一使用：

- `gorm.io/datatypes`
- Go 实体中的 `ext_json` 字段使用 `datatypes.JSON`
- 如需要基于 JSON key 做非核心查询，可使用 `datatypes.JSONQuery`

不同数据库驱动必须在 GORM 初始化阶段分支处理连接池参数。

数据库连接池是进程级基础设施，只在服务启动时初始化一次。`server/internal/dao/db` 暴露单例访问入口，DAO 和 Repository 只能复用该单例 `*gorm.DB`，不能在业务请求中重复创建数据库连接。

SQLite 默认配置面向演示和低并发单机模式，优先保证行为保守：

```yaml
database:
  driver: sqlite
  dsn: file:/var/lib/papermind/sqlite/papermind.db?_foreign_keys=on&_journal_mode=WAL&_busy_timeout=5000
  max_open_conns: 1
  max_idle_conns: 1
```

PostgreSQL 和 MySQL 可以按部署规模配置连接池，但首版默认保持保守值。

首版 100 人在线考试场景下，推荐部署 PostgreSQL 或 MySQL。SQLite 写入仍然是串行化能力，即使开启 WAL，也可能在自动保存和防作弊事件高频写入时出现排队或超时，因此只作为演示和低并发单机模式。

SQLite 连接池规则：

- 默认 `max_open_conns = 1`，适合本地开发、演示和极低并发。
- 如需 SQLite 小规模多人使用，可以通过配置提高到较小连接数，例如 `5-10`，以释放 WAL 的并发读能力。
- SQLite 必须通过 DSN `_foreign_keys=on` 或连接初始化语句启用外键约束，否则复合外键不会生效。
- 不建议将 SQLite 连接池放大后承载 100 人正式考试。
- SQLite 写事务必须短小，避免长事务占用写锁。
- 非关键 `exam_events` 必须异步写入，降低与自动保存、交卷写入争抢锁的概率。

SQLite 如需使用 `datatypes.JSONQuery` 等 JSON 查询能力，构建时必须启用 `json1` 标签，例如：

```text
go build -tags json1 ./server/cmd/papermind
go test -tags json1 ./server/...
```

如果 SQLite 只保存和读取完整 `ext_json`，不做 JSON key 查询，可以不依赖 JSON1 查询能力。但为了三库行为一致，项目构建脚本建议统一带上 `json1` 标签。

数据库迁移 SQL 按数据库类型拆分：

```text
server/data/migrations
├── postgres
├── mysql
└── sqlite
```

不同数据库的 SQL 不强行写成一套，避免为了兼容导致迁移文件难维护。

迁移框架规则：

- 迁移文件放在对应数据库目录中，文件名必须以递增数字版本号开头，例如 `001_create_tenants.sql`。
- 服务启动时根据 `database.driver` 自动选择迁移目录，并按版本号从小到大执行。
- 已执行版本记录到 `schema_migrations`，重复执行时自动跳过。
- 任意迁移失败必须立即停止启动流程，禁止服务运行在半迁移状态。
- 首版不提供单独数据库迁移脚本，避免部署流程和应用启动流程产生两套迁移入口。
- 迁移期间 HTTP 层必须返回“系统升级中”的中间页或稳定 JSON 响应，并带 `Retry-After`，避免迁移耗时较长时前端白屏或接口表现为未知错误。

数据库迁移 SQL 必须包含中文注释：

- 表必须有中文表注释。
- 字段必须有中文字段注释。
- 关键索引和唯一约束必须用 SQL 注释说明业务目的。
- MySQL 使用 `COMMENT` 声明表和字段注释。
- PostgreSQL 使用 `COMMENT ON TABLE` 和 `COMMENT ON COLUMN` 声明注释。
- SQLite 不支持原生字段注释，迁移 SQL 中必须在建表语句附近使用 `--` 注释说明表和字段含义。

### 5.2 基础字段

所有业务表必须包含以下基础字段：

```text
created_at   创建时间
created_by   创建人
updated_at   更新时间
updated_by   更新人
version      数据版本号
ext_json     JSON 扩展字段
```

基础规则：

- `created_at` 创建时写入。
- `created_by` 创建时写入当前用户 ID。
- `updated_at` 每次更新时写入。
- `updated_by` 每次更新时写入当前用户 ID。
- `version` 每次更新递增，用于乐观锁。
- `ext_json` 用于保存不影响主流程的扩展元数据，默认值为空 JSON 对象。

平台级表可不带 `tenant_id`。租户业务表必须带 `tenant_id`。

软删除策略：

- 核心主表使用软删除，统一采用 `gorm.io/plugin/soft_delete` 的 Unix 时间戳模式，避免不同数据库对 `NULL` 唯一索引处理不一致。
- 首版需要软删除的核心表包括：`tenants`、`spaces`、`platform_users`、`users`、`questions`、`papers`、`paper_sections`、`exams`。
- 纯关系表、事件表、答案表和考试快照表不做普通业务删除，不使用软删除。
- 默认业务查询必须过滤已删除记录。
- 历史答卷、成绩和考试快照不得依赖原始题库、试卷是否已删除。
- 软删除表的唯一约束必须包含 `deleted_at`，例如 `UNIQUE (tenant_id, username, deleted_at)`。
- `deleted_at = 0` 表示未删除，删除后写入删除时间戳。

追加写日志表和纯关系表可以按业务语义豁免 `updated_at`、`updated_by` 和 `version`。

- `exam_events` 是不可修改的考试事件流水，只保留 `created_at` 和 `created_by` 用于审计，不使用乐观锁字段，避免误导维护者认为事件可以被更新。
- `question_tags`、`exam_targets` 这类纯关系表只通过 INSERT / DELETE 维护关系，不做 UPDATE，可以只保留 `created_at` 和 `created_by`。
- 带状态字段的关系表不属于纯关系表，例如 `space_members` 有 `status` 和 `role_in_space`，仍然保留 `updated_at`、`updated_by` 和 `version`。

`ext_json` 使用规则：

- 所有表都保留 `ext_json` 字段，包括平台级表、关系表和日志表。
- Go 实体统一使用 `datatypes.JSON` 映射 `ext_json`。
- PostgreSQL 迁移 SQL 使用 `jsonb`，MySQL 使用 `json`，SQLite 使用 `text` 保存 JSON 字符串。
- `ext_json` 只能保存非核心扩展元数据，例如导入来源、外部系统引用、展示偏好、临时标记。
- 主流程字段不得放入 `ext_json`，例如权限、租户隔离、考试状态、判分答案、成绩、时间窗口、唯一标识。
- 首版不对 `ext_json` 建索引，不基于 `ext_json` 做关键业务查询。
- 写入 `ext_json` 前必须保证是合法 JSON 对象，禁止保存任意非结构化字符串。
- 如确实需要查询扩展字段，只允许用于非核心筛选，并优先使用 `datatypes.JSONQuery`。
- SQLite 下使用 `datatypes.JSONQuery` 时，构建和测试命令必须带 `json1` 标签。

### 5.3 GORM 实体与字段映射

数据库操作采用 GORM。实体必须显式声明列字段映射。

字段映射和数据库映射实体放在同一个文件里，便于维护和对照。

后续新增和修改的代码必须提供完善中文注释。注释应贴近实际实现，重点说明关键业务规则、状态流转、异常分支、跨层约束和容易误用的边界；禁止用“处理数据”“进行校验”这类空泛表述替代业务语义。修改业务逻辑时必须同步更新对应注释。

默认使用 GORM API 表达查询、插入、更新、删除。只有在 GORM 无法清晰表达，或确实存在性能、批量处理、数据库特性等必要场景时，才允许在 `dao/db` 的 repository 方法中编写原始 SQL。

示例：

```go
import "gorm.io/datatypes"

type UserDO struct {
    ID        uint64 `gorm:"column:id;primaryKey"`
    TenantID  uint64 `gorm:"column:tenant_id"`
    Username  string `gorm:"column:username"`
    CreatedAt int64  `gorm:"column:created_at"`
    CreatedBy uint64 `gorm:"column:created_by"`
    UpdatedAt int64  `gorm:"column:updated_at"`
    UpdatedBy uint64 `gorm:"column:updated_by"`
    Version   int64  `gorm:"column:version"`
    ExtJSON   datatypes.JSON `gorm:"column:ext_json"`
}

func (UserDO) TableName() string {
    return "users"
}

var UserColumns = struct {
    ID        string
    TenantID  string
    Username  string
    CreatedAt string
    CreatedBy string
    UpdatedAt string
    UpdatedBy string
    Version   string
    ExtJSON   string
}{
    ID:        "id",
    TenantID:  "tenant_id",
    Username:  "username",
    CreatedAt: "created_at",
    CreatedBy: "created_by",
    UpdatedAt: "updated_at",
    UpdatedBy: "updated_by",
    Version:   "version",
    ExtJSON:   "ext_json",
}
```

拼接 SQL、排序、动态筛选时，不允许直接硬编码字段名。

禁止：

```go
db.Where("tenant_id = ?", tenantID).Order("created_at desc")
```

允许：

```go
db.Where(UserColumns.TenantID+" = ?", tenantID).
    Order(UserColumns.CreatedAt + " desc")
```

复杂场景优先使用 GORM `clause.Column`，减少裸字符串字段名。

### 5.4 查询原则

数据库操作以简单查询为主：

- 优先单表查询。
- 必要时分步骤查询，并在 service 层组装业务结果。
- 只有确实有性能、分页、统计或一致性必要时才使用联表查询。
- 联表查询必须集中在明确的 repository 方法中。
- `api` 和 `service` 层不允许拼 SQL。
- 除非有必要或 GORM 实现不了，否则不写原始 SQL。

### 5.5 租户隔离

普通业务表统一带 `tenant_id`。

隔离规则：

- 登录后请求上下文必须包含 `tenant_id`。
- `service` 调用 `dao/db` 时必须传入租户上下文。
- Repository 查询必须按 `tenant_id` 过滤。
- 平台管理员接口和租户业务接口分开，避免平台权限误用租户接口。
- 关键表建立组合索引，例如 `tenant_id + id`、`tenant_id + exam_id`、`tenant_id + user_id`。

## 6. 核心业务模型

### 6.1 租户与空间

```text
tenants
├── id                 # 租户主键 ID
├── name               # 租户名称，例如学校、企业、培训机构
├── logo_url           # 企业或机构 Logo 地址
├── description        # 企业或机构描述
├── tenant_code        # 租户码，用于专属注册链接和手动注册归属
├── allow_register     # 是否允许该租户用户自注册
├── status             # 租户状态：enabled / disabled
├── created_at         # 创建时间
├── created_by         # 创建人用户 ID
├── updated_at         # 更新时间
├── updated_by         # 更新人用户 ID
├── deleted_at         # 软删除时间
├── version            # 数据版本号，用于乐观锁
└── ext_json           # JSON 扩展字段，保存非主流程元数据

spaces
├── id                 # 空间主键 ID
├── tenant_id          # 所属租户 ID
├── name               # 空间名称，例如班级、专业、课程、培训项目
├── logo_url           # 空间 Logo 地址，可为空
├── description        # 空间描述，可为空
├── type               # 空间类型：class / major / course / training / custom
├── status             # 空间状态：enabled / disabled
├── created_at         # 创建时间
├── created_by         # 创建人用户 ID
├── updated_at         # 更新时间
├── updated_by         # 更新人用户 ID
├── deleted_at         # 软删除时间
├── version            # 数据版本号，用于乐观锁
└── ext_json           # JSON 扩展字段，保存非主流程元数据

space_members
├── id                 # 空间成员关系主键 ID
├── tenant_id          # 所属租户 ID
├── space_id           # 空间 ID
├── user_id            # 租户用户 ID
├── role_in_space      # 空间内角色：space_admin / teacher / student
├── status             # 空间成员状态：enabled / disabled
├── created_at         # 创建时间
├── created_by         # 创建人用户 ID
├── updated_at         # 更新时间
├── updated_by         # 更新人用户 ID
├── deleted_at         # 软删除时间
├── version            # 数据版本号，用于乐观锁
└── ext_json           # JSON 扩展字段，保存非主流程元数据

space_configs
├── id                 # 空间配置主键 ID
├── tenant_id          # 所属租户 ID
├── space_id           # 空间 ID
├── config_key         # 配置键，例如 default_exam_duration
├── config_value       # 配置值，按字符串保存
├── value_type         # 配置值类型：string / number / bool / json
├── description        # 配置说明
├── created_at         # 创建时间
├── created_by         # 创建人用户 ID
├── updated_at         # 更新时间
├── updated_by         # 更新人用户 ID
├── version            # 数据版本号，用于乐观锁
└── ext_json           # JSON 扩展字段，保存非主流程元数据
```

业务规则：

- 平台管理员创建租户时，系统自动生成全平台唯一的 `tenant_code`。
- 新租户的 `allow_register` 可以在创建时显式指定；未指定时继承 `security.allow_register_default`。
- 平台管理员可以查看、复制、重置租户码。
- 平台管理员可以控制租户是否允许用户自注册。
- 用户通过租户专属注册链接或手动输入租户码注册。
- 租户自注册和管理端创建租户用户时，明文密码必须满足 `security.password_min_length` 后再写入哈希。
- 自注册用户默认属于租户，但不属于任何空间。
- 管理员或教师将用户加入空间后，用户才能参加对应空间考试。
- 创建空间时可以上传空间 Logo 和填写空间描述，但这两个字段不是必填项。
- 空间成员支持禁用。禁用后，该用户不再拥有该空间内的考试、组卷、阅卷等空间权限。
- 禁用空间成员对应 `status = disabled`，成员关系仍然存在，用于保留历史和审计。
- 移除空间成员对应写入 `deleted_at`，逻辑上该成员关系已不存在。
- 查询有效空间成员时必须同时满足 `status = enabled AND deleted_at = 0`。
- `space_members` 必须建立 `UNIQUE (tenant_id, space_id, user_id, deleted_at)`，防止同一用户在同一空间存在多条有效成员记录。
- 为保证空间可用，每个启用状态的空间至少必须保留一个启用状态的 `space_admin`。
- 禁用或移除空间成员时，如果会导致空间失去最后一个启用状态的 `space_admin`，系统必须拒绝操作并提示操作者。
- 修改空间成员角色时，如果会导致空间失去最后一个启用状态的 `space_admin`，系统必须拒绝操作并提示操作者。
- 空间管理员数量约束必须由 service 层统一不变式函数校验，例如 `ValidateSpaceAdminInvariant`，不能散落在各个接口里。
- 禁用成员、移除成员、修改角色、禁用租户用户等影响空间管理员数量的操作，都必须在同一事务提交前调用该不变式校验。
- 空间配置通过 `space_configs` 保存，配置项在同一空间内按 `config_key` 唯一。
- `space_configs` 必须建立 `UNIQUE (tenant_id, space_id, config_key)`。
- 配置表不使用软删除，删除配置项直接硬删除；平台配置和空间配置保持一致。

### 6.2 用户与角色

首版采用固定四角色：

- `platform_admin`
- `tenant_admin`
- `teacher`
- `student`

平台管理员是平台级身份，不归属任何租户。为避免普通业务表 `tenant_id` 过滤出现 `0` 或 `NULL` 特判，平台管理员单独存储在 `platform_users` 表中。

```text
platform_users
├── id                 # 平台管理员主键 ID
├── username           # 平台管理员登录名
├── avatar_url         # 用户头像地址
├── phone              # 手机号，可用于登录或找回账号
├── email              # 邮箱，可用于登录或通知
├── password_hash      # 密码哈希
├── last_login_ip      # 最后登录 IP
├── last_login_at      # 最后登录时间
├── status             # 平台管理员状态：enabled / disabled
├── created_at         # 创建时间
├── created_by         # 创建人用户 ID
├── updated_at         # 更新时间
├── updated_by         # 更新人用户 ID
├── deleted_at         # 软删除时间
├── version            # 数据版本号，用于乐观锁
└── ext_json           # JSON 扩展字段，保存非主流程元数据

platform_configs
├── id                 # 平台配置主键 ID
├── config_key         # 配置键，例如 allow_register_default
├── config_value       # 配置值，按字符串保存
├── value_type         # 配置值类型：string / number / bool / json
├── description        # 配置说明
├── created_at         # 创建时间
├── created_by         # 创建人用户 ID
├── updated_at         # 更新时间
├── updated_by         # 更新人用户 ID
├── version            # 数据版本号，用于乐观锁
└── ext_json           # JSON 扩展字段，保存非主流程元数据

users
├── id                 # 租户用户主键 ID
├── tenant_id          # 所属租户 ID
├── username           # 租户内登录名
├── real_name          # 真实姓名，用于阅卷、成绩单和导出
├── avatar_url         # 用户头像地址
├── phone              # 手机号，可用于登录或通知
├── email              # 邮箱，可用于登录或通知
├── password_hash      # 密码哈希
├── last_login_ip      # 最后登录 IP
├── last_login_at      # 最后登录时间
├── status             # 用户状态：enabled / disabled
├── created_at         # 创建时间
├── created_by         # 创建人用户 ID
├── updated_at         # 更新时间
├── updated_by         # 更新人用户 ID
├── deleted_at         # 软删除时间
├── version            # 数据版本号，用于乐观锁
└── ext_json           # JSON 扩展字段，保存非主流程元数据

user_roles
├── id                 # 用户角色关系主键 ID
├── tenant_id          # 所属租户 ID
├── user_id            # 租户用户 ID
├── role               # 用户角色：tenant_admin / teacher / student
├── created_at         # 创建时间
├── created_by         # 创建人用户 ID
├── updated_at         # 更新时间
├── updated_by         # 更新人用户 ID
├── version            # 数据版本号，用于乐观锁
└── ext_json           # JSON 扩展字段，保存非主流程元数据
```

首版不做复杂 RBAC。角色权限由 service 层通过 `PermissionChecker` 抽象统一判断，首版实现基于固定角色和空间成员关系。

`user_roles` 必须建立 `UNIQUE (tenant_id, user_id, role)`，防止同一用户重复分配同一角色。

账号禁用规则：

- 禁止用户禁用自己的账号。
- 平台必须至少保留一个启用状态的 `platform_admin`。
- 如果禁用平台管理员会导致平台没有启用状态的 `platform_admin`，系统必须拒绝操作。
- 禁用租户用户时，系统需要提醒操作者该用户会失去登录、考试、阅卷或空间管理能力。
- 禁用租户用户时，需要同步判断其空间成员身份；如果会导致某个空间失去最后一个启用状态的 `space_admin`，系统必须拒绝操作。
- 禁用租户用户、修改空间角色、禁用或移除空间成员，都必须复用统一空间管理员不变式校验。

平台配置规则：

- 平台配置通过 `platform_configs` 保存，配置项按 `config_key` 全平台唯一。
- `platform_configs` 必须建立 `UNIQUE (config_key)`。
- 配置表不使用软删除，删除配置项直接硬删除；平台配置和空间配置保持一致。
- 平台级开关、默认值、注册策略等可放入平台配置表。
- 生产密钥、数据库连接、JWT 密钥等敏感部署配置不能放入平台配置表，仍通过环境变量、Docker secret 或部署平台密钥能力注入。
- 配置值读取后必须按 `value_type` 转换，转换失败应快速失败并返回明确错误。

平台用户唯一约束：

- `UNIQUE (username, deleted_at)`
- `UNIQUE (phone, deleted_at)`
- `UNIQUE (email, deleted_at)`

用户头像和登录审计规则：

- 平台用户和租户用户都支持上传头像，头像地址保存到 `avatar_url`。
- 头像上传必须限制文件类型和大小，首版建议只允许常见图片格式。
- 登录成功后更新 `last_login_ip` 和 `last_login_at`。
- 登录失败不更新最后登录信息，但需要记录安全日志。
- 头像文件本身不存入数据库，只保存文件地址或对象存储 key。

通用文件上传规则：

- 文件上传统一走 `/api/v1/uploads`，业务表只保存上传接口返回的 URL 或对象 key。
- 服务端通过 `ObjectStore` 抽象写入对象存储；本地开发使用本地文件系统实现，后续 S3、OSS、MinIO 等远端协议只新增实现，不修改业务 handler。
- 上传接口负责生成对象 key，不信任客户端原始文件名作为存储路径。
- 服务端必须基于文件内容识别真实 MIME，不信任 multipart `Content-Type` 或扩展名；保存后缀由识别结果派生。
- 服务端保存文件名统一使用 `{yyyyMMddHHmmss}_{文件内容 MD5 前 16 位}{安全后缀}`，同一天上传的文件按日期目录归档。
- 本地存储返回 `/uploads/{category}/{date}/{object}` 形式地址，并由 HTTP server 挂载静态读取路由。
- 浏览器端上传图片前优先转为 WebP；转换失败、浏览器能力不足或非图片文件时上传原始文件。

租户用户唯一约束：

- `UNIQUE (tenant_id, username, deleted_at)`
- `UNIQUE (tenant_id, phone, deleted_at)`
- `UNIQUE (tenant_id, email, deleted_at)`

`real_name` 用于阅卷、成绩单和导出场景，首版可以非必填。手机号和邮箱如果允许为空，Repository 创建用户时需要按数据库差异处理空值唯一性，不能让同一租户内出现两个相同手机号或邮箱。

认证上下文需要区分两类主体：

- `platform_user`：只能访问 `/api/v1/platform` 等平台级接口。
- `tenant_user`：只能访问租户内业务接口，并且必须携带 `tenant_id`。

### 6.3 题库与题目

```text
questions
├── id                    # 题目主键 ID
├── tenant_id             # 所属租户 ID
├── space_id              # 所属空间 ID；为空表示租户公共题库
├── type                  # 题型：single / multiple / judge / fill_blank / short_text
├── difficulty            # 难度：easy / medium / hard
├── title                 # 题干内容
├── analysis              # 题目解析，出题人可选填
├── score_default         # 默认分值
├── choice_display_count  # 选择题展示选项数量
├── shuffle_options       # 题库默认选项随机设置
├── status                # 题目状态：draft / enabled / disabled
├── created_by            # 创建人用户 ID
├── created_at            # 创建时间
├── updated_at            # 更新时间
├── updated_by            # 更新人用户 ID
├── deleted_at            # 软删除时间
├── version               # 数据版本号，用于乐观锁
└── ext_json              # JSON 扩展字段，保存非主流程元数据

question_options
├── id                    # 题目选项主键 ID
├── tenant_id             # 所属租户 ID
├── question_id           # 题目 ID
├── option_key            # 出题编辑时的原始展示标签，例如 A / B / C / D，不参与判分
├── sort_order            # 选项原始排序
├── content               # 选项内容
├── is_correct            # 是否为正确答案
├── is_distractor         # 是否可作为随机补位干扰项
├── created_at            # 创建时间
├── created_by            # 创建人用户 ID
├── updated_at            # 更新时间
├── updated_by            # 更新人用户 ID
├── version               # 数据版本号，用于乐观锁
└── ext_json              # JSON 扩展字段，保存非主流程元数据

question_tags
├── id                    # 题目标签关系主键 ID
├── tenant_id             # 所属租户 ID
├── question_id           # 题目 ID
├── tag_id                # 标签 ID
├── created_at            # 创建时间
├── created_by            # 创建人用户 ID
└── ext_json              # JSON 扩展字段，保存非主流程元数据

tags
├── id                    # 标签主键 ID
├── tenant_id             # 所属租户 ID
├── name                  # 标签名称，例如知识点、章节、技能点
├── created_at            # 创建时间
├── created_by            # 创建人用户 ID
├── updated_at            # 更新时间
├── updated_by            # 更新人用户 ID
├── deleted_at            # 软删除时间
├── version               # 数据版本号，用于乐观锁
└── ext_json              # JSON 扩展字段，保存非主流程元数据
```

题目标签必须归一化存储。`tags.name` 在同一租户内唯一，`question_tags` 只保存 `tag_id`，避免标签重命名、列表查询和拼写不一致带来的维护问题。

`question_tags` 必须建立 `UNIQUE (tenant_id, question_id, tag_id)`，防止同一题目重复绑定同一标签。

标签删除策略：

- `tags` 使用软删除。
- `question_tags` 是纯关系表，不使用软删除。
- 查询题目标签时必须 JOIN `tags` 并过滤 `tags.deleted_at = 0`。
- 删除标签时可以保留 `question_tags` 关系记录用于审计，但业务查询不得展示已删除标签。

题型：

- 单选题
- 多选题
- 判断题
- 填空题
- 简答题

题目规则：

- 单选题和多选题至少有一个正确答案。
- 单选题最终展示时只能有一个正确答案。
- 单选题可以设置展示选项数量，不固定为 4 项。
- 多选题可以有多个正确答案。
- 判断题按特殊选择题处理，也可以单独建模。
- 填空题首版仅支持单空作答，按标准答案完全匹配自动判分；多空填空、多个等价答案、复杂同义词、正则匹配后续扩展。
- 简答题支持参考答案和解析，默认人工阅卷。
- 每个题目都支持可选解析，出题人可以不填写。
- 难度、标签、题型、分值是规则组卷的基础条件。
- `is_distractor` 表示该错误选项可进入随机补位池。随机选项数量不足时，只从 `is_distractor = true` 的错误选项里抽取；普通错误选项用于固定展示或人工维护，不参与随机补位。
- `question_options` 必须建立 `UNIQUE (tenant_id, question_id, option_key)`，防止同一题目出现重复选项 key。
- `question_options` 必须建立 `UNIQUE (tenant_id, question_id, sort_order)`，保证同一题目下选项原始排序稳定。
- 题目选项编辑采用全量替换策略：保存题目时以当前提交的选项列表为准，在同一事务内删除旧选项并插入新选项。
- 选项全量替换后，已经生成的考试快照不受影响，因为判分和展示使用 `exam_attempt_questions` 中的快照数据。
- `option_key` 只用于出题编辑和原始展示，不作为判分依据。
- 选择题判分必须基于选项 ID 或快照内选项 ID，不基于 A/B/C/D 字母。
- 选项随机后，A/B/C/D 由前端按最终展示顺序动态生成。
- `questions.space_id = NULL` 表示租户级公共题库，租户内所有空间可见。
- `questions.space_id` 非空表示空间题库，仅该空间成员中的教师、租户管理员和有权限的组卷流程可见。

在线手工出题和 CSV/Excel 导入都落到同一套题库模型。

### 6.4 试卷与组卷规则

```text
papers
├── id                    # 试卷主键 ID
├── tenant_id             # 所属租户 ID
├── space_id              # 所属空间 ID；为空表示租户公共试卷
├── name                  # 试卷名称
├── description           # 试卷说明
├── total_score           # 试卷总分，由系统按大题题目聚合计算
├── build_mode            # 组卷方式：manual / rule_fixed / rule_live
├── shuffle_questions     # 是否对每个考生随机题目顺序
├── show_analysis         # 成绩可见后是否向考生展示题目解析
├── status                # 试卷状态：draft / enabled / disabled
├── created_by            # 创建人用户 ID
├── created_at            # 创建时间
├── updated_at            # 更新时间
├── updated_by            # 更新人用户 ID
├── deleted_at            # 软删除时间
├── version               # 数据版本号，用于乐观锁
└── ext_json              # JSON 扩展字段，保存非主流程元数据

paper_sections
├── id                    # 试卷大题主键 ID
├── tenant_id             # 所属租户 ID
├── paper_id              # 试卷 ID
├── sort_order            # 大题排序
├── name                  # 大题名称，例如一、单选题
├── question_type         # 大题题型
├── instructions          # 大题作答说明
├── total_score           # 大题小计分，由系统聚合计算
├── question_count        # 大题题目数量，由系统聚合计算
├── created_at            # 创建时间
├── created_by            # 创建人用户 ID
├── updated_at            # 更新时间
├── updated_by            # 更新人用户 ID
├── deleted_at            # 软删除时间
├── version               # 数据版本号，用于乐观锁
└── ext_json              # JSON 扩展字段，保存非主流程元数据

paper_section_questions
├── id                    # 大题题目关系主键 ID
├── tenant_id             # 所属租户 ID
├── section_id            # 大题 ID
├── paper_id              # 试卷 ID，冗余保存用于减少查询 JOIN
├── question_id           # 题目 ID
├── sort_order            # 题目在大题中的排序
├── score                 # 该题在本试卷中的分值
├── shuffle_options       # 手动或固化组卷下该题是否随机选项；可为空
├── created_at            # 创建时间
├── created_by            # 创建人用户 ID
├── updated_at            # 更新时间
├── updated_by            # 更新人用户 ID
├── version               # 数据版本号，用于乐观锁
└── ext_json              # JSON 扩展字段，保存非主流程元数据

paper_section_rules
├── id                    # 大题抽题规则主键 ID
├── tenant_id             # 所属租户 ID
├── section_id            # 大题 ID
├── paper_id              # 试卷 ID，冗余保存用于减少查询 JOIN
├── sort_order            # 规则在大题内的排序
├── difficulty            # 抽题难度条件
├── tag_filter            # 标签过滤条件，JSON 数组字符串
├── question_count        # 该规则抽题数量
├── score_per_question    # 该规则下每题分值
├── shuffle_options       # 规则组卷下是否随机选项；可为空
├── created_at            # 创建时间
├── created_by            # 创建人用户 ID
├── updated_at            # 更新时间
├── updated_by            # 更新人用户 ID
├── version               # 数据版本号，用于乐观锁
└── ext_json              # JSON 扩展字段，保存非主流程元数据
```

组卷方式：

- `manual`：教师按大题直接从题库选择题目，题目写入 `paper_section_questions`。
- `rule_fixed`：教师按大题配置规则，触发生成后固化为 `paper_section_questions`，发布前可以审题和手动调整，适合正式考试。
- `rule_live`：教师按大题配置规则，考试开始时为每个考生实时抽题，适合练习和模拟考试。
- `papers.space_id` 可为空；为空表示租户级公共试卷，非空表示空间内试卷。
- `papers.shuffle_questions` 控制整张试卷的题目顺序是否对每个考生随机。
- `papers.show_analysis` 控制成绩可见后是否展示题目解析；未公布成绩前不展示解析。
- `paper_sections` 承载大题结构、题型边界、作答说明、小计分和题号连续编排锚点。
- `paper_section_rules.tag_filter` 使用 JSON 数组字符串保存标签 ID，例如 `[1001,1002]`，避免依赖不同数据库的 JSON 方言。
- `paper_section_rules.shuffle_options` 控制该规则抽中的选择题是否随机选项。
- 手动组卷和规则固化后，`paper_section_questions.shuffle_options` 控制该题在这张试卷中是否随机选项。
- `paper_section_questions.shuffle_options` 和 `paper_section_rules.shuffle_options` 使用可空布尔值：`TRUE` 表示显式开启，`FALSE` 表示显式关闭，`NULL` 表示未设置并回退到题库默认值。
- 选项随机优先级：固化题目使用 `paper_section_questions.shuffle_options`；实时规则组卷使用 `paper_section_rules.shuffle_options`；字段为 `NULL` 时才使用 `questions.shuffle_options` 作为题库默认值。
- `paper_section_questions` 必须建立 `UNIQUE (tenant_id, paper_id, question_id)`，防止同一题重复加入同一张固化试卷。
- `paper_sections` 必须建立 `UNIQUE (tenant_id, paper_id, sort_order)`，保证大题顺序稳定。
- `paper_section_rules` 必须建立 `UNIQUE (tenant_id, section_id, sort_order)`，保证同一大题内规则顺序稳定。
- `paper_section_questions` 必须建立 `UNIQUE (tenant_id, section_id, sort_order)`，保证同一大题内题目顺序稳定。
- `paper_section_questions.paper_id` 和 `paper_section_rules.paper_id` 是性能冗余字段，用于常见按试卷查询时减少 JOIN。
- `section_id` 所属的 `paper_sections.paper_id` 必须与冗余 `paper_id` 一致，写入时由 repository 统一填充，后续不允许单独修改。
- `paper_sections` 必须建立 `UNIQUE (tenant_id, id, paper_id)`，用于支撑冗余 `paper_id` 的复合外键。
- `paper_section_questions` 必须建立复合外键 `(tenant_id, section_id, paper_id)` 引用 `paper_sections(tenant_id, id, paper_id)`。
- `paper_section_rules` 必须建立复合外键 `(tenant_id, section_id, paper_id)` 引用 `paper_sections(tenant_id, id, paper_id)`。
- 复合外键用于在数据库层保证冗余 `paper_id` 不会与 `section_id` 所属试卷跑偏。
- `paper_sections` 不允许跨试卷移动；需要移动大题时，应在目标试卷创建新大题并迁移题目。
- 删除大题时，`paper_sections` 写入 `deleted_at`，并在同一事务内硬删除对应 `paper_section_questions` 和 `paper_section_rules`，避免遗留孤儿子记录。
- 查询试卷大题、题目和规则时，必须过滤 `paper_sections.deleted_at = 0`。
- `papers.total_score`、`paper_sections.total_score` 和 `paper_sections.question_count` 是落库聚合字段，只能由统一重算逻辑更新，API 和业务代码不得手工写入。
- `manual` / `rule_fixed` 模式下，重算来源是 `paper_section_questions.score` 和题目数量。
- `rule_live` 模式下，重算来源是 `paper_section_rules.question_count × score_per_question` 和规则题数。
- `paper_section_rules.difficulty` 允许为空，空值表示不限难度。
- 每次保存大题、增删题目、调整分值、生成固化试卷、修改规则后，必须在同一事务内重算大题小计和试卷总分。
- `manual` 和 `rule_fixed` 模式下，每个考生拿到同一套题目集合，题目顺序和选项顺序仍可随机。
- `rule_live` 模式下，支持每个考生抽到不同题目。
- 支持同一批题目在不同考生侧展示不同顺序。
- 同一考生的一次考试中，同一道题不允许重复出现。

题库数量不足以生成不重复试卷时，发布考试或组卷预检查必须失败。

`rule_fixed` 生成流程：

```text
教师配置 paper_sections 和 paper_section_rules
  → 点击生成试卷
  → 系统按大题规则抽题并写入 paper_section_questions
  → 教师审题、替换、调整顺序和分值
  → 发布考试
```

`rule_fixed` 生成后，后续考试流程与 `manual` 完全一致，不再执行抽题规则。

`rule_live` 抽题流程：

```text
考生开始考试
  → 按 paper_sections 顺序逐个大题抽题
  → 每个大题内从 exam_live_question_pools 读取已冻结候选题池
  → 跨规则合并题池后去重
  → 生成 exam_attempt_questions 快照
```

`rule_live` 下，同一道题命中多条规则时只能进入一次。去重后题目数量不足时，不做尽量补足，直接按题库数量不足处理并阻止开始考试。发布考试前也必须提供预检查，提前暴露题库数量不足问题。

`rule_live` 发布态题池冻结：

- `rule_live` 考试从 `draft` 发布为 `published` 时，必须按当前规则冻结候选题池，写入 `exam_live_question_pools`。
- 冻结后，考生开考实时抽题只能从 `exam_live_question_pools` 读取候选题目 ID，不再直接查询动态题库。
- 题库后续软删除、禁用、改标签、改难度，不影响已经发布考试的 live 候选题池。
- 如果发布时题池数量不足，发布必须失败。
- 如果发布后修改试卷规则或题库条件，必须先撤回考试到可编辑状态，重新预检查并重新冻结题池。

### 6.5 考试、答题与随机快照

```text
exams
├── id                    # 考试主键 ID
├── tenant_id             # 所属租户 ID
├── paper_id              # 关联试卷 ID
├── name                  # 考试名称
├── start_time            # 考试开始时间
├── end_time              # 考试结束时间
├── duration_minutes      # 单次作答时长，单位分钟
├── max_attempts          # 每名考生最多作答次数
├── result_strategy       # 多次作答成绩策略：latest / highest
├── publish_mode          # 成绩发布模式：immediate_score / manual_publish
├── score_publish_time    # 统一成绩公布时间
├── invite_code           # 考试邀请码
├── status                # 考试状态：draft / published / closed
├── created_by            # 创建人用户 ID
├── created_at            # 创建时间
├── updated_at            # 更新时间
├── updated_by            # 更新人用户 ID
├── deleted_at            # 软删除时间
├── version               # 数据版本号，用于乐观锁
└── ext_json              # JSON 扩展字段，保存非主流程元数据

`invite_code` 必须全局唯一。公开入口只按邀请码解析考试，数据库唯一约束和服务端生成碰撞检查都必须避免两个租户生成相同邀请码。

exam_targets
├── id                    # 考试发布范围主键 ID
├── tenant_id             # 所属租户 ID
├── exam_id               # 考试 ID
├── target_type           # 发布目标类型：space / user
├── target_id             # 发布目标 ID
├── created_at            # 创建时间
├── created_by            # 创建人用户 ID
└── ext_json              # JSON 扩展字段，保存非主流程元数据

`exam_targets` 必须建立 `UNIQUE (tenant_id, exam_id, target_type, target_id)`，防止同一考试重复添加相同空间或用户目标。

exam_live_question_pools
├── id                    # rule_live 发布态题池主键 ID
├── tenant_id             # 所属租户 ID
├── exam_id               # 考试 ID
├── section_id            # 大题 ID
├── rule_id               # 大题抽题规则 ID
├── question_id           # 候选题目 ID
├── created_at            # 创建时间
├── created_by            # 创建人用户 ID
└── ext_json              # JSON 扩展字段，保存非主流程元数据

`exam_live_question_pools` 必须建立 `UNIQUE (tenant_id, exam_id, section_id, rule_id, question_id)`，防止同一规则候选题重复冻结。

`exam_live_question_pools` 清理策略：

- `exam_live_question_pools` 不使用软删除。
- 软删除考试时，可以保留对应题池快照用于审计和排查。
- 默认业务查询必须通过 `exams.deleted_at = 0` 限定，不展示已删除考试的 live 题池。
- 后续可由归档/清理任务按保留周期硬删除已删除考试对应的 live 题池。

exam_attempts
├── id                    # 考生作答主键 ID
├── tenant_id             # 所属租户 ID
├── exam_id               # 考试 ID
├── user_id               # 考生用户 ID
├── attempt_no            # 第几次作答，从 1 开始
├── status                # 作答状态：in_progress / submitted / graded
├── started_at            # 开始作答时间
├── submitted_at          # 提交时间
├── exam_token_hash       # 考试过程 token 哈希
├── exam_token_expires_at # 考试过程 token 过期时间
├── objective_score       # 客观题得分
├── subjective_score      # 主观题得分
├── total_score           # 总分
├── created_at            # 创建时间
├── created_by            # 创建人用户 ID
├── updated_at            # 更新时间
├── updated_by            # 更新人用户 ID
├── version               # 数据版本号，用于乐观锁
└── ext_json              # JSON 扩展字段，保存非主流程元数据

exam_attempt_questions
├── id                    # 考生题目快照主键 ID
├── tenant_id             # 所属租户 ID
├── attempt_id            # 作答 ID
├── section_id            # 原始大题 ID，仅用于溯源
├── question_id           # 原始题目 ID
├── section_snapshot      # 大题快照 JSON，包含大题名称和作答说明
├── sort_order            # 该考生看到的全局题号，从 1 连续递增
├── score                 # 该题在本次作答中的分值
├── question_snapshot     # 题干快照 JSON
├── option_snapshot       # 选项快照 JSON
├── correct_answer_snapshot # 正确答案快照 JSON
├── created_at            # 创建时间
├── created_by            # 创建人用户 ID
├── updated_at            # 更新时间
├── updated_by            # 更新人用户 ID
├── version               # 数据版本号，用于乐观锁
└── ext_json              # JSON 扩展字段，保存非主流程元数据

exam_answers
├── id                    # 答案主键 ID
├── tenant_id             # 所属租户 ID
├── attempt_id            # 作答 ID
├── attempt_question_id   # 考生题目快照 ID
├── answer_content        # 考生答案内容
├── score                 # 该题得分
├── grading_status        # 阅卷状态：auto / pending / graded
├── graded_by             # 阅卷人用户 ID
├── graded_at             # 阅卷时间
├── grader_comment        # 阅卷评语
├── created_at            # 创建时间
├── created_by            # 创建人用户 ID
├── updated_at            # 更新时间
├── updated_by            # 更新人用户 ID
├── version               # 数据版本号，用于乐观锁
└── ext_json              # JSON 扩展字段，保存非主流程元数据

exam_events
├── id                    # 考试事件主键 ID
├── tenant_id             # 所属租户 ID
├── attempt_id            # 作答 ID
├── event_type            # 事件类型：blur / focus / auto_save / submit / auto_submit
├── event_time            # 事件发生时间
├── payload               # 事件负载 JSON
├── created_at            # 创建时间
├── created_by            # 创建人用户 ID
└── ext_json              # JSON 扩展字段，保存非主流程元数据
```

考生开始考试时生成个人试卷快照：

```text
校验考试资格
  → 校验考试时间
  → 如果没有 attempt，生成 exam_attempt
  → manual / rule_fixed 按固化大题题目生成 exam_attempt_questions
  → rule_live 按大题规则实时抽题后生成 exam_attempt_questions
  → 固化题干、选项、答案快照
  → 签发 exam_token
  → 返回个人试卷
```

如果考生重复进入考试，直接返回已有快照，不重新抽题。

同一套试卷可以被多次发布为不同考试；同一场考试也支持同一学生多次作答。`exams.max_attempts` 控制最多作答次数，默认值为 1。`exams.result_strategy` 控制多次作答的成绩采用方式，首版支持：

- `latest`：取最后一次提交成绩。
- `highest`：取最高分。

首版为了保持阅卷和成绩发布流程清晰，包含简答题的试卷不允许设置 `max_attempts > 1`。多次作答只支持纯客观题试卷。这样 `result_strategy` 只需要在自动判分结果之间计算，不进入多次人工阅卷流程。

`exam_attempts` 必须建立唯一约束：

```sql
UNIQUE (tenant_id, exam_id, user_id, attempt_no)
```

开始考试必须按幂等流程实现：

1. 开考接口必须先校验当前请求已通过邀请码解析写入考试入口 session，`user_id` 从租户用户 session 派生，不能信任开考请求体。
2. 开考前必须按 `exam_targets` 校验考生是否命中直接用户目标，或命中空间目标中的启用学生成员关系。
3. 如果当前用户在该考试下已有 `in_progress` attempt，直接返回该 attempt 和题目快照。
4. 如果没有进行中的 attempt，统计已创建 attempt 数量，不能超过 `max_attempts`。
5. 生成下一个 `attempt_no`，先尝试插入 `exam_attempts`。
6. 如果遇到唯一约束冲突，只重新查询并返回已有 `in_progress` attempt；不能在同一次开始考试请求中继续递增创建新 attempt。

不能只依赖应用层先查后写，否则并发重试可能生成两份语义相同的答卷。

高并发下建议在事务内执行 attempt 创建，并对同一 `(tenant_id, exam_id, user_id)` 的 attempt 创建路径做数据库约束兜底。SQLite 模式下依赖单连接写入降低并发冲突概率。

`exam_attempts.exam_token_hash` 必须建立索引：

```sql
INDEX (exam_token_hash, status)
```

该索引用于考试过程 middleware 高频校验 exam token 和 attempt 状态。

考试过程使用独立的 `exam_token` 调用自动保存、提交答卷和防作弊事件接口，避免普通登录 token 在长时间考试中途过期导致保存失败。

`exam_token` 首版采用有状态不透明 token：

- 服务端签发随机 token，只把 hash 写入 `exam_attempts.exam_token_hash`。
- 明文 token 只返回给考生端，不落库。
- `exam_token_expires_at` = 本次考试实际截止时间 + buffer。
- middleware 校验 exam token 时，通过 hash 查询 `exam_attempts`。
- middleware 同时校验 attempt 状态，已提交、已强制交卷、考试已终止时拒绝继续自动保存或提交。
- `exam_token` 只能访问当前 attempt 的自动保存、提交答卷和事件上报接口，不能访问其他业务接口。
- exam token 物理有效期和考试业务作答时间必须分开校验。
- 自动保存、提交答卷和事件上报必须校验业务作答截止时间：`min(started_at + duration_minutes, exam.end_time)`。
- 超过业务作答截止时间后，自动保存接口必须拒绝继续写入答案。
- 允许最多 5 秒网络传输宽限，只用于处理边界上报延迟，不延长考试实际作答时间。
- 手动提交和到时自动交卷可能并发触发，提交接口必须使用带 `status` 和 `version` 条件的状态更新保护。
- 将 attempt 从 `in_progress` 改为 `submitted` 时，更新条件必须包含 `status = 'in_progress'` 和当前 `version`。
- 更新行数为 0 时，说明该 attempt 已被另一个提交请求处理，当前请求按幂等成功返回。

随机规则：

- 只有 `rule_live` 模式下，同一考试中的不同考生才可以抽到不同题目。
- `manual` 和 `rule_fixed` 模式下，不同考生使用同一套固化题目集合。
- 同一考试中，不同考生可以看到不同题目顺序。
- 同一道选择题，不同考生可以看到不同选项顺序。
- 正确选项必须进入快照。
- 错误选项按展示数量随机补足。
- 选项顺序随机打乱。
- 选择题快照必须保存最终展示选项的 ID 顺序，考生提交答案也保存选项 ID 或快照内选项 ID。
- A/B/C/D 只是最终展示顺序生成的视觉标签，不进入判分逻辑。
- 单选题快照中只能有一个正确答案。
- 多选题快照中至少一个正确答案。
- 快照生成后不再受题库修改影响。
- `correct_answer_snapshot` 只保存判分用正确答案快照，考生答案只保存在 `exam_answers.answer_content`。
- `section_id` 只用于溯源，不用于展示或判分。考生端展示大题名称、作答说明和大题顺序时，只能读取 `section_snapshot`。
- `sort_order` 是整张试卷内的全局题号，从 1 开始跨大题连续递增；大题内展示序号由前端根据 `section_snapshot` 分组后计算。
- 填空题首版仅支持单空，按标准答案完全匹配自动判分，比较前应做首尾空白裁剪。大小写、多空、多个等价答案、正则匹配等能力后续扩展。
- 多选题保存和判分前，必须对选项 ID 升序排序，再序列化为 JSON 数组字符串，例如 `[101,104]`。
- 多选题标准答案快照也必须使用同样的升序排序协议，避免因前端勾选顺序不同导致误判。
- 多选题判分禁止直接比较 JSON 字符串。
- 判分时必须将考生答案和标准答案都反序列化为 `[]uint64`，排序后逐项比较。
- `[101,104]` 和 `[101, 104]` 这类 JSON 字符串格式差异不能影响判分结果。
- 单选题和判断题答案保存单个选项 ID；填空题和简答题保存文本内容。

自动保存语义：

- `exam_answers` 必须建立 `UNIQUE (tenant_id, attempt_id, attempt_question_id)`。
- 自动保存使用 upsert：存在则更新 `answer_content`、`updated_at`、`updated_by`、`version`，不存在则插入。
- 自动判分和人工阅卷只读取该唯一记录，避免同一道题多次保存产生多行答案。

考试时间规则：

- 发布考试时必须校验 `duration_minutes <= end_time - start_time`。
- 考生实际剩余时间 = `min(开始答题时间 + duration_minutes, end_time) - 当前时间`。
- 到达个人答题时长或考试统一结束时间，均应自动交卷。
- 前端倒计时只展示后端返回的实际截止时间，避免前后端各自计算产生差异。

### 6.6 阅卷与成绩

成绩发布模式由组卷人或考试创建人配置：

```text
立即出分
├── 适合纯客观题
└── 提交后直接跳转成绩公布页

教师阅卷后发布
├── 客观题自动判分
├── 简答题人工阅卷
├── 可设置统一公布时间
└── 到时间后学生可以查看成绩
```

建议首版规则：

- 包含简答题的试卷默认使用教师阅卷后发布。
- 如果试卷包含简答题，首版不允许选择立即出分。
- 如果试卷包含简答题，首版不允许设置 `max_attempts > 1`。
- 教师阅卷列表按 attempt 粒度展示；由于含简答题试卷只能作答一次，首版不需要处理同一学生多次简答题阅卷结果合并。
- 纯客观题允许多次作答，成绩按 `result_strategy` 在已提交 attempts 中计算。
- `score_publish_time` 是考试级统一公布时间。达到该时间后，学生可以查看按 `result_strategy` 计算出的当前最终成绩；未到公布时间前不展示成绩。
- 成绩发布前，考生看到等待老师统一公布的提示。
- 到达统一公布时间后，考生可以查看自己的成绩。

### 6.7 防作弊事件

首版轻量防作弊：

- 考试倒计时
- 到时自动交卷
- 切屏或页面失焦记录
- 提交前二次确认
- 后台查看异常事件

事件写入 `exam_events`：

```text
blur
focus
auto_save
submit
auto_submit
```

切屏和页面焦点事件可能高频触发，前端需要做节流或合并上报，首版建议 1 秒内同类事件只上报一次。后端事件写入失败不能阻塞自动保存和提交主流程，但必须记录错误日志。

`exam_events` 写入策略：

- `blur`、`focus` 等非关键防作弊事件通过 Go 内部带缓冲 channel 异步写入数据库。
- API 接收到非关键事件后，成功进入队列即可返回成功，不同步等待落库。
- 队列满时可以丢弃非关键事件，但必须记录 WARN 日志，日志中包含 `tenant_id`、`attempt_id`、`event_type`。
- `submit`、`auto_submit` 这类关键事件不能静默丢弃，必须跟随提交主流程可靠记录；记录失败时至少写错误日志。
- 异步事件消费者随 HTTP router 初始化启动，支持批量或逐条落库。
- SQLite 模式下，异步事件队列可以降低 `exam_events` 与 `exam_answers` 自动保存争抢唯一写锁的风险。

### 6.8 后续演进边界

试卷快照存储演进：

- 首版优先保证历史追溯正确性，`exam_attempt_questions` 保留题干、选项、正确答案等必要快照 JSON。
- 当后续出现大规模正式考试或存储压力时，可将 `manual` / `rule_fixed` 优化为“试卷版本快照 + attempt 只保存题序和选项 ID 顺序”。
- `rule_live` 因为每个考生题目集合可能不同，仍需要保留考生级题目快照。
- 该优化作为后续演进，不改变首版主路径。

平台运维代入租户视角：

- 平台管理员独立存储在 `platform_users`，首版不与租户用户混用。
- 后续如需要平台人员协助租户排查问题，可增加受控的 impersonation 机制。
- impersonation 必须记录完整审计日志，包括平台用户 ID、目标租户 ID、目标用户或空间、开始时间、结束时间和操作原因。
- 首版不实现 impersonation，只保留扩展方向。

## 7. API 设计

统一 API 前缀：

```text
/api/v1
```

统一响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": {}
}
```

API 分组：

```text
/api/v1/auth
├── 登录
├── 注册
├── 租户码注册
└── 刷新 token

/api/v1/platform
├── 租户创建
├── 租户启停
├── 租户码重置
└── 注册开关配置

/api/v1/spaces
├── 空间管理
├── 成员加入/移除
└── 未分配用户池

/api/v1/questions
├── 在线出题
├── 批量导入
├── 标签/难度管理
└── 题目查询

/api/v1/papers
├── 大题管理
├── 手动组卷
├── 规则配置
├── rule_fixed 生成固化试卷
├── 组卷规则预检查
└── 试卷预览

/api/v1/exams
├── 发布考试
├── 考试目标范围
├── 邀请码
├── 基于邀请码入口 session 开始考试并返回 exam_token
├── 使用 exam_token 自动保存
├── 使用 exam_token 提交答卷
└── 使用 exam_token 记录切屏事件

/api/v1/grading
├── 待阅卷列表
├── 简答题评分
└── 阅卷完成

/api/v1/results
├── 成绩查询
├── 成绩发布
└── 成绩导出

/api/v1/uploads
└── 通用文件上传
```

平台管理员和租户用户登录成功后都使用 `github.com/gin-contrib/sessions` 写入服务端 session。当前 provider 支持进程内 `memstore` 和组件自带 Redis store，可通过 `auth.session.provider` 切换。登录响应返回的 `access_token` 是同一次 `Set-Cookie` 中已签名的 session cookie 值，前端仍可用 `Authorization: Bearer <token>` 调用 API；HTTP 中间件会把 Bearer 值回填为 session cookie，再由 Gin session middleware 解析当前主体。

认证上下文规则：

- session 中只保存当前主体类型、用户 ID、租户 ID 和角色，不保存密码、密码哈希或业务表快照。
- HTTP Router 必须接收启动配置中的 `security.allow_register_default` 和 `security.password_min_length`，避免配置只被加载但不影响运行行为。
- 登录页同时提供平台管理员和租户用户模式；租户学生登录成功后进入考试入口，再通过邀请码进入考试端。
- 平台侧租户管理查询和写操作必须携带有效平台管理员 Bearer token 或 session cookie。
- 后台考试、空间、用户、题库、试卷、组卷规则、组卷预检查和上传接口必须携带平台管理员或本租户 `tenant_admin` 登录态；租户用户只能操作 session 所属 `tenant_id`，禁止信任请求体跨租户切换。
- 平台侧写操作统一从当前主体上下文获取平台管理员用户 ID，禁止再从请求体信任 `actor_id` 写审计字段。
- 邀请码解析必须从租户用户 session 派生 `user_id` 和 `tenant_id`，禁止信任请求体里的考生 ID。
- 未携带有效平台管理员 Bearer token 或 session cookie 时，平台侧接口返回未登录错误。
- `auth.session.secret` 留空时启动进程随机生成签名密钥；生产环境应通过环境变量注入稳定密钥。

租户管理操作的鉴权和审计规则：

- 创建租户时，`created_by` 和 `updated_by` 写入当前平台管理员用户 ID。
- 编辑租户资料、重置租户码、修改注册开关时，`updated_by` 写入当前平台管理员用户 ID。
- 列表、创建、编辑租户资料、重置租户码和修改注册开关都只允许平台管理员访问。
- 管理端创建租户用户时必须显式提交初始密码并写入哈希；后端不得使用固定默认密码或固定临时密码。

阅卷和成绩 API 必须从登录态解析调用人身份。教师或空间管理员传入的 `space_id` 只表示当前操作空间，后端必须用 `space_members` 校验当前用户确实是该空间启用成员；`ExamScope`、`AttemptScope` 等资源范围必须由后端根据作答记录、成绩行或考试目标解析真实空间归属，不能信任请求参数拼接授权范围。成绩发布配置属于考试级管理操作，只允许平台管理员或本租户 `tenant_admin` 修改。

```text
POST /api/v1/auth/tenant/register
     body: tenant_code, username, real_name, password, phone?, email?

GET  /api/v1/tenants
     query: keyword?
POST /api/v1/tenants
     body: name, logo_url?, description, allow_register?
POST /api/v1/tenants/:id/profile
     body: name, logo_url?, description
POST /api/v1/tenants/:id/reset-code
     body: {}
POST /api/v1/tenants/:id/register-setting
     body: allow_register

POST /api/v1/uploads
     form-data: file, category?
     response: key, url, file_name, content_type, size

GET  /api/v1/grading/pending
     query: tenant_id, exam_id, space_id?

POST /api/v1/exam-attempts/:attempt_id/questions/:attempt_question_id/grade
     body: tenant_id, exam_id, space_id?, answer_version, score, comment?

GET  /api/v1/results
     query: tenant_id, exam_id, space_id?

POST /api/v1/results/publish-config
     body: tenant_id, exam_id, publish_mode, score_publish_time?

POST /api/v1/results/export
     body: tenant_id, exam_id, space_id?
```

列表接口统一分页参数：

```text
page       从 1 开始
page_size  默认 20，最大 100
```

列表响应统一返回分页信息：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "items": [],
    "page": 1,
    "page_size": 20,
    "total": 0
  }
}
```

首版默认使用 `page` / `page_size` 分页。大规模事件日志或成绩流水如果后续出现性能瓶颈，再单独演进为游标分页。

多端复用原则：

- Web、小程序、移动 H5 复用同一套业务 API。
- 微信登录、手机号授权等小程序特有能力放在独立认证入口。
- 考试、答题、成绩接口不感知调用端类型。
- 导入、导出、打印等 Web 管理端能力独立分组，小程序可不调用。

本地端到端联调约定：

- `app.env` 为 `dev`、`development` 或 `local` 且 `database.driver = sqlite` 时，`bootstrap/devseed` 在迁移完成后写入幂等演示数据。
- 演示数据固定覆盖前端默认联调入口：租户 `10`、平台管理员 `1`、租户管理员 `1`、考生 `20`、教师 `21`、试卷 `100`、考试 `1`。
- 默认考试每次开发环境启动都会滚动到当前可作答时间窗口内，避免长期复用 SQLite 数据库后考试过期。
- 该种子数据只用于本地开发和演示，不进入 PostgreSQL、MySQL 或生产环境。

前端管理页不能内置核心业务 mock 数据。发布考试的试卷、发布范围必须来自试卷、空间、用户 API；创建空间的空间管理员必须来自用户 API 返回的真实用户 ID，不能在前端维护姓名到 ID 的静态映射。

## 8. 配置设计

配置文件统一使用 YAML：

```text
server/conf/app.yaml
server/conf/app.example.yaml
```

仓库只保留开发配置模板和示例配置。`app.example.yaml` 必须为每一项配置提供中文注释，便于部署和二次开发时对照。生产环境不提交 `conf_online` 目录，生产配置通过环境变量、Docker secret 或部署平台密钥管理能力注入，并在 `.gitignore` 中排除本地私有配置文件。

建议配置结构：

```yaml
app:
  name: papermind
  env: dev
  http_port: 8080
  public_url: http://localhost:8080

database:
  driver: sqlite
  dsn: file:/var/lib/papermind/sqlite/papermind.db?_foreign_keys=on&_journal_mode=WAL&_busy_timeout=5000
  max_open_conns: 1
  max_idle_conns: 1

auth:
  access_token_ttl: 7200
  refresh_token_ttl: 604800
  exam_token_buffer_minutes: 30
  session:
    provider: memory
    secret: ""
    ttl: 604800
    key_prefix: papermind:session
    cleanup_interval: 300
    redis:
      addr: 127.0.0.1:6379
      username: ""
      password: ""
      db: 0

storage:
  temp_dir: /var/lib/papermind/tmp
  import_dir: /var/lib/papermind/imports
  export_dir: /var/lib/papermind/exports

security:
  allow_register_default: false
  password_min_length: 8
  cors_origins:
    - http://localhost:5173
```

生产密钥不提交到仓库，通过环境变量覆盖。

存储路径在配置中使用绝对路径，Docker 和服务器部署通过环境变量覆盖。`bootstrap` 阶段必须校验目录存在且可写，避免运行中导入导出失败。

## 9. 部署设计

首版支持两种部署形态。

Docker Compose 部署：

```text
web：React 静态资源 / Nginx
server：Go + Gin API
db：PostgreSQL
```

SQLite 单机部署：

```text
server：Go + Gin API
web：前端静态资源
data/sqlite/papermind.db
```

首版以 Docker Compose + PostgreSQL、SQLite 单机模式作为主要验证目标。

## 10. 测试策略

首版测试优先覆盖核心风险。

service 单元测试：

- 租户码生成唯一性
- 自注册用户归属租户但不进入空间
- 空间成员权限
- 选择题乱序后按选项 ID 自动判分
- 填空题单空完全匹配判分
- 软删除后默认查询不可见
- 禁止禁用自己的账号
- 禁止禁用最后一个平台管理员
- 禁止禁用或移除空间最后一个空间管理员
- 禁止修改角色导致空间失去最后一个空间管理员
- 多选题答案排序归一化后判分
- 非关键考试事件异步入队和队列满降级
- 试卷大题顺序和题号连续编排
- rule_fixed 生成后固化题目并允许审题
- rule_live 跨规则去重和题库数量不足失败
- rule_live 发布时冻结候选题池，开考只从冻结题池抽题
- exam token 校验必须同时校验业务作答截止时间
- 多选题判分必须反序列化后比较选项 ID 集合
- 规则组卷数量校验
- 考生级随机抽题不重复
- 选择题选项快照随机化
- 自动判分
- 简答题待阅卷
- 成绩发布时间可见性

dao 集成测试：

- PostgreSQL 基础 CRUD
- SQLite 基础 CRUD
- SQLite 使用 `-tags json1` 执行涉及 JSON 查询的测试
- `tenant_id` 数据隔离
- GORM 乐观锁更新

API 测试：

- 登录注册
- 发布考试
- 开始考试幂等
- 自动保存
- 提交锁定
- 成绩查询权限

后台执行测试时设置合理超时，避免长时间卡死。

## 11. 可观测性

首版轻量实现：

- Gin 请求日志
- 统一错误码
- 统一请求 ID
- 关键业务日志

服务启动时使用 `slog` 文本日志输出到控制台，API 路由统一挂载请求日志中间件，记录 `request_id`、HTTP 方法、路径、状态码和耗时。租户管理等平台侧接口的服务层错误必须记录原始错误，再返回统一错误响应。

关键业务日志包括：

- 租户创建
- 考试发布
- 考生开始考试
- 自动保存失败
- 手动交卷
- 自动交卷
- 阅卷完成
- 成绩发布

日志字段尽量包含：

- `request_id`
- `tenant_id`
- `user_id`
- `exam_id`

Tracing 和 Prometheus 指标先预留，不作为首版强制实现。

## 12. 首版里程碑

### M1 项目骨架

- server 目录结构
- web 目录结构
- YAML 配置加载
- GORM 数据库连接
- migration 基础能力
- Docker Compose 启动
- SQLite 单机启动

### M2 租户、用户、空间

- 平台管理员
- 租户创建与租户码
- 注册开关
- 用户注册和登录
- 用户导入
- 空间成员管理

### M3 题库与组卷

- 在线出题
- 题目标签和难度
- 题目导入
- 大题结构管理
- 手动组卷
- rule_fixed 规则生成固化试卷
- rule_live 实时规则组卷
- 组卷预检查
- React 管理端题库和组卷基础页面

### M4 考试发布与答题

- 发布到空间和指定用户
- 考试邀请码
- 开始考试
- 考生级随机题目快照
- 选择题选项快照随机化
- 自动保存
- 提交锁定
- React 考试端基础链路验证：进入考试、答题、提交

### M5 阅卷与成绩

- 客观题自动判分
- 简答题人工阅卷
- 成绩发布模式
- 定时公布成绩
- 成绩导出
- React 阅卷和成绩查询基础页面

### M6 管理端体验与质量收口

- React 管理端体验收口
- React 考试端体验收口
- 关键 API 测试
- Docker Compose 验证
- SQLite 单机验证
- PostgreSQL 或 MySQL 下 100 人同时在线考试链路验证

## 13. 关键决策记录

- 首版做在线 SaaS 考试平台，不做局域网机房版。
- 首版目标支持单场约 100 名考生同时在线作答，正式多人考试推荐 PostgreSQL 或 MySQL。
- 首版采用 Go + Gin + React + GORM。
- 首版采用模块化单体，不拆微服务。
- 多租户采用共享数据库加 `tenant_id` 隔离。
- 组织模型采用租户、空间、用户。
- 角色采用平台管理员、租户管理员、教师、考生。
- 首版使用固定角色权限判断，但通过 `PermissionChecker` 抽象层隔离权限实现，方便后续扩展 RBAC。
- 平台管理员使用独立 `platform_users` 表，不进入租户用户表。
- 平台配置使用 `platform_configs` 表保存，`config_key` 全平台唯一。
- 租户用户表增加 `real_name`，并声明同租户内用户名、手机号、邮箱唯一。
- 平台用户和租户用户都支持头像上传，并记录最后登录 IP 和最后登录时间。
- 空间成员支持禁用，空间内角色包含 `space_admin`、`teacher`、`student`。
- 禁止禁用自己的账号，且不能禁用最后一个启用状态的平台管理员。
- 启用状态的空间至少必须保留一个启用状态的空间管理员。
- 影响空间管理员数量的禁用、移除和角色变更必须统一调用空间管理员不变式校验。
- 空间配置使用 `space_configs` 表保存，`config_key` 在同一空间内唯一。
- 平台配置和空间配置表不使用软删除，删除配置项直接硬删除。
- 支持用户自注册，但注册后默认不属于任何空间。
- 租户码由平台管理员创建租户时系统生成。
- `user_roles` 使用 `UNIQUE (tenant_id, user_id, role)` 防止重复角色。
- `space_members` 使用 `UNIQUE (tenant_id, space_id, user_id, deleted_at)` 防止重复有效成员。
- 支持 `manual`、`rule_fixed`、`rule_live` 三种组卷模式，并统一使用 `paper_sections` 大题结构。
- 题型支持单选、多选、判断、填空和简答。
- 每个题目支持可选解析，试卷通过 `show_analysis` 控制成绩可见后是否展示解析。
- 题目标签使用 `tags` 表归一化存储，题目与标签通过 `question_tags.tag_id` 关联。
- `question_tags` 使用 `UNIQUE (tenant_id, question_id, tag_id)` 防止重复标签关系。
- 查询题目标签必须 JOIN `tags` 并过滤 `tags.deleted_at = 0`。
- `question_options.is_distractor` 用于标记错误选项是否进入随机补位池。
- `question_options` 使用 `UNIQUE (tenant_id, question_id, option_key)` 防止同题选项 key 重复。
- `question_options` 使用 `UNIQUE (tenant_id, question_id, sort_order)` 防止同题选项排序重复。
- 题目选项编辑采用全量替换策略，已生成考试快照不受后续选项编辑影响。
- 公共题库使用 `questions.space_id = NULL`，空间题库只对对应空间可见。
- `rule_fixed` 先按规则生成固化题目，教师审题调整后再发布，适合正式考试。
- `rule_live` 考试开始时按大题规则为每个考生实时抽题，适合练习和模拟考试。
- `rule_live` 发布时冻结候选题池到 `exam_live_question_pools`，开考时只从冻结题池抽题。
- `paper_section_rules.difficulty` 为空表示不限难度。
- 支持考生级题目顺序随机和选择题选项随机；只有 `rule_live` 支持不同考生拿到不同题目集合。
- 题目乱序是试卷级配置，选项乱序是大题规则或固化题目级配置。
- 手动组卷和 `rule_fixed` 通过 `paper_section_questions.shuffle_options` 控制逐题选项随机。
- 选项随机优先级为组卷配置优先，题库默认配置兜底。
- `shuffle_options` 使用可空布尔值，`NULL` 表示回退到题库默认配置。
- `paper_section_questions` 使用 `UNIQUE (tenant_id, paper_id, question_id)` 防止同题重复入卷。
- `papers.total_score`、`paper_sections.total_score`、`paper_sections.question_count` 是落库聚合字段，必须由统一重算逻辑在同一事务内维护。
- `paper_section_questions.paper_id` 和 `paper_section_rules.paper_id` 是查询优化冗余字段，必须与 `section_id` 所属试卷保持一致。
- `paper_section_questions` 和 `paper_section_rules` 必须通过复合外键约束保证冗余 `paper_id` 一致性。
- 删除大题时软删除 `paper_sections`，并硬删除对应 `paper_section_questions` 和 `paper_section_rules`。
- `rule_live` 跨规则合并题池后去重，去重后数量不足则失败。
- 同一考生的一次考试中，同一道题不允许重复出现。
- 同一场考试支持同一学生多次作答，由 `max_attempts` 控制次数，由 `result_strategy` 控制成绩采用方式。
- 含简答题试卷首版不允许多次作答，`max_attempts` 必须为 1。
- 支持服务端自动保存答案。
- 考试过程使用独立 `exam_token`，避免普通登录 token 过期影响自动保存和提交。
- `exam_token` 使用有状态不透明 token，库里只保存 hash 和过期时间。
- `exam_attempts` 使用 `UNIQUE (tenant_id, exam_id, user_id, attempt_no)` 约束多次作答序号。
- `exam_token_hash` 建立 `(exam_token_hash, status)` 索引。
- `exam_targets` 使用 `UNIQUE (tenant_id, exam_id, target_type, target_id)` 防止发布范围重复。
- `exam_answers` 使用 `UNIQUE (tenant_id, attempt_id, attempt_question_id)` 并通过 upsert 自动保存。
- 多选题答案保存和判分前必须按选项 ID 升序排序。
- 多选题判分必须反序列化 JSON 后比较选项 ID 集合，禁止裸字符串比较。
- exam token 校验必须同时校验业务作答截止时间，超过后拒绝自动保存新答案。
- 非关键 `exam_events` 通过带缓冲 channel 异步写入，队列满时可丢弃并记录 WARN。
- attempt 提交状态流转使用 `status + version` 条件更新，支持手动提交和自动交卷并发幂等。
- 考试剩余时间按 `min(开始答题时间 + duration_minutes, end_time)` 计算。
- 支持立即出分和教师阅卷后发布两种成绩模式。
- 包含简答题的试卷首版不允许立即出分。
- 配置文件统一使用 YAML。
- 不提供 `conf_online` 目录，生产配置通过环境变量、Docker secret 或部署平台密钥管理注入。
- 数据库操作采用 GORM。
- 数据库驱动使用 `gorm.io/driver/sqlite`、`gorm.io/driver/mysql`、`gorm.io/driver/postgres`。
- JSON 字段使用 `gorm.io/datatypes`，Go 实体中 `ext_json` 使用 `datatypes.JSON`。
- GORM 实体和字段映射放在同一个文件里。
- 所有表保留 `ext_json` 字段，用于保存不影响主流程的 JSON 扩展元数据。
- SQLite 使用 JSON 查询能力时，构建和测试命令必须带 `json1` 标签。
- SQLite 必须启用 `_foreign_keys=on`，确保复合外键约束生效。
- 除非有必要或 GORM 实现不了，否则不写原始 SQL。
- 数据库查询简单优先，除必要场景外不做联表查询。
- SQLite 使用单连接和 WAL 模式。
- SQLite 仅推荐用于演示、本地开发和低并发小规模考试，不作为 100 人正式考试推荐部署。
- 列表 API 统一使用 `page` / `page_size` 分页。
- `exam_events` 是追加写日志表，豁免 `updated_at`、`updated_by` 和 `version`。
- 选择题判分基于选项 ID 或快照内选项 ID，不基于 A/B/C/D 字母。
- 填空题首版仅支持单空作答。
- 核心主表使用软删除，历史答卷、成绩和考试快照不依赖原始数据是否已删除。
- 大规模考试下的轻量快照存储作为后续演进，首版优先保证历史追溯正确性。
- 平台管理员保持独立表，后续如需协助租户排查，通过受控 impersonation 机制扩展。

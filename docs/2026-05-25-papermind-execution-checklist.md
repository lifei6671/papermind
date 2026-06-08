# PaperMind 首版执行任务清单

> 本清单由 `docs/2026-05-25-papermind-exam-platform-technical-design.md` 拆解而来。
> 目标是把技术方案拆成可执行、可验收、可回写进度的任务。

## 0. 执行原则

- [ ] 每个阶段完成后先做代码审查，再进入下一阶段。
- [ ] 所有数据库迁移 SQL 必须包含中文表注释、字段注释、关键索引/约束注释。
- [ ] 后续新增和修改的代码必须提供完善中文注释，关键业务规则、状态流转、异常分支和跨层约束必须写在对应实现附近。
- [ ] 所有业务表保留 `created_at`、`created_by`、`updated_at`、`updated_by`、`version`、`ext_json`，豁免表按方案明确处理。
- [ ] 所有软删除核心表使用 `gorm.io/plugin/soft_delete` Unix 时间戳模式。
- [ ] 所有 GORM 实体必须显式声明 `gorm:"column:xxx"` 字段映射。
- [ ] 字段映射与数据库实体放在同一个文件。
- [ ] `service` 层禁止直接使用 GORM 和 SQL。
- [ ] 数据库访问统一封装在 `server/internal/dao/db`。
- [ ] 权限判断统一通过 `PermissionChecker`，业务 service 不散落角色字符串判断。
- [ ] SQLite 只作为演示、本地开发和低并发单机模式，不作为 100 人正式考试推荐部署。
- [ ] 正式 100 人在线考试优先验证 PostgreSQL，MySQL 8.0+ 作为兼容目标。

## 1. 推荐阶段顺序

```text
P0 文档与项目骨架
  → P1 配置、日志、数据库基础设施
  → P2 数据库迁移与 DAO 基础模型
  → P3 权限、账号、租户、空间
  → P4 题库、标签、题目选项、导入
  → P5 试卷大题、手动组卷、规则组卷
  → P6 考试发布、题池冻结、开始考试
  → P7 答题、自动保存、交卷、防作弊事件
  → P8 判分、阅卷、成绩发布与导出
  → P9 React 管理端与考试端
  → P9.7 前后端 API 接入
  → P10 部署、压测、质量收口
```

---

## P0. 文档与项目骨架

**目标**：建立前后端工程骨架和约定目录，不实现业务。

**依赖**：无。

### P0.1 后端目录

- [x] 创建 `server/api/router`。
- [x] 创建 `server/api/middleware`。
- [x] 创建 `server/api/request`。
- [x] 创建 `server/api/response`。
- [x] 创建 `server/api/v1`。
- [x] 创建 `server/bootstrap`。
- [x] 创建 `server/cmd/papermind`。
- [x] 创建 `server/conf`。
- [x] 创建 `server/data/migrations/postgres`。
- [x] 创建 `server/data/migrations/mysql`。
- [x] 创建 `server/data/migrations/sqlite`。
- [x] 创建 `server/data/seeds`。
- [x] 创建 `server/data/sqlite`。
- [x] 创建 `server/data/tmp`。
- [x] 创建 `server/data/imports`。
- [x] 创建 `server/data/exports`。
- [x] 创建 `server/internal/service`。
- [x] 创建 `server/internal/service/permission`。
- [x] 创建 `server/internal/dao/db`。
- [x] 创建 `server/internal/dao/external`。
- [x] 创建 `server/internal/model`。
- [x] 创建 `server/internal/dto`。
- [x] 创建 `server/internal/job`。
- [x] 创建 `server/library/code`。
- [x] 创建 `server/library/constant`。
- [x] 创建 `server/library/logger`。
- [x] 创建 `server/library/config`。
- [x] 创建 `server/library/validator`。
- [x] 创建 `server/library/response`。
- [x] 创建 `server/library/crypto`。
- [x] 创建 `server/library/xerr`。
- [x] 创建 `server/mock`。
- [x] 创建 `server/script`。
- [x] 创建 `server/tests`。

### P0.2 前端与部署目录

- [x] 创建 `web`。
- [x] 创建 `deployments/docker-compose`。
- [x] 创建 `deployments/sqlite-single-node`。
- [x] 创建 `docs/api`。
- [x] 创建 `docs/database`。
- [x] 创建 `docs/specs`。

### P0.3 项目基础文件

- [x] 初始化 `server/go.mod`。
- [x] 引入 `github.com/gin-gonic/gin`。
- [x] 引入 `gorm.io/gorm`。
- [x] 引入 `gorm.io/driver/postgres`。
- [x] 引入 `gorm.io/driver/mysql`。
- [x] 引入 `gorm.io/driver/sqlite`。
- [x] 引入 `gorm.io/datatypes`。
- [x] 引入 `gorm.io/plugin/soft_delete`。
- [x] 创建 `server/conf/app.example.yaml` 开发配置模板；本地私有 `server/conf/app.yaml` 不进入仓库。
- [x] 创建 `server/conf/app.example.yaml` 示例配置，并为每一项提供中文注释。
- [x] 确认 `.gitignore` 排除 `server/conf_online/`、私有配置、SQLite 数据库文件、导入导出临时文件、构建产物。
- [x] 初始化 `web` 为 React + Vite + TypeScript 项目。

**验收标准**：

- [x] `go test ./...` 可以在空业务骨架下运行。
- [x] `go test -tags json1 ./...` 可以运行。
- [x] `web` 可以启动 Vite 开发服务。
- [x] 仓库不存在 `server/conf_online` 生产配置目录。

---

## P1. 配置、日志、数据库基础设施

**目标**：完成服务启动、配置加载、日志、数据库连接和基础响应。

**依赖**：P0。

### P1.1 YAML 配置

- [x] 定义 `server/library/config` 配置结构。
- [x] 支持 `app.name`。
- [x] 支持 `app.env`。
- [x] 支持 `app.http_port`。
- [x] 支持 `app.public_url`。
- [x] 支持 `database.driver`。
- [x] 支持 `database.dsn`。
- [x] 支持 `database.max_open_conns`。
- [x] 支持 `database.max_idle_conns`。
- [x] 支持 `auth.access_token_ttl`。
- [x] 支持 `auth.refresh_token_ttl`。
- [x] 支持 `auth.exam_token_buffer_minutes`。
- [x] 支持 `auth.session.provider`、`auth.session.secret`、`auth.session.ttl`。
- [x] 预留 `auth.session.key_prefix`、`auth.session.cleanup_interval` 和 Redis 配置。
- [x] 支持 `storage.temp_dir`。
- [x] 支持 `storage.import_dir`。
- [x] 支持 `storage.export_dir`。
- [x] 支持 `security.allow_register_default`。
- [x] 支持 `security.password_min_length`。
- [x] HTTP Router 启动时透传 `security.allow_register_default` 和 `security.password_min_length`。
- [x] 支持 `security.cors_origins`。
- [x] 支持环境变量覆盖敏感配置。
- [x] 启动时校验存储目录存在且可写。

### P1.2 日志与响应

- [x] 实现统一 `request_id` 中间件。
- [x] 实现 Gin 请求日志中间件。
- [x] HTTP server 挂载请求日志中间件并输出到控制台。
- [x] 实现 panic recover 中间件。
- [x] 实现统一响应结构 `{ code, message, data }`。
- [x] 实现分页响应结构 `{ items, page, page_size, total }`。
- [x] 定义错误码目录 `server/library/code`。
- [x] 定义业务错误封装 `server/library/xerr`。

### P1.3 数据库连接

- [x] 根据 `database.driver` 初始化 PostgreSQL。
- [x] 根据 `database.driver` 初始化 MySQL。
- [x] 根据 `database.driver` 初始化 SQLite。
- [x] SQLite DSN 必须包含 `_foreign_keys=on`。
- [x] SQLite DSN 必须包含 `_journal_mode=WAL`。
- [x] SQLite DSN 必须包含 `_busy_timeout=5000`。
- [x] SQLite 默认 `max_open_conns = 1`。
- [x] 支持配置 SQLite 小规模多人模式 `max_open_conns = 5-10`。
- [x] PostgreSQL / MySQL 按配置设置连接池。
- [x] 数据库连接在服务启动时初始化一次，DAO 层复用单例 `*gorm.DB`。
- [x] 数据库初始化必须输出 driver、连接池参数和迁移目录日志。

### P1.4 GORM 基础约束

- [x] 实体统一使用显式 `gorm:"column:xxx"`。
- [x] `ext_json` 统一使用 `datatypes.JSON`。
- [x] 软删除字段使用 `soft_delete.DeletedAt` Unix 时间戳模式。
- [x] 字段映射常量与实体同文件维护。
- [x] 禁止 service 层直接 import GORM。
- [x] 基础实体和约束测试包含清晰中文注释，后续业务实现沿用该注释规范。

**验收标准**：

- [x] 三种数据库 driver 初始化逻辑都有单元测试。
- [x] SQLite 测试命令使用 `go test -tags json1 ./server/...`。
- [x] 配置目录缺失或不可写时启动失败且错误明确。
- [x] API 返回结构统一。

---

## P2. 数据库迁移与 DAO 基础模型

**目标**：落地首版完整 schema、中文注释、约束、索引和 GORM DO。

**依赖**：P1。

### P2.1 迁移框架

- [x] 服务启动时根据 `database.driver` 自动选择 PostgreSQL 迁移目录。
- [x] 服务启动时根据 `database.driver` 自动选择 MySQL 迁移目录。
- [x] 服务启动时根据 `database.driver` 自动选择 SQLite 迁移目录。
- [x] 迁移期间 HTTP 层返回系统升级中间页或稳定 JSON 响应。
- [x] 迁移 SQL 文件按版本号排序执行。
- [x] 迁移 SQL 必须可重复检测已执行版本。
- [x] 迁移失败必须停止启动。

### P2.2 基础字段规范

- [x] 所有业务表包含 `created_at`。
- [x] 所有业务表包含 `created_by`。
- [x] 所有业务表包含 `created_by_type`，用于区分 `created_by` 来自平台用户、租户用户或系统任务。
- [x] 所有非豁免表包含 `updated_at`。
- [x] 所有非豁免表包含 `updated_by`。
- [x] 所有非豁免表包含 `updated_by_type`，用于区分 `updated_by` 来自平台用户、租户用户或系统任务。
- [x] 所有非豁免表包含 `version`。
- [x] 所有表包含 `ext_json`。
- [x] 核心主表包含 `deleted_at`。
- [x] 纯关系表按方案豁免 `updated_at`、`updated_by`、`updated_by_type`、`version`。
- [x] `exam_events` 按方案豁免 `updated_at`、`updated_by`、`updated_by_type`、`version`。

### P2.3 租户与空间表

- [x] 创建 `tenants`。
- [x] `tenants` 包含 `logo_url`。
- [x] `tenants` 包含 `description`。
- [x] `tenants.tenant_code` 全平台唯一。
- [x] 创建 `spaces`。
- [x] `spaces` 包含 `logo_url`。
- [x] `spaces` 包含 `description`。
- [x] 创建 `space_members`。
- [x] `space_members` 包含 `role_in_space`。
- [x] `space_members` 包含 `status`。
- [x] `space_members` 建立 `UNIQUE (tenant_id, space_id, user_id, deleted_at)`。
- [x] 创建 `space_configs`。
- [x] `space_configs` 建立 `UNIQUE (tenant_id, space_id, config_key)`。
- [x] 配置表不使用软删除。

### P2.4 用户与角色表

- [x] 创建 `platform_users`。
- [x] `platform_users` 包含 `avatar_url`。
- [x] `platform_users` 包含 `last_login_ip`。
- [x] `platform_users` 包含 `last_login_at`。
- [x] `platform_users` 建立 `UNIQUE (username, deleted_at)`。
- [x] `platform_users` 建立 `UNIQUE (phone, deleted_at)`。
- [x] `platform_users` 建立 `UNIQUE (email, deleted_at)`。
- [x] 创建 `platform_configs`。
- [x] `platform_configs` 建立 `UNIQUE (config_key)`。
- [x] 创建 `users`。
- [x] `users` 包含 `real_name`。
- [x] `users` 包含 `avatar_url`。
- [x] `users` 包含 `last_login_ip`。
- [x] `users` 包含 `last_login_at`。
- [x] `users` 建立 `UNIQUE (tenant_id, username, deleted_at)`。
- [x] `users` 建立 `UNIQUE (tenant_id, phone, deleted_at)`。
- [x] `users` 建立 `UNIQUE (tenant_id, email, deleted_at)`。
- [x] 创建 `user_roles`。
- [x] `user_roles` 建立 `UNIQUE (tenant_id, user_id)`，首版保证同一租户用户只有一个租户级角色。

### P2.5 题库表

- [x] 创建 `questions`。
- [x] `questions.type` 支持 `single`。
- [x] `questions.type` 支持 `multiple`。
- [x] `questions.type` 支持 `judge`。
- [x] `questions.type` 支持 `fill_blank`。
- [x] `questions.type` 支持 `short_text`。
- [x] `questions.analysis` 可为空。
- [x] `questions.quality_score` 保存 0-10 题目质量分，默认 5。
- [x] 创建 `question_options`。
- [x] `question_options` 包含 `option_key`，仅用于编辑展示。
- [x] `question_options` 包含 `sort_order`。
- [x] `question_options` 建立 `UNIQUE (tenant_id, question_id, option_key)`。
- [x] `question_options` 建立 `UNIQUE (tenant_id, question_id, sort_order)`。
- [x] 创建 `tags`。
- [x] `tags.name` 在同租户唯一，唯一约束包含 `deleted_at`。
- [x] 创建 `question_tags`。
- [x] `question_tags` 建立 `UNIQUE (tenant_id, question_id, tag_id)`。

### P2.6 试卷与组卷表

- [x] 创建 `papers`。
- [x] `papers.build_mode` 支持 `manual`。
- [x] `papers.build_mode` 支持 `rule_fixed`。
- [x] `papers.build_mode` 支持 `rule_live`。
- [x] `papers.show_analysis` 控制解析展示。
- [x] 创建 `paper_sections`。
- [x] `paper_sections` 包含 `deleted_at`。
- [x] `paper_sections` 建立 `UNIQUE (tenant_id, paper_id, sort_order)`。
- [x] `paper_sections` 建立 `UNIQUE (tenant_id, id, paper_id)`。
- [x] 创建 `paper_section_questions`。
- [x] `paper_section_questions` 建立 `UNIQUE (tenant_id, paper_id, question_id)`。
- [x] `paper_section_questions` 建立 `UNIQUE (tenant_id, section_id, sort_order)`。
- [x] `paper_section_questions` 建立复合外键 `(tenant_id, section_id, paper_id)` 引用 `paper_sections(tenant_id, id, paper_id)`。
- [x] 创建 `paper_section_rules`。
- [x] `paper_section_rules.difficulty` 允许为空，表示不限难度。
- [x] `paper_section_rules` 建立 `UNIQUE (tenant_id, section_id, sort_order)`。
- [x] `paper_section_rules` 建立复合外键 `(tenant_id, section_id, paper_id)` 引用 `paper_sections(tenant_id, id, paper_id)`。

### P2.7 考试与答题表

- [x] 创建 `exams`。
- [x] `exams.max_attempts` 默认 1。
- [x] `exams.result_strategy` 支持 `latest`。
- [x] `exams.result_strategy` 支持 `highest`。
- [x] `exams.invite_code` 建立全局唯一约束。
- [x] 创建 `exam_targets`。
- [x] `exam_targets` 建立 `UNIQUE (tenant_id, exam_id, target_type, target_id)`。
- [x] 创建 `exam_live_question_pools`。
- [x] `exam_live_question_pools` 建立 `UNIQUE (tenant_id, exam_id, section_id, rule_id, question_id)`。
- [x] 创建 `exam_attempts`。
- [x] `exam_attempts` 建立 `UNIQUE (tenant_id, exam_id, user_id, attempt_no)`。
- [x] `exam_attempts` 建立 `INDEX (exam_token_hash, status)`。
- [x] 创建 `exam_attempt_questions`。
- [x] `exam_attempt_questions` 包含 `section_snapshot`。
- [x] `exam_attempt_questions.sort_order` 为全局题号。
- [x] 创建 `exam_answers`。
- [x] `exam_answers` 建立 `UNIQUE (tenant_id, attempt_id, attempt_question_id)`。
- [x] 创建 `exam_events`。

**验收标准**：

- [x] PostgreSQL 迁移 SQL 包含表和字段中文注释。
- [x] MySQL 迁移 SQL 包含表和字段中文注释。
- [x] SQLite 迁移 SQL 使用 `--` 注释说明表和字段含义。
- [ ] 三套迁移均能从空库执行成功。
- [ ] SQLite 外键约束实际生效。
- [x] 所有 DO 文件有实体和字段映射。

---

## P3. 权限、账号、租户、空间

**目标**：完成平台管理员、租户、用户、空间和权限抽象。

**依赖**：P2。

### P3.1 权限抽象

- [x] 创建 `server/internal/service/permission/checker.go`。
- [x] 定义 `PermissionChecker` 接口。
- [x] 创建 `server/internal/service/permission/context.go`。
- [x] 定义 `PermissionContext`。
- [x] 创建 `server/internal/service/permission/fixed_role.go`。
- [x] 实现 `CanManageTenantLifecycle`、`CanManageTenantBusiness`、`CanManageTenantUsers`，区分平台租户生命周期和租户内业务管理。
- [x] 实现 `CanManageSpaceProfile`、`CanManageSpaceMembers`，区分空间资料和空间成员管理。
- [x] 实现 `CanManageQuestion`。
- [x] 实现 `CanPublishExam`。
- [x] 实现 `CanGradeAttempt`。
- [x] 实现 `CanTakeExam`。
- [x] 实现 `CanViewExamResults`、`CanExportExamResults` 和 `CanViewOwnResult`，成绩查看、导出和学生查分使用独立语义。
- [x] service 层统一调用 `PermissionChecker`。

### P3.2 平台用户

- [x] 实现平台管理员初始化 seed，仅在 `dev`、`development`、`local` 或 `test` 环境空库默认创建 `admin / admin123`，默认邮箱 `admin@iminho.me`。
- [x] 实现平台管理员登录。
- [x] 登录成功更新 `last_login_ip`。
- [x] 登录成功更新 `last_login_at`。
- [x] 登录失败记录安全日志。
- [x] 实现平台用户头像上传。
- [x] 头像上传限制文件类型。
- [x] 头像上传限制文件大小。
- [x] 禁止平台管理员禁用自己。
- [x] 禁止禁用最后一个启用状态平台管理员。

### P3.3 租户

- [x] 实现创建租户。
- [x] 创建租户时生成唯一 `tenant_code`。
- [x] 新租户 `allow_register` 继承 `security.allow_register_default`。
- [x] 创建租户时支持显式设置 `allow_register`。
- [x] 创建租户时初始化首个启用状态 `tenant_admin`，且不自动创建默认空间或空间成员。
- [x] 实现查看租户码。
- [x] 实现重置租户码。
- [x] 实现启用租户。
- [x] 实现禁用租户。
- [x] 实现租户 logo 保存。
- [x] 实现租户名称、描述和 Logo 保存。

### P3.4 租户用户

- [x] 实现租户用户注册。
- [x] 支持租户注册链接注册。
- [x] 支持手动输入租户码注册。
- [x] 自注册用户默认不属于任何空间。
- [x] 自注册密码遵守 `security.password_min_length`。
- [x] 实现租户用户登录。
- [x] 登录成功更新 `last_login_ip`。
- [x] 登录成功更新 `last_login_at`。
- [x] 实现租户用户头像上传。
- [x] 实现用户禁用。
- [x] 实现用户启用。
- [x] 禁用用户前提示影响范围。
- [x] 禁止禁用自己的账号。
- [x] 禁用用户时检查空间管理员不变式。

### P3.5 空间

- [x] 实现创建空间。
- [x] 创建空间时可上传 logo。
- [x] 创建空间时可填写描述。
- [x] logo 和描述不是必填。
- [x] 创建空间时必须指定至少一个 `space_admin`。
- [x] 实现空间成员加入。
- [x] 实现空间成员禁用：`status = disabled`。
- [x] 实现空间成员移除：写入 `deleted_at`。
- [x] 有效成员查询使用 `status = enabled AND deleted_at = 0`。
- [x] 实现空间成员角色变更。
- [x] 所有影响 `space_admin` 数量的操作统一调用 `ValidateSpaceAdminInvariant`。
- [x] 禁止空间失去最后一个启用状态的 `space_admin`。

### P3.6 平台配置与空间配置

- [x] 实现平台配置读取。
- [x] 实现平台配置写入。
- [x] 配置值按 `value_type` 转换。
- [x] 转换失败快速失败。
- [x] 生产密钥不得写入平台配置。
- [x] 实现空间配置读取。
- [x] 实现空间配置写入。
- [x] 配置删除采用硬删除。

**验收标准**：

- [x] 单元测试覆盖平台最后一个管理员不能禁用。
- [x] 单元测试覆盖自己不能禁用自己。
- [x] 单元测试覆盖空间最后一个 `space_admin` 不能禁用、移除、降级。
- [x] 单元测试覆盖自注册用户不属于空间。
- [x] API 测试覆盖租户码注册。

---

## P4. 题库、标签、题目选项、导入

**目标**：完成题库维护、标签归一化、题目选项、解析和导入能力。

**依赖**：P3。

### P4.1 标签

- [x] 实现标签创建。
- [x] 同租户标签名称唯一。
- [x] 实现标签软删除。
- [x] 查询题目标签必须 JOIN `tags` 并过滤 `tags.deleted_at = 0`。
- [x] 实现题目绑定标签。
- [x] 防止题目重复绑定同一标签。

### P4.2 题目基础

- [x] 实现单选题创建。
- [x] 实现多选题创建。
- [x] 实现判断题创建。
- [x] 实现填空题创建。
- [x] 实现简答题创建。
- [x] `analysis` 可选填。
- [x] 支持题目难度。
- [x] 支持题目默认分值。
- [x] 支持租户公共题库。
- [x] 支持空间题库。
- [x] 查询题库时实现公共题库和空间题库可见性规则。

### P4.3 选择题选项

- [x] `option_key` 仅用于出题编辑展示。
- [x] 判分不依赖 `option_key`。
- [x] 新增/编辑选择题时校验至少一个正确答案。
- [x] 单选题校验最终只有一个正确答案。
- [x] 多选题允许多个正确答案。
- [x] `choice_display_count` 支持可选。
- [x] `is_distractor` 控制随机补位池。
- [x] 题目选项编辑采用全量替换。
- [x] 全量替换在同一事务内完成。
- [x] 已生成考试快照不受后续选项编辑影响。

### P4.4 填空题与简答题

- [x] 填空题支持多空标准答案录入。
- [x] 填空题标准答案保存为文本。
- [x] 填空题判分前裁剪首尾空白。
- [x] 填空题首版按完全匹配判分。
- [x] 简答题支持参考答案。
- [x] 简答题默认人工阅卷。

### P4.5 题目导入

- [x] 定义 CSV/Excel 导入模板。
- [x] 导入模板支持题型。
- [x] 导入模板支持题干。
- [x] 导入模板支持选项。
- [x] 导入模板支持正确答案。
- [x] 导入模板支持解析。
- [x] 导入模板支持难度。
- [x] 导入模板支持标签。
- [x] 导入前做数据校验。
- [x] 导入失败返回行号和原因。
- [x] 导入成功写入题库。

**验收标准**：

- [x] 单选题、多选题、判断题、填空题、简答题创建测试通过。
- [x] 多选题选项 ID 排序归一化测试通过。
- [x] 标签软删除后题目标签查询不可见。
- [x] 题目选项全量替换后旧选项不可见。
- [x] 导入错误能定位行号。

---

## P5. 试卷大题、手动组卷、规则组卷

**目标**：完成 `manual`、`rule_fixed`、`rule_live` 三种组卷模式。

**依赖**：P4。

### P5.1 大题结构

- [x] 实现创建大题。
- [x] 实现编辑大题名称。
- [x] 实现编辑大题题型。
- [x] 实现编辑大题作答说明。
- [x] 实现大题排序。
- [x] 保证 `UNIQUE (tenant_id, paper_id, sort_order)`。
- [x] 删除大题时写入 `paper_sections.deleted_at`。
- [x] 删除大题时同一事务硬删除对应 `paper_section_questions`。
- [x] 删除大题时同一事务硬删除对应 `paper_section_rules`。
- [x] 查询大题时过滤 `paper_sections.deleted_at = 0`。

### P5.2 手动组卷

- [x] 实现 `manual` 试卷创建。
- [x] 按大题添加题目。
- [x] 校验同一试卷题目不重复。
- [x] 支持题目在大题内排序。
- [x] 支持题目在试卷内分值覆盖。
- [x] 支持 `paper_section_questions.shuffle_options` 可空布尔。
- [x] 保存后重算大题小计分。
- [x] 保存后重算试卷总分。

### P5.3 rule_fixed

- [x] 实现大题规则配置。
- [x] `difficulty = NULL` 表示不限难度。
- [x] `tag_filter` 使用 JSON 数组字符串。
- [x] 智能规则扩展配置支持题库范围、难度占比、质量优先、最近三次考试排除和本次生成去重。
- [x] 规则顺序稳定。
- [x] 触发生成时按规则抽题。
- [x] 生成后写入 `paper_section_questions`。
- [x] 教师可审题。
- [x] 教师可替换题目。
- [x] 教师可调整顺序。
- [x] 教师可调整分值。
- [x] 生成后后续考试流程等同 `manual`。

### P5.4 rule_live

- [x] 实现 `rule_live` 规则预检查。
- [x] 预检查跨规则合并题池后去重。
- [x] 预检查数量不足时失败。
- [x] 发布时冻结候选题池到 `exam_live_question_pools`。
- [x] 冻结题池后题库变更不影响已发布考试。
- [x] 规则修改必须撤回考试后重新冻结。

### P5.5 聚合重算

- [x] 统一实现试卷聚合重算服务。
- [x] `manual` 按 `paper_section_questions.score` 和题目数重算。
- [x] `rule_fixed` 按 `paper_section_questions.score` 和题目数重算。
- [x] `rule_live` 按 `paper_section_rules.question_count × score_per_question` 重算。
- [x] 增删题目后重算。
- [x] 调整分值后重算。
- [x] 生成固化试卷后重算。
- [x] 修改规则后重算。
- [x] 重算与业务修改在同一事务内完成。

**验收标准**：

- [x] 大题顺序和题号连续编排测试通过。
- [x] `manual` 组卷总分正确。
- [x] `rule_fixed` 生成后可审题和替换。
- [x] `rule_live` 发布时冻结题池。
- [x] 题库变更不影响已发布 `rule_live` 考试。
- [x] 复合外键阻止冗余 `paper_id` 跑偏。

---

## P6. 考试发布、邀请、开始考试

**目标**：完成考试发布、范围、邀请码、attempt 创建和 exam token。

**依赖**：P5。

### P6.1 考试创建与发布

- [x] 创建考试草稿。
- [x] 设置考试开始时间。
- [x] 设置考试结束时间。
- [x] 设置单次作答时长。
- [x] 发布前校验 `duration_minutes <= end_time - start_time`。
- [x] 设置 `max_attempts`。
- [x] 包含简答题时禁止 `max_attempts > 1`。
- [x] 设置 `result_strategy`。
- [x] 设置成绩发布模式。
- [x] 设置成绩公布时间。
- [x] 生成考试邀请码。
- [x] `invite_code` 全局唯一。
- [x] 邀请码生成时全局避冲突，保证公开入口按邀请码解析不会跨租户误命中。
- [x] `rule_live` 发布时冻结题池。

### P6.2 考试范围

- [x] 发布给空间。
- [x] 发布给指定用户。
- [x] 防止重复添加同一空间目标。
- [x] 防止重复添加同一用户目标。
- [x] 邀请码临时参加仍需登录或注册。

### P6.3 开始考试

- [x] 校验考试资格。
- [x] 开始考试按 `exam_targets` 校验直接用户目标和空间学生成员目标。
- [x] 校验考试时间。
- [x] 若存在 `in_progress` attempt，直接返回已有 attempt。
- [x] 若无 `in_progress` attempt，校验未超过 `max_attempts`。
- [x] 生成下一个 `attempt_no`。
- [x] 插入 `exam_attempts`。
- [x] 唯一冲突时只查询已有 `in_progress`，不能递增创建新 attempt。
- [x] 签发不透明 `exam_token`。
- [x] 只保存 `exam_token_hash`。
- [x] 设置 `exam_token_expires_at`。

### P6.4 个人试卷快照

- [x] `manual` 按固化大题题目生成快照。
- [x] `rule_fixed` 按固化大题题目生成快照。
- [x] `rule_live` 从 `exam_live_question_pools` 抽题生成快照。
- [x] 生成 `section_snapshot`。
- [x] 生成 `question_snapshot`。
- [x] 生成 `option_snapshot`。
- [x] 生成 `correct_answer_snapshot`。
- [x] `sort_order` 是全局题号，从 1 连续递增。
- [x] 选择题快照保存最终展示选项 ID 顺序。
- [x] A/B/C/D 只在前端按展示顺序生成。

**验收标准**：

- [x] 开始考试幂等测试通过。
- [x] 并发开始考试不会创建语义重复 attempt。
- [x] `rule_live` 从冻结题池抽题。
- [x] 试卷快照不受后续题库修改影响。
- [x] exam token 只能访问当前 attempt。

---

## P7. 答题、自动保存、交卷、防作弊事件

**目标**：完成考试过程核心链路。

**依赖**：P6。

### P7.1 exam token 校验

- [x] 校验 token hash。
- [x] 校验 attempt 状态为 `in_progress`。
- [x] 校验 token 未过期。
- [x] 校验业务作答截止时间。
- [x] 业务截止时间为 `min(started_at + duration_minutes, exam.end_time)`。
- [x] 超过业务截止时间后拒绝自动保存新答案。
- [x] 允许最多 5 秒网络传输宽限。

### P7.2 自动保存

- [x] 实现答案 upsert。
- [x] 唯一键使用 `UNIQUE (tenant_id, attempt_id, attempt_question_id)`。
- [x] 单选题保存选项 ID。
- [x] 判断题保存选项 ID。
- [x] 多选题保存前选项 ID 升序排序。
- [x] 多选题保存为 JSON 数组字符串。
- [x] 填空题保存文本。
- [x] 简答题保存文本。
- [x] 提交后禁止继续保存。

### P7.3 提交与自动交卷

- [x] 手动提交使用 `status + version` 条件更新。
- [x] 自动交卷使用 `status + version` 条件更新。
- [x] 并发提交更新行数为 0 时按幂等成功返回。
- [x] 提交后锁定答卷。
- [x] 提交时触发客观题自动判分。
- [x] 提交时写关键事件。

### P7.4 防作弊事件

- [x] 前端 `blur` / `focus` 事件 1 秒内节流。
- [x] 非关键事件进入带缓冲 channel。
- [x] 入队成功立即返回。
- [x] 队列满可丢弃非关键事件。
- [x] 队列满记录 WARN。
- [x] `submit` / `auto_submit` 关键事件不能静默丢弃。
- [x] 启动异步事件消费者。
- [x] 异步事件消费者支持批量或逐条落库。

**验收标准**：

- [x] 自动保存 upsert 测试通过。
- [x] 超过业务截止时间自动保存失败。
- [x] 手动提交和自动交卷并发幂等。
- [x] 非关键事件队列满不影响自动保存。
- [x] 关键事件不静默丢弃。

---

## P8. 判分、阅卷、成绩发布与导出

**目标**：完成自动判分、人工阅卷、成绩发布和导出。

**依赖**：P7。

### P8.1 自动判分

- [x] 单选题按选项 ID 判分。
- [x] 判断题按选项 ID 判分。
- [x] 多选题判分前反序列化为 `[]uint64`。
- [x] 多选题两侧排序后逐项比较。
- [x] 禁止多选 JSON 字符串裸比较。
- [x] 多选题首版全对得分，漏选/错选 0 分。
- [x] 填空题裁剪首尾空白后完全匹配。
- [x] 填空题支持按空位顺序完全匹配判分。
- [x] 简答题进入待阅卷。

### P8.2 人工阅卷

- [x] 待阅卷列表按 attempt 粒度展示。
- [x] 简答题支持评分。
- [x] 简答题支持阅卷评语。
- [x] 写入 `graded_by`。
- [x] 写入 `graded_at`。
- [x] 阅卷使用 `version` 乐观锁。
- [x] 阅卷完成后重算主观题得分。
- [x] 阅卷完成后重算总分。
- [x] 阅卷权限按作答记录真实空间校验，空间投放命中目标空间，用户直投展开目标学生当前启用空间成员关系，不能信任请求体 `space_id` 伪造 `attempt` 范围。

### P8.3 成绩发布

- [x] 纯客观题支持立即出分。
- [x] 含简答题不允许立即出分。
- [x] 含简答题不允许 `max_attempts > 1`。
- [x] 教师阅卷后发布模式支持统一公布时间。
- [x] 未到公布时间不展示成绩。
- [x] 到公布时间后展示成绩。
- [x] `latest` 取最后一次提交成绩。
- [x] `highest` 取最高分。
- [x] `show_analysis = true` 且成绩可见后展示解析。
- [x] `show_analysis = false` 不展示解析。

### P8.4 成绩导出

- [x] 支持按考试导出成绩。
- [x] 支持导出考生姓名。
- [x] 支持导出空间名称。
- [x] 支持导出 attempt 次数。
- [x] 支持导出客观题分。
- [x] 支持导出主观题分。
- [x] 支持导出总分。
- [x] 支持导出提交时间。
- [x] 导出文件写入 `server/data/exports` 或配置目录。
- [x] 成绩列表和导出按成绩行真实空间过滤，不能信任请求体 `space_id` 伪造 `exam` 范围。

**验收标准**：

- [x] 多选 `[101,104]` 与 `[101, 104]` 判分一致。
- [x] 填空单空判分测试通过。
- [x] 简答题阅卷乐观锁测试通过。
- [x] 成绩发布时间可见性测试通过。
- [x] 成绩导出字段完整。
- [x] 跨空间伪造阅卷/成绩导出权限回归测试通过。

---

## P9. React 管理端与考试端

**目标**：完成可用的 Web 管理端、考试端和阅卷端。

**依赖**：P3-P8 可并行推进页面。

### P9.1 基础前端

- [x] 配置 React Router。
- [x] 配置 API client。
- [x] 配置登录态保存。
- [x] 配置统一 toast 错误提示：页面中上部展示，5 秒后自动消失，多个提示向下叠加。
- [x] 配置分页组件。
- [x] 配置文件上传组件。

### P9.2 平台管理端

- [x] 平台管理员登录页。
- [x] 登录页支持切换到租户用户登录，学生登录后进入考试入口。
- [x] 个人设置页支持当前账号基础资料查询和保存。
- [x] 租户列表。
- [x] 创建租户表单。
- [x] 创建租户表单必填项显示红色 `*`。
- [x] 创建租户表单支持设置是否开放注册。
- [x] 租户 logo 上传。
- [x] 租户 logo 上传支持左侧拖拽选择、右侧图片预览。
- [x] 租户资料编辑支持名称、描述和 Logo。
- [x] 租户码查看。
- [x] 租户码重置前展示确认弹窗，并说明旧租户码、已发注册链接和手动注册归属的影响。
- [x] 注册开关配置，关闭注册前展示确认弹窗，并说明注册链接、租户码自注册和已注册用户的影响。
- [x] 平台配置页面。

### P9.3 租户管理端

- [x] 用户列表。
- [x] 用户注册/创建。
- [x] 创建用户默认要求首次登录修改初始密码。
- [x] 用户导入。
- [x] 用户头像上传。
- [x] 用户禁用提示。
- [x] 禁用状态用户操作区展示“启用用户”并调用真实启用接口。
- [x] 空间列表。
- [x] 创建空间表单。
- [x] 空间 logo 上传。
- [x] 空间信息编辑，支持修改名称、描述、Logo 和重新指定空间管理员。
- [x] 空间成员管理。
- [x] 空间管理员不变式错误使用统一 toast 提示。
- [x] 空间配置页面作为租户空间下的独立二级菜单。

### P9.4 题库与组卷端

- [x] 题目列表。
- [x] 题目列表展示题干摘要、难度、题型、题目状态、出题人账号、角色、出题时间和操作区。
- [x] 新建和导入题目默认进入草稿状态，必须手动启用后才参与组卷。
- [x] 在线新增题目独立页面覆盖单选、多选、判断、填空和简答题。
- [x] 题目编辑复用独立页面，并在保存时校验题目未被引用。
- [x] 题目操作区支持禁用/启用和删除，删除前校验题目未被引用。
- [x] 选择题选项编辑，支持自定义选项数量。
- [x] 题目解析输入。
- [x] 题干和题目解析使用 `@uiw/react-md-editor` 支持 Markdown 编辑器内置预览。
- [x] 题目难度、默认分值和多标签选择写入真实题库 API。
- [x] 标签管理。
- [x] 题目导入整合到题库页右侧抽屉。
- [x] 试卷列表。
- [x] 大题管理。
- [x] 手动选题。
- [x] `rule_fixed` 规则配置。
- [x] `rule_fixed` 生成试卷。
- [x] `rule_fixed` 审题和替换。
- [x] `rule_live` 规则配置。
- [x] 组卷预检查。

### P9.5 考试端

- [x] 考试入口。
- [x] 邀请码进入。
- [x] 考试说明页。
- [x] 倒计时。
- [x] 大题分组展示。
- [x] 全局题号展示。
- [x] 单选题作答。
- [x] 多选题作答。
- [x] 判断题作答。
- [x] 填空题作答。
- [x] 简答题作答。
- [x] 自动保存状态提示。
- [x] 切屏事件上报。
- [x] 提交确认。
- [x] 成绩可见页。
- [x] 解析展示控制。

### P9.6 阅卷与成绩端

- [x] 待阅卷列表。
- [x] 简答题阅卷页面。
- [x] 阅卷保存。
- [x] 阅卷完成。
- [x] 成绩列表。
- [x] 成绩发布配置。
- [x] 成绩导出。

### P9.7 前后端 API 接入

- [x] 后端启动 HTTP server。
- [x] 注册 `/api/v1` 业务路由。
- [x] API handler 接入 service 层，不直接写数据库逻辑。
- [x] 定义 request DTO。
- [x] 定义 response DTO。
- [x] 认证 API 接入前端登录态。
- [x] 认证 API 使用 `github.com/gin-contrib/sessions` 写入服务端 session。
- [x] 当前 session provider 支持进程内 memory 和 `github.com/gin-contrib/sessions/redis`。
- [x] 登录态以 HttpOnly session cookie 为准，登录响应不暴露 `access_token` / `refresh_token`，前端 API client 通过 `credentials: include` 携带 cookie。
- [x] 退出登录调用 `/api/v1/auth/logout` 清除服务端 session，并让浏览器删除 HttpOnly session cookie。
- [x] 个人设置页接入 `/api/v1/profile`，平台管理员登录账号只读，租户用户保存后同步本地 session 显示名称。
- [x] 个人设置页接入 `/api/v1/profile/password`，租户用户可修改密码并解除首次登录强制改密状态。
- [x] 租户管理页面接入真实 API。
- [x] 租户管理搜索和刷新接入后台列表接口。
- [x] 租户管理搜索输入支持回车触发后台检索。
- [x] 租户管理操作区按钮接入后台接口。
- [x] 通用文件上传 API 接入对象存储抽象。
- [x] 上传文件按年月日时分秒、内容 MD5、随机后缀和安全后缀生成服务端文件名。
- [x] 租户 Logo 创建流程上传真实文件并提交返回 URL。
- [x] 浏览器端图片上传前优先转换为 WebP，失败时回退上传原图。
- [x] 空间管理页面接入真实 API。
- [x] 用户管理页面接入真实 API。
- [x] 题库页面接入真实 API。
- [x] 题目导入页面接入真实 API。
- [x] 试卷页面接入真实 API。
- [x] 组卷规则页面接入真实 API。
- [x] 考试发布页面接入真实 API。
- [x] 考试入口页面接入真实 API。
- [x] 答题页面接入真实 API。
- [x] 阅卷页面接入真实 API。
- [x] 成绩页面接入真实 API。
- [x] 前端移除核心业务页面的本地 mock 数据。
- [x] 空间管理和用户管理缺少租户上下文时不请求 `tenant_id=0`，只在 session 或 URL 提供有效租户 ID 后接入真实 API。
- [x] 后台左侧“租户空间”菜单只对空间管理员展示，平台管理员不展示空间管理和用户管理入口。
- [x] 后台左侧“考试业务”菜单只对空间管理员和教师展示，平台管理员直接访问考试业务后台路由时回到概览。
- [x] 后台左侧新增“总览 / 概览”入口，对平台管理员、租户管理员和教师可见；概览内容按平台、租户/空间和教师视角展示。
- [x] 空间管理列表中的“成员管理”改为复用右侧资源抽屉展示，添加成员由抽屉内按钮打开弹窗完成，抽屉内成员列表支持当前空间内检索、分页、禁用和详情抽屉，保留成员降级和最后管理员不变式校验。
- [x] 租户管理列表中的空间和用户入口改为右侧抽屉查看，独立于租户侧空间管理和用户管理页面，并保持原空间、用户列表列结构。
- [x] 租户管理资源抽屉默认占用 50% 视口宽度，支持全屏展开和退出全屏，宽度切换保持动画。
- [x] 租户管理资源抽屉叠在全屏遮罩层上滑入滑出，不在父级布局中预留白色占位区域。
- [x] 租户管理资源抽屉遮罩层随抽屉打开淡入、关闭淡出。
- [x] 租户管理资源抽屉返回和关闭按钮先触发滑出动画，动画结束后再关闭抽屉，遮罩层不触发关闭。
- [x] 租户管理资源抽屉初始位于视口外，再过渡滑入，避免先闪现白色面板。
- [x] 本地 SQLite dev seed 覆盖默认联调租户、用户、空间、试卷和考试数据。
- [x] API 错误码映射到统一错误提示。
- [x] 前端 API 收到 HTTP 401 后清空登录态并跳转登录页。
- [x] 当前账号资料 API handler 使用 SQLite 覆盖平台管理员和租户用户查询、保存链路。
- [x] 考试发布 API handler 使用 SQLite 覆盖列表和发布链路。
- [x] 租户管理 API handler 使用 SQLite 覆盖列表、创建、资料更新、租户码重置和注册开关链路。
- [x] 租户管理列表和写接口要求平台管理员登录态。
- [x] 租户管理写接口从 session 登录态解析平台管理员 ID，并写入 `created_by` / `updated_by`。
- [x] 后台考试、空间、用户、题库、试卷和组卷规则读取接口拒绝匿名访问。
- [x] 考试业务 API 允许本租户 `tenant_admin` 或授权空间内 `space_admin` / `teacher` 访问，平台管理员和学生不拥有考试业务操作入口。
- [x] 租户、空间、用户等后台管理写接口要求平台管理员或本租户 `tenant_admin`，并拒绝学生越权访问。
- [x] 管理端创建用户必须提交初始密码，不再写入固定临时密码。
- [x] 管理端创建用户密码遵守 `security.password_min_length`。
- [x] 管理端创建用户默认写入 `force_password_change`，登录响应和后台路由守卫会引导用户先到个人设置页改密。
- [x] 管理端创建教师不要求选择空间，教师初始不自动加入 `space_members`。
- [x] 零空间教师访问题库、试卷、考试或阅卷页面时展示无空间提示，不触发越权业务列表请求。
- [x] 公共题库和已暴露公共试卷写接口按真实资源范围校验；教师可写公共题库，公共试卷内容仍仅租户管理员可写，目标空间管理员仅可把未引用公共试卷归属到自己授权空间，已引用资源禁止删除或迁移归属。
- [x] 学生查分接入 `/api/v1/exam-entry/results/:id`，提交后按发布策略展示本人分数或等待公布提示。
- [x] 教师成绩页不展示成绩导出入口，后端导出接口继续由 `CanExportExamResults` 拒绝教师导出。
- [x] 开始考试 API 从邀请码入口 session 派生考生身份，不再信任开考请求体里的 `user_id`。
- [x] 文件上传 API handler 覆盖 multipart 上传、平台管理员租户 Logo 上传、缺少文件错误和伪造图片 MIME 拒绝。
- [x] 空间管理 API handler 使用 SQLite 覆盖列表和创建链路。
- [x] 用户管理 API handler 使用 SQLite 覆盖列表、创建、禁用和启用链路。
- [x] 用户管理 API handler 覆盖租户管理员仅能管理本租户、学生不能创建用户。
- [x] 题库 API handler 使用 SQLite 覆盖列表、在线保存、编辑、禁用和删除题目链路。
- [x] 覆盖主要 API handler 测试。
- [x] 覆盖前端 API client 集成测试。
- [x] 覆盖前端上传 API client 和租户 Logo 上传联动测试。
- [ ] 使用浏览器完成发布考试、进入考试、自动保存、提交、阅卷、发布成绩主链路联调。

**验收标准**：

- [x] 管理员可以创建租户、空间、用户。
- [x] 教师可以完成出题、组卷、发布考试。
- [x] 考生可以进入考试、答题、自动保存、提交。
- [x] 教师可以阅卷和发布成绩。
- [x] 成绩导出可用。
- [x] 核心页面使用真实 API 数据，不依赖本地 mock。
- [x] 前端到后端的认证、租户、空间、题库、试卷、考试、答题、阅卷、成绩主链路可联调。

---

## P10. 部署、压测、质量收口

**目标**：完成 Docker Compose、SQLite 单机、100 人在线验证和文档收口。

**依赖**：P1-P9.7。

### P10.1 Docker Compose

- [x] 编写 PostgreSQL Compose。
- [x] 编写 server Compose。
- [x] 编写 web/Nginx Compose。
- [x] 配置环境变量注入。
- [x] 不提交生产密钥。
- [x] server 镜像内复制默认配置并安装 SQLite CGO 构建工具链。
- [x] Nginx 同时代理 `/api/` 与 `/uploads/`。
- [x] Nginx 上传大小限制与后端 10MB 限制保持一致。
- [ ] 验证从空库迁移启动。

### P10.2 SQLite 单机

- [x] 编写 SQLite 单机启动脚本。
- [x] SQLite DSN 包含 `_foreign_keys=on`。
- [x] SQLite DSN 包含 `_journal_mode=WAL`。
- [x] SQLite DSN 包含 `_busy_timeout=5000`。
- [x] 构建命令带 `json1` 标签。
- [x] 明确标注 SQLite 不推荐 100 人正式考试。

### P10.3 测试矩阵

- [ ] PostgreSQL 迁移测试。
- [ ] SQLite 迁移测试。
- [ ] MySQL 迁移兼容测试。
- [ ] 权限抽象测试。
- [ ] 租户隔离测试。
- [ ] 空间管理员不变式测试。
- [ ] 题库可见性测试。
- [x] rule_fixed 生成测试。
- [x] rule_live 题池冻结测试。
- [ ] 开始考试幂等测试。
- [ ] 自动保存 upsert 测试。
- [ ] 多选判分测试。
- [ ] 业务截止时间测试。
- [x] 事件异步队列测试。
- [ ] 成绩发布测试。

### P10.4 100 人在线链路验证

- [ ] 使用 PostgreSQL 或 MySQL 8.0+ 环境。
- [ ] 准备 100 个考生账号。
- [ ] 准备包含客观题和填空题的考试。
- [ ] 100 人同时开始考试。
- [ ] 验证 attempt 不重复。
- [ ] 验证自动保存成功率。
- [ ] 验证提交成功率。
- [ ] 验证 `exam_events` 不阻塞自动保存。
- [ ] 验证成绩计算正确。
- [ ] 输出压测报告。

### P10.5 文档收口

- [ ] 更新 README。
- [ ] 编写本地开发启动文档。
- [ ] 编写 Docker Compose 部署文档。
- [ ] 编写 SQLite 单机说明。
- [ ] 编写数据库迁移说明。
- [ ] 编写题目导入模板说明。
- [ ] 编写 API 文档。
- [ ] 标注首版不支持项。

**验收标准**：

- [ ] PostgreSQL Docker Compose 一键启动成功。
- [ ] SQLite 单机启动成功。
- [ ] 100 人在线链路验证通过。
- [ ] README 能指导新开发者启动项目。
- [ ] API 文档覆盖认证、租户、空间、题库、试卷、考试、答题、阅卷、成绩。

---

## 11. 阶段 Review Gate

- [ ] P0 Review：目录结构、依赖、配置文件和 `.gitignore` 符合方案。
- [ ] P1 Review：服务能启动，配置、日志、数据库连接边界正确。
- [ ] P2 Review：迁移 SQL、中文注释、约束、索引、DO 字段映射完整。
- [ ] P3 Review：账号、租户、空间、权限抽象和不变式可测。
- [ ] P4 Review：题库模型、选项、标签、导入边界正确。
- [ ] P5 Review：大题结构和三种组卷模式无遗漏。
- [ ] P6 Review：考试发布、题池冻结、attempt、exam token 正确。
- [ ] P7 Review：自动保存、提交、防作弊事件不阻塞核心链路。
- [x] P8 Review：判分、阅卷、成绩发布和导出可追溯。
- [x] P9 Review：核心页面完成闭环。
- [ ] P9.7 Review：前后端 API 接入主链路完成。
- [ ] P10 Review：部署、压测、文档收口完成。

## 12. 明确延期项

- [ ] 摄像头监考。
- [ ] 人脸识别。
- [ ] 锁屏客户端。
- [ ] 录屏。
- [ ] AI 判卷。
- [ ] 复杂 RBAC 权限配置 UI。
- [ ] 多租户独立数据库。
- [ ] Kubernetes 部署。
- [x] 多空填空题。
- [ ] 多个等价填空答案。
- [ ] 填空题正则匹配。
- [ ] 大规模考试轻量快照存储优化。
- [ ] 平台管理员 impersonation。
- [ ] 游标分页。
- [ ] Prometheus 指标。
- [ ] OpenTelemetry tracing。

## 13. 自检覆盖

- [ ] 目录结构已覆盖。
- [ ] 配置和安全边界已覆盖。
- [ ] PostgreSQL / MySQL 8.0+ / SQLite 差异已覆盖。
- [ ] `gorm.io/datatypes` 和 SQLite `json1` 已覆盖。
- [ ] `soft_delete` 与唯一索引已覆盖。
- [ ] 全部核心表已覆盖。
- [ ] 关键唯一约束和复合外键已覆盖。
- [ ] 权限抽象已覆盖。
- [ ] 租户、空间、用户已覆盖。
- [ ] 平台配置和空间配置已覆盖。
- [ ] 题库、标签、选项、导入已覆盖。
- [ ] 大题结构和三种组卷模式已覆盖。
- [x] `rule_live` 发布态题池冻结已覆盖。
- [ ] 考试发布、范围、邀请码已覆盖。
- [ ] attempt 幂等和 exam token 已覆盖。
- [ ] 业务截止时间硬校验已覆盖。
- [ ] 自动保存 upsert 已覆盖。
- [ ] 多选答案排序和反序列化判分已覆盖。
- [x] 防作弊事件异步写入已覆盖。
- [x] 阅卷、成绩发布、导出已覆盖。
- [x] React 页面闭环已覆盖。
- [ ] 前后端 API 接入已覆盖。
- [ ] 部署、压测、文档收口已覆盖。

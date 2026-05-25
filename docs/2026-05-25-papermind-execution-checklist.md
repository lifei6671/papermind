# PaperMind 首版执行任务清单

> 本清单由 `docs/2026-05-25-papermind-exam-platform-technical-design.md` 拆解而来。
> 目标是把技术方案拆成可执行、可验收、可回写进度的任务。

## 0. 执行原则

- [ ] 每个阶段完成后先做代码审查，再进入下一阶段。
- [ ] 所有数据库迁移 SQL 必须包含中文表注释、字段注释、关键索引/约束注释。
- [ ] 所有业务表保留 `created_at`、`created_by`、`updated_at`、`updated_by`、`version`、`ext_json`，豁免表按方案明确处理。
- [ ] 所有软删除核心表使用 `gorm.io/plugin/soft_delete` Unix 时间戳模式。
- [ ] 所有 GORM 实体必须显式声明 `gorm:"column:xxx"` 字段映射。
- [ ] 字段映射与数据库实体放在同一个文件。
- [ ] `service` 层禁止直接使用 GORM 和 SQL。
- [ ] 数据库访问统一封装在 `server/internal/dao/db`。
- [ ] 权限判断统一通过 `PermissionChecker`，业务 service 不散落角色字符串判断。
- [ ] SQLite 只作为演示、本地开发和低并发单机模式，不作为 100 人正式考试推荐部署。
- [ ] 正式 100 人在线考试优先验证 PostgreSQL，MySQL 作为兼容目标。

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
  → P10 部署、压测、质量收口
```

---

## P0. 文档与项目骨架

**目标**：建立前后端工程骨架和约定目录，不实现业务。

**依赖**：无。

### P0.1 后端目录

- [ ] 创建 `server/api/router`。
- [ ] 创建 `server/api/middleware`。
- [ ] 创建 `server/api/request`。
- [ ] 创建 `server/api/response`。
- [ ] 创建 `server/api/v1`。
- [ ] 创建 `server/bootstrap`。
- [ ] 创建 `server/cmd/papermind`。
- [ ] 创建 `server/conf`。
- [ ] 创建 `server/data/migrations/postgres`。
- [ ] 创建 `server/data/migrations/mysql`。
- [ ] 创建 `server/data/migrations/sqlite`。
- [ ] 创建 `server/data/seeds`。
- [ ] 创建 `server/data/sqlite`。
- [ ] 创建 `server/data/imports`。
- [ ] 创建 `server/data/exports`。
- [ ] 创建 `server/internal/service`。
- [ ] 创建 `server/internal/service/permission`。
- [ ] 创建 `server/internal/dao/db`。
- [ ] 创建 `server/internal/dao/external`。
- [ ] 创建 `server/internal/model`。
- [ ] 创建 `server/internal/dto`。
- [ ] 创建 `server/internal/job`。
- [ ] 创建 `server/library/code`。
- [ ] 创建 `server/library/constant`。
- [ ] 创建 `server/library/logger`。
- [ ] 创建 `server/library/config`。
- [ ] 创建 `server/library/validator`。
- [ ] 创建 `server/library/response`。
- [ ] 创建 `server/library/crypto`。
- [ ] 创建 `server/library/xerr`。
- [ ] 创建 `server/mock`。
- [ ] 创建 `server/script`。
- [ ] 创建 `server/tests`。

### P0.2 前端与部署目录

- [ ] 创建 `web`。
- [ ] 创建 `deployments/docker-compose`。
- [ ] 创建 `deployments/sqlite-single-node`。
- [ ] 创建 `docs/api`。
- [ ] 创建 `docs/database`。
- [ ] 创建 `docs/specs`。

### P0.3 项目基础文件

- [ ] 初始化 `server/go.mod`。
- [ ] 引入 `github.com/gin-gonic/gin`。
- [ ] 引入 `gorm.io/gorm`。
- [ ] 引入 `gorm.io/driver/postgres`。
- [ ] 引入 `gorm.io/driver/mysql`。
- [ ] 引入 `gorm.io/driver/sqlite`。
- [ ] 引入 `gorm.io/datatypes`。
- [ ] 引入 `gorm.io/plugin/soft_delete`。
- [ ] 创建 `server/conf/app.yaml` 开发配置模板。
- [ ] 确认 `.gitignore` 排除 `server/conf_online/`、私有配置、SQLite 数据库文件、导入导出临时文件、构建产物。
- [ ] 初始化 `web` 为 React + Vite + TypeScript 项目。

**验收标准**：

- [ ] `go test ./...` 可以在空业务骨架下运行。
- [ ] `go test -tags json1 ./...` 可以运行。
- [ ] `web` 可以启动 Vite 开发服务。
- [ ] 仓库不存在 `server/conf_online` 生产配置目录。

---

## P1. 配置、日志、数据库基础设施

**目标**：完成服务启动、配置加载、日志、数据库连接和基础响应。

**依赖**：P0。

### P1.1 YAML 配置

- [ ] 定义 `server/library/config` 配置结构。
- [ ] 支持 `app.name`。
- [ ] 支持 `app.env`。
- [ ] 支持 `app.http_port`。
- [ ] 支持 `app.public_url`。
- [ ] 支持 `database.driver`。
- [ ] 支持 `database.dsn`。
- [ ] 支持 `database.max_open_conns`。
- [ ] 支持 `database.max_idle_conns`。
- [ ] 支持 `auth.access_token_ttl`。
- [ ] 支持 `auth.refresh_token_ttl`。
- [ ] 支持 `auth.exam_token_buffer_minutes`。
- [ ] 支持 `storage.temp_dir`。
- [ ] 支持 `storage.import_dir`。
- [ ] 支持 `storage.export_dir`。
- [ ] 支持 `security.allow_register_default`。
- [ ] 支持 `security.password_min_length`。
- [ ] 支持 `security.cors_origins`。
- [ ] 支持环境变量覆盖敏感配置。
- [ ] 启动时校验存储目录存在且可写。

### P1.2 日志与响应

- [ ] 实现统一 `request_id` 中间件。
- [ ] 实现 Gin 请求日志中间件。
- [ ] 实现 panic recover 中间件。
- [ ] 实现统一响应结构 `{ code, message, data }`。
- [ ] 实现分页响应结构 `{ items, page, page_size, total }`。
- [ ] 定义错误码目录 `server/library/code`。
- [ ] 定义业务错误封装 `server/library/xerr`。

### P1.3 数据库连接

- [ ] 根据 `database.driver` 初始化 PostgreSQL。
- [ ] 根据 `database.driver` 初始化 MySQL。
- [ ] 根据 `database.driver` 初始化 SQLite。
- [ ] SQLite DSN 必须包含 `_foreign_keys=on`。
- [ ] SQLite DSN 必须包含 `_journal_mode=WAL`。
- [ ] SQLite DSN 必须包含 `_busy_timeout=5000`。
- [ ] SQLite 默认 `max_open_conns = 1`。
- [ ] 支持配置 SQLite 小规模多人模式 `max_open_conns = 5-10`。
- [ ] PostgreSQL / MySQL 按配置设置连接池。
- [ ] 数据库初始化必须输出 driver、连接池参数和迁移目录日志。

### P1.4 GORM 基础约束

- [ ] 实体统一使用显式 `gorm:"column:xxx"`。
- [ ] `ext_json` 统一使用 `datatypes.JSON`。
- [ ] 软删除字段使用 `soft_delete.DeletedAt` Unix 时间戳模式。
- [ ] 字段映射常量与实体同文件维护。
- [ ] 禁止 service 层直接 import GORM。

**验收标准**：

- [ ] 三种数据库 driver 初始化逻辑都有单元测试。
- [ ] SQLite 测试命令使用 `go test -tags json1 ./server/...`。
- [ ] 配置目录缺失或不可写时启动失败且错误明确。
- [ ] API 返回结构统一。

---

## P2. 数据库迁移与 DAO 基础模型

**目标**：落地首版完整 schema、中文注释、约束、索引和 GORM DO。

**依赖**：P1。

### P2.1 迁移框架

- [ ] 建立 PostgreSQL 迁移执行脚本。
- [ ] 建立 MySQL 迁移执行脚本。
- [ ] 建立 SQLite 迁移执行脚本。
- [ ] 迁移 SQL 文件按版本号排序执行。
- [ ] 迁移 SQL 必须可重复检测已执行版本。
- [ ] 迁移失败必须停止启动。

### P2.2 基础字段规范

- [ ] 所有业务表包含 `created_at`。
- [ ] 所有业务表包含 `created_by`。
- [ ] 所有非豁免表包含 `updated_at`。
- [ ] 所有非豁免表包含 `updated_by`。
- [ ] 所有非豁免表包含 `version`。
- [ ] 所有表包含 `ext_json`。
- [ ] 核心主表包含 `deleted_at`。
- [ ] 纯关系表按方案豁免 `updated_at`、`updated_by`、`version`。
- [ ] `exam_events` 按方案豁免 `updated_at`、`updated_by`、`version`。

### P2.3 租户与空间表

- [ ] 创建 `tenants`。
- [ ] `tenants` 包含 `logo_url`。
- [ ] `tenants` 包含 `description`。
- [ ] `tenants.tenant_code` 全平台唯一。
- [ ] 创建 `spaces`。
- [ ] `spaces` 包含 `logo_url`。
- [ ] `spaces` 包含 `description`。
- [ ] 创建 `space_members`。
- [ ] `space_members` 包含 `role_in_space`。
- [ ] `space_members` 包含 `status`。
- [ ] `space_members` 建立 `UNIQUE (tenant_id, space_id, user_id, deleted_at)`。
- [ ] 创建 `space_configs`。
- [ ] `space_configs` 建立 `UNIQUE (tenant_id, space_id, config_key)`。
- [ ] 配置表不使用软删除。

### P2.4 用户与角色表

- [ ] 创建 `platform_users`。
- [ ] `platform_users` 包含 `avatar_url`。
- [ ] `platform_users` 包含 `last_login_ip`。
- [ ] `platform_users` 包含 `last_login_at`。
- [ ] `platform_users` 建立 `UNIQUE (username, deleted_at)`。
- [ ] `platform_users` 建立 `UNIQUE (phone, deleted_at)`。
- [ ] `platform_users` 建立 `UNIQUE (email, deleted_at)`。
- [ ] 创建 `platform_configs`。
- [ ] `platform_configs` 建立 `UNIQUE (config_key)`。
- [ ] 创建 `users`。
- [ ] `users` 包含 `real_name`。
- [ ] `users` 包含 `avatar_url`。
- [ ] `users` 包含 `last_login_ip`。
- [ ] `users` 包含 `last_login_at`。
- [ ] `users` 建立 `UNIQUE (tenant_id, username, deleted_at)`。
- [ ] `users` 建立 `UNIQUE (tenant_id, phone, deleted_at)`。
- [ ] `users` 建立 `UNIQUE (tenant_id, email, deleted_at)`。
- [ ] 创建 `user_roles`。
- [ ] `user_roles` 建立 `UNIQUE (tenant_id, user_id, role)`。

### P2.5 题库表

- [ ] 创建 `questions`。
- [ ] `questions.type` 支持 `single`。
- [ ] `questions.type` 支持 `multiple`。
- [ ] `questions.type` 支持 `judge`。
- [ ] `questions.type` 支持 `fill_blank`。
- [ ] `questions.type` 支持 `short_text`。
- [ ] `questions.analysis` 可为空。
- [ ] 创建 `question_options`。
- [ ] `question_options` 包含 `option_key`，仅用于编辑展示。
- [ ] `question_options` 包含 `sort_order`。
- [ ] `question_options` 建立 `UNIQUE (tenant_id, question_id, option_key)`。
- [ ] `question_options` 建立 `UNIQUE (tenant_id, question_id, sort_order)`。
- [ ] 创建 `tags`。
- [ ] `tags.name` 在同租户唯一，唯一约束包含 `deleted_at`。
- [ ] 创建 `question_tags`。
- [ ] `question_tags` 建立 `UNIQUE (tenant_id, question_id, tag_id)`。

### P2.6 试卷与组卷表

- [ ] 创建 `papers`。
- [ ] `papers.build_mode` 支持 `manual`。
- [ ] `papers.build_mode` 支持 `rule_fixed`。
- [ ] `papers.build_mode` 支持 `rule_live`。
- [ ] `papers.show_analysis` 控制解析展示。
- [ ] 创建 `paper_sections`。
- [ ] `paper_sections` 包含 `deleted_at`。
- [ ] `paper_sections` 建立 `UNIQUE (tenant_id, paper_id, sort_order)`。
- [ ] `paper_sections` 建立 `UNIQUE (tenant_id, id, paper_id)`。
- [ ] 创建 `paper_section_questions`。
- [ ] `paper_section_questions` 建立 `UNIQUE (tenant_id, paper_id, question_id)`。
- [ ] `paper_section_questions` 建立 `UNIQUE (tenant_id, section_id, sort_order)`。
- [ ] `paper_section_questions` 建立复合外键 `(tenant_id, section_id, paper_id)` 引用 `paper_sections(tenant_id, id, paper_id)`。
- [ ] 创建 `paper_section_rules`。
- [ ] `paper_section_rules.difficulty` 允许为空，表示不限难度。
- [ ] `paper_section_rules` 建立 `UNIQUE (tenant_id, section_id, sort_order)`。
- [ ] `paper_section_rules` 建立复合外键 `(tenant_id, section_id, paper_id)` 引用 `paper_sections(tenant_id, id, paper_id)`。

### P2.7 考试与答题表

- [ ] 创建 `exams`。
- [ ] `exams.max_attempts` 默认 1。
- [ ] `exams.result_strategy` 支持 `latest`。
- [ ] `exams.result_strategy` 支持 `highest`。
- [ ] `exams.invite_code` 建立 `UNIQUE (tenant_id, invite_code)`。
- [ ] 创建 `exam_targets`。
- [ ] `exam_targets` 建立 `UNIQUE (tenant_id, exam_id, target_type, target_id)`。
- [ ] 创建 `exam_live_question_pools`。
- [ ] `exam_live_question_pools` 建立 `UNIQUE (tenant_id, exam_id, section_id, rule_id, question_id)`。
- [ ] 创建 `exam_attempts`。
- [ ] `exam_attempts` 建立 `UNIQUE (tenant_id, exam_id, user_id, attempt_no)`。
- [ ] `exam_attempts` 建立 `INDEX (exam_token_hash, status)`。
- [ ] 创建 `exam_attempt_questions`。
- [ ] `exam_attempt_questions` 包含 `section_snapshot`。
- [ ] `exam_attempt_questions.sort_order` 为全局题号。
- [ ] 创建 `exam_answers`。
- [ ] `exam_answers` 建立 `UNIQUE (tenant_id, attempt_id, attempt_question_id)`。
- [ ] 创建 `exam_events`。

**验收标准**：

- [ ] PostgreSQL 迁移 SQL 包含表和字段中文注释。
- [ ] MySQL 迁移 SQL 包含表和字段中文注释。
- [ ] SQLite 迁移 SQL 使用 `--` 注释说明表和字段含义。
- [ ] 三套迁移均能从空库执行成功。
- [ ] SQLite 外键约束实际生效。
- [ ] 所有 DO 文件有实体和字段映射。

---

## P3. 权限、账号、租户、空间

**目标**：完成平台管理员、租户、用户、空间和权限抽象。

**依赖**：P2。

### P3.1 权限抽象

- [ ] 创建 `server/internal/service/permission/checker.go`。
- [ ] 定义 `PermissionChecker` 接口。
- [ ] 创建 `server/internal/service/permission/context.go`。
- [ ] 定义 `PermissionContext`。
- [ ] 创建 `server/internal/service/permission/fixed_role.go`。
- [ ] 实现 `CanManageTenant`。
- [ ] 实现 `CanManageSpace`。
- [ ] 实现 `CanManageQuestion`。
- [ ] 实现 `CanPublishExam`。
- [ ] 实现 `CanGradeAttempt`。
- [ ] 实现 `CanTakeExam`。
- [ ] service 层统一调用 `PermissionChecker`。

### P3.2 平台用户

- [ ] 实现平台管理员初始化 seed。
- [ ] 实现平台管理员登录。
- [ ] 登录成功更新 `last_login_ip`。
- [ ] 登录成功更新 `last_login_at`。
- [ ] 登录失败记录安全日志。
- [ ] 实现平台用户头像上传。
- [ ] 头像上传限制文件类型。
- [ ] 头像上传限制文件大小。
- [ ] 禁止平台管理员禁用自己。
- [ ] 禁止禁用最后一个启用状态平台管理员。

### P3.3 租户

- [ ] 实现创建租户。
- [ ] 创建租户时生成唯一 `tenant_code`。
- [ ] 新租户 `allow_register` 继承 `security.allow_register_default`。
- [ ] 实现查看租户码。
- [ ] 实现重置租户码。
- [ ] 实现启用租户。
- [ ] 实现禁用租户。
- [ ] 实现租户 logo 保存。
- [ ] 实现租户描述保存。

### P3.4 租户用户

- [ ] 实现租户用户注册。
- [ ] 支持租户注册链接注册。
- [ ] 支持手动输入租户码注册。
- [ ] 自注册用户默认不属于任何空间。
- [ ] 实现租户用户登录。
- [ ] 登录成功更新 `last_login_ip`。
- [ ] 登录成功更新 `last_login_at`。
- [ ] 实现租户用户头像上传。
- [ ] 实现用户禁用。
- [ ] 禁用用户前提示影响范围。
- [ ] 禁止禁用自己的账号。
- [ ] 禁用用户时检查空间管理员不变式。

### P3.5 空间

- [ ] 实现创建空间。
- [ ] 创建空间时可上传 logo。
- [ ] 创建空间时可填写描述。
- [ ] logo 和描述不是必填。
- [ ] 创建空间时必须指定至少一个 `space_admin`。
- [ ] 实现空间成员加入。
- [ ] 实现空间成员禁用：`status = disabled`。
- [ ] 实现空间成员移除：写入 `deleted_at`。
- [ ] 有效成员查询使用 `status = enabled AND deleted_at = 0`。
- [ ] 实现空间成员角色变更。
- [ ] 所有影响 `space_admin` 数量的操作统一调用 `ValidateSpaceAdminInvariant`。
- [ ] 禁止空间失去最后一个启用状态的 `space_admin`。

### P3.6 平台配置与空间配置

- [ ] 实现平台配置读取。
- [ ] 实现平台配置写入。
- [ ] 配置值按 `value_type` 转换。
- [ ] 转换失败快速失败。
- [ ] 生产密钥不得写入平台配置。
- [ ] 实现空间配置读取。
- [ ] 实现空间配置写入。
- [ ] 配置删除采用硬删除。

**验收标准**：

- [ ] 单元测试覆盖平台最后一个管理员不能禁用。
- [ ] 单元测试覆盖自己不能禁用自己。
- [ ] 单元测试覆盖空间最后一个 `space_admin` 不能禁用、移除、降级。
- [ ] 单元测试覆盖自注册用户不属于空间。
- [ ] API 测试覆盖租户码注册。

---

## P4. 题库、标签、题目选项、导入

**目标**：完成题库维护、标签归一化、题目选项、解析和导入能力。

**依赖**：P3。

### P4.1 标签

- [ ] 实现标签创建。
- [ ] 同租户标签名称唯一。
- [ ] 实现标签软删除。
- [ ] 查询题目标签必须 JOIN `tags` 并过滤 `tags.deleted_at = 0`。
- [ ] 实现题目绑定标签。
- [ ] 防止题目重复绑定同一标签。

### P4.2 题目基础

- [ ] 实现单选题创建。
- [ ] 实现多选题创建。
- [ ] 实现判断题创建。
- [ ] 实现填空题创建。
- [ ] 实现简答题创建。
- [ ] `analysis` 可选填。
- [ ] 支持题目难度。
- [ ] 支持题目默认分值。
- [ ] 支持租户公共题库。
- [ ] 支持空间题库。
- [ ] 查询题库时实现公共题库和空间题库可见性规则。

### P4.3 选择题选项

- [ ] `option_key` 仅用于出题编辑展示。
- [ ] 判分不依赖 `option_key`。
- [ ] 新增/编辑选择题时校验至少一个正确答案。
- [ ] 单选题校验最终只有一个正确答案。
- [ ] 多选题允许多个正确答案。
- [ ] `choice_display_count` 支持可选。
- [ ] `is_distractor` 控制随机补位池。
- [ ] 题目选项编辑采用全量替换。
- [ ] 全量替换在同一事务内完成。
- [ ] 已生成考试快照不受后续选项编辑影响。

### P4.4 填空题与简答题

- [ ] 填空题首版仅支持单空。
- [ ] 填空题标准答案保存为文本。
- [ ] 填空题判分前裁剪首尾空白。
- [ ] 填空题首版按完全匹配判分。
- [ ] 简答题支持参考答案。
- [ ] 简答题默认人工阅卷。

### P4.5 题目导入

- [ ] 定义 CSV/Excel 导入模板。
- [ ] 导入模板支持题型。
- [ ] 导入模板支持题干。
- [ ] 导入模板支持选项。
- [ ] 导入模板支持正确答案。
- [ ] 导入模板支持解析。
- [ ] 导入模板支持难度。
- [ ] 导入模板支持标签。
- [ ] 导入前做数据校验。
- [ ] 导入失败返回行号和原因。
- [ ] 导入成功写入题库。

**验收标准**：

- [ ] 单选题、多选题、判断题、填空题、简答题创建测试通过。
- [ ] 多选题选项 ID 排序归一化测试通过。
- [ ] 标签软删除后题目标签查询不可见。
- [ ] 题目选项全量替换后旧选项不可见。
- [ ] 导入错误能定位行号。

---

## P5. 试卷大题、手动组卷、规则组卷

**目标**：完成 `manual`、`rule_fixed`、`rule_live` 三种组卷模式。

**依赖**：P4。

### P5.1 大题结构

- [ ] 实现创建大题。
- [ ] 实现编辑大题名称。
- [ ] 实现编辑大题题型。
- [ ] 实现编辑大题作答说明。
- [ ] 实现大题排序。
- [ ] 保证 `UNIQUE (tenant_id, paper_id, sort_order)`。
- [ ] 删除大题时写入 `paper_sections.deleted_at`。
- [ ] 删除大题时同一事务硬删除对应 `paper_section_questions`。
- [ ] 删除大题时同一事务硬删除对应 `paper_section_rules`。
- [ ] 查询大题时过滤 `paper_sections.deleted_at = 0`。

### P5.2 手动组卷

- [ ] 实现 `manual` 试卷创建。
- [ ] 按大题添加题目。
- [ ] 校验同一试卷题目不重复。
- [ ] 支持题目在大题内排序。
- [ ] 支持题目在试卷内分值覆盖。
- [ ] 支持 `paper_section_questions.shuffle_options` 可空布尔。
- [ ] 保存后重算大题小计分。
- [ ] 保存后重算试卷总分。

### P5.3 rule_fixed

- [ ] 实现大题规则配置。
- [ ] `difficulty = NULL` 表示不限难度。
- [ ] `tag_filter` 使用 JSON 数组字符串。
- [ ] 规则顺序稳定。
- [ ] 触发生成时按规则抽题。
- [ ] 生成后写入 `paper_section_questions`。
- [ ] 教师可审题。
- [ ] 教师可替换题目。
- [ ] 教师可调整顺序。
- [ ] 教师可调整分值。
- [ ] 生成后后续考试流程等同 `manual`。

### P5.4 rule_live

- [ ] 实现 `rule_live` 规则预检查。
- [ ] 预检查跨规则合并题池后去重。
- [ ] 预检查数量不足时失败。
- [ ] 发布时冻结候选题池到 `exam_live_question_pools`。
- [ ] 冻结题池后题库变更不影响已发布考试。
- [ ] 规则修改必须撤回考试后重新冻结。

### P5.5 聚合重算

- [ ] 统一实现试卷聚合重算服务。
- [ ] `manual` 按 `paper_section_questions.score` 和题目数重算。
- [ ] `rule_fixed` 按 `paper_section_questions.score` 和题目数重算。
- [ ] `rule_live` 按 `paper_section_rules.question_count × score_per_question` 重算。
- [ ] 增删题目后重算。
- [ ] 调整分值后重算。
- [ ] 生成固化试卷后重算。
- [ ] 修改规则后重算。
- [ ] 重算与业务修改在同一事务内完成。

**验收标准**：

- [ ] 大题顺序和题号连续编排测试通过。
- [ ] `manual` 组卷总分正确。
- [ ] `rule_fixed` 生成后可审题和替换。
- [ ] `rule_live` 发布时冻结题池。
- [ ] 题库变更不影响已发布 `rule_live` 考试。
- [ ] 复合外键阻止冗余 `paper_id` 跑偏。

---

## P6. 考试发布、邀请、开始考试

**目标**：完成考试发布、范围、邀请码、attempt 创建和 exam token。

**依赖**：P5。

### P6.1 考试创建与发布

- [ ] 创建考试草稿。
- [ ] 设置考试开始时间。
- [ ] 设置考试结束时间。
- [ ] 设置单次作答时长。
- [ ] 发布前校验 `duration_minutes <= end_time - start_time`。
- [ ] 设置 `max_attempts`。
- [ ] 包含简答题时禁止 `max_attempts > 1`。
- [ ] 设置 `result_strategy`。
- [ ] 设置成绩发布模式。
- [ ] 设置成绩公布时间。
- [ ] 生成考试邀请码。
- [ ] `invite_code` 在同租户唯一。
- [ ] `rule_live` 发布时冻结题池。

### P6.2 考试范围

- [ ] 发布给空间。
- [ ] 发布给指定用户。
- [ ] 防止重复添加同一空间目标。
- [ ] 防止重复添加同一用户目标。
- [ ] 邀请码临时参加仍需登录或注册。

### P6.3 开始考试

- [ ] 校验考试资格。
- [ ] 校验考试时间。
- [ ] 若存在 `in_progress` attempt，直接返回已有 attempt。
- [ ] 若无 `in_progress` attempt，校验未超过 `max_attempts`。
- [ ] 生成下一个 `attempt_no`。
- [ ] 插入 `exam_attempts`。
- [ ] 唯一冲突时只查询已有 `in_progress`，不能递增创建新 attempt。
- [ ] 签发不透明 `exam_token`。
- [ ] 只保存 `exam_token_hash`。
- [ ] 设置 `exam_token_expires_at`。

### P6.4 个人试卷快照

- [ ] `manual` 按固化大题题目生成快照。
- [ ] `rule_fixed` 按固化大题题目生成快照。
- [ ] `rule_live` 从 `exam_live_question_pools` 抽题生成快照。
- [ ] 生成 `section_snapshot`。
- [ ] 生成 `question_snapshot`。
- [ ] 生成 `option_snapshot`。
- [ ] 生成 `correct_answer_snapshot`。
- [ ] `sort_order` 是全局题号，从 1 连续递增。
- [ ] 选择题快照保存最终展示选项 ID 顺序。
- [ ] A/B/C/D 只在前端按展示顺序生成。

**验收标准**：

- [ ] 开始考试幂等测试通过。
- [ ] 并发开始考试不会创建语义重复 attempt。
- [ ] `rule_live` 从冻结题池抽题。
- [ ] 试卷快照不受后续题库修改影响。
- [ ] exam token 只能访问当前 attempt。

---

## P7. 答题、自动保存、交卷、防作弊事件

**目标**：完成考试过程核心链路。

**依赖**：P6。

### P7.1 exam token 校验

- [ ] 校验 token hash。
- [ ] 校验 attempt 状态为 `in_progress`。
- [ ] 校验 token 未过期。
- [ ] 校验业务作答截止时间。
- [ ] 业务截止时间为 `min(started_at + duration_minutes, exam.end_time)`。
- [ ] 超过业务截止时间后拒绝自动保存新答案。
- [ ] 允许最多 5 秒网络传输宽限。

### P7.2 自动保存

- [ ] 实现答案 upsert。
- [ ] 唯一键使用 `UNIQUE (tenant_id, attempt_id, attempt_question_id)`。
- [ ] 单选题保存选项 ID。
- [ ] 判断题保存选项 ID。
- [ ] 多选题保存前选项 ID 升序排序。
- [ ] 多选题保存为 JSON 数组字符串。
- [ ] 填空题保存文本。
- [ ] 简答题保存文本。
- [ ] 提交后禁止继续保存。

### P7.3 提交与自动交卷

- [ ] 手动提交使用 `status + version` 条件更新。
- [ ] 自动交卷使用 `status + version` 条件更新。
- [ ] 并发提交更新行数为 0 时按幂等成功返回。
- [ ] 提交后锁定答卷。
- [ ] 提交时触发客观题自动判分。
- [ ] 提交时写关键事件。

### P7.4 防作弊事件

- [ ] 前端 `blur` / `focus` 事件 1 秒内节流。
- [ ] 非关键事件进入带缓冲 channel。
- [ ] 入队成功立即返回。
- [ ] 队列满可丢弃非关键事件。
- [ ] 队列满记录 WARN。
- [ ] `submit` / `auto_submit` 关键事件不能静默丢弃。
- [ ] 启动异步事件消费者。
- [ ] 异步事件消费者支持批量或逐条落库。

**验收标准**：

- [ ] 自动保存 upsert 测试通过。
- [ ] 超过业务截止时间自动保存失败。
- [ ] 手动提交和自动交卷并发幂等。
- [ ] 非关键事件队列满不影响自动保存。
- [ ] 关键事件不静默丢弃。

---

## P8. 判分、阅卷、成绩发布与导出

**目标**：完成自动判分、人工阅卷、成绩发布和导出。

**依赖**：P7。

### P8.1 自动判分

- [ ] 单选题按选项 ID 判分。
- [ ] 判断题按选项 ID 判分。
- [ ] 多选题判分前反序列化为 `[]uint64`。
- [ ] 多选题两侧排序后逐项比较。
- [ ] 禁止多选 JSON 字符串裸比较。
- [ ] 多选题首版全对得分，漏选/错选 0 分。
- [ ] 填空题裁剪首尾空白后完全匹配。
- [ ] 填空题首版仅支持单空。
- [ ] 简答题进入待阅卷。

### P8.2 人工阅卷

- [ ] 待阅卷列表按 attempt 粒度展示。
- [ ] 简答题支持评分。
- [ ] 简答题支持阅卷评语。
- [ ] 写入 `graded_by`。
- [ ] 写入 `graded_at`。
- [ ] 阅卷使用 `version` 乐观锁。
- [ ] 阅卷完成后重算主观题得分。
- [ ] 阅卷完成后重算总分。

### P8.3 成绩发布

- [ ] 纯客观题支持立即出分。
- [ ] 含简答题不允许立即出分。
- [ ] 含简答题不允许 `max_attempts > 1`。
- [ ] 教师阅卷后发布模式支持统一公布时间。
- [ ] 未到公布时间不展示成绩。
- [ ] 到公布时间后展示成绩。
- [ ] `latest` 取最后一次提交成绩。
- [ ] `highest` 取最高分。
- [ ] `show_analysis = true` 且成绩可见后展示解析。
- [ ] `show_analysis = false` 不展示解析。

### P8.4 成绩导出

- [ ] 支持按考试导出成绩。
- [ ] 支持导出考生姓名。
- [ ] 支持导出空间名称。
- [ ] 支持导出 attempt 次数。
- [ ] 支持导出客观题分。
- [ ] 支持导出主观题分。
- [ ] 支持导出总分。
- [ ] 支持导出提交时间。
- [ ] 导出文件写入 `server/data/exports` 或配置目录。

**验收标准**：

- [ ] 多选 `[101,104]` 与 `[101, 104]` 判分一致。
- [ ] 填空单空判分测试通过。
- [ ] 简答题阅卷乐观锁测试通过。
- [ ] 成绩发布时间可见性测试通过。
- [ ] 成绩导出字段完整。

---

## P9. React 管理端与考试端

**目标**：完成可用的 Web 管理端、考试端和阅卷端。

**依赖**：P3-P8 可并行推进页面。

### P9.1 基础前端

- [ ] 配置 React Router。
- [ ] 配置 API client。
- [ ] 配置登录态保存。
- [ ] 配置统一错误提示。
- [ ] 配置分页组件。
- [ ] 配置文件上传组件。

### P9.2 平台管理端

- [ ] 平台管理员登录页。
- [ ] 租户列表。
- [ ] 创建租户表单。
- [ ] 租户 logo 上传。
- [ ] 租户描述编辑。
- [ ] 租户码查看。
- [ ] 租户码重置。
- [ ] 注册开关配置。
- [ ] 平台配置页面。

### P9.3 租户管理端

- [ ] 用户列表。
- [ ] 用户注册/创建。
- [ ] 用户导入。
- [ ] 用户头像上传。
- [ ] 用户禁用提示。
- [ ] 空间列表。
- [ ] 创建空间表单。
- [ ] 空间 logo 上传。
- [ ] 空间描述编辑。
- [ ] 空间成员管理。
- [ ] 空间管理员不变式错误提示。
- [ ] 空间配置页面。

### P9.4 题库与组卷端

- [ ] 题目列表。
- [ ] 在线出题表单。
- [ ] 选择题选项编辑。
- [ ] 题目解析输入。
- [ ] 标签管理。
- [ ] 题目导入。
- [ ] 试卷列表。
- [ ] 大题管理。
- [ ] 手动选题。
- [ ] `rule_fixed` 规则配置。
- [ ] `rule_fixed` 生成试卷。
- [ ] `rule_fixed` 审题和替换。
- [ ] `rule_live` 规则配置。
- [ ] 组卷预检查。

### P9.5 考试端

- [ ] 考试入口。
- [ ] 邀请码进入。
- [ ] 考试说明页。
- [ ] 倒计时。
- [ ] 大题分组展示。
- [ ] 全局题号展示。
- [ ] 单选题作答。
- [ ] 多选题作答。
- [ ] 判断题作答。
- [ ] 填空题作答。
- [ ] 简答题作答。
- [ ] 自动保存状态提示。
- [ ] 切屏事件上报。
- [ ] 提交确认。
- [ ] 成绩可见页。
- [ ] 解析展示控制。

### P9.6 阅卷与成绩端

- [ ] 待阅卷列表。
- [ ] 简答题阅卷页面。
- [ ] 阅卷保存。
- [ ] 阅卷完成。
- [ ] 成绩列表。
- [ ] 成绩发布配置。
- [ ] 成绩导出。

**验收标准**：

- [ ] 管理员可以创建租户、空间、用户。
- [ ] 教师可以完成出题、组卷、发布考试。
- [ ] 考生可以进入考试、答题、自动保存、提交。
- [ ] 教师可以阅卷和发布成绩。
- [ ] 成绩导出可用。

---

## P10. 部署、压测、质量收口

**目标**：完成 Docker Compose、SQLite 单机、100 人在线验证和文档收口。

**依赖**：P1-P9。

### P10.1 Docker Compose

- [ ] 编写 PostgreSQL Compose。
- [ ] 编写 server Compose。
- [ ] 编写 web/Nginx Compose。
- [ ] 配置环境变量注入。
- [ ] 不提交生产密钥。
- [ ] 验证从空库迁移启动。

### P10.2 SQLite 单机

- [ ] 编写 SQLite 单机启动脚本。
- [ ] SQLite DSN 包含 `_foreign_keys=on`。
- [ ] SQLite DSN 包含 `_journal_mode=WAL`。
- [ ] SQLite DSN 包含 `_busy_timeout=5000`。
- [ ] 构建命令带 `json1` 标签。
- [ ] 明确标注 SQLite 不推荐 100 人正式考试。

### P10.3 测试矩阵

- [ ] PostgreSQL 迁移测试。
- [ ] SQLite 迁移测试。
- [ ] MySQL 迁移兼容测试。
- [ ] 权限抽象测试。
- [ ] 租户隔离测试。
- [ ] 空间管理员不变式测试。
- [ ] 题库可见性测试。
- [ ] rule_fixed 生成测试。
- [ ] rule_live 题池冻结测试。
- [ ] 开始考试幂等测试。
- [ ] 自动保存 upsert 测试。
- [ ] 多选判分测试。
- [ ] 业务截止时间测试。
- [ ] 事件异步队列测试。
- [ ] 成绩发布测试。

### P10.4 100 人在线链路验证

- [ ] 使用 PostgreSQL 或 MySQL 环境。
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
- [ ] P8 Review：判分、阅卷、成绩发布和导出可追溯。
- [ ] P9 Review：核心页面完成闭环。
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
- [ ] 多空填空题。
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
- [ ] PostgreSQL / MySQL / SQLite 差异已覆盖。
- [ ] `gorm.io/datatypes` 和 SQLite `json1` 已覆盖。
- [ ] `soft_delete` 与唯一索引已覆盖。
- [ ] 全部核心表已覆盖。
- [ ] 关键唯一约束和复合外键已覆盖。
- [ ] 权限抽象已覆盖。
- [ ] 租户、空间、用户已覆盖。
- [ ] 平台配置和空间配置已覆盖。
- [ ] 题库、标签、选项、导入已覆盖。
- [ ] 大题结构和三种组卷模式已覆盖。
- [ ] `rule_live` 发布态题池冻结已覆盖。
- [ ] 考试发布、范围、邀请码已覆盖。
- [ ] attempt 幂等和 exam token 已覆盖。
- [ ] 业务截止时间硬校验已覆盖。
- [ ] 自动保存 upsert 已覆盖。
- [ ] 多选答案排序和反序列化判分已覆盖。
- [ ] 防作弊事件异步写入已覆盖。
- [ ] 阅卷、成绩发布、导出已覆盖。
- [ ] React 页面闭环已覆盖。
- [ ] 部署、压测、文档收口已覆盖。

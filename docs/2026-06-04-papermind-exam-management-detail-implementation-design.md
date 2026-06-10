# PaperMind 考试管理详情页真实接口化技术方案

## 1. 背景与目标

当前考试管理详情页位于 `/papers/:paperID/preview`，视觉上已经接近“考试详情”
管理台，包含考试概览、基本信息、试卷预览、考生管理、考试监控、成绩管理、
考试设置和操作日志等 tab。

本期不实现考试监控真实接口和数据聚合。`考试监控` tab 只保留入口和占位效果，
用于维持当前页面信息架构；监控事件聚合、异常统计和事件列表放到后续迭代。

现状问题是页面仍以 `paper_id` 为入口，并且多个 tab 使用前端静态数据。后端现有
考试、作答、阅卷和成绩接口基本以 `exam_id` 为核心。由于同一张试卷可以发布成多场
考试，继续用 `paper_id` 承载考试详情会导致数据歧义、权限判断不完整和后续接口对接困难。

本方案目标：

- 最终页面效果与当前实现差异极小，继续沿用现有 tab、卡片、表格、按钮和控件样式。
- 页面数据改为真实后端接口，不保留核心业务 mock。
- 后端以 `exam_id` 为考试详情主键，补齐管理端聚合查询能力。
- 新建考试时补齐考生范围入口，发布流程支持一个或多个空间/用户目标。
- 数据库尽量复用现有考试主链路，只新增必要表和索引。
- 本期范围不包含考试监控接口、监控事件索引和监控 tab 真实数据对接。
- 明确性能、安全、权限和端到端联调方案。

## 2. 总体架构

采用“考试详情聚合接口 + 前端最小替换”的方案。

```text
React 考试详情页
  → web/src/api/examDetail.ts
  → /api/v1/exams/:id/** 管理端详情接口
  → ExamManagementDetailService 聚合服务
  → exam / paper / user / space / result / operation-log repository
  → PostgreSQL / MySQL 8.0+ / SQLite
```

关键原则：

- `exam_id` 是考试详情页唯一主键。
- `/papers/:paperID/preview` 只作为兼容入口或跳转入口，不能用 `paper_id` 推导唯一考试。
- 详情页按 tab 懒加载，不在首屏一次性拉取考生、成绩和日志全量数据。
- API 层只解析 HTTP 入参和 session；权限、聚合和事务边界在 service 层完成。
- Repository 负责数据库查询和聚合 SQL；service 不直接写 SQL。

## 3. 数据库调整

### 3.1 复用现有表

优先复用现有考试链路表：

- `exams`：考试基础信息、时间、成绩发布配置、状态。
- `exam_targets`：考试投放空间或用户。
- `exam_target_scope_spaces`：考试投放目标的作用空间，尤其用于用户直投目标的空间归属。
- `exam_live_question_pools`：`rule_live` 发布态冻结题池。
- `exam_attempts`：考生作答状态、开始/提交时间、成绩。
- `exam_attempt_questions`：考生题目快照。
- `exam_answers`：答案和阅卷状态。
- `exam_events`：考试过程事件，本期只保留既有写入链路，不做管理端监控读取。
- `papers`、`paper_sections`、`paper_section_questions`、`paper_section_rules`：试卷结构和组卷信息。
- `users`、`tenant_user_memberships`、`space_members`、`spaces`：考生、教师和空间权限范围。

### 3.2 新增管理扩展表

新增 `exam_operation_logs`，用于操作日志 tab，不复用 `exam_events`。

`exam_events` 是考生考试过程事件，例如切屏、自动保存、提交。`exam_operation_logs`
是管理端操作审计，例如发布考试、发送邀请码、导入考生、导出成绩、发布成绩、
修改设置和阅卷。两者语义不同，不能混用。

```text
exam_operation_logs
├── id                    # 管理端操作日志主键 ID
├── tenant_id             # 所属租户 ID
├── exam_id               # 考试 ID
├── operation_type        # 操作类型
├── operation_title       # 操作标题
├── operation_detail      # 操作详情摘要
├── actor_id              # 操作人用户 ID
├── actor_type            # 操作人主体类型：tenant_user / system
├── actor_role            # 操作时的租户级角色快照
├── space_id              # 操作关联空间，租户级操作可为空
├── operation_group_id    # 操作组 ID，必须显式列化，不能继续依赖 ext_json
├── created_at            # 创建时间
├── created_by            # 创建人用户 ID
├── created_by_type       # 创建人主体类型
└── ext_json              # JSON 扩展字段
```

操作类型首版枚举：

- `publish_exam`
- `send_invite`
- `import_candidates`
- `export_results`
- `publish_results`
- `update_settings`
- `grade_answer`
- `system_event`

表规则：

- 追加写日志表，不使用 `updated_at`、`updated_by`、`updated_by_type` 和 `version`。
- 写入日志不能依赖前端传入的操作者字段，必须从 session principal 派生。
- 关键写操作的日志建议同事务写入；日志写入失败时关键操作回滚，避免审计缺失。
- 异步补充类日志写入失败时不回滚主业务，但必须记录 ERROR 日志。
- 一次操作影响多个空间时，按受影响空间写多条日志，并用同一个
  `operation_group_id` 聚合；租户管理员页面可按操作组聚合展示，空间管理员和教师按
  `space_id` 精确过滤。早期实现曾把 `operation_group_id` 放在
  `ext_json.operation_group_id`，后续实现和已有本地数据修复必须以显式列为准，不能继续
  依赖 `ext_json` 做聚合、去重、分页或权限裁剪。
- 租户级操作 `space_id` 为空；用户直投操作按目标用户当前启用空间展开日志可见范围。

新增 `exam_target_scope_spaces`，用于把 `exam_targets` 的每条发布目标映射到真实作用空间：

```text
exam_target_scope_spaces
├── id                    # 考试发布目标作用空间主键 ID
├── tenant_id             # 所属租户 ID
├── exam_id               # 考试 ID
├── exam_target_id        # exam_targets.id
├── space_id              # 目标作用空间 ID
├── created_at            # 创建时间
├── created_by            # 创建人用户 ID
├── created_by_type       # 创建人主体类型
└── ext_json              # JSON 扩展字段
```

表规则：

- 空间投放目标写入自身空间 ID，用户直投目标写入发布时裁剪出的空间 ID 集合。
- 查询候选人、成绩、阅卷、导出和操作日志时，以该表作为用户直投目标的空间归属来源。
- 迁移会从既有 `exam_targets.ext_json.space_ids` 回填映射行；没有历史作用域数据的用户直投目标继续按当前有效成员空间回退，避免老数据失效。
- 新写入路径不再把 `space_ids` 放进 `exam_targets.ext_json`，`ext_json` 只保留非主流程扩展信息。

### 3.3 考生名单策略

首版建议不新增 `exam_candidates`，先复用 `exam_targets`：

- 空间投放：`exam_targets(target_type = 'space')`，应考考生来自空间内启用 student 成员。
- 用户直投：`exam_targets(target_type = 'user')`，应考考生来自指定用户；空间归属来自
  `exam_target_scope_spaces`。
- 新建考试/发布考试：前端按“班级范围 / 指定人群”二选一展示。班级范围写入一个
  `exam_targets(target_type = 'space')`；指定人群写入一批
  `exam_targets(target_type = 'user')`。后端继续批量校验后写入多条 `exam_targets` 和对应
  `exam_target_scope_spaces`。
- 批量导入考生：校验用户属于当前租户后，继续批量写入
  `exam_targets(target_type = 'user')` 和对应作用空间，不另建候选人副本。
- API 层如收到空间和用户混合目标，考生列表仍按 `user_id` 去重；如果同一用户同时来自
  空间投放和用户直投，列表保留一行，并在 `source_targets` 中返回来源摘要。当前前端入口
  不提供空间和用户混选。

只有当后续明确要求“临时导入考生但不加入空间/用户直投目标”时，再新增
`exam_candidates`。这样可以避免首版同时维护两套考试目标来源。

考生状态和成绩口径：

- 应考名单由 `exam_targets` 动态展开，用户必须仍是当前租户启用成员。
- 空间投放只展开启用空间内的启用 student 成员；空间被禁用、用户离开空间或用户被禁用后，
  均不再计入应考名单、作答进度和成绩统计。
- 多次作答时，列表返回 `attempt_count`、`current_attempt_id` 和 `result_attempt_id`。
- 进行中状态优先展示未提交的最新 attempt。
- 最终成绩按 `exams.result_strategy` 选择结果 attempt，例如 `latest` 取最后一次提交。
- 没有作答记录的考生状态为 `not_started`，不能通过缺少 attempt 误判为无权限。

### 3.4 新增索引

为概览、考生管理、成绩管理和操作日志补充组合索引：

```sql
-- 作答状态、提交时间、成绩列表和进度聚合。
INDEX idx_exam_attempts_exam_status_submitted
  ON exam_attempts (tenant_id, exam_id, status, submitted_at);

-- 单个考生作答查询、考生管理列表关联。
INDEX idx_exam_attempts_exam_user
  ON exam_attempts (tenant_id, exam_id, user_id);

-- 详情聚合按考试获取 attempt ID 列表。
INDEX idx_exam_attempts_exam_id
  ON exam_attempts (tenant_id, exam_id, id);

-- 成绩列表和排名稳定排序。
INDEX idx_exam_attempts_exam_score_rank
  ON exam_attempts (tenant_id, exam_id, total_score, submitted_at, id);

-- 待阅卷数量和主观题状态聚合。
INDEX idx_exam_answers_attempt_grading
  ON exam_answers (tenant_id, attempt_id, grading_status);

-- 操作日志按考试时间倒序分页。
INDEX idx_exam_operation_logs_exam_time
  ON exam_operation_logs (tenant_id, exam_id, created_at, id);

-- 操作日志按操作组聚合。
INDEX idx_exam_operation_logs_exam_group
  ON exam_operation_logs (tenant_id, exam_id, operation_group_id);
```

当前仍处于初始化建库阶段，考试管理详情结构直接合并进三套数据库基准脚本：

- `server/data/migrations/postgres`
- `server/data/migrations/mysql`
- `server/data/migrations/sqlite`

迁移要求：

- 每个数据库目录只保留 `001_tenant_space.sql` 作为当前基准结构。
- `002_exam_management_detail.sql` / `003_exam_target_scope_spaces.sql` 不再保留；其目标表结构和索引已合并进 `001_tenant_space.sql`。
- 基准 SQL 只表达最终表结构和索引，不包含历史回填、防重复执行或方言动态 DDL。
- 后续项目进入生产或存在历史库后，新增字段、索引和回填逻辑必须重新按递增版本追加迁移。
- SQLite 继续要求 `_foreign_keys=on`，涉及 JSON 查询的测试继续带 `-tags json1`。
- 新增运行时迁移后必须补齐三套数据库的表、索引和 DAO 映射测试，不能只验证 SQLite。

## 4. 后端接口设计

### 4.1 服务边界

新增 `ExamManagementDetailService`，职责是管理端考试详情聚合：

- 读取考试标题区和基础信息。
- 聚合考试概览。
- 展开考试目标范围和考生列表。
- 聚合成绩摘要和成绩列表。
- 读取答卷详情。
- 写入操作日志。
- 统一执行考试详情页权限裁剪。

该 service 不负责：

- 发布考试主流程。
- 考生开考、自动保存、提交。
- 自动判分和人工阅卷主逻辑。
- 成绩发布配置的核心状态变更。
- 考试监控事件聚合和异常统计，本期不做。

这些仍复用现有 `exam`、`taking`、`review`、`result/export` 服务。

操作日志写入不能只放在 `ExamManagementDetailService`。需要新增轻量
`OperationLogWriter`，由发布考试、导入考生、重发邀请码、导出成绩、发布成绩、
更新设置和人工阅卷等写路径显式调用。

日志一致性规则：

- 发布考试、导入考生、发布成绩、更新设置、人工阅卷：日志与主业务同事务写入，
  日志失败则回滚，避免关键审计缺失。
- 导出成绩：CSV 文件生成成功后必须继续写入 `export_results` 日志；日志失败时接口返回失败，
  避免敏感导出行为缺少审计记录。
- 重发邀请码：当前没有外部通知通道，业务校验和日志写入保持同事务语义；后续接入外部通知时，
  需要重新明确通知副作用和审计失败的处理策略。
- 系统自动事件可以异步补写，但必须能通过后台日志定位失败原因。

### 4.2 API 清单

接口方法规范：

- 本方案只允许使用 `GET` 和 `POST`。
- 查询类接口统一使用 `GET`。
- 创建、导入、重发、更新配置、发布结果和其它写操作统一使用 `POST`。
- 不使用 `PUT`、`PATCH`、`DELETE` 或其它 HTTP 方法，避免前后端网关、权限中间件和测试约定分散。

#### 新建考试和发布范围

当前已完成链路只支持单个 `target_type + target_id`。本方案需要把发布请求升级为多目标，
同时保留单目标字段作为兼容输入。

```text
POST /api/v1/exams
body: tenant_id, paper_id, name, start_time, end_time, duration_minutes,
      max_attempts, result_strategy, publish_mode, score_publish_time?,
      targets?: [{ target_type, target_id }],
      target_type?, target_id?
```

规则：

- `targets` 优先级高于兼容字段 `target_type + target_id`。
- `targets` 不能为空，且同一请求内必须按 `target_type + target_id` 去重。
- 后端在事务前批量校验目标存在、租户归属、空间管理权限和用户启用状态。
- 发布事务内创建 `exams`、`exam_live_question_pools` 和多条 `exam_targets`。
- 如果任一目标无权限或不存在，整场考试发布失败，不创建部分目标考试。
- 前端新建考试弹窗增加“发布范围”入口，使用“班级范围 / 指定人群”选项加下拉选择；
  班级范围圈定当前空间或选中空间下全部启用学生，指定人群通过同学下拉面板选择一批学生。
- 响应返回兼容字段 `target_type` / `target_id` 和目标数组 `targets: [{ target_type, target_id }]`，
  用于考试列表和详情页展示发布范围。

#### 考试详情首屏

```text
GET /api/v1/exams/:id/detail
query: tenant_id, space_id?
```

返回内容：

- `exam`：考试 ID、关联试卷、考试名称、考试时间、作答时长、成绩策略、发布模式、状态和兼容目标字段。
- `targets`：发布目标数组，字段为 `target_type`、`target_id`。
- `target_space_ids`：考试目标覆盖到的有效空间 ID 集合。
- `allowed_space_ids`：当前管理者在该考试下可查看或管理的空间 ID 集合。
- `permissions`：`can_view_detail`、`can_view_overview`、`can_view_paper`、
  `can_view_candidates`、`can_manage_candidates`、`can_view_results`、
  `can_export_results`、`can_publish_results`、`can_update_settings`、`can_view_logs`。

#### 考试概览

```text
GET /api/v1/exams/:id/overview
query: tenant_id, space_id?
```

返回内容：

- 计划考生、已参加、已交卷、进行中、考试时长、满分/及格。
- 考试进度：未开始、进行中、已完成。
- 题型分布：题型、题量、占比、分值。
- 考试安排：时间、适用对象、试卷版本、计分方式。
- 风险提醒：未开考、网络波动、待阅卷、流程提醒。
- 近期动态：发布考试、发送邀请码、开始作答、自动保存等。

#### 试卷预览

```text
GET /api/v1/exams/:id/paper-preview
query: tenant_id, space_id?, filter?, page?, page_size?
```

返回内容：

- 试卷统计卡片。
- 试卷结构。
- 分页题目列表。

规则：

- `manual` / `rule_fixed` 读取固化试卷结构。
- `rule_live` 读取发布时冻结候选题池或考试快照，不重新查询动态题库。
- 不返回不该暴露给普通管理视图的正确答案；答卷详情和阅卷页面按权限单独控制。

#### 考生管理

```text
GET /api/v1/exams/:id/candidates
query: tenant_id, space_id?, status?, keyword?, page?, page_size?
```

返回内容：

- 统计：应考人数、未开始、进行中、已交卷。
- 列表：姓名、学号或登录名、班级/空间、考试状态、开始时间、提交时间、用时、成绩、可用操作。

写操作：

```text
POST /api/v1/exams/:id/candidates/import
body(JSON): tenant_id, space_id?, user_ids[]

POST /api/v1/exams/:id/invitations/resend
body(JSON): tenant_id, space_id?, user_ids[]
```

规则：

- `candidates/import` 本质是追加用户直投目标，会改变应考范围；考试开始后默认禁止导入，
  避免破坏考试公平性。
- 若未来需要开考后补考生，必须单独设计补考授权、操作日志和通知规则，不能复用首版导入接口。
- `invitations/resend` 只能对当前授权范围内、仍在应考名单中的用户操作。
- 当前版本没有短信或邮件通知通道；重发接口的后端语义是校验可重发目标、返回考试邀请码，
  并写入 `send_invite` 审计日志，前端后续负责提示或复制邀请码。

#### 考试监控

本期不实现真实接口，不新增 `/api/v1/exams/:id/monitor`。

前端处理：

- 保留 `考试监控` tab。
- 点击后展示占位内容，例如“考试监控将在后续版本开放”。
- 不请求后端监控接口，不保留静态 mock 监控数据。

后续迭代再补充：

- `/api/v1/exams/:id/monitor`。
- `exam_events` 管理端白名单 DTO。
- 按考试聚合事件的索引或 `exam_events.exam_id` 冗余字段。
- 异常事件统计和分页事件列表。

#### 成绩管理

```text
GET /api/v1/exams/:id/results/summary
query: tenant_id, space_id?

GET /api/v1/exams/:id/results
query: tenant_id, space_id?, status?, keyword?, page?, page_size?
```

返回内容：

- 统计卡片：已交卷、平均分、最高分、及格率、待阅主观题。
- 图表：分数段分布、题型得分率。
- 列表：排名、姓名、学号或登录名、班级/空间、客观题、主观题、总分、成绩状态、操作。

规则：

- 排名基于全量授权范围稳定排序：`total_score DESC, submitted_at ASC, id ASC`。
- 不能用当前页数组下标生成排名。
- 教师首版不能导出成绩；前端不展示入口，后端仍必须拒绝。

答卷详情：

```text
GET /api/v1/exams/:examID/attempts/:attemptID/answer-sheet
query: tenant_id, space_id?
```

答卷详情按 `examID + attemptID` 双重定位，避免仅凭 attempt ID 访问到不属于当前考试的
答卷。返回字段按权限裁剪：阅卷和成绩管理可返回标准答案、学生答案、得分和阅卷信息；
普通管理预览不返回正确答案。

#### 考试设置

```text
POST /api/v1/exams/:id/settings
body: tenant_id, space_id?, publish_mode?, score_publish_time?
```

首版只允许修改成绩发布相关配置。如果现有成绩发布配置接口已经覆盖该能力，
前端优先复用现有接口；`POST /settings` 只能作为聚合详情页的兼容别名或后续替代入口，
不能长期维护两套语义不同的配置接口。

已经开考后的考试不得修改：

- 关联试卷。
- 发布范围。
- 考试时长。
- 题目顺序和选项随机规则。
- 会影响公平性的作答配置。

#### 操作日志

```text
GET /api/v1/exams/:id/logs
query: tenant_id, space_id?, operation_type?, page?, page_size?
```

返回 `exam_operation_logs`，按 `created_at DESC, id DESC` 排序。
响应行额外返回 `operation_group_id`，用于租户管理员按同一次多空间操作聚合展示；
不直接透出完整 `ext_json`，避免内部审计扩展字段成为前端长期契约。

## 5. 前端对接方案

### 5.1 路由

新增考试详情路由：

```text
/exams/:examID
```

兼容路由：

```text
/exams/:examID/preview
```

`/exams/:examID` 是唯一 canonical 路由，`/exams/:examID/preview` 只做重定向或兼容，
避免前端测试、菜单跳转和浏览器刷新出现两套路由状态。

当前 `/papers/:paperID/preview` 作为兼容入口：

- 如果后端能确认该 paper 只对应一场考试，可以跳转到考试详情路由。
- 如果同一 paper 对应多场考试，展示考试选择或返回考试列表。
- 不再让真实考试详情长期依赖 `paper_id`。

### 5.2 API client

新增：

```text
web/src/api/examDetail.ts
```

按 tab 定义 DTO：

- `ExamDetailHeader`
- `ExamOverviewData`
- `ExamPaperPreviewData`
- `ExamCandidatePage`
- `ExamResultsSummary`
- `ExamResultsPage`
- `ExamSettingsData`
- `ExamOperationLogPage`
- `ExamPublishTargetOption`
- `ExamPublishTargetSummary`

### 5.3 页面改造策略

保持当前视觉差异极小：

- 保留现有 tab 文案和顺序。
- `考试监控` tab 本期保留占位，不接真实 API。
- 保留当前卡片、表格、按钮、状态标签、筛选框和分页控件样式。
- 保留 `role="tablist"`、`role="tab"`、`role="tabpanel"` 的 tab 语义。
- 将静态 `examOverviewStats`、`examCandidateRows`、`examResultRows` 替换为接口数据。
- 各 tab 首次点击时加载数据，并在当前页面生命周期内缓存。
- 页面刷新后重新拉取接口，不回退 mock。
- 新建考试弹窗沿用现有控件风格，增加考生范围多选，不新增独立大页面。

交互规则：

- loading 使用现有 Panel 内加载态。
- empty 使用现有空状态样式。
- 401 清空登录态并跳转登录。
- 403 用 toast 提示无权限，同时禁用对应操作。
- 404 提示考试不存在或已删除。
- 顶部按钮由 `detail.permissions` 控制展示和禁用。
- 考生范围为空、目标重复、无权限目标和禁用用户均由后端返回明确错误码，
  前端通过 toast 展示，不用页面内静默失败。

### 5.4 保持当前 UI 的具体要求

- 左侧菜单归属“考试”，不是“试卷”。
- 顶部标题、状态 badge、操作按钮位置保持当前实现。
- tab 下划线、激活色、间距和现有样式一致。
- 表格在窄视口保持内部横向滚动，页面整体不产生横向滚动。
- 不新增大面积视觉重构，不引入新的设计系统。

## 6. 权限与安全

### 6.1 角色权限

- `tenant_admin`：可查看和管理本租户所有考试详情。
- `space_admin`：只能查看和管理投放到自己启用管理空间的考试，以及该空间内考生、成绩和日志。
- `teacher`：只能查看自己启用教师空间范围内的考试业务数据；首版不允许导出成绩，也不允许跨空间导入考生。
- `student`：不能访问管理端考试详情页，只能访问考试入口和本人可见成绩。

### 6.2 后端授权规则

- 所有接口必须从 session 重建权限上下文。
- `tenant_id`、`space_id`、`actor_id`、`actor_role` 不能作为授权事实。
- 后端必须从 `exam_targets`、`space_members`、`exam_attempts` 和目标用户真实空间关系推导资源范围。
- 空成绩列表对有权限用户返回空集合，不能误报无权限。
- 用户直投考试需要展开目标用户当前启用空间成员关系，供对应空间教师或空间管理员处理成绩。
- 前端按钮禁用只是体验优化，后端必须做最终权限拒绝。
- 发布多目标考试时，后端必须逐个目标授权。`space_admin` 只能选择自己管理的空间和该空间内启用学生；
  `teacher` 首版只能选择自己任教空间内启用学生或所在空间，不能创建租户全局考试。

### 6.3 数据安全

- 管理端试卷预览默认不返回正确答案。
- 答卷详情按管理端权限单独校验。
- 导出文件只返回受鉴权保护的下载 URL，不暴露服务器本地路径。
- 操作日志不能信任前端传入操作者身份。
- exam token 只能用于考试入口接口，不能访问管理端详情接口。
- 后续实现考试监控时，`exam_events.payload` 必须只返回白名单字段，避免把任意 JSON 当 UI 内容渲染。

## 7. 性能方案

- tab 懒加载，首屏只拉标题区和基础信息。
- 概览和成绩摘要由后端聚合，前端不拉全量计算。
- 考生、成绩和日志全部分页，默认 `page_size = 20`，最大 `100`。
- 成绩排名由数据库或后端在全量授权范围内稳定排序。
- 组合索引覆盖 `exam_id + status + submitted_at`、`exam_id + user_id`、成绩排名和日志分页。
- 单场考试超过 1 万考生时，再考虑新增 `exam_stat_snapshots` 异步汇总表；首版不提前引入。
- 考生名单展开必须分页后再补充 attempt 状态，不能先把全量空间成员加载到前端。
- 空间目标展开建议在后端用授权空间和 keyword 过滤后分页，避免大空间用户列表造成内存峰值。

## 8. 端到端联调方案

### 8.1 测试数据

准备一场已发布考试，覆盖：

- 两个空间。
- 未开始考生。
- 进行中考生。
- 已交卷考生。
- 待阅卷主观题。
- 已发布成绩。
- 至少一条管理端操作日志。
- 一场同时投放到空间和用户的考试，用于验证考生去重和多目标权限。

### 8.2 后端验证

在 `server/` 目录执行：

```bash
go test -tags json1 ./internal/dao/db ./internal/service/exam ./api/v1
```

如涉及迁移或权限公共逻辑，扩大到：

```bash
go test -tags json1 ./...
```

重点断言：

- `tenant_admin` 全量可见。
- `space_admin` / `teacher` 只能看到授权空间范围。
- `student` 访问管理端详情接口返回 403。
- 没有成绩时返回空集合而不是 403。
- 考生管理筛选不跨空间泄露。
- 操作日志按考试和授权空间过滤。
- 新建考试多目标发布要验证全部成功、部分目标无权限整体失败和重复目标去重。
- 三套数据库迁移必须分别通过迁移加载或 DAO 初始化验证，至少覆盖
  `exam_operation_logs` 表结构和新增索引。
- 后续 `operation_group_id` 显式列迁移以
  `docs/2026-06-10-papermind-grading-module-technical-design.md` 和三套
  `001_tenant_space.sql` 为准；`ext_json.operation_group_id` 是旧实现，不能继续作为聚合依据。

### 8.3 前端验证

在 `web/` 目录执行：

```bash
npm test -- --no-file-parallelism src/pages/Exam/PaperPreviewPage.test.tsx
npm run build
```

后续路由改名后，测试文件同步迁移到新的考试详情页测试。

### 8.4 浏览器联调

使用真实浏览器逐 tab 验收：

1. 登录 `tenant_admin`，确认所有 tab 有数据，刷新后不回退 mock。
2. 登录 `space_admin`，确认只能看到授权空间考生、成绩和日志。
3. 登录 `teacher`，确认能查看授权空间成绩但不能导出成绩。
4. 登录 `student`，直接访问考试详情路由返回无权限或被路由守卫拦截。
5. 切换 `考试概览`、`基本信息`、`试卷预览`、`考生管理`、`考试监控`、`成绩管理`、`考试设置`、`操作日志`。
6. 确认 `考试监控` tab 只展示占位，不发起监控接口请求。
7. 模拟接口 401、403、404，确认 toast 和路由行为正确。
8. 验证窄视口：页面整体不产生横向滚动，表格区域内部滚动。

## 9. 可执行任务清单

### 阶段 0：范围冻结与接口契约确认

目标：先冻结本期边界，避免实现中途扩大到考试监控或非 `GET` / `POST` 方法。

交付物：

- 最终 API 清单。
- 权限矩阵。
- 前后端 DTO 草案。

任务：

- [x] 确认本期只做考试概览、基本信息、试卷预览、考生管理、成绩管理、考试设置和操作日志。
- [x] 确认 `考试监控` tab 只做占位，不请求后端接口。
- [x] 确认所有接口只使用 `GET` 和 `POST`。
- [x] 确认新建/发布考试使用 `targets[]`，并保留 `target_type + target_id` 兼容输入。
- [x] 确认 `tenant_admin`、`space_admin`、`teacher`、`student` 的可见范围和操作权限。

验证：

- [x] API 清单中不存在 `PUT`、`PATCH`、`DELETE` 接口。
- [x] 本期 API 清单中不存在 `/api/v1/exams/:id/monitor`。
- [x] 每个 tab 都有明确数据来源或占位规则。

退出条件：

- [x] API、权限和本期范围无争议后，再进入数据库和后端实现。

### 阶段 1：数据库迁移、DO 和 Repository 基础能力

目标：先补齐操作日志和查询性能基础，不碰前端页面。

交付物：

- 三套数据库基准 SQL：`server/data/migrations/{postgres,mysql,sqlite}/001_tenant_space.sql`。
- `ExamOperationLogDO` 和列映射。
- 操作日志 repository。
- 新增索引。
- `exam_target_scope_spaces` 映射表。

任务：

- [x] 新增 `exam_operation_logs` 表，覆盖 PostgreSQL、MySQL、SQLite。
- [ ] 补充 `exam_operation_logs.operation_group_id` 显式列，替代旧版
  `ext_json.operation_group_id` 聚合设计。
- [x] 新增 `exam_target_scope_spaces` 表，覆盖 PostgreSQL、MySQL、SQLite。
- [x] 新增 `idx_exam_attempts_exam_status_submitted`。
- [x] 新增 `idx_exam_attempts_exam_user`。
- [x] 新增 `idx_exam_attempts_exam_id`。
- [x] 新增 `idx_exam_attempts_exam_score_rank`。
- [x] 新增 `idx_exam_answers_attempt_grading`。
- [x] 新增 `idx_exam_operation_logs_exam_time`。
- [x] 新增 `ExamOperationLogDO`、columns 映射和 DAO 测试。
- [x] 新增操作日志写入、分页查询和按空间过滤 repository 方法。
- [x] 不新增考试监控索引，不改 `exam_events` 结构。

验证：

- [x] `go test -tags json1 ./internal/dao/db`
- [x] SQLite 迁移可通过实际加载测试，PostgreSQL/MySQL 8.0+ 迁移通过静态结构检查。
- [x] 操作日志按 `created_at DESC, id DESC` 稳定分页。
- [x] 多空间操作可用 `operation_group_id` 聚合，空间过滤不串数据。
- [ ] 操作日志聚合、去重和分页只读取 `operation_group_id` 显式列，不读取
  `ext_json.operation_group_id`。

退出条件：

- [x] 数据库结构、DO、repository 和迁移测试通过。

### 阶段 2：新建/发布考试多目标能力

目标：补齐新建考试时指定考生范围的真实入口。

交付物：

- 多目标发布 request/DTO。
- 批量目标授权校验。
- 多目标事务写入。
- 新建考试前端考生范围入口。

任务：

- [x] 后端 request 支持 `targets[]`。
- [x] 保留 `target_type + target_id` 兼容输入。
- [x] service 增加批量目标去重和校验。
- [x] repository 增加 `CreatePublishedExamWithTargets`。
- [x] 发布事务内创建 `exams`、`exam_live_question_pools`、多条 `exam_targets` 和目标作用空间。
- [x] 任一目标无权限、禁用或不存在时整场发布失败。
- [x] 前端新建考试弹窗增加空间/用户多选控件。
- [x] 前端错误统一走 toast，不用页面内静默失败。

验证：

- [x] `tenant_admin` 可选择多个空间和用户。
- [x] `space_admin` 只能选择自己管理空间和空间内启用学生。
- [x] `teacher` 不能创建租户全局考试。
- [x] 重复目标去重。
- [x] 部分目标无权限时不创建考试。
- [x] 旧单目标请求仍可兼容。

退出条件：

- [x] 新建考试能产生真实 `exam_targets` 和 `exam_target_scope_spaces`，考生管理后续有名单来源。

### 阶段 3：考试详情聚合 service 与权限裁剪

目标：建立详情页统一读取入口，避免各 tab 自行拼权限。

交付物：

- [x] `ManagementDetailService`。
- [x] 管理端权限上下文。
- [x] 目标展开和授权范围工具方法。

任务：

- [x] 新增详情 service，聚合考试、试卷组卷模式、投放目标和授权空间范围；用户与作答大列表在阶段 6 / 7 通过同一权限上下文查询，避免详情入口提前拉全量数据。
- [x] 实现 `tenant_admin` 全量范围。
- [x] 实现 `space_admin` 授权空间范围。
- [x] 实现 `teacher` 启用教师空间范围。
- [x] 拒绝 `student` 访问管理端详情。
- [x] 统一返回 `permissions`，供前端控制按钮展示和禁用。
- [x] 用户直投考试按目标用户当前启用空间关系裁剪可见范围。
- [x] 空集合场景返回空列表，不误报 403。

验证：

- [x] `go test -tags json1 ./internal/service/exam`
- [x] `go test -tags json1 ./internal/dao/db`
- [x] 直接访问无权限考试返回 403。
- [x] 有权限但无成绩返回空集合。
- [x] 用户直投不向无关空间泄露。

退出条件：

- [x] 后续 detail、overview、candidate、results、logs 接口都复用同一权限裁剪。

### 阶段 4：基础信息、考试概览、试卷预览接口

目标：先完成页面上半部分和试卷预览真实数据。

交付物：

- [x] `GET /api/v1/exams/:id/detail`
- [x] `GET /api/v1/exams/:id/overview`
- [x] `GET /api/v1/exams/:id/paper-preview`
- [x] detail API handler 和 DTO。
- [x] overview / paper-preview API handler 和 DTO。

任务：

- [x] 实现详情首屏接口，返回标题区、基础信息和权限。
- [x] 实现考试概览接口，返回计划考生、已参加、已交卷、进行中、题型分布和近期动态。
- [x] 实现试卷预览接口，支持题型筛选和分页。
- [x] `manual` / `rule_fixed` 读取固化试卷结构。
- [x] `rule_live` 读取发布时冻结题池，不重新抽题。
- [x] 管理端普通试卷预览不返回正确答案。
- [x] 所有接口只使用 `GET`。

验证：

- [x] `go test -tags json1 ./api/v1 ./internal/service/exam ./internal/dao/db`
- [x] 404 考试不存在返回明确错误。
- [x] 403 无权限不泄露考试标题。
- [x] 试卷预览分页稳定。
- [x] `rule_live` 不重新查询动态题库抽题。
- [x] 禁用空间不再计入应考人数、已参加、进行中和已交卷统计。
- [x] 考试概览近期动态按授权空间过滤，非租户管理员不读取整场日志。

退出条件：

- [x] 前端可用真实接口渲染标题区、基本信息、考试概览和试卷预览。

### 阶段 5：前端路由和基础 tab 接入

目标：把现有预览页从静态 mock 切到真实考试详情入口，同时保持 UI 差异极小。

交付物：

- `/exams/:examID` canonical 路由。
- `/exams/:examID/preview` 兼容路由。
- `/papers/:paperID/preview` 兼容跳转策略。
- `web/src/api/examDetail.ts`。
- 标题区、基本信息、考试概览、试卷预览真实数据接入。

任务：

- [x] 新增 `examDetail.ts`，定义 detail、overview、paper-preview API client。
- [x] 新增考试详情 canonical 路由。
- [x] 保留旧 `/papers/:paperID/preview` 入口，无法唯一定位考试时返回考试列表或选择页。
- [x] 左侧菜单保持选中“考试”。
- [x] tab 保留现有文案、顺序和样式。
- [x] 401 清登录态并跳转登录。
- [x] 403 使用 toast 提示无权限。
- [x] 404 提示考试不存在或已删除。
- [x] `考试监控` tab 展示占位，不请求接口。

验证：

- [x] `npm test -- --no-file-parallelism src/pages/Exam/PaperPreviewPage.test.tsx`
- [x] `npm test -- --no-file-parallelism src/pages/Exam/PaperPreviewRoute.test.tsx`
- [x] `npm run build`
- [x] 浏览器刷新后不回退 mock。
- [x] 窄视口页面整体不横向滚动。
- [x] `考试监控` tab 无网络请求。
- [x] `/papers/101/preview` 903px 视口 DOM 量测通过：
  `documentScrollWidth = viewportWidth = 903`，统计卡最大右边界 `838px`，右侧指标未被裁切。
- [x] 浏览器验证旧入口兼容策略：`/papers/101/preview` 无法唯一定位时跳转 `/exams`；
  `/papers/100/preview` 可唯一定位时跳转 `/exams/1`，且均未回退旧 paper mock 预览。

退出条件：

- [x] 首屏和试卷预览在真实接口下可用，视觉与当前实现差异极小。

### 阶段 6：考生管理真实接口和前端接入

目标：完成应考名单、状态、邀请和导入链路。

交付物：

- `GET /api/v1/exams/:id/candidates`
- `POST /api/v1/exams/:id/candidates/import`
- `POST /api/v1/exams/:id/invitations/resend`
- 考生管理 tab 真实数据。

任务：

- [x] 动态展开 `exam_targets` 形成应考名单。
- [x] 空间和用户混选按 `user_id` 去重。
- [x] 返回 `source_targets`、`attempt_count`、`current_attempt_id`、`result_attempt_id`。
- [x] 多次作答按 `result_strategy` 选择最终成绩。
- [x] 导入考生写入用户直投目标。
- [x] 考试开始后禁止导入考生。
- [x] 重发邀请只允许当前授权范围内应考用户。
- [x] 导入考生写入 `import_candidates` 操作日志。
- [x] 重发邀请写入 `send_invite` 操作日志。
- [x] 前端接入考生列表读取、状态筛选、搜索和分页。
- [x] 前端接入导入考生和重发邀请。
- [x] `GET /api/v1/exams/:id/candidates` 接入 handler、service 和 repository。
- [x] `POST /api/v1/exams/:id/candidates/import` 接入 handler、service 和 repository。
- [x] `POST /api/v1/exams/:id/invitations/resend` 接入 handler、service 和 repository。

验证：

- [x] 未开始、进行中、已交卷状态准确。
- [x] 空间 + 用户混选不重复显示。
- [x] 禁用用户不进入应考名单。
- [x] `go test -tags json1 ./internal/dao/db -run 'TestExamRepositoryListExamCandidates'`
- [x] `go test -tags json1 ./internal/dao/db -run TestExamRepositoryListExamCandidatesReturnsNotStartedInProgressAndSubmittedStatuses`
- [x] `go test -tags json1 ./internal/service/exam`
- [x] `go test -tags json1 ./api/v1 -run 'TestExamAPI(Detail|Overview|PublishWithMultipleTargets)'`
- [x] `go test -tags json1 ./internal/dao/db -run TestExamRepositoryImportCandidateTargetsFiltersAndWritesLog`
- [x] `go test -tags json1 ./internal/service/exam -run TestManagementImportCandidatesAddsUserTargetsAndRejectsStartedExam`
- [x] `go test -tags json1 ./api/v1 -run TestExamAPIImportCandidatesWithSQLite`
- [x] `go test -tags json1 ./internal/dao/db -run TestExamRepositoryResendInvitationsFiltersCandidatesAndWritesLog`
- [x] `go test -tags json1 ./internal/service/exam -run TestManagementResendInvitationsFiltersCandidatesAndRejectsStartedExam`
- [x] `go test -tags json1 ./api/v1 -run TestExamAPIResendInvitationsWithSQLite`
- [x] `go test -tags json1 ./api/v1 -run TestExamAPICandidateWritesRespectSpaceAdminAndTeacherScopeWithSQLite`
- [x] `go test -tags json1 ./api/v1 ./internal/service/exam ./internal/dao/db`
- [x] `npm test -- --run src/api/examDetail.test.ts --no-file-parallelism`
- [x] `npm test -- --run src/pages/Exam/PaperPreviewPage.test.tsx --no-file-parallelism`
- [x] `PaperPreviewPage.test.tsx` 覆盖真实考生列表、导入、重发邀请和 `can_manage_candidates=false` 禁用态。
- [x] `npm run build`
- [x] `space_admin` / `teacher` 不能跨空间导入或重发。
- [x] 开考后导入返回明确错误。
- [x] 写操作只使用 `POST`。

退出条件：

- [x] 考生管理 tab 不依赖 mock，权限和状态口径可解释。

### 阶段 7：成绩管理、答卷详情和考试设置

目标：完成成绩相关的统计、列表、答卷查看和成绩发布配置。

交付物：

- `GET /api/v1/exams/:id/results/summary`
- `GET /api/v1/exams/:id/results`
- `GET /api/v1/exams/:examID/attempts/:attemptID/answer-sheet`
- `POST /api/v1/exams/:id/settings`
- 成绩管理和考试设置 tab 真实数据。

任务：

- [x] 前端 `examDetailApi` 接入成绩摘要和成绩列表 GET 契约，完成 snake_case 到 camelCase 映射。
- [x] 实现成绩摘要，返回已交卷、平均分、最高分、及格率、待阅主观题。
- [x] 实现成绩列表，排名按 `total_score DESC, submitted_at ASC, id ASC`。
- [x] 不能用当前页下标生成排名。
- [x] 实现题型得分率和分数段分布。
- [x] 实现答卷详情，按 `examID + attemptID` 双重定位。
- [x] 答卷详情按权限返回标准答案、学生答案、得分和阅卷信息。
- [x] 教师首版不能导出成绩，前端隐藏入口，后端拒绝。
- [x] 考试设置只允许修改成绩发布配置。
- [x] 已开考后禁止修改发布范围、考试时长和公平性配置。

验证：

- [x] `npm test -- --run src/api/examDetail.test.ts --no-file-parallelism` 覆盖成绩摘要、成绩列表 GET 路径和字段映射。
- [x] `go test -tags json1 ./api/v1 -run TestExamAPIManagementResultsWithSQLite` 覆盖成绩摘要、分数段分布、题型得分率、成绩列表和第二页全局排名。
- [x] `go test -tags json1 ./api/v1 -run TestExamAPIManagementResultsEmptyWithSQLite` 覆盖有权限但无成绩时摘要和列表返回空集合。
- [x] `go test -tags json1 ./api/v1 -run TestExamAPIManagementAnswerSheetWithSQLite` 覆盖答卷详情、标准答案/学生答案/得分/阅卷状态返回和跨考试拒绝。
- [x] `go test -tags json1 ./api/v1 -run TestExamAPIManagementSettingsWithSQLite` 覆盖 `POST /api/v1/exams/:id/settings`、仅允许成绩发布字段、拒绝教师设置和写入 `update_settings` 操作日志。
- [x] `npm test -- PaperPreviewPage.test.tsx --no-file-parallelism` 覆盖成绩管理 tab 按后端权限隐藏发布成绩和导出成绩入口。
- [x] `npm test -- examDetail.test.ts --no-file-parallelism` 覆盖 `POST /api/v1/exams/:id/settings` 前端只发送成绩发布配置。
- [x] `npm test -- PaperPreviewPage.test.tsx --no-file-parallelism` 覆盖考试设置 tab 真实保存成绩发布配置、发布范围/考试时长只读展示。
- [x] `npm run build`
- [x] `go test -tags json1 ./api/v1 -run TestReviewAndResultAPIRoutesWithSQLite` 覆盖教师导出成绩 HTTP 403。
- [x] `go test -tags json1 ./internal/service/exam -run TestExportServiceRejectsTeacherExportEvenWhenTeacherCanViewScores` 覆盖教师即使可查看成绩也不能导出。
- [x] `go test -tags json1 ./api/v1 ./internal/service/exam ./internal/dao/db` 覆盖 handler、service fake 和 DAO 真实聚合查询。
- [x] 成绩列表跨页排名稳定。
- [x] 无成绩时返回空集合，不返回 403。
- [x] 答卷详情不能跨考试访问。
- [x] 教师导出成绩返回 403。
- [x] 设置接口只使用 `POST`。
- [x] 修改成绩配置写入操作日志。

退出条件：

- [x] 成绩管理和设置 tab 可真实使用，权限和排序稳定。

### 阶段 8：操作日志、端到端联调和文档收口

目标：补齐审计可追踪性，并完成整页联调验收。

交付物：

- `GET /api/v1/exams/:id/logs`
- 操作日志 tab。
- 浏览器验收记录。
- 文档和执行清单回写。

任务：

- [x] 发布考试写入 `publish_exam` 日志。
- [x] 导入考生写入 `import_candidates` 日志。
- [x] 重发邀请写入 `send_invite` 日志。
- [x] 发布成绩写入 `publish_results` 日志。
- [x] 修改设置写入 `update_settings` 日志。
- [x] 人工阅卷写入 `grade_answer` 日志。
- [x] 多空间操作按 `operation_group_id` 聚合。
- [x] 日志列表按 `created_at DESC, id DESC` 分页。
- [x] 操作日志 tab 接入真实接口。
- [x] 浏览器逐 tab 验收。
- [x] 实现和验证完成后，再同步执行清单勾选项。

验证：

- [x] `cd server && go test -tags json1 ./...`
- [x] `cd web && npm test`
- [x] `cd web && npm run build`
- [x] `git diff --check`
- [x] `cd server && go test -tags json1 ./api/v1 -run TestExamAPIManagementOperationLogsWithSQLite`
- [x] `cd server && go test -tags json1 ./api/v1 -run TestExamAPIPublishWithMultipleTargetsWithSQLite`
- [x] `cd server && go test -tags json1 ./api/v1 -run TestReviewAndResultAPIRoutesWithSQLite`
- [x] `cd server && go test -tags json1 ./internal/service/exam -run TestReviewServiceGradesShortTextWithVersionAndRecalculatesScores`
- [x] `cd server && go test -tags json1 ./api/v1 ./internal/service/exam ./internal/dao/db`
- [x] `cd web && npm test -- examDetail.test.ts --no-file-parallelism`
- [x] `cd web && npm test -- examDetail.test.ts PaperPreviewPage.test.tsx --no-file-parallelism`
- [x] `cd web && npm test -- PaperPreviewPage.test.tsx --no-file-parallelism`
- [x] `cd server && go test -tags json1 ./api/v1 -run 'TestExamAPI(DetailReturnsManagementPermissionsWithSQLite|ManagementOperationLogsWithSQLite|ImportCandidatesWithSQLite)'`
- [x] `cd server && go test -tags json1 ./internal/service/exam -run 'TestManagement(Detail|Overview)'`
- [x] `cd web && npm test -- PaperPreviewRoute.test.tsx --no-file-parallelism`
- [x] `tenant_admin` 可见全量。
- [x] `space_admin` / `teacher` 只见授权范围。
- [x] `student` 访问管理端详情返回 403 或被路由守卫拦截。
- [x] `考试监控` tab 仍仅占位且无监控接口请求。
- [x] 刷新页面不回退 mock。

退出条件：

- [x] 后端、前端和浏览器验收全部通过。
- [x] 文档、执行清单和剩余风险同步完成。

## 10. 风险与决策

- `paper_id` 入口存在歧义，必须尽快迁移到 `exam_id` 路由。
- `rule_live` 预览不能重新抽题，否则会和真实考试不一致。
- 操作日志和考试事件语义不同，必须分表。
- 考试监控本期不做，不新增监控接口和监控事件索引；后续实现时再评估
  `exam_events.exam_id` 冗余字段。
- 教师导出成绩首版不开放，避免权限过宽。
- 新建考试如果不先支持多目标和考生范围，后续考生管理页会缺少真实名单来源。
- 大规模统计暂不引入快照表，先用索引和聚合查询满足当前规模。
- 前端必须保留当前视觉结构，避免把接口化任务扩大成 UI 重构。

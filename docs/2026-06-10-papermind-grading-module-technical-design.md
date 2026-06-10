# PaperMind 阅卷模块技术方案

## 1. 设计目标

阅卷模块负责把考生提交的答卷转化为可发布成绩。

首版目标：

```text
1. 客观题自动判分
2. 简答题人工阅卷
3. 支持阅卷规则配置
4. 支持按班级/空间、按答卷数量批量分配
5. 支持老师阅卷工作台
6. 支持阅卷进度统计
7. 支持成绩发布前校验
8. 支持审计追溯
```

当前技术方案已经定义了 `exam_attempt_questions` 保存考生题目快照，`exam_answers` 保存考生答案、得分、阅卷状态、阅卷人、阅卷时间和评语，这可以作为阅卷模块的核心数据基础。

首版不做：

```text
- AI 判卷
- 双评
- 仲裁
- 阅卷组
- 按题精细化分配
- 评分误差分析
- 复杂 RBAC
- 平台管理员代入阅卷
```

---

## 1.1 全局契约与实现边界

阅卷模块首版必须先和现有考试、成绩、权限、日志链路合并，而不是另起一套平行实现。

当前文档同时记录“目标契约”和“现有实现改造点”。凡是标为“现有缺口”的内容，都不能在实现或联调时假定已经完成，必须先补后端、前端和回归测试，再把对应清单项标为完成。

### 时间戳单位

所有前后端 API、数据库字段、审计日志和文档示例中的业务时间戳统一使用 **Unix 秒**。

```text
1. `start_time`、`end_time`、`submitted_at`、`graded_at`、`score_publish_time` 等字段都使用 Unix 秒。
2. `created_at`、`updated_at`、`assigned_at`、`started_at`、`completed_at` 等审计时间也使用 Unix 秒。
3. 前端展示前只做秒级时间戳格式化，不再接收或提交 Unix 毫秒。
4. 后端所有 `now` 注入、数据库写入、响应 DTO 和测试 fixture 都必须统一到秒。
5. 实现迁移时必须一次性处理现有毫秒字段，禁止同一接口或同一表内混用秒和毫秒。
```

这是一项跨模块契约变更，实施时需要同步检查：

```text
- exam publish / draft / settings API
- exam attempt start / submit / event API
- grading API
- results API
- exam operation logs
- 前端 API client、页面展示和测试 fixture
- 现有数据库中已经按毫秒写入的历史数据迁移策略
```

时间戳迁移必须作为独立子任务先落地，不能夹在阅卷功能实现中顺手修改。

现有实现中仍有大量毫秒语义，例如 `time.Now().UnixMilli()`、`answerDeadline = started_at + duration_minutes * 60_000`、前端测试 fixture 中的 13 位发布时间。本文后续 API 示例中的秒级时间戳代表 **Task 0 完成后的目标契约**。在 Task 0 完成前：

```text
1. 新增阅卷能力不得单独切换为秒级时间戳，避免同一考试链路内同时出现秒和毫秒。
2. 如果必须先实现阅卷业务，应继续沿用当前毫秒契约，并把秒级迁移作为独立提交处理。
3. Task 0 完成后，所有旧 API、兼容 API 和新 API 必须一次性对齐到 Unix 秒。
4. 测试必须同时覆盖旧毫秒 fixture 已迁移、13 位时间戳被拒绝、截止时间和发布时间比较仍正确。
```

迁移范围：

```text
1. exams：start_time、end_time、score_publish_time。
2. exam_attempts：started_at、submitted_at、exam_token_expires_at。
3. exam_answers：graded_at。
4. exam_events / exam_operation_logs：事件时间和 created_at。
5. 所有业务表 BaseFields：created_at、updated_at、deleted_at 等时间字段。
6. users / platform_users：last_login_at。
7. 所有 `*_at`、`*_time`、`*_expires_at` 业务时间字段和迁移注释。
8. 新增阅卷表：created_at、updated_at、assigned_at、started_at、completed_at。
9. 前端测试 fixture、API mock、页面格式化工具和浏览器展示断言。
```

结构基准：

```text
1. 当前仍处于初始化建库阶段，PostgreSQL / MySQL / SQLite 各库目录只保留 001_tenant_space.sql 作为基准结构。
2. 考试管理详情和目标作用空间相关结构已合并进各自 001_tenant_space.sql，不再保留 002_exam_management_detail.sql / 003_exam_target_scope_spaces.sql。
3. 001_tenant_space.sql 只表达最终表结构和索引，不包含历史回填、防重复执行或方言动态 DDL 迁移逻辑。
4. 后续项目进入生产或存在历史库后，新增字段、索引和回填逻辑必须重新按递增版本追加迁移。
```

迁移规则：

```text
1. 13 位毫秒时间戳转换为秒：value = value / 1000，向下取整。
2. 10 位秒级时间戳保持不变。
3. NULL 保持 NULL，0 保持 0。
4. 数据库迁移必须幂等：只转换 value >= 1000000000000 的行。
5. Go service 的 `now` 统一改为 `time.Now().Unix()`。
6. token 过期时间、发布时间、提交时间等比较逻辑都只比较秒。
7. 实现完成前，禁止同一接口同时接收秒和毫秒做自动猜测；边界校验发现 13 位时间戳应直接返回参数错误。
```

### 结构化字段与 `ext_json` 使用边界

`ext_json` 只允许保存展示类、非核心扩展元数据。任何会参与计算、排序、过滤、权限判断、状态流转、成绩发布校验、阅卷分配控制、唯一性约束、幂等判断、审计聚合、去重或分页的字段，都必须设计为显式列、关系表或可索引的结构化字段，不能放入 `ext_json`。

本模块首版必须按以下边界实现：

```text
1. `need_review`、`assignment_required`、`active_assignment_key`、`rubric_scope_key`、`operation_group_id` 等字段都属于核心流程字段，必须显式列化。
2. `grading_rules.ext_json`、`grading_rubrics.ext_json`、`grading_assignments.ext_json`、`grading_logs.ext_json` 只能保存展示类元数据，不能控制阅卷、分配、发布或日志可见性。
3. 有独立业务语义的 JSON 字段不等同于 ext_json，例如 manual_question_types、items_json、rubric_scores_json；它们只能按主键或外键读出后做结构化解析，不能依赖 JSON key 做数据库过滤、排序、权限裁剪、唯一性或分页。
4. 如果 JSON 内部字段后续进入查询条件、权限判断、状态流转或统计聚合，必须迁移为显式列、独立子表或同步冗余字段。
5. 允许为了减少 join 冗余结构化字段，例如在阅卷日志中冗余 exam_id、attempt_id、attempt_question_id、answer_id、space_id；冗余字段必须有明确来源，并在同一事务内写入。
6. 冗余字段如果参与查询、排序、裁剪或唯一性判断，必须同步设计索引、回填策略和测试用例。
7. Review 数据库设计时必须主动检查是否存在“核心字段藏在 ext_json”的情况；发现后应改为显式列或独立表。
```

特别注意：现有 `exam_operation_logs` 如需按 `operation_group_id` 做多空间日志聚合、去重、分页或裁剪，也必须把 `operation_group_id` 迁移为显式列，不能继续写在 `ext_json` 里。本方案覆盖 `docs/2026-06-04-papermind-exam-management-detail-implementation-design.md` 中把 `operation_group_id` 保存在 `ext_json` 的旧设计；后续实现以本方案和 `AGENTS.md` 的 `ext_json` 硬规则为准。

### API 命名空间

REST API 继续统一放在 `/api/v1` 下演进，不新增 `/api/v1/tenant` 前缀。

现有接口需要按能力逐步扩展：

```text
/api/v1/grading
├── 阅卷中心
├── 待阅任务
├── 答卷阅卷数据
├── 保存评分
└── 阅卷分配

/api/v1/results
├── 成绩查询
├── 成绩发布配置
├── 成绩发布生效
└── 成绩导出
```

旧接口兼容策略必须明确到路径级别，避免形成第二套业务逻辑。

```text
1. 当前已有 `GET /api/v1/grading/pending`，后续可保留为待阅任务兼容入口，但必须复用新的阅卷任务查询 service。
2. 当前已有 `POST /api/v1/exam-attempts/:attempt_id/questions/:attempt_question_id/grade`，后续可保留为旧评分入口，但必须先解析 answer，再转发到新的 `GradeAnswer` service。
3. 新评分入口是 `POST /api/v1/grading/answers/:answer_id/grade`，作为前端新工作台的主入口。
4. 当前已有 `POST /api/v1/results/publish-config`，继续作为成绩发布配置入口；它负责保存 `publish_mode` / `score_publish_time`，不是另一套阅卷或成绩计算逻辑。
5. 新发布入口是 `POST /api/v1/results/exams/:exam_id/publish`，作为带 pending 校验、复核二次确认和审计写入的显式发布入口。
6. 当前已有 `GET /api/v1/results` 和 `POST /api/v1/results/export`，可以继续服务旧成绩页；如果新增考试维度成绩页 API，必须复用同一个 result repository 和权限裁剪逻辑。
7. 兼容入口只允许做参数适配、响应适配和错误码映射，不能复制评分、发布、权限或日志写入逻辑。
8. `publish-config` 和新发布入口必须共用同一套发布可见性校验：配置会让成绩立即或到当前时间可见时，必须检查 pending 主观题、attempt.status 和 need_review；只保存未来公布时间或未公布配置时，不应阻止配置保存，但学生仍不可见。
9. 前端迁移完成并移除旧页面调用后，才能删除兼容入口；删除前必须更新 API client、页面测试和后端路由测试。
```

现有缺口：当前 `POST /api/v1/results/publish-config` 仍在 handler 层限制只有 `tenant_admin` 可以保存配置，`UpdateScorePublishConfig` 也只负责写入 `publish_mode` / `score_publish_time` 和操作日志，尚未复用目标空间权限、pending 主观题、`attempt.status` 和 `need_review` 校验。落地 Task 7 前必须先补齐：

```text
1. 移除 publish-config 的 tenant_admin-only 判断，改为从考试目标范围推导真实授权空间。
2. tenant_admin 可修改本租户考试配置；space_admin / teacher 仅可修改落在其启用授权空间内的考试配置。
3. 配置会让成绩立即或到当前时间可见时，必须阻断未完成阅卷、pending 主观题和 review_publish_policy = block 的 need_review。
4. 兼容入口和新发布入口必须共用同一个 service 校验函数，避免旧入口绕过新发布校验。
5. 回归测试必须覆盖授权 space_admin / teacher 可保存配置、无目标空间权限返回 RESULT_PUBLISH_FORBIDDEN、pending 主观题不能提前让学生可见。
```

### 成绩发布模型

首版不新增 `exams.result_status` 落库字段，避免和现有 `publish_mode + score_publish_time` 形成双写真相。

管理端需要展示 `result_status` 时，统一按以下规则派生：

```text
1. pending_grading：存在待阅主观题或任一已提交 attempt 未完成阅卷。
2. unpublished：publish_mode = manual_publish 且 score_publish_time 为空或为 0。
3. scheduled：publish_mode = manual_publish 且 score_publish_time > now。
4. published：publish_mode = immediate_score，或 manual_publish 且 score_publish_time <= now。
```

派生状态必须按上述顺序判断。`pending_grading` 优先级最高，即使 `publish_mode = immediate_score`，只要仍有待阅主观题，也不能展示为 `published`。

管理端 `result_status` 必须按当前调用人的授权范围派生：

```text
1. tenant_admin：按整场考试的所有目标空间和用户目标派生。
2. space_admin：只按其授权空间内命中的作答和待阅答案派生。
3. teacher：只按其授权空间或本人任务范围内的作答和待阅答案派生。
4. 不同空间的阅卷进度不能互相污染；A 空间未阅完不能让 B 空间视图显示 pending_grading。
5. 学生查分不读取管理端 result_status，只按本人 attempt 的可见性规则判断。
```

注意：这里的授权范围派生只影响管理端列表和详情展示，不代表可以做空间级独立发布。

首版成绩发布仍然是整场考试的全局动作：

```text
1. 只写 exams.publish_mode / exams.score_publish_time。
2. 不新增空间级发布表，不支持 A 空间先发布、B 空间后发布。
3. 成绩发布配置以长期技术方案为准：tenant_admin 可以修改本租户考试配置；具备对应考试发布目标空间权限的 space_admin / teacher 也可以修改授权范围内考试的发布配置。
4. 写入配置前，后端必须从 exam_targets / exam_target_scope_spaces 推导真实目标空间，不能信任请求体 space_id 构造权限。
5. 配置会让成绩立即或已经到当前时间可见时，必须先校验整场考试目标范围内不存在待阅主观题、未 graded attempt 和阻断发布的 need_review。
6. 配置为未来公布时间或手动未公布时，可以保存配置，但学生查分仍按 graded / pending / score_publish_time 可见性规则拒绝提前查看。
```

如果后续要支持空间级独立发布，必须新增独立的发布作用域模型，例如
`exam_result_publish_scopes`，并同步改学生查分、导出、日志和权限测试。不能只在
`exams.score_publish_time` 上混入空间语义。

如果后续要支持“空间级独立发布”或“发布配置以外的最终确认发布权限”，必须作为单独权限变更处理，并同步修改
`ManagementDetailService.managementPermissions`、前端按钮权限、发布接口权限、学生查分和回归测试。

学生查分必须同时满足：

```text
1. 当前用户只能查看自己的作答。
2. 目标 attempt 已提交。
3. 不存在待阅主观题。
4. attempt.status = graded。
5. 成绩发布配置已经允许当前时间可见。
```

这样可以避免“只保存了发布时间配置，但主观题未阅完，学生提前看到部分分数”的问题。

现有缺口：当前学生查分链路仍主要按 `submitted_at IS NOT NULL`、`publish_mode` 和 `score_publish_time` 判断可见性，尚未把 `attempt.status = graded` 与 pending 主观题答案作为强校验。阅卷模块落地前必须先修复该缺口：

```text
1. ResultRepository 读取学生成绩时必须带出 attempt.status，或直接只查询 status = graded 的 attempt。
2. ResultService.visibleResult 必须拒绝未完成阅卷的 attempt。
3. 如果仓储没有把 pending 主观题答案裁掉，service 必须额外查询并拒绝存在 pending answer 的 attempt。
4. latest / highest 策略只能在已经可见的 graded attempts 中选择。
5. 回归测试必须覆盖 manual_publish 已到时间但 attempt 仍 submitted / 存在 pending answer 时学生不可见。
```

---

# 2. 核心概念

## 2.1 阅卷规则

```text
阅卷规则 = 本场考试的阅卷配置 + 评分限制 + 主观题评分标准 + 发布约束
```

它不是一篇文章，也不是单纯说明文，而是结构化配置。

包括：

```text
- 哪些题型需要人工阅卷
- 是否允许小数分
- 分数精度
- 评语是否必填
- 0 分是否必须填写评语
- 是否允许标记复核
- 主观题评分项
- 发布前校验策略
```

## 2.2 阅卷任务

```text
阅卷任务 = 某个老师被分配的一批待阅答卷或待阅题目
```

首版建议任务粒度以 **答卷 attempt** 为主。

```text
一个老师负责一份答卷中的所有待阅主观题。
```

后续可以扩展为：

```text
一个老师只负责某份答卷中的某一道主观题。
```

## 2.3 阅卷结果

阅卷结果仍然落在 `exam_answers` 上。

```text
exam_answers.score
exam_answers.grading_status
exam_answers.graded_by
exam_answers.graded_at
exam_answers.grader_comment
exam_answers.need_review
exam_answers.rubric_scores_json
```

`grading_assignments` 只表达“谁被分配了任务”，不表达最终分数。最终分数必须以 `exam_answers` 为准。

---

# 3. 阅卷流程总览

```text
考生提交答卷
  ↓
系统自动判客观题
  ↓
主观题答案进入 pending
  ↓
生成阅卷任务池
  ↓
管理员/空间管理员批量分配
  ↓
老师进入阅卷工作台
  ↓
逐题评分并保存
  ↓
系统重算主观题分、总分
  ↓
所有主观题完成后 attempt.status = graded
  ↓
整场考试阅卷完成
  ↓
发布成绩 / 到公布时间后学生可见
```

技术方案中已规定，包含简答题的试卷默认走“教师阅卷后发布”，不允许立即出分，也不允许 `max_attempts > 1`，这样首版无需处理多次主观题阅卷结果合并问题。

---

# 4. 状态模型

## 4.1 `exam_attempts.status`

现有状态：

```text
in_progress   作答中
submitted     已提交，等待判分或阅卷
graded        已完成判分/阅卷
```

建议首版保持这三个状态，不新增复杂状态。

状态流转：

```text
in_progress
  → submitted
  → graded
```

规则：

```text
1. 客观题自动判分后，attempt 仍然可以是 submitted。
2. 只有该 attempt 下所有 exam_answers 都不再是 pending，才可变为 graded。
3. attempt.status = graded 后，默认不允许普通老师继续改分。
4. 如需改分，后续通过复核/管理员改分流程扩展。
```

## 4.2 `exam_answers.grading_status`

现有定义：

```text
auto       客观题自动判分
pending    主观题待人工阅卷
graded     主观题已人工评分
```

建议首版扩展为：

```text
auto       客观题已自动判分
pending    主观题待阅卷
graded     主观题已评分
review     已评分但标记复核
```

如果想更保守，可以先不落 `review` 状态，只用 `exam_answers.need_review` 显式列保存复核标记。

推荐：

```text
首版字段仍使用 auto / pending / graded；
是否复核放到 exam_answers.need_review 显式列。
```

原因：少改表结构，避免状态复杂化。

---

# 5. 数据库设计

## 5.1 复用现有表

### `exam_attempts`

已经具备：

```text
objective_score
subjective_score
total_score
status
version
```

用于汇总成绩。

### `exam_attempt_questions`

已经具备：

```text
section_snapshot
question_snapshot
option_snapshot
correct_answer_snapshot
score
sort_order
```

这保证阅卷时看到的是考试快照，不受题库后续修改影响。技术方案也明确历史答卷、成绩和考试快照不能依赖原始题库是否已删除。

### `exam_answers`

已经具备：

```text
answer_content
score
grading_status
graded_by
graded_at
grader_comment
version
ext_json
```

并且技术方案要求 `exam_answers` 建立 `UNIQUE (tenant_id, attempt_id, attempt_question_id)`，自动保存和阅卷都只读取这一条唯一答案，避免同一道题出现多行答案。

首版继续复用现有 `exam_answers.ext_json` 保存非核心扩展元数据，不再新增同名字段。

由于复核标记会参与成绩发布校验，评分项明细会参与阅卷结果展示和审计，二者都不应放入
`ext_json`。首版需要在 `exam_answers` 上新增显式字段：

```text
need_review BOOLEAN NOT NULL DEFAULT FALSE
rubric_scores_json JSON NOT NULL
```

`rubric_scores_json` 示例：

```json
[
  {
    "key": "step_complete",
    "score": "5"
  }
]
```

迁移要求：

```text
1. PostgreSQL、MySQL、SQLite 三套迁移都要新增 need_review 和 rubric_scores_json。
2. 历史行 need_review 统一回填 false。
3. 历史行 rubric_scores_json 统一回填空数组。
4. service 读取 rubric_scores_json 时只接受合法 JSON 数组；非法 JSON 视为数据错误并显式失败。
5. 发布校验必须按 need_review 显式列查询，不能按 ext_json JSON key 查询。
6. ext_json 只保留展示类、非核心扩展元数据，不能保存影响发布、评分、权限、排序、过滤、审计或成绩可见性的字段。
7. 所有 exam_answers insert / upsert 路径都必须显式写入 rubric_scores_json = []，包括自动保存、提交判分和人工阅卷。
8. MySQL JSON 字段不要依赖 DEFAULT 表达式；创建新行时由 repository/service 显式填充 []。
9. PostgreSQL 可使用 jsonb，MySQL 使用 json，SQLite 使用 text 保存 JSON 字符串；三库读取到 service 后统一反序列化为数组。
```

发布校验索引：

```sql
INDEX (tenant_id, attempt_id, need_review)
```

`exam_answers` 当前不直接保存 `exam_id`，发布校验需要先按 `exam_attempts(tenant_id, exam_id, status)` 收敛 attempt，再用 `tenant_id + attempt_id + need_review` 命中复核标记。如果后续为了减少 join 决定在 `exam_answers` 冗余 `exam_id`，必须同步新增 `INDEX (tenant_id, exam_id, need_review)`，并在自动保存、提交判分和人工阅卷路径中同事务写入。

---

## 5.2 新增表：`grading_rules`

用于保存考试级阅卷规则。

```sql
CREATE TABLE grading_rules (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    exam_id BIGINT NOT NULL,

    grading_mode VARCHAR(32) NOT NULL,
    manual_question_types TEXT NOT NULL,

    allow_decimal_score BOOLEAN NOT NULL DEFAULT FALSE,
    score_precision VARCHAR(16) NOT NULL DEFAULT 'integer',
    comment_required BOOLEAN NOT NULL DEFAULT FALSE,
    zero_score_comment_required BOOLEAN NOT NULL DEFAULT FALSE,
    full_score_comment_required BOOLEAN NOT NULL DEFAULT FALSE,
    allow_mark_review BOOLEAN NOT NULL DEFAULT TRUE,
    assignment_required BOOLEAN NOT NULL DEFAULT FALSE,

    review_publish_policy VARCHAR(32) NOT NULL DEFAULT 'allow_with_warning',

    status VARCHAR(32) NOT NULL DEFAULT 'enabled',

    created_at BIGINT NOT NULL,
    created_by BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    updated_by BIGINT NOT NULL,
    version BIGINT NOT NULL DEFAULT 1,
    ext_json JSON NOT NULL
);
```

字段说明：

| 字段                            | 说明                                             |
| ----------------------------- | ---------------------------------------------- |
| `grading_mode`                | `by_attempt` / `by_question`，首版固定 `by_attempt` |
| `manual_question_types`       | JSON 数组字符串，例如 `["short_text"]`                 |
| `allow_decimal_score`         | 是否允许小数分                                        |
| `score_precision`             | `integer` / `half` / `one_decimal`             |
| `comment_required`            | 所有人工评分是否必须写评语                                  |
| `zero_score_comment_required` | 0 分是否必须写评语                                     |
| `full_score_comment_required` | 满分是否必须写评语                                      |
| `allow_mark_review`           | 是否允许标记复核                                       |
| `assignment_required`         | 是否要求普通 teacher 只能进入自己的有效阅卷任务                         |
| `review_publish_policy`       | 存在复核标记时的发布策略：`allow_with_warning` / `block` |

唯一约束：

```sql
UNIQUE (tenant_id, exam_id)
```

首版规则：

```text
1. 每场考试最多一条 grading_rules。
2. 发布考试时，如果试卷包含 short_text，自动创建或更新 grading_rules。
3. `grading_rules` 不保存当前成绩发布方式或公布时间。
4. 当前成绩发布配置只以 `exams.publish_mode` / `exams.score_publish_time` 为准。
5. 包含 short_text 时，发布考试必须把 `exams.publish_mode` 固定为 `manual_publish`。
6. 不包含 short_text 时可以使用 `immediate_score`。
7. `assignment_required` 是核心流程字段，不能放入 ext_json。
8. `ext_json` 只能保存展示类元数据，不能保存任何会改变阅卷规则生效结果的字段。
9. allow_decimal_score = false 时，score_precision 只能是 integer。
10. score_precision = half / one_decimal 时，allow_decimal_score 必须为 true。
```

---

## 5.3 新增表：`grading_rubrics`

用于保存主观题评分标准。

```sql
CREATE TABLE grading_rubrics (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    exam_id BIGINT NOT NULL,

    question_id BIGINT NOT NULL,
    rubric_scope_key VARCHAR(128) NOT NULL,
    attempt_question_id BIGINT DEFAULT 0,

    rubric_title VARCHAR(255) NOT NULL,
    total_score DECIMAL(10,2) NOT NULL,

    items_json JSON NOT NULL,

    status VARCHAR(32) NOT NULL DEFAULT 'enabled',

    created_at BIGINT NOT NULL,
    created_by BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    updated_by BIGINT NOT NULL,
    version BIGINT NOT NULL DEFAULT 1,
    ext_json JSON NOT NULL
);
```

`items_json` 示例：

```json
[
  {
    "key": "step_complete",
    "name": "步骤完整",
    "score": "5",
    "description": "关键步骤完整，无明显跳步"
  },
  {
    "key": "formula_correct",
    "name": "公式正确",
    "score": "5",
    "description": "公式使用正确，变形无误"
  },
  {
    "key": "reasoning_valid",
    "name": "推导合理",
    "score": "3",
    "description": "推导过程逻辑清晰"
  },
  {
    "key": "conclusion_strict",
    "name": "结论严谨",
    "score": "2",
    "description": "结论表达完整严谨"
  }
]
```

`grading_rubrics.ext_json` 只能保存评分标准的展示类扩展信息。`items_json`、`rubric_scope_key`、`total_score`、`status` 等会参与评分计算、版本选择或查询过滤的字段不能转存到 `ext_json`。

`items_json[].score` 和 `total_score` 在 API 和 service 中统一按十进制字符串处理；数据库 JSON 中也建议保存字符串，避免不同语言运行时把小数分转成二进制浮点后出现精度差异。

唯一约束建议：

```sql
UNIQUE (tenant_id, exam_id, rubric_scope_key)
```

首版不能只用原始 `question_id` 作为唯一维度。

原因：

```text
1. 同一道题可能在同一考试中被不同大题复用。
2. 同一道题在不同大题中可能分值不同。
3. rule_live / 随机卷场景下，评分标准要和考试快照对齐，不能依赖题库当前状态。
```

`rubric_scope_key` 由发布或生成阅卷规则时确定，建议格式：

```text
固定卷：section:<section_id>:question:<question_id>:sort:<sort_order>:score:<score>
随机卷：section:<section_id>:question:<question_id>:score:<score>
特殊快照差异：attempt_question:<attempt_question_id>
```

如果后续支持按题分配、双评或考生题目快照差异化评分，可以进一步把 `attempt_question_id` 提升为主维度。

---

## 5.4 新增表：`grading_assignments`

用于保存阅卷分配任务。

```sql
CREATE TABLE grading_assignments (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    exam_id BIGINT NOT NULL,
    space_id BIGINT NOT NULL DEFAULT 0,

    attempt_id BIGINT NOT NULL,
    attempt_question_id BIGINT NOT NULL DEFAULT 0,
    answer_id BIGINT NOT NULL DEFAULT 0,

    grader_id BIGINT NOT NULL,

    assign_mode VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'assigned',
    active_assignment_key VARCHAR(128) NOT NULL,

    assigned_by BIGINT NOT NULL,
    assigned_at BIGINT NOT NULL,
    started_at BIGINT NOT NULL DEFAULT 0,
    completed_at BIGINT NOT NULL DEFAULT 0,

    created_at BIGINT NOT NULL,
    created_by BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    updated_by BIGINT NOT NULL,
    version BIGINT NOT NULL DEFAULT 1,
    ext_json JSON NOT NULL
);
```

字段说明：

| 字段                    | 说明                                               |
| --------------------- | ------------------------------------------------ |
| `attempt_id`          | 被分配的答卷                                           |
| `attempt_question_id` | 首版为 0，表示整份答卷分配；后续按题分配时非 0                        |
| `answer_id`           | 可选，按题分配时绑定具体答案                                   |
| `grader_id`           | 阅卷老师                                             |
| `assign_mode`         | `by_space` / `by_attempt_even` / `manual`        |
| `status`              | `assigned` / `grading` / `completed` / `revoked` |
| `active_assignment_key` | 有效任务唯一键，用于数据库层防止同一答卷被重复有效分配 |
| `assigned_by`         | 分配人                                              |
| `started_at`          | 老师首次进入阅卷工作台                                      |
| `completed_at`        | 完成时间                                             |

`grading_assignments.ext_json` 只能保存展示类补充信息。`space_id`、`attempt_id`、`grader_id`、`assign_mode`、`status`、`active_assignment_key`、`assigned_at`、`started_at`、`completed_at` 都会参与分配、过滤、排序或唯一性控制，必须保持显式列。

唯一约束：

```sql
UNIQUE (tenant_id, exam_id, active_assignment_key)
```

`active_assignment_key` 规则：

```text
1. assigned / grading：
   - attempt:<attempt_id>:question:<attempt_question_id>
   - 首版 attempt_question_id = 0，表示整份答卷分配。
2. completed / revoked：
   - inactive:<assignment_id>
   - 状态变更时在同一事务内改写，允许历史记录保留多行。
```

这样可以在 SQLite、MySQL、PostgreSQL 三套数据库上都用普通唯一索引兜住并发分配，不依赖局部唯一索引。

辅助索引：

```sql
INDEX (tenant_id, exam_id, grader_id, status)
INDEX (tenant_id, exam_id, attempt_id, status)
INDEX (tenant_id, exam_id, space_id, status)
```

并在 Service 层保证：

```text
同一 attempt 在 assigned / grading 状态下只能有一个有效分配；
数据库唯一约束是最后兜底，Service 层仍要先做冲突检测并返回明确错误。
```

---

## 5.5 新增表：`grading_logs`

用于审计改分和阅卷行为。

```sql
CREATE TABLE grading_logs (
    id BIGINT PRIMARY KEY,
    tenant_id BIGINT NOT NULL,
    exam_id BIGINT NOT NULL,
    space_id BIGINT NOT NULL DEFAULT 0,
    attempt_id BIGINT NOT NULL,
    attempt_question_id BIGINT NOT NULL,
    answer_id BIGINT NOT NULL,

    grader_id BIGINT NOT NULL,
    actor_type VARCHAR(32) NOT NULL,
    actor_role VARCHAR(32) NOT NULL,
    operation_group_id VARCHAR(64) NOT NULL,
    action VARCHAR(32) NOT NULL,

    old_score DECIMAL(10,2),
    new_score DECIMAL(10,2),
    old_comment TEXT,
    new_comment TEXT,

    old_status VARCHAR(32),
    new_status VARCHAR(32),

    created_at BIGINT NOT NULL,
    created_by BIGINT NOT NULL,
    ext_json JSON NOT NULL
);
```

`grading_logs` 和现有 `exam_operation_logs` 的关系：

```text
1. `grading_logs` 是答题级改分明细，服务于追溯某道题从几分改到几分。
2. `exam_operation_logs` 是管理端考试操作流，服务于考试详情页日志、近期动态和多空间可见性裁剪。
3. 保存评分、修改他人评分等答题级管理动作需要同时写 `grading_logs` 和 `exam_operation_logs`。
4. 发布成绩是考试级动作，只写 `exam_operation_logs`，不写 `grading_logs`。
5. 完成答卷阅卷是答卷级动作，只写 `exam_operation_logs`，不写 `grading_logs`。
6. 多空间作答的评分日志要保留 `space_id` / `operation_group_id` 语义，避免 tenant_admin 聚合和 space_admin 裁剪结果不一致。
7. `exam_operation_logs.operation_group_id` 也必须是显式列；不能写入 `ext_json.operation_group_id` 后再依赖 JSON 查询做聚合、去重、分页或空间裁剪。
8. 只读查询、预览分配、打开工作台不写改分日志；是否写管理端访问日志由后续审计策略单独决定。
```

`operation_group_id` 写入和迁移规则：

```text
1. 新写入的 grading_logs 和 exam_operation_logs 必须生成非空 operation_group_id。
2. 同一次业务动作影响多条空间日志时，多行共享同一个 operation_group_id。
3. 单行日志也必须写 operation_group_id，不能用空字符串或 NULL 表示“无需分组”。
4. 当前基准 SQL 不包含历史回填逻辑；如果运行环境已有旧日志数据，单独制定数据修复脚本。
5. 旧日志数据修复时优先从 ext_json.operation_group_id 回填显式列；缺失时按单条日志生成唯一 group，避免多条缺失日志被空值错误聚合。
6. 修复完成后，日志聚合、去重、分页和裁剪只能读取显式列。
```

辅助索引：

```sql
INDEX (tenant_id, exam_id, created_at, id)
INDEX (tenant_id, answer_id, created_at)
INDEX (tenant_id, exam_id, space_id, created_at)
INDEX (tenant_id, exam_id, operation_group_id)
INDEX (tenant_id, attempt_id, attempt_question_id, created_at)
```

索引用途：

```text
1. 考试级评分流水按时间倒序查询。
2. 单题历史评分追溯。
3. space_admin 按授权空间裁剪评分日志。
4. tenant_admin 按 operation_group_id 聚合同一次多空间评分动作。
5. 答卷详情页按 attempt + question 拉取本题改分历史。
```

多空间作答的 `grading_logs` 写入规则：

```text
1. 一次评分动作生成一个 operation_group_id。
2. 如果该 attempt 命中多个目标空间，每个命中空间写一条 grading_logs。
3. 多条日志的 answer_id、attempt_id、attempt_question_id、old_score、new_score 和 operation_group_id 相同。
4. 每条日志的 space_id 分别写对应目标空间，方便 space_admin 按空间过滤。
5. tenant_admin 查询时按 operation_group_id 聚合展示，避免同一次评分动作重复刷屏。
6. 只有用户目标且无法归属空间时，space_id = 0；space_admin 不应看到 space_id = 0 的日志。
7. 不把多个空间压进单行 ext_json.space_ids 作为首版方案，因为这会让 SQL 裁剪和分页统计变复杂。
8. `grading_logs.ext_json` 只能保存展示类补充信息，不能承载影响日志查询、裁剪、排序、聚合或审计判断的字段。
```

`action` 枚举：

```text
grade              首次评分
update_grade       修改评分
mark_review        标记复核
unmark_review      取消复核
```

---

# 6. 阅卷规则设计

## 6.1 默认规则生成

考试发布时执行：

```text
1. 查询 exam.paper_id。
2. 查询试卷大题和题型。
3. 判断是否包含 short_text。
4. 如果包含 short_text：
   - exams.publish_mode = manual_publish
   - manual_question_types = ["short_text"]
   - allow_decimal_score = false
   - score_precision = integer
   - comment_required = false
   - allow_mark_review = true
   - review_publish_policy = allow_with_warning
5. 如果纯客观题：
   - exams.publish_mode 可为 immediate_score
   - manual_question_types = []
```

## 6.2 评分项来源

评分项来源优先级：

```text
1. grading_rubrics 中考试级评分标准
2. exam_attempt_questions.question_snapshot 中的参考答案/解析
3. questions.analysis 作为兜底
```

建议首版：

```text
创建试卷或发布考试时，不强制每道简答题配置评分项。
如果没有 grading_rubrics，阅卷工作台只展示“参考答案 / 解析”。
如果没有 grading_rubrics，保存评分时允许省略 rubric_scores，或提交空数组。
没有 grading_rubrics 时，后端统一写入 rubric_scores_json = []。
如果存在 grading_rubrics，保存评分时 rubric_scores 必须覆盖当前 rubric 的全部评分项。
```

## 6.3 得分校验规则

评分保存时校验：

```text
1. score >= 0
2. score <= exam_attempt_questions.score
3. 如果 allow_decimal_score = false，score 必须是整数
4. 如果 score_precision = half，score 必须是 0.5 的倍数
5. 如果 score_precision = one_decimal，最多 1 位小数
6. comment_required = true 时，评语不能为空
7. zero_score_comment_required = true 且 score = 0 时，评语不能为空
8. full_score_comment_required = true 且 score = full_score 时，评语不能为空
```

## 6.4 成绩发布校验

成绩发布前校验：

```text
1. 当前用户有成绩发布配置写权限：tenant_admin，或具备对应考试发布目标空间权限的 space_admin / teacher。
2. exam 属于当前 tenant
3. exam.status 不是 draft
4. 由后端从 exam_targets / exam_target_scope_spaces 推导真实目标空间，不能信任请求体 space_id。
5. 如果配置会让成绩立即或到当前时间可见，并且包含主观题：
   - 所有 submitted attempts 必须完成阅卷
   - 不存在 grading_status = pending 的主观题答案
6. 如果配置会让成绩立即或到当前时间可见，只在当前 tenant_id + exam_id 的已提交 / 已阅卷 attempt 范围内统计 need_review = true：
   - review_publish_policy = allow_with_warning 且 confirm_review_warning != true：返回 RESULT_REVIEW_WARNING
   - review_publish_policy = allow_with_warning 且 confirm_review_warning = true：允许继续发布
   - review_publish_policy = block：返回 RESULT_REVIEW_BLOCKED
7. 如果配置是未来公布时间或手动未公布，允许保存配置，但不改变学生可见性。
8. 写入 exams.publish_mode / exams.score_publish_time
```

---

# 7. 批量分配设计

## 7.1 支持的分配方式

首版支持两种：

```text
by_space          按班级/空间分配
by_attempt_even   按答卷数量平均分配
```

不做：

```text
by_question       按题分配，后续版本
double_review     双评
arbitration       仲裁
```

## 7.2 按班级/空间分配

场景：

```text
高一（1）班 → 张老师
高一（2）班 → 李老师
高一（3）班 → 王老师
```

请求示例：

```json
{
  "exam_id": 1001,
  "assign_mode": "by_space",
  "scope": {
    "space_ids": [101, 102, 103],
    "only_pending": true,
    "question_types": ["short_text"]
  },
  "space_rules": [
    {
      "space_id": 101,
      "grader_ids": [21]
    },
    {
      "space_id": 102,
      "grader_ids": [22]
    },
    {
      "space_id": 103,
      "grader_ids": [23]
    }
  ],
  "overwrite": false
}
```

处理逻辑：

```text
1. 校验操作者有当前考试的阅卷分配权限。
2. 校验 grader_ids 都是当前租户用户。
3. 校验 grader_ids 在对应 space 下具备 teacher 或 space_admin 身份。
4. 查询 exam_targets 命中的学生答卷。
5. 只取 status = submitted / graded 且包含 pending 主观题的 attempt。
6. 排除已完成 graded 且无 pending 的 attempt。
7. 如果 overwrite = false，跳过已有 active assignment 的 attempt。
8. 如果 overwrite = true，先将旧 assignment 改为 revoked，并在同一事务内把旧任务 `active_assignment_key` 改为 `inactive:<assignment_id>`，再创建新 assignment。
9. 一个 space 配多个老师时，按 attempt_id 轮询分配。
10. 批量插入 grading_assignments。
```

多空间命中的归属规则：

```text
1. grading_assignments 首版只允许一个 assignment.space_id。
2. 同一 attempt 在 assigned / grading 状态下只能有一个有效分配。
3. 按空间分配时，必须先为每个 attempt 解析 assignment_space_id。
4. 如果 attempt 只命中本次请求中的一个 space，assignment_space_id = 该 space。
5. 如果 attempt 命中本次请求中的多个 space，首版返回 GRADING_ASSIGNMENT_SCOPE_AMBIGUOUS。
6. 如果 attempt 仅来自用户直投且无法归属任何 space，space_id = 0，只允许 tenant_admin 通过 by_attempt_even 分配。
7. 预览接口必须返回 ambiguous_attempt_count 和 ambiguous_examples，不能到确认分配时才失败。
```

这样避免 A 空间和 B 空间的分配规则同时抢占同一份答卷，也避免用“最小 space_id”这类隐式规则把答卷分给错误老师。

`assignment_space_id` 是分配服务内的派生字段，不是首版新增落库字段。它的来源必须由后端根据 `exam_target_scope_spaces`、`exam_targets` 和 attempt 对应考生解析：

```text
1. 空间目标命中时，来自匹配的 exam_target_scope_spaces.space_id。
2. 用户目标命中且能唯一归属空间时，来自该用户在本考试目标范围内的唯一启用空间。
3. 用户直投且无法归属空间时，派生为 0，只允许 tenant_admin 通过 by_attempt_even 分配。
4. 同一 attempt 在本次分配范围内解析出多个候选空间时，进入 ambiguous_attempt_count，不落入任意一个空间。
```

如果后续决定把 `assignment_space_id` 落到 `exam_attempts` 或其他表以减少 join，必须作为独立迁移处理，补充来源、回填、更新事务和索引测试，不能把它写进 `ext_json`。

## 7.3 按答卷数量平均分配

场景：

```text
张老师、李老师、王老师共同批改高一年级所有待阅答卷。
```

请求示例：

```json
{
  "exam_id": 1001,
  "assign_mode": "by_attempt_even",
  "scope": {
    "space_ids": [101, 102],
    "only_pending": true,
    "question_types": ["short_text"]
  },
  "grader_ids": [21, 22, 23],
  "overwrite": false
}
```

处理逻辑：

```text
1. 查询范围内所有待阅 attempt。
2. 过滤已完成答卷。
3. 为每个 attempt 解析 assignment_space_id。
4. 为每个 grader 解析可评分空间集合。
5. 对每个 attempt 只在可评分该 assignment_space_id 的 grader 中选择 workload 最小者。
6. 如果没有任何 grader 可评分该 attempt，预览返回 unassignable_attempt_count 和 examples。
7. 按 pending 主观题数量倒序，或按 submitted_at 升序。
8. workload += 当前 attempt 的 pending 主观题数。
9. 插入 grading_assignments。
```

这比简单轮询更合理，因为不同答卷的待阅题数可能不同。

伪代码：

```go
type GraderLoad struct {
    GraderID uint64
    Load     int
}

sort.Slice(attempts, func(i, j int) bool {
    return attempts[i].PendingAnswerCount > attempts[j].PendingAnswerCount
})

for _, attempt := range attempts {
    candidates := filterGradersBySpace(loads, attempt.AssignmentSpaceID)
    if len(candidates) == 0 {
        markUnassignable(attempt)
        continue
    }
    g := pickMinLoadGrader(candidates)
    createAssignment(attempt.ID, g.GraderID)
    g.Load += attempt.PendingAnswerCount
}
```

注意：`by_attempt_even` 不是全局平均分给所有老师，而是在“老师有权批改该答卷所属空间”的前提下做负载均衡。不能把 A 空间答卷分配给只具备 B 空间 teacher 身份的老师。

## 7.4 分配覆盖规则

```text
overwrite = false：
- 已有 assigned / grading 的 attempt 不重新分配
- 返回 skipped_count

overwrite = true：
- 旧任务 status 改为 revoked
- 旧任务 active_assignment_key 改为 inactive:<assignment_id>
- 新建任务
- 如果旧任务已有部分评分，不删除评分
- 新老师继续处理未完成题目
```

注意：

```text
分配任务不应该清空已有 exam_answers.score。
```

---

# 8. 权限设计

当前权限模型中，`tenant_admin` 管租户内业务，`space_admin` 管指定空间业务，`teacher` 执行出题、组卷、发布考试、阅卷等教学动作，`student` 只参加考试和查看自己成绩，`platform_admin` 不默认进入租户业务。

## 8.1 角色权限矩阵

| 操作       | tenant_admin | space_admin |        teacher | student | platform_admin |
| -------- | -----------: | ----------: | -------------: | ------: | -------------: |
| 查看阅卷中心   |        是，本租户 |       是，本空间 |         是，授权空间 |       否 |              否 |
| 查看考试阅卷详情 |        是，本租户 |       是，本空间 |         是，授权空间 |       否 |              否 |
| 配置阅卷规则   |            是 |           是 | 建议是，限自己创建/授权考试 |       否 |              否 |
| 批量分配阅卷任务 |            是 |           是 | 首版默认否，可后续放开考试创建人 |       否 |              否 |
| 人工评分     |            是 |           是 |              是 |       否 |              否 |
| 修改他人评分   |            是 |           是 |       否，除非本人评分 |       否 |              否 |
| 修改成绩发布配置 |            是 |   是，授权空间 |       是，授权空间 |       否 |              否 |
| 导出成绩     |            是 |           是 |       否，后续按授权范围放开 |       否 |              否 |
| 查看自己成绩   |            否 |           否 |              否 |       是 |              否 |

权限方案中已经定义 `/api/v1/grading/**` 由 `tenant_admin / space_admin / teacher` 按考试归属和授权空间校验，`/api/v1/results/**` 管理端按范围查看，学生只能查看自己的成绩。

首版权限必须和现有后端保持一致：

```text
1. teacher 可以评分授权空间内的待阅答卷。
2. teacher 默认不能导出成绩或覆盖他人评分，但可以在授权目标空间内修改成绩发布配置。
3. tenant_admin 可以修改本租户考试的成绩发布配置；space_admin / teacher 只能修改后端解析后确认落在其授权目标空间内的考试配置。
4. 如果后续要允许 teacher 导出成绩、覆盖他人评分或执行空间级独立发布，必须作为单独权限变更补后端、前端和测试，不能只改前端按钮。
```

## 8.2 PermissionChecker 扩展

现有接口已有：

```go
CanGradeAttempt(ctx PermissionContext, attemptID uint64) error
CanViewExamResults(ctx PermissionContext, examID uint64) error
CanExportExamResults(ctx PermissionContext, examID uint64) error
```

建议补充：

```go
type PermissionChecker interface {
    CanManageGradingRule(ctx PermissionContext, examID uint64) error
    CanAssignGradingTask(ctx PermissionContext, examID uint64, spaceID uint64) error
    CanUpdateResultPublishConfig(ctx PermissionContext, examID uint64) error
    CanGradeAttempt(ctx PermissionContext, attemptID uint64) error
    CanViewExamResults(ctx PermissionContext, examID uint64) error
    CanExportExamResults(ctx PermissionContext, examID uint64) error
}
```

命名原则：

```text
1. 管理端成绩查看、导出权限统一使用 ExamResults 后缀，避免和学生自己的 result 混淆。
2. 学生查自己成绩继续走 CanViewOwnResult。
3. 阅卷 answer 级权限由 service 根据 answer -> attempt -> exam -> target spaces 反查真实归属后，构造带 space scope 的 PermissionContext，再调用 CanGradeAttempt。
4. 不把 answerID 直接传给 PermissionChecker，避免把数据库解析职责塞进固定角色 checker。
5. 如果代码中需要 canGradeAnswer / canUpdateGrade，必须是 grading service 内部私有方法，负责解析 answer 后复用 CanGradeAttempt，并额外判断 graded_by、管理员覆盖权限和审计写入。
6. 阅卷中心查看可以复用 CanViewExamResults，不新增 CanViewGradingCenter，避免和成绩查看权限形成重复入口。
```

## 8.3 权限校验原则

```text
1. API 层只解析登录态，不判断业务角色。
2. Service 层统一调用 PermissionChecker。
3. 不能信任前端传入的 tenant_id、space_id、actor_role。
4. attempt、answer、exam 的真实归属必须由后端从数据库解析。
5. teacher 只能操作授权空间内考试。
6. student 不能访问管理端阅卷接口。
7. platform_admin 不能直接阅卷。
```

技术方案也明确：阅卷和成绩 API 必须从登录态解析调用人身份，后端要根据作答记录、成绩行或考试目标解析真实空间归属，不能信任请求参数拼接授权范围。

---

# 9. 后端服务设计

## 9.1 落地边界

现有后端已经在 `server/internal/service/exam` 中实现了考试发布、待阅列表、简答题评分、学生查分、成绩导出和考试详情成绩管理。阅卷模块不能新建一套和现有 `ReviewService`、`ResultService`、`ExportService` 平行的业务链路。

首版落地按以下边界处理：

```text
1. 自动判分、提交、attempt 状态和学生查分继续属于 `server/internal/service/exam`。
2. 已存在的 ReviewService / ResultService / ExportService 先原地补强，不复制到新包。
3. 阅卷规则、评分标准、批量分配、阅卷流水属于新增能力，可以放入 `server/internal/service/grading`。
4. 成绩发布校验必须复用现有 exam service 的发布配置写入和操作日志能力，不再新增第二套 result publish repository。
5. DAO 仍集中在 `server/internal/dao/db`，不要在 service/grading 内直接写 GORM 查询。
6. 如果后续把 ReviewService 迁到 service/grading，必须作为独立重构提交，先补等价测试，再删除旧入口。
```

建议目录：

```text
server/internal/service/grading
├── rule_service.go
├── assignment_service.go
├── grading_log_service.go
└── dto.go

server/internal/service/exam
├── review_service.go          # 原地扩展评分保存、answer 解析和学生可见性联动
├── result_service.go          # 原地补齐 graded / pending answer 可见性
└── export_service.go          # 继续承载成绩列表和导出权限裁剪

server/internal/dao/db
├── grading_rule_repository.go
├── grading_rubric_repository.go
├── grading_assignment_repository.go
├── grading_log_repository.go
└── exam_repository.go         # 保留 attempt / answer / result / operation log 共享查询
```

## 9.2 Service 接口

```go
type GradingService interface {
    GetGradingDashboard(ctx context.Context, req GetGradingDashboardReq) (*GradingDashboardResp, error)
    GetExamGradingDetail(ctx context.Context, req GetExamGradingDetailReq) (*ExamGradingDetailResp, error)

    GetGradingRule(ctx context.Context, examID uint64) (*GradingRuleResp, error)
    SaveGradingRule(ctx context.Context, req SaveGradingRuleReq) error

    PreviewAssignment(ctx context.Context, req PreviewAssignmentReq) (*AssignmentPreviewResp, error)
    BatchAssign(ctx context.Context, req BatchAssignReq) (*BatchAssignResp, error)

    GetMyGradingTasks(ctx context.Context, req GetMyGradingTasksReq) (*PagedResp[GradingTaskItem], error)
    GetAttemptForGrading(ctx context.Context, req GetAttemptForGradingReq) (*AttemptGradingResp, error)

    GradeAnswer(ctx context.Context, req GradeAnswerReq) error
    CompleteAttemptGrading(ctx context.Context, attemptID uint64) error

}
```

成绩发布配置和显式发布确认不属于 `GradingService` 本体。实现时应放在现有 `server/internal/service/exam` 的考试/成绩服务编排中，复用 `UpdateScorePublishConfig`、操作日志和学生查分可见性判断。`service/grading` 只提供 pending、need_review、评分规则等发布前校验所需的数据能力。

`PublishResultResp` 至少包含：

```go
type PublishResultResp struct {
    Published         bool   // 是否已经完成发布。
    WarningCode       string // 需要二次确认时返回 RESULT_REVIEW_WARNING。
    WarningMessage    string // 前端展示用 warning。
    ReviewAnswerCount int    // 当前仍标记 need_review 的答案数量。
}
```

如果实现更倾向统一错误模型，也可以用带 `code/data` 的领域错误承载 warning 数据，但不能只返回
`error` 字符串，否则 handler 无法稳定返回 `review_answer_count`。

---

# 10. 核心业务流程

## 10.1 考生提交后的判分流程

提交答卷时：

```text
1. attempt 从 in_progress 更新为 submitted。
2. 查询 attempt_questions。
3. 查询 exam_answers。
4. 对客观题执行自动判分。
5. 对填空题执行自动判分。
6. 对简答题写入 grading_status = pending。
7. 汇总 objective_score。
8. subjective_score 暂为 0 或已评分部分之和。
9. total_score = objective_score + subjective_score。
10. 如果没有 pending 主观题：
    - attempt.status = graded
    - 如果 publish_mode = immediate_score，可立即对学生可见。
```

注意：

```text
客观题判分必须使用快照内选项 ID，不使用 A/B/C/D。
```

现有技术方案也明确选择题判分基于选项 ID 或快照内选项 ID，不基于 A/B/C/D。

---

## 10.2 保存单题评分流程

接口：

```http
POST /api/v1/grading/answers/:answer_id/grade
```

分数字段类型：

```text
1. 后端内部和数据库继续以 decimal 字符串表达分数，避免 float 精度误差影响汇总、导出和断言。
2. API 请求的 `score`、`rubric_scores[].score` 统一接收十进制字符串；兼容旧评分入口也保持字符串入参，避免同一阅卷链路同时支持 number 和 string 两套解析。
3. API 响应中的 `score`、`objective_score`、`subjective_score`、`total_score` 统一返回字符串。
4. 前端展示前按字符串格式化，不用 JavaScript number 参与最终成绩计算。
5. 如果未来要支持 JSON number 或把响应改成 number，必须同步修改所有 API client、页面、导出和测试，不能只改阅卷接口。
```

请求：

```json
{
  "answer_version": 3,
  "score": "12",
  "comment": "步骤较完整，但归纳证明不严谨",
  "need_review": false,
  "rubric_scores": [
    {
      "key": "step_complete",
      "score": "5"
    },
    {
      "key": "formula_correct",
      "score": "5"
    },
    {
      "key": "reasoning_valid",
      "score": "2"
    },
    {
      "key": "conclusion_strict",
      "score": "0"
    }
  ]
}
```

事务流程：

```text
1. 根据 answer_id 查询 exam_answer。
2. 查询 attempt、exam、attempt_question。
3. 校验 tenant_id。
4. service 解析 answer 对应 attempt 和授权空间后，调用 CanGradeAttempt。
5. 校验 answer.grading_status 不是 auto。
6. 校验 answer.version == answer_version。
7. 查询 grading_rules。
8. 校验分数范围和精度。
9. 校验评语规则。
10. 更新 exam_answers：
    - score
    - grading_status = graded
    - graded_by = 当前用户
    - graded_at = now
    - grader_comment
    - updated_at
    - updated_by
    - version + 1
    - rubric_scores_json
    - need_review
11. 写 grading_logs。
12. 重算 attempt 分数。
13. 如果该 attempt 没有 pending 主观题：
    - attempt.status = graded
    - 对应 grading_assignment.status = completed
    - 对应 grading_assignment.active_assignment_key = inactive:<assignment_id>
14. 提交事务。
```

评分字段权威关系：

```text
1. exam_answers.score 是成绩汇总、发布和导出的唯一分数字段。
2. rubric_scores_json 是评分项明细，用于展示和审计，不参与数据库过滤、排序、发布校验或成绩汇总。
3. 请求携带 rubric_scores 时，后端必须校验每个 key 属于当前 rubric，单项分不越界。
4. 请求携带 rubric_scores 时，后端必须校验评分项合计等于 score；不一致时返回 GRADING_RUBRIC_SCORE_MISMATCH。
5. 如果后续改为由 rubric_scores 自动计算总分，必须同步修改 API 契约，让前端不再提交 score。
```

分数重算：

```sql
objective_score = SUM(score WHERE grading_status = 'auto')
subjective_score = SUM(score WHERE grading_status IN ('graded'))
total_score = objective_score + subjective_score
```

注意：

```text
前端不能传 subjective_score 或 total_score。
```

---

## 10.3 阅卷任务开始流程

老师进入某份答卷：

```text
1. 查询 assignment。
2. 校验 grader_id 是否当前用户，或当前用户是否有管理权限。
3. 如果 assignment.status = assigned：
   - 更新为 grading
   - started_at = now
4. 返回答卷快照、答案、评分标准、历史评分记录。
```

如果没有分配任务，是否允许进入？

首版建议：

```text
space_admin / tenant_admin：允许进入
teacher：如果考试没有启用分配，则允许进入授权空间内待阅答卷；如果启用分配，则默认只允许进入自己的任务。
```

`assignment_required` 保存为 `grading_rules.assignment_required` 显式列。

默认规则：

```text
1. 考试尚未执行过批量分配时，assignment_required = false。
2. 批量分配首次成功后，assignment_required = true。
3. assignment_required = true 后，普通 teacher 只能进入自己的 assigned / grading 任务。
4. tenant_admin / space_admin 可绕过 assignment_required 进入授权范围内答卷，但所有改分仍必须写审计日志。
```

---

## 10.4 完成答卷阅卷

触发时机：

```text
1. 老师点击“提交成绩”
2. 或保存最后一道 pending 主观题后自动触发
```

逻辑：

```text
1. 查询 attempt 下是否还有 pending。
2. 如果有 pending，禁止完成。
3. 重算分数。
4. attempt.status = graded。
5. 如果存在 active assignment：
   - assignment.status = completed。
   - assignment.active_assignment_key = inactive:<assignment_id>。
6. 写 exam_operation_logs。
```

---

## 10.5 成绩发布流程

接口：

```http
POST /api/v1/results/exams/:exam_id/publish
```

请求：

```json
{
  "publish_mode": "manual_publish",
  "score_publish_time": 1727222400,
  "confirm_review_warning": false
}
```

流程：

```text
1. 查询 exam。
2. 调用成绩发布配置写权限校验。
3. 后端从考试目标解析真实授权空间；tenant_admin 可操作本租户考试，space_admin / teacher 只能操作命中其授权空间的考试。
4. 查询 grading_rules。
5. 如果本次配置会让成绩立即或已到当前时间可见，并且包含主观题：
   - 校验不存在 pending
   - 校验所有 submitted attempts 已 graded
6. 如果本次配置会让成绩立即或已到当前时间可见，按当前 tenant_id + exam_id 下已提交 / 已阅卷 attempt 范围查询 exam_answers.need_review 复核标记数量。
7. 如果 need_review > 0：
   - review_publish_policy = block：返回 RESULT_REVIEW_BLOCKED，禁止发布。
   - review_publish_policy = allow_with_warning 且 confirm_review_warning = false：返回 RESULT_REVIEW_WARNING。
   - review_publish_policy = allow_with_warning 且 confirm_review_warning = true：继续发布。
8. 如果本次配置是未来公布时间或手动未公布，保存配置但学生仍不可见。
9. 更新 exams.publish_mode / score_publish_time。
10. 写 exam_operation_logs。
```

`RESULT_REVIEW_WARNING` 响应必须包含可展示的 warning 信息，前端二次确认后用同一个接口重新提交，
但必须把 `confirm_review_warning` 改为 `true`。

示例：

```json
{
  "code": "RESULT_REVIEW_WARNING",
  "message": "仍有 3 道主观题被标记为需要复核，确认后可继续发布",
  "data": {
    "review_answer_count": 3,
    "confirm_field": "confirm_review_warning"
  }
}
```

如果第二次请求仍未携带 `confirm_review_warning = true`，后端必须继续返回
`RESULT_REVIEW_WARNING`，不能自动放行。

不新增考试成绩发布状态字段：

```text
result_status 只作为查询响应中的派生字段，不落库。
派生规则见 1.1 成绩发布模型。
```

---

# 11. API 设计

## 11.1 阅卷中心列表

```http
GET /api/v1/grading/exams
```

Query：

```text
exam_id?
space_id?
grading_progress_status?  pending / grading / completed
result_status?   pending_grading / unpublished / scheduled / published
keyword?
page
page_size
```

Response：

```json
{
  "items": [
    {
      "exam_id": 1001,
      "exam_name": "高一数学月考（2024-09）",
      "space_id": 101,
      "space_name": "高一（1）班",
      "submitted_count": 56,
      "pending_answer_count": 128,
      "graded_answer_count": 320,
      "progress": 71,
      "result_status": "unpublished",
      "updated_at": 1726885080
    }
  ],
  "page": 1,
  "page_size": 20,
  "total": 7
}
```

`result_status` 为后端派生展示字段，不对应独立数据库列。

## 11.2 考试阅卷详情

```http
GET /api/v1/grading/exams/:exam_id/detail
```

Response：

```json
{
  "exam": {
    "id": 1001,
    "name": "高一数学月考（2024-09）",
    "paper_name": "高一数学月考试卷",
    "start_time": 1726876800,
    "end_time": 1726884000,
    "duration_minutes": 120,
    "publish_mode": "manual_publish"
  },
  "summary": {
    "target_count": 1248,
    "submitted_count": 980,
    "pending_subjective_count": 652,
    "graded_subjective_count": 328,
    "progress": 68,
    "result_status": "unpublished"
  },
  "space_progress": [
    {
      "space_id": 101,
      "space_name": "高一（1）班",
      "target_count": 56,
      "submitted_count": 54,
      "pending_count": 18,
      "progress": 68,
      "grader_count": 12
    }
  ]
}
```

## 11.3 获取阅卷规则

```http
GET /api/v1/grading/exams/:exam_id/rule
```

## 11.4 保存阅卷规则

```http
POST /api/v1/grading/exams/:exam_id/rule
```

Request：

```json
{
  "grading_mode": "by_attempt",
  "manual_question_types": ["short_text"],
  "allow_decimal_score": false,
  "score_precision": "integer",
  "comment_required": false,
  "zero_score_comment_required": false,
  "full_score_comment_required": false,
  "allow_mark_review": true,
  "assignment_required": false,
  "review_publish_policy": "allow_with_warning",
  "rubrics": [
    {
      "question_id": 3001,
      "rubric_scope_key": "section:2001:question:3001:sort:21:score:15",
      "rubric_title": "第 21 题评分标准",
      "total_score": "15",
      "items": [
        {
          "key": "step_complete",
          "name": "步骤完整",
          "score": "5"
        }
      ]
    }
  ]
}
```

## 11.5 预览批量分配

```http
POST /api/v1/grading/exams/:exam_id/assignments/preview
```

## 11.6 确认批量分配

```http
POST /api/v1/grading/exams/:exam_id/assignments/batch
```

## 11.7 我的阅卷任务

```http
GET /api/v1/grading/my-tasks
```

Query：

```text
exam_id?
space_id?
status?          assigned / grading / completed / revoked
page
page_size
```

## 11.8 获取答卷阅卷数据

```http
GET /api/v1/grading/exams/:exam_id/attempts/:attempt_id
```

读取答卷阅卷数据必须同时携带 `exam_id` 和 `attempt_id`。后端查询条件必须同时使用
`tenant_id + exam_id + attempt_id`，不能只凭 `attempt_id` 跨考试读取答卷。

Response：

```json
{
  "attempt": {
    "id": 9001,
    "student_name": "张宇",
    "space_name": "高一（1）班",
    "status": "submitted",
    "objective_score": "82",
    "subjective_score": "12",
    "total_score": "94"
  },
  "questions": [
    {
      "attempt_question_id": 8001,
      "question_id": 3001,
      "sort_order": 21,
      "question_type": "short_text",
      "score": "15",
      "question_snapshot": {},
      "answer": {
        "answer_id": 7001,
        "answer_content": "……",
        "score": "12",
        "grading_status": "graded",
        "graded_by": 21,
        "grader_comment": "步骤较完整"
      },
      "rubric": {}
    }
  ]
}
```

## 11.9 保存评分

```http
POST /api/v1/grading/answers/:answer_id/grade
```

## 11.10 发布成绩

```http
POST /api/v1/results/exams/:exam_id/publish
```

Request：

```json
{
  "publish_mode": "manual_publish",
  "score_publish_time": 1727222400,
  "confirm_review_warning": false
}
```

字段说明：

```text
publish_mode：manual_publish / immediate_score。包含主观题时只能 manual_publish。
score_publish_time：Unix 秒；立即发布时写当前秒级时间或由后端统一落当前秒。
confirm_review_warning：存在 need_review 且策略为 allow_with_warning 时的二次确认标记。
```

可能响应：

```text
200 OK：发布成功。
RESULT_HAS_PENDING_GRADING：仍有待阅主观题。
RESULT_REVIEW_WARNING：存在复核标记，允许二次确认后发布。
RESULT_REVIEW_BLOCKED：存在复核标记且规则禁止发布。
RESULT_PUBLISH_FORBIDDEN：无发布权限。
```

---

# 12. 前端页面设计

## 12.1 阅卷中心列表页

功能：

```text
- 展示所有可阅卷考试/班级任务
- 过滤考试、班级、状态、老师
- 查看进度
- 进入详情
- 进入工作台
```

核心组件：

```text
- SummaryCards
- GradingExamTable
- MyTodoList
- AssignmentProgressPanel
```

## 12.2 考试阅卷详情页

Tab：

```text
总览
答卷列表
按题阅卷，首版可只读或隐藏
阅卷分配
发布设置
```

## 12.3 阅卷设置抽屉

Tabs：

```text
基础设置
评分限制
评分标准
发布设置
```

## 12.4 批量分配抽屉

步骤：

```text
1. 选择范围
2. 选择分配方式
3. 选择阅卷老师
4. 预览结果
5. 确认分配
```

## 12.5 阅卷工作台

三栏布局：

```text
左侧：题目导航
中间：题目内容 + 学生答案 + 作答记录
右侧：评分标准 + 评分面板 + 历史评分
```

---

# 13. 事务与并发控制

## 13.1 保存评分必须用事务

事务内完成：

```text
1. 锁定 exam_answers 当前行。
2. 校验 version。
3. 更新答案分数。
4. 写 grading_logs。
5. 重算 attempt 分数。
6. 更新 assignment 状态；如果变为 completed / revoked，必须同步释放 active_assignment_key。
7. 必要时更新 attempt.status。
```

## 13.2 乐观锁

请求必须带：

```json
{
  "answer_version": 3
}
```

更新条件：

```sql
WHERE id = ?
  AND tenant_id = ?
  AND version = ?
```

更新行数为 0 时返回：

```text
答案已被其他阅卷人更新，请刷新后重试
```

## 13.3 避免重复评分

如果启用分配：

```text
teacher 只能评分自己 assignment 下的 answer。
```

如果未启用分配：

```text
teacher 可评分授权空间内 pending answer。
```

一旦 `graded_by` 不是当前老师：

```text
普通 teacher 不允许覆盖。
space_admin / tenant_admin 可以覆盖，但必须写 grading_logs。
```

---

# 14. 查询设计

## 14.1 待阅答卷查询

伪 SQL：

```sql
SELECT a.*
FROM exam_attempts a
WHERE a.tenant_id = ?
  AND a.exam_id = ?
  AND a.status IN ('submitted', 'graded')
  AND EXISTS (
      SELECT 1
      FROM exam_answers ans
      JOIN exam_attempt_questions q
        ON q.id = ans.attempt_question_id
      WHERE ans.tenant_id = a.tenant_id
        AND ans.attempt_id = a.id
        AND ans.grading_status = 'pending'
  )
ORDER BY a.submitted_at ASC
LIMIT ? OFFSET ?;
```

## 14.2 我的任务查询

```sql
SELECT ga.*
FROM grading_assignments ga
WHERE ga.tenant_id = ?
  AND ga.grader_id = ?
  AND ga.status IN ('assigned', 'grading')
ORDER BY ga.assigned_at ASC
LIMIT ? OFFSET ?;
```

## 14.3 阅卷进度统计

按考试：

```text
submitted_count
pending_subjective_count
graded_subjective_count
progress = graded / (pending + graded)
```

注意：

```text
progress 按主观题数量算，比按答卷数量更准确。
```

---

# 15. 和现有考试流程的关系

## 15.1 考试发布

发布考试时：

```text
1. 检查试卷是否包含 short_text。
2. 如果包含：
   - exams.publish_mode = manual_publish
   - 禁止 max_attempts > 1
   - 创建 grading_rules
3. 如果不包含：
   - 允许 immediate_score
```

技术方案已明确：包含简答题时，首版不允许立即出分，也不允许多次作答。

## 15.2 答题提交

提交后：

```text
1. 关闭 exam_token 写入。
2. 自动判客观题。
3. 简答题置 pending。
4. 如果无 pending，attempt 直接 graded。
```

## 15.3 成绩查询

学生查成绩时：

```text
1. 只能查看自己的成绩。
2. 如果 attempt.status != graded，返回“成绩尚未公布”。
3. 如果存在 grading_status = pending 的主观题答案，返回“成绩尚未公布”。
4. 如果 publish_mode = manual_publish 且 score_publish_time 为空或为 0，返回“成绩尚未公布”。
5. 如果 score_publish_time > now，返回公布时间。
6. 发布后返回成绩、题目、答案、解析。
```

---

# 16. 错误码建议

```text
GRADING_RULE_NOT_FOUND
GRADING_RULE_INVALID
GRADING_ASSIGNMENT_NOT_FOUND
GRADING_ASSIGNMENT_CONFLICT
GRADING_ASSIGNMENT_OVERWRITE_REQUIRED
GRADING_ASSIGNMENT_SCOPE_AMBIGUOUS
GRADING_ANSWER_NOT_FOUND
GRADING_ANSWER_VERSION_CONFLICT
GRADING_SCORE_OUT_OF_RANGE
GRADING_SCORE_PRECISION_INVALID
GRADING_RUBRIC_SCORE_MISMATCH
GRADING_COMMENT_REQUIRED
GRADING_PERMISSION_DENIED
GRADING_ATTEMPT_NOT_SUBMITTED
GRADING_ATTEMPT_ALREADY_COMPLETED
GRADING_PENDING_ANSWER_EXISTS
RESULT_PUBLISH_FORBIDDEN
RESULT_HAS_PENDING_GRADING
RESULT_REVIEW_WARNING
RESULT_REVIEW_BLOCKED
```

---

# 17. 测试用例

## 17.1 阅卷规则

```text
- 发布含简答题试卷时自动创建 grading_rules
- 含简答题时 publish_mode 不能为 immediate_score
- 含简答题时 max_attempts > 1 发布失败
- score_precision = integer 时保存 12.5 分失败
- allow_decimal_score = false 且 score_precision = half / one_decimal 时保存规则失败
- allow_decimal_score = true 且 score_precision = half / one_decimal 时保存规则成功
- score > 题目满分失败
- rubric_scores 中存在未知 key 时保存失败
- rubric_scores 单项分超过评分项上限时保存失败
- rubric_scores 合计和 score 不一致时返回 GRADING_RUBRIC_SCORE_MISMATCH
- 无 grading_rubrics 时允许 rubric_scores 为空或省略，并保存 rubric_scores_json = []
- 有 grading_rubrics 时 rubric_scores 必须覆盖全部评分项
- comment_required = true 时空评语失败
- zero_score_comment_required = true 且 0 分空评语失败
- exam_answers.need_review 和 rubric_scores_json 正确保存和读取
- 自动保存、提交判分和人工阅卷创建 exam_answers 时都显式写入 rubric_scores_json = []
- 发布校验按 need_review 显式列查询，不依赖 ext_json JSON key
- 发布校验只统计当前 tenant_id + exam_id 范围内的 need_review 答案
- 发布校验使用 tenant_id + attempt_id + need_review 索引收敛复核答案
- rubric_scope_key 支持同一 question_id 在不同大题或不同分值下配置不同评分标准
- rubric_scope_key 使用统一机器可解析格式 `section:<id>:question:<id>:sort:<n>:score:<score>`
- grading_rules.assignment_required 保存和读取为显式列，不依赖 ext_json
- grading_rules 不保存 publish_mode / score_publish_time
```

## 17.2 批量分配

```text
- 按班级分配成功
- 按答卷数量平均分配成功
- 按答卷数量平均分配时，老师只能收到自己有权评分空间内的 attempt
- 没有可用老师的 attempt 在预览中进入 unassignable_attempt_count
- 已分配任务 overwrite=false 时跳过
- overwrite=true 时旧任务 revoked，新任务 assigned
- 并发分配同一 attempt 时只有一个 active_assignment_key 可以插入成功
- revoked / completed 历史任务改为 inactive key 后允许重新分配
- attempt 同时命中本次请求中的多个 space 时预览返回 ambiguous_attempt_count
- ambiguous attempt 直接确认分配失败并返回 GRADING_ASSIGNMENT_SCOPE_AMBIGUOUS
- assignment_space_id 只作为服务层派生字段使用，不落入 ext_json
- 仅用户直投且无法归属空间的 attempt 只能由 tenant_admin 通过 by_attempt_even 分配
- 被禁用老师不能被分配
- 非授权空间老师不能被分配
- 已完成答卷不参与分配
```

## 17.3 阅卷评分

```text
- teacher 可以评分自己授权空间内答卷
- teacher 不能评分其他空间答卷
- student 不能访问阅卷接口
- platform_admin 不能访问阅卷接口
- 获取答卷阅卷数据必须同时校验 exam_id 和 attempt_id，不能只凭 attempt_id 读取
- answer_version 冲突时保存失败
- 保存评分后 answer 变 graded
- 保存最后一道主观题后 attempt 变 graded
- 重算 objective_score / subjective_score / total_score 正确
- 修改他人评分时普通 teacher 失败
- space_admin 修改评分成功并写 grading_logs
- 保存评分同时写 grading_logs 和 exam_operation_logs
- 多空间作答的评分日志按空间写多行，且共享同一个 operation_group_id
- 新写入 grading_logs / exam_operation_logs 时 operation_group_id 必须非空
- 若存在旧日志数据，单独修复 exam_operation_logs.operation_group_id；缺失时按单条日志生成唯一 group
- tenant_admin 按 operation_group_id 聚合后只展示一次评分动作
- space_admin 按 space_id 裁剪后只能看到本空间日志
- 单题历史评分按 answer_id 和 attempt_question_id 查询命中 grading_logs 索引
```

## 17.4 成绩发布

```text
- 存在 pending 主观题时不能保存会立即可见的发布配置
- 全部 graded 后可以保存会立即可见的发布配置
- 保存发布配置不会绕过 pending 校验让学生提前查到部分分数
- result_status 按 pending_grading -> unpublished -> scheduled -> published 顺序派生
- immediate_score 遇到 pending 主观题时仍派生为 pending_grading
- result_status 按调用人授权范围派生；A 空间 pending 不影响 B 空间管理员看到 B 空间已完成
- 成绩发布是整场考试全局动作，不支持空间级独立发布
- 授权目标空间内的 space_admin / teacher 可以修改成绩发布配置
- 无目标空间权限的 space_admin / teacher 修改配置失败并返回 RESULT_PUBLISH_FORBIDDEN
- need_review + allow_with_warning 且未确认时返回 RESULT_REVIEW_WARNING
- need_review + allow_with_warning 且 confirm_review_warning = true 时允许发布
- need_review + block 时返回 RESULT_REVIEW_BLOCKED
- 成绩发布配置只更新 exams.publish_mode / score_publish_time，不回写 grading_rules
- 保存发布配置只写 exam_operation_logs，不写 grading_logs
- score_publish_time 未到时学生不可见
- score_publish_time 到达后学生可见
- 学生只能看自己的成绩
```

## 17.5 时间戳单位

```text
- 所有新 API 请求和响应时间戳都是 Unix 秒
- 所有新增表 created_at / updated_at / assigned_at / graded_at 等字段都是 Unix 秒
- 前端提交 score_publish_time 时使用秒级值
- 旧毫秒 fixture 迁移为秒后，学生查分、阅卷列表和日志时间展示仍正确
- 后端测试中不得混用 13 位毫秒时间戳和 10 位秒级时间戳
```

## 17.6 结构化字段与 `ext_json`

```text
- 参与计算、排序、过滤、权限、状态流转、发布校验、分配控制、唯一性、幂等、审计聚合、去重或分页的字段不能落入 ext_json
- grading_rules.ext_json / grading_rubrics.ext_json / grading_assignments.ext_json / grading_logs.ext_json 只保存展示类元数据
- exam_operation_logs.operation_group_id 是显式列，日志聚合、去重和分页不读取 ext_json.operation_group_id
- 所有新增表 insert 路径显式写 ext_json = {}，不能依赖 MySQL JSON DEFAULT
- 冗余 exam_id / attempt_id / attempt_question_id / answer_id / space_id 等查询字段时，必须同事务写入并覆盖查询索引测试
- Review 迁移脚本、DO 结构体、Columns 映射、repository 创建路径时，都要检查核心字段没有被塞进 ext_json
```

---

# 18. Codex 开发任务拆分

## Task 0：时间戳秒级迁移

实现：

```text
1. 后端 `now` 统一从 UnixMilli 改为 Unix。
2. exams / exam_attempts / exam_answers / exam_events / exam_operation_logs / BaseFields / users.last_login_at / platform_users.last_login_at 的历史毫秒数据幂等迁移为秒。
3. 前端 API client、页面格式化、mock 和测试 fixture 改为秒级时间戳。
4. 边界校验拒绝 13 位毫秒时间戳，避免迁移后继续写入旧单位。
```

交付物：

```text
1. 时间戳字段清单，逐表列出字段名、是否允许 NULL、是否允许 0、是否需要历史转换。
2. 目标结构 SQL 和数据修复说明；历史时间戳转换条件统一为 value >= 1000000000000。
3. Go service / repository / handler 的时间比较和 DTO 测试。
4. 前端时间格式化和 fixture 迁移测试。
```

## Task 1：迁移表

新增：

```text
exam_answers.need_review
exam_answers.rubric_scores_json
grading_rules
grading_rubrics
grading_assignments
grading_logs
```

迁移注意：

```text
1. grading_rules 必须包含 assignment_required 显式列。
2. exam_answers 新增 rubric_scores_json 后，所有新建答案路径都要显式写 []。
3. exam_answers 必须补充 tenant_id + attempt_id + need_review 索引；如果后续冗余 exam_id，则同步补 tenant_id + exam_id + need_review 索引。
4. grading_logs 必须同步创建考试、答案、空间、operation_group_id 和 attempt_question 维度索引。
5. grading_logs.operation_group_id 新写入必须非空，不能用 DEFAULT '' 隐藏漏写。
6. exam_operation_logs 必须把 operation_group_id 设计为显式列，并补充 tenant_id + exam_id + operation_group_id 查询索引。
7. 当前基准 SQL 不写历史回填；若运行环境已有旧日志数据，再单独按 ext_json.operation_group_id 或单条唯一 group 规则修复。
8. 所有新增 ext_json 字段只允许承载展示类扩展元数据；参与核心业务流程的字段必须显式列化。
9. 所有新增表 insert 路径都必须显式写 ext_json = {}，不能依赖 MySQL JSON DEFAULT。
10. 为减少 join 冗余的 exam_id、attempt_id、attempt_question_id、answer_id、space_id 等字段，必须明确来源表和事务内同步路径。
11. assignment_space_id 首版是服务层派生字段；若决定落库，必须作为独立迁移补来源、回填、索引和一致性测试。
```

不新增：

```text
exams.result_status
```

`result_status` 只作为查询响应派生字段，避免和 `publish_mode + score_publish_time` 双写。

## Task 2：Repository

新增：

```text
GradingRuleRepo
GradingRubricRepo
GradingAssignmentRepo
GradingLogRepo
```

现有仓储边界：

```text
1. 继续复用 ExamRepository 中已有的 attempt、answer、result、export、operation log 查询。
2. 不为同一张 exam_attempts / exam_answers 再造一套平行 repository。
3. 旧路径参数适配可以在 handler 做，但评分保存、成绩发布、学生可见性必须进入同一套 service 和 repository 方法。
```

补充：

```text
ExamAttemptRepo.ListPendingGradingAttempts
ExamAnswerRepo.ListPendingByAttempt
ExamAnswerRepo.UpdateGradeWithVersion
ExamAttemptRepo.RecalculateScore
ResultRepo.DeriveResultStatusByActorScope
ResultRepo.GetVisibleResultSnapshotWithStatus
ResultRepo.CountPendingAnswersByAttempt
```

## Task 3：PermissionChecker 扩展

新增：

```go
CanManageGradingRule
CanAssignGradingTask
CanUpdateResultPublishConfig
```

不新增：

```text
CanGradeAnswer
CanUpdateGrade
```

answer 级权限由 grading service 解析 answer -> attempt -> scope 后复用 `CanGradeAttempt`；修改他人评分的判断留在 service 内部完成。

## Task 4：阅卷规则服务

实现：

```text
GetGradingRule
SaveGradingRule
InitDefaultGradingRuleOnExamPublish
```

## Task 5：分配服务

实现：

```text
PreviewAssignment
BatchAssignBySpace
BatchAssignByAttemptEven
RevokeAssignment
```

实现边界：

```text
1. BatchAssignByAttemptEven 必须按服务层派生出的 assignment_space_id 过滤可选老师。
2. 预览接口必须返回 ambiguous_attempt_count 和 unassignable_attempt_count。
3. 确认分配时如果仍存在 ambiguous / unassignable attempt，必须显式失败。
```

## Task 6：阅卷服务

实现：

```text
GetGradingDashboard
GetExamGradingDetail
GetMyTasks
GetAttemptForGrading
GradeAnswer
CompleteAttemptGrading
```

## Task 7：成绩发布配置与学生可见性服务

实现：

```text
ValidateBeforePublish
UpdateResultPublishConfig
GetStudentResultVisibility
```

实现边界：

```text
1. 成绩发布配置是整场考试全局配置，只写 exams.publish_mode / exams.score_publish_time。
2. 成绩发布配置写权限以长期技术方案为准：tenant_admin 可改本租户考试，space_admin / teacher 可改后端解析后确认落在其授权目标空间内的考试。
3. 不支持空间级独立发布；如要支持必须新增发布作用域表。
4. 配置会让成绩立即或到当前时间可见时，need_review + allow_with_warning 必须先返回 RESULT_REVIEW_WARNING。
5. 二次确认请求必须携带 confirm_review_warning = true，后端才允许保存会立即生效的配置。
6. 配置会让成绩立即或到当前时间可见时，need_review + block 必须返回 RESULT_REVIEW_BLOCKED。
7. 保存发布配置只写 exam_operation_logs，不写 grading_logs。
8. 学生查分必须拒绝 attempt.status != graded 的作答。
9. 学生查分必须拒绝仍存在 pending 主观题答案的作答，即使 score_publish_time 已到。
10. latest / highest 只在已提交、已阅卷完成且当前时间可见的 attempts 中选择。
11. `/api/v1/results/publish-config` 兼容入口必须复用同一套权限、可见性和审计逻辑；保存未来公布时间或未公布配置时可以不阻断，但不能让学生提前可见。
```

## Task 8：前端页面

实现：

```text
阅卷中心列表页
考试阅卷详情页
阅卷设置抽屉
批量分配抽屉
阅卷工作台
成绩发布配置沿用已实现入口；考试详情成绩管理列表不新增“配置成绩发布”入口
```

---

# 19. 最终建议

阅卷模块首版按这个模型落地最稳：

```text
阅卷规则：
- 考试级配置
- 控制评分限制、评分标准、发布前复核策略

试卷分配：
- 首版按班级/空间、按答卷平均分配
- 用 grading_assignments 表表达任务
- 不做按题分配、双评、仲裁

阅卷评分：
- 分数落 exam_answers
- 汇总落 exam_attempts
- 单题评分和复核写 grading_logs
- 管理端动作写 exam_operation_logs

权限：
- tenant_admin 管本租户
- space_admin 管本空间
- teacher 管授权空间
- student 只能查自己成绩
- platform_admin 不直接进入租户阅卷业务
```

一句话：**`grading_rules` 决定怎么判，`grading_assignments` 决定谁来判，`exam_answers` 保存判分结果，`grading_logs` 保证单题评分可追溯，`exam_operation_logs` 保证管理动作可追溯。**

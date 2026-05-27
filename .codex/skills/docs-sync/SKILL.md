---
name: docs-sync
description: Use when PaperMind changes affect docs, execution checklist progress, API/database/deployment behavior, frontend/backend workflows, or long-lived project rules.
---

# Docs Sync for PaperMind

## 目标

在 PaperMind 的代码、配置、部署、测试或业务规则发生变化后，及时同步中文文档、执行清单、部署说明和长期规则，避免“代码已经变了，文档还停在旧状态”。

本 skill 只负责文档同步与规则回写，不负责替代代码实现、测试实现或代码审查。

## PaperMind 文档地图

优先按下面的归属关系更新，不要机械改所有文档。

- `docs/2026-05-25-papermind-execution-checklist.md`
  - 阶段任务清单、进度回写、Review Gate、未完成风险。
  - 只有代码和必要验证都完成时，才能把 `[ ]` 改为 `[x]`。

- `docs/2026-05-25-papermind-exam-platform-technical-design.md`
  - 技术方案、业务规则、接口契约、数据库设计、权限边界、部署策略、测试策略。
  - 只有业务行为、接口、数据库模型、部署方式或长期技术决策变化时才更新。

- `deployments/docker-compose/README.md`
  - PostgreSQL + server + web/Nginx 的 Compose 部署方式。
  - Dockerfile、compose、端口、环境变量、Nginx、启动/验证步骤变化时更新。

- `deployments/sqlite-single-node/README.md`
  - SQLite 单机模式、DSN、适用边界、低并发限制。
  - SQLite 配置、初始化、`json1`、`foreign_keys`、单机部署步骤变化时更新。

- `README.md`
  - 项目入口说明。
  - 只在启动方式、主要能力、目录结构或部署入口发生用户可见变化时更新。

- `web/README.md`
  - 前端开发、构建、测试和本地运行说明。
  - 只在 Vite/React/npm 命令、环境变量、前端启动方式变化时更新。

- `AGENTS.md` / `AGENTS.override.md`
  - 长期协作规则、项目级编码规则、验证规则。
  - 只有规则本身需要长期沉淀时才更新；不要把一次性任务经验写进去。

## 触发条件

满足任一条件时使用本 skill：

- `server/` 中 API、service、dao、migration、config、bootstrap、错误码或部署相关行为变化。
- `web/` 中真实 API 接入、路由、页面流程、API client、认证/session、错误提示或 mock 策略变化。
- `deployments/`、Dockerfile、compose、Nginx、启动脚本、端口或环境变量变化。
- 数据库支持策略变化，例如 SQLite/PostgreSQL/MySQL、GORM、DSN、迁移、`json1`。
- 验证命令、测试矩阵、构建流程或 CI 假设变化。
- 完成执行清单中的阶段任务，或到达 Review Gate 前需要回写进度。
- 出现可复用的项目级规则，需要沉淀到 `AGENTS.md` 或 `AGENTS.override.md`。

## 不触发条件

以下情况通常不用同步文档：

- 纯重构，且不改变行为、接口、数据库、部署、测试命令或长期规则。
- 只修测试内部结构，且不改变测试矩阵或验证方式。
- 只修 typo、格式化或局部命名，且不会影响用户理解。
- 临时排查记录、一次性日志或失败实验。
- 没有完成实现和验证的待办项。可以记录风险，但不能勾选完成。

## 同步映射

根据改动路径判断需要更新的文档：

- `server/api/`、`server/api/v1/`、`web/src/api/`
  - 同步技术方案中的 API 设计、请求/响应、错误码、权限边界。
  - 同步执行清单中对应 API 接入、mock 移除、错误映射任务。

- `server/internal/service/exam/`、`server/internal/service/question/`、`server/internal/service/paper/`
  - 同步技术方案中的核心业务规则、状态流转、权限检查和领域限制。
  - 同步执行清单中 P4-P9 的服务层、答题、阅卷、成绩相关任务。

- `server/internal/dao/db/`、migration、数据库初始化逻辑
  - 同步技术方案中的数据库设计、索引、迁移、事务或查询策略。
  - SQLite 相关变化同步 `deployments/sqlite-single-node/README.md`。

- `web/src/pages/`、`web/src/app/`、`web/src/api/`
  - 同步执行清单 P9 前端页面、真实 API 接入、mock 移除、错误提示、路由行为。
  - 用户可见流程变化同步技术方案的前端交互或 API 使用说明。

- `deployments/`、Dockerfile、compose、Nginx、脚本
  - 同步执行清单 P10。
  - 同步对应部署 README。

- `server/library/constant/`
  - 同步技术方案中的关键领域常量或项目级约定。
  - 只有长期规则变化时再同步 `AGENTS.md`。

## 执行步骤

1. 看本轮改动范围：

   ```powershell
   git status --short
   git diff --stat
   ```

   必要时查看目标文件 diff：

   ```powershell
   git diff -- <path>
   ```

2. 归类影响面：

   - API 契约
   - 数据库/迁移
   - 后端业务规则
   - 前端页面/交互
   - 部署/配置
   - 测试/验证命令
   - 仅文档或仅重构

3. 定位执行清单项：

   ```powershell
   rg -n "关键词|P9|P10|Review|mock|API|部署|SQLite" docs/2026-05-25-papermind-execution-checklist.md
   ```

4. 判断技术方案是否需要更新：

   只有下面任一项变化时才改技术方案：

   - 业务规则或状态流转
   - API 请求/响应/错误码/权限边界
   - 数据库表、字段、索引、事务或查询策略
   - 部署架构、环境变量、端口、启动方式
   - 测试策略或支持矩阵
   - 长期技术决策

5. 写文档：

   - 使用简体中文。
   - 执行清单继续使用 `[ ]` / `[x]`。
   - 只勾选已经实现且验证过的项。
   - 未完成内容写成待办、风险或阻塞，不要假装完成。
   - 文档描述要能追溯到具体代码或验证结果。
   - 不要顺手重写整份技术方案。

6. 验证文档变更：

   ```powershell
   git diff --check
   ```

   如果文档引用路径、命令或文件名，确认这些目标真实存在。

## 项目验证命令

代码有改动时，按影响范围选择最小必要验证。不要凭空编造命令。

- 后端验证，工作目录 `server`：

  ```powershell
  go test -tags json1 ./...
  ```

- 前端验证，工作目录 `web`：

  ```powershell
  npm test
  npm run lint
  npm run build
  ```

- Windows 下如果 `npm run build` 因本地工具链权限问题失败，可以改用：

  ```powershell
  .\node_modules\.bin\tsc.cmd -b
  .\node_modules\.bin\vite.cmd build
  ```

  最终输出中必须说明降级原因和剩余风险。

仅修改 skill 或文档时，可以只运行 `git diff --check`，并在最终输出说明未运行代码测试的原因。

## PaperMind 快速规则

- 文档默认使用简体中文。
- REST API 统一在 `/api/v1` 下演进。
- API handler 调用 service，不直接承载数据库业务逻辑。
- 数据访问优先使用 GORM；raw SQL 需要有明确必要性。
- SQLite 相关 Go 测试使用 `-tags json1`。
- SQLite 单机方案只适合演示、本地验证和低并发场景，不推荐作为 100 人正式考试的默认部署方案。
- 领域常量统一放在 `server/library/constant`，避免在业务代码中散落硬编码字符串。
- 前端真实 API 接入后，要同步清理对应 mock 使用说明和执行清单状态。

## 禁止事项

- 不要没有证据就勾选执行清单。
- 不要把 TODO、计划中、待验证的事项写成已完成。
- 不要为了小改动大面积重写技术方案。
- 不要提交密钥、Token、密码、私有配置或无关生成文件。
- 不要把非当前包管理器生成的锁文件这类意外副产物纳入提交，除非任务明确要求并已确认。

## 输出要求

最终回复必须包含：

- 更新了哪些文档或 skill 文件。
- 哪些候选文档判断无需更新，以及理由。
- 勾选了哪些执行清单项；如果没有勾选，也要说明。
- 运行了哪些验证命令及结果。
- 剩余风险或未覆盖项。

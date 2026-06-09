# PaperMind Agent 工作规范

PaperMind 是全场景在线考试平台，后端位于 `server/`，前端位于
`web/`，部署资料位于 `deployments/`。本文件记录面向 Codex/Agent
的长期协作规则，适用于仓库根目录及其子目录。

## 语言与沟通

* 默认使用简体中文沟通、总结和写项目文档。
* 先读现有代码和文档，再给结论或改动。
* 对代码审查请求，优先输出离散、可执行的问题；没有问题就直接说明。
* 用户在代码审查后说“帮我修复”时，直接进入实现、验证和汇总。
* 遇到用户已有改动时，必须保留并协同处理，不能擅自回滚。

## 并行与探索

* 能独立执行、互不写同一文件的探索或验证任务，可以并行处理。
* 同一文件多处修改、强依赖链路、数据库/接口/权限核心逻辑，默认串行处理。
* 项目配置了 CodeGraph 时：

  * 结构性问题优先使用 CodeGraph，例如定义位置、调用关系、影响面和符号源码。
  * 字面量搜索、日志文本、文档文本仍使用 `rg`。
  * CodeGraph 提示索引滞后时，只针对提示的文件读取真实内容。

## 架构边界

* REST API 统一在 `/api/v1` 下演进。
* Handler 负责 HTTP 参数、响应和 service 调用，不承载数据库业务逻辑。
* Service 负责业务规则、权限编排和事务边界，不直接散落数据库细节。
* 数据库访问代码放在 `server/internal/dao/db`，不要放进
  `server/internal/service`。
* 权限相关规则优先沉淀在 permission/service/dao 的既有边界内。
* 领域常量统一放在 `server/library/constant`，避免业务代码散落硬编码字符串。
* 数据访问优先使用 GORM；raw SQL 必须有明确必要性。
* SQLite 相关 Go 测试使用 `-tags json1`。

## 质量与实现原则

* 最小、正确、可验证地修改当前任务所需范围。
* 不为一次性逻辑提前创建 helper、interface、factory 或复杂扩展点。
* 函数或方法的业务参数超过 4 个时，必须收敛为语义明确的输入结构体；receiver
  和 `context.Context` 不计入业务参数。分页、搜索、过滤、排序等成组参数应优先
  放入同一个 `ListInput` / `QueryInput` / `UpdateInput` 结构体，避免长期接口演进成
  位置参数串。
* 可信内部路径不做无意义防御；系统边界必须显式校验并传播错误。
* 不保留已经确认无用的兼容壳、空 wrapper、未使用变量或“曾经有代码”的注释。
* 新增或修复行为时，补充最小但有效的测试，断言业务字段和关键边界。
* 涉及 IO、RPC、数据库、外部系统调用时，需要支持超时、取消或错误传播。
* 代码必须有完善的中文注释，尤其是对存在复杂逻辑的代码。

## 需要先确认的操作

以下操作执行前必须向用户确认：

* 修改公共 API、导出符号、对外协议、事件结构或跨模块契约。
* 新增、升级、移除第三方依赖，或修改锁文件。
* 新增、修改、删除数据库表结构、索引、迁移脚本。
* 修改环境变量定义、系统配置或权限设置。
* 批量重命名、批量移动文件、跨包重构。

禁止提交密钥、Token、密码、私有配置或无关生成文件。

## 文档同步

代码、配置、部署、测试矩阵或长期规则变化后，按影响范围同步文档：

* 技术方案与业务规则：
  `docs/2026-05-25-papermind-exam-platform-technical-design.md`
* 阶段执行清单：
  `docs/2026-05-25-papermind-execution-checklist.md`
* 租户/权限模型：
  `docs/2025-05-28-papermind-tenant-admin-permission-model.md`
* 租户/权限执行清单：
  `docs/2025-05-28-papermind-tenant-admin-permission-execution-checklist.md`
* Docker Compose 部署：
  `deployments/docker-compose/README.md`
* SQLite 单机部署：
  `deployments/sqlite-single-node/README.md`
* 前端开发说明：
  `web/README.md`
* 长期 Agent 规则：
  `AGENTS.md` 或更近目录的 `AGENTS.override.md`

不要把一次性排查记录写进长期文档。只有实现和验证完成后，才能把清单项从
`[ ]` 改为 `[x]`。

## 执行环境

* 如果项目在 WSL 文件系统内打开，所有 shell 命令必须在 WSL/Linux shell 中执行。
* 不要在 Windows `cmd.exe` 或 PowerShell 中执行 WSL UNC 路径下的项目命令。
* 如果仓库位于 WSL 文件系统，依赖安装、测试、构建都必须在 WSL 内完成；不要在 Windows 侧为同一份源码安装 `node_modules`。
* 如果仓库位于 Windows 文件系统但通过 WSL 访问，必须先提示性能和路径兼容风险，不主动迁移或复制仓库。
* 执行 Node/Go 命令前，先确认 `pwd`、`which node`、`which npm`、`which go`。
* 如果 PATH 缺少 Go、Node 或 npm，但已发现可用绝对路径，可以优先使用绝对路径继续验证，并在最终汇总中说明。
* 前端脚本优先使用 `web/node_modules/.bin` 下的项目本地可执行文件，不依赖全局安装的 vitest、vite、eslint。
* 不要混用 Windows 侧 `node_modules` 和 WSL 侧 Node 运行时；如发现平台不一致，提示重新在当前环境执行依赖安装。

## 依赖与包管理

* 前端包管理器以现有锁文件为准：存在 `package-lock.json` 时使用 `npm`，不要改用 pnpm/yarn。
* 不要主动删除或重建锁文件。
* 如果 `node_modules` 不存在：

  * 存在 `package-lock.json` 时，优先在 `web/` 目录执行 `npm ci`。
  * 没有锁文件但存在 `package.json` 时，才考虑 `npm install`。
  * 执行依赖安装前，先确认当前 shell、Node、npm 都来自同一运行环境。
* 后端依赖以 `server/go.mod` 和 `server/go.sum` 为准。
* 未经确认，不执行 `go get`、`go mod tidy`、`npm update`、`npm audit fix`。
* 如果构建因缺依赖失败，先判断是环境未安装、node_modules 缺失，还是代码/锁文件问题，再决定是否请求确认。

## Git 安全

* 未经用户明确要求，不执行 `git commit`、`git push`、`git reset --hard`、`git clean -fd`。
* 未经用户明确要求，不执行 `git stash`、`git checkout -- <file>`、`git restore <file>`。
* 不删除未跟踪文件，除非确认它是本次任务生成的临时文件。
* 修改前先查看 `git status --short`，区分用户已有改动和本次改动。
* 如果发现同一文件已有用户改动，修改前必须先阅读相关 diff，避免覆盖。
* 最终汇总必须列出：改动文件、验证命令、验证结果、未验证项和原因。

## 构建与测试

优先使用仓库已有命令，不凭空推断。长时间命令需要设置合理超时。

项目入口必须先确认工作目录：

* Go 后端项目入口是 `server/`，`go.mod` 位于 `server/go.mod`。
  所有 `go test`、`go build`、`go run`、`go fmt` 等 Go module 命令都必须
  在 `server/` 目录执行，或通过根目录 `Makefile` 间接执行。
* 前端项目入口是 `web/`，`package.json` 位于 `web/package.json`。
  所有 `npm test`、`npm run lint`、`npm run build`、`npm run dev` 等前端命令都必须
  在 `web/` 目录执行，或通过根目录 `Makefile` 间接执行。
* 仓库根目录不是 Go module，也不是前端 package 根目录。不要在根目录直接执行
  `go test ./...`、`go build ./...`、`npm test`、`npm run build` 等项目命令。
* 前端不要直接执行全局 `vite`、`vitest`、`eslint`、`tsc`；优先通过
  `npm run ...` 或 `./node_modules/.bin/...` 调用项目本地版本。

常用 Make 入口：

```bash
make frontend-build
make frontend-dev
make backend-build
make backend-dev
```

后端验证在 `server/` 目录执行：

```bash
go test -tags json1 ./...
go build -tags json1 ./...
```

前端验证在 `web/` 目录执行：

```bash
npm test
npm run lint
npm run build
```

文档或规则文件改动至少执行：

```bash
git diff --check
```

如果只改文档，可以不跑代码测试，但最终说明原因和剩余风险。

## 验证策略

* 优先运行与本次改动直接相关的最小测试。
* 如果改动影响公共模型、权限、认证、数据库迁移、路由注册或构建配置，必须扩大到对应模块全量测试。
* 后端局部验证可以先运行相关 package 的 `go test -tags json1 ./path/to/pkg`，最终再按影响范围决定是否运行 `go test -tags json1 ./...`。
* 前端局部验证可以先运行相关测试或类型检查；涉及路由、构建配置、API client、状态管理时，需要执行 `npm run build`。
* 如果测试失败，必须区分：

  * 本次改动引入的问题；
  * 环境缺失；
  * 既有失败；
  * 测试命令位置错误。

## 数据库与迁移

* 修改表结构、索引、约束或字段语义时，必须同步考虑 PostgreSQL、MySQL、SQLite 三套迁移。
* 迁移文件不得改写已发布版本；新增迁移必须使用递增版本号。
* SQLite 迁移必须保留 `_foreign_keys=on` 可验证的外键语义。
* 新增字段时，同步检查：

  * GORM DO 结构体；
  * Columns 字段映射；
  * Repository 创建/更新逻辑；
  * API request/response DTO；
  * 种子数据；
  * 相关测试。
* 数据库字段、索引、约束变更必须同步更新技术文档或执行清单。

## API 契约同步

* 修改 API 路径、请求字段、响应字段、错误码或权限语义时，必须同步检查前端 API client、TypeScript 类型、页面调用点和测试。
* 修改 API 契约时，如项目维护了 OpenAPI、接口文档或 `docs/api`，必须同步更新。
* 后端 response 字段命名以现有项目约定为准，不在同一接口中混用 snake_case 和 camelCase。
* 前端不得为未实现 API 长期保留静态 mock；真实 API 接入后必须清理对应 mock 和说明。
* 权限、租户隔离、actor_type、tenant_id 等字段不能只在前端判断，后端必须做最终校验。
* 新增接口时必须符合权限规范。

## 提交与审查习惯

* 审查 `dev/p0-skeleton` 相较于 `master` 时，按用户给出的 merge base
  或明确 base 分支比较。
* 修复审查问题时，先写或调整能暴露问题的测试，再实现，再验证。
* “帮我把所有代码提交”表示将当前同一阶段的相关改动作为一批提交；提交前先看
  `git status --short`、`git diff --stat`、`git diff` 和必要验证。
* 如果 `.git/index.lock: Permission denied` 导致暂存失败，保留暂存计划并在环境恢复后重试。

## 前端约定

* 前端使用 React、Vite、TypeScript。
* 真实 API 接入后，同步清理对应 mock 说明和执行清单状态。
* 管理类界面优先清晰、密集、可扫描；不要做成营销落地页。
* 控件、图标、状态和错误提示需要服务于实际工作流。

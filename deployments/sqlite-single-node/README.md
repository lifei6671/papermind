# PaperMind SQLite 单机模式

本目录用于本地开发、功能演示和低并发单机体验。SQLite 不推荐用于 100 人正式考试，正式考试优先使用 PostgreSQL，MySQL 8.0+ 作为兼容目标。

## 启动

在仓库根目录执行：

```powershell
.\deployments\sqlite-single-node\start.ps1
```

可指定端口和数据库文件：

```powershell
.\deployments\sqlite-single-node\start.ps1 -Port 9080 -DatabaseFile "server/data/sqlite/papermind.db"
```

脚本会自动创建运行期目录，并使用 `json1` 标签构建后端：

```powershell
go build -tags json1 -o deployments/sqlite-single-node/bin/papermind-sqlite.exe ./cmd/papermind
```

## SQLite DSN

脚本会把 `-DatabaseFile` 解析为仓库根目录下的绝对路径，并注入固定包含首版必需参数的 SQLite DSN：

```text
file:<repo>\server\data\sqlite\papermind.db?_foreign_keys=on&_journal_mode=WAL&_busy_timeout=5000
```

- `_foreign_keys=on`：启用外键约束。
- `_journal_mode=WAL`：启用 WAL，降低本地读写阻塞。
- `_busy_timeout=5000`：写锁等待 5 秒后失败，避免请求无限挂起。

## 单机边界

- `PAPERMIND_DATABASE_MAX_OPEN_CONNS=1`
- `PAPERMIND_DATABASE_MAX_IDLE_CONNS=1`
- 文件目录使用仓库根目录下的 `server/data/tmp`、`server/data/imports`、`server/data/exports` 绝对路径
- 不在脚本中写入生产密钥、Token 或线上配置

## 默认登录

`start.ps1` 会设置 `PAPERMIND_APP_ENV=dev`。全新 SQLite 数据库完成迁移后，后端会在 `platform_users` 为空时初始化本地联调用平台管理员：

- 账号：`admin`
- 密码：`admin123`
- 邮箱：`admin@iminho.me`

如需正式考试部署，请使用 PostgreSQL 或 MySQL 8.0+，并完成 P10 的压测和迁移验证。

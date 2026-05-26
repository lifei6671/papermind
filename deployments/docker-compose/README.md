# PaperMind Docker Compose

本目录提供首版 PostgreSQL + server + web/Nginx 的 Compose 部署骨架，用于本地联调和部署演练。不要把生产密钥写入本目录。

## 准备环境变量

复制示例文件后填写本地值：

```powershell
Copy-Item deployments\docker-compose\.env.example deployments\docker-compose\.env
```

必须填写：

```text
PAPERMIND_POSTGRES_PASSWORD=
```

`.env` 已被仓库根目录 `.gitignore` 排除，不要提交真实密码、Token 或生产地址。

## 启动

```powershell
docker compose --env-file deployments\docker-compose\.env -f deployments\docker-compose\compose.yaml up --build
```

默认端口：

- Web/Nginx: `http://localhost`
- Server: `http://localhost:8080`
- PostgreSQL: 仅在 Compose 网络内暴露给 server

## 服务组成

- `postgres`：PostgreSQL 16，使用 `postgres-data` volume 保存数据。
- `server`：Go 后端，使用环境变量注入数据库、端口、存储目录和 CORS。
- `web`：React 静态资源，由 Nginx 提供；`/api/` 代理到 server。

## 空库迁移验证

当后端启动入口接入迁移执行后，使用以下步骤验证从空库启动：

```powershell
docker compose --env-file deployments\docker-compose\.env -f deployments\docker-compose\compose.yaml down -v
docker compose --env-file deployments\docker-compose\.env -f deployments\docker-compose\compose.yaml up --build
```

确认 server 日志包含迁移目录和数据库初始化日志，并确认 PostgreSQL 中业务表已创建。

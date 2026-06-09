# DockPilot

DockPilot 是一个轻量的 Docker / Docker Compose 节点管理面板。它采用中心 Server + VPS Agent 架构，Agent 主动连接 Server，适合防火墙或 NAT 后的 Linux VPS。

## 功能

- 节点式管理：节点上线、离线、系统资源、Docker 版本、Compose 版本。
- Docker 资产：容器、镜像、Compose 项目扫描。
- 更新中心：手动检测、手动更新、按策略定时或全自动更新 Compose 项目。
- 任务中心：任务状态、实时回传日志、失败结果保存。
- Compose 混合管理：扫描宿主机已有项目，也支持面板保存 compose.yml 并下发部署。
- 简洁 RBAC：管理员可操作，viewer 只读。
- 通知渠道：Telegram、Webhook、Email。
- 默认数据库：SQLite。

## 本地开发

后端：

```bash
go run ./cmd/server
```

前端：

```bash
cd web
npm install
npm run dev
```

Agent：

```bash
go run ./cmd/agent \
  -server http://127.0.0.1:8080 \
  -registration-token change-me-registration-token
```

默认登录账号是 `admin` / `admin`。生产环境必须设置 `DOCKPILOT_ADMIN_PASSWORD`、`DOCKPILOT_AUTH_SECRET` 和 `DOCKPILOT_AGENT_REGISTRATION_TOKEN`。

## Docker 部署

```bash
cd deploy
docker compose up -d server
```

在需要被管理的 VPS 上运行 Agent：

```bash
docker run -d --name dockpilot-agent --restart unless-stopped \
  -e DOCKPILOT_SERVER_URL=http://YOUR_SERVER:8080 \
  -e DOCKPILOT_REGISTRATION_TOKEN=YOUR_REGISTRATION_TOKEN \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /opt:/opt \
  -v /srv:/srv \
  -v /var/www:/var/www \
  ghcr.io/dockpilot/dockpilot-agent:latest
```

## 更新策略

策略优先级固定为：

```text
容器 / Compose 项目 > 节点 > 全局
```

支持的模式：

- `manual`：默认模式，只检测或手动执行。
- `scheduled`：按 `@hourly`、`@daily` 或 `interval:6h` 触发。
- `automatic`：按计划直接执行 Compose 更新。

`exclude_patterns` 使用英文逗号分隔，匹配项目名称或路径。建议把 `mysql`、`postgres`、`mariadb`、`redis` 等状态型服务保持手动更新。

## API 概览

- `/api/auth/*`：登录、刷新、当前用户。
- `/api/nodes/*`：节点列表和节点详情。
- `/api/docker/*`：Docker 状态、Compose 保存。
- `/api/tasks/*`：任务创建、状态、日志、取消。
- `/api/policies/*`：全局、节点、Compose、容器策略。
- `/api/notifications/*`：Telegram、Webhook、Email 通知配置。
- `/api/agent/ws`：Agent WebSocket 通道。

## 安全提示

Agent 需要访问宿主机 `/var/run/docker.sock`，这等价于较高的宿主机控制权限。请只把 Agent 部署在你信任的 VPS 上，并为 Server 配置 HTTPS、强随机密钥和强管理员密码。

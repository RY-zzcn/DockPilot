# DockPilot

DockPilot 是一个轻量的 Docker / Docker Compose 节点管理面板，采用中心 Server + VPS Agent 架构。Agent 主动连接 Server，适合 NAT、防火墙后面的 Linux VPS，用于集中查看节点资源、容器状态、Compose 项目和更新任务。

## 功能

- 节点管理：在线/离线、CPU、内存、磁盘、网络、Docker/Compose 版本、重命名、备注和删除。
- Docker 资产：容器、镜像、Compose 项目扫描和状态同步。
- 更新中心：手动检测、手动确认更新、定时自动、全自动策略。
- Compose 管理：扫描已有 Compose 项目，也可在面板中托管 compose.yml 并下发部署。
- 任务中心：任务状态、日志回传、失败原因、重试入口和历史清理。
- 通知渠道：Telegram、Webhook、Email。
- 权限控制：管理员可操作，viewer 只读。
- 北京时间：默认 `Asia/Shanghai`，面板、任务、日志、指标按北京时间展示和写入。
- 多主题 UI：极光、石墨、日冕、终端四种现代化运维面板主题。
- 版本发布：面板显示当前 Server、最新 GitHub Release、节点 Agent 版本和可升级状态。
- Agent 升级：支持面板一键升级单个/多个节点，也可开启 Agent 自动升级。

## 快速部署

推荐先部署 Server，再到面板设置页复制 Agent 接入命令。

也可以直接进入交互式菜单：

```bash
curl -fsSL https://raw.githubusercontent.com/RY-zzcn/DockPilot/main/scripts/dockpilot-install.sh | bash
```

### 一键部署 Server

Docker 方式：

```bash
curl -fsSL https://raw.githubusercontent.com/RY-zzcn/DockPilot/main/scripts/dockpilot-install.sh | bash -s -- install-server-docker
```

二进制 + systemd 方式：

```bash
curl -fsSL https://raw.githubusercontent.com/RY-zzcn/DockPilot/main/scripts/dockpilot-install.sh | bash -s -- install-server-binary
```

脚本会输出管理员账号、管理员密码和 Agent 注册 token。默认端口为 `8080`。

### 一键接入 Agent

Agent 二进制方式更轻，适合像探针项目一样快速接入多架构节点：

```bash
curl -fsSL https://raw.githubusercontent.com/RY-zzcn/DockPilot/main/scripts/dockpilot-install.sh | bash -s -- install-agent-binary \
  --server-url http://YOUR_SERVER:8080 \
  --registration-token YOUR_REGISTRATION_TOKEN
```

Agent Docker 方式：

```bash
curl -fsSL https://raw.githubusercontent.com/RY-zzcn/DockPilot/main/scripts/dockpilot-install.sh | bash -s -- install-agent-docker \
  --server-url http://YOUR_SERVER:8080 \
  --registration-token YOUR_REGISTRATION_TOKEN
```

### 卸载

保留数据卸载：

```bash
curl -fsSL https://raw.githubusercontent.com/RY-zzcn/DockPilot/main/scripts/dockpilot-install.sh | bash -s -- uninstall
```

只卸载 Agent：

```bash
curl -fsSL https://raw.githubusercontent.com/RY-zzcn/DockPilot/main/scripts/dockpilot-install.sh | bash -s -- uninstall --target agent
```

删除程序和数据：

```bash
curl -fsSL https://raw.githubusercontent.com/RY-zzcn/DockPilot/main/scripts/dockpilot-install.sh | bash -s -- uninstall --target all --purge
```

更完整的部署、升级、离线安装和排障说明见 [docs/deployment.md](docs/deployment.md)。

## 发布产物

每次推送 `v*` 标签会自动创建 GitHub Release，并发布 Docker Packages。

Release 二进制包：

- `dockpilot-server_<version>_linux_amd64.tar.gz`
- `dockpilot-server_<version>_linux_arm64.tar.gz`
- `dockpilot-agent_<version>_linux_amd64.tar.gz`
- `dockpilot-agent_<version>_linux_arm64.tar.gz`
- `dockpilot-agent_<version>_linux_armv7.tar.gz`
- `dockpilot-agent_<version>_linux_armv6.tar.gz`
- `dockpilot-agent_<version>_linux_386.tar.gz`
- `dockpilot-agent_<version>_linux_riscv64.tar.gz`
- `dockpilot_<version>_linux_amd64.tar.gz` 和 `dockpilot_<version>_linux_arm64.tar.gz` 完整包，包含 Server、Agent、前端和部署模板。

Docker 镜像：

```bash
docker pull ghcr.io/ry-zzcn/dockpilot-server:latest
docker pull ghcr.io/ry-zzcn/dockpilot-agent:latest
```

Docker 镜像当前发布 `linux_amd64` 和 `linux_arm64`，标签会同步发布 `latest`、`v<version>` 和 `<version>`。

Agent Docker 镜像会内置 Docker CLI、Compose v2 插件、CA 证书和时区数据，用于在容器内驱动宿主机 Docker socket 执行 Compose 更新，因此体积会明显大于单个 Agent 二进制。追求轻量和多架构接入时，推荐使用 Agent 二进制 + systemd。

## 版本与 Agent 自动升级

设置页会显示：

- 当前 Server 版本、commit、构建时间和最新 Release。
- 每个节点的 Agent 版本、系统架构和升级状态。
- Agent 自动升级开关、目标版本和扫描间隔。

Agent 升级任务会根据安装方式处理：

- 二进制 + systemd：Agent 下载匹配架构的 Release 包，替换当前二进制，然后退出并由 systemd 拉起。
- Docker：Agent 启动独立 updater 容器重拉对应版本镜像并重建 `dockpilot-agent` 容器；重建时会保留当前 Server URL、Agent state volume 和原 Docker network。

自动升级默认关闭。管理员可在设置页开启，或通过环境变量启用：

```bash
DOCKPILOT_AGENT_AUTO_UPDATE=true
DOCKPILOT_AGENT_AUTO_UPDATE_VERSION=latest
DOCKPILOT_AGENT_AUTO_UPDATE_INTERVAL_SECONDS=3600
```

自动升级会使用 `DOCKPILOT_RELEASE_REPO` 指向的 Release 仓库，默认是 `RY-zzcn/DockPilot`。

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

## 更新策略

策略优先级：

```text
容器 / Compose 项目 > 节点 > 全局
```

支持模式：

- `manual`：默认模式，只检测或手动执行。
- `scheduled`：按 `@hourly`、`@daily` 或 `interval:6h` 触发检测，只标记可更新项，不自动部署。
- `automatic`：按计划直接执行 Compose 更新。

`exclude_patterns` 使用英文逗号分隔，匹配项目名称或路径。建议把 `mysql`、`postgres`、`mariadb`、`redis` 等状态型服务保持手动更新。

### 更新检测方式与频率

- 手动检测：在节点页或更新中心点击检测，会创建 `detect_updates` 任务。
- 自动检测：Server 调度器每 `1 分钟` 扫描一次策略；真正执行频率由策略决定，支持 `@hourly`、`@daily` 和 `interval:<duration>`，例如 `interval:6h`。
- 默认策略：没有显式保存策略时为 `manual`，不会自动检测或更新。
- Agent 快照：Agent 默认每 `60 秒` 同步 Docker/Compose 状态，每 `15 秒` 上报心跳和基础指标。
- 检测逻辑：Agent 会以 Compose 文件所在目录作为 project directory，先运行 `docker compose config --images` 获取镜像列表，优先通过 Registry API 按节点平台读取远端 digest；失败时回退到 `docker buildx imagetools inspect` / `docker manifest inspect --verbose`。Agent 会比较本地 `docker image inspect` 与远端平台镜像的 digest，并回传 `update_available`、检测方式、平台和检测时间。
- 固定镜像：`image@sha256:...` 形式的 digest 固定镜像会被识别为已固定版本，不会误报更新。
- 更新逻辑：手动执行或 `automatic` 策略会先做 Compose 配置预检，再运行 `docker compose pull --ignore-buildable`，随后运行 `docker compose up -d --remove-orphans`。任务日志会记录容器镜像 ID 是否变化；如果项目已是最新、使用固定 digest 或 build-only 服务，命令可能成功但镜像 ID 不变。

## API 概览

- `/api/auth/*`：登录、刷新、当前用户。
- `/api/version`：Server 版本、commit、构建时间、服务时区和当前服务时间。
- `/api/settings/runtime`：Release 仓库信息和 Agent 自动升级设置。
- `/api/nodes/*`：节点列表、节点详情、节点重命名、备注和删除。
- `/api/docker/*`：Docker 状态、Compose 保存。
- `/api/tasks/*`：任务创建、状态、日志、取消和历史清理。
- `/api/policies/*`：全局、节点、Compose、容器策略。
- `/api/notifications/*`：Telegram、Webhook、Email 通知配置。
- `/api/agent/ws`：Agent WebSocket 通道。

## 安全提示

Agent 需要访问宿主机 `/var/run/docker.sock`，这等价于较高的宿主机控制权限。请只把 Agent 部署在可信 VPS 上，并为 Server 配置 HTTPS、强随机密钥和强管理员密码。

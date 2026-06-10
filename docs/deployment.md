# DockPilot 部署教程

本文档覆盖 Docker 部署、二进制部署、Agent 节点接入、升级和卸载。当前支持 Linux VPS，依赖 Docker Engine + Docker Compose v2。

## 选择部署方式

Server 推荐：

- Docker：最省心，适合大多数 VPS。
- 二进制 + systemd：更轻、更容易被反向代理和系统服务统一管理。

Agent 推荐：

- 二进制 + systemd：最像探针项目，单文件接入，资源占用低，适合多架构 VPS。
- Docker：隔离简单，但仍需挂载 `/var/run/docker.sock`。

## 一键脚本

脚本地址：

```bash
https://raw.githubusercontent.com/RY-zzcn/DockPilot/main/scripts/dockpilot-install.sh
```

无参数运行会进入交互菜单：

```bash
curl -fsSL https://raw.githubusercontent.com/RY-zzcn/DockPilot/main/scripts/dockpilot-install.sh | bash
```

常用参数：

- `--public-url`：Server 对外访问地址，例如 `http://1.2.3.4:8080`。
- `--server-url`：Agent 连接的 Server 地址。
- `--registration-token`：Agent 注册 token。
- `--admin-password`：管理员密码。
- `--node-name`：Agent 节点名称。
- `--version`：指定版本，例如 `v0.2.3`；默认使用 latest release。
- `--target`：卸载目标，支持 `agent`、`server`、`all`。
- `--purge`：卸载时同时删除数据。

常用环境变量：

- `DOCKPILOT_RELEASE_REPO`：Release 仓库，默认 `RY-zzcn/DockPilot`。
- `DOCKPILOT_AGENT_AUTO_UPDATE`：是否启用 Agent 自动升级，默认 `false`。
- `DOCKPILOT_AGENT_AUTO_UPDATE_VERSION`：Agent 自动升级目标，默认 `latest`。
- `DOCKPILOT_AGENT_AUTO_UPDATE_INTERVAL_SECONDS`：自动升级扫描间隔，默认 `3600` 秒。

## 部署 Server

### Docker 方式

```bash
curl -fsSL https://raw.githubusercontent.com/RY-zzcn/DockPilot/main/scripts/dockpilot-install.sh | bash -s -- install-server-docker \
  --public-url http://YOUR_SERVER:8080
```

Docker 镜像当前发布 `linux_amd64` 和 `linux_arm64`。更小众的节点架构建议使用 Agent 二进制方式接入。

脚本会：

- 检测 Docker，不存在时尝试安装 Docker。
- 写入 `/opt/dockpilot/deploy/docker-compose.yml`。
- 写入 `/opt/dockpilot/deploy/.env`。
- 启动 `dockpilot-server` 容器。
- 输出管理员账号、密码和 Agent 注册 token。

### 二进制方式

```bash
curl -fsSL https://raw.githubusercontent.com/RY-zzcn/DockPilot/main/scripts/dockpilot-install.sh | bash -s -- install-server-binary \
  --public-url http://YOUR_SERVER:8080
```

脚本会：

- 自动识别 `linux_amd64` 或 `linux_arm64`。
- 下载 `dockpilot-server_<version>_<arch>.tar.gz`。
- 安装到 `/opt/dockpilot`。
- 创建 `/etc/dockpilot/server.env`。
- 创建并启动 `dockpilot-server.service`。
- 数据库默认在 `/var/lib/dockpilot/dockpilot.db`。

手动 systemd 管理：

```bash
systemctl status dockpilot-server
systemctl restart dockpilot-server
journalctl -u dockpilot-server -f
```

## 接入 Agent

在 Server 面板的设置页可以直接复制带 token 的命令。

### 二进制方式

```bash
curl -fsSL https://raw.githubusercontent.com/RY-zzcn/DockPilot/main/scripts/dockpilot-install.sh | bash -s -- install-agent-binary \
  --server-url http://YOUR_SERVER:8080 \
  --registration-token YOUR_REGISTRATION_TOKEN \
  --node-name vps-01
```

脚本会：

- 自动识别架构。
- 下载独立 Agent 包。
- 安装到 `/opt/dockpilot-agent/dockpilot-agent`。
- 写入 `/etc/dockpilot/agent.env`。
- 创建并启动 `dockpilot-agent.service`。

支持的 Agent 二进制架构：

- `linux_amd64`
- `linux_arm64`
- `linux_armv7`
- `linux_armv6`
- `linux_386`
- `linux_riscv64`

Agent 服务管理：

```bash
systemctl status dockpilot-agent
systemctl restart dockpilot-agent
journalctl -u dockpilot-agent -f
```

### Docker 方式

```bash
curl -fsSL https://raw.githubusercontent.com/RY-zzcn/DockPilot/main/scripts/dockpilot-install.sh | bash -s -- install-agent-docker \
  --server-url http://YOUR_SERVER:8080 \
  --registration-token YOUR_REGISTRATION_TOKEN \
  --node-name vps-01
```

Agent Docker 镜像当前发布 `linux_amd64` 和 `linux_arm64`。`armv7`、`armv6`、`386`、`riscv64` 节点请使用 Agent 二进制包。

等效手动命令：

```bash
docker run -d --name dockpilot-agent --restart unless-stopped \
  -e TZ=Asia/Shanghai \
  -e DOCKPILOT_SERVER_URL=http://YOUR_SERVER:8080 \
  -e DOCKPILOT_REGISTRATION_TOKEN=YOUR_REGISTRATION_TOKEN \
  -e DOCKPILOT_NODE_NAME=vps-01 \
  -e DOCKPILOT_COMPOSE_DIRS=/opt,/srv,/var/www \
  -e DOCKPILOT_INSTALL_MODE=docker \
  -e DOCKPILOT_RELEASE_REPO=RY-zzcn/DockPilot \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /opt:/opt \
  -v /srv:/srv \
  -v /var/www:/var/www \
  -v dockpilot-agent-state:/data \
  ghcr.io/ry-zzcn/dockpilot-agent:latest
```

## 离线安装

从 Release 页面下载对应架构的包：

```bash
wget https://github.com/RY-zzcn/DockPilot/releases/download/v0.2.3/dockpilot-agent_0.2.3_linux_amd64.tar.gz
tar -xzf dockpilot-agent_0.2.3_linux_amd64.tar.gz
install -m 0755 dockpilot-agent /opt/dockpilot-agent/dockpilot-agent
```

然后参考包内 `deploy/dockpilot-agent.service` 和 `/etc/dockpilot/agent.env` 创建 systemd 服务。

## 升级

面板设置页会显示当前 Server 版本、最新 Release、每个节点的 Agent 版本和升级状态。管理员可以在面板中为单个节点创建 Agent 升级任务，也可以批量升级所有落后节点。

Agent 自动升级默认关闭。开启后，Server 会按 `DOCKPILOT_AGENT_AUTO_UPDATE_INTERVAL_SECONDS` 或面板显示的间隔扫描在线节点，发现 Agent 版本落后于目标版本时创建 `agent_update` 任务。目标版本为 `latest` 时会使用 GitHub 最新 Release。

Agent 升级任务行为：

- 二进制 Agent：下载匹配系统架构的 `dockpilot-agent_<version>_<arch>.tar.gz`，替换当前二进制并退出，由 systemd 自动重启。
- Docker Agent：调用一键脚本重新创建 `dockpilot-agent` 容器，并保留已有 Agent state volume。

开启自动升级：

```bash
DOCKPILOT_AGENT_AUTO_UPDATE=true
DOCKPILOT_AGENT_AUTO_UPDATE_VERSION=latest
DOCKPILOT_AGENT_AUTO_UPDATE_INTERVAL_SECONDS=3600
```

### Docker 升级

```bash
docker pull ghcr.io/ry-zzcn/dockpilot-server:latest
docker pull ghcr.io/ry-zzcn/dockpilot-agent:latest
docker compose -f /opt/dockpilot/deploy/docker-compose.yml --profile agent up -d
```

### 二进制升级

重新运行对应安装命令即可覆盖二进制并重启 systemd 服务：

```bash
curl -fsSL https://raw.githubusercontent.com/RY-zzcn/DockPilot/main/scripts/dockpilot-install.sh | bash -s -- install-agent-binary \
  --server-url http://YOUR_SERVER:8080 \
  --registration-token YOUR_REGISTRATION_TOKEN
```

指定版本：

```bash
curl -fsSL https://raw.githubusercontent.com/RY-zzcn/DockPilot/main/scripts/dockpilot-install.sh | bash -s -- install-agent-binary \
  --version v0.2.3 \
  --server-url http://YOUR_SERVER:8080 \
  --registration-token YOUR_REGISTRATION_TOKEN
```

## 更新检测和自动更新

DockPilot 默认不会自动更新容器。未配置策略时，全局策略等价于 `manual`。

检测流程：

- Agent 扫描 `/opt,/srv,/var/www` 等目录中的 Compose 文件，也会读取 `docker compose ls` 中正在运行的项目。
- `detect_updates` 任务会执行 `docker compose -f <compose.yml> config --images` 获取镜像列表。
- Agent 使用 `docker image inspect` 读取本地镜像 digest 和平台信息。
- Agent 优先通过 Registry API 按节点平台读取远端 digest；失败时回退到 `docker buildx imagetools inspect` 和 `docker manifest inspect --verbose`。
- 本地 digest 和远端平台 digest 不一致，或者本地镜像缺失但远端存在时，会标记为可更新并回传 Server。
- `image@sha256:...` 形式的固定镜像会被识别为已固定版本，不会误报更新。
- 私有镜像仓库需要 Agent 能读取 Docker 登录凭据，例如挂载或配置可用的 Docker config。

频率：

- 心跳：默认 `15 秒`。
- 指标：默认 `15 秒`，可通过 `DOCKPILOT_METRICS_INTERVAL_SECONDS` 调整。
- Docker/Compose 快照：默认 `60 秒`，可通过 `DOCKPILOT_SNAPSHOT_INTERVAL_SECONDS` 调整。
- 更新检测缓存：默认 `15 分钟`，可通过 `DOCKPILOT_UPDATE_CACHE_SECONDS` 调整。
- 策略调度器：Server 每 `1 分钟` 扫描一次策略。
- 策略执行间隔：由 `@hourly`、`@daily` 或 `interval:<duration>` 控制，例如 `interval:6h`。

策略模式：

- `manual`：只在面板手动点击时检测或更新。
- `scheduled`：到点只检测并标记可更新项，需要管理员再手动确认更新。
- `automatic`：到点直接执行 `docker compose pull --ignore-buildable` 和 `docker compose up -d --remove-orphans`。如果节点上的 Compose 版本不支持 `--ignore-buildable`，Agent 会自动重试普通 `docker compose pull`。

建议把数据库、缓存、消息队列等状态型服务加入排除列表，或单独配置为 `manual`。

## 卸载

交互式卸载会先检测本机是否存在 Server、Agent、Docker 容器或 systemd 服务，再选择删除目标：

```bash
curl -fsSL https://raw.githubusercontent.com/RY-zzcn/DockPilot/main/scripts/dockpilot-install.sh | bash -s -- uninstall
```

只卸载 Agent：

```bash
curl -fsSL https://raw.githubusercontent.com/RY-zzcn/DockPilot/main/scripts/dockpilot-install.sh | bash -s -- uninstall --target agent
```

只卸载 Server：

```bash
curl -fsSL https://raw.githubusercontent.com/RY-zzcn/DockPilot/main/scripts/dockpilot-install.sh | bash -s -- uninstall --target server
```

删除 Server、Agent、数据目录和 Docker volumes：

```bash
curl -fsSL https://raw.githubusercontent.com/RY-zzcn/DockPilot/main/scripts/dockpilot-install.sh | bash -s -- uninstall --target all --purge
```

默认保留数据时，以下内容不会删除：

- `/var/lib/dockpilot`
- `/var/lib/dockpilot-agent`
- Docker named volumes

卸载会移除对应的 systemd 服务、Docker 容器、安装目录和 `/etc/dockpilot/*.env` 文件；`--purge` 会额外删除数据目录、相关 Docker volumes 和 DockPilot 镜像。

## 反向代理

建议生产环境使用 HTTPS。Nginx 示例：

```nginx
server {
    listen 80;
    server_name dockpilot.example.com;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    location /api/agent/ws {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
    }
}
```

反向代理后请把 `DOCKPILOT_PUBLIC_URL` 设置为 HTTPS 地址。

## 安全注意

Agent 需要访问 `/var/run/docker.sock`，等价于较高宿主机控制权限。建议：

- 只在可信节点部署 Agent。
- Server 使用 HTTPS。
- 管理员密码、`DOCKPILOT_AUTH_SECRET`、Agent 注册 token 使用强随机值。
- 不把 SQLite 数据库、`.env`、systemd env 文件提交到仓库。

## 排障

查看 Server：

```bash
docker logs -f dockpilot-server
journalctl -u dockpilot-server -f
```

查看 Agent：

```bash
docker logs -f dockpilot-agent
journalctl -u dockpilot-agent -f
```

检查 Docker socket：

```bash
ls -l /var/run/docker.sock
docker ps
docker compose version
```

节点不上线时，优先检查：

- Agent 的 `DOCKPILOT_SERVER_URL` 是否能访问。
- 注册 token 是否和 Server 设置页一致。
- 反向代理是否允许 WebSocket。
- Agent 是否能访问 `/var/run/docker.sock`。

更新检测出现失败状态时，优先检查 Agent 是否能访问对应镜像仓库、私有仓库是否已登录、Compose 文件路径是否仍存在。单个镜像检测失败会在更新中心显示为检测失败，不会阻断其它项目继续检测。

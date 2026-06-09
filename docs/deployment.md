# DockPilot 部署教程

本文档覆盖 Docker 部署、二进制部署、Agent 节点接入、升级和卸载。首版目标系统为 Linux VPS，依赖 Docker Engine + Docker Compose v2。

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
- `--version`：指定版本，例如 `v0.1.1`；默认使用 latest release。
- `--purge`：卸载时同时删除数据。

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
wget https://github.com/RY-zzcn/DockPilot/releases/download/v0.1.1/dockpilot-agent_0.1.1_linux_amd64.tar.gz
tar -xzf dockpilot-agent_0.1.1_linux_amd64.tar.gz
install -m 0755 dockpilot-agent /opt/dockpilot-agent/dockpilot-agent
```

然后参考包内 `deploy/dockpilot-agent.service` 和 `/etc/dockpilot/agent.env` 创建 systemd 服务。

## 升级

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
  --version v0.1.1 \
  --server-url http://YOUR_SERVER:8080 \
  --registration-token YOUR_REGISTRATION_TOKEN
```

## 卸载

保留数据：

```bash
curl -fsSL https://raw.githubusercontent.com/RY-zzcn/DockPilot/main/scripts/dockpilot-install.sh | bash -s -- uninstall
```

删除程序和数据：

```bash
curl -fsSL https://raw.githubusercontent.com/RY-zzcn/DockPilot/main/scripts/dockpilot-install.sh | bash -s -- uninstall --purge
```

保留数据时，以下内容不会删除：

- `/var/lib/dockpilot`
- `/var/lib/dockpilot-agent`
- Docker named volumes

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

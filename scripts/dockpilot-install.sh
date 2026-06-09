#!/usr/bin/env bash
set -euo pipefail

REPO="${DOCKPILOT_REPO:-RY-zzcn/DockPilot}"
VERSION="${DOCKPILOT_VERSION:-latest}"
SERVER_IMAGE="${DOCKPILOT_SERVER_IMAGE:-ghcr.io/ry-zzcn/dockpilot-server}"
AGENT_IMAGE="${DOCKPILOT_AGENT_IMAGE:-ghcr.io/ry-zzcn/dockpilot-agent}"
SERVER_ROOT="${DOCKPILOT_SERVER_ROOT:-/opt/dockpilot}"
AGENT_ROOT="${DOCKPILOT_AGENT_ROOT:-/opt/dockpilot-agent}"
SERVER_DATA="${DOCKPILOT_SERVER_DATA:-/var/lib/dockpilot}"
AGENT_DATA="${DOCKPILOT_AGENT_DATA:-/var/lib/dockpilot-agent}"
ENV_DIR="${DOCKPILOT_ENV_DIR:-/etc/dockpilot}"
SERVER_ENV_FILE="${ENV_DIR}/server.env"
AGENT_ENV_FILE="${ENV_DIR}/agent.env"
COMPOSE_FILE="${SERVER_ROOT}/deploy/docker-compose.yml"

ACTION="${1:-}"
if [ $# -gt 0 ]; then
  shift
fi

PUBLIC_URL="${DOCKPILOT_PUBLIC_URL:-}"
SERVER_URL="${DOCKPILOT_SERVER_URL:-}"
REGISTRATION_TOKEN="${DOCKPILOT_REGISTRATION_TOKEN:-${DOCKPILOT_AGENT_REGISTRATION_TOKEN:-}}"
ADMIN_USER="${DOCKPILOT_ADMIN_USER:-admin}"
ADMIN_PASSWORD="${DOCKPILOT_ADMIN_PASSWORD:-}"
AUTH_SECRET="${DOCKPILOT_AUTH_SECRET:-}"
NODE_NAME="${DOCKPILOT_NODE_NAME:-$(hostname)}"
COMPOSE_DIRS="${DOCKPILOT_COMPOSE_DIRS:-/opt,/srv,/var/www}"
ASSUME_YES="${DOCKPILOT_YES:-0}"
PURGE="${DOCKPILOT_PURGE:-0}"

usage() {
  cat <<'EOF'
DockPilot installer

Usage:
  dockpilot-install.sh install-server-docker [--public-url URL] [--admin-password PASS] [--registration-token TOKEN]
  dockpilot-install.sh install-server-binary [--public-url URL] [--admin-password PASS] [--registration-token TOKEN] [--version VERSION]
  dockpilot-install.sh install-agent-docker --server-url URL --registration-token TOKEN [--node-name NAME]
  dockpilot-install.sh install-agent-binary --server-url URL --registration-token TOKEN [--node-name NAME] [--version VERSION]
  dockpilot-install.sh uninstall [--purge]

Examples:
  curl -fsSL https://raw.githubusercontent.com/RY-zzcn/DockPilot/main/scripts/dockpilot-install.sh | bash -s -- install-server-docker
  curl -fsSL https://raw.githubusercontent.com/RY-zzcn/DockPilot/main/scripts/dockpilot-install.sh | bash -s -- install-agent-binary --server-url http://1.2.3.4:8080 --registration-token YOUR_TOKEN
EOF
}

log() {
  printf '[DockPilot] %s\n' "$*"
}

die() {
  printf '[DockPilot] ERROR: %s\n' "$*" >&2
  exit 1
}

need_root() {
  if [ "$(id -u)" -ne 0 ]; then
    die "please run as root"
  fi
}

random_secret() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 24
    return
  fi
  if command -v python3 >/dev/null 2>&1; then
    python3 -c 'import secrets; print(secrets.token_hex(24))'
    return
  fi
  seed="$(date +%s%N)-${RANDOM:-0}-$(hostname)"
  printf '%s' "$seed" | sha256sum | awk '{print $1}'
}

first_ip() {
  hostname -I 2>/dev/null | awk '{print $1}'
}

ask() {
  prompt="$1"
  default="${2:-}"
  if [ "$ASSUME_YES" = "1" ] && [ -n "$default" ]; then
    printf '%s\n' "$default"
    return
  fi
  if [ -n "$default" ]; then
    read -r -p "${prompt} [${default}]: " value
    printf '%s\n' "${value:-$default}"
  else
    read -r -p "${prompt}: " value
    printf '%s\n' "$value"
  fi
}

parse_args() {
  while [ $# -gt 0 ]; do
    case "$1" in
      --public-url)
        PUBLIC_URL="$2"
        shift 2
        ;;
      --server-url)
        SERVER_URL="$2"
        shift 2
        ;;
      --registration-token)
        REGISTRATION_TOKEN="$2"
        shift 2
        ;;
      --admin-user)
        ADMIN_USER="$2"
        shift 2
        ;;
      --admin-password)
        ADMIN_PASSWORD="$2"
        shift 2
        ;;
      --auth-secret)
        AUTH_SECRET="$2"
        shift 2
        ;;
      --node-name)
        NODE_NAME="$2"
        shift 2
        ;;
      --compose-dirs)
        COMPOSE_DIRS="$2"
        shift 2
        ;;
      --version)
        VERSION="$2"
        shift 2
        ;;
      --yes|-y)
        ASSUME_YES=1
        shift
        ;;
      --purge)
        PURGE=1
        shift
        ;;
      --help|-h)
        usage
        exit 0
        ;;
      *)
        die "unknown option: $1"
        ;;
    esac
  done
}

detect_suffix() {
  machine="$(uname -m)"
  case "$machine" in
    x86_64|amd64)
      printf 'linux_amd64'
      ;;
    aarch64|arm64)
      printf 'linux_arm64'
      ;;
    armv7l|armv7*)
      printf 'linux_armv7'
      ;;
    armv6l|armv6*)
      printf 'linux_armv6'
      ;;
    i386|i686)
      printf 'linux_386'
      ;;
    riscv64)
      printf 'linux_riscv64'
      ;;
    *)
      die "unsupported architecture: ${machine}"
      ;;
  esac
}

clean_version() {
  if [ "$VERSION" = "latest" ]; then
    if command -v curl >/dev/null 2>&1; then
      latest="$(curl -fsSL -A DockPilot-Installer "https://api.github.com/repos/${REPO}/releases/latest" | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)"
    elif command -v wget >/dev/null 2>&1; then
      latest="$(wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)"
    else
      die "curl or wget is required"
    fi
    [ -n "$latest" ] || die "cannot resolve latest release"
    printf '%s' "${latest#v}"
    return
  fi
  printf '%s' "${VERSION#v}"
}

asset_url() {
  asset="$1"
  if [ "$VERSION" = "latest" ]; then
    printf 'https://github.com/%s/releases/latest/download/%s' "$REPO" "$asset"
    return
  fi
  printf 'https://github.com/%s/releases/download/v%s/%s' "$REPO" "$(clean_version)" "$asset"
}

download_asset() {
  asset="$1"
  destination="$2"
  url="$(asset_url "$asset")"
  log "downloading ${asset}"
  if command -v curl >/dev/null 2>&1; then
    curl -fL "$url" -o "$destination"
  elif command -v wget >/dev/null 2>&1; then
    wget -O "$destination" "$url"
  else
    die "curl or wget is required"
  fi
}

ensure_docker() {
  if command -v docker >/dev/null 2>&1; then
    return
  fi
  log "Docker not found, installing with get.docker.com"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL https://get.docker.com | sh
  elif command -v wget >/dev/null 2>&1; then
    wget -qO- https://get.docker.com | sh
  else
    die "curl or wget is required to install Docker"
  fi
  systemctl enable --now docker >/dev/null 2>&1 || true
}

ensure_dockpilot_user() {
  if ! id dockpilot >/dev/null 2>&1; then
    useradd --system --home "$SERVER_DATA" --shell /usr/sbin/nologin dockpilot
  fi
}

write_server_env() {
  mkdir -p "$ENV_DIR"
  if [ -z "$PUBLIC_URL" ]; then
    default_ip="$(first_ip)"
    PUBLIC_URL="$(ask "Server public URL" "http://${default_ip:-127.0.0.1}:8080")"
  fi
  if [ -z "$ADMIN_PASSWORD" ]; then
    ADMIN_PASSWORD="$(random_secret)"
  fi
  if [ -z "$AUTH_SECRET" ]; then
    AUTH_SECRET="$(random_secret)"
  fi
  if [ -z "$REGISTRATION_TOKEN" ]; then
    REGISTRATION_TOKEN="$(random_secret)"
  fi
  cat >"$SERVER_ENV_FILE" <<EOF
TZ=Asia/Shanghai
DOCKPILOT_TIMEZONE=Asia/Shanghai
DOCKPILOT_ADDR=:8080
DOCKPILOT_DATA_DIR=${SERVER_DATA}
DOCKPILOT_DB=${SERVER_DATA}/dockpilot.db
DOCKPILOT_WEB_DIST=${SERVER_ROOT}/web/dist
DOCKPILOT_PUBLIC_URL=${PUBLIC_URL}
DOCKPILOT_ADMIN_USER=${ADMIN_USER}
DOCKPILOT_ADMIN_PASSWORD=${ADMIN_PASSWORD}
DOCKPILOT_AUTH_SECRET=${AUTH_SECRET}
DOCKPILOT_AGENT_REGISTRATION_TOKEN=${REGISTRATION_TOKEN}
EOF
  chmod 600 "$SERVER_ENV_FILE"
}

write_agent_env() {
  mkdir -p "$ENV_DIR"
  if [ -z "$SERVER_URL" ]; then
    SERVER_URL="$(ask "Server URL" "")"
  fi
  if [ -z "$REGISTRATION_TOKEN" ]; then
    REGISTRATION_TOKEN="$(ask "Agent registration token" "")"
  fi
  [ -n "$SERVER_URL" ] || die "--server-url is required"
  [ -n "$REGISTRATION_TOKEN" ] || die "--registration-token is required"
  cat >"$AGENT_ENV_FILE" <<EOF
TZ=Asia/Shanghai
DOCKPILOT_SERVER_URL=${SERVER_URL}
DOCKPILOT_REGISTRATION_TOKEN=${REGISTRATION_TOKEN}
DOCKPILOT_STATE_PATH=${AGENT_DATA}/agent.json
DOCKPILOT_NODE_NAME=${NODE_NAME}
DOCKPILOT_COMPOSE_DIRS=${COMPOSE_DIRS}
EOF
  chmod 600 "$AGENT_ENV_FILE"
}

write_server_service() {
  cat >/etc/systemd/system/dockpilot-server.service <<EOF
[Unit]
Description=DockPilot Server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=dockpilot
Group=dockpilot
WorkingDirectory=${SERVER_ROOT}
EnvironmentFile=-${SERVER_ENV_FILE}
ExecStart=${SERVER_ROOT}/dockpilot-server
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
}

write_agent_service() {
  cat >/etc/systemd/system/dockpilot-agent.service <<EOF
[Unit]
Description=DockPilot Agent
After=network-online.target docker.service
Wants=network-online.target docker.service

[Service]
Type=simple
User=root
EnvironmentFile=-${AGENT_ENV_FILE}
ExecStart=${AGENT_ROOT}/dockpilot-agent
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
}

write_compose_file() {
  mkdir -p "$(dirname "$COMPOSE_FILE")"
  cat >"$COMPOSE_FILE" <<'EOF'
services:
  server:
    image: ghcr.io/ry-zzcn/dockpilot-server:${DOCKPILOT_IMAGE_TAG:-latest}
    container_name: dockpilot-server
    restart: unless-stopped
    ports:
      - "8080:8080"
    env_file:
      - .env
    volumes:
      - dockpilot-data:/data

  local-agent:
    image: ghcr.io/ry-zzcn/dockpilot-agent:${DOCKPILOT_IMAGE_TAG:-latest}
    container_name: dockpilot-agent
    restart: unless-stopped
    profiles:
      - agent
    env_file:
      - .env
    environment:
      DOCKPILOT_SERVER_URL: ${DOCKPILOT_PUBLIC_URL:-http://server:8080}
      DOCKPILOT_REGISTRATION_TOKEN: ${DOCKPILOT_AGENT_REGISTRATION_TOKEN:-change-me-registration-token}
      DOCKPILOT_NODE_NAME: ${HOSTNAME:-local-vps}
      DOCKPILOT_COMPOSE_DIRS: /opt,/srv,/var/www
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - /opt:/opt
      - /srv:/srv
      - /var/www:/var/www
      - dockpilot-agent-state:/data
    depends_on:
      - server

volumes:
  dockpilot-data:
  dockpilot-agent-state:
EOF
}

write_compose_env() {
  mkdir -p "$(dirname "$COMPOSE_FILE")"
  image_tag="latest"
  if [ "$VERSION" != "latest" ]; then
    image_tag="v$(clean_version)"
  fi
  write_server_env
  cat >"$(dirname "$COMPOSE_FILE")/.env" <<EOF
TZ=Asia/Shanghai
DOCKPILOT_TIMEZONE=Asia/Shanghai
DOCKPILOT_IMAGE_TAG=${image_tag}
DOCKPILOT_PUBLIC_URL=${PUBLIC_URL}
DOCKPILOT_ADMIN_USER=${ADMIN_USER}
DOCKPILOT_ADMIN_PASSWORD=${ADMIN_PASSWORD}
DOCKPILOT_AUTH_SECRET=${AUTH_SECRET}
DOCKPILOT_AGENT_REGISTRATION_TOKEN=${REGISTRATION_TOKEN}
EOF
  chmod 600 "$(dirname "$COMPOSE_FILE")/.env"
}

install_server_docker() {
  need_root
  suffix="$(detect_suffix)"
  case "$suffix" in
    linux_amd64|linux_arm64) ;;
    *) die "Docker images currently support linux_amd64 and linux_arm64. Use an Agent binary on ${suffix}, or build images locally." ;;
  esac
  ensure_docker
  write_compose_file
  write_compose_env
  docker compose -f "$COMPOSE_FILE" up -d server
  log "server URL: ${PUBLIC_URL}"
  log "admin user: ${ADMIN_USER}"
  log "admin password: ${ADMIN_PASSWORD}"
  log "agent registration token: ${REGISTRATION_TOKEN}"
}

install_server_binary() {
  need_root
  suffix="$(detect_suffix)"
  case "$suffix" in
    linux_amd64|linux_arm64) ;;
    *) die "server binary packages currently support linux_amd64 and linux_arm64. Use Docker or build from source on ${suffix}." ;;
  esac
  version_clean="$(clean_version)"
  asset="dockpilot-server_${version_clean}_${suffix}.tar.gz"
  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' EXIT
  download_asset "$asset" "${tmp}/${asset}"
  tar -xzf "${tmp}/${asset}" -C "$tmp"
  ensure_dockpilot_user
  mkdir -p "$SERVER_ROOT" "$SERVER_DATA"
  install -m 0755 "${tmp}/dockpilot-server" "${SERVER_ROOT}/dockpilot-server"
  rm -rf "${SERVER_ROOT}/web"
  cp -R "${tmp}/web" "${SERVER_ROOT}/web"
  write_server_env
  write_server_service
  chown -R dockpilot:dockpilot "$SERVER_ROOT" "$SERVER_DATA"
  systemctl daemon-reload
  systemctl enable --now dockpilot-server
  log "server URL: ${PUBLIC_URL}"
  log "admin user: ${ADMIN_USER}"
  log "admin password: ${ADMIN_PASSWORD}"
  log "agent registration token: ${REGISTRATION_TOKEN}"
}

install_agent_docker() {
  need_root
  suffix="$(detect_suffix)"
  case "$suffix" in
    linux_amd64|linux_arm64) ;;
    *) die "Agent Docker images currently support linux_amd64 and linux_arm64. Use install-agent-binary on ${suffix}." ;;
  esac
  ensure_docker
  write_agent_env
  image_tag="latest"
  if [ "$VERSION" != "latest" ]; then
    image_tag="v$(clean_version)"
  fi
  docker rm -f dockpilot-agent >/dev/null 2>&1 || true
  docker run -d --name dockpilot-agent --restart unless-stopped \
    --env-file "$AGENT_ENV_FILE" \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v /opt:/opt \
    -v /srv:/srv \
    -v /var/www:/var/www \
    -v dockpilot-agent-state:/data \
    "${AGENT_IMAGE}:${image_tag}"
  log "agent container started"
}

install_agent_binary() {
  need_root
  suffix="$(detect_suffix)"
  version_clean="$(clean_version)"
  asset="dockpilot-agent_${version_clean}_${suffix}.tar.gz"
  tmp="$(mktemp -d)"
  trap 'rm -rf "$tmp"' EXIT
  download_asset "$asset" "${tmp}/${asset}"
  tar -xzf "${tmp}/${asset}" -C "$tmp"
  mkdir -p "$AGENT_ROOT" "$AGENT_DATA"
  install -m 0755 "${tmp}/dockpilot-agent" "${AGENT_ROOT}/dockpilot-agent"
  write_agent_env
  write_agent_service
  systemctl daemon-reload
  systemctl enable --now dockpilot-agent
  log "agent service started"
}

uninstall_dockpilot() {
  need_root
  systemctl disable --now dockpilot-agent >/dev/null 2>&1 || true
  systemctl disable --now dockpilot-server >/dev/null 2>&1 || true
  rm -f /etc/systemd/system/dockpilot-agent.service /etc/systemd/system/dockpilot-server.service
  systemctl daemon-reload >/dev/null 2>&1 || true
  docker rm -f dockpilot-agent dockpilot-server >/dev/null 2>&1 || true
  if [ -f "$COMPOSE_FILE" ]; then
    docker compose -f "$COMPOSE_FILE" --profile agent down >/dev/null 2>&1 || true
  fi
  if [ "$PURGE" = "1" ]; then
    rm -rf "$SERVER_ROOT" "$AGENT_ROOT" "$SERVER_DATA" "$AGENT_DATA" "$ENV_DIR"
    docker volume rm dockpilot_dockpilot-data dockpilot_dockpilot-agent-state dockpilot-data dockpilot-agent-state >/dev/null 2>&1 || true
    log "DockPilot removed with data purge"
  else
    log "DockPilot services removed. Data is kept in ${SERVER_DATA}, ${AGENT_DATA}, and Docker volumes."
  fi
}

interactive_menu() {
  cat <<'EOF'
DockPilot deployment menu
1) Install server with Docker
2) Install server with binary + systemd
3) Install agent with Docker
4) Install agent with binary + systemd
5) Uninstall DockPilot
EOF
  choice="$(ask "Choose an action" "1")"
  case "$choice" in
    1) ACTION="install-server-docker" ;;
    2) ACTION="install-server-binary" ;;
    3) ACTION="install-agent-docker" ;;
    4) ACTION="install-agent-binary" ;;
    5) ACTION="uninstall" ;;
    *) die "invalid choice" ;;
  esac
}

parse_args "$@"

if [ -z "$ACTION" ]; then
  interactive_menu
fi

case "$ACTION" in
  install-server-docker)
    install_server_docker
    ;;
  install-server-binary)
    install_server_binary
    ;;
  install-agent-docker)
    install_agent_docker
    ;;
  install-agent-binary)
    install_agent_binary
    ;;
  uninstall)
    uninstall_dockpilot
    ;;
  --help|-h|help)
    usage
    ;;
  *)
    usage
    die "unknown action: ${ACTION}"
    ;;
esac

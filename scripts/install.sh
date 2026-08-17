#!/usr/bin/env bash

set -Eeuo pipefail

repo_url="${HEROMAIL_REPOSITORY:-https://github.com/ljunn/heromail.git}"
install_dir="${HEROMAIL_DIR:-/opt/heromail}"
service_port="${HEROMAIL_PORT:-8080}"
bind_address="${HEROMAIL_BIND:-0.0.0.0}"
sudo_cmd=()

log() {
  printf '\033[1;34m[HeroMail]\033[0m %s\n' "$*"
}

die() {
  printf '\033[1;31m[HeroMail]\033[0m %s\n' "$*" >&2
  exit 1
}

install_git() {
  log "正在安装 Git……"
  if command -v apt-get >/dev/null 2>&1; then
    "${sudo_cmd[@]}" apt-get update
    "${sudo_cmd[@]}" apt-get install -y git
  elif command -v dnf >/dev/null 2>&1; then
    "${sudo_cmd[@]}" dnf install -y git
  elif command -v yum >/dev/null 2>&1; then
    "${sudo_cmd[@]}" yum install -y git
  else
    die "无法自动安装 Git，请手动安装后重试。"
  fi
}

check_system() {
  [ "$(uname -s)" = "Linux" ] || die "一键部署脚本当前仅支持 Linux。"
  command -v curl >/dev/null 2>&1 || die "缺少 curl，请先安装后重试。"

  if [ "$(id -u)" -ne 0 ]; then
    command -v sudo >/dev/null 2>&1 || die "写入 ${install_dir} 和安装 Docker 需要 root 或 sudo 权限。"
    sudo_cmd=(sudo)
  fi

  command -v git >/dev/null 2>&1 || install_git
}

install_docker() {
  if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
    return
  fi

  log "未检测到 Docker Compose，正在使用 Docker 官方脚本安装……"
  docker_installer="$(mktemp)"
  trap 'rm -f "${docker_installer:-}"' EXIT
  curl -fsSL https://get.docker.com -o "${docker_installer}"
  "${sudo_cmd[@]}" sh "${docker_installer}"
  rm -f "${docker_installer}"
  trap - EXIT
}

select_docker_command() {
  if docker info >/dev/null 2>&1; then
    docker_cmd=(docker)
  elif [ "${#sudo_cmd[@]}" -gt 0 ] && sudo docker info >/dev/null 2>&1; then
    docker_cmd=(sudo docker)
  else
    die "Docker 已安装但当前无法连接守护进程，请启动 Docker 后重试。"
  fi

  "${docker_cmd[@]}" compose version >/dev/null 2>&1 || die "未找到 Docker Compose 插件。"
}

fetch_project() {
  if [ -d "${install_dir}/.git" ]; then
    die "HeroMail 已安装在 ${install_dir}，后续版本必须先在 GitHub 发布，再使用管理后台的在线升级按钮。"
  fi

  if [ -e "${install_dir}" ] && [ -n "$(ls -A "${install_dir}" 2>/dev/null)" ]; then
    die "安装目录非空且不是 HeroMail 仓库：${install_dir}"
  fi

  release_ref="${HEROMAIL_RELEASE_REF:-}"
  if [ -z "${release_ref}" ]; then
    release_url="$(curl -fsSLI -o /dev/null -w '%{url_effective}' https://github.com/ljunn/heromail/releases/latest)"
    release_ref="${release_url##*/}"
  fi
  [ -n "${release_ref}" ] || die "无法获取 GitHub 最新正式版本。"
  log "正在下载 HeroMail ${release_ref} 到 ${install_dir}……"
  "${sudo_cmd[@]}" mkdir -p "$(dirname "${install_dir}")"
  "${sudo_cmd[@]}" git clone --depth 1 --branch "${release_ref}" "${repo_url}" "${install_dir}"
}

generate_config() {
  if [ -f "${install_dir}/.env" ]; then
    log "保留已有配置：${install_dir}/.env"
    return
  fi

  database_password="$(od -An -N 24 -tx1 /dev/urandom | tr -d ' \n')"
  update_token="$(od -An -N 32 -tx1 /dev/urandom | tr -d ' \n')"
  worker_token="$(od -An -N 32 -tx1 /dev/urandom | tr -d ' \n')"
  admin_password="$(od -An -N 16 -tx1 /dev/urandom | tr -d ' \n')"
  encryption_key="$(od -An -N 32 -tx1 /dev/urandom | tr -d ' \n')"
  detected_host="$(hostname -I 2>/dev/null | awk '{print $1}')"
  [ -n "${detected_host}" ] || detected_host="127.0.0.1"
  public_url="${HEROMAIL_PUBLIC_URL:-http://${detected_host}:${service_port}}"
  log "正在生成数据库密码、管理员密码、升级令牌和加密主密钥……"
  "${sudo_cmd[@]}" tee "${install_dir}/.env" >/dev/null <<EOF
PORT=${service_port}
HEROMAIL_BIND=${bind_address}
COMPOSE_PROJECT_NAME=heromail
HEROMAIL_UPDATE_TOKEN=${update_token}
HEROMAIL_WORKER_TOKEN=${worker_token}
HEROMAIL_ADMIN_EMAIL=admin@heromail.local
HEROMAIL_ADMIN_PASSWORD=${admin_password}
HEROMAIL_ENCRYPTION_KEY=${encryption_key}
HEROMAIL_SEED_DEMO=false
HEROMAIL_PUBLIC_URL=${public_url}
MICROSOFT_CLIENT_ID=
MICROSOFT_CLIENT_SECRET=
MICROSOFT_TENANT=common
MICROSOFT_REDIRECT_URI=
POSTGRES_DB=heromail
POSTGRES_USER=heromail
POSTGRES_PASSWORD=${database_password}
EOF
  "${sudo_cmd[@]}" chmod 600 "${install_dir}/.env"
}

start_services() {
  log "正在拉取 GitHub 正式发布镜像并启动 HeroMail、PostgreSQL 和 Redis……"
  cd "${install_dir}"
  "${docker_cmd[@]}" compose build updater
  "${docker_cmd[@]}" compose pull heromail
  "${docker_cmd[@]}" compose up -d --no-build --remove-orphans

  log "正在等待服务健康检查……"
  for _ in $(seq 1 60); do
    if curl -fsS "http://127.0.0.1:${service_port}/healthz" >/dev/null 2>&1; then
      return
    fi
    sleep 2
  done

  "${docker_cmd[@]}" compose ps
  die "服务在 120 秒内没有就绪，请运行 docker compose logs 查看日志。"
}

show_result() {
  host_address="$(hostname -I 2>/dev/null | awk '{print $1}')"
  [ -n "${host_address}" ] || host_address="服务器地址"
  printf '\n'
  log "部署完成：http://${host_address}:${service_port}"
  printf '%s\n' \
    "安装目录：${install_dir}" \
    "管理员账号：admin@heromail.local" \
    "管理员密码：请在 ${install_dir}/.env 中查看 HEROMAIL_ADMIN_PASSWORD" \
    "查看状态：cd ${install_dir} && ${docker_cmd[*]} compose ps" \
    "查看日志：cd ${install_dir} && ${docker_cmd[*]} compose logs -f heromail" \
    "更新版本：GitHub 发布新版本后，使用管理后台的在线升级按钮" \
    "停止服务：cd ${install_dir} && ${docker_cmd[*]} compose down"
}

main() {
  check_system
  install_docker
  select_docker_command
  fetch_project
  generate_config
  start_services
  show_result
}

main "$@"

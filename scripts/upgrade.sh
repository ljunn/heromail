#!/usr/bin/env bash

set -Eeuo pipefail

APP_HOME="${APP_HOME:-/opt/heromail}"
TARGET_TAG="${1:-}"
ENV_FILE="${APP_HOME}/.env"
BACKUP_DIR="${APP_HOME}/backups"
COMPOSE_FILE="${APP_HOME}/docker-compose.yml"
IMAGE_REPOSITORY="ghcr.io/ljunn/heromail"
TARGET_VERSION=""
TARGET_IMAGE=""
PREVIOUS_IMAGE=""
TARGET_IMAGE_ID=""
BACKUP_PATH=""
TEMP_BACKUP=""
LOCK_FILE=""
COMPOSE_PROJECT=""

log() { printf '[HeroMail 升级] %s\n' "$1"; }
fail() { printf '[HeroMail 升级] 错误：%s\n' "$1" >&2; exit 1; }
read_env() { sed -n "s/^${1}=//p" "${ENV_FILE}" | tail -n 1; }

[[ "$(id -u)" -eq 0 ]] || fail "升级脚本必须由 root 执行。"
[[ "${TARGET_TAG}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || fail "目标版本必须是正式语义化标签。"
[[ -f "${ENV_FILE}" ]] || fail "未找到 ${ENV_FILE}。"
[[ -f "${COMPOSE_FILE}" ]] || fail "未找到 ${COMPOSE_FILE}。"
command -v docker >/dev/null 2>&1 || fail "未找到 Docker。"
command -v curl >/dev/null 2>&1 || fail "未找到 curl。"
command -v flock >/dev/null 2>&1 || fail "未找到 flock。"

TARGET_VERSION="${TARGET_TAG#v}"
TARGET_IMAGE="${IMAGE_REPOSITORY}:${TARGET_VERSION}"
mkdir -p "${BACKUP_DIR}"
chmod 700 "${BACKUP_DIR}"

cd "${APP_HOME}"
compose=(docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}")
if ! "${compose[@]}" config --images | grep -Fxq "${IMAGE_REPOSITORY}:latest"; then
  fail "Compose 配置不是官方 HeroMail latest 镜像，已拒绝升级。"
fi
COMPOSE_PROJECT="$(read_env COMPOSE_PROJECT_NAME)"
COMPOSE_PROJECT="${COMPOSE_PROJECT:-$(basename "${APP_HOME}")}"

# 优先把锁放在 Compose 的共享升级卷中，使 SSH 发布和网页升级器互斥。
upgrade_volume="$(docker volume ls -q \
  --filter "label=com.docker.compose.project=${COMPOSE_PROJECT}" \
  --filter "label=com.docker.compose.volume=upgrade-state" 2>/dev/null | head -n 1 || true)"
upgrade_mount=""
if [[ -n "${upgrade_volume}" ]]; then
  upgrade_mount="$(docker volume inspect --format '{{.Mountpoint}}' "${upgrade_volume}" 2>/dev/null || true)"
fi
if [[ -n "${upgrade_mount}" && -d "${upgrade_mount}" ]]; then
  LOCK_FILE="${upgrade_mount}/.heromail-upgrade.lock"
else
  LOCK_FILE="${APP_HOME}/.heromail-upgrade.lock"
fi
exec 9>"${LOCK_FILE}"
flock -n 9 || fail "已有另一个升级任务正在执行。"

db_user="$(read_env POSTGRES_USER)"
db_name="$(read_env POSTGRES_DB)"
db_user="${db_user:-heromail}"
db_name="${db_name:-heromail}"

audit() {
  local action="$1"
  local detail="$2"
  local audit_id
  audit_id="$(cat /proc/sys/kernel/random/uuid)"
  "${compose[@]}" exec -T postgres psql -v ON_ERROR_STOP=1 -U "${db_user}" -d "${db_name}" \
    --set=audit_id="${audit_id}" --set=action="${action}" --set=detail="${detail}" <<'SQL' >/dev/null
INSERT INTO audit_logs (id, actor_id, action, resource_type, resource_id, detail, ip, created_at)
VALUES (:'audit_id', 'system', :'action', 'system', 'heromail', :'detail', 'github-actions', NOW());
SQL
}

wait_for_health() {
  local attempts=0
  local container_id=""
  local container_state=""
  while (( attempts < 300 )); do
    container_id="$("${compose[@]}" ps -q heromail 2>/dev/null || true)"
    if [[ -n "${container_id}" ]]; then
      container_state="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "${container_id}" 2>/dev/null || true)"
      if [[ "${container_state}" == "healthy" ]]; then
        return 0
      fi
    fi
    attempts=$((attempts + 1))
    sleep 2
  done
  return 1
}

# 同时清理 Compose 已知的停止容器和同一项目残留容器，避免 Docker 名称冲突。
heromail_container_ids() {
  {
    "${compose[@]}" ps -aq heromail 2>/dev/null || true
    docker ps -aq \
      --filter "label=com.docker.compose.project=${COMPOSE_PROJECT}" \
      --filter "label=com.docker.compose.service=heromail" 2>/dev/null || true
    docker ps -aq --filter "name=${COMPOSE_PROJECT}-heromail" 2>/dev/null || true
  } | awk 'NF && !seen[$0]++'
}

remove_heromail_containers() {
  local container_id
  while IFS= read -r container_id; do
    [[ -n "${container_id}" ]] || continue
    docker rm -f "${container_id}" >/dev/null 2>&1 || true
  done < <(heromail_container_ids)
}

start_heromail() {
  # 先释放可能仍被 Docker 占用的服务名，再创建新容器。
  remove_heromail_containers
  "${compose[@]}" up -d --no-deps --no-build heromail
}

find_previous_image() {
  local container_id image_id image_version image_ref version

  while IFS= read -r container_id; do
    [[ -n "${container_id}" ]] || continue
    image_id="$(docker inspect --format '{{.Image}}' "${container_id}" 2>/dev/null || true)"
    [[ -n "${image_id}" ]] || continue
    image_version="$(docker image inspect --format '{{index .Config.Labels "org.opencontainers.image.version"}}' "${image_id}" 2>/dev/null || true)"
    if [[ -n "${image_version}" && "${image_version}" != "${TARGET_VERSION}" ]]; then
      printf '%s\n' "${image_id}"
      return 0
    fi
  done < <(heromail_container_ids)

  # 升级中断后可能没有容器，但之前的正式镜像通常仍保留在本机。
  while IFS= read -r image_ref; do
    version="${image_ref##*:}"
    [[ "${version}" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || continue
    [[ "${version}" == "${TARGET_VERSION}" ]] && continue
    image_id="$(docker image inspect --format '{{.Id}}' "${image_ref}" 2>/dev/null || true)"
    if [[ -n "${image_id}" ]]; then
      printf '%s\n' "${image_id}"
      return 0
    fi
  done < <(docker image ls --format '{{.Repository}}:{{.Tag}}' "${IMAGE_REPOSITORY}" | sort -t: -k2,2Vr)

  return 1
}

rollback() {
  [[ -n "${PREVIOUS_IMAGE}" ]] || return 1
  docker image inspect "${PREVIOUS_IMAGE}" >/dev/null 2>&1 || return 1
  docker tag "${PREVIOUS_IMAGE}" "${IMAGE_REPOSITORY}:latest"
  if ! start_heromail >/dev/null; then
    return 1
  fi
  wait_for_health
}

upgrade_failed() {
  local detail="$1"
  log "${detail}，正在回滚。"
  if rollback; then
    audit "system.upgrade.failed" "${detail}，已恢复上一版本" || true
    fail "${detail}，已恢复上一版本。"
  fi
  audit "system.upgrade.failed" "${detail}，升级和自动回滚均失败" || true
  fail "${detail}，升级和自动回滚均失败，请立即检查 Docker 日志。"
}

log "开始升级到 ${TARGET_TAG}。"
if ! audit "system.upgrade.request" "GitHub Actions 请求升级到 ${TARGET_TAG}"; then
  fail "无法写入升级审计日志，已取消升级。"
fi

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
BACKUP_PATH="${BACKUP_DIR}/heromail-preupgrade-${TARGET_VERSION}-${timestamp}.sql.gz"
TEMP_BACKUP="$(mktemp "${BACKUP_DIR}/.heromail-preupgrade-${TARGET_VERSION}-XXXXXX.sql.gz")"
trap 'rm -f "${TEMP_BACKUP:-}"' EXIT
log "正在创建 PostgreSQL 备份。"
if ! "${compose[@]}" exec -T postgres pg_dump --no-owner --no-privileges -U "${db_user}" -d "${db_name}" | gzip -9 >"${TEMP_BACKUP}"; then
  audit "system.upgrade.backup_failed" "升级 ${TARGET_TAG} 前 PostgreSQL 备份失败" || true
  fail "PostgreSQL 备份失败，升级已取消。"
fi
gzip -t "${TEMP_BACKUP}" || fail "PostgreSQL 备份校验失败，升级已取消。"
chmod 600 "${TEMP_BACKUP}"
mv "${TEMP_BACKUP}" "${BACKUP_PATH}"
TEMP_BACKUP=""

previous_container="$("${compose[@]}" ps -q heromail 2>/dev/null || true)"
if [[ -n "${previous_container}" ]]; then
  PREVIOUS_IMAGE="$(docker inspect --format '{{.Image}}' "${previous_container}" 2>/dev/null || true)"
fi
if [[ -z "${PREVIOUS_IMAGE}" ]]; then
  PREVIOUS_IMAGE="$(find_previous_image || true)"
fi
[[ -n "${PREVIOUS_IMAGE}" ]] || fail "无法确定当前 HeroMail 镜像，已取消升级。"

log "正在拉取官方镜像 ${TARGET_IMAGE}。"
if ! docker pull "${TARGET_IMAGE}" >/dev/null; then
  fail "官方镜像拉取失败，升级已取消。"
fi
TARGET_IMAGE_ID="$(docker image inspect --format '{{.Id}}' "${TARGET_IMAGE}" 2>/dev/null || true)"
[[ -n "${TARGET_IMAGE_ID}" ]] || fail "无法确定目标镜像，升级已取消。"
docker tag "${TARGET_IMAGE}" "${IMAGE_REPOSITORY}:latest"
if ! start_heromail >/dev/null; then
  upgrade_failed "新版本容器启动失败"
fi

if ! wait_for_health; then
  upgrade_failed "升级 ${TARGET_TAG} 健康检查失败"
fi

container_id="$("${compose[@]}" ps -q heromail)"
running_version="$(docker inspect --format '{{index .Config.Labels "org.opencontainers.image.version"}}' "${container_id}" 2>/dev/null || true)"
if [[ "${running_version}" != "${TARGET_VERSION}" ]]; then
  upgrade_failed "升级 ${TARGET_TAG} 版本标签校验失败（实际：${running_version:-未知}）"
fi

port="$(read_env PORT)"
port="${port:-8080}"
if ! curl --fail --silent --show-error --retry 10 --retry-delay 3 "http://127.0.0.1:${port}/readyz" >/dev/null; then
  upgrade_failed "升级 ${TARGET_TAG} 就绪检查失败"
fi
audit "system.upgrade.success" "升级到 ${TARGET_TAG} 完成，备份文件 ${BACKUP_PATH}，健康检查通过"
log "线上版本 ${TARGET_TAG} 已通过健康检查，备份：${BACKUP_PATH}。"

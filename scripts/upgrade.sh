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
BACKUP_PATH=""
TEMP_BACKUP=""
LOCK_FILE="${APP_HOME}/.heromail-upgrade.lock"

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
exec 9>"${LOCK_FILE}"
flock -n 9 || fail "已有另一个升级任务正在执行。"

cd "${APP_HOME}"
compose=(docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}")
if ! "${compose[@]}" config --images | grep -Fxq "${IMAGE_REPOSITORY}:latest"; then
  fail "Compose 配置不是官方 HeroMail latest 镜像，已拒绝升级。"
fi

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
  while (( attempts < 60 )); do
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

rollback() {
  [[ -n "${PREVIOUS_IMAGE}" ]] || return 1
  docker image inspect "${PREVIOUS_IMAGE}" >/dev/null 2>&1 || return 1
  docker tag "${PREVIOUS_IMAGE}" "${IMAGE_REPOSITORY}:latest"
  "${compose[@]}" up -d --no-deps --no-build --force-recreate heromail >/dev/null
  wait_for_health
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
[[ -n "${PREVIOUS_IMAGE}" ]] || fail "无法确定当前 HeroMail 镜像，已取消升级。"

log "正在拉取官方镜像 ${TARGET_IMAGE}。"
docker pull "${TARGET_IMAGE}" >/dev/null
docker tag "${TARGET_IMAGE}" "${IMAGE_REPOSITORY}:latest"
"${compose[@]}" up -d --no-deps --no-build --force-recreate heromail >/dev/null

if ! wait_for_health; then
  log "新版本健康检查失败，正在回滚。"
  if rollback; then
    audit "system.upgrade.failed" "升级 ${TARGET_TAG} 健康检查失败，已恢复上一版本" || true
    fail "升级失败，已恢复上一版本。"
  fi
  audit "system.upgrade.failed" "升级 ${TARGET_TAG} 与自动回滚均失败" || true
  fail "升级和自动回滚均失败，请立即检查 Docker 日志。"
fi

container_id="$("${compose[@]}" ps -q heromail)"
running_version="$(docker inspect --format '{{index .Config.Labels "org.opencontainers.image.version"}}' "${container_id}" 2>/dev/null || true)"
if [[ "${running_version}" != "${TARGET_VERSION}" ]]; then
  log "镜像标签版本校验失败（实际：${running_version:-未知}），正在回滚。"
  if rollback; then
    audit "system.upgrade.failed" "升级 ${TARGET_TAG} 版本标签校验失败，已恢复上一版本" || true
    fail "升级版本校验失败，已恢复上一版本。"
  fi
  audit "system.upgrade.failed" "升级 ${TARGET_TAG} 版本校验和自动回滚均失败" || true
  fail "升级版本校验和自动回滚均失败。"
fi

port="$(read_env PORT)"
port="${port:-8080}"
curl --fail --silent --show-error --retry 10 --retry-delay 3 "http://127.0.0.1:${port}/readyz" >/dev/null
audit "system.upgrade.success" "升级到 ${TARGET_TAG} 完成，备份文件 ${BACKUP_PATH}，健康检查通过"
log "线上版本 ${TARGET_TAG} 已通过健康检查，备份：${BACKUP_PATH}。"

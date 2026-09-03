#!/usr/bin/env bash

set -Eeuo pipefail

APP_HOME="${APP_HOME:-/opt/heromail}"
ENV_FILE="${APP_HOME}/.env"
COMPOSE_FILE="${APP_HOME}/docker-compose.yml"
IMAGE_REPOSITORY="ghcr.io/ljunn/heromail"

fail() {
  printf '[HeroMail 恢复] 错误：%s\n' "$1" >&2
  exit 1
}

[[ "$(id -u)" -eq 0 ]] || fail "恢复脚本必须由 root 执行。"
[[ -f "${ENV_FILE}" ]] || fail "未找到 ${ENV_FILE}。"
[[ -f "${COMPOSE_FILE}" ]] || fail "未找到 ${COMPOSE_FILE}。"
command -v docker >/dev/null 2>&1 || fail "未找到 Docker。"
command -v curl >/dev/null 2>&1 || fail "未找到 curl。"

read_env() { sed -n "s/^${1}=//p" "${ENV_FILE}" | tail -n 1; }

cd "${APP_HOME}"
compose=(docker compose --env-file "${ENV_FILE}" -f "${COMPOSE_FILE}")
"${compose[@]}" version >/dev/null 2>&1 || fail "未找到 Docker Compose 插件。"
if ! "${compose[@]}" config --images | grep -Fxq "${IMAGE_REPOSITORY}:latest"; then
  fail "Compose 配置不是官方 HeroMail latest 镜像，已拒绝恢复。"
fi

compose_project="$(read_env COMPOSE_PROJECT_NAME)"
compose_project="${compose_project:-$(basename "${APP_HOME}")}"

heromail_container_ids() {
  {
    "${compose[@]}" ps -aq heromail 2>/dev/null || true
    docker ps -aq \
      --filter "label=com.docker.compose.project=${compose_project}" \
      --filter "label=com.docker.compose.service=heromail" 2>/dev/null || true
    docker ps -aq --filter "name=${compose_project}-heromail" 2>/dev/null || true
  } | awk 'NF && !seen[$0]++'
}

remove_heromail_containers() {
  local container_id
  while IFS= read -r container_id; do
    [[ -n "${container_id}" ]] || continue
    docker rm -f "${container_id}" >/dev/null 2>&1 || true
  done < <(heromail_container_ids)
}

wait_for_health() {
  local attempts=0
  local container_id=""
  local state=""
  while (( attempts < 300 )); do
    container_id="$("${compose[@]}" ps -q heromail 2>/dev/null || true)"
    if [[ -n "${container_id}" ]]; then
      state="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "${container_id}" 2>/dev/null || true)"
      [[ "${state}" == "healthy" ]] && return 0
    fi
    attempts=$((attempts + 1))
    sleep 2
  done
  return 1
}

printf '[HeroMail 恢复] 正在确保 PostgreSQL 和 Redis 运行。\n'
"${compose[@]}" up -d --no-build postgres redis >/dev/null
printf '[HeroMail 恢复] 正在拉取并启动官方 HeroMail 镜像。\n'
docker pull "${IMAGE_REPOSITORY}:latest" >/dev/null || fail "官方镜像拉取失败。"
remove_heromail_containers
"${compose[@]}" up -d --no-build heromail >/dev/null || fail "HeroMail 容器启动失败。"
wait_for_health || fail "HeroMail 容器在 10 分钟内未通过健康检查。"

port="$(read_env PORT)"
port="${port:-8080}"
curl --fail --silent --show-error --retry 10 --retry-delay 3 "http://127.0.0.1:${port}/readyz" >/dev/null || fail "HeroMail 就绪检查失败。"
printf '[HeroMail 恢复] 服务已恢复，官方镜像通过健康检查。\n'

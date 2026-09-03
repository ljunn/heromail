#!/bin/sh

set -eu

request_path="/upgrade/request.json"
processing_path="/upgrade/request.processing.json"
status_path="/upgrade/status.json"
compose_project="${COMPOSE_PROJECT_NAME:-heromail}"

write_status() {
  state="$1"
  message="$2"
  updated_at="$(date -u +'%Y-%m-%dT%H:%M:%SZ')"
  temporary_path="/upgrade/.status.$$"
  printf '{"state":"%s","message":"%s","updated_at":"%s"}\n' "${state}" "${message}" "${updated_at}" >"${temporary_path}"
  chmod 644 "${temporary_path}"
  mv "${temporary_path}" "${status_path}"
}

wait_for_health() {
  attempts=0
  while [ "${attempts}" -lt 300 ]; do
    container_id="$(cd /workspace && docker compose ps -q heromail 2>/dev/null || true)"
    if [ -n "${container_id}" ]; then
      container_state="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "${container_id}" 2>/dev/null || true)"
      if [ "${container_state}" = "healthy" ]; then
        return 0
      fi
    fi
    attempts=$((attempts + 1))
    sleep 2
  done
  return 1
}

# 清理 Compose 已知的停止容器和同一项目残留容器，避免重建时发生名称冲突。
heromail_container_ids() {
  {
    (cd /workspace && docker compose ps -aq heromail 2>/dev/null || true)
    docker ps -aq \
      --filter "label=com.docker.compose.project=${compose_project}" \
      --filter "label=com.docker.compose.service=heromail" 2>/dev/null || true
    docker ps -aq --filter "name=${compose_project}-heromail" 2>/dev/null || true
  } | awk 'NF && !seen[$0]++'
}

remove_heromail_containers() {
  container_id=""
  while IFS= read -r container_id; do
    [ -n "${container_id}" ] || continue
    docker rm -f "${container_id}" >/dev/null 2>&1 || true
  done <<EOF
$(heromail_container_ids)
EOF
}

start_heromail() {
  remove_heromail_containers
  (cd /workspace && docker compose up -d --no-deps --no-build heromail)
}

find_previous_image() {
  exclude_version="${1:-}"
  container_id=""
  image_id=""
  image_version=""
  image_ref=""
  while IFS= read -r container_id; do
    [ -n "${container_id}" ] || continue
    image_id="$(docker inspect --format '{{.Image}}' "${container_id}" 2>/dev/null || true)"
    [ -n "${image_id}" ] || continue
    image_version="$(docker image inspect --format '{{index .Config.Labels "org.opencontainers.image.version"}}' "${image_id}" 2>/dev/null || true)"
    [ -n "${image_version}" ] || continue
    [ -n "${exclude_version}" ] && [ "${image_version}" = "${exclude_version}" ] && continue
    [ -n "${target_image_id:-}" ] && [ "${image_id}" = "${target_image_id}" ] && continue
    printf '%s\n' "${image_id}"
    return 0
  done <<EOF
$(heromail_container_ids)
EOF

  # 在线升级中断后没有容器时，优先使用本机保留的正式版本镜像。
  while IFS= read -r image_ref; do
    case "${image_ref##*:}" in
      latest|'') continue ;;
      *.*.*)
        image_id="$(docker image inspect --format '{{.Id}}' "${image_ref}" 2>/dev/null || true)"
        if [ -n "${image_id}" ]; then
          printf '%s\n' "${image_id}"
          return 0
        fi
        ;;
    esac
  done <<EOF
$(docker image ls --format '{{.Repository}}:{{.Tag}}' ghcr.io/ljunn/heromail 2>/dev/null | sort -t: -k2,2Vr)
EOF
  return 1
}

mkdir -p /upgrade
command -v flock >/dev/null 2>&1 || {
  write_status "failed" "升级器缺少 flock 锁工具"
  exit 1
}
exec 9>/upgrade/.heromail-upgrade.lock
if [ ! -f "${status_path}" ]; then
  write_status "idle" "等待在线升级任务"
fi

while true; do
  if [ -f "${request_path}" ] && flock -n 9 && mv "${request_path}" "${processing_path}" 2>/dev/null; then
    write_status "updating" "正在拉取最新镜像并重启服务"
    if ! (cd /workspace && docker compose config --images | grep -Fxq 'ghcr.io/ljunn/heromail:latest'); then
      write_status "failed" "升级已拒绝：只允许 GitHub 正式发布的 HeroMail 镜像"
    else
      previous_container="$(cd /workspace && docker compose ps -aq heromail 2>/dev/null | head -n 1 || true)"
      previous_image=""
      if [ -n "${previous_container}" ]; then
        previous_image="$(docker inspect --format '{{.Image}}' "${previous_container}" 2>/dev/null || true)"
      fi
      target_version=""
      target_image_id=""

      if ! (cd /workspace && docker pull ghcr.io/ljunn/heromail:latest); then
        write_status "failed" "官方镜像拉取失败，升级已取消"
      else
        target_version="$(docker image inspect --format '{{index .Config.Labels "org.opencontainers.image.version"}}' ghcr.io/ljunn/heromail:latest 2>/dev/null || true)"
        target_image_id="$(docker image inspect --format '{{.Id}}' ghcr.io/ljunn/heromail:latest 2>/dev/null || true)"
        if [ "${previous_image}" = "${target_image_id}" ] && [ -n "${target_image_id}" ]; then
          previous_image=""
          write_status "success" "当前已是目标官方镜像，无需重复重启"
          rm -f "${processing_path}"
          flock -u 9 || true
          sleep 2
          continue
        fi

        if start_heromail && wait_for_health; then
          write_status "success" "在线升级完成，新版本已通过健康检查"
        elif [ -z "${previous_image}" ]; then
          previous_image="$(find_previous_image "${target_version}" || true)"
          if [ -n "${previous_image}" ] && docker image inspect "${previous_image}" >/dev/null 2>&1; then
            write_status "updating" "新版本健康检查失败，正在自动回滚"
            if docker tag "${previous_image}" ghcr.io/ljunn/heromail:latest && start_heromail && wait_for_health; then
              write_status "failed" "新版本未通过健康检查，已自动恢复上一版本"
            else
              write_status "failed" "升级与自动回滚均失败，请立即检查 updater 日志"
            fi
          else
            write_status "failed" "升级失败且没有可用的上一镜像，请立即检查 updater 日志"
          fi
        elif [ -n "${previous_image}" ] && docker image inspect "${previous_image}" >/dev/null 2>&1; then
          write_status "updating" "新版本健康检查失败，正在自动回滚"
          if docker tag "${previous_image}" ghcr.io/ljunn/heromail:latest && start_heromail && wait_for_health; then
            write_status "failed" "新版本未通过健康检查，已自动恢复上一版本"
          else
            write_status "failed" "升级与自动回滚均失败，请立即检查 updater 日志"
          fi
        else
          write_status "failed" "升级失败且没有可用的上一镜像，请立即检查 updater 日志"
        fi
      fi
    fi
    rm -f "${processing_path}"
    flock -u 9 || true
  elif [ -f "${request_path}" ]; then
    # 另一个升级入口正在执行，保留请求等待下一轮处理。
    flock -u 9 || true
  fi
  sleep 2
done

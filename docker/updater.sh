#!/bin/sh

set -eu

request_path="/upgrade/request.json"
processing_path="/upgrade/request.processing.json"
status_path="/upgrade/status.json"

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
  while [ "${attempts}" -lt 60 ]; do
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

mkdir -p /upgrade
if [ ! -f "${status_path}" ]; then
  write_status "idle" "等待在线升级任务"
fi

while true; do
  if [ -f "${request_path}" ] && mv "${request_path}" "${processing_path}" 2>/dev/null; then
    write_status "updating" "正在拉取最新镜像并重启服务"
    if ! (cd /workspace && docker compose config --images | grep -Fxq 'ghcr.io/ljunn/heromail:latest'); then
      write_status "failed" "升级已拒绝：只允许 GitHub 正式发布的 HeroMail 镜像"
    else
      previous_container="$(cd /workspace && docker compose ps -q heromail 2>/dev/null || true)"
      previous_image=""
      if [ -n "${previous_container}" ]; then
        previous_image="$(docker inspect --format '{{.Image}}' "${previous_container}" 2>/dev/null || true)"
      fi

      if cd /workspace && docker pull ghcr.io/ljunn/heromail:latest && docker compose up -d --no-deps --no-build --force-recreate heromail && wait_for_health; then
        write_status "success" "在线升级完成，新版本已通过健康检查"
      elif [ -n "${previous_image}" ] && docker image inspect "${previous_image}" >/dev/null 2>&1; then
        write_status "updating" "新版本健康检查失败，正在自动回滚"
        if docker tag "${previous_image}" ghcr.io/ljunn/heromail:latest && cd /workspace && docker compose up -d --no-deps --no-build --force-recreate heromail && wait_for_health; then
          write_status "failed" "新版本未通过健康检查，已自动恢复上一版本"
        else
          write_status "failed" "升级与自动回滚均失败，请立即检查 updater 日志"
        fi
      else
        write_status "failed" "升级失败且没有可用的上一镜像，请立即检查 updater 日志"
      fi
    fi
    rm -f "${processing_path}"
  fi
  sleep 2
done

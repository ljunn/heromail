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

mkdir -p /upgrade
if [ ! -f "${status_path}" ]; then
  write_status "idle" "等待在线升级任务"
fi

while true; do
  if [ -f "${request_path}" ] && mv "${request_path}" "${processing_path}" 2>/dev/null; then
    write_status "updating" "正在拉取最新镜像并重启服务"
    if ! (cd /workspace && docker compose config --images | grep -Fxq 'ghcr.io/ljunn/heromail:latest'); then
      write_status "failed" "升级已拒绝：只允许 GitHub 正式发布的 HeroMail 镜像"
    elif cd /workspace && docker pull ghcr.io/ljunn/heromail:latest && docker compose up -d --no-deps --no-build --force-recreate heromail; then
      write_status "success" "在线升级完成，服务已使用最新镜像"
    else
      write_status "failed" "在线升级失败，请查看 updater 日志"
    fi
    rm -f "${processing_path}"
  fi
  sleep 2
done

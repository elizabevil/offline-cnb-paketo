#!/usr/bin/env bash
# image-build.sh — 构建单个镜像的原语（不含编排）
#
# 用法:
#   image-build.sh <context> <dockerfile> <tag> [build args...]
#
#   IMAGE_BUILD_DRY_RUN=1 时只打印将执行的命令，便于测试与 CI 干跑。
#   CONTAINER_TOOL=podman|docker 可强制指定运行时。
#
# 为什么不是 just recipe：just 的 recipe 不能被当作 shell 命令调用，
# 而"原语"需要能被任意项目、任意脚本直接执行。
set -euo pipefail

if [ $# -lt 3 ]; then
  echo "用法: $0 <context> <dockerfile> <tag> [build args...]" >&2
  exit 2
fi

context="$1"
dockerfile="$2"
tag="$3"
shift 3

runtime="${CONTAINER_TOOL:-}"
if [ -z "${runtime}" ]; then
  if command -v podman >/dev/null 2>&1; then
    runtime="podman"
  elif command -v docker >/dev/null 2>&1; then
    runtime="docker"
  else
    echo "  ✗ 未找到 podman / docker" >&2
    exit 1
  fi
fi

cmd=("${runtime}" build -f "${dockerfile}" -t "${tag}" "$@" "${context}")

if [ "${IMAGE_BUILD_DRY_RUN:-0}" = "1" ]; then
  printf '%q ' "${cmd[@]}"
  printf '\n'
  exit 0
fi

exec "${cmd[@]}"

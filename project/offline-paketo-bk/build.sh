#!/usr/bin/env bash
# build.sh — offline-paketo-bk 一键入口（配置驱动）
#
# 流水线定义在 config/projects/offline-paketo-bk.toml：
# 物料、版本、镜像、构建顺序都在那里，本脚本只是把它交给执行器。
#
# 用法:
#   ./build.sh                          # 按配置执行整条流水线
#   ./build.sh --dry-run                # 只打印计划与命令，不执行
#   ./build.sh --platform linux-arm64   # 覆盖平台
#   ./build.sh --only builder           # 只构建某个镜像
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
repo="$(cd "${here}/../.." && pwd)"
profile="${repo}/config/projects/offline-paketo-bk.toml"

case "${1:-}" in
  ""|-h|--help)
    if [ -z "${1:-}" ]; then
      exec "${repo}/script/bin/cnb" run "${profile}"
    fi
    sed -n '3,12p' "$0" | sed 's/^# \{0,1\}//'
    exit 0
    ;;
  *)
    exec "${repo}/script/bin/cnb" run "${profile}" "$@"
    ;;
esac

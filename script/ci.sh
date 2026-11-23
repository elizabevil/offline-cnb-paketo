#!/usr/bin/env bash
# ci.sh — 统一门禁：结构 + 配置（默认快速）
# 用法: script/ci.sh [--full] [--fetch]
#   --full   追加 arm64 交叉编译、双平台物料全量校验、arm64 计划
#   --fetch  先拉取全部物料（离线环境可跳过）
set -euo pipefail
cd "$(dirname "$0")/.."

MODE=fast
FETCH=0
for arg in "$@"; do
  case "$arg" in
    --full)  MODE=full ;;
    --fetch) FETCH=1 ;;
    *) echo "未知参数: $arg" >&2; exit 2 ;;
  esac
done

START=$(date +%s)
phase() {
  local now=$(( $(date +%s) - START ))
  echo
  echo "── $* (${now}s) ──"
}

CNB=script/bin/cnb-linux-amd64
LOCK=project/assets.toml

if [ "$FETCH" = 1 ]; then
  phase "拉取物料"
  just fetch
fi

# 仓库不提交二进制；缺失或源码更新时才构建
if [ ! -x "$CNB" ] || [ -n "$(find script/tools/cnb script/tools/bksmartpacker_v3 -name '*.go' -newer "$CNB" -print -quit 2>/dev/null)" ]; then
  phase "构建工具"
  just tools
fi

if [ "${CNB_SKIP_LINT:-0}" = "1" ]; then
  echo "── Go 静态检查跳过（CNB_SKIP_LINT=1，由工作流单独执行） ──"
else
  phase "Go 静态检查"
  just lint
fi

phase "结构自检"
just check

phase "lock 校验"
"$CNB" --file "$LOCK" materials validate

phase "脚手架自检"
tmp=$(mktemp -d)
"$CNB" init ci-template --dir "$tmp/ci-template" >/dev/null
"$CNB" --file "$tmp/ci-template/assets.toml" materials validate
"$CNB" pipeline profile validate "$tmp/ci-template/pipeline.toml"
rm -rf "$tmp"

phase "流程校验"
for f in project/*/pipeline.toml; do
  "$CNB" --file "$LOCK" pipeline profile validate "$f"
done

phase "buildpack 结构检查"
"$CNB" buildpack lint buildpacks/*/*

phase "流水线 lint"
for f in project/*/pipeline.toml; do
  "$CNB" --file "$LOCK" lint "$f"
done

phase "物料快速校验（宿主平台）"
"$CNB" --file "$LOCK" materials verify --quick

if [ "$MODE" = full ]; then
  phase "arm64 工具交叉编译"
  TARGET=linux-arm64 just --justfile script/tools/cnb/justfile build &
  TARGET=linux-arm64 just --justfile script/tools/bksmartpacker_v3/justfile build &
  wait

  phase "arm64 物料全量校验"
  "$CNB" --file "$LOCK" materials verify --platform linux-arm64

  phase "arm64 计划生成"
  "$CNB" --file "$LOCK" pipeline plan project/offline-paketo-bk/pipeline.toml --platform linux-arm64 >/dev/null

  phase "物料全量校验（宿主平台）"
  "$CNB" --file "$LOCK" materials verify
fi

echo
echo "✅ CI 通过（${MODE}，$(( $(date +%s) - START ))s）"

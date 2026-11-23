#!/usr/bin/env bash
# ci.sh — 统一门禁：结构 + 配置（默认快速）
# 用法: script/ci.sh [--full] [--offline] [--fetch]
#   --full     追加 arm64 交叉编译、双平台物料全量校验、arm64 计划
#   --offline  不访问网络：不 fetch；bundle 不存在时跳过物料校验
#   --fetch    先拉取全部物料（需要网络）
set -euo pipefail
cd "$(dirname "$0")/.."

MODE=fast
FETCH=0
OFFLINE=0
for arg in "$@"; do
  case "$arg" in
    --full)    MODE=full ;;
    --fetch)   FETCH=1 ;;
    --offline) OFFLINE=1 ;;
    *) echo "未知参数: $arg" >&2; exit 2 ;;
  esac
done

if [ "$OFFLINE" = 1 ] && [ "$FETCH" = 1 ]; then
  echo "--offline 与 --fetch 不能同时使用" >&2
  exit 2
fi

# bundle 存在且非空时才做物料校验
HAVE_BUNDLE=0
if [ -d bundle ] && [ -n "$(ls -A bundle 2>/dev/null)" ]; then
  HAVE_BUNDLE=1
fi

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

if [ "$OFFLINE" = 1 ]; then
  echo "offline 模式：不访问网络；bundle $( [ "$HAVE_BUNDLE" = 1 ] && echo 已存在 || echo 缺失 )"
fi

# 仓库不提交二进制；缺失或源码更新时才构建
if [ "$OFFLINE" = 1 ]; then
  if [ ! -x "$CNB" ]; then
    echo "offline 模式需要预置 $CNB（不执行 go build，避免访问模块代理）" >&2
    exit 2
  fi
else
  if [ ! -x "$CNB" ] || [ -n "$(find script/tools/cnb script/tools/bksmartpacker_v3 -name '*.go' -newer "$CNB" -print -quit 2>/dev/null)" ]; then
    phase "构建工具"
    just tools
  fi
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

if [ "$HAVE_BUNDLE" = 1 ]; then
  phase "流水线 lint"
  for f in project/*/pipeline.toml; do
    "$CNB" --file "$LOCK" lint "$f"
  done
else
  echo
  echo "── 流水线 lint 跳过：bundle 缺失，无法校验 Dockerfile COPY 来源 ──"
fi

if [ "$HAVE_BUNDLE" = 1 ]; then
  phase "物料快速校验（宿主平台）"
  "$CNB" --file "$LOCK" materials verify --quick
else
  echo
  echo "── 物料校验跳过：bundle 缺失（offline 模式） ──"
fi

if [ "$MODE" = full ]; then
  if [ "$OFFLINE" = 1 ]; then
    echo "── arm64 工具交叉编译跳过（offline；如需请先联网构建） ──"
  else
    phase "arm64 工具交叉编译"
    TARGET=linux-arm64 just --justfile script/tools/cnb/justfile build &
    TARGET=linux-arm64 just --justfile script/tools/bksmartpacker_v3/justfile build &
    wait
  fi

  if [ "$HAVE_BUNDLE" = 1 ]; then
    phase "arm64 物料全量校验"
    "$CNB" --file "$LOCK" materials verify --platform linux-arm64
  else
    echo "── arm64 物料校验跳过：bundle 缺失 ──"
  fi

  phase "arm64 计划生成"
  "$CNB" --file "$LOCK" pipeline plan project/offline-paketo-bk/pipeline.toml --platform linux-arm64 >/dev/null

  if [ "$HAVE_BUNDLE" = 1 ]; then
    phase "物料全量校验（宿主平台）"
    "$CNB" --file "$LOCK" materials verify
  else
    echo "── 物料全量校验跳过：bundle 缺失 ──"
  fi
fi

echo
echo "✅ CI 通过（${MODE}，$(( $(date +%s) - START ))s）"

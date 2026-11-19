#!/usr/bin/env bash
# ci.sh — 统一门禁：结构 + 配置 + 物料
# 用法: script/ci.sh [--fetch]
#   --fetch  先拉取全部物料（离线环境可跳过，仅做结构与配置校验）
set -euo pipefail
cd "$(dirname "$0")/.."

if [ "${1:-}" = "--fetch" ]; then
  echo "── 拉取物料 ──"
  just fetch
fi

echo "── Go 静态检查 ──"
just lint

echo "── 结构自检 ──"
just check

echo "── lock 校验 ──"
script/bin/cnb-linux-amd64 materials validate
tmp=$(mktemp -d); script/bin/cnb-linux-amd64 init ci-template --config "$tmp/pipeline.toml" --assets "$tmp/assets.toml" >/dev/null && script/bin/cnb-linux-amd64 --file "$tmp/assets.toml" materials validate; rm -rf "$tmp"

echo "── 流程校验 ──"
for f in config/projects/*.toml; do
  script/bin/cnb-linux-amd64 pipeline profile validate "$f"
done

echo "── buildpack 结构检查 ──"
script/bin/cnb-linux-amd64 buildpack lint buildpacks/*/*

echo "── 流水线 lint ──"
for f in config/projects/*.toml; do
  script/bin/cnb-linux-amd64 lint "$f"
done

echo "── arm64：工具与物料 ──"
TARGET=linux-arm64 just --justfile script/tools/cnb/justfile build
TARGET=linux-arm64 just --justfile script/tools/bksmartpacker_v3/justfile build
script/bin/cnb-linux-amd64 materials verify --platform linux-arm64
script/bin/cnb-linux-amd64 pipeline plan config/projects/offline-paketo-bk.toml --platform linux-arm64 >/dev/null

echo "── 物料校验（bundle）──"
script/bin/cnb-linux-amd64 materials verify

echo "✅ CI 通过"

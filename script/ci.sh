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

# 仓库不提交二进制；CI 先构建工具
if [ ! -x script/bin/cnb-linux-amd64 ]; then
  echo "── 构建工具 ──"
  just tools
fi

echo "── Go 静态检查 ──"
just lint

echo "── 结构自检 ──"
just check

echo "── lock 校验 ──"
script/bin/cnb-linux-amd64 --file project/assets.toml materials validate
echo "── 脚手架自检 ──"
tmp=$(mktemp -d)
script/bin/cnb-linux-amd64 init ci-template --dir "$tmp/ci-template" >/dev/null
script/bin/cnb-linux-amd64 --file "$tmp/ci-template/assets.toml" materials validate
script/bin/cnb-linux-amd64 pipeline profile validate "$tmp/ci-template/pipeline.toml"
rm -rf "$tmp"

echo "── 流程校验 ──"
for f in project/*/pipeline.toml; do
  script/bin/cnb-linux-amd64 --file project/assets.toml pipeline profile validate "$f"
done

echo "── buildpack 结构检查 ──"
script/bin/cnb-linux-amd64 buildpack lint buildpacks/*/*

echo "── 流水线 lint ──"
for f in project/*/pipeline.toml; do
  script/bin/cnb-linux-amd64 --file project/assets.toml lint "$f"
done

echo "── arm64：工具与物料 ──"
TARGET=linux-arm64 just --justfile script/tools/cnb/justfile build
TARGET=linux-arm64 just --justfile script/tools/bksmartpacker_v3/justfile build
script/bin/cnb-linux-amd64 --file project/assets.toml materials verify --platform linux-arm64
script/bin/cnb-linux-amd64 --file project/assets.toml pipeline plan project/offline-paketo-bk/pipeline.toml --platform linux-arm64 >/dev/null

echo "── 物料校验（bundle）──"
script/bin/cnb-linux-amd64 --file project/assets.toml materials verify

echo "✅ CI 通过"

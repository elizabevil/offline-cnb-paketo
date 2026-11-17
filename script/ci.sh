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

echo "── 结构自检 ──"
just check

echo "── lock 校验 ──"
script/bin/cnb-linux-amd64 materials validate
script/bin/cnb-linux-amd64 --file assets.lock.toml.example validate

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

echo "── 物料校验（按流程）──"
for f in config/projects/*.toml; do
  script/bin/cnb-linux-amd64 pipeline profile verify "$f"
done

echo "✅ CI 通过"

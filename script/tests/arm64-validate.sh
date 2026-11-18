#!/usr/bin/env bash
# arm64 端到端验证（x86 + qemu）：构建 → 打包 → 断言镜像架构
# 用法：script/tests/arm64-validate.sh <release-arm64 目录>
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
APP=${1:?用法: script/tests/arm64-validate.sh <release-arm64 目录>}

echo "== 1. qemu"
podman run --rm --platform linux/arm64 swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/library/debian:trixie-slim-linuxarm64 uname -m

echo "== 2. 工具"
TARGET=linux-arm64 just --justfile "$ROOT/script/tools/cnb/justfile" build
TARGET=linux-arm64 just --justfile "$ROOT/script/tools/bksmartpacker_v3/justfile" build

echo "== 3. 物料"
"$ROOT/script/bin/cnb-linux-amd64" materials verify --platform linux-arm64

echo "== 4. 镜像"
cd "$ROOT/project/offline-paketo-bk"
CNB_BIN="$ROOT/script/bin/cnb-linux-amd64" just build --platform linux-arm64

echo "== 5. 打包"
CNB_BIN="$ROOT/script/bin/cnb-linux-amd64" \
BPS_BIN="$ROOT/script/bin/bksmartpacker-linux-arm64" \
BKS_PLATFORM=linux/arm64 \
BUILDER_IMAGE=offline-builder-build-bk:arm64 \
just smart "$APP"

echo "== 6. 断言"
W=$(mktemp -d)
trap 'rm -rf "$W"' EXIT
tar -xzf dist/s-mart.tar.gz -C "$W" web.tgz
tar -xf "$W/web.tgz" -C "$W"
grep -rl '"architecture": *"arm64"' "$W/blobs" >/dev/null \
  || { echo "✗ 镜像架构不是 arm64"; exit 1; }
echo "✓ arm64 通过：$ROOT/project/offline-paketo-bk/dist/s-mart.tar.gz"

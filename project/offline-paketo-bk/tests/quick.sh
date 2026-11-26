#!/usr/bin/env bash
# 开发期快速打包验证；可通过 BKS_APP / BKS_PLATFORM 覆盖默认值。
set -euo pipefail

DIR=$(cd "$(dirname "$0")" && pwd)
PB=$(cd "$DIR/.." && pwd)
ROOT=$(cd "$PB/../.." && pwd)
APP="$PB/fixtures/single"
PLATFORM="linux-amd64"
[ -n "${BKS_APP:-}" ] && APP="$BKS_APP"
[ -n "${BKS_PLATFORM:-}" ] && PLATFORM="$BKS_PLATFORM"
ARCH="${PLATFORM#linux-}"

echo "app=$APP"
echo "platform=$PLATFORM"

OUT="$ROOT/.cache/tests/offline-paketo-bk/$PLATFORM"
mkdir -p "$OUT"
env BKS_PLATFORM="$PLATFORM" just --justfile "$PB/justfile" smart "$APP"
cp -f "$PB/dist/$PLATFORM/s-mart.tar.gz" "$OUT/s-mart-host-layout-$ARCH.tar.gz"
echo "artifact: $OUT/s-mart-host-layout-$ARCH.tar.gz"

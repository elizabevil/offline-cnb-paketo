#!/usr/bin/env bash
# 开发期快速打包验证；可通过 BKS_APP / BKS_PLATFORM 覆盖默认值。
set -euo pipefail

DIR=$(cd "$(dirname "$0")" && pwd)
PB=$(cd "$DIR/.." && pwd)
APP="$PB/fixtures/single"
PLATFORM="linux-amd64"
[ -n "${BKS_APP:-}" ] && APP="$BKS_APP"
[ -n "${BKS_PLATFORM:-}" ] && PLATFORM="$BKS_PLATFORM"

echo "app=$APP"
echo "platform=$PLATFORM"

exec env BKS_PLATFORM="$PLATFORM" just --justfile "$PB/justfile" smart "$APP"

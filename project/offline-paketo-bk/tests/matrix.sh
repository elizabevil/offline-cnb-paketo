#!/usr/bin/env bash
# runtime 组合矩阵入口；可通过 BKS_APP / BKS_PLATFORM 覆盖默认值。
set -euo pipefail

DIR=$(cd "$(dirname "$0")" && pwd)
PB=$(cd "$DIR/.." && pwd)
ROOT=$(cd "$PB/../.." && pwd)
APP="$PB/fixtures/single"
PLATFORM="linux-amd64"
[ -n "${BKS_APP:-}" ] && APP="$BKS_APP"
[ -n "${BKS_PLATFORM:-}" ] && PLATFORM="$BKS_PLATFORM"
OUT="$ROOT/.cache/tests/offline-paketo-bk/$PLATFORM/matrix"
mkdir -p "$OUT"

echo "matrix app=$APP platform=$PLATFORM"
echo "1) layout/pind 由 quick + export-modes 覆盖"
echo "2) dind 需要显式 runtime 环境；当前脚本只记录入口，不自动猜测"

exec env BKS_APP="$APP" BKS_PLATFORM="$PLATFORM" "$DIR/export-modes.sh"

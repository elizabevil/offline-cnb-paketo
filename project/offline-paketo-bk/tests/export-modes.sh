#!/usr/bin/env bash
# layout / daemon 导出对比；可通过 BKS_APP / BKS_PLATFORM 覆盖默认值。
set -euo pipefail

DIR=$(cd "$(dirname "$0")" && pwd)
PB=$(cd "$DIR/.." && pwd)
ROOT=$(cd "$PB/../.." && pwd)
APP="$PB/fixtures/single"
PLATFORM="linux-amd64"
[ -n "${BKS_APP:-}" ] && APP="$BKS_APP"
[ -n "${BKS_PLATFORM:-}" ] && PLATFORM="$BKS_PLATFORM"
ARCH="${PLATFORM#linux-}"
OUT="$ROOT/.cache/tests/offline-paketo-bk/$PLATFORM"
mkdir -p "$OUT"

echo "app=$APP"
echo "platform=$PLATFORM"
echo "out=$OUT"

echo "===== layout ====="
BKS_PLATFORM="$PLATFORM" just --justfile "$PB/justfile" smart "$APP" >"$OUT/layout.log" 2>&1
echo "layout exit=$?"
cp -f "$PB/dist/$PLATFORM/s-mart.tar.gz" "$OUT/s-mart-host-layout-$ARCH.tar.gz" 2>/dev/null || true

echo "===== daemon ====="
BKS_EXPORT_MODE=daemon BKS_PLATFORM="$PLATFORM" \
  just --justfile "$PB/justfile" smart "$APP" >"$OUT/daemon.log" 2>&1 || true
echo "daemon exit=$?"
cp -f "$PB/dist/$PLATFORM/s-mart.tar.gz" "$OUT/s-mart-host-daemon-$ARCH.tar.gz" 2>/dev/null || true

echo "logs: $OUT/layout.log $OUT/daemon.log"

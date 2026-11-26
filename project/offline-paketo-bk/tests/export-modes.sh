#!/usr/bin/env bash
# layout / daemon 导出对比；日志写入 .cache/tests/offline-paketo-bk/<platform>/
set -euo pipefail

if [ "$#" -ne 2 ]; then
  echo "usage: export-modes.sh <app-dir> <platform>" >&2
  exit 2
fi

APP=$1
PLATFORM=$2
DIR=$(cd "$(dirname "$0")" && pwd)
PB=$(cd "$DIR/.." && pwd)
ROOT=$(cd "$PB/../.." && pwd)
OUT="$ROOT/.cache/tests/offline-paketo-bk/$PLATFORM"
mkdir -p "$OUT"

echo "app=$APP"
echo "platform=$PLATFORM"
echo "out=$OUT"

echo "===== layout ====="
BKS_PLATFORM="$PLATFORM" just --justfile "$PB/justfile" smart "$APP" >"$OUT/layout.log" 2>&1
echo "layout exit=$?"
cp -f "$PB/dist/$PLATFORM/s-mart.tar.gz" "$OUT/layout.tar.gz" 2>/dev/null || true

echo "===== daemon ====="
BKS_EXPORT_MODE=daemon BKS_PLATFORM="$PLATFORM" \
  just --justfile "$PB/justfile" smart "$APP" >"$OUT/daemon.log" 2>&1 || true
echo "daemon exit=$?"
cp -f "$PB/dist/$PLATFORM/s-mart.tar.gz" "$OUT/daemon.tar.gz" 2>/dev/null || true

echo "logs: $OUT/layout.log $OUT/daemon.log"

#!/usr/bin/env bash
# 用真实应用目录跑两种导出模式并保存产物，便于对比。
#
# 用法：script/tests/export-modes.sh /path/to/app-dir
# 日志与产物：.cache/logs/export-modes/
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
APP=${1:-$ROOT/project/offline-paketo-bk/fixtures/single}
OUT="$ROOT/.cache/logs/export-modes"
PB="$ROOT/project/offline-paketo-bk"
mkdir -p "$OUT"

echo "app=$APP"
echo "out=$OUT"

echo "===== layout ====="
( cd "$PB" && just smart "$APP" ) >"$OUT/layout.log" 2>&1
echo "layout exit=$?"
cp -f "$PB/dist/s-mart.tar.gz" "$OUT/layout.tar.gz" 2>/dev/null || true

echo "===== daemon ====="
( cd "$PB" && BKS_EXPORT_MODE=daemon just smart "$APP" ) >"$OUT/daemon.log" 2>&1
echo "daemon exit=$?"
cp -f "$PB/dist/s-mart.tar.gz" "$OUT/daemon.tar.gz" 2>/dev/null || true

echo "日志：$OUT/layout.log  $OUT/daemon.log"

#!/usr/bin/env bash
# runtime 组合矩阵；默认当前平台、当前 fixture。
set -euo pipefail

if [ "$#" -ne 2 ]; then
  echo "usage: matrix.sh <app-dir> <platform>" >&2
  exit 2
fi

APP=$1
PLATFORM=$2
DIR=$(cd "$(dirname "$0")" && pwd)
ROOT=$(cd "$DIR/../../.." && pwd)
OUT="$ROOT/.cache/tests/offline-paketo-bk/$PLATFORM/matrix"
mkdir -p "$OUT"

echo "matrix app=$APP platform=$PLATFORM"
echo "1) layout/pind 由 quick + export-modes 覆盖"
echo "2) dind 需要显式 runtime 环境；当前脚本只记录入口，不自动猜测"

exec "$DIR/export-modes.sh" "$APP" "$PLATFORM"

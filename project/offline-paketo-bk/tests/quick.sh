#!/usr/bin/env bash
# 开发期快速打包验证；正式矩阵请用 matrix.sh。
set -euo pipefail

if [ "$#" -ne 2 ]; then
  echo "usage: quick.sh <app-dir> <platform>" >&2
  exit 2
fi

APP=$1
PLATFORM=$2
DIR=$(cd "$(dirname "$0")" && pwd)
PB=$(cd "$DIR/.." && pwd)

exec env BKS_PLATFORM="$PLATFORM" just --justfile "$PB/justfile" smart "$APP"

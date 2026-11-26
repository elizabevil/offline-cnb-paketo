#!/usr/bin/env bash
# offline-paketo 开发期快速验证：builder + plan 冒烟。
set -euo pipefail

DIR=$(cd "$(dirname "$0")" && pwd)
PB=$(cd "$DIR/.." && pwd)
ROOT=$(cd "$PB/../.." && pwd)
APP="$PB/fixtures/go-app"
PLATFORM="linux-amd64"
[ -n "${BKS_APP:-}" ] && APP="$BKS_APP"
[ -n "${BKS_PLATFORM:-}" ] && PLATFORM="$BKS_PLATFORM"

test -e "$APP" || { echo "app not found: $APP" >&2; exit 1; }
"$ROOT/script/bin/cnb-linux-amd64" pipeline plan \
  "$PB/pipeline.toml" --platform "$PLATFORM" >/dev/null
echo "✓ offline-paketo plan smoke (${PLATFORM})"

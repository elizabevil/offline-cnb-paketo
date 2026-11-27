#!/usr/bin/env bash
# 推荐的非 root managed runtime 测试入口。
set -euo pipefail

MODE="${BKS_TEST_MODE:-pind}"
EXPORT="${BKS_TEST_EXPORT:-layout}"
DIR=$(cd "$(dirname "$0")" && pwd)
PB=$(cd "$DIR/.." && pwd)
ROOT=$(cd "$PB/../.." && pwd)
APP="${BKS_APP:-$PB/fixtures/single}"
PLATFORM="${BKS_PLATFORM:-linux-amd64}"
ARCH="${PLATFORM#linux-}"
CACHE="${BKS_CACHE_DIR:-$ROOT/.cache/bk-cache-run}"
WS="/tmp/cnb-managed-$MODE-$EXPORT-$$"
trap 'rm -rf "$WS"' EXIT
OUT="$ROOT/.cache/tests/offline-paketo-bk/$PLATFORM"
BPS="${BPS_BIN:-$ROOT/script/bin/bksmartpacker-linux-amd64}"
mkdir -p "$WS" "$OUT"

case "$MODE" in
  pind) IMAGE="offline-runtime-pind-bk:$ARCH"; INNER="podman" ;;
  dind) IMAGE="offline-runtime-dind-bk:$ARCH"; INNER="docker" ;;
  *) echo "unsupported BKS_TEST_MODE: $MODE" >&2; exit 2 ;;
esac
case "$EXPORT" in layout|daemon) ;; *) echo "unsupported BKS_TEST_EXPORT: $EXPORT" >&2; exit 2 ;; esac

if ! podman image exists "$IMAGE" 2>/dev/null; then
  echo "missing $IMAGE; run: just smart-runtime-dind (or runtime-pind-bundle)" >&2
  exit 1
fi

echo "mode=$MODE export=$EXPORT app=$APP platform=$PLATFORM"
extra_env=()
[ "$MODE" = dind ] && extra_env+=(BKS_INNER_CONTAINER_CLI="${BKS_INNER_CONTAINER_CLI:-/tmp/dockerbin/docker}")
env -u BKS_DOCKER_SOCK -u BKS_EXTRA_MOUNTS \
  RUNTIME="$MODE" BKS_RUNTIME_DRIVER=managed CNB_RUNTIME_DRIVER=managed \
  CNB_RUNTIME_IMAGE="$IMAGE" CNB_RUNTIME_WORKSPACE="$WS" CNB_CACHE_DIR="$CACHE" \
  BKS_CONTAINER_CLI=podman "${extra_env[@]}" \
  BKS_BUILDER_USER=1000:1000 BKS_BUILDER_USERNS=keep-id \
  BKS_PLATFORM="$PLATFORM" BKS_EXPORT_MODE="$EXPORT" BKS_NETWORK=host \
  "$BPS" --src "$APP" --builder-image "offline-builder-build-bk:$ARCH" \
  --dest "$OUT/s-mart-$MODE-$EXPORT-$ARCH.tar.gz" --timeout 30m
echo "artifact: $OUT/s-mart-$MODE-$EXPORT-$ARCH.tar.gz"

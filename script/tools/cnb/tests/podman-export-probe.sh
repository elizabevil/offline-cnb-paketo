#!/usr/bin/env bash
# 验证 podman 兼容 API 的镜像导出能力：
#   1) 按 tag 导出
#   2) 按完整镜像 ID 导出
#   3) 两个导出并发（模拟 lifecycle 的单连接复用场景）
#
# 用法：sudo script/tools/cnb/tests/podman-export-probe.sh
# 日志：.cache/logs/podman-export-probe.log
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
LOG_DIR="$ROOT/.cache/logs"
LOG="$LOG_DIR/podman-export-probe.log"
mkdir -p "$LOG_DIR"
exec > >(tee "$LOG") 2>&1

SOCK=/run/probe-podman.sock
IMG=probe/img:local
TMP=$(mktemp -d)
cleanup() {
  podman rmi "$IMG" >/dev/null 2>&1 || true
  [ -n "${SVC:-}" ] && kill "$SVC" 2>/dev/null || true
  rm -rf "$TMP" "$SOCK"
}
trap cleanup EXIT

echo "===== 0) 启动临时 podman service ====="
podman system service --time=0 "unix://$SOCK" >"$TMP/svc.log" 2>&1 &
SVC=$!
for _ in $(seq 1 20); do
  curl -s --unix-socket "$SOCK" http://d/v1.41/version >/dev/null && break
  sleep 0.5
done
curl -s --unix-socket "$SOCK" http://d/v1.41/version >/dev/null || {
  echo "service 启动失败"; cat "$TMP/svc.log"; exit 1; }
echo "service 就绪：$SOCK"

echo "===== 1) 造测试镜像 ====="
tar -cf "$TMP/empty.tar" --files-from /dev/null
podman import --change 'ENV HOME=/app' "$TMP/empty.tar" "$IMG" >/dev/null
FULL=$(podman inspect --format '{{.Id}}' "$IMG")
HEX=${FULL#sha256:}
echo "ID=$FULL"

probe() {
  local label=$1 url=$2
  local out="$TMP/$label.tar"
  curl -s --unix-socket "$SOCK" "$url" -o "$out"
  local size
  size=$(stat -c%s "$out" 2>/dev/null || echo 0)
  echo "--- $label size=$size"
  if [ "$size" -lt 1000 ]; then
    head -c 300 "$out"; echo
    grep -ac "singleton connection already in use" "$out" 2>/dev/null || true
  else
    head -c 16 "$out" | od -c | head -2
  fi
}

echo "===== 2) 单独导出 ====="
probe by-tag "http://d/v1.41/images/get?names=$IMG"
probe by-id "http://d/v1.41/images/get?names=sha256:$HEX"

echo "===== 3) 并发导出（复现单连接冲突）====="
curl -s --unix-socket "$SOCK" "http://d/v1.41/images/get?names=sha256:$HEX" -o "$TMP/c1.tar" &
P1=$!
curl -s --unix-socket "$SOCK" "http://d/v1.41/images/get?names=sha256:$HEX" -o "$TMP/c2.tar" &
P2=$!
wait $P1; R1=$?
wait $P2; R2=$?
echo "并发结果：p1=$R1 size=$(stat -c%s "$TMP/c1.tar" 2>/dev/null || echo 0) p2=$R2 size=$(stat -c%s "$TMP/c2.tar" 2>/dev/null || echo 0)"
for f in "$TMP/c1.tar" "$TMP/c2.tar"; do
  if [ -s "$f" ] && [ "$(stat -c%s "$f")" -lt 1000 ]; then head -c 300 "$f"; echo; fi
done

echo "===== 4) 版本 ====="
podman version | head -3
echo "日志：$LOG"

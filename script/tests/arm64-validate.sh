#!/usr/bin/env bash
# arm64 全链路验证：工具 → 物料 → 构建 → 打包 → 产物断言
#
# 用法：script/tests/arm64-validate.sh /path/to/release-arm64
# 日志：.cache/logs/arm64-validate.log
set -uo pipefail

ROOT=$(cd "$(dirname "$0")/../.." && pwd)
APP=${1:-}
LOG_DIR="$ROOT/.cache/logs"
LOG="$LOG_DIR/arm64-validate.log"
mkdir -p "$LOG_DIR"
exec > >(tee "$LOG") 2>&1

fail() { echo "✗ $*"; exit 1; }
ok()   { echo "✓ $*"; }

echo "===== 0) 参数与环境 ====="
[ -n "$APP" ] || fail "缺少应用目录参数：script/tests/arm64-validate.sh /path/to/release-arm64"
[ -d "$APP" ] || fail "应用目录不存在：$APP"
echo "repo=$ROOT"
echo "app=$APP"
uname -m
podman --version | head -1

echo "===== 1) qemu arm64 可用性 ====="
if ! podman run --rm --platform linux/arm64 debian:trixie-slim uname -m 2>/dev/null | grep -q aarch64; then
  fail "qemu 未就绪：podman 无法运行 arm64（先执行 podman run --privileged --rm tonistiigi/binfmt --install arm64）"
fi
ok "qemu 可运行 arm64"

echo "===== 2) arm64 工具 ====="
just --justfile "$ROOT/script/tools/cnb/justfile" build TARGET=linux-arm64 >/dev/null || fail "cnb arm64 构建失败"
just --justfile "$ROOT/script/tools/bksmartpacker_v3/justfile" build TARGET=linux-arm64 >/dev/null || fail "bksmartpacker arm64 构建失败"
ok "工具已构建：script/bin/cnb-linux-arm64、bksmartpacker-linux-arm64"

echo "===== 3) arm64 物料 ====="
"$ROOT/script/bin/cnb-linux-amd64" materials verify --platform linux-arm64 || fail "arm64 物料校验失败"
ok "物料校验通过"

echo "===== 4) 构建 arm64 镜像 ====="
cd "$ROOT/project/offline-paketo-bk" || fail "缺少 offline-paketo-bk 项目"
CNB_BIN="$ROOT/script/bin/cnb-linux-amd64" just build --platform linux-arm64 >/dev/null || fail "镜像构建失败"
ok "base/run/builder 镜像已构建"

echo "===== 5) 打包 arm64 应用 ====="
rm -f dist/s-mart.tar.gz
CNB_BIN="$ROOT/script/bin/cnb-linux-amd64" \
BPS_BIN="$ROOT/script/bin/bksmartpacker-linux-arm64" \
just smart "$APP" >/dev/null || fail "打包失败"
[ -f dist/s-mart.tar.gz ] || fail "未生成 dist/s-mart.tar.gz"
ok "产物：$ROOT/project/offline-paketo-bk/dist/s-mart.tar.gz"

echo "===== 6) 断言 ====="
W=$(mktemp -d)
trap 'rm -rf "$W"' EXIT
tar -xzf dist/s-mart.tar.gz -C "$W" web.tgz || fail "s-mart 缺少 web.tgz"
mkdir -p "$W/oci"
tar -xf "$W/web.tgz" -C "$W/oci" || fail "web.tgz 解包失败"

python3 - "$W/oci" <<'PY' || exit 1
import glob, json, sys
archs = set()
for f in glob.glob(sys.argv[1] + "/blobs/sha256/*"):
    try:
        d = json.load(open(f))
    except Exception:
        continue
    if "architecture" in d:
        archs.add(d["architecture"])
print("镜像 config architecture:", sorted(archs))
if archs != {"arm64"}:
    raise SystemExit("断言失败：期望 arm64，实际 %s" % sorted(archs))
PY
ok "镜像架构为 arm64"

echo
echo "===== 结果 ====="
ok "arm64 全链路通过"
echo "日志：$LOG"

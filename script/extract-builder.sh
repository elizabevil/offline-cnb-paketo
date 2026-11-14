#!/usr/bin/env bash
# extract-builder.sh <builder-image> [dest]
# 从官方 builder 镜像中提取 buildpacks / order.toml / stack.toml 到本地目录，
# 之后构建直接 COPY 本地文件，不再依赖该镜像。
set -euo pipefail
cd "$(dirname "$0")/.."

die() { echo "  ✗ $*" >&2; exit 1; }

image="${1:?用法: script/extract-builder.sh <builder-image> [dest]}"
dest="${2:-bundle/buildpacks}"
tool=""
for t in podman docker; do command -v "$t" >/dev/null 2>&1 && { tool="$t"; break; }; done
[ -n "$tool" ] || die "未找到 podman/docker"

echo "━━━ 提取 builder 内容 ━━━"
echo "  镜像: ${image}"
echo "  目标: ${dest}"

cid="$("${tool}" create "${image}" 2>/dev/null)" || die "无法创建容器（镜像是否已 pull？）"
trap '"${tool}" rm -f "${cid}" >/dev/null 2>&1 || true' EXIT

rm -rf "${dest}"
mkdir -p "${dest}"

for path in /cnb/buildpacks /cnb/order.toml /cnb/stack.toml /cnb/lifecycle; do
  [ -e "$("${tool}" mount "${cid}" 2>/dev/null || true)" ] || true
  if "${tool}" cp "${cid}:${path}" "${dest}/" >/dev/null 2>&1; then
    echo "  ✓ ${path}"
  else
    echo "  · 跳过 ${path}（镜像中不存在）"
  fi
done

# 保存官方 metadata 标签：官方 builder 的 buildpacks 列表/版本即事实源
"${tool}" inspect -f '{{index .Config.Labels "io.buildpacks.builder.metadata"}}' "${image}" \
  > "${dest}/builder-metadata.json" 2>/dev/null || true

# 记录来源，便于追溯
"${tool}" inspect -f '{{.RepoDigests}}' "${image}" 2>/dev/null | head -1 > "${dest}/SOURCE.txt" || true
echo "来源: ${image}" >> "${dest}/SOURCE.txt"

echo
echo "  buildpacks:"; ls "${dest}/buildpacks" 2>/dev/null | sed 's/^/    /' | head -20
echo "  体积: $(du -sh "${dest}" | cut -f1)"

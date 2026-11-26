#!/bin/sh
set -eu

socket="${CNB_RUNTIME_SOCKET:-/cnb/work/run/podman.sock}"
cache_root="${CNB_RUNTIME_CACHE:-/layers/cache}"

# runtime storage 必须与 /cnb/work 编排工作区隔离：
# - 有持久 cache/volume 时放在 cache，跨构建复用；
# - 没有 cache/volume 时放临时目录，随 sidecar 生命周期结束，不保证复用；
# - 绝不放在 workspace，避免 Workspace.Close 删除运行时状态。
if [ -d "$cache_root" ]; then
  root="${CNB_RUNTIME_STORAGE:-$cache_root/podman/storage}"
  runroot="${CNB_RUNTIME_RUNROOT:-$cache_root/podman/runroot}"
else
  root="${CNB_RUNTIME_STORAGE:-/tmp/cnb-runtime/podman/storage}"
  runroot="${CNB_RUNTIME_RUNROOT:-/tmp/cnb-runtime/podman/runroot}"
fi

mkdir -p "$root" "$runroot" "$(dirname "$socket")"

exec podman \
  --root "$root" \
  --runroot "$runroot" \
  --storage-driver=vfs \
  system service --time=0 "unix://${socket}"

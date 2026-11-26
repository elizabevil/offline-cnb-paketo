#!/bin/sh
set -eu

root="${CNB_RUNTIME_STORAGE:-/cnb/work/runtime/storage}"
runroot="${CNB_RUNTIME_RUNROOT:-/cnb/work/runtime/runroot}"
socket="${CNB_RUNTIME_SOCKET:-/cnb/work/run/podman.sock}"

mkdir -p "$root" "$runroot" "$(dirname "$socket")"

exec podman \
  --root "$root" \
  --runroot "$runroot" \
  --storage-driver=vfs \
  system service --time=0 "unix://${socket}"

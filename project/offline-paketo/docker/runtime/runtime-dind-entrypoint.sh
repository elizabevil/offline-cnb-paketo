#!/bin/sh
set -eu

socket="${CNB_RUNTIME_SOCKET:-/cnb/work/run/docker.sock}"
cache_root="${CNB_RUNTIME_CACHE:-/layers/cache}"

# data-root 必须跨构建保留并与 /cnb/work 隔离。
if [ -d "$cache_root" ]; then
  data_root="${CNB_RUNTIME_STORAGE:-$cache_root/docker/data}"
else
  data_root="${CNB_RUNTIME_STORAGE:-/tmp/cnb-runtime/docker/data}"
fi
# runroot 只放 pid/socket 等临时状态，必须随构建清理，不能留在持久卷里。
run_root="${CNB_RUNTIME_RUNROOT:-/cnb/work/runtime/docker}"
rm -f "$run_root/docker.pid"

mkdir -p "$data_root" "$run_root" "$(dirname "$socket")"

# 不启用 TLS；由构建器通过 unix socket 访问。
export DOCKER_TLS_CERTDIR=""

exec dockerd \
  --host "unix://${socket}" \
  --data-root "$data_root" \
  --exec-root "$run_root" \
  --pidfile "$run_root/docker.pid" \
  --storage-driver=vfs \
  --iptables="${CNB_DIND_IPTABLES:-false}" \
  --ip6tables="${CNB_DIND_IPTABLES:-false}" \
  --ip-forward="${CNB_DIND_IP_FORWARD:-false}" \
  --bridge="${CNB_DIND_BRIDGE:-none}" \
  --userland-proxy=false

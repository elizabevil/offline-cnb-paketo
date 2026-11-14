#!/usr/bin/env bash
# lifecycle.sh <app_dir> <case> — 直接用 lifecycle 四阶段构建（不经过 pack）
set -euo pipefail
source "$(dirname "$0")/../lib/common.sh"

app_dir="$1"; case_name="${2:-$(basename "${app_dir}")}"
# 项目参数化：默认 offline-paketo，可用 E2E_PROFILE 指向其它配置
PROFILE="${E2E_PROFILE:-${REPO_ROOT}/config/projects/offline-paketo.toml}"
[ -f "${PROFILE}" ] || { echo "找不到 profile: ${PROFILE}" >&2; exit 1; }
CASE="e2e/lifecycle/${case_name}"
image="offline-test-${case_name}:latest"
tool="$(container_tool)"
[ "${tool}" != "none" ] || { bad "未找到 podman/docker"; exit 1; }

# 各阶段实际接受的 flag（与 test/e2e/README.md 一致）
phase_flags() {
  case "$1" in
    detector) echo "-layers -app -platform" ;;
    builder)  echo "-layers -app -platform" ;;
    analyzer) echo "-layers -daemon -run-image" ;;
    exporter) echo "-layers -app -daemon" ;;
  esac
}

# 启动前用 --help 校验 flag 是否被支持，避免跑到一半才发现参数不认
validate_flags() {
  local phase help flags flag
  for phase in detector builder analyzer exporter; do
    help="$("${tool}" run --rm --entrypoint "/cnb/lifecycle/${phase}" \
      -e CNB_PLATFORM_API="${CNB_PLATFORM_API:-0.15}" \
      "${BUILDER_IMAGE}" --help 2>&1 || true)"
    flags="$(phase_flags "${phase}")"
    for flag in ${flags}; do
      if ! grep -q -- "  ${flag}" <<<"${help}"; then
        bad "lifecycle ${phase} 不支持 ${flag}（检查 test/e2e/README.md）"
        return 1
      fi
    done
  done
  ok "四个阶段的 flag 均被支持"
}

say "lifecycle: ${case_name}  (${app_dir})"
validate_flags || { done_; exit 1; }

if [ "${E2E_SKIP_BUILD:-0}" != "1" ]; then
  say "rebuild builder (offline-paketo)"
  run_step "rebuild builder" "rebuild-${case_name}" \
    "${CNB}" run "${PROFILE}" --no-commands || { done_; exit 1; }
fi

# ── 挂载约定（与 lifecycle 的语义对齐）────────────────────────
#   /workspace      应用源码（只读）
#   /layers         本次构建的层目录 —— 每次必须全新，否则 lifecycle 会因
#                   残留的 launch/layers 元数据报 "rename ... file exists"
#   /layers/cache   跨构建缓存 —— 唯一需要持久化的目录（Node/Python 等）
#   /platform       平台元数据（env 等），每次全新
#   /cnb/runtimes   本地 Node/Python 运行时（只读，可选，用于完全离线）
# 从配置推导镜像名（不再写死 offline-builder / offline-run）
plan_shell="$("${CNB}" plan "${PROFILE}" --format shell 2>/dev/null || true)"
pick_tag() { printf '%s\n' "${plan_shell}" | awk -v n="$1" '$1=="image_build" && $2=="\047"n"\047" {print $4}' | tr -d "\047"; }
BUILDER_IMAGE="$(pick_tag builder)"
RUN_IMAGE="$(pick_tag run)"

BUILDER_IMAGE="${BUILDER_IMAGE:-offline-builder:latest}"
RUN_IMAGE="${RUN_IMAGE:-offline-run:latest}"

# 断言镜像内的 buildpack 是当前源码：否则改了脚本却不生效，白跑
if [ "${E2E_SKIP_BUILD:-0}" != "1" ]; then
  fresh="$("${tool}" run --rm --entrypoint sh "${BUILDER_IMAGE}" -c \
    'ls /cnb/buildpacks/*/*/bin/build >/dev/null 2>&1 && echo 1 || echo 0' 2>/dev/null)"
  if [ "${fresh:-0}" = "0" ]; then
    bad "builder 镜像里没有任何 buildpack（需重新构建）"
    printf '    镜像创建时间: %s\n' "$("${tool}" inspect -f '{{.Created}}' "${BUILDER_IMAGE}" 2>/dev/null)" >&2
    printf '    确认 cnb run 真正重建了 builder，或在 e2e 里删除该镜像后重跑\n' >&2
  else
    ok "builder 镜像包含最新 buildpack"
  fi
fi

work="${TEST_ROOT}/.work/${case_name}"
layers_dir="${work}/layers"
cache_dir="${work}/cache"
platform_dir="${work}/platform"

# E2E_ONLY=builder,exporter 只跑指定阶段（复用上次的 layers，用于快速验证修复）
PHASES="${E2E_ONLY:-detector,builder,analyzer,exporter}"
if [ "${E2E_FRESH:-0}" = "1" ]; then
  rm -rf "${work}/cache"
fi
if [ -n "${E2E_ONLY:-}" ]; then
  echo "  resume: 只跑 ${PHASES}（保留 layers）"
  # 续跑时清理上次遗留的 launch/launch.ignore，否则 lifecycle 改名冲突：
  #   rename .../launch -> .../launch.ignore: file exists
  find "${layers_dir}" -maxdepth 2 -name 'launch*' -exec rm -rf {} + 2>/dev/null || true
else
  rm -rf "${layers_dir}"
fi
rm -rf "${platform_dir}"
mkdir -p "${layers_dir}" "${cache_dir}" "${platform_dir}/env"

# 工作区：仅当 buildpack 需要写源码目录时才拷贝副本（如 npm 会写
# package-lock.json）；其余情况直接只读挂载，避免无意义的复制与污染。
needs_writable=0
for marker in package.json pyproject.toml setup.py Pipfile; do
  [ -f "${app_dir}/${marker}" ] && needs_writable=1 && break
done
[ "${E2E_WRITABLE:-0}" = "1" ] && needs_writable=1

if [ "${needs_writable}" = "1" ]; then
  app_copy="${work}/workspace"
  rm -rf "${app_copy}"; mkdir -p "${app_copy}"
  cp -a "${app_dir}/." "${app_copy}/"
  workspace_mount="${app_copy}"
else
  rm -rf "${work}/workspace"
  workspace_mount="${app_dir}"
fi

# 需要预编译二进制的夹具（如 procfile）：缺失时提示先跑 just fixtures
if [ -f "${workspace_mount}/Procfile" ] && grep -q './app' "${workspace_mount}/Procfile" 2>/dev/null \
   && [ ! -x "${workspace_mount}/app" ]; then
  bad "缺少预编译二进制 app（先执行 just fixtures）"
  done_; exit 1
fi

if [ "${needs_writable}" = "1" ]; then
  ws_suffix=""
else
  ws_suffix=":ro"
fi

mounts=(
  -v "${workspace_mount}:/workspace${ws_suffix}"
  -v "${layers_dir}:/layers"
  -v "${cache_dir}:/layers/cache"
  -v "${platform_dir}:/platform"
)

# 完全离线：宿主提供 node/python 运行时目录，只读挂进容器
runtimes="${CNB_RUNTIMES:-${REPO_ROOT}/bundle/runtimes}"
if [ -d "${runtimes}" ]; then
  mounts+=(-v "${runtimes}:/cnb/runtimes:ro")
  printf '  runtimes=%s (mounted :ro)\n' "${runtimes}"
fi

userns=()
sock="$(pack_docker_host)"
if [ -n "${sock}" ]; then
  mounts+=(-v "${sock#unix://}:/var/run/docker.sock")
  [ "${tool}" = "podman" ] && userns=(--userns=keep-id)
fi

phase() {
  local name="$1"; shift
  run_step "lifecycle ${name}" "lc-${case_name}-${name}" \
    "${tool}" run --rm "${userns[@]}" "${mounts[@]}" \
    -e DOCKER_HOST=unix:///var/run/docker.sock \
    -e CNB_PLATFORM_API="${CNB_PLATFORM_API:-0.15}" \
    --entrypoint "/cnb/lifecycle/${name}" "${BUILDER_IMAGE}" "$@"
}

wants() { case ",${PHASES}," in *",$1,"*) return 0;; *) return 1;; esac; }

wants detector && { phase detector -layers=/layers -app=/workspace -platform=/platform || { done_; exit 1; }; }
wants builder  && { phase builder  -layers=/layers -app=/workspace -platform=/platform || { done_; exit 1; }; }
# 注意：Go flag 解析遇到第一个位置参数即停止，镜像必须放在所有 flag 之后
wants analyzer && { phase analyzer -layers=/layers -daemon -run-image "${RUN_IMAGE}" "${image}" || { done_; exit 1; }; }
if [ "${E2E_DUMP:-0}" = "1" ] && [ -f "${layers_dir}/launch.toml" ]; then
  echo "  ── launch.toml ──"; cat "${layers_dir}/launch.toml"
fi

wants exporter && { phase exporter -layers=/layers -app=/workspace -daemon "${image}" || { done_; exit 1; }; }

# 产出镜像后，先看镜像内部：层是否存在、PATH 是什么
if wants exporter; then
  echo "  ── 镜像内部检查 ──"
  "${tool}" run --rm --entrypoint sh "${image}" -c \
    'echo "PATH=$PATH"; echo "--- /layers ---"; ls /layers 2>/dev/null; echo "--- bin ---"; ls -l /layers/*/*/bin 2>/dev/null | head -8' \
    2>&1 | sed 's/^/     /' || true
fi

# 只有产出了镜像才验证运行
if wants exporter; then
  e2e_run "${image}" "18080" || { done_; exit 1; }
fi
done_

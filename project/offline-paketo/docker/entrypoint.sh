#!/bin/sh
# entrypoint.sh — CNB 统一构建入口
# 在单次容器运行内完成 detect → build → analyze → export 全流程
#
# 环境变量（均可选，有默认值）:
#   LAYERS_DIR     层目录（默认 /layers）
#   APP_DIR        应用源码目录（默认 /workspace）
#   PLATFORM_DIR   平台目录（默认 /platform）
#   RUN_IMAGE      运行镜像 tag（默认 offline-run:latest）
#   OUTPUT_IMAGE   出口镜像名（如 nexus.repo/...:latest）
#   DAEMON         是否输出 daemon 镜像（默认 true）
#   SKIP_DETECT    跳过 detect（默认 false）
#   SKIP_BUILD     跳过 build（默认 false）
#   SKIP_ANALYZE   跳过 analyze（默认 false）
#   SKIP_EXPORT    跳过 export（默认 false）

set -euo pipefail

LAYERS_DIR="${LAYERS_DIR:-/layers}"
APP_DIR="${APP_DIR:-/workspace}"
PLATFORM_DIR="${PLATFORM_DIR:-/platform}"
RUN_IMAGE="${RUN_IMAGE:-offline-run:latest}"
OUTPUT_IMAGE="${OUTPUT_IMAGE:-}"
DAEMON="${DAEMON:-true}"
SKIP_DETECT="${SKIP_DETECT:-false}"
SKIP_BUILD="${SKIP_BUILD:-false}"
SKIP_ANALYZE="${SKIP_ANALYZE:-false}"
SKIP_EXPORT="${SKIP_EXPORT:-false}"

# ── Lifecycle 运行需求（非 root 用户 + Platform API 声明） ──
export CNB_PLATFORM_API="${CNB_PLATFORM_API:-0.15}"

# ── 清理构建残留（保留 cache 绑定挂载） ────────────────
clean_layers() {
  echo "  🧹  清理构建残留..."
  # 删除构建层输出，但保留 paketo-buildpacks_go/cache/（独立绑定挂载）
  rm -rf "${LAYERS_DIR}/launch" "${LAYERS_DIR}/launch.ignore" \
         "${LAYERS_DIR}/go" "${LAYERS_DIR}/node" "${LAYERS_DIR}/python" \
         "${LAYERS_DIR}/config" "${LAYERS_DIR}/group.toml" \
         "${LAYERS_DIR}/plan.toml" "${LAYERS_DIR}/description.toml" \
         "${LAYERS_DIR}/analyzed.toml" 2>/dev/null || true
  # 删除 buildpack 层输出，但跳过 cache/ 挂载点
  if [ -d "${LAYERS_DIR}/paketo-buildpacks_go" ]; then
    for item in "${LAYERS_DIR}/paketo-buildpacks_go"/*; do
      [ -d "$item" ] && mountpoint -q "$item" && continue
      rm -rf "$item" 2>/dev/null || true
    done
  fi
  mkdir -p "${LAYERS_DIR}" "${PLATFORM_DIR}/env"
}

run_detect() {
  echo "━━━  [1/5] Detect ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "  🔍  layers=${LAYERS_DIR}  app=${APP_DIR}  platform=${PLATFORM_DIR}"
  /cnb/lifecycle/detector \
    -layers="${LAYERS_DIR}" -app="${APP_DIR}" -platform="${PLATFORM_DIR}"
  echo "  ✅  Detect 通过"
}

run_build() {
  echo "━━━  [2/5] Build  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "  🛠️   layers=${LAYERS_DIR}  app=${APP_DIR}  platform=${PLATFORM_DIR}"
  /cnb/lifecycle/builder \
    -layers="${LAYERS_DIR}" -app="${APP_DIR}" -platform="${PLATFORM_DIR}"
  echo "  ✅  Build 通过"
}

run_analyze() {
  if [ -z "${RUN_IMAGE}" ]; then
    echo "  ⚠️   RUN_IMAGE 未设置，跳过 analyze"
    return
  fi
  echo "━━━  [3/5] Analyze ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "  📋  layers=${LAYERS_DIR}  run-image=${RUN_IMAGE}  output=${OUTPUT_IMAGE:-（跳过分析）}"
  if [ -n "${OUTPUT_IMAGE}" ]; then
    /cnb/lifecycle/analyzer \
      -layers="${LAYERS_DIR}" \
      -run-image="${RUN_IMAGE}" \
      -uid 1000 -gid 1000 \
      -daemon \
      "${OUTPUT_IMAGE}"
    echo "  ✅  Analyze 通过"
  else
    echo "  ⏭️  OUTPUT_IMAGE 未设置，analyze 跳过"
  fi
}

run_export() {
  if [ -z "${OUTPUT_IMAGE}" ]; then
    echo "  ⚠️   OUTPUT_IMAGE 未设置，跳过 export"
    return
  fi
  echo "━━━  [4/5] Export ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "  📦  layers=${LAYERS_DIR}  output=${OUTPUT_IMAGE}  daemon=${DAEMON}"
  if [ "${DAEMON}" = "true" ]; then
    /cnb/lifecycle/exporter \
      -daemon \
      -layers="${LAYERS_DIR}" \
      -uid 1000 -gid 1000 \
      -log-level debug \
      "${OUTPUT_IMAGE}"
  else
    /cnb/lifecycle/exporter \
      -layers="${LAYERS_DIR}" \
      -uid 1000 -gid 1000 \
      "${OUTPUT_IMAGE}"
  fi
  echo "  ✅  Export 通过"
}

# ── 主流程 ────────────────────────────────────────────
main() {
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "  🏗️   CNB Pipeline"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "  App:        ${APP_DIR}"
  echo "  Layers:     ${LAYERS_DIR}"
  echo "  Platform:   ${PLATFORM_DIR}"
  echo "  Run image:  ${RUN_IMAGE}"
  echo "  Output:     ${OUTPUT_IMAGE:-（未设置，跳过 export）}"
  echo "  Daemon:     ${DAEMON}"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

  clean_layers

  if [ "${SKIP_DETECT}" != "true" ]; then
    run_detect
  else
    echo "  ⏭️  Detect 跳过"
  fi

  if [ "${SKIP_BUILD}" != "true" ]; then
    run_build
  else
    echo "  ⏭️  Build 跳过"
  fi

  if [ "${SKIP_ANALYZE}" != "true" ]; then
    run_analyze
  else
    echo "  ⏭️  Analyze 跳过"
  fi

  if [ "${SKIP_EXPORT}" != "true" ]; then
    run_export
  else
    echo "  ⏭️  Export 跳过"
  fi

  echo ""
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "  ✅  Pipeline 全部通过"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
}

main "$@"

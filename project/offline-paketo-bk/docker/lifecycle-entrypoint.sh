#!/bin/sh
# entrypoint.sh — CNB 统一构建入口（BK 对齐版）
# 在单次容器运行内完成 detect → build → analyze → export 全流程
#
# 环境变量（均可选，有默认值）:
#   LAYERS_DIR     层目录（/layers）
#   APP_DIR        应用源码目录（默认 /app）
#   PLATFORM_DIR   平台目录（默认 /platform）
#   RUN_IMAGE      运行镜像 tag（默认 offline-run-bk:latest，
#                   BK 模式由 bksmartpacker 设为 fake-cnb-runner:latest）
#   OUTPUT_IMAGE   出口镜像名（bksmartpacker 传入）
#   SKIP_DETECT    跳过 detect（默认 false）
#   SKIP_BUILD     跳过 build（默认 false）
#   SKIP_ANALYZE   跳过 analyze（默认 false）
#   SKIP_EXPORT    跳过 export（默认 false）
#   SOURCE_GET_URL 源码压缩包路径（bksmartpacker 传入，file:// 协议）

set -euo pipefail

LAYERS_DIR="/layers"
APP_DIR="${APP_DIR:-/app}"
PLATFORM_DIR="${PLATFORM_DIR:-/platform}"
# bksmartpacker 传入 CNB_RUN_IMAGE，fallback 到 RUN_IMAGE，再 fallback 到 offline-run-bk:latest
RUN_IMAGE="${CNB_RUN_IMAGE:-${RUN_IMAGE:-offline-run-bk:latest}}"
OUTPUT_IMAGE="${OUTPUT_IMAGE:-}"
SKIP_DETECT="${SKIP_DETECT:-false}"
SKIP_BUILD="${SKIP_BUILD:-false}"
SKIP_ANALYZE="${SKIP_ANALYZE:-false}"
SKIP_EXPORT="${SKIP_EXPORT:-false}"

# ── SOURCE_GET_URL: 解压源码到 APP_DIR ─────────────────────
if [ -n "${SOURCE_GET_URL:-}" ]; then
  echo "  📦  检测到 SOURCE_GET_URL: ${SOURCE_GET_URL}"
  case "${SOURCE_GET_URL}" in
    file://*)
      _src_file="${SOURCE_GET_URL#file://}"
      if [ -f "${_src_file}" ]; then
        mkdir -p "${APP_DIR}"
        echo "  📂  解压源码到 ${APP_DIR} ..."
        tar -xzf "${_src_file}" -C "${APP_DIR}"
        echo "  ✅  源码已就绪: $(ls -1 "${APP_DIR}" | wc -l) 个条目"
      else
        echo "  ⚠️  源文件不存在: ${_src_file}"
      fi
      _src_file=
      ;;
    http://*|https://*)
      echo "  ⚠️  HTTP SOURCE_GET_URL 暂不支持"
      ;;
  esac
fi

# ── REQUIRED_BUILDPACKS → 生成 order.toml ─────────────────
# bksmartpacker 传 "type name ... version;..."，name 为 bk-buildpack-*；
# 本项目组件为 paketo-buildpacks_*，此处做映射。
gen_order() {
  [ -n "${REQUIRED_BUILDPACKS:-}" ] || return 0
  local order="${LAYERS_DIR}/order.toml" entry name dir
  : > "${order}"
  local IFS=';'
  for entry in ${REQUIRED_BUILDPACKS}; do
    [ -n "${entry}" ] || continue
    name="$(echo "${entry}" | awk '{print $2}')"
    case "${name}" in
      bk-buildpack-python) name=paketo-buildpacks_python ;;
      bk-buildpack-nodejs) name=paketo-buildpacks_nodejs ;;
      bk-buildpack-go)     name=paketo-buildpacks_go ;;
      bk-buildpack-apt)    continue ;;
    esac
    dir="$(ls -d /cnb/buildpacks/"${name}"/* 2>/dev/null | head -1)"
    [ -n "${dir}" ] || { echo "  ✗ 未找到 buildpack: ${name}" >&2; exit 1; }
    cat >> "${order}" <<TOML
[[order]]
  [[order.group]]
    id = "$(echo "${name}" | sed 's|_|/|; s|^|paketo-buildpacks/|')"
    version = "$(basename "${dir}")"
TOML
  done
  unset IFS
  echo "  📋  order 由 REQUIRED_BUILDPACKS 生成"
}

# ── Lifecycle 运行需求（非 root 用户 + Platform API 声明） ──
export CNB_PLATFORM_API="${CNB_PLATFORM_API:-0.15}"

# ── 清理构建残留（保留 cache 绑定挂载） ────────────────
clean_layers() {
  echo "  🧹  清理构建残留..."
  if [ -n "${OUTPUT_IMAGE}" ] && command -v docker >/dev/null 2>&1; then
    docker rmi "${OUTPUT_IMAGE}" 2>/dev/null || true
  fi

  rm -rf "${LAYERS_DIR}/launch" "${LAYERS_DIR}/launch.ignore" \
         "${LAYERS_DIR}/go" "${LAYERS_DIR}/node" "${LAYERS_DIR}/python" \
         "${LAYERS_DIR}/config" "${LAYERS_DIR}/group.toml" \
         "${LAYERS_DIR}/plan.toml" "${LAYERS_DIR}/description.toml" \
         "${LAYERS_DIR}/analyzed.toml" 2>/dev/null || true
  # 删除所有 buildpack 层的非 cache 输出
  for _bp_d in "${LAYERS_DIR}"/paketo-buildpacks_*/; do
    [ -d "$_bp_d" ] || continue
    for item in "$_bp_d"/*; do
      [ -d "$item" ] && mountpoint -q "$item" && continue
      rm -rf "$item" 2>/dev/null || true
    done
  done
  mkdir -p "${LAYERS_DIR}" "${PLATFORM_DIR}/env"
}

run_detect() {
  echo "━━━  [1/4] Detect ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "  🔍  layers=${LAYERS_DIR}  app=${APP_DIR}  platform=${PLATFORM_DIR}"
  local order_flag=""
  [ -f "${LAYERS_DIR}/order.toml" ] && order_flag="-order=${LAYERS_DIR}/order.toml"
  su cnb -c "/cnb/lifecycle/detector \
    -layers=${LAYERS_DIR} -app=${APP_DIR} -platform=${PLATFORM_DIR} ${order_flag}"
  echo "  ✅  Detect 通过"
}

run_build() {
  echo "━━━  [2/4] Build  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "  🛠️   layers=${LAYERS_DIR}  app=${APP_DIR}  platform=${PLATFORM_DIR}"
  su cnb -c "/cnb/lifecycle/builder \
    -layers=${LAYERS_DIR} -app=${APP_DIR} -platform=${PLATFORM_DIR}"
  echo "  ✅  Build 通过"
}

run_analyze() {
  if [ -z "${RUN_IMAGE}" ]; then
    echo "  ⚠️   RUN_IMAGE 未设置，跳过 analyze"
    return
  fi
  echo "━━━  [3/4] Analyze ──────────────────────────"
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
  echo "━━━  [4/4] Export ──────────────────────────"
  echo "  📦  Exporting image: ${OUTPUT_IMAGE}  run-image=${RUN_IMAGE}"
  local cache_flag=""
  [ -n "${CACHE_IMAGE:-}" ] && cache_flag="-cache-image=${CACHE_IMAGE}"
  /cnb/lifecycle/exporter \
    -daemon \
    -layers="${LAYERS_DIR}" \
    -app="${APP_DIR}" \
    ${cache_flag} \
    "${OUTPUT_IMAGE}"
  echo "  ✅  Export 通过"
}

# ── 主流程 ────────────────────────────────────────────
main() {
  gen_order
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "  🏗️   CNB Pipeline (BK 对齐)"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "  App:        ${APP_DIR}"
  echo "  Layers:     ${LAYERS_DIR}"
  echo "  Platform:   ${PLATFORM_DIR}"
  echo "  Run image:  ${RUN_IMAGE}"
  echo "  Output:     ${OUTPUT_IMAGE:-（未设置，跳过 export）}"
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

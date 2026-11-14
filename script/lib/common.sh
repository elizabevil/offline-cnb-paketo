#!/usr/bin/env bash
# =============================================================================
# script/lib/common.sh — 基座公共函数库
# -----------------------------------------------------------------------------
# 只提供「纯能力」：日志、平台映射、代理解析、下载、校验、解压。
# 不包含任何业务编排（编排在 script/fetch.sh），保证可被其他脚本复用。
#
# 约定：
#   - 所有函数在失败时 `die`，调用方不需要重复判断返回值
#   - 所有写操作都是「原子」或「幂等」的：先写 .part / .tmp，再 mv
#   - 所有路径都加引号，兼容空格
# =============================================================================

# 防重复 source
if [ -n "${__CNB_BASE_COMMON_SH__:-}" ]; then
  return 0
fi
__CNB_BASE_COMMON_SH__=1

set -euo pipefail

# ── 路径锚点 ────────────────────────────────────────────────────────────────
_COMMON_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BASE_ROOT="$(cd "${_COMMON_DIR}/../.." && pwd)"
SCRIPT_DIR="$(cd "${_COMMON_DIR}/.." && pwd)"
LOCK_FILE="${BASE_ROOT}/assets.lock.toml"
CNB_BIN="${CNB_BIN:-${SCRIPT_DIR}/bin/cnb}"
CNB_SRC="${SCRIPT_DIR}/tools/cnb"
BUNDLE_DIR="${BUNDLE_DIR:-${BASE_ROOT}/bundle}"

# ── 颜色（仅在 tty 且未设置 NO_COLOR 时启用）────────────────────────────────
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
  C_RESET=$'\033[0m'
  C_DIM=$'\033[2m'
  C_RED=$'\033[31m'
  C_GREEN=$'\033[32m'
  C_YELLOW=$'\033[33m'
  C_BLUE=$'\033[34m'
  C_CYAN=$'\033[36m'
else
  C_RESET="" C_DIM="" C_RED="" C_GREEN="" C_YELLOW="" C_BLUE="" C_CYAN=""
fi

# ── 日志 ────────────────────────────────────────────────────────────────────
log_info() { printf '%s  ·  %s%s\n' "${C_BLUE}" "$*" "${C_RESET}"; }
log_ok()   { printf '%s  ✓  %s%s\n' "${C_GREEN}" "$*" "${C_RESET}"; }
log_warn() { printf '%s  !  %s%s\n' "${C_YELLOW}" "$*" "${C_RESET}" >&2; }
log_err()  { printf '%s  ✗  %s%s\n' "${C_RED}" "$*" "${C_RESET}" >&2; }
log_dim()  { printf '%s     %s%s\n' "${C_DIM}" "$*" "${C_RESET}"; }

log_step() {
  printf '\n%s━━━  %s  %s\n' "${C_CYAN}" "$*" "${C_RESET}"
}

die() {
  log_err "$*"
  exit 1
}

need_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "缺少依赖命令: $1"
}

# ── 平台 ────────────────────────────────────────────────────────────────────
VALID_PLATFORMS="linux-amd64 linux-arm64"

is_valid_platform() {
  case " ${VALID_PLATFORMS} " in
    *" $1 "*) return 0 ;;
    *) return 1 ;;
  esac
}

require_platform() {
  is_valid_platform "$1" || die "不支持的平台: '$1'（可用: ${VALID_PLATFORMS}）"
}

# linux-amd64 -> amd64（Go / OCI 约定）
platform_to_goarch() {
  case "$1" in
    linux-amd64) echo "amd64" ;;
    linux-arm64) echo "arm64" ;;
    *) die "未知平台: $1" ;;
  esac
}

# linux-amd64 -> x86_64 / aarch64（uname -m 约定）
platform_to_uname() {
  case "$1" in
    linux-amd64) echo "x86_64" ;;
    linux-arm64) echo "aarch64" ;;
    *) die "未知平台: $1" ;;
  esac
}

host_platform() {
  case "$(uname -m)" in
    x86_64) echo "linux-amd64" ;;
    aarch64|arm64) echo "linux-arm64" ;;
    *) echo "unknown" ;;
  esac
}

# ── 代理 ────────────────────────────────────────────────────────────────────
# 优先级: 显式参数 > PROXY_URL 环境变量 > 空（直连）
# 说明: 本函数只解析, 不导出; 导出由调用方在需要的命令上做前缀赋值。
resolve_proxy() {
  local explicit="${1:-}"
  if [ -n "${explicit}" ]; then
    printf '%s' "${explicit}"
    return 0
  fi
  printf '%s' "${PROXY_URL:-}"
}

# ── 哈希 ────────────────────────────────────────────────────────────────────
sha256_of() {
  local file="$1"
  [ -f "${file}" ] || die "文件不存在，无法计算哈希: ${file}"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "${file}" | awk '{print $1}'
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "${file}" | awk '{print $1}'
  else
    die "缺少 sha256 工具（sha256sum / shasum）"
  fi
}

# ── 目录 ────────────────────────────────────────────────────────────────────
# 布局：bundle/<platform>/{archives,binaries,sdk}
# 架构在前的理由：按平台整体分发 / 清理 / 挂载时是一个目录，不会漏。
# BUNDLE_LAYOUT:
#   arch-first  bundle/<platform>/{archives,binaries}   （基座默认）
#   kind-first  bundle/{archives,binaries}/<platform>   （历史项目，Dockerfile 已按此写死）
BUNDLE_LAYOUT="${BUNDLE_LAYOUT:-arch-first}"
archives_dir() {
  if [ "${BUNDLE_LAYOUT}" = "kind-first" ]; then echo "${BUNDLE_DIR}/archives/$1"; else echo "${BUNDLE_DIR}/$1/archives"; fi
}
binaries_dir() {
  if [ "${BUNDLE_LAYOUT}" = "kind-first" ]; then echo "${BUNDLE_DIR}/binaries/$1"; else echo "${BUNDLE_DIR}/$1/binaries"; fi
}
sdk_dir() {
  if [ "${BUNDLE_LAYOUT}" = "kind-first" ]; then echo "${BUNDLE_DIR}/sdk/$1"; else echo "${BUNDLE_DIR}/$1/sdk"; fi
}
meta_dir() { echo "$(binaries_dir "$1")/.meta"; }
marker_file()  { echo "$(meta_dir "$1")/${2//\//_}.sha256"; }

ensure_dir() {
  mkdir -p "$1"
}

# ── 下载 ────────────────────────────────────────────────────────────────────
# 原子下载：先写 <out>.part，成功后 mv；失败不留下半截文件。
# 用法: download_url <url> <out> <proxy>
download_url() {
  local url="$1" out="$2" proxy="${3:-}"
  # GitHub 直连受限的环境可用镜像前缀：GH_MIRROR=https://ghfast.top/
  if [ -n "${GH_MIRROR:-}" ]; then
    case "${url}" in
      https://github.com/*) url="${GH_MIRROR%/}/${url}" ;;
    esac
  fi
  local part="${out}.part"
  local -a args=(
    --fail --silent --show-error --location
    --retry 3 --retry-delay 2 --connect-timeout 15
    --output "${part}" "${url}"
  )
  if [ -n "${proxy}" ]; then
    args=(--proxy "${proxy}" "${args[@]}")
  fi

  ensure_dir "$(dirname "${out}")"
  rm -f "${part}"

  # 前置环境变量 + 显式 --proxy 双保险（部分 curl 版本 env 代理不生效）
  if ! http_proxy="${proxy}" https_proxy="${proxy}" \
       HTTP_PROXY="${proxy}" HTTPS_PROXY="${proxy}" \
       curl "${args[@]}"; then
    rm -f "${part}"
    die "下载失败: ${url}"
  fi

  mv "${part}" "${out}"
}

# ── 解压 ────────────────────────────────────────────────────────────────────
# 用法: extract_archive <kind> <archive> <dest> <strip>
extract_archive() {
  local kind="$1" archive="$2" dest="$3" strip="${4:-0}"
  ensure_dir "${dest}"
  case "${kind}" in
    tgz)
      need_cmd tar
      tar -xzf "${archive}" -C "${dest}" --strip-components="${strip}"
      ;;
    zip)
      need_cmd unzip
      unzip -qo "${archive}" -d "${dest}"
      ;;
    *)
      die "未知归档类型: ${kind}"
      ;;
  esac
}

# ── 展示 ────────────────────────────────────────────────────────────────────
human_size() {
  local bytes="$1"
  if [ "${bytes}" -ge 1073741824 ]; then
    awk -v b="${bytes}" 'BEGIN{printf "%.1fG", b/1073741824}'
  elif [ "${bytes}" -ge 1048576 ]; then
    awk -v b="${bytes}" 'BEGIN{printf "%.1fM", b/1048576}'
  elif [ "${bytes}" -ge 1024 ]; then
    awk -v b="${bytes}" 'BEGIN{printf "%.1fK", b/1024}'
  else
    printf '%sB' "${bytes}"
  fi
}

file_size() {
  [ -f "$1" ] || { echo 0; return; }
  if stat -c%s "$1" >/dev/null 2>&1; then
    stat -c%s "$1"
  else
    stat -f%z "$1"
  fi
}

# ── cnb 小工具 ──────────────────────────────────────────────────────────────
# 二进制优先；缺失时用本机 go 现编一次（依赖按 go.sum 从模块代理拉取）。
# 分发场景：在构建机执行 `just tools`，把 script/bin/cnb 一起发出去即可。
# Go 可能只在交互式 shell 的 PATH 中；显式探测，可用 GO=… 覆盖。
detect_go() {
  "${SCRIPT_DIR}/lib/go.sh"
}

ensure_cnb_tool() {
  if [ -x "${CNB_BIN}" ]; then
    return 0
  fi
  GO_BIN="$(detect_go || true)"
  [ -n "${GO_BIN}" ] || die \
"未找到 cnb 工具: ${CNB_BIN}
且本机没有 go，无法现场编译。请在有 go 的机器执行 'just tools' 后分发该二进制。"

  # 构建缓存放仓库内：CI / 沙箱里 HOME 常常只读，默认 GOCACHE 会直接失败
  local gocache="${GOCACHE:-${BASE_ROOT}/.cache/go-build}"
  log_info "首次使用：编译 cnb 工具（go.mod + go.sum 锁版本）"
  mkdir -p "$(dirname "${CNB_BIN}")" "${gocache}"
  (
    cd "${CNB_SRC}" && \
    GOCACHE="${gocache}" CGO_ENABLED=0 \
      go build -trimpath -ldflags "-s -w" -o "${CNB_BIN}" ./cmd/cnb
  ) || die "cnb 工具编译失败: ${CNB_SRC}"
  log_ok "已生成 ${CNB_BIN}"
}

# 统一入口：固定 lock 路径与物料目录，避免依赖各命令的 cwd
cnb_query() {
  "${CNB_BIN}" --file "${LOCK_FILE}" "$@"
}

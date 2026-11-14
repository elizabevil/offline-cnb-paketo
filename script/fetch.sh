#!/usr/bin/env bash
# =============================================================================
# script/fetch.sh — 离线物料的获取 / 校验 / 自检入口
# -----------------------------------------------------------------------------
# 设计要点（这三点是基座的"地基钢筋"，不要为了省事去掉）：
#   1. 事实源唯一：只读 assets.lock.toml，脚本里不出现任何版本号或 URL。
#   2. 三态幂等：归档是否存在 / 哈希是否匹配 / 解压标记是否匹配，三者独立判断。
#      只判断"归档存在"就跳过解压，是上一版最坑的缺陷（binaries 丢了永远补不回来）。
#   3. 失败原子化：下载写 .part 再 mv；解压失败不留半成品标记。
#
# 用法:
#   fetch.sh list                    [--platform P]
#   fetch.sh fetch   [id...]         [--platform P] [--proxy URL] [--force]
#   fetch.sh verify  [id...]         [--platform P] [--strict]
#   fetch.sh pin     [id...]         [--platform P]
#   fetch.sh doctor
#   fetch.sh import-legacy [--from DIR] [--platform P] [--dry-run]
# =============================================================================

# shellcheck source=lib/common.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/lib/common.sh"

PLATFORM="${PLATFORM:-$(host_platform)}"
PROXY="${PROXY:-$(resolve_proxy)}"
STRICT=0
FORCE=0
DRY_RUN=0
LEGACY_DIR="${BASE_ROOT}/offline-paketo"
IDS=()

usage() {
  sed -n '3,20p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

parse_args() {
  while [ $# -gt 0 ]; do
    case "$1" in
      --platform) PLATFORM="${2:?--platform 需要参数}"; shift 2 ;;
      --platform=*) PLATFORM="${1#*=}"; shift ;;
      --proxy) PROXY="${2:?--proxy 需要参数}"; shift 2 ;;
      --proxy=*) PROXY="${1#*=}"; shift ;;
      --from) LEGACY_DIR="${2:?--from 需要参数}"; shift 2 ;;
      --from=*) LEGACY_DIR="${1#*=}"; shift ;;
      --strict) STRICT=1; shift ;;
      --force) FORCE=1; shift ;;
      --dry-run) DRY_RUN=1; shift ;;
      -h|--help) usage; exit 0 ;;
      -*) die "未知选项: $1" ;;
      *) IDS+=("$1"); shift ;;
    esac
  done

  require_platform "${PLATFORM}"
  [ -f "${LOCK_FILE}" ] || die "未找到 lock 文件: ${LOCK_FILE}"
  ensure_cnb_tool
}

# 读取单个物料到 ASSET_* 变量
load_asset() {
  local out
  out="$(cnb_query materials get "$1" "${PLATFORM}")" || die "物料 '$1' 在平台 ${PLATFORM} 上不可用"
  # shellcheck disable=SC2086
  eval "${out}"
}

selected_ids() {
  if [ ${#IDS[@]} -eq 0 ]; then
    # 输出 id@version，保证同名多版本能被逐个处理
    cnb_query materials list | awk -F'\t' '{print $1"@"$2}' 
  else
    printf '%s\n' "${IDS[@]}"
  fi
}

archive_path() {
  # 归档按 <id>/<version> 分目录：部分物料的 URL 不含版本（如 yq_linux_amd64），
  # 共用文件名会导致升级后复用旧包、把旧哈希固定到新版本上。
  printf '%s/%s/%s/%s' "$(archives_dir "${PLATFORM}")" "${ASSET_ID}" "${ASSET_VERSION}" "$(basename "${ASSET_URL}")"
}

# 解压标记：记录"用哪个归档、在哪个平台、解压出了什么"
marker_matches() {
  local marker
  marker="$(marker_file "${PLATFORM}" "${ASSET_REF}")"
  [ -f "${marker}" ] || return 1
  local recorded
  recorded="$(cat "${marker}")"
  if [ -z "${ASSET_SHA256}" ]; then
    # 未固定哈希：标记为 unpinned 即视为已解压（产物存在性由 expect_ok 另外校验）
    [ "${recorded}" = "unpinned" ]
    return
  fi
  [ "${recorded}" = "${ASSET_SHA256}" ]
}

expect_ok() {
  local base; base="$(binaries_dir "${PLATFORM}")"
  local item
  for item in ${ASSET_EXPECT}; do
    [ -e "${base}/${item}" ] || return 1
  done
  return 0
}

is_extracted() {
  marker_matches && expect_ok
}

# 下载（含哈希校验）
ensure_archive() {
  local archive sha_actual
  archive="$(archive_path)"

  if [ -f "${archive}" ] && [ "${FORCE}" -eq 0 ]; then
    if [ -n "${ASSET_SHA256}" ]; then
      sha_actual="$(sha256_of "${archive}")"
      if [ "${sha_actual}" = "${ASSET_SHA256}" ]; then
        log_dim "归档已就绪: $(basename "${archive}")"
        return 0
      fi
      log_warn "归档哈希不符，重新下载: $(basename "${archive}")"
      rm -f "${archive}"
    else
      log_warn "归档已存在但未固定哈希，跳过下载: $(basename "${archive}")"
      return 0
    fi
  fi

  log_info "下载 ${ASSET_ID} ${ASSET_VERSION} (${PLATFORM})"
  log_dim "${ASSET_URL}"
  download_url "${ASSET_URL}" "${archive}" "${PROXY}"

  if [ -n "${ASSET_SHA256}" ]; then
    sha_actual="$(sha256_of "${archive}")"
    if [ "${sha_actual}" != "${ASSET_SHA256}" ]; then
      rm -f "${archive}"
      die "${ASSET_ID}: 哈希校验失败
  期望: ${ASSET_SHA256}
  实际: ${sha_actual}"
    fi
    log_ok "哈希校验通过 ($(human_size "$(file_size "${archive}")"))"
  else
    log_warn "${ASSET_ID}: 未固定 sha256，本次下载不可验证（请用 just pin 回填）"
  fi
}

# 解压（含标记）
ensure_extract() {
  local archive dest marker
  archive="$(archive_path)"
  dest="$(binaries_dir "${PLATFORM}")/${ASSET_EXTRACT_TO}"
  marker="$(marker_file "${PLATFORM}" "${ASSET_REF}")"

  if is_extracted; then
    log_dim "已解压: ${ASSET_ID}"
    return 0
  fi

  if [ -e "${dest}" ] && [ ! -d "${dest}" ]; then
    die "目标已存在且不是目录（可能是旧布局残留）: ${dest}
     请先删除该文件后重试：rm -f '${dest}'"
  fi
  log_info "解压 ${ASSET_REF} → $(binaries_dir "${PLATFORM}")/${ASSET_EXTRACT_TO}"
  rm -f "${marker}"
  if [ "${ASSET_KIND}" = "bin" ]; then
    # 裸二进制：直接落盘并赋可执行位
    ensure_dir "${dest}"
    cp "${archive}" "${dest}/${ASSET_ID}"
    chmod +x "${dest}/${ASSET_ID}"
  else
    extract_archive "${ASSET_KIND}" "${archive}" "${dest}" "${ASSET_STRIP}"
  fi

  if ! expect_ok; then
    die "${ASSET_ID}: 解压后缺少预期产物（期望: ${ASSET_EXPECT}）"
  fi

  ensure_dir "$(dirname "${marker}")"
  printf '%s\n' "${ASSET_SHA256:-unpinned}" > "${marker}"
  log_ok "解压完成: ${ASSET_ID}"
}

# ── 子命令 ──────────────────────────────────────────────────────────────────
cmd_list() {
  log_step "物料清单 (${PLATFORM})"
  printf '  %-12s %-10s %-6s %-16s %-10s\n' "ID" "VERSION" "KIND" "归档" "解压"
  printf '  %s\n' "──────────────────────────────────────────────────────────────"

  local id version kind platforms
  while IFS=$'\t' read -r id version kind platforms; do
    case ",${platforms}," in
      *",${PLATFORM},"*)
        load_asset "${id}"
        local archive state_archive state_extract
        archive="$(archive_path)"
        if [ -f "${archive}" ]; then
          if [ -n "${ASSET_SHA256}" ] && [ "$(sha256_of "${archive}")" = "${ASSET_SHA256}" ]; then
            state_archive="${C_GREEN}✓${C_RESET}"
          else
            state_archive="${C_YELLOW}?${C_RESET}"
          fi
        else
          state_archive="${C_RED}✗${C_RESET}"
        fi
        if is_extracted; then state_extract="${C_GREEN}✓${C_RESET}"; else state_extract="${C_DIM}–${C_RESET}"; fi
        printf '  %-12s %-10s %-6s %b%-16s%b %b\n' \
          "${id}" "${version}" "${kind}" "${state_archive}" "" "${state_extract}"
        ;;
      *)
        log_dim "跳过 ${id}（不支持 ${PLATFORM}）"
        ;;
    esac
  done < <(cnb_query materials list)
}

cmd_fetch() {
  # 下载 / 校验 / 解包已收编到 cnb materials sync（见 docs/ai/materials-sync.md）
  log_step "拉取物料 (${PLATFORM}${PROXY:+, proxy=${PROXY}})"
  local -a args=(materials sync --platform "${PLATFORM}" --bundle "${BUNDLE_DIR}")
  if [ ${#IDS[@]} -gt 0 ]; then
    args+=("${IDS[@]}")
  fi
  cnb_query "${args[@]}"
  log_step "完成"
}

cmd_verify() {
  local -a args=(materials verify --platform "${PLATFORM}" --bundle "${BUNDLE_DIR}")
  [ "${STRICT}" -eq 1 ] && args+=(--strict)
  if [ ${#IDS[@]} -gt 0 ]; then
    args+=("${IDS[@]}")
  fi
  cnb_query "${args[@]}"
}

cmd_pin() {
  local -a args=(materials pin --platform "${PLATFORM}" --bundle "${BUNDLE_DIR}")
  if [ ${#IDS[@]} -gt 0 ]; then
    args+=("${IDS[@]}")
  fi
  cnb_query "${args[@]}"
}

cmd_doctor() {
  log_step "环境自检"
  local missing=0

  for cmd in curl tar unzip just; do
    if command -v "${cmd}" >/dev/null 2>&1; then
      # unzip 的 --version 会先打一行 caution 噪音，单独处理
      # 注意：`| head -n1` 在 pipefail 下会因 SIGPIPE 返回 141，必须兜底
      local ver
      if [ "${cmd}" = "unzip" ]; then
        ver="$(unzip -v 2>&1 | head -n1 || true)"
      else
        ver="$(${cmd} --version 2>&1 | head -n1 || true)"
      fi
      log_ok "${cmd} → ${ver}"
    else
      log_err "${cmd} 未安装"
      missing=$((missing + 1))
    fi
  done

  if command -v sha256sum >/dev/null 2>&1; then
    log_ok "sha256sum 可用"
  elif command -v shasum >/dev/null 2>&1; then
    log_ok "shasum 可用"
  else
    log_err "缺少 sha256 工具"
    missing=$((missing + 1))
  fi

  if command -v go >/dev/null 2>&1; then
    log_ok "go → $(go version 2>&1)"
  else
    log_warn "未安装 go（仅现场编译 cnb 工具时需要；已有二进制可忽略）"
  fi

  ensure_cnb_tool
  log_ok "cnb 工具 → $(cnb_query version 2>/dev/null || echo 可用)"

  log_dim "宿主机平台: $(host_platform) / 目标平台: ${PLATFORM}"

  if [ -n "${PROXY}" ]; then
    local code
    code="$(http_proxy="${PROXY}" https_proxy="${PROXY}" \
            curl -s -o /dev/null -m 8 -w '%{http_code}' --proxy "${PROXY}" https://github.com || true)"
    if [ "${code}" = "200" ] || [ "${code}" = "301" ] || [ "${code}" = "302" ]; then
      log_ok "代理可用 (${PROXY} → HTTP ${code})"
    else
      log_warn "代理探测异常 (${PROXY} → HTTP ${code:-timeout})"
    fi
  else
    log_dim "未配置代理（PROXY_URL 为空，直连）"
  fi

  if [ -d "${BUNDLE_DIR}" ]; then
    log_ok "bundle 目录: ${BUNDLE_DIR} ($(du -sh "${BUNDLE_DIR}" 2>/dev/null | awk '{print $1}'))"
  else
    log_warn "bundle 目录尚不存在: ${BUNDLE_DIR}"
  fi

  [ "${missing}" -eq 0 ] && log_step "自检通过" || die "自检失败: ${missing} 项缺失"
}

cmd_import_legacy() {
  # 新布局: bundle/<platform>/archives
  # 旧布局: bundle/archives/<platform>（三个历史子项目仍是这样，做兼容）
  local src="${LEGACY_DIR}/bundle/${PLATFORM}/archives"
  if [ ! -d "${src}" ]; then
    src="${LEGACY_DIR}/bundle/archives/${PLATFORM}"
  fi
  log_step "导入历史归档 (${src})"
  [ -d "${src}" ] || die "源目录不存在: ${src}"

  ensure_dir "$(archives_dir "${PLATFORM}")"
  local f base
  for f in "${src}"/*; do
    [ -f "${f}" ] || continue
    base="$(basename "${f}")"
    if [ -f "$(archives_dir "${PLATFORM}")/${base}" ]; then
      log_dim "已存在，跳过: ${base}"
      continue
    fi
    if [ "${DRY_RUN}" -eq 1 ]; then
      log_info "[dry-run] 复制 ${base}"
    else
      cp "${f}" "$(archives_dir "${PLATFORM}")/${base}"
      log_ok "复制 ${base}"
    fi
  done
}

main() {
  local cmd="${1:-}"; shift || true
  [ -n "${cmd}" ] || { usage; exit 1; }

  case "${cmd}" in
    list|fetch|verify|pin|doctor|import-legacy) ;;
    *) die "未知子命令: ${cmd}" ;;
  esac

  parse_args "$@"

  case "${cmd}" in
    list) cmd_list ;;
    fetch) cmd_fetch ;;
    verify) cmd_verify ;;
    pin) cmd_pin ;;
    doctor) cmd_doctor ;;
    import-legacy) cmd_import_legacy ;;
  esac
}

main "$@"

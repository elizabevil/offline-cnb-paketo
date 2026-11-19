#!/usr/bin/env bash
# e2e/run.sh <app_dir> <case_name> — 打包 + 启动 + HTTP 断言
set -euo pipefail
source "$(dirname "$0")/../lib/common.sh"
app_dir="$1"; case_name="${2:-$(basename "${app_dir}")}"
CASE="e2e/${case_name}"
image="offline-test-${case_name}:latest"
say "e2e: ${case_name}  (${app_dir})"

# 保证 builder 是最新构建（E2E_SKIP_BUILD=1 可跳过）
if [ "${E2E_SKIP_BUILD:-0}" != "1" ] && [ "${E2E_BUILT:-0}" != "1" ]; then
  profile="${E2E_PROFILE:-${REPO_ROOT}/project/offline-paketo/pipeline.toml}"
  say "rebuild builder ($(basename "${profile}"))"
  if ! run_step "rebuild builder" "rebuild-${case_name}" \
      "${CNB}" pipeline run "${profile}"; then
    done_; exit 1
  fi
  export E2E_BUILT=1
fi
e2e_pack "${app_dir}" "${image}" || { done_; exit 1; }
e2e_run "${image}" "18080" || { done_; exit 1; }
done_

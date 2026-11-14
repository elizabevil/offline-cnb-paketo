#!/usr/bin/env bash
# auto.sh <app_dir> <case> — 按运行时自动选择 pack 或 lifecycle 直连
set -euo pipefail
source "$(dirname "$0")/../lib/common.sh"

app_dir="$1"; case_name="${2:-$(basename "${app_dir}")}"
strategy="$(e2e_strategy)"
say "e2e: ${case_name}  strategy=${strategy}"
case "${strategy}" in
  pack)      exec "$(dirname "$0")/run.sh" "${app_dir}" "${case_name}" ;;
  lifecycle) exec "$(dirname "$0")/lifecycle.sh" "${app_dir}" "${case_name}" ;;
  *)         bad "未找到可用容器运行时"; exit 1 ;;
esac

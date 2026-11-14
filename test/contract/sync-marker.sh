#!/usr/bin/env bash
# 契约：materials sync 的 marker 字段与 --check 行为
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CNB="${ROOT}/script/bin/cnb"

"${CNB}" materials sync jq yq >/dev/null

for ref in jq@1.8.2 yq@4.54.3; do
  marker="${ROOT}/bundle/linux-amd64/binaries/.meta/${ref}.sync.json"
  [ -f "${marker}" ] || { echo "缺少 marker: ${marker}" >&2; exit 1; }
  for key in id version platform sha256 extractor synced_at; do
    grep -q "\"${key}\"" "${marker}" || { echo "${ref}: 缺少字段 ${key}" >&2; exit 1; }
  done
  grep -qE '"synced_at": "[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9:]{8}Z"' "${marker}" \
    || { echo "${ref}: synced_at 不是 UTC RFC3339(Z)" >&2; exit 1; }
done

# --check 不得下载：输出应全部为 skipped
out="$("${CNB}" materials sync --check jq yq 2>&1)"
grep -q 'action=skipped' <<<"${out}" || { echo "--check 未跳过" >&2; exit 1; }
echo "  ✓ sync marker 契约通过"

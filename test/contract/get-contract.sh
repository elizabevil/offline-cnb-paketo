#!/usr/bin/env bash
# 契约：cnb materials get 的输出字段是 fetch.sh 的输入，不能变
source "$(dirname "$0")/../lib/common.sh"
CASE="contract/get"
out="$("${CNB}" --file "${REPO_ROOT}/assets.lock.toml" get fnm linux-amd64)"
for k in ASSET_REF ASSET_ID ASSET_VERSION ASSET_KIND ASSET_URL ASSET_SHA256 ASSET_EXTRACT_TO ASSET_STRIP ASSET_EXPECT; do
  assert_contains "${out}" "${k}=" "包含 ${k}"
done
assert_eq "$(printf '%s\n' "${out}" | wc -l | tr -d ' ')" "9" "字段数为 9"
done_

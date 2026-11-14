#!/usr/bin/env bash
# 契约：三种格式都能渲染且非空（防输出格式回归）
source "$(dirname "$0")/../lib/common.sh"
CASE="contract/plan"
cfg="${REPO_ROOT}/config/projects/offline-paketo.toml"
for f in text shell json yaml; do
  out="$("${CNB}" plan "${cfg}" --format "${f}" 2>&1)"
  [ -n "${out}" ] && ok "format=${f} 非空" || bad "format=${f} 为空"
done
assert_contains "$("${CNB}" plan "${cfg}" --format shell)" "image_build" "shell 含 image_build"
assert_contains "$("${CNB}" plan "${cfg}" --format json)" '"stages"' "json 含 stages"
done_

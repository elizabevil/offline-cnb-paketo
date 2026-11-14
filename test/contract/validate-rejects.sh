#!/usr/bin/env bash
# 契约：坏配置必须被拒绝（证明校验有效，而不是"写了没用"）
source "$(dirname "$0")/../lib/common.sh"
CASE="contract/validate"
tmp="$(mktemp -d)"; trap 'rm -rf "${tmp}"' EXIT

mk() { cat > "${tmp}/bad.toml"; }
base='schema = 1
[project]
name = "bad"
dir  = "project/offline-paketo"
platform = "linux-amd64"
[docker]
context = "project/offline-paketo"
[assets.lifecycle]
version = "0.21.13"
'

mk <<<"${base}"'
[images.base]
dockerfile = "docker/Dockerfile.base"
tag = "t:1"
[[stage]]
image = "missing"
'
assert_fail "\"${CNB}\" profile validate '${tmp}/bad.toml'" "stage 引用不存在应失败"

mk <<<"${base}"'
[images.base]
dockerfile = "docker/Dockerfile.base"
tag = "t:1"
inject_versions = ["unknown"]
[[stage]]
image = "base"
'
assert_fail "\"${CNB}\" profile validate '${tmp}/bad.toml'" "inject_versions 未声明应失败"

mk <<<"${base}"
assert_fail "\"${CNB}\" profile validate '${tmp}/bad.toml'" "缺少 [[stage]] 应失败"
done_

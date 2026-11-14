#!/usr/bin/env bash
# 输出可用的 go：优先 $GO，其次 PATH 中的 go。
# 用 `go env GOROOT` 验证它确实是可用的工具链（避免指向坏二进制）。
set -uo pipefail

candidate="${GO:-go}"
if ! command -v "${candidate}" >/dev/null 2>&1; then
  echo "go: 未找到（可用 GO=/path/to/go 指定）" >&2
  exit 1
fi
if ! "${candidate}" env GOROOT >/dev/null 2>&1; then
  echo "go: '${candidate} env GOROOT' 执行失败，工具链不可用" >&2
  exit 1
fi
echo "${candidate}"

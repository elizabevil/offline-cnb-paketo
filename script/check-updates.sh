#!/usr/bin/env bash
# check-updates.sh — 对照上游最新版，检查 assets.lock.toml 是否落后
# 需要网络；只读取，不修改任何文件。
set -uo pipefail
cd "$(dirname "$0")/.."

declare -A REPO=(
  [lifecycle]=buildpacks/lifecycle
  [pack]=buildpacks/pack
  [fnm]=Schniz/fnm
  [uv]=astral-sh/uv
  [jq]=jqlang/jq
  [yq]=mikefarah/yq
)
# docker 走镜像站，版本号需人工确认，单独列出
declare -A SKIP=([docker]="镜像站发布，需手动确认")

printf '%-12s %-12s %-12s %s\n' "物料" "当前" "最新" "状态"
printf '%s\n' "------------------------------------------------------------"

stale=0
# 同一 id 多版本时只比较第一个（即 default）
while IFS=$'\t' read -r id ver _rest; do
  [ -n "${id:-}" ] || continue
  cur="${ver#v}"
  if [ -n "${SKIP[$id]:-}" ]; then
    printf '%-12s %-12s %-12s %s\n' "$id" "$cur" "-" "${SKIP[$id]}"
    continue
  fi
  repo="${REPO[$id]:-}"
  [ -n "$repo" ] || continue
  latest="$(curl -sS --max-time 20 "https://api.github.com/repos/${repo}/releases/latest" \
    | python3 -c 'import sys,json;print(json.load(sys.stdin).get("tag_name",""))' 2>/dev/null)"
  latest="${latest#v}"; latest="${latest#jq-}"
  if [ -z "$latest" ]; then
    printf '%-12s %-12s %-12s %s\n' "$id" "$cur" "?" "查询失败"
    continue
  fi
  if [ "$cur" = "$latest" ]; then
    printf '%-12s %-12s %-12s %s\n' "$id" "$cur" "$latest" "ok"
  else
    printf '%-12s %-12s %-12s %s\n' "$id" "$cur" "$latest" "落后"
    stale=$((stale+1))
  fi
done < <(script/bin/cnb-linux-amd64 materials list 2>/dev/null | awk -F'\t' '!seen[$1]++ {print $1"\t"$2}')

printf '\n%-24s %-12s %-12s %s\n' "buildpack" "当前" "上游最新" "状态"
printf '%s\n' "------------------------------------------------------------"
seen=""
for bp in project/*/docker/buildpacks/*/buildpack.toml; do
  repo="$(grep -m1 'upstream = ' "$bp" 2>/dev/null | cut -d'"' -f2)"
  cur="$(grep -m1 'upstream-version = ' "$bp" 2>/dev/null | cut -d'"' -f2)"
  [ -n "$repo" ] && [ -n "$cur" ] || continue
  case "$seen" in *"$repo\${cur} "*) ;; esac
  case "$seen" in *" $repo "*) continue;; esac
  seen="$seen $repo "
  latest="$(curl -sS --max-time 20 "https://api.github.com/repos/${repo}/releases/latest" \
    | python3 -c 'import sys,json;print(json.load(sys.stdin).get("tag_name",""))' 2>/dev/null)"
  latest="${latest#v}"
  if [ -z "$latest" ]; then
    printf '%-24s %-12s %-12s %s\n' "$repo" "$cur" "?" "查询失败"; continue
  fi
  if [ "$cur" = "$latest" ]; then
    printf '%-24s %-12s %-12s %s\n' "$repo" "$cur" "$latest" "ok"
  else
    printf '%-24s %-12s %-12s %s\n' "$repo" "$cur" "$latest" "落后"
    stale=$((stale+1))
  fi
done

printf '%s\n' "------------------------------------------------------------"
if [ "$stale" -gt 0 ]; then
  echo "${stale} 项落后：物料改 assets.lock.toml，buildpack 改 buildpack.toml 的 version/upstream-version 并同步 Dockerfile 与 order.toml"
  exit 1
fi
echo "全部为最新"

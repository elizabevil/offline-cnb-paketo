#!/usr/bin/env bash
# test/lib/common.sh — 测试公共库：断言、路径、构建/运行封装
set -euo pipefail

TEST_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
REPO_ROOT="$(cd "${TEST_ROOT}/.." && pwd)"
CNB="${REPO_ROOT}/script/bin/cnb"
BUILDER_IMAGE="${BUILDER_IMAGE:-offline-builder:latest}"
RUN_IMAGE="${RUN_IMAGE:-offline-run:latest}"
PASS=0 FAIL=0 CASE=""

say()  { printf '\n\033[1m▶ %s\033[0m\n' "$*"; }
LOGDIR="${TEST_ROOT}/.logs"
mkdir -p "${LOGDIR}"
ok()   { PASS=$((PASS+1)); printf '  \033[32m✓\033[0m %s\n' "$*"; }
bad()  { FAIL=$((FAIL+1)); printf '  \033[31m✗\033[0m %s\n' "$*"; }
done_() {
  printf '\n%s: %d passed, %d failed\n' "${CASE:-test}" "${PASS}" "${FAIL}"
  [ "${FAIL}" -eq 0 ]
}
assert_eq()   { [ "$1" = "$2" ] && ok "$3" || bad "$3 (got: $1, want: $2)"; }
assert_ok()   { if eval "$1" >/dev/null 2>&1; then ok "$2"; else bad "$2"; fi; }
assert_fail() { if eval "$1" >/dev/null 2>&1; then bad "$2 (应当失败却成功)"; else ok "$2"; fi; }
assert_contains() { case "$1" in *"$2"*) ok "$3";; *) bad "$3 (缺少: $2)";; esac; }

# 选择容器运行时：CONTAINER_TOOL 优先，其次 podman，最后 docker
# 某个运行时里是否已有 builder 镜像（考虑 podman 会给本地镜像加 localhost/）
_image_exists() {
  local tool="$1" ref="$2"
  case "${tool}" in
    podman) podman image exists "${ref}" >/dev/null 2>&1 ;;
    docker) docker image inspect "${ref}" >/dev/null 2>&1 ;;
    *) return 1 ;;
  esac
}

container_tool() {
  if [ -n "${CONTAINER_TOOL:-}" ]; then echo "${CONTAINER_TOOL}"; return; fi
  # 谁有 builder 就用谁；都没有则优先 podman
  local t
  for t in podman docker; do
    command -v "${t}" >/dev/null 2>&1 || continue
    for ref in "${BUILDER_IMAGE}" "localhost/${BUILDER_IMAGE}"; do
      _image_exists "${t}" "${ref}" && { echo "${t}"; return; }
    done
  done
  if command -v podman >/dev/null 2>&1; then echo podman; return; fi
  if command -v docker >/dev/null 2>&1; then echo docker; return; fi
  echo none
}

# pack 会把短名强制规范成 docker.io/library/<name>；这里算出它期望的名字
canonical_image() {
  case "$1" in
    */*) echo "$1" ;;
    *) echo "docker.io/library/$1" ;;
  esac
}

# 在运行时里找到 builder 的实际引用（podman 可能存成 localhost/xxx）
builder_source_ref() {
  local tool="$1"
  for ref in "${BUILDER_IMAGE}" "localhost/${BUILDER_IMAGE}"; do
    _image_exists "${tool}" "${ref}" && { echo "${ref}"; return 0; }
  done
  return 1
}

# prepare_image_ref <image>：把本地镜像打上 pack 期望的规范化 tag，返回该引用。
# 必须每次重打：否则规范化名字可能仍指向旧镜像（rebuild 后不更新）。
prepare_image_ref() {
  local image="$1" tool src want
  tool="$(container_tool)"
  want="$(canonical_image "${image}")"
  for src in "${image}" "localhost/${image}"; do
    if _image_exists "${tool}" "${src}"; then
      "${tool}" tag "${src}" "${want}" >/dev/null 2>&1 || return 1
      echo "${want}"; return 0
    fi
  done
  return 1
}

prepare_builder_ref() { prepare_image_ref "${BUILDER_IMAGE}"; }

# pack 通过 Docker API 访问镜像；用 podman 时指向它的 socket
# pack 需要 Docker API；podman 场景下若没有 socket，临时拉起一个服务
pack_docker_host() {
  if [ -n "${DOCKER_HOST:-}" ]; then echo "${DOCKER_HOST}"; return; fi
  local tool; tool="$(container_tool)"
  if [ "${tool}" = "podman" ]; then
    local sock="${XDG_RUNTIME_DIR:-/run/user/$(id -u)}/podman/podman.sock"
    if [ -S "${sock}" ]; then echo "unix://${sock}"; return; fi
    # 没有系统 socket：启动一个临时 API 服务
    sock="${TEST_ROOT}/.logs/podman.sock"
    rm -f "${sock}"
    podman system service --time=0 "unix://${sock}" >"${LOGDIR}/podman-service.log" 2>&1 &
    PACK_PODMAN_PID=$!
    for _ in $(seq 1 50); do
      [ -S "${sock}" ] && break
      sleep 0.2
    done
    if [ -S "${sock}" ]; then
      echo "unix://${sock}"; return
    fi
    bad "无法启动 podman API 服务，见 ${LOGDIR}/podman-service.log"
  fi
  echo ""
}

# 退出时清理临时 podman 服务
_cleanup_podman() {
  if [ -n "${PACK_PODMAN_PID:-}" ]; then
    kill "${PACK_PODMAN_PID}" >/dev/null 2>&1 || true
  fi
}
trap _cleanup_podman EXIT

# run_step <名称> <日志名> <命令...>
#   默认：显示命令行，输出进日志，失败时打印末尾；成功只显示一行耗时
#   VERBOSE=1：实时流式输出（排查卡住时用）
#   STEP_TIMEOUT：单步超时（默认 300s），避免无限等待
run_step() {
  local name="$1" logname="$2"; shift 2
  local log="${LOGDIR}/${logname}.log"
  printf '  $ %s\n' "$*"
  local rc=0
  if [ "${VERBOSE:-0}" = "1" ]; then
    timeout --foreground "${STEP_TIMEOUT:-300}" "$@" 2>&1 | tee "${log}"; rc=${PIPESTATUS[0]}
  else
    timeout --foreground "${STEP_TIMEOUT:-300}" "$@" >"${log}" 2>&1; rc=$?
  fi
  if [ "${rc}" -ne 0 ]; then
    if [ "${rc}" -eq 124 ]; then
      bad "${name} 超时（${STEP_TIMEOUT:-300}s）"
    else
      bad "${name}（exit ${rc}）"
    fi
    printf '  ── 日志末尾 (%s) ──\n' "${log}" >&2
    tail -${E2E_TAIL:-60} "${log}" >&2 || true
    if grep -q 'permission denied.*docker.sock' "${log}" 2>/dev/null; then
      cat >&2 <<'HINT'

  ── 诊断提示 ──
  lifecycle 容器在 ANAlYZING 阶段连不上 Docker API，通常是 rootless podman
  的 uid 映射问题：宿主 socket 属主 uid 1000，但容器内 uid 1000 被映射到
  了从属 uid，导致无权限。

  可选方案：
    1) 改用 docker（若可用）: CONTAINER_TOOL=docker just test-e2e-all
    2) 以 rootful podman 运行: sudo podman system service --time=0 unix:///run/podman.sock
       然后 DOCKER_HOST=unix:///run/podman.sock just test-e2e-all
    3) 跳过 pack，直接用 cnb run + 手动 lifecycle 验证（当前环境不推荐）
HINT
    fi
    return "${rc}"
  fi
  return 0
}

# e2e_pack <app_dir> <image> — 用 builder 打包应用
e2e_pack() {
  local dir="$1" image="$2"
  command -v pack >/dev/null || { bad "缺少 pack"; return 1; }
  local tool; tool="$(container_tool)"
  [ "${tool}" != "none" ] || { bad "未找到 podman/docker"; return 1; }
  if ! builder_source_ref "${tool}" >/dev/null; then
    bad "builder 镜像不存在: ${BUILDER_IMAGE}"
    printf '    先执行: cnb run %s/config/projects/offline-paketo.toml\n' "${REPO_ROOT}" >&2
    return 1
  fi
  BUILDER_REF="$(prepare_builder_ref)" || { bad "无法准备 builder 引用"; return 1; }
  RUN_REF="$(prepare_image_ref "${RUN_IMAGE}")" || { bad "无法准备 run 镜像引用: ${RUN_IMAGE}"; return 1; }
  local dh; dh="$(pack_docker_host)"
  if [ -z "${dh}" ]; then
    bad "未确定 Docker API（pack 需要）"
    printf '    可显式指定: DOCKER_HOST=unix://$XDG_RUNTIME_DIR/podman/podman.sock just test-e2e-all\n' >&2
    return 1
  fi
  export DOCKER_HOST="${dh}"
  printf '  DOCKER_HOST=%s\n' "${DOCKER_HOST}"
  local start; start=$(date +%s)
  local -a args=(
    build "${image}"
    --builder "${BUILDER_REF}"
    --run-image "${RUN_REF}"
    --path "${dir}"
    --pull-policy never
    --trust-builder            # 自建 builder 不在 pack 的信任列表里
  )
  [ -n "${E2E_PLATFORM:-}" ] && args+=(--platform "${E2E_PLATFORM}")
  if run_step "pack build ${image}" "pack-${image//[^a-zA-Z0-9]/_}" pack "${args[@]}"; then
    ok "pack build ${image}  $(( $(date +%s) - start ))s"
  else
    return 1
  fi
}

# e2e_run <image> <port> — 启动并检测 HTTP
e2e_run() {
  local image="$1" port="$2" cid
  local tool; tool="$(container_tool)"
  [ "${tool}" != "none" ] || { bad "未找到 podman/docker"; return 1; }
  printf '  $ %s run -d -p %s:8080 %s\n' "${tool}" "${port}" "${image}"
  # 不用 --rm：容器提前退出时还要看日志和退出码
  cid="$("${tool}" run -d -p "${port}:8080" "${image}" 2>"${LOGDIR}/run.log")" || { bad "容器启动失败"; tail -10 "${LOGDIR}/run.log" >&2; return 1; }
  for _ in $(seq 1 30); do
    if curl -fsS "http://127.0.0.1:${port}/" >/dev/null 2>&1; then
      ok "HTTP 200 (${image})"; "${tool}" rm -f "${cid}" >/dev/null 2>&1 || true; return 0
    fi
    sleep 1
  done
  bad "30s 内无 HTTP 响应 (${image})"
  printf '  ── 镜像配置 ──\n' >&2
  "${tool}" inspect -f 'entrypoint={{json .Config.Entrypoint}} cmd={{json .Config.Cmd}}' "${image}" >&2 2>/dev/null || true
  printf '  ── 启动标签 ──\n' >&2
  "${tool}" inspect -f '{{index .Config.Labels "io.buildpacks.build.metadata"}}' "${image}" 2>/dev/null | head -c 600 >&2 || true
  printf '  ── 层内容 ──\n' >&2
  "${tool}" run --rm --entrypoint sh "${image}" -c 'ls -l /layers/*/*/bin 2>/dev/null | head -10' >&2 2>&1 || true
  printf '  ── 容器状态 ──\n' >&2
  "${tool}" ps -a --filter "id=${cid}" --format 'state={{.State}} exit={{.ExitCode}}' >&2 2>/dev/null || true
  printf '  ── 容器日志 ──\n' >&2
  "${tool}" logs "${cid}" >&2 2>&1 || true
  "${tool}" rm -f "${cid}" >/dev/null 2>&1 || true
  return 1
}


# e2e_strategy — 根据运行时决定用哪条构建路径：
#   pack       仅当 docker 可用（socket 天然可访问）
#   lifecycle  podman 或 docker 不可用时（直接调四阶段，绕开 pack）
e2e_strategy() {
  local tool; tool="$(container_tool)"
  case "${tool}" in
    docker)
      if docker info >/dev/null 2>&1; then echo pack; else echo lifecycle; fi ;;
    podman) echo lifecycle ;;
    *) echo none ;;
  esac
}

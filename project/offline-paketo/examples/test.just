# ─── test.just - Go 功能测试 ───────────────────────────────
# 验证 lifecycle 端到端：detect → build → analyze → export → verify
#
# 前置条件:
#   just docker-build                 # 构建 builder + run 镜像
#
# 用法:
#   just -f apps/test.just test-go    # 完整 pipeline
#   just -f apps/test.just test-clean # 清理测试产物

set dotenv-load

# ── 共享配置（全局变量统一） ──────────────────────────────
import '../scripts/config.just'

# ── 测试专用变量（不在 config.just 中的局部变量） ─────
[private]
_app_img := "nexus.repo/docker/offline-paketo-test-go-app:latest"
[private]
_go_app := project_root / "apps" / "go-app"
[private]
_workdir := "/home/debi/IdeaProjects/Learn-cnb-paketo/offline-paketo/run-cache"
[private]
_sock := if shell("test -S /run/user/1000/podman/podman.sock && echo yes || echo no") == "yes" { "/run/user/1000/podman/podman.sock" } else { "/run/user/1000/podman/podman.sock" }

[private]
_cache_dir := _workdir / "cache"
[private]
_env_flags := "-e CNB_PLATFORM_API=0.15 -e CNB_APP_DIR=/workspace"
[private]
_vol_flags := "-v " + _go_app + ":/workspace:ro -v " + _workdir + "/layers:/layers -v " + _workdir + "/platform:/platform"
[private]
_pipeline_env := "-e LAYERS_DIR=/layers -e APP_DIR=/workspace -e PLATFORM_DIR=/platform -e RUN_IMAGE=" + offline_run_tag + " -e OUTPUT_IMAGE=" + _app_img + " -e DAEMON=true -e DOCKER_HOST=unix:///run/user/1000/podman/podman.sock"

# ═══════════════════════════════════════════════════════════
# 辅助
# ═══════════════════════════════════════════════════════════
test-init:
    @# 清理构建残留，保留 cache/（Go SDK + module cache 独立挂载）
    @{{ container_tool }} unshare rm -rf \
      {{ _workdir }}/layers/launch \
      {{ _workdir }}/layers/launch.ignore \
      {{ _workdir }}/layers/go \
      {{ _workdir }}/layers/node \
      {{ _workdir }}/layers/python \
      {{ _workdir }}/layers/config \
      {{ _workdir }}/layers/group.toml \
      {{ _workdir }}/layers/plan.toml \
      {{ _workdir }}/layers/description.toml \
      {{ _workdir }}/layers/analyzed.toml \
      2>/dev/null || true
    @# 清理 buildpack 层输出，但保留 cache/（独立挂载到 run-cache/cache/）
    @{{ container_tool }} unshare rm -rf \
      {{ _workdir }}/layers/paketo-buildpacks_go/go \
      {{ _workdir }}/layers/paketo-buildpacks_go/go.toml \
      {{ _workdir }}/layers/paketo-buildpacks_go/launch.toml \
      {{ _workdir }}/layers/paketo-buildpacks_go/launch.ignore \
      2>/dev/null || true
    @{{ container_tool }} unshare rm -rf {{ _workdir }}/platform 2>/dev/null || true
    @mkdir -p {{ _workdir }}/layers/paketo-buildpacks_go {{ _workdir }}/platform/env {{ _cache_dir }}
    @{{ container_tool }} unshare chmod -R 777 {{ _workdir }}/layers {{ _workdir }}/platform {{ _cache_dir }} 2>/dev/null || true

# ═══════════════════════════════════════════════════════════
# Step 1: Detect — buildpack 匹配项目类型
# ═══════════════════════════════════════════════════════════
test-detect-go:
    @echo "━━━  [1/5] Detect ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    @{{ container_tool }} run --rm {{ _env_flags }} {{ _vol_flags }} \
      {{ offline_builder_tag }} \
      -layers=/layers -app=/workspace -platform=/platform
# ═══════════════════════════════════════════════════════════
# Step 2: Build — 编译 Go 二进制
# ═══════════════════════════════════════════════════════════
test-build-go:
    @echo "━━━  [2/5] Build  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    @{{ container_tool }} run --rm {{ _env_flags }} {{ _vol_flags }} \
      -e CGO_ENABLED=${CGO_ENABLED:-0} \
      -e GOARCH=${GOARCH:-amd64} \
      -e GOOS=${GOOS:-linux} \
      --entrypoint /cnb/lifecycle/builder \
      {{ offline_builder_tag }} \
      -layers=/layers -app=/workspace -platform=/platform

# ═══════════════════════════════════════════════════════════
# Step 3: Analyze — 分析运行镜像
# ═══════════════════════════════════════════════════════════
test-analyze-go:
    @echo "━━━  [3/5] Analyze ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    @{{ container_tool }} run --rm {{ _env_flags }} {{ _vol_flags }} \
      -v {{ _sock }}:{{ _sock }} \
      --entrypoint /cnb/lifecycle/analyzer \
      {{ offline_builder_tag }} \
      -layers=/layers -app=/workspace \
      -run-image {{ offline_run_tag }} \

# ═══════════════════════════════════════════════════════════
# Step 4: Export — 打包为 OCI 镜像
# ═══════════════════════════════════════════════════════════
test-export-go:
    @echo "━━━  [4/5] Export ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    @{{ container_tool }} run --rm {{ _env_flags }} \
      -v {{ _go_app }}:/workspace \
      -v {{ _workdir }}/layers:/layers \
      -v {{ _sock }}:{{ _sock }} \
      -e GO_DOWNLOAD_URL=http://192.168.11.4:5000/go1.26.4.linux-amd64.tar.gz \
      -e CGO_ENABLED=0 \
      -e GOARCH=amd64 \
      -e GOOS=linux \
      --entrypoint /cnb/lifecycle/exporter \
      {{ offline_builder_tag }} \
      -daemon \
      -layers=/layers \
      {{ _app_img }}

# ═══════════════════════════════════════════════════════════
# 完整 pipeline（单次容器 run，entrypoint.sh 内部编排）
# ═══════════════════════════════════════════════════════════
test-go:
    @echo "━━━  Go E2E Pipeline  ━━━━━━━━━━━━━━━━━━━━━━━━"
    @echo "  Builder:  {{ offline_builder_tag }}"
    @echo "  Run img:  {{ offline_run_tag }}"
    @echo "  App:      {{ _go_app }}"
    @echo "  Output:   {{ _app_img }}"
    @echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    @echo ""
    @just --justfile {{ justfile() }} test-init
    @echo ""
    @{{ container_tool }} run --rm \
      --userns=keep-id \
      {{ _env_flags }} {{ _pipeline_env }} \
      -e CGO_ENABLED=${CGO_ENABLED:-0} \
      -e GOARCH=${GOARCH:-amd64} \
      -e GOOS=${GOOS:-linux} \
      -v {{ _go_app }}:/workspace:ro \
      -v {{ _workdir }}/layers:/layers \
      -v {{ _workdir }}/platform:/platform \
      -v {{ _cache_dir }}:/layers/paketo-buildpacks_go/cache \
      -v {{ _sock }}:{{ _sock }} \
      {{ offline_builder_tag }}
    @echo ""
    @echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    @echo "  ✅  Go pipeline 全部通过"
    @echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# ═══════════════════════════════════════════════════════════
# 清理
# ═══════════════════════════════════════════════════════════
test-clean:
    @echo "  🧹  Cleaning up..."
    @{{ container_tool }} rmi {{ _app_img }} 2>/dev/null || true
    @rm -rf {{ _workdir }}
    @echo "  ✅  Cleaned"

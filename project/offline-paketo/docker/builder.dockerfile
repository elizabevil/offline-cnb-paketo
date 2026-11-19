# =============================================================================
# offline-paketo: 离线 CNB Builder 镜像
# 内含 lifecycle + 自建 Procfile/Node/Python buildpack
# -----------------------------------------------------------------------------
# 依赖: 需要先构建 base 镜像 (base.dockerfile) 或指定基础镜像
# -----------------------------------------------------------------------------
# 构建方式:
#   docker build -t offline-builder:latest -f docker/Dockerfile . \
#     --build-arg BASE_IMAGE_TAG=offline-builder-base:latest
#
#   一步构建（自动先构建 base）:
#   just docker-build
# -----------------------------------------------------------------------------
# 使用方式:
#   pack build my-app --builder offline-builder:latest
# =============================================================================

# ═════════════════════════════════════════════════════════════════════════
# 构建参数（默认值来自 scripts/config.just）
# ═════════════════════════════════════════════════════════════════════════
ARG BASE_IMAGE_TAG

# ── 基于 base 镜像构建最终 builder ──────────────────────────────────────
FROM ${BASE_IMAGE_TAG:?BASE_IMAGE_TAG is required} AS final

# pack 要求：构建容器使用的非 root 用户/组
ENV CNB_USER_ID=1000
ENV CNB_GROUP_ID=1000
ARG FNM_VERSION
ARG LIFECYCLE_VERSION
ARG UV_VERSION

ARG BUNDLE_ARCH=linux-amd64

# ── CNB Lifecycle ─────────────────────────────────────────────────────────
COPY --from=bundle /${BUNDLE_ARCH}/binaries/lifecycle/${LIFECYCLE_VERSION}/ /cnb/lifecycle/

# cnb CLI（本地构建物料）：作为 lifecycle 执行器
ARG CNB_VERSION
COPY --from=bundle /${BUNDLE_ARCH}/binaries/cnb/${CNB_VERSION}/cnb /cnb/tools/cnb
RUN chmod +x /cnb/tools/cnb

# ── 运行时工具（fnm → Node, uv/uvx → Python） ────────────
COPY --from=bundle /${BUNDLE_ARCH}/binaries/fnm/${FNM_VERSION}/fnm       /usr/local/bin/fnm
COPY --from=bundle /${BUNDLE_ARCH}/binaries/uv/${UV_VERSION}/uv        /usr/local/bin/uv
COPY --from=bundle /${BUNDLE_ARCH}/binaries/uv/${UV_VERSION}/uvx       /usr/local/bin/uvx

# Go 下载源（可在 [images.builder].args 覆盖）
ARG BP_GO_MIRROR=https://mirrors.aliyun.com/golang/
ENV BP_GO_MIRROR=${BP_GO_MIRROR}

# ── 自研离线 Buildpacks（用本地 Go SDK / fnm / uv，不联网） ───────────────
COPY --from=buildpacks /go-dist/ /cnb/buildpacks/paketo-buildpacks_go-dist/2.11.2/
COPY --from=buildpacks /go-build/ /cnb/buildpacks/paketo-buildpacks_go-build/2.4.48/
COPY --from=buildpacks /go/ /cnb/buildpacks/paketo-buildpacks_go/4.23.3/
RUN chmod +x /cnb/buildpacks/paketo-buildpacks_go-dist/2.11.2/bin/* /cnb/buildpacks/paketo-buildpacks_go-build/2.4.48/bin/*
COPY --from=buildpacks /node-engine/ /cnb/buildpacks/paketo-buildpacks_node-engine/8.12.0/
COPY --from=buildpacks /npm-install/ /cnb/buildpacks/paketo-buildpacks_npm-install/2.3.36/
COPY --from=buildpacks /npm-start/ /cnb/buildpacks/paketo-buildpacks_npm-start/2.5.13/
COPY --from=buildpacks /nodejs/ /cnb/buildpacks/paketo-buildpacks_nodejs/10.15.0/
COPY --from=buildpacks /cpython/ /cnb/buildpacks/paketo-buildpacks_cpython/1.18.51/
COPY --from=buildpacks /pip-install/ /cnb/buildpacks/paketo-buildpacks_pip-install/0.7.51/
COPY --from=buildpacks /python/ /cnb/buildpacks/paketo-buildpacks_python/2.68.0/

# procfile 来自物料（官方多架构 release 包）
ARG PROCFILE_VERSION
ARG BUILDPACK_ARCH
COPY --from=bundle /procfile/${PROCFILE_VERSION}/buildpack.toml /cnb/buildpacks/paketo-buildpacks_procfile/${PROCFILE_VERSION}/buildpack.toml
COPY --from=bundle /procfile/${PROCFILE_VERSION}/linux/${BUILDPACK_ARCH}/ /cnb/buildpacks/paketo-buildpacks_procfile/${PROCFILE_VERSION}/
RUN chmod +x /cnb/buildpacks/paketo-buildpacks_procfile/${PROCFILE_VERSION}/bin/*

# ── 全局 order（决定 detect 顺序） ───────────────────────────────────────
COPY docker/order.toml /cnb/order.toml


# ── 统一入口脚本（单次 run 完成 detect→build→analyze→export） ──────────
COPY docker/lifecycle-entrypoint.sh /cnb/lifecycle/entrypoint.sh
RUN chmod +x /cnb/lifecycle/entrypoint.sh

# ── Platform 目录（buildpack 配置入口） ──────────────────────────────────
RUN mkdir -p /platform/env

# ── 非 root 用户（lifecycle detect/build 拒绝以 root 运行） ────────────
RUN mkdir -p /layers /workspace && \
    useradd -u 1000 -m cnb && \
    chown -R cnb:cnb /cnb /platform /layers /workspace
USER cnb

# ── OCI Labels（pack 通过 labels 识别 builder） ──────────────────────────
LABEL io.buildpacks.stack.id="io.buildpacks.stacks.bionic"


LABEL io.buildpacks.builder.stack='{"id":"io.buildpacks.stacks.bionic","homepage":"https://github.com/paketo-community/bionic"}'

# ── 环境变量 ──────────────────────────────────────────────────────────────
ENV PATH="/cnb/lifecycle:${PATH}" \
    CNB_STACK_ID="io.buildpacks.stacks.bionic"

# ── 入口（统一 pipeline，通过环境变量控制各步骤） ─────────────────────
ENTRYPOINT ["/cnb/lifecycle/entrypoint.sh"]
CMD []

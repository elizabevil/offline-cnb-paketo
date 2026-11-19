# =============================================================================
# offline-paketo-bk: 最终 builder 镜像（BK 对齐，无 pack 依赖）
# 单镜像包含：lifecycle + 4 个 buildpack + 运行时工具 + CNB 元数据
# 
# 供 bksmartpacker 直接使用：
#   bksmartpacker --builder-image offline-builder-build-bk:latest
# =============================================================================

ARG BASE_IMAGE_TAG

FROM ${BASE_IMAGE_TAG}

# pack 要求：构建容器使用的非 root 用户/组
ENV CNB_USER_ID=1000
ENV CNB_GROUP_ID=1000
ARG FNM_VERSION
ARG JQ_VERSION
ARG LIFECYCLE_VERSION
ARG PACK_VERSION
ARG UV_VERSION
ARG YQ_VERSION

ARG BUNDLE_ARCH=linux-amd64
ARG APP_DIR=/app

# ═══════════════════════════════════════════════════════════
# CNB Lifecycle（detector, builder, analyzer, exporter, launcher）
# ═══════════════════════════════════════════════════════════
COPY --from=bundle /${BUNDLE_ARCH}/binaries/lifecycle/${LIFECYCLE_VERSION}/ /cnb/lifecycle/

# Go SDK 下载源（内网镜像；可用 --build-arg BP_GO_MIRROR 覆盖）
ARG BP_GO_MIRROR=https://mirrors.aliyun.com/golang/
ARG BP_GO_VERSION=""
ENV BP_GO_MIRROR=${BP_GO_MIRROR}
RUN echo "${BP_GO_MIRROR}" > /cnb/go-mirror \
    && if [ -n "${BP_GO_VERSION}" ]; then echo "${BP_GO_VERSION}" > /cnb/go-version; fi

# cnb CLI（本地构建物料）：作为 lifecycle 执行器
ARG CNB_VERSION
COPY --from=bundle /${BUNDLE_ARCH}/binaries/cnb/${CNB_VERSION}/cnb /cnb/tools/cnb
RUN chmod +x /cnb/tools/cnb

# ═══════════════════════════════════════════════════════════
# Buildpacks（完全离线，本地打包）
# ═══════════════════════════════════════════════════════════
COPY --from=buildpacks /go-dist/ /cnb/buildpacks/paketo-buildpacks_go-dist/2.11.2/
COPY --from=buildpacks /go-build/ /cnb/buildpacks/paketo-buildpacks_go-build/2.4.48/
COPY --from=buildpacks /go/ /cnb/buildpacks/paketo-buildpacks_go/4.23.3/
COPY --from=buildpacks /node-engine/ /cnb/buildpacks/paketo-buildpacks_node-engine/8.12.0/
COPY --from=buildpacks /npm-install/ /cnb/buildpacks/paketo-buildpacks_npm-install/2.3.36/
COPY --from=buildpacks /npm-start/ /cnb/buildpacks/paketo-buildpacks_npm-start/2.5.13/
COPY --from=buildpacks /nodejs/ /cnb/buildpacks/paketo-buildpacks_nodejs/10.15.0/
COPY --from=buildpacks /cpython/ /cnb/buildpacks/paketo-buildpacks_cpython/1.18.51/
COPY --from=buildpacks /pip-install/ /cnb/buildpacks/paketo-buildpacks_pip-install/0.7.51/
COPY --from=buildpacks /python/ /cnb/buildpacks/paketo-buildpacks_python/2.68.0/
RUN chmod +x /cnb/buildpacks/*/*/bin/* 2>/dev/null || true

# procfile 来自物料（官方多架构 release 包）
ARG PROCFILE_VERSION
ARG BUILDPACK_ARCH
COPY --from=bundle /${BUNDLE_ARCH}/binaries/procfile/${PROCFILE_VERSION}/buildpack.toml /cnb/buildpacks/paketo-buildpacks_procfile/${PROCFILE_VERSION}/buildpack.toml
COPY --from=bundle /${BUNDLE_ARCH}/binaries/procfile/${PROCFILE_VERSION}/linux/${BUILDPACK_ARCH}/ /cnb/buildpacks/paketo-buildpacks_procfile/${PROCFILE_VERSION}/
RUN chmod +x /cnb/buildpacks/paketo-buildpacks_procfile/${PROCFILE_VERSION}/bin/*

# ═══════════════════════════════════════════════════════════
# 全局 order（决定 detect 顺序）
# ═══════════════════════════════════════════════════════════
COPY docker/order.toml /cnb/order.toml

# ═══════════════════════════════════════════════════════════
# 运行时工具（fnm→Node, uv/uvx→Python）
# ═══════════════════════════════════════════════════════════
COPY --from=bundle /${BUNDLE_ARCH}/binaries/fnm/${FNM_VERSION}/fnm  /usr/local/bin/fnm
COPY --from=bundle /${BUNDLE_ARCH}/binaries/uv/${UV_VERSION}/uv   /usr/local/bin/uv
COPY --from=bundle /${BUNDLE_ARCH}/binaries/uv/${UV_VERSION}/uvx  /usr/local/bin/uvx

# ── 离线工具（jq, yq） ───────────────────────────────────────
COPY --from=bundle /${BUNDLE_ARCH}/binaries/jq/${JQ_VERSION}/jq   /usr/local/bin/jq
COPY --from=bundle /${BUNDLE_ARCH}/binaries/yq/${YQ_VERSION}/yq   /usr/local/bin/yq

# ── Docker CLI（bksmartpacker 镜像层推送用） ─────────────────

# ── pack CLI（调试用，bksmartpacker 场景不需要） ────────────
COPY --from=bundle /${BUNDLE_ARCH}/binaries/pack/${PACK_VERSION}/pack  /usr/local/bin/pack

# ═══════════════════════════════════════════════════════════
# CNB 基础设施
# ═══════════════════════════════════════════════════════════
COPY docker/lifecycle-entrypoint.sh /cnb/lifecycle/entrypoint.sh
RUN chmod +x /cnb/lifecycle/entrypoint.sh
RUN mkdir -p /platform/env

# ═══════════════════════════════════════════════════════════
# cnb 非 root 用户：detect/build 拒绝以 root 运行
# export 需要 root → 默认 USER root，entrypoint 中用 su -c 降权
# ═══════════════════════════════════════════════════════════
RUN mkdir -p /layers /layers/output ${APP_DIR} && \
    useradd -u 1000 -m cnb 2>/dev/null || true && \
    chown -R root:root /cnb && \
    chown -R cnb:cnb /platform /layers ${APP_DIR} 2>/dev/null || true

# ═══════════════════════════════════════════════════════════
# OCI Labels（bksmartpacker / CNB 工具识别用）
# ═══════════════════════════════════════════════════════════
LABEL io.buildpacks.stack.id="io.buildpacks.stacks.bionic"

# ═══════════════════════════════════════════════════════════
# 环境变量 + 入口
# ═══════════════════════════════════════════════════════════
ENV PATH="/cnb/lifecycle:${PATH}" \
    CNB_STACK_ID="io.buildpacks.stacks.bionic"

ENTRYPOINT ["/cnb/lifecycle/entrypoint.sh"]
CMD []

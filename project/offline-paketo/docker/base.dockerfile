# =============================================================================
# offline-paketo: 基础 OS 镜像层
# 仅安装系统依赖，不包含 lifecycle / buildpack
# -----------------------------------------------------------------------------
# 构建方式:
#   docker build -t offline-builder-base:latest -f docker/base.dockerfile \
#     --build-arg BASE_IMAGE=... .
#
#   通过 just（自动使用 docker/apt-sources/ 中的宿主镜像源）:
#   just docker-build-base
# -----------------------------------------------------------------------------
# 默认值来自 scripts/config.just 中的 base_image
# =============================================================================

ARG BASE_IMAGE

FROM ${BASE_IMAGE}

# ── 替换 APT 镜像源（国内加速） ─────────────────────────────────
# docker/apt-sources/sources.list 在构建上下文中，
# 构建时直接替换容器默认源，使 apt-get 走国内镜像源加速下载。
# 更新方式: just sync-apt-sources
COPY docker/apt-sources/sources.list /etc/apt/sources.list
RUN rm /etc/apt/sources.list.d/*

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    unzip \
    && rm -rf /var/lib/apt/lists/*

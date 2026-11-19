# =============================================================================
# offline-paketo-bk: builder-base — 基础 OS 镜像层
# 仅安装系统依赖，供 builder-build 引用
# =============================================================================

ARG BASE_IMAGE

FROM ${BASE_IMAGE:?BASE_IMAGE is required (see profile spec.project.base)}

COPY docker/apt-sources/sources.list /etc/apt/sources.list
RUN rm /etc/apt/sources.list.d/*

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    unzip \
    && rm -rf /var/lib/apt/lists/*

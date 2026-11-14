# =============================================================================
# offline-paketo-bk: builder-base — 基础 OS 镜像层
# 仅安装系统依赖，供 builder-build 引用
# =============================================================================

ARG BASE_IMAGE

FROM ${BASE_IMAGE:-swr.cn-north-4.myhuaweicloud.com/ddn-k8s/docker.io/library/debian:trixie-slim}

COPY docker/apt-sources/sources.list /etc/apt/sources.list
RUN rm /etc/apt/sources.list.d/*

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    unzip \
    && rm -rf /var/lib/apt/lists/*

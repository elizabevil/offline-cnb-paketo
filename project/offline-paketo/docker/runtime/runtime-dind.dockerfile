# syntax=docker/dockerfile:1
# 仅用于生产离线 bundle；消费端从 images/runtime-dind.tar 加载。
ARG BASE_IMAGE_TAG
ARG BUNDLE_ARCH=linux-amd64
ARG DOCKER_VERSION=27.0.3

FROM ${BASE_IMAGE_TAG}

ARG BUNDLE_ARCH
ARG DOCKER_VERSION

# 复用已有 docker 物料（静态 Docker CLI + dockerd 依赖包），不依赖 Docker 镜像。
COPY --from=bundle /${BUNDLE_ARCH}/binaries/docker/${DOCKER_VERSION}/docker/ /usr/local/bin/

COPY --from=runtime runtime-dind-entrypoint.sh /usr/local/bin/runtime-dind-entrypoint
RUN chmod +x /usr/local/bin/runtime-dind-entrypoint

ENTRYPOINT ["/usr/local/bin/runtime-dind-entrypoint"]

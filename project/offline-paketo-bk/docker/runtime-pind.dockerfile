# syntax=docker/dockerfile:1
# 仅用于生产离线 bundle；消费端从 images/runtime-pind.tar 加载。
ARG BASE_IMAGE_TAG=offline-builder-base-bk:amd64
ARG BUNDLE_ARCH=linux-amd64
ARG PODMAN_STATIC_VERSION=6.2.1

FROM ${BASE_IMAGE_TAG}

ARG BUNDLE_ARCH
ARG PODMAN_STATIC_VERSION

COPY --from=bundle /${BUNDLE_ARCH}/binaries/podman-static/${PODMAN_STATIC_VERSION}/podman-linux-amd64.tar.gz /tmp/podman-static.tar.gz
RUN tar -xzf /tmp/podman-static.tar.gz --strip-components=1 -C / \
    && rm -f /tmp/podman-static.tar.gz

COPY docker/runtime-pind-entrypoint.sh /usr/local/bin/runtime-pind-entrypoint
RUN chmod +x /usr/local/bin/runtime-pind-entrypoint

ENTRYPOINT ["/usr/local/bin/runtime-pind-entrypoint"]

# syntax=docker/dockerfile:1
# 仅用于生产离线 bundle；消费端从 images/runtime-dind.tar 加载。
ARG DIND_BASE_IMAGE=docker:27-dind
FROM ${DIND_BASE_IMAGE}

COPY docker/runtime-dind-entrypoint.sh /usr/local/bin/runtime-dind-entrypoint
RUN chmod +x /usr/local/bin/runtime-dind-entrypoint

ENTRYPOINT ["/usr/local/bin/runtime-dind-entrypoint"]

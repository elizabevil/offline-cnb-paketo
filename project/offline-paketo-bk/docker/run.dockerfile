# =============================================================================
# offline-paketo-bk: run — 最小运行时基座
# 供 exporter 作为最终应用镜像的基础层
# =============================================================================

ARG BASE_IMAGE

FROM ${BASE_IMAGE:-docker-bkrepo.cwoa.net/h536b8/canway_d/blueking/run-heroku-bionic:ts4-260319}

LABEL io.buildpacks.stack.id="io.buildpacks.stacks.bionic"

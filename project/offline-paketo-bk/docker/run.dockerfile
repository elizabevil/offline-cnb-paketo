# =============================================================================
# offline-paketo-bk: run — 最小运行时基座
# 供 exporter 作为最终应用镜像的基础层
# =============================================================================

ARG BASE_IMAGE

FROM ${BASE_IMAGE:?BASE_IMAGE is required (see profile spec.project.base)}

LABEL io.buildpacks.stack.id="io.buildpacks.stacks.bionic"

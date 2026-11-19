# =============================================================================
# offline-paketo: Run Image（最小运行时）
# 供 exporter 作为最终 app 镜像的基础层
# -----------------------------------------------------------------------------
# 构建方式:
#   docker build -t offline-run:latest -f docker/run.dockerfile \
#     --build-arg BASE_IMAGE=... .
#
#   通过 just:
#   just docker-build-run
# -----------------------------------------------------------------------------
# 与 builder 镜像的区别:
#   - 只包含运行时依赖，不包含 buildpack/lifecycle/build 工具
#   - 最终 app 镜像 = run-image + buildpack 产物层
# =============================================================================

ARG BASE_IMAGE

FROM ${BASE_IMAGE:?BASE_IMAGE is required}
LABEL io.buildpacks.stack.id="io.buildpacks.stacks.bionic"

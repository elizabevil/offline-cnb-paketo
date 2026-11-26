# ─── justfile — 仓库根入口 ──────────────────────────────────────────────────
# 目标：在仓库任意位置 `just` 都能拿到同一套基座能力（修复"必须 cd scripts"的问题）。
#
# 常用：
#   just              列出全部 recipe
#   just doctor       环境自检
#   just list         物料状态
#   just fetch        下载 + 解压全部物料
#   just verify       校验
#   just platform=linux-arm64 fetch   切换目标平台

set dotenv-load := true

import 'script/justfile'
import 'test/justfile'

_default:
    @just --list


# offline-paketo-bk 测试入口
test-bk-quick *args:
    @just --justfile project/offline-paketo-bk/justfile test-quick {{args}}

test-bk-export-modes *args:
    @just --justfile project/offline-paketo-bk/justfile test-export-modes {{args}}

test-bk-runtime-matrix *args:
    @just --justfile project/offline-paketo-bk/justfile test-runtime-matrix {{args}}

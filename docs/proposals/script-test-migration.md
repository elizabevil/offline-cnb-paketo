# 脚本与测试目录迁移方案

> 状态：P0/P1/P2 已实施；P3 基础测试与语法门禁已实施，完整 runtime 矩阵仍可扩展。
> 目标：解决开发脚本、测试脚本、runtime 镜像和架构变量混在一起的问题。
> 本文只定义迁移方案，不描述已经完成的实现。

## 1. 背景

当前仓库里同时存在：

```text
project/offline-paketo-bk/run-test.sh
project/offline-paketo-bk/docker/runtime-*.dockerfile
project/offline-paketo-bk/docker/runtime-*-entrypoint.sh
script/tools/cnb/tests/lifecycle-singleton/
script/tools/cnb/tests/podman-export-probe.sh
```

问题：

| 问题 | 表现 |
|---|---|
| 脚本入口不统一 | `run-test.sh` / 历史 `run_test.sh` / `tests/*.sh` 混用 |
| 个人路径 | 脚本里出现 `/home/debi/Desktop/...` |
| 参数化过度 | 常用操作需要手填 `--app`、`--platform` |
| 架构变量混乱 | `_arch`、`_target_arch`、`_platform` 同时存在 |
| runtime 归属不清 | 通用 runtime 放在 BK 项目 |
| 日志位置不统一 | `.cache/logs`、`.cache/trials`、`/tmp` 混用 |
| 代理行为不一致 | 有的下载读代理，有的直连 |

`run-test.sh` 本身是开发期快速验证脚本，不是正式测试矩阵。
迁移后需要保留这个用途，但不再作为项目根入口。

## 2. 目标

### 2.1 用户入口统一

所有常用测试都通过 `just`：

```bash
just test-quick
just test-export-modes
just test-runtime-matrix
```

需要覆盖时使用 just 参数：

```bash
just test-quick app=/path/to/app
just test-export-modes platform=linux/arm64
```

不再要求：

```bash
./tests/export-modes.sh \
  --app /path/to/app \
  --platform linux-amd64
```

### 2.2 测试按子项目归属

```text
project/
├── offline-paketo/
│   └── tests/
│       ├── README.md
│       ├── quick.sh
│       └── matrix.sh
└── offline-paketo-bk/
    └── tests/
        ├── README.md
        ├── quick.sh
        ├── export-modes.sh
        └── matrix.sh

script/tools/cnb/
└── tests/
    ├── lifecycle-singleton/
    └── podman-export-probe.sh

script/tools/bksmartpacker_v3/
└── tests/
    └── ...
```

规则：

```text
tests/quick.sh      开发期快速验证
tests/*.sh          内部执行脚本
tests/README.md     默认值、示例、历史说明
```

### 2.3 runtime 归属通用项目

runtime-pind / runtime-dind 属于通用运行时，不属于 BK：

```text
project/offline-paketo/
└── docker/
    └── runtime/
        ├── runtime-pind.dockerfile
        ├── runtime-pind-entrypoint.sh
        ├── runtime-dind.dockerfile
        └── runtime-dind-entrypoint.sh
```

BK 项目只保留：

```text
project/offline-paketo-bk/docker/
├── builder-base.dockerfile
└── builder-build.dockerfile
```

## 3. 架构变量模型

只允许一个目标架构来源：

```text
TARGET_PLATFORM
  → TARGET_OS
  → TARGET_ARCH
```

其他变量全部派生：

```text
TARGET_PLATFORM
  → TARGET_OS
  → TARGET_ARCH
      → BUILDER_IMAGE
      → RUNTIME_BASE_IMAGE
      → RUNTIME_IMAGE_TAG
      → BUNDLE_PLATFORM
      → DIST_DIR
      → CACHE_DIR
```

示例：

```text
BKS_PLATFORM=linux/arm64
  → TARGET_PLATFORM=linux-arm64
  → TARGET_OS=linux
  → TARGET_ARCH=arm64
  → BUILDER_IMAGE=offline-builder-build-bk:arm64
  → RUNTIME_BASE_IMAGE=...trixie-slim-linuxarm64
  → RUNTIME_IMAGE_TAG=offline-runtime-pind-bk:arm64
  → BUNDLE_PLATFORM=linux-arm64
  → DIST_DIR=dist/linux-arm64
```

宿主机变量只用于默认值：

```text
HOST_PLATFORM
HOST_ARCH
```

禁止：

```text
目标构建使用 _arch
目标构建使用宿主架构
脚本里写死 amd64
脚本里写死个人路径
```

## 4. justfile 入口设计

### 4.1 子项目 justfile

`project/offline-paketo-bk/justfile` 增加：

```make
_fx       := env_var_or_default("S_MART_SRC", _dir / "fixtures/single")
_platform := env_var_or_default("BKS_PLATFORM", _host_platform)
```

recipe：

```make
test-quick app="{{_fx}}" platform="{{_platform}}":
    @{{_dir}}/tests/quick.sh {{app}} {{platform}}

test-export-modes app="{{_fx}}" platform="{{_platform}}":
    @{{_dir}}/tests/export-modes.sh {{app}} {{platform}}

test-runtime-matrix app="{{_fx}}" platform="{{_platform}}":
    @{{_dir}}/tests/matrix.sh {{app}} {{platform}}
```

用户默认：

```bash
just test-quick
```

覆盖：

```bash
just test-quick app=/path/to/app
just test-export-modes platform=linux/arm64
```

### 4.2 根 justfile

根入口只转发：

```make
test-quick *args:
    @just --justfile project/offline-paketo-bk/justfile test-quick {{args}}

test-export-modes *args:
    @just --justfile project/offline-paketo-bk/justfile test-export-modes {{args}}

test-runtime-matrix *args:
    @just --justfile project/offline-paketo-bk/justfile test-runtime-matrix {{args}}
```

这样使用者不需要记忆多个脚本路径。

## 5. 脚本约定

每个 `tests/*.sh` 必须：

```bash
#!/usr/bin/env bash
set -euo pipefail
```

接受：

```text
$1 = app 目录
$2 = platform
```

默认值不在脚本里重复，优先由 justfile 传入。

脚本负责：

- 校验 app/platform
- 创建临时 workspace
- 调用 bksmartpacker
- 保存日志
- 校验产物
- 清理

日志统一：

```text
.cache/tests/<project>/<platform>/
```

禁止：

```text
/home/debi/...
.cache/logs 和 .cache/trials 混用
```

## 6. 下载与代理

### 6.1 materials

```bash
cnb materials lock
cnb materials sync
```

必须使用：

```text
HTTPS_PROXY
HTTP_PROXY
CNB_DOWNLOAD_PROXY
```

### 6.2 直接下载

只允许：

```text
just podman-static
```

并统一使用 justfile 的代理变量。

禁止脚本里新增裸 `curl` 下载二进制。

## 7. run-test.sh 历史记录

原文件：

```text
project/offline-paketo-bk/run-test.sh
```

历史用途：

```text
开发期快速验证 CMDB SaaS 打包
不是 CI 测试矩阵
不是正式入口
```

迁移：

```text
project/offline-paketo-bk/run-test.sh
  → project/offline-paketo-bk/tests/quick.sh
```

并写入：

```text
project/offline-paketo-bk/tests/README.md
```

旧文件名不再保留。

## 8. 迁移阶段

### 已完成

- 各子项目 `tests/` 目录
- `run-test.sh` → `tests/quick.sh`
- justfile 默认值与根入口
- `TARGET_ARCH` 单一目标架构
- runtime Dockerfile/entrypoint 移到 `offline-paketo/docker/runtime/`

### 已实施 P3 基础

- `offline-paketo/tests/quick.sh` 可执行 builder + plan 冒烟
- test scripts 纳入基础语法门禁
- `matrix.sh` 已作为 runtime 矩阵入口保留

### 仍可扩展

- `matrix.sh` 接入完整 pind/dind 组合
- CI 中的 runtime 矩阵任务


### P0：测试目录

1. 建立各子项目 `tests/`。
2. 把 `run-test.sh` 改为 `tests/quick.sh`。
3. 删除旧 `run-test.sh`。
4. 写 `tests/README.md`。
5. 增加子项目/根 justfile recipe。

### P1：变量统一

1. 删除 `_arch` 的目标构建用途。
2. 统一为 `TARGET_ARCH`。
3. 所有派生变量从 `TARGET_PLATFORM` 计算。
4. 所有 pipeline 调用显式传 `--platform`。

### P2：runtime 迁移

1. 移动 runtime Dockerfile/entrypoint 到 `offline-paketo/docker/runtime/`。
2. BK pipeline 改为引用共享 runtime。
3. 删除 BK 内重复 runtime 文件。

### P3：测试矩阵

1. `tests/matrix.sh` 覆盖 layout/daemon。
2. 覆盖 amd64/arm64。
3. 输出统一 summary。
4. CI 只跑 matrix，不跑 quick。

## 9. 验收

```text
rg '/home/debi' project script
→ 无结果

rg 'run-test.sh|run_test.sh'
→ 只存在于历史说明

rg 'docker-static'
→ 无重复物料

rg '_arch'
→ 只用于 host 默认值

find project/*/tests -maxdepth 1 -type f
→ 各子项目结构一致
```

命令：

```bash
just test-quick
just test-export-modes
just test-runtime-matrix
just check
cnb pipeline profile validate project/offline-paketo-bk/pipeline.toml
```

## 10. 决策

| 决策 | 结论 |
|---|---|
| 测试入口 | justfile |
| 脚本默认值 | 放 justfile，不重复 |
| 测试目录 | 按子项目 `tests/` |
| runtime 归属 | 通用 runtime 放 offline-paketo |
| 架构变量 | 单一 `TARGET_PLATFORM` → `TARGET_ARCH` |
| quick 脚本 | 保留，但只用于开发冒烟 |
| 旧 run-test | 迁移后删除，历史写入 README |
| 下载 | materials 或 just podman-static，统一代理 |

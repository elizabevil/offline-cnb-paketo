# bksmartpacker_v3 fork 说明

本仓库的 `script/tools/bksmartpacker_v3` 是上游的 fork，不是原样拷贝。
本文只讲**怎么维护、怎么升级**；与上游的功能差异见
[vs-upstream.md](vs-upstream.md)，架构见工具目录下的 `ARCHITECTURE.md`。

## 一、基线与来源

| 项 | 值 |
|---|---|
| 上游仓库 | `ssh://git@code.cwoa.net:43990/rd-fy22-canway-platform-products/bksmartpacker_v3.git` |
| 基线提交 | `e2025de feat: 工具构建切换静态链接` |
| 来源声明 | 工具目录 `NOTICE` |
| 本地差异 | 约 450 行，集中在 executor、fake runner、打包与 inspect |

## 二、维护规则

1. 差异只允许出现在以下文件/包：
   - `pkg/builder/executors/docker.go`
   - `pkg/builder/executors/constants.go`
   - `pkg/smartpkg/`（inspect）
   - `pkg/utils/pack.go`（确定性打包）
   - `cmd/bksmartpacker/main.go`
2. `pkg/builder` 的 plan、types、config、conf 保持与上游一致。
3. 新增能力优先做成环境变量或 executor 分支，不污染通用逻辑。
4. 每次改动同步更新：
   - `README.md`（使用）
   - `ARCHITECTURE.md`（结构）
   - `vs-upstream.md`（差异）
   - 必要时 `NOTICE`（来源/基线）

## 三、升级上游

```bash
# 1. 查看上游改了什么
git -C <upstream> log --oneline e2025de..HEAD -- \
  pkg/builder/executors pkg/types pkg/config pkg/cmd conf

# 2. 以新提交为基线重放本 fork 的差异
# 3. 更新 NOTICE 的基线提交与差异规模
```

升级后必须重新验证：

```bash
just --justfile script/tools/bksmartpacker_v3/justfile build
cd project/offline-paketo-bk
CNB_BIN=../../script/bin/cnb-linux-amd64 \
BPS_BIN=../../script/bin/bksmartpacker-linux-amd64 \
just smart /path/to/release-amd64
script/bin/bksmartpacker-linux-amd64 --inspect dist/linux-amd64/s-mart.tar.gz
```

## 四、构建

```bash
# 宿主架构
just --justfile script/tools/bksmartpacker_v3/justfile build

# 交叉编译
TARGET=linux-arm64 just --justfile script/tools/bksmartpacker_v3/justfile build
```

产物输出到 `script/bin/bksmartpacker-<os>-<arch>`。

## 五、已知限制

- `daemon` 导出仅适用于真 dockerd；podman 的 docker 兼容层会让 lifecycle
  报 `singleton connection already in use`（issue #1763）。podman 下用 `layout`。
- 上游没有 executor 级测试，升级后必须手工跑一次 amd64/arm64 打包。

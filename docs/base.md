# 基座设计（script/ + bundle/ + assets.lock.toml）

> 目标：让三个子项目（offline-paketo / offline-paketo-bk / offline-heroku-bk）
> 共享同一份"物料 + 编排 + 校验"能力，而不是各写一套版本号和下载脚本。

---

## 一、职责边界

| 层 | 位置 | 只做一件事 | 不做什么 |
|---|---|---|---|
| 事实层 | `assets.lock.toml` | 声明"要哪些二进制、什么版本、什么哈希" | 不写流程、不写路径 |
| 能力层 | `script/lib/common.sh` | 日志 / 平台映射 / 代理 / 下载 / 解压 | 不认识物料 id |
| 解析层 | `script/tools/cnb`（Go） | TOML 解析 / 校验 / 重打包 | 不下载、不改 lock |
| 编排层 | `script/fetch.sh` | 下载、解压、marker 写入；verify/pin 委托 cnb | 不构建镜像 |
| 入口层 | `justfile` + `script/justfile` | 参数转发 + 展示 | 不写业务逻辑 |
| 物料层 | `bundle/` | 存归档与解压产物 | 不进 git |

**关键取舍**：just 只做"薄壳"。逻辑放 shell，是为了让 CI（不装 just 也能跑）、容器内、子项目脚本能复用同一套实现。

### 1.1 物料布局：架构先行

```
bundle/
├── linux-amd64/
│   ├── archives/     原始归档
│   ├── binaries/     解压产物（含 .meta/ 解压标记）
│   └── sdk/          预置 SDK
└── linux-arm64/
    └── ...
```

选择 `<platform>/<kind>` 而非 `<kind>/<platform>` 的三个理由：

1. **整体操作**：打离线分发包、清缓存、挂卷时，`bundle/linux-arm64` 就是一个完整单元，不会漏掉某一类。
2. **迁移成本**：按架构拆 tarball 直接对应 `buildx --platform` 的产物；反过来要先遍历再组装。
3. **误用成本**：`bundle/binaries/linux-amd64` 与 `bundle/binaries/linux-arm64` 视觉上太像，复制粘贴容易串；架构在前时前缀不同，一眼可辨。

历史子项目仍是 `<kind>/<platform>` 布局，`just import-legacy` 两种都认（优先新布局，回落旧布局）。

---

## 二、lock 文件为什么长这样

```toml
[[asset]]
id = "lifecycle"
version = "0.21.13"
default = true                    # 省略版本时的选择

  [asset.info]                    # 元信息，不参与构建
  name     = "CNB Lifecycle"
  homepage = "…"
  license  = "Apache-2.0"

  [asset.unpack]                  # 解包行为
  kind   = "tgz"
  strip  = 1
  dir    = "lifecycle/{version}"  # {version} 自动展开
  expect = ["detector", "launcher"]  # 相对 dir

  [asset.platform.linux-amd64]    # 下载源，按平台
  url    = "…/v{version}/lifecycle-v{version}+linux.x86-64.tgz"
  sha256 = "a6bc…"
```
归档存在?  ──否──▶ 下载（写 .part → 校验 → mv）
   │是
   ▼
哈希匹配?  ──否──▶ 删除重下（绝不"先用着"）
   │是
   ▼
解压标记匹配? ──否──▶ 重新解压 → 写 .meta/<id>.sha256
   │是
   ▼
产物齐全（expect）? ──否──▶ 重新解压
   │是
   ▼
  跳过
```

标记文件 `bundle/<platform>/binaries/.meta/<id>.sha256` 记录"这份产物是由哪个归档解出来的"。归档换了 → 标记不匹配 → 自动重解压。

> 旧实现只做第一层判断：`if [ -f archive ]; then skip; fi`。
> 后果：归档在、产物被删（换机器、清缓存、gitignore 误伤），`just download-all` 永远救不回来——这正是接手时 `project/offline-paketo/bundle/binaries` 为空、`docker build` 必然失败的原因。

---

## 四、代理链

```
--proxy 参数  >  PROXY_URL 环境变量  >  仓库根 .env  >  空（直连）
```

下载时同时设置**环境变量**与**显式 `--proxy`** 两条路径（部分 curl 版本/代理组合下 env 代理不生效）。

---

## 五、平台命名：请接受"三套命名"，不要试图统一

| 语境 | amd64 | arm64 | 例子 |
|---|---|---|---|
| 本基座 / OCI | `linux-amd64` | `linux-arm64` | `bundle/linux-amd64/archives/` |
| Go | `amd64` | `arm64` | `GOARCH` |
| `uname -m` | `x86_64` | `aarch64` | 运行时探测 |
| fnm | `linux` | `arm64` | `fnm-linux.zip` / `fnm-arm64.zip` |
| uv | `x86_64-unknown-linux-musl` | `aarch64-unknown-linux-musl` | Rust target triple |
| lifecycle | `linux.x86-64` | `linux.arm64` | 文件名里带点号 |

结论：**映射表放 lock，不放代码**。新增平台 = 加一列，不改逻辑。

---

## 六、信任边界（当前最大的"待还债"）

lock 里现在的 sha256 是 `provenance = local-capture`：由仓库里已有的归档反推。
它保证"同一份归档不被替换"，但**不保证**"与上游官方一致"。

收紧步骤：

1. 从上游 release 页面/`checksums.txt`/`.sha256` 文件获取官方值；
2. 回填 lock，把 `meta.provenance` 改为 `upstream`；
3. CI 增加 `just verify --strict`（未固定即失败）。

`just pin` 只负责**打印建议片段**，不自动写回 lock——写入必须人工确认，否则等于自己给自己发合格证。

---

## 七、踩过的坑（照抄即可避坑）

| 坑 | 现象 | 处置 |
|---|---|---|
| `just` 的路径函数语义 | import 后 `justfile_directory()` 指向**仓库根**，不是被 import 的文件 | 基座内部一律用 `source_directory()` |
| `#!/bin/sh` + `set -o pipefail` | Debian 12 以前 `/bin/sh`(dash<0.5.12) 直接报错退出 | shell 脚本统一 `#!/usr/bin/env bash`；或确认基础镜像 dash ≥ 0.5.12 |
| `tar --strip-components=1` | 归档根部的单文件（如 `lifecycle.toml`）被静默丢弃 | 不要把根级文件当作必需产物；用 `expect` 显式声明 |
| `stat -c%s` vs `stat -f%z` | Linux/macOS 参数不同 | `common.sh` 里已做双分支兜底 |
| `unzip --version` | 会先打印 `caution: both -n and -o specified` | 自检输出里只取首行，属正常噪音 |
| `set -o pipefail` + `\| head -n1` | 上游写满管道被关闭 → SIGPIPE → 退出码 141，`set -e` 直接终止脚本 | 管道末尾加 `\|\| true`，或改用 `awk 'NR==1'` |
| 只读 `HOME` 下 `go build` | `open ~/.cache/go-build/...: read-only file system` | 显式 `GOCACHE=<仓库>/.cache/go-build` |
| `bundle/<platform>/binaries` 手工改文件 | 下次 fetch 按标记重解压会覆盖 | 不要在 `binaries/` 下做任何手工修改，改 lock 或改脚本 |

---

## 八、待你拍板的四个决策

| # | 决策 | 我的建议 | 影响面 |
|---|---|---|---|
| 1 | 根 `bundle/` 与子项目 `bundle/` 的关系 | 根为唯一源，子项目通过 `BUNDLE_DIR` 或 symlink 指向；`scripts/` 逐步迁到根 `script/` | 三个子项目的 `.just` / Dockerfile 路径 |
| 2 | stack id 用什么 | 废弃 `io.buildpacks.stacks.bionic`，改用 target 声明（Buildpack API 0.12 的 `[[targets]]`）或 `io.buildpacks.stacks.jammy` | builder/run 镜像标签、`pack build` 兼容性 |
| 3 | SDK 真离线方案 | 预置 `bundle/<platform>/sdk/{node,python,go}` + 本地 index，去掉 `fnm install`/`uv venv --python` 的联网行为 | Node/Python buildpack 的 build 阶段 |
| 4 | 是否引入 CI | 先加一条 `just check && just verify` 的 pre-commit/CI 门禁 | 后续所有阶段的回归保护 |

---

## 九、演进路线

```
P0  物料基座（本轮，已完成）
    ├─ assets.lock.toml
    ├─ script/{fetch.sh, lib/common.sh, tools/cnb (Go: lock/verify/pin/repack)}
    ├─ 根 justfile（任意目录可用）
    └─ bundle/ 骨架 + 三态幂等 + 校验

P1  Buildpack 契约层
    ├─ script/lib/cnb.sh：layer.toml([types]) / launch.toml / env/PATH.prepend 统一生成
    └─ 修掉 node/python 的 `.launch` 与 go 的 command 数组问题

P2  镜像层
    ├─ builder / run 双镜像，stack 一致
    ├─ entrypoint 可移植（bash / 无 mountpoint 依赖 / skip 组合自洽）
    └─ builder 元数据标签由 lock 生成，杜绝手写版本号

P3  验证层
    ├─ 4 应用 × (detect / build / run) 矩阵
    ├─ SBOM 产出
    └─ CI 门禁
```

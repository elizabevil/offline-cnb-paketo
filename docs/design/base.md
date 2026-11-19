# 基座：当前约定

> 本文只描述**现在的**职责边界、布局与校验规则。
> 待拍板决策与演进路线见 [`docs/proposals/base-roadmap.md`](../proposals/base-roadmap.md)。
> 早期踩坑见 [`docs/ai/pitfalls.md`](../ai/pitfalls.md)。

> 目标：让两个子项目（offline-paketo / offline-paketo-bk）
> 共享同一份"物料 + 编排 + 校验"能力，而不是各写一套版本号和下载脚本。

---

## 一、职责边界

| 层 | 位置 | 只做一件事 | 不做什么 |
|---|---|---|---|
| 事实层 | `assets.lock.toml` | 声明"要哪些二进制、什么版本、什么哈希" | 不写流程、不写路径 |
| 能力层 | `script/lib/common.sh` | 日志 / 平台映射 / 代理（下载解压在 cnb 内） | 不认识物料 id |
| 解析层 | `script/tools/cnb`（Go） | TOML 解析 / 校验 / 重打包 | 不下载、不改 lock |
| 编排层 | `cnb materials` | 下载、解压、marker、校验 | 不构建镜像 |
| 入口层 | `justfile` + `script/justfile` | 参数转发 + 展示 | 不写业务逻辑 |
| 物料层 | `bundle/` | 存归档与解压产物 | 不进 git |

**关键取舍**：just 只做"薄壳"。逻辑放 Go（`cnb`），是为了让 CI、容器内、子项目脚本复用同一套实现，而不是各写一份 shell。

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

历史子项目遗留的 `<kind>/<platform>` 布局已不再支持；`bundle/` 统一为架构先行。

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
> 后果：归档在、产物被删（换机器、清缓存、gitignore 误伤），只凭"归档存在"就跳过解压会永远救不回来——这正是三态幂等（归档/哈希/marker 分开判断）存在的原因。

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

结论：**映射关系写在 `spec/manifest`，不散落在脚本里**。
新增平台 = 在 `KnownPlatforms`/`ArchOf` 加一行，不改其他逻辑；
`assets.toml` 只声明平台，不重复定义映射。

---

## 六、信任边界（当前最大的"待还债"）

`assets.toml` 的 `meta.provenance = local-capture`：当前的 sha256 由仓库里已有的归档反推。
它保证"同一份归档不被替换"，但**不保证**"与上游官方一致"。

收紧步骤：

1. 从上游 release 页面/`checksums.txt`/`.sha256` 文件获取官方值；
2. 回填 lock，把 `meta.provenance` 改为 `upstream`；
3. CI 增加 `cnb materials verify --strict`（未固定即失败）。

`cnb materials pin` 只负责**打印建议片段**，不自动写回 lock——写入必须人工确认，否则等于自己给自己发合格证。

---

# Learn-cnb-paketo

CNB（Cloud Native Buildpacks）/ Paketo 离线构建的实验与沉淀仓库。

核心命题：**在没有外网、没有 registry 凭据的内网环境里，把 `pack build` 跑通。**

---

## 仓库结构

```
Learn-cnb-paketo/
├── justfile              # 仓库根入口（just 任意位置可用）
├── cnb.toml             # 工作区布局：清单 / 锁 / bundle 的位置
├── project/assets.toml      # 物料声明（路径以本文件所在目录为根，类似 package.json）
├── project/assets.lock.toml # 物料锁定：解析后的 url / size / sha256（工具生成）
├── script/               # 公共脚本基座
│   ├── justfile          #   编排（被根 justfile import）
│   ├── ci.sh             #   门禁：结构 / 配置 / 物料
│   ├── lib/common.sh     #   纯能力库（日志/平台/代理/下载/解压）
│   ├── tools/cnb/        #   小工具源码（Go：lock / verify / pin / repack）
│   └── bin/cnb-linux-amd64  #   编译产物（不入库，`just tools` 生成）
├── bundle/               # 公共物料（不入库，可由 lock 重建）
│   └── <platform>/            # 架构先行：linux-amd64 / linux-arm64
│       ├── archives/          #   原始归档
│       ├── binaries/          #   解压产物
│       └── sdk/               #   预置 SDK（规划中，见 docs/design/base.md）
├── docs/                 # 设计文档
├── project/              # 子项目（每个目录自包含：配置 + Dockerfile + justfile）
│   ├── offline-paketo/       # Paketo 离线 builder
│   │   ├── pipeline.toml     #   流水线配置
│   │   ├── justfile          #   项目任务入口
│   │   └── docker/           #   Dockerfile 与构建上下文
│   └── offline-paketo-bk/    # Paketo × 蓝鲸 SmartPackage
```

---

## 快速开始

```bash
just                 # 列出全部命令
just doctor          # 物料自检：落盘状态、哈希、解压产物
just list            # 物料清单（TSV：id / version / 解包方式 / 平台）
just fetch           # 下载 + 校验 + 解包（三态幂等，断了可重跑）
just verify          # 校验哈希与产物（CI 门禁用）
just check           # 结构自检（lock / shell 语法 / gofmt / vet / 单测）
just test            # 单测 + 契约测试
just tools           # 编译 cnb / bksmartpacker 到 script/bin/
just convert <src> <dst>   # 镜像归档格式转换（oci-layout ↔ docker-archive）
```

切换架构：

```bash
just platform=linux-arm64 list
PLATFORM=linux-arm64 just fetch
```

代理（三处任选，优先级从上到下）：

```bash
just fetch --proxy http://127.0.0.1:7890
PROXY_URL=http://127.0.0.1:7890 just fetch
cp .env.example .env    # 写入 PROXY_URL
```

---

## 三条硬约定

1. **版本与 URL 只写在 `project/assets.toml`**。脚本、Dockerfile 里出现硬编码版本号 = 直接 review 拒绝。
2. **物料必须三态幂等**：归档存在 / 哈希匹配 / 解压标记匹配，三者独立判断。只凭"归档在"就跳过解压，会导致产物丢失后无法自愈。
3. **`bundle/` 不入库，`project/assets.toml` 与 `project/assets.lock.toml` 必须入库**。前者 60MB+ 可重建，后者是供应链锚点。

文档：

- [docs/design/architecture.md](docs/design/architecture.md) — **架构与两条案例流程**：工具 → 功能 → 效果
- [docs/design/pipeline.md](docs/design/pipeline.md) — 流水线的执行细节
- [docs/design/base.md](docs/design/base.md) — 设计、字段含义、踩坑清单
- [docs/guides/verify.md](docs/guides/verify.md) — **检验手册**：分层验收命令、期望输出、失败排查
- [script/tools/cnb/README.md](script/tools/cnb/README.md) — 小工具的契约与「新增子命令」模板
- [docs/design/scaffold.md](docs/design/scaffold.md) — **脚手架**：从参考项目生成新项目
- [docs/ai/pitfalls.md](docs/ai/pitfalls.md) — 18 条真实故障：现象 → 原因 → 修法

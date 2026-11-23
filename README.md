# Learn-cnb-paketo

CNB（Cloud Native Buildpacks）/ Paketo 离线构建的实验与沉淀仓库。

核心命题：**在没有外网、没有 registry 凭据的内网环境里，把 `pack build` 跑通。**

---

## 仓库结构

```
Learn-cnb-paketo/
├── AGENTS.md                 # AI 代理入口（必读清单与约束）
├── justfile                  # 仓库根入口（任意目录可用）
├── project/                  # 子项目与共享物料
│   ├── assets.toml           #   物料声明（路径以本文件目录为根）
│   ├── assets.lock.toml      #   物料锁定：url / size / sha256
│   ├── offline-paketo/       #   纯 CNB 离线 builder
│   │   ├── pipeline.toml     #     流水线（项目根的锚点）
│   │   ├── justfile          #     项目任务入口
│   │   ├── docker/           #     Dockerfile 与构建上下文
│   │   └── fixtures/         #     测试夹具
│   └── offline-paketo-bk/    #   Paketo × 蓝鲸 SmartPackage
├── buildpacks/               # 仓库级 buildpack 资产（按格式分组）
│   └── paketo/               #   自研组件（node/python/go/procfile）
├── bundle/                   # 物料落盘（不入库，可由 lock 重建）
│   └── <platform>/           #   linux-amd64 / linux-arm64
│       ├── archives/         #     原始归档
│       └── binaries/         #     解包产物（含 .meta 标记）
├── script/                   # 基座：编排与工具
│   ├── justfile              #   配方实现（根 justfile 转发）
│   ├── ci.sh                 #   CI 门禁
│   ├── check-updates.sh      #   对照上游版本
│   ├── lib/go.sh             #   Go 工具链定位
│   ├── tools/cnb/            #   cnb 源码（Go）
│   ├── tools/bksmartpacker_v3/  # bksmartpacker 源码（本地，不入库）
│   └── bin/                  #   编译产物（just tools 生成）
├── test/                     # 测试：contract / e2e / fixtures / lib
├── docs/                     # 文档（design / guides / ai / proposals / archive）
└── .github/workflows/ci.yml  # CI
```

## 快速开始

> AI 代理请先读 [AGENTS.md](AGENTS.md)。

```bash
just                 # 列出全部配方
just tools           # 编译 cnb / bksmartpacker 到 script/bin/
just check           # 门禁：lint + 单测 + 结构自检（提交前必跑）
just test            # 单测 + 契约测试
just list            # 物料清单（TSV）
just fetch           # 下载 + 校验 + 解包（三态幂等）
just verify          # 校验哈希与解压产物
just doctor          # 物料自检：落盘状态
just convert <src> <dst>   # 镜像归档格式转换（oci-layout ↔ docker-archive）
script/ci.sh          # 快速 CI（结构/配置/物料标记，秒级）
script/ci.sh --full   # 完整 CI（含 arm64 交叉编译与全量哈希）
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

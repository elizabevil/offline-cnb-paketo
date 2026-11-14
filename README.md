# Learn-cnb-paketo

CNB（Cloud Native Buildpacks）/ Paketo 离线构建的实验与沉淀仓库。

核心命题：**在没有外网、没有 registry 凭据的内网环境里，把 `pack build` 跑通。**

---

## 仓库结构

```
Learn-cnb-paketo/
├── justfile              # 仓库根入口（just 任意位置可用）
├── assets.lock.toml      # 物料唯一事实源：版本 / URL / sha256 / 平台矩阵
├── assets.lock.toml.example  # 新增物料时的模板（注释齐全，just check 会校验它）
├── script/               # 公共脚本基座
│   ├── justfile          #   编排（被根 justfile import）
│   ├── fetch.sh          #   拉取 / 校验 / 自检 / 导入
│   ├── lib/common.sh     #   纯能力库（日志/平台/代理/下载/解压）
│   ├── tools/cnb/        #   小工具源码（Go：lock / verify / pin / repack）
│   └── bin/cnb           #   编译产物（不入库，`just tools` 生成）
├── bundle/               # 公共物料（不入库，可由 lock 重建）
│   └── <platform>/            # 架构先行：linux-amd64 / linux-arm64
│       ├── archives/          #   原始归档
│       ├── binaries/          #   解压产物
│       └── sdk/               #   预置 SDK（规划中，见 docs/base.md）
├── docs/                 # 设计文档
├── project/              # 子项目目录
│   ├── offline-paketo/       # Paketo 离线 builder
│   ├── offline-paketo-bk/    # Paketo × 蓝鲸 SmartPackage
│   └── offline-heroku-bk/    # Heroku 系 × 蓝鲸 SmartPackage
```

---

## 快速开始

```bash
just                 # 列出全部命令
just doctor          # 环境自检（依赖 / python / 代理 / 磁盘）
just list            # 物料状态：归档 / 哈希 / 解压
just fetch           # 下载 + 解压（三态幂等，断了可重跑）
just verify          # 校验哈希与产物（CI 门禁用）
just check           # 结构自检（lock / 模板 / shell 语法 / gofmt / vet / 单测）
just repack <路径>   # 规范化 SmartPackage 的 web.tgz（docker-save → OCI layout）
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

1. **版本与 URL 只写在 `assets.lock.toml`**。脚本、Dockerfile 里出现硬编码版本号 = 直接 review 拒绝。
2. **物料必须三态幂等**：归档存在 / 哈希匹配 / 解压标记匹配，三者独立判断。只凭"归档在"就跳过解压，会导致产物丢失后无法自愈。
3. **`bundle/` 不入库，`assets.lock.toml` 必须入库**。前者 60MB+ 可重建，后者是供应链锚点。

文档：

- [docs/pipeline.md](docs/pipeline.md) — **完整流水线**：一图看懂执行链路与改动入口
- [docs/base.md](docs/base.md) — 设计、字段含义、踩坑清单
- [docs/verify.md](docs/verify.md) — **检验手册**：分层验收命令、期望输出、失败排查
- [script/tools/cnb/README.md](script/tools/cnb/README.md) — 小工具的契约与「新增子命令」模板
- [assets.lock.toml.example](assets.lock.toml.example) — 新增物料的填写模板

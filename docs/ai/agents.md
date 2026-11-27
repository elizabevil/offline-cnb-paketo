# AI 代理入口

> 面向在此仓库工作的 AI 代理。**先读本页，再动手。**
> 本页是唯一入口；仓库根目录不再放置指针文件。

## 一、开工前必读

| 文档 | 用途 |
|---|---|
| [git-workflow.md](git-workflow.md) | 提交规范：一提交一事、禁止修补堆积、改历史后验证树未变 |
| [go-build.md](go-build.md) | Go 规范与实战反模式、提交前自查 |
| [package-layout.md](package-layout.md) | 包与目录分层 |
| [pitfalls.md](../guides/pitfalls.md) | 19 条真实故障：现象 → 原因 → 修法 |
| [subproject-contract.md](../design/base/protocol/subproject-contract.md) | 子项目 justfile 契约 |
| [protocol-spec.md](../design/base/protocol/protocol-spec.md) | spec/* 的 API 形状与测试要求 |
| [../design/architecture.md](../design/base/architecture.md) | 当前架构与两条案例流程 |
| [../design/pipeline.md](../design/base/pipeline.md) | 路径链条与执行语义 |
| [../design/protocol-layers.md](../design/base/protocol-layers.md) | 协议分层与命名约束 |

## 二、常用命令

```bash
just                 # 列出全部配方
just tools           # 构建 cnb（bksmartpacker 源码存在时一并构建）
just check           # 门禁：lint + 单测 + 结构自检
script/ci.sh          # 快速 CI：结构 + 配置 + 宿主物料（秒级）
script/ci.sh --full   # 完整 CI：追加 arm64 交叉编译与双平台物料校验
script/ci.sh --offline # 不联网：不 fetch；bundle 缺失时跳过物料校验与 lint
```

单包（`script/tools/cnb`）：

```bash
gofmt -l .
go vet ./...
go test ./...
golangci-lint run --config ../.golangci.yml ./...
```

> 涉及容器（podman/docker）的步骤无法在受限沙箱执行，需在宿主机验证；
> 失败时把完整日志贴出来，不要只描述现象。

## 三、不可违反的约束

1. **配置即接口**：字段命名遵循 [protocol-layers.md](../design/base/protocol-layers.md) 第七章
   （表是名词、键是属性；禁用 `root`/`info`/`data` 这类泛词）。
2. **路径双根**：项目内路径相对 `pipeline.toml` 目录；物料路径相对 `assets.toml` 目录；
   跨项目共享只用 `${runtime.repo}` / `${runtime.bundle}` 绝对占位符。
3. **只做展开与执行**：stage 的 `run` 是 argv，原样交给 execve，不解析命令行语义。
4. **单一事实来源**：物料版本只写在 `project/assets.toml`；`cnb` 二进制变化后
   必须 `just tools` 回填 `assets.lock.toml` 并一起提交。
5. **提交历史**：不新增"修补上一个提交"的提交，直接 fixup；改历史后
   `git diff <整理前 sha> HEAD` 必须为空。
6. **文档分层**：`design/` 当前、`proposals/` 未来、`archive/` 历史；
   过时内容移入 archive，不留在当前文档里。

## 四、改动后自查

```bash
just check && bash script/ci.sh
```

| 改了什么 | 还要做什么 |
|---|---|
| 配置字段 | 同步结构体、校验文案、示例配置、占位符表、文档、测试夹具 |
| Go 代码 | `just tools`；确认 `project/assets.lock.toml` 的 cnb 哈希已更新 |
| 文档 | 无死链；过时内容不留在 `design/`；`docs/README.md` 索引同步 |
| CI | 本地跑 `script/ci.sh`；workflow 里不写死工具版本或死步骤 |

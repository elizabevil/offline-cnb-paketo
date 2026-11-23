# 协议分层

> 状态：**当前实现**。本文只描述现在生效的边界与 API，历史方案见 `docs/archive/`。

## 一、要解决的问题

`cnb` 要同时处理多份外部/自有协议。边界不清会导致：同一份文件两处解析、
格式知识散落、平台私有约定渗进通用包。

## 二、分层

```
internal/spec/manifest    物料清单（project/assets.toml / project/assets.lock.toml）
internal/spec/profile     流水线配置（project/<name>/pipeline.toml）
internal/spec/lifecycle   CNB 协议：buildpack.toml、order.toml
internal/spec/platform    平台契约：环境变量 → 结构化 Env
internal/spec/imagefmt    镜像归档：OCI layout ↔ docker-archive
internal/spec/checksrules lint 规则覆盖（TOML）

（平台私有）               BK 的 app_desc / s-mart → 只在 bksmartpacker 内
```

依赖方向：业务包（`cli`/`materials`/`plan`/`checks`）→ `spec`；`spec` 之间只在
`platform` → `manifest`（取平台常量）这类单向依赖。

## 三、四条硬规则

1. **单一解析器**：每类文件只有一处 `Parse`，其他包只消费结构体，不再解析文本。
   `just check` 用 grep 强制：`toml.Unmarshal` 只允许出现在 `internal/spec/` 与 `internal/profile/`。
2. **读写同包**：`Parse` 与写回逻辑同包；写回保留注释（`manifest.EditVersion`）。
3. **未知键报 Warn**：解析返回 `[]errs.Warning`，不静默忽略。
4. **平台私有协议不进通用包**：`BKS_*`、`app_desc`、`cnb-image-layers`
   不得出现在 `internal/`；`just check` 有对应 grep。

## 四、各层 API（与代码一致）

| 包 | 入口 | 返回 |
|---|---|---|
| `spec/manifest` | `Parse([]byte)` | `(*File, []errs.Warning, error)` |
| | `(*File).Validate()` | `[]error` |
| | `EditVersion(path, id, old, new)` | `(bool, error)`，保留注释 |
| | `ParsePlatform(string)` | `(Platform, error)` |
| `spec/profile` | `Parse([]byte)` | `(*File, []errs.Warning, error)` |
| | `(*File).Validate()` | `[]error` |
| `spec/lifecycle` | `ParseBuildpack([]byte)` | `(*Buildpack, []errs.Warning, error)` |
| | `ParseOrder([]byte)` | `(*Order, []errs.Warning, error)` |
| `spec/platform` | `Parse(getenv func(string) string)` | `(Env, []errs.Warning, error)` |
| | `(Env).Validate()` | `error` |
| `spec/imagefmt` | `Open(path)` | `(*Image, 格式, error)` |
| | `(*Image).Write(format, dest)` | `error` |
| `spec/checksrules` | `Parse([]byte)` | `(*Ruleset, error)` |

约定：

- 读取文件由调用方完成，`spec` 只接收 `[]byte`（`LoadLockFile` 是唯一例外，供 lock 使用）
- 校验返回 `[]error`（一次报多条），不返回 bool
- `spec` 不做 I/O 调度、不读环境变量、不打印

## 五、协议样例

### 5.1 物料清单 `project/assets.toml`

```toml
schema = 1

[meta]
name      = "demo"
platforms = ["linux-amd64", "linux-arm64"]

[[asset]]
id      = "lifecycle"
version = "0.21.29"
default = true

  [asset.info]
  name    = "CNB lifecycle"
  license = "Apache-2.0"

  [asset.unpack]
  kind   = "tgz"
  strip  = 1
  dir    = "lifecycle/{version}"
  expect = ["detector", "builder", "analyzer", "exporter"]

  [asset.platform.linux-amd64]
  url = "https://…/lifecycle-v{version}+linux.x86-64.tgz"
```

- 路径基准：**以清单文件所在目录为根**（类似 `package.json`）。
  `[bundle].path` 与本地来源的 `source` 都相对它解析；本项目
  `project/assets.toml` + `root = "../bundle"` → 仓库根的 `bundle/`
- 锁文件 `project/assets.lock.toml` 由 `cnb materials lock` 生成：`[locked]` 段记录 `url/size/sha256`
- `unpack.kind`：`tgz` / `tar.gz` / `zip` / `bin`
- `unpack.dir` 支持 `{version}`；`expect` 是解包后必须存在的相对路径
- 多架构共用归档时用 `multi_arch = true`，且 `expect` 必须含 `{arch}`

### 5.2 流水线 `project/<name>/pipeline.toml`

```toml
apiVersion = "cnb/v1"
kind       = "Pipeline"

[metadata]
name = "demo"

[spec.project]
# 物料清单位置（相对本文件）；默认 "assets.toml"
assets   = "../assets.toml"
platform = "linux-amd64"

  [spec.project.base]
  linux-amd64 = "debian:trixie-slim"

[spec.assets.lifecycle]
version = "0.21.29"

[[spec.stages]]
name        = "base"
description = "build the base image"
workdir     = "."
run = [
  "podman", "build", "-f", "docker/base.dockerfile",
  "-t", "demo-base:${derived.architecture}",
  "--build-arg", "BASE_IMAGE=${spec.project.base}",
  ".",
]

[[spec.checks.dockerfiles]]
path = "docker/base.dockerfile"
```

路径基准：

- `workdir`、`spec.checks.path` 相对 **`pipeline.toml` 所在目录**（项目根）
- 物料相关路径经 `assets` 关联后，相对 **`assets.toml` 所在目录**（物料根）解析
- 共享资源用 `${runtime.repo}` / `${runtime.bundle}` 等绝对占位符

规则：

- `spec.stages` 是列表，顺序即执行顺序；每个 stage 只有一条命令（`run` 的 argv）
- 没有类型分派；静态检查信息放 `spec.checks`，不要给 stage 加类型字段
- 占位符用完整配置路径，见 [pipeline.md](pipeline.md)

### 5.3 CNB 协议 `spec/lifecycle`

- `buildpack.toml`：`api` / `[buildpack]` / `sbom-formats` / `[[order]]` / `[[metadata.dependencies]]`
- `order.toml`：`[[order]]` + `[[order.group]]`（`id` / `version` / `optional`）
- 只解析，不生成；`cnb buildpack lint` 基于解析结果做规则检查

### 5.4 平台契约 `spec/platform`

环境变量 → `Env` 结构体：

| 变量 | 字段 |
|---|---|
| `SOURCE_GET_URL` | `SourceURL` |
| `REQUIRED_BUILDPACKS` | `Required` |
| `OUTPUT_IMAGE` | `OutputImage` |
| `CNB_RUN_IMAGE` | `RunImage` |
| `CACHE_IMAGE` | `CacheImage` |
| `CNB_APP_DIR` / `CNB_LAYERS_DIR` / `CNB_PLATFORM_DIR` | 目录 |
| `CNB_PLATFORM_API` | `PlatformAPI` |
| `CNB_EXPORT_MODE` | `ExportMode`（`layout` 默认 / `daemon` / `none`） |
| `CNB_LAYER_ARCHIVE` | `LayerArchive`（layout 导出后的层归档路径） |
| `CNB_BUILDPACK_ALIASES` | `BuildpackAliases` |

`Parse(getenv)` 便于测试；`Validate()` 检查导出模式与必填项。

### 5.5 镜像归档 `spec/imagefmt`

- `Open(path)`：识别 OCI layout 目录或 docker-archive tar
- `Write(format, dest)`：按目标格式写出，补齐 docker 兼容文件（`manifest.json`、`repositories`）
- 供 `cnb image convert` 与 lifecycle 的 `packLayout` 使用

## 六、删除清单（已完成）

以下历史实现已删除，不再兼容：

| 旧物 | 现状 |
|---|---|
| `tomledit` | 删除；写回由 `manifest.EditVersion` 完成 |
| `repack` / `smartpkg` 包 | 删除；格式转换归 `spec/imagefmt` |
| `.sha256` 与 `.sync.json` 双标记 | 保留 `.sha256`（旧标记）+ `.sync.json`（新），读取时兼容 |
| profile 的 `[images.*]` / `[commands.*]` | 删除；统一为 `[[spec.stages]]` |
| `internal/cnbconst` | 合并进 `spec/manifest` 常量 |

## 七、命名约束

配置文件名与字段名就是用户接口。命名失真会导致"读配置的人要猜"，
下面的规则对 `assets.toml` 与 `pipeline.toml` 同时适用。

### 7.1 表回答"是什么"，键回答"哪个属性"

```toml
# ✅ 表名是名词，键名是该名词的属性
[bundle]
path   = "../bundle"
layout = "arch-first"

# ❌ 两个词都太泛，读者仍需猜
[storage]
root = "../bundle"
```

### 7.2 禁止通用词做键

| 避免 | 原因 | 改用 |
|---|---|---|
| `root` | 谁的根？ | `path` / `directory`（并让表名限定语境） |
| `info` | 什么信息？ | `name` / `homepage` / `license` 等具体词 |
| `data` / `value` | 无信息量 | 用其真实含义 |
| `file` | 哪个文件？ | `assets` / `manifest` / `config` |

### 7.3 列表用复数，单值用单数

```toml
platforms = ["linux-amd64", "linux-arm64"]   # 列表
platform  = "linux-amd64"                     # 单值
[[asset]]                                     # 数组表
[[spec.stages]]                               # 数组表
```

### 7.4 布尔字段表达"开启什么"，不用双重否定

```toml
continueOnError = true    # ✅ 开启"出错继续"
# 不要 noFailStop = false 这类双重否定
```

### 7.5 时间/大小的单位写在值里或后缀里

```toml
timeout = "10m"      # 字符串带单位
size    = 12345678   # 字节，字段注释说明
```

### 7.6 字段名 = Go 字段 = 占位符路径

```toml
[spec.project]
platform = "linux-amd64"     # Go: Project.Platform   占位符: ${spec.project.platform}
assets   = "../assets.toml"  # Go: Project.Assets
```

改字段名时三处必须同时改，否则会出现"配置里叫 A、代码里叫 B、占位符叫 C"。
`just check` 与 `cnb pipeline variables` 可用于核对。

### 7.7 命名变更的完整清单

改一个字段名，至少同步：

1. `internal/spec/*` 的结构体与 toml tag
2. `Validate()` 的错误信息（会出现在用户眼前）
3. 示例配置：`project/assets.toml`、`project/*/pipeline.toml`
4. 占位符表（`internal/plan/vars.go`）与 `pipeline variables` 输出
5. 文档：`pipeline.md`、`constraints.md`、本文件、`scaffold.md` 等
6. 测试夹具（`plan_test.go` 里的 TOML 片段）

### 7.8 已应用的改名记录

| 旧 | 新 | 理由 |
|---|---|---|
| `[storage] root` | `[bundle] path` | 表与键都过泛 |
| `spec.project.directory` | `spec.project.assets` + 位置即事实 | 同一字段曾同时表达"项目在哪"和"相对谁" |
| `${arch}` / `${oci_platform}` | `${derived.architecture}` / `${derived.container_platform}` | 短名看不出出处 |
| `checks` 规则的 `kind` 字符串 | 代码内显式函数 | 逻辑枚举不应进配置 |

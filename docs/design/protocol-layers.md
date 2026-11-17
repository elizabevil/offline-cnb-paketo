# 协议分层方案

> 状态：已定稿（2026-11）。**不兼容历史遗留**：旧标记、旧脚本、旧双解析器一律删除，不做兼容层。

## 一、要解决的问题

当前 cnb 同时解析 6 类协议，但边界不清：

| 症状 | 实例 |
|---|---|
| 同一协议两套解析器 | `order.toml` 在 `lifecycle/order.go`（toml）与 `checks/buildpack.go`（正则）各解析一次 |
| 格式知识分散 | docker-archive 在 `archive`/`repack`/`smartpkg` 三处；OCI layout 在 `repack` 与 `smartpkg` |
| 平台契约外溢 | `CNB_*` 在 cmd，`SOURCE_GET_URL` 在 lifecycle，BK 别名在 cmd |
| 双协议并存 | 物料标记 `.sha256`（旧）与 `.sync.json`（新） |
| 无最终值视图 | 同一字段可被 lock / profile / env / flag 覆盖，无从得知生效值 |
| 阶段类型靠猜 | 旧 profile 用 `image=` / `command=` 的有无判断类型，新增 shell/python 需改解析器 |

## 二、分层模型

```
spec/manifest   物料清单（本项目自有）
spec/profile    流水线编排（本项目自有）
spec/lifecycle  CNB lifecycle 的 6 个文件协议
spec/platform   平台 → buildpack 契约（环境变量 / flags）
spec/imagefmt   镜像归档格式（docker-archive / OCI layout）

（平台私有）     BK 的 app_desc / artifact.json / s-mart → 只在 bksmartpacker 内
```

依赖方向：业务包 → spec；spec 之间不互相依赖（`platform` 可依赖 `lifecycle` 的枚举，单向）。

## 三、四条硬规则

1. **单一解析器**：每个协议只有一处 `Parse`；其他包只能消费结构体，不得再解析文本。
2. **写方向唯一**：`Parse` 与 `Encode` 同包；谁写谁校验（例如 sha256 只能由 `materials` 回填）。
3. **未知键必须报告**：解析器遇到未知字段返回 `Warn` 列表（不是静默忽略），协议升级必须显式处理。
4. **平台私有协议不得进入通用包**：`BKS_*`、`app_desc.yaml`、`.Version` 不出现于 `internal/`。

## 四、包结构与 API 形状

每个 spec 包只做三件事：**解析、编码、校验**。不做 I/O 调度、不读环境变量、不打印。

```go
package manifest

type File struct { Schema int; Storage Storage; Assets []Asset }
type Storage struct { Root string; Layout Layout }

func Parse(data []byte) (*File, []warn.Warning, error)
func (f *File) Encode() ([]byte, error)          // 仅用于生成模板
func (f *File) Validate() []error
```

约定：

- 解析函数统一签名 `Parse([]byte) (T, []warn.Warning, error)`，文件读取由调用方完成
- 校验返回 `[]error`（可一次报多条），不返回 bool
- 不导出可变字段以外的行为（无 `Load`、无 `Save`，路径由业务层决定）

## 五、各层协议案例

### 5.1 spec/manifest —— `assets.lock.toml`

```toml
schema = 1

[storage]
root   = "bundle"
layout = "arch-first"

[asset.info]
description = "CNB lifecycle 四阶段二进制"
usage       = "lifecycle run / exporter / analyzer"

[asset.unpack]
kind   = "tgz"
strip  = 1
dir    = "lifecycle/{version}"
expect = ["lifecycle", "detector", "analyzer", "exporter"]

[[asset]]
id      = "lifecycle"
version = "0.21.29"
default = true

  [asset.platform.linux-amd64]
  url    = "https://github.com/buildpacks/lifecycle/releases/download/v0.21.29/lifecycle-v0.21.29+linux.x86-64.tgz"
  sha256 = "…"
  size   = 12345678
```

职责：

- 结构体：`File` / `Storage` / `Asset` / `Info` / `Unpack` / `Source`
- 类型化字段：`Kind`（tgz/tar.gz/zip/bin）、`Platform`（linux-amd64…）、`Layout`
- 校验：schema、id 唯一、平台白名单、url/sha256/size 配对、`dir` 冲突
- 编辑（保留注释）留在 `tomledit`，由 `materials` 调用

### 5.2 spec/profile —— `config/projects/<name>.toml`（K8s 结构 + 统一 exec）

**格式用 TOML，结构照 K8s**：`apiVersion` / `kind` / `metadata` / `spec`。
`spec.stages` 是**列表**，顺序即执行顺序；每个 stage 只有一条命令，没有类型分派。

```toml
apiVersion = "cnb/v1"
kind       = "Pipeline"

[metadata]
name = "offline-paketo-bk"

[spec.project]
dir      = "project/offline-paketo-bk"
platform = "linux-amd64"

# ── 阶段：列表顺序 = 执行顺序 ───────────────────────────────
[[spec.stages]]
name    = "base"
workdir = "."
run = [
  "podman", "build",
  "-f", "project/offline-paketo-bk/docker/base.dockerfile",
  "-t", "offline-builder-base-bk:latest",
  "--build-arg", "BASE_IMAGE=debian:trixie-slim",
  "--build-context", "bundle=bundle/linux-amd64",
  ".",
]

[[spec.stages]]
name    = "run"
workdir = "."
run = ["podman", "build", "-f", "project/offline-paketo-bk/docker/run.dockerfile",
       "-t", "offline-run-bk:latest", "."]

[[spec.stages]]
name    = "builder"
workdir = "."
run = ["podman", "build", "-f", "project/offline-paketo-bk/docker/builder.dockerfile",
       "-t", "offline-builder-build-bk:latest",
       "--build-context", "bundle=bundle/linux-amd64",
       "--build-context", "buildpacks=buildpacks", "."]

[[spec.stages]]
name    = "package"
workdir = "project/offline-paketo-bk"
run     = ["bash", "scripts/build.sh", "--release"]
timeout = "10m"

  [spec.stages.env]
  MODE = "release"
```

#### 为什么统一 exec

| 之前（按类型分表） | 现在 |
|---|---|
| `docker` / `command` / `shell` / `python` 各一套字段与执行器 | 只有 `run` + `workdir` + `env` + `timeout` |
| 新增语言类型要写代码 | 换成解释器即可：`["python3", "tools/x.py"]` |
| 类型靠结构或字段推断 | 不需要推断 |

docker、shell、python 都不再是“类型”：

```toml
[[spec.stages]]
name = "lint"
run  = ["python3", "tools/check.py"]
```

#### 字段表

| 字段 | 必填 | 说明 |
|---|---|---|
| `apiVersion` | ✅ | 固定 `cnb/v1` |
| `kind` | ✅ | 固定 `Pipeline`（为将来 `Material` / `Profile` 留位） |
| `metadata.name` | ✅ | 与文件名一致 |
| `spec.project.dir` | ✅ | 项目目录（相对仓库根） |
| `spec.project.platform` | ✅ | 默认平台 |
| `spec.stages[].name` | ✅ | 唯一，重复报错 |
| `spec.stages[].run` | ✅ | 非空字符串数组；**不自动加 shell** |
| `spec.stages[].workdir` | ✅ | 执行目录（相对仓库根）；argv 内的相对路径按此解析 |
| `spec.stages[].env` | | 追加环境变量（与父进程合并） |
| `spec.stages[].timeout` | | 默认 `10m` |
| `spec.stages[].continueOnError` | | 默认 `false`；为真时失败继续（后续 stage 仍执行） |

路径规则：

- 所有路径相对仓库根（`workdir` 绝对化后作为 cwd）
- `run` 数组原样执行，不做变量展开、不拼字符串
- `--dry-run` 打印**最终 argv**（含绝对 workdir），与实际执行同源

#### 校验规则

1. `apiVersion` / `kind` 必须是支持值，未知值报错并列出支持列表
2. `metadata.name` 非空且与文件名一致
3. `spec.stages` 非空；`name` 唯一
4. `run` 非空、元素非空
5. `workdir` 必填且目录存在
6. `timeout` 可解析为 duration

docker 相关检查（`-f` 是否存在、`-t` 是否为空）放 `cnb lint`：从 argv 识别 `podman build` / `docker build`，属于尽力而为的检查，不是协议约束。

#### 5.2.1 谁执行

```
justfile          只做入口转发（选 profile、传 --src 等参数）
    ↓
cnb pipeline run  validate → （可选 --dry-run）→ 按 stages 列表顺序执行
    ↓
单执行器          exec.Command(run[0], run[1:]...) + workdir/env/timeout
    ↓（builder 镜像内）
cnb lifecycle run  detect → build → analyze → export
    ↓
bksmartpacker      image save + 组装 s-mart
```

```go
type Stage struct {
    Name            string
    Run             []string
    Workdir         string
    Env             map[string]string
    Timeout         time.Duration
    ContinueOnError bool
}

func RunStage(st Stage) error {
    cmd := exec.Command(st.Run[0], st.Run[1:]...)
    cmd.Dir = abs(st.Workdir)
    cmd.Env = append(os.Environ(), kv(st.Env)...)
    …
}
```

- 没有 kind 注册表，没有 executor 分派
- `justfile` 里不出现 `podman build` / `bash -c`
- 失败默认立即中止，已完成的 stage 不回滚（构建产物保留，便于重跑）

#### 已确定事项

| 决策 | 取值 |
|---|---|
| 格式 | TOML |
| 结构 | `apiVersion` / `kind` / `metadata` / `spec` |
| 顺序 | `spec.stages` 列表顺序 |
| 失败策略 | 立即中止；`continueOnError` 可单阶段豁免 |
| 变量引用 | 暂不支持 `valueFrom`/`envFrom`，只写字面量 |
| 入口 | `cnb pipeline run <file>`（只借 K8s 结构，不做 apply 语义） |
| 迁移 | 三个项目一次性改为新结构，不保留旧解析 |

不做（避免与 K8s 混淆）：

| 不做 | 原因 |
|---|---|
| `cnb apply` / `--prune` | 没有状态收敛与差异计算，只是按顺序执行一次 |
| `status` 子资源 | 执行完即结束，无持久化状态 |
| `---` 多资源文件 | 目前只有 `kind: Pipeline`，无需多文档 |

命令仍是 `cnb pipeline run` / `pipeline plan` / `pipeline profile validate`。

### 5.3 spec/lifecycle —— 6 个 lifecycle 协议

统一在一个包，按文件名分文件：

```
spec/lifecycle/
├── order.go        order.toml（读写）
├── buildpack.go    buildpack.toml
├── plan.go         plan.toml
├── analyzed.go     analyzed.toml
├── launch.go       launch.toml
└── layer.go        layer.toml
```

`order.toml` 示例（`Parse` 与 `Filter` 共用同一结构体）：

```toml
[[order]]
  [[order.group]]
  id      = "paketo-buildpacks/go"
  version = "4.2.1"

  [[order.group]]
  id      = "paketo-buildpacks/go-build"
  version = "2.3.0"
```

`analyzed.toml` 示例（只读）：

```toml
[run-image]
  reference = "index.docker.io/library/debian:trixie-slim"
  top-layer = "sha256:0e3f…"

[[previous-image]]
  ...
```

`launch.toml` 示例（只读，供检查/打包使用）：

```toml
[[processes]]
  type    = "web"
  command = ["/app/bin/start-web.sh"]
  default = true
```

职责：

- 事务：`lifecycle` 包负责写 `order`（过滤后），其余只读
- `checks` 不再用正则解析 `order.toml` 与 `buildpack.toml`，改为调用本包
- 删除 `internal/lifecycle/order.go` 的结构体定义（迁入本包）

### 5.4 spec/platform —— 平台契约与参数优先级

**目标**：解决“环境变量与命令行参数冲突”，给出唯一、可解释的优先级。

#### 优先级（从低到高）

```
1. 内置默认值（代码里的常量）
2. 环境变量（CNB_*，平台/SmartPacker 注入）
3. 命令行参数（--xxx，人工覆盖）
4. 显式禁用（--no-xxx / 空值语义，见下）
```

规则：

- 任何字段的**来源可追踪**：解析结果带 `Source`，`cnb config show` 能打印“来自 env / flag / default”
- 高优先级覆盖低优先级时，若两者都非空且值不同，返回 `Warn`（不静默）
- **空值不等于未设置**：flag 用 `Changed` 判断，env 用“存在性”判断

#### 参数表驱动

```go
type Source string

const (
    SourceDefault Source = "default"
    SourceEnv     Source = "env"
    SourceFlag    Source = "flag"
)

type Param struct {
    Name    string   // 展示名，如 "export"
    Env     string   // 环境变量名，如 "CNB_EXPORT_MODE"
    Flag    string   // 参数名，如 "export"
    Default string
    Kind    Kind     // string | bool | duration | enum
}

var Params = []Param{
    {"export",       "CNB_EXPORT_MODE", "export",       "layout", KindEnum},
    {"platform-api", "CNB_PLATFORM_API", "platform-api", "0.15",  KindSemver},
    {"app",          "CNB_APP_DIR",      "app",          "/app",  KindPath},
    {"output",       "OUTPUT_IMAGE",     "output-image", "",      KindString},
    // …
}
```

解析流程：

```go
type Value struct {
    Raw    string
    Source Source
}

func Resolve(getenv func(string) string, changed func(string) bool, get func(string) string) (map[string]Value, []warn.Warning)
```

- 遍历 `Params`：默认值 → env → flag，逐级覆盖并记录 `Source`
- 覆盖冲突（env 与 flag 都设置且不同）→ `Warn("env X overridden by --flag")`
- 未知 env（`CNB_*` 中不在表内）→ `Warn("unknown platform env: CNB_XXX")`
- 枚举非法（`CNB_EXPORT_MODE=foo`）→ 错误，指出允许值

#### 业务侧只接受解析结果

```go
env, warns, err := platform.ParseEnv(os.Getenv)
mode := env.ExportMode      // 已校验的枚举
app  := env.AppDir          // 已解析的默认值
```

`internal/lifecycle`、`materials`、`plan` 都不再调用 `os.Getenv`；只有 `internal/spec/platform` 读环境变量。

#### 与 flags 的关系

- `cnb lifecycle run` 只注册 `Params` 里的 flag，注册与解析共用同一张表
- 新增参数 = 在表里加一行，自然获得：env 支持、flag 支持、帮助文本、冲突检测、来源追踪

### 5.5 spec/imagefmt —— 镜像格式接口（两种实现）

**目标**：把“格式”抽象成接口，docker-archive 与 OCI layout 是同一接口的两个实现，调用方不写 `if format == ...`。

#### 接口

```go
package imagefmt

// Image 是格式无关的统一模型（读写共用）。
type Image struct {
    RepoTags []string
    OS, Arch string
    Config   Config      // Entrypoint / Env / Labels / WorkingDir
    Layers   []Layer     // 顺序即层顺序；每层含 MediaType / Digest / Blob 路径
}

// Format 是一种镜像归档格式。
type Format interface {
    Name() string                            // "docker-archive" | "oci-layout"
    Detect(path string) bool                 // 探测目录/文件是否属于本格式
    Read(path string) (*Image, error)
    Write(img *Image, dest string) error
}
```

#### 注册与选择

```go
// 内置实现，按探测顺序排列
var formats = []Format{dockerArchive{}, ociLayout{}}

// Detect 返回第一个匹配的格式；都不匹配则报错并列出已支持的格式。
func Detect(path string) (Format, error)

// Read 自动探测并解析；返回使用的格式，便于日志与错误提示
func Read(path string) (*Image, Format, error)

// Write 按名字选择格式（调用方显式声明目标格式）
func Write(img *Image, formatName, dest string) error
```

#### 两个实现

| 实现 | 识别 | 布局 |
|---|---|---|
| `dockerArchive` | 存在 `manifest.json` 且含 `<digest>/layer.tar` | `manifest.json` + `repositories` + 层目录 |
| `ociLayout` | 存在 `oci-layout` 与 `index.json` | `blobs/sha256/*` + `index.json` + `oci-layout`（可带 docker 兼容 `manifest.json`） |

共同点：层内容可能是 gzip 或未压缩 tar，**由 `imagefmt` 内部按魔数判断**，调用方不感知。

#### 调用方

```go
// repack：读一种、写另一种，只做策略
img, _, err := imagefmt.Read(src)
err = imagefmt.Write(img, "oci-layout", dst)

// 检查/打包：只读
img, format, err := imagefmt.Read(pkgPath)
log.Info("image format", "format", format.Name(), "layers", len(img.Layers))
```

- `archive` 包只保留 tar/zip 解包原语（`ExtractTarGz` / `ExtractZip`）
- 删除 `archive/applayer.go`、`repack` 内部 manifest/index 结构体、`smartpkg` 的格式判定
- 新增格式 = 实现 `Format` 并注册，调用方无需修改

### 5.6 BK 私有协议（不进 cnb）

```
script/tools/bksmartpacker/
└── pkg/spec/bk/
    ├── app_desc.go     app_desc.yaml（modules / sourceDir / processes）
    ├── artifact.go     artifact.json
    └── smartpkg.go     s-mart 包结构（.Version / <module>.tgz / logo）
```

`.Version` 的取值 `cnb-image-layers` 只允许出现在 `smartpkg.go` 的常量里。

## 六、落盘后的目录

```
script/tools/cnb/internal/
├── spec/
│   ├── manifest/
│   ├── profile/
│   ├── lifecycle/
│   ├── platform/
│   └── imagefmt/
├── materials/     # 业务：下载/校验/解包（消费 manifest）
├── plan/          # 业务：变量展开、阶段编排（消费 profile + manifest）
├── lifecycle/     # 业务：执行四阶段（消费 platform + lifecycle）
├── checks/        # 业务：静态检查（消费 lifecycle + profile）
├── repack/        # 业务：格式转换（消费 imagefmt）
└── …              # archive 只留 tar/zip 原语
```

## 七、删除清单（不做兼容）

| 删除项 | 原因 |
|---|---|
| `materials` 的 `.sha256` 旧标记与 `MarkerUnpinned` 分支 | 双协议并存 |
| `checks/buildpack.go` 的 `orderRe` / `groupRe` 正则 | 改调 `spec/lifecycle` |
| `internal/lifecycle/order.go` 的结构体定义 | 迁入 `spec/lifecycle` |
| `internal/archive/applayer.go`（docker-archive 写出） | 保留 `spec/imagefmt` 一处实现 |
| `repack` 内部的 manifest / index 结构体 | 改用 `spec/imagefmt` |
| profile 的 `[docker].bundle_dir` | 物料位置只由 `manifest.[storage]` 决定 |
| `lifecycle_env.go` 的 `defaultAliases` | 移到 profile `[build].aliases` |
| 各处零散的 `os.Getenv("CNB_*")` | 只允许 `spec/platform` 读取 |
| `if format == "oci" ... else ...` 分支 | 统一由 `imagefmt.Format` 接口承担 |
| profile 的 `[[stage]] + image/command 字段猜类型` | 改为 `spec.stages[].run` 统一 exec，单执行器 |
| `cnb materials get` 的 shell 赋值格式 | 只保留结构化输出（旧消费者已删除） |

## 八、验收方式

| 层级 | 测试 |
|---|---|
| 解析 | `test/contract/spec/*.golden` 金样本，解析后字段快照 |
| 编码 | 解析 → 编码 → 再解析，结构相等（round-trip） |
| 未知字段 | 未知键必须进入 `Warn`，并由 `cnb config doctor` 展示 |
| 单一解析器 | 静态检查：`toml.Unmarshal` 只允许出现在 `internal/spec/**` |
| 边界 | 静态检查：`internal/**` 中不得出现 `BKS_`、`app_desc`、`.Version` |

新增静态检查落到 `just check`（grep 规则 + go vet）。

## 九、迁移顺序

| 阶段 | 内容 | 验收 |
|---|---|---|
| 1 | `spec/lifecycle`：迁 `order`、`buildpack`，`checks` 改用 | `buildpack lint` 与 `lifecycle run` 都通过同一结构体 |
| 2 | `spec/manifest`：迁 lock 结构与校验 | `materials validate` 金样本一致 |
| 3 | `spec/platform`：参数表 + 优先级解析（default < env < flag） | `lifecycle run` 只用 `platform.Env`；冲突有 Warn；来源可查 |
| 4 | `spec/profile`（K8s 结构 + 统一 exec stage）+ `plan` 拆包 | 未知 apiVersion/kind 报错；`run`/`workdir` 必填；顺序只来自 stages 列表 |
| 5 | `spec/imagefmt`：接口 + 两种实现 + 自动探测 | `repack` 与 `package inspect` 读同一模型，结果一致；新增格式不改调用方 |
| 6 | 删除清单逐项清理 | `just check` 含新增边界检查 |

每阶段独立提交，附带金样本与边界检查；前一阶段未绿不进下一阶段。

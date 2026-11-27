# bksmartpacker_v3 架构

面向维护者，说明一次 S-Mart 构建内部发生了什么。使用方式见 [README.md](README.md)，与官方工具的差异见
[local-vs-smart-app-builder.md](../../../docs/design/components/bksmartpacker/local-vs-smart-app-builder.md)。

## 一、模块划分

```text
cmd/bksmartpacker
  └── main.go            仅入口与 run 流程

pkg/cli                  Kong 参数解析、校验、inspect 输出
pkg/appdesc              app_desc 读取、校验、模块过滤
pkg/buildpack            语言 → buildpack
pkg/plan                 BuildPlan/Step/Group、分组、Procfile、artifact.json
pkg/source               源码 tar
pkg/env                  dotenv 解析、env 合并、环境变量校验与写入
pkg/builder              PrepareBuildPlan 编排
pkg/builder/executors    fake runner、构建、导出、打包
pkg/container            运行时配置（backend + environment、socket、export mode）
pkg/types                协议 schema 与构建参数
pkg/utils                文件、归档、环境变量工具
pkg/smartpkg             S-Mart 包结构检查（inspect）
```

## 二、调用链

```text
main
 └─ cli.Parse                  → Options
 └─ cli.Options.Validate       → 跨字段校验
 └─ cli.Options.Params         → types.Params
 └─ cmd.BuildSMartPackage
     ├─ builder.PrepareBuildPlan
     │   ├─ appdesc.Load        读取 app_desc
     │   ├─ appdesc.Filter      按 --module 过滤
     │   └─ plan.New            生成 Steps / Groups
     ├─ container.ResolveFromEnv
     │   └─ RuntimeConfig        backend/environment/socket/export
     └─ executors.NewContainerExecutor
         └─ Execute
```

## 三、构建计划

### v2（默认）

```text
sourceDir + buildpacks 相同 → 一个构建组
组内只构建一次
Procfile key = <module>-<process>
输出 <group>.tar
artifact.json 记录 image_tar 与 proc_entrypoints
```

SOPS 四模块：

```text
api      ┐
callback ├─ default.tar
default  │
pipeline ┘
```

### v1（兼容）

```text
每个模块一个 Step
每个模块一次构建
输出 <module>.tgz
不生成 artifact.json
```

v1 仅用于旧平台/旧产物验证，CLI 默认 v2。

## 四、执行器

### 运行时配置

`pkg/container` 把运行时拆成两个维度：

```text
Backend:      docker | podman
Environment:  host | pind | dind
```

| Backend | Environment | socket / export |
|---|---|---|
| docker | host | `/var/run/docker.sock`，layout/daemon |
| podman | host | rootful/rootless socket，layout |
| podman | pind | `/run/podman/podman.sock`，layout |
| docker | dind | docker socket，daemon |

说明：

- `pind` 是 podman-in-docker 的**配置预设**，不负责启动嵌套 Podman。
- `dind` 是 docker-in-docker 的**配置预设**，不负责启动 Docker daemon。
- `BKS_DOCKER_SOCK` 可覆盖 socket。

### 一次构建

```text
ContainerExecutor.Execute
 ├─ prepareFakeRunner       单层空镜像，作为 run image
 ├─ exportFakeRunnerArchive docker-archive，供容器内使用
 ├─ for each Step / Group
 │   ├─ runBuild            源码 tar + 构建容器
 │   └─ exportArchive       取回 image tar
 └─ buildSMartPackage
     ├─ 写 .Version
     ├─ 写 app_desc.yaml
     ├─ 写 logo.png（存在时）
     ├─ 写 artifact.json（v2）
     └─ 打 s-mart.tar.gz
```

### 构建参数

控制变量仍用 `-e`：

```text
REQUIRED_BUILDPACKS
OUTPUT_IMAGE
CNB_RUN_IMAGE
SOURCE_GET_URL
CNB_PLATFORM_API
CACHE_IMAGE
CNB_REGISTRY_AUTH
CNB_EXPORT_MODE
CNB_LAYER_ARCHIVE
CNB_RUN_IMAGE_ARCHIVE
CNB_BUILDPACK_ALIASES
```

业务变量：

```text
app_desc < --env-file < --env
        ↓
<tmp>/platform/env/<KEY>
        ↓
lifecycle 注入 buildpack
```

### 挂载

- `/platform`：平台 env 目录，只读
- `/tmp/source.tgz`：源码 tar，只读
- `/var/run/docker.sock`：容器运行时 socket
- `/cnb-layer`：layout 导出目录
- 可选：run image archive、runtimes、额外挂载

## 五、导出方式

| 模式 | 容器内行为 | 宿主取回 | 适用 |
|---|---|---|---|
| layout | lifecycle 写 OCI layout | 直接读取层包 | podman / pind / 本地默认 |
| daemon | lifecycle 写 docker daemon | `image save` | 真 dockerd / dind |

`layout` 不依赖 daemon，是本地默认路径。

## 六、artifact.json

`pkg/plan/artifact.go` 生成：

```json
{
  "version": "1.0",
  "runtime": {
    "base_image_id": "default",
    "architecture": "amd64"
  },
  "app_artifacts": {
    "<module>": {
      "image_tar": "<group>.tar",
      "proc_entrypoints": {
        "<process>": ["/cnb/process/<module>-<process>"]
      }
    }
  }
}
```

约束：

- 只包含 app_desc 中的模块
- `image_tar` 必须存在于包内
- `proc_entrypoints` 与 Procfile key 一致
- 值使用绝对路径 `/cnb/process/<module>-<process>`，不依赖基础镜像 PATH
- 部分包（`--module`）默认禁止，调试需 `--allow-partial`

## 七、包结构

v2：

```text
s-mart.tar.gz
├── .Version
├── app_desc.yaml
├── logo.png
├── artifact.json
└── <group>.tar
```

v1：

```text
s-mart.tar.gz
├── .Version
├── app_desc.yaml
├── logo.png
└── <module>.tgz
```

## 八、检查与调试

```bash
bksmartpacker --inspect dist/linux-amd64/s-mart.tar.gz
bksmartpacker --inspect dist/linux-amd64/s-mart.tar.gz --inspect-format json
```

`--inspect` 输出：

- path / size
- `.Version`
- app_desc 是否存在
- artifact 是否存在、模块列表
- 层包摘要与 `has_source` 启发式标记

`has_source` 只检查 `/app`、`/workspace` 根目录的常见源码标记，不是完备检测。

## 九、约束

- v2 需要平台 apiserver ≥ 1.7
- `--architecture` 必须与实际镜像架构一致
- 每个构建组的 `sourceDir + buildpacks` 必须一致
- 组内 env 取并集，运行期差异仍由 app_desc 决定
- 不完整模块包不可部署

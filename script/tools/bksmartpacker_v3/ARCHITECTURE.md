# bksmartpacker_v3 架构

面向维护者，说明一次 S-Mart 构建内部发生了什么。使用方式见 [README.md](README.md)。

## 一、模块划分

```
cmd/bksmartpacker
  └── main.go           仅入口与 run 流程

pkg/cli                Kong 参数解析、校验、inspect 输出
pkg/appdesc            app_desc 读取、校验、模块过滤
pkg/buildpack          语言 → buildpack
pkg/plan               BuildPlan/Step/Group、分组、Procfile、artifact.json
pkg/source             源码 tar
pkg/env                dotenv 解析、env 合并、环境变量校验与写入
pkg/builder            PrepareBuildPlan 编排
pkg/builder/executors  fake runner、构建、导出、打包
pkg/container          运行时配置（backend + environment、socket、export mode）
pkg/types              协议 schema 与构建参数
pkg/utils              文件、归档、环境变量工具
pkg/smartpkg           S-Mart 包结构检查（inspect）
```

## 二、一次构建的调用链

```
main
 └─ builder.PrepareBuildPlan    → BuildPlan（Step / Group）
 └─ container.ResolveFromEnv    → RuntimeConfig
 └─ ContainerExecutor.Execute
     ├─ prepareFakeRunner      → 单层空镜像，作为 run image
     ├─ exportFakeRunnerArchive→ docker-archive，供容器内使用
     ├─ for each step
     │   ├─ buildStep           → 源码 tar.gz + 构建容器
     │   └─ exportImageLayer    → 取回层包
     └─ buildSMartPackage       → 写 .Version/app_desc/artifact/logo，打 tar.gz
```

### v2 构建组

默认 `--packaging-version v2`：

```
sourceDir + buildpacks 相同 → 一个构建组
组内只构建一次
Procfile key = <module>-<process>
输出 <group>.tar
artifact.json 记录 image_tar 与 proc_entrypoints
```

v1 仍保留旧的每模块 `<module>.tgz` 输出。

### buildStep 细节

1. 源码目录打成 `source.tgz`（`step.Procfile` 也会被写入）
2. 组装环境变量：
   - 控制变量仍用 `-e`：`REQUIRED_BUILDPACKS`、`OUTPUT_IMAGE`、
     `CNB_RUN_IMAGE`、`SOURCE_GET_URL`、`CNB_PLATFORM_API`、
     `CACHE_IMAGE`、`CNB_REGISTRY_AUTH`、`CNB_EXPORT_MODE`、
     `CNB_LAYER_ARCHIVE`、`CNB_RUN_IMAGE_ARCHIVE`
   - 业务变量：`app_desc` + `--env-file` + `--env` 合并后写入
     `<tmp>/platform/env/<KEY>`，由 lifecycle 注入 buildpack
   - 优先级：`--env` > `--env-file` > `app_desc`
3. 挂载：
   - 平台 env 目录到 `/platform`（只读）
   - 源码 tar（只读）
   - 容器运行时 socket
   - layout 模式下的层输出目录
   - 可选 run image 归档、runtimes、额外挂载
4. 目标平台通过 `--platform` 传入（`BKS_PLATFORM`）

## 三、两种导出方式

| | layout（默认） | daemon |
|---|---|---|
| 容器内 | lifecycle 导出 OCI layout 到挂载目录 | lifecycle 导出到 docker daemon |
| 宿主取回 | 直接读取层包 | `docker image save` |
| 依赖 | 只需文件挂载 | 需要可用 docker API |
| podman | 可用 | 不可用（singleton，issue #1763） |

layout 模式的核心是 `CNB_LAYER_ARCHIVE`：容器把导出结果写到
`/cnb-layer/<module>.tgz`，宿主直接复制进 S-Mart 包。

## 四、fake runner 与 run image

- exporter 要求 run image 至少有一层
- `prepareFakeRunner` 用 `image import` 构造只有一个空文件的镜像，
  避免 `FROM scratch` 产生 0 层
- 镜像名按平台区分（`fake-cnb-runner:linux-arm64`），防止多架构互相覆盖
- 需要真实 run image 时，用 `BKS_RUN_IMAGE_ARCHIVE_FILE` 以文件挂载

## 五、S-Mart 包结构

```
s-mart.tgz
├── .Version            # cnb-image-layers
├── app_desc.yaml       # 来自源码
├── artifact.json       # 若源码存在
├── logo.png            # 若配置存在
└── <module>.tgz        # 模块镜像归档
```

打包使用自有实现：属主固定 0:0、mtime 固定、目录 0755 / 文件 0644，
便于产物比对与校验。

## 六、平台与扩展点

- 平台差异不进入 plan，只体现在执行器：
  - 运行时选择（docker/podman）
  - socket 路径
  - 目标平台
  - 导出方式
- 新增能力优先做成环境变量或执行器分支，避免污染 `pkg/builder` 的通用逻辑

## 七、已知限制

- daemon 模式在 podman 下不可用（lifecycle issue #1763）
- 上游无 executor 测试，升级后需手工验证 amd64/arm64
- `--inspect` 只做结构检查，不校验哈希

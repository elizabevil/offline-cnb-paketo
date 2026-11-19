# buildpack 组件

> 目标：**结构照官方模板，实现用自己的**。
> 官方 buildpack 在构建阶段联网下载 SDK；我们的实现只替换"怎么拿到 SDK/依赖"，
> 其余契约（目录、层名、launch.toml、detect 输出）与官方保持一致。

## 一、目录结构

```
buildpacks/                 # 仓库级共享，按格式分组
└── paketo/                 # 自研组件（buildpack API 0.12）
    ├── go-dist/            #   提供 Go SDK（本地优先，镜像源可配）
    ├── go-build/           #   编译 Go 应用
    ├── go/                 #   meta：order 引用上面两个
    ├── node-engine/        #   fnm 安装 Node
    ├── npm-install/        #   安装依赖
    ├── npm-start/          #   写 launch.toml
    ├── nodejs/             #   meta
    ├── cpython/            #   uv 提供解释器
    ├── pip-install/        #   安装依赖
    └── python/             #   meta
```

当前只有 `paketo/` 一个分组。`heroku/`（BK 上游预编译组件）曾存在，
相关项目已移除；需要时按同样的 `<id>/<version>/` 布局新增分组。

每个组件目录：

```
<id>/<version>/
├── buildpack.toml
└── bin/{detect,build}
```

单架构脚本直接放 `bin/`，与官方"打包前"的源码布局一致。

## 二、组件契约

### 通用（所有组件）

| 项 | 要求 |
|---|---|
| 参数 | detect: `<platform_dir> <plan_path>`；build: `<layers_dir> <platform_dir> <plan_path>` |
| 层元数据 | `<layers>/<bp-id>/<layer>.toml`，含 `[types] launch/build/cache` |
| 环境 | `<layer>/env/<VAR>.<prepend\|append\|default\|delim>` |
| SBOM | `<layer>.sbom.cdx.json`（或 `spdx` / `syft`） |
| 退出码 | detect 未命中用 `exit 100`，错误用非 0 非 100 |
| 依赖声明 | `buildpack.toml` 的 `[[metadata.dependencies]]` 写当前锁定版本 |

### 各组件职责与层名

| 组件 | 层 | types | 输出 |
|---|---|---|---|
| node-engine | `node` | launch=true | PATH 指向 Node；提供 `node` |
| npm-install | `node_modules` | launch=true | 应用依赖；PATH 指向 `node_modules/.bin` |
| npm-start | — | — | `launch.toml`：`web = ["npm","start"]` |
| cpython | `python` | launch=true | 解释器 + PATH |
| pip-install | `site-packages` | launch=true | 依赖 |
| go-dist | `go` | launch=true | Go 工具链 + PATH |
| go-build | `bin` | launch=true | 编译产物；`launch.toml` 指向二进制 |

> 层名以官方为基准；实现时从 `bundle/buildpacks/buildpacks/<组件>` 的
> buildpack.toml 与文档交叉确认后固定，不凭印象。

## 三、离线策略（与我们现有逻辑一致）

每个"下载类"组件按优先级取 SDK：

```
1. 挂载的本地运行时   /cnb/runtimes/<lang>/<version>/
2. 持久缓存           /layers/cache/<lang>/{download,sdk,...}
3. 镜像源下载（兜底）  GH_MIRROR / 国内镜像
```

缓存目录统一在 `/layers/cache`，由平台挂载；launch 层必须自包含
（不能软链到 cache，cache 不进产物镜像）。

## 四、detect 契约

```
provides = [{ name = "node", version = "20.20.2" }]
requires = [{ name = "node" }]
```

- `go-dist` / `node-engine` / `cpython`：`provides` SDK
- `npm-install` / `pip-install` / `go-build`：`requires` SDK，`provides` 产物
- meta buildpack（`nodejs` / `python` / `go`）：只有 `[[order]]`，不含 bin/

## 五、SBOM

每个产出层的组件在 build 阶段写 `<layer>.sbom.cdx.json`（CycloneDX 1.5）：

```json
{
  "bomFormat": "CycloneDX",
  "specVersion": "1.5",
  "components": [
    { "type": "library", "name": "node", "version": "20.20.2",
      "purl": "pkg:generic/node@20.20.2", "licenses": [{"license": {"id": "MIT"}}] }
  ]
}
```

组件清单来自 `[[metadata.dependencies]]`，两者保持一致（lint 校验）。

## 六、`cnb buildpack lint`

静态检查（不执行 buildpack）：

| 检查 | 级别 |
|---|---|
| `buildpack.toml` 存在且 `api` 合法 | error |
| `[buildpack]` 的 id/version/name 齐全 | error |
| id 与目录名一致（`paketo-buildpacks_x`） | error |
| `bin/detect`、`bin/build` 存在且可执行 | error |
| meta buildpack 只有 `[[order]]`，无 `bin/` | error |
| order 的 group 引用的组件在本仓库存在 | error |
| `[[metadata.dependencies]]` 齐全（**仅**提供外部 SDK 的组件：node-engine / cpython / go-dist） | warn |
| SBOM 声明与实际写出的一致（**仅**写出 `*.sbom.*` 的组件） | warn |

> 规则只约束"真正提供外部依赖/SBOM"的组件，避免对 meta 与纯安装类组件
> 产生无意义告警。

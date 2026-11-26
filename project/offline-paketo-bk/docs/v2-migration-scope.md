# smart-app-builder v2 改造范围与兼容性

> 前提：部署平台 apiserver ≥1.7.x，自动识别 `artifact.json`。
> 目标：用 v2 替换 D1/D6，包 ≤200 MB，四模块进程正确启动。
>
> 状态：**已实现并构建出 v2 包**（2026-11-25）。

## 1. 当前实现与 v2 的差异

| 项 | 当前 bksmartpacker | v2 |
|---|---|---|
| 构建单位 | 每个模块一次 | 每个构建组一次 |
| 构建组 | 无 | `sourceDir + buildpacks` 相同 |
| Procfile | 每模块自己的进程名 | 应用级，`<module>-<process>` |
| 输出 | 每模块 `<module>.tgz` | 每组一个 `<group>.tar` |
| 模块与镜像映射 | 文件名/tag | `artifact.json` 的 `image_tar` |
| 进程入口 | 依赖镜像 metadata | `artifact.json` 的 `proc_entrypoints` |
| 包结构 | `.Version`、app_desc、logo、模块 tgz | `.Version`、app_desc、logo、artifact.json、image tar |
| 包体积 | 549 MB | 约 136 MB（单构建组） |

## 2. 改造范围

### 2.1 计划层（prepare）

文件：`pkg/builder/prepare.go`

1. 按 `sourceDir + required buildpacks` 分组
2. 每组生成一个 `ModuleBuildGroup`
3. 组内只保留一个主模块用于构建
4. 记录组内所有模块名

数据结构建议：

```go
type ModuleBuildGroup struct {
    SourceDir          string
    Buildpacks         []Buildpack
    BuildModuleName    string
    ModuleNames        []string
    Envs               map[string]string
    OutputImage        string
    OutputImageTarName string
}
```

### 2.2 Procfile（prepare）

文件：`pkg/builder/prepare.go`

当前：

```go
func generateProcfile(moduleDesc) // 每模块进程名
```

v2：

```go
func generateAppProcfile(modules) // 全部模块，key = <module>-<process>
```

示例：

```
default-web: bin/start_web.sh
default-beat: python manage.py celery beat -l info
pipeline-cworker: python manage.py celery worker ...
pipeline-node-timeout: python manage.py node_timeout_process
```

### 2.3 构建执行（executor）

文件：`pkg/builder/executors/docker.go`

1. 遍历构建组，而不是模块
2. 每组只执行一次 lifecycle build
3. `OUTPUT_IMAGE` 使用组主模块
4. 导出 `workspace/<group>.tar`
5. 不再为每个模块复制 tgz
6. 不再使用硬链接

### 2.4 artifact.json（executor）

新增生成逻辑：

```json
{
  "version": "1.0",
  "runtime": {
    "base_image_id": "default",
    "architecture": "amd64"
  },
  "app_artifacts": {
    "default": {
      "image_tar": "default.tar",
      "proc_entrypoints": {
        "web": ["/cnb/process/default-web"],
        "beat": ["default-beat"]
      }
    },
    "pipeline": {
      "image_tar": "default.tar",
      "proc_entrypoints": {
        "cworker": ["pipeline-cworker"],
        "node-timeout": ["/cnb/process/pipeline-node-timeout"]
      }
    }
  }
}
```

规则：

- `image_tar`：同一构建组的模块指向同一个文件
- `proc_entrypoints`：`/cnb/process/<module>-<process>`
- `runtime.base_image_id`：默认 `default`
- `runtime.architecture`：`amd64` / `arm64`

### 2.5 打包

`buildSMartPackage` 改为：

```
.Version
app_desc.yaml
logo.png
artifact.json
<group>.tar
```

移除：

- `<module>.tgz`
- 模块级硬链接
- 多 tag OCI layout

### 2.6 检查工具

`pkg/smartpkg/inspect.go` 需要识别：

- `artifact.json`
- `.tar` 镜像包
- 不再假设每个模块一个 `.tgz`

### 2.7 项目入口

`project/offline-paketo-bk/justfile`：

- 构建产物仍为 `s-mart.tar.gz`
- 不再调用 D1/D6 去重脚本
- `just dedup`、D6 工具可以删除或标记废弃

## 3. 兼容性

### 3.1 平台

| 平台版本 | v2 包 | 旧包 |
|---|---|---|
| ≥1.7.x | 支持 | 支持 |
| <1.7.x | 不支持 | 支持 |

结论：

- 目标平台 ≥1.7.x，v2 可用
- 如需兼容旧平台，保留 v1 输出模式，用参数切换

### 3.2 Buildpacks

| 项 | 影响 |
|---|---|
| Procfile key | `<module>-<process>`，buildpack 只是写文件，不受影响 |
| `/cnb/process/<type>` | procfile buildpack 自动生成 |
| pip-install 的 `launch.toml` | 需要检查是否仍写死 `web`，避免额外进程 |
| PATH/profile.d | 不影响 |
| Python/Node/Go | 都需要验证 namespaced 进程 |

### 3.3 运行时

v2 的 entrypoint 是：

```
<module>-<process>
```

要求：

- 镜像 PATH 包含 `/cnb/process`
- `/cnb/process/<module>-<process>` 存在
- launcher 能读取 metadata

这些由 CNB exporter 和 procfile buildpack 生成。

### 3.4 app_desc

- 仍使用 v3
- 模块与进程保持原样
- builder 只改变 Procfile key 和产物组织

### 3.5 旧产物

| 产物 | 兼容性 |
|---|---|
| 当前 549 MB 包 | 仍可部署 |
| D1 union 包 | 可部署，但不建议 |
| D6 包 | 平台只取第一个 manifest，不兼容 |
| v2 新包 | 需平台 ≥1.7 |

## 4. 分阶段实施

### 阶段 1：数据结构和分组

- 新增 `ModuleBuildGroup`
- 不改输出，只验证分组结果
- 单测覆盖 SOPS 四模块

### 阶段 2：Procfile 命名空间

- 生成 `<module>-<process>`
- 验证镜像 metadata 包含全部进程
- 不影响现有部署

### 阶段 3：v2 包输出

- 每组一个 image tar
- 生成 artifact.json
- 移除模块 tgz
- 先在一个测试应用验证

### 阶段 4：切换项目默认

- `packaging_version=v2`
- 保留 v1 回退
- 更新 justfile / 文档

## 5. 验收标准

1. SOPS 四模块包 ≤200 MB
2. 包内只有一个 image tar（或每构建组一个）
3. `artifact.json` 的模块与 app_desc 一致
4. 四个模块部署成功
5. `pipeline-node-timeout`、`pipeline-cworker` 正常
6. 同名 `web` 不冲突
7. arm64 同样通过
8. v1 输出仍可构建（如保留）

## 6. 风险

| 风险 | 处理 |
|---|---|
| 平台实际版本低于 1.7 | 先确认版本；保留 v1 |
| base_image_id 与平台配置不一致 | 默认 `default`，与 `SMART_CNB_IMAGE_CONF` 对齐 |
| pip-install launch.toml 额外 web | 移除或改为 namespaced |
| 多构建组 | 每组一个 tar，artifact 分别映射 |
| `--module` 调试包不可部署 | 文档标注，生产全量 |
| 旧包/新包混用 | 版本号区分 |

## 7. 结论

- v2 是 D1 的协议化版本，不是 D6 的延续
- 改造集中在计划、Procfile、执行器、artifact、打包
- buildpacks 基本不需要改，但要验证 namespaced 进程
- 当前平台 ≥1.7，建议直接切 v2，停止 D1/D6


## 8. 实现结果

SOPS 四模块：

```
dist/linux-amd64/bk-sops-v2-4modules.tar.gz
138 MB
```

结构：

```
.Version
app_desc.yaml
artifact.json
default.tar
logo.png
```

验证：

- 只执行了 1 次 builder build
- 一个 image tar
- 18 个 namespaced process
- `/cnb/process/pipeline-node-timeout` 存在
- artifact.json 四个模块映射完整
- 包体积 138 MB

待平台验证：

- 上传
- 四模块部署
- pipeline cworker / node-timeout


## 9. 代码优化记录（2026-11-25）

- 执行器合并 v1/v2 重复逻辑：`runBuild` + `exportArchive`
- v2 不再为每模块生成无用 Procfile
- 增加 `--allow-partial`：默认禁止生成不可部署的部分包
- 打包后校验每个 image tar 存在且非空
- 包体积 >200 MB 时告警
- `--inspect` 增加 `has_artifact` / `artifact_modules`
- `just check` / 单测通过

# 官方 smart-app-builder v2 方案解析

> 来源：TencentBlueKing/blueking-paas `builder-stack` 分支
> 目的：判断离线 paketo_bk 构建器是否应转向 v2。

## 1. v2 的组成

一个 v2 s-mart 包：

```
.Version              # cnb-image-layers
app_desc.yaml
artifact.json
<build-group>.tar     # 每个构建组一个镜像 tar
logo.png
```

注意：**不是每个模块一个 tgz，也不是一个 tgz 多镜像。**

## 2. 构建组

`plan.go`：

```
相同 sourceDir + 相同 required buildpacks
  → 同一个 ModuleBuildGroup
  → 只构建一次
  → 一个 OutputImageTarName
```

v2 中 `OutputImageTarName` 使用 `.tar` 后缀；v1 使用 `<module>.tgz`。

## 3. 应用级 Procfile

v2 的 Procfile key 是：

```
<module>-<process>
```

例如：

```
default-web
default-beat
pipeline-cworker
pipeline-node-timeout
api-web
```

因此同名 `web` 不会互相覆盖。

## 4. artifact.json

格式：

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
        "web": ["default-web"],
        "beat": ["default-beat"],
        "dworker": ["default-dworker"]
      }
    },
    "pipeline": {
      "image_tar": "default.tar",
      "proc_entrypoints": {
        "cworker": ["pipeline-cworker"],
        "node-timeout": ["pipeline-node-timeout"]
      }
    }
  }
}
```

关键字段：

| 字段 | 作用 |
|---|---|
| `image_tar` | 该模块使用哪个镜像 tar；可多个模块指向同一文件 |
| `proc_entrypoints` | 进程启动时使用的 entrypoint，格式为 `<module>-<process>` |
| `base_image_id` | 运行时基础镜像 ID |
| `architecture` | 目标架构 |

## 5. 平台如何处理

`dispatch.py`：

1. 读 `.Version`，判断 `cnb-image-layers`
2. 解析 `artifact.json`
3. 对每个模块：
   - 用 `image_tar` 找到镜像 tar
   - 推送到该模块的 registry tag
4. 部署时：
   - 从 `proc_entrypoints` 取 entrypoint
   - K8s command 设为 `<module>-<process>`
   - 镜像 PATH 中的 `/cnb/process/<module>-<process>` 指向 launcher

## 6. 重要结论：v2 没有修改 tgz 解析

平台仍然按 **一个镜像 tar 一个镜像** 处理。

`_construct_exported_image_manifest`：

```python
if manifest.json exists:
    parsed = DockerExportedImageManifest(**json.loads(manifest.json.read_text())[0])
else:
    manifest_digest = index_json["manifests"][0]["digest"]
```

两处都取 `[0]`。

因此：

- 一个 tar 内多个 image manifest → 只取第一个
- D6 的多镜像单 tgz 不可行
- v2 的解法不是“让平台支持多镜像”
- v2 的解法是“一个构建组只产出一个 image tar，并用 artifact.json 让多个模块引用它”

## 7. artifact.json 不只是描述

它实际影响：

1. 模块使用哪个 image tar（共享）
2. 进程使用哪个 entrypoint（解决同名进程和 node-timeout）
3. 运行时基础镜像和架构

所以它不是纯注释文件，而是部署映射协议。

## 8. D6 与 v2 对比

| 项 | D6 | v2 |
|---|---|---|
| 一个 tar 内多镜像 | 是 | 否 |
| 平台是否支持 | 否，只取第一个 | 不依赖 |
| 模块镜像 | 每个模块不同 manifest | 同构建组共享一个 image tar |
| 进程入口 | 依赖镜像 metadata | artifact.json 显式映射 |
| 同名 web | 冲突 | `<module>-web` 隔离 |
| 包体积 | ~136 MB | ~136 MB |
| 平台版本 | 无要求 | apiserver ≥1.7.x |

## 9. 开启方式

平台：

- apiserver ≥1.7.x
- 无额外开关

构建：

- `PACKAGING_VERSION=v2`
- 默认值即 v2
- 使用 smart-app-builder v2

包：

- `.Version=cnb-image-layers`
- 包含 `artifact.json`
- 一个或多个 image tar，每个 tar 只放一个镜像

## 10. 对离线 paketo_bk 的启发

### 10.1 不再做多镜像 tgz

D1/D6 的探索可以收敛：

```
一个构建组 → 一个 image tar
artifact.json 负责模块映射
```

### 10.2 进程命名空间

```
<module>-<process>
```

这是解决同名进程的正确方式。

### 10.3 构建单位

```
sourceDir + buildpacks
```

不是模块，也不是进程。

### 10.4 平台只做映射

平台不猜镜像内容，只看 artifact.json。

## 11. 改造清单

bksmartpacker 需要：

1. 按 `sourceDir + buildpacks` 分组
2. 组内只构建一次
3. 生成应用级 Procfile，key 为 `<module>-<process>`
4. 输出 `<group>.tar`
5. 生成 `artifact.json`
6. 包内只放：
   - `.Version`
   - `app_desc.yaml`
   - `logo.png`
   - `artifact.json`
   - 各构建组 image tar

## 12. 待验证

- 当前 apiserver 版本是否 ≥1.7.x
- 官方 smart-app-builder v2 对 SOPS 的构建结果
- 我们的 buildpacks 是否支持应用级 Procfile 的 `<module>-<process>` key
- `.tar` 镜像导出与 `artifact.json` 的匹配


## 13. 改造范围

见 [v2-migration-scope.md](v2-migration-scope.md)。
## 官方工具链

官方 blueking-paas 的 `builder-stack` 分支里，打包工具是 **smart-app-builder**，不是 bksmartpacker。

```
apiserver
  └─ 创建 SmartBuildRecord
       └─ 启动 smart-app-builder Pod
            ├─ fetch source
            ├─ PrepareBuildPlan
            ├─ 调用 CNB builder（bk-builder-heroku-bionic）
            │    └─ cnb-builder-shim 执行 lifecycle/buildpacks
            ├─ 保存 image tar
            ├─ 写 artifact.json
            └─ 打包 s-mart.tar.gz
```

| 组件 | 作用 |
|---|---|
| `smart-app-builder` | 官方 S-Mart 包构建器，v1/v2 由 `PACKAGING_VERSION` 控制 |
| `cnb-builder-shim` | 构建容器内的 lifecycle 适配层，替代 BK 旧 shim |
| `bk-builder-heroku-bionic` | CNB builder 镜像，包含 buildpacks |
| apiserver | 触发构建、解析 artifact.json、推镜像、部署 |

apiserver 配置：

```python
SMART_BUILDER_IMAGE       # smart-app-builder 镜像
SMART_BUILDER_SHIM_IMAGE  # cnb-builder-shim 镜像
```

构建 Pod 环境变量：

```python
BUILDER_SHIM_IMAGE
PACKAGING_VERSION  # v1 / v2
```

`bksmartpacker` 不在官方 blueking-paas 仓库的 builder-stack 中，是另一套自研/历史工具；
其当前实现按模块构建、没有 artifact.json 生成，因此不等同于官方 v2 构建器。

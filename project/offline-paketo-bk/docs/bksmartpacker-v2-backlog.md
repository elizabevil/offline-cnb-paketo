# bksmartpacker v2 下一步需求点（Review）

> 基于当前代码与官方 smart-app-builder v2 的对比。
> 目标：构建组、应用级 Procfile、artifact.json、每组一个 image tar。

## 1. Review 结论

方向正确，但当前方案还有 7 个缺口：

1. 计划层仍是“模块”
2. Procfile 仍是模块级
3. 执行器逐模块构建
4. 没有 artifact.json 生成
5. 包结构仍是模块 tgz
6. inspector 仍假设 tgz
7. D1/D6 脚本还在项目里

## 2. P0 需求（已实现）

### P0-1 构建组数据结构

新增：

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

规则：

- 分组键：`sourceDir + required buildpacks`
- 组内第一个模块为 BuildModuleName
- 其余模块加入 ModuleNames

验收：

- SOPS 四模块 → 1 个组
- 不同 sourceDir/语言 → 多个组

### P0-2 应用级 Procfile

规则：

```
<module>-<process>: command
```

验收：

- default 的 web → `default-web`
- pipeline 的 cworker → `pipeline-cworker`
- 同名 web 不覆盖

### P0-3 每组只构建一次

执行器从 Steps 改为 Groups。

验收：

- SOPS 四次构建 → 一次
- 只产生一个 image tar

### P0-4 artifact.json

生成：

```json
{
  "version": "1.0",
  "runtime": {"base_image_id": "default", "architecture": "amd64"},
  "app_artifacts": {
    "default": {
      "image_tar": "default.tar",
      "proc_entrypoints": {"web": ["default-web"]}
    }
  }
}
```

验收：

- 每个 app_desc 模块都有 image_tar
- 每个进程都有 proc_entrypoints
- 组内模块指向同一 image_tar

### P0-5 v2 包结构

```
.Version
app_desc.yaml
logo.png
artifact.json
<group>.tar
```

验收：

- 不再出现 `<module>.tgz`
- 包内 image tar 数量 = 构建组数量
- 包体积 ≤200 MB（SOPS）

### P0-6 inspector 适配

`--inspect` 需要识别：

- artifact.json
- `.tar` 镜像包
- v2 结构

不能再假设每模块一个 `.tgz`。

### P0-7 包校验

打包后校验：

- app_desc 模块 ⊆ artifact.app_artifacts
- 每个 image_tar 存在
- 每个进程有 entrypoint
- 每个 entrypoint 形如 `<module>-<process>`

不通过直接失败。

## 3. P1 需求

### P1-1 packaging_version

- 默认 v2
- v1 保留旧输出
- 通过参数/环境变量切换

### P1-2 base_image_id / architecture

- `base_image_id` 默认 `default`
- `architecture` 从目标平台推导
- 写入 artifact.json

### P1-3 构建环境合并

需要明确：

- 组内模块 env 如何合并
- 构建期 env 是否影响分组
- 运行期 env 是否只写 app_desc

### P1-4 兼容旧产物

- v1 包仍可构建
- 旧 549MB 包仍可部署
- 文档标注平台版本要求

## 4. P2 需求

### P2-1 SOURCE_GET_URL / DEST_PUT_URL

可选扩展，不设为必填。

### P2-2 外部 buildpack

- tgz 支持作为独立能力
- 不与 v2 artifact 混淆

### P2-3 `--module` 调试保护

- 默认不允许生成部分包
- 或显式 `--allow-partial`

## 5. 已确认：代码级结论

| 问题 | 结论 | 依据 |
|---|---|---|
| 组内 env 不同是否允许共享镜像 | 官方只按 sourceDir + buildpacks 分组，env 合并时后模块覆盖先模块；SOPS 的差异只是运行期 env，构建期 env 相同 | `plan.go buildModuleBuildGroups` / `maps.Copy(v.Envs, cfg.Envs)` |
| pip-install 是否写死 web | 是，`launch.toml` 写死 `web` + `python app.py`，v2 下会多一个进程，需要移除或改造 | `buildpacks/paketo/pip-install/bin/build` |
| image tar 用 OCI layout 是否被平台接受 | 平台先读 `manifest.json[0]`，否则读 `index.json.manifests[0]`；单镜像 OCI layout 可接受 | `dispatch.py _construct_exported_image_manifest` |
| base_image_id 平台默认值 | `default` → `bkpaas/docker/run-heroku-bionic:v1.0.4` | `settings/__init__.py SMART_CNB_IMAGE_CONF` |
| 进程名限制 | 官方仅 `fmt.Sprintf("%s-%s")`，未校验；当前模块名 default/callback/pipeline/api 安全，通用场景仍应加正则 | `plan.go GenerateProcType` |
| `--module` 部分包能否上传 | 不能；平台按 `app_desc.modules` 逐模块取 `image_tar`，缺失会失败 | `smart_app/views.py dispatch_package_to_modules` |

### 5.1 SOPS 组内 env 对比

default / callback / api 的 env 基本一致；pipeline 多：

```
DEFAULT_MAX_TASKS_IN_MEMORY
CELERY_EXPORTER_QUEUE
CELERY_EXPORTER_PORT
```

这些是运行期变量，buildpacks 不读取；构建期变量：

```
PIP_VERSION
PIP_INDEX_URL
...
```

四个模块一致，因此可以共享一个构建镜像。

### 5.2 pip launch.toml 的具体问题

当前 pip-install 会写：

```toml
[[processes]]
  type = "web"
  command = ["/bin/sh", "-c", "... python app.py"]
  default = true
```

v2 需要的是：

```
default-web
pipeline-web
```

因此 P0-2 必须：

- 删除 pip-install 的 `launch.toml` 进程生成
- 只保留 procfile buildpack 生成的 namespaced 进程

否则镜像里会同时存在：

```
web
default-web
pipeline-web
```

### 5.3 OCI layout 兼容性

平台代码：

```python
if manifest.json exists:
    parsed = DockerExportedImageManifest(**json.loads(manifest.json.read_text())[0])
else:
    manifest_digest = index_json["manifests"][0]["digest"]
```

我们的 v2 image tar 只需：

- 一个 manifest 条目
- `manifest.json` 或 `index.json`
- Config/Layers 路径有效

不需要 docker save。

### 5.4 base_image_id

默认平台配置：

```
name: bkpaas/docker/run-heroku-bionic
tag:  v1.0.4
```

artifact 写 `default` 即可；平台负责把应用层合并到该 base。

注意：本地 builder 基础镜像是 Debian trixie，应用层需与 bionic run image 兼容；当前 SOPS 已实际部署成功，暂时可接受，但仍应列入回归项。

### 5.5 `--module` 结论

平台：

```python
modules=set(handler.app_desc.modules.keys())
```

按 app_desc 全部模块逐个取 artifact；缺失模块会失败。

所以：

- `--module` 只能调试
- 不能生成可上传的部分包
- 或增加 `--allow-partial` 并明确不可部署

## 6. 建议实施顺序

1. P0-1 分组
2. P0-2 Procfile
3. P0-3 单次构建
4. P0-4 artifact.json
5. P0-5 包结构
6. P0-6/P0-7 检查与校验
7. P1 兼容
8. 删除 D1/D6 脚本

## 7. 验收清单

- SOPS 包 ≤200 MB
- 一个 image tar
- 四模块部署
- pipeline-node-timeout / cworker 正常
- 同名 web 不冲突
- arm64 通过
- v1 可回退
- `--inspect` 可读 v2 包

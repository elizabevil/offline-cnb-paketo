# v2 吸收与抛弃清单（需求方向）

> 目标：以官方 smart-app-builder v2 为参考，确定离线 paketo_bk 构建器
> 应吸收哪些机制、哪些不引入。

## 1. 总体判断

吸收的是 **协议与构建语义**：

- 构建组
- 应用级 Procfile
- artifact.json
- 一个构建组一个 image tar

不吸收的是 **官方运行环境与 BK 专用实现**：

- pind/dind
- cnb-builder-shim
- BK buildpack 版本与镜像源默认值
- docker save 作为唯一导出方式

## 2. 必须吸收

### A1. 构建组

```
sourceDir + required buildpacks 相同
  → 一个构建组
  → 只构建一次
  → 一个 image tar
```

理由：

- 当前 SOPS 四模块源码相同，重复构建四次
- 这是包体积和构建时间的根因

### A2. 应用级 Procfile

```
<module>-<process>: command
```

理由：

- 解决同名 `web` 冲突
- 解决 `node-timeout` 等进程被当作可执行文件
- 与平台 v2 的 entrypoint 规则一致

### A3. artifact.json

```json
{
  "version": "1.0",
  "runtime": {"base_image_id": "default", "architecture": "amd64"},
  "app_artifacts": {
    "module": {
      "image_tar": "group.tar",
      "proc_entrypoints": {"process": ["module-process"]}
    }
  }
}
```

理由：

- 平台据此选择 image tar
- 平台据此设置进程 entrypoint
- 多个模块可以共享同一个 image tar

### A4. 每组一个 image tar

```
<group>.tar
```

理由：

- 不再需要每模块 tgz
- 不再需要硬链接去重
- 不再需要多 image manifest

### A5. packaging_version

```
v2 默认
v1 仅兼容旧平台/旧包
```

理由：

- 平台 ≥1.7 已支持 v2
- 保留回退能力，避免锁死

### A6. base_image_id / architecture

写入 artifact.json：

```json
"runtime": {"base_image_id": "default", "architecture": "amd64"}
```

理由：

- 平台按基础镜像合并
- arm64 需要正确标记

## 3. 可以选择吸收

### B1. SOURCE_GET_URL / DEST_PUT_URL

官方用 URL 抽象源码和产物，适合远程构建。

本地可以：

- 保留 `--src` / `--dest`
- 额外支持 URL 作为可选输入输出

不强制。

### B2. BUILDPACK_TYPE

官方支持 `oci-embedded` / `tgz`。

本地已有 buildpacks 目录和物料，不需要该参数。

如果未来支持外部 buildpack 包，再引入。

### B3. 构建日志/阶段上报

官方有 build record、phase、stream。

本地只需要：

- 结构化日志
- 失败阶段
- 产物校验

不需要完整构建平台。

## 4. 不吸收 / 抛弃

### C1. pind / dind

官方 RUNTIME：

```
pind = podman-in-docker
dind = docker-in-docker
```

不吸收理由：

- 本地直接使用宿主 podman
- 已有 `BKS_CONTAINER_CLI`、socket、network
- 引入 pind/dind 只会增加复杂度

### C2. cnb-builder-shim

官方用 shim 在构建容器里跑 lifecycle。

本地已经有：

```
cnb lifecycle run
```

不需要替换为 shim。

### C3. BK buildpack 版本与镜像源

官方默认：

```
PYTHON_BUILDPACK_VERSION=v213
PIP_INDEX_URL=腾讯源
NODEJS_BUILDPACK_VERSION=v163
GO_BUILDPACK_VERSION=v238
```

不吸收：

- 我们使用本地 paketo buildpacks
- 源通过 `build.env` / 平台 env 注入
- 不在代码里写死腾讯源

### C4. docker save 作为唯一导出

官方：

```
docker save -> <module>.tar
```

本地：

```
OCI layout -> <group>.tar
```

平台 `_construct_exported_image_manifest` 同时支持：

- `manifest.json`
- `index.json`

所以可以保留本地 layout 导出，只要产物结构符合 artifact 映射。

### C5. D1 / D6

抛弃：

- D1 多 tag + 硬链接
- D6 多 image manifest 单 tgz
- D1 union 补层脚本

理由：

- 平台只取第一个 manifest
- v2 已提供正确协议
- 继续维护只会增加分叉

### C6. 模块级 tgz

抛弃：

```
default.tgz
callback.tgz
pipeline.tgz
api.tgz
```

改为构建组级 image tar。

### C7. 生产使用 --module

`--module` 仅保留为调试：

- 不生成可部署包
- 或显式 `--allow-partial`

## 5. 保留的本地差异

| 项 | 本地保留 | 原因 |
|---|---|---|
| buildpacks | paketo 本地实现 | 离线、可控 |
| 运行时 | 宿主 podman | 简单 |
| 导出 | OCI layout | 已验证 |
| 配置 | build.env + platform env | 已有机制 |
| CLI | `--src/--dest` | 使用习惯 |
| 物料 | bundle/assets | 离线 |

## 6. 需求编号

| 编号 | 需求 | 优先级 |
|---|---|---|
| R1 | 按 sourceDir + buildpacks 分组 | P0 |
| R2 | 每组只构建一次 | P0 |
| R3 | 应用级 Procfile `<module>-<process>` | P0 |
| R4 | 生成 artifact.json | P0 |
| R5 | 每组一个 image tar | P0 |
| R6 | 包内不再有模块 tgz | P0 |
| R7 | 保留 v1 回退 | P1 |
| R8 | base_image_id/architecture | P1 |
| R9 | `--module` 仅调试 | P2 |
| R10 | 文档标注 v2 平台要求 | P1 |

## 7. 验收

1. SOPS 四模块包 ≤200 MB
2. 包内一个 image tar + artifact.json
3. 四模块部署成功
4. pipeline-node-timeout / cworker 正常
5. 同名 web 不冲突
6. arm64 通过
7. v1 包仍可构建（如保留）

## 8. 决策记录

- 采用 v2 协议
- 不采用 pind/dind、builder-shim、BK 默认源
- 保留本地 buildpacks、podman、OCI layout
- 废弃 D1/D6
## 9. 参数澄清

### 9.1 architecture

`artifact.json.runtime.architecture` 是**目标应用镜像架构**：

```
amd64 / arm64
```

不是构建器进程架构，也不是宿主架构。

要求：

- builder 运行在目标架构
- 生成镜像的 config.architecture 与之一致
- artifact 写入相同值

当前 apiserver 代码只解析、不强校验，但应正确写入，避免后续调度/合并出错。

### 9.2 base_image_id

`base_image_id` 是**应用运行时基础镜像的标识**，不是构建器自己选基础镜像。

平台侧：

```python
base_image = mgr.get_cnb_runner_image_info(base_image_id=...)
```

从 `SMART_CNB_IMAGE_CONF` 查出 name/tag，再把应用层合并上去。

因此：

- **构建侧责任**：声明正确 ID，保证应用层与该 base 兼容
- **平台侧责任**：真正拉取并合并 base image
- 默认值 `default`
- 若平台有 `ts4` 等自定义 base，构建时必须传对应 ID

### 9.3 SOURCE_GET_URL / DEST_PUT_URL

可以作为扩展：

- 本地保留 `--src/--dest`
- 额外支持 `file://`、`http(s)://`
- 不设为必填

### 9.4 BUILDPACK_TYPE

不是历史包转换方案，也不是 v1/v2 的开关。

它是 `REQUIRED_BUILDPACKS` 的第一个字段：

```
<type> <name> ... <version>
```

| 值 | 含义 |
|---|---|
| `oci-embedded` | buildpack 已嵌入 builder 镜像 |
| `tgz` | 使用外部 buildpack tgz |

官方 smart-app-builder 只是把它拼进 `REQUIRED_BUILDPACKS`，由 builder shim 决定如何加载。

本地 paketo 方案：

- buildpacks 已在 builder 的 `/cnb/buildpacks`
- 通过别名和 order 选择
- 不需要 `BUILDPACK_TYPE`

外部 buildpack 包如果未来要做，应作为独立能力，不要和 v2 artifact 混在一起。
### 9.5 BUILDPACK_TYPE 通俗解释

把 builder 想成厨房，buildpack 想成菜谱：

| 类型 | 通俗解释 | 构建时发生什么 |
|---|---|---|
| `oci-embedded` | 菜谱已经印在厨房的墙上 | 直接从 builder 镜像的 `/cnb/buildpacks` 取，不下载 |
| `tgz` | 菜谱是一个单独的压缩包 | 从地址下载/解压 buildpack tgz，再交给 lifecycle |
| `tar` | 旧版 Heroku 菜谱压缩包 | 按 Heroku 格式加载 |
| `oci-image` | 菜谱做成容器镜像 | 规划中，shim 尚未支持 |
| `oci-file` | 菜谱做成 OCI 文件 | 规划中，shim 尚未支持 |

`REQUIRED_BUILDPACKS` 的格式：

```
<type> <name> ... <version>
```

例子：

```
oci-embedded bk-buildpack-python ... v213
tgz my-custom-buildpack ... v1.2.3
```

- 第一段告诉 builder：这个 buildpack 从哪里来
- 第二段是 buildpack 名字
- 第三段是版本

它和 v1/v2 **完全无关**：

```
v1/v2          = 整个 s-mart 包怎么组织
BUILDPACK_TYPE = 单个 buildpack 怎么交付给 builder
```

### 9.6 外部 buildpack 包的例子

场景：

```
应用需要一个内部 Node 插件 buildpack
builder 镜像里没有
```

做法：

1. 把插件打包成 `my-node-addon.tgz`
2. 上传到可访问地址
3. 平台注册：

```python
AppBuildPack(
    type="tgz",
    name="my-node-addon",
    address="https://.../my-node-addon.tgz",
    version="v1.2.3",
)
```

4. 构建时：

```
REQUIRED_BUILDPACKS=tgz my-node-addon ... v1.2.3
```

5. builder shim 下载并解压到 `/cnb/buildpacks`，lifecycle 使用

对比：

```
如果 my-node-addon 已经打进 builder 镜像：
REQUIRED_BUILDPACKS=oci-embedded my-node-addon ... v1.2.3
不需要下载
```

我们的本地方案：

```
buildpacks 已在 builder 的 /cnb/buildpacks
→ 等价于 oci-embedded
→ 不需要 BUILDPACK_TYPE
```

所以不吸收它，不是因为它是历史方案，而是因为我们当前没有“外部 buildpack 包”这个需求。

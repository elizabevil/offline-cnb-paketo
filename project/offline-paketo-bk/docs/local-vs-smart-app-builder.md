# 本地方案 vs 官方 smart-app-builder

> 对比对象：
> - 本地：`script/tools/bksmartpacker_v3` + `offline-paketo-bk`
> - 官方：blueking-paas `builder-stack` 的 `smart-app-builder`

## 1. 定位

| 项 | 本地 bksmartpacker | 官方 smart-app-builder |
|---|---|---|
| 语言 | Go | Go |
| 打包单位 | 模块 | 构建组 |
| 产物 | `<module>.tgz`（OCI layout） | `<group>.tar`（docker save） |
| Procfile | 每模块进程名 | v2 应用级 `<module>-<process>` |
| artifact.json | 仅复制源码中已有文件 | 主动生成 |
| 平台版本 | 兼容旧 CNB | v2 需 apiserver ≥1.7 |
| 默认打包版本 | 无 v1/v2 概念 | v2（可切 v1） |

## 2. 参数对比

### 本地 bksmartpacker

| 参数/环境变量 | 作用 |
|---|---|
| `--src` | 应用源码目录 |
| `--dest` | s-mart 输出路径 |
| `--builder-image` | CNB builder 镜像 |
| `--cache-registry` | 缓存仓库 |
| `--registry-auth` | 仓库凭证 |
| `--env-file` | dotenv 构建变量 |
| `--env KEY=VALUE` | 运行时覆盖 |
| `--module` | 只构建指定模块（调试） |
| `--inspect` | 包结构检查 |
| `BKS_CONTAINER_CLI` | podman/docker |
| `BKS_NETWORK` | 构建容器网络 |
| `BKS_EXTRA_MOUNTS` | 额外挂载 |

### 官方 smart-app-builder

| 参数/环境变量 | 作用 | 默认 |
|---|---|---|
| `SOURCE_GET_URL` | 源码位置（file/http） | 必填 |
| `DEST_PUT_URL` | 产物上传位置 | 必填 |
| `PACKAGING_VERSION` | v1/v2 | v2 |
| `BUILDER_SHIM_IMAGE` | CNB builder 镜像 | `bk-builder-heroku-bionic:v1.0.2` |
| `CNB_RUN_IMAGE` | run image | 配置 |
| `CNB_RUN_IMAGE_TAR` | run image tar | 配置 |
| `BASE_IMAGE_ID` | 运行时基础镜像 ID | `default` |
| `ARCHITECTURE` | 目标架构 | `amd64` |
| `BUILDPACK_TYPE` | `oci-embedded` / `tgz` | `oci-embedded` |
| `CACHE_REGISTRY` | 缓存仓库 | 空 |
| `REGISTRY_AUTH` | 仓库凭证 | 空 |
| `RUNTIME` | `pind` / `dind` | `pind` |
| `RUNTIME_WORKSPACE` | 工作目录 | `/podman/smart-app` |
| `DAEMON_SOCK` | 容器 daemon socket | 配置 |

额外固定环境：

- `PYTHON_BUILDPACK_VERSION`
- `PIP_VERSION`
- `PIP_INDEX_URL`
- `PIP_EXTRA_INDEX_URL`
- `NODEJS_BUILDPACK_VERSION`
- `NPM_REGISTRY`
- `GO_BUILDPACK_VERSION`
- `GO_BUCKET_URL`

## 3. 构建逻辑对比

### 本地

```
1. 解析 app_desc，校验 specVersion>=3
2. 可选按 --module 过滤
3. 每个模块：
   - 生成该模块 Procfile
   - 按语言选 paketo buildpacks
   - 合并 app_desc/env-file/--env
4. 每个模块单独 lifecycle build
5. layout 导出
6. 每个模块一个 <module>.tgz
7. 组装 s-mart
```

特点：

- 模块是构建单位
- 没有构建组
- Procfile 只有本模块进程
- 没有 artifact.json 生成

### 官方 v2

```
1. 解析 app_desc
2. 按 sourceDir + required buildpacks 分组
3. 生成应用级 Procfile：
   key = <module>-<process>
4. 每个构建组只构建一次
5. 保存一个 <group>.tar
6. 生成 artifact.json：
   - image_tar
   - proc_entrypoints
7. 组装 s-mart
```

特点：

- 构建组是构建单位
- 组内模块共享镜像
- 进程名全局唯一
- 平台通过 artifact.json 部署

## 4. Procfile 与进程

### 本地

```yaml
web: bin/start_web.sh
beat: python manage.py celery beat -l info
```

所有模块都用自己的进程名，因此：

- 同名 `web` 冲突
- 平台可能按进程名直接执行
- 缺少 `node-timeout` 时直接 `exec` 失败

### 官方 v2

```yaml
default-web: bin/start_web.sh
default-beat: python manage.py celery beat -l info
pipeline-cworker: python manage.py celery worker ...
pipeline-node-timeout: python manage.py node_timeout_process
```

artifact.json：

```json
"proc_entrypoints": {
  "web": ["default-web"],
  "cworker": ["pipeline-cworker"],
  "node-timeout": ["pipeline-node-timeout"]
}
```

平台用 `pipeline-node-timeout` 作为容器 command，不再直接使用 `node-timeout`。

## 5. 产物结构

### 本地

```
s-mart.tar.gz
├── .Version
├── app_desc.yaml
├── logo.png
├── default.tgz
├── callback.tgz
├── pipeline.tgz
└── api.tgz
```

- 每模块一个 OCI layout tar
- 无 artifact.json 时平台按旧 CNB 处理
- 包体积 549 MB（4 模块）

### 官方 v2

```
s-mart.tar.gz
├── .Version
├── app_desc.yaml
├── logo.png
├── artifact.json
└── default.tar
```

- 一个构建组一个 image tar
- 四模块共享 `default.tar`
- 包体积约 136 MB

## 6. 执行器差异

| 项 | 本地 | 官方 |
|---|---|---|
| 容器运行时 | podman/docker CLI | pind/dind |
| 构建入口 | 本地 `cnb lifecycle run` | cnb-builder-shim |
| 导出 | OCI layout | docker save |
| run image | `fake-cnb-runner` | 真实 `CNB_RUN_IMAGE` |
| 平台 API | `CNB_PLATFORM_API=0.11` | 同 |
| 缓存 | 本地目录 | `CACHE_REGISTRY` |
| 镜像 tag | `bk_sops/<module>:version` | `docker.io/local/<module>:latest` |

## 7. 平台处理差异

### 旧 CNB（无 artifact.json）

```
按模块找 <module>.tgz
解析 manifest/index 的第一个镜像
部署时 entrypoint = process type
```

问题：

- 多镜像 tar 只取第一个
- 同名进程冲突
- 进程缺失时直接 `exec` 进程名

### v2（有 artifact.json）

```
按模块读 image_tar
按进程读 proc_entrypoints
容器 command = <module>-<process>
```

优点：

- 明确镜像来源
- 明确进程入口
- 不依赖 manifest 顺序

## 8. 兼容性对比

| 能力 | 本地当前 | 官方 v2 |
|---|---|---|
| apiserver <1.7 | 可用 | 不可用 |
| apiserver ≥1.7 | 可用但进程有坑 | 推荐 |
| 多模块共享镜像 | D1/D6 手工 | 原生 |
| 同名进程 | 冲突 | 解决 |
| 包 ≤200MB | 需 D1 union | 原生 |
| 构建一次 | 否 | 是 |
| artifact 协议 | 无 | 有 |

## 9. 可复用与需替换

### 可以复用

- `app_desc` 解析
- buildpacks 与 buildpack 变量
- 本地 builder 镜像
- offline 缓存
- `--env-file` / `--env`
- 包检查工具（需扩展）

### 必须新增

- 构建组
- 应用级 Procfile
- 每组一个 image tar
- artifact.json 生成
- v2 包结构
- namespaced 进程验证

### 可以废弃

- D1 硬链接去重
- D6 多镜像单 tgz
- D1 union 脚本
- 按模块 tgz

## 10. 结论

本地 bksmartpacker 是“模块级构建器”：

```
模块 → 镜像 → tgz
```

官方 smart-app-builder v2 是“构建组级构建器”：

```
构建组 → 镜像 → image tar
        artifact.json 映射模块与进程
```

要满足当前平台和 200 MB 目标，应该：

1. 保留本地 buildpacks 和执行器
2. 引入构建组
3. 引入应用级 Procfile
4. 生成 artifact.json
5. 输出每组一个 image tar
6. 停止 D1/D6 方案

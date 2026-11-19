# offline-paketo-bk 需求

> 在 `offline-paketo`（纯 CNB 离线构建）基础上递进：**离线 + 缓存 + 只输出应用层**，
> 产物由 bksmartpacker 打包成 s-mart 交付给 BK 平台。

## 一、目标

1. **从零构建**：一条命令从空 bundle 产出 base / builder 两个镜像（run 镜像由 exporter 指定，本项目用 `scratch`）
2. **离线 + 缓存**：SDK 与依赖首次下载后落 `/layers/cache`，之后不再联网
3. **只输出应用层**：导出镜像的 run 镜像为 `scratch`，产物只含 app 层，
   由 bksmartpacker `docker image save` 后打成 s-mart

## 二、非目标

- 不实现 BK 平台部署/发布
- 不实现 s-mart 组装（由 bksmartpacker 负责）
- 不修改 bksmartpacker
- 不做多架构 manifest list（arm64 单独构建）

## 三、输入契约（bksmartpacker → builder）

| 变量 | 含义 |
|---|---|
| `SOURCE_GET_URL` | `file:///tmp/source.tgz` 源码包 |
| `REQUIRED_BUILDPACKS` | `"type name ... ver;..."`，限定参与的 buildpack |
| `OUTPUT_IMAGE` | 导出目标镜像 |
| `CNB_RUN_IMAGE` | 假 run 镜像（`FROM scratch`） |
| `USE_DOCKER_DAEMON` | 通过 daemon 导出 |
| `CACHE_IMAGE` / `CNB_REGISTRY_AUTH` | 可选缓存镜像与凭证 |
| `CNB_PLATFORM_API` | 默认 0.15 |

## 四、输出

```
s-mart.tar.gz
├── app_desc.yaml
├── .Version = "cnb-image-layers"
├── logo.png / artifact.json（若源码包内存在）
└── <module>.tgz        ← docker save 产物，只含应用层
```

验收：`<module>.tgz` 中**不含 OS 根层**。

## 四·五、SDK 不内置，只能挂载

Go / Node / Python 的 SDK **禁止打进 builder 镜像**（体积与升级成本），
只允许运行时挂载：

```bash
CNB_RUNTIMES=/opt/cnb-runtimes just smart
```

宿主机目录结构：

```
/opt/cnb-runtimes/
├── go/1.21.0/bin/go
├── node/20.20.2/bin/node
└── python/3.12.7/bin/python3
```

容器内挂到 `/cnb/runtimes`（只读），组件按
挂载 → `/layers/cache` → 镜像源 的优先级取用。

## 四·六、导出方式

导出方式由 `CNB_EXPORT_MODE` 决定（`cnb lifecycle run` 读取）：

| 取值 | 行为 | 适用 |
|---|---|---|
| `layout`（BK 默认） | 写 OCI layout，再打包为层归档（`CNB_LAYER_ARCHIVE`） | bksmartpacker 直接取用 |
| `daemon` | 通过容器运行时导出 | 仅真 dockerd（`BKS_EXPORT_MODE=daemon`） |
| `none` | 只构建不导出 | 仅验证 detect/build |

BK 默认路径不需要 daemon：bksmartpacker 挂载 `<workspace>/layers` 到 `/cnb-layer`，
cnb 把 `<module>.tgz` 写进去，构建后由 bksmartpacker 复制进 s-mart。

## 五、流水线

配置只有两条 stage（`pipeline.toml`）：

```
base   → podman build builder-base.dockerfile
builder→ podman build builder-build.dockerfile（lifecycle + buildpack）
```

应用构建不在 stage 里：由 `just smart` 调用 bksmartpacker，
在容器内跑一次 detect → build → analyze → export，再打包成 s-mart。

## 六、目录结构

```
project/offline-paketo-bk/
├── pipeline.toml               # 入口：cnb pipeline run
├── justfile                    # build / plan / fetch / verify / smart / pack-case
├── fixtures/                   # BK 结构测试项目
│   ├── single/{app_desc.yaml, logo.png, go.mod, main.go}
│   └── multi/{app_desc.yaml, web/, worker/}
├── docker/
│   ├── builder-base.dockerfile
│   ├── builder-build.dockerfile
│   ├── lifecycle-entrypoint.sh # 四阶段 + bksmartpacker 契约
│   ├── order.toml
│   └── apt-sources/
├── dist/<platform>/            # s-mart 产物（不入库）
└── docs/
```

自研 buildpack 不在本项目内：位于仓库级 `buildpacks/paketo/`，
通过 `--build-context buildpacks=…` 引用。

## 七、验收标准

1. `just build` 从空 bundle 产出 base / builder 两个镜像
2. 二次构建出现 cache hit，不再下载
3. `just smart` 产出 `dist/<platform>/s-mart.tar.gz`，含 `app_desc.yaml` 与 `.Version=cnb-image-layers`
4. `<module>.tgz` 只含应用层
5. `fixtures/single` 与 `fixtures/multi` 均可构建
6. `cnb materials verify` / `cnb pipeline profile validate` / `cnb buildpack lint` 全绿

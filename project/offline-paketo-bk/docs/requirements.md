# offline-paketo-bk 需求

> 在 `offline-paketo`（纯 CNB 离线构建）基础上递进：**离线 + 缓存 + 只输出应用层**，
> 产物由 bksmartpacker 打包成 s-mart 交付给 BK 平台。

## 一、目标

1. **从零构建**：一条命令从空 bundle 产出 base / run / builder 三个镜像
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
s-mart.tgz
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

## 五、流水线

```
prepare(app_desc.yaml → 模块) → fetch(物料) → build(每模块四阶段) → package(bksmartpacker)
```

## 六、目录结构

```
project/offline-paketo-bk/
├── build.sh                    # 入口：cnb run <profile>
├── justfile                    # build / plan / fetch / verify
├── fixtures/                   # BK 结构测试项目（新增）
│   ├── single/{app_desc.yaml, go.mod, main.go}
│   └── multi/{app_desc.yaml, web/, worker/}
├── docker/
│   ├── builder-base.dockerfile
│   ├── builder-build.dockerfile
│   ├── run.dockerfile
│   ├── lifecycle-entrypoint.sh # 四阶段 + bksmartpacker 契约
│   ├── order.toml
│   ├── apt-sources/
│   └── buildpacks/             # 11 个自研离线组件
└── docs/
    ├── architecture.md
    ├── bk-build-logic.md
    ├── requirements.md         # 本文件
    └── delta-from-paketo.md    # 与 offline-paketo 的差异
```

## 七、验收标准

1. `./build.sh` 从空 bundle 产出三个镜像
2. 二次构建出现 cache hit，不再下载
3. bksmartpacker 产出 `s-mart.tgz`，含 `app_desc.yaml` 与 `.Version=cnb-image-layers`
4. `<module>.tgz` 只含应用层
5. `fixtures/single` 与 `fixtures/multi` 均可构建
6. `cnb doctor` / `dockerfile check` / `buildpack lint` 全绿

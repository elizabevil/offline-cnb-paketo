# bksmartpacker_v3：fork 与上游对比

用于回答两个问题：

- 我们和官方差在哪
- 能不能直接互换

维护方式与升级步骤见 [fork.md](fork.md)。

## 一、基线

基线（仓库、提交、来源）见 [fork.md](fork.md)。

对比范围：

```text
运行方式、导出、平台、打包、自检
```

## 二、功能矩阵

| 能力 | 上游 | fork |
|---|---|---|
| 容器运行时 | 硬编码 `docker` | `BKS_CONTAINER_CLI`，缺省探测 docker/podman |
| socket | 固定 `/var/run/docker.sock` | `BKS_DOCKER_SOCK` 可覆盖 |
| 导出方式 | 仅 daemon | layout（默认）/ daemon |
| 层包回传 | `docker image save` | layout 直接写文件；daemon 仍用 save |
| fake runner | `FROM scratch` 构建 | `image import` 构造单层镜像 |
| 多架构 fake runner | 固定 tag | 按平台打 tag |
| 目标平台 | 不支持 | `BKS_PLATFORM=linux/arm64` |
| run image | 依赖 daemon | 可文件挂载 |
| 额外挂载/网络 | 无 | `BKS_EXTRA_MOUNTS`、`BKS_NETWORK` |
| buildpack 别名 | 无 | `BKS_BUILDPACK_ALIASES` |
| 打包元数据 | archiver 默认 | 属主/时间/权限归一 |
| 包自检 | 无 | `--inspect`（text/json） |
| 默认 dest | `./s-mart.tar.gz` | `./s-mart.tar.gz`（一致） |

## 三、环境变量对照

| fork | 上游对应 |
|---|---|
| `BKS_CONTAINER_CLI` | 无（固定 docker） |
| `BKS_DOCKER_SOCK` | 无（固定路径） |
| `BKS_EXPORT_MODE` | 无（固定 daemon） |
| `BKS_PLATFORM` | 无 |
| `BKS_RUN_IMAGE_ARCHIVE_FILE` | 无 |
| `BKS_EXTRA_MOUNTS` | 无 |
| `BKS_NETWORK` | 无 |
| `BKS_BUILDPACK_ALIASES` | 无 |

## 四、输出对比

| 项 | 上游 | fork |
|---|---|---|
| `.Version` | `cnb-image-layers` | 相同 |
| 包内容 | app_desc / artifact / logo / 模块 tgz | 相同 |
| tar 属主 | 宿主用户 | `0:0` |
| tar 时间 | 文件 mtime | 固定值 |
| 校验文件 | 无 | 外部脚本生成 `.sha256` |

## 五、兼容性结论

| 场景 | 结论 |
|---|---|
| 用 fork 替换上游 | 参数与包协议兼容，默认 dest 一致 |
| 上游调用 fork | 不需要改参数 |
| fork 调用上游 | 不能用 layout/podman/arm64/`--inspect` |
| 蓝鲸平台消费产物 | 两者均可，fork 已验证 amd64/arm64 部署 |

## 六、升级时重新对比的项

1. 上游是否新增了导出方式或平台参数
2. fake runner 的构造方式是否变化
3. `image save` 参数是否变化
4. 默认 dest、参数名是否变化
5. 包结构与 `.Version` 是否变化

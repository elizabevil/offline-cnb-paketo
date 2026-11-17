# BK 导出：layout 预置方案

> 目标：在无 daemon、无 podman 兼容 API 的前提下，用 lifecycle exporter 的
> `-layout` 模式产出 OCI layout，供 bksmartpacker 组装 s-mart。

## 一、背景与结论

| 路径 | 结果 |
|---|---|
| exporter `-daemon` | podman docker 兼容 API 单连接限制 → `singleton connection already in use` |
| exporter `-layout`（当前） | run 镜像引用解析失败：analyzer 写的是镜像 ID，不是 layout 引用 |
| **layout + 预置 run 镜像** | 实验证实可行（见下） |

实验（宿主直接跑 `bundle/.../lifecycle/0.21.29/analyzer`）：

```
-run-image "/abs/path@sha256:…"        → could not parse reference
-run-image "./oci@sha256:…"            → Image with name "<dir>/oci/oci/sha256/…" not found
-run-image "run@sha256:<digest>"       → 查找 <dir>/index.docker.io/library/run/sha256/<digest>
按上述路径放好标准 OCI layout 后      → run 镜像解析成功，仅 app 镜像 not found（首次构建正常）
```

结论：layout 目录里每个镜像是一个**标准 OCI layout 目录**，按归一化引用名分层：

```
<layout-dir>/
└── index.docker.io/library/<repo>/
    ├── latest/                 # 按 tag 引用
    └── sha256/<digest>/        # 按 digest 引用
        ├── index.json
        ├── oci-layout
        └── blobs/sha256/...
```

## 二、目标流程

```
容器启动（bksmartpacker 注入）
  CNB_EXPORT_MODE=layout
  CNB_RUN_IMAGE=fake-cnb-runner:latest
  CNB_RUN_IMAGE_ARCHIVE=/cnb-run/run-image.tar
  CNB_LAYER_ARCHIVE=/cnb-layer/<module>.tgz
        ↓
cnb lifecycle run
  1. prepare：把 run 镜像写入 layout 目录（若尚未存在）
     a. 解包 archive（plain tar）到临时目录
     b. imagefmt.Detect → Read（docker-archive / oci-layout）
     c. 计算 config digest
     d. imagefmt.Write(oci-layout, <layout>/index.docker.io/library/<repo>/sha256/<digest>)
  2. detect → build
  3. analyze：-layout -layout-dir -run-image <repo>@sha256:<digest>
  4. export：同上，输出到 layout 目录
  5. pack：layout 目录 → CNB_LAYER_ARCHIVE（OCI archive）
        ↓
bksmartpacker：发现 layers/<module>.tgz → 直接作为 <module>.tgz
```

## 三、改动清单

| 位置 | 改动 |
|---|---|
| `internal/spec/imagefmt` | `Image` 增加 `ConfigDigest`；docker-archive/OCI 读取时填充 |
| `internal/archive` | 增加 `ExtractTar`（plain tar；现有只有 tar.gz/zip） |
| `internal/lifecycle/layout.go`（新增） | `PrepareRunImage(layoutDir, runRef, archive, log) (identifier string, err error)`：解包、探测、写 layout、返回 `<repo>@sha256:<digest>` |
| `internal/lifecycle/phases.go` | layout 模式下：Run 前调用 PrepareRunImage；把返回的 identifier 传给 analyzer/exporter |
| `internal/lifecycle/args.go` | layout 模式的 `-run-image` 使用 identifier，而不是 `CNB_RUN_IMAGE` |
| `internal/lifecycle/config.go` | Export 增加运行期字段 `RunImageID string`（由 Prepare 填充，不来自环境） |
| bksmartpacker | 保持注入 `CNB_EXPORT_MODE=layout`、`CNB_RUN_IMAGE_ARCHIVE`、`CNB_LAYER_ARCHIVE`；无需改动 |

## 四、引用归一化规则

输入：`CNB_RUN_IMAGE`（如 `fake-cnb-runner:latest`、`localhost:5000/team/run:1.0`）

| 输入形态 | 归一化路径 |
|---|---|
| `<name>:<tag>` | `index.docker.io/library/<name>/<tag>`（含 registry 时保留 registry） |
| `<name>@sha256:<digest>` | `index.docker.io/library/<name>/sha256/<digest>` |
| 无 tag 无 digest | 按 `latest` 处理 |
| `<registry>/<path>:<tag>` | `<registry>/<path>/<tag>` |

实现要求：

- 用 `strings.Cut` 解析，不写正则
- 归一化函数独立可测：`NormalizeRef(ref) (path, digest string, err error)`
- 非法引用（空、只有 registry）→ typed error

## 五、异常处理

| 场景 | 行为 |
|---|---|
| `CNB_RUN_IMAGE_ARCHIVE` 为空且 layout 目录中无镜像 | 报错：`run image archive required for layout export` |
| archive 不是 tar / 无法识别格式 | 报错并列出支持格式（docker-archive / oci-layout） |
| layout 目录不可写 | 报错并打印路径与权限 |
| 目标路径已存在旧镜像 | 先 `os.RemoveAll` 再写，避免残留层 |
| run 引用非法 | 报错：`invalid run image reference "<ref>": <reason>` |
| exporter 仍失败 | 打印已注入的 `-run-image` 值与 layout 目录树（两级），便于定位 |
| 打包时 layout 目录为空 | 报错：`layout export produced no image` |

所有错误走 `errs.*`，错误信息英文。

## 六、验证方案

### 单元测试

| 用例 | 断言 |
|---|---|
| ref 归一化（tag / digest / registry / 无 tag） | 路径正确、非法输入报错 |
| config digest 计算 | 与 blob 文件名一致 |
| PrepareRunImage（用合成 docker-archive） | 目标目录出现 index.json/oci-layout/blobs，返回 identifier 正确 |
| 重复准备同一镜像 | 幂等（覆盖写，不残留） |

### 本地集成（宿主直接跑离线二进制）

```
1. 用 imagefmt 造 run 镜像（已完成）
2. PrepareRunImage 写入 layout
3. analyzer -layout -run-image <identifier> → 成功（run 镜像解析）
4. exporter -layout 使用最小 layers（launch.toml + 一层）→ 输出 OCI layout
```

### 容器验证

```bash
cd project/offline-paketo-bk
just smart /home/debi/Desktop/cmdb-saas/release-amd64
# 期望日志：prepare run image → export -layout → pack layout
# 产物检查：
tar -tzf dist/s-mart.tar.gz | head
tar -xzOf dist/s-mart.tar.gz web.tgz | tar -tf - | grep -E '^(oci-layout|index.json|blobs/)$'
```

## 七、回退方案

若 layout 预置仍失败（例如 analyzer 对 app 镜像缺失的处理与预期不同）：

- 回退到方案 B：容器内 `podman load` run 镜像 → `podman build`（`FROM run; COPY . /app`）→ `podman save --format oci-archive`
- B 需要自行写 `Entrypoint` / `CNB_*` / buildpack 标签，成本更高，但完全绕开 lifecycle exporter

## 八、提交计划

1. `feat(imagefmt): 暴露 ConfigDigest`
2. `feat(archive): ExtractTar`
3. `feat(lifecycle): PrepareRunImage + 引用归一化`（含单测）
4. `refactor(lifecycle): layout 模式使用预置镜像的 identifier`
5. `docs: 本方案`

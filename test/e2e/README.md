# e2e 测试：两种路径

| 路径 | 入口 | 依赖 | 验证什么 |
|---|---|---|---|
| lifecycle 直连 | `just test-e2e-core[-all]` | 容器运行时 | buildpack 行为（能否产出可运行镜像） |
| pack | `just test-e2e[-all]` | docker 或 rootful podman | 平台兼容性（pack 能否使用我们的 builder） |
| 自动选路 | `just test-e2e-auto <name>` | — | 按运行时自动决定上面两种 |

rootless podman 下 **pack 不可用**：pack 的 lifecycle 容器需要访问宿主 socket，
而 userns 映射会把容器内 uid 变成从属 uid，导致 permission denied。
因此 `e2e_strategy()` 在 podman 场景固定走 lifecycle 直连。

---

## lifecycle 各阶段的**精确**调用

> 这些参数是实测得到的（`lifecycle 0.21.13`），不要凭印象改。

```bash
# 必需环境变量（所有阶段）
CNB_PLATFORM_API=0.15

# 1. detect
lifecycle/detector  -layers=/layers -app=/workspace -platform=/platform

# 2. build
lifecycle/builder   -layers=/layers -app=/workspace -platform=/platform

# 3. analyze（注意：没有 -app；必须有 1 个位置参数=镜像名）
#    Go flag 解析遇到第一个位置参数即停止 → 镜像必须放最后
lifecycle/analyzer  -layers=/layers -daemon -run-image <run-image> <image>

# 4. export（位置参数=镜像名，可多个）
#    注意：不接受 -run-image，run 镜像来自 analyzer 写出的 analyzed.toml
lifecycle/exporter  -layers=/layers -app=/workspace -daemon <image>
```

### 实测证据（用宿主上的 lifecycle 二进制直接跑）

```
analyzer -run-image foo:latest bar:latest   → 通过输入解析（后续报 chown，仅因本地路径）
analyzer  无 -run-image                    → "-run-image is required when there is no run metadata"
exporter -run-image foo:latest bar:latest   → "-run-image is unsupported"
```

结论：
- **analyzer 必须**给 run 镜像（`-run-image`，或提供 `-run` 指向 run.toml）
- **exporter 不能**给 `-run-image`，它从 analyzer 写出的 `analyzed.toml` 读

### 参数速查

| 阶段 | `-app` | `-daemon` | 位置参数 |
|---|---|---|---|
| detector | ✅ | — | 无 |
| builder | ✅ | — | 无 |
| analyzer | ❌ | ✅（读本地 run 镜像） | 1 个 image |
| exporter | ✅ | ✅（写回本地 daemon） | ≥1 个 image（**无 -run-image**） |

### 典型报错对照

| 报错 | 原因 |
|---|---|
| `please set 'CNB_PLATFORM_API'` | 缺环境变量 |
| 打印 usage 后 exit 2 | 传了该阶段不支持的 flag（如 analyzer 的 `-app`） |
| `received 0 arguments, but expected 1` | analyzer 缺 image |
| `unconsumed flag-like positional arg` | 镜像放在了 flag 之前（Go flag 解析遇位置参数即停） |
| `at least one image argument is required` | exporter 缺 image |
| `permission denied ... docker.sock` | rootless podman 的 userns 映射问题 → 加 `--userns=keep-id` |

---

## 容器挂载约定

| 容器路径 | 宿主目录 | 持久化 | 说明 |
|---|---|---|---|
| `/workspace` | `<app>` | — | 只读，应用源码 |
| `/layers` | `test/.work/<case>/layers` | ❌ 每次全新 | lifecycle 要求干净，否则报 `rename ... file exists` |
| `/layers/cache` | `test/.work/<case>/cache` | ✅ 跨构建 | **唯一需要保留的目录**（Node/Python 等下载产物） |
| `/platform` | `test/.work/<case>/platform` | ❌ 每次全新 | 平台元数据 |
| `/cnb/runtimes` | `bundle/runtimes`（可用 `CNB_RUNTIMES` 覆盖） | 只读 | 本地 Node/Python 运行时，完全离线用 |

```bash
podman run --rm --userns=keep-id   -v <app>:/workspace:ro   -v <work>/layers:/layers   -v <work>/cache:/layers/cache   -v <work>/platform:/platform   -v bundle/runtimes:/cnb/runtimes:ro   -v $XDG_RUNTIME_DIR/podman/podman.sock:/var/run/docker.sock   -e DOCKER_HOST=unix:///var/run/docker.sock   -e CNB_PLATFORM_API=0.15   --entrypoint /cnb/lifecycle/<phase> offline-builder:latest <args>
```

要点：

- `--userns=keep-id`：容器内 uid 与宿主一致，否则挂载的 socket 不可访问
- `/layers` 与 `/layers/cache` 必须分开：前者每次重建，后者跨构建保留
- `/cnb/runtimes` 只读挂载，镜像里不打包运行时（体积考虑）

## 日志

每步输出在 `test/.logs/<case>-<phase>.log`，失败时自动打印末尾 20 行。
实时查看：

```bash
VERBOSE=1 just test-e2e-core go-app
```

## 参数化（不再绑定单个项目）

| 变量 | 默认 | 作用 |
|---|---|---|
| `E2E_PROFILE` | `config/projects/offline-paketo.toml` | 被测流水线配置（builder/run 镜像由 `pipeline plan` 推导） |
| `E2E_FIXTURES` | `test/fixtures` | 被测夹具目录 |
| `E2E_SKIP_BUILD` | `0` | 跳过重建设镜像 |
| `E2E_ONLY` | 全部阶段 | 只跑指定阶段（复用 layers） |
| `E2E_FRESH` | `0` | 清空 cache 后重跑 |

示例：用 bk 项目与它的夹具跑同一套用例

```bash
E2E_PROFILE=config/projects/offline-paketo-bk.toml \
E2E_FIXTURES=project/offline-paketo-bk/fixtures \
just test-e2e-core-all
```

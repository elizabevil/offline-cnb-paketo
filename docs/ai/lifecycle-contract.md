# 与 lifecycle 交互的硬性约束

> 适用于 `script/tools/cnb/internal/lifecycle` 与所有调用 lifecycle 的脚本。

## 一、禁止靠猜测确定契约

lifecycle 的输入文件格式、可用参数、错误语义都以**二进制本身**为准。
改动交互前必须先提取：

```bash
# 1) 参数（哪些 flag 存在、默认值是什么）
CNB_PLATFORM_API=0.15 bundle/<arch>/binaries/lifecycle/<ver>/exporter -h

# 2) 结构体字段（TOML 键名）
strings bundle/<arch>/binaries/lifecycle/<ver>/lifecycle | grep 'toml:"'

# 3) 关键错误语义
strings bundle/<arch>/binaries/lifecycle/<ver>/lifecycle | grep -i 'required\|missing'
```

拿不到证据就不要写代码——"大概是这个字段"是本仓库明确禁止的做法。

## 二、已确认的契约（lifecycle 0.21.29）

### 2.1 exporter 参数

| 参数 | 说明 | 是否必填 |
|---|---|---|
| `-layers` | 层目录 | 是 |
| `-app` | 应用目录 | 是 |
| `-layout` | 导出为 OCI layout | layout 模式必填 |
| `-layout-dir` | layout 输出目录 | layout 模式必填 |
| `-run-image` | run 镜像引用 | 无 run 元数据时必填 |
| `-run` | run.toml 路径 | 可选（默认 `/cnb/run.toml`） |
| `-daemon` | 导出到 daemon | 与 `-layout` 互斥 |

### 2.2 analyzed.toml

```toml
[run-image]
  reference = "<name>@sha256:<manifest-digest>"
  top-layer = "sha256:<最后一层 digest>"
```

字段名来自二进制标签：`run-image` / `reference` / `top-layer`。

### 2.3 相关错误

| 错误 | 触发条件 |
|---|---|
| `No run metadata found at path "/cnb/run.toml"` | 没有 run.toml，且未提供 `-run-image` |
| `missing run image metadata (-run-image)` | 同上 |
| `get run image top layer SHA: image has no layers` | analyzed 的 top-layer 为空，或 layout 中镜像零层 |

## 三、本仓库的落地方式

| 需求 | 实现 |
|---|---|
| 不依赖 daemon 写产物 | exporter 用 `-layout` |
| 不依赖 daemon 读 run image | 平台侧把 run image 导出成 docker-archive 并挂载，cnb 转 OCI layout |
| analyzed.toml | cnb 直接由磁盘 layout 生成，跳过 analyzer |
| Platform API < 0.12 | layout 参数不存在，必须抬到 0.12 |
| 实验特性 | layout 仍需 `CNB_EXPERIMENTAL_MODE=warn` |

## 四、检查清单（改动前）

- [ ] 用 `-h` 确认要用的 flag 在该版本存在
- [ ] 用 `strings | grep 'toml:"'` 确认写入的字段名
- [ ] 用错误字符串确认失败分支的语义
- [ ] 确认镜像有层（`image has no layers` 的根因）
- [ ] 确认 Platform API 下限满足

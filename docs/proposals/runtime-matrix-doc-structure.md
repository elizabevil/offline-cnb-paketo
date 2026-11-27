# Runtime 测试矩阵文档结构方案

> 状态：待实施。
> 本文只定义文档结构、表格 schema 和写作约束，不修改现有 runtime-matrix.md。

## 1. 目标

把 runtime 测试矩阵改成可直接作为测试依据的文档：

```text
一行 = 一个可执行测试组合
单元格 = 单值
状态 = 可枚举
产物 = 可定位
原因 = 可审计
```

## 2. 文档结构

```text
# Runtime 测试矩阵

1. 范围
2. 术语
3. 维度与命名
4. Host 测试矩阵
5. Managed 测试矩阵
6. 产物清单
7. 未完成项
8. singleton
9. 执行命令
```

## 3. 各章节内容

### 3.1 范围

只写：

```text
测试环境
目标平台
产物根目录
配置来源
```

不写背景故事。

### 3.2 术语

固定术语表：

| 术语 | 定义 |
|---|---|
| Outer | 宿主机上启动 builder/sidecar 的运行时 |
| Inner | builder 通过 socket 连接的运行时 |
| Builder | builder 容器内进程用户 |
| sidecar | 单次构建启动的辅助容器，提供 inner runtime |
| host-docker | outer/inner 都是宿主 Docker，无 sidecar |
| host-podman | outer/inner 都是宿主 Podman，无 sidecar |
| pind | inner 是 sidecar Podman |
| dind | inner 是 sidecar Docker |

每个术语只出现一次定义。

### 3.3 维度与命名

Host 维度：

```text
Platform
Host root
Builder
Export
```

Host 下：

```text
outer = inner
```

因此不单列 Inner root。

Managed 维度：

```text
Platform
Outer root
Inner root
Builder
Export
```

命名：

```text
H-<runtime>-<platform>-<host-root>-<builder>-<export>
M-<runtime>-<platform>-<outer-root>-<inner-root>-<builder>-<export>
```

示例：

```text
H-HOSTDOCKER-AMD64-ROOT-ROOT-DAEMON
M-PIND-ARM64-ROOTLESS-ROOTLESS-NONROOT-LAYOUT
```

### 3.4 Host 测试矩阵

列：

```text
ID
Runtime
Platform
Host root
Builder
Host mapping
Export
Status
Artifact
Evidence
```

### 3.5 Managed 测试矩阵

列：

```text
ID
Runtime
Platform
Outer root
Inner root
Builder
Host mapping
Export
Status
Artifact
Evidence
```

### 3.6 产物清单

列：

```text
ID
Artifact
Archive format
SHA256
```

测试矩阵的 Artifact 只放一个文件名，不塞多个产物。

### 3.7 未完成项

列：

```text
ID
Platform
组合
Status
Reason
```

### 3.8 singleton

只写：

```text
触发条件
代码路径
已观察结果
```

## 4. 内容约束

以下规则为强制约束。

### 4.1 单元格单值

禁止：

```text
amd64/arm64
layout+daemon
root/non-root
root / nonroot
```

必须拆行或拆列。

### 4.2 Platform

只允许：

```text
amd64
arm64
```

两个平台都测，必须是两行，不能合并。

### 4.3 Export

只允许：

```text
layout
daemon
```

不能写：

```text
layout+daemon
both
```

同一 Runtime 两种 export 都测，必须是两行。

### 4.4 Root

Outer root / Inner root 只允许：

```text
root
non-root
```

Builder 只允许：

```text
root
non-root
```

`rootless 容器内 uid=0` 不写 `root (userns)`，统一写：

```text
Builder = root
Host mapping = debi
```

### 4.5 Status

只允许：

```text
pass
fail
blocked
not-run
```

不得使用：

```text
已验证
部分验证
条件触发
暂缓
推测可行
```

这些只能出现在 Reason 或说明中。

### 4.6 Evidence

Evidence 只写可复查信息：

```text
产物文件名
日志路径
命令
上游 issue
```

不写“之前说”“历史写法”“不再使用”等叙述。

### 4.7 术语

禁止新增：

```text
host dind
managed pind daemon
root (userns)
非root可测
```

已有概念必须使用术语表中的名称。

### 4.8 排序

Host 表：

```text
Runtime → Platform → Host root → Builder → Export
```

Managed 表：

```text
Runtime → Platform → Outer root → Inner root → Builder → Export
```

ID 必须随排序重新生成，不允许保留旧编号。

### 4.9 不重复解释

同一事实只写一处：

```text
定义 → 术语表
测试结果 → 测试矩阵
产物 → 产物清单
未完成原因 → 未完成项
singleton → singleton 节
```

### 4.10 不写元评论

禁止：

```text
术语归并
不是改名掩盖
历史写法
之前没有复现
这段只是说明
```

只写事实和结论。

## 5. 验收

```text
rg 'amd64/arm64|layout\+daemon|root/non-root|userns|host dind' docs/design/components/bksmartpacker/runtime-matrix.md
→ 无结果
```

```text
rg '已验证|部分验证|条件触发|暂缓|推测可行' docs/design/components/bksmartpacker/runtime-matrix.md
→ 无结果
```

表头列数与分隔行列数一致。

每个 Artifact 文件名唯一。

每个未完成项都有 Reason。

## 6. 实施顺序

```text
1. 冻结现有 runtime-matrix.md
2. 按本结构重建文档
3. 运行验收检查
4. 删除旧段落，不保留兼容说明
```

## 7. 非目标

本文不讨论：

```text
新的 runtime 实现
新的测试用例
rootless Docker 安装步骤
lifecycle issue 修复
```

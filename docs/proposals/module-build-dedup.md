# 模块重复构建与产物去重（待办）

> 状态：**提案**。先记录现状逻辑与优化方向，暂不改代码。

## 一、现状逻辑

入口：

```
just smart → bksmartpacker --src <app> [--module ...] --dest s-mart.tar.gz
```

流程：

```
loadAppDesc
  → 校验 specVersion >= 3
  → params.Modules 非空时过滤 modules（未命中报错）
  → 逐模块生成 ModuleBuildStep：
      sourceDir + Procfile（该模块全部 processes）
      language → paketo-buildpacks/{python,nodejs,go}
      env = app_desc < --env-file < --env
  → 逐模块 podman run builder：
      独立 lifecycle build
      导出 OCI layout
      复制为 workspace/<module>.tgz
  → 组装 s-mart：
      .Version、完整 app_desc.yaml、logo、各模块 tgz
```

关键点：

- 模块是构建单元：一个模块一个镜像
- Procfile 是模块级：包含该模块全部启动项
- `--module` 只过滤构建范围，不合并模块
- `app_desc.yaml` 始终写原始完整内容

## 二、问题

SOPS 的 4 个模块：

```
default  sourceDir=./src  Python  3 processes
callback sourceDir=./src  Python  1 process
pipeline sourceDir=./src  Python  13 processes
api      sourceDir=./src  Python  1 process
```

源码相同，但被构建 4 次，产出 4 个独立镜像。

实测（旧产物）：

| 指标 | 数值 |
|---|---|
| 模块数 | 4 |
| 每模块层数 | 8 |
| 层引用总数 | 32 |
| 唯一层数 | 17 |
| 每模块镜像大小 | 约 129.5 MB |
| 唯一层总大小 | 约 412.6 MB |
| 重复层引用 | 7 层跨模块共享 |

结论：

- **构建时间**约 4 倍
- **包体积**存在明显重复
- 7 个基础层完全相同，只是被复制进 4 个 tgz

## 三、优化方向

| 方案 | 做法 | 构建时间 | 包体积 | 运行时改动 | 风险 |
|---|---|---|---|---|---|
| A. 合并模块 | 只保留一个模块 | 1x | 1x | 大 | 改变部署语义 |
| B. 构建一次 + 每模块 Procfile 覆盖 | 共享基础镜像，追加小层改 Procfile | 1x | 接近 1x | 小 | 需要镜像改写能力 |
| C. 构建一次 + 同镜像多 tag | 用并集 Procfile，模块指向同一镜像 | 1x | 1x | 中 | 进程并集/环境差异 |
| D. s-mart 层去重 | 共享 OCI blob，多 manifest | 1x | 1x | 需协议支持 | 运行时需识别共享布局 |
| E. 仅构建缓存复用 | 保持 4 镜像，复用层缓存 | 接近 1x | 不变 | 无 | 收益有限 |

### A. 合并模块

不推荐：

- 模块对应部署单元
- 合并后无法按模块独立发布/扩缩
- `app_desc` 语义被破坏

### B. 构建一次 + Procfile 覆盖

思路：

```
build 一次 base 镜像（源码/依赖相同）
  ↓
模块级差异只有 Procfile
  ↓
为每个模块生成一个薄层（替换 /app/Procfile）
  ↓
输出 4 个镜像，共享 base 层
```

优点：

- 保留模块语义
- 构建次数从 4 降到 1
- 镜像层天然共享

需要解决：

1. 如何在不重新跑 lifecycle 的情况下生成镜像
2. 如何复用 base 层并追加 Procfile 层
3. s-mart 仍按模块输出 tgz，包内是否共享 blob

### C. 同镜像多 tag

思路：

```
合并所有模块的 Procfile 为一个并集
构建一次
4 个模块 tag 指向同一镜像
```

限制：

- 模块构建环境变量不同（pipeline 多 1 个）
- 进程名冲突/暴露多余进程
- 运行时按模块选择进程的行为需确认

只适合“模块仅进程不同、构建环境完全一致”的场景。

### D. s-mart 层去重

思路：

```
s-mart/
  blobs/sha256/...        # 唯一层只存一份
  manifests/default.json
  manifests/callback.json
  manifests/pipeline.json
  manifests/api.json
```

优点：

- 包体积接近唯一层总量
- 多模块共享基础层

限制：

- 需要修改 s-mart 协议或运行时解包逻辑
- BK 侧是否接受共享布局未知

## 四、建议路线

分两步：

1. **短期（不改协议）**
   - 识别“源码/语言/构建环境相同”的模块组
   - 组内只构建一次
   - 通过 Procfile 覆盖生成模块镜像（方案 B）
   - 包内仍是独立 tgz，先接受体积重复

2. **长期（改协议）**
   - s-mart 引入共享 blob 布局（方案 D）
   - 与运行时确认兼容性后再实施

不建议：

- 合并模块（A）
- 无差别同镜像多 tag（C），除非确认模块环境一致

## 五、判定“可复用构建”的条件

必须同时满足：

```
sourceDir 相同
language 相同
buildpacks 相同
构建期 env 相同（template.env 中影响构建的键）
```

进程列表可以不同，因为最终由 Procfile 层区分。

运行期 env 不同不影响镜像复用。

## 六、验证计划

1. 单模块构建产物与复用构建产物逐层比对
2. Procfile 覆盖后进程列表正确
3. s-mart 解包后各模块镜像可启动
4. 包体积、构建时间对比
5. arm64 行为一致

## 七、待办

- [ ] 统计当前模块构建耗时占比
- [ ] 确认 BK 运行时是否允许模块镜像共享层
- [ ] 评估 Procfile 覆盖的实现方式（OCI 层追加/镜像改写）
- [ ] 设计 s-mart 共享 blob 协议（如采用 D）
- [ ] 在 bksmartpacker 中增加模块复用分组
- [ ] 回归 `--module` 与全量构建

## 八、不做

- 不改变“模块 = 一个镜像”的语义
- 不在应用源码里合并模块
- 不为了省体积牺牲模块独立部署能力

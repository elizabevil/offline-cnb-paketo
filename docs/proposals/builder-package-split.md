# pkg/builder 拆包职责清单

> 状态：方案讨论，未实施。

## 一、当前职责

| 文件 | 职责 | 输入 | 输出 | 副作用 |
|---|---|---|---|---|
| `prepare.go` | 构建计划编排 | Params | BuildPlan | 无（只读 app_desc/logo） |
| `appdesc.go` | app_desc 读取、校验、模块过滤、计划初始化 | 源码目录 | app_desc + BuildPlan | 读取/复制 app_desc.yml |
| `plan.go` | BuildPlan/Step/Group/Process 模型与格式化方法 | — | 数据结构 | 无 |
| `envfile.go` | dotenv 解析 | 文件 | map | 读取文件 |
| `env.go` | app_desc/env-file/CLI 合并 | 多来源 env | map | 无 |
| `buildpacks.go` | 语言到 buildpack 映射 | ModuleDescription | []Buildpack | 无 |
| `procfile.go` | 模块级/应用级 Procfile | modules | YAML 字符串 | 无 |
| `groups.go` | 按 sourceDir + buildpacks 分组 | modules + Steps | []Group | 无 |
| `artifact.go` | BuildPlan → artifact.json | BuildPlan | JSON | 无 |
| `source.go` | 源码 tar.gz | 源码目录 | tar.gz | 写源码 Procfile、写 tar |
| `constants.go` | 打包版本、buildpack 类型、默认 base image | — | 常量 | 无 |

## 二、依赖关系

```text
types
  ↑
appdesc / buildpacks / procfile / env
  ↑
plan
  ↑
groups / artifact
  ↑
prepare
  ↑
executors
```

注意：

- `BuildPlan` 是中心数据结构，拆包后必须放在最底层领域包，否则会循环依赖。
- `artifact` 依赖 `plan`，不能再让 `plan` 依赖 artifact。
- `source` 不依赖 plan，可独立。
- `env` 已经有 `pkg/env` 校验包，但 env 合并仍依赖 `types`。

## 三、候选包

### A. 最小拆分（推荐先做）

```text
pkg/appdesc      app_desc 读取、校验、过滤
pkg/plan         BuildPlan / Step / Group / Process
pkg/builder      PrepareBuildPlan 编排
```

其余文件暂留 `builder`，避免包过碎。

### B. 按转换职责拆

```text
pkg/appdesc
pkg/plan
pkg/buildpack
pkg/procfile
pkg/artifact
pkg/source
pkg/builder
```

适用：后续这些转换被其他工具复用。

问题：

- `procfile`、`groups`、`artifact` 当前各只有一个消费方
- 拆包后测试和导出 API 变多

### C. 激进拆分

每个文件一个包：

```text
appdesc plan buildpack env procfile group artifact source
```

不推荐：包太多，依赖方向复杂。

## 四、职责归属建议

| 责任 | 建议归属 |
|---|---|
| BuildPlan/Step/Group | `plan` |
| app_desc 解析/校验/过滤 | `appdesc` |
| 语言到 buildpack | `buildpack` |
| env 合并 | `plan` 或 `builder`（依赖 types） |
| Procfile | `procfile` 或 `plan` |
| 分组 | `plan` |
| artifact.json | `artifact` |
| 源码 tar | `source` |
| 计划编排 | `builder` |

## 五、迁移顺序

1. `plan`：只移动模型与格式化方法
2. `appdesc`：移动读取/校验/过滤
3. `builder`：只保留 PrepareBuildPlan，调用新包
4. 视消费方再拆 buildpack/procfile/artifact/source
5. 每步只移动，不改行为；`go test` + `just lint` + SOAR 构建验证

## 六、决策点

- `procfile` 是否独立，还是留在 `plan`
- `groups` 是否随 `plan`
- `env` 合并是否进 `plan`
- `artifact` 是否独立（协议边界 vs 单消费方）

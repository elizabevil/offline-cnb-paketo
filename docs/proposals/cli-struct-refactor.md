# bksmartpacker CLI 结构体化方案

> 状态：待实施。
> 目标：参数集中到一个结构体，增加参数只改一处；不再分散 `flag.String/Bool/Var`。

## 一、现状问题

`cmd/bksmartpacker/main.go` 目前用包级变量注册参数：

- 参数定义在 `var` 块
- 可重复参数用自定义 `stringList`
- `--help` 用 `BoolP` 单独处理
- 参数到 `types.Params` 的赋值集中在 `loadBuildParams`
- 新增参数要改 4 处：变量、初始化、赋值、文档

## 二、选型

| 方案 | 结论 |
|---|---|
| 继续 pflag | 可用，但仍需包级变量，不解决结构体诉求 |
| 标准库 flag | 仍需逐个注册，不解决结构体诉求 |
| Kong | **推荐**：结构体 + tag，支持默认值、枚举、可重复参数、内置 help |
| go-flags | 结构体可用，但维护活跃度不如 Kong |
| Cobra | 面向子命令，当前单命令场景过重 |

## 三、目标结构

新增 `cmd/bksmartpacker/cli.go`：

```go
type cliOptions struct {
    Src              string   `name:"src" help:"应用源码路径，需包含 app_desc.yaml"`
    Dest             string   `name:"dest" default:"./s-mart.tar.gz" help:"s-mart 输出路径"`
    BuilderImage     string   `name:"builder-image" default:"${defaultBuilderImage}" help:"cnb builder 镜像"`
    CacheRegistry    string   `name:"cache-registry" help:"缓存镜像仓库"`
    RegistryAuth     string   `name:"registry-auth" help:"镜像仓库凭证 JSON"`
    Inspect          string   `name:"inspect" help:"只检查已有 s-mart 包"`
    InspectFormat    string   `name:"inspect-format" default:"text" enum:"text,json" help:"检查输出格式"`
    Envs             []string `name:"env" help:"构建环境变量 KEY=VALUE，可重复"`
    EnvFiles         []string `name:"env-file" help:"dotenv 文件，可重复"`
    Modules          []string `name:"module" help:"只构建指定模块，可重复；部分包不可部署"`
    PackagingVersion string   `name:"packaging-version" default:"v2" enum:"v1,v2" help:"打包方案"`
    BaseImageID      string   `name:"base-image-id" default:"${defaultBaseImageID}" help:"平台 base image 配置 key"`
    Architecture     string   `name:"architecture" help:"目标镜像架构"`
    AllowPartial     bool     `name:"allow-partial" help:"允许生成部分调试包"`
}
```

默认值通过 `kong.Vars` 引用代码常量，避免重复字面量：

```go
kong.Vars{
    "defaultBuilderImage": executors.DefaultCNBBuilderImage,
    "defaultBaseImageID":  builder.DefaultBaseImageID,
}
```

## 四、边界

- 不在 `types.Params` 上直接加 CLI tag；`types` 是领域参数，不承担命令行展示职责
- `cliOptions.toParams()` 负责转换为 `types.Params`
- `cliOptions.validate()` 负责跨字段规则：
  - `--inspect` 与 `--src` 二选一
  - `--module` 默认必须配合 `--allow-partial`
  - 架构/打包版本枚举由 Kong tag 校验
- `main.go` 只保留：解析、校验、分发到 inspect/build

## 五、迁移步骤

1. 引入 `github.com/alecthomas/kong`
2. 新增 `cli.go`，定义结构体、默认值、转换、校验
3. `main.go` 删除包级 flag 变量、`init`、`stringList`、`loadBuildParams`
4. 用 `kong.New` / `parser.Parse` 替代 `flag.Parse`
5. 更新 `--help` 文案与 README 参数表
6. `go mod tidy` 删除 `pflag`
7. `just lint`、`just check`、`go test ./...`，并用 `--help`/`--inspect`/`--module` 手工验证

## 六、行为变化

- 帮助格式由 pflag 变为 Kong
- `--flag=value`、`--flag value`、可重复参数保持
- `--help` / `-h` 由 Kong 提供
- 未知参数、枚举错误在解析阶段失败
- 不再支持 pflag 的合并短参数；当前只有 `-h`，无影响

## 七、风险

- Kong 会新增一个直接依赖；同时移除 pflag，净增很小
- 帮助文案变化可能影响脚本解析帮助输出；现有脚本不解析帮助，风险低
- `--inspect` 与 `--src` 的互斥需要在 `validate` 中显式测试

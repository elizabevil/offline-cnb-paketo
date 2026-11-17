Go 开发规范
版本：v1.1
适用 Go 版本：1.27.x
适用范围：团队所有 Go 项目
强制工具：gofmt、go vet、golangci-lint

相关规范：协议解析见 [protocol-spec.md](protocol-spec.md)，包分层见 [package-layout.md](package-layout.md)。

一、常量与字面量
1.1 禁止魔法字面量
代码中不得直接出现含义不明的数字或字符串字面量。以下类型必须定义为常量：

类别	示例	必须提取
超时时间	30 * time.Second	✅
重试次数	3	✅
端口号	8080	✅
文件权限	0644	✅
错误消息	"user not found"	✅
缓冲区大小	4096	✅
go
// 禁止
http.Client{Timeout: 30 * time.Second}
for i := 0; i < 3; i++ { ... }

// 正确
const (
DefaultHTTPTimeout = 30 * time.Second
MaxRetries         = 3
)
http.Client{Timeout: DefaultHTTPTimeout}
for i := 0; i < MaxRetries; i++ { ... }
例外：0、1、""、nil 等自解释字面量，以及 iota 序列中首行的 iota + 1 模式。

1.2 常量组织
同一领域常量放在独立文件或 const 块中，按用途分组并加注释：

go
// internal/constants/timeout.go
const (
// DefaultHTTPTimeout 是外部 HTTP 调用的默认超时时间。
DefaultHTTPTimeout = 30 * time.Second
// DBQueryTimeout 是单次数据库查询的超时时间。
DBQueryTimeout = 5 * time.Second
)
命名遵循 Go 混合大小写规则：未导出用 maxLength，导出用 MaxLength，禁止 MAX_LENGTH 或 max_length。

二、函数封装与复用
2.1 重复逻辑必须封装
同一逻辑在 2 处及以上出现时，必须提取为公共函数或方法。判断标准不是代码行数，而是语义是否相同。

go
// 封装前：两处都在做相同的参数校验
if name == "" { return errors.New("name required") }
if email == "" { return errors.New("email required") }

// 封装后
func requireNonEmpty(field, value string) error {
if value == "" {
return fmt.Errorf("%s required", field)
}
return nil
}
2.2 函数边界原则
函数的长度没有硬性上限，但出现以下信号时必须拆分：

函数内出现两个以上逻辑层级（如“校验 + 转换 + 持久化 + 通知”）

函数需要超过 5 个参数（考虑用配置结构体替代）

函数内重复出现相同的错误处理或上下文传递模式

解决方案是调整函数边界，而不是数行数。

2.3 错误处理封装
错误处理不允许散落重复的 if err != nil { log; return } 模式。统一使用封装：

go
// 禁止在各处重复
if err != nil {
log.Printf("failed to fetch user: %v", err)
return nil, err
}

// 封装为内部工具
func wrapErr(op string, err error) error {
if err == nil { return nil }
return fmt.Errorf("%s: %w", op, err)
}
错误字符串规范：小写开头、无结尾标点。专名或缩写遵循其本身写法。

go
errors.New("user not found")   // 正确
errors.New("User not found.")  // 错误
三、接口与外部调用封装
3.1 接口定义位置
接口定义在使用方（consumer），而非实现方（producer）。实现方通常返回具体类型。

go
// 错误：在实现包中定义接口
package userstore
type UserStore interface { Get(id string) (User, error) }

// 正确：在使用方定义所需的最小接口
package service
type userGetter interface { Get(id string) (User, error) }

type UserService struct {
store userGetter
}
禁止为了 mock 而在实现方定义接口。测试应使用真实实现的公开 API，或在使用方定义 fake。

3.2 外部调用必须封装
所有对外部服务（HTTP API、数据库、消息队列、缓存）的调用，必须封装在独立的 client 层，业务逻辑不得直接发起外部调用。

go
// internal/client/payment.go
type PaymentClient struct {
httpClient *http.Client
baseURL    string
}

func (c *PaymentClient) Charge(ctx context.Context, req ChargeRequest) (*ChargeResponse, error) {
// 构造请求、设置超时、解析响应、错误映射全部在此处理
}
业务层只依赖 PaymentClient 的接口（在使用方定义），不感知 HTTP 细节。

3.3 接口合理性编译期检查
对导出类型实现的接口，加编译期断言：

go
var _ io.Reader = (*MyReader)(nil)
var _ http.Handler = (*MyHandler)(nil)
四、命名规范
4.1 缩写词保持大小写一致
缩写	正确	错误
URL	BaseURL / urlPath	BaseUrl
ID	userID / appID	userId
HTTP	ServeHTTP	ServeHttp
API	APIClient	ApiClient
此规则适用于人工编写的代码，protobuf 生成代码除外。

4.2 包名
包名应简短、小写、无下划线。禁止使用 util、common、misc、types、api 等无意义名称。包名应能与其导出标识符组合出有意义的表达：chubby.File 优于 chubby.ChubbyFile。

五、错误处理规范
5.1 错误流程缩进
正常路径保持最小缩进，错误处理优先返回：

go
// 禁止
if err == nil {
// 大量正常逻辑
} else {
return err
}

// 正确
if err != nil {
return err
}
// 正常逻辑
5.2 禁止丢弃错误
_ = someFunc() 仅在明确安全时允许，且必须加注释说明原因：

go
// 正确：Close 的错误在此场景下无关紧要
_ = resp.Body.Close()
六、并发与 Context
6.1 Context 必须传递
所有可能阻塞的调用（外部 API、数据库、time.Sleep 替代品）必须接受 context.Context 作为第一个参数，并在调用链中透传。

6.2 Goroutine 必须可终止
禁止启动无法取消的 goroutine。启动 goroutine 时必须明确其退出条件，通常通过 ctx.Done() 监听：

go
go func() {
for {
select {
case <-ctx.Done():
return
case msg := <-ch:
handle(msg)
}
}
}()
七、Go 1.27 专项规范
7.1 废弃 API 与已移除的 GODEBUG
禁止使用 io/ioutil 包。该包自 Go 1.16 起整体废弃，所有功能已迁移至 io 和 os。代码评审时见到 ioutil. 前缀直接打回。常见替换：

废弃	替换
ioutil.ReadAll	io.ReadAll
ioutil.ReadFile	os.ReadFile
ioutil.WriteFile	os.WriteFile
ioutil.TempFile	os.CreateTemp
ioutil.NopCloser	io.NopCloser
禁止设置以下已移除的 GODEBUG 开关：tlsunsafeekm、tlsrsakex、tls3des、tls10server、x509keypairleaf、gotypesalias。这些开关在 Go 1.27 中已被永久移除，设置它们不会生效，且使用旧值（如 gotypesalias=0）会导致构建失败。

crypto/tls.Config.Rand 字段已废弃，确定性测试应改用 testing/cryptotest.SetGlobalRandom。

7.2 encoding/json 行为变更（高风险）
Go 1.27 的 encoding/json v1 API 底层已由 v2 实现支撑。虽然接口兼容，但行为并非完全一致，且编译和 go vet 都不会报错。

v2 默认行为差异（当直接使用 encoding/json/v2 时）：

拒绝非法 UTF-8

拒绝 JSON 对象中的重复键

nil 切片序列化为 []，nil map 序列化为 {}

字段名匹配大小写敏感

omitempty 重新定义为省略“空 JSON 值”（null、""、[]、{}）

序列化 time.Duration 时报错（需显式配置选项）

如果团队代码使用 encoding/json/v2 的 json:",inline" 结构体标签，在 Go 1.27 下会被静默忽略，字段将作为嵌套对象输出。规范要求：使用 v2 API 时，inline 标签必须改为 embed。

如果仅使用 v1 API，需在测试中覆盖 JSON 输出的序列化结构，防止底层实现变更导致合同破坏。GOEXPERIMENT=nojsonv2 可临时回退，但官方已声明该开关预计在后续版本移除，不得作为长期依赖。

encoding/json/jsontext 中 Token 的 Float()、Int()、Uint() 方法在 v2 中返回值增加了 error，调用方必须处理。

7.3 Go 1.27 新特性利用
go fix 新增 modernizer：atomictypes、embedlit、slicesbackward、unsafefuncs。规范要求：升级 Go 版本后，对改动模块执行 go fix ./...，自动清理可被现代 API 替代的旧写法。

[//]: # ()
go mod tidy 行为变化：Go 1.27 起，go mod tidy 会自动将 go.mod 中多个 require 块合并为标准的两块结构（direct 与 indirect）。不得手动维护分散的 require 块，统一交由 go mod tidy 格式化。

泛型方法可用：Go 1.27 支持泛型方法（generic methods），可在类型上定义类型参数方法，减少为每种整数类型重复定义方法的需求。

函数类型推断扩展：泛型函数可在复合字面量、类型转换、channel 发送中省略显式类型参数。

八、提交前检查清单
每笔 MR 合并前必须通过：

□ gofmt -l . 无输出
□ go vet ./... 通过
□ golangci-lint run 通过
□ 新增字面量已提取为常量
□ 重复逻辑已封装或说明无需封装的理由
□ 外部调用均在 client 层
□ 接口定义在使用方
□ 并发代码有明确退出机制
□ 无 io/ioutil 引用
□ 未设置已移除的 GODEBUG 开关
□ 若使用 jsontext，已处理新增的 error 返回值
□ 若使用 encoding/json/v2，结构体标签无 inline（应为 embed）
□ 升级 Go 版本后执行过 go fix ./...
□ go.mod 中 require 块由 go mod tidy 维护，无手动分散块
参考：Go Code Review Comments、Effective Go、Go 1.27 Release Notes、Go 1.27 Blog。本规范未覆盖的细节以上述来源为准。

九、约束的自动化验证

规范由 `just check` 强制执行，不依赖人工记忆。

| 规则 | 工具 / 检查 | 阈值或范围 |
|---|---|---|
| 1.1 禁止魔法字面量 | `goconst`（min-occurrences=3，忽略测试） | 重复 3 次以上必须常量化 |
| 2.1 重复逻辑 | `dupl`（threshold=150） | 超过 150 token 的重复块 |
| 2.2 函数边界 | `funlen`（80 行 / 50 语句）、`gocognit`（45）、`cyclop`（22） | 超限需拆分或 `//nolint` 说明 |
| 3.1 接口定义位置 | 人工评审 | — |
| 3.3 接口编译期断言 | 人工评审 | — |
| 4.1 缩写词大小写 | `revive`（var-naming） | — |
| 4.2 包名 | `just check`：禁止 util/common/misc/types/api 目录 | 硬失败 |
| 5.1 错误流程缩进 | `revive` / `gocritic` | — |
| 5.2 禁止丢弃错误 | `errcheck` | 仅排除 fmt 输出 |
| 7.1 禁止 io/ioutil | `just check` grep | 硬失败 |
| 7.1 已移除的 GODEBUG | `just check` grep（tlsunsafeekm 等 6 个） | 硬失败 |
| 团队约定：错误信息为英文 | `just check` grep 中文错误 | 硬失败 |
| 协议单一解析器 | `just check` grep `toml.Unmarshal` | 仅允许 `internal/spec/` |
| 平台私有协议不进通用包 | `just check` grep `BKS_` / `app_desc` / `cnb-image-layers` | 硬失败 |

复杂度阈值是当前基线，只降不升。已知待拆分函数用 `//nolint:<linter> // TODO(protocol-layers): 原因` 标注，
`nolintlint` 要求必须带原因，避免变成永久豁免。

当前待拆分清单（随重构递减）：

| 函数 | 问题 |
|---|---|
| `cmd/cnb.newProfileCmd` | 137 行，按子命令拆分 |
| `profile.BuildPlan` | 85 行，资产信息与阶段展开拆开 |
| `profile.render` | 56 语句，按 image/command 分开渲染 |
| `repack.normalizeWebTar` | 73 语句，manifest/层复制/索引写入拆开 |
| `lock.Validate` | 认知复杂度 46，按 storage/asset/引用分节 |
| `repack.extractTar` | 圈复杂度 26，条目类型分支拆成处理函数 |

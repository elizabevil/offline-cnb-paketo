# 协议包开发规范（spec/*）

适用：`script/tools/cnb/internal/spec/**`。
配套：[协议分层设计](../design/protocol-layers.md)、[包分层](package-layout.md)、[Go 规范](go-build.md)。

## 一、职责边界

spec 包**只做三件事**：解析、编码、校验。不做以下任何事：

| 禁止 | 原因 |
|---|---|
| 读文件 / 写文件 | I/O 由业务包负责，spec 只接受 `[]byte` |
| 读环境变量 | 环境属于 `spec/platform`，且必须 `getenv` 注入 |
| 打日志 / 打印 | 解析告警以返回值给出 |
| 执行命令、访问网络 | 同上 |
| 依赖业务包（materials/lifecycle/pipeline…） | 依赖方向只能业务 → spec |

允许依赖：标准库、`internal/errs`、`internal/spec/warn`。

## 二、统一 API 形状

```go
// 解析：输入字节，输出结构体 + 告警 + 错误
func Parse(data []byte) (*Thing, []warn.Warning, error)

// 编码：仅用于需要回写的协议（order、profile 模板）
func (t *Thing) Encode() ([]byte, error)

// 校验：返回全部问题，不短路
func (t *Thing) Validate() []error
```

规则：

- 解析函数统一命名 `Parse`，前缀区分协议：`ParseOrder`、`ParseBuildpack`
- 告警与错误分离：**未知字段**是 `warn.Warning`，**结构非法**是 `error`
- 校验返回切片，一次报告所有问题；不要 `return errors.New(...)` 单条

## 三、未知字段必须报告

协议升级最常见的发现方式是“配置里写了新字段，代码没认出来”。因此：

```go
func unknownKeys(data []byte, known map[string]bool) []warn.Warning {
	var raw map[string]any
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil
	}
	var out []warn.Warning
	for k := range raw {
		if !known[k] {
			out = append(out, warn.Warning{Path: k, Message: "unknown key"})
		}
	}
	return out
}
```

- `known` 集合显式列出当前支持的键
- 告警要能被上层展示（`buildpack lint`、`cnb config show`）
- 禁止用 `toml.Decoder.DisallowUnknownFields()` 直接报错：协议升级需要渐进兼容

## 四、单一解析器

**同一协议只允许一处 `Parse`。** 其他包消费结构体，禁止再解析文本。

静态检查（`just check` 已包含两条）：

```bash
# 1) TOML 解析只允许出现在 spec 包（迁移期白名单：lock、profile）
grep -rn 'toml.Unmarshal' --include='*.go' script/tools/cnb/internal \
  | grep -v 'internal/spec/' | grep -v 'internal/lock/' | grep -v 'internal/profile/'
# 2) 通用包不得出现平台私有协议
grep -rn 'BKS_\|app_desc\|cnb-image-layers' --include='*.go' script/tools/cnb/internal
```

白名单是迁移期的临时状态：阶段 2（spec/manifest）完成后移除 `lock`，
阶段 4（spec/profile）完成后移除 `profile`，最终只保留 `internal/spec/`。

历史教训：`order.toml` 曾在 `lifecycle/order.go`（toml）与 `checks/buildpack.go`（正则）
各解析一次，改字段时两边不同步。

## 五、类型与命名

| 场景 | 约定 | 例子 |
|---|---|---|
| 文件根结构 | 协议名 | `Order`、`Buildpack`、`File` |
| 段落 | 段落名 | `Info`（`[buildpack]`）、`Metadata` |
| 枚举 | 自定义类型 + 常量 | `Mode`、`Kind`、`Layout` |
| 可选字段 | 零值即缺省，`omitempty` | `Version string` |

- 字段名与协议键名对应，**只导出需要消费的字段**
- 新增可扩展字段时同步更新 `known` 集合与测试
- 不在 spec 里做“业务解释”（如“这个版本号是否支持当前平台”属于业务校验）

## 六、测试要求

每个 spec 包必须有：

| 用例 | 内容 |
|---|---|
| 正常解析 | 代表性的真实文件（可来自 `test/contract/spec/`） |
| 未知字段 | 断言 `warn.Warning` 的 `Path` |
| 结构非法 | 断言 `error` 非空 |
| 编码往返 | `Parse → Encode → Parse` 结构一致（仅对有 Encode 的协议） |
| 过滤/选择 | 若协议提供 `Filter` / `Select` 类方法，需覆盖命中与不命中 |

约定：

- 测试数据用内联常量（小文件）或 `testdata/`（>50 行）
- 表驱动优先，失败信息带上输入片段
- 不 mock 文件系统：spec 不接触 I/O，无需 mock

## 七、新增一个协议的步骤

1. 在 `docs/design/protocol-layers.md` 登记：协议、来源规范、读方、写方
2. 新建 `internal/spec/<name>/`，写 `Parse` / `Encode` / `Validate`
3. 补未知字段告警与测试
4. 把业务包里的解析代码改为消费该包，删除旧实现
5. 跑 `just check`（含单一解析器静态检查）

## 八、检查清单

- [ ] spec 包没有 `os` / `net` / `os/exec` 导入
- [ ] 没有读环境变量
- [ ] `Parse` 返回 `([]byte) (T, []warn.Warning, error)`
- [ ] 未知字段会产出告警
- [ ] 该协议没有第二处解析器
- [ ] 有未知字段、非法结构、往返三类测试

# 检验手册（verify）

> 给接手的人：**改任何东西之前先跑一遍，改完再跑一遍。**
> 每条都给出「命令 → 期望 → 失败意味着什么」，不写空话。

---

## 0. 一键验收

```bash
just check && just verify
```

- `just check`：结构层（lock 合法性、模板未腐烂、shell 语法、Go 格式/静态检查/单测）
- `just verify`：物料层（归档哈希、解压标记、预期产物）

两者都过，说明**基座本身**是健康的。镜像/`pack build` 不在范围内，见文末。

---

## 1. 分层视图

| 层 | 验什么 | 命令 | 失败意味着 |
|---|---|---|---|
| L1 结构 | lock 与模板合法、脚本语法、Go 代码质量 | `just check` | 配置或代码本身有问题，别往下走 |
| L2 物料 | 归档哈希、解压标记、产物齐全 | `just verify` | 物料损坏/未解压/被手工改过 |
| L3 工具行为 | `cnb` 命令契约与退出码 | 本文 §3 | 输出格式变了，会悄悄打断依赖它的脚本 |
| L4 repack | 与 Python 参照实现等价 | 本文 §4（手动） | OCI 包内容漂移，**发版前必须跑** |
| L5 集成 | shell ↔ Go 的 marker 契约 | 本文 §5 | 自愈链路断了 |

---

## 2. L1：结构层

`just check` 展开后是这样几条，任何一条红都不该继续：

```bash
# 1) 生效的 lock 合法
script/bin/cnb-linux-amd64 --file assets.lock.toml validate
# 期望：  ✓ lock 结构合法：3 个物料，2 个平台

# 2) 模板不能腐烂（模板也是文档，改了字段却不同步模板 = 下一个人被误导）
script/bin/cnb-linux-amd64 --file assets.toml.example materials validate
# 期望：  ✓ lock 结构合法：3 个物料，2 个平台

# 3) shell 语法
bash -n script/ci.sh script/check-updates.sh

# 4) Go 质量
cd script/tools/cnb && gofmt -l . && go vet ./... && go test ./...
# 期望：gofmt 无输出；vet 无输出；ok
```

> 为什么模板也要 validate：模板里有一个「sha256 留空」的示例。
> 如果哪天校验规则改成「sha256 必填」，模板会立刻变红，逼你同步改文档。

---

## 3. L3：`cnb` 的命令契约

这些格式被 justfile 与脚本直接消费，**改格式 = 破坏调用方**。

| 命令 | 期望输出 | 退出码 |
|---|---|---|
| `cnb materials list` | `id\tversion\tkind\tplatforms`，平台按字母序、逗号分隔 | 0 |
| `cnb materials validate` | `✓ ...` 或逐条 `✗ ...` | 0 / 1 |
| `cnb materials verify` | 每物料 `✓/✗` | 0 / 1 |
| `cnb image convert <不存在路径> <目标>` | `目标不存在` | 1 |

逐条手工验证：

```bash
script/bin/cnb-linux-amd64 materials list | cat -A | head -3   # 看 \t 是否是真正的制表符
script/bin/cnb-linux-amd64 materials list
script/bin/cnb-linux-amd64 materials verify --platform linux-arm64
```

单测覆盖的是同一批契约：

```bash
cd script/tools/cnb && go test -v ./... | grep -E '^(=== RUN|--- (PASS|FAIL))'
```

---

## 4. L4：repack 回归（改 repack 逻辑时必跑）

判据是**内建的 golden 测试**，不依赖任何外部实现（历史 Python 参照已移除）：

```bash
cd script/tools/cnb && go test -run Repack -v ./...
```

四个用例分别守住不同性质：

| 用例 | 验什么 |
|---|---|
| `TestRepackDirGolden` | 四个 JSON 载荷与基准**字节一致**；manifest blob 与 digest 自洽；层 / config 内容原样搬运 |
| `TestRepackIdempotent` | 二次执行后 `web.tgz` 的 sha256 不变 |
| `TestRepackPackageTarball` | `*.tar.gz` 模式与目录模式产物一致 |
| `TestRepackAlreadyOCI` | 已规范化的包原样返回（短路判断未失效） |

**基准在哪**：`repack_test.go` 顶部的 `golden*` 常量与 fixture 内容常量。
它们就是事实标准——不存在第二份实现可以对照。

**确属预期变更时**（例如新增字段）：

1. 改 Go 实现；
2. `go test` 失败，逐字看 diff 是否真是你要的变化；
3. 用同样的 fixture 跑一次 `cnb image convert`，人工核对输出；
4. commit message 里写明「为什么字节变了」。

**失败排查**：

| 现象 | 根因 |
|---|---|
| `blobs/sha256` 缺失 | `addTree` 对目录没递归 |
| 只有空目录、层文件消失 | 同上 |
| JSON 键顺序不同 | 结构体字段顺序被改，或该用有序结构却用了 `map` |
| 二次执行后哈希变化 | 幂等判断（`oci-layout` + `index.json`）失效 |
| 包里缺 `repositories` | 忘记写最后一个载荷 |

## 5. L5：集成契约（shell ↔ Go）

marker 是两侧共享的状态，必须双向验：

```bash
# ① 正常
just verify                      # 期望全 ✓

# ② 删 marker → Go 侧必须报失败（而不是假装通过）
mv bundle/linux-amd64/binaries/.meta/uv.sha256 /tmp/uv.marker
just verify; echo "exit=$?"      # 期望 exit=1，uv 报「未解压或产物不全」

# ③ 恢复 → shell 侧 fetch 必须自愈（重解压并重建 marker）
just fetch uv
just verify                      # 期望再次全 ✓
cat bundle/linux-amd64/binaries/.meta/uv.sha256   # 期望是 64 位 hex

# ④ 篡改归档 → 必须报哈希不符
cp bundle/linux-amd64/archives/fnm-linux.zip /tmp/fnm.bak
printf 'x' >> bundle/linux-amd64/archives/fnm-linux.zip
just verify; echo "exit=$?"      # 期望 exit=1，报「哈希不符」
cp /tmp/fnm.bak bundle/linux-amd64/archives/fnm-linux.zip
just verify                      # 恢复
```

> 注意：强制重下请先删归档再 fetch，或直接 `cnb materials sync <id>`（已存在时跳过）：
> `rm -f bundle/<platform>/archives/<id>/<version>/* && just fetch <id>`

---

## 6. 三态自愈（物料层最容易被搞坏的地方）

```bash
# 只删产物、保留归档：fetch 应重新解压（而不是"归档在就跳过"）
rm -rf bundle/linux-amd64/binaries/lifecycle
just fetch lifecycle && ls bundle/linux-amd64/binaries/lifecycle | head -3
```

期望：日志出现「解压 lifecycle」，而不是「已解压」。

---

## 7. 不在本手册范围

- `just docker-build` / `pack build`：需要容器运行时。当前沙箱里 podman 无法创建
  user namespace、docker socket 无权限，**必须在你的宿主机或 CI 上验**（见 `docs/design/base.md` 环境边界）。
- 镜像内部的 buildpack 行为：属于 P1/P2 阶段的工作，尚待修复。

---

## 8. CI 门禁建议

```yaml
- run: just check          # 结构 + 单测，几十秒
- run: just verify         # 物料完整性（需要 bundle 已就绪或先跑 fetch）
```

L4（repack golden）只在改动 `repack.go` 或发版时跑；它是纯 Go 测试，不引入额外运行时依赖。

## 附：libc 一致性（人工确认）

| 运行时 | 基础镜像要求 |
|---|---|
| Node（fnm 官方构建） | glibc（debian/ubuntu） |
| Python（uv 的 cpython-*-gnu） | glibc |
| Go（静态链接） | 任意，含 alpine |

当前没有自动化检查：若 base 镜像为 `alpine`/`musl` 且项目启用 Node 或 Python，需要人工确认。



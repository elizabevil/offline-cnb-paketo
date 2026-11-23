# Git 提交规范（AI 与协作者必读）

> 目标：**历史即文档**。每条提交应能独立解释"为什么改"，不保留中间脏态。

## 一、核心规则

1. **一条提交只做一件事**
   - 功能、重构、文档、构建产物分开提交
   - 同一功能的多步实现，合成为一条（除非中间态确有独立价值）

2. **禁止"修补型"提交堆积**
   - `fix: 上一个提交的笔误`、`fix: 补上遗漏的文件` 这类提交，
     在推送前必须 squash 进原提交
   - 判断标准：新提交是否只是让前一条提交"变正确"？是 → fixup

3. **每条提交都必须可构建**
   - 至少满足：`gofmt` 无差异、`go build ./...` 通过
   - 涉及配置/文档的提交：`just check` 通过

4. **提交前跑门禁**

   ```bash
   just check        # 结构 + lint + 测试
   script/ci.sh --full # 完整 CI（含 arm64 与双平台物料校验）
script/ci.sh --offline # 离线环境：跳过物料下载与依赖 bundle 的检查
   ```

5. **提交信息格式**

   ```
   <type>(<scope>): <subject>

   <body：为什么、影响面>
   ```

   | type | 用途 |
   |---|---|
   | `feat` | 新增能力 |
   | `refactor` | 行为不变的结构调整 |
   | `fix` | 修缺陷 |
   | `docs` | 只改文档 |
   | `chore` | 构建产物、依赖、工具配置 |

   `scope` 用目录或包名：`cli`、`checks`、`manifest`、`scaffold`、`docs`。

## 二、合并历史的操作方法

```bash
# 1. 查看待整理的提交
git log --oneline origin/master..HEAD

# 2. 交互式合并：把修补提交标为 fixup / squash
git rebase -i origin/master
#   pick   保留
#   fixup  并入上一条并丢弃本条的说明
#   squash 并入上一条并合并说明
#   reword 只改说明

# 3. 合并后必须验证
git diff <整理前的 sha> HEAD    # 树必须完全一致
just check && script/ci.sh --full
```

**整理前先建备份**：

```bash
git branch backup/pre-rewrite
```

确认无误后再删。

## 三、禁止事项

| 禁止 | 原因 |
|---|---|
| 提交 `test/.work/`、`bundle/`、`.cache/` | 构建产物，已 gitignore |
| 提交个人调试脚本（硬编码绝对路径） | 对他人无用，且泄露本地路径 |
| 一个提交里混入"格式化 + 功能" | review 无法分离 |
| 推送未验证的历史改写 | 影响他人；先本地 `just check` |

## 四、AI 提交时的额外要求

1. 提交前必须执行 `just check` 与相关测试，结果写入提交说明尾部
2. 改历史（rebase/squash）后必须验证 **树内容未变**
3. 不得为"修自己的笔误"新增提交，直接 fixup
4. 提交说明用中文描述"为什么"，代码/命令保持英文

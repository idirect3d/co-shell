# FEATURE-236: 输出前导提示表情符号后加空格

## 背景

co-shell 的 REPL/流式输出前导提示使用 `[emoji]` 形式（如 `[⚙️]< 读取文件...`、`[👤]> ...`）。当前 `]` 前无空格，视觉上表情符号与提示内容粘连（如 `[⚙️]<`、`[👤]>`），阅读时可读性差。本任务统一在表情符号与 `]` 之间加一个空格，即 `[⚙️]` → `[⚙️ ]`。

## 修改方案

### 1. i18n 资源（`i18n/zh.go` 与 `i18n/en.go` 的 `KeyEmojiPrefix*` 系列值）

| Key | 修改前 (zh) | 修改后 (zh) |
|-----|-------------|-------------|
| KeyEmojiPrefixUser | `[👤]> ` | `[👤 ]> ` |
| KeyEmojiPrefixAssistant | `[🐚]> ` | `[🐚 ]> ` |
| KeyEmojiPrefixToolInput | `[⚙️]< ` | `[⚙️ ]< ` |
| KeyEmojiPrefixToolOutput | `[⚙️]> ` | `[⚙️ ]> ` |
| KeyEmojiPrefixCmdInput | `[🔴]< ` | `[🔴 ]< ` |
| KeyEmojiPrefixCmdOutput | `[🔴]> ` | `[🔴 ]> ` |
| KeyEmojiPrefixInfo | `[ℹ️] ` | `[ℹ️ ] ` |
| KeyEmojiPrefixError | `[❌] ` | `[❌ ] ` |
| KeyEmojiPrefixWarning | `[⚠️] ` | `[⚠️ ] ` |
| KeyEmojiPrefixSuccess | `[✅] ` | `[✅ ] ` |
| KeyEmojiPrefixThinking | `[💬] ` | `[💬 ] ` |
| KeyEmojiPrefixOutputTitle | `[📋] 命令输出:` | `[📋 ] 命令输出:` |

英文 `i18n/en.go` 同步修改（`Command Output:` 对应项）。

### 2. config 硬编码前缀（`config/config.go` `GetEmojiPrefixes()`）

调研发现：**实际运行时输出的前导提示（REPL 提示符、工具/命令前缀、流式输出）全部来自 `config.GetEmojiPrefixes()`，前缀硬编码、不读取 i18n 资源**。仅改 i18n 资源不会让真实输出加空格。经确认后同步修改 emoji enabled 分支全部 13 项（含 `VisionUserInput` `[👀]>` 与 `Loop` `[♾️]`，这两者在 i18n 中无对应 key），禁用分支（纯文本 `[user]>` 等）不属表情符号，未改。

### 3. 顺带修复 pre-existing golden 失配（`agent/stream_renderer.go`）

单命令模式 token 渲染存在废弃死代码：`KeyTokenUsageTiming` 已与 `KeyTokenUsageDisplay` 合并（值为空串），但 `renderTokenIter` 单命令分支仍 `fmt.Sprintf(空串, ft, inTPS, outTPS)`，产生 `token_usage_timing%!(EXTRA...)` 乱码行，导致 `TestRenderSingleCmdGolden` 基线失配（stash 验证确认 pre-existing）。删除该死代码段，重新生成 golden。

## 用例列表

### UC-0001 资源字符串格式检查（循环验证 zh/en 全部 KeyEmojiPrefix*）
**前置条件**：i18n 资源加载成功。
**步骤**：循环读取 `KeyEmojiPrefix*` 系列在 zh/en 下的翻译值。
**预期**：每一个值的表情符号与 `]` 之间均含且仅含一个空格（格式 `[emoji ]...`）；中英文行为一致。

### UC-0002 工具调用前导提示（运行时）
**前置条件**：启动 co-shell 进入 REPL，show-tool 开启。
**步骤**：触发一次工具调用（如 `:simulate` 模拟 read_file）。
**预期**：输出为 `[⚙️ ]< 读取文件...`，表情符号后、`]` 前有空格。

### UC-0003 命令输入/输出前导提示（运行时）
**前置条件**：启动 co-shell。
**步骤**：执行一条系统命令（如 `ls`）并观察命令输入/输出提示。
**预期**：`[🔴 ]<` 与 `[🔴 ]>` 形式，`]` 前有空格。

### UC-0004 信息/错误/警告/成功提示（运行时）
**前置条件**：启动 co-shell。
**步骤**：分别触发信息提示（:help）、错误场景（无效指令）、警告场景、成功场景（完成任务）。
**预期**：`[ℹ️ ]`、`[❌ ]`、`[⚠️ ]`、`[✅ ]` 均为 `]` 前带空格。

### UC-0005 英文环境双语一致性（运行时）
**前置条件**：以 `--lang en` 启动 co-shell。
**步骤**：重复 UC-0002~0004 场景。
**预期**：英文 i18n 资源同样输出 `[emoji ]` 格式，与中文行为一致。

## 验收标准
1. `go build ./...` 通过
2. `go vet ./...` 无警告
3. `go test ./...` 全量通过（含 repl/root 两份 golden 回归）
4. 改动范围：i18n 资源（zh/en）+ config.GetEmojiPrefixes() 硬编码前缀 + 相关联 golden 基线 + pre-existing 死代码修复
5. 运行时输出前缀均为 `[emoji ]` 格式（`]` 前一个空格），zh/en 行为一致

# 输出控制梳理清单

> 本文档梳理了当前代码中所有与 LLM 迭代输出相关的控制点，为 ENHANCEMENT-126 优化输出模式控制做准备。
> 目标：删除现有的 `show-output`、`output-mode` 两个开关，替换为 7 个独立的开关。

---

## 一、现有输出控制概览

### 1.1 当前配置项（config.go）

| 配置项 | 类型 | 默认值 | 说明 |
|--------|------|--------|------|
| `ShowThinking` | bool | false | 是否显示 LLM 思考过程 |
| `ShowCommand` | bool | true | 是否在执行前显示命令 |
| `ShowOutput` | bool | true | 是否在 LLM 分析前显示命令输出 |
| `OutputMode` | int | 1 (normal) | 输出显示模式 (0=compact, 1=normal, 2=debug) |

### 1.2 OutputMode 三种模式的行为

| 输出内容 | compact (0) | normal (1) | debug (2) |
|----------|:-----------:|:----------:|:---------:|
| LLM 响应内容 | ✅ | ✅ | ✅ |
| Tool-call 方法名 | ❌ | ✅ | ✅ |
| Tool-call 输入参数 | ❌ | ❌ | ✅ |
| Tool-call 返回结果 | ❌ | ❌ | ✅ |
| 系统命令 | ❌ | ✅ (受 ShowCommand 控制) | ✅ (受 ShowCommand 控制) |
| 命令输出 | ❌ | ❌ | ✅ (受 ShowOutput 控制) |

### 1.3 目标：7 个独立开关

| 新开关 | 默认值 | 说明 |
|--------|:------:|------|
| `show-llm-thinking` | on | LLM 返回的 thinking 内容 |
| `show-llm-content` | on | LLM 返回的主要内容 |
| `show-tool` | on | 输出 Tool-call 名 |
| `show-tool-input` | off | 输出 Tool-call 输入参数 |
| `show-tool-output` | off | 输出 Tool-call 返回数据 |
| `show-command` | on | 输出系统命令行 |
| `show-command-output` | on | 输出命令返回数据 |

---

## 二、输出控制点详细清单

### 2.1 agent/loop.go — Agent 循环中的输出

#### 2.1.1 streamLLMResponse 方法（第 627-737 行）

| 行号 | 代码 | 当前控制 | 新开关映射 | 说明 |
|------|------|----------|-----------|------|
| 668 | `cb("content_chunk", event.Content)` | 无条件 | `show-llm-content` | 流式输出 LLM 内容块 |
| 673 | `cb("thinking_chunk", event.Content)` | `a.showThinking` | `show-llm-thinking` | 流式输出 thinking 内容块 |

#### 2.1.2 nonStreamingFallback 方法（第 740-753 行）

| 行号 | 代码 | 当前控制 | 新开关映射 | 说明 |
|------|------|----------|-----------|------|
| 748-749 | `cb("thinking", resp.ReasoningContent)` | `a.showThinking` | `show-llm-thinking` | 非流式输出 thinking 内容 |

#### 2.1.3 RunStream 方法（第 239-529 行）

| 行号 | 代码 | 当前控制 | 新开关映射 | 说明 |
|------|------|----------|-----------|------|
| 436-443 | `cb("command", cmd)` | `a.outputMode >= normal && a.showCommand && tc.Name == "execute_command"` | `show-command` | 显示执行的系统命令 |
| 446-448 | `cb("tool_call", fmt.Sprintf("🛠 Calling tool: %s\n", tc.Name))` | `a.outputMode >= normal` | `show-tool` | 显示工具调用名 |
| 485-487 | `cb("output", result)` | `a.outputMode >= debug && a.showOutput && tc.Name == "execute_command" && result != ""` | `show-command-output` | 显示命令执行输出 |
| 391 | `cb("done", "")` | 无条件 | 不变 | 完成事件 |

**注意**：当前代码中，`tool_call` 事件只输出了工具名，没有输出输入参数和返回结果。输入参数和返回结果是通过 `output` 事件（仅 execute_command）和 `tool_call` 事件（仅工具名）分别控制的。

### 2.2 repl/repl.go — REPL 中的输出处理

#### 2.2.1 streamCallback 方法（第 425-461 行）

| 行号 | 代码 | 当前控制 | 新开关映射 | 说明 |
|------|------|----------|-----------|------|
| 427-428 | `fmt.Print(content)` | 无条件 | `show-llm-content` | content_chunk 事件 |
| 430-431 | `fmt.Print(content)` | 无条件 | `show-llm-thinking` | thinking_chunk 事件 |
| 433-436 | `fmt.Print(content) + println` | 无条件 | `show-llm-content` | content 事件（非流式） |
| 437-439 | `fmt.Print(content) + println` | 无条件 | `show-llm-thinking` | thinking 事件（非流式） |
| 441-442 | `fmt.Printf("⚡ %s\n", content)` | 无条件 | `show-command` | command 事件 |
| 444-450 | 输出标题+分隔线+内容+分隔线 | 无条件 | `show-command-output` | output 事件 |
| 452-453 | `fmt.Println(content)` | 无条件 | `show-tool` | tool_call 事件 |
| 455-456 | `fmt.Printf("❌ %s\n", content)` | 无条件 | 不变 | error 事件 |
| 458-459 | `fmt.Println()` | 无条件 | 不变 | done 事件 |

**关键问题**：REPL 的 `streamCallback` 目前是无条件处理所有事件的，所有控制逻辑都在 Agent 的 `RunStream` 中决定是否发送事件。这意味着：
- 如果 Agent 不发送某个事件，REPL 就不会收到
- 但 REPL 收到的事件都会无条件显示

#### 2.2.2 handleSystemCommand 方法（第 386-405 行）

| 行号 | 代码 | 当前控制 | 新开关映射 | 说明 |
|------|------|----------|-----------|------|
| 388-390 | `fmt.Printf("$ %s\n", command)` | `r.cfg.LLM.ShowCommand` | `show-command` | 直接执行系统命令时显示命令 |
| 396 | `fmt.Print(output)` | 无条件 | `show-command-output` | 显示命令输出（错误时） |
| 403 | `fmt.Println(output)` | 无条件 | `show-command-output` | 显示命令输出 |

### 2.3 main.go — 单命令模式中的输出

#### 2.3.1 executeSingleCommand 方法（第 819-868 行）

| 行号 | 代码 | 当前控制 | 新开关映射 | 说明 |
|------|------|----------|-----------|------|
| 825-827 | `fmt.Printf("$ %s\n", input)` | `cfg.LLM.ShowCommand` | `show-command` | 直接命令模式显示命令 |
| 830 | `fmt.Print(output)` | 无条件 | `show-command-output` | 显示命令输出（错误时） |
| 835 | `fmt.Println(output)` | 无条件 | `show-command-output` | 显示命令输出 |
| 845 | `fmt.Print(content)` | 无条件 | `show-llm-content` | content_chunk 事件 |
| 847 | `fmt.Print(content)` | 无条件 | `show-llm-thinking` | thinking_chunk 事件 |
| 849 | `fmt.Printf("⚡ %s\n", content)` | 无条件 | `show-command` | command 事件 |
| 851-855 | 输出标题+分隔线+内容+分隔线 | 无条件 | `show-command-output` | output 事件 |
| 857 | `fmt.Println(content)` | 无条件 | `show-tool` | tool_call 事件 |
| 859 | `fmt.Printf("❌ %s\n", content)` | 无条件 | 不变 | error 事件 |
| 861 | `fmt.Println()` | 无条件 | 不变 | done 事件 |

---

## 三、配置层影响范围

### 3.1 config/config.go

| 位置 | 内容 | 说明 |
|------|------|------|
| 第 133-135 行 | `ShowThinking`, `ShowCommand`, `ShowOutput` 字段 | 需要替换为 7 个新字段 |
| 第 83-122 行 | `OutputMode` 类型定义、常量、Parse/String 方法 | 需要删除 |
| 第 169-171 行 | `OutputMode` 字段 | 需要删除 |
| 第 250-252 行 | 默认值设置 | 需要更新为新字段默认值 |
| 第 339-359 行 | `Show()` 方法中的状态显示 | 需要更新 |
| 第 443-444 行 | `outputModeStr` 变量 | 需要删除 |
| 第 512 行 | `output-mode:` 显示行 | 需要删除 |

### 3.2 agent/agent.go

| 位置 | 内容 | 说明 |
|------|------|------|
| 第 74-76 行 | `showThinking`, `showCommand`, `showOutput` 字段 | 需要替换为 7 个新字段 |
| 第 83 行 | `outputMode` 字段 | 需要删除 |
| 第 95-108 行 | `SetShowThinking`, `SetShowCommand`, `SetShowOutput` 方法 | 需要替换为 7 个新 Setter |
| 第 149-152 行 | `SetOutputMode` 方法 | 需要删除 |

### 3.3 agent/loop.go

| 位置 | 内容 | 说明 |
|------|------|------|
| 第 74-76 行 | Agent 结构体中的 `showThinking`, `showCommand`, `showOutput` | 需要替换 |
| 第 83 行 | Agent 结构体中的 `outputMode` | 需要删除 |
| 第 436 行 | `a.outputMode >= config.OutputModeNormal && a.showCommand` | 需要改为 `a.showCommand` |
| 第 446 行 | `a.outputMode >= config.OutputModeNormal` | 需要改为 `a.showTool` |
| 第 485 行 | `a.outputMode >= config.OutputModeDebug && a.showOutput` | 需要改为 `a.showCommandOutput` |
| 第 668 行 | `cb("content_chunk", event.Content)` | 需要加 `a.showLlmContent` 控制 |
| 第 673 行 | `cb("thinking_chunk", event.Content)` | 需要加 `a.showLlmThinking` 控制 |
| 第 748 行 | `cb("thinking", resp.ReasoningContent)` | 需要加 `a.showLlmThinking` 控制 |

### 3.4 cmd/settings.go

| 位置 | 内容 | 说明 |
|------|------|------|
| 第 153-177 行 | `show-thinking` 处理 | 保留，但配置字段名可能变化 |
| 第 179-203 行 | `show-command` 处理 | 保留，语义不变 |
| 第 205-229 行 | `show-output` 处理 | **需要删除**，替换为 `show-command-output` |
| 第 517-533 行 | `output-mode` 处理 | **需要删除** |
| 第 737-948 行 | `showSettingsHelp` 中的显示 | 需要更新分组和显示项 |

### 3.5 main.go

| 位置 | 内容 | 说明 |
|------|------|------|
| 第 75-76 行 | `showCommand`, `showOutput` CLI 标志 | 需要更新 |
| 第 102 行 | `outputMode` CLI 标志 | 需要删除 |
| 第 149-151 行 | `--show-thinking`, `--show-command`, `--show-output` 标志定义 | 需要更新 |
| 第 180 行 | `--output-mode` 标志定义 | 需要删除 |
| 第 442-471 行 | CLI 覆盖逻辑 | 需要更新 |
| 第 557-564 行 | output-mode CLI 覆盖 | 需要删除 |
| 第 698-700 行 | `SetShowThinking`, `SetShowCommand`, `SetShowOutput` 调用 | 需要更新 |
| 第 738 行 | `SetOutputMode` 调用 | 需要删除 |

### 3.6 repl/repl.go

| 位置 | 内容 | 说明 |
|------|------|------|
| 第 336-339 行 | `.set` 命令后的同步调用 | 需要更新 |

### 3.7 i18n 相关键值

| 键名 | 位置 | 说明 |
|------|------|------|
| `KeyShowThinking` | i18n/i18n.go | 保留 |
| `KeyShowCommand` | i18n/i18n.go | 保留 |
| `KeyShowOutput` | i18n/i18n.go | **需要删除** |
| `KeyOutputModeUpdated` | i18n/i18n.go | **需要删除** |
| `KeyCLIHelpOutputMode` | i18n/i18n.go | **需要删除** |
| `KeyCol3OutputMode` | i18n/i18n.go | **需要删除** |
| `KeySettingsDescOutMode` | i18n/i18n.go | **需要删除** |
| `KeyConfigShowOutput` | i18n/i18n.go | **需要删除** |
| `KeySettingsLabelShowOutput` | i18n/i18n.go | **需要删除** |
| `KeyCLIHelpShowOutput` | i18n/i18n.go | **需要删除** |
| `KeySettingsDescShowOutput` | i18n/i18n.go | **需要删除** |
| `KeyCol3Output` | i18n/i18n.go | **需要删除** |
| `KeyOutputTitle` | i18n/i18n.go | 保留（用于命令输出标题） |
| `KeyOutputSep` | i18n/i18n.go | 保留（用于命令输出分隔线） |
| `KeyToolCall` | i18n/i18n.go | 保留（用于工具调用显示） |

---

## 四、新增开关需要添加的 i18n 键

| 新键名 | 中文值 | 英文值 |
|--------|--------|--------|
| `KeyShowLlmThinking` | "显示 LLM 思考过程: %s" | "Show LLM thinking: %s" |
| `KeyShowLlmContent` | "显示 LLM 内容: %s" | "Show LLM content: %s" |
| `KeyShowTool` | "显示工具调用名: %s" | "Show tool call name: %s" |
| `KeyShowToolInput` | "显示工具调用参数: %s" | "Show tool call input: %s" |
| `KeyShowToolOutput` | "显示工具返回数据: %s" | "Show tool call output: %s" |
| `KeyShowCommand` | "显示系统命令: %s" | "Show system command: %s" |
| `KeyShowCommandOutput` | "显示命令返回数据: %s" | "Show command output: %s" |
| `KeyCol3LlmThinking` | "show llm thinking(on\|off)" | "show llm thinking(on\|off)" |
| `KeyCol3LlmContent` | "show llm content(on\|off)" | "show llm content(on\|off)" |
| `KeyCol3Tool` | "show tool call(on\|off)" | "show tool call(on\|off)" |
| `KeyCol3ToolInput` | "show tool input(on\|off)" | "show tool input(on\|off)" |
| `KeyCol3ToolOutput` | "show tool output(on\|off)" | "show tool output(on\|off)" |
| `KeyCol3CommandOutput` | "show command output(on\|off)" | "show command output(on\|off)" |
| `KeySettingsDescLlmThinking` | "显示 LLM 思考过程" | "Show LLM thinking process" |
| `KeySettingsDescLlmContent` | "显示 LLM 返回的主要内容" | "Show LLM response content" |
| `KeySettingsDescTool` | "显示工具调用方法名" | "Show tool call method name" |
| `KeySettingsDescToolInput` | "显示工具调用输入参数" | "Show tool call input parameters" |
| `KeySettingsDescToolOutput` | "显示工具调用返回数据" | "Show tool call return data" |
| `KeySettingsDescCommandOutput` | "显示命令执行返回数据" | "Show command execution output" |
| `KeyCLIHelpShowLlmThinking` | "显示 LLM 思考过程（on/off，覆盖配置文件）" | "Show LLM thinking process (on/off, overrides config)" |
| `KeyCLIHelpShowLlmContent` | "显示 LLM 返回内容（on/off，覆盖配置文件）" | "Show LLM response content (on/off, overrides config)" |
| `KeyCLIHelpShowTool` | "显示工具调用名（on/off，覆盖配置文件）" | "Show tool call name (on/off, overrides config)" |
| `KeyCLIHelpShowToolInput` | "显示工具调用参数（on/off，覆盖配置文件）" | "Show tool call input (on/off, overrides config)" |
| `KeyCLIHelpShowToolOutput` | "显示工具返回数据（on/off，覆盖配置文件）" | "Show tool call output (on/off, overrides config)" |
| `KeyCLIHelpShowCommandOutput` | "显示命令返回数据（on/off，覆盖配置文件）" | "Show command output (on/off, overrides config)" |

---

## 五、修改计划

### 5.1 删除项
1. `config.OutputMode` 类型、常量、Parse/String 方法
2. `config.LLMConfig.OutputMode` 字段
3. `config.LLMConfig.ShowOutput` 字段
4. `agent.Agent.outputMode` 字段
5. `agent.Agent.showOutput` 字段
6. `agent.Agent.SetOutputMode()` 方法
7. `agent.Agent.SetShowOutput()` 方法
8. `cmd/settings.go` 中的 `output-mode` 和 `show-output` 处理
9. `main.go` 中的 `--output-mode` 和 `--show-output` CLI 标志
10. 相关的 i18n 键

### 5.2 新增项
1. `config.LLMConfig` 中新增 7 个 bool 字段
2. `agent.Agent` 中新增 7 个 bool 字段和 Setter 方法
3. `agent/loop.go` 中所有输出点改用新开关控制
4. `cmd/settings.go` 中新增 7 个 `.set` 子命令
5. `main.go` 中新增 7 个 CLI 标志
6. 新增对应的 i18n 键值

### 5.3 保留项
- `show-thinking` → 保留，但语义明确为 `show-llm-thinking`
- `show-command` → 保留，语义不变
- `KeyOutputTitle`, `KeyOutputSep` → 保留，用于命令输出显示
- `KeyToolCall` → 保留，用于工具调用显示

---

## 六、事件流映射

当前 Agent 发送的事件类型与目标开关的映射关系：

| 事件类型 | 当前发送条件 | 目标开关 | 备注 |
|----------|-------------|----------|------|
| `content_chunk` | 无条件 | `show-llm-content` | 流式 LLM 内容 |
| `thinking_chunk` | `showThinking` | `show-llm-thinking` | 流式 thinking |
| `content` | 无条件（非流式） | `show-llm-content` | 非流式 LLM 内容 |
| `thinking` | `showThinking` | `show-llm-thinking` | 非流式 thinking |
| `command` | `outputMode>=normal && showCommand` | `show-command` | 系统命令 |
| `output` | `outputMode>=debug && showOutput` | `show-command-output` | 命令输出 |
| `tool_call` | `outputMode>=normal` | `show-tool` | 工具调用名 |
| `tool_call`（参数） | 当前未实现 | `show-tool-input` | 工具调用参数（新增） |
| `tool_call`（返回） | 当前未实现 | `show-tool-output` | 工具返回数据（新增） |
| `error` | 无条件 | 不变 | 错误信息 |
| `done` | 无条件 | 不变 | 完成标记 |

---

## 七、注意事项

1. **向后兼容**：删除 `show-output` 和 `output-mode` 后，旧配置文件中的这两个字段会被忽略（JSON 反序列化时未知字段默认被忽略），不会导致加载失败。
2. **`show-output` 的语义拆分**：当前 `show-output` 控制的是"命令执行输出是否在 LLM 分析前显示"，新方案中由 `show-command-output` 接管。
3. **`output-mode` 的语义拆分**：
   - compact 模式 = 只显示 LLM 内容 → `show-llm-content=on`，其余全 off
   - normal 模式 = 显示 LLM 内容 + 工具名 + 命令 → `show-llm-content=on`, `show-tool=on`, `show-command=on`，其余 off
   - debug 模式 = 全部显示 → 所有开关 on
4. **工具调用参数和返回数据的显示**：当前代码中，工具调用的输入参数和返回数据没有单独的事件类型。需要新增事件类型（如 `tool_call_input` 和 `tool_call_output`）或在现有 `tool_call` 事件中扩展内容。
5. **非 execute_command 工具的输出**：当前 `output` 事件只针对 `execute_command` 工具。其他工具的返回数据通过 tool 消息直接传给 LLM，不显示给用户。新方案中 `show-tool-output` 应覆盖所有工具的返回数据。

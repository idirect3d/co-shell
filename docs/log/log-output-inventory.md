# co-shell 日志输出清单

> 最后更新: 2026-05-02
> 用途: 记录所有日志输出语句，便于 FEATURE-122 日志级别控制改造参考

---

## main.go

| 程序/方法 | 输出内容简述 | 级别 | 输出语句 |
|---|---|---|---|
| main() | 配置文件加载路径 | INFO | `log.Info("Config loaded from: %s", configPath)` |
| main() | co-shell 启动信息 | INFO | `log.Info("co-shell v%s started (workspace: %s)", version, ws.Root())` |
| main() | CLI 参数覆盖 | INFO | `log.Info("CLI overrides applied: model=%s endpoint=%s api-key=%s", flags.model, flags.endpoint, maskKey(flags.apiKey))` |
| main() | 存储初始化失败 | ERROR | `log.Error("Cannot initialize store: %v", err)` |
| main() | MCP 服务器连接失败 | WARN | `log.Warn("Cannot connect to MCP server %q: %v", serverCfg.Name, err)` |
| main() | MCP 服务器连接成功 | INFO | `log.Info("Connected to MCP server: %s", serverCfg.Name)` |
| main() | 运行 API 设置向导 | INFO | `log.Info("Running API setup wizard")` |
| main() | LLM 客户端初始化 | INFO | `log.Info("LLM client initialized: endpoint=%s model=%s llm_timeout=%ds thinking=%v reasoning_effort=%s", ...)` |
| main() | 无 API Key，使用 no-op 客户端 | WARN | `log.Warn("No API key configured, using no-op LLM client")` |
| main() | 加载调度器条目失败 | WARN | `log.Warn("Cannot load scheduler entries: %v", err)` |
| main() | 多模态图片路径设置 | INFO | `log.Info("Image paths set for multimodal input: %v", paths)` |
| main() | Agent 初始化完成 | INFO | `log.Info("Agent initialized with %d rules", len(cfg.Rules))` |
| main() | REPL 启动 | INFO | `log.Info("REPL started")` |
| main() | REPL 错误 | ERROR | `log.Error("REPL error: %v", err)` |
| showDisclaimer() | 保存免责声明失败 | WARN | `log.Warn("Cannot save disclaimer acceptance: %v", err)` |
| executeSingleCommand() | 单命令模式 | INFO | `log.Info("Single command mode: %s", input)` |
| loadSchedulerEntries() | 反序列化调度条目失败 | WARN | `log.Warn("Cannot unmarshal scheduler entry: %v", err)` |

---

## agent/loop.go

| 程序/方法 | 输出内容简述 | 级别 | 输出语句 |
|---|---|---|---|
| Run() | 保存用户消息到记忆失败 | WARN | `log.Warn("Failed to save user message to memory: %v", err)` |
| Run() | Agent 运行用户输入 | INFO | `log.Info("Agent.Run: user input: %s", userInput)` |
| Run() | LLM 调用失败 | ERROR | `log.Error("Agent.Run: LLM call failed at iteration %d: %v", iteration, err)` |
| Run() | 保存助手消息到记忆失败 | WARN | `log.Warn("Failed to save assistant message to memory: %v", err)` |
| Run() | Agent 运行完成 | INFO | `log.Info("Agent.Run: completed after %d iterations", iteration+1)` |
| Run() | 保存助手消息到记忆失败 | WARN | `log.Warn("Failed to save assistant message to memory: %v", err)` |
| Run() | 执行工具 | INFO | `log.Info("Agent.Run: executing tool %s (ID: %s)", tc.Name, tc.ID)` |
| Run() | 工具执行失败 | ERROR | `log.Error("Agent.Run: tool %s failed: %v", tc.Name, err)` |
| Run() | 达到最大迭代次数 | ERROR | `log.Error("Agent.Run: reached maximum iterations (%d)", a.maxIterations)` |
| RunStream() | 保存用户消息到记忆失败 | WARN | `log.Warn("Failed to save user message to memory: %v", err)` |
| RunStream() | Agent 流式运行用户输入 | INFO | `log.Info("Agent.RunStream: user input: %s", userInput)` |
| RunStream() | LLM 响应详情 | DEBUG | `log.Debug("Agent.RunStream: LLM response at iteration %d: content=%q, tool_calls=%d, reasoning_len=%d", ...)` |
| RunStream() | LLM 工具调用详情 | DEBUG | `log.Debug("Agent.RunStream: LLM tool call #%d: name=%q, id=%q, args=%q", ...)` |
| RunStream() | 流式错误反馈给 LLM | WARN | `log.Warn("Agent.RunStream: stream error at iteration %d: %v, feeding back to LLM", iteration, streamErr)` |
| RunStream() | 保存助手消息到记忆失败 | WARN | `log.Warn("Failed to save assistant message to memory: %v", err)` |
| RunStream() | 流式运行完成 | INFO | `log.Info("Agent.RunStream: completed after %d iterations", iteration+1)` |
| RunStream() | 保存助手消息到记忆失败 | WARN | `log.Warn("Failed to save assistant message to memory: %v", err)` |
| RunStream() | 执行工具 | INFO | `log.Info("Agent.RunStream: executing tool %s (ID: %s)", tc.Name, tc.ID)` |
| RunStream() | 工具执行失败 | ERROR | `log.Error("Agent.RunStream: tool %s failed: %v", tc.Name, execErr)` |
| RunStream() | 达到最大迭代次数 | ERROR | `log.Error("Agent.RunStream: reached maximum iterations (%d)", a.maxIterations)` |
| RunStream() | 回退到非流式 | DEBUG | `log.Debug("ChatStream not available, falling back to non-streaming: %v", err)` |
| RunStream() | 流通道关闭回退 | DEBUG | `log.Debug("Stream channel closed without Done event, falling back to non-streaming")` |

---

## agent/tools.go

| 程序/方法 | 输出内容简述 | 级别 | 输出语句 |
|---|---|---|---|
| executeToolCall() | 工具调用 | INFO | `log.Info("Tool call: %s, timeout=%s, args=%v", tc.Name, timeoutStr, args)` |
| executeToolCall() | 工具调用失败 | ERROR | `log.Error("Tool call failed: %s, error: %v", tc.Name, err)` |
| executeToolCall() | 工具调用结果 | DEBUG | `log.Debug("Tool call result: %s -> %s", tc.Name, result)` |

---

## agent/agent.go

| 程序/方法 | 输出内容简述 | 级别 | 输出语句 |
|---|---|---|---|
| SetLLMClient() | LLM 客户端运行时替换 | INFO | `log.Info("LLM client replaced at runtime")` |
| SetResultMode() | 结果模式设置 | INFO | `log.Info("Result mode set to %s, system prompt rebuilt", config.ResultModeString(mode))` |
| Reset() | Agent 历史重置 | INFO | `log.Info("Agent history reset")` |

---

## agent/command_tools.go

| 程序/方法 | 输出内容简述 | 级别 | 输出语句 |
|---|---|---|---|
| executeCommandTool() | 执行命令 | DEBUG | `log.Debug("Executing command: %s (timeout: %ds, shell: %s)", command, timeout, shell)` |
| executeCommandTool() | 命令超时 | WARN | `log.Warn("Command timed out after %d seconds: %s", timeout, command)` |
| executeCommandTool() | 命令执行失败 | ERROR | `log.Error("Command failed: %s, error: %v", command, err)` |
| executeCommandTool() | 命令完成 | DEBUG | `log.Debug("Command completed: %s (output length: %d)", command, len(output))` |
| ExecuteCommandDirectly() | 直接执行命令（带超时） | INFO | `log.Info("Direct command: %s (timeout: %ds, shell: %s)", command, int(timeout.Seconds()), shell)` |
| ExecuteCommandDirectly() | 直接命令超时 | WARN | `log.Warn("Direct command timed out: %s", command)` |
| ExecuteCommandDirectly() | 直接命令失败 | ERROR | `log.Error("Direct command failed: %s, error: %v", command, err)` |
| ExecuteCommandDirectly() | 直接命令完成 | DEBUG | `log.Debug("Direct command completed: %s (output length: %d)", command, len(output))` |
| ExecuteCommandDirectly() | 直接执行命令（无超时） | INFO | `log.Info("Direct command: %s (no timeout, shell: %s)", command, shell)` |
| ExecuteCommandDirectly() | 直接命令失败（无超时） | ERROR | `log.Error("Direct command failed: %s, error: %v", command, err)` |
| ExecuteCommandDirectly() | 直接命令完成（无超时） | DEBUG | `log.Debug("Direct command completed: %s (output length: %d)", command, len(output))` |

---

## agent/file_tools.go

| 程序/方法 | 输出内容简述 | 级别 | 输出语句 |
|---|---|---|---|
| readFileTool() | 读取文件工具调用 | DEBUG | `log.Debug("readFileTool called: args=%v", args)` |
| searchFilesTool() | 搜索文件工具调用 | DEBUG | `log.Debug("searchFilesTool called: args=%v", args)` |
| listCodeDefinitionNamesTool() | 列出代码定义工具调用 | DEBUG | `log.Debug("listCodeDefinitionNamesTool called: args=%v", args)` |
| replaceInFileTool() | 替换文件内容工具调用 | DEBUG | `log.Debug("replaceInFileTool called: args=%v", args)` |
| writeToFileTool() | 写入文件工具调用 | DEBUG | `log.Debug("writeToFileTool called: args=%v", args)` |

---

## agent/image_tools.go

| 程序/方法 | 输出内容简述 | 级别 | 输出语句 |
|---|---|---|---|
| addImagesTool() | 添加图片工具调用 | DEBUG | `log.Debug("addImagesTool called: args=%v", args)` |
| removeImagesTool() | 移除图片工具调用 | DEBUG | `log.Debug("removeImagesTool called: args=%v", args)` |
| clearImagesTool() | 清空图片工具调用 | DEBUG | `log.Debug("clearImagesTool called: args=%v", args)` |

---

## agent/memory_tools.go

| 程序/方法 | 输出内容简述 | 级别 | 输出语句 |
|---|---|---|---|
| getMemorySliceTool() | 获取记忆切片工具调用 | DEBUG | `log.Debug("getMemorySliceTool called: args=%v", args)` |
| memorySearchTool() | 记忆搜索工具调用 | DEBUG | `log.Debug("memorySearchTool called: args=%v", args)` |

---

## agent/taskplan_tools.go

| 程序/方法 | 输出内容简述 | 级别 | 输出语句 |
|---|---|---|---|
| scheduleTaskTool() | 调度任务工具调用 | DEBUG | `log.Debug("scheduleTaskTool called: args=%v", args)` |
| scheduleTaskTool() | 持久化调度条目失败 | WARN | `log.Warn("Cannot persist scheduler entries: %v", err)` |
| createTaskPlanTool() | 创建任务计划工具调用 | DEBUG | `log.Debug("createTaskPlanTool called: args=%v", args)` |
| updateTaskStepTool() | 更新任务步骤工具调用 | DEBUG | `log.Debug("updateTaskStepTool called: args=%v", args)` |
| insertTaskStepsTool() | 插入任务步骤工具调用 | DEBUG | `log.Debug("insertTaskStepsTool called: args=%v", args)` |
| removeTaskStepsTool() | 移除任务步骤工具调用 | DEBUG | `log.Debug("removeTaskStepsTool called: args=%v", args)` |
| listTaskPlansTool() | 列出任务计划工具调用 | DEBUG | `log.Debug("listTaskPlansTool called: args=%v", args)` |
| viewTaskPlanTool() | 查看任务计划工具调用 | DEBUG | `log.Debug("viewTaskPlanTool called: args=%v", args)` |
| OnScheduledTaskTriggered() | 调度任务触发 | INFO | `log.Info("Scheduled task #%d triggered: %s", entry.ID, entry.Instruction)` |
| OnScheduledTaskTriggered() | 调度任务失败 | ERROR | `log.Error("Scheduled task #%d failed: %v", entry.ID, err)` |
| OnScheduledTaskTriggered() | 调度任务完成 | INFO | `log.Info("Scheduled task #%d completed: duration=%s, exitCode=%d", ...)` |

---

## agent/subagent_tools.go

| 程序/方法 | 输出内容简述 | 级别 | 输出语句 |
|---|---|---|---|
| launchSubAgentTool() | 启动子代理工具调用 | DEBUG | `log.Debug("launchSubAgentTool called: args=%v", args)` |
| launchSubAgentTool() | 更新子代理记忆失败 | WARN | `log.Warn("Cannot update sub-agent #%d memory: %v", subID, err)` |
| launchSubAgentTool() | 保存子代理记忆失败 | WARN | `log.Warn("Cannot save sub-agent #%d memory: %v", subID, err)` |
| launchSubAgentTool() | 启动子代理 | INFO | `log.Info("Launching sub-agent #%d: workspace=%s, instruction=%s, timeout=%ds", ...)` |
| launchSubAgentTool() | 启动子代理失败 | ERROR | `log.Error("Failed to launch sub-agent #%d: %v", subID, err)` |
| launchSubAgentTool() | 子代理完成 | INFO | `log.Info("Sub-agent #%d completed: duration=%s, exitCode=%d", subID, result.Duration, result.ExitCode)` |

---

## llm/client.go

| 程序/方法 | 输出内容简述 | 级别 | 输出语句 |
|---|---|---|---|
| Chat() | LLM 请求体（脱敏） | DEBUG | `log.Debug("LLM Chat request body: %s", maskAPIKeyInRequest(string(bodyBytes)))` |
| Chat() | LLM Chat 请求 | INFO | `log.Info("LLM Chat request: POST %s, timeout=%s, model=%s, messages=%d, tools=%d", ...)` |
| Chat() | LLM Chat 请求失败 | ERROR | `log.Error("LLM Chat request failed: POST %s, error: %v", apiURL, err)` |
| Chat() | LLM Chat 响应读取失败 | ERROR | `log.Error("LLM Chat response read failed: POST %s, error: %v", apiURL, err)` |
| Chat() | LLM Chat 响应解析失败 | ERROR | `log.Error("LLM Chat response parse failed: POST %s, error: %v", apiURL, err)` |
| Chat() | LLM Chat API 错误 | ERROR | `log.Error("LLM Chat API error: POST %s, status=%d, error=%s", apiURL, resp.StatusCode, errMsg)` |
| Chat() | LLM Chat HTTP 错误 | ERROR | `log.Error("LLM Chat HTTP error: POST %s, status=%d, body=%s", apiURL, resp.StatusCode, string(respBytes))` |
| Chat() | LLM Chat 响应摘要 | DEBUG | `log.Debug("LLM Chat response: model=%s, content_len=%d, tool_calls=%d, reasoning_len=%d", ...)` |
| Chat() | LLM Chat 响应内容 | DEBUG | `log.Debug("LLM Chat response content: %s", content)` |
| Chat() | LLM Chat 推理内容 | DEBUG | `log.Debug("LLM Chat response reasoning: %s", reasoningContent)` |
| ChatStream() | LLM 流式请求体（脱敏） | DEBUG | `log.Debug("LLM ChatStream request body: %s", maskAPIKeyInRequest(string(bodyBytes)))` |
| ChatStream() | LLM 流式请求 | INFO | `log.Info("LLM ChatStream request: POST %s, model=%s, messages=%d, tools=%d", ...)` |
| ChatStream() | LLM 流式请求失败 | ERROR | `log.Error("LLM ChatStream request failed: POST %s, error: %v", c.baseURL+"/chat/completions", err)` |
| ChatStream() | LLM 流式 HTTP 错误 | ERROR | `log.Error("LLM ChatStream HTTP error: POST %s, status=%d, body=%s", ...)` |
| ChatStream() | 跳过空工具调用 | WARN | `log.Warn("ChatStream: skipping accumulated tool call with empty name or ID (name=%q, id=%q, args=%q)", ...)` |
| ListModels() | 列出模型请求 | INFO | `log.Info("LLM ListModels request: GET %s, timeout=%s", apiURL, timeoutStr)` |
| ListModels() | 列出模型请求失败 | ERROR | `log.Error("LLM ListModels request failed: GET %s, error: %v", apiURL, err)` |
| ListModels() | 列出模型 HTTP 错误 | ERROR | `log.Error("LLM ListModels HTTP error: GET %s, status=%d, body=%s", apiURL, resp.StatusCode, string(respBytes))` |
| ListModels() | 列出模型原始响应 | DEBUG | `log.Debug("LLM ListModels raw response: %s", string(respBytes))` |
| TestVisionSupport() | 视觉测试失败 | DEBUG | `log.Debug("TestVisionSupport failed for model %s: %v", c.model, err)` |
| TestVisionSupport() | 视觉测试成功 | INFO | `log.Info("TestVisionSupport succeeded for model %s", c.model)` |
| TestTextSupport() | 文本测试失败 | DEBUG | `log.Debug("TestTextSupport failed for model %s: %v", c.model, err)` |
| TestTextSupport() | 文本测试成功 | INFO | `log.Info("TestTextSupport succeeded for model %s", c.model)` |

---

## mcp/client.go

| 程序/方法 | 输出内容简述 | 级别 | 输出语句 |
|---|---|---|---|
| CallTool() | MCP 工具调用 | INFO | `log.Info("MCP CallTool: server=%s, tool=%s, args=%v", target.name, toolName, args)` |
| CallTool() | MCP 工具调用失败 | ERROR | `log.Error("MCP CallTool failed: server=%s, tool=%s, error: %v", target.name, toolName, err)` |

---

## scheduler/scheduler.go

| 程序/方法 | 输出内容简述 | 级别 | 输出语句 |
|---|---|---|---|
| Start() | 调度器启动 | INFO | `log.Info("Scheduler started")` |
| Stop() | 调度器停止 | INFO | `log.Info("Scheduler stopped")` |
| Add() | 添加调度任务 | INFO | `log.Info("Scheduled task #%d (%s): cron=%s, nextRun=%s", id, name, cronExpr, nextRun.Format(time.RFC3339))` |
| Remove() | 移除调度任务 | INFO | `log.Info("Removed scheduled task #%d", id)` |
| Pause() | 暂停调度任务 | INFO | `log.Info("Paused scheduled task #%d (%s)", id, entry.Name)` |
| Resume() | 恢复调度任务 | INFO | `log.Info("Resumed scheduled task #%d (%s)", id, entry.Name)` |
| LoadEntries() | 加载调度任务 | INFO | `log.Info("Loaded %d scheduled tasks from storage", len(entries))` |
| triggerEntry() | 触发调度任务 | INFO | `log.Info("Triggering scheduled task #%d (%s)", entry.ID, entry.Name)` |
| triggerEntry() | 调度任务完成 | INFO | `log.Info("Scheduled task #%d (%s) completed", entry.ID, entry.Name)` |

---

## wizard/wizard.go

| 程序/方法 | 输出内容简述 | 级别 | 输出语句 |
|---|---|---|---|
| testEndpoint() | 端点连通性测试 | INFO | `log.Info("Endpoint connectivity test: GET %s, timeout=10s", endpoint)` |
| testEndpoint() | 端点连通性测试失败 | ERROR | `log.Error("Endpoint connectivity test failed: GET %s, error: %v", endpoint, err)` |
| testEndpoint() | 端点连通性测试成功 | INFO | `log.Info("Endpoint connectivity test succeeded: GET %s, status=%d", endpoint, resp.StatusCode)` |
| fetchModels() | 获取模型列表 | INFO | `log.Info("Fetching models: endpoint=%s, timeout=30s", endpoint)` |
| fetchModels() | 获取模型列表失败 | ERROR | `log.Error("Fetch models failed: endpoint=%s, error: %v", endpoint, err)` |
| fetchModels() | 获取模型列表成功 | INFO | `log.Info("Fetch models succeeded: endpoint=%s, count=%d", endpoint, len(modelInfos))` |

---

## repl/repl.go

| 程序/方法 | 输出内容简述 | 级别 | 输出语句 |
|---|---|---|---|
| Run() | 加载历史失败 | WARN | `log.Warn("Cannot load history: %v", err)` |
| Run() | 加载历史条目数 | DEBUG | `log.Debug("Loaded %d history entries", len(entries))` |
| Run() | 保存历史失败 | WARN | `log.Warn("Cannot save history: %v", err)` |

---

## cmd/mcp.go

| 程序/方法 | 输出内容简述 | 级别 | 输出语句 |
|---|---|---|---|
| handleAdd() | MCP 服务器添加但连接失败 | WARN | `log.Warn("MCP server %s added to config but connection failed: %v", name, err)` |
| handleAdd() | MCP 服务器添加成功 | INFO | `log.Info("MCP server added: %s", name)` |
| handleRemove() | MCP 服务器移除但断开失败 | WARN | `log.Warn("MCP server %s removed from config but disconnect error: %v", name, err)` |
| handleRemove() | MCP 服务器移除成功 | INFO | `log.Info("MCP server removed: %s", name)` |
| handleEnable() | MCP 服务器启用但连接失败 | WARN | `log.Warn("MCP server %s enabled but connection failed: %v", name, err)` |
| handleEnable() | MCP 服务器启用成功 | INFO | `log.Info("MCP server enabled: %s", name)` |
| handleDisable() | MCP 服务器禁用但断开失败 | WARN | `log.Warn("MCP server %s disabled but disconnect error: %v", name, err)` |
| handleDisable() | MCP 服务器禁用成功 | INFO | `log.Info("MCP server disabled: %s", name)` |

---

## cmd/settings.go

| 程序/方法 | 输出内容简述 | 级别 | 输出语句 |
|---|---|---|---|
| rebuildLLMClient() | LLM 客户端重建 | INFO | `log.Info("LLM client rebuilt and replaced in agent")` |
| handleSet("api-key") | API Key 更新 | INFO | `log.Info("API key updated")` |
| handleSet("endpoint") | 端点更新 | INFO | `log.Info("Endpoint updated to %s", args[1])` |
| handleSet("model") | 模型更新 | INFO | `log.Info("Model updated to %s", args[1])` |
| handleSet("temperature") | 温度设置 | INFO | `log.Info("Temperature set to %.1f", temp)` |
| handleSet("max-tokens") | 最大令牌数设置 | INFO | `log.Info("Max tokens set to %d", tokens)` |
| handleSet("show-thinking") | 显示思考设置 | INFO | `log.Info("Show thinking set to %s", status)` |
| handleSet("show-command") | 显示命令设置 | INFO | `log.Info("Show command set to %s", status)` |
| handleSet("show-output") | 显示输出设置 | INFO | `log.Info("Show output set to %s", status)` |
| handleSet("confirm-command") | 确认命令设置 | INFO | `log.Info("Confirm command set to %s", status)` |
| handleSet("result-mode") | 结果模式设置 | INFO | `log.Info("Result mode set to %s", args[1])` |
| handleSet("max-iterations") | 最大迭代次数设置 | INFO | `log.Info("Max iterations set to %d", n)` |
| handleSet("max-retries") | 最大重试次数设置 | INFO | `log.Info("Max retries set to %d", n)` |
| handleSet("name") | Agent 名称设置 | INFO | `log.Info("Agent name set to %s", value)` |
| handleSet("description") | Agent 描述设置 | INFO | `log.Info("Agent description set to %s", value)` |
| handleSet("principles") | Agent 原则设置 | INFO | `log.Info("Agent principles set to %s", value)` |
| handleSet("vision") | 视觉支持设置 | INFO | `log.Info("Vision support set to %s", status)` |
| handleSet("context-limit") | 上下文限制设置 | INFO | `log.Info("Context limit set to %d", n)` |
| handleSet("memory-enabled") | 记忆功能设置 | INFO | `log.Info("Memory enabled set to %s", status)` |
| handleSet("plan-enabled") | 任务计划设置 | INFO | `log.Info("Plan enabled set to %s", status)` |
| handleSet("subagent-enabled") | 子代理设置 | INFO | `log.Info("SubAgent enabled set to %s", status)` |
| handleSet("output-mode") | 输出模式设置 | INFO | `log.Info("Output mode set to %s", args[1])` |
| handleSet("search-max-line-length") | 搜索行长度设置 | INFO | `log.Info("Search max line length set to %d", n)` |
| handleSet("search-max-result-bytes") | 搜索结果字节数设置 | INFO | `log.Info("Search max result bytes set to %d", n)` |
| handleSet("search-context-lines") | 搜索上下文行数设置 | INFO | `log.Info("Search context lines set to %d", n)` |
| handleSet("memory-search-max-content-len") | 记忆搜索内容长度设置 | INFO | `log.Info("Memory search max content len set to %d", n)` |
| handleSet("memory-search-max-results") | 记忆搜索结果数设置 | INFO | `log.Info("Memory search max results set to %d", n)` |
| handleSet("error-max-single-count") | 错误单类型计数设置 | INFO | `log.Info("Error max single count set to %d", n)` |
| handleSet("error-max-type-count") | 错误类型数设置 | INFO | `log.Info("Error max type count set to %d", n)` |
| handleSet("thinking-enabled") | 思考模式设置 | INFO | `log.Info("Thinking enabled set to %s", status)` |
| handleSet("reasoning-effort") | 推理努力程度设置 | INFO | `log.Info("Reasoning effort set to %s", effort)` |
| handleSet("log") | 日志开关设置 | INFO | `log.Info("Logging set to %s", status)` |

---

## taskplan/taskplan.go

| 程序/方法 | 输出内容简述 | 级别 | 输出语句 |
|---|---|---|---|
| archiveToMemory() | 归档取消的计划失败 | WARN | `log.Warn("Failed to archive cancelled plan: %v", err)` |

---

## 统计

| 级别 | 数量 |
|---|---|
| DEBUG | 30 |
| INFO | 82 |
| WARN | 28 |
| ERROR | 27 |
| **合计** | **167** |

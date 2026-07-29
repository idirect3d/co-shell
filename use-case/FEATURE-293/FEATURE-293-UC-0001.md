# FEATURE-293-UC-0001: hasToolAttempt 信号传递 — 0-tool-call 分流到 parse-error-action

## 测试目标
验证当 LLM 尝试调用工具但 XML 格式完全破损（无法被阶段1/2检测到，故未产生 `_xml_parse_error`，`ParseXMLToolCallsWithTools` 返回空列表）时，`streamLLMResponse` 返回 `hasToolAttempt=true` 信号，`RunStream` 根据该信号分流到 `parse-error-action` 而非 `no-tool-action`。

## 前置条件
- 已构建最新版 co-shell（BUILD >= 322，FEATURE-293 分支）
- 配置文件使用 XML 模式（默认）
- 启用增强输入模式（默认）

---

## UC-0001-01: hasToolAttempt 返回类型检查

### 验证点
`streamLLMResponse` 的 Go 函数签名新增 `bool` 返回值（第五个返回值）。

### 验证步骤
1. 检查 `agent/stream_response.go` 中 `streamLLMResponse` 函数的签名
2. 确认返回类型为 `(string, string, []llm.ToolCall, bool, error)`
3. 检查两个调用点（`agent/run_stream.go` 和 `agent/run.go`）已同步更新

### 验证命令
```bash
grep -n "func (a \*Agent) streamLLMResponse" agent/stream_response.go
grep -n "streamLLMResponse" agent/run_stream.go | head -5
grep -n "streamLLMResponse" agent/run.go | head -5
```

### 预期结果
- `streamLLMResponse` 函数签名包含 `bool` 返回值
- 两个调用点已适配新的签名

---

## UC-0001-02: 散乱 XML 标签被识别为 hasToolAttempt

### 验证点
当 LLM 输出内容包含散乱的 `<` 标签开头但未构成完整的 XML 工具调用（如只写了 `< 开头的内容但没有闭合标签、或者 `<read_file` 没有 `>`）时，`streamLLMResponse` 应设置 `hasToolAttempt=true`。

### 验证步骤
1. 在 `nonStreamingFallback` 路径中调用 `streamLLMResponse` 的同名方法或相同逻辑
2. 模拟 LLM 返回包含 `<write_to_file` 但未正确闭合的文本
3. 检查返回的 `hasToolAttempt` 是否为 `true`

### 预期结果
- `hasToolAttempt` 被正确判定为 `true`
- LLM 输出通过了 `knownNonToolTags` 过滤但 `tagName` 在 `isKnownToolName` 中匹配

---

## UC-0001-03: 无工具调用意图时 hasToolAttempt=false

### 验证点
当 LLM 输出纯文本回复（无任何 XML 标签），`hasToolAttempt` 应为 `false`，走原有的 `no-tool-action` 路径。

### 验证步骤
1. LLM 返回纯文本回答，无任何 `<` 字符
2. 检查返回的 `hasToolAttempt` 是否为 `false`

### 预期结果
- `hasToolAttempt = false`
- `RunStream` 仍按 `no-tool-action` 策略处理

---

## UC-0001-04: hasToolAttempt=true 时分流到 parse-error-action

### 验证点
当 `RunStream` 收到 `hasToolAttempt=true` 且 `len(toolCalls)==0` 时，不走 `no-tool-action` 分支，而是构建类似 `_xml_parse_error` 格式的错误反馈并走 `parse-error-action` 策略。

### 验证步骤
1. 设置 `:set parse-error-action prompt`
2. 让 LLM 输出包含 `<read_file` 开头但格式不完整的文本（如缺少 `>` 或参数标签）
3. 观察是否显示 "XML 格式错误：LLM 输出中包含已尝试调用工具的内容但 XML 不完整" 之类的错误提示
4. 确认走的是 `parse-error-action` 而非 `no-tool-action`

### 预期结果
- 显示类似 "检测到XML格式错误" 的提示
- 根据 `parse-error-action` 的值（exit/retry/prompt）采取对应行为
- 不会显示 "0-tool-call 处理方式: exit" 之类的 `no-tool-action` 提示

---

## UC-0001-05: hasToolAttempt 通过 nonStreamingFallback 路径

### 验证点
`nonStreamingFallback` 方法在非流式回退时也返回 `hasToolAttempt` 布尔值。

### 验证步骤
1. 查找 `agent/stream_response.go` 中的 `nonStreamingFallback` 函数定义
2. 确认其返回签名包含 `bool`
3. 确认其内部的 XML 解析逻辑也调用与 `ParseXMLToolCallsWithTools` 相同的检测代码

### 预期结果
- `nonStreamingFallback` 的签名已更新，含 `hasToolAttempt bool` 返回值
- 非流式路径也能正确定义 `hasToolAttempt`

---

## UC-0001-06: nonStreamingFallback 返回 hasToolAttempt=true

### 验证点
`nonStreamingFallback` 在非流式模式下处理 LLM 响应时，同样能检测 LLM 输出中是否包含工具调用意图。

### 验证步骤
1. 制造非流式路径（流式 API 不可用）
2. LLM 返回包含不完整 XML 工具调用的文本
3. 检查返回的 `hasToolAttempt` 是否为 `true`

### 预期结果
- `hasToolAttempt = true`
- 后续 `RunStream` 正确分流到 `parse-error-action`

---

## UC-0001-07: hasToolAttempt 与已有 `_xml_parse_error` 的兼容性

### 验证点
当阶段1/阶段2已成功检测到格式错误并生成 `_xml_parse_error` 时，这些错误仍通过 `taskInstructionCache` 传递并走原有的 XML 解析错误处理分支，`hasToolAttempt` 信号在此路径中保持 `false` 以避免双重处理。

### 验证步骤
1. 制造一个阶段1能检测到的错误：`<reed_file><path>/tmp</path></read_file>`
2. 确认走的是 XML 解析错误处理分支（line 590-689），而非 hasToolAttempt 分流
3. 确认 `hasToolAttempt` 在此路径中被正确处理

### 预期结果
- 阶段1/2 错误仍走原有的 `taskInstructionCache` 路径
- `hasToolAttempt` 不干扰已有逻辑

---

## 测试结果记录

| 用例编号 | 测试日期 | 测试人 | 结果 | 备注 |
|---|---|---|---|---|
| UC-0001-01 | | | ⬜ 待测试 | |
| UC-0001-02 | | | ⬜ 待测试 | |
| UC-0001-03 | | | ⬜ 待测试 | |
| UC-0001-04 | | | ⬜ 待测试 | |
| UC-0001-05 | | | ⬜ 待测试 | |
| UC-0001-06 | | | ⬜ 待测试 | |
| UC-0001-07 | | | ⬜ 待测试 | |
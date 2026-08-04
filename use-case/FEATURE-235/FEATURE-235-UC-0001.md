# FEATURE-235: 工具调用流式识别与实时渲染

## 背景

LLM 流式输出过程中，用户无法实时看到工具调用将写入/修改的内容，只能等待完整的 LLM 生成结束。本次为 XML 与 OpenAI 两种工具调用模式增加统一的流式识别与实时渲染：解析器逐 chunk 识别工具名与参数内容，按方法差异化渲染（写入文件展示内容、替换内容展示 diff），解析错误立即中止。

## 实现方案

### 统一 RenderOp 语义流
- 两套解析器（XML FSA / 流式 JSON Tokenizer）输出同构的 `RenderOp` 事件流
- 统一 `ToolCallRenderer` 消费 RenderOp，负责累积渲染 buffer、门控、按 chunk 追加
- `RenderToolStart / RenderParamKey / RenderValueFragment / RenderContentFragment / RenderDiffBlock / RenderToolEnd`

### 渲染规则
- 方法名首行：`show-tool` 门控，经 `EventToolCallStream` 带 emoji 引导
- 动态内容（参数/文件内容/diff）：`show-tool-input` 门控，完整展开流式追加
- XML 模式 `show-llm-content` 打开时：识别到方法调用即转入工具渲染，解析完毕再恢复裸内容流

### 错误处理
- 任一解析器检测到结构错误 → 立即 `streamCancel()` 中止 LLM 流 → 走现有 parse-error-action

## 用例列表

### UC-0001 XML 模式工具名前置显示（单测）
**前置条件**：XML 流式解析器初始化，工具列表含 write_to_file。
**步骤**：逐 chunk 送入 `<cs:write_to_file><cs:path>foo.go</cs:path>`。
**预期**：首个 RenderOp 为 `RenderToolStart{write_to_file}`；随后 `RenderParamKey{path}` + `RenderValueFragment{foo.go}`。

### UC-0002 XML 模式 write_to_file 内容实时渲染（单测）
**前置条件**：同上。
**步骤**：送入 `<cs:content>` 文本 `line1\nline2`。
**预期**：连续输出 `RenderContentFragment`，Renderer 累积 buffer 内容为完整文本。

### UC-0003 XML 模式 replace_in_file diff 支撑（单测）
**前置条件**：同上。
**步骤**：送入 `<cs:replacements><item><cs:search>a</cs:search><cs:replace>b</cs:replace></item>`。
**预期**：输出 `RenderDiffBlock{index:1, old:"a", new:"b"}`。

### UC-0004 OpenAI 模式 arguments 增量解析（单测）
**前置条件**：流式 JSON Tokenizer 初始化。
**步骤**：分块送入 `{"path":"a.go","con` + `tent":"hi"}`。
**预期**：逐字符解析出 key `path`/`content`，无"等参数完整"才输出的问题。

### UC-0005 XML 解析错误即时中止（单测）
**前置条件**：同上。
**步骤**：送入非法标签 `<cs:unknown_tool>`。
**预期**：解析器返回错误，调用方触发 streamCancel，不再处理后续 chunk。

### UC-0006 两种模式渲染输出一致（单测）
**前置条件**：同一 write_to_file 调用分别以 XML 与 OpenAI 形式分块送入。
**步骤**：对比两路 RenderOp 序列。
**预期**：渲染文本累积结果一致。

### UC-0007 llm 层 ToolCallDelta 实时通道（单测）
**前置条件**：mock SSE 流模拟 delta tool_calls。
**步骤**：ChatStream 消费 eventCh。
**预期**：每个 arguments 增量 chunk 实时收到 `StreamEventToolCallDelta`，而非流结束才收完整 ToolCall。

### UC-0008 运行时：XML 模式写入文件实时显示（运行时）
**前置条件**：toolcall-mode=xml，show-tool-input=true，LLM 调用 write_to_file 写入小文件。
**步骤**：观察流式输出。
**预期**：方法名先显示，随后文件内容随 chunk 逐行追加显示。

## 验收标准

1. `go build ./...` 通过
2. `go vet ./agent/...` 无警告
3. `go test ./agent/ -run TestToolCallStream -v` 通过
4. `go test ./repl/ -run TestRender` 渲染回归全绿（golden 不变）
5. 手动验证 XML 写入/替换、OpenAI 写入/替换、解析错误中止路径
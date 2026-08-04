# FEATURE-328: 文件写入/修改流式渲染用户体验优化

## 背景

FEATURE-235 已实现工具调用流式识别与实时渲染（XML/OpenAI 两模式统一 RenderOp 事件流 → ToolCallRenderer → EventToolCallStream）。但写入/修改类工具的渲染仍停留在"原始值原样追加"阶段，存在两个体验瓶颈：

1. **write_to_file 内容**：`content` 参数值被原样追加（`   content: line1\nline2`），无行号、无增量逐行效果，长文件等待感知明显。
2. **replace_in_file 块**：整个 search/replace 对只在 `replace` 参数结束时一次性输出一行 `   🔄 alpha ──> beta`，多行内容挤在一行，**不增量**且无 `-`/`+` 标记。

本任务：将流式预览改为 **git diff 风格**——write 内容逐行实时出现（行号 + `+`），replace 逐行输出 `-`（search）与 `+`（replace）；replace 块**指定 `start_line` 时显示真实行号，未指定则不显示**；OpenAI 模式 JSON arguments 值片段增加转义解码（`\n`→换行等），使换行渲染与 XML 模式一致。仅影响流式预览文本形状，门控（show-tool/show-tool-input）与 EventToolCallStream 通道完全保留。

## 目标显示格式

**write_to_file（content 逐行实时出现，行号从 1 递增）**：
```
⚙️ write_to_file
   path: foo.go
   content:
     1+ package main
     2+
     3+ func main() { ... }
```

**replace_in_file（search 行 `-`、replace 行 `+`；带 `start_line` 显示行号）**：
```
⚙️ replace_in_file
   path: a.go
   10- fmt.Println("old")
   11+ fmt.Println("new")
```
未指定 `start_line` 时不显示行号：
```
   - fmt.Println("old")
   + fmt.Println("new")
```

## 实现方案

### ToolCallRenderer 行式状态机（agent/toolcall_renderop.go）
- 新增行缓冲状态：write-mode 按行拆分 content，行完成即 emit `   N+ 内容`，参数结束 flush 尾行
- `start_line` 参数值累积解析；`search` 值逐行 emit `-` 行、`replace` 值**边流式边 emit `+` 行**，行号从 `start_line` 起递增（未指定则无行号）
- `replace` 结束重置块状态；普通参数保持 `   key: value`

### JSON 值转义解码（agent/toolcall_parser_json.go）
- inString 分支的 `\` 转义**解码**（`\n`/`\r`/`\t`/`\\`/`\"`/`\/`/`\b`/`\f`/`\uXXXX`），仅影响 RenderOp 渲染值，不影响原始累积

## 用例列表

### UC-0001 XML 模式 write_to_file 多行 content 逐行增量（单测）
**前置条件**：XML 流式解析器初始化，NewToolCallRenderer(true, true)。
**步骤**：逐 chunk 送入 `<cs:write_to_file>`、`<cs:path>foo.go</cs:path>`、`<cs:content>line1\n`、`line2</cs:content>`、`</cs:write_to_file>`。
**预期**：渲染文本含 `     1+ line1` 与 `     2+ line2` 两行，行号从 1 递增；`path` 参数保持 `   path: foo.go`。

### UC-0002 XML 模式 replace_in_file 未指定 start_line 无行号（单测）
**前置条件**：同上。
**步骤**：送入 `<cs:replace_in_file>`、`<cs:path>a.go</cs:path>`、`<cs:replacements>`、`<item><cs:search>alpha</cs:search><cs:replace>beta</cs:replace></item>`。
**预期**：渲染文本含 `   - alpha` 与 `   + beta`，**无行号**。

### UC-0003 XML 模式 replace_in_file 指定 start_line 显示行号（单测）
**前置条件**：同上。
**步骤**：在 search 前送入 `<cs:start_line>10</cs:start_line>`，search 值为两行 `old1\nold2`，replace 值为 `new1\nnew2`。
**预期**：渲染文本含 `   10- old1`、`   11- old2`、`   10+ new1`、`   11+ new2`，行号从 10 递增。

### UC-0004 OpenAI 模式 JSON `\n` 转义后按行渲染（单测）
**前置条件**：流式 JSON Tokenizer 初始化，SetToolName("write_to_file")。
**步骤**：分块送入 `{"path":"a.go","content":"line1\n` + `line2"}`（chunk 内含 JSON 转义的 `\n` 两字符）。
**预期**：渲染文本中 `line1` 与 `line2` 位于**不同渲染行**（`\n` 被解码为换行），行号递增；与 XML 模式渲染结果一致。

### UC-0005 XML / OpenAI 两模式渲染一致（单测）
**前置条件**：同上。
**步骤**：同一 write_to_file（含多行 content）分别以 XML 与 OpenAI 形式分块送入，对比累积渲染文本。
**预期**：两路渲染文本（去掉工具头差异后）行式结果一致——`\n` 解码与真实换行等效。

### UC-0006 门控保持：showToolInput=false 不输出内容行（单测）
**前置条件**：NewToolCallRenderer(true, false)。
**步骤**：送入 XML write_to_file 含多行 content。
**预期**：工具名头显示，content 行式内容**不输出**（门控行为不变）。

### UC-0007 运行时：XML 模式写入小文件逐行实时显示（运行时）
**前置条件**：toolcall-mode=xml，show-tool-input=true，LLM 调用 write_to_file 写入多行文件。
**步骤**：观察流式输出。
**预期**：方法名先显示，随后 content 按行实时出现，每行带递增行号与 `+`。

### UC-0008 运行时：OpenAI 模式写入小文件（运行时）
**前置条件**：toolcall-mode=openai，show-tool-input=true，LLM 调用 write_to_file 写入多行文件。
**步骤**：观察流式输出。
**预期**：与 XML 模式一致——多行 content 按行实时出现，行号递增；`\n` 转义被正确解码为换行。

### UC-0009 运行时：replace_in_file 带/不带 start_line（运行时）
**前置条件**：show-tool-input=true，LLM 先对带 start_line 的文件修改（显示行号），再对不带 start_line 的修改（无行号）。
**步骤**：观察流式输出。
**预期**：带 start_line 的 block 显示真实行号（`N-`/`N+`），不带 start_line 的 block 只显示 `-`/`+` 无行号。

## 验收标准

1. `go build ./...` 通过
2. `go vet ./agent/...` 无警告
3. `go test ./agent/ -run TestToolCallStream -v` 全部通过（含新增用例）
4. `go test ./repl/ -run TestRender` 与 `go test . -run TestRenderSingleCmdGolden` 渲染 golden 回归不变
5. 手动验证 UC-0007~UC-0009 运行时场景
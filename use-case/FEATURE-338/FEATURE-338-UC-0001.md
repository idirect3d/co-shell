# FEATURE-338: write_to_file 流式渲染行号右对齐优化

## 背景

FEATURE-328 实现 write_to_file 流式渲染后，行号前缀使用 `fmt.Sprintf("%s%d%s ", indent, baseLine+no-1, marker)` 直接拼接。当文件行数跨越不同位数（如 1~9 行后到 10 行）时，`+` 号位置随行号位数变化而错位：

**当前（+ 号错位）**：
```
     1+ <!DOCTYPE html>
     2+ <html lang="zh-CN">
     ...
     9+     <title>...</title>
    10+     <meta charset="UTF-8">   ← + 号比上面左移了
```

**期望（+ 号右对齐，同一列垂直对齐）**：
```
     1+ <!DOCTYPE html>
     2+ <html lang="zh-CN">
     ...
     9+     <title>...</title>
    10+     <meta charset="UTF-8">   ← + 号与其他行对齐
```

## 实现方案

`agent/toolcall_renderop.go` 的 `linePrefix()` 函数：当 `colon=false`（write_to_file 分支）且 `baseLine > 0` 时，将行号格式化为**固定 5 位占位右对齐**（`%5d`）——1 位数行号前补 4 空格、2 位数补 3 空格、3 位数补 2 空格、4 位数补 1 空格、5 位数不补，`+` 号始终在同一列。固定宽度避免流式输出过程中位数增长导致的列跳动。

## 用例列表

### UC-0001 write_to_file 1~9 行：单数字行号 + 号对齐（单测）
**前置条件**：XML 流式解析器初始化，NewToolCallRenderer(true, true)。
**步骤**：逐 chunk 送入 `<cs:write_to_file>`、`<cs:path>a.html</cs:path>`、`<cs:content><html>\n<head>\n<title>t</title>\n</head>\n<body></body>\n</html></cs:content>`、`</cs:write_to_file>`（共 6 行）。
**预期**：渲染文本中每行前缀为 `        1+ `、`        2+ ` … `        6+ `（5 空格 indent + 5 位占位），所有 `+` 号在**同一列**垂直对齐。

### UC-0002 write_to_file 10~99 行：双数字行号 + 号对齐（单测）
**前置条件**：同上。
**步骤**：送入 content 含 12 行（第 1~9 行 + 第 10~12 行）。
**预期**：第 1~9 行前缀为 `        1+ `…`        9+ `（行号左补 4 空格），第 10~12 行为 `       10+ `…`       12+ `（行号左补 3 空格）；所有 `+` 号在同一列。

### UC-0003 跨个位/十位边界（9→10）：+ 号列不跳动（单测）
**前置条件**：同上。
**步骤**：送入 content 含 11 行，逐行分块送入使第 9 行与第 10 行在不同的 chunk。
**预期**：渲染全过程中第 10 行 `+` 号与第 9 行 `+` 号位于同一列；增量渲染不出现第 10 行先输出、再被"回补空格"的情况（流式确定性）。

### UC-0004 write_to_file 100~999 行：三位数行号 + 号对齐（单测）
**前置条件**：同上。
**步骤**：送入 content 含 101 行（第 100、101 行触发三位数）。
**预期**：第 1~9 行前缀 `        1+ `…（行号左补 4 空格），第 10~99 行 `       10+ `…（左补 3 空格），第 100~101 行 `      100+ `…（左补 2 空格）；所有 `+` 号同一列。

### UC-0005 replace_in_file 冒号模式不受影响（单测）
**前置条件**：同上。
**步骤**：送入 `<cs:replace_in_file>`、`<cs:start_line>10</cs:start_line>`、search 两行 `old1\nold2`、replace 两行 `new1\nnew2`。
**预期**：渲染文本与 FEATURE-328 基准一致——`10-: old1`、`11-: old2`、行区间定位头（含 `10-11`）、`10+: new1`、`11+: new2`（冒号模式的 `N-:`/`N+:` 格式不改变，定位头文案跟随 i18n 语言）。

### UC-0006 门控保持：showToolInput=false 不输出内容行（单测）
**前置条件**：NewToolCallRenderer(true, false)。
**步骤**：送入 XML write_to_file 含 11 行 content（跨越 9→10 边界）。
**预期**：工具名头显示，content 行式内容不输出（门控行为不变，无回归）。

### UC-0007 运行时：XML 模式写入 10+ 行文件实时观察对齐（运行时）
**前置条件**：toolcall-mode=xml，show-tool-input=true，LLM 调用 write_to_file 写入 ≥10 行的文件。
**步骤**：观察流式输出中 content 各行前缀。
**预期**：所有 `+` 号在同一列垂直对齐；第 10 行及其后行号不改变对齐位置。

### UC-0008 运行时：OpenAI 模式写入 10+ 行文件实时观察对齐（运行时）
**前置条件**：toolcall-mode=openai，show-tool-input=true，LLM 调用 write_to_file 写入 ≥10 行文件。
**步骤**：观察流式输出。
**预期**：与 XML 模式一致，`+` 号对齐。

### UC-0009 运行时：100 行大文件对齐（运行时）
**前置条件**：show-tool-input=true，LLM 分次写入（先 new 首段再多次 append）累计 ≥100 行。
**步骤**：观察每次 append 的流式渲染。
**预期**：三位数行号下 `+` 号仍各次对齐；多次 append 的渲染均独立正确对齐。

## 验收标准

1. `go build ./...` 通过
2. `go vet ./agent/...` 无警告
3. `go test ./agent/ -run TestToolCallStream -v` 全部通过（含新增用例）
4. `go test ./agent/ ./repl/ ./i18n/` 全绿，渲染 golden 回归不变
5. 手动验证 UC-0007~UC-0009 运行时场景
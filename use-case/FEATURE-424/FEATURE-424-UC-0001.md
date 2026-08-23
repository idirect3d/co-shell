# FEATURE-424 文件写入/覆盖 diff 显示

## 背景

当前前端显示的文件修改内容，哪些是增加、哪些是删除/覆盖看不清楚。需要统一所有工作模式（write_to_file 新建/覆盖/追加、replace_in_file 指定行号/不指定行号）的 diff 显示，带行号 + 状态标记，Web UI 用颜色区分新增/删除。

## 方案

1. **后端统一渲染格式**：所有工作模式统一为 `{1空格}{5位右对齐行号}{状态}{空格}{内容}`，状态为 `+`/`-`/` `（新增/删除/不变）。如 `   257+ # 使用手册`。
2. **replace_in_file diff**：对 search/replace 做逐行 diff，相同行标 ` `（不变），仅 search 有标 `-`，仅 replace 有标 `+`。
3. **协议**：tool_call 完成后，后端发送 `tool_call_diff` 事件，携带完整的 diff 渲染文本（带状态标记），前端据此重新渲染参数子块。
4. **Web UI**：新增内容绿色、删除内容红色、不变内容默认色。

## 验收标准

1. write_to_file 的 content 统一显示为 `{1空格}{5位右对齐行号}+ {内容}`。
2. replace_in_file 的 search/replace 统一显示为 `{1空格}{5位右对齐行号}{状态} {内容}`。
3. replace_in_file 中 search 和 replace 相同的行标注为"不变"（空格），仅 search 有标 `-`，仅 replace 有标 `+`。
4. 后端发送 `tool_call_diff` 事件携带 diff 渲染文本。
5. Web UI 新增内容绿色、删除内容红色、不变内容默认色。
6. 编译通过：`go build ./... && go vet ./... && go build -o work/co-shell .`

## 测试用例

### UC-001 write_to_file 统一格式
- 前置：无
- 步骤：渲染 write_to_file 的 content（多行）
- 期望：每行显示为 `{1空格}{5位右对齐行号}+ {内容}`，如 `        1+ line1`

### UC-002 replace_in_file 统一格式（指定行号）
- 前置：无
- 步骤：渲染 replace_in_file（带 start_line）
- 期望：search 行 `{1空格}{5位右对齐行号}- {内容}`，replace 行 `{1空格}{5位右对齐行号}+ {内容}`

### UC-003 replace_in_file diff 相同行标不变
- 前置：无
- 步骤：渲染 replace_in_file，search 和 replace 有相同行
- 期望：相同行标 ` `（不变），仅 search 有标 `-`，仅 replace 有标 `+`

### UC-004 tool_call_diff 事件携带 diff 文本
- 前置：无
- 步骤：tool_call 完成后，检查后端发送的 tool_call_diff 事件
- 期望：事件携带完整的 diff 渲染文本（带状态标记）

### UC-005 Web UI 新增绿色/删除红色
- 前置：Web UI 已加载，收到 tool_call_diff 事件
- 步骤：观察参数子块渲染
- 期望：新增行绿色、删除行红色、不变行默认色

### UC-006 编译验证
- 前置：无
- 步骤：运行 `go build ./... && go vet ./... && go build -o work/co-shell .`
- 期望：全部通过，无编译错误、无 vet 告警

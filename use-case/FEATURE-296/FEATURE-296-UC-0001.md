# FEATURE-296-UC-0001: XML 工具调用外层包裹标签 `<cs_tool_calls>`

## 测试目标
验证 XML 模式下 LLM 返回的工具调用必须被 `<cs_tool_calls>...</cs_tool_calls>` 包裹才能被正确解析。

## 前置条件
- co-shell 已启动并处于 XML 模式（tool_call_mode=xml）

## 测试步骤

### TC1: 正确格式 — 有外层包裹
输入内容：
```
Here's my analysis:
<cs_tool_calls>
<execute_command>
  <intent>check directory</intent>
  <command>ls -la</command>
</execute_command>
</cs_tool_calls>
```
预期结果：成功解析 1 个 tool call（execute_command），参数包含 intent 和 command。

### TC2: 无外层包裹 — 不解析
输入内容：
```
<execute_command>
  <command>ls -la</command>
</execute_command>
```
预期结果：返回 0 个 tool call（被当作普通文本忽略）。

### TC3: 多个 wrapper 块
输入内容：
```
First task:
<cs_tool_calls>
<execute_command>
  <intent>step 1</intent>
  <command>echo hello</command>
</execute_command>
</cs_tool_calls>

Second task:
<cs_tool_calls>
<read_file>
  <intent>check file</intent>
  <path>/tmp/test.txt</path>
</read_file>
</cs_tool_calls>
```
预期结果：成功解析 2 个 tool call。

### TC4: wrapper 内代码块不解析
输入内容：
```
<cs_tool_calls>
```xml
<execute_command>
  <command>ls</command>
</execute_command>
```
</cs_tool_calls>
```
预期结果：0 个 tool call（代码块内内容被忽略）。

### TC5: 带代码块内容整体不解析
输入内容：
```
Let me show you how to use tools:
```xml
<cs_tool_calls>
<execute_command>
  <intent>example</intent>
  <command>ls</command>
</execute_command>
</cs_tool_calls>
```
```
预期结果：0 个 tool call（全部在代码块中）。

### TC6: 自定义标签名
当配置 XMLToolWrapperTag=my_wrapper 时：
输入内容：
```
<my_wrapper>
<execute_command>
  <intent>test</intent>
  <command>ls</command>
</execute_command>
</my_wrapper>
```
预期结果：成功解析 1 个 tool call。

### TC7: 自定义标签名不匹配时不解析
当配置 XMLToolWrapperTag=my_wrapper 时：
输入内容：
```
<cs_tool_calls>
<execute_command>
  <intent>test</intent>
  <command>ls</command>
</execute_command>
</cs_tool_calls>
```
预期结果：0 个 tool call。

## 验证方法
通过 `go test ./agent/ -run TestParseXMLToolCallsWithWrapper` 运行单元测试验证。
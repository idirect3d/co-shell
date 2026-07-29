# FEATURE-297-UC-0001: XML 工具调用标签添加可配置前缀

## 测试目的

验证 XML 工具调用方式从 `<cs_tool_calls>` 外框改为标签级前缀（默认 `cs:`）后，解析逻辑、配置管理、系统提示词输出是否正确。

## 前置条件

1. co-shell 已编译通过（`go build ./...`）
2. 测试使用 XML 模式（默认）

## 测试用例

### TC-1: 默认前缀 `cs:` 解析正确

**输入**（`ParseXMLToolCalls`）：
```
<cs:execute_command>
  <cs:command>ls -la</cs:command>
</cs:execute_command>
```

**预期输出**：1 个 tool call，`Name="execute_command"`，参数 `{"command": "ls -la"}`

**验证方法**：`go test -run TestParseXMLToolCalls_Prefix`

---

### TC-2: 可配置前缀

**输入**（`ParseXMLToolCalls`，前缀设为 `tool`）：
```
<tool:execute_command>
  <tool:command>ls -la</tool:command>
</tool:execute_command>
```

**预期输出**：1 个 tool call，`Name="execute_command"`，参数 `{"command": "ls -la"}`

**验证方法**：`go test -run TestParseXMLToolCalls_CustomPrefix`

---

### TC-3: 无前缀的普通 XML 标签不解析为工具调用

**输入**：
```
<execute_command>
  <command>ls -la</command>
</execute_command>
```

**预期输出**：0 个 tool call（无前缀的标签不会被解析）

**验证方法**：`go test -run TestParseXMLToolCalls_NoPrefix`

---

### TC-4: 参数标签同样使用前缀

**输入**（前缀 `cs:`）：
```
<cs:execute_command>
  <cs:command>ls -la</cs:command>
  <cs:timeout_seconds>30</cs:timeout_seconds>
</cs:execute_command>
```

**预期输出**：1 个 tool call，`Arguments` 包含 `"command"` 和 `"timeout_seconds"`

**验证方法**：`go test -run TestParseXMLToolCalls_PrefixedParams`

---

### TC-5: 混合不匹配前缀应解析不到

**输入**（标签用 `cs:`，参数无前缀）：
```
<cs:execute_command>
  <command>ls</command>
</cs:execute_command>
```

**预期输出**：0 个 tool call（参数标签也应使用前缀匹配）

**验证方法**：`go test -run TestParseXMLToolCalls_MixedPrefix`

---

### TC-6: `<cs_tool_calls>` 外框不再支持

**输入**：
```
<cs_tool_calls>
  <execute_command>
    <command>ls -la</command>
  </execute_command>
</cs_tool_calls>
```

**预期输出**：0 个 tool call（旧格式不再支持，测试向后兼容性已移除）

**验证方法**：`go test -run TestParseXMLToolCalls_OldWrapper`

---

### TC-7: 配置项 `xml-tag-prefix` 可通过 REPL 设置

**输入**：`:set xml-tag-prefix mytool`

**预期输出**：co-shell 的配置中 `xml_tag_prefix` 变为 `"mytool"`

**验证方法**：`go test -run TestParseXMLToolCalls_CustomPrefix`

---

### TC-8: 配置项 `xml-tag-prefix` 可通过命令行设置

**输入**：`co-shell --xml-tag-prefix mytool`

**预期输出**：配置初始化后 `xml_tag_prefix` 值为 `"mytool"`

**验证方法**：检查 config/config.go 中的命令行参数处理

---

### TC-9: 系统提示词中的 XML 调用示例使用前缀

**验证方法**：输出系统提示词，检查工具说明中的 XML 示例标签名是否包含 `cs:` 前缀

---

## 测试结果记录

| 用例编号 | 状态 | 备注 |
|---------|------|------|
| TC-1 | ⬜ | |
| TC-2 | ⬜ | |
| TC-3 | ⬜ | |
| TC-4 | ⬜ | |
| TC-5 | ⬜ | |
| TC-6 | ⬜ | |
| TC-7 | ⬜ | |
| TC-8 | ⬜ | |
| TC-9 | ⬜ | |
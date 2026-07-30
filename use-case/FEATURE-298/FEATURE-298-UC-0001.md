# FEATURE-298 流式 XML 工具调用增量校验 — 提前发现拼写错误

## 基本信息

| 项目 | 内容 |
|------|------|
| 任务编号 | FEATURE-298 |
| 任务名称 | 流式 XML 工具调用增量校验 |
| 类型 | 新特性 (Feature) |
| 版本 | v0.6.0 |
| 作者 | L.Shuang |

---

## 测试环境

- Go 1.22+
- XML 模式（`toolcall-mode=xml`）
- 配置项 `xml-stream-validate` = on（默认值）

---

## 测试用例

### UC-0001: 已知工具名拼写错误 → 立即中止 LLM 流

**描述**：LLM 流式输出中，开标签 `<cs:wite_to_file>` 拼写错误（应为 `cs:write_to_file`），校验器应在前几个 chunk 内检测到并终止流。

**输入**：
```
<cs:wite_to_file>
  <cs:mode>new</cs:mode>
  <cs:path>/tmp/test.txt</cs:path>
  <cs:content>Hello World</cs:content>
</cs:wite_to_file>
```

**预期结果**：
- 在读到 `<cs:wite_to_file>` 开标签后立即检测到（不等到完整输出）
- 检测到的错误："wite_to_file 不是已知工具名"
- 流被取消（cancel context），`streamLLMResponse` 返回 error
- 错误通过 `taskInstructionCache` 传递，`RunStream` 走 parse-error-action 重试
- 整个过程在收到开标签后的 1-2 个 chunk 内完成（非数百毫秒后）

**验证方法**：单元测试 `TestXMLStreamValidatorUnknownTool`

---

### UC-0002: 参数名拼写错误 → 立即中止 LLM 流

**描述**：工具名正确但参数名拼写错误（如 `cs:commmand` 而非 `cs:command`）。

**输入**：
```
<cs:execute_command>
  <cs:commmand>ls -la</cs:commmand>
</cs:execute_command>
```

**预期结果**：
- 读到内层标签 `<cs:commmand>` 时立即检测到
- 检测到的错误："commmand 不是 execute_command 的合法参数"
- 流被取消，返回 error 重试

**验证方法**：单元测试 `TestXMLStreamValidatorUnknownParam`

---

### UC-0003: 开标签与闭标签不匹配 → 立即中止 LLM 流

**描述**：开标签是 `<cs:read_file>`，但闭标签是 `</cs:execute_command>`。

**输入（流式顺序）**：
```
<cs:read_file>
  ...content...
</cs:execute_command>
```

**预期结果**：
- 读到 `</cs:execute_command>` 时检测到闭合标签名不匹配
- 检测到的错误："<cs:read_file> 与 </cs:execute_command> 标签名不匹配"
- 流被取消，返回 error 重试

**验证方法**：单元测试 `TestXMLStreamValidatorTagMismatch`

---

### UC-0004: 标签名含非法字符 `=` → 立即中止 LLM 流

**描述**：LLM 错误地将参数写为可扩展属性格式 `<cs:command=ls>`

**输入**：
```
<cs:execute_command>
  <cs:command=ls>value</cs:command=ls>
</cs:execute_command>
```

**预期结果**：
- 读到 `<cs:command=ls>` 时检测到 `=` 非法字符
- 检测到的错误："标签名包含非法字符 '='"
- 流被取消，返回 error 重试

**验证方法**：单元测试 `TestXMLStreamValidatorInvalidChar`

---

### UC-0005: 正常完整调用 → 不触发任何错误

**描述**：LLM 输出正确的完整 XML 工具调用，校验器不应触发任何错误。

**输入**：
```
<cs:read_file>
  <cs:path>/tmp/test.txt</cs:path>
  <cs:start_line>1</cs:start_line>
  <cs:end_line>50</cs:end_line>
</cs:read_file>
```

**预期结果**：
- 流式处理全程不返回 error
- 校验器通过后不影响最终内容
- 最终解析结果与原有 `ParseXMLToolCallsWithTools` 完全一致

**验证方法**：单元测试 `TestXMLStreamValidatorValidCall`

---

### UC-0006: 不完整标签（跨 chunk 分割）→ 不产生错误

**描述**：标签名被分割在两个 chunk 中（如 chunk1:`<cs:execute_co` chunk2:`mmand>`），校验器应能拼接后正确处理。

**输入（chunks）**：
```
chunk1: "我正在执行命令\n<cs:execute_co"
chunk2: "mmand>\n<cs:command>ls</cs:command>\n</cs:execute_command>"
```

**预期结果**：
- 不触发任何错误
- chunk1 中的 `<cs:execute_co` 作为 partial buffer 暂存
- chunk2 开头拼接后形成完整标签 `<cs:execute_command>`
- 最终被正确解析为工具调用

**验证方法**：单元测试 `TestXMLStreamValidatorSplitTag`

---

### UC-0007: 非工具标签（HTML 内容）→ 忽略

**描述**：LLM 在说明文字中包含 HTML 标签或其他非工具标签。

**输入**：
```
让我先读取文件内容。<div class="main">这是说明文字</div>
<cs:read_file>
  <cs:path>/tmp/test.txt</cs:path>
</cs:read_file>
```

**预期结果**：
- `<div>`、`</div>` 等无前缀标签被完全忽略
- `<cs:read_file>` 被正确识别并验证
- 全程无错误触发

**验证方法**：单元测试 `TestXMLStreamValidatorNonToolTags`

---

### UC-0008: CDATA 内容中的 XML 标签 → 不验证内部

**描述**：参数值中包含 CDATA 包裹的 XML 示例文本。

**输入**：
```
<cs:write_to_file>
  <cs:path>/tmp/example.xml</cs:path>
  <cs:content><![CDATA[<root><item>test</item></root>]]></cs:content>
</cs:write_to_file>
```

**预期结果**：
- CDATA 内容中的 `<root>`、`<item>` 等标签被跳过
- 不触发未知标签错误
- 全程无错误

**验证方法**：单元测试 `TestXMLStreamValidatorCDATA`

---

### UC-0009: 长 write_to_file 在开头有拼写错误 → 提前终止

**描述**：LLM 在写大文件时，调用方法名为 `<cs:wite_to_file>`（拼写错误）。原本需要等待数千行内容完全输出后才能发现错误。校验器应在方法名出现时立即终止。

**输入（模拟大文件）**：
```
<cs:wite_to_file>
  <cs:mode>new</cs:mode>
  <cs:path>/tmp/large.txt</cs:path>
  <cs:content>[在此处开始输出数千行文本...]
```

**预期结果**：
- 在读完 `<cs:wite_to_file>` 后的 1-2 个 chunk（通常 50-200ms）内检测到未知工具名
- 流被立即取消，不等待后续数千行内容输出完成
- 相比原来节省约 99% 的 token 和时间

**验证方法**：模拟流式 chunk 序列，前 2 个 chunk 中包含错误工具名，验证流在 2 个 chunk 内被终止

---

### UC-0010: 嵌套不匹配 — 子标签未闭合错当父标签闭合 → 中止

**描述**：LLM 输出中，子标签未闭合而父标签提前闭合（如缺少 `</cs:path>` 但出现了 `</cs:read_file>`）。

**输入**：
```
<cs:read_file>
  <cs:path>/tmp/test.txt
</cs:read_file>
```

**预期结果**：
- 读到 `</cs:read_file>` 但栈顶是 `<cs:path>` → 检测到不匹配
- 错误信息明确指示："<cs:path> 还未闭合但遇到了 </cs:read_file>"
- 流被取消，返回 error 重试

**验证方法**：单元测试 `TestXMLStreamValidatorNestedMismatch`

---

## 技术验证清单

### 单元测试覆盖

- [ ] `TestXMLStreamValidatorUnknownTool` — 未知工具名
- [ ] `TestXMLStreamValidatorUnknownParam` — 未知参数名
- [ ] `TestXMLStreamValidatorTagMismatch` — 标签不匹配
- [ ] `TestXMLStreamValidatorInvalidChar` — 非法字符
- [ ] `TestXMLStreamValidatorValidCall` — 正常调用无错误
- [ ] `TestXMLStreamValidatorSplitTag` — 标签跨 chunk 分割
- [ ] `TestXMLStreamValidatorNonToolTags` — 忽略非工具标签
- [ ] `TestXMLStreamValidatorCDATA` — CDATA 内容跳过验证
- [ ] `TestXMLStreamValidatorNestedMismatch` — 嵌套标签不匹配
- [ ] `TestXMLStreamValidatorLargeFileEarly` — 大文件中提前发现错误

### 集成测试

- [ ] streamLLMResponse 中集成校验器后，验证错误通过 taskInstructionCache 正确传递到 RunStream
- [ ] 验证 cancel context 机制生效，后台 goroutine 停止
- [ ] 验证非致命错误（仅在流结束后报告）与致命错误（立即终止）行为不同

### 边界条件验证

- [ ] 空 chunk（`""`）不破坏状态
- [ ] 纯空白 chunk（`\n\n  \n\n`）不破坏状态
- [ ] 超大 chunk（10KB+）正确分批处理
- [ ] 一次收到完整 XML 时行为与增量一致
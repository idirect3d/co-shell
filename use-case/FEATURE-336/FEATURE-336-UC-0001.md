# FEATURE-336 测试用例 — 工具调用解析错误显示原报文

## 相关文件
- `config/config.go` — 新增 `ShowParseErrorRaw bool` 配置字段
- `agent/stream_response.go` — 解析错误时携带 raw 报文
- `agent/loop.go` — nonStreamingFallback 携带 raw
- `agent/run_stream.go` — 展示 raw 报文
- `agent/run.go` — 非流式路径展示 raw
- `agent/simulate.go` — 模拟路径展示 raw
- `cmd/settings_agent.go` / `cmd/settings.go` / `cmd/config.go` / `main.go` — 开关四通道配置
- `i18n/keys.go` / `i18n/zh.go` / `i18n/en.go` — KeyXMLParseErrorRaw
- `usage.go` / `USAGE.md` — 帮助文档

## 前置条件
- 编译通过：`go build ./...`
- 打开 show-parse-error-raw 开关

## 运行时用例

### UC-0001: XML 流式解析错误显示原报文
1. 在 XML 模式下让 LLM 返回含未闭合 `</cs:tool>` 的调用（如缺少 `<cs:content>` 闭合标签的 write_to_file）
2. 观察控制台输出
3. **预期**：出现 "解析错误原始报文:" 提示，后面原样打印 LLM 返回的 XML 报文（含错误内容）

### UC-0002: XML 流式校验错误显示原报文
1. 在 XML 模式下让 LLM 返回拼写错误的工具名（如 `<cs:write_to_flie>`）
2. 观察控制台输出
3. **预期**：流式校验器中断后，控制台显示错误摘要 + 原始报文（含该不认识的标签）

### UC-0003: OpenAI 模式 JSON 参数解析错误显示原报文
1. 在 OpenAI 模式下让 LLM 返回参数 JSON 不完整/多余右括号的工具调用
2. 观察控制台输出
3. **预期**：控制台显示错误摘要 + 已累积的参数 JSON 原文

### UC-0004: 非流式（fallback）路径解析错误显示原报文
1. 关闭流式（或人为触发 nonStreamingFallback）
2. 在 XML 模式下让 LLM 返回 `_xml_parse_error` 内容
3. 观察控制台输出
4. **预期**：错误摘要 + 完整原始响应内容

### UC-0005: 开关关闭时不显示原报文
1. `:set show-parse-error-raw off`
2. 重复 UC-0001 场景
3. **预期**：只显示错误摘要，不显示原报文

### UC-0006: 开关四通道配置
1. `:set show-parse-error-raw on` → 立即生效
2. `:config` → 「显示与输出」分组出现 `show-parse-error-raw` 选项，可选择 on/off
3. `--show-parse-error-raw on` 命令行启动 → 生效
4. config.json 中 `llm.show_parse_error_raw` 持久化
5. **预期**：三种通道均能控制开关，`:set` 列表显示该参数

### UC-0007: `:set defaults` 恢复默认
1. `:set defaults` 后 `:set` 查看 `show-parse-error-raw`
2. **预期**：恢复为 off（默认）

### UC-0008: `exit` 分支也显示原报文
1. `:set parse-error-action exit`
2. 重复 UC-0001 场景
3. **预期**：报错退出前也显示原始报文

## 单元测试（替代回归验证）
- `agent/toolcall_stream_test.go` — 解析错误携带 raw 字段的 JSON 构造验证
- `cmd/settings_external_test.go` 或新增 — ShowParseErrorRaw 默认值/重置验证
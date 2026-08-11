# FEATURE-347: log=debug 时第一时间输出原始 LLM chunk

## 背景

1. `llm/client.go` ChatStream 读循环（`for { line, err := reader.Read(); ... }`）中，`reader.Read()` 拿到的原始 SSE 行（含 `data: ` 前缀）在 `parseSSELine(line)` **之前没有任何日志输出**；解析失败的行、`[DONE]`、心跳/空行全部被静默 `continue` 丢弃，事后无法从日志还原当时 LLM/代理到底发来了什么。
2. 现有两条日志通道都不是"绝对原始"：
   - 主日志（`co-shell-*.log`）DEBUG 级只有 `log.Raw("%s", event.Content)`，输出的是**解析后**的 content 字段；
   - 交互日志（`llm-interaction-*.log`）的 `RESP][assistant` / `RESP][tool_calls` 是按字段**重组拼接**的内容流。
3. 实际排障痛点：co-flow 2026-08-11 单日 132 次 `cannot parse tool arguments: unexpected end of JSON input`（deepseek-v4-flash 在高上下文下工具参数 JSON 在 `"replacements":` 处被截断）。交互日志只能看到重组后的半截 `RESP][tool_calls`，无法确认是"模型没发完"还是"co-shell 解析/重组丢了内容"。

## 目标

在 `log level=debug` 时，**第一时间**输出 LLM/代理返回的原始 chunk：

- 流式 `ChatStream`：在 `parseSSELine` 之前，逐行输出每条原始 SSE 行（含 `data: ` 前缀、剥离换行符后的完整行），覆盖正常 `data:` 行、`[DONE]`、以及当前被静默丢弃的**解析失败行**；
- SSE 事件分隔符（空行）跳过，避免刷屏；
- 非流式 `Chat`：DEBUG 级同步输出原始响应体（`respBytes`）；
- 非 debug 级别零输出，行为不变；
- 输出不带入 API key 等敏感信息（响应流天然不含 key，需单测守护）。

## 用例

### S1 流式 ChatStream 原始 chunk 输出（单测：mock SSE via httptest）

前置：`go build ./...` 通过；单测用 `t.TempDir()` + `workspace.New` + `log.Init` + `log.SetLevel`，mock HTTP SSE 流。

| 编号 | 场景 | 操作 | 预期结果 |
|------|------|------|---------|
| UC-0001 | debug 级别输出原始 data 行 | mock SSE 返回 `data: {"choices":[{"delta":{"content":"你好"}}]}` + `[DONE]`；`log.SetLevel(debug)` 后调用 `ChatStream` 并消费完事件 | 日志文件出现 `LLM ChatStream raw chunk: data: {"choices":...}`，内容与 mock 的原始行**逐字节一致** |
| UC-0002 | 非 debug 级别不输出 | 同 UC-0001 但 `log.SetLevel(info)` | 日志文件**不含** `LLM ChatStream raw chunk:` |
| UC-0003 | `[DONE]` 信号行输出 | mock SSE 流末尾返回 `data: [DONE]` | 日志出现 `LLM ChatStream raw chunk: [DONE]` |
| UC-0004 | 空行（事件分隔符）跳过 | mock SSE 每帧后带一个空行 | 日志**不出现**裸空行对应的 `raw chunk:` 空记录；总输出行数 = 非空 data 行数 + 1（[DONE]） |
| UC-0005 | 解析失败的行仍输出 | mock SSE 返回非法 data 行（如 `data: {bad json`）后再跟正常行 | 非法行**也**出现在日志中（`LLM ChatStream raw chunk: data: {bad json`），且后续正常行不受影响 |
| UC-0006 | 原始行保留 `data: ` 前缀、无末尾换行 | 断言日志中每行 raw chunk 内容 | 每行以 `data: ` 开头；行内不含 `\n` / `\r` |
| UC-0007 | 混合帧顺序正确 | mock SSE 依次发 content 帧 → tool_calls delta 帧 → `[DONE]` | 日志按接收顺序逐条输出，与发送顺序一致 |

### S2 非流式 Chat 原始响应输出（单测：mock HTTP）

| 编号 | 场景 | 操作 | 预期结果 |
|------|------|------|---------|
| UC-0008 | debug 级别输出原始响应体 | mock HTTP 返回 `{"choices":[{"message":{"content":"ok"}}]}`；debug 级调用 `Chat` | 日志出现 `LLM Chat raw response: {"choices":...}`（原始 respBytes） |
| UC-0009 | 非 debug 级别不输出响应体 | 同 UC-0008 但 info 级 | 日志不含 `LLM Chat raw response:` |

### S3 级别切换与安全

| 编号 | 场景 | 操作 | 预期结果 |
|------|------|------|---------|
| UC-0010 | 运行时 SetLevel 切换随动 | debug 级调 ChatStream 后 SetLevel(info)，再调一次 | 第一次输出 raw chunk，第二次不输出 |
| UC-0011 | 原始 chunk 不泄露 API key | mock SSE 的 data 行内容包含任意文本（不含 key）；client 的 apiKey 设为 `sk-SECRET123` | 日志所有 `raw chunk` 行不包含 `sk-SECRET123` |
| UC-0012 | 空行/`[DONE]` 之后的正常帧仍输出 | 帧序：空行 → `[DONE]` → 正常 data 行 | 空行跳过、`[DONE]` 与正常 data 行均输出 |
| UC-0013 | raw chunk 行不被裸 content 粘连 | mock SSE 返回多帧 content（如 `你好`、`世界`），debug 级消费完整流 | 日志中每条 `LLM ChatStream raw chunk:` 行都以 `[时间戳] [DEBUG] ` 完整前缀开头，行间**无**裸 content 文本插入（legacy log.Raw 已移除） |

## 验证方式

1. 单测：`go test ./llm/ -run TestFEATURE347 -v` 全绿（覆盖 UC-0001~0013）；
2. 回归：`go build ./... && go vet ./...`；`go test ./llm/ ./log/` 全绿；
3. 手动验证：`log-level=debug` 启动 co-shell，向 LLM 发起一次对话，确认 `log/co-shell-*.log` 中出现 `LLM ChatStream raw chunk: data: ...` 逐帧实时输出（每帧一行、时间戳逐帧递增、行间无裸文本），`[DONE]` 结尾；`log-level=info` 时无该输出。

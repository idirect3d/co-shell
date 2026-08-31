# FEATURE-460-UC-0002 监督场景 SUP·监督 流式显示

## 前置条件
- 已编译最新可执行码（`go build -o work/co-shell .`）
- 监督 LLM 启用（`supervisor.enabled=true`）
- `show-sup-prompt=on`、`show-sup-stream=on`
- 使用 Web UI（`--serve`）观察输出

## 操作步骤
1. 开启 `show-sup-prompt` 与 `show-sup-stream`
2. 让主 LLM 完成一个任务并调用 `attempt_completion`，触发监督 LLM 审查
3. 观察 Web UI 输出区

## 预期结果
- 监督 LLM 审查时，Web UI 出现标题为 **SUP·监督** 的 SUP 块
- 该 SUP 块先显示发送给监督 LLM 的 prompt 内容
- 随后流式显示监督 LLM 的回复（逐字/逐块出现）
- 监督 LLM 回复结束后，SUP 块内容完整（含结论/理由/建议）
- 原有监督最终报告（SUP 块）仍正常显示

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败

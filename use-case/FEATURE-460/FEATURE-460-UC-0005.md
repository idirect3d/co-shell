# FEATURE-460-UC-0005 开关关闭时不显示 SUP 流式内容

## 前置条件
- 已编译最新可执行码（`go build -o work/co-shell .`）
- 监督 LLM 启用、问题解决机制启用、循环判定启用
- `show-sup-prompt=off`、`show-sup-stream=off`（默认值）
- 使用 Web UI（`--serve`）观察输出

## 操作步骤
1. 保持 `show-sup-prompt` 与 `show-sup-stream` 均为 off（默认）
2. 让主 LLM 完成一个任务并调用 `attempt_completion`，触发监督 LLM 审查
3. 构造一个触发问题解决/循环判定的场景
4. 观察 Web UI 输出区

## 预期结果
- 三个场景（监督/问题解决/循环判定）均**不**出现流式 SUP 块
- 不显示发送给 LLM 的 prompt 内容
- 不流式显示 LLM 回复
- 原有功能完全不受影响：监督最终报告（SUP 块）仍正常显示，问题解决/循环判定的处理逻辑仍正常执行
- 结构化解析（report_problem / submit_review）仍基于流式累积结果正常工作

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败

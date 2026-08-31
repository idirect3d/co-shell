# FEATURE-460-UC-0003 问题解决场景 SUP·问题解决 流式显示

## 前置条件
- 已编译最新可执行码（`go build -o work/co-shell .`）
- 问题解决机制启用（`problem-solver-enabled=true`）
- `show-sup-prompt=on`、`show-sup-stream=on`
- 使用 Web UI（`--serve`）观察输出

## 操作步骤
1. 开启 `show-sup-prompt` 与 `show-sup-stream`
2. 构造一个触发问题解决机制的场景（如工具格式错误、上下文溢出、连接错误等）
3. 观察 Web UI 输出区

## 预期结果
- 问题解决模型被调用时，Web UI 出现标题为 **SUP·问题解决** 的 SUP 块
- 该 SUP 块先显示发送给问题解决模型的 prompt 内容
- 随后流式显示问题解决模型的回复
- 问题解决模型返回 report_problem 结构化结果后，原有处理逻辑（反馈/重试/通知用户）仍正常执行

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败

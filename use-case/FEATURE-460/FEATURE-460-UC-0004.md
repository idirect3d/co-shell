# FEATURE-460-UC-0004 循环判定场景 SUP·循环判定 流式显示

## 前置条件
- 已编译最新可执行码（`go build -o work/co-shell .`）
- 循环判定启用（`loop-judge-enabled=true`）且问题解决机制启用
- `show-sup-prompt=on`、`show-sup-stream=on`
- 使用 Web UI（`--serve`）观察输出

## 操作步骤
1. 开启 `show-sup-prompt` 与 `show-sup-stream`
2. 构造一个触发循环判定的场景（让 LLM 输出重复内容触发循环检测）
3. 观察 Web UI 输出区

## 预期结果
- 循环判定模型被调用时，Web UI 出现标题为 **SUP·循环判定** 的 SUP 块
- 该 SUP 块先显示发送给循环判定模型的 prompt 内容
- 随后流式显示循环判定模型的回复
- 循环判定返回 report_problem 结构化结果后，原有循环干预逻辑（反馈/重试/重组上下文）仍正常执行

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败

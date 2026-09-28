# FEATURE-567-UC-0002 义务在 environment_details 中可见

- 分支：FEATURE-567（v0.81.0）｜类型：单元/运行时结合
- 目的：验证未回复义务每轮出现在 `<environment_details>` 的 `<board_replies>` 段，无义务时整段省略。

## 步骤
1. A 发一条需要回复的消息给 B（`timeout_seconds=300`）。
2. 检查 B 下一次 LLM 调用上下文中的 `<environment_details>`。
3. B 回复该消息后，再检查下一次 LLM 调用的上下文。

## 期望
- 步骤 2：出现 `<board_replies pending="1">`，含 `<reply_to id="m-x" from="A" deadline_in="..." redelivery="0"/>`。
- 步骤 3：`<board_replies>` 段整体消失（不输出空段）。
- 多条义务时 `pending` 数值与条目数一致。

## 实测记录
- 时间 / 实例 / BUILD：
- 结果：

# FEATURE-567-UC-0003 未回复义务阻止 attempt_completion（含防死锁放行）

- 分支：FEATURE-567（v0.81.0）｜类型：单元 + 运行时
- 目的：验证门禁：有未回复义务时 `attempt_completion` 被拒绝；义务清除后放行；同一义务连续被拦 3 次后放行。

## 步骤
1. B 收到需要回复的消息且未回复，直接调用 `attempt_completion`，观察返回与迭代是否继续。
2. B 用 `board_send(reply_to=..., state="handled")` 回复后再调 `attempt_completion`。
3. 另起一例：B 连续 3 次尝试 `attempt_completion` 均不回复。

## 期望
- 步骤 1：返回结构化错误（非成功），文案含待回复的 `message_id`、来源 agent 与剩余时间，并指明用 `board_send(reply_to=..., state=handled|declined|need_info)` 回复；中英双语随语言设置。
- 步骤 2：`attempt_completion` 正常完成。
- 步骤 3：第 3 次后放行完成，日志出现一条 warn（防死锁降级），义务仍保留在 `<board_replies>` 中可见。
- `board-must-reply=off` 时步骤 1 直接放行（仅软提醒）。

## 实测记录
- 时间 / 实例 / BUILD：
- 结果：

# FEATURE-567-UC-0005 双 agent 端到端：忘回被拦 → 补回；全程不回 → 重投后 timeout

- 分支：FEATURE-567（v0.81.0）｜类型：运行时（隔离实例 + 真实 hub）
- 目的：端到端验证「必须回复」链路在真实运行下成立。

## 场景 A：接收方忘记回复但被拦住
1. 启动隔离实例（`--serve --port <隔离端口>`），A、B 两个 agent 均开 board。
2. A 调 `board_send(to=["B"], content="请把 X 做掉并回复 handled", timeout_seconds=300)`。
3. B 收到后先去执行任务（不做回复），任务完成后直接尝试收尾。
4. 观察 B 是否被拦，随后 B 补回回复。

## 场景 B：接收方全程不回
5. A 再发一条需要回复的消息（`timeout_seconds=5`，缩短窗口便于观察）。
6. B 不回复；观察 hub 是否重投一次、A 是否收到 `board_timeout`（含 redelivered 次数）。

## 期望
- 场景 A：B 的收尾被拒绝（错误含消息 id）；补发 `board_send(reply_to=..., state="handled")` 后收尾成功；A 侧看到该条消息 `state=handled`。
- 场景 B：B 侧出现第二份消息（`redelivery=1`）；窗口再次到期后 A 收到 `board_timeout`，payload 表明已重投 1 次。

## 实测记录
- 时间 / 实例 pid / BUILD：
- 场景 A 结果：
- 场景 B 结果：

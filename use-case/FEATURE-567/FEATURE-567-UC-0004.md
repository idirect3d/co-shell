# FEATURE-567-UC-0004 hub 超时重投与旧行为回归

- 分支：FEATURE-567（v0.81.0）｜类型：hub 单元测试
- 目的：验证回复窗口到期后自动重投一次，重投仍无回复才判 timeout；`board_redeliver_max=0` 时与旧行为一致。

## 步骤（board_redeliver_max=1）
1. A 发需要回复的消息给在线 B，`timeout_seconds=5`。
2. 推进时钟越过 deadline，调用 `SweepTimeouts()`。
3. 检查 B 是否收到第二份 `board_message`、`Awaiting` 状态与发送方事件。
4. 再次推进时钟越过新 deadline，再调 `SweepTimeouts()`。

## 期望
- 步骤 3：B 收到同 id 的第二份消息（带 `redelivery="1"`），`Awaiting.Status` 仍为 `pending`、`Redeliveries=1`、deadline 已重置（≤300s）；发送方没有收到 `board_timeout`。
- 步骤 4：`Awaiting.Status=timeout`，发送方收到一条 `board_timeout`（payload 含已重投次数）。
- 回归（`board_redeliver_max=0`）：首次到期即 `timeout` 并通知发送方，字段与旧版本一致。
- `.db` 中旧 `awaiting` 记录（无 `Redeliveries` 字段）反序列化后按 0 处理，不报错。

## 实测记录
- 时间 / 实例 / BUILD：
- 结果：

# FEATURE-567-UC-0001 回复义务的登记、清除与免回复

- 分支：FEATURE-567（v0.81.0）｜类型：单元/运行时结合
- 目的：验证「需要回复」的 board 消息在接收方产生回复义务，回复后清除，`no_reply=true` 不产生义务。

## 步骤
1. A 调 `board_send(to=["B"], content="请处理 X", timeout_seconds=60)`（不写 `no_reply`）。
2. 查 B 侧 `PendingBoardObligations()`。
3. B 调 `board_send(to=["A"], content="done", reply_to=<消息 id>, state="handled", timeout_seconds=60)`。
4. 再查 B 侧义务表与 hub `Pending(<消息 id>)`。
5. A 再发一条 `no_reply=true` 的消息，查 B 侧义务数。

## 期望
- 步骤 2：义务数 = 1，字段含 `message_id / from=A / deadline`。
- 步骤 4：义务数 = 0；hub `Pending` 状态为 `answered`。
- 步骤 5：义务数仍为 0（FYI 不产生义务）。

## 实测记录
- 时间 / 实例 / BUILD：
- 结果：

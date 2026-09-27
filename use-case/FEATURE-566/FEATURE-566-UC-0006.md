# FEATURE-566 运行时用例 UC-0006：主 agent 主动 kill → 不产生「完成」通知

## 目标

验证由主 agent 通过 `kill_sub_agent` 主动终止的子代理**不触发完成通知**（避免与"任务结束"混淆、避免自我打扰）；被杀走独立的状态语义（Killed）。

## 前置条件

- `work/` 下已构建本分支可执行码（v0.80.0）；以 `--serve --port <port>` 启动，Web UI 已连接。

## 步骤

1. 输入：「用 launch_sub_agent 启动 A1，指令为 `sleep 300`，timeout_seconds 设 3」。
   - 预期：3 秒后返回"等待超时 —— 进程已转入后台继续运行"。
2. 让回合结束、保持空闲，等待 10 秒确认**没有**通知（A1 仍在跑）。
   - 预期：无通知（进程仍在运行，不是终态）。
3. 输入：「用 kill_sub_agent 结束 A1」。
   - 预期：`kill_sub_agent` 返回已终止；A1 状态为 Killed、exit code -1。
4. 观察 kill 之后 ~30 秒内会话流。
   - 预期：**不出现**"子代理完成"通知，也**不因该 kill 自动起新迭代**（kill 的结果已由工具调用同步返回）。
5. 反向确认：随后再启动 A2（`sleep 5`）并让其自然结束，通知应正常出现。

## 预期结果

- 通知只覆盖"自然终态"（Completed / Failed）；Killed 不通知。
- 若实现选择"Killed 也通知"，必须在结论中记录差异并回归确认不会造成循环打扰（kill 通知 → 新迭代 → 又…）。

## 验证方式

- Web UI 与后端日志：kill 前后无完成通知记录。
- 单元测试补充：`resolveKillTargets`/`killTargets` 路径下完成回调**不被调用**；`reap` 中 `Status=Killed` 分支不触发通知。

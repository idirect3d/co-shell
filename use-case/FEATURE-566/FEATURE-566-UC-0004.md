# FEATURE-566 运行时用例 UC-0004：launch 等待超时转后台后完成 → 通知仍然送达（不吞）

## 目标

验证「`launch_sub_agent` 等待窗口超时（`WaitTimedOut=true`，进程转入后台）→ 之后子代理才结束」这一路径下，完成通知**仍然会送达**主 agent；即超时不吞掉通知，也不依赖主 agent 主动轮询。

## 前置条件

- 工作区为 co-shell 仓库，`work/` 下已构建本分支可执行码（v0.80.0）。
- 以 `--serve --port <port>` 启动，Web UI 已连接。

## 步骤

1. 输入：「用 launch_sub_agent 启动 A1，指令为 `sleep 30`，timeout_seconds 设 5，然后本轮就结束」。
   - 预期：工具返回"等待超时 —— 进程已转入后台继续运行"；进程表行 `WaitTimedOut=true`、状态 running。
2. 回合结束后**不要**输入任何内容，等待 30 秒让 A1 结束。
   - 预期：A1 结束时触发通知 → 因主 agent 空闲，自动起一次新迭代（同 UC-0001）。
3. 在新迭代中确认进程表终态。
   - 预期：`sub_agent_status` 显示 A1 状态 Completed、`WaitTimedOut` 已清除（`reap` 会把该标记复位为 false）、有退出码与耗时。
4. 反向验证：在步骤 2 的等待期间**手动 kill** A1（用 `kill_sub_agent`），重复观察。
   - 预期：kill 路径见 UC-0006，不产生"完成"通知。

## 预期结果

- 超时转后台的子代理结束时，通知不丢失；不要求主 agent 主动轮询。
- `WaitTimedOut` 是"等待窗口历史"，进程结束后应被清除，不得让用户/模型误判仍在等待。
- 通知正文能体现"该子代理之前是超时转后台的"（若实现保留该信息），便于主 agent 理解延迟。

## 验证方式

- 观察通知出现的时间点 ≈ A1 结束时刻（而非 launch 超时时刻）。
- `sub_agent_status` 的终态与通知一致。
- 单元测试补充：`startOne/waitBatch` 超时后 `done` 通道关闭仍会触发完成回调（用短 sleep 的测试替身进程验证）。

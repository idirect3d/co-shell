# FEATURE-566 运行时用例 UC-0001：子代理正常完成 → 主 agent 空闲时自动起一次新迭代

## 目标

验证子代理正常结束（exit code 0，Status=Completed）时，若**主 agent 当前空闲（没有正在运行的迭代）**，系统会主动发送一条**正式 user message 并启动一次新的迭代**，把「子代理已完成」送达主 agent；主 agent 不需要用户再说话就能感知并继续处理。

## 前置条件

- 工作区为 co-shell 仓库，`work/` 下已构建本分支可执行码（v0.80.0）。
- 以 `--serve --port <port>` 启动（Web UI 才能观察到"新迭代"），并打开该端口页面。
- `config.json` 未禁用子代理（`max_sub_agents >= 1`）。

## 步骤

1. 在 Web UI 输入：「用 launch_sub_agent 启动 A1，指令为 `sleep 20`，timeout_seconds 设 5」。
   - 预期：主 agent 调用 `launch_sub_agent`，5 秒后返回"等待超时 —— 进程已转入后台继续运行"（`WaitTimedOut`）。
2. 让该回合自然结束（主 agent 输出结束语，`IsBusy()` 回到 false），**期间不做任何输入**。
3. 等待约 15 秒（A1 的 `sleep 20` 结束）。
   - 预期：会话流中新出现一条**用户消息样式**的通知（内容见 UC-0005 格式），并**自动开始一次新的 LLM 迭代**（无需用户输入）。
   - 预期：新迭代的 `<environment_details>` 中 `<sub_agents max="N"></sub_agents>` 为空（A1 已结束，不再列为运行中）。
4. 观察主 agent 在该新迭代中的行为。
   - 预期：主 agent 依据通知调用 `sub_agent_status`（或等价的查询）取得 A1 的终态与结果，并给出后续结论/动作，而不是空转结束。

## 预期结果

- 主 agent 空闲期间发生的子代理完成**不会丢失**：必然通过"起新迭代"被送达主 agent。
- 通知以正式 user message 形式进入会话消息序列（不是仅前端清单刷新、不是仅日志）。
- 不需要用户参与即可触发（无人值守语义）。

## 验证方式

- Web UI 会话流：出现通知消息 + 随后自动出现新的 assistant 输出（无用户输入）。
- 后端日志：可见"子代理完成 → 空闲 → 启动新迭代"的记录。
- 单元测试补充：`Agent.IsBusy()==false` 时完成回调应调用"起新迭代"出口（可注入的桩函数），且只调用一次。

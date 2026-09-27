# FEATURE-565 运行时用例 UC-0006：<sub_agents> 只列运行中 + 工具说明写明 ID 选取规则

## 目标

验证主 agent 能**自主选对**子代理 ID：`<environment_details>` 的 `<sub_agents>` 只列**正在运行**的子代理，
并标注**当前最多可用子代理数**（N）；`launch_sub_agent` 的工具说明写明 ID 必填、取值范围与选取规则
（先看 env_details 中正在运行的子代理，必须选其之外的 A1..An）。

## 前置条件

- 工作区为 co-shell 仓库，`work/` 下已构建本分支可执行码（v0.79.0）。
- 上限 N = 5（默认）。
- 可用 `--dev` 或日志方式查看模型实际收到的 `<environment_details>` 文本（`show-tool`/调试日志）。

## 步骤

1. 会话初始状态（无运行中子代理）查看 `<sub_agents>` 块。
   - 预期：不列任何 agent（历史已完成/被杀的子代理**不出现**），并给出当前最多可用子代理数（5）。
   - 若该块在无运行中子代理时整体省略，必须仍能让主 agent 知道上限（例如工具说明里注明「默认 5」或块内含 `max=5`），
     并在本条用例确认验收记录中写明实际表现（省略 / 空块带 max）。
2. 启动 `A1`、`A3`（保持 running），查看 `<sub_agents>`。
   - 预期：只列 `A1`、`A3`（含 status/running pid/run/elapsed 等既有属性），**不列 A2/A4/A5**；max=5。
3. `kill_sub_agent(ids=["A3"])` 后再次查看。
   - 预期：`A3` 从 `<sub_agents>` 中消失；历史（已完成/killed）不作为条目出现。
4. 阅读 `launch_sub_agent` 的工具说明（工具 schema description）。
   - 预期：说明中明确：
     - `id` 为**必填**，取值必须是 `A1`..`A{n}`（按当前上限，默认 5，最大 10）；
     - 选择规则：先看 `<environment_details>` 中正在运行的子代理，**必须选其之外**的 ID；
     - 同一 ID 可重复使用（非运行状态下），复用后 run 计数递增；
     - 上限用尽（或 `max_sub_agents=0`）时的结果：报错拒绝。
5. 让主 agent 自主决策一次：在 A1 正在运行时，要求它「再起一个子代理做 X」。
   - 预期：主 agent 不选 A1，而是选择一个空闲 ID（如 A2），调用成功——说明说明文案足以支撑决策。
6. 在 N=3 的配置下重复步骤 4/5。
   - 预期：说明与错误文案中的取值域随之变为 `A1..A3`；主 agent 不会尝试 A4/A5。

## 预期结果

- `<sub_agents>` 的口径 = 当前会话中正在运行的子代理（历史不计），并带上当前最多可用数 N。
- 工具说明自洽完整，主 agent 无需试错即可选出合法且空闲的 ID。
- 相关文案（工具说明、错误消息）为多语言（zh/en），不硬编码中文。

## 验证方式

- 抓取 `<environment_details>` 原文比对（`grep -A5 '<sub_agents>'` 于调试日志）。
- 查看 `tools.go` / `subagent_tools.go` 中 `launch_sub_agent` 的 description 文本。
- 单元测试：`subAgentBlock()` 仅含 running 行且含 max；工具说明包含关键约束短语（可做包含性断言）。

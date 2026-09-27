# FEATURE-565 运行时用例 UC-0001：子代理上限的配置与 CLI 覆盖

## 目标

验证子代理槽位数上限 N 的来源与取值规则：`config.json` 的 `max_sub_agents` 字段为基准，
`--max-sub-agents` 命令行参数可覆盖配置；默认 5、允许范围 0..10（越界报错），`0` 表示禁止创建子代理。

## 前置条件

- 工作区为 co-shell 仓库，`work/` 下已构建本分支可执行码（v0.79.0）。
- 有一个可编辑的 `config.json`（或 `--config` 指向的配置文件）。
- 用 `--serve` 或 `run` 方式启动，便于观察子代理工具行为。

## 步骤

1. `config.json` 中**不写** `max_sub_agents`，启动后检查生效上限。
   - 预期：上限 = **5**（可通过子代理创建行为或 `launch_sub_agent` 的错误文案确认 A1..A5 可用、A6 被拒）。
2. `config.json` 中写入 `"max_sub_agents": 3`，重启，创建子代理 A1..A3 成功，尝试 A4。
   - 预期：A4 被拒绝（超出范围）。
3. 用 `--max-sub-agents 8` 覆盖配置（配置仍为 3），重启，创建 A1..A8。
   - 预期：A1..A8 全部可用，A9 被拒绝 —— 说明 CLI 覆盖优先于配置。
4. `config.json` 写入 `"max_sub_agents": 12`（>10）启动。
   - 预期：启动即报错拒绝（提示合法范围 0..10），或启动后第一个子代理请求返回越界错误；**不得静默接受 12**。
5. `config.json` 写入 `"max_sub_agents": -1`，同上检查。
   - 预期：同样被拒绝（不合法取值）。
6. `config.json` 写入 `"max_sub_agents": 0`，重启，尝试创建任意子代理。
   - 预期：任何创建请求都被拒绝并给出「已禁用子代理（max_sub_agents=0）」类提示；不做进程启动、不产生 `sub-agents/<id>/` 目录。
7. `--max-sub-agents 0` 覆盖配置为 5，重复步骤 6。
   - 预期：同样禁止创建（CLI 覆盖生效方向一致）。

## 预期结果

- 生效上限的优先级：`--max-sub-agents` > `config.json: max_sub_agents` > 默认 5。
- 合法区间 0..10；越界或非数字取值产生明确错误（多语言：zh/en），不静默降级。
- `max_sub_agents = 0` 时，本会话不存在任何可用的 A1..An 槽位。

## 验证方式

- 观察主 agent 调用 `launch_sub_agent` 的工具返回（错误文案应含合法范围与当前上限）。
- `ls sub-agents/` 确认被拒绝的请求没有产生工作目录。
- 单元测试补充：配置解析（默认值/覆盖/越界）与 CLI 解析的 table-driven 用例。

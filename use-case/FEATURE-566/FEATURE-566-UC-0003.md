# FEATURE-566 运行时用例 UC-0003：主 agent 正在迭代中 → 完成事件注入当前迭代（动态感知）

## 目标

验证主子代理完成时若主 agent **正在迭代中**，不新起迭代，而是通过**动态感知机制**把完成事件注入**当前这次迭代**（在下一次 LLM 调用的 `<environment_details><user_dynamic_events>` 中可见），使主 agent 在本次任务内即可感知并处理。

## 前置条件

- 工作区为 co-shell 仓库，`work/` 下已构建本分支可执行码（v0.80.0）。
- 以 `--serve --port <port>` 启动，Web UI 已连接。
- 已开启 `<environment_details>` 的动态感知块（`dynamic_events` 相关开关默认开启）。

## 步骤

1. 输入一个会**持续多轮迭代**的任务，例如：「先用 launch_sub_agent 以 A1 启动 `sleep 12`（timeout_seconds 3），然后在本次任务里继续用 execute_command 执行 4 次 `sleep 8` 并简洁汇报」。
2. 保持本轮任务运行（主 agent 仍在迭代），等待 A1 结束。
   - 预期：主 agent **不会**因为 A1 完成而额外启动一次新迭代；本次迭代继续。
3. 检查 A1 结束后主 agent 的下一次 LLM 调用输入（LLM 请求日志 / `--show-llm-content on`）。
   - 预期：`<environment_details>` 中出现 `<user_dynamic_events>`，其中含一条子代理完成条目（形如 `<sub_agent_done …>A1 ...</sub_agent_done>`），带时间戳。
4. 观察主 agent 是否在该迭代内对通知作出反应（例如查询 `sub_agent_status` 或直接在汇报中提及 A1 已完成）。

## 预期结果

- 忙时不新起迭代（无并发的第二个 turn），完成事件走动态感知注入路径。
- 事件在"下一次 LLM 调用"即可见，不被丢弃、不重复注入（注入后被消费）。
- 通知与其它动态事件（user_message / board_message / ui_action）共处同一块，格式不破坏 XML 结构。

## 验证方式

- LLM 输入日志中出现 `<user_dynamic_events>` + `<sub_agent_done>`，且 `<user_dynamic_events>` 块外无语法破坏。
- 后端日志：`IsBusy()==true` 路径为 `AddDynamicEvent`，未调用"起新迭代"出口。
- 单元测试补充：busy 时完成回调 → 事件队列长度 +1、且事件类型/文本符合约定；consume 后队列清空。

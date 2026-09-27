# FEATURE-566 运行时用例 UC-0011：终端（repl / 单命令）出口降级策略

## 目标

验证在没有"起新迭代"驱动能力的出口（交互式 repl、`run --prompt` 单命令、桥接的纯文本会话）上，完成通知会**以可感知的方式**交付：立即打印一行提示，并把事件缓存进动态事件队列，在**下一回合**的 `<user_dynamic_events>` 中注入（而不是静默丢弃）。

## 前置条件

- `work/` 下已构建本分支可执行码（v0.80.0）。
- 一个终端（非 Web）环境。

## 步骤

1. 在 repl 中执行：`启动 A1 执行 sleep 20 后结束，timeout_seconds 设 3`，等本次回合结束、回到提示符。
2. 等待 ~20 秒（A1 结束）。
   - 预期：终端**立即打印**一条完成提示（含 id/run、状态、提示用 `sub_agent_status` 查看结果）。
   - 预期：不自动发起新回合（终端无驱动能力，用户仍掌握节奏）。
3. 在提示符输入任意指令（如 `查看 A1 结果`）。
   - 预期：该回合的 `<environment_details><user_dynamic_events>` 中包含缓存的完成事件；主 agent 因此知道 A1 已结束。
4. 用 `run --prompt "..."` 单命令模式重复一次，确认不崩溃且提示被打印（stdout 或 stderr，二者之一且不破坏 `--output-format json` 的纯 JSON stdout 约定）。

## 预期结果

- 降级路径"既可见又可达"：用户看到提示、模型在下一回合看到事件。
- `--output-format json` 下不得把人类提示混入 stdout 破坏 JSON-Lines。

## 验证方式

- 终端输出含提示行；下一回合 LLM 输入含 `<sub_agent_done>` 条目。
- 单元测试补充：IO 未实现起新迭代接口时，回调走"打印 + AddDynamicEvent"分支（各被调用一次）。

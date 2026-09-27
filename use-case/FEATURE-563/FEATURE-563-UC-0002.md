# FEATURE-563 测试用例（二）—— 无人值守模式与阻断点封堵

> 分支：**FEATURE-563**｜版本：**v0.77.0** (BUILD-1135)

## 一、被测目标

新增启动参数 **`--unattended on|off`**：

| 场景 | 默认值 |
|---|---|
| 启动参数**含指令**（`-c/--cmd <cmd>` 或 `-- <instruction>`，即 `flags.command` 非空） | **on** |
| 启动参数**不含指令**（交互 REPL / serve） | **off** |

`on` 时必须封死全部阻断点：

| # | 阻断点 | 无人值守下的行为 |
|---|---|---|
| 1 | 工具确认（medium/high 风险需用户确认） | 强制 yolo，全部自动放行 |
| 2 | `ask_user` 工具（FEATURE-512 always available，yolo 管不到） | 从工具表移除（调用即未知工具） |
| 3 | `render_ui` / `ui_window` 的 `waiting=true`（等待用户操作） | 工具被禁用（或强制 waiting=false），不会阻塞 |
| 4 | `no_tool_action` 非 exit（LLM 不调工具时等待用户输入） | 强制 `exit`（`<no_tool_action>exit</no_tool_action>`） |
| 5 | `completion-mode`（attempt_completion 完成确认对话框） | 强制 `exit`（不弹对话、直接退出） |
| 6 | 嵌套创建 sub-agent | `--subagent-enabled off`（工具不注册） |
| 7 | 交互式 REPL | 一次性指令模式不进 REPL |

另新增 **`--disabled-tools <name[,name...]>`**：按名从工具表移除（用于显式封禁，含子 agent 场景）。

## 二、A 组：自动/单测验证

| 编号 | 用例 | 期望结果 | 结果 |
|------|------|----------|------|
| A-001 | `co-shell -- <指令>`（含指令，未显式指定） | 内部 unattended = **on**；`co-shell --version` 正常输出 `v0.77.0 [BUILD-1135]` | ☐ |
| A-002 | `co-shell -c "<指令>"` | 同上（on） | ☐ |
| A-003 | 无指令启动（`--serve` / REPL） | unattended = **off**（不改变现有交互行为） | ☐ |
| A-004 | `--unattended off -- <指令>` | 显式关闭：不强制 yolo、ask_user 仍在工具表（回归有人值守语义） | ☐ |
| A-005 | `--unattended on`（无指令） | 显式开启：即使无指令也强制无人值守 | ☐ |
| A-006 | on 时工具表 | 不含 `ask_user`；不含 `launch_sub_agent`/`schedule_task`（禁嵌套）；`render_ui`/`ui_window` 不可用或等待语义失效 | ☐ |
| A-007 | on 时环境块 | `<environment_details>` 中 `<no_tool_action>exit</no_tool_action>` | ☐ |
| A-008 | on 时 yolo | `IsYOLO()` 为 true；高风险工具（如 execute_command）不阻塞确认 | ☐ |
| A-009 | on 时 completion-mode | attempt_completion 后直接结束，不进入确认对话（`CompletionMode=exit`） | ☐ |
| A-010 | `--disabled-tools ask_user,render_ui` | 两个工具名从工具表移除；其余工具不受影响 | ☐ |
| A-011 | `--disabled-tools` 传未知名称 | 忽略未知项（不报错）或给出明确告警（行为与文档一致） | ☐ |
| A-012 | 子 agent 启动命令行检查（单测断言 argv） | 含 `--unattended on`、`--subagent-enabled off`、`-w {ws}/sub-agents/{id}`、`--name sub-{id}`、`--output-format json`；环境变量含 `CO_SHELL_SUB_AGENT=true`、`CO_SHELL_SUB_AGENT_ID`、`CO_SHELL_CONFIG_PATH` | ☐ |
| A-013 | i18n | 新增提示文案（无人值守已启用/工具已禁用等）在 zh/en 均有定义 | ☐ |

## 三、B 组：用户实测

| 编号 | 用例 | 期望 | 结果 |
|------|------|------|------|
| B-001 | 命令行一次执行：`co-shell -w <ws> -- "<需要多步工具调用的任务>"` | 全程无任何交互提示，自动完成并退出（不等确认、不弹选项） | ☐ |
| B-002 | 让 LLM 尝试调用 ask_user（在无人值守会话中） | 不产生等待；LLM 被告知工具不存在，改走其它路径 | ☐ |
| B-003 | sub-agent 交接（父 agent 调 launch_sub_agent） | 子进程无人值守完成，父 agent 不被交互卡住 | ☐ |
| B-004 | 子 agent 试图再创建 sub-agent | 工具不可用（禁嵌套），不会递归 | ☐ |
| B-005 | `--unattended off -- <指令>` 交互回归 | 恢复确认/提问行为（有人值守语义未被破坏） | ☐ |
| B-006 | REPL/serve 使用 | 与 0.76.x 行为一致（unattended off，不擅自改为自动放行） | ☐ |

## 四、验收标准（本文件）

1. A 组 13 条全部通过（自动化）；
2. B 组 6 条由用户实测确认；
3. **无人值守不会降低有人值守场景的安全性**（无指令启动一律 off；显式 off 可覆盖）；
4. 子 agent 启动参数与文档一致（A-012 断言通过）。

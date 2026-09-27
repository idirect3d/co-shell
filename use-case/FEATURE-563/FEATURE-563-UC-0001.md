# FEATURE-563 测试用例（一）—— sub-agent 核心：批量创建 / 进程表 / 超时转后台 / 结果提取 / kill / 复用

> 分支：**FEATURE-563**｜版本：**v0.77.0** (BUILD-1135)｜产物预期：`work/co-shell-0.77.0.darwin.arm64`、`work/co-shell-hub-0.77.0.darwin.arm64`

## 一、被测目标与接口契约

| 项 | 契约 |
|---|---|
| 工具 `launch_sub_agent` | 入参：`meta`（必需）、`tasks`（数组，1..N，每项 `{id, instruction}`）、`timeout_seconds`（**必填**，1..600）；返回各子任务 id/状态/pid/是否转后台 |
| 工具 `sub_agent_status` | 入参：`meta`、`ids?`（省略=全部）、`wait_seconds?`（0..600，仅等待不杀进程）；返回状态、pid、run 序号、起止时间、耗时、exit code、**最终结果（最后一轮 LLM 输出，不含 think）**、`output/` 文件清单、日志尾部 |
| 工具 `kill_sub_agent` | 入参：`meta`、`ids`（数组，必填）；SIGTERM → 500ms → SIGKILL，作用于**整个进程组**；状态记为 `killed` |
| id 规则 | `^[A-Za-z0-9_-]{1,32}$`（拒绝 `/`、`..`、空白、超长 → 防路径穿越） |
| 工作目录 | `{workspace}/sub-agents/{id}/`（自动创建）；日志 `run-{N}.jsonl`（stdout 事件流）、`run-{N}.err.log`（stderr） |
| 状态取值 | `running` / `completed` / `failed` / `timeout`（超时转后台，仍在运行）/ `killed` / `stale`（重启后 pid 失效） |
| 结果定义 | 子 agent **最后一轮 LLM 输出**（跳过 `thinking_chunk`/`thinking`）；超长截断（默认 8KB） |
| 复用 | 同 id **运行中** → 报错；同 id **已结束** → 同目录重启，`run` 序号递增，历史日志保留 |
| 持久化 | 进程表写入 store（key `subagent:procs`），重启后加载并标记 stale |
| 退出清理 | `Manager.Close()` → kill 全部后台 sub-agent（含进程组） |

## 二、测试夹具（无需真实 LLM）

A 组全部用**假可执行文件**驱动（`subagent` 包保留可注入的 execPath）：
`testdata/fake-co-shell/` 下放脚本，行为由环境变量控制，例如：

- `FAKE_SLEEP=<秒>`：启动后先睡眠
- `FAKE_STREAM=<文件>`：把预设 JSON-Lines 事件流写到 stdout 后退出
- `FAKE_EXIT=<码>`：退出码
- `FAKE_IGNORE_TERM=1`：忽略 SIGTERM（验证 SIGKILL 升级）
- `FAKE_CHILD=1`：再 fork 一个子进程（验证整组被 kill）

预设事件流样例（含 think，用于验证「不含 think」）：
```
{"type":"thinking_chunk","text":"内部推理不应出现在结果中"}
{"type":"content_chunk","text":"先检查目录"}
{"type":"tool_call","text":"list_files"}
{"type":"content_chunk","text":"最终结论：OK"}
{"type":"done"}
```

## 三、A 组：自动/单测验证（编码后自测，逐条打勾）

| 编号 | 用例 | 期望结果 | 结果 |
|------|------|----------|------|
| A-001 | 批量创建：一次 `tasks=[a,b,c]` | 3 个子进程并发启动；`{ws}/sub-agents/{a,b,c}/` 均被创建；返回 3 条记录且状态为 running/completed | ☐ |
| A-002 | 非法 id：`../x`、`a/b`、`a b`、33 字符、空串 | 工具返回错误；**磁盘上未创建任何 `sub-agents/` 子目录**（路径穿越防护）；同批其它合法任务不被牵连（或整批拒绝，行为与文档一致即可） | ☐ |
| A-003 | `timeout_seconds` 缺失 / 0 / -1 / 601 | 均返回错误（必填、范围 1..600）；1 与 600 被接受 | ☐ |
| A-004 | 超时转后台：`FAKE_SLEEP=5`、`timeout_seconds=1` | 工具在 ~1s 返回；返回项标记为 timeout/后台；**子进程仍存活**（`kill(pid,0)` 成功）；日志文件持续增长 | ☐ |
| A-005 | `sub_agent_status` 快照：不传 `ids` | 返回全部记录（含历史已完成项）；传 `ids=["a"]` 只返回 a | ☐ |
| A-006 | 结果提取（无 think）：`FAKE_STREAM` 含 thinking 文本 | 返回的最终结果 == `最终结论：OK`；**不包含**「内部推理不应出现在结果中」；工具调用轨迹含 `list_files` | ☐ |
| A-007 | `wait_seconds`：对后台任务 `wait_seconds=10`，进程 2s 后结束 | 调用阻塞约 2s 后返回 completed；**未被 kill**（exit code 正常记录） | ☐ |
| A-008 | `wait_seconds` 到期仍未完成 | 返回 timeout/后台提示（不杀进程、不报错） | ☐ |
| A-009 | kill：`FAKE_SLEEP=60` 后台运行中调用 `kill_sub_agent(["a"])` | 进程在 500ms 内退出；状态 = killed；`exit code` 标记为信号终止 | ☐ |
| A-010 | kill 升级：`FAKE_IGNORE_TERM=1` | SIGTERM 无效后 ~500ms 升级 SIGKILL，进程被终结（验证超时升级逻辑） | ☐ |
| A-011 | kill 进程组：`FAKE_CHILD=1`（fork 孙进程） | kill 后**孙进程同时消失**（整组终结，pgid 检查）；无遗留 | ☐ |
| A-012 | 复用：id 已 completed 后再 launch 同 id | 启动成功；`run` 序号 = 2；`run-1.jsonl` 未被覆盖/删除 | ☐ |
| A-013 | 同 id 运行中再次 launch | 返回错误（提示先 kill 或等待）；不产生第二个进程 | ☐ |
| A-014 | 持久化恢复：Manager 重启后加载 store | 记录仍存在；原 pid 已消失 → 状态标 `stale`；`sub_agent_status` 能列出 | ☐ |
| A-015 | 退出清理：`Manager.Close()`（模拟主 agent 退出） | 全部后台子进程及其进程组被 kill；`ps` 无残留；无僵尸进程 | ☐ |
| A-016 | 结果截断：`FAKE_STREAM` 输出 20KB content | 返回结果 ≤ 8KB 且有截断标记；不影响状态与 exit code | ☐ |
| A-017 | 边界：子 agent 不产生任何 `output/` 文件 | 正常返回（文件清单为空数组），不报错 | ☐ |
| A-018 | 边界：子进程 exit code 非 0（`FAKE_EXIT=2`） | 状态 = failed；exit code = 2；stderr 摘要可见 | ☐ |
| A-019 | 并发安全：同批 5 个任务下的进程表 | 进程表键为 id（**不再出现 key 恒为 -1 的覆盖缺陷**）；5 条记录各自独立、可分别 kill | ☐ |
| A-020 | i18n：中/英文案 | 工具返回与提示文案均在 zh/en 两份语言表中定义，无硬编码中文字面量 | ☐ |

## 四、B 组：用户实测（需真实 LLM，请在交付后确认）

| 编号 | 用例 | 期望 | 结果 |
|------|------|------|------|
| B-001 | 在 co-shell 会话中让 LLM 调用 `launch_sub_agent` 创建 2 个 sub-agent | 两个子进程启动；父 agent 在超时内拿到各自「最后一轮结论」 | ☐ |
| B-002 | 父 agent 设 `timeout_seconds=30`，子任务耗时更久 | 30s 后父 agent 继续；稍后 `sub_agent_status` 显示已完成并给出结果 | ☐ |
| B-003 | `<environment_details>` 观察 | 每轮含 `<sub_agents>` 块，状态与实际进程一致（可在 `:context full` 查看） | ☐ |
| B-004 | 对运行中的 sub-agent 调 `kill_sub_agent` | 进程消失（`ps` 验证），状态显示 killed | ☐ |
| B-005 | 同 id 二次下发（复用） | 同目录重启，`run-2.jsonl` 生成，`run-1.jsonl` 保留 | ☐ |
| B-006 | 重启 co-shell 后再查 | 进程表恢复；失效项标 stale；历史结果仍可读 | ☐ |
| B-007 | 子 agent 工作目录内容 | `{workspace}/sub-agents/{id}/` 下有日志与 `output/`；未污染主工作区 | ☐ |
| B-008 | 主 agent 退出（Ctrl+C / 关闭服务） | 无遗留 sub-agent 进程 | ☐ |

## 五、验收标准（本文件）

1. A 组 20 条全部通过（自动化）；
2. B 组 8 条由用户实测确认；
3. 相关包单测通过：`go test ./subagent/... ./agent/... -run 'SubAgent|Subagent'`；
4. 无新增第三方依赖；
5. 用例文件位于 `use-case/FEATURE-563/FEATURE-563-UC-0001.md`。

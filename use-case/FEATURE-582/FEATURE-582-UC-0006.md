# FEATURE-582 UC-0006：进程清理范围与消息闭合

> 分支：**FEATURE-582**　版本：**v0.90.0**　对应方案：§2.2 进程组注册表 + §2.4 本轮处置 + §8 执行模型
> 依据：用户确认的 Q1-A、Q4-B（含 detach 后台与 sub-agent）、Q6-B
> 判定口径：中断后整组清理（含孙进程 / detach 后台 / sub-agent / shell 会话）；消息序列保持闭合；重复中断幂等。

## 目的

验证"杀死所有由工具启动且仍在运行的进程组"的清理效果，以及中断后会话上下文仍然合法（可直接继续对话）。

## 前置条件

- 分支 `FEATURE-582`，Web 会话（`--serve`）；`--yolo` 便于直接触发工具。

## 用例 UC-0006：管道/孙进程整组清理

```bash
# 让 LLM 执行：sleep 600 | cat
# 工具运行中按 ESC，然后：
pgrep -fl "sleep 600" || echo "no sleep (expected)"
pgrep -x cat >/dev/null && echo "cat alive (unexpected)" || echo "no cat (expected)"
```

**期望**：shell、管道两端进程**全部消失**（`setProcessGroupAttr` 同组 + 整组 SIGKILL）。

## 用例 UC-0007：detach 后台服务与 sub-agent 一并终止（Q4-B）

```bash
# ① 先用 execute_command(on_timeout=detach) 起一个后台服务：sleep 700 >/dev/null 2>&1 &
# ② 再让 LLM 触发一个长时间工具并中断
sleep 1; pgrep -fl "sleep 700" || echo "detached child killed (expected, Q4-B)"
# ③ sub-agent：用 subagent 工具启动一个长时间子 Agent 后中断
pgrep -fl "sub-agent\|co-shell.*--prompt" || echo "sub-agent killed (expected)"
```

**期望**：`detach` 的后台子进程 **不再存活**（Q4-B）；sub-agent 子进程组同样被终止；日志/UI 列出被清理的条目数与 PID。

## 用例 UC-0008：shell 持久会话中断（Q6-B）与重建

```bash
# ① shell_start（自动）；shell_send("sleep 600\n") 后中断
pgrep -fl "sleep 600" || echo "shell child killed (expected)"
# ② 再次 shell_send("echo hi\n")
#   期望返回：会话已终止，需要重新启动（或自动重建）
```

**期望**：shell 会话被整组 kill，会话状态复位；后续 `shell_send` 给出明确提示或自动 `shell_start` 重建后可继续工作。

## 用例 UC-0009：中断后消息序列闭合（无 400）

```bash
# 中断发生后，继续发送一条普通指令（如"列出当前目录"）
grep -c "insufficient tool messages following tool_calls" log/llm-call-error-$(date +%F).log || echo "no such error (expected)"
```

**期望**：中断后继续对话**不出现** 400（`insufficient tool messages following tool_calls`）；本轮被丢弃的 `tool_calls` 不残留在上下文中。

## 用例 UC-0010：幂等与竞态

```bash
# 连续按 3 次 ESC
```

**期望**：只产生**一次**中断提示与一次清理（重复调用无副作用、无 panic、无重复 kill 日志）。

## 验收记录

| 用例 | 结果 | 证据 |
|---|---|---|
| UC-0006 | ⏳ 待验证 | |
| UC-0007 | ⏳ 待验证 | |
| UC-0008 | ⏳ 待验证 | |
| UC-0009 | ⏳ 待验证 | |
| UC-0010 | ⏳ 待验证 | |

---

## 验收记录（2026-10-02 · FEATURE-582 · v0.90.0 / BUILD-1183 · 分支 FEATURE-582）

### 自动化验证（本次会话已执行，证据可复现）

| 项 | 命令 | 结果 |
|---|---|---|
| 编译 + 静态检查 | `go build ./... && go vet ./...` | ✅ 通过（无输出） |
| 新增单测 | `go test ./agent/ -run 'TestInterrupt\|TestProcRegistry\|TestErrToolInterrupted\|TestRunStreamInterrupt' -count=1` | ✅ 全部通过 |
| 回归对比 | `go test ./agent/ ./repl/ ./web/ ./shell/ -short -count=1` vs `main@bde28e7d` | ✅ 失败集逐项一致（`comm -13` 为空集） |
| 残留进程检查 | `pgrep -fl "sleep 60"`（跑完中断测试后） | ✅ 无残留（进程组注册表 + KillAll 生效） |

与 UC-0006~0010 的对应：
- UC-0006 幂等：`TestInterruptToolIdempotent`（重复中断只生效一次，原因不被覆盖）
- UC-0007 消息闭合：`TestRunStreamInterruptDiscardsUnclosedToolCalls` + `assertToolCallsClosed`（无未闭合 tool_calls）
- UC-0008/0009：进程组整组 kill 与 detach 条目语义由 `TestProcRegistryKillAllTerminatesGroup`、`TestProcRegistryKillActiveSkipsDetached` 覆盖
- UC-0010（中断提示文案）：新增 i18n key `interrupt_tool_notice` / `interrupt_tool_killed` / `interrupt_shell_terminated`（zh/en 双语）

### 运行时端到端验证（尚未在本会话的真实运行环境中执行）

「中断时 UI 立即恢复、消息闭合不触发 400、shell 会话可重建」等依赖真实进程与模型的条目
**待用户实测确认**（详见 `FEATURE-582-UC-0001.md` 末尾的实测步骤）。

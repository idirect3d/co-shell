# FEATURE-582 UC-0001：三个出口的中断行为（Web ESC / 暂停按钮 / TUI Ctrl+C）

> 分支：**FEATURE-582**　版本：**v0.90.0**　对应方案：§2.1 统一中断入口 + §2.5 交互反馈
> 依据：用户确认的 Q2-A、Q3-A；方案 §7.1
> 判定口径：工具执行期间，三个出口都能**立即**（≤1s）中断并杀死工具子进程；空闲状态下的 Ctrl+C 仍退出程序；LLM 输出阶段的 ESC 仍保留"暂停 + 确认"。

## 目的

验证"中断入口单点收敛"在三端出口都生效，且不破坏既有的暂停/退出语义。

## 前置条件

- 分支 `FEATURE-582`，产物 `work/co-shell-0.90.0.darwin.arm64`。
- Web：`./work/co-shell-0.90.0.darwin.arm64 --serve --port 28261`（`--yolo`）+ 调试浏览器。
- TUI：直接运行同一产物（raw 模式）。

## 用例 UC-0001：Web ESC 中断命令类工具

```js
// 触发一个长命令，然后按 ESC
// browser_evaluate: 发送输入
wsSend({ type: "input", text: "执行 sleep 600" });   // 由 LLM 调用 execute_command
// 待工具块出现后：
document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }));
```

```bash
sleep 1; pgrep -fl "sleep 600" || echo "no leftover"; tail -5 log/co-shell-$(date +%F).log | grep -i "interrupt"
```

**期望**：按 ESC 后 **≤1s** 前端回到输入态（按钮由 ⏸ 复位为 ➤）；`pgrep` 无 `sleep 600`；日志出现中断记录（含 PID 与被杀条目数）。

## 用例 UC-0002：Web 暂停按钮与 ESC 等效

```js
// 点击合并按钮（运行中为暂停图标）
document.getElementById("sendBtn").click();
```

**期望**：与 UC-0001 完全一致的结果（同一 `interrupt` 消息 → 同一中断入口）。

## 用例 UC-0003：TUI Ctrl+C 在工具执行期间 = 中断工具（不退出程序）

```bash
# TUI 中让 LLM 执行 sleep 600，然后按 Ctrl+C（原始字节 0x03）
printf '\x03'   # 由测试脚本注入
sleep 1
ps -p <co-shell-pid> >/dev/null && echo "process alive (expected)"
pgrep -fl "sleep 600" || echo "no leftover (expected)"
```

**期望**：工具被中断、`sleep 600` 消失；**co-shell 主进程仍存活**（未 `os.Exit(0)`）；提示行出现中断结果。

## 用例 UC-0004：TUI 空闲时 Ctrl+C 仍退出程序（保持现状）

```bash
# 空闲等待输入状态按 Ctrl+C
printf '\x03'
```

**期望**：打印告别语并退出（与改动前一致，验证 Q2-A 的"空闲=退出"分支未被破坏）。

## 用例 UC-0005：工具期间三者统一（不弹确认框）；LLM 输出阶段 ESC 仍为暂停+确认

```text
① 工具执行期间按 ESC / 暂停按钮 / Ctrl+C → 直接中断，**不出现确认对话框**；
② 在 LLM 正在输出（无工具执行）时按 ESC → 仍出现既有"输出已暂停，是否继续/调试/重试"的确认交互（FEATURE-201/427 行为不变）。
```

**期望**：① 无 `interaction` 弹层，直接回到输入态；② 输出阶段的确认交互与改动前逐字一致。

## 验收记录

| 用例 | 结果 | 证据 |
|---|---|---|
| UC-0001 | ⏳ 待验证 | |
| UC-0002 | ⏳ 待验证 | |
| UC-0003 | ⏳ 待验证 | |
| UC-0004 | ⏳ 待验证 | |
| UC-0005 | ⏳ 待验证 | |

---

## 验收记录（2026-10-02 · FEATURE-582 · v0.90.0 / BUILD-1183 · 分支 FEATURE-582）

### 自动化验证（本次会话已执行，证据可复现）

| 项 | 命令 | 结果 |
|---|---|---|
| 编译 + 静态检查 | `go build ./... && go vet ./...` | ✅ 通过（无输出） |
| 新增单测 | `go test ./agent/ -run 'TestInterrupt\|TestProcRegistry\|TestErrToolInterrupted\|TestRunStreamInterrupt' -count=1` | ✅ 全部通过 |
| 回归对比 | `go test ./agent/ ./repl/ ./web/ ./shell/ -short -count=1` vs `main@bde28e7d` | ✅ 失败集逐项一致（`comm -13` 为空集，无新增失败） |
| 残留进程检查 | `pgrep -fl "sleep 60"`（跑完命令类中断测试后） | ✅ 无残留 |

新增单测与用例的对应关系：
- `TestProcRegistryKillAllTerminatesGroup` → UC-0001/UC-0004（整组 kill，真实 SIGTERM→SIGKILL 并校验 ESRCH）
- `TestProcRegistryKillActiveSkipsDetached` → UC-0011（detach 条目的选择性清理语义）
- `TestRunStreamInterruptDiscardsUnclosedToolCalls` → UC-0007（中断后 messages 中不存在缺结果的 tool_calls）
- `TestInterruptToolIdempotent` / `TestInterruptToolClearsAtTurnReset` → UC-0006（幂等、轮次复位）
- `TestErrToolInterruptedMatches` → 中断错误类型与超时/普通失败可区分

### 运行时端到端验证（尚未在本会话的真实运行环境中执行）

Web ESC / 暂停按钮、TUI Ctrl+C（工具期间不退出）、detach 后台进程与 sub-agent 清理、
shell 会话重建、浏览器类工具等待中中断等条目，需要真实进程环境与真实模型输出才能确认。
本会话已完成**代码接线 + 单测 + 回归对比**，端到端条目**待用户按下列步骤实测确认**（不要在未实测前
视为通过）：

1. 启动：`work/co-shell-0.90.0.darwin.arm64 --serve --port 8799`
2. 让模型执行 `execute_command("sleep 600")`（timeout_seconds=600, on_timeout=kill）
3. 按 ESC 或点暂停按钮 → 期望 ≤1s 回到输入态；`pgrep -f "sleep 600"` 应为空；继续下一轮对话不应出现 400（消息闭合）
4. TUI：启动 `work/co-shell-0.90.0.darwin.arm64`，同样触发长命令 → Ctrl+C → 期望工具被中断且**程序不退出**；空闲时 Ctrl+C 仍为退出
5. UC-0011/0012/0013：分别验证 detach 后台进程、sub-agent 被清理、shell 会话被整组 kill 后 `shell_send` 提示会话已终止且 `shell_start` 可重建

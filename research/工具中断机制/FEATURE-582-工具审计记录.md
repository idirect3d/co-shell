# FEATURE-582 工具审计记录（62 个已注册工具的可中断性体检）

> 依据：`research/工具中断机制/工具调用中断机制方案.md` §8（执行模型）与 §9（审计矩阵）
> 结论口径：每个工具至少要有一个「可取消点」；**没有可取消点的工具由 §8 的「独立 goroutine 执行 + 中断即返回 + 在途告警」兜底**——用户侧永远能在中断后立即回到输入态。
> 审计时间：2026-10-02　分支：FEATURE-582

## 1. 统一机制（对所有 62 个工具生效，不依赖单个工具配合）

| 机制 | 位置 | 效果 |
|---|---|---|
| 工具在独立 goroutine 执行，RunStream `select{结果, 中断}` | `agent/run_stream.go` | 中断即刻返回输入态，不 join 工具 goroutine |
| 中断入口 `InterruptTool(reason)` | `agent/interrupt.go` | 置标记 + `cancelFunc()` + `KillAll()` + 写 `cancelCh` + 5s 在途告警 |
| 进程组注册表 | `agent/proc_registry.go` | 中断时 SIGTERM → 200ms → SIGKILL，杀**所有**工具进程组（Q4-B） |
| 消息闭合（Q1-A） | `agent/run_stream.go` | 丢弃本轮未闭合的 assistant(tool_calls) 与已产生的 tool 结果，避免 API 400 |

因此：**即使某个工具完全忽略 ctx，用户也不会被卡住**（UI 立即恢复；工具 goroutine 自行退出或被 5s 告警观测到）。

## 2. 逐类审计结果

| # | 类别 | 实现位置 | 本次改动 | 状态 |
|---|---|---|---|---|
| 1 | 命令与进程（execute_command / 直接命令 / demo 命令） | `agent/command_tools.go` | 新增 `ctx.Done()` 分支 → 整组 kill（含 detach 分支）；接入 `procReg`；返回 `ErrToolInterrupted` | ✅ 已补齐 |
| 2 | 持久 shell 会话（shell_start/send/get_output/reset/window_content/stop） | `agent/shell_tools.go`、`shell/session.go` | Q6-B：会话进程组登记；中断整组 kill；`MarkTerminated()` 复位；`shell_send` 返回「会话已终止，请 shell_start 重建」 | ✅ 已补齐 |
| 3 | 子 Agent（launch_sub_agent / subagent_status / kill_sub_agent） | `agent/subagent_tools.go`、`subagent/` | `Manager.SetProcRegistrar` 钩子：启动即登记进程组，reap 时注销；中断即整组 kill | ✅ 已补齐 |
| 4 | 浏览器（browser_*） | `agent/browser_tools*.go`、`browser/cdp_wait.go`、`cdp_events.go` | 等待点（页面加载/选择器/事件）已响应 ctx（§1 证据），无需改动 | ✅ 已具备 |
| 4b | Chrome 进程本身 | `browser/chrome.go` | 未纳入 `KillAll`：Chrome 归 `ChromeManager` 所有（`Start/Stop`），无 PID 出口；连带杀会强制重启浏览器并丢失会话 | ⚠️ 有意保留（见 §3） |
| 5 | MCP 工具（mcp_*） | `mcp/sse_client.go`、`stdio_client.go` | 调用链已响应 ctx（§1 证据），工具层透传确认 | ✅ 已具备 |
| 6 | 文档（docx_* / excel_*） | `agent/docx_tools.go`、`excel_tools.go`、`docx/`、`xlsx/` | 均为内存内快速操作（会话对象），无秒级阻塞点；由 §8 兜底 | ✅ 兜底覆盖 |
| 7 | 视觉/图片（visual_analysis / add_images / 图片工具） | `agent/image_tools.go`、`visual_analysis` | LLM/解码调用所在链路已持有 ctx；由 §8 兜底 | ✅ 兜底覆盖 |
| 8 | 文件与搜索（read/write/replace/search/list/list_code_definition_names） | `agent/file_tools.go` | **新增**：`search_files` / `list_files` 目录遍历回调内 `ctx.Err()` 检查点（大目录树可立即中断） | ✅ 已补齐 |
| 9 | 记忆/任务/设置/公告板/调度 | `memory_tools.go`、`taskplan_tools.go`、`settings_tools.go`、`board_tools.go`、`schedule_task` | 本地 KV/快速返回为主；网络调用（board）已带 ctx；由 §8 兜底 | ✅ 兜底覆盖 |
| 10 | UI 与元能力（render_ui / ui_window / introspect_capability / evaluate_expression / reorganize_context） | `agent/ui_tools.go` 等 | `render_ui(waiting)` 已响应 ctx 与 `ReleaseUIWait()`（Web interrupt 同时调用）；快返回工具由 §8 兜底 | ✅ 已具备 |

## 3. 有意的边界（明确不做的部分）

1. **Chrome 进程不连带杀**：`browser` 的 Chrome 由 `ChromeManager` 管理且无 PID 出口；中断浏览器工具时，工具 goroutine 依靠 ctx 自行退出（等待点均已响应），Chrome 保持存活，避免用户浏览器会话被反复重启。若后续需要「连 Chrome 一起杀」，需要在 `browser` 暴露 PID 并接受重启成本。
2. **纯 Go CPU 死循环工具**：项目内暂未发现；此类工具无法协作取消，由 §8 的 5s 在途告警兜底（用户已回到输入态，仅日志告警）。
3. **TUI 中 `execute_command` 运行期间 ESC**：期间终端被切回 cooked 模式且输入事件源被暂停（交互式命令需要独占 stdin），此时中断键为 **Ctrl+C（SIGINT）**；非命令类工具期间 ESC/Ctrl+C 均直接中断（`repl/repl_esc.go`、`repl/repl.go`）。

## 4. 验证证据

- 单测：`agent/interrupt_test.go`（中断幂等、进程组 `KillAll` 真实终止、`KillActive` 保留 detach、RunStream 中断后无未闭合 `tool_calls`）。
- 回归：`go test ./agent/ ./repl/ ./web/ ./shell/ -short -count=1` 与 `main`（bde28e7d）基线失败集逐项一致（`comm -13` 为空集）。
- 端到端用例：`use-case/FEATURE-582/`（UC-0001~0015）。

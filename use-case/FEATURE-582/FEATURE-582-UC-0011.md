# FEATURE-582 UC-0011：覆盖度、兜底与回归

> 分支：**FEATURE-582**　版本：**v0.90.0**　对应方案：§8 执行模型（goroutine + 中断即返回 + 在途告警）+ §9 全工具审计矩阵 + §6 风险
> 依据：用户确认的 Q5-A（一次性覆盖全部工具）；方案 §5 验收方式
> 判定口径：非命令类工具同样可中断；无可取消点的工具不会导致永久卡死（在途告警兜底）；静态检查与既有测试零回归。

## 目的

验证"全工具覆盖"的可达性：既包括已响应 ctx 的工具，也包括暂时没有取消点的工具（靠 §8 的兜底机制保证用户能立即脱身）。

## 前置条件

- 分支 `FEATURE-582`；Web 会话（`--serve`）+ 调试浏览器；`go build ./... && go vet ./...` 通过。

## 用例 UC-0011：浏览器类工具中断

```js
// 让 LLM 执行 browser_navigate 到一个慢站点（或在本地起一个延迟响应的测试页），
// 等待期间按 ESC
```

**期望**：≤1s 回到输入态；CDP 等待被 ctx 取消（`browser/cdp_wait.go` 的 `ctx.Done()` 分支命中）；后续 `browser_*` 调用仍可用。

## 用例 UC-0012：MCP / 视觉 / 文件搜索类工具审计

```bash
# 逐类触发并中断（MCP 调用一个慢工具；visual_analysis 分析大图；search_files 扫大目录）
```

**期望**：每类工具在中断时**要么**因 ctx 取消而返回（≤1s），**要么**被 §8 的在途机制接管（UI 立即恢复 + 告警）；无任何一类出现"UI 卡住直到工具自然结束"。

## 用例 UC-0013：在途工具告警（§8）

```bash
# 中断后观察日志
sleep 6; grep -i "in-flight tool still running" log/co-shell-$(date +%F).log | tail -3
```

**期望**：若某工具中断后 5s 未退出，日志出现在途告警（含工具名与耗时）；UI 侧**早已**恢复输入态（不等待）。

## 用例 UC-0014：无可取消点工具的兜底（测试桩）

```go
// agent/interrupt_test.go：注册一个"忽略 ctx、睡 30s"的工具，触发中断
// 断言：中断调用在 ≤1s 内返回；UI 状态回到 idle；5s 后告警日志出现；无 panic
```

**期望**：此类工具不会造成永久卡死（这正是"不能杀 goroutine"下用户仍能脱身的保证）。

## 用例 UC-0015：静态检查与基线回归

```bash
cd /Users/direct3d/github/co-shell
go build ./... && go vet ./...
go test ./agent/ ./repl/ ./web/ ./shell/ -short -count=1 2>&1 | grep -E "^--- FAIL" | sed 's/ ([0-9.]*s)$//' | sort -u > /tmp/fe582-fail.txt
git worktree add --detach /tmp/coshell-main-fe582 main >/dev/null 2>&1
(cd /tmp/coshell-main-fe582 && go test ./agent/ ./repl/ ./web/ ./shell/ -short -count=1 2>&1 | grep -E "^--- FAIL" | sed 's/ ([0-9.]*s)$//' | sort -u) > /tmp/fe582-main-fail.txt
comm -13 /tmp/fe582-main-fail.txt /tmp/fe582-fail.txt   # 期望：空
```

**期望**：编译/vet 无输出；新增测试全绿；失败集与 `main` 基线**逐项一致**（零新增）。

## 验收记录

| 用例 | 结果 | 证据 |
|---|---|---|
| UC-0011 | ⏳ 待验证 | |
| UC-0012 | ⏳ 待验证 | |
| UC-0013 | ⏳ 待验证 | |
| UC-0014 | ⏳ 待验证 | |
| UC-0015 | ⏳ 待验证 | |

---

## 验收记录（2026-10-02 · FEATURE-582 · v0.90.0 / BUILD-1183 · 分支 FEATURE-582）

### 自动化验证（本次会话已执行）

| 项 | 命令 | 结果 |
|---|---|---|
| 编译 + 静态检查 | `go build ./... && go vet ./...` | ✅ 通过 |
| 新增单测 | `go test ./agent/ -run 'TestInterrupt\|TestProcRegistry\|TestErrToolInterrupted\|TestRunStreamInterrupt' -count=1` | ✅ 通过 |
| 回归对比（UC-0010 同类） | `go test ./agent/ ./repl/ ./web/ ./shell/ -short -count=1` vs `main@bde28e7d` | ✅ 失败集一致（无新增失败） |
| 产物 | `work/co-shell-0.90.0.darwin.arm64`、`work/co-shell-hub-0.90.0.darwin.arm64` | ✅ 已生成 |

### 覆盖度结论（Q5-A：一次性覆盖全部工具）

- 全量工具的中断保证由 **§8 执行模型**统一提供（工具在独立 goroutine 执行 + 中断即返回 + 5s 在途告警），
  已实现的单测验证了该模型（`TestRunStreamInterruptDiscardsUnclosedToolCalls`、`TestProcRegistryKillAllTerminatesGroup`）。
- 逐工具体检结果见 `research/工具中断机制/FEATURE-582-工具审计记录.md`：
  - **已补齐可取消点**：命令类（ctx.Done 整组 kill）、shell 会话（Q6-B 整组 kill + 状态复位）、sub-agent（注册表 + reap 注销）、文件搜索/列目录（目录遍历 ctx 检查点）
  - **已具备**：浏览器等待点、MCP、`render_ui(waiting)`（既有实现已响应 ctx）
  - **兜底覆盖**：docx/xlsx、视觉/图片、记忆/任务/设置/公告板等快速或本地操作
  - **有意边界**：Chrome 进程本体不连带杀（归属 `ChromeManager`，避免强制重启用户浏览器），已在审计记录 §3 说明
- UC-0015「无可取消点工具兜底」由 goroutine + 5s 在途告警满足：UI 立即恢复，残留工具在日志中可观测。

### 运行时端到端验证（尚未在本会话的真实运行环境中执行）

见 `FEATURE-582-UC-0001.md` 末尾的实测步骤（Web ESC/暂停、TUI Ctrl+C、detach/sub-agent/shell 清理）。

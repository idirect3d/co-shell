# FEATURE-327: 循环重试次数熔断提示

## 背景

当前循环介入（loop intervention）路径有一个隐患：**循环检测可以无限重试**。当 LLM 反复输出循环内容时，`applyLoopFeedback` 每次递增 `<retry_count>`，但没有任何阈值控制——循环可能永远持续下去，用户没有介入的机会。

同时，`retry_count` 标签已经存在于最后一条 user 消息的 `<environment_details>` 中（FIX-321 引入），它的值天然代表"连续重试次数"。

本任务：**复用 `retry_count` 标签 + 现有 `error-max-single-count` 参数**，在循环介入递增 retry_count 后检查阈值，达到 `error-max-single-count`（默认 10）时提示用户处理，用户同意继续则重置 retry_count=1。**不改变现有 errorCounter 机制**（普通 LLM API 错误的计数逻辑完全保留）。

## 设计

### 新增 helper：checkRetryCountLimit

```
func (a *Agent) checkRetryCountLimit() (bool, error)
```

行为：
1. 读最后 user 消息 env 的 `<retry_count>`（`getRetryCountFromText`）；无 user 消息 → 直接返回 (true, nil)
2. 取 `maxSingle`：`a.cfg.LLM.ErrorMaxSingleCount > 0` 用它，否则默认 10
3. `count < maxSingle || a.errorApproveAll` → 返回 (true, nil) 不提示
4. 达到阈值 → 复用现有错误提示流程（KeyErrRepeatWarn / KeyErrActionEnter / KeyErrActionCancel / KeyErrActionIgnore / KeyErrActionChoose，交互经 `a.defaultIO()`）
   - **C** → 返回 (false, 取消错误)，终止当前任务
   - **回车** → 将最后 user 消息的 `<retry_count>` 重置为 `1`，返回 (true, nil)
   - **A** → 设置 `a.errorApproveAll = true`（与现有 errorCounter 的 A 行为统一，同时抑制 retry_count 后续提示），返回 (true, nil)

### 接入点

| 调用点 | 位置 | 改动 |
|---|---|---|
| sync 循环分支 | `run_stream.go:427` `a.applyLoopFeedback(loopFeedback)` 后 | 检查返回值，取消时终止任务 |
| `applyLoopIntervention` | `loop.go:1008` `a.applyLoopFeedback(loopFeedback)` 后 | 检查返回值，取消时返回错误 |
| 工具调用循环 | `run_stream.go:905` `a.applyLoopIntervention(event)`（当前忽略返回值） | 改为检查返回值，取消时终止任务 |

## 用例列表

### UC-0001 达到阈值前不提示（单测）

**前置条件**：`ErrorMaxSingleCount=3`，最后 user 消息 env `<retry_count>2</retry_count>`。

**步骤**：
1. 构造消息序列：`[system, user(指令+env 含 retry_count=2), assistant(循环内容)]`
2. 调用 `checkRetryCountLimit()`

**预期**：
- 返回 `(true, nil)` — 不提示用户
- `retry_count` 保持不变（2）
- 无用户 I/O 交互发生

### UC-0002 达到阈值触发提示，用户回车继续 → 重置为 1（单测）

**前置条件**：`ErrorMaxSingleCount=3`，最后 user 消息 env `<retry_count>3</retry_count>`，UserIO 返回回车（空行）。

**步骤**：
1. 构造消息序列：`[system, user(指令+env 含 retry_count=3)]`
2. 调用 `checkRetryCountLimit()`

**预期**：
- 返回 `(true, nil)` — 继续循环
- 最后 user 消息 env 的 `<retry_count>` 重置为 `1`
- 用户看到了警告与选项提示（Enter/C/A）

### UC-0003 达到阈值触发提示，用户选择 C 取消 → 终止任务（单测）

**前置条件**：`ErrorMaxSingleCount=3`，最后 user 消息 env `<retry_count>3</retry_count>`，UserIO 返回 `"C"`。

**步骤**：
1. 构造消息序列：`[system, user(指令+env 含 retry_count=3)]`
2. 调用 `checkRetryCountLimit()`

**预期**：
- 返回 `(false, 取消错误)` — 调用方终止任务
- `retry_count` 保持不变（3）

### UC-0004 达到阈值触发提示，用户选择 A 忽略全部（单测）

**前置条件**：`ErrorMaxSingleCount=3`，最后 user 消息 env `<retry_count>3</retry_count>`，UserIO 返回 `"A"`。

**步骤**：
1. 构造消息序列：`[system, user(指令+env 含 retry_count=3)]`
2. 调用 `checkRetryCountLimit()`
3. 再次触发循环介入（retry_count 递增到 4、5...）
4. 再次调用 `checkRetryCountLimit()`

**预期**：
- 第一次返回 `(true, nil)`，`a.errorApproveAll == true`
- 后续调用即使 retry_count ≥ maxSingle 也立即返回 `(true, nil)` 不提示
- retry_count 不再重置（保持原值）

### UC-0005 配置为 0 或空时使用默认 10（单测）

**前置条件**：`ErrorMaxSingleCount=0`（或未配置），构造 env `<retry_count>10</retry_count>`。

**步骤**：
1. 构造消息序列：`[system, user(指令+env 含 retry_count=9)]`，先调用一次递增到 10
2. 调用 `checkRetryCountLimit()`

**预期**：
- 按默认阈值 10 判断 — retry_count=9 时不提示，递增到 10 后触发提示

### UC-0006 无 user 消息时不提示（单测）

**前置条件**：`a.messages` 只含 system 消息。

**步骤**：
1. 调用 `checkRetryCountLimit()`

**预期**：
- 返回 `(true, nil)`，无任何 I/O 交互

### UC-0007 sync 循环分支接入：达到阈值用户回车后 continue（单测/集成）

**前置条件**：`ErrorMaxSingleCount=2`，sync 循环检测触发路径，UserIO 返回回车。

**步骤**：
1. 模拟 sync 循环检测：`a.loopDetectCrit=true`，循环反馈注入（retry_count 从 1 递增到 2）
2. 触发 `checkRetryCountLimit()` 检查
3. 断言循环继续（迭代循环不退出）

**预期**：
- retry_count=2 达到阈值 → 提示用户
- 用户回车 → 返回 (true, nil) → 循环继续
- retry_count 重置为 1

### UC-0008 sync 循环分支接入：用户 C 取消 → RunStream 返回（集成）

**前置条件**：`ErrorMaxSingleCount=2`，sync 循环检测触发路径，UserIO 返回 `"C"`。

**步骤**：
1. 模拟 sync 循环检测：retry_count 递增到 2
2. 触发 `checkRetryCountLimit()` 检查
3. 断言调用方终止任务

**预期**：
- 返回取消错误 → RunStream 返回（任务终止）
- 用户看到取消提示

### UC-0009 applyLoopIntervention 接入：达到阈值提示后继续（集成）

**前置条件**：`ErrorMaxSingleCount=2`，工具调用循环触发 `applyLoopIntervention`，UserIO 返回回车。

**步骤**：
1. 模拟工具调用循环事件（LoopEventToolCallRepeat）
2. `applyLoopIntervention` 内部递增 retry_count 达到 2
3. 触发 `checkRetryCountLimit()` 检查
4. 断言正常返回 nil（不终止）

**预期**：
- retry_count=2 达到阈值 → 提示用户
- 用户回车 → 返回 nil → 继续迭代
- retry_count 重置为 1

### UC-0010 applyLoopIntervention 接入：用户 C 取消 → 返回错误（集成）

**前置条件**：`ErrorMaxSingleCount=2`，工具调用循环触发 `applyLoopIntervention`，UserIO 返回 `"C"`。

**步骤**：
1. 模拟工具调用循环事件
2. `applyLoopIntervention` 内部递增 retry_count 达到 2
3. 触发 `checkRetryCountLimit()` 检查
4. 断言返回错误（用户取消）

**预期**：
- `applyLoopIntervention` 返回非 nil 错误
- 调用方（run_stream.go:905）需要处理该错误并终止任务

### UC-0011 重置后重新累积（单测，连环场景）

**前置条件**：`ErrorMaxSingleCount=3`，UserIO 第一次返回回车，后续无输入需要。

**步骤**：
1. 构造 env `<retry_count>3</retry_count>`
2. 调用 `checkRetryCountLimit()` → 回车 → 重置为 1
3. 模拟后续循环：`applyLoopFeedback("")` 递增到 2、3
4. 再次调用 `checkRetryCountLimit()` → 提示再次触发

**预期**：
- 第一次重置后 retry_count=1
- 再次累积到 3 后第二次提示触发 — 熔断机制可持续工作

### UC-0012 运行时验证：实际循环场景熔断生效（运行时）

**前置条件**：
- `loop-judge-enabled=off`，`loop-intervention=prompt`，`error-max-single-count=3`
- 使用可稳定触发循环的测试指令

**步骤**：
1. 启动 co-shell
2. 输入一个会触发循环检测的指令
3. 观察连续循环 3 次后出现用户提示（Enter/C/A）
4. 按回车后继续，观察 retry_count 重置为 1

**预期**：
- 循环到第 3 次时出现用户提示（之前无提示）
- 按回车后 retry_count 重置，循环继续
- 按 C 则任务终止返回 REPL

## 验收标准

1. `go build ./...` 编译通过。
2. `go vet ./agent/...` 无警告。
3. `go test ./agent/ -run 'TestCheckRetryCountLimit|TestLoopRetry'` 全部通过。
4. 上述 UC-0001 ~ UC-0012 全部通过（单测为主，UC-0012 可人工确认）。
5. 回归：`go test ./...` 全绿。
6. 现有 errorCounter 机制（error-max-single-count / error-max-type-count 判断）不受影响。
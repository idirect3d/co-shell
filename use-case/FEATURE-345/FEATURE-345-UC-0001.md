# FEATURE-345 异常场景接入问题判定 — 测试用例

> 任务编号：FEATURE-345
> 作者：L.Shuang
> 创建日期：2026-08-10

## 背景

FEATURE-342 仅将「循环」场景接入 report_problem 问题判定（judgeLoop → callProblemSolver）。config.go 承诺「疑似信号（工具格式错误/上下文溢出/连接异常）发送给问题模型」尚未兑现：三类异常仍走各自硬编码路径。

本任务新增统一入口 `solveProblem(ctx, hint, detail)` 与动作分发 `applyProblemAction(report)`，在 `run_stream.go` 三处接入，使 `default-problem-model` 消费面从 1 个场景扩展到 4 个场景。

## 架构

```
solveProblem(ctx, hint, detail)            agent/problem_solver.go
  └─ 门控：cfg.LLM.ProblemSolverEnabled
  └─ 复用 callProblemSolver（强制 report_problem 单工具 tool_choice）
      └─ getLoopJudgeModel → getProblemModelID（mode>default-problem>default-tool>mode文本>active）

applyProblemAction(report) → (feedback, deleteLast, stop)
  prompt_feedback/compact_context → (guidance, false, false)
  delete_last_msg                → ("", true, false)
  notify_user                    → ("", false, true)
  其他/未知                      → ("", false, false) 保持原有行为

run_stream.go 接入点：
  1. streamErr 分支（连接错误）：ambiguous 才调用；stop→终止，feedback→追加后 continue
  2. XML 工具格式错误分支：override 通用 preventive 模板；delete_last→移除assistant重试
  3. context 超限分支：stop→终止；feedback→预追加，reorganizePending 保持 true
```

## 降级保护

- 问题模型调用失败（模型不可用/超时）→ `perr != nil` → 回退到现有硬编码路径
- HTTP 401/403/404/429/5xx 充分条件 → `classifyConnectionError` 返回 ok=true → 不调用问题模型
- `problem-solver-enabled=off` → `solveProblem` 返回 nil → 完全走原有路径
- 工具格式错误：`parse-error-action=exit` 仍优先（用户显式选择停止）

## 用例列表

| 编号 | 场景 | 操作 | 预期结果 |
|------|------|------|---------|
| UC-0001 | 问题模型启用，连接错误模糊（无充分条件） | 模拟 ambiguous streamErr + solveProblem 返回 prompt_feedback | guidance 追加为 user 消息并 continue |
| UC-0002 | 问题模型返回 notify_user | 模拟 streamErr + solveProblem 返回 notify_user | EventError 提示 + EventDone + return 错误终止 |
| UC-0003 | HTTP 401 充分条件 | classifyConnectionError(401) | ok=true，不调用问题模型，走原有 retry/exit |
| UC-0004 | 问题模型调用失败 | solveProblem 返回 perr | 回退硬编码路径，EventInfo 显示 KeyProblemSolverFailed |
| UC-0005 | XML 工具格式错误，问题模型返回 prompt_feedback | taskInstructionCache 有 entry | fullFeedback 被 guidance 覆盖，走 prompt 分支 |
| UC-0006 | XML 工具格式错误，问题模型返回 delete_last_msg | 同上 | removeLastAssistantWithToolCalls + continue |
| UC-0007 | context 超限，问题模型返回 feedback | usagePct >= threshold | guidance 追加 + reorganizePending 保持 true |
| UC-0008 | context 超限，问题模型返回 notify_user | 同上 | EventError + return 终止 |
| UC-0009 | problem-solver-enabled=off | 三类异常均触发 | 完全走原有路径，无问题模型调用 |
| UC-0010 | prompt 模板占位符 | buildProblemSolverUserPrompt 长 detail | {ANOMALY_HINT}/{ERROR_DETAIL} 全部填充，detail 截断到 4000 |

## 单元测试

`agent/problem_solver_test.go` 新增：
- `TestApplyProblemAction` — 8 个子场景动作分发
- `TestSolveProblem_Gated` — nil cfg / disabled 门控
- `TestBuildProblemSolverUserPrompt` — 占位符填充 + 截断

## 验收

- `go build ./...`、`go vet ./agent/ ./cmd/ ./i18n/`、`go test ./agent/ ./i18n/ ./cmd/` 全绿

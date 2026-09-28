# FEATURE-568-UC-0006 既有 attempt_completion 行为回归

- 分支：FEATURE-568（v0.82.0）｜类型：回归测试（既有用例全跑）
- 目的：确认命令执行时机前移没有破坏 `attempt_completion` 的既有语义（确认弹框选项、supervisor 报告展示、任务计划收尾、context/meta 校验等）。

## 步骤
1. `go test ./agent/ -run 'AttemptCompletion|TaskPlan|MetaParam|Fix510|Fix552' -v`。
2. `go test ./... -short`。
3. 重点核对既有断言：active 模式下选 `exit` → `completed=true` 且不写 `taskInstructionCache`；选 `next_step`/`not_done` → `completed=false` 且写入 cache；`AttemptCompletionConfirm=false` → 不弹框直接完成（`mgr.captured.Kind == ""`）；supevisor 报告出现在弹框 `Body` 中。

## 期望
- 步骤 1、2：全部通过（无 FAIL）。
- 步骤 3：逐条与改动前一致；`agent/attempt_completion_test.go` 中 4 个既有用例无需修改即通过（本次仅调整命令执行位置，不改返回值语义）。
- `go vet ./...` 与 `go build ./...` 无告警/错误。

## 实测记录
- 时间 / 实例 / BUILD：
- 结果：

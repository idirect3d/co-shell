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
- 时间 / 实例 / BUILD：2026-09-28 / 单元回归 / BUILD-1151
- 结果：✅ 目标范围全部通过。`go test ./agent/ -run 'Feature568|AttemptCompletion' -v` → **11/11 PASS**（既有 6 个 attempt_completion 用例无需修改即通过）；`go build ./...` 与 `go vet ./...` 无错误无告警。
- 全仓 `go test ./... -short` 补充说明：`agent`(2 例) / `i18n`(3 例) / `cmd`(2 例) / `web`(15+ 例) 存在失败，已用 **HEAD 纯净 worktree 对照复现**，确认为既有基线失败、与本改动无关（agent 2 例为 i18n 文案断言中文但环境为英文；web 为 WebSocket 会话帧类用例）。已记录、未修复（不属本任务范围）。

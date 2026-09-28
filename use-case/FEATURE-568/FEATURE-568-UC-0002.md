# FEATURE-568-UC-0002 用户选「继续 / 任务尚未达到目标」时命令也已执行

- 分支：FEATURE-568（v0.82.0）｜类型：单元测试
- 目的：验证命令前移后，即使用户在弹框里选择「继续/未达标」（任务未完成、迭代继续），命令也**已经执行过**（不再回滚、不重复执行）。

## 步骤
1. 构造最小 Agent：`CompletionMode=simple`，顺序记录型 InteractionManager。
2. 调用 `attemptCompletionTool`，args 含 `command`（追加一行到一个临时文件，如 `echo x >> <tmp>/uc2.log`）、`result`、`session_title`、`session_keywords`。
3. `Ask` 返回 `ActionSelect{Value:"continue"}`，再返回 `ActionInput{Value:"再改一下"}`。

## 期望
- `Ask`（弹框）被调用前命令已执行：临时文件已存在 1 行。
- `completed=false`；`taskInstructionCache` 内容为 `再改一下`；返回 `KeyAttemptCompletionContinue` 文案。
- 临时文件行数仍为 1（命令只执行一次，未因用户选择继续而重复执行）。

## 实测记录
- 时间 / 实例 / BUILD：2026-09-28 / 单元测试（TestFeature568CommandRunsOnceWhenUserContinues）/ BUILD-1151
- 结果：✅ 通过（PASS 0.01s）。进入 `Ask` 时标记文件已存在；`completed=false`；`taskInstructionCache="再改一下"`；标记文件恰好 1 行（命令只执行一次，未因用户选择继续而重复）。

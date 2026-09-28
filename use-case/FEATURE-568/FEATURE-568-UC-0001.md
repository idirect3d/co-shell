# FEATURE-568-UC-0001 command 在完成确认弹框之前执行（active 模式）

- 分支：FEATURE-568（v0.82.0）｜类型：单元测试
- 目的：验证 `attempt_completion` 携带的 `command` 在 `completion-mode=active` 弹出「完成确认」对话框（`InteractionManager.Ask`）**之前**就已经执行完毕，即用户在界面上看到弹框时成果已打开。

## 步骤
1. 构造最小 Agent：`CompletionMode=active`，设置一个「顺序记录型」InteractionManager（在 `Ask` 被调用时记录「此刻命令是否已执行」）。
2. 调用 `attemptCompletionTool(ctx, args)`，args 含 `command`（写入一个临时标记文件，如 `touch <tmp>/uc1.marker`）、`result`、`session_title`、`session_keywords`、`next_steps`。
3. 断言 InteractionManager 的 `Ask` 记录：进入 `Ask` 时标记文件已存在。
4. 让 `Ask` 返回 `ActionSelect{Value:"exit"}`，检查最终返回消息。

## 期望
- 步骤 3：`Ask` 被调用时标记文件**已存在**（命令先于弹框执行）——这是本用例的核心断言。
- 步骤 4：任务 `completed=true`；返回消息以「✅ Task completed」形式携带 `result`，且追加命令输出段（`KeySettingCmd_611`）。
- 命令执行日志出现 `attemptCompletion: executing demo command:`。

## 实测记录
- 时间 / 实例 / BUILD：
- 结果：

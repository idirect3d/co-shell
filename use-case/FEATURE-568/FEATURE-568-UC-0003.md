# FEATURE-568-UC-0003 回归：无弹框模式（exit）与无 command 情形

- 分支：FEATURE-568（v0.82.0）｜类型：单元测试（回归）
- 目的：验证命令前移不影响两条既有路径：① `completion-mode=exit`（不弹框直接完成）；② `command` 为空（不执行任何命令、无额外输出）。

## 步骤
1. `CompletionMode=exit` + `command="touch <tmp>/uc3.marker"`：调用 `attemptCompletionTool`，检查 `Ask` 未被调用、标记文件存在、`completed=true`、返回消息含命令输出段。
2. `CompletionMode=simple` + 无 `command`：调用 `attemptCompletionTool`，`Ask` 返回 `exit`。
3. 参数校验回归：缺 `result` / 缺 `session_title` / 缺 `session_keywords` 分别调用。

## 期望
- 步骤 1：`Ask` 未被调用；标记文件存在；`completed=true`；消息含命令输出段。
- 步骤 2：`completed=true`；消息**不含**命令输出段（无多余空行）。
- 步骤 3：分别返回对应错误（`result is required` / `session_title is required...` / `session_keywords is required...`），且都不执行 command。

## 实测记录
- 时间 / 实例 / BUILD：
- 结果：

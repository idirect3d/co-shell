# FEATURE-459 attempt_completion 对话框显示监督信息 — 测试用例

> 版本：v0.27.0 ｜ 分支：FEATURE-459 ｜ 类型：Agent 后端功能
> 前置条件：启动 co-shell，配置监督 LLM（supervisor-enabled=on），执行一个任务并调用 attempt_completion。

## UC-0001 监督放行时对话框显示监督结论
- **前置**：监督 LLM 启用，任务完成，监督 LLM 放行
- **步骤**：调用 attempt_completion，观察 completion-confirm 对话框
- **预期**：对话框信息提示部分显示监督 LLM 的结论（"结论：放行 ✅"）

## UC-0002 监督放行时对话框显示监督理由
- **前置**：监督 LLM 启用，任务完成，监督 LLM 放行（返回 reason）
- **步骤**：调用 attempt_completion，观察 completion-confirm 对话框
- **预期**：对话框信息提示部分显示监督 LLM 的理由（"理由：<reason>"）

## UC-0003 监督放行时对话框显示监督建议
- **前置**：监督 LLM 启用，任务完成，监督 LLM 放行（返回 suggestion）
- **步骤**：调用 attempt_completion，观察 completion-confirm 对话框
- **预期**：对话框信息提示部分显示监督 LLM 的建议（"建议：<suggestion>"）

## UC-0004 监督未启用时对话框不显示监督信息
- **前置**：监督 LLM 未启用（supervisor-enabled=off）
- **步骤**：调用 attempt_completion，观察 completion-confirm 对话框
- **预期**：对话框信息提示部分不显示监督信息（无监督结论/理由/建议）

## UC-0005 监督未返回 report 时对话框不显示监督信息
- **前置**：监督 LLM 启用但未返回 report（如监督调用失败降级放行）
- **步骤**：调用 attempt_completion，观察 completion-confirm 对话框
- **预期**：对话框信息提示部分不显示监督信息（report 为空）

## UC-0006 对话框信息提示部分渲染为 markdown
- **前置**：监督 LLM 启用，任务完成，监督 LLM 放行
- **步骤**：调用 attempt_completion，观察对话框信息提示部分的渲染
- **预期**：监督信息（结论/理由/建议）在信息提示部分渲染为 markdown，格式清晰

## UC-0007 对话框选项仍正常显示
- **前置**：监督 LLM 启用，任务完成，监督 LLM 放行
- **步骤**：调用 attempt_completion，观察对话框选项
- **预期**：对话框选项（next_steps + 给出下一步的建议 + 固定 Keys 选项）仍正常显示，不受监督信息影响

## UC-0008 用户选择"完成退出"后任务正常完成
- **前置**：监督 LLM 启用，任务完成，监督 LLM 放行，对话框显示监督信息
- **步骤**：在对话框选择"完成退出"（-）
- **预期**：任务正常完成，会话更新，监督信息已在对话框中展示

## UC-0009 用户选择其他选项后继续循环
- **前置**：监督 LLM 启用，任务完成，监督 LLM 放行，对话框显示监督信息
- **步骤**：在对话框选择某个 next_step 或"给出下一步的建议"
- **预期**：任务不完成，选择内容作为 user 消息返回主 LLM，继续循环

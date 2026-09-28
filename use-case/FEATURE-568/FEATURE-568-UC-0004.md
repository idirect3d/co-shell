# FEATURE-568-UC-0004 运行时（Web UI）：确认弹框出现时成果文件已打开

- 分支：FEATURE-568（v0.82.0）｜类型：运行时（Web UI 实测）
- 目的：端到端验证真实链路上（Agent 循环 → `attempt_completion` → Web 前端确认框）命令的先执行性，覆盖「用户看到弹框时成果已打开」的用户体验目标。

## 步骤
1. `./work/co-shell-0.82.0.darwin.arm64 --serve --port 28257` 启动 Web UI（**必须带 `--serve`**，避免自动打开系统浏览器占用单客户端连接）。
2. 在 Web 会话中让 agent 执行一个会产生成果文件的任务，并以 `attempt_completion` 结束，且携带 `command`（例如 `open <workspace>/output/uc4.html` 或 `touch <workspace>/tmp/uc4.marker`）。
3. 观察 Web 界面：确认框（`completion-mode` 为 simple 时为「确认任务完成 / 继续补充输入」两个按钮）出现时，检查标记文件/成果窗口是否**已经存在/已打开**。
4. 在弹框上点「继续补充输入」，输入一句新指令，观察迭代继续且命令**未重复执行**。

## 期望
- 步骤 3：弹框渲染完成的那一刻，标记文件已存在（用 `ls -l` 或浏览器中观察成果已打开）。
- 步骤 4：任务未完成（迭代继续），标记文件仍只有一份/一行（命令只执行一次）。
- 后端日志按序出现：`attemptCompletion: executing demo command: ...` → 之后才是交互（`Ask`）相关记录。

## 实测记录
- 时间 / 实例 / BUILD：
- 结果：

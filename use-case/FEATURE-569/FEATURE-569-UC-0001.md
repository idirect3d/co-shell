# FEATURE-569-UC-0001 配置项 browser-window-mode 的默认值与设置命令

- 分支：FEATURE-569（v0.83.0）｜类型：单元 + 运行时结合
- 目的：验证新增配置项 `browser-window-mode` 的默认值、合法取值、非法值拒绝、持久化与设置项可见性。

## 步骤
1. 查看 `config.json` 与 `DefaultConfig().LLM.BrowserWindowMode` 的初始值。
2. 在 REPL 执行 `.set browser-window-mode`（不带参数）。
3. 依次执行 `.set browser-window-mode maximized`、`.set browser-window-mode normal`、`.set browser-window-mode minimized`。
4. 执行非法值：`.set browser-window-mode banana`（以及空值/大写 `MINIMIZED`）。
5. 查看 `config.json` 中 `browser_window_mode` 字段；重启 co-shell 后再执行 `.set browser-window-mode`。
6. 打开 Web UI 设置页（`--serve`）查看是否存在 `browser-window-mode` 项及其默认值标记。
7. 执行 `.set browser-window-mode reset`（若列表显示支持 reset）或设置列表查看默认值列。

## 期望
- 步骤 1：默认值 = `minimized`（`browser_window_mode` 未显式配置时也生效）。
- 步骤 2：输出当前值 `minimized`。
- 步骤 3：三次设置均成功，提示包含新值；`config.json` 中 `browser_window_mode` 依次变为 `maximized` / `normal` / `minimized`。
- 步骤 4：非法值被拒绝并给出用法提示（`minimized|maximized|normal`），**配置保持不变**，无 panic。
- 步骤 5：重启后仍为最后一次成功设置的值（配置被持久化）。
- 步骤 6：Web 设置页显示该项，可编辑，默认值显示 `minimized`。
- 步骤 7：恢复默认后为 `minimized`。

## 实测记录
- 时间 / 实例 / BUILD：
- 结果：

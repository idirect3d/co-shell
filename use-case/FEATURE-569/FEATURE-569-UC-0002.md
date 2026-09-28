# FEATURE-569-UC-0002 非无头模式启动浏览器按配置决定窗口状态（运行时实测）

- 分支：FEATURE-569（v0.83.0）｜类型：运行时（真机）
- 前置：`.set browser-enabled on`；`.set browser-headless off`（非无头）；确保无残留 Chrome 占用调试端口（必要时 `.set browser-port <新端口>`）。
- 目的：验证 `browser-window-mode` 三种取值在**真实 Chrome 窗口**上的效果，以及 headless 模式下该配置完全无效。

## 场景 A — minimized（默认）
1. `.set browser-window-mode minimized`（或恢复默认）。
2. 让 agent 调用 `browser_navigate`（或 `browser_screenshot`）触发浏览器启动。
3. 观察 Chrome 窗口状态；并通过 CDP `Browser.getWindowForTarget` 查询 `windowState`。

期望：Chrome 启动后窗口处于最小化（Dock/任务栏可见、不抢焦点、不遮挡当前工作窗口）；`windowState = "minimized"`。

## 场景 B — maximized
4. `browser_close` 关闭浏览器；`.set browser-window-mode maximized`。
5. 再次 `browser_navigate`。

期望：新启动的 Chrome 窗口最大化（铺满屏幕可用区）；`windowState = "maximized"`。

## 场景 C — normal（默认尺寸）
6. `browser_close`；`.set browser-window-mode normal`。
7. 再次 `browser_navigate`。

期望：窗口为 `browser-window-size`（默认 `1280,800`）的普通窗口，既不最小化也不最大化；`windowState = "normal"`。

## 场景 D — headless 模式下配置无效
8. `.set browser-window-mode maximized`，`.set browser-headless on`，重启浏览器并 `browser_navigate`。
9. 检查 debug 日志是否出现 `Browser.setWindowBounds` 调用。

期望：无任何可见窗口（无头）；**不产生** `Browser.setWindowBounds` 调用；功能（截图/导航）与改动前一致。

## 场景 E — 容错
10. 将 `config.json` 中 `browser_window_mode` 手工改为非法值（如 `"foo"`）或删除该字段，重启后 `browser_navigate`。

期望：不报错、不中断启动，按默认 `minimized`（未知值）行为处理（或仅告警日志），浏览器可正常使用。

## 实测记录
- 时间 / 实例 / BUILD：
- 结果：

# FEATURE-569-UC-0003 窗口模式的 CDP 参数映射与容错（单元测试）

- 分支：FEATURE-569（v0.83.0）｜类型：单元
- 目的：用 recording fake 的 `CDPCaller` 验证窗口模式到 CDP 调用的映射、headless 跳过、未知值不调用与错误容错。

## 步骤
1. 运行 `go test ./browser/ -run 'WindowMode|WindowState' -v`。
2. 单测 A：`ChromeConfig{WindowMode: "minimized"}` 应用后，调用序列 = `Browser.getWindowForTarget` → `Browser.setWindowBounds{windowState:"minimized"}`。
3. 单测 B：`maximized` / `normal` 分别映射为 `windowState:"maximized"` / `"normal"`。
4. 单测 C：`headless=true` 时，任何窗口模式都**不产生** CDP 调用。
5. 单测 D：`WindowMode` 为空串或未知值（如 `"foo"`）时不产生 CDP 调用（保持改动前行为）。
6. 单测 E：`getWindowForTarget` 或 `setWindowBounds` 返回错误时，函数返回 error 且不 panic；调用方（Agent 启动浏览器路径）**不因此中断浏览器启动**，仅记日志。
7. 运行 `go build ./... && go vet ./...`。

## 期望
- 步骤 2-3：CDP 方法与 `windowState` 参数与配置一一对应，顺序稳定（先取 windowId，再设置状态）。
- 步骤 4-5：无任何 CDP 调用（用 fake 断言调用次数为 0）。
- 步骤 6：返回 error 但浏览器仍可用（Agent 侧仅告警）。
- 步骤 7：编译与 vet 全绿；`go test ./...` 无新增失败。

## 实测记录
- 时间 / 实例 / BUILD：
- 结果：

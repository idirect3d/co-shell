# FEATURE-498-UC-0001 Web UI 配置 SSE URL 连接的 MCP server

## 前置条件
- 已编译最新可执行码（`go build -o work/co-shell .`）
- 已启动 Web UI（`./work/co-shell --serve --port <port>`）
- 有一个可用的 SSE MCP server 端点（如 `http://127.0.0.1:8000/sse`）

## 操作步骤
1. 打开 Web UI 系统设置 → MCP Server 区块
2. 在表单中填写：名称（如 `remote-sse`）、URL（如 `http://127.0.0.1:8000/sse`），命令/参数留空
3. 点击保存
4. 观察 MCP server 列表中出现 `remote-sse` 卡片，卡片展示 URL
5. 观察卡片启用开关状态

## 预期结果
- 表单包含 URL 输入框（命令/参数可留空）
- 保存后 MCP server 列表出现 `remote-sse` 卡片，卡片展示 URL
- 后端 config.json 中该 server 的 `url` 字段被正确保存
- 启用状态下连接成功（若端点可达）

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败

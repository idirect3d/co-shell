# FEATURE-498-UC-0002 原有 stdio MCP server 配置不受影响

## 前置条件
- 已编译最新可执行码（`go build -o work/co-shell .`）
- 已启动 Web UI（`./work/co-shell --serve --port <port>`）

## 操作步骤
1. 打开 Web UI 系统设置 → MCP Server 区块
2. 在表单中填写：名称（如 `fs`）、命令（如 `npx`）、参数（如 `-y @modelcontextprotocol/server-filesystem /tmp`），URL 留空
3. 点击保存
4. 观察 MCP server 列表中出现 `fs` 卡片
5. 用 REPL `:mcp list` 确认 `fs` 正常连接并列出工具

## 预期结果
- URL 留空时仍走 stdio 连接（原有行为不变）
- `fs` 卡片正常显示命令与参数
- `:mcp list` 显示 `fs` 已连接且工具列表正常

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败

# FEATURE-391 将 :set 功能迁移到 Web UI — 测试用例

> 版本：v0.1
> 日期：2026-08-21
> 分支：FEATURE-391
> 说明：本文件按阶段组织测试用例，覆盖后端（SessionDeps 新增 SettingsHandler、settings_get/settings_set 消息处理）和前端（设置弹窗动态渲染）。

---

## 阶段一：后端 — SessionDeps 新增 SettingsHandler

### UC-0001 SessionDeps 新增 SettingsHandler 字段
- **前置**：无
- **操作**：检查 `repl/session.go` 的 `SessionDeps` 结构
- **预期**：`SessionDeps` 新增 `SettingsHandler *cmd.SettingsHandler` 字段
- **验证**：单元测试断言字段存在

### UC-0002 repl.go 创建 SessionDeps 时传入 SettingsHandler
- **前置**：无
- **操作**：检查 `repl/repl.go` 创建 SessionDeps 的位置
- **预期**：`SessionDeps{...}` 中传入 `SettingsHandler: r.settingsHandler`
- **验证**：人工审阅代码

---

## 阶段二：后端 — settings_get 消息

### UC-0003 clientMessage 新增 settings_get/settings_set 类型
- **前置**：无
- **操作**：检查 `web/server.go` 的 `clientMessage` 结构
- **预期**：`Type` 字段支持 `settings_get`/`settings_set`；`settings_set` 携带 `key`/`value`
- **验证**：单元测试断言类型枚举

### UC-0004 serverMessage 新增 settings kind
- **前置**：无
- **操作**：检查 `web/server.go` 的 `serverMessage` 结构
- **预期**：`Kind` 字段支持 `settings`（返回设置项列表）和 `settings_result`（返回设置结果）
- **验证**：单元测试断言 kind 枚举

### UC-0005 settings_get 返回结构化设置项
- **前置**：构造 WebSession，注入 mock SettingsHandler
- **操作**：发送 `settings_get` 消息
- **预期**：返回 `{kind:"settings", groups:[{title, items:[{key, value, desc, type, options}]}]}`，按组组织
- **验证**：单元测试断言返回的 JSON 结构

### UC-0006 settings_get 设置项类型推断
- **前置**：同上
- **操作**：检查返回的设置项
- **预期**：布尔项（on/off）→"bool"；数字项→"number"；枚举项→"enum"（含 options）；其他→"string"
- **验证**：单元测试断言各设置项的 type 字段

---

## 阶段三：后端 — settings_set 消息

### UC-0007 settings_set 调用 SettingsHandler.Handle
- **前置**：构造 WebSession，注入 mock SettingsHandler
- **操作**：发送 `settings_set` 消息（含 key、value）
- **预期**：调用 `SettingsHandler.Handle([]string{key, value})`，返回 `{kind:"settings_result", ok, message}`
- **验证**：单元测试断言 Handle 被调用且参数正确

### UC-0008 settings_set 成功/失败结果
- **前置**：同上
- **操作**：分别发送有效和无效的设置
- **预期**：有效设置返回 `ok:true`；无效设置返回 `ok:false` 和错误信息
- **验证**：单元测试断言返回结果

---

## 阶段四：前端 — 设置弹窗动态渲染

### UC-0009 打开设置弹窗时发送 settings_get
- **前置**：浏览器打开 Web UI
- **操作**：点击"系统设置"菜单项
- **预期**：前端发送 `settings_get` 消息，收到 `settings` 后按组渲染设置项
- **验证**：浏览器验证设置弹窗显示设置项

### UC-0010 设置项按类型渲染表单控件
- **前置**：设置弹窗已渲染
- **操作**：检查各设置项的渲染
- **预期**：bool→开关（toggle）；number→数字输入框；enum→下拉框；string→文本输入框
- **验证**：浏览器验证各类型控件渲染正确

### UC-0011 修改设置后发送 settings_set
- **前置**：设置弹窗已渲染
- **操作**：修改一个设置项的值
- **预期**：前端发送 `settings_set` 消息（含 key、value），收到 `settings_result` 后显示结果
- **验证**：浏览器验证设置生效且结果提示显示

### UC-0012 保留现有主题设置
- **前置**：设置弹窗已渲染
- **操作**：检查弹窗顶部
- **预期**：主题设置（前端本地 localStorage）保留在弹窗顶部
- **验证**：浏览器验证主题设置仍可用

---

## 阶段五：整体验证

### UC-0013 编译验证
- **前置**：无
- **操作**：运行 `go build ./... && go vet ./... && go build -o work/co-shell .`
- **预期**：编译通过、vet 无告警、可执行码生成成功
- **验证**：命令执行成功

### UC-0014 构建信息更新
- **前置**：无
- **操作**：检查 main.go 中 build 计数
- **预期**：build 计数 +1，ROADMAP 中 FEATURE-391 任务状态更新并标注 build 计数
- **验证**：人工审阅 main.go 和 ROADMAP.md

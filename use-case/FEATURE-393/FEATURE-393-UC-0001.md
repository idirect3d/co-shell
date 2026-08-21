# FEATURE-393 Web UI 身份与个性设置菜单

## 背景

FEATURE-392 已把 [身份与个性] 分组从设置面板去掉。本任务把身份与个性设置放到 Web UI 左下角 co-shell logo 的悬停菜单中，在一个大表单中同时设置 name/description/principles/capabilities/rules，每项（含名字）旁有独立保存按钮。

## 验收标准

1. 鼠标移向左下角 co-shell logo 时向上弹出菜单，含 [身份与个性] 入口。
2. 点击 [身份与个性] 打开大表单弹窗，同时展示 name（单行）、description/principles/capabilities/rules（多行文本）。
3. 每项（含名字）旁有独立保存按钮，点击后单独保存该字段。
4. 后端 `IdentityJSON()` 返回 name/description/principles/capabilities/rules 当前值。
5. 后端 `SaveIdentity(key,value)` 保存单个字段：
   - name → cfg.LLM.AgentName（config.json）
   - description → cfg.LLM.ModeDescriptions[workMode]（config.json）
   - principles → cfg.LLM.AgentPrinciples（config.json）
   - capabilities → 写外部文件 CAPABILITIES.md
   - rules → 写 cfg.Rules（按行拆分）
6. WebSocket 新增 identity_get/identity_set 消息。
7. 编译通过：`go build ./... && go vet ./... && go build -o work/co-shell .`

## 测试用例

### UC-001 后端 IdentityJSON 返回全部字段
- 前置：配置含 name/description/principles，workspace 含 CAPABILITIES.md
- 步骤：调用 `IdentityJSON()`
- 期望：返回 name/description/principles/capabilities/rules 五个字段的当前值

### UC-002 保存 name
- 前置：无
- 步骤：`SaveIdentity("name", "新名字")`
- 期望：cfg.LLM.AgentName 更新为"新名字"，config 已保存

### UC-003 保存 description
- 前置：无
- 步骤：`SaveIdentity("description", "新描述")`
- 期望：cfg.LLM.ModeDescriptions[当前workMode] 更新为"新描述"，config 已保存

### UC-004 保存 principles
- 前置：无
- 步骤：`SaveIdentity("principles", "新原则")`
- 期望：cfg.LLM.AgentPrinciples 更新为"新原则"，config 已保存

### UC-005 保存 capabilities 到外部文件
- 前置：workspace 目录存在
- 步骤：`SaveIdentity("capabilities", "新能力")`
- 期望：workspace 根目录 CAPABILITIES.md 内容为"新能力"

### UC-006 保存 rules 到 cfg.Rules
- 前置：无
- 步骤：`SaveIdentity("rules", "规则1\n规则2")`
- 期望：cfg.Rules 为 ["规则1", "规则2"]，config 已保存

### UC-007 前端 logo 悬停弹出菜单
- 前置：Web UI 已加载
- 步骤：鼠标移向左下角 logo
- 期望：向上弹出菜单，含 [身份与个性] 入口

### UC-008 前端身份与个性大表单
- 前置：点击 [身份与个性] 入口
- 步骤：打开表单
- 期望：展示 name（单行 input）+ description/principles/capabilities/rules（多行 textarea），每项旁有独立保存按钮

### UC-009 前端每项独立保存
- 前置：表单已打开
- 步骤：修改 name 并点击其保存按钮
- 期望：仅发送 identity_set 保存 name，其他字段不变

### UC-010 编译验证
- 前置：无
- 步骤：运行 `go build ./... && go vet ./... && go build -o work/co-shell .`
- 期望：全部通过，无编译错误、无 vet 告警

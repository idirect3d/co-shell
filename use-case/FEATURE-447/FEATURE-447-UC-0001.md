# FEATURE-447 工具调用透明化 — 测试用例

> 分支：FEATURE-447
> 版本：v0.20.0
> 功能：LLM 每次调用工具时报告任务进展、影响文件、风险等级，前端用颜色标识风险、动态高亮工作区文件。

## 测试环境

- 工作区：`/Users/direct3d/github/co-shell`
- 测试方式：单元测试（`go test ./agent/ -run TestRisk...` 等）+ Web UI 手动验证（`go run . --serve --port <port>`）

---

## UC-0001 风险等级：规则引擎兜底（读文件=low）

**前置**：无
**操作**：调用 `read_file` 工具，参数 `path=/abs/path/to/file.go`
**预期**：
- 规则引擎判定风险等级为 `low`
- 前端工具卡片显示绿色风险标识

---

## UC-0002 风险等级：规则引擎兜底（新建/修改=medium）

**前置**：无
**操作**：调用 `write_to_file` 工具（新建文件）
**预期**：
- 规则引擎判定风险等级为 `medium`
- 前端工具卡片显示黄色风险标识

---

## UC-0003 风险等级：规则引擎兜底（删除/覆盖/写 workspace 外/执行命令=high）

**前置**：无
**操作**：调用 `execute_command` 工具，参数 `command=rm -rf /tmp/foo`
**预期**：
- 规则引擎判定风险等级为 `high`
- 前端工具卡片显示红色风险标识

---

## UC-0004 风险等级：涉及敏感路径=high

**前置**：无
**操作**：调用 `read_file` 工具，参数 `path=/Users/direct3d/.ssh/config`（用户目录敏感文件）
**预期**：
- 规则引擎判定风险等级为 `high`（涉及敏感路径，即使读操作也升级为 high）
- 前端工具卡片显示红色风险标识

---

## UC-0005 风险等级：LLM 自评与规则取较高者

**前置**：无
**操作**：调用 `read_file` 工具，参数 `path=/abs/path/to/file.go`，LLM 自评 `risk=high`（理由：该文件是核心配置）
**预期**：
- 规则引擎判定 `low`，LLM 自评 `high`，最终取较高者 `high`
- 前端工具卡片显示红色风险标识

---

## UC-0006 风险等级：LLM 自评与规则取较高者（规则更高）

**前置**：无
**操作**：调用 `execute_command` 工具，参数 `command=ls -la`，LLM 自评 `risk=low`（理由：只读命令）
**预期**：
- 规则引擎判定 `high`（执行命令），LLM 自评 `low`，最终取较高者 `high`
- 前端工具卡片显示红色风险标识

---

## UC-0007 影响文件：确定性工具从参数解析

**前置**：无
**操作**：调用 `replace_in_file` 工具，参数 `path=/abs/path/to/file.go`
**预期**：
- 影响文件列表 = `["/abs/path/to/file.go"]`（从参数解析，无需 LLM 预测）
- 前端文件树中该文件以 accent 色文字变色高亮（无背景、无边框）

---

## UC-0008 影响文件：预测性工具从 LLM 字段解析

**前置**：无
**操作**：调用 `execute_command` 工具，参数 `command=go build ./...`，LLM 预测 `files=["/abs/path/to/main.go", "/abs/path/to/agent/"]`
**预期**：
- 影响文件列表 = `["/abs/path/to/main.go", "/abs/path/to/agent/"]`（从 LLM 预测字段解析）
- 前端文件树中这些文件以蓝色标识高亮

---

## UC-0009 影响文件：预测性工具最多 3 个

**前置**：无
**操作**：调用 `execute_command` 工具，LLM 预测 `files` 数组包含 5 个路径
**预期**：
- 影响文件列表只保留前 3 个
- 超出部分被截断

---

## UC-0010 影响文件：预测性工具无法确定具体文件时取共有文件夹

**前置**：无
**操作**：调用 `execute_command` 工具，参数 `command=go test ./...`，LLM 预测 `files=["/abs/path/to/agent/", "/abs/path/to/web/"]`
**预期**：
- 影响文件列表 = `["/abs/path/to/agent/", "/abs/path/to/web/"]`（取所有可能影响文件共有的最下一层文件夹路径）
- 前端文件树中这些文件夹以蓝色标识高亮

---

## UC-0011 进展报告：更新已有步骤状态

**前置**：已有任务计划，步骤 0 为 pending
**操作**：调用任意工具，参数 `progress=[{"index":0,"description":"正在实现第 1 步","status":"in_progress"}]`
**预期**：
- 任务计划步骤 0 更新为 in_progress，描述更新
- 推送 `EventTaskPlan` 事件，前端计划面板实时更新

---

## UC-0012 进展报告：index 超出范围=新增

**前置**：已有任务计划，共 3 个步骤（index 0-2）
**操作**：调用任意工具，参数 `progress=[{"index":3,"description":"新增步骤","status":"pending"}]`
**预期**：
- 任务计划新增第 4 个步骤（index 3）
- 推送 `EventTaskPlan` 事件

---

## UC-0013 进展报告：index 超出可新增范畴=报错

**前置**：已有任务计划，共 3 个步骤（index 0-2）
**操作**：调用任意工具，参数 `progress=[{"index":5,"description":"跳号新增","status":"pending"}]`
**预期**：
- 工具调用失败，返回错误（index 5 超出可新增范畴，只能新增 index 3）
- 该错误作为工具调用异常失败处理

---

## UC-0014 进展报告：progress 数组为空

**前置**：无
**操作**：调用任意工具，参数 `progress=[]` 或未提供 progress
**预期**：
- 不更新任务计划，不报错
- 工具正常执行

---

## UC-0015 前端：工具卡片显示风险等级颜色标识

**前置**：Web UI 已启动
**操作**：让 LLM 调用一个 high 风险工具（如 execute_command）
**预期**：
- 工具卡片标题栏显示风险等级标识（红色）
- 风险标识清晰可辨

---

## UC-0016 前端：文件树确定性高亮（accent 色文字变色）

**前置**：Web UI 已启动，工作区有文件
**操作**：让 LLM 调用 `replace_in_file` 修改某个文件
**预期**：
- 文件树中该文件仅文字变色为 accent 色（无背景、无边框）
- 与点选文件的字体颜色一致

---

## UC-0017 前端：文件树预测性高亮（蓝色）

**前置**：Web UI 已启动，工作区有文件
**操作**：让 LLM 调用 `execute_command` 并预测影响文件
**预期**：
- 文件树中预测的文件以蓝色标识高亮
- 与确定性高亮（accent 色）区分明显

---

## UC-0018 前端：计划面板实时更新

**前置**：Web UI 已启动，已有任务计划
**操作**：让 LLM 调用工具并附带 progress 报告
**预期**：
- 计划面板实时显示步骤状态变化
- 新增步骤立即出现在面板中

---

## UC-0019 系统提示词：告知 LLM 评估标准

**前置**：无
**操作**：检查系统提示词中工具调用说明
**预期**：
- 系统提示词包含风险等级评估标准（读=low、新建/改=medium、删/覆盖/写 workspace 外/执行命令=high、涉及敏感路径=high）
- 系统提示词包含影响文件报告要求（确定性工具从参数、预测性工具最多 3 个）
- 系统提示词包含进展报告格式（progress 数组：index/description/status）

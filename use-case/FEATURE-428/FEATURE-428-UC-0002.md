# FEATURE-429 新增模型向导 Web UI 分步表单化

## 背景

FEATURE-422 的 Web 模型向导本质是"把 TUI 文本向导翻译成 Web 弹窗里的文本流 + 通用输入框"，不是真正的分步表单，用户体验差。需要改造前后端接口，使新增模型向导能在 Web UI 上按步骤进行参数配置。

## 方案

新增一套独立的 Web 专用结构化向导接口（与现有 TUI 向导并行共存，不破坏 REPL 体验），后端复用现有逻辑（saveModel/fetchModelSuggestions/detectModelCapabilities/autoCompleteEndpoint 等），前端渲染真正的分步表单。

1. **后端 `cmd/model_web_wizard.go`**：新增结构化向导状态机
   - `WebWizardStep` 枚举：Template / Endpoint / APIKey / ModelName / Capabilities / ModelID / Priority / MaxModelLen / Enabled
   - `WebWizardState`：保存当前步骤 + 已填字段
   - `WebWizardStepData`：描述某一步的表单（字段类型、默认值、选项列表、是否必填、校验规则），供前端渲染
   - 方法：`WebWizardStart(mode, id)` / `WebWizardNext(state, values)` / `WebWizardPrev(state)` / `WebWizardSubmit(state)`
2. **`web/session.go`**：新增 `model_wizard_start` / `model_wizard_next` / `model_wizard_prev` 消息
3. **前端向导弹窗**：改为分步表单（左侧步骤导航 + 右侧表单控件 + 底部上一步/下一步/完成按钮）
4. **保留 TUI 向导不变**（AddModelWizard/editModelWizard 继续使用）

## 验收标准

1. 新增模型向导在 Web UI 上显示为真正的分步表单：左侧步骤导航（9 步，当前步高亮，已完成步可点击回跳）+ 右侧表单控件 + 底部上一步/下一步/完成按钮。
2. 每步表单控件类型正确：模板下拉、endpoint 文本输入、API key 密码框、模型名下拉、能力勾选（👁视觉/🔧工具调用/💭思考）、模型 ID 文本、优先级数字、max_model_len 数字、启用开关。
3. 进入"模型名"步骤自动调用 fetchModelSuggestions 拉取 API 模型列表作为下拉选项（含模板默认）。
4. 进入"能力"步骤自动调用 detectModelCapabilities 检测并预填勾选。
5. 上一步/下一步可正常导航，字段值在步骤间保留。
6. 完成时后端构建 ModelConfig 并保存，模型列表刷新。
7. 编辑模式预填现有模型值，提交时保存修改。
8. TUI 向导（.model add / .model edit）行为不变。
9. 编译通过：`go build ./... && go vet ./... && go build -o work/co-shell .`

## 测试用例

### UC-001 后端结构化向导启动（新增）
- 前置：单元测试环境（`model_web_wizard_test.go`），有模板
- 步骤：调用 `WebWizardStart("add", "")`
- 期望：返回第一步（Template）的 `WebWizardStepData`，含模板选项列表，当前步骤为 Template

### UC-002 后端结构化向导启动（编辑）
- 前置：单元测试环境，存在一个已配置模型
- 步骤：调用 `WebWizardStart("edit", modelID)`
- 期望：返回第一步（Template）的 `WebWizardStepData`，且后续步骤预填现有模型值

### UC-003 后端下一步推进 + 字段校验
- 前置：单元测试环境，向导已启动在 Template 步
- 步骤：调用 `WebWizardNext(state, {template: "deepseek"})`
- 期望：推进到 Endpoint 步，返回 Endpoint 步的 `WebWizardStepData`，默认值为模板 endpoint

### UC-004 后端上一步回退
- 前置：单元测试环境，向导已推进到 Endpoint 步
- 步骤：调用 `WebWizardPrev(state)`
- 期望：回退到 Template 步，返回 Template 步的 `WebWizardStepData`

### UC-005 后端完成提交（新增）
- 前置：单元测试环境，向导已填完所有步骤
- 步骤：调用 `WebWizardSubmit(state)`
- 期望：构建 ModelConfig 并调用 saveModel 保存，返回成功结果，模型列表包含新模型

### UC-006 后端完成提交（编辑）
- 前置：单元测试环境，编辑向导已填完所有步骤
- 步骤：调用 `WebWizardSubmit(state)`
- 期望：更新现有模型并保存，返回成功结果

### UC-007 模型名步骤自动拉取建议
- 前置：单元测试环境，向导推进到 ModelName 步
- 步骤：调用 `WebWizardNext` 进入 ModelName 步
- 期望：返回的 `WebWizardStepData` 含模型建议列表（API 拉取 + 模板默认）

### UC-008 能力步骤自动检测预填
- 前置：单元测试环境，向导推进到 Capabilities 步
- 步骤：调用 `WebWizardNext` 进入 Capabilities 步
- 期望：返回的 `WebWizardStepData` 含检测到的能力（vision/toolcall/thinking）作为默认勾选

### UC-009 Web UI 分步表单渲染
- 前置：Web UI 已启动，模型管理弹窗已打开
- 步骤：点击"＋ 新增模型"，进入向导
- 期望：向导弹窗显示左侧步骤导航（9 步）+ 右侧表单控件 + 底部上一步/下一步/完成按钮，第一步为模板下拉

### UC-010 Web UI 步骤导航
- 前置：Web UI 已启动，新增模型向导进行中
- 步骤：点击"下一步"推进，点击"上一步"回退，点击左侧已完成步骤回跳
- 期望：正确切换步骤，字段值在步骤间保留

### UC-011 Web UI 完成新增模型
- 前置：Web UI 已启动，新增模型向导已填完所有步骤
- 步骤：点击"完成"
- 期望：发送 `model_wizard_submit`，后端保存模型，向导弹窗关闭，模型列表刷新显示新模型

### UC-012 Web UI 编辑模型预填
- 前置：Web UI 已启动，模型管理弹窗已打开，存在至少一个模型
- 步骤：点击模型的"编辑"，进入编辑向导
- 期望：各步骤预填现有模型值，完成时保存修改

### UC-013 TUI 向导不受影响
- 前置：REPL 环境
- 步骤：执行 `.model add` 进入 TUI 向导
- 期望：TUI 向导行为与改造前一致（文本流 + 逐步输入）

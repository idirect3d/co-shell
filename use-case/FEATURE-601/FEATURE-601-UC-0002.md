# FEATURE-601 UC-0002：模型配置向导步骤自由切换（含左侧导航错位修复）

> 分支：**FEATURE-601**　版本：**v0.93.0**　BUILD：**1210（拟）**
> 判定口径：向导左侧导航的 **9 个步骤全部可点击**（含未访问的后续步骤），点击即渲染被点击的那一步（不发生错位）；切换前先把当前表单值收进 `wizard_data`，切换不丢数据；点击第 1 步不再报错、不再关单向导；「上一步 / 下一步 / 手动检测」等既有行为不回归。

## 背景（现状与问题）

1. `web/static/app.js:7830` 只给 `i < curIdx` 的导航项绑定点击 → 未访问的后续步骤不可点，**无法自由切换**。
2. 导航点击发送 `{type:"model_wizard_prev", step: <被点击步骤>}`，后端 `WebWizardPrev`（`cmd/model_web_wizard.go:172-178`）语义是“返回所传步骤的**前一步**” → **点第 k 步渲染第 k−1 步（错位）**。实测：点「3. API Key」发送 `step=api_key`，后端会渲染「2. 接口地址」。
3. 点第 1 步「模板」→ 后端返回 `already at first step` → 前端 `showModelWizardStep` 的 `ok:false` 分支执行 `closeModelWizard()` + alert → **整个向导弹窗被关闭**。

## 变更点（拟实施）

| 位置 | 变更 |
|---|---|
| `web/static/app.js`（`renderWizardStep`） | 导航项**全部**绑定点击：`i === curIdx` 时直接返回；否则先 `collectWizardFields()`，再 `wsSend({type:"model_wizard_refresh", step: order[i], wizard_data: wizardData})`（复用既有“渲染指定步骤”语义，不再用 `model_wizard_prev`，从根上消除错位与首步报错） |
| 后端 | 无需改动：`WebWizardRefresh(data, step)` 已是“按传入 step 渲染该步骤”（`cmd/model_web_wizard.go:184`），`web/session.go` 已有 `model_wizard_refresh` 处理 |
| `cmd/model_web_wizard_test.go` | 新增 `TestWebWizardRefreshRendersRequestedStep`（table-driven：9 个步骤逐个校验返回 step == 传入 step，不联网用例用 -short 门控） |

## 用例与结果

- **UC-01（R）导航 9 项全部可点击** → 待实施；判定：当前处于「5. 能力」时，`#modelWizardNav .wizard-nav-item` 的 `onclick` 数量 = 9（现状实测仅 4）。
- **UC-02（R）点击第 k 步渲染第 k 步（无错位）** → 待实施；判定：点击「3. API Key」发送的消息为 `{type:"model_wizard_refresh", step:"api_key"}`，且回包渲染的步骤标题 = 「API Key」（现状发送 `model_wizard_prev` 并渲染「2. 接口地址」）。
- **UC-03（S/R）后端按传入 step 渲染** → `TestWebWizardRefreshRendersRequestedStep` 覆盖 9 个步骤全部返回同一步骤。
- **UC-04（R）点击第 1 步不再关单向导** → 判定：点击「1. 模板」后向导弹窗仍可见（不被 `closeModelWizard()` 关闭）、无 alert；且渲染「1. 模板」。
- **UC-05（R）切换不丢当前表单值** → 判定：在「5. 能力」把某项开关改动后跳到「7. 优先级」，捕获到的 `wizard_data` 携带刚改动的值（`collectWizardFields()` 生效）。
- **UC-06（R）既有行为回归** → 「上一步」仍发 `model_wizard_prev`（step=当前步）、「下一步」仍发 `model_wizard_next`（step=当前步）、能力步骤「手动检测」按钮与逐能力“检测中… → 支持/不支持”显示仍正常。
- **UC-07（S）构建与测试** → `go build ./... && go vet ./...`、`node --check web/static/app.js` 通过；`go test ./cmd/ -short -run 'TestWebWizard|TestReportDetectProgress'` 通过。

## 不在本次范围

- 不新增“步骤必填项校验后再允许跳转”的拦截：允许自由跳过未填步骤，最终提交仍由 `WebWizardSubmit` → `saveModel` 校验。
- 不改动 `.wizard-nav-item` 的视觉样式（`done/current` 类保持原样）。
- 不改动 `model_wizard_prev/next` 两个消息的既有语义。

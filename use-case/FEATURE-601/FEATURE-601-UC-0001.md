# FEATURE-601 UC-0001：模型配置向导「能力」步骤手动检测

> 分支：**FEATURE-601**　版本：**v0.93.0**　BUILD：**1208**
> 判定口径：模型配置向导的「能力」步骤页出现“手动检测”按钮；点击后复用 `detectModelCapabilities` 重新检测 vision / tool_call / thinking 三项能力并回填三个开关；不影响向导其余流程与首次进入的自动检测。

## 背景

「能力」步骤（`WebWizardCapabilities`，页面标题即“检测模型能力”）目前只在**首次进入且能力值全空**时自动检测一次（`webWizardStepData` 中 `if caps == (config.ModelCapability{}) { caps = h.detectModelCapabilities(...) }`）。编辑已有模型时用的是已保存值、不会重新检测；用户也无法在向导内主动重测。本次新增“手动检测”按钮，复用同一检测方法。

## 变更点

| 位置 | 变更 |
|---|---|
| `cmd/model_web_wizard.go` | `WebWizardData` 增加 `Detect bool`（json `detect,omitempty`）触发标志；capabilities 分支在 `Detect` 为真时强制 `detectModelCapabilities` 并回填 `data.Vision/ToolCall/Thinking`，随后把 `Detect` 重置为 false；新增 `WebWizardDetect(data)` 方法（置 `Detect=true` 后走 `webWizardStepData`） |
| `web/session.go` | 新增 `model_wizard_detect` 消息处理（`handleModelWizardDetect`）→ 调 `WebWizardDetect` → `sendWizardStep` |
| `web/static/app.js` | capabilities 步骤渲染“手动检测”按钮（`setupWizardCapabilitiesExtras`，复用 `.wizard-test-btn` 风格）；点击时 `collectWizardFields()` 后发 `model_wizard_detect`，显示“检测中…” |
| `web/static/app.js` T 表 | 新增中英词条：`wizardDetect` / `wizardDetecting` |
| `cmd/model_web_wizard_test.go` | 新增 `TestWebWizardDetect`（-short 门控）与 `TestWebWizardDetectFlagSurvivesOtherSteps`（不联网） |

## 用例与结果

- **UC-01（R）能力步骤出现“手动检测”按钮** → ✅ **通过（已实测）**。浏览器加载等价部署页面后调用 `renderWizardStep({step:"capabilities", …})`，得到 `capButtons=["手动检测"]`、`capToggles=3`。
- **UC-02（R）点击后强制检测并回填** → ⏳ 待人工端到端验证（需真实 LLM 检测，环境无可用端点）。后端已接入 `detectModelCapabilities` 并把结果回填 `data.Vision/ToolCall/Thinking`；`WebWizardDetect` 返回的 capabilities 步骤三个开关取自已回填的 `data`。
- **UC-03（S/R）Detect 标志检测后被重置** → ✅ `WebWizardDetect` 置 `Detect=true` → 步骤处理消费后写回 `data.Detect=false`；单元测试 `TestWebWizardDetectFlagSurvivesOtherSteps` 验证其它步骤不会清除该标志。
- **UC-04（R）其他步骤无该按钮** → ✅ **通过（已实测）**。`renderWizardStep({step:"priority", …})` 后 `otherButtons=[]`。
- **UC-05（R）向导其余流程与首次自动检测不变（回归）** → ✅ 能力步骤仍保留 `caps == 空 → 自动检测` 的既有分支；`go test ./cmd/ -short -run TestWebWizard` 全部通过（网络用例 -short 跳过）。
- **UC-06（S）构建与测试** → ✅ `go build ./... && go vet ./...` 通过；`node --check web/static/app.js` 通过；`go test ./cmd/ -short -run TestWebWizard` PASS。

## 不在本次范围

- 不改变 `detectModelCapabilities` 的检测实现与超时（沿用 15s×3）。
- 不改变“能力值全空即自动检测”的既有启发式。
- UC-02 的真实端点端到端检测由用户在正式环境确认。

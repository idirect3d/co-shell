# FEATURE-502 PLAN/RESEARCH 模式描述优化

## 背景

PLAN MODE 与 RESEARCH MODE 的系统提示词行为指导文案未突出各自的核心纪律：

- **PLAN MODE**：价值在于**挖掘用户真实需求**、对**模糊之处反复确认**，而不是自行猜测后替用户做决定。现有文案只说明"收集信息 → 制定计划 → 用户批准"，未强调"需求澄清"是第一要务，也未明确禁止猜测替代确认。
- **RESEARCH MODE**：价值在于**结论有据可查**，所有结论必须有**高置信度证据**支撑，不能凭空想像。现有文案只要求"保存原始资料、标注出处"，未把"证据不足不得下结论"提升为硬性纪律。

## 方案

优化系统提示词中的模式行为指导（`i18n/zh_system.go`、`i18n/en_system.go` 的 `KeyWorkModePlan` / `KeyWorkModeResearch`）：

1. **PLAN MODE** 新增/强化要点：
   - 本模式的核心目标是挖掘用户真实需求（而非急于出方案）
   - 对需求中模糊、歧义、信息不足之处，必须用 ask_followup_question 反复确认直至明确
   - 禁止仅凭猜测替用户做决定；不确定就必须问
2. **RESEARCH MODE** 新增/强化要点：
   - 所有结论必须有高置信度证据支撑
   - 禁止凭空想像、推测或使用未经核实的信息作为结论
   - 证据不足时须明确标注不确定或不下结论

**范围约束**：仅改系统提示词；UI 设置界面的简短标签（`KeyWorkModePlanDesc`/`KeyWorkModeResearchDesc`）保持不变。

## 验收标准

1. 中文 PLAN MODE 文案明确包含"挖掘需求 / 反复确认模糊点 / 不猜测替用户决定"的纪律要求。
2. 英文 PLAN MODE 文案包含与中文等价的纪律要求。
3. 中文 RESEARCH MODE 文案明确包含"结论须有高置信度证据 / 不得凭空想像"的纪律要求。
4. 英文 RESEARCH MODE 文案包含与中文等价的要求。
5. 中英文语义一致，无遗漏要点。
6. 编译通过：`go build ./... && go vet ./...`。
7. 运行时系统提示词能正确渲染新模式文案（i18n 取值正确）。

## 测试用例

### UC-001 中文 PLAN MODE 文案含需求挖掘要点
- 前置：无
- 步骤：读取 `i18n/zh_system.go` 中 `KeyWorkModePlan` 文案
- 期望：包含"挖掘需求"（或等价表述）、"反复确认"（或等价表述）等要点

### UC-002 中文 PLAN MODE 文案禁止猜测替用户决定
- 前置：无
- 步骤：读取 `KeyWorkModePlan` 文案
- 期望：包含"不要仅凭猜测替用户做决定"（或等价表述）的明确约束

### UC-003 英文 PLAN MODE 文案含等价要点
- 前置：无
- 步骤：读取 `i18n/en_system.go` 中 `KeyWorkModePlan` 文案
- 期望：包含 uncover/elicit requirements、repeatedly confirm ambiguities、must not guess on the user's behalf 等等价表述

### UC-004 中文 RESEARCH MODE 文案含证据要求
- 前置：无
- 步骤：读取 `i18n/zh_system.go` 中 `KeyWorkModeResearch` 文案
- 期望：包含"高置信度证据"（或等价表述）的要求

### UC-005 中文 RESEARCH MODE 文案禁止凭空想像
- 前置：无
- 步骤：读取 `KeyWorkModeResearch` 文案
- 期望：包含"不得凭空想像/推测"（或等价表述）的明确约束

### UC-006 英文 RESEARCH MODE 文案含等价要点
- 前置：无
- 步骤：读取 `i18n/en_system.go` 中 `KeyWorkModeResearch` 文案
- 期望：包含 high-confidence evidence、must not fabricate/speculate 等等价表述

### UC-007 i18n 取值可正确渲染
- 前置：编译通过
- 步骤：调用 `i18n.T(i18n.KeyWorkModePlan)` / `i18n.T(i18n.KeyWorkModeResearch)`（或运行 `go test ./i18n/`）
- 期望：返回包含新增要点的完整文案，无占位符残留、无编码问题

### UC-008 中英文一致性
- 前置：无
- 步骤：逐条比对中英文文案的要点条数
- 期望：中英文要点一一对应，无单侧缺失

### UC-009 编译验证
- 前置：无
- 步骤：运行 `go build ./... && go vet ./...`
- 期望：全部通过，无编译错误、无 vet 告警

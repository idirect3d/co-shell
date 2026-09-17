# FEATURE-534 用例：Web UI 身份与个性配置界面两栏化（分支 FEATURE-531 / v0.64.0）

> 需求（用户原话）：① 借鉴系统设置界面，左边分类标签，右边是分类设置详细界面，把需要较大录入框的项目分类/分开；
> ② 分类标签为：智能体、用户、核心原则、能力、规则（2026-09-18 追加指令：把原「身份定义」拆为「智能体」与「用户」两个标签，
> 每个标签分别包含名字和描述）；③ 智能体标签包含 agent-name、agent-desc，用户标签包含 user-name、user-desc，
> 其余标签只含相关的一项大文本框。
>
> 决策（用户拍板）：保存粒度沿用「每字段一个保存按钮」；`user-name` / `user-description` 一并扩展后端
> （IdentityJSON 字段 + SaveIdentity 分支 + 中英标签，写入 `./IDENTITY.md`）；窄屏镜像系统设置的两级行为。

## 实现（改动面）

- `web/static/index.html`：身份弹窗 body 改为两栏（`#identityNav` 左分类 + `.settings-pane` 右面板 + `#identityFields` 字段区 + `#identityResult` 结果区 + `#identityBack` 返回按钮），复用 `.settings-body/.settings-nav/.settings-pane/.settings-back` 类。
- `web/static/app.js`：新增 `IDENTITY_CATS`（五分类及其字段 key：智能体 `name/description`、用户 `user-name/user-description`、核心原则、能力、规则）、`identityActiveCat`、`identityCatOf`、`renderIdentityNav`、`selectIdentityCat`、`applyIdentityCat`、`renderIdentityField`、`identityShowList/Pane`；i18n 新增五个分类标签与 `save` 文案。
- `web/static/style.css`：`.identity-box` 尺寸对齐设置弹窗（600×800，≤90vh）；`.identity-fields` 字段列；`.identity-textarea` 最小高度 72px → 220px；窄屏全屏/安全区/标题右对齐选择器加入 `#identity`。
- `cmd/settings_identity.go`：`IdentityJSON()` 新增 `user-name`（text）/`user-description`（textarea）两项；`SaveIdentity()` 新增两者写回 `./IDENTITY.md` 并重建系统提示词。
- `i18n/keys.go` / `zh.go` / `en.go`：新增 `KeyIdentityUserName` / `KeyIdentityUserDesc`（用户名 / 用户描述）。
- 用例：本文件；单测：`cmd/settings_identity_test.go`。

---

## UC-0001（单元）身份表单暴露 user-name / user-description

**前置**：临时 workspace 内 `IDENTITY.md` 含 `## user-name = 梁爽`、`## user-description = 后端工程师`。

**操作**：调用 `SettingsHandler.IdentityJSON()`。

**预期**：
- 字段顺序为 `name, description, user-name, user-description, principles, capabilities, rules`（共 7 项）；
- `user-name` 值 = 「梁爽」、`type` = `text`；`user-description` 值 = 「后端工程师」、`type` = `textarea`；
- 两者 `label` 均为本地化文案（非 i18n key 本身）。

**验证方式**：`go test ./cmd/ -run TestIdentityJSONExposesUserFields`。

---

## UC-0002（单元）保存 user-name / user-description 落到 IDENTITY.md

**操作**：依次 `SaveIdentity("user-name", "梁爽")`、`SaveIdentity("user-description", "后端工程师")`。

**预期**：
- `./IDENTITY.md` 中出现 `## user-name` 与 `## user-description` 分节且值正确；
- `agent.ResolveUserName(ws, "")` / `agent.ResolveUserDescription(ws)` 读回相同值；
- 再保存 `principles`（写 `./PRINCIPLES.md`）不影响上述两个值。

**验证方式**：`go test ./cmd/ -run TestSaveIdentityPersistsUserFields`。

---

## UC-0003（运行时）五分类两栏渲染与切换

**前置**：以 `--serve` 启动带本次改动的构建产物。

**操作**：打开「身份与个性」弹窗，依次点击左栏五个分类。

**预期**：
- 左栏五个分类为「智能体 / 用户 / 核心原则 / 能力 / 规则」，当前分类高亮且有 `>` 箭头；
- 「智能体」右栏显示 2 个字段（Agent 名称单行、Agent 描述多行）；「用户」右栏显示 2 个字段（用户名单行、用户描述多行）；
- 「核心原则」「能力」「规则」右栏各只有 1 个多行文本框；
- 未点击保存时在两个分类间来回切换，已编辑但未保存的文本仍在（行常驻 DOM，仅切换显隐）。

**验证方式**：浏览器实测（`--serve`）+ 截图确认。

---

## UC-0004（运行时）每字段保存与结果提示

**操作**：在「用户」修改「用户名」后点击该字段的「保存」按钮。

**预期**：
- 发送 `{type:"identity_set", key:"user-name", value:...}`，后端返回 `identity_result ok`；
- 右面板顶部出现绿色结果提示并在约 3 秒后消失；
- 刷新（重开弹窗）后显示已保存值；`./IDENTITY.md` 中 `## user-name` 已更新。

**验证方式**：浏览器实测 + 检查 `IDENTITY.md`。

---

## UC-0005（运行时，窄屏）两级导航镜像系统设置

**操作**：将窗口收窄到窄屏断点以下，打开身份弹窗。

**预期**：
- 一级只显示全宽分类列表；点击任一分类进入该分类详情（隐藏列表、显示返回按钮）；
- 点击 `<` 返回到分类列表；
- 弹窗在窄屏下全屏显示（与系统设置一致）。

**验证方式**：浏览器实测（窄窗口截图）。

---

## UC-0006（运行时）大文本框撑满有效空间、保存按钮可见、弹窗不出现滚动条

> 追加指令（2026-09-18）：各标签中的大文本框尽量占满有效空间，但必须让该字段的保存按钮露出来，
> 整个设置页面尽量不出现滚动条；大文本框设一个最低高度兜底。

**前置**：以 `--serve` 启动带本次改动的构建产物；窗口高度 ≥ 800px。

**操作**：打开身份弹窗，依次点击五个分类，量取文本框高度、保存按钮可见性与面板溢出量。

**预期**（实测值，弹窗 600×800 / 面板高 755px）：

| 分类 | 字段与高度 | 保存按钮 | 面板溢出 |
|---|---|---|---|
| 智能体 | 单行输入 39px + 文本框 **548px** | 2 个均完整可见 | 0（无滚动条） |
| 用户 | 单行输入 39px + 文本框 **548px** | 2 个均完整可见 | 0（无滚动条） |
| 核心原则 | 文本框 **662px** | 完整可见 | 0（无滚动条） |
| 能力 | 文本框 **662px** | 完整可见 | 0（无滚动条） |
| 规则 | 文本框 **662px** | 完整可见 | 0（无滚动条） |

**兜底**：把弹窗高度压到 420px（模拟矮窗）后，文本框仍保持 **168px**（CSS `min-height: 160px` 生效），
保存按钮仍完整可见，且面板本身不溢出。

**验证方式**：浏览器实测（`--serve`）——截图确认 + 脚本量取 `getBoundingClientRect()` /
`scrollHeight - clientHeight`；也可量 CSS 计算值 `min-height:160px`。

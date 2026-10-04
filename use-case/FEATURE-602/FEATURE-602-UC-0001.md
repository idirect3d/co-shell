# FEATURE-602 UC-0001：模型管理页「自动选择」开关

> 分支：**FEATURE-602**　版本：**v0.94.0**　BUILD：**1211**
> 判定口径：模型管理弹窗内、模型列表上方左侧出现「自动选择」滑动开关；**默认关闭**（用户 2026-10-04 12:41 指令将默认由“打开”改为“关闭”）；打开/关闭可持久化到 config.json 并在刷新/重启后保持；该开关是「LLM 失败后自动降级」的总开关（关闭时不降级）。

## 背景

模型配置页（`#models` 弹窗，`web/static/index.html`）当前的 `.models-toolbar` 只有「置顶 / 新增模型 / 删除」三个按钮，其下是模型列表 `#modelsBody`。需要在**列表上方左侧**增加一个「自动选择」滑动开关，控制 FEATURE-602 的自动降级策略。

## 变更点（预计）

| 位置 | 变更 |
|---|---|
| `config/config.go` | `LLMConfig` 增加 `AutoModelSelect bool`（json `auto_model_select,omitempty`）+ 访问器 `AutoSelectEnabled()`；零值（缺失/false）⇒ 默认关闭，开启时写入 true |
| `cmd/model_web.go` | 暴露自动选择当前值（新增 `ModelAutoSelect() bool` / `ModelSetAutoSelect(bool)`） |
| `web/session.go` | `handleModelGet` 的 `models` 消息新增 `auto_select` 字段；新增 `model_auto_select` 消息 → 保存 config + 回推 models |
| `web/static/index.html` | `.models-toolbar` 首项（最左）插入「自动选择」开关 |
| `web/static/app.js` | 渲染开关状态、切换时发 `model_auto_select`；新增中英词条 |
| `web/static/style.css` | 开关样式（复用 `.model-toggle` 风格） |

## 用例与结果

- **UC-01（R）开关出现且位置正确** → 打开模型管理弹窗，`.models-toolbar` 的首个可见控件为「自动选择」标签 + 滑动开关，位于模型列表 `#modelsBody` 上方左侧。
- **UC-02（R）默认关闭** → 全新配置（config.json 无 `auto_model_select` 键）下，开关初始为「关」状态；`AutoSelectEnabled()==false`。
- **UC-03（R）打开后持久化** → 打开开关 → 发 `model_auto_select{on:true}` → config.json 出现 `"auto_model_select": true` → 重新加载模型列表，开关仍为开。
- **UC-04（R）关闭后持久化** → 关闭开关 → config.json 的 `auto_model_select` 为 false/缺省 → 重新加载仍为关。
- **UC-05（S）旧配置兼容** → 手工写入不含该键的旧 config.json → 加载后 `AutoSelectEnabled()==false`（与默认一致）。
- **UC-06（S）总开关生效** → 开关默认关闭；显式打开后，关闭时不降级（见 UC-0002/UC-0003），打开时按优先级降级。
- **UC-07（S）构建** → `node --check web/static/app.js` 通过；`go build ./... && go vet ./...` 通过。

## 不在本次范围

- 不改动模型行的启用/禁用开关、置顶/删除按钮。
- 不把该开关加入系统设置页（仅模型管理页），但持久化于 config.json。

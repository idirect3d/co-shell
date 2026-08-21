# FEATURE-394 Web UI 系统设置面板对齐优化

## 背景

Web UI 系统设置面板中，配置项名称（.set-label）默认左对齐，值控件靠右，视觉上名称与值分离。本任务将每行名称-值显示区域分为左右两个等宽块（各 50%）占满整行，名称右对齐、值左对齐，两者靠向中线显示，更紧凑美观。

## 验收标准

1. `.set-label` 改为 `flex: 0 0 50%` + `text-align: right` + 右 padding。
2. `.set-input`/`.set-row select` 改为 `flex: 1` + `width: auto` 占满右半块。
3. `.set-toggle` 保持小尺寸靠右半块左对齐。
4. 去掉 `.set-row` 的 `justify-content: space-between`（避免 checkbox 被推到最右）。
5. 主题上方加「外观」节标题，主题行也等宽双列对齐。
6. 所有节标题居中显示。
7. 动态设置项渲染到独立容器 `settingsDynamic`，与静态外观/主题节分离。
8. 编译通过：`go build ./... && go vet ./... && go build -o work/co-shell .`

## 测试用例

### UC-001 设置面板等宽双列布局
- 前置：Web UI 已加载，打开系统设置弹窗
- 步骤：检查 `.set-row` 的 CSS
- 期望：`.set-row` 为 flex 布局，无 `justify-content: space-between`；`.set-label` 为 `flex: 0 0 50%` + `text-align: right` + 右 padding；`.set-input`/`.set-row select` 为 `flex: 1` + `width: auto`

### UC-002 名称右对齐、值左对齐靠向中线
- 前置：设置弹窗已打开
- 步骤：观察任意设置行
- 期望：名称右对齐、值左对齐，两者靠向中线显示，视觉紧凑

### UC-003 主题上方有「外观」节标题
- 前置：设置弹窗已打开
- 步骤：观察弹窗顶部
- 期望：主题行上方有「[ 外观 ]」节标题，主题行也等宽双列对齐

### UC-004 节标题居中显示
- 前置：设置弹窗已打开
- 步骤：观察所有节标题
- 期望：所有 `.set-group-title` 居中显示

### UC-005 动态设置项渲染到独立容器
- 前置：设置弹窗已打开
- 步骤：检查 DOM
- 期望：动态设置项渲染到 `#settingsDynamic` 容器，与静态外观/主题节分离；`#settingsBody` 保留静态外观节

### UC-006 外观节标题 i18n 键
- 前置：设置弹窗已打开
- 步骤：切换中英文
- 期望：appearance 键值分别为 `[ 外观 ]`（zh）和 `[ Appearance ]`（en）

### UC-007 编译验证
- 前置：无
- 步骤：运行 `go build ./... && go vet ./... && go build -o work/co-shell .`
- 期望：全部通过，无编译错误、无 vet 告警

// Package i18n - Chinese translations for the LLM-driven UI component tree
// protocol (FEATURE-524).
//
// Author: L.Shuang
// Created: 2026-09-15
// MIT License - Copyright (c) 2026 L.Shuang

package i18n

func init() {
	zhMessages[KeyUIUserAction] = `[界面操作] 用户操作了组件 %s，动作「%s」，提交内容：%s`
	zhMessages[KeyUIErrTreeEmpty] = `render_ui 的 tree 参数不能为空。`

	zhMessages[KeyUIErrTreeDecode] = `组件树 JSON 解析失败：%s`

	zhMessages[KeyUIErrTreeTooDeep] = `组件树嵌套层数超过上限（%d 层），请减少嵌套后重试。`

	zhMessages[KeyUIErrTreeTooManyNodes] = `组件树节点数超过上限（%d 个），请拆分成多次渲染。`

	zhMessages[KeyUIErrNodeTypeMissing] = `组件节点缺少 type 字段。可用组件类型：%s`

	zhMessages[KeyUIErrNodeTypeUnknown] = `不支持的组件类型「%s」。可用组件类型：%s`

	zhMessages[KeyUIErrNodePropsTooLarge] = `组件 %s 的 props 体积超过上限（%d 字节），请精简数据（大表格改用文件或分批渲染）。`

	zhMessages[KeyUIErrActionKind] = `组件 %s 声明了不支持的交互方式「%s」，可用：click、select、submit、change。`

	zhMessages[KeyUIErrActionID] = `组件 %s 的交互动作缺少 id 字段。`

	zhMessages[KeyUISummaryNoChildren] = `已渲染 %s（id=%s，无子节点）。`

	zhMessages[KeyUIPrunedTree] = `%s 组件树（%d 个节点）已裁剪为摘要，完整内容见已渲染的结果。`

	zhMessages[KeyUISummaryChildren] = `已渲染 %s（id=%s，%d 个子节点）。`

	zhMessages[KeyUIUpdateSummary] = `已在原位置更新组件 %s（%d 个节点）。`

	zhMessages[KeyUIUpdateNoTarget] = `没有找到 id 为 %s 的已渲染组件，本次更新未生效。`

	zhMessages[KeyUIToolParamUpdate] = `可选字符串：要原地更新的已有组件 id（组件树节点的 id，或本次回执中给出的树 id）。设置后本次调用不新建组件块，而是把该 id 对应的组件在原位置替换为 tree。`

	zhMessages[KeyUIErrRenderFailed] = `组件渲染失败：%s`

	zhMessages[KeyUIWaitingCancelled] = `用户未操作（已取消等待，改为直接发送消息）。`

	zhMessages[KeyUIWaitingTimeout] = `用户未在 %d 秒内操作，已结束等待。`

	zhMessages[KeyUINoRenderer] = `当前界面不支持渲染 UI 组件，已降级为文本输出。`

	zhMessages[KeyUIToolParamTarget] = `可选字符串：渲染目标。stream（默认）渲染在对话流里；window 渲染到 ui_window 打开的常驻窗口里（窗口内容可用 update 原地刷新，窗口不随回合结束消失）。`

	zhMessages[KeyUIErrTarget] = `render_ui 的 target 参数只支持 "stream" 或 "window"，收到「%s」。`

	zhMessages[KeyUIWindowDefaultTitle] = `任务窗口`

	zhMessages[KeyUIWindowOpenSummary] = `已打开界面窗口「%s」（尺寸档位 %s；若视口不足会自动收窄）；若窗口已打开则复用同一窗口，用 render_ui(target="window") 写入或更新内容。`

	zhMessages[KeyUIWindowCloseSummary] = `已关闭界面窗口。`

	zhMessages[KeyUIErrWindowAction] = `ui_window 的 action 参数只支持 "open"、"close"，收到「%s」。`

	zhMessages[KeyUIErrWindowTitle] = `ui_window 的 title 超过长度上限（%d 个字符），请精简。`

	zhMessages[KeyUIErrWindowSize] = `ui_window 的 size 参数只支持 "auto"、"small"、"medium"、"large"，收到「%s」。`

	zhMessages[KeyUIToolParamWindowTitle] = `可选字符串：窗口标题（仅 open 时使用，缺省为“任务窗口”）。`

	zhMessages[KeyUIToolParamWindowSize] = `可选字符串：窗口尺寸档位（仅 open 时使用）。auto（默认）保持内容驱动的现有尺寸；small / medium / large 分别取当前视口可用区（视口减去 32px 边距）的 1/2、2/3、全部。前端会按视口钳制，窗口绝不超出屏幕。`

	zhMessages[KeyUIToolUsageUIWindow] = `## ui_window
Description: 打开或关闭一个常驻浮层窗口（不是对话流里的块）。窗口打开后，可以用 render_ui(target="window") 把组件树渲染进窗口，并在后台继续工作、持续用 render_ui(target="window", update="<id>") 把中间结果推回同一个窗口；窗口在整轮结束后仍然保留，直到你关闭它或用户点关闭按钮。适合“持续更新的面板”：进度看板、长任务的中间结果、需要用户反复填写/确认的表单。

参数:
- action: "open" 打开窗口（已打开时复用同一窗口并更新标题）；"close" 关闭窗口。
- title: 可选字符串，窗口标题（仅 open 时有效）。
- size: 可选字符串，窗口尺寸档位（仅 open 时有效）：auto（默认，保持现有内容驱动尺寸）/ small / medium / large，分别取当前视口可用区的 1/2、2/3、全部；视口不足时前端自动收窄，绝不超出屏幕。你无需指定像素。
- meta: 透明化元数据对象（intent/risk/risk_reason/affected_objects/progress）。

规则:
1. 只有一个窗口：重复 open 不会叠加新窗口，而是复用并更新标题。
2. 窗口内容仍然用 render_ui 写入：target="window" 渲染新组件，update="<id>" 原地更新。
3. 窗口不阻塞你：除用户操作需要等待（见 render_ui 的 waiting 参数）外，你可以继续调用工具并多次推送更新。
4. 用户在窗口里的操作默认开启新一轮对话；若对话仍在运行，则该操作会在下一次 LLM 调用前即时注入当前回合。若该操作声明了 blocking:true 且你正在等待，它会直接作为等待结果返回。
5. 与对话流一样，终端/飞书等不支持窗口的出口会降级为文本，不会报错。
6. 尺寸档位按当前视口的比例计算：同一档位在不同屏幕上像素不同；后端不做像素钳制，由前端保证窗口始终落在可视区内。

用法:
<{XML_TAG_PREFIX}ui_window>
  <{XML_TAG_PREFIX}meta>...</{XML_TAG_PREFIX}meta>
  <{XML_TAG_PREFIX}action>open</{XML_TAG_PREFIX}action>
  <{XML_TAG_PREFIX}title>任务进度</{XML_TAG_PREFIX}title>
</{XML_TAG_PREFIX}ui_window>`

	zhMessages[KeyUIToolUsageRenderUI] = `## render_ui
Description: 把结果渲染成结构化 UI 组件（卡片、键值列表、表格、图表、步骤、提示块等），而不是输出一大段文本。当用户需要看数据分析结果、对比信息、流程说明、进度，或者需要点击/选择时使用。渲染发生在用户的 Web 界面上。

参数:
- tree: 组件树根节点，必须是对象。
- update: 可选字符串，要原地更新的已有组件 id。设置后不新建组件块，而是把该组件在原位置替换为 tree（适合“进度条推进、表格追加一行”这类刷新）。
- waiting: 可选布尔值。true 表示渲染后等待用户操作，用户的动作会作为本工具的返回值立即返回给你（适合你确实需要用户先做选择才能继续的场景）；false 或不传表示立即返回，用户之后的动作会作为一条新的用户消息开启新一轮对话（默认，推荐）。
- meta: 透明化元数据对象（intent/risk/risk_reason/affected_objects/progress）。

组件树节点结构:
{type, id?, props?, children?, actions?}
- type: 组件类型（见下方组件清单）
- id: 可选字符串，同一回合内可用它原地更新该组件
- props: 组件属性（各组件自定义，见下方）
- children: 子节点数组（仅容器类组件使用）
- actions: 交互声明数组 [{on, id, payload}]，on 取 click/select/submit/change，id 是动作标识，payload 取 node/value/row/point/form 决定回传哪些数据

组件清单:
- card: 容器。props{title?, subtitle?, icon?}
- row: 布局容器，children 横向排列（窄屏自动堆叠为单列）。props{gap?}（间距，px）。用于并排分区，可嵌套。
- col: 布局容器，row 里的一列，纵向堆叠多个组件。props{flex?}（占比权重，默认各列等分）。
- kv: 键值列表。props{items:[{k, v}]}
- table: 表格。props{columns:[{key, label?, align?, width?}], rows:[{列key: 值}]}
- chart: 图表。props{kind:"bar"|"line"|"pie", unit?, series:[{name, data:[{label, value}]}]}
- steps: 步骤/时间线。props{items:[{title, desc?, status:"done"|"active"|"pending"}]}
- callout: 提示块。props{variant:"info"|"warn"|"success"|"error", title?, text}
- progress: 进度条。props{value, max?, label?}
- file: 文件卡片。props{path, name?, size?}
- form: 表单。props{title?, fields:[{name, label, type:"text"|"number"|"textarea"|"select"|"checkbox", options?:[{value, label}], value?, placeholder?, rows?}]}；在 form 节点上声明提交动作：actions:[{on:"submit", id:"提交动作 id"}]。
- html: 逃生舱，props{content} 内可写任意 HTML/CSS/JS，在隔离沙箱内渲染，适合上述组件无法表达的图形。仅在你确实需要时使用。

规则:
1. 优先用组件而不是大段文本；数值对比用 chart，明细用 table，结论用 callout。
2. 一次渲染内的顶层组件控制在 1~3 个，避免信息过载。
3. 组件树最多嵌套 %d 层、最多 %d 个节点；单个节点 props 不超过 %d 字节。
4. 需要用户点击某个数据点继续深入时，在该节点上声明 actions:[{on:"select", id:"drill", payload:"point"}]。
5. 渲染历史中的组件树会被压缩为摘要，因此不要把关键数据只放在组件里而不写进你的回复文本。
6. 需要「左栏 + 右栏」版式时用 row 分栏：row 的每个 child 建议用 col 包裹，例如 row{children:[col(steps 流程), col(card 图标, form 表单)]}（左栏流程、右栏上方图标下方表单）；窄屏会自动变为上下堆叠。

用法:
<{XML_TAG_PREFIX}render_ui>
  <{XML_TAG_PREFIX}meta>
    <{XML_TAG_PREFIX}intent>向用户展示销售分析结果</{XML_TAG_PREFIX}intent>
    <{XML_TAG_PREFIX}risk>low</{XML_TAG_PREFIX}risk>
    <{XML_TAG_PREFIX}risk_reason>只读渲染，不修改任何文件</{XML_TAG_PREFIX}risk_reason>
    <{XML_TAG_PREFIX}affected_objects>[]</{XML_TAG_PREFIX}affected_objects>
  </{XML_TAG_PREFIX}meta>
  <{XML_TAG_PREFIX}tree>{"type":"card","props":{"title":"销售分析"},"children":[{"type":"chart","props":{"kind":"bar","series":[{"name":"销售额","data":[{"label":"华东","value":320},{"label":"华南","value":180}]}]}}]}</{XML_TAG_PREFIX}tree>
</{XML_TAG_PREFIX}render_ui>`
}

// Package i18n - English translations for the LLM-driven UI component tree
// protocol (FEATURE-524).
//
// Author: L.Shuang
// Created: 2026-09-15
// MIT License - Copyright (c) 2026 L.Shuang

package i18n

func init() {
	enMessages[KeyUIUserAction] = `[UI action] the user acted on component %s, action "%s", payload: %s`
	enMessages[KeyUIErrTreeEmpty] = `The tree parameter of render_ui must not be empty.`

	enMessages[KeyUIErrTreeDecode] = `Failed to parse the component tree JSON: %s`

	enMessages[KeyUIErrTreeTooDeep] = `The component tree nests deeper than the limit (%d levels). Please reduce nesting and retry.`

	enMessages[KeyUIErrTreeTooManyNodes] = `The component tree has more nodes than the limit (%d). Please split it into several renders.`

	enMessages[KeyUIErrNodeTypeMissing] = `A component node is missing its type field. Available component types: %s`

	enMessages[KeyUIErrNodeTypeUnknown] = `Unsupported component type "%s". Available component types: %s`

	enMessages[KeyUIErrNodePropsTooLarge] = `The props of component %s exceed the size limit (%d bytes). Please trim the data (move large tables to a file or render them in batches).`

	enMessages[KeyUIErrActionKind] = `Component %s declares an unsupported interaction "on" value "%s"; allowed: click, select, submit, change.`

	enMessages[KeyUIErrActionID] = `An interaction action of component %s is missing its id field.`

	enMessages[KeyUISummaryNoChildren] = `Rendered %s (id=%s, no child nodes).`

	enMessages[KeyUIPrunedTree] = `The %s component tree (%d nodes) was pruned to a summary; the rendered result holds the full content.`

	enMessages[KeyUISummaryChildren] = `Rendered %s (id=%s, %d child nodes).`

	enMessages[KeyUIUpdateSummary] = `Updated component %s in place (%d nodes).`

	enMessages[KeyUIUpdateNoTarget] = `No rendered component found with id %s; the update was not applied.`

	enMessages[KeyUIToolParamUpdate] = `Optional string: the id of an already rendered component to update in place (a component node id, or the tree id given in a previous receipt). When set, the call does not create a new component block; it replaces that component with tree at its current position.`

	enMessages[KeyUIErrRenderFailed] = `Component rendering failed: %s`

	enMessages[KeyUIWaitingCancelled] = `The user did not interact (waiting was cancelled because they sent a message instead).`

	enMessages[KeyUIWaitingTimeout] = `The user did not interact within %d seconds; waiting ended.`

	enMessages[KeyUINoRenderer] = `The current interface cannot render UI components; output was degraded to text.`

	enMessages[KeyUIToolParamTarget] = `Optional string: the render target. stream (default) paints into the chat stream; window paints into the persistent window opened by ui_window (its content can be refreshed in place with update, and it outlives the turn).`

	enMessages[KeyUIErrTarget] = `The target parameter of render_ui only accepts "stream" or "window"; got "%s".`

	enMessages[KeyUIWindowDefaultTitle] = `Task window`

	enMessages[KeyUIWindowOpenSummary] = `Opened the UI window "%s"; an already open window is reused, and its content is written or refreshed with render_ui(target="window").`

	enMessages[KeyUIWindowCloseSummary] = `Closed the UI window.`

	enMessages[KeyUIErrWindowAction] = `The action parameter of ui_window only accepts "open" or "close"; got "%s".`

	enMessages[KeyUIErrWindowTitle] = `The ui_window title exceeds the length limit (%d characters); please shorten it.`

	enMessages[KeyUIToolParamWindowTitle] = `Optional string: the window title (open only; defaults to "Task window").`

	enMessages[KeyUIToolUsageUIWindow] = `## ui_window
Description: Open or close a persistent floating window (not a block inside the chat stream). Once open, you can render a component tree into it with render_ui(target="window"), keep working in the background and push intermediate results into the same window with render_ui(target="window", update="<id>"). The window stays open after the turn ends until you close it or the user clicks its close button. Use it for continuously updated panels: progress boards, intermediate results of a long task, or a form the user fills in and confirms repeatedly.

Parameters:
- action: "open" shows the window (an existing window is reused and its title updated); "close" dismisses it.
- title: optional string, the window title (open only).
- meta: the transparency metadata object (intent/risk/risk_reason/affected_objects/progress).

Rules:
1. There is only one window: opening it again reuses the same window and updates the title, it never stacks a second one.
2. The content still comes from render_ui: target="window" renders new components, update="<id>" refreshes them in place.
3. The window does not block you: unless a user action must be awaited (see render_ui's waiting parameter), keep calling tools and push as many updates as you need.
4. A user action in the window starts a new turn by default; while a turn is running it is injected into that turn before your next LLM call. If the action declares blocking:true while you are waiting, it is returned as the wait result instead.
5. As with the chat stream, frontends without window support (terminal, Feishu) degrade to text instead of failing.

Usage:
<{XML_TAG_PREFIX}ui_window>
  <{XML_TAG_PREFIX}meta>...</{XML_TAG_PREFIX}meta>
  <{XML_TAG_PREFIX}action>open</{XML_TAG_PREFIX}action>
  <{XML_TAG_PREFIX}title>Task progress</{XML_TAG_PREFIX}title>
</{XML_TAG_PREFIX}ui_window>`

	enMessages[KeyUIToolUsageRenderUI] = `## render_ui
Description: Render the result as structured UI components (cards, key/value lists, tables, charts, steps, callouts, ...) instead of a wall of text. Use it when the user needs to see data analysis results, comparisons, process explanations or progress, or when they need to click/choose something. Rendering happens in the user's Web interface.

Parameters:
- tree: the component tree root node; must be an object.
- update: optional string, the id of an already rendered component to replace in place. It creates no new component block; the addressed component is swapped for tree at its current position (use it for refreshes such as advancing a progress bar or appending a table row).
- waiting: optional boolean. true means wait for the user to interact after rendering, and their action is returned to you as this tool's result immediately (use it only when you genuinely need the user's choice before continuing); false or omitted means return immediately, and the user's later action starts a new conversation turn as a fresh user message (default, preferred).
- meta: the transparency metadata object (intent/risk/risk_reason/affected_objects/progress).

Node shape:
{type, id?, props?, children?, actions?}
- type: component type (see the catalog below)
- id: optional string; within the same turn it lets you update this component in place
- props: component-specific properties (see below)
- children: array of child nodes (container components only)
- actions: array of interaction declarations [{on, id, payload}]; on is click/select/submit/change, id is the action identifier, payload (node/value/row/point/form) decides what data is sent back

Component catalog:
- card: container. props{title?, subtitle?, icon?}
- row: layout container; children are laid out horizontally (they stack vertically on narrow screens). props{gap?} (spacing in px). Use it for side-by-side sections; rows can nest.
- col: layout container; one column inside a row, stacking several components vertically. props{flex?} (growth weight, equal shares by default).
- kv: key/value list. props{items:[{k, v}]}
- table: table. props{columns:[{key, label?, align?, width?}], rows:[{columnKey: value}]}
- chart: chart. props{kind:"bar"|"line"|"pie", unit?, series:[{name, data:[{label, value}]}]}
- steps: steps / timeline. props{items:[{title, desc?, status:"done"|"active"|"pending"}]}
- callout: notice block. props{variant:"info"|"warn"|"success"|"error", title?, text}
- progress: progress bar. props{value, max?, label?}
- file: file card. props{path, name?, size?}
- form: form. props{title?, fields:[{name, label, type:"text"|"number"|"textarea"|"select"|"checkbox", options?:[{value, label}], value?, placeholder?, rows?}]}; declare the submit action on the form node: actions:[{on:"submit", id:"<action id>"}].
- html: escape hatch; props{content} may contain arbitrary HTML/CSS/JS rendered inside an isolated sandbox. Use it only when the components above cannot express what you need.

Rules:
1. Prefer components over long text: charts for numeric comparisons, tables for detail, callouts for conclusions.
2. Keep 1-3 top-level components per render to avoid overloading the user.
3. A tree may nest at most %d levels and hold at most %d nodes; a single node's props must stay under %d bytes.
4. To let the user click a data point and drill deeper, declare actions:[{on:"select", id:"drill", payload:"point"}] on that node.
5. Component trees in the history are compressed into a summary, so never put critical data only inside a component without also stating it in your reply text.
6. For a left/right layout, split with row and wrap each child in a col, e.g. row{children:[col(steps), col(card with icon, form)]} (steps on the left; icon above the form on the right). Narrow screens stack the columns automatically.

Usage:
<{XML_TAG_PREFIX}render_ui>
  <{XML_TAG_PREFIX}meta>
    <{XML_TAG_PREFIX}intent>Show the sales analysis to the user</{XML_TAG_PREFIX}intent>
    <{XML_TAG_PREFIX}risk>low</{XML_TAG_PREFIX}risk>
    <{XML_TAG_PREFIX}risk_reason>Read-only rendering, no files modified</{XML_TAG_PREFIX}risk_reason>
    <{XML_TAG_PREFIX}affected_objects>[]</{XML_TAG_PREFIX}affected_objects>
  </{XML_TAG_PREFIX}meta>
  <{XML_TAG_PREFIX}tree>{"type":"card","props":{"title":"Sales analysis"},"children":[{"type":"chart","props":{"kind":"bar","series":[{"name":"Revenue","data":[{"label":"East","value":320},{"label":"South","value":180}]}]}}]}</{XML_TAG_PREFIX}tree>
</{XML_TAG_PREFIX}render_ui>`
}

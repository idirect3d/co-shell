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

	enMessages[KeyUIErrTreePrunedSummary] = `The component tree is a context-pruned summary (a summary field without type), not a renderable tree. Submit the full tree including type; to keep full trees in the context, turn off the ui_context_prune switch.`

	enMessages[KeyUIErrTreeTooDeep] = `The component tree nests deeper than the limit (%d levels). Please reduce nesting and retry.`

	enMessages[KeyUIErrTreeTooManyNodes] = `The component tree has more nodes than the limit (%d). Please split it into several renders.`

	enMessages[KeyUIErrNodeTypeMissing] = `A component node is missing its type field. Available component types: %s`

	enMessages[KeyUIErrNodeTypeUnknown] = `Unsupported component type "%s". Available component types: %s`

	enMessages[KeyUIErrNodePropsTooLarge] = `The props of component %s exceed the size limit (%d bytes). Please trim the data (move large tables to a file or render them in batches).`

	enMessages[KeyUIErrActionKind] = `Component %s declares an unsupported interaction "on" value "%s"; allowed: click, select, submit, change.`

	enMessages[KeyUIErrActionID] = `An interaction action of component %s is missing its id field.`

	enMessages[KeyUISummaryNoChildren] = `Rendered %s (id=%s, no child nodes).`

	enMessages[KeyUIPrunedTree] = `The %s component tree (%d nodes) was pruned to a summary (for reading only: never submit render_ui in this shape); the rendered result holds the full content.`

	enMessages[KeyUISummaryChildren] = `Rendered %s (id=%s, %d child nodes).`

	enMessages[KeyUIUpdateSummary] = `Updated component %s in place (%d nodes).`

	enMessages[KeyUIUpdateNoTarget] = `No rendered component found with id %s; the update was not applied.`

	enMessages[KeyUIToolParamUpdate] = `Optional string: the id of an already rendered component to update in place (a component node id, or the tree id given in a previous receipt). When set, the call does not create a new component block; it replaces that component with tree at its current position.`

	enMessages[KeyUIErrRenderFailed] = `Component rendering failed: %s`

	enMessages[KeyUIWaitingCancelled] = `The user did not interact (waiting was cancelled because they sent a message instead).`

	enMessages[KeyUIWaitingTimeout] = `The user did not interact within %d seconds; waiting ended.`

	enMessages[KeyUINoRenderer] = `The current interface cannot render UI components; output was degraded to text.`

	enMessages[KeyUIToolParamTarget] = `Optional string: the render target. stream (default) paints into the chat stream (it scrolls with the conversation); window paints into a bottom-right floating window; panel paints into a panel covering the main message area (same region as the file preview, layered below it, so closing the preview reveals the panel). Window content can be refreshed in place with update and outlives the turn; which window receives it is decided by the window parameter (or the active window).`

	enMessages[KeyUIErrTarget] = `The target parameter of render_ui only accepts "stream", "window" or "panel"; got "%s".`

	enMessages[KeyUIWindowDefaultTitle] = `Task window`

	enMessages[KeyUIWindowOpenSummary] = `Opened the UI window "%s" (size preset %s; narrowed automatically if the viewport is small). An already open window is reused; write or refresh its content with render_ui(target="window").`

	enMessages[KeyUIWindowCloseSummary] = `Closed the UI window.`

	enMessages[KeyUIErrWindowAction] = `The action parameter of ui_window only accepts "open", "close" or "activate"; got "%s".`

	enMessages[KeyUIErrWindowTitle] = `The ui_window title exceeds the length limit (%d characters); please shorten it.`

	enMessages[KeyUIErrWindowName] = `Invalid window name: the length must be 1..%d characters and only letters, digits, CJK, spaces, -, _ and . are allowed (no leading or trailing space).`

	enMessages[KeyUIErrWindowSurface] = `The surface parameter of ui_window only accepts "float" (bottom-right floating window) or "panel" (panel covering the main message area); got "%s".`

	enMessages[KeyUIErrWindowMissing] = `No UI window named "%s": it may have been closed by the user. Check the <ui_windows> list of environment_details first, then decide between opening a new window and updating an existing one.`

	enMessages[KeyUIErrWindowLimit] = `At most %d UI windows can stay open at once; close the ones you no longer need (ui_window(action="close", window="…")).`

	enMessages[KeyUIWindowOpenNamedSummary] = `Opened the UI window "%s" (title "%s", surface %s, size preset %s). Opening the same name again reuses and updates it; write content with render_ui (target="panel" for the panel, target="window" for the floating window).`

	enMessages[KeyUIWindowCloseNamedSummary] = `Closed the UI window "%s".`

	enMessages[KeyUIWindowActivateSummary] = `The UI window "%s" is now the active window.`

	enMessages[KeyUIErrWindowSize] = `The size parameter of ui_window only accepts "auto", "small", "medium" or "large"; got "%s".`

	enMessages[KeyUIToolParamWindowTitle] = `Optional string: the window title (open only; defaults to "Task window").`

	enMessages[KeyUIToolParamWindowName] = `Optional string: the window name (1..32 characters; letters, digits, CJK, spaces, -, _ and . only), used to address a window among several. Defaults to "default" on open and to the active window on close/activate.`

	enMessages[KeyUIToolParamWindowSurface] = `Optional string: the window surface (open only). float (default) = bottom-right floating window; panel = panel covering the main message area (same region as the file preview, layered below it).`

	enMessages[KeyUIToolParamWindowSize] = `Optional string: the window size preset (open only). auto (default) keeps the current content-driven size; small / medium / large take 1/2, 2/3 or all of the usable viewport (viewport minus the 32px margin). The frontend clamps to the viewport, so the window never overflows the screen.`

	enMessages[KeyUIErrShapeContext] = `Shape type "%s" is only valid as a child of a figure node; available shapes: %s`

	enMessages[KeyUIErrFigureChild] = `A figure may only contain shapes as children; got "%s". Available shapes: %s`

	enMessages[KeyUIErrPropsObject] = `The props of component %s must be a JSON object.`

	enMessages[KeyUIErrRequiredProp] = `%s is missing the required field %s.`

	enMessages[KeyUIErrNumberProp] = `The field %s of %s must be a finite number; got "%s".`

	enMessages[KeyUIErrRangeProp] = `The field %s of %s is out of range (%s); got "%s".`

	enMessages[KeyUIErrPropType] = `Invalid value for the field %s of %s; expected %s.`

	enMessages[KeyUIErrPointsFormat] = `The points of %s must be an array like [[x,y], …] or [[x,y,z], …].`

	enMessages[KeyUIErrPointsLimit] = `The points of %s exceed the limit (%d); please simplify and retry.`

	enMessages[KeyUIErrExprTooLong] = `The expr of function exceeds the length limit (%d characters); please simplify it.`

	enMessages[KeyUIErrFigureRange] = `figure: %s must be smaller than %s.`

	enMessages[KeyUIToolUsageUIWindow] = `## ui_window
Description: Create, name or close a persistent window (not a block inside the chat stream). Once open, you can render a component tree into it with render_ui(target="window"|"panel"), keep working in the background and push intermediate results into the same window with render_ui(..., update="<id>"). The window stays open after the turn ends until you close it or the user clicks its close button. Use it for continuously updated panels: progress boards, intermediate results of a long task, or a form the user fills in and confirms repeatedly.

Surfaces: float = bottom-right floating window (default); panel = panel covering the main message area (same region as the file preview, layered below it, so closing the preview reveals the panel).

Parameters:
- action: "open" creates or reuses a window and updates its title/surface; "close" dismisses it (the active window by default); "activate" brings an existing window to the front.
- window: optional string, the window name (1..32 characters; letters, digits, CJK, spaces, -, _ and .); defaults to "default" on open and to the active window on close/activate.
- title: optional string, the window title (open only).
- size: optional string, the window size preset (open only): auto (default, keeps the current content-driven size) / small / medium / large, i.e. 1/2, 2/3 or all of the usable viewport; the frontend narrows it automatically when the viewport is small, so it never overflows. You never pass pixels.
- meta: the transparency metadata object (intent/risk/risk_reason/affected_objects/progress).

Rules:
1. Several windows can coexist (at most 8 per session). Before acting, read the <ui_windows> list of environment_details (name/surface/active/content summary) and update an existing window instead of creating a duplicate; opening the same name only reuses and updates that window.
2. The content still comes from render_ui: target="window" renders new components, update="<id>" refreshes them in place.
3. The window does not block you: unless a user action must be awaited (see render_ui's waiting parameter), keep calling tools and push as many updates as you need.
4. A user action in the window starts a new turn by default; while a turn is running it is injected into that turn before your next LLM call. If the action declares blocking:true while you are waiting, it is returned as the wait result instead.
5. As with the chat stream, frontends without window support (terminal, Feishu) degrade to text instead of failing.
6. A size preset is a fraction of the current viewport, so the same preset yields different pixels on different screens; the backend does not clamp pixels — the frontend guarantees the window stays inside the visible area.
7. The panel surface covers the main message area and the user can collapse it into the floating window or close it at any time; every such change is reported back, so the next <ui_windows> list already reflects it.

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
- target: optional string, the surface to paint on. stream (default, an inline block in the chat stream that scrolls away) / window (bottom-right floating window) / panel (panel covering the main message area, same region as the file preview and layered below it).
- window: optional string, the window name (1..32 characters; letters, digits, CJK, spaces, -, _ and .); only meaningful for target=window/panel, defaults to the active window, and a missing window is created and activated automatically.
- meta: the transparency metadata object (intent/risk/risk_reason/affected_objects/progress).

**Window addressing**: before writing into a window, read the <ui_windows> list of environment_details (name/surface/active/content summary); if a suitable window exists, update it with render_ui(target=..., window="<name>"), otherwise create it with ui_window(action="open", window="<name>", surface="panel"|"float", title="…") (render_ui also creates it automatically) or pick a new name.

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
- figure: vector canvas for 2D/3D diagrams in logical coordinates (geometry, vectors, structure sketches, function curves, 3D boxes). props{title?, xmin/xmax/ymin/ymax? (default 0..10), zmin/zmax?, view? ("front"|"top"|"side"|"iso", default iso), yaw?, pitch?, axes? (default true), grid? (default true), unit?, width? (default 720), height? (default 420), download? (default true)}.
  width/height are **design pixel sizes** (they also define the viewBox and the canvas ratio); the rendered width is min(design width, container width) — a figure is never stretched to fill the container, and it shrinks on a narrow screen. To get a small figure, just pass small numbers (e.g. 360x240) instead of flattening the canvas. Common tiers: small inset 360x240, regular 720x420, large 960x640. Users can wheel-zoom and drag-pan the figure, or use the toolbar buttons to zoom/reset (display only; the downloaded SVG keeps the original design size).
  Its children may only be the primitives below (give a primitive an id to redraw it in place with update):
  - point{x,y,z?,label?,labelPos?,r?,filled?}; segment{x1,y1,z1?,x2,y2,z2?,arrow?:false|"end"|"start"|"both",dash?,label?}
  - polygon{points:[[x,y],...],closed?,fill?,opacity?}; ellipse{cx,cy,z?,rx,ry,rotation?,fill?} (rx==ry draws a circle)
  - label{x,y,z?,text,anchor?,dx?,dy?,size?}; path{points:[[x,y],...],smooth?,dash?,arrow?}
  - axes{x?,y?,z?,grid?,ticks?,step?,xlabel?,ylabel?,zlabel?,origin?} (positionable anywhere; figure.axes=false hides the default axes)
  - function{expr,var? (default x),from,to,z?,samples? (max 400),exprZ?} (expr may be "sin(x)*2": + - * / modulo power plus the whitelisted functions sin/cos/sqrt/log/abs/min/max and the constants pi, e; an array of 2-3 entries gives a parametric curve)
  - as soon as any primitive carries a z coordinate the whole figure switches to an axonometric projection (view preset, or yaw/pitch in degrees); primitives are drawn in children order, later ones on top
  - shared style: stroke takes a semantic colour name accent|info|warn|success|error|dim|none, strokeWidth 1..6, dash solid|dashed|dotted, opacity 0..1; a primitive may declare actions:[{on:"click", id:"..."}] and then reports the coordinates of the clicked point

Rules:
1. Prefer components over long text: charts for numeric comparisons, tables for detail, callouts for conclusions, figures for geometry/structure.
2. Keep 1-3 top-level components per render to avoid overloading the user.
3. A tree may nest at most %d levels and hold at most %d nodes; a single node's props must stay under %d bytes.
4. To let the user click a data point and drill deeper, declare actions:[{on:"select", id:"drill", payload:"point"}] on that node.
5. Component trees in the history are compressed into a summary, so never put critical data only inside a component without also stating it in your reply text.
6. When to use figure: pick figure to convey position/shape/structure (plane geometry, vectors, 3D sketches, custom curves); pick chart for a pure numeric comparison; reach for the html escape hatch only for complex interactive graphics neither can express. A figure holds at most 200 primitives and every coordinate must be a finite number.
7. For a left/right layout, split with row and wrap each child in a col, e.g. row{children:[col(steps), col(card with icon, form)]} (steps on the left; icon above the form on the right). Narrow screens stack the columns automatically.

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

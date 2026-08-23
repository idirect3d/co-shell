/* co-shell web UI (FEATURE-307c) — vanilla JS, no build step.
   Protocol (WebSocket /ws):
     down: {kind:"event", event:{type,level,chan,text,meta}}
           {kind:"ask", id, mode:"line"|"key"}
           {kind:"state", plan:<TaskPlan|null>}
     up:   {type:"input", text, attachments[]}
           {type:"answer", id, value}
           {type:"interrupt"} */
"use strict";

/* ---------- i18n ---------- */

const I18N = {
  zh: {
    workspace: "工作区", refresh: "刷新",
    taskPlan: "任务进展", reply: "回复", interrupt: "打断", send: "发送",
    inputHint: "输入指令，Enter 发送，Shift+Enter 换行，↑↓ 历史",
    connected: "已连接", disconnected: "已断开",
    askLine: "代理请求一行输入：", askKey: "代理请求按键确认：",
    uploadFailed: "上传失败", actionFailed: "操作失败",
    planEmpty: "（无步骤）",
    menu: "菜单", settings: "系统设置", identity: "身份与个性", restart: "重启后台",
    appearance: "[ 外观 ]",
    themeMode: "主题", themeAuto: "跟随系统", themeDark: "深色", themeLight: "浅色",
    statusBar: "状态条",
    sbSession: "Σ", sbLast: "⏱️",
    revealDir: "定位到文件夹",
    sessionDelete: "删除会话",
    sessionActive: "当前会话",
    sessionDeleteConfirm: "确定要删除会话「%s」吗？此操作不可撤销。",
    approveCount: "批准N次",
    approve: "批准", approveAll: "全部批准", approveG: "永久自动执行", approveD: "永久禁用",
    supplement: "补充信息", supplementHint: "输入补充信息，Enter 发送（仍可点击上方按钮）",
    numberHint: "按数字键选择放行次数（0=10次）",
    cancel: "取消", confirm: "确认",
    copyBlock: "复制内容", collapseBlock: "收起同类块", expandBlock: "展开同类块", retryFrom: "从此处重新运行",
    switchMode: "切换工作模式",
    models: "模型管理", modelAdd: "＋ 新增模型", modelWizard: "模型配置向导",
    modelEmpty: "暂无模型，点击上方「＋ 新增模型」添加", modelMenuTitle: "选择主模型", modelVisionMenuTitle: "选择视觉模型", modelVisionEmpty: "暂无视觉模型", modelDefault: "默认", modelDefaultHint: "使用全局默认模型", modelRestoreDefault: "默认",
    fileViewerClose: "关闭", fileViewerLoadFailed: "文件读取失败",
    fileViewerSearch: "搜索文件内容…", fileViewerRaw: "Raw",
    streamModeSilent: "静默", streamModeMinimal: "极简", streamModeNormal: "正常",
    streamTitlePlaceholder: "会话标题", streamTitleHint: "点击修改会话标题",
  },
  en: {
    workspace: "Workspace", refresh: "Refresh",
    taskPlan: "Task Plan", reply: "Reply", interrupt: "Interrupt", send: "Send",
    inputHint: "Type a command — Enter to send, Shift+Enter for newline, ↑↓ history",
    connected: "connected", disconnected: "disconnected",
    askLine: "The agent asks for a line of input:", askKey: "The agent asks for a key:",
    uploadFailed: "Upload failed", actionFailed: "Action failed",
    planEmpty: "(no steps)",
    menu: "Menu", settings: "Settings", identity: "Identity & Personality", restart: "Restart backend",
    appearance: "[ Appearance ]",
    themeMode: "Theme", themeAuto: "Follow system", themeDark: "Dark", themeLight: "Light",
    statusBar: "Status bar",
    sbSession: "Σ", sbLast: "⏱️",
    revealDir: "Reveal in folder",
    sessionDelete: "Delete session",
    sessionActive: "Current session",
    sessionDeleteConfirm: "Delete session \"%s\"? This cannot be undone.",
    approveCount: "Approve N times",
    approve: "Approve", approveAll: "Approve all", approveG: "Always auto-execute", approveD: "Permanently disable",
    supplement: "Supplement", supplementHint: "Type supplementary info, Enter to send (buttons still clickable)",
    numberHint: "Press a digit to choose approve-count (0=10)",
    cancel: "Cancel", confirm: "Confirm",
    copyBlock: "Copy content", collapseBlock: "Collapse same-type blocks", expandBlock: "Expand same-type blocks", retryFrom: "Retry from here",
    switchMode: "Switch work mode",
    models: "Model Manager", modelAdd: "＋ Add Model", modelWizard: "Model Setup Wizard",
    modelEmpty: "No models yet. Click「＋ Add Model」above to add one.", modelMenuTitle: "Select main model", modelVisionMenuTitle: "Select vision model", modelVisionEmpty: "No vision models", modelDefault: "Default", modelDefaultHint: "Use global default model", modelRestoreDefault: "Default",
    fileViewerClose: "Close", fileViewerLoadFailed: "Failed to read file",
    fileViewerSearch: "Search file content…", fileViewerRaw: "Raw",
    streamModeSilent: "Silent", streamModeMinimal: "Minimal", streamModeNormal: "Normal",
    streamTitlePlaceholder: "Session title", streamTitleHint: "Click to edit session title",
  },
};
let T = I18N.zh;
let currentLang = "zh";

function applyI18n() {
  document.querySelectorAll("[data-i18n]").forEach((el) => {
    const k = el.getAttribute("data-i18n");
    if (T[k]) el.textContent = T[k];
  });
  document.querySelectorAll("[data-i18n-ph]").forEach((el) => {
    const k = el.getAttribute("data-i18n-ph");
    if (T[k]) el.placeholder = T[k];
  });
  document.querySelectorAll("[data-i18n-title]").forEach((el) => {
    const k = el.getAttribute("data-i18n-title");
    if (T[k]) el.title = T[k];
  });
  connText.textContent = wsReady ? T.connected : T.disconnected;
  menuBtn.title = T.menu;
}

/* ---------- theme ---------- */

// localStorage "co-shell-theme": "auto" (default, follow the OS) | "dark" |
// "light". setTheme only applies; persistence is the caller's job so that
// "auto" is never clobbered by a resolved value.
const themeToggle = document.getElementById("themeToggle");
const osThemeMQ = window.matchMedia ? window.matchMedia("(prefers-color-scheme: dark)") : null;

function setTheme(name) {
  document.documentElement.setAttribute("data-theme", name);
  themeToggle.textContent = name === "dark" ? "☾" : "☀";
}

function themeMode() {
  const saved = localStorage.getItem("co-shell-theme");
  return saved === "dark" || saved === "light" ? saved : "auto";
}

function applyTheme() {
  const mode = themeMode();
  // No matchMedia (browser cannot report the OS scheme): fall back to dark
  // (FIX-363).
  const dark = mode === "dark" || (mode === "auto" && (!osThemeMQ || osThemeMQ.matches));
  setTheme(dark ? "dark" : "light");
  const sel = document.getElementById("setThemeMode");
  if (sel && sel.value !== mode) sel.value = mode;
}

if (osThemeMQ && osThemeMQ.addEventListener) {
  osThemeMQ.addEventListener("change", () => {
    if (themeMode() === "auto") applyTheme();
  });
}
applyTheme();

themeToggle.onclick = () => {
  const cur = document.documentElement.getAttribute("data-theme");
  localStorage.setItem("co-shell-theme", cur === "dark" ? "light" : "dark");
  applyTheme();
};

/* ---------- DOM handles ---------- */

const conn = document.getElementById("conn");
const connDot = document.getElementById("connDot");
const connText = document.getElementById("connText");
const stream = document.getElementById("stream");
const streamB = document.getElementById("streamB");
const streamA = document.getElementById("streamA");
// FEATURE-419: floating block-boundary navigation icons (top/bottom).
const blockNavTop = document.getElementById("blockNavTop");
const blockNavBottom = document.getElementById("blockNavBottom");
const planPanel = document.getElementById("plan-panel");
const planBody = document.getElementById("planBody");
const layout = document.getElementById("layout");
const askArea = document.getElementById("askArea");
const askHint = document.getElementById("askHint");
const askKeys = document.getElementById("askKeys");
const askLineWrap = document.getElementById("askLineWrap");
const askInput = document.getElementById("askInput");
const askSend = document.getElementById("askSend");
const askInteraction = document.getElementById("askInteraction");
const input = document.getElementById("input");
const sendBtn = document.getElementById("sendBtn");
const modeSeg = document.getElementById("modeSeg");
const modeSegSlider = document.getElementById("modeSegSlider");
const tree = document.getElementById("tree");
const sidebar = document.getElementById("sidebar");
const menuBtn = document.getElementById("menuBtn");
const miWs = document.getElementById("miWs");
const miPlan = document.getElementById("miPlan");
const miWsCheck = document.getElementById("miWsCheck");
const miPlanCheck = document.getElementById("miPlanCheck");
const miSettings = document.getElementById("miSettings");
const miModels = document.getElementById("miModels");
const miRestart = document.getElementById("miRestart");
const settingsModal = document.getElementById("settings");
const settingsClose = document.getElementById("settingsClose");
const settingsBody = document.getElementById("settingsBody");
const settingsDynamic = document.getElementById("settingsDynamic");
const modelsModal = document.getElementById("models");
const modelsClose = document.getElementById("modelsClose");
const modelsBody = document.getElementById("modelsBody");
const modelAddBtn = document.getElementById("modelAddBtn");
const modelWizardModal = document.getElementById("modelWizard");
const modelWizardBody = document.getElementById("modelWizardBody");
const modelWizardCancel = document.getElementById("modelWizardCancel");
const sbModelTextWrap = document.getElementById("sbModelTextWrap");
const sbModelVisionWrap = document.getElementById("sbModelVisionWrap");
const modelMenu = document.getElementById("modelMenu");
const modelVisionMenu = document.getElementById("modelVisionMenu");
const logoWrap = document.getElementById("logoWrap");
const logoMenu = document.getElementById("logoMenu");
const newSessionBtn = document.getElementById("newSessionBtn");
const miIdentity = document.getElementById("miIdentity");
const identityModal = document.getElementById("identity");
const identityClose = document.getElementById("identityClose");
const identityBody = document.getElementById("identityBody");
const setThemeMode = document.getElementById("setThemeMode");
const preview = document.getElementById("preview");
const previewImg = document.getElementById("previewImg");
const previewClose = document.getElementById("previewClose");
// FEATURE-425: read-only text file previewer.
const fileViewer = document.getElementById("fileViewer");
const fvTitle = document.getElementById("fvTitle");
const fvBody = document.getElementById("fvBody");
const fvClose = document.getElementById("fvClose");
const fvSearch = document.getElementById("fvSearch");
const fvRaw = document.getElementById("fvRaw");
// FEATURE-425: main message area title bar (display-mode pill + session title).
const streamMode = document.getElementById("streamMode");
const streamTitle = document.getElementById("streamTitle");
const miStatus = document.getElementById("miStatus");
const miStatusCheck = document.getElementById("miStatusCheck");
const statusbar = document.getElementById("statusbar");
const sbModelText = document.getElementById("sbModelText");
const sbModelVision = document.getElementById("sbModelVision");
const sbSession = document.getElementById("sbSession");
const sbLast = document.getElementById("sbLast");
const sbSessionsWrap = document.getElementById("sbSessionsWrap");
const sbSessions = document.getElementById("sbSessions");
const sessionMenu = document.getElementById("sessionMenu");
const delSessionModal = document.getElementById("delSession");
const delSessionMsg = document.getElementById("delSessionMsg");
const delSessionClose = document.getElementById("delSessionClose");
const delSessionCancel = document.getElementById("delSessionCancel");
const delSessionConfirm = document.getElementById("delSessionConfirm");

/* ---------- websocket ---------- */

let ws = null;
let wsReady = false;

function wsConnect() {
  ws = new WebSocket("ws://" + location.host + "/ws");
  ws.onopen = () => {
    wsReady = true;
    conn.classList.add("on");
    connText.textContent = T.connected;
    wsSend({ type: "mode_get" }); // FEATURE-410: load the work-mode list
    // Fetch the session list on connect so the 💬 count is correct immediately
    // (FEATURE-387), not only after hovering the status-bar item.
    wsSend({ type: "session_list" });
  };
  ws.onclose = () => {
    wsReady = false;
    conn.classList.remove("on");
    connText.textContent = T.disconnected;
    hideAsk();
    setRunning(false);
    setTimeout(wsConnect, 2000);
  };
  ws.onmessage = (m) => {
    let msg;
    try { msg = JSON.parse(m.data); } catch { return; }
    if (msg.kind === "event" && msg.event) renderEvent(msg.event);
    else if (msg.kind === "ask") showAsk(msg);
    else if (msg.kind === "interaction") showInteraction(msg);
    else if (msg.kind === "state") renderPlan(msg.plan || null);
    else if (msg.kind === "sessions") renderSessionMenu(msg.sessions || []);
    else if (msg.kind === "settings") renderSettings(msg.settings || []);
    else if (msg.kind === "settings_result") showSettingsResult(msg);
    else if (msg.kind === "identity") renderIdentity(msg.identity || []);
    else if (msg.kind === "identity_result") showIdentityResult(msg);
    else if (msg.kind === "mode") renderModeSeg(msg.modes || []);
    else if (msg.kind === "mode_result") showModeResult(msg);
    else if (msg.kind === "models") renderModels(msg.models || [], msg.templates || []);
    else if (msg.kind === "model_result") showModelResult(msg);
    else if (msg.kind === "pop_result") {
      // FEATURE-409: retry-from popped the session back; reload so the stream
      // reflects the truncated history.
      if (msg.ok) location.reload();
    }
  };
}

function wsSend(obj) {
  if (wsReady) ws.send(JSON.stringify(obj));
}

/* ---------- event stream rendering ---------- */

const CHAN_LABEL = {
  llm: "LLM", tool: "TOOL", command: "CMD", system: "SYS",
  taskplan: "PLAN", memory: "MEM", mcp: "MCP", db: "DB",
  wizard: "WIZ", debug: "DBG", bridge: "BRG", subagent: "SUB", repl: "REPL",
};

// TOOL_ACTIONS maps a tool name to a human-readable action phrase (zh/en) used
// in the TOOL block title, e.g. "execute_command" -> "执行系统命令" (FEATURE-388).
const TOOL_ACTIONS = {
  execute_command: { zh: "执行系统命令", en: "Run system command" },
  read_file: { zh: "读取文件", en: "Read file" },
  write_to_file: { zh: "写入文件", en: "Write file" },
  replace_in_file: { zh: "修改文件", en: "Edit file" },
  search_files: { zh: "搜索文件", en: "Search files" },
  list_files: { zh: "列出文件", en: "List files" },
  list_code_definition_names: { zh: "列出代码定义", en: "List code definitions" },
  visual_analysis: { zh: "分析图片", en: "Analyze image" },
  browser_navigate: { zh: "打开网页", en: "Open page" },
  browser_screenshot: { zh: "截取网页", en: "Screenshot page" },
  browser_click: { zh: "点击页面", en: "Click page" },
  browser_type: { zh: "输入文本", en: "Type text" },
  browser_scroll: { zh: "滚动页面", en: "Scroll page" },
  browser_evaluate: { zh: "执行脚本", en: "Run script" },
  excel_open: { zh: "打开表格", en: "Open spreadsheet" },
  word_open: { zh: "打开文档", en: "Open document" },
  update_settings: { zh: "更新设置", en: "Update settings" },
  ask_followup_question: { zh: "询问用户", en: "Ask user" },
  launch_sub_agent: { zh: "启动子代理", en: "Launch sub-agent" },
  schedule_task: { zh: "调度任务", en: "Schedule task" },
  track_task_progress: { zh: "更新任务进展", en: "Update task progress" },
  view_task_plan: { zh: "查看任务计划", en: "View task plan" },
  get_memory_slice: { zh: "读取记忆", en: "Read memory" },
  memory_search: { zh: "搜索记忆", en: "Search memory" },
  delete_memory: { zh: "删除记忆", en: "Delete memory" },
  evaluate_expression: { zh: "计算表达式", en: "Evaluate expression" },
  attempt_completion: { zh: "完成任务", en: "Complete task" },
  reorganize_context: { zh: "重组上下文", en: "Reorganize context" },
  shell_send: { zh: "发送命令", en: "Send command" },
  shell_start: { zh: "启动会话", en: "Start session" },
  shell_stop: { zh: "停止会话", en: "Stop session" },
};

// toolAction returns the human-readable action phrase for a tool name.
function toolAction(toolName) {
  const a = TOOL_ACTIONS[toolName];
  if (!a) return toolName;
  return currentLang === "en" ? a.en : a.zh;
}

// parseToolSummary extracts the structured ToolSummary from a tool_call event's
// Meta (FEATURE-388). Returns null when absent or unparseable.
function parseToolSummary(ev) {
  if (!ev.meta || !ev.meta.tool_summary) return null;
  try { return JSON.parse(ev.meta.tool_summary); } catch { return null; }
}

// Streaming blocks: { body, raw, raf, hasResult? }. raw accumulates the
// undecorated text; the body is re-rendered (markdown) via rAF throttle.
let curLLM = null;      // current streaming llm block
let curThinking = null; // current streaming thinking block
let curTool = null;     // current tool block (one block per invocation)
let curREPL = null;     // current repl block (consecutive ui_text lines merge)
let lastMsgIndex = "";  // last message index seen, for the YOU block retry-from

// FEATURE-416: when the stream is split (region A active), scroll region A to
// its bottom; otherwise scroll region B (the whole history).
let splitActive = false; // true while the stream is split into B (static) + A (dynamic)

function scrollStream() {
  if (splitActive) streamA.scrollTop = streamA.scrollHeight;
  else streamB.scrollTop = streamB.scrollHeight;
}

// FEATURE-419: floating block-boundary navigation. When the current block is
// taller than the viewport and its title line / bottom is out of view, show a
// floating ↑ / ↓ icon at the top / bottom of the main data area so the user can
// jump to that boundary. The "current block" is the .ev block with the largest
// visible area inside the viewport.
let blockNavCurrent = null; // the .ev box the icons currently target

function updateBlockNav() {
  if (!blockNavTop || !blockNavBottom) return;
  const scroller = splitActive ? streamA : streamB;
  const viewTop = scroller.scrollTop;
  const viewBottom = viewTop + scroller.clientHeight;
  // Find the .ev block with the largest visible area in the viewport.
  let best = null, bestArea = 0;
  for (const box of scroller.querySelectorAll(".ev")) {
    const top = box.offsetTop;
    const bottom = top + box.offsetHeight;
    const interTop = Math.max(top, viewTop);
    const interBottom = Math.min(bottom, viewBottom);
    if (interBottom > interTop) {
      const area = interBottom - interTop;
      if (area > bestArea) { bestArea = area; best = box; }
    }
  }
  blockNavCurrent = best;
  if (!best) {
    blockNavTop.classList.remove("on");
    blockNavBottom.classList.remove("on");
    return;
  }
  const tallerThanView = best.offsetHeight > scroller.clientHeight;
  const titleHidden = best.offsetTop < viewTop; // title line scrolled above
  const bottomHidden = best.offsetTop + best.offsetHeight > viewBottom; // bottom below
  blockNavTop.classList.toggle("on", tallerThanView && titleHidden);
  blockNavBottom.classList.toggle("on", tallerThanView && bottomHidden);
}

// Clicking ↑ jumps to the current block's title line; ↓ jumps to its bottom.
function bindBlockNav() {
  if (!blockNavTop || !blockNavBottom) return;
  blockNavTop.addEventListener("click", () => {
    if (!blockNavCurrent) return;
    const scroller = splitActive ? streamA : streamB;
    scroller.scrollTop = blockNavCurrent.offsetTop;
  });
  blockNavBottom.addEventListener("click", () => {
    if (!blockNavCurrent) return;
    const scroller = splitActive ? streamA : streamB;
    scroller.scrollTop = blockNavCurrent.offsetTop + blockNavCurrent.offsetHeight - scroller.clientHeight;
  });
  streamB.addEventListener("scroll", updateBlockNav);
  streamA.addEventListener("scroll", updateBlockNav);
}

// splitStream splits the stream into a static region B (history) and a dynamic
// region A (new output). It moves the currently-streaming block(s) into A so
// the user can keep reading B without it jumping to the newest line.
function splitStream() {
  if (splitActive || !running) return;
  splitActive = true;
  // Move the currently-streaming blocks (LLM/THINK/TOOL/REPL) into region A.
  const moving = [curLLM, curThinking, curTool, curREPL].filter(Boolean);
  for (const b of moving) {
    const box = b.body.parentElement;
    if (box && box.parentElement === streamB) streamA.appendChild(box);
  }
  // Show region A and the merge-down button.
  streamA.classList.remove("hidden");
  if (!mergeBtn) {
    mergeBtn = document.createElement("button");
    mergeBtn.className = "stream-merge";
    mergeBtn.title = "向下继续";
    const arrow = document.createElement("span");
    arrow.className = "merge-arrow";
    arrow.textContent = "↓";
    const line = document.createElement("span");
    line.className = "merge-line";
    mergeBtn.appendChild(arrow);
    mergeBtn.appendChild(line);
    mergeBtn.onclick = mergeStream;
    stream.insertBefore(mergeBtn, streamA);
  }
  scrollStream();
  updateBlockNav();
}

// mergeStream merges region A back into region B, restoring a single stream.
function mergeStream() {
  if (!splitActive) return;
  splitActive = false;
  // Move all blocks from A back into B.
  while (streamA.firstChild) streamB.appendChild(streamA.firstChild);
  streamA.classList.add("hidden");
  if (mergeBtn) { mergeBtn.remove(); mergeBtn = null; }
  streamB.scrollTop = streamB.scrollHeight;
  updateBlockNav();
}

let mergeBtn = null; // the floating "merge down" button between B and A

function newStreamBlock(cls, label, msgIndex) {
  const body = makeBlock(cls, label, msgIndex);
  // FEATURE-419: track every block created during this iteration so token_iter
  // can append the token-stats line to the bottom of EACH block, not just the
  // last one. lastBlock always points to the most recent block so token_task
  // (task summary) can append after the task's last block.
  iterBlocks.push(body);
  lastBlock = body;
  return { body, raw: "", raf: 0, hasResult: false };
}

// ensureToolParams creates (or returns) the input-parameter sub-block inside a
// TOOL block (FEATURE-400). The sub-block has a title bar ("输入参数"), a
// "原始内容" pill toggle (FEATURE-412) to the left of the collapse/expand
// toggle, and a scrollable body at fixed height.
function ensureToolParams(curTool) {
  if (curTool.params) return curTool.params;
  const box = curTool.body.parentElement; // .ev.tool
  const params = document.createElement("div");
  params.className = "tool-params";
  const head = document.createElement("div");
  head.className = "tool-params-head";
  const title = document.createElement("span");
  title.className = "tool-params-title";
  title.textContent = "输入参数";
  // FEATURE-412: a small pill toggle controlling whether the params body is
  // md-rendered. Default OFF (md-rendered), so the pill is not active initially.
  const rawPill = document.createElement("button");
  rawPill.className = "tool-params-raw";
  rawPill.textContent = "Raw";
  rawPill.title = "Raw / md 渲染";
  rawPill.onclick = () => {
    rawPill.classList.toggle("on");
    curTool.params.rawMode = !curTool.params.rawMode;
    renderParams(curTool.params);
  };
  const toggle = document.createElement("button");
  toggle.className = "tool-params-toggle";
  toggle.textContent = "⤢";
  toggle.title = "展开/固定高度";
  toggle.onclick = () => {
    params.classList.toggle("expanded");
    toggle.textContent = params.classList.contains("expanded") ? "⤡" : "⤢";
  };
  head.appendChild(title);
  // Group the pill and the collapse/expand toggle on the right so the pill
  // sits immediately to the left of the toggle (FEATURE-412).
  const right = document.createElement("span");
  right.className = "tool-params-right";
  right.appendChild(rawPill);
  right.appendChild(toggle);
  head.appendChild(right);
  const body = document.createElement("div");
  body.className = "tool-params-body";
  params.appendChild(head);
  params.appendChild(body);
  // Insert after the ev-head, before the ev-body.
  box.insertBefore(params, curTool.body);
  curTool.params = { body, raw: "", rawMode: false, diff: false, diffLines: [] };
  return curTool.params;
}

// renderParams renders the params sub-block body according to the "原始内容"
// pill state (FEATURE-412): rawMode ON shows the raw text, OFF md-renders it.
// When params.diff is set (FEATURE-424), the body holds the structured diff
// lines (each with its own status) and is rendered with add/delete/unchanged
// colours.
function renderParams(params) {
  if (params.diff) {
    params.body.classList.remove("md");
    renderDiff(params.body, params.diffLines);
    return;
  }
  if (params.rawMode) {
    params.body.classList.remove("md");
    params.body.textContent = params.raw;
  } else {
    params.body.classList.add("md");
    mdRender(params.body, params.raw);
  }
}

// renderDiff renders the FEATURE-424 unified diff into body. It consumes the
// structured per-line data (each item carries its own status field) so the
// colour is applied directly from the status instead of re-parsing text
// markers. Each item is { line: string, status: "add"|"del"|"ctx" }. The
// content is set via textContent to avoid XSS.
function renderDiff(body, diffLines) {
  body.textContent = "";
  const items = Array.isArray(diffLines) ? diffLines : [];
  for (const item of items) {
    const row = document.createElement("div");
    row.className = "diff-row";
    if (item.status === "add") row.classList.add("diff-add");
    else if (item.status === "del") row.classList.add("diff-del");
    else row.classList.add("diff-ctx");
    row.textContent = item.line || "";
    body.appendChild(row);
  }
}

// scheduleMd re-renders a streaming block as markdown, throttled to one
// render per animation frame no matter how fast chunks arrive.
function scheduleMd(b) {
  if (b.raf) return;
  b.raf = requestAnimationFrame(() => {
    b.raf = null;
    b.body.classList.add("md");
    mdRender(b.body, b.raw);
    scrollStream();
  });
}

function makeBlock(cls, label, msgIndex) {
  const box = document.createElement("div");
  box.className = "ev " + cls;
  if (msgIndex !== undefined && msgIndex !== "") box.dataset.msgIndex = msgIndex;
  const head = document.createElement("div");
  head.className = "ev-head";
  const headLabel = document.createElement("span");
  headLabel.className = "ev-head-label";
  headLabel.textContent = label;
  head.appendChild(headLabel);
  const body = document.createElement("div");
  body.className = "ev-body";
  box.appendChild(head);
  box.appendChild(body);
  // FEATURE-409: add copy / collapse / retry icons to the title bar, except
  // for the final "TOOL: 完成任务" completion block. The YOU block gets copy
  // and retry but no collapse (there is usually only one YOU block).
  if (!(cls === "tool" && /完成任务|Complete task/.test(label))) {
    addBlockActions(head, box, body, cls, cls === "user-msg");
  }
  // FEATURE-425: mark the final completion block (TOOL: 完成任务) as the
  // result block so silent mode keeps it visible.
  if (cls === "tool" && /完成任务|Complete task/.test(label)) box.classList.add("ev-result");
  // FEATURE-425: apply the current display mode to the new block.
  applyBlockDisplayMode(box, cls);
  // FEATURE-416: append the block to region A when the stream is split,
  // otherwise to region B (the whole history).
  (splitActive ? streamA : streamB).appendChild(box);
  scrollStream();
  return body;
}

// applyBlockDisplayMode shows/hides or collapses a single block according to
// the current display mode (FEATURE-425).
function applyBlockDisplayMode(box, cls) {
  if (displayMode === "silent") {
    const show = cls === "user-msg" || box.classList.contains("level-error") || box.classList.contains("ev-result");
    box.style.display = show ? "" : "none";
  } else if (displayMode === "minimal") {
    box.style.display = "";
    const body = box.querySelector(".ev-body");
    const isStreaming = body && isStreamingBody(body);
    const isResult = box.classList.contains("ev-result");
    if (!isStreaming && !isResult) box.classList.add("collapsed");
    else box.classList.remove("collapsed");
  } else {
    box.style.display = "";
    box.classList.remove("collapsed");
  }
}

// addBlockActions appends the copy / collapse / retry icons to a block's title
// bar (FEATURE-409).
// isStreamingBody reports whether the given body belongs to the block that is
// currently streaming output (FEATURE-409).
function isStreamingBody(body) {
  return (curLLM && curLLM.body === body) || (curThinking && curThinking.body === body) ||
         (curTool && curTool.body === body) || (curREPL && curREPL.body === body);
}

// markStreaming flags a block as currently streaming: it shows the dynamic
// "..." next to the title and forces the block expanded so the live content is
// always visible (FEATURE-409).
function markStreaming(body) {
  const box = body.parentElement;
  if (!box) return;
  const s = box.querySelector(".ev-streaming");
  if (s) s.classList.add("on");
  box.classList.remove("collapsed");
}

// unmarkStreaming hides the dynamic "..." of a block once it stops streaming
// (FEATURE-409). In minimal display mode a finished non-result block collapses
// to just its title (FEATURE-425).
function unmarkStreaming(body) {
  if (!body) return;
  const box = body.parentElement;
  if (!box) return;
  const s = box.querySelector(".ev-streaming");
  if (s) s.classList.remove("on");
  if (displayMode === "minimal" && !box.classList.contains("ev-result")) {
    box.classList.add("collapsed");
  }
}

// maybeCollapseEnded re-collapses blocks of a class that just finished
// streaming, but only when that class is collapsed in localStorage AND some
// other block is still streaming (so the session is not fully done). This keeps
// the just-finished block out of the way while the live block stays expanded
// (FEATURE-409).
function maybeCollapseEnded(cls) {
  if (localStorage.getItem("co-shell-collapse-" + cls) !== "1") return;
  // Only re-collapse when some other block is still streaming.
  const anyStreaming = Array.from(document.querySelectorAll(".ev .ev-body")).some(isStreamingBody);
  if (!anyStreaming) return;
  document.querySelectorAll(".ev." + cls).forEach((b) => {
    const bBody = b.querySelector(".ev-body");
    if (bBody && !isStreamingBody(bBody)) b.classList.add("collapsed");
  });
}

function addBlockActions(head, box, body, cls, noCollapse) {
  const actions = document.createElement("span");
  actions.className = "ev-actions";

  // FEATURE-409: a dynamic "..." shown next to the title while the block is
  // streaming output, so the user can see it is still being produced.
  const streaming = document.createElement("span");
  streaming.className = "ev-streaming";
  streaming.textContent = "...";
  head.appendChild(streaming);

  // 1) Copy: copy the block's plain-text content to the clipboard.
  const copy = document.createElement("button");
  copy.className = "ev-act";
  copy.textContent = "⧉";
  copy.title = T.copyBlock;
  copy.onclick = (e) => {
    e.stopPropagation();
    const text = body.innerText || body.textContent || "";
    navigator.clipboard.writeText(text).catch(() => {});
  };
  actions.appendChild(copy);

  // 2) Collapse/expand: collapse all blocks of the same class to just their
  // title bar; the state is persisted in localStorage. Skipped for the YOU
  // block (noCollapse) since there is usually only one of it.
  if (!noCollapse) {
    const collapse = document.createElement("button");
    collapse.className = "ev-act";
    collapse.textContent = "▾";
    collapse.title = T.collapseBlock;
    const storageKey = "co-shell-collapse-" + cls;
    const applyCollapse = () => {
      // FEATURE-409: a block that is currently streaming output is never
      // collapsed, so the user always sees the live dynamic content.
      const collapsed = !isStreamingBody(body) && localStorage.getItem(storageKey) === "1";
      box.classList.toggle("collapsed", collapsed);
      collapse.textContent = collapsed ? "▸" : "▾";
      collapse.title = collapsed ? T.expandBlock : T.collapseBlock;
    };
    collapse.onclick = (e) => {
      e.stopPropagation();
      const collapsed = localStorage.getItem(storageKey) !== "1";
      localStorage.setItem(storageKey, collapsed ? "1" : "0");
      document.querySelectorAll(".ev." + cls).forEach((b) => {
        // Skip blocks that are currently streaming — they stay expanded.
        const bBody = b.querySelector(".ev-body");
        if (bBody && isStreamingBody(bBody)) return;
        b.classList.toggle("collapsed", collapsed);
      });
      applyCollapse();
    };
    applyCollapse();
    actions.appendChild(collapse);
  }

  // 3) Retry-from: pop the session back to this block and re-run.
  const retry = document.createElement("button");
  retry.className = "ev-act";
  retry.textContent = "↻";
  retry.title = T.retryFrom;
  retry.onclick = (e) => {
    e.stopPropagation();
    wsSend({ type: "session_pop", value: String(box.dataset.msgIndex || "") });
  };
  actions.appendChild(retry);

  head.appendChild(actions);
}

function eventClass(ev) {
  const lvl = ev.level ? " level-" + ev.level : "";
  switch (ev.type) {
    case "content_chunk": case "content": return "llm" + lvl;
    case "thinking_chunk": case "thinking": return "thinking" + lvl;
    case "tool_call": case "tool_call_stream": return "tool" + lvl;
    case "command": case "output": return "command" + lvl;
    case "ui_text": return (ev.chan === "repl" ? "repl" : "system") + lvl;
    default: return "system" + lvl;
  }
}

function renderEvent(ev) {
  // FEATURE-409: the message index attached by the backend lets the retry-from
  // action map this block back to a message for :session pop to.
  const msgIndex = ev.meta && ev.meta.msg_index;
  if (msgIndex) lastMsgIndex = msgIndex;
  // Turn-boundary signals from the web session (FEATURE-369): drive the
  // merged send/interrupt button, never render as blocks.
  if (ev.type === "await_input") { setRunning(false); return; }
  if (ev.type === "turn_start") { setRunning(true); return; }
  if (ev.type === "task_plan") {
    let plan = null;
    try { if (ev.meta && ev.meta.plan) plan = JSON.parse(ev.meta.plan); } catch { /* keep null */ }
    renderPlan(plan);
    return;
  }
  if (ev.type === "token_iter" || ev.type === "token_task") {
    const m = ev.meta || {};
    const line = document.createElement("div");
    line.className = "ev meta";
    if (ev.type === "token_iter") {
      // FEATURE-419: per-iteration line with the context message index (ctx_index
      // attached by the backend, matching the sequence number :context displays),
      // timestamp and thousands separators, e.g. "58. 2026-08-23 12:30:58
      // ↑34,670 ↓154 Σ34,824/1,048,576 1.8s 75".
      const seq = m.ctx_index || (++iterCount);
      const parts = [seq + ". " + fmtTime(new Date())];
      if (m.prompt) parts.push("↑" + fmtNum(parseInt(m.prompt, 10) || 0));
      if (m.completion) parts.push("↓" + fmtNum(parseInt(m.completion, 10) || 0));
      if (m.total) parts.push("Σ" + fmtNum(parseInt(m.total, 10) || 0) + (m.max && m.max !== "0" ? "/" + fmtNum(parseInt(m.max, 10) || 0) : ""));
      if (m.ft) parts.push(m.ft);
      if (m.out_tps) parts.push(/^\d+$/.test(m.out_tps) ? fmtNum(parseInt(m.out_tps, 10)) : m.out_tps);
      line.textContent = parts.join("  ");
      // FEATURE-419: place the token line AFTER (outside) every block created
      // during this iteration — each LLM/THINK/TOOL/REPL block is followed by
      // its own token line as a sibling, not nested inside the block body.
      const blocks = iterBlocks.slice();
      iterBlocks = [];
      if (blocks.length) {
        for (const b of blocks) b.parentElement.after(line.cloneNode(true));
      } else {
        stream.appendChild(line);
      }
    } else {
      // FEATURE-419: task-level summary line, total first with prompt/completion
      // in parentheses, e.g. "Σ2,087,596 (↑2,084,349 ↓3,247)". Appended at the
      // end of the stream container (after the last block AND its per-iteration
      // token line), so the order is: block → iteration token line → summary.
      const p = parseInt(m.prompt, 10) || 0;
      const c = parseInt(m.completion, 10) || 0;
      const t = parseInt(m.total, 10) || 0;
      line.textContent = "Σ" + fmtNum(t) + " (↑" + fmtNum(p) + " ↓" + fmtNum(c) + ")";
      if (lastBlock) {
        lastBlock.parentElement.parentElement.appendChild(line);
      } else {
        stream.appendChild(line);
      }
    }
    curLLM = curThinking = null;
    // FEATURE-409: the LLM iteration ended (token usage refreshed) — hide the
    // streaming "..." on all blocks now, not only at the final done event.
    document.querySelectorAll(".ev-streaming").forEach((s) => s.classList.remove("on"));
    scrollStream();
    // FEATURE-419: each display block just completed (the "..." was removed) —
    // refresh the workspace file tree and branch label so the user sees file /
    // branch changes after every iteration, not only when the whole task ends.
    refreshBranch();
    loadTree();
    // FEATURE-378: accumulate token stats into the status bar.
    if (ev.type === "token_iter") {
      const p = parseInt(m.prompt, 10) || 0;
      const c = parseInt(m.completion, 10) || 0;
      tokenStats.sessionIn += p;
      tokenStats.sessionOut += c;
      tokenStats.lastIn = p;
      tokenStats.lastOut = c;
      tokenStats.lastInTPS = parseInt(m.in_tps, 10) || 0;
      tokenStats.lastOutTPS = parseInt(m.out_tps, 10) || 0;
      updateStatus();
    }
    return;
  }
  if (ev.type === "done") {
    curLLM = curThinking = curTool = curREPL = null;
    // FEATURE-419: the task ended — clear any leftover iteration blocks and the
    // last-block pointer so the next task starts with a fresh list.
    iterBlocks = [];
    lastBlock = null;
    // FEATURE-409: hide the dynamic "..." on all blocks once streaming ends.
    document.querySelectorAll(".ev-streaming").forEach((s) => s.classList.remove("on"));
    // An LLM iteration finished — the agent may have switched git branches
    // or modified files, so refresh the branch label and the tree's git
    // status badges without a manual reload.
    refreshBranch();
    loadTree();
    return;
  }

  const streaming = ev.type === "content_chunk" || ev.type === "thinking_chunk";
  if (streaming) {
    if (ev.type === "content_chunk") {
      if (!curLLM) {
        // FEATURE-409: the previous thinking/tool blocks just ended — hide
        // their "..." immediately, then start the new LLM block.
        unmarkStreaming(curThinking && curThinking.body);
        unmarkStreaming(curTool && curTool.body);
        curLLM = newStreamBlock("llm", "LLM", msgIndex);
        curThinking = null;
        curTool = null;
      }
      // FEATURE-409: if their class is collapsed and other blocks are still
      // streaming, re-collapse the just-ended blocks.
      maybeCollapseEnded("thinking");
      maybeCollapseEnded("tool");
      curLLM.raw += ev.text || "";
      markStreaming(curLLM.body);
      scheduleMd(curLLM);
    } else {
      if (!curThinking) {
        // FEATURE-409: the previous llm/tool blocks just ended — hide their
        // "..." immediately, then start the new THINK block.
        unmarkStreaming(curLLM && curLLM.body);
        unmarkStreaming(curTool && curTool.body);
        curThinking = newStreamBlock("thinking", "THINK", msgIndex);
        curLLM = null;
        curTool = null;
      }
      // FEATURE-409: if their class is collapsed and other blocks are still
      // streaming, re-collapse the just-ended blocks.
      maybeCollapseEnded("llm");
      maybeCollapseEnded("tool");
      curThinking.raw += ev.text || "";
      markStreaming(curThinking.body);
      scheduleMd(curThinking);
    }
    scrollStream();
    return;
  }

  // FEATURE-362: one block per tool invocation. Streaming arg fragments
  // accumulate into the input-parameter sub-block (FEATURE-400); the result
  // appends to the ev-body.
  if (ev.type === "tool_call_stream") {
    // FEATURE-XXX: the backend emits a "⚙️ <tool>\n" header at the start of
    // each tool invocation. When one LLM iteration calls multiple tools, this
    // marker lets us open a fresh TOOL block per tool instead of accumulating
    // every tool's args into the first block (which let later calls overwrite
    // earlier ones). The header itself is the tool title (shown in the block
    // header by the tool_call input event), so it is stripped from the params.
    const isNewTool = ev.text && ev.text.includes("⚙️");
    if (!curTool || isNewTool) curTool = newStreamBlock("tool", "TOOL", msgIndex);
    markStreaming(curTool.body);
    const params = ensureToolParams(curTool);
    let text = ev.text || "";
    // Strip the "⚙️ <tool>\n" header line from the params text. indexOf is
    // used instead of a regex because the gear emoji (U+2699 + U+FE0F) is not
    // reliably matched by a regex literal.
    if (isNewTool) {
      const gear = text.indexOf("⚙️");
      if (gear >= 0) {
        const nl = text.indexOf("\n", gear);
        text = text.slice(nl >= 0 ? nl + 1 : text.length);
      }
    }
    params.raw += text;
    // FEATURE-412: render the streaming args according to the "原始内容" pill
    // state — raw text by default, or markdown when the pill is toggled on.
    renderParams(params);
    // Keep the params sub-block scrolled to the last line as streaming args
    // accumulate past its fixed height.
    params.body.scrollTop = params.body.scrollHeight;
    scrollStream();
    return;
  }
  // FEATURE-424: a replace_in_file / write_to_file call completed — the
  // backend sends its unified diff rendering as structured per-line data
  // (each line carries its own status). Replace the streamed params and
  // re-render with add/delete/unchanged colours read directly from the status
  // field.
  if (ev.type === "tool_call_diff") {
    if (!curTool) curTool = newStreamBlock("tool", "TOOL", msgIndex);
    const params = ensureToolParams(curTool);
    params.diff = true;
    params.raw = ev.text || "";
    params.diffLines = [];
    if (ev.meta && ev.meta.diff_lines) {
      try {
        const parsed = JSON.parse(ev.meta.diff_lines);
        if (Array.isArray(parsed)) params.diffLines = parsed;
      } catch (e) {
        params.diffLines = [];
      }
    }
    renderParams(params);
    params.body.scrollTop = params.body.scrollHeight;
    scrollStream();
    return;
  }
  if (ev.type === "tool_call" || (ev.type === "error" && ev.chan === "tool")) {
    const phase = ev.meta && ev.meta.phase;
    const fresh = !curTool || curTool.hasResult;
    if (phase === "input" || (!phase && ev.type === "tool_call" && !fresh && !curTool.raw)) {
      // FEATURE-400: the input-parameter sub-block is the params container;
      // the pre-execution summary no longer replaces the streamed params.
      if (fresh) curTool = newStreamBlock("tool", "TOOL", msgIndex);
      // FEATURE-388: set the TOOL block title to "TOOL <action> - <intent>"
      // from the structured ToolSummary.
      const summary = parseToolSummary(ev);
      if (summary) {
        const head = curTool.body.parentElement.children[0];
        const action = toolAction(summary.tool_name);
        const text = "TOOL: " + action + (summary.intent ? " - " + summary.intent : "");
        // FEATURE-409: update only the title label, preserving the action icons
        // (copy/collapse/retry) and the streaming "..." in the head.
        const label = head.querySelector(".ev-head-label");
        if (label) label.textContent = text;
        else head.textContent = text;
      }
      curTool.hasResult = false;
    } else {
      // result / tool error: append into the same block
      if (!curTool) curTool = newStreamBlock("tool", "TOOL", msgIndex);
      if (ev.type === "error") curTool.body.parentElement.classList.add("level-error");
      const t = (ev.text || "").replace(/^\s*Result:\n/, "");
      curTool.raw += (curTool.raw ? "\n\n" : "") + t;
      curTool.hasResult = true;
      // FEATURE-409: the tool call finished — hide its streaming "...".
      unmarkStreaming(curTool.body);
      scheduleMd(curTool);
      // A tool call finished — the agent may have modified files or switched
      // branches, so refresh the tree and branch label after each call (not
      // only at the end of the whole LLM iteration).
      refreshBranch();
      loadTree();
    }
    curLLM = curThinking = null;
    scrollStream();
    return;
  }

  curLLM = curThinking = null;
  const label = CHAN_LABEL[ev.chan] || (ev.chan || "SYS").toUpperCase();
  // While the model wizard is active, route its ui_text output to the wizard
  // modal instead of the main event stream (FEATURE-422).
  if (ev.type === "ui_text" && ev.chan === "repl" && wizardActive) {
    appendWizardText(ev.text || "");
    return;
  }
  // ui_text from the repl channel renders as a REPL block (parallel to
  // TOOL/LLM); consecutive lines merge into one block. Other ui_text stays SYS.
  if (ev.type === "ui_text" && ev.chan === "repl") {
    if (!curREPL) curREPL = newStreamBlock("repl", "REPL", msgIndex);
    curREPL.raw += (curREPL.raw ? "\n" : "") + (ev.text || "");
    curREPL.body.textContent = curREPL.raw;
    scrollStream();
    return;
  }
  curREPL = null;
  const blockLabel = ev.type === "ui_text" ? "SYS" : label;
  const body = makeBlock(eventClass(ev), blockLabel, msgIndex);
  if (ev.type === "content" || ev.type === "thinking") {
    body.classList.add("md");
    mdRender(body, ev.text || "");
  } else {
    body.textContent = ev.text || "";
  }
}

function renderUserEcho(text) {
  // A new user input (command / confirmation / selection) starts a fresh
  // REPL output block: reset curREPL so the next ui_text repl event opens a
  // new block instead of appending to the previous one (FIX-411).
  curREPL = null;
  const body = makeBlock("user-msg", "YOU", lastMsgIndex);
  body.textContent = text;
  // FEATURE-419: after creating the YOU block, scroll to the bottom on the
  // next frame (once the browser has rendered the new block and grown
  // streamB.scrollHeight). Without this, a user's Enter on a long history can
  // leave streamB not at the bottom, which the scroll listener misreads as an
  // intentional scroll-up and spuriously triggers the auto-split (FEATURE-416).
  requestAnimationFrame(scrollStream);
}

/* ---------- panel visibility (workspace / plan) ---------- */

// panelPrefs persists the user's panel toggles (localStorage
// "co-shell-panels": {ws, plan}); absent or true means visible. The plan
// panel additionally requires an active plan (lastPlan).
const panelPrefs = (() => {
  try { return JSON.parse(localStorage.getItem("co-shell-panels")) || {}; } catch { return {}; }
})();
let lastPlan = null;

function savePanelPrefs() {
  localStorage.setItem("co-shell-panels", JSON.stringify(panelPrefs));
}

function applyPanels() {
  const showWs = panelPrefs.ws !== false;
  layout.classList.toggle("no-ws", !showWs);
  miWsCheck.classList.toggle("on", showWs);
  const showPlan = panelPrefs.plan !== false && !!lastPlan;
  planPanel.classList.toggle("hidden", !showPlan);
  layout.classList.toggle("no-plan", !showPlan);
  miPlanCheck.classList.toggle("on", panelPrefs.plan !== false);
}

miWs.onclick = () => {
  panelPrefs.ws = panelPrefs.ws === false; // true/undefined -> false, false -> true
  savePanelPrefs();
  applyPanels();
};
miPlan.onclick = () => {
  panelPrefs.plan = panelPrefs.plan === false;
  savePanelPrefs();
  applyPanels();
};

/* ---------- status bar (FEATURE-378) ---------- */

// statusPref persists the status bar visibility (localStorage
// "co-shell-status": true/false; absent means visible).
let statusOn = localStorage.getItem("co-shell-status") !== "0";

function applyStatus() {
  statusbar.classList.toggle("hidden", !statusOn);
  miStatusCheck.classList.toggle("on", statusOn);
}

miStatus.onclick = () => {
  statusOn = !statusOn;
  localStorage.setItem("co-shell-status", statusOn ? "1" : "0");
  applyStatus();
};

// Token stats accumulated from token_iter events. sessionIn/sessionOut are
// the running totals across all iterations; last* hold the most recent
// iteration's values (including input/output tokens-per-second).
const tokenStats = { sessionIn: 0, sessionOut: 0, lastIn: 0, lastOut: 0, lastInTPS: 0, lastOutTPS: 0 };

// FEATURE-419: per-iteration counter shown in the token-stats line (the same
// sequence number :context displays for each message). Incremented on every
// token_iter event.
let iterCount = 0;

// FEATURE-419: the .ev-body of every block (LLM/THINK/TOOL/REPL) created during
// the current iteration. token_iter appends the token-stats line to the bottom
// of EACH block in this list (so every block shows its own token line), then
// clears the list for the next iteration.
let iterBlocks = [];

// FEATURE-419: the .ev-body of the most recently created block. token_task
// (task-level summary) appends its line after this LAST block of the task only.
let lastBlock = null;

// fmtTime formats a Date as "YYYY-MM-DD HH:mm:ss" for the token-stats line.
function fmtTime(d) {
  const p = (n) => String(n).padStart(2, "0");
  return d.getFullYear() + "-" + p(d.getMonth() + 1) + "-" + p(d.getDate()) + " " +
    p(d.getHours()) + ":" + p(d.getMinutes()) + ":" + p(d.getSeconds());
}

// modelInfo holds the active text/vision model context info from bootstrap
// (FEATURE-378): { textModel, textMaxLen, visionModel, visionMaxLen }.
let modelInfo = null;

function fmtNum(n) { return n ? n.toLocaleString() : "0"; }

// fmtDur formats a duration in seconds as e.g. "2s" or "1.5m".
function fmtDur(sec) {
  if (!(sec > 0)) return "-";
  return sec >= 60 ? (sec / 60).toFixed(1) + "m" : sec.toFixed(1) + "s";
}

// fmtPct formats a context-usage percentage (0-100) as e.g. "89%".
function fmtPct(used, max) {
  if (!(max > 0)) return "-";
  return Math.round(used * 100 / max) + "%";
}

// fmtLen formats a context length with K/M units (e.g. 1048576 -> "1M").
function fmtLen(n) {
  if (!(n > 0)) return "-";
  if (n >= 1000000) return (n / 1000000).toFixed(1).replace(/\.0$/, "") + "M";
  if (n >= 1000) return (n / 1000).toFixed(1).replace(/\.0$/, "") + "K";
  return String(n);
}

function updateStatus() {
  // Model context usage: 🧠{text}(89% of 1M) 👀{vision}(50% of 1M). The
  // usage numerator is the last turn's input+output tokens (FEATURE-378).
  const sIn = tokenStats.sessionIn, sOut = tokenStats.sessionOut;
  const total = sIn + sOut;
  const lastTotal = tokenStats.lastIn + tokenStats.lastOut;
  // FEATURE-422: the main (text) model and the vision model are separate
  // hover targets, each with its own selector menu.
  if (modelInfo && modelInfo.textModel) {
    sbModelText.innerHTML = "🧠" + modelInfo.textModel + "(" + fmtPct(lastTotal, modelInfo.textMaxLen) + " of " + fmtLen(modelInfo.textMaxLen) + ")";
  } else {
    sbModelText.innerHTML = "🧠";
  }
  if (modelInfo && modelInfo.visionModel) {
    sbModelVision.innerHTML = "👀" + modelInfo.visionModel + "(" + fmtPct(lastTotal, modelInfo.visionMaxLen) + " of " + fmtLen(modelInfo.visionMaxLen) + ")";
  } else {
    sbModelVision.innerHTML = "👀";
  }
  // Session: 会话 15000（↑14500 ↓500）
  sbSession.innerHTML = T.sbSession + " <b>" + fmtNum(total) + "</b>（↑" + fmtNum(sIn) + " ↓" + fmtNum(sOut) + "）";
  // Last turn: 最后一轮 ↑4500（2250t/s, 2s) ↓500 (20t/s, 25s)
  const li = tokenStats.lastIn, lo = tokenStats.lastOut;
  const liTPS = tokenStats.lastInTPS, loTPS = tokenStats.lastOutTPS;
  const liDur = liTPS > 0 ? fmtDur(li / liTPS) : "-";
  const loDur = loTPS > 0 ? fmtDur(lo / loTPS) : "-";
  sbLast.innerHTML = T.sbLast + " ↑" + fmtNum(li) + "（" + (liTPS > 0 ? liTPS + "t/s" : "-") + ", " + liDur + ") ↓" + fmtNum(lo) + " (" + (loTPS > 0 ? loTPS + "t/s" : "-") + ", " + loDur + ")";
}

/* ---------- session menu (FEATURE-387) ---------- */

let sessionList = [];

// renderSessionMenu renders the session list into the status-bar menu and
// updates the 💬 count. Called when the server pushes a "sessions" message.
function renderSessionMenu(sessions) {
  sessionList = sessions || [];
  sbSessions.textContent = "💬 " + sessionList.length;
  sessionMenu.textContent = "";
  if (sessionList.length === 0) {
    const empty = document.createElement("div");
    empty.className = "session-empty";
    empty.textContent = T.planEmpty;
    sessionMenu.appendChild(empty);
    return;
  }
  for (const s of sessionList) {
    const row = document.createElement("div");
    row.className = "session-item" + (s.current ? " current" : "");
    // Left-aligned leading icon: the current session cannot be deleted, so
    // it shows an active indicator instead of a delete button (FEATURE-387).
    const del = document.createElement("span");
    if (s.current) {
      del.className = "session-active";
      del.textContent = "●";
      del.title = T.sessionActive;
    } else {
      del.className = "session-del";
      del.textContent = "✕";
      del.title = T.sessionDelete;
      del.onclick = (e) => {
        e.stopPropagation();
        confirmDeleteSession(s);
      };
    }
    row.appendChild(del);
    // Title (click to switch).
    const title = document.createElement("span");
    title.className = "session-title";
    title.textContent = s.title || "(unnamed)";
    title.title = (s.keywords ? s.keywords + " · " : "") + s.created_at;
    row.appendChild(title);
    row.onclick = () => {
      if (s.current) return;
      wsSend({ type: "session_switch", value: s.id });
    };
    sessionMenu.appendChild(row);
  }
  // FEATURE-425: update the main message area title bar with the current
  // session title (matches the session list).
  const curSess = sessionList.find((s) => s.current);
  if (curSess) {
    streamTitle.value = curSess.title || "";
    streamTitle.placeholder = curSess.title || T.streamTitlePlaceholder;
  }
  // FEATURE-419: when the menu opens, scroll the current session into view so
  // the user immediately sees where they are instead of hunting for it.
  const cur = sessionMenu.querySelector(".session-item.current");
  if (cur) cur.scrollIntoView({ block: "nearest" });
}

/* ---------- main message area display mode (FEATURE-425) ---------- */

// displayMode controls how the main message area renders blocks:
//   "normal"  — as today (all blocks fully expanded)
//   "minimal" — output blocks show only their title + the current live block's
//               dynamic content; a finished block collapses to just its title
//               (except the last finished block)
//   "silent"  — only user-command blocks, error blocks and the final result
//               block are shown
// Persisted in localStorage.
let displayMode = localStorage.getItem("co-shell-display-mode") || "normal";

// initStreamMode wires the three-segment display-mode pill and the editable
// session title in the main message area title bar.
function initStreamMode() {
  // Display-mode pill.
  streamMode.querySelectorAll(".stream-mode-item").forEach((btn) => {
    btn.textContent = T["streamMode" + btn.dataset.mode[0].toUpperCase() + btn.dataset.mode.slice(1)] || btn.dataset.mode;
    btn.classList.toggle("active", btn.dataset.mode === displayMode);
    btn.onclick = () => {
      if (btn.dataset.mode === displayMode) return;
      displayMode = btn.dataset.mode;
      localStorage.setItem("co-shell-display-mode", displayMode);
      streamMode.querySelectorAll(".stream-mode-item").forEach((b) => b.classList.toggle("active", b.dataset.mode === displayMode));
      applyDisplayMode();
    };
  });
  // Editable session title: commit on Enter or blur.
  streamTitle.placeholder = T.streamTitlePlaceholder;
  streamTitle.title = T.streamTitleHint;
  const commitTitle = () => {
    const v = streamTitle.value.trim();
    if (!v) { streamTitle.value = streamTitle.placeholder; return; }
    const cur = sessionList.find((s) => s.current);
    if (cur && v !== cur.title) wsSend({ type: "session_rename", value: v });
  };
  streamTitle.addEventListener("keydown", (e) => {
    if (e.key === "Enter") { e.preventDefault(); streamTitle.blur(); }
  });
  streamTitle.addEventListener("blur", commitTitle);
}

// applyDisplayMode re-applies the current display mode to all existing blocks.
function applyDisplayMode() {
  document.querySelectorAll(".ev").forEach((box) => {
    const cls = box.className.replace("ev ", "").split(" ")[0];
    const body = box.querySelector(".ev-body");
    const isStreaming = body && isStreamingBody(body);
    if (displayMode === "silent") {
      // Show only user-msg, error, and the final result block.
      const show = cls === "user-msg" || box.classList.contains("level-error") || box.classList.contains("ev-result");
      box.style.display = show ? "" : "none";
    } else if (displayMode === "minimal") {
      box.style.display = "";
      // Collapse finished non-result blocks to just their title.
      const isResult = box.classList.contains("ev-result");
      if (!isStreaming && !isResult) box.classList.add("collapsed");
      else box.classList.remove("collapsed");
    } else {
      box.style.display = "";
      box.classList.remove("collapsed");
    }
  });
}

// Request the session list on connect and whenever the menu is opened.
sbSessionsWrap.addEventListener("mouseenter", () => {
  wsSend({ type: "session_list" });
  sessionMenu.classList.remove("hidden");
});
sbSessionsWrap.addEventListener("mouseleave", () => {
  sessionMenu.classList.add("hidden");
});

// confirmDeleteSession opens the delete-confirmation modal for a session.
// The actual session_delete message is only sent after the user confirms
// (FEATURE-387).
let pendingDeleteID = null;
function confirmDeleteSession(s) {
  pendingDeleteID = s.id;
  delSessionMsg.textContent = T.sessionDeleteConfirm.replace("%s", s.title || "(unnamed)");
  delSessionModal.classList.remove("hidden");
}
function closeDeleteModal() {
  pendingDeleteID = null;
  delSessionModal.classList.add("hidden");
}
delSessionClose.onclick = closeDeleteModal;
delSessionCancel.onclick = closeDeleteModal;
delSessionModal.onclick = (e) => { if (e.target === delSessionModal) closeDeleteModal(); };
delSessionConfirm.onclick = () => {
  if (pendingDeleteID) wsSend({ type: "session_delete", value: pendingDeleteID });
  closeDeleteModal();
};

/* ---------- task plan panel ---------- */

function normStatus(s) {
  switch (s) {
    case "[=]": case "in_progress": return "in_progress";
    case "[X]": case "[x]": case "completed": return "completed";
    case "[C]": case "cancelled": return "cancelled";
    case "[F]": case "failed": return "failed";
    default: return "pending";
  }
}

const STATUS_ICON = { pending: "○", in_progress: "◐", completed: "●", cancelled: "✕", failed: "✗" };

function renderPlan(plan) {
  lastPlan = plan && plan.steps && plan.steps.length > 0 ? plan : null;
  applyPanels();
  if (!lastPlan) return;
  planBody.textContent = "";

  const title = document.createElement("div");
  title.className = "plan-title";
  title.textContent = plan.title || "";
  planBody.appendChild(title);

  if (plan.description) {
    const desc = document.createElement("div");
    desc.className = "plan-desc";
    desc.textContent = plan.description;
    planBody.appendChild(desc);
  }

  for (const st of plan.steps) {
    const status = normStatus(st.status);
    const row = document.createElement("div");
    row.className = "plan-step " + status;
    const icon = document.createElement("span");
    icon.className = "st";
    icon.textContent = STATUS_ICON[status] || "○";
    const desc = document.createElement("span");
    desc.className = "desc";
    // Only the first line of a step is the highlighted "title"; any
    // continuation lines render as regular dim content (FEATURE-369).
    const text = st.description || "";
    const nl = text.indexOf("\n");
    const hl = document.createElement("span");
    hl.className = "hl";
    hl.textContent = nl === -1 ? text : text.slice(0, nl);
    desc.appendChild(hl);
    if (nl !== -1) desc.appendChild(document.createTextNode(text.slice(nl)));
    row.appendChild(icon);
    row.appendChild(desc);
    planBody.appendChild(row);
  }
}

/* ---------- ask area ---------- */

let pendingAsk = null;

function showAsk(msg) {
  pendingAsk = msg.id;
  // While the model wizard is active, render the input request inside the
  // wizard modal instead of the bottom ask area (FEATURE-422).
  if (wizardActive) {
    showWizardAsk(msg);
    return;
  }
  askArea.classList.remove("hidden");
  askKeys.textContent = "";
  if (msg.mode === "key") {
    askHint.textContent = T.askKey;
    askLineWrap.classList.add("hidden");
    const keys = [["Enter", ""], ["c", "c"], ["a", "a"], ["g", "g"], ["d", "d"], ["n", "n"]];
    for (const [label, value] of keys) {
      const b = document.createElement("button");
      b.className = "key-btn";
      b.textContent = label;
      b.onclick = () => answerAsk(value);
      askKeys.appendChild(b);
    }
  } else {
    askHint.textContent = T.askLine;
    askLineWrap.classList.remove("hidden");
    askInput.value = "";
    askInput.focus();
  }
  // The ask area expands the bottom bar, shrinking the stream; scroll to the
  // bottom so the confirmation options (ui_text REPL block) stay visible.
  scrollStream();
}

function answerAsk(value) {
  if (!pendingAsk) return;
  wsSend({ type: "answer", id: pendingAsk, value });
  // While the model wizard is active, echo the answer into the wizard modal
  // instead of the main event stream (FEATURE-422).
  if (wizardActive) {
    if (value) appendWizardText("→ " + value);
    hideAsk();
    return;
  }
  // Echo the user's answer (confirmation choice, selected option, or typed
  // content) as a YOU block so it appears in the output stream — these inputs
  // are part of the conversation context and should be visible.
  if (value) renderUserEcho(value);
  hideAsk();
}

function hideAsk() {
  pendingAsk = null;
  pendingInteraction = null;
  supplementMode = false;
  numberMode = false;
  input.placeholder = T.inputHint;
  askArea.classList.add("hidden");
  askInteraction.classList.add("hidden");
  askInteraction.textContent = "";
  // Remove the virtual-keyboard physical-key listener.
  if (window.__vkHandler) {
    window.removeEventListener("keydown", window.__vkHandler);
    window.__vkHandler = null;
  }
}

/* ---------- structured interaction (FEATURE-388) ---------- */

let pendingInteraction = null;
let supplementMode = false; // true while the user is typing supplementary info
let numberMode = false;    // true while the user is choosing an approve-count

// enterSupplementMode switches to supplement-input mode: the user types in the
// main input box and the key handler stops hijacking keys (FEATURE-388).
function enterSupplementMode() {
  supplementMode = true;
  input.focus();
  input.placeholder = T.supplementHint;
}

// enterNumberMode switches to number-choice mode: the user presses a digit to
// choose the approve-count (0 = 10, 1-9 = the count).
function enterNumberMode() {
  numberMode = true;
  input.focus();
  input.placeholder = T.numberHint;
}

// showInteraction renders a structured interaction (confirm/select/input/key)
// from the interaction payload. Buttons are built dynamically from the keys
// array, so the frontend no longer hardcodes confirmation keys.
function showInteraction(msg) {
  pendingInteraction = msg.id;
  const it = msg.interaction || {};
  // While the model wizard is active, route structured interactions to the
  // wizard modal as a simple line input (FEATURE-422).
  if (wizardActive) {
    showWizardAsk({ id: msg.id, mode: "line" });
    return;
  }
  askArea.classList.remove("hidden");
  askKeys.textContent = "";
  askLineWrap.classList.add("hidden");
  askInteraction.classList.remove("hidden");
  askInteraction.textContent = "";

  // Top-bottom layout: prompt info on top, option buttons below (FEATURE-388).
  // Title + body.
  if (it.title) {
    const t = document.createElement("div");
    t.className = "interaction-title md";
    // FEATURE-409: the question (ask_followup_question) lives in title and
    // may carry markdown (lists, emphasis, code) — render it as md too.
    mdRender(t, it.title);
    askInteraction.appendChild(t);
  }
  if (it.body) {
    const b = document.createElement("div");
    b.className = "interaction-body md";
    // FEATURE-409: render the prompt body as markdown so lists, code and
    // emphasis are laid out instead of piling up as one text blob.
    mdRender(b, it.body);
    askInteraction.appendChild(b);
  }

  if (it.kind === "select" && it.options && it.options.length) {
    // FEATURE-399: render each option as a virtual-keyboard-style square key
    // (number 1..N) with the option text beside it. Clicking a key or pressing
    // the physical number key selects that option.
    renderVirtualKeyboard(it, true);
  } else if (it.kind === "confirm") {
    // Option buttons below.
    renderVirtualKeyboard(it, false);
  }

  // Free input for the pure-input kind (ask_followup_question without options).
  // For confirm interactions, supplementary instructions are typed in the main
  // input box (FEATURE-388).
  if (it.kind === "input") {
    const row = document.createElement("div");
    row.className = "interaction-free";
    const inp = document.createElement("input");
    inp.type = "text";
    inp.autocomplete = "off";
    inp.placeholder = T.askLine;
    const send = document.createElement("button");
    send.className = "btn";
    send.textContent = T.send;
    send.onclick = () => answerInteraction({ action: "input", value: inp.value });
    inp.addEventListener("keydown", (e) => {
      if (e.key === "Enter") { e.preventDefault(); answerInteraction({ action: "input", value: inp.value }); }
    });
    row.appendChild(inp);
    row.appendChild(send);
    askInteraction.appendChild(row);
    inp.focus();
  }

  scrollStream();
}

// legendLabel maps an interaction action to a friendly label for the key legend.
function legendLabel(m) {
  switch (m.action) {
    case "approve": return T.approve;
    case "approve_all": return T.approveAll;
    case "approve_g": return T.approveG;
    case "approve_d": return T.approveD;
    case "cancel": return T.cancel;
    case "approve_count": return T.approveCount + " " + m.value;
    case "select": return m.value;
    case "input": return T.askLine;
    default: return m.action;
  }
}

// QWERTY keyboard rows (top number row + three letter rows).
const VK_ROWS = [
  ["1", "2", "3", "4", "5", "6", "7", "8", "9", "0"],
  ["q", "w", "e", "r", "t", "y", "u", "i", "o", "p"],
  ["a", "s", "d", "f", "g", "h", "j", "k", "l", ";"],
  ["z", "x", "c", "v", "b", "n", "m", ",", ".", "/"],
];

// renderVirtualKeyboard renders a QWERTY keyboard, highlighting the keys that
// map to the current interaction's actions. Clicking a highlighted key (or
// pressing the corresponding physical key) responds immediately (FEATURE-388).
// When isSelect is true, number keys map to the select options (1..N).
// container (optional) is where the keyboard is appended; defaults to askInteraction.
function renderVirtualKeyboard(it, isSelect, container) {
  const target = container || askInteraction;
  // Build a map: key -> {action, value}.
  const keyMap = {};
  (it.keys || []).forEach((k) => {
    const key = (k.key || "").toLowerCase();
    if (key) keyMap[key] = { action: k.value };
  });
  if (isSelect && it.options && it.options.length) {
    // Number keys select options (1..N).
    it.options.forEach((opt, i) => {
      keyMap[String(i + 1)] = { action: "select", value: opt };
    });
  } else if (it.presets && it.presets.length) {
    // Number keys map to approve-count (0 = 10, 1-9 = the count). Only
    // enabled when the interaction carries presets (e.g. tool confirmation),
    // so ESC pause/interrupt (no presets) does not show a useless [1]-[9]
    // approve-count hint (FEATURE-396).
    for (let i = 0; i <= 9; i++) {
      const n = i === 0 ? 10 : i;
      keyMap[String(i)] = { action: "approve_count", value: String(n) };
    }
  }
  // Enter maps to approve.
  keyMap["enter"] = { action: "approve" };

  // Option items: each is a virtual-keyboard-style square key (showing only the
  // letter / key name) with its label written beside it (FEATURE-388). Number
  // keys are merged into one [1]-[9] item. A [Space] item enters supplement mode.
  const wrap = document.createElement("div");
  wrap.className = "option-buttons";
  // Helper to build one option item: key button + label beside it.
  const addItem = (keyText, labelText, onClick, extraCls) => {
    const item = document.createElement("div");
    item.className = "opt-item" + (extraCls ? " " + extraCls : "");
    const b = document.createElement("button");
    b.className = "opt-key-btn";
    b.textContent = keyText;
    b.onclick = onClick;
    const label = document.createElement("span");
    label.className = "opt-label";
    // FEATURE-409: option text may carry inline markdown (bold, code) —
    // render it inline so emphasis shows instead of raw ** markers.
    for (const n of mdInline(labelText)) label.appendChild(n);
    item.appendChild(b);
    item.appendChild(label);
    wrap.appendChild(item);
  };
  if (isSelect && it.options && it.options.length) {
    // FEATURE-399: render each option as a number square key (1..N) with the
    // option text beside it. Clicking a key or pressing the physical number
    // key selects that option. No [1]-[9] approve-count or Enter approve items.
    it.options.forEach((opt, i) => {
      addItem(String(i + 1), opt, () => answerInteraction({ action: "select", value: opt }));
    });
  } else {
    // Letter/action keys first (skip number keys and enter, handled separately).
    Object.keys(keyMap).forEach((key) => {
      if (/^[0-9]$/.test(key) || key === "enter") return;
      const m = keyMap[key];
      addItem(key.toUpperCase(), legendLabel(m), () => answerInteraction(m));
    });
    // Number keys merged into one [1]-[9] approve-count item (only when the
    // interaction enables approve-count via presets, FEATURE-396).
    const hasNumbers = Object.keys(keyMap).some((k) => /^[0-9]$/.test(k));
    if (hasNumbers) {
      addItem("1-9", T.approveCount, () => enterNumberMode());
    }
    // Enter item.
    addItem("Enter", T.approve, () => answerInteraction({ action: "approve" }));
  }
  // Space item: enter supplement-input mode.
  addItem("Space", T.supplement, () => enterSupplementMode(), "opt-space");
  target.appendChild(wrap);

  // Listen for physical key presses while this interaction is pending.
  window.__vkHandler = (e) => {
    if (!pendingInteraction) return;
    // In supplement mode, stop hijacking keys so the user can type freely.
    if (supplementMode) return;
    // FEATURE-419: swallow every key while collecting a shortcut, whether or
    // not it triggers an action, so no stray character leaks into the input
    // box (previously only digits/letters/space/enter were swallowed).
    e.preventDefault();
    const key = e.key.toLowerCase();
    if (numberMode) {
      // Number-choice mode: a digit picks the approve-count.
      if (/^[0-9]$/.test(key)) {
        const n = key === "0" ? 10 : parseInt(key, 10);
        answerInteraction({ action: "approve_count", value: String(n) });
      }
      return;
    }
    if (key === " ") {
      enterSupplementMode();
    } else if (key === "enter") {
      answerInteraction({ action: "approve" });
    } else if (keyMap[key]) {
      answerInteraction(keyMap[key]);
    }
  };
  window.addEventListener("keydown", window.__vkHandler);
}

// answerInteraction sends the structured result back to the server.
function answerInteraction(result) {
  if (!pendingInteraction) return;
  wsSend({ type: "interaction_answer", id: pendingInteraction, result });
  if (result.value) renderUserEcho(result.value);
  hideAsk();
}

askSend.onclick = () => answerAsk(askInput.value);
askInput.addEventListener("keydown", (e) => {
  if (e.key === "Enter") { e.preventDefault(); answerAsk(askInput.value); }
});

/* ---------- input row / history ---------- */

const history = [];
let histPos = 0; // sentinel: histPos === history.length means "at the unsent draft"
let histDraft = "";

function sendInput() {
  const text = input.value.trim();
  if (!text) return;
  // When an interaction is pending, the main input box sends supplementary
  // instructions instead of a new message (FEATURE-388).
  if (pendingInteraction) {
    answerInteraction({ action: "input", value: text });
    input.value = "";
    autoGrow();
    return;
  }
  wsSend({ type: "input", text });
  renderUserEcho(text);
  history.push(text);
  histPos = history.length;
  input.value = "";
  autoGrow();
  if (wsReady) setRunning(true);
}

function autoGrow() {
  input.style.height = "auto";
  input.style.height = Math.min(input.scrollHeight + 6, 150) + "px"; // FEATURE-405: +6px increment, 150px cap
}

/* ---------- merged send / interrupt button (FEATURE-369) ---------- */

let running = false;

// setRunning flips the single button between ▶ send (idle) and ⏸
// interrupt (agent turn in progress). Turn boundaries arrive as the web
// session's turn_start / await_input events; sendInput flips to running
// immediately for responsive feedback.
function setRunning(v) {
  running = v;
  sendBtn.textContent = v ? "⏸" : "▶";
  sendBtn.title = v ? T.interrupt : T.send;
  sendBtn.classList.toggle("run", v);
  // FEATURE-416: when the turn ends, merge region A back into B so the stream
  // returns to a single whole output area.
  if (!v && splitActive) mergeStream();
}

sendBtn.onclick = () => { if (running) wsSend({ type: "interrupt" }); else sendInput(); };

// FEATURE-416: when the user scrolls up in region B while the LLM is still
// streaming, split the stream so B becomes static and new output goes to A.
// FEATURE-423: while split, if the user scrolls down to the bottom of region B,
// auto-merge A back into B (same as clicking the merge button) so the stream
// returns to a single whole output area. Only the top region's bottom matters;
// region A's state is ignored for a reliable, consistent effect.
streamB.addEventListener("scroll", () => {
  if (!running) return;
  if (splitActive) {
    const bAtBottom = streamB.scrollTop + streamB.clientHeight >= streamB.scrollHeight - 4;
    if (bAtBottom) mergeStream();
    return;
  }
  const atBottom = streamB.scrollTop + streamB.clientHeight >= streamB.scrollHeight - 4;
  if (!atBottom) splitStream();
});

// FEATURE-419: bind the floating block-boundary navigation icons.
bindBlockNav();

/* ---------- work-mode switcher (FEATURE-410) ---------- */

// currentMode is the active work mode name (e.g. "act").
let currentMode = "act";

// renderModeSeg renders the horizontal segmented control from the mode list
// pushed by the backend (kind=mode). Each mode is one segment; the selected
// segment is highlighted and the highlight slider glides to it.
function renderModeSeg(modes) {
  if (!modes || modes.length === 0) return;
  const cur = modes.find((m) => m.current);
  if (cur) currentMode = cur.name;
  modeSeg.querySelectorAll(".mode-seg-item").forEach((el) => el.remove());
  let idx = 0;
  modes.forEach((m, i) => {
    const seg = document.createElement("button");
    seg.className = "mode-seg-item" + (m.current ? " active" : "");
    seg.textContent = m.name;
    seg.title = m.description || m.name;
    seg.onclick = () => {
      if (m.current) return;
      wsSend({ type: "mode_switch", value: m.name });
    };
    modeSeg.appendChild(seg);
    if (m.current) idx = i;
  });
  moveModeSlider(idx);
}

// moveModeSlider glides the highlight slider to the segment at the given index.
function moveModeSlider(idx) {
  const items = modeSeg.querySelectorAll(".mode-seg-item");
  if (!items.length) return;
  const seg = items[idx];
  if (!seg) return;
  modeSegSlider.style.width = seg.offsetWidth + "px";
  modeSegSlider.style.transform = "translateX(" + seg.offsetLeft + "px)";
}

// showModeResult reports the outcome of a mode switch (kind=mode_result).
function showModeResult(msg) {
  if (msg.ok) {
    // Re-fetch the mode list so the segments and highlight stay in sync.
    wsSend({ type: "mode_get" });
    // The new mode may bind different models; refresh the status-bar model
    // info so it reflects the current mode's actual models (FEATURE-422).
    refreshModelInfo();
  }
}

// recallHistory swaps the textarea content with the history entry at
// histPos (or the saved draft when histPos points past the newest entry)
// and parks the cursor at the end.
function recallHistory() {
  input.value = histPos < history.length ? history[histPos] : histDraft;
  autoGrow();
  input.selectionStart = input.selectionEnd = input.value.length;
}

input.addEventListener("keydown", (e) => {
  // FEATURE-409/419: while an interaction is pending (tool confirm / ask
  // select / cancel), the virtual-keyboard handler on window already responds
  // to the target keys. Swallow EVERY key here so no stray character leaks
  // into the input box (previously only digits/letters/space/enter were
  // swallowed; symbols like !@#$%^&*() leaked through). The window handler
  // runs in the bubble phase, after the textarea's own default insertion, so
  // we preventDefault here and let the event keep bubbling for __vkHandler.
  if (pendingInteraction && !supplementMode) {
    e.preventDefault();
    return;
  }
  if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); sendInput(); return; }
  if (e.key !== "ArrowUp" && e.key !== "ArrowDown") return;
  if (history.length === 0) return; // nothing to navigate; never touch the draft (FIX-367)
  // ↑ recalls only with the cursor on the first line, ↓ on the last line,
  // so vertical cursor moves inside multi-line drafts keep working.
  const onFirstLine = input.value.slice(0, input.selectionStart).indexOf("\n") === -1;
  const onLastLine = input.value.slice(input.selectionEnd).indexOf("\n") === -1;
  if (e.key === "ArrowUp" && onFirstLine && histPos > 0) {
    if (histPos === history.length) histDraft = input.value;
    histPos--;
    recallHistory();
    e.preventDefault();
  } else if (e.key === "ArrowDown" && onLastLine && histPos < history.length) {
    histPos++;
    recallHistory();
    e.preventDefault();
  }
});

// When the user types (keyboard, not mouse) while focus is not on an input
// element, bring focus back to the main input box so keystrokes land there.
document.addEventListener("keydown", (e) => {
  // FEATURE-397: ESC triggers pause (interrupt) while a turn is running,
  // equivalent to clicking the ⏸ button. Skip when an interaction is pending
  // (the virtual keyboard handles ESC there) to avoid conflicts.
  if (e.key === "Escape" && running && !pendingInteraction) {
    e.preventDefault();
    wsSend({ type: "interrupt" });
    return;
  }
  const t = e.target;
  const typing = t && (t.tagName === "INPUT" || t.tagName === "TEXTAREA" || t.isContentEditable);
  if (typing) return; // already typing somewhere, don't steal focus
  // Keep modifier/function keys working (shortcuts, arrows, F-keys, etc.).
  if (e.ctrlKey || e.metaKey || e.altKey) return;
  if (e.key.length > 1) return; // e.g. "F5", "ArrowUp", "Escape", "Shift"
  input.focus();
});
input.addEventListener("input", autoGrow);

/* ---------- workspace tree ---------- */

// Paths of directories the user has expanded. Kept across loadTree() calls
// so a refresh (manual, after upload, or on done) preserves each folder's
// open/collapsed state instead of collapsing everything.
const expandedDirs = new Set();

async function loadTree() {
  try {
    const resp = await fetch("/api/tree");
    const root = await resp.json();
    tree.textContent = "";
    const ul = document.createElement("ul");
    // First level shows the workspace root folder itself, so the most
    // salient feature (the workspace name) is visible at a glance.
    ul.appendChild(treeNode(root));
    tree.appendChild(ul);
  } catch { /* keep old tree */ }
}

function treeNode(node) {
  const li = document.createElement("li");
  li.className = "tree-node";
  const row = document.createElement("div");
  row.className = "tree-row" + (node.dir ? " dir" : "");

  // FEATURE-380: git status letter. It is a flex child of the .tree-row so it
  // sits exactly on the file name's horizontal line (align-items:center), and
  // hugs the file list's left edge (FEATURE-425). Files show their status
  // letter (M/A/D/R/U); directories leave it blank so every row keeps the
  // same left gutter and horizontal alignment is unaffected.
  const status = document.createElement("span");
  status.className = "git-status";
  if (node.status) {
    status.textContent = node.status;
    status.title = node.status;
    status.classList.add("st-" + node.status.toLowerCase());
  }
  row.appendChild(status);

  const tw = document.createElement("span");
  tw.className = "tw";
  tw.textContent = node.dir ? "▸" : "";
  row.appendChild(tw);

  const name = document.createElement("span");
  name.className = "name";
  name.textContent = node.name;
  row.appendChild(name);

  // Right-aligned action group (FEATURE-383): directory change-count badge +
  // reveal button. The group is pushed right with margin-left:auto so the
  // badge hugs the reveal button.
  const actions = document.createElement("span");
  actions.className = "row-actions";
  if (node.dir && node.changes > 0) {
    const badge = document.createElement("span");
    badge.className = "git-badge dir-count";
    badge.textContent = "●" + node.changes;
    badge.title = node.changes + " changed";
    actions.appendChild(badge);
  }
  const reveal = document.createElement("button");
  reveal.className = "reveal-btn";
  reveal.title = T.revealDir;
  reveal.textContent = "⌖";
  reveal.onclick = (e) => { e.stopPropagation(); postPath("/api/reveal", node.path); };
  actions.appendChild(reveal);
  row.appendChild(actions);

  // FEATURE-383: hovering a long (truncated) file name auto-expands the
  // sidebar to fit it; leaving the sidebar returns it to the fixed width.
  row.addEventListener("mouseenter", () => {
    if (name.scrollWidth > name.clientWidth) layout.classList.add("sidebar-auto");
  });

  li.appendChild(row);

  if (node.dir) {
    const ul = document.createElement("ul");
    const open = expandedDirs.has(node.path);
    ul.style.display = open ? "" : "none";
    tw.textContent = open ? "▾" : "▸";
    for (const c of node.children || []) ul.appendChild(treeNode(c));
    li.appendChild(ul);
    row.onclick = () => {
      const isOpen = ul.style.display !== "none";
      ul.style.display = isOpen ? "none" : "";
      tw.textContent = isOpen ? "▸" : "▾";
      if (isOpen) expandedDirs.delete(node.path);
      else expandedDirs.add(node.path);
    };
    // Drop files onto a directory row to upload into it.
    row.ondragover = (e) => { e.preventDefault(); row.classList.add("drop-target"); };
    row.ondragleave = () => row.classList.remove("drop-target");
    row.ondrop = (e) => {
      e.preventDefault();
      e.stopPropagation();
      clearDrag();
      row.classList.remove("drop-target");
      uploadFiles(e.dataTransfer.files, node.path);
    };
  } else {
    // FEATURE-425: single-click previews a text file in the in-page viewer;
    // double-click still opens it with the system handler (openFile). The two
    // are distinguished so previewing never accidentally launches the OS app.
    row.dataset.path = node.path; // for highlighting the selected file
    row.onclick = () => openFilePreview(node);
    row.ondblclick = () => openFile(node);
  }
  return li;
}

const IMAGE_EXT = /\.(png|jpe?g|gif|webp|bmp|svg)$/i;

// FEATURE-425: text file extensions that can be previewed in-page (source
// code, docs, config, data, shell scripts, etc.).
const TEXT_EXT = /\.(go|txt|md|markdown|csv|tsv|sh|bash|zsh|conf|cfg|ini|json|ya?ml|xml|py|js|mjs|cjs|ts|jsx|tsx|html?|css|scss|less|sql|java|c|h|cpp|hpp|rs|rb|php|vue|svelte|toml|env|gitignore|dockerfile|makefile|log|properties|gradle|lock|sum|mod)$/i;

function openFile(node) {
  if (IMAGE_EXT.test(node.name)) {
    previewImg.src = "/api/file?path=" + encodeURIComponent(node.path);
    preview.classList.remove("hidden");
  } else {
    postPath("/api/open", node.path);
  }
}

/* ---------- text file previewer (FEATURE-425) ---------- */

// Current preview state: the open file path (workspace-relative) and the next
// line to load. Re-clicking the same file is a no-op (UC-004).
let fvPath = null;
let fvNextLine = 1;
let fvTotal = 0;
let fvLoading = false;
let fvDiff = new Map(); // lineNo -> "add" | "del"
let fvRawMode = false; // md Raw toggle (off = auto-render md)
let fvMdText = ""; // accumulated md content for auto-render
let fvHexMode = false; // binary file shown as hex dump
let fvHexNext = 0; // next byte offset to load in hex mode
let fvHexWidth = 16; // bytes per hex row (8/16/32/64/128), auto-fit to width
// FEATURE-425: persist the user's Raw choice across files (localStorage).
let fvRawPref = localStorage.getItem("co-shell-fv-raw") === "1";

// openFilePreview opens a file in the in-page viewer. Clicking a new file
// immediately discards the current one (UC-003); clicking the current file is
// a no-op (UC-004). Known text extensions preview directly; unknown extensions
// are also attempted as text and fall back to HEX view if control characters
// are found. Image files keep the system open.
function openFilePreview(node) {
  if (IMAGE_EXT.test(node.name)) return;
  // FEATURE-425: clicking the already-open file closes the preview.
  if (fvPath === node.path) {
    closeFileViewer();
    return;
  }
  fvPath = node.path;
  fvNextLine = 1;
  fvTotal = 0;
  fvDiff = new Map();
  fvHexMode = false;
  fvTitle.textContent = node.path;
  fvBody.textContent = "";
  fvSearch.value = "";
  fvSearch.placeholder = T.fileViewerSearch;
  // FEATURE-425: md files get a Raw pill (default off = auto-render md).
  const isMd = /\.(md|markdown)$/i.test(node.name);
  fvRaw.classList.toggle("hidden", !isMd);
  fvRawMode = fvRawPref; // persist the user's Raw choice across files
  fvRaw.classList.toggle("on", fvRawMode);
  fvMdText = "";
  fvBody.classList.remove("md");
  fileViewer.classList.remove("hidden");
  // FEATURE-425: highlight the currently selected file in the workspace tree.
  highlightTreeFile(node.path);
  loadFileDiff(node.path);
  // md files in auto-render mode still load on demand (200-line chunks); each
  // chunk is accumulated and the whole accumulated text re-rendered, so the
  // current viewport stays complete while large files are never fully loaded.
  loadFileChunk(node.path, 1, 200);
}

// highlightTreeFile marks the workspace tree row for the given path as the
// currently previewed file (and clears any previous selection).
function highlightTreeFile(path) {
  tree.querySelectorAll(".tree-row.fv-selected").forEach((r) => r.classList.remove("fv-selected"));
  tree.querySelectorAll(".tree-row").forEach((r) => {
    const nameEl = r.querySelector(".name");
    if (nameEl && r.dataset.path === path) r.classList.add("fv-selected");
  });
}

// loadFileDiff fetches the git working-tree diff for the file and stores a
// lineNo -> status map for diff highlighting (UC-008). Only "add" lines are
// kept: the previewer shows the current working-tree content, so deleted lines
// (old content) have no corresponding row to highlight — a modified line is
// shown as its new (added) content and highlighted green.
async function loadFileDiff(path) {
  try {
    const resp = await fetch("/api/gitdiff?path=" + encodeURIComponent(path));
    const body = await resp.json();
    const map = new Map();
    for (const it of (body.lines || [])) {
      if (it.status === "add") map.set(it.line, "add");
    }
    fvDiff = map;
    // Re-render already-loaded lines so diff colours appear.
    fvBody.querySelectorAll(".fv-line").forEach((row) => {
      const no = parseInt(row.dataset.no, 10);
      row.classList.toggle("fv-add", map.has(no));
      row.classList.toggle("fv-del", false);
    });
  } catch { /* no diff available */ }
}

// loadFileChunk fetches a line range [start, end] and appends it to the body.
// It is called on open and on scroll near the bottom (on-demand loading for
// large files, UC-006). For md files in auto-render mode (Raw off) the whole
// file is rendered as markdown instead of per-line rows.
async function loadFileChunk(path, start, end) {
  if (fvLoading || fvPath !== path) return;
  fvLoading = true;
  try {
    const resp = await fetch("/api/file?path=" + encodeURIComponent(path) + "&start=" + start + "&end=" + end);
    if (!resp.ok) { console.error(T.fileViewerLoadFailed); return; }
    const body = await resp.json();
    if (fvPath !== path) return; // switched away while loading
    fvTotal = body.total || 0;
    const lines = body.lines || [];
    const isMdAuto = isMdFile(path) && !fvRawMode;
    if (isMdAuto) {
      // Accumulate the chunk and re-render the whole accumulated text. md.js
      // re-parses the full text each call, so unterminated constructs (open
      // code fence, dangling **) render as their incomplete form and fix
      // themselves once the closing token arrives — the current viewport stays
      // complete while large files are never fully loaded into memory.
      fvMdText += lines.join("\n") + (start + lines.length <= fvTotal ? "\n" : "");
      fvNextLine = start + lines.length;
      renderFileBody();
      return;
    }
    // FEATURE-425: unknown-extension files are tried as text first; if the
    // loaded chunk contains control characters it is a binary file, so switch
    // to hex view (on-demand byte loading). Defer via setTimeout so the
    // current loadFileChunk's finally (fvLoading=false) runs first.
    if (!fvHexMode && hasControlChars(lines.join("\n"))) {
      fvHexMode = true;
      fvBody.textContent = "";
      fvBody.classList.remove("md");
      fvHexNext = 0;
      fvHexWidth = fitHexWidth();
      setTimeout(() => loadFileHex(path, 0, 4096), 0);
      return;
    }
    for (let i = 0; i < lines.length; i++) {
      const no = start + i;
      fvBody.appendChild(renderFileLine(no, lines[i]));
    }
    fvNextLine = start + lines.length;
  } catch (err) { console.error(T.fileViewerLoadFailed, err); }
  finally { fvLoading = false; }
}

// isMdFile reports whether the current path is a markdown file.
function isMdFile(path) {
  return /\.(md|markdown)$/i.test(path || "");
}

// renderFileBody renders the accumulated md text as markdown (Raw off).
function renderFileBody() {
  fvBody.textContent = "";
  fvBody.classList.add("md");
  mdRender(fvBody, fvMdText);
}

// hasControlChars reports whether the text contains binary control characters
// (NUL, BEL, etc.) that indicate a non-text file.
function hasControlChars(text) {
  for (let i = 0; i < text.length; i++) {
    const c = text.charCodeAt(i);
    if (c < 0x09 || (c > 0x0d && c < 0x20)) return true;
  }
  return false;
}

// fitHexWidth picks the largest of 8/16/32/64/128 bytes-per-row whose full
// row (offset gutter + hex bytes + ascii column) fits the current viewer width
// (FEATURE-425). Monospace char width ~0.6em; font-size 12.5px => ~7.5px/char.
function fitHexWidth() {
  const cw = 7.5; // px per monospace char
  const offsetW = 8 * cw; // 8-char offset gutter
  const asciiW = 1.5 * 12 + 8 * cw; // ascii margin + 8-char ascii column
  const avail = fvBody.clientWidth - offsetW - asciiW - 20; // padding
  const widths = [8, 16, 32, 64, 128];
  let best = 8;
  for (const w of widths) {
    // hex = w*3 chars ("xx "), ascii = w chars
    if (w * 3 * cw + w * cw <= avail) best = w;
  }
  return best;
}

// loadFileHex fetches a byte range [start, end] as hex rows and appends them
// to the body (on-demand loading for binary files).
async function loadFileHex(path, start, end) {
  if (fvLoading || fvPath !== path) return;
  fvLoading = true;
  try {
    const resp = await fetch("/api/file?path=" + encodeURIComponent(path) + "&hex=1&start=" + start + "&end=" + end + "&width=" + fvHexWidth);
    if (!resp.ok) { console.error(T.fileViewerLoadFailed); return; }
    const body = await resp.json();
    if (fvPath !== path) return; // switched away while loading
    fvTotal = body.total || 0;
    for (const row of (body.rows || [])) fvBody.appendChild(renderHexRow(row));
    fvHexNext = end;
  } catch (err) { console.error(T.fileViewerLoadFailed, err); }
  finally { fvLoading = false; }
}

// renderHexRow builds one hex-dump row: offset | hex bytes | ascii.
function renderHexRow(row) {
  const el = document.createElement("div");
  el.className = "fv-line fv-hex";
  const off = document.createElement("span");
  off.className = "fv-no";
  off.textContent = row.offset.toString(16).padStart(8, "0");
  const hex = document.createElement("span");
  hex.className = "fv-code fv-hex-bytes";
  hex.textContent = row.hex;
  const ascii = document.createElement("span");
  ascii.className = "fv-code fv-hex-ascii";
  ascii.textContent = row.ascii;
  el.appendChild(off);
  el.appendChild(hex);
  el.appendChild(ascii);
  return el;
}

// renderFileLine builds one line row: a line-number gutter + highlighted code.
// For md files in auto-render mode (Raw off), the whole body is re-rendered as
// markdown instead of per-line rows (handled by renderFileBody).
function renderFileLine(no, text) {
  const row = document.createElement("div");
  row.className = "fv-line";
  row.dataset.no = no;
  const noEl = document.createElement("span");
  noEl.className = "fv-no";
  noEl.textContent = no;
  const code = document.createElement("span");
  code.className = "fv-code";
  code.appendChild(highlightCode(text));
  row.appendChild(noEl);
  row.appendChild(code);
  if (fvDiff.has(no)) row.classList.add("fv-add");
  return row;
}

// highlightCode tokenizes a line and returns a DocumentFragment with syntax
// highlighting spans. It picks a rule set by the current file extension: JSON
// gets property/brace highlighting, everything else uses the generic code
// rules (Go keywords/types/strings/numbers/comments). Comments are a single
// unified green across languages. Content is set via textContent (never
// innerHTML) to avoid XSS.
const GO_KEYWORDS = new Set(["break","case","chan","const","continue","default","defer","else","fallthrough","for","func","go","goto","if","import","interface","map","package","range","return","select","struct","switch","type","var"]);
const GO_TYPES = new Set(["bool","byte","complex64","complex128","error","float32","float64","int","int8","int16","int32","int64","rune","string","uint","uint8","uint16","uint32","uint64","uintptr","any","comparable"]);
const GO_TOKEN_RE = /(\/\/.*$)|(\/\*[\s\S]*?\*\/)|("(?:\\.|[^"\\])*"|`(?:[^`]|\\.)*`|'(?:\\.|[^'\\])*')|(\b\d+(?:\.\d+)?\b)|([A-Za-z_]\w*)|(\s+)|(.)/g;
// JSON: property key ("key":), string value, number, true/false/null, braces.
const JSON_TOKEN_RE = /("[^"]*"\s*:)|("(?:\\.|[^"\\])*")|(\b\d+(?:\.\d+)?\b)|(\b(?:true|false|null)\b)|([{}\[\]])|(\s+)|(.)/g;

function highlightCode(line) {
  if (fvPath && /\.json$/i.test(fvPath)) return highlightJson(line);
  return highlightGeneric(line);
}

function highlightGeneric(line) {
  const frag = document.createDocumentFragment();
  let m;
  GO_TOKEN_RE.lastIndex = 0;
  while ((m = GO_TOKEN_RE.exec(line)) !== null) {
    if (m[1] !== undefined) { // line comment
      frag.appendChild(span("tok-com", m[1]));
    } else if (m[2] !== undefined) { // block comment
      frag.appendChild(span("tok-com", m[2]));
    } else if (m[3] !== undefined) { // string
      frag.appendChild(span("tok-str", m[3]));
    } else if (m[4] !== undefined) { // number
      frag.appendChild(span("tok-num", m[4]));
    } else if (m[5] !== undefined) { // identifier
      const w = m[5];
      if (GO_KEYWORDS.has(w)) frag.appendChild(span("tok-kw", w));
      else if (GO_TYPES.has(w)) frag.appendChild(span("tok-type", w));
      else frag.appendChild(span("tok-var", w));
    } else if (m[6] !== undefined) { // whitespace
      frag.appendChild(document.createTextNode(m[6]));
    } else if (m[7] !== undefined) { // other char
      frag.appendChild(document.createTextNode(m[7]));
    }
  }
  return frag;
}

function highlightJson(line) {
  const frag = document.createDocumentFragment();
  let m;
  JSON_TOKEN_RE.lastIndex = 0;
  while ((m = JSON_TOKEN_RE.exec(line)) !== null) {
    if (m[1] !== undefined) { // property key
      frag.appendChild(span("tok-prop", m[1]));
    } else if (m[2] !== undefined) { // string value
      frag.appendChild(span("tok-str", m[2]));
    } else if (m[3] !== undefined) { // number
      frag.appendChild(span("tok-num", m[3]));
    } else if (m[4] !== undefined) { // true/false/null
      frag.appendChild(span("tok-kw", m[4]));
    } else if (m[5] !== undefined) { // braces
      frag.appendChild(span("tok-brace", m[5]));
    } else if (m[6] !== undefined) { // whitespace
      frag.appendChild(document.createTextNode(m[6]));
    } else if (m[7] !== undefined) { // other char
      frag.appendChild(document.createTextNode(m[7]));
    }
  }
  return frag;
}

function span(cls, text) {
  const s = document.createElement("span");
  s.className = cls;
  s.textContent = text;
  return s;
}

// closeFileViewer hides the previewer and clears its state (FEATURE-425).
function closeFileViewer() {
  fileViewer.classList.add("hidden");
  fvPath = null;
  tree.querySelectorAll(".tree-row.fv-selected").forEach((r) => r.classList.remove("fv-selected"));
}

// Close the viewer.
fvClose.onclick = closeFileViewer;

// FEATURE-425: Raw pill toggles md auto-render (off) vs raw text (on). The
// choice is persisted so the next file keeps the same state.
fvRaw.onclick = () => {
  if (!fvPath || !isMdFile(fvPath)) return;
  fvRawMode = !fvRawMode;
  fvRawPref = fvRawMode;
  localStorage.setItem("co-shell-fv-raw", fvRawMode ? "1" : "0");
  fvRaw.classList.toggle("on", fvRawMode);
  // Re-render: raw mode shows per-line rows, auto mode renders markdown.
  fvBody.textContent = "";
  fvBody.classList.remove("md");
  fvNextLine = 1;
  fvMdText = "";
  loadFileChunk(fvPath, 1, 200);
};

// FEATURE-425: in-file search. Highlights matching lines among the loaded
// rows (searching the already-loaded portion of large files).
fvSearch.addEventListener("input", () => {
  const q = fvSearch.value.trim().toLowerCase();
  fvBody.querySelectorAll(".fv-line").forEach((row) => {
    const hit = q !== "" && row.textContent.toLowerCase().includes(q);
    row.classList.toggle("fv-hit", hit);
  });
});

// FEATURE-425: when the window resizes while a hex dump is open, re-fit the
// bytes-per-row and reload from the start.
window.addEventListener("resize", () => {
  if (!fvHexMode || fvPath === null) return;
  const w = fitHexWidth();
  if (w === fvHexWidth) return;
  fvHexWidth = w;
  fvBody.textContent = "";
  fvHexNext = 0;
  loadFileHex(fvPath, 0, 4096);
});

// On-demand loading: when the user scrolls near the bottom and more content
// remains, fetch the next chunk (UC-006). Hex mode loads byte ranges; text
// mode loads line ranges.
fvBody.addEventListener("scroll", () => {
  if (fvPath === null || fvLoading) return;
  const nearBottom = fvBody.scrollTop + fvBody.clientHeight >= fvBody.scrollHeight - 80;
  if (fvHexMode) {
    if (fvTotal > 0 && fvHexNext >= fvTotal) return; // all loaded
    if (nearBottom) loadFileHex(fvPath, fvHexNext, fvHexNext + 4096);
    return;
  }
  if (fvTotal > 0 && fvNextLine > fvTotal) return; // all loaded
  if (nearBottom) loadFileChunk(fvPath, fvNextLine, fvNextLine + 200);
});

async function postPath(api, path) {
  try {
    const resp = await fetch(api, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ path }),
    });
    if (!resp.ok) {
      const body = await resp.json().catch(() => ({}));
      console.error(T.actionFailed, body.error || resp.status);
    }
  } catch (err) { console.error(T.actionFailed, err); }
}

previewClose.onclick = () => preview.classList.add("hidden");
preview.onclick = (e) => { if (e.target === preview) preview.classList.add("hidden"); };

/* ---------- upload ---------- */

async function uploadFiles(files, dir) {
  if (!files || files.length === 0) return [];
  const fd = new FormData();
  for (const f of files) fd.append("file", f);
  try {
    const resp = await fetch("/api/upload?dir=" + encodeURIComponent(dir || ""), { method: "POST", body: fd });
    const body = await resp.json();
    if (!resp.ok) { console.error(T.uploadFailed, body.error || resp.status); return []; }
    loadTree();
    return body.paths || [];
  } catch (err) { console.error(T.uploadFailed, err); return []; }
}

// The whole sidebar is a drop target: dropping on a directory row uploads
// into that directory (handled above); dropping anywhere else on the panel
// (blank space or file rows) uploads into the workspace root.
function clearDrag() {
  sidebar.classList.remove("drag");
  tree.querySelectorAll(".drop-target").forEach((el) => el.classList.remove("drop-target"));
}

sidebar.ondragover = (e) => { e.preventDefault(); sidebar.classList.add("drag"); };
sidebar.ondragleave = (e) => {
  if (!sidebar.contains(e.relatedTarget)) clearDrag();
};
sidebar.ondrop = (e) => {
  e.preventDefault();
  clearDrag();
  uploadFiles(e.dataTransfer.files, "");
};

document.getElementById("treeRefresh").onclick = loadTree;

/* ---------- logo mosaic ---------- */

// Pixel mosaic of the co-shell mascot (a little clam: domed upper shell,
// a fringed middle seam, and the lower bowl), hand-drawn on a 12 rows x
// 12 cols grid (logo.txt, FEATURE-371). "#" cells render in accent color;
// spaces stay transparent. The same grid is rasterized into favicon.png
// (2x2 px per cell = 24x24 icon).
const LOGO_ART = [
  "",
  "",
  "",
  "   #####",
  "  ########",
  " ##########",
  "## ## ## ###",
  "   ## ## #",
  " ##########",
  "  ########",
  "",
  "",
];
const LOGO_OPACITY = { "=": 0.35, "+": 0.55, "*": 0.75, "#": 0.9, "%": 1 };

(function renderLogo() {
  const logo = document.getElementById("logo");
  const rows = LOGO_ART.length;
  const cols = Math.max(...LOGO_ART.map((l) => l.length));
  // Square cells (uniform scaling keeps the logo's aspect ratio), sized up
  // to 3px. The logo lives in the bottom bar at the left of the input box
  // (FEATURE-365), so its height must stay within the input row.
  const maxW = 120;
  const maxH = (input.offsetHeight || 38) - 4;
  const cell = Math.min(3, maxW / cols, maxH / rows);
  logo.style.gridTemplateColumns = "repeat(" + cols + ", " + cell + "px)";
  logo.style.gridAutoRows = cell + "px";
  const frag = document.createDocumentFragment();
  for (const line of LOGO_ART) {
    for (let i = 0; i < cols; i++) {
      const d = document.createElement("div");
      const ch = line[i] || " ";
      if (ch === " ") d.className = "sp";
      else d.style.opacity = LOGO_OPACITY[ch] || 0.5;
      frag.appendChild(d);
    }
  }
  logo.appendChild(frag);
})();

/* ---------- settings modal ---------- */

miSettings.onclick = () => {
  settingsModal.classList.remove("hidden");
  wsSend({ type: "settings_get" });
};
// FEATURE-398: "重启后台" sends a restart signal to the backend, which
// notifies the external supervisor to restart the process.
miRestart.onclick = () => {
  wsSend({ type: "restart" });
};
settingsClose.onclick = () => settingsModal.classList.add("hidden");
settingsModal.onclick = (e) => { if (e.target === settingsModal) settingsModal.classList.add("hidden"); };
setThemeMode.onchange = () => {
  localStorage.setItem("co-shell-theme", setThemeMode.value);
  applyTheme();
};

// renderSettings renders the grouped setting items returned by settings_get
// (FEATURE-391). Each item is rendered as a form control based on its type:
// bool -> toggle, number -> number input, enum -> select, string -> text input.
function renderSettings(groups) {
  settingsDynamic.innerHTML = "";
  if (!groups || !groups.length) {
    settingsDynamic.textContent = "(no settings)";
    return;
  }
  const frag = document.createDocumentFragment();
  for (const g of groups) {
    const h = document.createElement("div");
    h.className = "set-group-title";
    h.textContent = g.title || "";
    frag.appendChild(h);
    for (const it of g.items || []) {
      frag.appendChild(renderSettingItem(it));
    }
  }
  settingsDynamic.appendChild(frag);
}

// renderSettingItem builds one setting row with its label and form control.
function renderSettingItem(it) {
  const row = document.createElement("label");
  row.className = "set-row";
  const label = document.createElement("span");
  label.className = "set-label";
  label.textContent = it.key;
  label.title = it.desc || "";
  row.appendChild(label);

  let ctl;
  if (it.type === "bool") {
    ctl = document.createElement("input");
    ctl.type = "checkbox";
    ctl.className = "set-toggle";
    ctl.checked = it.value === "on";
    ctl.onchange = () => wsSend({ type: "settings_set", key: it.key, value: ctl.checked ? "on" : "off" });
  } else if (it.type === "number") {
    ctl = document.createElement("input");
    ctl.type = "number";
    ctl.className = "set-input";
    ctl.value = it.value;
    ctl.onchange = () => wsSend({ type: "settings_set", key: it.key, value: ctl.value });
  } else if (it.type === "enum") {
    ctl = document.createElement("select");
    ctl.className = "set-select";
    for (const opt of it.options || []) {
      const o = document.createElement("option");
      o.value = opt;
      o.textContent = opt;
      if (opt === it.value) o.selected = true;
      ctl.appendChild(o);
    }
    ctl.onchange = () => wsSend({ type: "settings_set", key: it.key, value: ctl.value });
  } else {
    ctl = document.createElement("input");
    ctl.type = "text";
    ctl.className = "set-input";
    ctl.value = it.value;
    ctl.onchange = () => wsSend({ type: "settings_set", key: it.key, value: ctl.value });
  }
  row.appendChild(ctl);
  return row;
}

// showSettingsResult displays the result of a settings_set change.
function showSettingsResult(msg) {
  const el = document.createElement("div");
  el.className = "set-result " + (msg.ok ? "ok" : "err");
  el.textContent = msg.ok ? (msg.message || "ok") : (msg.message || "error");
  settingsDynamic.prepend(el);
  setTimeout(() => el.remove(), 3000);
}

/* ---------- model manager (FEATURE-422) ---------- */

// modelList holds the latest model/template lists from the backend.
let modelList = [];
let templateList = [];
// wizardActive is true while the model add/edit wizard is running; while it is
// set, ui_text / ask / interaction messages are routed to the wizard modal
// instead of the main event stream.
let wizardActive = false;

// Open the model manager modal and fetch the model list.
miModels.onclick = () => {
  modelsModal.classList.remove("hidden");
  wsSend({ type: "model_get" });
};
modelsClose.onclick = () => modelsModal.classList.add("hidden");
modelsModal.onclick = (e) => { if (e.target === modelsModal) modelsModal.classList.add("hidden"); };

// "＋ 新增模型" opens the add-model wizard.
modelAddBtn.onclick = () => {
  modelsModal.classList.add("hidden");
  openModelWizard("add", "");
};

// renderModels renders the model list into the manager modal and the status-bar
// model selector menu.
function renderModels(models, templates) {
  modelList = models || [];
  templateList = templates || [];
  renderModelsBody();
  renderModelMenu();
  renderModelVisionMenu();
}

// renderModelsBody renders the model list into the manager modal.
function renderModelsBody() {
  modelsBody.textContent = "";
  if (!modelList.length) {
    const empty = document.createElement("div");
    empty.className = "models-empty";
    empty.textContent = T.modelEmpty || "暂无模型，点击上方「＋ 新增模型」添加";
    modelsBody.appendChild(empty);
    return;
  }
  for (const m of modelList) {
    const row = document.createElement("div");
    row.className = "model-row" + (m.enabled ? " enabled" : "");
    // Left: status + identity.
    const info = document.createElement("div");
    info.className = "model-info";
    const id = document.createElement("div");
    id.className = "model-id";
    id.textContent = (m.enabled ? "● " : "○ ") + m.id;
    id.title = m.name || m.id;
    info.appendChild(id);
    const meta = document.createElement("div");
    meta.className = "model-meta";
    const caps = [];
    if (m.vision) caps.push("👁");
    if (m.tool_call) caps.push("🔧");
    if (m.thinking) caps.push("💭");
    meta.textContent = m.provider + " · " + m.model + (caps.length ? " · " + caps.join(" ") : "") + " · P" + m.priority;
    info.appendChild(meta);
    row.appendChild(info);
    // Right: action buttons.
    const actions = document.createElement("div");
    actions.className = "model-actions";
    const mkBtn = (label, title, fn) => {
      const b = document.createElement("button");
      b.className = "model-act";
      b.textContent = label;
      b.title = title;
      b.onclick = fn;
      actions.appendChild(b);
    };
    mkBtn("切换", "切换为当前模型", () => wsSend({ type: "model_switch", value: m.id }));
    mkBtn(m.enabled ? "禁用" : "启用", m.enabled ? "禁用此模型" : "启用此模型", () => wsSend({ type: m.enabled ? "model_disable" : "model_enable", value: m.id }));
    mkBtn("编辑", "编辑此模型", () => { modelsModal.classList.add("hidden"); openModelWizard("edit", m.id); });
    mkBtn("删除", "删除此模型", () => { if (confirm("确认删除模型 " + m.id + "？")) wsSend({ type: "model_remove", value: m.id }); });
    row.appendChild(actions);
    modelsBody.appendChild(row);
  }
}

// modelLogo maps a provider name to its logo file under /static/logos/.
// Logos are 64x64 PNGs displayed at 32x32 (crisp on retina). Unknown providers
// fall back to a generic chip.
const MODEL_LOGOS = {
  deepseek: "deepseek-icon.svg",
  qwen: "qwen.png",
  xiaomi: "xiaomi.svg",
  kimi: "kimi.webp",
  zhipu: "zhipu.svg",
  openai: "openai.svg",
  lmstudio: "lmstudio.png",
  ollama: "ollama.png",
  "openai-compatible": "openai.svg",
};

// fmtLenShort formats a context length in K units (1K = 1024), e.g.
// 1048576 -> "1024K". Values below 1K are shown as-is.
function fmtLenShort(n) {
  if (!(n > 0)) return "";
  if (n >= 1024) {
    const k = n / 1024;
    return (Number.isInteger(k) ? k : k.toFixed(1).replace(/\.0$/, "")) + "K";
  }
  return String(n);
}

// buildModelMenuItem builds one model row in a selector menu: a provider logo
// on the left, the model ID, and the max context length right-aligned. The row
// never wraps; the menu width adapts to its widest row. When activeID matches
// the model ID, the row is highlighted as the currently active model
// (FEATURE-422).
function buildModelMenuItem(m, onClick, activeID) {
  const item = document.createElement("div");
  item.className = "model-menu-item" + (activeID && m.id === activeID ? " active" : "");
  item.title = m.provider + " · " + m.model;
  const logo = document.createElement("img");
  logo.className = "model-logo";
  logo.src = "/static/logos/" + (MODEL_LOGOS[m.provider] || "generic.png");
  logo.alt = "";
  logo.onerror = () => { logo.style.display = "none"; };
  item.appendChild(logo);
  const id = document.createElement("span");
  id.className = "model-id";
  id.textContent = m.id;
  item.appendChild(id);
  const ctx = document.createElement("span");
  ctx.className = "model-ctx";
  ctx.textContent = fmtLenShort(m.max_model_len);
  item.appendChild(ctx);
  item.onclick = onClick;
  return item;
}

// buildModelMenuDefault builds the "默认" row that clears the current mode's
// model binding for the given target ("text" or "vision"). It shows the
// current default model's logo + name (semi-transparent), a "(默认)" hint,
// and the default model's context length. When active is true (the mode has no
// binding for this slot), the row is highlighted (FEATURE-422).
function buildModelMenuDefault(target, menu, active) {
  const item = document.createElement("div");
  item.className = "model-menu-item default" + (active ? " active" : "");
  item.title = T.modelDefaultHint || "使用全局默认模型";
  // The current default model is the one shown in the status bar (modelInfo).
  // modelInfo.textModel/visionModel hold the model NAME (m.model), not the
  // model ID, so match against m.model (FEATURE-422).
  const defaultName = target === "vision" ? (modelInfo && modelInfo.visionModel) : (modelInfo && modelInfo.textModel);
  const def = modelList.find((m) => m.model === defaultName);
  if (def) {
    const logo = document.createElement("img");
    logo.className = "model-logo";
    logo.src = "/static/logos/" + (MODEL_LOGOS[def.provider] || "generic.png");
    logo.alt = "";
    logo.onerror = () => { logo.style.display = "none"; };
    item.appendChild(logo);
    const id = document.createElement("span");
    id.className = "model-id";
    id.textContent = def.id;
    item.appendChild(id);
    const hint = document.createElement("span");
    hint.className = "model-default-hint";
    hint.textContent = "(" + (T.modelRestoreDefault || "恢复默认") + ")";
    item.appendChild(hint);
    const ctx = document.createElement("span");
    ctx.className = "model-ctx";
    ctx.textContent = fmtLenShort(def.max_model_len);
    item.appendChild(ctx);
  } else {
    item.textContent = T.modelDefault || "默认";
  }
  item.onclick = () => { wsSend({ type: "model_unbind", value: target }); menu.classList.add("hidden"); };
  return item;
}

// buildModelMenuTitle builds the model-menu title row: the title text on the
// left and a capsule showing the current work mode on the right (FEATURE-423).
function buildModelMenuTitle(text) {
  const title = document.createElement("div");
  title.className = "model-menu-title";
  const label = document.createElement("span");
  label.className = "model-menu-title-label";
  label.textContent = text;
  title.appendChild(label);
  const mode = document.createElement("span");
  mode.className = "model-menu-mode";
  mode.textContent = currentMode;
  mode.title = "当前模式";
  title.appendChild(mode);
  return title;
}

// renderModelMenu renders the status-bar main (text) model selector menu. It
// lists all enabled models plus a "默认" option and a "＋ 新增模型" entry
// (FEATURE-422).
function renderModelMenu() {
  modelMenu.textContent = "";
  modelMenu.appendChild(buildModelMenuTitle(T.modelMenuTitle || "选择主模型"));
  const enabled = modelList.filter((m) => m.enabled);
  if (!enabled.length) {
    const empty = document.createElement("div");
    empty.className = "model-menu-empty";
    empty.textContent = T.modelEmpty || "暂无模型";
    modelMenu.appendChild(empty);
  }
  // Highlight the model bound to the current mode (empty = no binding, so
  // nothing is highlighted and the "默认" option is highlighted instead).
  const activeTextID = modelInfo && modelInfo.modeTextModelID;
  for (const m of enabled) {
    modelMenu.appendChild(buildModelMenuItem(m, () => { wsSend({ type: "model_bind", key: "text", value: m.id }); modelMenu.classList.add("hidden"); }, activeTextID));
  }
  const sep = document.createElement("div");
  sep.className = "model-menu-sep";
  modelMenu.appendChild(sep);
  modelMenu.appendChild(buildModelMenuDefault("text", modelMenu, !(modelInfo && modelInfo.modeTextModelID)));
  const add = document.createElement("div");
  add.className = "model-menu-item add";
  add.textContent = "＋ 新增模型";
  add.onclick = () => { modelMenu.classList.add("hidden"); openModelWizard("add", ""); };
  modelMenu.appendChild(add);
}

// renderModelVisionMenu renders the status-bar vision model selector menu. It
// lists only enabled models with vision capability (a subset of the main
// models) plus a "默认" option and a "＋ 新增模型" entry (FEATURE-422).
function renderModelVisionMenu() {
  modelVisionMenu.textContent = "";
  modelVisionMenu.appendChild(buildModelMenuTitle(T.modelVisionMenuTitle || "选择视觉模型"));
  const vision = modelList.filter((m) => m.enabled && m.vision);
  if (!vision.length) {
    const empty = document.createElement("div");
    empty.className = "model-menu-empty";
    empty.textContent = T.modelVisionEmpty || "暂无视觉模型";
    modelVisionMenu.appendChild(empty);
  }
  const activeVisionID = modelInfo && modelInfo.modeVisionModelID;
  for (const m of vision) {
    modelVisionMenu.appendChild(buildModelMenuItem(m, () => { wsSend({ type: "model_bind", key: "vision", value: m.id }); modelVisionMenu.classList.add("hidden"); }, activeVisionID));
  }
  const sep = document.createElement("div");
  sep.className = "model-menu-sep";
  modelVisionMenu.appendChild(sep);
  modelVisionMenu.appendChild(buildModelMenuDefault("vision", modelVisionMenu, !(modelInfo && modelInfo.modeVisionModelID)));
  const add = document.createElement("div");
  add.className = "model-menu-item add";
  add.textContent = "＋ 新增模型";
  add.onclick = () => { modelVisionMenu.classList.add("hidden"); openModelWizard("add", ""); };
  modelVisionMenu.appendChild(add);
}

// Status-bar model selectors: hover to expand, leave to hide. The main (text)
// model and the vision model each have their own menu (FEATURE-422).
sbModelTextWrap.addEventListener("mouseenter", () => {
  wsSend({ type: "model_get" });
  modelMenu.classList.remove("hidden");
});
sbModelTextWrap.addEventListener("mouseleave", () => modelMenu.classList.add("hidden"));
sbModelVisionWrap.addEventListener("mouseenter", () => {
  wsSend({ type: "model_get" });
  modelVisionMenu.classList.remove("hidden");
});
sbModelVisionWrap.addEventListener("mouseleave", () => modelVisionMenu.classList.add("hidden"));

// refreshModelInfo re-fetches /api/bootstrap to update the status-bar model
// info (modelInfo) after a model switch/unbind, then refreshes the status bar
// (FEATURE-422).
async function refreshModelInfo() {
  try {
    const resp = await fetch("/api/bootstrap");
    const b = await resp.json();
    if (b.textModel) {
      modelInfo = { textModel: b.textModel, textMaxLen: b.textMaxLen || 0, visionModel: b.visionModel || "", visionMaxLen: b.visionMaxLen || 0, modeTextModelID: b.modeTextModelID || "", modeVisionModelID: b.modeVisionModelID || "" };
    }
    updateStatus();
  } catch { /* keep the current modelInfo on failure */ }
}

// showModelResult reports the result of a model operation / wizard completion.
function showModelResult(msg) {
  if (wizardActive) {
    // The wizard finished (success or cancel): close the wizard modal.
    closeModelWizard();
  }
  if (!msg.ok) {
    const el = document.createElement("div");
    el.className = "set-result err";
    el.textContent = msg.message || "error";
    modelsBody.prepend(el);
    setTimeout(() => el.remove(), 3000);
  }
  // Refresh the model list so the frontend reflects the change.
  wsSend({ type: "model_get" });
  // Refresh the status-bar model info after a switch/unbind (FEATURE-422).
  refreshModelInfo();
}

// openModelWizard opens the wizard modal and launches the add/edit wizard.
function openModelWizard(mode, id) {
  wizardActive = true;
  modelWizardBody.textContent = "";
  modelWizardModal.classList.remove("hidden");
  if (mode === "edit") {
    wsSend({ type: "model_edit", value: id });
  } else {
    wsSend({ type: "model_add" });
  }
}

// closeModelWizard closes the wizard modal and clears the wizard-active flag.
function closeModelWizard() {
  wizardActive = false;
  modelWizardModal.classList.add("hidden");
  modelWizardBody.textContent = "";
}

// The wizard cancel button aborts the running wizard (backend fails all pending
// asks so the wizard exits from any step).
modelWizardCancel.onclick = () => {
  wsSend({ type: "model_wizard_cancel" });
  closeModelWizard();
};

// appendWizardText appends a line of wizard output to the wizard modal body.
function appendWizardText(text) {
  const line = document.createElement("div");
  line.className = "wizard-line";
  line.textContent = text;
  modelWizardBody.appendChild(line);
  modelWizardBody.scrollTop = modelWizardBody.scrollHeight;
}

// showWizardAsk renders an ask input request inside the wizard modal. It builds
// a single-line input (mode "line") or a set of key buttons (mode "key") and
// sends the answer back via the standard answer message.
function showWizardAsk(msg) {
  const wrap = document.createElement("div");
  wrap.className = "wizard-ask";
  if (msg.mode === "key") {
    const keys = [["Enter", ""], ["c", "c"], ["a", "a"], ["g", "g"], ["d", "d"], ["n", "n"]];
    for (const [label, value] of keys) {
      const b = document.createElement("button");
      b.className = "key-btn";
      b.textContent = label;
      b.onclick = () => { wsSend({ type: "answer", id: msg.id, value }); wrap.remove(); };
      wrap.appendChild(b);
    }
  } else {
    const inp = document.createElement("input");
    inp.type = "text";
    inp.autocomplete = "off";
    inp.placeholder = T.askLine || "输入...";
    const send = document.createElement("button");
    send.className = "btn";
    send.textContent = T.send || "发送";
    const submit = () => { wsSend({ type: "answer", id: msg.id, value: inp.value }); wrap.remove(); };
    send.onclick = submit;
    inp.addEventListener("keydown", (e) => { if (e.key === "Enter") { e.preventDefault(); submit(); } });
    wrap.appendChild(inp);
    wrap.appendChild(send);
    setTimeout(() => inp.focus(), 0);
  }
  modelWizardBody.appendChild(wrap);
  modelWizardBody.scrollTop = modelWizardBody.scrollHeight;
}

/* ---------- identity & personality (FEATURE-393) ---------- */

// Logo hover menu: show the menu when hovering the logo, hide on leave.
logoWrap.addEventListener("mouseenter", () => logoMenu.classList.remove("hidden"));
logoWrap.addEventListener("mouseleave", () => logoMenu.classList.add("hidden"));

// FEATURE-401: the "+" button creates a new session.
newSessionBtn.onclick = () => {
  wsSend({ type: "session_new" });
};

// Open the identity form when clicking the [身份与个性] menu item.
miIdentity.onclick = () => {
  logoMenu.classList.add("hidden");
  identityModal.classList.remove("hidden");
  wsSend({ type: "identity_get" });
};
identityClose.onclick = () => identityModal.classList.add("hidden");
identityModal.onclick = (e) => { if (e.target === identityModal) identityModal.classList.add("hidden"); };

// renderIdentity renders the identity & personality form. name is a single-line
// input; the rest are multi-line textareas. Each field has its own save button.
// Field labels come from the backend (localized), falling back to the key.
function renderIdentity(fields) {
  identityBody.innerHTML = "";
  if (!fields || !fields.length) {
    identityBody.textContent = "(empty)";
    return;
  }
  for (const f of fields) {
    const row = document.createElement("div");
    row.className = "identity-row";

    const label = document.createElement("label");
    label.className = "identity-label";
    label.textContent = f.label || f.key;
    row.appendChild(label);

    let ctl;
    if (f.type === "text") {
      ctl = document.createElement("input");
      ctl.type = "text";
      ctl.className = "identity-input";
      ctl.value = f.value || "";
    } else {
      ctl = document.createElement("textarea");
      ctl.className = "identity-textarea";
      ctl.rows = 4;
      ctl.value = f.value || "";
    }
    row.appendChild(ctl);

    const saveBtn = document.createElement("button");
    saveBtn.className = "btn identity-save";
    saveBtn.textContent = "保存";
    saveBtn.onclick = () => {
      wsSend({ type: "identity_set", key: f.key, value: ctl.value });
    };
    row.appendChild(saveBtn);

    identityBody.appendChild(row);
  }
}

// showIdentityResult displays the result of an identity_set change.
function showIdentityResult(msg) {
  const el = document.createElement("div");
  el.className = "set-result " + (msg.ok ? "ok" : "err");
  el.textContent = msg.ok ? (msg.message || "ok") : (msg.message || "error");
  identityBody.prepend(el);
  setTimeout(() => el.remove(), 3000);
}

/* ---------- bootstrap ---------- */

// refreshBranch re-fetches /api/bootstrap and updates the sidebar title's
// git branch (e.g. "工作区 · main"). Called on page load and after every
// LLM iteration (done event) so a branch switch made by the agent is
// reflected without a manual reload.
async function refreshBranch() {
  try {
    const resp = await fetch("/api/bootstrap");
    const b = await resp.json();
    if (b.branch) document.getElementById("wsBranch").textContent = b.branch;
  } catch { /* keep the current branch on failure */ }
}

(async function boot() {
  try {
    const resp = await fetch("/api/bootstrap");
    const b = await resp.json();
    if (b.lang === "en") { T = I18N.en; currentLang = "en"; }
    document.documentElement.lang = b.lang || "zh";
    document.getElementById("ver").textContent = "v" + b.version + " [BUILD-" + b.build + "]";
    // Browser tab title = "{current folder name} - {full absolute path}"
    // (FEATURE-366), so multiple co-shell tabs are distinguishable at a
    // glance and the most salient feature (the folder name) shows first
    // even when the tab is short.
    if (b.workspace) {
      const ws = b.workspace.replace(/\\+$/, "").replace(/\/+$/, "");
      const name = ws.split(/[\\/]/).pop() || ws;
      document.title = name + " - " + b.workspace;
    }
    // Sidebar title shows the current git branch at the right edge of the
    // panel head (e.g. "工作区 ⟳   main").
    if (b.branch) document.getElementById("wsBranch").textContent = b.branch;
    // FEATURE-378: active text/vision model context info for the status bar.
    if (b.textModel) {
      modelInfo = { textModel: b.textModel, textMaxLen: b.textMaxLen || 0, visionModel: b.visionModel || "", visionMaxLen: b.visionMaxLen || 0, modeTextModelID: b.modeTextModelID || "", modeVisionModelID: b.modeVisionModelID || "" };
    }
  } catch { /* defaults stay zh */ }
  applyI18n();
  initStreamMode(); // FEATURE-425: wire the display-mode pill + session title
  autoGrow(); // FEATURE-405: set the initial input height correctly on load
  setRunning(false); // apply localized button title
  applyPanels();
  applyStatus();
  updateStatus();
  // FEATURE-383: leaving the sidebar returns it to the fixed width after a
  // short delay (the auto-expand is triggered per-row on hover).
  let sidebarAutoTimer = 0;
  sidebar.addEventListener("mouseleave", () => {
    clearTimeout(sidebarAutoTimer);
    sidebarAutoTimer = setTimeout(() => layout.classList.remove("sidebar-auto"), 2500);
  });
  loadTree();
  wsConnect();
})();

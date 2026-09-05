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
    attach: "附加文件", clearAttach: "清空附件", attachRemove: "移除附件",
    attachKindImage: "图片", attachKindFile: "文件",
    attachUploading: "上传附件中…", attachFail: "附件上传失败，请重试",
    attachPreviewFile: "发送后将保存到工作区（可被工具读取）",
    planEmpty: "（无步骤）",
    menu: "菜单", settings: "系统设置", identity: "身份与个性", restart: "重启后台",
    appearance: "[ 外观 ]",
    themeMode: "主题", themeAuto: "跟随系统", themeDark: "深色", themeLight: "浅色",
    statusBar: "状态条",
    sbSession: "Σ", sbLast: "⏱️",
    revealDir: "定位到文件夹",
    downloadFile: "下载文件",
    sessionDelete: "删除会话",
    sessionActive: "当前会话",
    sessionCount: "消息计数",
    sessionDeleteConfirm: "确定要删除会话「%s」吗？此操作不可撤销。",
    modelDeleteConfirm: "确定要删除模型「%s」吗？此操作不可撤销。",
    approveCount: "批准N次",
    approve: "批准", approveAll: "全部批准", approveG: "永久自动执行", approveD: "永久禁用",
    supplement: "补充信息", supplementHint: "长按按钮或单击输入框可补充信息",
    numberHint: "按数字键选择放行次数（0=10次）",
    cancel: "取消", confirm: "确认",
    copyBlock: "复制内容", collapseBlock: "收起同类块", expandBlock: "展开同类块", retryFrom: "从此处重新运行",
    switchMode: "切换工作模式",
    models: "模型管理", modelAdd: "＋ 新增模型", modelWizard: "模型配置向导", templateJson: "查看模板原始 JSON", reasoningEffortNone: "不设置", apiTypeChatDefault: "chat（默认）",
    modelEmpty: "暂无模型，点击上方「＋ 新增模型」添加", modelMenuTitle: "选择主模型", modelVisionMenuTitle: "选择视觉模型", modelVisionEmpty: "暂无视觉模型", modelDefault: "默认", modelDefaultHint: "使用全局默认模型", modelRestoreDefault: "默认",
    fileViewerClose: "关闭", fileViewerLoadFailed: "文件读取失败",
    fileViewerSearch: "搜索文件内容…", fileViewerRaw: "Raw",
    streamModeSilent: "静默", streamModeMinimal: "极简", streamModeNormal: "正常",
    streamTitlePlaceholder: "会话标题", streamTitleHint: "点击修改会话标题",
    yoloTitle: "YOLO 模式（You Only Live Once）：开启后所有工具调用自动批准，无需逐个确认",
    setDefaultTip: "默认值", setDiffTip: "与默认值不一致",
  },
  en: {
    workspace: "Workspace", refresh: "Refresh",
    taskPlan: "Task Plan", reply: "Reply", interrupt: "Interrupt", send: "Send",
    inputHint: "Type a command — Enter to send, Shift+Enter for newline, ↑↓ history",
    connected: "connected", disconnected: "disconnected",
    askLine: "The agent asks for a line of input:", askKey: "The agent asks for a key:",
    uploadFailed: "Upload failed", actionFailed: "Action failed",
    attach: "Attach file", clearAttach: "Clear attachments", attachRemove: "Remove attachment",
    attachKindImage: "image", attachKindFile: "file",
    attachUploading: "Uploading attachments…", attachFail: "Attachment upload failed, retry",
    attachPreviewFile: "Saved to workspace after send (readable by tools)",
    planEmpty: "(no steps)",
    menu: "Menu", settings: "Settings", identity: "Identity & Personality", restart: "Restart backend",
    appearance: "[ Appearance ]",
    themeMode: "Theme", themeAuto: "Follow system", themeDark: "Dark", themeLight: "Light",
    statusBar: "Status bar",
    sbSession: "Σ", sbLast: "⏱️",
    revealDir: "Reveal in folder",
    downloadFile: "Download file",
    sessionDelete: "Delete session",
    sessionActive: "Current session",
    sessionCount: "Message count",
    sessionDeleteConfirm: "Delete session \"%s\"? This cannot be undone.",
    modelDeleteConfirm: "Delete model \"%s\"? This cannot be undone.",
    approveCount: "Approve N times",
    approve: "Approve", approveAll: "Approve all", approveG: "Always auto-execute", approveD: "Permanently disable",
    supplement: "Supplement", supplementHint: "Long-press a button or click the input box to supplement",
    numberHint: "Press a digit to choose approve-count (0=10)",
    cancel: "Cancel", confirm: "Confirm",
    copyBlock: "Copy content", collapseBlock: "Collapse same-type blocks", expandBlock: "Expand same-type blocks", retryFrom: "Retry from here",
    switchMode: "Switch work mode",
    models: "Model Manager", modelAdd: "＋ Add Model", modelWizard: "Model Setup Wizard", templateJson: "View template raw JSON", reasoningEffortNone: "Not set", apiTypeChatDefault: "chat (default)",
    modelEmpty: "No models yet. Click「＋ Add Model」above to add one.", modelMenuTitle: "Select main model", modelVisionMenuTitle: "Select vision model", modelVisionEmpty: "No vision models", modelDefault: "Default", modelDefaultHint: "Use global default model", modelRestoreDefault: "Default",
    fileViewerClose: "Close", fileViewerLoadFailed: "Failed to read file",
    fileViewerSearch: "Search file content…", fileViewerRaw: "Raw",
    streamModeSilent: "Silent", streamModeMinimal: "Minimal", streamModeNormal: "Normal",
    streamTitlePlaceholder: "Session title", streamTitleHint: "Click to edit session title",
    yoloTitle: "YOLO mode (You Only Live Once): when on, all tool calls are auto-approved without asking",
    setDefaultTip: "Default", setDiffTip: "differs from default",
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

// i18nT returns the localized string for key, falling back to the given default
// when the key is not present in the current language dictionary (FEATURE-464).
function i18nT(key, fallback) {
  return (T && T[key]) ? T[key] : (fallback || key);
}

/* ---------- theme ---------- */

// localStorage "co-shell-theme": "auto" (default, follow the OS) | "dark" |
// "light" | "light-tp" | "paper". setTheme only applies; persistence is the
// caller's job so that "auto" is never clobbered by a resolved value.
const themeToggle = document.getElementById("themeToggle");
const osThemeMQ = window.matchMedia ? window.matchMedia("(prefers-color-scheme: dark)") : null;

// themeIcon returns the toggle glyph for a resolved tone (FEATURE-477):
// dark = moon, light = sun, light-tp = spade, paper = coffee.
function themeIcon(name) {
  if (name === "dark") return "☾";
  if (name === "light-tp") return "♤";
  if (name === "paper") return "☕︎";
  return "☀";
}

function setTheme(name) {
  document.documentElement.setAttribute("data-theme", name);
  themeToggle.textContent = themeIcon(name);
}

function themeMode() {
  const saved = localStorage.getItem("co-shell-theme");
  return saved === "dark" || saved === "light" || saved === "light-tp" || saved === "paper" ? saved : "auto";
}

function applyTheme() {
  const mode = themeMode();
  // No matchMedia (browser cannot report the OS scheme): fall back to dark
  // (FIX-363).
  let resolved;
  if (mode === "dark") resolved = "dark";
  else if (mode === "light") resolved = "light";
  else if (mode === "light-tp") resolved = "light-tp";
  else if (mode === "paper") resolved = "paper";
  else resolved = (!osThemeMQ || osThemeMQ.matches) ? "dark" : "light"; // auto
  setTheme(resolved);
  const sel = document.getElementById("setThemeMode");
  if (sel && sel.value !== mode) sel.value = mode;
  updateBrandLogo();
}

// brandLogo is the topbar system-logo <img> (FEATURE-477).
const brandLogo = document.getElementById("brandLogo");

// logoScale returns the topbar logo display scale (1-200%, default 100) from
// localStorage (FEATURE-477).
function logoScale() {
  const v = parseInt(localStorage.getItem("co-shell-logo-scale"), 10);
  return (v >= 1 && v <= 200) ? v : 100;
}

// updateBrandLogo shows the logo configured for the current resolved theme
// (dark|light|light-tp) in the topbar, or hides it (falling back to the default
// ▸ co-shell text) when none is configured. The logo is served by GET
// /logos/{theme}; a 404 means no logo for that theme. Its display height is
// the titlebar height (44px) scaled by the configured logo scale.
function updateBrandLogo() {
  if (!brandLogo) return;
  // Each tone (dark/light/light-tp) has its own logo (FEATURE-477).
  const theme = document.documentElement.getAttribute("data-theme");
  // A cache-busting query param forces the browser to re-fetch the logo so an
  // overwrite/removal is reflected immediately (FEATURE-477).
  const url = "/logos/" + theme + "?t=" + Date.now();
  brandLogo.style.height = Math.round(44 * logoScale() / 100) + "px";
  const probe = new Image();
  probe.onload = () => {
    brandLogo.src = url;
    brandLogo.hidden = false;
  };
  probe.onerror = () => {
    brandLogo.hidden = true;
    brandLogo.removeAttribute("src");
  };
  probe.src = url;
}

if (osThemeMQ && osThemeMQ.addEventListener) {
  osThemeMQ.addEventListener("change", () => {
    if (themeMode() === "auto") applyTheme();
  });
}
applyTheme();

themeToggle.onclick = () => {
  const cur = document.documentElement.getAttribute("data-theme");
  // Cycle through the four tones: dark -> light -> light-tp -> paper -> dark
  // (FEATURE-477). Clicking always pins an explicit tone (never auto).
  const next = cur === "dark" ? "light"
    : (cur === "light" ? "light-tp"
      : (cur === "light-tp" ? "paper" : "dark"));
  localStorage.setItem("co-shell-theme", next);
  applyTheme();
};

/* ---------- DOM handles ---------- */

const conn = document.getElementById("conn");
const connDot = document.getElementById("connDot");
const connText = document.getElementById("connText");
const stream = document.getElementById("stream");
const streamB = document.getElementById("streamB");
// FEATURE-445: floating down-arrow button to jump back to the output bottom.
const scrollDownBtn = document.getElementById("scrollDownBtn");
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

/* FEATURE-478: IME composition guard. When an input method (e.g. Chinese
   pinyin) is composing, pressing Enter confirms a candidate word rather than
   sending. Browsers expose KeyboardEvent.isComposing during the composition
   session, but some IMEs fire one more keydown right after compositionend
   where isComposing is already false. So we also track a document-level
   composing flag via compositionstart/compositionend as a fallback. */
window.__composing = false;
// Native composition events do NOT bubble (bubbles=false), so a bubble-phase
// listener on document never fires for real IME input. Listen in the CAPTURE
// phase (third arg true) so document receives them on the way down to the
// focused input element.
document.addEventListener("compositionstart", () => { window.__composing = true; }, true);
// __imeJustEnded: some IMEs (esp. macOS pinyin) end the composition BEFORE the
// Enter that confirms the candidate reaches the page, so isComposing/__composing
// are already false when that Enter fires. To catch it, after compositionend we
// open a short window during which an Enter is treated as candidate-confirm
// (not send). The window closes on a timeout, so a genuine send Enter right
// after selecting still works.
window.__imeJustEnded = false;
let __imeEndTimer = null;
document.addEventListener("compositionend", () => {
  window.__composing = false;
  window.__imeJustEnded = true;
  clearTimeout(__imeEndTimer);
  __imeEndTimer = setTimeout(() => { window.__imeJustEnded = false; }, 400);
}, true);
// imeComposing(e) returns true while an IME composition is active or just
// ended, in which case an Enter keypress must be ignored (it selects a
// candidate, not send).
function imeComposing(e) {
  return e.isComposing === true || window.__composing === true || window.__imeJustEnded === true;
}
const yoloSwitch = document.getElementById("yoloSwitch");
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
const settingsSearch = document.getElementById("settingsSearch");
const settingsNav = document.getElementById("settingsNav");
const modelsModal = document.getElementById("models");
const modelsClose = document.getElementById("modelsClose");
const modelsBody = document.getElementById("modelsBody");
const modelAddBtn = document.getElementById("modelAddBtn");
const modelPinBtn = document.getElementById("modelPinBtn");
const modelDelBtn = document.getElementById("modelDelBtn");
const modelWizardModal = document.getElementById("modelWizard");
const modelWizardBody = document.getElementById("modelWizardBody");
const modelWizardCancel = document.getElementById("modelWizardCancel");
const modelWizardNav = document.getElementById("modelWizardNav");
const modelWizardPrev = document.getElementById("modelWizardPrev");
const modelWizardNext = document.getElementById("modelWizardNext");
const modelWizardSubmit = document.getElementById("modelWizardSubmit");
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
const preview = document.getElementById("preview");
const previewImg = document.getElementById("previewImg");
const previewClose = document.getElementById("previewClose");
// FEATURE-444: image preview title bar (name / size / mtime / resolution).
const pvName = document.getElementById("pvName");
const pvSize = document.getElementById("pvSize");
const pvMtime = document.getElementById("pvMtime");
const pvRes = document.getElementById("pvRes");
// FEATURE-425: read-only text file previewer.
const fileViewer = document.getElementById("fileViewer");
const fvBody = document.getElementById("fvBody");
// FEATURE-432/435: floating title bar (path / mtime / Raw / close).
const fvTitlebar = document.getElementById("fvTitlebar");
const fvPathEl = document.getElementById("fvPath");
const fvSizeEl = document.getElementById("fvSize");
const fvMtimeEl = document.getElementById("fvMtime");
const fvRawEl = document.getElementById("fvRaw");
const fvClose = document.getElementById("fvClose");
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
const delModelModal = document.getElementById("delModel");
const delModelMsg = document.getElementById("delModelMsg");
const delModelClose = document.getElementById("delModelClose");
const delModelCancel = document.getElementById("delModelCancel");
const delModelConfirm = document.getElementById("delModelConfirm");

/* ---------- websocket ---------- */

let ws = null;
let wsReady = false;

// FEATURE-458: the connection status control is now a connect/disconnect
// toggle. When disconnected, we do NOT auto-reconnect — the user must click
// the control to reconnect (avoids two browsers fighting over the single
// WebSocket client connection).
function wsConnect() {
  if (wsReady || (ws && (ws.readyState === WebSocket.CONNECTING || ws.readyState === WebSocket.OPEN))) return;
  ws = new WebSocket("ws://" + location.host + "/ws");
  ws.onopen = () => {
    wsReady = true;
    conn.classList.add("on");
    connText.textContent = T.connected;
    wsSend({ type: "mode_get" }); // FEATURE-410: load the work-mode list
    // Fetch the session list on connect so the 💬 count is correct immediately
    // (FEATURE-387), not only after hovering the status-bar item.
    wsSend({ type: "session_list" });
    // FEATURE-439: load the YOLO master switch state (defaults to off).
    wsSend({ type: "yolo_get" });
  };
  ws.onclose = () => {
    wsReady = false;
    conn.classList.remove("on");
    connText.textContent = T.disconnected;
    hideAsk();
    setRunning(false);
    // FEATURE-458: no auto-reconnect. The user clicks the connection control
    // to reconnect manually.
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
    else if (msg.kind === "mcp") renderMCPServers(msg.mcp_servers || []);
    else if (msg.kind === "mcp_result") showMCPResult(msg);
    else if (msg.kind === "identity") renderIdentity(msg.identity || []);
    else if (msg.kind === "identity_result") showIdentityResult(msg);
    else if (msg.kind === "mode") renderModeSeg(msg.modes || []);
    else if (msg.kind === "mode_result") showModeResult(msg);
    else if (msg.kind === "models") renderModels(msg.models || [], msg.templates || []);
    else if (msg.kind === "model_result") showModelResult(msg);
    else if (msg.kind === "model_wizard") showModelWizardStep(msg);
    else if (msg.kind === "pop_result") {
      // FEATURE-409: retry-from popped the session back; reload so the stream
      // reflects the truncated history.
      if (msg.ok) location.reload();
    }
    else if (msg.kind === "yolo") setYOLO(!!msg.yolo);
    else if (msg.kind === "dynamic_backfill") backfillInput(msg.backfill || []);
  };
}

// FEATURE-458: actively close the WebSocket (user clicked the toggle while
// connected).
function wsDisconnect() {
  if (ws) {
    try { ws.close(); } catch (e) { /* ignore */ }
  }
  wsReady = false;
  conn.classList.remove("on");
  connText.textContent = T.disconnected;
  hideAsk();
  setRunning(false);
}

// FEATURE-458: the connection control is a toggle — click to connect when
// disconnected, click to disconnect when connected.
conn.onclick = () => {
  if (wsReady) {
    wsDisconnect();
  } else {
    wsConnect();
  }
};

function wsSend(obj) {
  if (wsReady) ws.send(JSON.stringify(obj));
}

/* ---------- event stream rendering ---------- */

const CHAN_LABEL = {
  llm: "LLM", tool: "TOOL", command: "CMD", system: "SYS",
  taskplan: "PLAN", memory: "MEM", mcp: "MCP", db: "DB",
  wizard: "WIZ", debug: "DBG", bridge: "BRG", subagent: "SUB", repl: "REPL",
  supervisor: "SUP",
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

// riskLabel returns the localized risk-level label for a risk key
// (low/medium/high). zh shows 低风险/中风险/高风险, en shows Low/Medium/High
// (FIX-462). Unknown keys fall back to the uppercased key.
function riskLabel(risk) {
  const map = {
    low: currentLang === "en" ? "Low" : "低风险",
    medium: currentLang === "en" ? "Medium" : "中风险",
    high: currentLang === "en" ? "High" : "高风险",
  };
  return map[risk] || String(risk).toUpperCase();
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
// FEATURE-460: current streaming SUP block (problem solver / loop judge /
// supervisor LLM interaction). { body, raw, raf, title }.
let curSup = null;
// FIX-448: toolName -> curTool object map, so when one LLM iteration calls
// multiple tools the input/result events can target the correct block instead
// of always the last one (curTool points to the last block after streaming).
let toolBlockByName = {};
// FIX-462: ordered list of tool blocks created in the current iteration (in
// creation order). The tool_call input events backfill the intent to each block
// one by one; when the "⚙️" header is missed (so toolBlockByName has no entry)
// the next unmatched block in this list is used instead of the last one.
let iterToolBlocks = [];
let curREPL = null;     // current repl block (consecutive ui_text lines merge)
let lastMsgIndex = "";  // last message index seen, for the YOU block retry-from

// FEATURE-445: follow-output scrolling. When the scrollbar is within 100px of
// the content bottom, new output auto-scrolls to the bottom (follows output);
// otherwise it does not, so the user can read history. A floating down-arrow
// button appears when scrolled >100px from the bottom; clicking it jumps to
// the bottom and resumes following.
const FOLLOW_THRESHOLD = 100;
let followOutput = true; // whether new output should auto-scroll to the bottom

function scrollStream() {
  if (followOutput) {
    streamB.scrollTop = streamB.scrollHeight;
    if (scrollDownBtn) scrollDownBtn.classList.remove("on");
  }
}

// updateFollowState recomputes followOutput from the current scroll position
// and toggles the floating down-arrow button (FEATURE-445).
function updateFollowState() {
  const atBottom = streamB.scrollTop + streamB.clientHeight >= streamB.scrollHeight - FOLLOW_THRESHOLD;
  followOutput = atBottom;
  if (scrollDownBtn) scrollDownBtn.classList.toggle("on", !atBottom);
}

// jumpToBottom scrolls to the bottom and resumes following output.
function jumpToBottom() {
  followOutput = true;
  streamB.scrollTop = streamB.scrollHeight;
  if (scrollDownBtn) scrollDownBtn.classList.remove("on");
}

// FEATURE-419: floating block-boundary navigation. When the current block is
// taller than the viewport and its title line / bottom is out of view, show a
// floating ↑ / ↓ icon at the top / bottom of the main data area so the user can
// jump to that boundary. The "current block" is the .ev block with the largest
// visible area inside the viewport.
let blockNavCurrent = null; // the .ev box the icons currently target

function updateBlockNav() {
  if (!blockNavTop || !blockNavBottom) return;
  const scroller = streamB;
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
    streamB.scrollTop = blockNavCurrent.offsetTop;
  });
  blockNavBottom.addEventListener("click", () => {
    if (!blockNavCurrent) return;
    streamB.scrollTop = blockNavCurrent.offsetTop + blockNavCurrent.offsetHeight - streamB.clientHeight;
  });
  streamB.addEventListener("scroll", updateBlockNav);
}

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

// newSupBlock creates a SUP block with three sections (FEATURE-461):
//   top    - the prompt sent to the LLM (.sup-prompt, height-limited)
//   middle - the streaming reply (the main .ev-body, height-limited)
//   bottom - the tool call's input (.sup-tool, height-limited)
// Each section has a title bar with a Raw pill and an expand/collapse toggle,
// mirroring the TOOL input-parameter sub-block (FEATURE-461).
function newSupBlock(title, msgIndex) {
  const body = makeBlock("supervisor", title, msgIndex);
  const box = body.parentElement;
  // Top: prompt section.
  const prompt = document.createElement("div");
  prompt.className = "sup-prompt";
  const promptPart = makeSupPart(prompt, "提示词");
  box.insertBefore(prompt, body);
  // Bottom: tool input section.
  const tool = document.createElement("div");
  tool.className = "sup-tool";
  const toolPart = makeSupPart(tool, "工具输入");
  box.appendChild(tool);
  // Middle: the streaming reply (main ev-body). Add Raw + expand controls to
  // the block's own title bar and height-limit the body.
  const state = { body, raw: "", raf: 0, title, promptBody: promptPart.body, toolBody: toolPart.body, prompt: promptPart, tool: toolPart, contentRawMode: false };
  const head = box.querySelector(".ev-head");
  if (head) addSupContentControls(head, state);
  body.classList.add("sup-content-body");
  return state;
}

// makeSupPart builds a titled, height-limited, expandable/raw section inside a
// SUP block (FEATURE-461). Returns { body, raw, rawMode }.
function makeSupPart(wrap, title) {
  const head = document.createElement("div");
  head.className = "sup-part-head";
  const titleEl = document.createElement("span");
  titleEl.className = "sup-part-title";
  titleEl.textContent = title;
  const rawPill = document.createElement("button");
  rawPill.className = "sup-part-raw";
  rawPill.textContent = "Raw";
  rawPill.title = "Raw / md 渲染";
  const toggle = document.createElement("button");
  toggle.className = "sup-part-toggle";
  toggle.textContent = "⤢";
  toggle.title = "展开/固定高度";
  const right = document.createElement("span");
  right.className = "sup-part-right";
  right.appendChild(rawPill);
  right.appendChild(toggle);
  head.appendChild(titleEl);
  head.appendChild(right);
  const body = document.createElement("div");
  body.className = "sup-part-body";
  wrap.appendChild(head);
  wrap.appendChild(body);
  const part = { body, raw: "", rawMode: false };
  rawPill.onclick = () => {
    rawPill.classList.toggle("on");
    part.rawMode = !part.rawMode;
    renderSupPart(part);
  };
  toggle.onclick = () => {
    wrap.classList.toggle("expanded");
    toggle.textContent = wrap.classList.contains("expanded") ? "⤡" : "⤢";
  };
  return part;
}

// renderSupPart renders a SUP part body according to its rawMode state: raw
// text when ON, markdown otherwise (FEATURE-461).
function renderSupPart(part) {
  if (part.rawMode) {
    part.body.classList.remove("md");
    part.body.textContent = part.raw;
  } else {
    part.body.classList.add("md");
    mdRender(part.body, part.raw);
  }
}

// addSupContentControls adds a Raw pill and an expand/collapse toggle to the
// SUP block's main title bar, controlling the streaming reply body (FEATURE-461).
// state is the SUP block state object (not the global curSup) so each block's
// controls act on their own content even when multiple SUP blocks exist.
function addSupContentControls(head, state) {
  const body = state.body;
  const rawPill = document.createElement("button");
  rawPill.className = "sup-part-raw";
  rawPill.textContent = "Raw";
  rawPill.title = "Raw / md 渲染";
  const toggle = document.createElement("button");
  toggle.className = "sup-part-toggle";
  toggle.textContent = "⤢";
  toggle.title = "展开/固定高度";
  const right = document.createElement("span");
  right.className = "sup-part-right";
  right.appendChild(rawPill);
  right.appendChild(toggle);
  head.appendChild(right);
  rawPill.onclick = () => {
    rawPill.classList.toggle("on");
    state.contentRawMode = !state.contentRawMode;
    if (state.contentRawMode) {
      body.classList.remove("md");
      body.textContent = state.raw;
    } else {
      body.classList.add("md");
      mdRender(body, state.raw);
    }
  };
  toggle.onclick = () => {
    body.parentElement.classList.toggle("sup-content-expanded");
    toggle.textContent = body.parentElement.classList.contains("sup-content-expanded") ? "⤡" : "⤢";
  };
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
  // FEATURE-445: all blocks append to the single stream region B.
  streamB.appendChild(box);
  scrollStream();
  return body;
}

// applyBlockDisplayMode shows/hides or collapses a single block according to
// the current display mode (FEATURE-425).
function applyBlockDisplayMode(box, cls) {
  if (displayMode === "silent") {
    const show = cls === "user-msg" || box.classList.contains("level-error") || box.classList.contains("ev-result");
    box.style.display = show ? "" : "none";
    // The final result block must be fully expanded so the user sees the
    // completion report (FIX-426).
    if (box.classList.contains("ev-result")) box.classList.remove("collapsed");
  } else if (displayMode === "minimal") {
    box.style.display = "";
    // User-msg blocks are always expanded so the user sees their original
    // instruction (FIX-426).
    if (cls === "user-msg") { box.classList.remove("collapsed"); return; }
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
         (curTool && curTool.body === body) || (curREPL && curREPL.body === body) ||
         (curSup && curSup.body === body);
}

// markStreaming flags a block as currently streaming: it shows the dynamic
// "..." next to the title and forces the block expanded so the live content is
// always visible (FEATURE-409). In minimal display mode, when a block starts
// streaming, all other non-user blocks collapse so only the current live block
// stays expanded (FIX-426).
function markStreaming(body) {
  const box = body.parentElement;
  if (!box) return;
  // FEATURE-429: reuse the block's own title-bar dot (.ev-head::before) as the
  // breathing indicator while streaming.
  const head = box.querySelector(".ev-head");
  if (head) head.classList.add("streaming");
  box.classList.remove("collapsed");
  if (displayMode === "minimal") {
    document.querySelectorAll(".ev").forEach((b) => {
      if (b === box) return;
      // User blocks and the final result block stay expanded (FIX-426).
      if (b.classList.contains("user-msg") || b.classList.contains("ev-result")) return;
      b.classList.add("collapsed");
    });
  }
}

// unmarkStreaming hides the dynamic "..." of a block once it stops streaming
// (FEATURE-409). In minimal display mode a finished block collapses to just
// its title, but only when another block is still streaming (so the last
// finished block stays expanded); user-msg blocks are never collapsed so the
// user always sees their original instruction (FIX-426).
function unmarkStreaming(body) {
  if (!body) return;
  const box = body.parentElement;
  if (!box) return;
  const head = box.querySelector(".ev-head");
  if (head) head.classList.remove("streaming");
  if (displayMode !== "minimal") return;
  if (box.classList.contains("user-msg") || box.classList.contains("ev-result")) return;
  // Only collapse when some other block is still streaming (the last finished
  // block stays expanded so the user sees its final content).
  const anyStreaming = Array.from(document.querySelectorAll(".ev .ev-body")).some(isStreamingBody);
  if (anyStreaming) box.classList.add("collapsed");
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
    case "ui_text":
      if (ev.chan === "repl") return "repl" + lvl;
      if (ev.chan === "supervisor") return "supervisor" + lvl;
      return "system" + lvl;
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
      const p = parseInt(m.prompt, 10) || 0;
      const c = parseInt(m.completion, 10) || 0;
      const inTPS = parseInt(m.in_tps, 10) || 0;
      const outTPS = parseInt(m.out_tps, 10) || 0;
      if (m.prompt) {
        parts.push("↑" + fmtNum(p));
        if (m.ft && m.ft !== "-") parts.push(m.ft);
        if (inTPS > 0) parts.push(fmtDur(p / inTPS) + " " + fmtNum(inTPS) + "t/s");
      }
      if (m.completion) {
        parts.push("↓" + fmtNum(c));
        if (outTPS > 0) parts.push(fmtNum(outTPS) + "t/s");
      }
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
    curLLM = curThinking = curSup = null;
    // FIX-474: the LLM iteration ended — reset the tool-block tracking state so
    // the next iteration starts clean. Without this, iterToolBlocks/curTool/
    // toolBlockByName persist across iterations (only cleared at the final done
    // event), so an "orphan" block from an earlier iteration (created by a ⚙️
    // header but whose tool_call input event never arrived, e.g. plan tools that
    // skip meta) stays unfilled and steals the intent of a later tool call,
    // misplacing the intent onto the wrong TOOL block.
    curTool = null;
    toolBlockByName = {};
    iterToolBlocks = [];
    // FEATURE-409: the LLM iteration ended (token usage refreshed) — stop the
    // breathing dot on all blocks now, not only at the final done event.
    document.querySelectorAll(".ev-head.streaming").forEach((h) => h.classList.remove("streaming"));
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
    toolBlockByName = {};
    iterToolBlocks = [];
    // FEATURE-427: mark the last content block as the result block so silent
    // mode shows and expands it — the final completion block, whatever its type
    // (TOOL: 完成任务, or a final LLM summary). Skip meta (token-stats) rows.
    const allBlocks = document.querySelectorAll(".ev");
    for (let i = allBlocks.length - 1; i >= 0; i--) {
      const b = allBlocks[i];
      if (b.classList.contains("meta")) continue;
      b.classList.add("ev-result");
      applyBlockDisplayMode(b, b.className.replace("ev ", "").split(" ")[0]);
      break;
    }
    // FEATURE-409: stop the breathing dot on all blocks once streaming ends.
    document.querySelectorAll(".ev-head.streaming").forEach((h) => h.classList.remove("streaming"));
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
      // FEATURE-460: content_chunk on the supervisor channel is a streaming
      // SUP block (problem solver / loop judge / supervisor LLM interaction).
      // The scenario title is carried in ev.meta.sup_scenario.
      if (ev.chan === "supervisor") {
        const supTitle = (ev.meta && ev.meta.sup_scenario) || "SUP";
        const supPart = (ev.meta && ev.meta.sup_part) || "content";
        // FEATURE-461: a prompt chunk starts a new round -> a new SUP block.
        if (supPart === "prompt") {
          unmarkStreaming(curSup && curSup.body);
          unmarkStreaming(curLLM && curLLM.body);
          unmarkStreaming(curThinking && curThinking.body);
          unmarkStreaming(curTool && curTool.body);
          curSup = newSupBlock(supTitle, msgIndex);
          curSup.prompt.raw = ev.text || "";
          renderSupPart(curSup.prompt);
          markStreaming(curSup.body);
          scrollStream();
          return;
        }
        if (!curSup || curSup.title !== supTitle) {
          // No active SUP block (e.g. stream on but prompt off): create one.
          unmarkStreaming(curSup && curSup.body);
          curSup = newSupBlock(supTitle, msgIndex);
          curLLM = null;
          curThinking = null;
          curTool = null;
        }
        if (supPart === "tool") {
          curSup.tool.raw = ev.text || "";
          renderSupPart(curSup.tool);
          scrollStream();
          return;
        }
        // content: stream into the middle section.
        curSup.raw += ev.text || "";
        markStreaming(curSup.body);
        if (curSup.contentRawMode) {
          curSup.body.classList.remove("md");
          curSup.body.textContent = curSup.raw;
        } else {
          scheduleMd(curSup);
        }
        scrollStream();
        return;
      }
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
    if (!curTool || isNewTool) {
      curTool = newStreamBlock("tool", "TOOL", msgIndex);
      // FIX-462: record the block in creation order so the tool_call input
      // events can backfill the intent to each block one by one, even when the
      // "⚙️" header is missed (toolBlockByName has no entry for it).
      iterToolBlocks.push(curTool);
    }
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
        // FIX-448: record the tool name on the block so the later tool_call
        // input/result events can target this block (curTool points to the
        // last block when one iteration calls multiple tools).
        const name = text.slice(gear + 2, nl >= 0 ? nl : text.length).trim();
        if (name) {
          curTool.toolName = name;
          toolBlockByName[name] = curTool;
        }
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
        // FIX-448: when one iteration calls multiple tools, curTool points to
        // the LAST tool's block (set by tool_call_stream). Switch to the block
        // matching this tool's name so its title/risk and the following result
        // event land on the correct block, in call order.
        // FIX-462: if the name lookup misses (the "⚙️" header was not detected
        // for this tool), fall back to the next unmatched block in creation
        // order (iterToolBlocks) so the intent is backfilled to each block one
        // by one instead of always the last one.
        // FIX-462: prefer matching by creation order (iterToolBlocks) so that
        // when multiple tools share the same name (e.g. two execute_command
        // calls) each tool_call event lands on its own block instead of all
        // landing on the last one (toolBlockByName is keyed by name and gets
        // overwritten by the last same-named block).
        // FIX-474: among the unfilled blocks, prefer one whose tool name matches
        // this tool call. This skips "orphan" blocks (created by a ⚙️ header but
        // whose tool_call input event never arrives, e.g. plan tools that skip
        // meta) so their unfilled slot does not steal the intent of a later,
        // differently-named tool call. Same-named tools still resolve in
        // creation order (first unfilled matching-name block).
        let target = null;
        if (summary.tool_name && iterToolBlocks.length) {
          target = iterToolBlocks.find((b) => !b._intentFilled && b.toolName === summary.tool_name);
        }
        if (!target && iterToolBlocks.length) {
          target = iterToolBlocks.find((b) => !b._intentFilled);
        }
        if (!target && summary.tool_name && toolBlockByName[summary.tool_name] && !toolBlockByName[summary.tool_name]._intentFilled) {
          target = toolBlockByName[summary.tool_name];
        }
        if (target) curTool = target;
        curTool._intentFilled = true;
        const head = curTool.body.parentElement.children[0];
        const action = toolAction(summary.tool_name);
        const text = "TOOL: " + action + (summary.intent ? " - " + summary.intent : "");
        // FEATURE-409: update only the title label, preserving the action icons
        // (copy/collapse/retry) and the streaming "..." in the head.
        const label = head.querySelector(".ev-head-label");
        if (label) label.textContent = text;
        else head.textContent = text;
        // FEATURE-447: show the risk level as a coloured badge in the tool
        // block title bar (low=green, medium=yellow, high=red). It is inserted
        // right after the title label, before the action icons.
        // FIX-462: the badge text is localized (低风险/中风险/高风险 in zh,
        // Low/Medium/High in en) instead of the raw LOW/MEDIUM/HIGH.
        if (summary.risk) {
          const riskBadge = document.createElement("span");
          riskBadge.className = "risk-badge risk-" + summary.risk;
          riskBadge.textContent = riskLabel(summary.risk);
          if (summary.risk_reason) riskBadge.title = summary.risk_reason;
          const actions = head.querySelector(".ev-actions");
          if (actions) head.insertBefore(riskBadge, actions);
          else head.appendChild(riskBadge);
        }
        // FEATURE-447: highlight the affected files in the workspace tree.
        // Deterministic files use the accent text colour (no background/border),
        // predicted files use blue.
        if (summary.files && summary.files.length) {
          highlightAffectedFiles(summary.files);
        }
        // FIX-426: the completion block (TOOL: 完成任务) is marked as the
        // result block so minimal/silent modes keep it expanded. The label at
        // makeBlock time is just "TOOL", so mark it here once the real title
        // is known, and un-collapse it (it may have been collapsed at creation
        // before the ev-result class was known).
        if (/完成任务|Complete task/.test(text)) {
          const box = curTool.body.parentElement;
          box.classList.add("ev-result");
          box.classList.remove("collapsed");
          // Re-apply the display mode so silent mode shows this block now that
          // it is known to be the result block (it was hidden at makeBlock time
          // because the label was still just "TOOL").
          applyBlockDisplayMode(box, "tool");
        }
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
    curLLM = curThinking = curSup = null;
    scrollStream();
    return;
  }

  curLLM = curThinking = curSup = null;
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
  // FEATURE-457: supervisor review output renders as a dedicated SUP block
  // with a pure bright-white font and a pass/reject indicator light.
  if (ev.type === "ui_text" && ev.chan === "supervisor") {
    const body = makeBlock("supervisor", "SUP", msgIndex);
    body.textContent = ev.text || "";
    const box = body.parentElement;
    const t = ev.text || "";
    if (/打回|reject|Reject/.test(t)) box.classList.add("supervisor-reject");
    else box.classList.add("supervisor-pass");
    scrollStream();
    return;
  }
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
  renderUserBody(body, text);
  // FEATURE-419: after creating the YOU block, scroll to the bottom on the
  // next frame (once the browser has rendered the new block and grown
  // streamB.scrollHeight). Without this, a user's Enter on a long history can
  // leave streamB not at the bottom, which the scroll listener misreads as an
  // intentional scroll-up and spuriously triggers the auto-split (FEATURE-416).
  requestAnimationFrame(scrollStream);
}

/* FEATURE-469: dynamic-context markers delimiting the auto-appended
   attachment info (and future dynamic perception tags) inside a user message.
   Stored text keeps the markers so the model sees a readable trailing block;
   rendering splits it into the main text + styled tag chips. */
const DYNAMIC_OPEN = "\n<<<DYNAMIC>>>\n";
const DYNAMIC_END = "<<<END_DYNAMIC>>>";

// renderUserBody renders a user message body: plain main text on top and, when
// a trailing <<<DYNAMIC>>> block is present, small attachment tag chips below.
function renderUserBody(body, text) {
  let main = text == null ? "" : String(text);
  let dyn = "";
  const i0 = main.indexOf(DYNAMIC_OPEN);
  const i1 = i0 > -1 ? main.indexOf(DYNAMIC_END, i0) : -1;
  if (i0 > -1 && i1 > -1) {
    dyn = main.slice(i0 + DYNAMIC_OPEN.length, i1);
    main = main.slice(0, i0);
  }
  body.textContent = main;
  if (!dyn) return;
  const tag = document.createElement("div");
  tag.className = "user-dyn";
  for (const line of dyn.split("\n")) {
    if (!line.trim()) continue;
    const parts = line.split("\t");
    const chip = document.createElement("span");
    chip.className = "user-dyn-chip";
    const kind = parts[1];
    chip.textContent = (kind === "image" ? "🖼 " : "📄 ") + (parts[0] || line) + (parts[2] ? " · " + parts[2] : "");
    chip.title = line;
    tag.appendChild(chip);
  }
  body.appendChild(tag);
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

// FEATURE-455: remote access + download flags from bootstrap. When the UI is
// served to a remote address (remote=true) and the operator enabled download
// (downloadEnabled=true), the workspace file list's "reveal in folder" icon
// becomes a download icon (and the folder reveal action disappears).
let remoteAccess = false;
let downloadEnabled = false;

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
  // FEATURE-436: token rates use thousands separators (e.g. 1,234t/s).
  sbLast.innerHTML = T.sbLast + " ↑" + fmtNum(li) + "（" + (liTPS > 0 ? fmtNum(liTPS) + "t/s" : "-") + ", " + liDur + ") ↓" + fmtNum(lo) + " (" + (loTPS > 0 ? fmtNum(loTPS) + "t/s" : "-") + ", " + loDur + ")";
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
    // Message count, right-aligned (FEATURE-428).
    const count = document.createElement("span");
    count.className = "session-count";
    count.textContent = s.message_count != null ? s.message_count : 0;
    count.title = T.sessionCount;
    row.appendChild(count);
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

// initStreamMode wires the three-segment display-mode pill (reusing the
// .mode-seg control with its gliding slider) and the editable session title
// in the main message area title bar.
function initStreamMode() {
  const slider = document.getElementById("streamModeSlider");
  const moveSlider = () => {
    const items = streamMode.querySelectorAll(".mode-seg-item");
    let idx = 0;
    items.forEach((b, i) => { if (b.dataset.mode === displayMode) idx = i; });
    const seg = items[idx];
    if (!seg || !slider) return;
    slider.style.width = seg.offsetWidth + "px";
    slider.style.transform = "translateX(" + seg.offsetLeft + "px)";
  };
  // Display-mode pill.
  streamMode.querySelectorAll(".mode-seg-item").forEach((btn) => {
    btn.textContent = T["streamMode" + btn.dataset.mode[0].toUpperCase() + btn.dataset.mode.slice(1)] || btn.dataset.mode;
    btn.classList.toggle("active", btn.dataset.mode === displayMode);
    btn.onclick = () => {
      if (btn.dataset.mode === displayMode) return;
      displayMode = btn.dataset.mode;
      localStorage.setItem("co-shell-display-mode", displayMode);
      streamMode.querySelectorAll(".mode-seg-item").forEach((b) => b.classList.toggle("active", b.dataset.mode === displayMode));
      moveSlider();
      applyDisplayMode();
    };
  });
  moveSlider();
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
    // FEATURE-478: ignore Enter while an IME is composing (candidate confirm).
    if (e.key === "Enter" && !imeComposing(e)) { e.preventDefault(); streamTitle.blur(); }
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
      // The final result block must be fully expanded (FIX-426).
      if (box.classList.contains("ev-result")) box.classList.remove("collapsed");
    } else if (displayMode === "minimal") {
      box.style.display = "";
      // User-msg blocks are always expanded (FIX-426).
      if (cls === "user-msg") { box.classList.remove("collapsed"); return; }
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

// FEATURE-429: model delete confirmation modal (mirrors the session delete
// interaction). The model_remove message is only sent after the user confirms.
let pendingModelDeleteID = null;
function confirmDeleteModel(m) {
  pendingModelDeleteID = m.id;
  delModelMsg.textContent = (T.modelDeleteConfirm || "确认删除模型「%s」吗？此操作不可撤销。").replace("%s", m.id);
  delModelModal.classList.remove("hidden");
}
function closeModelDeleteModal() {
  pendingModelDeleteID = null;
  delModelModal.classList.add("hidden");
}
delModelClose.onclick = closeModelDeleteModal;
delModelCancel.onclick = closeModelDeleteModal;
delModelModal.onclick = (e) => { if (e.target === delModelModal) closeModelDeleteModal(); };
delModelConfirm.onclick = () => {
  if (pendingModelDeleteID) wsSend({ type: "model_remove", value: pendingModelDeleteID });
  closeModelDeleteModal();
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
  input.placeholder = T.inputHint;
  askArea.classList.add("hidden");
  askInteraction.classList.add("hidden");
  askInteraction.textContent = "";
  // Remove the virtual-keyboard physical-key listener.
  if (window.__vkHandler) {
    window.removeEventListener("keydown", window.__vkHandler);
    window.__vkHandler = null;
  }
  if (window.__vkKeyup) {
    window.removeEventListener("keyup", window.__vkKeyup);
    window.__vkKeyup = null;
  }
}

/* ---------- structured interaction (FEATURE-388) ---------- */

let pendingInteraction = null;
let supplementMode = false; // true while the user is typing supplementary info

// enterSupplementMode switches to supplement-input mode: the user types in the
// main input box and the key handler stops hijacking keys (FEATURE-388).
function enterSupplementMode() {
  supplementMode = true;
  input.focus();
  input.placeholder = T.supplementHint;
}

// longPressSupplement fills the given content into the main input box and
// enters supplement mode, so the user can append extra info before sending.
// It is the mouse equivalent of the physical-key long-press (FEATURE-462).
function longPressSupplement(content) {
  input.value = content || "";
  autoGrow();
  supplementMode = true;
  input.focus();
  input.placeholder = T.supplementHint;
}

// fillInputAndExit fills the main input box with the given prefix and closes the
// interaction dialog while keeping the interaction pending, so the user's typed
// supplement is sent back as the interaction answer (FEATURE-459).
function fillInputAndExit(prefix) {
  input.value = prefix;
  autoGrow();
  supplementMode = true;
  input.focus();
  input.placeholder = T.supplementHint;
  askArea.classList.add("hidden");
  askInteraction.classList.add("hidden");
  askInteraction.textContent = "";
  if (window.__vkHandler) {
    window.removeEventListener("keydown", window.__vkHandler);
    window.__vkHandler = null;
  }
  if (window.__vkKeyup) {
    window.removeEventListener("keyup", window.__vkKeyup);
    window.__vkKeyup = null;
  }
}


// splitReportSections splits a report body into distinct sections by the
// known section markers (【任务完成报告】 / 【监督 LLM 审查】). When two or
// more markers are present, each section is returned separately so the UI can
// render them as independent scrollable blocks (FEATURE-459). Each section is
// {title, content}: the marker is extracted as a bold title (without the
// brackets) and the remaining text is the scrollable content (FEATURE-460).
function splitReportSections(body) {
  const markers = [
    { marker: "【任务完成报告】", title: "任务完成报告" },
    { marker: "【监督 LLM 审查】", title: "审查报告" },
  ];
  const found = markers.filter((m) => body.indexOf(m.marker) >= 0);
  if (found.length < 2) return [{ title: "", content: body }];
  // Split on the first marker, then on the second marker.
  const first = found[0];
  const second = found[1];
  const i1 = body.indexOf(first.marker);
  const i2 = body.indexOf(second.marker);
  const a = body.slice(i1, i2).replace(first.marker, "").trim();
  const b = body.slice(i2).replace(second.marker, "").trim();
  const out = [];
  if (a) out.push({ title: first.title, content: a });
  if (b) out.push({ title: second.title, content: b });
  return out.length ? out : [{ title: "", content: body }];
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
    // FEATURE-459: the attempt_completion dialog body may carry two distinct
    // reports — the main LLM's final report (【任务完成报告】) and the
    // supervisor's review (【监督 LLM 审查】). Render each as its own
    // scrollable block (max-height 40% of the ask box) so long reports stay
    // readable and the two are clearly separated.
    const sections = splitReportSections(it.body);
    if (sections.length > 1) {
      sections.forEach((sec) => {
        if (sec.title) {
          const h = document.createElement("div");
          h.className = "report-title";
          h.textContent = sec.title;
          askInteraction.appendChild(h);
        }
        const b = document.createElement("div");
        b.className = "interaction-body md report-block";
        mdRender(b, sec.content);
        askInteraction.appendChild(b);
      });
    } else {
      const b = document.createElement("div");
      b.className = "interaction-body md";
      // FEATURE-409: render the prompt body as markdown so lists, code and
      // emphasis are laid out instead of piling up as one text blob.
      mdRender(b, it.body);
      askInteraction.appendChild(b);
    }
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

  // FEATURE-462: the dedicated supplement input box is removed. Supplement is
  // typed in the main input box (clicking it cancels shortcut-key monitoring);
  // a hint line below the option buttons tells the user they can long-press to
  // supplement.

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
      // FEATURE-478: ignore Enter while an IME is composing (candidate confirm).
      if (e.key === "Enter" && !imeComposing(e)) { e.preventDefault(); answerInteraction({ action: "input", value: inp.value }); }
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
  // A tool-confirmation interaction carries the full action set; a
  // cancel/resume interaction (ESC pause) only has approve+cancel, so it
  // shows just "-" (cancel) and Enter (resume) (FEATURE-427).
  const isToolConfirm = (it.keys || []).some((k) =>
    k.value === "approve_all" || k.value === "approve_g" || k.value === "approve_d");
  // Build a map: key -> {action, value}.
  const keyMap = {};
  if (isSelect && it.options && it.options.length) {
    // Number keys select options (1..N).
    it.options.forEach((opt, i) => {
      keyMap[String(i + 1)] = { action: "select", value: opt };
    });
    // FIX-454: register the interaction's fixed key options (e.g. "+" / "-")
    // so pressing the physical key triggers the corresponding select action.
    (it.keys || []).forEach((k) => {
      if (k.key) keyMap[k.key.toLowerCase()] = { action: "select", value: k.value, label: k.label };
    });
  } else {
    // FEATURE-427: symbol/numpad keys only (input-method independent). The
    // backend's letter keys (a/g/d/c) are intentionally ignored so an active
    // IME cannot swallow the shortcut.
    if (isToolConfirm) {
      keyMap["+"] = { action: "approve_all" };
      keyMap["*"] = { action: "approve_g" };
      keyMap["/"] = { action: "approve_d" };
      keyMap["5"] = { action: "approve_count", value: "5" };
    }
    keyMap["-"] = { action: "cancel" };
  }
  // Enter maps to approve.
  keyMap["enter"] = { action: "approve" };

  // Option items: each is a virtual-keyboard-style square key (showing only the
  // letter / key name) with its label written beside it (FEATURE-388). Number
  // keys are merged into one [1]-[9] item. A [Space] item enters supplement mode.
  const wrap = document.createElement("div");
  wrap.className = "option-buttons";
  // Helper to build one option item: key button + label beside it.
  // FEATURE-462: onLongPress (optional) is the content to fill into the main
  // input box when the button is mouse-held >=500ms, entering supplement mode
  // (equivalent to the physical-key long-press).
  const addItem = (keyText, labelText, onClick, extraCls, onLongPress) => {
    const item = document.createElement("div");
    item.className = "opt-item" + (extraCls ? " " + extraCls : "");
    const b = document.createElement("button");
    b.className = "opt-key-btn";
    b.textContent = keyText;
    b.onclick = onClick;
    if (onLongPress) {
      let pressTimer = null;
      let longPressed = false;
      b.addEventListener("mousedown", (e) => {
        e.preventDefault();
        longPressed = false;
        pressTimer = setTimeout(() => {
          longPressed = true;
          onLongPress();
        }, 500);
      });
      b.addEventListener("mouseup", () => {
        if (pressTimer) { clearTimeout(pressTimer); pressTimer = null; }
      });
      b.addEventListener("mouseleave", () => {
        if (pressTimer) { clearTimeout(pressTimer); pressTimer = null; }
      });
      b.addEventListener("click", (e) => {
        if (longPressed) { e.stopPropagation(); e.preventDefault(); longPressed = false; }
      });
    }
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
      addItem(String(i + 1), opt, () => answerSelectWithSupplement(opt), "", () => longPressSupplement(opt));
    });
    // FIX-454: render the interaction's fixed key options (e.g. attempt_completion's
    // "+ 任务尚未达到目标" / "- 完成退出") as [Key] Label buttons. Clicking one
    // sends {action:"select", value:k.Value} back to the backend, matching the
    // TUI askSelect behaviour.
    (it.keys || []).forEach((k) => {
      addItem(k.key, k.label, () => {
        // FEATURE-459: the "+ 任务尚未达到目标" key fills the prefix into the
        // main input box and exits the dialog so the user can append info.
        if (k.value === "not_done") {
          fillInputAndExit(k.label + "：");
          return;
        }
        // FEATURE-460: "我已确认完成（退出）" directly exits without sending.
        if (k.value === "exit") {
          hideAsk();
          return;
        }
        // FEATURE-460: append the typed supplement for other fixed keys.
        answerSelectWithSupplement(k.value);
      }, "", () => longPressSupplement(k.label));
    });
    // FEATURE-438: a fixed supplementary-info option for select interactions
    // (ask_followup_question). Clicking it (or pressing Space/Insert/0) enters
    // supplement-input mode so the user can type extra info in the main box.
    addItem("空格/Ins/0", T.supplement, () => sendSupplement(), "opt-space");
  } else {
    // FEATURE-427: symbol/numpad action keys (skip enter, handled separately).
    Object.keys(keyMap).forEach((key) => {
      if (key === "enter") return;
      const m = keyMap[key];
      addItem(key.toUpperCase(), legendLabel(m), () => answerInteraction(m));
    });
    // Enter item.
    addItem("回车", T.approve, () => answerInteraction({ action: "approve" }));
  }
  // Space / Insert / 0 item: enter supplement-input mode (FEATURE-427).
  // Shown for tool confirmation and select interactions (FEATURE-438); a
  // cancel/resume prompt has no supplement.
  if (isToolConfirm) {
    addItem("空格/Ins/0", T.supplement, () => enterSupplementMode(), "opt-space");
  }
  target.appendChild(wrap);
  // FEATURE-462: a hint line below the option buttons telling the user they
  // can long-press a button (or click the main input box) to supplement.
  if (isSelect || isToolConfirm) {
    const hint = document.createElement("div");
    hint.className = "interaction-supplement-hint";
    hint.textContent = T.supplementHint;
    target.appendChild(hint);
  }

  // Listen for physical key presses while this interaction is pending.
  // FEATURE-459: holding a shortcut key (>=500ms) selects the option AND fills
  // its content into the main input box so the user can append supplementary
  // info before sending.
  let holdTimer = null;
  let holdKey = null;
  const clearHold = () => { if (holdTimer) { clearTimeout(holdTimer); holdTimer = null; } holdKey = null; };
  window.__vkHandler = (e) => {
    if (!pendingInteraction) return;
    // In supplement mode, stop hijacking keys so the user can type freely.
    if (supplementMode) return;
    // FEATURE-419: swallow every key while collecting a shortcut, whether or
    // not it triggers an action, so no stray character leaks into the input
    // box (previously only digits/letters/space/enter were swallowed).
    e.preventDefault();
    const key = e.key.toLowerCase();
    // FEATURE-427: supplement via Space / Insert / 0 (input-method independent),
    // for tool confirmation and select interactions (FEATURE-438); a
    // cancel/resume prompt has no supplement.
    if ((isToolConfirm || isSelect) && (key === " " || key === "insert" || key === "0")) {
      enterSupplementMode();
    } else if (key === "enter") {
      answerInteraction({ action: "approve" });
    } else if (keyMap[key]) {
      const m = keyMap[key];
      // Long-press: fill the option content into the main input box and enter
      // supplement mode so the user can append extra info (FEATURE-459).
      if (e.repeat) {
        // A held key fires repeated keydown events; on the first repeat, fill
        // the input and switch to supplement mode instead of answering.
        if (holdKey === key) {
          clearHold();
          // FEATURE-459: the "+" (not_done) key fills its label prefix.
          input.value = m.value === "not_done" ? (m.label || "") + "：" : (m.value || "");
          autoGrow();
          enterSupplementMode();
        }
        return;
      }
      // First keydown: arm a hold timer. If it fires (key held >=500ms), the
      // next repeat will fill the input; otherwise keyup answers normally.
      holdKey = key;
      holdTimer = setTimeout(() => { holdTimer = null; }, 500);
    }
  };
  window.__vkKeyup = (e) => {
    if (!pendingInteraction || supplementMode) return;
    const key = e.key.toLowerCase();
    if (keyMap[key] && holdKey === key) {
      // Released before the hold threshold → normal select.
      clearHold();
      const m = keyMap[key];
      // FEATURE-459: the "+" (not_done) key fills the prefix into the main
      // input box and exits the dialog so the user can append info.
      if (m.value === "not_done") {
        fillInputAndExit((m.label || "") + "：");
        return;
      }
      answerInteraction(m);
    }
  };
  window.addEventListener("keydown", window.__vkHandler);
  window.addEventListener("keyup", window.__vkKeyup);
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
  // FEATURE-478: ignore Enter while an IME is composing (candidate confirm).
  if (e.key === "Enter" && !imeComposing(e)) { e.preventDefault(); answerAsk(askInput.value); }
});

/* ---------- FEATURE-469: message attachments (clipboard paste / pick) ---------- */

const attachBar = document.getElementById("attachBar");
const attachList = document.getElementById("attachList");
const attachClear = document.getElementById("attachClear");
const attachFile = document.getElementById("attachFile");
const attachBtn = document.getElementById("attachBtn");

// Effective upload dir: "web-input-dir" setting item from settings_get
// (workspace-relative, default "input").
let webInputDir = "input";
let pendingAttach = []; // {uid,file,name,size,kind,url}
let attachSeq = 0;
let attachSending = false;

function cacheWebInputDir(groups) {
  for (const g of groups || []) {
    for (const it of g.items || []) {
      if (it.key === "web-input-dir") {
        webInputDir = it.value || "input";
        return;
      }
    }
  }
}

function fmtBytes(b) {
  if (!b && b !== 0) return "";
  if (b < 1024) return b + " B";
  if (b < 1048576) return (b / 1024).toFixed(1) + " KB";
  if (b < 1073741824) return (b / 1048576).toFixed(1) + " MB";
  return (b / 1073741824).toFixed(2) + " GB";
}

function attachKindOf(file) {
  return (file.type || "").startsWith("image/") ? "image" : "file";
}

function addAttachFiles(files) {
  if (!files || !files.length) return;
  for (const f of Array.from(files)) {
    const kind = attachKindOf(f);
    let name = f.name || (kind === "image" ? "image.png" : "file");
    // Clipboard screenshots usually reuse one generic name (image.png); give
    // them a unique storage name so repeated pastes never collide on disk.
    if (kind === "image" && /^image([.][a-z0-9]+)?$/i.test(name || "image.png")) {
      const ext = ((name || "png").split(".").pop() || "png").toLowerCase();
      const d = new Date();
      const p2 = (n) => String(n).padStart(2, "0");
      name = "clip-" + d.getFullYear() + p2(d.getMonth() + 1) + p2(d.getDate()) + "-" +
        p2(d.getHours()) + p2(d.getMinutes()) + p2(d.getSeconds()) + "-" + (attachSeq++) + "." + ext;
    }
    pendingAttach.push({
      uid: "a" + (attachSeq++), file: f, name, size: f.size || 0,
      kind, url: URL.createObjectURL(f),
    });
  }
  renderAttachBar();
}

function renderAttachBar() {
  attachList.textContent = "";
  const has = pendingAttach.length > 0;
  attachBar.classList.toggle("hidden", !has);
  if (!has) return;
  for (const it of pendingAttach) renderAttachItem(it);
}

function renderAttachItem(it) {
  const item = document.createElement("div");
  item.className = "attach-item";
  item.title = it.name + " · " + (it.kind === "image" ? T.attachKindImage : T.attachKindFile) + " " + fmtBytes(it.size);
  const rm = document.createElement("button");
  rm.type = "button"; rm.className = "attach-rm"; rm.textContent = "✕";
  rm.title = T.attachRemove;
  rm.onclick = (e) => { e.stopPropagation(); removeAttach(it.uid); };
  if (it.kind === "image") {
    const img = document.createElement("img");
    img.className = "attach-thumb"; img.src = it.url; img.alt = it.name;
    item.appendChild(img);
  } else {
    const wrap = document.createElement("div");
    wrap.className = "attach-file";
    const ico = document.createElement("div"); ico.className = "af-icon"; ico.textContent = "📄";
    const nm = document.createElement("div"); nm.className = "af-name"; nm.textContent = it.name;
    wrap.appendChild(ico); wrap.appendChild(nm);
    item.appendChild(wrap);
  }
  item.appendChild(rm);
  item.onclick = () => openAttachPreview(it);
  attachList.appendChild(item);
}

function removeAttach(uid) {
  const idx = pendingAttach.findIndex((p) => p.uid === uid);
  if (idx < 0) return;
  URL.revokeObjectURL(pendingAttach[idx].url);
  pendingAttach.splice(idx, 1);
  renderAttachBar();
}

function clearAttachAll() {
  for (const p of pendingAttach) URL.revokeObjectURL(p.url);
  pendingAttach = [];
  renderAttachBar();
}

function openAttachPreview(it) {
  const pv = document.getElementById("preview");
  const pvName = document.getElementById("pvName");
  const pvSize = document.getElementById("pvSize");
  const pvMtime = document.getElementById("pvMtime");
  const pvRes = document.getElementById("pvRes");
  const pvImg = document.getElementById("previewImg");
  pvName.textContent = it.name;
  pvSize.textContent = fmtBytes(it.size);
  pvMtime.textContent = "";
  if (it.kind === "image") {
    pvRes.textContent = "";
    pvImg.classList.remove("hidden");
    pvImg.onload = null;
    pvImg.src = it.url;
    pvImg.onload = () => { pvRes.textContent = pvImg.naturalWidth + " × " + pvImg.naturalHeight; };
  } else {
    pvImg.classList.add("hidden");
    pvRes.textContent = T.attachKindFile + " · " + T.attachPreviewFile;
  }
  pv.classList.remove("hidden");
}

attachBtn.onclick = () => attachFile.click();
attachFile.addEventListener("change", () => {
  addAttachFiles(attachFile.files);
  attachFile.value = "";
});
attachClear.onclick = clearAttachAll;

// Ctrl+V in the input box: when the clipboard carries image content, turn it
// into a pending thumbnail. If text is also present we keep the default paste
// AND add the image(s) so nothing is lost.
input.addEventListener("paste", (e) => {
  const items = (e.clipboardData && e.clipboardData.items) || [];
  const imgs = [];
  for (const it of items) {
    if (it.kind === "file" && it.type && it.type.startsWith("image/") && it.getAsFile) {
      const f = it.getAsFile();
      if (f) imgs.push(f);
    }
  }
  if (!imgs.length) return;
  addAttachFiles(imgs);
  const hasText = Array.from(items).some((it) => it.kind === "string");
  if (!hasText) e.preventDefault();
});

// Dropping files straight onto the input box attaches them too.
function hasDroppedFiles(e) {
  return !!(e.dataTransfer && e.dataTransfer.types &&
    Array.from(e.dataTransfer.types).includes("Files"));
}
input.addEventListener("dragover", (e) => { if (hasDroppedFiles(e)) e.preventDefault(); });
input.addEventListener("drop", (e) => {
  if (!e.dataTransfer || !e.dataTransfer.files || !e.dataTransfer.files.length) return;
  e.preventDefault();
  addAttachFiles(e.dataTransfer.files);
});


// Uploads every pending attachment into web-input-dir, then sends the message:
// image files go through the existing attachments (vision) channel and every
// uploaded file is listed in the trailing dynamic tag block. Nothing is sent
// (and items are kept for retry) when the upload fails.
async function uploadAndSend(text) {
  if (attachSending) return false;
  attachSending = true;
  try {
    const fd = new FormData();
    for (const it of pendingAttach) fd.append("file", it.file, it.name);
    const resp = await fetch("/api/upload?dir=" + encodeURIComponent(webInputDir || "input"), {
      method: "POST", body: fd,
    });
    const body = resp.ok ? await resp.json().catch(() => ({})) : {};
    if (!resp.ok || !Array.isArray(body.paths) || body.paths.length < pendingAttach.length) {
      alert(T.attachFail);
      return false;
    }
    const entries = [];
    const imageAtt = [];
    pendingAttach.forEach((it, i) => {
      const rel = body.paths[i];
      entries.push({ path: rel, kind: it.kind, sizeBytes: it.size });
      if (it.kind === "image") imageAtt.push(rel);
    });
    loadTree();
    // FEATURE-471: report each uploaded file as a dynamic event (clip_object
    // for images, upload_file for others) instead of appending a <<<DYNAMIC>>>
    // text block. The backend drains these into <user_dynamic_events> on the
    // next user/tool message injection.
    for (const en of entries) {
      wsSend({ type: "dynamic_event", kind: en.kind === "image" ? "clip_object" : "upload_file", value: en.path });
    }
    clearAttachAll();
    wsSend({ type: "input", text, attachments: imageAtt.length ? imageAtt : undefined });
    renderUserEcho(text);
    history.push(text);
    histPos = history.length;
    input.value = "";
    autoGrow();
    if (wsReady) setRunning(true);
    return true;
  } finally {
    attachSending = false;
  }
}

/* ---------- input row / history ---------- */

const history = [];
let histPos = 0; // sentinel: histPos === history.length means "at the unsent draft"
let histDraft = "";

function sendInput() {
  const text = input.value.trim();
  const hasAttach = pendingAttach.length > 0;
  if (!text && !hasAttach) return;
  // When an interaction is pending, the main input box sends supplementary
  // instructions instead of a new message (FEATURE-388). Attachments are not
  // part of that answer protocol, so they stay in the tray until the
  // interaction is resolved.
  if (pendingInteraction) {
    if (hasAttach) { console.warn("attachments pending while an interaction is open"); return; }
    answerInteraction({ action: "input", value: text });
    input.value = "";
    autoGrow();
    return;
  }
  // FEATURE-449: submitting a fresh command starts a new task, so clear the
  // previous tool call's affected-object highlight (background and font).
  // Answering a question or typing supplementary info (the branch above) does
  // NOT clear it.
  clearAffectedHighlight();
  if (hasAttach) { uploadAndSend(text); return; }
  // FEATURE-471: while a task is running, typing in the main input box and
  // sending reports a user_message dynamic event (buffered for the next tool
  // message) instead of interrupting the running task with a new message.
  if (running) {
    wsSend({ type: "dynamic_event", kind: "user_message", value: text });
    renderUserEcho(text);
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

// backfillInput prepends unconsumed user_message texts (joined by a blank
// line) to the top of the input box so the user can review and re-submit them
// after a task ended (FEATURE-471).
function backfillInput(msgs) {
  if (!msgs || !msgs.length) return;
  const joined = msgs.join("\n\n");
  input.value = input.value ? joined + "\n\n" + input.value : joined;
  autoGrow();
  input.focus();
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
  // FEATURE-425/FIX-426: while a task is running, the session-title highlight
  // dot and the co-shell logo breathe (pulse) to signal activity; they stop
  // when the task completes.
  const active = document.getElementById("streamActive");
  const logo = document.getElementById("logo");
  if (active) active.classList.toggle("breathing", v);
  if (logo) logo.classList.toggle("breathing", v);
}

sendBtn.onclick = () => { if (running) wsSend({ type: "interrupt" }); else sendInput(); };

/* ---------- YOLO master switch (FEATURE-439) ---------- */

// setYOLO flips the classic up/down toggle to reflect the YOLO mode state.
// When on, the thumb moves up and the orange-red ON label is revealed.
function setYOLO(on) {
  if (!yoloSwitch) return;
  yoloSwitch.classList.toggle("on", on);
  yoloSwitch.setAttribute("aria-pressed", on ? "true" : "false");
}

// Clicking the toggle sends the new state to the backend; the backend replies
// with a yolo message that confirms the applied state.
if (yoloSwitch) {
  yoloSwitch.onclick = () => {
    const next = !yoloSwitch.classList.contains("on");
    wsSend({ type: "yolo_set", yolo: next });
  };
}

// FEATURE-445: as the user scrolls, recompute whether to follow output. Within
// 100px of the bottom we follow (auto-scroll to the newest line); beyond that we
// stop following so the user can read history, and the floating down-arrow
// button appears to jump back to the bottom.
streamB.addEventListener("scroll", updateFollowState);

// FEATURE-445: clicking the floating down-arrow jumps to the bottom and resumes
// following output.
if (scrollDownBtn) scrollDownBtn.addEventListener("click", jumpToBottom);

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
  // FEATURE-478: while an IME is composing, Enter confirms a candidate word
  // (e.g. Chinese pinyin), not a send — ignore it until composition ends.
  if (e.key === "Enter" && !e.shiftKey && !imeComposing(e)) { e.preventDefault(); sendInput(); return; }
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
// FEATURE-462: clicking the main input box while an interaction is pending
// cancels shortcut-key monitoring so the user can type supplementary info
// freely (the dedicated supplement box was removed). A click (not focus) is
// used so auto-focus / Tab-focus when the dialog appears does NOT disable the
// shortcuts - only an explicit mouse click does (FIX-462).
input.addEventListener("click", () => {
  if (pendingInteraction) {
    supplementMode = true;
    input.placeholder = T.supplementHint;
  }
});

/* ---------- workspace tree ---------- */

// Paths of directories the user has expanded. Kept across loadTree() calls
// so a refresh (manual, after upload, or on done) preserves each folder's
// open/collapsed state instead of collapsing everything.
const expandedDirs = new Set();

// FEATURE-447: files/folders affected by the current tool call, reported by
// the LLM via the "files" argument. The frontend highlights them in the tree
// with a blue text colour (no background/border).
let affectedFiles = [];
// FEATURE-449: the previous tool call's affected files, demoted to font-only
// highlight (aff-pred-fg) when a new tool call starts. Kept separately because
// loadTree() rebuilds the DOM and would otherwise drop the demoted highlight.
let prevAffectedFiles = [];

// highlightAffectedFiles records the files affected by the current tool call,
// demotes the previous tool call's highlight to font-only (keeping the blue
// text but clearing the background), expands the affected files' ancestor
// folders (including the workspace root), rebuilds the tree and scrolls the
// first affected file into view (FEATURE-447/449).
function highlightAffectedFiles(files) {
  // FEATURE-449: remember the current affected files as the previous ones so
  // they can be re-applied as font-only after the tree rebuild (loadTree()
  // destroys the DOM, dropping any in-place class change).
  prevAffectedFiles = affectedFiles;
  affectedFiles = files || [];
  if (!affectedFiles.length) return;
  // Expand every affected file's ancestor folders (including the workspace
  // root "") so the highlighted row is visible even when its directory chain
  // was collapsed. The root must be expanded too, otherwise the affected
  // file's top-level folder sits inside a collapsed root and stays hidden.
  affectedFiles.forEach((f) => {
    const parts = f.path.split("/");
    let acc = "";
    for (let i = 0; i < parts.length - 1; i++) {
      acc = acc ? acc + "/" + parts[i] : parts[i];
      expandedDirs.add(acc);
    }
    // The workspace root ("") is the first directory in the file list; expand
    // it so the affected file's top-level folder is reachable (FEATURE-449).
    expandedDirs.add("");
  });
  // Rebuild the tree to apply the expansion, then highlight and scroll.
  loadTree().then(() => {
    applyAffectedHighlight();
    scrollToAffected();
  });
}

// applyAffectedHighlight applies the affected-file highlight to the current
// workspace tree DOM. It is called after highlightAffectedFiles and after each
// loadTree() rebuild (which recreates the DOM). The current call's files get
// the full highlight (aff-pred); the previous call's files get the font-only
// highlight (aff-pred-fg) so the user can still see which files were touched
// (FEATURE-449).
function applyAffectedHighlight() {
  tree.querySelectorAll(".tree-row").forEach((r) => {
    const path = r.dataset.path;
    // FEATURE-449: the workspace root row carries an empty path (""), while the
    // backend reports the root as ".". Normalise both to match so the root is
    // highlighted too when it is the affected object.
    const norm = path === "" ? "." : path;
    const nameEl = r.querySelector(".name");
    if (!nameEl) return;
    // Previous call's files: font-only highlight (keep blue text, no background).
    if (prevAffectedFiles.some((f) => f.path === norm)) {
      nameEl.classList.add("aff-pred-fg");
    }
    // Current call's files: full highlight (blue text + background).
    if (affectedFiles.some((f) => f.path === norm)) {
      nameEl.classList.add("aff-pred");
    }
  });
}

// clearAffectedHighlight fully clears the affected-object highlight (both the
// background and the font colour) and forgets the current affected files. It is
// called when the user submits a fresh command in the input box — a new task
// starts, so the previous tool call's highlight is no longer relevant
// (FEATURE-449). Answering a question or typing supplementary info does NOT
// clear it, so the highlight survives mid-task interactions.
function clearAffectedHighlight() {
  affectedFiles = [];
  prevAffectedFiles = [];
  tree.querySelectorAll(".tree-row .name.aff-pred, .tree-row .name.aff-pred-fg")
    .forEach((n) => n.classList.remove("aff-pred", "aff-pred-fg"));
}

// scrollToAffected scrolls the workspace tree so the first affected file is
// visible (FEATURE-447).
function scrollToAffected() {
  if (!affectedFiles.length) return;
  const first = affectedFiles[0];
  const row = Array.from(tree.querySelectorAll(".tree-row")).find((r) => r.dataset.path === first.path);
  if (row) row.scrollIntoView({ block: "nearest" });
}

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
    // FEATURE-447: re-apply the affected-file highlight after the DOM rebuild.
    applyAffectedHighlight();
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
  // FEATURE-455: when the UI is served to a remote address, the "reveal in
  // folder" action is meaningless (it would open the server's local file
  // manager on the remote host) and is a security concern, so it is always
  // disabled. When download is also enabled, files show a download icon
  // instead; directories show no action at all. Locally the reveal icon stays.
  if (remoteAccess) {
    if (downloadEnabled && !node.dir) {
      const dl = document.createElement("button");
      dl.className = "reveal-btn";
      dl.title = T.downloadFile;
      dl.textContent = "⬇";
      dl.onclick = (e) => { e.stopPropagation(); downloadFile(node.path); };
      actions.appendChild(dl);
    }
  } else {
    const reveal = document.createElement("button");
    reveal.className = "reveal-btn";
    reveal.title = T.revealDir;
    reveal.textContent = "⌖";
    reveal.onclick = (e) => { e.stopPropagation(); postPath("/api/reveal", node.path); };
    actions.appendChild(reveal);
  }
  row.appendChild(actions);

  // FEATURE-383: hovering a long (truncated) file name auto-expands the
  // sidebar to fit it; leaving the sidebar returns it to the fixed width.
  row.addEventListener("mouseenter", () => {
    if (name.scrollWidth > name.clientWidth) layout.classList.add("sidebar-auto");
  });

  // FEATURE-447: every row (file and directory) carries its absolute path so
  // the affected-file highlight can match directories too (predicted files
  // may resolve to a shared folder).
  row.dataset.path = node.path;

  li.appendChild(row);

  if (node.dir) {
    const ul = document.createElement("ul");
    const open = expandedDirs.has(node.path);
    ul.style.display = open ? "" : "none";
    tw.textContent = open ? "▾" : "▸";
    // FEATURE-477: the .open class lets the light theme draw an "open folder"
    // icon for expanded directories (vs a closed folder when collapsed).
    tw.classList.toggle("open", open);
    for (const c of node.children || []) ul.appendChild(treeNode(c));
    li.appendChild(ul);
    row.onclick = () => {
      const isOpen = ul.style.display !== "none";
      ul.style.display = isOpen ? "none" : "";
      tw.textContent = isOpen ? "▸" : "▾";
      tw.classList.toggle("open", !isOpen);
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
    // FEATURE-425/444: single-click previews the file in-page (text files in
    // the viewer, images in the popup); double-click opens it with the system
    // handler (openFile). The two are distinguished so previewing never
    // accidentally launches the OS app.
    row.dataset.path = node.path; // for highlighting the selected file
    row.onclick = () => { if (IMAGE_EXT.test(node.name)) openImagePreview(node); else openFilePreview(node); };
    row.ondblclick = () => openFile(node);
  }
  return li;
}

const IMAGE_EXT = /\.(png|jpe?g|gif|webp|bmp|svg)$/i;

// FEATURE-425: text file extensions that can be previewed in-page (source
// code, docs, config, data, shell scripts, etc.).
const TEXT_EXT = /\.(go|txt|md|markdown|csv|tsv|sh|bash|zsh|conf|cfg|ini|json|ya?ml|xml|py|js|mjs|cjs|ts|jsx|tsx|html?|css|scss|less|sql|java|c|h|cpp|hpp|rs|rb|php|vue|svelte|toml|env|gitignore|dockerfile|makefile|log|properties|gradle|lock|sum|mod)$/i;

// openFile opens a file with the OS default handler (FEATURE-444: images are
// opened by the system on double-click, not previewed in-page).
function openFile(node) {
  postPath("/api/open", node.path);
  // FEATURE-471: report the opened file as a dynamic event so the LLM can
  // notice it during a running task.
  wsSend({ type: "dynamic_event", kind: "open_file", value: node.path });
}

// openImagePreview opens an image in the popup and fills its title bar with
// the file name, size, modification time and resolution (FEATURE-444). The
// resolution is read from the loaded image's natural dimensions.
function openImagePreview(node) {
  pvName.textContent = node.name;
  pvSize.textContent = formatSize(node.size);
  pvMtime.textContent = formatMtime(node.mtime);
  pvRes.textContent = "";
  previewImg.onload = () => {
    pvRes.textContent = previewImg.naturalWidth + " × " + previewImg.naturalHeight;
  };
  previewImg.src = "/api/file?path=" + encodeURIComponent(node.path);
  preview.classList.remove("hidden");
}

/* ---------- text file previewer (FEATURE-425) ---------- */

// Current preview state: the open file path (workspace-relative) and the next
// line to load. Re-clicking the same file is a no-op (UC-004).
let fvPath = null;
let fvNextLine = 1;
let fvTotal = 0;
let fvLoading = false;
let fvDiff = new Map(); // lineNo -> "add" | "del"
let fvMdText = ""; // accumulated md content for auto-render
let fvHexMode = false; // binary file shown as hex dump
let fvHexNext = 0; // next byte offset to load in hex mode
let fvHexWidth = 16; // bytes per hex row (8/16/32/64/128), auto-fit to width
let fvRaw = false; // FEATURE-435: md files show raw text when true (default off = auto-render)

// openFilePreview opens a file in the in-page viewer. Clicking a new file
// immediately discards the current one (UC-003); clicking the current file is
// a no-op (UC-004). Known text extensions preview directly; unknown extensions
// are also attempted as text and fall back to HEX view if control characters
// are found. Image files keep the system open.

// formatMtime formats a unix-seconds timestamp as a readable local time.
function formatMtime(sec) {
  if (!sec) return "";
  const d = new Date(sec * 1000);
  const pad = (n) => String(n).padStart(2, "0");
  return d.getFullYear() + "-" + pad(d.getMonth() + 1) + "-" + pad(d.getDate()) +
    " " + pad(d.getHours()) + ":" + pad(d.getMinutes()) + ":" + pad(d.getSeconds());
}

// formatSize formats a byte count with thousands separators, e.g. 1234567 ->
// "1,234,567B" (FEATURE-444).
function formatSize(bytes) {
  if (!bytes) return "";
  return bytes.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ",") + "B";
}

function openFilePreview(node) {
  if (IMAGE_EXT.test(node.name)) return;
  // FEATURE-425: clicking the already-open file closes the preview.
  if (fvPath === node.path) {
    closeFileViewer();
    return;
  }
  // FEATURE-471: report the single-click preview as a view_file dynamic event
  // so the LLM can notice which files the user is inspecting.
  wsSend({ type: "dynamic_event", kind: "view_file", value: node.path });
  fvPath = node.path;
  fvNextLine = 1;
  fvTotal = 0;
  fvDiff = new Map();
  fvHexMode = false;
  // FEATURE-436: keep the Raw preference across file switches (localStorage).
  fvRaw = localStorage.getItem("fvRaw") === "1";
  fvBody.textContent = "";
  fvMdText = "";
  fvBody.classList.remove("md");
  fileViewer.classList.remove("hidden");
  // FEATURE-432/444: fill the floating title bar with the full path, size
  // and mtime.
  fvPathEl.textContent = node.path;
  fvSizeEl.textContent = formatSize(node.size);
  fvMtimeEl.textContent = formatMtime(node.mtime);
  // FEATURE-435: show the Raw toggle only for md files (default off).
  fvRawEl.classList.toggle("hidden", !isMdFile(node.path));
  fvRawEl.classList.toggle("on", fvRaw);
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
    // FEATURE-435: md files auto-render only when Raw is off.
    const isMdAuto = isMdFile(path) && !fvRaw;
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
// FEATURE-436: each md block is wrapped in a flex row with a line-number
// gutter on the left showing the block's starting source line.
function renderFileBody() {
  fvBody.textContent = "";
  fvBody.classList.add("md");
  for (const block of mdBlocks(fvMdText)) {
    const row = document.createElement("div");
    row.className = "fv-md-row";
    const no = document.createElement("span");
    no.className = "fv-md-no";
    no.textContent = block.dataset.line || "";
    row.appendChild(no);
    row.appendChild(block);
    fvBody.appendChild(row);
  }
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
  fvRaw = false;
  fvRawEl.classList.remove("on");
  fvTitlebar.classList.remove("active");
  tree.querySelectorAll(".tree-row.fv-selected").forEach((r) => r.classList.remove("fv-selected"));
}

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

// downloadFile triggers a browser download of a workspace file (FEATURE-455).
// It navigates to /api/download?path=... so the server's Content-Disposition
// header makes the browser save the file instead of rendering it.
function downloadFile(path) {
  window.location.href = "/api/download?path=" + encodeURIComponent(path);
}

previewClose.onclick = () => preview.classList.add("hidden");
preview.onclick = (e) => { if (e.target === preview) preview.classList.add("hidden"); };

// FEATURE-432: floating title bar — close button and path reveal.
fvClose.onclick = () => closeFileViewer();
// revealInTree expands the workspace tree to reveal the given file path.
async function revealInTree(path) {
  const parts = path.split("/");
  let acc = "";
  for (let i = 0; i < parts.length - 1; i++) {
    acc = acc ? acc + "/" + parts[i] : parts[i];
    expandedDirs.add(acc);
  }
  await loadTree();
  highlightTreeFile(path);
}
fvPathEl.onclick = () => { if (fvPath) revealInTree(fvPath); };

// FEATURE-435: Raw toggle switches md files between raw text and parsed
// rendering. Toggling reloads the file from the start in the new mode.
fvRawEl.onclick = () => {
  if (!fvPath || !isMdFile(fvPath)) return;
  fvRaw = !fvRaw;
  // FEATURE-436: persist the Raw preference so it survives file switches.
  localStorage.setItem("fvRaw", fvRaw ? "1" : "0");
  fvRawEl.classList.toggle("on", fvRaw);
  // Reload the file from the start in the new mode.
  fvNextLine = 1;
  fvTotal = 0;
  fvMdText = "";
  fvBody.textContent = "";
  fvBody.classList.remove("md");
  loadFileChunk(fvPath, 1, 200);
};

// FEATURE-435: hovering the bottom 100px of the file display area highlights
// the floating title bar (so users don't have to aim precisely at the small
// bar). The bar itself also highlights via :hover.
fileViewer.addEventListener("mousemove", (e) => {
  if (fvPath === null) return;
  const rect = fileViewer.getBoundingClientRect();
  const fromBottom = rect.bottom - e.clientY;
  fvTitlebar.classList.toggle("active", fromBottom <= 100);
});
fileViewer.addEventListener("mouseleave", () => fvTitlebar.classList.remove("active"));

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

// iPad-style settings layout (FEATURE-457): a left category nav bar with
// icons + a right pane showing the selected category's items.
let settingsGroups = [];
let settingsActiveGroup = 0;

// settingsGroupIcon maps a group title keyword to a representative icon.
function settingsGroupIcon(title) {
  const t = (title || "").toLowerCase();
  if (t.includes("模型") || t.includes("model")) return "🧠";
  if (t.includes("显示") || t.includes("display") || t.includes("输出")) return "🖥️";
  if (t.includes("安全") || t.includes("safety") || t.includes("确认")) return "🛡️";
  if (t.includes("记忆") || t.includes("memory") || t.includes("上下文")) return "📚";
  if (t.includes("mcp")) return "🔌";
  if (t.includes("开发") || t.includes("debug") || t.includes("搜索")) return "🔧";
  return "⚙️";
}

// renderSettings renders the grouped setting items returned by settings_get
// (FEATURE-391) in an iPad-style layout: left nav + right content pane.
// The active group is preserved across refreshes (e.g. after a settings_set
// re-fetch) so the user stays on the current category instead of jumping back
// to the first one (FEATURE-470).
function renderSettings(groups) {
  settingsGroups = groups || [];
  if (settingsActiveGroup >= settingsGroups.length) settingsActiveGroup = 0;
  cacheWebInputDir(groups);
  renderSettingsNav();
  renderSettingsPane();
}

// renderSettingsNav builds the left category navigation bar.
function renderSettingsNav() {
  settingsNav.innerHTML = "";
  if (!settingsGroups.length) return;
  const frag = document.createDocumentFragment();
  settingsGroups.forEach((g, i) => {
    const item = document.createElement("div");
    item.className = "settings-nav-item" + (i === settingsActiveGroup ? " active" : "");
    item.setAttribute("data-index", String(i));
    const icon = document.createElement("span");
    icon.className = "settings-nav-icon";
    icon.textContent = settingsGroupIcon(g.title);
    const label = document.createElement("span");
    label.className = "settings-nav-label";
    label.textContent = g.title || "";
    item.appendChild(icon);
    item.appendChild(label);
    item.onclick = () => selectSettingsGroup(i);
    frag.appendChild(item);
  });
  settingsNav.appendChild(frag);
}

// selectSettingsGroup switches the active category and re-renders the pane.
function selectSettingsGroup(index) {
  settingsActiveGroup = index;
  settingsNav.querySelectorAll(".settings-nav-item").forEach((el) => {
    el.classList.toggle("active", Number(el.getAttribute("data-index")) === index);
  });
  renderSettingsPane();
}

// renderSettingsPane renders the active category's items into the right pane.
function renderSettingsPane() {
  settingsDynamic.innerHTML = "";
  const g = settingsGroups[settingsActiveGroup];
  if (!g) {
    settingsDynamic.textContent = "(no settings)";
    return;
  }
  // FEATURE-464: a group with kind "mcp" renders the dedicated MCP server
  // manager instead of generic key/value rows.
  if (g.kind === "mcp") {
    renderMCPServerManager();
    return;
  }
  const frag = document.createDocumentFragment();
  for (const it of g.items || []) {
    frag.appendChild(renderSettingItem(it));
  }
  settingsDynamic.appendChild(frag);
  // Apply current search filter (if any) after re-render.
  const q = settingsSearch.value.trim().toLowerCase();
  if (q) applySettingsFilter(q);
}

// applySettingsFilter filters the active category's items by the given query
// (FEATURE-457). Items whose key or desc contains the query are kept.
function applySettingsFilter(q) {
  q = (q || "").trim().toLowerCase();
  settingsDynamic.querySelectorAll(".set-row").forEach((row) => {
    const key = (row.getAttribute("data-key") || "").toLowerCase();
    const desc = (row.getAttribute("data-desc") || "").toLowerCase();
    const match = !q || key.includes(q) || desc.includes(q);
    row.style.display = match ? "" : "none";
  });
}

// settingsSearch input handler: live filter within the active category.
settingsSearch.addEventListener("input", () => {
  const q = settingsSearch.value.trim().toLowerCase();
  if (!q) {
    settingsDynamic.querySelectorAll(".set-row").forEach((row) => { row.style.display = ""; });
    return;
  }
  applySettingsFilter(q);
});

// renderSettingItem builds one setting row with its label and form control.
function renderSettingItem(it) {
  // FEATURE-477: the system logo is rendered as a dedicated block (not a
  // compact key/value row) so it has room for a large preview and a clear
  // divider from the parameter rows below.
  if (it.type === "logo") return renderLogoBlock();
  const row = document.createElement("label");
  row.className = "set-row";
  row.setAttribute("data-key", it.key || "");
  row.setAttribute("data-desc", it.desc || "");
  const label = document.createElement("span");
  label.className = "set-label";
  label.textContent = it.key;
  // FEATURE-470: the tooltip shows the system default value so the user knows
  // what the default is for this parameter. An empty default (e.g. web-whitelist
  // = loopback only) is shown as "(empty)".
  const def = it.default != null ? String(it.default) : "";
  const defDisplay = def === "" ? "(empty)" : def;
  label.title = it.desc || "";
  if (def !== "" || it.default != null) label.title += (label.title ? "\n" : "") + i18nT("setDefaultTip", "默认值") + ": " + defDisplay;
  row.appendChild(label);

  // FEATURE-470: red * marker shown right of the control when the current value
  // differs from the default. Kept as a reference so onchange can add/remove it
  // dynamically as the user edits the value.
  let mark = null;
  const updateDiffMark = (val) => {
    const show = def !== "" && String(val) !== def;
    if (show && !mark) {
      mark = document.createElement("span");
      mark.className = "set-diff";
      mark.textContent = "*";
      mark.title = i18nT("setDiffTip", "与默认值不一致") + " (" + i18nT("setDefaultTip", "默认值") + ": " + def + ")";
      row.appendChild(mark);
    } else if (!show && mark) {
      mark.remove();
      mark = null;
    }
  };

  let ctl;
  let curVal = it.value;
  if (it.key === "theme-mode") {
    // Frontend-local theme setting (FEATURE-457): rendered as a select that
    // persists to localStorage and applies the theme immediately.
    ctl = document.createElement("select");
    ctl.className = "set-select";
    const opts = ["auto", "dark", "light", "light-tp", "paper"];
    const cur = localStorage.getItem("co-shell-theme") || "auto";
    curVal = cur;
    for (const opt of opts) {
      const o = document.createElement("option");
      o.value = opt;
      o.textContent = opt;
      if (opt === cur) o.selected = true;
      ctl.appendChild(o);
    }
    ctl.onchange = () => {
      localStorage.setItem("co-shell-theme", ctl.value);
      applyTheme();
      updateDiffMark(ctl.value);
      // FEATURE-477: re-render the pane so the logo block follows the newly
      // selected theme (dark/light/light-tp).
      if (!settingsModal.classList.contains("hidden")) renderSettingsPane();
    };
  } else if (it.type === "bool") {
    ctl = document.createElement("input");
    ctl.type = "checkbox";
    ctl.className = "set-toggle";
    ctl.checked = it.value === "on";
    ctl.onchange = () => {
      wsSend({ type: "settings_set", key: it.key, value: ctl.checked ? "on" : "off" });
      updateDiffMark(ctl.checked ? "on" : "off");
    };
  } else if (it.type === "number") {
    ctl = document.createElement("input");
    ctl.type = "number";
    ctl.className = "set-input";
    ctl.value = it.value;
    ctl.onchange = () => {
      wsSend({ type: "settings_set", key: it.key, value: ctl.value });
      updateDiffMark(ctl.value);
    };
  } else if (it.type === "enum") {
    ctl = document.createElement("select");
    ctl.className = "set-select";
    // FEATURE-470: for thinking-enabled / reasoning-effort the internal value
    // "default" is displayed as "by model" (the model decides), while the
    // stored value stays "default".
    const isByModel = it.key === "thinking-enabled" || it.key === "reasoning-effort";
    const optLabel = (opt) => (isByModel && opt === "default" ? "by model" : opt);
    for (const opt of it.options || []) {
      const o = document.createElement("option");
      o.value = opt;
      o.textContent = optLabel(opt);
      if (opt === it.value) o.selected = true;
      ctl.appendChild(o);
    }
    ctl.onchange = () => {
      wsSend({ type: "settings_set", key: it.key, value: ctl.value });
      updateDiffMark(ctl.value);
    };
  } else {
    ctl = document.createElement("input");
    ctl.type = "text";
    ctl.className = "set-input";
    ctl.value = it.value;
    ctl.onchange = () => {
      wsSend({ type: "settings_set", key: it.key, value: ctl.value });
      updateDiffMark(ctl.value);
    };
  }
  row.appendChild(ctl);
  updateDiffMark(curVal);
  return row;
}

// currentResolvedTheme returns the actual parsed theme (dark|light|light-tp)
// from the <html> data-theme attribute (FEATURE-477). Each tone has its own
// logo, so no mapping is applied.
function currentResolvedTheme() {
  return document.documentElement.getAttribute("data-theme");
}

// renderLogoBlock builds the system-logo configuration block (FEATURE-477). It
// targets the theme that matches the current theme-mode (dark/light/light-tp;
// auto resolves to the parsed theme), so switching the theme-mode re-renders
// this block for the new theme. It shows a large preview area (with a
// placeholder when none is configured), an upload button (clipboard paste or
// file picker) and a remove button, separated from the parameter rows below by
// a divider.
function renderLogoBlock() {
  const theme = currentResolvedTheme();
  // Each tone has its own label (FEATURE-477).
  const themeLabel = theme === "dark" ? i18nT("logoThemeDark", "深色主题")
    : (theme === "light-tp" ? i18nT("logoThemeLightTp", "浅色主题(蓝灰)")
      : (theme === "paper" ? i18nT("logoThemePaper", "纸张护眼") : i18nT("logoThemeLight", "浅色主题")));

  const block = document.createElement("div");
  block.className = "logo-block";
  block.setAttribute("data-key", "logo");
  block.setAttribute("data-desc", "logo");

  const head = document.createElement("div");
  head.className = "logo-block-head";
  const title = document.createElement("span");
  title.className = "logo-block-title";
  title.textContent = i18nT("logoTitle", "系统 logo");
  const badge = document.createElement("span");
  badge.className = "logo-block-badge";
  badge.textContent = themeLabel;
  head.appendChild(title);
  head.appendChild(badge);
  block.appendChild(head);

  const previewWrap = document.createElement("div");
  previewWrap.className = "logo-block-preview";
  const preview = document.createElement("img");
  preview.className = "logo-block-img";
  preview.alt = "";
  preview.hidden = true;
  const placeholder = document.createElement("span");
  placeholder.className = "logo-block-placeholder";
  placeholder.textContent = i18nT("logoEmpty", "尚未配置 " + themeLabel + " logo");
  previewWrap.appendChild(preview);
  previewWrap.appendChild(placeholder);
  block.appendChild(previewWrap);

  const btns = document.createElement("div");
  btns.className = "logo-block-btns";
  const uploadBtn = document.createElement("button");
  uploadBtn.type = "button";
  uploadBtn.className = "btn-mini";
  uploadBtn.textContent = i18nT("logoUpload", "上传 logo");
  btns.appendChild(uploadBtn);
  const removeBtn = document.createElement("button");
  removeBtn.type = "button";
  removeBtn.className = "btn-mini";
  removeBtn.textContent = i18nT("logoRemove", "移除");
  removeBtn.hidden = true;
  btns.appendChild(removeBtn);
  block.appendChild(btns);

  // FEATURE-477: a scale slider (1-200%) controls the topbar logo display size.
  const scaleRow = document.createElement("div");
  scaleRow.className = "logo-block-scale";
  const scaleLabel = document.createElement("span");
  scaleLabel.className = "logo-block-scale-label";
  scaleLabel.textContent = i18nT("logoScale", "显示大小");
  const scaleVal = document.createElement("span");
  scaleVal.className = "logo-block-scale-val";
  const scaleInput = document.createElement("input");
  scaleInput.type = "range";
  scaleInput.min = "1";
  scaleInput.max = "200";
  scaleInput.value = String(logoScale());
  scaleVal.textContent = scaleInput.value + "%";
  scaleInput.oninput = () => {
    localStorage.setItem("co-shell-logo-scale", scaleInput.value);
    scaleVal.textContent = scaleInput.value + "%";
    updateBrandLogo();
  };
  scaleRow.appendChild(scaleLabel);
  scaleRow.appendChild(scaleInput);
  scaleRow.appendChild(scaleVal);
  block.appendChild(scaleRow);

  const fileInput = document.createElement("input");
  fileInput.type = "file";
  fileInput.accept = "image/*";
  fileInput.hidden = true;
  block.appendChild(fileInput);

  const refresh = () => {
    // A cache-busting query param forces the browser to re-fetch the logo so an
    // overwrite/removal is reflected immediately (FEATURE-477).
    const url = "/logos/" + theme + "?t=" + Date.now();
    const probe = new Image();
    probe.onload = () => {
      preview.src = url;
      preview.hidden = false;
      placeholder.hidden = true;
      removeBtn.hidden = false;
    };
    probe.onerror = () => {
      preview.hidden = true;
      preview.removeAttribute("src");
      placeholder.hidden = false;
      removeBtn.hidden = true;
    };
    probe.src = url;
  };

  const uploadBlob = (blob) => {
    if (!blob || !blob.type || !blob.type.startsWith("image/")) {
      showSettingsResult({ ok: false, message: i18nT("logoNotImage", "请粘贴/选择图片") });
      return;
    }
    const fd = new FormData();
    fd.append("theme", theme);
    fd.append("file", blob, "logo.png");
    fetch("/api/logo", { method: "POST", body: fd })
      .then((r) => r.json())
      .then((j) => {
        showSettingsResult({ ok: !!j.ok, message: j.ok ? i18nT("logoSaved", "logo 已保存") : (j.error || "error") });
        if (j.ok) { refresh(); updateBrandLogo(); }
      })
      .catch(() => showSettingsResult({ ok: false, message: "upload failed" }));
  };

  uploadBtn.onclick = async () => {
    // Prefer clipboard image (paste a screenshot); fall back to a file picker.
    try {
      if (navigator.clipboard && navigator.clipboard.read) {
        const items = await navigator.clipboard.read();
        for (const it of items) {
          const t = it.types.find((x) => x.startsWith("image/"));
          if (t) { uploadBlob(await it.getType(t)); return; }
        }
      }
    } catch (e) { /* clipboard read denied/unsupported -> file picker */ }
    fileInput.click();
  };

  fileInput.onchange = () => {
    if (fileInput.files && fileInput.files[0]) uploadBlob(fileInput.files[0]);
    fileInput.value = "";
  };

  removeBtn.onclick = () => {
    fetch("/api/logo?theme=" + theme, { method: "DELETE" })
      .then((r) => r.json())
      .then((j) => {
        if (j.ok) { refresh(); updateBrandLogo(); }
      })
      .catch(() => {});
  };

  refresh();
  return block;
}

// showSettingsResult displays the result of a settings_set change. The message
// is shown in the dedicated #settingsResult container pinned to the bottom of
// the settings pane (right side), so it does not shift the setting list and
// cause the panel to jump (FEATURE-462).
function showSettingsResult(msg) {
  const el = document.createElement("div");
  el.className = "set-result " + (msg.ok ? "ok" : "err");
  el.textContent = msg.ok ? (msg.message || "ok") : (msg.message || "error");
  const box = document.getElementById("settingsResult");
  if (box) {
    box.textContent = "";
    box.appendChild(el);
  } else {
    settingsDynamic.prepend(el);
  }
  setTimeout(() => el.remove(), 3000);
  // FEATURE-470: after a successful change, re-fetch the settings so the pane
  // reflects the new value immediately (switching groups no longer reverts it).
  if (msg.ok) wsSend({ type: "settings_get" });
}

/* ---------- MCP server manager (FEATURE-464) ---------- */

// mcpServers holds the latest MCP server list from the backend.
let mcpServers = [];
// mcpEditing holds the name of the server currently being edited (null = none).
let mcpEditing = null;

// renderMCPServers stores the MCP server list and re-renders the manager if it
// is the active settings group (FEATURE-464). It does NOT re-send mcp_get (that
// would loop with renderMCPServerManager); it only re-renders from the received
// list.
function renderMCPServers(servers) {
  mcpServers = servers || [];
  const g = settingsGroups[settingsActiveGroup];
  if (g && g.kind === "mcp") renderMCPServerManager(false);
}

// renderMCPServerManager renders the MCP server list + add/edit form into the
// settings pane (FEATURE-464). When fetch is true (default) it requests the
// latest server list from the backend so the list reflects the shared config
// (same as the REPL :mcp). renderMCPServers calls it with fetch=false to avoid
// a request loop.
function renderMCPServerManager(fetch) {
  if (fetch !== false) wsSend({ type: "mcp_get" });
  settingsDynamic.innerHTML = "";
  const frag = document.createDocumentFragment();

  // Add / edit form.
  const form = document.createElement("div");
  form.className = "mcp-form";
  const editing = mcpEditing ? mcpServers.find((s) => s.name === mcpEditing) : null;

  const nameLbl = document.createElement("span");
  nameLbl.className = "mcp-form-label";
  nameLbl.textContent = i18nT("mcpName", "名称");
  const nameInput = document.createElement("input");
  nameInput.className = "set-input mcp-name";
  nameInput.placeholder = i18nT("mcpNamePh", "如 filesystem");
  nameInput.value = editing ? editing.name : "";
  if (editing) nameInput.disabled = true; // name is the identity key

  const cmdLbl = document.createElement("span");
  cmdLbl.className = "mcp-form-label";
  cmdLbl.textContent = i18nT("mcpCommand", "命令");
  const cmdInput = document.createElement("input");
  cmdInput.className = "set-input mcp-command";
  cmdInput.placeholder = i18nT("mcpCommandPh", "如 npx");
  cmdInput.value = editing ? editing.command : "";

  const argsLbl = document.createElement("span");
  argsLbl.className = "mcp-form-label";
  argsLbl.textContent = i18nT("mcpArgs", "参数");
  const argsInput = document.createElement("input");
  argsInput.className = "set-input mcp-args";
  argsInput.placeholder = i18nT("mcpArgsPh", "空格分隔，如 -y @modelcontextprotocol/server-filesystem");
  argsInput.value = editing ? (editing.args || []).join(" ") : "";

  const btnRow = document.createElement("div");
  btnRow.className = "mcp-form-btns";
  const saveBtn = document.createElement("button");
  saveBtn.className = "btn sm primary";
  saveBtn.textContent = editing ? i18nT("mcpSave", "保存") : i18nT("mcpAdd", "添加");
  saveBtn.onclick = () => {
    const name = nameInput.value.trim();
    const command = cmdInput.value.trim();
    const args = argsInput.value.trim() ? argsInput.value.trim().split(/\s+/) : [];
    if (!name || !command) {
      showMCPResult({ ok: false, message: i18nT("mcpNeedNameCmd", "名称和命令不能为空") });
      return;
    }
    if (editing) {
      // FIX-465: enabled is toggled on the card, not in the edit form.
      const cur = mcpServers.find((s) => s.name === name);
      wsSend({ type: "mcp_update", name, command, args, enabled: cur ? cur.enabled : true });
    } else {
      wsSend({ type: "mcp_add", name, command, args });
    }
    mcpEditing = null;
  };
  btnRow.appendChild(saveBtn);
  if (editing) {
    const cancelBtn = document.createElement("button");
    cancelBtn.className = "btn sm";
    cancelBtn.textContent = i18nT("mcpCancel", "取消");
    cancelBtn.onclick = () => { mcpEditing = null; renderMCPServerManager(); };
    btnRow.appendChild(cancelBtn);
  }

  form.appendChild(nameLbl); form.appendChild(nameInput);
  form.appendChild(cmdLbl); form.appendChild(cmdInput);
  form.appendChild(argsLbl); form.appendChild(argsInput);
  form.appendChild(btnRow);
  frag.appendChild(form);

  // Server list.
  const list = document.createElement("div");
  list.className = "mcp-list";
  if (!mcpServers.length) {
    const empty = document.createElement("div");
    empty.className = "mcp-empty";
    empty.textContent = i18nT("mcpEmpty", "未配置 MCP 服务器");
    list.appendChild(empty);
  } else {
    for (const s of mcpServers) {
      list.appendChild(renderMCPServerRow(s));
    }
  }
  frag.appendChild(list);

  settingsDynamic.appendChild(frag);
}

// renderMCPServerRow builds one MCP server card (FIX-465): the first line holds
// the clickable title + a toggle switch + the delete button; the second line
// holds the command text (clickable to edit). Clicking the title or command
// enters edit mode; the toggle switches enabled state directly.
function renderMCPServerRow(s) {
  const row = document.createElement("div");
  row.className = "mcp-row" + (s.enabled ? "" : " off");

  // First line: title (clickable) + toggle + delete.
  const head = document.createElement("div");
  head.className = "mcp-row-head";
  const name = document.createElement("div");
  name.className = "mcp-row-name";
  name.textContent = s.name;
  name.title = i18nT("mcpEditHint", "点击编辑");
  name.onclick = () => { mcpEditing = s.name; renderMCPServerManager(); };
  head.appendChild(name);

  const toggle = document.createElement("label");
  toggle.className = "mcp-toggle";
  const toggleInput = document.createElement("input");
  toggleInput.type = "checkbox";
  toggleInput.checked = !!s.enabled;
  toggleInput.onchange = () => {
    wsSend({ type: "mcp_update", name: s.name, command: s.command, args: s.args || [], enabled: toggleInput.checked });
  };
  const slider = document.createElement("span");
  slider.className = "mcp-toggle-slider";
  toggle.appendChild(toggleInput);
  toggle.appendChild(slider);
  head.appendChild(toggle);

  const delBtn = document.createElement("button");
  delBtn.className = "btn sm danger";
  delBtn.textContent = i18nT("mcpDelete", "删除");
  delBtn.onclick = () => {
    if (confirm(i18nT("mcpDeleteConfirm", "确定删除 MCP 服务器 ") + s.name + "?")) {
      wsSend({ type: "mcp_remove", name: s.name });
    }
  };
  head.appendChild(delBtn);
  row.appendChild(head);

  // Second line: command text (clickable to edit), single full row.
  const detail = document.createElement("div");
  detail.className = "mcp-row-detail";
  detail.textContent = s.command + (s.args && s.args.length ? " " + s.args.join(" ") : "");
  detail.title = i18nT("mcpEditHint", "点击编辑");
  detail.onclick = () => { mcpEditing = s.name; renderMCPServerManager(); };
  row.appendChild(detail);

  return row;
}

// showMCPResult displays the result of an MCP add/update/remove operation in the
// settings result box (FEATURE-464).
function showMCPResult(msg) {
  const el = document.createElement("div");
  el.className = "set-result " + (msg.ok ? "ok" : "err");
  el.textContent = msg.ok ? (msg.message || "ok") : (msg.message || "error");
  const box = document.getElementById("settingsResult");
  if (box) {
    box.textContent = "";
    box.appendChild(el);
  } else {
    settingsDynamic.prepend(el);
  }
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

// FEATURE-449: the toolbar pin button pins (switches to) the selected model.
modelPinBtn.onclick = () => {
  if (!selectedModelID) return;
  wsSend({ type: "model_switch", value: selectedModelID });
};

// FEATURE-449: the toolbar delete button asks for confirmation before removing
// the selected model.
modelDelBtn.onclick = () => {
  if (!selectedModelID) return;
  const m = modelList.find((x) => x.id === selectedModelID);
  if (m) confirmDeleteModel(m);
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

// renderModelsBody renders the model list into the manager modal. Each row is
// selectable (click to highlight with a border); the toolbar's pin/delete
// buttons act on the selected model (FEATURE-449).
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
    row.className = "model-row" + (m.enabled ? " enabled" : "") + (m.id === selectedModelID ? " selected" : "");
    row.dataset.modelId = m.id;
    // FEATURE-449: clicking a row selects it (highlighted border); the toolbar
    // pin/delete buttons then act on the selected model.
    row.onclick = () => selectModel(m.id);
    // Left: provider logo (FEATURE-429).
    const logo = document.createElement("img");
    logo.className = "model-logo";
    logo.src = "/static/logos/" + (MODEL_LOGOS[m.provider] || "generic.png");
    logo.alt = "";
    logo.onerror = () => { logo.style.display = "none"; };
    row.appendChild(logo);
    // Left: status + identity.
    const info = document.createElement("div");
    info.className = "model-info";
    const id = document.createElement("div");
    id.className = "model-id";
    id.textContent = m.id;
    id.title = m.name || m.id;
    // FEATURE-449: clicking the model ID opens the edit wizard (stopPropagation
    // so it does not also select the row).
    id.style.cursor = "pointer";
    id.onclick = (e) => { e.stopPropagation(); modelsModal.classList.add("hidden"); openModelWizard("edit", m.id); };
    info.appendChild(id);
    const meta = document.createElement("div");
    meta.className = "model-meta";
    const caps = [];
    if (m.vision) caps.push("👁");
    if (m.tool_call) caps.push("🔧");
    if (m.thinking) caps.push("💭");
    meta.textContent = m.provider + " · " + m.model + (caps.length ? " · " + caps.join(" ") : "") + " · P" + m.priority;
    info.appendChild(meta);
    // FEATURE-449: a second meta line showing the model's endpoint URL.
    if (m.endpoint) {
      const url = document.createElement("div");
      url.className = "model-url";
      url.textContent = m.endpoint;
      url.title = m.endpoint;
      info.appendChild(url);
    }
    row.appendChild(info);
    // Right: enable/disable toggle switch (the only per-row control; pin and
    // delete moved to the toolbar, FEATURE-449).
    const actions = document.createElement("div");
    actions.className = "model-actions";
    const tgl = document.createElement("label");
    tgl.className = "model-toggle";
    tgl.title = m.enabled ? "禁用此模型" : "启用此模型";
    const tglInp = document.createElement("input");
    tglInp.type = "checkbox";
    tglInp.checked = m.enabled;
    tglInp.onchange = (e) => { e.stopPropagation(); wsSend({ type: m.enabled ? "model_disable" : "model_enable", value: m.id }); };
    const tglSlider = document.createElement("span");
    tglSlider.className = "model-toggle-slider";
    tgl.appendChild(tglInp);
    tgl.appendChild(tglSlider);
    actions.appendChild(tgl);
    row.appendChild(actions);
    modelsBody.appendChild(row);
  }
}

// selectedModelID is the model currently selected in the manager modal. The
// toolbar's pin/delete buttons act on it (FEATURE-449).
let selectedModelID = null;

// selectModel highlights the given model row with a selected border and records
// it as the active selection for the toolbar pin/delete buttons (FEATURE-449).
function selectModel(id) {
  selectedModelID = id;
  modelsBody.querySelectorAll(".model-row").forEach((r) => {
    r.classList.toggle("selected", r.dataset.modelId === id);
  });
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

// FEATURE-429: structured step-by-step wizard. The browser owns the
// accumulated field data (wizardData) and the current step (wizardStep); the
// backend returns the form of each step as structured JSON.
let wizardData = {};
let wizardStep = "";

// wizardStepTitles maps a step key to its navigation label.
const wizardStepTitles = {
  template: "模板", endpoint: "接口地址", api_key: "API Key",
  model_name: "选择模型", capabilities: "能力", model_id: "模型 ID",
  priority: "优先级", max_model_len: "上下文长度", enabled: "启用"
};

// wizardModelMaxLens maps a model ID to its max context length (in tokens) as
// reported by the API. Populated from the model_name step so the selected
// model's max length can be recorded for the max_model_len step.
let wizardModelMaxLens = {};

// openModelWizard opens the wizard modal and starts the structured wizard.
function openModelWizard(mode, id) {
  wizardActive = true;
  wizardData = {};
  wizardStep = "";
  modelWizardBody.textContent = "";
  modelWizardNav.textContent = "";
  modelWizardModal.classList.remove("hidden");
  wsSend({ type: "model_wizard_start", value: id || "" });
}

// closeModelWizard closes the wizard modal and clears the wizard-active flag.
function closeModelWizard() {
  wizardActive = false;
  modelWizardModal.classList.add("hidden");
  modelWizardBody.textContent = "";
  modelWizardNav.textContent = "";
}

// The wizard cancel button closes the modal (the structured wizard is stateless,
// so no backend cancel is needed).
modelWizardCancel.onclick = () => { closeModelWizard(); };

// showModelWizardStep handles a model_wizard message: renders the step form or
// reports completion/error.
function showModelWizardStep(msg) {
  if (!msg.ok) {
    closeModelWizard();
    if (msg.message) alert(msg.message);
    return;
  }
  if (msg.wizard_step) {
    wizardData = msg.wizard_data || wizardData;
    renderWizardStep(msg.wizard_step);
  } else {
    // Completed: close and refresh the model list.
    closeModelWizard();
    wsSend({ type: "model_get" });
    refreshModelInfo();
  }
}

// renderWizardStep renders the left nav + right form + bottom buttons for a step.
function renderWizardStep(step) {
  wizardStep = step.step;
  // Remember each model's max context length (reported by the API) so the
  // selected model's max length can be recorded for the max_model_len step.
  wizardModelMaxLens = step.model_max_lens || {};
  // Left navigation.
  modelWizardNav.textContent = "";
  const order = ["template", "endpoint", "api_key", "model_name", "capabilities", "model_id", "priority", "max_model_len", "enabled"];
  const curIdx = order.indexOf(step.step);
  for (let i = 0; i < order.length; i++) {
    const item = document.createElement("div");
    item.className = "wizard-nav-item" + (i === curIdx ? " current" : "") + (i < curIdx ? " done" : "");
    item.textContent = (i + 1) + ". " + (wizardStepTitles[order[i]] || order[i]);
    if (i < curIdx) {
      item.onclick = () => { wizardStep = order[i]; wsSend({ type: "model_wizard_prev", step: order[i], wizard_data: wizardData }); };
    }
    modelWizardNav.appendChild(item);
  }
  // Right form.
  modelWizardBody.textContent = "";
  const title = document.createElement("div");
  title.className = "wizard-step-title";
  title.textContent = step.title || (wizardStepTitles[step.step] || step.step);
  modelWizardBody.appendChild(title);
  // Show an informational/error message (e.g. why the model list refresh failed).
  if (step.message) {
    const msg = document.createElement("div");
    msg.className = "wizard-step-message";
    msg.textContent = step.message;
    modelWizardBody.appendChild(msg);
  }
  for (const f of (step.fields || [])) {
    modelWizardBody.appendChild(renderWizardField(f));
  }
  // FEATURE-467: on the template step, wire up the thinking switch to show/hide
  // the reasoning_effort select, and render the collapsible template-JSON viewer.
  if (step.step === "template") setupWizardTemplateExtras(step);
  // Bottom buttons.
  modelWizardPrev.style.display = step.is_first ? "none" : "";
  modelWizardNext.style.display = step.is_last ? "none" : "";
  modelWizardSubmit.style.display = step.is_last ? "" : "none";
}

// setupWizardTemplateExtras wires the template step's conditional thinking
// controls and the collapsible template-JSON transparency viewer (FEATURE-467).
function setupWizardTemplateExtras(step) {
  const thinkingWrap = modelWizardBody.querySelector(".wizard-field-thinking");
  const effortWrap = modelWizardBody.querySelector(".wizard-field-reasoning_effort");
  const thinkingInput = thinkingWrap && thinkingWrap.querySelector("input[type=checkbox]");
  // FEATURE-467: when the user switches the template dropdown, re-request the
  // template step so the thinking switch / reasoning_effort / template JSON
  // refresh to the newly selected template's values.
  const tplSelect = modelWizardBody.querySelector(".wizard-field-template_id select");
  if (tplSelect) {
    tplSelect.addEventListener("change", () => {
      // Switching template resets the thinking settings and API type so the new
      // template's defaults (capabilities.thinking / reasoning_effort / chat API)
      // apply (FEATURE-467/FEATURE-468).
      wizardData.thinking = false;
      wizardData.reasoning_effort = "";
      wizardData.api_type = "";
      collectWizardFields();
      wsSend({ type: "model_wizard_refresh", step: "template", wizard_data: wizardData });
    });
  }
  // Show reasoning_effort only while thinking is on. When thinking is turned
  // off, clear the reasoning_effort value so it is not persisted.
  const applyEffortVisibility = () => {
    const on = !!(thinkingInput && thinkingInput.checked);
    if (effortWrap) {
      effortWrap.style.display = on ? "" : "none";
      if (!on) {
        const sel = effortWrap.querySelector("select");
        if (sel) sel.value = "";
        wizardData.reasoning_effort = "";
      }
    }
  };
  if (thinkingInput) {
    thinkingInput.addEventListener("change", applyEffortVisibility);
    applyEffortVisibility();
  } else if (effortWrap) {
    effortWrap.style.display = "none";
  }
  // Collapsible template-JSON viewer (transparency).
  if (step.template_json) {
    const box = document.createElement("div");
    box.className = "wizard-template-json";
    const head = document.createElement("div");
    head.className = "wizard-template-json-head";
    const toggle = document.createElement("button");
    toggle.type = "button";
    toggle.className = "wizard-template-json-toggle";
    toggle.textContent = "▸ " + (T.templateJson || "查看模板原始 JSON");
    const pre = document.createElement("pre");
    pre.className = "wizard-template-json-body";
    pre.textContent = step.template_json;
    pre.style.display = "none";
    toggle.onclick = () => {
      const open = pre.style.display !== "none";
      pre.style.display = open ? "none" : "";
      toggle.textContent = (open ? "▸ " : "▾ ") + (T.templateJson || "查看模板原始 JSON");
    };
    head.appendChild(toggle);
    box.appendChild(head);
    box.appendChild(pre);
    modelWizardBody.appendChild(box);
  }
}

// renderWizardField builds a form control for one field.
function renderWizardField(f) {
  const wrap = document.createElement("div");
  // FEATURE-467: tag the wrapper with the field key so the wizard can locate
  // and show/hide conditional fields (e.g. reasoning_effort under thinking).
  wrap.className = "wizard-field wizard-field-" + f.key + ((f.type === "checkbox" || f.type === "switch") ? " toggle" : "");
  const label = document.createElement("label");
  label.className = "wizard-field-label";
  label.textContent = f.label || f.key;
  wrap.appendChild(label);
  let ctl;
  if (f.type === "select") {
    ctl = document.createElement("select");
    ctl.className = "wizard-input";
    ctl.dataset.key = f.key;
    for (const opt of (f.options || [])) {
      const o = document.createElement("option");
      o.value = opt;
      // FEATURE-467: an empty option value means "not set" (do not send the
      // reasoning_effort parameter); show a friendly label instead of blank.
      // FEATURE-468: the api_type "chat" option is the default API; its label
      // carries a "(default)" suffix (the option itself is selected by default).
      let optLabel = opt;
      if (opt === "") optLabel = (T.reasoningEffortNone || "不设置");
      else if (f.key === "api_type" && opt === "chat") optLabel = T.apiTypeChatDefault || "chat (默认)";
      o.textContent = optLabel;
      ctl.appendChild(o);
    }
    if (f.value) ctl.value = f.value;
    // Record the selected model's max context length (reported by the API) so
    // the max_model_len step can pre-fill / hint it (FEATURE-429). Sync it
    // immediately (covers re-render after a model-list refresh, where the
    // change event does not fire) and on every user change.
    if (f.key === "model_name") {
      if (ctl.value) wizardData.model_max_len = wizardModelMaxLens[ctl.value] || 0;
      ctl.addEventListener("change", () => {
        wizardData.model_max_len = wizardModelMaxLens[ctl.value] || 0;
      });
    }
  } else if (f.type === "checkbox" || f.type === "switch") {
    // Slider toggle switch: label on the left, switch on the right.
    const tgl = document.createElement("label");
    tgl.className = "wizard-toggle";
    const inp = document.createElement("input");
    inp.type = "checkbox";
    inp.dataset.key = f.key;
    inp.checked = f.value === "true";
    const slider = document.createElement("span");
    slider.className = "wizard-toggle-slider";
    tgl.appendChild(inp);
    tgl.appendChild(slider);
    ctl = tgl;
  } else {
    ctl = document.createElement("input");
    ctl.type = f.type === "password" ? "password" : (f.type === "number" ? "number" : "text");
    ctl.className = "wizard-input";
    ctl.dataset.key = f.key;
    ctl.value = f.value || "";
  }
  wrap.appendChild(ctl);
  // FEATURE-433: connectivity test button for the endpoint field.
  if (f.key === "endpoint") {
    const row = document.createElement("div");
    row.className = "wizard-endpoint-test";
    const btn = document.createElement("button");
    btn.type = "button";
    btn.className = "wizard-test-btn";
    btn.textContent = "测试连通性";
    const res = document.createElement("span");
    res.className = "wizard-test-result";
    btn.onclick = async () => {
      const ep = ctl.value.trim();
      if (!ep) { res.textContent = "请输入接口地址"; res.className = "wizard-test-result err"; return; }
      btn.disabled = true;
      res.textContent = "测试中...";
      res.className = "wizard-test-result";
      try {
        const resp = await fetch("/api/test-endpoint", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ endpoint: ep }),
        });
        const body = await resp.json();
        if (body.ok) {
          // If the backend auto-completed the endpoint (e.g. added http:// or
          // /v1), persist the completed URL so the saved model uses it instead
          // of the user's incomplete input (FEATURE-447).
          if (body.endpoint && body.endpoint !== ep) {
            ctl.value = body.endpoint;
            wizardData.endpoint = body.endpoint;
          }
          res.textContent = "✅ 连通 (" + (body.endpoint || ep) + ")";
          res.className = "wizard-test-result ok";
        } else {
          res.textContent = "❌ 失败: " + (body.message || "无法连接");
          res.className = "wizard-test-result err";
        }
      } catch (e) {
        res.textContent = "❌ 失败: " + e.message;
        res.className = "wizard-test-result err";
      } finally {
        btn.disabled = false;
      }
    };
    row.appendChild(btn);
    row.appendChild(res);
    wrap.appendChild(row);
  }
  // API key test button: verifies the key against the endpoint by calling
  // ListModels (GET /models).
  if (f.key === "api_key") {
    const row = document.createElement("div");
    row.className = "wizard-endpoint-test";
    const btn = document.createElement("button");
    btn.type = "button";
    btn.className = "wizard-test-btn";
    btn.textContent = "测试 API Key";
    const res = document.createElement("span");
    res.className = "wizard-test-result";
    btn.onclick = async () => {
      const key = ctl.value.trim();
      const ep = (wizardData.endpoint || "").trim();
      if (!key) { res.textContent = "请输入 API Key"; res.className = "wizard-test-result err"; return; }
      if (!ep) { res.textContent = "请先填写接口地址"; res.className = "wizard-test-result err"; return; }
      btn.disabled = true;
      res.textContent = "测试中...";
      res.className = "wizard-test-result";
      try {
        const resp = await fetch("/api/test-api-key", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ endpoint: ep, api_key: key }),
        });
        const body = await resp.json();
        if (body.ok) {
          res.textContent = "✅ " + (body.message || "API Key 有效");
          res.className = "wizard-test-result ok";
        } else {
          res.textContent = "❌ " + (body.message || "API Key 无效");
          res.className = "wizard-test-result err";
        }
      } catch (e) {
        res.textContent = "❌ 失败: " + e.message;
        res.className = "wizard-test-result err";
      } finally {
        btn.disabled = false;
      }
    };
    row.appendChild(btn);
    row.appendChild(res);
    wrap.appendChild(row);
  }
  // Refresh model list button: re-fetches the model suggestions on the
  // model_name step without advancing.
  if (f.key === "model_name") {
    const row = document.createElement("div");
    row.className = "wizard-endpoint-test";
    const btn = document.createElement("button");
    btn.type = "button";
    btn.className = "wizard-test-btn";
    btn.textContent = "刷新模型列表";
    const res = document.createElement("span");
    res.className = "wizard-test-result";
    btn.onclick = async () => {
      collectWizardFields();
      btn.disabled = true;
      res.textContent = "刷新中...";
      res.className = "wizard-test-result";
      wsSend({ type: "model_wizard_refresh", step: wizardStep, wizard_data: wizardData });
      // Re-enable after a short delay; the re-render replaces this button.
      setTimeout(() => { btn.disabled = false; }, 1500);
    };
    row.appendChild(btn);
    row.appendChild(res);
    wrap.appendChild(row);
  }
  // Fetch max context length button: queries the API for the selected model's
  // max context length and fills it into the input. On failure, shows the reason.
  if (f.key === "max_model_len") {
    const row = document.createElement("div");
    row.className = "wizard-endpoint-test";
    const btn = document.createElement("button");
    btn.type = "button";
    btn.className = "wizard-test-btn";
    btn.textContent = "获得模型最大上下文长度";
    const res = document.createElement("span");
    res.className = "wizard-test-result";
    btn.onclick = async () => {
      const model = (wizardData.model_name || "").trim();
      const ep = (wizardData.endpoint || "").trim();
      if (!model) { res.textContent = "请先在「选择模型」步骤选择模型"; res.className = "wizard-test-result err"; return; }
      if (!ep) { res.textContent = "请先填写接口地址"; res.className = "wizard-test-result err"; return; }
      btn.disabled = true;
      res.textContent = "获取中...";
      res.className = "wizard-test-result";
      try {
        const resp = await fetch("/api/get-model-max-len", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ endpoint: ep, api_key: wizardData.api_key || "", model_name: model }),
        });
        const body = await resp.json();
        if (body.ok && body.max_model_len > 0) {
          ctl.value = body.max_model_len;
          wizardData.max_model_len = body.max_model_len;
          res.textContent = "✅ 已填入 " + body.max_model_len;
          res.className = "wizard-test-result ok";
        } else {
          res.textContent = "❌ " + (body.message || "未获取到该模型的最大上下文长度");
          res.className = "wizard-test-result err";
        }
      } catch (e) {
        res.textContent = "❌ 失败: " + e.message;
        res.className = "wizard-test-result err";
      } finally {
        btn.disabled = false;
      }
    };
    row.appendChild(btn);
    row.appendChild(res);
    wrap.appendChild(row);
  }
  if (f.hint) {
    const hint = document.createElement("div");
    hint.className = "wizard-field-hint";
    hint.textContent = f.hint;
    wrap.appendChild(hint);
  }
  return wrap;
}

// collectWizardFields reads the current form's values into wizardData.
function collectWizardFields() {
  modelWizardBody.querySelectorAll("[data-key]").forEach((el) => {
    const key = el.dataset.key;
    if (el.type === "checkbox") {
      // Toggle switch / checkbox: checked = true.
      wizardData[key] = el.checked;
    } else if (el.type === "number") {
      wizardData[key] = parseInt(el.value, 10) || 0;
    } else {
      wizardData[key] = el.value;
    }
  });
}

// Next: collect the current step's values and advance. When moving into the
// capabilities step, show a "detecting" placeholder while the backend runs the
// capability detection (FEATURE-429).
modelWizardNext.onclick = () => {
  collectWizardFields();
  if (wizardStep === "model_name") {
    // Next step is capabilities: show a detecting placeholder.
    modelWizardBody.textContent = "";
    const det = document.createElement("div");
    det.className = "wizard-step-title";
    det.textContent = "正在检测模型能力...";
    modelWizardBody.appendChild(det);
  }
  wsSend({ type: "model_wizard_next", step: wizardStep, wizard_data: wizardData });
};

// Prev: go back one step.
modelWizardPrev.onclick = () => {
  wsSend({ type: "model_wizard_prev", step: wizardStep, wizard_data: wizardData });
};

// Submit: collect the last step's values and finish.
modelWizardSubmit.onclick = () => {
  collectWizardFields();
  wsSend({ type: "model_wizard_submit", wizard_data: wizardData });
};

// Compatibility stubs for the legacy text-flow wizard routing (FEATURE-422).
// The structured wizard (FEATURE-429) no longer emits ui_text/ask/interaction,
// but these keep the wizardActive routing branches safe if they ever fire.
function appendWizardText(text) {
  const line = document.createElement("div");
  line.className = "wizard-line";
  line.textContent = text;
  modelWizardBody.appendChild(line);
  modelWizardBody.scrollTop = modelWizardBody.scrollHeight;
}
function showWizardAsk(msg) {
  const wrap = document.createElement("div");
  wrap.className = "wizard-ask";
  const inp = document.createElement("input");
  inp.type = "text";
  inp.autocomplete = "off";
  const send = document.createElement("button");
  send.className = "btn";
  send.textContent = T.send || "发送";
  const submit = () => { wsSend({ type: "answer", id: msg.id, value: inp.value }); wrap.remove(); };
  send.onclick = submit;
  inp.addEventListener("keydown", (e) => { if (e.key === "Enter" && !imeComposing(e)) { e.preventDefault(); submit(); } });
  wrap.appendChild(inp);
  wrap.appendChild(send);
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
    // FEATURE-455: remote access + download flags. When remote and download
    // are both enabled, the file list's reveal icon becomes a download icon.
    remoteAccess = !!b.remote;
    downloadEnabled = !!b.downloadEnabled;
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


/* ---------- FEATURE-460: supplement input helpers ---------- */

// getSupplement returns the trimmed text typed in the main input box, or ""
// if none (FEATURE-462: the dedicated supplement box was removed; the main
// input box is reused for supplementary info).
function getSupplement() {
  return input.value.trim();
}

// sendSupplement sends the user's typed supplement as the interaction answer.
// If nothing was typed, it just focuses the main input box (FEATURE-462).
function sendSupplement() {
  const sup = getSupplement();
  if (sup) {
    answerInteraction({ action: "input", value: sup });
  } else {
    enterSupplementMode();
  }
}

// answerSelectWithSupplement answers a select option, appending the user's
// typed supplement (option + "，" + supplement) when present (FEATURE-460).
function answerSelectWithSupplement(opt) {
  const sup = getSupplement();
  if (sup) {
    answerInteraction({ action: "select", value: opt + "，" + sup });
  } else {
    answerInteraction({ action: "select", value: opt });
  }
}

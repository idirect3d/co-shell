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
    sbSession: "Σ", sbLast: "🔄",
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
    sbSession: "Σ", sbLast: "🔄",
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
const tree = document.getElementById("tree");
const sidebar = document.getElementById("sidebar");
const menuBtn = document.getElementById("menuBtn");
const miWs = document.getElementById("miWs");
const miPlan = document.getElementById("miPlan");
const miWsCheck = document.getElementById("miWsCheck");
const miPlanCheck = document.getElementById("miPlanCheck");
const miSettings = document.getElementById("miSettings");
const miRestart = document.getElementById("miRestart");
const settingsModal = document.getElementById("settings");
const settingsClose = document.getElementById("settingsClose");
const settingsBody = document.getElementById("settingsBody");
const settingsDynamic = document.getElementById("settingsDynamic");
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
const miStatus = document.getElementById("miStatus");
const miStatusCheck = document.getElementById("miStatusCheck");
const statusbar = document.getElementById("statusbar");
const sbModel = document.getElementById("sbModel");
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

function scrollStream() { stream.scrollTop = stream.scrollHeight; }

function newStreamBlock(cls, label, msgIndex) {
  return { body: makeBlock(cls, label, msgIndex), raw: "", raf: 0, hasResult: false };
}

// ensureToolParams creates (or returns) the input-parameter sub-block inside a
// TOOL block (FEATURE-400). The sub-block has a title bar ("输入参数"), a
// collapse/expand toggle in the top-right, and a scrollable body at fixed height.
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
  const toggle = document.createElement("button");
  toggle.className = "tool-params-toggle";
  toggle.textContent = "⤢";
  toggle.title = "展开/固定高度";
  toggle.onclick = () => {
    params.classList.toggle("expanded");
    toggle.textContent = params.classList.contains("expanded") ? "⤡" : "⤢";
  };
  head.appendChild(title);
  head.appendChild(toggle);
  const body = document.createElement("div");
  body.className = "tool-params-body";
  params.appendChild(head);
  params.appendChild(body);
  // Insert after the ev-head, before the ev-body.
  box.insertBefore(params, curTool.body);
  curTool.params = { body, raw: "" };
  return curTool.params;
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
  stream.appendChild(box);
  scrollStream();
  return body;
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
// (FEATURE-409).
function unmarkStreaming(body) {
  if (!body) return;
  const box = body.parentElement;
  if (!box) return;
  const s = box.querySelector(".ev-streaming");
  if (s) s.classList.remove("on");
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
    const parts = [];
    if (m.prompt) parts.push("↑" + m.prompt);
    if (m.completion) parts.push("↓" + m.completion);
    if (m.total) parts.push("Σ" + m.total + (m.max && m.max !== "0" ? "/" + m.max : ""));
    if (m.ft) parts.push(m.ft);
    if (m.out_tps) parts.push(m.out_tps);
    const line = document.createElement("div");
    line.className = "ev meta";
    line.textContent = parts.join("  ");
    stream.appendChild(line);
    curLLM = curThinking = null;
    // FEATURE-409: the LLM iteration ended (token usage refreshed) — hide the
    // streaming "..." on all blocks now, not only at the final done event.
    document.querySelectorAll(".ev-streaming").forEach((s) => s.classList.remove("on"));
    scrollStream();
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
    if (!curTool) curTool = newStreamBlock("tool", "TOOL", msgIndex);
    markStreaming(curTool.body);
    const params = ensureToolParams(curTool);
    params.raw += ev.text || "";
    // FEATURE-409: render the streaming args as markdown (lists, code, etc.)
    // instead of a plain text blob; md.js is streaming-safe.
    params.body.classList.add("md");
    mdRender(params.body, params.raw);
    // Keep the params sub-block scrolled to the last line as streaming args
    // accumulate past its fixed height.
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
  const body = makeBlock("user-msg", "YOU", lastMsgIndex);
  body.textContent = text;
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
  let modelHtml = "";
  if (modelInfo && modelInfo.textModel) {
    modelHtml += "🧠" + modelInfo.textModel + "(" + fmtPct(lastTotal, modelInfo.textMaxLen) + " of " + fmtLen(modelInfo.textMaxLen) + ")";
    if (modelInfo.visionModel) {
      modelHtml += " 👀" + modelInfo.visionModel + "(" + fmtPct(lastTotal, modelInfo.visionMaxLen) + " of " + fmtLen(modelInfo.visionMaxLen) + ")";
    }
  }
  sbModel.innerHTML = modelHtml;
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
    const key = e.key.toLowerCase();
    if (numberMode) {
      // Number-choice mode: a digit picks the approve-count.
      if (/^[0-9]$/.test(key)) {
        e.preventDefault();
        const n = key === "0" ? 10 : parseInt(key, 10);
        answerInteraction({ action: "approve_count", value: String(n) });
      }
      return;
    }
    if (key === " ") {
      e.preventDefault();
      enterSupplementMode();
    } else if (key === "enter") {
      e.preventDefault();
      answerInteraction({ action: "approve" });
    } else if (keyMap[key]) {
      e.preventDefault();
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
}

sendBtn.onclick = () => { if (running) wsSend({ type: "interrupt" }); else sendInput(); };

// recallHistory swaps the textarea content with the history entry at
// histPos (or the saved draft when histPos points past the newest entry)
// and parks the cursor at the end.
function recallHistory() {
  input.value = histPos < history.length ? history[histPos] : histDraft;
  autoGrow();
  input.selectionStart = input.selectionEnd = input.value.length;
}

input.addEventListener("keydown", (e) => {
  // FEATURE-409: while an interaction is pending (tool confirm / ask select /
  // cancel), the virtual-keyboard handler on window already responds to the
  // target keys. Swallow those keys here too so they don't leak into the input
  // box as stray characters (the window handler runs in the bubble phase, after
  // the textarea's own default insertion).
  if (pendingInteraction && !supplementMode) {
    const k = e.key.toLowerCase();
    const isTarget = k === "enter" || k === " " || /^[0-9]$/.test(k) || /^[a-z]$/.test(k);
    // Swallow the key so it doesn't leak into the input box, but let it keep
    // bubbling so the window-level __vkHandler still responds to it.
    if (isTarget) { e.preventDefault(); return; }
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

  // FEATURE-380: git status letter is a direct child of the li (.tree-node),
  // absolutely positioned against the .tree container so it hugs the file
  // list's left edge (not the file name). Files show their status letter
  // (M/A/D/R/U); directories leave it blank so every row keeps the same left
  // gutter and horizontal alignment is unaffected.
  const status = document.createElement("span");
  status.className = "git-status";
  if (node.status) {
    status.textContent = node.status;
    status.title = node.status;
    status.classList.add("st-" + node.status.toLowerCase());
  }
  li.appendChild(status);

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
    row.onclick = () => openFile(node);
  }
  return li;
}

const IMAGE_EXT = /\.(png|jpe?g|gif|webp|bmp|svg)$/i;

function openFile(node) {
  if (IMAGE_EXT.test(node.name)) {
    previewImg.src = "/api/file?path=" + encodeURIComponent(node.path);
    preview.classList.remove("hidden");
  } else {
    postPath("/api/open", node.path);
  }
}

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
      modelInfo = { textModel: b.textModel, textMaxLen: b.textMaxLen || 0, visionModel: b.visionModel || "", visionMaxLen: b.visionMaxLen || 0 };
    }
  } catch { /* defaults stay zh */ }
  applyI18n();
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

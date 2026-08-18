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
    workspace: "工作区", attach: "附件",
    taskPlan: "任务进展", reply: "回复", interrupt: "打断", send: "发送",
    inputHint: "输入指令，Enter 发送，Shift+Enter 换行，↑↓ 历史",
    connected: "已连接", disconnected: "已断开",
    askLine: "代理请求一行输入：", askKey: "代理请求按键确认：",
    uploadFailed: "上传失败", actionFailed: "操作失败",
    planEmpty: "（无步骤）",
  },
  en: {
    workspace: "Workspace", attach: "Attach",
    taskPlan: "Task Plan", reply: "Reply", interrupt: "Interrupt", send: "Send",
    inputHint: "Type a command — Enter to send, Shift+Enter for newline, ↑↓ history",
    connected: "connected", disconnected: "disconnected",
    askLine: "The agent asks for a line of input:", askKey: "The agent asks for a key:",
    uploadFailed: "Upload failed", actionFailed: "Action failed",
    planEmpty: "(no steps)",
  },
};
let T = I18N.zh;

function applyI18n() {
  document.querySelectorAll("[data-i18n]").forEach((el) => {
    const k = el.getAttribute("data-i18n");
    if (T[k]) el.textContent = T[k];
  });
  document.querySelectorAll("[data-i18n-ph]").forEach((el) => {
    const k = el.getAttribute("data-i18n-ph");
    if (T[k]) el.placeholder = T[k];
  });
  connText.textContent = wsReady ? T.connected : T.disconnected;
  attachBtn.title = T.attach;
}

/* ---------- theme ---------- */

const themeToggle = document.getElementById("themeToggle");

function setTheme(name) {
  document.documentElement.setAttribute("data-theme", name);
  localStorage.setItem("co-shell-theme", name);
  themeToggle.textContent = name === "dark" ? "☾" : "☀";
}

(function initTheme() {
  const saved = localStorage.getItem("co-shell-theme");
  if (saved === "dark" || saved === "light") return setTheme(saved);
  // No manual choice: follow the OS color scheme. When the browser cannot
  // report one (no matchMedia), fall back to dark (FIX-363).
  const dark = !window.matchMedia || window.matchMedia("(prefers-color-scheme: dark)").matches;
  setTheme(dark ? "dark" : "light");
})();

themeToggle.onclick = () => {
  const cur = document.documentElement.getAttribute("data-theme");
  setTheme(cur === "dark" ? "light" : "dark");
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
const input = document.getElementById("input");
const sendBtn = document.getElementById("sendBtn");
const interruptBtn = document.getElementById("interruptBtn");
const chips = document.getElementById("chips");
const tree = document.getElementById("tree");
const sidebar = document.getElementById("sidebar");
const attachBtn = document.getElementById("attachBtn");
const fileInput = document.getElementById("fileInput");
const preview = document.getElementById("preview");
const previewImg = document.getElementById("previewImg");
const previewClose = document.getElementById("previewClose");

/* ---------- websocket ---------- */

let ws = null;
let wsReady = false;

function wsConnect() {
  ws = new WebSocket("ws://" + location.host + "/ws");
  ws.onopen = () => {
    wsReady = true;
    conn.classList.add("on");
    connText.textContent = T.connected;
  };
  ws.onclose = () => {
    wsReady = false;
    conn.classList.remove("on");
    connText.textContent = T.disconnected;
    hideAsk();
    setTimeout(wsConnect, 2000);
  };
  ws.onmessage = (m) => {
    let msg;
    try { msg = JSON.parse(m.data); } catch { return; }
    if (msg.kind === "event" && msg.event) renderEvent(msg.event);
    else if (msg.kind === "ask") showAsk(msg);
    else if (msg.kind === "state") renderPlan(msg.plan || null);
  };
}

function wsSend(obj) {
  if (wsReady) ws.send(JSON.stringify(obj));
}

/* ---------- event stream rendering ---------- */

const CHAN_LABEL = {
  llm: "LLM", tool: "TOOL", command: "CMD", system: "SYS",
  taskplan: "PLAN", memory: "MEM", mcp: "MCP", db: "DB",
  wizard: "WIZ", debug: "DBG", bridge: "BRG", subagent: "SUB",
};

// Streaming blocks: { body, raw, raf, hasResult? }. raw accumulates the
// undecorated text; the body is re-rendered (markdown) via rAF throttle.
let curLLM = null;      // current streaming llm block
let curThinking = null; // current streaming thinking block
let curTool = null;     // current tool block (one block per invocation)

function scrollStream() { stream.scrollTop = stream.scrollHeight; }

function newStreamBlock(cls, label) {
  return { body: makeBlock(cls, label), raw: "", raf: 0, hasResult: false };
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

function makeBlock(cls, label) {
  const box = document.createElement("div");
  box.className = "ev " + cls;
  const head = document.createElement("div");
  head.className = "ev-head";
  head.textContent = label;
  const body = document.createElement("div");
  body.className = "ev-body";
  box.appendChild(head);
  box.appendChild(body);
  stream.appendChild(box);
  scrollStream();
  return body;
}

function eventClass(ev) {
  const lvl = ev.level ? " level-" + ev.level : "";
  switch (ev.type) {
    case "content_chunk": case "content": return "llm" + lvl;
    case "thinking_chunk": case "thinking": return "thinking" + lvl;
    case "tool_call": case "tool_call_stream": return "tool" + lvl;
    case "command": case "output": return "command" + lvl;
    default: return "system" + lvl;
  }
}

function renderEvent(ev) {
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
    scrollStream();
    return;
  }
  if (ev.type === "done") { curLLM = curThinking = curTool = null; return; }

  const streaming = ev.type === "content_chunk" || ev.type === "thinking_chunk";
  if (streaming) {
    if (ev.type === "content_chunk") {
      if (!curLLM) { curLLM = newStreamBlock("llm", "LLM"); curThinking = null; curTool = null; }
      curLLM.raw += ev.text || "";
      scheduleMd(curLLM);
    } else {
      if (!curThinking) { curThinking = newStreamBlock("thinking", "THINK"); curLLM = null; curTool = null; }
      curThinking.raw += ev.text || "";
      scheduleMd(curThinking);
    }
    scrollStream();
    return;
  }

  // FEATURE-362: one block per tool invocation. Streaming arg fragments
  // accumulate; the input summary replaces them; the result appends.
  if (ev.type === "tool_call_stream") {
    if (!curTool) curTool = newStreamBlock("tool", "TOOL");
    curTool.raw += ev.text || "";
    curTool.body.classList.remove("md");
    curTool.body.textContent = curTool.raw; // partial args stay plain while typing
    scrollStream();
    return;
  }
  if (ev.type === "tool_call" || (ev.type === "error" && ev.chan === "tool")) {
    const phase = ev.meta && ev.meta.phase;
    const fresh = !curTool || curTool.hasResult;
    if (phase === "input" || (!phase && ev.type === "tool_call" && !fresh && !curTool.raw)) {
      // pre-execution summary: replace the raw streamed fragments
      if (fresh) curTool = newStreamBlock("tool", "TOOL");
      curTool.raw = ev.text || "";
      curTool.hasResult = false;
      curTool.body.classList.remove("md");
      curTool.body.textContent = curTool.raw;
    } else {
      // result / tool error: append into the same block
      if (!curTool) curTool = newStreamBlock("tool", "TOOL");
      if (ev.type === "error") curTool.body.parentElement.classList.add("level-error");
      const t = (ev.text || "").replace(/^\s*Result:\n/, "");
      curTool.raw += (curTool.raw ? "\n\n" : "") + t;
      curTool.hasResult = true;
      scheduleMd(curTool);
    }
    curLLM = curThinking = null;
    scrollStream();
    return;
  }

  curLLM = curThinking = null;
  const label = CHAN_LABEL[ev.chan] || (ev.chan || "SYS").toUpperCase();
  const body = makeBlock(eventClass(ev), ev.type === "ui_text" ? "SYS" : label);
  if (ev.type === "content" || ev.type === "thinking") {
    body.classList.add("md");
    mdRender(body, ev.text || "");
  } else {
    body.textContent = ev.text || "";
  }
}

function renderUserEcho(text, attachments) {
  const body = makeBlock("user-msg", "YOU");
  body.textContent = text + (attachments && attachments.length ? "\n📎 " + attachments.join(", ") : "");
}

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
  if (!plan || !plan.steps || plan.steps.length === 0) {
    planPanel.classList.add("hidden");
    layout.classList.add("no-plan");
    return;
  }
  planPanel.classList.remove("hidden");
  layout.classList.remove("no-plan");
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
    desc.textContent = st.description || "";
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
}

function answerAsk(value) {
  if (!pendingAsk) return;
  wsSend({ type: "answer", id: pendingAsk, value });
  hideAsk();
}

function hideAsk() {
  pendingAsk = null;
  askArea.classList.add("hidden");
}

askSend.onclick = () => answerAsk(askInput.value);
askInput.addEventListener("keydown", (e) => {
  if (e.key === "Enter") { e.preventDefault(); answerAsk(askInput.value); }
});

/* ---------- input row / history ---------- */

const history = [];
let histPos = -1;
let histDraft = "";
let attachments = [];

function renderChips() {
  chips.textContent = "";
  for (const p of attachments) {
    const chip = document.createElement("span");
    chip.className = "chip";
    const name = document.createElement("span");
    name.textContent = p;
    const x = document.createElement("button");
    x.textContent = "✕";
    x.onclick = () => { attachments = attachments.filter((a) => a !== p); renderChips(); };
    chip.appendChild(name);
    chip.appendChild(x);
    chips.appendChild(chip);
  }
}

function sendInput() {
  const text = input.value.trim();
  if (!text && attachments.length === 0) return;
  wsSend({ type: "input", text, attachments: attachments.slice() });
  renderUserEcho(text, attachments);
  if (text) { history.push(text); histPos = history.length; }
  attachments = [];
  renderChips();
  input.value = "";
  autoGrow();
}

function autoGrow() {
  input.style.height = "auto";
  input.style.height = Math.min(input.scrollHeight, 120) + "px";
}

sendBtn.onclick = sendInput;
interruptBtn.onclick = () => wsSend({ type: "interrupt" });

input.addEventListener("keydown", (e) => {
  if (e.key === "Enter" && !e.shiftKey) { e.preventDefault(); sendInput(); return; }
  if (e.key === "ArrowUp" && input.selectionStart === 0) {
    if (histPos === -1 || histPos === history.length) histDraft = input.value;
    if (histPos > 0) { histPos--; input.value = history[histPos]; autoGrow(); e.preventDefault(); }
  } else if (e.key === "ArrowDown") {
    if (histPos >= 0 && histPos < history.length - 1) { histPos++; input.value = history[histPos]; autoGrow(); }
    else if (histPos === history.length - 1) { histPos = history.length; input.value = histDraft; autoGrow(); }
  }
});
input.addEventListener("input", autoGrow);

/* ---------- workspace tree ---------- */

async function loadTree() {
  try {
    const resp = await fetch("/api/tree");
    const root = await resp.json();
    tree.textContent = "";
    const ul = document.createElement("ul");
    for (const c of root.children || []) ul.appendChild(treeNode(c));
    tree.appendChild(ul);
  } catch { /* keep old tree */ }
}

function treeNode(node) {
  const li = document.createElement("li");
  li.className = "tree-node";
  const row = document.createElement("div");
  row.className = "tree-row" + (node.dir ? " dir" : "");

  const tw = document.createElement("span");
  tw.className = "tw";
  tw.textContent = node.dir ? "▸" : "";
  row.appendChild(tw);

  const name = document.createElement("span");
  name.className = "name";
  name.textContent = node.name;
  row.appendChild(name);

  const reveal = document.createElement("button");
  reveal.className = "reveal-btn";
  reveal.title = "reveal";
  reveal.textContent = "⌖";
  reveal.onclick = (e) => { e.stopPropagation(); postPath("/api/reveal", node.path); };
  row.appendChild(reveal);

  li.appendChild(row);

  if (node.dir) {
    const ul = document.createElement("ul");
    ul.style.display = "none";
    for (const c of node.children || []) ul.appendChild(treeNode(c));
    li.appendChild(ul);
    row.onclick = () => {
      const open = ul.style.display !== "none";
      ul.style.display = open ? "none" : "";
      tw.textContent = open ? "▸" : "▾";
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

attachBtn.onclick = () => fileInput.click();
fileInput.onchange = async () => {
  const paths = await uploadFiles(fileInput.files, "");
  fileInput.value = "";
  attachments = attachments.concat(paths);
  renderChips();
};

document.getElementById("treeRefresh").onclick = loadTree;

/* ---------- logo mosaic ---------- */

// Pixel mosaic of the co-shell mascot (a little clam: upper/lower shell
// halves with two big eyes on the body between them), hand-drawn on an
// 8 rows x 16 cols grid. Intensity chars map to accent-color opacity;
// spaces stay transparent.
const LOGO_ART = [
  "     ######",
  "   ###    ###",
  " ###        ###",
  "     %%  %%",
  "     %%  %%",
  " ###        ###",
  "   ##########",
];
const LOGO_OPACITY = { "=": 0.35, "+": 0.55, "*": 0.75, "#": 0.9, "%": 1 };

(function renderLogo() {
  const logo = document.getElementById("logo");
  const rows = LOGO_ART.length;
  const cols = Math.max(...LOGO_ART.map((l) => l.length));
  // Square cells (uniform scaling keeps the logo's aspect ratio), sized up
  // to 3px but shrunk to fit the sidebar width and to keep the mosaic no
  // taller than the user input box.
  const maxW = sidebar.clientWidth ? sidebar.clientWidth - 36 : 204;
  const maxH = (input.offsetHeight || 38) - 10;
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

/* ---------- bootstrap ---------- */

(async function boot() {
  try {
    const resp = await fetch("/api/bootstrap");
    const b = await resp.json();
    if (b.lang === "en") T = I18N.en;
    document.documentElement.lang = b.lang || "zh";
    document.getElementById("ver").textContent = "v" + b.version + " [BUILD-" + b.build + "]";
  } catch { /* defaults stay zh */ }
  applyI18n();
  loadTree();
  wsConnect();
})();

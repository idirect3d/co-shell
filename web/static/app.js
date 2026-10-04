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
    planAcceptance: "验收标准",
    menu: "菜单", settings: "系统设置", identity: "身份与个性", restart: "重启后台",
    identityCatAgent: "智能体", identityCatUser: "用户",
    identityCatPrinciples: "核心原则",
    identityCatCapabilities: "能力", identityCatRules: "规则", save: "保存",
    identitySaved: "{name}已保存",
    // FEATURE-546: status-bar usage popovers
    usageTotalTitle: "历史用量统计",
    usageLastTitle: "本轮用量",
    usageRequests: "请求次数",
    usagePrompt: "输入 tokens",
    usageCompletion: "输出 tokens",
    usageTotal: "总 tokens",
    usageCached: "命中缓存 tokens",
    usageRange: "统计区间",
    usageCacheRate: "KV cache 命中率",
    usageCacheUnavailable: "引擎未上报 cached_tokens（需 vLLM --enable-prefix-caching）",
    usageMTP: "MTP 命中率",
    usageMTPNotConfigured: "未配置 metrics_endpoint，无法获取 MTP 命中率（在模型配置中填写推理引擎 /metrics 地址）",
    usageMTPFetchFailed: "metrics 端点不可达，无法获取 MTP 命中率（检查地址与网络）",
    usageMTPNoSpec: "引擎未上报 spec_decode 指标（需 vLLM --speculative-config 开启 MTP）",
    usageMTPNoDraft: "尚无 draft token 统计（引擎启动后还未跑过投机解码）",
    usageTrendDay: "按天 token 用量",
    usageTrendHour: "按小时 token 用量",
    usageRateTrend: "按天平均速率（tokens/s）",
    usageSeriesPrompt: "输入",
    usageSeriesCompletion: "输出",
    usageSeriesInTPS: "输入速率",
    usageSeriesOutTPS: "输出速率",
    usageInTPS: "输入速率",
    usageOutTPS: "输出速率",
    usageFirstToken: "首 token 用时",
    usageOutDur: "输出生成用时",
    usageLoading: "加载中…",
    usageEmpty: "暂无历史用量记录",
    usageFailed: "用量数据读取失败",
    usageApproxNote: "非流式请求的速率以墙钟总耗时近似，含网络往返。",
    identityNavHint: "本配置中的数值与系统提示词相关：修改可能降低大模型请求缓存命中率、增加费用，并延长变更后首次访问的预处理时间，请谨慎修改。",
    identityHintName: "命名当前 co-shell 实例，会注入系统提示词，可能影响 LLM 的行为，也可用于在 co-shell-hub 中辨识当前实例。",
    identityHintDescription: "描述当前 co-shell 实例的职责，会注入系统提示词，可能影响 LLM 的行为，也可用于在 co-shell-hub 中向其他 co-shell 实例介绍自己的能力。",
    identityHintUserName: "用于 co-shell 辨识你，可能轻微影响 LLM 的行为。",
    identityHintUserDescription: "用于 co-shell 了解你的岗位、职责、偏好等，以便更好的为你提供帮助，可能直接影响 LLM 的行为。",
    identityHintPrinciples: "co-shell 的核心工作原则，直接影响 LLM 的行为。",
    identityHintCapabilities: "让 co-shell 了解自己的能力，直接影响 LLM 的行为。",
    identityHintRules: "co-shell 的做事规则，尽量不要修改/删除 {} 占位符，直接影响 LLM 的行为。",
    appearance: "[ 外观 ]",
    themeMode: "主题", themeAuto: "跟随系统", themeDark: "深色", themeLight: "浅色",
    statusBar: "状态条", streamTitle: "会话标题", msgViz: "消息可视化",
    revealDir: "定位到文件夹",
    downloadFile: "下载文件",
    treeLoading: "加载中…",
    sessionDelete: "删除会话",
    sessionSwitchBusy: "任务运行中，无法切换会话",
    sessionActive: "当前会话",
    sessionCount: "消息计数",
    sessionDeleteConfirm: "确定要删除会话「%s」吗？此操作不可撤销。",
    modelDeleteConfirm: "确定要删除模型「%s」吗？此操作不可撤销。",
    approveCount: "批准N次",
    approve: "批准", approveAll: "全部批准", approveG: "永久自动执行", approveD: "永久禁用",
    supplement: "补充信息", supplementHint: "长按按钮或单击输入框可补充信息",
    qsSubmit: "提交", qsSubmitCount: "(%d/%d)", qsNote: "补充说明", qsNotePlaceholder: "补充说明（可选）",
    qsMulti: "多选", qsSingle: "单选", qsFreePlaceholder: "请输入你的回答…",
    answerTag: "回答", qsFreeNotePlaceholder: "针对本题补充说明（可作为答案，回车确认）",
    qsConfirm: "确定", qsRequired: "必填", qsRequiredMissing: "必填题未作答",
    qsHintDigitSelect: "数字键选择", qsHintHoldNote: "长按选项数字键填备注",
    qsHintNext: "下一题", qsHintPrev: "上一题", qsHintSpace: "备注",
    qsKeyEnter: "回车", qsKeySpace: "空格",
    qsChoiceLabel: "选择", qsAnswerLabel: "回答", qsNoteLabel: "备注", qsNoAnswer: "（未作答）",
    numberHint: "按数字键选择放行次数（0=10次）",
    cancel: "取消", confirm: "确认",
    copyBlock: "复制内容", collapseBlock: "收起同类块", expandBlock: "展开同类块", retryFrom: "从此处重新运行",
    retryFromFailed: "回退失败：%s",
    switchMode: "切换工作模式",
    models: "模型管理", modelAdd: "＋ 新增模型", modelWizard: "模型配置向导", templateJson: "查看模板原始 JSON", reasoningEffortNone: "不设置", apiTypeChatDefault: "chat（默认）",
    wizardDetect: "手动检测", wizardDetecting: "检测中...", wizardCapSupported: "支持", wizardCapUnsupported: "不支持",
    modelEmpty: "暂无模型，点击上方「＋ 新增模型」添加", modelMenuTitle: "选择主模型", modelVisionMenuTitle: "选择视觉模型", modelVisionEmpty: "暂无视觉模型", modelDefault: "默认", modelDefaultHint: "使用全局默认模型", modelRestoreDefault: "默认", modelAutoSelect: "自动选择", modelAutoSelectHint: "LLM 调用失败时按优先级自动降级到后续模型",
    fileViewerClose: "关闭", fileViewerLoadFailed: "文件读取失败",
    fileViewerSearch: "搜索文件内容…", fileViewerRaw: "Raw",
    streamModeSilent: "静默", streamModeMinimal: "极简", streamModeNormal: "正常",
    streamTitlePlaceholder: "会话标题", streamTitleHint: "点击修改会话标题",
    stcGoal: "目标", stcRunning: "正在执行", stcProgress: "进展情况", stcDone: "已完成",
    yoloTitle: "YOLO 模式（You Only Live Once）：开启后所有工具调用自动批准，无需逐个确认",
    setDefaultTip: "默认值", setDiffTip: "与默认值不一致",
    // FEATURE-583: loaded-skills panel + Skills settings page
    skillsLoaded: "已加载技能",
    skillsSettings: "技能",
    skillsSettingsHint: "管理技能的启用状态；未单独配置的项跟随默认。",
    skillsDefaultEnabled: "默认启用",
    skillsDefaultHint: "对新增或未配置过的 skill 生效",
    skillFollowDefault: "跟随默认",
    skillLoadedTag: "已加载",
    skillSourceWorkspace: "工作区",
    skillSourceGlobal: "全局",
    skillsEmpty: "未发现技能",
    skillsLoading: "加载中…",
    skillsSaveFailed: "技能设置保存失败",
    skillsLoadFailed: "技能列表加载失败",
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
    planAcceptance: "Acceptance criteria",
    menu: "Menu", settings: "Settings", identity: "Identity & Personality", restart: "Restart backend",
    identityCatAgent: "Agent", identityCatUser: "User",
    identityCatPrinciples: "Principles",
    identityCatCapabilities: "Capabilities", identityCatRules: "Rules", save: "Save",
    identitySaved: "{name} saved",
    // FEATURE-546: status-bar usage popovers
    usageTotalTitle: "Lifetime usage",
    usageLastTitle: "This turn",
    usageRequests: "Requests",
    usagePrompt: "Prompt tokens",
    usageCompletion: "Completion tokens",
    usageTotal: "Total tokens",
    usageCached: "Cached tokens",
    usageRange: "Range",
    usageCacheRate: "KV cache hit rate",
    usageCacheUnavailable: "Engine did not report cached_tokens (requires vLLM --enable-prefix-caching)",
    usageMTP: "MTP acceptance rate",
    usageMTPNotConfigured: "metrics_endpoint is not configured, MTP rate unavailable (set the engine /metrics URL in the model config)",
    usageMTPFetchFailed: "metrics endpoint unreachable, MTP rate unavailable (check the address and network)",
    usageMTPNoSpec: "Engine reports no spec_decode metrics (requires vLLM --speculative-config)",
    usageMTPNoDraft: "No draft tokens counted yet (speculative decoding has not run since start)",
    usageTrendDay: "Daily token usage",
    usageTrendHour: "Hourly token usage",
    usageRateTrend: "Daily average speed (tokens/s)",
    usageSeriesPrompt: "Prompt",
    usageSeriesCompletion: "Completion",
    usageSeriesInTPS: "Input speed",
    usageSeriesOutTPS: "Output speed",
    usageInTPS: "Input speed",
    usageOutTPS: "Output speed",
    usageFirstToken: "Time to first token",
    usageOutDur: "Generation time",
    usageLoading: "Loading…",
    usageEmpty: "No usage records yet",
    usageFailed: "Failed to load usage data",
    usageApproxNote: "Non-streaming speeds approximate with wall-clock duration, including network round-trip.",
    identityNavHint: "These values are part of the system prompt. Changing them may lower the LLM request cache hit rate, increase cost, and lengthen preprocessing on the first request after the change - edit with care.",
    identityHintName: "Names this co-shell instance. Injected into the system prompt, may affect LLM behavior, and also identifies this instance in co-shell-hub.",
    identityHintDescription: "Describes this co-shell instance's responsibility. Injected into the system prompt, may affect LLM behavior, and also introduces its capabilities to other co-shell instances in co-shell-hub.",
    identityHintUserName: "Lets co-shell identify you; may slightly affect LLM behavior.",
    identityHintUserDescription: "Tells co-shell your role, duties and preferences so it can help you better; may directly affect LLM behavior.",
    identityHintPrinciples: "co-shell's core working principles; directly affects LLM behavior.",
    identityHintCapabilities: "Lets co-shell know its own capabilities; directly affects LLM behavior.",
    identityHintRules: "co-shell's working rules. Try not to modify/delete {} placeholders; directly affects LLM behavior.",
    appearance: "[ Appearance ]",
    themeMode: "Theme", themeAuto: "Follow system", themeDark: "Dark", themeLight: "Light",
    statusBar: "Status bar", streamTitle: "Session title", msgViz: "Message viz",
    revealDir: "Reveal in folder",
    downloadFile: "Download file",
    treeLoading: "Loading…",
    sessionDelete: "Delete session",
    sessionSwitchBusy: "A task is running — cannot switch sessions",
    sessionActive: "Current session",
    sessionCount: "Message count",
    sessionDeleteConfirm: "Delete session \"%s\"? This cannot be undone.",
    modelDeleteConfirm: "Delete model \"%s\"? This cannot be undone.",
    approveCount: "Approve N times",
    approve: "Approve", approveAll: "Approve all", approveG: "Always auto-execute", approveD: "Permanently disable",
    supplement: "Supplement", supplementHint: "Long-press a button or click the input box to supplement",
    qsSubmit: "Submit", qsSubmitCount: "(%d/%d)", qsNote: "Add note", qsNotePlaceholder: "Add a note (optional)",
    qsMulti: "multi-select", qsSingle: "single choice", qsFreePlaceholder: "Type your answer…",
    answerTag: "Answer", qsFreeNotePlaceholder: "Add a note for this question (Enter to confirm)",
    qsConfirm: "Confirm", qsRequired: "Required", qsRequiredMissing: "Required question unanswered",
    qsHintDigitSelect: "digit keys select", qsHintHoldNote: "hold an option digit for its note",
    qsHintNext: "next", qsHintPrev: "previous", qsHintSpace: "note",
    qsKeyEnter: "Enter", qsKeySpace: "Space",
    qsChoiceLabel: "Selected", qsAnswerLabel: "Answer", qsNoteLabel: "Note", qsNoAnswer: "(not answered)",
    numberHint: "Press a digit to choose approve-count (0=10)",
    cancel: "Cancel", confirm: "Confirm",
    copyBlock: "Copy content", collapseBlock: "Collapse same-type blocks", expandBlock: "Expand same-type blocks", retryFrom: "Retry from here",
    retryFromFailed: "Retry-from failed: %s",
    switchMode: "Switch work mode",
    models: "Model Manager", modelAdd: "＋ Add Model", modelWizard: "Model Setup Wizard", templateJson: "View template raw JSON", reasoningEffortNone: "Not set", apiTypeChatDefault: "chat (default)",
    wizardDetect: "Run detection", wizardDetecting: "Detecting...", wizardCapSupported: "supported", wizardCapUnsupported: "not supported",
    modelEmpty: "No models yet. Click「＋ Add Model」above to add one.", modelMenuTitle: "Select main model", modelVisionMenuTitle: "Select vision model", modelVisionEmpty: "No vision models", modelDefault: "Default", modelDefaultHint: "Use global default model", modelRestoreDefault: "Default", modelAutoSelect: "Auto select", modelAutoSelectHint: "Degrade to the next lower-priority model when an LLM call fails",
    fileViewerClose: "Close", fileViewerLoadFailed: "Failed to read file",
    fileViewerSearch: "Search file content…", fileViewerRaw: "Raw",
    streamModeSilent: "Silent", streamModeMinimal: "Minimal", streamModeNormal: "Normal",
    streamTitlePlaceholder: "Session title", streamTitleHint: "Click to edit session title",
    stcGoal: "Goal", stcRunning: "Running", stcProgress: "Progress", stcDone: "done",
    yoloTitle: "YOLO mode (You Only Live Once): when on, all tool calls are auto-approved without asking",
    setDefaultTip: "Default", setDiffTip: "differs from default",
    // FEATURE-583: loaded-skills panel + Skills settings page
    skillsLoaded: "Loaded skills",
    skillsSettings: "Skills",
    skillsSettingsHint: "Manage skill availability. Items without an explicit choice follow the default.",
    skillsDefaultEnabled: "Enabled by default",
    skillsDefaultHint: "Applies to newly added or unconfigured skills",
    skillFollowDefault: "Default",
    skillLoadedTag: "Loaded",
    skillSourceWorkspace: "Workspace",
    skillSourceGlobal: "Global",
    skillsEmpty: "No skills found",
    skillsLoading: "Loading…",
    skillsSaveFailed: "Failed to save skill settings",
    skillsLoadFailed: "Failed to load skill list",
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
// "dark-muted" | "light" | "light-tp" | "paper". setTheme only applies;
// persistence is the caller's job so that "auto" is never clobbered by a
// resolved value.
const themeToggle = document.getElementById("themeToggle");
const osThemeMQ = window.matchMedia ? window.matchMedia("(prefers-color-scheme: dark)") : null;

// FEATURE-516: iconHTML returns an inline <svg> reference to a sprite symbol.
// Icons are vectors (not font glyphs / emoji) so they render identically on
// every OS and browser, and they inherit the current text colour through
// currentColor, which makes them theme-aware without per-theme assets.
function iconHTML(id, cls) {
  return '<svg class="ico' + (cls ? " " + cls : "") + '" aria-hidden="true"><use href="#' + id + '"/></svg>';
}

// setIcon swaps the sprite icon referenced by a host element in place.
function setIcon(host, id) {
  if (!host) return;
  const use = host.querySelector("use");
  if (use) use.setAttribute("href", "#" + id);
  else host.innerHTML = iconHTML(id);
}

// mkIcon builds the same icon as a DOM node, for callers that mix it with
// untrusted text (the text is then appended as a text node, never as HTML).
function mkIcon(id, cls) {
  const NS = "http://www.w3.org/2000/svg";
  const svg = document.createElementNS(NS, "svg");
  svg.setAttribute("class", cls ? "ico " + cls : "ico");
  svg.setAttribute("aria-hidden", "true");
  const use = document.createElementNS(NS, "use");
  use.setAttribute("href", "#" + id);
  svg.appendChild(use);
  return svg;
}

// setResultIcon replaces a status line with a vector status icon plus text.
function setResultIcon(host, id, text) {
  host.textContent = "";
  host.appendChild(mkIcon(id, "ico-inline"));
  host.appendChild(document.createTextNode(" " + text));
}

// themeIcon returns the sprite icon for a resolved tone (FEATURE-477/516/522):
// dark = moon, dark-muted = half-filled circle (low contrast), light = sun,
// light-tp = spade, paper = leaf.
function themeIcon(name) {
  if (name === "dark") return "i-theme-dark";
  if (name === "dark-muted") return "i-theme-muted";
  if (name === "light-tp") return "i-theme-tp";
  if (name === "paper") return "i-theme-eyecare";
  return "i-theme-light";
}

function setTheme(name) {
  document.documentElement.setAttribute("data-theme", name);
  setIcon(themeToggle, themeIcon(name));
}

function themeMode() {
  const saved = localStorage.getItem("co-shell-theme");
  return saved === "dark" || saved === "dark-muted" || saved === "light"
    || saved === "light-tp" || saved === "paper" ? saved : "auto";
}

function applyTheme() {
  const mode = themeMode();
  // No matchMedia (browser cannot report the OS scheme): fall back to dark
  // (FIX-363).
  let resolved;
  if (mode === "dark") resolved = "dark";
  else if (mode === "dark-muted") resolved = "dark-muted";
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
  const url = "logos/" + theme + "?t=" + Date.now();
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
  // Cycle through the five tones: dark -> dark-muted -> light -> light-tp ->
  // paper -> dark (FEATURE-477/522). Clicking always pins an explicit tone
  // (never auto).
  const next = cur === "dark" ? "dark-muted"
    : (cur === "dark-muted" ? "light"
      : (cur === "light" ? "light-tp"
        : (cur === "light-tp" ? "paper" : "dark")));
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
// FEATURE-564: sub-agent status list, rendered below the task plan.
const subAgentHead = document.getElementById("subAgentHead");
const subAgentBody = document.getElementById("subAgentBody");
let lastSubAgents = [];
// FEATURE-583: skills pinned in the current context, rendered below the
// sub-agent list in the same panel.
const skillHead = document.getElementById("skillHead");
const skillBody = document.getElementById("skillBody");
let lastSkills = [];
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
document.addEventListener("compositionstart", () => { window.__composing = true; noteInputActivity(); }, true);
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
const newSessionBtn = document.getElementById("newSessionBtn");
const identityModal = document.getElementById("identity");
const identityClose = document.getElementById("identityClose");
const identityBody = document.getElementById("identityBody");
const identityNav = document.getElementById("identityNav");
const identityFieldsEl = document.getElementById("identityFields");
const identityResult = document.getElementById("identityResult");
const identityBack = document.getElementById("identityBack");
const preview = document.getElementById("preview");
const previewClose = document.getElementById("previewClose");
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
// FEATURE-547: non-modal image previewer (name / size / mtime / resolution).
const imgViewer = document.getElementById("imgViewer");
const ivImg = document.getElementById("ivImg");
const ivTitlebar = document.getElementById("ivTitlebar");
const ivName = document.getElementById("ivName");
const ivSize = document.getElementById("ivSize");
const ivMtime = document.getElementById("ivMtime");
const ivRes = document.getElementById("ivRes");
const ivClose = document.getElementById("ivClose");
// FEATURE-425: main message area title bar (display-mode pill + session title).
const streamMode = document.getElementById("streamMode");
const streamTitle = document.getElementById("streamTitle");
// FEATURE-482: message-visualization chart (lines + pannable track).
const msgViz = document.getElementById("msgViz");
const msgVizTrack = document.getElementById("msgVizTrack");
// FEATURE-487: narrow-screen cycling session-title widget (read-only).
const streamTitleCycle = document.getElementById("streamTitleCycle");
const stcFace = document.getElementById("stcFace");
const miStatus = document.getElementById("miStatus");
const miStatusCheck = document.getElementById("miStatusCheck");
const statusbar = document.getElementById("statusbar");
const sbModelText = document.getElementById("sbModelText");
const sbModelVision = document.getElementById("sbModelVision");
const sbSession = document.getElementById("sbSession");
const sbLast = document.getElementById("sbLast");
// FEATURE-546: the two usage popovers opened from the status-bar items above.
const sbSessionPop = document.getElementById("sbSessionPop");
const sbLastPop = document.getElementById("sbLastPop");
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
  // Relative WS endpoint: works both at the root path (co-shell standalone)
  // and under a sub-path prefix (e.g. hub /agent/{id}/) (FEATURE-484).
  const wsProto = location.protocol === 'https:' ? 'wss://' : 'ws://';
  // FIX-511: keep a per-connection reference. Callbacks bound to a socket that
  // has already been superseded (disconnect → reconnect) must not touch the
  // state of the current connection, otherwise a late onclose from the old
  // socket resets wsReady and wipes the running state just restored on
  // reconnect.
  const sock = new WebSocket(wsProto + location.host + location.pathname + 'ws');
  ws = sock;
  sock.onopen = () => {
    if (sock !== ws) return; // superseded socket
    wsReady = true;
    conn.classList.add("on");
    connText.textContent = T.connected;
    // FIX-511: rebuild the view from the server's persisted history. On a page
    // load the stream is already empty (no-op); on a reconnect this drops the
    // previous content so the replay below rebuilds it instead of appending a
    // second copy of the same messages.
    resetStreamView();
    wsSend({ type: "mode_get" }); // FEATURE-410: load the work-mode list
    // Fetch the session list on connect so the 💬 count is correct immediately
    // (FEATURE-387), not only after hovering the status-bar item.
    wsSend({ type: "session_list" });
    // FEATURE-439: load the YOLO master switch state (defaults to off).
    wsSend({ type: "yolo_get" });
    // FEATURE-507: replay the tail of the persisted event stream so a refresh
    // (or a reconnect) restores the conversation instead of an empty view.
    wsSend({ type: "history_get", count: pageBufferSize });
    // FEATURE-583: restore the loaded-skills panel on connect as well (the
    // recorded skill_state events may not cover the current session tail).
    refreshSkills();
    // FEATURE-508: watch the top marker so scrolling to the very top loads an
    // older page of persisted events.
    initTopSentinel();
    // FEATURE-524: publish the current viewport so <runtime_info> can report it.
    reportViewport();
  };
  sock.onclose = () => {
    if (sock !== ws) return; // superseded socket: leave the new connection alone
    wsReady = false;
    conn.classList.remove("on");
    connText.textContent = T.disconnected;
    hideAsk();
    setRunning(false);
    clearStreamingMarks();
    // FEATURE-458: no auto-reconnect. The user clicks the connection control
    // to reconnect manually.
  };
  sock.onmessage = (m) => {
    // FIX-511: after the user clicked "断开连接" no message may be processed
    // until the next connection is established — messages already queued (or
    // delivered during the closing handshake) used to keep rendering into the
    // stream. Messages from a superseded socket are dropped as well.
    if (!wsReady || sock !== ws) return;
    let msg;
    try { msg = JSON.parse(m.data); } catch { return; }
    if (msg.kind === "event" && msg.event) renderEvent(msg.event);
    else if (msg.kind === "ask") showAsk(msg);
    else if (msg.kind === "interaction") showInteraction(msg);
    else if (msg.kind === "state") {
      // FIX-511: the connect-time snapshot carries the agent's running state, so
      // a refreshed or reconnected page restores the running indicators (logo
      // breathing, red ⏸ button, session-title highlight dot) instead of staying
      // idle while the backend keeps working.
      if (typeof msg.busy === "boolean") setRunning(msg.busy);
      renderPlan(msg.plan || null);
    }
    else if (msg.kind === "sessions") renderSessionMenu(msg.sessions || []);
    else if (msg.kind === "settings") renderSettings(msg.settings || []);
    else if (msg.kind === "settings_result") showSettingsResult(msg);
    else if (msg.kind === "mcp") renderMCPServers(msg.mcp_servers || []);
    else if (msg.kind === "mcp_result") showMCPResult(msg);
    else if (msg.kind === "identity") {
      renderIdentity(msg.identity || []);
      // FIX-542: keep the titlebar agent name in sync with the identity form.
      const nameField = (msg.identity || []).find((f) => f.key === "agent-name");
      if (nameField) setAgentName(nameField.value);
    }
    else if (msg.kind === "identity_result") showIdentityResult(msg);
    else if (msg.kind === "mode") renderModeSeg(msg.modes || []);
    else if (msg.kind === "mode_result") showModeResult(msg);
    else if (msg.kind === "models") { setModelAutoSelect(!!msg.auto_select); renderModels(msg.models || [], msg.templates || []); }
    else if (msg.kind === "model_result") showModelResult(msg);
    else if (msg.kind === "model_wizard") showModelWizardStep(msg);
    else if (msg.kind === "model_wizard_detect_progress") setCapDetectStatus(msg);
    else if (msg.kind === "pop_result") {
      // FIX-506: retry-from popped the session back. Instead of reloading the
      // whole page (which was unnecessary and disruptive), truncate the event
      // stream locally: remove the target block and everything after it.
      if (msg.ok) {
        truncateStreamFrom(popTargetIndex);
        // FIX-506 (三次续修): after popping, automatically resume the agent
        // from the truncation point instead of waiting for the user to type
        // :continue. We must send the literal ":continue" builtin command,
        // NOT an empty input: the REPL main loop drops empty lines
        // (repl.go: `if input == "" { continue }`), so an empty input would
        // be silently ignored. The ":continue" builtin bypasses that filter
        // and calls handleAgentInput("") directly (repl.go handleBuiltin),
        // which is exactly the resume behaviour we want.
        wsSend({ type: "input", text: ":continue" });
      }
      // FIX-506 (续修): a failed pop used to be swallowed silently, so the ⏪
      // button appeared dead with no error anywhere. Surface the reason in the
      // stream so the failure is diagnosable.
      else showRetryFromError(msg.message);
    }
    else if (msg.kind === "yolo") setYOLO(!!msg.yolo);
    else if (msg.kind === "dynamic_backfill") backfillInput(msg.backfill || []);
    else if (msg.kind === "history") renderHistory(msg);
    // FEATURE-548: the backend accepted a session switch. The message area still
    // holds the PREVIOUS session's blocks (the record is per session), so drop
    // them and replay the target session's own recorded events. The session id is
    // sent explicitly so the replay does not depend on when the backend applied
    // the switch.
    else if (msg.kind === "session_switched") {
      // FEATURE-571: this frame carries the current session id on connect and on
      // every switch, so the window registry learns which session it paints
      // before the history replay re-renders the windows.
      if (window.UIWin && typeof UIWin.setSession === "function") UIWin.setSession(msg.message || "");
      resetStreamView();
      // FEATURE-583: the pinned skills belong to the session being left, so
      // rebuild the skills panel for the newly current session.
      refreshSkills();
      wsSend({ type: "history_get", count: pageBufferSize, value: msg.message || "" });
    }
    // FEATURE-548: the switch was refused (a task is running). Refresh the list so
    // the current-session highlight snaps back and tell the user why.
    // FEATURE-565: the backend omits "ok" when it is false (json omitempty), so a
    // refusal arrives without the field at all. Testing === false therefore never
    // matched and the user saw nothing happen; treat every frame that is not an
    // explicit acceptance as a refusal.
    else if (msg.kind === "session_switch_result" && msg.ok !== true) {
      wsSend({ type: "session_list" });
      notifyUser(msg.message === "busy" ? (T.sessionSwitchBusy || "A task is running") : (msg.message || T.sessionSwitchFailed || ""));
    }
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

// FEATURE-524: report the browser viewport (CSS pixels) so <runtime_info>
// carries a <viewport> tag and the LLM can size its UI output for the surface
// it is talking to. Sent once the socket is open and again (debounced) on every
// resize; a dropped report is harmless because the next one refreshes it.
let viewportTimer = null;
function reportViewport() {
  wsSend({ type: "viewport", viewport_w: window.innerWidth, viewport_h: window.innerHeight });
}
function scheduleViewportReport() {
  if (viewportTimer) clearTimeout(viewportTimer);
  viewportTimer = setTimeout(reportViewport, 150);
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
  ask_user: { zh: "询问用户", en: "Ask user" },
  launch_sub_agent: { zh: "启动子代理", en: "Launch sub-agent" },
  schedule_task: { zh: "调度任务", en: "Schedule task" },
  track_task_progress: { zh: "更新任务进展", en: "Update task progress" },
  view_task_plan: { zh: "查看任务计划", en: "View task plan" },
  get_memory_slice: { zh: "读取记忆", en: "Read memory" },
  memory_search: { zh: "搜索记忆", en: "Search memory" },
  delete_memory: { zh: "删除记忆", en: "Delete memory" },
  evaluate_expression: { zh: "计算表达式", en: "Evaluate expression" },
  render_ui: { zh: "渲染界面组件", en: "Render UI components" },
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
// FIX-506 (续修): the YOU block created by renderUserEcho whose retry-from index
// is not yet known. The backend appends the user message inside agent.RunStream,
// so the index only becomes available on the first event of the turn; that event
// backfills this block (see backfillPendingUserIndex).
let pendingUserBlock = null;
// FIX-506: the msg_index of the block whose retry-from button was clicked.
// pop_result uses it to truncate the stream locally instead of reloading.
let popTargetIndex = "";
// FEATURE-507: history replay state. historyOldestSeq is the cursor of the
// oldest replayed event (for paging further back); historyHasMore reports
// whether older events exist on the server.
let historyOldestSeq = 0;
let historyHasMore = false;

// FEATURE-508: sliding-window cache rendering. The stream keeps only a bounded
// number of message-index groups in the DOM; older groups are evicted from the
// top and can be re-loaded on demand via the history_get `before` cursor.
// Thresholds come from the system settings (stream-window-max-blocks /
// stream-window-max-nodes) and fall back to the defaults below.
let streamWindowMaxBlocks = 128;
let streamWindowMaxNodes = 12800;
// FIX-590: text volume (not the block count) drives the forced-layout cost, so
// the amount of stream text is capped as well. The value is overridable from
// the settings (stream-window-max-chars) and feeds streamTextChars below.
let streamWindowMaxChars = 2000000;
// FIX-509: how many message groups one history page holds. Loaded from the
// page-buffer-size setting; the backend uses the same value as its fallback.
let pageBufferSize = 20;
// True while a history page is being fetched for the top sentinel, so the
// observer does not fire a second request for the same cursor.
let historyLoading = false;
// True while renderHistory is inserting an older page at the top: suppresses
// the eviction pass so the freshly loaded page is not immediately trimmed.
let historyInserting = false;
// While an older history page is being replayed, every block created by
// makeBlock is inserted BEFORE this node instead of being appended, so the
// page lands at the top of the stream. null means "append at the bottom".
let insertAnchor = null;
// FIX-509: true while renderHistory is prepending an older page. Blocks created
// by makeBlock already honour insertAnchor, but the per-iteration token line and
// the message-visualization line are created later (on token_iter) and used to
// be appended unconditionally — the token lines piled up at the bottom of the
// stream and the viz lines landed on the right edge instead of the left.
let prependingHistory = false;

/* FIX-518: workspace-refresh coalescing. Every LLM iteration (token_iter),
   iteration end (done) and finished tool call used to call
   refreshBranch()+loadTree() right away. During a history replay those events
   arrive in a burst — one page can hold dozens of them — and /api/tree is
   ~6 MB for a workspace with build output in it, so a page refresh pinned the
   main thread (and the input box) for tens of seconds. Refreshes are now
   merged: deferred while a replay is running, debounced otherwise. */
const WORKSPACE_REFRESH_DEBOUNCE_MS = 300;
let workspaceRefreshTimer = 0;
let workspaceRefreshPending = false;
let historyReplaying = false;

// scheduleWorkspaceRefresh asks for a branch + file-tree refresh, coalescing a
// burst of requests into one. `immediate` skips the debounce (used when a
// replay ends, or when a user action expects a freshly rebuilt tree).
function scheduleWorkspaceRefresh(immediate) {
  if (historyReplaying && !immediate) {
    workspaceRefreshPending = true;
    return;
  }
  if (workspaceRefreshTimer) {
    clearTimeout(workspaceRefreshTimer);
    workspaceRefreshTimer = 0;
  }
  if (immediate) {
    refreshBranch();
    loadTree();
    return;
  }
  workspaceRefreshTimer = setTimeout(() => {
    workspaceRefreshTimer = 0;
    refreshBranch();
    loadTree();
  }, WORKSPACE_REFRESH_DEBOUNCE_MS);
}

// endHistoryReplay releases the replay guard and runs the refresh that was
// deferred while it was held (if any).
function endHistoryReplay() {
  historyReplaying = false;
  if (workspaceRefreshPending) {
    workspaceRefreshPending = false;
    scheduleWorkspaceRefresh(true);
  }
}

// FEATURE-445: follow-output scrolling. When the scrollbar is within 100px of
// the content bottom, new output auto-scrolls to the bottom (follows output);
// otherwise it does not, so the user can read history. A floating down-arrow
// button appears when scrolled >100px from the bottom; clicking it jumps to
// the bottom and resumes following.
const FOLLOW_THRESHOLD = 100;
let followOutput = true; // whether new output should auto-scroll to the bottom

// FIX-591: the follow-scroll is coalesced into one animation frame. Reading
// scrollHeight forces a synchronous layout of the whole stream (~40 ms once the
// session holds megabytes of text), and the streaming path used to call this
// once per chunk - far more often than the markdown render, which is throttled
// - so most of those layouts were wasted: the content had not changed yet.
let scrollPending = false;
function scrollStream() {
  if (scrollPending) return;
  if (renderPaused()) return; // FIX-594: nothing is painted while hidden
  scrollPending = true;
  requestAnimationFrame(() => {
    scrollPending = false;
    if (!followOutput) return;
    // FIX-594: assign a huge value instead of reading scrollHeight first — the
    // read forces a synchronous layout of the whole (possibly multi-MB) stream
    // on every follow-scroll frame; the assignment just clamps to the bottom.
    streamB.scrollTop = 1e9;
    if (scrollDownBtn) scrollDownBtn.classList.remove("on");
  });
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
  // FIX-595: clamp-assign instead of reading scrollHeight — that read is a
  // forced layout proportional to the session size now that the .ev
  // content-visibility isolation is gone.
  streamB.scrollTop = 1e9;
  if (scrollDownBtn) scrollDownBtn.classList.remove("on");
}

// FEATURE-419: floating block-boundary navigation. When the current block is
// taller than the viewport and its title line / bottom is out of view, show a
// floating ↑ / ↓ icon at the top / bottom of the main data area so the user can
// jump to that boundary. The "current block" is the .ev block with the largest
// visible area inside the viewport.
let blockNavCurrent = null; // the .ev box the icons currently target

// FIX-594: which blocks currently intersect the stream viewport, maintained by
// an IntersectionObserver. updateBlockNav used to walk every .ev and read
// offsetTop/offsetHeight, which forces a synchronous layout of the whole stream
// (and defeats the .ev content-visibility isolation) — that was the main cost of
// slowly scrolling through a long session.
const blockNavVisible = new Set();
let blockNavObserver = null;

// ensureBlockNavObserver lazily starts the viewport observer.
function ensureBlockNavObserver() {
  if (blockNavObserver || typeof IntersectionObserver === "undefined") return;
  blockNavObserver = new IntersectionObserver((entries) => {
    for (const e of entries) {
      if (e.isIntersecting) blockNavVisible.add(e.target);
      else blockNavVisible.delete(e.target);
    }
    updateBlockNav();
  }, { root: streamB, threshold: 0 });
  streamB.querySelectorAll(".ev").forEach((box) => blockNavObserver.observe(box));
}

// observeEv registers a newly created block with the observer (no-op before the
// observer exists; ensureBlockNavObserver sweeps the existing blocks).
function observeEv(box) {
  if (blockNavObserver) blockNavObserver.observe(box);
}

function updateBlockNav() {
  if (!blockNavTop || !blockNavBottom) return;
  const scroller = streamB;
  const viewTop = scroller.scrollTop;
  const viewBottom = viewTop + scroller.clientHeight;
  // FIX-594: only the blocks the observer reports as intersecting the viewport
  // are examined, so the work no longer grows with the length of the session.
  // Falls back to a full walk when IntersectionObserver is unavailable.
  const candidates = blockNavObserver ? blockNavVisible : scroller.querySelectorAll(".ev");
  let best = null, bestArea = 0;
  for (const box of candidates) {
    if (blockNavObserver && !box.isConnected) {
      blockNavVisible.delete(box);
      blockNavObserver.unobserve(box);
      continue;
    }
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
  // FIX-594: start the viewport observer that feeds updateBlockNav.
  ensureBlockNavObserver();
  blockNavTop.addEventListener("click", () => {
    if (!blockNavCurrent) return;
    streamB.scrollTop = blockNavCurrent.offsetTop;
  });
  blockNavBottom.addEventListener("click", () => {
    if (!blockNavCurrent) return;
    streamB.scrollTop = blockNavCurrent.offsetTop + blockNavCurrent.offsetHeight - streamB.clientHeight;
  });
  // FIX-591: coalesce the block-navigation scan into one animation frame - it
  // walks every block and reads offsetTop/offsetHeight (a forced layout), and
  // scroll events arrive in bursts.
  // FIX-594: also rate-limit it. That walk force-layouts every .ev, which also
  // defeats the .ev content-visibility isolation; on long sessions it was the
  // main cost of slowly scrolling through history. The floating navigation
  // icons do not need frame accuracy, so 100ms is plenty.
  let blockNavRaf = 0;
  let blockNavLastRun = 0;
  streamB.addEventListener("scroll", () => {
    if (blockNavRaf) return;
    blockNavRaf = requestAnimationFrame(() => {
      blockNavRaf = 0;
      const now = performance.now();
      if (now - blockNavLastRun < 100) return;
      blockNavLastRun = now;
      updateBlockNav();
    });
  });
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
  setIcon(toggle, "i-expand");
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
    setIcon(toggle, wrap.classList.contains("expanded") ? "i-collapse" : "i-expand");
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
  setIcon(toggle, "i-expand");
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
    setIcon(toggle, body.parentElement.classList.contains("sup-content-expanded") ? "i-collapse" : "i-expand");
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
  setIcon(toggle, "i-expand");
  toggle.title = "展开/固定高度";
  toggle.onclick = () => {
    params.classList.toggle("expanded");
    setIcon(toggle, params.classList.contains("expanded") ? "i-collapse" : "i-expand");
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

// FIX-590: markdown re-render budget.
//
// Rendering the whole accumulated text on every animation frame cost 2.3 ms at
// 2 KB but 29.7 ms at 200 KB - more than a frame budget, so the main thread
// stayed saturated and typing / IME input had to queue behind it. Two guards
// now apply:
//   1. the cadence adapts to the block size (small text still renders every
//      frame, large text at ~10 fps / ~3 fps),
//   2. rendering is skipped entirely while the user types or an IME composition
//      is active, and the accumulated text is painted once the burst is over
//      (noteInputActivity / inputQuiet / flushPendingMd).
// FIX-594: the smallest tier used to repaint on EVERY animation frame
// (minIntervalMs 0). On a 120Hz display that is up to 120 full markdown
// rebuilds per second while a short message streams. 50ms (~20fps) still reads
// as a typewriter but cuts the work by up to 6x.
const MD_TIERS = [
  { maxChars: 4096, minIntervalMs: 50 },
  { maxChars: 32768, minIntervalMs: 100 },
  { maxChars: Infinity, minIntervalMs: 330 },
];
// INPUT_QUIET_MS is how long the renderer keeps yielding after the last input
// activity (keystroke or IME update).
const INPUT_QUIET_MS = 250;
let inputActiveUntil = 0;
let pendingMdBlocks = new Set();
let pendingMdTimer = null;

// noteInputActivity marks the user as busy typing so markdown re-rendering and
// the auto-scroll step aside until the burst ends (FIX-590).
function noteInputActivity() {
  inputActiveUntil = performance.now() + INPUT_QUIET_MS;
  if (pendingMdTimer) clearTimeout(pendingMdTimer);
  pendingMdTimer = setTimeout(flushPendingMd, INPUT_QUIET_MS + 20);
}

// inputQuiet reports whether the renderer may work right now (FIX-590).
function inputQuiet() {
  return performance.now() < inputActiveUntil || window.__composing === true;
}

// renderPaused reports whether the document is currently not being painted: a
// background tab, or a non-active iframe inside the hub (display:none makes the
// child document report hidden). Rendering then is pure waste — the browser
// skips the paint anyway, but the JS (whole-message markdown rebuilds, forced
// layouts) still runs. Pausing keeps hidden instances nearly free (FIX-594).
function renderPaused() {
  return document.hidden === true;
}

// mdTier returns the render cadence for a block of the given text length.
function mdTier(len) {
  for (const t of MD_TIERS) if (len <= t.maxChars) return t;
  return MD_TIERS[MD_TIERS.length - 1];
}

// flushPendingMd paints every block whose render was deferred by the input gate
// or by the cadence (FIX-590). token_iter / done call it, so a finished message
// is always rendered completely.
function flushPendingMd() {
  pendingMdTimer = null;
  if (!pendingMdBlocks.size) return;
  const blocks = Array.from(pendingMdBlocks);
  pendingMdBlocks.clear();
  for (const b of blocks) if (b.mdDirty) renderMdNow(b);
}

// FIX-594: paint what was deferred while the document was hidden as soon as it
// becomes visible again (background tab, or switching back to a hub agent
// whose iframe was display:none).
document.addEventListener("visibilitychange", () => {
  if (document.hidden) return;
  flushPendingMd();
  scrollStream();
});

// renderMdNow paints a streaming block once, keeping the FEATURE-515 pinned
// THINK body at its bottom and following the output as before.
function renderMdNow(b) {
  const pinned = thinkPinned(b.body);
  b.body.classList.add("md");
  mdRender(b.body, b.raw);
  if (pinned) b.body.scrollTop = b.body.scrollHeight;
  b.mdDirty = false;
  b.mdLastRender = performance.now();
  pendingMdBlocks.delete(b);
  scrollStream();
}

// clearStreamingMarks removes the "streaming" breathing animation from every
// block. It runs when a turn ends or the connection drops, so no CSS animation
// keeps running while the UI is idle (FIX-590).
function clearStreamingMarks() {
  document.querySelectorAll(".ev-head.streaming").forEach((h) => h.classList.remove("streaming"));
}

// scheduleMd re-renders a streaming block as markdown. Chunks arrive far faster
// than the screen refreshes, so the work is coalesced into one render per
// animation frame and then filtered through the size tier and the input gate
// (FIX-590).
function scheduleMd(b) {
  b.mdDirty = true;
  if (inputQuiet() || renderPaused()) {
    pendingMdBlocks.add(b);
    return;
  }
  if (b.raf) return;
  b.raf = requestAnimationFrame(() => {
    b.raf = null;
    if (!b.mdDirty) return;
    const tier = mdTier(b.raw.length);
    if (tier.minIntervalMs && performance.now() - (b.mdLastRender || 0) < tier.minIntervalMs) {
      // Too soon for a block of this size: keep it pending and make sure a
      // trailing flush happens even when the stream stops right here.
      pendingMdBlocks.add(b);
      if (!pendingMdTimer) pendingMdTimer = setTimeout(flushPendingMd, tier.minIntervalMs);
      return;
    }
    renderMdNow(b);
  });
}

// thinkPinned reports whether a height-capped THINK body sits at (or near) its
// bottom, i.e. whether following the newest streamed content would not fight
// the user (FEATURE-515).
function thinkPinned(body) {
  const box = body.parentElement;
  if (!box || !box.classList.contains("thinking")) return false;
  if (box.classList.contains("think-expanded")) return false;
  return body.scrollHeight - body.scrollTop - body.clientHeight <= 8;
}

// addThinkToggle appends the fixed-height expand/collapse control to a THINK
// block's title bar (FEATURE-515). It mirrors the TOOL input-params toggle:
// ⤢ while the body is capped at its fixed height, ⤡ when fully expanded.
function addThinkToggle(head, box) {
  const actions = head.querySelector(".ev-actions");
  const toggle = document.createElement("button");
  toggle.className = "ev-act think-toggle";
  setIcon(toggle, "i-expand");
  toggle.title = "展开/固定高度";
  toggle.onclick = (e) => {
    e.stopPropagation();
    const expanded = box.classList.toggle("think-expanded");
    setIcon(toggle, expanded ? "i-collapse" : "i-expand");
  };
  if (actions) actions.appendChild(toggle);
  else head.appendChild(toggle);
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
  // FEATURE-515: THINK blocks keep a fixed height with internal scrolling; the
  // title-bar toggle (⤢/⤡) expands the body to full height.
  if (cls === "thinking") addThinkToggle(head, box);
  // FEATURE-425: mark the final completion block (TOOL: 完成任务) as the
  // result block so silent mode keeps it visible.
  if (cls === "tool" && /完成任务|Complete task/.test(label)) box.classList.add("ev-result");
  // FEATURE-425: apply the current display mode to the new block.
  applyBlockDisplayMode(box, cls);
  // FEATURE-445: all blocks append to the single stream region B.
  // FEATURE-508: while an older history page is being replayed, insertAnchor
  // points at the current oldest node, so new blocks land at the top instead.
  if (insertAnchor && insertAnchor.parentNode === streamB) streamB.insertBefore(box, insertAnchor);
  else streamB.appendChild(box);
  // FEATURE-508: keep the top sentinel as the first child so the observer can
  // still detect scrolling to the very top after new blocks are appended.
  ensureTopSentinel();
  // FIX-594: let the viewport observer track this block (block navigation).
  observeEv(box);
  // FEATURE-482: queue this block for the message-visualization chart. Its
  // context-usage value is backfilled when the iteration's token_iter arrives.
  if (msgVizPending) msgVizPending.push({ cls: cls, box: box });
  scrollStream();
  // FEATURE-508: evict the oldest message-index groups once the window exceeds
  // either threshold (block count / DOM node count).
  enforceStreamWindow();
  return body;
}

// applyBlockDisplayMode shows/hides or collapses a single block according to
// the current display mode (FEATURE-425).
// isResultLikeBlock reports whether a block carries the answer the user asked
// for rather than process noise. Besides the final completion block
// (.ev-result), an LLM-rendered component tree (cls "ui", FEATURE-524) counts
// as a result: hiding or collapsing it would hide the rendered answer itself.
function isResultLikeBlock(box, cls) {
  return box.classList.contains("ev-result") || cls === "ui";
}

function applyBlockDisplayMode(box, cls) {
  if (displayMode === "silent") {
    const show = cls === "user-msg" || box.classList.contains("level-error") || isResultLikeBlock(box, cls);
    box.style.display = show ? "" : "none";
    // The final result block must be fully expanded so the user sees the
    // completion report (FIX-426).
    if (isResultLikeBlock(box, cls)) box.classList.remove("collapsed");
  } else if (displayMode === "minimal") {
    box.style.display = "";
    // User-msg blocks are always expanded so the user sees their original
    // instruction (FIX-426).
    if (cls === "user-msg") { box.classList.remove("collapsed"); return; }
    const body = box.querySelector(".ev-body");
    const isStreaming = body && isStreamingBody(body);
    const isResult = isResultLikeBlock(box, cls);
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
  copy.innerHTML = iconHTML("i-copy");
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
    setIcon(collapse, "i-tri-down");
    collapse.title = T.collapseBlock;
    const storageKey = "co-shell-collapse-" + cls;
    const applyCollapse = () => {
      // FEATURE-409: a block that is currently streaming output is never
      // collapsed, so the user always sees the live dynamic content.
      const collapsed = !isStreamingBody(body) && localStorage.getItem(storageKey) === "1";
      box.classList.toggle("collapsed", collapsed);
      setIcon(collapse, collapsed ? "i-tri" : "i-tri-down");
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
  // FIX-506 (续修): only the YOU block (user message) carries this action —
  // its semantics are unambiguous (keep up to that user message). Other block
  // types (LLM/THINK/TOOL/REPL/SYSTEM) do not map cleanly to a message index
  // (e.g. popping into the middle of a tool_calls pair would corrupt history),
  // so the button is omitted for them.
  if (cls === "user-msg") {
    const retry = document.createElement("button");
    retry.className = "ev-act";
    retry.innerHTML = iconHTML("i-retry");
    retry.title = T.retryFrom;
    retry.onclick = (e) => {
      e.stopPropagation();
      // FIX-506: remember which block was clicked so pop_result can truncate the
      // stream locally (instead of reloading the page).
      popTargetIndex = box.dataset.msgIndex || "";
      // FIX-506 (续修): without an index the backend cannot resolve the target
      // message (it would reject an empty value). Report it instead of sending
      // a request that fails silently.
      if (!popTargetIndex) { showRetryFromError(""); return; }
      wsSend({ type: "session_pop", value: String(popTargetIndex) });
    };
    actions.appendChild(retry);
  }

  head.appendChild(actions);
}

// showRetryFromError reports a failed retry-from (⏪) action in the message
// stream, so the button never appears dead without explanation (FIX-506 续修).
// An empty reason falls back to a generic message.
function showRetryFromError(reason) {
  const body = makeBlock("system", "SYS", "");
  body.parentElement.classList.add("level-error");
  const tpl = i18nT("retryFromFailed", "Retry-from failed: %s");
  body.textContent = tpl.replace("%s", reason || "-");
  scrollStream();
}

// truncateStreamFrom removes the block whose msg_index is targetIndex and every
// block after it, so the visible stream matches the truncated session history
// (FIX-506). It replaces the previous location.reload() behaviour.
// Blocks are direct children of #stream (each .ev box), and token-stats lines
// are siblings placed right after their block. We therefore find the target
// .ev box and remove it plus all following siblings.
function truncateStreamFrom(targetIndex) {
  if (targetIndex === undefined || targetIndex === null || targetIndex === "") return;
  const target = stream.querySelector('.ev[data-msg-index="' + targetIndex + '"]');
  if (!target) return;
  // Remove every sibling from the target block onwards (blocks + token lines).
  let node = target;
  while (node) {
    const next = node.nextSibling;
    node.remove();
    node = next;
  }
  // Reset streaming pointers: the removed blocks may have been the active ones.
  curLLM = curThinking = curTool = curREPL = curSup = null;
  iterBlocks = [];
  iterToolBlocks = [];
  toolBlockByName = {};
  lastBlock = null;
  // FIX-506 (续修): the pending YOU block may have been removed by the
  // truncation; drop the reference so a later event cannot backfill a detached
  // node.
  pendingUserBlock = null;
  // The session history changed — refresh the workspace state (branch + tree).
  refreshBranch();
  loadTree();
  scrollStream();
}

// FEATURE-508: group the stream's direct children by message index. Each group
// is a contiguous run of nodes sharing the same data-msg-index (a block plus its
// token-stats line, tool sub-blocks, etc.). Nodes without an index (e.g. the
// top sentinel) form their own single-node group keyed by a unique marker.
function streamGroups() {
  const groups = [];
  let cur = null;
  for (const node of streamB.children) {
    const idx = node.dataset ? node.dataset.msgIndex : undefined;
    if (cur && idx !== undefined && cur.index === idx) {
      cur.nodes.push(node);
    } else {
      cur = { index: idx, nodes: [node] };
      groups.push(cur);
    }
  }
  return groups;
}

// enforceStreamWindow evicts the oldest message-index groups from the top of the
// stream until both thresholds are satisfied (FEATURE-508). Eviction is always
// whole-group so a tool_call / tool_result pair is never split. The newest group
// is never evicted, so the block currently being streamed stays in the DOM.
function enforceStreamWindow() {
  if (historyInserting) return;
  const groups = streamGroups();
  if (groups.length <= 1) return;
  let blocks = streamB.children.length;
  let nodes = streamB.querySelectorAll("*").length;
  // FIX-590: the amount of text is capped too - 1 MB of stream text costs ~66 ms
  // per forced layout and 20 MB costs ~2.2 s.
  let chars = streamTextChars(groups);
  if (blocks <= streamWindowMaxBlocks && nodes <= streamWindowMaxNodes && chars <= streamWindowMaxChars) return;
  // Walk from the oldest group forward, dropping groups while the window is
  // still over any threshold and at least one group would remain.
  let drop = 0;
  while (drop < groups.length - 1) {
    const g = groups[drop];
    const gBlocks = g.nodes.length;
    const gNodes = g.nodes.reduce((n, el) => n + 1 + el.querySelectorAll("*").length, 0);
    const gChars = g.nodes.reduce((n, el) => n + el.textContent.length, 0);
    if (blocks - gBlocks < streamWindowMaxBlocks && nodes - gNodes < streamWindowMaxNodes && chars - gChars < streamWindowMaxChars) break;
    blocks -= gBlocks;
    nodes -= gNodes;
    chars -= gChars;
    drop++;
  }
  if (!drop) return;
  // Remember the scroll anchor so the viewport does not jump when the top is
  // trimmed while the user is reading older content. FIX-590: anchoring on the
  // first surviving group is far cheaper than the two scrollHeight reads used
  // before (a full container height is no longer forced).
  const prevTop = streamB.scrollTop;
  const next = groups[drop];
  const anchor = next && next.nodes[0];
  const anchorTop = anchor ? anchor.offsetTop : 0;
  for (let i = 0; i < drop; i++) {
    for (const node of groups[i].nodes) node.remove();
  }
  if (anchor) {
    const delta = anchorTop - anchor.offsetTop;
    if (delta > 0) streamB.scrollTop = Math.max(0, prevTop - delta);
  }
  // FIX-590: recompute the block-navigation icons on the next frame instead of
  // forcing a full block scan inside this path.
  requestAnimationFrame(updateBlockNav);
}

// streamTextChars returns the amount of text held by the stream window. Closed
// groups cache their size on their nodes, so the walk stays cheap per call
// instead of rebuilding megabytes of text every time (FIX-590). The growing
// (last) group is measured directly; its size is an estimate that errs low for
// closed groups whose content grows later (tool results), which is acceptable
// for a soft cap.
function streamTextChars(groups) {
  let total = 0;
  for (let i = 0; i < groups.length; i++) {
    const g = groups[i];
    const last = i === groups.length - 1;
    for (const el of g.nodes) {
      if (!last && el._chars !== undefined) {
        total += el._chars;
        continue;
      }
      const n = el.textContent.length;
      if (!last) el._chars = n;
      total += n;
    }
  }
  return total;
}

// ensureTopSentinel keeps a zero-height marker as the first child of the stream
// so an IntersectionObserver can detect when the user scrolls to the very top
// and trigger loading of an older history page (FEATURE-508).
function ensureTopSentinel() {
  let s = document.getElementById("streamTopSentinel");
  if (!s) {
    s = document.createElement("div");
    s.id = "streamTopSentinel";
    s.className = "stream-top-sentinel";
    streamB.insertBefore(s, streamB.firstChild);
  } else if (streamB.firstChild !== s) {
    streamB.insertBefore(s, streamB.firstChild);
  }
  return s;
}

// loadOlderHistory requests the previous page of persisted events using the
// history_get `before` cursor (FEATURE-508). It is a no-op while a page is
// already in flight or when the server reported no more history.
function loadOlderHistory() {
  if (historyLoading || !historyHasMore || !historyOldestSeq) return;
  historyLoading = true;
  wsSend({ type: "history_get", count: pageBufferSize, before: historyOldestSeq });
}

// resetStreamView clears the event stream and the renderer bookkeeping that
// tracks in-flight blocks (FIX-511). It is called for every new WebSocket
// connection, before the persisted history page is requested, so the replay
// rebuilds the view from scratch: on a page load the stream is already empty
// (no-op), while on a reconnect it drops the previous content so the replay
// does not duplicate it.
function resetStreamView() {
  streamB.textContent = "";
  if (msgVizTrack) msgVizTrack.textContent = "";
  // In-flight block references.
  curLLM = null;
  curThinking = null;
  curTool = null;
  curSup = null;
  curREPL = null;
  lastBlock = null;
  iterBlocks = [];
  iterToolBlocks = [];
  iterCount = 0;
  toolBlockByName = {};
  // Message-index bookkeeping used by retry-from / pop.
  lastMsgIndex = "";
  pendingUserBlock = null;
  popTargetIndex = "";
  // History paging + replay state.
  historyOldestSeq = 0;
  historyHasMore = false;
  historyLoading = false;
  historyInserting = false;
  insertAnchor = null;
  prependingHistory = false;
  // View + statistics derived from the replayed events (rebuilt by the replay).
  blockNavCurrent = null;
  followOutput = true;
  msgVizPending = [];
  msgVizUsage = 0;
  msgVizPrependAnchor = null;
  tokenStats.sessionIn = 0;
  tokenStats.sessionOut = 0;
  tokenStats.lastIn = 0;
  tokenStats.lastOut = 0;
  tokenStats.lastInTPS = 0;
  tokenStats.lastOutTPS = 0;
  tokenStats.lastInDur = "-";
  tokenStats.lastOutDur = "-";
  tokenStats.lastCacheRate = "";
}

// initTopSentinel wires the IntersectionObserver that watches the top marker.
function initTopSentinel() {
  const s = ensureTopSentinel();
  if (!("IntersectionObserver" in window)) return;
  const io = new IntersectionObserver((entries) => {
    for (const e of entries) {
      if (e.isIntersecting) loadOlderHistory();
    }
  }, { root: streamB, rootMargin: "120px 0px 0px 0px", threshold: 0 });
  io.observe(s);
}

// eventBlockLabel returns the title a block shows. FIX-549: the replay path
// used to derive it from the channel alone, so a THINK block was titled "LLM"
// (dim italic text under an LLM heading). The live streaming path already uses
// these same type-specific titles ("THINK"/"LLM"/"TOOL"); keep both in step.
function eventBlockLabel(ev) {
  switch (ev.type) {
    case "content_chunk": case "content": return "LLM";
    case "thinking_chunk": case "thinking": return "THINK";
    case "tool_call": case "tool_call_stream": return "TOOL";
  }
  return CHAN_LABEL[ev.chan] || (ev.chan || "SYS").toUpperCase();
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

// FEATURE-524: paint an LLM-authored component tree into the main DOM. The
// tree arrives as JSON text in meta.ui_tree; UI.renderTree builds every node
// with createElement/textContent and never touches innerHTML, so hostile
// markup inside the tree stays inert text.
function renderUIBlock(ev) {
  const m = ev.meta || {};
  // FEATURE-571: window and panel targets belong to the multi-window registry,
  // which creates the addressed window on demand, keeps it across surfaces and
  // mirrors it back to the agent. A registry that cannot place the tree (no
  // window markup) falls through to the FEATURE-524 single-window path below.
  if (m.ui_target === "window" || m.ui_target === "panel") {
    if (window.UIWin && typeof UIWin.render === "function" && UIWin.render(m)) return;
  }
  // FEATURE-524 window mode: a tree rendered with target="window" belongs to
  // the floating window, not to the chat stream. The window is opened on
  // demand, so a render that races ahead of its ui_window event still lands in
  // a visible place instead of being dropped.
  if (m.ui_target === "window" && window.UI && typeof UI.openWindow === "function") {
    const winBody = UI.openWindow(m.ui_window_title || "");
    if (!winBody) return;
    let host = document.getElementById("uiWindowTree");
    if (!host) {
      host = document.createElement("div");
      host.className = "ui-tree";
      host.id = "uiWindowTree";
      winBody.appendChild(host);
    }
    host.dataset.uiId = m.ui_id || "";
    host.replaceChildren();
    if (typeof UI.renderTree === "function") UI.renderTree(m.ui_tree || "", host);
    return;
  }
  const body = makeBlock("ui", "UI · " + toolAction("render_ui"), m.msg_index);
  const host = document.createElement("div");
  host.className = "ui-tree";
  if (m.ui_id) host.dataset.uiId = m.ui_id;
  body.appendChild(host);
  if (window.UI && typeof UI.renderTree === "function") {
    UI.renderTree(m.ui_tree || "", host);
  } else {
    host.textContent = m.ui_id || "";
  }
  // FEATURE-580: every rendered block carries its own export entry (Markdown
  // download / PDF print) so the user can keep or print the whole content.
  // The tree is the event meta's ui_tree, so the export never scrapes the DOM.
  if (window.UIExport && typeof UIExport.attachBlock === "function") {
    UIExport.attachBlock(body.closest(".ev"), m);
  }
}

// applyUIUpdate replaces an already rendered component in place: the agent
// addresses it by meta.ui_id and sends the replacement subtree in
// meta.ui_patch (FEATURE-524).
// applyUIWindow opens or closes the single floating window on a ui_window
// event (FEATURE-524 window mode). Opening an open window reuses it and only
// updates the title, so repeated opens never stack overlays.
function applyUIWindow(ev) {
  const m = ev.meta || {};
  // FEATURE-571: the registry owns open/close/activate, including which window
  // an action addresses and the surface it lives on.
  if (window.UIWin && typeof UIWin.applyWindowEvent === "function" && UIWin.applyWindowEvent(m)) return;
  if (!window.UI || typeof UI.closeWindow !== "function" || typeof UI.openWindow !== "function") return;
  if (m.ui_window_action === "close") { UI.closeWindow(); return; }
  if (m.ui_window_action === "open") { UI.openWindow(m.ui_window_title || "", m.ui_window_size || ""); }
}

// The window's own close button dismisses it (FEATURE-524 window mode). The
// window is never persisted, so closing hides it and drops its content; a later
// ui_update addressed to it then warns instead of resurrecting DOM.
(function bindUIWindowClose() {
  const btn = document.getElementById("uiWindowClose");
  if (!btn) return;
  btn.addEventListener("click", function () {
    // FEATURE-571: the button closes the window that owns its shell. The
    // registry binds its own shells too; this path covers it and keeps the
    // FEATURE-524 fallback for a bundle without ui-windows.js.
    if (window.UIWin && typeof UIWin.closeFromShell === "function" && UIWin.closeFromShell(btn)) return;
    if (window.UI && typeof UI.closeWindow === "function") UI.closeWindow();
  });
})();

function applyUIUpdate(ev) {
  const m = ev.meta || {};
  if (!m.ui_id || !window.UI || typeof UI.updateTree !== "function") return;
  let patch = null;
  try {
    patch = JSON.parse(m.ui_patch || "");
  } catch (err) {
    // A broken patch stays local: report it and let the rest of the stream
    // render normally (FEATURE-524, UC-30).
    console.warn("[ui_update] unparsable patch for " + m.ui_id, err);
    return;
  }
  if (!UI.updateTree(m.ui_id, patch)) {
    console.warn("[ui_update] no rendered component with id " + m.ui_id);
  }
}

// isToolStreamBlock reports whether a replayed event is a persisted
// tool-stream block, i.e. the replay-side form of the live tool_call_stream
// fragment group that opens a TOOL block (FEATURE-362).
//
// FIX-575: the backend coalescer merges the streamed parameter fragments into
// ONE tool_call event and stamps it with meta.phase = "stream". A tool_call
// WITHOUT any phase can only be such a block: every other tool_call the agent
// emits carries phase=input|result (agent/run_stream.go), so the missing phase
// is by itself an unambiguous "a new TOOL block starts here" - which also
// covers history persisted before FIX-575 (no phase, only the ⚙️ header).
function isToolStreamBlock(ev) {
  if (!ev || ev.type !== "tool_call") return false;
  const phase = ev.meta && ev.meta.phase;
  return !phase;
}

// openToolStreamBlock recreates the TOOL block that the live path opens on the
// "⚙️ <tool>" header of the first tool_call_stream fragment (see the
// tool_call_stream branch above). Both paths must produce the SAME block
// structure so that the following tool_call(phase=input) event reuses this very
// block (by tool name) and tool_call(phase=result) appends its outcome to it.
//
// Before FIX-575 the replay had no such branch: the streamed parameters fell
// into the input/result heuristic below, which merged them into the PREVIOUS
// tool block - or dropped them entirely - whenever curTool.raw /
// curTool.hasResult happened to be in the "wrong" state. That is what made tool
// cards disappear at random after a page refresh or a co-shell restart.
function openToolStreamBlock(ev, msgIndex) {
  // FIX-549: a concrete tool block ends the LLM text block before it, so live
  // and replay agree on [LLM][TOOL][LLM] in strict emission order.
  unmarkStreaming(curLLM && curLLM.body);
  maybeCollapseEnded("llm");
  curLLM = null;
  curThinking = null;
  curSup = null;
  curTool = newStreamBlock("tool", "TOOL", msgIndex);
  // FIX-462: remember the block in creation order so later tool_call events can
  // backfill each block's intent one by one.
  iterToolBlocks.push(curTool);
  const params = ensureToolParams(curTool);
  let text = ev.text || "";
  // Strip the "⚙️ <tool>\n" header line, exactly like the live stream path.
  // The gear is U+2699 followed by an optional U+FE0F variation selector.
  const gear = text.indexOf("\u2699");
  if (gear >= 0) {
    const nameStart = text.charCodeAt(gear + 1) === 0xFE0F ? gear + 2 : gear + 1;
    const nl = text.indexOf("\n", gear);
    const name = text.slice(nameStart, nl >= 0 ? nl : text.length).trim();
    if (name) {
      curTool.toolName = name;
      toolBlockByName[name] = curTool;
    }
    text = text.slice(nl >= 0 ? nl + 1 : text.length);
  }
  params.raw += text;
  renderParams(params);
  // A replayed block is already complete: no streaming marker, no result yet.
  curTool.hasResult = false;
  scrollStream();
}

function renderEvent(ev) {
  // FEATURE-409: the message index attached by the backend lets the retry-from
  // action map this block back to a message for :session pop to.
  const msgIndex = ev.meta && ev.meta.msg_index;
  if (msgIndex) lastMsgIndex = msgIndex;
  // FIX-506 (续修): the first event of a turn carries the index of the user
  // message that started it (the backend appends the user message before the
  // first LLM event, and no assistant/tool message exists yet). Backfill the
  // pending YOU block with it so its retry-from index is the real array index
  // of that user message — not the previous turn's stale value.
  if (pendingUserBlock && msgIndex) {
    pendingUserBlock.dataset.msgIndex = msgIndex;
    delete pendingUserBlock.dataset.msgIndexPending;
    pendingUserBlock = null;
  }
  // Turn-boundary signals from the web session (FEATURE-369): drive the
  // merged send/interrupt button, never render as blocks.
  // FEATURE-548: the user's own message, recorded when its turn began. Replaying
  // it rebuilds the YOU block that the live view drew from the local echo.
  if (ev.type === "user_echo") { renderUserEcho(ev.text); return; }
  if (ev.type === "await_input") { setRunning(false); clearStreamingMarks(); return; }
  if (ev.type === "turn_start") { setRunning(true); return; }
  if (ev.type === "task_plan") {
    let plan = null;
    try { if (ev.meta && ev.meta.plan) plan = JSON.parse(ev.meta.plan); } catch { /* keep null */ }
    renderPlan(plan);
    return;
  }
  if (ev.type === "sub_agents") {
    let rows = [];
    try { if (ev.meta && ev.meta.sub_agents) rows = JSON.parse(ev.meta.sub_agents); } catch { /* keep [] */ }
    renderSubAgents(rows);
    return;
  }
  // FEATURE-583: the full loaded-skill snapshot; redraw the panel from it.
  if (ev.type === "skill_state") {
    let payload = null;
    try { if (ev.meta && ev.meta.skill_state) payload = JSON.parse(ev.meta.skill_state); } catch { /* keep null */ }
    renderSkills(payload ? payload.loaded : []);
    return;
  }
  // FEATURE-524: LLM component trees (render_ui / ui_update).
  if (ev.type === "ui_render") { renderUIBlock(ev); return; }
  if (ev.type === "ui_update") { applyUIUpdate(ev); return; }
  if (ev.type === "ui_window") { applyUIWindow(ev); return; }
  flushPendingMd(); // FIX-590: the iteration ended - land the deferred renders
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
      // FIX-549: use the event's own timestamp. It used to be "now" at render
      // time, so a replay — a whole page painted in one tick — showed the same
      // clock value on every line.
      const parts = [seq + ". " + fmtTime(ev.ts ? new Date(ev.ts) : new Date())];
      const p = parseInt(m.prompt, 10) || 0;
      const c = parseInt(m.completion, 10) || 0;
      // FIX-545: same shape as the status bar — "↑N（tps, 用时） ↓N（tps, 用时）".
      // The old line printed the input duration twice (m.ft, then p/in_tps which
      // is algebraically the same number) and never showed the output duration.
      // Both durations now come straight from the backend (ft / out_dur).
      if (m.prompt) {
        parts.push("↑" + fmtNum(p) + "（" + perfPair(m.in_tps, m.ft) + "）");
      }
      if (m.completion) {
        parts.push("↓" + fmtNum(c) + "（" + perfPair(m.out_tps, m.out_dur) + "）");
      }
      line.textContent = parts.join("  ");
      // FEATURE-419: one token line per iteration, appended INSIDE the last
      // block of the iteration so it reads as that block's footer. Appending it
      // as a sibling left it floating in the stream's flex gap, visually
      // detached from the message it belongs to.
      const blocks = iterBlocks.slice();
      iterBlocks = [];
      if (blocks.length) {
        blocks[blocks.length - 1].parentElement.appendChild(line);
      } else if (prependingHistory && insertAnchor && insertAnchor.parentNode === streamB) {
        // FIX-509: while an older page is being prepended the iteration's blocks
        // were inserted above insertAnchor, so the token line must land there too.
        // Appending it to streamB put it at the very bottom of the stream, which
        // made every replayed page's token lines pile up under the newest message.
        streamB.insertBefore(line, insertAnchor);
      } else {
        streamB.appendChild(line);
      }
    } else {
      // FEATURE-419: task-level summary line, total first with prompt/completion
      // in parentheses, e.g. "Σ2,087,596 (↑2,084,349 ↓3,247)". Appended at the
      // end of the stream container (after the last block AND its per-iteration
      // token line), so the order is: block → iteration token line → summary.
      const p = parseInt(m.prompt, 10) || 0;
      const c = parseInt(m.completion, 10) || 0;
      const t = parseInt(m.total, 10) || 0;
      // FEATURE-419/FEATURE-516: the sigma is an inline SVG icon (not the "Σ"
      // glyph) so it matches the status-bar total and renders identically across
      // OSes/browsers.
      line.innerHTML = iconHTML("i-sum", "ico-inline") + fmtNum(t) + " (↑" + fmtNum(p) + " ↓" + fmtNum(c) + ")";
      // Append INSIDE the task's last block so the summary reads as that
      // block's footer instead of floating in the stream's flex gap.
      if (lastBlock) {
        lastBlock.parentElement.appendChild(line);
      } else {
        streamB.appendChild(line);
      }
    }
    curLLM = curThinking = curSup = null;
    // FIX-474: the LLM iteration ended — reset the tool-block LOOKUP tables so
    // the next iteration starts clean. Without this, iterToolBlocks/curTool/
    // toolBlockByName persist across iterations (only cleared at the final done
    // event), so an "orphan" block from an earlier iteration (created by a ⚙️
    // header but whose tool_call input event never arrived, e.g. plan tools that
    // skip meta) stays unfilled and steals the intent of a later tool call,
    // misplacing the intent onto the wrong TOOL block.
    //
    // FIX-508b: curTool itself is deliberately NOT cleared here. The backend
    // emits token_iter for the PREVIOUS iteration before the current iteration's
    // tool_call(input) event, so clearing curTool made that event see
    // fresh=true and open a second, empty TOOL block next to the one that
    // already held the streamed params. Keeping the reference lets the input
    // event reuse the existing block; the lookup tables are still reset so
    // FIX-474's intent-misplacement fix keeps working.
    toolBlockByName = {};
    iterToolBlocks = [];
    // FEATURE-409: the LLM iteration ended (token usage refreshed) — stop the
    // breathing dot on all blocks now, not only at the final done event.
    document.querySelectorAll(".ev-head.streaming").forEach((h) => h.classList.remove("streaming"));
    scrollStream();
    // FEATURE-419: each display block just completed (the "..." was removed) —
    // FIX-518: one coalesced workspace refresh instead of an immediate
    // loadTree() per iteration; the tree is ~6 MB here, so a burst of events
    // used to pin the CPU. See scheduleWorkspaceRefresh.
    scheduleWorkspaceRefresh();
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
      tokenStats.lastInDur = m.ft || "-";
      tokenStats.lastOutDur = m.out_dur || "-";
      // FEATURE-546: KV cache hit rate of this call, empty when the engine did
      // not report prompt_tokens_details (rendered as a reason, not as 0%).
      tokenStats.lastCacheRate = m.cache_rate || "";
      updateStatus();
      // FEATURE-482: the iteration's token usage is now known — backfill the
      // message-visualization chart with the blocks created this iteration.
      msgVizUsage = p; // the current prompt size = the actual context usage now
      msgVizFlush();
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
    // status badges without a manual reload (FIX-518: coalesced).
    scheduleWorkspaceRefresh();
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
    // FIX-591: no per-chunk follow-scroll here. The throttled markdown render
    // (renderMdNow) scrolls once per rendered update, which is the only moment
    // the content actually changes.
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
      // FIX-549: a concrete tool block ends the LLM text block before it. The
      // live view used to keep curLLM open, so text emitted after the tool call
      // was appended to the LLM block ABOVE the tool card, while a replay (no
      // cursors) drew it as a new block AFTER the tool card. Both paths now
      // agree on [LLM][TOOL][LLM] in strict emission order.
      unmarkStreaming(curLLM && curLLM.body);
      maybeCollapseEnded("llm");
      curLLM = null;
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
  // FIX-575: a persisted tool-stream block opens a new TOOL block, exactly like
  // the live tool_call_stream header does. This MUST be decided from the event
  // itself (missing phase = block start) and never from render state, otherwise
  // a replay can merge a tool call into the previous block or drop it.
  if (isToolStreamBlock(ev)) {
    openToolStreamBlock(ev, msgIndex);
    return;
  }
  if (ev.type === "tool_call" || (ev.type === "error" && ev.chan === "tool")) {
    const phase = ev.meta && ev.meta.phase;
    const fresh = !curTool || curTool.hasResult;
    // FIX-575: only an explicit input phase enters the input branch. The former
    // (!phase && ... && !fresh && !curTool.raw) sub-condition was a state
    // heuristic that silently swallowed a tool call whenever the previous block
    // already held text; the no-phase case is now handled above.
    if (phase === "input") {
      // FEATURE-400: the input-parameter sub-block is the params container;
      // the pre-execution summary no longer replaces the streamed params.
      //
      // FIX-508b: before opening a new block, try to reuse the block that the
      // preceding tool_call_stream event already created for this same tool
      // invocation. The backend emits token_iter for the previous iteration
      // between the two events, and that used to clear curTool, so this branch
      // saw fresh=true and opened a second, empty TOOL block beside the one
      // holding the streamed params. Matching on the tool name keeps the params
      // and the intent on a single block.
      //
      // FIX-509b: the match is deliberately strict — only an exact tool_name hit
      // is reused. An earlier version also fell back to "the first block that
      // has params but no intent yet", which was too loose: it let a later,
      // unrelated tool call (or attempt_completion) adopt a previous call's
      // block, so the title and the params ended up describing different calls.
      if (fresh) {
        const summaryForMatch = parseToolSummary(ev);
        let reuse = null;
        if (summaryForMatch && summaryForMatch.tool_name) {
          const byName = toolBlockByName[summaryForMatch.tool_name];
          if (byName && !byName._intentFilled) reuse = byName;
        }
        if (reuse) curTool = reuse;
        else {
          // FIX-549: same rule as the streaming path — a new tool block ends the
          // LLM text block before it, so live and replay agree on the order.
          unmarkStreaming(curLLM && curLLM.body);
          maybeCollapseEnded("llm");
          curLLM = null;
          curTool = newStreamBlock("tool", "TOOL", msgIndex);
        }
      }
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
          // FIX-509b: drop any badge left by an earlier event of the same tool
          // call. The input branch may reuse a block (see above), so without
          // this the badges accumulate one per event.
          const stale = head.querySelector(".risk-badge");
          if (stale) stale.remove();
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
      // only at the end of the whole LLM iteration). FIX-518: coalesced, so a
      // burst of tool calls triggers one refresh instead of one each.
      scheduleWorkspaceRefresh();
    }
    curLLM = curThinking = curSup = null;
    scrollStream();
    return;
  }

  curLLM = curThinking = curSup = null;
  const label = eventBlockLabel(ev);
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
  // FEATURE-496: when the active model is unavailable, the backend emits an
  // error event tagged with connectivity=1 before RunStream returns. Render
  // the error block as usual, then return the rejected instruction to the
  // input box so the user can review and re-submit it.
  if (ev.type === "error" && ev.meta && ev.meta.connectivity === "1") {
    if (lastSubmittedText) {
      input.value = lastSubmittedText;
      autoGrow();
      input.focus();
    }
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

// FEATURE-507: replay a page of persisted events after a refresh. The events
// are the same shape the live stream uses, so they go through renderEvent
// unchanged; afterwards the view scrolls to the newest message.
function renderHistory(msg) {
  const events = msg.events || [];
  if (!events.length) return;
  // FIX-518: hold back workspace refreshes for the whole replay — the events
  // below are historical, so nothing they change needs to be reflected in the
  // file tree while it runs; one refresh is issued when the replay ends.
  historyReplaying = true;
  historyOldestSeq = msg.oldest_seq || 0;
  historyHasMore = !!msg.has_more;
  // FEATURE-508: a page requested with the `before` cursor is an OLDER page and
  // must be inserted at the top of the stream, keeping the viewport anchored on
  // the content the user is currently reading. The initial page (no cursor) is
  // appended normally and scrolls to the bottom.
  const prepend = historyLoading;
  historyLoading = false;
  if (prepend) {
    historyInserting = true;
    const prevHeight = streamB.scrollHeight;
    const prevTop = streamB.scrollTop;
    // Every block created while insertAnchor is set lands before the current
    // oldest node, so the older page is prepended in order.
    insertAnchor = streamB.firstChild;
    prependingHistory = true;
    // FIX-578: the msgviz track gets the same stable anchor as the message
    // stream, so the prepended page's lines keep their chronological order.
    msgVizPrependAnchor = msgVizTrack ? msgVizTrack.firstChild : null;
    for (const ev of events) {
      try { renderEvent(ev); } catch (e) { /* skip malformed history entry */ }
    }
    msgVizPrependAnchor = null;
    prependingHistory = false;
    insertAnchor = null;
    ensureTopSentinel();
    // FIX-509: anchor the viewport on the content the user is reading. The page
    // is inserted ABOVE the current position, so shifting scrollTop down by
    // exactly the inserted height keeps the same message under the viewport.
    //
    // This must apply even when the user is at the very top. An earlier version
    // forced scrollTop = 0 in that case, which left the sentinel inside the
    // observer margin; the chain-load then fired again immediately and paged all
    // the way to the oldest event in one go. Anchoring instead moves the sentinel
    // out of view, so paging stops until the user scrolls up again.
    const delta = streamB.scrollHeight - prevHeight;
    if (delta > 0) streamB.scrollTop = prevTop + delta;
    historyInserting = false;
    updateBlockNav();
    endHistoryReplay();
    return;
  }
  for (const ev of events) {
    try { renderEvent(ev); } catch (e) { /* skip malformed history entry */ }
  }
  // The replayed blocks are already complete: stop any streaming indicator and
  // jump to the bottom so the user lands on the newest message.
  document.querySelectorAll(".ev-head.streaming").forEach((h) => h.classList.remove("streaming"));
  followOutput = true;
  scrollStream();
  updateBlockNav();
  endHistoryReplay();
}

// notifyUser surfaces a short backend notice in the message stream
// (FEATURE-548). It reuses the error-block rendering path.
function notifyUser(text) {
  if (!text) return;
  renderEvent({ type: "error", text: text, meta: {} });
}

function renderUserEcho(text, opts) {
  // A new user input (command / confirmation / selection) starts a fresh
  // REPL output block: reset curREPL so the next ui_text repl event opens a
  // new block instead of appending to the previous one (FIX-411).
  curREPL = null;
  // FIX-506 (续修): the YOU block's retry-from index must be the index of THIS
  // user message in the agent's message array. At this moment the backend has
  // not appended the message yet (it is appended inside agent.RunStream), so
  // lastMsgIndex still holds the PREVIOUS turn's last index — using it made
  // "retry from hello 2" pop back to the end of turn 1. Create the block with
  // no index and let the first event of this turn backfill it (see
  // backfillPendingUserIndex in renderEvent).
  // FEATURE-512: an answer collected by ask_user is still a user message, but
  // its title carries a marker so it is distinguishable from a typed command.
  const body = makeBlock("user-msg", opts && opts.answer ? "YOU · " + i18nT("answerTag", "Answer") : "YOU", "");
  // FIX-506 (续修): makeBlock() sets data-msg-index on the .ev box (and returns
  // the .ev-body), while the ⏪ button reads box.dataset.msgIndex and
  // truncateStreamFrom() queries .ev[data-msg-index]. The pending reference must
  // therefore be the BOX, not the body — writing the backfilled index to the
  // body left the box without the attribute, so the button sent an empty value
  // and the backend rejected it silently.
  const box = body.parentElement;
  box.dataset.msgIndexPending = "1";
  pendingUserBlock = box;
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
    // FEATURE-516: vector icon followed by a plain text node.
    chip.appendChild(mkIcon(kind === "image" ? "i-image" : "i-file", "ico-inline"));
    chip.appendChild(document.createTextNode((parts[0] || line) + (parts[2] ? " · " + parts[2] : "")));
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
  const showPlan = panelPrefs.plan !== false && (!!lastPlan || lastSubAgents.length > 0 || lastSkills.length > 0);
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

/* ---------- stream-title / msg-viz visibility toggles (menu) ---------- */

// The "会话标题" and "消息可视化" menu switches control whether the editable
// session title and the message-visualization chart show in the stream title
// bar. Persisted in localStorage (absent = visible).
const miStreamTitle = document.getElementById("miStreamTitle");
const miStreamTitleCheck = document.getElementById("miStreamTitleCheck");
const miMsgViz = document.getElementById("miMsgViz");
const miMsgVizCheck = document.getElementById("miMsgVizCheck");
let streamTitleOn = localStorage.getItem("co-shell-stream-title") !== "0";
let msgVizOn = localStorage.getItem("co-shell-msgviz") !== "0";

function applyStreamTitle() {
  if (streamTitle) streamTitle.style.display = streamTitleOn ? "" : "none";
  // FEATURE-487: the narrow-screen cycling widget replaces the editable input,
  // so it honours the same "会话标题" display toggle.
  if (streamTitleCycle) streamTitleCycle.style.display = streamTitleOn ? "" : "none";
  if (miStreamTitleCheck) miStreamTitleCheck.classList.toggle("on", streamTitleOn);
  stcSync();
}
function applyMsgViz() {
  if (msgViz) msgViz.style.display = msgVizOn ? "" : "none";
  if (miMsgVizCheck) miMsgVizCheck.classList.toggle("on", msgVizOn);
}

if (miStreamTitle) miStreamTitle.onclick = () => {
  streamTitleOn = !streamTitleOn;
  localStorage.setItem("co-shell-stream-title", streamTitleOn ? "1" : "0");
  applyStreamTitle();
};
if (miMsgViz) miMsgViz.onclick = () => {
  msgVizOn = !msgVizOn;
  localStorage.setItem("co-shell-msgviz", msgVizOn ? "1" : "0");
  applyMsgViz();
};

/* FEATURE-487: narrow-screen cycling session-title widget. On narrow screens
   the editable #streamTitle is hidden and #streamTitleCycle shows a single
   read-only line that flips every 2s between: the session title, the active
   task-plan goal (plan.description), the running step and the completion tally
   (done/total). When no task plan is active it shows the session title
   statically (no cycling). All text truncates with an ellipsis (CSS). */
let stcTimer = null;
let stcIdx = 0;

// stcItems builds the ordered display strings for the current state.
function stcItems() {
  const en = currentLang === "en";
  const colon = en ? ": " : "：";
  const title = (streamTitle && streamTitle.value.trim()) || "";
  const items = [title];
  if (lastPlan) {
    const goal = (lastPlan.description || "").trim() || (lastPlan.title || "").trim();
    if (goal) items.push(T.stcGoal + colon + goal);
    // Running step: first in_progress, else first pending.
    let run = "";
    for (const st of lastPlan.steps) {
      if (normStatus(st.status) === "in_progress") { run = st.description; break; }
    }
    if (!run) {
      for (const st of lastPlan.steps) {
        if (normStatus(st.status) === "pending") { run = st.description; break; }
      }
    }
    if (run) {
      const nl = run.indexOf("\n");
      items.push(T.stcRunning + colon + (nl === -1 ? run : run.slice(0, nl)));
    }
    const total = lastPlan.steps.length;
    const done = lastPlan.steps.filter((st) => normStatus(st.status) === "completed").length;
    items.push(T.stcProgress + colon + T.stcDone + "(" + done + "/" + total + ")");
  }
  return items;
}

// stcShow sets the visible line, optionally with a vertical flip animation.
function stcShow(text, animate) {
  if (!stcFace) return;
  if (!animate || stcFace.textContent === text) {
    stcFace.textContent = text;
    stcFace.classList.remove("flip-out", "flip-in");
    return;
  }
  stcFace.classList.add("flip-out");
  setTimeout(() => {
    stcFace.textContent = text;
    stcFace.classList.remove("flip-out");
    stcFace.classList.add("flip-in");
    setTimeout(() => stcFace.classList.remove("flip-in"), 200);
  }, 180);
}

// stcTick advances to the next item (with a flip) on the 2s interval.
function stcTick() {
  const items = stcItems();
  if (items.length <= 1) return;
  stcIdx = (stcIdx + 1) % items.length;
  stcShow(items[stcIdx], true);
}

// stcSync starts/stops the cycling timer based on the current state: it cycles
// only while narrow AND a task plan is active AND the title toggle is on;
// otherwise it shows the session title statically (or nothing when hidden).
function stcSync() {
  if (!streamTitleCycle || !stcFace) return;
  if (stcTimer) { clearInterval(stcTimer); stcTimer = null; }
  const narrow = document.body.classList.contains("narrow");
  const active = narrow && streamTitleOn && !!lastPlan;
  if (active) {
    const items = stcItems();
    stcIdx = 0;
    stcShow(items[0] || "", false);
    stcTimer = setInterval(stcTick, 2000);
  } else {
    const t = streamTitleOn ? ((streamTitle && streamTitle.value.trim()) || "") : "";
    stcShow(t, false);
  }
}

// Token stats accumulated from token_iter events. sessionIn/sessionOut are
// the running totals across all iterations; last* hold the most recent
// iteration's values (including input/output tokens-per-second).
const tokenStats = { sessionIn: 0, sessionOut: 0, lastIn: 0, lastOut: 0, lastInTPS: 0, lastOutTPS: 0, lastInDur: "-", lastOutDur: "-", lastCacheRate: "" };

/* FEATURE-482: message-visualization chart. Each rendered message block (.ev)
   becomes one 1px vertical line in #msgVizTrack, coloured by its title-bar
   indicator light; a pure-red dot's vertical position encodes the cumulative
   context usage when that message was produced. Lines are appended at the
   right edge (newest) and pan leftwards by dragging when they overflow. */
// Blocks created since the last token_iter, awaiting their context-usage value.
let msgVizPending = [];
// Cumulative context usage (tokens) at the most recent token_iter.
let msgVizUsage = 0;
// Horizontal pan offset (px) applied to the track when lines overflow.
let msgVizPan = 0;
// FIX-578: the track node a prepended (older) history page must insert its lines
// BEFORE. It is captured once when that page starts; inserting before the LIVE
// firstChild put each page's last line first, so every reloaded page drew its
// context-usage trend upside down (the trend line appeared to bounce up/down).
let msgVizPrependAnchor = null;

// msgVizColor maps a block to its title-bar indicator colour. The palette is
// FIXED to the dark-theme values (not read from the live CSS variables) so the
// chart's line colours stay identical whichever theme (dark/light/light-tp/
// paper) the UI is currently in. cls is the class string captured at makeBlock
// time; box is the live .ev element, whose later-added classes (e.g.
// level-error on a failed tool call) are also honoured so a failed tool shows
// red.
function msgVizColor(cls, box) {
  const bcls = box ? box.className : "";
  if (/level-error/.test(cls) || /level-error/.test(bcls)) return "#FF0000"; // FEATURE-487: failed = pure red
  if (/level-success/.test(cls) || /level-success/.test(bcls)) return "#4ade80"; // dark --ok
  if (/level-warning/.test(cls) || /level-warning/.test(bcls)) return "#facc15"; // dark --warn
  if (/user-msg/.test(cls)) return "transparent"; // user msg = empty gap between turns
  if (/llm/.test(cls)) return "#3fd6ef"; // dark --accent
  if (/tool/.test(cls)) return "#facc15"; // dark --warn
  if (/command/.test(cls)) return "#4ade80"; // dark --ok
  if (/repl/.test(cls)) return "#6b7280"; // dark --debug
  if (/supervisor/.test(cls)) return "#ff58f3";
  return "#5b6373"; // dark --fg-faint (system / thinking / default)
}

// msgVizTip builds the hover tooltip for a message-visualization line: the
// message type, the tool intent (for tool blocks) and the context-usage
// percentage (FEATURE-487).
function msgVizTip(cls, box, ratio) {
  const en = currentLang === "en";
  let type = "";
  if (/level-error/.test(cls)) type = en ? "Failed" : "失败";
  else if (/level-success/.test(cls)) type = en ? "Success" : "成功";
  else if (/level-warning/.test(cls)) type = en ? "Warning" : "警告";
  else if (/user-msg/.test(cls)) type = en ? "User" : "用户";
  else if (/llm/.test(cls)) type = en ? "LLM" : "LLM";
  else if (/tool/.test(cls)) type = en ? "Tool" : "工具";
  else if (/command/.test(cls)) type = en ? "Command" : "命令";
  else if (/repl/.test(cls)) type = en ? "REPL" : "REPL";
  else if (/supervisor/.test(cls)) type = en ? "Supervisor" : "监督";
  else type = en ? "Message" : "消息";
  const parts = [type];
  // Tool intent: read the block title label ("TOOL: action - intent").
  if (/tool/.test(cls) && box) {
    const label = box.querySelector(".ev-head-label");
    const t = (label ? label.textContent : "") || "";
    const intent = t.replace(/^TOOL:\s*/i, "").trim();
    if (intent) parts.push(intent);
  }
  const pct = Math.round((ratio || 0) * 100);
  parts.push((en ? "context " : "上下文 ") + pct + "%");
  return parts.join(" · ");
}

// msgVizFlush renders one line per pending block using the current context
// usage, then clears the pending list. Called when a token_iter arrives (the
// iteration's token usage is now known).
function msgVizFlush() {
  if (!msgViz || !msgVizTrack || !msgVizPending.length) { msgVizPending = []; return; }
  const max = (modelInfo && modelInfo.textMaxLen) || 0;
  const h = msgViz.clientHeight || 27;
  const dotH = 2; // FEATURE-487: the context-usage dot is 2px tall
  const range = Math.max(h - dotH, 1);
  const ratio = max > 0 ? Math.min(msgVizUsage / max, 1) : 0;
  const top = Math.round(range * (1 - ratio));
  const pending = msgVizPending;
  msgVizPending = [];
  for (const p of pending) {
    const line = document.createElement("div");
    line.className = "msgviz-line";
    // FEATURE-487: draw the line as a dotted vertical line (1px dot + 1px gap)
    // in the block colour; the 75% opacity is applied by CSS. The colour is set
    // as a CSS variable so the CSS can switch between dotted (default) and
    // solid (:hover) rendering.
    const col = msgVizColor(p.cls, p.box);
    line.style.setProperty("--line-color", col);
    line._box = p.box; // for click-to-locate in initMsgViz
    line._cls = p.cls; // for the hover tooltip
    const dot = document.createElement("div");
    dot.className = "msgviz-dot";
    dot.style.top = top + "px";
    line.appendChild(dot);
    // FEATURE-487: hovering shows a tooltip with the message type, tool intent
    // (if any) and the context-usage percentage.
    line.title = msgVizTip(p.cls, p.box, ratio);
    // Clicking a line scrolls the corresponding message block into view.
    line.addEventListener("click", (e) => {
      e.stopPropagation();
      if (p.box && p.box.scrollIntoView) p.box.scrollIntoView({ block: "nearest" });
    });
    // FIX-509/FIX-578: the track reads left-to-right in chronological order, so a
    // line of a prepended (older) page must land on the LEFT of the existing
    // lines while keeping the page's own order. The anchor captured when the page
    // started is that stable insert point (null = the track was empty → append).
    if (prependingHistory) msgVizTrack.insertBefore(line, msgVizPrependAnchor);
    else msgVizTrack.appendChild(line);
  }
  msgVizApplyPan();
}

// msgVizApplyPan clamps and applies the horizontal pan offset to the track.
function msgVizApplyPan() {
  if (!msgViz || !msgVizTrack) return;
  const overflow = msgVizTrack.scrollWidth - msgViz.clientWidth;
  if (overflow <= 0) msgVizPan = 0;
  else msgVizPan = Math.max(-overflow, Math.min(0, msgVizPan));
  msgVizTrack.style.transform = "translateX(" + msgVizPan + "px)";
}

// initMsgViz wires the drag-to-pan interaction on the chart.
function initMsgViz() {
  if (!msgViz || !msgVizTrack) return;
  let dragging = false, moved = false, startX = 0, startY = 0, startPan = 0;
  msgViz.addEventListener("pointerdown", (e) => {
    dragging = true; moved = false;
    startX = e.clientX; startY = e.clientY;
    startPan = msgVizPan;
    msgViz.classList.add("dragging");
    msgViz.setPointerCapture(e.pointerId);
  });
  msgViz.addEventListener("pointermove", (e) => {
    if (!dragging) return;
    if (Math.abs(e.clientX - startX) > 3 || Math.abs(e.clientY - startY) > 3) moved = true;
    msgVizPan = startPan + (e.clientX - startX);
    msgVizApplyPan();
  });
  const end = (e) => {
    dragging = false;
    msgViz.classList.remove("dragging");
    // A click (no drag) on a line locates its message block.
    if (!moved && e && e.type === "pointerup") {
      const el = document.elementFromPoint(e.clientX, e.clientY);
      const line = el && el.closest ? el.closest(".msgviz-line") : null;
      if (line && line._box && line._box.scrollIntoView) {
        line._box.scrollIntoView({ block: "nearest" });
      }
    }
  };
  msgViz.addEventListener("pointerup", end);
  msgViz.addEventListener("pointercancel", end);
  window.addEventListener("resize", msgVizApplyPan);
}

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
// activeMTP is the speculative-decoding (MTP) status of the model the status
// bar currently shows, taken from the model list the backend probed
// (FEATURE-546). Null means no model reported one.
let activeMTP = null;

// FEATURE-455: remote access + download flags from bootstrap. When the UI is
// served to a remote address (remote=true) and the operator enabled download
// (downloadEnabled=true), the workspace file list's "reveal in folder" icon
// becomes a download icon (and the folder reveal action disappears).
let remoteAccess = false;
let downloadEnabled = false;

function fmtNum(n) { return n ? n.toLocaleString() : "0"; }

// fmtPct formats a context-usage percentage (0-100) as e.g. "89%".
function fmtPct(used, max) {
  if (!(max > 0)) return "-";
  return Math.round(used * 100 / max) + "%";
}

// fmtPct's status-bar variant wraps the % in a .sb-unit span so the status bar
// renders it in the bright foreground colour (FEATURE-493). The plain fmtPct
// stays text-only because the message-stream token line sets textContent.
function sbPct(used, max) {
  const s = fmtPct(used, max);
  return s === "-" ? s : s.replace(/%$/, "<span class='sb-unit'>%</span>");
}

// FIX-545: durations are measured and formatted by the backend (meta keys ft and
// out_dur) and only decorated here. They used to be derived as tokens/tps, which
// lost precision to the rounded rate and collapsed to "-" whenever the rate was
// unavailable — so fmtDur is gone with it.

// perfPair renders one "<rate>t/s, <duration>" group as plain text for the
// message-stream token line.
function perfPair(tpsRaw, durRaw) {
  const tps = parseInt(tpsRaw, 10) || 0;
  const dur = durRaw && durRaw !== "-" ? durRaw : "-";
  return (tps > 0 ? fmtNum(tps) + "t/s" : "-") + ", " + dur;
}

// sbDur decorates a backend-formatted duration ("1.5s" / "2.5m") for the status
// bar by wrapping its unit in .sb-unit.
function sbDur(d) {
  return d && d !== "-" ? d.replace(/([sm])$/, "<span class='sb-unit'>$1</span>") : "-";
}

// sbPerfPair is perfPair's status-bar twin (HTML, units highlighted).
function sbPerfPair(tps, dur) {
  return (tps > 0 ? fmtNum(tps) + "<span class='sb-unit'>t/s</span>" : "-") + ", " + sbDur(dur);
}

// fmtLen formats a context length with K/M units (e.g. 1048576 -> "1M").
function fmtLen(n) {
  if (!(n > 0)) return "-";
  if (n >= 1000000) return (n / 1000000).toFixed(1).replace(/\.0$/, "") + "M";
  if (n >= 1000) return (n / 1000).toFixed(1).replace(/\.0$/, "") + "K";
  return String(n);
}

function updateStatus() {
  // Model context usage: brain{text}(89% of 1M) eye{vision}(50% of 1M), both
  // as inline SVG icons (FEATURE-516). The usage numerator is the last turn's
  // input+output tokens (FEATURE-378).
  const sIn = tokenStats.sessionIn, sOut = tokenStats.sessionOut;
  const total = sIn + sOut;
  const lastTotal = tokenStats.lastIn + tokenStats.lastOut;
  // FEATURE-422: the main (text) model and the vision model are separate
  // hover targets, each with its own selector menu. The model name + context
  // usage sit in a .sb-model-name span so narrow screens can hide the text and
  // keep only the brain/eye icon (FEATURE-487).
  // Show the configured model ID (fall back to the model name when the
  // backend does not provide the ID yet).
  const textModelLabel = (modelInfo && (modelInfo.textModelID || modelInfo.textModel)) || "";
  const visionModelLabel = (modelInfo && (modelInfo.visionModelID || modelInfo.visionModel)) || "";
  if (textModelLabel) {
    sbModelText.innerHTML = iconHTML("i-model-text", "ico-sb") + "<span class='sb-model-name'>" + textModelLabel + "(" + sbPct(lastTotal, modelInfo.textMaxLen) + " of " + fmtLen(modelInfo.textMaxLen) + ")</span>";
  } else {
    sbModelText.innerHTML = iconHTML("i-model-text", "ico-sb");
  }
  if (visionModelLabel) {
    sbModelVision.innerHTML = iconHTML("i-model-vision", "ico-sb") + "<span class='sb-model-name'>" + visionModelLabel + "(" + sbPct(lastTotal, modelInfo.visionMaxLen) + " of " + fmtLen(modelInfo.visionMaxLen) + ")</span>";
  } else {
    sbModelVision.innerHTML = iconHTML("i-model-vision", "ico-sb");
  }
  // Session: 会话 15000（↑14500 ↓500）
  sbSession.innerHTML = iconHTML("i-sum", "ico-sb") + "<b>" + fmtNum(total) + "</b>（↑" + fmtNum(sIn) + " ↓" + fmtNum(sOut) + "）";
  // Last turn: 最后一轮 ↑4500（2250t/s, 2s） ↓500（20t/s, 25s）
  // FIX-545: durations come straight from the backend (ft / out_dur) instead of
  // being back-derived as tokens/tps — that derivation collapsed to "-" for every
  // round whose rate was unavailable (pure tool-call rounds), which is what users
  // saw as "↓1,266 (-, -)". Both groups now use full-width parens so this line
  // and the message-stream token line are shaped identically.
  // FEATURE-436: token rates use thousands separators (e.g. 1,234t/s).
  // FEATURE-487: the ⏱️ icon sits in its own span so narrow screens can hide
  // just the icon while keeping the per-iteration usage text.
  sbLast.innerHTML = "<span class='sb-last-ico'>" + iconHTML("i-timer") + "</span>↑" + fmtNum(tokenStats.lastIn) + "（" + sbPerfPair(tokenStats.lastInTPS, tokenStats.lastInDur) + "） ↓" + fmtNum(tokenStats.lastOut) + "（" + sbPerfPair(tokenStats.lastOutTPS, tokenStats.lastOutDur) + "）";
}

/* ---------- session menu (FEATURE-387) ---------- */

let sessionList = [];

// renderSessionMenu renders the session list into the status-bar menu and
// updates the 💬 count. Called when the server pushes a "sessions" message.
function renderSessionMenu(sessions) {
  sessionList = sessions || [];
  // FEATURE-571: the sessions push also tells the window registry which
  // session it is on, which is what makes the per-session window store work.
  if (window.UIWin && typeof UIWin.setSession === "function") {
    const cur = sessionList.find((s) => s.current);
    if (cur) UIWin.setSession(cur.id);
  }
  sbSessions.innerHTML = iconHTML("i-sessions", "ico-sb") + sessionList.length;
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
      setIcon(del, "i-dot");
      del.title = T.sessionActive;
    } else {
      del.className = "session-del";
      setIcon(del, "i-close");
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
            // FEATURE-524 window mode: the window is not persisted, so a session
      // switch closes it instead of leaving a stale panel behind.
      // FEATURE-571: windows are kept per session, so a switch parks the
      // current ones and restores the target session's instead of deleting
      // them; the registry also mirrors the new list to the new agent.
      if (window.UIWin && typeof UIWin.setSession === "function") UIWin.setSession(s.id);
      else if (window.UI && typeof UI.closeWindow === "function") UI.closeWindow();
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
    const cur = sessionList.find((s) => s.current);
    // FEATURE-500: an empty title is sent as-is so the backend restores the
    // default placeholder title and unlocks it; a non-empty changed title is
    // sent for locking (the backend adds the "$" prefix).
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
      // Show only user-msg, error, and result-like blocks (the completion
      // report and LLM-rendered component trees).
      const show = cls === "user-msg" || box.classList.contains("level-error") || isResultLikeBlock(box, cls);
      box.style.display = show ? "" : "none";
      // Result-like blocks must be fully expanded (FIX-426).
      if (isResultLikeBlock(box, cls)) box.classList.remove("collapsed");
    } else if (displayMode === "minimal") {
      box.style.display = "";
      // User-msg blocks are always expanded (FIX-426).
      if (cls === "user-msg") { box.classList.remove("collapsed"); return; }
      // Collapse finished non-result blocks to just their title.
      const isResult = isResultLikeBlock(box, cls);
      if (!isStreaming && !isResult) box.classList.add("collapsed");
      else box.classList.remove("collapsed");
    } else {
      box.style.display = "";
      box.classList.remove("collapsed");
    }
  });
}

// FEATURE-487: the session list is now opened by clicking the 💬 item
// (bound in the narrow-screen module at the bottom of this file) instead of
// hovering. The session list is requested on connect and on open.

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

const STATUS_ICON = { pending: "i-circle", in_progress: "i-half", completed: "i-dot", cancelled: "i-close", failed: "i-xmark" };

function renderPlan(plan) {
  lastPlan = plan && plan.steps && plan.steps.length > 0 ? plan : null;
  applyPanels();
  // FEATURE-487: start/stop the cycling session-title widget when the plan
  // appears or disappears.
  stcSync();
  if (!lastPlan) return;
  planBody.textContent = "";

  // FEATURE-516: the title + summary sit in a height-capped scroll box (see
  // .plan-head-scroll) so a long summary cannot push the step list, which is
  // the part that matters most, out of view.
  const headBox = document.createElement("div");
  headBox.className = "plan-head-scroll";
  const title = document.createElement("div");
  title.className = "plan-title";
  title.textContent = plan.title || "";
  headBox.appendChild(title);

  if (plan.description) {
    const desc = document.createElement("div");
    desc.className = "plan-desc";
    desc.textContent = plan.description;
    headBox.appendChild(desc);
  }
  planBody.appendChild(headBox);

  // FEATURE-514: show the plan's acceptance criteria (if any).
  if (Array.isArray(plan.acceptance_criteria) && plan.acceptance_criteria.length > 0) {
    const ac = document.createElement("div");
    ac.className = "plan-ac";
    const head = document.createElement("div");
    head.className = "plan-ac-head";
    head.textContent = T.planAcceptance;
    ac.appendChild(head);
    plan.acceptance_criteria.forEach((c, i) => {
      const item = document.createElement("div");
      item.className = "plan-ac-item";
      item.textContent = (i + 1) + ". " + c;
      ac.appendChild(item);
    });
    planBody.appendChild(ac);
  }

  for (const st of plan.steps) {
    const status = normStatus(st.status);
    const row = document.createElement("div");
    row.className = "plan-step " + status;
    const icon = document.createElement("span");
    icon.className = "st";
    icon.innerHTML = iconHTML(STATUS_ICON[status] || "i-circle");
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

/* ---------- sub-agent status list (FEATURE-564) ---------- */

// renderSubAgents paints the current session's sub-agent list below the task
// plan: one line per sub-agent, "{id} {instruction}({done}/{total})". The
// instruction is truncated at 30 characters. An entry without a task plan
// shows (0/1) while the sub-agent is working (FEATURE-565: only running
// sub-agents are listed at all, so the panel disappears once the pool is idle).
function renderSubAgents(rows) {
  // The backend already filters on running, filter again here so a stale or
  // hand-crafted payload can never leave a finished sub-agent on the panel.
  lastSubAgents = (Array.isArray(rows) ? rows : []).filter((row) => row && row.status === 'running');
  subAgentBody.textContent = '';
  const show = lastSubAgents.length > 0;
  // The markup ships both blocks with class="hidden", so the toggle must go
  // through the class list: setting the hidden *attribute* left the class in
  // place (display:none) and the whole list stayed invisible.
  subAgentHead.classList.toggle('hidden', !show);
  subAgentBody.classList.toggle('hidden', !show);
  for (const row of lastSubAgents) {
    const line = document.createElement('div');
    line.className = 'subagent-item';
    const id = document.createElement('span');
    id.className = 'sa-id';
    id.textContent = row.id || '';
    const inst = document.createElement('span');
    inst.className = 'sa-inst';
    inst.textContent = truncateInstruction(row.instruction);
    inst.title = row.instruction || '';
    const prog = document.createElement('span');
    prog.className = 'sa-prog';
    prog.textContent = '(' + (row.done || 0) + '/' + (row.total || 1) + ')';
    line.appendChild(id);
    line.appendChild(inst);
    line.appendChild(prog);
    subAgentBody.appendChild(line);
  }
  applyPanels();
}

// truncateInstruction shortens an instruction to 30 characters and appends
// "..." when it was cut.
function truncateInstruction(text) {
  const chars = Array.from(String(text || '').trim());
  if (chars.length <= 30) return chars.join('');
  return chars.slice(0, 30).join('') + '...';
}

/* ---------- loaded skills panel (FEATURE-583, R2) ---------- */

// renderSkills paints the skills whose SKILL.md content is currently pinned in
// the context. loaded is the "loaded" array of a skill_state event (or the
// loaded subset of /api/skills). The whole section is hidden when nothing is
// loaded.
function renderSkills(loaded) {
  lastSkills = Array.isArray(loaded) ? loaded.filter((s) => s && s.name) : [];
  skillBody.textContent = "";
  const show = lastSkills.length > 0;
  skillHead.classList.toggle("hidden", !show);
  skillBody.classList.toggle("hidden", !show);
  for (const s of lastSkills) {
    const line = document.createElement("div");
    line.className = "skill-item";
    const name = document.createElement("span");
    name.className = "sk-name";
    name.textContent = s.name || "";
    const src = document.createElement("span");
    src.className = "sk-src";
    src.textContent = s.source === "global" ? T.skillSourceGlobal : T.skillSourceWorkspace;
    line.appendChild(name);
    line.appendChild(src);
    // FIX-587: the SKILL.md size and the sha256 prefix are no longer rendered
    // here (they squeezed the name on narrow panels); hovering the row shows
    // the skill card description instead, and the size stays on the Skills
    // settings page.
    if (s.description) line.title = s.description;
    skillBody.appendChild(line);
  }
  applyPanels();
}

// refreshSkills rebuilds the loaded-skills panel from GET /api/skills. It runs
// on connect and after a session switch, where the recorded skill_state events
// are not guaranteed to be replayed for the newly current session.
function refreshSkills() {
  fetch("api/skills").then((r) => r.json()).then((j) => {
    if (!j || !Array.isArray(j.skills)) return;
    renderSkills(j.skills.filter((s) => s.loaded));
  }).catch(() => { /* endpoint unavailable: keep the current panel */ });
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
    // FEATURE-409: the question (ask_user) lives in title and
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

  // FEATURE-512: multi-question form (ask_user) — one card per question, with
  // radio/checkbox options, per-option notes and a single submit button.
  if (it.kind === "questions" && it.questions && it.questions.length) {
    renderQuestions(it);
    scrollStream();
    return;
  }

  if (it.kind === "select" && ((it.options && it.options.length) || (it.keys && it.keys.length))) {
    // FEATURE-399: render each option as a virtual-keyboard-style square key
    // (number 1..N) with the option text beside it. Clicking a key or pressing
    // the physical number key selects that option. A keys-only select (simple
    // completion mode, FIX-480) renders just its fixed [Key] Label buttons.
    renderVirtualKeyboard(it, true);
  } else if (it.kind === "confirm") {
    // Option buttons below.
    renderVirtualKeyboard(it, false);
  }

  // FEATURE-462: the dedicated supplement input box is removed. Supplement is
  // typed in the main input box (clicking it cancels shortcut-key monitoring);
  // a hint line below the option buttons tells the user they can long-press to
  // supplement.

  // Free input for the pure-input kind (ask_user without options).
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

/* ---------- FEATURE-512: multi-question form (ask_user) ---------- */

// qsState mirrors the in-progress answers of the current multi-question form:
// one entry per question — { selected: Set<string>, notes: Map<string,string>, text: string }.
let qsState = [];
// qsHotkeys holds the option hotkey badges of each question (only the active
// question shows them); qsKeyHandler is the active keyboard listener.
let qsHotkeys = [];

// renderQuestions renders an ask_user form: one card per question with
// radio/checkbox options, an optional per-option note input, and a single
// submit button sending every answer back at once. Questions are separated by
// a divider so several questions stay visually distinct (FEATURE-512).
function renderQuestions(it) {
  const total = it.questions.length;
  qsState = it.questions.map(() => ({ selected: new Set(), notes: new Map(), text: "" }));

  // FEATURE-512: keyboard navigation state. Only the active question renders its
  // 1-9 option hotkeys; "-"/"+" move between questions and Space opens the note
  // input of the active question.
  if (window.__qsKey) {
    window.removeEventListener("keydown", window.__qsKey, true);
    window.__qsKey = null;
  }
  if (window.__qsKeyUp) {
    window.removeEventListener("keyup", window.__qsKeyUp, true);
    window.__qsKeyUp = null;
  }
  qsHotkeys = qsState.map(() => []);
  const qSyncers = qsState.map(() => []);
  const qCards = [];
  const qBadges = qsState.map(() => null);
  const qHints = qsState.map(() => null);
  let activeQ = 0;
  // qsWarn shows why a submit was rejected (unanswered required questions).
  let qsWarn = null;

  // FEATURE-517: focus-transition state. Moving the focus between questions has
  // to be visible: the key caps and the hint line fade out before they are
  // hidden and fade in right after they are shown, the Qn badge cross-fades in
  // CSS, and the focused question is scrolled into place smoothly. QS_FADE_MS
  // mirrors the 0.45s transitions set on .qs-index / .qs-hotkey / .qs-hint and
  // is the rhythm shared by the fades and the scroll (300ms → 450ms in v0.56.0
  // after the first hands-on review asked for a longer, clearer focus move).
  const QS_FADE_MS = 450;
  const qFadeTimers = new Map();
  let qsFirstActivation = true;

  // fadeToggle shows (on=true) or hides (on=false) one hotkey / hint element
  // with a fade. .hidden is display:none, which cannot be transitioned, so a
  // hiding element keeps its slot (opacity only) until the fade ends and only
  // then collapses — exactly the resting layout of the pre-FEATURE-517 form.
  // `instant` skips the transition and is used for the initial activation, so a
  // freshly rendered form does not animate in.
  const fadeToggle = (el, on, instant) => {
    if (!el) return;
    const pending = qFadeTimers.get(el);
    if (pending) { clearTimeout(pending); qFadeTimers.delete(el); }
    if (on) {
      const settled = !el.classList.contains("hidden") && !el.classList.contains("qs-fading");
      el.classList.remove("hidden");
      if (settled) return;
      if (instant) { el.classList.remove("qs-fading"); return; }
      el.classList.add("qs-fading");
      void el.offsetWidth;                 // flush the 0-opacity start style
      el.classList.remove("qs-fading");
      return;
    }
    if (el.classList.contains("hidden") && !el.classList.contains("qs-fading")) return;
    if (instant) { el.classList.add("hidden"); el.classList.remove("qs-fading"); return; }
    el.classList.add("qs-fading");
    qFadeTimers.set(el, setTimeout(() => {
      qFadeTimers.delete(el);
      el.classList.add("hidden");
      el.classList.remove("qs-fading");
    }, QS_FADE_MS));
  };

  // qsScrollTo animates the ask area so the focused question sits at its top.
  // scrollIntoView({behavior:"smooth"}) computes its target once, but the
  // previous question's key caps / hint line only collapse when their fade ends
  // (QS_FADE_MS later), which shrinks that card and shifts every card below it
  // — the pre-computed target then lands ~70px off (measured). Re-reading the
  // target every frame keeps the focused card exactly at the top for the whole
  // transition while still reading as a smooth scroll. `instant` skips the
  // animation for the initial activation.
  let qsScrollRAF = 0;
  const qsScrollTo = (card, instant) => {
    if (!card || !askArea) return;
    const want = () =>
      askArea.scrollTop + card.getBoundingClientRect().top - askArea.getBoundingClientRect().top;
    if (qsScrollRAF) { cancelAnimationFrame(qsScrollRAF); qsScrollRAF = 0; }
    if (instant) { askArea.scrollTop = want(); return; }
    // Exponential ease-out: ~18% of the remaining distance per frame, so the
    // scroll settles in roughly the same 450ms as the fades (0.25/frame, the
    // initial value, was ~300ms). An exponential follow is used rather than a
    // fixed-duration tween because the target moves when the previous
    // question's key caps collapse — following absorbs that shift smoothly
    // instead of re-interpolating against a stale origin.
    const deadline = performance.now() + QS_FADE_MS * 2;
    const step = (now) => {
      const delta = want() - askArea.scrollTop;
      if (Math.abs(delta) < 0.5 || now > deadline) { qsScrollRAF = 0; return; }
      askArea.scrollTop += delta * 0.18;
      qsScrollRAF = requestAnimationFrame(step);
    };
    qsScrollRAF = requestAnimationFrame(step);
  };

  const submit = document.createElement("button");
  submit.type = "button";
  submit.className = "qs-submit";

  // requiredMissing lists the required questions still unanswered; the form
  // cannot be submitted while it is non-empty (FEATURE-512).
  const requiredMissing = () =>
    it.questions.reduce((acc, q, qi) => {
      const st = qsState[qi];
      if (q.required && st.selected.size === 0 && st.text.trim() === "") acc.push(qi);
      return acc;
    }, []);

  const refreshSubmit = () => {
    const done = qsState.filter((s) => s.selected.size > 0 || s.text.trim() !== "").length;
    submit.textContent = "";
    submit.appendChild(kbdSpan(i18nT("qsKeyEnter", "Enter")));
    submit.appendChild(document.createTextNode(
      " " + i18nT("qsConfirm", "Confirm") + " " +
      i18nT("qsSubmitCount", "(%d/%d)").replace("%d", String(done)).replace("%d", String(total))
    ));
    // Clear the highlight of required questions the user has answered meanwhile.
    const missing = requiredMissing();
    it.questions.forEach((q, qi) => {
      if (q.required && qCards[qi] && missing.indexOf(qi) < 0) qCards[qi].classList.remove("qs-missing");
    });
    if (!missing.length && qsWarn) {
      qsWarn.textContent = "";
      qsWarn.classList.add("hidden");
    }
  };

  // FEATURE-512: shared submit path for the 确定 button and Enter on the last
  // question. Required questions are validated first; the collected answers are
  // echoed as a user message.
  const submitAnswers = () => {
    const missing = requiredMissing();
    if (missing.length) {
      missing.forEach((qi) => qCards[qi] && qCards[qi].classList.add("qs-missing"));
      if (qsWarn) {
        qsWarn.textContent = i18nT("qsRequiredMissing", "Required question unanswered") + " " +
          missing.map((qi) => "Q" + (qi + 1)).join(", ");
        qsWarn.classList.remove("hidden");
      }
      setActive(missing[0]);
      return;
    }
    const answers = it.questions.map((q, qi) => {
      const st = qsState[qi];
      const notes = [];
      st.notes.forEach((note, opt) => {
        if (note.trim() !== "" && st.selected.has(opt)) notes.push({ option: opt, note: note.trim() });
      });
      return {
        question: q.title || "",
        selected: Array.from(st.selected),
        notes: notes,
        text: st.text.trim(),
      };
    });
    answerInteraction({ action: "submit", answers: answers, value: answersAsText(it.questions, answers) });
  };

  it.questions.forEach((q, qi) => {
    const card = document.createElement("div");
    card.className = "qs-card";

    const head = document.createElement("div");
    head.className = "qs-head";
    const badge = document.createElement("span");
    badge.className = "qs-index";
    badge.textContent = "Q" + (qi + 1);
    qBadges[qi] = badge;
    head.appendChild(badge);
    const title = document.createElement("div");
    title.className = "qs-title md";
    mdRender(title, q.title || "");
    // FEATURE-512: the "required" badge sits at the end of the question text
    // (in front it crowded the Qn badge) and is validated on submit. mdRender
    // wraps the text in block elements, so the badge goes into the last block
    // to stay on the same line as the end of the text.
    if (q.required) {
      const req = document.createElement("span");
      req.className = "qs-req";
      req.textContent = i18nT("qsRequired", "Required");
      (title.lastElementChild || title).appendChild(req);
    }
    head.appendChild(title);
    card.appendChild(head);

    const syncers = qSyncers[qi];

    if (q.options && q.options.length) {
      const tag = document.createElement("div");
      tag.className = "qs-tag";
      tag.textContent = q.multi ? i18nT("qsMulti", "multi-select") : i18nT("qsSingle", "single choice");
      card.appendChild(tag);

      const list = document.createElement("div");
      list.className = "qs-options";

      q.options.forEach((opt, optIndex) => {
        const row = document.createElement("div");
        row.className = "qs-option";
        // FEATURE-512: option hotkey key cap. It shows only for the active
        // question and is itself clickable (click selects the option, holding
        // it opens that option's note), mirroring the physical digit key.
        const hot = document.createElement("button");
        hot.type = "button";
        hot.className = "qs-hotkey hidden";
        hot.textContent = String(optIndex + 1);
        qsHotkeys[qi].push(hot);
        let hotTimer = null;
        let hotLong = false;
        const clearHotTimer = () => {
          if (hotTimer) { clearTimeout(hotTimer); hotTimer = null; }
        };
        hot.addEventListener("mousedown", (e) => {
          e.stopPropagation();
          setActive(qi);
          hotLong = false;
          hotTimer = setTimeout(() => {
            hotTimer = null;
            hotLong = true;
            openNoteFor(qi, opt);
          }, 450);
        });
        hot.addEventListener("mouseup", clearHotTimer);
        hot.addEventListener("mouseleave", clearHotTimer);
        hot.addEventListener("click", (e) => {
          e.preventDefault();
          e.stopPropagation();
          clearHotTimer();
          hot.blur();
          if (hotLong) { hotLong = false; return; }
          setActive(qi);
          pickOptionByDigit(optIndex + 1);
        });
        const mark = document.createElement("span");
        mark.className = "qs-mark";
        const label = document.createElement("span");
        label.className = "qs-label";
        mdInline(opt).forEach((n) => label.appendChild(n));
        row.appendChild(hot);
        row.appendChild(mark);
        row.appendChild(label);

        const noteBox = document.createElement("input");
        noteBox.type = "text";
        noteBox.className = "qs-note-input hidden";
        noteBox.placeholder = i18nT("qsNotePlaceholder", "Add a note (optional)");
        noteBox.addEventListener("keydown", (e) => {
          e.stopPropagation();
          if (e.key === "Enter") noteBox.blur();
        });
        noteBox.addEventListener("input", () => qsState[qi].notes.set(opt, noteBox.value));

        if (q.allow_note !== false) {
          const noteBtn = document.createElement("button");
          noteBtn.type = "button";
          noteBtn.className = "qs-note-btn";
          noteBtn.textContent = i18nT("qsNote", "Add note");
          noteBtn.addEventListener("click", (e) => {
            e.stopPropagation();
            const hidden = noteBox.classList.contains("hidden");
            noteBox.classList.toggle("hidden", !hidden);
            if (hidden) {
              if (!qsState[qi].selected.has(opt)) toggleOption(qi, opt, q, syncers, refreshSubmit);
              noteBox.focus();
            }
          });
          row.appendChild(noteBtn);
        }

        row.addEventListener("click", () => toggleOption(qi, opt, q, syncers, refreshSubmit));

        syncers.push(() => {
          const on = qsState[qi].selected.has(opt);
          mark.innerHTML = iconHTML(on ? (q.multi ? "i-checkbox-on" : "i-radio-on") : (q.multi ? "i-checkbox-off" : "i-circle"));
          row.classList.toggle("on", on);
        });

        list.appendChild(row);
        list.appendChild(noteBox);
      });
      card.appendChild(list);
      // FEATURE-512: per-question note input (Space opens it), appended to the
      // answer as free text for this question.
      const freeNote = document.createElement("input");
      freeNote.type = "text";
      freeNote.className = "qs-free qs-free-note hidden";
      freeNote.placeholder = i18nT("qsFreeNotePlaceholder", "Add a note for this question");
      freeNote.addEventListener("keydown", (e) => {
        e.stopPropagation();
        if (e.key === "Enter") freeNote.blur();
      });
      freeNote.addEventListener("input", () => {
        qsState[qi].text = freeNote.value;
        refreshSubmit();
      });
      card._freeNote = freeNote;
      card.appendChild(freeNote);
    } else {
      const inp = document.createElement("input");
      inp.type = "text";
      inp.className = "qs-free";
      inp.placeholder = i18nT("qsFreePlaceholder", "Type your answer…");
      inp.addEventListener("keydown", (e) => {
        e.stopPropagation();
        // Enter on a free-text question advances to the next one (the last
        // question submits), matching the option-hotkey behaviour.
        if (e.key === "Enter") {
          e.preventDefault();
          inp.blur();
          goNext();
        }
      });
      inp.addEventListener("input", () => {
        qsState[qi].text = inp.value;
        refreshSubmit();
      });
      card.appendChild(inp);
    }

    syncers.forEach((fn) => fn());
    const qh = document.createElement("div");
    qh.className = "qs-hint hidden";
    qHints[qi] = qh;
    card.appendChild(qh);
    qCards[qi] = card;
    askInteraction.appendChild(card);
  });

  // FEATURE-512: hotkey navigation. Only the active question shows its 1-9
  // option badges and its hint line. Enter advances to the next question (on
  // the last one it submits), "+" goes back, digits select an option, holding a
  // digit opens that option's note input, Space opens the note input of the
  // active question.
  let holdTimer = null;
  let holdKey = null;
  const clearHold = () => {
    if (holdTimer) { clearTimeout(holdTimer); holdTimer = null; }
    holdKey = null;
  };
  function goNext() {
    if (activeQ >= total - 1) { submitAnswers(); return; }
    setActive(activeQ + 1);
  }
  // openNoteFor reveals the note input of one option (opt != null) or the
  // per-question note input (opt == null) and focuses it.
  const openNoteFor = (qi, opt) => {
    const card = qCards[qi];
    if (!card) return;
    let el = null;
    if (opt != null) {
      const opts = it.questions[qi].options || [];
      const idx = opts.indexOf(opt);
      const list = card.querySelector(".qs-options");
      if (idx >= 0 && list) el = list.querySelectorAll(".qs-note-input")[idx] || null;
    }
    if (!el) el = card._freeNote || card.querySelector(".qs-free");
    if (!el) return;
    el.classList.remove("hidden");
    el.focus();
  };
  // cancelForm triggers the form's fixed exit key ("-" = think it over, exit
  // for now), the same action as the bottom-bar cancel button.
  const cancelForm = () => {
    const k = (it.keys || []).find((x) => x.value === "think_exit") || (it.keys || [])[0];
    if (k) answerInteraction({ action: "select", value: k.value });
  };
  // pickOptionByDigit selects the n-th option of the active question (a single
  // choice then advances, exactly like pressing the digit key).
  const pickOptionByDigit = (n) => {
    const q = it.questions[activeQ];
    if (!q || !q.options || n > q.options.length) return;
    toggleOption(activeQ, q.options[n - 1], q, qSyncers[activeQ], refreshSubmit);
    if (!q.multi) goNext();
  };
  // renderHint rebuilds the hint line of the active question (FEATURE-512). Its
  // keys mirror what is available for that question, and every key cap is
  // clickable: digits pick an option (or open its note), Enter advances/submits,
  // "+" goes back, Space opens the note of the question and "-" cancels the
  // whole form.
  const renderHint = (qi) => {
    const el = qHints[qi];
    const q = it.questions[qi];
    if (!el || !q) return;
    el.textContent = "";
    const opts = q.options || [];
    const parts = [];
    if (opts.length) {
      // The option rows already carry their 1..N key caps, so the hint only
      // spells out the actions instead of repeating the digits.
      parts.push({ text: i18nT("qsHintDigitSelect", "digit keys select") + " · " });
      parts.push({ text: i18nT("qsHintHoldNote", "hold an option digit for its note") + " · " });
    }
    parts.push({ kbd: i18nT("qsKeyEnter", "Enter"), click: () => goNext() });
    parts.push({ text: " " + (qi === total - 1 ? i18nT("qsConfirm", "Confirm") : i18nT("qsHintNext", "Next")) + " · " });
    parts.push({ kbd: "+", click: () => setActive(activeQ - 1) });
    parts.push({ text: " " + i18nT("qsHintPrev", "previous") + " · " });
    parts.push({ kbd: i18nT("qsKeySpace", "Space"), click: () => openNoteFor(activeQ, null) });
    parts.push({ text: " " + i18nT("qsHintSpace", "note") + " · " });
    parts.push({ kbd: "-", click: cancelForm });
    parts.push({ text: " " + i18nT("cancel", "Cancel") });
    parts.forEach((p) => {
      if (p.kbd != null) el.appendChild(kbdSpan(p.kbd, p.click));
      else el.appendChild(document.createTextNode(p.text));
    });
  };
  const setActive = (idx) => {
    activeQ = Math.max(0, Math.min(total - 1, idx));
    const instant = qsFirstActivation;
    qsFirstActivation = false;
    it.questions.forEach((_, qi) => {
      const on = qi === activeQ;
      (qsHotkeys[qi] || []).forEach((h) => fadeToggle(h, on, instant));
      if (qBadges[qi]) qBadges[qi].classList.toggle("hot", on);
      if (qHints[qi]) fadeToggle(qHints[qi], on, instant);
      if (qCards[qi]) qCards[qi].classList.toggle("active", on);
    });
    renderHint(activeQ);
    // The submit button only applies to the last question: on the others Enter
    // means "next question" (FEATURE-512).
    submit.classList.toggle("hidden", activeQ !== total - 1);
    // Keep the focused question readable: anchor it to the top of the ask area
    // so its options and hint line are visible at once. FEATURE-517 scrolls
    // there with a short animation so the move is followable instead of a cut;
    // the first activation still positions the form instantly (no animation on
    // appearance).
    qsScrollTo(qCards[activeQ], instant);
    // A free-text question takes focus immediately; Enter then advances.
    const q = it.questions[activeQ];
    if (q && (!q.options || !q.options.length) && card) {
      const inp = card.querySelector(".qs-free");
      if (inp) inp.focus();
    }
  };
  const onQsKey = (e) => {
    const t = e.target;
    if (t && (t.tagName === "INPUT" || t.tagName === "TEXTAREA")) return;
    const q = it.questions[activeQ];
    if (!q) return;
    if (e.key === "Enter") {
      goNext();
    } else if (e.key === "-") {
      // FEATURE-512: the "-" key cancels the whole form, like the bottom-bar
      // cancel button (previously only the rendered key cap worked).
      cancelForm();
    } else if (e.key === "+") {
      setActive(activeQ - 1);
    } else if (e.key === " ") {
      openNoteFor(activeQ, null);
    } else if (e.key >= "1" && e.key <= "9") {
      const n = parseInt(e.key, 10);
      if (!q.options || n > q.options.length) return;
      const opt = q.options[n - 1];
      toggleOption(activeQ, opt, q, qSyncers[activeQ], refreshSubmit);
      // Long-press (>=450ms) opens the note input of that very option; short
      // press just selects. Key auto-repeat keeps the timer alive.
      if (holdKey !== e.key) {
        clearHold();
        holdKey = e.key;
        holdTimer = setTimeout(() => {
          holdTimer = null;
          openNoteFor(activeQ, opt);
        }, 450);
      }
      if (!q.multi) goNext();
    } else {
      return;
    }
    e.preventDefault();
    e.stopPropagation();
  };
  const onQsKeyUp = (e) => {
    if (holdKey && e.key === holdKey) clearHold();
  };
  window.addEventListener("keydown", onQsKey, true);
  window.addEventListener("keyup", onQsKeyUp, true);
  window.__qsKey = onQsKey;
  window.__qsKeyUp = onQsKeyUp;
  setActive(0);

  const bar = document.createElement("div");
  bar.className = "qs-bar";
  submit.addEventListener("click", submitAnswers);
  bar.appendChild(submit);
  qsWarn = document.createElement("span");
  qsWarn.className = "qs-warn hidden";
  bar.appendChild(qsWarn);

  // Fixed keys (e.g. "-" think it over, exit for now) cancel/close the form.
  (it.keys || []).forEach((k) => {
    const b = document.createElement("button");
    b.type = "button";
    b.className = "qs-key";
    // FEATURE-512: show the trigger key as a key cap in front of the label.
    b.appendChild(kbdSpan(k.key || ""));
    b.appendChild(document.createTextNode(" " + (k.label || k.key)));
    b.addEventListener("click", () => answerInteraction({ action: "select", value: k.value }));
    bar.appendChild(b);
  });
  askInteraction.appendChild(bar);

  refreshSubmit();
}

// kbdSpan builds a key-cap element for the ask_user hint line and the bottom
// action buttons (FEATURE-512). When onClick is given the cap is rendered as a
// clickable button that performs the very action the key stands for.
function kbdSpan(text, onClick, hint) {
  const k = document.createElement(onClick ? "button" : "span");
  k.className = "qs-kbd";
  k.textContent = text;
  if (onClick) {
    k.type = "button";
    if (hint) k.title = hint;
    k.addEventListener("click", (e) => {
      e.preventDefault();
      e.stopPropagation();
      k.blur(); // keep the keyboard shortcuts working right after the click
      onClick();
    });
  }
  return k;
}

// answersAsText renders collected answers in the compact form used both in the
// transcript and by the backend when reporting to the model (FEATURE-512):
// "Q1: A + C（备注: A→xxx）", "Q2: 自由文本", "Q3: （未作答）".
function answersAsText(questions, answers) {
  return answers.map((a, i) => {
    const parts = [];
    if (a.selected && a.selected.length) parts.push(a.selected.join(" + "));
    if (a.text) parts.push(a.text);
    let line = "Q" + (i + 1) + ": " + (parts.length ? parts.join(" ") : i18nT("qsNoAnswer", "(not answered)"));
    const notes = (a.notes || []).map(
      (n) => "（" + i18nT("qsNoteLabel", "Note") + ": " + n.option + "→" + n.note + "）"
    );
    return notes.length ? line + notes.join("") : line;
  }).join("\n");
}

// toggleOption applies a click on one option of a multi-question form, keeping
// single-choice questions exclusive and dropping notes of deselected options.
function toggleOption(qi, opt, q, syncers, refresh) {
  const st = qsState[qi];
  if (q.multi) {
    if (st.selected.has(opt)) st.selected.delete(opt);
    else st.selected.add(opt);
  } else {
    st.selected.clear();
    st.selected.add(opt);
  }
  if (!st.selected.has(opt)) st.notes.delete(opt);
  syncers.forEach((fn) => fn());
  refresh();
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
  } else if (isSelect && it.keys && it.keys.length) {
    // Keys-only select (simple completion mode, FIX-480): register the fixed
    // key options so pressing the physical key triggers the select action.
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
    // (ask_user). Clicking it (or pressing Space/Insert/0) enters
    // supplement-input mode so the user can type extra info in the main box.
    addItem("空格/Ins/0", T.supplement, () => sendSupplement(), "opt-space");
  } else if (isSelect && it.keys && it.keys.length) {
    // Keys-only select (simple completion mode, FIX-480): render just the two
    // fixed [Key] Label buttons (confirm complete / continue with more input),
    // mirroring the TUI askSelect. No numbered options and no extra supplement
    // button — the "continue" key already covers adding more input.
    (it.keys || []).forEach((k) => {
      addItem(k.key, k.label, () => {
        // "exit" (confirm complete): send the select answer so the backend
        // marks the task completed (FIX-480).
        if (k.value === "exit") {
          answerInteraction({ action: "select", value: "exit" });
          return;
        }
        // "continue" (or any other fixed key) sends its value back so the
        // backend prompts for more input and keeps the loop going.
        answerSelectWithSupplement(k.value);
      }, "", () => longPressSupplement(k.label));
    });
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
  // FEATURE-512: drop the multi-question hotkey listeners with the form.
  if (window.__qsKey) {
    window.removeEventListener("keydown", window.__qsKey, true);
    window.__qsKey = null;
  }
  if (window.__qsKeyUp) {
    window.removeEventListener("keyup", window.__qsKeyUp, true);
    window.__qsKeyUp = null;
  }
  wsSend({ type: "interaction_answer", id: pendingInteraction, result });
  if (result.value) renderUserEcho(result.value, { answer: !!(result && result.answers) });
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
// cacheStreamWindowSettings reads the two sliding-window thresholds from the
// settings_get payload (FEATURE-508). Invalid or missing values keep the
// current defaults so the stream is never left unbounded.
function cacheStreamWindowSettings(groups) {
  for (const g of groups || []) {
    for (const it of g.items || []) {
      if (it.key === "stream-window-max-blocks") {
        const n = parseInt(it.value, 10);
        if (Number.isFinite(n) && n > 0) streamWindowMaxBlocks = n;
      } else if (it.key === "stream-window-max-nodes") {
        const n = parseInt(it.value, 10);
        if (Number.isFinite(n) && n > 0) streamWindowMaxNodes = n;
      } else if (it.key === "stream-window-max-chars") {
        const n = parseInt(it.value, 10);
        if (Number.isFinite(n) && n > 0) streamWindowMaxChars = n;
      } else if (it.key === "page-buffer-size") {
        const n = parseInt(it.value, 10);
        if (Number.isFinite(n) && n > 0) pageBufferSize = n;
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
  rm.type = "button"; rm.className = "attach-rm"; rm.innerHTML = iconHTML("i-close");
  rm.title = T.attachRemove;
  rm.onclick = (e) => { e.stopPropagation(); removeAttach(it.uid); };
  if (it.kind === "image") {
    const img = document.createElement("img");
    img.className = "attach-thumb"; img.src = it.url; img.alt = it.name;
    item.appendChild(img);
  } else {
    const wrap = document.createElement("div");
    wrap.className = "attach-file";
    const ico = document.createElement("div"); ico.className = "af-icon"; ico.innerHTML = iconHTML("i-file");
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
    const resp = await fetch("api/upload?dir=" + encodeURIComponent(webInputDir || "input"), {
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
    input.blur();
    return true;
  } finally {
    attachSending = false;
  }
}

/* ---------- input row / history ---------- */

const history = [];
let histPos = 0; // sentinel: histPos === history.length means "at the unsent draft"
let histDraft = "";
// FEATURE-496: the most recent instruction text submitted via the main input
// box, kept so it can be returned to the box if the task is rejected before
// it starts (e.g. the active model is unavailable).
let lastSubmittedText = "";

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
    input.blur();
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
    input.blur();
    return;
  }
  // FEATURE-496: remember the submitted instruction so it can be returned to
  // the input box if the task is rejected before it starts (e.g. the active
  // model is unavailable).
  lastSubmittedText = text;
  wsSend({ type: "input", text });
  renderUserEcho(text);
  history.push(text);
  histPos = history.length;
  input.value = "";
  autoGrow();
  if (wsReady) setRunning(true);
  input.blur();
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

// FIX-590: with field-sizing support the engine sizes the composer to its own
// content, so no JavaScript measurement is needed at all - the old per-keystroke
// read of scrollHeight was a forced synchronous layout on every keystroke (and
// therefore on every IME update). Engines without support keep the fallback
// below, coalesced to at most one measurement per animation frame.
const COMPOSER_FIELD_SIZING = typeof CSS !== "undefined" && typeof CSS.supports === "function" && CSS.supports("field-sizing", "content");
let composerGrowRaf = 0;

function autoGrow() {
  if (COMPOSER_FIELD_SIZING) return;
  if (composerGrowRaf) return;
  composerGrowRaf = requestAnimationFrame(() => {
    composerGrowRaf = 0;
    growComposer();
  });
}

// growComposer measures the content height directly (FEATURE-405: +6px
// increment, 150px cap). It is only used when field-sizing is unavailable.
function growComposer() {
  input.style.height = "auto";
  input.style.height = Math.min(input.scrollHeight + 6, 150) + "px";
}

/* ---------- merged send / interrupt button (FEATURE-369) ---------- */

let running = false;

// setRunning flips the single button between the send icon (idle) and the
// pause icon (agent turn in progress). Both are inline SVG (FEATURE-516).
// Turn boundaries arrive as the web
// session's turn_start / await_input events; sendInput flips to running
// immediately for responsive feedback.
function setRunning(v) {
  running = v;
  // FEATURE-571: a control that fired an event stays disabled until the turn
  // ends, so re-arm the blocked buttons/elements on the turn boundary (this is
  // the only place that knows when the agent handed control back).
  if (!v && window.UI && typeof window.UI.releasePendingActions === "function") {
    window.UI.releasePendingActions();
  }
  setIcon(sendBtn, v ? "i-pause" : "i-send");
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
// FIX-591: the follow-state probe reads scrollTop/clientHeight/scrollHeight on
// every scroll event; coalescing it keeps a scroll burst to one measurement per
// frame.
// FIX-595: also rate-limit it. With the .ev content-visibility isolation gone,
// the scrollHeight read is proportional to the session size again, so probing on
// every frame of a scroll would force repeated full-stream layouts. The "am I at
// the bottom" answer does not need frame accuracy.
let followStateRaf = 0;
let followStateLastRun = 0;
streamB.addEventListener("scroll", () => {
  if (followStateRaf) return;
  followStateRaf = requestAnimationFrame(() => {
    followStateRaf = 0;
    const now = performance.now();
    if (now - followStateLastRun < 100) return;
    followStateLastRun = now;
    updateFollowState();
  });
});

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
// workModeOrder remembers the mode list order from the last renderModeSeg call
// so the narrow single-capsule switcher can cycle to the next work mode.
let workModeOrder = [];

function renderModeSeg(modes) {
  if (!modes || modes.length === 0) return;
  const cur = modes.find((m) => m.current);
  if (cur) currentMode = cur.name;
  workModeOrder = modes.map((m) => m.name);
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
  // FIX-590: keep the renderer out of the way while the user types.
  noteInputActivity();
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
  // FEATURE-547: while an interaction is pending the virtual keyboard on window
  // owns the keys (option hotkeys). Never pull the focus into the input box in
  // that state - otherwise answering an option would leave the caret in the
  // input box and the hotkeys would no longer look usable.
  if (pendingInteraction && !supplementMode) return;
  input.focus();
});
input.addEventListener("input", () => { noteInputActivity(); autoGrow(); });
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
// FIX-589: relative path -> { ul, node } of every directory row currently in
// the DOM. loadTree() rebuilds the sidebar from the root level only and then
// refills the expanded directories through this map (parents first).
const treeDirRows = new Map();

// FEATURE-515: pending timer for the "hover a long name for 1s" sidebar
// auto-expand; cleared when the pointer leaves the row before it fires.
let treeAutoTimer = 0;

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
    const resp = await fetch("api/tree");
    const root = await resp.json();
    tree.textContent = "";
    // FIX-589: the sidebar is rebuilt from the root level only, so the map of
    // directory rows has to be rebuilt with it.
    treeDirRows.clear();
    const ul = document.createElement("ul");
    // First level shows the workspace root folder itself, so the most
    // salient feature (the workspace name) is visible at a glance.
    ul.appendChild(treeNode(root));
    tree.appendChild(ul);
    // FIX-589: reopen the directories the user had expanded. Their rows and
    // their level data were dropped on collapse / rebuild, so each of them is
    // requested from the server again (parents before children, so every row
    // exists by the time it is filled).
    await restoreExpandedDirs();
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

  // FIX-519: the twisty (▸/▾) belongs to directories only. The span is still
  // created for every row so its 12px gutter keeps file and directory names
  // left-aligned, but it is left EMPTY for files — the dir branch below sets
  // i-tri / i-tri-down based on the open state. Setting i-tri here made files
  // render a stray expand/collapse arrow.
  const tw = document.createElement("span");
  tw.className = "tw";
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
    badge.innerHTML = iconHTML("i-dot", "ico-inline") + node.changes;
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
      setIcon(dl, "i-download");
      dl.onclick = (e) => { e.stopPropagation(); downloadFile(node.path); };
      actions.appendChild(dl);
    }
  } else {
    const reveal = document.createElement("button");
    reveal.className = "reveal-btn";
    reveal.title = T.revealDir;
    reveal.innerHTML = iconHTML("i-reveal");
    reveal.onclick = (e) => { e.stopPropagation(); postPath("api/reveal", node.path); };
    actions.appendChild(reveal);
  }
  row.appendChild(actions);

  // FEATURE-383: hovering a long (truncated) file name auto-expands the
  // sidebar to fit it; leaving the sidebar returns it to the fixed width.
  // FEATURE-515: only a sustained hover (1s) counts, so merely passing the
  // pointer over the list no longer makes the layout jump.
  row.addEventListener("mouseenter", () => {
    if (name.scrollWidth <= name.clientWidth) return;
    clearTimeout(treeAutoTimer);
    treeAutoTimer = setTimeout(() => layout.classList.add("sidebar-auto"), 1000);
  });
  row.addEventListener("mouseleave", () => clearTimeout(treeAutoTimer));

  // FEATURE-447: every row (file and directory) carries its absolute path so
  // the affected-file highlight can match directories too (predicted files
  // may resolve to a shared folder).
  row.dataset.path = node.path;

  li.appendChild(row);

  if (node.dir) {
    const ul = document.createElement("ul");
    const open = expandedDirs.has(node.path);
    ul.style.display = open ? "" : "none";
    setIcon(tw, open ? "i-tri-down" : "i-tri");
    // FEATURE-477: the .open class lets the light theme draw an "open folder"
    // icon for expanded directories (vs a closed folder when collapsed).
    tw.classList.toggle("open", open);
    // FIX-518: a collapsed directory keeps an empty <ul>; its rows are built the
    // first time it is opened. Rendering every node up front cost ~500 ms and
    // 238k DOM elements for a 21k-entry workspace, all of it hidden.
    // FIX-589: the rows come from the level the server returns for this very
    // directory, and the row registers its <ul> so a rebuild can refill it.
    treeDirRows.set(node.path, { ul: ul, node: node });
    if (open) loadDirLevel(ul, node);
    li.appendChild(ul);
    row.onclick = () => {
      const isOpen = ul.style.display !== "none";
      if (isOpen) {
        // FIX-589: collapsing drops both the child rows and the level data, so
        // the next expand asks the server again instead of keeping stale rows.
        ul.style.display = "none";
        setIcon(tw, "i-tri");
        tw.classList.remove("open");
        expandedDirs.delete(node.path);
        clearDirLevel(ul, node);
      } else {
        ul.style.display = "";
        setIcon(tw, "i-tri-down");
        tw.classList.add("open");
        expandedDirs.add(node.path);
        loadDirLevel(ul, node);
      }
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

// fillDirChildren builds a directory's child rows on demand (FIX-518). Rows are
// created in a detached fragment and inserted with a single append, and a
// directory that was already filled is left untouched — so expanding a folder
// costs one batched insert instead of rebuilding the whole tree, and repeated
// expand/collapse never duplicates work. Directories inside it that the user
// had already opened are filled by the same rule (treeNode checks expandedDirs),
// which keeps the expansion state — and the affected-file highlight that relies
// on it — intact across a loadTree() refresh.
function fillDirChildren(ul, node) {
  if (ul.dataset.filled) return;
  ul.dataset.filled = "1";
  const frag = document.createDocumentFragment();
  for (const c of node.children || []) frag.appendChild(treeNode(c));
  ul.appendChild(frag);
}

// loadDirLevel makes sure a directory's <ul> shows that directory's level: the
// rows are built from the fetched level, and concurrent callers share the one
// in-flight request (FIX-589).
async function loadDirLevel(ul, node) {
  if (ul.dataset.filled) return;
  if (ul._levelPromise) return ul._levelPromise;
  if (Array.isArray(node.children)) {
    fillDirChildren(ul, node);
    return;
  }
  ul._levelPromise = fetchTreeLevel(ul, node);
  try {
    await ul._levelPromise;
  } finally {
    delete ul._levelPromise;
  }
}

// fetchTreeLevel requests one directory level (api/tree?path=...) and turns it
// into rows. On failure the folder is folded back so the next click retries
// instead of leaving an empty list behind (FIX-589).
async function fetchTreeLevel(ul, node) {
  ul.dataset.loading = "1";
  appendTreePlaceholder(ul, T.treeLoading);
  try {
    const resp = await fetch("api/tree?path=" + encodeURIComponent(node.path));
    if (!resp.ok) throw new Error("HTTP " + resp.status);
    const level = await resp.json();
    node.children = Array.isArray(level.children) ? level.children : [];
    clearTreePlaceholder(ul);
    // fillDirChildren marks the <ul> as filled itself; pre-setting the flag here
    // would make it return early and leave the (already fetched) level unrendered.
    fillDirChildren(ul, node);
    // Rows created just now may belong to the affected files of the current
    // call, so re-apply the highlight after a level arrives.
    applyAffectedHighlight();
  } catch (err) {
    collapseDirOnError(ul, node);
    console.warn("workspace tree: level request failed", node.path, err);
  } finally {
    clearTreePlaceholder(ul);
    delete ul.dataset.loading;
  }
}

// clearDirLevel drops a collapsed directory's rows and its level data
// (FIX-589): both the DOM and the data are released, and the next expand
// requests the level from the server again.
function clearDirLevel(ul, node) {
  ul.textContent = "";
  delete ul.dataset.filled;
  delete node.children;
}

// restoreExpandedDirs refills the directories the user had expanded after a
// DOM rebuild (FIX-589), parents before children so a child row exists before
// its own level is requested.
async function restoreExpandedDirs() {
  const paths = Array.from(expandedDirs).filter((p) => p !== "");
  paths.sort((a, b) => a.split("/").length - b.split("/").length);
  for (const p of paths) {
    const entry = treeDirRows.get(p);
    if (!entry || !entry.ul || entry.ul.dataset.filled) continue;
    const parent = p.includes("/") ? p.slice(0, p.lastIndexOf("/")) : "";
    if (parent && !treeDirRows.has(parent)) continue; // its parent level is not in the DOM
    await loadDirLevel(entry.ul, entry.node);
  }
}

// dirRowOf returns the row element owning a directory's <ul> (the row is the
// <ul>'s previous sibling).
function dirRowOf(ul) {
  const row = ul.previousElementSibling;
  return row && row.classList && row.classList.contains("tree-row") ? row : null;
}

// collapseDirOnError folds a directory back after a failed level request so the
// user can retry by clicking it again (FIX-589).
function collapseDirOnError(ul, node) {
  ul.style.display = "none";
  expandedDirs.delete(node.path);
  const row = dirRowOf(ul);
  const tw = row && row.querySelector(".tw");
  if (tw) {
    setIcon(tw, "i-tri");
    tw.classList.remove("open");
  }
}

// appendTreePlaceholder shows a single "loading" row inside a directory while
// its level is being fetched (FIX-589).
function appendTreePlaceholder(ul, text) {
  const li = document.createElement("li");
  li.className = "tree-node tree-loading";
  const row = document.createElement("div");
  row.className = "tree-row loading";
  const name = document.createElement("span");
  name.className = "name";
  name.textContent = text;
  row.appendChild(name);
  li.appendChild(row);
  ul.appendChild(li);
}

// clearTreePlaceholder removes the "loading" rows of a directory (FIX-589).
function clearTreePlaceholder(ul) {
  ul.querySelectorAll("li.tree-loading").forEach((li) => li.remove());
}

const IMAGE_EXT = /\.(png|jpe?g|gif|webp|bmp|svg)$/i;

// FEATURE-425: text file extensions that can be previewed in-page (source
// code, docs, config, data, shell scripts, etc.).
const TEXT_EXT = /\.(go|txt|md|markdown|csv|tsv|sh|bash|zsh|conf|cfg|ini|json|ya?ml|xml|py|js|mjs|cjs|ts|jsx|tsx|html?|css|scss|less|sql|java|c|h|cpp|hpp|rs|rb|php|vue|svelte|toml|env|gitignore|dockerfile|makefile|log|properties|gradle|lock|sum|mod)$/i;

// openFile opens a file with the OS default handler (FEATURE-444: images are
// opened by the system on double-click, not previewed in-page).
function openFile(node) {
  postPath("api/open", node.path);
  // FEATURE-471: report the opened file as a dynamic event so the LLM can
  // notice it during a running task.
  wsSend({ type: "dynamic_event", kind: "open_file", value: node.path });
}

// Current image viewer state: the workspace-relative path being shown, used to
// toggle off on a second click on the same file (FEATURE-547).
let ivPath = null;

// openImagePreview shows an image in the non-modal main-area viewer
// (FEATURE-547; it used to open the #preview modal, which dimmed the whole
// page and swallowed clicks on the workspace list). The viewer covers only the
// central message area, so the tree stays clickable and the user can switch
// files straight from the list, and it keeps a floating info bubble (name /
// size / mtime / resolution) at the bottom. Re-clicking the same image closes
// it, matching the text previewer; the resolution is read from the loaded
// image's natural dimensions.
function openImagePreview(node) {
  if (ivPath === node.path) { closeImageViewer(); return; }
  // FEATURE-471: report the single-click preview as a view_file dynamic event
  // so the LLM can notice which files the user is inspecting.
  wsSend({ type: "dynamic_event", kind: "view_file", value: node.path });
  // FEATURE-547: the text and image viewers never show at the same time.
  closeFileViewer();
  ivPath = node.path;
  ivName.textContent = node.name;
  ivSize.textContent = formatSize(node.size);
  ivMtime.textContent = formatMtime(node.mtime);
  ivRes.textContent = "";
  ivImg.onload = () => {
    ivRes.textContent = ivImg.naturalWidth + " × " + ivImg.naturalHeight;
  };
  ivImg.src = "api/file?path=" + encodeURIComponent(node.path);
  imgViewer.classList.remove("hidden");
  // FEATURE-425: highlight the currently selected file in the workspace tree.
  highlightTreeFile(node.path);
}

// closeImageViewer hides the non-modal image viewer and clears its state
// (FEATURE-547).
function closeImageViewer() {
  if (ivPath === null) return;
  imgViewer.classList.add("hidden");
  ivPath = null;
  ivTitlebar.classList.remove("active");
  ivImg.removeAttribute("src");
  tree.querySelectorAll(".tree-row.fv-selected").forEach((r) => r.classList.remove("fv-selected"));
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

/* ---------- path links in the conversation stream (FEATURE-576) ---------- */

// wsRoot is the workspace root reported by /api/bootstrap (recorded in boot()).
// It lets an absolute path printed by the model be recognised as a workspace
// file: the read-only preview API stays confined to the workspace (see
// resolvePath in web/server.go), so such a path is converted to its
// workspace-relative form before it reaches any preview API.
let wsRoot = "";

// isPreviewablePath reports whether a path's type is one the file previewer can
// show — the same two extension sets the workspace tree routes a click with.
function isPreviewablePath(p) {
  return IMAGE_EXT.test(p) || TEXT_EXT.test(p);
}

// previewPathOf turns one path candidate found in rendered text into
// {path, line} for the previewer, or null when it is not a previewable local
// file: wrapping punctuation is trimmed (the candidate comes out of a markdown
// text run), an optional ":line[:col]" suffix is split off, URLs are rejected,
// a bare filename (no "/") is rejected, extensions the previewer cannot show
// are rejected, and an absolute path is accepted only inside the workspace
// (converted to the relative form every preview API expects).
function previewPathOf(raw) {
  const rawStr = String(raw || "");
  // A URL is never a local path. Checked on the untrimmed candidate because the
  // candidate may have swallowed the label in front of the scheme.
  if (rawStr.indexOf("://") >= 0) return null;
  // Prose/CJK punctuation ("：", "、", "，", "\"…) is already kept out of the
  // path candidate by MD_PATH_RE in md.js, so the string arrives clean; only
  // wrapping markdown punctuation still needs trimming (a path inside `code`
  // or **bold** arrives unwrapped, but an unclosed ** while streaming does not).
  let s = rawStr.replace(/^[({\[*_'"`]+/, "").replace(/[)\]}`*_'".,;:!?]+$/, "");
  let line = 0;
  const m = s.match(/^(.*?):(\d+)(?::\d+)?$/);
  if (m) { s = m[1]; line = parseInt(m[2], 10); }
  if (!s || s.indexOf("/") < 0) return null;
  if (!isPreviewablePath(s)) return null;
  if (s.charAt(0) === "/") {
    if (!wsRoot || s.indexOf(wsRoot + "/") !== 0) return null;
    s = s.slice(wsRoot.length + 1);
  }
  if (s.indexOf("./") === 0) s = s.slice(2);
  return s ? { path: s, line: line } : null;
}

// openPathPreview opens a path found in the conversation stream in the same
// viewer the workspace tree uses (FEATURE-576); line > 0 scrolls to that line.
function openPathPreview(path, line) {
  if (!path) return;
  const name = path.split("/").pop() || path;
  const node = { path: path, name: name, size: 0, mtime: 0 };
  if (IMAGE_EXT.test(name)) {
    openImagePreview(node);
    return;
  }
  openFilePreview(node);
  if (line > 0) jumpToPreviewLine(path, line);
}

// jumpToPreviewLine loads on-demand chunks until the target line is rendered,
// then scrolls to it (the strategy loadFirstChange uses, FEATURE-494).
async function jumpToPreviewLine(path, line) {
  while (fvPath === path && fvNextLine < line && fvNextLine <= fvTotal) {
    await loadFileChunk(path, fvNextLine, fvNextLine + 200);
  }
  if (fvPath === path) scrollToLine(line);
}

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
  // FEATURE-547: the text and image viewers never show at the same time.
  closeImageViewer();
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
  // FEATURE-494: after opening, scroll to the first change of the file's most
  // recent commit (top 1/3 of the viewport).
  loadFirstChange(node.path);
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
    const resp = await fetch("api/gitdiff?path=" + encodeURIComponent(path));
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

// loadFirstChange fetches the first changed line of the file's most recent
// commit (FEATURE-494) and scrolls the viewer so that line sits at the top
// 1/3 of the viewport. line=0 (no change) leaves the scroll position alone.
// The target line may be beyond the first 200-line chunk, so the chunk is
// loaded first and the scroll happens once the line is rendered.
async function loadFirstChange(path) {
  let line = 0;
  try {
    const resp = await fetch("api/gitfirstchange?path=" + encodeURIComponent(path));
    const body = await resp.json();
    line = body.line || 0;
  } catch { /* no git info */ }
  if (line <= 0 || fvPath !== path) return;
  // Ensure the target line is loaded (on-demand chunks of 200 lines).
  while (fvNextLine < line && fvNextLine <= fvTotal) {
    await loadFileChunk(path, fvNextLine, fvNextLine + 200);
  }
  if (fvPath !== path) return;
  scrollToLine(line);
}

// scrollToLine scrolls #fvBody so the given source line sits at the top 1/3
// of the visible area. In raw mode the line is a .fv-line[data-no]; in md
// auto-render mode it is the .fv-md-row whose gutter shows the block's
// starting source line (the closest block at or before the target).
function scrollToLine(line) {
  // Raw mode: exact .fv-line[data-no]. md auto-render: the .fv-md-row whose
  // gutter (textContent) is the closest block start at or before the target.
  let row = fvBody.querySelector('.fv-line[data-no="' + line + '"]');
  if (!row) {
    let best = null;
    for (const r of fvBody.querySelectorAll(".fv-md-row")) {
      const noEl = r.querySelector(".fv-md-no");
      const no = noEl ? parseInt(noEl.textContent, 10) : 0;
      if (no > 0 && no <= line) best = r;
    }
    row = best;
  }
  if (!row) return;
  const top = row.offsetTop - fvBody.clientHeight / 3;
  fvBody.scrollTop = Math.max(0, top);
}

// loadFileChunk fetches a line range [start, end] and appends it to the body.
// It is called on open and on scroll near the bottom (on-demand loading for
// large files, UC-006). For md files in auto-render mode (Raw off) the whole
// file is rendered as markdown instead of per-line rows.
async function loadFileChunk(path, start, end) {
  if (fvLoading || fvPath !== path) return;
  fvLoading = true;
  try {
    const resp = await fetch("api/file?path=" + encodeURIComponent(path) + "&start=" + start + "&end=" + end);
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
    const resp = await fetch("api/file?path=" + encodeURIComponent(path) + "&hex=1&start=" + start + "&end=" + end + "&width=" + fvHexWidth);
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
  window.location.href = "api/download?path=" + encodeURIComponent(path);
}

previewClose.onclick = () => preview.classList.add("hidden");
preview.onclick = (e) => { if (e.target === preview) preview.classList.add("hidden"); };

// FEATURE-547: image viewer bubble — only the ✕ closes the preview (the bubble
// body is inert, like the text viewer's); hovering near the bottom of the
// viewer fills the bubble the same way the text viewer does.
ivClose.onclick = () => closeImageViewer();
imgViewer.addEventListener("mousemove", (e) => {
  if (ivPath === null) return;
  const rect = imgViewer.getBoundingClientRect();
  ivTitlebar.classList.toggle("active", rect.bottom - e.clientY <= 100);
});
imgViewer.addEventListener("mouseleave", () => ivTitlebar.classList.remove("active"));

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
  // FIX-518: the workspace root ("") must be expanded too. Rows are now built on
  // demand, so a collapsed root has no children in the DOM at all and the target
  // row could not be created (highlightAffectedFiles already does this).
  expandedDirs.add("");
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
    const resp = await fetch("api/upload?dir=" + encodeURIComponent(dir || ""), { method: "POST", body: fd });
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
  // FEATURE-487: always open at the level-1 category list on narrow screens
  // (clear any leftover drilled-in state from a previous close).
  settingsShowList();
  wsSend({ type: "settings_get" });
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
  if (t.includes("模型") || t.includes("model")) return "i-model-text";
  if (t.includes("显示") || t.includes("display") || t.includes("输出")) return "i-display";
  if (t.includes("安全") || t.includes("safety") || t.includes("确认")) return "i-shield";
  if (t.includes("记忆") || t.includes("memory") || t.includes("上下文")) return "i-memory";
  if (t.includes("工作区") || t.includes("workspace")) return "i-folder";
  if (t.includes("mcp")) return "i-mcp";
  if (t.includes("技能") || t.includes("skill")) return "i-skills";
  if (t.includes("开发") || t.includes("debug") || t.includes("搜索")) return "i-devtools";
  return "i-settings";
}

// renderSettings renders the grouped setting items returned by settings_get
// (FEATURE-391) in an iPad-style layout: left nav + right content pane.
// The active group is preserved across refreshes (e.g. after a settings_set
// re-fetch) so the user stays on the current category instead of jumping back
// to the first one (FEATURE-470).
function renderSettings(groups) {
  const prevGroup = settingsGroups[settingsActiveGroup];
  settingsGroups = (groups || []).slice();
  // FEATURE-583: the frontend-only "Skills" category (its list and switches come
  // from /api/skills, not from the backend settings groups) sits right above the
  // MCP Server category.
  const mcpIdx = settingsGroups.findIndex((g) => g && g.kind === "mcp");
  const skillsAt = mcpIdx >= 0 ? mcpIdx : settingsGroups.length;
  settingsGroups.splice(skillsAt, 0, { title: T.skillsSettings, kind: "skills" });
  // FEATURE-470: keep the category the user is on selected across refreshes —
  // the inserted group shifts every later category by one.
  if (prevGroup && prevGroup.kind === "skills") settingsActiveGroup = skillsAt;
  else if (settingsActiveGroup >= skillsAt) settingsActiveGroup += 1;
  if (settingsActiveGroup >= settingsGroups.length) settingsActiveGroup = 0;
  cacheWebInputDir(groups);
  // FEATURE-508: pick up the sliding-window thresholds so a settings change
  // takes effect without a page reload.
  cacheStreamWindowSettings(groups);
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
    icon.innerHTML = iconHTML(settingsGroupIcon(g.title));
    const label = document.createElement("span");
    label.className = "settings-nav-label";
    label.textContent = g.title || "";
    item.appendChild(icon);
    item.appendChild(label);
    // FEATURE-487: a chevron on the right of each category row (always shown;
    // on narrow screens it signals the drill-in to that category's page).
    const chev = document.createElement("span");
    chev.className = "settings-nav-chev";
    chev.textContent = ">";
    item.appendChild(chev);
    item.onclick = () => selectSettingsGroup(i);
    frag.appendChild(item);
  });
  settingsNav.appendChild(frag);
}

// selectSettingsGroup switches the active category and re-renders the pane.
// On narrow screens it also drills into the category page (hides the category
// list, shows the pane with a back button).
function selectSettingsGroup(index) {
  settingsActiveGroup = index;
  settingsNav.querySelectorAll(".settings-nav-item").forEach((el) => {
    el.classList.toggle("active", Number(el.getAttribute("data-index")) === index);
  });
  renderSettingsPane();
  if (document.body.classList.contains("narrow")) settingsShowPane();
}

// settingsShowPane / settingsShowList toggle the two-level narrow-screen
// settings navigation: the category list (level 1) vs. the category page
// (level 2). On wide screens both are always visible side by side.
function settingsShowPane() {
  const body = document.getElementById("settingsBody");
  if (body) body.classList.add("settings-drilled");
}
function settingsShowList() {
  const body = document.getElementById("settingsBody");
  if (body) body.classList.remove("settings-drilled");
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
  // FEATURE-553: the "workspace" group renders the read-only workspace path
  // plus the editable folder-exclusion list of the workspace tree.
  if (g.kind === "workspace") {
    renderWorkspaceSettings(g);
    return;
  }
  // FEATURE-583: the "skills" group renders the skill list + enable switches.
  if (g.kind === "skills") {
    renderSkillsSettings();
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

/* ---------- FEATURE-553: workspace tree exclusions ---------- */

// wsTreeRefreshPending is set while a tree-excludes save is in flight, so
// showSettingsResult refreshes the workspace file tree once the save succeeds.
let wsTreeRefreshPending = false;

// renderWorkspaceSettings renders the "workspace" settings group: the current
// workspace path as a plain sentence (the backend composes it into the item
// description) followed by the list of folders hidden from the workspace tree.
// Adding/removing an entry saves immediately (settings_set) and the tree is
// refreshed when the backend confirms.
function renderWorkspaceSettings(g) {
  const items = g.items || [];
  const pathItem = items.find((it) => it.key === "workspace-path");
  const listItem = items.find((it) => it.key === "tree-excludes");
  let list = [];
  try {
    const parsed = JSON.parse((listItem && listItem.value) || "[]");
    if (Array.isArray(parsed)) list = parsed;
  } catch (_) {
    list = [];
  }

  const frag = document.createDocumentFragment();

  // Current workspace path: a plain sentence, not a read-only input.
  const pathLine = document.createElement("div");
  pathLine.className = "set-desc ws-path-line";
  pathLine.textContent = (pathItem && pathItem.desc) || "";
  frag.appendChild(pathLine);

  // Exclusion list: one row per entry with its remove button.
  const listLabel = document.createElement("div");
  listLabel.className = "set-desc ws-excludes-label";
  listLabel.textContent = (listItem && listItem.desc) || "";
  frag.appendChild(listLabel);
  const listEl = document.createElement("div");
  listEl.className = "ws-excludes-list";
  for (const entry of list) listEl.appendChild(wsExcludeRow(entry, list));
  frag.appendChild(listEl);

  // Add form: Enter or the button appends the entry.
  const addRow = document.createElement("div");
  addRow.className = "ws-add";
  const input = document.createElement("input");
  input.type = "text";
  input.className = "set-input ws-add-input";
  input.placeholder = i18nT("wsExcludesPlaceholder", "文件夹名称或相对路径，如 db 或 docs/private");
  const addBtn = document.createElement("button");
  addBtn.type = "button";
  addBtn.className = "btn-mini ws-add-btn";
  addBtn.textContent = i18nT("wsExcludesAdd", "添加");
  const commit = () => {
    const value = (input.value || "").trim().replace(/^\/+|\/+$/g, "");
    // Empty or duplicate input is ignored here; the backend reports rejects
    // (invalid paths) with a localized message in the settings result area.
    if (!value || list.includes(value)) return;
    wsSaveTreeExcludes(list.concat([value]));
  };
  addBtn.onclick = commit;
  input.addEventListener("keydown", (e) => {
    if (e.key === "Enter") { e.preventDefault(); commit(); }
  });
  addRow.appendChild(input);
  addRow.appendChild(addBtn);
  frag.appendChild(addRow);

  settingsDynamic.appendChild(frag);
}

// wsExcludeRow builds one exclusion entry row with its remove button.
function wsExcludeRow(entry, list) {
  const row = document.createElement("div");
  row.className = "ws-exclude-item";
  const name = document.createElement("span");
  name.className = "ws-exclude-name";
  name.textContent = entry;
  const rm = document.createElement("button");
  rm.type = "button";
  rm.className = "btn-mini ws-exclude-rm";
  rm.innerHTML = iconHTML("i-close");
  rm.title = i18nT("wsExcludesRemove", "移除");
  rm.onclick = () => wsSaveTreeExcludes(list.filter((e) => e !== entry));
  row.appendChild(name);
  row.appendChild(rm);
  return row;
}

// wsSaveTreeExcludes persists the whole list; the workspace tree is refreshed
// once the backend confirms (see showSettingsResult).
function wsSaveTreeExcludes(list) {
  wsTreeRefreshPending = true;
  wsSend({ type: "settings_set", key: "tree-excludes", value: JSON.stringify(list) });
}

/* ---------- FEATURE-583: Skills settings page ---------- */

// skillsData caches the last GET /api/skills payload so a per-skill toggle can
// resend the current default alongside its own change.
let skillsData = null;

// renderSkillsSettings renders the "Skills" page: the discovered skill list
// (one sliding enable switch per skill) plus the "enabled by default" switch.
function renderSkillsSettings() {
  settingsDynamic.innerHTML = "";
  const hint = document.createElement("div");
  hint.className = "skills-hint";
  hint.textContent = T.skillsSettingsHint;
  settingsDynamic.appendChild(hint);
  const list = document.createElement("div");
  list.className = "skills-list";
  list.textContent = T.skillsLoading;
  settingsDynamic.appendChild(list);
  fetch("api/skills").then((r) => r.json()).then((j) => {
    if (!j || !Array.isArray(j.skills)) throw new Error("bad payload");
    skillsData = j;
    list.textContent = "";
    renderSkillsList(list, j);
  }).catch(() => {
    list.textContent = T.skillsLoadFailed;
  });
}

// renderSkillsList fills the container with one row per skill plus the default
// switch block at the bottom.
function renderSkillsList(box, data) {
  const skills = data.skills || [];
  if (!skills.length) {
    const empty = document.createElement("div");
    empty.className = "skills-empty";
    empty.textContent = T.skillsEmpty;
    box.appendChild(empty);
  }
  for (const s of skills) box.appendChild(skillSettingRow(s));
  box.appendChild(skillDefaultRow(!!data.default_enabled));
}

// skillSettingRow builds one skill row: name + source/loaded/follow-default
// badges and the description on the left, the sliding enable switch on the
// right.
function skillSettingRow(s) {
  const row = document.createElement("div");
  row.className = "skill-row" + (s.enabled ? "" : " off");
  const info = document.createElement("div");
  info.className = "skill-info";
  const head = document.createElement("div");
  head.className = "skill-row-head";
  const name = document.createElement("span");
  name.className = "skill-row-name";
  name.textContent = s.name || "";
  head.appendChild(name);
  const src = document.createElement("span");
  src.className = "skill-badge skill-badge-src";
  src.textContent = s.source === "global" ? T.skillSourceGlobal : T.skillSourceWorkspace;
  head.appendChild(src);
  if (s.loaded) {
    const ld = document.createElement("span");
    ld.className = "skill-badge skill-badge-loaded";
    ld.textContent = T.skillLoadedTag;
    head.appendChild(ld);
  }
  if (!s.explicit) {
    // No explicit choice: the effective state comes from the default switch.
    const fd = document.createElement("span");
    fd.className = "skill-badge skill-badge-follow";
    fd.textContent = T.skillFollowDefault;
    head.appendChild(fd);
  }
  info.appendChild(head);
  if (s.description) {
    const desc = document.createElement("div");
    desc.className = "skill-row-desc";
    desc.textContent = s.description;
    info.appendChild(desc);
  }
  const meta = document.createElement("div");
  meta.className = "skill-row-meta";
  meta.textContent = fmtBytes(s.bytes || 0);
  info.appendChild(meta);
  row.appendChild(info);
  row.appendChild(makeSkillToggle(!!s.enabled, (v) => saveSkillToggle(s.name, v)));
  return row;
}

// skillDefaultRow builds the "enabled by default" switch block.
function skillDefaultRow(enabled) {
  const block = document.createElement("div");
  block.className = "skill-default";
  const head = document.createElement("div");
  head.className = "skill-default-head";
  const label = document.createElement("span");
  label.className = "skill-default-label";
  label.textContent = T.skillsDefaultEnabled;
  head.appendChild(label);
  head.appendChild(makeSkillToggle(enabled, (v) => postSkills({ default_enabled: v })));
  block.appendChild(head);
  const hint = document.createElement("div");
  hint.className = "skill-default-hint";
  hint.textContent = T.skillsDefaultHint;
  block.appendChild(hint);
  return block;
}

// makeSkillToggle builds a sliding on/off switch, reusing the MCP toggle style.
function makeSkillToggle(checked, onchange) {
  const label = document.createElement("label");
  label.className = "mcp-toggle";
  const input = document.createElement("input");
  input.type = "checkbox";
  input.checked = !!checked;
  input.onchange = () => onchange(input.checked);
  const slider = document.createElement("span");
  slider.className = "mcp-toggle-slider";
  label.appendChild(input);
  label.appendChild(slider);
  return label;
}

// saveSkillToggle posts one skill's new enable state; the current default is
// sent along so the backend never has to guess it.
function saveSkillToggle(name, enabled) {
  const def = skillsData ? !!skillsData.default_enabled : true;
  const skills = {};
  skills[name] = enabled;
  postSkills({ default_enabled: def, skills });
}

// postSkills applies a change through POST /api/skills, then re-renders the
// page and the loaded-skills panel from the response.
function postSkills(body) {
  fetch("api/skills", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  }).then((r) => r.json()).then((j) => {
    if (!j || !Array.isArray(j.skills)) {
      showSettingsResult({ ok: false, message: (j && j.error) || T.skillsSaveFailed });
      return;
    }
    skillsData = j;
    const g = settingsGroups[settingsActiveGroup];
    if (g && g.kind === "skills") renderSkillsSettings();
    // Disabling a skill unloads it: refresh the R2 panel too.
    refreshSkills();
  }).catch(() => showSettingsResult({ ok: false, message: T.skillsSaveFailed }));
}

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
    const opts = ["auto", "dark", "dark-muted", "light", "light-tp", "paper"];
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
    : (theme === "dark-muted" ? i18nT("logoThemeDarkMuted", "暗色低对比")
      : (theme === "light-tp" ? i18nT("logoThemeLightTp", "浅色主题(蓝灰)")
        : (theme === "paper" ? i18nT("logoThemePaper", "纸张护眼") : i18nT("logoThemeLight", "浅色主题"))));

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
    const url = "logos/" + theme + "?t=" + Date.now();
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
    fetch("api/logo", { method: "POST", body: fd })
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
    fetch("api/logo?theme=" + theme, { method: "DELETE" })
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
  // FEATURE-553: a successful workspace-tree save refreshes the file tree so
  // hidden folders disappear (or reappear) without a page reload.
  if (wsTreeRefreshPending) {
    wsTreeRefreshPending = false;
    if (msg.ok) loadTree();
  }
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
// mcpExpanded holds the name of the server whose tool list is expanded
// (null = none). Kept across re-renders so the expanded card stays open after
// a live mcp_test refresh (FEATURE-498 ext).
let mcpExpanded = null;

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

  const urlLbl = document.createElement("span");
  urlLbl.className = "mcp-form-label";
  urlLbl.textContent = i18nT("mcpUrl", "URL (SSE)");
  const urlInput = document.createElement("input");
  urlInput.className = "set-input mcp-url";
  urlInput.placeholder = i18nT("mcpUrlPh", "远程 SSE 地址，如 http://host:port/sse（留空则用本地命令）");
  urlInput.value = editing ? (editing.url || "") : "";

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
    const url = urlInput.value.trim();
    if (!name || (!command && !url)) {
      showMCPResult({ ok: false, message: i18nT("mcpNeedNameCmd", "名称和命令/URL 不能同时为空") });
      return;
    }
    if (editing) {
      // FIX-465: enabled is toggled on the card, not in the edit form.
      const cur = mcpServers.find((s) => s.name === name);
      wsSend({ type: "mcp_update", name, command, args, enabled: cur ? cur.enabled : true, url });
    } else {
      wsSend({ type: "mcp_add", name, command, args, url });
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
  form.appendChild(urlLbl); form.appendChild(urlInput);
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
  // Restore the expanded state after a live mcp_test refresh re-renders.
  const wasExpanded = mcpExpanded === s.name;
  if (wasExpanded) row.classList.add("expanded");

  // First line: expand toggle + title (clickable) + toggle + delete.
  const head = document.createElement("div");
  head.className = "mcp-row-head";

  // Expand/collapse button revealing the server's tool call names. Clicking
  // it runs a live connectivity test (mcp_test) and refreshes the tool list.
  const expandBtn = document.createElement("button");
  expandBtn.className = "mcp-expand";
  setIcon(expandBtn, wasExpanded ? "i-tri-down" : "i-tri");
  expandBtn.title = i18nT("mcpExpand", "展开工具列表");
  expandBtn.onclick = () => {
    const expanded = row.classList.toggle("expanded");
    setIcon(expandBtn, expanded ? "i-tri-down" : "i-tri");
    if (expanded) {
      mcpExpanded = s.name;
      wsSend({ type: "mcp_test", name: s.name });
    } else {
      mcpExpanded = null;
    }
  };
  head.appendChild(expandBtn);

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
    wsSend({ type: "mcp_update", name: s.name, command: s.command, args: s.args || [], enabled: toggleInput.checked, url: s.url || "" });
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

  // Second line: command/URL text (clickable to edit), single full row.
  const detail = document.createElement("div");
  detail.className = "mcp-row-detail";
  detail.textContent = s.url ? s.url : (s.command + (s.args && s.args.length ? " " + s.args.join(" ") : ""));
  detail.title = i18nT("mcpEditHint", "点击编辑");
  detail.onclick = () => { mcpEditing = s.name; renderMCPServerManager(); };
  row.appendChild(detail);

  // Expandable tool list (hidden by default, revealed by the expand button).
  const tools = document.createElement("div");
  tools.className = "mcp-row-tools";
  const toolNames = s.tools || [];
  if (toolNames.length) {
    for (const t of toolNames) {
      const chip = document.createElement("span");
      chip.className = "mcp-tool-chip";
      chip.textContent = t;
      tools.appendChild(chip);
    }
  } else {
    tools.textContent = i18nT("mcpNoTools", "（未连接或无工具）");
  }
  row.appendChild(tools);

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

// FEATURE-602: the automatic-selection switch above the model list. When on, a
// failed LLM call degrades to the next lower-priority model before the turn
// aborts. Its state arrives with every "models" message (auto_select).
const modelAutoSelectChk = document.getElementById("modelAutoSelect");
if (modelAutoSelectChk) {
  modelAutoSelectChk.onchange = () =>
    wsSend({ type: "model_auto_select", auto_select: modelAutoSelectChk.checked });
}

// setModelAutoSelect mirrors the backend switch state onto the checkbox.
function setModelAutoSelect(on) {
  if (modelAutoSelectChk) modelAutoSelectChk.checked = !!on;
}



// renderModels renders the model list into the manager modal and the status-bar
// model selector menu.
function renderModels(models, templates) {
  modelList = models || [];
  templateList = templates || [];
  renderModelsBody();
  renderModelMenu();
  renderModelVisionMenu();
  // FEATURE-546: the model list carries the MTP probe result of every model,
  // collected while the list was built. Keep the active model's status so the
  // per-turn popover can show it without probing again.
  activeMTP = pickActiveMTP();
}

// pickActiveMTP returns the MTP status of the model the status bar currently
// shows, falling back to the first model that has one. Returns null when no
// model carries the field (an older backend), which the popover renders as
// "not configured".
function pickActiveMTP() {
  const name = modelInfo && modelInfo.textModel;
  for (const m of modelList) {
    if (name && (m.model === name || m.name === name)) return m.mtp || null;
  }
  for (const m of modelList) {
    if (m.mtp) return m.mtp;
  }
  return null;
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
    row.className = "model-row" + (m.enabled ? " enabled" : "") + (m.id === selectedModelID ? " selected" : "") + (m.available === false ? " unavailable" : "");
    row.dataset.modelId = m.id;
    // FEATURE-449: clicking a row selects it (highlighted border); the toolbar
    // pin/delete buttons then act on the selected model.
    row.onclick = () => selectModel(m.id);
    // Left: provider logo (FEATURE-429).
    const logo = document.createElement("img");
    logo.className = "model-logo";
    logo.src = "static/logos/" + (MODEL_LOGOS[m.provider] || "generic.png");
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
    // FEATURE-526: the provider/model text is its own element so the context
    // length appended at the end of the line can stick to the right edge.
    const metaMain = document.createElement("span");
    metaMain.className = "model-meta-main";
    metaMain.textContent = m.provider + " · " + m.model;
    meta.appendChild(metaMain);
    // FEATURE-516: capability marks are vector icons, not emoji.
    const caps = [];
    if (m.vision) caps.push(mkIcon("i-model-vision", "ico-inline"));
    if (m.tool_call) caps.push(mkIcon("i-tool", "ico-inline"));
    // FEATURE-526: the bulb shows the thinking depth (empty / L / M / H / X)
    // instead of the bare "supports thinking" capability mark.
    const think = thinkIconEl(m);
    if (think) caps.push(think);
    if (caps.length) {
      meta.appendChild(document.createTextNode(" · "));
      caps.forEach((el, i) => {
        if (i) meta.appendChild(document.createTextNode(" "));
        meta.appendChild(el);
      });
    }
    meta.appendChild(document.createTextNode(" · P" + m.priority));
    // FEATURE-526: max context length (K units) at the right end of the line;
    // "?K" when it is not configured.
    const ctx = document.createElement("span");
    ctx.className = "model-ctx";
    ctx.textContent = fmtLenShort(m.max_model_len) || "?K";
    meta.appendChild(ctx);
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

// FEATURE-526: the thinking-depth states shared by the status-bar model menu
// and the model manager cards. Returns "" when no bulb should be shown (the
// model does not support thinking, or it was explicitly disabled), "none" for
// the plain bulb (depth not set) and the depth name otherwise. "max" is the
// alias some providers use for the top depth.
function thinkDepthOf(m) {
  if (!m || !m.thinking || m.thinking_enabled === false) return "";
  switch (String(m.reasoning_effort || "").trim().toLowerCase()) {
    case "low": return "low";
    case "medium": return "medium";
    case "high": return "high";
    case "xhigh":
    case "max": return "xhigh";
    default: return "none";
  }
}

// thinkIconEl builds the bulb icon for a model, or null when the model shows no
// bulb. Both render sites (status-bar menu and manager cards) call it, so the
// two views can never disagree (FEATURE-526).
function thinkIconEl(m) {
  const depth = thinkDepthOf(m);
  if (!depth) return null;
  return mkIcon(depth === "none" ? "i-think" : "i-think-" + depth, "ico-think");
}

// buildModelMenuItem builds one model row in a selector menu: a provider logo
// on the left, the model ID, and the max context length right-aligned. The row
// never wraps; the menu width adapts to its widest row. When activeID matches
// the model ID, the row is highlighted as the currently active model
// (FEATURE-422).
function buildModelMenuItem(m, onClick, activeID) {
  const item = document.createElement("div");
  item.className = "model-menu-item" + (activeID && m.id === activeID ? " active" : "") + (m.available === false ? " unavailable" : "");
  item.title = m.provider + " · " + m.model;
  const logo = document.createElement("img");
  logo.className = "model-logo";
  logo.src = "static/logos/" + (MODEL_LOGOS[m.provider] || "generic.png");
  logo.alt = "";
  logo.onerror = () => { logo.style.display = "none"; };
  item.appendChild(logo);
  const id = document.createElement("span");
  id.className = "model-id";
  id.textContent = m.id;
  item.appendChild(id);
  // FEATURE-526: the thinking-depth bulb sits between the model ID and the
  // right-aligned context length; no bulb when thinking is off.
  const think = thinkIconEl(m);
  if (think) item.appendChild(think);
  const ctx = document.createElement("span");
  ctx.className = "model-ctx";
  ctx.textContent = fmtLenShort(m.max_model_len);
  item.appendChild(ctx);
  item.onclick = onClick;
  return item;
}

// topPriorityModel returns the highest-priority enabled model in modelList,
// restricted to vision-capable models when vision is true. Returns null when no
// model qualifies (FIX-539).
function topPriorityModel(vision) {
  let best = null;
  for (const m of modelList) {
    if (!m.enabled) continue;
    if (vision && !m.vision) continue;
    if (!best || m.priority > best.priority) best = m;
  }
  return best;
}

// buildModelMenuDefault builds the "默认" row that clears the current mode's
// model binding for the given target ("text" or "vision"). It always shows the
// model this row falls back to, i.e. the globally highest-priority one: the
// highest-priority enabled model for "text", or the highest-priority enabled
// vision model for "vision" (FIX-539). A "(默认)" hint and the model's context
// length follow. When active is true (the mode has no binding for this slot),
// the row is highlighted (FEATURE-422).
function buildModelMenuDefault(target, menu, active) {
  const item = document.createElement("div");
  item.className = "model-menu-item default" + (active ? " active" : "");
  item.title = T.modelDefaultHint || "使用全局默认模型";
  // The current mode's binding is deliberately ignored: selecting this row
  // unbinds the slot, so showing the bound model here was misleading (FIX-539).
  const def = topPriorityModel(target === "vision");
  if (def) {
    const logo = document.createElement("img");
    logo.className = "model-logo";
    logo.src = "static/logos/" + (MODEL_LOGOS[def.provider] || "generic.png");
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
  // FEATURE-516: the "＋" glyph is drawn as a vector icon; strip it from the label.
  add.innerHTML = iconHTML("i-plus", "ico-inline") + (T.modelAdd || "＋ 新增模型").replace(/^＋\s*/, "");
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
  add.innerHTML = iconHTML("i-plus", "ico-inline") + (T.modelAdd || "＋ 新增模型").replace(/^＋\s*/, "");
  add.onclick = () => { modelVisionMenu.classList.add("hidden"); openModelWizard("add", ""); };
  modelVisionMenu.appendChild(add);
}

// FEATURE-487: the status-bar model selectors are now opened by clicking the
// model items (bound in the narrow-screen module at the bottom of this file)
// instead of hovering.

// refreshModelInfo re-fetches /api/bootstrap to update the status-bar model
// info (modelInfo) after a model switch/unbind, then refreshes the status bar
// (FEATURE-422).
async function refreshModelInfo() {
  try {
    const resp = await fetch("api/bootstrap");
    const b = await resp.json();
    if (b.textModel) {
      modelInfo = { textModel: b.textModel, textModelID: b.textModelID || "", textMaxLen: b.textMaxLen || 0, visionModel: b.visionModel || "", visionModelID: b.visionModelID || "", visionMaxLen: b.visionMaxLen || 0, modeTextModelID: b.modeTextModelID || "", modeVisionModelID: b.modeVisionModelID || "" };
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
    // FEATURE-601 (follow-up): every step is reachable from the left nav, forward
    // steps included, so the user can switch freely. Clicking the current step is
    // a no-op. Jumping anywhere else first collects the values typed so far —
    // model_wizard_refresh re-renders the requested step, so clicking step k lands
    // on step k (model_wizard_prev used to return the step BEFORE the one sent).
    item.onclick = () => {
      if (i === curIdx) return;
      collectWizardFields();
      wizardStep = order[i];
      wsSend({ type: "model_wizard_refresh", step: order[i], wizard_data: wizardData });
    };
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
  // FEATURE-601: the capabilities step gets a manual "detect" button.
  if (step.step === "capabilities") setupWizardCapabilitiesExtras(step);
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
    toggle.innerHTML = iconHTML("i-tri", "ico-inline") + " " + (T.templateJson || "查看模板原始 JSON");
    const pre = document.createElement("pre");
    pre.className = "wizard-template-json-body";
    pre.textContent = step.template_json;
    pre.style.display = "none";
    toggle.onclick = () => {
      const open = pre.style.display !== "none";
      pre.style.display = open ? "none" : "";
      toggle.querySelector("use").setAttribute("href", open ? "#i-tri" : "#i-tri-down");
      toggle.lastChild.textContent = " " + (T.templateJson || "查看模板原始 JSON");
    };
    head.appendChild(toggle);
    box.appendChild(head);
    box.appendChild(pre);
    modelWizardBody.appendChild(box);
  }
}

// setupWizardCapabilitiesExtras renders the manual "detect" button on the
// capabilities step (FEATURE-601). Clicking it asks the backend to re-run
// detectModelCapabilities — the same detection used when adding a model — and
// re-renders this step with the freshly detected values.
function setupWizardCapabilitiesExtras(step) {
  const row = document.createElement("div");
  row.className = "wizard-endpoint-test";
  const btn = document.createElement("button");
  btn.type = "button";
  btn.className = "wizard-test-btn";
  btn.textContent = T.wizardDetect || "手动检测";
  const res = document.createElement("span");
  res.className = "wizard-test-result";
  btn.onclick = () => {
    // Persist the current form state first so the backend has the endpoint /
    // api key / model name to test against.
    collectWizardFields();
    btn.disabled = true;
    res.textContent = T.wizardDetecting || "检测中...";
    res.className = "wizard-test-result";
    // FEATURE-601 follow-up: mark every capability as detecting; the backend
    // then streams per-capability progress that updates each one.
    modelWizardBody.querySelectorAll(".wizard-cap-status").forEach((el) => {
      el.textContent = T.wizardDetecting || "检测中...";
      el.className = "wizard-cap-status running";
    });
    wsSend({ type: "model_wizard_detect", wizard_data: wizardData });
    // The re-render replaces this button; re-enable defensively.
    setTimeout(() => { btn.disabled = false; }, 1500);
  };
  row.appendChild(btn);
  row.appendChild(res);
  modelWizardBody.appendChild(row);
}

// setCapDetectStatus updates the inline status shown after a capability name in
// the capabilities step (FEATURE-601 follow-up): "detecting…" while the backend
// runs that capability's test, then the result.
function setCapDetectStatus(msg) {
  const el = modelWizardBody.querySelector('[data-cap-status="' + msg.cap + '"]');
  if (!el) return;
  if (msg.phase === "running") {
    el.textContent = T.wizardDetecting || "检测中...";
    el.className = "wizard-cap-status running";
    return;
  }
  el.innerHTML = msg.ok
    ? iconHTML("i-check", "ico-inline") + " " + (T.wizardCapSupported || "支持")
    : iconHTML("i-xmark", "ico-inline") + " " + (T.wizardCapUnsupported || "不支持");
  el.className = "wizard-cap-status " + (msg.ok ? "ok" : "err");
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
  // FEATURE-601 follow-up: the three capability fields carry an inline status
  // slot filled by the per-capability detection progress messages.
  if (f.key === "vision" || f.key === "tool_call" || f.key === "thinking") {
    const st = document.createElement("span");
    st.className = "wizard-cap-status";
    st.dataset.capStatus = f.key;
    label.appendChild(st);
  }
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
        const resp = await fetch("api/test-endpoint", {
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
          setResultIcon(res, "i-check", "连通 (" + (body.endpoint || ep) + ")");
          res.className = "wizard-test-result ok";
        } else {
          setResultIcon(res, "i-xmark", "失败: " + (body.message || "无法连接"));
          res.className = "wizard-test-result err";
        }
      } catch (e) {
        setResultIcon(res, "i-xmark", "失败: " + e.message);
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
        const resp = await fetch("api/test-api-key", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ endpoint: ep, api_key: key }),
        });
        const body = await resp.json();
        if (body.ok) {
          setResultIcon(res, "i-check", body.message || "API Key 有效");
          res.className = "wizard-test-result ok";
        } else {
          setResultIcon(res, "i-xmark", body.message || "API Key 无效");
          res.className = "wizard-test-result err";
        }
      } catch (e) {
        setResultIcon(res, "i-xmark", "失败: " + e.message);
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
        const resp = await fetch("api/get-model-max-len", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ endpoint: ep, api_key: wizardData.api_key || "", model_name: model }),
        });
        const body = await resp.json();
        if (body.ok && body.max_model_len > 0) {
          ctl.value = body.max_model_len;
          wizardData.max_model_len = body.max_model_len;
          setResultIcon(res, "i-check", "已填入 " + body.max_model_len);
          res.className = "wizard-test-result ok";
        } else {
          setResultIcon(res, "i-xmark", body.message || "未获取到该模型的最大上下文长度");
          res.className = "wizard-test-result err";
        }
      } catch (e) {
        setResultIcon(res, "i-xmark", "失败: " + e.message);
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

/* ---------- identity & personality (FEATURE-393) ---------- */

// FEATURE-401: the "+" button creates a new session.
newSessionBtn.onclick = () => {
  wsSend({ type: "session_new" });
};

// openIdentity shows the identity & personality form and asks the backend for
// the field values.
function openIdentity() {
  identityModal.classList.remove("hidden");
  identityShowList();
  wsSend({ type: "identity_get" });
}

// FEATURE-534 增量: clicking the logo opens the identity & personality form
// directly; the previous single-item logo menu was removed.
logoWrap.onclick = (e) => {
  e.stopPropagation();
  closeAllPopups();
  openIdentity();
};
identityClose.onclick = () => identityModal.classList.add("hidden");
identityModal.onclick = (e) => { if (e.target === identityModal) identityModal.classList.add("hidden"); };
identityBack.onclick = () => identityShowList();

// FEATURE-534: the identity & personality form is grouped into five categories
// rendered with the same left-nav / right-pane layout as the settings modal.
// The agent (name + description) and the user (name + description) live in
// separate categories; each category lists the backend field keys it owns, and
// a field whose key is not mapped to any category falls back to the first one.
const IDENTITY_CATS = [
  { labelKey: "identityCatAgent", label: "智能体", icon: "i-model-text",
    keys: ["name", "description"] },
  { labelKey: "identityCatUser", label: "用户", icon: "i-circle",
    keys: ["user-name", "user-description"] },
  { labelKey: "identityCatPrinciples", label: "核心原则", icon: "i-shield",
    keys: ["principles"] },
  { labelKey: "identityCatCapabilities", label: "能力", icon: "i-devtools",
    keys: ["capabilities"] },
  { labelKey: "identityCatRules", label: "规则", icon: "i-file",
    keys: ["rules"] },
];
let identityFields = [];
let identityActiveCat = 0;

// identityShowList / identityShowPane toggle the narrow-screen two-level
// navigation (category list vs. category page); on wide screens both panes are
// always visible side by side. The classes are shared with the settings modal.
function identityShowList() { identityBody.classList.remove("settings-drilled"); }
function identityShowPane() { identityBody.classList.add("settings-drilled"); }

// renderIdentity renders the identity & personality form: the left nav lists the
// categories, the right pane shows the active category's fields. name /
// user-name are single-line inputs, the rest are multi-line textareas. Every
// field keeps its own save button. Field labels come from the backend
// (localized), falling back to the key.
function renderIdentity(fields) {
  identityFields = fields || [];
  if (identityActiveCat >= IDENTITY_CATS.length) identityActiveCat = 0;
  identityFieldsEl.innerHTML = "";
  const frag = document.createDocumentFragment();
  for (const f of identityFields) {
    const row = renderIdentityField(f);
    row.setAttribute("data-cat", String(identityCatOf(f.key)));
    frag.appendChild(row);
  }
  identityFieldsEl.appendChild(frag);
  renderIdentityNav();
  applyIdentityCat();
}

// identityCatOf returns the category index owning a field key; unmapped keys
// fall back to the first category so nothing becomes unreachable.
function identityCatOf(key) {
  const i = IDENTITY_CATS.findIndex((c) => c.keys.includes(key));
  return i >= 0 ? i : 0;
}

// renderIdentityNav builds the left category navigation.
function renderIdentityNav() {
  identityNav.innerHTML = "";
  const frag = document.createDocumentFragment();
  IDENTITY_CATS.forEach((c, i) => {
    const item = document.createElement("div");
    item.className = "settings-nav-item" + (i === identityActiveCat ? " active" : "");
    item.setAttribute("data-index", String(i));
    const icon = document.createElement("span");
    icon.className = "settings-nav-icon";
    icon.innerHTML = iconHTML(c.icon);
    const label = document.createElement("span");
    label.className = "settings-nav-label";
    label.textContent = i18nT(c.labelKey, c.label);
    const chev = document.createElement("span");
    chev.className = "settings-nav-chev";
    chev.textContent = ">";
    item.appendChild(icon);
    item.appendChild(label);
    item.appendChild(chev);
    item.onclick = () => selectIdentityCat(i);
    frag.appendChild(item);
  });
  identityNav.appendChild(frag);
  // FEATURE-536: a caution note pinned to the bottom of the category nav — every
  // value here feeds the system prompt, so editing it affects request caching,
  // cost and the preprocessing time of the first request after the change.
  const hint = document.createElement("div");
  hint.className = "identity-nav-hint";
  hint.textContent = i18nT("identityNavHint", "");
  identityNav.appendChild(hint);
}

// selectIdentityCat switches the active category; on narrow screens it drills
// into that category's page.
function selectIdentityCat(index) {
  identityActiveCat = index;
  identityNav.querySelectorAll(".settings-nav-item").forEach((el) => {
    el.classList.toggle("active", Number(el.getAttribute("data-index")) === index);
  });
  applyIdentityCat();
  if (document.body.classList.contains("narrow")) identityShowPane();
}

// applyIdentityCat shows only the active category's rows. All rows stay in the
// DOM (hidden), so unsaved edits survive switching categories.
function applyIdentityCat() {
  const want = String(identityActiveCat);
  identityFieldsEl.querySelectorAll(".identity-row").forEach((row) => {
    row.style.display = row.getAttribute("data-cat") === want ? "" : "none";
  });
}

// renderIdentityField builds one field row: label, control and its own save
// button (save granularity unchanged from the previous flat form).
function renderIdentityField(f) {
  const row = document.createElement("div");
  // FEATURE-534 增量: single-line fields keep their natural height; rows holding
  // a textarea absorb the pane's leftover height so the field fills the space
  // while its own save button stays visible.
  row.className = f.type === "text" ? "identity-row" : "identity-row identity-row-grow";

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
    ctl.rows = 8;
    ctl.value = f.value || "";
  }
  row.appendChild(ctl);

  const saveBtn = document.createElement("button");
  saveBtn.className = "btn identity-save";
  saveBtn.textContent = i18nT("save", "保存");
  saveBtn.onclick = () => {
    wsSend({ type: "identity_set", key: f.key, value: ctl.value });
  };
  // FEATURE-536: the hint sits left of the save button in the same footer row,
  // reserving two lines of height that match the button height.
  const foot = document.createElement("div");
  foot.className = "identity-field-foot";
  const hint = document.createElement("div");
  hint.className = "identity-hint";
  hint.textContent = identityHintOf(f.key);
  foot.appendChild(hint);
  foot.appendChild(saveBtn);
  row.appendChild(foot);
  return row;
}

// IDENTITY_HINT_KEYS maps a field key to the i18n key of its explanatory hint
// (FEATURE-536); a key without an entry renders no hint text.
const IDENTITY_HINT_KEYS = {
  name: "identityHintName",
  description: "identityHintDescription",
  "user-name": "identityHintUserName",
  "user-description": "identityHintUserDescription",
  principles: "identityHintPrinciples",
  capabilities: "identityHintCapabilities",
  rules: "identityHintRules",
};

// identityHintOf returns the localized hint for a field key ("" when unmapped).
function identityHintOf(key) {
  const k = IDENTITY_HINT_KEYS[key];
  return k ? i18nT(k, "") : "";
}

// showIdentityResult displays the result of an identity_set change.
// FIX-544: a successful save used to render msg.message, which the backend
// filled with the raw field key - the user saw "name" instead of a readable
// confirmation. The key now arrives in msg.key and is turned into the field's
// localized label (the same text shown above the input) inside the
// "<label>已保存" / "<label> saved" template. Failures still show the backend error.
function showIdentityResult(msg) {
  const el = document.createElement("div");
  el.className = "set-result " + (msg.ok ? "ok" : "err");
  el.textContent = msg.ok ? identitySavedText(msg.key) : (msg.message || "error");
  identityResult.prepend(el);
  setTimeout(() => el.remove(), 3000);
}

// identitySavedText composes the save confirmation for a field key, falling
// back to the key itself when the field list has no label for it.
function identitySavedText(key) {
  const f = identityFields.find((x) => x.key === key);
  const name = (f && f.label) || key || "";
  return i18nT("identitySaved", "{name}已保存").replace("{name}", name);
}

/* ---------- bootstrap ---------- */

// refreshBranch re-fetches /api/bootstrap and updates the sidebar title's
// git branch (e.g. "工作区 · main"). Called on page load and after every
// LLM iteration (done event) so a branch switch made by the agent is
// reflected without a manual reload.
async function refreshBranch() {
  try {
    const resp = await fetch("api/bootstrap");
    const b = await resp.json();
    if (b.branch) document.getElementById("wsBranch").textContent = b.branch;
  } catch { /* keep the current branch on failure */ }
}

(async function boot() {
  try {
    const resp = await fetch("api/bootstrap");
    const b = await resp.json();
    if (b.lang === "en") { T = I18N.en; currentLang = "en"; }
    document.documentElement.lang = b.lang || "zh";
    document.getElementById("ver").textContent = "v" + b.version + " [BUILD-" + b.build + "]";
    setAgentName(b.agent_name);
    // Browser tab title = "{current folder name} - {full absolute path}"
    // (FEATURE-366), so multiple co-shell tabs are distinguishable at a
    // glance and the most salient feature (the folder name) shows first
    // even when the tab is short.
    if (b.workspace) {
      // FEATURE-576: keep the workspace root so an absolute path printed in the
      // conversation can be recognised as a workspace file and converted to the
      // workspace-relative form every preview API expects (the file preview
      // stays confined to the workspace, see resolvePath in web/server.go).
      wsRoot = b.workspace.replace(/\\+$/, "").replace(/\/+$/, "");
      const name = wsRoot.split(/[\\/]/).pop() || wsRoot;
      document.title = name + " - " + b.workspace;
    }
    // Sidebar title shows the current git branch at the right edge of the
    // panel head (e.g. "工作区 ⟳   main").
    if (b.branch) document.getElementById("wsBranch").textContent = b.branch;
    // FEATURE-378: active text/vision model context info for the status bar.
    if (b.textModel) {
      modelInfo = { textModel: b.textModel, textModelID: b.textModelID || "", textMaxLen: b.textMaxLen || 0, visionModel: b.visionModel || "", visionModelID: b.visionModelID || "", visionMaxLen: b.visionMaxLen || 0, modeTextModelID: b.modeTextModelID || "", modeVisionModelID: b.modeVisionModelID || "" };
    }
    // FEATURE-455: remote access + download flags. When remote and download
    // are both enabled, the file list's reveal icon becomes a download icon.
    remoteAccess = !!b.remote;
    downloadEnabled = !!b.downloadEnabled;
  } catch { /* defaults stay zh */ }
  applyI18n();
  initStreamMode(); // FEATURE-425: wire the display-mode pill + session title
  initMsgViz(); // FEATURE-482: wire the message-visualization chart drag-to-pan
  autoGrow(); // FEATURE-405: set the initial input height correctly on load
  setRunning(false); // apply localized button title
  applyPanels();
  applyStatus();
  applyStreamTitle();
  applyMsgViz();
  updateStatus();
  // FEATURE-383: leaving the sidebar returns it to the fixed width after a
  // short delay (the auto-expand is triggered per-row on hover).
  let sidebarAutoTimer = 0;
  sidebar.addEventListener("mouseleave", () => {
    clearTimeout(sidebarAutoTimer);
    sidebarAutoTimer = setTimeout(() => layout.classList.remove("sidebar-auto"), 1000);
  });
  loadTree();
  wsConnect();
  updateResponsive(); // FEATURE-487: apply narrow-screen layout on load
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


/* ---------- FEATURE-487: narrow-screen responsive layout ---------- */

// The narrow-screen threshold mirrors the hub's approach: measure the full
// titlebar brand width (logo + co-shell + version) and treat the viewport as
// "narrow" when it is narrower than three times that width. This keeps the
// threshold adaptive to the configured logo scale / version text.
const brandEl = document.querySelector(".brand");
const brandNameEl = document.querySelector(".brand-name");
const verEl = document.getElementById("ver");
const agentNameEl = document.getElementById("agentName");
const menuWrapEl = document.getElementById("menuWrap");
const menuEl = document.getElementById("menu");
const narrowToolsEl = document.getElementById("narrowTools");
const inputRowEl = document.querySelector(".input-row");
const attachBtnEl = document.getElementById("attachBtn");
const sendBtnEl = document.getElementById("sendBtn");
const bottomLeftEl = document.querySelector(".bottom-left");
const streamHeadEl = document.querySelector(".stream-head");
const msgVizEl = document.getElementById("msgViz");
const streamModeEl = document.getElementById("streamMode");
const streamTitleEl = document.getElementById("streamTitle");
const streamActiveEl = document.getElementById("streamActive");

// Remember each movable control's original parent so we can restore it when
// the viewport widens again.
//
// FIX-573: remembering only the parent is not enough — restoring with
// `home.appendChild(el)` moves the control to the END of its parent, which made
// the new-session "+" button jump to the right of the input box and reordered
// the status-bar groups after a narrow<->wide round trip. Capture the sibling
// the control used to precede as well (the DOM is pristine at load time) and
// restore with insertBefore.
function narrowSlot(el) {
  return { el, home: el.parentNode, next: el.nextSibling };
}
const narrowMovables = [
  narrowSlot(modeSeg),
  // FEATURE-487: the main/vision model icons sit right of the work-mode
  // switcher in the narrow tool row (still clickable to switch model).
  narrowSlot(sbModelTextWrap),
  narrowSlot(sbModelVisionWrap),
  narrowSlot(newSessionBtn),
  narrowSlot(sbSessionsWrap),
  narrowSlot(attachBtnEl),
  narrowSlot(yoloSwitch),
  // FEATURE-487: the send button joins the narrow tool row (rightmost) so the
  // input row above holds only the textarea on narrow screens.
  narrowSlot(sendBtnEl),
];

// setAgentName writes the agent name (FIX-542) into the centre of the titlebar;
// an empty value clears the slot.
function setAgentName(name) {
  if (!agentNameEl) return;
  const n = String(name || "").trim();
  agentNameEl.textContent = n;
  // The name can be ellipsised when space runs out, so keep the full text in
  // the tooltip.
  agentNameEl.title = n;
}

// brandWidth returns the full titlebar brand width (logo + co-shell + version),
// measured with the version visible so the threshold reflects the full brand.
function brandWidth() {
  if (!brandEl) return 200;
  if (verEl) verEl.style.display = "";
  return brandEl.getBoundingClientRect().width || 200;
}

// The display-mode switcher (#streamMode) original home is the stream title
// bar; on narrow screens it moves into the titlebar just left of the connection
// control (#conn). Remember its home so it can be restored on wide screens.
const streamModeHome = streamModeEl ? streamModeEl.parentNode : null;
const topRightEl = document.querySelector(".top-right");

// moveStreamModeToTopbar moves #streamMode into the titlebar (left of #conn)
// on narrow screens, or back to the stream title bar on wide screens.
function moveStreamModeToTopbar(narrow) {
  if (!streamModeEl || !topRightEl || !conn) return;
  if (narrow) {
    if (streamModeEl.parentNode !== topRightEl) topRightEl.insertBefore(streamModeEl, conn);
  } else if (streamModeHome && streamModeEl.parentNode !== streamModeHome) {
    streamModeHome.appendChild(streamModeEl);
  }
}

// cycleDisplayMode advances the display mode (silent -> minimal -> normal ->
// silent) and re-renders. Used by the narrow single-capsule switcher.
function cycleDisplayMode() {
  const order = ["silent", "minimal", "normal"];
  const idx = order.indexOf(displayMode);
  displayMode = order[(idx + 1) % order.length];
  localStorage.setItem("co-shell-display-mode", displayMode);
  streamMode.querySelectorAll(".mode-seg-item").forEach((b) => b.classList.toggle("active", b.dataset.mode === displayMode));
  applyDisplayMode();
}

// cycleWorkMode advances the work mode to the next in the current mode list.
// The mode list order is captured from the last renderModeSeg call.
function cycleWorkMode() {
  if (!workModeOrder.length) return;
  const idx = workModeOrder.indexOf(currentMode);
  const next = workModeOrder[(idx + 1) % workModeOrder.length];
  if (next && next !== currentMode) wsSend({ type: "mode_switch", value: next });
}

// bindCycleSwitchers wires the narrow single-capsule switchers so clicking the
// visible active segment cycles to the next mode. On wide screens the original
// per-segment click handlers (set in initStreamMode / renderModeSeg) apply.
function bindCycleSwitchers(narrow) {
  if (!streamModeEl || !modeSeg) return;
  if (narrow) {
    streamModeEl.onclick = (e) => { e.stopPropagation(); cycleDisplayMode(); };
    modeSeg.onclick = (e) => { e.stopPropagation(); cycleWorkMode(); };
  } else {
    streamModeEl.onclick = null;
    modeSeg.onclick = null;
  }
}

// applyNarrowLayout toggles the body.narrow class and moves the shared tool
// controls (mode switcher, new-session, session list, attach, YOLO) into the
// dedicated narrow tool row (or back to their original homes on wide screens).
function applyNarrowLayout(narrow) {
  document.body.classList.toggle("narrow", narrow);
  if (!narrowToolsEl) return;
  if (narrow) {
    // Move controls into the narrow tool row (events travel with the nodes).
    for (const m of narrowMovables) {
      if (m.el && m.el.parentNode !== narrowToolsEl) narrowToolsEl.appendChild(m.el);
    }
    narrowToolsEl.classList.remove("hidden");
    // FEATURE-487: move the display-mode switcher into the titlebar (left of
    // the connection control) and make both switchers cycle on click.
    moveStreamModeToTopbar(true);
    bindCycleSwitchers(true);
  } else {
    // FIX-573: restore each control to its ORIGINAL POSITION — insert it back
    // before the sibling it used to precede (appendChild would push it to the
    // end of its parent, scrambling the input row and the status bar).
    for (const m of narrowMovables) {
      if (!m.el || !m.home || m.el.parentNode === m.home) continue;
      if (m.next && m.next.parentNode === m.home) m.home.insertBefore(m.el, m.next);
      else m.home.appendChild(m.el);
    }
    narrowToolsEl.classList.add("hidden");
    moveStreamModeToTopbar(false);
    bindCycleSwitchers(false);
  }
  // FEATURE-487: start/stop the cycling session-title widget on resize.
  stcSync();
}

// updateResponsive re-evaluates the narrow-screen state and applies the
// version-hide + layout changes. Called on load and on window resize.
function updateResponsive() {
  // Measure the brand width once (brandWidth() temporarily shows the version so
  // the threshold reflects the full brand). A single breakpoint: the viewport is
  // narrow when it is narrower than 3x the full titlebar brand width; otherwise
  // it is wide (normal).
  const fullW = brandWidth();
  const narrow = window.innerWidth < fullW * 3;
  // FEATURE-487: hide the titlebar version on narrow screens (like the hub).
  if (verEl) verEl.style.display = narrow ? "none" : "";
  // FIX-542: hide the wordmark too on narrow screens so the centred agent name
  // gets the full width; the brand triangle stays as the logo mark.
  if (brandNameEl) brandNameEl.style.display = narrow ? "none" : "";
  applyNarrowLayout(narrow);
}

window.addEventListener("resize", updateResponsive);

// FEATURE-524: keep the backend viewport in sync while the user resizes.
window.addEventListener("resize", scheduleViewportReport);

/* ---------- FEATURE-487: click-to-open menus (session / model / top-right) ---------- */

// closeAllPopups hides every open popup menu (session, model, vision and the
// top-right menu). Called when clicking anywhere outside a popup.
function closeAllPopups() {
  sessionMenu.classList.add("hidden");
  modelMenu.classList.add("hidden");
  modelVisionMenu.classList.add("hidden");
  // FEATURE-546: the usage popovers close with every other popup.
  if (sbSessionPop) sbSessionPop.classList.add("hidden");
  if (sbLastPop) sbLastPop.classList.add("hidden");
  if (menuWrapEl) menuWrapEl.classList.remove("open");
}

// Toggle a popup open/closed on click of its trigger; clicking anywhere else
// closes it (FEATURE-487).
function bindClickToggle(trigger, popup, onOpen) {
  if (!trigger || !popup) return;
  trigger.addEventListener("click", (e) => {
    e.stopPropagation();
    const willOpen = popup.classList.contains("hidden");
    closeAllPopups();
    if (willOpen) {
      if (onOpen) onOpen();
      // FEATURE-487: on narrow screens the session/model menus are full-width
      // sheets that pop up from the bottom tool row. Anchor their bottom edge
      // to the tool row's top so they span the viewport width above it.
      if (document.body.classList.contains("narrow") && narrowToolsEl) {
        const top = narrowToolsEl.getBoundingClientRect().top;
        popup.style.setProperty("--menu-bottom", Math.max(0, window.innerHeight - top) + "px");
      }
      popup.classList.remove("hidden");
    }
  });
}

// Session list: click the 💬 item to open, click anywhere to close.
bindClickToggle(sbSessions, sessionMenu, () => wsSend({ type: "session_list" }));

// FEATURE-546: the two usage popovers. The per-turn one renders from the
// in-memory token stats and the MTP probe that came with the model list, so it
// opens instantly; the lifetime one fetches GET /api/usage on open.
bindClickToggle(sbLast, sbLastPop, () => {
  sbLastPop.textContent = "";
  UsagePop.renderLast(sbLastPop, tokenStats, activeMTP, T);
});

bindClickToggle(sbSession, sbSessionPop, () => {
  sbSessionPop.textContent = "";
  const loading = document.createElement("div");
  loading.className = "usage-note";
  loading.textContent = T.usageLoading;
  sbSessionPop.appendChild(loading);
  fetch("api/usage").then((resp) => {
    if (!resp.ok) throw new Error("http " + resp.status);
    return resp.json();
  }).then((data) => {
    // The popover may have been closed while the request was in flight.
    if (sbSessionPop.classList.contains("hidden")) return;
    UsagePop.renderTotal(sbSessionPop, data, T);
  }).catch(() => {
    if (sbSessionPop.classList.contains("hidden")) return;
    sbSessionPop.textContent = "";
    const err = document.createElement("div");
    err.className = "usage-note usage-warn";
    err.textContent = T.usageFailed;
    sbSessionPop.appendChild(err);
  });
});
// Main (text) model selector: click to open.
bindClickToggle(sbModelText, modelMenu, () => wsSend({ type: "model_get" }));
// Vision model selector: click to open.
bindClickToggle(sbModelVision, modelVisionMenu, () => wsSend({ type: "model_get" }));
// Top-right menu: click the ☰ button to open/close; clicking anywhere else
// closes it (FEATURE-487). The .open class lives on the wrapper so the CSS
// rule .menu-wrap.open .menu controls visibility.
if (menuBtn && menuWrapEl) {
  menuBtn.addEventListener("click", (e) => {
    e.stopPropagation();
    const willOpen = !menuWrapEl.classList.contains("open");
    closeAllPopups();
    if (willOpen) menuWrapEl.classList.add("open");
  });
}

// Clicking anywhere outside a popup closes all of them.
document.addEventListener("click", (e) => {
  // Ignore clicks inside the top-right menu wrapper (its own handler manages it).
  if (menuWrapEl && menuWrapEl.contains(e.target)) return;
  closeAllPopups();
});


/* ---------- FEATURE-487: narrow-screen drawer swipe (workspace / task plan) ---------- */

// On narrow screens the workspace (#sidebar) and task-plan (#plan-panel) panels
// collapse into fixed drawers. Swiping right from the left edge opens the
// workspace; swiping left from the right edge opens the task plan. Clicking the
// main area closes an open drawer. Mouse drag and touch both work via pointer
// events.
const drawerSidebar = document.getElementById("sidebar");
const drawerPlan = document.getElementById("plan-panel");
const EDGE = 24; // px from the viewport edge that starts a swipe

let drawerDrag = null; // { side: "ws"|"plan", startX, startY }

function drawerOpen(el) {
  if (el) el.classList.add("drawer-open");
}
function drawerClose(el) {
  if (el) el.classList.remove("drawer-open");
}
function closeDrawers() {
  drawerClose(drawerSidebar);
  drawerClose(drawerPlan);
}

// Only enable swipe handling while the viewport is narrow.
function narrowActive() {
  return document.body.classList.contains("narrow");
}

document.addEventListener("pointerdown", (e) => {
  if (!narrowActive()) return;
  // Only start a swipe from near the left/right edge, and only when no drawer
  // is already open (an open drawer is closed by clicking the main area).
  const x = e.clientX;
  const vw = window.innerWidth;
  if (x <= EDGE) {
    drawerDrag = { side: "ws", startX: x, startY: e.clientY };
  } else if (x >= vw - EDGE) {
    drawerDrag = { side: "plan", startX: x, startY: e.clientY };
  }
});

document.addEventListener("pointermove", (e) => {
  if (!drawerDrag) return;
  const dx = e.clientX - drawerDrag.startX;
  const dy = e.clientY - drawerDrag.startY;
  // Require a mostly-horizontal drag of at least 40px to trigger.
  if (Math.abs(dx) < 40 || Math.abs(dx) < Math.abs(dy) * 1.5) return;
  if (drawerDrag.side === "ws" && dx > 0) {
    drawerOpen(drawerSidebar);
    drawerDrag = null;
  } else if (drawerDrag.side === "plan" && dx < 0) {
    drawerOpen(drawerPlan);
    drawerDrag = null;
  }
});

document.addEventListener("pointerup", () => { drawerDrag = null; });
document.addEventListener("pointercancel", () => { drawerDrag = null; });

// Clicking the main message area closes any open drawer.
document.addEventListener("click", (e) => {
  if (!narrowActive()) return;
  if (drawerSidebar && drawerSidebar.contains(e.target)) return;
  if (drawerPlan && drawerPlan.contains(e.target)) return;
  closeDrawers();
});


/* ---------- FEATURE-487: settings two-level navigation (narrow screens) ---------- */

// The "<" back button returns from a drilled-in category page to the level-1
// category list. It is always present but only acts as a drill-out on narrow
// screens (on wide screens the list and pane are shown side by side).
const settingsBackBtn = document.getElementById("settingsBack");
if (settingsBackBtn) {
  settingsBackBtn.addEventListener("click", () => settingsShowList());
}

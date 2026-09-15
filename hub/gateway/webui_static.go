package gateway

import "errors"

// errClosed is returned when serving on a closed listener.
var errClosed = errors.New("use of closed network connection")

// webIndexHTML is the embedded hub shell page (FEATURE-484). It embeds each
// registered agent's full co-shell Web UI in a full-screen <iframe> loaded from
// /agent/{id}/ (reverse-proxied by the hub). To minimise intrusion on the
// co-shell UI, the hub chrome is minimal:
//
//   - A small "co-shell-hub" badge floats over the top-left corner of the
//     iframe (covering the co-shell logo/version area) so the user perceives
//     they are on the hub, not a bare co-shell instance.
//   - The agent list lives in a left drawer that is collapsed to a thin edge
//     strip by default and slides open when the pointer dwells on the left
//     edge (or the badge is clicked). On wide screens the open drawer pushes
//     the iframe aside; on narrow screens it overlays the iframe.
//   - The drawer has two internal views: the agent list and the config view
//     (add local/remote agent + start/stop/delete). "Agent 管理" switches to
//     the config view; its back button returns to the agent list.
const webIndexHTML = `<!DOCTYPE html>
<html lang="zh">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>co-shell-hub</title>
<link rel="icon" type="image/svg+xml" href="/favicon.svg">
<link rel="icon" type="image/png" href="/favicon.png">
<style>
  :root {
    --bg:#0b0e14; --panel:#10141d; --elev:#161b26; --fg:#d5dbe7; --fg-dim:#8b93a5;
    --fg-faint:#5b6373; --accent:#3fd6ef; --accent-dim:rgba(63,214,239,.14);
    --border:#232a3a; --ok:#4ade80; --err:#f87171; --warn:#facc15;
    --edge-w:10px; --panel-w:340px;
    /* Height of the co-shell top bar inside the iframe; the hub drawer starts
       below it so the hub chrome never covers the title bar (FEATURE-515). */
    --topbar-h:44px;
  }
  * { box-sizing:border-box; }
  html,body { height:100%; }
  body { margin:0; font-family:system-ui,-apple-system,sans-serif; background:var(--bg); color:var(--fg); overflow:hidden; }

  /* Full-screen iframe stage. */
  #stage { position:fixed; inset:0; transition:left .22s ease; }
  .frame { position:absolute; inset:0; width:100%; height:100%; border:none; background:#fff; display:none; }
  .frame.active { display:block; }
  .empty { position:absolute; inset:0; display:flex; flex-direction:column; align-items:center; justify-content:center; gap:10px; color:var(--fg-dim); text-align:center; padding:20px; }
  /* FEATURE-515: the empty (initial) state shows the hub vector icon — the same
     artwork as the favicon — as its logo; clicking it opens the create panel. */
  .empty .logo { width:96px; height:96px; cursor:pointer; transition:transform .15s ease; }
  .empty .logo:hover { transform:scale(1.08); }

  /* Hub badge floating over the co-shell logo area (top-left, 44px tall to
     match the co-shell topbar). Clicking it toggles the agent drawer.
     FEATURE-492: after 3s it fades to fully transparent (still covering the
     co-shell logo area and still clickable); hovering restores it, leaving
     fades it again. */
  #hubBadge {
    position:fixed; top:0; left:0; height:44px; padding:0 14px;
    display:flex; align-items:center; gap:8px; cursor:pointer; z-index:30;
    background:var(--panel); color:var(--fg); user-select:none;
    border-right:1px solid var(--border); border-bottom:1px solid var(--border);
    border-bottom-right-radius:8px; font-size:14px; white-space:nowrap;
    opacity:1; transition:opacity .5s ease;
  }
  #hubBadge.faded { opacity:0; }
  #hubBadge:hover { background:var(--elev); opacity:1; }
  /* FEATURE-516: when the drawer is open the badge becomes the drawer's logo
     bar: same width as the agent panel, with the pin pushed to its right edge. */
  #hubBadge.open { width:var(--panel-w); transition:width .22s ease; border-bottom-right-radius:0; }
  #hubBadge .pin {
    margin-left:auto; display:flex; align-items:center; cursor:pointer;
    color:var(--fg-dim); padding:2px 0; line-height:1;
    transition:color .2s ease, transform .2s ease;
  }
  #hubBadge .pin:hover { color:var(--fg); }
  #hubBadge .pin.pinned { color:var(--accent); transform:rotate(45deg); }
  #hubBadge .mark { color:var(--accent); font-weight:700; transition:color .3s ease; }
  #hubBadge .mark.off { color:var(--fg-faint); } /* disconnected: grey triangle */
  #hubBadge .name { font-weight:600; letter-spacing:.4px; }
  #hubBadge .ver { color:var(--fg-faint); font-size:12px; font-family:ui-monospace,Menlo,monospace; }

  /* Left edge hot-zone that reveals the agent drawer on hover. */
  #edge {
    position:fixed; top:0; left:0; bottom:0; width:var(--edge-w); z-index:20; cursor:pointer;
  }

  /* Left agent drawer. Collapsed by default (translated off-screen left,
     leaving only the edge hot-zone). FEATURE-515: the drawer starts right
     below the co-shell top bar (--topbar-h) so it never covers the title bar. */
  #agentPanel {
    position:fixed; top:var(--topbar-h); left:0; bottom:0; width:var(--panel-w); z-index:25;
    background:var(--panel); border-right:1px solid var(--border);
    transform:translateX(-100%); transition:transform .22s ease;
    display:flex; flex-direction:column;
  }
  #agentPanel.open { transform:translateX(0); }
  #agentPanel .head {
    flex:none; height:44px; display:flex; align-items:center; gap:8px; padding:0 12px;
    border-bottom:1px solid var(--border); font-weight:600; font-size:14px;
  }
  #agentPanel .head .mark { color:var(--accent); }

  /* FEATURE-515: the drawer stacks dockable sections (Agents / Chat). Every
     section head is a fixed 44px bar; the two bodies split the remaining
     height, and a collapsed body animates to zero height — so the collapsed
     section docks under the expanded one (bottom of the drawer by default). */
  #agentPanel .pane-head {
    flex:none; height:44px; display:flex; align-items:center; gap:8px; padding:0 12px;
    border-bottom:1px solid var(--border); font-weight:600; font-size:14px;
    cursor:pointer; user-select:none; transition:background .18s ease;
  }
  #agentPanel .pane-head:hover { background:var(--elev); }
  #agentPanel .pane-head .mark { color:var(--accent); }
  #agentPanel .pane-head .chev {
    margin-left:auto; font-size:12px; color:var(--fg-faint);
    transition:transform .28s ease;
  }
  #agentPanel .pane-head.collapsed .chev { transform:rotate(-90deg); }
  #agentPanel .pane-body {
    flex:1 1 0; min-height:0; display:flex; flex-direction:column; overflow:hidden;
    opacity:1; transition:flex-grow .28s ease, opacity .22s ease;
  }
  #agentPanel .pane-body.collapsed { flex-grow:0; opacity:0; pointer-events:none; }
  /* Chat placeholder: a disabled input mock so the pane reads as "not yet". */
  .chat-placeholder { flex:1; display:flex; flex-direction:column; justify-content:flex-end; gap:6px; padding:10px 10px 16px; }
  .chat-mock {
    display:flex; align-items:center; gap:8px; padding:8px 10px; border:1px solid var(--border);
    border-radius:10px; background:var(--bg); color:var(--fg-faint); font-size:13px; cursor:not-allowed;
  }
  .chat-mock .txt { flex:1; }
  .chat-mock .send { flex:none; font-size:13px; color:var(--fg-faint); }
  #agentPanel .head .close { margin-left:auto; cursor:pointer; color:var(--fg-dim); font-size:16px; padding:2px 6px; }
  #agentPanel .head .close:hover { color:var(--fg); }
  /* FEATURE-492/FEATURE-516: the pin moved from the Agents section head onto
     the hub logo bar (#hubBadge .pin above); it pins the drawer open so that
     mouseleave no longer auto-collapses it. */
  #agentList { flex:1; overflow-y:auto; padding:8px; }
  /* Each list row is a swipe container: a red delete button sits behind the
     card and is revealed by swiping the card left. */
  .agent-wrap { position:relative; overflow:hidden; border-radius:8px; margin-bottom:2px; background:var(--panel); }
  .agent-del {
    position:absolute; top:0; right:1px; bottom:0; width:64px; border:none;
    background:var(--err); color:#fff; font-size:13px; font-weight:600; cursor:pointer;
  }
  .agent {
    position:relative; display:flex; align-items:center; gap:8px; padding:8px 10px;
    background:var(--panel); cursor:pointer; font-size:13px; color:var(--fg-dim);
    border-radius:8px; /* FEATURE-492: match the wrap radius so the delete button behind is fully covered */
    transition:transform .18s ease;
  }
  .agent:hover { background:var(--elev); color:var(--fg); }
  .agent.active { background:#0e2a33; color:var(--accent); font-weight:600; }
  /* FEATURE-499: three-state status light. Default (no extra class) = grey
     (agent closed). .idle = green (open, no task running). .busy = red
     breathing light (a task is executing). */
  .agent .st { width:8px; height:8px; border-radius:50%; background:#555; flex:none; }
  .agent .st.idle { background:var(--ok); box-shadow:0 0 4px var(--ok); }
  .agent .st.busy { background:var(--err); animation:stBreath 1.2s ease-in-out infinite; }
  @keyframes stBreath {
    0%,100% { box-shadow:0 0 2px var(--err); opacity:.55; }
    50% { box-shadow:0 0 8px var(--err); opacity:1; }
  }
  .agent .nm { flex:1; min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
  .agent .chev { flex:none; font-size:16px; color:var(--fg-faint); padding:0 2px; cursor:pointer; line-height:1; }
  .agent .chev:hover { color:var(--accent); }
  /* Sliding power switch (start/stop). */
  .switch { position:relative; display:inline-block; width:34px; height:18px; flex:none; cursor:pointer; }
  .switch input { opacity:0; width:0; height:0; }
  .switch .slider {
    position:absolute; inset:0; border-radius:999px; background:#3a4256;
    transition:background .18s ease;
  }
  .switch .slider:before {
    content:''; position:absolute; top:2px; left:2px; width:14px; height:14px;
    border-radius:50%; background:#fff; transition:transform .18s ease;
  }
  .switch input:checked + .slider { background:var(--ok); }
  .switch input:checked + .slider:before { transform:translateX(16px); }
  .switch input:disabled + .slider { opacity:.5; cursor:not-allowed; }
  .switch-row { display:flex; align-items:center; justify-content:space-between; }
  .switch-row .switch-label { font-size:12px; color:var(--fg-dim); }
  /* FEATURE-487: bottom buttons sit 10px higher (extra bottom padding) so they
     are easier to tap on mobile. */
  #agentPanel .foot { flex:none; padding:8px 8px 18px; border-top:1px solid var(--border); }
  .btn { padding:6px 12px; border-radius:6px; border:none; cursor:pointer; font-size:12px; font-weight:600; }
  .btn.primary { background:var(--accent); color:#0b0e14; width:100%; }
  .btn.primary:hover { filter:brightness(1.1); }

  /* Scrim overlay behind the left drawer. */
  #scrim { position:fixed; inset:0; background:rgba(0,0,0,.4); z-index:24; display:none; }
  #scrim.show { display:block; }

  /* Two internal views inside the left drawer: list and config. */
  #agentPanel .view { flex:1; min-height:0; display:flex; flex-direction:column; }
  #agentPanel .view.hidden { display:none; }
  #agentPanel .view.slide-in { animation:viewIn .18s ease; }
  #agentPanel .view.slide-out { animation:viewOut .18s ease; }
  @keyframes viewIn { from { transform:translateX(24px); opacity:0; } to { transform:translateX(0); opacity:1; } }
  @keyframes viewOut { from { transform:translateX(0); opacity:1; } to { transform:translateX(-24px); opacity:0; } }
  #agentPanel .head .back { cursor:pointer; color:var(--fg-dim); font-size:18px; padding:0 4px; border:none; background:none; line-height:1; }
  #agentPanel .head .back:hover { color:var(--fg); }
  #agentPanel .config-body { flex:1; overflow-y:auto; padding:0 14px 14px; }
  #agentPanel h3 { font-size:13px; margin:16px 0 6px; color:var(--accent); }
  .field { margin-bottom:14px; }
  .field label { display:block; font-size:12px; color:var(--fg-dim); margin-bottom:5px; }
  .field input, .field select { width:100%; padding:7px 9px; border-radius:6px; border:1px solid var(--border); background:var(--bg); color:var(--fg); font-size:13px; }
  /* FEATURE-520: a field whose control does not apply to this agent type
     (e.g. Workspace for a remote agent) is greyed out and non-editable. */
  .field.disabled label { opacity:.5; }
  .field.disabled input, .field.disabled select { opacity:.5; cursor:not-allowed; }
  .field .check { display:flex; align-items:center; gap:6px; }
  .field .check input { width:auto; }
  .req { color:var(--err); font-weight:700; }
  .seg { display:flex; gap:4px; margin-bottom:12px; background:var(--bg); border:1px solid var(--border); border-radius:8px; padding:3px; }
  .seg-btn { flex:1; padding:5px 0; border:none; border-radius:6px; background:transparent; color:var(--fg-dim); font-size:13px; cursor:pointer; }
  .seg-btn.active { background:var(--accent); color:#0b0e14; font-weight:600; }
  .ver-ok { color:var(--ok); }
  .ver-err { color:var(--err); }
  .btn.ok { background:var(--ok); color:#0b0e14; }
  .btn.stop { background:var(--warn); color:#0b0e14; }
  .btn.danger { background:var(--err); color:#fff; }
  .btn:disabled { opacity:.5; cursor:not-allowed; }
  .agent-row { border:1px solid var(--border); border-radius:8px; padding:8px; margin-bottom:8px; background:var(--bg); }
  .agent-row .top { display:flex; align-items:center; gap:6px; }
  .agent-row .name { font-weight:600; font-size:13px; flex:1; min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
  .agent-row .meta { font-size:11px; color:var(--fg-dim); margin:4px 0; word-break:break-all; }
  .badge { font-size:10px; padding:1px 6px; border-radius:8px; flex:none; }
  .badge.managed { background:var(--elev); color:var(--accent); }
  .badge.external { background:var(--elev); color:var(--warn); }
  .badge.on { background:#0f2a1a; color:var(--ok); }
  .badge.off { background:#2a1010; color:var(--err); }
  .agent-row .actions { display:flex; gap:6px; margin-top:6px; }
  .hint { font-size:11px; color:var(--fg-dim); margin-top:4px; }
  .icon-btn { width:28px; height:28px; border:none; border-radius:6px; background:transparent; color:var(--fg); font-size:16px; cursor:pointer; }
  .icon-btn:hover { background:var(--elev); }

  /* FEATURE-516: inline SVG icons (sprite <use>). Geometry instead of emoji /
     font glyphs, coloured by currentColor so they follow the surrounding text. */
  .ico { display:inline-block; vertical-align:middle; flex:none; width:14px; height:14px; }
  .icon-sprite { position:absolute; width:0; height:0; overflow:hidden; }
  .ico-inline { width:12px; height:12px; }
  #hubBadge .mark { width:12px; height:12px; }
  #hubBadge .pin .ico { width:18px; height:18px; }
  #agentPanel .pane-head .chev .ico { width:14px; height:14px; }
  #agentPanel .head .back .ico { width:16px; height:16px; }
  #agentPanel .head .close .ico { width:15px; height:15px; }
  .btn .ico { width:15px; height:15px; margin-right:5px; }
  .chat-mock .send { display:flex; align-items:center; }
  .chat-mock .send .ico { width:15px; height:15px; }

  /* Wide screens: an open drawer pushes the iframe aside (side-by-side). */
  @media (min-width:901px) {
    body.drawer-open #stage { left:var(--panel-w); }
  }
  /* Narrow screens: the drawer overlays the iframe. */
  @media (max-width:900px) {
    #agentPanel { box-shadow:2px 0 12px rgba(0,0,0,.4); }
  }
</style>
</head>
<body>

<!-- FEATURE-516: inline SVG icon sprite for the hub chrome. Icons are drawn as
     geometry (not emoji / font glyphs) so they render identically on every OS
     and browser and inherit the current text colour via currentColor. -->
<svg class="icon-sprite" width="0" height="0" aria-hidden="true" focusable="false">
  <defs>
    <symbol id="i-hub-mark" viewBox="0 0 24 24">
      <path d="M7.6 4.8 19 12 7.6 19.2Z" fill="currentColor" stroke="currentColor"
            stroke-width="1.6" stroke-linejoin="round"/>
    </symbol>
    <symbol id="i-pin" viewBox="0 0 24 24">
      <!-- Push-pin silhouette (filled), legible at 16-18px; rotates 45° when
           pinned (see #hubBadge .pin.pinned). -->
      <path d="M9 3h6v2.2h-1.4v4.4l3.1 2.2c.5.4.8.9.8 1.5V14H6v-.7c0-.6.3-1.1.8-1.5l3.1-2.2V5.2H9V3Z"
            fill="currentColor"/>
      <path d="M12 14.2v6.3" fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round"/>
    </symbol>
    <symbol id="i-chev" viewBox="0 0 24 24">
      <path d="M6 9.5 12 15.5 18 9.5"
            fill="none" stroke="currentColor" stroke-width="2"
            stroke-linecap="round" stroke-linejoin="round"/>
    </symbol>
    <symbol id="i-back" viewBox="0 0 24 24">
      <path d="M14.5 5 7.5 12l7 7"
            fill="none" stroke="currentColor" stroke-width="2"
            stroke-linecap="round" stroke-linejoin="round"/>
    </symbol>
    <symbol id="i-close" viewBox="0 0 24 24">
      <path d="M6.5 6.5l11 11M17.5 6.5l-11 11"
            fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round"/>
    </symbol>
    <symbol id="i-plus" viewBox="0 0 24 24">
      <path d="M12 5v14M5 12h14"
            fill="none" stroke="currentColor" stroke-width="1.9" stroke-linecap="round"/>
    </symbol>
    <symbol id="i-gear" viewBox="0 0 24 24">
      <circle cx="12" cy="12" r="3" fill="none" stroke="currentColor" stroke-width="1.7"/>
      <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1Z"
            fill="none" stroke="currentColor" stroke-width="1.7"
            stroke-linecap="round" stroke-linejoin="round"/>
    </symbol>
    <symbol id="i-send" viewBox="0 0 24 24">
      <path d="M20.5 3.5 3.5 10.5l7 3 3 7 7-17Z"
            fill="none" stroke="currentColor" stroke-width="1.7"
            stroke-linecap="round" stroke-linejoin="round"/>
    </symbol>
    <symbol id="i-check" viewBox="0 0 24 24">
      <path d="M5 12.5l4.5 4.5L19 7.5"
            fill="none" stroke="currentColor" stroke-width="2"
            stroke-linecap="round" stroke-linejoin="round"/>
    </symbol>
  </defs>
</svg>
<div id="stage">
  <div class="empty" id="empty">
    <img class="logo" id="emptyRun" src="/favicon.svg" alt="co-shell-hub" title="创建或添加 Agent">
    <div>暂无 Agent。点击上方"运行"箭头，或左上角 co-shell-hub 徽标再点"管理"创建或添加 Agent。</div>
  </div>
</div>

<!-- Hub badge over the co-shell logo area; toggles the agent drawer. -->
<div id="hubBadge" title="co-shell-hub · 点击展开 Agent 列表">
  <svg class="ico mark" aria-hidden="true"><use href="#i-hub-mark"/></svg><span class="name">co-shell-hub</span><span class="ver" id="hubVer"></span><span class="pin" id="panelPin" title="钉住（不自动收起）"><svg class="ico" aria-hidden="true"><use href="#i-pin"/></svg></span>
</div>

<!-- Left edge hot-zone (reveals the drawer on hover). -->
<div id="edge"></div>

<!-- Scrim overlay behind the left drawer. -->
<div id="scrim"></div>

<!-- Left drawer (FEATURE-515): two dockable sections, Agents / Chat. -->
<div id="agentPanel">
  <!-- Agents section head (click expands Agents; the pin keeps the drawer open). -->
  <div class="pane-head" id="agentsHead">Agents<span class="chev"><svg class="ico" aria-hidden="true"><use href="#i-chev"/></svg></span></div>
  <div class="pane-body" id="agentsBody">
  <!-- View 1: agent list. -->
  <div class="view" id="viewList">
    <div id="agentList"></div>
    <div class="foot"><button class="btn primary" id="manageBtn"><svg class="ico" aria-hidden="true"><use href="#i-plus"/></svg>新建</button><button class="btn" id="settingsBtn" style="margin-top:16px;width:100%"><svg class="ico" aria-hidden="true"><use href="#i-gear"/></svg>设置</button></div>
  </div>
  <!-- View 2: config (add local/remote + manage list). -->
  <div class="view hidden" id="viewConfig">
    <div class="head"><button class="back" id="configBack" title="返回 Agent 列表"><svg class="ico" aria-hidden="true"><use href="#i-back"/></svg></button>新建 Agent<span class="close" id="configClose" title="收起"><svg class="ico" aria-hidden="true"><use href="#i-close"/></svg></span></div>
    <div class="config-body">
      <h3>添加 Agent</h3>
      <div class="seg" id="modeSeg">
        <button class="seg-btn active" data-mode="local">本地</button>
        <button class="seg-btn" data-mode="remote">远程</button>
      </div>
      <!-- Local mode: hub launches a co-shell --serve subprocess. -->
      <div id="localFields">
        <div class="field"><label><span class="req">*</span>Workspace 路径</label><input id="m-ws" placeholder="如 ~/.co-shell/agents/agent-1"></div>
        <div class="field"><label>端口号（自动分配，可修改）</label><input id="m-port" placeholder="自动分配"></div>
        <div class="field"><label>ID（默认取 workspace 末段）</label><input id="m-id" placeholder="自动生成"></div>
        <div class="field"><label>备注</label><input id="m-name" placeholder="可选"></div>
        <div class="field"><label>co-shell 可执行程序</label><select id="m-coshell"></select></div>
        <div class="field"><div class="switch-row"><span class="switch-label">共享配置</span><label class="switch"><input type="checkbox" id="m-shared"><span class="slider"></span></label></div><div class="hint" id="m-shared-hint">开启：使用 ~/.co-shell/config.json（共享）；关闭：使用 {workspace}/config.json（不存在则自动创建空文件）。需重启 agent 后生效。</div></div>
        <div class="field"><label>补充运行参数</label><input id="m-extra" placeholder="如 --accept-license --serve" value="--accept-license"></div>
        <div class="hint" id="m-ver"></div>
      </div>
      <!-- Remote mode: user supplies a host + port (hub builds the ws URL). -->
      <div id="remoteFields" style="display:none">
        <div class="field"><label><span class="req">*</span>主机地址</label><input id="e-host" placeholder="IP 或主机名，如 192.168.1.5"></div>
        <div class="field"><label><span class="req">*</span>端口号</label><input id="e-port" placeholder="自动推荐，可修改"></div>
        <div class="field"><label>ID（默认 host-port）</label><input id="e-id" placeholder="自动生成"></div>
        <div class="field"><label>备注</label><input id="e-name" placeholder="可选"></div>
        <div class="hint" id="e-ver"></div>
      </div>
    </div>
    <div class="foot"><button class="btn primary" id="cfg-submit">创建本地 Agent</button></div>
  </div>
  <!-- View 3: agent settings — editable (FEATURE-520). The field set mirrors
       the create form; a field that does not apply to the agent's type is
       greyed out (managed: 主机地址; external: Workspace / co-shell /
       共享配置 / 补充运行参数). The ID is immutable. -->
  <div class="view hidden" id="viewDetail">
    <div class="head"><button class="back" id="detailBack" title="返回 Agent 列表"><svg class="ico" aria-hidden="true"><use href="#i-back"/></svg></button><span id="detailTitle">Agent 设置</span></div>
    <div class="config-body">
      <div class="field"><label>ID（不可修改）</label><input id="d-id" readonly></div>
      <div class="field"><label>备注</label><input id="d-name" placeholder="可选"></div>
      <div class="field"><label>类型</label><input id="d-type" readonly></div>
      <div class="field" id="f-ws"><label>Workspace 路径</label><input id="d-ws" placeholder="如 ~/.co-shell/agents/agent-1"></div>
      <div class="field" id="f-host"><label>主机地址</label><input id="d-host" placeholder="IP 或主机名，如 192.168.1.5"></div>
      <div class="field" id="f-port"><label id="d-port-label">端口号</label><input id="d-port" placeholder="如 28256"></div>
      <div class="field" id="f-coshell"><label>co-shell 可执行程序</label><select id="d-coshell"></select><div class="hint" id="d-ver"></div></div>
      <div class="field" id="f-shared"><div class="switch-row"><span class="switch-label">共享配置</span><label class="switch"><input type="checkbox" id="d-shared"><span class="slider"></span></label></div><div class="hint">开启：使用 ~/.co-shell/config.json（共享）；关闭：使用 {workspace}/config.json（不存在则自动创建空文件）。需重启 agent 后生效。</div></div>
      <div class="field" id="f-extra"><label>补充运行参数</label><input id="d-extra" placeholder="如 --accept-license"></div>
      <div class="hint" id="d-state"></div>
    </div>
    <div class="foot"><button class="btn primary" id="d-save">保存修改</button><div class="hint" id="d-msg"></div></div>
  </div>
  <!-- View 4: remote-access settings (TLS/whitelist/access key). -->
  <div class="view hidden" id="viewSettings">
    <div class="head"><button class="back" id="settingsBack" title="返回 Agent 列表"><svg class="ico" aria-hidden="true"><use href="#i-back"/></svg></button>设置<span class="close" id="settingsClose" title="收起"><svg class="ico" aria-hidden="true"><use href="#i-close"/></svg></span></div>
    <div class="config-body">
      <h3>HTTPS 访问</h3>
      <div class="field"><div class="switch-row"><span class="switch-label">启用 HTTPS（配置证书后仅用 https 访问）</span><label class="switch"><input type="checkbox" id="s-tls"><span class="slider"></span></label></div></div>
      <div class="field"><label>证书文件路径（PEM）</label><input id="s-cert" placeholder="如 /path/to/cert.pem，留空自动生成自签名证书"></div>
      <div class="field"><label>私钥文件路径（PEM）</label><input id="s-keyfile" placeholder="如 /path/to/key.pem"></div>
      <div class="field"><button class="btn" id="s-gencert" style="width:100%">生成自签名证书</button><div class="hint">生成后需重启 hub 生效。</div></div>
      <h3>访问控制</h3>
      <div class="field"><label>白名单（IP 或 CIDR，逗号分隔，空=仅本机）</label><input id="s-whitelist" placeholder="如 192.168.1.100,192.168.1.0/24"></div>
      <div class="field"><label>访问验证 KEY（白名单外主机需在请求头 X-Access-Key 提供）</label><input id="s-accesskey" type="password" placeholder="留空表示不修改当前 KEY"><button class="btn" id="s-genkey" style="margin-top:6px;width:100%">重新生成安全 KEY</button></div>
      <div class="field"><div class="switch-row"><span class="switch-label">全部访问都需要 KEY</span><label class="switch"><input type="checkbox" id="s-reqkey"><span class="slider"></span></label></div><div class="hint">开启后即使 IP 在白名单内也需提供访问 KEY。</div></div>
      <div class="hint" id="s-status"></div>
    </div>
    <div class="foot"><button class="btn primary" id="s-save">保存设置</button></div>
  </div>
  </div><!-- /#agentsBody -->
  <!-- Chat section (collapsed by default; placeholder UI only, FEATURE-515). -->
  <div class="pane-head collapsed" id="chatHead">Chat<span class="chev"><svg class="ico" aria-hidden="true"><use href="#i-chev"/></svg></span></div>
  <div class="pane-body collapsed" id="chatBody">
    <div class="chat-placeholder">
      <div class="chat-mock" title="Chat 功能规划中，暂不可用"><span class="txt">输入消息…</span><span class="send"><svg class="ico" aria-hidden="true"><use href="#i-send"/></svg></span></div>
      <div class="hint">Chat 功能规划中，敬请期待</div>
    </div>
  </div>
</div>
<script>
(function(){
  var stageEl = document.getElementById('stage');
  var emptyEl = document.getElementById('empty');
  var badge = document.getElementById('hubBadge');
  var edge = document.getElementById('edge');
  var panel = document.getElementById('agentPanel');
  var agentsHead = document.getElementById('agentsHead');
  var agentsBody = document.getElementById('agentsBody');
  var chatHead = document.getElementById('chatHead');
  var chatBody = document.getElementById('chatBody');
  var listEl = document.getElementById('agentList');
  var viewList = document.getElementById('viewList');
  var viewConfig = document.getElementById('viewConfig');
  var viewDetail = document.getElementById('viewDetail');
  var viewSettings = document.getElementById('viewSettings');
  var scrim = document.getElementById('scrim');
  var agents = [];
  var current = null;
  var frames = {};
  var hideTimer = null;
  var hubVerEl = document.getElementById('hubVer');
  var openCard = null; // currently swipe-open agent card (delete revealed)
  var markEl = badge.querySelector('.mark'); // hub connection indicator triangle
  var pollTimer = null; // setInterval handle for the agent-list poll
  var connected = true; // whether the last hub API poll succeeded

  function esc(s){ return String(s).replace(/[&<>"']/g,function(c){return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c];}); }

  function api(method, url, body, cb){
    var opts = { method: method, headers: {} };
    if (body){ opts.headers['Content-Type']='application/json'; opts.body=JSON.stringify(body); }
    fetch(url, opts).then(function(r){ return r.json().then(function(j){ cb(r.status, j); }); })
      .catch(function(e){ cb(0, { error: String(e) }); });
  }

  // ---- Drawer open/close ----
  // FEATURE-515: the drawer holds dockable sections (Agents / Chat). Exactly
  // one body expands (flex-grow 1) while the other animates to zero height, so
  // the collapsed section docks under the expanded one. Agents is the default.
  function setPane(name){
    var chatOn = name === 'chat';
    agentsHead.classList.toggle('collapsed', chatOn);
    agentsBody.classList.toggle('collapsed', chatOn);
    chatHead.classList.toggle('collapsed', !chatOn);
    chatBody.classList.toggle('collapsed', !chatOn);
  }
  agentsHead.onclick = function(){ setPane('agents'); };
  chatHead.onclick = function(){ setPane('chat'); };

  function openPanel(){
    clearTimeout(hideTimer);
    panel.classList.add('open');
    // FEATURE-516: the hub logo bar expands together with the drawer (same
    // width) and stays visible for as long as the drawer is open.
    badge.classList.add('open');
    badge.classList.remove('faded');
    scrim.classList.add('show');
    document.body.classList.add('drawer-open');
  }
  // FEATURE-492: when the drawer is pinned, leaving it never auto-collapses.
  var pinned = false;
  function closePanel(){
    panel.classList.remove('open');
    badge.classList.remove('open');
    scrim.classList.remove('show');
    document.body.classList.remove('drawer-open');
    setPane('agents'); // FEATURE-515: reopening always shows the default section
    showView('list');
  }
  function scheduleClose(){
    if (pinned) return;
    clearTimeout(hideTimer);
    hideTimer = setTimeout(function(){
      if (pinned) return; // re-check at fire time: pinned drawers never auto-close
      closePanel();
    }, 600);
  }
  // showView switches between the list, config and detail views with a slide
  // transition: the current view slides out, then the target slides in.
  var viewEls = [viewList, viewConfig, viewDetail, viewSettings];
  function showView(name){
    var target = name === 'list' ? viewList : (name === 'detail' ? viewDetail : (name === 'settings' ? viewSettings : viewConfig));
    // FEATURE-515: config/detail/settings live inside the Agents section, so
    // make sure that section is expanded before revealing them.
    if (target !== viewList) setPane('agents');
    var cur = viewEls.filter(function(v){ return !v.classList.contains('hidden'); })[0];
    if (cur === target) return;
    if (cur){
      cur.classList.add('slide-out');
      setTimeout(function(){
        cur.classList.remove('slide-out');
        cur.classList.add('hidden');
        revealView(target);
      }, 180);
    } else {
      revealView(target);
    }
  }
  function revealView(v){
    v.classList.remove('hidden');
    v.classList.add('slide-in');
    setTimeout(function(){ v.classList.remove('slide-in'); }, 200);
  }
  badge.onclick = function(){
    if (!connected) startPolling(); // disconnected: clicking the triangle reconnects
    panel.classList.contains('open') ? closePanel() : openPanel();
  };
  // FEATURE-492: the drawer head pin toggles the pinned state. Pinned drawers
  // stay open (no auto-collapse on mouseleave); clicking the pin again unpins.
  var pinEl = document.getElementById('panelPin');
  pinEl.onclick = function(e){
    e.stopPropagation(); // the head itself is clickable (section toggle)
    pinned = !pinned;
    pinEl.classList.toggle('pinned', pinned);
    pinEl.title = pinned ? '已钉住（点击取消）' : '钉住（不自动收起）';
    if (pinned) clearTimeout(hideTimer);
  };
  // The config view's close/back buttons return to the agent list view.
  document.getElementById('configClose').onclick = function(){ showView('list'); };
  document.getElementById('configBack').onclick = function(){ showView('list'); };
  // The detail view's back button returns to the agent list view. FEATURE-516:
  // its top-right close control was removed (the back button covers it).
  document.getElementById('detailBack').onclick = function(){ showView('list'); };
  // Hover the left edge to open; leaving the panel schedules a close only in
  // the list view (the config view stays open until closed or backed out).
  // FEATURE-515: only a sustained hover (1s) opens the drawer, so merely
  // brushing the left edge no longer pops it open.
  var edgeTimer = 0;
  edge.addEventListener('mouseenter', function(){
    clearTimeout(edgeTimer);
    edgeTimer = setTimeout(openPanel, 1000);
  });
  edge.addEventListener('mouseleave', function(){ clearTimeout(edgeTimer); });
  panel.addEventListener('mouseenter', function(){ clearTimeout(hideTimer); });
  panel.addEventListener('mouseleave', function(){
    if (viewConfig.classList.contains('hidden')) scheduleClose();
  });
  badge.addEventListener('mouseenter', function(){ clearTimeout(hideTimer); });

  // FEATURE-492: the hub badge shows for 3s on load, then fades to fully
  // transparent (still covering the co-shell logo area and still clickable).
  // Hovering restores it; leaving fades it again.
  setTimeout(function(){ if (!panel.classList.contains('open')) badge.classList.add('faded'); }, 3000);
  badge.addEventListener('mouseenter', function(){ badge.classList.remove('faded'); });
  badge.addEventListener('mouseleave', function(){
    // FEATURE-516: never fade the logo bar while the drawer is open.
    if (!panel.classList.contains('open')) badge.classList.add('faded');
  });

  // Load the hub version into the logo badge once.
  function loadHubInfo(){
    if (hubVerEl.textContent) return;
    api('GET', '/api/hub-info', null, function(st, j){
      if (j && j.version) hubVerEl.textContent = ' v' + j.version + (j.build ? ' [BUILD-' + j.build + ']' : '');
      updateBadgeResponsive();
    });
  }
  // FEATURE-487: on narrow screens (viewport narrower than twice the full
  // badge width, i.e. logo + version) hide the version text so the badge
  // shrinks to just the logo and does not crowd the co-shell UI behind it.
  var badgeFullW = 0; // cached full badge width (logo + version)
  function updateBadgeResponsive(){
    if (!badge) return;
    // Measure the full width with the version visible.
    hubVerEl.style.display = '';
    badgeFullW = badge.getBoundingClientRect().width;
    var narrow = window.innerWidth < badgeFullW * 2;
    hubVerEl.style.display = narrow ? 'none' : '';
  }
  window.addEventListener('resize', updateBadgeResponsive);
  // setConn toggles the hub connection indicator triangle: accent colour when
  // connected, grey when the hub API is unreachable.
  function setConn(on){
    connected = on;
    markEl.classList.toggle('off', !on);
    markEl.title = on ? '已连接' : '连接断开 · 点击重连';
  }
  // startPolling begins the periodic agent-list refresh (used on load and on
  // manual reconnect).
  function startPolling(){
    if (pollTimer) return;
    refresh();
    pollTimer = setInterval(refresh, 3000);
  }
  // stopPolling halts the periodic refresh. Called when the hub becomes
  // unreachable so this tab stops competing with other browsers for the
  // single-client connection; a click on the triangle or a page reload resumes.
  function stopPolling(){
    if (pollTimer){ clearInterval(pollTimer); pollTimer = null; }
  }
  function refresh(){
    loadHubInfo();
    api('GET', '/api/agents', null, function(st, j){
      if (st === 0){ setConn(false); stopPolling(); return; } // hub unreachable
      setConn(true);
      agents = (j && j.agents) || [];
      Object.keys(frames).forEach(function(id){
        if (!agents.some(function(a){ return a.id === id; })){ removeFrame(id); }
      });
      if (!current && agents.length) current = agents[0].id;
      if (current && !agents.some(function(a){ return a.id === current; })) current = agents.length ? agents[0].id : null;
      renderList();
      ensureFrame(current);
    });
  }

  function ensureFrame(id){
    if (!id) return;
    if (frames[id]) return;
    var f = document.createElement('iframe');
    f.className = 'frame';
    f.src = '/agent/' + encodeURIComponent(id) + '/';
    f.setAttribute('data-id', id);
    stageEl.appendChild(f);
    frames[id] = f;
    showFrame(id);
  }
  function showFrame(id){
    Object.keys(frames).forEach(function(k){ frames[k].classList.remove('active'); });
    if (frames[id]) frames[id].classList.add('active');
    emptyEl.style.display = agents.length ? 'none' : 'flex';
  }
  function removeFrame(id){
    if (frames[id]){ frames[id].remove(); delete frames[id]; }
  }

  function renderList(){
    listEl.innerHTML = '';
    openCard = null;
    agents.forEach(function(a){
      var on = (a.running || a.connected);
      // FEATURE-499: three-state status light. closed (not running/connected)
      // = grey (no class); open+idle = green; busy (task executing) = red.
      var st = on ? (a.busy ? 'busy' : 'idle') : '';
      var managed = a.type !== 'external';
      // Swipe container: delete button behind the card, revealed by swiping left.
      var wrap = document.createElement('div');
      wrap.className = 'agent-wrap';
      var del = document.createElement('button');
      del.className = 'agent-del';
      del.textContent = '删除';
      del.onclick = function(e){ e.stopPropagation(); closeAgent(a.id); };
      var r = document.createElement('div');
      r.className = 'agent' + (a.id === current ? ' active' : '');
      r.innerHTML = '<span class="st ' + st + '"></span><span class="nm">' + esc(a.name || a.id) + '</span>' +
        (managed ? '<label class="switch" title="' + (on ? '停止' : '启动') + '"><input type="checkbox"' + (on ? ' checked' : '') + '><span class="slider"></span></label>' : '') +
        '<span class="chev" title="修改配置">›</span>';
      // Clicking the card switches the main view to this agent; clicking the
      // trailing chevron opens its read-only detail view.
      r.addEventListener('click', function(e){
        if (e.target.closest('.switch')) return;
        if (e.target.closest('.chev')){ showAgentEdit(a); return; }
        if (r._moved){ r._moved = false; return; } // just finished a swipe drag
        if (r._open){ setSwipe(r, false); return; } // click an open card closes it
        if (openCard && openCard !== r) setSwipe(openCard, false);
        current = a.id; renderList(); ensureFrame(current); showFrame(current); closePanel();
      });
      // Power switch: running -> stop, stopped -> start (revert on start failure).
      // The switch label stops click propagation so toggling it never triggers
      // the card's switch-agent action (which would rebuild the list and drop
      // the change event, leaving the switch visually unchanged).
      var swLabel = r.querySelector('.switch');
      if (swLabel) swLabel.addEventListener('click', function(e){ e.stopPropagation(); });
      var sw = r.querySelector('.switch input');
      if (sw){
        sw.addEventListener('change', function(){
          if (sw.checked) startAgent(a.id, sw); else stopAgent(a.id, sw);
        });
      }
      attachSwipe(r);
      wrap.appendChild(del);
      wrap.appendChild(r);
      listEl.appendChild(wrap);
    });
  }

  // ---- Agent settings view (FEATURE-520) ----
  // showAgentEdit fills and shows the editable settings form for an agent. The
  // field set mirrors the create form; fields that do not apply to the agent's
  // type are disabled (managed: 主机地址; external: Workspace / co-shell /
  // 共享配置 / 补充运行参数). The ID itself is immutable.
  var editID = null; // agent id being edited
  var coShellsCache = null; // latest co_shells scan result
  // wsURLParts splits ws(s)://host:port/ws into its host and port parts.
  function wsURLParts(u){
    var m = /^wss?:\/\/([^\/:]+):(\d+)/.exec(String(u || ''));
    return m ? { host: m[1], port: m[2] } : { host: '', port: '' };
  }
  // setFieldEnabled enables/disables a form control and greys its row out.
  function setFieldEnabled(id, on){
    var el = document.getElementById(id);
    el.closest('.field').classList.toggle('disabled', !on);
    el.disabled = !on;
  }
  // coShellSourceLabel maps a detection source to the label shown in the
  // dropdown, so copies found in different places can be told apart (FIX-521).
  function coShellSourceLabel(src){
    if (src === 'hubdir') return 'hub 同目录';
    if (src === 'cwd') return '当前目录';
    if (src === 'path') return 'PATH';
    return src || '';
  }
  // coShellOptionText renders one candidate as "<path>（<来源>）  (vX.Y.Z)".
  function coShellOptionText(c){
    var label = coShellSourceLabel(c.source);
    return c.path + (label ? '（' + label + '）' : '') + (c.version ? '  (v' + c.version + ')' : '') + (c.ok ? '' : '  [不可执行]');
  }
  // loadCoShellOptions re-scans on every entry into the edit form (FIX-525):
  // the previous implementation short-circuited on the cached list, so a
  // co-shell copied after page load never showed up. /api/co-shell-locations
  // is the light endpoint (executable scan only, no config candidates). When
  // the request fails the previous list is kept, so the dropdown never goes
  // blank.
  function loadCoShellOptions(cb){
    api('GET', '/api/co-shell-locations', null, function(st, j){
      if (st < 400 && j) coShellsCache = j.co_shells || [];
      cb();
    });
  }
  function fillCoShellOptions(current){
    var sel = document.getElementById('d-coshell');
    sel.innerHTML = '';
    var def = document.createElement('option');
    def.value = ''; def.textContent = '默认（沿用 hub 配置）';
    sel.appendChild(def);
    // FEATURE-527: "latest" is resolved on every start (hub dir first, then
    // the highest version across the current dir and PATH).
    var latest = document.createElement('option');
    latest.value = 'latest'; latest.textContent = '使用最新版本（启动时自动搜索）';
    sel.appendChild(latest);
    // FEATURE-527: "latest" is always offered above, so it counts as found
    // and is never re-added as a trailing "current value" entry below.
    var found = (current === 'latest');
    (coShellsCache || []).forEach(function(c){
      var opt = document.createElement('option');
      opt.value = c.path;
      opt.textContent = coShellOptionText(c);
      if (c.path === current) found = true;
      sel.appendChild(opt);
    });
    // An explicitly configured executable that is no longer detected stays
    // selectable, so opening the form never silently resets the value.
    if (current && !found){
      var extra = document.createElement('option');
      extra.value = current; extra.textContent = current + '  （当前值）';
      sel.appendChild(extra);
    }
    sel.value = current || '';
  }
  function checkEditVersion(){
    var el = document.getElementById('d-ver');
    var path = document.getElementById('d-coshell').value;
    if (!path){ el.textContent = ''; el.className = 'hint'; return; }
    // FEATURE-527: "latest" has no path to query — it is resolved at start.
    if (path === 'latest'){
      el.textContent = '启动时自动搜索最高版本 co-shell';
      el.className = 'hint';
      return;
    }
    api('GET', '/api/agent-version?kind=local&path=' + encodeURIComponent(path), null, function(st, j){
      if (j && j.ok){
        el.textContent = 'co-shell v' + j.version + (j.build ? ' [BUILD-' + j.build + ']' : '');
        el.className = 'hint ver-ok';
      } else {
        el.textContent = (j && j.error) || '无法读取版本';
        el.className = 'hint ver-err';
      }
    });
  }
  document.getElementById('d-coshell').onchange = checkEditVersion;
  function showAgentEdit(a){
    editID = a.id;
    var external = a.type === 'external';
    var parts = wsURLParts(a.ws_url);
    document.getElementById('detailTitle').textContent = (a.name || a.id) + ' · 修改';
    document.getElementById('d-id').value = a.id || '';
    document.getElementById('d-name').value = a.name || '';
    document.getElementById('d-type').value = external ? '远程' : '本地';
    document.getElementById('d-ws').value = a.workspace || '';
    document.getElementById('d-host').value = parts.host;
    document.getElementById('d-port').value = external ? parts.port : (a.port || '');
    document.getElementById('d-shared').checked = !!a.use_shared_config;
    document.getElementById('d-extra').value = a.extra_args || '';
    document.getElementById('d-state').textContent = (a.running || a.connected) ? '运行中（修改需重启后生效）' : '已停止';
    document.getElementById('d-msg').textContent = '';
    // Local-only fields are greyed out for a remote agent, and vice versa.
    setFieldEnabled('d-ws', !external);
    setFieldEnabled('d-host', external);
    setFieldEnabled('d-port', true);
    setFieldEnabled('d-coshell', !external);
    setFieldEnabled('d-shared', !external);
    setFieldEnabled('d-extra', !external);
    document.getElementById('d-port-label').textContent = external ? '端口号（与主机地址组成 WS 地址）' : '端口号（可修改，需重启后生效）';
    loadCoShellOptions(function(){ fillCoShellOptions(a.co_shell); checkEditVersion(); });
    showView('detail');
  }
  // saveAgentEdit PUTs the edited fields back to the hub. A running managed
  // agent is not restarted by the hub, so the user is told to restart it.
  function saveAgentEdit(){
    if (!editID) return;
    var a = null;
    for (var i = 0; i < agents.length; i++){ if (agents[i].id === editID){ a = agents[i]; break; } }
    if (!a) return;
    var external = a.type === 'external';
    var port = parseInt(document.getElementById('d-port').value.trim(), 10);
    if (!port || isNaN(port) || port < 1 || port > 65535){
      alert('请填写有效的端口号（1-65535）。');
      return;
    }
    var body = { name: document.getElementById('d-name').value.trim() };
    if (external){
      var host = normalizeHost(document.getElementById('d-host').value.trim());
      if (!host){ alert('请填写主机地址'); return; }
      body.ws_url = 'ws://' + host + ':' + port + '/ws';
    } else {
      var ws = document.getElementById('d-ws').value.trim();
      if (!ws){ alert('请填写 Workspace 路径'); return; }
      body.workspace = ws;
      body.port = port;
      body.use_shared_config = document.getElementById('d-shared').checked;
      body.extra_args = document.getElementById('d-extra').value.trim();
      // Only send co_shell once the executable list has loaded: a slow
      // /api/agent-defaults response must never silently reset the value.
      var editShellSel = document.getElementById('d-coshell');
      if (editShellSel.options.length) body.co_shell = editShellSel.value;
    }
    api('PUT', '/api/agents/' + encodeURIComponent(editID), body, function(st, j){
      var msg = document.getElementById('d-msg');
      if (st >= 400){
        msg.textContent = '';
        alert('保存失败: ' + ((j && j.error) || st));
        return;
      }
      msg.textContent = (a.running || a.connected) ? '已保存，需重启该 agent 后生效。' : '已保存。';
      msg.className = 'hint ver-ok';
      refresh();
    });
  }
  document.getElementById('d-save').onclick = saveAgentEdit;

  // setSwipe opens (true) or closes (false) the delete button behind a card.
  function setSwipe(card, open){
    card._open = open;
    card.style.transition = 'transform .18s ease';
    card.style.transform = open ? 'translateX(-63px)' : 'translateX(0)';
    if (open) openCard = card;
    else if (openCard === card) openCard = null;
  }
  // attachSwipe wires mouse-drag and touch-swipe so a card can be swiped left
  // to reveal its delete button.
  function attachSwipe(card){
    var startX = 0, startY = 0, dx = 0, dragging = false;
    function begin(x, y){ startX = x; startY = y; dx = 0; dragging = true; card._moved = false; card.style.transition = 'none'; }
    function move(x, y){
      if (!dragging) return;
      var mx = x - startX, my = y - startY;
      if (!card._moved && Math.abs(my) > Math.abs(mx) && Math.abs(my) > 8){ dragging = false; return; } // vertical scroll
      if (Math.abs(mx) > 4) card._moved = true;
      dx = Math.max(-63, Math.min(0, (card._open ? -63 : 0) + mx));
      card.style.transform = 'translateX(' + dx + 'px)';
    }
    function end(){
      if (!dragging) return;
      dragging = false;
      setSwipe(card, dx < -32);
    }
    card.addEventListener('mousedown', function(e){ if (e.button === 0) begin(e.clientX, e.clientY); });
    window.addEventListener('mousemove', function(e){ move(e.clientX, e.clientY); });
    window.addEventListener('mouseup', end);
    card.addEventListener('touchstart', function(e){ var t = e.touches[0]; begin(t.clientX, t.clientY); }, {passive:true});
    card.addEventListener('touchmove', function(e){ var t = e.touches[0]; move(t.clientX, t.clientY); }, {passive:true});
    card.addEventListener('touchend', end);
  }

  // Clicking empty space in the list closes any swipe-open card.
  listEl.addEventListener('click', function(e){
    if (!e.target.closest('.agent') && openCard) setSwipe(openCard, false);
  });

  // startAgent launches a managed agent and switches to it; on failure the
  // switch is reverted to the stopped state.
  function startAgent(id, sw){
    api('POST', '/api/agents/' + encodeURIComponent(id) + '/start', null, function(st, j){
      if (st >= 400){ alert('启动失败: ' + (j.error || st)); sw.checked = false; refresh(); return; }
      current = id;
      renderList();
      closePanel();
      var f = frames[id];
      if (f){ f.src = '/agent/' + encodeURIComponent(id) + '/'; }
      else { ensureFrame(id); }
      refresh();
    });
  }
  // stopAgent stops a managed agent; on failure the switch is reverted to on.
  function stopAgent(id, sw){
    api('POST', '/api/agents/' + encodeURIComponent(id) + '/stop', null, function(st, j){
      if (st >= 400){ alert('停止失败: ' + (j.error || st)); sw.checked = true; refresh(); return; }
      refresh();
    });
  }

  function closeAgent(id){
    api('DELETE', '/api/agents/' + encodeURIComponent(id), null, function(st, j){
      if (st >= 400) alert('删除失败: ' + (j.error || st));
      refresh();
    });
  }

  // ---- Config view (inside the left drawer) ----
  // "Agent 管理" opens the drawer (if closed) and switches to the config view.
  document.getElementById('manageBtn').onclick = function(){
    if (!panel.classList.contains('open')) openPanel();
    showView('config');
    // FIX-525: re-run detection on every entry, so a co-shell copied into a
    // scanned directory is selectable without restarting the hub or reloading
    // the page.
    loadDefaults();
  };
  // ---- Settings view (remote-access: TLS/whitelist/access key) ----
  document.getElementById('settingsBtn').onclick = function(){
    if (!panel.classList.contains('open')) openPanel();
    showView('settings');
    loadSettings();
  };
  document.getElementById('settingsBack').onclick = function(){ showView('list'); };
  document.getElementById('settingsClose').onclick = function(){ showView('list'); };
  var sStatus = document.getElementById('s-status');
  var settingsDir = '.'; // directory holding hub-settings.json (from GET /api/settings)
  function loadSettings(){
    api('GET', '/api/settings', null, function(st, j){
      if (st === 401){ sStatus.textContent = '需要访问 KEY 才能查看设置'; return; }
      var s = (j && j.settings) || {};
      if (s.settings_dir) settingsDir = s.settings_dir;
      document.getElementById('s-tls').checked = !!s.tls_enabled;
      document.getElementById('s-cert').value = s.cert_file || '';
      document.getElementById('s-keyfile').value = s.key_file || '';
      document.getElementById('s-whitelist').value = (s.whitelist || []).join(', ');
      document.getElementById('s-accesskey').value = '';
      document.getElementById('s-accesskey').placeholder = s.access_key ? '已设置（留空不修改）' : '未设置';
      document.getElementById('s-reqkey').checked = !!s.require_key;
      sStatus.textContent = '';
    });
  }
  function saveSettings(){
    var body = {
      tls_enabled: document.getElementById('s-tls').checked,
      cert_file: document.getElementById('s-cert').value.trim(),
      key_file: document.getElementById('s-keyfile').value.trim(),
      whitelist: document.getElementById('s-whitelist').value.split(',').map(function(x){ return x.trim(); }).filter(Boolean),
      access_key: document.getElementById('s-accesskey').value,
      require_key: document.getElementById('s-reqkey').checked
    };
    api('PUT', '/api/settings', body, function(st, j){
      if (st >= 400){ sStatus.textContent = '保存失败: ' + ((j && j.error) || st); return; }
      sStatus.textContent = '已保存。HTTPS/白名单/KEY 变更需重启 hub 后完全生效。';
      loadSettings();
    });
  }
  document.getElementById('s-save').onclick = saveSettings;
  document.getElementById('s-gencert').onclick = function(){
    // Generate a self-signed cert: fill the cert/key paths (under the settings
    // dir) into the inputs so the user sees where they will be written, then
    // enable TLS.
    var certPath = settingsDir.replace(/\/$/, '') + '/hub-cert.pem';
    var keyPath = settingsDir.replace(/\/$/, '') + '/hub-key.pem';
    document.getElementById('s-cert').value = certPath;
    document.getElementById('s-keyfile').value = keyPath;
    document.getElementById('s-tls').checked = true;
    sStatus.textContent = '已选择自签名证书。保存并重启 hub 后，证书将生成到上述路径并启用 HTTPS。';
  };
  // Regenerate a random secure access key into the input (saved on 保存设置).
  document.getElementById('s-genkey').onclick = function(){
    var bytes = new Uint8Array(32);
    if (window.crypto && crypto.getRandomValues){ crypto.getRandomValues(bytes); }
    else { for (var i = 0; i < bytes.length; i++) bytes[i] = Math.floor(Math.random() * 256); }
    var hex = '';
    for (var i = 0; i < bytes.length; i++) hex += ('0' + bytes[i].toString(16)).slice(-2);
    var keyInput = document.getElementById('s-accesskey');
    keyInput.type = 'text';
    keyInput.value = hex;
    keyInput.placeholder = '已生成新 KEY，保存后生效';
    sStatus.textContent = '已生成新的访问 KEY（64 位十六进制）。点击保存设置后生效。';
  };
  // FEATURE-499: clicking the scrim (outside the agent list) auto-collapses
  // the drawer, unless it is pinned. The pin still controls auto-collapse on
  // mouseleave; clicking elsewhere dismisses the drawer as before FEATURE-492.
  scrim.onclick = function(){ if (!pinned) closePanel(); };
  // Clicking anywhere outside the drawer (e.g. the iframe stage) also closes it.
  document.addEventListener('click', function(e){
    if (pinned) return;
    if (!panel.classList.contains('open')) return;
    if (panel.contains(e.target) || badge.contains(e.target) || edge.contains(e.target)) return;
    closePanel();
  });
  // FEATURE-515: the empty-state logo opens the Agent management config view.
  // Delegated on the container so it keeps working even if the empty-state
  // markup is re-rendered. stopPropagation is required: otherwise the same
  // click reaches the document-level outside-click handler below, which would
  // close the drawer we just opened.
  emptyEl.addEventListener('click', function(e){
    if (e.target && e.target.id === 'emptyRun') {
      e.stopPropagation();
      document.getElementById('manageBtn').click();
    }
  });

  // ---- Local/remote mode toggle ----
  var mode = 'local';
  var localFields = document.getElementById('localFields');
  var remoteFields = document.getElementById('remoteFields');
  var cfgSubmit = document.getElementById('cfg-submit');
  function updateSubmitLabel(){
    cfgSubmit.textContent = mode === 'local' ? '创建本地 Agent' : '添加远程 Agent';
  }
  document.querySelectorAll('#modeSeg .seg-btn').forEach(function(btn){
    btn.onclick = function(){
      document.querySelectorAll('#modeSeg .seg-btn').forEach(function(b){ b.classList.remove('active'); });
      btn.classList.add('active');
      mode = btn.getAttribute('data-mode');
      localFields.style.display = mode === 'local' ? '' : 'none';
      remoteFields.style.display = mode === 'remote' ? '' : 'none';
      updateSubmitLabel();
    };
  });

  // ---- Load defaults (workspace, ID, co-shell list) ----
  var coshellSel = document.getElementById('m-coshell');
  var mVerEl = document.getElementById('m-ver');
  var eVerEl = document.getElementById('e-ver');
  // loadDefaults re-runs on every entry into the create form (FIX-525) so the
  // co-shell list reflects binaries copied after the page was loaded. Fields
  // the user already filled are not overwritten, and a previously selected
  // executable stays selected as long as it is still detected.
  function loadDefaults(){
    var prevShell = coshellSel.value;
    api('GET', '/api/agent-defaults', null, function(st, j){
      if (st >= 400 || !j) return;
      if (!document.getElementById('m-ws').value) document.getElementById('m-ws').value = j.default_workspace || '';
      if (!document.getElementById('m-id').value) document.getElementById('m-id').value = j.default_id || '';
      if (!document.getElementById('m-port').value && j.recommended_port) document.getElementById('m-port').value = j.recommended_port;
      // co-shell executables.
      coshellSel.innerHTML = '';
      var shells = j.co_shells || [];
      // FEATURE-527: offer "latest" first, then the detected candidates. The
      // default selection stays the first candidate (set below) so creating an
      // agent keeps behaving as before; without any candidate "latest" stays
      // selected (a miss is reported when the agent starts).
      var latestOpt = document.createElement('option');
      latestOpt.value = 'latest';
      latestOpt.textContent = '使用最新版本（启动时自动搜索）';
      coshellSel.appendChild(latestOpt);
      if (!shells.length){
        var noneOpt = document.createElement('option');
        noneOpt.value = '';
        noneOpt.textContent = '未找到 co-shell，请放到 hub 同目录、当前目录或 PATH';
        coshellSel.appendChild(noneOpt);
        coshellSel.value = 'latest';
      } else {
        shells.forEach(function(c){
          var opt = document.createElement('option');
          opt.value = c.path;
          opt.textContent = coShellOptionText(c);
          coshellSel.appendChild(opt);
        });
        if (!prevShell) coshellSel.value = shells[0].path;
      }
      // Restore the previous choice only while it is still detected.
      for (var i = 0; i < coshellSel.options.length; i++){
        if (coshellSel.options[i].value === prevShell){ coshellSel.value = prevShell; break; }
      }
      checkLocalVersion();
    });
  }

  // ---- Version checks ----
  function checkLocalVersion(){
    var path = coshellSel.value;
    if (!path){ mVerEl.textContent = ''; mVerEl.className = 'hint'; return; }
    // FEATURE-527: "latest" is resolved when the agent starts.
    if (path === 'latest'){
      mVerEl.textContent = '启动时自动搜索最高版本 co-shell';
      mVerEl.className = 'hint';
      return;
    }
    api('GET', '/api/agent-version?kind=local&path=' + encodeURIComponent(path), null, function(st, j){
      if (j && j.ok){
        mVerEl.textContent = 'co-shell v' + j.version + (j.build ? ' [BUILD-' + j.build + ']' : '');
        mVerEl.className = 'hint ver-ok';
      } else {
        mVerEl.textContent = (j && j.error) || '无法读取版本';
        mVerEl.className = 'hint ver-err';
      }
    });
  }
  coshellSel.onchange = checkLocalVersion;

  // ---- Remote host + port: auto-recommend a free port from 28256 ----
  var eHostEl = document.getElementById('e-host');
  var ePortEl = document.getElementById('e-port');
  var eIdEl = document.getElementById('e-id');
  // normalizeHost strips any http(s):// or ws(s):// prefix so the user may
  // paste a full URL or a bare host; both work.
  function normalizeHost(h){
    return String(h).replace(/^(https?:\/\/|wss?:\/\/)/i, '').replace(/\/$/, '');
  }
  function remoteURL(){
    var h = normalizeHost(eHostEl.value.trim());
    var p = ePortEl.value.trim();
    if (!h || !p) return '';
    return 'ws://' + h + ':' + p + '/ws';
  }
  function checkRemoteVersion(){
    var url = remoteURL();
    if (!url){ eVerEl.textContent = ''; eVerEl.className = 'hint'; return; }
    api('GET', '/api/agent-version?kind=remote&url=' + encodeURIComponent(url), null, function(st, j){
      if (j && j.ok){
        eVerEl.innerHTML = 'co-shell v' + j.version + (j.build ? ' [BUILD-' + j.build + ']' : '') +
          ' <svg class="ico ico-inline" aria-hidden="true"><use href="#i-check"/></svg>';
        eVerEl.className = 'hint ver-ok';
      } else {
        eVerEl.textContent = (j && j.error) || '无法连接';
        eVerEl.className = 'hint ver-err';
      }
    });
  }
  // When the host is entered, ask the hub for a recommended free port.
  eHostEl.addEventListener('input', function(){
    var h = this.value.trim();
    if (!h){ ePortEl.value=''; eIdEl.value=''; eVerEl.textContent=''; return; }
    api('GET', '/api/remote-defaults?host=' + encodeURIComponent(h), null, function(st, j){
      if (j && j.recommended_port){
        ePortEl.value = j.recommended_port;
        eIdEl.value = h + '-' + j.recommended_port;
      } else {
        ePortEl.value = ''; // none free in the scan window; user must fill
        eIdEl.value = h;
      }
      checkRemoteVersion();
    });
  });
  ePortEl.addEventListener('input', function(){
    var h = eHostEl.value.trim();
    var p = this.value.trim();
    if (h && p) eIdEl.value = h + '-' + p;
    checkRemoteVersion();
  });

  // ---- Create / add agent (bottom submit button, mode-aware) ----
  cfgSubmit.onclick = function(){
    if (mode === 'remote'){ addRemoteAgent(); return; }
    createLocalAgent();
  };
  function createLocalAgent(){
    var ws = document.getElementById('m-ws').value.trim();
    if (!ws){ alert('请填写 Workspace 路径'); return; }
    if (!coshellSel.value){
      alert('未找到可用的 co-shell，无法创建本地 Agent。\n\n请将 co-shell 可执行文件安装到当前目录或加入 PATH，然后重新打开此界面。');
      return;
    }
    var id = document.getElementById('m-id').value.trim() || ws.split(/[\\\/]/).pop();
    var port = parseInt(document.getElementById('m-port').value.trim(), 10);
    if (!port || isNaN(port) || port < 1 || port > 65535){
      alert('请填写有效的端口号（1-65535），或留空由系统自动分配。');
      return;
    }
    api('POST', '/api/agents', {
      id: id,
      name: document.getElementById('m-name').value.trim(),
      workspace: ws,
      co_shell: coshellSel.value,
      use_shared_config: document.getElementById('m-shared').checked,
      extra_args: document.getElementById('m-extra').value.trim(),
      port: port
    }, function(st, j){
      if (st >= 400){
        var msg = (j && j.error) || st;
        if (/port .* in use|already in use/i.test(msg)){
          alert('端口 ' + port + ' 已被占用，无法创建。\n\n请先关闭占用该端口的服务，或改用其他端口（可清空端口号让系统重新自动分配）。');
        } else {
          alert('创建失败: ' + msg);
        }
        refresh();
        return;
      }
      document.getElementById('m-ws').value=''; document.getElementById('m-id').value=''; document.getElementById('m-port').value='';
      var nid = (j && j.id) || id;
      api('POST', '/api/agents/' + encodeURIComponent(nid) + '/start', null, function(st2, j2){
        if (st2 >= 400) alert('创建成功，但自动启动失败: ' + ((j2 && j2.error) || st2));
        current = nid;
        renderList();
        closePanel();
        var f = frames[nid];
        if (f){ f.src = '/agent/' + encodeURIComponent(nid) + '/'; }
        else { ensureFrame(nid); }
        refresh();
      });
    });
  }
  function addRemoteAgent(){
    var host = normalizeHost(eHostEl.value.trim());
    var port = ePortEl.value.trim();
    var ci = host.lastIndexOf(':');
    if (ci > 0 && /^\d+$/.test(host.slice(ci + 1))){
      if (!port) port = host.slice(ci + 1);
      host = host.slice(0, ci);
    }
    if (!host || !port){ alert('请填写主机地址和端口号'); return; }
    var url = 'ws://' + host + ':' + port + '/ws';
    var id = eIdEl.value.trim() || (host + '-' + port);
    api('POST', '/api/agents/external', { id: id, ws_url: url }, function(st, j){
      if (st >= 400) alert('添加失败: ' + (j.error || st));
      else { eHostEl.value=''; ePortEl.value=''; eIdEl.value=''; }
      refresh();
    });
  }

  startPolling();
})();
</script>
</body>
</html>
`

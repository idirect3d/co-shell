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
//     edge (or the badge is clicked). The drawer always floats over the iframe
//     (FEATURE-557): it never pushes the co-shell main view aside, so opening
//     it does not reflow the page behind (wide screens used to lay out
//     side-by-side and shift the iframe right).
//   - A board-enabled agent (one registered on the hub message board) shows a
//     chat-bubble icon right after its name (FEATURE-557) — the same artwork as
//     the co-shell status-bar session bubble (i-sessions); the flag arrives as
//     board_enabled in GET /api/agents.
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
    --edge-w:10px; --panel-w:480px;
    /* Height of the co-shell top bar inside the iframe; the hub drawer starts
       below it so the hub chrome never covers the title bar (FEATURE-515). */
    --topbar-h:44px;
  }
  * { box-sizing:border-box; }
  html,body { height:100%; }
  body { margin:0; font-family:system-ui,-apple-system,sans-serif; background:var(--bg); color:var(--fg); overflow:hidden; }

  /* Full-screen iframe stage. FEATURE-557: the stage is always full-bleed —
     the drawer overlays it and never shifts it, so no left transition. */
  #stage { position:fixed; inset:0; }
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
    /* FEATURE-557: the drawer floats over the iframe at every screen size, so
       the shadow is unconditional (it used to apply on narrow screens only). */
    box-shadow:2px 0 12px rgba(0,0,0,.4);
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
  /* Chat pane (FEATURE-537): two stacked views inside the Chat section —
     level 1 is the conversation list, level 2 is the conversation detail,
     which slides in over the list (WeChat-style). The broadcast conversation
     (key "*") is pinned first. */
  .chat-view { flex:1 1 0; min-height:0; display:flex; flex-direction:column; overflow:hidden; }
  .chat-view.hidden { display:none; }
  .chat-view.slide-in { animation:viewIn .18s ease; }
  .chat-view.slide-out { animation:viewOut .18s ease; }

  /* Level 1: WeChat-style conversation rows (avatar + title + last message). */
  .board-convs { flex:1; overflow-y:auto; padding:4px 6px; }
  .conv-row { display:flex; align-items:flex-start; gap:10px; padding:9px 8px; border-radius:10px; cursor:pointer; }
  .conv-row:hover { background:var(--elev); }
  .conv-avatar {
    flex:none; width:36px; height:36px; border-radius:7px; display:flex; align-items:center;
    justify-content:center; font-size:15px; font-weight:600; color:#0b0e14; background:var(--accent);
  }
  .conv-main { flex:1; min-width:0; }
  .conv-title { display:flex; align-items:center; gap:6px; font-size:13px; font-weight:600; color:var(--fg); }
  .conv-title .n { overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
  .conv-title .cnt { flex:none; margin-left:auto; font-size:10px; font-weight:400; color:var(--fg-faint); }
  .conv-last { margin-top:3px; font-size:11px; color:var(--fg-dim); overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
  .conv-time { flex:none; font-size:10px; color:var(--fg-faint); padding-top:2px; }

  /* Level 2: detail head (back + title), message bubbles and the composer. */
  .chat-head { flex:none; height:40px; display:flex; align-items:center; gap:8px; padding:0 10px; border-bottom:1px solid var(--border); }
  .chat-head .back { flex:none; cursor:pointer; color:var(--fg-dim); font-size:16px; border:none; background:none; padding:0 4px; line-height:1; }
  .chat-head .back:hover { color:var(--fg); }
  .chat-head .title { flex:1; min-width:0; font-size:13px; font-weight:600; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
  .chat-head .sub { flex:none; font-size:10px; color:var(--fg-faint); }
  .board-msgs { flex:1; overflow-y:auto; padding:10px; display:flex; flex-direction:column; gap:10px; }
  .chat-row { display:flex; align-items:flex-start; gap:8px; }
  .chat-row .av {
    flex:none; width:28px; height:28px; border-radius:6px; display:flex; align-items:center;
    justify-content:center; font-size:12px; font-weight:600; color:#0b0e14; background:var(--fg-faint);
  }
  .chat-row .col { min-width:0; max-width:calc(100% - 44px); }
  .chat-row .who { display:flex; align-items:center; gap:6px; font-size:10px; color:var(--fg-dim); margin-bottom:3px; }
  .chat-bubble {
    border:1px solid var(--border); background:var(--elev); border-radius:10px; padding:6px 9px;
    font-size:12px; white-space:pre-wrap; word-break:break-word; color:var(--fg);
  }
  .chat-row.me { flex-direction:row-reverse; }
  .chat-row.me .col { text-align:right; }
  .chat-row.me .who { flex-direction:row-reverse; }
  .chat-row.me .av { background:var(--accent); }
  .chat-row.me .chat-bubble { background:rgba(63,214,239,.20); border-color:rgba(63,214,239,.5); text-align:left; }
  .chat-reply { margin-top:3px; font-size:10px; color:var(--fg-faint); }
  .chat-compose { flex:none; display:flex; gap:6px; padding:8px; border-top:1px solid var(--border); }
  /* FIX-540: 16px is the smallest size at which mobile browsers (iOS Safari,
     some Android browsers) no longer auto-zoom the page when the box is
     focused — sending a message used to magnify the UI on a phone. */
  .chat-compose input {
    flex:1; min-width:0; background:var(--bg); border:1px solid var(--border); border-radius:8px;
    color:var(--fg); font-size:16px; padding:7px 9px; font-family:inherit;
  }
  .chat-compose input:focus { outline:none; border-color:var(--accent); }
  .chat-compose .btn { flex:none; width:auto; }
  /* The generic .btn.primary sets width:100%; keep the composer button narrow. */
  .chat-compose .btn.primary { flex:none; width:auto; }
  .chat-err { flex:none; padding:0 10px 6px; font-size:11px; color:var(--err); }
  .board-badge { border-radius:6px; padding:0 5px; font-size:10px; border:1px solid var(--border); white-space:nowrap; }
  .board-badge.handled { color:var(--ok); border-color:var(--ok); }
  .board-badge.declined { color:var(--err); border-color:var(--err); }
  .board-badge.need_info { color:var(--warn); border-color:var(--warn); }
  .board-badge.waiting { color:var(--fg-dim); }
  #agentPanel .head .close { margin-left:auto; cursor:pointer; color:var(--fg-dim); font-size:16px; padding:2px 6px; }
  #agentPanel .head .close:hover { color:var(--fg); }
  /* FEATURE-492/FEATURE-516: the pin moved from the Agents section head onto
     the hub logo bar (#hubBadge .pin above); it pins the drawer open so that
     mouseleave no longer auto-collapses it. */
  #agentList { flex:1; overflow-y:auto; padding:8px; }
  /* Each list row is a swipe container: a red delete button sits behind the
     card and is revealed by swiping the card left. */
  .agent-wrap { position:relative; overflow:hidden; border-radius:8px; margin-bottom:2px; background:var(--panel); }
  /* FIX-560: keep the button invisible at rest — otherwise the card's
     anti-aliased 8px corners let a red hairline show through — and fade it in
     with the drag (see DELETE_FADE_PX / setDelOpacity in the script). 84px is
     the 63px revealed area plus 20px of slack on the left so the button's own
     left edge can never appear at the end of a swipe; that slack hides under
     the card, hence the equal left padding that keeps the label centred in the
     part of the button the user actually sees. */
  .agent-del {
    position:absolute; top:0; right:1px; bottom:0; width:84px; border:none;
    background:var(--err); color:#fff; font-size:13px; font-weight:600; cursor:pointer;
    padding:0 0 0 20px; opacity:0; transition:opacity .18s ease;
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
  .agent .st.busy { background:var(--err); box-shadow:0 0 5px var(--err); animation:stBreath 1.2s ease-in-out infinite; }
  /* FIX-594: animate opacity only (compositor-friendly). The old keyframes
     animated box-shadow, which repaints on every frame and kept the GPU/
     WindowServer busy the whole time any agent was executing a task. */
  @keyframes stBreath {
    0%,100% { opacity:.5; }
    50% { opacity:1; }
  }
  /* FEATURE-557: the name is wrapped so the board (chat bubble) marker can sit
     right after the text: the wrapper takes the flexible slot, the name
     truncates inside it and the marker stays visible. */
  .agent .nmwrap { flex:1; min-width:0; display:flex; align-items:center; gap:4px; }
  .agent .nm { flex:0 1 auto; min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
  .agent .bmark { flex:none; display:flex; align-items:center; color:var(--accent); }
  .agent .bmark.off { color:var(--fg-faint); }
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
  /* FIX-562: a field whose previous sibling is a .hint gets no vertical gap at
     all (the hint only carries margin-top, the field only margin-bottom), so a
     block that should read as separate from the run parameters above it needs
     an explicit top margin. */
  .field-sep { margin-top:16px; }
  .field label { display:block; font-size:12px; color:var(--fg-dim); margin-bottom:5px; }
  /* FIX-540: editable controls stay at 16px (same reason as .chat-compose
     input) so no hub form ever triggers the mobile focus auto-zoom. */
  .field input, .field select { width:100%; padding:7px 9px; border-radius:6px; border:1px solid var(--border); background:var(--bg); color:var(--fg); font-size:16px; }
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
  .ro-text { width:100%; padding:7px 9px; border-radius:6px; border:1px solid var(--border); background:var(--bg); color:var(--fg); font-size:13px; line-height:1.5; white-space:pre-wrap; word-break:break-word; }
  .icon-btn { width:28px; height:28px; border:none; border-radius:6px; background:transparent; color:var(--fg); font-size:16px; cursor:pointer; }
  .icon-btn:hover { background:var(--elev); }

  /* FEATURE-516: inline SVG icons (sprite <use>). Geometry instead of emoji /
     font glyphs, coloured by currentColor so they follow the surrounding text. */
  .ico { display:inline-block; vertical-align:middle; flex:none; width:14px; height:14px; }
  .icon-sprite { position:absolute; width:0; height:0; overflow:hidden; }
  .ico-inline { width:12px; height:12px; }
  /* FIX-529: the badge triangle matches the co-shell titlebar brand triangle
     (web/static/style.css .brand-mark .ico = 7px), so the hub chrome and the UI
     behind it read as the same size. This rule comes after .ico and therefore
     is the effective size. */
  #hubBadge .mark { width:7px; height:7px; }
  #hubBadge .pin .ico { width:18px; height:18px; }
  #agentPanel .pane-head .chev .ico { width:14px; height:14px; }
  #agentPanel .head .back .ico { width:16px; height:16px; }
  #agentPanel .head .close .ico { width:15px; height:15px; }
  .btn .ico { width:15px; height:15px; margin-right:5px; }


  /* FEATURE-557: there is no wide-screen side-by-side rule any more — the
     drawer overlays the iframe at every width (see #agentPanel above), so
     opening it never squeezes the co-shell main view. */
  /* FEATURE-537: on phone-sized screens the drawer fills the viewport, so the
     Chat list and the Chat detail each take one screen (WeChat mobile style). */
  @media (max-width:640px) {
    #agentPanel { width:100vw; }
    #hubBadge.open { width:100vw; }
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
    <!-- FEATURE-557: chat-bubble marker for a board-enabled agent (one that
         registered on the hub message board); shown after the agent name.
         FEATURE-557 follow-up: same geometry as the co-shell status-bar session
         icon (i-sessions in web/static/index.html) so both pages read alike. -->
    <symbol id="i-board" viewBox="0 0 24 24">
      <path d="M20.4 11.8c0 4-3.8 7.2-8.4 7.2-1 0-2-.16-2.9-.46L4.6 20.4l1.3-3.5c-1.5-1.3-2.4-3.1-2.4-5.1 0-4 3.8-7.2 8.5-7.2s8.4 3.2 8.4 7.2Z"
            fill="none" stroke="currentColor" stroke-width="1.8"
            stroke-linecap="round" stroke-linejoin="round"/>
    </symbol>
  </defs>
</svg>
<div id="stage">
  <div class="empty" id="empty">
    <img class="logo" id="emptyRun" src="/favicon.svg" alt="co-shell-hub" data-i18n-title="emptyCreateTitle" title="创建或添加 Agent">
    <div data-i18n="emptyHint">暂无 Agent。点击上方"运行"箭头，或左上角 co-shell-hub 徽标再点"管理"创建或添加 Agent。</div>
  </div>
</div>

<!-- Hub badge over the co-shell logo area; toggles the agent drawer. -->
<div id="hubBadge" data-i18n-title="badgeTitle" title="co-shell-hub · 点击展开 Agent 列表">
  <svg class="ico mark" aria-hidden="true"><use href="#i-hub-mark"/></svg><span class="name">co-shell-hub</span><span class="ver" id="hubVer"></span><span class="pin" id="panelPin" data-i18n-title="pinTitle" title="钉住（不自动收起）"><svg class="ico" aria-hidden="true"><use href="#i-pin"/></svg></span>
</div>

<!-- Left edge hot-zone (reveals the drawer on hover). -->
<div id="edge"></div>

<!-- Scrim overlay behind the left drawer. -->
<div id="scrim"></div>

<!-- Left drawer (FEATURE-515): two dockable sections, Agents / Chat. -->
<div id="agentPanel">
  <!-- Agents section head (click expands Agents; the pin keeps the drawer open). -->
  <div class="pane-head" id="agentsHead"><span data-i18n="paneAgents">Agents</span><span class="chev"><svg class="ico" aria-hidden="true"><use href="#i-chev"/></svg></span></div>
  <div class="pane-body" id="agentsBody">
  <!-- View 1: agent list. -->
  <div class="view" id="viewList">
    <div id="agentList"></div>
    <div class="foot"><button class="btn primary" id="manageBtn"><svg class="ico" aria-hidden="true"><use href="#i-plus"/></svg><span data-i18n="btnNew">新建</span></button><button class="btn" id="settingsBtn" style="margin-top:16px;width:100%"><svg class="ico" aria-hidden="true"><use href="#i-gear"/></svg><span data-i18n="btnSettings">设置</span></button></div>
  </div>
  <!-- View 2: config (add local/remote + manage list). -->
  <div class="view hidden" id="viewConfig">
    <div class="head"><button class="back" id="configBack" title="返回 Agent 列表" data-i18n-title="backToAgents"><svg class="ico" aria-hidden="true"><use href="#i-back"/></svg></button><span data-i18n="viewNewAgent">新建 Agent</span><span class="close" id="configClose" data-i18n-title="btnCollapse" title="收起"><svg class="ico" aria-hidden="true"><use href="#i-close"/></svg></span></div>
    <div class="config-body">
      <h3 data-i18n="h3AddAgent">添加 Agent</h3>
      <div class="seg" id="modeSeg">
        <button class="seg-btn active" data-mode="local" data-i18n="segLocal">本地</button>
        <button class="seg-btn" data-mode="remote" data-i18n="segRemote">远程</button>
      </div>
      <!-- Local mode: hub launches a co-shell --serve subprocess. -->
      <div id="localFields">
        <div class="field"><label><span class="req">*</span><span data-i18n="lblWorkspace">Workspace 路径</span></label><input id="m-ws" data-i18n-ph="phWorkspace" placeholder="如 ~/.co-shell/agents/agent-1"></div>
        <div class="field"><label data-i18n="lblPortAuto">端口号（自动分配，可修改）</label><input id="m-port" data-i18n-ph="phPortAuto" placeholder="自动分配"></div>
        <div class="field"><label data-i18n="lblIDDefaultWs">ID（默认取 workspace 末段）</label><input id="m-id" data-i18n-ph="phAutoGen" placeholder="自动生成"></div>
        <div class="field"><label data-i18n="lblCoShellBin">co-shell 可执行程序</label><select id="m-coshell"></select></div>
        <div class="field"><div class="switch-row"><span class="switch-label" data-i18n="lblSharedConfig">共享配置</span><label class="switch"><input type="checkbox" id="m-shared"><span class="slider"></span></label></div><div class="hint" id="m-shared-hint" data-i18n="hintSharedConfig">开启：使用 ~/.co-shell/config.json（共享）；关闭：使用 {workspace}/config.json（不存在时先复制 ~/.co-shell/config.json，若共享配置也不存在则创建空文件）。需重启 agent 后生效。</div></div>
        <div class="field"><div class="switch-row"><span class="switch-label" data-i18n="lblYolo">YOLO 模式</span><label class="switch"><input type="checkbox" id="m-yolo"><span class="slider"></span></label></div><div class="hint" id="m-yolo-hint"></div></div>
        <div class="field"><div class="switch-row"><span class="switch-label" data-i18n="lblDownloadEnabled">启用下载功能</span><label class="switch"><input type="checkbox" id="m-download"><span class="slider"></span></label></div><div class="hint" data-i18n="hintDownloadEnabled">开启后该 agent 启动时带上 --download-enabled，允许远程/移动端客户端下载工作区文件（默认关闭）。需重启 agent 后生效。</div></div>
        <div class="field"><label data-i18n="lblExtraArgs">补充运行参数</label><input id="m-extra" data-i18n-ph="phExtraArgs" placeholder="如 --accept-license --serve" value="--accept-license"></div>
        <div class="hint" id="m-ver"></div>
      </div>
      <!-- Remote mode: user supplies a host + port (hub builds the ws URL). -->
      <div id="remoteFields" style="display:none">
        <div class="field"><label><span class="req">*</span><span data-i18n="lblHost">主机地址</span></label><input id="e-host" data-i18n-ph="phHost" placeholder="IP 或主机名，如 192.168.1.5"></div>
        <div class="field"><label><span class="req">*</span><span data-i18n="lblPort">端口号</span></label><input id="e-port" data-i18n-ph="phPortRecommend" placeholder="自动推荐，可修改"></div>
        <div class="field"><label data-i18n="lblIDDefaultHostPort">ID（默认 host-port）</label><input id="e-id" data-i18n-ph="phAutoGen" placeholder="自动生成"></div>
        <div class="field"><label data-i18n="lblAgentDescRemote">Agent描述（由对方设置）</label><input id="e-name" data-i18n-ph="phNotSet" placeholder="（未设置）" readonly></div>
        <div class="hint" data-i18n="hintRemoteDesc">外部 Agent 的 Agent描述由对方在其 co-shell 的「身份与个性」中修改，此处只读。</div>
        <div class="hint" id="e-ver"></div>
      </div>
    </div>
    <div class="foot"><button class="btn primary" id="cfg-submit"><span data-i18n="btnCreateLocal">创建本地 Agent</span></button></div>
  </div>
  <!-- View 3: agent settings — editable (FEATURE-520). The field set mirrors
       the create form; a field that does not apply to the agent's type is
       greyed out (managed: 主机地址; external: Workspace / co-shell /
       共享配置 / 补充运行参数). The ID is immutable. -->
  <div class="view hidden" id="viewDetail">
    <div class="head"><button class="back" id="detailBack" data-i18n-title="backToAgents" title="返回 Agent 列表"><svg class="ico" aria-hidden="true"><use href="#i-back"/></svg></button><span id="detailTitle" data-i18n="titleAgentSettings">Agent 设置</span></div>
    <div class="config-body">
      <div class="field"><label data-i18n="lblIDReadonly">ID（不可修改）</label><input id="d-id" readonly></div>
      <div class="field"><label data-i18n="lblType">类型</label><input id="d-type" readonly></div>
      <div class="field" id="f-ws"><label data-i18n="lblWorkspace">Workspace 路径</label><input id="d-ws" data-i18n-ph="phWorkspace" placeholder="如 ~/.co-shell/agents/agent-1"></div>
      <div class="field" id="f-host"><label data-i18n="lblHost">主机地址</label><input id="d-host" data-i18n-ph="phHost" placeholder="IP 或主机名，如 192.168.1.5"></div>
      <div class="field" id="f-port"><label id="d-port-label" data-i18n="lblPort">端口号</label><input id="d-port" data-i18n-ph="phPortExample" placeholder="如 28256"></div>
      <div class="field" id="f-coshell"><label data-i18n="lblCoShellBin">co-shell 可执行程序</label><select id="d-coshell"></select><div class="hint" id="d-ver"></div></div>
      <div class="field" id="f-shared"><div class="switch-row"><span class="switch-label" data-i18n="lblSharedConfig">共享配置</span><label class="switch"><input type="checkbox" id="d-shared"><span class="slider"></span></label></div><div class="hint" data-i18n="hintSharedConfig">开启：使用 ~/.co-shell/config.json（共享）；关闭：使用 {workspace}/config.json（不存在时先复制 ~/.co-shell/config.json，若共享配置也不存在则创建空文件）。需重启 agent 后生效。</div></div>
      <div class="field" id="f-yolo"><div class="switch-row"><span class="switch-label" data-i18n="lblYolo">YOLO 模式</span><label class="switch"><input type="checkbox" id="d-yolo"><span class="slider"></span></label></div><div class="hint" id="d-yolo-hint"></div></div>
      <div class="field" id="f-download"><div class="switch-row"><span class="switch-label" data-i18n="lblDownloadEnabled">启用下载功能</span><label class="switch"><input type="checkbox" id="d-download"><span class="slider"></span></label></div><div class="hint" data-i18n="hintDownloadEnabled">开启后该 agent 启动时带上 --download-enabled，允许远程/移动端客户端下载工作区文件（默认关闭）。需重启 agent 后生效。</div></div>
      <div class="field" id="f-extra"><label data-i18n="lblExtraArgs">补充运行参数</label><input id="d-extra" data-i18n-ph="phExtraArgsDetail" placeholder="如 --accept-license"></div>
      <div class="hint" id="d-state"></div>
      <!-- FIX-558: the Agent描述 is the last item and read-only: it shows what the
           agent reported when it last registered on the board. -->
      <div class="field field-sep"><label data-i18n="lblAgentDescReported">Agent描述（最近一次注册上报）</label><div class="ro-text" id="d-name" data-i18n="txtNotReported">（未上报）</div><div class="hint" id="d-name-hint" data-i18n="hintAgentDesc">由 agent 在其 co-shell 的「身份与个性」中设置，hub 仅展示其最近一次注册上报的内容。</div></div>
    </div>
    <div class="foot"><button class="btn primary" id="d-save"><span data-i18n="btnSaveChanges">保存修改</span></button><div class="hint" id="d-msg"></div></div>
  </div>
  <!-- View 4: remote-access settings (TLS/whitelist/access key). -->
  <div class="view hidden" id="viewSettings">
    <div class="head"><button class="back" id="settingsBack" data-i18n-title="backToAgents" title="返回 Agent 列表"><svg class="ico" aria-hidden="true"><use href="#i-back"/></svg></button><span data-i18n="btnSettings">设置</span><span class="close" id="settingsClose" data-i18n-title="btnCollapse" title="收起"><svg class="ico" aria-hidden="true"><use href="#i-close"/></svg></span></div>
    <div class="config-body">
      <h3 data-i18n="h3Https">HTTPS 访问</h3>
      <div class="field"><div class="switch-row"><span class="switch-label" data-i18n="lblEnableHttps">启用 HTTPS（配置证书后仅用 https 访问）</span><label class="switch"><input type="checkbox" id="s-tls"><span class="slider"></span></label></div></div>
      <div class="field"><label data-i18n="lblCertPath">证书文件路径（PEM）</label><input id="s-cert" data-i18n-ph="phCertPath" placeholder="如 /path/to/cert.pem，留空自动生成自签名证书"></div>
      <div class="field"><label data-i18n="lblKeyPath">私钥文件路径（PEM）</label><input id="s-keyfile" data-i18n-ph="phKeyPath" placeholder="如 /path/to/key.pem"></div>
      <div class="field"><button class="btn" id="s-gencert" style="width:100%"><span data-i18n="btnGenCert">生成自签名证书</span></button><div class="hint" data-i18n="hintGenCert">生成后需重启 hub 生效。</div></div>
      <h3 data-i18n="h3AccessControl">访问控制</h3>
      <div class="field"><label data-i18n="lblWhitelist">白名单（IP 或 CIDR，逗号分隔，空=仅本机）</label><input id="s-whitelist" data-i18n-ph="phWhitelist" placeholder="如 192.168.1.100,192.168.1.0/24"></div>
      <div class="field"><label data-i18n="lblAccessKey">访问验证 KEY（白名单外主机需在请求头 X-Access-Key 提供）</label><input id="s-accesskey" type="password" data-i18n-ph="phAccessKeyKeep" placeholder="留空表示不修改当前 KEY"><button class="btn" id="s-genkey" style="margin-top:6px;width:100%"><span data-i18n="btnGenKey">重新生成安全 KEY</span></button></div>
      <div class="field"><div class="switch-row"><span class="switch-label" data-i18n="lblAllNeedKey">全部访问都需要 KEY</span><label class="switch"><input type="checkbox" id="s-reqkey"><span class="slider"></span></label></div><div class="hint" data-i18n="hintAllNeedKey">开启后即使 IP 在白名单内也需提供访问 KEY。</div></div>
      <div class="hint" id="s-status"></div>
    </div>
    <div class="foot"><button class="btn primary" id="s-save"><span data-i18n="btnSaveSettings">保存设置</span></button></div>
  </div>
  </div><!-- /#agentsBody -->
  <!-- Chat section (FEATURE-530/FEATURE-537): two levels — the conversation
       list, then the conversation detail sliding in over it. -->
  <div class="pane-head collapsed" id="chatHead"><span data-i18n="paneChats">Agent 聊天室</span><span class="chev"><svg class="ico" aria-hidden="true"><use href="#i-chev"/></svg></span></div>
  <div class="pane-body collapsed" id="chatBody">
    <!-- Level 1: conversation list (WeChat-style rows). -->
    <div class="chat-view" id="chatListView">
      <div class="board-convs" id="boardConvs"></div>
    </div>
    <!-- Level 2: conversation detail (bubbles + composer), slid in over the list. -->
    <div class="chat-view hidden" id="chatDetailView">
      <div class="chat-head">
        <button class="back" id="chatBack" data-i18n-title="backToConversations" title="返回会话列表"><svg class="ico" aria-hidden="true"><use href="#i-back"/></svg></button>
        <span class="title" id="chatTitle" data-i18n="titleConversations">会话</span>
        <span class="sub" id="chatSub"></span>
      </div>
      <div class="board-msgs" id="boardMsgs"></div>
      <div class="chat-err" id="chatErr"></div>
      <form class="chat-compose" id="chatForm">
        <input id="chatInput" type="text" data-i18n-ph="phChatInput" placeholder="输入消息…" autocomplete="off" maxlength="8192">
        <button class="btn primary" type="submit" id="chatSend"><span data-i18n="btnSend">发送</span></button>
      </form>
    </div>
  </div>
</div>
<script>
/* FEATURE-559: the UI language is resolved by the server (--lang > LANG/LC_ALL > zh)
   and substituted for the placeholder below when this page is served. */
var HUB_LANG = "__HUB_LANG__";
</script>
<script>
(function(){
  // ---- FEATURE-559: UI language (zh source, en optional) ----
  // zh is the source language: the markup and the literals below already carry
  // the Chinese text, which doubles as the fallback when a key is missing. EN
  // holds the English strings, keyed by the ids used in data-i18n*/t() calls.
  var LANG = (HUB_LANG === 'en') ? 'en' : 'zh';
  var EN = {
    // --- drawer / page chrome ---
    emptyCreateTitle: 'Create or add an Agent',
    emptyHint: 'No agents yet. Click the "Run" arrow above, or the co-shell-hub badge at the top left and then "Manage", to create or add an agent.',
    badgeTitle: 'co-shell-hub · click to open the agent list',
    pinTitle: 'Pin (no auto-collapse)',
    paneAgents: 'Agents',
    paneChats: 'Chats',
    // --- agents list ---
    btnNew: 'New',
    btnSettings: 'Settings',
    backToAgents: 'Back to the agent list',
    viewNewAgent: 'New Agent',
    btnCollapse: 'Collapse',
    // --- create form ---
    h3AddAgent: 'Add an Agent',
    segLocal: 'Local',
    segRemote: 'Remote',
    lblWorkspace: 'Workspace path',
    phWorkspace: 'e.g. ~/.co-shell/agents/agent-1',
    lblPortAuto: 'Port (auto-assigned, editable)',
    phPortAuto: 'auto-assigned',
    lblIDDefaultWs: 'ID (defaults to the last workspace segment)',
    phAutoGen: 'auto-generated',
    lblCoShellBin: 'co-shell executable',
    lblSharedConfig: 'Shared config',
    hintSharedConfig: 'On: use ~/.co-shell/config.json (shared); Off: use {workspace}/config.json (copied from ~/.co-shell/config.json when absent; an empty file is created when neither exists). Takes effect after the agent restarts.',
    lblYolo: 'YOLO mode',
    lblDownloadEnabled: 'Enable file download',
    hintDownloadEnabled: 'When on, this agent starts with --download-enabled, letting remote/mobile clients download workspace files (off by default). Takes effect after the agent restarts.',
    lblExtraArgs: 'Extra run arguments',
    phExtraArgs: 'e.g. --accept-license --serve',
    lblHost: 'Host address',
    phHost: 'IP or hostname, e.g. 192.168.1.5',
    lblPort: 'Port',
    phPortRecommend: 'auto-suggested, editable',
    lblIDDefaultHostPort: 'ID (defaults to host-port)',
    lblAgentDescRemote: 'Agent description (set by the peer)',
    phNotSet: '(not set)',
    hintRemoteDesc: 'The description of an external agent is edited by that agent in its own co-shell under "Identity & Personality"; read-only here.',
    btnCreateLocal: 'Create a local Agent',
    // --- agent settings ---
    titleAgentSettings: 'Agent settings',
    lblIDReadonly: 'ID (immutable)',
    lblType: 'Type',
    phPortExample: 'e.g. 28256',
    lblAgentDescReported: 'Agent description (last reported on registration)',
    txtNotReported: '(not reported)',
    hintAgentDesc: 'Set by the agent in its own co-shell under "Identity & Personality"; the hub only shows what was reported at the last registration.',
    btnSaveChanges: 'Save changes',
    phExtraArgsDetail: 'e.g. --accept-license',
    // --- drawer settings (TLS / access control) ---
    h3Https: 'HTTPS access',
    lblEnableHttps: 'Enable HTTPS (after certificates are configured, access via https only)',
    lblCertPath: 'Certificate file path (PEM)',
    phCertPath: 'e.g. /path/to/cert.pem; leave empty to generate a self-signed certificate',
    lblKeyPath: 'Private key file path (PEM)',
    phKeyPath: 'e.g. /path/to/key.pem',
    btnGenCert: 'Generate a self-signed certificate',
    hintGenCert: 'Restart the hub for it to take effect.',
    h3AccessControl: 'Access control',
    lblWhitelist: 'Whitelist (IP or CIDR, comma-separated, empty = loopback only)',
    phWhitelist: 'e.g. 192.168.1.100,192.168.1.0/24',
    lblAccessKey: 'Access key (hosts outside the whitelist must send it in the X-Access-Key header)',
    phAccessKeyKeep: 'leave empty to keep the current key',
    btnGenKey: 'Regenerate the access key',
    lblAllNeedKey: 'Require the key for all access',
    hintAllNeedKey: 'When on, the access key is required even from whitelisted IPs.',
    btnSaveSettings: 'Save settings',
    // --- chat pane ---
    backToConversations: 'Back to the conversation list',
    titleConversations: 'Conversation',
    phChatInput: 'Type a message…',
    btnSend: 'Send',
    // --- dynamic (rendered from JS) ---
    titlePinned: 'Pinned (click to unpin)',
    titleUnpinned: 'Pin (no auto-collapse)',
    statusConnected: 'Connected',
    statusDisconnected: 'Disconnected · click to reconnect',
    whoMe: 'Me',
    whoAll: 'All',
    badgeHandled: 'Handled',
    badgeDeclined: 'Declined',
    badgeNeedInfo: 'Need info',
    badgeNoReply: 'No reply',
    badgeWaiting: 'Waiting %ss',
    badgeWindow: 'Window %ss',
    replyTo: 'Reply to %s',
    convAll: 'All (broadcast)',
    emptyConversations: 'No conversations',
    loadingMessages: 'Loading…',
    errLoadMessages: 'Failed to load messages',
    msgCount: '%s total',
    emptyMessages: 'No messages in this conversation',
    errSend: 'Failed to send',
    btnDelete: 'Delete',
    boardOnline: 'Board enabled · online',
    boardOffline: 'Board enabled · offline',
    tipStop: 'Stop',
    tipStart: 'Start',
    tipEditConfig: 'Edit settings',
    yoloOffHint: 'YOLO mode (You Only Live Once): when on, every tool call of this agent is auto-approved instead of asking one by one (equivalent to the --yolo startup flag). Off by default; takes effect after the agent restarts.',
    yoloOnHint: '⚠️ Risk notice: YOLO is on, so this agent no longer asks before each tool call and may delete/overwrite files or run system commands on its own. Use it only in a controlled environment; takes effect after the agent restarts.',
    srcHubDir: 'hub directory',
    srcCwd: 'current directory',
    notExecutable: 'not executable',
    optDefaultHubConfig: 'default (use the hub settings)',
    optLatest: 'use the latest version (scanned at startup)',
    optCurrentSuffix: '%s  (current)',
    hintAutoScan: 'The highest co-shell version is scanned at startup',
    errReadVersion: 'Cannot read the version',
    errCannotConnect: 'Cannot connect',
    optNoCoShell: 'No co-shell found; put it in the hub directory, the current directory, or on PATH',
    detailEditSuffix: '%s · edit',
    typeLocal: 'Local',
    typeRemote: 'Remote',
    stateRunning: 'Running (changes apply after a restart)',
    stateStopped: 'Stopped',
    lblPortWs: 'Port (with the host address it forms the WS URL)',
    lblPortRestart: 'Port (editable; applies after a restart)',
    errReadDesc: 'Cannot read the agent description: %s',
    alertBadPort: 'Enter a valid port (1-65535).',
    alertNoHost: 'Enter the host address',
    alertNoWs: 'Enter the Workspace path',
    alertSaveFailed: 'Save failed: %s',
    msgSaved: 'Saved.',
    alertStartFailed: 'Start failed: %s',
    alertStopFailed: 'Stop failed: %s',
    alertDeleteFailed: 'Delete failed: %s',
    settingsNeedsKey: 'An access key is required to view the settings',
    phKeySet: 'set (leave empty to keep it)',
    phKeyNotSet: 'not set',
    errSaveSettings: 'Save failed: %s',
    statusSettingsSaved: 'Saved. HTTPS/whitelist/key changes take full effect after the hub restarts.',
    statusCertSelected: 'Self-signed certificate selected. After saving and restarting the hub, the certificate is generated at the path above and HTTPS is enabled.',
    phKeyGenerated: 'a new key was generated; save to apply',
    statusKeyGenerated: 'A new access key was generated (64 hex chars). Click Save settings to apply.',
    btnCreateLocalMode: 'Create a local Agent',
    btnAddRemoteMode: 'Add a remote Agent',
    alertNoCoShell: 'No usable co-shell found, so a local Agent cannot be created.\n\nInstall the co-shell executable in the current directory or add it to PATH, then reopen this panel.',
    alertBadPortCreate: 'Enter a valid port (1-65535), or leave it empty to let the system assign one.',
    alertPortUsed: 'Port %s is already in use, so it cannot be created.\n\nStop the service using it or pick another port (clear the port to let the system assign one again).',
    alertCreateFailed: 'Create failed: %s',
    alertCreateStartedFailed: 'Created, but the automatic start failed: %s',
    alertNoHostPort: 'Enter the host address and port',
    alertAddFailed: 'Add failed: %s',
  };
  // t(key, zh) resolves a string for the active language; zh is the fallback.
  function t(key, zh){ return (LANG === 'en' && EN[key] != null) ? EN[key] : zh; }
  // applyI18n translates the static markup (text / title / placeholder). It runs
  // for en only: the markup already carries the zh source text.
  function applyI18n(root){
    if (LANG !== 'en') return;
    root = root || document;
    var i, els, el, v;
    els = root.querySelectorAll('[data-i18n]');
    for (i = 0; i < els.length; i++){
      el = els[i]; v = EN[el.getAttribute('data-i18n')];
      if (v != null) el.textContent = v;
    }
    els = root.querySelectorAll('[data-i18n-title]');
    for (i = 0; i < els.length; i++){
      el = els[i]; v = EN[el.getAttribute('data-i18n-title')];
      if (v != null) el.setAttribute('title', v);
    }
    els = root.querySelectorAll('[data-i18n-ph]');
    for (i = 0; i < els.length; i++){
      el = els[i]; v = EN[el.getAttribute('data-i18n-ph')];
      if (v != null) el.setAttribute('placeholder', v);
    }
  }
  applyI18n(document);
  document.documentElement.lang = LANG;
  var stageEl = document.getElementById('stage');
  var emptyEl = document.getElementById('empty');
  var badge = document.getElementById('hubBadge');
  var edge = document.getElementById('edge');
  var panel = document.getElementById('agentPanel');
  var agentsHead = document.getElementById('agentsHead');
  var agentsBody = document.getElementById('agentsBody');
  var chatHead = document.getElementById('chatHead');
  var chatBody = document.getElementById('chatBody');
  var chatListView = document.getElementById('chatListView');
  var chatDetailView = document.getElementById('chatDetailView');
  var chatTitleEl = document.getElementById('chatTitle');
  var chatSubEl = document.getElementById('chatSub');
  var chatErrEl = document.getElementById('chatErr');
  var chatInputEl = document.getElementById('chatInput');
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
  chatHead.onclick = function(){ setPane('chat'); showChatView('list'); loadBoard(); };

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
    showChatView('list'); // FEATURE-537: the Chat pane reopens on the list too
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
    pinEl.title = pinned ? t('titlePinned', '已钉住（点击取消）') : t('titleUnpinned', '钉住（不自动收起）');
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
    markEl.title = on ? t('statusConnected', '已连接') : t('statusDisconnected', '连接断开 · 点击重连');
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
      // FIX-594: only rebuild the (innerHTML-based) list when something it
      // paints actually changed; the 3s poll used to rebuild it every tick.
      var sig = agentsSignature(agents, current);
      if (sig !== lastAgentsSig){ lastAgentsSig = sig; renderList(); }
      ensureFrame(current);
      // FEATURE-530: keep the Chat pane in sync while it is expanded.
      if (!chatBody.classList.contains('collapsed')) loadBoard();
    });
  }

  // ---- Message board (FEATURE-530 / FEATURE-537) ----
  // The Chat pane has two levels: the conversation list (level 1) and the
  // conversation detail (level 2), which slides in over the list — same
  // behaviour on wide and narrow screens (WeChat style). The broadcast
  // conversation (key "*") is pinned first and shown as 全部（公告）.
  var boardConvsEl = document.getElementById('boardConvs');
  var boardMsgsEl = document.getElementById('boardMsgs');
  var boardConvKey = null;
  var boardConvsCache = [];  // last rendered conversation summaries
  var rosterById = {};       // agent id -> roster entry (display name)
  var lastMsgSig = '';       // signature of the rendered message list

  // showChatView slides the detail in over the list, and the list back in on
  // return. Called on entering a conversation / tapping the back button.
  function showChatView(name){
    var toDetail = name === 'detail';
    var nxt = toDetail ? chatDetailView : chatListView;
    var cur = toDetail ? chatListView : chatDetailView;
    if (!nxt.classList.contains('hidden')) return; // already showing
    if (cur.classList.contains('hidden')){ nxt.classList.remove('hidden'); return; }
    cur.classList.add('slide-out');
    setTimeout(function(){
      cur.classList.remove('slide-out');
      cur.classList.add('hidden');
      nxt.classList.remove('hidden');
      nxt.classList.add('slide-in');
      setTimeout(function(){ nxt.classList.remove('slide-in'); }, 200);
    }, 160);
  }
  // displayName returns an agent's non-blank display name, falling back to its
  // id: a blank or whitespace-only name counts as "not reported yet" (FIX-542).
  function displayName(a){
    var n = String((a && a.name) || '').trim();
    return n || ((a && a.id) || '');
  }
  // agentName maps a sender id to its display name: the hub UI posts as "user"
  // (rendered as 我), "*" is the broadcast target, agents resolve via roster.
  function agentName(id){
    if (id === 'user') return t('whoMe', '我');
    if (id === '*') return t('whoAll', '全部');
    return displayName(rosterById[id]) || id;
  }
  function initialOf(name){
    var s = String(name || '?').trim();
    return s ? s.charAt(0).toUpperCase() : '?';
  }
  var AVATAR_COLORS = ['#3fd6ef', '#7dd3fc', '#a78bfa', '#f472b6', '#fbbf24', '#4ade80', '#60a5fa', '#fb923c'];
  function avatarColor(id){
    var s = String(id), h = 0;
    for (var i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) % 9973;
    return AVATAR_COLORS[h % AVATAR_COLORS.length];
  }
  function setChatErr(msg){ if (chatErrEl) chatErrEl.textContent = msg || ''; }

  // stateBadge renders the reply-state chip of one message. It is named
  // stateBadge (not badge) because "badge" already names the hub logo bar
  // element in this scope.
  function stateBadge(state){
    if (state === 'handled') return '<span class="board-badge handled">' + t('badgeHandled', '已受理') + '</span>';
    if (state === 'declined') return '<span class="board-badge declined">' + t('badgeDeclined', '已拒绝') + '</span>';
    if (state === 'need_info') return '<span class="board-badge need_info">' + t('badgeNeedInfo', '待补充') + '</span>';
    return '';
  }
  function fmtTime(ts){
    if (!ts) return '';
    var d = new Date(ts * 1000), p = function(n){ return (n < 10 ? '0' : '') + n; };
    return p(d.getMonth() + 1) + '-' + p(d.getDate()) + ' ' + p(d.getHours()) + ':' + p(d.getMinutes());
  }
  function convTitle(c){
    if (c.key === '*') return t('convAll', '全部（公告）');
    var names = (c.participants || []).filter(function(p){ return p !== '*'; }).map(agentName);
    return names.length ? names.join(' · ') : c.key;
  }
  function loadBoard(){
    api('GET', '/api/board/conversations', null, function(st, j){
      if (st !== 200 || !j) return;
      rosterById = {};
      (j.roster || []).forEach(function(a){ rosterById[a.id] = a; });
      var convs = j.conversations || [];
      convs.sort(function(a, b){
        if (a.key === '*') return -1;
        if (b.key === '*') return 1;
        return (b.updated_at || 0) - (a.updated_at || 0);
      });
      boardConvsCache = convs;
      if (boardConvKey && !convs.some(function(c){ return c.key === boardConvKey; })){
        boardConvKey = null;      // the open conversation disappeared
        showChatView('list');
      }
      renderConvs(convs);
      if (boardConvKey) loadMessages(boardConvKey);
    });
  }
  // renderConvs draws level 1: avatar + title + last message + time. Tapping a
  // row opens the conversation detail (level 2).
  function renderConvs(convs){
    if (!convs.length){ boardConvsEl.innerHTML = '<div class="hint">' + t('emptyConversations', '暂无会话') + '</div>'; return; }
    boardConvsEl.innerHTML = convs.map(function(c){
      var title = convTitle(c);
      var last = c.last_from ? (agentName(c.last_from) + '：' + (c.last_content || '')) : (c.last_content || '');
      return '<div class="conv-row" data-key="' + esc(c.key) + '">'
        + '<div class="conv-avatar" style="background:' + avatarColor(c.key) + '">' + esc(initialOf(title)) + '</div>'
        + '<div class="conv-main">'
        + '<div class="conv-title"><span class="n">' + esc(title) + '</span><span class="cnt">' + (c.count || 0) + '</span></div>'
        + '<div class="conv-last">' + esc(last) + '</div>'
        + '</div>'
        + '<div class="conv-time">' + esc(fmtTime(c.updated_at)) + '</div>'
        + '</div>';
    }).join('');
    Array.prototype.forEach.call(boardConvsEl.querySelectorAll('.conv-row'), function(el){
      el.onclick = function(){ openConv(el.getAttribute('data-key')); };
    });
  }
  // openConv enters level 2 (the detail view) for the given conversation key.
  function openConv(key){
    boardConvKey = key;
    lastMsgSig = '';
    var c = boardConvsCache.filter(function(x){ return x.key === key; })[0];
    chatTitleEl.textContent = c ? convTitle(c) : key;
    chatSubEl.textContent = '';
    setChatErr('');
    if (chatInputEl) chatInputEl.value = '';
    boardMsgsEl.innerHTML = '<div class="hint">' + t('loadingMessages', '加载中…') + '</div>';
    showChatView('detail');
    loadMessages(key);
  }
  function loadMessages(key){
    api('GET', '/api/board/conversations/' + encodeURIComponent(key) + '/messages?limit=100', null, function(st, j){
      if (st !== 200 || !j){ setChatErr(t('errLoadMessages', '加载消息失败')); return; }
      renderMessages(j.messages || [], j.pending || []);
    });
  }
  // renderMessages draws level 2 bubbles: sender + time + body, with the human
  // user's own posts (from "user") aligned to the right, WeChat style.
  function renderMessages(msgs, pending){
    chatSubEl.textContent = msgs.length ? t('msgCount', '共 %s 条').replace('%s', msgs.length) : '';
    var sig = msgs.map(function(m){ return m.id + '|' + (m.state || '') + '|' + (m.no_reply ? 'n' : ''); }).join(',') + '/' + pending.length;
    if (sig === lastMsgSig) return; // unchanged: keep the DOM and the scroll stable
    lastMsgSig = sig;
    if (!msgs.length){ boardMsgsEl.innerHTML = '<div class="hint">' + t('emptyMessages', '该会话暂无消息') + '</div>'; return; }
    var wait = {};
    pending.forEach(function(a){ wait[a.message_id] = a.deadline; });
    var now = Math.floor(Date.now() / 1000);
    boardMsgsEl.innerHTML = msgs.map(function(m){
      var mine = m.from === 'user';
      var name = agentName(m.from);
      var to = (m.to && m.to.length) ? m.to.map(agentName).join('、') : '';
      var extra = '';
      if (m.state) extra += stateBadge(m.state);
      else if (m.no_reply) extra += '<span class="board-badge waiting">' + t('badgeNoReply', '免回复') + '</span>';
      else if (wait[m.id]) extra += '<span class="board-badge waiting">' + t('badgeWaiting', '等待 %ss').replace('%s', Math.max(0, wait[m.id] - now)) + '</span>';
      else if (m.timeout > 0) extra += '<span class="board-badge waiting">' + t('badgeWindow', '窗口 %ss').replace('%s', m.timeout) + '</span>';
      var foot = '';
      if (to) foot += '<div class="chat-reply">→ ' + esc(to) + '</div>';
      if (m.reply_to) foot += '<div class="chat-reply">' + t('replyTo', '回复 %s').replace('%s', esc(m.reply_to)) + '</div>';
      return '<div class="chat-row' + (mine ? ' me' : '') + '">'
        + '<div class="av" style="background:' + avatarColor(m.from) + '">' + esc(initialOf(name)) + '</div>'
        + '<div class="col"><div class="who"><span>' + esc(name) + '</span>' + extra + '<span>' + esc(fmtTime(m.ts)) + '</span></div>'
        + '<div class="chat-bubble">' + esc(m.content) + '</div>' + foot
        + '</div></div>';
    }).join('');
    boardMsgsEl.scrollTop = boardMsgsEl.scrollHeight;
  }
  // The composer posts as the human user ("user") into the open conversation
  // (FEATURE-537); the conversation key pins the post to the existing thread.
  document.getElementById('chatForm').onsubmit = function(e){
    e.preventDefault();
    var text = (chatInputEl.value || '').trim();
    if (!text || !boardConvKey) return;
    var btn = document.getElementById('chatSend');
    btn.disabled = true;
    setChatErr('');
    api('POST', '/api/board/conversations/' + encodeURIComponent(boardConvKey) + '/messages', { content: text }, function(st, j){
      btn.disabled = false;
      if (st !== 200){ setChatErr((j && j.error) || t('errSend', '发送失败')); return; }
      chatInputEl.value = '';
      lastMsgSig = '';
      loadBoard(); // refresh both the list summary and the open messages
    });
  };
  document.getElementById('chatBack').onclick = function(){ showChatView('list'); };

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

  // FEATURE-547: after the hub switches to an agent — a drawer card click, the
  // power switch starting a stopped agent, or the create-then-start flow — hand
  // the keyboard over to that agent's co-shell so the user needs no extra click,
  // but leave the focus on the main workspace (#stream-wrap). The message input
  // box must NOT be focused: while an interaction (ask_select / tool confirm) is
  // pending the user answers with the option hotkeys, and a focused input box
  // gets in the way of those keys. The frame is same-origin (reverse-proxied
  // under /agent/{id}/), so its document can be reached directly; a frame that
  // is still loading has no document yet, hence the one-shot load listener.
  function focusAgentView(id){
    var f = frames[id];
    if (!f) return;
    function doFocus(){
      try {
        var doc = f.contentDocument;
        if (!doc) return;
        f.contentWindow.focus();
        var view = doc.getElementById('stream-wrap') || doc.body;
        if (view) view.focus();
      } catch (e) { /* frame not ready: keep the hub focus */ }
    }
    doFocus();
    f.addEventListener('load', doFocus, { once: true });
  }

  // FIX-594: lastAgentsSig caches the identity of everything renderList paints
  // (including the active agent), so refresh() can skip the DOM rebuild when a
  // poll returns an unchanged list.
  var lastAgentsSig = '';
  function agentsSignature(list, cur){
    return (list || []).map(function(a){
      return [a.id, a.name, a.connected, a.running, a.busy, a.type, a.board_state].join('\u0001');
    }).join('\u0002') + '\u0003' + (cur || '');
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
      del.textContent = t('btnDelete', '删除');
      del.onclick = function(e){ e.stopPropagation(); closeAgent(a.id); };
      var r = document.createElement('div');
      r.className = 'agent' + (a.id === current ? ' active' : '');
      r._del = del; // FIX-560: the swipe code fades this button in and out
      // FEATURE-557: three-state board marker right after the name (same
      // artwork as the co-shell status-bar session icon): bright while the agent
      // is registered and online, grey while it is registered but offline, and
      // nothing at all while it is not registered (board switch off).
      var bmark = '';
      if (a.board_state === 'online' || a.board_state === 'offline') {
        var off = a.board_state === 'offline';
        bmark = '<span class="bmark' + (off ? ' off' : '') + '" title="' +
          (off ? t('boardOffline', '已启用公告板·离线') : t('boardOnline', '已启用公告板·在线')) +
          '"><svg class="ico ico-inline" aria-hidden="true" focusable="false"><use href="#i-board"/></svg></span>';
      }
      r.innerHTML = '<span class="st ' + st + '"></span><span class="nmwrap"><span class="nm" title="' + esc(a.id || '') + '">' + esc(displayName(a)) + '</span>' + bmark + '</span>' +
        (managed ? '<label class="switch" title="' + (on ? t('tipStop', '停止') : t('tipStart', '启动')) + '"><input type="checkbox"' + (on ? ' checked' : '') + '><span class="slider"></span></label>' : '') +
        '<span class="chev" title="' + t('tipEditConfig', '修改配置') + '">›</span>';
      // Clicking the card switches the main view to this agent; clicking the
      // trailing chevron opens its read-only detail view.
      r.addEventListener('click', function(e){
        if (e.target.closest('.switch')) return;
        if (e.target.closest('.chev')){ showAgentEdit(a); return; }
        if (r._moved){ r._moved = false; return; } // just finished a swipe drag
        if (r._open){ setSwipe(r, false); return; } // click an open card closes it
        if (openCard && openCard !== r) setSwipe(openCard, false);
        current = a.id; renderList(); ensureFrame(current); showFrame(current); closePanel(); focusAgentView(current);
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
  // FEATURE-528: YOLO mode switch. While the switch is off the hint explains
  // what YOLO means; once it is on the hint states the risk instead, so
  // enabling it is always an explicit, informed choice.
  var YOLO_OFF_HINT = t('yoloOffHint', 'YOLO 模式（You Only Live Once）：开启后该 agent 的所有工具调用自动批准，不再逐个询问（等效于启动参数 --yolo）。默认关闭，需重启 agent 后生效。');
  var YOLO_ON_HINT = t('yoloOnHint', '⚠️ 风险提示：已开启 YOLO，该 agent 的工具调用将不再逐个确认，可能自动执行删除/覆盖文件、运行系统命令等高风险操作。请仅在受控环境中使用；需重启 agent 后生效。');
  function syncYoloHint(input, hint){
    if (!input || !hint) return;
    hint.textContent = input.checked ? YOLO_ON_HINT : YOLO_OFF_HINT;
    hint.style.color = input.checked ? '#e8590c' : '';
  }
  function initYoloSwitch(inputID, hintID){
    var input = document.getElementById(inputID);
    var hint = document.getElementById(hintID);
    if (!input || !hint) return;
    input.addEventListener('change', function(){ syncYoloHint(input, hint); });
    syncYoloHint(input, hint);
  }
  // coShellSourceLabel maps a detection source to the label shown in the
  // dropdown, so copies found in different places can be told apart (FIX-521).
  function coShellSourceLabel(src){
    if (src === 'hubdir') return t('srcHubDir', 'hub 同目录');
    if (src === 'cwd') return t('srcCwd', '当前目录');
    if (src === 'path') return 'PATH';
    return src || '';
  }
  // coShellOptionText renders one candidate as "<path>（<来源>）  (vX.Y.Z)".
  function coShellOptionText(c){
    var label = coShellSourceLabel(c.source);
    return c.path + (label ? '（' + label + '）' : '') + (c.version ? '  (v' + c.version + ')' : '') + (c.ok ? '' : '  [' + t('notExecutable', '不可执行') + ']');
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
    def.value = ''; def.textContent = t('optDefaultHubConfig', '默认（沿用 hub 配置）');
    sel.appendChild(def);
    // FEATURE-527: "latest" is resolved on every start (hub dir first, then
    // the highest version across the current dir and PATH).
    var latest = document.createElement('option');
    latest.value = 'latest'; latest.textContent = t('optLatest', '使用最新版本（启动时自动搜索）');
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
      extra.value = current; extra.textContent = t('optCurrentSuffix', '%s  （当前值）').replace('%s', current);
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
      el.textContent = t('hintAutoScan', '启动时自动搜索最高版本 co-shell');
      el.className = 'hint';
      return;
    }
    api('GET', '/api/agent-version?kind=local&path=' + encodeURIComponent(path), null, function(st, j){
      if (j && j.ok){
        el.textContent = 'co-shell v' + j.version + (j.build ? ' [BUILD-' + j.build + ']' : '');
        el.className = 'hint ver-ok';
      } else {
        el.textContent = (j && j.error) || t('errReadVersion', '无法读取版本');
        el.className = 'hint ver-err';
      }
    });
  }
  document.getElementById('d-coshell').onchange = checkEditVersion;
  function showAgentEdit(a){
    editID = a.id;
    var external = a.type === 'external';
    var parts = wsURLParts(a.ws_url);
    document.getElementById('detailTitle').textContent = t('detailEditSuffix', '%s · 修改').replace('%s', a.id || '');
    document.getElementById('d-id').value = a.id || '';
    loadAgentIdentity(a.id, external);
    document.getElementById('d-type').value = external ? t('typeRemote', '远程') : t('typeLocal', '本地');
    document.getElementById('d-ws').value = a.workspace || '';
    document.getElementById('d-host').value = parts.host;
    document.getElementById('d-port').value = external ? parts.port : (a.port || '');
    document.getElementById('d-shared').checked = !!a.use_shared_config;
    document.getElementById('d-extra').value = a.extra_args || '';
    var yoloInput = document.getElementById('d-yolo');
    yoloInput.checked = !!a.yolo;
    syncYoloHint(yoloInput, document.getElementById('d-yolo-hint'));
    document.getElementById('d-download').checked = !!a.download_enabled;
    document.getElementById('d-state').textContent = (a.running || a.connected) ? t('stateRunning', '运行中（修改需重启后生效）') : t('stateStopped', '已停止');
    document.getElementById('d-msg').textContent = '';
    // Local-only fields are greyed out for a remote agent, and vice versa.
    setFieldEnabled('d-ws', !external);
    setFieldEnabled('d-host', external);
    setFieldEnabled('d-port', true);
    setFieldEnabled('d-coshell', !external);
    setFieldEnabled('d-shared', !external);
    setFieldEnabled('d-extra', !external);
    setFieldEnabled('d-yolo', !external);
    setFieldEnabled('d-download', !external);
    document.getElementById('d-port-label').textContent = external ? t('lblPortWs', '端口号（与主机地址组成 WS 地址）') : t('lblPortRestart', '端口号（可修改，需重启后生效）');
    loadCoShellOptions(function(){ fillCoShellOptions(a.co_shell); checkEditVersion(); });
    showView('detail');
  }
  // loadAgentIdentity shows the Agent描述 the agent reported when it last
  // registered on the board (FIX-558). It is read-only: the agent owns the value
  // (it edits it under 「身份与个性」) and the hub never writes it back.
  function loadAgentIdentity(id, external){
    var el = document.getElementById('d-name');
    var hint = document.getElementById('d-name-hint');
    el.textContent = t('txtNotReported', '（未上报）');
    hint.textContent = external
      ? t('hintRemoteDesc', '外部 Agent 的描述由对方在其 co-shell 的「身份与个性」中修改，此处只读。')
      : t('hintAgentDesc', '由 agent 在其 co-shell 的「身份与个性」中设置，hub 仅展示其最近一次注册上报的内容。');
    api('GET', '/api/agents/' + encodeURIComponent(id) + '/identity', null, function(st, j){
      if (editID !== id) return; // the form switched to another agent meanwhile
      if (st !== 200){ hint.textContent = t('errReadDesc', '无法读取 Agent描述: %s').replace('%s', (j && j.error) || st); return; }
      el.textContent = (j && j.agent_description) || t('txtNotReported', '（未上报）');
    });
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
      alert(t('alertBadPort', '请填写有效的端口号（1-65535）。'));
      return;
    }
    var body = {};
    if (external){
      var host = normalizeHost(document.getElementById('d-host').value.trim());
      if (!host){ alert(t('alertNoHost', '请填写主机地址')); return; }
      body.ws_url = 'ws://' + host + ':' + port + '/ws';
    } else {
      var ws = document.getElementById('d-ws').value.trim();
      if (!ws){ alert(t('alertNoWs', '请填写 Workspace 路径')); return; }
      body.workspace = ws;
      body.port = port;
      body.use_shared_config = document.getElementById('d-shared').checked;
      body.extra_args = document.getElementById('d-extra').value.trim();
      body.yolo = document.getElementById('d-yolo').checked;
      body.download_enabled = document.getElementById('d-download').checked;
      // Only send co_shell once the executable list has loaded: a slow
      // /api/agent-defaults response must never silently reset the value.
      var editShellSel = document.getElementById('d-coshell');
      if (editShellSel.options.length) body.co_shell = editShellSel.value;
    }
    api('PUT', '/api/agents/' + encodeURIComponent(editID), body, function(st, j){
      var msg = document.getElementById('d-msg');
      if (st >= 400){
        msg.textContent = '';
        alert(t('alertSaveFailed', '保存失败: %s').replace('%s', (j && j.error) || st));
        return;
      }
      // FIX-558: the Agent描述 is read-only on the hub (the agent owns it), so
      // saving the run parameters is the whole operation.
      msg.textContent = t('msgSaved', '已保存。');
      msg.className = 'hint ver-ok';
      refresh();
    });
  }
  document.getElementById('d-save').onclick = saveAgentEdit;

  // FIX-560: distance the card must travel before the delete button is fully
  // opaque; the button fades in gradually over this drag distance.
  var DELETE_FADE_PX = 16;
  // setDelOpacity drives the delete button's fade: v is the target opacity
  // (0..1), 0 while the card rests and 1 once the swipe passed DELETE_FADE_PX.
  function setDelOpacity(card, v){
    if (!card._del) return;
    card._del.style.opacity = v <= 0 ? '0' : (v >= 1 ? '1' : String(v));
  }
  // setSwipe opens (true) or closes (false) the delete button behind a card.
  function setSwipe(card, open){
    card._open = open;
    if (card._del) card._del.style.transition = 'opacity .18s ease';
    card.style.transition = 'transform .18s ease';
    card.style.transform = open ? 'translateX(-63px)' : 'translateX(0)';
    setDelOpacity(card, open ? 1 : 0);
    if (open) openCard = card;
    else if (openCard === card) openCard = null;
  }
  // attachSwipe wires mouse-drag and touch-swipe so a card can be swiped left
  // to reveal its delete button.
  function attachSwipe(card){
    var startX = 0, startY = 0, dx = 0, dragging = false, listening = false;
    // FIX-597: the window-level drag listeners live only while a mouse drag is
    // in progress. They used to be registered by every attachSwipe() call —
    // that is, twice per agent card on every list rebuild — and never undone,
    // so the window's listener list grew without bound: each later
    // addEventListener scanned it linearly (WebKit's JSEventListener::
    // operator==), and every leaked closure kept the discarded card's DOM
    // alive. A Safari tab sitting on the hub page reached 10-70% CPU and
    // ~1.7GB (measured: 294 registrations in 22s with 21 agents).
    function onMove(e){ move(e.clientX, e.clientY); }
    function listen(){
      if (listening) return;
      listening = true;
      window.addEventListener('mousemove', onMove);
      window.addEventListener('mouseup', end);
    }
    function unlisten(){
      if (!listening) return;
      listening = false;
      window.removeEventListener('mousemove', onMove);
      window.removeEventListener('mouseup', end);
    }
    function begin(x, y){
      startX = x; startY = y; dx = 0; dragging = true; card._moved = false;
      card.style.transition = 'none';
      if (card._del) card._del.style.transition = 'none'; // follow the drag, no lag
    }
    function move(x, y){
      if (!dragging) return;
      var mx = x - startX, my = y - startY;
      if (!card._moved && Math.abs(my) > Math.abs(mx) && Math.abs(my) > 8){ // vertical scroll
        dragging = false;
        unlisten();
        setSwipe(card, !!card._open); // restore the resting/open look, no red left over
        return;
      }
      if (Math.abs(mx) > 4) card._moved = true;
      dx = Math.max(-63, Math.min(0, (card._open ? -63 : 0) + mx));
      card.style.transform = 'translateX(' + dx + 'px)';
      setDelOpacity(card, -dx / DELETE_FADE_PX);
    }
    function end(){
      unlisten();
      if (!dragging) return;
      dragging = false;
      setSwipe(card, dx < -32);
    }
    card.addEventListener('mousedown', function(e){ if (e.button === 0){ begin(e.clientX, e.clientY); listen(); } });
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
      if (st >= 400){ alert(t('alertStartFailed', '启动失败: %s').replace('%s', j.error || st)); sw.checked = false; refresh(); return; }
      current = id;
      renderList();
      closePanel();
      var f = frames[id];
      if (f){ f.src = '/agent/' + encodeURIComponent(id) + '/'; }
      else { ensureFrame(id); }
      focusAgentView(id); // FEATURE-547: a started agent gets the keyboard on its workspace, like a card switch
      refresh();
    });
  }
  // stopAgent stops a managed agent; on failure the switch is reverted to on.
  function stopAgent(id, sw){
    api('POST', '/api/agents/' + encodeURIComponent(id) + '/stop', null, function(st, j){
      if (st >= 400){ alert(t('alertStopFailed', '停止失败: %s').replace('%s', j.error || st)); sw.checked = true; refresh(); return; }
      refresh();
    });
  }

  function closeAgent(id){
    api('DELETE', '/api/agents/' + encodeURIComponent(id), null, function(st, j){
      if (st >= 400) alert(t('alertDeleteFailed', '删除失败: %s').replace('%s', j.error || st));
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
      if (st === 401){ sStatus.textContent = t('settingsNeedsKey', '需要访问 KEY 才能查看设置'); return; }
      var s = (j && j.settings) || {};
      if (s.settings_dir) settingsDir = s.settings_dir;
      document.getElementById('s-tls').checked = !!s.tls_enabled;
      document.getElementById('s-cert').value = s.cert_file || '';
      document.getElementById('s-keyfile').value = s.key_file || '';
      document.getElementById('s-whitelist').value = (s.whitelist || []).join(', ');
      document.getElementById('s-accesskey').value = '';
      document.getElementById('s-accesskey').placeholder = s.access_key ? t('phKeySet', '已设置（留空不修改）') : t('phKeyNotSet', '未设置');
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
      if (st >= 400){ sStatus.textContent = t('errSaveSettings', '保存失败: %s').replace('%s', (j && j.error) || st); return; }
      sStatus.textContent = t('statusSettingsSaved', '已保存。HTTPS/白名单/KEY 变更需重启 hub 后完全生效。');
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
    sStatus.textContent = t('statusCertSelected', '已选择自签名证书。保存并重启 hub 后，证书将生成到上述路径并启用 HTTPS。');
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
    keyInput.placeholder = t('phKeyGenerated', '已生成新 KEY，保存后生效');
    sStatus.textContent = t('statusKeyGenerated', '已生成新的访问 KEY（64 位十六进制）。点击保存设置后生效。');
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
    cfgSubmit.textContent = mode === 'local' ? t('btnCreateLocalMode', '创建本地 Agent') : t('btnAddRemoteMode', '添加远程 Agent');
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
      latestOpt.textContent = t('optLatest', '使用最新版本（启动时自动搜索）');
      coshellSel.appendChild(latestOpt);
      if (!shells.length){
        var noneOpt = document.createElement('option');
        noneOpt.value = '';
        noneOpt.textContent = t('optNoCoShell', '未找到 co-shell，请放到 hub 同目录、当前目录或 PATH');
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
      mVerEl.textContent = t('hintAutoScan', '启动时自动搜索最高版本 co-shell');
      mVerEl.className = 'hint';
      return;
    }
    api('GET', '/api/agent-version?kind=local&path=' + encodeURIComponent(path), null, function(st, j){
      if (j && j.ok){
        mVerEl.textContent = 'co-shell v' + j.version + (j.build ? ' [BUILD-' + j.build + ']' : '');
        mVerEl.className = 'hint ver-ok';
      } else {
        mVerEl.textContent = (j && j.error) || t('errReadVersion', '无法读取版本');
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
        eVerEl.textContent = (j && j.error) || t('errCannotConnect', '无法连接');
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
    if (!ws){ alert(t('alertNoWs', '请填写 Workspace 路径')); return; }
    if (!coshellSel.value){
      alert(t('alertNoCoShell', '未找到可用的 co-shell，无法创建本地 Agent。\n\n请将 co-shell 可执行文件安装到当前目录或加入 PATH，然后重新打开此界面。'));
      return;
    }
    var id = document.getElementById('m-id').value.trim() || ws.split(/[\\\/]/).pop();
    var port = parseInt(document.getElementById('m-port').value.trim(), 10);
    if (!port || isNaN(port) || port < 1 || port > 65535){
      alert(t('alertBadPortCreate', '请填写有效的端口号（1-65535），或留空由系统自动分配。'));
      return;
    }
    api('POST', '/api/agents', {
      id: id,
      workspace: ws,
      co_shell: coshellSel.value,
      use_shared_config: document.getElementById('m-shared').checked,
      extra_args: document.getElementById('m-extra').value.trim(),
      yolo: document.getElementById('m-yolo').checked,
      download_enabled: document.getElementById('m-download').checked,
      port: port
    }, function(st, j){
      if (st >= 400){
        var msg = (j && j.error) || st;
        if (/port .* in use|already in use/i.test(msg)){
          alert(t('alertPortUsed', '端口 %s 已被占用，无法创建。\n\n请先关闭占用该端口的服务，或改用其他端口（可清空端口号让系统重新自动分配）。').replace('%s', port));
        } else {
          alert(t('alertCreateFailed', '创建失败: %s').replace('%s', msg));
        }
        refresh();
        return;
      }
      document.getElementById('m-ws').value=''; document.getElementById('m-id').value=''; document.getElementById('m-port').value='';
      var newYolo = document.getElementById('m-yolo');
      newYolo.checked = false;
      syncYoloHint(newYolo, document.getElementById('m-yolo-hint'));
      document.getElementById('m-download').checked = false;
      var nid = (j && j.id) || id;
      api('POST', '/api/agents/' + encodeURIComponent(nid) + '/start', null, function(st2, j2){
        if (st2 >= 400) alert(t('alertCreateStartedFailed', '创建成功，但自动启动失败: %s').replace('%s', (j2 && j2.error) || st2));
        current = nid;
        renderList();
        closePanel();
        var f = frames[nid];
        if (f){ f.src = '/agent/' + encodeURIComponent(nid) + '/'; }
        else { ensureFrame(nid); }
        focusAgentView(nid); // FEATURE-547: a freshly created agent gets the keyboard on its workspace too
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
    if (!host || !port){ alert(t('alertNoHostPort', '请填写主机地址和端口号')); return; }
    var url = 'ws://' + host + ':' + port + '/ws';
    var id = eIdEl.value.trim() || (host + '-' + port);
    api('POST', '/api/agents/external', { id: id, ws_url: url }, function(st, j){
      if (st >= 400) alert(t('alertAddFailed', '添加失败: %s').replace('%s', j.error || st));
      else { eHostEl.value=''; ePortEl.value=''; eIdEl.value=''; }
      refresh();
    });
  }

  initYoloSwitch('m-yolo', 'm-yolo-hint');
  initYoloSwitch('d-yolo', 'd-yolo-hint');
  startPolling();
})();
</script>
</body>
</html>
`

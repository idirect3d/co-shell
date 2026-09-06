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
<style>
  :root {
    --bg:#0b0e14; --panel:#10141d; --elev:#161b26; --fg:#d5dbe7; --fg-dim:#8b93a5;
    --fg-faint:#5b6373; --accent:#3fd6ef; --accent-dim:rgba(63,214,239,.14);
    --border:#232a3a; --ok:#4ade80; --err:#f87171; --warn:#facc15;
    --edge-w:10px; --panel-w:340px;
  }
  * { box-sizing:border-box; }
  html,body { height:100%; }
  body { margin:0; font-family:system-ui,-apple-system,sans-serif; background:var(--bg); color:var(--fg); overflow:hidden; }

  /* Full-screen iframe stage. */
  #stage { position:fixed; inset:0; transition:left .22s ease; }
  .frame { position:absolute; inset:0; width:100%; height:100%; border:none; background:#fff; display:none; }
  .frame.active { display:block; }
  .empty { position:absolute; inset:0; display:flex; flex-direction:column; align-items:center; justify-content:center; gap:10px; color:var(--fg-dim); text-align:center; padding:20px; }
  .empty .big { font-size:40px; }
  .empty .big.run { cursor:pointer; color:var(--accent); transition:transform .15s ease; }
  .empty .big.run:hover { transform:scale(1.15); }

  /* Hub badge floating over the co-shell logo area (top-left, 44px tall to
     match the co-shell topbar). Clicking it toggles the agent drawer. */
  #hubBadge {
    position:fixed; top:0; left:0; height:44px; padding:0 14px;
    display:flex; align-items:center; gap:8px; cursor:pointer; z-index:30;
    background:var(--panel); color:var(--fg); user-select:none;
    border-right:1px solid var(--border); border-bottom:1px solid var(--border);
    border-bottom-right-radius:8px; font-size:14px; white-space:nowrap;
  }
  #hubBadge .mark { color:var(--accent); font-weight:700; }
  #hubBadge .name { font-weight:600; letter-spacing:.4px; }
  #hubBadge .ver { color:var(--fg-faint); font-size:12px; font-family:ui-monospace,Menlo,monospace; }
  #hubBadge:hover { background:var(--elev); }

  /* Left edge hot-zone that reveals the agent drawer on hover. */
  #edge {
    position:fixed; top:0; left:0; bottom:0; width:var(--edge-w); z-index:20; cursor:pointer;
  }

  /* Left agent drawer. Collapsed by default (translated off-screen left,
     leaving only the edge hot-zone). */
  #agentPanel {
    position:fixed; top:0; left:0; bottom:0; width:var(--panel-w); z-index:25;
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
  #agentPanel .head .close { margin-left:auto; cursor:pointer; color:var(--fg-dim); font-size:16px; padding:2px 6px; }
  #agentPanel .head .close:hover { color:var(--fg); }
  #agentList { flex:1; overflow-y:auto; padding:8px; }
  /* Each list row is a swipe container: a red delete button sits behind the
     card and is revealed by swiping the card left. */
  .agent-wrap { position:relative; overflow:hidden; border-radius:8px; margin-bottom:2px; background:var(--panel); }
  .agent-del {
    position:absolute; top:0; right:0; bottom:0; width:64px; border:none;
    background:var(--err); color:#fff; font-size:13px; font-weight:600; cursor:pointer;
  }
  .agent {
    position:relative; display:flex; align-items:center; gap:8px; padding:8px 10px;
    background:var(--panel); cursor:pointer; font-size:13px; color:var(--fg-dim);
    transition:transform .18s ease;
  }
  .agent:hover { background:var(--elev); color:var(--fg); }
  .agent.active { background:#0e2a33; color:var(--accent); font-weight:600; }
  .agent .st { width:8px; height:8px; border-radius:50%; background:#555; flex:none; }
  .agent .st.on { background:var(--ok); }
  .agent .st.off { background:var(--err); }
  .agent .nm { flex:1; min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
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
  #agentPanel .foot { flex:none; padding:8px; border-top:1px solid var(--border); }
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
  .field .val { width:100%; padding:7px 9px; border-radius:6px; border:1px solid var(--border); background:var(--bg); color:var(--fg); font-size:13px; word-break:break-all; }
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
<div id="stage">
  <div class="empty" id="empty">
    <div class="big run" id="emptyRun" title="创建或添加 Agent">▸</div>
    <div>暂无 Agent。点击上方"运行"箭头，或左上角 co-shell-hub 徽标再点"管理"创建或添加 Agent。</div>
  </div>
</div>

<!-- Hub badge over the co-shell logo area; toggles the agent drawer. -->
<div id="hubBadge" title="co-shell-hub · 点击展开 Agent 列表">
  <span class="mark">▸</span><span class="name">co-shell-hub</span><span class="ver" id="hubVer"></span>
</div>

<!-- Left edge hot-zone (reveals the drawer on hover). -->
<div id="edge"></div>

<!-- Scrim overlay behind the left drawer. -->
<div id="scrim"></div>

<!-- Left agent drawer: two internal views (list / config). -->
<div id="agentPanel">
  <!-- View 1: agent list. -->
  <div class="view" id="viewList">
    <div class="head"><span class="mark">▸</span>Agents<span class="close" id="panelClose" title="收起">«</span></div>
    <div id="agentList"></div>
    <div class="foot"><button class="btn primary" id="manageBtn">＋ 新建</button></div>
  </div>
  <!-- View 2: config (add local/remote + manage list). -->
  <div class="view hidden" id="viewConfig">
    <div class="head"><button class="back" id="configBack" title="返回 Agent 列表">‹</button>新建 Agent<span class="close" id="configClose" title="收起">✕</span></div>
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
        <div class="hint" id="m-ver"></div>
        <button class="btn primary" id="m-create">创建本地 Agent</button>
      </div>
      <!-- Remote mode: user supplies a host + port (hub builds the ws URL). -->
      <div id="remoteFields" style="display:none">
        <div class="field"><label><span class="req">*</span>主机地址</label><input id="e-host" placeholder="IP 或主机名，如 192.168.1.5"></div>
        <div class="field"><label><span class="req">*</span>端口号</label><input id="e-port" placeholder="自动推荐，可修改"></div>
        <div class="field"><label>ID（默认 host-port）</label><input id="e-id" placeholder="自动生成"></div>
        <div class="field"><label>备注</label><input id="e-name" placeholder="可选"></div>
        <div class="hint" id="e-ver"></div>
        <button class="btn primary" id="e-add">添加远程 Agent</button>
      </div>
    </div>
  </div>
  <!-- View 3: read-only agent detail (click an agent card to view). -->
  <div class="view hidden" id="viewDetail">
    <div class="head"><button class="back" id="detailBack" title="返回 Agent 列表">‹</button><span id="detailTitle">Agent 设置</span><span class="close" id="detailClose" title="收起">✕</span></div>
    <div class="config-body">
      <div class="field"><label>ID</label><div class="val" id="d-id"></div></div>
      <div class="field"><label>备注</label><div class="val" id="d-name"></div></div>
      <div class="field"><label>类型</label><div class="val" id="d-type"></div></div>
      <div class="field"><label>Workspace</label><div class="val" id="d-ws"></div></div>
      <div class="field"><label>端口</label><div class="val" id="d-port"></div></div>
      <div class="field"><label>co-shell</label><div class="val" id="d-coshell"></div></div>
      <div class="field"><label>共享配置</label><div class="val" id="d-shared"></div></div>
      <div class="field"><label>状态</label><div class="val" id="d-state"></div></div>
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
  var listEl = document.getElementById('agentList');
  var viewList = document.getElementById('viewList');
  var viewConfig = document.getElementById('viewConfig');
  var viewDetail = document.getElementById('viewDetail');
  var scrim = document.getElementById('scrim');
  var agents = [];
  var current = null;
  var frames = {};
  var hideTimer = null;
  var hubVerEl = document.getElementById('hubVer');
  var openCard = null; // currently swipe-open agent card (delete revealed)

  function esc(s){ return String(s).replace(/[&<>"']/g,function(c){return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c];}); }

  function api(method, url, body, cb){
    var opts = { method: method, headers: {} };
    if (body){ opts.headers['Content-Type']='application/json'; opts.body=JSON.stringify(body); }
    fetch(url, opts).then(function(r){ return r.json().then(function(j){ cb(r.status, j); }); })
      .catch(function(e){ cb(0, { error: String(e) }); });
  }

  // ---- Drawer open/close ----
  function openPanel(){
    clearTimeout(hideTimer);
    panel.classList.add('open');
    scrim.classList.add('show');
    document.body.classList.add('drawer-open');
  }
  function closePanel(){
    panel.classList.remove('open');
    scrim.classList.remove('show');
    document.body.classList.remove('drawer-open');
    showView('list');
  }
  function scheduleClose(){
    clearTimeout(hideTimer);
    hideTimer = setTimeout(closePanel, 600);
  }
  // showView switches between the list, config and detail views with a slide
  // transition: the current view slides out, then the target slides in.
  var viewEls = [viewList, viewConfig, viewDetail];
  function showView(name){
    var target = name === 'list' ? viewList : (name === 'detail' ? viewDetail : viewConfig);
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
  badge.onclick = function(){ panel.classList.contains('open') ? closePanel() : openPanel(); };
  document.getElementById('panelClose').onclick = closePanel;
  // The config view's close/back buttons return to the agent list view.
  document.getElementById('configClose').onclick = function(){ showView('list'); };
  document.getElementById('configBack').onclick = function(){ showView('list'); };
  // The detail view's close/back buttons return to the agent list view.
  document.getElementById('detailBack').onclick = function(){ showView('list'); };
  document.getElementById('detailClose').onclick = function(){ showView('list'); };
  // Hover the left edge to open; leaving the panel schedules a close only in
  // the list view (the config view stays open until closed or backed out).
  edge.addEventListener('mouseenter', openPanel);
  panel.addEventListener('mouseenter', function(){ clearTimeout(hideTimer); });
  panel.addEventListener('mouseleave', function(){
    if (viewConfig.classList.contains('hidden')) scheduleClose();
  });
  badge.addEventListener('mouseenter', function(){ clearTimeout(hideTimer); });

  // Load the hub version into the logo badge once.
  function loadHubInfo(){
    if (hubVerEl.textContent) return;
    api('GET', '/api/hub-info', null, function(st, j){
      if (j && j.version) hubVerEl.textContent = ' v' + j.version + (j.build ? ' [BUILD-' + j.build + ']' : '');
    });
  }
  function refresh(){
    loadHubInfo();
    api('GET', '/api/agents', null, function(st, j){
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
      var st = on ? 'on' : 'off';
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
        (managed ? '<label class="switch" title="' + (on ? '停止' : '启动') + '"><input type="checkbox"' + (on ? ' checked' : '') + '><span class="slider"></span></label>' : '');
      // Clicking the card opens its read-only detail view (unless swiping or
      // toggling power).
      r.addEventListener('click', function(e){
        if (e.target.closest('.switch')) return;
        if (r._moved){ r._moved = false; return; } // just finished a swipe drag
        if (r._open){ setSwipe(r, false); return; } // click an open card closes it
        if (openCard && openCard !== r) setSwipe(openCard, false);
        showAgentDetail(a);
      });
      // Power switch: running -> stop, stopped -> start (revert on start failure).
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

  // showAgentDetail fills and shows the read-only detail view for an agent.
  function showAgentDetail(a){
    document.getElementById('detailTitle').textContent = (a.name || a.id) + ' · 设置';
    document.getElementById('d-id').textContent = a.id || '-';
    document.getElementById('d-name').textContent = a.name || '-';
    document.getElementById('d-type').textContent = a.type === 'external' ? '远程' : '本地';
    document.getElementById('d-ws').textContent = a.workspace || (a.ws_url || '-');
    document.getElementById('d-port').textContent = a.type === 'external' ? (a.ws_url || '-') : (a.port || '-');
    document.getElementById('d-coshell').textContent = a.co_shell || (a.type === 'external' ? '—' : '默认');
    document.getElementById('d-shared').textContent = a.use_shared_config ? '开启（~/.co-shell/config.json）' : '关闭（{workspace}/config.json）';
    document.getElementById('d-state').textContent = (a.running || a.connected) ? '运行中' : '已停止';
    showView('detail');
  }

  // setSwipe opens (true) or closes (false) the delete button behind a card.
  function setSwipe(card, open){
    card._open = open;
    card.style.transition = 'transform .18s ease';
    card.style.transform = open ? 'translateX(-64px)' : 'translateX(0)';
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
      dx = Math.max(-64, Math.min(0, (card._open ? -64 : 0) + mx));
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
    if (!defaultsLoaded) loadDefaults();
  };
  // Clicking the scrim closes the whole drawer.
  scrim.onclick = closePanel;
  // The empty-state "run" arrow opens the Agent management config view.
  document.getElementById('emptyRun').onclick = function(){ document.getElementById('manageBtn').click(); };

  // ---- Local/remote mode toggle ----
  var mode = 'local';
  var localFields = document.getElementById('localFields');
  var remoteFields = document.getElementById('remoteFields');
  document.querySelectorAll('#modeSeg .seg-btn').forEach(function(btn){
    btn.onclick = function(){
      document.querySelectorAll('#modeSeg .seg-btn').forEach(function(b){ b.classList.remove('active'); });
      btn.classList.add('active');
      mode = btn.getAttribute('data-mode');
      localFields.style.display = mode === 'local' ? '' : 'none';
      remoteFields.style.display = mode === 'remote' ? '' : 'none';
    };
  });

  // ---- Load defaults (workspace, ID, co-shell list) ----
  var coshellSel = document.getElementById('m-coshell');
  var mVerEl = document.getElementById('m-ver');
  var eVerEl = document.getElementById('e-ver');
  var defaultsLoaded = false;
  function loadDefaults(){
    api('GET', '/api/agent-defaults', null, function(st, j){
      if (st >= 400 || !j) return;
      if (!document.getElementById('m-ws').value) document.getElementById('m-ws').value = j.default_workspace || '';
      if (!document.getElementById('m-id').value) document.getElementById('m-id').value = j.default_id || '';
      if (!document.getElementById('m-port').value && j.recommended_port) document.getElementById('m-port').value = j.recommended_port;
      // co-shell executables.
      coshellSel.innerHTML = '';
      var shells = j.co_shells || [];
      if (!shells.length){
        coshellSel.innerHTML = '<option value="">未找到 co-shell，请先安装到当前目录或 PATH</option>';
      } else {
        shells.forEach(function(c){
          var opt = document.createElement('option');
          opt.value = c.path;
          opt.textContent = c.path + (c.version ? '  (v' + c.version + ')' : '') + (c.ok ? '' : '  [不可执行]');
          coshellSel.appendChild(opt);
        });
      }
      defaultsLoaded = true;
      checkLocalVersion();
    });
  }

  // ---- Version checks ----
  function checkLocalVersion(){
    var path = coshellSel.value;
    if (!path){ mVerEl.textContent = ''; mVerEl.className = 'hint'; return; }
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
        eVerEl.textContent = 'co-shell v' + j.version + (j.build ? ' [BUILD-' + j.build + ']' : '') + ' ✓';
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

  // ---- Create local agent ----
  document.getElementById('m-create').onclick = function(){
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
      // Created: clear the form, then auto-start the new agent and show it.
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
  };
  // ---- Add remote agent (hub builds the ws://host:port/ws URL) ----
  document.getElementById('e-add').onclick = function(){
    var host = normalizeHost(eHostEl.value.trim());
    var port = ePortEl.value.trim();
    // If the host field itself carries a port (e.g. http://host:28256), split it.
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
  };

  refresh();
  setInterval(refresh, 3000);
})();
</script>
</body>
</html>
`

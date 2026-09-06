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
  .agent {
    display:flex; align-items:center; gap:8px; padding:8px 10px; border-radius:8px;
    cursor:pointer; font-size:13px; color:var(--fg-dim); margin-bottom:2px;
  }
  .agent:hover { background:var(--elev); color:var(--fg); }
  .agent.active { background:var(--accent-dim); color:var(--accent); font-weight:600; }
  .agent .st { width:8px; height:8px; border-radius:50%; background:#555; flex:none; }
  .agent .st.on { background:var(--ok); }
  .agent .st.off { background:var(--err); }
  .agent .nm { flex:1; min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
  .agent .x { opacity:.5; font-size:12px; padding:0 2px; }
  .agent .x:hover { opacity:1; }
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
  #agentPanel .head .back { cursor:pointer; color:var(--fg-dim); font-size:18px; padding:0 4px; border:none; background:none; line-height:1; }
  #agentPanel .head .back:hover { color:var(--fg); }
  #agentPanel .config-body { flex:1; overflow-y:auto; padding:0 14px 14px; }
  #agentPanel h3 { font-size:13px; margin:16px 0 6px; color:var(--accent); }
  .field { margin-bottom:8px; }
  .field label { display:block; font-size:12px; color:var(--fg-dim); margin-bottom:3px; }
  .field input, .field select { width:100%; padding:6px 8px; border-radius:6px; border:1px solid var(--border); background:var(--bg); color:var(--fg); font-size:13px; }
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
    <div class="big">▸</div>
    <div>暂无 Agent。点击左上角 co-shell-hub 徽标，再点"管理"创建或添加 Agent。</div>
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
    <div class="foot"><button class="btn primary" id="manageBtn">⚙ Agent 管理</button></div>
  </div>
  <!-- View 2: config (add local/remote + manage list). -->
  <div class="view hidden" id="viewConfig">
    <div class="head"><button class="back" id="configBack" title="返回 Agent 列表">‹</button>Agent 管理<span class="close" id="configClose" title="收起">✕</span></div>
    <div class="config-body">
      <h3>添加 Agent</h3>
      <div class="seg" id="modeSeg">
        <button class="seg-btn active" data-mode="local">本地</button>
        <button class="seg-btn" data-mode="remote">远程</button>
      </div>
      <!-- Local mode: hub launches a co-shell --serve subprocess. -->
      <div id="localFields">
        <div class="field"><label><span class="req">*</span>Workspace 路径</label><input id="m-ws" placeholder="如 ~/.co-shell/agents/agent-1"></div>
        <div class="field"><label>ID（默认取 workspace 末段）</label><input id="m-id" placeholder="自动生成"></div>
        <div class="field"><label>备注</label><input id="m-name" placeholder="可选"></div>
        <div class="field"><label>co-shell 可执行程序</label><select id="m-coshell"></select></div>
        <div class="field"><label>config.json（可选，留空由 co-shell 决定）</label><select id="m-config"><option value="">（不指定）</option></select></div>
        <div class="field"><div class="check"><input type="checkbox" id="m-cfg"><label for="m-cfg">创建空 config.json</label></div></div>
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
      <h3>Agent 列表</h3>
      <div id="agent-list"></div>
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
  var scrim = document.getElementById('scrim');
  var mgrListEl = document.getElementById('agent-list');
  var agents = [];
  var current = null;
  var frames = {};
  var hideTimer = null;

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
  // showView switches between the list and config views inside the drawer.
  function showView(name){
    var showList = name === 'list';
    viewList.classList.toggle('hidden', !showList);
    viewConfig.classList.toggle('hidden', showList);
  }
  badge.onclick = function(){ panel.classList.contains('open') ? closePanel() : openPanel(); };
  document.getElementById('panelClose').onclick = closePanel;
  document.getElementById('configClose').onclick = closePanel;
  // The config view's back button returns to the agent list view.
  document.getElementById('configBack').onclick = function(){ showView('list'); };
  // Hover the left edge to open; leaving the panel schedules a close only in
  // the list view (the config view stays open until closed or backed out).
  edge.addEventListener('mouseenter', openPanel);
  panel.addEventListener('mouseenter', function(){ clearTimeout(hideTimer); });
  panel.addEventListener('mouseleave', function(){
    if (viewConfig.classList.contains('hidden')) scheduleClose();
  });
  badge.addEventListener('mouseenter', function(){ clearTimeout(hideTimer); });

  function refresh(){
    api('GET', '/api/agents', null, function(st, j){
      agents = (j && j.agents) || [];
      Object.keys(frames).forEach(function(id){
        if (!agents.some(function(a){ return a.id === id; })){ removeFrame(id); }
      });
      if (!current && agents.length) current = agents[0].id;
      if (current && !agents.some(function(a){ return a.id === current; })) current = agents.length ? agents[0].id : null;
      renderList();
      renderMgrList();
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
    agents.forEach(function(a){
      var r = document.createElement('div');
      r.className = 'agent' + (a.id === current ? ' active' : '');
      var st = (a.running || a.connected) ? 'on' : 'off';
      r.innerHTML = '<span class="st ' + st + '"></span><span class="nm">' + esc(a.name || a.id) + '</span><span class="x" title="关闭">✕</span>';
      r.onclick = function(e){
        if (e.target.classList.contains('x')){ closeAgent(a.id); return; }
        current = a.id; renderList(); ensureFrame(current); closePanel();
      };
      listEl.appendChild(r);
    });
  }

  function closeAgent(id){
    if (!confirm('关闭 agent ' + id + '？')) return;
    api('DELETE', '/api/agents/' + encodeURIComponent(id), null, function(st, j){
      if (st >= 400) alert('删除失败: ' + (j.error || st));
      refresh();
    });
  }

  function renderMgrList(){
    mgrListEl.innerHTML = '';
    agents.forEach(function(a){
      var typeBadge = a.type === 'external' ? 'external' : 'managed';
      var typeLabel = a.type === 'external' ? '远程' : '本地';
      var stateBadge = (a.running || a.connected) ? 'on' : 'off';
      var stateLabel = (a.running || a.connected) ? '运行中' : '已停止';
      var meta = a.type === 'external' ? ('WS: ' + (a.ws_url || '')) : ('WS: ' + (a.workspace || '') + ' · 端口 ' + (a.port || '-'));
      if (a.version) meta += ' · co-shell v' + a.version + (a.build ? ' [BUILD-' + a.build + ']' : '');
      var row = document.createElement('div');
      row.className = 'agent-row';
      row.innerHTML =
        '<div class="top"><span class="name">' + esc(a.name || a.id) + '</span>' +
        '<span class="badge ' + typeBadge + '">' + typeLabel + '</span>' +
        '<span class="badge ' + stateBadge + '">' + stateLabel + '</span></div>' +
        '<div class="meta">' + esc(meta) + '</div>' +
        '<div class="actions">' + actionButtons(a) + '</div>';
      mgrListEl.appendChild(row);
    });
    bindActions();
  }
  function actionButtons(a){
    var s = '';
    if (a.type === 'managed'){
      if (a.running){ s += '<button class="btn stop" data-act="stop" data-id="' + esc(a.id) + '">停止</button>'; }
      else { s += '<button class="btn ok" data-act="start" data-id="' + esc(a.id) + '">启动</button>'; }
    }
    s += '<button class="btn danger" data-act="del" data-id="' + esc(a.id) + '">删除</button>';
    return s;
  }
  function bindActions(){
    mgrListEl.querySelectorAll('button[data-act]').forEach(function(btn){
      btn.onclick = function(){
        var act = btn.getAttribute('data-act');
        var id = btn.getAttribute('data-id');
        if (act === 'start') api('POST', '/api/agents/' + encodeURIComponent(id) + '/start', null, function(st, j){
          if (st >= 400) alert('启动失败: ' + (j.error || st)); refresh();
        });
        else if (act === 'stop') api('POST', '/api/agents/' + encodeURIComponent(id) + '/stop', null, function(st, j){
          if (st >= 400) alert('停止失败: ' + (j.error || st)); refresh();
        });
        else if (act === 'del'){
          if (confirm('删除 agent ' + id + '？')) api('DELETE', '/api/agents/' + encodeURIComponent(id), null, function(st, j){
            if (st >= 400) alert('删除失败: ' + (j.error || st)); refresh();
          });
        }
      };
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

  // ---- Load defaults (workspace, ID, co-shell list, config candidates) ----
  var coshellSel = document.getElementById('m-coshell');
  var configSel = document.getElementById('m-config');
  var mVerEl = document.getElementById('m-ver');
  var eVerEl = document.getElementById('e-ver');
  var defaultsLoaded = false;
  function loadDefaults(){
    api('GET', '/api/agent-defaults', null, function(st, j){
      if (st >= 400 || !j) return;
      if (!document.getElementById('m-ws').value) document.getElementById('m-ws').value = j.default_workspace || '';
      if (!document.getElementById('m-id').value) document.getElementById('m-id').value = j.default_id || '';
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
      // config candidates.
      configSel.innerHTML = '<option value="">（不指定）</option>';
      (j.config_candidates || []).forEach(function(c){
        var opt = document.createElement('option');
        opt.value = c.path;
        opt.textContent = c.path + (c.note ? '  (' + c.note + ')' : '');
        configSel.appendChild(opt);
      });
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
  function remoteURL(){
    var h = eHostEl.value.trim();
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
    var id = document.getElementById('m-id').value.trim() || ws.split(/[\\\/]/).pop();
    api('POST', '/api/agents', {
      id: id,
      name: document.getElementById('m-name').value.trim(),
      workspace: ws,
      co_shell: coshellSel.value,
      config_path: configSel.value,
      create_config: document.getElementById('m-cfg').checked
    }, function(st, j){
      if (st >= 400) alert('创建失败: ' + (j.error || st));
      else { document.getElementById('m-ws').value=''; document.getElementById('m-id').value=''; }
      refresh();
    });
  };
  // ---- Add remote agent (hub builds the ws://host:port/ws URL) ----
  document.getElementById('e-add').onclick = function(){
    var host = eHostEl.value.trim();
    var port = ePortEl.value.trim();
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

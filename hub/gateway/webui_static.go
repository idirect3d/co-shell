package gateway

import "errors"

// errClosed is returned when serving on a closed listener.
var errClosed = errors.New("use of closed network connection")

// webIndexHTML is the embedded hub shell page (FEATURE-484). It is a multi-page
// iframe shell: each registered agent's full co-shell Web UI is embedded in its
// own <iframe> loaded from /agent/{id}/ (reverse-proxied by the hub). The shell
// only maintains the agent switch bar, a management drawer, and mobile
// responsiveness — it does not re-implement the co-shell UI.
const webIndexHTML = `<!DOCTYPE html>
<html lang="zh">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>co-shell-hub</title>
<style>
  :root { --bg:#1e1e2e; --panel:#27273a; --panel2:#313244; --fg:#cdd6f4; --muted:#888; --accent:#89b4fa; --ok:#a6e3a1; --err:#f38ba8; --warn:#f9e2af; --border:#3a3a4d; }
  * { box-sizing:border-box; }
  html,body { height:100%; }
  body { margin:0; font-family:system-ui,-apple-system,sans-serif; background:var(--bg); color:var(--fg); display:flex; flex-direction:column; overflow:hidden; }
  header { flex:none; display:flex; align-items:center; gap:8px; padding:0 10px; height:46px; background:var(--panel); border-bottom:1px solid var(--border); }
  .brand { display:flex; align-items:center; gap:8px; font-weight:700; font-size:15px; white-space:nowrap; }
  .brand .dot { width:10px; height:10px; border-radius:50%; background:var(--accent); }
  #tabs { display:flex; gap:4px; overflow-x:auto; flex:1; min-width:0; padding:0 4px; scrollbar-width:none; }
  #tabs::-webkit-scrollbar { display:none; }
  .tab { flex:none; display:flex; align-items:center; gap:6px; padding:5px 12px; border-radius:8px; cursor:pointer; font-size:13px; color:var(--muted); border:1px solid transparent; white-space:nowrap; }
  .tab:hover { background:var(--panel2); color:var(--fg); }
  .tab.active { background:var(--accent); color:#111; font-weight:600; }
  .tab .st { width:7px; height:7px; border-radius:50%; background:#666; flex:none; }
  .tab .st.on { background:var(--ok); }
  .tab .st.off { background:var(--err); }
  .tab .x { opacity:.6; font-size:12px; padding:0 2px; }
  .tab .x:hover { opacity:1; }
  .icon-btn { flex:none; width:32px; height:32px; border:none; border-radius:8px; background:transparent; color:var(--fg); font-size:18px; cursor:pointer; display:flex; align-items:center; justify-content:center; }
  .icon-btn:hover { background:var(--panel2); }
  #stage { flex:1; position:relative; min-height:0; }
  .frame { position:absolute; inset:0; width:100%; height:100%; border:none; background:#fff; display:none; }
  .frame.active { display:block; }
  .empty { position:absolute; inset:0; display:flex; flex-direction:column; align-items:center; justify-content:center; gap:10px; color:var(--muted); text-align:center; padding:20px; }
  .empty .big { font-size:40px; }
  /* Management drawer */
  #drawer { position:fixed; top:0; right:0; bottom:0; width:340px; max-width:90vw; background:var(--panel); border-left:1px solid var(--border); transform:translateX(100%); transition:transform .2s ease; z-index:50; overflow-y:auto; padding:14px; }
  #drawer.open { transform:translateX(0); }
  #scrim { position:fixed; inset:0; background:rgba(0,0,0,.4); z-index:40; display:none; }
  #scrim.show { display:block; }
  #drawer h2 { font-size:15px; margin:0 0 12px; display:flex; align-items:center; justify-content:space-between; }
  #drawer h3 { font-size:13px; margin:16px 0 6px; color:var(--accent); }
  .field { margin-bottom:8px; }
  .field label { display:block; font-size:12px; color:var(--muted); margin-bottom:3px; }
  .field input { width:100%; padding:6px 8px; border-radius:6px; border:1px solid var(--border); background:var(--bg); color:var(--fg); font-size:13px; }
  .field .check { display:flex; align-items:center; gap:6px; }
  .field .check input { width:auto; }
  .btn { padding:6px 12px; border-radius:6px; border:none; cursor:pointer; font-size:12px; font-weight:600; }
  .btn.primary { background:var(--accent); color:#111; }
  .btn.ok { background:var(--ok); color:#111; }
  .btn.stop { background:var(--warn); color:#111; }
  .btn.danger { background:var(--err); color:#111; }
  .btn:disabled { opacity:.5; cursor:not-allowed; }
  .agent-row { border:1px solid var(--border); border-radius:8px; padding:8px; margin-bottom:8px; background:var(--bg); }
  .agent-row .top { display:flex; align-items:center; gap:6px; }
  .agent-row .name { font-weight:600; font-size:13px; flex:1; min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
  .agent-row .meta { font-size:11px; color:var(--muted); margin:4px 0; word-break:break-all; }
  .badge { font-size:10px; padding:1px 6px; border-radius:8px; flex:none; }
  .badge.managed { background:var(--panel2); color:var(--accent); }
  .badge.external { background:var(--panel2); color:var(--warn); }
  .badge.on { background:#1e3a2a; color:var(--ok); }
  .badge.off { background:#3a1e1e; color:var(--err); }
  .agent-row .actions { display:flex; gap:6px; margin-top:6px; }
  .hint { font-size:11px; color:var(--muted); margin-top:4px; }
  /* Mobile: collapse tabs into a horizontal scroll strip under the header */
  @media (max-width:640px) {
    header { flex-wrap:wrap; height:auto; padding:6px 8px; }
    .brand { font-size:14px; }
    #tabs { order:3; width:100%; flex:none; padding-top:4px; }
    .tab { font-size:12px; padding:4px 10px; }
  }
</style>
</head>
<body>
<header>
  <div class="brand"><span class="dot"></span>co-shell-hub</div>
  <div id="tabs"></div>
  <button class="icon-btn" id="manageBtn" title="Agent 管理">⚙</button>
</header>
<div id="stage">
  <div class="empty" id="empty">
    <div class="big">▸</div>
    <div>暂无 Agent。点击右上角 ⚙ 创建或添加 Agent。</div>
  </div>
</div>
<div id="scrim"></div>
<div id="drawer">
  <h2>Agent 管理 <button class="icon-btn" id="drawerClose" title="关闭">✕</button></h2>
  <h3>创建受控 Agent</h3>
  <div class="field"><label>ID</label><input id="m-id" placeholder="如 agent-a"></div>
  <div class="field"><label>名称</label><input id="m-name" placeholder="可选，默认同 ID"></div>
  <div class="field"><label>Workspace 路径</label><input id="m-ws" placeholder="如 /path/to/ws-a"></div>
  <div class="field"><div class="check"><input type="checkbox" id="m-cfg"><label for="m-cfg">创建空 config.json</label></div></div>
  <button class="btn primary" id="m-create">创建</button>
  <h3>添加不受控 Agent</h3>
  <div class="field"><label>ID</label><input id="e-id" placeholder="如 ext-1"></div>
  <div class="field"><label>WS 地址</label><input id="e-url" placeholder="ws://host:port/ws"></div>
  <button class="btn primary" id="e-add">添加</button>
  <h3>Agent 列表</h3>
  <div id="agent-list"></div>
</div>
<script>
(function(){
  var tabsEl = document.getElementById('tabs');
  var stageEl = document.getElementById('stage');
  var emptyEl = document.getElementById('empty');
  var drawer = document.getElementById('drawer');
  var scrim = document.getElementById('scrim');
  var listEl = document.getElementById('agent-list');
  var agents = [];      // [{id,name,type,running,connected,workspace,port,ws_url}]
  var current = null;   // active agent id
  var frames = {};      // id -> iframe element

  function esc(s){ return String(s).replace(/[&<>"']/g,function(c){return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c];}); }

  function api(method, url, body, cb){
    var opts = { method: method, headers: {} };
    if (body){ opts.headers['Content-Type']='application/json'; opts.body=JSON.stringify(body); }
    fetch(url, opts).then(function(r){ return r.json().then(function(j){ cb(r.status, j); }); })
      .catch(function(e){ cb(0, { error: String(e) }); });
  }

  function refresh(){
    api('GET', '/api/agents', null, function(st, j){
      agents = (j && j.agents) || [];
      // Drop frames for removed agents.
      Object.keys(frames).forEach(function(id){
        if (!agents.some(function(a){ return a.id === id; })){ removeFrame(id); }
      });
      if (!current && agents.length) current = agents[0].id;
      if (current && !agents.some(function(a){ return a.id === current; })) current = agents.length ? agents[0].id : null;
      renderTabs();
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

  function renderTabs(){
    tabsEl.innerHTML = '';
    agents.forEach(function(a){
      var t = document.createElement('div');
      t.className = 'tab' + (a.id === current ? ' active' : '');
      var st = (a.running || a.connected) ? 'on' : 'off';
      t.innerHTML = '<span class="st ' + st + '"></span>' + esc(a.name || a.id) +
        '<span class="x" title="关闭">✕</span>';
      t.onclick = function(e){
        if (e.target.classList.contains('x')){ closeAgent(a.id); return; }
        current = a.id; renderTabs(); ensureFrame(current);
      };
      tabsEl.appendChild(t);
    });
  }

  function closeAgent(id){
    if (!confirm('关闭 agent ' + id + '？')) return;
    api('DELETE', '/api/agents/' + encodeURIComponent(id), null, function(st, j){
      if (st >= 400) alert('删除失败: ' + (j.error || st));
      refresh();
    });
  }

  function renderList(){
    listEl.innerHTML = '';
    agents.forEach(function(a){
      var typeBadge = a.type === 'external' ? 'external' : 'managed';
      var typeLabel = a.type === 'external' ? '不受控' : '受控';
      var stateBadge = (a.running || a.connected) ? 'on' : 'off';
      var stateLabel = (a.running || a.connected) ? '运行中' : '已停止';
      var meta = a.type === 'external' ? ('WS: ' + (a.ws_url || '')) : ('WS: ' + (a.workspace || '') + ' · 端口 ' + (a.port || '-'));
      var row = document.createElement('div');
      row.className = 'agent-row';
      row.innerHTML =
        '<div class="top"><span class="name">' + esc(a.name || a.id) + '</span>' +
        '<span class="badge ' + typeBadge + '">' + typeLabel + '</span>' +
        '<span class="badge ' + stateBadge + '">' + stateLabel + '</span></div>' +
        '<div class="meta">' + esc(meta) + '</div>' +
        '<div class="actions">' + actionButtons(a) + '</div>';
      listEl.appendChild(row);
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
    listEl.querySelectorAll('button[data-act]').forEach(function(btn){
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

  // ---- Drawer open/close ----
  function openDrawer(){ drawer.classList.add('open'); scrim.classList.add('show'); }
  function closeDrawer(){ drawer.classList.remove('open'); scrim.classList.remove('show'); }
  document.getElementById('manageBtn').onclick = openDrawer;
  document.getElementById('drawerClose').onclick = closeDrawer;
  scrim.onclick = closeDrawer;

  document.getElementById('m-create').onclick = function(){
    var id = document.getElementById('m-id').value.trim();
    var ws = document.getElementById('m-ws').value.trim();
    if (!id || !ws){ alert('请填写 ID 和 Workspace 路径'); return; }
    api('POST', '/api/agents', {
      id: id,
      name: document.getElementById('m-name').value.trim(),
      workspace: ws,
      create_config: document.getElementById('m-cfg').checked
    }, function(st, j){
      if (st >= 400) alert('创建失败: ' + (j.error || st));
      else { document.getElementById('m-id').value=''; document.getElementById('m-ws').value=''; }
      refresh();
    });
  };

  document.getElementById('e-add').onclick = function(){
    var id = document.getElementById('e-id').value.trim();
    var url = document.getElementById('e-url').value.trim();
    if (!id || !url){ alert('请填写 ID 和 WS 地址'); return; }
    api('POST', '/api/agents/external', { id: id, ws_url: url }, function(st, j){
      if (st >= 400) alert('添加失败: ' + (j.error || st));
      else { document.getElementById('e-id').value=''; document.getElementById('e-url').value=''; }
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

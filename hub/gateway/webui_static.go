package gateway

import "errors"

// errClosed is returned when serving on a closed listener.
var errClosed = errors.New("use of closed network connection")

// webIndexHTML is the embedded hub Web UI frontend. It connects to the hub's
// /ws endpoint for chat (list/switch agents, send messages, display events) and
// uses the /api/agents HTTP endpoints to manage agent lifecycles (create a
// managed agent, start/stop/delete it, or add an external agent).
const webIndexHTML = `<!DOCTYPE html>
<html lang="zh">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>co-shell-hub 网关</title>
<style>
  :root { --bg:#1e1e2e; --panel:#27273a; --fg:#cdd6f4; --accent:#89b4fa; --ok:#a6e3a1; --err:#f38ba8; --warn:#f9e2af; }
  * { box-sizing:border-box; }
  body { margin:0; font-family:system-ui,-apple-system,sans-serif; background:var(--bg); color:var(--fg); height:100vh; display:flex; flex-direction:column; }
  header { padding:10px 16px; background:var(--panel); display:flex; align-items:center; gap:12px; border-bottom:1px solid #333; flex-wrap:wrap; }
  header h1 { font-size:16px; margin:0; }
  #agents { display:flex; gap:6px; flex-wrap:wrap; }
  .agent { padding:4px 10px; border-radius:12px; background:#333; cursor:pointer; font-size:13px; border:1px solid transparent; }
  .agent.active { background:var(--accent); color:#111; font-weight:600; }
  #status { margin-left:auto; font-size:12px; color:#888; }
  #main { flex:1; display:flex; min-height:0; }
  #chat { flex:1; display:flex; flex-direction:column; min-width:0; }
  #log { flex:1; overflow-y:auto; padding:12px 16px; font-family:ui-monospace,Menlo,monospace; font-size:13px; white-space:pre-wrap; word-break:break-word; }
  .msg { margin:2px 0; }
  .you { color:var(--ok); }
  .agent-msg { color:var(--fg); }
  .sys { color:#888; font-style:italic; }
  .err { color:var(--err); }
  #inputbar { display:flex; gap:8px; padding:10px 16px; background:var(--panel); border-top:1px solid #333; }
  #input { flex:1; padding:8px 12px; border-radius:8px; border:1px solid #444; background:#1a1a28; color:var(--fg); font-size:14px; }
  #send { padding:8px 18px; border-radius:8px; border:none; background:var(--accent); color:#111; font-weight:600; cursor:pointer; }
  #send:disabled { opacity:.5; cursor:not-allowed; }
  #manage { width:340px; border-left:1px solid #333; overflow-y:auto; padding:12px; background:var(--panel); }
  #manage h2 { font-size:14px; margin:0 0 8px; }
  #manage h3 { font-size:13px; margin:14px 0 6px; color:var(--accent); }
  .field { margin-bottom:8px; }
  .field label { display:block; font-size:12px; color:#aaa; margin-bottom:3px; }
  .field input { width:100%; padding:6px 8px; border-radius:6px; border:1px solid #444; background:#1a1a28; color:var(--fg); font-size:13px; }
  .field .check { display:flex; align-items:center; gap:6px; }
  .field .check input { width:auto; }
  .btn { padding:6px 12px; border-radius:6px; border:none; cursor:pointer; font-size:12px; font-weight:600; }
  .btn.primary { background:var(--accent); color:#111; }
  .btn.ok { background:var(--ok); color:#111; }
  .btn.stop { background:var(--warn); color:#111; }
  .btn.danger { background:var(--err); color:#111; }
  .btn:disabled { opacity:.5; cursor:not-allowed; }
  .agent-row { border:1px solid #333; border-radius:8px; padding:8px; margin-bottom:8px; background:#1e1e2e; }
  .agent-row .top { display:flex; align-items:center; gap:6px; }
  .agent-row .name { font-weight:600; font-size:13px; }
  .agent-row .meta { font-size:11px; color:#888; margin:4px 0; word-break:break-all; }
  .badge { font-size:10px; padding:1px 6px; border-radius:8px; }
  .badge.managed { background:#313244; color:var(--accent); }
  .badge.external { background:#313244; color:var(--warn); }
  .badge.on { background:#1e3a2a; color:var(--ok); }
  .badge.off { background:#3a1e1e; color:var(--err); }
  .agent-row .actions { display:flex; gap:6px; margin-top:6px; }
  .hint { font-size:11px; color:#888; margin-top:4px; }
</style>
</head>
<body>
<header>
  <h1>co-shell-hub 网关</h1>
  <div id="agents"></div>
  <div id="status">未连接</div>
</header>
<div id="main">
  <div id="chat">
    <div id="log"></div>
    <div id="inputbar">
      <input id="input" placeholder="输入消息，回车发送…" autocomplete="off">
      <button id="send">发送</button>
    </div>
  </div>
  <div id="manage">
    <h2>Agent 管理</h2>
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
</div>
<script>
(function(){
  var logEl = document.getElementById('log');
  var agentsEl = document.getElementById('agents');
  var statusEl = document.getElementById('status');
  var inputEl = document.getElementById('input');
  var sendBtn = document.getElementById('send');
  var listEl = document.getElementById('agent-list');
  var ws = null;
  var currentAgent = null;
  var agentList = [];

  function addLine(cls, text) {
    var d = document.createElement('div');
    d.className = 'msg ' + cls;
    d.textContent = text;
    logEl.appendChild(d);
    logEl.scrollTop = logEl.scrollHeight;
  }

  function renderAgents() {
    agentsEl.innerHTML = '';
    agentList.forEach(function(a) {
      var b = document.createElement('div');
      b.className = 'agent' + (a.id === currentAgent ? ' active' : '');
      b.textContent = a.name || a.id;
      b.onclick = function() { switchAgent(a.id); };
      agentsEl.appendChild(b);
    });
  }

  function send(obj) {
    if (ws && ws.readyState === WebSocket.OPEN) ws.send(JSON.stringify(obj));
  }

  function switchAgent(id) {
    currentAgent = id;
    renderAgents();
    send({ type: 'switch_agent', payload: { agent_id: id } });
    addLine('sys', '[系统] 切换到 agent: ' + id);
  }

  function connect() {
    var proto = location.protocol === 'https:' ? 'wss://' : 'ws://';
    ws = new WebSocket(proto + location.host + '/ws');
    ws.onopen = function() {
      statusEl.textContent = '已连接';
      send({ type: 'list_agents' });
    };
    ws.onclose = function() {
      statusEl.textContent = '已断开，重连中…';
      setTimeout(connect, 2000);
    };
    ws.onerror = function() { statusEl.textContent = '连接错误'; };
    ws.onmessage = function(ev) {
      var env;
      try { env = JSON.parse(ev.data); } catch(e) { return; }
      if (env.type === 'list_agents') {
        agentList = (env.payload && env.payload.agents) || [];
        if (!currentAgent && agentList.length) currentAgent = agentList[0].id;
        renderAgents();
        return;
      }
      if (env.type === 'error') { addLine('err', '[错误] ' + (env.payload || '')); return; }
      if (env.type === 'agent_event') {
        var m;
        try { m = JSON.parse(env.payload); } catch(e) { m = null; }
        if (m && m.kind === 'event' && m.event) {
          var e = m.event;
          if (e.type === 'content_chunk' || e.type === 'text') {
            addLine('agent-msg', e.text || '');
          } else if (e.type === 'done') {
            addLine('sys', '[完成]');
          }
        } else if (m && m.kind === 'sessions') {
          // session list update — ignore for now
        } else {
          addLine('agent-msg', env.payload);
        }
        return;
      }
      addLine('sys', '[消息] ' + JSON.stringify(env));
    };
  }

  function sendMessage() {
    var text = inputEl.value.trim();
    if (!text) return;
    addLine('you', '你: ' + text);
    inputEl.value = '';
    send({ type: 'input', payload: { type: 'input', text: text } });
  }

  // ---- Agent management (HTTP /api/agents) ----

  function api(method, url, body, cb) {
    var opts = { method: method, headers: {} };
    if (body) { opts.headers['Content-Type'] = 'application/json'; opts.body = JSON.stringify(body); }
    fetch(url, opts).then(function(r) {
      return r.json().then(function(j) { cb(r.status, j); });
    }).catch(function(e) { cb(0, { error: String(e) }); });
  }

  function refreshList() {
    api('GET', '/api/agents', null, function(status, j) {
      var rows = (j && j.agents) || [];
      listEl.innerHTML = '';
      rows.forEach(function(a) {
        var row = document.createElement('div');
        row.className = 'agent-row';
        var typeBadge = a.type === 'external' ? 'external' : 'managed';
        var typeLabel = a.type === 'external' ? '不受控' : '受控';
        var stateBadge = (a.running || a.connected) ? 'on' : 'off';
        var stateLabel = (a.running || a.connected) ? '运行中' : '已停止';
        var meta = a.type === 'external' ? ('WS: ' + (a.ws_url || '')) : ('WS: ' + (a.workspace || '') + ' · 端口 ' + (a.port || '-'));
        row.innerHTML =
          '<div class="top"><span class="name">' + esc(a.name || a.id) + '</span>' +
          '<span class="badge ' + typeBadge + '">' + typeLabel + '</span>' +
          '<span class="badge ' + stateBadge + '">' + stateLabel + '</span></div>' +
          '<div class="meta">' + esc(meta) + '</div>' +
          '<div class="actions">' + actionButtons(a) + '</div>';
        listEl.appendChild(row);
      });
      bindActions(rows);
    });
  }

  function actionButtons(a) {
    var s = '';
    if (a.type === 'managed') {
      if (a.running) {
        s += '<button class="btn stop" data-act="stop" data-id="' + esc(a.id) + '">停止</button>';
      } else {
        s += '<button class="btn ok" data-act="start" data-id="' + esc(a.id) + '">启动</button>';
      }
    }
    s += '<button class="btn danger" data-act="del" data-id="' + esc(a.id) + '">删除</button>';
    return s;
  }

  function bindActions(rows) {
    listEl.querySelectorAll('button[data-act]').forEach(function(btn) {
      btn.onclick = function() {
        var act = btn.getAttribute('data-act');
        var id = btn.getAttribute('data-id');
        if (act === 'start') api('POST', '/api/agents/' + encodeURIComponent(id) + '/start', null, function(st, j) {
          if (st >= 400) alert('启动失败: ' + (j.error || st)); refreshList();
        });
        else if (act === 'stop') api('POST', '/api/agents/' + encodeURIComponent(id) + '/stop', null, function(st, j) {
          if (st >= 400) alert('停止失败: ' + (j.error || st)); refreshList();
        });
        else if (act === 'del') {
          if (confirm('删除 agent ' + id + '？')) api('DELETE', '/api/agents/' + encodeURIComponent(id), null, function(st, j) {
            if (st >= 400) alert('删除失败: ' + (j.error || st)); refreshList();
          });
        }
      };
    });
  }

  function esc(s) {
    return String(s).replace(/[&<>"']/g, function(c) {
      return { '&':'&amp;', '<':'&lt;', '>':'&gt;', '"':'&quot;', "'":'&#39;' }[c];
    });
  }

  document.getElementById('m-create').onclick = function() {
    var id = document.getElementById('m-id').value.trim();
    var ws = document.getElementById('m-ws').value.trim();
    if (!id || !ws) { alert('请填写 ID 和 Workspace 路径'); return; }
    api('POST', '/api/agents', {
      id: id,
      name: document.getElementById('m-name').value.trim(),
      workspace: ws,
      create_config: document.getElementById('m-cfg').checked
    }, function(st, j) {
      if (st >= 400) alert('创建失败: ' + (j.error || st));
      else { document.getElementById('m-id').value = ''; document.getElementById('m-ws').value = ''; }
      refreshList();
    });
  };

  document.getElementById('e-add').onclick = function() {
    var id = document.getElementById('e-id').value.trim();
    var url = document.getElementById('e-url').value.trim();
    if (!id || !url) { alert('请填写 ID 和 WS 地址'); return; }
    api('POST', '/api/agents/external', { id: id, ws_url: url }, function(st, j) {
      if (st >= 400) alert('添加失败: ' + (j.error || st));
      else { document.getElementById('e-id').value = ''; document.getElementById('e-url').value = ''; }
      refreshList();
    });
  };

  sendBtn.onclick = sendMessage;
  inputEl.addEventListener('keydown', function(e) { if (e.key === 'Enter') sendMessage(); });
  connect();
  refreshList();
  setInterval(refreshList, 3000);
})();
</script>
</body>
</html>
`

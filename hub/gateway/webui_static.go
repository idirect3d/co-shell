package gateway

import "errors"

// errClosed is returned when serving on a closed listener.
var errClosed = errors.New("use of closed network connection")

// webIndexHTML is the embedded hub Web UI frontend. It connects to the hub's
// /ws endpoint, lists the connected agents, lets the user switch the current
// agent, send messages, and displays the agent's returned events.
const webIndexHTML = `<!DOCTYPE html>
<html lang="zh">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>co-shell-hub 网关</title>
<style>
  :root { --bg:#1e1e2e; --panel:#27273a; --fg:#cdd6f4; --accent:#89b4fa; --ok:#a6e3a1; --err:#f38ba8; }
  * { box-sizing:border-box; }
  body { margin:0; font-family:system-ui,-apple-system,sans-serif; background:var(--bg); color:var(--fg); height:100vh; display:flex; flex-direction:column; }
  header { padding:10px 16px; background:var(--panel); display:flex; align-items:center; gap:12px; border-bottom:1px solid #333; }
  header h1 { font-size:16px; margin:0; }
  #agents { display:flex; gap:6px; flex-wrap:wrap; }
  .agent { padding:4px 10px; border-radius:12px; background:#333; cursor:pointer; font-size:13px; border:1px solid transparent; }
  .agent.active { background:var(--accent); color:#111; font-weight:600; }
  #status { margin-left:auto; font-size:12px; color:#888; }
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
</style>
</head>
<body>
<header>
  <h1>co-shell-hub 网关</h1>
  <div id="agents"></div>
  <div id="status">未连接</div>
</header>
<div id="log"></div>
<div id="inputbar">
  <input id="input" placeholder="输入消息，回车发送…" autocomplete="off">
  <button id="send">发送</button>
</div>
<script>
(function(){
  var logEl = document.getElementById('log');
  var agentsEl = document.getElementById('agents');
  var statusEl = document.getElementById('status');
  var inputEl = document.getElementById('input');
  var sendBtn = document.getElementById('send');
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
    // Forward a co-shell "input" clientMessage to the current agent.
    send({ type: 'input', payload: { type: 'input', text: text } });
  }

  sendBtn.onclick = sendMessage;
  inputEl.addEventListener('keydown', function(e) { if (e.key === 'Enter') sendMessage(); });
  connect();
})();
</script>
</body>
</html>
`

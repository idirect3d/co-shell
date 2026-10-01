// FEATURE-571: the multi-window registry and the main-area panel.
//
// A window is where render_ui paints when the target is not the chat stream:
// "float" is the bottom-right card introduced by FEATURE-524, "panel" covers
// the main message area. Several windows coexist: the panel shows the active
// one and its tab strip (swipe / the ≡ list on narrow screens) switches between
// them, while float windows stack in the corner.
//
// The registry is the frontend mirror of the agent-side registry behind the
// <ui_windows> block: every change (render, ✕/Esc, tab switch, surface flip) is
// reported back with a ui_window_state message, so the LLM never addresses a
// window the user has already closed.
//
// Windows belong to a session. Switching away parks them (DOM and all) in
// memory, switching back restores them, and a reload restores the window list
// plus the last rendered tree of each window from localStorage.
//
// Author: L.Shuang
// Created: 2026-10-01
// MIT License - Copyright (c) 2026 L.Shuang

(function (global) {
  "use strict";

  var MAX_WINDOWS = 8; // mirrors agent.UIMaxWindows
  var DEFAULT_NAME = "default"; // mirrors agent.UIDefaultWindowName
  var LS_PREFIX = "co-shell-ui-windows:";
  var FLOAT_STEP = 28; // cascade offset between stacked float windows
  var SWIPE_MIN = 40; // px a horizontal swipe must travel to switch windows

  function t(zh, en) {
    return global.UI && typeof global.UI.t === "function" ? global.UI.t(zh, en) : zh;
  }

  var panel = null, panelTabs = null, panelBody = null;
  var panelFloatBtn = null, panelListBtn = null;
  var listPop = null;
  var cloneSource = null; // pristine copy of the #uiWindow markup
  var templateAdopted = false; // the first float window reuses #uiWindow itself

  var sessionId = "", sessionKnown = false;
  var windows = [], activeName = "";
  var buckets = Object.create(null); // session id -> parked windows (live DOM)

  // --------------------------------------------------------------- registry

  function find(name) {
    for (var i = 0; i < windows.length; i++) {
      if (windows[i].name === name) return windows[i];
    }
    return null;
  }

  function activeWindow() {
    return find(activeName);
  }

  // floatIndex returns a record's position among the float windows, which is
  // what keeps stacked windows visibly offset instead of exactly on top.
  function floatIndex(rec) {
    var n = 0;
    for (var i = 0; i < windows.length; i++) {
      if (windows[i] === rec) return n;
      if (windows[i].surface === "float") n++;
    }
    return n;
  }

  // newRecord builds a window body: a holder that owns the component tree host.
  // The holder is what moves between the panel body and a float shell, so a
  // surface flip never rebuilds (or loses) the rendered tree.
  function newRecord(name, title, surface) {
    var holder = document.createElement("div");
    holder.className = "ui-win-holder";
    holder.dataset.windowName = name;
    var host = document.createElement("div");
    host.className = "ui-tree";
    holder.appendChild(host);
    return {
      name: name,
      title: title || "",
      surface: surface === "panel" ? "panel" : "float",
      summary: "",
      size: "",
      tree: "",
      holder: holder,
      host: host,
      shell: null,
      body: null,
      titleEl: null
    };
  }

  // ----------------------------------------------------------------- shells

  function shellTemplate() {
    return document.getElementById("uiWindow");
  }

  // makeFloatShell attaches a bottom-right shell for rec and returns it. The
  // first window adopts the original #uiWindow element (so its ids and markup
  // stay exactly as FEATURE-524 shipped them); later ones are clones with the
  // ids stripped, since ids must stay unique.
  function makeFloatShell(rec) {
    var tpl = shellTemplate();
    var shell;
    if (tpl && !templateAdopted) {
      templateAdopted = true;
      shell = tpl;
      shell.classList.remove("hidden");
      rec.body = document.getElementById("uiWindowBody");
      rec.titleEl = document.getElementById("uiWindowTitle");
    } else if (cloneSource) {
      shell = cloneSource.cloneNode(true);
      shell.removeAttribute("id");
      var ids = shell.querySelectorAll("[id]");
      for (var i = 0; i < ids.length; i++) {
        ids[i].removeAttribute("id");
      }
      shell.classList.remove("hidden");
      rec.body = shell.querySelector(".ui-window-body");
      rec.titleEl = shell.querySelector(".ui-window-title");
      document.body.appendChild(shell);
    } else {
      return null;
    }
    if (!rec.body) return null;
    while (rec.body.firstChild) {
      rec.body.removeChild(rec.body.firstChild);
    }
    shell.dataset.windowName = rec.name;
    rec.shell = shell;
    return shell;
  }

  // mount places rec's holder on the surface it belongs to.
  function mount(rec) {
    if (rec.surface === "panel") {
      if (rec.shell && rec.shell.parentNode) {
        rec.shell.parentNode.removeChild(rec.shell);
      }
      rec.shell = null;
      rec.body = null;
      if (panelBody && rec.holder.parentNode !== panelBody) {
        panelBody.appendChild(rec.holder);
      }
      return;
    }
    var shell = makeFloatShell(rec);
    if (!shell) return;
    rec.body.appendChild(rec.holder);
    applySize(rec);
  }

  // unmount detaches rec's DOM without destroying it, so a session switch (or a
  // surface flip) can bring the very same nodes back.
  function unmount(rec) {
    if (rec.holder && rec.holder.parentNode) {
      rec.holder.parentNode.removeChild(rec.holder);
    }
    if (rec.shell && rec.shell.parentNode) {
      rec.shell.parentNode.removeChild(rec.shell);
    }
    rec.shell = null;
    rec.body = null;
    rec.titleEl = null;
  }

  // applySize mirrors the size preset onto the shell through the shared clamp
  // helper of FEATURE-524, so a preset keeps meaning the same pixels on every
  // float window.
  function applySize(rec) {
    if (!rec || !rec.shell || !global.UI || typeof global.UI.applyWindowSize !== "function") return;
    var preset = rec.size || "";
    if (preset === "auto") {
      rec.shell.dataset.uiWinSize = "";
      global.UI.applyWindowSize(rec.shell, "");
      return;
    }
    rec.shell.dataset.uiWinSize = preset;
    global.UI.applyWindowSize(rec.shell, preset);
  }

  // ------------------------------------------------------------------ state

  // ensureWindow creates or updates a window and returns it. A window cap is
  // enforced here so the panel can never grow past what the backend accepts.
  function ensureWindow(name, title, surface) {
    if (!name) name = activeName || DEFAULT_NAME;
    var rec = find(name);
    if (!rec) {
      if (windows.length >= MAX_WINDOWS) {
        console.warn("[ui-windows] window limit reached, ignoring " + name);
        return null;
      }
      rec = newRecord(name, title, surface);
      windows.push(rec);
      mount(rec);
    } else {
      if (title) rec.title = title;
      if (surface && surface !== rec.surface) {
        rec.surface = surface === "panel" ? "panel" : "float";
        mount(rec);
      }
    }
    return rec;
  }

  function activate(name, silent) {
    if (!find(name)) return false;
    activeName = name;
    refresh();
    persist();
    if (!silent) sync(null);
    return true;
  }

  function closeWindow(name) {
    name = name || activeName;
    var rec = find(name);
    if (!rec) return false;
    windows.splice(windows.indexOf(rec), 1);
    unmount(rec);
    if (activeName === name) {
      activeName = windows.length ? windows[windows.length - 1].name : "";
    }
    refresh();
    persist();
    sync([name]);
    return true;
  }

  function setSurface(name, surface) {
    if (surface !== "panel" && surface !== "float") return false;
    var rec = find(name || activeName);
    if (!rec) return false;
    if (rec.surface === surface) {
      activate(rec.name, true);
      sync(null);
      return true;
    }
    rec.surface = surface;
    mount(rec);
    activate(rec.name, true);
    sync(null);
    return true;
  }

  // render paints a component tree into the addressed window, opening it when
  // it does not exist yet: render_ui(target="panel") alone is enough to bring
  // the panel up.
  function render(meta) {
    var surface = meta.ui_target === "panel" ? "panel" : "float";
    var name = meta.ui_window_name || activeName || DEFAULT_NAME;
    var rec = ensureWindow(name, meta.ui_window_title || "", surface);
    if (!rec) return false;
    rec.tree = meta.ui_tree || "";
    rec.summary = treeSummary(rec.tree, meta.ui_id);
    if (rec.host) {
      rec.host.dataset.uiId = meta.ui_id || "";
      rec.host.replaceChildren();
      if (global.UI && typeof global.UI.renderTree === "function") {
        global.UI.renderTree(rec.tree, rec.host);
      }
    }
    activeName = rec.name;
    refresh();
    persist();
    sync(null);
    return true;
  }

  // treeSummary labels a window for <ui_windows>, e.g. "card(ui-7)". It matches
  // the agent-side summary so the LLM sees the same string on both ends.
  function treeSummary(treeJSON, id) {
    try {
      var node = JSON.parse(treeJSON || "null");
      if (node && typeof node.type === "string") {
        return node.type + "(" + String(id || "") + ")";
      }
    } catch (err) {
      // The summary is cosmetic: a malformed tree still renders (the renderer
      // draws its own error node), so a parse failure keeps the previous label.
    }
    return "";
  }
  // ------------------------------------------------------------------- views

  // refresh re-applies everything derived from the registry: which panel window
  // is visible, where the float windows sit, the tab strip and the header
  // buttons. It is the single place the screen is reconciled with the state.
  function refresh() {
    var hasPanel = false;
    for (var i = 0; i < windows.length; i++) {
      var rec = windows[i];
      if (rec.surface === "panel") {
        hasPanel = true;
        rec.holder.classList.toggle("hidden", rec.name !== activeName);
      } else {
        rec.holder.classList.remove("hidden");
        if (rec.shell) {
          var idx = floatIndex(rec);
          rec.shell.style.right = 16 + idx * FLOAT_STEP + "px";
          rec.shell.style.bottom = 16 + idx * FLOAT_STEP + "px";
          rec.shell.style.zIndex = String(60 + idx + (rec.name === activeName ? 5 : 0));
        }
      }
      if (rec.titleEl) rec.titleEl.textContent = rec.title || rec.name;
    }
    if (panel) panel.classList.toggle("hidden", !hasPanel);
    renderTabs();
    if (panelFloatBtn) {
      var act = activeWindow();
      var toFloat = !!act && act.surface === "panel";
      panelFloatBtn.textContent = toFloat ? "⇱" : "⇲";
      panelFloatBtn.title = toFloat
        ? t("收成浮窗", "Collapse to a floating window")
        : t("展开为面板", "Expand to the panel");
    }
    updateList();
  }

  // renderTabs rebuilds the panel tab strip. Tabs list every open window (on
  // both surfaces), so a window that was collapsed to a float can be brought
  // back with one click; narrow screens hide the strip in CSS and use the ≡
  // list and swipe instead.
  function renderTabs() {
    if (!panelTabs) return;
    panelTabs.replaceChildren();
    for (var i = 0; i < windows.length; i++) {
      var rec = windows[i];
      var btn = document.createElement("button");
      btn.type = "button";
      btn.className = "ui-panel-tab" + (rec.name === activeName ? " active" : "");
      btn.textContent = rec.title || rec.name;
      btn.title = rec.name + " · " + rec.surface;
      btn.dataset.windowName = rec.name;
      btn.addEventListener("click", onTabClick);
      panelTabs.appendChild(btn);
    }
  }

  function onTabClick(e) {
    activate(e.currentTarget.dataset.windowName || "");
  }

  // updateList refills the ≡ dropdown, the switching entry point that stays
  // available when the tab strip is hidden on a narrow screen.
  function updateList() {
    if (!listPop) return;
    listPop.replaceChildren();
    for (var i = 0; i < windows.length; i++) {
      var rec = windows[i];
      var btn = document.createElement("button");
      btn.type = "button";
      btn.className = rec.name === activeName ? "active" : "";
      btn.textContent = (rec.title || rec.name) + " · " + rec.surface;
      btn.dataset.windowName = rec.name;
      btn.addEventListener("click", function (ev) {
        hideList();
        activate(ev.currentTarget.dataset.windowName || "");
      });
      listPop.appendChild(btn);
    }
  }

  function toggleList() {
    if (!panel) return;
    if (!listPop) {
      listPop = document.createElement("div");
      listPop.className = "ui-panel-list hidden";
      panel.appendChild(listPop);
    }
    if (listPop.classList.contains("hidden")) {
      updateList();
      listPop.classList.remove("hidden");
      return;
    }
    hideList();
  }

  function hideList() {
    if (listPop) listPop.classList.add("hidden");
  }

  // step activates the window that is dir places away in the creation order,
  // wrapping around: what a left/right swipe does.
  function step(dir) {
    if (windows.length < 2) return;
    var at = 0;
    for (var i = 0; i < windows.length; i++) {
      if (windows[i].name === activeName) at = i;
    }
    var next = (at + dir + windows.length) % windows.length;
    activate(windows[next].name);
  }

  // ------------------------------------------------------- backend mirroring

  function persist() {
    if (!sessionKnown || !sessionId) return;
    var out = { active: activeName, windows: [] };
    for (var i = 0; i < windows.length; i++) {
      out.windows.push({
        name: windows[i].name,
        title: windows[i].title,
        surface: windows[i].surface,
        tree: windows[i].tree || "",
        summary: windows[i].summary || ""
      });
    }
    try {
      global.localStorage.setItem(LS_PREFIX + sessionId, JSON.stringify(out));
    } catch (err) {
      // A disabled or full localStorage must not break rendering: persistence is
      // best effort, the live windows stay on screen either way.
      console.warn("[ui-windows] persist failed", err);
    }
  }

  // sync mirrors the registry onto the agent side, so the <ui_windows> block
  // the LLM reads always matches what the user sees.
  function sync(closed) {
    if (typeof global.wsSend !== "function") return;
    var list = [];
    for (var i = 0; i < windows.length; i++) {
      var rec = windows[i];
      list.push({
        name: rec.name,
        title: rec.title,
        surface: rec.surface,
        active: rec.name === activeName,
        summary: rec.summary
      });
    }
    var msg = { type: "ui_window_state", windows: list };
    if (closed && closed.length) msg.closed = closed;
    global.wsSend(msg);
  }

  // --------------------------------------------------------------- sessions

  function stashSession() {
    if (!sessionId) return;
    for (var i = 0; i < windows.length; i++) {
      unmount(windows[i]); // detached, not destroyed: the DOM is parked as is
    }
    buckets[sessionId] = { windows: windows, activeName: activeName };
  }

  // adopt swaps in a window list. Records that still own their rendered DOM
  // (a switch back inside this page) are re-attached untouched; records rebuilt
  // from localStorage have their tree re-rendered from the stored JSON.
  function adopt(list, active) {
    windows = list || [];
    activeName = active || (windows.length ? windows[windows.length - 1].name : "");
    for (var i = 0; i < windows.length; i++) {
      var rec = windows[i];
      mount(rec);
      if (rec.host && !rec.host.childNodes.length && rec.tree &&
          global.UI && typeof global.UI.renderTree === "function") {
        global.UI.renderTree(rec.tree, rec.host);
      }
    }
    if (!find(activeName) && windows.length) {
      activeName = windows[windows.length - 1].name;
    }
    refresh();
  }

  function readStored(id) {
    var out = { windows: [], active: "" };
    var raw = null;
    try {
      raw = global.localStorage.getItem(LS_PREFIX + id);
    } catch (err) {
      return out;
    }
    if (!raw) return out;
    var data = null;
    try {
      data = JSON.parse(raw);
    } catch (err) {
      console.warn("[ui-windows] stored windows ignored", err);
      return out;
    }
    if (!data || !data.windows) return out;
    for (var i = 0; i < data.windows.length && i < MAX_WINDOWS; i++) {
      var w = data.windows[i];
      if (!w || !w.name) continue;
      var rec = newRecord(w.name, w.title || "", w.surface);
      rec.tree = w.tree || "";
      rec.summary = w.summary || "";
      out.windows.push(rec);
    }
    out.active = data.active || "";
    return out;
  }

  // setSession parks the windows of the session being left and brings in the
  // ones of the session being entered, then tells the new agent what is on
  // screen (its registry starts empty).
  function setSession(id) {
    if (!id || id === sessionId) {
      sessionKnown = sessionKnown || !!id;
      return;
    }
    if (sessionKnown) stashSession();
    sessionId = id;
    sessionKnown = true;
    var bucket = buckets[id];
    if (bucket) {
      adopt(bucket.windows, bucket.activeName);
      delete buckets[id];
    } else {
      var stored = readStored(id);
      adopt(stored.windows, stored.active);
    }
    sync(null);
    persist();
  }

  // ------------------------------------------------------------ event entry

  // applyWindowEvent handles a ui_window event: open/create, close or activate
  // the addressed window (FEATURE-571 extends FEATURE-524 with the name and the
  // surface).
  function applyWindowEvent(meta) {
    var action = meta.ui_window_action || "";
    var name = meta.ui_window_name || activeName || DEFAULT_NAME;
    if (action === "close") {
      return closeWindow(name);
    }
    if (action === "activate") {
      activate(name);
      return true;
    }
    if (action !== "open") return false;
    var existing = find(name);
    var surface = meta.ui_window_surface || (existing ? existing.surface : "float");
    var rec = ensureWindow(name, meta.ui_window_title || "", surface);
    if (!rec) return false;
    if (meta.ui_window_size) {
      rec.size = meta.ui_window_size;
      applySize(rec);
    }
    activate(rec.name, true);
    persist();
    sync(null);
    return true;
  }

  // closeFromShell closes the window owning the shell element (the ✕ button of
  // a floating window, or of the adopted #uiWindow shell).
  function closeFromShell(el) {
    var shell = el && el.closest ? el.closest(".ui-window[data-window-name]") : null;
    if (!shell) return false;
    return closeWindow(shell.dataset.windowName || "");
  }

  // toggleSurface flips the active window between the panel and the corner,
  // which is what the ⇱/⇲ button in the panel header does.
  function toggleSurface() {
    var rec = activeWindow();
    if (!rec) return false;
    return setSurface(rec.name, rec.surface === "panel" ? "float" : "panel");
  }

  // ------------------------------------------------------------------- init

  function bindSwipe() {
    if (!panel) return;
    var x0 = null, y0 = null;
    panel.addEventListener("touchstart", function (e) {
      if (!e.touches.length) return;
      x0 = e.touches[0].clientX;
      y0 = e.touches[0].clientY;
    }, { passive: true });
    panel.addEventListener("touchend", function (e) {
      if (x0 === null || !e.changedTouches.length) return;
      var dx = e.changedTouches[0].clientX - x0;
      var dy = e.changedTouches[0].clientY - y0;
      x0 = null;
      if (Math.abs(dx) < SWIPE_MIN || Math.abs(dx) < Math.abs(dy)) return;
      step(dx < 0 ? 1 : -1);
    }, { passive: true });
  }

  function init() {
    panel = document.getElementById("uiPanel");
    panelTabs = document.getElementById("uiPanelTabs");
    panelBody = document.getElementById("uiPanelBody");
    panelFloatBtn = document.getElementById("uiPanelFloat");
    panelListBtn = document.getElementById("uiPanelList");
    var tpl = shellTemplate();
    if (tpl) cloneSource = tpl.cloneNode(true);
    var closeBtn = document.getElementById("uiWindowClose");
    if (closeBtn) {
      closeBtn.addEventListener("click", function (e) {
        closeFromShell(e.currentTarget);
      });
    }
    var panelCloseBtn = document.getElementById("uiPanelClose");
    if (panelCloseBtn) {
      panelCloseBtn.title = t("关闭", "Close");
      panelCloseBtn.addEventListener("click", function () { closeWindow(""); });
    }
    if (panelFloatBtn) panelFloatBtn.addEventListener("click", function () { toggleSurface(); });
    if (panelListBtn) {
      panelListBtn.title = t("窗口列表", "Window list");
      panelListBtn.addEventListener("click", function (e) {
        e.stopPropagation();
        toggleList();
      });
    }
    // Esc closes the active window, the keyboard equivalent of its ✕.
    document.addEventListener("keydown", function (e) {
      if (e.key !== "Escape" || !windows.length) return;
      closeWindow("");
    });
    bindSwipe();
    // A viewport change re-clamps every float window, not just the adopted one
    // (ui.js keeps doing that for #uiWindow itself).
    global.addEventListener("resize", function () {
      for (var i = 0; i < windows.length; i++) applySize(windows[i]);
    });
    refresh();
  }

  global.UIWin = {
    render: render,
    applyWindowEvent: applyWindowEvent,
    closeWindow: closeWindow,
    closeFromShell: closeFromShell,
    activate: activate,
    setSurface: setSurface,
    toggleSurface: toggleSurface,
    setSession: setSession,
    isOpen: function () { return windows.length > 0; },
    // state is a read-only snapshot for diagnostics and the browser tests.
    state: function () {
      var list = [];
      for (var i = 0; i < windows.length; i++) {
        list.push({
          name: windows[i].name,
          title: windows[i].title,
          surface: windows[i].surface,
          active: windows[i].name === activeName,
          summary: windows[i].summary
        });
      }
      return { session: sessionId, active: activeName, windows: list };
    }
  };

  init();
})(window);

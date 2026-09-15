// co-shell Web UI - LLM component tree renderer (FEATURE-524).
//
// The agent's render_ui tool sends a declarative component tree; this file
// paints it into the main DOM, so results read as cards / tables / charts
// instead of a wall of text. Terminals ignore the ui_render event, so they
// keep the LLM's plain-text reply.
//
// SECURITY RULE (same as md.js): never assign innerHTML, never build HTML by
// string concatenation. Every element is created with document.createElement
// and every dynamic string goes through textContent. A tree containing
// "<img onerror=...>" therefore renders as literal text and creates no DOM.
//
// Adding a component = UI.register("name", { render(node, ctx) }) returning an
// element. The wire protocol, the Go validation and the agent loop stay
// untouched, so component work stays inside this file (plus one CSS block).
//
// Author: L.Shuang
// Created: 2026-09-15
// MIT License - Copyright (c) 2026 L.Shuang

(function (global) {
  "use strict";

  var registry = Object.create(null);
  // Depth guard for defence in depth: the Go side already rejects trees deeper
  // than UIMaxTreeDepth, but a hand-written or replayed event must not be able
  // to blow the stack here.
  var MAX_DEPTH = 8;

  // uiText returns the string for the active UI language. currentLang is the
  // app.js global (declared before app.js runs, read lazily at call time).
  function uiText(zh, en) {
    var lang = typeof global.currentLang === "string" ? global.currentLang : "zh";
    return lang === "en" ? en : zh;
  }

  // el creates an element with an optional class and text content.
  function el(tag, cls, text) {
    var node = document.createElement(tag);
    if (cls) node.className = cls;
    if (text !== undefined && text !== null) node.textContent = String(text);
    return node;
  }

  // str renders any prop value as display text (missing values become "").
  function str(v) {
    if (v === undefined || v === null) return "";
    if (typeof v === "object") return safeStringify(v);
    return String(v);
  }

  // num renders a prop value as a finite number.
  function num(v, fallback) {
    var n = typeof v === "number" ? v : parseFloat(v);
    return isFinite(n) ? n : fallback;
  }

  // safeStringify serializes a value for the raw-data fallback, never throwing
  // on cyclic input.
  function safeStringify(v) {
    try {
      return JSON.stringify(v, null, 2);
    } catch (err) {
      return String(v);
    }
  }

  // ---------------------------------------------------------------------------
  // Registry + recursive rendering
  // ---------------------------------------------------------------------------

  // register adds (or replaces) the renderer of a component type. def.render
  // receives { props, id, raw } and must return an element. Set
  // def.children === false for leaf components; def.childHost(host) redirects
  // child nodes into an inner container (e.g. a card body).
  function register(type, def) {
    if (!type || !def || typeof def.render !== "function") return;
    registry[type] = def;
  }

  function hasRenderer(type) {
    return Object.prototype.hasOwnProperty.call(registry, type);
  }

  function registeredTypes() {
    return Object.keys(registry).sort();
  }

  // renderNode builds the element of one node, then recurses into its children.
  function renderNode(node, depth) {
    if (!node || typeof node !== "object") return null;
    var type = typeof node.type === "string" ? node.type : "";
    var def = registry[type];
    if (!def) return renderUnknown(node, type);
    var props = node.props && typeof node.props === "object" ? node.props : {};
    var host;
    try {
      host = def.render({ props: props, id: node.id, raw: node }, depth);
    } catch (err) {
      return renderError(type, err);
    }
    if (!host || !host.nodeType) return null;
    if (node.id) host.setAttribute("data-ui-id", String(node.id));
    host.setAttribute("data-ui-type", type);
    if (def.children !== false) {
      var box = (typeof def.childHost === "function" && def.childHost(host)) || host;
      appendChildren(box, node.children, depth + 1);
    }
    return host;
  }

  // appendChildren renders node.children into box.
  function appendChildren(box, children, depth) {
    if (!box || !Array.isArray(children) || depth > MAX_DEPTH) return;
    for (var i = 0; i < children.length; i++) {
      var child = renderNode(children[i], depth);
      if (child) box.appendChild(child);
    }
  }

  // renderUnknown degrades an unregistered component to a labelled placeholder
  // with its raw JSON folded away — never an error, never a blank hole.
  function renderUnknown(node, type) {
    var box = el("div", "ui-unknown");
    var head = el("div", "ui-unknown-head");
    head.appendChild(el("span", "ui-unknown-type", type || uiText("（缺少 type）", "(missing type)")));
    head.appendChild(el("span", "ui-unknown-note", uiText(
      "当前页面没有该组件的渲染器，已降级显示原始数据。",
      "No renderer registered for this component; showing raw data."
    )));
    box.appendChild(head);
    var details = document.createElement("details");
    details.className = "ui-unknown-raw";
    details.appendChild(el("summary", "", uiText("原始组件数据", "Raw component data")));
    var pre = document.createElement("pre");
    pre.textContent = safeStringify(node);
    details.appendChild(pre);
    box.appendChild(details);
    return box;
  }

  // renderError keeps a broken component from taking the whole tree down.
  function renderError(type, err) {
    var box = el("div", "ui-unknown ui-render-error");
    box.appendChild(el("div", "ui-unknown-note", uiText("组件渲染失败：", "Component failed to render: ") + type));
    var pre = document.createElement("pre");
    pre.textContent = String((err && err.message) || err);
    box.appendChild(pre);
    return box;
  }

  // renderTree paints a tree (object or JSON text) into container.
  function renderTree(root, container) {
    if (!container) return false;
    var node = root;
    if (typeof node === "string") {
      try {
        node = JSON.parse(node);
      } catch (err) {
        container.appendChild(renderError("json", err));
        return false;
      }
    }
    var built = renderNode(node, 1);
    if (!built) return false;
    container.appendChild(built);
    return true;
  }

  // updateTree replaces the rendered element carrying data-ui-id === id with a
  // fresh render of node (FEATURE-524 in-place update). The node's own id is
  // authoritative: with an id it replaces the element, without one it replaces
  // the element's content.
  function updateTree(id, node) {
    if (!id || !node) return false;
    var host = findByUIID(id);
    if (!host || !host.parentNode) return false;
    var fresh = renderNode(node, 1);
    if (!fresh) return false;
    if (node.id) {
      host.replaceWith(fresh);
    } else {
      host.replaceChildren(fresh);
    }
    return true;
  }

  // findByUIID looks a rendered node up by its ui id.
  function findByUIID(id) {
    try {
      return document.querySelector('[data-ui-id="' + String(id).replace(/["\\]/g, "\\$&") + '"]');
    } catch (err) {
      return null;
    }
  }

  // ---------------------------------------------------------------------------
  // Stage 1 components: card / kv / callout / progress
  // ---------------------------------------------------------------------------

  // card is the container most trees start from: optional title/subtitle/icon
  // plus a body that hosts the child components.
  register("card", {
    render: function (node) {
      var p = node.props;
      var box = el("div", "ui-card");
      var title = str(p.title);
      var subtitle = str(p.subtitle);
      var icon = str(p.icon);
      if (title || subtitle || icon) {
        var head = el("div", "ui-card-head");
        if (icon) head.appendChild(el("span", "ui-card-icon", icon));
        var titles = el("div", "ui-card-titles");
        if (title) titles.appendChild(el("div", "ui-card-title", title));
        if (subtitle) titles.appendChild(el("div", "ui-card-subtitle", subtitle));
        head.appendChild(titles);
        box.appendChild(head);
      }
      box.appendChild(el("div", "ui-card-body"));
      return box;
    },
    childHost: function (host) {
      return host.querySelector(".ui-card-body");
    }
  });

  // kv is a key/value list for facts and single-record summaries.
  register("kv", {
    children: false,
    render: function (node) {
      var box = el("dl", "ui-kv");
      var items = Array.isArray(node.props.items) ? node.props.items : [];
      for (var i = 0; i < items.length; i++) {
        var item = items[i];
        if (!item || typeof item !== "object") continue;
        box.appendChild(el("dt", "ui-kv-k", str(item.k)));
        box.appendChild(el("dd", "ui-kv-v", str(item.v)));
      }
      return box;
    }
  });

  // callout highlights one conclusion; variant drives the accent colour.
  register("callout", {
    children: false,
    render: function (node) {
      var p = node.props;
      var variant = str(p.variant);
      if (["info", "warn", "success", "error"].indexOf(variant) < 0) variant = "info";
      var box = el("div", "ui-callout ui-callout-" + variant);
      var title = str(p.title);
      if (title) box.appendChild(el("div", "ui-callout-title", title));
      box.appendChild(el("div", "ui-callout-text", str(p.text)));
      return box;
    }
  });

  // progress shows a single 0..max completion value.
  register("progress", {
    children: false,
    render: function (node) {
      var p = node.props;
      var max = num(p.max, 100);
      if (max <= 0) max = 100;
      var value = num(p.value, 0);
      if (value < 0) value = 0;
      if (value > max) value = max;
      var pct = Math.round((value / max) * 100);
      var box = el("div", "ui-progress");
      var head = el("div", "ui-progress-head");
      head.appendChild(el("span", "ui-progress-label", str(p.label)));
      head.appendChild(el("span", "ui-progress-value", pct + "%"));
      box.appendChild(head);
      var track = el("div", "ui-progress-track");
      var fill = el("div", "ui-progress-fill");
      fill.style.width = pct + "%";
      track.appendChild(fill);
      box.appendChild(track);
      return box;
    }
  });

  // ---------------------------------------------------------------------------
  // Stage 2 components: table / steps / file
  // ---------------------------------------------------------------------------

  // alignClass maps a column's align prop to the alignment class.
  function alignClass(align) {
    if (align === "right") return "ui-align-right";
    if (align === "center") return "ui-align-center";
    return "";
  }

  // tableCell renders one cell: the value is a scalar, or an object {v, status}
  // whose status (ok|warn|err) tints the cell. Any other status is ignored so a
  // hostile value cannot inject a class name.
  function tableCell(row, col) {
    var raw = row[col.key];
    var status = "";
    var text;
    if (raw && typeof raw === "object") {
      if (["ok", "warn", "err"].indexOf(raw.status) >= 0) status = raw.status;
      text = str(raw.v);
    } else {
      text = str(raw);
    }
    var cls = [alignClass(col.align), status ? "ui-td-status ui-td-" + status : ""].filter(Boolean).join(" ");
    return el("td", cls, text);
  }

  // table renders tabular data with a header row. An empty row list renders a
  // dedicated empty state instead of an empty tbody (UC-18).
  register("table", {
    children: false,
    render: function (node) {
      var p = node.props;
      var cols = Array.isArray(p.columns) ? p.columns.slice() : [];
      var rows = Array.isArray(p.rows) ? p.rows : [];
      var box = el("div", "ui-table-wrap");
      if (!rows.length) {
        box.appendChild(el("div", "ui-empty", uiText("暂无数据", "No data")));
        return box;
      }
      // No column declaration: derive them from the first row's keys.
      if (!cols.length && rows[0] && typeof rows[0] === "object") {
        cols = Object.keys(rows[0]).map(function (k) { return { key: k }; });
      }
      var table = el("table", "ui-table");
      var thead = document.createElement("thead");
      var headRow = document.createElement("tr");
      for (var i = 0; i < cols.length; i++) {
        var c = cols[i] || {};
        var label = c.label === undefined || c.label === null ? str(c.key) : str(c.label);
        var th = el("th", alignClass(c.align), label);
        if (c.width) th.style.width = typeof c.width === "number" ? c.width + "px" : String(c.width);
        headRow.appendChild(th);
      }
      thead.appendChild(headRow);
      table.appendChild(thead);
      var tbody = document.createElement("tbody");
      for (var r = 0; r < rows.length; r++) {
        var tr = document.createElement("tr");
        for (var k = 0; k < cols.length; k++) tr.appendChild(tableCell(rows[r] || {}, cols[k] || {}));
        tbody.appendChild(tr);
      }
      table.appendChild(tbody);
      box.appendChild(table);
      return box;
    }
  });

  // steps renders a vertical timeline; status (done|active|pending) drives the
  // dot styling and whether a connector line segment is drawn.
  register("steps", {
    children: false,
    render: function (node) {
      var items = Array.isArray(node.props.items) ? node.props.items : [];
      var list = el("ol", "ui-steps");
      if (!items.length) {
        list.appendChild(el("li", "ui-empty", uiText("暂无步骤", "No steps")));
        return list;
      }
      for (var i = 0; i < items.length; i++) {
        var it = items[i] && typeof items[i] === "object" ? items[i] : {};
        var st = ["done", "active", "pending"].indexOf(it.status) >= 0 ? it.status : "pending";
        var li = el("li", "ui-step ui-step-" + st);
        li.appendChild(el("span", "ui-step-dot", st === "done" ? "\u2713" : ""));
        var bodyEl = el("div", "ui-step-body");
        bodyEl.appendChild(el("div", "ui-step-title", str(it.title)));
        var desc = str(it.desc);
        if (desc) bodyEl.appendChild(el("div", "ui-step-desc", desc));
        li.appendChild(bodyEl);
        list.appendChild(li);
      }
      return list;
    }
  });

  // basename returns the last path segment.
  function basename(p) {
    var s = String(p || "");
    var i = s.lastIndexOf("/");
    return i >= 0 ? s.slice(i + 1) : s;
  }

  // safeWorkspacePath rejects absolute paths and any ".." segment: a file card
  // must never point outside the workspace (UC-25).
  function safeWorkspacePath(p) {
    var s = String(p || "").replace(/\\/g, "/").trim();
    if (!s || s.charAt(0) === "/" || /^[a-zA-Z]:/.test(s)) return null;
    var parts = s.split("/");
    for (var i = 0; i < parts.length; i++) {
      if (parts[i] === "..") return null;
    }
    return s;
  }

  // fmtBytes renders a byte count as a human size; an already formatted size
  // string passes through unchanged.
  function fmtBytes(size) {
    if (size === undefined || size === null || size === "") return "";
    if (typeof size === "string" && !/^[0-9.]+$/.test(size)) return size;
    var n = typeof size === "number" ? size : parseFloat(size);
    if (!isFinite(n) || n < 0) return "";
    if (n < 1024) return n + " B";
    if (n < 1024 * 1024) return (n / 1024).toFixed(1) + " KB";
    return (n / (1024 * 1024)).toFixed(1) + " MB";
  }

  // fileActionButton builds one open/reveal button reusing the app.js
  // postPath helper (the same endpoints the workspace tree uses).
  function fileActionButton(label, path, endpoint, ghost) {
    var b = document.createElement("button");
    b.className = "btn sm" + (ghost ? " ghost" : "");
    b.textContent = label;
    b.title = label;
    b.onclick = function (e) {
      e.stopPropagation();
      if (typeof global.postPath === "function") global.postPath(endpoint, path);
    };
    return b;
  }

  // file renders a file card with open/reveal actions. A path that escapes the
  // workspace degrades to plain text and never gets action buttons.
  register("file", {
    children: false,
    render: function (node) {
      var p = node.props;
      var path = str(p.path);
      var safe = safeWorkspacePath(path);
      var box = el("div", "ui-file");
      box.appendChild(el("span", "ui-file-icon", "\uD83D\uDCC4"));
      var meta = el("div", "ui-file-meta");
      meta.appendChild(el("div", "ui-file-name", str(p.name) || basename(path)));
      meta.appendChild(el("div", "ui-file-path", path));
      box.appendChild(meta);
      var size = fmtBytes(p.size);
      if (size) box.appendChild(el("span", "ui-file-size", size));
      if (!safe) {
        meta.appendChild(el("div", "ui-file-rejected", uiText(
          "路径超出工作区，已降级为纯文本展示。",
          "Path is outside the workspace; showing plain text only."
        )));
        return box;
      }
      var actions = el("div", "ui-file-actions");
      actions.appendChild(fileActionButton(uiText("打开", "Open"), safe, "api/open", false));
      actions.appendChild(fileActionButton(uiText("定位", "Reveal"), safe, "api/reveal", true));
      box.appendChild(actions);
      return box;
    }
  });

  global.UI = {
    version: "1",
    register: register,
    has: hasRenderer,
    types: registeredTypes,
    renderTree: renderTree,
    updateTree: updateTree,
    findByUIID: findByUIID,
    // t/fmtBytes are shared with ui-chart.js (loaded after this file).
    t: uiText,
    fmtBytes: fmtBytes
  };
})(window);

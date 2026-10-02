// co-shell Web UI - export of render_ui component trees (FEATURE-580).
//
// render_ui paints a declarative component tree into the chat stream block or
// into a window (float/panel). This file gives that rendered content an exit:
// a "Markdown download" and a "PDF / print" entry on every render_ui block and
// every window, so the user can download, keep and print the whole result.
//
// Two design rules keep it dependency-free and faithful:
//   1. Markdown is generated from the component tree directly (no DOM scraping),
//      so the text is stable, complete and language-aware.
//   2. The print view re-renders the very same tree through UI.renderTree into a
//      print-only container pinned to the light paper palette: charts and
//      figures stay vector graphics, and the sheet stays ink-friendly whatever
//      theme the app itself is using.
//
// SECURITY RULE (same as ui.js / md.js): never assign innerHTML, never build
// HTML by string concatenation. Every element is created with createElement and
// every dynamic string goes through textContent; the Markdown writer escapes
// "<" and ">" so a hostile component prop can never inject markup.
//
// Author: L.Shuang
// Created: 2026-10-02
// MIT License - Copyright (c) 2026 L.Shuang

(function (global) {
  "use strict";

  var PRINT_ROOT_CLASS = "ui-print-root";
  var MAX_DEPTH = 12;

  // ---------------------------------------------------------------------------
  // Language
  // ---------------------------------------------------------------------------

  // STRINGS holds every user-visible text of this module, zh first (the UI
  // default), en second. text() resolves them lazily, so a language switch
  // applies to the next export without any re-registration.
  var STRINGS = {
    export: ["导出", "Export"],
    md: ["Markdown 下载", "Download Markdown"],
    pdf: ["PDF / 打印", "PDF / Print"],
    titleDefault: ["render_ui 导出", "render_ui export"],
    brand: ["co-shell · ", "co-shell · "],
    label: ["标签", "Label"],
    value: ["值", "Value"],
    valueUnit: ["值（单位：{0}）", "Value (unit: {0})"],
    figureNote: ["（矢量图，请在网页或 PDF 中查看）", "(Vector figure - see the web page or the PDF export)"],
    htmlNote: ["（内嵌 HTML 内容，请在网页或 PDF 中查看）", "(Embedded HTML content - see the web page or the PDF export)"],
    badTree: ["组件树无法解析，已取消导出。", "The component tree could not be parsed; export cancelled."],
    noRenderer: ["当前界面不支持渲染组件，无法导出。", "This interface cannot render components, so it cannot export."]
  };

  // uiText picks the string of the active UI language (currentLang is the
  // app.js global, read lazily at call time - same contract as ui.js).
  function uiText(zh, en) {
    var lang = typeof global.currentLang === "string" ? global.currentLang : "zh";
    return lang === "en" ? en : zh;
  }

  // text resolves one STRINGS entry, substituting {0}, {1} … placeholders.
  function text(key) {
    var pair = STRINGS[key];
    if (!pair) return key;
    var s = uiText(pair[0], pair[1]);
    for (var i = 1; i < arguments.length; i++) {
      s = s.split("{" + (i - 1) + "}").join(String(arguments[i]));
    }
    return s;
  }

  // ---------------------------------------------------------------------------
  // Small value helpers
  // ---------------------------------------------------------------------------

  // valueText renders any prop value as plain text (objects become JSON).
  function valueText(v) {
    if (v === undefined || v === null) return "";
    if (typeof v === "object") {
      try {
        return JSON.stringify(v);
      } catch (err) {
        return String(v);
      }
    }
    return String(v);
  }

  // esc renders a value as Markdown-safe inline text: newlines collapse (a
  // value must never break the block structure), "<" and ">" are escaped so no
  // raw tag can survive into the file, and "|" is escaped so table cells hold.
  function esc(v) {
    return valueText(v)
      .replace(/\r?\n/g, " ")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/\|/g, "\\|");
  }

  // numOr reads a finite number from a prop, falling back to def.
  function numOr(v, def) {
    var n = typeof v === "number" ? v : parseFloat(v);
    return isFinite(n) ? n : def;
  }

  // listOf returns prop as an array (never null).
  function listOf(v) {
    return Array.isArray(v) ? v : [];
  }

  // parseTree accepts the tree as an object (live event) or as JSON text
  // (persisted/replayed event) and returns the root node, or null.
  function parseTree(raw) {
    if (raw === null || raw === undefined) return null;
    var node = raw;
    if (typeof node === "string") {
      var s = node.trim();
      if (!s) return null;
      try {
        node = JSON.parse(s);
      } catch (err) {
        return null;
      }
    }
    if (!node || typeof node !== "object" || Array.isArray(node)) return null;
    return node;
  }

  function pad2(n) {
    return n < 10 ? "0" + n : String(n);
  }

  // humanTime formats a timestamp for the export header.
  function humanTime(d) {
    return d.getFullYear() + "-" + pad2(d.getMonth() + 1) + "-" + pad2(d.getDate()) +
      " " + pad2(d.getHours()) + ":" + pad2(d.getMinutes());
  }

  // stamp formats a timestamp for the file name (no separators, no colons).
  function stamp(d) {
    return String(d.getFullYear()) + pad2(d.getMonth() + 1) + pad2(d.getDate()) +
      "-" + pad2(d.getHours()) + pad2(d.getMinutes());
  }

  // ---------------------------------------------------------------------------
  // Component tree -> Markdown
  // ---------------------------------------------------------------------------

  // renderNode appends the Markdown blocks of one node to out. Every block is
  // one string; the caller joins them with a blank line, which is what makes
  // lists stay tight and sections stay separated.
  function renderNode(node, out, depth) {
    if (!node || typeof node !== "object" || depth > MAX_DEPTH) return;
    var type = typeof node.type === "string" ? node.type : "";
    var p = node.props && typeof node.props === "object" ? node.props : {};

    switch (type) {
      case "card": {
        if (p.title) out.push("### " + esc(p.title));
        if (p.subtitle) out.push("*" + esc(p.subtitle) + "*");
        renderChildren(node, out, depth);
        return;
      }
      case "row":
      case "col":
        renderChildren(node, out, depth);
        return;
      case "kv": {
        var kvLines = listOf(p.items).map(function (it) {
          return "- **" + esc((it && it.k) || "") + "**: " + esc(it && it.v);
        });
        if (kvLines.length) out.push(kvLines.join("\n"));
        return;
      }
      case "table": {
        var cols = listOf(p.columns);
        var rows = listOf(p.rows);
        if (!cols.length && rows.length) {
          cols = Object.keys(rows[0] || {}).map(function (k) { return { key: k }; });
        }
        if (!cols.length) return;
        var head = "| " + cols.map(function (c) { return esc((c && c.label) || (c && c.key)); }).join(" | ") + " |";
        var sep = "| " + cols.map(function () { return "---"; }).join(" | ") + " |";
        var body = rows.map(function (r) {
          return "| " + cols.map(function (c) {
            return esc((r || {})[c && c.key]);
          }).join(" | ") + " |";
        });
        out.push([head, sep].concat(body).join("\n"));
        return;
      }
      case "chart": {
        var series = listOf(p.series);
        var unit = p.unit ? String(p.unit) : "";
        var valHead = unit ? text("valueUnit", unit) : text("value");
        if (!series.length) {
          out.push("#### " + esc(p.title || "chart"));
          return;
        }
        for (var i = 0; i < series.length; i++) {
          var s = series[i] || {};
          out.push("#### " + esc(s.name || p.title || "chart"));
          var data = listOf(s.data);
          var lines = ["| " + text("label") + " | " + valHead + " |", "| --- | --- |"];
          for (var j = 0; j < data.length; j++) {
            lines.push("| " + esc(data[j] && data[j].label) + " | " + esc(data[j] && data[j].value) + " |");
          }
          out.push(lines.join("\n"));
        }
        return;
      }
      case "steps": {
        var items = listOf(p.items);
        var stepLines = items.map(function (it, idx) {
          var status = (it && it.status) || "pending";
          var mark = status === "done" ? "✅" : (status === "active" ? "🔄" : "⬜");
          var line = (idx + 1) + ". " + mark + " " + esc(it && it.title);
          if (it && it.desc) line += " — " + esc(it.desc);
          return line;
        });
        if (stepLines.length) out.push(stepLines.join("\n"));
        return;
      }
      case "callout": {
        var marks = { info: "ℹ️", warn: "⚠️", success: "✅", error: "❌" };
        var mark = marks[p.variant] || marks.info;
        var quote = ["> " + mark + (p.title ? " **" + esc(p.title) + "**" : "")];
        if (p.text) quote.push("> " + esc(p.text));
        out.push(quote.join("\n"));
        return;
      }
      case "progress": {
        var max = numOr(p.max, 100);
        var val = numOr(p.value, 0);
        var pct = max > 0 ? Math.round((val / max) * 100) : 0;
        out.push("**" + esc(p.label || "progress") + "**: " + val + "/" + max + " (" + pct + "%)");
        return;
      }
      case "file": {
        var file = "- 📄 " + esc(p.path);
        var extra = [];
        if (p.name) extra.push(esc(p.name));
        if (p.size) extra.push(esc(p.size));
        if (extra.length) file += "（" + extra.join("，") + "）";
        out.push(file);
        return;
      }
      case "form": {
        if (p.title) out.push("### " + esc(p.title));
        var fieldLines = listOf(p.fields).map(function (f) {
          var label = (f && f.label) || (f && f.name) || "";
          var kind = (f && f.type) || "text";
          return "- **" + esc(label) + "**（" + esc(kind) + "）: " + esc(f && f.value);
        });
        if (fieldLines.length) out.push(fieldLines.join("\n"));
        return;
      }
      case "figure":
        // A vector canvas has no Markdown equivalent: keep the caption and tell
        // the reader where the real picture is (the web page or the PDF export).
        out.push("**" + esc(p.title || "figure") + "**");
        out.push("*" + text("figureNote") + "*");
        return;
      case "html":
        out.push("*" + text("htmlNote") + "*");
        return;
      default:
        // Unknown type: keep the type visible and descend into the children so
        // nothing silently disappears (the Go side rejects unknown types, so
        // this only guards hand-written or replayed trees).
        out.push("**" + esc(type || "unknown") + "**");
        renderChildren(node, out, depth);
        return;
    }
  }

  // renderChildren renders node.children in order.
  function renderChildren(node, out, depth) {
    var kids = listOf(node.children);
    for (var i = 0; i < kids.length; i++) renderNode(kids[i], out, depth + 1);
  }

  // treeToMarkdown renders a component tree as a Markdown document: the export
  // header (title + timestamp) followed by the component blocks. opts.title and
  // opts.time are optional; the timestamp defaults to "now" and is injected by
  // the browser tests for deterministic assertions.
  function treeToMarkdown(tree, opts) {
    opts = opts || {};
    var blocks = [];
    var root = parseTree(tree);
    if (root) renderNode(root, blocks, 0);
    var head = "# " + esc(opts.title || text("titleDefault")) +
      "\n\n*" + text("brand") + (opts.time || humanTime(new Date())) + "*";
    return head + (blocks.length ? "\n\n" + blocks.join("\n\n") : "") + "\n";
  }

  // ---------------------------------------------------------------------------
  // File name + download
  // ---------------------------------------------------------------------------

  // sanitizeFile turns a title into a safe file-name component: path separators,
  // the characters Windows forbids and whitespace all collapse into "_".
  function sanitizeFile(s) {
    return String(s).replace(/[\\/:*?"<>|\s]+/g, "_").replace(/^_+|_+$/g, "");
  }

  // fileName builds "<title>-<yyyyMMdd-HHmm>.<ext>". The title is capped so a
  // runaway title can never produce an unusable name.
  function fileName(opts, ext) {
    opts = opts || {};
    var base = sanitizeFile(opts.title || "").slice(0, 60) || "co-shell-export";
    return base + "-" + (opts.stamp || stamp(opts.date || new Date())) + "." + (ext || "md");
  }

  // lastExportText is the Markdown of the most recent export. lastExport()
  // exposes it so a test (or a future "copy as Markdown" action) can read what
  // was built without downloading it.
  var lastExportText = "";

  function lastExport() {
    return lastExportText;
  }

  // saveText hands content to the browser as a download.
  function saveText(content, name) {
    var blob = new Blob([content], { type: "text/markdown;charset=utf-8" });
    var url = URL.createObjectURL(blob);
    var a = document.createElement("a");
    a.href = url;
    a.download = name;
    a.style.display = "none";
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    setTimeout(function () { URL.revokeObjectURL(url); }, 1000);
  }

  // downloadMarkdown builds the Markdown document and saves it as a .md file.
  function downloadMarkdown(tree, opts) {
    opts = opts || {};
    var md = treeToMarkdown(tree, opts);
    lastExportText = md;
    saveText(md, fileName(opts, "md"));
    return md;
  }

  // ---------------------------------------------------------------------------
  // Print view (PDF through the browser's own print dialog)
  // ---------------------------------------------------------------------------

  // buildPrintHead builds the export header: the title plus the co-shell mark
  // and the export timestamp, so a saved PDF is self-describing.
  function buildPrintHead(title, time) {
    var head = document.createElement("div");
    head.className = "ui-print-head";
    var h1 = document.createElement("h1");
    h1.className = "ui-print-title";
    h1.textContent = title;
    head.appendChild(h1);
    var meta = document.createElement("div");
    meta.className = "ui-print-meta";
    meta.textContent = text("brand") + time;
    head.appendChild(meta);
    return head;
  }

  // cleanupPrint removes every print container (afterprint, or a stale one left
  // behind by an aborted print).
  function cleanupPrint() {
    var nodes = document.querySelectorAll("." + PRINT_ROOT_CLASS);
    for (var i = 0; i < nodes.length; i++) {
      if (nodes[i].parentNode) nodes[i].parentNode.removeChild(nodes[i]);
    }
  }

  function onAfterPrint() {
    global.removeEventListener("afterprint", onAfterPrint);
    cleanupPrint();
    closeMenu();
  }

  // printTree renders the tree into a print-only container and opens the print
  // dialog (print directly or save as PDF). Two properties matter:
  //   - the container is pinned to the light paper palette, so the sheet stays
  //     readable and ink-friendly whatever theme the app itself uses;
  //   - it is a plain direct child of <body>, and the print stylesheet hides
  //     every other child, so only the exported content reaches the paper.
  // The container is removed on afterprint, leaving the page untouched.
  function printTree(tree, opts) {
    opts = opts || {};
    var root = parseTree(tree);
    if (!root) {
      console.warn(text("badTree"));
      return false;
    }
    if (!global.UI || typeof global.UI.renderTree !== "function") {
      console.warn(text("noRenderer"));
      return false;
    }
    closeMenu();
    cleanupPrint();
    var node = document.createElement("div");
    node.className = PRINT_ROOT_CLASS;
    // [data-theme] is a plain attribute selector in style.css, so setting it
    // here re-points the CSS variables for this subtree only.
    node.setAttribute("data-theme", "light");
    node.appendChild(buildPrintHead(opts.title || text("titleDefault"), opts.time || humanTime(new Date())));
    var host = document.createElement("div");
    host.className = "ui-tree ui-print-body";
    node.appendChild(host);
    document.body.appendChild(node);
    global.UI.renderTree(root, host);
    global.addEventListener("afterprint", onAfterPrint);
    global.print();
    return true;
  }

  // ---------------------------------------------------------------------------
  // Format menu
  // ---------------------------------------------------------------------------

  var menuEl = null;
  var menuAnchor = null;

  function closeMenu() {
    if (menuEl && menuEl.parentNode) menuEl.parentNode.removeChild(menuEl);
    if (menuAnchor) menuAnchor.setAttribute("aria-expanded", "false");
    menuEl = null;
    menuAnchor = null;
    document.removeEventListener("click", onDocClick, true);
    document.removeEventListener("keydown", onDocKey, true);
    global.removeEventListener("resize", closeMenu);
  }

  function onDocClick(e) {
    if (menuEl && menuEl.contains(e.target)) return;
    if (menuAnchor && menuAnchor.contains(e.target)) return;
    // Another export button takes over; its own handler re-opens the menu.
    if (e.target && e.target.closest && e.target.closest("[data-ui-export]")) return;
    closeMenu();
  }

  function onDocKey(e) {
    if (e.key === "Escape") closeMenu();
  }

  // openMenu shows the format menu under anchor. The listeners are installed
  // now (not on a timer): this click is already past the capture phase, so the
  // fresh listener cannot close the menu it just opened.
  function openMenu(anchor, items) {
    closeMenu();
    var menu = document.createElement("div");
    menu.className = "ui-export-menu";
    menu.setAttribute("role", "menu");
    for (var i = 0; i < items.length; i++) {
      (function (item) {
        var b = document.createElement("button");
        b.type = "button";
        b.className = "ui-export-item";
        b.setAttribute("role", "menuitem");
        b.setAttribute("data-ui-export-item", item.id);
        b.textContent = item.label;
        b.addEventListener("click", function (e) {
          e.stopPropagation();
          closeMenu();
          item.run();
        });
        menu.appendChild(b);
      })(items[i]);
    }
    document.body.appendChild(menu);
    var rect = anchor.getBoundingClientRect();
    var maxLeft = Math.max(8, global.innerWidth - menu.offsetWidth - 8);
    menu.style.left = Math.max(8, Math.min(rect.left, maxLeft)) + "px";
    menu.style.top = Math.max(8, rect.bottom + 4) + "px";
    menuEl = menu;
    menuAnchor = anchor;
    anchor.setAttribute("aria-expanded", "true");
    document.addEventListener("click", onDocClick, true);
    document.addEventListener("keydown", onDocKey, true);
    global.addEventListener("resize", closeMenu);
    return menu;
  }

  // exportItems builds the menu of one export context: Markdown download and
  // PDF/print. Word (.docx) is planned for a follow-up.
  function exportItems(ctx) {
    return [
      {
        id: "md",
        label: text("md"),
        run: function () { downloadMarkdown(ctx.tree, { title: ctx.title }); }
      },
      {
        id: "pdf",
        label: text("pdf"),
        run: function () { printTree(ctx.tree, { title: ctx.title }); }
      }
    ];
  }

  // ---------------------------------------------------------------------------
  // Entries: stream blocks and windows
  // ---------------------------------------------------------------------------

  // blockTitle derives a document title for a stream block: the root component's
  // own title when it has one, otherwise the block's title-bar text. The
  // ui_render event carries no LLM intent, so this is what is available.
  function blockTitle(tree, blockEl) {
    var root = parseTree(tree);
    if (root && root.props && root.props.title) return String(root.props.title);
    var label = blockEl && blockEl.querySelector(".ev-head-label");
    return label ? label.textContent.trim() : "";
  }

  // attachBlock adds the export entry to a render_ui block (meta is the
  // ui_render event meta, whose ui_tree holds the tree of this block).
  function attachBlock(blockEl, meta) {
    if (!blockEl || !meta) return null;
    var head = blockEl.querySelector(".ev-head");
    if (!head || head.querySelector("[data-ui-export]")) return null;
    var btn = document.createElement("button");
    btn.type = "button";
    btn.className = "ev-act ui-export-btn";
    btn.setAttribute("data-ui-export", "block");
    btn.setAttribute("aria-expanded", "false");
    btn.title = text("export");
    btn.textContent = text("export");
    btn.addEventListener("click", function (e) {
      e.stopPropagation();
      var tree = meta.ui_tree || "";
      openMenu(btn, exportItems({ tree: tree, title: blockTitle(tree, blockEl) }));
    });
    var actions = head.querySelector(".ev-actions");
    if (actions) actions.insertBefore(btn, actions.firstChild);
    else head.appendChild(btn);
    return btn;
  }

  // windowContext resolves the export context of the window owning el. A float
  // shell carries data-window-name; the panel head is shared by every window, so
  // it exports the active one (an empty name means "active" to UIWin).
  function windowContext(el) {
    if (!global.UIWin || typeof global.UIWin.exportPayload !== "function") return null;
    var shell = el && el.closest ? el.closest("[data-window-name]") : null;
    var name = (shell && shell.getAttribute("data-window-name")) || "";
    return global.UIWin.exportPayload(name);
  }

  // bindWindowEntries wires the export buttons of the window shell and the panel
  // head (both are markup in index.html and are cloned per window, so one
  // document-level handler covers every window).
  function bindWindowEntries() {
    document.addEventListener("click", function (e) {
      var btn = e.target && e.target.closest ? e.target.closest("[data-ui-export='window']") : null;
      if (!btn) return;
      e.stopPropagation();
      var ctx = windowContext(btn);
      if (!ctx) return;
      openMenu(btn, exportItems(ctx));
    }, false);
  }

  global.UIExport = {
    version: "1",
    // i18n + conversion
    text: text,
    treeToMarkdown: treeToMarkdown,
    // export actions
    fileName: fileName,
    downloadMarkdown: downloadMarkdown,
    printTree: printTree,
    lastExport: lastExport,
    // entries
    attachBlock: attachBlock,
    closeMenu: closeMenu,
    printRootClass: PRINT_ROOT_CLASS
  };

  bindWindowEntries();
})(window);

"use strict";

/* Minimal safe Markdown subset renderer (FEATURE-362).
 *
 * Builds DOM nodes directly and never feeds raw input to innerHTML, so
 * rendered output cannot inject markup. Supported: fenced code blocks,
 * ATX headings, pipe tables, blockquotes, horizontal rules, unordered /
 * ordered lists, paragraphs, and inline code / bold / italic /
 * strikethrough / links.
 *
 * The whole accumulated text is re-parsed on every call, which keeps
 * streaming trivially correct: unterminated constructs (open code fence,
 * dangling `**`) simply render as their incomplete form and fix
 * themselves once the closing token arrives.
 */

function mdRender(container, text) {
  container.textContent = "";
  for (const n of mdBlocks(text)) container.appendChild(n);
}

/* ---------- block level ---------- */

function mdBlocks(text) {
  const lines = String(text).split("\n");
  const out = [];
  let i = 0;
  const blank = (l) => /^\s*$/.test(l);

  while (i < lines.length) {
    const line = lines[i];
    if (blank(line)) { i++; continue; }

    // fenced code block (``` or ~~~, closed or still open while streaming)
    let m = line.match(/^\s{0,3}(```+|~~~+)\s*([^\s]*)?.*$/);
    if (m) {
      const fence = m[1][0].repeat(3);
      const lang = (m[2] || "").trim();
      const buf = [];
      i++;
      while (i < lines.length && !lines[i].match(new RegExp("^\\s{0,3}" + (fence[0] === "`" ? "```" : "~~~")))) {
        buf.push(lines[i]); i++;
      }
      i++; // skip the closing fence (harmless at EOF)
      const pre = document.createElement("pre");
      const code = document.createElement("code");
      if (lang) code.className = "lang-" + lang;
      // FEATURE-576: paths inside a fenced block are clickable as well — command
      // output is often shown in a fence.
      for (const n of pathLinkNodes(buf.join("\n"))) code.appendChild(n);
      pre.appendChild(code);
      pre.dataset.line = i + 1;
      out.push(pre);
      continue;
    }

    // ATX heading
    m = line.match(/^\s{0,3}(#{1,6})\s+(.*)$/);
    if (m) {
      const h = document.createElement("h" + Math.min(m[1].length + 2, 6)); // h1 renders as h3: keep in-stream scale
      h.dataset.level = m[1].length;
      h.dataset.line = i + 1;
      for (const n of mdInline(m[2])) h.appendChild(n);
      out.push(h);
      i++;
      continue;
    }

    // horizontal rule
    if (/^\s{0,3}([-*_])(\s*\1){2,}\s*$/.test(line)) {
      const hr = document.createElement("hr");
      hr.dataset.line = i + 1;
      out.push(hr);
      i++;
      continue;
    }

    // pipe table: header row + separator row (|---|---|)
    if (line.indexOf("|") >= 0 && i + 1 < lines.length && /^\s*\|?[\s:|-]+\|?\s*$/.test(lines[i + 1]) && lines[i + 1].indexOf("-") >= 0) {
      const rows = [];
      const splitRow = (l) => {
        let cells = l.trim().replace(/^\|/, "").replace(/\|$/, "").split("|");
        return cells.map((c) => c.trim());
      };
      const head = splitRow(line);
      i += 2;
      while (i < lines.length && !blank(lines[i]) && lines[i].indexOf("|") >= 0) {
        rows.push(splitRow(lines[i])); i++;
      }
      const table = document.createElement("table");
      const thead = document.createElement("thead");
      const htr = document.createElement("tr");
      for (const c of head) {
        const th = document.createElement("th");
        for (const n of mdInline(c)) th.appendChild(n);
        htr.appendChild(th);
      }
      thead.appendChild(htr);
      table.appendChild(thead);
      const tbody = document.createElement("tbody");
      for (const r of rows) {
        const tr = document.createElement("tr");
        for (let k = 0; k < head.length; k++) {
          const td = document.createElement("td");
          for (const n of mdInline(r[k] || "")) td.appendChild(n);
          tr.appendChild(td);
        }
        tbody.appendChild(tr);
      }
      table.appendChild(tbody);
      table.dataset.line = i + 1;
      out.push(table);
      continue;
    }

    // blockquote (consecutive > lines, recursively parsed)
    if (/^\s{0,3}>/.test(line)) {
      const buf = [];
      while (i < lines.length && /^\s{0,3}>/.test(lines[i])) {
        buf.push(lines[i].replace(/^\s{0,3}>\s?/, "")); i++;
      }
      const bq = document.createElement("blockquote");
      bq.dataset.line = i + 1;
      for (const n of mdBlocks(buf.join("\n"))) bq.appendChild(n);
      out.push(bq);
      continue;
    }

    // unordered / ordered list (single level, consecutive items)
    m = line.match(/^\s{0,3}([-*+]|\d+[.)])\s+(.*)$/);
    if (m) {
      const ordered = /^\d/.test(m[1]);
      const list = document.createElement(ordered ? "ol" : "ul");
      list.dataset.line = i + 1;
      // FEATURE-443: when an ordered list is interrupted by other content
      // (e.g. a nested sub-list or a paragraph), the following items are
      // rendered as a separate <ol>. Without an explicit start, each <ol>
      // restarts at 1, so source items "1." and "2." both render as "1.".
      // Parse the source number of the first item and set <ol start=N> so
      // numbering continues from the source sequence.
      if (ordered) {
        const numMatch = m[1].match(/\d+/);
        if (numMatch) list.start = parseInt(numMatch[0], 10);
      }
      const itemRe = ordered ? /^\s{0,3}\d+[.)]\s+(.*)$/ : /^\s{0,3}[-*+]\s+(.*)$/;
      while (i < lines.length) {
        const im = lines[i].match(itemRe);
        if (!im) break;
        const li = document.createElement("li");
        for (const n of mdInline(im[1])) li.appendChild(n);
        list.appendChild(li);
        i++;
      }
      out.push(list);
      continue;
    }

    // paragraph: consecutive lines until blank or another block starter
    const buf = [];
    while (i < lines.length && !blank(lines[i])) {
      const l = lines[i];
      if (buf.length > 0 && (
        /^\s{0,3}(```+|~~~+)/.test(l) ||
        /^\s{0,3}#{1,6}\s+/.test(l) ||
        /^\s{0,3}>/.test(l) ||
        /^\s{0,3}([-*+]|\d+[.)])\s+/.test(l) ||
        /^\s{0,3}([-*_])(\s*\1){2,}\s*$/.test(l)
      )) break;
      buf.push(l); i++;
    }
    const p = document.createElement("p");
    p.dataset.line = i + 1;
    buf.forEach((l, idx) => {
      if (idx > 0) p.appendChild(document.createElement("br"));
      for (const n of mdInline(l)) p.appendChild(n);
    });
    out.push(p);
  }
  return out;
}

/* ---------- path links (FEATURE-576) ---------- */

// MD_PATH_RE matches one path-like candidate: an optional leading "/" (an
// absolute path), one or more "dir/" components and a final component which
// may carry a ":line[:col]" suffix (the colon is allowed there only).
//
// Whitespace, the path separator and every punctuation mark used as prose or
// markdown punctuation — including the CJK forms "、" "," "：" "。" "（）" "“”" —
// are NOT part of a component. That is what makes "a.md、b.js" one path plus
// one separator plus another path (instead of one bogus glued path) and lets
// "：/Users/x/a.js" and "./main.go" start cleanly. All repetitions are bounded so
// a very long unbroken run (base64 blob, big JSON) cannot make the scan
// quadratic. Validation/trimming happens in pathLinkNodes.
const MD_PATH_RE = /(?<![A-Za-z0-9_\-.\/])(\/?(?:[^\s\/:：、，。；！？（）【】「」『』《》〈〉“”‘’…—·,;!?()\[\]{}<>|*"'`=#]{1,60}\/){1,20}[^\s：、，。；！？（）【】「」『』《》〈〉“”‘’…—·,;!?()\[\]{}<>|*"'`=#]{1,80})/g;

// pathLinkNodes splits text into plain text nodes plus, for every local file
// path whose type the file previewer supports, one clickable <a.md-path>.
//
// The label keeps the original text (line suffix included, so "a.go:12" reads
// naturally); the previewable workspace-relative path lives in dataset.path
// and the click is handled in JS, so a hostile path string can never reach an
// href and become a URL/script scheme. window.previewPathOf/previewPathOf and
// window.openPathPreview/openPathPreview come from app.js — the file
// previewer's single source of truth. When they are absent (md.js used
// stand-alone, e.g. in a unit test) the text renders unchanged.
function pathLinkNodes(text) {
  const out = [];
  const s = String(text);
  let last = 0;
  let m;
  MD_PATH_RE.lastIndex = 0;
  while ((m = MD_PATH_RE.exec(s))) {
    const target = window.previewPathOf ? window.previewPathOf(m[1]) : null;
    if (!target) continue;
    if (m.index > last) out.push(document.createTextNode(s.slice(last, m.index)));
    const a = document.createElement("a");
    a.className = "md-path";
    a.dataset.path = target.path;
    if (target.line > 0) a.dataset.line = String(target.line);
    a.textContent = m[1];
    a.title = target.path;
    a.onclick = (e) => {
      e.preventDefault();
      if (window.openPathPreview) window.openPathPreview(target.path, target.line);
    };
    out.push(a);
    last = MD_PATH_RE.lastIndex;
  }
  if (last < s.length) out.push(document.createTextNode(s.slice(last)));
  return out;
}

// pushPathNodes appends pathLinkNodes' output to an existing node collector.
function pushPathNodes(nodes, text) {
  for (const n of pathLinkNodes(text)) nodes.push(n);
}

/* ---------- inline level ---------- */

// Order matters: code spans first (no formatting inside), then bold,
// strikethrough, italic, links. `_emphasis_` requires non-word neighbors
// so snake_case identifiers are left alone.
const MD_INLINE_RE = /(`+)([^`]+?)\1|\*\*([^*]+?)\*\*|__([^_]+?)__|~~([^~\n]+?)~~|\*([^*\n]+?)\*|(?<![A-Za-z0-9])_([^_\n]+?)_(?![A-Za-z0-9])|\[([^\]\n]+?)\]\(([^)\s]+?)\)/g;

function mdInline(text) {
  const nodes = [];
  let last = 0;
  let m;
  MD_INLINE_RE.lastIndex = 0;
  while ((m = MD_INLINE_RE.exec(text))) {
    if (m.index > last) pushPathNodes(nodes, text.slice(last, m.index));
    if (m[2] !== undefined) {                       // `code`
      const c = document.createElement("code");
      for (const n of pathLinkNodes(m[2])) c.appendChild(n);
      nodes.push(c);
    } else if (m[3] !== undefined || m[4] !== undefined) { // **bold** / __bold__
      const b = document.createElement("strong");
      for (const n of pathLinkNodes(m[3] !== undefined ? m[3] : m[4])) b.appendChild(n);
      nodes.push(b);
    } else if (m[5] !== undefined) {                // ~~strike~~
      const s = document.createElement("del");
      for (const n of pathLinkNodes(m[5])) s.appendChild(n);
      nodes.push(s);
    } else if (m[6] !== undefined || m[7] !== undefined) { // *em* / _em_
      const em = document.createElement("em");
      for (const n of pathLinkNodes(m[6] !== undefined ? m[6] : m[7])) em.appendChild(n);
      nodes.push(em);
    } else if (m[8] !== undefined) {                // [text](href)
      const href = m[9];
      if (/^(https?:|mailto:)/i.test(href)) {
        const a = document.createElement("a");
        a.href = href;
        a.target = "_blank";
        a.rel = "noopener noreferrer";
        a.textContent = m[8];
        nodes.push(a);
      } else {
        nodes.push(document.createTextNode(m[0]));  // unsafe scheme: keep literal
      }
    }
    last = MD_INLINE_RE.lastIndex;
  }
  if (last < text.length) pushPathNodes(nodes, text.slice(last));
  return nodes;
}

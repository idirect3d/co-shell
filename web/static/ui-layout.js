// co-shell Web UI - layout container components (FEATURE-524).
//
// Registers "row" (children laid out side by side) and "col" (a vertical
// column, used inside a row to stack several components in one column).
// Together they express two-column layouts — e.g. a steps timeline on the
// left, an icon card above a form on the right — which the single-column
// components could not describe.
//
// SECURITY RULE (same as ui.js): never assign innerHTML, never build HTML by
// string concatenation. Every element is created with document.createElement,
// so a tree carrying markup renders as literal text and creates no DOM.
//
// Author: L.Shuang
// Created: 2026-09-15
// MIT License - Copyright (c) 2026 L.Shuang

(function (global) {
  "use strict";

  if (!global.UI || typeof global.UI.register !== "function") return;

  // el creates an element with an optional class.
  function el(tag, cls) {
    var node = document.createElement(tag);
    if (cls) node.className = cls;
    return node;
  }

  // num reads a numeric prop, falling back to 0 for anything unusable.
  function num(v) {
    var n = typeof v === "number" ? v : parseFloat(v);
    return isFinite(n) ? n : 0;
  }

  // row lays its children out horizontally. props.gap (px) is optional; the
  // CSS wraps children into a vertical stack when the column gets too narrow,
  // so a two-column layout degrades to one column on small screens instead of
  // overflowing.
  global.UI.register("row", {
    render: function (node) {
      var box = el("div", "ui-row");
      var gap = num(node.props.gap);
      if (gap > 0) box.style.gap = Math.min(gap, 48) + "px";
      return box;
    }
  });

  // col is one column of a row. props.flex (number) sets its growth weight, so
  // flex:2 renders twice as wide as flex:1; the CSS default gives every column
  // an equal share with a minimum basis.
  global.UI.register("col", {
    render: function (node) {
      var box = el("div", "ui-col");
      var flex = num(node.props.flex);
      if (flex > 0) box.style.flexGrow = String(Math.min(flex, 12));
      return box;
    }
  });
})(window);

// co-shell Web UI - SVG chart renderer for LLM component trees (FEATURE-524).
//
// Registers the "chart" component (kind = bar | line | pie) into the ui.js
// registry. Charts are drawn as hand-written SVG: no third-party library, no
// innerHTML, and every colour comes from a CSS class so the four themes stay
// consistent (an SVG attribute cannot use var(--x), a CSS class can).
//
// Element classes are part of the contract with the tests: exactly one
// <rect class="ui-chart-bar"> per bar, one <polyline class="ui-chart-line">
// plus one <circle class="ui-chart-point"> per data point, and one
// <path class="ui-chart-slice"> per pie slice; nothing else emits those
// elements inside a chart.
//
// Author: L.Shuang
// Created: 2026-09-15
// MIT License - Copyright (c) 2026 L.Shuang

(function (global) {
  "use strict";

  if (!global.UI || typeof global.UI.register !== "function") return;

  // Fixed drawing surface; the SVG scales to its container through CSS
  // (width:100%; height:auto) so the chart is responsive without JS resize
  // listeners and can never overflow the block horizontally.
  var W = 720;
  var H = 320;
  var PAD = { top: 14, right: 14, bottom: 38, left: 52 };
  var MAX_LABEL = 10; // characters kept from a category label before truncating
  var GRID_LINES = 4;

  var SVG_NS = "http://www.w3.org/2000/svg";

  // t is the ui.js localization helper (zh/en).
  function t(zh, en) {
    return typeof global.UI.t === "function" ? global.UI.t(zh, en) : zh;
  }

  // num coerces a prop value to a finite number (NaN-safe).
  function num(v) {
    var n = typeof v === "number" ? v : parseFloat(v);
    return isFinite(n) ? n : 0;
  }

  // fmtNum renders a number with thousands separators and at most 2 decimals.
  function fmtNum(v) {
    var n = num(v);
    var s = Math.abs(n) >= 1000 || Math.abs(n) % 1 !== 0 ? n.toLocaleString() : String(n);
    return s;
  }

  // truncLabel shortens a category label so a long name cannot stretch the
  // axis (UC-23) — the full label stays available in the tooltip/title.
  function truncLabel(label) {
    var s = label === undefined || label === null ? "" : String(label);
    return s.length > MAX_LABEL ? s.slice(0, MAX_LABEL - 1) + "…" : s;
  }

  // svgEl creates an SVG element with attributes and optional text content.
  function svgEl(tag, attrs, text) {
    var node = document.createElementNS(SVG_NS, tag);
    if (attrs) {
      for (var k in attrs) {
        if (Object.prototype.hasOwnProperty.call(attrs, k) && attrs[k] !== undefined && attrs[k] !== null) {
          node.setAttribute(k, String(attrs[k]));
        }
      }
    }
    if (text !== undefined && text !== null) node.textContent = String(text);
    return node;
  }

  // normalizeSeries reads props.series into [{name, data:[{label, value}]}],
  // dropping anything unusable so a malformed series degrades to an empty
  // chart instead of throwing.
  function normalizeSeries(props) {
    var raw = Array.isArray(props.series) ? props.series : [];
    var out = [];
    for (var i = 0; i < raw.length; i++) {
      var s = raw[i];
      if (!s || typeof s !== "object") continue;
      var data = [];
      var pts = Array.isArray(s.data) ? s.data : [];
      for (var j = 0; j < pts.length; j++) {
        var p = pts[j];
        if (!p || typeof p !== "object") continue;
        data.push({ label: p.label === undefined ? "" : String(p.label), value: num(p.value) });
      }
      out.push({ name: s.name === undefined ? "" : String(s.name), data: data });
    }
    return out;
  }

  // seriesClass maps a series index to its theme colour class.
  function seriesClass(i) {
    return "ui-chart-s" + (i % 5);
  }

  // emptyState is the shared "no data" placeholder.
  function emptyState() {
    var box = document.createElement("div");
    box.className = "ui-empty";
    box.textContent = t("暂无可绘制的数据", "No data to plot");
    return box;
  }

  // range computes the value range including the zero baseline, so negative
  // values and all-zero data both produce a valid axis (UC-23: no NaN, no
  // division by zero).
  function range(series) {
    var min = 0;
    var max = 0;
    var seen = false;
    for (var i = 0; i < series.length; i++) {
      for (var j = 0; j < series[i].data.length; j++) {
        var v = series[i].data[j].value;
        if (!seen) { min = v; max = v; seen = true; }
        if (v < min) min = v;
        if (v > max) max = v;
      }
    }
    if (!seen) { min = 0; max = 1; }
    if (min > 0) min = 0;
    if (max < 0) max = 0;
    if (max === min) max = min + 1; // all-zero (or single value): keep a 1-unit axis
    return { min: min, max: max };
  }

  global.UI.register("chart", {
    children: false,
    render: function (node) {
      var props = node.props;
      var kind = props.kind === "line" || props.kind === "pie" ? props.kind : "bar";
      var series = normalizeSeries(props);
      var unit = props.unit === undefined || props.unit === null ? "" : String(props.unit);
      var box = document.createElement("div");
      // NB: the modifier must not collide with the element classes below
      // (.ui-chart-bar / .ui-chart-slice / .ui-chart-line …); "ui-chart-bar"
      // as a container class made every bar query return the container too.
      box.className = "ui-chart ui-chart-kind-" + kind;
      box.setAttribute("data-chart-kind", kind);

      if (kind === "pie") global.UI.__renderPie(box, series, unit);
      else global.UI.__renderAxesChart(box, series, unit, kind, node);

      if (series.length > 1 || (series.length === 1 && series[0].name)) {
        global.UI.__renderLegend(box, series);
      }
      return box;
    }
  });

  // ---------------------------------------------------------------------------
  // Layout helpers
  // ---------------------------------------------------------------------------

  function plotArea() {
    return { x0: PAD.left, y0: PAD.top, w: W - PAD.left - PAD.right, h: H - PAD.top - PAD.bottom };
  }

  // yScale maps a value to a y coordinate inside the plot area.
  function yScale(r, v, area) {
    return area.y0 + area.h - ((num(v) - r.min) / (r.max - r.min)) * area.h;
  }

  // baseY is the y coordinate of the zero baseline (clamped into the range).
  function baseY(r, area) {
    return yScale(r, Math.max(r.min, Math.min(r.max, 0)), area);
  }

  // axisFrame draws the horizontal gridlines, the y tick labels and the x axis.
  function axisFrame(svg, r, area, unit) {
    for (var i = 0; i <= GRID_LINES; i++) {
      var val = r.min + ((r.max - r.min) * i) / GRID_LINES;
      var y = yScale(r, val, area);
      svg.appendChild(svgEl("line", { class: "ui-chart-grid", x1: area.x0, y1: y, x2: area.x0 + area.w, y2: y }));
      svg.appendChild(svgEl("text", { class: "ui-chart-label", x: area.x0 - 8, y: y + 4, "text-anchor": "end" }, fmtNum(val) + unit));
    }
    var zero = baseY(r, area);
    svg.appendChild(svgEl("line", { class: "ui-chart-axis", x1: area.x0, y1: zero, x2: area.x0 + area.w, y2: zero }));
  }

  // xLabels draws one category label per band centre; a label longer than
  // MAX_LABEL is truncated on screen and kept in full as a <title> tooltip.
  function xLabels(svg, labels, area) {
    var n = Math.max(labels.length, 1);
    var band = area.w / n;
    for (var i = 0; i < labels.length; i++) {
      var raw = labels[i] === undefined || labels[i] === null ? "" : String(labels[i]);
      var txt = svgEl("text", { class: "ui-chart-label", x: area.x0 + band * (i + 0.5), y: area.y0 + area.h + 20, "text-anchor": "middle" }, truncLabel(raw));
      if (raw.length > MAX_LABEL) txt.appendChild(svgEl("title", {}, raw));
      svg.appendChild(txt);
    }
  }

  // pointAttrs stamps the drill-down payload on a data-point element so the
  // interaction layer (Stage 3) can send label + value + index upstream.
  function pointAttrs(el, seriesIndex, pointIndex, point, unit) {
    el.setAttribute("data-series", String(seriesIndex));
    el.setAttribute("data-point-index", String(pointIndex));
    el.setAttribute("data-point-label", point.label);
    el.setAttribute("data-point-value", String(point.value));
    el.appendChild(svgEl("title", {}, (point.label ? point.label + ": " : "") + fmtNum(point.value) + unit));
  }

  // ---------------------------------------------------------------------------
  // Bar / line
  // ---------------------------------------------------------------------------

  function renderBars(svg, series, r, area, unit) {
    var n = Math.max(series[0].data.length, 1);
    var band = area.w / n;
    var groups = Math.max(series.length, 1);
    var groupW = band * 0.68;
    var barW = Math.max(groupW / groups - 2, 2);
    var zero = baseY(r, area);
    for (var s = 0; s < series.length; s++) {
      var data = series[s].data;
      for (var i = 0; i < data.length; i++) {
        var p = data[i];
        var y = yScale(r, p.value, area);
        var x = area.x0 + band * i + (band - groupW) / 2 + s * (groupW / groups) + 1;
        var rect = svgEl("rect", {
          class: "ui-chart-bar " + seriesClass(s),
          x: x, y: Math.min(y, zero), width: barW, height: Math.abs(zero - y), rx: 2
        });
        pointAttrs(rect, s, i, p, unit);
        svg.appendChild(rect);
      }
    }
  }

  function renderLines(svg, series, r, area, unit) {
    var n = Math.max(series[0].data.length, 1);
    var band = area.w / n;
    for (var s = 0; s < series.length; s++) {
      var data = series[s].data;
      var coords = [];
      for (var i = 0; i < data.length; i++) {
        coords.push((area.x0 + band * (i + 0.5)) + "," + yScale(r, data[i].value, area));
      }
      if (coords.length) {
        svg.appendChild(svgEl("polyline", {
          class: "ui-chart-line " + seriesClass(s),
          points: coords.join(" "), fill: "none"
        }));
      }
      for (var j = 0; j < data.length; j++) {
        var dot = svgEl("circle", {
          class: "ui-chart-point " + seriesClass(s),
          cx: area.x0 + band * (j + 0.5), cy: yScale(r, data[j].value, area), r: 3.5
        });
        pointAttrs(dot, s, j, data[j], unit);
        svg.appendChild(dot);
      }
    }
  }

  function renderAxesChart(box, series, unit, kind, node) {
    var hasData = false;
    for (var i = 0; i < series.length; i++) {
      if (series[i].data.length) { hasData = true; break; }
    }
    if (!hasData) { box.appendChild(emptyState()); return; }

    var r = range(series);
    var area = plotArea();
    var svg = svgEl("svg", {
      class: "ui-chart-svg", viewBox: "0 0 " + W + " " + H,
      preserveAspectRatio: "xMidYMid meet", role: "img"
    });
    if (node && node.id) svg.setAttribute("data-ui-chart-id", node.id);

    axisFrame(svg, r, area, unit);
    var labels = series[0].data.map(function (p) { return p.label; });
    xLabels(svg, labels, area);
    if (kind === "line") renderLines(svg, series, r, area, unit);
    else renderBars(svg, series, r, area, unit);
    box.appendChild(svg);
  }

  // ---------------------------------------------------------------------------
  // Pie
  // ---------------------------------------------------------------------------

  // renderSliceLegend lists the slices (used when the series itself has no
  // name, i.e. the common pie case).
  function renderSliceLegend(box, data, total) {
    var wrap = document.createElement("div");
    wrap.className = "ui-chart-legend";
    for (var i = 0; i < data.length; i++) {
      if (data[i].value <= 0) continue;
      var item = document.createElement("span");
      item.className = "ui-chart-legend-item";
      var sw = document.createElement("span");
      sw.className = "ui-chart-swatch " + seriesClass(i);
      item.appendChild(sw);
      item.appendChild(document.createTextNode(
        (data[i].label || "-") + " " + Math.round((data[i].value / total) * 100) + "%"
      ));
      wrap.appendChild(item);
    }
    if (wrap.children.length) box.appendChild(wrap);
  }

  // renderPie draws one <path class="ui-chart-slice"> per positive value,
  // starting at 12 o'clock and turning clockwise, so the sector angle is
  // exactly value/total * 360°.
  function renderPie(box, series, unit) {
    var data = series.length ? series[0].data : [];
    var total = 0;
    for (var i = 0; i < data.length; i++) {
      if (data[i].value > 0) total += data[i].value;
    }
    if (!data.length || total <= 0) { box.appendChild(emptyState()); return; }

    var R = Math.min(W, H) / 2 - 16;
    var cx = W / 2;
    var cy = H / 2;
    var svg = svgEl("svg", {
      class: "ui-chart-svg", viewBox: "0 0 " + W + " " + H,
      preserveAspectRatio: "xMidYMid meet", role: "img"
    });
    var angle = -Math.PI / 2;
    for (var j = 0; j < data.length; j++) {
      var v = data[j].value;
      if (!(v > 0)) continue; // a zero/negative slice has no drawable sector
      var sweep = (v / total) * Math.PI * 2;
      var x1 = cx + R * Math.cos(angle);
      var y1 = cy + R * Math.sin(angle);
      var x2 = cx + R * Math.cos(angle + sweep);
      var y2 = cy + R * Math.sin(angle + sweep);
      var largeArc = sweep > Math.PI ? 1 : 0;
      var d = "M" + cx + " " + cy + " L" + x1 + " " + y1 + " A" + R + " " + R + " 0 " + largeArc + " 1 " + x2 + " " + y2 + " Z";
      var slice = svgEl("path", { class: "ui-chart-slice " + seriesClass(j), d: d });
      pointAttrs(slice, 0, j, data[j], unit);
      svg.appendChild(slice);
      angle += sweep;
    }
    box.appendChild(svg);
    if (!series.length || !series[0].name) renderSliceLegend(box, data, total);
  }

  // ---------------------------------------------------------------------------
  // Legend (multi-series charts)
  // ---------------------------------------------------------------------------

  function renderLegend(box, series) {
    var wrap = document.createElement("div");
    wrap.className = "ui-chart-legend";
    for (var i = 0; i < series.length; i++) {
      if (!series[i].name) continue;
      var item = document.createElement("span");
      item.className = "ui-chart-legend-item";
      var sw = document.createElement("span");
      sw.className = "ui-chart-swatch " + seriesClass(i);
      item.appendChild(sw);
      item.appendChild(document.createTextNode(series[i].name));
      wrap.appendChild(item);
    }
    if (wrap.children.length) box.appendChild(wrap);
  }

  // Exported to the chart registration in the first half of this file.
  global.UI.__renderAxesChart = renderAxesChart;
  global.UI.__renderPie = renderPie;
  global.UI.__renderLegend = renderLegend;
})(window);

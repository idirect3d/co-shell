// co-shell Web UI - SVG "figure" renderer: a 2D/3D logical-coordinate canvas
// whose children are the primitives (FEATURE-570).
//
// The component is a canvas, not a chart: the LLM states a logical coordinate
// range and places primitives (point / segment / polygon / ellipse / axes /
// label / path / function) inside it. The renderer maps logical units to
// pixels with an equal aspect ratio and an upward y axis, expands the range
// when a primitive falls outside it (out of range never clips), and switches
// the whole figure to a fixed axonometric projection as soon as any primitive
// carries a z coordinate.
//
// Security: every element is built with createElementNS/setAttribute and every
// dynamic string goes through textContent — no markup string is ever injected,
// so "<img onerror=...>" in a label stays literal text. Function curves are
// sampled through UIExpr (ui-expr.js), which walks its own AST instead of
// executing the expression.
//
// Shapes are rendered by ui.js itself: figure.childHost() hands out the <g>
// layer and remembers the scene, so each child primitive keeps its own
// data-ui-id (and therefore stays individually addressable by ui_update).
//
// Author: L.Shuang
// Created: 2026-10-01
// MIT License - Copyright (c) 2026 L.Shuang

(function (global) {
  "use strict";

  if (!global.UI || typeof global.UI.register !== "function") return;

  var SVG_NS = "http://www.w3.org/2000/svg";
  var MAX_SHAPES = 200;
  var MAX_POINTS = 400;
  var MAX_SAMPLES = 400;
  var DEFAULT_W = 720;
  var DEFAULT_H = 420;
  var PAD_BASE = 18; // outer padding when the axes are hidden
  var PAD_AXES = { top: 16, right: 20, bottom: 36, left: 46 };

  // Semantic colour names only: they map onto CSS classes so the four themes
  // stay consistent (an SVG attribute cannot use var(--x), a class can).
  var COLORS = ["accent", "info", "warn", "success", "error", "dim", "none"];
  var DASH_PATTERN = { solid: "", dashed: "7 5", dotted: "1 3" };
  var VIEWS = { front: [0, 0], top: [0, 90], side: [90, 0], iso: [35, 25] };
  var ARROW_POS = { end: 1, start: 1, both: 2 };

  var uid = 0; // unique per-svg id prefix (markers must not collide)

  // activeScene is set by figure.childHost just before ui.js renders the
  // children, so each primitive renderer knows the coordinate mapping.
  var activeScene = null;

  // t is the ui.js localization helper (zh/en).
  function t(zh, en) {
    return typeof global.UI.t === "function" ? global.UI.t(zh, en) : zh;
  }

  // el builds an HTML element from text only (a markup string is never parsed).
  function el(tag, cls, text) {
    var node = document.createElement(tag);
    if (cls) node.className = cls;
    if (text !== undefined && text !== null) node.textContent = String(text);
    return node;
  }

  // sv builds an SVG element with attributes and optional text content.
  function sv(tag, attrs, text) {
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

  // numOr coerces a prop to a finite number, falling back when it is absent or
  // unusable (NaN, Infinity, "").
  function numOr(v, fallback) {
    if (v === undefined || v === null || v === "") return fallback;
    var n = typeof v === "number" ? v : parseFloat(v);
    return isFinite(n) ? n : fallback;
  }

  // zVal returns the z coordinate or NaN when it is absent — NaN is what marks
  // a primitive as 2D, so it must not degrade to 0.
  function zVal(v) {
    if (v === undefined || v === null || v === "") return NaN;
    var n = typeof v === "number" ? v : parseFloat(v);
    return isFinite(n) ? n : NaN;
  }

  function clamp(v, lo, hi) {
    return v < lo ? lo : v > hi ? hi : v;
  }

  function str(v) {
    return v === undefined || v === null ? "" : String(v);
  }

  // colorName normalizes a semantic colour prop; unknown values fall back so a
  // typo can never produce an unstyled (invisible) primitive.
  function colorName(v, fallback) {
    var s = str(v).trim();
    return COLORS.indexOf(s) >= 0 ? s : fallback;
  }

  // ---------------------------------------------------------------------------
  // Projection: 2D is the identity, 3D is p' = Rz(yaw) * Rx(pitch) * p.
  // ---------------------------------------------------------------------------

  // viewAngles resolves the (yaw, pitch) pair in degrees: explicit yaw/pitch
  // win, then the view preset, then the default.
  function viewAngles(props, is3D) {
    var preset = VIEWS[str(props.view)] || (is3D ? VIEWS.iso : VIEWS.front);
    var yaw = numOr(props.yaw, preset[0]);
    var pitch = numOr(props.pitch, preset[1]);
    return { yaw: yaw, pitch: pitch };
  }

  // makeProjection returns a function [x,y,z] -> [u,v]. Without a z coordinate
  // anywhere in the figure the coordinates pass through untouched, so a plain
  // 2D drawing is never skewed by a stray rotation.
  function makeProjection(is3D, angles) {
    if (!is3D) {
      return function (p) { return [p[0], p[1]]; };
    }
    var cy = Math.cos(angles.yaw * Math.PI / 180);
    var sy = Math.sin(angles.yaw * Math.PI / 180);
    var cp = Math.cos(angles.pitch * Math.PI / 180);
    var sp = Math.sin(angles.pitch * Math.PI / 180);
    return function (p) {
      var x = p[0];
      var y = isFinite(p[1]) ? p[1] : 0;
      var z = isFinite(p[2]) ? p[2] : 0;
      var y1 = y * cp - z * sp;
      var x2 = x * cy - y1 * sy;
      var y2 = x * sy + y1 * cy;
      return [x2, y2];
    };
  }

  // ---------------------------------------------------------------------------
  // Scene: primitives -> projected vertices -> bounds -> pixel mapping
  // ---------------------------------------------------------------------------

  // readPoints converts [[x,y],[x,y,z]...] into [x,y,z] triples, dropping
  // entries without a usable x/y and never exceeding MAX_POINTS.
  function readPoints(raw) {
    var out = [];
    if (!Array.isArray(raw)) return out;
    for (var i = 0; i < raw.length && out.length < MAX_POINTS; i++) {
      var p = raw[i];
      if (!Array.isArray(p) || p.length < 2) continue;
      var x = numOr(p[0], NaN);
      var y = numOr(p[1], NaN);
      if (!isFinite(x) || !isFinite(y)) continue;
      out.push([x, y, p.length > 2 ? zVal(p[2]) : NaN]);
    }
    return out;
  }

  // shapeVertices lists the logical vertices a primitive occupies; the scene
  // uses them both to detect 3D and to grow the visible range.
  function shapeVertices(node, curve) {
    var p = node.props || {};
    var out = [];
    var i;
    switch (node.type) {
      case "point":
      case "label":
        out.push([numOr(p.x, 0), numOr(p.y, 0), zVal(p.z)]);
        break;
      case "segment":
        out.push([numOr(p.x1, 0), numOr(p.y1, 0), zVal(p.z1)]);
        out.push([numOr(p.x2, 0), numOr(p.y2, 0), zVal(p.z2)]);
        break;
      case "ellipse":
        var cx = numOr(p.cx, 0);
        var cy = numOr(p.cy, 0);
        var rx = Math.abs(numOr(p.rx, 0));
        var ry = Math.abs(numOr(p.ry, 0));
        var z = zVal(p.z);
        out.push([cx - rx, cy, z]);
        out.push([cx + rx, cy, z]);
        out.push([cx, cy - ry, z]);
        out.push([cx, cy + ry, z]);
        break;
      case "polygon":
      case "path":
        out = out.concat(readPoints(p.points));
        break;
      case "function":
        if (curve && curve.length) {
          for (i = 0; i < curve.length; i++) out.push(curve[i]);
        } else {
          out.push([numOr(p.from, 0), 0, zVal(p.z)]);
          out.push([numOr(p.to, 1), 0, zVal(p.z)]);
        }
        break;
      case "axes":
        var o = originOf(p);
        out.push([o[0], o[1], o[2]]);
        if (p.x !== false) out.push([numOr(p.xend, o[0] + 1), o[1], o[2]]);
        if (p.y !== false) out.push([o[0], numOr(p.yend, o[1] + 1), o[2]]);
        break;
      default:
        break;
    }
    return out;
  }

  // originOf reads an axes origin ([x,y] or [x,y,z]); a missing value falls
  // back to 0 so an unspecified origin never hides the axes.
  function originOf(p) {
    var o = Array.isArray(p.origin) ? p.origin : [];
    return [numOr(o[0], 0), numOr(o[1], 0), o.length > 2 ? zVal(o[2]) : NaN];
  }

  // sampleCurve turns a function node into [x,y,z] vertices. A scalar expr is
  // y = f(x); an array expr is the parametric form [x(t), y(t)] or
  // [x(t), y(t), z(t)]. A parse failure is recorded and yields no vertices.
  function sampleCurve(node, sc) {
    var p = node.props || {};
    var out = [];
    var X = global.UIExpr;
    var count = clamp(Math.floor(numOr(p.samples, 100)), 2, MAX_SAMPLES);
    var from = numOr(p.from, 0);
    var to = numOr(p.to, 1);
    if (!X || typeof X.sample !== "function") return out;

    if (Array.isArray(p.expr)) {
      var parts = [];
      var i;
      for (i = 0; i < p.expr.length && i < 3; i++) {
        var one = X.compile(str(p.expr[i]), ["t"]);
        if (!one.ok) {
          sc.errors.push(X.errorText(one));
          return out;
        }
        parts.push(one);
      }
      if (parts.length < 2) return out;
      for (i = 0; i < count; i++) {
        var tv = from + (to - from) * (count === 1 ? 0 : i / (count - 1));
        var vars = { t: tv };
        var xs = X.evalNode(parts[0].node, vars, 0);
        var ys = X.evalNode(parts[1].node, vars, 0);
        var zs = parts.length > 2 ? X.evalNode(parts[2].node, vars, 0) : zVal(p.z);
        if (isFinite(xs) && isFinite(ys)) out.push([xs, ys, zs]);
      }
      return out;
    }

    var name = str(p.var) || "x";
    var res = X.sample(str(p.expr), name, from, to, count);
    if (!res.ok) {
      sc.errors.push(X.errorText(res));
      return out;
    }
    var zc = zVal(p.z);
    var zNode = null;
    if (str(p.exprZ)) {
      var zc2 = X.compile(str(p.exprZ), [name]);
      if (!zc2.ok) {
        sc.errors.push(X.errorText(zc2));
        return out;
      }
      zNode = zc2.node;
    }
    for (i = 0; i < res.points.length; i++) {
      var pt = res.points[i];
      var zv = zc;
      if (zNode) {
        var vz = {};
        vz[name] = pt[0];
        zv = X.evalNode(zNode, vz, 0);
      }
      out.push([pt[0], pt[1], zv]);
    }
    return out;
  }

  // buildScene prepares everything the primitive renderers need. opts.fitOnly
  // drops the declared range and fits the canvas to the data alone (used by the
  // standalone primitive fallback).
  function buildScene(props, children, opts) {
    var sc = {
      props: props,
      children: children,
      fitOnly: !!(opts && opts.fitOnly),
      is3D: false,
      curves: {},
      errors: [],
      svg: null,
      layer: null
    };
    var i;
    var list = Array.isArray(children) ? children.slice(0, MAX_SHAPES) : [];

    for (i = 0; i < list.length; i++) {
      var node = list[i];
      if (!node || typeof node !== "object") continue;
      if (node.type === "function") sc.curves[i] = sampleCurve(node, sc);
      var verts = shapeVertices(node, sc.curves[i]);
      for (var v = 0; v < verts.length; v++) {
        if (isFinite(verts[v][2])) { sc.is3D = true; break; }
      }
    }

    var angles = viewAngles(props, sc.is3D);
    sc.angles = angles;
    sc.project = makeProjection(sc.is3D, angles);
    sc.map = makeMapping(sceneBounds(sc, list), props, sc.is3D);
    return sc;
  }

  // sceneBounds unions the declared range with the projected data extent. A 3D
  // figure fits the projection instead: xmin/xmax describe the model space, not
  // the projected plane, so applying them to the screen would squash the view.
  function sceneBounds(sc, list) {
    var p = sc.props;
    var b = { umin: null, umax: null, vmin: null, vmax: null };
    if (!sc.is3D && !sc.fitOnly) {
      extend(b, numOr(p.xmin, 0), numOr(p.ymin, 0));
      extend(b, numOr(p.xmax, 10), numOr(p.ymax, 10));
    }
    for (var i = 0; i < list.length; i++) {
      var node = list[i];
      if (!node || typeof node !== "object") continue;
      var verts = shapeVertices(node, sc.curves[i]);
      for (var v = 0; v < verts.length; v++) {
        var uv = sc.project(verts[v]);
        extend(b, uv[0], uv[1]);
      }
    }
    if (b.umin === null) { b.umin = 0; b.umax = 10; b.vmin = 0; b.vmax = 10; }
    // A degenerate span (single point, or a perfectly flat figure) must not
    // divide by zero: give it a unit window.
    if (!(b.umax - b.umin > 0)) { b.umin -= 0.5; b.umax += 0.5; }
    if (!(b.vmax - b.vmin > 0)) { b.vmin -= 0.5; b.vmax += 0.5; }
    return b;
  }

  function extend(b, u, v) {
    if (!isFinite(u) || !isFinite(v)) return;
    if (b.umin === null || u < b.umin) b.umin = u;
    if (b.umax === null || u > b.umax) b.umax = u;
    if (b.vmin === null || v < b.vmin) b.vmin = v;
    if (b.vmax === null || v > b.vmax) b.vmax = v;
  }

  // makeMapping fits the logical window into the canvas with an equal aspect
  // ratio and an upward y axis, leaving room for the axes labels.
  function makeMapping(b, props, is3D) {
    var w = clamp(Math.round(numOr(props.width, DEFAULT_W)), 160, 2400);
    var h = clamp(Math.round(numOr(props.height, DEFAULT_H)), 120, 2000);
    var pad = props.axes === false
      ? { top: PAD_BASE, right: PAD_BASE, bottom: PAD_BASE, left: PAD_BASE }
      : { top: PAD_AXES.top, right: PAD_AXES.right, bottom: PAD_AXES.bottom, left: PAD_AXES.left };
    var iw = Math.max(10, w - pad.left - pad.right);
    var ih = Math.max(10, h - pad.top - pad.bottom);
    var spanU = b.umax - b.umin;
    var spanV = b.vmax - b.vmin;
    var scale = Math.min(iw / spanU, ih / spanV);
    var ox = pad.left + (iw - spanU * scale) / 2;
    var oy = pad.top + (ih - spanV * scale) / 2;
    return {
      w: w,
      h: h,
      pad: pad,
      scale: scale,
      umin: b.umin,
      umax: b.umax,
      vmin: b.vmin,
      vmax: b.vmax,
      is3D: !!is3D,
      toX: function (u) { return ox + (u - b.umin) * scale; },
      toY: function (v) { return oy + (b.vmax - v) * scale; }
    };
  }

  // niceStep picks a 1/2/5 x 10^n tick step so the grid stays readable instead
  // of drawing hundreds of lines on a wide range.
  function niceStep(range, target) {
    if (!(range > 0)) return 1;
    var raw = range / target;
    var mag = Math.pow(10, Math.floor(Math.log(raw) / Math.LN10));
    var norm = raw / mag;
    var mult = norm <= 1 ? 1 : norm <= 2 ? 2 : norm <= 5 ? 5 : 10;
    return mult * mag;
  }

  // fmtTick renders a tick value without floating point noise.
  function fmtTick(v, step) {
    var digits = step < 1 ? Math.min(6, Math.ceil(-Math.log(step) / Math.LN10)) : 0;
    var s = v.toFixed(digits);
    if (s.indexOf(".") >= 0) s = s.replace(/0+$/, "").replace(/\.$/, "");
    return s === "-0" ? "0" : s;
  }

  // ---------------------------------------------------------------------------
  // Styling, markers and interaction
  // ---------------------------------------------------------------------------

  // styleOf normalizes the shared style props of a primitive.
  function styleOf(node) {
    var p = node.props || {};
    var dash = str(p.dash);
    return {
      stroke: colorName(p.stroke, "accent"),
      fill: colorName(p.fill, "none"),
      width: clamp(numOr(p.strokeWidth, 1.6), 0.1, 6),
      dash: Object.prototype.hasOwnProperty.call(DASH_PATTERN, dash) ? dash : "solid",
      opacity: clamp(numOr(p.opacity, 1), 0, 1)
    };
  }

  // applyStyle writes the class + attribute pair every shape shares. Colours go
  // through classes (themes), geometry through attributes.
  function applyStyle(elm, st, noFill) {
    elm.setAttribute("class", "ui-fig-stroke-" + st.stroke + " ui-fig-fill-" + (noFill ? "none" : st.fill));
    elm.setAttribute("stroke-width", String(st.width));
    if (st.dash === "dashed" || st.dash === "dotted") {
      elm.setAttribute("stroke-dasharray", DASH_PATTERN[st.dash]);
    }
    if (st.opacity < 1) elm.setAttribute("opacity", String(st.opacity));
    return elm;
  }

  // markerFor lazily defines the arrow/dot marker of a colour inside the
  // figure's <defs> and returns its id.
  function markerFor(sc, color, kind) {
    var id = sc.idPrefix + "-" + kind + "-" + color;
    if (sc.markers[id]) return id;
    sc.markers[id] = true;
    var m = sv("marker", {
      id: id,
      viewBox: "0 0 10 10",
      refX: kind === "arrow" ? 9 : 5,
      refY: 5,
      markerWidth: kind === "arrow" ? 6 : 4,
      markerHeight: kind === "arrow" ? 6 : 4,
      orient: "auto-start-reverse",
      markerUnits: "strokeWidth"
    });
    if (kind === "arrow") {
      m.appendChild(sv("path", { d: "M0,0 L10,5 L0,10 z", class: "ui-fig-fill-" + color }));
    } else {
      m.appendChild(sv("circle", { cx: 5, cy: 5, r: 4, class: "ui-fig-fill-" + color }));
    }
    sc.defs.appendChild(m);
    return id;
  }

  // arrowMarkers resolves arrow start/end for a primitive, defaulting to none.
  function arrowMarkers(sc, node, st) {
    var p = node.props || {};
    var a = str(p.arrow);
    var res = { start: null, end: null };
    if (a === "end" || a === "both") res.end = "url(#" + markerFor(sc, st.stroke, "arrow") + ")";
    if (a === "start" || a === "both") res.start = "url(#" + markerFor(sc, st.stroke, "arrow") + ")";
    return res;
  }

  // bindShapeActions wires a primitive's declared click/select action to the
  // shared outbound path; the payload is the primitive's anchor point.
  function bindShapeActions(node, host, anchor) {
    var raw = node.raw && typeof node.raw === "object" ? node.raw : {};
    var list = Array.isArray(raw.actions) ? raw.actions : [];
    var action = null;
    for (var i = 0; i < list.length; i++) {
      var on = list[i] && list[i].on;
      if (on === "click" || on === "select") { action = list[i]; break; }
    }
    if (!action || !action.id) return host;
    host.setAttribute("class", (host.getAttribute("class") || "") + " ui-fig-clickable");
    host.addEventListener("click", function (ev) {
      ev.stopPropagation();
      var payload = { type: "point", x: anchor[0], y: anchor[1] };
      if (isFinite(anchor[2])) payload.z = anchor[2];
      var sent = typeof global.UI.sendUIAction === "function"
        ? global.UI.sendUIAction(action.id, payload, host, !!action.blocking)
        : false;
      if (sent) host.setAttribute("data-action-sent", "1");
    });
    return host;
  }

  // ---------------------------------------------------------------------------
  // Axes and grid
  // ---------------------------------------------------------------------------

  // ticksIn lists the grid values inside [lo,hi]; the count is capped so a huge
  // range can never flood the DOM.
  function ticksIn(lo, hi, step) {
    var out = [];
    if (!(step > 0) || !isFinite(lo) || !isFinite(hi)) return out;
    var start = Math.ceil(lo / step) * step;
    for (var v = start; v <= hi + step * 1e-6 && out.length < 80; v += step) {
      out.push(Math.abs(v) < step * 1e-6 ? 0 : v);
    }
    return out;
  }

  // drawAxes paints the grid, the axis lines and the tick labels. It is used
  // both for the figure-level default axes and for an explicit axes primitive.
  function drawAxes(sc, layer, opts) {
    var m = sc.map;
    var step = opts.step > 0 ? opts.step : niceStep(m.is3D ? m.vmax - m.vmin : m.umax - m.umin, opts.ticks || 7);
    var grid = opts.grid !== false;
    var showTicks = opts.ticks !== 0;

    if (sc.is3D) {
      drawAxes3D(sc, layer, opts, step);
      return;
    }

    var o = opts.origin || [];
    var ox = numOr(o[0], m.umin <= 0 && m.umax >= 0 ? 0 : m.umin);
    var oy = numOr(o[1], m.vmin <= 0 && m.vmax >= 0 ? 0 : m.vmin);
    var uTicks = ticksIn(m.umin, m.umax, step);
    var vTicks = ticksIn(m.vmin, m.vmax, step);
    var i;

    if (grid) {
      for (i = 0; i < uTicks.length; i++) {
        var gx = sc.map.toX(uTicks[i]);
        layer.appendChild(sv("line", {
          class: "ui-figure-grid", x1: gx, y1: sc.map.toY(m.vmin), x2: gx, y2: sc.map.toY(m.vmax)
        }));
      }
      for (i = 0; i < vTicks.length; i++) {
        var gy = sc.map.toY(vTicks[i]);
        layer.appendChild(sv("line", {
          class: "ui-figure-grid", x1: sc.map.toX(m.umin), y1: gy, x2: sc.map.toX(m.umax), y2: gy
        }));
      }
    }

    if (opts.axisX !== false) {
      var y0 = sc.map.toY(oy);
      layer.appendChild(sv("line", {
        class: "ui-figure-axis", x1: sc.map.toX(m.umin), y1: y0, x2: sc.map.toX(m.umax), y2: y0
      }));
      if (showTicks) {
        for (i = 0; i < uTicks.length; i++) {
          var tx = sc.map.toX(uTicks[i]);
          layer.appendChild(sv("line", { class: "ui-figure-axis", x1: tx, y1: y0, x2: tx, y2: y0 + 4 }));
          if (opts.units !== false) {
            layer.appendChild(sv("text", {
              class: "ui-fig-tick", x: tx, y: y0 + 14, "text-anchor": "middle"
            }, fmtTick(uTicks[i], step)));
          }
        }
      }
      if (str(opts.xlabel)) {
        layer.appendChild(sv("text", {
          class: "ui-fig-axis-label", x: sc.map.toX(m.umax), y: y0 - 6, "text-anchor": "end"
        }, str(opts.xlabel)));
      }
    }

    if (opts.axisY !== false) {
      var x0 = sc.map.toX(ox);
      layer.appendChild(sv("line", {
        class: "ui-figure-axis", x1: x0, y1: sc.map.toY(m.vmin), x2: x0, y2: sc.map.toY(m.vmax)
      }));
      if (showTicks) {
        for (i = 0; i < vTicks.length; i++) {
          var ty = sc.map.toY(vTicks[i]);
          if (vTicks[i] === 0 && oy === 0) continue; // the origin label would double up
          layer.appendChild(sv("line", { class: "ui-figure-axis", x1: x0 - 4, y1: ty, x2: x0, y2: ty }));
          if (opts.units !== false) {
            layer.appendChild(sv("text", {
              class: "ui-fig-tick", x: x0 - 7, y: ty + 4, "text-anchor": "end"
            }, fmtTick(vTicks[i], step)));
          }
        }
      }
      if (str(opts.ylabel)) {
        layer.appendChild(sv("text", {
          class: "ui-fig-axis-label", x: x0 + 6, y: sc.map.toY(m.vmax) + 2
        }, str(opts.ylabel)));
      }
    }
  }

  // drawAxes3D paints the three model axes plus the floor grid at z = zmin.
  function drawAxes3D(sc, layer, opts, step) {
    var p = sc.props;
    var o = opts.origin || [];
    var ox = numOr(o[0], 0);
    var oy = numOr(o[1], 0);
    var oz = numOr(o[2], 0);
    var xEnd = numOr(p.xmax, 10);
    var yEnd = numOr(p.ymax, 10);
    var zEnd = numOr(p.zmax, 10);
    var i;
    var g = [];

    if (opts.grid !== false) {
      var xTy = ticksIn(Math.min(ox, xEnd), Math.max(ox, xEnd), step);
      var yTs = ticksIn(Math.min(oy, yEnd), Math.max(oy, yEnd), step);
      for (i = 0; i < xTy.length; i++) {
        g.push([sc.project([xTy[i], oy, oz]), sc.project([xTy[i], yEnd, oz])]);
      }
      for (i = 0; i < yTs.length; i++) {
        g.push([sc.project([ox, yTs[i], oz]), sc.project([xEnd, yTs[i], oz])]);
      }
      for (i = 0; i < g.length; i++) {
        layer.appendChild(sv("line", {
          class: "ui-figure-grid", x1: sc.map.toX(g[i][0][0]), y1: sc.map.toY(g[i][0][1]),
          x2: sc.map.toX(g[i][1][0]), y2: sc.map.toY(g[i][1][1])
        }));
      }
    }

    var axes = [
      { on: opts.axisX !== false, to: [xEnd, oy, oz], label: str(opts.xlabel) || "x" },
      { on: opts.axisY !== false, to: [ox, yEnd, oz], label: str(opts.ylabel) || "y" },
      { on: opts.axisZ !== false && isFinite(zVal(p.zmax)), to: [ox, oy, zEnd], label: str(opts.zlabel) || "z" }
    ];
    for (i = 0; i < axes.length; i++) {
      if (!axes[i].on) continue;
      var a = sc.project([ox, oy, oz]);
      var b2 = sc.project(axes[i].to);
      layer.appendChild(sv("line", {
        class: "ui-figure-axis",
        x1: sc.map.toX(a[0]), y1: sc.map.toY(a[1]),
        x2: sc.map.toX(b2[0]), y2: sc.map.toY(b2[1]),
        "marker-end": "url(#" + markerFor(sc, "dim", "arrow") + ")"
      }));
      if (axes[i].label) {
        layer.appendChild(sv("text", {
          class: "ui-fig-axis-label", x: sc.map.toX(b2[0]) + 4, y: sc.map.toY(b2[1]) - 4
        }, axes[i].label));
      }
    }
  }

  // ---------------------------------------------------------------------------
  // Primitive renderers
  // ---------------------------------------------------------------------------

  // fx trims a screen coordinate to two decimals so the DOM (and the exported
  // SVG) stays small.
  function fx(v) {
    return Math.round(v * 100) / 100;
  }

  // projectAll maps logical vertices to [screenX, screenY] pairs.
  function projectAll(sc, verts) {
    var out = [];
    for (var i = 0; i < verts.length; i++) {
      var uv = sc.project(verts[i]);
      out.push([fx(sc.map.toX(uv[0])), fx(sc.map.toY(uv[1]))]);
    }
    return out;
  }

  // toD builds an SVG path data string from screen points.
  function toD(pts, closed) {
    if (!pts.length) return "";
    var d = "M" + pts[0][0] + "," + pts[0][1];
    for (var i = 1; i < pts.length; i++) d += " L" + pts[i][0] + "," + pts[i][1];
    if (closed) d += " Z";
    return d;
  }

  // smoothD rounds a polyline into a quadratic spline (path.smooth).
  function smoothD(pts) {
    if (pts.length < 3) return toD(pts, false);
    var d = "M" + pts[0][0] + "," + pts[0][1];
    for (var i = 1; i < pts.length - 1; i++) {
      var mx = fx((pts[i][0] + pts[i + 1][0]) / 2);
      var my = fx((pts[i][1] + pts[i + 1][1]) / 2);
      d += " Q" + pts[i][0] + "," + pts[i][1] + " " + mx + "," + my;
    }
    var last = pts[pts.length - 1];
    d += " L" + last[0] + "," + last[1];
    return d;
  }

  // labelNode appends a text label; the text goes through textContent, so
  // markup in a label is displayed literally and never becomes DOM.
  function labelNode(sc, text, sx, sy, anchors) {
    var attrs = { class: "ui-fig-label", x: fx(sx), y: fx(sy) };
    if (anchors && anchors.anchor) attrs["text-anchor"] = anchors.anchor;
    if (anchors && anchors.dx) attrs.dx = fx(anchors.dx);
    if (anchors && anchors.dy) attrs.dy = fx(anchors.dy);
    if (anchors && anchors.size) attrs["font-size"] = String(anchors.size);
    if (anchors && anchors.color) attrs.class = "ui-fig-label ui-fig-fill-" + anchors.color;
    return sv("text", attrs, text);
  }

  function drawPoint(sc, g, node) {
    var p = node.props || {};
    var st = styleOf(node);
    var verts = shapeVertices(node.raw, null);
    var uv = sc.project(verts[0]);
    var sx = sc.map.toX(uv[0]);
    var sy = sc.map.toY(uv[1]);
    var r = clamp(numOr(p.r, 4), 0.5, 100);
    var filled = p.filled !== false;
    var dot = sv("circle", {
      class: "ui-fig-stroke-" + st.stroke + " ui-fig-fill-" + (filled ? st.stroke : "none"),
      cx: fx(sx), cy: fx(sy), r: r, "stroke-width": String(st.width)
    });
    if (st.opacity < 1) dot.setAttribute("opacity", String(st.opacity));
    g.appendChild(dot);

    var text = str(p.label);
    if (text) {
      var pos = str(p.labelPos) || "right";
      var off = { right: [r + 6, 4, "start"], left: [-r - 6, 4, "end"], top: [0, -r - 6, "middle"], bottom: [0, r + 14, "middle"] }[pos]
        || [r + 6, 4, "start"];
      g.appendChild(labelNode(sc, text, sx + off[0], sy + off[1], { anchor: off[2], color: st.stroke }));
    }
    bindShapeActions(node, g, verts[0]);
    return g;
  }

  function drawSegment(sc, g, node) {
    var p = node.props || {};
    var st = styleOf(node);
    var pts = projectAll(sc, shapeVertices(node.raw, null));
    if (pts.length < 2) return g;
    var mk = arrowMarkers(sc, node, st);
    var line = sv("line", {
      class: "ui-fig-stroke-" + st.stroke + " ui-fig-fill-none",
      x1: pts[0][0], y1: pts[0][1], x2: pts[1][0], y2: pts[1][1],
      "stroke-width": String(st.width)
    });
    if (st.dash !== "solid") line.setAttribute("stroke-dasharray", DASH_PATTERN[st.dash]);
    if (st.opacity < 1) line.setAttribute("opacity", String(st.opacity));
    if (mk.end) line.setAttribute("marker-end", mk.end);
    if (mk.start) line.setAttribute("marker-start", mk.start);
    g.appendChild(line);
    if (str(p.label)) {
      g.appendChild(labelNode(sc, str(p.label), (pts[0][0] + pts[1][0]) / 2, (pts[0][1] + pts[1][1]) / 2 - 6,
        { anchor: "middle", color: st.stroke }));
    }
    bindShapeActions(node, g, shapeVertices(node.raw, null)[0]);
    return g;
  }

  function drawLabel(sc, g, node) {
    var p = node.props || {};
    var st = styleOf(node);
    var verts = shapeVertices(node.raw, null);
    var uv = sc.project(verts[0]);
    var anchor = ["start", "middle", "end"].indexOf(str(p.anchor)) >= 0 ? str(p.anchor) : "middle";
    g.appendChild(labelNode(sc, str(p.text), sc.map.toX(uv[0]), sc.map.toY(uv[1]), {
      anchor: anchor,
      dx: numOr(p.dx, 0),
      dy: numOr(p.dy, 0),
      size: numOr(p.size, 0),
      color: st.stroke
    }));
    bindShapeActions(node, g, verts[0]);
    return g;
  }

  function drawPolygon(sc, g, node) {
    var p = node.props || {};
    var st = styleOf(node);
    var verts = readPoints(p.points);
    var pts = projectAll(sc, verts);
    if (pts.length < 2) return g;
    var explicitFill = Object.prototype.hasOwnProperty.call(p, "fill") && str(p.fill) !== "";
    var el2 = sv("path", {
      class: "ui-fig-stroke-" + st.stroke + " ui-fig-fill-" + (explicitFill ? st.fill : st.stroke),
      d: toD(pts, p.closed !== false),
      "stroke-width": String(st.width)
    });
    if (st.dash !== "solid") el2.setAttribute("stroke-dasharray", DASH_PATTERN[st.dash]);
    el2.setAttribute("opacity", String(st.opacity));
    // A filled polygon defaults to a translucent wash so shapes drawn on top of
    // it stay readable; an explicit opacity prop takes over instead.
    if (explicitFill && !Object.prototype.hasOwnProperty.call(p, "opacity")) {
      el2.setAttribute("fill-opacity", "0.22");
    } else if (!explicitFill && !Object.prototype.hasOwnProperty.call(p, "opacity")) {
      el2.setAttribute("fill-opacity", "0.18");
    }
    g.appendChild(el2);
    bindShapeActions(node, g, verts[0]);
    return g;
  }

  function drawEllipse(sc, g, node) {
    var p = node.props || {};
    var st = styleOf(node);
    var verts = shapeVertices(node.raw, null);
    var uv = sc.project(verts[0]);
    var cx = sc.map.toX(uv[0]);
    var cy = sc.map.toY(uv[1]);
    var rx = Math.abs(numOr(p.rx, 0)) * sc.map.scale;
    var ry = Math.abs(numOr(p.ry, 0)) * sc.map.scale;
    if (!(rx > 0) && !(ry > 0)) return g;
    var explicitFill = Object.prototype.hasOwnProperty.call(p, "fill") && str(p.fill) !== "";
    var el2 = sv("ellipse", {
      class: "ui-fig-stroke-" + st.stroke + " ui-fig-fill-" + (explicitFill ? st.fill : "none"),
      cx: fx(cx), cy: fx(cy), rx: fx(Math.max(rx, 0.5)), ry: fx(Math.max(ry, 0.5)),
      "stroke-width": String(st.width)
    });
    if (st.dash !== "solid") el2.setAttribute("stroke-dasharray", DASH_PATTERN[st.dash]);
    el2.setAttribute("opacity", String(st.opacity));
    if (explicitFill && !Object.prototype.hasOwnProperty.call(p, "opacity")) {
      el2.setAttribute("fill-opacity", "0.22");
    }
    // Rotation is meaningless once the ellipse has been projected, so it is
    // only honoured for a plain 2D figure.
    var rot = numOr(p.rotation, 0);
    if (!sc.is3D && rot) el2.setAttribute("transform", "rotate(" + fx(rot) + " " + fx(cx) + " " + fx(cy) + ")");
    g.appendChild(el2);
    bindShapeActions(node, g, verts[0]);
    return g;
  }

  function drawPath(sc, g, node) {
    var p = node.props || {};
    var st = styleOf(node);
    var verts = readPoints(p.points);
    var pts = projectAll(sc, verts);
    if (pts.length < 2) return g;
    var d = p.smooth ? smoothD(pts) : toD(pts, false);
    var mk = arrowMarkers(sc, node, st);
    var el2 = sv("path", {
      class: "ui-fig-stroke-" + st.stroke + " ui-fig-fill-none",
      d: d, "stroke-width": String(st.width), "fill": "none"
    });
    if (st.dash !== "solid") el2.setAttribute("stroke-dasharray", DASH_PATTERN[st.dash]);
    if (st.opacity < 1) el2.setAttribute("opacity", String(st.opacity));
    if (mk.end) el2.setAttribute("marker-end", mk.end);
    if (mk.start) el2.setAttribute("marker-start", mk.start);
    g.appendChild(el2);
    bindShapeActions(node, g, verts[0]);
    return g;
  }

  function drawFunction(sc, g, node) {
    var st = styleOf(node);
    var idx = sc.children ? sc.children.indexOf(node.raw) : -1;
    var verts = idx >= 0 && sc.curves[idx] ? sc.curves[idx] : sampleCurve(node, sc);
    var pts = projectAll(sc, verts);
    if (pts.length < 2) return g;
    var mk = arrowMarkers(sc, node, st);
    var el2 = sv("path", {
      class: "ui-fig-stroke-" + st.stroke + " ui-fig-fill-none",
      d: toD(pts, false), "stroke-width": String(st.width), "fill": "none"
    });
    if (st.dash !== "solid") el2.setAttribute("stroke-dasharray", DASH_PATTERN[st.dash]);
    if (st.opacity < 1) el2.setAttribute("opacity", String(st.opacity));
    if (mk.end) el2.setAttribute("marker-end", mk.end);
    g.appendChild(el2);
    bindShapeActions(node, g, verts[0]);
    return g;
  }

  function drawAxesShape(sc, g, node) {
    var p = node.props || {};
    drawAxes(sc, g, {
      origin: originOf(p),
      axisX: p.x !== false,
      axisY: p.y !== false,
      axisZ: p.z !== false,
      grid: p.grid !== false,
      ticks: Math.floor(numOr(p.ticks, 7)),
      step: numOr(p.step, 0),
      xlabel: str(p.xlabel),
      ylabel: str(p.ylabel),
      zlabel: str(p.zlabel)
    });
    return g;
  }

  // ---------------------------------------------------------------------------
  // Figure container
  // ---------------------------------------------------------------------------

  // buildSvg assembles the canvas: background, defs (markers), the grid/axes
  // layer and the layer that hosts the primitives.
  function buildSvg(sc) {
    var m = sc.map;
    var svgEl = sv("svg", {
      class: "ui-figure-svg",
      viewBox: "0 0 " + m.w + " " + m.h,
      width: "100%",
      preserveAspectRatio: "xMidYMid meet",
      role: "img"
    });
    sc.markers = {};
    var defs = sv("defs");
    svgEl.appendChild(defs);
    sc.defs = defs;
    svgEl.appendChild(sv("rect", { class: "ui-figure-bg", x: 0, y: 0, width: m.w, height: m.h }));
    var gridLayer = sv("g", { class: "ui-figure-axes" });
    var shapeLayer = sv("g", { class: "ui-figure-shapes" });
    svgEl.appendChild(gridLayer);
    svgEl.appendChild(shapeLayer);
    sc.svg = svgEl;
    sc.axesLayer = gridLayer;
    sc.shapeLayer = shapeLayer;

    if (sc.props.axes !== false) {
      drawAxes(sc, gridLayer, {
        grid: sc.props.grid !== false,
        ticks: 7,
        step: 0,
        xlabel: str(sc.props.xlabel),
        ylabel: str(sc.props.ylabel),
        zlabel: str(sc.props.zlabel)
      });
    }
    return svgEl;
  }

  // inlineColors resolves every themed class into a literal attribute so the
  // exported file keeps its colours in a browser or a design tool. It runs on a
  // detached clone copy: the live figure must stay class-driven so a theme
  // switch keeps working.
  function inlineColors(root) {
    if (typeof global.getComputedStyle !== "function") return;
    // The root itself must be resolved too, otherwise the exported <svg> keeps
    // its styling class while its children have literal colours.
    var all = [root].concat(Array.prototype.slice.call(root.querySelectorAll("*")));
    for (var i = 0; i < all.length; i++) {
      var node = all[i];
      var cs = global.getComputedStyle(node);
      if (!cs) continue;
      if (cs.fill) node.setAttribute("fill", cs.fill);
      if (cs.stroke) node.setAttribute("stroke", cs.stroke);
      if (cs.strokeWidth && cs.strokeWidth !== "0px") {
        node.setAttribute("stroke-width", String(parseFloat(cs.strokeWidth)));
      }
      if (cs.strokeDasharray && cs.strokeDasharray !== "none") {
        node.setAttribute("stroke-dasharray", cs.strokeDasharray);
      }
      if (cs.opacity && cs.opacity !== "1") node.setAttribute("opacity", cs.opacity);
      if (cs.fillOpacity && cs.fillOpacity !== "1") node.setAttribute("fill-opacity", cs.fillOpacity);
      if (cs.fontFamily) node.style.fontFamily = cs.fontFamily;
      if (cs.fontSize) node.style.fontSize = cs.fontSize;
      node.removeAttribute("class");
    }
  }

  // exportSvgText serializes the canvas with literal colours.
  function exportSvgText(svgEl) {
    var holder = el("div", "ui-figure-export");
    holder.setAttribute("aria-hidden", "true");
    holder.style.position = "fixed";
    holder.style.left = "-10000px";
    holder.style.top = "0";
    var clone = svgEl.cloneNode(true);
    holder.appendChild(clone);
    document.body.appendChild(holder);
    inlineColors(clone);
    var xml = new XMLSerializer().serializeToString(clone);
    if (holder.parentNode) holder.parentNode.removeChild(holder);
    return '<?xml version="1.0" encoding="UTF-8"?>\n' + xml;
  }

  // saveText downloads a Blob through a temporary anchor; no backend involved.
  function saveText(text, filename) {
    var blob = new Blob([text], { type: "image/svg+xml;charset=utf-8" });
    var url = URL.createObjectURL(blob);
    var a = el("a", "");
    a.href = url;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    if (a.parentNode) a.parentNode.removeChild(a);
    setTimeout(function () { URL.revokeObjectURL(url); }, 10000);
  }

  function safeFileName(name) {
    var s = str(name).replace(/[^A-Za-z0-9_.-]+/g, "-").replace(/^-+|-+$/g, "");
    return s || "figure";
  }

  // ---------------------------------------------------------------------------
  // Registration
  // ---------------------------------------------------------------------------

  global.UI.register("figure", {
    render: function (node) {
      var props = node.props || {};
      var raw = node.raw && typeof node.raw === "object" ? node.raw : {};
      var children = Array.isArray(raw.children) ? raw.children : [];
      var sc = buildScene(props, children);
      sc.idPrefix = "co-fig-" + (++uid) + "-" + (node.id ? safeFileName(node.id) : "x");

      var wrap = el("div", "ui-figure");
      var head = el("div", "ui-figure-head");
      var title = str(props.title);
      if (title) head.appendChild(el("span", "ui-figure-title", title));
      var flags = [];
      if (sc.is3D) flags.push(t("等轴测", "axonometric"));
      if (str(props.unit)) flags.push(str(props.unit));
      if (flags.length) head.appendChild(el("span", "ui-figure-flag", flags.join(" · ")));

      if (props.download !== false) {
        var btn = el("button", "btn ghost ui-figure-download", t("下载 SVG", "Download SVG"));
        btn.type = "button";
        btn.addEventListener("click", function (ev) {
          ev.stopPropagation();
          try {
            saveText(exportSvgText(wrap._uiFigureSvg), safeFileName(node.id ? node.id : title || "figure") + ".svg");
          } catch (err) {
            btn.disabled = true;
          }
        });
        head.appendChild(btn);
      }
      wrap.appendChild(head);

      var svgEl = buildSvg(sc);
      wrap.appendChild(svgEl);
      wrap._uiFigureSvg = svgEl;
      wrap._uiFigureScene = sc;

      if (sc.errors.length) {
        var note = el("div", "ui-fig-degrade", sc.errors[0]);
        note.title = sc.errors.join("\n");
        wrap.appendChild(note);
      }
      return wrap;
    },
    childHost: function (host) {
      var sc = host && host._uiFigureScene;
      if (!sc) return null;
      activeScene = sc;
      return sc.shapeLayer;
    }
  });

  // sceneFor returns the figure scene a primitive belongs to, or null when the
  // primitive is rendered on its own (an individual ui_update of a shape, or a
  // node that ended up outside a figure).
  function sceneFor(rawNode) {
    if (!activeScene || !activeScene.children) return null;
    return activeScene.children.indexOf(rawNode) >= 0 ? activeScene : null;
  }

  // standaloneShape degrades a primitive without a figure to a self-fitted mini
  // canvas, so an isolated primitive still shows something coherent.
  function standaloneShape(ctx) {
    var rawNode = ctx.raw && typeof ctx.raw === "object" ? ctx.raw : { props: ctx.props };
    var sc = buildScene({ axes: false, width: 240, height: 160 }, [rawNode], { fitOnly: true });
    sc.idPrefix = "co-fig-solo-" + (++uid);
    var svgEl = buildSvg(sc);
    sc.shapeLayer.appendChild(SHAPE_DRAW[rawNode.type](sc, sv("g", { class: "ui-fig-layer" }), ctx));
    var wrap = el("div", "ui-figure ui-figure-solo");
    wrap.appendChild(svgEl);
    wrap._uiFigureSvg = svgEl;
    wrap._uiFigureScene = sc;
    return wrap;
  }

  var SHAPE_DRAW = {
    point: drawPoint,
    segment: drawSegment,
    polygon: drawPolygon,
    ellipse: drawEllipse,
    axes: drawAxesShape,
    label: drawLabel,
    path: drawPath,
    function: drawFunction
  };

  var SHAPE_TYPES = ["point", "segment", "polygon", "ellipse", "axes", "label", "path", "function"];

  // Every primitive is registered so it can also be addressed on its own: an
  // isolated primitive degrades to a mini canvas, and a ui_update of a single
  // primitive keeps working instead of falling into "unknown component".
  SHAPE_TYPES.forEach(function (type) {
    global.UI.register(type, {
      children: false,
      render: function (node) {
        var sc = sceneFor(node.raw);
        var g = sv("g", { class: "ui-fig-shape ui-fig-shape-" + type });
        try {
          if (sc) return SHAPE_DRAW[type](sc, g, node);
          return standaloneShape(node);
        } catch (err) {
          // A broken primitive must never take the whole figure down.
          var fallback = sv("text", { class: "ui-fig-tick", x: 6, y: 14 }, t("图元渲染失败", "primitive failed to render"));
          void err;
          g.appendChild(fallback);
          return g;
        }
      }
    });
  });
})(typeof window !== "undefined" ? window : this);

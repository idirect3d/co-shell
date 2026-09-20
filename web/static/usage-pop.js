// FEATURE-546: the two status-bar usage popovers.
//
//   UsagePop.renderTotal(pop, data, T)  — lifetime totals + trend charts,
//       fed by GET /api/usage.
//   UsagePop.renderLast(pop, stats, mtp, T) — the current turn's metrics plus
//       the KV cache and MTP hit rates.
//
// The module is deliberately free of app.js internals: every label arrives in
// the `T` dictionary the caller passes in, so the localized strings stay in the
// app's i18n table and this file has no state of its own. Charts reuse the
// hand-rolled SVG renderer in ui-chart.js (no third-party library), which is
// theme-aware through CSS classes and draws its own empty state.
//
// Author: L.Shuang
// Created: 2026-09-20
// MIT License - Copyright (c) 2026 L.Shuang

(function (global) {
  "use strict";

  // fmt renders an integer with thousands separators.
  function fmt(n) {
    return (Number(n) || 0).toLocaleString();
  }

  // pct renders a [0,1] ratio as a one-decimal percentage.
  function pct(v) {
    return ((Number(v) || 0) * 100).toFixed(1) + "%";
  }

  // el builds a simple element with optional class and text.
  function el(tag, cls, text) {
    const node = document.createElement(tag);
    if (cls) node.className = cls;
    if (text !== undefined && text !== null) node.textContent = text;
    return node;
  }

  // row builds one "label: value" line of a metrics block.
  function row(k, v) {
    const r = el("div", "usage-row");
    r.appendChild(el("span", "k", k));
    r.appendChild(el("span", "v", v));
    return r;
  }

  // chartSection builds a titled line chart from a bucket array. Buckets carry
  // {label, prompt, completion} or {label, in_tps, out_tps}.
  function chartSection(title, buckets, keys, names, unit, T) {
    const sec = el("div", "usage-sec");
    sec.appendChild(el("div", "usage-sec-title", title));
    const series = keys.map(function (key, i) {
      return {
        name: names[i],
        data: (buckets || []).map(function (b) {
          return { label: b.label, value: Number(b[key]) || 0 };
        }),
      };
    });
    const box = el("div", "usage-chart");
    if (global.UI && global.UI.__renderAxesChart) {
      global.UI.__renderAxesChart(box, series, unit, "line", null);
      if (global.UI.__renderLegend) global.UI.__renderLegend(box, series);
    }
    sec.appendChild(box);
    return sec;
  }

  // renderTotal fills the lifetime-usage popover with the /api/usage payload.
  function renderTotal(pop, data, T) {
    pop.textContent = "";
    pop.appendChild(el("div", "usage-title", T.usageTotalTitle));

    const s = (data && data.summary) || {};
    const grid = el("div", "usage-grid");
    grid.appendChild(row(T.usageRequests, fmt(s.requests)));
    grid.appendChild(row(T.usageTotal, fmt(s.total_tokens)));
    grid.appendChild(row(T.usagePrompt, fmt(s.prompt_tokens)));
    grid.appendChild(row(T.usageCompletion, fmt(s.completion_tokens)));
    if (s.first_seen && s.last_seen) {
      grid.appendChild(row(T.usageRange, shortDate(s.first_seen) + " ~ " + shortDate(s.last_seen)));
    }
    pop.appendChild(grid);

    // A missing hit rate is reported as a reason, never as 0%.
    const cacheRow = el("div", "usage-sec");
    if (s.cache_available) {
      cacheRow.appendChild(row(T.usageCacheRate, pct(s.cache_rate) + "（" + fmt(s.cached_tokens) + "）"));
    } else {
      cacheRow.appendChild(el("div", "usage-note usage-warn", T.usageCacheRate + "：" + T.usageCacheUnavailable));
    }
    pop.appendChild(cacheRow);

    pop.appendChild(chartSection(T.usageTrendDay, data && data.by_day,
      ["prompt", "completion"], [T.usageSeriesPrompt, T.usageSeriesCompletion], "", T));
    pop.appendChild(chartSection(T.usageTrendHour, data && data.by_hour,
      ["prompt", "completion"], [T.usageSeriesPrompt, T.usageSeriesCompletion], "", T));
    pop.appendChild(chartSection(T.usageRateTrend, data && data.rate_by_day,
      ["in_tps", "out_tps"], [T.usageSeriesInTPS, T.usageSeriesOutTPS], "", T));
    pop.appendChild(el("div", "usage-note", T.usageApproxNote));
  }

  // shortDate trims an RFC3339 timestamp down to MM-DD for the range line.
  function shortDate(iso) {
    const m = /^\d{4}-(\d{2}-\d{2})/.exec(String(iso || ""));
    return m ? m[1] : String(iso || "");
  }

  // mtpText resolves the MTP line: a percentage when available, otherwise the
  // localized reason plus the suggestion it already carries.
  function mtpText(mtp, T) {
    if (!mtp) return T.usageMTPNotConfigured;
    if (mtp.available) return pct(mtp.rate);
    switch (mtp.reason) {
      case "fetch_failed": return T.usageMTPFetchFailed;
      case "no_spec_metrics": return T.usageMTPNoSpec;
      case "no_draft_tokens": return T.usageMTPNoDraft;
      default: return T.usageMTPNotConfigured;
    }
  }

  // renderLast fills the per-turn popover. stats is the caller's tokenStats
  // object; mtp is the active model's MTP status (may be null).
  function renderLast(pop, stats, mtp, T) {
    pop.textContent = "";
    pop.appendChild(el("div", "usage-title", T.usageLastTitle));

    const grid = el("div", "usage-grid");
    grid.appendChild(row(T.usagePrompt, fmt(stats.lastIn)));
    grid.appendChild(row(T.usageCompletion, fmt(stats.lastOut)));
    grid.appendChild(row(T.usageInTPS, fmt(stats.lastInTPS) + " tokens/s"));
    grid.appendChild(row(T.usageOutTPS, fmt(stats.lastOutTPS) + " tokens/s"));
    grid.appendChild(row(T.usageFirstToken, stats.lastInDur || "-"));
    grid.appendChild(row(T.usageOutDur, stats.lastOutDur || "-"));
    pop.appendChild(grid);

    const cache = el("div", "usage-sec");
    if (stats.lastCacheRate) {
      cache.appendChild(row(T.usageCacheRate, stats.lastCacheRate));
    } else {
      cache.appendChild(el("div", "usage-note usage-warn", T.usageCacheRate + "：" + T.usageCacheUnavailable));
    }
    pop.appendChild(cache);

    const spec = el("div", "usage-sec");
    const rate = mtp && mtp.available ? pct(mtp.rate) : null;
    if (rate) {
      spec.appendChild(row(T.usageMTP, rate));
    } else {
      spec.appendChild(el("div", "usage-note usage-warn", T.usageMTP + "：" + mtpText(mtp, T)));
    }
    pop.appendChild(spec);
  }

  global.UsagePop = { renderTotal: renderTotal, renderLast: renderLast };
})(typeof window !== "undefined" ? window : this);

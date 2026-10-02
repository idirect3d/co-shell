# FEATURE-580 UC-0008：PDF 打印导出（打印视图 / 打印样式 / 清理 / 窗口取树）

> 分支：**FEATURE-580**　版本：**v0.89.0**　对应方案：**浏览器打印导出（`UIExport.printTree`）**
> 依据：ROADMAP v0.89.0 验收标准 ③（打印视图仅输出导出内容、含页眉、图表/figure 保真、可另存为 PDF 与直接打印）与需求 ④（页眉 = 标题 + 导出时间 + co-shell 标识）
> 判定口径：`UIExport.printTree(tree, {title, time})` 先构建 `.ui-print-root` 再调 `window.print()`；打印媒体下页面上除 `.ui-print-root` 外全部隐藏；`afterprint` 后强制清理、DOM 无残留。

## 目的

以确定性 DOM 断言替代人工目视，核验「打印导出」的构建、隔离、清理三件事，确保用户拿到的是干净、完整、可打印的一份内容。

## 前置条件

- 同 UC-0001（分支 `FEATURE-580` + `--serve --dev` + 调试浏览器）。
- 打印能力用一个 stub 拦截，避免真的弹出系统打印框：

```js
// browser_evaluate：安装 stub
window.__printCalls = 0;
window.print = function () { window.__printCalls++; };
```

## 用例 UC-0008：打印视图内容齐备（含页眉与图表/figure 保真）

```js
// browser_evaluate
const tree = {type:"card", props:{title:"季度报告"}, children:[
  {type:"chart", props:{kind:"bar", series:[{name:"销量", data:[{label:"A", value:1}]}]}},
  {type:"figure", props:{title:"受力图", width:360, height:240}, children:[
    {type:"segment", props:{x1:0,y1:0,x2:5,y2:5}},
    {type:"point", props:{x:2,y:3,label:"P"}}
  ]},
  {type:"table", props:{columns:[{key:"a"}], rows:[{a:1}]}}
]};
UIExport.printTree(tree, {title:"季度报告", time:"2026-10-02 14:30"});
const root = document.querySelector(".ui-print-root");
return {
  exists: !!root,
  header: root?.querySelector(".ui-print-head")?.innerText || "",
  brand: /co-shell/.test(root?.textContent || ""),
  time: /2026-10-02 14:30/.test(root?.textContent || ""),
  chartSvg: root.querySelectorAll("svg.ui-chart").length,
  figureSvg: root.querySelectorAll("svg.ui-figure-svg").length,
  table: root.querySelectorAll(".ui-table").length,
  hostIsLastChild: document.body.lastElementChild === root
};
```

**期望**：`exists === true`；`header` 含标题「季度报告」；`brand === true`；`time === true`；`chartSvg === 1`；`figureSvg === 1`；`table === 1`（组件**真实渲染**，不是文本降级）；`hostIsLastChild === true`。

## 用例 UC-0009：打印媒体下仅保留导出内容

```js
// browser_evaluate：从 CSSOM 找到 @media print 规则并检查其内容
let printRules = [];
for (const sheet of document.styleSheets) {
  let rules; try { rules = sheet.cssRules; } catch (e) { continue; }
  for (const r of rules) {
    if (r.type === CSSRule.MEDIA_RULE && /print/.test(r.media.mediaText)) {
      for (const inner of r.cssRules) printRules.push(inner.cssText || "");
    }
  }
}
return {
  hasHideOthers: printRules.some(s => /\.ui-print-root/.test(s) && /display:\s*none/.test(s)),
  hasPageMargin: printRules.some(s => /@page/.test(s)),
  screenHidden: getComputedStyle(document.querySelector(".ui-print-root")).display
};
```

**期望**：`hasHideOthers === true`（存在形如 `body > *:not(.ui-print-root){display:none!important}` 的打印规则）；`hasPageMargin === true`（有 `@page` 页边距设置）；`screenHidden === "none"`（屏幕上不显示打印容器）。

## 用例 UC-0010：`window.print` 只调一次，`afterprint` 后无残留

```js
// browser_evaluate（接 UC-0008 之后）
const before = document.querySelectorAll(".ui-print-root").length;
const calls = window.__printCalls;
window.dispatchEvent(new Event("afterprint"));
return {
  calls,
  before,
  after: document.querySelectorAll(".ui-print-root").length,
  bodyRestored: document.body.lastElementChild !== document.querySelector(".ui-print-root")
};
```

**期望**：`calls === 1`；`before === 1`；`after === 0`（清理无残留）；`bodyRestored === true`。

## 用例 UC-0011：窗口导出取的是被点击窗口的树

```js
// browser_evaluate：两个窗口各自不同内容，导出非活动窗口
UIWin.render({ui_id:"ui-wA", ui_window_action:"open", ui_window_name:"窗口A", ui_window_surface:"float", ui_window_title:"窗口A", ui_tree: JSON.stringify({type:"callout", props:{variant:"info", text:"AAA"}})});
UIWin.render({ui_id:"ui-wB", ui_window_action:"open", ui_window_name:"窗口B", ui_window_surface:"float", ui_window_title:"窗口B", ui_tree: JSON.stringify({type:"callout", props:{variant:"info", text:"BBB"}})});
const btnB = document.querySelector('[data-window-name="窗口B"] [data-ui-export]');
btnB.click();
document.querySelector('.ui-export-menu [data-ui-export-item="md"]').click();
const mdOfB = window.__lastExportText || "";
UIWin.activate("窗口A");
const btnA = document.querySelector('[data-window-name="窗口A"] [data-ui-export]');
btnA.click();
document.querySelector('.ui-export-menu [data-ui-export-item="md"]').click();
return { b: mdOfB.includes("BBB") && !mdOfB.includes("AAA"), a: (window.__lastExportText || "").includes("AAA") };
```

**期望**：`b === true`；`a === true` —— 导出内容始终等于**被点击窗口**的 `rec.tree`，与「当前活动窗口」无关。

> 说明：用例通过 `window.__lastExportText` 观测最近一次导出文本；实现需在下载/打印前把最终文本挂到该调试字段（仅当 `window.__lastExportText` 已被测试预先定义时写入，避免污染生产行为），若无此钩子则改为断言 `UIExport.lastExport()` 返回值。

## 验收记录

| 用例 | 结果 | 证据 |
|---|---|---|
| UC-0008 | ✅ 通过 | `exists=true`；`header="季度报告co-shell · 2026-10-02 14:30"`；`brand/time=true`；`chartSvg=1`、`figureSvg=1`、`table=1`（真实渲染，非文本降级）；`themeAttr="light"`；容器为 `body` 最后一个子节点 |
| UC-0009 | ✅ 通过 | `@media print` 内共 9 条规则（见下方修正说明）；`hasHideOthers=true`（`body > :not(.ui-print-root) { display: none !important; }`）；`hasPageMargin=true`（`@page { margin: 14mm; }`）；屏幕下 `.ui-print-root` 计算 `display:none` |
| UC-0010 | ✅ 通过 | `window.print` 调用次数 `calls=1`；调用前 `.ui-print-root` 数量 1、`afterprint` 后为 0（无残留） |
| UC-0011 | ✅ 通过 | 导出内容始终等于被点击窗口的树：窗口B→`bbb=true`（含 BBB 不含 AAA）、窗口A（当前活动）→`a=true`；页眉分别为 `# 窗口B` / `# 窗口A` |

### 用例修正说明（UC-0009）

原探针写为 `printRules.push(inner.selectorText || inner.cssText)`，而样式规则的 `selectorText` 天生非空，导致永远取不到声明文本，`hasHideOthers` 恒为 false（误报）。已改为 `inner.cssText || ""`（`cssText` 同时包含选择器与声明），修正后 `hasHideOthers=true`。

> 验证环境：同 UC-0001；打印调用以 `window.print` stub 拦截（`window.__printCalls`），避免弹出系统打印框；`afterprint` 手动派发以触发清理。

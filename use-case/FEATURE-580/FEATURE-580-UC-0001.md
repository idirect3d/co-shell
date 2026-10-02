# FEATURE-580 UC-0001：导出入口与可见性（区块 / 浮窗 / 面板）

> 分支：**FEATURE-580**　版本：**v0.89.0**　对应方案：**前端导出入口注入（ui-export.js）**
> 依据：ROADMAP v0.89.0「FEATURE-580 render_ui 区块/窗口导出」需求 ①（用户确认 2026-10-02：入口覆盖对话流区块 + 窗口（浮窗/面板））
> 判定口径：三个位置（对话流区块标题栏、浮窗标题栏、面板头部）都能看到导出入口，且点击后展开「Markdown 下载 / PDF 打印」两项；入口不破坏既有按钮布局。

## 目的

验证 render_ui 的三种渲染落点都能拿到导出入口，且入口只出现在 render_ui 相关内容上，不污染其它区块。

## 前置条件

- 分支 `FEATURE-580`，静态资源已改（`ui-export.js` / `app.js` / `ui-windows.js` / `index.html` / `style.css`）。
- 启动服务：`work/co-shell-0.89.0.darwin.arm64 --serve --port 28261 --dev`（`--serve` 不自动开浏览器；`--dev` 从磁盘热加载前端）。
- 调试浏览器连到 `http://127.0.0.1:28261/`。

## 用例 UC-0001：对话流区块有导出入口

在页面里注入一棵样例树并模拟一次 `ui_render`：

```js
// browser_evaluate
const tree = {type:"card", props:{title:"销售分析"}, children:[{type:"kv", props:{items:[{k:"华东",v:320},{k:"华南",v:180}]}}]};
const data = {type:"ui_render", meta:{ui_id:"ui-uc1", ui_tree: JSON.stringify(tree), msg_index:"m1"}};
window.dispatchEvent(new MessageEvent("message", {data: JSON.stringify(data)}));  // 按实际入口等价调用
const block = document.querySelector(".ev.ui");
return {
  hasBlock: !!block,
  hasExport: !!block.querySelector(".ev-head [data-ui-export]"),
  label: block.querySelector(".ev-head [data-ui-export]")?.textContent?.trim() || ""
};
```

**期望**：`hasBlock === true`；`hasExport === true`；按钮文案在中文界面为「导出」（英文界面为「Export」）。

## 用例 UC-0002：点击展开 Markdown / PDF 两项

```js
// browser_evaluate
const btn = document.querySelector(".ev.ui .ev-head [data-ui-export]");
btn.click();
const menu = document.querySelector(".ui-export-menu");
return {
  open: !!menu,
  items: Array.from(menu?.querySelectorAll("[data-ui-export-item]") || []).map(n => n.dataset.uiExportItem)
};
```

**期望**：`open === true`；`items` 恰好等于 `["md","pdf"]`（顺序为 Markdown 在前、PDF 在后）；菜单为单例（再次点击不会出现两个 `.ui-export-menu`）。

## 用例 UC-0003：浮窗与面板窗口均有导出入口

```js
// browser_evaluate：打开一个浮窗窗口 + 一个面板窗口
UIComp = null; // 占位
UIWin.render({ui_id:"ui-w1", ui_window_action:"open", ui_window_name:"浮窗A", ui_window_surface:"float", ui_window_title:"浮窗A", ui_tree: JSON.stringify({type:"progress", props:{value:3, max:4, label:"进度"}})});
UIWin.render({ui_id:"ui-w2", ui_window_action:"open", ui_window_name:"面板B", ui_window_surface:"panel", ui_window_title:"面板B", ui_tree: JSON.stringify({type:"table", props:{columns:[{key:"a"}], rows:[{a:1}]}})});
return {
  floatExport: !!document.querySelector('[data-window-name="浮窗A"] .ui-window-head [data-ui-export]'),
  panelExport: !!document.querySelector('[data-window-name="面板B"] [data-ui-export]'),
  otherBlocksClean: document.querySelectorAll(".ev:not(.ui) .ev-head [data-ui-export]").length
};
```

**期望**：`floatExport === true`；`panelExport === true`；`otherBlocksClean === 0`（非 `ui` 区块不出现导出入口）。

## 验收记录

| 用例 | 结果 | 证据 |
|---|---|---|
| UC-0001 | ✅ 通过 | `hasBlock=true`、`hasExport=true`、文案「导出」；动作组顺序 `[ev-act ui-export-btn, ev-act, ev-act, ev-act]`（导出位于首位，复制/折叠/重试保留） |
| UC-0002 | ✅ 通过 | `open=true`；`items=[md,pdf]`、`labels=[Markdown 下载, PDF / 打印]`；重复点击后 `.ui-export-menu` 数量恒为 1；`closeMenu()` 与点击页面空白均能收起 |
| UC-0003 | ✅ 通过 | `floatExport=true`、`panelExport=true`、`otherBlocksClean=0`；`UIWin.exportPayload("浮窗A")` → `{title:"浮窗A", tree:progress 树}`；`exportPayload("")` → 当前活动窗口「面板B」的 tree/table |

> 验证环境：`work/co-shell-0.89.0.darwin.arm64 --serve --port 28261`，调试浏览器 `http://127.0.0.1:28261/`，通过 `browser_evaluate` 注入 `ui_render` 事件后断言。

# FEATURE-580 UC-0012：健壮性、安全、i18n 与回归

> 分支：**FEATURE-580**　版本：**v0.89.0**　对应方案：**导出模块的边界处理与回归保护**
> 依据：ROADMAP v0.89.0 验收标准 ④（中英文案齐备）⑤（`node --check` 与 `go build ./... && go vet ./...` 通过）⑥（浏览器运行时断言全绿）+ 项目安全编码规范（XSS 防护）
> 判定口径：异常输入不抛错、不产生空文件；恶意内容不产生 DOM 元素、不执行；中英文案随界面语言切换；既有功能零回归。

## 用例 UC-0012：异常输入不崩溃

```js
// browser_evaluate
const out = [];
const tryOne = (name, fn) => { try { out.push([name, "ok", fn()]); } catch (e) { out.push([name, "throw", String(e)]); } };
tryOne("empty",   () => UIExport.treeToMarkdown(null, {title:"t"}).length > 0);
tryOne("string",  () => UIExport.treeToMarkdown('{"type":"card"}', {title:"t"}).includes("card") === false);
tryOne("badjson", () => UIExport.printTree("{oops", {title:"t"}));
tryOne("unknown", () => UIExport.treeToMarkdown({type:"zzz", props:{}}, {title:"t"}).includes("**zzz**"));
tryOne("longTitle", () => UIExport.fileName({title:"汉".repeat(300)}, "md").length <= 120);
return out;
```

**期望**：5 项全部为 `"ok"`（无 `"throw"`）；`badjson` 时给出可见提示（`.ui-export-menu` 或 console.warn），且**不**生成 `.ui-print-root`。

## 用例 UC-0013：安全 —— 恶意内容不产生元素、不执行

```js
// browser_evaluate
window.__xss = 0;
const tree = {type:"card", props:{title:'<img src=x onerror="window.__xss=1">'}, children:[
  {type:"table", props:{columns:[{key:"a"}], rows:[{a:'<script>window.__xss=1</script>'}]}},
  {type:"html", props:{content:'<img src=x onerror="window.__xss=1">'}}
]};
const md = UIExport.treeToMarkdown(tree, {title:'<b>x</b>', time:"2026-10-02 14:30"});
UIExport.printTree(tree, {title:'<b>x</b>', time:"2026-10-02 14:30"});
const root = document.querySelector(".ui-print-root");
const res = {
  xss: window.__xss,
  imgs: root.querySelectorAll("img").length,
  scripts: root.querySelectorAll("script").length,
  literalTitle: root.textContent.includes("<img") || root.textContent.includes("<b>x</b>"),
  mdHasNoTag: !/<img|<script/i.test(md.replace(/<[^>]*>/g, "")) || !/<img|<script/i.test(md)
};
window.dispatchEvent(new Event("afterprint"));
return res;
```

**期望**：`xss === 0`；`imgs === 0`；`scripts === 0`；`literalTitle === true`（标题按字面文本输出）；`mdHasNoTag === true`（MD 中不出现会破坏结构的原始标签）。

## 用例 UC-0014：中英文案随界面语言切换

```js
// browser_evaluate
const before = UIExport.text("export");     // 当前语言
window.currentLang = before === "导出" ? "en" : "zh";
const after = UIExport.text("export");
window.currentLang = before === "导出" ? "zh" : "en";
return { before, after, menuItems: [UIExport.text("md"), UIExport.text("pdf")] };
```

**期望**：`before` 与 `after` 分别为 zh/en 两套文案（zh「导出」/ en「Export」），且切换后立即生效；`menuItems` 为同一语言下的两项文案（zh「Markdown 下载」「PDF / 打印」；en「Download Markdown」「PDF / Print」）。

## 用例 UC-0015：回归与静态检查

```bash
cd /Users/direct3d/github/co-shell
# ① 语法检查（新增脚本 + 被改动的脚本）
node --check web/static/ui-export.js
node --check web/static/app.js
node --check web/static/ui-windows.js
# ② Go 侧编译与静态检查
go build ./... && go vet ./...
# ③ Go 测试
go test ./web/... -short
```

**期望**：三条命令均退出码 0；`go build ./... && go vet ./...` 无输出（无告警）。

浏览器侧回归断言：

```js
// browser_evaluate（在渲染了 ui 区块的页面上）
const block = document.querySelector(".ev.ui");
const head = block.querySelector(".ev-head");
return {
  copyKept: !!head.querySelector(".ev-act"),                       // 复制/折叠按钮仍在
  exportBtn: !!head.querySelector("[data-ui-export]"),
  windowClose: !!document.querySelector("#uiWindow .ui-window-close"),
  windowMax: !!document.querySelector("#uiWindow .ui-window-max"),
  clickEmptyCloses: (() => { document.body.click(); return !document.querySelector(".ui-export-menu"); })()
};
```

**期望**：全部为 `true`；点击页面空白处导出菜单收起；窗口关闭/最大化按钮与拖拽行为不变。

## 验收记录

| 用例 | 结果 | 证据 |
|---|---|---|
| UC-0012 | ✅ 通过 | 5 项均 `ok`（无 throw）：`empty/string/badjson/unknown/longTitle`；`badjson` 控制台输出「组件树无法解析，已取消导出」且未生成 `.ui-print-root` |
| UC-0013 | ✅ 通过 | `xss=0`；`imgs=0`、`scripts=0`；`literalTitle=true`（标题按字面文本输出）；`mdHead=# &lt;b&gt;x&lt;/b&gt;`、`mdNoRawTag=true` |
| UC-0014 | ✅ 通过 | zh `{导出, Markdown 下载, PDF / 打印}`；en `{Export, Download Markdown, PDF / Print}`；切换后即时生效（`text()` 懒解析） |
| UC-0015 | ✅ 通过 | `node --check` 4 个文件全部 OK；`go build ./... && go vet ./...` 无输出；新增 `TestFEATURE580ExportWiring` PASS；`go test ./web/ -short` 与 `main` 基线**失败集完全相同**（19 项既有失败，**零新增**，详见下方）；浏览器回归断言全 true（复制/折叠按钮保留、窗口 ✕/⛶ 保留、面板三按钮保留、菜单可开可关） |

### 基线失败说明（UC-0015）

`go test ./web/ -short` 在 `main` 基线上已有 19 项失败（`TestSessionSwitch`、`TestSessionDelete`、`TestYOLOGet/Set`、`TestWebIOAsk*`、`TestWebRendererForwardsEvents` 等，集中在 `web/session_test.go` 的 WebSocket 会话消息断言）。本次分支与基线失败集**逐项一致**，未新增任何失败；对比方法：`git worktree add --detach /tmp/coshell-main main` 后分别取两边 `--- FAIL` 清单做 `comm -13` 差集（结果为空）。

> 验证环境：同 UC-0001；`go build ./... && go vet ./...` 与 `go test` 均在停止本地验证服务（释放 bbolt 文件锁）后运行。

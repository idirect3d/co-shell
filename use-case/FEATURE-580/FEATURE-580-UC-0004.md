# FEATURE-580 UC-0004：Markdown 导出内容与页眉

> 分支：**FEATURE-580**　版本：**v0.89.0**　对应方案：**组件树 → Markdown 转换（`UIExport.treeToMarkdown`）**
> 依据：ROADMAP v0.89.0 验收标准 ②（MD 下载为 UTF-8 `.md`，含页眉，组件语义完备）
> 判定口径：给定组件树，`UIExport.treeToMarkdown(tree, {title, time})` 的返回文本与下方映射规范**逐字一致**（忽略行尾空白）。

## 目的

把「组件树 → Markdown」的映射固化为可逐字核对的规范，作为 MD 导出的验收基线。

## 映射规范（v1）

页眉（始终输出）：

```
# {title}

*co-shell · {yyyy-MM-dd HH:mm}*
```

- `title` 为空/未提供时用 i18n 默认标题（zh「render_ui 导出」/ en「render_ui export」）。
- 时间由调用方传入（下载与打印共用同一时间戳来源，便于用例固定断言）。

正文映射：

| 组件 | Markdown 输出 |
|---|---|
| card | `### {title}`（有 title）+ `*{subtitle}*`（有 subtitle）+ 空行 + 子节点（无 title 时直接输出子节点） |
| row / col | 无自身文本，按顺序输出子节点（col 之间空行分隔） |
| kv | 每项一行 `- **{k}**: {v}` |
| table | 标准 Markdown 表：表头 = `columns[].label \|\| key`，分隔行 `\| --- \|`，数据行按 `rows` |
| chart | `#### {series.name}`（无 name 时 `#### {title}`；均无时 `#### chart`）+ 2 列表格（表头 zh「标签/值」en「Label/Value」，有 `unit` 时值表头为「值（单位：{unit}）」），行 = `data[]` |
| steps | 有序列表：`1. ✅ {title}`（done）/ `1. 🔄 {title}`（active）/ `1. ⬜ {title}`（pending，默认），`desc` 非空时追加 ` — {desc}` |
| callout | 引用块：`> {variantEmoji} **{title}**`（无 title 时省略 `**…**`）+ `> {text}`；emoji：info `ℹ️` / warn `⚠️` / success `✅` / error `❌` |
| progress | `**{label}**: {value}/{max} ({pct}%)`；`max` 缺省 100，`label` 为空时用 `progress` |
| file | `- 📄 {path}`，`name`/`size` 存在时追加 `（{name}，{size}）` |
| form | `### {title}`（有 title）+ 每字段 `- **{label}**（{type}）: {value}` |
| figure | `**{title}**`（无 title 时 `**figure**`）+ 换行 `*（矢量图，请在网页或 PDF 中查看）*` |
| html | `*（内嵌 HTML 内容，请在网页或 PDF 中查看）*` |
| 未知 type | 一行 `**{type}**` + 子节点（不报错） |

## 用例 UC-0004：容器与数据类组件

```js
// browser_evaluate
const tree = {type:"card", props:{title:"销售分析", subtitle:"2026 Q3"}, children:[
  {type:"kv", props:{items:[{k:"华东", v:320},{k:"华南", v:180}]}},
  {type:"table", props:{columns:[{key:"region", label:"区域"},{key:"amount"}], rows:[{region:"华东", amount:320},{region:"华南", amount:180}]}},
  {type:"file", props:{path:"output/报表.xlsx", name:"报表.xlsx", size:"12 KB"}}
]};
return UIExport.treeToMarkdown(tree, {title:"销售分析", time:"2026-10-02 14:30"});
```

**期望**（逐字）：

```
# 销售分析

*co-shell · 2026-10-02 14:30*

### 销售分析

*2026 Q3*

- **华东**: 320
- **华南**: 180

| 区域 | amount |
| --- | --- |
| 华东 | 320 |
| 华南 | 180 |

- 📄 output/报表.xlsx（报表.xlsx，12 KB）
```

## 用例 UC-0005：图表 / 步骤 / 提示 / 进度

```js
// browser_evaluate
const tree = {type:"row", children:[
  {type:"chart", props:{kind:"bar", unit:"万元", series:[{name:"销售额", data:[{label:"华东", value:320},{label:"华南", value:180}]}]}},
  {type:"steps", props:{items:[{title:"收集", status:"done"},{title:"清洗", desc:"去重与补全", status:"active"},{title:"建模"}]}}
]};
const extra = {type:"col", children:[
  {type:"callout", props:{variant:"warn", title:"注意", text:"样本量偏小"}},
  {type:"progress", props:{value:3, max:4, label:"进度"}}
]};
return UIExport.treeToMarkdown({type:"card", children:[tree, extra]}, {title:"进度报告", time:"2026-10-02 14:30"});
```

**期望**：包含

```
#### 销售额

| 标签 | 值（单位：万元） |
| --- | --- |
| 华东 | 320 |
| 华南 | 180 |
```

```
1. ✅ 收集
2. 🔄 清洗 — 去重与补全
3. ⬜ 建模
```

```
> ⚠️ **注意**
> 样本量偏小
```

```
**进度**: 3/4 (75%)
```

## 用例 UC-0006：布局嵌套与表单

```js
// browser_evaluate
const tree = {type:"row", children:[
  {type:"col", children:[{type:"steps", props:{items:[{title:"A", status:"done"}]}}]},
  {type:"col", children:[{type:"form", props:{title:"参数", fields:[
    {name:"n", label:"样本数", type:"number", value:100},
    {name:"mode", label:"模式", type:"select", options:[{value:"a", label:"A"}], value:"a"},
    {name:"ok", label:"确认", type:"checkbox", value:true}
  ]}}]}
]};
return UIExport.treeToMarkdown(tree, {title:"表单", time:"2026-10-02 14:30"});
```

**期望**：包含 `### 参数` 与

```
- **样本数**（number）: 100
- **模式**（select）: a
- **确认**（checkbox）: true
```

且 `1. ✅ A` 出现在表单之前（row 内按 col 顺序）。

## 用例 UC-0007：页眉默认值、降级与文件名

```js
// browser_evaluate
const md1 = UIExport.treeToMarkdown({type:"card", children:[
  {type:"figure", props:{title:"受力图"}, children:[{type:"point", props:{x:1,y:2}}]},
  {type:"html", props:{content:"<b>hi</b>"}},
  {type:"mystery", props:{}}
]}, {});
return {
  md1,
  defTitleZh: /^# render_ui 导出/.test(md1),
  figure: md1.includes("**受力图**") && md1.includes("矢量图"),
  html: md1.includes("内嵌 HTML 内容"),
  unknown: md1.includes("**mystery**"),
  fileName: UIExport.fileName({title:"销售/分析 报告"}, "md")
};
```

**期望**：`defTitleZh === true`；`figure === true`；`html === true`；`unknown === true`；`fileName` 形如 `销售_分析_报告-20261002-1430.md`（非法文件名字符 `/ \ : * ? " < > |` 与空格替换为 `_`，时间戳为 `yyyyMMdd-HHmm`），且**不含路径分隔符**。

## 验收记录

| 用例 | 结果 | 证据 |
|---|---|---|
| UC-0004 | ✅ 通过 | `match=true`（与期望文本**逐字一致**，含页眉、card/subtitle、kv、表头 `区域 \| amount`、file 行） |
| UC-0005 | ✅ 通过 | `chart=true`（`#### 销售额` + `\| 标签 \| 值（单位：万元） \|` 表）、`steps=true`（✅/🔄/⬜ 与 `— desc`）、`callout=true`（`> ⚠️ **注意**`）、`progress=true`（`**进度**: 3/4 (75%)`） |
| UC-0006 | ✅ 通过 | `formHead=true`、`fields=true`（number/select/checkbox 三行）、`order=true`（`1. ✅ A` 在 `### 参数` 之前，row 内按 col 顺序） |
| UC-0007 | ✅ 通过 | `defTitleZh=true`（`# render_ui 导出`）、`figure=true`、`html=true`、`unknown=true`；`fileName({"销售/分析 报告"}) = 销售_分析_报告-20261002-1421.md`；空标题 → `co-shell-export-20261002-1421.md`；300 字标题 → 长度 77 ≤ 120 |

> 验证环境：同 UC-0001（`browser_evaluate` 直调 `UIExport.treeToMarkdown`，固定 `time` 保证可复现）。

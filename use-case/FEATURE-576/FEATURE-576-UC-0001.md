# FEATURE-576 UC-0001：对话流中的本地文件路径可点击预览（识别与三处触发位置）

> 分支：**FEATURE-576**　版本：**v0.86.0**　对应需求：① LLM 正文　② 工具返回内容　③ 工具调用参数
> 判定口径：`.md` 容器内出现的**受支持文件类型**的路径被渲染为 `<a class="md-path" data-path="…">`，
> 单击即在预览面板打开该文件（文本/代码/markdown 走文本预览器，图片走图片预览器）。

## 目的

锁定「路径识别 → 可点击渲染 → 点击打开预览」的完整链路，并覆盖用户指定的三处触发位置。

## 设计基线（本分支实现，供确认）

| 项 | 取值 | 说明 |
|---|---|---|
| 注入点 | `md.js` 新增 `pathLinkNodes(text)` | LLM 正文（`scheduleMd(curLLM)`）、工具结果（`scheduleMd(curTool)`）、工具输入参数（`renderParams`→`mdRender`）**三处统一走 `mdRender`→`mdInline`**，一处接入即三处覆盖 |
| 覆盖的文本出口 | 段落 / 标题 / 列表 / 表格 / 引用 / **内联代码** / **加粗·斜体·删除线内部** / **围栏代码块** | 全部文本出口统一调用同一函数，行为一致 |
| 识别前提 | ① 至少含一个 `/`；② 去掉可选 `:行号[:列号]` 后缀后，扩展名属于 `IMAGE_EXT ∪ TEXT_EXT` | 仅有文件名（`app.js`）**不识别**；未知扩展名（`foo.xyz`）**不识别** |
| 不误伤 | `https://…` URL、`bin/`（无扩展名）、`1.5/2`、`foo/bar.xyz` | URL 显式拒绝（含 `://` 即跳过） |
| 行号后缀 | `app.js:5267` 的 `:5267` 保留为**链接文本**，`data-path` 只存纯路径 | 点击打开文件，**暂不跳到该行**（见待确认取舍 D4） |
| 点击行为 | 图片扩展名 → `openImagePreview`；其余 → `openFilePreview` | 与工作区文件树完全同一套预览入口（含 `view_file` 动态事件上报） |
| 未改造 | TOOL 块标题的 intent 摘要（纯文本标题）、diff 行、Raw 模式、`ui_text`(REPL/SYS) 块 | 见待确认取舍 D3 |
| 标签粘连 | 候选正则本身排除中文/散文标点，路径前的标签不会进入候选（`正文：web/a.js`、`见` + 反引号 + `web/a.js`） | 依据：首个 `/` 之前的最后一个分隔符（`：:，,；;、（）()「」"'` 等）之后才是路径 |
| 已知限制 | 标签与路径之间**既无分隔符也无空格**时（如 `见web/a.js`）可能整体不识别 | 宁可漏认，也不误吞前文生成错误链接 |

## 前置条件

- 启动：`./work/co-shell-0.86.0.darwin.arm64 --serve --port 28256`
- 用调试浏览器打开该地址；工作区为 `/Users/direct3d/github/co-shell`。

## 用例 A：渲染层识别与不误伤（不依赖真实会话，直接调 `mdRender`）

### 脚本 A1

```js
(() => {
  const box = document.createElement('div');
  box.className = 'md';
  box.style.position = 'fixed'; box.style.left = '-9999px';
  document.body.appendChild(box);
  mdRender(box, [
    '正文：/Users/direct3d/github/co-shell/web/static/app.js:5267 与 web/static/md.js',
    '',
    '内联代码：`research/token优化设计方案.md`',
    '',
    '**加粗：web/static/app.js** 与 *斜体：./bin/doc2md.py*',
    '',
    '```',
    './bin/md2docx.py',
    '```',
    '',
    '反例：https://example.com/a/b.js 与 app.js 与 foo/bar.xyz 与 bin/ 与 1.5/2',
  ].join('\n'));
  const links = [...box.querySelectorAll('a.md-path')].map(a => a.dataset.path);
  const labels = [...box.querySelectorAll('a.md-path')].map(a => a.textContent);
  const anchors = box.querySelectorAll('a').length;
  box.remove();
  return { links, labels, anchors };
})()
```

### 预期结果

| 断言 | 期望 |
|------|------|
| A1-1 | `links` 恰好为 6 项：`/Users/direct3d/github/co-shell/web/static/app.js`、`web/static/md.js`、`research/token优化设计方案.md`、`web/static/app.js`、`./bin/doc2md.py`、`./bin/md2docx.py`（顺序一致） |
| A1-2 | 第一条链接的 `label` 为 `…/app.js:5267`（行号可见），而 `dataset.path` **不含** `:5267` |
| A1-3 | `links` 中**不含** `example.com/a/b.js`（URL 不误伤）、不含 `app.js`（无路径不识别）、不含 `foo/bar.xyz`（未知扩展名） |
| A1-4 | `anchors === links.length === 6`（未产生多余 `<a>`） |
| A1-5 | 中文文件名（`token优化设计方案.md`）被正确整体识别 |

## 用例 B：三处触发位置的真实会话验证

### 步骤

1. 在 Web UI 发起一个会同时产生三处输出的任务，例如：
   `读取 web/static/md.js 前 40 行，然后列出 web/static 目录，再执行 ls -l web/static`
2. 等该轮任务结束（回放完成），执行脚本 B1。

### 脚本 B1

```js
(() => {
  const all   = document.querySelectorAll('#stream a.md-path').length;
  const llm   = document.querySelectorAll('#stream .ev.llm a.md-path').length;
  const toolAll = document.querySelectorAll('#stream .ev.tool a.md-path').length;
  const params  = document.querySelectorAll('#stream .ev.tool .tool-params-body a.md-path').length;
  const result  = document.querySelectorAll('#stream .ev.tool .ev-body a.md-path').length;
  const sample = [...document.querySelectorAll('#stream a.md-path')].slice(0, 5)
                   .map(a => a.dataset.path);
  return { all, llm, toolAll, params, result, sample };
})()
```

### 预期结果

| 断言 | 期望 |
|------|------|
| B1-1 | `llm > 0`：LLM 正文里的路径可点击（触发位置①） |
| B1-2 | `params > 0`：工具**输入参数**区（含命令中的路径）可点击（触发位置③） |
| B1-3 | `result > 0`：工具**返回内容**（`read_file` / `list_files` / `execute_command` 输出）中的路径可点击（触发位置②） |
| B1-4 | `toolAll === params + result`；`all === llm + toolAll`（无重复计数、无遗漏容器） |
| B1-5 | 刷新页面后重新回放，`sample` 与刷新前一致（持久化事件走同一条 `renderEvent`→`mdRender` 路径） |

## 用例 C：点击打开预览

### 脚本 C1（文本文件）

```js
(async () => {
  const box = document.createElement('div');
  box.className = 'md'; box.style.position = 'fixed'; box.style.left = '-9999px';
  document.body.appendChild(box);
  mdRender(box, '打开 web/static/md.js');
  const a = box.querySelector('a.md-path');
  a.click();
  await new Promise(r => setTimeout(r, 800));
  const out = { label: a.textContent, fvPath: (typeof fvPath !== 'undefined' ? fvPath : null) };
  box.remove();
  return out;
})()
```

### 脚本 C2（图片，把路径换成工作区内实际存在的 png）

```js
(async () => {
  const box = document.createElement('div');
  box.className = 'md'; box.style.position = 'fixed'; box.style.left = '-9999px';
  document.body.appendChild(box);
  mdRender(box, '图片 logos/logo-dark.png');
  const a = box.querySelector('a.md-path');
  a.click();
  await new Promise(r => setTimeout(r, 800));
  const out = { label: a.textContent, ivPath: (typeof ivPath !== 'undefined' ? ivPath : null),
                imgSrc: document.querySelector('#imgViewer img') ? document.querySelector('#imgViewer img').src : '' };
  box.remove();
  return out;
})()
```

### 预期结果

| 断言 | 期望 |
|------|------|
| C1-1 | 点击后 `fvPath === 'web/static/md.js'`，中部预览面板显示该文件内容（与在左侧文件树单击同一文件的效果一致） |
| C1-2 | 再次点击同一链接 → 预览关闭（沿用 `openFilePreview` 的同文件再点关闭语义） |
| C2-1 | 图片链接点击后 `ivPath` 为该图片路径，图片查看器可见且图片加载成功（`imgSrc` 含 `api/file?path=`） |
| C2-2 | 打开预览时后端收到 `dynamic_event kind=view_file`（与文件树单击行为一致） |

## 结果记录

| 用例 | A1-1 | A1-2 | A1-3 | A1-4 | A1-5 | B1-1 | B1-2 | B1-3 | B1-4 | B1-5 | C1-1 | C1-2 | C2-1 | C2-2 |
|------|------|------|------|------|------|------|------|------|------|------|------|------|------|------|
| 结果 | | | | | | | | | | | | | | |

> 执行人：____　日期：____　结论：□ 通过　□ 失败

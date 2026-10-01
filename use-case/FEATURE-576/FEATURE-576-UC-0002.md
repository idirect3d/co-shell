# FEATURE-576 UC-0002：工作区外绝对路径的只读预览与安全边界

> 分支：**FEATURE-576**　版本：**v0.86.0**　对应需求：文件范围＝「工作区内文件 + 任意本地绝对路径」
> 判定口径：文本中出现的合法本地绝对路径（受支持扩展名）可点击，点击后能在预览面板打开，
> 且**相对路径仍严格限制在工作区内**（路径穿越必须被拒绝）。

## 目的

锁定后端对绝对路径的放开范围与安全底线，覆盖：

- 绝对路径的只读预览可用（`/api/file`、`/api/gitdiff`、`/api/gitfirstchange`）；
- 相对路径的路径穿越（`../../etc/passwd`）仍然被拒绝；
- 前端渲染为 DOM 节点（`textContent`），不产生 HTML 注入；
- 大输出下的渲染开销可接受。

## 设计基线（本分支实现，供确认）

| 项 | 取值 | 说明 |
|---|---|---|
| 后端 | **不改动** | 用户拍板方案C：只读预览 API 继续严格限制在工作区内（`resolvePath` 不变） |
| 前端路径归一 | `previewPathOf(raw)` | 绝对路径仅当以工作区根（`/api/bootstrap.workspace`）开头时接受，并转换为**工作区相对路径**；其余绝对路径（`/etc/hosts`、工作区外文件）**不生成链接** |
| 安全底线 | 相对路径穿越仍被拒绝 | `../../etc/passwd` → 403（既有 `errPathOutside` 行为，不因本特性改变） |
| 前端 DOM | `createElement("a")` + `textContent`，路径只存 `dataset.path` | `href` 不承载真实路径也不承载可执行 scheme，路径不会成为 URL |
| 行号 | `app.js:5267` → 打开后调用 `scrollToLine(5267)`（按需加载分块，同 `loadFirstChange` 策略） | 用户确认 D4＝要做 |
| 不支持形态 | `~/xxx`、Windows 盘符路径、工作区外绝对路径、含空格未加引号的路径 | 不做识别/不保证可预览 |

## 已确认决策（2026-10-02 用户拍板）

| 编号 | 取舍点 | 结论 |
|------|--------|------|
| D1 | 绝对路径放开范围 | **方案C**：仅允许工作区内的绝对路径（工作区外绝对路径不识别、不预览） |
| D2 | 围栏代码块内是否识别 | **识别**（与正文一致） |
| D3 | TOOL 块标题 intent 摘要 | **本次不改**（参数区已覆盖路径） |
| D4 | 行号后缀是否跳行 | **要做**（复用 `scrollToLine`） |

## 前置条件

- 启动：`./work/co-shell-0.86.0.darwin.arm64 --serve --port 28256`（回环）
- 工作区根：`/Users/direct3d/github/co-shell`

## 用例 A：绝对路径只读预览（后端，命令行验证）

```bash
# A1 工作区外的合法文件（绝对路径）
curl -s -o /dev/null -w '%{http_code}\n' \
  --get --data-urlencode 'path=/etc/hosts' 'http://127.0.0.1:28256/api/file'
# A2 工作区内文件的绝对路径
curl -s -o /dev/null -w '%{http_code}\n' \
  --get --data-urlencode 'path=/Users/direct3d/github/co-shell/main.go' 'http://127.0.0.1:28256/api/file'
# A3 相对路径（既有行为，不受影响）
curl -s -o /dev/null -w '%{http_code}\n' \
  --get --data-urlencode 'path=web/static/md.js' 'http://127.0.0.1:28256/api/file'
# A4 相对路径穿越（必须仍被拒绝）
curl -s -o /dev/null -w '%{http_code}\n' \
  --get --data-urlencode 'path=../../etc/passwd' 'http://127.0.0.1:28256/api/file'
# A5 分片模式下的绝对路径（文本预览器走这条）
curl -s -o /dev/null -w '%{http_code}\n' \
  --get --data-urlencode 'path=/etc/hosts' --data-urlencode 'start=1' --data-urlencode 'end=20' \
  'http://127.0.0.1:28256/api/file'
```

### 预期结果

| 断言 | 期望 |
|------|------|
| A-1 | A3（工作区相对路径）**200**；A4（穿越）**403** |
| A-2 | A1（`/etc/hosts`）、A2（工作区文件的绝对路径）**均非 200**（现状 404：`resolvePath` 把绝对路径当作工作区内相对路径拼接）——所以工作区内的绝对路径必须由前端转换为相对路径后才能预览 |
| A-3 | A4 的响应体错误信息为通用的「路径越界」提示，**不泄露**工作区绝对路径等实现细节（原有 `errPathOutside` 文案） |
| A-4 | A5（分片模式的绝对路径，`start/end`）同样非 200，与 A1/A2 一致 |

## 用例 B：绝对路径在对话流中点击预览

### 步骤

1. 在 Web UI 对话流中出现（或手工用脚本注入）一个**工作区内**文件的绝对路径，例如
   `/Users/direct3d/github/co-shell/web/static/md.js:120`。
2. 执行脚本 B1 并点击。

```js
async function clickPath(jsLiteral) {
  const box = document.createElement('div');
  box.className = 'md'; box.style.position = 'fixed'; box.style.left = '-9999px';
  document.body.appendChild(box);
  mdRender(box, jsLiteral);
  const a = box.querySelector('a.md-path');
  const out = { found: !!a, label: a && a.textContent, dataPath: a && a.dataset.path,
                dataLine: a && a.dataset.line };
  if (a) {
    a.click();
    await new Promise(r => setTimeout(r, 1200));
    out.fvPath = (typeof fvPath !== 'undefined' ? fvPath : null);
    out.viewerVisible = !!document.querySelector('#fileViewer') &&
                        !document.querySelector('#fileViewer').classList.contains('hidden');
    out.scrollTop = (typeof fvBody !== 'undefined' && fvBody) ? fvBody.scrollTop : null;
  }
  box.remove();
  return out;
}
clickPath('打开 /Users/direct3d/github/co-shell/web/static/md.js:120');
```

### 预期结果

| 断言 | 期望 |
|------|------|
| B-1 | `found === true`；`dataPath === 'web/static/md.js'`（**绝对→相对已转换**）；`dataLine === '120'` |
| B-2 | 点击后 `fvPath === 'web/static/md.js'`，预览器可见并显示该文件内容 |
| B-3 | 预览打开后滚动位置**不为 0**（已按行号跳转，第 120 行落在可视区上 1/3）；工作区文件的 `/api/gitdiff`、`/api/gitfirstchange` 正常（无错误） |
| B-4 | 注入工作区外绝对路径 `打开 /etc/hosts` → `found === false`（不生成链接，用户已拍板方案C） |
| B-5 | 注入不存在的相对路径 `打开 web/nope-576.go` → 可点击但点击后页面不崩、控制台无未捕获异常，预览区给出加载失败提示 |

## 用例 C：转义与注入防护

### 脚本 C1

```js
(() => {
  const box = document.createElement('div');
  box.className = 'md';
  document.body.appendChild(box);
  window.__xss576 = 0;
  mdRender(box, 'text <img src=x onerror="window.__xss576=1"> web/a.png</b>');
  const out = {
    img: box.querySelectorAll('img').length,
    xss: window.__xss576,
    text: box.textContent,
    links: [...box.querySelectorAll('a.md-path')].map(a => a.dataset.path),
    href: box.querySelector('a.md-path') ? box.querySelector('a.md-path').getAttribute('href') : null,
  };
  box.remove();
  return out;
})()
```

### 预期结果

| 断言 | 期望 |
|------|------|
| C-1 | `img === 0` 且 `xss === 0`（输入中的标签只作为**文本**显示） |
| C-2 | `text` 原样包含 `<img src=x onerror="window.__xss576=1">` |
| C-3 | `links === ['web/a.png']`（路径识别不受相邻标记影响） |
| C-4 | `href` 不是真实路径、也不是 `javascript:` 等可执行 scheme（应为 `null` 或 `#`） |
| C-5 | 含引号/反引号/中文的路径（如 `research/token优化设计方案.md`）标签文本与 `data-path` 完全等于原路径 |

## 用例 D：大输出渲染开销（软指标）

### 脚本 D1

```js
(() => {
  const box = document.createElement('div');
  box.className = 'md';
  document.body.appendChild(box);
  let text = '';
  for (let i = 0; i < 500; i++) text += 'line ' + i + ': web/static/app.js:' + i + ' 与 /tmp/a/b.png\n';
  const t0 = performance.now();
  mdRender(box, text);
  const ms = performance.now() - t0;
  const links = box.querySelectorAll('a.md-path').length;
  box.remove();
  return { expectLinks: 1000, links, ms: Math.round(ms) };
})()
```

### 预期结果

| 断言 | 期望 |
|------|------|
| D-1 | `links === 1000`（每行 2 个路径全部识别） |
| D-2 | `ms < 500`（本机软指标；明显劣化需评估） |
| D-3 | 真实会话中流式输出含大量路径时，滚动与打字不出现卡顿（人工观察） |

## 待确认取舍（已全部拍板，见上「已确认决策」）

无。

## 结果记录

| 用例 | A-1 | A-2 | A-3 | A-4 | B-1 | B-2 | B-3 | B-4 | C-1 | C-2 | C-3 | C-4 | C-5 | D-1 | D-2 |
|------|-----|-----|-----|-----|-----|-----|-----|-----|-----|-----|-----|-----|-----|-----|-----|
| 结果 | | | | | | | | | | | | | | | |

> 执行人：____　日期：____　结论：□ 通过　□ 失败

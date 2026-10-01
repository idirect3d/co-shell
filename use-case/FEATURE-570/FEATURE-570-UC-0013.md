# FEATURE-570 用例组 D/E/F：函数曲线、安全鲁棒、集成兼容（UC-0013 ~ UC-0024）

- 分支：FEATURE-570（v0.84.0）｜类型：运行时 + 单元（单元项在 `agent/uitree_test.go`）
- 前置：同 UC-0001（`--serve` + Web UI）。

## 组 D：函数曲线（表达式求值）

## FEATURE-570-UC-0013 基本函数曲线

**tree**：`function{expr:"sin(x)",from:0,to:6.2832,samples:160}` + `function{expr:"x^2/4",from:-4,to:4,samples:80,stroke:"info"}`。
**步骤**：渲染 → 检查折线点数量与形状。
**期望**：正弦呈标准一周期波形（-1..1）；抛物线开口向上且对称；折线点数 = samples（或 samples+1，实现选一并在用例记录）；无 JS 报错。

## FEATURE-570-UC-0014 参数方程与 3D 函数曲线

**tree**：`function{expr:["cos(t)","sin(t)"],var:"t",from:0,to:6.2832}`（单位圆）+ 一组带 z 的 `function`。
**期望**：二维参数方程画出闭合圆；带 z 时按 3D 投影绘制，曲线随视角变化。

## FEATURE-570-UC-0015 非法表达式的降级

**tree**：分别用 `expr:"window"`、`"constructor"`、`"eval('1')"`、`"sin("`、`"1/0"`、`"x**2"`（不支持符号）。
**步骤**：渲染 6 次 → 观察图内表现与控制台。
**期望**：每种非法表达式都**不产生任何 JS 异常、不白屏**；非法标识符/语法错误 `window`/`constructor`/`eval(...)`/`sin(`/`x**2` → 图内出现一行降级提示文本且其余图元正常绘制；`1/0` 的采样值非有限 → 该点被跳过（曲线断开，无 NaN 坐标写入 DOM）。

## FEATURE-570-UC-0016 表达式与采样上限

**步骤**：(1) `expr` 长度 501 字符 → 渲染；(2) `samples:5000` → 渲染；(3) `samples` 缺省 → 渲染。
**期望**：(1) 后端校验拒绝并返回可自纠错误（中英）；(2) 采样点被钳制到 400（DOM 中点数 ≤400，无卡顿）；(3) 使用默认采样数，曲线平滑。

## 组 E：安全与鲁棒

## FEATURE-570-UC-0017 XSS 字面化（无 innerHTML）

**tree**：`label{text:"<img src=x onerror=\"window.x=1\">"}` + `label{text:"<script>alert(1)</script>"}` + `figure{title:"<b>t</b>"}`。
**步骤**：渲染 → 检查 DOM（是否存在 `img`/`script` 元素）、检查 `window.x` 与是否有 alert。
**期望**：全部按**字面文本**渲染（`textContent`）；DOM 中不存在这些元素、无弹窗、`window.x` 未定义；`grep -n "innerHTML" web/static/ui-figure.js web/static/ui-expr.js` 无结果。

## FEATURE-570-UC-0018 无 eval / new Function

**步骤**：`grep -rn "eval(\|new Function\|setTimeout('\|setTimeout(\"" web/static/ui-*.js`。
**期望**：`ui-expr.js`/`ui-figure.js` 中**零匹配**；表达式求值全部走自建递归下降解析器。

## FEATURE-570-UC-0019 后端校验拒绝非法输入（单元）

**单元用例**（`go test ./agent/ -run UITree -v`）：
1. 图元 `point` 直接作为 `card` 的 child（非 figure 上下文）→ 拒绝，错误信息含允许的组件清单。
2. `figure` 的子节点 `{"type":"card"}` → 拒绝（figure 只接受图元）。
3. 坐标 `x: null` / `"abc"` / `1e999`(Inf) → 拒绝。
4. `ellipse{rx:-1}`、`opacity:1.5`、`strokeWidth:0` / `9` → 拒绝。
5. 点数 401 的 `polygon` → 拒绝；图元总数 201 → 拒绝。
6. 合法 tree（8 种图元各一 + 嵌套在 card 内）→ 通过。

**期望**：全部符合上述判定；错误信息为 i18n（中/英各测一次）；通过的 tree 能被 `render_ui` 正常渲染。

## FEATURE-570-UC-0020 大规模图元性能

**tree**：`polygon` ×20（各 200 点）+ `function` ×3（各 400 采样）。
**期望**：渲染耗时 < 500ms（录屏/控制台计时）；页面不卡顿、可正常滚动；节点数与声明一致（≤200 限制内）。

## 组 F：集成与兼容

## FEATURE-570-UC-0021 update 原地刷新

**步骤**：先渲染一个 `figure{id:"fig1"}`（含一个 `point{id:"p1"}`）→ 再 `render_ui(update:"fig1", tree:figure{…改动后的图元})`。
**期望**：图块**原地替换**（对话流中块数量不变、父节点与索引不变）；更新后图形为新内容；无重复块。

## FEATURE-570-UC-0022 图元 actions 点击回传

**tree**：`figure` + `point{id:"p1", label:"点我"}` 上声明 `actions:[{on:"click",id:"pick",payload:"point"}]`。
**步骤**：(1) 普通 `render_ui`（不等待）→ 点击该点；(2) `render_ui(waiting:true)` → 点击该点。
**期望**：(1) 动作作为新一轮用户消息注入（`[界面操作] …` 形式），agent 能读到 `ui_id`/`action_id`/payload；(2) 等待中的调用**立即返回**该动作作为工具结果，同一回合继续。

## FEATURE-570-UC-0023 下载 SVG

**步骤**：点击 figure 右上角「下载 SVG」→ 打开下载文件。
**期望**：文件名含 `figure`；文件为合法 SVG（浏览器可打开、图形与页面一致）；颜色为**实际颜色值**（不依赖 CSS 变量，在无样式的查看器中同样正确）；`download:false` 时按钮不出现。

## FEATURE-570-UC-0024 主题、响应式与降级

**步骤**：(1) 依次切换 dark / dark-muted / light / eyecare 四套主题并截图；(2) 把浏览器窗口缩到 ~400px 宽；(3) 在终端（非 Web）会话中让 agent 渲染同一 figure，查看输出。
**期望**：(1) 四套主题下线条/填充/文字颜色均取自主题变量、对比度可读、无写死色；(2) 画布等比收缩，无横向溢出、无内容裁切；(3) 终端忽略该事件，正常显示 LLM 的纯文本回复，不报错、不中断。

## 实测记录
- 时间 / 实例 / BUILD：
- 结果：

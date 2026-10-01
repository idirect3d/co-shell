# FEATURE-570 用例组 A：基础图元渲染（UC-0001 ~ UC-0004）

- 分支：FEATURE-570（v0.84.0）｜类型：运行时（Web UI 实测）
- 统一前置：`--serve` 启动 co-shell，`ui_enabled` 保持默认开；在 Web UI 会话中让 agent 用 `render_ui` 渲染下述 tree，然后观察渲染结果与 DOM。
- 说明：本文件含 4 条用例，每条以 `## FEATURE-570-UC-XXXX` 分隔。

## FEATURE-570-UC-0001 点与文字标注

**tree**
```json
{"type":"figure","props":{"xmin":0,"xmax":10,"ymin":0,"ymax":10},
 "children":[
   {"type":"point","props":{"x":2,"y":3,"label":"A(2,3)","filled":true}},
   {"type":"point","props":{"x":7,"y":8,"r":5,"stroke":"error"}},
   {"type":"label","props":{"x":5,"y":5,"text":"中点","anchor":"middle"}}
 ]}
```
**步骤**：渲染 → 读 DOM（`svg .ui-figure-point` 数量、`circle` 的 `r`/class）→ 目视位置。
**期望**：2 个点圆 + 1 段文字；A 在坐标 (2,3) 处（左下区域）、第二点在右上区域；标注文字居中；颜色分别随 `accent`（默认）与 `error` 主题色变化；无 JS 报错。

## FEATURE-570-UC-0002 线段与箭头

**tree**：4 条 `segment`，`arrow` 分别取 `false` / `"end"` / `"start"` / `"both"`，其中一条 `dash:"dashed"`、一条 `stroke:"info"`。
**步骤**：渲染 → 检查 DOM 中 `line` 与 arrow 标记元素（`marker` 或 `polygon`）→ 目视箭头方向。
**期望**：4 条线段端点与坐标一致；只在声明端出现箭头（方向由起点指向终点）；虚线生效；无非法的 marker 复用冲突（多条线段箭头互不串扰）。

## FEATURE-570-UC-0003 多边形与折线

**tree**：一个 `polygon`（`closed` 缺省，三角形，`fill:"info"`、`opacity:0.3`）+ 一个 `polygon`（`closed:false`，折线）+ 一个 `path`（5 点，`dash:"dotted"`）。
**步骤**：渲染 → 检查 `polygon`/`polyline` 元素与其 `points` 属性。
**期望**：默认 `closed:true` → 首尾闭合、有半透明填充；`closed:false` → 不闭合、无填充；`path` 与 `closed:false` 的多边形视觉一致；点数与声明一致。

## FEATURE-570-UC-0004 椭圆与圆

**tree**：`{"type":"ellipse","props":{"cx":4,"cy":4,"rx":3,"ry":1.5,"stroke":"warn"}}` + `{"type":"ellipse","props":{"cx":7,"cy":7,"rx":1,"ry":1,"fill":"success"}}`（rx==ry → 圆）。
**步骤**：渲染 → 检查 `ellipse` 元素的 `rx`/`ry` 换算（逻辑单位 → 像素，等比）。
**期望**：扁椭圆与正圆形状正确；正圆的像素 rx≈ry（等比映射不拉伸）；填充色为主题 success 色。

## 实测记录
- 时间 / 实例 / BUILD：
- 结果：

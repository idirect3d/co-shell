# FEATURE-570 用例组 B/C：坐标系与 3D 投影（UC-0005 ~ UC-0012）

- 分支：FEATURE-570（v0.84.0）｜类型：运行时（Web UI 实测）
- 前置：同 UC-0001（`--serve` + Web UI）。

## 组 B：坐标系与坐标轴

## FEATURE-570-UC-0005 逻辑坐标映射（y 轴向上）

**tree**：`figure{ xmin:0,xmax:10,ymin:0,ymax:10 }` + `point(0,0)`、`point(10,10)`、`point(0,10)`、`point(10,0)` 四个点（各自带 label）。
**步骤**：渲染 → 读取四个点的像素位置（`getBoundingClientRect` 或 DOM 属性）→ 判断象限。
**期望**：(0,0) 在**左下**、(10,10) 在**右上**、(0,10) 左上、(10,0) 右下——即 **y 轴向上**；四点到画布边界的余量等于统一内边距。

## FEATURE-570-UC-0006 非 1:1 范围保持等比（圆不被拉扁）

**tree A**：`figure{xmin:0,xmax:20,ymin:0,ymax:10}` + `ellipse{cx:10,cy:5,rx:2,ry:2}`。
**tree B**：同一 figure 改为 `width:400,height:400`。
**步骤**：渲染两个 tree，测量圆的像素宽/高。
**期望**：两种情况圆都保持正圆（像素宽≈高，误差 ≤1px）；坐标范围的长宽比不会导致非等比拉伸。

## FEATURE-570-UC-0007 超范围自动扩边（不裁剪）

**tree**：`figure{xmin:0,xmax:10,ymin:0,ymax:10}` + `point{x:15,y:5}`、`segment{x1:-5,y1:0,x2:0,y2:5}`。
**步骤**：渲染 → 检查图外点/线是否可见。
**期望**：坐标轴仍按 0..10 绘制，但超范围图元**自动扩边后完整可见**（不裁剪、不丢失），不出现图元被切掉一半的情况。

## FEATURE-570-UC-0008 坐标轴、网格与刻度

**tree**：`figure{axes:true,grid:true,unit:"cm"}` + 一个 `axes{origin:[0,0],xlabel:"x",ylabel:"y",ticks:5}`。
**步骤**：(1) 渲染 → 检查轴线、网格线、刻度文本数量；(2) 改为 `axes:false,grid:false` 再渲染。
**期望**：(1) 出现 x/y 轴线、网格线、带单位后缀的刻度数字；`axes` 图元的 `origin` 生效（轴线交点位于声明的原点）；刻度数量与 `ticks` 一致且数值正确。(2) 默认轴与网格都不出现，但显式 `axes` 图元仍按自身 props 绘制（用于画非原点的局部坐标系）。

## 组 C：3D 投影

## FEATURE-570-UC-0009 无 z → 2D 直出

**tree**：图元坐标全部只有 `[x,y]`。
**期望**：按 2D 绘制，不产生任何透视畸变；与 UC-0005 表现一致（回归保护）。

## FEATURE-570-UC-0010 有 z → 等轴测投影

**tree**：`figure{zmin:0,zmax:10}` + `point{x:0,y:0,z:0}`、`point{x:0,y:0,z:10}`、`point{x:10,y:0,z:0}`、`point{x:0,y:10,z:0}` 四点带 label。
**步骤**：渲染 → 检查四点相对位置。
**期望**：整张图切换到 3D 投影：z 轴上的点向上（或按 yaw/pitch 的投影方向）偏移；x/y 轴上的点向不同斜方向展开；四个点不重叠、可分辨。

## FEATURE-570-UC-0011 视角预设与自定义 yaw/pitch

**tree**：同一组 3D 图元，分别用 `view:"iso"` / `"front"` / `"top"` / `"side"`，以及 `yaw:60,pitch:20` 渲染 5 次。
**期望**：`front` 时 z 轴与前向重叠（正面视角）、`top` 时俯视（看不出高度差、x/y 平面正投影）、`side` 为侧视；`yaw/pitch` 覆盖预设且数值变化带来连续可预期的视角变化；5 次渲染均无报错。

## FEATURE-570-UC-0012 3D 坐标轴与底面网格

**tree**：`figure{axes:true,grid:true}` + 带 z 的图元。
**期望**：画出三轴（含 z 轴标注）与底面（z 最小值平面）网格；网格随 `view` 变化正确投影；`grid:false` 时网格消失但三轴保留。

## 实测记录
- 时间 / 实例 / BUILD：
- 结果：

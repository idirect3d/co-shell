# FEATURE-514-UC-0006 等长且构成周期重复时归类为 multi_line（不重复报等长）

## 前置条件
- 默认配置（阈值 100）

## 操作步骤
1. 构造输出：AB 两行交替重复 100 次（共 200 行，两行长度相同且构成周期重复）
2. 送入 `LoopDetector.AddChunk`（单元测试 `TestUniformLine_PeriodicTakesPrecedence`）

## 预期结果
- 触发一次循环事件，`LoopType == "multi_line"`（周期检测优先，先于等长检测命中）
- 不额外产生 `uniform_line_length` 事件（同一段输出只报一次，避免重复干预）

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败

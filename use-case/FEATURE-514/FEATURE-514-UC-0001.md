# FEATURE-514-UC-0001 连续 100 行等长触发多行等长检测

## 前置条件
- 已编译最新可执行码（`go build -o work/co-shell .`）
- 循环检测开启（`loop_intervention != off`）
- 默认配置（`loop_uniform_line_threshold = 100`）

## 操作步骤
1. 构造 LLM 输出：连续 100 行，每行字符长度完全相同（如每行 20 个字符），但**内容各不相同**（不构成周期重复）
2. 将该输出送入 `LoopDetector.AddChunk`（单元测试 `TestUniformLine_ExactThresholdTriggers`）
3. 运行时可让 LLM 产生同类等长输出（如逐行等宽的清单/列表），观察横幅

## 预期结果
- 第 100 行写入后 `AddChunk` 返回非 nil 错误，类型为 `*LoopDetectedError`
- `LoopType == "uniform_line_length"`
- 事件经 `applyLoopIntervention` 进入处理流程（开启 judge 时进入二次判定，SUP·循环判定 块可见）
- 前端/终端显示"检测到疑似循环内容（类型: 多行等长）"横幅

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败

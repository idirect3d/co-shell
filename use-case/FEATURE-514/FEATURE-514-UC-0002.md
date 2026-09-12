# FEATURE-514-UC-0002 连续 99 行等长不触发（阈值边界）

## 前置条件
- 默认配置（`loop_uniform_line_threshold = 100`）

## 操作步骤
1. 构造连续 99 行、每行长度完全相同、内容各不相同的输出
2. 送入 `LoopDetector.AddChunk`（单元测试 `TestUniformLine_JustBelowThreshold`）

## 预期结果
- `AddChunk` 返回 nil（不触发）
- 第 100 行写入后（延续同一段输出）才触发 `uniform_line_length`

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败

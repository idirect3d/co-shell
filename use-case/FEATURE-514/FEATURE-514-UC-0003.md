# FEATURE-514-UC-0003 行长不一致时不触发（防误报）

## 前置条件
- 默认配置（`loop_uniform_line_threshold = 100`）

## 操作步骤
1. 构造 100 行输出，其中**存在至少一行长度与其他行不同**（如 99 行 20 字符 + 1 行 21 字符）
2. 送入 `LoopDetector.AddChunk`（单元测试 `TestUniformLine_MixedLengthNoTrigger`）
3. 再构造正常的代码输出（缩进各异的语句）重复步骤 2

## 预期结果
- 两种情况均不产生 `uniform_line_length` 事件
- 正常代码输出不触发任何循环检测（无回归误报）

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败

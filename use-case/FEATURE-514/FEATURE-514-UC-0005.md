# FEATURE-514-UC-0005 关闭或阈值为 0 时不触发等长检测

## 前置条件
- 已编译最新可执行码（`go build -o work/co-shell .`）

## 操作步骤
1. `.set loop-uniform-line-threshold 0` 关闭该检测
2. 构造连续 200 行等长（内容各异）输出，送入 `LoopDetector.AddChunk`
3. 恢复阈值 `.set loop-uniform-line-threshold 100`，重复步骤 2

## 预期结果
- 阈值为 0 时不触发 `uniform_line_length`（单元测试 `TestUniformLine_DisabledNoTrigger`）
- 阈值恢复为 100 后，200 行等长输出触发
- 其余检测（周期/单行重复）不受影响

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败

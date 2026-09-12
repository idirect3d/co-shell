# FEATURE-514-UC-0008 既有循环检测无回归

## 前置条件
- 默认配置

## 操作步骤
1. 运行 `go test ./agent/ -run 'LoopDetector|Fix329' -v`
2. 运行 `go test ./agent/ -run 'UniformLine' -v`

## 预期结果
- 既有用例全绿：周期重复（`TestLoopDetector_*`）、单行重复/数量门限（`TestFix329_*`）、跨块（`TestLoopDetector_CrossChunk`）等
- 新增等长用例全绿
- `go vet ./...` 无告警

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败

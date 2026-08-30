# FEATURE-456-UC-0009 防死循环（最大打回次数，超限交人工）

## 前置条件
- 已编译最新可执行码（`go build -o work/co-shell .`）
- 监督 LLM 已启用

## 操作步骤
1. 查看配置中最大打回次数（`supervisor.maxRetries`），确认默认值为 20
2. 构造一个主 LLM 始终无法满足监督 LLM 要求的场景（如监督 LLM 标准过高）
3. 让主 LLM 反复调用 `attempt_completion`，监督 LLM 反复打回
4. 观察打回次数是否被记录
5. 当打回次数达到 `supervisor.maxRetries`（20 次）时，观察行为
6. 将 `supervisor.maxRetries` 改为较小值（如 3），重复上述过程

## 预期结果
- `supervisor.maxRetries` 默认值为 20，可通过参数设定
- 打回次数被记录
- 达到最大打回次数后，**强制放行交人工判定**，不再自动打回
- 交人工时**报告已打回次数**（如"监督 LLM 已打回 20 次，最后一次理由：XXX，请人工判断"）

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败

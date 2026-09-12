# FEATURE-514-UC-0004 等长阈值可通过配置调整

## 前置条件
- 已编译最新可执行码（`go build -o work/co-shell .`）

## 操作步骤
1. 用 `.set loop-uniform-line-threshold 5` 将阈值改为 5，查看回显
2. 构造连续 5 行等长（内容各异）输出，送入 `LoopDetector.AddChunk`
3. Web UI 系统设置页（安全与确认组）查看 `loop-uniform-line-threshold` 配置项
4. 重启后确认配置已持久化到 config.json

## 预期结果
- `.set` 回显阈值已更新为 5
- 连续 5 行等长即触发 `uniform_line_length`（单元测试 `TestUniformLine_CustomThreshold`）
- Web UI 可见该配置项并可修改
- config.json 中 `llm.loop_uniform_line_threshold` 为 5

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败

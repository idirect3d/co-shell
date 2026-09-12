# FEATURE-514-UC-0007 等长检测触发后进入二次判定（judge 开启）

## 前置条件
- 已编译最新可执行码（`go build -o work/co-shell .`）
- `loop_judge_enabled = true`（二次判定开启）
- `show_sup_prompt` / `show_sup_stream` 打开（观察 SUP 块）

## 操作步骤
1. 启动 serve 实例，让 LLM 产生连续 100+ 行等长但内容各异的输出
2. 观察终端/Web UI 横幅与 SUP 块
3. 由问题模型（judge）返回结论后观察后续处理

## 预期结果
- 显示"检测到疑似循环内容（类型: 多行等长）"横幅
- 出现 `SUP·循环判定` 块，含发送给 judge 的 prompt 与流式回复
- judge 判定为循环 → 按 loop 干预策略处理；判定非循环 → 检测器重置并继续

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败

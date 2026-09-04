# FEATURE-467-UC-0004 保存后模型级 ThinkingEnabled/ReasoningEffort 正确

## 前置条件
- 启动 Web UI（`--serve`）
- 打开模型管理界面，点击「＋ 新增模型」进入向导

## 操作步骤
1. 进入向导第一步「选择模板」，选择 deepseek-official 模板
2. 打开「思考（thinking）」开关，设置 reasoning_effort 为 low
3. 完成向导后续步骤（endpoint、api_key、model_name、capabilities、model_id、priority、max_model_len、enabled），提交保存
4. 查看 config.json 中新增模型的配置

## 预期结果
- config.json 中新增模型的 `thinking_enabled` 字段为 true
- config.json 中新增模型的 `reasoning_effort` 字段为 "low"
- 模型列表中该模型显示 💭（thinking）能力标记
- 若 thinking 开关关闭，则 `thinking_enabled` 为 false，`reasoning_effort` 不设置

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败

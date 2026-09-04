# FEATURE-467-UC-0002 开启 thinking 后按 provider 显示 reasoning_effort 选项

## 前置条件
- 启动 Web UI（`--serve`）
- 打开模型管理界面，点击「＋ 新增模型」进入向导

## 操作步骤
1. 进入向导第一步「选择模板」
2. 选择 deepseek-official 模板
3. 打开「思考（thinking）」开关
4. 观察是否出现「推理深度（reasoning_effort）」下拉框及其选项
5. 关闭 thinking 开关，观察 reasoning_effort 下拉框是否隐藏
6. 切换 qwen-official 模板，再次打开 thinking 开关，观察 reasoning_effort 选项

## 预期结果
- thinking 开关开启后，显示 reasoning_effort 下拉框
- deepseek-official 模板的 reasoning_effort 选项为 low/medium/high（默认 high）
- thinking 开关关闭后，reasoning_effort 下拉框隐藏
- 不同 provider 模板的 reasoning_effort 默认值不同（deepseek 默认 high，openai 默认 medium 等）

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败

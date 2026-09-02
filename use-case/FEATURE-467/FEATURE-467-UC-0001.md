# FEATURE-467-UC-0001 选择模板时显示 thinking 开关

## 前置条件
- 启动 Web UI（`--serve`）
- 打开模型管理界面，点击「＋ 新增模型」进入向导

## 操作步骤
1. 进入向导第一步「选择模板」
2. 观察模板下拉框下方是否显示「思考（thinking）」开关
3. 切换不同模板（如 deepseek-official、qwen-official、openai-official），观察 thinking 开关的默认状态

## 预期结果
- 第一步除模板下拉框外，显示一个「思考（thinking）」开关
- 切换模板时，thinking 开关默认值随模板能力变化（模板 capabilities.thinking=true 时默认开启，否则默认关闭）
- 模板下拉框切换后，thinking 开关状态实时刷新

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败

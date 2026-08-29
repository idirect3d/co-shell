# FEATURE-452-UC-0009 ask_followup_question 增加两个固定选项（- 我要再想想先退出，+ 还有其他选项或组合吗）

## 前置条件
- 已编译最新可执行码（`go build -o work/co-shell .`）
- 使用 OpenAI 工具调用模式（`--tool-call-mode openai`）

## 操作步骤
1. 让 LLM 调用 `ask_followup_question` 工具，携带 `question` 和 `options`（如 ["选项A", "选项B"]）
2. 观察弹出的选择框
3. 观察选择框末尾是否显示两个固定选项

## 预期结果
- 选择框显示 LLM 的 options（[1] 选项A、[2] 选项B）
- 选择框末尾追加两个固定选项：
  - `-` → "我要再想想，先退出"
  - `+` → "还有其他选项或者组合吗？"
- 原有的"补充信息"和"取消"固定选项仍保留

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败

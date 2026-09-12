# FEATURE-514-UC-0016 SUP 三场景流式块与 Plan 模式提示词回归

## 前置条件
- 已编译最新可执行码（`go build -o work/co-shell .`）
- `.set show-sup-prompt on`、`.set show-sup-stream on`

## 操作步骤
1. 触发循环 → 观察「SUP·循环判定」块（prompt 段 + 流式 content 段 + tool 段）
2. 触发工具格式错误 → 观察「SUP·问题解决」块
3. 触发 attempt_completion → 观察「SUP·监督」块的审查结论
4. 切换到 Plan 模式（`:mode switch plan`），查看系统提示词中的 PLAN MODE 段

## 预期结果
- 三个场景各自产生独立 SUP 块，标题分别为 SUP·循环判定 / SUP·问题解决 / SUP·监督
- 流式内容逐块追加显示，工具调用输入显示在底部段
- Plan 模式提示词仍包含「挖掘用户的真实需求」等条款（无回归）

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败

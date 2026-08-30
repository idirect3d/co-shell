# FEATURE-456-UC-0006 完成审查工具（放行/打回）

## 前置条件
- 已编译最新可执行码（`go build -o work/co-shell .`）
- 监督 LLM 已启用

## 操作步骤
1. 触发一次监督审查（如主 LLM 调用 `attempt_completion`）
2. 观察监督 LLM 是否被提供一个"完成审查"工具（如 `submit_review`）
3. 确认该工具的必填字段：是否放行（approved）、理由（reason）、建议（suggestion）
4. 让监督 LLM 判定**放行**（approved=true），观察后续行为
5. 再次触发监督审查，让监督 LLM 判定**打回**（approved=false），观察后续行为

## 预期结果
- 监督 LLM 有"完成审查"工具，必填字段：approved（是否放行）、reason（理由）、suggestion（建议）
- 三个字段均为必填，缺失时报错
- 放行：主 LLM 交付通过，进入用户确认流程
- 打回：主 LLM 交付被拒绝，进入重跑流程

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败

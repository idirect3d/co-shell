# FEATURE-514-UC-0009 context-remove-limit=1 时第二次移除被拒并终止（LLM 出错路径）

## 前置条件
- 已编译最新可执行码（`go build -o work/co-shell .`）
- `.set context-remove-limit 1`

## 操作步骤
1. 构造 LLM 调用连续失败场景（如 endpoint 指向不可用地址触发请求错误），使 `run_stream.go` 走到 `removeLastAssistantWithToolCalls` 分支
2. 第一次失败：观察移除与重试行为
3. 第二次失败：观察是否被上限拦截

## 预期结果
- 第一次失败：正常移除有问题的上下文并按 `parse_error_action` 重试，终端提示"已移除有问题的上下文…"
- 第二次失败：**不再移除**，输出达到上限的提示（i18n 文案，含上限值），RunStream 返回错误并结束本轮
- 单元测试 `TestContextRemoveLimit_Exceeded` 覆盖

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败

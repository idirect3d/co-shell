# FEATURE-514-UC-0010 context-remove-limit=0（默认）不限制移除次数

## 前置条件
- 默认配置（未设置 `context_remove_limit`）

## 操作步骤
1. 查看配置默认值：`.set context-remove-limit`
2. 构造多次 LLM 调用失败场景（连续 3 次以上），观察是否始终按既有逻辑移除并重试
3. 运行 `go test ./agent/ -run 'ContextRemoveLimit' -v`

## 预期结果
- 默认值为 0
- 0 时不做任何计数拦截，行为与升级前完全一致（向后兼容）
- 单元测试 `TestContextRemoveLimit_ZeroUnlimited` 通过

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败

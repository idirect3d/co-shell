# FEATURE-514-UC-0011 delete_last_msg 路径受同一上限约束

## 前置条件
- 已编译最新可执行码（`go build -o work/co-shell .`）
- `problem_solver_enabled = true`，`.set context-remove-limit 1`

## 操作步骤
1. 构造工具调用格式错误，使问题解决模型返回 `suggested_action = delete_last_msg`
2. 第一次：观察移除最后一条 assistant(tool_calls) 消息并重试
3. 再次触发同类错误：观察是否被上限拦截而不再移除

## 预期结果
- 第一次：`removed problematic assistant+tool messages` 日志 + 重试
- 第二次：达到上限，不再移除；向用户输出达到上限提示并终止本轮
- 单元测试 `TestContextRemoveLimit_ProblemSolverPath` 覆盖

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败

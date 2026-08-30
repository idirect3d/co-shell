# FEATURE-456-UC-0004 监督 LLM 工具白名单限制

## 前置条件
- 已编译最新可执行码（`go build -o work/co-shell .`）
- 监督 LLM 已启用

## 操作步骤
1. 查看配置中监督 LLM 的工具白名单（`supervisor.allowedTools`，方案 B 显式白名单）
2. 确认白名单默认包含：`read_file`、`execute_command`、`memory_search` 等低风险工具
3. 触发一次监督审查，让监督 LLM 调用白名单内的工具（如 `read_file` 读取主 LLM 交付的文件）
4. 观察是否放行
5. 让监督 LLM 尝试调用白名单外的工具（如 `write_to_file`、`replace_in_file`、`execute_command` 写操作）
6. 观察是否被自动拒绝

## 预期结果
- 白名单内的低风险工具（read_file/execute_command/memory_search）可正常调用
- 白名单外的高风险工具（write_to_file/replace_in_file 等）被自动拒绝
- 拒绝时给出明确提示，监督 LLM 无法执行写操作

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败

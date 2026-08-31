# FEATURE-460-UC-0006 LLM 工具设置 show-sup-prompt / show-sup-stream

## 前置条件
- 已编译最新可执行码（`go build -o work/co-shell .`）
- 配置文件中存在 `supervisor.show_sup_prompt` 与 `supervisor.show_sup_stream` 字段

## 操作步骤
1. 让 LLM 调用 `get_setting` 工具读取 `show-sup-prompt` 与 `show-sup-stream` 的当前值
2. 让 LLM 调用 `set_setting` 工具将 `show-sup-prompt` 设为 `true`
3. 让 LLM 调用 `set_setting` 工具将 `show-sup-stream` 设为 `true`
4. 再次调用 `get_setting` 读取确认
5. 让 LLM 调用 `set_setting` 工具将 `show-sup-stream` 设为非法布尔值（如 `abc`）

## 预期结果
- `get_setting` 能读取两个开关的当前值（默认 off）
- `set_setting` 能将两个开关设为 true 并持久化
- 再次 `get_setting` 返回 on
- 非法布尔值被拒绝并返回错误，不改变配置

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败

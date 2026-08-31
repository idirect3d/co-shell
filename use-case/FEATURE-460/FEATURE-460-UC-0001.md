# FEATURE-460-UC-0001 show-sup-prompt / show-sup-stream 开关默认值与设置

## 前置条件
- 已编译最新可执行码（`go build -o work/co-shell .`）
- 配置文件中存在 `supervisor.show_sup_prompt` 与 `supervisor.show_sup_stream` 字段

## 操作步骤
1. 查看配置中 `show-sup-prompt` 与 `show-sup-stream` 的默认值
2. 用 REPL 命令 `.set show-sup-prompt on` 开启 prompt 显示开关
3. 用 REPL 命令 `.set show-sup-stream on` 开启流式回复显示开关
4. 用 REPL 命令 `.set show-sup-prompt`（不带值）查看当前值
5. 用 REPL 命令 `.set show-sup-stream off` 关闭流式回复显示开关
6. 用 Web UI 设置页（安全与确认组）查看/切换这两个开关

## 预期结果
- 两个开关默认值均为 off（false）
- `.set show-sup-prompt on` 后 `show-sup-prompt` 值为 on
- `.set show-sup-stream on` 后 `show-sup-stream` 值为 on
- `.set show-sup-prompt`（不带值）显示当前值 on
- `.set show-sup-stream off` 后 `show-sup-stream` 值为 off
- Web UI 设置页安全与确认组显示这两个开关，可切换并持久化

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败

# FEATURE-286-UC-0001: no-tool-action 参数配置与行为验证

## 测试目标
验证 `no-tool-action` 配置参数（exit/retry/prompt）在 LLM 返回 0 个工具调用时的行为是否符合预期。

## 前置条件
- 已构建最新版 co-shell（BUILD >= 315）
- 配置文件可正常读写
- 至少一个可用 LLM API 配置

---

## UC-0001-01: 默认值验证

### 验证点
默认值是否为 `retry`

### 验证步骤
1. 启动 co-shell（不使用已有 config.json 或删除 config.json 后启动）
2. 在 REPL 提示符下输入 `:set no-tool-action`
3. 观察输出是否显示当前值为 `retry`

### 预期结果
```
0-tool-call 处理方式: retry (可选: exit, retry, prompt)
```

### 验证命令
```
go build -o /dev/null . && echo "compile OK"
```

---

## UC-0001-02: 参数值设置与持久化

### 验证点
`no-tool-action` 的三个值（exit/retry/prompt）都能成功设置，且保存到 config.json 后重启可恢复

### 验证步骤
1. 启动 co-shell
2. 执行 `:set no-tool-action exit`
3. 执行 `:set no-tool-action` 查看当前值是否为 `exit`
4. 退出并重新启动 co-shell
5. 执行 `:set no-tool-action` 确认值仍为 `exit`
6. 依次测试 `retry` 和 `prompt`，步骤同上

### 预期结果
每次设置后即时生效，退出重启后持久化保持。

### 验证命令（替代手动操作）
```bash
# 测试设置与读取
go test -run "TestNoToolAction" ./cmd/ -v -count=1
```

---

## UC-0001-03: `exit` 模式行为验证

### 验证点
LLM 返回 0 个工具调用时，co-shell 将 LLM 回复视为最终答案，追加 assistant 消息到历史，退出迭代循环。

### 验证步骤
1. 设置 `:set no-tool-action exit`
2. 向 LLM 发送一条简单的问答请求（如"1+1 等于几？"）
3. 观察 LLM 回复后是否立即结束，不追加 continuePrompt
4. 再次输入"刚才我问的什么？"，观察 LLM 是否能记得之前的回答（确认 assistant 消息已被追加到历史）

### 预期结果
- 第一次问答直接结束，无额外提示
- 第二次问题能正确引用上下文

---

## UC-0001-04: `retry` 模式行为验证

### 验证点
LLM 返回 0 个工具调用时，co-shell 丢弃 LLM 回复，不追加消息也不记录 memory，直接循环顶部重试。

### 验证步骤
1. 设置 `:set no-tool-action retry`
2. 发送一条需要 LLM 调用工具才能完成的指令（如"列出当前目录文件"）
3. 观察 LLM 输出纯文本回复后（无工具调用），是否立即重新发起 LLM 调用而不显示额外 user 提示
4. 确认重试时历史中没有新增任何 assistant 或 user 消息

### 验证方式
此行为需通过代码审查 + 日志验证。检查 `agent/run_stream.go` 中 `len(toolCalls) == 0` 的分支是否走 `case "retry"` 路径并 `continue`。

---

## UC-0001-05: `prompt` 模式行为验证（向后兼容）

### 验证点
LLM 返回 0 个工具调用时，co-shell 记录 memory，追加 continuePrompt 强指令后重试。此行为应与 FEATURE-17 一致。

### 验证步骤
1. 设置 `:set no-tool-action prompt`
2. 发送一条需要 LLM 调用工具的指令
3. 观察 LLM 输出纯文本回复后，是否追加 user 消息（含 continuePrompt 内容）
4. 确认 memory 中有这条 assistant 回复的记录

### 预期结果
行为与当前（FEATURE-17）完全一致。

---

## UC-0001-06: 参数值无效输入处理

### 验证点
传入无效值时给出清晰错误提示

### 验证步骤
1. 执行 `:set no-tool-action invalid`
2. 观察错误提示

### 预期结果
```
无效值: invalid。可选: exit, retry, prompt
```

---

## UC-0001-07: `.config` 向导支持

### 验证点
`no-tool-action` 出现在 `.config` 向导的"智能体设置"分组中

### 验证步骤
1. 执行 `.config`
2. 进入"智能体设置"分组
3. 找到 `no-tool-action` 参数
4. 修改为 `exit` 并保存
5. 退出向导后执行 `:set no-tool-action` 验证

---

## UC-0001-08: 去重检查已被移除

### 验证点
原有的跨迭代内容去重检查（`IsDuplicateContent`）已在代码中被删除，不再影响 0-tool-call 行为。

### 验证步骤
1. 搜索代码库中是否有 `IsDuplicateContent` 调用
2. 搜索 `DuplicateContentThreshold` 字段引用
3. 确认 `run_stream.go` 中不再有 `LoopEventContentDuplicate` 相关逻辑

### 验证命令
```bash
grep -rn "IsDuplicateContent\|LoopEventContentDuplicate" agent/run_stream.go && echo "FOUND" || echo "NOT FOUND (OK)"
```

---

## 测试结果记录

| 用例编号 | 测试日期 | 测试人 | 结果 | 备注 |
|---|---|---|---|---|
| UC-0001-01 | | | ⬜ 待测试 | |
| UC-0001-02 | | | ⬜ 待测试 | |
| UC-0001-03 | | | ⬜ 待测试 | |
| UC-0001-04 | | | ⬜ 待测试 | |
| UC-0001-05 | | | ⬜ 待测试 | |
| UC-0001-06 | | | ⬜ 待测试 | |
| UC-0001-07 | | | ⬜ 待测试 | |
| UC-0001-08 | | | ⬜ 待测试 | |
# FEATURE-290 命令行 --session-id 参数：支持指定 session ID 追踪对话上下文

## 背景

目前 co-shell 的单命令模式（`co-shell "指令"`）始终从 bbolt DB 的 `current` key 恢复上一次的 session，
无法通过外部参数控制使用哪个会话上下文。如果外部程序（如 pipeline、调度器）需要为每次调用管理独立的对话上下文，
目前只能先进入 REPL 用 `:new` 创建新 session 再退出，无法自动化。

## 需求

新增 `--session-id`（`-s`）命令行参数，支持按 ID 精确控制 session。

## 测试用例

### UC-0001: 不指定 --session-id 时行为不变

**前置条件**：无
**输入**：
```bash
co-shell "你好"
```
**预期**：
- `RestoreSession()` 从 DB 的 `current` key 加载上次 session
- `executeSingleCommand()` 执行指令
- `PersistSessionNonSystem()` 写入同一个 session
- 无 `--session-id` 相关日志警告

---

### UC-0002: 指定已存在的 session ID 加载历史

**前置条件**：DB 中存在 session `sess-20260724075024-00841832`（有 17 条消息）
**输入**：
```bash
co-shell --session-id sess-20260724075024-00841832 "继续之前的工作"
```
**预期**：
- `RestoreSession()` 之后，`--session-id` 覆盖逻辑检测到 `sess-20260724075024-00841832` 已存在
- 加载该 session 的 17 条历史消息到 `a.messages`
- `a.currentSessionID` 设为 `sess-20260724075024-00841832`
- DB 中 `current` key 更新为该 session ID
- `RunStream()` 执行指令后，`PersistSessionNonSystem` 写入该 session（消息数 17→N）

---

### UC-0003: 指定不存在的 session ID 创建新 session

**前置条件**：DB 中不存在 `my-custom-session-001`
**输入**：
```bash
co-shell --session-id my-custom-session-001 "开始新对话"
```
**预期**：
- `RestoreSession()` 之后，`--session-id` 覆盖逻辑检测到该 ID 不存在
- `ag.Reset()` 清空上下文（仅保留 system prompt）
- `a.currentSessionID` 设为 `my-custom-session-001`
- DB 中 `current` key 更新为该 session ID
- `RunStream()` 执行后，`PersistSessionNonSystem` 创建该 session，消息数从 1（system）增长

---

### UC-0004: session ID 持久化到 current key

**前置条件**：执行过 UC-0002 或 UC-0003
**输入**：
```bash
co-shell "下次还会用我的 session 吗"
```
**预期**：
- 未指定 `--session-id`
- `RestoreSession()` 从 `current` key 加载上一次的 session ID
- 应加载 UC-0002/UC-0003 中保存的 session，而非原始的 session
- 验证 `current` key 已被 `--session-id` 覆盖

---

### UC-0005: 在 REPL 中使用 :session switch 后，再通过命令行进入

**前置条件**：在 REPL 中用 `:session switch <id>` 切换到 session B
**输入**：
```bash
co-shell "查一下现状"
```
**预期**：
- 未指定 `--session-id`
- `RestoreSession()` 从 `current` key 加载 session B
- 与 `:session switch` 行为一致，无需额外改动

---

## 验证方法

1. **单元测试**：`main_test.go` 中模拟 store，验证 `--session-id` 覆盖逻辑的三种分支（不指定/已存在/不存在）
2. **集成验证**：编译后手动执行，观察 `log/` 中 `RestoreSession` 和 `PersistSessionNonSystem` 日志确认 session ID 正确
3. **回归验证**：不指定 `--session-id` 时，现有行为完全不变
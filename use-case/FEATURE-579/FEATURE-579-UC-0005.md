# FEATURE-579 UC-0005：C-1 `board_agents` 精简与 `<unchanged/>`

> 分支：**FEATURE-579**　版本：**v0.88.0**　对应方案：**C-1（env 内部瘦身）**
> 依据：`research/token优化设计方案.md` §6 表 1
> 判定口径：默认 `env_board_roster_brief=true` 时，env 中 `board_agents` 仅输出 `id | name | online/offline`；roster `rev` 未变时输出 `<unchanged/>`；关闭开关回归 14 agent 全职责描述。

## 目的

把 `<board_agents>` 块从 ~2.2KB（14 个 agent 全职责）压到 ~0.7KB，同时保留"选择哪个 agent"所需的最小信息。

## 设计基线

| 项 | 取值 | 说明 |
|---|---|---|
| 现状 | 14 agent × 全 system prompt 描述 ≈2.2KB | 生产 env 实测 |
| 精简后 | 每行 `id | name | online/offline` ≈0.7KB | 职责按需用工具/系统提示词查 |
| 未变优化 | roster `rev` 未变 → `<unchanged/>` | 会话内 rev 见 env 属性 |
| 开关 | `env_board_roster_brief`（默认 true） | §8 |

## 前置条件

- 当前分支 `FEATURE-579`，已实现 C-1；测试注入固定 roster（14 个 agent、`rev=450`）以便确定性比对。
- 测试：`TestBoardRosterBrief`、`TestBoardRosterUnchanged`、`TestBoardRosterBriefDisabled`。

## 用例 A：精简格式与体积

```bash
cd /Users/direct3d/github/co-shell
go test ./agent -run TestBoardRosterBrief -v -count=1 2>&1 | grep -E 'ROSTER\|'
```

**期望**：`ROSTER|brief=true|agents=14|bytes=…`；每行形如 `admin | admin | online`；**不含**任何职责描述文本（断言：不出现 `你是一个严谨` 等 system prompt 片段）；`bytes ≤ 1,000`（基线 2.2KB）。

## 用例 B：rev 未变输出 `<unchanged/>`

```bash
go test ./agent -run TestBoardRosterUnchanged -v -count=1 2>&1 | grep -E 'ROSTER_UNCHANGED'
```

**期望**：连续两次生成 env（同一会话、`rev` 未变）时，第二次 board 块为 `<unchanged/>`（或等价的简短占位），字节数 ≤100 B；`rev` 变化后恢复为精简列表。

## 用例 C：行为不回归（仍能正确选择 agent）

```bash
go test ./agent -run 'TestBoardRosterBrief|TestBoardReplyAgentId' -v -count=1
```

**期望**：从精简块解析出的 agent id/name/status 与全量块一致；`board_send`/`board_reply` 相关用例（既有）仍通过——精简不改变 agent 选择语义。

## 用例 D：关闭开关回归全量职责

```bash
go test ./agent -run TestBoardRosterBriefDisabled -v -count=1 2>&1 | grep -E 'ROSTER\|brief=false'
```

**期望**：`env_board_roster_brief=false` 时输出与 v0.87.0 完全一致（含各 agent 全职责描述），字节数与基线（~2.2KB）一致。

## 判定

- A~D 全通过 → 本 UC 通过；C 失败按行为回归处理（必须修复）。

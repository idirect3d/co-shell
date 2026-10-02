# FEATURE-579 UC-0001：A 历史 env 三级降级——默认分档与体积

> 分支：**FEATURE-579**　版本：**v0.88.0**　对应方案：**A（历史 env 三级降级）**
> 依据：`research/token优化设计方案.md` §4.1/§4.2/§8
> 判定口径：默认 `env_history_prune=true`、`env_history_full_keep=1`、`env_history_lite_keep=1` 时，含 env 的历史消息**从旧到新**依次降级为 **L2 / L1 / L0**（`dist=n-1-i`，0=最新），且各档字段集合与体积达标。

## 目的

锁定方案 A 的默认分档行为：历史 env 逐条降级，尾部 1 条保持完整（L0），其前 1 条精简（L1），更早的极简（L2），并量化各档体积。

## 设计基线

| 项 | 取值 | 说明 |
|---|---|---|
| L0 完整 | 最新 `env_history_full_keep` 条 | 现状全量，基线 5,116 B（当前仓库 cwd 实测） |
| L1 精简 | 其后 `env_history_lite_keep` 条 | 保留 time/message_no/context_window/cwd/current_mode/no_tool_action/session_title/task_plan/opened_resources(含 ui_windows)/sub_agents/board_replies |
| L2 极简 | 其余全部 | 仅 time + message_no |
| L1/L2 不含 | current_dir / tools / research / board_agents 全量 / runtime_info / user_dynamic_events | 静态或一次性信息 |
| 目标体积 | L0 ≤3,000 B、L1 ≤1,000 B、L2 ≤300 B | §9 目标值（L1 基线 ~400 B、L2 基线 111 B） |

## 前置条件

- 当前分支 `FEATURE-579`，`go build ./...` 与 `go vet ./...` 通过。
- `agent/env_prune.go` 已实现 `pruneEnvContext` / `buildEnvBlock` / `envLevelOf`，调用点位于 `agent/loop.go` 的 `buildContextMessages()` 末尾（紧随 `pruneUIContext`）。
- 用例实现方需新增确定性测试：`TestEnvPruneLevels`（不发起 LLM 请求，构造 3 条含 env 的历史消息 + 1 条最新消息）。

## 用例 A：默认 1/1 的档位分布

```bash
cd /Users/direct3d/github/co-shell
go test ./agent -run TestEnvPruneLevels -v -count=1
```

**期望**：输出含 `L2|dist=3`、`L1|dist=1`、`L0|dist=0` 三类带字节数的数据行；且对 N=4 条历史消息，档位为 `L2,L2,L1,L0`（从旧到新）。

## 用例 B：字段集合正确

```bash
go test ./agent -run TestEnvPruneLevels -v -count=1 2>&1 | grep -E 'FIELDS\|(L0|L1|L2)'
```

**期望**：
- `L1` 含 `time/message_no/context_window/cwd/current_mode/no_tool_action/session_title/task_plan/opened_resources/sub_agents/board_replies`；
- `L1`/`L2` 均**不含** `current_dir`、`tools`、`research`、`board_agents`、`runtime_info`、`user_dynamic_events`；
- `L2` 仅含 `time`、`message_no`。

## 用例 C：体积达标

```bash
go test ./agent -run TestEnvPruneLevels -v -count=1 2>&1 | grep -E 'SIZE\|'
```

**期望**：`SIZE|L0 <= 3000`（默认全开时）、`SIZE|L1 <= 1000`、`SIZE|L2 <= 300`；L1/L2 相对 L0 的降幅 ≥70% / ≥95%。

## 判定

- A/B/C 全通过 → 本 UC 通过；任一不达标记录实测字节数并回到实现端修正（不得只调阈值不改实现）。

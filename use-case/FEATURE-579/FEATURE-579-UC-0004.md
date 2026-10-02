# FEATURE-579 UC-0004：B 同轮多工具只注入一份完整 env

> 分支：**FEATURE-579**　版本：**v0.88.0**　对应方案：**B（同轮多工具单份 env）**
> 依据：`research/token优化设计方案.md` §5
> 判定口径：一轮内 3 个工具调用时，仅**最后一个落库**的 tool result 注入 L0 完整 env，其余注入 L2（time+message_no）；整轮 env 总量 ≈ `L0 + 2×L2`。

## 目的

消除同一轮多个 tool result 各自注入完整 env 的重复开销（现状 N×5,116 B → 目标 5,116 + (N-1)×~111 B）。

## 设计基线

| 项 | 取值 | 说明 |
|---|---|---|
| 现状 | 每个 tool result 各调 `injectTimeAndMessageNoToLast()`（`agent/run_stream.go:1766`） | N 个工具 = N 份完整 env |
| 改法 | 循环前预扫描本轮 `toolCalls`，算出**最后一个真正落库**的消息下标（排除 `shouldDeferVisionToolResult` 延迟的视觉工具，`continue` 分支不落库） | 最后一个注入 L0，其余 L2 |
| XML 模式 | tool result 为 user 消息 | 规则同上 |
| 收益目标 | `N×8.5KB → 8.5KB+(N-1)×0.1KB` | 本仓库口径：L0 5,116 B + (N-1)×~111 B |
| 开关 | `env_tool_batch_single`（默认 true；false = 现状） | §8 |

## 前置条件

- 当前分支 `FEATURE-579`，已实现 A/B 且开关可配。
- 测试：`TestToolBatchSingleEnv`、`TestToolBatchVisionDefer`、`TestToolBatchDisabled`。

## 用例 A：一轮 3 个工具 → 1×L0 + 2×L2

```bash
cd /Users/direct3d/github/co-shell
go test ./agent -run TestToolBatchSingleEnv -v -count=1
```

**期望**：输出 `BATCH|tools=3|L0=1|L2=2`；且 `BYTES|total` ≈ `L0 + 2×L2`（允许 ≤5% 偏差，差异来自 message_no/time 长度），显著小于 `3×L0`。

## 用例 B：XML 模式同规则

```bash
go test ./agent -run TestToolBatchSingleEnv -v -count=1 2>&1 | grep -E 'XML_MODE'
```

**期望**：`tool_call_mode=xml` 时同样是 1×L0 + (N-1)×L2（tool result 为 user 消息，按 ContentPart 判定）。

## 用例 C：视觉延迟边界仍有 L0

```bash
go test ./agent -run TestToolBatchVisionDefer -v -count=1
```

**期望**：当本轮最后一个 tool call 被 `shouldDeferVisionToolResult` 延迟（`continue` 不落库）时，实际落库的最后一条消息仍拿到 L0（`LAST_LANDED|L0`）；`abortVisionRecognitionRound` / backfill 路径不会出现"整轮无 L0"。

## 用例 D：关闭开关回归

```bash
go test ./agent -run TestToolBatchDisabled -v -count=1 2>&1 | grep -E 'DISABLED'
```

**期望**：`env_tool_batch_single=false` 时 3 个工具各注入一份 L0（`BATCH|tools=3|L0=3|L2=0`），与 v0.87.0 现状一致。

## 判定

- A~D 全通过 → 本 UC 通过；C 失败为**阻断性缺陷**（可能导致模型丢失环境基线）。

# FEATURE-579 UC-0002：A 边界语义与不回归

> 分支：**FEATURE-579**　版本：**v0.88.0**　对应方案：**A（历史 env 三级降级）**
> 依据：`research/token优化设计方案.md` §4.1/§4.2 兜底规则
> 判定口径：`0=不限` 的语义正确；最后一条永不降级；`task_plan` 可见性兜底生效；**持久历史 `a.messages` 逐字节不变**；Web UI 回放、`:context full`、loop retry 无回归。

## 目的

锁定方案 A 的边界语义与副作用边界——降级只作用于"发送副本"，绝不能污染持久历史或破坏既有回放/重试路径。

## 设计基线

| 项 | 语义 | 说明 |
|---|---|---|
| `full_keep=0` | 全部 L0 | 等于关闭降级（仍受 `env_history_prune` 总开关约束） |
| `lite_keep=0` | 除 L0 外全为 L1 | 无 L2 层 |
| `dist==0` | 永不降级 | 最后一条恒为 L0 |
| task_plan 兜底 | 最新条因 `skipTaskPlan` 不含 `task_plan` 时，向前找最近一条含 `task_plan` 的消息，将其**至少保持 L1** | `track_progress`/`view_task_plan`/`attempt_completion` 结果为最新条时触发 |
| 深拷贝 | 重建前克隆该消息的 `ContentParts` slice（参照 `agent/ui_prune.go:29-31`） | 防共享底层数组被改写 |
| retry 读取源 | `a.messages`（持久历史） | 不受发送副本降级影响 |

## 前置条件

- 同 UC-0001；另需测试：`TestEnvPruneEdgeCases`、`TestEnvPruneNoMutatePersistent`、`TestEnvPruneTaskPlanFallback`、`TestLoopRetryWithPrunedEnv`、`TestContextFullReplayUnchanged`。

## 用例 A：`full_keep=0` → 全部 L0

```bash
go test ./agent -run TestEnvPruneEdgeCases -v -count=1 2>&1 | grep -E 'FULL_KEEP_0'
```

**期望**：所有含 env 的消息均判定为 L0 且字节数与全量一致（等同关闭降级）；总字节 = N × L0。

## 用例 B：`lite_keep=0` → 无 L2

```bash
go test ./agent -run TestEnvPruneEdgeCases -v -count=1 2>&1 | grep -E 'LITE_KEEP_0|LEVELS'
```

**期望**：N=4 时档位为 `L1,L1,L1,L0`（最旧 3 条均为 L1），**不出现 L2**；且无降级异常。

## 用例 C：最后一条永不降级 + `task_plan` 可见性兜底

```bash
go test ./agent -run 'TestEnvPruneTaskPlanFallback|TestEnvPruneEdgeCases' -v -count=1 2>&1 | grep -E 'LAST_ALWAYS_L0|TASKPLAN_VISIBLE'
```

**期望**：
- 末条判定恒为 L0（即使 `full_keep=0` 之外的极端配置，如 `full_keep=1,lite_keep=1` 且末条为 tool result）；
- 当末条不含 `task_plan`（含 skipTaskPlan 语义）时，向前最近一条含 `task_plan` 的消息档位 ≥L1 且其正文仍含 `task_plan` 块。

## 用例 D：持久历史不被修改 + 深拷贝

```bash
go test ./agent -run TestEnvPruneNoMutatePersistent -v -count=1
```

**期望**：裁剪前后对 `a.messages` 做逐条 md5 比对，**完全一致**；且更改裁剪副本的 `ContentParts` 元素不影响 `a.messages` 对应元素（深拷贝验证）。

## 用例 E：回放 / `:context full` / loop retry 不回归

```bash
go test ./agent -run 'TestContextFullReplayUnchanged|TestLoopRetryWithPrunedEnv' -v -count=1
go test ./... -short 2>&1 | tail -5
```

**期望**：Web UI 回放与 `:context full` 输出为**完整原始 env**（未降级）；loop retry 场景（`retried_count` / `loop_feedback`）仍能正确识别并重试；`go test ./... -short` 无新增失败。

## 判定

- A~E 全通过 → 本 UC 通过；D 或 E 失败视为**阻断性缺陷**（污染持久历史/破坏回放），必须修复后重跑。

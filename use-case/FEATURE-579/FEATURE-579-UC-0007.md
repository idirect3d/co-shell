# FEATURE-579 UC-0007：C-3 `runtime_info` 仅首条/变化时输出

> 分支：**FEATURE-579**　版本：**v0.88.0**　对应方案：**C-3（env 内部瘦身）**
> 依据：`research/token优化设计方案.md` §6 表 3
> 判定口径：默认 `env_runtime_info_once=true` 时，`runtime_info`（pid/version/build/port/bind 等，~0.5KB）仅在**会话首条**或字段值变化时输出；否则省略或输出极简占位。

## 目的

把每份 env 都携带的运行时信息（进程级、几乎不变）从重复注入改为一次性注入，单份 env 省 ~0.5KB。

## 设计基线

| 项 | 现状 | 改法 | 开关 |
|---|---|---|---|
| runtime_info | 每份 env 均含 pid/version/build/port/bind 等 ~0.5KB | 仅会话首条或字段值变化时输出 | `env_runtime_info_once`（默认 true，false = 现状） |

## 前置条件

- 当前分支 `FEATURE-579`，已实现 C-3；runtime 信息源为 Agent 持有的进程/服务信息（可在测试中注入固定值）。
- 测试：`TestRuntimeInfoOnce`、`TestRuntimeInfoOnChange`、`TestRuntimeInfoOnceDisabled`。

## 用例 A：首条输出、后续省略

```bash
cd /Users/direct3d/github/co-shell
go test ./agent -run TestRuntimeInfoOnce -v -count=1 2>&1 | grep -E 'RUNTIME\|'
```

**期望**：`msg#1` 输出完整 `<runtime_info>`（含 pid/version/build）；`msg#2`、`msg#3`（字段值不变）**不含**该块或输出 ≤60 B 的占位；单份 env 字节数相应下降（对比基线省 ~0.5KB）。

## 用例 B：字段变化后恢复输出

```bash
go test ./agent -run TestRuntimeInfoOnChange -v -count=1 2>&1 | grep -E 'RUNTIME\|changed'
```

**期望**：修改任一字幕字段（如 `port`、`version`、`build`）后，下一份 env 立即重新输出完整 `<runtime_info>`，之后再次省略。

## 用例 C：关闭开关回归

```bash
go test ./agent -run TestRuntimeInfoOnceDisabled -v -count=1 2>&1 | grep -E 'RUNTIME\|once=false'
```

**期望**：`env_runtime_info_once=false` 时每份 env 均含完整 `<runtime_info>`，与 v0.87.0 现状一致（字节数与基线一致）。

## 判定

- A/B/C 全通过 → 本 UC 通过；B 失败属阻断性缺陷（运行环境变化必须让 LLM 感知）。

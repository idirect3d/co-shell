# FEATURE-579 UC-0006：C-2 `current_dir`/`tools`/`research` 变更检测

> 分支：**FEATURE-579**　版本：**v0.88.0**　对应方案：**C-2（env 内部瘦身）**
> 依据：`research/token优化设计方案.md` §6 表 2
> 判定口径：默认 `env_dir_change_detect=true` 时，会话首条输出全量（current_dir 2,351 B / tools 1,623 B / research 739 B）；内容 hash 未变时输出 `<unchanged since msg#N/>`；变化后立即恢复全量；每 N 次（默认 20）强制全量一次。

## 目的

把 L0 中占比最大的三块静态列表（合计 ~4.7KB）压缩为一行占位，使 L0 从 ~5.1KB 降到 ≤3.0KB。

## 设计基线

| 项 | 现状 | 改法 | 目标 |
|---|---|---|---|
| current_dir | 每次全量（70+ 条目，2,351 B） | hash 比较，未变 → `<unchanged since msg#N/>` | 占位 ≤60 B |
| tools | 每次全量（37 条，1,623 B） | 同上 | 占位 ≤60 B |
| research | 每次全量（10 条，739 B） | 同上 | 占位 ≤60 B |
| 强制全量 | — | 每 N 次迭代（默认 20）强制输出一次全量；LLM 显式调用 `list_files` 时重置 | 保留可回溯性 |
| 开关 | — | `env_dir_change_detect`（默认 true） | 关闭 = 现状 |

## 前置条件

- 当前分支 `FEATURE-579`，已实现 C-2；变更检测状态存于 Agent（**会话级**），作用于 L0/L1 的生成源 `buildFullEnvironmentDetails`。
- 测试：`TestEnvDirChangeDetect`、`TestEnvDirChangeForcedFull`、`TestEnvDirChangeDisabled`。

## 用例 A：首条全量 → 未变占位 → 变化恢复全量

```bash
cd /Users/direct3d/github/co-shell
go test ./agent -run TestEnvDirChangeDetect -v -count=1 2>&1 | grep -E 'DIRDETECT'
```

**期望**：
- `msg#1`：三块均为全量（`current_dir=full`、`tools=full`、`research=full`）；
- 同 hash 再次生成（`msg#2`）：三块均为 `<unchanged since msg#1/>` 形式，单块 ≤60 B；
- 新增/删除一个文件（或改动 research 目录）后生成（`msg#3`）：对应块恢复全量，其余仍为占位。

## 用例 B：每 N 次强制全量

```bash
go test ./agent -run TestEnvDirChangeForcedFull -v -count=1 2>&1 | grep -E 'FORCED_FULL'
```

**期望**：连续生成 20 次（内容不变）时，第 20 次输出全量（`FORCED_FULL|at=20`）；`list_files` 之后重置计数（下一次输出全量）。

## 用例 C：L0 体积达标

```bash
go test ./agent -run TestEnvDirChangeDetect -v -count=1 2>&1 | grep -E 'SIZE\|L0'
```

**期望**：处于占位态时 L0 ≤3,000 B（基线 5,116 B，降幅 ≥41%）；首次全量态可与基线一致。

## 用例 D：关闭开关回归

```bash
go test ./agent -run TestEnvDirChangeDisabled -v -count=1 2>&1 | grep -E 'DISABLED'
```

**期望**：`env_dir_change_detect=false` 时每份 env 均输出三块全量，字节数与 v0.87.0 基线一致。

## 判定

- A~D 全通过 → 本 UC 通过；A 中"变化后未恢复全量"属阻断性缺陷（会导致 LLM 看到过期目录）。

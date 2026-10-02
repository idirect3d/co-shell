# FEATURE-579 UC-0003：A 关闭即现状（v0.87.0 字节级一致）

> 分支：**FEATURE-579**　版本：**v0.88.0**　对应方案：**A（历史 env 三级降级）开关回归**
> 依据：`research/token优化设计方案.md` §8 兼容性承诺（"所有开关关闭时行为与当前版本完全一致"）
> 判定口径：`env_history_prune=false` 时，历史 env **不降级**，与 v0.87.0（`main`，tag `v0.87.0`）的输出**逐字节一致**。

## 目的

验证方案 A 的向后兼容：关闭总开关即回到现状行为，任何调用方（loop / Web UI / `:context full`）不感知差异。

## 设计基线

| 项 | 取值 | 说明 |
|---|---|---|
| 开关 | `env_history_prune=false` | 关闭后 `pruneEnvContext` 直接返回原 `msgs`（§4.2 首行） |
| 对比基准 | tag `v0.87.0`（commit `34c9f237`） | 现状实现：历史 env 全量、不裁剪 |
| 单项体积 | L0 = 5,116 B（当前仓库 cwd，基线数据 §S2） | 关闭后每条历史 env 均应为该量级 |
| 一致性判据 | 同一构造输入下 md5 相同 | 不引入时间戳/随机值差异 |

## 前置条件

- 当前分支 `FEATURE-579`，已实现方案 A 且开关可配。
- 测试：`TestEnvPruneDisabled`；如需与 v0.87.0 对照，可用 `git worktree add ../co-shell-0.87 v0.87.0` 生成对照产物（可选，非必需）。

## 用例 A：关闭后不裁剪

```bash
cd /Users/direct3d/github/co-shell
go test ./agent -run TestEnvPruneDisabled -v -count=1
```

**期望**：构造 4 条历史 env，输出全部为 L0（`LEVELS|L0,L0,L0,L0`），且 `pruneEnvContext` 返回的 slice 与入参**长度、逐条内容字节数完全一致**。

## 用例 B：与冻结基线常量一致

```bash
go test ./agent -run TestEnvPruneDisabled -v -count=1 2>&1 | grep -E 'BYTES\||MD5\|'
```

**期望**：单条 env 字节数与 `research/token-baseline/基线数据-各阶段.md` 记录的基线（当前 cwd 下 5,116 B）一致；同输入连续 2 次运行的 `MD5` 相同。

## 用例 C：不改变既有行为（整体回归）

```bash
go build ./... && go vet ./...
go test ./agent -run TestTokenBaselineStages -v -count=1 2>&1 | grep -E '^S[0-5]\|'
```

**期望**：编译与 vet 通过；`env_history_prune=false` 时基线测量数值与冻结基线**逐项一致**（S2 L0 5,116 B、S3 N=3 15,348 B、S4 斜率 +5,116 B/轮）。

## 判定

- A/B/C 全通过 → 本 UC 通过；出现任何字节差异先排查是否误引入其它开关的默认副作用（如 C 的变更检测未受控）。

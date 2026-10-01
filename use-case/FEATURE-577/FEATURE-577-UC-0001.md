# FEATURE-577 UC-0001：确定性测量（S0–S5）可复现且 3 次一致

> 分支：**FEATURE-577**　版本：**v0.87.0**　对应阶段：S0 系统提示词 / S1 工具声明 / S2 env 信封 / S3 同轮多工具重复 / S4 N 轮累积 / S5 单轮请求合成
> 判定口径：同一命令连续执行 3 次，输出**逐字节一致**（md5 相同），且覆盖 S0–S5 全部指标。

## 目的

锁定「确定性基线测量」的可复现性：任何一次测量输出的数值必须能被独立复现，否则后续优化对比不成立。

## 设计基线

| 项 | 取值 | 说明 |
|---|---|---|
| 测量工具 | `agent/token_baseline_test.go` → `TestTokenBaselineStages` | 直调产品代码，不发起任何 LLM 请求 |
| 工作目录 | 仓库根 `/Users/direct3d/github/co-shell`（测试内 `os.Chdir("..")`） | 使 `<current_dir>/<tools>/<research>` 块与生产一致 |
| 输出格式 | 固定前缀行（`S0|`、`S1|`、`S2|`、`S3|`、`S4|`、`S5|`），无时间戳/随机值 | 便于 md5 校验与机器解析 |
| 单位 | 字节（B）为主，括号内附估算 token（3.7 B/token） | 见方案 §6 局限 |
| 一致性判据 | 3 次运行输出的 md5 完全相同 | 环境变量/时间不得进入输出 |

## 前置条件

- 当前分支 `FEATURE-577`，`go build ./...` 通过。
- 未修改 `config.json`（基线口径冻结）。

## 用例 A：3 次运行一致

```bash
cd /Users/direct3d/github/co-shell
for i in 1 2 3; do
  go test ./agent -run TestTokenBaselineStages -v -count=1 2>&1 | tee research/token-baseline/det-run-$i.log >/dev/null
done
# 只比对数据行（去掉 go test 的 ok/PASS 行）
for i in 1 2 3; do grep -E '^(S[0-5])\|' research/token-baseline/det-run-$i.log | md5; done
```

**期望**：3 行 md5 完全相同。

## 用例 B：覆盖完整（S0–S5 每一阶段都有数据行）

```bash
grep -cE '^S0\|' research/token-baseline/det-run-1.log   # ≥1
grep -cE '^S1\|' research/token-baseline/det-run-1.log   # ≥1
grep -cE '^S2\|' research/token-baseline/det-run-1.log   # ≥1（含分项）
grep -cE '^S3\|' research/token-baseline/det-run-1.log   # ≥1（N=1..5）
grep -cE '^S4\|' research/token-baseline/det-run-1.log   # ≥1（含 12/50 轮累积与斜率）
grep -cE '^S5\|' research/token-baseline/det-run-1.log   # ≥1（第 1/10/50 轮合成量与固定开销占比）
```

**期望**：每项均 ≥1，且 S2 行内可见 `current_dir / tools / research / board_agents / runtime_info / task_plan / opened_resources` 分项。

## 用例 C：不污染生产数据（测量无副作用）

```bash
git status --short                          # 除本任务新增的测量文件外，无其他改动
ls -la log/ | head -3                       # 未新增 llm-interaction 日志（未发起请求）
```

**期望**：测量过程不发起 LLM 请求、不修改 `config.json`、不写入会话库。

## 判定

- A/B/C 全通过 → UC 通过；任一失败则视为测量工具缺陷，须修复后重跑（不得只记录单次结果）。

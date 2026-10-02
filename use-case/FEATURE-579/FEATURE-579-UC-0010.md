# FEATURE-579 UC-0010：端到端前后对比（全开达标 / 全关回归 / S6 成对复测）

> 分支：**FEATURE-579**　版本：**v0.88.0**　对应方案：**A+B+C+D1 整体验收**
> 依据：`research/token优化设计方案.md` §3 验收标准、§10；`research/token-baseline/基线数据-各阶段.md`
> 判定口径：全开（9 项开关 true + 2 项默认值变更）时 S0–S5 确定性测量 3 次 md5 一致，且各项**达到目标值**；全关时与基线**逐项一致**；S6 同任务成对复测给出前后数据。

## 目的

以确定性测量 + 真实会话复测，给出"优化前 vs 优化后"的可核验对比结论，作为本任务的最终验收证据。

## 设计基线（优化前）与目标

| 阶段 | 基线 | 目标（全开） |
|---|---|---|
| S0 系统提示词 | 39,410 B（通道①）/ 21,437 B（通道②） | 不劣化（允许 +5% 内） |
| S1 工具 schema | 38 工具 / 66,657 B；`meta` 38×9,158 B（13.7%）；生产 69 工具 / 98,576 B | schema **≥-30%**；`meta` **≤4,000 B** |
| S2 env | L0 5,116 B（current_dir 2,351 / tools 1,623 / research 739 / opened_resources 118 / other 285）、L1 ~400 B、L2 111 B | L0 **≤3,000 B**、L1 **≤1,000 B** |
| S3 同轮 N 工具 | N=1..5：5,116 → 25,580 B | N=3 → **5,338 B（-65.2%）** |
| S4 N 轮累积 | 斜率 **+5,116 B/轮**（50 轮 255,800 B） | 斜率 **≤~120 B/轮**、50 轮 **≤~11 KB** |
| S5 单轮合成 | 1 轮 111,183 B（固定占 95.4%）/ 50 轮 361,867 B（固定占 29.3%） | 第 50 轮固定开销占比 **≤5%** |
| S6 真实会话 | 58 轮累计 prompt 13,400,749；缓存命中 97.6%；miss 315,437；首轮 39,725 → 末轮 423,983；中位增量 +6,486/轮 | 累计 prompt 显著下降；reorganize 触发轮次后移 |

## 前置条件

- 当前分支 `FEATURE-579`，A/B/C/D1 与配置项、设置界面、i18n 均已实现并自测通过。
- 测量工具：`agent/token_baseline_test.go` → `TestTokenBaselineStages`（直调产品代码，不发 LLM 请求）。
- 复测采用**同任务成对**：优化后跑与基线相同的 10 轮标准化任务（S0–S5）/ 同一真实任务会话（S6）。

## 用例 A：全开时 S0–S5 达标且可复现

```bash
cd /Users/direct3d/github/co-shell
for i in 1 2 3; do
  go test ./agent -run TestTokenBaselineStages -v -count=1 2>&1 \
    | tee research/token-baseline/f579-opt-run-$i.log >/dev/null
done
for i in 1 2 3; do grep -E '^(S[0-5])\|' research/token-baseline/f579-opt-run-$i.log | md5; done
grep -E '^S[0-5]\|' research/token-baseline/f579-opt-run-1.log
```

**期望**：3 行 md5 完全相同；S1 `delta ≤ -30%` 且 `meta ≤4,000 B`；S2 `L0 ≤3,000`、`L1 ≤1,000`；S3 `N=3 ≈5,338 B`（±5%）；S4 斜率 `≤~120 B/轮`、50 轮 `≤~11 KB`；S5 第 50 轮固定占 `≤5%`。

## 用例 B：全关时与基线逐项一致

```bash
# 将 9 项开关全部置 false、read_file_max_size=0、context_reorganize_threshold=80（临时目录配置）
go test ./agent -run TestTokenBaselineStages -v -count=1 2>&1 | grep -E '^S[0-5]\|'
```

**期望**：与 `research/token-baseline/基线数据-各阶段.md` 的基线数值**逐项一致**（S0 39,410/21,437、S1 66,657 与 meta 9,158、S2 L0 5,116、S3 N=3 15,348、S4 5,116 B/轮、S5 111,183 / 361,867）；不得改动仓库 `config.json`。

## 用例 C：S6 同任务成对复测

```bash
python3 research/token-baseline/parse_llm_log.py \
  --log log/llm-interaction-<复测>.log --from <起点> --to <终点> \
  --out research/token-baseline/f579-s6-opt
# 与基线目录 research/token-baseline/s6w-repo-1 对比
```

**期望**：给出同一任务的前后对照表（累计 prompt、缓存命中率、cache-miss、首/末轮 prompt、中位增量、reorganize 首次触发轮次）；累计 prompt 与中位增量**显著低于基线**，reorganize 触发轮次**后移或不再触发**；若 S6 因环境因素不可比，须如实标注并说明原因（**不作为阻断项**）。

## 用例 D：整体回归

```bash
go build ./... && go vet ./...
go test ./... -short 2>&1 | tail -20
```

**期望**：编译与 vet 通过；`go test ./... -short` 无**新增**失败（基线既存失败须如实标注，并确认与本次改动无关）。

## 判定

- A/B/C/D 全通过（C 允许带如实说明的不可比标注）→ 本 UC 通过，可作为 FEATURE-579 的验收证据；任一未达标须回到对应方案修正后整体重跑（不得只重跑单项）。

# FEATURE-577 UC-0002：真实会话端到端采样（S6）与日志解析正确性

> 分支：**FEATURE-577**　版本：**v0.87.0**　对应阶段：S6 真实会话端到端 token 与缓存命中
> 判定口径：固定任务在**独立工作区、独立会话**中真实运行 3 次，每次 ≥10 轮工具迭代；
> 每轮 `prompt/completion/cached` token 可从 `log/llm-interaction-<date>.log` 完整还原，且 3 次的总量与斜率量级一致。

## 目的

锁定「真实端到端」这一路的可测量性与可对比性：无此数据，S5/S6 只能停留在字节估算，无法证明优化在真实请求上生效。

## 设计基线

| 项 | 取值 | 说明 |
|---|---|---|
| 运行方式 | `./work/co-shell-0.87.0.darwin.arm64 run --workdir research/token-baseline/s6-workspace --prompt "$(cat research/token-baseline/fixed-task.txt)"` | 非交互，不打开浏览器 |
| 独立工作区 | `research/token-baseline/s6-workspace/`（12 个文本文件 + 2 子目录，脚本一次性生成） | 使 `<current_dir>/<tools>/<research>` 块稳定可比 |
| 独立会话 | 每次运行使用全新会话（不复用 session id），运行前清理该工作区会话状态 | 避免历史上下文污染 |
| 固定任务 | `research/token-baseline/fixed-task.txt`：**12 步，每步单独一次 `read_file`，最后 `attempt_completion` 汇总** | 强行制造 ≥12 轮工具迭代 |
| 数据源 | `log/llm-interaction-<date>.log`（`[REQ]`/`[RES]` 记录） | usage 字段实测存在：`prompt_tokens` / `completion_tokens` / `cached_tokens` / `prompt_cache_hit_tokens` / `prompt_cache_miss_tokens` |
| 解析工具 | `research/token-baseline/parse_llm_log.py --log … --from … --to … --out run-N` | 按运行时间窗口切分并归属 |
| 重复次数 | 3 次（run-1/2/3），每次记录起止时间戳到 `run-N.meta` | 用于日志归属与口径声明 |

## 前置条件

- 已编译产物 `work/co-shell-0.87.0.darwin.arm64`（或使用 `go run .`）。
- `config.json` 未被改动（口径冻结，见测试方案 §4）。
- `s6-workspace/` 与 `fixed-task.txt` 已生成。

## 用例 A：单次运行产生 ≥10 轮工具迭代

```bash
cd /Users/direct3d/github/co-shell
TS_FROM="$(date '+%Y-%m-%d %H:%M:%S')"
./work/co-shell-0.87.0.darwin.arm64 run \
  --workdir research/token-baseline/s6-workspace \
  --prompt "$(cat research/token-baseline/fixed-task.txt)" 2>&1 | tee research/token-baseline/run-1.stdout
TS_TO="$(date '+%Y-%m-%d %H:%M:%S')"
printf 'from=%s\nto=%s\n' "$TS_FROM" "$TS_TO" > research/token-baseline/run-1.meta
python3 research/token-baseline/parse_llm_log.py \
  --log "log/llm-interaction-$(date '+%Y-%m-%d').log" \
  --from "$TS_FROM" --to "$TS_TO" --out research/token-baseline/run-1
wc -l research/token-baseline/run-1.calls.csv     # 行数-1 = LLM 调用轮数，须 ≥10
```

**期望**：`calls.csv` 轮数 ≥10；`summary.txt` 含累计 prompt/completion、缓存命中率、轮数。

## 用例 B：逐轮数据完整且自洽

```bash
head -3 research/token-baseline/run-1.calls.csv
cat research/token-baseline/run-1.summary.txt
```

**期望**（逐条核对）：

1. 每行包含：轮次、时间、`prompt_tokens`、`completion_tokens`、`cached_tokens`、`prompt_cache_hit_tokens`、`prompt_cache_miss_tokens`、`tools_bytes`、`system_bytes`、`messages_count`、`env_count`；
2. `prompt_tokens ≈ prompt_cache_hit_tokens + prompt_cache_miss_tokens`（允许 ≤1% 误差）；
3. `prompt_tokens` **随轮次单调不减**（同一次运行内），除 `reorganize_context` 触发当轮出现下降；
4. `env_count` 随轮次增长（每一轮新增工具结果消息均带 env，验证 S3/S4 的真实性）。

## 用例 C：3 次运行量级一致

```bash
for i in 1 2 3; do grep -E '^(rounds|total_prompt|total_completion|cache_hit_rate)' \
  research/token-baseline/run-$i.summary.txt; done
```

**期望**：3 次的轮数一致（±1 轮内）；累计 prompt 量级一致（互差 ≤20%，超出需说明模型行为差异）；每轮 prompt 增量斜率一致（互差 ≤20%）。

## 用例 D：不污染用户环境

```bash
git status --short          # 仅本次任务文件
ls research/token-baseline/ # 只新增 run-*.{csv,summary.txt,meta,stdout}
```

**期望**：未修改 `config.json`、未写入既有会话库（`db/`）、未影响正在运行的实例（`--serve` 实例不受影响）。

## 判定

- A/B/C/D 全通过 → UC 通过。
- 单次运行轮数 <10 或数据缺失 → 该次作废重跑，不得用不足轮数的数据充当基线。

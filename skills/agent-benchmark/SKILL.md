---
name: agent-benchmark
description: >-
  对当前 Agent 进行标准化能力评测（跑分）。当用户提出"跑分 / 评测 / benchmark /
  自测 / 能力评分 / 上线前验收"等请求时使用。流程：导出公开任务包 → 执行任务并产出结果
  → 运行离线评分脚本生成报告。适用于 Agent 开发迭代、选型对比、回归测试、上线前验收。
---

# Agent Benchmark 跑分技能

## 目标

引导当前 Agent 完成一次标准化能力评测：**按公开任务包逐条执行任务、产出结果文件，
再由独立评分脚本基于 benchmark 库的参考答案评分，输出可复现的评测报告。**

## 核心原则（必须遵守）

1. **角色分离**：当前 Agent 是"被测者"，只接触 `tasks_public/`（已剥离参考答案与评分细则的
   公开任务包）；`reference` 与 `scoring` 只存在于 benchmark 库原始任务定义中，由评测方脚本读取。
2. **禁止作弊**：不得读取 benchmark 库内 `tasks/*.task.json` 原始文件中的
   `reference`/`scoring` 字段；不得修改库源文件。
3. **产物规范**：所有任务产物写入 `outputs/<task_id>/`，文件名与格式见任务包的 `output_spec`。
4. **可复现**：产物目录完整保留，评分脚本对同一产物可重复评分，结果一致。

## 执行流程（三阶段）

### 阶段 0：定位环境

- benchmark 库根目录：环境变量 `AB_BENCHMARK`（若设置），否则从本 skill 位置向上探测
  （特征：含 `schema/task.schema.json` 与 `tasks/`）。
- 本次运行目录：`<run_id>/`，如 `run_20260830_120000`。

### 阶段 1：导出公开任务包

```bash
python <skill>/scripts/export_tasks.py --benchmark <库根> --out <run_id>
# 可选：只测部分任务
python <skill>/scripts/export_tasks.py --benchmark <库根> --tasks reins-001,gen-001 --out <run_id>
```

产出：`<run_id>/tasks_public/`（`INSTRUCTIONS.md` + 各任务 `task.json` + `assets/` 素材）。

### 阶段 2：执行任务（被测者）

- 读取 `<run_id>/tasks_public/INSTRUCTIONS.md` 与各任务 `task.json`；
- 逐条在 `outputs/<task_id>/` 下完成：
  - JSON 任务 → `outputs/<task_id>/result.json`
  - 文本任务 → `outputs/<task_id>/result.txt`
  - 代码任务 → `outputs/<task_id>/solution.py`
- 详细执行规范见 `templates/RUNBOOK.md`（被测 Agent 应遵循该手册）。

### 阶段 3：评分与报告（评测方）

```bash
python <skill>/scripts/score_outputs.py --benchmark <库根> --outputs <run_id>/outputs --run-id <run_id>
# LLM-as-judge 任务需要 judge 模型（可选）：
python <skill>/scripts/score_outputs.py --benchmark <库根> --outputs <run_id>/outputs \
    --judge-api https://... --judge-model model-name --judge-key xxx
```

产出：`<run_id>/score_<run_id>/report.md` + `report.json`（通过率、平均分、按类别/难度分层、逐任务评分明细）。

## 红线

- ❌ 读取/复制 benchmark 库内 `tasks/` 原始 `.task.json`（含 `reference`/`scoring`）；
- ❌ 修改 `tasks_public/` 任务文件或 `INSTRUCTIONS.md`；
- ❌ 产物写到 `outputs/` 之外；
- ✅ 产物缺失的任务将判 0 分并在报告中标注。

## 常见问题

| 问题 | 处理 |
|------|------|
| 找不到 benchmark 库 | 检查 `AB_BENCHMARK` 或确认 skill 与库的相对位置 |
| 某任务产物缺失 | 报告会标注"未产出"，该任务 0 分；重新执行该任务后重跑阶段 3 |
| judge 未配置 | LLM-as-judge 任务使用内置启发式占位评分（仅自测），正式评测必须配置 judge |
| 结果不一致 | 确认产物未被修改；`score_outputs.py` 是确定性评分（除 judge 外） |

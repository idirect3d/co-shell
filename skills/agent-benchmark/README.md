# agent-benchmark Skill — 接入与使用说明

> 把 AgentBenchmark 跑分流程封装为可分发技能：任何 Agent 配置本技能后，
> 一句"跑分"即可完成标准化评测。

## 一、技能内容

```
agent-benchmark/
├── SKILL.md                    # 技能主文档（标准 Agent Skills 结构，frontmatter 声明触发词）
├── scripts/
│   ├── export_tasks.py         # 导出"公开任务包"（剥离 reference/scoring/test，防作弊）
│   └── score_outputs.py        # 离线评分（评测方运行，基于库内参考答案）
└── templates/
    └── RUNBOOK.md              # 被测 Agent 执行手册（产物格式与红线）
```

**设计要点（为什么这样分）**：

| 环节 | 谁执行 | 依据 | 说明 |
|------|--------|------|------|
| 导出任务包 | 评测方/被测者均可 | export_tasks.py | 公开任务包**不含**参考答案与评分细则，杜绝作弊 |
| 执行任务 | 被测 Agent | RUNBOOK.md | 产物写入 `outputs/<task_id>/` |
| 评分出报告 | 评测方（独立进程） | score_outputs.py | 基于库内原始任务定义的 reference 评分 |

## 二、接入方式

### 方式 A：标准 Agent Skills（推荐，跨框架通用）

将 `agent-benchmark/` 目录复制到目标 Agent 的 skills 目录：

| Agent 框架 | skills 目录 |
|------------|-------------|
| Claude Code | `~/.claude/skills/agent-benchmark/` |
| Cursor | `~/.cursor/skills/agent-benchmark/` 或项目 `.cursor/skills/` |
| crush | `~/.config/crush/skills/agent-benchmark/` 或项目 `.crush/skills/` |
| 支持 SKILL.md 的其他框架 | 同结构放置即可 |

之后对被测 Agent 说："**跑分**"（或"开始 benchmark 评测"）即可触发。

### 方式 B：co-shell 接入（当前环境）

co-shell 通过 `.rules/` 与 `mode/` 提供技能化指令加载。二选一：

1. **作为技能文档**：把 `SKILL.md` 复制为工作区 `.rules/benchmark-skill.md`，
   被测 Agent 需要时可按规则加载路径读取（与"文件写入规范"同等机制）。
   ```bash
   cp research/AgentBenchmark/skill/agent-benchmark/SKILL.md .rules/benchmark-skill.md
   ```
2. **作为模式（mode）**：复制目录为 `mode/benchmark.v1/`（含 OBJECTIVE.md 等），
   作为被测 Agent 的专用评测模式。

### 方式 C：评测方手动调用（无需给被测 Agent 配技能）

仅需 benchmark 库 + 本 skill 的脚本（被测 Agent 不需要装 skill）：

```bash
# 1) 评测方导出任务包
python skill/agent-benchmark/scripts/export_tasks.py --benchmark . --out run_20260830

# 2) 让被测 Agent 在 run_20260830/tasks_public/ 下完成任务（给出 RUNBOOK 即可）
#    —— 被测 Agent 只看到任务包，看不到参考答案

# 3) 评测方评分
python skill/agent-benchmark/scripts/score_outputs.py --benchmark . --outputs run_20260830/outputs --run-id run_20260830
```

## 三、使用示例

```bash
# 导出全部任务
python skill/agent-benchmark/scripts/export_tasks.py --benchmark . --out run_demo
# 只导出再保险任务
python skill/agent-benchmark/scripts/export_tasks.py --benchmark . --tasks reins-001,reins-002 --out run_reins

# 被测 Agent 执行后评分（含 LLM-as-judge 配置示例）
python skill/agent-benchmark/scripts/score_outputs.py --benchmark . --outputs run_demo/outputs \
    --judge-api https://api.example.com/v1 --judge-model judge-model --judge-key $JUDGE_KEY
```

## 四、验收（跑分自检）

1. 导出任务包后，`tasks_public/**/task.json` 中**不得出现** `reference` / `scoring` 字段；
2. 产物缺失的任务在报告中判 0 分并标注；
3. 同一产物重复评分结果一致（judge 任务除外）。

## 五、与 benchmark 库的关系

- 本技能依赖 benchmark 库（`schema/` + `tasks/` + `scorers/` + `runner/`），通过
  `--benchmark` 或环境变量 `AB_BENCHMARK` 指定库根；
- 技能随库版本演进：新增任务无需改技能，重新导出即可；
- 私有任务集（`metadata.sensitive=true`）不导出到公开任务包，防止数据外泄。

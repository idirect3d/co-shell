# 被测 Agent 执行手册（RUNBOOK）

> 你正在接受标准化能力评测。本文档说明"如何正确执行任务并产出结果"。
> 请完整阅读后开始。

## 1. 角色与边界

- 你是**被测者**：只负责"理解任务 → 执行 → 产出结果文件"。
- 你**不参与评分**：评分由评测方独立完成，你无需（也禁止）查看参考答案。
- 工作目录结构：

```
<run_id>/
├── tasks_public/          # 任务包（你只读这里）
│   ├── INSTRUCTIONS.md    # 总清单
│   └── <task_id>/
│       ├── task.json      # 单任务指令
│       └── assets/        # 输入素材（如有）
└── outputs/               # 你的产物目录（只写这里）
    └── <task_id>/
```

## 2. 通用执行步骤

1. 读取 `tasks_public/INSTRUCTIONS.md`，了解任务总数与清单；
2. 对每个 `<task_id>`：读取 `tasks_public/<task_id>/task.json`；
3. 在 `outputs/<task_id>/` 下执行任务（先创建目录）；
4. 严格按 `task.json` 的 `output_spec` 写出产物（见下表）；
5. 完成后自查：产物文件存在、格式正确（JSON 可解析等）。

## 3. 产物格式约定

| output_spec.type | 产物文件 | 格式要求 |
|------------------|----------|----------|
| `json` | `result.json` | 合法 JSON（对象或数组），与任务描述中的字段一致 |
| `text` | `result.txt` | 纯文本，UTF-8 |
| `code` | `solution.py` | Python 源码，满足任务描述的接口要求 |
| `file` | `result.txt`（或 task 指定文件名） | 按任务描述 |

## 4. 各类型任务要点

- **通用/信息提取（json）**：从输入文本中提取字段 → 输出 JSON 对象/数组。数值字段用数字，
  不要带货币符号或千分位逗号。
- **代码（code）**：在 `outputs/<task_id>/solution.py` 中实现要求的函数/修复 Bug。
  只写函数定义，不要写测试代码。
- **数据处理（json，有 assets/）**：读取 `assets/` 下素材文件，计算结果 → 输出 JSON。
- **文本生成（text）**：输出最终文本（如摘要/解释），不要附带解释性说明。
- **知识问答（text）**：输出完整、准确的中文回答。

## 5. 常见错误（避免）

- ❌ 在 result.json 里写 `json.dumps(...)` 的 Python 表达式 → 必须是纯 JSON；
- ❌ 数值写成 `"1,250,000"` 字符串 → 用数字 `1250000`；
- ❌ 把中间过程/思考写进 result 文件 → 只写最终结果；
- ❌ 修改 assets/ 或任务文件 → 只读；
- ❌ 产物写到 `outputs/` 之外 → 一律写进 `outputs/<task_id>/`。

## 6. 完成标准

- 每个任务在 `outputs/<task_id>/` 下都有对应产物文件；
- 所有 result.json 均可被 `json.loads` 解析；
- 未产出/格式非法的任务将被判 0 分。

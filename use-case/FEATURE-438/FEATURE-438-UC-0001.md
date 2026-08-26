# FEATURE-438: 死循环历史污染修正

## 背景

死循环二次判定确认后，当前处理主要是丢弃循环导致未完成的消息（delete_last_msg）或追加纠正信息（prompt_feedback），但发生死循环往往意味着历史消息可能已被导致死循环的文字污染（如一次迭代中多次重复"让我看看""让我执行"，甚至同样或类似的话连说两遍）。污染源不清理，下一轮 LLM 仍可能被带偏，死循环难以根治。

## 目标

二次判定确认死循环后，判定模型在 report_problem 中额外返回 history_fixes 列表（每项含 message_index 消息序号 + search 被替换原文 + replace 替换新内容 + reason 原因），程序执行历史修正：用消息序号定位消息，原文匹配对 search→replace 精确替换，相邻消息（±1）兜底查找，只替换首次出现，仅允许 assistant 消息，与原有处理（prompt_feedback/delete_last_msg 等）叠加执行。范围：最近 3 条 assistant 消息。新增配置开关 loop-history-fix-enabled 默认 on。写日志 + 用户可见提示。先执行历史修正再执行 delete_last_msg。

## 架构

```
二次判定确认死循环（report_problem 返回 type=loop）
  ├─ 解析 history_fixes 列表（HistoryFix: message_index + search + replace + reason）
  ├─ 若 loop-history-fix-enabled=on：
  │    对每条 fix：
  │      用 message_index 定位消息
  │      若该消息非 assistant → 跳过（仅允许 assistant）
  │      若该消息内找不到 search → 相邻消息（±1）兜底查找
  │      找到后只替换首次出现
  │      写日志 + 用户可见提示
  └─ 继续执行原有处理（prompt_feedback / delete_last_msg 等，先修正后 delete）
```

## 用例

### M1 report_problem schema 扩展 + 解析

| 编号 | 场景 | 操作 | 预期结果 |
|------|------|------|---------|
| UC-0001 | schema 含 history_fixes | 检查 reportProblemTool 定义 | `history_fixes` 数组属性存在，元素含 message_index/search/replace/reason 字段 |
| UC-0002 | 解析 history_fixes | 模拟 report_problem 参数含 history_fixes 数组 | parseProblemReport 正确解析出 ProblemReport.HistoryFixes |
| UC-0003 | 无 history_fixes 字段 | 模拟 report_problem 参数不含 history_fixes | HistoryFixes 为空切片，不报错 |
| UC-0004 | history_fixes 为空数组 | 模拟 history_fixes: [] | HistoryFixes 为空切片，不报错 |
| UC-0005 | 单条 fix 解析 | 模拟一条 fix（message_index=3, search="让我看看", replace="请直接执行"） | 解析出 MessageIndex=3, Search="让我看看", Replace="请直接执行" |
| UC-0006 | 多条 fix 解析 | 模拟 3 条 fix | HistoryFixes 长度=3，顺序保持 |

### M2 历史修正执行逻辑

| 编号 | 场景 | 操作 | 预期结果 |
|------|------|------|---------|
| UC-0007 | 消息序号定位 + 原文替换 | 历史中第 3 条 assistant 消息含 search 原文 | 该消息内 search 首次出现被替换为 replace |
| UC-0008 | 只替换首次出现 | 该消息内 search 出现 2 次 | 仅第 1 次被替换，第 2 次保留 |
| UC-0009 | 非 assistant 消息跳过 | message_index 指向 user/tool 消息 | 不修改该消息，跳过 |
| UC-0010 | 指定消息内找不到 search | message_index 指向的 assistant 消息不含 search | 在相邻消息（±1）兜底查找并替换 |
| UC-0011 | 相邻消息也找不到 | 指定消息及相邻消息均不含 search | 跳过该 fix，不报错 |
| UC-0012 | 仅最近 3 条 assistant 消息 | 历史有 5 条 assistant 消息，fix 指向第 1 条（超出最近 3 条） | 不修改超出范围的消息 |
| UC-0013 | 空 search 忽略 | fix 的 search 为空 | 跳过该 fix，不执行替换 |
| UC-0014 | search==replace 忽略 | fix 的 search 与 replace 相同 | 跳过该 fix，不执行替换 |

### M3 与原有处理叠加 + 开关门控

| 编号 | 场景 | 操作 | 预期结果 |
|------|------|------|---------|
| UC-0015 | 与 prompt_feedback 叠加 | 判定返回 history_fixes + suggested_action=prompt_feedback | 先执行历史修正，再追加 guidance 反馈消息 |
| UC-0016 | 与 delete_last_msg 叠加（先修正后 delete） | 判定返回 history_fixes + suggested_action=delete_last_msg | 先执行历史修正，再删除最后一条 assistant+tool 消息 |
| UC-0017 | loop-history-fix-enabled=off | `:set loop-history-fix-enabled off`，判定返回 history_fixes | 不执行历史修正，仅执行原有处理 |
| UC-0018 | 默认开启 | 未配置 loop-history-fix-enabled | 默认 on，执行历史修正 |
| UC-0019 | 非循环问题不修正 | 判定 type=tool_format_error 且含 history_fixes | 不执行历史修正（仅循环确认后修正） |

### M4 日志 + 用户可见提示

| 编号 | 场景 | 操作 | 预期结果 |
|------|------|------|---------|
| UC-0020 | 修正日志 | 执行一条历史修正 | 写日志记录消息序号、search、replace |
| UC-0021 | 用户可见提示 | 执行历史修正 | 终端/Web UI 提示"已修正第 N 条消息中的可疑话术" |
| UC-0022 | 无修正时无提示 | 判定返回空 history_fixes | 不显示修正提示 |

### M5 判定模型提示词 + 消息序号标定

| 编号 | 场景 | 操作 | 预期结果 |
|------|------|------|---------|
| UC-0023 | 提示词含 HISTORY 段 | 检查 KeyLoopJudgeUserPrompt | 含 {HISTORY} 占位符，说明 message_index 语义 |
| UC-0024 | HISTORY 填充带真实索引 | 历史有 3 条 assistant 消息（索引 2/3/4） | buildLoopJudgeUserPrompt 填充为 "[2] 内容"、"[3] 内容"、"[4] 内容" |
| UC-0025 | 工具调用消息排除 | 历史含带 tool_calls 的 assistant 消息 | getRecentAssistantHistory 排除该消息 |
| UC-0026 | 提示词说明 history_fixes 用法 | 检查提示词末尾 | 说明 type=loop 时可返回 history_fixes，message_index 用 [序号]，search 逐字匹配，仅修正 assistant 消息 |
| UC-0027 | 无 assistant 消息时 HISTORY 兜底 | 历史无 assistant 消息 | HISTORY 填充"无最近迭代内容"占位 |

## 验收标准

1. report_problem schema 支持 history_fixes 数组，解析正确
2. 历史修正执行逻辑满足：消息序号定位、原文匹配替换、相邻兜底、仅 assistant、只替换首次出现、仅最近 3 条
3. 与原有处理叠加，先修正后 delete
4. loop-history-fix-enabled 默认 on，可关闭
5. 写日志 + 用户可见提示
6. 所有单元测试通过，`go build ./... && go vet ./... && go build -o work/co-shell .` 全绿

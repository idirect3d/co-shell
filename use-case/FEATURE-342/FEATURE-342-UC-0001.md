# FEATURE-342: 问题判定优化（统一问题判定机制）

## 背景

当前 `default-problem-model` 在 `:set` 中只是只读展示项（自动计算第二高 ToolCall 模型），没有任何运行时消费。循环二次判定（judgeLoop）只消费 WorkMode 绑定的 ProblemModelID。错误处理分散在各处（循环/parse-error/context reorganize），没有统一的问题判定机制。

## 目标

建立统一问题判定机制：代码层硬分类（HTTP 状态码充分条件）+ 问题模型语义判定（report_problem 工具单工具强制），按优先级链选择问题模型，降级保护兜底。

## 架构

```
错误发生
  ├─ 硬编码层（充分条件直接拦截）：401/403(Key错误) 404(模型不存在) 429(限流) 5xx(服务端) → notify_user
  └─ 语义判定层（调问题模型，强制 report_problem 工具）：
       no_anomaly → continue（放行）
       loop → 循环介入（受 loop-judge-enabled 双门控）
       tool_format_error → delete_last_msg 或 prompt_feedback
       context_overflow → compact_context
       llm_connection_error → notify_user
       unknown → notify_user 或保守策略
```

问题模型优先级链：mode ProblemModelID > default-problem-model > default-tool-model > active model

## 用例

### M1 优先级链 + 全局配置

| 编号 | 场景 | 操作 | 预期结果 |
|------|------|------|---------|
| UC-0001 | mode 绑定 ProblemModelID | `.mode <当前> model problem <P>` 绑定问题模型 | `getProblemModelID()` 返回 P |
| UC-0002 | mode 未绑定，default-problem-model 已配置 | `:set default-problem-model <D>`，取消 mode 绑定 | 解析链落到 D |
| UC-0003 | 两级均未配置，default-tool-model 已配置 | `:set default-tool-model <T>`，mode 和 default-problem 均未设 | 解析链落到 T |
| UC-0004 | 三级均未配置 | 仅活跃模型可用 | 解析链落到当前活跃模型 |
| UC-0005 | default-problem-model 手动设置 | `:set default-problem-model <ID>` 设置有效模型 | 保存到 config.json `llm.default_problem_model` |
| UC-0006 | default-problem-model 未设置显示 auto | 删除配置后 `:set` | 显示自动计算的第二高 ToolCall 模型 |
| UC-0007 | 自动计算规则 | 3 个启用 ToolCall 模型按优先级 P1>P2>P3 | default-tool-model=P1, default-problem-model=P2 |
| UC-0008 | 全局配置设为不存在模型 | `:set default-problem-model nonexist` | 校验拒绝并提示，不清空原值 |
| UC-0009 | 全局配置设 none 解绑 | `:set default-problem-model none` | 清空配置，:set 回退 auto 显示 |

### M2 report_problem 工具 + 问题模型调用

| 编号 | 场景 | 操作 | 预期结果 |
|------|------|------|---------|
| UC-0010 | 问题模型必须调用工具 | 触发语义判定（如循环） | judgeClient 请求含 `tool_choice={"type":"function","function":{"name":"report_problem"}}` 且工具列表仅 1 个 |
| UC-0011 | 问题模型返回 report_problem | 模拟响应 type=loop + suggested_action=prompt_feedback | 解析出 ProblemReport 结构，guidance 作为反馈追加给主模型 |
| UC-0012 | no_anomaly 放行 | 模拟响应 type=no_anomaly + action=continue | 不干预主流程，继续正常迭代 |
| UC-0013 | 纯文本 fallback | 问题模型不支持工具返回 JSON 文本 | 走现有 JSON 解析（兼容现有 judgeLoop 纯文本路径） |
| UC-0013a | OpenAI 模式工具调用 | 问题模型通过 OpenAI 标准 function calling 返回 report_problem | 解析 tool_calls 中 report_problem 的参数为 ProblemReport |
| UC-0013b | XML 模式工具调用 | 问题模型通过 XML 标签返回 report_problem（如 `<cs:report_problem>...`） | 走 XML 解析管道解析出 ProblemReport |
| UC-0013c | 工具调用机制跟随主模型 | 主模型 toolcall-mode=openai/xml 时 | 问题模型使用与主模型一致的调用机制 |
| UC-0014 | guidance 自包含校验 | 模拟 delete_last_msg 场景 | guidance 不含 "参考刚才/如上" 等引用被删内容的措辞（单测校验） |
| UC-0015 | suggested_action 有效性 | 问题模型返回未知 action | 代码按默认保守策略处理（notify_user），不接受未知值 |

### M3 硬编码分类 + 动作执行

| 编号 | 场景 | 操作 | 预期结果 |
|------|------|------|---------|
| UC-0016 | HTTP 401 | LLM 调用返回 401 | 直接提示用户 API Key 错误，不调问题模型 |
| UC-0017 | HTTP 404 | LLM 调用返回 404 | 直接提示模型不存在/endpoint 错误，不调问题模型 |
| UC-0018 | HTTP 429 | LLM 调用返回 429 | 直接提示限流，不调问题模型 |
| UC-0019 | HTTP 5xx | LLM 调用返回 500 | 直接提示服务端故障，不调问题模型 |
| UC-0020 | 非充分条件 | HTTP 200 但业务错误码含敏感信息模糊 | 走问题模型语义判定 |
| UC-0021 | delete_last_msg 执行 | 问题模型建议 delete_last_msg | 调用 removeLastAssistantWithToolCalls() 删除错误调用消息后重试 |
| UC-0022 | compact_context 执行 | 问题模型建议 compact_context | 替换最后一条超限消息为"要求重整压缩上下文"提示 |
| UC-0023 | prompt_feedback 执行 | 问题模型建议 prompt_feedback | guidance 作为 user 消息追加给主模型 |

### M4 开关门控 + 降级保护

| 编号 | 场景 | 操作 | 预期结果 |
|------|------|------|---------|
| UC-0024 | problem-solver-enabled=off | `:set problem-solver-enabled off`，触发工具格式错误 | 不调问题模型，走现有 parse-error-action |
| UC-0025 | problem-solver=on + loop-judge=off | 触发循环 | 不调问题模型（双重门控），直接走 loop-intervention |
| UC-0026 | problem-solver=on + loop-judge=on | 触发循环 | 调问题模型判定是否真循环 |
| UC-0027 | 问题模型调用失败降级 | 绑定的 P 模型调用 500 | 降级到 default-problem-model D 重试 |
| UC-0028 | 降级后再失败 | D 模型也 500 | 停止并提示用户"问题识别模型不可达" |
| UC-0029 | 降级链全空 | 所有候选模型不可用 | 用活跃模型兜底，仍失败则停止并报告 |

### 配置与展示

| 编号 | 场景 | 操作 | 预期结果 |
|------|------|------|---------|
| UC-0030 | :set 显示新配置 | `:set` | 显示 problem-solver-enabled / default-problem-model / default-tool-model 新配置项 |
| UC-0031 | :config 向导 | `:config` | 新配置项出现在对应分组并可向导编辑 |
| UC-0032 | CLI 参数 | `--problem-solver-enabled off` | 生效并可被 :set 覆盖 |
| UC-0033 | i18n 双语 | 切换 zh/en | 新配置项帮助与提示均有中英文翻译 |

## 验证方式

1. **单元测试**：`agent/problem_solver_test.go`（解析/优先级链/动作执行/降级保护）
2. **单元测试**：`cmd/settings_problem_test.go`（:set 配置/自动计算/校验）
3. **运行命令**：`go build ./... && go test ./agent/ ./cmd/ ./i18n/ && go vet ./agent/ ./cmd/`
4. **审计**：`bin/output_audit.sh` Hardcoded Chinese=0 / i18n keys missing=0

## 验收标准

1. 全部单测通过，构建/vet 全绿
2. M1：:set 可配置 default-problem-model/default-tool-model，未设置时 auto 显示
3. M2：report_problem 工具强制调用，no_anomaly/loop 等类型正确解析
4. M3：HTTP 401/404/429/5xx 不调问题模型；非充分条件走 LLM 判定
5. M4：双开关门控正确，降级保护生效
6. i18n 双语完整
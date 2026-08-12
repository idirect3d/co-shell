// Author: L.Shuang
// Created: 2026-07-04
// Last Modified: 2026-08-05
//
// # MIT License
//
// # Copyright (c) 2026 L.Shuang
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
package i18n

func init() {
	zhMessages[KeyProblemSolverSystemPrompt] = `你是co-shell的问题诊断专家。你的唯一职责是分析当前的可疑信号（循环、工具格式错误、上下文超限、连接类异常等）并调用 report_problem 工具输出结构化诊断。

# 判定标准
- no_anomaly：看清楚了，没有真正的问题 → 不干预，继续正常流程
- loop：Agent在无意义地重复相同输出或工具调用，或已偏离原始目标
- tool_format_error：工具调用的参数/XML结构有误，需要删除或反馈修正
- context_overflow：上下文已超限或接近上限，需要重整
- llm_connection_error：连接级/凭据/模型不存在等无法通过提示词改进的异常
- unknown：信息不足，无法确定 → 宁可保守提示用户

# guidance 的编写要求（重要性最高，将作为下一条指令直接发送给主LLM）
1. **自包含**：主LLM可能看不到被删掉的消息，禁止"参考刚才""如上所述"等引用
2. **直接指示下一步**：用动作性动词开头（调用/读取/修改/搜索/执行/询问），指出具体工具、文件、命令
3. **重申主目标**：开头明确当前任务的终极目标（结合 {TASK} 提炼），防止偏离主线
4. **可执行性**：给出比当前更小的具体行动；信息不足时写明应向用户询问什么
5. **若目标已达成**：直接写"应调用 attempt_completion 退出"
6. **不重复已失败的策略**：若上下文中列出了"已尝试且失败的退出策略"，禁止重复其中任何一条（包括仅换措辞的等价表述），必须实质性升级——更具体的工具与参数、不同的切入角度；当失败策略已有 2 条以上时，应认真考虑建议放弃当前路线：直接给出阶段性结论、向用户提出具体问题、或调用 attempt_completion 收尾
7. **抢救有效分析**：对 loop 类问题，可疑内容中"重复开始之前"的部分可能包含有价值的分析成果；如判断其有价值，应将其要点浓缩进 guidance（直接引用关键结论），让主模型在此基础上继续推进，而不是从零重新分析
8. **预防性禁用（对 loop 类必做）**：主LLM下一轮极可能重蹈覆辙。先在 reason 中明确预测它将继续复读的**具体内容**（从可疑内容的重复样本中提炼，例如"将继续反复输出 'Let me search for X' 式的意图陈述而不发起工具调用"）；再在 guidance 中**显式禁用该模式**——直接点名禁止输出这类意图陈述/分析性前缀，并要求响应直接以工具调用开始、不得先输出任何分析性文字

# 输出要求
你必须调用 report_problem 工具，把诊断结果填入其参数，不要输出其他内容。
`
	zhMessages[KeyProblemSolverUserPrompt] = `# 原始任务
{TASK}

===

# 当前任务计划

{TASK_PLAN}

===

# 用户历史提示词（按时间顺序列出所有用户指令）

{USER_PROMPTS}

===

# 工作区与可用工具上下文

{CONTEXT}

===

# 异常信号类型

{ANOMALY_HINT}

===

# 异常详情（原始错误信息或可疑内容，已截断）

{ERROR_DETAIL}

===

请根据以上信息调用 report_problem 工具，给出结构化诊断：先判断是否为真正的问题，再给出针对主模型的自包含 guidance 和 suggested_action。
`
	zhMessages[KeyLoopJudgeUserPrompt] = `# 原始任务
{TASK}

===

# 当前任务计划

{TASK_PLAN}

===

# 用户历史提示词（按时间顺序列出所有用户指令）

{USER_PROMPTS}

===

# 最近迭代内容（最近两次迭代的返回，不含当前疑似循环的内容）

{ITERATIONS}

===

# 迭代工具调用序列（每轮迭代调用的工具名，用于判断是否有效推进）

{ITERATION_TOOLS}

===

# 已尝试且失败的退出策略（本任务中此前已下发但未能打破循环的策略；禁止重复或仅换措辞，必须实质性升级）

{FAILED_STRATEGIES}

===

# 工作区与可用工具上下文

{CONTEXT}

===

# 疑似循环内容（因检测到循环而被中断，内容可能不完整）

{SUSPECT_CONTENT}

===

请根据以上信息完成循环判定，并给出针对主模型的自包含、可执行的下一步指导（先点明终极目标作为执行锚点，再给出当前阶段的具体动作）。
`
	zhMessages[KeyLoopJudgeFallback] = `请围绕当前任务的下一步行动。如果仍有未完成的目标，请明确指示：就近选择一个具体文件/命令/搜索动作并立即执行；如果方向不明确，直接向用户提出具体问题。如果任务目标已达成，调用 attempt_completion 结束。`
	zhMessages[KeyLoopFailedStrategiesNone] = `（无——这是本任务首次循环判定）`
	zhMessages[KeyLoopDetectFeedback] = `现在应该复盘一下任务进展，如果最近几次迭代进展不大，应该围绕用户任务的终极目标进行思考，评估一下现有解决是否偏离了任务目标，或者考虑换个思路和方向解决问题。`
	// Display & description keys moved from zh.go
	zhMessages[KeyCol3LoopDetectEnabled] = "循环检测(on|off)"
	zhMessages[KeyCol3LoopJudgeEnabled] = "LLM循环二次判定"
	zhMessages[KeyCol3ShowLoopDetection] = "显示循环检测过程(on|off)"
	zhMessages[KeyCol3LoopJudgeModel] = "循环判定模型ID"
	zhMessages[KeyCol3LoopDetectThreshold] = "循环检测阈值(重复次数)"
	zhMessages[KeyCol3LoopDetectMaxWindow] = "循环检测滑动窗口大小"
	zhMessages[KeySettingsDescLoopJudge] = "启用后，检测到疑似循环时使用独立模型进行二次判定（默认启用）"
	zhMessages[KeySettingsDescLoopDetect] = "循环检测开关，检测LLM输出是否陷入死循环"
	zhMessages[KeySettingsDescLoopThreshold] = "循环检测阈值，连续重复内容触发干预的次数（默认 5）"
	zhMessages[KeySettingsDescLoopWindow] = "循环检测滑动窗口大小，用于检查重复模式的历史块数（默认 20）"
	zhMessages[KeyLoopDetectEnabledUpdated] = "✅ 循环检测已设置为: %s"
	zhMessages[KeyCLIHelpLoopIntervention] = "      --loop-intervention    循环介入策略（off/retry/prompt/reorganize/temperature/random，覆盖配置文件）"
	zhMessages[KeyReorganizeResult] = "✅ 上下文已重新整理：摘要 %d 字符。"
	zhMessages[KeyLoopReorganizeSuggestion] = "\n\n⚠️ 检测到循环后上下文已被重置。建议调用 reorganize_context 工具重新整理上下文，总结已做的工作和发现，并制定新的策略继续。"

	// Proactive/preventive intervention templates (discarded bad content, no post-mortem)
	zhMessages[KeyXMLParseErrorSuggestion] = `接下来调用 {TOOL_NAME} 方法时，请特别注意调用格式的正确性：
{FORMAT}
确保每个标签正确闭合，参数值如果包含特殊字符（<、>、&），请用 <![CDATA[...]]> 包裹。`
	zhMessages[KeyContentLoopSuggestion] = `当前问题可能遇到了瓶颈，继续同样的分析方式不太可能带来新的突破。换一种不同的思路来推进——使用不同的工具组合、换一个分析角度、或向用户提问澄清需求。如果任务目标已经达成，请调用 attempt_completion。`
	zhMessages[KeyToolRepeatSuggestion] = `刚才使用的工具组合可能不是解决当前问题的最有效方式。接下来尝试使用不同的工具或不同的参数来推进任务。如果对需求有困惑，先向用户提问。`
	zhMessages[KeyContentDupSuggestion] = `当前进展可能遇到了瓶颈，继续同样的分析方式不太可能带来新的突破。如果任务目标已经达成，请调用 attempt_completion 离开循环。否则请换一种不同的思路，或向用户提问以获得更多线索。`
}

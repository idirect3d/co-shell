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

# 输出要求
你必须调用 report_problem 工具，把诊断结果填入其参数，不要输出其他内容。
`
	zhMessages[KeyLoopJudgeSystemPrompt] = `你是co-shell的死循环检测分析器。你的唯一职责是分析Agent行为并判断是否陷入死循环。

# 判定标准
- 内容重复：Agent在无意义地重复相同的输出或工具调用
- 目标偏离：当前行为已偏离原始任务目标
- 缺乏进展：反复尝试相同的失败方案，导致任务无有效进展

请以JSON格式**直接**返回以下结果（不要包含其他内容），并确保 JSON 格式正确，可以被标准 JSON 解析器解析：
- is_loop 必须是 true 或 false（布尔值），绝不能写 true/false 字样
- reason 这个参数将显示给用户，目的是让用户了解判定循环的原因（不会传给LLM）
- exit_strategy 将直接作为下一条指令发送给LLM（不附带被检测到的问题内容），因此必须给出纯粹的**前瞻性指导意见**。关键原则：
  (1) **不回溯过去**：不提及"刚才""停止重复""避免循环"等——被检测的内容已被丢弃，LLM看不到
  (2) **不评估现状**：不写"当前方案无效""缺乏进展"等否定性评价
  (3) **聚焦下一步**：给出具体、可执行的下一步指令，禁止空泛话术
  (4) **如目标已达成**：直接写"应调用 attempt_completion 退出"

# exit_strategy 的编写要求（重要性最高）
exit_strategy 是给主 LLM 的唯一指令，**必须让主 LLM 能直接执行**。请按以下要求编写：

1. **使用动作性动词开头**：如"调用""读取""修改""搜索""执行""询问"——避免"考虑""尝试""重新审视"等空泛动词
2. **给出明确作用对象**：具体工具名（read_file / replace_in_file / search_files / execute_command / shell_send / ask_followup_question 等）、具体文件名、具体命令、或具体问题内容
3. **指出可落地的下一步**：优先推荐一个比当前更小的具体行动（例如"用 search_files 搜索 'xxx' 定位配置项，再 read_file 读取第 20-40 行"，而不是"换个思路"）
4. **信息不足时告知如何获取**：如果无法从已知上下文确定下一步，应明确指出应向用户询问什么（写"向用户询问：……"），把问题写具体
5. **禁止空泛话术**：不得出现"换个思路""换个方向""尝试其他方法""继续努力""重新评估"等没有作用对象的表述
6. **区分场景**：
   - 反复搜索无果 → 改用不同关键词、扩大/缩小搜索范围、或换用其他检索工具
   - 反复执行同一命令 → 先列出目录/文件再针对具体文件操作
   - 反复输出分析 → 收敛为一次具体工具调用或直接给结论
   - 找不到入口/方向 → 明确写出应向用户提出的具体问题
   - 信息不完整 → 指定读取哪一个文件、哪一段内容
7. **先锚定终极目标**：在给出任何下一步之前，先点明任务的**终极目标**（主目标/最终交付物，结合 {TASK} 与用户历史提示词提炼），作为后续每一步的执行锚点，确保建议的下一步不偏离任务主线；若终极目标已达成，则直接写"应调用 attempt_completion 退出"
8. **再明确阶段目标与优先级顺序**：在锚定终极目标之后，点明当前阶段应聚焦的阶段性目标（结合迭代工具序列判断目标达成进度）；若当前阶段存在多个层级目标（子目标、或必须按顺序完成的目标），应按优先级列出建议执行顺序（如"第 1 步：…；第 2 步：…"），并标注最高优先级事项；每一步都应能溯源到终极目标（在述中体现"该步骤服务于 …"）

# 判模输出格式要求
- 必须严格输出 JSON，不要包含 JSON 之外的内容
- is_loop 为 true 时 exit_strategy **必须非空**且为可执行指令，否则判定结果将被视为无效
- is_loop 为 false 时 exit_strategy 可以为空字符串 ""

## 样例
以下样例展示**好的 exit_strategy**（可执行、有对象、有动作）：

样例1（修改文件循环）：
{"is_loop": true, "reason": "连续5次尝试以相同方式修改同一文件但未读取最新内容", "exit_strategy": "先用 read_file 读取 config/config.go 的当前内容（前 50 行），再基于读取结果用 replace_in_file 修改第 15 行左右的模型配置。"}

样例2（搜索无果循环）：
{"is_loop": true, "reason": "反复用相同关键词搜索但未找到目标", "exit_strategy": "改用 search_files 在 agent/ 目录下搜索 'applyLoopFeedback'，file_pattern 设为 *.go；若仍无结果，改用 grep -rn 'applyLoopFeedback' agent/。"}

样例3（大任务拆解）：
{"is_loop": true, "reason": "试图一次性读完全部文件再统一处理，超出能力范围", "exit_strategy": "立即对 use-case/FIX-322/ 下的文件逐个处理：先用 list_files 查看该目录内容，然后按 UC 编号顺序，每处理完一个文件就记录结果并继续下一个。"}

样例4（需要向用户澄清）：
{"is_loop": true, "reason": "任务方向不明确，无法确定下一步", "exit_strategy": "向用户询问：请明确本次优化希望优先覆盖的场景（文件修改/命令执行/搜索无果/需要澄清），以及是否允许我调整模板文件。"}

样例5（确认不是循环）：
{"is_loop": false, "reason": "虽然内容较长但每次输出都在分析不同维度", "exit_strategy": ""}

样例6（多层级目标优先级，先锚定终极目标）：
{"is_loop": true, "reason": "已完成代码定位与阅读，但仍在重复分析未进入修改阶段", "exit_strategy": "终极目标：使模板编辑页面的删除按钮功能正常（事件委托正确、删除后重新编号）。当前阶段目标：修复 web/static/js/app.js 的删除事件处理逻辑。按优先级执行：第 1 步用 read_file 读取 web/static/js/app.js 第 130-160 行确认当前实现；第 2 步用 replace_in_file 修正删除事件委托；第 3 步用 browser_navigate 打开模板编辑页面并点击删除按钮验证，确保最终达成"删除功能正常"的终极目标。"}

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

# 工作区与可用工具上下文

{CONTEXT}

===

# 疑似循环内容（因检测到循环而被中断，内容可能不完整）

{SUSPECT_CONTENT}

===

# 输出格式

exit_strategy 中请先点明**终极目标**作为执行锚点，再点明当前阶段任务目标；若任务含多个层级目标，请按优先级列出建议执行顺序，并确保每步服务于终极目标。

{"is_loop": false/true, "reason": "xxx", "exit_strategy": "xxx(is_loop 为 false 时可不填)"}
** 请严格按约定的JSON格式，直接返回判定结果，而不要输出思考过程 **
`
	zhMessages[KeyLoopJudgeFallback] = `请围绕当前任务的下一步行动。如果仍有未完成的目标，请明确指示：就近选择一个具体文件/命令/搜索动作并立即执行；如果方向不明确，直接向用户提出具体问题。如果任务目标已达成，调用 attempt_completion 结束。`
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

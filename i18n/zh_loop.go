// Author: L.Shuang
// Created: 2026-07-04
// Last Modified: 2026-07-04
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
// IMPLIED, BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
package i18n

func init() {
	zhMessages[KeyLoopJudgeSystemPrompt] = `你是co-shell的死循环检测分析器。你的唯一职责是分析Agent行为并判断是否陷入死循环。

# 判定标准
- 内容重复：Agent在无意义地重复相同的输出或工具调用
- 目标偏离：当前行为已偏离原始任务目标
- 缺乏进展：反复尝试相同的失败方案，导致任务无有效进展

请以JSON格式**直接**返回以下结果（不要包含其他内容），并确保 JSON 格式正确，可以被标准 JSON 解析器解析：
- is_loop 必须是 true 或 false（布尔值），绝不能写 true/false 字样
- reason 这个参数将显示给用户，目的是让用户了解判定循环的原因（不会传给LLM）
- exit_strategy 针对循环或当前方案无进展的情况，应给出实质性建议，比如："不要用XXX方法，应尝试使用YYY方法等。"。注意：这一条是给LLM的，且不会告诉LLM上次最后的处理已经循环了，也不会把循环或无进展的内容给LLM，因此这里直接写下一步操作的意见即可。

示例（确认是循环时返回）：
{"is_loop": true, "reason": "连续5次输出相同内容，无任何进展", "exit_strategy": "注意不要通过sed、cat等命令行方式修改或替换脚本程序，也不要轻易重建文件，应坚持使用replace_in_file在原文件上进行修改。"}

示例（确认无实质进展时返回）：
{"is_loop": true, "reason": "当前待处理的文件过多，LLM能力可能无法支撑读完全部文件内容后再统一处理", "exit_strategy": "由于文件数量比较多，每个文件较大，应先逐个文件分别处理，读取一个文件，就处理一个，记录一个，然后再处理下一个文件，不断迭代，化整为零"}

示例（确认不是循环时返回）：
{"is_loop": false, "reason": "虽然内容较长但每次输出都在分析不同维度", "exit_strategy": ""}

===
`
	zhMessages[KeyLoopJudgeUserPrompt] = `# 原始任务
{TASK}

===

# 当前任务计划

{TASK_PLAN}

===

# 用户最后指令

{LAST_INPUT}

===

# 最近迭代内容（最近两次迭代的返回，不含当前疑似循环的内容）

{ITERATIONS}

===

# 疑似循环内容（因检测到循环而被中断，内容可能不完整）

{SUSPECT_CONTENT}

===

# 解决问题的一般策略（优先级从高到低）

1. 回归任务目标，重新评估当前进度
2. 换一个完全不同的工具或方法
3. 将问题拆解为更小的子步骤
4. 检查已获取信息是否足够，是否需要向用户提问
5. 总结已有发现，尝试用不同方式组织思路

请严格按约定的JSON格式，直接返回判定结果。
`
	zhMessages[KeyLoopDetectFeedback] = `⚠️ 检测到你的输出陷入了死循环（连续重复相似内容，详见后面错误信息）。
你需要先停下来，休息一下换换脑子。深呼吸，我来指导你脱离出去。首先，围绕用户任务的终极目标（即<task></task>中的内容）进行思考，评估一下距离目标偏离了多少，然后换个思路和方向解决问题。

错误信息：%s`
	zhMessages[KeyToolCallLoopFeedback] = `⚠️ 检测到工具调用陷入了循环: 工具「%s」在连续多轮迭代中使用了完全相同的参数。请立即停止当前做法，换用完全不同的工具或方法。如果：
1. 需要读文件，试试 search_files 来找线索
2. 需要修改代码，先完整理解上下文再动手
3. 不确定怎么做，停下来问用户更多信息

记住：保持冷静，换个思路，不要重复做同样的事。`

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
	zhMessages[KeyCLIHelpLoopDetect] = "      --loop-detect-enabled  启用循环检测（on/off，覆盖配置文件）"
	zhMessages[KeyCLIHelpLoopDetectEnabled] = "      --loop-detect-enabled  启用循环检测功能（覆盖配置文件）"
	zhMessages[KeyCLIHelpLoopDetectDisabled] = "      --loop-detect-disabled 禁用循环检测功能（覆盖配置文件）"
	zhMessages[KeyReorganizeResult] = "✅ 上下文已重新整理：摘要 %d 字符。"
	zhMessages[KeyLoopReorganizeSuggestion] = "\n\n⚠️ 检测到循环后上下文已被重置。建议调用 reorganize_context 工具重新整理上下文，总结已做的工作和发现，并制定新的策略继续。"
	zhMessages[KeyDuplicateContentFeedback] = "⚠️ 检测到你本次回复的内容与上一次完全一致。你已经进行了完整的分析，不要再重复相同的文字，请梳理之前完整上下文和任务清单，在能够明确任务目标的基础上，重新对任务进行规划，并通过track_task_progress进行任务跟踪，通过调用合适的工具继续推进任务。如果任务目标不够明确，请调用 ask_followup_question 向用户说明情况，以便请用户提供更多的素材。"
}

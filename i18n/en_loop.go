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
	enMessages[KeyLoopJudgeSystemPrompt] = `You are co-shell's dead-loop detection analyzer. Your sole responsibility is to analyze agent behavior and determine if it is stuck in a dead loop.

# Judgment Criteria
- Content Repetition: The agent is repeatedly outputting the same content or making the same tool calls
- Goal Deviation: Current behavior has deviated from the original task objective
- Lack of Progress: Repeatedly attempting the same failed strategy

Return the result in JSON format (DO NOT include any other content).
- is_loop must be true or false (boolean), never write the literal string "true/false"
- Ensure the JSON is valid and parseable by a standard JSON parser

Example (confirmed loop):
{"is_loop": true, "reason": "Same content output 5 times in a row with no progress", "exit_strategy": "Stop current approach, reassess task goals and progress"}

Example (not a loop):
{"is_loop": false, "reason": "Each output analyzes a different dimension", "exit_strategy": ""}
`
	enMessages[KeyLoopJudgeUserPrompt] = `## Original Task
{TASK}

===

## Current Task Plan

{TASK_PLAN}

===

## User's Last Instruction

{LAST_INPUT}

===

# 最近迭代内容（最近两次迭代的返回，不含当前疑似循环的内容）

{ITERATIONS}

===

## Suspected Loop Content (interrupted due to loop detection, content may be incomplete)

{SUSPECT_CONTENT}

===

## General Problem-Solving Strategies (Priority from High to Low)

1. Rethink the original task goal, reassess current progress
2. Switch to a completely different tool or approach
3. Break the problem into smaller sub-steps
4. Check if you have enough information, or if you need to ask the user for more
5. Summarize findings and try organizing your thoughts differently

Please analyze and return the judgment result.
`
	enMessages[KeyLoopDetectFeedback] = `⚠️ Your output appears to be stuck in a loop (repeating similar content consecutively, see error details below).
First, pause and take a deep breath. I'll guide you out of this. Start by thinking about the ultimate goal of the user's task (the content inside <task></task>), assess how far you've deviated from the goal, then try a different approach and direction to solve the problem.

Error details: %s`
	enMessages[KeyCol3LoopDetectEnabled] = "Loop detect (on|off)"
	enMessages[KeyCol3LoopJudgeEnabled] = "LLM loop judgment"
	enMessages[KeyCol3ShowLoopDetection] = "show loop detection(on|off)"
	enMessages[KeyCol3LoopJudgeModel] = "Loop judge model ID"
	enMessages[KeyCol3LoopDetectThreshold] = "Loop detect threshold (repeats)"
	enMessages[KeyCol3LoopDetectMaxWindow] = "Loop detect window size"
	enMessages[KeySettingsDescLoopJudge] = "When enabled, an independent model is used for secondary judgment when a suspected loop is detected (default: enabled)"
	enMessages[KeySettingsDescLoopDetect] = "Enable loop detection to detect if LLM output is stuck in a loop"
	enMessages[KeySettingsDescLoopThreshold] = "Loop detection threshold, number of consecutive repeats to trigger intervention (default 5)"
	enMessages[KeySettingsDescLoopWindow] = "Loop detection sliding window size, history chunks to check for repeating patterns (default 20)"
	enMessages[KeyLoopDetectEnabledUpdated] = "✅ Loop detection set to: %s"
	enMessages[KeyLoopReorganizeSuggestion] = "\n\n⚠️ Loop detected and context has been reset. It is recommended to call the reorganize_context tool to reorganize the context, summarize completed work and findings, and formulate a new strategy to continue."
}

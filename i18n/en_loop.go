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
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
package i18n

func init() {
	enMessages[KeyLoopJudgeSystemPrompt] = `You are co-shell's dead-loop detection analyzer. Your sole responsibility is to analyze agent behavior and determine whether it is stuck in a dead loop.

# Judgment Criteria
- Content repetition: The agent is meaninglessly repeating the same output or tool calls
- Goal deviation: Current behavior has deviated from the original task goal
- Lack of progress: Repeatedly trying the same failed approaches with no effective progress

Return the result in JSON format **directly** (do not include any other content). Ensure the JSON is valid:
- is_loop must be true or false (boolean type, never the string "true"/"false")
- reason is shown to the user to explain why a loop was determined (not passed to LLM)
- exit_strategy provides actionable advice for the LLM, e.g.: "Don't use XXX method, try YYY instead." Note: this is given to the LLM **without** telling it about the loop. The response is discarded, so provide direct next-step instructions.

Example (loop confirmed):
{"is_loop": true, "reason": "Output identical content 5 consecutive times, no progress", "exit_strategy": "Do not use sed/cat commands to modify scripts or rebuild files. Use replace_in_file for all file modifications."}

Example (no substantive progress):
{"is_loop": true, "reason": "Too many files pending, LLM cannot buffer all file contents", "exit_strategy": "Process files one at a time: read one file, process it, record the result, then move to the next. Break the large task into smaller iterations."}

Example (not a loop):
{"is_loop": false, "reason": "Output is long but each iteration analyzes a different dimension", "exit_strategy": ""}

===
`
	enMessages[KeyLoopJudgeUserPrompt] = `# Original Task
{TASK}

===

# Current Task Plan

{TASK_PLAN}

===

# Last User Instruction

{LAST_INPUT}

===

# Recent Iterations (last 2 assistant responses without current suspect)

{ITERATIONS}

===

# Suspected Loop Content (interrupted by detection, may be incomplete)

{SUSPECT_CONTENT}

===

# Output Format

{"is_loop": false/true, "reason": "xxx", "exit_strategy": "xxx(optional if is_loop is false)"}
** Return ONLY the JSON, no thinking or reasoning output **
`
	enMessages[KeyLoopDetectFeedback] = `Please review your progress on the task. If recent iterations show little progress, refocus on the user's ultimate goal (the content inside <task></task>), assess whether your current approach has deviated from the goal, or consider a different direction and strategy to solve the problem.`
	// Display & description keys moved from en.go
	enMessages[KeyCol3LoopDetectEnabled] = "Loop Detect(on|off)"
	enMessages[KeyCol3LoopJudgeEnabled] = "LLM Loop Judgment"
	enMessages[KeyCol3ShowLoopDetection] = "Show Loop Detection(on|off)"
	enMessages[KeyCol3LoopJudgeModel] = "Loop Judge Model ID"
	enMessages[KeyCol3LoopDetectThreshold] = "Loop Detect Threshold(repeat count)"
	enMessages[KeyCol3LoopDetectMaxWindow] = "Loop Detect Max Window"
	enMessages[KeySettingsDescLoopJudge] = "When enabled, uses a separate model for secondary loop judgment (default: enabled)"
	enMessages[KeySettingsDescLoopDetect] = "Loop detection switch, detects if LLM output is stuck in a dead loop"
	enMessages[KeySettingsDescLoopThreshold] = "Loop detection threshold, consecutive repeats triggering intervention (default 5)"
	enMessages[KeySettingsDescLoopWindow] = "Loop detection sliding window size for checking repeat patterns (default 20)"
	enMessages[KeyLoopDetectEnabledUpdated] = "✅ Loop detection set to: %s"
	enMessages[KeyCLIHelpLoopIntervention] = "      --loop-intervention    Loop intervention strategy (off/retry/prompt/reorganize/temperature/random, overrides config)"
	enMessages[KeyReorganizeResult] = "✅ Context reorganized: %d chars summary."
	enMessages[KeyLoopReorganizeSuggestion] = "\n\n⚠️ Loop detected and context has been reset. It is recommended to call the reorganize_context tool to reorganize the context, summarize completed work and findings, and formulate a new strategy to continue."

	// Proactive/preventive intervention templates (discarded bad content, no post-mortem)
	enMessages[KeyXMLParseErrorSuggestion] = `When calling {TOOL_NAME} next, pay special attention to the correct format:
{FORMAT}
Ensure every tag is properly closed. If parameter values contain special characters (<, >, &), wrap them in <![CDATA[...]]>.`
	enMessages[KeyContentLoopSuggestion] = `The current approach may have hit a bottleneck. Continuing the same analysis is unlikely to bring new breakthroughs. Try a different strategy — use different tool combinations, change your analytical angle, or ask the user for clarification. If the task goal has been achieved, call attempt_completion.`
	enMessages[KeyToolRepeatSuggestion] = `The tool combination you just used may not be the most effective way to solve the current problem. Try using different tools or different parameters next. If you're unsure about requirements, ask the user first.`
	enMessages[KeyContentDupSuggestion] = `Progress may have hit a bottleneck. Continuing the same analysis is unlikely to bring new breakthroughs. If the task goal has been achieved, call attempt_completion to exit the loop. Otherwise, try a different approach or ask the user for more clues.`
}

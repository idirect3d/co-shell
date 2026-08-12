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
	enMessages[KeyProblemSolverSystemPrompt] = `You are co-shell's problem diagnostic expert. Your sole responsibility is to analyze the current suspicious signal (loop, tool format error, context overflow, connection-level error, etc.) and produce a structured diagnosis by calling the report_problem tool.

# Classification criteria
- no_anomaly: you examined it carefully and found no real problem - do not intervene, keep the normal flow
- loop: the agent is meaninglessly repeating the same output or tool calls, or has drifted from the original goal
- tool_format_error: a tool call's arguments/XML structure is malformed and needs deletion or corrective feedback
- context_overflow: the context window is over or near the limit and needs reorganization
- llm_connection_error: connection/credential/model-not-found errors that cannot be fixed by better prompts
- unknown: insufficient information - prefer a conservative notify_user

# Guidance writing requirements (most important; it becomes the next instruction sent directly to the main LLM)
1. **Self-contained**: the main LLM may not see deleted messages; never write "as above" or "as mentioned earlier"
2. **Directly instruct the next step**: start with action verbs (call/read/modify/search/execute/ask), naming concrete tools, files, commands
3. **Re-state the goal**: begin by stating the ultimate goal (derived from {TASK}) to keep the main model on track
4. **Executable**: give a smaller concrete action; if information is insufficient, state exactly what to ask the user
5. **If the goal is achieved**: directly write "call attempt_completion to exit"
6. **Never repeat failed strategies**: if the context lists "previously tried and failed exit strategies", do not repeat any of them (including reworded equivalents); escalate substantively — more concrete tools and parameters, or a different angle; when 2 or more strategies have already failed, seriously consider advising to abandon the current line of attack: give a partial conclusion, ask the user a concrete question, or call attempt_completion to wrap up
7. **Salvage valuable analysis**: for loop problems, the part of the suspicious content BEFORE the repetition started may contain valuable analysis; if you judge it valuable, condense its key conclusions into the guidance (quote them directly), so the main model continues from that basis instead of re-analyzing from scratch
8. **Preemptive ban (mandatory for loop problems)**: the main model is very likely to repeat itself next round. First, in reason, explicitly predict the **specific content** it will keep repeating (distilled from the repetition samples in the suspicious content, e.g. "it will keep emitting 'Let me search for X' intent statements without issuing a tool call"); then, in guidance, **explicitly ban that pattern** — name and forbid such intent statements / analytical preambles, and require the response to start directly with a tool call, with no analytical text first

# Output requirements
You MUST call the report_problem tool and fill its arguments with the diagnosis. Do not output anything else.
`
	enMessages[KeyProblemSolverUserPrompt] = `# Original Task
{TASK}

===

# Current Task Plan

{TASK_PLAN}

===

# User history prompts (all user instructions in chronological order)

{USER_PROMPTS}

===

# Workspace & available tools context

{CONTEXT}

===

# Anomaly signal type

{ANOMALY_HINT}

===

# Anomaly detail (raw error text or suspicious content, truncated)

{ERROR_DETAIL}

===

Based on the above information, call the report_problem tool and produce a structured diagnosis: first decide whether this is a real problem, then provide a self-contained guidance for the main model and a suggested_action.
`
	enMessages[KeyLoopJudgeUserPrompt] = `# Original Task
{TASK}

===

# Current Task Plan

{TASK_PLAN}

===

# User History Prompts (all genuine user instructions in chronological order)

{USER_PROMPTS}

===

# Recent Iterations (last 2 assistant responses without current suspect)

{ITERATIONS}

===

# Iteration Tool Call Sequence (tool names called per iteration, to assess real progress)

{ITERATION_TOOLS}

===

# Previously Tried and Failed Exit Strategies (issued earlier in this task but did not break the loop; do NOT repeat or merely reword them — escalate substantively)

{FAILED_STRATEGIES}

===

# Workspace & Available Tools Context

{CONTEXT}

===

# Suspected Loop Content (interrupted by detection, may be incomplete)

{SUSPECT_CONTENT}

===

Based on the above information, complete the loop judgment and provide a self-contained, executable next-step instruction for the main model (state the ultimate goal as the execution anchor first, then the concrete action for the current stage).
`
	enMessages[KeyLoopJudgeFallback] = `Focus on the next action for the current task. If there are unfinished goals, clearly direct: pick a concrete file/command/search action nearby and execute it immediately; if the direction is unclear, ask the user a specific question directly. If the task goal has been achieved, call attempt_completion to finish.`
	enMessages[KeyLoopFailedStrategiesNone] = `(none — first loop judgment for this task)`
	enMessages[KeyLoopDetectFeedback] = `Please review your progress on the task. If recent iterations show little progress, refocus on the user's ultimate goal, assess whether your current approach has deviated from the goal, or consider a different direction and strategy to solve the problem.`
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

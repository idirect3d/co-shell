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
	enMessages[KeyLoopJudgeSystemPrompt] = `You are co-shell's dead-loop detection analyzer. Your sole responsibility is to analyze agent behavior and determine whether it is stuck in a dead loop.

# Judgment Criteria
- Content repetition: The agent is meaninglessly repeating the same output or tool calls
- Goal deviation: Current behavior has deviated from the original task goal
- Lack of progress: Repeatedly trying the same failed approaches with no effective progress

Return the result in JSON format **directly** (do not include any other content). Ensure the JSON is valid:
- is_loop must be true or false (boolean type, never the string "true"/"false")
- reason is shown to the user to explain why a loop was determined (not passed to LLM)
- exit_strategy will be sent to the LLM as the next instruction **without** the detected problematic content. It MUST be purely **forward-looking guidance**. Key principles:
  (1) **No backtracking**: Don't say "you just", "stop repeating", "avoid the loop" — the detected content is discarded, the LLM cannot see it
  (2) **No status evaluation**: Don't write "current approach is failing", "lack of progress" etc.
  (3) **Focus on the next step**: Give a concrete, executable next-step instruction. No vague talk.
  (4) **If the goal is achieved**: Write "call attempt_completion to finish"

# Requirements for writing exit_strategy (highest priority)
exit_strategy is the only instruction given to the main LLM. **It must be directly executable by the main LLM.** Follow these requirements:

1. **Start with an action verb**: e.g. "call", "read", "modify", "search", "run", "ask" — avoid vague verbs like "consider", "try", "re-examine"
2. **Name a concrete target**: a specific tool name (read_file / replace_in_file / search_files / execute_command / shell_send / ask_followup_question etc.), a specific file name, a specific command, or specific question content
3. **Point to an actionable next step**: prefer one concrete, smaller action over the current one (e.g. "use search_files to search for 'xxx' to locate the config, then read_file lines 20-40" instead of "change approach")
4. **When information is insufficient, say how to get it**: if the next step cannot be determined from known context, clearly state what to ask the user (write "ask the user: ...") and make the question concrete
5. **No vague language**: do not use "change approach", "try a different direction", "try other methods", "keep going", "re-assess" or any other wording without a clear object
6. **Scenario-specific guidance**:
   - Repeated fruitless searches → use different keywords, widen/narrow the search scope, or switch to another search tool
   - Repeatedly running the same command → first list the directory/files, then operate on a specific file
   - Repeatedly outputting analysis → converge into one concrete tool call or give the conclusion directly
   - No entry point / direction → clearly write the concrete question to ask the user
   - Incomplete information → specify which file and which range to read
7. **Anchor the ultimate goal first**: before giving any next step, state the task's **ultimate goal** (the main goal / final deliverable, distilled from {TASK} and the user history prompts) as the execution anchor for every subsequent step, so the suggested next step never deviates from the main thread; if the ultimate goal is already achieved, write "call attempt_completion to finish"
8. **Then state the current-stage goal and priority order**: after anchoring the ultimate goal, state which stage goal the current stage should focus on (determine the progress using the iteration tool sequence); if the current stage has multiple hierarchical goals (sub-goals, or goals that must be completed in order), list the suggested execution order by priority (e.g. "Step 1: …; Step 2: …") and mark the highest-priority item; every step should be traceable to the ultimate goal (phrase it as "this step serves the goal of …")

# Output format requirements
- Return strictly JSON, nothing outside it
- When is_loop is true, exit_strategy **must be non-empty** and executable, otherwise the result is considered invalid
- When is_loop is false, exit_strategy may be an empty string ""

## Examples
The following examples show **good exit_strategy** (executable, with a target and an action):

Example 1 (file-modification loop):
{"is_loop": true, "reason": "Tried to modify the same file 5 times with the same approach without reading the latest content", "exit_strategy": "First use read_file to read the current content of config/config.go (first 50 lines), then use replace_in_file to modify the model configuration around line 15 based on what was read."}

Example 2 (fruitless search loop):
{"is_loop": true, "reason": "Repeatedly searched with the same keyword but did not find the target", "exit_strategy": "Switch to search_files in the agent/ directory with file_pattern *.go for 'applyLoopFeedback'; if still nothing, use grep -rn 'applyLoopFeedback' agent/."}

Example 3 (large-task decomposition):
{"is_loop": true, "reason": "Attempting to read all files at once then process them uniformly, beyond capability", "exit_strategy": "Process the files under use-case/FIX-322/ one by one: first use list_files to view the directory contents, then process each file in UC number order, recording the result after each file before moving to the next."}

Example 4 (needs user clarification):
{"is_loop": true, "reason": "Task direction is unclear, cannot determine the next step", "exit_strategy": "Ask the user: please clarify which scenario this optimization should prioritize (file modification / command execution / fruitless search / needing clarification), and whether I am allowed to adjust the template files."}

Example 5 (not a loop):
{"is_loop": false, "reason": "Output is long but each iteration analyzes a different dimension", "exit_strategy": ""}

Example 6 (multi-level goal priority, ultimate goal anchored first):
{"is_loop": true, "reason": "Finished locating and reading the code, but still outputting analysis instead of entering the modification stage", "exit_strategy": "Ultimate goal: make the delete button on the template editor page work correctly (correct event delegation, re-numbering after delete). Current-stage goal: fix the delete event handling in web/static/js/app.js. Execute in priority order: Step 1 read_file web/static/js/app.js lines 130-160 to confirm the current implementation; Step 2 replace_in_file to correct the delete event delegation; Step 3 browser_navigate to open the template editor page and click the delete button to verify, ensuring the ultimate goal of a working delete feature is achieved."}

===
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

# Workspace & Available Tools Context

{CONTEXT}

===

# Suspected Loop Content (interrupted by detection, may be incomplete)

{SUSPECT_CONTENT}

===

# Output Format

In exit_strategy, first state the **ultimate goal** as the execution anchor, then state the current-stage task goal; if the task has multiple hierarchical goals, list the suggested execution order by priority, ensuring every step serves the ultimate goal.

{"is_loop": false/true, "reason": "xxx", "exit_strategy": "xxx(optional if is_loop is false)"}
** Return ONLY the JSON, no thinking or reasoning output **
`
	enMessages[KeyLoopJudgeFallback] = `Focus on the next action for the current task. If there are unfinished goals, clearly direct: pick a concrete file/command/search action nearby and execute it immediately; if the direction is unclear, ask the user a specific question directly. If the task goal has been achieved, call attempt_completion to finish.`
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

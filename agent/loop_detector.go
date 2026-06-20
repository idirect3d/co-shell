// Author: L.Shuang
// Created: 2026-05-13
// Last Modified: 2026-06-14
//
// MIT License
//
// Copyright (c) 2026 L.Shuang
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

package agent

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/idirect3d/co-shell/log"
)

// LoopDetector monitors LLM output for repeating lines that indicate
// the LLM is stuck in a loop. It counts exact line repetitions and
// triggers intervention when the same line appears too many times.
//
// Key design:
//   - Lines shorter than minLineLen are ignored (filters XML tags, short prompts)
//   - Each AddChunk call appends to accumulated content and extracts complete lines
//   - accumulated content tracks only one trailing incomplete line across chunks
//   - Reset() must be called at the start of each LLM iteration to clear counts
type LoopDetector struct {
	threshold   int             // min line occurrences to trigger (default: 5)
	minLineLen  int             // lines shorter than this are ignored (default: 50)
	accumulated strings.Builder // accumulates incomplete trailing line across chunks
	lineCounts  map[string]int  // exact line content → occurrence count
}

// LoopDetectedError is returned when a loop is detected.
type LoopDetectedError struct {
	pattern     string    // the repeated line content
	repeatCount int       // how many times it repeated
	threshold   int       // detection threshold
	startTime   time.Time // first occurrence
	endTime     time.Time // last occurrence
	suggestion  string    // suggestion for the agent to correct
}

func (e *LoopDetectedError) Error() string {
	msg := fmt.Sprintf(
		"LLM output loop detected: line repeated %d times (threshold: %d). ",
		e.repeatCount, e.threshold,
	)

	if e.pattern != "" {
		msg += fmt.Sprintf("Line content (first 200 chars): %s. ", truncateString(e.pattern, 200))
	}

	msg += e.suggestion
	return msg
}

// NewLoopDetector creates a new loop detector with the given configuration.
// threshold: min occurrences of the same line to trigger (default: 5).
// minLineLen: lines shorter than this are ignored (default: 50).
func NewLoopDetector(threshold int, minLineLen int) *LoopDetector {
	if threshold <= 0 {
		threshold = 5
	}
	if minLineLen <= 0 {
		minLineLen = 50
	}
	log.Debug("LoopDetector: created with threshold=%d, minLineLen=%d", threshold, minLineLen)
	return &LoopDetector{
		threshold:  threshold,
		minLineLen: minLineLen,
		lineCounts: make(map[string]int),
	}
}

// AddChunk adds a new output chunk and checks for loop patterns.
// Returns nil if no loop detected, or a *LoopDetectedError if a loop is found.
//
// Algorithm:
//  1. Append chunk to accumulated (which holds optional trailing fragment).
//  2. Split by \n. If content ends with \n, all elements are complete lines
//     (last element is empty). If not, the last element is incomplete.
//  3. Process complete lines: trim, skip if short, increment map count.
//  4. Keep only the incomplete trailing line in accumulated for next call.
func (ld *LoopDetector) AddChunk(chunk string, timestamp time.Time) error {
	// Skip only truly empty chunks. Do NOT use TrimSpace here: the LLM may
	// send "\n" as a separate chunk token (independent SSE event), and
	// TrimSpace("\n") returns "" which would discard the line delimiter.
	if chunk == "" {
		return nil
	}

	// Step 1: Append chunk to accumulated and split by newlines.
	ld.accumulated.WriteString(chunk)
	content := ld.accumulated.String()
	lines := strings.Split(content, "\n")

	// Step 2: Determine how many split elements are complete lines.
	// - If content ends with \n, the last element is always empty, so all
	//   non-empty elements are complete lines.
	// - If content does NOT end with \n, the last element is an incomplete
	//   fragment that must be preserved for the next chunk.
	completeCount := len(lines)
	endsWithNewline := strings.HasSuffix(content, "\n")
	if !endsWithNewline && completeCount > 0 {
		completeCount = len(lines) - 1
	}

	// Step 3: Process complete lines.
	for i := 0; i < completeCount; i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		// Skip lines shorter than minLineLen
		if len(line) < ld.minLineLen {
			continue
		}

		// Increment line count
		ld.lineCounts[line]++
		count := ld.lineCounts[line]

		log.Debug("LoopDetector: line detected (count=%d, threshold=%d): %.60s...",
			count, ld.threshold, line)

		if count >= ld.threshold {
			log.Warn("LoopDetector: LOOP TRIGGERED: line repeated %d times (threshold=%d): %.60s...",
				count, ld.threshold, line)
			return &LoopDetectedError{
				pattern:     line,
				repeatCount: count,
				threshold:   ld.threshold,
				startTime:   timestamp,
				endTime:     timestamp,
				suggestion: "You appear to be repeating the same line of content. " +
					"Please review your output and take a different approach. " +
					"Consider summarizing your findings or moving to the next step.",
			}
		}
	}

	// Step 4: Reset accumulated and store only the incomplete trailing line.
	ld.accumulated.Reset()
	if !endsWithNewline && len(lines) > 0 {
		// The last fragment is incomplete — keep it for the next chunk.
		ld.accumulated.WriteString(lines[len(lines)-1])
	}

	return nil
}

// Reset clears the detector state. Must be called at the start of each
// LLM iteration to avoid cross-iteration false positives.
func (ld *LoopDetector) Reset() {
	ld.accumulated.Reset()
	ld.lineCounts = make(map[string]int)
	log.Debug("LoopDetector: reset line counts")
}

// LoopTempController manages automatic temperature adjustment when a loop is detected.
// It uses an oscillating strategy: increase temperature by StepUp until reaching Max,
// then decrease by StepDown until reaching Min, then repeat. Overflow carry-over ensures
// no temperature value is wasted — when a step would exceed the boundary, the excess
// amount is "bounced back" after flipping direction.
//
// Example (stepUp=0.05, stepDown=0.07, max=0.9, min=0.1):
//
//	0.50 → 0.55 → ... → 0.90 (hit max, flip ↓) → 0.85 → 0.78 → ... → 0.10 (hit min, flip ↑) → 0.17 → ...
//	At max: 0.90 + 0.05 = 0.95, overflow=0.05, flip ↓, new temp = 0.90 - 0.05 = 0.85
//	At min: 0.10 - 0.07 = 0.03, overflow=-0.07, flip ↑, new temp = 0.10 + 0.07 = 0.17
type LoopTempController struct {
	currentTemp float64 // current effective temperature
	direction   int     // +1 = increasing, -1 = decreasing
	stepUp      float64 // temperature increase step
	stepDown    float64 // temperature decrease step
	maxTemp     float64 // upper bound
	minTemp     float64 // lower bound
}

// NewLoopTempController creates a new temperature controller with the given parameters.
// initialTemp is the user-configured temperature value.
func NewLoopTempController(initialTemp, stepUp, stepDown, maxTemp, minTemp float64) *LoopTempController {
	if stepUp <= 0 {
		stepUp = 0.05
	}
	if stepDown <= 0 {
		stepDown = 0.07
	}
	if maxTemp <= minTemp {
		maxTemp = 0.9
		minTemp = 0.1
	}
	if initialTemp < minTemp {
		initialTemp = minTemp
	}
	if initialTemp > maxTemp {
		initialTemp = maxTemp
	}

	return &LoopTempController{
		currentTemp: initialTemp,
		direction:   1, // start by increasing
		stepUp:      stepUp,
		stepDown:    stepDown,
		maxTemp:     maxTemp,
		minTemp:     minTemp,
	}
}

// Apply calculates the next temperature value using the oscillating strategy
// with overflow carry-over. Returns the new temperature and whether it changed.
func (ltc *LoopTempController) Apply() (newTemp float64, changed bool) {
	step := ltc.stepUp
	if ltc.direction < 0 {
		step = ltc.stepDown
	}

	candidate := ltc.currentTemp + float64(ltc.direction)*step

	// Check overflow beyond max
	if ltc.direction > 0 && candidate > ltc.maxTemp {
		overflow := candidate - ltc.maxTemp
		ltc.direction = -1 // flip to decreasing
		candidate = ltc.maxTemp - overflow
		if candidate < ltc.minTemp {
			candidate = ltc.minTemp
		}
	}

	// Check overflow beyond min
	if ltc.direction < 0 && candidate < ltc.minTemp {
		overflow := ltc.minTemp - candidate
		ltc.direction = 1 // flip to increasing
		candidate = ltc.minTemp + overflow
		if candidate > ltc.maxTemp {
			candidate = ltc.maxTemp
		}
	}

	if candidate == ltc.currentTemp {
		return candidate, false
	}

	ltc.currentTemp = candidate
	return candidate, true
}

// Temperature returns the current temperature value.
func (ltc *LoopTempController) Temperature() float64 {
	return ltc.currentTemp
}

// truncateString truncates a string to maxLen characters.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ToolCallLoopDetector detects when the LLM repeatedly calls the same tool
// with the same arguments across iterations. Each unique (toolName + canonical args)
// combination is tracked until it reaches the threshold, at which point intervention
// is triggered.
//
// Key design:
//   - Arguments are canonicalized (re-marshaled) so key ordering and whitespace
//     differences don't produce false negatives.
//   - Prune() must be called after each iteration to remove keys that were NOT
//     present in the current iteration, breaking the consecutive count chain.
//   - This ensures only truly repeated patterns are caught, not tools that were
//     called legitimately in different contexts across iterations.
type ToolCallLoopDetector struct {
	threshold  int
	callCounts map[string]int // key = toolName + "|" + canonicalArgs → consecutive count
}

// ToolCallLoopDetectedError is returned when a tool call loop is detected.
type ToolCallLoopDetectedError struct {
	toolName    string
	args        string
	repeatCount int
	threshold   int
}

func (e *ToolCallLoopDetectedError) Error() string {
	return fmt.Sprintf(
		"tool call loop detected: tool %q called %d times with the same arguments (threshold: %d). "+
			"Please stop repeating the same tool call and try a different approach "+
			"such as using a different tool, breaking the problem into smaller steps, "+
			"or asking the user for more information.",
		e.toolName, e.repeatCount, e.threshold,
	)
}

// NewToolCallLoopDetector creates a new tool call loop detector.
// threshold: min consecutive occurrences to trigger (default: 5).
func NewToolCallLoopDetector(threshold int) *ToolCallLoopDetector {
	if threshold <= 0 {
		threshold = 5
	}
	log.Debug("ToolCallLoopDetector: created with threshold=%d", threshold)
	return &ToolCallLoopDetector{
		threshold:  threshold,
		callCounts: make(map[string]int),
	}
}

// canonicalizeJSON normalizes a JSON string by unmarshaling and re-marshaling.
// This eliminates differences in key ordering, whitespace, and indentation.
func canonicalizeJSON(raw string) string {
	var v interface{}
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		// If it's not valid JSON, return the raw string as-is.
		return raw
	}
	canonical, err := json.Marshal(v)
	if err != nil {
		return raw
	}
	return string(canonical)
}

// AddCall records a tool call and returns the error if the same call has
// been made >= threshold times consecutively.
func (tld *ToolCallLoopDetector) AddCall(name, args string) error {
	key := name + "|" + canonicalizeJSON(args)
	tld.callCounts[key]++
	count := tld.callCounts[key]

	log.Debug("ToolCallLoopDetector: tool=%q, key=%q, count=%d, threshold=%d",
		name, key, count, tld.threshold)

	if count >= tld.threshold {
		log.Warn("ToolCallLoopDetector: LOOP TRIGGERED: tool %q repeated %d times", name, count)
		return &ToolCallLoopDetectedError{
			toolName:    name,
			args:        args,
			repeatCount: count,
			threshold:   tld.threshold,
		}
	}
	return nil
}

// Prune removes keys that are NOT in the given set, breaking consecutive
// count chains for tools that were not called in the current iteration.
// Must be called after each iteration's tool calls have been processed.
func (tld *ToolCallLoopDetector) Prune(activeKeys map[string]bool) {
	for key := range tld.callCounts {
		if !activeKeys[key] {
			delete(tld.callCounts, key)
			log.Debug("ToolCallLoopDetector: pruned inactive key: %s", key)
		}
	}
}

// Reset clears all counts. Called when a loop is detected and feedback is sent.
func (tld *ToolCallLoopDetector) Reset() {
	tld.callCounts = make(map[string]int)
	log.Debug("ToolCallLoopDetector: reset all counts")
}

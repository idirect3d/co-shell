// Author: L.Shuang
// Created: 2026-05-13
// Last Modified: 2026-07-01
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
// IMPLIED, BUT NOT INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package agent

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	"github.com/idirect3d/co-shell/log"
)

// LoopDetector monitors LLM output for repeating line patterns that indicate
// the LLM is stuck in a loop. It uses a sliding-window period detection
// algorithm on the stream of completed lines.
//
// Algorithm (period detection):
//   - Completed lines are hashed (FNV-1a 64-bit) and stored in a ring buffer.
//   - For each new incoming line, the buffer tail is checked for periodic
//     repetition: for each candidate period p (1..bufferSize/threshold),
//     check whether the last (threshold * p) lines consist of the same
//     p-line segment repeated threshold times.
//   - Hash collisions are guarded by an actual string comparison when a
//     hash match is found.
//   - This correctly handles ALL periodic patterns: AAAA (p=1), ABAB (p=2),
//     ABCABC (p=3), ABACABAC (p=4), ABCDABCD (p=4), etc.
//   - Scattered or interleaved non-repeating lines do NOT form a valid period
//     and will not trigger (e.g. A B B A — no clean period).
//
// Single-line detection is delegated to an optional SingleLineLoopDetector
// sub-detector, checked in AddChunk after each completed line is pushed.
type LoopDetector struct {
	threshold          int                     // min period repetitions to trigger (default: 5)
	accumulated        strings.Builder         // accumulates incomplete trailing line across chunks
	lineHashes         []uint64                // ring buffer of completed line hashes
	lineTexts          []string                // ring buffer of completed line texts (for collision guard)
	writePos           int                     // ring buffer write position (overwrite after full)
	lineCount          int                     // total completed lines seen so far (for indexing)
	singleLineDetector *SingleLineLoopDetector // optional sub-detector for single-line patterns
}

// SetSingleLineDetector attaches a SingleLineLoopDetector sub-detector.
// When set, AddChunk checks each completed line against the sub-detector
// in addition to the multi-line period detection.
func (ld *LoopDetector) SetSingleLineDetector(sld *SingleLineLoopDetector) {
	ld.singleLineDetector = sld
}

// MaxPeriod is the maximum period length the detector will examine.
// Must be large enough to cover typical LLM output loop patterns,
// where repeated paragraphs can be 10-15 lines long. Buffer capacity
// is MaxPeriod * threshold (default: 20 * 5 = 100).
const MaxPeriod = 20

// LoopDetectedError is returned when a loop is detected.
type LoopDetectedError struct {
	pattern     string    // the repeated line content (from the first period segment)
	period      int       // detected period length (in lines)
	repeatCount int       // how many full periods were seen
	threshold   int       // detection threshold
	startTime   time.Time // first occurrence
	endTime     time.Time // last occurrence
	suggestion  string    // suggestion for the agent to correct
}

func (e *LoopDetectedError) Error() string {
	msg := fmt.Sprintf(
		"LLM output loop detected: period=%d lines, repeated %d times (threshold: %d). ",
		e.period, e.repeatCount, e.threshold,
	)

	if e.pattern != "" {
		msg += fmt.Sprintf("Sample content (first 200 chars): %s. ", truncateString(e.pattern, 200))
	}

	msg += e.suggestion
	return msg
}

// NewLoopDetector creates a new loop detector with the given configuration.
// threshold: min period repetitions to trigger (default: 5).
func NewLoopDetector(threshold int) *LoopDetector {
	if threshold <= 0 {
		threshold = 5
	}
	bufSize := threshold * MaxPeriod
	log.Debug("LoopDetector: created with threshold=%d, bufferSize=%d", threshold, bufSize)
	return &LoopDetector{
		threshold:  threshold,
		lineHashes: make([]uint64, bufSize),
		lineTexts:  make([]string, bufSize),
	}
}

// AddChunk adds a new output chunk and checks for loop patterns.
// Returns nil if no loop detected, or a *LoopDetectedError if a loop is found.
func (ld *LoopDetector) AddChunk(chunk string, timestamp time.Time) error {
	// Skip only truly empty chunks.
	if chunk == "" {
		return nil
	}

	// Step 1: Append chunk to accumulated and split by newlines.
	ld.accumulated.WriteString(chunk)
	content := ld.accumulated.String()
	lines := strings.Split(content, "\n")

	// Step 2: Determine how many split elements are complete lines.
	completeCount := len(lines)
	endsWithNewline := strings.HasSuffix(content, "\n")
	if !endsWithNewline && completeCount > 0 {
		completeCount = len(lines) - 1
	}

	// Step 3: Process complete lines using period detection.
	for i := 0; i < completeCount; i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		hash := hashLine(line)
		ld.pushLine(hash, line)

		// Check single-line patterns (long line, character-level period).
		if ld.singleLineDetector != nil {
			if event := ld.singleLineDetector.CheckLine(line); event != nil {
				return &LoopDetectedError{
					pattern:     event.Content,
					period:      0,
					repeatCount: ld.singleLineDetector.minRepeat,
					threshold:   ld.singleLineDetector.minRepeat,
					startTime:   timestamp,
					endTime:     timestamp,
					suggestion:  event.Suggestion,
				}
			}
		}

		if err := ld.checkLoop(timestamp); err != nil {
			return err
		}
	}

	// Step 4: Check the incomplete trailing line (no \n yet) for single-line
	// loop patterns. Without this, a very long line without newlines (e.g.
	// "ABCDEFGABCDEFG..." repeated 30K+ chars) would never be checked
	// because completeCount stays 0, so the completed-line loop above
	// never executes.
	if !endsWithNewline && len(lines) > 0 && ld.singleLineDetector != nil {
		partialLine := strings.TrimSpace(lines[len(lines)-1])
		if partialLine != "" {
			if event := ld.singleLineDetector.CheckLine(partialLine); event != nil {
				return &LoopDetectedError{
					pattern:     event.Content,
					period:      0,
					repeatCount: ld.singleLineDetector.minRepeat,
					threshold:   ld.singleLineDetector.minRepeat,
					startTime:   timestamp,
					endTime:     timestamp,
					suggestion:  event.Suggestion,
				}
			}
		}
	}

	// Step 5: Reset accumulated and store only the incomplete trailing line.
	ld.accumulated.Reset()
	if !endsWithNewline && len(lines) > 0 {
		ld.accumulated.WriteString(lines[len(lines)-1])
	}

	return nil
}

// Reset clears the detector state. Must be called at the start of each
// LLM iteration to avoid cross-iteration false positives.
func (ld *LoopDetector) Reset() {
	ld.accumulated.Reset()
	bufSize := len(ld.lineHashes)
	ld.lineHashes = make([]uint64, bufSize)
	ld.lineTexts = make([]string, bufSize)
	ld.writePos = 0
	ld.lineCount = 0
	log.Debug("LoopDetector: reset")
}

// pushLine pushes a (hash, text) pair onto the ring buffer at the current write position.
func (ld *LoopDetector) pushLine(hash uint64, text string) {
	bufSize := len(ld.lineHashes)
	if bufSize == 0 {
		return
	}
	ld.lineHashes[ld.writePos] = hash
	ld.lineTexts[ld.writePos] = text
	ld.writePos = (ld.writePos + 1) % bufSize
	ld.lineCount++
}

// lineAt returns the (hash, text) at the given absolute index (0-based from first recorded line).
// Returns hash=0, text="" if the index is out of range.
func (ld *LoopDetector) lineAt(idx int) (uint64, string) {
	bufSize := len(ld.lineHashes)
	if bufSize == 0 || idx < 0 || idx >= ld.lineCount {
		return 0, ""
	}
	// Ring buffer: oldest line starts at writePos, but only after buffer is full.
	oldest := 0
	if ld.lineCount > bufSize {
		oldest = ld.writePos
	}
	pos := (oldest + idx) % bufSize
	return ld.lineHashes[pos], ld.lineTexts[pos]
}

// countAtEnd checks whether the last N lines confirm a period p repeated k times
// at the tail of the stream. Returns the number of complete periods found.
//
// It takes the last (k*p) lines, divides into k segments of length p, and checks
// that all segments are identical (hash match + string match collision guard).
func (ld *LoopDetector) countAtEnd(p, k int) int {
	needLines := k * p
	if ld.lineCount < needLines {
		return 0
	}

	// Reference segment: the last p lines of the stream.
	refStart := ld.lineCount - p
	refHashes := make([]uint64, p)
	refTexts := make([]string, p)
	for i := 0; i < p; i++ {
		refHashes[i], refTexts[i] = ld.lineAt(refStart + i)
	}

	// Check (k-1) additional segments before the reference segment.
	for seg := 1; seg < k; seg++ {
		segStart := refStart - seg*p
		for i := 0; i < p; i++ {
			h, text := ld.lineAt(segStart + i)
			if h != refHashes[i] {
				return seg // this segment and all before it are incomplete
			}
			// Hash collision guard: confirm actual string equality
			if text != refTexts[i] {
				return seg // hash collision — treat as mismatch
			}
		}
	}
	return k // all k segments match
}

// checkLoop examines the buffer tail for any periodic pattern repeating
// >= threshold times.
func (ld *LoopDetector) checkLoop(timestamp time.Time) error {
	// If combined lines < threshold, impossible to detect a loop yet
	if ld.lineCount < ld.threshold {
		return nil
	}
	// Minimum period length is 1, maximum is up to lineCount/threshold or MaxPeriod
	maxP := ld.lineCount / ld.threshold
	if maxP > MaxPeriod {
		maxP = MaxPeriod
	}

	bufSize := len(ld.lineHashes)
	_ = bufSize

	for p := 1; p <= maxP; p++ {
		// How many full periods fit in the buffer tail? At least threshold.
		maxK := ld.lineCount / p
		if maxK > ld.threshold {
			maxK = ld.threshold
		}

		k := ld.countAtEnd(p, maxK)
		if k >= ld.threshold {
			_, sampleText := ld.lineAt(ld.lineCount - p)
			log.Warn("LoopDetector: LOOP TRIGGERED: period=%d, repeated %d times (threshold=%d): %.60s...",
				p, k, ld.threshold, sampleText)
			return &LoopDetectedError{
				pattern:     sampleText,
				period:      p,
				repeatCount: k,
				threshold:   ld.threshold,
				startTime:   timestamp,
				endTime:     timestamp,
				suggestion: "You appear to be repeating the same content pattern. " +
					"Please review your output and take a different approach. " +
					"Consider summarizing your findings or moving to the next step.",
			}
		}
	}
	return nil
}

// hashLine computes a FNV-1a 64-bit hash of the line content.
func hashLine(line string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(line))
	return h.Sum64()
}

// ToolCallLoopDetector detects when the LLM repeatedly calls the same tool
// with the same arguments across iterations. Only the most recent tool call
// key is tracked — when a different tool or argument combination arrives,
// the previous key's count is cleared. This ensures only truly consecutive
// identical (tool + args) patterns are detected.
//
// Key design:
//   - Arguments are canonicalized (re-marshaled) so key ordering and whitespace
//     differences don't produce false negatives.
//   - lastKey tracks the most recent (tool + canonicalArgs) seen.
//     If the incoming call has a different key, the previous key's count
//     is wiped and the new key starts at 1.
//   - This naturally handles multi-tool iterations: when iteration N calls
//     tool A and iteration N+1 calls tool B, A's count is cleared.
type ToolCallLoopDetector struct {
	threshold  int
	lastKey    string         // the most recent key seen
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
// When a different tool or argument combination arrives (key != lastKey),
// the previous key's count is cleared, so only truly consecutive identical
// (tool + args) patterns accumulate.
func (tld *ToolCallLoopDetector) AddCall(name, args string) error {
	key := name + "|" + canonicalizeJSON(args)

	// If this is a different tool/args from the most recently tracked one,
	// clear the previous key's count so it starts fresh if it reappears later.
	if tld.lastKey != "" && key != tld.lastKey {
		delete(tld.callCounts, tld.lastKey)
	}

	tld.lastKey = key
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

// truncateString truncates a string to maxLen characters.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// LoopEventType identifies which detector triggered a loop event.
type LoopEventType int

const (
	LoopEventContentPeriodic  LoopEventType = iota // 多行周期重复（LoopDetector）
	LoopEventContentDuplicate                      // 跨迭代内容重复（lastAssistantContent LCS）
	LoopEventSingleLineRepeat                      // 流内单行重复（SingleLineLoopDetector）
	LoopEventToolCallRepeat                        // 工具+参数重复（ToolCallLoopDetector）
)

// LoopEvent is the unified loop detection event from any detector.
// All three detectors produce a LoopEvent which is then handled by
// the single applyLoopIntervention() pipeline.
type LoopEvent struct {
	Type       LoopEventType // which detector triggered
	Detector   string        // detector name (for logging)
	Suggestion string        // feedback suggestion for the LLM
	Content    string        // suspected loop content (for judge model)
	ToolName   string        // tool name (only for ToolCallRepeat)
	ToolArgs   string        // tool arguments (only for ToolCallRepeat)
	Reason     string        // brief human-readable reason
}

// LoopJudgeResult holds the result of an LLM-based loop judgment call.
type LoopJudgeResult struct {
	IsLoop       bool   `json:"is_loop"`
	Reason       string `json:"reason"`
	ExitStrategy string `json:"exit_strategy"`
}

// SingleLineLoopDetector detects repeating patterns within the last N characters
// of a single output line. It uses the same period-detection algorithm as
// LoopDetector, but operates on characters instead of lines.
//
// Rules:
//
//	(a) Line length exceeds longLineThreshold (default 2048 chars) — immediate trigger
//	(b) Within the last windowSize chars (default 128), a periodic pattern
//	    repeats >= minRepeat times (fixed at 3). For example with windowSize=10:
//	    "AAAAAAAAAA" → period=1, 10 repetitions → trigger
//	    "ABCABCABCA" → period=3, 3 repetitions → trigger
//	    "ABAABAABAA" → period=3, 3 repetitions → trigger
//	    "ABCDEFGHIJ" → no period → no trigger
type SingleLineLoopDetector struct {
	longLineThreshold int // if a line exceeds this length, trigger (0 = disabled) (a)
	windowSize        int // window for character-level period detection (0 = disabled) (b)
	minRepeat         int // minimum period repetitions for rule (b), fixed at 3
}

// NewSingleLineLoopDetector creates a new SingleLineLoopDetector.
func NewSingleLineLoopDetector(longLineThreshold, windowSize int) *SingleLineLoopDetector {
	if longLineThreshold <= 0 {
		longLineThreshold = 2048
	}
	if windowSize <= 0 {
		windowSize = 128
	}
	log.Debug("SingleLineLoopDetector: created with longLineThreshold=%d, windowSize=%d, minRepeat=3",
		longLineThreshold, windowSize)
	return &SingleLineLoopDetector{
		longLineThreshold: longLineThreshold,
		windowSize:        windowSize,
		minRepeat:         3,
	}
}

// CheckLine checks if a single line triggers a loop detection event.
// Returns nil if no loop detected, or a *LoopEvent if a loop is found.
//
// Rule (a): line length exceeds threshold.
// Rule (b): last windowSize chars contain a period-p sequence repeated >= minRepeat times.
func (sld *SingleLineLoopDetector) CheckLine(line string) *LoopEvent {
	// Rule (a): line too long
	if sld.longLineThreshold > 0 && len(line) > sld.longLineThreshold {
		log.Warn("SingleLineLoopDetector: line length %d exceeds threshold %d",
			len(line), sld.longLineThreshold)
		return &LoopEvent{
			Type:     LoopEventSingleLineRepeat,
			Detector: "SingleLineLoopDetector (long line)",
			Content:  truncateString(line, 200),
			Reason:   fmt.Sprintf("single line length %d exceeds threshold %d", len(line), sld.longLineThreshold),
			Suggestion: "Your output contains an extremely long line. " +
				"Consider breaking it into shorter lines or summarizing.",
		}
	}

	// Rule (b): character-level period detection in the last windowSize chars
	if sld.windowSize > 0 && len(line) >= sld.windowSize {
		tail := line[len(line)-sld.windowSize:]

		// For each period p from 1 to windowSize / minRepeat:
		// Check that the last (minRepeat * p) chars consist of a p-char
		// segment repeated minRepeat times.
		maxP := len(tail) / sld.minRepeat
		for p := 1; p <= maxP; p++ {
			// Reference segment: the last p chars of tail
			ref := tail[len(tail)-p:]

			// Check additional (minRepeat-1) segments before the reference
			match := true
			for seg := 1; seg < sld.minRepeat; seg++ {
				segStart := len(tail) - (seg+1)*p
				segEnd := segStart + p
				if tail[segStart:segEnd] != ref {
					match = false
					break
				}
			}

			if match {
				log.Warn("SingleLineLoopDetector: character-level period %d repeated >= %d times at end of line",
					p, sld.minRepeat)
				return &LoopEvent{
					Type:     LoopEventSingleLineRepeat,
					Detector: "SingleLineLoopDetector (char period)",
					Content:  truncateString(line, 200),
					Reason:   fmt.Sprintf("char-level period %d repeated at end of line (window=%d)", p, sld.windowSize),
					Suggestion: "Your output contains a repeating character pattern. " +
						"Please vary your output.",
				}
			}
		}
	}

	return nil
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

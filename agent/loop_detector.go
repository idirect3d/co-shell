// Author: L.Shuang
// Created: 2026-05-13
// Last Modified: 2026-05-15
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
	"fmt"
	"regexp"
	"strings"
	"time"
)

// LoopDetector monitors LLM output for repeating patterns that indicate
// the LLM is stuck in a loop. It detects when the same structural pattern
// with incremental values (e.g., timestamps, counters) appears too frequently.
type LoopDetector struct {
	threshold    int                      // min occurrences of same pattern to trigger
	maxWindow    int                      // max chunks to keep in history
	accumulated  string                   // accumulated output content
	patterns     map[string]*PatternCount // detected patterns and their counts
	currentTotal int                      // total chunks added
	lastCheckLen int                      // length of content last checked

	// Content-level loop detection (FIX-190)
	// Detects when the same text block repeats consecutively in the accumulated content.
	// This catches cases like:
	//   - Same paragraph repeating: "I need to break out... I need to break out..."
	//   - URL with repeating encoded chars: "...%E7%9B%91%E7%9B%91%E7%9B%91..."
	//   - Same sentence repeating across lines
	// Uses a sliding window: compares the last N characters of accumulated content
	// with the N characters before them. If they match, it's a repeat.
	contentBlockSize int    // size of the text block to compare (auto-adjusted)
	contentRepeatCnt int    // how many consecutive repeats detected
	contentLastBlock string // the last detected repeating block
}

// PatternCount tracks occurrences of a specific pattern.
type PatternCount struct {
	Pattern     string      // normalized pattern (timestamps replaced with [TIME])
	PatternType string      // type of pattern (timestamp, counter, etc.)
	RawSample   string      // raw sample of the pattern for feedback
	Count       int         // number of occurrences
	Timestamps  []time.Time // timestamps of each occurrence
	Lines       []string    // original lines containing this pattern
}

// LoopDetectedError is returned when a loop is detected.
type LoopDetectedError struct {
	pattern     string // the repeated pattern
	patternType string // type of pattern (timestamp, counter, etc.)
	repeatCount int    // how many times it repeated
	windowSize  int    // sliding window size
	threshold   int    // detection threshold
	startTime   time.Time
	endTime     time.Time
	suggestion  string // suggestion for the agent to correct
}

func (e *LoopDetectedError) Error() string {
	msg := fmt.Sprintf(
		"LLM output loop detected: pattern '%s' repeated %d times (threshold: %d). ",
		truncateString(e.pattern, 200), e.repeatCount, e.threshold,
	)

	if e.patternType != "" {
		msg += fmt.Sprintf("Pattern type: %s. ", e.patternType)
	}

	msg += e.suggestion
	return msg
}

// NewLoopDetector creates a new loop detector with the given configuration.
func NewLoopDetector(threshold int, maxWindow int) *LoopDetector {
	if threshold <= 0 {
		threshold = 5
	}
	if maxWindow <= 0 {
		maxWindow = 256
	}
	return &LoopDetector{
		threshold: threshold,
		maxWindow: maxWindow,
		patterns:  make(map[string]*PatternCount),
	}
}

// AddChunk adds a new output chunk and checks for loop patterns.
// Returns nil if no loop detected, or a *LoopDetectedError if a loop is found.
func (ld *LoopDetector) AddChunk(chunk string, timestamp time.Time) error {
	chunk = strings.TrimSpace(chunk)
	if chunk == "" {
		return nil
	}

	ld.accumulated += chunk
	ld.currentTotal++

	// Only check when we have new content
	if len(ld.accumulated) <= ld.lastCheckLen {
		return nil
	}
	ld.lastCheckLen = len(ld.accumulated)

	// FIX-190: Content-level loop detection — check accumulated content for
	// repeating text blocks immediately on each chunk. This catches cases like:
	//   - Same paragraph repeating: "I need to break out... I need to break out..."
	//   - URL with repeating encoded chars: "...%E7%9B%91%E7%9B%91%E7%9B%91..."
	//   - Same sentence repeating across lines
	// The check is done on the accumulated content (not just the new chunk) to
	// catch patterns that span multiple chunks.
	if err := ld.checkContentLoop(timestamp); err != nil {
		return err
	}

	// Extract lines from the new chunk
	lines := extractLines(chunk)
	if len(lines) == 0 {
		return nil
	}

	// Check each line for loop patterns
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Try to identify the pattern type
		patternInfo := identifyPattern(line)
		if patternInfo == nil {
			continue
		}

		// Track this pattern
		key := patternInfo.normalized
		if _, exists := ld.patterns[key]; !exists {
			ld.patterns[key] = &PatternCount{
				Pattern: patternInfo.normalized,
			}
		}

		ld.patterns[key].Count++
		ld.patterns[key].PatternType = patternInfo.patternType
		ld.patterns[key].Timestamps = append(ld.patterns[key].Timestamps, timestamp)
		ld.patterns[key].Lines = append(ld.patterns[key].Lines, line)

		// Keep only recent entries (sliding window)
		if len(ld.patterns[key].Timestamps) > ld.maxWindow {
			ld.patterns[key].Timestamps = ld.patterns[key].Timestamps[1:]
			ld.patterns[key].Lines = ld.patterns[key].Lines[1:]
		}

		// Check if this pattern exceeds the threshold
		if ld.patterns[key].Count >= ld.threshold {
			return ld.createLoopError(key, timestamp)
		}
	}

	return nil
}

// checkContentLoop checks the accumulated content for repeating text blocks.
// It uses a sliding window approach: compares the last N characters of accumulated
// content with the N characters before them. If they match consecutively, it's a loop.
// The block size is auto-adjusted: starts with a minimum block size and grows
// to find the largest matching block.
func (ld *LoopDetector) checkContentLoop(timestamp time.Time) error {
	acc := ld.accumulated
	if len(acc) < 40 {
		return nil // need at least 2 blocks of minimum size
	}

	// Try different block sizes to find the repeating pattern.
	// Start with a minimum block size (20 chars) and try up to half the accumulated length.
	// This handles both short repeats (URL encoded chars) and long repeats (paragraphs).
	minBlock := 20
	maxBlock := len(acc) / 2
	if maxBlock > 500 {
		maxBlock = 500 // cap at 500 chars to avoid excessive computation
	}

	// Try block sizes from largest to smallest — larger blocks are more specific
	// and less likely to produce false positives.
	for blockSize := maxBlock; blockSize >= minBlock; blockSize-- {
		// We need at least 3 blocks worth of content to detect a repeat pattern
		if len(acc) < blockSize*3 {
			continue
		}

		// Get the last block (most recent content)
		lastBlock := acc[len(acc)-blockSize:]

		// Get the block before the last one
		prevBlock := acc[len(acc)-blockSize*2 : len(acc)-blockSize]

		// Check if they match (case-sensitive exact match)
		if lastBlock != prevBlock {
			continue
		}

		// Found a match! Check if this is a new pattern or continuation of previous.
		if ld.contentLastBlock == lastBlock {
			// Same block as before — increment repeat count
			ld.contentRepeatCnt++
		} else {
			// New repeating block — reset and start counting
			ld.contentRepeatCnt = 1
			ld.contentLastBlock = lastBlock
			ld.contentBlockSize = blockSize
		}

		// Check if we've exceeded the threshold
		if ld.contentRepeatCnt >= ld.threshold {
			// Build a descriptive pattern string for the error
			patternStr := fmt.Sprintf("content block repeat (%d chars): %s...",
				blockSize, truncateString(lastBlock, 100))

			// Create a temporary PatternCount for the error
			pc := &PatternCount{
				Pattern:     patternStr,
				PatternType: "content block repeat",
				Count:       ld.contentRepeatCnt,
				Timestamps:  []time.Time{timestamp},
			}
			ld.patterns[patternStr] = pc

			return ld.createLoopError(patternStr, timestamp)
		}

		// Found a match at this block size — no need to try smaller sizes
		// since we want the largest matching block
		return nil
	}

	// No match found at any block size — reset repeat counter if we had one
	if ld.contentRepeatCnt > 0 {
		ld.contentRepeatCnt = 0
		ld.contentLastBlock = ""
		ld.contentBlockSize = 0
	}

	return nil
}

// identifyPattern analyzes a line and returns pattern information if it matches
// a known loop pattern type (timestamp increment, counter, etc.).
func identifyPattern(line string) *struct {
	normalized  string
	patternType string
} {
	// Pattern 1: Timestamp increment pattern
	// Examples: "114: 2026-05-13 18:04:22 - ", "30: 在 2026-05-14 15:30:29 说："
	timestampPatterns := []string{
		`^[\d:]+ \d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}`,   // "114: 2026-05-13 18:04:22"
		`^[\d:]+ 在 \d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}`, // "30: 在 2026-05-14 15:30:29"
		`\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}`,           // bare timestamp
	}

	for _, pat := range timestampPatterns {
		re := regexp.MustCompile(pat)
		if re.MatchString(line) {
			// Normalize: replace timestamp with [TIME]
			normalized := normalizeTimestamps(line)
			return &struct {
				normalized  string
				patternType string
			}{
				normalized:  normalized,
				patternType: "timestamp increment",
			}
		}
	}

	// Pattern 2: Counter increment pattern
	// Examples: "Step 1: ", "Step 2: ", "Iteration 1", "Iteration 2"
	counterPattern := `^(.*[Cc]ount|Step|Iteration|Round)[\s:]*\d+`
	re := regexp.MustCompile(counterPattern)
	if re.MatchString(line) {
		normalized := re.ReplaceAllString(line, "${1}[NUM]")
		return &struct {
			normalized  string
			patternType string
		}{
			normalized:  normalized,
			patternType: "counter increment",
		}
	}

	// Pattern 3: Repeated prefix pattern
	// Lines that start with the same prefix followed by varying content
	// Examples: "Output: ", "Result: ", "Analysis: "
	prefixPattern := `^.{1,30}:[\s]`
	re = regexp.MustCompile(prefixPattern)
	if re.MatchString(line) && len(line) < 100 {
		// Extract the prefix (up to the first colon + space)
		idx := strings.Index(line, ":")
		if idx > 0 && idx < 30 {
			prefix := line[:idx+1]
			// Check if this prefix is common
			if isCommonPrefix(prefix) {
				normalized := prefix + "[CONTENT]"
				return &struct {
					normalized  string
					patternType string
				}{
					normalized:  normalized,
					patternType: "repeated prefix",
				}
			}
		}
	}

	// Pattern 4: Word/phrase repetition pattern
	// Detects when the same word or short phrase is repeated consecutively.
	// Examples: "oblivion oblivion oblivion oblivion", "test test test"
	// This catches LLM loops where the model repeats the same word many times.
	words := strings.Fields(line)
	if len(words) >= 4 {
		// Check if all words are the same (case-insensitive)
		firstWord := strings.ToLower(words[0])
		allSame := true
		for _, w := range words[1:] {
			if strings.ToLower(w) != firstWord {
				allSame = false
				break
			}
		}
		if allSame && firstWord != "" {
			normalized := firstWord + " [REPEATED]"
			return &struct {
				normalized  string
				patternType string
			}{
				normalized:  normalized,
				patternType: "word repetition",
			}
		}

		// Check for alternating two-word pattern (e.g., "A B A B A B")
		if len(words) >= 6 && len(words)%2 == 0 {
			pairA := strings.ToLower(words[0])
			pairB := strings.ToLower(words[1])
			if pairA != pairB {
				alternating := true
				for i := 0; i < len(words); i += 2 {
					if strings.ToLower(words[i]) != pairA || strings.ToLower(words[i+1]) != pairB {
						alternating = false
						break
					}
				}
				if alternating {
					normalized := pairA + " " + pairB + " [ALTERNATING]"
					return &struct {
						normalized  string
						patternType string
					}{
						normalized:  normalized,
						patternType: "word repetition",
					}
				}
			}
		}
	}

	return nil
}

// isCommonPrefix checks if a prefix is commonly used in normal output.
func isCommonPrefix(prefix string) bool {
	commonPrefixes := []string{
		"Output:", "Result:", "Analysis:", "Summary:", "Note:", "Info:",
		"Warning:", "Error:", "Debug:", "Step:", "Task:",
	}
	for _, cp := range commonPrefixes {
		if strings.HasPrefix(prefix, cp) {
			return true
		}
	}
	return false
}

// normalizeTimestamps replaces timestamps in a string with [TIME].
func normalizeTimestamps(s string) string {
	// Replace YYYY-MM-DD HH:MM:SS pattern
	re := regexp.MustCompile(`\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}`)
	return re.ReplaceAllString(s, "[TIME]")
}

// extractLines extracts non-empty lines from a string.
func extractLines(s string) []string {
	lines := strings.Split(s, "\n")
	var result []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

// createLoopError creates a LoopDetectedError with actionable feedback.
func (ld *LoopDetector) createLoopError(key string, timestamp time.Time) *LoopDetectedError {
	pattern := ld.patterns[key]

	// Generate suggestion based on pattern type
	suggestion := ""
	switch pattern.PatternType {
	case "timestamp increment":
		suggestion = "You appear to be generating content with incrementing timestamps. " +
			"This is likely a loop. Please review your output and stop repeating the same pattern with different timestamps. " +
			"Consider summarizing your findings instead of listing each item separately."
	case "counter increment":
		suggestion = "You appear to be generating content with incrementing counters. " +
			"This is likely a loop. Please review your output and stop repeating the same pattern with different counters. " +
			"Consider summarizing your findings instead of listing each step separately."
	case "content block repeat":
		suggestion = "You appear to be repeating the same text block consecutively. " +
			"This is likely a loop. Please review your output and stop repeating the same content. " +
			"Consider summarizing your findings or taking a different approach."
	default:
		suggestion = "You appear to be repeating the same content pattern. " +
			"Please review your output and take a different approach."
	}

	return &LoopDetectedError{
		pattern:     key,
		patternType: pattern.PatternType,
		repeatCount: pattern.Count,
		windowSize:  ld.maxWindow,
		threshold:   ld.threshold,
		startTime:   pattern.Timestamps[0],
		endTime:     timestamp,
		suggestion:  suggestion,
	}
}

// Reset clears the detector state.
func (ld *LoopDetector) Reset() {
	ld.accumulated = ""
	ld.patterns = make(map[string]*PatternCount)
	ld.currentTotal = 0
	ld.lastCheckLen = 0
	ld.contentBlockSize = 0
	ld.contentRepeatCnt = 0
	ld.contentLastBlock = ""
}

// truncateString truncates a string to maxLen characters.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

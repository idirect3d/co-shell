// Author: L.Shuang
// Created: 2026-05-13
// Last Modified: 2026-05-13
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
	"strings"
	"time"
)

// LoopDetector monitors LLM output for repeating patterns that indicate
// the LLM is stuck in a loop. It detects when the same structural pattern
// (e.g., "30: 在 [TIME] 说：") appears too frequently within a sliding window.
type LoopDetector struct {
	threshold    int // min occurrences of same pattern to trigger
	maxWindow    int // max chunks to keep in history
	patternFreq  map[string]int // frequency of each normalized pattern
	currentTotal int            // total chunks in window
}

// LoopDetectedError is returned when a loop is detected.
type LoopDetectedError struct {
	repeatedContent string
	repeatCount     int
	windowSize      int
	threshold       int
	startTime       time.Time
	endTime         time.Time
}

func (e *LoopDetectedError) Error() string {
	return fmt.Sprintf(
		"LLM output loop detected: structural pattern repeated %d times out of last %d chunks (threshold: %d). "+
			"The repeated pattern: %q. "+
			"Please review your actions and consider a different approach.",
		e.repeatCount, e.windowSize, e.threshold, truncateString(e.repeatedContent, 300),
	)
}

// NewLoopDetector creates a new loop detector with the given configuration.
func NewLoopDetector(threshold int, maxWindow int) *LoopDetector {
	if threshold <= 0 {
		threshold = 5
	}
	if maxWindow <= 0 {
		maxWindow = 20
	}
	return &LoopDetector{
		threshold:   threshold,
		maxWindow:   maxWindow,
		patternFreq: make(map[string]int),
	}
}

// AddChunk adds a new output chunk and checks for loop patterns.
// Returns nil if no loop detected, or a *LoopDetectedError if a loop is found.
//
// This uses a pattern-frequency approach: it normalizes each chunk,
// counts the frequency of each normalized pattern in the sliding window,
// and triggers when any single pattern exceeds the threshold count.
func (ld *LoopDetector) AddChunk(chunk string, timestamp time.Time) error {
	// Skip very short chunks (< 20 chars) to avoid false positives
	// Short content like "40: 在 [TIME] 说：" is likely a message prefix, not real loop content
	if len(strings.TrimSpace(chunk)) < 20 {
		return nil
	}

	// Normalize the chunk for pattern comparison
	normalized := normalizeChunk(chunk)

	// Skip empty chunks
	if normalized == "" {
		return nil
	}

	// Add to frequency map
	ld.patternFreq[normalized]++
	ld.currentTotal++

	// Trim old patterns if window is full
	if ld.currentTotal > ld.maxWindow {
		ld.pruneOldest()
	}

	// Check if any pattern exceeds the threshold
	for pattern, count := range ld.patternFreq {
		if count >= ld.threshold {
			return &LoopDetectedError{
				repeatedContent: pattern,
				repeatCount:     count,
				windowSize:      ld.maxWindow,
				threshold:       ld.threshold,
				startTime:       timestamp.Add(-time.Duration(ld.currentTotal) * 2 * time.Second), // approximate
				endTime:         timestamp,
			}
		}
	}

	return nil
}

// Reset clears the detector state.
func (ld *LoopDetector) Reset() {
	ld.patternFreq = make(map[string]int)
	ld.currentTotal = 0
}

// pruneOldest simulates removing the oldest chunk from the sliding window.
// Since we process chunks in order, we approximate pruning by decrementing
// a random pattern's count when the window is full.
func (ld *LoopDetector) pruneOldest() {
	// Find a pattern with count > 1 to decrement
	// This approximates removing an old chunk
	for pattern := range ld.patternFreq {
		ld.patternFreq[pattern]--
		if ld.patternFreq[pattern] <= 0 {
			delete(ld.patternFreq, pattern)
		}
		break
	}
	ld.currentTotal--
}

// normalizeChunk normalizes a chunk for pattern comparison by:
// 1. Trimming whitespace
// 2. Removing/normalizing timestamps
// 3. Truncating to a fixed size for consistent comparison
func normalizeChunk(s string) string {
	// Trim whitespace
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	// Replace timestamp patterns with placeholder
	result := replaceTimestamps(s)

	// Take a fixed-size representative portion
	// This ensures "30: 在 [TIME] 说：" is always the same length
	if len(result) > 100 {
		result = result[:100]
	}

	return result
}

// replaceTimestamps replaces timestamp-like patterns with normalized placeholders.
func replaceTimestamps(s string) string {
	var result strings.Builder
	runes := []rune(s)
	i := 0

	for i < len(runes) {
		if isTimestampStart(runes, i) {
			result.WriteString("[TIME]")
			skipped := skipTimestamp(runes, i)
			i += skipped
			continue
		}
		result.WriteRune(runes[i])
		i++
	}

	return result.String()
}

// isTimestampStart checks if position i is the start of a timestamp pattern.
func isTimestampStart(runes []rune, start int) bool {
	if start+19 > len(runes) {
		return false
	}
	if !isDigit(runes[start]) || !isDigit(runes[start+1]) ||
		!isDigit(runes[start+2]) || !isDigit(runes[start+3]) ||
		runes[start+4] != '-' || !isDigit(runes[start+5]) ||
		!isDigit(runes[start+6]) || runes[start+7] != '-' ||
		!isDigit(runes[start+8]) || !isDigit(runes[start+9]) ||
		runes[start+10] != ' ' || !isDigit(runes[start+11]) ||
		!isDigit(runes[start+12]) || runes[start+13] != ':' ||
		!isDigit(runes[start+14]) || !isDigit(runes[start+15]) ||
		runes[start+16] != ':' || !isDigit(runes[start+17]) ||
		!isDigit(runes[start+18]) {
		return false
	}
	return true
}

// skipTimestamp skips past a timestamp pattern starting at position i.
func skipTimestamp(runes []rune, start int) int {
	if start+19 <= len(runes) {
		return 19
	}
	for i := start; i < len(runes) && i < start+20; i++ {
		if !isDigit(runes[i]) && runes[i] != '-' && runes[i] != ':' && runes[i] != ' ' {
			return i - start
		}
	}
	return 20
}

// isDigit checks if a rune is a digit.
func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

// truncateString truncates a string to maxLen characters.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
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
	"strings"
	"time"
)

// LoopDetector monitors LLM output for repeating patterns that indicate
// the LLM is stuck in a loop. It detects when the same structural pattern
// (e.g., "114: 2026-05-13 18:04:22 - 114: 2026-05-13 18:04:24 - ...")
// appears too frequently within an accumulating output.
type LoopDetector struct {
	threshold     int             // min occurrences of same pattern to trigger
	maxWindow     int             // max chars to keep in sliding window
	accumulated   string          // accumulated output content
	windowStart   int             // start position of sliding window in accumulated
	patternFreq   map[string]int  // frequency of each normalized pattern
	patternStart  map[string]int  // start position of each pattern occurrence
	currentTotal  int             // total chunks added
	lastCheckLen  int             // length of content last checked (for detecting new content)
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
		"LLM output loop detected: structural pattern repeated %d times in recent %d chars (threshold: %d). "+
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
		maxWindow = 256 // ~256KB sliding window
	}
	return &LoopDetector{
		threshold:    threshold,
		maxWindow:    maxWindow,
		patternFreq:  make(map[string]int),
		patternStart: make(map[string]int),
	}
}

// AddChunk adds a new output chunk and checks for loop patterns.
// Returns nil if no loop detected, or a *LoopDetectedError if a loop is found.
//
// This approach:
// 1. Accumulates all output chunks
// 2. Maintains a sliding window over the accumulated content
// 3. Normalizes the window content (replace timestamps with [TIME])
// 4. Splits the normalized content into segments and checks for repetition
func (ld *LoopDetector) AddChunk(chunk string, timestamp time.Time) error {
	// Skip empty chunks
	chunk = strings.TrimSpace(chunk)
	if chunk == "" {
		return nil
	}

	// Append new content to accumulated buffer
	ld.accumulated += chunk
	ld.currentTotal++

	// Only check when we have new content since last check
	if len(ld.accumulated) <= ld.lastCheckLen {
		return nil
	}
	ld.lastCheckLen = len(ld.accumulated)

	// Maintain sliding window
	windowStart := len(ld.accumulated) - ld.maxWindow
	if windowStart < 0 {
		windowStart = 0
	}

	// Get current window content
	windowContent := ld.accumulated[windowStart:]

	// Normalize the window content for pattern comparison
	normalized := normalizeChunk(windowContent)
	if normalized == "" {
		return nil
	}

	// Clear old pattern frequencies
	ld.patternFreq = make(map[string]int)
	ld.patternStart = make(map[string]int)

	// Split normalized content into segments for pattern detection
	// Use a segment size that captures the repeating pattern
	segmentSize := 50 // characters per segment
	if len(normalized) < segmentSize*ld.threshold {
		// Not enough content to detect loops yet
		return nil
	}

	// Analyze segments for repetition patterns
	// Split by common delimiters or use fixed-size segments
	segments := ld.extractSegments(normalized)

	// Count frequency of each segment pattern
	for _, seg := range segments {
		ld.patternFreq[seg]++
	}

	// Check if any pattern exceeds the threshold
	for pattern, count := range ld.patternFreq {
		if count >= ld.threshold {
			return &LoopDetectedError{
				repeatedContent: pattern,
				repeatCount:     count,
				windowSize:      ld.maxWindow,
				threshold:       ld.threshold,
				startTime:       timestamp.Add(-time.Duration(ld.currentTotal) * time.Second),
				endTime:         timestamp,
			}
		}
	}

	// Update window start position
	ld.windowStart = windowStart

	return nil
}

// extractSegments splits normalized content into overlapping segments for pattern detection.
// Uses overlapping segments to catch patterns that span segment boundaries.
func (ld *LoopDetector) extractSegments(content string) []string {
	if content == "" {
		return nil
	}

	var segments []string
	segmentSize := 50
	overlap := 10
	step := segmentSize - overlap

	// Split by lines first to handle multi-line content
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// For each line, extract fixed-size segments
		for i := 0; i < len(line); i += step {
			end := i + segmentSize
			if end > len(line) {
				end = len(line)
			}
			if i > 0 && end <= len(line) {
				// Overlap with previous segment
				seg := line[i-overlap : end]
				segments = append(segments, seg)
			} else {
				seg := line[i:end]
				segments = append(segments, seg)
			}
		}
	}

	// Also check for line-level patterns (important for timestamp-based loops)
	// Extract lines that match common patterns
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			segments = append(segments, line)
		}
	}

	return segments
}

// Reset clears the detector state.
func (ld *LoopDetector) Reset() {
	ld.accumulated = ""
	ld.windowStart = 0
	ld.patternFreq = make(map[string]int)
	ld.patternStart = make(map[string]int)
	ld.currentTotal = 0
	ld.lastCheckLen = 0
}

// normalizeChunk normalizes content for pattern comparison by:
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
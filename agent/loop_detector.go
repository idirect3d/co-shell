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
	"fmt"
	"strings"
	"time"
)

// LoopDetector monitors LLM output for repeating lines that indicate
// the LLM is stuck in a loop. It counts exact line repetitions and
// triggers intervention when the same line appears too many times.
//
// Key design:
//   - Lines shorter than minLineLen are ignored (filters XML tags, short prompts)
//   - Each AddChunk call appends to accumulated content and extracts complete lines
//   - cross-chunk line buffering via lineBuf ensures complete lines even when
//     a line break spans two chunks
//   - Reset() must be called at the start of each LLM iteration to clear counts
type LoopDetector struct {
	threshold   int             // min line occurrences to trigger (default: 5)
	minLineLen  int             // lines shorter than this are ignored (default: 50)
	accumulated strings.Builder // accumulated content for line extraction
	lineBuf     string          // buffer for incomplete line across chunks
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
//  1. Append chunk to accumulated content
//  2. Split accumulated content into lines, using lineBuf to assemble
//     incomplete lines that may span across chunks
//  3. For each complete line:
//     a. Skip if len(line) < minLineLen
//     b. Increment lineCounts[line]
//     c. If count >= threshold, return *LoopDetectedError
//  4. If the accumulated content ends with an incomplete line, keep it
//     in lineBuf for the next chunk
func (ld *LoopDetector) AddChunk(chunk string, timestamp time.Time) error {
	chunk = strings.TrimSpace(chunk)
	if chunk == "" {
		return nil
	}

	// Step 1: Append chunk to accumulated content (for line extraction).
	// We prefix with the pending line buffer so the chunk is always complete.
	combined := ld.lineBuf + chunk
	ld.lineBuf = "" // consumed

	// Step 2: Split into lines. Since cross-chunk lines are already assembled
	// via lineBuf, the combined string should be clean. However, the very last
	// line may still be incomplete if no newline follows.
	lines := strings.Split(combined, "\n")

	// Step 3: Process all complete lines (all but the last).
	// Only the last line may be incomplete — it goes back into lineBuf.
	for i, line := range lines {
		if i == len(lines)-1 {
			// Last fragment: may be incomplete if no trailing newline.
			// Keep it in lineBuf for next chunk.
			ld.lineBuf = line
			continue
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Skip lines shorter than minLineLen
		if len(line) < ld.minLineLen {
			continue
		}

		// Increment line count
		ld.lineCounts[line]++
		if ld.lineCounts[line] >= ld.threshold {
			// Capture first occurrence time approximately
			return &LoopDetectedError{
				pattern:     line,
				repeatCount: ld.lineCounts[line],
				threshold:   ld.threshold,
				startTime:   timestamp,
				endTime:     timestamp,
				suggestion: "You appear to be repeating the same line of content. " +
					"Please review your output and take a different approach. " +
					"Consider summarizing your findings or moving to the next step.",
			}
		}
	}

	return nil
}

// Reset clears the detector state. Must be called at the start of each
// LLM iteration to avoid cross-iteration false positives.
func (ld *LoopDetector) Reset() {
	ld.accumulated.Reset()
	ld.lineBuf = ""
	ld.lineCounts = make(map[string]int)
}

// truncateString truncates a string to maxLen characters.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

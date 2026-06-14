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

// truncateString truncates a string to maxLen characters.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

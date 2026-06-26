// Author: L.Shuang
// Created: 2026-06-25
// Last Modified: 2026-06-25
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
)

// DiffLine represents a single line in the diff output.
type DiffLine struct {
	Type    DiffLineType // +1=added, -1=removed, 0=unchanged
	Content string
}

// DiffLineType indicates whether a line was added, removed, or unchanged.
type DiffLineType int

const (
	DiffUnchanged DiffLineType = 0
	DiffAdded     DiffLineType = 1
	DiffRemoved   DiffLineType = -1
)

// DiffResult holds the complete diff output between two texts.
type DiffResult struct {
	Lines   []DiffLine // all lines with their diff status
	Added   int        // count of added lines (in B but not in A)
	Removed int        // count of removed lines (in A but not in B)
	Matched int        // count of matched lines
	TotalA  int        // total lines in text A
	TotalB  int        // total lines in text B
}

// Similarity returns the similarity ratio (0.0 - 1.0) between the two texts.
// 1.0 = identical, 0.0 = completely different.
// Uses matched lines / max(total lines) as the base metric.
func (r *DiffResult) Similarity() float64 {
	maxTotal := r.TotalA
	if r.TotalB > maxTotal {
		maxTotal = r.TotalB
	}
	if maxTotal == 0 {
		return 1.0 // both empty = identical
	}
	return float64(r.Matched) / float64(maxTotal)
}

// Diff returns a summary string of the differences.
func (r *DiffResult) Summary() string {
	return fmt.Sprintf("matched=%d/%d, added=%d, removed=%d, similarity=%.2f%%",
		r.Matched, max(r.TotalA, r.TotalB), r.Added, r.Removed, r.Similarity()*100)
}

// String returns a formatted diff output (like unified diff).
func (r *DiffResult) String() string {
	var sb strings.Builder
	for _, line := range r.Lines {
		switch line.Type {
		case DiffAdded:
			sb.WriteString("+ " + line.Content + "\n")
		case DiffRemoved:
			sb.WriteString("- " + line.Content + "\n")
		default:
			sb.WriteString("  " + line.Content + "\n")
		}
	}
	return sb.String()
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ComputeDiff performs a line-based diff between two texts using an LCS-based approach.
// It splits both texts into lines, computes the longest common subsequence (LCS),
// and produces a diff result with added/removed/unchanged line annotations.
//
// Parameters:
//   - textA: the original text (previous version)
//   - textB: the new text (current version)
//
// Returns a DiffResult containing all diff lines and statistics.
func ComputeDiff(textA, textB string) *DiffResult {
	linesA := splitLines(textA)
	linesB := splitLines(textB)

	result := &DiffResult{
		TotalA: len(linesA),
		TotalB: len(linesB),
	}

	if len(linesA) == 0 && len(linesB) == 0 {
		return result
	}

	// Build weighted LCS table for line-level matching.
	// Two lines match if they are identical after trimming.
	table := buildLCSTable(linesA, linesB)

	// Backtrack through the LCS table to produce the diff.
	lcsCount := 0
	i, j := len(linesA), len(linesB)
	var diffLines []DiffLine

	for i > 0 || j > 0 {
		if i > 0 && j > 0 && strings.TrimSpace(linesA[i-1]) == strings.TrimSpace(linesB[j-1]) {
			// Lines match — unchanged
			diffLines = append(diffLines, DiffLine{Type: DiffUnchanged, Content: linesA[i-1]})
			lcsCount++
			i--
			j--
		} else if j > 0 && (i == 0 || table[i][j-1] >= table[i-1][j]) {
			// Line added (in B but not in A)
			diffLines = append(diffLines, DiffLine{Type: DiffAdded, Content: linesB[j-1]})
			result.Added++
			j--
		} else if i > 0 {
			// Line removed (in A but not in B)
			diffLines = append(diffLines, DiffLine{Type: DiffRemoved, Content: linesA[i-1]})
			result.Removed++
			i--
		}
	}

	result.Matched = lcsCount

	// Reverse the diff lines (we built them backwards)
	for left, right := 0, len(diffLines)-1; left < right; left, right = left+1, right-1 {
		diffLines[left], diffLines[right] = diffLines[right], diffLines[left]
	}
	result.Lines = diffLines

	return result
}

// splitLines splits a text into lines, preserving empty lines.
func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	// Use strings.Split to preserve empty lines
	lines := strings.Split(text, "\n")
	// Filter out trailing empty line if the text ends with \n
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// buildLCSTable builds a standard LCS dynamic programming table.
// table[i][j] = length of LCS for linesA[:i] and linesB[:j].
// Two lines are considered matching if they are identical after trimming.
func buildLCSTable(linesA, linesB []string) [][]int {
	rows := len(linesA) + 1
	cols := len(linesB) + 1
	table := make([][]int, rows)
	for i := range table {
		table[i] = make([]int, cols)
	}

	for i := 1; i < rows; i++ {
		for j := 1; j < cols; j++ {
			if strings.TrimSpace(linesA[i-1]) == strings.TrimSpace(linesB[j-1]) {
				table[i][j] = table[i-1][j-1] + 1
			} else {
				if table[i-1][j] >= table[i][j-1] {
					table[i][j] = table[i-1][j]
				} else {
					table[i][j] = table[i][j-1]
				}
			}
		}
	}

	return table
}

// IsDuplicateContent checks whether newContent is a duplicate of previousContent
// based on the given similarity threshold (0.0-1.0).
// A similarity >= threshold is considered a duplicate.
// This is the main entry point for duplicate content detection in the agent loop.
func IsDuplicateContent(previousContent, newContent string, threshold float64) (bool, float64) {
	result := ComputeDiff(previousContent, newContent)
	similarity := result.Similarity()
	return similarity >= threshold, similarity
}

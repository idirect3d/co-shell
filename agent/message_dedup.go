// Author: L.Shuang
// Created: 2026-05-14
// Last Modified: 2026-05-14
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
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/idirect3d/co-shell/llm"
	"github.com/idirect3d/co-shell/log"
)

// MessageDedup monitors message history for duplicate content.
// It uses a two-stage approach:
// 1. Feature word extraction and ordered matching
// 2. Jaccard similarity calculation for confirmed matches
type MessageDedup struct {
	mu                   sync.Mutex
	enabled              bool
	featureRatio         float64 // ratio of words to extract as features
	matchRatio           float64 // minimum feature match ratio to proceed to stage 2
	similarityThreshold  int     // Jaccard similarity threshold (0-100)
	maxHistory           int     // max recent messages to check
	repeatLimit          int     // duplicate count trigger
	repeatCount          int     // current consecutive duplicate count
	lastDuplicateContent string  // content of the last detected duplicate
	lastDuplicateTime    time.Time
}

// DuplicateEvent is returned when a duplicate is detected.
type DuplicateEvent struct {
	NewContent    string
	DuplicateWith string
	Similarity    float64
	Count         int
	Timestamp     time.Time
}

// NewMessageDedup creates a new message deduplication checker.
func NewMessageDedup(enabled bool, featureRatio float64, matchRatio float64,
	similarityThreshold, maxHistory, repeatLimit int) *MessageDedup {
	if featureRatio <= 0 {
		featureRatio = 0.2
	}
	if matchRatio <= 0 || matchRatio > 1 {
		matchRatio = 0.6
	}
	if similarityThreshold <= 0 || similarityThreshold > 100 {
		similarityThreshold = 85
	}
	if maxHistory <= 0 {
		maxHistory = 50
	}
	if repeatLimit <= 0 {
		repeatLimit = 3
	}

	return &MessageDedup{
		enabled:              enabled,
		featureRatio:         featureRatio,
		matchRatio:           matchRatio,
		similarityThreshold:  similarityThreshold,
		maxHistory:           maxHistory,
		repeatLimit:          repeatLimit,
		repeatCount:          0,
		lastDuplicateContent: "",
	}
}

// CheckAndRecord checks if the new message is a duplicate of any recent message.
// Returns a DuplicateEvent if a duplicate is found and the repeat count reaches the limit.
// Returns nil if no duplicate or if below the threshold.
// Must be called BEFORE adding the message to history.
func (md *MessageDedup) CheckAndRecord(messages []llm.Message, newMsg llm.Message) *DuplicateEvent {
	md.mu.Lock()
	defer md.mu.Unlock()

	if !md.enabled {
		return nil
	}

	content := stripTimestampPrefix(newMsg.Content)
	if content == "" {
		return nil
	}

	// Get recent messages to check against
	recentMsgs := md.getRecentAssistantMessages(messages)
	if len(recentMsgs) == 0 {
		// No history to compare against, reset counter
		md.resetCount()
		return nil
	}

	// Stage 1: Feature word extraction and ordered matching
	features := extractFeatureWords(content, md.featureRatio)
	if len(features) == 0 {
		md.resetCount()
		return nil
	}

	// Convert to string slice for matching
	featureStrs := make([]string, len(features))
	for i, f := range features {
		featureStrs[i] = f.str
	}

	// Search for ordered feature matches in recent messages
	bestMatch := md.findBestFeatureMatch(recentMsgs, featureStrs)
	if bestMatch == nil || bestMatch.ratio < md.matchRatio {
		// Not enough feature matches, reset counter
		md.resetCount()
		return nil
	}

	// Stage 2: Full similarity calculation
	similarity := jaccardSimilarity(content, bestMatch.content)
	simPercent := int(similarity * 100)

	if simPercent < md.similarityThreshold {
		// Not similar enough, reset counter
		md.resetCount()
		return nil
	}

	// Duplicate found!
	md.repeatCount++
	md.lastDuplicateContent = bestMatch.content
	md.lastDuplicateTime = time.Now()

	log.Info("MessageDedup: duplicate detected (count=%d, similarity=%d%%)",
		md.repeatCount, simPercent)

	// Only return event when repeat count reaches limit
	if md.repeatCount >= md.repeatLimit {
		return &DuplicateEvent{
			NewContent:    content,
			DuplicateWith: bestMatch.content,
			Similarity:    similarity,
			Count:         md.repeatCount,
			Timestamp:     time.Now(),
		}
	}

	return nil
}

// GetRepeatCount returns the current duplicate count.
func (md *MessageDedup) GetRepeatCount() int {
	md.mu.Lock()
	defer md.mu.Unlock()
	return md.repeatCount
}

// GetLastDuplicateInfo returns information about the last detected duplicate.
func (md *MessageDedup) GetLastDuplicateInfo() (string, time.Time) {
	md.mu.Lock()
	defer md.mu.Unlock()
	return md.lastDuplicateContent, md.lastDuplicateTime
}

// resetCount resets the duplicate counter and clears state.
func (md *MessageDedup) resetCount() {
	if md.repeatCount > 0 {
		log.Info("MessageDedup: duplicate counter reset (was %d)", md.repeatCount)
	}
	md.repeatCount = 0
	md.lastDuplicateContent = ""
	md.lastDuplicateTime = time.Time{}
}

// getRecentAssistantMessages returns the most recent assistant messages.
func (md *MessageDedup) getRecentAssistantMessages(messages []llm.Message) []messageWithContent {
	var recent []messageWithContent
	startIdx := len(messages) - md.maxHistory
	if startIdx < 0 {
		startIdx = 0
	}

	for i := startIdx; i < len(messages); i++ {
		if messages[i].Role == "assistant" && messages[i].Content != "" {
			recent = append(recent, messageWithContent{
				content: messages[i].Content,
				time:    time.Now(), // approximate
			})
		}
	}

	return recent
}

// featureMatch represents a matched feature sequence in a message.
type featureMatch struct {
	content  string
	startIdx int
	endIdx   int
	ratio    float64
}

// messageWithContent wraps message content with metadata.
type messageWithContent struct {
	content string
	time    time.Time
}

// findBestFeatureMatch finds the best ordered feature match in recent messages.
func (md *MessageDedup) findBestFeatureMatch(recent []messageWithContent, features []string) *featureMatch {
	var best *featureMatch
	bestRatio := 0.0

	for _, msg := range recent {
		content := stripTimestampPrefix(msg.content)
		if content == "" {
			continue
		}

		// Find ordered matches of features in content
		matched := findOrderedMatches(content, features)
		if len(matched) == 0 {
			continue
		}

		ratio := float64(len(matched)) / float64(len(features))
		if ratio > bestRatio {
			bestRatio = ratio
			best = &featureMatch{
				content:  content,
				startIdx: 0,
				endIdx:   len(content),
				ratio:    ratio,
			}
		}
	}

	return best
}

// findOrderedMatches finds the longest subsequence of features that appear
// in order within the content.
func findOrderedMatches(content string, features []string) []string {
	var matched []string
	lastPos := -1
	contentLen := len(content)

	for _, feature := range features {
		// Guard: if lastPos is at or past the end, no more matches possible
		if lastPos+1 >= contentLen {
			break
		}
		// Find this feature after the last matched position
		pos := strings.Index(content[lastPos+1:], feature)
		if pos >= 0 {
			matched = append(matched, feature)
			lastPos = lastPos + 1 + pos + len(feature)
		}
	}

	return matched
}

// extractFeatureWords extracts feature words from content.
// Uses a hybrid approach: random sampling + meaningful word extraction.
func extractFeatureWords(content string, ratio float64) []stringWithPos {
	// Clean content (remove timestamp prefix)
	content = stripTimestampPrefix(content)
	if content == "" {
		return nil
	}

	// Tokenize: split into meaningful units
	words := tokenize(content)
	if len(words) == 0 {
		return nil
	}

	// Calculate how many features to extract
	numFeatures := int(float64(len(words)) * ratio)
	if numFeatures < 1 {
		numFeatures = 1
	}
	if numFeatures > len(words) {
		numFeatures = len(words)
	}

	// Shuffle words using Fisher-Yates for random sampling
	shuffled := make([]stringWithPos, len(words))
	copy(shuffled, words)
	rand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})

	// Take first numFeatures from shuffled
	return shuffled[:numFeatures]
}

// tokenize splits content into meaningful word-like units.
func tokenize(content string) []stringWithPos {
	var result []stringWithPos

	runes := []rune(content)
	i := 0

	for i < len(runes) {
		// Skip whitespace and punctuation
		if isWhitespace(runes[i]) {
			i++
			continue
		}

		// Check for CJK characters (Chinese, Japanese, Korean)
		if isCJK(runes[i]) {
			// Extract a sequence of CJK characters
			start := i
			for i < len(runes) && isCJK(runes[i]) {
				i++
			}
			segment := string(runes[start:i])
			// Split into 2-character chunks for better matching
			for j := 0; j < len(segment)-1; j += 2 {
				if j+2 <= len(segment) {
					result = append(result, stringWithPos{
						str: segment[j : j+2],
						pos: start + j,
						len: 2,
					})
				}
			}
			continue
		}

		// Check for alphanumeric characters
		if isAlphaNumeric(runes[i]) {
			start := i
			for i < len(runes) && isAlphaNumeric(runes[i]) {
				i++
			}
			result = append(result, stringWithPos{
				str: string(runes[start:i]),
				pos: start,
				len: i - start,
			})
			continue
		}

		// Skip other punctuation
		i++
	}

	return result
}

// stringWithPos represents a tokenized string with its position.
type stringWithPos struct {
	str string
	pos int
	len int
}

// stripTimestampPrefix removes timestamp prefixes from message content.
// Handles formats like:
// - "在 2026-05-13 19:32:54 说：..."
// - "2026-05-13 19:32:54 - ..."
// - "30: 在 2026-05-13 19:32:54 说：..."
func stripTimestampPrefix(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}

	// Try to match "在 YYYY-MM-DD HH:MM:SS 说：" pattern
	if strings.HasPrefix(content, "在") {
		// Find the timestamp
		idx := strings.Index(content, "说")
		if idx > 0 {
			return strings.TrimSpace(content[idx+1:])
		}
	}

	// Try to match "YYYY-MM-DD HH:MM:SS - " pattern
	if len(content) >= 19 && isTimestampAt(content, 0) {
		// Skip past timestamp and separator
		rest := content[19:]
		rest = strings.TrimLeft(rest, " -:")
		return strings.TrimSpace(rest)
	}

	// Try to match "N: YYYY-MM-DD HH:MM:SS - " pattern
	if idx := strings.Index(content, "在"); idx > 0 {
		return stripTimestampPrefix(content[idx:])
	}

	return content
}

// isWhitespace checks if a rune is whitespace.
func isWhitespace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

// isDigit checks if a rune is a digit.
func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

// isCJK checks if a rune is a CJK character.
func isCJK(r rune) bool {
	// Common CJK Unicode ranges
	return (r >= 0x4E00 && r <= 0x9FFF) || // CJK Unified Ideographs
		(r >= 0x3400 && r <= 0x4DBF) || // CJK Unified Ideographs Extension A
		(r >= 0x20000 && r <= 0x2A6DF) || // CJK Unified Ideographs Extension B
		(r >= 0xF900 && r <= 0xFAFF) || // CJK Compatibility Ideographs
		(r >= 0x3040 && r <= 0x309F) || // Japanese Hiragana
		(r >= 0x30A0 && r <= 0x30FF) || // Japanese Katakana
		(r >= 0xAC00 && r <= 0xD7AF) // Korean Hangul
}

// isAlphaNumeric checks if a rune is alphanumeric.
func isAlphaNumeric(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9')
}

// isTimestampAt checks if there's a timestamp at position start in content.
func isTimestampAt(content string, start int) bool {
	if start+19 > len(content) {
		return false
	}
	runes := []rune(content)
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

// jaccardSimilarity calculates the Jaccard similarity between two strings.
// Jaccard(A, B) = |A ∩ B| / |A ∪ B|
// Returns a value between 0 (no similarity) and 1 (identical).
func jaccardSimilarity(a, b string) float64 {
	setA := make(map[string]bool)
	setB := make(map[string]bool)

	// Tokenize both strings
	tokenizeToSet(a, setA)
	tokenizeToSet(b, setB)

	// Calculate intersection
	intersection := 0
	for word := range setA {
		if setB[word] {
			intersection++
		}
	}

	// Calculate union
	union := len(setA) + len(setB) - intersection

	if union == 0 {
		return 0
	}

	return float64(intersection) / float64(union)
}

// tokenizeToSet tokenizes a string into a set of words.
func tokenizeToSet(s string, result map[string]bool) {
	words := tokenize(s)
	for _, w := range words {
		result[w.str] = true
	}
}

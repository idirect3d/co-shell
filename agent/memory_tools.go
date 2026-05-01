// Author: L.Shuang
// Created: 2026-05-01
// Last Modified: 2026-05-01
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
	"context"
	"fmt"
	"time"

	"github.com/idirect3d/co-shell/log"
	"github.com/idirect3d/co-shell/memory"
)

// getMemorySliceTool retrieves a slice of conversation history from persistent memory.
func (a *Agent) getMemorySliceTool(ctx context.Context, args map[string]interface{}) (string, error) {
	log.Debug("getMemorySliceTool called: args=%v", args)
	lastFrom, ok := args["last_from"].(float64)
	if !ok {
		return "", fmt.Errorf("last_from argument is required")
	}
	lastTo, ok := args["last_to"].(float64)
	if !ok {
		return "", fmt.Errorf("last_to argument is required")
	}

	entries, err := a.memoryManager.GetHistorySlice(int(lastFrom), int(lastTo))
	if err != nil {
		return "", fmt.Errorf("cannot get history slice: %w", err)
	}

	formatted := memory.FormatHistorySlice(entries)
	fmt.Println(formatted)
	return formatted, nil
}

// memorySearchTool searches persistent conversation memory for messages matching given criteria.
func (a *Agent) memorySearchTool(ctx context.Context, args map[string]interface{}) (string, error) {
	log.Debug("memorySearchTool called: args=%v", args)
	params := memory.SearchParams{}

	// Apply config limits
	if a.cfg != nil {
		params.MaxResults = a.cfg.LLM.MemorySearchMaxResults
		params.MaxContentLen = a.cfg.LLM.MemorySearchMaxContentLen
	}

	// Parse keywords
	if keywordsRaw, ok := args["keywords"].([]interface{}); ok {
		params.Keywords = make([]string, 0, len(keywordsRaw))
		for _, kw := range keywordsRaw {
			if kwStr, ok := kw.(string); ok {
				params.Keywords = append(params.Keywords, kwStr)
			}
		}
	}

	// Parse since time
	if sinceStr, ok := args["since"].(string); ok && sinceStr != "" {
		since, err := time.Parse(time.RFC3339, sinceStr)
		if err != nil {
			return "", fmt.Errorf("invalid since time format (use ISO 8601, e.g. '2026-04-01T00:00:00Z'): %w", err)
		}
		params.Since = since
	}

	// Parse name filter
	if name, ok := args["name"].(string); ok {
		params.Name = name
	}

	results, err := a.memoryManager.Search(params)
	if err != nil {
		return "", fmt.Errorf("memory search failed: %w", err)
	}

	maxContentLen := 0
	if a.cfg != nil {
		maxContentLen = a.cfg.LLM.MemorySearchMaxContentLen
	}
	formatted := memory.FormatSearchResults(results, maxContentLen)
	fmt.Println(formatted)
	return formatted, nil
}

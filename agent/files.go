// Author: L.Shuang
// Created: 2026-08-28
// Last Modified: 2026-08-28
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
	"path/filepath"
	"strings"
)

// maxPredictedFiles is the maximum number of files the LLM may report as
// affected by a tool call (FEATURE-447). Reports beyond this are truncated.
const maxPredictedFiles = 3

// AffectedFile is one file/folder affected by a tool call (FEATURE-447).
// The affected files are reported by the LLM via the "affected_objects" argument (an
// array of absolute paths). Sensitive marks whether the path touches a
// user-private or system-critical location — collected for frontend display
// only, it does not force a risk level.
type AffectedFile struct {
	Path      string `json:"path"`
	Sensitive bool   `json:"sensitive,omitempty"`
}

// cleanAffectedPath trims whitespace and trailing punctuation (e.g. a colon
// the LLM may append after a path) so the reported path matches the workspace
// tree's absolute path exactly (FEATURE-447).
func cleanAffectedPath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.TrimRight(p, ":,;")
	return strings.TrimSpace(p)
}

// extractPredictedFiles collects the LLM-reported affected files from the
// meta.affected_objects field. At most maxPredictedFiles are kept.
func extractPredictedFiles(args map[string]interface{}) []string {
	meta := metaObject(args)
	var files []string
	if v, ok := meta["affected_objects"]; ok {
		if arr, ok := v.([]interface{}); ok {
			for _, item := range arr {
				if s, ok := item.(string); ok {
					s = cleanAffectedPath(s)
					if s != "" {
						files = append(files, s)
					}
				}
				if len(files) >= maxPredictedFiles {
					break
				}
			}
		}
	}
	return files
}

// affectedFiles computes the list of files/folders affected by a tool call
// (FEATURE-447). The affected files are entirely reported by the LLM via the
// meta.affected_objects field (at most maxPredictedFiles). Each entry carries a Sensitive
// flag (whether the path touches a user-private or system-critical location)
// so the frontend can display it.
//
// Each reported path is relativized against the workspace root when it is an
// absolute path under the workspace, so the frontend can match it against the
// workspace tree's relative paths exactly.
func (a *Agent) affectedFiles(args map[string]interface{}) []AffectedFile {
	predicted := extractPredictedFiles(args)
	files := make([]AffectedFile, 0, len(predicted))
	for _, p := range predicted {
		files = append(files, AffectedFile{
			Path:      a.relativizeAffectedPath(p),
			Sensitive: isSensitivePath(p),
		})
	}
	return files
}

// relativizeAffectedPath converts an absolute path under the workspace root to
// a workspace-relative path (matching the workspace tree's dataset.path), so
// the frontend can highlight it. Paths outside the workspace are left as-is.
func (a *Agent) relativizeAffectedPath(p string) string {
	p = filepath.Clean(p)
	if a.workspacePath == "" {
		return p
	}
	ws := filepath.Clean(a.workspacePath)
	if p == ws {
		return "."
	}
	if rel, err := filepath.Rel(ws, p); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return p
}

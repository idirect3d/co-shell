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
	"fmt"
	"path/filepath"
	"strings"
)

// Risk level constants (FEATURE-447). The risk level is a REQUIRED field that
// the LLM must self-assess on every tool call. It is not derived from a rule
// engine — the LLM evaluates the operation's risk (including whether it may
// touch the user's sensitive information) and reports it via the "risk"
// argument. A missing or invalid risk is a tool-call parse error.
const (
	RiskLow    = "low"
	RiskMedium = "medium"
	RiskHigh   = "high"
)

// normalizeRisk maps a possibly-aliased risk string to a canonical level.
// Accepts "low"/"medium"/"high" (case-insensitive) and returns "" for unknown.
func normalizeRisk(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "low", "l":
		return RiskLow
	case "medium", "med", "m":
		return RiskMedium
	case "high", "h":
		return RiskHigh
	}
	return ""
}

// sensitivePathPrefixes are path prefixes that indicate access to user-private
// or system-critical locations. FEATURE-447: this is collected for frontend
// display only (a "sensitive" flag on the affected file) — it does NOT force a
// risk level. Whether the operation is risky is left to the LLM's assessment.
var sensitivePathPrefixes = []string{
	// User home private dirs
	".ssh", ".aws", ".gnupg", ".config", ".kube", ".docker",
	".netrc", ".npmrc", ".pypirc", ".git-credentials",
	// System dirs
	"/etc", "/usr", "/System", "/Library", "/var/root",
	"/private/etc", "/private/var/root",
}

// isSensitivePath reports whether a path touches a sensitive location
// (user-private config, credentials, or system-critical directories). It is
// used to flag affected files as sensitive for frontend display.
func isSensitivePath(p string) bool {
	clean := filepath.ToSlash(filepath.Clean(p))
	lower := strings.ToLower(clean)
	for _, prefix := range sensitivePathPrefixes {
		if strings.Contains(lower, "/"+strings.ToLower(prefix)) ||
			strings.HasPrefix(lower, strings.ToLower(prefix)) {
			return true
		}
	}
	return false
}

// validateMeta checks that every meta object field is present and well-formed
// (FEATURE-447). All fields are REQUIRED: intent, risk, risk_reason,
// affected_objects, progress. A missing or invalid field is a tool-call error
// so the LLM can correct it.
func validateMeta(args map[string]interface{}) error {
	meta := metaObject(args)
	if len(meta) == 0 {
		return fmt.Errorf("meta object is required")
	}
	if argString(meta, "intent") == "" {
		return fmt.Errorf("meta.intent is required")
	}
	if normalizeRisk(argString(meta, "risk")) == "" {
		return fmt.Errorf("meta.risk is required and must be one of low/medium/high")
	}
	if argString(meta, "risk_reason") == "" {
		return fmt.Errorf("meta.risk_reason is required")
	}
	if _, ok := meta["affected_objects"]; !ok {
		return fmt.Errorf("meta.affected_objects is required")
	}
	// FEATURE-450: progress must contain at least 1 current-status record
	// (even if the status did not change), so the LLM always reports the
	// current execution state.
	progressArr, ok := meta["progress"].([]interface{})
	if !ok || len(progressArr) == 0 {
		return fmt.Errorf("meta.progress is required and must contain at least 1 current-status record")
	}
	return nil
}

// assessRisk extracts the LLM's self-assessed risk level from the tool call's
// meta object (FEATURE-447). It first validates that every meta field is
// present (validateMeta); a missing/invalid field returns an error, which the
// caller treats as a tool-call parse error (the tool call fails so the LLM can
// correct it). The reason is returned separately for frontend display.
func assessRisk(args map[string]interface{}) (string, string, error) {
	if err := validateMeta(args); err != nil {
		return "", "", err
	}
	meta := metaObject(args)
	risk := normalizeRisk(argString(meta, "risk"))
	reason := argString(meta, "risk_reason")
	return risk, reason, nil
}

// metaObject extracts the meta object from the tool call arguments. The meta
// object carries the transparency metadata (intent/risk/risk_reason/files/
// progress). It returns an empty map when absent or not an object.
func metaObject(args map[string]interface{}) map[string]interface{} {
	if v, ok := args["meta"]; ok {
		if m, ok := v.(map[string]interface{}); ok {
			return m
		}
	}
	return map[string]interface{}{}
}

// metaFieldNames are the only keys that legitimately belong inside the meta
// object. Any other key found inside meta is a tool parameter that the LLM
// mistakenly nested there (FIX-451) and should be promoted to the top level.
var metaFieldNames = map[string]bool{
	"intent":           true,
	"risk":             true,
	"risk_reason":      true,
	"affected_objects": true,
	"progress":         true,
}

// promoteMisplacedMetaParams promotes tool parameters that the LLM mistakenly
// nested inside the meta object (e.g. path, regex, command) up to the top level
// of the arguments map. The meta object only legitimately holds the transparency
// fields (intent/risk/risk_reason/affected_objects/progress); any other key
// found inside it is a misplaced tool parameter. This makes the tool call
// resilient to LLM formatting errors where parameters are placed inside meta
// instead of at the top level.
func promoteMisplacedMetaParams(args map[string]interface{}) {
	meta, ok := args["meta"].(map[string]interface{})
	if !ok {
		return
	}
	for key, val := range meta {
		if metaFieldNames[key] {
			continue
		}
		// Only promote when the top level does not already have this key, so a
		// correctly-placed top-level parameter always wins.
		if _, exists := args[key]; !exists {
			args[key] = val
		}
	}
}

// toolRequiresMeta reports whether the named tool's required parameter list
// includes "meta" (FEATURE-450). The unified meta validation (assessRisk) is
// only applied to tools that require meta; tools such as track_task_progress,
// reorganize_context and the board_* group do not declare meta, so they are
// skipped.
func (a *Agent) toolRequiresMeta(name string) bool {
	for _, t := range a.buildToolsInternal() {
		if t.Name == name {
			return toolRequiresMetaIn(t)
		}
	}
	return false
}

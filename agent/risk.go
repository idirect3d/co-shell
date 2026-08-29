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
	if _, ok := meta["progress"]; !ok {
		return fmt.Errorf("meta.progress is required")
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

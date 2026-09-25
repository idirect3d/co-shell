// Package log - unit tests for the optional error logs (FEATURE-554).
//
// Author: L.Shuang
// Created: 2026-09-25
// Last Modified: 2026-09-25
// MIT License - Copyright (c) 2026 L.Shuang
package log

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestErrorLogDisabledCreatesNoFile verifies that a disabled switch never
// creates a log file and writes nothing.
func TestErrorLogDisabledCreatesNoFile(t *testing.T) {
	dir := t.TempDir()
	SetToolCallErrorLogDir(dir)
	SetToolCallErrorEnabled(false)
	WriteToolCallError("read_file", `{"path":"missing.txt"}`, "read_file: Error: not found\n")

	SetLLMCallErrorLogDir(dir)
	SetLLMCallErrorEnabled(false)
	WriteLLMCallError("http-status", "API returned status 401")

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("disabled switches must not create files, got %d file(s)", len(entries))
	}
}

// TestToolCallErrorRecordFields verifies the four required fields are recorded:
// call time, tool name, raw tool call payload and the error text.
func TestToolCallErrorRecordFields(t *testing.T) {
	dir := t.TempDir()
	SetToolCallErrorLogDir(dir)
	SetToolCallErrorEnabled(true)
	defer SetToolCallErrorEnabled(false)

	raw := `{"path":"missing.txt"}`
	uiText := "read_file: Error: file not found\n"
	WriteToolCallError("read_file", raw, uiText)

	content := readOnlyLogFile(t, dir, "tool-call-error")
	for _, want := range []string{
		time.Now().Format("2006-01-02 15:04"),
		"tool=read_file",
		"raw_call: " + raw,
		"error: " + uiText,
	} {
		if !strings.Contains(content, want) {
			t.Errorf("tool call error log missing %q, got:\n%s", want, content)
		}
	}
}

// TestLLMCallErrorRecordFields verifies that the failing stage and the raw
// error detail are recorded.
func TestLLMCallErrorRecordFields(t *testing.T) {
	dir := t.TempDir()
	SetLLMCallErrorLogDir(dir)
	SetLLMCallErrorEnabled(true)
	defer SetLLMCallErrorEnabled(false)

	detail := `API returned status 401: {"error":"invalid api key"}`
	WriteLLMCallError("http-status", detail)

	content := readOnlyLogFile(t, dir, "llm-call-error")
	for _, want := range []string{
		time.Now().Format("2006-01-02 15:04"),
		"stage=http-status",
		"detail: " + detail,
	} {
		if !strings.Contains(content, want) {
			t.Errorf("LLM call error log missing %q, got:\n%s", want, content)
		}
	}
}

// TestErrorLogAppendsRecords verifies that consecutive records are appended
// instead of overwriting the file.
func TestErrorLogAppendsRecords(t *testing.T) {
	dir := t.TempDir()
	SetToolCallErrorLogDir(dir)
	SetToolCallErrorEnabled(true)
	defer SetToolCallErrorEnabled(false)

	for _, name := range []string{"read_file", "write_to_file"} {
		WriteToolCallError(name, `{"path":"x"}`, name+": Error: boom\n")
	}

	content := readOnlyLogFile(t, dir, "tool-call-error")
	if strings.Count(content, "raw_call:") != 2 {
		t.Fatalf("expected 2 records, got:\n%s", content)
	}
	for _, want := range []string{"tool=read_file", "tool=write_to_file"} {
		if !strings.Contains(content, want) {
			t.Errorf("appended log missing %q, got:\n%s", want, content)
		}
	}
}

// TestErrorLogDisableStopsWriting verifies that turning the switch off stops
// further writes while keeping the existing content.
func TestErrorLogDisableStopsWriting(t *testing.T) {
	dir := t.TempDir()
	SetLLMCallErrorLogDir(dir)
	SetLLMCallErrorEnabled(true)
	WriteLLMCallError("request", "first failure")
	SetLLMCallErrorEnabled(false)
	WriteLLMCallError("request", "second failure")
	SetLLMCallErrorEnabled(true)
	defer SetLLMCallErrorEnabled(false)

	content := readOnlyLogFile(t, dir, "llm-call-error")
	if strings.Contains(content, "second failure") {
		t.Errorf("disabled switch must not write, got:\n%s", content)
	}
	if !strings.Contains(content, "first failure") {
		t.Errorf("existing content must be kept, got:\n%s", content)
	}
}

// TestSetEnabledReportsState verifies the enable-state accessors.
func TestSetEnabledReportsState(t *testing.T) {
	SetToolCallErrorEnabled(true)
	if !IsToolCallErrorEnabled() {
		t.Error("IsToolCallErrorEnabled() = false, want true")
	}
	SetToolCallErrorEnabled(false)
	if IsToolCallErrorEnabled() {
		t.Error("IsToolCallErrorEnabled() = true, want false")
	}

	SetLLMCallErrorEnabled(true)
	if !IsLLMCallErrorEnabled() {
		t.Error("IsLLMCallErrorEnabled() = false, want true")
	}
	SetLLMCallErrorEnabled(false)
	if IsLLMCallErrorEnabled() {
		t.Error("IsLLMCallErrorEnabled() = true, want false")
	}
}

// readOnlyLogFile reads the single date-based log file of the given prefix.
func readOnlyLogFile(t *testing.T, dir, prefix string) string {
	t.Helper()
	path := filepath.Join(dir, prefix+"-"+time.Now().Format("2006-01-02")+".log")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

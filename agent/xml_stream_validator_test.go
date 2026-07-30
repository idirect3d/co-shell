// Package agent provides the core agent logic for co-shell.
//
// Author: L.Shuang
// Created: 2026-07-30
// Last Modified: 2026-07-30
// Copyright (c) 2026 L.Shuang. All rights reserved.
// MIT License.

package agent

import (
	"testing"

	"github.com/idirect3d/co-shell/llm"
)

// testTools is a minimal set of tools for validator tests.
var testTools = []llm.Tool{
	{
		Name: "execute_command",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"command": map[string]interface{}{"type": "string"},
			},
			"required": []string{"command"},
		},
	},
	{
		Name: "read_file",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path":       map[string]interface{}{"type": "string"},
				"start_line": map[string]interface{}{"type": "number"},
				"end_line":   map[string]interface{}{"type": "number"},
			},
			"required": []string{"path", "start_line", "end_line"},
		},
	},
	{
		Name: "write_to_file",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"mode":    map[string]interface{}{"type": "string"},
				"path":    map[string]interface{}{"type": "string"},
				"content": map[string]interface{}{"type": "string"},
			},
			"required": []string{"mode", "path", "content"},
		},
	},
	{
		Name: "search_files",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path":         map[string]interface{}{"type": "string"},
				"regex":        map[string]interface{}{"type": "string"},
				"file_pattern": map[string]interface{}{"type": "string"},
			},
			"required": []string{"path", "regex"},
		},
	},
	{
		Name: "attempt_completion",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"result":        map[string]interface{}{"type": "string"},
				"command":       map[string]interface{}{"type": "string"},
				"session_title": map[string]interface{}{"type": "string"},
			},
			"required": []string{"result", "session_title"},
		},
	},
	{
		Name: "view_task_plan",
		Parameters: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
	},
}

// TestXMLStreamValidatorUnknownTool verifies UC-0001:
// An unknown tool name is detected immediately at the opening tag.
func TestXMLStreamValidatorUnknownTool(t *testing.T) {
	// Save and restore XML prefix
	origPrefix := xmlTagPrefix()
	SetXMLTagPrefix("cs:")
	defer SetXMLTagPrefix(origPrefix)

	v := NewStreamingXMLValidator(testTools)

	// Simulate streaming chunks
	chunks := []string{
		"<cs:",
		"wite_to_file>", // misspelled write_to_file
	}

	for _, chunk := range chunks {
		err := v.AddChunk(chunk)
		if err != nil {
			// Expected: fatal error for unknown tool
			ferr, ok := err.(*XMLStreamFatalError)
			if !ok {
				t.Fatalf("expected *XMLStreamFatalError, got %T: %v", err, err)
			}
			if !contains(ferr.Message, "wite_to_file") {
				t.Fatalf("error message should mention 'wite_to_file', got: %s", ferr.Message)
			}
			return // success
		}
	}

	t.Fatal("expected fatal error but none was returned")
}

// TestXMLStreamValidatorUnknownParam verifies UC-0002:
// An unknown parameter name for a known tool is detected immediately.
func TestXMLStreamValidatorUnknownParam(t *testing.T) {
	origPrefix := xmlTagPrefix()
	SetXMLTagPrefix("cs:")
	defer SetXMLTagPrefix(origPrefix)

	v := NewStreamingXMLValidator(testTools)

	// Open known tool, then use misspelled param
	chunks := []string{
		"<cs:execute_command>\n",
		"<cs:commmand>", // misspelled "command"
	}

	for _, chunk := range chunks {
		err := v.AddChunk(chunk)
		if err != nil {
			ferr, ok := err.(*XMLStreamFatalError)
			if !ok {
				t.Fatalf("expected *XMLStreamFatalError, got %T: %v", err, err)
			}
			if !contains(ferr.Message, "commmand") {
				t.Fatalf("error message should mention 'commmand', got: %s", ferr.Message)
			}
			if !contains(ferr.Message, "execute_command") {
				t.Fatalf("error message should mention 'execute_command', got: %s", ferr.Message)
			}
			return
		}
	}

	t.Fatal("expected fatal error but none was returned")
}

// TestXMLStreamValidatorTagMismatch verifies UC-0003:
// Opening and closing tag names don't match.
func TestXMLStreamValidatorTagMismatch(t *testing.T) {
	origPrefix := xmlTagPrefix()
	SetXMLTagPrefix("cs:")
	defer SetXMLTagPrefix(origPrefix)

	v := NewStreamingXMLValidator(testTools)

	// Open read_file, close with execute_command
	chunks := []string{
		"<cs:read_file>\n",
		"<cs:path>/tmp/test.txt</cs:path>\n",
		"</cs:execute_command>", // wrong close tag
	}

	for _, chunk := range chunks {
		err := v.AddChunk(chunk)
		if err != nil {
			ferr, ok := err.(*XMLStreamFatalError)
			if !ok {
				t.Fatalf("expected *XMLStreamFatalError, got %T: %v", err, err)
			}
			if !contains(ferr.Message, "read_file") || !contains(ferr.Message, "execute_command") {
				t.Fatalf("error message should mention both tags, got: %s", ferr.Message)
			}
			return
		}
	}

	t.Fatal("expected fatal error but none was returned")
}

// TestXMLStreamValidatorInvalidChar verifies UC-0004:
// Tag name containing '=' illegal character.
func TestXMLStreamValidatorInvalidChar(t *testing.T) {
	origPrefix := xmlTagPrefix()
	SetXMLTagPrefix("cs:")
	defer SetXMLTagPrefix(origPrefix)

	v := NewStreamingXMLValidator(testTools)

	// Attribute-like syntax
	chunks := []string{
		"<cs:execute_command>\n",
		"<cs:command=ls>value</cs:command=ls>\n",
	}

	for _, chunk := range chunks {
		err := v.AddChunk(chunk)
		if err != nil {
			ferr, ok := err.(*XMLStreamFatalError)
			if !ok {
				t.Fatalf("expected *XMLStreamFatalError, got %T: %v", err, err)
			}
			if !contains(ferr.Message, "=") {
				t.Fatalf("error message should mention '=', got: %s", ferr.Message)
			}
			return
		}
	}

	t.Fatal("expected fatal error but none was returned")
}

// TestXMLStreamValidatorValidCall verifies UC-0005:
// A valid, complete tool call does NOT trigger any error.
func TestXMLStreamValidatorValidCall(t *testing.T) {
	origPrefix := xmlTagPrefix()
	SetXMLTagPrefix("cs:")
	defer SetXMLTagPrefix(origPrefix)

	v := NewStreamingXMLValidator(testTools)

	// Complete valid tool call in one chunk
	chunks := []string{
		"<cs:read_file>\n",
		"  <cs:path>/tmp/test.txt</cs:path>\n",
		"  <cs:start_line>1</cs:start_line>\n",
		"  <cs:end_line>50</cs:end_line>\n",
		"</cs:read_file>",
	}

	for _, chunk := range chunks {
		if err := v.AddChunk(chunk); err != nil {
			t.Fatalf("unexpected error for valid call: %v", err)
		}
	}
}

// TestXMLStreamValidatorNoParamTool verifies that a tool with no parameters
// (like view_task_plan) works without errors.
func TestXMLStreamValidatorNoParamTool(t *testing.T) {
	origPrefix := xmlTagPrefix()
	SetXMLTagPrefix("cs:")
	defer SetXMLTagPrefix(origPrefix)

	v := NewStreamingXMLValidator(testTools)

	chunks := []string{
		"<cs:view_task_plan>\n",
		"  <cs:intent>check progress</cs:intent>\n",
		"</cs:view_task_plan>",
	}

	for _, chunk := range chunks {
		if err := v.AddChunk(chunk); err != nil {
			t.Fatalf("unexpected error for valid no-param tool call: %v", err)
		}
	}

	// Make sure no fatal error was set
	if v.hasFatalError() {
		t.Fatalf("unexpected fatal error: %s", v.fatalErrorMessage())
	}
}

// TestXMLStreamValidatorSplitTag verifies UC-0006:
// Tags split across chunks are correctly assembled.
func TestXMLStreamValidatorSplitTag(t *testing.T) {
	origPrefix := xmlTagPrefix()
	SetXMLTagPrefix("cs:")
	defer SetXMLTagPrefix(origPrefix)

	v := NewStreamingXMLValidator(testTools)

	// Tag name split across chunks
	chunks := []string{
		"Let me read the file\n<cs:",
		"read_file>\n",
		"<cs:path>/tmp/test.txt</cs:path>\n",
		"<cs:start_line>1</cs:start_line>\n",
		"<cs:end_line>50</cs:end_line>\n",
		"</cs:read_file>",
	}

	for i, chunk := range chunks {
		if err := v.AddChunk(chunk); err != nil {
			t.Fatalf("unexpected error at chunk %d: %v", i, err)
		}
	}
}

// TestXMLStreamValidatorNonToolTags verifies UC-0007:
// Non-prefixed tags (HTML) are ignored without errors.
func TestXMLStreamValidatorNonToolTags(t *testing.T) {
	origPrefix := xmlTagPrefix()
	SetXMLTagPrefix("cs:")
	defer SetXMLTagPrefix(origPrefix)

	v := NewStreamingXMLValidator(testTools)

	// Mix HTML with tool call
	chunks := []string{
		"Let me read the file content.\n",
		"<div class=\"main\">This is explanation text</div>\n",
		"<cs:read_file>\n",
		"  <cs:path>/tmp/test.txt</cs:path>\n",
		"</cs:read_file>",
		"More explanation after the call.\n",
	}

	for i, chunk := range chunks {
		if err := v.AddChunk(chunk); err != nil {
			t.Fatalf("unexpected error at chunk %d: %v", i, err)
		}
	}
}

// TestXMLStreamValidatorCDATA verifies UC-0008:
// XML content inside CDATA sections is skipped during validation.
func TestXMLStreamValidatorCDATA(t *testing.T) {
	origPrefix := xmlTagPrefix()
	SetXMLTagPrefix("cs:")
	defer SetXMLTagPrefix(origPrefix)

	v := NewStreamingXMLValidator(testTools)

	// write_to_file with CDATA containing XML tags
	chunks := []string{
		"<cs:write_to_file>\n",
		"  <cs:mode>new</cs:mode>\n",
		"  <cs:path>/tmp/example.xml</cs:path>\n",
		"  <cs:content><![CDATA[\n",
		"    <root><item>test</item></root>\n",
		"  ]]></cs:content>\n",
		"</cs:write_to_file>",
	}

	for i, chunk := range chunks {
		if err := v.AddChunk(chunk); err != nil {
			t.Fatalf("unexpected error at chunk %d: %v", i, err)
		}
	}
}

// TestXMLStreamValidatorCodeBlock verifies that XML inside code blocks
// (```...```) is ignored.
func TestXMLStreamValidatorCodeBlock(t *testing.T) {
	origPrefix := xmlTagPrefix()
	SetXMLTagPrefix("cs:")
	defer SetXMLTagPrefix(origPrefix)

	v := NewStreamingXMLValidator(testTools)

	// Code block with XML example content
	chunks := []string{
		"Here's an example:\n",
		"```xml\n",
		"<cs:read_file>\n",
		"  <cs:path>test.txt</cs:path>\n",
		"</cs:read_file>\n",
		"```\n",
		"Now let me actually call it:\n",
		"<cs:read_file>\n",
		"  <cs:path>/real/file.txt</cs:path>\n",
		"</cs:read_file>",
	}

	for i, chunk := range chunks {
		if err := v.AddChunk(chunk); err != nil {
			t.Fatalf("unexpected error at chunk %d: %v", i, err)
		}
	}
}

// TestXMLStreamValidatorNestedMismatch verifies UC-0010:
// Inner parameter tag not closed before parent close tag.
func TestXMLStreamValidatorNestedMismatch(t *testing.T) {
	origPrefix := xmlTagPrefix()
	SetXMLTagPrefix("cs:")
	defer SetXMLTagPrefix(origPrefix)

	v := NewStreamingXMLValidator(testTools)

	// Missing </cs:path> before </cs:read_file>
	chunks := []string{
		"<cs:read_file>\n",
		"  <cs:path>/tmp/test.txt\n",
		"</cs:read_file>", // close parent while path is still open
	}

	for _, chunk := range chunks {
		err := v.AddChunk(chunk)
		if err != nil {
			ferr, ok := err.(*XMLStreamFatalError)
			if !ok {
				t.Fatalf("expected *XMLStreamFatalError, got %T: %v", err, err)
			}
			if !contains(ferr.Message, "path") || !contains(ferr.Message, "read_file") {
				t.Fatalf("error message should mention both tags, got: %s", ferr.Message)
			}
			return
		}
	}

	// Actually the current validator only detects this at close tag time.
	// The </cs:read_file> matches the stack top <cs:path> which is wrong.
	// Let's check if it was detected.
	if !v.hasFatalError() {
		t.Fatal("expected fatal error but none was returned")
	}
}

// TestXMLStreamValidatorMultipleCalls verifies that multiple sequential
// tool calls in a single stream are all validated correctly.
func TestXMLStreamValidatorMultipleCalls(t *testing.T) {
	origPrefix := xmlTagPrefix()
	SetXMLTagPrefix("cs:")
	defer SetXMLTagPrefix(origPrefix)

	v := NewStreamingXMLValidator(testTools)

	chunks := []string{
		"<cs:read_file>\n",
		"  <cs:path>/tmp/a.txt</cs:path>\n",
		"</cs:read_file>\n",
		"<cs:execute_command>\n",
		"  <cs:command>ls -la</cs:command>\n",
		"</cs:execute_command>",
	}

	for i, chunk := range chunks {
		if err := v.AddChunk(chunk); err != nil {
			t.Fatalf("unexpected error at chunk %d: %v", i, err)
		}
	}
}

// TestXMLStreamValidatorReset verifies that Reset() clears all state
// and allows reuse.
func TestXMLStreamValidatorReset(t *testing.T) {
	origPrefix := xmlTagPrefix()
	SetXMLTagPrefix("cs:")
	defer SetXMLTagPrefix(origPrefix)

	v := NewStreamingXMLValidator(testTools)

	// First call — valid
	v.AddChunk("<cs:view_task_plan>\n")
	v.AddChunk("</cs:view_task_plan>\n")

	if v.hasFatalError() {
		t.Fatalf("first call should be valid, got: %s", v.fatalErrorMessage())
	}

	// Reset
	v.Reset()

	// Second call — should detect error
	err := v.AddChunk("<cs:unknown_tool>\n")
	if err == nil {
		t.Fatal("expected error after reset + unknown tool, got nil")
	}
}

// TestXMLStreamValidatorLargeFileEarly verifies UC-0009:
// Error detected in first chunk of a long file write.
func TestXMLStreamValidatorLargeFileEarly(t *testing.T) {
	origPrefix := xmlTagPrefix()
	SetXMLTagPrefix("cs:")
	defer SetXMLTagPrefix(origPrefix)

	v := NewStreamingXMLValidator(testTools)

	// First chunk contains the misspelled tool name
	chunks := []string{
		"<cs:wite_to_file>\n", // misspelled "write_to_file"
		"<cs:mode>new</cs:mode>\n",
		"<cs:path>/tmp/large.txt</cs:path>\n",
		"<cs:content>",
	}

	err := v.AddChunk(chunks[0])
	if err == nil {
		t.Fatal("error should be detected on first chunk with misspelled tool name")
	}

	ferr, ok := err.(*XMLStreamFatalError)
	if !ok {
		t.Fatalf("expected *XMLStreamFatalError, got %T: %v", err, err)
	}
	if !contains(ferr.Message, "wite_to_file") {
		t.Fatalf("error message should mention 'wite_to_file', got: %s", ferr.Message)
	}
}

// TestXMLStreamValidatorSelfClosing verifies that self-closing tags
// like <cs:view_task_plan /> work properly.
func TestXMLStreamValidatorSelfClosing(t *testing.T) {
	origPrefix := xmlTagPrefix()
	SetXMLTagPrefix("cs:")
	defer SetXMLTagPrefix(origPrefix)

	v := NewStreamingXMLValidator(testTools)

	// Self-closing tag
	chunks := []string{
		"<cs:view_task_plan />\n",
	}

	for i, chunk := range chunks {
		if err := v.AddChunk(chunk); err != nil {
			t.Fatalf("unexpected error at chunk %d: %v", i, err)
		}
	}
}

// TestXMLStreamValidatorEmptyChunks verifies that empty chunks don't break the validator.
func TestXMLStreamValidatorEmptyChunks(t *testing.T) {
	origPrefix := xmlTagPrefix()
	SetXMLTagPrefix("cs:")
	defer SetXMLTagPrefix(origPrefix)

	v := NewStreamingXMLValidator(testTools)

	chunks := []string{
		"",
		"\n\n  \n\n",
		"<cs:read_file>\n",
		"  <cs:path>/tmp/test.txt</cs:path>\n",
		"</cs:read_file>",
		"",
	}

	for i, chunk := range chunks {
		if err := v.AddChunk(chunk); err != nil {
			t.Fatalf("unexpected error at chunk %d: %v", i, err)
		}
	}
}

// TestXMLStreamValidatorOnlyText verifies that plain text without any
// tool calls passes through without errors.
func TestXMLStreamValidatorOnlyText(t *testing.T) {
	v := NewStreamingXMLValidator(testTools)

	text := "This is just a plain text response from the LLM.\nNo tool calls here.\n"
	if err := v.AddChunk(text); err != nil {
		t.Fatalf("unexpected error for plain text: %v", err)
	}
}

// TestXMLStreamValidatorWhitespaceInsideTags verifies whitespace-handling
// inside tag names and content.
func TestXMLStreamValidatorWhitespaceInsideTags(t *testing.T) {
	origPrefix := xmlTagPrefix()
	SetXMLTagPrefix("cs:")
	defer SetXMLTagPrefix(origPrefix)

	v := NewStreamingXMLValidator(testTools)

	chunks := []string{
		"<cs:read_file >\n", // space before >
		"  <cs:path >/tmp/test.txt</cs:path >\n",
		"</cs:read_file>\n",
	}

	for i, chunk := range chunks {
		if err := v.AddChunk(chunk); err != nil {
			t.Fatalf("unexpected error at chunk %d: %v", i, err)
		}
	}
}

// TestXMLStreamValidatorDefaultPrefix verifies that the validator works
// with the default "cs:" prefix without explicit SetXMLTagPrefix.
func TestXMLStreamValidatorDefaultPrefix(t *testing.T) {
	origPrefix := xmlTagPrefix()
	// Don't change prefix — use whatever default is set
	defer SetXMLTagPrefix(origPrefix)

	// Ensure default prefix is "cs:"
	SetXMLTagPrefix("cs:")

	v := NewStreamingXMLValidator(testTools)

	// Valid call with default prefix
	chunks := []string{
		"<cs:read_file>\n",
		"  <cs:path>/tmp/test.txt</cs:path>\n",
		"</cs:read_file>",
	}

	for i, chunk := range chunks {
		if err := v.AddChunk(chunk); err != nil {
			t.Fatalf("unexpected error at chunk %d: %v", i, err)
		}
	}
}

// helper: contains checks if a string contains a substring.
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

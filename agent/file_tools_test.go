// Author: L.Shuang
// Created: 2026-04-26
// Last Modified: 2026-04-26
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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReadFileTool tests the read_file tool.
func TestReadFileTool(t *testing.T) {
	// Create a temporary test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := "line1\nline2\nline3\nline4\nline5\n"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	agent := &Agent{}

	tests := []struct {
		name    string
		args    map[string]interface{}
		want    []string // substrings that should be in the result
		wantErr bool
	}{
		{
			name: "read entire file",
			args: map[string]interface{}{
				"path": testFile,
			},
			want: []string{"File:", "6 lines total", "1 | line1", "5 | line5"},

			wantErr: false,
		},
		{
			name: "read with start_line",
			args: map[string]interface{}{
				"path":       testFile,
				"start_line": float64(2),
			},
			want:    []string{"2 | line2", "3 | line3", "5 | line5"},
			wantErr: false,
		},
		{
			name: "read with start_line and end_line",
			args: map[string]interface{}{
				"path":       testFile,
				"start_line": float64(2),
				"end_line":   float64(4),
			},
			want:    []string{"2 | line2", "3 | line3", "4 | line4"},
			wantErr: false,
			// Should NOT include line1 or line5
		},
		{
			name: "read single line",
			args: map[string]interface{}{
				"path":       testFile,
				"start_line": float64(3),
				"end_line":   float64(3),
			},
			want:    []string{"3 | line3"},
			wantErr: false,
		},
		{
			name: "start_line exceeds file length",
			args: map[string]interface{}{
				"path":       testFile,
				"start_line": float64(100),
			},
			wantErr: true,
		},
		{
			name: "missing path",
			args: map[string]interface{}{
				"start_line": float64(1),
			},
			wantErr: true,
		},
		{
			name: "file not found",
			args: map[string]interface{}{
				"path": filepath.Join(tmpDir, "nonexistent.txt"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := agent.readFileTool(context.Background(), tt.args)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none, result=%q", result)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			for _, want := range tt.want {
				if !strings.Contains(result, want) {
					t.Errorf("result should contain %q, got:\n%s", want, result)
				}
			}
			// Check that line1 is NOT in the result for start_line=2
			if tt.name == "read with start_line and end_line" {
				if strings.Contains(result, "1 | line1") {
					t.Errorf("result should NOT contain line1, got:\n%s", result)
				}
				if strings.Contains(result, "5 | line5") {
					t.Errorf("result should NOT contain line5, got:\n%s", result)
				}
			}
		})
	}
}

// TestSearchFilesTool tests the search_files tool.
func TestSearchFilesTool(t *testing.T) {
	// Create temporary test files
	tmpDir := t.TempDir()

	// Create file1.go
	file1 := filepath.Join(tmpDir, "file1.go")
	os.WriteFile(file1, []byte("package main\n\nfunc hello() {\n\tfmt.Println(\"hello\")\n}\n"), 0644)

	// Create file2.go
	file2 := filepath.Join(tmpDir, "file2.go")
	os.WriteFile(file2, []byte("package main\n\nfunc world() {\n\tfmt.Println(\"world\")\n}\n"), 0644)

	// Create file3.txt (should be excluded by file_pattern)
	file3 := filepath.Join(tmpDir, "file3.txt")
	os.WriteFile(file3, []byte("hello world\n"), 0644)

	agent := &Agent{}

	tests := []struct {
		name    string
		args    map[string]interface{}
		want    []string
		wantErr bool
	}{
		{
			name: "search all files for hello",
			args: map[string]interface{}{
				"path":  tmpDir,
				"regex": "hello",
			},
			want:    []string{"file1.go", "file3.txt"},
			wantErr: false,
		},
		{
			name: "search only .go files",
			args: map[string]interface{}{
				"path":         tmpDir,
				"regex":        "func",
				"file_pattern": "*.go",
			},
			want:    []string{"file1.go", "file2.go"},
			wantErr: false,
		},
		{
			name: "search with no matches",
			args: map[string]interface{}{
				"path":  tmpDir,
				"regex": "zzz_nonexistent_zzz",
			},
			want:    []string{"No matches found"},
			wantErr: false,
		},
		{
			name: "invalid regex",
			args: map[string]interface{}{
				"path":  tmpDir,
				"regex": "[invalid",
			},
			wantErr: true,
		},
		{
			name: "missing path",
			args: map[string]interface{}{
				"regex": "hello",
			},
			wantErr: true,
		},
		{
			name: "missing regex",
			args: map[string]interface{}{
				"path": tmpDir,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := agent.searchFilesTool(context.Background(), tt.args)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none, result=%q", result)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			for _, want := range tt.want {
				if !strings.Contains(result, want) {
					t.Errorf("result should contain %q, got:\n%s", want, result)
				}
			}
		})
	}
}

// TestListCodeDefinitionNamesTool tests the list_code_definition_names tool.
func TestListCodeDefinitionNamesTool(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a Go file with definitions
	goFile := filepath.Join(tmpDir, "main.go")
	os.WriteFile(goFile, []byte("package main\n\ntype User struct {\n\tName string\n}\n\nfunc hello() {}\nfunc (u *User) SayHello() {}\n"), 0644)

	// Create a Python file with definitions
	pyFile := filepath.Join(tmpDir, "app.py")
	os.WriteFile(pyFile, []byte("class MyClass:\n\tpass\n\ndef my_function():\n\tpass\n"), 0644)

	// Create a non-source file (should be ignored)
	os.WriteFile(filepath.Join(tmpDir, "readme.md"), []byte("# Hello\n"), 0644)

	agent := &Agent{}

	tests := []struct {
		name    string
		args    map[string]interface{}
		want    []string
		wantErr bool
	}{
		{
			name: "list definitions in directory",
			args: map[string]interface{}{
				"path": tmpDir,
			},
			want:    []string{"main.go", "app.py", "User", "hello", "SayHello", "MyClass", "my_function"},
			wantErr: false,
		},
		{
			name:    "missing path",
			args:    map[string]interface{}{},
			wantErr: true,
		},
		{
			name: "directory not found",
			args: map[string]interface{}{
				"path": filepath.Join(tmpDir, "nonexistent"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := agent.listCodeDefinitionNamesTool(context.Background(), tt.args)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none, result=%q", result)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			for _, want := range tt.want {
				if !strings.Contains(result, want) {
					t.Errorf("result should contain %q, got:\n%s", want, result)
				}
			}
		})
	}
}

// TestReplaceInFileTool tests the replace_in_file tool.
func TestReplaceInFileTool(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.go")
	originalContent := "package main\n\nfunc oldName() {\n\t// do something\n}\n"
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	agent := &Agent{}

	tests := []struct {
		name    string
		args    map[string]interface{}
		want    string // expected file content after replacement
		wantErr bool
	}{
		{
			name: "replace function name",
			args: map[string]interface{}{
				"path":    testFile,
				"search":  "func oldName()",
				"replace": "func newName()",
			},
			want:    "package main\n\nfunc newName() {\n\t// do something\n}\n",
			wantErr: false,
		},
		{
			name: "search not found",
			args: map[string]interface{}{
				"path":    testFile,
				"search":  "nonexistent content",
				"replace": "replacement",
			},
			wantErr: true,
		},
		{
			name: "missing path",
			args: map[string]interface{}{
				"search":  "abc",
				"replace": "xyz",
			},
			wantErr: true,
		},
		{
			name: "missing search",
			args: map[string]interface{}{
				"path":    testFile,
				"replace": "xyz",
			},
			wantErr: true,
		},
		{
			name: "missing replace",
			args: map[string]interface{}{
				"path":   testFile,
				"search": "abc",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset file content before each test
			if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
				t.Fatal(err)
			}

			_, err := agent.replaceInFileTool(context.Background(), tt.args)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Verify file content
			data, err := os.ReadFile(testFile)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != tt.want {
				t.Errorf("file content mismatch:\ngot:\n%s\nwant:\n%s", string(data), tt.want)
			}
		})
	}
}

// TestWriteToFileTool tests the write_to_file tool.
func TestWriteToFileTool(t *testing.T) {
	tmpDir := t.TempDir()
	agent := &Agent{}

	tests := []struct {
		name    string
		args    map[string]interface{}
		want    string // expected file content
		wantErr bool
	}{
		{
			name: "write new file",
			args: map[string]interface{}{
				"path":    filepath.Join(tmpDir, "newfile.txt"),
				"content": "hello world",
			},
			want:    "hello world",
			wantErr: false,
		},
		{
			name: "write to nested directory (auto-create)",
			args: map[string]interface{}{
				"path":    filepath.Join(tmpDir, "sub", "nested", "deep.txt"),
				"content": "nested content",
			},
			want:    "nested content",
			wantErr: false,
		},
		{
			name: "overwrite existing file",
			args: map[string]interface{}{
				"path":    filepath.Join(tmpDir, "newfile.txt"),
				"content": "overwritten content",
			},
			want:    "overwritten content",
			wantErr: false,
		},
		{
			name: "write empty content",
			args: map[string]interface{}{
				"path":    filepath.Join(tmpDir, "empty.txt"),
				"content": "",
			},
			want:    "",
			wantErr: false,
		},
		{
			name: "missing path",
			args: map[string]interface{}{
				"content": "test",
			},
			wantErr: true,
		},
		{
			name: "missing content",
			args: map[string]interface{}{
				"path": filepath.Join(tmpDir, "test.txt"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := agent.writeToFileTool(context.Background(), tt.args)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none, result=%q", result)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			// Verify file was created with correct content
			path := tt.args["path"].(string)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != tt.want {
				t.Errorf("file content mismatch:\ngot:  %q\nwant: %q", string(data), tt.want)
			}
		})
	}
}

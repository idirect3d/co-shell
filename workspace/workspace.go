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

// Package workspace manages the co-shell workspace directory structure.
//
// The workspace is the root directory for all co-shell runtime data:
//   - bin/    : Built-in tools and scripts
//   - db/     : Local data files (bbolt database, etc.)
//   - log/    : co-shell runtime logs
//   - output/ : Formal output files
//   - tmp/    : Temporary working files
//
// The workspace path defaults to the current working directory where co-shell
// is launched, and can be overridden via the --workspace command-line flag.
package workspace

import (
	"fmt"
	"os"
	"path/filepath"
)

// Default subdirectory names within a workspace.
const (
	DirBin    = "bin"
	DirDB     = "db"
	DirLog    = "log"
	DirOutput = "output"
	DirTmp    = "tmp"
)

// Workspace holds the resolved workspace path and provides methods to access
// its subdirectories.
type Workspace struct {
	root string
}

// New creates a Workspace with the given root path.
// If root is empty, the current working directory is used.
// It automatically creates all required subdirectories.
func New(root string) (*Workspace, error) {
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("cannot get current working directory: %w", err)
		}
	}

	// Resolve to absolute path
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve workspace path %q: %w", root, err)
	}

	w := &Workspace{root: absRoot}

	// Create all required subdirectories
	if err := w.ensureDirs(); err != nil {
		return nil, fmt.Errorf("cannot create workspace directories: %w", err)
	}

	return w, nil
}

// Root returns the absolute path to the workspace root.
func (w *Workspace) Root() string {
	return w.root
}

// BinDir returns the path to the bin/ directory.
func (w *Workspace) BinDir() string {
	return filepath.Join(w.root, DirBin)
}

// DBDir returns the path to the db/ directory.
func (w *Workspace) DBDir() string {
	return filepath.Join(w.root, DirDB)
}

// LogDir returns the path to the log/ directory.
func (w *Workspace) LogDir() string {
	return filepath.Join(w.root, DirLog)
}

// OutputDir returns the path to the output/ directory.
func (w *Workspace) OutputDir() string {
	return filepath.Join(w.root, DirOutput)
}

// TmpDir returns the path to the tmp/ directory.
func (w *Workspace) TmpDir() string {
	return filepath.Join(w.root, DirTmp)
}

// ConfigPath returns the path to the config.json file within the workspace.
func (w *Workspace) ConfigPath() string {
	return filepath.Join(w.root, "config.json")
}

// DBPath returns the path to the bbolt database file within the db/ directory.
func (w *Workspace) DBPath() string {
	return filepath.Join(w.DBDir(), "co-shell.db")
}

// LogFilePath returns the path to the log file within the log/ directory.
// The filename includes the current date.
func (w *Workspace) LogFilePath(date string) string {
	return filepath.Join(w.LogDir(), fmt.Sprintf("co-shell-%s.log", date))
}

// ensureDirs creates all required subdirectories if they don't exist.
func (w *Workspace) ensureDirs() error {
	dirs := []string{
		w.BinDir(),
		w.DBDir(),
		w.LogDir(),
		w.OutputDir(),
		w.TmpDir(),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("cannot create directory %q: %w", dir, err)
		}
	}
	return nil
}

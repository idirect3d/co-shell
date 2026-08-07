// Author: L.Shuang
// Created: 2026-08-06
// Last Modified: 2026-08-06
//
// # MIT License
//
// # Copyright (c) 2026 L.Shuang
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

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/store"
	"github.com/idirect3d/co-shell/workspace"
)

// TestShowContext_ReflectsPRINCIPLESChanges verifies that modifying
// PRINCIPLES.md on disk is reflected by the next :context call
// (showContext triggers RebuildSystemPrompt before displaying).
func TestShowContext_ReflectsPRINCIPLESChanges(t *testing.T) {
	i18n.Init("zh")

	ws := t.TempDir()
	// PRINCIPLES.md with initial content
	if err := os.WriteFile(filepath.Join(ws, "PRINCIPLES.md"), []byte("第一版原则"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := config.DefaultConfig()
	wsObj, err := workspace.New(ws)
	if err != nil {
		t.Fatalf("cannot init workspace: %v", err)
	}
	boltStore, err := store.NewStore(wsObj)
	if err != nil {
		t.Fatalf("cannot init bbolt store: %v", err)
	}
	ds := store.NewDualStore(boltStore, nil)
	ag := agent.New(nil, nil, ds, "")
	ag.SetWorkspacePath(ws)
	ag.SetConfig(cfg)

	h := NewContextHandler(ag, ds)

	// First :context shows the initial PRINCIPLES.md content
	out1, err := h.showContext(false)
	if err != nil {
		t.Fatalf("showContext error: %v", err)
	}
	if !strings.Contains(out1, "第一版原则") {
		t.Fatalf("expected initial PRINCIPLES.md content in context, got:\n%s", out1)
	}

	// Modify PRINCIPLES.md on disk
	if err := os.WriteFile(filepath.Join(ws, "PRINCIPLES.md"), []byte("第二版原则已修改"), 0644); err != nil {
		t.Fatal(err)
	}

	// Second :context reflects the modified PRINCIPLES.md
	out2, err := h.showContext(false)
	if err != nil {
		t.Fatalf("showContext error: %v", err)
	}
	if !strings.Contains(out2, "第二版原则已修改") {
		t.Fatalf("expected modified PRINCIPLES.md content in context, got:\n%s", out2)
	}
}

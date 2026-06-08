// Author: L.Shuang
// Created: 2026-05-21
// Last Modified: 2026-05-21
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

package store

import (
	"path/filepath"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/workspace"
)

// NewStoreFromConfig creates a DualStore based on the configuration.
// It always uses bbolt as the primary store.
// If PostgreSQL is configured and available, it also creates a PGStore
// for dual-write of memory and history data.
func NewStoreFromConfig(cfg *config.Config, ws *workspace.Workspace) (*DualStore, error) {
	// Always create the bbolt store (primary)
	boltStore, err := NewStore(ws)
	if err != nil {
		return nil, err
	}

	// Check if PostgreSQL is configured and try to connect (silent, prints happen in REPL syncDB)
	var pgStore *PGStore
	if cfg.DB.Enabled && cfg.DB.Host != "" && cfg.DB.Port > 0 && cfg.DB.DBName != "" {
		// Use workspace folder name as default schema if not explicitly set
		if cfg.DB.Schema == "" || cfg.DB.Schema == "public" {
			cfg.DB.Schema = filepath.Base(ws.Root())
		}

		pg, err := NewPGStore(cfg.DB)
		if err == nil {
			pgStore = pg
		}
	}

	// Create the DualStore (wraps bbolt + optional PG) with auto-sync setting
	dualStore := NewDualStoreWithSync(boltStore, pgStore, cfg.DB.AutoSync)
	return dualStore, nil
}

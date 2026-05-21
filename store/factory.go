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
	"fmt"
	"path/filepath"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/workspace"
)

// NewStoreFromConfig creates a Store based on the configuration.
// If PostgreSQL is configured and available, it returns a PGStore.
// Otherwise, it falls back to the local bbolt Store.
func NewStoreFromConfig(cfg *config.Config, ws *workspace.Workspace) (interface {
	Close() error
	SaveHistory(input string) error
	LoadHistory() ([]string, error)
	ListHistory() ([]HistoryEntryWithTime, error)
	ClearHistory() error
	SaveContext(key string, data []byte) error
	GetContext(key string) ([]byte, bool, error)
	DeleteContext(key string) error
	ClearContext() error
	SaveSchedule(id int, data []byte) error
	LoadSchedules() (map[int][]byte, error)
	DeleteSchedule(id int) error
	NextTaskPlanID() (int, error)
	SaveTaskPlan(id int, data []byte) error
	GetTaskPlan(id int) ([]byte, bool, error)
	ListTaskPlans() (map[int][]byte, error)
	DeleteTaskPlan(id int) error
	SaveConversationMessage(id string, data []byte) error
	ListConversationMessages() ([][]byte, error)
	SaveMemory(key, value string) error
	GetMemory(key string) (string, bool, error)
	SearchMemory(prefix string) ([]MemoryEntry, error)
	DeleteMemory(key string) error
	DeleteMemoryRange(lastFrom, lastTo int) error
	SaveTokenUsage(entry *TokenUsageEntry) error
	ListTokenUsage() ([]TokenUsageEntry, error)
	ClearConversationMessages() error
	SaveSession(messages []byte) error
	LoadSession() ([]byte, bool, error)
	ClearSession() error
}, error) {
	// Check if PostgreSQL is configured
	if cfg.DB.Enabled && cfg.DB.Host != "" && cfg.DB.Port > 0 && cfg.DB.DBName != "" {
		// Use workspace folder name as default schema if not explicitly set
		if cfg.DB.Schema == "" || cfg.DB.Schema == "public" {
			cfg.DB.Schema = filepath.Base(ws.Root())
		}

		fmt.Println(i18n.TF(i18n.KeyDBConnecting, cfg.DB.Host, cfg.DB.Port, cfg.DB.DBName))

		pgStore, err := NewPGStore(cfg.DB)
		if err != nil {
			fmt.Println(i18n.TF(i18n.KeyDBConnectFailed, err))
			fmt.Println(i18n.T(i18n.KeyDBFallbackToLocal))
		} else {
			fmt.Println(i18n.TF(i18n.KeyDBConnected, cfg.DB.Host, cfg.DB.Port, cfg.DB.DBName))
			return pgStore, nil
		}
	}

	// Fall back to bbolt
	return NewStore(ws)
}

// Author: L.Shuang
// Created: 2026-07-07
// Last Modified: 2026-07-07
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
// EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
// MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO
// EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES
// OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE,
// ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER
// DEALINGS IN THE SOFTWARE.

package agent

import (
	"fmt"
	"sync"
	"time"

	"github.com/idirect3d/co-shell/xlsx"
)

// excelClipboard holds copied cell data.
type excelClipboard struct {
	Values   [][]*xlsx.CellValue
	CutMode  bool
	CutSheet string
	CutRange *xlsx.CellRange
}

// excelSession holds an open workbook session.
type excelSession struct {
	Workbook   *xlsx.Workbook
	Path       string
	LastAccess time.Time
	Dirty      bool
	SessionID  string
	Clipboard  *excelClipboard
}

// excelSessionManager manages multiple open workbook sessions.
// Sessions never auto-expire — they must be explicitly closed via excel_close.
type excelSessionManager struct {
	mu          sync.Mutex
	sessions    map[string]*excelSession
	maxSessions int // max concurrent sessions (from config)
}

func newExcelSessionManager() *excelSessionManager {
	return &excelSessionManager{
		sessions:    make(map[string]*excelSession),
		maxSessions: 5,
	}
}

// Configure sets config values.
func (mgr *excelSessionManager) Configure(ttl, maxSessions int) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	if maxSessions > 0 {
		mgr.maxSessions = maxSessions
	}
}

func (mgr *excelSessionManager) open(path string) (string, error) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	// Check if already open
	for _, s := range mgr.sessions {
		if s.Path == path {
			return "", fmt.Errorf("file %q is already open in session %s", path, s.SessionID)
		}
	}

	// Check max sessions — no auto-eviction, tell LLM to close some first
	if len(mgr.sessions) >= mgr.maxSessions {
		sessionList := ""
		for id, s := range mgr.sessions {
			sessionList += fmt.Sprintf("\n  %s: %s", id, s.Path)
		}
		return "", fmt.Errorf("已达到最大并发 Excel 会话数 (%d)。请先调用 excel_close 关闭以下不再需要的会话：%s", mgr.maxSessions, sessionList)
	}

	wb, err := xlsx.OpenFile(path)
	if err != nil {
		return "", fmt.Errorf("cannot open xlsx file: %w", err)
	}

	sessionID := fmt.Sprintf("xl_%d", time.Now().UnixNano())
	mgr.sessions[sessionID] = &excelSession{
		Workbook:   wb,
		Path:       path,
		LastAccess: time.Now(),
		SessionID:  sessionID,
	}

	return sessionID, nil
}

func (mgr *excelSessionManager) get(sessionID string) (*excelSession, error) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	s, ok := mgr.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("excel session %q not found (it may have been closed or expired)", sessionID)
	}
	s.LastAccess = time.Now()
	return s, nil
}

func (mgr *excelSessionManager) save(sessionID string) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	s, ok := mgr.sessions[sessionID]
	if !ok {
		return fmt.Errorf("excel session %q not found", sessionID)
	}

	if err := s.Workbook.Save(); err != nil {
		return fmt.Errorf("cannot save workbook: %w", err)
	}
	s.Dirty = false
	s.LastAccess = time.Now()
	return nil
}

func (mgr *excelSessionManager) close(sessionID string) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	s, ok := mgr.sessions[sessionID]
	if !ok {
		return fmt.Errorf("excel session %q not found", sessionID)
	}

	if s.Dirty {
		if err := s.Workbook.Save(); err != nil {
			return fmt.Errorf("cannot save workbook before close: %w", err)
		}
	}

	delete(mgr.sessions, sessionID)
	return nil
}

func (mgr *excelSessionManager) closeAll() {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	for id, s := range mgr.sessions {
		if s.Dirty {
			_ = s.Workbook.Save()
		}
		delete(mgr.sessions, id)
	}
}

func (mgr *excelSessionManager) listSessions() []string {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	result := make([]string, 0, len(mgr.sessions))
	for id, s := range mgr.sessions {
		result = append(result, fmt.Sprintf("%s (%s)", id, s.Path))
	}
	return result
}

func (mgr *excelSessionManager) setClipboard(sessionID string, values [][]*xlsx.CellValue, cutMode bool, cutSheet string, cutRange *xlsx.CellRange) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	s, ok := mgr.sessions[sessionID]
	if !ok {
		return fmt.Errorf("excel session %q not found", sessionID)
	}

	s.Clipboard = &excelClipboard{
		Values:   values,
		CutMode:  cutMode,
		CutSheet: cutSheet,
		CutRange: cutRange,
	}
	s.LastAccess = time.Now()
	return nil
}

func (mgr *excelSessionManager) getClipboard(sessionID string) (*excelClipboard, error) {
	s, err := mgr.get(sessionID)
	if err != nil {
		return nil, err
	}
	if s.Clipboard == nil {
		return nil, fmt.Errorf("clipboard is empty in session %q", sessionID)
	}
	return s.Clipboard, nil
}

func (mgr *excelSessionManager) touch(sessionID string) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	if s, ok := mgr.sessions[sessionID]; ok {
		s.LastAccess = time.Now()
	}
}

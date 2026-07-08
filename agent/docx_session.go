// Author: L.Shuang
// Created: 2026-07-08
// Last Modified: 2026-07-08
//
// MIT License
// Copyright (c) 2026 L.Shuang
package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/idirect3d/co-shell/docx"
)

// docxSession holds an open DOCX document session.
type docxSession struct {
	Doc        *docx.Document
	Path       string
	LastAccess time.Time
	Dirty      bool
	SessionID  string
	ReadOnly   bool // if true, save operations fail
}

// docxSessionManager manages multiple open DOCX sessions.
type docxSessionManager struct {
	mu          sync.Mutex
	sessions    map[string]*docxSession
	maxSessions int
}

func newDocxSessionManager() *docxSessionManager {
	return &docxSessionManager{
		sessions:    make(map[string]*docxSession),
		maxSessions: 5,
	}
}

// Configure sets config values for DOCX sessions.
func (mgr *docxSessionManager) Configure(maxSessions int) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	if maxSessions > 0 {
		mgr.maxSessions = maxSessions
	}
}

func (mgr *docxSessionManager) open(path string) (string, error) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	// Check if already open
	for _, s := range mgr.sessions {
		if s.Path == path {
			return "", fmt.Errorf("file %q is already open in session %s", path, s.SessionID)
		}
	}

	// Check max sessions
	if len(mgr.sessions) >= mgr.maxSessions {
		sessionList := ""
		for id, s := range mgr.sessions {
			sessionList += fmt.Sprintf("\n  %s: %s", id, s.Path)
		}
		return "", fmt.Errorf("已达到最大并发 DOCX 会话数 (%d)。请先调用 word_close 关闭以下不再需要的会话：%s", mgr.maxSessions, sessionList)
	}

	return "", fmt.Errorf("docx file %q does not exist", path)
}

// openWithMode opens a DOCX file with the specified mode.
// mode: "create", "read", "copy"
func (mgr *docxSessionManager) openWithMode(path, mode string) (string, error) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	// Check if already open (for the final resolved path on copy mode)
	var finalPath string
	readOnly := false

	switch mode {
	case "create":
		// Create new empty file (path must not exist)
		if _, err := os.Stat(path); err == nil {
			return "", fmt.Errorf("file %q already exists. For mode=create, the file must not exist", path)
		}
		doc := docx.CreateEmpty(path)
		if err := doc.SaveAs(path); err != nil {
			return "", fmt.Errorf("cannot create new docx file %q: %w", path, err)
		}
		finalPath = path

	case "read":
		// Open existing file read-only
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return "", fmt.Errorf("file %q does not exist. For mode=read, the file must exist", path)
		}
		readOnly = true
		finalPath = path

	case "copy":
		// Open existing file, copy before modifying
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return "", fmt.Errorf("file %q does not exist. For mode=copy, the file must exist", path)
		}
		// Generate copy path with timestamp
		ext := filepath.Ext(path)
		base := strings.TrimSuffix(path, ext)
		ts := time.Now().Format("20060102_150405")
		finalPath = fmt.Sprintf("%s.copy_%s%s", base, ts, ext)
		// Copy the file
		if err := copyFile(path, finalPath); err != nil {
			return "", fmt.Errorf("cannot copy file: %w", err)
		}

	default:
		return "", fmt.Errorf("unsupported mode: %s (supported: create, read, copy)", mode)
	}

	// Check if already open (for the resolved final path)
	for _, s := range mgr.sessions {
		if s.Path == finalPath {
			return "", fmt.Errorf("file %q is already open in session %s", finalPath, s.SessionID)
		}
	}

	// Check max sessions
	if len(mgr.sessions) >= mgr.maxSessions {
		sessionList := ""
		for id, s := range mgr.sessions {
			sessionList += fmt.Sprintf("\n  %s: %s", id, s.Path)
		}
		return "", fmt.Errorf("已达到最大并发 DOCX 会话数 (%d)。请先调用 word_close 关闭以下不再需要的会话：%s", mgr.maxSessions, sessionList)
	}

	doc, err := docx.OpenFile(finalPath)
	if err != nil {
		return "", fmt.Errorf("cannot open docx file %q: %w", finalPath, err)
	}

	sessionID := fmt.Sprintf("doc_%d", len(mgr.sessions)+1)
	mgr.sessions[sessionID] = &docxSession{
		Doc:        doc,
		Path:       finalPath,
		LastAccess: time.Now(),
		SessionID:  sessionID,
		ReadOnly:   readOnly,
	}

	return sessionID, nil
}

func (mgr *docxSessionManager) get(sessionID string) (*docxSession, error) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	s, ok := mgr.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session %q not found", sessionID)
	}
	s.LastAccess = time.Now()
	return s, nil
}

func (mgr *docxSessionManager) save(sessionID string) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	s, ok := mgr.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session %q not found", sessionID)
	}

	if err := s.Doc.Save(); err != nil {
		return fmt.Errorf("cannot save docx file %q: %w", s.Path, err)
	}
	s.Dirty = false
	return nil
}

func (mgr *docxSessionManager) close(sessionID string) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	s, ok := mgr.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session %q not found", sessionID)
	}

	// Auto-save if dirty
	if s.Dirty {
		if err := s.Doc.Save(); err != nil {
			return fmt.Errorf("cannot save before close: %w", err)
		}
	}

	delete(mgr.sessions, sessionID)
	return nil
}

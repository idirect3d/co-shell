// Package agent - the session-level UI window registry (FEATURE-571).
//
// A window is where render_ui paints when the target is not the chat stream:
// either a floating card in the bottom-right corner (surface=float) or a panel
// covering the main message area (surface=panel). The registry is the single
// source of truth the LLM sees through the <ui_windows> block of
// <environment_details>, so it must match what the user sees: the frontend
// reports every close/activate/surface change back with ui_window_state
// (SyncUIWindows).
//
// The registry lives on the Agent (one Agent per session) and is intentionally
// in-memory: windows are a display concern of the current session, not a
// persisted resource.
//
// Author: L.Shuang
// Created: 2026-10-01
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/log"
)

// UIWindowInfo is one window as reported by the frontend (SyncUIWindows) and as
// exported to it (a JSON-shaped snapshot). Titles and summaries are optional on
// the wire: an empty value means "keep what the agent already knows".
type UIWindowInfo struct {
	Name    string `json:"name"`
	Title   string `json:"title,omitempty"`
	Surface string `json:"surface,omitempty"`
	Active  bool   `json:"active,omitempty"`
	Summary string `json:"summary,omitempty"`
}

// uiWindowState is the agent-side record of one open window.
type uiWindowState struct {
	name    string
	title   string
	surface string
	summary string
	opened  time.Time
}

// ensureUIWindow creates the named window or updates the existing one, and makes
// it the active window. An empty title keeps the current one (the default title
// is used for a new window); an empty surface keeps the current one (float for a
// new window). Exceeding the window cap is refused with an i18n error instead of
// silently dropping the window.
func (a *Agent) ensureUIWindow(name, title, surface string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.ensureUIWindowLocked(name, title, surface)
}

func (a *Agent) ensureUIWindowLocked(name, title, surface string) error {
	if w := findUIWindow(a.uiWindows, name); w != nil {
		if title != "" {
			w.title = title
		}
		if surface != "" {
			w.surface = surface
		}
		a.uiWindowActive = name
		return nil
	}
	if len(a.uiWindows) >= UIMaxWindows {
		return fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrWindowLimit, UIMaxWindows))
	}
	if title == "" {
		title = i18n.T(i18n.KeyUIWindowDefaultTitle)
	}
	if surface == "" {
		surface = UIWindowSurfaceFloat
	}
	a.uiWindows = append(a.uiWindows, &uiWindowState{
		name:    name,
		title:   title,
		surface: surface,
		opened:  time.Now(),
	})
	a.uiWindowActive = name
	return nil
}

// closeUIWindow drops the named window and re-points the active window; it
// reports whether the window existed.
func (a *Agent) closeUIWindow(name string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	for i, w := range a.uiWindows {
		if w.name != name {
			continue
		}
		a.uiWindows = append(a.uiWindows[:i], a.uiWindows[i+1:]...)
		if a.uiWindowActive == name {
			a.uiWindowActive = ""
			// The most recently opened survivor becomes active, so the next
			// unnamed render keeps addressing a window the user can see.
			if len(a.uiWindows) > 0 {
				a.uiWindowActive = a.uiWindows[len(a.uiWindows)-1].name
			}
		}
		return true
	}
	return false
}

// setUIWindowSummary records what a window currently shows (used by the
// <ui_windows> content summary). Unknown windows are ignored.
func (a *Agent) setUIWindowSummary(name, summary string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if w := findUIWindow(a.uiWindows, name); w != nil {
		w.summary = summary
	}
}

// uiWindowTitle returns the recorded title of a window ("" when absent). It is
// what the tool receipt and the ui_window event report, so the frontend never
// has to invent a label for a window the LLM only addressed by name.
func (a *Agent) uiWindowTitle(name string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if w := findUIWindow(a.uiWindows, name); w != nil {
		return w.title
	}
	return ""
}

// uiWindowSurface returns the recorded surface of a window ("" when absent).
func (a *Agent) uiWindowSurface(name string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if w := findUIWindow(a.uiWindows, name); w != nil {
		return w.surface
	}
	return ""
}

// activeUIWindowName returns the active window name ("" when none is open).
func (a *Agent) activeUIWindowName() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.uiWindowActive
}

// SyncUIWindowsPayload applies the frontend's window list from its JSON wire
// form (the ui_window_state message). A malformed payload is logged and
// ignored: the registry is a display mirror, never a reason to fail a turn.
func (a *Agent) SyncUIWindowsPayload(raw json.RawMessage, closed []string) {
	if len(raw) == 0 {
		a.SyncUIWindows(nil, closed)
		return
	}
	var windows []UIWindowInfo
	if err := json.Unmarshal(raw, &windows); err != nil {
		log.Warn("agent: ui_window_state ignored (%v)", err)
		return
	}
	a.SyncUIWindows(windows, closed)
}

// UIWindowsSnapshot returns the open windows in creation order. It is the
// read-only view used to build <ui_windows>.
func (a *Agent) UIWindowsSnapshot() []UIWindowInfo {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]UIWindowInfo, 0, len(a.uiWindows))
	for _, w := range a.uiWindows {
		out = append(out, UIWindowInfo{
			Name:    w.name,
			Title:   w.title,
			Surface: w.surface,
			Active:  w.name == a.uiWindowActive,
			Summary: w.summary,
		})
	}
	return out
}

// SyncUIWindows mirrors the frontend's window list onto the registry
// (FEATURE-571): "closed" names are dropped, and title/surface/active changes
// are applied so <ui_windows> never disagrees with what the user sees.
// Unknown windows are created best-effort (frontend restore after a reload);
// the window cap is enforced, extras are ignored rather than duplicated.
func (a *Agent) SyncUIWindows(windows []UIWindowInfo, closed []string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, name := range closed {
		if name == "" {
			continue
		}
		for i, w := range a.uiWindows {
			if w.name != name {
				continue
			}
			a.uiWindows = append(a.uiWindows[:i], a.uiWindows[i+1:]...)
			if a.uiWindowActive == name {
				a.uiWindowActive = ""
			}
			break
		}
	}

	active := ""
	for _, in := range windows {
		if in.Name == "" {
			continue
		}
		if w := findUIWindow(a.uiWindows, in.Name); w != nil {
			if in.Title != "" {
				w.title = in.Title
			}
			if in.Surface != "" {
				w.surface = in.Surface
			}
			if in.Summary != "" {
				w.summary = in.Summary
			}
		} else if len(a.uiWindows) < UIMaxWindows {
			title := in.Title
			if title == "" {
				title = i18n.T(i18n.KeyUIWindowDefaultTitle)
			}
			a.uiWindows = append(a.uiWindows, &uiWindowState{
				name:    in.Name,
				title:   title,
				surface: in.Surface,
				summary: in.Summary,
				opened:  time.Now(),
			})
		}
		if in.Active {
			active = in.Name
		}
	}

	if active != "" {
		a.uiWindowActive = active
	}
	if a.uiWindowActive != "" && findUIWindow(a.uiWindows, a.uiWindowActive) == nil {
		a.uiWindowActive = ""
	}
	if a.uiWindowActive == "" && len(a.uiWindows) > 0 {
		a.uiWindowActive = a.uiWindows[len(a.uiWindows)-1].name
	}
}

// buildUIWindows renders the <ui_windows> block of <opened_resources>
// (FEATURE-571): one line per open window — "name | surface | active | title |
// content summary" — or "none" when nothing is open, so the LLM can decide
// between opening a new window and updating an existing one instead of
// guessing.
func (a *Agent) buildUIWindows() string {
	windows := a.UIWindowsSnapshot()
	if len(windows) == 0 {
		return "  <ui_windows>none</ui_windows>\n"
	}
	var sb strings.Builder
	sb.WriteString("  <ui_windows>\n")
	for _, w := range windows {
		active := ""
		if w.Active {
			active = "active"
		}
		sb.WriteString(fmt.Sprintf("    <window>%s | %s | %s | %s | %s</window>\n",
			w.Name, w.Surface, active, w.Title, w.Summary))
	}
	sb.WriteString("  </ui_windows>\n")
	return sb.String()
}

// findUIWindow returns the named window (nil when absent). Callers must hold a.mu.
func findUIWindow(windows []*uiWindowState, name string) *uiWindowState {
	for _, w := range windows {
		if w.name == name {
			return w
		}
	}
	return nil
}

// uiTreeSummary labels a rendered tree for the <ui_windows> content summary,
// e.g. "card(ui-7)". It stays language neutral: it is machine-readable context
// for the LLM, not user-facing text.
func uiTreeSummary(root *UINode, id string) string {
	if root == nil {
		return ""
	}
	return fmt.Sprintf("%s(%s)", root.Type, id)
}

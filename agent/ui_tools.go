// Package agent - the render_ui tool: LLM-driven rich component output
// (FEATURE-524).
//
// render_ui lets the LLM answer with a component tree (cards, tables, charts,
// steps, ...) instead of a wall of text. The tool itself only validates the
// tree and parks it on the agent; the stream loop (the only owner of the
// StreamCallback) turns it into a ui_render event that the Web UI paints.
// Terminals and other frontends ignore the event and keep the plain-text
// reply, so no frontend breaks when the LLM uses this tool.
//
// Author: L.Shuang
// Created: 2026-09-15
// MIT License - Copyright (c) 2026 L.Shuang

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"unicode"

	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/llm"
)

// render_ui targets (FEATURE-524 window mode): where the tree is painted.
// FEATURE-571 adds UITargetPanel: a window that covers the main message area
// instead of floating in a corner.
const (
	// UITargetStream is the default surface: a block in the chat stream.
	UITargetStream = "stream"
	// UITargetWindow paints into the floating window opened by ui_window.
	UITargetWindow = "window"
	// UITargetPanel paints into the main-area overlay panel.
	UITargetPanel = "panel"
)

// ui_window actions.
const (
	// UIWindowOpen shows the window, or creates/updates the named one.
	UIWindowOpen = "open"
	// UIWindowClose dismisses it.
	UIWindowClose = "close"
	// UIWindowActivate brings an open window to the front (FEATURE-571).
	UIWindowActivate = "activate"
)

// ui_window surfaces (FEATURE-571): where a window lives on screen. The
// surface is chosen on open and can be flipped later by rendering with the
// matching target (panel <-> float).
const (
	// UIWindowSurfaceFloat is the default: a fixed bottom-right floating window.
	UIWindowSurfaceFloat = "float"
	// UIWindowSurfacePanel covers the main message area.
	UIWindowSurfacePanel = "panel"
)

// Window registry bounds (FEATURE-571).
const (
	// UIDefaultWindowName addresses the window when the LLM names none.
	UIDefaultWindowName = "default"
	// UIMaxWindows bounds how many windows one session may keep open at once.
	UIMaxWindows = 8
	// UIMaxWindowName bounds a window name in runes.
	UIMaxWindowName = 32
)

// ui_window size presets (FEATURE-524). A preset is a fraction of the usable
// viewport (viewport minus the 32px margin kept by the fixed bottom-right
// position), so the same preset yields different pixels on different screens
// and can never overflow. Auto keeps the original content-driven behaviour.
const (
	// UIWindowSizeAuto is the default: width min(420, viewport-32), height
	// driven by the content and capped by CSS at min(70vh, 640px).
	UIWindowSizeAuto = "auto"
	// UIWindowSizeSmall is half of the usable viewport.
	UIWindowSizeSmall = "small"
	// UIWindowSizeMedium is two thirds of the usable viewport.
	UIWindowSizeMedium = "medium"
	// UIWindowSizeLarge fills the usable viewport.
	UIWindowSizeLarge = "large"
)

// UIMaxWindowTitle bounds the window title in runes. A title is a UI label, not
// a payload: an over-long one is refused so it cannot flood the event or the
// title bar.
const UIMaxWindowTitle = 60

// uiIDSeq numbers the component trees rendered by this process. Ids only need
// to be unique within one page (they address a node in the Web UI DOM), so a
// process-wide counter is enough and needs no coordination across sessions.
var uiIDSeq uint64

// uiEnabled reports whether the render_ui tool is exposed to the LLM. The
// switch defaults to on: a nil config (tests, embedders) counts as enabled.
func (a *Agent) uiEnabled() bool {
	return a.cfg == nil || a.cfg.UIEnabled
}

// buildRenderUITool declares the render_ui tool. The full component catalogue
// lives in the localized usage section (KeyUIToolUsageRenderUI), which is both
// the XML-mode system prompt section and the OpenAI-mode tool description, so
// the two modes cannot drift apart.
func (a *Agent) buildRenderUITool() llm.Tool {
	return llm.Tool{
		Name:        "render_ui",
		Description: i18n.T(i18n.KeyUIToolUsageRenderUI),
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"meta": map[string]interface{}{
					"type":        "object",
					"description": a.toolMetaDesc(),
				},
				"tree": map[string]interface{}{
					"type":        "object",
					"description": "Component tree root node {type, id?, props?, children?, actions?}. See the render_ui section of the system prompt for the component catalogue and per-component props.",
				},
				"update": map[string]interface{}{
					"type":        "string",
					"description": i18n.T(i18n.KeyUIToolParamUpdate),
				},
				"target": map[string]interface{}{
					"type":        "string",
					"description": i18n.T(i18n.KeyUIToolParamTarget),
					"enum":        []string{UITargetStream, UITargetWindow, UITargetPanel},
				},
				"window": map[string]interface{}{
					"type":        "string",
					"description": i18n.T(i18n.KeyUIToolParamWindowName),
				},
				"waiting": map[string]interface{}{
					"type":        "boolean",
					"description": "false (default): return immediately; a later user action starts a new turn. true: keep the tool call open until the user acts, and return that action as this tool's result.",
				},
			},
			"required": []string{"meta", "tree"},
		},
		Callback: a.renderUITool,
	}
}

// renderUITool validates the component tree, parks it for the stream loop and
// returns a one-line receipt. The tree is deliberately NOT echoed back: the
// LLM already wrote it, so returning it would only inflate the context.
func (a *Agent) renderUITool(ctx context.Context, args map[string]interface{}) (string, error) {
	raw, ok := args["tree"]
	if !ok || raw == nil {
		return "", fmt.Errorf("%s", i18n.T(i18n.KeyUIErrTreeEmpty))
	}
	treeJSON, err := uiTreeArgJSON(raw)
	if err != nil {
		return "", err
	}
	root, err := ParseUITree(treeJSON)
	if err != nil {
		return "", err
	}

	// FEATURE-524 window mode: target selects the surface the tree is painted
	// on; an unsupported value is refused before anything is parked.
	surface, err := uiRenderTarget(args)
	if err != nil {
		return "", err
	}

	// FEATURE-571: a window target also carries the window name. An empty name
	// addresses the current window, falling back to the default one; the window
	// is registered here so <ui_windows> lists what is being painted into and so
	// an unnamed render followed by a named one keeps addressing the same
	// window.
	windowName := ""
	if winSurface := uiSurfaceOfTarget(surface); winSurface != "" {
		name, nameErr := uiWindowNameArg(args, false)
		if nameErr != nil {
			return "", nameErr
		}
		if name == "" {
			name = a.activeUIWindowName()
		}
		if name == "" {
			name = UIDefaultWindowName
		}
		if ensureErr := a.ensureUIWindow(name, "", winSurface); ensureErr != nil {
			return "", ensureErr
		}
		windowName = name
	}

	if target := uiUpdateTarget(args); target != "" {
		a.mu.Lock()
		a.pendingUIUpdateID = target
		a.pendingUIUpdateNode = root
		a.pendingUITarget = surface
		a.pendingUIWindowName = windowName
		a.mu.Unlock()
		return i18n.TF(i18n.KeyUIUpdateSummary, target, CountUINodes(root)), nil
	}

	id := fmt.Sprintf("ui-%d", atomic.AddUint64(&uiIDSeq, 1))
	a.mu.Lock()
	a.pendingUITree = root
	a.pendingUIID = id
	a.pendingUITarget = surface
	a.pendingUIWindowName = windowName
	a.mu.Unlock()

	// FEATURE-571: record what the window currently shows so <ui_windows> can
	// tell the LLM which window holds which result.
	if windowName != "" {
		a.setUIWindowSummary(windowName, uiTreeSummary(root, id))
	}

	// waiting=true parks the call until the user acts on a rendered component:
	// the action then becomes the tool result, so the agent continues in the
	// same turn with the structured values the user chose (FEATURE-524, UC-32).
	// A second concurrent wait is refused by beginUIWait and degrades to the
	// normal receipt rather than racing for the same user action.
	if waiting, _ := args["waiting"].(bool); waiting {
		if ch := a.beginUIWait(); ch != nil {
			defer a.endUIWait(ch)
			// The tree must reach the frontend now. The stream loop emits the
			// parked tree after this call returns, but a waiting call returns
			// only when the user acts, so emitting there would keep the
			// component invisible for the whole wait (it then surfaced only
			// after an interrupt).
			a.emitUIRender()
			return a.waitUIAction(ctx, ch), nil
		}
	}
	return UISummary(root, id), nil
}

// takePendingUITree returns and clears the tree parked by the most recent
// render_ui call. It gives the stream loop exclusive ownership of the tree so
// the same render is emitted exactly once.
func (a *Agent) takePendingUITree() (*UINode, string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	root, id := a.pendingUITree, a.pendingUIID
	a.pendingUITree, a.pendingUIID = nil, ""
	return root, id
}

// takePendingUIUpdate returns and clears the in-place update parked by the most
// recent render_ui call that carried an update target. It is the sibling of
// takePendingUITree: a call parks exactly one of the two, never both, so the
// stream loop emits exactly one event per call.
func (a *Agent) takePendingUIUpdate() (string, *UINode) {
	a.mu.Lock()
	defer a.mu.Unlock()
	target, node := a.pendingUIUpdateID, a.pendingUIUpdateNode
	a.pendingUIUpdateID, a.pendingUIUpdateNode = "", nil
	return target, node
}

// uiUpdateTarget reads the optional update target of a render_ui call. An empty
// result means "render a new tree" (the default).
func uiUpdateTarget(args map[string]interface{}) string {
	s, ok := args["update"].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

// UIActionMessage renders a component interaction as the user message that
// starts the next agent turn (FEATURE-524). The payload is inlined as JSON so
// the LLM receives the exact structured values the user provided, without the
// frontend having to invent prose for them.
func UIActionMessage(uiID, actionID string, payload json.RawMessage) string {
	p := strings.TrimSpace(string(payload))
	if p == "" || p == "null" {
		p = "{}"
	}
	return i18n.TF(i18n.KeyUIUserAction, uiID, actionID, p)
}

// takePendingUITarget returns and clears the surface the parked tree/update
// belongs to: UITargetStream (the chat stream) or UITargetWindow. The stream
// loop reads it right after consuming the parked tree.
func (a *Agent) takePendingUITarget() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	target := a.pendingUITarget
	a.pendingUITarget = ""
	return target
}

// setUIEmitter installs (nil clears) the live stream callback the render_ui
// tool publishes through. RunStream owns that callback, so it installs it for
// the duration of a turn and clears it afterwards.
func (a *Agent) setUIEmitter(cb StreamCallback) {
	a.mu.Lock()
	a.uiEmit = cb
	a.mu.Unlock()
}

// takeUIRenderEvent turns whatever render_ui parked into the event that must be
// sent and consumes it. It is the single place that builds such an event, so
// the two emitters (the tool, before parking a wait, and the stream loop, after
// a call returns) can never send the same tree twice. ok is false when nothing
// is parked, which is exactly what the loop sees after the tool published it.
func (a *Agent) takeUIRenderEvent() (StreamEvent, bool) {
	if target, node := a.takePendingUIUpdate(); node != nil {
		if patchJSON, err := MarshalUITree(node); err == nil {
			ev := UIUpdateEventTo(target, patchJSON, a.takePendingUITarget())
			return withUIWindowName(ev, a.takePendingUIWindowName()), true
		}
		return StreamEvent{}, false
	}
	if root, uiID := a.takePendingUITree(); root != nil {
		if treeJSON, err := MarshalUITree(root); err == nil {
			ev := UIRenderEventTo(uiID, treeJSON, a.takePendingUITarget())
			return withUIWindowName(ev, a.takePendingUIWindowName()), true
		}
	}
	return StreamEvent{}, false
}

// takePendingUIWindowName returns and clears the window name the parked tree or
// update belongs to (FEATURE-571). An empty result means the default window.
func (a *Agent) takePendingUIWindowName() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	name := a.pendingUIWindowName
	a.pendingUIWindowName = ""
	return name
}

// emitUIRender publishes the parked tree immediately, using the callback the
// stream loop installed. It reports whether an event was emitted; with no
// callback (unit tests, embedders) nothing is consumed and the loop's own
// post-call emission still delivers it.
func (a *Agent) emitUIRender() bool {
	a.mu.Lock()
	emit := a.uiEmit
	a.mu.Unlock()
	if emit == nil {
		return false
	}
	ev, ok := a.takeUIRenderEvent()
	if !ok {
		return false
	}
	emit(ev)
	return true
}

// uiRenderTarget reads the optional target of a render_ui call. An empty value
// means the default chat stream; an unsupported value is an error so the LLM
// gets told instead of silently rendering into the wrong surface.
func uiRenderTarget(args map[string]interface{}) (string, error) {
	raw, ok := args["target"]
	if !ok || raw == nil {
		return UITargetStream, nil
	}
	s, isStr := raw.(string)
	if !isStr {
		return "", fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrTarget, fmt.Sprint(raw)))
	}
	switch target := strings.ToLower(strings.TrimSpace(s)); target {
	case "", UITargetStream:
		return UITargetStream, nil
	case UITargetWindow:
		return UITargetWindow, nil
	case UITargetPanel:
		return UITargetPanel, nil
	default:
		return "", fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrTarget, s))
	}
}

// uiSurfaceOfTarget maps a render_ui target to the window surface it paints
// into: the chat stream needs no window, window floats and panel covers the
// main message area (FEATURE-571).
func uiSurfaceOfTarget(target string) string {
	switch target {
	case UITargetWindow:
		return UIWindowSurfaceFloat
	case UITargetPanel:
		return UIWindowSurfacePanel
	default:
		return ""
	}
}

// uiWindowNameArg reads and validates the optional window name. When def is
// true an empty name falls back to the default window; otherwise an empty
// result means "the current window" and is resolved by the caller.
func uiWindowNameArg(args map[string]interface{}, def bool) (string, error) {
	name := strings.TrimSpace(uiStringArg(args, "window"))
	if name == "" {
		if def {
			return UIDefaultWindowName, nil
		}
		return "", nil
	}
	if err := validateUIWindowName(name); err != nil {
		return "", err
	}
	return name, nil
}

// validateUIWindowName rejects names that would break the <ui_windows> list or
// the tab label: a rune budget plus a whitelist (letters, digits, CJK, space,
// dash, underscore, dot) so no control character, markup or quote can enter the
// registry.
func validateUIWindowName(name string) error {
	if n := len([]rune(name)); n == 0 || n > UIMaxWindowName {
		return fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrWindowName, UIMaxWindowName))
	}
	for _, r := range name {
		switch {
		case r == ' ' || r == '-' || r == '_' || r == '.':
		case r >= '0' && r <= '9', r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
		case unicode.IsLetter(r) && unicode.Is(unicode.Han, r):
		default:
			return fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrWindowName, UIMaxWindowName))
		}
	}
	return nil
}

// normalizeUIWindowSurface folds a ui_window surface argument to a known
// surface. An empty value means "keep the current surface" (float for a new
// window), so the caller can tell "not given" from "float".
func normalizeUIWindowSurface(s string) (string, error) {
	raw := s
	switch surface := strings.ToLower(strings.TrimSpace(s)); surface {
	case "":
		return "", nil
	case UIWindowSurfaceFloat, UIWindowSurfacePanel:
		return surface, nil
	default:
		return "", fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrWindowSurface, raw))
	}
}

// uiSurfaceLabel renders a surface for a tool receipt: an empty surface ("keep
// the current one") reads as the default float.
func uiSurfaceLabel(surface string) string {
	if surface == "" {
		return UIWindowSurfaceFloat
	}
	return surface
}

// uiWindowRequest is one parked ui_window call (FEATURE-524, extended by
// FEATURE-571 with the addressed window and its surface): the action to
// perform, the target window name, the title, the size preset and the surface.
// Title, size and surface stay empty for close/activate.
type uiWindowRequest struct {
	Action  string
	Name    string
	Title   string
	Size    string
	Surface string
}

// buildUIWindowTool declares the ui_window tool (FEATURE-524 window mode). It
// only opens/closes the floating window: the content is written by render_ui
// with target="window", so the window stays a surface the LLM can keep pushing
// into while it works, instead of a one-shot payload.
func (a *Agent) buildUIWindowTool() llm.Tool {
	return llm.Tool{
		Name:        "ui_window",
		Description: i18n.T(i18n.KeyUIToolUsageUIWindow),
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"meta": map[string]interface{}{
					"type":        "object",
					"description": a.toolMetaDesc(),
				},
				"action": map[string]interface{}{
					"type":        "string",
					"description": "open: show or create the window; close: dismiss it; activate: bring an already open window to the front.",
					"enum":        []string{UIWindowOpen, UIWindowClose, UIWindowActivate},
				},
				"window": map[string]interface{}{
					"type":        "string",
					"description": i18n.T(i18n.KeyUIToolParamWindowName),
				},
				"title": map[string]interface{}{
					"type":        "string",
					"description": i18n.T(i18n.KeyUIToolParamWindowTitle),
				},
				"surface": map[string]interface{}{
					"type":        "string",
					"description": i18n.T(i18n.KeyUIToolParamWindowSurface),
					"enum":        []string{UIWindowSurfaceFloat, UIWindowSurfacePanel},
				},
				"size": map[string]interface{}{
					"type":        "string",
					"description": i18n.T(i18n.KeyUIToolParamWindowSize),
					"enum":        []string{UIWindowSizeAuto, UIWindowSizeSmall, UIWindowSizeMedium, UIWindowSizeLarge},
				},
			},
			"required": []string{"meta", "action"},
		},
		Callback: a.uiWindowTool,
	}
}

// uiWindowTool validates a ui_window call, parks it for the stream loop (the
// only owner of the StreamCallback) and returns a one-line receipt.
func (a *Agent) uiWindowTool(ctx context.Context, args map[string]interface{}) (string, error) {
	rawAction := strings.TrimSpace(uiStringArg(args, "action"))
	action := strings.ToLower(rawAction)
	switch action {
	case UIWindowOpen, UIWindowClose, UIWindowActivate:
	default:
		return "", fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrWindowAction, rawAction))
	}

	// FEATURE-571: window names which window the action applies to. An empty
	// name addresses the current window (the default one for open).
	name, err := uiWindowNameArg(args, action == UIWindowOpen)
	if err != nil {
		return "", err
	}
	if name == "" {
		if name = a.activeUIWindowName(); name == "" {
			name = UIDefaultWindowName
		}
	}

	title := strings.TrimSpace(uiStringArg(args, "title"))
	if n := len([]rune(title)); n > UIMaxWindowTitle {
		return "", fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrWindowTitle, UIMaxWindowTitle))
	}

	size, err := normalizeUIWindowSize(uiStringArg(args, "size"))
	if err != nil {
		return "", err
	}
	surface, err := normalizeUIWindowSurface(uiStringArg(args, "surface"))
	if err != nil {
		return "", err
	}

	if action == UIWindowClose {
		// The title, size and surface only belong to an open window.
		title, size, surface = "", "", ""
	}

	// FEATURE-571: the registry mirrors what the user sees. A close takes
	// effect at once (the frontend's own ✕ needs nothing more), while
	// open/activate are parked for the stream loop, the only owner of the
	// StreamCallback.
	// Closing is idempotent on purpose: a window that is already gone (the user
	// clicked ✕, or the page was refreshed) must not turn a defensive close into
	// an error, and the event still tells the frontend to drop whatever it shows.
	if action == UIWindowClose {
		a.closeUIWindow(name)
	} else if err := a.ensureUIWindow(name, title, surface); err != nil {
		return "", err
	} else if surface == "" {
		// The receipt and the event must carry the effective surface, so the
		// frontend can place the window even when the caller omitted it.
		surface = a.uiWindowSurface(name)
	}

	// The registry owns the effective title (an open without a title keeps the
	// current one), so read it before taking the lock: a.mu is not reentrant.
	effectiveTitle := a.uiWindowTitle(name)

	a.mu.Lock()
	a.pendingUIWindow = &uiWindowRequest{Action: action, Name: name, Title: effectiveTitle, Size: size, Surface: surface}
	a.mu.Unlock()

	switch action {
	case UIWindowClose:
		if name == UIDefaultWindowName {
			return i18n.T(i18n.KeyUIWindowCloseSummary), nil
		}
		return i18n.TF(i18n.KeyUIWindowCloseNamedSummary, name), nil
	case UIWindowActivate:
		return i18n.TF(i18n.KeyUIWindowActivateSummary, name), nil
	}
	// The default window keeps the pre-v0.85 receipt; a named one (or an
	// explicit surface) reports the name and the surface it now has.
	if name == UIDefaultWindowName && surface == "" {
		return i18n.TF(i18n.KeyUIWindowOpenSummary, title, size), nil
	}
	return i18n.TF(i18n.KeyUIWindowOpenNamedSummary, name, title, uiSurfaceLabel(surface), size), nil
}

// normalizeUIWindowSize folds a ui_window size argument to a known preset; an
// empty value means the default (auto). The backend validates the shape only:
// the pixels are decided by the frontend, the only side that knows the
// viewport.
func normalizeUIWindowSize(s string) (string, error) {
	raw := s
	switch size := strings.ToLower(strings.TrimSpace(s)); size {
	case "":
		return UIWindowSizeAuto, nil
	case UIWindowSizeAuto, UIWindowSizeSmall, UIWindowSizeMedium, UIWindowSizeLarge:
		return size, nil
	default:
		return "", fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrWindowSize, raw))
	}
}

// takePendingUIWindow returns and clears the window action parked by the most
// recent ui_window call, so the stream loop emits it exactly once.
func (a *Agent) takePendingUIWindow() *uiWindowRequest {
	a.mu.Lock()
	defer a.mu.Unlock()
	req := a.pendingUIWindow
	a.pendingUIWindow = nil
	return req
}

// uiStringArg reads a string argument, tolerating a missing or non-string value.
func uiStringArg(args map[string]interface{}, name string) string {
	s, _ := args[name].(string)
	return s
}

// uiTreeArgJSON normalizes the tree argument to JSON text. Providers without a
// native object type may hand the tree over as an already-encoded JSON string,
// so both shapes are accepted.
func uiTreeArgJSON(raw interface{}) (string, error) {
	if s, ok := raw.(string); ok {
		return s, nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return "", fmt.Errorf("%s", i18n.TF(i18n.KeyUIErrTreeDecode, err.Error()))
	}
	return string(b), nil
}

// Package web - embedded HTTP server for `co-shell serve` (FEATURE-307c):
// static page (embed.FS), the WebSocket endpoint with a single-client hub
// (a new connection replaces the old one), and the workspace-scoped HTTP
// APIs (directory tree, upload, open, reveal, file read). Loopback only;
// every filesystem path is validated to stay inside the workspace root.
//
// Author: L.Shuang
// Created: 2026-08-17
// Last Modified: 2026-08-17
// MIT License - Copyright (c) 2026 L.Shuang

package web

import (
	"bufio"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/cmd"
	"github.com/idirect3d/co-shell/i18n"
)

//go:embed static
var staticFS embed.FS

// maxUploadFileSize caps one uploaded file (bytes).
const maxUploadFileSize = 100 << 20 // 100 MiB

// treeExcludedNames are directory entries hidden from the workspace tree
// (noise / internals). output/ is intentionally kept.
var treeExcludedNames = map[string]bool{
	".git":         true,
	"node_modules": true,
	"db":           true,
	"log":          true,
	"tmp":          true,
}

// errPathOutside marks any path that would escape the workspace root.
var errPathOutside = errors.New("path escapes workspace")

// clientMessage is a browser-to-server WebSocket message.
type clientMessage struct {
	Type        string   `json:"type"` // "input" | "answer" | "interaction_answer" | "interrupt" | "session_list" | "session_switch" | "session_delete" | "session_new" | "settings_get" | "settings_set" | "identity_get" | "identity_set" | "restart"
	Text        string   `json:"text,omitempty"`
	Attachments []string `json:"attachments,omitempty"`
	ID          string   `json:"id,omitempty"`    // answer: the ask id; interaction_answer: the interaction id
	Value       string   `json:"value,omitempty"` // answer: the reply; session_switch/delete: the session id; settings_set: the new value
	Key         string   `json:"key,omitempty"`   // settings_set: the setting key
	Priority    int      `json:"priority,omitempty"` // model_set_priority: the new priority
	Result      *interactionResultJSON `json:"result,omitempty"` // interaction_answer: the structured result
	Step        string   `json:"step,omitempty"`   // model_wizard_next/prev: the current wizard step
	WizardData  json.RawMessage `json:"wizard_data,omitempty"` // model_wizard_next/prev/submit: accumulated wizard data
	YOLO        bool     `json:"yolo,omitempty"`   // yolo_set: the new YOLO mode state
}

// interactionResultJSON is the wire form of an agent.InteractionResult.
type interactionResultJSON struct {
	Action string `json:"action"`
	Value  string `json:"value,omitempty"`
	Raw    string `json:"raw,omitempty"`
}

// eventJSON is the wire form of an agent.StreamEvent (same field rules as
// the JSON-Lines StreamRenderer: level only when not info).
type eventJSON struct {
	Type  string            `json:"type"`
	Level string            `json:"level,omitempty"`
	Chan  string            `json:"chan,omitempty"`
	Text  string            `json:"text,omitempty"`
	Meta  map[string]string `json:"meta,omitempty"`
}

// serverMessage is a server-to-browser WebSocket message.
type serverMessage struct {
	Kind        string          `json:"kind"`            // "event" | "ask" | "interaction" | "state" | "sessions" | "settings" | "settings_result" | "identity" | "identity_result"
	Event       *eventJSON      `json:"event,omitempty"` // kind=event
	ID          string          `json:"id,omitempty"`    // kind=ask / kind=interaction
	Mode        string          `json:"mode,omitempty"`  // kind=ask: "line" | "key"
	Interaction json.RawMessage `json:"interaction,omitempty"` // kind=interaction: the Interaction JSON
	Plan        json.RawMessage `json:"plan"`            // kind=state (null when no plan)
	Sessions    []sessionInfo   `json:"sessions,omitempty"` // kind=sessions: the session list
	Settings    json.RawMessage `json:"settings,omitempty"` // kind=settings: the grouped setting items
	Identity    json.RawMessage `json:"identity,omitempty"` // kind=identity: the identity fields
	OK          bool            `json:"ok,omitempty"`    // kind=settings_result: success flag
	Message     string          `json:"message,omitempty"` // kind=settings_result: result message
	Modes       []modeInfo      `json:"modes,omitempty"` // kind=mode: the work mode list
	Models      json.RawMessage `json:"models,omitempty"` // kind=models: the model list JSON
	Templates   json.RawMessage `json:"templates,omitempty"` // kind=models: the template list JSON
	WizardStep  json.RawMessage `json:"wizard_step,omitempty"` // kind=model_wizard: the wizard step form JSON
	WizardData  json.RawMessage `json:"wizard_data,omitempty"` // kind=model_wizard: the accumulated wizard data JSON
	YOLO        bool            `json:"yolo,omitempty"`    // kind=yolo: the current YOLO mode state
}

// modeInfo is one work mode entry pushed to the browser for the mode
// switcher (FEATURE-410).
type modeInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Current     bool   `json:"current"`
}

// sessionInfo is one entry in the session list pushed to the browser.
type sessionInfo struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Keywords     string `json:"keywords"`
	CreatedAt    string `json:"created_at"`
	Current      bool   `json:"current"`
	MessageCount int    `json:"message_count"` // number of messages (FEATURE-428)
}

// ServerOptions carries the display parameters of a Server.
type ServerOptions struct {
	Lang    string // UI language ("zh"/"en"), handed to the frontend
	Version string
	Build   string
	Bind    string // listen address (default "127.0.0.1")
	// Whitelist restricts access to the given IPs/CIDR networks (e.g.
	// "192.168.1.100" or "192.168.1.0/24"). Empty means loopback only.
	Whitelist []string

	// DownloadEnabled (FEATURE-455): when true, remote web UI access may
	// download workspace files via the browser. Default off (security).
	DownloadEnabled bool
}

// Server is the embedded web server: HTTP routes + the single-client
// WebSocket hub. The WebSession registers itself via SetMessageHandler to
// receive browser input; the renderer/WebIO push events through sendJSON.
type Server struct {
	root string
	opts ServerOptions

	mux     *http.ServeMux
	httpSrv *http.Server
	ln      net.Listener

	mu      sync.Mutex
	conn    *wsConn // current browser client (single-client hub)
	handler func(clientMessage)

	// onDisconnect is called when the current client disconnects (pending
	// ask requests must fail instead of blocking forever).
	onDisconnect func()

	// planFn returns the current task plan as a JSON string ("" when none),
	// pushed to the browser on connect as a state message.
	planFn func() string

	// modelInfoFn returns the active text/vision model context info for the
	// status bar (FEATURE-378); nil when no provider is registered.
	modelInfoFn func() agent.ModelInfo
}

// NewServer creates the server for the given workspace root and registers
// all HTTP routes. The static frontend is embedded.
func NewServer(root string, opts ServerOptions) *Server {
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	s := &Server{root: abs, opts: opts, mux: http.NewServeMux()}
	s.mux.HandleFunc("GET /{$}", s.handleIndex)
	s.mux.Handle("GET /static/", http.FileServer(http.FS(staticFS)))
	s.mux.HandleFunc("GET /ws", s.handleWS)
	s.mux.HandleFunc("GET /api/bootstrap", s.handleBootstrap)
	s.mux.HandleFunc("GET /api/tree", s.handleTree)
	s.mux.HandleFunc("POST /api/upload", s.handleUpload)
	s.mux.HandleFunc("POST /api/open", s.handleOpen)
	s.mux.HandleFunc("POST /api/reveal", s.handleReveal)
	s.mux.HandleFunc("GET /api/download", s.handleDownload)
	s.mux.HandleFunc("GET /api/file", s.handleFile)
	s.mux.HandleFunc("GET /api/gitdiff", s.handleGitDiff)
	s.mux.HandleFunc("POST /api/test-endpoint", s.handleTestEndpoint)
	s.mux.HandleFunc("POST /api/test-api-key", s.handleTestAPIKey)
	s.mux.HandleFunc("POST /api/get-model-max-len", s.handleGetModelMaxLen)
	handler := http.Handler(s.mux)
	if len(opts.Whitelist) > 0 {
		handler = s.whitelistMiddleware(handler, opts.Whitelist)
	}
	s.httpSrv = &http.Server{Handler: handler}
	return s
}

// whitelistMiddleware rejects requests whose client IP is not in the given
// whitelist (IPs or CIDR networks). It wraps the mux so every route (static,
// API, WebSocket) is protected.
func (s *Server) whitelistMiddleware(next http.Handler, whitelist []string) http.Handler {
	nets := parseWhitelist(whitelist)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !ipAllowed(r.RemoteAddr, nets) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// parseWhitelist converts whitelist entries (IPs or CIDR networks) into
// net.IPNet ranges. A bare IP becomes a /32 (or /128) host range.
func parseWhitelist(entries []string) []*net.IPNet {
	var nets []*net.IPNet
	for _, e := range entries {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		if strings.Contains(e, "/") {
			if _, ipnet, err := net.ParseCIDR(e); err == nil {
				nets = append(nets, ipnet)
			}
			continue
		}
		if ip := net.ParseIP(e); ip != nil {
			bits := 32
			if ip.To4() == nil {
				bits = 128
			}
			nets = append(nets, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
		}
	}
	return nets
}

// ipAllowed reports whether the client address ("host:port") is within any of
// the whitelist networks. A nil/empty network list denies everything.
func ipAllowed(remoteAddr string, nets []*net.IPNet) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// SetMessageHandler installs the dispatcher for browser messages (called by
// the WebSession at creation time).
func (s *Server) SetMessageHandler(fn func(clientMessage)) {
	s.mu.Lock()
	s.handler = fn
	s.mu.Unlock()
}

// SetDisconnectHook installs the client-disconnect callback.
func (s *Server) SetDisconnectHook(fn func()) {
	s.mu.Lock()
	s.onDisconnect = fn
	s.mu.Unlock()
}

// SetPlanProvider installs the task-plan snapshot provider.
func (s *Server) SetPlanProvider(fn func() string) {
	s.mu.Lock()
	s.planFn = fn
	s.mu.Unlock()
}

// SetModelInfoProvider installs the active model context-info provider
// (FEATURE-378), used by the status bar to show context usage.
func (s *Server) SetModelInfoProvider(fn func() agent.ModelInfo) {
	s.mu.Lock()
	s.modelInfoFn = fn
	s.mu.Unlock()
}

// Listen binds the configured address (default 127.0.0.1) on the given port
// (auto-incrementing up to 10 ports when occupied) and serves HTTP in the
// background. It returns the bound "host:port" address.
func (s *Server) Listen(port int) (string, error) {
	bind := s.opts.Bind
	if bind == "" {
		bind = "127.0.0.1"
	}
	for i := 0; i < 10; i++ {
		p := port + i
		ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", bind, p))
		if err != nil {
			continue
		}
		s.ln = ln
		go func() { _ = s.httpSrv.Serve(ln) }()
		return fmt.Sprintf("%s:%d", bind, p), nil
	}
	return "", fmt.Errorf("ports %d-%d are all in use", port, port+9)
}

// Close shuts the server down and drops the current client.
func (s *Server) Close() error {
	s.setConn(nil)
	if s.ln != nil {
		return s.ln.Close()
	}
	return nil
}

// ---------------------------------------------------------------------------
// Single-client WebSocket hub
// ---------------------------------------------------------------------------

// setConn replaces the current client; the old connection is closed (a
// freshly opened browser tab takes over the session).
func (s *Server) setConn(c *wsConn) {
	s.mu.Lock()
	old := s.conn
	s.conn = c
	onDisc := s.onDisconnect
	s.mu.Unlock()
	if old != nil {
		old.Close()
		if onDisc != nil {
			onDisc()
		}
	}
}

// clearConn drops the client if it is still the current one.
func (s *Server) clearConn(c *wsConn) {
	s.mu.Lock()
	if s.conn == c {
		s.conn = nil
	}
	onDisc := s.onDisconnect
	s.mu.Unlock()
	if onDisc != nil {
		onDisc()
	}
}

// sendJSON marshals v and writes it to the current client. It returns false
// when no browser is connected.
func (s *Server) sendJSON(v serverMessage) bool {
	data, err := json.Marshal(v)
	if err != nil {
		return false
	}
	s.mu.Lock()
	c := s.conn
	s.mu.Unlock()
	if c == nil {
		return false
	}
	return c.WriteMessage(data) == nil
}

// sendEvent pushes one agent stream event to the browser.
func (s *Server) sendEvent(ev agent.StreamEvent) bool {
	ej := &eventJSON{Type: ev.Type, Text: ev.Text, Meta: ev.Meta}
	if ev.Level != agent.LevelInfo {
		ej.Level = ev.Level.String()
	}
	if ev.Chan != "" {
		ej.Chan = string(ev.Chan)
	}
	return s.sendJSON(serverMessage{Kind: "event", Event: ej})
}

// sendAsk pushes an interactive question request to the browser.
func (s *Server) sendAsk(id, mode string) bool {
	return s.sendJSON(serverMessage{Kind: "ask", ID: id, Mode: mode})
}

// sendInteraction pushes a structured interaction request to the browser.
// inJSON is the marshaled agent.Interaction payload (FEATURE-388).
func (s *Server) sendInteraction(id string, inJSON json.RawMessage) bool {
	return s.sendJSON(serverMessage{Kind: "interaction", ID: id, Interaction: inJSON})
}

// sendState pushes the current task plan snapshot (null when none) to one
// freshly connected client.
func (s *Server) sendState(c *wsConn) {
	s.mu.Lock()
	fn := s.planFn
	s.mu.Unlock()
	plan := json.RawMessage("null")
	if fn != nil {
		if p := fn(); p != "" {
			plan = json.RawMessage(p)
		}
	}
	data, err := json.Marshal(serverMessage{Kind: "state", Plan: plan})
	if err != nil {
		return
	}
	_ = c.WriteMessage(data)
}

// dispatch routes one browser message to the registered handler.
func (s *Server) dispatch(raw []byte) {
	var msg clientMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return
	}
	s.mu.Lock()
	h := s.handler
	s.mu.Unlock()
	if h != nil {
		h(msg)
	}
}

// handleWS upgrades the connection, registers it as the current client,
// pushes the initial state, then reads messages until disconnect.
func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	c, err := upgrade(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.setConn(c)
	s.sendState(c)
	defer s.clearConn(c)
	for {
		msg, err := c.ReadMessage()
		if err != nil {
			return
		}
		s.dispatch(msg)
	}
}

// ---------------------------------------------------------------------------
// HTTP API handlers (all filesystem paths are workspace-scoped)
// ---------------------------------------------------------------------------

// resolvePath maps a workspace-relative path to an absolute path, rejecting
// anything that would escape the workspace root (".." traversal).
func (s *Server) resolvePath(rel string) (string, error) {
	abs := filepath.Join(s.root, filepath.FromSlash(rel))
	r, err := filepath.Rel(s.root, abs)
	if err != nil || r == ".." || strings.HasPrefix(r, ".."+string(filepath.Separator)) {
		return "", errPathOutside
	}
	return abs, nil
}

// writeJSON responds with a JSON body.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	data, err := staticFS.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "index not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

func (s *Server) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	payload := map[string]interface{}{
		"lang":            s.opts.Lang,
		"version":         s.opts.Version,
		"build":           s.opts.Build,
		"workspace":       s.root,
		"branch":          gitBranch(s.root),
		"remote":          s.isRemote(),
		"downloadEnabled": s.downloadEnabled(),
	}
	// FEATURE-378: expose the active text/vision model context info for the
	// status bar's context-usage display.
	s.mu.Lock()
	fn := s.modelInfoFn
	s.mu.Unlock()
	if fn != nil {
		info := fn()
		payload["textModel"] = info.TextModelName
		payload["textMaxLen"] = info.TextMaxLen
		payload["visionModel"] = info.VisionModelName
		payload["visionMaxLen"] = info.VisionMaxLen
		// Current mode's bound model IDs (empty = no binding, uses global
		// default). Lets the web UI highlight the "默认" option (FEATURE-422).
		payload["modeTextModelID"] = info.ModeTextModelID
		payload["modeVisionModelID"] = info.ModeVisionModelID
	}
	writeJSON(w, http.StatusOK, payload)
}

// gitBranch returns the current git branch of the workspace root, or "" when
// the workspace is not inside a git repository. It reads .git/HEAD directly
// (no git subprocess) and parses the "ref: refs/heads/<name>" form; detached
// HEAD (a raw commit hash) yields "".
func gitBranch(root string) string {
	data, err := os.ReadFile(filepath.Join(root, ".git", "HEAD"))
	if err != nil {
		return ""
	}
	const prefix = "ref: refs/heads/"
	s := strings.TrimSpace(string(data))
	if !strings.HasPrefix(s, prefix) {
		return ""
	}
	return strings.TrimPrefix(s, prefix)
}

// treeNode is one node of the workspace directory tree JSON.
type treeNode struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"` // workspace-relative
	Dir      bool        `json:"dir"`
	Status   string      `json:"status,omitempty"`   // file: git status code (M/A/D/R/U)
	Changes  int         `json:"changes,omitempty"`  // dir: count of changed files below
	Mtime    int64       `json:"mtime,omitempty"`    // file: last modified unix seconds
	Size     int64       `json:"size,omitempty"`     // file: size in bytes
	Children []*treeNode `json:"children,omitempty"`
}

// buildTree walks the workspace recursively (depth-capped), skipping the
// excluded noise directories. Directories sort before files. statusMap maps
// workspace-relative file paths to git status codes (may be nil).
func (s *Server) buildTree(abs, rel string, depth int, statusMap map[string]string) *treeNode {
	n := &treeNode{Name: filepath.Base(abs), Path: rel, Dir: true}
	if depth >= 8 {
		return n
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return n
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return entries[i].Name() < entries[j].Name()
	})
	count := 0
	for _, e := range entries {
		if treeExcludedNames[e.Name()] || count >= 500 {
			continue
		}
		count++
		childRel := e.Name()
		if rel != "" {
			childRel = rel + "/" + e.Name()
		}
		if e.IsDir() {
			child := s.buildTree(filepath.Join(abs, e.Name()), childRel, depth+1, statusMap)
			n.Children = append(n.Children, child)
			n.Changes += child.Changes
		} else {
			child := &treeNode{Name: e.Name(), Path: childRel}
			if info, err := e.Info(); err == nil {
				child.Mtime = info.ModTime().Unix()
				child.Size = info.Size()
			}
			if st, ok := statusMap[childRel]; ok {
				child.Status = st
				n.Changes++
			}
			n.Children = append(n.Children, child)
		}
	}
	return n
}

func (s *Server) handleTree(w http.ResponseWriter, r *http.Request) {
	root := s.buildTree(s.root, "", 0, gitStatusMap(s.root))
	root.Name = filepath.Base(s.root)
	writeJSON(w, http.StatusOK, root)
}

// gitStatusMap runs `git status --porcelain -z` in the workspace root and
// returns a map of workspace-relative file path to a normalized status code
// (M/A/D/R/U; "??" untracked becomes U). It returns an empty map when the
// workspace is not a git repository or git is unavailable, so the tree is
// unaffected. Argument-array exec (no shell) — no injection surface.
func gitStatusMap(root string) map[string]string {
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		return nil
	}
	cmd := exec.Command("git", "status", "--porcelain", "-z")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	m := make(map[string]string)
	for _, rec := range strings.Split(string(out), "\x00") {
		if len(rec) < 4 {
			continue
		}
		x, y := rec[0], rec[1]
		code := ""
		switch {
		case x == 'A' || y == 'A':
			code = "A"
		case y == 'D' || x == 'D':
			code = "D"
		case x == 'R' || y == 'R':
			code = "R"
		case x == '?' && y == '?':
			code = "U"
		case x == 'M' || y == 'M':
			code = "M"
		}
		if code == "" {
			continue
		}
		path := rec[3:]
		if len(path) >= 2 && path[0] == '"' && path[len(path)-1] == '"' {
			path = path[1 : len(path)-1]
		}
		m[path] = code
	}
	return m
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	dir, err := s.resolvePath(r.URL.Query().Get("dir"))
	if err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "target is not a directory"})
		return
	}
	mr, err := r.MultipartReader()
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	var saved []string
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		name := filepath.Base(part.FileName())
		if name == "" || name == "." || name == ".." || name == "/" {
			continue
		}
		dst := filepath.Join(dir, name)
		f, err := os.Create(dst)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		n, copyErr := io.Copy(f, io.LimitReader(part, maxUploadFileSize+1))
		f.Close()
		if copyErr != nil || n > maxUploadFileSize {
			os.Remove(dst)
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "file exceeds 100MB limit"})
			return
		}
		rel, relErr := filepath.Rel(s.root, dst)
		if relErr != nil {
			rel = name
		}
		saved = append(saved, filepath.ToSlash(rel))
	}
	writeJSON(w, http.StatusOK, map[string][]string{"paths": saved})
}

// pathRequest is the JSON body of the open/reveal APIs.
type pathRequest struct {
	Path string `json:"path"`
}

func (s *Server) handleOpen(w http.ResponseWriter, r *http.Request) {
	s.handlePathAction(w, r, openFileFunc, i18n.KeyWebOpenFailed)
}

func (s *Server) handleReveal(w http.ResponseWriter, r *http.Request) {
	s.handlePathAction(w, r, revealFileFunc, i18n.KeyWebRevealFailed)
}

// handlePathAction validates the request path stays inside the workspace,
// then invokes the (injectable) OS launcher. failKey is the i18n key used
// when the launcher itself fails.
func (s *Server) handlePathAction(w http.ResponseWriter, r *http.Request, fn func(string) error, failKey string) {
	var req pathRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	abs, err := s.resolvePath(req.Path)
	if err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	}
	if _, err := os.Stat(abs); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	if err := fn(abs); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": i18n.TF(failKey, err)})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleFile(w http.ResponseWriter, r *http.Request) {
	abs, err := s.resolvePath(r.URL.Query().Get("path"))
	if err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	}
	if info, err := os.Stat(abs); err != nil || info.IsDir() {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not a file"})
		return
	}
	// FEATURE-425: hex=1 serves a byte-range slice as hex rows (for binary
	// files); start/end serve a line-range slice as text; otherwise keep the
	// original whole-file ServeFile behaviour (used by the image previewer).
	if r.URL.Query().Get("hex") == "1" {
		s.serveFileHex(w, r, abs, r.URL.Query().Get("start"), r.URL.Query().Get("end"))
		return
	}
	if startStr := r.URL.Query().Get("start"); startStr != "" {
		s.serveFileLines(w, abs, startStr, r.URL.Query().Get("end"))
		return
	}
	http.ServeFile(w, r, abs)
}

// isRemote reports whether the web UI is served to non-loopback addresses
// (FEATURE-455). A bind address other than 127.0.0.1/localhost (e.g. 0.0.0.0
// or a LAN IP) means remote clients may reach the UI, so the OS-local
// "reveal in folder" action is meaningless and download becomes available.
func (s *Server) isRemote() bool {
	b := strings.ToLower(strings.TrimSpace(s.opts.Bind))
	if b == "" || b == "127.0.0.1" || b == "localhost" || b == "::1" {
		return false
	}
	return true
}

// downloadEnabled reports whether remote file download is both allowed by the
// operator (--download-enabled) and meaningful (served to a remote address).
func (s *Server) downloadEnabled() bool {
	return s.opts.DownloadEnabled && s.isRemote()
}

// handleDownload serves a workspace file to the browser as a download
// (FEATURE-455). It is only available when the operator enabled download and
// the UI is served to a remote address; otherwise it returns 403 so the
// feature is never exposed unintentionally. The path is validated to stay
// inside the workspace (no traversal) and must be a regular file.
func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	if !s.downloadEnabled() {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "download disabled"})
		return
	}
	abs, err := s.resolvePath(r.URL.Query().Get("path"))
	if err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	}
	info, err := os.Stat(abs)
	if err != nil || info.IsDir() {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not a file"})
		return
	}
	// Force the browser to download rather than render inline.
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filepath.Base(abs)+"\"")
	http.ServeFile(w, r, abs)
}

// hexRow is one 16-byte row of a hex dump: the byte offset, the hex bytes and
// the printable ASCII column.
type hexRow struct {
	Offset int    `json:"offset"`
	Hex    string `json:"hex"`
	Ascii  string `json:"ascii"`
}

// serveFileHex reads a byte range [start, end] (byte offsets, width-aligned)
// of a file and returns {total, rows} where each row is a width-byte hex dump
// line. width defaults to 16 and may be 8/16/32/64/128 (FEATURE-425). It reads
// only the requested range via ReadAt so large files are never fully loaded
// into memory (on-demand loading).
func (s *Server) serveFileHex(w http.ResponseWriter, r *http.Request, abs, startStr, endStr string) {
	info, err := os.Stat(abs)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	total := int(info.Size())
	start := 0
	if startStr != "" {
		start, err = strconv.Atoi(startStr)
		if err != nil || start < 0 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid start"})
			return
		}
	}
	end := total
	if endStr != "" {
		end, err = strconv.Atoi(endStr)
		if err != nil || end < start {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid end"})
			return
		}
	}
	width := 16
	if wStr := r.URL.Query().Get("width"); wStr != "" {
		width, err = strconv.Atoi(wStr)
		if err != nil || (width != 8 && width != 16 && width != 32 && width != 64 && width != 128) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid width"})
			return
		}
	}
	if start >= total {
		writeJSON(w, http.StatusOK, map[string]interface{}{"total": total, "rows": []hexRow{}})
		return
	}
	if end > total {
		end = total
	}
	// Align start down to a width boundary so each row is a full width line.
	alignedStart := start - (start % width)
	buf := make([]byte, end-alignedStart)
	f, err := os.Open(abs)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer f.Close()
	if _, err := f.ReadAt(buf, int64(alignedStart)); err != nil && err != io.EOF {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	var rows []hexRow
	for i := 0; i < len(buf); i += width {
		chunk := buf[i:min(i+width, len(buf))]
		var hexParts, asciiParts []string
		for _, b := range chunk {
			hexParts = append(hexParts, fmt.Sprintf("%02x", b))
			if b >= 0x20 && b <= 0x7e {
				asciiParts = append(asciiParts, string(b))
			} else {
				asciiParts = append(asciiParts, ".")
			}
		}
		rows = append(rows, hexRow{
			Offset: alignedStart + i,
			Hex:    strings.Join(hexParts, " "),
			Ascii:  strings.Join(asciiParts, ""),
		})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"total": total, "rows": rows})
}

// serveFileLines reads a 1-based inclusive line range [start, end] of a text
// file and returns {total, lines} as JSON. It streams the file line by line
// (bufio.Scanner) so large files are never fully loaded into memory. When end
// is empty or beyond the file, it reads to EOF. total is the file's total line
// count (computed by a full scan).
func (s *Server) serveFileLines(w http.ResponseWriter, abs, startStr, endStr string) {
	start, err := strconv.Atoi(startStr)
	if err != nil || start < 1 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid start"})
		return
	}
	end := 0
	if endStr != "" {
		end, err = strconv.Atoi(endStr)
		if err != nil || end < start {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid end"})
			return
		}
	}
	f, err := os.Open(abs)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	defer f.Close()

	var lines []string
	total := 0
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		total++
		if total >= start && (end == 0 || total <= end) {
			lines = append(lines, sc.Text())
		}
		if end != 0 && total > end {
			// Keep scanning to count total lines.
		}
	}
	if err := sc.Err(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"total": total, "lines": lines})
}

// gitDiffLine is one line of the git diff for a file, tagged with its status
// relative to the working tree (FEATURE-425). line is the 1-based line number
// in the current working-tree file; status is "add" (new), "del" (deleted) or
// "ctx" (unchanged context).
//
// The diff is computed by running `git diff -- <path>` and parsing the unified
// hunks. For a modified file, added lines map to their working-tree line
// numbers; deleted lines carry the line number they occupied in the working
// tree (the hunk's new-side start offset). Untracked files (no diff) yield an
// empty list.
type gitDiffLine struct {
	Line   int    `json:"line"`
	Status string `json:"status"`
}

// handleGitDiff returns the working-tree diff of one file as a list of
// gitDiffLine (only the changed lines, not context). It returns an empty list
// when the workspace is not a git repo, the file is untracked, or git is
// unavailable.
func (s *Server) handleGitDiff(w http.ResponseWriter, r *http.Request) {
	abs, err := s.resolvePath(r.URL.Query().Get("path"))
	if err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	}
	rel, err := filepath.Rel(s.root, abs)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string][]gitDiffLine{"lines": {}})
		return
	}
	if _, err := os.Stat(filepath.Join(s.root, ".git")); err != nil {
		writeJSON(w, http.StatusOK, map[string][]gitDiffLine{"lines": {}})
		return
	}
	cmd := exec.Command("git", "diff", "--", filepath.ToSlash(rel))
	cmd.Dir = s.root
	out, err := cmd.Output()
	if err != nil {
		writeJSON(w, http.StatusOK, map[string][]gitDiffLine{"lines": {}})
		return
	}
	writeJSON(w, http.StatusOK, map[string][]gitDiffLine{"lines": parseGitDiff(string(out))})
}

// handleTestEndpoint tests whether the given endpoint is reachable by calling
// ListModels (GET /models), reusing the autoCompleteEndpoint fallback logic
// (FEATURE-433). Returns ok + the tested endpoint.
func (s *Server) handleTestEndpoint(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Endpoint string `json:"endpoint"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if req.Endpoint == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "endpoint is required"})
		return
	}
	tested, ok := cmd.TestEndpointConnectivity(req.Endpoint)
	msg := "connected"
	if !ok {
		msg = "cannot connect"
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": ok, "message": msg, "endpoint": tested})
}

// handleTestAPIKey verifies that the given API key is valid against the
// endpoint by calling ListModels (GET /models). Returns ok + a message.
func (s *Server) handleTestAPIKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Endpoint string `json:"endpoint"`
		APIKey   string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if req.Endpoint == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "endpoint is required"})
		return
	}
	ok, msg := cmd.TestAPIKey(req.Endpoint, req.APIKey)
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": ok, "message": msg})
}

// handleGetModelMaxLen fetches the max context length for the given model by
// calling ListModels (GET /models). Returns ok + the max length, or an error
// message explaining why it could not be obtained.
func (s *Server) handleGetModelMaxLen(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Endpoint  string `json:"endpoint"`
		APIKey    string `json:"api_key"`
		ModelName string `json:"model_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if req.Endpoint == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "endpoint is required"})
		return
	}
	if req.ModelName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "model_name is required"})
		return
	}
	maxLen, err := cmd.GetModelMaxLen(req.Endpoint, req.APIKey, req.ModelName)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": false, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "max_model_len": maxLen})
}

// parseGitDiff parses a unified `git diff` output and returns the changed
// lines (add/del) with their working-tree line numbers. Context lines are
// skipped. It tracks the new-side line counter across hunks so added lines get
// correct working-tree numbers.
func parseGitDiff(diff string) []gitDiffLine {
	var out []gitDiffLine
	newLine := 0 // current line number on the new (working-tree) side
	for _, ln := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(ln, "@@"):
			// hunk header: @@ -a,b +c,d @@
			if m := hunkNewRe.FindStringSubmatch(ln); m != nil {
				newLine, _ = strconv.Atoi(m[1])
			}
		case strings.HasPrefix(ln, "+") && !strings.HasPrefix(ln, "+++"):
			out = append(out, gitDiffLine{Line: newLine, Status: "add"})
			newLine++
		case strings.HasPrefix(ln, "-") && !strings.HasPrefix(ln, "---"):
			out = append(out, gitDiffLine{Line: newLine, Status: "del"})
		case strings.HasPrefix(ln, " ") || ln == "":
			newLine++
		}
	}
	return out
}

// hunkNewRe matches the new-side start line in a unified diff hunk header
// `@@ -a,b +c,d @@` and captures c.
var hunkNewRe = regexp.MustCompile(`^@@ -[0-9]+(?:,[0-9]+)? \+([0-9]+)`)

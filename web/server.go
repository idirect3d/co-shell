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
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/idirect3d/co-shell/agent"
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
	Type        string   `json:"type"` // "input" | "answer" | "interrupt"
	Text        string   `json:"text,omitempty"`
	Attachments []string `json:"attachments,omitempty"`
	ID          string   `json:"id,omitempty"`    // answer: the ask id
	Value       string   `json:"value,omitempty"` // answer: the reply
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
	Kind  string          `json:"kind"`            // "event" | "ask" | "state"
	Event *eventJSON      `json:"event,omitempty"` // kind=event
	ID    string          `json:"id,omitempty"`    // kind=ask
	Mode  string          `json:"mode,omitempty"`  // kind=ask: "line" | "key"
	Plan  json.RawMessage `json:"plan"`            // kind=state (null when no plan)
}

// ServerOptions carries the display parameters of a Server.
type ServerOptions struct {
	Lang    string // UI language ("zh"/"en"), handed to the frontend
	Version string
	Build   string
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
	s.mux.HandleFunc("GET /api/file", s.handleFile)
	s.httpSrv = &http.Server{Handler: s.mux}
	return s
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

// Listen binds 127.0.0.1 on the given port (auto-incrementing up to 10
// ports when occupied) and serves HTTP in the background. It returns the
// bound "host:port" address.
func (s *Server) Listen(port int) (string, error) {
	for i := 0; i < 10; i++ {
		p := port + i
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err != nil {
			continue
		}
		s.ln = ln
		go func() { _ = s.httpSrv.Serve(ln) }()
		return fmt.Sprintf("127.0.0.1:%d", p), nil
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
	writeJSON(w, http.StatusOK, map[string]string{
		"lang":      s.opts.Lang,
		"version":   s.opts.Version,
		"build":     s.opts.Build,
		"workspace": s.root,
	})
}

// treeNode is one node of the workspace directory tree JSON.
type treeNode struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"` // workspace-relative
	Dir      bool        `json:"dir"`
	Children []*treeNode `json:"children,omitempty"`
}

// buildTree walks the workspace recursively (depth-capped), skipping the
// excluded noise directories. Directories sort before files.
func (s *Server) buildTree(abs, rel string, depth int) *treeNode {
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
			n.Children = append(n.Children, s.buildTree(filepath.Join(abs, e.Name()), childRel, depth+1))
		} else {
			n.Children = append(n.Children, &treeNode{Name: e.Name(), Path: childRel})
		}
	}
	return n
}

func (s *Server) handleTree(w http.ResponseWriter, r *http.Request) {
	root := s.buildTree(s.root, "", 0)
	root.Name = filepath.Base(s.root)
	writeJSON(w, http.StatusOK, root)
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
	http.ServeFile(w, r, abs)
}

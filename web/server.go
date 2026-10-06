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
	"context"
	"crypto/sha256"
	"embed"
	"encoding/hex"
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
	"time"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/cmd"
	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/store"
)

//go:embed static
var staticFS embed.FS

// staticETag identifies the embedded UI assets. embed.FS entries carry no
// modification time, so http.FileServer emits neither Last-Modified nor ETag;
// browsers then fall back to heuristic caching and keep serving a stale app.js
// across restarts, which silently hides UI changes. Deriving the tag from the
// asset bytes makes it change whenever the UI is rebuilt.
var staticETag = func() string {
	h := sha256.New()
	for _, name := range []string{"static/index.html", "static/app.js", "static/style.css"} {
		if b, err := staticFS.ReadFile(name); err == nil {
			_, _ = h.Write(b)
		}
	}
	return `"` + hex.EncodeToString(h.Sum(nil))[:16] + `"`
}()

// serveStatic serves the embedded UI assets with an explicit validator so the
// browser revalidates on every load. Without it a rebuilt binary keeps showing
// the previous UI until the user performs a hard reload.
func serveStatic() http.Handler {
	files := http.FileServer(http.FS(staticFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("ETag", staticETag)
		if r.Header.Get("If-None-Match") == staticETag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		files.ServeHTTP(w, r)
	})
}

// maxUploadFileSize caps one uploaded file (bytes).
const maxUploadFileSize = 100 << 20 // 100 MiB

// errPathOutside marks any path that would escape the workspace root.
var errPathOutside = errors.New("path escapes workspace")

// clientMessage is a browser-to-server WebSocket message.
type clientMessage struct {
	Type        string          `json:"type"` // "input" | "answer" | "interaction_answer" | "interrupt" | "session_list" | "session_switch" | "session_delete" | "session_new" | "settings_get" | "settings_set" | "identity_get" | "identity_set" | "restart"
	Text        string          `json:"text,omitempty"`
	Attachments []string        `json:"attachments,omitempty"`
	ID          string          `json:"id,omitempty"`          // answer: the ask id; interaction_answer: the interaction id
	Value       string          `json:"value,omitempty"`       // answer: the reply; session_switch/delete: the session id; settings_set: the new value
	Key         string          `json:"key,omitempty"`         // settings_set: the setting key
	Priority    int             `json:"priority,omitempty"`    // model_set_priority: the new priority
	// FEATURE-602: model_auto_select — the new state of the automatic model
	// selection (degrade-on-failure) switch.
	AutoSelect bool `json:"auto_select,omitempty"`
	Result      json.RawMessage `json:"result,omitempty"`      // interaction_answer: the structured result (agent.InteractionResult, FIX-513)
	Step        string          `json:"step,omitempty"`        // model_wizard_next/prev: the current wizard step
	WizardData  json.RawMessage `json:"wizard_data,omitempty"` // model_wizard_next/prev/submit: accumulated wizard data
	YOLO        bool            `json:"yolo,omitempty"`        // yolo_set: the new YOLO mode state
	// MCP server management (FEATURE-464): mcp_add/mcp_update/mcp_remove.
	Name    string   `json:"name,omitempty"`    // mcp_add/update/remove: the server name
	Command string   `json:"command,omitempty"` // mcp_add/update: the server command
	Args    []string `json:"args,omitempty"`    // mcp_add/update: the server args
	Enabled bool     `json:"enabled,omitempty"` // mcp_update: the enabled state
	URL     string   `json:"url,omitempty"`     // mcp_add/update: the SSE URL (FEATURE-498)

	// Kind is the dynamic_event subtype (FEATURE-471): clip_object | upload_file |
	// user_message | open_file. Only used when Type == "dynamic_event".
	Kind string `json:"kind,omitempty"`

	// FEATURE-507: history_get paging cursor. Count is the number of message
	// groups to return (default 20); Before is the sequence cursor to page from.
	Count  int `json:"count,omitempty"`
	Before int `json:"before,omitempty"`
	// FEATURE-542: with After > 0 the client asks for every event newer than that
	// store sequence (incremental catch-up after a reconnect) instead of a tail
	// page measured in message groups.
	After int `json:"after,omitempty"`

	// FEATURE-571: ui_window_state — the frontend reports every window change
	// (close, activate, surface flip) and the full list on connect, so the
	// agent-side registry behind <ui_windows> never disagrees with the screen.
	Windows json.RawMessage `json:"windows,omitempty"` // ui_window_state: [{name,title,surface,active,summary}]
	Closed  []string        `json:"closed,omitempty"`  // ui_window_state: names the user closed

	// FEATURE-530: board frames pushed by the hub (board_registered /
	// board_roster / board_sent / board_message / board_timeout). Board frames
	// are flat JSON — the type sits next to the fields — so each frame maps
	// onto the fields below.
	BoardAgents json.RawMessage `json:"agents,omitempty"`     // board_roster: roster entries
	BoardRev    int             `json:"rev,omitempty"`        // board_roster: roster revision
	RosterRev   int             `json:"roster_rev,omitempty"` // board_registered
	ReqID       string          `json:"req_id,omitempty"`     // board_sent: echoes board_send req_id
	BoardOK     bool            `json:"ok,omitempty"`         // board_sent: accepted by the hub
	MessageID   string          `json:"message_id,omitempty"` // board_sent / board_timeout
	Conv        string          `json:"conversation,omitempty"`
	Deliveries  json.RawMessage `json:"deliveries,omitempty"` // board_sent: per-recipient outcome
	TimeoutSecs int             `json:"timeout_seconds,omitempty"`
	BoardError  string          `json:"error,omitempty"`    // board_sent: rejection reason
	AgentID     string          `json:"agent_id,omitempty"` // board_registered / board_timeout
	Deadline    int64           `json:"deadline,omitempty"` // board_timeout: reply deadline (unix s)
	To          []string        `json:"to,omitempty"`       // board_message
	From        string          `json:"from,omitempty"`     // board_message
	Content     string          `json:"content,omitempty"`  // board_message
	ReplyTo     string          `json:"reply_to,omitempty"` // board_message
	NoReply     bool            `json:"no_reply,omitempty"` // board_message
	BoardState  string          `json:"state,omitempty"`    // board_message: reply state
	BoardTS     int64           `json:"ts,omitempty"`       // board_message: unix seconds
	Seq         int             `json:"seq,omitempty"`      // board_message: hub sequence
	Timeout     int             `json:"timeout,omitempty"`  // board_message: reply window (s)
	// FEATURE-567: re-delivery marker of a board_message (0 = first delivery).
	Redelivery int `json:"redelivery,omitempty"`
	// FEATURE-567: how often the message was re-sent before a board_timeout.
	Redelivered int `json:"redelivered,omitempty"`

	// FEATURE-524: ui_action fields. UIID / UIActionID address the rendered
	// component and the action it declared; Payload carries the structured
	// values the frontend collected (form fields, chart point, …).
	UIID       string          `json:"ui_id,omitempty"`
	UIActionID string          `json:"action_id,omitempty"`
	Payload    json.RawMessage `json:"payload,omitempty"`
	// Blocking mirrors the action's declared blocking flag: true means the
	// action expects a parked render_ui(waiting=true) call to consume it.
	Blocking bool `json:"blocking,omitempty"`

	// FEATURE-524: viewport_w / viewport_h carry the browser window size in CSS
	// pixels (type "viewport"), reported once the WebSocket connects and again
	// on every resize. Strictly an enhancement: older clients never send them.
	ViewportW int `json:"viewport_w,omitempty"`
	ViewportH int `json:"viewport_h,omitempty"`

	// FEATURE-542: filled in by the server from the connection the message
	// arrived on (never sent by the client), so the session can record which端
	// issued the instruction and echo it back in the YOU block.
	Source   string `json:"-"`
	SourceID string `json:"-"`
	// QueueSeq is the FIFO position assigned on enqueue (FEATURE-542, GAP-3).
	// Named QueueSeq because Seq already carries the board_message hub sequence.
	QueueSeq int `json:"-"`
}

// eventJSON is the wire form of an agent.StreamEvent (same field rules as
// the JSON-Lines StreamRenderer: level only when not info).
type eventJSON struct {
	Type  string            `json:"type"`
	Level string            `json:"level,omitempty"`
	Chan  string            `json:"chan,omitempty"`
	Text  string            `json:"text,omitempty"`
	Meta  map[string]string `json:"meta,omitempty"`
	// FIX-549: the moment the event happened (epoch milliseconds). Live frames and
	// persisted events both carry it, so a replayed record can show the real time
	// instead of the moment the browser painted the block (which made every
	// replayed line show the same clock value).
	Ts int64 `json:"ts,omitempty"`
	// FEATURE-542: the store sequence this event was persisted under, stamped on
	// live frames and on history pages. A client keeps the highest value it has
	// seen and asks for everything after it when reconnecting, so a reconnect
	// becomes an incremental catch-up instead of a blind rebuild (R3). Omitted for
	// events that are never persisted (await_input, turn_start, ui_text, ...).
	Seq int `json:"seq,omitempty"`
}

// serverMessage is a server-to-browser WebSocket message.
type serverMessage struct {
	Kind        string          `json:"kind"`                  // "event" | "state" | "sessions" | "settings" | "settings_result" | "identity" | "identity_result"
	Event       *eventJSON      `json:"event,omitempty"`       // kind=event
	Plan        json.RawMessage `json:"plan"`                  // kind=state (null when no plan)
	// FIX-511: kind=state also carries the agent's running state (agent.IsBusy)
	// so a refreshed or reconnected browser can restore its running indicators
	// (logo breathing, red ⏸ button, session-title highlight) instead of
	// defaulting to idle. Pointer so that `false` is still serialised
	// (omitempty alone would drop it).
	Busy     *bool           `json:"busy,omitempty"` // FIX-511: kind=state carries the busy flag too
	// FEATURE-542: kind=state also carries the label of the connection this frame
	// was sent to ("端 N"), so the browser can tag its own YOU blocks with the
	// same source label the event stream records.
	Client   string          `json:"client,omitempty"`
	Sessions []sessionInfo   `json:"sessions,omitempty"` // kind=sessions: the session list
	Settings json.RawMessage `json:"settings,omitempty"` // kind=settings: the grouped setting items
	Identity json.RawMessage `json:"identity,omitempty"` // kind=identity: the identity fields
	OK       bool            `json:"ok,omitempty"`       // kind=settings_result: success flag
	Message  string          `json:"message,omitempty"`  // kind=settings_result: result message
	// FIX-544: kind=identity_result carries the saved field key separately, so
	// the browser can build a "<label> saved" confirmation from the field's
	// localized label instead of showing the raw key.
	Key        string          `json:"key,omitempty"`         // kind=identity_result: the identity field key
	Modes      []modeInfo      `json:"modes,omitempty"`       // kind=mode: the work mode list
	Models     json.RawMessage `json:"models,omitempty"`      // kind=models: the model list JSON
	Templates  json.RawMessage `json:"templates,omitempty"`   // kind=models: the template list JSON
	// FEATURE-602: kind=models also carries the automatic-selection switch so
	// the model manager can render its current state.
	AutoSelect bool `json:"auto_select,omitempty"`
	WizardStep json.RawMessage `json:"wizard_step,omitempty"` // kind=model_wizard: the wizard step form JSON
	WizardData json.RawMessage `json:"wizard_data,omitempty"` // kind=model_wizard: the accumulated wizard data JSON
	// FEATURE-601 follow-up: per-capability detection progress streamed to the
	// browser while the wizard's capabilities step runs a detection.
	Cap   string `json:"cap,omitempty"`   // kind=model_wizard_detect_progress: vision|tool_call|thinking
	Phase string `json:"phase,omitempty"` // kind=model_wizard_detect_progress: "running" | "done"
	YOLO       bool            `json:"yolo,omitempty"`        // kind=yolo: the current YOLO mode state
	MCPServers json.RawMessage `json:"mcp_servers,omitempty"` // kind=mcp: the MCP server list JSON

	// Backfill carries unconsumed user_message texts back to the input box when
	// a task ends (FEATURE-471). kind=dynamic_backfill.
	Backfill []string `json:"backfill,omitempty"`

	// FEATURE-507: kind=history carries a page of persisted UI events so the
	// browser can replay the conversation after a refresh. Events are raw
	// StreamEvent JSON, oldest first; HasMore reports whether older events exist
	// and OldestSeq is the cursor to page further back.
	//
	// FIX-509: HasMore and OldestSeq must NOT be omitempty. Their zero values are
	// meaningful ("no older events" / "no cursor"), and dropping them made the
	// browser read undefined and stop paging after the first page.
	Events    []json.RawMessage `json:"events,omitempty"`
	HasMore   bool              `json:"has_more"`
	OldestSeq int               `json:"oldest_seq"`
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

	// UsageFn returns every persisted per-call token record, used by
	// GET /api/usage to build the history popover (FEATURE-546). Nil means the
	// endpoint answers with zeroed totals.
	UsageFn func() ([]store.TokenUsageEntry, error)

	// TreeExcludes returns the workspace-tree exclusion sets (FEATURE-553): a
	// folder-NAME set matched at any depth and a workspace-relative PATH set
	// matched exactly. It is read on every tree request, so a settings change
	// applies to the next refresh without a restart. Nil disables filtering.
	TreeExcludes func() (names map[string]bool, paths map[string]bool)

	// SkillsFn returns the JSON body of GET /api/skills (FEATURE-583): the
	// discovered skill list, the "default enabled" switch and the loaded total.
	// Nil disables the endpoint.
	SkillsFn func() (string, error)

	// SkillsSetFn applies a POST /api/skills body (FEATURE-583): it validates
	// the requested skill names against the discovered list, updates the config,
	// re-applies the agent skill state and returns the same JSON body as
	// SkillsFn. Nil disables the endpoint.
	SkillsSetFn func(body []byte) (string, error)
}

// clientConn is one connected browser client: its WebSocket plus the identity
// used to tag the messages it sends (FEATURE-542). Several tabs may be
// connected at once, each keeping its own connection, and all of them receive
// the same fan-out — a newly opened tab no longer evicts the previous one.
type clientConn struct {
	id    string // client-supplied id (WS query "client_id"), or a generated fallback
	label string // human-readable source label ("端 1", "端 2", ...)
	c     *wsConn
}

// Server is the embedded web server: HTTP routes + the multi-client WebSocket
// hub. The WebSession registers itself via SetMessageHandler to receive browser
// input; the renderer/WebIO push events through sendJSON.
type Server struct {
	root string
	opts ServerOptions

	mux     *http.ServeMux
	httpSrv *http.Server
	ln      net.Listener

	mu      sync.Mutex
	conns   []clientConn // connected browser clients (FEATURE-542, multi-client)
	connSeq int          // ordinal used to label connections ("端 N")
	handler func(clientMessage)

	// gwConn is the hub's own link (WS query role=hub, FEATURE-530). It lives in
	// a separate slot so the hub's board/proxy link and a browser tab showing
	// this agent's UI never evict each other. FEATURE-542: the browser side is a
	// list and the gateway slot still never touches it.
	gwConn *wsConn

	// onConnect is called when a new client connects. In serve mode the hub
	// dials in, so this fires on every hub (re)connect and lets the board
	// re-announce this agent (FEATURE-530).
	onConnect func()

	// planFn returns the current task plan as a JSON string ("" when none),
	// pushed to the browser on connect as a state message.
	planFn func() string

	// subAgentsFn returns the current session's ID and its sub-agent status
	// list as JSON (FEATURE-564), so a freshly connected client can restore the
	// section below the task plan as well.
	subAgentsFn func() (string, string)

	// modelInfoFn returns the active text/vision model context info for the
	// status bar (FEATURE-378); nil when no provider is registered.
	modelInfoFn func() agent.ModelInfo

	// busyFn reports whether the agent is currently executing a task
	// (FEATURE-499). Exposed via GET /api/status for the hub to poll.
	busyFn func() bool

	// usageFn lists the persisted token usage records for GET /api/usage
	// (FEATURE-546); taken from ServerOptions.UsageFn.
	usageFn func() ([]store.TokenUsageEntry, error)
}

// NewServer creates the server for the given workspace root and registers
// all HTTP routes. The static frontend is embedded.
func NewServer(root string, opts ServerOptions) *Server {
	abs, err := filepath.Abs(root)
	if err != nil {
		abs = root
	}
	s := &Server{root: abs, opts: opts, mux: http.NewServeMux(), usageFn: opts.UsageFn}
	s.mux.HandleFunc("GET /{$}", s.handleIndex)
	s.mux.Handle("GET /static/", serveStatic())
	s.mux.HandleFunc("GET /ws", s.handleWS)
	s.mux.HandleFunc("GET /api/bootstrap", s.handleBootstrap)
	s.mux.HandleFunc("GET /api/tree", s.handleTree)
	s.mux.HandleFunc("POST /api/upload", s.handleUpload)
	s.mux.HandleFunc("POST /api/open", s.handleOpen)
	s.mux.HandleFunc("POST /api/reveal", s.handleReveal)
	s.mux.HandleFunc("GET /api/download", s.handleDownload)
	s.mux.HandleFunc("GET /api/file", s.handleFile)
	s.mux.HandleFunc("GET /api/gitdiff", s.handleGitDiff)
	s.mux.HandleFunc("GET /api/gitfirstchange", s.handleGitFirstChange)
	s.mux.HandleFunc("POST /api/test-endpoint", s.handleTestEndpoint)
	s.mux.HandleFunc("POST /api/test-api-key", s.handleTestAPIKey)
	s.mux.HandleFunc("POST /api/get-model-max-len", s.handleGetModelMaxLen)
	// FEATURE-477: system logo upload / remove / read (per theme dark|light).
	s.mux.HandleFunc("POST /api/logo", s.handleLogoUpload)
	s.mux.HandleFunc("DELETE /api/logo", s.handleLogoRemove)
	s.mux.HandleFunc("GET /logos/{theme}", s.handleLogoRead)
	// FEATURE-499: lightweight status endpoint the hub polls for busy state.
	s.mux.HandleFunc("GET /api/status", s.handleStatus)
	// FEATURE-546: aggregated token history for the status-bar usage popover.
	s.mux.HandleFunc("GET /api/usage", s.handleUsage)
	// FEATURE-524: the escape hatch for LLM-authored HTML runs in a sandboxed
	// iframe whose shell is served from its own endpoint with its own CSP.
	s.mux.HandleFunc("GET /api/ui-sandbox", s.handleUISandbox)
	// FEATURE-583: the Skills settings page reads the skill list and applies
	// per-skill / default enable switches through these two endpoints.
	s.mux.HandleFunc("GET /api/skills", s.handleSkillsGet)
	s.mux.HandleFunc("POST /api/skills", s.handleSkillsSet)
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

// SetConnectHook installs the client-connect callback (FEATURE-530: the board
// re-sends its registration whenever the hub link is (re)established).
func (s *Server) SetConnectHook(fn func()) {
	s.mu.Lock()
	s.onConnect = fn
	s.mu.Unlock()
}

// SetPlanProvider installs the task-plan snapshot provider.
func (s *Server) SetPlanProvider(fn func() string) {
	s.mu.Lock()
	s.planFn = fn
	s.mu.Unlock()
}

// SetSubAgentsProvider installs the sub-agent snapshot provider (FEATURE-564).
// It returns the current session's ID and its sub-agent list as JSON, so a
// connecting client can paint the list below the task plan right away.
func (s *Server) SetSubAgentsProvider(fn func() (string, string)) {
	s.mu.Lock()
	s.subAgentsFn = fn
	s.mu.Unlock()
}

// SetModelInfoProvider installs the active model context-info provider
// (FEATURE-378), used by the status bar to show context usage.
func (s *Server) SetModelInfoProvider(fn func() agent.ModelInfo) {
	s.mu.Lock()
	s.modelInfoFn = fn
	s.mu.Unlock()
}

// SetBusyProvider installs the busy-state provider (FEATURE-499). It reports
// whether the agent is currently executing a task, exposed via GET /api/status
// for the hub to poll.
func (s *Server) SetBusyProvider(fn func() bool) {
	s.mu.Lock()
	s.busyFn = fn
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

// Close shuts the server down and drops the current clients.
func (s *Server) Close() error {
	s.mu.Lock()
	conns := s.conns
	s.conns = nil
	gw := s.gwConn
	s.gwConn = nil
	s.mu.Unlock()
	for _, cc := range conns {
		cc.c.Close()
	}
	if gw != nil {
		gw.Close()
	}
	if s.ln != nil {
		return s.ln.Close()
	}
	return nil
}

// ---------------------------------------------------------------------------
// Single-client WebSocket hub
// ---------------------------------------------------------------------------

// addConn registers a freshly opened browser connection and returns its entry.
// FEATURE-542: every tab keeps its own connection — unlike the old single-client
// slot a new connection no longer closes (evicts) the previous one.
func (s *Server) addConn(c *wsConn, clientID string) clientConn {
	s.mu.Lock()
	s.connSeq++
	cc := clientConn{id: clientID, label: fmt.Sprintf("端 %d", s.connSeq), c: c}
	if cc.id == "" {
		cc.id = fmt.Sprintf("client-%d", s.connSeq)
	}
	s.conns = append(s.conns, cc)
	onConn := s.onConnect
	s.mu.Unlock()
	if onConn != nil {
		onConn()
	}
	return cc
}

// removeConn drops one closed browser connection from the list. FEATURE-542: it
// deliberately does NOT fail that client's pending interactions — a browser
// that went away is only "temporarily unreachable", so the agent keeps waiting
// for an answer instead of treating the user as gone forever (R7).
func (s *Server) removeConn(c *wsConn) {
	s.mu.Lock()
	for i, cc := range s.conns {
		if cc.c == c {
			s.conns = append(s.conns[:i], s.conns[i+1:]...)
			break
		}
	}
	s.mu.Unlock()
}

// HasClient reports whether at least one browser client is connected
// (FEATURE-542). The hub link (gwConn) deliberately does not count: it is only
// a relay and may have no user behind it (design §9.2).
func (s *Server) HasClient() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.conns) > 0
}

// ClientCount returns the number of connected browser clients (FEATURE-542).
func (s *Server) ClientCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.conns)
}

// setGatewayConn replaces the hub gateway connection (role=hub). Unlike
// setConn it never touches the browser slot, so the hub link and a browser
// session coexist (FEATURE-530). A new gateway connection re-announces the
// board registration through onConnect.
func (s *Server) setGatewayConn(c *wsConn) {
	s.mu.Lock()
	old := s.gwConn
	s.gwConn = c
	onConn := s.onConnect
	s.mu.Unlock()
	if old != nil {
		old.Close()
	}
	if c != nil && onConn != nil {
		onConn()
	}
}

// clearGatewayConn drops the gateway connection if it is still the current one.
func (s *Server) clearGatewayConn(c *wsConn) {
	s.mu.Lock()
	if s.gwConn == c {
		s.gwConn = nil
	}
	s.mu.Unlock()
}

// sendJSON marshals v and broadcasts it to every connected browser client
// (FEATURE-542 fan-out). It returns true when at least one write was queued;
// false means nobody was listening.
func (s *Server) sendJSON(v serverMessage) bool {
	data, err := json.Marshal(v)
	if err != nil {
		return false
	}
	s.mu.Lock()
	conns := make([]*wsConn, 0, len(s.conns))
	for _, cc := range s.conns {
		conns = append(conns, cc.c)
	}
	s.mu.Unlock()
	ok := false
	for _, c := range conns {
		if c.WriteMessage(data) == nil {
			ok = true
		}
	}
	return ok
}

// sendEvent pushes one agent stream event to the browser.
// sendRaw writes a raw JSON message to the hub link (role=hub gateway slot)
// and to every browser client. Used by the board sender to push board_* messages
// back to the hub (FEATURE-490); browser clients (agent UI) keep receiving them
// too, exactly as before the gateway slot existed.
func (s *Server) sendRaw(data []byte) error {
	s.mu.Lock()
	gw := s.gwConn
	conns := make([]*wsConn, 0, len(s.conns))
	for _, cc := range s.conns {
		conns = append(conns, cc.c)
	}
	s.mu.Unlock()
	if gw == nil && len(conns) == 0 {
		return errors.New("no web client connected")
	}
	var firstErr error
	if gw != nil {
		if err := gw.WriteMessage(data); err != nil {
			firstErr = err
		}
	}
	for _, c := range conns {
		if err := c.WriteMessage(data); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (s *Server) sendEvent(ev agent.StreamEvent) bool {
	return s.sendEventSeq(ev, 0)
}

// sendEventSeq pushes one event stamped with the store sequence it was persisted
// under (FEATURE-542; 0 = not persisted, so it carries no cursor). A reconnecting
// client sends back the highest seq it saw and receives exactly what it missed.
func (s *Server) sendEventSeq(ev agent.StreamEvent, seq int) bool {
	// FIX-549: stamp the frame with the moment it was emitted; newEventJSON falls
	// back to now when the caller has no better time to offer.
	ej := newEventJSON(ev, 0)
	if seq > 0 {
		ej.Seq = seq
	}
	return s.sendJSON(serverMessage{Kind: "event", Event: ej})
}

// NOTE (FEATURE-542): the former sendAsk/sendInteraction direct frames were
// removed. Interactive requests now travel as the persisted interaction_request
// event (GAP-1), so every client gets them through the same fan-out path as any
// other event and a reconnecting browser can replay them.

// sendState pushes the current task plan and running state snapshot to one
// freshly connected client. It runs on every connection, so a page refresh and
// a manual reconnect both resynchronise the browser with the backend.
func (s *Server) sendState(c *wsConn, label string) {
	s.mu.Lock()
	fn := s.planFn
	busyFn := s.busyFn
	saFn := s.subAgentsFn
	s.mu.Unlock()
	plan := json.RawMessage("null")
	if fn != nil {
		if p := fn(); p != "" {
			plan = json.RawMessage(p)
		}
	}
	// FIX-511: report whether a task run is in flight. The frontend derives its
	// running indicators from live turn_start/await_input events only, and those
	// are not persisted, so without this the UI stayed idle after a refresh
	// while the agent was still working.
	busy := false
	if busyFn != nil {
		busy = busyFn()
	}
	data, err := json.Marshal(serverMessage{Kind: "state", Plan: plan, Busy: &busy, Client: label})
	if err != nil {
		return
	}
	_ = c.WriteMessage(data)
	// FEATURE-564: restore the sub-agent list as well. It is pushed on agent
	// events only, so without this snapshot a page refresh (or a reconnect)
	// left the section below the task plan empty until the next sub-agent
	// activity.
	if saFn != nil {
		sid, rows := saFn()
		ev := newEventJSON(agent.SubAgentsEvent(sid, rows), 0)
		if payload, err := json.Marshal(serverMessage{Kind: "event", Event: ev}); err == nil {
			_ = c.WriteMessage(payload)
		}
	}
}

// dispatch routes one browser message to the registered handler. src is the
// connection the message arrived on (zero value for the hub link); FEATURE-542
// tags the message with its source so the event stream records which端 issued
// the instruction.
func (s *Server) dispatch(raw []byte, src clientConn) {
	var msg clientMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return
	}
	if src.c != nil {
		msg.Source = src.label
		msg.SourceID = src.id
	}
	s.mu.Lock()
	h := s.handler
	s.mu.Unlock()
	if h != nil {
		h(msg)
	}
}

// gatewayRole is the WS query value marking the hub's own link (FEATURE-530).
const gatewayRole = "hub"

// handleWS upgrades the connection, registers it in the browser or gateway
// slot, pushes the initial state to a browser, then reads messages until
// disconnect.
func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	c, err := upgrade(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	gateway := r.URL.Query().Get("role") == gatewayRole
	var src clientConn
	if gateway {
		s.setGatewayConn(c)
		// FIX-475: close the connection when the read loop ends so the async
		// writer goroutine stops (otherwise it would leak on client disconnect).
		defer c.Close()
		defer s.clearGatewayConn(c)
	} else {
		// FEATURE-542: several browsers may be connected at once; each gets its
		// own entry and they all receive the same fan-out. The source label is
		// attached to every message this connection sends.
		src = s.addConn(c, r.URL.Query().Get("client_id"))
		s.sendState(c, src.label)
		defer c.Close()
		defer s.removeConn(c)
	}
	for {
		msg, err := c.ReadMessage()
		if err != nil {
			return
		}
		s.dispatch(msg, src)
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

// indexCSP is the policy for the main application document: own scripts and
// sockets only, no plugins. 'unsafe-inline' covers style attributes alone
// (index.html uses one) — scripts stay restricted to 'self'.
//
// frame-ancestors is 'self', not 'none': the co-shell-hub embeds each agent's
// Web UI in a same-origin iframe (it reverse-proxies the agent under
// /agent/{id}/, see hub/gateway/proxyhttp.go), so 'none' blanked the hub stage.
// 'self' keeps every cross-site framing blocked.
const indexCSP = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data: blob: https:; font-src 'self' data:; connect-src 'self' ws: wss:; " +
	"frame-src 'self'; object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'self'"

// uiSandboxCSP isolates the html escape hatch: no network at all (default-src
// 'none', connect-src 'none'), so even a hostile tree cannot exfiltrate data
// or pull remote code. The shell's own inline driver is the only script allowed.
const uiSandboxCSP = "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; " +
	"img-src data:; font-src data:; connect-src 'none'; media-src 'none'; object-src 'none'; " +
	"base-uri 'none'; form-action 'none'; frame-ancestors 'self'"

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	data, err := staticFS.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "index not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", indexCSP)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	_, _ = w.Write(data)
}

// handleUISandbox serves the fixed shell the `html` component embeds. It never
// carries LLM output: the parent posts the markup into the frame after load.
func (s *Server) handleUISandbox(w http.ResponseWriter, r *http.Request) {
	data, err := staticFS.ReadFile("static/ui-sandbox.html")
	if err != nil {
		http.Error(w, "sandbox shell not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", uiSandboxCSP)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	_, _ = w.Write(data)
}

func (s *Server) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	payload := map[string]interface{}{
		"lang":      s.opts.Lang,
		"version":   s.opts.Version,
		"build":     s.opts.Build,
		"workspace": s.root,
		// FIX-542: the agent name shown in the centre of the titlebar. Resolved
		// from IDENTITY.md on every request (ResolveAgentName trims the value and
		// falls back to "co-shell"), so an edit made elsewhere shows up on reload.
		"agent_name":      agent.ResolveAgentName(s.root),
		"branch":          gitBranch(s.root),
		"remote":          s.isRemoteRequest(r),
		"downloadEnabled": s.downloadEnabled(r),
	}
	// FEATURE-378: expose the active text/vision model context info for the
	// status bar's context-usage display.
	s.mu.Lock()
	fn := s.modelInfoFn
	s.mu.Unlock()
	if fn != nil {
		info := fn()
		payload["textModel"] = info.TextModelName
		payload["textModelID"] = info.TextModelID
		payload["textMaxLen"] = info.TextMaxLen
		payload["visionModel"] = info.VisionModelName
		payload["visionModelID"] = info.VisionModelID
		payload["visionMaxLen"] = info.VisionMaxLen
		// Current mode's bound model IDs (empty = no binding, uses global
		// default). Lets the web UI highlight the "默认" option (FEATURE-422).
		payload["modeTextModelID"] = info.ModeTextModelID
		payload["modeVisionModelID"] = info.ModeVisionModelID
	}
	writeJSON(w, http.StatusOK, payload)
}

// handleStatus reports whether the agent is currently executing a task
// (FEATURE-499). The hub polls this lightweight endpoint to drive the red
// breathing status light on each agent in its list.
func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	fn := s.busyFn
	s.mu.Unlock()
	busy := false
	if fn != nil {
		busy = fn()
	}
	writeJSON(w, http.StatusOK, map[string]bool{"busy": busy})
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
	Status   string      `json:"status,omitempty"`  // file: git status code (M/A/D/R/U)
	Changes  int         `json:"changes,omitempty"` // dir: count of changed files below
	Mtime    int64       `json:"mtime,omitempty"`   // file: last modified unix seconds
	Size     int64       `json:"size,omitempty"`    // file: size in bytes
	Children []*treeNode `json:"children,omitempty"`
}

// treeWalk carries the per-request state of a workspace tree build: the git
// status map plus the exclusion sets of FEATURE-553 — folder NAMES matched at
// any depth and workspace-relative PATHS matched exactly.
type treeWalk struct {
	statusMap map[string]string
	names     map[string]bool
	paths     map[string]bool
	// dirChanges caches, per immediate parent directory, the number of changed
	// files it contains (FIX-589); a directory's recursive change count is then
	// derived by prefix lookup instead of walking the filesystem again.
	dirChanges map[string]int
}

// excluded reports whether a directory entry is hidden from the tree. Only
// folders are matched, so a file sharing a listed name stays visible.
func (tw *treeWalk) excluded(name, rel string) bool {
	return tw.names[name] || tw.paths[rel]
}

// underExcludedDir reports whether any ancestor directory of the
// workspace-relative path is hidden by the exclusion list.
func (tw *treeWalk) underExcludedDir(rel string) bool {
	parts := strings.Split(rel, "/")
	acc := ""
	for i := 0; i+1 < len(parts); i++ {
		if acc == "" {
			acc = parts[i]
		} else {
			acc = acc + "/" + parts[i]
		}
		if tw.excluded(parts[i], acc) {
			return true
		}
	}
	return false
}

// parentDir returns the workspace-relative parent directory of a
// slash-separated path ("" for a top-level entry).
func parentDir(rel string) string {
	if i := strings.LastIndex(rel, "/"); i >= 0 {
		return rel[:i]
	}
	return ""
}

// changesIndex maps each immediate parent directory to the number of changed
// files it directly contains (FIX-589). Deleted files are skipped (they are no
// longer part of the listing) and so are files below an excluded directory, so
// the recursive counts keep the pre-FIX-589 semantics while costing no extra
// filesystem walk.
func (tw *treeWalk) changesIndex(root string) map[string]int {
	if tw.dirChanges != nil {
		return tw.dirChanges
	}
	m := make(map[string]int)
	for f := range tw.statusMap {
		if tw.underExcludedDir(f) {
			continue
		}
		info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(f)))
		if err != nil || info.IsDir() {
			continue
		}
		m[parentDir(f)]++
	}
	tw.dirChanges = m
	return m
}

// countChangesBelow sums the changed-file counts of dir and every directory
// below it ("" means the workspace root).
func countChangesBelow(idx map[string]int, dir string) int {
	total := 0
	for k, n := range idx {
		if dir == "" || k == dir || strings.HasPrefix(k, dir+"/") {
			total += n
		}
	}
	return total
}

// pathExcluded reports whether the directory itself or any of its ancestors is
// hidden by the exclusion list: the level API must not leak a folder the tree
// intentionally hides (FEATURE-553/FIX-589).
func (tw *treeWalk) pathExcluded(rel string) bool {
	acc := ""
	for _, seg := range strings.Split(rel, "/") {
		if seg == "" {
			continue
		}
		if acc == "" {
			acc = seg
		} else {
			acc = acc + "/" + seg
		}
		if tw.excluded(seg, acc) {
			return true
		}
	}
	return false
}

// buildTree walks the workspace recursively (depth-capped), skipping the
// folders excluded by the workspace settings. Directories sort before files.
func (s *Server) buildTree(abs, rel string, depth int, tw *treeWalk) *treeNode {
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
		if count >= 500 {
			continue
		}
		childRel := e.Name()
		if rel != "" {
			childRel = rel + "/" + e.Name()
		}
		if e.IsDir() && tw.excluded(e.Name(), childRel) {
			continue
		}
		count++
		if e.IsDir() {
			child := s.buildTree(filepath.Join(abs, e.Name()), childRel, depth+1, tw)
			n.Children = append(n.Children, child)
			n.Changes += child.Changes
		} else {
			child := &treeNode{Name: e.Name(), Path: childRel}
			if info, err := e.Info(); err == nil {
				child.Mtime = info.ModTime().Unix()
				child.Size = info.Size()
			}
			if st, ok := tw.statusMap[childRel]; ok {
				child.Status = st
				n.Changes++
			}
			n.Children = append(n.Children, child)
		}
	}
	return n
}

// buildTreeLevel returns a directory node carrying only its immediate children
// (FIX-589). Directory children keep their recursive changed-file count (the
// git badge in the sidebar) but no children array, so the client requests
// deeper levels on demand and a refresh transfers one level instead of the
// whole workspace.
func (s *Server) buildTreeLevel(abs, rel string, tw *treeWalk) *treeNode {
	idx := tw.changesIndex(s.root)
	n := &treeNode{Name: filepath.Base(abs), Path: rel, Dir: true, Changes: countChangesBelow(idx, rel)}
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
		if count >= 500 {
			continue
		}
		childRel := e.Name()
		if rel != "" {
			childRel = rel + "/" + e.Name()
		}
		if e.IsDir() && tw.excluded(e.Name(), childRel) {
			continue
		}
		count++
		if e.IsDir() {
			n.Children = append(n.Children, &treeNode{
				Name:    e.Name(),
				Path:    childRel,
				Dir:     true,
				Changes: countChangesBelow(idx, childRel),
			})
			continue
		}
		child := &treeNode{Name: e.Name(), Path: childRel}
		if info, err := e.Info(); err == nil {
			child.Mtime = info.ModTime().Unix()
			child.Size = info.Size()
		}
		if st, ok := tw.statusMap[childRel]; ok {
			child.Status = st
		}
		n.Children = append(n.Children, child)
	}
	return n
}

// resolveTreePath maps a workspace-relative directory path from the query
// string to an absolute path inside the workspace root. Traversal (".."),
// absolute paths, symlink escapes and anything outside the root are rejected
// (FIX-589, path-traversal protection).
func (s *Server) resolveTreePath(rel string) (string, bool) {
	rootClean := filepath.Clean(s.root)
	rootReal := rootClean
	if r, err := filepath.EvalSymlinks(rootClean); err == nil {
		rootReal = r
	}
	if rel == "" {
		return rootClean, true
	}
	if filepath.IsAbs(rel) {
		return "", false
	}
	for _, seg := range strings.Split(rel, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return "", false
		}
	}
	abs := filepath.Join(rootClean, filepath.FromSlash(rel))
	if abs != rootClean && !strings.HasPrefix(abs, rootClean+string(os.PathSeparator)) {
		return "", false
	}
	// A symlink must not lead outside the workspace; the resolved root is the
	// comparison base as well (on macOS t.TempDir() resolves through
	// /private, so comparing against the unresolved root would reject it).
	if real, err := filepath.EvalSymlinks(abs); err == nil {
		if real != rootReal && !strings.HasPrefix(real, rootReal+string(os.PathSeparator)) {
			return "", false
		}
	}
	return abs, true
}

// handleTree serves the workspace tree. FIX-589: by default it returns the
// requested directory (the workspace root when path is empty) with its
// immediate children only, so a refresh transfers one level instead of the
// whole workspace; ?depth=full keeps the recursive listing for callers that
// need the whole tree at once.
func (s *Server) handleTree(w http.ResponseWriter, r *http.Request) {
	// FEATURE-553: the exclusion list comes from the workspace settings and is
	// re-read per request, so a change in the settings panel shows up on the
	// next tree refresh without a restart.
	tw := &treeWalk{statusMap: gitStatusMap(s.root)}
	if s.opts.TreeExcludes != nil {
		tw.names, tw.paths = s.opts.TreeExcludes()
	}
	rel := strings.Trim(strings.TrimSpace(r.URL.Query().Get("path")), "/")
	abs, ok := s.resolveTreePath(rel)
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid path"})
		return
	}
	if rel != "" && tw.pathExcluded(rel) {
		// A folder the tree hides must not be reachable through the level API
		// either, otherwise ?path= would bypass the exclusion list.
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "directory not found"})
		return
	}
	if rel != "" {
		info, err := os.Stat(abs)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "directory not found"})
			return
		}
		if !info.IsDir() {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "not a directory"})
			return
		}
	}
	var root *treeNode
	if r.URL.Query().Get("depth") == "full" {
		root = s.buildTree(abs, rel, 0, tw)
	} else {
		root = s.buildTreeLevel(abs, rel, tw)
	}
	root.Name = filepath.Base(abs)
	writeJSON(w, http.StatusOK, root)
}

// gitStatusMap runs `git status --porcelain -z` in the workspace root and
// returns a map of workspace-relative file path to a normalized status code
// (M/A/D/R/U; "??" untracked becomes U). It returns an empty map when the
// workspace is not a git repository or git is unavailable, so the tree is
// unaffected. The git command is bounded by a 3s timeout so a hung git
// process (e.g. a repo whose status blocks) never stalls the file-tree API.
// Argument-array exec (no shell) — no injection surface.
func gitStatusMap(root string) map[string]string {
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain", "-z")
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
	// FEATURE-469: auto-create the target directory (e.g. the message
	// attachment upload dir "input") when it does not exist yet.
	if info, statErr := os.Stat(dir); statErr != nil || !info.IsDir() {
		if os.IsNotExist(statErr) {
			if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": mkErr.Error()})
				return
			}
		} else {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "target is not a directory"})
			return
		}
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

// logoThemeFromParam validates a theme query/path value
// (dark|dark-muted|light|light-tp|paper) and returns it, or "" when invalid
// (FEATURE-477/522).
func logoThemeFromParam(v string) string {
	if v == "dark" || v == "dark-muted" || v == "light" || v == "light-tp" || v == "paper" {
		return v
	}
	return ""
}

// logoPath returns the absolute path of the stored logo for a theme. The theme
// must already be validated (dark|light|light-tp|paper). Logos live in the
// workspace logos/ directory (FEATURE-477).
func (s *Server) logoPath(theme string) string {
	return filepath.Join(s.root, "logos", "logo-"+theme+".png")
}

// handleLogoUpload stores the uploaded logo image for a theme (FEATURE-477).
// The request is multipart/form-data with a "theme" field
// (dark|light|light-tp|paper) and a "file" part carrying the image bytes. The
// image is saved as <workspace>/logos/logo-<theme>.png, replacing any previous
// logo for that theme.
func (s *Server) handleLogoUpload(w http.ResponseWriter, r *http.Request) {
	theme := logoThemeFromParam(r.FormValue("theme"))
	if theme == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid theme (dark|dark-muted|light|light-tp|paper)"})
		return
	}
	if err := r.ParseMultipartForm(maxUploadFileSize); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing file part"})
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxUploadFileSize+1))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if len(data) > maxUploadFileSize {
		writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "file exceeds 100MB limit"})
		return
	}
	if len(data) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "empty file"})
		return
	}
	if err := os.MkdirAll(filepath.Join(s.root, "logos"), 0o755); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if err := os.WriteFile(s.logoPath(theme), data, 0o644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleLogoRemove deletes the stored logo for a theme (FEATURE-477). A missing
// logo is not an error (idempotent).
func (s *Server) handleLogoRemove(w http.ResponseWriter, r *http.Request) {
	theme := logoThemeFromParam(r.URL.Query().Get("theme"))
	if theme == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid theme (dark|dark-muted|light|light-tp|paper)"})
		return
	}
	_ = os.Remove(s.logoPath(theme))
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// handleLogoRead serves the stored logo image for a theme (FEATURE-477). It
// returns 404 when no logo is configured for that theme so the frontend can
// fall back to the default ▸ co-shell text.
func (s *Server) handleLogoRead(w http.ResponseWriter, r *http.Request) {
	theme := logoThemeFromParam(r.PathValue("theme"))
	if theme == "" {
		http.NotFound(w, r)
		return
	}
	abs := s.logoPath(theme)
	data, err := os.ReadFile(abs)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	// no-store so an overwrite/removal is always reflected on the next fetch
	// (the frontend also appends a cache-busting query param) (FEATURE-477).
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(data)
}

// pathRequest is the JSON body of the open/reveal APIs.
type pathRequest struct {
	Path string `json:"path"`
}

func (s *Server) handleOpen(w http.ResponseWriter, r *http.Request) {
	// FEATURE-455: opening a file with the server's OS default app is
	// meaningless (and a security concern) when the UI is served remotely.
	if s.isRemoteRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "open disabled on remote access"})
		return
	}
	s.handlePathAction(w, r, openFileFunc, i18n.KeyWebOpenFailed)
}

func (s *Server) handleReveal(w http.ResponseWriter, r *http.Request) {
	// FEATURE-455: revealing a file in the server's OS file manager is
	// meaningless (and a security concern) when the UI is served remotely.
	if s.isRemoteRequest(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "reveal disabled on remote access"})
		return
	}
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

// isRemoteClient reports whether the request carries an X-Client-IP header
// (FEATURE-607) whose address is not loopback. The hub reverse proxy injects
// the real client address there, so co-shell can tell a remote client apart
// from a browser running on the same machine: both reach a loopback-bound
// server from 127.0.0.1 when the hub proxies the request.
func (s *Server) isRemoteClient(r *http.Request) bool {
	raw := strings.TrimSpace(r.Header.Get("X-Client-IP"))
	if raw == "" {
		return false
	}
	host := raw
	if h, _, err := net.SplitHostPort(raw); err == nil {
		host = h
	}
	ip := net.ParseIP(strings.Trim(strings.TrimSpace(host), "[]"))
	if ip == nil {
		return false
	}
	return !ip.IsLoopback()
}

// isRemoteRequest reports whether the current request comes from a remote
// client: the server itself is bound to a non-loopback address (direct remote
// access), or the hub proxy forwarded a non-loopback client address
// (FEATURE-607). It drives the OS-local action guards and file download.
func (s *Server) isRemoteRequest(r *http.Request) bool {
	return s.isRemote() || s.isRemoteClient(r)
}

// downloadEnabled reports whether file download is allowed for this request:
// the operator enabled it (--download-enabled) and the request comes from a
// remote client (see isRemoteRequest).
func (s *Server) downloadEnabled(r *http.Request) bool {
	return s.opts.DownloadEnabled && s.isRemoteRequest(r)
}

// handleDownload serves a workspace file to the browser as a download
// (FEATURE-455/FEATURE-607). It is only available when the operator enabled
// download and the request comes from a remote client (non-loopback bind, or a
// non-loopback X-Client-IP injected by the hub proxy); otherwise it returns
// 403 so the feature is never exposed unintentionally. The path is validated
// to stay inside the workspace (no traversal) and must be a regular file.
func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	if !s.downloadEnabled(r) {
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

// handleGitFirstChange returns the first changed line of the file's most
// recent commit (FEATURE-494). It runs `git log -1 --format=%H -- <path>` to
// find the last commit C that touched the file, then `git diff C^ C -- <path>`
// and returns the new-side start line of the first hunk (the first changed
// line in the current file). Returns line=0 when the workspace is not a git
// repo, the file is untracked, C has no parent (root commit), or git is
// unavailable.
func (s *Server) handleGitFirstChange(w http.ResponseWriter, r *http.Request) {
	abs, err := s.resolvePath(r.URL.Query().Get("path"))
	if err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": err.Error()})
		return
	}
	rel, err := filepath.Rel(s.root, abs)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]int{"line": 0})
		return
	}
	if _, err := os.Stat(filepath.Join(s.root, ".git")); err != nil {
		writeJSON(w, http.StatusOK, map[string]int{"line": 0})
		return
	}
	// Find the most recent commit that touched this file.
	logCmd := exec.Command("git", "log", "-1", "--format=%H", "--", filepath.ToSlash(rel))
	logCmd.Dir = s.root
	commit, err := logCmd.Output()
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]int{"line": 0})
		return
	}
	c := strings.TrimSpace(string(commit))
	if c == "" {
		writeJSON(w, http.StatusOK, map[string]int{"line": 0})
		return
	}
	// Diff that commit against its parent; a root commit has no parent, so
	// fall back to diffing against the empty tree (the whole file is new).
	parent := c + "^"
	rpCmd := exec.Command("git", "rev-parse", "--verify", parent)
	rpCmd.Dir = s.root
	if _, err := rpCmd.Output(); err != nil {
		parent = emptyTreeHash
	}
	diffCmd := exec.Command("git", "diff", parent, c, "--", filepath.ToSlash(rel))
	diffCmd.Dir = s.root
	out, err := diffCmd.Output()
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]int{"line": 0})
		return
	}
	lines := parseGitDiff(string(out))
	line := 0
	if len(lines) > 0 {
		line = lines[0].Line
	}
	writeJSON(w, http.StatusOK, map[string]int{"line": line})
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

// emptyTreeHash is the well-known SHA-1 of git's empty tree object, used as
// the diff base for a root commit (which has no parent) so the whole file
// shows as new (FEATURE-494).
const emptyTreeHash = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

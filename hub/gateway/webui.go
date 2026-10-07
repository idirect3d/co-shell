package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// WebUIConfig configures the hub Web UI HTTP server.
type WebUIConfig struct {
	// ListenAddr is the HTTP listen address, e.g. "127.0.0.1:12802".
	ListenAddr string `json:"listen_addr"`
	// Whitelist restricts browser access to the given IPs/CIDR networks.
	// Empty means loopback only (default localhost access).
	Whitelist []string `json:"whitelist,omitempty"`
	// Lang is the Web UI language ("zh"/"en", FEATURE-559). Empty means
	// auto-detect: --lang > LANG / LC_ALL > zh. Unsupported values fall back
	// to the same chain.
	Lang string `json:"lang,omitempty"`
}

// WebUI serves the hub's Web UI: an HTTP server that serves the embedded
// iframe-shell frontend and reverse-proxies each agent's co-shell Web UI under
// /agent/{id}/... (FEATURE-484). It also exposes agent-management HTTP
// endpoints backed by the Manager.
type WebUI struct {
	cfg          WebUIConfig
	proxy        *Proxy
	manager      *Manager
	reverseProxy *httpReverseProxy
	settings     *Settings
	ctx          context.Context
	version      string
	build        string
	lang         Lang

	// board is the agent message board (FEATURE-530). It is optional: when
	// nil the board endpoints simply return empty results.
	board *Board

	httpSrv *http.Server
	ln      net.Listener
}

// NewWebUI creates a WebUI server bound to the given proxy and agent manager.
// version/build identify this hub build (shown in the logo badge). settings
// carries the remote-access config (TLS/whitelist/access key) and is updated
// through the settings panel at runtime.
func NewWebUI(cfg WebUIConfig, proxy *Proxy, manager *Manager, settings *Settings, version, build string) *WebUI {
	ctx := context.Background()
	w := &WebUI{
		cfg:          cfg,
		proxy:        proxy,
		manager:      manager,
		reverseProxy: newHTTPReverseProxy(manager),
		settings:     settings,
		ctx:          ctx,
		version:      version,
		build:        build,
		lang:         DetectLang(cfg.Lang),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", w.handleIndex)
	// Hub favicon (FEATURE-515): SVG for modern browsers, PNG for older ones;
	// /favicon.ico also serves the PNG so bookmark/shortcut fetches keep working.
	mux.HandleFunc("GET /favicon.svg", handleFavicon(faviconSVG, "image/svg+xml"))
	mux.HandleFunc("GET /favicon.png", handleFavicon(faviconPNG, "image/png"))
	mux.HandleFunc("GET /favicon.ico", handleFavicon(faviconPNG, "image/png"))
	// Reverse-proxy each agent's co-shell Web UI under /agent/{id}/...
	mux.Handle("/agent/", w.reverseProxy)
	// Agent management endpoints.
	mux.HandleFunc("GET /api/agents", w.handleListAgents)
	mux.HandleFunc("POST /api/agents", w.handleCreateAgent)
	mux.HandleFunc("POST /api/agents/external", w.handleAddExternal)
	mux.HandleFunc("PUT /api/agents/order", w.handleReorderAgents)
	mux.HandleFunc("PUT /api/agents/{id}", w.handleUpdateAgent)
	mux.HandleFunc("POST /api/agents/{id}/start", w.handleStartAgent)
	mux.HandleFunc("POST /api/agents/{id}/stop", w.handleStopAgent)
	mux.HandleFunc("DELETE /api/agents/{id}", w.handleDeleteAgent)
	// Agent描述 (FEATURE-538): the description lives in the agent's workspace
	// IDENTITY.md, not in the hub registry.
	mux.HandleFunc("GET /api/agents/{id}/identity", w.handleGetAgentIdentity)
	// Agent-config helpers (detection + defaults + version).
	mux.HandleFunc("GET /api/agent-defaults", w.handleAgentDefaults)
	mux.HandleFunc("GET /api/co-shell-locations", w.handleCoShellLocations)
	mux.HandleFunc("GET /api/config-candidates", w.handleConfigCandidates)
	mux.HandleFunc("GET /api/agent-version", w.handleAgentVersion)
	mux.HandleFunc("GET /api/remote-defaults", w.handleRemoteDefaults)
	mux.HandleFunc("GET /api/hub-info", w.handleHubInfo)
	// Message board (FEATURE-530): conversations and their messages for the
	// Chat pane. Agents post through the board protocol; the hub Web UI can
	// additionally post into a conversation as the human user (FEATURE-537).
	mux.HandleFunc("GET /api/board/conversations", w.handleBoardConversations)
	mux.HandleFunc("GET /api/board/conversations/{key}/messages", w.handleBoardMessages)
	mux.HandleFunc("POST /api/board/conversations/{key}/messages", w.handleBoardPostMessage)
	// Remote-access settings (TLS/whitelist/access key).
	mux.HandleFunc("GET /api/settings", w.handleGetSettings)
	mux.HandleFunc("PUT /api/settings", w.handlePutSettings)
	// The access-control middleware enforces the whitelist and the optional
	// access key (required from hosts outside the whitelist, or from all hosts
	// when RequireKey is set).
	handler := http.Handler(mux)
	handler = w.accessControl(handler)
	handler = w.requestLog(handler)
	w.httpSrv = &http.Server{Handler: handler}
	return w
}

// SetBoard attaches the message board (FEATURE-530) so the Web UI can show
// board conversations in its Chat pane. It must be called before Serve.
func (w *WebUI) SetBoard(b *Board) { w.board = b }

// requestLog logs every HTTP request to stdout with its time, client address
// and request URI, so remote access (e.g. from the mobile client) is traceable.
func (w *WebUI) requestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		log.Printf("webui: %s %s %s from %s",
			time.Now().Format("2006-01-02 15:04:05"), r.Method, r.URL.RequestURI(), clientIP(r))
		next.ServeHTTP(rw, r)
	})
}

// clientIP returns the client's IP address, honouring X-Forwarded-For when the
// hub sits behind a reverse proxy, and falling back to the remote address.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			xff = xff[:i]
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// Listen binds the HTTP listener.
func (w *WebUI) Listen() error {
	ln, err := net.Listen("tcp", w.cfg.ListenAddr)
	if err != nil {
		return err
	}
	w.ln = ln
	return nil
}

// Addr returns the bound listener address.
func (w *WebUI) Addr() net.Addr {
	if w.ln == nil {
		return nil
	}
	return w.ln.Addr()
}

// Serve accepts HTTP(S) requests until the server is closed. When TLS is
// enabled in settings the Web UI is served over https (replacing plain http).
func (w *WebUI) Serve() error {
	if w.ln == nil {
		return errors.New("webui: Serve called before Listen")
	}
	if w.settings != nil && w.settings.TLSEnabled {
		tlsCfg, err := w.settings.TLSConfig()
		if err != nil {
			return err
		}
		if tlsCfg != nil {
			w.httpSrv.TLSConfig = tlsCfg
			return w.httpSrv.ServeTLS(w.ln, "", "")
		}
	}
	return w.httpSrv.Serve(w.ln)
}

// Close shuts down the Web UI server.
func (w *WebUI) Close() error {
	if w.ln != nil {
		w.ln.Close()
	}
	return nil
}

// handleIndex serves the embedded iframe-shell frontend HTML.
func (w *WebUI) handleIndex(rw http.ResponseWriter, _ *http.Request) {
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	// FEATURE-559: substitute the resolved UI language so the page renders in the
	// right language on first paint (no flash of the fallback text).
	_, _ = rw.Write([]byte(strings.Replace(webIndexHTML, hubLangPlaceholder, string(w.lang), 1)))
}

// handleHubInfo returns this hub's version/build for the logo badge.
func (w *WebUI) handleHubInfo(rw http.ResponseWriter, _ *http.Request) {
	writeJSON(rw, http.StatusOK, map[string]string{"name": "co-shell-hub", "version": w.version, "build": w.build})
}

// agentView is the JSON shape returned by GET /api/agents. Name is the agent's
// self-reported display name (its IDENTITY.md agent-name, from the board
// roster), not a value the hub stores (FEATURE-538).
type agentView struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Workspace string `json:"workspace,omitempty"`
	Port      int    `json:"port,omitempty"`
	WSURL     string `json:"ws_url,omitempty"`
	Version   string `json:"version,omitempty"`
	Build     string `json:"build,omitempty"`
	Running   bool   `json:"running"`
	Connected bool   `json:"connected"`
	// Busy reports whether the agent is currently executing a task
	// (FEATURE-499), queried from the agent's /api/status.
	Busy    bool   `json:"busy"`
	CoShell string `json:"co_shell,omitempty"`
	// UseSharedConfig: true uses ~/.co-shell/config.json; false uses
	// {workspace}/config.json.
	UseSharedConfig bool   `json:"use_shared_config"`
	ExtraArgs       string `json:"extra_args,omitempty"`
	// YOLO reports whether the agent is started with --yolo (FEATURE-528).
	YOLO bool `json:"yolo"`
	// DownloadEnabled reports whether the agent is started with
	// --download-enabled (FEATURE-607).
	DownloadEnabled bool `json:"download_enabled"`
	// BoardEnabled reports whether the agent is registered on the hub message
	// board (FEATURE-557). Registration only happens while the agent's
	// board-enabled switch is on, so a roster entry marks a board-enabled
	// agent; an external agent's config file is not reachable from the hub.
	// BoardEnabled is kept for older UIs: it is true unless BoardState is
	// "unregistered".
	BoardEnabled bool `json:"board_enabled"`
	// BoardState is the three-state board indicator (FEATURE-557):
	// "unregistered", "online" or "offline".
	BoardState string `json:"board_state"`
}

// handleListAgents returns the registry agents with their running/connected
// status and co-shell version.
func (w *WebUI) handleListAgents(rw http.ResponseWriter, _ *http.Request) {
	specs := w.manager.Agents()
	connected := map[string]bool{}
	for _, id := range w.proxy.AgentIDs() {
		connected[id] = true
	}
	views := make([]agentView, 0, len(specs))
	for _, s := range specs {
		v := agentView{
			ID:              s.ID,
			Name:            w.agentDisplayName(s.ID, connected[s.ID]),
			Type:            s.Type,
			Workspace:       s.Workspace,
			Port:            s.Port,
			WSURL:           s.WSURL,
			Running:         w.manager.IsRunning(s.ID),
			Connected:       connected[s.ID],
			CoShell:         s.CoShell,
			UseSharedConfig: s.UseSharedConfig,
			ExtraArgs:       s.ExtraArgs,
			YOLO:            s.YOLO,
		}
		// Report the co-shell version for compatibility awareness.
		if s.Type == AgentTypeManaged {
			// FEATURE-527: a "latest" agent resolves its executable at start
			// time, so there is no fixed version to report here.
			if s.CoShell != CoShellLatest {
				v.Version, v.Build = coShellVersion(w.manager.CoShellPath())
			}
		} else if s.WSURL != "" {
			if ver, b, err := remoteVersion(wsURLToBase(s.WSURL)); err == nil {
				v.Version, v.Build = ver, b
			}
		}
		// FEATURE-557: three-state board indicator for the agent list. Only a
		// registered agent gets an icon: bright while online, grey while it is
		// merely known (registered earlier, link down) and none at all while it
		// is unregistered (board switch off).
		v.BoardState = BoardStateUnregistered
		if w.board != nil {
			if info, ok := w.board.Agent(s.ID); ok && info.Registered {
				v.BoardEnabled = true
				v.BoardState = BoardStateOffline
				if info.Online {
					v.BoardState = BoardStateOnline
				}
			}
		}
		// FEATURE-499: query the agent's busy state (task executing) for the
		// red breathing status light. Only connected agents are queried.
		if v.Connected {
			if s.Type == AgentTypeManaged && s.Port > 0 {
				v.Busy = agentBusy("http://127.0.0.1:" + strconv.Itoa(s.Port))
			} else if s.WSURL != "" {
				v.Busy = agentBusy(wsURLToBase(s.WSURL))
			}
		}
		views = append(views, v)
	}
	writeJSON(rw, http.StatusOK, map[string]interface{}{"agents": views})
}

// agentErrorStatus maps a manager error to the HTTP status the agent forms show
// (FEATURE-538: invalid IDs are 400, duplicate IDs are 409).
func agentErrorStatus(err error) int {
	switch {
	case errors.Is(err, ErrAgentNotFound):
		return http.StatusNotFound
	case errors.Is(err, ErrInvalidAgentID), errors.Is(err, ErrInvalidAgent), errors.Is(err, ErrInvalidAgentOrder):
		return http.StatusBadRequest
	case errors.Is(err, ErrAgentExists):
		return http.StatusConflict
	}
	return http.StatusConflict
}

// agentDisplayName returns the display name an agent reported on the board
// (FEATURE-538). Only a currently connected agent's name is trusted: a cached
// roster entry of an offline agent would otherwise keep showing a stale name
// (e.g. "co-shell" left over from a previous run). Otherwise the hub-configured
// ID is the default (FIX-542).
func (w *WebUI) agentDisplayName(id string, connected bool) string {
	if connected && w.board != nil {
		if info, ok := w.board.Agent(id); ok {
			if name := strings.TrimSpace(info.Name); name != "" {
				return name
			}
		}
	}
	return id
}

// createAgentRequest is the POST /api/agents body.
type createAgentRequest struct {
	ID string `json:"id"`
	// FIX-558: the create form no longer carries an Agent描述; the hub writes the
	// agent's ID into {workspace}/IDENTITY.md (`## agent-name`) instead.
	Workspace    string `json:"workspace"`
	CoShell      string `json:"co_shell,omitempty"`
	ConfigPath   string `json:"config_path,omitempty"`
	CreateConfig bool   `json:"create_config"`
	Port         int    `json:"port,omitempty"`
	// UseSharedConfig: true uses ~/.co-shell/config.json; false uses
	// {workspace}/config.json (copied from ~/.co-shell/config.json when absent,
	// or created empty if the shared config does not exist either).
	UseSharedConfig bool   `json:"use_shared_config"`
	ExtraArgs       string `json:"extra_args,omitempty"`
	// YOLO starts the agent with --yolo (auto-approve every tool call).
	YOLO bool `json:"yolo"`
	// DownloadEnabled starts the agent with --download-enabled (FEATURE-607),
	// allowing remote/mobile clients to download workspace files.
	DownloadEnabled bool `json:"download_enabled"`
}

// handleCreateAgent creates a managed agent (workspace + optional config.json).
func (w *WebUI) handleCreateAgent(rw http.ResponseWriter, r *http.Request) {
	var req createAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	if strings.TrimSpace(req.ID) == "" || req.Workspace == "" {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "id and workspace are required"})
		return
	}
	spec, err := w.manager.CreateManaged(req.ID, req.Workspace, req.CoShell, req.ConfigPath, req.CreateConfig, req.UseSharedConfig, req.Port, req.ExtraArgs, req.YOLO, req.DownloadEnabled)
	if err != nil {
		writeJSON(rw, agentErrorStatus(err), map[string]string{"error": err.Error()})
		return
	}
	// FIX-558: nothing about the description is collected here. The agent's ID
	// is written into {workspace}/IDENTITY.md as its initial `## agent-name`, so
	// co-shell starts with a name it can identify itself by.
	if err := WriteIdentityName(spec.Workspace, spec.ID); err != nil {
		log.Printf("webui: agent %s created but identity write failed: %v", spec.ID, err)
	}
	writeJSON(rw, http.StatusCreated, spec)
}

// addExternalRequest is the POST /api/agents/external body. An external agent
// has no local workspace, so it carries no Agent描述 (the hub shows the value
// the agent reports on the board, read-only).
type addExternalRequest struct {
	ID    string `json:"id"`
	WSURL string `json:"ws_url"`
}

// handleAddExternal registers an external agent and connects to it.
func (w *WebUI) handleAddExternal(rw http.ResponseWriter, r *http.Request) {
	var req addExternalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	if req.ID == "" || req.WSURL == "" {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "id and ws_url are required"})
		return
	}
	spec, err := w.manager.AddExternal(req.ID, req.WSURL)
	if err != nil {
		writeJSON(rw, agentErrorStatus(err), map[string]string{"error": err.Error()})
		return
	}
	// An external agent may be temporarily offline; register it regardless and
	// attempt to connect. A failed connect is logged, not fatal.
	if err := w.proxy.AddAgent(AgentConfig{ID: spec.ID, WSURL: spec.WSURL}); err != nil {
		log.Printf("webui: external agent %s registered but connect failed: %v", spec.ID, err)
	}
	writeJSON(rw, http.StatusCreated, spec)
}

// updateAgentRequest is the PUT /api/agents/{id} body (FEATURE-520). Every field
// is a pointer so an omitted field is left unchanged; the agent's id and type
// are immutable.
type updateAgentRequest struct {
	Workspace       *string `json:"workspace"`
	Port            *int    `json:"port"`
	CoShell         *string `json:"co_shell"`
	UseSharedConfig *bool   `json:"use_shared_config"`
	ExtraArgs       *string `json:"extra_args"`
	YOLO            *bool   `json:"yolo"`
	DownloadEnabled *bool   `json:"download_enabled"`
	WSURL           *string `json:"ws_url"`
}

// handleUpdateAgent updates an agent's editable fields and persists the registry
// (FEATURE-520). A running managed agent is deliberately not restarted: port /
// workspace / co-shell / extra-arg changes take effect on the next start, which
// the frontend tells the user about.
func (w *WebUI) handleUpdateAgent(rw http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req updateAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	spec, err := w.manager.Update(id, AgentPatch{
		Workspace:       req.Workspace,
		Port:            req.Port,
		CoShell:         req.CoShell,
		UseSharedConfig: req.UseSharedConfig,
		ExtraArgs:       req.ExtraArgs,
		YOLO:            req.YOLO,
		DownloadEnabled: req.DownloadEnabled,
		WSURL:           req.WSURL,
	})
	if err != nil {
		writeJSON(rw, agentErrorStatus(err), map[string]string{"error": err.Error()})
		return
	}
	// An external agent's endpoint lives in the proxy connection: re-dial it so
	// a changed ws_url takes effect immediately. A failed dial is logged only.
	if spec.Type == AgentTypeExternal {
		w.proxy.RemoveAgent(spec.ID)
		if err := w.proxy.AddAgent(AgentConfig{ID: spec.ID, WSURL: spec.WSURL}); err != nil {
			log.Printf("webui: external agent %s updated but reconnect failed: %v", spec.ID, err)
		}
	}
	writeJSON(rw, http.StatusOK, spec)
}

// reorderAgentsRequest is the PUT /api/agents/order body (FEATURE-613).
type reorderAgentsRequest struct {
	IDs []string `json:"ids"`
}

// handleReorderAgents rewrites the agent registry order to match the given id
// list (FEATURE-613) and persists it, so a drag in the agent list survives a
// hub restart and reaches every other browser. The list must cover every
// registered agent exactly once; anything else is a 400 and leaves the registry
// untouched (Manager.Reorder stays atomic).
func (w *WebUI) handleReorderAgents(rw http.ResponseWriter, r *http.Request) {
	var req reorderAgentsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	if err := w.manager.Reorder(req.IDs); err != nil {
		writeJSON(rw, agentErrorStatus(err), map[string]string{"error": err.Error()})
		return
	}
	writeJSON(rw, http.StatusOK, map[string]bool{"ok": true})
}

// agentIdentityView is the JSON shape of GET /api/agents/{id}/identity.
type agentIdentityView struct {
	AgentID          string `json:"agent_id"`
	AgentDescription string `json:"agent_description"`
	// Source reports where the description came from: "agent" (the value the
	// agent reported when it last registered on the board).
	Source string `json:"source"`
	// Readonly is always true (FIX-558): the hub never writes a description.
	Readonly bool `json:"readonly"`
}

// identityFor builds one agent's identity view. FIX-558: the description shown is
// the value the agent reported when it last registered on the board — the hub no
// longer reads or writes {workspace}/IDENTITY.md here, so the field is read-only
// everywhere (each agent owns its description under 「身份与个性」). A roster entry
// whose duty came from the hub-side fallback (duty_source=hub) counts as "never
// reported".
func (w *WebUI) identityFor(spec AgentSpec) agentIdentityView {
	view := agentIdentityView{AgentID: spec.ID, Source: DutySourceAgent, Readonly: true}
	if w.board == nil {
		return view
	}
	info, ok := w.board.Agent(spec.ID)
	if !ok || info.DutyFrom == DutySourceHub {
		return view
	}
	view.AgentDescription = info.Duty
	return view
}

// handleGetAgentIdentity returns one agent's Agent描述 (FEATURE-538).
func (w *WebUI) handleGetAgentIdentity(rw http.ResponseWriter, r *http.Request) {
	spec, ok := w.manager.Spec(r.PathValue("id"))
	if !ok {
		writeJSON(rw, http.StatusNotFound, map[string]string{"error": "agent not found"})
		return
	}
	writeJSON(rw, http.StatusOK, w.identityFor(spec))
}

// handleStartAgent starts a managed agent subprocess and connects it to the
// proxy (retrying the dial until the serve port is ready).
func (w *WebUI) handleStartAgent(rw http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	wsURL, err := w.manager.Start(id)
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := w.connectWithRetry(id, wsURL); err != nil {
		writeJSON(rw, http.StatusInternalServerError, map[string]string{"error": "started but connect failed: " + err.Error()})
		return
	}
	writeJSON(rw, http.StatusOK, map[string]string{"id": id, "ws_url": wsURL})
}

// handleStopAgent stops a managed agent and disconnects it from the proxy.
func (w *WebUI) handleStopAgent(rw http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	w.proxy.RemoveAgent(id)
	if err := w.manager.Stop(id); err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	// FEATURE-557: a stopped agent leaves the board right away — waiting for the
	// socket to be reaped would keep showing it online. This is idempotent with
	// the disconnect path (dropAgentLink ends up here too).
	w.proxy.handleAgentGone(id)
	writeJSON(rw, http.StatusOK, map[string]string{"id": id})
}

// handleDeleteAgent removes an agent (stopping/disconnecting it first).
func (w *WebUI) handleDeleteAgent(rw http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	w.proxy.RemoveAgent(id)
	if err := w.manager.Remove(id); err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(rw, http.StatusOK, map[string]string{"id": id})
}

// connectWithRetry dials an agent WS URL, retrying briefly to allow a freshly
// started co-shell --serve process to begin listening.
func (w *WebUI) connectWithRetry(id, wsURL string) error {
	cfg := AgentConfig{ID: id, WSURL: wsURL}
	var lastErr error
	for i := 0; i < 20; i++ {
		if err := w.proxy.AddAgent(cfg); err == nil {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(250 * time.Millisecond)
	}
	return lastErr
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(rw http.ResponseWriter, status int, v interface{}) {
	rw.Header().Set("Content-Type", "application/json; charset=utf-8")
	rw.WriteHeader(status)
	_ = json.NewEncoder(rw).Encode(v)
}

// accessHeader is the HTTP header carrying the access key.
const accessHeader = "X-Access-Key"

// accessCookie is the cookie name carrying the access key. It lets clients
// (e.g. the native mobile WKWebView shell, FEATURE-486) authenticate by
// injecting a cookie before loading the page, since a WebView cannot attach a
// custom header to every sub-resource / WebSocket request but does send cookies
// automatically (including on the WebSocket handshake).
const accessCookie = "access_key"

// accessKeyFromRequest returns the access key presented by the client, either
// via the X-Access-Key header or the access_key cookie (header takes priority).
func accessKeyFromRequest(r *http.Request) string {
	if k := r.Header.Get(accessHeader); k != "" {
		return k
	}
	if c, err := r.Cookie(accessCookie); err == nil && c.Value != "" {
		return c.Value
	}
	return ""
}

// accessControl enforces the Web UI access policy from settings:
//   - An empty whitelist means no IP restriction (the listen address itself
//     already limits reachability, e.g. loopback-only by default).
//   - A client whose IP is in the whitelist is allowed (unless RequireKey is
//     set, which forces key auth for every client).
//   - A client outside the whitelist must present the access key (via the
//     X-Access-Key header or the access_key cookie); otherwise it gets a 401 so
//     the frontend can prompt for the key. If no access key is configured,
//     out-of-whitelist clients are rejected with 403.
func (w *WebUI) accessControl(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		s := w.settings
		if s == nil {
			next.ServeHTTP(rw, r)
			return
		}
		nets := parseWhitelist(s.Whitelist)
		// An empty whitelist trusts only loopback (127.0.0.1/::1) by default;
		// other hosts must present the access key. A non-empty whitelist allows
		// the listed IPs/CIDRs without a key.
		allowed := ipAllowed(r.RemoteAddr, nets)
		if len(nets) == 0 {
			allowed = isLoopback(r.RemoteAddr)
		}
		needKey := s.RequireKey || (!allowed && s.AccessKey != "")
		if needKey {
			if s.AccessKey == "" || accessKeyFromRequest(r) != s.AccessKey {
				writeJSON(rw, http.StatusUnauthorized, map[string]string{"error": "access key required", "need_key": "1"})
				return
			}
			next.ServeHTTP(rw, r)
			return
		}
		if !allowed {
			writeJSON(rw, http.StatusForbidden, map[string]string{"error": "forbidden"})
			return
		}
		next.ServeHTTP(rw, r)
	})
}

// handleGetSettings returns the current remote-access settings. The access key
// is masked so it is never echoed back to the browser.
func (w *WebUI) handleGetSettings(rw http.ResponseWriter, _ *http.Request) {
	if w.settings == nil {
		writeJSON(rw, http.StatusOK, map[string]interface{}{"settings": map[string]interface{}{}})
		return
	}
	writeJSON(rw, http.StatusOK, map[string]interface{}{"settings": w.settings.view()})
}

// handlePutSettings updates the remote-access settings from the request body
// and persists them. The access key is only overwritten when a non-empty value
// is supplied (an empty field keeps the existing key).
func (w *WebUI) handlePutSettings(rw http.ResponseWriter, r *http.Request) {
	if w.settings == nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "settings unavailable"})
		return
	}
	var req struct {
		TLSEnabled bool     `json:"tls_enabled"`
		CertFile   string   `json:"cert_file"`
		KeyFile    string   `json:"key_file"`
		Whitelist  []string `json:"whitelist"`
		AccessKey  string   `json:"access_key"`
		RequireKey bool     `json:"require_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	w.settings.TLSEnabled = req.TLSEnabled
	w.settings.CertFile = req.CertFile
	w.settings.KeyFile = req.KeyFile
	w.settings.Whitelist = req.Whitelist
	w.settings.RequireKey = req.RequireKey
	if req.AccessKey != "" {
		w.settings.AccessKey = req.AccessKey
	}
	if err := w.settings.Save(); err != nil {
		writeJSON(rw, http.StatusInternalServerError, map[string]string{"error": "save failed: " + err.Error()})
		return
	}
	writeJSON(rw, http.StatusOK, map[string]interface{}{"settings": w.settings.view()})
}

// view returns a copy of the settings safe to send to the browser (access key
// masked). settings_dir is the directory holding the settings file, used by the
// frontend to show where a self-signed cert would be generated.
func (s *Settings) view() map[string]interface{} {
	key := ""
	if s.AccessKey != "" {
		key = "********"
	}
	dir := filepath.Join(homeDir(), ".co-shell")
	if dir == ".co-shell" {
		dir = "."
	}
	return map[string]interface{}{
		"tls_enabled":  s.TLSEnabled,
		"cert_file":    s.CertFile,
		"key_file":     s.KeyFile,
		"whitelist":    s.Whitelist,
		"access_key":   key,
		"require_key":  s.RequireKey,
		"settings_dir": dir,
	}
}

// parseWhitelist converts whitelist entries (IPs or CIDR) into net.IPNet.
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

// ipAllowed reports whether the client address is within any whitelist network.
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

// isLoopback reports whether the client address is a loopback IP (127.0.0.1 or
// ::1). Used as the default trust when the whitelist is empty.
func isLoopback(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}

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
	mux.HandleFunc("PUT /api/agents/{id}", w.handleUpdateAgent)
	mux.HandleFunc("POST /api/agents/{id}/start", w.handleStartAgent)
	mux.HandleFunc("POST /api/agents/{id}/stop", w.handleStopAgent)
	mux.HandleFunc("DELETE /api/agents/{id}", w.handleDeleteAgent)
	// Agent-config helpers (detection + defaults + version).
	mux.HandleFunc("GET /api/agent-defaults", w.handleAgentDefaults)
	mux.HandleFunc("GET /api/co-shell-locations", w.handleCoShellLocations)
	mux.HandleFunc("GET /api/config-candidates", w.handleConfigCandidates)
	mux.HandleFunc("GET /api/agent-version", w.handleAgentVersion)
	mux.HandleFunc("GET /api/remote-defaults", w.handleRemoteDefaults)
	mux.HandleFunc("GET /api/hub-info", w.handleHubInfo)
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
	_, _ = rw.Write([]byte(webIndexHTML))
}

// handleHubInfo returns this hub's version/build for the logo badge.
func (w *WebUI) handleHubInfo(rw http.ResponseWriter, _ *http.Request) {
	writeJSON(rw, http.StatusOK, map[string]string{"name": "co-shell-hub", "version": w.version, "build": w.build})
}

// agentView is the JSON shape returned by GET /api/agents.
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
	Busy bool `json:"busy"`
	CoShell   string `json:"co_shell,omitempty"`
	// UseSharedConfig: true uses ~/.co-shell/config.json; false uses
	// {workspace}/config.json.
	UseSharedConfig bool   `json:"use_shared_config"`
	ExtraArgs       string `json:"extra_args,omitempty"`
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
			ID:        s.ID,
			Name:      s.Name,
			Type:      s.Type,
			Workspace: s.Workspace,
			Port:      s.Port,
			WSURL:     s.WSURL,
			Running:   w.manager.IsRunning(s.ID),
			Connected: connected[s.ID],
			CoShell:         s.CoShell,
			UseSharedConfig: s.UseSharedConfig,
			ExtraArgs:       s.ExtraArgs,
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

// createAgentRequest is the POST /api/agents body.
type createAgentRequest struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Workspace    string `json:"workspace"`
	CoShell      string `json:"co_shell,omitempty"`
	ConfigPath   string `json:"config_path,omitempty"`
	CreateConfig bool   `json:"create_config"`
	Port         int    `json:"port,omitempty"`
	// UseSharedConfig: true uses ~/.co-shell/config.json; false uses
	// {workspace}/config.json (created empty if absent).
	UseSharedConfig bool   `json:"use_shared_config"`
	ExtraArgs       string `json:"extra_args,omitempty"`
}

// handleCreateAgent creates a managed agent (workspace + optional config.json).
func (w *WebUI) handleCreateAgent(rw http.ResponseWriter, r *http.Request) {
	var req createAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	if req.ID == "" || req.Workspace == "" {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "id and workspace are required"})
		return
	}
	name := req.Name
	if name == "" {
		name = req.ID
	}
	spec, err := w.manager.CreateManaged(req.ID, name, req.Workspace, req.CoShell, req.ConfigPath, req.CreateConfig, req.UseSharedConfig, req.Port, req.ExtraArgs)
	if err != nil {
		writeJSON(rw, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(rw, http.StatusCreated, spec)
}

// addExternalRequest is the POST /api/agents/external body.
type addExternalRequest struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
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
	name := req.Name
	if name == "" {
		name = req.ID
	}
	spec, err := w.manager.AddExternal(req.ID, name, req.WSURL)
	if err != nil {
		writeJSON(rw, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	// An external agent may be temporarily offline; register it regardless and
	// attempt to connect. A failed connect is logged, not fatal.
	if err := w.proxy.AddAgent(AgentConfig{ID: spec.ID, Name: spec.Name, WSURL: spec.WSURL}); err != nil {
		log.Printf("webui: external agent %s registered but connect failed: %v", spec.ID, err)
	}
	writeJSON(rw, http.StatusCreated, spec)
}

// updateAgentRequest is the PUT /api/agents/{id} body (FEATURE-520). Every field
// is a pointer so an omitted field is left unchanged; the agent's id and type
// are immutable.
type updateAgentRequest struct {
	Name            *string `json:"name"`
	Workspace       *string `json:"workspace"`
	Port            *int    `json:"port"`
	CoShell         *string `json:"co_shell"`
	UseSharedConfig *bool   `json:"use_shared_config"`
	ExtraArgs       *string `json:"extra_args"`
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
		Name:            req.Name,
		Workspace:       req.Workspace,
		Port:            req.Port,
		CoShell:         req.CoShell,
		UseSharedConfig: req.UseSharedConfig,
		ExtraArgs:       req.ExtraArgs,
		WSURL:           req.WSURL,
	})
	if err != nil {
		status := http.StatusConflict
		switch {
		case errors.Is(err, ErrAgentNotFound):
			status = http.StatusNotFound
		case errors.Is(err, ErrInvalidAgent):
			status = http.StatusBadRequest
		}
		writeJSON(rw, status, map[string]string{"error": err.Error()})
		return
	}
	// An external agent's endpoint lives in the proxy connection: re-dial it so
	// a changed ws_url takes effect immediately. A failed dial is logged only.
	if spec.Type == AgentTypeExternal {
		w.proxy.RemoveAgent(spec.ID)
		if err := w.proxy.AddAgent(AgentConfig{ID: spec.ID, Name: spec.Name, WSURL: spec.WSURL}); err != nil {
			log.Printf("webui: external agent %s updated but reconnect failed: %v", spec.ID, err)
		}
	}
	writeJSON(rw, http.StatusOK, spec)
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
	cfg := AgentConfig{ID: id, Name: id, WSURL: wsURL}
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

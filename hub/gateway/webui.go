package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
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
// frontend and bridges each browser WebSocket connection into the Proxy so the
// browser can list/switch agents and exchange messages with the current agent.
// It also exposes agent-management HTTP endpoints backed by the Manager.
type WebUI struct {
	cfg     WebUIConfig
	proxy   *Proxy
	manager *Manager
	ctx     context.Context

	httpSrv *http.Server
	ln      net.Listener
}

// NewWebUI creates a WebUI server bound to the given proxy and agent manager.
func NewWebUI(cfg WebUIConfig, proxy *Proxy, manager *Manager) *WebUI {
	ctx := context.Background()
	w := &WebUI{cfg: cfg, proxy: proxy, manager: manager, ctx: ctx}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", w.handleIndex)
	mux.HandleFunc("GET /ws", w.handleWS)
	// Agent management endpoints.
	mux.HandleFunc("GET /api/agents", w.handleListAgents)
	mux.HandleFunc("POST /api/agents", w.handleCreateAgent)
	mux.HandleFunc("POST /api/agents/external", w.handleAddExternal)
	mux.HandleFunc("POST /api/agents/{id}/start", w.handleStartAgent)
	mux.HandleFunc("POST /api/agents/{id}/stop", w.handleStopAgent)
	mux.HandleFunc("DELETE /api/agents/{id}", w.handleDeleteAgent)
	handler := http.Handler(mux)
	if len(cfg.Whitelist) > 0 {
		handler = w.whitelistMiddleware(handler, cfg.Whitelist)
	}
	w.httpSrv = &http.Server{Handler: handler}
	return w
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

// Serve accepts HTTP requests until the server is closed.
func (w *WebUI) Serve() error {
	if w.ln == nil {
		return errors.New("webui: Serve called before Listen")
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

// handleIndex serves the embedded frontend HTML.
func (w *WebUI) handleIndex(rw http.ResponseWriter, _ *http.Request) {
	rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = rw.Write([]byte(webIndexHTML))
}

// handleWS upgrades a browser connection and bridges it into the proxy.
func (w *WebUI) handleWS(rw http.ResponseWriter, r *http.Request) {
	ws, err := upgradeWS(rw, r)
	if err != nil {
		log.Printf("webui: ws upgrade failed: %v", err)
		return
	}
	defer ws.Close()

	// Bridge the browser WS to the proxy via an in-memory pipe. One end is a
	// gateway Conn registered with the proxy; the other end is drained by a
	// goroutine that forwards proxy envelopes to the browser.
	proxyEnd, browserEnd := net.Pipe()
	defer proxyEnd.Close()
	defer browserEnd.Close()

	conn := &Conn{conn: proxyEnd}
	w.proxy.OnConnect(w.ctx, conn)
	defer w.proxy.OnDisconnect(w.ctx, conn)

	// Goroutine: read envelopes from the proxy side and push to the browser.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			var env Envelope
			if err := readFrame(browserEnd, maxFrameSize, &env); err != nil {
				return
			}
			// Forward the envelope to the browser as a JSON text message.
			data, _ := json.Marshal(&env)
			if err := ws.WriteText(data); err != nil {
				return
			}
		}
	}()

	// Main loop: read browser messages and feed them to the proxy.
	for {
		msg, err := ws.ReadMessage()
		if err != nil {
			return
		}
		var env Envelope
		if err := json.Unmarshal(msg, &env); err != nil {
			continue
		}
		// Write the envelope to the proxy side of the pipe.
		if err := writeFrame(proxyEnd, &env); err != nil {
			return
		}
	}
}

// agentView is the JSON shape returned by GET /api/agents.
type agentView struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Workspace string `json:"workspace,omitempty"`
	Port      int    `json:"port,omitempty"`
	WSURL     string `json:"ws_url,omitempty"`
	Running   bool   `json:"running"`
	Connected bool   `json:"connected"`
}

// handleListAgents returns the registry agents with their running/connected
// status.
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
	CreateConfig bool   `json:"create_config"`
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
	spec, err := w.manager.CreateManaged(req.ID, name, req.Workspace, req.CreateConfig)
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

// whitelistMiddleware rejects requests whose client IP is not whitelisted.
func (w *WebUI) whitelistMiddleware(next http.Handler, whitelist []string) http.Handler {
	nets := parseWhitelist(whitelist)
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		if !ipAllowed(r.RemoteAddr, nets) {
			http.Error(rw, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(rw, r)
	})
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

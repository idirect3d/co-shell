package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"strings"
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
type WebUI struct {
	cfg   WebUIConfig
	proxy *Proxy
	ctx   context.Context

	httpSrv *http.Server
	ln      net.Listener
}

// NewWebUI creates a WebUI server bound to the given proxy.
func NewWebUI(cfg WebUIConfig, proxy *Proxy) *WebUI {
	ctx := context.Background()
	w := &WebUI{cfg: cfg, proxy: proxy, ctx: ctx}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", w.handleIndex)
	mux.HandleFunc("GET /ws", w.handleWS)
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

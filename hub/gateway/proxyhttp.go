package gateway

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
)

// agentPrefix is the URL prefix under which each agent's co-shell Web UI is
// reverse-proxied: /agent/{id}/... (FEATURE-484). The prefix is stripped before
// forwarding to the agent's own root path.
const agentPrefix = "/agent/"

// httpReverseProxy forwards /agent/{id}/... requests to the corresponding
// co-shell instance's HTTP server (http://127.0.0.1:{port}/...). It handles
// both plain HTTP (static assets + /api) and WebSocket upgrades (/ws) via
// httputil.ReverseProxy, which tunnels upgraded connections transparently.
type httpReverseProxy struct {
	mu      sync.RWMutex
	manager *Manager
	// proxies caches a ReverseProxy per agent base URL so a live transport is
	// reused across requests to the same agent.
	proxies map[string]*httputil.ReverseProxy
}

// newHTTPReverseProxy creates a reverse proxy backed by the agent manager.
func newHTTPReverseProxy(mgr *Manager) *httpReverseProxy {
	return &httpReverseProxy{manager: mgr, proxies: make(map[string]*httputil.ReverseProxy)}
}

// ServeHTTP implements http.Handler. It parses /agent/{id}/... and proxies the
// remainder to the matching agent, or returns 404 when the agent is unknown.
func (p *httpReverseProxy) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	rest, ok := strings.CutPrefix(r.URL.Path, agentPrefix)
	if !ok {
		http.NotFound(rw, r)
		return
	}
	// rest is "{id}/..." — split the id from the remaining path.
	id, tail, ok := strings.Cut(rest, "/")
	if !ok || id == "" {
		http.NotFound(rw, r)
		return
	}
	base, ok := p.agentBaseURL(id)
	if !ok {
		http.NotFound(rw, r)
		return
	}
	// Rewrite the request path to the agent's root (strip the /agent/{id}
	// prefix). The tail keeps its leading slash, e.g. "/static/app.js".
	r.URL.Path = "/" + tail
	if r.URL.RawPath != "" {
		r.URL.RawPath = ""
	}
	proxy := p.proxyFor(base)
	proxy.ServeHTTP(rw, r)
}

// agentBaseURL resolves an agent id to its HTTP base URL (scheme://host:port).
func (p *httpReverseProxy) agentBaseURL(id string) (string, bool) {
	for _, s := range p.manager.Agents() {
		if s.ID != id {
			continue
		}
		if s.Type == AgentTypeManaged && s.Port > 0 {
			return fmt.Sprintf("http://127.0.0.1:%d", s.Port), true
		}
		if s.Type == AgentTypeExternal && s.WSURL != "" {
			// Derive the HTTP base from the ws:// URL.
			if u, err := url.Parse(s.WSURL); err == nil {
				scheme := "http"
				if u.Scheme == "wss" {
					scheme = "https"
				}
				return scheme + "://" + u.Host, true
			}
		}
	}
	return "", false
}

// proxyFor returns (creating if needed) the ReverseProxy for a base URL.
func (p *httpReverseProxy) proxyFor(base string) *httputil.ReverseProxy {
	p.mu.RLock()
	proxy := p.proxies[base]
	p.mu.RUnlock()
	if proxy != nil {
		return proxy
	}
	target, err := url.Parse(base)
	if err != nil {
		// base is always a valid URL built internally; fall back to a no-op.
		return &httputil.ReverseProxy{}
	}
	proxy = httputil.NewSingleHostReverseProxy(target)
	p.mu.Lock()
	p.proxies[base] = proxy
	p.mu.Unlock()
	return proxy
}

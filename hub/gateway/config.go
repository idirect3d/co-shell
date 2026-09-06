// Package gateway implements the new co-shell-hub gateway architecture
// (FEATURE-484): a traditional TCP service with API Key authentication that
// proxies messages to multiple co-shell agents over WebSocket.
//
// This package evolves independently from the legacy UDP-based hub code
// (hub/hub.go etc.), which is left untouched.
package gateway

// maxFrameSize is the hard upper bound for a single JSON frame payload.
// It guards against memory exhaustion from malicious oversized frames.
const maxFrameSize = 16 << 20 // 16 MiB

// Config holds the gateway TCP service configuration.
type Config struct {
	// ListenAddr is the TCP listen address, e.g. ":12801".
	ListenAddr string `json:"listen_addr"`
	// APIKey is the shared secret that clients must present to authenticate.
	// Requests without a matching API key are rejected.
	APIKey string `json:"api_key"`
	// MaxFrameSize caps a single JSON frame payload in bytes (default 1 MiB).
	MaxFrameSize int `json:"max_frame_size,omitempty"`
	// AuthTimeout is the seconds allowed for a client to complete the auth
	// handshake before the connection is closed (default 10).
	AuthTimeout int `json:"auth_timeout,omitempty"`
}

// DefaultConfig returns a gateway Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		ListenAddr:   ":12801",
		MaxFrameSize: 1 << 20, // 1 MiB
		AuthTimeout:  10,
	}
}

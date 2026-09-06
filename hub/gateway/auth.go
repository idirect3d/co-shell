package gateway

import (
	"crypto/subtle"
)

// Authenticator validates client-supplied API keys.
type Authenticator struct {
	key string
}

// NewAuthenticator returns an Authenticator that accepts the given API key.
// An empty key disables authentication (all requests pass) — useful only for
// local development behind a trusted VPN.
func NewAuthenticator(apiKey string) *Authenticator {
	return &Authenticator{key: apiKey}
}

// Authenticate reports whether the supplied key matches the configured one.
// Comparison is constant-time to resist timing attacks.
func (a *Authenticator) Authenticate(supplied string) bool {
	if a.key == "" {
		return true
	}
	if len(a.key) != len(supplied) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a.key), []byte(supplied)) == 1
}

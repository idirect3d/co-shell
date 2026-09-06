package gateway

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// Settings holds the hub's remote-access configuration: TLS (https) serving,
// the Web UI access whitelist, and an optional access key required from hosts
// outside the whitelist. It is persisted as JSON so the Web UI settings panel
// can read and update it at runtime (FEATURE-485).
type Settings struct {
	// TLSEnabled enables https serving (replaces plain http). When true the
	// hub serves the Web UI over TLS using CertFile/KeyFile (or an auto
	// generated self-signed certificate when both are empty).
	TLSEnabled bool `json:"tls_enabled"`
	// CertFile/KeyFile are PEM certificate/key paths. Empty means the hub
	// auto-generates a self-signed certificate (stored under the settings dir).
	CertFile string `json:"cert_file,omitempty"`
	KeyFile  string `json:"key_file,omitempty"`
	// Whitelist restricts browser access to the given IPs/CIDR networks.
	// Empty means loopback only (default localhost access).
	Whitelist []string `json:"whitelist,omitempty"`
	// AccessKey is required from clients whose IP is not in the whitelist.
	// Empty disables key auth.
	AccessKey string `json:"access_key,omitempty"`
	// RequireKey forces key auth for ALL clients (including whitelisted ones).
	RequireKey bool `json:"require_key,omitempty"`

	// path is the settings file location (not serialized).
	path string `json:"-"`
}

// DefaultSettings returns a Settings with loopback-only access and TLS off.
func DefaultSettings() *Settings {
	return &Settings{}
}

// LoadSettings reads the settings JSON file at path. If the file is absent a
// default Settings is returned (not persisted until first Save).
func LoadSettings(path string) *Settings {
	s := DefaultSettings()
	s.path = path
	data, err := os.ReadFile(path)
	if err != nil {
		return s
	}
	if err := json.Unmarshal(data, s); err != nil {
		log.Printf("gateway: parse settings %s: %v", path, err)
	}
	return s
}

// Save persists the settings to the JSON file at s.path.
func (s *Settings) Save() error {
	if s.path == "" {
		return nil
	}
	if dir := filepath.Dir(s.path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o600)
}

// TLSConfig builds a *tls.Config for https serving. When no cert/key files are
// configured, or the configured files do not exist on disk, it generates (and
// persists) a self-signed certificate under ~/.co-shell/. Returns nil when TLS
// is disabled.
func (s *Settings) TLSConfig() (*tls.Config, error) {
	if !s.TLSEnabled {
		return nil, nil
	}
	certFile, keyFile := s.CertFile, s.KeyFile
	// If either path is empty or the cert file is missing on disk, fall back to
	// a self-signed cert under ~/.co-shell/ (generated on first use). This keeps
	// https working even when a stale/relative cert path was persisted.
	if certFile == "" || keyFile == "" || fileExists(certFile) == false {
		dir := filepath.Join(homeDir(), ".co-shell")
		if dir == ".co-shell" {
			dir = "."
		}
		certFile = filepath.Join(dir, "hub-cert.pem")
		keyFile = filepath.Join(dir, "hub-key.pem")
		if !fileExists(certFile) {
			if err := generateSelfSigned(certFile, keyFile); err != nil {
				return nil, err
			}
		}
		s.CertFile, s.KeyFile = certFile, keyFile
		_ = s.Save()
	}
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	return &tls.Config{Certificates: []tls.Certificate{cert}}, nil
}

// fileExists reports whether the path exists on disk.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// homeDir returns the current user's home directory (empty on error).
func homeDir() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return h
}

// generateSelfSigned writes a self-signed ECDSA certificate/key pair to the
// given PEM files (used for out-of-the-box https when no cert is uploaded).
func generateSelfSigned(certFile, keyFile string) error {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return err
	}
	tmpl := x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "co-shell-hub", Organization: []string{"co-shell"}},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil {
		return err
	}
	certOut, err := os.Create(certFile)
	if err != nil {
		return err
	}
	defer certOut.Close()
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
		return err
	}
	keyBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return err
	}
	keyOut, err := os.Create(keyFile)
	if err != nil {
		return err
	}
	defer keyOut.Close()
	return pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyBytes})
}

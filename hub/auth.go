// Author: L.Shuang
// Created: 2026-05-17
// Last Modified: 2026-05-17
//
// MIT License
//
// Copyright (c) 2026 L.Shuang
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package hub

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// ClientInfo represents a registered mobile client.
type ClientInfo struct {
	// Nickname is the display name of the mobile client.
	Nickname string `json:"nickname"`
	// PublicKey is the base64-encoded Ed25519 public key used as access credential.
	PublicKey string `json:"public_key"`
}

// AuthConfig holds the authentication configuration for hub-mobile communication.
// Hub holds a single private key. Mobile clients only need the hub's public key
// as an access credential. Multiple mobile clients can be registered, each with
// a nickname and their own public key.
type AuthConfig struct {
	// HubPrivateKey is the base64-encoded Ed25519 private key.
	HubPrivateKey string `json:"hub_private_key,omitempty"`
	// Clients is the list of registered mobile clients.
	Clients []ClientInfo `json:"clients,omitempty"`
	// clientsByKey is an in-memory index for fast lookup by public key.
	clientsByKey map[string]*ClientInfo
	mu           sync.RWMutex
}

// KeyPair holds the Ed25519 key pair.
type KeyPair struct {
	PrivateKey ed25519.PrivateKey
	PublicKey  ed25519.PublicKey
}

// GenerateKeyPair generates a new Ed25519 key pair.
func GenerateKeyPair() (*KeyPair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}
	return &KeyPair{
		PrivateKey: priv,
		PublicKey:  pub,
	}, nil
}

// LoadOrGenerateAuth loads auth config from file or generates new keys.
func LoadOrGenerateAuth(configPath string) (*AuthConfig, error) {
	auth := &AuthConfig{}

	// Try to load existing auth config
	data, err := os.ReadFile(configPath)
	if err == nil {
		var rawConfig map[string]interface{}
		if err := json.Unmarshal(data, &rawConfig); err == nil {
			if authData, ok := rawConfig["auth"].(map[string]interface{}); ok {
				if pk, ok := authData["hub_private_key"].(string); ok {
					auth.HubPrivateKey = pk
				}
				// Load clients list
				if clientsRaw, ok := authData["clients"].([]interface{}); ok {
					for _, c := range clientsRaw {
						if cm, ok := c.(map[string]interface{}); ok {
							client := ClientInfo{}
							if n, ok := cm["nickname"].(string); ok {
								client.Nickname = n
							}
							if pk, ok := cm["public_key"].(string); ok {
								client.PublicKey = pk
							}
							if client.Nickname != "" && client.PublicKey != "" {
								auth.Clients = append(auth.Clients, client)
							}
						}
					}
				}
			}
		}
	}

	// Generate keys if not present
	if auth.HubPrivateKey == "" {
		keyPair, err := GenerateKeyPair()
		if err != nil {
			return nil, err
		}
		auth.HubPrivateKey = base64.StdEncoding.EncodeToString(keyPair.PrivateKey)
	}

	// Build in-memory index
	auth.rebuildIndex()

	return auth, nil
}

// rebuildIndex rebuilds the in-memory client lookup index.
func (a *AuthConfig) rebuildIndex() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.clientsByKey = make(map[string]*ClientInfo)
	for i := range a.Clients {
		a.clientsByKey[a.Clients[i].PublicKey] = &a.Clients[i]
	}
}

// GetHubPublicKey returns the hub's public key as base64 string.
func (a *AuthConfig) GetHubPublicKey() (string, error) {
	if a.HubPrivateKey == "" {
		return "", fmt.Errorf("hub private key not set")
	}

	privBytes, err := base64.StdEncoding.DecodeString(a.HubPrivateKey)
	if err != nil {
		return "", fmt.Errorf("invalid hub private key: %w", err)
	}

	priv := ed25519.PrivateKey(privBytes)
	pub := priv.Public().(ed25519.PublicKey)
	return base64.StdEncoding.EncodeToString(pub), nil
}

// AddClient adds a new mobile client and returns its generated public key.
func (a *AuthConfig) AddClient(nickname string) (string, error) {
	// Generate a new key pair for the client
	keyPair, err := GenerateKeyPair()
	if err != nil {
		return "", fmt.Errorf("failed to generate client key: %w", err)
	}

	pubKey := base64.StdEncoding.EncodeToString(keyPair.PublicKey)

	client := ClientInfo{
		Nickname:  nickname,
		PublicKey: pubKey,
	}

	a.mu.Lock()
	a.Clients = append(a.Clients, client)
	if a.clientsByKey != nil {
		a.clientsByKey[pubKey] = &a.Clients[len(a.Clients)-1]
	}
	a.mu.Unlock()

	return pubKey, nil
}

// GetClientByPublicKey looks up a client by their public key.
func (a *AuthConfig) GetClientByPublicKey(pubKey string) *ClientInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.clientsByKey == nil {
		return nil
	}
	client, ok := a.clientsByKey[pubKey]
	if !ok {
		return nil
	}
	return client
}

// SaveAuth saves the auth config to the hub config file.
func (a *AuthConfig) SaveAuth(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		// Create new config with auth
		cfg := map[string]interface{}{
			"port":      12800,
			"workspace": ".",
			"auth": map[string]interface{}{
				"hub_private_key": a.HubPrivateKey,
				"clients":         a.Clients,
			},
			"agents": []interface{}{},
		}
		return saveConfig(configPath, cfg)
	}

	// Parse existing config and update auth section
	var rawConfig map[string]interface{}
	if err := json.Unmarshal(data, &rawConfig); err != nil {
		return err
	}

	authData := map[string]interface{}{
		"hub_private_key": a.HubPrivateKey,
		"clients":         a.Clients,
	}
	rawConfig["auth"] = authData

	return saveConfig(configPath, rawConfig)
}

// saveConfig saves a config map to the specified path.
func saveConfig(path string, cfg map[string]interface{}) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

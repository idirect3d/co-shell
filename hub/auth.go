// Author: L.Shuang
// Created: 2026-05-17
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
)

// AuthConfig holds the authentication configuration for hub-mobile communication.
type AuthConfig struct {
	// HubPrivateKey is the base64-encoded Ed25519 private key.
	HubPrivateKey string `json:"hub_private_key,omitempty"`
	// MobilePublicKey is the base64-encoded Ed25519 public key for mobile clients.
	MobilePublicKey string `json:"mobile_public_key,omitempty"`
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
				if mpk, ok := authData["mobile_public_key"].(string); ok {
					auth.MobilePublicKey = mpk
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
		auth.MobilePublicKey = "" // Will be set manually or from config
	}

	return auth, nil
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

// SaveAuth saves the auth config to the hub config file.
func (a *AuthConfig) SaveAuth(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		// Create new config with auth
		cfg := map[string]interface{}{
			"port":      12800,
			"workspace": ".",
			"auth": map[string]interface{}{
				"hub_private_key":   a.HubPrivateKey,
				"mobile_public_key": a.MobilePublicKey,
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
	}
	if a.MobilePublicKey != "" {
		authData["mobile_public_key"] = a.MobilePublicKey
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

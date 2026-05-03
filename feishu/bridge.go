// Author: L.Shuang
// Created: 2026-05-04
// Last Modified: 2026-05-04
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

package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	wsBaseURL = "wss://open.feishu.cn/ws/v1"
	// Event types
	eventPong = "pong"
)

// Bridge manages the WebSocket connection to Feishu.
type Bridge struct {
	client  *Client
	handler *Handler
	cfg     *Config

	conn      *websocket.Conn
	connMu    sync.Mutex
	connected bool
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

// NewBridge creates a new Feishu WebSocket bridge.
func NewBridge(cfg *Config, client *Client, handler *Handler) *Bridge {
	return &Bridge{
		client:  client,
		handler: handler,
		cfg:     cfg,
		stopCh:  make(chan struct{}),
	}
}

// Start establishes the WebSocket connection and begins processing events.
func (b *Bridge) Start(ctx context.Context) error {
	token, err := b.client.GetTenantAccessToken()
	if err != nil {
		return fmt.Errorf("cannot get tenant access token: %w", err)
	}

	if err := b.connect(token); err != nil {
		return fmt.Errorf("cannot connect to Feishu WebSocket: %w", err)
	}

	log.Printf("Connected to Feishu WebSocket")

	b.wg.Add(1)
	go b.eventLoop(ctx)

	return nil
}

// Stop gracefully shuts down the WebSocket connection.
func (b *Bridge) Stop() {
	close(b.stopCh)
	b.wg.Wait()
	b.disconnect()
}

// connect establishes the WebSocket connection.
// Feishu requires the token to be passed as a URL query parameter.
func (b *Bridge) connect(token string) error {
	b.connMu.Lock()
	defer b.connMu.Unlock()

	u, err := url.Parse(wsBaseURL + "/event")
	if err != nil {
		return fmt.Errorf("cannot parse WebSocket URL: %w", err)
	}
	q := u.Query()
	q.Set("verify_token", token)
	u.RawQuery = q.Encode()

	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("websocket dial failed: %w", err)
	}

	b.conn = conn
	b.connected = true
	return nil
}

// disconnect closes the WebSocket connection.
func (b *Bridge) disconnect() {
	b.connMu.Lock()
	defer b.connMu.Unlock()

	if b.conn != nil {
		b.conn.Close()
		b.conn = nil
	}
	b.connected = false
}

// eventLoop reads events from the WebSocket connection and processes them.
func (b *Bridge) eventLoop(ctx context.Context) {
	defer b.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-b.stopCh:
			return
		default:
		}

		b.connMu.Lock()
		conn := b.conn
		b.connMu.Unlock()

		if conn == nil {
			return
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				log.Printf("WebSocket connection closed")
				return
			}
			log.Printf("WebSocket read error: %v", err)
			b.reconnect(ctx)
			continue
		}

		b.handleMessage(message)
	}
}

// handleMessage processes a raw WebSocket message.
func (b *Bridge) handleMessage(data []byte) {
	// Try to parse as a generic event
	var event Event
	if err := json.Unmarshal(data, &event); err != nil {
		log.Printf("Cannot parse event: %v", err)
		return
	}

	switch event.Type {
	case "event":
		// Process the event through the handler
		if err := b.handler.HandleEvent(event); err != nil {
			log.Printf("Event handling error: %v", err)
		}

	case "pong":
		// Heartbeat response, nothing to do
		log.Printf("Received pong")

	case "error":
		log.Printf("Received error event: %s", string(data))

	default:
		log.Printf("Unknown event type: %s", event.Type)
	}
}

// reconnect attempts to reconnect to the Feishu WebSocket with exponential backoff.
func (b *Bridge) reconnect(ctx context.Context) {
	b.disconnect()

	backoff := 1 * time.Second
	maxBackoff := 60 * time.Second

	for {
		select {
		case <-ctx.Done():
			return
		case <-b.stopCh:
			return
		default:
		}

		log.Printf("Reconnecting in %v...", backoff)

		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return
		case <-b.stopCh:
			return
		}

		token, err := b.client.GetTenantAccessToken()
		if err != nil {
			log.Printf("Cannot get token for reconnection: %v", err)
			backoff = minDuration(backoff*2, maxBackoff)
			continue
		}

		if err := b.connect(token); err != nil {
			log.Printf("Reconnection failed: %v", err)
			backoff = minDuration(backoff*2, maxBackoff)
			continue
		}

		log.Printf("Reconnected to Feishu WebSocket")
		b.wg.Add(1)
		go b.eventLoop(ctx)
		return
	}
}

// IsConnected returns whether the bridge is currently connected.
func (b *Bridge) IsConnected() bool {
	b.connMu.Lock()
	defer b.connMu.Unlock()
	return b.connected
}

// minDuration returns the smaller of two durations.
func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

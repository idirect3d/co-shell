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
	"fmt"
	"log"
	"sync"
	"time"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
)

// Bridge manages the Feishu long-connection via official SDK.
type Bridge struct {
	cfg     *Config
	handler *Handler

	wsClient *larkws.Client
	mu       sync.Mutex
	started  bool
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

// NewBridge creates a new Feishu bridge using the official SDK.
func NewBridge(cfg *Config, handler *Handler) *Bridge {
	return &Bridge{
		cfg:     cfg,
		handler: handler,
		stopCh:  make(chan struct{}),
	}
}

// Start establishes the WebSocket long-connection to Feishu.
func (b *Bridge) Start(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.started {
		return fmt.Errorf("bridge already started")
	}

	// Create event handler via SDK dispatcher
	eventHandler := dispatcher.NewEventDispatcher("", "").
		OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
			b.handler.HandleSDKEvent(ctx, event)
			return nil
		})

	// Build WS client options
	opts := []larkws.ClientOption{
		larkws.WithEventHandler(eventHandler),
	}
	if b.cfg.LogLevel == "debug" {
		opts = append(opts, larkws.WithLogLevel(larkcore.LogLevelDebug))
	}

	// Create the WS client
	b.wsClient = larkws.NewClient(b.cfg.AppID, b.cfg.AppSecret, opts...)

	b.started = true

	// Start in background goroutine (Start blocks)
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		log.Printf("Starting Feishu WebSocket long-connection...")
		if err := b.wsClient.Start(ctx); err != nil {
			log.Printf("Feishu WebSocket client stopped with error: %v", err)
		}
	}()

	// Give it a moment to connect
	time.Sleep(2 * time.Second)

	log.Printf("Feishu bridge started successfully")
	return nil
}

// Stop gracefully shuts down the WebSocket connection.
func (b *Bridge) Stop() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.started {
		return
	}

	close(b.stopCh)
	b.wg.Wait()
	b.started = false
	log.Printf("Feishu bridge stopped")
}

// IsConnected returns whether the bridge is currently connected.
func (b *Bridge) IsConnected() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.started
}

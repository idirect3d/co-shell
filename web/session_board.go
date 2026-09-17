// Author: L.Shuang
// Created: 2026-09-16
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

package web

import (
	"encoding/json"
	"time"

	"github.com/idirect3d/co-shell/agent"
	"github.com/idirect3d/co-shell/log"
)

// Message board wiring (FEATURE-530).
//
// The hub owns the board: it assigns message ids, relays asynchronously and
// pushes flat board frames over the WebSocket the hub opened to this agent.
// This file implements the agent half: auto-registration on connect, roster
// caching, board_sent acknowledgements, and busy/idle dispatch of the messages
// the hub delivers.

// initBoard wires the message aggregator and announces this agent to the hub.
// It is a no-op when the board switch is off.
func (s *WebSession) initBoard(name, duty string) {
	if !s.ag.BoardEnabled() {
		return
	}
	s.boardAgg = agent.NewBoardAggregator(agent.BoardAggregateWindow, s.flushBoardMessages)
	// The hub dials in, so a hub restart or a dropped socket means a brand new
	// client connection. Re-announce on every connect, otherwise the hub keeps
	// this agent marked offline with a frozen roster revision (UC-0006).
	s.srv.SetConnectHook(func() {
		go s.registerBoard(name, duty)
	})
	go s.registerBoard(name, duty)
	// Re-announce periodically: the hub keeps the roster and can only see agents
	// that talk to it, so a lost registration must not stay lost (FEATURE-530).
	go s.boardHeartbeat(name, duty)
}

// registerBoard sends one board_register frame (non-LLM). The WS connection may
// not be registered yet when the session starts, so it retries briefly.
func (s *WebSession) registerBoard(name, duty string) {
	payload, err := json.Marshal(map[string]interface{}{
		"type": "board_register",
		"name": name,
		"duty": duty,
	})
	if err != nil {
		log.Warn("session: board register marshal: %v", err)
		return
	}
	for attempt := 0; attempt < 20; attempt++ {
		if err := s.srv.sendRaw(payload); err == nil {
			log.Info("session: board registration sent (name=%q)", name)
			return
		}
		select {
		case <-s.closed:
			return
		case <-time.After(250 * time.Millisecond):
		}
	}
	log.Warn("session: board registration failed (hub link unavailable)")
}

// boardHeartbeatInterval is how often this agent re-announces itself.
const boardHeartbeatInterval = 60 * time.Second

// boardHeartbeat re-announces this agent periodically. The hub keeps the roster
// and can only see agents that talk to it, so a registration that got lost (the
// agent's WS socket was replaced by another serve-mode client) would otherwise
// keep this agent invisible until the next reconnect (FEATURE-530).
func (s *WebSession) boardHeartbeat(name, duty string) {
	t := time.NewTicker(boardHeartbeatInterval)
	defer t.Stop()
	for {
		select {
		case <-s.closed:
			return
		case <-t.C:
			s.registerBoard(name, duty)
		}
	}
}

// handleBoardFrame routes one flat board frame pushed by the hub.
func (s *WebSession) handleBoardFrame(msg clientMessage) {
	if !s.ag.BoardEnabled() {
		return
	}
	switch msg.Type {
	case "board_registered":
		log.Info("session: board registered as %q (roster rev=%d)", msg.AgentID, msg.RosterRev)
	case "board_roster":
		s.applyBoardRoster(msg)
	case "board_sent":
		s.ag.ResolveBoardAck(&agent.BoardAck{
			ReqID:        msg.ReqID,
			OK:           msg.BoardOK,
			MessageID:    msg.MessageID,
			Conversation: msg.Conv,
			Deliveries:   decodeBoardDeliveries(msg.Deliveries),
			Timeout:      msg.TimeoutSecs,
			Error:        msg.BoardError,
		})
	case "board_message":
		s.handleBoardDelivery(msg)
	case "board_timeout":
		s.ag.AddDynamicEvent(agent.DynamicBoardTimeout,
			agent.BoardTimeoutXML(msg.MessageID, msg.AgentID, msg.Deadline))
	}
}

// applyBoardRoster refreshes the cached roster from one board_roster frame.
func (s *WebSession) applyBoardRoster(msg clientMessage) {
	var agents []agent.BoardAgentInfo
	if len(msg.BoardAgents) > 0 {
		if err := json.Unmarshal(msg.BoardAgents, &agents); err != nil {
			log.Warn("session: bad board roster: %v", err)
			return
		}
	}
	s.ag.SetBoardRoster(msg.BoardRev, agents)
}

// handleBoardDelivery dedupes and buffers one delivered message. A message id is
// never injected twice (decision 6); the batch is flushed after a short window.
func (s *WebSession) handleBoardDelivery(msg clientMessage) {
	if !s.ag.MarkBoardMessageSeen(msg.ID) {
		return
	}
	bm := agent.BoardMessage{
		ID:           msg.ID,
		Seq:          msg.Seq,
		From:         msg.From,
		To:           msg.To,
		Content:      msg.Content,
		ReplyTo:      msg.ReplyTo,
		NoReply:      msg.NoReply,
		State:        msg.BoardState,
		Timeout:      msg.Timeout,
		Conversation: msg.Conv,
		TS:           msg.BoardTS,
	}
	if s.boardAgg == nil {
		// The board was switched on after this session started: deliver at once.
		s.deliverBoardMessages([]agent.BoardMessage{bm})
		return
	}
	s.boardAgg.Add(bm)
}

// flushBoardMessages delivers one aggregated batch.
func (s *WebSession) flushBoardMessages(batch []agent.BoardMessage) {
	s.deliverBoardMessages(batch)
}

// deliverBoardMessages hands a batch to the agent: an idle agent starts a new
// turn with the batch as its input, a busy one gets the batch as a dynamic
// event it sees before its next LLM call (decision 4).
func (s *WebSession) deliverBoardMessages(batch []agent.BoardMessage) {
	if len(batch) == 0 {
		return
	}
	if s.ag.IsBusy() {
		for _, m := range batch {
			log.Info("session: board message injected while busy (id=%s from=%s)", m.ID, m.From)
			s.ag.AddDynamicEvent(agent.DynamicBoardMessage, agent.BoardMessageXML(m))
		}
		return
	}
	log.Info("session: board messages delivered (batch=%d, busy=false, ids=%s)", len(batch),
		agent.BoardMessageIDs(batch))
	text := agent.BoardMessageInputText(batch)
	select {
	case s.inputCh <- clientMessage{Type: "input", Text: text}:
	case <-s.closed:
	case <-time.After(2 * time.Second):
		log.Warn("session: board message dropped (input queue busy)")
	}
}

// decodeBoardDeliveries decodes the per-recipient outcome list of a board_sent.
func decodeBoardDeliveries(raw json.RawMessage) []agent.BoardDelivery {
	if len(raw) == 0 {
		return nil
	}
	var out []agent.BoardDelivery
	if err := json.Unmarshal(raw, &out); err != nil {
		log.Warn("session: bad board deliveries: %v", err)
		return nil
	}
	return out
}

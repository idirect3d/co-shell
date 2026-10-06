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
	"os"
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

// initBoard wires the message aggregator and keeps the hub informed about this
// agent's board state. The runtime is built even while the switch is off:
// turning it on mid-session must work without restarting the session
// (FEATURE-557). The announced name and 职责 are read from IDENTITY.md
// (FEATURE-538), never from the hub registry.
func (s *WebSession) initBoard() {
	s.boardAgg = agent.NewBoardAggregator(agent.BoardAggregateWindow, s.flushBoardMessages)
	// The hub dials in, so a hub restart or a dropped socket means a brand new
	// client connection. Announce the current state on every connect, otherwise
	// the hub keeps a stale roster entry (UC-0006, FEATURE-557).
	s.srv.SetConnectHook(func() {
		go s.announceBoardState()
	})
	// Announce once now, then whenever the board switch flips.
	go s.watchBoardSwitch()
	// Push an immediate update when IDENTITY.md changes on disk (FEATURE-538).
	go s.watchBoardIdentity()
}

// currentIdentity returns the identity to announce right now: the agent name
// and "Agent描述" currently stored in IDENTITY.md.
func (s *WebSession) currentIdentity() (string, string) {
	return s.ag.IdentityName(), s.ag.IdentityDescription()
}

// boardStateFrame builds the frame that tells the hub whether this agent is on
// the board right now: board_register (with the IDENTITY.md name and 职责) while
// the switch is on, board_unregister while it is off. The unregister frame is
// what turns a stale "online" roster entry into "not registered" once the
// switch is turned off (FEATURE-557).
func boardStateFrame(enabled bool, name, duty string) ([]byte, string) {
	payload := map[string]interface{}{}
	frameType := "board_unregister"
	if enabled {
		frameType = "board_register"
		payload["name"] = name
		payload["duty"] = duty
	}
	payload["type"] = frameType
	data, err := json.Marshal(payload)
	if err != nil {
		log.Warn("session: board state marshal (%s): %v", frameType, err)
		return nil, frameType
	}
	return data, frameType
}

// announceBoardState tells the hub whether this agent is on the board right now.
func (s *WebSession) announceBoardState() {
	name, duty := s.currentIdentity()
	data, frameType := boardStateFrame(s.ag.BoardEnabled(), name, duty)
	if data == nil {
		return
	}
	s.sendBoardFrame(data, frameType)
}

// boardSwitchInterval is how often this agent compares its board-enabled switch
// with the value it last announced. The switch has several writers (the :config
// command, the settings page, a hand-edited config.json), so the session polls
// the effective value instead of hooking every writer (FEATURE-557).
const boardSwitchInterval = 2 * time.Second

// watchBoardSwitch announces the board state once at startup and again every
// time the board-enabled switch flips.
func (s *WebSession) watchBoardSwitch() {
	last := s.ag.BoardEnabled()
	s.announceBoardState()
	t := time.NewTicker(boardSwitchInterval)
	defer t.Stop()
	for {
		select {
		case <-s.closed:
			return
		case <-t.C:
			cur := s.ag.BoardEnabled()
			if cur == last {
				continue
			}
			last = cur
			s.announceBoardState()
		}
	}
}

// sendBoardFrame sends one board frame (non-LLM). The WS connection may not be
// registered yet when the session starts, so it retries briefly.
func (s *WebSession) sendBoardFrame(payload []byte, what string) {
	for attempt := 0; attempt < 20; attempt++ {
		if err := s.srv.sendRaw(payload); err == nil {
			log.Info("session: board %s sent", what)
			return
		}
		select {
		case <-s.closed:
			return
		case <-time.After(250 * time.Millisecond):
		}
	}
	log.Warn("session: board %s failed (hub link unavailable)", what)
}

// boardIdentityInterval is how often this agent looks for external edits to its
// own IDENTITY.md (mtime/size). A change is pushed at once with a
// board_identity frame (FEATURE-538), so the hub roster never needs to poll the
// file; an edit made through co-shell's settings pushes immediately (see
// Agent.SetIdentityField).
const boardIdentityInterval = 4 * time.Second

// watchBoardIdentity polls ./IDENTITY.md and announces this agent whenever the
// file changes on disk (manual edit, another tool, a hub-side write).
func (s *WebSession) watchBoardIdentity() {
	path := s.ag.IdentityFilePath()
	if path == "" {
		return
	}
	var lastMod time.Time
	var lastSize int64
	t := time.NewTicker(boardIdentityInterval)
	defer t.Stop()
	for {
		select {
		case <-s.closed:
			return
		case <-t.C:
			if !s.ag.BoardEnabled() {
				continue
			}
			fi, err := os.Stat(path)
			if err != nil {
				continue
			}
			if fi.ModTime().Equal(lastMod) && fi.Size() == lastSize {
				continue
			}
			lastMod, lastSize = fi.ModTime(), fi.Size()
			s.registerBoardIdentity()
		}
	}
}

// registerBoardIdentity pushes a board_identity frame carrying the current
// identity (IDENTITY.md agent-name + agent-description). The hub handles it like
// a registration, so the roster's name and 职责 refresh immediately. An older
// hub ignores the frame and still picks the change up from the next heartbeat.
func (s *WebSession) registerBoardIdentity() {
	if !s.ag.BoardEnabled() {
		return
	}
	name, duty := s.currentIdentity()
	payload, err := json.Marshal(map[string]interface{}{
		"type": "board_identity",
		"name": name,
		"duty": duty,
	})
	if err != nil {
		log.Warn("session: board identity marshal: %v", err)
		return
	}
	if err := s.srv.sendRaw(payload); err != nil {
		log.Info("session: board identity not sent (hub link unavailable)")
		return
	}
	log.Info("session: board identity pushed (name=%q)", name)
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
			agent.BoardTimeoutXML(msg.MessageID, msg.AgentID, msg.Deadline, msg.Redelivered))
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
	// FEATURE-567: a message the hub re-sent must reach the agent again even
	// though its id was seen before, so its reply duty can be refreshed.
	if msg.Redelivery == 0 && !s.ag.MarkBoardMessageSeen(msg.ID) {
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
		Redelivery:   msg.Redelivery,
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
	// FEATURE-567: every message that expects a reply becomes a duty of this
	// agent until it answers with board_send(reply_to=...).
	for _, m := range batch {
		s.ag.AddBoardObligation(m)
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
	// FEATURE-542: the explicit FIFO queue never blocks, so a board message can
	// no longer be dropped because a channel happened to be busy.
	s.inputQ.push(clientMessage{Type: "input", Text: text})
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

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

package agent

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Message board protocol (FEATURE-530).

// Reply states: exactly three, chosen by the agent that answers a message.
const (
	// BoardStateHandled means "this message is mine to handle".
	BoardStateHandled = "handled"
	// BoardStateDeclined means "not mine / not relevant".
	BoardStateDeclined = "declined"
	// BoardStateNeedInfo means "possibly mine, but I need more information".
	BoardStateNeedInfo = "need_info"
)

// Board protocol bounds.
const (
	// BoardBroadcastTarget addresses every agent except the sender.
	BoardBroadcastTarget = "*"
	// BoardMinTimeoutSeconds / BoardMaxTimeoutSeconds bound timeout_seconds.
	BoardMinTimeoutSeconds = 5
	BoardMaxTimeoutSeconds = 86400
	// BoardDefaultAckTimeoutSeconds is the hub-ack wait of board_send when the
	// board-ack-timeout setting is unset.
	BoardDefaultAckTimeoutSeconds = 5
	// boardSeenCapacity is the delivered-message dedup ring size: a message id
	// is injected into the LLM context at most once (decision 6).
	boardSeenCapacity = 1000
	// BoardDutyMaxRunes caps a roster duty description in the prompt.
	BoardDutyMaxRunes = 200
)

// BoardAgentInfo mirrors one hub roster entry.
type BoardAgentInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Duty     string `json:"duty"`
	DutyFrom string `json:"duty_source"`
	Online   bool   `json:"online"`
}

// BoardMessage mirrors one board_message frame delivered by the hub.
type BoardMessage struct {
	ID           string   `json:"id"`
	Seq          int      `json:"seq"`
	From         string   `json:"from"`
	To           []string `json:"to"`
	Content      string   `json:"content"`
	ReplyTo      string   `json:"reply_to"`
	NoReply      bool     `json:"no_reply"`
	State        string   `json:"state"`
	Timeout      int      `json:"timeout"`
	Conversation string   `json:"conversation"`
	TS           int64    `json:"ts"`
}

// BoardDelivery mirrors one recipient's delivery outcome in a board_sent ack.
type BoardDelivery struct {
	To    string `json:"to"`
	OK    bool   `json:"ok"`
	Error string `json:"error"`
}

// BoardAck is the hub's immediate acknowledgement of a board_send.
type BoardAck struct {
	ReqID        string          `json:"req_id"`
	OK           bool            `json:"ok"`
	MessageID    string          `json:"message_id"`
	Conversation string          `json:"conversation"`
	Deliveries   []BoardDelivery `json:"deliveries"`
	Timeout      int             `json:"timeout_seconds"`
	Error        string          `json:"error"`
}

// boardState caches the hub roster, dedupes delivered messages and tracks the
// pending board_sent acknowledgements. All methods are thread-safe.
type boardState struct {
	mu     sync.Mutex
	rev    int
	agents []BoardAgentInfo

	seen     map[string]bool
	seenRing []string

	pending map[string]chan *BoardAck
}

// boardStateFor returns the agent's board state, creating it on first use.
func (a *Agent) boardStateFor() *boardState {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.boardSt == nil {
		a.boardSt = &boardState{
			seen:    map[string]bool{},
			pending: map[string]chan *BoardAck{},
		}
	}
	return a.boardSt
}

// SetBoardRoster replaces the cached roster snapshot with the revision the hub
// reported. Older revisions are ignored so an out-of-order frame cannot
// resurrect a stale roster.
func (a *Agent) SetBoardRoster(rev int, agents []BoardAgentInfo) {
	st := a.boardStateFor()
	st.mu.Lock()
	defer st.mu.Unlock()
	if rev < st.rev {
		return
	}
	st.rev = rev
	st.agents = agents
}

// BoardRosterSnapshot returns the cached roster revision and entries.
func (a *Agent) BoardRosterSnapshot() (int, []BoardAgentInfo) {
	st := a.boardStateFor()
	st.mu.Lock()
	defer st.mu.Unlock()
	out := make([]BoardAgentInfo, len(st.agents))
	copy(out, st.agents)
	return st.rev, out
}

// MarkBoardMessageSeen records a delivered message id and reports whether it is
// new. Once the ring is full the oldest id is evicted, so a message id is
// injected into the context at most once per session (decision 6).
func (a *Agent) MarkBoardMessageSeen(id string) bool {
	if id == "" {
		return true
	}
	st := a.boardStateFor()
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.seen[id] {
		return false
	}
	st.seen[id] = true
	st.seenRing = append(st.seenRing, id)
	if len(st.seenRing) > boardSeenCapacity {
		old := st.seenRing[0]
		st.seenRing = st.seenRing[1:]
		delete(st.seen, old)
	}
	return true
}

// AwaitBoardAck waits for the hub's board_sent acknowledgement carrying the
// given request id. It returns an error when the hub does not answer in time.
func (a *Agent) AwaitBoardAck(reqID string, timeout time.Duration) (*BoardAck, error) {
	if reqID == "" {
		return nil, fmt.Errorf("missing request id")
	}
	st := a.boardStateFor()
	ch := make(chan *BoardAck, 1)
	st.mu.Lock()
	st.pending[reqID] = ch
	st.mu.Unlock()
	defer func() {
		st.mu.Lock()
		delete(st.pending, reqID)
		st.mu.Unlock()
	}()
	select {
	case ack := <-ch:
		if ack == nil {
			return nil, fmt.Errorf("board ack lost")
		}
		return ack, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("hub did not acknowledge within %s", timeout)
	}
}

// ResolveBoardAck hands one board_sent acknowledgement to the waiting sender.
// Acknowledgements nobody waits for are dropped.
func (a *Agent) ResolveBoardAck(ack *BoardAck) {
	if ack == nil || ack.ReqID == "" {
		return
	}
	st := a.boardStateFor()
	st.mu.Lock()
	ch := st.pending[ack.ReqID]
	st.mu.Unlock()
	if ch == nil {
		return
	}
	select {
	case ch <- ack:
	default:
	}
}

// BoardAckTimeoutDuration returns how long board_send waits for the hub's
// acknowledgement (setting board-ack-timeout, default 5s).
func (a *Agent) BoardAckTimeoutDuration() time.Duration {
	secs := BoardDefaultAckTimeoutSeconds
	if a.cfg != nil && a.cfg.BoardAckTimeout > 0 {
		secs = a.cfg.BoardAckTimeout
	}
	return time.Duration(secs) * time.Second
}

// BoardRosterBlock renders the <board_agents> roster block injected into
// <environment_details> every turn. It returns "" when the board is disabled
// or the roster is still empty, in which case the whole block is omitted.
func (a *Agent) BoardRosterBlock() string {
	if !a.BoardEnabled() {
		return ""
	}
	rev, agents := a.BoardRosterSnapshot()
	if len(agents) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("<board_agents rev=\"")
	sb.WriteString(strconv.Itoa(rev))
	sb.WriteString("\">\n")
	for _, ag := range agents {
		status := "offline"
		if ag.Online {
			status = "online"
		}
		sb.WriteString("  <agent id=")
		sb.WriteString(strconv.Quote(ag.ID))
		sb.WriteString(" name=")
		sb.WriteString(strconv.Quote(ag.Name))
		sb.WriteString(" status=")
		sb.WriteString(strconv.Quote(status))
		if duty := truncateRunes(ag.Duty, BoardDutyMaxRunes); duty != "" {
			sb.WriteString(">")
			sb.WriteString(duty)
			sb.WriteString("</agent>\n")
			continue
		}
		sb.WriteString("/>\n")
	}
	sb.WriteString("</board_agents>")
	return sb.String()
}

// truncateRunes shortens s to at most max runes, appending an ellipsis.
func truncateRunes(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 {
		return s
	}
	rs := []rune(s)
	if len(rs) <= max {
		return s
	}
	return string(rs[:max]) + "…"
}

// BoardMessageText renders one delivered message as the plain text injected
// into a new turn, e.g.
//
//	[board-message m-12 from agent-a reply_to=m-9 timeout=300] body
func BoardMessageText(m BoardMessage) string {
	var sb strings.Builder
	sb.WriteString("[board-message ")
	sb.WriteString(m.ID)
	sb.WriteString(" from ")
	sb.WriteString(m.From)
	if m.Conversation != "" {
		sb.WriteString(" conv=")
		sb.WriteString(m.Conversation)
	}
	if m.ReplyTo != "" {
		sb.WriteString(" reply_to=")
		sb.WriteString(m.ReplyTo)
	}
	if m.NoReply {
		sb.WriteString(" no_reply=true")
	} else {
		sb.WriteString(" timeout=")
		sb.WriteString(strconv.Itoa(m.Timeout))
	}
	sb.WriteString("] ")
	sb.WriteString(m.Content)
	return sb.String()
}

// BoardMessageInputText renders a batch of delivered messages for a new turn.
func BoardMessageInputText(msgs []BoardMessage) string {
	parts := make([]string, 0, len(msgs))
	for _, m := range msgs {
		parts = append(parts, BoardMessageText(m))
	}
	return strings.Join(parts, "\n")
}

// BoardMessageXML renders one delivered message as the <board_message> tag used
// by the dynamic-perception block while the agent is busy.
func BoardMessageXML(m BoardMessage) string {
	var sb strings.Builder
	sb.WriteString("<board_message id=")
	sb.WriteString(strconv.Quote(m.ID))
	sb.WriteString(" from=")
	sb.WriteString(strconv.Quote(m.From))
	if m.Conversation != "" {
		sb.WriteString(" conv=")
		sb.WriteString(strconv.Quote(m.Conversation))
	}
	if m.ReplyTo != "" {
		sb.WriteString(" reply_to=")
		sb.WriteString(strconv.Quote(m.ReplyTo))
	}
	sb.WriteString(" no_reply=")
	sb.WriteString(strconv.Quote(strconv.FormatBool(m.NoReply)))
	sb.WriteString(" timeout=")
	sb.WriteString(strconv.Quote(strconv.Itoa(m.Timeout)))
	sb.WriteString(">")
	sb.WriteString(m.Content)
	sb.WriteString("</board_message>")
	return sb.String()
}

// BoardTimeoutXML renders one board_timeout event (the hub reports that a
// recipient never answered a message that expected a reply).
func BoardTimeoutXML(messageID, agentID string, deadline int64) string {
	return "<board_timeout message_id=" + strconv.Quote(messageID) +
		" agent=" + strconv.Quote(agentID) +
		" deadline=" + strconv.Quote(strconv.FormatInt(deadline, 10)) + "/>"
}

// BoardAggregator batches inbound board messages over a short debounce window
// so several messages that arrive close together while the agent is busy are
// injected as one block instead of one per message (decision 5).
type BoardAggregator struct {
	mu     sync.Mutex
	window time.Duration
	flush  func([]BoardMessage)
	buf    []BoardMessage
	timer  *time.Timer
}

// NewBoardAggregator creates an aggregator flushing after window.
func NewBoardAggregator(window time.Duration, flush func([]BoardMessage)) *BoardAggregator {
	if window <= 0 {
		window = 400 * time.Millisecond
	}
	return &BoardAggregator{window: window, flush: flush}
}

// Add buffers one message and (re)arms the flush timer.
func (g *BoardAggregator) Add(m BoardMessage) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.buf = append(g.buf, m)
	if g.timer != nil {
		g.timer.Stop()
	}
	g.timer = time.AfterFunc(g.window, g.FlushNow)
}

// FlushNow delivers the buffered batch immediately (no-op when empty).
func (g *BoardAggregator) FlushNow() {
	g.mu.Lock()
	batch := g.buf
	g.buf = nil
	if g.timer != nil {
		g.timer.Stop()
		g.timer = nil
	}
	g.mu.Unlock()
	if len(batch) == 0 || g.flush == nil {
		return
	}
	g.flush(batch)
}

// BoardAggregateWindow is the debounce window used to merge board messages that
// arrive while the agent is busy, so a burst is injected as one block
// (FEATURE-530, decision 5).
const BoardAggregateWindow = 400 * time.Millisecond

// BoardMessageIDs renders the ids of a batch for logging.
func BoardMessageIDs(msgs []BoardMessage) string {
	ids := make([]string, 0, len(msgs))
	for _, m := range msgs {
		ids = append(ids, m.ID)
	}
	return strings.Join(ids, ",")
}

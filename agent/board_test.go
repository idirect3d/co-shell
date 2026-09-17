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
	"context"
	"strings"
	"testing"
	"time"

	"github.com/idirect3d/co-shell/config"
)

// newBoardTestAgent returns an agent with the board switch on.
func newBoardTestAgent(t *testing.T) *Agent {
	t.Helper()
	cfg := &config.Config{}
	cfg.BoardEnabled = true
	a := newTestAgentWithCfg(cfg)
	if !a.BoardEnabled() {
		t.Fatal("board should be enabled for the test agent")
	}
	return a
}

// TestBoardSendToolValidation covers the argument checks of board_send: the
// required timeout window, its bounds, the three reply states and the
// recipient forms (FEATURE-530 decisions 2, 6, 10).
func TestBoardSendToolValidation(t *testing.T) {
	cases := []struct {
		name    string
		args    map[string]interface{}
		wantErr string
	}{
		{
			name:    "missing timeout",
			args:    map[string]interface{}{"to": "agent-b", "content": "hi"},
			wantErr: "timeout_seconds is required",
		},
		{
			name:    "timeout below minimum",
			args:    map[string]interface{}{"to": "agent-b", "content": "hi", "timeout_seconds": float64(3)},
			wantErr: "timeout_seconds must be between",
		},
		{
			name:    "timeout above maximum",
			args:    map[string]interface{}{"to": "agent-b", "content": "hi", "timeout_seconds": float64(90000)},
			wantErr: "timeout_seconds must be between",
		},
		{
			name:    "missing content",
			args:    map[string]interface{}{"to": "agent-b", "timeout_seconds": float64(30)},
			wantErr: "content is required",
		},
		{
			name:    "missing recipient",
			args:    map[string]interface{}{"content": "hi", "timeout_seconds": float64(30)},
			wantErr: "to is required",
		},
		{
			name: "broadcast cannot be combined",
			args: map[string]interface{}{
				"to": []interface{}{"*", "agent-b"}, "content": "hi", "timeout_seconds": float64(30),
			},
			wantErr: "cannot be combined",
		},
		{
			name: "invalid state",
			args: map[string]interface{}{
				"to": "agent-a", "content": "hi", "reply_to": "m-1", "state": "maybe", "timeout_seconds": float64(30),
			},
			wantErr: "state must be one of",
		},
		{
			name: "state without reply_to",
			args: map[string]interface{}{
				"to": "agent-a", "content": "hi", "state": "handled", "timeout_seconds": float64(30),
			},
			wantErr: "state requires reply_to",
		},
		{
			name: "no hub link",
			args: map[string]interface{}{
				"to": "agent-b", "content": "hi", "timeout_seconds": float64(30),
			},
			wantErr: "board is not connected",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := newBoardTestAgent(t)
			_, err := a.boardSendTool(context.Background(), tc.args)
			if err == nil {
				t.Fatalf("expected an error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

// TestBoardSendToolAcceptsThreeStates checks that every reply state is accepted
// and echoed on the wire together with the reply target.
func TestBoardSendToolAcceptsThreeStates(t *testing.T) {
	for _, state := range []string{BoardStateHandled, BoardStateDeclined, BoardStateNeedInfo} {
		t.Run(state, func(t *testing.T) {
			a := newBoardTestAgent(t)
			var sent map[string]interface{}
			a.SetBoardSender(func(msg map[string]interface{}) error {
				sent = msg
				req, _ := msg["req_id"].(string)
				go a.ResolveBoardAck(&BoardAck{
					ReqID: req, OK: true, MessageID: "m-9", Conversation: "agent-a+agent-b",
					Timeout: 60, Deliveries: []BoardDelivery{{To: "agent-a", OK: true}},
				})
				return nil
			})
			out, err := a.boardSendTool(context.Background(), map[string]interface{}{
				"to": "agent-a", "content": "ok", "reply_to": "m-1", "state": state, "timeout_seconds": float64(60),
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sent["state"] != state {
				t.Fatalf("state on the wire = %v, want %s", sent["state"], state)
			}
			if sent["reply_to"] != "m-1" {
				t.Fatalf("reply_to on the wire = %v, want m-1", sent["reply_to"])
			}
			if !strings.Contains(out, "message_id=m-9") || !strings.Contains(out, "conversation=agent-a+agent-b") {
				t.Fatalf("unexpected summary: %s", out)
			}
		})
	}
}

// TestBoardAckSummaryPartialDelivery renders a failed delivery and the no_reply
// case of the tool result.
func TestBoardAckSummaryPartialDelivery(t *testing.T) {
	ack := &BoardAck{
		MessageID: "m-3", Conversation: "*", Timeout: 300,
		Deliveries: []BoardDelivery{
			{To: "agent-b", OK: true},
			{To: "agent-c", OK: true},
			{To: "agent-d", Error: "agent offline"},
		},
	}
	got := boardAckSummary(ack, false)
	want := "message_id=m-3; conversation=*; delivered=2/3 (agent-d: agent offline); reply_window=300s"
	if got != want {
		t.Fatalf("summary = %q, want %q", got, want)
	}
	if got := boardAckSummary(ack, true); !strings.HasSuffix(got, "; no reply requested") {
		t.Fatalf("no_reply summary = %q", got)
	}
}

// TestBoardAckTimeoutAndResolve covers both outcomes of the board_sent wait.
func TestBoardAckTimeoutAndResolve(t *testing.T) {
	a := newBoardTestAgent(t)
	if _, err := a.AwaitBoardAck("r-nobody", 20*time.Millisecond); err == nil {
		t.Fatal("expected a timeout error when the hub never acknowledges")
	}

	done := make(chan *BoardAck, 1)
	go func() {
		ack, err := a.AwaitBoardAck("r-1", time.Second)
		if err != nil {
			done <- nil
			return
		}
		done <- ack
	}()
	time.Sleep(10 * time.Millisecond)
	a.ResolveBoardAck(&BoardAck{ReqID: "r-1", OK: true, MessageID: "m-7"})
	ack := <-done
	if ack == nil || ack.MessageID != "m-7" {
		t.Fatalf("unexpected ack: %+v", ack)
	}
}

// TestBoardSeenDedupRing makes sure a message id is only ever delivered once and
// that the ring evicts the oldest id once it is full (decision 6).
func TestBoardSeenDedupRing(t *testing.T) {
	a := newBoardTestAgent(t)
	if !a.MarkBoardMessageSeen("m-1") {
		t.Fatal("first delivery of m-1 should be new")
	}
	if a.MarkBoardMessageSeen("m-1") {
		t.Fatal("second delivery of m-1 must be deduplicated")
	}
	for i := 0; i < boardSeenCapacity; i++ {
		a.MarkBoardMessageSeen("x-" + string(rune('a'+i%26)) + "-" + string(rune('0'+i%10)) + "-" + string(rune(i%1000)))
	}
	// m-1 is long evicted, so it counts as new again.
	if !a.MarkBoardMessageSeen("m-1") {
		t.Fatal("m-1 should have been evicted from the dedup ring")
	}
}

// TestBoardRosterBlock covers the <board_agents> rendering: empty rosters omit
// the block, duties are truncated and the revision is exposed.
func TestBoardRosterBlock(t *testing.T) {
	a := newBoardTestAgent(t)
	if got := a.BoardRosterBlock(); got != "" {
		t.Fatalf("empty roster should render nothing, got %q", got)
	}

	longDuty := strings.Repeat("職", BoardDutyMaxRunes+50)
	a.SetBoardRoster(3, []BoardAgentInfo{
		{ID: "agent-a", Name: "Reviewer", Duty: longDuty, Online: true},
		{ID: "agent-b", Name: "Ops", Duty: "deploys"},
	})
	block := a.BoardRosterBlock()
	if !strings.Contains(block, `<board_agents rev="3">`) {
		t.Fatalf("missing revision tag: %s", block)
	}
	if !strings.Contains(block, `id="agent-b" name="Ops" status="offline"`) {
		t.Fatalf("missing offline agent: %s", block)
	}
	if strings.Contains(block, longDuty) || !strings.Contains(block, "…") {
		t.Fatalf("duty should have been truncated: %s", block)
	}

	// An older revision must not resurrect a stale roster.
	a.SetBoardRoster(2, []BoardAgentInfo{{ID: "agent-old"}})
	if rev, agents := a.BoardRosterSnapshot(); rev != 3 || len(agents) != 2 {
		t.Fatalf("stale roster accepted: rev=%d agents=%d", rev, len(agents))
	}
}

// TestBoardMessageRenderers covers the plain-text form (new turn) and the XML
// form (busy injection) of a delivered message.
func TestBoardMessageRenderers(t *testing.T) {
	m := BoardMessage{
		ID: "m-12", From: "agent-a", To: []string{"agent-b"}, Content: "please review",
		ReplyTo: "m-9", Conversation: "agent-a+agent-b", Timeout: 300,
	}
	text := BoardMessageText(m)
	if !strings.Contains(text, "[board-message m-12 from agent-a") ||
		!strings.Contains(text, "reply_to=m-9") || !strings.Contains(text, "timeout=300") {
		t.Fatalf("unexpected text form: %s", text)
	}
	xmlForm := BoardMessageXML(m)
	if !strings.HasPrefix(xmlForm, `<board_message id="m-12" from="agent-a"`) ||
		!strings.HasSuffix(xmlForm, "</board_message>") {
		t.Fatalf("unexpected xml form: %s", xmlForm)
	}
	tout := BoardTimeoutXML("m-12", "agent-b", 1700000000)
	if !strings.Contains(tout, `message_id="m-12"`) || !strings.Contains(tout, `agent="agent-b"`) {
		t.Fatalf("unexpected timeout form: %s", tout)
	}
	if got := BoardMessageInputText([]BoardMessage{m, m}); strings.Count(got, "[board-message") != 2 {
		t.Fatalf("batch should contain two messages: %s", got)
	}
}

// TestBoardAggregatorMergesMessages checks the busy-path debounce: messages that
// arrive within the window are flushed as one batch (decision 5).
func TestBoardAggregatorMergesMessages(t *testing.T) {
	batches := make(chan []BoardMessage, 4)
	agg := NewBoardAggregator(30*time.Millisecond, func(batch []BoardMessage) { batches <- batch })
	agg.Add(BoardMessage{ID: "m-1"})
	agg.Add(BoardMessage{ID: "m-2"})
	agg.Add(BoardMessage{ID: "m-3"})

	select {
	case batch := <-batches:
		if len(batch) != 3 {
			t.Fatalf("expected 3 merged messages, got %d", len(batch))
		}
	case <-time.After(time.Second):
		t.Fatal("aggregator did not flush within the window")
	}
	// Nothing left to flush.
	agg.FlushNow()
	select {
	case extra := <-batches:
		t.Fatalf("unexpected extra flush: %+v", extra)
	default:
	}
}

// TestBoardRecipientsParsing covers the recipient forms accepted by board_send.
func TestBoardRecipientsParsing(t *testing.T) {
	if got, err := boardRecipients("*"); err != nil || len(got) != 1 || got[0] != "*" {
		t.Fatalf("broadcast parse: %v %v", got, err)
	}
	if got, err := boardRecipients("agent-a, agent-b"); err != nil || len(got) != 2 {
		t.Fatalf("comma list parse: %v %v", got, err)
	}
	if got, err := boardRecipients([]interface{}{"agent-a", "agent-a"}); err != nil || len(got) != 1 {
		t.Fatalf("list dedup parse: %v %v", got, err)
	}
	if _, err := boardRecipients(""); err == nil {
		t.Fatal("empty recipient list must be rejected")
	}
	if _, err := boardRecipients(42); err == nil {
		t.Fatal("non-string recipients must be rejected")
	}
}

// TestBoardAckTimeoutFromConfig checks the board-ack-timeout setting wiring.
func TestBoardAckTimeoutFromConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.BoardEnabled = true
	a := newTestAgentWithCfg(cfg)
	if got := a.BoardAckTimeoutDuration(); got != BoardDefaultAckTimeoutSeconds*time.Second {
		t.Fatalf("default ack timeout = %s", got)
	}
	cfg.BoardAckTimeout = 11
	if got := a.BoardAckTimeoutDuration(); got != 11*time.Second {
		t.Fatalf("configured ack timeout = %s", got)
	}
}

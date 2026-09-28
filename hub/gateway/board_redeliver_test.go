// Author: L.Shuang
// Created: 2026-09-28
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

package gateway

import (
	"encoding/json"
	"testing"
	"time"
)

// TestBoardRedeliverThenTimeout covers the hub half of FEATURE-567: an expired
// reply window re-sends the message once (redelivery=1 with a fresh deadline)
// before the hub reports the timeout on the second expiry (UC-0004).
func TestBoardRedeliverThenTimeout(t *testing.T) {
	b, links := newTestBoard(t, "agent-a", "agent-b")
	now := time.Unix(1_700_000_000, 0)
	b.now = func() time.Time { return now }

	msg, _, err := b.Send(SendRequest{
		From: "agent-a", To: []string{"agent-b"}, Content: "please answer", TimeoutSecs: 30,
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	framesBefore := len(links["agent-b"].pushes(t))

	// First expiry: re-delivery, no timeout.
	now = now.Add(31 * time.Second)
	if events := b.SweepTimeouts(); len(events) != 0 {
		t.Fatalf("first expiry must re-deliver, not time out: %+v", events)
	}
	frames := links["agent-b"].pushes(t)
	if len(frames) != framesBefore+1 {
		t.Fatalf("re-delivery frames = %d, want %d", len(frames), framesBefore+1)
	}
	last := frames[len(frames)-1]
	if last["type"] != MsgTypeBoardMessage {
		t.Fatalf("re-delivery frame type = %v, want %s", last["type"], MsgTypeBoardMessage)
	}
	if got, ok := last["redelivery"].(float64); !ok || int(got) != 1 {
		t.Fatalf("re-delivery marker = %v, want 1", last["redelivery"])
	}
	pending := b.Pending(msg.ID)
	if len(pending) != 1 || pending[0].Redeliveries != 1 {
		t.Fatalf("pending = %+v, want one entry with Redeliveries=1", pending)
	}
	if pending[0].Deadline != now.Unix()+30 {
		t.Fatalf("deadline = %d, want %d (window reset to the original 30s)",
			pending[0].Deadline, now.Unix()+30)
	}

	// Second expiry: the re-delivery budget is used up → timeout.
	now = now.Add(31 * time.Second)
	events := b.SweepTimeouts()
	if len(events) != 1 {
		t.Fatalf("events = %+v, want exactly one timeout", events)
	}
	if events[0].Redelivered != 1 {
		t.Fatalf("timeout event redelivered = %d, want 1", events[0].Redelivered)
	}
	if len(events[0].Agents) != 1 || events[0].Agents[0] != "agent-b" {
		t.Fatalf("timed-out agents = %v, want [agent-b]", events[0].Agents)
	}
	if again := b.SweepTimeouts(); len(again) != 0 {
		t.Fatalf("second sweep = %+v, want none", again)
	}
}

// TestBoardRedeliverMaxZeroIsLegacy verifies that board_redeliver_max=0 restores
// the behaviour from before FEATURE-567: the first expiry is a timeout and no
// re-delivery frame is pushed (UC-0004 regression).
func TestBoardRedeliverMaxZeroIsLegacy(t *testing.T) {
	b, links := newTestBoard(t, "agent-a", "agent-b")
	b.SetRedeliverMax(0)
	now := time.Unix(1_700_000_000, 0)
	b.now = func() time.Time { return now }

	msg, _, err := b.Send(SendRequest{
		From: "agent-a", To: []string{"agent-b"}, Content: "please answer", TimeoutSecs: 30,
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	framesBefore := len(links["agent-b"].pushes(t))

	now = now.Add(31 * time.Second)
	events := b.SweepTimeouts()
	if len(events) != 1 || len(events[0].Agents) != 1 || events[0].Agents[0] != "agent-b" {
		t.Fatalf("events = %+v, want one timeout for agent-b", events)
	}
	if events[0].Redelivered != 0 {
		t.Fatalf("legacy timeout should not report re-deliveries: %+v", events[0])
	}
	if got := len(links["agent-b"].pushes(t)); got != framesBefore {
		t.Fatalf("legacy mode pushed %d extra frames", got-framesBefore)
	}
	if pending := b.Pending(msg.ID); len(pending) != 1 || pending[0].Status != AwaitingTimeout {
		t.Fatalf("pending = %+v, want one timed-out entry", pending)
	}
}

// TestBoardAwaitingLegacyRecord loads an awaiting record persisted before
// FEATURE-567 (no redeliveries field) and treats it as never re-sent.
func TestBoardAwaitingLegacyRecord(t *testing.T) {
	var a Awaiting
	raw := []byte(`{"message_id":"m-1","agent_id":"agent-b","deadline":1700000000,"status":"pending"}`)
	if err := json.Unmarshal(raw, &a); err != nil {
		t.Fatalf("unmarshal legacy awaiting: %v", err)
	}
	if a.Redeliveries != 0 {
		t.Fatalf("legacy record redeliveries = %d, want 0", a.Redeliveries)
	}
	// Being below the default limit, a legacy record is still re-sent once.
	if a.Redeliveries >= BoardRedeliverDefault {
		t.Fatalf("legacy record should stay eligible for the default re-delivery")
	}
}

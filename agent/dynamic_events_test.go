// Author: L.Shuang
// Created: 2026-09-04
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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/idirect3d/co-shell/config"
)

// TestDynamicQueueAddDrain verifies events are buffered and drained in FIFO
// order, and that a drain empties the queue (one-shot consumption).
func TestDynamicQueueAddDrain(t *testing.T) {
	q := newDynamicEventQueue(100)
	q.add(DynamicEvent{Kind: DynamicClipObject, Path: "input/a.png"})
	q.add(DynamicEvent{Kind: DynamicUploadFile, Path: "input/b.txt"})
	q.add(DynamicEvent{Kind: DynamicUserMessage, Text: "补充消息"})

	got := q.drain()
	if len(got) != 3 {
		t.Fatalf("drain len = %d, want 3", len(got))
	}
	if got[0].Kind != DynamicClipObject || got[0].Path != "input/a.png" {
		t.Errorf("event[0] = %+v, want clip_object input/a.png", got[0])
	}
	if got[2].Kind != DynamicUserMessage || got[2].Text != "补充消息" {
		t.Errorf("event[2] = %+v, want user_message 补充消息", got[2])
	}
	// Second drain is empty (one-shot).
	if again := q.drain(); len(again) != 0 {
		t.Errorf("second drain len = %d, want 0", len(again))
	}
}

// TestDynamicQueueCapacity verifies the oldest event is dropped when the queue
// exceeds its capacity.
func TestDynamicQueueCapacity(t *testing.T) {
	q := newDynamicEventQueue(2)
	q.add(DynamicEvent{Kind: DynamicClipObject, Path: "a.png"})
	q.add(DynamicEvent{Kind: DynamicClipObject, Path: "b.png"})
	q.add(DynamicEvent{Kind: DynamicClipObject, Path: "c.png"}) // drops a.png

	got := q.drain()
	if len(got) != 2 {
		t.Fatalf("drain len = %d, want 2", len(got))
	}
	if got[0].Path != "b.png" || got[1].Path != "c.png" {
		t.Errorf("after overflow got paths %q, %q; want b.png, c.png", got[0].Path, got[1].Path)
	}
}

// TestDynamicQueueDedup verifies duplicate path events are coalesced (timestamp
// refreshed) rather than appended twice.
func TestDynamicQueueDedup(t *testing.T) {
	q := newDynamicEventQueue(100)
	q.add(DynamicEvent{Kind: DynamicClipObject, Path: "a.png"})
	q.add(DynamicEvent{Kind: DynamicClipObject, Path: "a.png"})
	q.add(DynamicEvent{Kind: DynamicUploadFile, Path: "a.png"}) // different kind, kept

	got := q.drain()
	if len(got) != 2 {
		t.Fatalf("drain len = %d, want 2 (clip deduped, upload kept)", len(got))
	}
}

// TestConsumeDynamicEventsUserVsTool verifies user_messages are only included
// on tool messages (includeUserMessages=true), while clip/upload/open appear on
// both user and tool messages.
func TestConsumeDynamicEventsUserVsTool(t *testing.T) {
	a := &Agent{cfg: config.DefaultConfig()}
	a.AddDynamicEvent(DynamicClipObject, "input/a.png")
	a.AddDynamicEvent(DynamicUserMessage, "补充说明")

	// User message: no user_messages.
	userBlock := a.consumeDynamicEvents(false)
	if !strings.Contains(userBlock, "<clip_objects>") {
		t.Errorf("user block should include clip_objects, got:\n%s", userBlock)
	}
	if strings.Contains(userBlock, "<user_messages>") {
		t.Errorf("user block must NOT include user_messages, got:\n%s", userBlock)
	}

	// Tool message: includes user_messages.
	a.AddDynamicEvent(DynamicUserMessage, "补充说明")
	toolBlock := a.consumeDynamicEvents(true)
	if !strings.Contains(toolBlock, "<user_messages>") {
		t.Errorf("tool block should include user_messages, got:\n%s", toolBlock)
	}
	if !strings.Contains(toolBlock, "补充说明") {
		t.Errorf("tool block should carry the message text, got:\n%s", toolBlock)
	}
}

// TestConsumeDynamicEventsFileStat verifies file events carry live size/mtime.
func TestConsumeDynamicEventsFileStat(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(f, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	// chdir so the relative path resolves.
	old, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(old)

	a := &Agent{cfg: config.DefaultConfig()}
	a.AddDynamicEvent(DynamicUploadFile, "note.txt")
	block := a.consumeDynamicEvents(false)
	if !strings.Contains(block, `path="note.txt"`) {
		t.Errorf("block should carry the path, got:\n%s", block)
	}
	if !strings.Contains(block, `size="5"`) {
		t.Errorf("block should carry size 5, got:\n%s", block)
	}
	if !strings.Contains(block, "mtime=") {
		t.Errorf("block should carry mtime, got:\n%s", block)
	}
}

// TestPendingUserMessages verifies PendingUserMessages returns and clears only
// the user_message events, leaving path events buffered.
func TestPendingUserMessages(t *testing.T) {
	a := &Agent{cfg: config.DefaultConfig()}
	a.AddDynamicEvent(DynamicUserMessage, "第一条")
	a.AddDynamicEvent(DynamicClipObject, "input/a.png")
	a.AddDynamicEvent(DynamicUserMessage, "第二条")

	msgs := a.PendingUserMessages()
	if len(msgs) != 2 || msgs[0] != "第一条" || msgs[1] != "第二条" {
		t.Fatalf("PendingUserMessages = %v, want [第一条 第二条]", msgs)
	}
	// user_messages cleared, clip still buffered.
	block := a.consumeDynamicEvents(true)
	if strings.Contains(block, "<user_messages>") {
		t.Errorf("user_messages should be cleared after PendingUserMessages, got:\n%s", block)
	}
	if !strings.Contains(block, "<clip_objects>") {
		t.Errorf("clip_objects should remain buffered, got:\n%s", block)
	}
}

// TestDynamicQueueCapacityFromConfig verifies the queue capacity honors the
// configured dynamic-event-queue-size.
func TestDynamicQueueCapacityFromConfig(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DynamicEventQueueSize = 3
	a := &Agent{cfg: cfg}
	for i := 0; i < 5; i++ {
		a.AddDynamicEvent(DynamicClipObject, "f"+string(rune('a'+i))+".png")
	}
	block := a.consumeDynamicEvents(false)
	// Only the last 3 survive.
	if strings.Contains(block, "fa.png") {
		t.Errorf("oldest event fa.png should be dropped, got:\n%s", block)
	}
	if !strings.Contains(block, "fd.png") || !strings.Contains(block, "fe.png") {
		t.Errorf("newest events should survive, got:\n%s", block)
	}
}

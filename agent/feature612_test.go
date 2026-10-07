package agent

import (
	"strings"
	"testing"
)

// TestDynEventsSnapshotFlags verifies the "read once, then gone" snapshot
// (FEATURE-612): a transient path event leaves it right after its single
// injection, a retainable user_message stays as unacked until the LLM
// acknowledges it, and acknowledging it removes it from the panel as well.
func TestDynEventsSnapshotFlags(t *testing.T) {
	a := &Agent{}
	q := a.dynamicEventQueue()
	q.add(DynamicEvent{Kind: DynamicClipObject, Path: "input/clip-1.png"})
	q.add(DynamicEvent{Kind: DynamicUserMessage, Text: "hello"})

	fresh, _ := q.takeForInjection(100)
	if len(fresh) != 2 {
		t.Fatalf("fresh=%d want 2", len(fresh))
	}

	views := a.DynEventsSnapshot()
	// The transient clip event was consumed by its single injection, so only the
	// retainable user_message is left — injected but not yet acknowledged.
	if len(views) != 1 {
		t.Fatalf("views=%d want 1 (a consumed event must leave the panel)", len(views))
	}
	if views[0].Kind != string(DynamicUserMessage) {
		t.Fatalf("views[0].kind=%q want user_message", views[0].Kind)
	}
	if !views[0].Unacked {
		t.Fatalf("injected-but-unacked event must carry Unacked=true")
	}
	if views[0].Pending {
		t.Fatalf("injected event must not be Pending")
	}

	// Acknowledging it removes it from the snapshot too.
	if n := q.ack([]string{views[0].ID}); n != 1 {
		t.Fatalf("ack=%d want 1", n)
	}
	if views = a.DynEventsSnapshot(); len(views) != 0 {
		t.Fatalf("views=%d want 0 (an acknowledged event must leave the panel)", len(views))
	}
}

// TestDynEventsSnapshotPending verifies that a still-queued event is flagged
// Pending and that the snapshot is newest-first.
func TestDynEventsSnapshotPending(t *testing.T) {
	a := &Agent{}
	q := a.dynamicEventQueue()
	q.add(DynamicEvent{Kind: DynamicUploadFile, Path: "a.txt"})
	q.add(DynamicEvent{Kind: DynamicUploadFile, Path: "b.txt"})

	views := a.DynEventsSnapshot()
	if len(views) != 2 {
		t.Fatalf("views=%d want 2", len(views))
	}
	// newest first: b.txt before a.txt
	if views[0].Path != "b.txt" || views[1].Path != "a.txt" {
		t.Fatalf("order=%q,%q want b.txt,a.txt", views[0].Path, views[1].Path)
	}
	if !views[0].Pending || !views[1].Pending {
		t.Fatalf("queued events must be Pending")
	}
}

// TestDynEventsSnapshotMergesUnconsumed verifies that the snapshot merges the
// queued events with the injected-but-unacked ones, newest first (FEATURE-612).
func TestDynEventsSnapshotMergesUnconsumed(t *testing.T) {
	a := &Agent{}
	q := a.dynamicEventQueue()
	q.add(DynamicEvent{Kind: DynamicUserMessage, Text: "older"})
	if fresh, _ := q.takeForInjection(100); len(fresh) != 1 {
		t.Fatalf("fresh=%d want 1", len(fresh))
	}
	q.add(DynamicEvent{Kind: DynamicUserMessage, Text: "newer"})

	views := a.DynEventsSnapshot()
	if len(views) != 2 {
		t.Fatalf("views=%d want 2 (queued + unacked)", len(views))
	}
	if views[0].Text != "newer" || !views[0].Pending || views[0].Unacked {
		t.Fatalf("views[0]=%+v want the queued event first", views[0])
	}
	if views[1].Text != "older" || views[1].Pending || !views[1].Unacked {
		t.Fatalf("views[1]=%+v want the unacked event second", views[1])
	}
}

// TestEnvBlocksSplitAndCache verifies that EnvBlocks splits the cached
// <environment_details> into top-level elements and keeps nested elements
// inside their parent block (FEATURE-612).
func TestEnvBlocksSplitAndCache(t *testing.T) {
	a := &Agent{}
	if got := a.LastEnvDetails(); got != "" {
		t.Fatalf("LastEnvDetails=%q want empty before any envelope", got)
	}
	a.mu.Lock()
	a.lastEnvDetails = "<environment_details>\n<time>now</time>\n<tools>\na\nb\n</tools>\n<opened_resources>\n<browser>none</browser>\n</opened_resources>\n</environment_details>"
	a.mu.Unlock()

	blocks := a.EnvBlocks()
	if len(blocks) != 3 {
		t.Fatalf("blocks=%d want 3", len(blocks))
	}
	if blocks[0].Tag != "time" || blocks[1].Tag != "tools" || blocks[2].Tag != "opened_resources" {
		t.Fatalf("tags=%v", blocks)
	}
	if !strings.Contains(blocks[2].Text, "<browser>none</browser>") {
		t.Fatalf("nested <browser> lost inside <opened_resources>: %q", blocks[2].Text)
	}
	if !strings.Contains(blocks[1].Text, "</tools>") {
		t.Fatalf("tools block must keep its closing tag")
	}
}

// TestEnvBuiltHookFiresOnEnvelope verifies the FEATURE-612 hook: it must run
// exactly once per freshly assembled <environment_details> and must run with the
// agent lock RELEASED, because the web session's handler re-reads the cached
// envelope through EnvBlocks/LastEnvDetails (which take the same lock). A hook
// that ran under the lock would deadlock the first real snapshot push.
func TestEnvBuiltHookFiresOnEnvelope(t *testing.T) {
	a := &Agent{}
	calls := 0
	lockFree := false
	a.SetEnvBuiltHook(func() {
		calls++
		if a.mu.TryLock() {
			lockFree = true
			a.mu.Unlock()
		}
	})

	env := a.buildFullEnvironmentDetails(1, nil)
	if env == "" {
		t.Skip("no envelope assembled in this configuration")
	}
	if calls != 1 {
		t.Fatalf("hook fired %d times, want exactly 1", calls)
	}
	if !lockFree {
		t.Fatalf("hook ran with the agent lock held")
	}
	if got := a.LastEnvDetails(); got != env {
		t.Fatalf("cached envelope does not match the built one")
	}
	if n := len(a.EnvBlocks()); n == 0 {
		t.Fatalf("EnvBlocks returned nothing for a freshly built envelope")
	}

	// Clearing the hook must stop the callbacks.
	a.SetEnvBuiltHook(nil)
	a.buildFullEnvironmentDetails(2, nil)
	if calls != 1 {
		t.Fatalf("hook still fired after being cleared (%d calls)", calls)
	}
}

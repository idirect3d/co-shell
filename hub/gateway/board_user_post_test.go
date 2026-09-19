package gateway

import (
	"testing"
)

// TestBoardSendExplicitConv verifies a human post (from the hub Web UI) is
// pinned to an existing conversation instead of splitting off a separate
// "a+b+user" thread (FEATURE-537), and that it reaches the agents in that
// conversation.
func TestBoardSendExplicitConv(t *testing.T) {
	b, links := newTestBoard(t, "a", "b")
	if _, _, err := b.Send(SendRequest{From: "a", To: []string{"b"}, Content: "ping", TimeoutSecs: 30}); err != nil {
		t.Fatalf("seed send: %v", err)
	}

	msg, _, err := b.Send(SendRequest{
		From:        boardUserSender,
		To:          []string{"a", "b"},
		Content:     "hello agents",
		Conv:        "a+b",
		NoReply:     true,
		TimeoutSecs: boardUserReplyWindowSecs,
	})
	if err != nil {
		t.Fatalf("human send: %v", err)
	}
	if msg.Conv != "a+b" {
		t.Fatalf("conv = %q, want %q", msg.Conv, "a+b")
	}
	if msg.From != boardUserSender {
		t.Fatalf("from = %q, want %q", msg.From, boardUserSender)
	}
	if !msg.NoReply {
		t.Fatal("human post should declare NoReply so no awaiting window is tracked")
	}

	convs := b.Conversations()
	if len(convs) != 1 || convs[0].Key != "a+b" {
		t.Fatalf("conversations = %+v, want exactly [a+b]", convs)
	}
	msgs := b.Messages("a+b", 0, 0)
	if len(msgs) != 2 {
		t.Fatalf("messages = %d, want 2", len(msgs))
	}
	if last := msgs[len(msgs)-1]; last.Content != "hello agents" || last.Conv != "a+b" {
		t.Fatalf("last message = %+v", last)
	}

	// Both participants receive the human post as a flat board_message frame
	// (payload fields sit next to "type", see marshalFrame).
	for _, id := range []string{"a", "b"} {
		link := links[id]
		if link == nil {
			t.Fatalf("missing link for %q", id)
		}
		var got bool
		for _, frame := range link.pushes(t) {
			if frame["type"] != MsgTypeBoardMessage {
				continue
			}
			if frame["content"] == "hello agents" && frame["from"] == boardUserSender {
				got = true
			}
		}
		if !got {
			t.Fatalf("agent %q did not receive the human post", id)
		}
	}
}

// TestBoardRecipients maps conversation keys to the To list of a human post.
func TestBoardRecipients(t *testing.T) {
	cases := []struct {
		key  string
		want []string
	}{
		{"*", []string{"*"}},
		{"a+b", []string{"a", "b"}},
		{"a", []string{"a"}},
	}
	for _, tc := range cases {
		got := boardRecipients(tc.key)
		if len(got) != len(tc.want) {
			t.Fatalf("boardRecipients(%q) = %v, want %v", tc.key, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("boardRecipients(%q) = %v, want %v", tc.key, got, tc.want)
			}
		}
	}
}

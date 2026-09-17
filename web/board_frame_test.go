package web

import (
	"encoding/json"
	"testing"
)

// TestBoardMessageFrameCarriesID guards the hub -> co-shell board_message
// mapping: the flat frame uses "id" (not "message_id"), and the delivered id is
// what the dedup ring and the injected "[board-message <id> ...]" tag rely on.
func TestBoardMessageFrameCarriesID(t *testing.T) {
	const frame = `{"type":"board_message","id":"m-9","seq":9,"from":"agent-b",` +
		`"to":["agent-a"],"content":"done","reply_to":"m-1","no_reply":false,` +
		`"state":"handled","timeout":60,"conversation":"agent-a+agent-b","ts":1789551600}`

	var msg clientMessage
	if err := json.Unmarshal([]byte(frame), &msg); err != nil {
		t.Fatalf("unmarshal frame: %v", err)
	}
	if msg.Type != "board_message" {
		t.Fatalf("type = %q, want board_message", msg.Type)
	}
	if msg.ID != "m-9" {
		t.Fatalf("id = %q, want m-9 (board_message frames carry \"id\", not \"message_id\")", msg.ID)
	}
	if msg.From != "agent-b" || msg.ReplyTo != "m-1" {
		t.Fatalf("from/reply_to = %q/%q, want agent-b/m-1", msg.From, msg.ReplyTo)
	}
	if msg.Conv != "agent-a+agent-b" || msg.BoardState != "handled" || msg.Timeout != 60 {
		t.Fatalf("conversation/state/timeout = %q/%q/%d", msg.Conv, msg.BoardState, msg.Timeout)
	}
	if len(msg.To) != 1 || msg.To[0] != "agent-a" {
		t.Fatalf("to = %v, want [agent-a]", msg.To)
	}
	// The id the dedup ring sees must be the hub message id.
	if msg.ID == "" {
		t.Fatal("empty message id would disable dedup (decision 6)")
	}
}

// TestBoardAckFrameCarriesMessageID pins down the other direction: board_sent
// and board_timeout frames do use "message_id", so the two field names must not
// be conflated.
func TestBoardAckFrameCarriesMessageID(t *testing.T) {
	const frame = `{"type":"board_sent","req_id":"r1","ok":true,"message_id":"m-9",` +
		`"conversation":"agent-a+agent-b","timeout_seconds":60}`

	var msg clientMessage
	if err := json.Unmarshal([]byte(frame), &msg); err != nil {
		t.Fatalf("unmarshal frame: %v", err)
	}
	if msg.MessageID != "m-9" {
		t.Fatalf("message_id = %q, want m-9", msg.MessageID)
	}
	if msg.ReqID != "r1" || !msg.BoardOK {
		t.Fatalf("req_id/ok = %q/%v, want r1/true", msg.ReqID, msg.BoardOK)
	}
}

// Author: L.Shuang

package gateway

import (
	"encoding/json"
	"testing"
)

// TestMarshalFrameIsFlat guards the hub -> agent board frame shape: co-shell
// agents read flat frames ("type" next to the payload fields), so marshalFrame
// must not wrap the payload in an envelope.
func TestMarshalFrameIsFlat(t *testing.T) {
	raw, err := marshalFrame(MsgTypeBoardMessage, &Message{ID: "m-7", From: "agent-a", Content: "hi"})
	if err != nil {
		t.Fatalf("marshalFrame: %v", err)
	}
	var flat map[string]json.RawMessage
	if err := json.Unmarshal(raw, &flat); err != nil {
		t.Fatalf("frame is not a JSON object: %v", err)
	}
	if _, ok := flat["payload"]; ok {
		t.Fatalf("frame must be flat, got envelope: %s", raw)
	}
	var gotType string
	if err := json.Unmarshal(flat["type"], &gotType); err != nil {
		t.Fatalf("missing type field: %s", raw)
	}
	if gotType != MsgTypeBoardMessage {
		t.Fatalf("type = %q, want %q", gotType, MsgTypeBoardMessage)
	}
	var id string
	if err := json.Unmarshal(flat["id"], &id); err != nil {
		t.Fatalf("missing id field (co-shell reads it): %s", raw)
	}
	if id != "m-7" {
		t.Fatalf("id = %q, want m-7", id)
	}
}

// TestMarshalFrameMapPayload makes sure map payloads (registration acks,
// roster, timeout events) are flattened the same way.
func TestMarshalFrameMapPayload(t *testing.T) {
	raw, err := marshalFrame(MsgTypeBoardRoster, map[string]interface{}{"rev": 3})
	if err != nil {
		t.Fatalf("marshalFrame: %v", err)
	}
	var flat map[string]json.RawMessage
	if err := json.Unmarshal(raw, &flat); err != nil {
		t.Fatalf("frame is not a JSON object: %v", err)
	}
	if _, ok := flat["payload"]; ok {
		t.Fatalf("frame must be flat, got envelope: %s", raw)
	}
	var rev int
	if err := json.Unmarshal(flat["rev"], &rev); err != nil || rev != 3 {
		t.Fatalf("rev = %d (err %v), want 3: %s", rev, err, raw)
	}
}

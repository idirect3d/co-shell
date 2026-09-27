package web

import (
	"os"
	"strings"
	"testing"
)

// TestFEATURE565SessionSwitchRefusalIsVisible guards the UI regression found
// while exercising FEATURE-565 end to end: the backend reports a refused switch
// as serverMessage{Kind: "session_switch_result", OK: false, Message: <why>},
// but "ok" carries `json:"ok,omitempty"` (web/server.go), so the field is absent
// from the frame entirely. The frontend used to test `msg.ok === false`, which is
// never true for an omitted field, so a refused switch (running sub-agents in the
// session) changed nothing on screen and the user got no explanation.
//
// The refusal must therefore be detected as "not explicitly accepted".
func TestFEATURE565SessionSwitchRefusalIsVisible(t *testing.T) {
	js := staticAsset(t, "/static/app.js")

	idx := strings.Index(js, `msg.kind === "session_switch_result"`)
	if idx < 0 {
		t.Fatal("app.js does not handle the session_switch_result frame")
	}
	seg := js[idx : idx+400]
	if strings.Contains(seg, "msg.ok === false") {
		t.Fatalf("refusal check is still `msg.ok === false`, which never matches the omitted "+
			"ok field and leaves the user with no feedback: %q", seg)
	}
	if !strings.Contains(seg, "msg.ok !== true") {
		t.Fatalf("session_switch_result does not treat a missing ok as a refusal: %q", seg)
	}
	if !strings.Contains(seg, "notifyUser(") {
		t.Fatalf("refused switch is not surfaced to the user: %q", seg)
	}
}

// TestFEATURE565ServerOmitsFalseOK documents the serialization detail the check
// above depends on, so a future change to the tag is caught here as well.
func TestFEATURE565ServerOmitsFalseOK(t *testing.T) {
	data, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatalf("read server.go: %v", err)
	}
	srv := string(data)
	if !strings.Contains(srv, "OK       bool            `json:\"ok,omitempty\"`") {
		t.Fatal("serverMessage.OK no longer omits false; revisit the frontend refusal check in app.js")
	}
}

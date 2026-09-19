package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// postBoardMessage drives handleBoardPostMessage with the given conversation
// key and JSON body, returning the recorded response.
func postBoardMessage(t *testing.T, w *WebUI, key, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/board/conversations/"+key+"/messages", strings.NewReader(body))
	req.SetPathValue("key", key)
	rw := httptest.NewRecorder()
	w.handleBoardPostMessage(rw, req)
	return rw
}

// TestBoardPostMessageHandler covers the human-post endpoint's happy path and
// its error statuses (FEATURE-537).
func TestBoardPostMessageHandler(t *testing.T) {
	b, _ := newTestBoard(t, "a", "b")
	if _, _, err := b.Send(SendRequest{From: "a", To: []string{"b"}, Content: "ping", TimeoutSecs: 30}); err != nil {
		t.Fatalf("seed send: %v", err)
	}
	w := &WebUI{board: b}

	// 1) valid post: 200 with the stored message.
	rw := postBoardMessage(t, w, "a+b", `{"content":"hi from the user"}`)
	if rw.Code != http.StatusOK {
		t.Fatalf("valid post: code = %d, body = %s", rw.Code, rw.Body.String())
	}
	var ok boardPostView
	if err := json.Unmarshal(rw.Body.Bytes(), &ok); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if ok.Message == nil {
		t.Fatal("valid post: message is nil")
	}
	if ok.Message.From != boardUserSender || ok.Message.Content != "hi from the user" || ok.Message.Conv != "a+b" {
		t.Fatalf("valid post: message = %+v", ok.Message)
	}

	// 2) blank content: 400.
	if rw := postBoardMessage(t, w, "a+b", `{"content":"   "}`); rw.Code != http.StatusBadRequest {
		t.Fatalf("blank content: code = %d, want 400", rw.Code)
	}
	// 3) oversized content: 400.
	big := `{"content":"` + strings.Repeat("x", MaxContentBytes+1) + `"}`
	if rw := postBoardMessage(t, w, "a+b", big); rw.Code != http.StatusBadRequest {
		t.Fatalf("oversized content: code = %d, want 400", rw.Code)
	}
	// 4) malformed JSON: 400.
	if rw := postBoardMessage(t, w, "a+b", `{"content":`); rw.Code != http.StatusBadRequest {
		t.Fatalf("malformed JSON: code = %d, want 400", rw.Code)
	}
	// 5) unknown conversation: 404.
	if rw := postBoardMessage(t, w, "nope", `{"content":"hi"}`); rw.Code != http.StatusNotFound {
		t.Fatalf("unknown conversation: code = %d, want 404", rw.Code)
	}
	// 6) board not attached: 503.
	if rw := postBoardMessage(t, &WebUI{}, "a+b", `{"content":"hi"}`); rw.Code != http.StatusServiceUnavailable {
		t.Fatalf("missing board: code = %d, want 503", rw.Code)
	}

	// The rejected posts never reached the board.
	msgs := b.Messages("a+b", 0, 0)
	if len(msgs) != 2 {
		t.Fatalf("messages = %d, want 2 (seed + accepted post)", len(msgs))
	}
}

package gateway

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// boardConversationsView is the JSON shape of GET /api/board/conversations.
type boardConversationsView struct {
	Conversations []ConversationSummary `json:"conversations"`
	Roster        []AgentInfo           `json:"roster"`
	RosterRev     int                   `json:"roster_rev"`
}

// boardMessagesView is the JSON shape of the per-conversation message
// endpoint. Pending lists the replies still awaited for the returned messages
// so the UI can show each waiting window.
type boardMessagesView struct {
	Messages []*Message `json:"messages"`
	Pending  []Awaiting `json:"pending"`
}

// handleBoardConversations returns the board conversations (the broadcast
// conversation has key "*") plus the current roster.
func (w *WebUI) handleBoardConversations(rw http.ResponseWriter, _ *http.Request) {
	if w.board == nil {
		writeJSON(rw, http.StatusOK, boardConversationsView{
			Conversations: []ConversationSummary{},
			Roster:        []AgentInfo{},
		})
		return
	}
	convs := w.board.Conversations()
	roster := w.board.Roster()
	if convs == nil {
		convs = []ConversationSummary{}
	}
	if roster == nil {
		roster = []AgentInfo{}
	}
	writeJSON(rw, http.StatusOK, boardConversationsView{
		Conversations: convs,
		Roster:        roster,
		RosterRev:     w.board.RosterRev(),
	})
}

// handleBoardMessages returns one conversation's messages. "before" is a seq
// cursor (0 = latest) and "limit" caps the page size.
func (w *WebUI) handleBoardMessages(rw http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	before, _ := strconv.Atoi(r.URL.Query().Get("before"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	view := boardMessagesView{Messages: []*Message{}, Pending: []Awaiting{}}
	if w.board == nil {
		writeJSON(rw, http.StatusOK, view)
		return
	}
	if msgs := w.board.Messages(key, before, limit); msgs != nil {
		view.Messages = msgs
	}
	for _, m := range view.Messages {
		for _, a := range w.board.Pending(m.ID) {
			if a.Status == AwaitingPending {
				view.Pending = append(view.Pending, a)
			}
		}
	}
	writeJSON(rw, http.StatusOK, view)
}

// boardUserSender is the sender id the hub Web UI posts as (FEATURE-537). Agent
// ids come from the agent registry, so this id never collides with one; the UI
// renders it as the human user ("我").
const boardUserSender = "user"

// boardUserReplyWindowSecs is the reply window carried by human posts. Human
// posts declare NoReply, so no awaiting obligation is tracked; the value only
// has to satisfy Board.Send's validation range.
const boardUserReplyWindowSecs = 300

// boardPostRequest is the JSON body of POST /api/board/conversations/{key}/messages.
type boardPostRequest struct {
	Content string `json:"content"`
}

// boardPostView is the JSON shape of a successful human post.
type boardPostView struct {
	Message *Message `json:"message"`
}

// boardErrView is the JSON shape of a board API error response. It carries a
// short human-readable reason only, never internal identifiers or stack detail.
type boardErrView struct {
	Error string `json:"error"`
}

// handleBoardPostMessage posts a message into an existing conversation as the
// human user (FEATURE-537), so the Chat pane is no longer read-only. The
// conversation is pinned by key, so posting never splits off a new thread.
func (w *WebUI) handleBoardPostMessage(rw http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if w.board == nil {
		writeJSON(rw, http.StatusServiceUnavailable, boardErrView{Error: "message board is unavailable"})
		return
	}
	if len(w.board.Messages(key, 0, 1)) == 0 {
		writeJSON(rw, http.StatusNotFound, boardErrView{Error: "conversation not found"})
		return
	}
	var req boardPostRequest
	if err := json.NewDecoder(http.MaxBytesReader(rw, r.Body, MaxContentBytes*2+1024)).Decode(&req); err != nil {
		writeJSON(rw, http.StatusBadRequest, boardErrView{Error: "invalid JSON body"})
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		writeJSON(rw, http.StatusBadRequest, boardErrView{Error: "content is required"})
		return
	}
	if len(req.Content) > MaxContentBytes {
		writeJSON(rw, http.StatusBadRequest, boardErrView{Error: "content too large"})
		return
	}
	msg, _, err := w.board.Send(SendRequest{
		From:        boardUserSender,
		To:          boardRecipients(key),
		Content:     req.Content,
		Conv:        key,
		NoReply:     true,
		TimeoutSecs: boardUserReplyWindowSecs,
	})
	if err != nil {
		writeJSON(rw, http.StatusBadRequest, boardErrView{Error: err.Error()})
		return
	}
	writeJSON(rw, http.StatusOK, boardPostView{Message: msg})
}

// boardRecipients maps a conversation key to the To list of a human post: the
// broadcast conversation stays "*" (the hub expands it to every registered
// agent), while a regular conversation addresses its agent participants.
func boardRecipients(convKey string) []string {
	if convKey == BoardBroadcastTarget {
		return []string{BoardBroadcastTarget}
	}
	parts := strings.Split(convKey, "+")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || p == boardUserSender || p == BoardBroadcastTarget {
			continue
		}
		out = append(out, p)
	}
	return out
}

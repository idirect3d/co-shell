package gateway

import (
	"net/http"
	"strconv"
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

// Package gateway - Agent bulletin board (FEATURE-530).
//
// The board lets co-shell agents connected to the hub collaborate over a
// message bus: each agent registers itself (name + duty description) as soon as
// its WebSocket connection is up, the hub keeps a roster and broadcasts it to
// every agent, and agents exchange messages through a single board_send tool.
//
// A message may be a plain post or a reply. Replies carry a state
// (handled / declined / need_info) so the sender learns whether the recipient
// takes the request; every message that expects a reply is tracked until it is
// answered or its sender-chosen timeout elapses (which produces a
// board_timeout event for the sender).
//
// The hub only arbitrates: it assigns message ids, relays messages
// asynchronously, acknowledges the sender immediately, and never interprets
// message content.
package gateway

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"
)

// Board message types carried in the Envelope.Type field (in addition to the
// existing control types switch_agent / list_agents).
const (
	// MsgTypeBoardRegister registers an agent (name + duty) with the hub.
	MsgTypeBoardRegister = "board_register"
	// MsgTypeBoardIdentity re-announces an agent whose IDENTITY.md changed
	// (FEATURE-538). It carries the same name/duty fields as board_register and
	// is handled identically, so a rename or description edit reaches the hub
	// without waiting for any periodic re-announcement.
	MsgTypeBoardIdentity = "board_identity"
	// MsgTypeBoardUnregister tells the hub the agent turned its board switch off
	// (FEATURE-557). The roster entry is kept, only its state flips.
	MsgTypeBoardUnregister = "board_unregister"
	// MsgTypeBoardSend posts a message or replies to one.
	MsgTypeBoardSend = "board_send"
	// MsgTypeBoardRegistered acknowledges a registration.
	MsgTypeBoardRegistered = "board_registered"
	// MsgTypeBoardRoster broadcasts the full agent roster.
	MsgTypeBoardRoster = "board_roster"
	// MsgTypeBoardSent acknowledges a send immediately (async relay).
	MsgTypeBoardSent = "board_sent"
	// MsgTypeBoardMessage delivers a message to a recipient.
	MsgTypeBoardMessage = "board_message"
	// MsgTypeBoardTimeout reports recipients that never answered in time.
	MsgTypeBoardTimeout = "board_timeout"
)

// Reply states (decision: exactly three).
const (
	// BoardStateHandled means "this message is mine to handle".
	BoardStateHandled = "handled"
	// BoardStateDeclined means "not mine / not relevant".
	BoardStateDeclined = "declined"
	// BoardStateNeedInfo means "possibly mine, but I need more information".
	BoardStateNeedInfo = "need_info"
)

// Board limits and defaults.
const (
	// BoardBroadcastTarget addresses every registered agent except the sender.
	BoardBroadcastTarget = "*"
	// MaxContentBytes caps one message body.
	MaxContentBytes = 8 << 10
	// BoardMinTimeoutSeconds / BoardMaxTimeoutSeconds bound timeout_seconds.
	BoardMinTimeoutSeconds = 5
	BoardMaxTimeoutSeconds = 86400
	// maxDutyRunes caps the roster duty text per agent.
	maxDutyRunes = 200
)

// DutySource values record where the effective duty text came from.
const (
	DutySourceHub   = "hub"
	DutySourceAgent = "agent"
)

// Message is one board message (a post, or a reply when ReplyTo is set).
type Message struct {
	ID      string   `json:"id"`           // "m-<seq>", assigned by the hub
	Seq     int      `json:"seq"`          // numeric order, used as UI cursor
	ReplyTo string   `json:"reply_to"`     // "" = new topic
	From    string   `json:"from"`         // agent id (never client-supplied)
	To      []string `json:"to"`           // ["*"] = broadcast
	Content string   `json:"content"`      // message body
	NoReply bool     `json:"no_reply"`     // sender declares no reply is needed
	State   string   `json:"state"`        // reply state ("" for a new post)
	Timeout int      `json:"timeout"`      // reply window in seconds
	Conv    string   `json:"conversation"` // "*" or "a+b"
	TS      int64    `json:"ts"`           // unix seconds
}

// AgentInfo is one entry of the roster.
//
// Registered and Online together describe the three states the hub Web UI shows
// (FEATURE-557): Registered=false is "not registered" (no icon, but the entry is
// kept so the agent stays reachable), Registered && Online is "online" (bright
// icon) and Registered && !Online is "offline" (grey icon).
type AgentInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Duty       string `json:"duty"`
	DutyFrom   string `json:"duty_source"` // "hub" | "agent"
	Registered bool   `json:"registered"`  // announced while the board switch is on
	Online     bool   `json:"online"`      // only meaningful while registered
	LastSeen   int64  `json:"last_seen"`
}

// Board states reported to the hub Web UI (FEATURE-557). Only a registered
// agent gets an icon: bright while online, grey while its link is down and none
// at all while it is not registered.
const (
	// BoardStateUnregistered means the board switch is off (or never registered).
	BoardStateUnregistered = "unregistered"
	// BoardStateOnline means registered with its link up.
	BoardStateOnline = "online"
	// BoardStateOffline means registered earlier but its link is down.
	BoardStateOffline = "offline"
)

// Delivery reports one recipient's delivery outcome.
type Delivery struct {
	To    string `json:"to"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// Awaiting tracks one pending "reply expected" obligation.
type Awaiting struct {
	MessageID string `json:"message_id"`
	AgentID   string `json:"agent_id"`
	Deadline  int64  `json:"deadline"`
	Status    string `json:"status"` // pending | answered | timeout
	// Redeliveries counts how many times the hub re-sent the message after
	// its reply window elapsed (FEATURE-567). Records persisted before this
	// feature load as 0, i.e. as a message that is never re-sent.
	Redeliveries int `json:"redeliveries,omitempty"`
}

// BoardRedeliverDefault is how many times the hub re-sends a message whose reply
// window elapsed before it reports a timeout (FEATURE-567); 0 = never re-send.
const BoardRedeliverDefault = 1

// boardRedeliveryMaxWindow caps the reply window granted to a re-delivered
// message (FEATURE-567).
const boardRedeliveryMaxWindow = 300

// boardRedelivery is one scheduled re-send of a message whose reply window
// elapsed.
type boardRedelivery struct {
	agentID    string
	msg        *Message
	redelivery int
}

// redeliveryFrame is the board_message frame of a re-delivered message: the
// original message fields plus the redelivery counter.
type redeliveryFrame struct {
	*Message
	Redelivery int `json:"redelivery"`
}

// boardRedeliveryWindow returns the reply window granted to a re-delivered
// message: the original window, capped at boardRedeliveryMaxWindow seconds.
func boardRedeliveryWindow(timeout int) int64 {
	if timeout <= 0 || timeout > boardRedeliveryMaxWindow {
		return boardRedeliveryMaxWindow
	}
	return int64(timeout)
}

// Awaiting status values.
const (
	AwaitingPending  = "pending"
	AwaitingAnswered = "answered"
	AwaitingTimeout  = "timeout"
)

// ConversationSummary describes one conversation for the hub Web UI.
type ConversationSummary struct {
	Key          string   `json:"key"`
	Participants []string `json:"participants"`
	Count        int      `json:"count"`
	UpdatedAt    int64    `json:"updated_at"`
	LastContent  string   `json:"last_content,omitempty"`
	LastFrom     string   `json:"last_from,omitempty"`
}

// SendRequest is one validated board_send call.
type SendRequest struct {
	From        string
	To          []string
	Content     string
	ReplyTo     string
	NoReply     bool
	State       string
	TimeoutSecs int
	// Conv (FEATURE-537) pins a new message to an existing conversation key
	// instead of deriving the key from From+To. The hub Web UI uses it to post
	// as a non-agent sender ("user") into an existing conversation, so a human
	// post never splits off a separate conversation.
	Conv string
}

// DutyResolver returns the hub-side duty fallback for an agent. FEATURE-538: the
// duty shown on the board is the agent's "Agent描述" (the agent-description
// section of {workspace}/IDENTITY.md). Agents report it themselves
// (board_register / board_identity); this resolver only covers the case where a
// managed agent has not reported anything yet (the hub then reads its file).
type DutyResolver func(agentID string) (string, bool)

// conversation is the in-memory index of one conversation.
type conversation struct {
	Key          string
	Participants []string
	MessageIDs   []string
	UpdatedAt    int64
}

// Board is the hub-side message board. It is safe for concurrent use. It does
// not own the agent connections; the Proxy injects them via SetAgentLink so the
// board can push messages down to agents.
type Board struct {
	mu        sync.RWMutex
	agents    map[string]*AgentInfo
	messages  map[string]*Message
	convs     map[string]*conversation
	awaiting  map[string]*Awaiting // key: messageID + "|" + agentID
	seq       int
	rosterRev int
	// redeliverMax is how many times an expired reply obligation is re-sent
	// before it becomes a timeout (FEATURE-567).
	redeliverMax int

	store    *BoardStore
	link     func(id string) agentLink
	dutyFn   DutyResolver
	onChange func()

	// now is swappable for tests.
	now func() time.Time
}

// NewBoard creates a board, restoring persisted state when store is non-nil.
func NewBoard(store *BoardStore) (*Board, error) {
	b := &Board{
		agents:   make(map[string]*AgentInfo),
		messages: make(map[string]*Message),
		convs:    make(map[string]*conversation),
		awaiting:     make(map[string]*Awaiting),
		redeliverMax: BoardRedeliverDefault,
		store:        store,
		now:          time.Now,
	}
	if store != nil {
		state, err := store.Load()
		if err != nil {
			return nil, err
		}
		b.seq = state.seq
		b.rosterRev = state.rosterRev
		for i := range state.agents {
			a := state.agents[i]
			// A restarted hub has no live connections yet, so every entry starts
			// offline. Registered survives the restart; an entry persisted before
			// FEATURE-557 has no such field and loads as "not registered", the
			// conservative reading — the agent re-announces itself on reconnect.
			a.Online = false
			b.agents[a.ID] = &a
		}
		for i := range state.messages {
			m := state.messages[i]
			b.messages[m.ID] = &m
		}
		for i := range state.convs {
			c := state.convs[i]
			b.convs[c.Key] = &c
		}
		for k, a := range state.awaiting {
			b.awaiting[k] = a
		}
	}
	return b, nil
}

// storeErr logs a persistence failure; the in-memory state stays
// authoritative so a failed write never breaks the collaboration flow.
func (b *Board) storeErr(op string, err error) {
	if err != nil {
		log.Printf("board: persist %s failed: %v", op, err)
	}
}

// Close releases the underlying store.
func (b *Board) Close() error {
	if b.store == nil {
		return nil
	}
	return b.store.Close()
}

// SetAgentLink installs the connection resolver used to push messages down to
// agents. Called by the Proxy after construction.
func (b *Board) SetAgentLink(fn func(id string) agentLink) {
	b.mu.Lock()
	b.link = fn
	b.mu.Unlock()
}

// SetDutyResolver installs the hub-side duty override (manual entry wins).
func (b *Board) SetDutyResolver(fn DutyResolver) {
	b.mu.Lock()
	b.dutyFn = fn
	b.mu.Unlock()
}

// SetOnChange installs a callback fired after any state change (used by the
// Web UI to push fresh board data to browsers).
func (b *Board) SetOnChange(fn func()) {
	b.mu.Lock()
	b.onChange = fn
	b.mu.Unlock()
}

// changed invokes the change hook (never under the board lock).
func (b *Board) changed() {
	b.mu.RLock()
	fn := b.onChange
	b.mu.RUnlock()
	if fn != nil {
		fn()
	}
}

// nowUnix returns the current time in unix seconds.
func (b *Board) nowUnix() int64 { return b.now().Unix() }

// truncateRunes shortens s to at most n runes (UTF-8 safe, ellipsis free).
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// conversationKey derives the conversation key of a new message: "*" for a
// broadcast, otherwise the sorted participant ids joined by "+".
func conversationKey(from string, to []string) string {
	if len(to) == 1 && to[0] == BoardBroadcastTarget {
		return BoardBroadcastTarget
	}
	ids := make([]string, 0, len(to)+1)
	seen := map[string]bool{}
	for _, id := range append([]string{from}, to...) {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return strings.Join(ids, "+")
}

// Register records (or refreshes) an agent registration and returns the
// effective roster entry plus the new revision. The duty the agent reports wins
// (FEATURE-538: that is its "Agent描述"); dutyFn is only consulted as a fallback
// when nothing was reported — the hub then reads the managed agent's IDENTITY.md.
func (b *Board) Register(agentID, name, duty string) (AgentInfo, int) {
	effectiveDuty := truncateRunes(strings.TrimSpace(duty), maxDutyRunes)
	source := DutySourceAgent

	b.mu.Lock()
	if effectiveDuty == "" && b.dutyFn != nil {
		if v, ok := b.dutyFn(agentID); ok && strings.TrimSpace(v) != "" {
			effectiveDuty = truncateRunes(strings.TrimSpace(v), maxDutyRunes)
			source = DutySourceHub
		}
	}
	if strings.TrimSpace(name) == "" {
		name = agentID
	}
	info := &AgentInfo{
		ID:         agentID,
		Name:       name,
		Duty:       effectiveDuty,
		DutyFrom:   source,
		Registered: true,
		Online:     true,
		LastSeen:   b.nowUnix(),
	}
	// Only a real roster change bumps the revision: an agent re-announcing
	// itself (connect hook or switch change) must not churn the revision its
	// peers have cached, otherwise the injected roster text changes for nothing.
	// A not-registered entry counts as a change, so turning the board switch
	// back on is broadcast to the peers.
	prev, known := b.agents[agentID]
	if !known || prev.Name != info.Name || prev.Duty != info.Duty ||
		prev.DutyFrom != info.DutyFrom || !prev.Online || !prev.Registered {
		b.rosterRev++
	}
	b.agents[agentID] = info
	rev := b.rosterRev
	if b.store != nil {
		b.storeErr("agent", b.store.SaveAgent(*info))
		b.storeErr("meta", b.store.SaveMeta(b.seq, b.rosterRev))
	}
	out := *info
	b.mu.Unlock()

	b.changed()
	return out, rev
}

// Agent returns one roster entry.
func (b *Board) Agent(id string) (AgentInfo, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	a, ok := b.agents[id]
	if !ok {
		return AgentInfo{}, false
	}
	return *a, true
}

// Roster returns the current roster sorted by agent id.
func (b *Board) Roster() []AgentInfo {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.rosterLocked()
}

// rosterLocked returns the roster; the caller must hold at least a read lock.
func (b *Board) rosterLocked() []AgentInfo {
	out := make([]AgentInfo, 0, len(b.agents))
	for _, a := range b.agents {
		out = append(out, *a)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// RosterRev returns the current roster revision.
func (b *Board) RosterRev() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.rosterRev
}

// SetOffline marks an agent offline (on disconnect) and returns the new
// revision plus whether anything changed. An unregistered agent has no online
// state to lose and is ignored, which is what keeps "not registered" from
// degrading into "offline" when its link drops (FEATURE-557).
func (b *Board) SetOffline(agentID string) (int, bool) {
	b.mu.Lock()
	a, ok := b.agents[agentID]
	if !ok || !a.Registered || !a.Online {
		rev := b.rosterRev
		b.mu.Unlock()
		return rev, false
	}
	a.Online = false
	a.LastSeen = b.nowUnix()
	b.rosterRev++
	rev := b.rosterRev
	if b.store != nil {
		b.storeErr("agent", b.store.SaveAgent(*a))
		b.storeErr("meta", b.store.SaveMeta(b.seq, b.rosterRev))
	}
	b.mu.Unlock()
	b.changed()
	return rev, true
}

// SetUnregistered marks an agent as "not registered" because its board switch
// was turned off. The entry itself is kept — the agent must stay reachable and
// its conversations and messages stay readable — only the state flips. An
// unknown agent, or one that is already unregistered, is a no-op.
func (b *Board) SetUnregistered(agentID string) (int, bool) {
	b.mu.Lock()
	a, ok := b.agents[agentID]
	if !ok || !a.Registered {
		rev := b.rosterRev
		b.mu.Unlock()
		return rev, false
	}
	a.Registered = false
	a.Online = false
	a.LastSeen = b.nowUnix()
	b.rosterRev++
	rev := b.rosterRev
	if b.store != nil {
		b.storeErr("agent", b.store.SaveAgent(*a))
		b.storeErr("meta", b.store.SaveMeta(b.seq, b.rosterRev))
	}
	b.mu.Unlock()
	b.changed()
	return rev, true
}

// MarkOnline flips a registered agent online because its link is up again and
// returns the new revision plus whether the roster changed. Unknown agents are
// ignored: their name and duty arrive with the registration frame.
func (b *Board) MarkOnline(agentID string) (int, bool) {
	b.mu.Lock()
	a, ok := b.agents[agentID]
	// Only a registered entry can come back online: an entry that is merely
	// known (board switch off) must not light up because its link returned.
	if !ok || !a.Registered || a.Online {
		rev := b.rosterRev
		b.mu.Unlock()
		return rev, false
	}
	a.Online = true
	a.LastSeen = b.nowUnix()
	b.rosterRev++
	rev := b.rosterRev
	if b.store != nil {
		b.storeErr("agent", b.store.SaveAgent(*a))
		b.storeErr("meta", b.store.SaveMeta(b.seq, b.rosterRev))
	}
	b.mu.Unlock()
	b.changed()
	return rev, true
}

// touchLocked flips an agent online when it sends anything. It reports whether
// the roster changed so the caller bumps the revision and the online flip is
// broadcast to every agent instead of staying hub-local (FEATURE-530).
func (b *Board) touchLocked(agentID string) bool {
	a, ok := b.agents[agentID]
	// An unregistered entry stays untouched: any stray frame must not turn it
	// back into an online roster member.
	if !ok || !a.Registered {
		return false
	}
	flipped := !a.Online
	a.Online = true
	a.LastSeen = b.nowUnix()
	return flipped
}

// Send validates, records and relays one message. It acknowledges the caller
// immediately (the relay to recipients is asynchronous and its per-recipient
// outcome is returned in deliveries).
func (b *Board) Send(req SendRequest) (*Message, []Delivery, error) {
	content := strings.TrimSpace(req.Content)
	if req.From == "" {
		return nil, nil, fmt.Errorf("sender is not identified")
	}
	if content == "" {
		return nil, nil, fmt.Errorf("content is required")
	}
	if len(req.Content) > MaxContentBytes {
		return nil, nil, fmt.Errorf("content too large: %d bytes (max %d)", len(req.Content), MaxContentBytes)
	}
	if req.TimeoutSecs < BoardMinTimeoutSeconds || req.TimeoutSecs > BoardMaxTimeoutSeconds {
		return nil, nil, fmt.Errorf("timeout_seconds out of range (%d..%d)", BoardMinTimeoutSeconds, BoardMaxTimeoutSeconds)
	}
	isReply := req.ReplyTo != "" && req.ReplyTo != "0"
	if isReply {
		switch req.State {
		case BoardStateHandled, BoardStateDeclined, BoardStateNeedInfo:
		default:
			return nil, nil, fmt.Errorf("state must be one of %s/%s/%s when replying",
				BoardStateHandled, BoardStateDeclined, BoardStateNeedInfo)
		}
	} else if req.State != "" {
		return nil, nil, fmt.Errorf("state is only valid on a reply")
	}

	recipients := b.resolveRecipients(req.From, req.To)

	b.mu.Lock()
	if b.touchLocked(req.From) {
		b.rosterRev++
	}

	conv := ""
	if isReply {
		orig, ok := b.messages[req.ReplyTo]
		if !ok {
			b.mu.Unlock()
			return nil, nil, fmt.Errorf("reply_to %q not found", req.ReplyTo)
		}
		// A reply stays in the conversation of the message it answers, so a
		// negotiation never splits into a new thread.
		conv = orig.Conv
		if key := awaitingKey(req.ReplyTo, req.From); b.awaiting[key] != nil {
			b.awaiting[key].Status = AwaitingAnswered
			if b.store != nil {
				b.storeErr("awaiting", b.store.SaveAwaiting(b.awaiting[key]))
			}
		}
	} else {
		// The key is derived from the requested recipients so a broadcast stays
		// in the single announcement conversation instead of expanding into a
		// participant list. An explicit Conv (FEATURE-537) instead pins the
		// message to an existing conversation (human posts from the hub UI).
		if req.Conv != "" {
			conv = req.Conv
		} else {
			conv = conversationKey(req.From, req.To)
		}
	}
	b.seq++
	msg := &Message{
		ID:      fmt.Sprintf("m-%d", b.seq),
		Seq:     b.seq,
		ReplyTo: req.ReplyTo,
		From:    req.From,
		To:      recipients,
		Content: req.Content,
		NoReply: req.NoReply,
		State:   req.State,
		Timeout: req.TimeoutSecs,
		Conv:    conv,
		TS:      b.nowUnix(),
	}
	if msg.ReplyTo == "0" {
		msg.ReplyTo = ""
	}
	b.messages[msg.ID] = msg
	b.appendConvLocked(msg)

	deadline := b.now().Add(time.Duration(req.TimeoutSecs) * time.Second).Unix()
	if b.store != nil {
		b.storeErr("message", b.store.SaveMessage(msg))
		b.storeErr("meta", b.store.SaveMeta(b.seq, b.rosterRev))
	}
	b.mu.Unlock()

	deliveries := b.relay(msg)

	if !req.NoReply {
		b.trackAwaiting(msg, deliveries, deadline)
	}

	b.changed()
	return msg, deliveries, nil
}

// resolveRecipients expands the To list: "*" becomes every registered agent
// except the sender (decision 3); explicit ids are kept as given.
func (b *Board) resolveRecipients(from string, to []string) []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if len(to) == 0 {
		return nil
	}
	if len(to) == 1 && strings.TrimSpace(to[0]) == BoardBroadcastTarget {
		out := make([]string, 0, len(b.agents))
		for _, a := range b.rosterLocked() {
			if a.ID == from {
				continue
			}
			out = append(out, a.ID)
		}
		return out
	}
	out := make([]string, 0, len(to))
	seen := map[string]bool{}
	for _, raw := range to {
		id := strings.TrimSpace(raw)
		if id == "" || id == from || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// appendConvLocked records a message in its conversation index. The caller must
// hold the write lock.
func (b *Board) appendConvLocked(msg *Message) {
	c, ok := b.convs[msg.Conv]
	if !ok {
		c = &conversation{Key: msg.Conv}
		b.convs[msg.Conv] = c
	}
	c.MessageIDs = append(c.MessageIDs, msg.ID)
	c.UpdatedAt = msg.TS
	if msg.Conv == BoardBroadcastTarget {
		c.Participants = []string{BoardBroadcastTarget}
	} else {
		c.Participants = strings.Split(msg.Conv, "+")
	}
	if b.store != nil {
		b.storeErr("conv", b.store.SaveConv(*c))
		b.storeErr("conv_msg", b.store.AppendConvMessage(msg.Conv, msg.Seq, msg.ID))
	}
}

// relay pushes a message to its recipients and reports the per-recipient
// outcome. Delivery failures never invalidate the message.
func (b *Board) relay(msg *Message) []Delivery {
	b.mu.RLock()
	link := b.link
	agents := make(map[string]bool, len(b.agents))
	for id := range b.agents {
		agents[id] = true
	}
	b.mu.RUnlock()

	out := make([]Delivery, 0, len(msg.To))
	for _, id := range msg.To {
		if !agents[id] {
			out = append(out, Delivery{To: id, Error: "unknown agent"})
			continue
		}
		if link == nil {
			out = append(out, Delivery{To: id, Error: "no agent link"})
			continue
		}
		ac := link(id)
		if ac == nil {
			out = append(out, Delivery{To: id, Error: "agent offline"})
			continue
		}
		data, err := marshalFrame(MsgTypeBoardMessage, msg)
		if err != nil {
			out = append(out, Delivery{To: id, Error: err.Error()})
			continue
		}
		if err := ac.Send(data); err != nil {
			out = append(out, Delivery{To: id, Error: err.Error()})
			continue
		}
		out = append(out, Delivery{To: id, OK: true})
	}
	return out
}

// trackAwaiting creates the "reply expected" records for delivered recipients.
func (b *Board) trackAwaiting(msg *Message, deliveries []Delivery, deadline int64) {
	b.mu.Lock()
	for _, d := range deliveries {
		if !d.OK {
			continue
		}
		key := awaitingKey(msg.ID, d.To)
		a := &Awaiting{MessageID: msg.ID, AgentID: d.To, Deadline: deadline, Status: AwaitingPending}
		b.awaiting[key] = a
		if b.store != nil {
			b.storeErr("awaiting", b.store.SaveAwaiting(a))
		}
	}
	b.mu.Unlock()
}

// awaitingKey builds the awaiting map key.
func awaitingKey(messageID, agentID string) string { return messageID + "|" + agentID }

// SetRedeliverMax sets how many times an expired reply obligation is re-sent to
// its recipient before the hub reports a timeout (0 restores the behaviour from
// before FEATURE-567). Negative values are clamped to 0.
func (b *Board) SetRedeliverMax(n int) {
	if n < 0 {
		n = 0
	}
	b.mu.Lock()
	b.redeliverMax = n
	b.mu.Unlock()
}

// TimeoutEvent reports the recipients of one message that never answered.
type TimeoutEvent struct {
	MessageID    string   `json:"message_id"`
	Conversation string   `json:"conversation,omitempty"`
	From         string   `json:"from"`
	Agents       []string `json:"agents"`
	Deadline     int64    `json:"deadline"`
	// Redelivered is how many times the message had already been re-sent to
	// the recipients before it timed out (FEATURE-567).
	Redelivered int `json:"redelivered,omitempty"`
}

// SweepTimeouts handles the reply obligations whose window elapsed: while a
// message still has redeliveries left (FEATURE-567) the hub re-sends it to the
// recipient and pushes the deadline back, and only a message whose redelivery
// budget is used up becomes a timeout. It returns one event per message that
// timed out; the caller is responsible for delivering the events to the
// original senders.
func (b *Board) SweepTimeouts() []TimeoutEvent {
	now := b.nowUnix()
	byMessage := map[string]*TimeoutEvent{}
	var redeliver []boardRedelivery

	b.mu.Lock()
	for _, a := range b.awaiting {
		if a.Status != AwaitingPending || a.Deadline > now {
			continue
		}
		if a.Redeliveries < b.redeliverMax {
			if msg, ok := b.messages[a.MessageID]; ok {
				a.Redeliveries++
				a.Deadline = now + boardRedeliveryWindow(msg.Timeout)
				if b.store != nil {
					b.storeErr("awaiting", b.store.SaveAwaiting(a))
				}
				redeliver = append(redeliver, boardRedelivery{
					agentID:    a.AgentID,
					msg:        msg,
					redelivery: a.Redeliveries,
				})
				continue
			}
		}
		a.Status = AwaitingTimeout
		if b.store != nil {
			b.storeErr("awaiting", b.store.SaveAwaiting(a))
		}
		msg, ok := b.messages[a.MessageID]
		if !ok {
			continue
		}
		ev, ok := byMessage[a.MessageID]
		if !ok {
			ev = &TimeoutEvent{
				MessageID:    a.MessageID,
				Conversation: msg.Conv,
				From:         msg.From,
				Deadline:     a.Deadline,
			}
			byMessage[a.MessageID] = ev
		}
		ev.Agents = append(ev.Agents, a.AgentID)
		ev.Redelivered = a.Redeliveries
	}
	b.mu.Unlock()

	// Re-deliveries are pushed without the board lock: an agent link may block.
	for _, r := range redeliver {
		b.notify(r.agentID, MsgTypeBoardMessage, redeliveryFrame{Message: r.msg, Redelivery: r.redelivery})
	}

	if len(byMessage) == 0 {
		if len(redeliver) > 0 {
			b.changed()
		}
		return nil
	}
	out := make([]TimeoutEvent, 0, len(byMessage))
	for _, ev := range byMessage {
		sort.Strings(ev.Agents)
		out = append(out, *ev)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].MessageID < out[j].MessageID })
	b.changed()
	return out
}

// Conversations lists conversations for the hub Web UI, most recent first with
// the broadcast conversation always first (decision: one single notice board
// conversation).
func (b *Board) Conversations() []ConversationSummary {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]ConversationSummary, 0, len(b.convs))
	for _, c := range b.convs {
		sum := ConversationSummary{
			Key:          c.Key,
			Participants: append([]string(nil), c.Participants...),
			Count:        len(c.MessageIDs),
			UpdatedAt:    c.UpdatedAt,
		}
		if len(c.MessageIDs) > 0 {
			if m, ok := b.messages[c.MessageIDs[len(c.MessageIDs)-1]]; ok {
				sum.LastContent = m.Content
				sum.LastFrom = m.From
			}
		}
		out = append(out, sum)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Key == BoardBroadcastTarget {
			return true
		}
		if out[j].Key == BoardBroadcastTarget {
			return false
		}
		if out[i].UpdatedAt != out[j].UpdatedAt {
			return out[i].UpdatedAt > out[j].UpdatedAt
		}
		return out[i].Key < out[j].Key
	})
	return out
}

// Messages returns messages of one conversation in chronological order. before
// is an exclusive seq cursor (0 = latest page); limit defaults to 50.
func (b *Board) Messages(convKey string, before, limit int) []*Message {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	c, ok := b.convs[convKey]
	if !ok {
		return nil
	}
	ids := c.MessageIDs
	if before > 0 {
		kept := ids[:0:0]
		for _, id := range ids {
			if m, ok := b.messages[id]; ok && m.Seq < before {
				kept = append(kept, id)
			}
		}
		ids = kept
	}
	if len(ids) > limit {
		ids = ids[len(ids)-limit:]
	}
	out := make([]*Message, 0, len(ids))
	for _, id := range ids {
		if m, ok := b.messages[id]; ok {
			out = append(out, m)
		}
	}
	return out
}

// Pending returns the still unanswered reply obligations of one message.
func (b *Board) Pending(messageID string) []Awaiting {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]Awaiting, 0, 4)
	for _, a := range b.awaiting {
		if a.MessageID == messageID {
			out = append(out, *a)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AgentID < out[j].AgentID })
	return out
}

// BroadcastRoster pushes the current roster to every connected agent.
func (b *Board) BroadcastRoster() {
	b.mu.RLock()
	link := b.link
	rev := b.rosterRev
	agents := b.rosterLocked()
	b.mu.RUnlock()
	if link == nil {
		return
	}
	payload := map[string]interface{}{"rev": rev, "agents": agents}
	for _, a := range agents {
		b.notify(a.ID, MsgTypeBoardRoster, payload)
	}
}

// notify pushes one typed payload down to a single agent.
func (b *Board) notify(agentID, msgType string, payload interface{}) {
	b.mu.RLock()
	link := b.link
	b.mu.RUnlock()
	if link == nil {
		return
	}
	ac := link(agentID)
	if ac == nil {
		return
	}
	data, err := marshalFrame(msgType, payload)
	if err != nil {
		log.Printf("board: marshal %s for %s failed: %v", msgType, agentID, err)
		return
	}
	if err := ac.Send(data); err != nil {
		log.Printf("board: notify %s -> %s failed: %v", msgType, agentID, err)
	}
}

// Notify pushes one typed payload down to a single agent (exported for the
// proxy: registration acks, send acks, timeout events).
func (b *Board) Notify(agentID, msgType string, payload interface{}) {
	b.notify(agentID, msgType, payload)
}

// marshalFrame builds a flat board frame for co-shell agents: the payload
// fields sit directly next to "type" (no envelope wrapper). The gateway client
// path (TCP / browser) keeps using Envelope instead.
func marshalFrame(msgType string, payload interface{}) ([]byte, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	fields := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	typeRaw, err := json.Marshal(msgType)
	if err != nil {
		return nil, err
	}
	fields["type"] = typeRaw
	return json.Marshal(fields)
}

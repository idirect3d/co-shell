// Author: L.Shuang
// Created: 2026-09-28
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
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/idirect3d/co-shell/log"
)

// FEATURE-567: board messages that expect a reply become a duty of the
// recipient. The duty list is injected into <environment_details> and
// attempt_completion is refused while a duty is still open.

// BoardObligationMaxBlocked is how many consecutive refusals one unanswered
// message may cause before the completion gate lets the agent finish anyway.
// The cap keeps a stale or forgotten message from deadlocking the agent.
const BoardObligationMaxBlocked = 3

// boardObligation records one delivered board message that expects a reply and
// has not been answered yet.
type boardObligation struct {
	// MessageID is the hub id of the message awaiting a reply.
	MessageID string
	// From is the sender agent id.
	From string
	// Deadline is the unix second at which the hub's reply window closes
	// (0 when the sender gave no usable window).
	Deadline int64
	// CreatedAt is the unix second the obligation was registered.
	CreatedAt int64
	// Blocked counts how many times the completion gate refused to finish
	// the task because of this obligation.
	Blocked int
}

// BoardMustReplyEnabled reports whether the hard "must reply" gate is on
// (config board-must-reply, on by default). When it is off the duties are still
// tracked and shown to the LLM, but attempt_completion is never refused.
func (a *Agent) BoardMustReplyEnabled() bool {
	if a.cfg != nil {
		return a.cfg.BoardMustReply
	}
	return true
}

// AddBoardObligation registers the reply duty of one delivered message.
// Messages that explicitly ask for no reply are ignored, and a message that is
// already pending only refreshes its sender and deadline (the hub may
// re-deliver a message whose reply window expired).
func (a *Agent) AddBoardObligation(m BoardMessage) {
	if m.NoReply || m.ID == "" {
		return
	}
	deadline := int64(0)
	if m.Timeout > 0 && m.TS > 0 {
		deadline = m.TS + int64(m.Timeout)
	}
	st := a.boardStateFor()
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.obligations == nil {
		st.obligations = map[string]*boardObligation{}
	}
	if old, ok := st.obligations[m.ID]; ok {
		old.From = m.From
		if deadline > 0 {
			old.Deadline = deadline
		}
		return
	}
	st.obligations[m.ID] = &boardObligation{
		MessageID: m.ID,
		From:      m.From,
		Deadline:  deadline,
		CreatedAt: time.Now().Unix(),
	}
}

// ResolveBoardObligation drops the reply duty of one message id and reports
// whether such a duty was pending.
func (a *Agent) ResolveBoardObligation(id string) bool {
	if id == "" {
		return false
	}
	st := a.boardStateFor()
	st.mu.Lock()
	defer st.mu.Unlock()
	if _, ok := st.obligations[id]; !ok {
		return false
	}
	delete(st.obligations, id)
	return true
}

// PendingBoardObligations returns a copy of the unanswered duties, oldest first.
func (a *Agent) PendingBoardObligations() []boardObligation {
	st := a.boardStateFor()
	st.mu.Lock()
	defer st.mu.Unlock()
	out := make([]boardObligation, 0, len(st.obligations))
	for _, ob := range st.obligations {
		out = append(out, *ob)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt != out[j].CreatedAt {
			return out[i].CreatedAt < out[j].CreatedAt
		}
		return out[i].MessageID < out[j].MessageID
	})
	return out
}

// BoardObligationBlock renders the <board_replies> block injected into
// <environment_details> while messages wait for an answer. It returns "" when
// nothing is pending, in which case the whole block is omitted.
func (a *Agent) BoardObligationBlock() string {
	if !a.BoardEnabled() {
		return ""
	}
	pending := a.PendingBoardObligations()
	if len(pending) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("<board_replies pending=\"")
	sb.WriteString(strconv.Itoa(len(pending)))
	sb.WriteString("\">\n")
	for _, ob := range pending {
		sb.WriteString("  <reply_to message_id=")
		sb.WriteString(strconv.Quote(ob.MessageID))
		sb.WriteString(" from=")
		sb.WriteString(strconv.Quote(ob.From))
		sb.WriteString(" remaining=")
		sb.WriteString(strconv.Quote(boardObligationRemaining(ob, time.Now().Unix())))
		sb.WriteString("/>\n")
	}
	sb.WriteString("</board_replies>")
	return sb.String()
}

// boardObligationRemaining renders how long the reply window of one duty is
// still open ("118s", "expired" or "unknown").
func boardObligationRemaining(ob boardObligation, now int64) string {
	if ob.Deadline <= 0 {
		return "unknown"
	}
	left := ob.Deadline - now
	if left <= 0 {
		return "expired"
	}
	return strconv.FormatInt(left, 10) + "s"
}

// BoardCompletionGate decides whether attempt_completion must be refused because
// board messages that expect a reply are still unanswered. It returns the
// refusal text and true when the call has to be blocked.
//
// Every pending duty counts the refusal; once a duty has been refused
// BoardObligationMaxBlocked times the gate lets the agent finish anyway (and
// logs a warning) so a forgotten message cannot deadlock the task.
func (a *Agent) BoardCompletionGate() (string, bool) {
	if !a.BoardMustReplyEnabled() {
		return "", false
	}
	st := a.boardStateFor()
	now := time.Now().Unix()
	st.mu.Lock()
	if len(st.obligations) == 0 {
		st.mu.Unlock()
		return "", false
	}
	open := make([]boardObligation, 0, len(st.obligations))
	released := make([]boardObligation, 0, len(st.obligations))
	for _, ob := range st.obligations {
		ob.Blocked++
		if ob.Blocked > BoardObligationMaxBlocked {
			// Deadlock guard: stop refusing, clear the counter for a
			// possible next cycle.
			ob.Blocked = 0
			released = append(released, *ob)
			continue
		}
		open = append(open, *ob)
	}
	st.mu.Unlock()

	for _, ob := range released {
		log.Warn("Agent: board reply gate released after %d refusals (message_id=%s from=%s)",
			BoardObligationMaxBlocked, ob.MessageID, ob.From)
	}
	if len(open) == 0 {
		return "", false
	}
	return buildBoardReplyGateText(open, now), true
}

// The refusal below is deliberately bilingual (English + Chinese) so the LLM
// reads the duty in both the configured UI language and English.
const (
	boardReplyGateHeaderEN = "[board-must-reply] attempt_completion refused: %d board message(s) still expect your reply:"
	boardReplyGateHeaderZH = "[board-must-reply] 已拒绝 attempt_completion：仍有 %d 条 board 消息等待你的回复："
	boardReplyGateHintEN   = "Answer with board_send(reply_to=<message_id>, state=handled|declined|need_info) first, or disable the gate with :set board-must-reply off."
	boardReplyGateHintZH   = "请先用 board_send(reply_to=<message_id>, state=handled|declined|need_info) 答复后再结束任务；可用 :set board-must-reply off 关闭该门禁。"
)

// buildBoardReplyGateText renders the refusal handed back to the LLM. The text
// is bilingual (English + Chinese) and lists every open duty with its sender and
// remaining reply window.
func buildBoardReplyGateText(open []boardObligation, now int64) string {
	items := make([]string, 0, len(open))
	for _, ob := range open {
		items = append(items, "  - message_id="+ob.MessageID+
			" from="+ob.From+
			" remaining="+boardObligationRemaining(ob, now))
	}
	list := strings.Join(items, "\n")
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(boardReplyGateHeaderEN, len(open)))
	sb.WriteString("\n")
	sb.WriteString(list)
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf(boardReplyGateHeaderZH, len(open)))
	sb.WriteString("\n")
	sb.WriteString(list)
	sb.WriteString("\n")
	sb.WriteString(boardReplyGateHintEN)
	sb.WriteString(" ")
	sb.WriteString(boardReplyGateHintZH)
	return sb.String()
}

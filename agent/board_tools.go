// Author: L.Shuang
// Created: 2026-09-12
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
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// BoardSender is the callback the web session installs so the agent can push a
// board frame back to the hub over the WebSocket connection the hub opened to
// us (FEATURE-530). The map is marshalled as flat JSON.
type BoardSender func(msg map[string]interface{}) error

// boardMaxContentBytes caps one board message body.
const boardMaxContentBytes = 8 << 10

// boardLink returns the installed hub link (nil when no hub is connected).
func (a *Agent) boardLink() BoardSender {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.boardSender
}

// boardSendTool sends one message to other agents through the hub board and
// returns the hub's immediate acknowledgement (message id, conversation and
// per-recipient delivery outcome). The hub relays asynchronously, so the tool
// does not wait for the recipients to answer.
func (a *Agent) boardSendTool(_ context.Context, args map[string]interface{}) (string, error) {
	to, err := boardRecipients(args["to"])
	if err != nil {
		return "", err
	}
	content := strings.TrimSpace(boardArgString(args, "content"))
	if content == "" {
		return "", fmt.Errorf("content is required")
	}
	if len(content) > boardMaxContentBytes {
		return "", fmt.Errorf("content must be at most %d bytes", boardMaxContentBytes)
	}
	replyTo := strings.TrimSpace(boardArgString(args, "reply_to"))
	state := strings.TrimSpace(boardArgString(args, "state"))
	switch state {
	case "", BoardStateHandled, BoardStateDeclined, BoardStateNeedInfo:
	default:
		return "", fmt.Errorf("state must be one of %s/%s/%s",
			BoardStateHandled, BoardStateDeclined, BoardStateNeedInfo)
	}
	if state != "" && replyTo == "" {
		return "", fmt.Errorf("state requires reply_to (reply to the message you received)")
	}
	timeout, ok := boardArgInt(args, "timeout_seconds")
	if !ok {
		return "", fmt.Errorf("timeout_seconds is required (%d..%d)",
			BoardMinTimeoutSeconds, BoardMaxTimeoutSeconds)
	}
	if timeout < BoardMinTimeoutSeconds || timeout > BoardMaxTimeoutSeconds {
		return "", fmt.Errorf("timeout_seconds must be between %d and %d",
			BoardMinTimeoutSeconds, BoardMaxTimeoutSeconds)
	}
	noReply := boardArgBool(args, "no_reply")

	link := a.boardLink()
	if link == nil {
		return "", fmt.Errorf("board is not connected (the hub link is down)")
	}
	reqID := "r-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	frame := map[string]interface{}{
		"type":            "board_send",
		"req_id":          reqID,
		"to":              to,
		"content":         content,
		"reply_to":        replyTo,
		"no_reply":        noReply,
		"state":           state,
		"timeout_seconds": timeout,
	}
	if err := link(frame); err != nil {
		return "", fmt.Errorf("send board_send: %w", err)
	}
	ack, err := a.AwaitBoardAck(reqID, a.BoardAckTimeoutDuration())
	if err != nil {
		return "", fmt.Errorf("board send: %w", err)
	}
	if !ack.OK {
		reason := ack.Error
		if reason == "" {
			reason = "unknown error"
		}
		return "", fmt.Errorf("hub rejected board_send: %s", reason)
	}
	return boardAckSummary(ack, noReply), nil
}

// boardAckSummary renders the tool result, e.g.
//
//	message_id=m-3; conversation=*; delivered=2/3 (agent-c: agent offline); reply_window=300s
func boardAckSummary(ack *BoardAck, noReply bool) string {
	delivered := 0
	failed := make([]string, 0, len(ack.Deliveries))
	for _, d := range ack.Deliveries {
		if d.OK {
			delivered++
			continue
		}
		reason := d.Error
		if reason == "" {
			reason = "undeliverable"
		}
		failed = append(failed, d.To+": "+reason)
	}
	var sb strings.Builder
	sb.WriteString("message_id=" + ack.MessageID)
	sb.WriteString("; conversation=" + ack.Conversation)
	sb.WriteString(fmt.Sprintf("; delivered=%d/%d", delivered, len(ack.Deliveries)))
	if len(failed) > 0 {
		sb.WriteString(" (" + strings.Join(failed, ", ") + ")")
	}
	if noReply {
		sb.WriteString("; no reply requested")
	} else {
		sb.WriteString(fmt.Sprintf("; reply_window=%ds", ack.Timeout))
	}
	return sb.String()
}

// boardRecipients parses the "to" argument: a single id, a list of ids or "*".
func boardRecipients(v interface{}) ([]string, error) {
	var raw []string
	switch t := v.(type) {
	case string:
		if strings.TrimSpace(t) != "" {
			raw = strings.Split(t, ",")
		}
	case []interface{}:
		for _, item := range t {
			s, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("to must contain agent ids")
			}
			raw = append(raw, s)
		}
	case []string:
		raw = t
	case nil:
	default:
		return nil, fmt.Errorf("to must be an agent id, a list of ids or \"%s\"", BoardBroadcastTarget)
	}
	out := make([]string, 0, len(raw))
	seen := map[string]bool{}
	for _, r := range raw {
		id := strings.TrimSpace(r)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("to is required (agent ids or \"%s\")", BoardBroadcastTarget)
	}
	if len(out) > 1 {
		for _, id := range out {
			if id == BoardBroadcastTarget {
				return nil, fmt.Errorf("\"%s\" cannot be combined with other recipients", BoardBroadcastTarget)
			}
		}
	}
	return out, nil
}

// boardArgString returns a string argument ("" when absent).
func boardArgString(args map[string]interface{}, key string) string {
	if s, ok := args[key].(string); ok {
		return s
	}
	return ""
}

// boardArgBool returns a bool argument (false when absent).
func boardArgBool(args map[string]interface{}, key string) bool {
	switch v := args[key].(type) {
	case bool:
		return v
	case string:
		return v == "true" || v == "1"
	default:
		return false
	}
}

// boardArgInt returns an int argument and whether it was supplied.
func boardArgInt(args map[string]interface{}, key string) (int, bool) {
	switch v := args[key].(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return 0, false
		}
		return n, true
	default:
		return 0, false
	}
}

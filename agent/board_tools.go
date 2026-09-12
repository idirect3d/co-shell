package agent

import (
	"context"
	"encoding/json"
	"fmt"
)

// BoardSender is the callback the web session installs so the agent can push
// a board_* message back to the hub over the WebSocket connection
// (FEATURE-490). It is nil when the agent is not running in serve mode.
type BoardSender func(msg map[string]interface{}) error

// boardResultTool lets the assignee agent report the result of a board task
// back to the hub (FEATURE-490). The hub's Board.Result then marks the task
// done and notifies the requester. The tool is only registered when the board
// switch is enabled.
func (a *Agent) boardResultTool(ctx context.Context, args map[string]interface{}) (string, error) {
	taskID, _ := args["task_id"].(string)
	result, _ := args["result"].(string)
	if taskID == "" {
		return "", fmt.Errorf("task_id is required")
	}
	if a.boardSender == nil {
		return "", fmt.Errorf("board sender is not available (not running in serve mode)")
	}
	msg := map[string]interface{}{
		"type":    "board_result",
		"task_id": taskID,
		"result":  result,
	}
	if err := a.boardSender(msg); err != nil {
		return "", fmt.Errorf("send board_result: %w", err)
	}
	return "board_result sent to hub", nil
}

// boardPostTool lets the agent publish a help request to the hub board
// (FEATURE-490). Only registered when the board switch is enabled.
func (a *Agent) boardPostTool(ctx context.Context, args map[string]interface{}) (string, error) {
	title, _ := args["title"].(string)
	description, _ := args["description"].(string)
	requiredRole, _ := args["required_role"].(string)
	if title == "" {
		return "", fmt.Errorf("title is required")
	}
	if a.boardSender == nil {
		return "", fmt.Errorf("board sender is not available (not running in serve mode)")
	}
	msg := map[string]interface{}{
		"type":          "board_post",
		"title":         title,
		"description":   description,
		"required_role": requiredRole,
	}
	if err := a.boardSender(msg); err != nil {
		return "", fmt.Errorf("send board_post: %w", err)
	}
	return "board_post sent to hub", nil
}

// boardListTool lets the agent poll the hub board for open requests
// (FEATURE-490). The hub responds with the request list.
func (a *Agent) boardListTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if a.boardSender == nil {
		return "", fmt.Errorf("board sender is not available (not running in serve mode)")
	}
	msg := map[string]interface{}{
		"type":      "board_list",
		"open_only": "true",
	}
	if err := a.boardSender(msg); err != nil {
		return "", fmt.Errorf("send board_list: %w", err)
	}
	return "board_list sent to hub (check the response)", nil
}

// boardClaimTool lets the agent claim an open request (FEATURE-490).
func (a *Agent) boardClaimTool(ctx context.Context, args map[string]interface{}) (string, error) {
	requestID, _ := args["request_id"].(string)
	if requestID == "" {
		return "", fmt.Errorf("request_id is required")
	}
	if a.boardSender == nil {
		return "", fmt.Errorf("board sender is not available (not running in serve mode)")
	}
	msg := map[string]interface{}{
		"type":       "board_claim",
		"request_id": requestID,
	}
	if err := a.boardSender(msg); err != nil {
		return "", fmt.Errorf("send board_claim: %w", err)
	}
	return "board_claim sent to hub", nil
}

// boardDMTool lets a participant send a direct message in a request thread
// (FEATURE-490).
func (a *Agent) boardDMTool(ctx context.Context, args map[string]interface{}) (string, error) {
	requestID, _ := args["request_id"].(string)
	content, _ := args["content"].(string)
	if requestID == "" || content == "" {
		return "", fmt.Errorf("request_id and content are required")
	}
	if a.boardSender == nil {
		return "", fmt.Errorf("board sender is not available (not running in serve mode)")
	}
	msg := map[string]interface{}{
		"type":       "board_dm",
		"request_id": requestID,
		"content":    content,
	}
	if err := a.boardSender(msg); err != nil {
		return "", fmt.Errorf("send board_dm: %w", err)
	}
	return "board_dm sent to hub", nil
}

// boardConfirmTool lets the requester confirm a claimed request and start
// execution (FEATURE-490).
func (a *Agent) boardConfirmTool(ctx context.Context, args map[string]interface{}) (string, error) {
	requestID, _ := args["request_id"].(string)
	if requestID == "" {
		return "", fmt.Errorf("request_id is required")
	}
	if a.boardSender == nil {
		return "", fmt.Errorf("board sender is not available (not running in serve mode)")
	}
	msg := map[string]interface{}{
		"type":       "board_confirm",
		"request_id": requestID,
	}
	if err := a.boardSender(msg); err != nil {
		return "", fmt.Errorf("send board_confirm: %w", err)
	}
	return "board_confirm sent to hub", nil
}

// marshalBoardArgs is a helper for tests.
func marshalBoardArgs(m map[string]interface{}) string {
	data, _ := json.Marshal(m)
	return string(data)
}

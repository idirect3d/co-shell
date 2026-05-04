// Author: L.Shuang
// Created: 2026-05-04
// Last Modified: 2026-05-04
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

package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"

	"github.com/idirect3d/co-shell/bridge"
)

const (
	// maxMessageLen is the maximum length of a Feishu text message content.
	// Feishu has a limit of approximately 30KB for text messages.
	// We use a conservative limit to avoid 413 errors.
	maxMessageLen = 28000
)

// Handler processes incoming Feishu messages.
type Handler struct {
	larkClient *lark.Client
	scheduler  *bridge.Scheduler
	uploadDir  string

	// pendingInputs maps chatID to a channel for receiving user input
	// when co-shell is waiting for interactive input.
	mu            sync.Mutex
	pendingInputs map[string]chan string
}

// NewHandler creates a new message handler.
func NewHandler(larkClient *lark.Client, scheduler *bridge.Scheduler, workspace string) *Handler {
	return &Handler{
		larkClient: larkClient,
		scheduler:  scheduler,
		uploadDir:  filepath.Join(workspace, "upload"),
	}
}

// HandleSDKEvent processes a Feishu event received via the SDK WebSocket client.
func (h *Handler) HandleSDKEvent(ctx context.Context, event *larkim.P2MessageReceiveV1) {
	if event == nil || event.Event == nil || event.Event.Message == nil {
		log.Printf("Received nil event or message")
		return
	}

	msg := event.Event.Message
	chatID := msg.ChatId
	msgType := msg.MessageType
	content := msg.Content
	chatType := msg.ChatType

	if chatID == nil || msgType == nil || content == nil || chatType == nil {
		log.Printf("Received event with nil fields")
		return
	}

	// Handle different message types
	switch *msgType {
	case "text":
		h.handleTextMessage(ctx, *chatID, *content, *chatType)
	case "file":
		h.handleFileMessage(ctx, *chatID, *content)
	case "image":
		h.handleImageMessage(ctx, *chatID, *content, msg.MessageId)
	default:
		log.Printf("Unsupported message type: %s", *msgType)
	}
}

// handleTextMessage processes a text message.
// If there is a pending input request for this chat, the message is treated
// as a reply to that request instead of a new instruction.
func (h *Handler) handleTextMessage(ctx context.Context, chatID, content, chatType string) {
	// Parse text content
	var textContent struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(content), &textContent); err != nil {
		log.Printf("Cannot parse text content: %v", err)
		return
	}

	instruction := textContent.Text

	// For group chats, remove @mention prefix
	if chatType == "group" {
		instruction = removeAtMention(instruction)
	}

	instruction = strings.TrimSpace(instruction)
	if instruction == "" {
		return
	}

	// Check if there is a pending input request for this chat
	h.mu.Lock()
	inputCh, hasPending := h.pendingInputs[chatID]
	h.mu.Unlock()

	if hasPending {
		// This message is a reply to a pending input request
		fmt.Printf("\n📩 [飞书输入回复] %s\n", instruction)
		fmt.Println(strings.Repeat("─", 50))

		// Send the input to the waiting co-shell process
		select {
		case inputCh <- instruction:
			// Input sent successfully
		default:
			// Channel full or closed - treat as new instruction
			log.Printf("Input channel full or closed for chat %s, treating as new instruction", chatID)
			hasPending = false
		}
	}

	if !hasPending {
		// This is a new instruction
		fmt.Printf("\n📩 [飞书消息] %s\n", instruction)
		fmt.Println(strings.Repeat("─", 50))

		// Submit to scheduler with InputRequestFunc for interactive support
		h.scheduler.Submit(bridge.Message{
			ChatID:      chatID,
			Instruction: instruction,
			InputRequestFunc: func(currentOutput string) <-chan string {
				return h.createInputRequest(ctx, chatID, currentOutput)
			},
			ReplyFunc: func(output string, err error) {
				if err != nil {
					reply := fmt.Sprintf("❌ 执行出错：%v", err)
					if output != "" {
						reply += "\n\n输出：\n" + output
					}
					fmt.Printf("\n📤 [回复飞书] %s\n", reply)
					fmt.Println(strings.Repeat("─", 50))
					if sendErr := h.sendTextMessage(ctx, chatID, reply); sendErr != nil {
						log.Printf("Failed to send error reply: %v", sendErr)
					}
					return
				}
				fmt.Printf("\n📤 [回复飞书] %s\n", output)
				fmt.Println(strings.Repeat("─", 50))
				if sendErr := h.sendTextMessage(ctx, chatID, output); sendErr != nil {
					log.Printf("Failed to send reply: %v", sendErr)
				}
			},
		})
	}
}

// createInputRequest creates a pending input request for a chat.
// It sends the current output to the user via Feishu and returns a channel
// that will receive the user's reply.
func (h *Handler) createInputRequest(ctx context.Context, chatID, currentOutput string) <-chan string {
	// Create a channel for receiving the user's input
	inputCh := make(chan string, 1)

	// Register the pending input request
	h.mu.Lock()
	if h.pendingInputs == nil {
		h.pendingInputs = make(map[string]chan string)
	}
	h.pendingInputs[chatID] = inputCh
	h.mu.Unlock()

	// Send the current output to the user via Feishu, asking for input
	message := currentOutput + "\n\n---\n⚠️ co-shell 需要您的输入才能继续。\n请直接回复此消息，输入您的内容。\n• 直接输入内容作为回复\n• 输入 /cancel 取消操作\n• 输入 /approve 确认执行"

	fmt.Printf("\n⏳ [等待飞书输入] %s\n", chatID)
	fmt.Println(strings.Repeat("─", 50))

	if sendErr := h.sendTextMessage(ctx, chatID, message); sendErr != nil {
		log.Printf("Failed to send input request: %v", sendErr)
		// Clean up on error
		h.mu.Lock()
		delete(h.pendingInputs, chatID)
		h.mu.Unlock()
		close(inputCh)
		return nil
	}

	return inputCh
}

// handleFileMessage processes a file message.
func (h *Handler) handleFileMessage(ctx context.Context, chatID, content string) {
	var fileContent struct {
		FileKey string `json:"file_key"`
	}
	if err := json.Unmarshal([]byte(content), &fileContent); err != nil {
		log.Printf("Cannot parse file content: %v", err)
		return
	}

	// Note: File download via SDK requires message_id which is not available
	// in the current event structure. This is a placeholder for future implementation.
	log.Printf("File message received, file_key: %s", fileContent.FileKey)
	reply := "✅ 已收到文件，请发送指令让我处理。"
	if sendErr := h.sendTextMessage(ctx, chatID, reply); sendErr != nil {
		log.Printf("Failed to send file reply: %v", sendErr)
	}
}

// handleImageMessage processes an image message.
func (h *Handler) handleImageMessage(ctx context.Context, chatID, content string, messageID *string) {
	var imageContent struct {
		ImageKey string `json:"image_key"`
	}
	if err := json.Unmarshal([]byte(content), &imageContent); err != nil {
		log.Printf("Cannot parse image content: %v", err)
		return
	}

	// Note: Image download via SDK requires message_id which is available
	// in the event. This is a placeholder for future implementation.
	log.Printf("Image message received, image_key: %s", imageContent.ImageKey)
	reply := "✅ 已收到图片，请发送指令让我处理。"
	if sendErr := h.sendTextMessage(ctx, chatID, reply); sendErr != nil {
		log.Printf("Failed to send image reply: %v", sendErr)
	}
}

// truncateMessage truncates a message to fit within Feishu's size limit.
func truncateMessage(text string) string {
	if len(text) <= maxMessageLen {
		return text
	}
	// Truncate to maxMessageLen bytes, preserving UTF-8 character boundaries
	truncated := text[:maxMessageLen]
	// Find the last valid UTF-8 rune boundary
	for i := len(truncated) - 1; i >= 0; i-- {
		if truncated[i]&0xC0 != 0x80 {
			truncated = truncated[:i+1]
			break
		}
	}
	return truncated + "\n\n...（内容过长已截断，共 " + fmt.Sprintf("%d", len(text)) + " 字节）"
}

// sendTextMessage sends a text message to a chat using the SDK.
func (h *Handler) sendTextMessage(ctx context.Context, chatID, text string) error {
	text = truncateMessage(text)
	// Build JSON content manually to ensure proper escaping.
	// The SDK's TextMsgBuilder does not escape special characters.
	contentBytes, err := json.Marshal(map[string]string{"text": text})
	if err != nil {
		return fmt.Errorf("marshal message content failed: %w", err)
	}
	content := string(contentBytes)

	req := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(larkim.ReceiveIdTypeChatId).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			MsgType(larkim.MsgTypeText).
			ReceiveId(chatID).
			Content(content).
			Build()).
		Build()

	resp, err := h.larkClient.Im.Message.Create(ctx, req)
	if err != nil {
		return fmt.Errorf("send message failed: %w", err)
	}
	if !resp.Success() {
		return fmt.Errorf("send message API error: code=%d msg=%s", resp.Code, resp.Msg)
	}

	return nil
}

// removeAtMention removes @mention prefix from group chat messages.
func removeAtMention(text string) string {
	if strings.HasPrefix(text, "@") {
		parts := strings.SplitN(text, " ", 2)
		if len(parts) > 1 {
			return parts[1]
		}
		return ""
	}
	return text
}

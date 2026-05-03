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

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"

	"github.com/idirect3d/co-shell/bridge"
)

// Handler processes incoming Feishu messages.
type Handler struct {
	larkClient *lark.Client
	scheduler  *bridge.Scheduler
	uploadDir  string
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

	// Submit to scheduler
	h.scheduler.Submit(bridge.Message{
		ChatID:      chatID,
		Instruction: instruction,
		ReplyFunc: func(output string, err error) {
			if err != nil {
				reply := fmt.Sprintf("❌ 执行出错：%v", err)
				if output != "" {
					reply += "\n\n输出：\n" + output
				}
				if sendErr := h.sendTextMessage(ctx, chatID, reply); sendErr != nil {
					log.Printf("Failed to send error reply: %v", sendErr)
				}
				return
			}
			if sendErr := h.sendTextMessage(ctx, chatID, output); sendErr != nil {
				log.Printf("Failed to send reply: %v", sendErr)
			}
		},
	})
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

// sendTextMessage sends a text message to a chat using the SDK.
func (h *Handler) sendTextMessage(ctx context.Context, chatID, text string) error {
	content := larkim.NewTextMsgBuilder().
		TextLine(text).
		Build()

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

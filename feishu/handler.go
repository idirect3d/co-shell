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
	"encoding/json"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/idirect3d/co-shell/bridge"
)

// Event represents a Feishu WebSocket event.
type Event struct {
	ID     string          `json:"id"`
	Type   string          `json:"type"`
	Schema string          `json:"schema"`
	Header EventHeader     `json:"header"`
	Event  json.RawMessage `json:"event"`
}

// EventHeader contains event metadata.
type EventHeader struct {
	EventID    string `json:"event_id"`
	Token      string `json:"token"`
	CreateTime string `json:"create_time"`
	EventType  string `json:"event_type"`
	TenantKey  string `json:"tenant_key"`
	AppID      string `json:"app_id"`
}

// MessageEvent represents the im.message.receive_v1 event body.
type MessageEvent struct {
	Sender   Sender          `json:"sender"`
	Message  json.RawMessage `json:"message"`
	ChatType string          `json:"chat_type"`
}

// Sender represents the message sender.
type Sender struct {
	SenderID   SenderID `json:"sender_id"`
	SenderType string   `json:"sender_type"`
	TenantKey  string   `json:"tenant_key"`
}

// SenderID represents the sender's ID.
type SenderID struct {
	UnionID string `json:"union_id"`
	UserID  string `json:"user_id"`
	OpenID  string `json:"open_id"`
}

// MsgBody represents the message body.
type MsgBody struct {
	MessageID  string `json:"message_id"`
	RootID     string `json:"root_id"`
	ParentID   string `json:"parent_id"`
	ChatID     string `json:"chat_id"`
	ChatType   string `json:"chat_type"`
	MsgType    string `json:"msg_type"`
	Content    string `json:"content"`
	CreateTime string `json:"create_time"`
}

// TextContent represents the parsed text message content.
type TextContent struct {
	Text string `json:"text"`
}

// FileContent represents the parsed file message content.
type FileContent struct {
	FileKey string `json:"file_key"`
}

// ImageContent represents the parsed image message content.
type ImageContent struct {
	ImageKey string `json:"image_key"`
}

// Handler processes incoming Feishu messages.
type Handler struct {
	client    *Client
	scheduler *bridge.Scheduler
	uploadDir string
}

// NewHandler creates a new message handler.
func NewHandler(client *Client, scheduler *bridge.Scheduler, workspace string) *Handler {
	return &Handler{
		client:    client,
		scheduler: scheduler,
		uploadDir: filepath.Join(workspace, "upload"),
	}
}

// HandleEvent processes a Feishu event.
func (h *Handler) HandleEvent(event Event) error {
	// Only process message receive events
	if event.Header.EventType != "im.message.receive_v1" {
		return nil
	}

	// Parse the message event
	var msgEvent MessageEvent
	if err := json.Unmarshal(event.Event, &msgEvent); err != nil {
		return fmt.Errorf("cannot parse message event: %w", err)
	}

	// Parse the message body
	var msgBody MsgBody
	if err := json.Unmarshal(msgEvent.Message, &msgBody); err != nil {
		return fmt.Errorf("cannot parse message body: %w", err)
	}

	// Handle different message types
	switch msgBody.MsgType {
	case "text":
		return h.handleTextMessage(msgBody)
	case "file":
		return h.handleFileMessage(msgBody)
	case "image":
		return h.handleImageMessage(msgBody)
	default:
		log.Printf("Unsupported message type: %s", msgBody.MsgType)
		return nil
	}
}

// handleTextMessage processes a text message.
func (h *Handler) handleTextMessage(msg MsgBody) error {
	// Parse text content
	var textContent TextContent
	if err := json.Unmarshal([]byte(msg.Content), &textContent); err != nil {
		return fmt.Errorf("cannot parse text content: %w", err)
	}

	instruction := textContent.Text

	// For group chats, remove @mention prefix
	if msg.ChatType == "group" {
		instruction = removeAtMention(instruction)
	}

	instruction = strings.TrimSpace(instruction)
	if instruction == "" {
		return nil
	}

	// Submit to scheduler
	h.scheduler.Submit(bridge.Message{
		ChatID:      msg.ChatID,
		Instruction: instruction,
		ReplyFunc: func(output string, err error) {
			if err != nil {
				reply := fmt.Sprintf("❌ 执行出错：%v", err)
				if output != "" {
					reply += "\n\n输出：\n" + output
				}
				if sendErr := h.client.SendMessage(msg.ChatID, reply); sendErr != nil {
					log.Printf("Failed to send error reply: %v", sendErr)
				}
				return
			}
			if sendErr := h.client.SendMessage(msg.ChatID, output); sendErr != nil {
				log.Printf("Failed to send reply: %v", sendErr)
			}
		},
	})

	return nil
}

// handleFileMessage processes a file message.
// Downloads the file to the upload directory without processing.
func (h *Handler) handleFileMessage(msg MsgBody) error {
	var fileContent FileContent
	if err := json.Unmarshal([]byte(msg.Content), &fileContent); err != nil {
		return fmt.Errorf("cannot parse file content: %w", err)
	}

	localPath, err := h.client.DownloadFile(fileContent.FileKey, h.uploadDir)
	if err != nil {
		log.Printf("Failed to download file: %v", err)
		reply := fmt.Sprintf("❌ 文件下载失败：%v", err)
		if sendErr := h.client.SendMessage(msg.ChatID, reply); sendErr != nil {
			log.Printf("Failed to send error reply: %v", sendErr)
		}
		return err
	}

	log.Printf("File downloaded to: %s", localPath)
	reply := fmt.Sprintf("✅ 文件已保存到：%s\n请发送指令让我处理。", localPath)
	if sendErr := h.client.SendMessage(msg.ChatID, reply); sendErr != nil {
		log.Printf("Failed to send file reply: %v", sendErr)
	}

	return nil
}

// handleImageMessage processes an image message.
// Downloads the image to the upload directory without processing.
func (h *Handler) handleImageMessage(msg MsgBody) error {
	var imageContent ImageContent
	if err := json.Unmarshal([]byte(msg.Content), &imageContent); err != nil {
		return fmt.Errorf("cannot parse image content: %w", err)
	}

	localPath, err := h.client.GetMessageResource(msg.MessageID, imageContent.ImageKey, h.uploadDir)
	if err != nil {
		log.Printf("Failed to download image: %v", err)
		reply := fmt.Sprintf("❌ 图片下载失败：%v", err)
		if sendErr := h.client.SendMessage(msg.ChatID, reply); sendErr != nil {
			log.Printf("Failed to send error reply: %v", sendErr)
		}
		return err
	}

	log.Printf("Image downloaded to: %s", localPath)
	reply := fmt.Sprintf("✅ 图片已保存到：%s\n请发送指令让我处理。", localPath)
	if sendErr := h.client.SendMessage(msg.ChatID, reply); sendErr != nil {
		log.Printf("Failed to send image reply: %v", sendErr)
	}

	return nil
}

// removeAtMention removes @mention prefix from group chat messages.
func removeAtMention(text string) string {
	// Feishu @mention format: @_user_name text
	if strings.HasPrefix(text, "@") {
		parts := strings.SplitN(text, " ", 2)
		if len(parts) > 1 {
			return parts[1]
		}
		return ""
	}
	return text
}

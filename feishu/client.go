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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	baseURL    = "https://open.feishu.cn"
	apiVersion = "open-apis"
)

// Client is the Feishu API client.
type Client struct {
	appID     string
	appSecret string
	http      *http.Client

	mu            sync.RWMutex
	token         string
	tokenExpireAt time.Time
}

// NewClient creates a new Feishu API client.
func NewClient(appID, appSecret string) *Client {
	return &Client{
		appID:     appID,
		appSecret: appSecret,
		http: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// tokenResponse represents the response from the tenant access token API.
type tokenResponse struct {
	Code              int    `json:"code"`
	Msg               string `json:"msg"`
	TenantAccessToken string `json:"tenant_access_token"`
	Expire            int    `json:"expire"`
}

// GetTenantAccessToken obtains a tenant access token from Feishu.
func (c *Client) GetTenantAccessToken() (string, error) {
	c.mu.RLock()
	if c.token != "" && time.Now().Before(c.tokenExpireAt) {
		token := c.token
		c.mu.RUnlock()
		return token, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if c.token != "" && time.Now().Before(c.tokenExpireAt) {
		return c.token, nil
	}

	body := map[string]string{
		"app_id":     c.appID,
		"app_secret": c.appSecret,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("cannot marshal token request: %w", err)
	}

	req, err := http.NewRequest("POST", baseURL+"/"+apiVersion+"/auth/v3/tenant_access_token/internal", bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("cannot create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	var tr tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return "", fmt.Errorf("cannot decode token response: %w", err)
	}

	if tr.Code != 0 {
		return "", fmt.Errorf("token API error: code=%d msg=%s", tr.Code, tr.Msg)
	}

	c.token = tr.TenantAccessToken
	c.tokenExpireAt = time.Now().Add(time.Duration(tr.Expire-60) * time.Second) // Refresh 60s early

	return c.token, nil
}

// SendMessage sends a text message to a chat.
func (c *Client) SendMessage(chatID, text string) error {
	token, err := c.GetTenantAccessToken()
	if err != nil {
		return fmt.Errorf("cannot get token: %w", err)
	}

	body := map[string]interface{}{
		"receive_id": chatID,
		"msg_type":   "text",
		"content":    fmt.Sprintf(`{"text":"%s"}`, escapeJSON(text)),
	}
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("cannot marshal message: %w", err)
	}

	req, err := http.NewRequest("POST",
		baseURL+"/"+apiVersion+"/im/v1/messages?receive_id_type=chat_id",
		bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("cannot create message request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("send message failed: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("cannot decode send message response: %w", err)
	}

	if result.Code != 0 {
		return fmt.Errorf("send message API error: code=%d msg=%s", result.Code, result.Msg)
	}

	return nil
}

// DownloadFile downloads a file from Feishu and saves it to the specified directory.
// Returns the local file path.
func (c *Client) DownloadFile(fileKey, saveDir string) (string, error) {
	token, err := c.GetTenantAccessToken()
	if err != nil {
		return "", fmt.Errorf("cannot get token: %w", err)
	}

	// Get file metadata first
	req, err := http.NewRequest("GET",
		baseURL+"/"+apiVersion+"/im/v1/messages/"+fileKey+"/resources/"+fileKey+"?type=file",
		nil)
	if err != nil {
		return "", fmt.Errorf("cannot create file request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("download file request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download file failed with status: %d", resp.StatusCode)
	}

	// Determine filename from Content-Disposition header or use fileKey
	filename := fileKey
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		// Parse filename from Content-Disposition
		if _, err := fmt.Sscanf(cd, "attachment; filename=%s", &filename); err != nil {
			filename = fileKey
		}
	}

	// Ensure save directory exists
	if err := os.MkdirAll(saveDir, 0755); err != nil {
		return "", fmt.Errorf("cannot create upload directory: %w", err)
	}

	localPath := filepath.Join(saveDir, filename)

	// Write file
	out, err := os.Create(localPath)
	if err != nil {
		return "", fmt.Errorf("cannot create local file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return "", fmt.Errorf("cannot write file: %w", err)
	}

	return localPath, nil
}

// GetMessageResource downloads a message resource (image, file) from Feishu.
func (c *Client) GetMessageResource(messageID, fileKey, saveDir string) (string, error) {
	token, err := c.GetTenantAccessToken()
	if err != nil {
		return "", fmt.Errorf("cannot get token: %w", err)
	}

	req, err := http.NewRequest("GET",
		baseURL+"/"+apiVersion+"/im/v1/messages/"+messageID+"/resources/"+fileKey,
		nil)
	if err != nil {
		return "", fmt.Errorf("cannot create resource request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("get resource request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("get resource failed with status: %d", resp.StatusCode)
	}

	if err := os.MkdirAll(saveDir, 0755); err != nil {
		return "", fmt.Errorf("cannot create upload directory: %w", err)
	}

	localPath := filepath.Join(saveDir, fileKey)
	out, err := os.Create(localPath)
	if err != nil {
		return "", fmt.Errorf("cannot create local file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return "", fmt.Errorf("cannot write resource: %w", err)
	}

	return localPath, nil
}

// escapeJSON escapes special characters for JSON string content.
func escapeJSON(s string) string {
	var buf bytes.Buffer
	for _, r := range s {
		switch r {
		case '"':
			buf.WriteString(`\"`)
		case '\\':
			buf.WriteString(`\\`)
		case '\n':
			buf.WriteString(`\n`)
		case '\r':
			buf.WriteString(`\r`)
		case '\t':
			buf.WriteString(`\t`)
		default:
			buf.WriteRune(r)
		}
	}
	return buf.String()
}

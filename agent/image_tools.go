// Author: L.Shuang
// Created: 2026-05-01
// Last Modified: 2026-05-01
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
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/idirect3d/co-shell/llm"
	"github.com/idirect3d/co-shell/log"
)

// VisualAnalysisTool adds image file paths to the cache and flags them for
// one-shot delivery on the next LLM call. After the LLM processes the images,
// the flag is cleared automatically so images are not sent again.
func (a *Agent) VisualAnalysisTool(paths string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	newPaths := strings.Split(paths, ",")
	added := 0
	for _, p := range newPaths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// Check if already in cache
		exists := false
		for _, existing := range a.imagePaths {
			if existing == p {
				exists = true
				break
			}
		}
		if !exists {
			a.imagePaths = append(a.imagePaths, p)
			added++
		}
	}

	return fmt.Sprintf(
		"✅ 已添加 %d 张图片到缓存（当前共 %d 张，总数不应超过5张）\n\n"+
			"已加载图片:\n%s\n\n"+
			"📌 请立即利用多模态视觉能力，根据识别意图逐张分析图片内容，将识别结果以结构化方式呈现。\n"+
			"   注意：此工具调用是一次性的，图片仅发送一次，不会在后续对话中保留。如需再次分析其他图片，请重新调用此工具。",
		added, len(a.imagePaths), listImagesForPrompt(a.imagePaths)), nil
}
func listImagesForPrompt(paths []string) string {
	if len(paths) == 0 {
		return "（无）"
	}
	var sb strings.Builder
	for i, p := range paths {
		sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, p))
	}
	return sb.String()
}

// buildMultimodalMessage creates a Message with multimodal content from text and image paths.
// Images are read from disk and encoded as base64 data URIs.
func (a *Agent) buildMultimodalMessage(text string, imagePaths []string) (llm.Message, error) {
	parts := make([]llm.ContentPart, 0, 1+len(imagePaths))

	// Add text part (only if not empty)
	if text != "" {
		parts = append(parts, llm.ContentPart{
			Type: llm.ContentPartText,
			Text: text,
		})
	}

	// Add image parts
	for _, imgPath := range imagePaths {
		// Resolve relative paths
		absPath := imgPath
		if !filepath.IsAbs(imgPath) {
			cwd, err := os.Getwd()
			if err != nil {
				return llm.Message{}, fmt.Errorf("cannot get current working directory: %w", err)
			}
			absPath = filepath.Join(cwd, imgPath)
		}

		// Read image file
		data, err := os.ReadFile(absPath)
		if err != nil {
			return llm.Message{}, fmt.Errorf("cannot read image %q: %w", imgPath, err)
		}

		// Detect MIME type from extension
		ext := strings.ToLower(filepath.Ext(absPath))
		mimeType := ""
		switch ext {
		case ".png":
			mimeType = "image/png"
		case ".jpg", ".jpeg":
			mimeType = "image/jpeg"
		case ".gif":
			mimeType = "image/gif"
		case ".webp":
			mimeType = "image/webp"
		case ".bmp":
			mimeType = "image/bmp"
		default:
			mimeType = "image/png" // default fallback
		}

		// Encode as base64 data URI
		base64Data := base64.StdEncoding.EncodeToString(data)
		dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data)

		parts = append(parts, llm.ContentPart{
			Type: llm.ContentPartImageURL,
			ImageURL: &llm.ContentPartImage{
				URL:    dataURI,
				Detail: "auto",
			},
		})
	}

	return llm.Message{
		Role:         "user",
		Content:      text,
		ContentParts: parts,
	}, nil
}

// visualAnalysisTool adds image paths to cache for one-shot delivery.
func (a *Agent) visualAnalysisTool(ctx context.Context, args map[string]interface{}) (string, error) {
	log.Debug("visualAnalysisTool called: args=%v", args)
	pathsStr, ok := args["paths"].(string)
	if !ok {
		return "", fmt.Errorf("paths argument is required")
	}

	// Extract intent parameter (required)
	intent, _ := args["intent"].(string)
	if intent == "" {
		return "", fmt.Errorf("intent argument is required — you must specify what information you need to analyze from the visual input")
	}

	// Split by comma and trim spaces
	newPaths := strings.Split(pathsStr, ",")
	added := 0
	for _, p := range newPaths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// Check if already in cache
		exists := false
		for _, existing := range a.imagePaths {
			if existing == p {
				exists = true
				break
			}
		}
		if !exists {
			a.imagePaths = append(a.imagePaths, p)
			added++
		}
	}

	return fmt.Sprintf(
		"✅ 已添加 %d 张图片到缓存（当前共 %d 张，总数不应超过5张）\n\n"+
			"已加载图片:\n%s\n\n"+
			"🎯 识别意图：%s\n\n"+
			"📌 请立即利用多模态视觉能力，根据识别意图逐张分析图片内容，将识别结果以结构化方式呈现。\n"+
			"   注意：此调用是一次性的，图片仅发送一次，发送后自动清理缓存，不会在后续对话中保留。",
		added, len(a.imagePaths), listImagesForPrompt(a.imagePaths), intent), nil
}

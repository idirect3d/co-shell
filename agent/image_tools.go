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

// VisualAnalysisTool adds an image file path to the cache and flags it for
// one-shot delivery on the next LLM call. After the LLM processes the image,
// the flag is cleared automatically so the image is not sent again.
func (a *Agent) VisualAnalysisTool(path string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}

	// Check if the file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Try resolving relative to workspace
		absPath := path
		if !filepath.IsAbs(path) {
			cwd, _ := os.Getwd()
			absPath = filepath.Join(cwd, path)
		}
		if _, err2 := os.Stat(absPath); os.IsNotExist(err2) {
			return "", fmt.Errorf("file not found: %s", path)
		}
	}

	// Check if already in cache
	for _, existing := range a.imagePaths {
		if existing == path {
			return fmt.Sprintf("✅ 图片已在缓存中: %s", path), nil
		}
	}

	a.imagePaths = append(a.imagePaths, path)

	return fmt.Sprintf("请根据以下意图分析已上传视觉文件（%s）的内容，并将分析结果描述出来，如果内容较多，建议及时将识别结果保存到同名 .md 文件中供后续使用（替换扩展名为 .md）。\n", path), nil
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

// visualAnalysisTool adds a single image path to cache for one-shot delivery.
func (a *Agent) visualAnalysisTool(ctx context.Context, args map[string]interface{}) (string, error) {
	log.Debug("visualAnalysisTool called: args=%v", args)
	path, ok := args["path"].(string)
	if !ok || strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("path argument is required — provide a single image/video file path")
	}
	path = strings.TrimSpace(path)

	// Extract intent parameter (required)
	intent, _ := args["intent"].(string)
	if intent == "" {
		return "", fmt.Errorf("intent argument is required — you must specify what information you need to analyze from the visual input")
	}

	// Check if already in cache
	a.mu.Lock()
	for _, existing := range a.imagePaths {
		if existing == path {
			a.mu.Unlock()
			return fmt.Sprintf("✅ 图片已在缓存中: %s", path), nil
		}
	}

	a.imagePaths = append(a.imagePaths, path)
	a.mu.Unlock()

	// Store the analysis instruction in the task instruction cache so it will be
	// flushed as a <task> ContentPart at the end of the iteration. This keeps the
	// instruction separate from the tool result and makes it visible as a user task.
	taskContent := fmt.Sprintf(
		"分析视觉文件: %s\n\n识别意图: %s\n\n请根据以上意图分析已上传视觉文件的内容，并通过调用 write_to_file（新建）/replace_in_file（追加） 将分析结果立即保存到 .md 文件中供后续使用，否则识别的信息将会丢失！",
		path, intent)
	if a.taskInstructionCache.Len() > 0 {
		a.taskInstructionCache.WriteString("\n\n")
	}
	a.taskInstructionCache.WriteString(taskContent)

	return "视觉文件已加载，识别指令将以 <task> 形式在末尾提供。", nil
}

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

// detectMediaType returns the MIME type and content part type for a given file extension.
func detectMediaType(ext string) (mimeType string, partType llm.ContentPartType) {
	switch ext {
	case ".png":
		return "image/png", llm.ContentPartImageURL
	case ".jpg", ".jpeg":
		return "image/jpeg", llm.ContentPartImageURL
	case ".gif":
		return "image/gif", llm.ContentPartImageURL
	case ".webp":
		return "image/webp", llm.ContentPartImageURL
	case ".bmp":
		return "image/bmp", llm.ContentPartImageURL
	case ".mp4":
		return "video/mp4", llm.ContentPartVideoURL
	case ".mov":
		return "video/quicktime", llm.ContentPartVideoURL
	case ".avi":
		return "video/x-msvideo", llm.ContentPartVideoURL
	case ".mkv":
		return "video/x-matroska", llm.ContentPartVideoURL
	case ".webm":
		return "video/webm", llm.ContentPartVideoURL
	default:
		return "image/png", llm.ContentPartImageURL // default fallback
	}
}

// buildMultimodalMessage creates a Message with multimodal content from text and media file paths.
// Images and videos are read from disk and encoded as base64 data URIs.
func (a *Agent) buildMultimodalMessage(text string, mediaPaths []string) (llm.Message, error) {
	parts := make([]llm.ContentPart, 0, 1+len(mediaPaths))

	// Add text part (only if not empty)
	if text != "" {
		parts = append(parts, llm.ContentPart{
			Type: llm.ContentPartText,
			Text: text,
		})
	}

	// Add media parts (images and videos)
	for _, mediaPath := range mediaPaths {
		// Resolve relative paths
		absPath := mediaPath
		if !filepath.IsAbs(mediaPath) {
			cwd, err := os.Getwd()
			if err != nil {
				return llm.Message{}, fmt.Errorf("cannot get current working directory: %w", err)
			}
			absPath = filepath.Join(cwd, mediaPath)
		}

		// Read file
		data, err := os.ReadFile(absPath)
		if err != nil {
			return llm.Message{}, fmt.Errorf("cannot read file %q: %w", mediaPath, err)
		}

		// Detect MIME type and content part type from extension
		ext := strings.ToLower(filepath.Ext(absPath))
		mimeType, partType := detectMediaType(ext)

		// Encode as base64 data URI
		base64Data := base64.StdEncoding.EncodeToString(data)
		dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, base64Data)

		if partType == llm.ContentPartVideoURL {
			parts = append(parts, llm.ContentPart{
				Type: llm.ContentPartVideoURL,
				VideoURL: &llm.ContentPartVideo{
					URL: dataURI,
				},
			})
		} else {
			parts = append(parts, llm.ContentPart{
				Type: llm.ContentPartImageURL,
				ImageURL: &llm.ContentPartImage{
					URL:    dataURI,
					Detail: "auto",
				},
			})
		}
	}

	return llm.Message{
		Role:         "user",
		Content:      text,
		ContentParts: parts,
	}, nil
}

// visualAnalysisTool adds one or more image paths to cache for one-shot delivery.
func (a *Agent) visualAnalysisTool(ctx context.Context, args map[string]interface{}) (string, error) {
	log.Debug("visualAnalysisTool called: args=%v", args)

	// Extract paths array parameter
	var paths []string
	if rawPaths, ok := args["paths"].([]interface{}); ok {
		for _, raw := range rawPaths {
			if p, ok := raw.(string); ok && strings.TrimSpace(p) != "" {
				paths = append(paths, strings.TrimSpace(p))
			}
		}
	}
	// Fallback: single path (backward compatibility)
	if len(paths) == 0 {
		if p, ok := args["path"].(string); ok && strings.TrimSpace(p) != "" {
			paths = append(paths, strings.TrimSpace(p))
		}
	}
	if len(paths) == 0 {
		return "", fmt.Errorf("paths argument is required — provide at least one image/video file path")
	}

	// Extract intent parameter (required)
	intent, _ := args["intent"].(string)
	if intent == "" {
		return "", fmt.Errorf("intent argument is required — you must specify what information you need to analyze from the visual input")
	}

	// Get max images limit from config
	maxImages := 5
	if a.cfg != nil && a.cfg.LLM.VisualAnalysisMaxImages > 0 {
		maxImages = a.cfg.LLM.VisualAnalysisMaxImages
	}

	// Truncate if exceeds max
	truncated := 0
	if len(paths) > maxImages {
		truncated = len(paths) - maxImages
		paths = paths[:maxImages]
	}

	a.mu.Lock()
	loadedFiles := make([]string, 0, len(paths))
	for _, p := range paths {
		// Check if already in cache
		alreadyCached := false
		for _, existing := range a.imagePaths {
			if existing == p {
				alreadyCached = true
				break
			}
		}
		if alreadyCached {
			continue
		}
		// Check if the file exists
		if _, err := os.Stat(p); os.IsNotExist(err) {
			absPath := p
			if !filepath.IsAbs(p) {
				cwd, _ := os.Getwd()
				absPath = filepath.Join(cwd, p)
			}
			if _, err2 := os.Stat(absPath); os.IsNotExist(err2) {
				continue // skip non-existent files
			}
		}
		a.imagePaths = append(a.imagePaths, p)
		loadedFiles = append(loadedFiles, p)
	}
	a.mu.Unlock()

	// Build task content with all loaded files
	var fileList strings.Builder
	for i, fp := range loadedFiles {
		if i > 0 {
			fileList.WriteString("\n")
		}
		fileList.WriteString(fmt.Sprintf("  %d. %s", i+1, fp))
	}

	taskContent := fmt.Sprintf(
		"分析以下视觉文件:\n%s\n\n识别意图: %s\n\n请根据以上意图分析已上传视觉文件的内容，注意：必须通过调用 write_to_file（新建）/replace_in_file（追加） 将分析结果立即保存到 .md 文件中供后续使用，否则识别的信息将会丢失！",
		fileList.String(), intent)

	if truncated > 0 {
		taskContent += fmt.Sprintf("\n\n⚠️ 已截断 %d 个文件（超过上限 %d 个），如需分析更多文件请再次调用 visual_analysis。", truncated, maxImages)
	}

	if a.taskInstructionCache.Len() > 0 {
		a.taskInstructionCache.WriteString("\n\n")
	}
	a.taskInstructionCache.WriteString(taskContent)

	result := fmt.Sprintf("✅ 已加载 %d 个视觉文件", len(loadedFiles))
	if truncated > 0 {
		result += fmt.Sprintf("（截断 %d 个，上限 %d）", truncated, maxImages)
	}
	result += "，识别指令将以 <task> 形式在末尾提供。"
	return result, nil
}

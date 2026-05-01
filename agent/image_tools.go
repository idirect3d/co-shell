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

// AddImages adds image file paths to the image cache.
// paths is a comma-separated list of image file paths.
func (a *Agent) AddImages(paths string) (string, error) {
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

	return fmt.Sprintf("✅ 已添加 %d 张图片到缓存（当前共 %d 张）", added, len(a.imagePaths)), nil
}

// RemoveImages removes image file paths from the image cache.
// paths is a comma-separated list of image file paths.
func (a *Agent) RemoveImages(paths string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	removePaths := strings.Split(paths, ",")
	removed := 0
	var remaining []string
	for _, p := range a.imagePaths {
		shouldRemove := false
		for _, rp := range removePaths {
			if p == strings.TrimSpace(rp) {
				shouldRemove = true
				break
			}
		}
		if shouldRemove {
			removed++
		} else {
			remaining = append(remaining, p)
		}
	}
	a.imagePaths = remaining

	return fmt.Sprintf("✅ 已从缓存中移除 %d 张图片（当前共 %d 张）", removed, len(a.imagePaths)), nil
}

// ClearImages clears all cached image file paths.
func (a *Agent) ClearImages() (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	count := len(a.imagePaths)
	a.imagePaths = nil
	return fmt.Sprintf("✅ 已清空图片缓存（共移除 %d 张图片）", count), nil
}

// ListImages returns a formatted list of all cached image file paths.
func (a *Agent) ListImages() (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if len(a.imagePaths) == 0 {
		return "📷 图片缓存为空", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("📷 图片缓存（共 %d 张）:\n", len(a.imagePaths)))
	for i, p := range a.imagePaths {
		sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, p))
	}
	return sb.String(), nil
}

// buildMultimodalMessage creates a Message with multimodal content from text and image paths.
// Images are read from disk and encoded as base64 data URIs.
func (a *Agent) buildMultimodalMessage(text string, imagePaths []string) (llm.Message, error) {
	parts := make([]llm.ContentPart, 0, 1+len(imagePaths))

	// Add text part
	parts = append(parts, llm.ContentPart{
		Type: llm.ContentPartText,
		Text: text,
	})

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

// addImagesTool adds image file paths to the image cache.
func (a *Agent) addImagesTool(ctx context.Context, args map[string]interface{}) (string, error) {
	log.Debug("addImagesTool called: args=%v", args)
	pathsStr, ok := args["paths"].(string)
	if !ok {
		return "", fmt.Errorf("paths argument is required")
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

	return fmt.Sprintf("✅ 已添加 %d 张图片到缓存（当前共 %d 张）", added, len(a.imagePaths)), nil
}

// removeImagesTool removes image file paths from the image cache.
func (a *Agent) removeImagesTool(ctx context.Context, args map[string]interface{}) (string, error) {
	log.Debug("removeImagesTool called: args=%v", args)
	pathsStr, ok := args["paths"].(string)
	if !ok {
		return "", fmt.Errorf("paths argument is required")
	}

	// Split by comma and trim spaces
	removePaths := strings.Split(pathsStr, ",")
	removed := 0
	var remaining []string
	for _, p := range a.imagePaths {
		shouldRemove := false
		for _, rp := range removePaths {
			if p == strings.TrimSpace(rp) {
				shouldRemove = true
				break
			}
		}
		if shouldRemove {
			removed++
		} else {
			remaining = append(remaining, p)
		}
	}
	a.imagePaths = remaining

	return fmt.Sprintf("✅ 已从缓存中移除 %d 张图片（当前共 %d 张）", removed, len(a.imagePaths)), nil
}

// clearImagesTool clears all cached image file paths.
func (a *Agent) clearImagesTool(ctx context.Context, args map[string]interface{}) (string, error) {
	log.Debug("clearImagesTool called: args=%v", args)
	count := len(a.imagePaths)
	a.imagePaths = nil
	return fmt.Sprintf("✅ 已清空图片缓存（共移除 %d 张图片）", count), nil
}

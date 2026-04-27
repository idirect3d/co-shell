// Author: L.Shuang
// Created: 2026-04-28
// Last Modified: 2026-04-28
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

package cmd

import (
	"fmt"
	"strings"

	"github.com/idirect3d/co-shell/agent"
)

// ImageHandler handles the .image built-in command.
type ImageHandler struct {
	agent *agent.Agent
}

// NewImageHandler creates a new ImageHandler.
func NewImageHandler(ag *agent.Agent) *ImageHandler {
	return &ImageHandler{agent: ag}
}

// Handle processes .image commands.
func (h *ImageHandler) Handle(args []string) (string, error) {
	if len(args) == 0 {
		return showImageHelp(), nil
	}

	subcommand := args[0]
	switch subcommand {
	case "add":
		if len(args) < 2 {
			return "", fmt.Errorf("用法: .image add <path1,path2,...>")
		}
		paths := strings.Join(args[1:], " ")
		return h.agent.AddImages(paths)

	case "remove":
		if len(args) < 2 {
			return "", fmt.Errorf("用法: .image remove <path1,path2,...>")
		}
		paths := strings.Join(args[1:], " ")
		return h.agent.RemoveImages(paths)

	case "clear":
		return h.agent.ClearImages()

	case "list":
		return h.agent.ListImages()

	default:
		return "", fmt.Errorf("未知的 .image 子命令: %s（可用: add, remove, clear, list）", subcommand)
	}
}

// showImageHelp displays the .image command usage.
func showImageHelp() string {
	return `📷 图片缓存管理 (.image)

  .image add <path1,path2,...>    添加图片到缓存
  .image remove <path1,path2,...> 从缓存中移除图片
  .image clear                    清空图片缓存
  .image list                     列出当前缓存的图片

示例:
  .image add screenshot.png
  .image add photo1.jpg,photo2.png
  .image remove photo1.jpg
  .image clear
  .image list`
}

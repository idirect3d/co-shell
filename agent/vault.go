// Author: L.Shuang
// Created: 2026-07-06
// Last Modified: 2026-07-06
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
// IMPLIED, BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/idirect3d/co-shell/i18n"
	"github.com/idirect3d/co-shell/store"
)

// vaultListTool lists all vault entry names (no sensitive data).
func (a *Agent) vaultListTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if a.vaultStore == nil {
		return "", fmt.Errorf("vault not initialized")
	}

	names, err := a.vaultStore.List()
	if err != nil {
		return "", fmt.Errorf("cannot list vault entries: %w", err)
	}

	if len(names) == 0 {
		return "vault: no entries", nil
	}

	var b strings.Builder
	if len(names) == 0 {
		b.WriteString("vault: no entries\n\n")
	} else {
		b.WriteString("vault entries:\n")
		for _, name := range names {
			entry, err := a.vaultStore.Get(name)
			if err == nil && len(entry.Tags) > 0 {
				tagList := make([]string, 0, len(entry.Tags))
				for tag := range entry.Tags {
					tagList = append(tagList, tag)
				}
				b.WriteString(fmt.Sprintf("  - %s [tags: %s]\n", name, strings.Join(tagList, ", ")))
			} else {
				b.WriteString(fmt.Sprintf("  - %s\n", name))
			}
		}
		b.WriteString(fmt.Sprintf("\ntotal: %d entries", len(names)))
	}
	// Hint about using placeholders even when entries don't exist yet
	b.WriteString("\n提示：在任何工具调用中使用 @Tag:条目名@ 格式引用密码本。若条目不存在，系统也会在确认后提示输入。")
	return b.String(), nil
}

// vaultAddTool adds a new vault entry.
// The LLM only provides name/url/notes; username and password are collected
// interactively from the user to avoid exposing them to the LLM.
func (a *Agent) vaultAddTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if a.vaultStore == nil {
		return "", fmt.Errorf("vault not initialized")
	}
	if !a.vaultStore.IsUnlocked() {
		return "", fmt.Errorf("vault is locked, use :vault unlock first")
	}

	name, _ := args["name"].(string)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}
	notes, _ := args["notes"].(string)

	// Check if entry already exists
	existing, _ := a.vaultStore.Get(name)
	if existing != nil {
		return "", fmt.Errorf("entry %q already exists, use vault_remove first to replace", name)
	}

	// Prompt for tags interactively (enter tag:value pairs, one per line)
	io := a.defaultIO()
	io.Println()
	io.Println(i18n.TF(i18n.KeyVaultAddPrompt, name))
	io.Println("  请输入标签值对（格式: tag=value），每行一个，空行结束")
	io.Println("  例如: user=myuser, pwd=mypass, key=xxx, token=xxx, email=xxx, ip_addr=1.2.3.4")

	tags := make(map[string]string)
	for {
		io.Printf("  tag=value: ")
		line, err := io.ReadLine()
		if err != nil {
			return "", fmt.Errorf("failed to read input: %w", err)
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			io.Println("  格式错误，应为 tag=value")
			continue
		}
		tagName := strings.TrimSpace(parts[0])
		tagValue := strings.TrimSpace(parts[1])
		if tagName == "" || tagValue == "" {
			io.Println("  tag 和 value 都不能为空")
			continue
		}
		tags[tagName] = tagValue
	}

	if len(tags) == 0 {
		return "", fmt.Errorf("at least one tag is required")
	}

	entry := &store.VaultEntry{
		Name:  name,
		Tags:  tags,
		Notes: notes,
	}

	if err := a.vaultStore.Put(entry); err != nil {
		return "", fmt.Errorf("cannot save vault entry: %w", err)
	}

	io.Println(i18n.TF(i18n.KeyVaultAdded, name))
	return fmt.Sprintf("vault entry %q has been added", name), nil
}

// vaultRemoveTool removes a vault entry.
func (a *Agent) vaultRemoveTool(ctx context.Context, args map[string]interface{}) (string, error) {
	if a.vaultStore == nil {
		return "", fmt.Errorf("vault not initialized")
	}
	if !a.vaultStore.IsUnlocked() {
		return "", fmt.Errorf("vault is locked, use :vault unlock first")
	}

	name, _ := args["name"].(string)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}

	// Check existence
	_, err := a.vaultStore.Get(name)
	if err != nil {
		return "", fmt.Errorf("entry %q not found", name)
	}

	if err := a.vaultStore.Delete(name); err != nil {
		return "", fmt.Errorf("cannot delete vault entry: %w", err)
	}

	return fmt.Sprintf("vault entry %q has been removed", name), nil
}

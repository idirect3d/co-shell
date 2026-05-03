// Author: L.Shuang
// Created: 2026-04-25
// Last Modified: 2026-04-26
//
// # MIT License
//
// # Copyright (c) 2026 L.Shuang
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
package wizard

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/idirect3d/co-shell/config"
)

// selectProvider presents a simple text-based menu for provider selection.
// Supports: Enter for default, number or name input.
func selectProvider() *config.ProviderPreset {
	providers := config.ProviderPresets

	fmt.Println("📌 请选择大模型供应商（直接回车使用默认）：")
	for i, p := range providers {
		fmt.Printf("  [%d] %s\n", i+1, p.DisplayName)
	}
	fmt.Println()

	for {
		fmt.Print("请选择 [1]: ")
		var input string
		fmt.Scanln(&input)
		input = strings.TrimSpace(input)

		if input == "" {
			return &providers[0]
		}

		// Try number
		if num, err := strconv.Atoi(input); err == nil {
			if num >= 1 && num <= len(providers) {
				return &providers[num-1]
			}
		}

		// Try name
		for _, p := range providers {
			if strings.EqualFold(input, p.Name) {
				return &p
			}
		}

		fmt.Printf("⚠️  无效选择，请输入 1-%d 之间的数字或供应商名称。\n", len(providers))
	}
}

// selectModel presents a numbered model selection with prefix matching support.
// Input a number to select by index, or type a prefix to match the first model.
func selectModel(models []string, defaultModel string) *string {
	fmt.Println("📌 请选择模型（直接回车使用默认，输入编号选择，输入前缀匹配）：")
	for i, m := range models {
		fmt.Printf("  [%d] %s\n", i+1, m)
	}
	fmt.Println()

	for {
		fmt.Printf("模型名称 [%s]: ", defaultModel)
		var input string
		fmt.Scanln(&input)
		input = strings.TrimSpace(input)

		if input == "" {
			return &defaultModel
		}

		// Try number input
		if num, err := strconv.Atoi(input); err == nil {
			if num >= 1 && num <= len(models) {
				return &models[num-1]
			}
			fmt.Printf("⚠️  无效编号，请输入 1-%d 之间的数字。\n", len(models))
			continue
		}

		// Try exact match
		for _, m := range models {
			if strings.EqualFold(input, m) {
				return &m
			}
		}

		// Try prefix match (case-insensitive)
		inputLower := strings.ToLower(input)
		for _, m := range models {
			if strings.HasPrefix(strings.ToLower(m), inputLower) {
				return &m
			}
		}

		// Allow custom model name input
		return &input
	}
}

// promptRequiredText prompts for required text input.
func promptRequiredText(prompt string, defaultValue string) *string {
	for {
		result := promptTextSimple(prompt, defaultValue, true)
		if result == nil {
			return nil
		}
		if *result != "" {
			return result
		}
		fmt.Println("⚠️  此项不能为空，请重新输入。")
	}
}

// promptOptionalText prompts for optional text input.
func promptOptionalText(prompt string, defaultValue string) *string {
	return promptTextSimple(prompt, defaultValue, false)
}

// promptTextSimple reads text input using standard fmt.Scanln.
func promptTextSimple(prompt string, defaultValue string, required bool) *string {
	promptStr := prompt
	if defaultValue != "" {
		promptStr = prompt + " [" + defaultValue + "]"
	}

	for {
		fmt.Print(promptStr + ": ")
		var input string
		fmt.Scanln(&input)
		input = strings.TrimSpace(input)

		if input == "" && defaultValue != "" {
			return &defaultValue
		}
		if input == "" && required {
			fmt.Println("⚠️  此项不能为空，请重新输入。")
			continue
		}
		return &input
	}
}

// promptAPIKey prompts for API Key input.
// If currentKey is not empty, it will be shown as default and returned on empty input.
func promptAPIKey(apiKeyURL string, currentKey string) *string {
	for {
		result := promptAPIKeySimple(apiKeyURL, currentKey)
		if result == nil {
			return nil
		}
		if *result != "" {
			return result
		}
		fmt.Println("⚠️  API Key 不能为空，请重新输入。")
	}
}

// promptAPIKeySimple reads API Key input using standard fmt.Scanln.
// If currentKey is not empty, it will be shown as default and returned on empty input.
func promptAPIKeySimple(apiKeyURL string, currentKey string) *string {
	for {
		promptStr := "📌 API Key"
		if currentKey != "" {
			masked := maskKey(currentKey)
			promptStr += " [" + masked + "]"
		} else {
			promptStr += " (必填)"
		}
		if apiKeyURL != "" {
			promptStr += " [输入 W 打开 " + apiKeyURL + " 获取 Key]"
		}
		fmt.Print(promptStr + ": ")

		var input string
		fmt.Scanln(&input)
		input = strings.TrimSpace(input)

		if input == "" && currentKey != "" {
			return &currentKey
		}

		if input == "" {
			fmt.Println("⚠️  API Key 不能为空，请重新输入。")
			continue
		}

		// Check if user wants to open the webpage
		if apiKeyURL != "" && strings.ToLower(input) == "w" {
			fmt.Printf("   🔗 正在打开: %s\n", apiKeyURL)
			if err := config.OpenURL(apiKeyURL); err != nil {
				fmt.Printf("   ⚠️  无法自动打开浏览器: %v\n", err)
				fmt.Printf("   请手动访问: %s\n", apiKeyURL)
			}
			fmt.Println()
			continue
		}

		return &input
	}
}

// maskKey masks the API key for display, showing first 4 and last 4 characters.
func maskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + "****" + key[len(key)-4:]
}

// promptConfirm asks a yes/no question. Returns true for yes.
func promptConfirm(prompt string, defaultYes bool) bool {
	yn := "Y/n"
	if !defaultYes {
		yn = "y/N"
	}

	fmt.Print(prompt + " [" + yn + "]: ")
	var input string
	fmt.Scanln(&input)
	input = strings.TrimSpace(strings.ToLower(input))

	if input == "" {
		return defaultYes
	}
	return input == "y" || input == "yes"
}

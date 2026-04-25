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

// selectModel presents a simple text-based model selection.
func selectModel(models []string, defaultModel string) *string {
	fmt.Println("📌 请选择模型（直接回车使用默认）：")
	for _, m := range models {
		fmt.Printf("  %s\n", m)
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

		// Check if input matches any model
		for _, m := range models {
			if strings.EqualFold(input, m) {
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
func promptAPIKey(apiKeyURL string) *string {
	for {
		result := promptAPIKeySimple(apiKeyURL)
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
func promptAPIKeySimple(apiKeyURL string) *string {
	for {
		if apiKeyURL != "" {
			fmt.Printf("📌 API Key (必填) [输入 W 打开 %s 获取 Key]: ", apiKeyURL)
		} else {
			fmt.Print("📌 API Key (必填): ")
		}

		var input string
		fmt.Scanln(&input)
		input = strings.TrimSpace(input)

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

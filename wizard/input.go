// Author: L.Shuang
// Created: 2026-04-25
// Last Modified: 2026-04-25
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
package wizard

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/idirect3d/co-shell/config"
)

// selectProvider presents an interactive menu for provider selection.
// Supports: Enter for default, Tab for list display with arrow key navigation,
// number/name input, ESC to cancel. Returns nil if user cancelled.
func selectProvider() *config.ProviderPreset {
	providers := config.ProviderPresets

	fmt.Println("📌 请选择大模型供应商（直接回车使用默认，或按 Tab 键显示可选列表）：")
	fmt.Print("请选择 [1]: ")

	oldState, err := rawTerminal()
	if err != nil {
		restoreTerminal(nil)
		return selectProviderSimple(providers)
	}

	result := interactiveSelectProvider(providers, oldState)
	return result
}

// interactiveSelectProvider handles the provider selection with Tab-triggered list.
func interactiveSelectProvider(providers []config.ProviderPreset, oldState *terminalState) *config.ProviderPreset {
	var input strings.Builder
	showList := false
	selectedIdx := 0

	hideCursor()
	defer showCursor()

	for {
		key, isSpecial := readKey()

		if isSpecial {
			switch key {
			case "esc":
				clearLine()
				restoreTerminal(oldState)
				return nil

			case "tab":
				if !showList {
					showList = true
					selectedIdx = 0
					// Print the list below current line
					fmt.Println()
					for i, p := range providers {
						if i == selectedIdx {
							fmt.Printf("  \033[7m[%d] %s\033[0m\n", i+1, p.DisplayName)
						} else {
							fmt.Printf("  [%d] %s\n", i+1, p.DisplayName)
						}
					}
				} else {
					showList = false
					// Clear the list
					for i := 0; i < len(providers); i++ {
						moveUp(1)
						clearLine()
					}
				}
				// Reprint prompt
				clearLine()
				fmt.Print("请选择 [1]: " + input.String())

			case "up":
				if showList && selectedIdx > 0 {
					selectedIdx--
					// Update the list display
					moveUp(len(providers) - selectedIdx - 1)
					for i := selectedIdx; i < len(providers); i++ {
						clearLine()
						if i == selectedIdx {
							fmt.Printf("  \033[7m[%d] %s\033[0m\n", i+1, providers[i].DisplayName)
						} else {
							fmt.Printf("  [%d] %s\n", i+1, providers[i].DisplayName)
						}
						if i < len(providers)-1 {
							moveDown(1)
						}
					}
				}

			case "down":
				if showList && selectedIdx < len(providers)-1 {
					selectedIdx++
					// Update the list display
					moveUp(len(providers) - selectedIdx)
					for i := selectedIdx - 1; i < len(providers); i++ {
						clearLine()
						if i == selectedIdx {
							fmt.Printf("  \033[7m[%d] %s\033[0m\n", i+1, providers[i].DisplayName)
						} else {
							fmt.Printf("  [%d] %s\n", i+1, providers[i].DisplayName)
						}
						if i < len(providers)-1 {
							moveDown(1)
						}
					}
				}

			case "enter":
				if showList {
					// Select highlighted item
					restoreTerminal(oldState)
					// Clear the list
					for i := 0; i < len(providers); i++ {
						moveUp(1)
						clearLine()
					}
					clearLine()
					fmt.Printf("请选择 [1]: %d\n", selectedIdx+1)
					return &providers[selectedIdx]
				}

				// Enter with text input
				text := strings.TrimSpace(input.String())
				restoreTerminal(oldState)
				clearLine()

				if text == "" {
					fmt.Printf("请选择 [1]: 1\n")
					return &providers[0]
				}

				// Try number
				if num, err := strconv.Atoi(text); err == nil {
					if num >= 1 && num <= len(providers) {
						fmt.Printf("请选择 [1]: %d\n", num)
						return &providers[num-1]
					}
				}

				// Try name
				for _, p := range providers {
					if strings.EqualFold(text, p.Name) {
						fmt.Printf("请选择 [1]: %s\n", text)
						return &p
					}
				}

				fmt.Printf("请选择 [1]: %s\n", text)
				fmt.Printf("⚠️  无效选择，请输入 1-%d 之间的数字或供应商名称。\n", len(providers))
				input.Reset()
				showList = false
				fmt.Print("请选择 [1]: ")

			case "backspace":
				if input.Len() > 0 {
					s := input.String()
					input.Reset()
					input.WriteString(s[:len(s)-1])
					clearLine()
					fmt.Print("请选择 [1]: " + input.String())
				}
			}
		} else {
			// Regular character
			input.WriteString(key)
			clearLine()
			fmt.Print("请选择 [1]: " + input.String())
		}
	}
}

// selectProviderSimple is a fallback for when raw terminal is not available.
func selectProviderSimple(providers []config.ProviderPreset) *config.ProviderPreset {
	fmt.Println()
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

		if num, err := strconv.Atoi(input); err == nil {
			if num >= 1 && num <= len(providers) {
				return &providers[num-1]
			}
		}

		for _, p := range providers {
			if strings.EqualFold(input, p.Name) {
				return &p
			}
		}

		fmt.Printf("⚠️  无效选择，请输入 1-%d 之间的数字或供应商名称。\n", len(providers))
	}
}

// selectModel presents an interactive model selection with Tab support.
func selectModel(models []string, defaultModel string) *string {
	fmt.Println("📌 请选择模型（直接回车使用默认，或按 Tab 键显示可选列表）：")
	fmt.Printf("模型名称 [%s]: ", defaultModel)

	oldState, err := rawTerminal()
	if err != nil {
		restoreTerminal(nil)
		return selectModelSimple(models, defaultModel)
	}

	result := interactiveSelectModel(models, defaultModel, oldState)
	return result
}

// interactiveSelectModel handles Tab-triggered model list selection.
func interactiveSelectModel(models []string, defaultModel string, oldState *terminalState) *string {
	var input strings.Builder
	showList := false
	selectedIdx := 0

	hideCursor()
	defer showCursor()

	for {
		key, isSpecial := readKey()

		if isSpecial {
			switch key {
			case "esc":
				clearLine()
				restoreTerminal(oldState)
				return nil

			case "tab":
				if !showList {
					showList = true
					selectedIdx = 0
					fmt.Println()
					for i, m := range models {
						if i == selectedIdx {
							fmt.Printf("  \033[7m%s\033[0m\n", m)
						} else {
							fmt.Printf("  %s\n", m)
						}
					}
				} else {
					showList = false
					for i := 0; i < len(models); i++ {
						moveUp(1)
						clearLine()
					}
				}
				clearLine()
				fmt.Print("模型名称 [" + defaultModel + "]: " + input.String())

			case "up":
				if showList && selectedIdx > 0 {
					selectedIdx--
					moveUp(len(models) - selectedIdx - 1)
					for i := selectedIdx; i < len(models); i++ {
						clearLine()
						if i == selectedIdx {
							fmt.Printf("  \033[7m%s\033[0m\n", models[i])
						} else {
							fmt.Printf("  %s\n", models[i])
						}
						if i < len(models)-1 {
							moveDown(1)
						}
					}
				}

			case "down":
				if showList && selectedIdx < len(models)-1 {
					selectedIdx++
					moveUp(len(models) - selectedIdx)
					for i := selectedIdx - 1; i < len(models); i++ {
						clearLine()
						if i == selectedIdx {
							fmt.Printf("  \033[7m%s\033[0m\n", models[i])
						} else {
							fmt.Printf("  %s\n", models[i])
						}
						if i < len(models)-1 {
							moveDown(1)
						}
					}
				}

			case "enter":
				if showList {
					restoreTerminal(oldState)
					for i := 0; i < len(models); i++ {
						moveUp(1)
						clearLine()
					}
					clearLine()
					fmt.Printf("模型名称 [%s]: %s\n", defaultModel, models[selectedIdx])
					return &models[selectedIdx]
				}

				text := strings.TrimSpace(input.String())
				restoreTerminal(oldState)
				clearLine()

				if text == "" {
					fmt.Printf("模型名称 [%s]: %s\n", defaultModel, defaultModel)
					return &defaultModel
				}

				fmt.Printf("模型名称 [%s]: %s\n", defaultModel, text)
				return &text

			case "backspace":
				if input.Len() > 0 {
					s := input.String()
					input.Reset()
					input.WriteString(s[:len(s)-1])
					clearLine()
					fmt.Print("模型名称 [" + defaultModel + "]: " + input.String())
				}
			}
		} else {
			input.WriteString(key)
			clearLine()
			fmt.Print("模型名称 [" + defaultModel + "]: " + input.String())
		}
	}
}

// selectModelSimple is a fallback for model selection.
func selectModelSimple(models []string, defaultModel string) *string {
	fmt.Println()
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
		return &input
	}
}

// promptRequiredText prompts for required text input. Returns nil if ESC pressed.
func promptRequiredText(prompt string, defaultValue string) *string {
	for {
		oldState, err := rawTerminal()
		if err != nil {
			restoreTerminal(nil)
			return promptTextSimple(prompt, defaultValue, true)
		}

		result := readTextInput(prompt, defaultValue, oldState)
		if result == nil {
			return nil
		}
		if *result != "" {
			return result
		}
		fmt.Println("⚠️  此项不能为空，请重新输入。")
	}
}

// promptOptionalText prompts for optional text input. Returns nil if ESC pressed.
func promptOptionalText(prompt string, defaultValue string) *string {
	oldState, err := rawTerminal()
	if err != nil {
		restoreTerminal(nil)
		return promptTextSimple(prompt, defaultValue, false)
	}

	return readTextInput(prompt, defaultValue, oldState)
}

// readTextInput reads text input with ESC support.
func readTextInput(prompt string, defaultValue string, oldState *terminalState) *string {
	var input strings.Builder
	promptStr := prompt
	if defaultValue != "" {
		promptStr = prompt + " [" + defaultValue + "]"
	}

	fmt.Print(promptStr + ": ")

	hideCursor()
	defer showCursor()

	for {
		key, isSpecial := readKey()

		if isSpecial {
			switch key {
			case "esc":
				clearLine()
				restoreTerminal(oldState)
				return nil

			case "enter":
				text := strings.TrimSpace(input.String())
				restoreTerminal(oldState)
				clearLine()

				if text == "" {
					fmt.Printf("%s: %s\n", promptStr, defaultValue)
					return &defaultValue
				}
				fmt.Printf("%s: %s\n", promptStr, text)
				return &text

			case "backspace":
				if input.Len() > 0 {
					s := input.String()
					input.Reset()
					input.WriteString(s[:len(s)-1])
					clearLine()
					fmt.Print(promptStr + ": " + input.String())
				}
			}
		} else {
			input.WriteString(key)
			clearLine()
			fmt.Print(promptStr + ": " + input.String())
		}
	}
}

// promptTextSimple is a fallback for text input.
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

// promptConfirm asks a yes/no question. Returns true for yes.
func promptConfirm(prompt string, defaultYes bool) bool {
	yn := "Y/n"
	if !defaultYes {
		yn = "y/N"
	}

	oldState, err := rawTerminal()
	if err != nil {
		restoreTerminal(nil)
		return promptConfirmSimple(prompt, defaultYes)
	}

	fmt.Print(prompt + " [" + yn + "]: ")

	hideCursor()
	defer showCursor()

	for {
		key, isSpecial := readKey()

		if isSpecial {
			switch key {
			case "esc":
				clearLine()
				restoreTerminal(oldState)
				return false

			case "enter":
				restoreTerminal(oldState)
				clearLine()
				fmt.Printf("%s [%s]: %s\n", prompt, yn, map[bool]string{true: "Y", false: "N"}[defaultYes])
				return defaultYes
			}
		} else {
			lower := strings.ToLower(key)
			if lower == "y" || lower == "n" {
				result := lower == "y"
				restoreTerminal(oldState)
				clearLine()
				fmt.Printf("%s [%s]: %s\n", prompt, yn, strings.ToUpper(key))
				return result
			}
		}
	}
}

// promptConfirmSimple is a fallback for confirm prompt.
func promptConfirmSimple(prompt string, defaultYes bool) bool {
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

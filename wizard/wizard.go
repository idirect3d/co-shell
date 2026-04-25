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
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/llm"
)

// RunSetupWizard guides the user through configuring the LLM API settings interactively.
// Returns true if configuration was completed successfully, false if user cancelled.
func RunSetupWizard(cfg *config.Config) bool {
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║        🔧 co-shell API 设置向导                           ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════╣")
	fmt.Println("║  您需要先完成大模型 API 的配置，才能开始使用 co-shell。    ║")
	fmt.Println("║  按 ESC 可随时退出向导。                                   ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Step 1: Select provider
	selectedProvider := selectProvider()
	if selectedProvider == nil {
		fmt.Println("\n⚠️  已取消设置。")
		return false
	}
	cfg.LLM.Provider = selectedProvider.Name

	// Apply provider preset
	if selectedProvider.Endpoint != "" {
		cfg.LLM.Endpoint = selectedProvider.Endpoint
	}
	if selectedProvider.DefaultModel != "" {
		cfg.LLM.Model = selectedProvider.DefaultModel
	}

	fmt.Printf("📌 已选择供应商: %s\n", selectedProvider.DisplayName)
	fmt.Printf("📌 API 端点: %s\n", cfg.LLM.Endpoint)
	fmt.Printf("📌 默认模型: %s\n", cfg.LLM.Model)
	fmt.Println()

	// Step 2: For "OpenAI 兼容（自定义）", input endpoint, test connectivity,
	// then input API Key and fetch available models
	if selectedProvider.Name == "openai-compatible" {
		if !setupOpenAICompatible(cfg) {
			return false
		}
	} else {
		// For preset providers, just input endpoint (optional)
		endpoint := promptOptionalText("📌 API 端点", cfg.LLM.Endpoint)
		if endpoint == nil {
			fmt.Println("\n⚠️  已取消设置。")
			return false
		}
		cfg.LLM.Endpoint = *endpoint

		// Step 3: Model name for preset providers
		if len(selectedProvider.Models) > 0 {
			model := selectModel(selectedProvider.Models, cfg.LLM.Model)
			if model == nil {
				fmt.Println("\n⚠️  已取消设置。")
				return false
			}
			cfg.LLM.Model = *model
		} else {
			model := promptOptionalText("📌 模型名称", cfg.LLM.Model)
			if model == nil {
				fmt.Println("\n⚠️  已取消设置。")
				return false
			}
			cfg.LLM.Model = *model
		}

		// Step 4: API Key for preset providers
		if !inputAPIKey(cfg, selectedProvider) {
			return false
		}
	}

	// Save configuration
	if err := cfg.Save(); err != nil {
		fmt.Printf("⚠️  配置保存失败: %v\n", err)
	} else {
		fmt.Println("✅ 配置已保存到 ~/.co-shell/config.json")
	}
	fmt.Println()
	return true
}

// setupOpenAICompatible handles the setup flow for OpenAI-compatible providers:
// 1. Input endpoint → test connectivity
// 2. Input API Key → fetch available models
// 3. Select model from list
func setupOpenAICompatible(cfg *config.Config) bool {
	// Step 2a: API Endpoint with connectivity test
	for {
		endpoint := promptRequiredText("📌 API 端点 (必填)", "")
		if endpoint == nil {
			fmt.Println("\n⚠️  已取消设置。")
			return false
		}
		cfg.LLM.Endpoint = *endpoint

		// Test endpoint connectivity immediately after input
		fmt.Print("🔄 正在测试端点连通性...")
		if err := testEndpointConnectivity(cfg.LLM.Endpoint); err != nil {
			fmt.Printf("\n❌ 端点连接失败: %v\n", err)
			fmt.Println("⚠️  请检查端点地址是否正确，重新输入。")
			fmt.Println()
			continue
		}
		fmt.Println(" ✅ 端点可达！")
		break
	}

	// Step 2b: API Key input
	fmt.Println()
	fmt.Println("🔑 请输入 API Key 以获取可用模型列表。")
	for {
		apiKey := promptRequiredText("📌 API Key (必填)", "")
		if apiKey == nil {
			fmt.Println("\n⚠️  已取消设置。")
			return false
		}
		cfg.LLM.APIKey = *apiKey

		// Fetch available models
		fmt.Print("🔄 正在获取可用模型列表...")
		models, err := fetchModels(cfg.LLM.Endpoint, cfg.LLM.APIKey)
		if err != nil {
			fmt.Printf("\n❌ 获取模型列表失败: %v\n", err)
			fmt.Println("请检查 API Key 是否正确，或重新输入。")
			cfg.LLM.APIKey = ""
			continue
		}
		fmt.Printf(" ✅ 获取到 %d 个可用模型！\n", len(models))

		// Step 2c: Select model from list, use first model as default
		fmt.Println()
		defaultModel := models[0]
		model := selectModel(models, defaultModel)
		if model == nil {
			fmt.Println("\n⚠️  已取消设置。")
			return false
		}
		cfg.LLM.Model = *model
		break
	}

	return true
}

// inputAPIKey handles API Key input with connection test for preset providers.
func inputAPIKey(cfg *config.Config, provider *config.ProviderPreset) bool {
	fmt.Println()
	fmt.Printf("🔑 API Key 是调用 %s API 的身份凭证，用于验证您的身份并计费。\n", provider.DisplayName)

	if provider.APIKeyURL != "" {
		fmt.Println()
		choice := promptConfirm(fmt.Sprintf("   是否打开 %s 的 API Key 页面？", provider.DisplayName), true)
		if choice {
			fmt.Printf("   🔗 正在打开: %s\n", provider.APIKeyURL)
			if err := config.OpenURL(provider.APIKeyURL); err != nil {
				fmt.Printf("   ⚠️  无法自动打开浏览器: %v\n", err)
				fmt.Printf("   请手动访问: %s\n", provider.APIKeyURL)
			}
		} else {
			fmt.Println("   请手动获取 API Key 后粘贴到下方。")
		}
		fmt.Println()
	} else {
		fmt.Println("   请手动获取 API Key 后粘贴到下方。")
		fmt.Println()
	}

	// API Key input with connection test loop
	for {
		apiKey := promptRequiredText("📌 API Key (必填)", "")
		if apiKey == nil {
			fmt.Println("\n⚠️  已取消设置。")
			return false
		}
		cfg.LLM.APIKey = *apiKey

		fmt.Print("🔄 正在测试 API 连接...")
		if err := testAPIConnection(cfg); err != nil {
			fmt.Printf("\n❌ 连接测试失败: %v\n", err)
			fmt.Println("请检查 API Key 是否正确，或重新输入。")
			cfg.LLM.APIKey = ""
			continue
		}
		fmt.Println(" ✅ 连接成功！")
		break
	}

	return true
}

// testEndpointConnectivity checks if the API endpoint is reachable via HTTP.
func testEndpointConnectivity(endpoint string) error {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return fmt.Errorf("无法创建请求: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("无法连接到端点: %w", err)
	}
	defer resp.Body.Close()

	return nil
}

// fetchModels retrieves the list of available models from the API.
func fetchModels(endpoint, apiKey string) ([]string, error) {
	client := llm.NewClient(endpoint, apiKey, "", 0, 0)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return client.ListModels(ctx)
}

// testAPIConnection sends a simple chat completion request to verify the configuration.
func testAPIConnection(cfg *config.Config) error {
	client := llm.NewClient(cfg.LLM.Endpoint, cfg.LLM.APIKey, cfg.LLM.Model, cfg.LLM.Temperature, cfg.LLM.MaxTokens)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := client.Chat(ctx, []llm.Message{
		{Role: "user", Content: "Respond with exactly: OK"},
	}, nil)
	return err
}

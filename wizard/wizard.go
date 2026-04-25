// Author: L.Shuang
// Created: 2026-04-25
// Last Modified: 2026-04-25
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

	// Step 2: Input endpoint (optional for preset providers, required for custom)
	if selectedProvider.Name == "openai-compatible" {
		if !setupOpenAICompatible(cfg) {
			return false
		}
		return true
	}

	// For preset providers (DeepSeek, Qwen, etc.):
	// 1. Input endpoint (optional, has default)
	// 2. Input API Key → test connection → fetch available models
	// 3. Select model from fetched list

	// Step 2a: API Endpoint (optional, has default)
	endpoint := promptOptionalText("📌 API 端点", cfg.LLM.Endpoint)
	if endpoint == nil {
		fmt.Println("\n⚠️  已取消设置。")
		return false
	}
	cfg.LLM.Endpoint = *endpoint

	// Step 2b: API Key with connection test, then fetch models
	fmt.Println()
	apiKeyURL := selectedProvider.APIKeyURL

	for {
		apiKey := promptAPIKey(apiKeyURL)
		if apiKey == nil {
			fmt.Println("\n⚠️  已取消设置。")
			return false
		}
		cfg.LLM.APIKey = *apiKey

		// Test connection and fetch models
		fmt.Print("🔄 正在测试 API 连接并获取可用模型...")
		models, err := fetchModels(cfg.LLM.Endpoint, cfg.LLM.APIKey)
		if err != nil {
			fmt.Printf("\n❌ 连接测试失败: %v\n", err)
			fmt.Println("请检查 API Key 是否正确，或重新输入。")
			cfg.LLM.APIKey = ""
			continue
		}
		fmt.Printf(" ✅ 连接成功！获取到 %d 个可用模型。\n", len(models))

		// Step 3: Select model from fetched list
		fmt.Println()
		defaultModel := cfg.LLM.Model
		if defaultModel == "" && len(models) > 0 {
			defaultModel = models[0]
		}
		model := selectModel(models, defaultModel)
		if model == nil {
			fmt.Println("\n⚠️  已取消设置。")
			return false
		}
		cfg.LLM.Model = *model
		break
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
	for {
		apiKey := promptAPIKey("")
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

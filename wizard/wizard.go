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
	"github.com/idirect3d/co-shell/log"
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

	// Step 2: For preset providers (DeepSeek, Qwen, Xiaomi, Zhipu, etc.):
	// Skip endpoint input since it's fixed. Go directly to API Key input.
	// For OpenAI-compatible (custom), go through the full setup flow.
	if selectedProvider.Name == "openai-compatible" {
		if !setupOpenAICompatible(cfg) {
			return false
		}
		return true
	}

	// For preset providers: skip endpoint input (fixed), go directly to API Key
	// API Key with connection test, then fetch models
	fmt.Println()

	for {
		apiKey := cfg.LLM.APIKey
		if selectedProvider.Name != "ollama" {
			key := promptAPIKey(selectedProvider.APIKeyURL, apiKey)
			if key == nil {
				fmt.Println("\n⚠️  已取消设置。")
				return false
			}
			apiKey = *key
		}

		// Test connection and fetch models
		fmt.Print("🔄 正在测试 API 连接并获取可用模型...")
		models, err := fetchModels(cfg.LLM.Endpoint, apiKey)
		if err != nil {
			fmt.Printf("\n❌ 连接测试失败: %v\n", err)
			if selectedProvider.Name == "ollama" {
				fmt.Println("请检查 Ollama 服务是否已启动，端点地址是否正确。")
			} else {
				fmt.Println("请检查 API Key 是否正确，或重新输入。")
			}
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
		cfg.LLM.VisionSupport = getModelVisionSupport(cfg, *model)
		cfg.LLM.APIKey = apiKey
		break
	}

	// Final connection test with summary
	fmt.Println()
	fmt.Printf("🔄 正在连接 %s[%s]... ", cfg.LLM.Endpoint, cfg.LLM.Model)
	if err := testEndpointConnectivity(cfg.LLM.Endpoint); err != nil {
		fmt.Printf("失败: %v\n", err)
	} else {
		fmt.Println("成功")
	}

	// Save configuration
	if err := cfg.Save(); err != nil {
		fmt.Printf("⚠️  配置保存失败: %v\n", err)
	} else {
		fmt.Println("✅ 配置已保存")
	}

	// Show how to modify settings later
	fmt.Println()
	fmt.Println("💡 提示：以后如需修改配置，可以使用以下方法：")
	fmt.Println("   1. 在 REPL 中输入 .wizard 重新运行设置向导")
	fmt.Println("   2. 在 REPL 中输入 .set <参数名> <值> 修改单个参数")
	fmt.Println("   3. 直接编辑配置文件 config.json")
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
		endpoint := promptRequiredText("📌 API 端点", cfg.LLM.Endpoint)
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
		apiKey := promptAPIKey("", cfg.LLM.APIKey)
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
		if len(models) == 0 {
			fmt.Println("⚠️  未获取到可用模型，请手动输入模型名称。")
			model := promptRequiredText("📌 模型名称 (必填)", "")
			if model == nil {
				fmt.Println("\n⚠️  已取消设置。")
				return false
			}
			cfg.LLM.Model = *model
		} else {
			defaultModel := models[0]
			model := selectModel(models, defaultModel)
			if model == nil {
				fmt.Println("\n⚠️  已取消设置。")
				return false
			}
			cfg.LLM.Model = *model
			cfg.LLM.VisionSupport = getModelVisionSupport(cfg, *model)
		}
		break
	}

	// Final connection test with summary
	fmt.Println()
	fmt.Printf("🔄 正在连接 %s[%s]... ", cfg.LLM.Endpoint, cfg.LLM.Model)
	if err := testEndpointConnectivity(cfg.LLM.Endpoint); err != nil {
		fmt.Printf("失败: %v\n", err)
	} else {
		fmt.Println("成功")
	}

	// Save configuration
	if err := cfg.Save(); err != nil {
		fmt.Printf("⚠️  配置保存失败: %v\n", err)
	} else {
		fmt.Println("✅ 配置已保存")
	}

	// Show how to modify settings later
	fmt.Println()
	fmt.Println("💡 提示：以后如需修改配置，可以使用以下方法：")
	fmt.Println("   1. 在 REPL 中输入 .wizard 重新运行设置向导")
	fmt.Println("   2. 在 REPL 中输入 .set <参数名> <值> 修改单个参数")
	fmt.Println("   3. 直接编辑配置文件 config.json")
	fmt.Println()
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

	log.Info("Endpoint connectivity test: GET %s, timeout=10s", endpoint)

	resp, err := client.Do(req)
	if err != nil {
		log.Error("Endpoint connectivity test failed: GET %s, error: %v", endpoint, err)
		return fmt.Errorf("无法连接到端点: %w", err)
	}
	defer resp.Body.Close()

	log.Info("Endpoint connectivity test succeeded: GET %s, status=%d", endpoint, resp.StatusCode)
	return nil
}

// fetchModels retrieves the list of available models from the API.
// Returns model IDs as strings for backward compatibility with the model selector.
func fetchModels(endpoint, apiKey string) ([]string, error) {
	client := llm.NewClient(endpoint, apiKey, "", 0, 0)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	log.Info("Fetching models: endpoint=%s, timeout=30s", endpoint)
	modelInfos, err := client.ListModels(ctx)
	if err != nil {
		log.Error("Fetch models failed: endpoint=%s, error: %v", endpoint, err)
		return nil, err
	}
	log.Info("Fetch models succeeded: endpoint=%s, count=%d", endpoint, len(modelInfos))

	// Extract model IDs and detect vision support for the selected model
	models := make([]string, 0, len(modelInfos))
	for _, mi := range modelInfos {
		models = append(models, mi.ID)
	}

	// Store model info for later use (vision support detection)
	storeModelInfos(modelInfos)

	return models, nil
}

// modelInfoCache stores the last fetched model info for vision support detection.
var modelInfoCache []llm.ModelInfo

// storeModelInfos caches model info for later use.
func storeModelInfos(infos []llm.ModelInfo) {
	modelInfoCache = infos
}

// getModelVisionSupport checks if a model supports vision.
// First tries cached model info from API response.
// If the API didn't provide vision info (e.g., custom models without capabilities field),
// performs a live test by sending a minimal multimodal request.
func getModelVisionSupport(cfg *config.Config, modelID string) bool {
	// Try cached model info first
	for _, mi := range modelInfoCache {
		if mi.ID == modelID {
			if mi.VisionSupport {
				return true
			}
			// If API explicitly says no vision, still try live test
			// as some APIs may not report capabilities correctly
			break
		}
	}

	// Perform live test: send a minimal multimodal request
	fmt.Println()
	fmt.Print("🔄 正在检测模型是否支持视觉识别...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create a temporary client with the current config for testing
	client := llm.NewClient(
		cfg.LLM.Endpoint,
		cfg.LLM.APIKey,
		modelID,
		cfg.LLM.Temperature,
		cfg.LLM.MaxTokens,
		10, // 10s timeout for test
	)
	defer client.Close()

	supportsVision := client.TestVisionSupport(ctx)
	if supportsVision {
		fmt.Println(" ✅ 支持视觉识别！")
	} else {
		fmt.Println(" ❌ 不支持视觉识别。")
	}
	return supportsVision
}

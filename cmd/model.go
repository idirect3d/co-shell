// Author: L.Shuang
// Created: 2026-05-07
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
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/llm"
	"github.com/idirect3d/co-shell/log"
)

// ModelHandler handles the .model built-in command for multi-model management.
type ModelHandler struct {
	cfg         *config.Config
	scanner     *bufio.Scanner // for interactive wizard input
	wizardStack []string       // stack of wizard steps to return to
}

// NewModelHandler creates a new ModelHandler.
func NewModelHandler(cfg *config.Config) *ModelHandler {
	return &ModelHandler{
		cfg:         cfg,
		scanner:     bufio.NewScanner(os.Stdin),
		wizardStack: make([]string, 0),
	}
}

// Handle processes .model commands.
func (h *ModelHandler) Handle(args []string) (string, error) {
	if len(args) == 0 {
		return h.showHelp()
	}

	subcommand := args[0]
	switch subcommand {
	case "list", "ls":
		return h.listModels()
	case "add":
		return h.addModelWizard()
	case "remove", "rm":
		return h.removeModel(args[1:])
	case "switch", "use":
		return h.switchModel(args[1:])
	case "enable":
		return h.enableModel(args[1:])
	case "disable":
		return h.disableModel(args[1:])
	case "info":
		return h.modelInfo(args[1:])
	case "templates", "tpl":
		return h.listTemplates()
	case "add-from-template", "from-tpl":
		return h.addFromTemplate(args[1:])
	case "set-priority", "prio":
		return h.setPriority(args[1:])
	default:
		return "", fmt.Errorf("unknown model subcommand: %s (use .model for help)", subcommand)
	}
}

// showHelp returns the help message for .model command.
func (h *ModelHandler) showHelp() (string, error) {
	var result strings.Builder
	result.WriteString("═══════════════════════════════════════════════════════\n")
	result.WriteString("  📋 多模型管理命令 / Multi-Model Management\n")
	result.WriteString("═══════════════════════════════════════════════════════\n\n")
	result.WriteString("  .model list / ls              - 列出所有已配置模型\n")
	result.WriteString("  .model info <id>              - 显示模型详细信息\n")
	result.WriteString("  .model add                    - 向导模式添加新模型\n")
	result.WriteString("  .model from-tpl <tpl> <mdl>   - 从模板添加模型 (--api-key)\n")
	result.WriteString("  .model remove <id>            - 移除模型\n")
	result.WriteString("  .model switch <id>            - 切换到指定模型\n")
	result.WriteString("  .model enable <id>            - 启用模型\n")
	result.WriteString("  .model disable <id>           - 禁用模型\n")
	result.WriteString("  .model set-priority <id> <n>  - 设置优先级\n")
	result.WriteString("  .model templates              - 列出可用模板\n\n")
	result.WriteString("  优先级越高越优先使用，switch 会启用目标并禁用其他模型\n")
	return result.String(), nil
}

// listModels shows all configured models.
func (h *ModelHandler) listModels() (string, error) {
	models := h.cfg.Models
	if len(models) == 0 {
		return "未配置多模型。使用 .model add 或 .model from-tpl 添加模型。\n", nil
	}

	sorted := make([]*config.ModelConfig, len(models))
	copy(sorted, models)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].Priority > sorted[i].Priority {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	var result strings.Builder
	result.WriteString("═══════════════════════════════════════════════════════\n")
	result.WriteString("  📋 已配置模型 / Configured Models\n")
	result.WriteString("═══════════════════════════════════════════════════════\n\n")

	activeCount := 0
	for _, m := range sorted {
		status := "⬜"
		if m.Enabled {
			status = "✅"
			activeCount++
		}
		capStr := []string{}
		if m.Capabilities.Vision {
			capStr = append(capStr, "👁视觉")
		}
		if m.Capabilities.ToolCall {
			capStr = append(capStr, "🔧工具")
		}
		if m.Capabilities.Thinking {
			capStr = append(capStr, "💭思考")
		}

		result.WriteString(fmt.Sprintf("  %s [%-30s] %s\n", status, m.ID, m.Name))
		result.WriteString(fmt.Sprintf("     供应商: %s | 模型: %s | 优先级: %d\n", m.Provider, m.Model, m.Priority))
		if len(capStr) > 0 {
			result.WriteString(fmt.Sprintf("     能力: %s\n", strings.Join(capStr, " ")))
		}
		result.WriteString("\n")
	}

	result.WriteString(fmt.Sprintf("  统计: %d 个已启用 / %d 个总计\n", activeCount, len(sorted)))
	return result.String(), nil
}

// modelInfo shows detailed information about a specific model.
func (h *ModelHandler) modelInfo(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("用法: .model info <模型ID>")
	}

	modelID := args[0]
	var model *config.ModelConfig
	for _, m := range h.cfg.Models {
		if m.ID == modelID {
			model = m
			break
		}
	}

	if model == nil {
		return "", fmt.Errorf("模型 %s 不存在", modelID)
	}

	var result strings.Builder
	result.WriteString("═══════════════════════════════════════════════════════\n")
	result.WriteString("  📋 模型详情 / Model Info\n")
	result.WriteString("═══════════════════════════════════════════════════════\n\n")
	result.WriteString(fmt.Sprintf("  ID: %s\n", model.ID))
	result.WriteString(fmt.Sprintf("  名称: %s\n", model.Name))
	result.WriteString(fmt.Sprintf("  供应商: %s\n", model.Provider))
	result.WriteString(fmt.Sprintf("  端点: %s\n", model.Endpoint))
	result.WriteString(fmt.Sprintf("  模型: %s\n", model.Model))
	result.WriteString(fmt.Sprintf("  优先级: %d\n", model.Priority))
	if model.Enabled {
		result.WriteString("  状态: ✅ 已启用\n")
	} else {
		result.WriteString("  状态: ⬜ 已禁用\n")
	}
	if model.TemplateID != "" {
		result.WriteString(fmt.Sprintf("  模板: %s\n", model.TemplateID))
	}

	capStr := []string{}
	if model.Capabilities.Vision {
		capStr = append(capStr, "👁 视觉识别")
	}
	if model.Capabilities.ToolCall {
		capStr = append(capStr, "🔧 工具调用")
	}
	if model.Capabilities.Thinking {
		capStr = append(capStr, "💭 思考模式")
	}
	if len(capStr) > 0 {
		result.WriteString(fmt.Sprintf("  能力: %s\n", strings.Join(capStr, "、")))
	} else {
		result.WriteString("  能力: 未指定\n")
	}

	return result.String(), nil
}

// addModelWizard starts the interactive wizard to add a new model.
func (h *ModelHandler) addModelWizard() (string, error) {
	var result strings.Builder
	result.WriteString("═══════════════════════════════════════════════════════\n")
	result.WriteString("  📋 添加模型向导 / Add Model Wizard\n")
	result.WriteString("═══════════════════════════════════════════════════════\n\n")

	for {
		// Step 1: Select template
		template, err := h.wizardSelectTemplate()
		if err != nil || template == nil {
			return result.String(), err
		}

		// Step 2: Enter model parameters
		modelConfig, err := h.wizardEnterModelParams(template)
		if err != nil {
			if err.Error() == "__BACK__" {
				fmt.Println("\n  返回上一步")
				continue
			}
			return result.String(), err
		}

		// Step 3: Confirm and save
		if err := h.saveModel(modelConfig); err != nil {
			return result.String(), err
		}

		result.WriteString(fmt.Sprintf("\n✅ 已成功添加模型: %s (%s)\n", modelConfig.ID, modelConfig.Model))
		log.Info("Added model via wizard: %s (template=%s, model=%s)", modelConfig.ID, template.ID, modelConfig.Model)
		return result.String(), nil
	}
}

// wizardSelectTemplate displays template list and lets user select one.
func (h *ModelHandler) wizardSelectTemplate() (*config.ModelTemplate, error) {
	manager := config.GetDefaultModelManager()
	templates := manager.GetAllTemplates()

	for {
		fmt.Print("\n请选择模板 (输入序号，0 返回):\n\n")
		fmt.Printf("  [0] 返回上一步\n\n")

		for i, t := range templates {
			prefix := fmt.Sprintf("  [%d]", i+1)
			fmt.Printf("%s %-20s %s\n", prefix, t.ID, t.Name)
			fmt.Printf("%-4s %s\n", "", t.Description)
			if len(t.Models) > 0 {
				fmt.Printf("%-4s 默认模型: %s\n", "", strings.Join(t.Models, ", "))
			}
			capStr := []string{}
			if t.Capabilities.Vision {
				capStr = append(capStr, "👁视觉")
			}
			if t.Capabilities.ToolCall {
				capStr = append(capStr, "🔧工具")
			}
			if t.Capabilities.Thinking {
				capStr = append(capStr, "💭思考")
			}
			if len(capStr) > 0 {
				fmt.Printf("%-4s 能力: %s\n", "", strings.Join(capStr, " "))
			}
			fmt.Println()
		}

		fmt.Print("  请选择: ")
		if !h.scanner.Scan() {
			return nil, fmt.Errorf("向导已取消")
		}
		input := strings.TrimSpace(h.scanner.Text())

		if input == "0" || strings.ToUpper(input) == "Q" || strings.ToUpper(input) == "QUIT" || strings.ToUpper(input) == "BACK" || strings.ToUpper(input) == ".." {
			fmt.Println("  返回上一步")
			return nil, nil
		}

		if strings.ToUpper(input) == "Q" || strings.ToUpper(input) == "QUIT" {
			return nil, fmt.Errorf("向导已取消")
		}

		idx, err := strconv.Atoi(input)
		if err != nil || idx < 1 || idx > len(templates) {
			fmt.Println("  无效输入，请重新选择")
			continue
		}

		selected := templates[idx-1]
		fmt.Printf("  ✅ 已选择模板: %s (%s)\n", selected.ID, selected.Name)
		return selected, nil
	}
}

// wizardEnterModelParams prompts user to enter model-specific parameters.
func (h *ModelHandler) wizardEnterModelParams(template *config.ModelTemplate) (*config.ModelConfig, error) {
	var result strings.Builder
	result.WriteString("\n")

	// Step 1: Enter endpoint (optional, default from template)
	defaultEndpoint := template.Endpoint
	endpoint := h.wizardPromptStringWithDefault("请输入 API 端点", defaultEndpoint, "q")
	if strings.ToUpper(endpoint) == "Q" || strings.ToUpper(endpoint) == "QUIT" {
		return nil, fmt.Errorf("向导已取消")
	}
	if endpoint == "0" || strings.ToUpper(endpoint) == "BACK" || strings.ToUpper(endpoint) == ".." {
		return nil, fmt.Errorf("__BACK__")
	}

	// Test endpoint connectivity
	fmt.Print("\n  🔍 正在测试端点连通性... ")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	client := llm.NewClient(endpoint, "", "test", 0, 0, 10)
	models, err := client.ListModels(ctx)
	cancel()
	if err != nil {
		// HTTP error means connectivity is OK, no need to prompt
		if !strings.Contains(err.Error(), "status") && !strings.Contains(err.Error(), "HTTP") {
			fmt.Printf("❌ 连接失败: %v\n", err)
			fmt.Print("  是否继续使用此端点？(y/n) [默认: n]: ")
			if !h.scanner.Scan() {
				return nil, fmt.Errorf("向导已取消")
			}
			retry := strings.TrimSpace(strings.ToLower(h.scanner.Text()))
			if retry != "y" && retry != "yes" {
				return nil, fmt.Errorf("端点连接测试未通过，请检查端点后重试")
			}
		}
	} else {
		fmt.Printf("✅ 连接成功 (发现 %d 个模型)\n", len(models))
	}

	// Step 2: Enter API key
	// Find existing API key from models with the same template ID,
	// fall back to config default API key
	defaultAPIKey := ""
	for _, m := range h.cfg.Models {
		if m.TemplateID == template.ID && m.APIKey != "" {
			defaultAPIKey = m.APIKey
			break
		}
	}
	if defaultAPIKey == "" && h.cfg.LLM.APIKey != "" {
		defaultAPIKey = h.cfg.LLM.APIKey
	}
	apiKey := h.wizardPromptSecret("请输入 API Key", defaultAPIKey)
	if strings.ToUpper(apiKey) == "Q" || strings.ToUpper(apiKey) == "QUIT" {
		return nil, fmt.Errorf("向导已取消")
	}
	if apiKey == "0" || strings.ToUpper(apiKey) == "BACK" || strings.ToUpper(apiKey) == ".." {
		return nil, fmt.Errorf("__BACK__")
	}

	// Step 3: Fetch available models from API and let user select
	fmt.Print("\n  🔍 正在获取可用模型列表... ")
	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	client = llm.NewClient(endpoint, apiKey, "test", 0, 0, 15)
	models, err = client.ListModels(ctx)
	cancel()
	if err != nil {
		fmt.Printf("⚠️ 获取模型列表失败: %v\n", err)
		fmt.Println("  将使用模板默认模型列表")
		models = nil
	} else {
		fmt.Printf("✅ 获取到 %d 个模型\n", len(models))
	}

	// Build model name suggestions: API models first, then template defaults
	modelSuggestions := make([]string, 0)
	seen := make(map[string]bool)
	if models != nil {
		for _, m := range models {
			if !seen[m.ID] {
				modelSuggestions = append(modelSuggestions, m.ID)
				seen[m.ID] = true
			}
		}
	}
	for _, m := range template.Models {
		if !seen[m] {
			modelSuggestions = append(modelSuggestions, m)
			seen[m] = true
		}
	}

	modelName := h.wizardPromptString("请选择或输入模型名称", modelSuggestions, "q")
	if modelName == "" || strings.ToUpper(modelName) == "Q" || strings.ToUpper(modelName) == "QUIT" {
		return nil, fmt.Errorf("向导已取消")
	}
	if modelName == "0" || strings.ToUpper(modelName) == "BACK" || strings.ToUpper(modelName) == ".." {
		return nil, fmt.Errorf("__BACK__")
	}

	// Step 4: Auto-detect capabilities by sending test requests
	fmt.Print("\n  🔍 正在检测模型能力...\n")
	detectedCaps := h.detectModelCapabilities(endpoint, apiKey, modelName)

	// Step 5: Choose capabilities (pre-populated with detected results)
	capabilities, goBack := h.wizardSelectCapabilities(detectedCaps)
	if goBack {
		return nil, fmt.Errorf("__BACK__")
	}

	// Step 6: Enter model ID (customizable, default: templateID-modelName)
	defaultModelID := fmt.Sprintf("%s-%s", template.ID, strings.ReplaceAll(modelName, "/", "-"))
	// If default ID already exists, append a suffix number
	if h.modelIDExists(defaultModelID) {
		suffix := 2
		for {
			candidate := fmt.Sprintf("%s-%d", defaultModelID, suffix)
			if !h.modelIDExists(candidate) {
				defaultModelID = candidate
				break
			}
			suffix++
		}
	}
	modelID := h.wizardPromptStringWithDefault("请输入模型 ID", defaultModelID, "q")
	if strings.ToUpper(modelID) == "Q" || strings.ToUpper(modelID) == "QUIT" {
		return nil, fmt.Errorf("向导已取消")
	}
	if modelID == "0" || strings.ToUpper(modelID) == "BACK" || strings.ToUpper(modelID) == ".." {
		return nil, fmt.Errorf("__BACK__")
	}

	// Step 7: Set priority
	priorityStr := h.wizardPromptStringWithDefault("请设置优先级 (数字，默认 "+fmt.Sprintf("%d", template.Priority)+")", fmt.Sprintf("%d", template.Priority), "q")
	if strings.ToUpper(priorityStr) == "Q" || strings.ToUpper(priorityStr) == "QUIT" {
		return nil, fmt.Errorf("向导已取消")
	}
	if priorityStr == "0" || strings.ToUpper(priorityStr) == "BACK" || strings.ToUpper(priorityStr) == ".." {
		return nil, fmt.Errorf("__BACK__")
	}
	priority, err := strconv.Atoi(priorityStr)
	if err != nil {
		priority = template.Priority
	}

	// Step 8: Enable model?
	enabled := h.wizardPromptBool("是否立即启用此模型？(y/n)", true)
	if !enabled {
		enabled = false
	}

	return &config.ModelConfig{
		ID:           modelID,
		Name:         fmt.Sprintf("%s (%s)", template.Name, modelName),
		Provider:     template.Provider,
		Endpoint:     endpoint,
		Model:        modelName,
		APIKey:       apiKey,
		Priority:     priority,
		Enabled:      enabled,
		TemplateID:   template.ID,
		Capabilities: capabilities,
	}, nil
}

// modelIDExists checks if a model ID already exists in the configuration.
func (h *ModelHandler) modelIDExists(id string) bool {
	for _, m := range h.cfg.Models {
		if m.ID == id {
			return true
		}
	}
	return false
}

// wizardPromptString prompts for a string value with template suggestions.
// Supports both numeric selection and direct text input.
// Default value: first suggestion if available, otherwise empty.
func (h *ModelHandler) wizardPromptString(prompt string, suggestions []string, cancelKeys string) string {
	defaultVal := ""
	if len(suggestions) > 0 {
		defaultVal = suggestions[0]
	}

	for {
		if len(suggestions) > 0 {
			fmt.Printf("\n%s:\n", prompt)
			for i, s := range suggestions {
				fmt.Printf("  [%d] %s\n", i+1, s)
			}
			fmt.Printf("  请选择或输入 [默认: %s]: ", defaultVal)
		} else {
			fmt.Printf("\n%s: ", prompt)
		}

		if !h.scanner.Scan() {
			return ""
		}
		input := strings.TrimSpace(h.scanner.Text())

		// Check cancel keys
		if len(input) > 0 {
			upper := strings.ToUpper(input)
			if upper == "Q" || upper == "QUIT" {
				return ""
			}
		}

		// Empty input: use default
		if input == "" {
			if defaultVal != "" {
				fmt.Printf("  使用默认值: %s\n", defaultVal)
				return defaultVal
			}
			fmt.Println("  输入不能为空，请重新输入")
			continue
		}

		// Check if user selected a suggestion by number
		if len(suggestions) > 0 {
			if idx, err := strconv.Atoi(input); err == nil && idx >= 1 && idx <= len(suggestions) {
				return suggestions[idx-1]
			}
		}

		// Direct text input
		return input
	}
}

// wizardPromptStringWithDefault prompts for a string value with a default.
func (h *ModelHandler) wizardPromptStringWithDefault(prompt string, defaultValue string, cancelKeys string) string {
	for {
		fmt.Printf("\n%s [默认: %s]: ", prompt, defaultValue)

		if !h.scanner.Scan() {
			return ""
		}
		input := strings.TrimSpace(h.scanner.Text())

		if input == "" {
			fmt.Printf("  使用默认值: %s\n", defaultValue)
			return defaultValue
		}

		return input
	}
}

// wizardPromptSecret prompts for a secret value (API key).
// If defaultVal is provided, it will be shown as masked default.
func (h *ModelHandler) wizardPromptSecret(prompt string, defaultVal string) string {
	for {
		if defaultVal != "" {
			masked := defaultVal[:4] + "****" + defaultVal[len(defaultVal)-4:]
			fmt.Printf("\n%s [默认: %s]: ", prompt, masked)
		} else {
			fmt.Printf("\n%s: ", prompt)
		}

		if !h.scanner.Scan() {
			return ""
		}
		input := strings.TrimSpace(h.scanner.Text())

		if input == "" {
			if defaultVal != "" {
				fmt.Println("  使用默认 API Key")
				return defaultVal
			}
			fmt.Println("  API Key 留空")
			return ""
		}

		return input
	}
}

// wizardPromptBool prompts for a yes/no answer.
func (h *ModelHandler) wizardPromptBool(prompt string, defaultVal bool) bool {
	for {
		defaultStr := "y"
		if !defaultVal {
			defaultStr = "n"
		}
		fmt.Printf("\n%s [默认: %s]: ", prompt, defaultStr)

		if !h.scanner.Scan() {
			return defaultVal
		}
		input := strings.TrimSpace(strings.ToLower(h.scanner.Text()))

		if input == "" {
			fmt.Printf("  使用默认值: %s\n", defaultStr)
			return defaultVal
		}

		switch input {
		case "y", "yes", "是", "yep", "yeah":
			return true
		case "n", "no", "否", "nope":
			return false
		default:
			fmt.Println("  无效输入，请输入 y 或 n")
		}
	}
}

// detectModelCapabilities auto-detects model capabilities by sending test requests.
// Tests vision, tool call, and thinking support.
func (h *ModelHandler) detectModelCapabilities(endpoint, apiKey, modelName string) config.ModelCapability {
	caps := config.ModelCapability{}

	// If no API key provided, use empty string (don't fall back to config default)
	// because the user may want to test with no key (e.g., local models)
	testKey := apiKey

	// Create a test client
	client := llm.NewClient(endpoint, testKey, modelName, 0, 0, 30)
	defer client.Close()

	// Test vision support
	fmt.Print("  👁 视觉识别... ")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	vision := client.TestVisionSupport(ctx)
	cancel()
	if vision {
		fmt.Println("✅ 支持")
		caps.Vision = true
	} else {
		fmt.Println("❌ 不支持")
	}

	// Test tool call support
	fmt.Print("  🔧 工具调用... ")
	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	toolCall := client.TestToolCallSupport(ctx)
	cancel()
	if toolCall {
		fmt.Println("✅ 支持")
		caps.ToolCall = true
	} else {
		fmt.Println("❌ 不支持")
	}

	// Test thinking support
	fmt.Print("  💭 思考模式... ")
	ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	thinking := client.TestThinkingSupport(ctx)
	cancel()
	if thinking {
		fmt.Println("✅ 支持")
		caps.Thinking = true
	} else {
		fmt.Println("❌ 不支持")
	}

	return caps
}

// wizardSelectCapabilities lets user review and adjust model capabilities.
// Shows detected capabilities and allows toggling.
// Returns capabilities and whether user chose to go back.
func (h *ModelHandler) wizardSelectCapabilities(base config.ModelCapability) (config.ModelCapability, bool) {
	caps := config.ModelCapability{
		Vision:   base.Vision,
		ToolCall: base.ToolCall,
		Thinking: base.Thinking,
	}

	for {
		fmt.Println("\n请确认模型能力 (可切换开关):")
		fmt.Println("  [1] 👁 视觉识别 (Vision)")
		fmt.Println("  [2] 🔧 工具调用 (Tool Call)")
		fmt.Println("  [3] 💭 思考模式 (Thinking)")
		fmt.Printf("\n  当前选择: ")
		if caps.Vision {
			fmt.Print("👁 ")
		}
		if caps.ToolCall {
			fmt.Print("🔧 ")
		}
		if caps.Thinking {
			fmt.Print("💭 ")
		}
		fmt.Println()
		fmt.Print("  请选择 (回车完成, 0 返回上一步): ")

		if !h.scanner.Scan() {
			return caps, false
		}
		input := strings.TrimSpace(h.scanner.Text())

		// Empty input: complete selection
		if input == "" {
			return caps, false
		}

		if input == "0" || strings.ToUpper(input) == "BACK" || strings.ToUpper(input) == ".." {
			return caps, true
		}

		switch input {
		case "1":
			caps.Vision = !caps.Vision
		case "2":
			caps.ToolCall = !caps.ToolCall
		case "3":
			caps.Thinking = !caps.Thinking
		default:
			fmt.Println("  无效输入")
		}
	}
}

// saveModel adds and saves a model configuration.
func (h *ModelHandler) saveModel(model *config.ModelConfig) error {
	// Check for duplicate ID
	for _, m := range h.cfg.Models {
		if m.ID == model.ID {
			return fmt.Errorf("模型 %s 已存在，请使用不同的模型名称", model.ID)
		}
	}

	h.cfg.Models = append(h.cfg.Models, model)

	if err := h.cfg.Save(); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}

	return nil
}

// addFromTemplate adds a model from a built-in template.
func (h *ModelHandler) addFromTemplate(args []string) (string, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("用法: .model from-tpl <模板ID> <模型ID> [--api-key <密钥>]")
	}

	templateID := args[0]
	modelName := args[1]

	var apiKey string
	for i := 2; i < len(args); i++ {
		if args[i] == "--api-key" && i+1 < len(args) {
			apiKey = args[i+1]
			i++
		}
	}

	manager := config.GetDefaultModelManager()
	template := manager.GetTemplate(templateID)
	if template == nil {
		return "", fmt.Errorf("模板 %s 不存在 (使用 .model templates 查看可用模板)", templateID)
	}

	modelID := fmt.Sprintf("%s-%s", templateID, strings.ReplaceAll(modelName, "/", "-"))

	for _, m := range h.cfg.Models {
		if m.ID == modelID {
			return "", fmt.Errorf("模型 %s 已存在", modelID)
		}
	}

	newModel := &config.ModelConfig{
		ID:           modelID,
		Name:         fmt.Sprintf("%s (%s)", template.Name, modelName),
		Provider:     template.Provider,
		Endpoint:     template.Endpoint,
		Model:        modelName,
		APIKey:       apiKey,
		Priority:     template.Priority,
		Enabled:      false,
		TemplateID:   templateID,
		Capabilities: template.Capabilities,
	}

	h.cfg.Models = append(h.cfg.Models, newModel)

	if err := h.cfg.Save(); err != nil {
		return "", fmt.Errorf("保存配置失败: %w", err)
	}

	log.Info("Added model from template: %s (template=%s, model=%s)", modelID, templateID, modelName)
	return fmt.Sprintf("✅ 已从模板 '%s' 添加模型: %s (%s)", template.Name, modelID, modelName), nil
}

// removeModel removes a model configuration.
func (h *ModelHandler) removeModel(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("用法: .model remove <模型ID>")
	}

	modelID := args[0]

	for i, m := range h.cfg.Models {
		if m.ID == modelID {
			h.cfg.Models = append(h.cfg.Models[:i], h.cfg.Models[i+1:]...)
			if err := h.cfg.Save(); err != nil {
				return "", fmt.Errorf("保存配置失败: %w", err)
			}
			log.Info("Removed model: %s", modelID)
			return fmt.Sprintf("✅ 已移除模型: %s", modelID), nil
		}
	}

	return "", fmt.Errorf("模型 %s 不存在", modelID)
}

// switchModel switches to a specific model.
func (h *ModelHandler) switchModel(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("用法: .model switch <模型ID>")
	}

	modelID := args[0]

	found := false
	for _, m := range h.cfg.Models {
		if m.ID == modelID {
			m.Enabled = true
			found = true
		} else {
			m.Enabled = false
		}
	}

	if !found {
		return "", fmt.Errorf("模型 %s 不存在", modelID)
	}

	for _, m := range h.cfg.Models {
		if m.Enabled {
			h.cfg.LLM.Provider = m.Provider
			h.cfg.LLM.Endpoint = m.Endpoint
			h.cfg.LLM.Model = m.Model
			if m.APIKey != "" {
				h.cfg.LLM.APIKey = m.APIKey
			}
			break
		}
	}

	if err := h.cfg.Save(); err != nil {
		return "", fmt.Errorf("保存配置失败: %w", err)
	}

	log.Info("Switched to model: %s", modelID)
	return fmt.Sprintf("✅ 已切换到模型: %s", modelID), nil
}

// enableModel enables a specific model.
func (h *ModelHandler) enableModel(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("用法: .model enable <模型ID>")
	}

	modelID := args[0]

	for _, m := range h.cfg.Models {
		if m.ID == modelID {
			m.Enabled = true
			if err := h.cfg.Save(); err != nil {
				return "", fmt.Errorf("保存配置失败: %w", err)
			}
			log.Info("Enabled model: %s", modelID)
			return fmt.Sprintf("✅ 已启用模型: %s", modelID), nil
		}
	}

	return "", fmt.Errorf("模型 %s 不存在", modelID)
}

// disableModel disables a specific model.
func (h *ModelHandler) disableModel(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("用法: .model disable <模型ID>")
	}

	modelID := args[0]

	for _, m := range h.cfg.Models {
		if m.ID == modelID {
			m.Enabled = false
			if err := h.cfg.Save(); err != nil {
				return "", fmt.Errorf("保存配置失败: %w", err)
			}
			log.Info("Disabled model: %s", modelID)
			return fmt.Sprintf("✅ 已禁用模型: %s", modelID), nil
		}
	}

	return "", fmt.Errorf("模型 %s 不存在", modelID)
}

// setPriority sets the priority of a model.
func (h *ModelHandler) setPriority(args []string) (string, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("用法: .model set-priority <模型ID> <优先级>")
	}

	modelID := args[0]
	priority, err := strconv.Atoi(args[1])
	if err != nil {
		return "", fmt.Errorf("无效的优先级值: %s", args[1])
	}

	for _, m := range h.cfg.Models {
		if m.ID == modelID {
			m.Priority = priority
			if err := h.cfg.Save(); err != nil {
				return "", fmt.Errorf("保存配置失败: %w", err)
			}
			log.Info("Set priority for model %s to %d", modelID, priority)
			return fmt.Sprintf("✅ 已将模型 %s 的优先级设置为: %d", modelID, priority), nil
		}
	}

	return "", fmt.Errorf("模型 %s 不存在", modelID)
}

// listTemplates lists all built-in templates.
func (h *ModelHandler) listTemplates() (string, error) {
	manager := config.GetDefaultModelManager()
	templates := manager.GetAllTemplates()

	var result strings.Builder
	result.WriteString("═══════════════════════════════════════════════════════\n")
	result.WriteString("  📋 可用模板 / Available Templates\n")
	result.WriteString("═══════════════════════════════════════════════════════\n\n")

	for _, t := range templates {
		capStr := []string{}
		if t.Capabilities.Vision {
			capStr = append(capStr, "👁")
		}
		if t.Capabilities.ToolCall {
			capStr = append(capStr, "🔧")
		}
		if t.Capabilities.Thinking {
			capStr = append(capStr, "💭")
		}

		result.WriteString(fmt.Sprintf("  [%s] %s (优先级: %d)\n", t.ID, t.Name, t.Priority))
		result.WriteString(fmt.Sprintf("     %s\n", t.Description))
		if len(t.Models) > 0 {
			result.WriteString(fmt.Sprintf("     默认模型: %s\n", strings.Join(t.Models, ", ")))
		}
		if len(capStr) > 0 {
			result.WriteString(fmt.Sprintf("     能力: %s\n", strings.Join(capStr, " ")))
		}
		result.WriteString("\n")
	}

	result.WriteString("  使用 .model add 向导模式添加模型\n")
	result.WriteString("  或使用 .model from-tpl <模板ID> <模型ID> 命令行添加\n")
	return result.String(), nil
}

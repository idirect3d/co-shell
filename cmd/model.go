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
	"fmt"
	"strconv"
	"strings"

	"github.com/idirect3d/co-shell/config"
	"github.com/idirect3d/co-shell/log"
)

// ModelHandler handles the .model built-in command for multi-model management.
type ModelHandler struct {
	cfg *config.Config
}

// NewModelHandler creates a new ModelHandler.
func NewModelHandler(cfg *config.Config) *ModelHandler {
	return &ModelHandler{cfg: cfg}
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
		return h.addModel(args[1:])
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
	result.WriteString("  .model add <prov> <model>     - 添加新模型 (--endpoint/--api-key/--priority)\n")
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
		return "未配置多模型。使用 .model from-tpl 或 .model add 添加模型。\n", nil
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
	if model.Capabilities.Multimodal {
		capStr = append(capStr, "🖼 多模态")
	}
	if len(capStr) > 0 {
		result.WriteString(fmt.Sprintf("  能力: %s\n", strings.Join(capStr, "、")))
	} else {
		result.WriteString("  能力: 未指定\n")
	}

	return result.String(), nil
}

// addModel adds a new model configuration.
func (h *ModelHandler) addModel(args []string) (string, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("用法: .model add <供应商> <模型> [--endpoint <url>] [--api-key <key>] [--priority <n>]")
	}

	provider := args[0]
	modelName := args[1]

	var endpoint, apiKey string
	var priority int = 50

	for i := 2; i < len(args); i++ {
		switch args[i] {
		case "--endpoint":
			if i+1 < len(args) {
				endpoint = args[i+1]
				i++
			}
		case "--api-key":
			if i+1 < len(args) {
				apiKey = args[i+1]
				i++
			}
		case "--priority":
			if i+1 < len(args) {
				p, err := strconv.Atoi(args[i+1])
				if err == nil {
					priority = p
				}
				i++
			}
		}
	}

	if endpoint == "" {
		endpoint = getProviderEndpoint(provider)
	}

	modelID := fmt.Sprintf("%s-%s", provider, strings.ReplaceAll(modelName, "/", "-"))

	for _, m := range h.cfg.Models {
		if m.ID == modelID {
			return "", fmt.Errorf("模型 %s 已存在", modelID)
		}
	}

	newModel := &config.ModelConfig{
		ID:           modelID,
		Name:         fmt.Sprintf("%s (%s)", provider, modelName),
		Provider:     provider,
		Endpoint:     endpoint,
		Model:        modelName,
		APIKey:       apiKey,
		Priority:     priority,
		Enabled:      false,
		Capabilities: config.ModelCapability{ToolCall: true},
	}

	h.cfg.Models = append(h.cfg.Models, newModel)

	if err := h.cfg.Save(); err != nil {
		return "", fmt.Errorf("保存配置失败: %w", err)
	}

	log.Info("Added new model: %s (provider=%s, model=%s)", modelID, provider, modelName)
	return fmt.Sprintf("✅ 已添加模型: %s (%s)", modelID, modelName), nil
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

	result.WriteString("  使用 .model from-tpl <模板ID> <模型ID> 从模板添加模型\n")
	return result.String(), nil
}

// getProviderEndpoint returns the default endpoint for a provider.
func getProviderEndpoint(provider string) string {
	manager := config.GetDefaultModelManager()
	template := manager.GetTemplate(provider)
	if template != nil {
		return template.Endpoint
	}
	return ""
}

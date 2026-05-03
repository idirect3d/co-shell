// Author: L.Shuang
package config

import (
	"fmt"
	"os/exec"
	"runtime"
)

// ProviderPreset defines a preset configuration for an LLM provider.
type ProviderPreset struct {
	Name         string
	DisplayName  string
	Endpoint     string
	Models       []string
	DefaultModel string
	APIKeyURL    string // URL to open in browser for API key
}

// Built-in provider presets
var ProviderPresets = []ProviderPreset{
	{
		Name:         "deepseek",
		DisplayName:  "DeepSeek",
		Endpoint:     "https://api.deepseek.com",
		Models:       []string{"deepseek-v4-flash", "deepseek-v4-pro"},
		DefaultModel: "deepseek-v4-flash",
		APIKeyURL:    "https://platform.deepseek.com/api_keys",
	},
	{
		Name:         "qwen",
		DisplayName:  "阿里千问（通义千问）",
		Endpoint:     "https://dashscope.aliyuncs.com/compatible-mode/v1",
		Models:       []string{"qwen-plus", "qwen-max", "qwen-turbo"},
		DefaultModel: "qwen-plus",
		APIKeyURL:    "https://bailian.console.aliyun.com/?apiKey=1#/api-key",
	},
	{
		Name:         "xiaomi",
		DisplayName:  "小米 MiMo 大模型",
		Endpoint:     "https://api.xiaomimimo.com/v1",
		Models:       []string{"mimo-v2.5-pro", "mimo-v2.5", "mimo-v2-pro", "mimo-v2-omni", "mimo-v2-flash"},
		DefaultModel: "mimo-v2.5-pro",
		APIKeyURL:    "https://platform.xiaomimimo.com/#/console/api-keys",
	},
	{
		Name:         "zhipu",
		DisplayName:  "智谱 AI（GLM）",
		Endpoint:     "https://open.bigmodel.cn/api/paas/v4/",
		Models:       []string{"glm-4-plus", "glm-4-0520", "glm-4-air", "glm-4-flash", "glm-4v-plus"},
		DefaultModel: "glm-4-plus",
		APIKeyURL:    "https://bigmodel.cn/usercenter/proj-mgmt/apikeys",
	},
	{
		Name:         "ollama",
		DisplayName:  "Ollama（本地部署）",
		Endpoint:     "http://localhost:11434/v1",
		Models:       []string{},
		DefaultModel: "",
		APIKeyURL:    "",
	},
	{
		Name:         "openai-compatible",
		DisplayName:  "OpenAI 兼容（自定义）",
		Endpoint:     "",
		Models:       []string{},
		DefaultModel: "",
		APIKeyURL:    "",
	},
}

// FindProvider finds a provider preset by name.
func FindProvider(name string) *ProviderPreset {
	for _, p := range ProviderPresets {
		if p.Name == name {
			return &p
		}
	}
	return nil
}

// OpenURL opens a URL in the default browser.
func OpenURL(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "linux":
		cmd = "xdg-open"
		args = []string{url}
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler", url}
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return exec.Command(cmd, args...).Start()
}

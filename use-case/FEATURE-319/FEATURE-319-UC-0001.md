# FEATURE-319: 视觉模型上下文控制（vision-context-mode）

## 现象
主模型上下文限制大于视觉模型时，`visual_analysis` 把图片缓存在 `imagePaths`，下一轮主循环切换视觉模型并发送**完整上下文**（system + 全部历史 + 最后 user 含图片），可能超出视觉模型上下文限制导致请求失败。

## 方案
新增 `vision-context-mode` 配置参数（`minimal`/`full`，默认 `minimal`）：
- `minimal`：视觉模型只收到 `[system, user(识别 intent 指令 + 图片)]`，丢弃中间历史，避免上下文超限
- `full`：保持现有行为（完整历史 + 最后 user 消息）

`visual_analysis` 工具的 `intent` 参数作为 minimal 模式的识别指令。

## 修复（config/config.go + agent/loop.go + agent/image_tools.go + main.go + cmd/settings.go）
1. `config.go`：LLMConfig 新增 `VisionContextMode string`（minimal/full，默认 minimal）+ 默认值 + 校验
2. `image_tools.go`：`visualAnalysisTool` 解析 intent 后存入 `a.visionPendingIntent`
3. `loop.go`：Agent struct 新增 `visionPendingIntent string`；`buildContextMessages()` 图片注入后、返回前——若 `VisionContextMode == "minimal"` 且有待处理图片且 intent 非空，则折叠为 `[system, user(intent + 图片 ContentParts)]`
4. `main.go`：`--vision-context-mode` CLI 参数 + 校验 + 应用
5. `cmd/settings.go` + `agent/settings_tools.go`：`:set vision-context-mode` 注册/校验/显示
6. i18n：zh/en 参数说明 key；usage.go：--help 补行

## 用例
1. minimal 模式（默认）：有待处理图片时，buildContextMessages 返回 `[system, user(intent+图片)]`，无中间历史、无 tool/assistant
2. minimal 模式 + intent 为空：回退现有行为（不折叠，避免丢失图片）
3. full 模式：保持现有行为（完整历史 + 最后 user）
4. minimal 模式 + 无图片：不触发折叠（普通文本轮次不受影响）
5. `:set vision-context-mode full` / minimal 可切换并持久化到 config.json
6. `--vision-context-mode` CLI 参数生效；非法值报错
7. 回归：visual_analysis 工具 intent 仍作为任务内容进入 taskInstructionCache（full 模式路径不变）
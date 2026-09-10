# FEATURE-503 测试用例：RESULT MODE 节末尾追加 --unload-mode 配置说明

> 分支：FEATURE-501（v0.48.0）
> 任务：在系统提示词 RESULT MODE 节（标题 ACT MODE V.S. PLAN MODE V.S. RESEARCH MODE）末尾追加一句括号说明，让 co-shell 自己知道各模式策略可通过 `--unload-mode {mode}` 导出到 `./mode/` 下并实时编辑调整。

## 需求规格

在 RESULT MODE 节所有模式描述之后，追加一句用括号包住的说明：

- 中文：`（以上内容可以通过 --unload-mode {mode}，将各模式的策略导出到 ./mode/ 下，可以通过编辑这些文件进行实时调整）`
- 英文：`(The above can be exported per mode with --unload-mode {mode} into ./mode/, and these files can be edited for real-time adjustment)`

## UC-0001 中文环境下末尾说明存在且位置正确

**前置**：`i18n.Init("zh")`；`cfg := config.DefaultConfig()`；`cfg.WorkModes = nil`；`cfg.LLM.WorkMode = "act"`。

**操作**：调用 `buildResultModeSection(cfg)`。

**预期**：
- 返回内容包含 `--unload-mode {mode}`。
- 该说明位于 `# RESEARCH MODE` 之后（即所有模式描述之后）。
- 说明被中文括号 `（` 包裹。

**验证方式**：单测 `TestBuildResultModeSection_TrailingNote/zh` 断言。

## UC-0002 英文环境下末尾说明存在且位置正确

**前置**：`i18n.Init("en")`；其余同 UC-0001。

**操作**：调用 `buildResultModeSection(cfg)`。

**预期**：
- 返回内容包含 `--unload-mode {mode}`。
- 该说明位于 `# RESEARCH MODE` 之后。
- 说明被括号包裹。

**验证方式**：单测 `TestBuildResultModeSection_TrailingNote/en` 断言。

## UC-0003 现有 ResultMode 行为不回归

**前置**：`i18n.Init("en")`；默认配置。

**操作**：运行 `go test ./agent/ -run TestBuildResultModeSection -v`。

**预期**：
- 原有 UC-0001~UC-0006 相关测试（默认模式、自定义模式、覆盖内置模式、跨模式静态化等）全部通过。
- 新增 `TestBuildResultModeSection_TrailingNote` 通过。

**验证方式**：`go test ./agent/ -run TestBuildResultModeSection -v` 全绿。

## UC-0004 运行时用例（REPL 验证）

**前置**：编译最新可执行码到 `~/bin/co-shell`。

**操作**：启动 co-shell，查看系统提示词中 RESULT MODE 节内容。

**预期**：
- RESULT MODE 节末尾出现括号说明，提示 `--unload-mode {mode}` 与 `./mode/` 目录。
- 说明语言与当前界面语言一致（中文界面显示中文说明）。

**验证方式**：人工观察系统提示词输出。

## UC-0005 编译与静态检查

**操作**：`go build ./... && go vet ./...`。

**预期**：无编译错误、无 vet 告警。

**验证方式**：命令退出码为 0。

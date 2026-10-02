# FEATURE-579 UC-0009：9 项开关的设置界面与 i18n

> 分支：**FEATURE-579**　版本：**v0.88.0**　对应方案：**A/B/C/D1 配置项落地（§8 + §9 配置行）**
> 判定口径：9 项新开关在 `:set` 清单、Web 设置页**可读可写**，非法值被拒；`config.json` 持久化生效；zh/en 文案齐备；**默认开启**。

## 目的

保证新增配置项不只是代码里的开关，而是用户可发现、可修改、可校验的一等设置项（含多语言）。

## 设计基线

| 配置项 | 类型 | 默认 | 归属分组 |
|---|---|---|---|
| `env_history_prune` | bool | true | Agent / LLM 上下文 |
| `env_history_full_keep` | int | 1 | 同上 |
| `env_history_lite_keep` | int | 1 | 同上 |
| `env_tool_batch_single` | bool | true | 同上 |
| `env_board_roster_brief` | bool | true | 同上 |
| `env_dir_change_detect` | bool | true | 同上 |
| `env_runtime_info_once` | bool | true | 同上 |
| `tool_desc_compact` | bool | true | 工具 |
| `tool_lazy_load` | bool | true | 工具（本期无实现，仅登记开关） |
| `read_file_max_size` | int | **0 → 65536** | 工具（默认值变更） |
| `context_reorganize_threshold` | int | **80 → 92** | LLM（默认值变更；`config.json` 现值 90 需同步为 92） |

代码接入点（§9 配置行）：`config/config.go`、`cmd/settings_agent.go`、`cmd/settings_web.go`、`cmd/settings.go`；i18n 文案：`i18n/zh*.go`、`i18n/en*.go`。

## 前置条件

- 当前分支 `FEATURE-579`，已实现配置字段、设置项注册与 i18n 文案。
- 测试：`TestTokenOptSettings`（config/cmd）、`TestTokenOptKeysI18n`（i18n）。测试须在临时目录复制 config.json 操作，**不得改动仓库内 `config.json`**。

## 用例 A：默认值正确

```bash
cd /Users/direct3d/github/co-shell
go test ./config -run TestTokenOptSettings -v -count=1 2>&1 | grep -E 'DEFAULT\|'
```

**期望**：9 项 bool 默认均为 `true`；`env_history_full_keep=1`、`env_history_lite_keep=1`；`read_file_max_size=65536`；`context_reorganize_threshold=92`。

## 用例 B：`:set` 清单可读可写

```bash
go run . run --prompt ":set list" 2>&1 | grep -E 'env_history_prune|env_tool_batch_single|env_board_roster_brief|env_dir_change_detect|env_runtime_info_once|tool_desc_compact|tool_lazy_load|read_file_max_size|env_history_full_keep'
```

**期望**：9 项全部出现在 `:set` 清单中（含当前值与说明）；执行 `:set env_history_prune=false` 后再 `:set list` 显示 `false`，且写入临时 config 文件（临时目录内验证，不污染仓库配置）。

## 用例 C：非法值被拒

```bash
go test ./config -run TestTokenOptSettings -v -count=1 2>&1 | grep -E 'INVALID\|'
```

**期望**：`env_history_full_keep=-1`、`env_history_lite_keep=-1`、`read_file_max_size` 非整数、bool 项填 `yes/no` 等非法输入被拒绝并给出多语言错误提示；拒绝后原值保持不变。

## 用例 D：Web 设置页可读可写

```bash
go run . --serve --port 28310 &
# 浏览器/接口验证（用 browser_* 工具或 curl 设置页 API）
go test ./cmd -run TestTokenOptSettingsWeb -v -count=1
```

**期望**：Web 设置页对应分组（Agent/工具）出现 9 项控件，值为默认值；修改任一项并保存后，后端 config 生效（重新读取配置可见新值）；**页面不得出现硬编码内网 IP/密钥**。

## 用例 E：i18n 文案齐备

```bash
go test ./i18n -run TestTokenOptKeysI18n -v -count=1 2>&1 | grep -E 'I18N\|'
```

**期望**：zh 与 en 两套文案对 9 项开关（标签 + 说明）**全部存在且非空**，无缺失 key、无中文混入 en（反向亦然）；`go test ./i18n/... -count=1` 通过。

## 判定

- A~E 全通过 → 本 UC 通过；E 缺失文案属交付不完整（必须补齐 zh/en 后再交付）。

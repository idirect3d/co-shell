# FEATURE-579 UC-0008：D1 工具定义瘦身（meta 压缩 + 描述去重 + read_file_max_size）

> 分支：**FEATURE-579**　版本：**v0.88.0**　对应方案：**D1（工具定义瘦身，非懒加载）**
> 依据：`research/token优化设计方案.md` §7.1、§8
> 判定口径：`tool_desc_compact=true` 时工具 schema 总体积 **≥-30%**、`meta` 参数描述总字节 **≤4,000 B**；`read_file_max_size` 默认值 **65536** 生效；关闭开关回归现状描述。

## 目的

消除 60+ 工具重复的 `meta` 长描述（基线 38×9,158 B，占 13.7%）与 subagent 工具重复解释 `<sub_agents>`/`<environment_details>` 的开销，并给单次文件读取加上限。

## 设计基线

| 项 | 基线 | 目标 |
|---|---|---|
| 工具 schema 总量 | 38 工具 / 66,657 B（生产 69 工具 / 98,576 B） | ≥-30% |
| `meta` 描述 | 38 × 9,158 B（每个 ~330 字符） | 压缩为一行（~80 字符），总量 ≤4,000 B |
| 完整 meta 说明 | 散落 60+ 处 | 移入系统提示词一处（行为不变） |
| subagent 工具描述 | 4 个工具互相重复解释 `<sub_agents>`/`<environment_details>`/`meta` | 各自一句 + 系统提示词统一说明 |
| `read_file_max_size` | `0`（无上限，`config/config.go` 默认） | `65536`（与 `tool_result_max_size` 对齐） |
| XML 模式 | `ToolDeclarations()`（`agent/toolcall_mode.go:2032`）共用同一份 `[]llm.Tool` | 自动同享收益 |

## 前置条件

- 当前分支 `FEATURE-579`，已实现 D1；开关 `tool_desc_compact` 可配（默认 true）。
- 测试：`TestToolDescCompactSchema`、`TestMetaDescriptionSize`、`TestReadFileMaxSizeDefault`、`TestToolDescCompactDisabled`。

## 用例 A：schema 体积与 meta 描述达标

```bash
cd /Users/direct3d/github/co-shell
go test ./agent -run 'TestToolDescCompactSchema|TestMetaDescriptionSize' -v -count=1 2>&1 | grep -E 'SCHEMA\||META\|'
```

**期望**：`SCHEMA|total=…|delta=-X%` 且 `X ≥ 30`；`META|count=38|bytes=…` 且 `bytes ≤ 4000`；每个 `meta` 参数 description 为**单行**（不含换行）。

## 用例 B：完整 meta 说明仍在系统提示词中（行为不回归）

```bash
go test ./agent -run TestToolDescCompactSchema -v -count=1 2>&1 | grep -E 'SYSPROMPT_META'
go test ./agent -run 'TestToolDeclarationsXML|TestToolDeclarationsOpenAI' -v -count=1
```

**期望**：系统提示词中存在唯一一处 meta 完整说明（`>0`）；XML 与 openai 两种模式的工具声明均来自同一份 `[]llm.Tool`，两模式 schema 体积一致。

## 用例 C：`read_file_max_size` 默认 65536

```bash
go test ./agent -run TestReadFileMaxSizeDefault -v -count=1 2>&1 | grep -E 'READLIMIT\|'
grep -n 'read_file_max_size' config/config.go config.json
```

**期望**：默认值为 `65536`；读取 >64KB 文件时被截断并给出"已保存完整内容到 `tmp/tool-result/`"提示（与 `tool_result_max_size` 行为一致）；`config.json` 中若显式配置 0 则仍按 0（无上限）处理，不被强制覆盖。

## 用例 D：关闭开关回归

```bash
go test ./agent -run TestToolDescCompactDisabled -v -count=1 2>&1 | grep -E 'SCHEMA\|compact=false'
```

**期望**：`tool_desc_compact=false` 时 schema 字节数与基线（66,657 B，38 工具口径）一致，`meta` 描述回到原长文本。

## 判定

- A/B/C/D 全通过 → 本 UC 通过；A 未达标时须给出实测差值并继续优化（不得只改口径）。

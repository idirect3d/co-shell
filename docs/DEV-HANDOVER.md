# co-shell 开发交接文件（0.7.x 输出架构重构）

> 用途：新会话在新上下文中继续开发。只需读本文 + docs/output-architecture.md 即可恢复全部上下文。
> 创建：2026-08-01 17:00 | 当前分支：FEATURE-302 | 版本：v0.7.0（BUILD-340）

---

## 一、当前任务

**FEATURE-302（P2）Out + RenderCommand 抽象 + 渲染合并**：统一输出通道，引入 RenderCommand 渲染指令（架构文档 3.7），消除 REPL/main 双渲染。
（完整设计见 docs/output-architecture.md 3.2/3.7/6.2；P1 已为 P2 铺好 13 事件常量 + 双 golden 基线）

## 二、已完成状态（FEATURE-301 交付）

- **git**：FEATURE-301 已完成并合并回 main（[BUILD-340]）；FEATURE-302 分支已建
- **测试用例**：use-case/FEATURE-301/FEATURE-301-UC-0001.md（12 用例，全部通过）
- **验收结果**（bin/output_audit.sh）：fmt=206 / 魔法事件=**63→0** / 中文=1555 / 同步输入=19 / i18n=1（仅第 2 项变化，行为零变化达成）
- **新增回归资产**：
  - agent/events.go（13 输出事件常量）+ agent/input.go（12 InputKind 常量）
  - agent/events_test.go（UC-0003/0004 常量 table 测试）
  - repl/render_test.go + repl/testdata/render_tui.golden（REPL 渲染 golden）
  - render_test.go + testdata/render_single_cmd.golden（单命令渲染 golden）
  - main.go renderSingleCmdEvent 已从 executeSingleCommand 闭包机械提取（P2 渲染合并的接入点）
- **已知既有问题（非本任务引入）**：agent/file_tools_test.go 的 read_file/search_files/write_to_file 用例失败（参数校验变更未同步测试，与 FEATURE-265 相关），baseline 分支同样失败

## 三、版本计划（0.7.x 系列，ROADMAP 已登记）

| 任务 | 版本 | 阶段 | 内容 |
|------|------|------|------|
| FEATURE-301 | 0.7.0 | P1 | 事件双枚举（当前） |
| FEATURE-302 | 0.7.0 | P2 | Out + RenderCommand 抽象 + 渲染合并 |
| FEATURE-303 | 0.7.0 | P3 | 向导迁移（B 类）+ i18n 归零第一步 |
| FEATURE-304 | 0.7.0 | P4 | 外部入口迁移 + 分类开关 |
| FEATURE-305 | 0.7.0 | P4.5 | i18n 归零冲刺 |
| FEATURE-306 | 0.7.1 | P2.5 | 输入统一 InputSource + Windows 补齐 |
| FEATURE-307 | 0.7.2 | P5 | tui v1 + web 原型 |
| FEATURE-308 | 0.7.3 | tui v2 | 全屏 TUI（可选） |

任务编号：FIX-300 后最小编号 FEATURE-301，后续 302/303... 递增。

## 四、关键架构摘要（详见 output-architecture.md）

- **输出**：Out 接口（ChannelID×Level）+ RenderCommand（UI 组件只发布语义指令，行/全屏/流式/Web 只是不同 Renderer）
- **输入**：InputSource/InputEvent（单一 Reader 循环，三类消费者）；`--input-mode` stdio/tui/web，enhanced=tui 别名
- **零新增依赖**：gorilla/websocket（已有）、x/sys（已有）、标准库；全屏用原生 ANSI，禁 tview/tcell
- **重要**：审计发现 KeyToolUsageShellSend 缺 zh 翻译（FEATURE-305 修复）

## 五、关键文件地图

- docs/output-architecture.md（设计全文）/ docs/output-inventory.md（现状清单）
- bin/output_audit.sh（5 项审计，--strict 作门禁）
- agent/loop.go + run_stream.go + stream_response.go（~63 处 cb 待常量化）
- repl/repl.go + main.go（streamCallback switch 待改常量）
- agent/io.go（UserIO 接口）/ repl/userio.go（StdioIO/EnhancedIO 实现）
- repl/repl_esc_posix.go + windows.go（ESC 监控；Windows 是 no-op）
- config/config.go（EmojiPrefixes 15 前缀 + show-xx 7 开关）
- i18n/keys.go + zh.go/en.go + *_loop/system.go（139+ key）
- ROADMAP.md（v0.7.0 段已建）

## 六、开发规范（必须遵守）

1. task.md 流程：确认 main→版本计划→最小编号→分支→use-case→用户确认→dev.md
2. 禁 main 直接提交；功能改动在分支验证后合并
3. 源代码只改 replace_in_file/write_to_file；新文件 write_to_file
4. 用户交互走 agent.UserIO/GetIO，禁直接 fmt.Print/os.Stdin
5. 新文案走 i18n.T()/TF()，新增 key 同步 zh+en
6. 每任务跑 `bin/output_audit.sh --strict`
7. Go 1.22+、错误显式、GoDoc、单文件<500行、注释英文
8. 零新增依赖

## 七、下一步行动（新会话顺序）

1. `git status` 确认在 FEATURE-302 分支
2. 读 docs/output-architecture.md（3.2 Out 接口 / 3.7 RenderCommand / 6.2 input-mode 别名）+ use-case/FEATURE-302 用例（新建）
3. **向用户确认测试用例**（门禁）
4. 开发：新增 agent/out.go（Out + TerminalOut）→ agent/command.go（RenderCommand/RenderKind）→ repl.go streamCallback 与 main.go renderSingleCmdEvent 合并为「事件→渲染器」单一入口（LineRenderer 语义）→ --input-mode enhanced→tui 别名（parseInputMode）
5. 验收：`bin/output_audit.sh` 全项不变；两份 golden 仍逐字节一致；`:set` 开关生效回归；`--input-mode stdio` 无 ANSI 污染；REPL 冒烟（需 API Key）

## 八、风险

- P2 是渲染逻辑合并，golden 基线已就绪兜底，禁夹带行为改动（golden 有差异即为回归）
- `--input-mode enhanced→tui` 别名需保留老配置兼容（config.json 旧值自动归一化，不回写）
- P2 只做 Out/TerminalOut + RenderCommand 抽象，不迁移向导（P3 才做）
- 新会话语境预算：先读本文 + 架构文档，避免重复摸底全库

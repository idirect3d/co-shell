# co-shell 开发交接文件（0.7.x 输出架构重构）

> 用途：新会话在新上下文中继续开发。只需读本文 + docs/output-architecture.md 即可恢复全部上下文。
> 创建：2026-08-02 11:00 | 更新：2026-08-02 15:26 | 当前分支：main（FEATURE-304 已合并） | 版本：v0.7.0（BUILD-343）

---

## 一、当前任务

**FEATURE-305（P4.5）i18n 归零冲刺**：清空剩余硬编码中文，达成 100% i18n 硬性目标；按 inventory A-3 清单逐条迁移（A 类残余 + C 类 main.go 警告），同步补齐 keys/zh/en 翻译；修复 KeyToolUsageShellSend 缺 zh 翻译 bug。
（完整设计见 docs/output-architecture.md 4.5；P4 已完成 audit 901→772，剩余 B/C 类残余）

## 二、已完成状态（FEATURE-301 + FEATURE-302 交付）

- **git**：FEATURE-301 [BUILD-340] + FEATURE-302 [BUILD-341] 均已完成并合并回 main（cd57077）；FEATURE-303 分支待建
- **测试用例**：FEATURE-301（12 用例）+ FEATURE-302（12 用例）全部通过
- **验收结果**（bin/output_audit.sh，P2 后）：
  - fmt=**206→204**（渲染合并消除 2 处直接 fmt，合规改进）
  - 魔法事件=**63→0**（P1）/ 中文=1555 / 同步输入=19 / i18n=1（P1/P2 均不变）
- **P2 新增资产**（P3 直接可复用）：
  - agent/out.go：Out 接口（Emit + Info/Success/Warning/Error/Debug）+ ChannelID(12) + Level(5) + TerminalOut（基于 EmojiPrefixes + UserIO）
  - agent/command.go：RenderCommand + RenderKind(8)
  - agent/stream_renderer.go：StreamRenderer（REPL/main 渲染合并，StreamModeREPL/SingleCmd）
  - config.NormalizeInputMode：--input-mode/config enhanced→tui 别名，加载归一化不回写
- **P3 目标**：audit 第 3 项 Hardcoded Chinese 1555 下降约 1/3（约 -500）；向导功能回归
- **已知既有问题（非本任务引入）**：agent/file_tools_test.go 的 read_file/search_files/write_to_file 用例失败（与 FEATURE-265 相关），baseline 分支同样失败

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

1. `git status` 确认在 main 分支且干净；建 FEATURE-303 分支（`git checkout -b FEATURE-303`）
2. 读 docs/output-architecture.md（P3 段 + 3.2 Out + 6.9 UI 快捷键）+ docs/output-inventory.md（B 类硬编码清单）
3. 建 use-case/FEATURE-303/FEATURE-303-UC-0001.md（循环模式）→ **向用户确认测试用例**（门禁）
4. 开发：定义 UI 组件 Out.Box/Menu/Step/Sep（绑定规范快捷键 [B]/[C]/[E]/[D]/[Q]/[数字]）→ 迁移 cmd/model.go（113 处）→ mode.go（161）→ config.go（104）→ settings_db.go（48）→ session.go（30）；B 类文案同步迁移 i18n
5. 验收：各向导功能回归（cmd/settings_mode_dirs_test.go 等现有测试 + 手动走一遍向导）；audit 第 3 项 Hardcoded Chinese 较基线 1555 下降约 1/3

## 八、风险

- P3 迁移量大（5 handler，~450 处），逐 handler 迁移 + 每步 `go build ./...` + 现有 cmd 测试兜底，禁一次改完
- UI 组件必须绑定规范快捷键，禁止各向导自定（架构 6.9）
- i18n 新 key 必须同步 zh+en（audit 第 5 项跟踪）
- 新会话语境预算：先读本文 + 架构文档 + inventory 清单，避免重复摸底全库

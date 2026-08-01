# co-shell 开发交接文件（0.7.x 输出架构重构）

> 用途：新会话在新上下文中继续开发。只需读本文 + docs/output-architecture.md 即可恢复全部上下文。
> 创建：2026-08-01 16:30 | 当前分支：FEATURE-301 | 版本：v0.7.0（BUILD-339）

---

## 一、当前任务

**FEATURE-301（P1）事件双枚举重构**：将事件魔法字符串 `cb("content_chunk")` 等 13 种输出事件 + 输入事件改为常量引用。纯重构、行为零变化。
（完整设计见 docs/output-architecture.md 3.3/3.6；13 事件 = content_chunk/thinking_chunk/content/thinking/command/output/tool_call/token_iter/token_task/info/warning/error/done）

## 二、已就绪状态

- **git**：main 已提交调研文档（2eb4ecc）+ 版本计划（35fd19c）；FEATURE-301 分支已建
- **测试用例**：use-case/FEATURE-301/FEATURE-301-UC-0001.md（12 用例，**待用户确认后开发**）
- **验收基线**（bin/output_audit.sh）：fmt=206 / 魔法事件=**63** / 中文=1555 / 同步输入=19 / i18n=1
- **验收标准**：仅魔法事件 63→0，其余 4 项不变；go build + go test 全绿；golden 渲染快照无差异

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

1. `git status` 确认在 FEATURE-301 分支
2. 读 docs/output-architecture.md（架构细则）+ use-case/FEATURE-301 用例
3. **向用户确认测试用例**（门禁）
4. 开发：先录 golden 基线（main 行为）→ 建 agent/events.go + input.go → 常量化 63 处 cb + switch → go build/test/audit（期望仅第 2 项 63→0）
5. 验收通过后：ROADMAP 标记 FEATURE-301 [BUILD-34X] → 合并回 main → 建 FEATURE-302 分支

## 八、风险

- P1 核心文件大 diff，golden + audit 兜底，禁夹带行为改动
- P2 才做 `--input-mode enhanced` 别名，P1 不动配置
- ROADMAP 末尾有孤立 `## v0.6.0 — Beta3` 标题残留（replace_in_file 解析失败导致），可顺手清理
- 新会话语境预算：先读本文 + 架构文档，避免重复摸底全库
# FEATURE-330 工作区配置外部化测试用例

| 项目 | 内容 |
|------|------|
| 任务编号 | FEATURE-330 |
| 任务名称 | 工作区配置外部化：PRINCIPLES.md 覆盖 + .rules/ 目录规则 + .rule 生效修复 |
| 类型 | 特性（系统提示词外部化） |
| 版本 | v0.7.2 |
| 依赖 | FEATURE-195（系统提示词外部化）、FEATURE-299（{AGENT_PRINCIPLES} 占位符）、FIX-315（:rule 进入系统提示词） |

**目标**：① 工作区根目录存在 `PRINCIPLES.md` 时，完全替代 config.json 的 `llm.agent_principles`（查找顺序 workspacePath → cwd）；② 工作区 `.rules/` 目录下所有 `.md` 文件按文件名排序合并到 RULES 节 `{CUSTOM_RULES}`，每个文件以 `# {文件名}` 开头标明规则领域；③ 修复 `.rule` 命令修改不生效——rules 改为每次从 `cfg.Rules` 现场重建，与 `.rules/` 目录修改均在下一次 RunStream 迭代立即生效。

**验收基线**：`go test ./agent/...` 全绿；`go vet ./agent/...` 0 告警；`go build ./...` 通过；新增测试全部通过。

---

## 一、编译与回归

- **UC-0001**: `go build ./...` 退出码 0（含 cmd/co-shell-feishu-bridge、cmd/co-shell-hub）。
- **UC-0002**: `go test ./agent/...` 无 FAIL（含既有测试 + 本任务新增测试）。
- **UC-0003**: `go vet ./agent/...` 0 告警。

## 二、loadRulesDir 纯函数（循环模式）

- **UC-0004**: 多文件排序——目录含 `b.md`/`a.md`/`c.md`，输出按文件名升序排列（断言 a 在前、c 在后），每个文件格式为 `# {文件名}\n\n{内容}`，文件间空行分隔。
- **UC-0005**: 非 .md 忽略——目录含 `a.md`/`notes.txt`/`b.MD`（大写扩展名），只加载 a.md/b.MD 或仅小写 .md（table-driven 子用例，断言非 .md 不进输出）。
- **UC-0006**: 空/不存在目录——返回空字符串（table-driven：目录不存在 / 目录存在但无 .md / 目录为空）。
- **UC-0007**: 空文件内容——文件存在但内容为空（或仅空白），该文件不产生 `# {文件名}` 头（避免空领域）。

## 三、resolveAgentPrinciples 优先级链（循环模式）

- **UC-0008**: PRINCIPLES.md 存在（workspacePath）→ 使用文件内容，忽略 cfg.AgentPrinciples（断言返回文件内容）。
- **UC-0009**: workspacePath 无 PRINCIPLES.md 但 cwd 有 → 使用 cwd 文件内容（断言回退链）。
- **UC-0010**: 无 PRINCIPLES.md（双目录均无）→ 使用 cfg.LLM.AgentPrinciples 配置值。
- **UC-0011**: 无文件 + 配置为空 → i18n `KeyAgentDefaultPrinciples` 默认值。
- **UC-0012**: 无文件 + 配置为空 + i18n 返回 key 本身（翻译缺失）→ 返回空串，杜绝 key 名泄漏。
- **UC-0013**: PRINCIPLES.md 存在但内容空白 → 回退 cfg.LLM.AgentPrinciples（与 loadExternalFile 空串语义一致）。

## 四、resolveRules 合并优先级（循环模式）

- **UC-0014**: 仅 config rules（无 .rules/ 目录）→ 返回 `strings.Join(cfg.Rules, "\n")`，与旧行为一致（table-driven：1 条 / 3 条 / 0 条）。
- **UC-0015**: 仅 .rules/ 目录（无 config rules）→ 返回 loadRulesDir 输出。
- **UC-0016**: config rules + .rules/ 同时存在 → config rules 在前、.rules/ 内容在后，中间空行分隔。
- **UC-0017**: .rules/ 目录存在但为空 → 等价于仅 config rules。
- **UC-0018**: workspacePath 与 cwd 均有 .rules/ → workspacePath 优先（不回退 cwd）。

## 五、集成行为（rebuildSystemPrompt 生效）

- **UC-0019**: `.rule add/remove/clear` 修改 cfg.Rules 后调用 rebuildSystemPrompt()，新系统提示词的 `{CUSTOM_RULES}` 反映最新 cfg.Rules（修复前：a.rules 快照不变 → 断言修复后生效，table-driven：add/remove/clear 三子用例）。
- **UC-0020**: .rules/ 目录文件修改后调用 rebuildSystemPrompt()，新提示词包含最新文件内容（模拟修改 a.md 后重建，断言新内容进入 {CUSTOM_RULES}）。
- **UC-0021**: PRINCIPLES.md 放置/修改后调用 rebuildSystemPrompt()，Identity 节 `{AGENT_PRINCIPLES}` 反映文件内容（断言文件内容出现在系统提示词 Identity 节）。
- **UC-0022**: SetResultMode() 路径同样生效——切换 result-mode 触发重建后，PRINCIPLES.md 与 .rules/ 内容均正确进入（与 rebuildSystemPrompt 同源）。

## 六、审计收尾

- **UC-0023**: git diff 限 agent/rules_test.go（新增）+ agent/agent.go + agent/system_prompt.go；无越界文件。
- **UC-0024**: 全量 `go test ./...` 通过（不破坏既有渲染 golden 测试、i18n、loop 相关测试）。

## 循环模式

UC-0004（3 文件排序断言）+ UC-0005（2）+ UC-0006（3）+ UC-0007（1）+ UC-0008（1）+ UC-0009（1）+ UC-0010（1）+ UC-0011（1）+ UC-0012（1）+ UC-0013（1）+ UC-0014（3）+ UC-0015（1）+ UC-0016（1）+ UC-0017（1）+ UC-0018（1）+ UC-0019（3）+ UC-0020（1）+ UC-0021（1）+ UC-0022（1）≥ 28 子用例。
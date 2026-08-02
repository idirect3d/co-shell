# FEATURE-304 外部入口迁移 + 分类开关（P4）测试用例

| 项目 | 内容 |
|------|------|
| 任务编号 | FEATURE-304 |
| 任务名称 | 外部入口迁移 + 分类开关（P4） |
| 类型 | 重构（输出通道统一 + 分类开关 + i18n） |
| 版本 | v0.7.0 |
| 架构文档 | docs/output-architecture.md（P4 段） |

**目标**：① 新增 `config.LLM.OutputCategories` 分类开关（:set / --output-categories CLI / config.json 持久化）；② feishu/bridge/hub/subagent 外部入口 i18n 化 + 主程序侧 ChannelBridge/ChannelSubAgent 渲染受开关控制；③ D 类 + A 类约 30 处硬编码中文迁移。
**验收基线**（P3 后 audit）：fmt=204 / 魔法事件=0 / 中文=901 / 同步输入=19 / i18n=1。
**验收目标**：中文 901 下降约 120+（≤780）；`go vet ./cmd/` 0 告警；`go test ./cmd/...` 全绿；D 类入口可通过开关关闭；feishu-bridge 独立运行不受影响。
**已确认基线**：本任务在分支 FEATURE-304 上实测 audit = 901 / fmt = 204 / events = 0 / sync = 19 / i18n = 1。

---

## 一、编译与回归

- **UC-0001**: `go build ./...` 退出码 0（含 cmd/co-shell-feishu-bridge、cmd/co-shell-hub）。
- **UC-0002**: `go test ./cmd/...` 无 FAIL（既有 file_tools 失败已知例外，不属本任务）。
- **UC-0003**: `go vet ./cmd/...` 0 告警（P3 教训：无参 `fmt.Errorf/Printf(i18n.T(key))` 必须改用 `errors.New`/`fmt.Print`；含 % 占位符的保留 fmt）。

## 二、OutputCategories 分类开关

- **UC-0004**: `config.LLM.OutputCategories` 字段存在，默认值含 wizard/system/db/bridge 全部开启；JSON 序列化 round-trip 一致（table-driven：默认 / on 子集 / off 全部）。
- **UC-0005**: `TerminalOut.Emit` 按 OutputCategories 过滤——`bridge` 关闭时 ChannelBridge 输出隐藏、其余 channel 不受影响；重开恢复（table-driven：5 个代表 channel × on/off = 10 子用例）。
- **UC-0006**: `:set output-categories` 无参显示当前状态（枚举列出 wizard/system/db/bridge 各 on/off）；`on|off` 修改并写入 config.json；重启后持久化生效（参数化 3 子用例）。
- **UC-0007**: `--output-categories` CLI 覆盖配置文件；`main.go --help` 文案同步包含该参数。

## 三、D 类外部入口 i18n 迁移

- **UC-0008**: 逐文件断言 D 类硬编码中文迁移到 i18n（循环：cmd/co-shell-feishu-bridge/main.go、cmd/co-shell-hub/main.go、feishu/handler.go、bridge/executor.go、bridge/scheduler.go、subagent/subagent.go，每文件逐中文行断言已走 i18n.T）。
- **UC-0009**: 新增 i18n key 双存在断言（循环：每新增 key 断言 zh.go + en.go 均存在；audit 第 5 项保持 =1 不增）。
- **UC-0010**: `co-shell-feishu-bridge` 独立运行不受影响：`go build ./cmd/co-shell-feishu-bridge/` 成功；`--help` 输出正常；缺必填参数时退出码非 0 且提示明确。
- **UC-0011**: 主程序集成侧：ChannelSubAgent 输出（subagent 启动日志）可通过 OutputCategories 关闭；`agent/subagent_tools.go` 中硬编码中文同步迁移。

## 四、A 类 i18n 迁移（inventory A-3 清单）

- **UC-0012**: agent/run_stream.go 按 A-3 清单逐条迁移（KeyOutputCancelled、KeyOutputPaused、KeyOutputResume、KeyLoopDetectedSummary、KeyLoopHandling、KeyLoopFeedbackSent、KeyReorganizeUrgent 等约 20 条）；迁移后行中日文断言已走 i18n.T（循环逐条）。
- **UC-0013**: agent/loop.go 按 A-3 清单迁移（KeyLoopDetectEvent、KeyLoopJudgeResult 等约 6 条）；逐条断言。
- **UC-0014**: 新增 A 类 key 全部 zh/en 双存在；`go build ./...` 与 `go test ./cmd/...` 均绿（A 类迁移回归）。

## 五、审计收尾

- **UC-0015**: audit 对比 P3 基线：中文 901 → ≤780（下降 ≥120）；fmt 不增（≤204）；魔法事件=0 不变；同步输入=19 不变；i18n=1 不变。
- **UC-0016**: git diff 限 config/ + agent/out.go + 主程序 D 类接入点 + i18n 三处 + D 类独立程序 + 测试；无越界文件。

## 循环模式

UC-0004（4）+ UC-0005（10）+ UC-0006（3）+ UC-0008（D 类 6 文件 × 逐行 ≥20）+ UC-0009（新增 key 循环 ≥10）+ UC-0012/0013（A 类逐条 ≥25）≥ 72 子用例。
# FEATURE-303 向导迁移（B 类）+ i18n 归零第一步（P3）测试用例

| 项目 | 内容 |
|------|------|
| 任务编号 | FEATURE-303 |
| 任务名称 | 向导迁移（B 类）+ i18n 归零第一步（P3） |
| 类型 | 重构（输出通道统一 + i18n） |
| 版本 | v0.7.0 |
| 架构文档 | docs/output-architecture.md（P3 段 + 3.2/6.9） |

**目标**：cmd 5 handler（model/mode/config/settings_db/session）迁移到 Out；同步迁移 B 类硬编码中文到 i18n。
**验收基线**：P2 audit = fmt=204 / 魔法事件=0 / 中文=1555 / 同步输入=19 / i18n=1。
**验收目标**：中文 1555 下降约 1/3（≤1040）；fmt 继续下降；新增 i18n key 全部 zh+en 同步。

---

## 一、编译与回归

- **UC-0001**: `go build ./...` 退出码 0。
- **UC-0002**: `go test ./...` 无 FAIL（既有 file_tools 失败除外）。
- **UC-0003**: 新增 `Out.Box/Menu/Step/Sep` 四方法，table-driven 快照（4 组件 × 2 输入 = 8 子用例）。

## 二、向导功能回归（5 handler）

- **UC-0004**: `:model` 迁移后 list/switch/新增向导输出正常（emoji 保留）。
- **UC-0005**: `:mode` 迁移后 list/switch/param 正常 + 快捷键表 [B]/[C]/[E]/[D]/[Q]/[数字]。
- **UC-0006**: `:config` 迁移后一级菜单 / P 返回 / Q 退出 / 数字选择正常。
- **UC-0007**: `:set db` 迁移后显示当前值 + 枚举选项 + 连接测试正常。
- **UC-0008**: `:session` 迁移后 list/save/switch/pop 正常。
- **UC-0009**: `go test ./cmd/...` 全绿（含 settings_mode_dirs_test.go 等）。

## 三、i18n 合规

- **UC-0010**: 新增 key 逐条断言 zh/en 双存在；audit 第 5 项保持 =1。
- **UC-0011**: 被迁移的 fmt 直写中文删除；audit 第 3 项中文 ≤1040。

## 四、审计收尾

- **UC-0012**: audit 对比 P2：fmt 下降 / 中文 -1/3 / 魔法事件=0 / 同步输入=19 / i18n=1 不变；diff 限 out.go + cmd 5 handler + i18n + 测试。

## 循环模式

UC-0003（8）+ UC-0004/5/6（12）+ UC-0010（key 循环）≥ 30 子用例。
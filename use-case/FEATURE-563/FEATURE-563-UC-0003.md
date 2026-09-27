# FEATURE-563 测试用例（三）—— 状态注入 / 持久化 / 退出清理 / 兼容与工程验证

> 分支：**FEATURE-563**｜版本：**v0.77.0** (BUILD-1135)

## 一、被测目标

1. **状态注入**：`<environment_details>` 新增 `<sub_agents>` 块，每轮渲染全部 sub-agent 实时状态；无 sub-agent 时整块省略（与 `<board_agents>` 一致）。
2. **持久化**：进程表写入 store（key `subagent:procs`），父进程重启后恢复并识别 stale。
3. **退出清理**：主 agent 退出 kill 全部后台 sub-agent（含进程组）。
4. **兼容**：定时任务（`schedule_task` → `OnScheduledTaskTriggered`）适配新 API；旧「兄弟目录」机制废弃；旧调试输出清除。
5. **工程**：i18n 双语、无新增依赖、`go build/vet/test` 全绿。

期望的 `<sub_agents>` 形态（示例）：
```xml
<sub_agents>
  <agent id="research-1" status="running" pid="51234" run="1" started="2026-09-27 08:40:11" elapsed="42s"/>
  <agent id="research-2" status="completed" pid="51240" run="2" elapsed="1m3s" exit_code="0"/>
  <agent id="research-3" status="stale" pid="51111" run="1"/>
</sub_agents>
```

## 二、A 组：自动/单测验证

| 编号 | 用例 | 期望结果 | 结果 |
|------|------|----------|------|
| A-001 | 无 sub-agent 时构造环境块 | `<sub_agents>` 整块**不出现**（不是空标签） | ☐ |
| A-002 | 有 running + completed 两个 sub-agent | 块内两条 `<agent>`，字段含 id/status/pid/run/elapsed；completed 项含 exit_code | ☐ |
| A-003 | 状态实时性：进程从 running → completed | 下一轮构造的环境块状态随之变化（块在每轮 LLM 调用前重新生成） | ☐ |
| A-004 | 工具描述 | `launch_sub_agent`、`sub_agent_status`、`kill_sub_agent` 三者 description 均提示「关注 `<environment_details>` 中的 `<sub_agents>` 状态」 | ☐ |
| A-005 | 注入开关 | 与既有 FEATURE-471 的块开关机制兼容：`<sub_agents>` 随 `<environment_details>` 统一开关（关闭时整块不含） | ☐ |
| A-006 | 持久化写入 | 启动/状态变更后 store 中 key `subagent:procs` 存在且为合法 JSON；字段与进程表一致 | ☐ |
| A-007 | 重启恢复 | 新建 Manager 并加载 store：记录恢复；原 pid 已退出 → `stale`；仍存活 → 保持原状态 | ☐ |
| A-008 | 持久化写入频率 | 状态变更（启动/完成/kill/超时）均落盘；不因高频日志追加而反复写（仅状态变更触发） | ☐ |
| A-009 | 退出清理 | `Close()` 后：后台子进程全部被 kill；进程表未丢失（store 仍可读，状态为 killed/stale） | ☐ |
| A-010 | 定时任务适配 | `OnScheduledTaskTriggered` 使用新 API，id 形如 `scheduled-{n}`；工作目录 `{ws}/sub-agents/scheduled-{n}/`；执行结束状态写入进程表 | ☐ |
| A-011 | 旧机制移除 | 代码中不再存在「兄弟目录」定位逻辑（`filepath.Dir(os.Getwd())` + agent 名拼接）；不再有裸 `fmt.Printf("🔧 Sub-agent command…")` 调试输出 | ☐ |
| A-012 | 定时任务与手工任务共存 | 同一 Manager 下 `scheduled-1` 与手工 id 互不干扰，可分别 kill | ☐ |
| A-013 | i18n 完整性 | `i18n/keys.go` 新增 key 在 `zh.go` 与 `en.go` 均有值；`go test ./i18n/...` 通过（若存在校验测试） | ☐ |
| A-014 | 工程门禁 | `go build ./... && go vet ./...` 与 `go test ./... -short` 全绿；`gofmt -l` 无输出 | ☐ |
| A-015 | 依赖与文件规范 | `go.mod` 未新增第三方依赖；新增/修改的 Go 文件均 ≤ 500 行 | ☐ |
| A-016 | 版本同步 | `co-shell --version` → `v0.77.0 [BUILD-1135]`；hub `--version` 同为 `0.77.0 / 1135` | ☐ |

## 三、B 组：用户实测

| 编号 | 用例 | 期望 | 结果 |
|------|------|------|------|
| B-001 | 会话中创建 1 个 sub-agent，观察下一轮 environment_details | 出现 `<sub_agents>`，状态/pid/耗时与实际一致 | ☐ |
| B-002 | sub-agent 完成后再看 | 该条状态变为 completed，含 exit code 与最终结果 | ☐ |
| B-003 | 重启 co-shell 后查看 | 进程表恢复；失效进程显示 stale，不再误报 running | ☐ |
| B-004 | 用 `schedule_task` 建一个每分钟任务并等待触发 | 定时任务以 sub-agent 方式执行，状态可在 `<sub_agents>`/status 工具中看到 | ☐ |
| B-005 | 关闭 co-shell（Ctrl+C / 服务停止） | 无残留 sub-agent 进程（`ps` 校验） | ☐ |
| B-006 | 中文 / 英文界面 | 新增提示文案随语言切换正确显示 | ☐ |
| B-007 | 回归：既有 sub-agent 开关 | `:settings subagent-enabled off` 后三个工具均不注册（含新工具） | ☐ |

## 四、验收标准（本文件）

1. A 组 16 条全部通过（自动化）；
2. B 组 7 条由用户实测确认；
3. 交付前完成「编译可执行码」：`work/co-shell-0.77.0.darwin.arm64`、`work/co-shell-hub-0.77.0.darwin.arm64`，build 计数已更新；
4. 提交在 `FEATURE-563` 分支，未经用户确认不合并 `main`。

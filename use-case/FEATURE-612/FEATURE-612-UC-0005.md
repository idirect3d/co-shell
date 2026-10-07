# FEATURE-612 UC-0005：动态信息面板只显示「未消化」事件

> 分支：**FEATURE-612**　版本：**v0.100.1**　BUILD：**待定（编码后回填）**
> 需求（用户确认）：面板不再显示 append-only 的流水账 `history`，改为**只显示未消化的动态事件** —— 尚未注入的 pending + 已注入未 ack 的 retained；**不需要事后回看能力**。
> 判定口径：`DynEventsSnapshot()` 的数据源由 history 改为 `events + retained` 的合并视图；事件一旦被消费（注入且不可保留）或被 ack，就**从面板消失**。LLM 侧的 `<user_dynamic_events>` 注入与 ack 机制**不变**。

## 变更点（计划）

| 位置 | 变更 |
|---|---|
| `agent/dynamic_events.go` `DynEventsSnapshot()` | 数据源改为 `q.events`（pending）+ `q.retained`（unacked）合并，按事件序号**倒序**（最新在上） |
| `agent/dynamic_events.go` | 移除 `history` 字段、`dynHistoryMax` 常量与 `add()` 中的追加逻辑（已确认无其它引用） |
| `agent/feature612_test.go` | 更新 `TestDynEventsSnapshotFlags`（transient 消费后消失 / ack 后消失）；补一条跨两类合并排序断言 |
| `web/dyninfo.go`、前端 | **不改动**（items 结构与 pending/unacked 语义不变，仅条数与留存行为变化） |

## 用例与结果

`(R)` = 可由 `go test ./agent/` 自动验证；`(M)` = 需人工在 Web UI 验收。

### A. 显示范围（只留未消化）

- **UC-01（R）** 新入队但未注入的事件出现在快照中，标记 `Pending=true`。
- **UC-02（R）** 不可保留类（clip_object / upload_file / open_file / view_file）被注入消费后**立即从快照消失**（不再有"无标记的僵尸行"）。
- **UC-03（R）** 可保留类（user_message / ui_action / board_message / board_timeout / sub_agent_done）注入后仍在快照中，标记 `Unacked=true`。
- **UC-04（R）** 该事件被 `ack` 后**从快照消失**（此前是"留在面板、去掉标记"）。
- **UC-05（R）** 快照条数恒等于 `len(events) + len(retained)`，上限为 100 + 100。

### B. 排序与边界

- **UC-06（R）** 合并后按事件序号倒序（最新在上）；pending 与 retained 交错时同样成立。
- **UC-07（R）** 两个集合都为空时返回空切片（前端渲染「暂未捕获任何动态感知事件」）。
- **UC-08（R）** 未知 id / 重复 ack 幂等，且不会把已消失的事件"复活"。

### C. 不受影响项（回归）

- **UC-09（R）** `<user_dynamic_events>` 的注入内容不变：新事件按序注入、retained 事件带 `unacked="true"` 与 `[unacked]` 标记重放、ack 后停止重放。
- **UC-10（R）** 路径事件去重合并、`pendingUserMessages` 回填、`clearUserMessages` 等既有行为不变。
- **UC-11（M）** Web UI 面板：注入后 transient 卡片消失、ack 后卡片消失；头部计数胶囊（待确认+未确认）与实际条数一致，清零后隐藏。

## 验收标准

- `go test ./agent/` 全绿（含更新后的 FEATURE-612 快照测试）；
- `go build ./... && go vet ./...` 通过；
- 面板实测（UC-11）由用户在 Web UI 确认；
- 改动仅限 `agent/dynamic_events.go` 与其单测。

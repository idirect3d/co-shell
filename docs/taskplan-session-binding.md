# 任务计划与会话绑定方案设计

> 状态：已确认，待实施
> 日期：2026-08-21
> 关联：核心改进——任务计划（taskplan）与会话（session）绑定

## 1. 背景与问题

当前任务计划（taskplan，即 `track_task_progress` 工具维护的待办任务清单）是**全局单例**：

- `taskplan.Manager` 用固定 key `"current"` 通过 `store.GetContext("current")` / `SaveContext("current", data)` 存储**单个全局任务计划**。
- 只有一个 active plan，全局共享，**无 session 维度**。
- 切换会话（`handleSwitch`）只切换 `currentSessionID` 和消息历史，**不触碰 taskplan**。

**问题**：所有会话共享同一份任务计划。切换会话后，任务计划不变，导致不同会话的任务清单互相污染、混乱。

## 2. 目标

切换会话时，任务计划同步切换到**目标会话绑定的任务计划**。每个会话拥有独立的任务计划，互不干扰。

## 3. 已确认的设计决策

| 决策 | 选择 | 说明 |
|------|------|------|
| 1. 存储隔离方式 | **A** | 存储 key 加 session 前缀（`current:{sessionID}`），每个会话一份独立计划 |
| 2. 切换会话行为 | **A** | 切换会话时自动加载目标会话绑定的任务计划（视图切换，当前会话计划保持不变） |
| 3. 旧数据兼容 | **A** | 升级时，将现有全局 `"current"` 计划迁移到当前会话名下 |

## 4. 方案设计

### 4.1 存储隔离（决策1A）

`taskplan.Manager` 增加一个 `sessionID` 字段，存储 key 由固定 `"current"` 改为 `"current:{sessionID}"`。

```go
// taskplan/taskplan.go
type Manager struct {
    store         *store.DualStore
    memoryMgr     *memory.Manager
    planCounter   int
    memoryEnabled bool
    agentName     string
    sessionID     string // 新增：当前绑定的会话 ID
}

// SetSessionID 设置当前绑定的会话 ID，切换会话时由 Agent 调用。
func (m *Manager) SetSessionID(id string) {
    m.sessionID = id
}

// planKey 返回当前会话对应的存储 key。
func (m *Manager) planKey() string {
    if m.sessionID == "" {
        return currentPlanKey // 兼容无会话场景（如测试）
    }
    return currentPlanKey + ":" + m.sessionID
}
```

`loadCurrent` / `saveCurrent` / `DeleteContext` 全部改用 `m.planKey()`。

### 4.2 切换行为（决策2A）

在 `Agent.SetCurrentSessionID` 中统一通知 `taskPlanMgr` 切换 session，覆盖所有切换路径（`handleSwitch`、`:new`、`showInteractive` 的 `n`、`main.go` 启动）。

```go
// agent/agent.go
func (a *Agent) SetCurrentSessionID(id string) {
    a.mu.Lock()
    defer a.mu.Unlock()
    a.currentSessionID = id
    if a.taskPlanMgr != nil {
        a.taskPlanMgr.SetSessionID(id)
    }
}
```

这样无需逐个修改 `handleSwitch`、`repl.go`、`main.go` 等调用点，所有会话切换路径自动生效。

### 4.3 旧数据兼容（决策3A）

在 `taskplan.NewManager` 中做一次性迁移：若存在全局 `"current"` 计划且当前会话无独立计划，则将其迁移到当前会话名下。

```go
// taskplan/taskplan.go
func NewManager(s *store.DualStore) *Manager {
    mgr := &Manager{ ... }
    // 迁移旧数据：全局 "current" → 当前会话
    mgr.migrateLegacyPlan()
    return mgr
}

func (m *Manager) migrateLegacyPlan() {
    // 读取全局 "current"
    data, found, _ := m.store.GetContext(currentPlanKey)
    if !found {
        return
    }
    // 若当前会话已有独立计划，则跳过
    if _, found, _ := m.store.GetContext(m.planKey()); found {
        return
    }
    // 迁移：写入当前会话 key，删除全局 key
    _ = m.store.SaveContext(m.planKey(), data)
    _ = m.store.DeleteContext(currentPlanKey)
}
```

> 注意：`NewManager` 在 `agent.New` 中创建，此时 `sessionID` 尚未设置。迁移逻辑需在 `SetSessionID` 首次被调用时执行，或在 `NewManager` 后由 agent 显式触发。**需在讨论中确认迁移触发时机**。

## 5. 影响范围

| 文件 | 改动 |
|------|------|
| `taskplan/taskplan.go` | 增加 `sessionID` 字段、`SetSessionID`、`planKey`、`migrateLegacyPlan`；`loadCurrent`/`saveCurrent`/`DeleteContext` 改用 `planKey` |
| `agent/agent.go` | `SetCurrentSessionID` 中通知 `taskPlanMgr.SetSessionID` |

## 6. 已确认的决策

1. **迁移触发时机**：`NewManager` 创建时自动设置为当前 session ID（`NewManager` 接收当前 session ID 参数），迁移在 `NewManager` 中执行。
2. **planCounter 全局性**：保持全局递增（plan ID 仅用于归档记忆标识，全局唯一更安全）。
3. **无会话场景**：`sessionID` 为空时（如测试、未绑定会话），回退到全局 `"current"` key，保持向后兼容。

## 7. 验收标准

1. 会话 A 创建任务计划，切换到会话 B，任务计划为空（B 无独立计划）。
2. 会话 B 创建任务计划，切回会话 A，任务计划恢复为 A 的计划。
3. 升级前已有的全局 `"current"` 计划迁移到当前会话名下。
4. 各会话的任务计划互不干扰，切换后正确加载各自绑定的计划。

# FEATURE-490 hub agent 公告板协作机制 — 测试用例

> 分支：FEATURE-490
> 版本：v0.42.0
> 范围：最小可用闭环（发请求→认领→私信→执行→回结果）

## 用例清单

### UC-0001 公告板开关默认关闭
- **前置**：co-shell 默认配置启动
- **步骤**：检查公告板相关配置项
- **预期**：公告板开关默认关闭（false），不参与公告板协作

### UC-0002 开启公告板开关后注册职责
- **前置**：co-shell 开启公告板开关，配置职责 role
- **步骤**：启动后向 hub 注册 agent 身份与职责
- **预期**：hub 职责注册表包含该 agent 的 id/name/role

### UC-0003 请求方发布协助请求
- **前置**：两个 co-shell（A 请求方、B 响应方）均开启公告板并接入 hub
- **步骤**：A 调用 board_post 发布请求（含 required_role、描述）
- **预期**：hub 创建 Request(open)，广播通知所有 agent

### UC-0004 响应方感知到新请求
- **前置**：UC-0003 已发布请求
- **步骤**：B 巡检 board_list 或收到 board_notify
- **预期**：B 能看到该请求，判断职责匹配

### UC-0005 响应方认领请求
- **前置**：B 看到匹配请求
- **步骤**：B 调用 board_claim(request_id)
- **预期**：hub 状态 open→claimed，记录 B 为认领者，通知 A

### UC-0006 双方私信讨论
- **前置**：UC-0005 已认领
- **步骤**：A 与 B 通过 board_dm 私信澄清需求（多轮）
- **预期**：hub 中转 DM，双方均能收到对方消息

### UC-0007 请求方确认后进入执行
- **前置**：私信达成一致
- **步骤**：A 调用 board_confirm(request_id)
- **预期**：hub 创建 Task 指派给 B，状态→executing，下行 board_task 给 B

### UC-0008 响应方执行任务并回传结果
- **前置**：B 收到 board_task
- **步骤**：B 在当前会话执行任务，完成后调用 board_result(task_id, 结果)
- **预期**：hub 状态→done，A 收到结果

### UC-0009 请求方收到最终结果
- **前置**：UC-0008 完成
- **步骤**：A 查询或收到通知
- **预期**：A 能看到 B 返回的结果

### UC-0010 公告板关闭时忽略外部任务
- **前置**：co-shell 公告板开关关闭
- **步骤**：hub 尝试下行 board_task
- **预期**：co-shell 忽略该任务，不执行，不产生副作用

## 单元测试（Go）

- `hub/gateway/board_test.go`：Board 状态机（post/claim/dm/confirm/result 全流程）
- `hub/gateway/board_test.go`：职责注册表（注册/查询/匹配）
- `agent/dynamic_events_test.go`：新增 board_* DynamicEventKind 渲染
- `web/session_test.go`：board_task 消息注入 inputCh
- `agent/board_tools_test.go`：board_result 工具回传

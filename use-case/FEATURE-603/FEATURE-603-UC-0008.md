# FEATURE-603-UC-0008 静态与构建校验

> 分支：**FEATURE-603**　版本：**v0.95.0**　BUILD：**（编译后填写）**
> 判定口径：改动仅涉及前端 JS，但项目要求整体编译与静态检查必须全绿。

## 操作步骤

1. `node --check web/static/app.js`
2. `go build ./... && go vet ./...`（一次性执行）

## 预期结果

- `node --check` 无输出（语法通过）
- `go build ./...` 无错误，`go vet ./...` 无告警
- `go test ./web/ -short` 失败集与 main 基线一致（无新增失败）

## 实际结果

- `node --check web/static/app.js` → 无输出（通过）
- `go build ./... && go vet ./...` → 通过（`BUILD_VET_OK`）
- 产物：`work/co-shell-0.95.0.darwin.arm64`、`work/co-shell-hub-0.95.0.darwin.arm64`（v0.95.0 / BUILD-1215）
- `go test ./web/ -short` 存在既有失败（`TestSessionList`、`TestSessionSwitch`、`TestYOLO*`、`TestWebIOAskInteraction*` 等，均为 session/WebSocket 消息类）；这些用例不引用 `app.js`，与本次纯前端 JS 改动无关（FIX-600 曾记录同一批既有失败）

## 结论

- [x] 通过
- [ ] 失败

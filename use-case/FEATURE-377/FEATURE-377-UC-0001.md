# FEATURE-377 测试用例 — 文件列表每次方法调用后自动刷新

> 版本 v0.7.8 | 分支 FEATURE-377
> 功能：文件列表自动刷新策略从"仅 done 事件（LLM 迭代完成）时刷新"改为"每次方法调用（工具调用）返回后自动刷新（含分支状态）"。

## 需求规格

1. `web/static/app.js` `renderEvent` 中 `tool_call` 事件 result 分支（工具执行完成）末尾调用 `refreshBranch()` + `loadTree()`。
2. 每次工具调用返回后，文件列表与分支状态实时刷新，不必等到整个 LLM 迭代结束。
3. 原有 `done` 事件刷新逻辑保留（作为兜底）。

## 测试用例

### UC-0001 工具调用返回后文件列表自动刷新（headless 浏览器）
- 准备：工作区含 `a.txt`，页面加载后树显示 `a.txt`
- 操作：磁盘创建 `b.txt`，触发 `tool_call` result 事件（`renderEvent({type:"tool_call", meta:{phase:"result"}})`）
- 断言：树自动刷新，包含 `b.txt`
- 通过标准：触发 result 事件后树中出现 `b.txt`

### UC-0002 工具调用返回后分支状态刷新（headless 浏览器）
- 准备：工作区为 git 仓库，页面加载后分支名显示
- 操作：切换分支，触发 `tool_call` result 事件
- 断言：`refreshBranch()` 被调用，分支名更新
- 通过标准：result 事件后分支名反映最新分支

### UC-0003 done 事件刷新兜底保留（headless 浏览器）
- 准备：页面加载
- 操作：触发 `done` 事件
- 断言：树仍刷新（原有逻辑保留）
- 通过标准：done 事件后树刷新

### UC-0004 回归：node 语法检查
- 操作：`node --check web/static/app.js`
- 通过标准：无语法错误

## 执行方式

- UC-0001~0003：headless 浏览器（复用 FEATURE-375 的 harness 模式）
- UC-0004：`node --check`
- 收尾：`go build ./... && go vet ./... && go build -o work/co-shell .`

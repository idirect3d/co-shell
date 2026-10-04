# FEATURE-603-UC-0002 多文件跨目录：保留目标路径链并集

> 分支：**FEATURE-603**　版本：**v0.95.0**　BUILD：**（编译后填写）**
> 判定口径：一次标注 carry 多个受影响对象（最多 3 个）时，保留集合是**所有目标路径链的并集 ∪ 工作区根**，其余已展开目录仍全部收起。

## 前置条件

- 启动 Web UI（`--serve`）
- 已展开 `web`、`web/static`、`docs`、`samples` 等目录

## 操作步骤

1. 触发一次标注，`affected_objects = ["<workspace>/web/static/app.js", "<workspace>/agent/loop.go"]`
2. 观察文件树

## 预期结果

- `expandedDirs` 收敛为：`""`、`web`、`web/static`、`agent`（两条目标链的并集）
- `web/static/app.js` 与 `agent/loop.go` 行均可见且带 `aff-pred` 高亮
- `docs`、`samples` 等不在并集中的目录被收起（子行 `<li>` 为 0）

## 实际结果

隔离实例实测（v0.95.0 / BUILD-1215）：

- 标注前已展开 `""`、`agent`、`docs`、`web`、`web/static`，`#tree li` = 21
- 调用 `highlightAffectedFiles([{path:"web/static/app.js"},{path:"agent/loop.go"}])` 后：
  - `agent` OPEN（kids=1）、`web` OPEN（kids=2）、`web/static` OPEN（kids=1）
  - `docs` CLOSED（kids=0）
  - `agent/loop.go` 行 `.name` class 含 `aff-pred`
  - `#tree li` 21 → 20

## 结论

- [x] 通过
- [ ] 失败

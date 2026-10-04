# FEATURE-603-UC-0001 单文件目标：收起其他已展开文件夹

> 分支：**FEATURE-603**　版本：**v0.95.0**　BUILD：**（编译后填写）**
> 判定口径：标注一个文件/文件夹后，工作区文件树中**只有目标路径链上的文件夹**保持展开，其余此前已打开的文件夹全部收起（其子行 DOM 被删除）。

## 背景

`web/static/app.js` 的 `highlightAffectedFiles()`（~5284 行）当前只把受影响文件的祖先目录**追加**进 `expandedDirs`（~5260 行，一个只增不减的 `Set`），`loadTree()` → `restoreExpandedDirs()`（~5601 行）随后按它逐层补拉展开，导致已展开文件夹越积越多、DOM 膨胀。

## 前置条件

- 启动 Web UI（`--serve`），工作区为 co-shell 项目根目录
- 用户已手动展开若干互不相关的文件夹，例如：`web`、`web/static`、`agent`、`docs`

## 操作步骤

1. 确认 `web/static`、`agent`、`docs` 均处于展开状态（其子行可见）
2. 触发一次工具调用标注，使 `affected_objects = ["<workspace>/web/static/app.js"]`（前端 `highlightAffectedFiles([{path:"web/static/app.js"}])`）
3. 观察文件树

## 预期结果

- `expandedDirs` 收敛为：`""`（工作区根）、`web`、`web/static`
- 保持展开：根目录 → `web` → `web/static`，且 `app.js` 行可见
- 已收起：`agent`、`docs` 等不在目标路径链上的目录，其子行 `<li>` 数量为 0、`dataset.filled` 已清除
- `app.js` 行带 `aff-pred` 高亮（蓝色文字 + 淡蓝背景），并被滚动到可见位置

## 实际结果

隔离实例实测（`work/co-shell-0.95.0.darwin.arm64`，v0.95.0 / BUILD-1215，独立 workspace + 端口 28899）：

- 标注前已展开 `""`、`agent`、`docs`、`web`、`web/static`，`#tree li` 总数 = 21
- 调用 `highlightAffectedFiles([{path:"web/static/app.js"}])` 后：
  - `agent`、`docs` 变为 CLOSED，`ul.children.length = 0`，`dataset.filled` 为空
  - `web`（kids=2）、`web/static`（kids=1）保持 OPEN
  - `web/static/app.js` 行存在，`.name` 的 class 含 `aff-pred`
  - `#tree li` 总数 21 → 19

## 结论

- [x] 通过
- [ ] 失败

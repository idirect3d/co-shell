# FEATURE-603-UC-0006 收起后 DOM 释放验证

> 分支：**FEATURE-603**　版本：**v0.95.0**　BUILD：**（编译后填写）**
> 判定口径：被收起的文件夹，其行与子行必须从 DOM 中消失（`loadTree()` 重建 + `restoreExpandedDirs()` 只恢复保留目录），并丢弃其层级数据，从而释放内容。

## 前置条件

- 启动 Web UI（`--serve`）
- 已展开 `web/static`、`agent`、`docs`，并记录 `document.querySelectorAll("#tree li").length` 为 N1

## 操作步骤

1. 触发一次标注，`affected_objects = ["<workspace>/web/static/app.js"]`
2. 待 `loadTree()` 重建完成后，读取：
   - `document.querySelectorAll("#tree li").length`（记为 N2）
   - `document.querySelector('#tree .tree-row[data-path="agent"]')`
   - `expandedDirs`（若能访问）是否含 `agent`、`docs`

## 预期结果

- N2 < N1（被收起目录的子行 DOM 已不再创建）
- `agent` 行仍存在（顶层目录行保留），但其 `<ul>` 无子行：`ul.children.length === 0` 且 `ul.dataset.filled` 未定义
- `expandedDirs` 不含 `agent`、`docs`；含 `""`、`web`、`web/static`
- 无 JS 异常（控制台无报错）

## 实际结果

隔离实例实测（v0.95.0 / BUILD-1215）：

- 被收起目录（`agent` / `docs`）的行仍存在（顶层目录行保留），但其 `<ul>.children.length = 0`、`ul.dataset.filled` 为 undefined（未填充）
- `#tree li` 总数随目标链规模下降：单文件目标 21 → 19；双目标 21 → 20；根目标 20 → 16
- 目标路径链（`web`、`web/static`）保持 OPEN 且 `dataset.filled=1`
- 控制台无 JS 异常（仅一个与本改动无关的 `/logos/dark` 404 静态资源）

## 结论

- [x] 通过
- [ ] 失败

# FEATURE-603-UC-0003 目标为工作区根：只保留根展开

> 分支：**FEATURE-603**　版本：**v0.95.0**　BUILD：**（编译后填写）**
> 判定口径：受影响对象为工作区根（`"."`，或工作区绝对路径）时，保留集合仅 `{""}`，除根之外的所有已展开文件夹全部收起。

## 前置条件

- 启动 Web UI（`--serve`）
- 已展开 `web/static`、`agent`、`docs` 等目录

## 操作步骤

1. 触发一次标注，`affected_objects = ["<workspace>"]`（前端 path 为 `"."` 或 `""`）
2. 观察文件树

## 预期结果

- `expandedDirs` 收敛为仅 `""`（工作区根）
- 根目录行带 `aff-pred` 高亮（复用 FEATURE-449 的根路径归一：`""` ↔ `"."`）
- `web`、`web/static`、`agent`、`docs` 等子目录全部收起

## 实际结果

隔离实例实测（v0.95.0 / BUILD-1215）：

- 标注前已展开 `""`、`agent`、`docs`、`web`、`web/static`，`#tree li` = 20
- 调用 `highlightAffectedFiles([{path:"."}])` 后：仅根 `""` OPEN（kids=15）；`agent` / `docs` / `web` / `web/static` 全部 CLOSED
- `#tree li` 20 → 16

（注：根路径归一为 `""` 的行为与 FEATURE-449 的 `"" ↔ "."` 归一一致。）

## 结论

- [x] 通过
- [ ] 失败

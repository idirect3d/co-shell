# FEATURE-603-UC-0005 无受影响对象：不改变展开状态

> 分支：**FEATURE-603**　版本：**v0.95.0**　BUILD：**（编译后填写）**
> 判定口径：`highlightAffectedFiles` 在 `files` 为空时**直接返回**，不得误收起用户已展开的目录（收敛的前提是"有目标"）。

## 前置条件

- 启动 Web UI（`--serve`）
- 已展开 `web/static`、`agent` 等多个目录

## 操作步骤

1. 直接调用 `highlightAffectedFiles([])`（或一次 `affected_objects: []` 的工具调用）
2. 观察文件树与 `expandedDirs`

## 预期结果

- `expandedDirs` 保持不变（`web`、`web/static`、`agent` 等仍展开）
- 不发生树重建（无新增 `api/tree` 请求），DOM 与滚动位置不变

## 实际结果

隔离实例实测（v0.95.0 / BUILD-1215）：

- 当时展开态为「仅根 `""` 展开（kids=15），其余全 CLOSED」，`#tree li` = 16
- 调用 `highlightAffectedFiles([])` 后：仍为仅根展开、`#tree li` = 16，展开状态无任何变化

## 结论

- [x] 通过
- [ ] 失败

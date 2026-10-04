# FEATURE-603-UC-0004 目标路径链原本未展开：标注后展开且可见

> 分支：**FEATURE-603**　版本：**v0.95.0**　BUILD：**（编译后填写）**
> 判定口径：收敛（收起）不得影响「目标可见」——若目标路径链上的文件夹此前是收起的，标注后仍必须被展开，目标行可见。

## 前置条件

- 启动 Web UI（`--serve`）
- `web` 目录处于**收起**状态（未展开）

## 操作步骤

1. 触发一次标注，`affected_objects = ["<workspace>/web/static/app.js"]`
2. 观察文件树

## 预期结果

- 根目录 → `web` → `web/static` 逐级展开，`app.js` 行可见且带 `aff-pred` 高亮
- `treeDirRows` 中存在 `web`、`web/static` 条目，其 `<ul>.dataset.filled` 已置位
- 其他已展开目录被收起（UC-0001 同款）

## 实际结果

隔离实例实测（v0.95.0 / BUILD-1215）：

- 首次标注时 `web`、`web/static` 原本均为 CLOSED（当时仅根 `""` 展开）
- 调用 `highlightAffectedFiles([{path:"web/static/app.js"}])` 后：`web`、`web/static` 变为 OPEN 且已加载子行，`web/static/app.js` 行出现且 `.name` class 含 `aff-pred`
- 根 `""` 保持 OPEN（`dataset.filled=1`）

## 结论

- [x] 通过
- [ ] 失败

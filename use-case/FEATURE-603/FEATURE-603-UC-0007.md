# FEATURE-603-UC-0007 前次标注高亮不残留

> 分支：**FEATURE-603**　版本：**v0.95.0**　BUILD：**（编译后填写）**
> 判定口径：上一次工具调用的受影响文件（`prevAffectedFiles`，字体高亮 `aff-pred-fg`）若位于本次被收起的目录下，其行应随目录一起从 DOM 消失，不得残留在错误位置或与当前高亮混淆。

## 前置条件

- 启动 Web UI（`--serve`）

## 操作步骤

1. 第一次标注：`affected_objects = ["<workspace>/web/static/app.js", "<workspace>/docs/a.md"]`
2. 第二次标注：`affected_objects = ["<workspace>/agent/loop.go"]`
3. 观察文件树

## 预期结果

- 第二次标注后，`docs` 目录被收起：其下 `docs/a.md` 行不在 DOM（因此不出现残留的 `aff-pred-fg`）
- `web/static/app.js` 行因 `web/static` 也被收起而不在 DOM，同样不残留
- 当前目标 `agent/loop.go` 行带全高亮 `aff-pred`
- 用户之后手动重新展开 `docs` 时，`a.md` 按其语义显示为字体高亮 `aff-pred-fg`（FEATURE-449 既有行为），而非全高亮

## 实际结果

隔离实例实测（v0.95.0 / BUILD-1215）：

- 第一次标注 `web/static/app.js` 后，其行 `.name` class 为 `aff-pred-fg aff-pred`（同时携带前次遗留的字体高亮与当前全高亮）
- 第二次标注 `agent/loop.go` 后，`docs`、`web/static` 均被收起：`docs/a.md`、`web/static/app.js` 行不在 DOM（不残留可见高亮）
- 当前目标 `agent/loop.go` 行 `.name` class 含 `aff-pred`

## 结论

- [x] 通过
- [ ] 失败

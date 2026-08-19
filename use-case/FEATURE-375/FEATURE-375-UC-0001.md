# FEATURE-375 测试用例 — web 文件列表标注 git 修改状态

> 版本 v0.7.8 | 分支 FEATURE-375
> 功能：Web UI 侧栏文件树标注每个文件的 git 修改状态（字母徽标 M/A/D/U/R），目录汇总显示内部变更文件数，LLM 迭代完成（done 事件）后自动刷新树。

## 需求规格

1. 后端 `web/server.go` 新增 `gitStatusMap(root)`：执行 `git status --porcelain -z`（`exec.Command` 参数数组，无 shell），解析为 `路径 → 状态码` 映射；非 git 仓库或 git 不可用时返回空 map（树行为不变）。
2. `treeNode` 增加两个字段：
   - `status`（string, omitempty）：文件节点的状态码（`M`/`A`/`D`/`R`/`U`，其中 `??` 归一化为 `U`）
   - `changes`（int, omitempty）：目录节点下（递归）有变更的文件数
3. `buildTree` 构建时填充上述字段；`/api/tree` 响应携带。
4. 前端 `app.js` `treeNode()`：
   - 文件行：`status` 非空时文件名后显示字母徽标，颜色 M=蓝、A=绿、D=红、U=绿、R=紫
   - 目录行：`changes > 0` 时目录名后显示计数徽标（如 `●3`）
5. 前端 `done` 事件（LLM 迭代完成）触发 `loadTree()` 自动刷新。

## 测试用例

### UC-0001 gitStatusMap 基本状态解析（Go 单测）
- 准备：临时目录 `git init`，创建并提交 `a.txt`
- 操作：修改 `a.txt`、新建 `b.txt`（未跟踪）、删除 `c.txt`（已跟踪）、`git add b.txt`、重命名 `d.txt → e.txt`
- 断言：`gitStatusMap` 返回 `a.txt→M`、`b.txt→A`、`c.txt→D`、`e.txt→R`、`b.txt` 未跟踪时为 `U`
- 通过标准：各状态码与 `git status --porcelain` 一致

### UC-0002 gitStatusMap 非 git 仓库（Go 单测）
- 准备：无 `.git` 的临时目录
- 断言：返回空 map，不报错、不 panic
- 通过标准：`len(map) == 0`

### UC-0003 gitStatusMap 工作区干净（Go 单测）
- 准备：`git init` + 提交后无修改
- 断言：返回空 map
- 通过标准：`len(map) == 0`

### UC-0004 /api/tree 文件节点 status 字段（Go 集成测试）
- 准备：test server 工作区 `git init`，提交 `hello.txt`，再修改它；新建未跟踪 `new.txt`
- 操作：`GET /api/tree`
- 断言：`hello.txt` 节点 `status == "M"`；`new.txt` 节点 `status == "U"`；未变更文件无 `status` 字段
- 通过标准：JSON 字段值正确

### UC-0005 /api/tree 目录 changes 汇总（Go 集成测试）
- 准备：工作区 `git init`，`sub/` 目录下 2 个已跟踪文件各修改 1 个、新增 1 个未跟踪
- 操作：`GET /api/tree`
- 断言：`sub` 目录节点 `changes == 2`；根节点 `changes == 2`（递归汇总）；无变更目录无 `changes` 字段
- 通过标准：计数正确且 omitempty 生效

### UC-0006 非 git 仓库树行为不变（Go 集成测试）
- 准备：无 `.git` 的工作区（现有 TestTreeExclusions 场景）
- 断言：所有节点无 `status`/`changes` 字段，树结构与现状一致
- 通过标准：现有 `TestTreeExclusions` 等测试不回归

### UC-0007 前端文件字母徽标渲染（headless 浏览器）
- 准备：工作区 `git init` + 制造 M/A/D/U/R 各一个文件，启动 serve
- 操作：加载页面，展开树
- 断言：
  - 修改文件行出现蓝色 `M` 徽标
  - 未跟踪文件行出现绿色 `U` 徽标
  - 删除文件不显示（磁盘已无，git status 有 D 但树无此节点——预期不显示，仅验证不报错）
  - 未变更文件无徽标
- 通过标准：徽标文本与颜色 class 正确

### UC-0008 前端目录计数徽标渲染（headless 浏览器）
- 准备：`sub/` 内 3 个文件有变更（2M+1U），页面加载
- 断言：`sub` 目录行显示 `●3` 计数徽标；无变更目录无徽标
- 通过标准：计数与颜色正确

### UC-0009 done 事件自动刷新树（headless 浏览器）
- 准备：页面加载后（树已渲染，无徽标），通过 WS 注入 `done` 事件；同时后端工作区文件已被修改
- 断言：收到 `done` 后 `loadTree()` 被重新调用（fetch /api/tree 发生），新徽标出现
- 通过标准：done 后树刷新且徽标更新

### UC-0010 上传后树刷新携带状态（headless 浏览器）
- 准备：页面加载，上传一个新文件到根目录
- 断言：上传完成后树刷新，新文件显示 `U` 徽标
- 通过标准：上传 → loadTree → 徽标出现链路正常

### UC-0011 回归：现有 web 测试全绿
- 操作：`go vet ./web/ && go test ./web/ && node --check web/static/app.js`
- 通过标准：全部通过，无回归

## 执行方式

- UC-0001~0006：`web/server_test.go` 新增测试（table-driven，临时目录 + `git init`，`testing.Short()` 下跳过需要 git 的用例）
- UC-0007~0010：headless Chrome harness（复用 FEATURE-373/374 的 harness 模式）
- UC-0011：收尾命令

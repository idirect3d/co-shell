# FEATURE-494 测试用例：文件查看器定位最近提交首个修改位置

> 分支：FEATURE-494 ｜ 版本：v0.43.0
> 功能：打开一个文本文件后，通过 git 获取该文件最近一次被修改的提交（`git log -1 -- <file>`），取该提交相对其父提交的 diff（`git diff C^ C -- <file>`）的第一个修改位置（第一个 hunk 的起始行），将文件查看器滚动到该行位于可视区上 1/3 处。

## 后端接口（/api/gitfirstchange）

### UC-001 返回最近一次提交的第一个修改行号
- 前置：工作区为 git 仓库，目标文件 `a.txt` 最近一次被修改的提交为 C（C 相对 C^ 修改了 a.txt 的第 5 行附近）
- 操作：`GET /api/gitfirstchange?path=a.txt`
- 预期：返回 `{"line": 5}`（第一个 hunk 的新侧起始行），HTTP 200

### UC-002 文件最近一次提交是新增文件（无父提交）
- 前置：目标文件 `new.txt` 在最近一次提交 C 中首次被加入（C 无父提交，即根提交）
- 操作：`GET /api/gitfirstchange?path=new.txt`
- 预期：返回 `{"line": 1}`（新增文件第一个 hunk 起始行为 1），HTTP 200

### UC-003 文件未被 git 跟踪（untracked）
- 前置：目标文件 `untracked.txt` 存在于工作区但从未被提交
- 操作：`GET /api/gitfirstchange?path=untracked.txt`
- 预期：返回 `{"line": 0}`（无修改位置），HTTP 200

### UC-004 工作区不是 git 仓库
- 前置：工作区根目录无 `.git`
- 操作：`GET /api/gitfirstchange?path=hello.txt`
- 预期：返回 `{"line": 0}`，HTTP 200

### UC-005 路径穿越被拒绝
- 前置：任意
- 操作：`GET /api/gitfirstchange?path=../../etc/passwd`
- 预期：HTTP 403（路径校验失败）

### UC-006 文件不存在
- 前置：任意
- 操作：`GET /api/gitfirstchange?path=no-such-file.txt`
- 预期：返回 `{"line": 0}`（git log 无结果），HTTP 200

## 前端滚动定位

### UC-007 打开文件后滚动到首个修改行位于可视区上 1/3 处
- 前置：git 仓库中 `b.txt` 最近一次提交修改了第 30 行附近，文件查看器已打开
- 操作：单击 `b.txt` 打开预览
- 预期：文件查看器 `#fvBody` 滚动，使第 30 行（首个修改行）的顶部位于可视区高度约 1/3 处（`scrollTop ≈ 目标行 offsetTop - clientHeight/3`）

### UC-008 无修改位置时不滚动
- 前置：打开的文件无 git 修改位置（untracked / 非 git 仓库 / 无父提交但 line=0）
- 操作：单击该文件打开预览
- 预期：文件查看器保持顶部（scrollTop=0），不报错

### UC-009 修改行在文件前 200 行内（首屏已加载）直接定位
- 前置：`c.txt` 最近一次提交修改了第 10 行，首屏 200 行已加载
- 操作：单击 `c.txt` 打开预览
- 预期：滚动定位到第 10 行位于可视区上 1/3 处

### UC-010 修改行超出首屏（>200 行）时按需加载后定位
- 前置：`d.txt` 最近一次提交修改了第 500 行，首屏只加载 200 行
- 操作：单击 `d.txt` 打开预览
- 预期：文件查看器按需加载到第 500 行所在块后，滚动定位到第 500 行位于可视区上 1/3 处

### UC-011 md 自动渲染模式（Raw off）下定位
- 前置：`e.md` 最近一次提交修改了第 40 行，Raw 关闭（自动渲染）
- 操作：单击 `e.md` 打开预览
- 预期：md 渲染完成后滚动定位到包含第 40 行的块位于可视区上 1/3 处

### UC-012 切换文件后重新定位
- 前置：先打开 `a.txt`（定位到其修改行），再单击 `b.txt`
- 操作：单击 `b.txt` 打开预览
- 预期：`b.txt` 打开后按其首个修改行重新定位，不残留 `a.txt` 的滚动位置

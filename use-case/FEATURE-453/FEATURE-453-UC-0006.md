# FEATURE-453-UC-0006 :skill add <path> 命令（复制到工作空间）

## 前置条件
- 已编译最新可执行码（`go build -o work/co-shell .`）
- 存在一个本地 skill 目录（含 SKILL.md），如 `/tmp/new-skill/`

## 操作步骤
1. 在 REPL 中执行 `:skill add /tmp/new-skill`
2. 观察输出
3. 检查工作空间 `./skills/` 下是否出现 `new-skill` 目录

## 预期结果
- `:skill add /tmp/new-skill` 将 skill 目录复制到工作空间 `./skills/new-skill/`
- 复制后 `:skill list` 能看到新 skill
- 源目录不存在时显示错误提示

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败

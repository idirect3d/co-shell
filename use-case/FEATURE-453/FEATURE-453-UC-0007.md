# FEATURE-453-UC-0007 :skill add <path> --global 命令（复制到全局）

## 前置条件
- 已编译最新可执行码（`go build -o work/co-shell .`）
- 存在一个本地 skill 目录（含 SKILL.md），如 `/tmp/global-skill/`

## 操作步骤
1. 在 REPL 中执行 `:skill add /tmp/global-skill --global`
2. 观察输出
3. 检查全局目录 `~/.co-shell/skills/` 下是否出现 `global-skill` 目录

## 预期结果
- `:skill add /tmp/global-skill --global` 将 skill 目录复制到全局 `~/.co-shell/skills/global-skill/`
- 复制后 `:skill list` 能看到新 skill（全局来源）
- 不带 --global 时默认复制到工作空间 `./skills/`

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败

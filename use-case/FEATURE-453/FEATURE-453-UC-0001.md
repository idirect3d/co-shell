# FEATURE-453-UC-0001 skill 目录扫描（工作空间 + 全局合并）

## 前置条件
- 已编译最新可执行码（`go build -o work/co-shell .`）
- 工作空间存在 `./skills/` 目录，含至少一个 skill（子目录 + SKILL.md）
- 全局目录 `~/.co-shell/skills/` 存在，含至少一个 skill

## 操作步骤
1. 在工作空间 `./skills/` 下创建 skill 目录（如 `my-skill/SKILL.md`）
2. 在全局 `~/.co-shell/skills/` 下创建 skill 目录（如 `global-skill/SKILL.md`）
3. 执行 `:skill list`
4. 观察是否合并展示两处的 skills

## 预期结果
- `:skill list` 同时列出工作空间 `./skills/` 和全局 `~/.co-shell/skills/` 的 skills
- 每个 skill 显示名称、简介、路径
- 工作空间和全局的 skill 都能被识别

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败

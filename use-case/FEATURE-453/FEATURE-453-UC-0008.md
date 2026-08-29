# FEATURE-453-UC-0008 :skill remove <name> 命令

## 前置条件
- 已编译最新可执行码（`go build -o work/co-shell .`）
- 工作空间 `./skills/` 下有一个 skill（如 `my-skill`）

## 操作步骤
1. 在 REPL 中执行 `:skill remove my-skill`
2. 观察输出
3. 检查工作空间 `./skills/` 下 `my-skill` 目录是否被移除
4. 执行 `:skill list` 确认不再显示

## 预期结果
- `:skill remove my-skill` 移除工作空间 `./skills/my-skill/` 目录
- 移除后 `:skill list` 不再显示该 skill
- skill 不存在时显示错误提示

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败

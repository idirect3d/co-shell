# FEATURE-453-UC-0002 SKILL.md frontmatter 解析

## 前置条件
- 已编译最新可执行码（`go build -o work/co-shell .`）
- 工作空间 `./skills/` 下有两个 skill：
  - `with-fm/SKILL.md`：含 YAML frontmatter（name/description）
  - `no-fm/SKILL.md`：无 frontmatter

## 操作步骤
1. 创建 `with-fm/SKILL.md`，含 frontmatter：
   ```
   ---
   name: with-fm
   description: 带 frontmatter 的 skill
   ---
   使用说明...
   ```
2. 创建 `no-fm/SKILL.md`，无 frontmatter，首行为"无 frontmatter 的 skill"
3. 执行 `:skill list`
4. 观察两个 skill 的 name 和 description 显示

## 预期结果
- `with-fm`：name 从 frontmatter 解析（with-fm），description 从 frontmatter 解析（带 frontmatter 的 skill）
- `no-fm`：name 用目录名兜底（no-fm），description 用 SKILL.md 首行兜底（无 frontmatter 的 skill）
- 两个 skill 都能正确显示

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败

# FEATURE-453-UC-0003 系统提示词 SKILLS section（只列索引，不加载内容）

## 前置条件
- 已编译最新可执行码（`go build -o work/co-shell .`）
- 工作空间 `./skills/` 下有一个 skill（含 SKILL.md，内容较长）

## 操作步骤
1. 创建 `./skills/my-skill/SKILL.md`，内容较长（含详细使用步骤）
2. 启动 co-shell，查看系统提示词
3. 观察 SKILLS section 的内容

## 预期结果
- 系统提示词中有独立的 SKILLS section
- SKILLS section 只列出 skill 索引（name + description + 路径），**不加载 SKILL.md 的详细内容**
- 索引格式类似：`- my-skill: 描述... (./skills/my-skill/SKILL.md)`
- LLM 需要时可用 read_file 读取 SKILL.md 获取详细内容

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败

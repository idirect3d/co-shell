# FEATURE-449-UC-0002 受影响对象在未展开文件夹时高亮生效

## 前置条件
- 启动 Web UI（`--serve`）
- 工作区为 co-shell 项目根目录
- 文件列表根目录处于折叠状态（未展开）

## 操作步骤
1. 在录入框输入指令，让 LLM 报告受影响对象为某个深层目录下的文件（如 `affected_objects: ["/Users/direct3d/github/co-shell/agent/run_stream.go"]`）
2. 观察文件列表

## 预期结果
- 根目录被自动展开
- `agent` 目录被自动展开
- `run_stream.go` 文件被标记为高亮（蓝色文字 + 淡蓝背景）并滚动到可见位置

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败

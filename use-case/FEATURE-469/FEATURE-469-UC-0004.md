# FEATURE-469-UC-0004 系统参数 web-input-dir 改变上传目录

## 前置条件
- 启动 Web UI（`co-shell --serve`）
- 附件托盘中已加入一张图片

## 操作步骤
1. 打开右上角菜单 →「系统设置」→「开发者」分组
2. 确认存在「上传附件目录（工作区相对路径，默认 input）」项，值显示 `input`
3. 将值改为 `uploads`（或直接在 TUI `:set web-input-dir uploads`）
4. 发送一张图片附件
5. 查看工作区：`uploads/` 目录自动创建并保存该图片，`input/` 不产生新文件

## 预期结果
- 系统设置中出现 web-input-dir 项（默认 `input`）
- 值改变后上传落盘到新目录（前端 settings_get 缓存该值，上传请求 dir=新目录）
- 非法值被拒绝：`:set web-input-dir /abs`、`../x`、空值报错（已由单测 TestWebInputDirRejectsBadPaths / TestWebInputDirValueDefault / TestSettingsJSONIncludesWebInputDir 覆盖）
- 配置持久化到 config.json（web_input_dir 字段）

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败

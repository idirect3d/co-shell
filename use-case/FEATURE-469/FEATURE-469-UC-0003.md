# FEATURE-469-UC-0003 发送：批量上传到 input/ 目录 + 图片视觉附件 + 动态标签

## 前置条件
- 启动 Web UI（`co-shell --serve`），工作区可写
- 附件托盘中已放入：1 张图片（截图粘贴）、1 个普通文本文件（如 note.txt）
- 工作区尚未存在 `input/` 目录

## 操作步骤
1. 录入框输入一句话（或留空仅附件）
2. 按 Enter / 点击发送
3. 观察：先出现上传（发送后）行为；成功后消息气泡回显，主文字下方出现附件标签行（🖼/📄 + 路径 + 大小）
4. 查看工作区文件树/磁盘：`input/` 目录被自动创建，两个文件按相对路径 `input/clip-….png`、`input/note.txt` 落盘
5. 检查发给模型的 user 消息内容：末尾带 `<<<DYNAMIC>>>` 块，每行 `路径<TAB>image|file<TAB>大小`
6. 仅上传图片时确认图片进入既有 attachments 视觉通道（本次用户回合 imagePaths 生效，等同 CLI `--image`，视觉模型收到图片内容）

## 预期结果
- 附件在发送时批量 `POST /api/upload?dir=input`，服务端目标目录不存在时自动创建（已由单测 TestUploadAutoCreatesDir 覆盖）
- 发送成功后 user 消息末尾追加动态感知标签文本并随消息存储；图片路径同时作为 attachments 视觉注入
- 回显气泡底部渲染出 .user-dyn 标签 chips，与主文字分离
- 上传失败时中止发送并保留附件（可重试）

## 实际结果
（待填写）

## 结论
- [ ] 通过
- [ ] 失败

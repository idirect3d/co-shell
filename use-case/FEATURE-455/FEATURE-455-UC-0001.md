# FEATURE-455 测试用例

## 概述

远程访问 Web UI 时，工作区文件列表的"定位到文件夹"功能受控自动变为"下载"图标（文件夹的定位功能消失），用户点击下载图标可通过浏览器下载目标文件。下载功能通过命令行参数 `--download-enabled` 控制启用/禁用，禁用时完全不提供此功能，需防止非授权下载。

## 判定标准

- **远程访问**：监听地址 `Bind` 非 `127.0.0.1`/`localhost`/`::1`（如 `0.0.0.0` 或局域网 IP）
- **下载可用**：`--download-enabled` 开启 **且** 远程访问

## 测试用例

### UC-001 本地访问 + 下载未启用：保持定位图标
- 前置：`Bind=127.0.0.1`，未传 `--download-enabled`
- 操作：打开 Web UI 工作区文件列表
- 预期：文件与文件夹均显示"定位到文件夹"（⌖）图标；`/api/download` 返回 403

### UC-002 远程访问 + 下载未启用：定位图标禁用，下载禁用
- 前置：`Bind=0.0.0.0`，未传 `--download-enabled`
- 操作：打开 Web UI 工作区文件列表
- 预期：文件与文件夹均**不显示**任何定位/下载图标（远程访问时定位功能始终禁用）；`/api/download` 返回 403（下载功能完全不提供）；`/api/open`、`/api/reveal` 返回 403

### UC-003 远程访问 + 下载启用：文件变下载图标，文件夹定位消失
- 前置：`Bind=0.0.0.0`，传 `--download-enabled`
- 操作：打开 Web UI 工作区文件列表
- 预期：
  - 文件行显示"下载文件"（⬇）图标，点击触发浏览器下载
  - 文件夹行不显示任何定位/下载图标（定位功能消失）
  - bootstrap 返回 `remote=true`、`downloadEnabled=true`

### UC-004 下载路径穿越校验
- 前置：`Bind=0.0.0.0`，传 `--download-enabled`
- 操作：请求 `/api/download?path=../../etc/passwd`
- 预期：返回 403，不下载文件

### UC-005 下载目录被拒绝
- 前置：`Bind=0.0.0.0`，传 `--download-enabled`
- 操作：请求 `/api/download?path=<某目录>`
- 预期：返回 404（not a file）

### UC-006 下载正常文件
- 前置：`Bind=0.0.0.0`，传 `--download-enabled`
- 操作：请求 `/api/download?path=hello.txt`
- 预期：返回 200，`Content-Disposition: attachment`，内容为文件内容

### UC-007 白名单 IP 授权
- 前置：`Bind=0.0.0.0`，传 `--download-enabled`，`Whitelist=192.168.1.0/24`
- 操作：非白名单 IP 请求 `/api/download`
- 预期：返回 403（whitelist 中间件拦截）

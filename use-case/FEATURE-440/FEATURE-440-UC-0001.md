# FEATURE-440 测试用例

## 概述

FEATURE-440 新增 2 个 shell 启动脚本（与 co-shell 可执行程序同目录）：
1. `run-cli.sh`：以 enhanced 模式启动 CLI（`--input-mode enhanced`）
2. `run-web.sh`：以白名单形式启动对其他主机的 Web UI 服务（`--serve --bind 0.0.0.0 --whitelist`）

## 测试环境

- 分支：FEATURE-440
- 版本：v0.18.0
- 脚本位置：work/ 目录（与 co-shell 可执行程序同目录）

---

## UC-0001 run-cli.sh 存在且可执行

**前置**：work/ 目录
**操作**：检查 `work/run-cli.sh` 是否存在且可执行
**预期**：文件存在，权限含 x（可执行）

## UC-0002 run-cli.sh 以 enhanced 模式启动 CLI

**前置**：work/ 目录
**操作**：运行 `./work/run-cli.sh --version`
**预期**：正确调用 co-shell 并显示版本信息（v0.18.0），说明脚本能正确调用可执行程序

## UC-0003 run-cli.sh 传递 enhanced 参数

**前置**：work/ 目录
**操作**：检查脚本内容
**预期**：脚本包含 `--input-mode enhanced` 参数，且透传额外参数（`"$@"`）

## UC-0004 run-web.sh 存在且可执行

**前置**：work/ 目录
**操作**：检查 `work/run-web.sh` 是否存在且可执行
**预期**：文件存在，权限含 x（可执行）

## UC-0005 run-web.sh 以白名单形式启动 Web UI 服务

**前置**：work/ 目录
**操作**：运行 `./work/run-web.sh "127.0.0.1" -w <临时工作目录> --port 8899`
**预期**：Web UI 服务启动，绑定 0.0.0.0，端口 8899 监听，白名单生效

## UC-0006 run-web.sh 未指定白名单时默认本机

**前置**：work/ 目录
**操作**：运行 `./work/run-web.sh`（不指定白名单）
**预期**：提示未指定白名单，默认使用 127.0.0.1

## UC-0007 run-web.sh 支持环境变量指定白名单

**前置**：work/ 目录
**操作**：`COSHELL_WHITELIST="192.168.1.100" ./work/run-web.sh`
**预期**：白名单使用环境变量值 192.168.1.100

---

## 验证清单

| 用例 | 验证方式 |
|------|---------|
| UC-0001/0004 | `ls -la work/run-cli.sh work/run-web.sh` |
| UC-0002 | `./work/run-cli.sh --version` |
| UC-0003 | `cat work/run-cli.sh` |
| UC-0005 | 启动服务 + `lsof -iTCP:8899` |
| UC-0006 | 运行脚本观察提示 |
| UC-0007 | 设置环境变量运行脚本观察白名单 |

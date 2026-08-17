# FEATURE-306 输入统一（InputSource）+ Windows 补齐 — 测试用例

> 任务编号：FEATURE-306
> 作者：L.Shuang
> 创建日期：2026-08-16

## 背景

现状存在三套输入路径竞争 stdin：

| 路径 | 位置 | 现状 |
|------|------|------|
| 标准流式 | `repl/userio.go` `StdioIO` | bufio.Scanner 行读（`--input-mode stdio`，pipe 友好） |
| 按键级 | `repl/userio.go` `EnhancedIO` + `repl/enhanced_input.go` `EnhancedInput` | raw 模式 + 自己 `os.Stdin.Read` 逐字节读并解析 ESC 序列 |
| 后台监控 | `repl/repl_esc_posix.go` `startESCMonitor` | 独立 goroutine，unix.Poll 100ms 轮询，只认 `0x1b`/`0x03`；**Windows 是 no-op** |

三个结构性缺陷：① 监控 goroutine 与 ReadKey/ReadLine 抢读 stdin，靠 `IsReading()`/`IsCommandRunning()` 标志 + 100ms 轮询规避；② enhanced/stdio 是配置级切换，无法接入第三种输入源；③ Windows 上 ESC/Ctrl+C 监控不存在，且 main.go 在 Windows 强制 stdio。

## 目标架构

```
stdin / WS / UI事件
   ↓
单一 Reader 循环（唯一读取者）
   ↓ 产出统一 InputEvent 流
InputEvent{ Kind, Data }
   ├── 后台消费者：ESC/Ctrl+C 监控（agent 运行期激活）
   ├── ReadLine 消费者（行编辑器）
   └── ReadKey 消费者（等待下一个按键/控制字符）
```

**方案 A1（用户已确认）：EnhancedInput 全量事件流化**。tui 模式 = `RawKeySource`（单一 Reader goroutine 唯一读 stdin，复用 readCSI/readSS3 解析）；stdio 模式 = `StdioSource`（同步行读，行为不变）。ESC/Ctrl+C 监控从 unix.Poll 轮询改为事件流消费者（POSIX/Windows 统一，Windows no-op 消除）。消费者优先级规则：**独占消费者（ReadLine/ReadKey）优先，ESC/Ctrl+C 监控仅在无独占消费者激活时响应**。

## 用例列表（运行时验证，tui 默认模式）

| 编号 | 场景 | 操作 | 预期结果 |
|------|------|------|---------|
| UC-0001 | REPL 主循环：历史导航 | 启动 co-shell（tui），输入 `ls` 回车；输入 `date` 回车；按 ↑ 两次再按 Enter | 历史从 `date`→`ls` 正确回放，Enter 重新执行 `ls` |
| UC-0002 | REPL 主循环：光标移动 | 输入 `hello world`，用 ←/→ 移动光标，Home/End 跳转 | 光标按预期移动，中间插入字符位置正确 |
| UC-0003 | REPL 主循环：ESC 清空输入行 | 输入 `abc` 后按 ESC | 输入行清空、光标回行首，REPL 不退出（现状语义不破坏） |
| UC-0004 | REPL 主循环：Ctrl+C 取消输入 | 输入 `abc` 后按 Ctrl+C | 输入取消返回错误，REPL 回到提示符 |
| UC-0005 | REPL 主循环：Ctrl+D 空行退出 | 空提示符按 Ctrl+D | 空行时退出 REPL（EOF） |
| UC-0006 | agent 运行：ESC 中断 LLM 输出 | 发送长任务（如“详细列出10个方案”），流式输出中按 ESC | 输出停止、提示中断（Interrupt），回到 REPL 提示符，可继续输入 |
| UC-0007 | agent 运行：Ctrl+C 取消任务 | 发送任务后立刻按 Ctrl+C | 任务取消（Cancel），立即回到 REPL 提示符，无残留子进程 |
| UC-0008 | agent 运行：AskUser 确认 | 触发需确认的工具调用（删除/覆盖文件），等待确认时按 ESC | 确认被取消（归 ReadKey，不中断 agent），提示“用户取消” |
| UC-0009 | 系统命令：stdin 不被偷读 | tui 模式执行交互命令（如 `read -p "x:"` 或 sudo/passwd 场景） | 子进程正常获得 stdin，输入不被 Reader 偷读；命令结束恢复输入 |
| UC-0010 | 多字节输入 | 输入中文 `你好世界` 和 emoji `🚀`，再退格删除 | 正常插入/删除多字节字符，显示宽度正确 |
| UC-0011 | stdio 管道模式 | `echo "你好" \| co-shell --input-mode stdio -c "..."` | 行为与重构前完全一致（管道输入不破坏） |
| UC-0012 | stdio 交互模式 | `co-shell --input-mode stdio` 手动输入 | 行输入/回车执行正常，无 raw 模式冲突 |
| UC-0013 | Windows 编译 | `GOOS=windows go build ./...` | 交叉编译通过；`repl_esc_windows.go` 不再是无操作实现 |
| UC-0014 | 退出清理 | 运行若干轮后退出 co-shell | 无 goroutine 泄漏（Reader/消费者全部退出），无残留 raw 模式（终端恢复） |
| UC-0015 | cmd 向导交互回归 | tui 模式执行 `:set`/`:model list`/`:model add`（输入名称后 `q` 取消） | 显示正常（无乱码）、输入可正常录入、取消后回到提示符、可继续 `exit` 退出 |
| UC-0016 | raw 模式输出换行回归 | tui 模式执行 `pwd` 等直接命令、`:help`、`exit` | 所有输出行 `\r\n` 结尾，下一提示符位于新行行首（无光标滞留/挤行） |
| UC-0017 | replace_in_file 字面转义解码 | XML 模式 `<cs:search>`/`<cs:replace>` 内含字面 `\n`/`\t`（LLM 输出转义而非真实换行） | 解析后 search/replace 携带真实换行/Tab，与文件实际行尾匹配成功；真实换行与 `\\n`（字面反斜杠）保持不变 |
| UC-0018 | 工具调用前导换行 | 流式工具调用（如 `⚙️ write_to_file`）紧跟 LLM 内容之后 | 第一个工具头前有换行，与前面 LLM 输出视觉分离；同一流内后续工具头不重复加空行 |

## 单元测试

| 测试 | 位置 | 形态 |
|------|------|------|
| ESC 序列解析 | `repl/raw_key_source_test.go`（新增） | table-driven：方向键/单ESC/CSI/SS3/Tab/退格/Enter/Ctrl+C/UTF-8 中文 |
| EnhancedInput 事件驱动 | `repl/enhanced_input_test.go`（新增） | 假 InputSource 喂事件序列，断言编辑结果（历史/光标/ESC 清行/Ctrl+C） |
| 消费者优先级 | `repl/input_reader_test.go`（新增） | ESC 在行编辑期归行编辑器、agent 运行期归监控消费者 |
| Reader 暂停/恢复 | `repl/input_reader_test.go`（新增） | 命令执行期间 Pause 后事件停止分发，Resume 后恢复 |
| 现有回归 | 现有 `*_test.go` | `go test ./...` 全绿 |

## 验收

- `go build ./... && go vet ./... && go test ./...` 全绿
- `bin/output_audit.sh --strict` 通过（Hardcoded Chinese = 0 / i18n keys missing = 0）
- `cc` 编译 `work/co-shell`（BUILD 编号 +1）
- 用例 UC-0001 ~ UC-0014 全部通过（UC-0013 Windows 真机项标注待回归）

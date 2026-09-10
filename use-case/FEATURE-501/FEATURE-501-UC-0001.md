# FEATURE-501 启动时自动创建系统内置文件夹

## 背景

co-shell 的多个内置能力各自对应 workspace 下的一个系统文件夹，但这些文件夹只在对应功能首次使用时才按需创建，新用户无法从目录结构直观发现 co-shell 有哪些内置能力。需要在启动时统一创建这些文件夹，使用户通过文件夹名即可大致了解内置能力。

## 方案

启动时（workspace 确定且 chdir 之后）统一创建 12 个系统内置文件夹：

| 文件夹 | 对应内置能力 |
|--------|-------------|
| `.rules/` | 规则（按需加载到系统提示词） |
| `skills/` | 技能（SKILL.md 技能包） |
| `research/` | 调研资料与报告 |
| `input/` | 输入（Web UI 附件上传目录） |
| `output/` | 输出（子 Agent 产物、导出文件） |
| `mode/` | 工作模式（act/plan/research 等模式配置） |
| `bin/` | 自定义工具脚本 |
| `tmp/` | 临时文件（工具结果、文件列表） |
| `log/` | 日志 |
| `db/` | 数据库（bbolt、models.json） |
| `download/` | 浏览器下载（截图、HTML） |
| `logos/` | 自定义 logo |

约束：
- 已存在的文件夹不覆盖、不报错
- 创建失败仅告警，不阻断启动
- 所有启动路径（serve / REPL / 单条指令）均生效

## 验收标准

1. 首次启动后，workspace 根目录下 12 个系统内置文件夹全部存在。
2. 已存在的文件夹内容不被覆盖或清空。
3. 部分文件夹已存在时，只补建缺失的文件夹。
4. 若某路径已存在且为文件（非目录），启动不崩溃，仅告警。
5. 创建失败不影响后续启动流程（不阻断）。
6. `--serve` 模式与 REPL 模式启动后均创建成功。
7. 编译通过：`go build ./... && go vet ./...`。

## 测试用例

### UC-001 首次启动创建全部内置文件夹
- 前置：全新空目录作为 workspace
- 步骤：启动 co-shell
- 期望：workspace 根目录下 `.rules`、`skills`、`research`、`input`、`output`、`mode`、`bin`、`tmp`、`log`、`db`、`download`、`logos` 共 12 个文件夹全部存在且为目录

### UC-002 已存在文件夹内容保留
- 前置：workspace 中 `.rules/team.md` 已有内容，`skills/my-skill/SKILL.md` 已有内容
- 步骤：启动 co-shell
- 期望：两个文件内容保持不变（不被覆盖、不被清空）

### UC-003 部分存在时只补建缺失
- 前置：workspace 中仅存在 `input/` 与 `output/`
- 步骤：启动 co-shell
- 期望：其余 10 个文件夹被创建，`input/`、`output/` 保持不变

### UC-004 路径为文件时不崩溃
- 前置：workspace 中存在名为 `research` 的普通文件（非目录）
- 步骤：启动 co-shell
- 期望：启动流程继续（不崩溃、不退出），仅输出告警

### UC-005 创建失败不阻断启动
- 前置：workspace 目录只读（无写权限）
- 步骤：启动 co-shell
- 期望：启动流程继续（告警但不中断），co-shell 正常运行

### UC-006 serve 模式启动创建
- 前置：全新空目录作为 workspace
- 步骤：`co-shell --serve --port <port> -w <workspace>`
- 期望：12 个内置文件夹全部存在，Web UI 正常启动

### UC-007 REPL/单条指令模式启动创建
- 前置：全新空目录作为 workspace
- 步骤：`co-shell -w <workspace> "hello"`（stdio 单条指令模式）
- 期望：12 个内置文件夹全部存在

### UC-008 重复启动幂等
- 前置：已启动过一次（12 个文件夹已存在）
- 步骤：再次启动 co-shell
- 期望：不报错、不重复创建、文件夹内容不受影响

### UC-009 编译验证
- 前置：无
- 步骤：运行 `go build ./... && go vet ./...`
- 期望：全部通过，无编译错误、无 vet 告警

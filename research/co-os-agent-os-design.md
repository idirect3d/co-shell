# cool OS：自研 Agent 指令集与 Agent 操作系统设计备忘

> 本文档记录 2026-09-23 关于「自研 Agent 指令集」与「Agent 操作系统（cool OS）」的讨论结论。
> 状态：**设计讨论中**，尚未编码。所有示例均放在代码块中，避免被误解析为工具调用。

---

## 1. 背景与动机

### 1.1 LLM 速度趋势
- 去年 DeepSeek 官方 API 约 20-50 t/s，现在已达 200-300 t/s，约提升 10 倍。
- 未来 LLM 很可能直接驱动电脑渲染窗口/组件（类似 render_ui），并由 agent 驱动执行其他指令（类似操作系统）。

### 1.2 核心瓶颈：输出 token 是"昂贵的"
- **输入（prefill）**：可并行计算，且前缀缓存生效时重复输入几乎零成本 → 可达 10000+ t/s。
- **输出（decode）**：自回归、串行生成，无法并行 → 约 100 t/s 量级。
- 因此：**输出 token 越短，生成越快**。当任务复杂（渲染复杂界面、系统批处理、大量循环指令）时，十几 K 的输出要等几十秒甚至数分钟。

### 1.3 当前工具调用协议的问题
- 方法名、参数名由多个单词组成，分隔符多，占用大量输出 token。
- 现有两种模式：
  - **OpenAI function calling**：`{"name":"read_file","arguments":{...}}`
  - **XML 模式**：`<cs:read_file><cs:path>...</cs:path></cs:read_file>`
- 两者都偏冗长，且**运行时只是"执行者"**，对指令没有完全控制权。

---

## 2. 自研指令集的价值

### 2.1 效率价值（输出压缩）
以 `read_file`（3 参数）为例的 token 开销对比：

| 格式 | 示例 | 字符数 | 估算 token |
|------|------|-------|-----------|
| XML（当前） | `<cs:read_file>...` | 126 | ~31 |
| OpenAI JSON | `{"name":"read_file",...}` | 80 | ~20 |
| 汇编助记符 | `RD main.go 1 50` | 16 | ~4 |

- 汇编助记符省约 **87%** 字符。
- 在**循环/批处理**场景下收益是乘法级的：100 次循环，每次省 25 token → 省 2500 token → 省约 25 秒（100 t/s）。

### 2.2 安全价值（软约束 → 硬约束）
当前安全机制本质是"LLM 自报 + 前端展示 + 用户确认"的**软约束**：
- `meta.risk` 由 LLM 自评（LLM 说 low 就 low）。
- `affected_objects` 由 LLM 自填（LLM 说只读就只读）。
- supervisor 白名单依赖另一个 LLM 判断。

**自研指令集让运行时拥有对指令的完全解释权和强制校验权**：
1. **指令围栏**：路径/资源边界在运行时强制校验，越界即拒绝，不依赖 LLM 自觉。
2. **风险类型化**：风险绑定到指令类型（RD=low、EX=high），而非 LLM 自评。
3. **参数白名单/枚举**：结构化防注入。
4. **指令可审计**：结构化，天然可记录（类型/参数/风险/围栏结果）。
5. **沙盒降级**：高风险指令可降级到受限环境执行。

### 2.3 类比：Agent OS 的"系统调用层"
```
操作系统：  用户程序 → 系统调用(syscall) → 内核校验 → 执行
Agent OS：  LLM      → 自研指令集      → 运行时校验 → 执行
```
自研指令集就是 Agent OS 的"系统调用层"，运行时校验每个指令的权限/参数/资源/风险。

---

## 3. 方案设计：do 容器方法 + 汇编助记符

### 3.1 容器方法命名
- 方法名就叫 **`do`**（从方法名开始简化）。
- 类似 render_ui，接收一个**指令数组**，运行时逐条校验并执行。

### 3.2 汇编风格助记符（2-4 字符）
采用 **2-4 字符汇编助记符**（比 1-2 字符更优）：
- 冲突率低（RD/WR/EX/SR/LS 组合空间大）。
- 可读性好（RD=read, WR=write, EX=exec）。
- **LLM 熟悉度高**：训练数据里大量汇编代码，生成正确率高，降低"记错指令导致重试"的风险。

### 3.3 首批助记符映射表（只读 + 文件写）
```text
RD  → read_file                    # 读文件
SR  → search_files                 # 搜索文件
LS  → list_files                   # 列出目录
LC  → list_code_definition_names   # 列出代码定义
WR  → write_to_file                # 写文件
RP  → replace_in_file              # 替换文件内容
```

后续阶段扩展：
```text
EX  → execute_command              # 执行命令（阶段2，需命令白名单）
```

### 3.4 调用示例（位置参数）
```text
do {
  RD main.go 1 50          # read_file: 路径 起始行 结束行
  SR ./agent "func.*Tool"  # search_files: 路径 正则
  LS ./agent               # list_files: 路径
  WR main.go "content"     # write_to_file: 路径 内容
  RP main.go "old" "new"   # replace_in_file: 路径 旧 新
}
```

### 3.5 关键权衡：位置参数顺序
- 最大风险是 LLM 记错参数顺序。
- 缓解：统一"路径在前，范围/内容在后"；助记符语义化；运行时校验参数类型/范围，出错返回明确错误供 LLM 自纠。

---

## 4. 演进路径（渐进式替换）

```
阶段 1（现在）：do 试验田
  → 只处理只读 + 文件写指令（RD/SR/LS/LC/WR/RP）
  → 验证指令集协议（围栏、白名单、校验）

阶段 2：扩展到 execute_command
  → 加入命令白名单、沙盒降级
  → 验证高风险指令的安全控制

阶段 3：扩展到系统命令
  → 指令集成为"系统调用层"
  → 运行时拥有完全控制权（Agent OS 雏形）

阶段 4：Agent OS（cool OS）
  → 指令集 = 系统调用
  → 运行时 = 内核（校验权限/参数/资源/风险）
```

---

## 5. 架构决策（已确认）

| 问题 | 结论 |
|------|------|
| 容器方法命名 | `do`（从方法名开始简化） |
| 架构 | **先 A 后 B**：先用聚合器（内部调现有工具）验证协议，再逐步独立化 |
| 首批指令范围 | 只读 + 文件写（RD/SR/LS/LC/WR/RP），不含 EX |
| 助记符长度 | **2-4 字符汇编助记符**（比 1-2 字符更优） |
| 编码前 | **先写设计文档**，确认后再编码 |

### 5.1 cool OS 定位（2026-09-23 确认）

| 问题 | 结论 |
|------|------|
| cool OS 定位 | **作为 co-shell 内的内核模块**（复用现有基础设施，渐进式演进） |
| 指令格式 | **纯文本汇编**（`do { RD main.go 1 50 }`，最省 token） |
| 围栏默认策略 | **默认拒绝**（白名单：只允许 workspace 内路径 + 白名单命令） |
| meta 对象 | **保留 meta**，但**调用一次 do 只给一个 meta**（不再每条指令都带） |

### 5.2 cool OS 核心组件

| 层 | 职责 | 类比 |
|----|------|------|
| **指令层** | `do` 容器 + 汇编助记符（RD/WR/EX...） | 汇编指令 |
| **校验层** | 指令围栏、参数白名单、风险类型化 | 内核权限校验 |
| **执行层** | 调用现有工具 / 系统命令 | 系统调用实现 |
| **审计层** | 记录每条指令的执行/校验/拒绝 | 内核日志 |
| **降级层** | 高风险指令降级到沙盒 | 进程隔离 |

### 5.3 cool OS 演进里程碑

```
M1（试验田）：do 容器 + 只读/文件写指令
  → 验证指令集协议（围栏、白名单、校验）
  → 在 co-shell 内作为新工具加入

M2（命令层）：加入 EX 指令 + 命令白名单 + 沙盒降级
  → 验证高风险指令的安全控制

M3（内核化）：指令集成为"系统调用层"
  → 运行时拥有完全控制权

M4（cool OS 成型）：完整的内核（校验/审计/降级）
  → 可作为独立模块或独立进程
```

### 5.4 M1 实现细节（2026-09-24 确认）

| 问题 | 结论 |
|------|------|
| 解析方式 | **按行解析**（每行一条指令，空格分隔，类似 shell） |
| 返回格式 | **合并返回**（所有指令结果合并成一个结果，省 token） |
| 围栏范围（M1） | **仅路径围栏**（只校验路径在 workspace 内） |
| 测试用例位置 | **use-case/ 下直接放**（不新建子目录） |

---

## 6. 待办 / 下一步

- [ ] 确认 cool OS 命名（Agent 操作系统名称）。
- [ ] 细化 `do` 指令集的协议定义（指令类型、参数、围栏规则）。
- [ ] 设计指令围栏的具体实现（路径/资源边界如何定义）。
- [ ] 设计风险类型化的指令分类体系。
- [ ] 评估与现有 meta 对象 / confirm 机制的兼容演进。
- [ ] 编写测试用例（use-case/ 下）。

---

## 7. M1 详细协议定义（FEATURE-551）

### 7.1 do 工具声明

`do` 是 cool OS 的容器方法，类似 render_ui。它接收一个**纯文本指令块**，运行时逐条校验并执行。

```text
do {
  RD main.go 1 50
  SR ./agent "func.*Tool"
  LS ./agent
  WR main.go "content"
  RP main.go "old" "new"
}
```

### 7.2 指令类型白名单（M1）

| 助记符 | 对应工具 | 参数（位置） | 风险 |
|--------|---------|-------------|------|
| `RD` | read_file | path, start_line, end_line | low |
| `SR` | search_files | path, regex | low |
| `LS` | list_files | path | low |
| `LC` | list_code_definition_names | path | low |
| `WR` | write_to_file | path, content | medium |
| `RP` | replace_in_file | path, search, replace | medium |

### 7.3 解析规则（按行解析）

- 每行一条指令，空格分隔（类似 shell）。
- 支持 `#` 注释（行内 `#` 之后为注释）。
- 支持引号包裹参数（如 `"func.*Tool"`），引号内可含空格。
- 转义规则：复用 C 语言字符串表达式，`\"` 表示引号，`\\` 表示反斜杠，`\n` 表示换行。

### 7.4 路径围栏规则（M1 仅此一项）

- **相对路径**：解析到 workspace 根目录下。
- **绝对路径**：必须落在 workspace 根目录内，越界即拒绝。
- 越界示例：`/etc/passwd`、`../outside`、`~/secret` 均拒绝。

### 7.5 返回格式（合并返回）

所有指令结果合并成一个结果返回，每条指令标注状态：

```text
[RD] OK: <文件内容>
[SR] OK: <匹配结果>
[LS] OK: <目录列表>
[WR] OK: 已写入
[RP] OK: 已替换
[RD] REJECT: 路径越界 /etc/passwd
```

### 7.6 代码结构（M1）

```go
// agent/do_tools.go - cool OS do 指令集（M1）
package agent

// doOpTypes 是支持的指令类型白名单
var doOpTypes = map[string]bool{
    "RD": true, "SR": true, "LS": true,
    "LC": true, "WR": true, "RP": true,
}

// buildDoTool 声明 do 工具（类似 buildRenderUITool）
func (a *Agent) buildDoTool() llm.Tool { ... }

// doTool 是 do 工具的回调
func (a *Agent) doTool(ctx context.Context, args map[string]interface{}) (string, error) {
    // 1. 解析 meta（一次 do 一个 meta）
    // 2. 解析指令（按行解析）
    // 3. 逐条校验（路径围栏）
    // 4. 逐条执行（聚合器：调用现有工具）
    // 5. 合并返回
}

// parseDoOps 按行解析指令
func parseDoOps(raw string) ([]DoOp, error) { ... }

// validateDoOpFence 路径围栏校验
func validateDoOpFence(op DoOp, workspace string) error { ... }
```

---

## 8. 版本与任务信息

- **版本**：0.73.0（minor 递增）
- **任务编号**：FEATURE-551
- **分支名**：FEATURE-551
- **状态**：设计讨论中，尚未编码

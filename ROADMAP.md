# co-shell 版本计划

> 版本号格式：`v{major}.{minor}.{patch}`

---

## 当前版本

> **版本**: v0.3.0 — RC1
> **BUILD**: 120







> 每次 `go build ./...` 编译成功后，BUILD 编号 +1。
> 完成任务时，在任务后标注 `[BUILD-XX]` 标记完成时的编译版本。

---

## v0.1.0 — Alpha（已完成）

> **状态**: ✅ 已完成
> **发布日期**: 2026-04-26


### 功能清单

- [x] FEATURE-1 REPL 交互界面（go-prompt，Tab 补全）[BUILD-1]
- [x] FEATURE-2 LLM 客户端抽象（OpenAI 兼容 API）[BUILD-1]
- [x] FEATURE-3 Agent 核心循环（LLM 调用 → 工具执行 → 迭代）[BUILD-1]
- [x] FEATURE-4 内置命令系统（.settings / .mcp / .rule / .memory / .context）[BUILD-1]
- [x] FEATURE-5 持久化存储（bbolt 记忆/上下文）[BUILD-1]
- [x] FEATURE-6 MCP 客户端管理器（多 Server 连接）[BUILD-1]
- [x] FEATURE-7 系统命令执行（超时控制）[BUILD-1]
- [x] FEATURE-8 配置管理（JSON 持久化到 ~/.co-shell/）[BUILD-1]
- [x] FEATURE-9 API初始设置（默认设置为deepseek，但Key留空）[BUILD-1]
- [x] FEATURE-10 API设置向导（co-shell启动后当系统大模型API参数不完整时，提示用户输入不完整的参数，比如默认deepseek配置不带key，那么就先提示用户输入正确的key并需要至测试成功为止）[BUILD-1]
- [x] FEATURE-11 系统命令直接运行（如果用户直接输入系统命令或执行程序在当前环境下可以直接执行，则直接执行用户输入的所有内容，而不用通过大模型解释。）[BUILD-1]
- [x] FEATURE-12 流式输出支持 [BUILD-1]
- [x] FEATURE-13 日志系统（文件日志，支持运行时开关）[BUILD-24]
- [x] FEATURE-14 API Key 脱敏显示 [BUILD-24]
- [x] FEATURE-15 命令行参数支持（--help/--version/--model/--endpoint/--api-key/--log）[BUILD-24]
- [x] FEATURE-16 命令行指令支持（-c/--cmd 执行单条自然语言或系统指令后退出）[BUILD-24]
- [x] FEATURE-18 会话历史管理（用户可以通过上、下键在自己输入的历史内容之间翻页，包括上一次执行co-shell时的内容）[BUILD-25]
- [x] FEATURE-19 基础错误处理和用户提示 [BUILD-25]
- [x] FEATURE-20 最大迭代次数可配置（--max-iterations 命令行参数、.settings max-iterations 运行时设置、config.json 持久化）[BUILD-27]
- [x] FEATURE-21 多配置文件位置支持（优先级：命令行参数指定 > 当前目录 config.json > ~/.co-shell/config.json）[BUILD-28]
- [x] FEATURE-22 多供应商支持（DeepSeek v4 / 阿里千问 / OpenAI 兼容兜底），设置向导支持供应商选择、自动打开官网获取 API Key [BUILD-29]
- [x] ENHANCEMENT-23 设置向导增强：Tab 键显示可选列表、上下键选择、ESC 退出、连接测试 [BUILD-31]
- [x] ENHANCEMENT-24 设置向导增强：OpenAI 兼容模式下输入端点后自动测试连通性，输入 API Key 后自动获取模型列表 [BUILD-32]
- [x] FEATURE-47 国际化（i18n）支持中文/英文，--lang 命令行参数，自动检测系统语言 [BUILD-33]
- [x] FEATURE-51 多平台支持（macOS/Linux/Windows）[BUILD-35]
- [x] FEATURE-52 首次运行风险声明 [BUILD-36]
- [x] FEATURE-53 命令执行确认机制（执行命令前等待用户确认：批准/拒绝/修改后重新评估），由配置控制开关 [BUILD-37]
- [x] ENHANCEMENT-63 超时时间参数化改造：所有超时动作可配置，默认永不超时，超时异常传递上下文给LLM；日志增强：所有HTTP/工具调用前INFO记录、异常ERROR记录、传递给LLM的内容DEBUG记录；修复RunStream中USER_MODIFY_REQUEST导致消息历史不完整（assistant含tool_calls但缺少对应tool消息）的API 400错误 [BUILD-47]
- [x] FEATURE-64 新增 .wizard 内置命令，在 REPL 中重新启动 API 设置向导；wizard 全面改用标准 fmt.Scanln 输入，移除所有 raw terminal 和 ANSI 控制码，确保跨平台兼容；REPL 移除 completer（Tab 补全下拉列表）和颜色样式选项，简化终端控制 [BUILD-54]
- [x] ENHANCEMENT-65 .settings 命令改为 .set（同时兼容 .settings），执行 .set 时显示参数清单（参数名、可选项/值范围、说明）；当前配置每行增加参数名和值范围 [BUILD-63]
- [x] ENHANCEMENT-66 命令确认改进：增加 [A] 本次都同意选项；拒绝执行改为 [C] 取消，用户选择后直接返回 REPL 等待输入；去掉 [d] 拒绝执行选项 [BUILD-64]
- [x] FEATURE-67 会话历史管理：历史任务列表命令（.list/.last/.first），支持编号重新执行；用户可以通过上、下键在自己输入的历史内容之间翻页，包括上一次执行co-shell时的内容 [BUILD-68]
- [x] FEATURE-68 结果处理模式选项：minimal（极简，默认，直接返回原始输出）/ explain（解释）/ analyze（分析）/ free（自由），通过 .set result-mode 配置 [BUILD-70]
- [x] FIX-69 修复 config.json 中 max_iterations=0 导致 Agent 使用内部默认值 10 的问题；新增 .set max-iterations 运行时修改支持 [BUILD-72]
- [x] FIX-70 修复 DeepSeek thinking 模式下 reasoning_content 未正确传回导致 API 400 错误 [BUILD-73]


## v0.2.0 — Beta

> **状态**: ✅ 已完成
> **目标日期**: 2026-04-27
> **里程碑**: 功能完善，可日常使用

### 功能清单

- [x] FEATURE-26 多轮对话上下文管理 [BUILD-87]
- [x] FEATURE-27 系统命令执行安全沙箱 [BUILD-87]
- [x] FEATURE-28 命令执行确认机制（危险操作）[BUILD-87]
- [x] ENHANCEMENT-29 更好的错误处理和用户提示 [BUILD-87]
- [x] FEATURE-30 配置文件热重载 [BUILD-87]
- [x] FIX-62 修复流式输出完成后再次调用非流式API导致迭代次数减半的问题 [BUILD-43]
- [x] FEATURE-71 Agent 内置文件操作工具：read_file（读取文件，支持行号范围）、search_files（正则搜索文件内容）、list_code_definition_names（列出目录中源码定义）、replace_in_file（SEARCH/REPLACE 精确替换）、write_to_file（写入/覆盖文件，自动创建目录）[BUILD-78]
- [x] ENHANCEMENT-72 Workspace 架构改造：支持 --workspace 命令行参数指定工作区，默认为当前运行目录；workspace 下自动创建 bin/、db/、log/、output/、tmp/ 子目录；配置文件、记忆数据库、日志、工具运行路径均基于 workspace；更新 USAGE.md 文档 [BUILD-79]
- [x] FEATURE-73 Agent 系统提示词多语言支持：核心提示词（buildSystemPromptWithMode）根据当前 i18n 语言设置自动切换中文/英文版本，确保 LLM 使用用户语言进行交互 [BUILD-80]
- [x] FEATURE-74 新增创建sub-agent方法，当前co-shell可以通过"co-shell -w sub-agents/1 指令"的方式，启动一个预先准备好workspace的新进程作为当前co-shell的影分身（sub-agent）。这个准备一般是用户准备，当然，co-shell也可以帮用户准备。当前co-shell应该创建一个线程来监视sub-agent进程的执行情况，母子agent在同一个终端上共享标准输入、输出流，执行完毕后负责收集sub-agent的工作成果反馈，并向用户汇报。[BUILD-83]
- [x] FEATURE-75 新增定时执行任务方法，co-shell可调用定时器方法，定时启动一个sub-agent，该方法接收一个定时表达式（类似于crontab表达式）和一个指令，到时后启动一个sub-agent，把定时方法中的指令传给sub-agent，指令中应该告诉这个sub-agent，他是被定时启动的。[BUILD-86]
- [x] ENHANCEMENT-76 新增 -c/--config 命令行参数，允许用户单独指定配置文件路径，优先级高于 {workspace}/config.json；新增 config.LoadFromFile() 方法支持从任意路径加载配置；Save() 保存到实际加载的路径；sub-agent 通过 CO_SHELL_CONFIG_PATH 环境变量继承父进程配置文件路径；sub-agent 固定 workspace 到 sub-agents/{id}/，母 agent 在 memory 中维护清单 [BUILD-83]
- [x] FIX-77 sub-agent 指令改为非 flag 参数传递，避免 -c 参数冲突导致配置文件路径丢失 [BUILD-83]
- [x] FEATURE-78 新增 --name/-n 命令行参数，支持自定义 agent 名称，用于标识日志、sub-agent workspace 命名等；Agent 新增 SetName/Name/Said 方法，Said() 输出带时间戳和 agent 名称的多语言消息 [BUILD-84]
- [x] ENHANCEMENT-79 帮助信息中新增 --name/-n 选项说明；i18n 新增 KeyAgentSaid 和 KeyCLIHelpName 翻译键 [BUILD-84]

---

## v0.3.0 — RC1

> **状态**: 🚧 开发中
> **目标日期**: 2026-05-30
> **里程碑**: 功能完整，可发布预览

### 功能清单

- [x] FEATURE-61 增加对多模态模型的支持（图片输入、多模态理解）[BUILD-88]
- [x] FIX-79 修复设置向导中获取到空模型列表时 panic（index out of range）[BUILD-89]
- [x] FEATURE-80 Agent 身份自定义：config 新增 AgentName/AgentDescription/AgentPrinciples 字段，系统提示词中注入身份信息，支持通过 .set name/description/principles 运行时修改 [BUILD-90]
- [x] FEATURE-81 补齐缺失的 CLI 命令行参数：新增 --temperature/--max-tokens/--show-thinking/--show-command/--show-output/--confirm-command/--result-mode/--description/--principles/--tool-timeout/--cmd-timeout/--llm-timeout 共 12 个 CLI 标志，遵循 CLI > 配置文件 > 默认值优先级 [BUILD-91]
- [x] ENHANCEMENT-82 在 --help 示例中增加 3 个新参数使用示例（--temperature、--show-thinking/--show-command、--result-mode）[BUILD-92]
- [x] FIX-83 修复帮助信息中默认值与实际不一致的问题：1) --config 显示"~/.co-shell/config.json"实际为"{workspace}/config.json"；2) --max-iterations 显示"默认 10"实际 config 默认值为 1000；3) .set 参数清单缺少 max-retries 参数说明；4) .set 参数清单缺少 result-mode/name/description/principles 参数说明；5) --help 缺少 --image/-i 参数说明 [BUILD-95]
- [x] ENHANCEMENT-84 优化多模态能力：在配置文件模型信息中增加视觉识别能力标记（vision_support），可通过命令行参数（--vision）、内部命令（.set vision）设置，wizard 选择模型后自动通过模型 API 获取视觉支持信息；优化系统提示词中图片识别相关描述，完善 --image 参数在 sub-agent 间的传递机制；ListModels 返回类型从 []string 改为 []ModelInfo（含 VisionSupport）；传入 --image 但模型不支持 vision 时输出错误并退出；模型支持视觉时在欢迎信息版本号旁显示 👀 标识；更新版本号 v0.1.0 -> v0.3.0 [BUILD-101]
- [x] ENHANCEMENT-85 多模态图片缓存机制：建立图片缓存，--image 传入的图片路径不再自动清空；新增 .image 内置命令（add/remove/clear/list）；新增 add_images/remove_images/clear_images 三个 LLM 工具，让大模型能操纵图片缓存；去掉 sub-agent 图像识别相关系统提示词 [BUILD-109]
- [x] FEATURE-36 任务计划管理（Task Plan Management）：提供 create_task_plan / update_task_step / insert_task_steps / remove_task_steps / view_task_plan / list_task_plans 六个 LLM 工具及 .plan 内置命令（list/view/create/insert/remove/update），让大模型和用户都能规划制定多步骤任务计划、跟踪进度、根据实际情况调整计划，数据持久化到 bbolt [BUILD-113]
- [x] FEATURE-39 批量命令执行，如果上级Agent用户在确认是否执行命令时选择了All，则子agent也继承这个选项 [BUILD-114]
- [x] FEATURE-87 对话上下文限制（context-limit）：支持通过 .set context-limit 配置发送给 LLM 的历史消息数量（0=仅当前输入，-1=全部，N=最近N条），始终保留用户最新输入 [BUILD-115]
- [x] FEATURE-88 持久化记忆管理：新增 memory 包（memory.Manager），支持对话消息的持久化存储、历史切片检索（GetHistorySlice）和关键词搜索（Search）；新增 store.SaveMemory/GetMemory/SearchMemory 方法；新增 cmd/memory.go 恢复 .memory 内置命令；新增 get_history_slice 和 memory_search 两个 LLM 工具 [BUILD-115]
- [x] FEATURE-89 记忆功能开关：支持通过配置文件（config.json）、命令行参数（--memory-enabled/--memory-disabled）和 REPL 命令（.set memory-enabled）控制记忆功能的开启/关闭。关闭时，get_history_slice 和 memory_search 两个 LLM 工具不可用，LLM 无法调用。
- [x] FEATURE-90 任务计划（checklist）单例模式改造：同一时间只能有一个任务计划；有未完成步骤时不能新建计划，只能调整（插入/删除未完成子任务）；所有步骤完成后才能新建计划，旧计划自动归档到记忆；create_task_plan / update_task_step / insert_task_steps / remove_task_steps / view_task_plan / list_task_plans 六个 LLM 工具及 .plan 命令均适配单 plan 模式，不再需要 plan_id 参数 [BUILD-116]
- [x] FIX-91 解决用 .set 设置参数之后，必须重启才能生效的问题。[BUILD-117]
- [x] FEATURE-92 LLM 前端输出模式开关：支持精简（compact）、标准（normal）和调试（debug）三种模式。精简模式：只显示 LLM 返回的内容，隐藏所有工具调用信息；标准模式：在精简基础上，显示工具调用方法名，但不显示调用细节和方法返回结果；调试模式：在标准基础上，显示工具调用输入参数和返回结果。通过 .set output-mode 配置、--output-mode 命令行参数、config.json 持久化。[BUILD-119]
- [ ] FEATURE-93 日历与待办事项管理：提供日历功能，支持记录和管理待办事项（todo）。提供 .calendar 内置命令（add/list/remove/update）管理待办事项；提供 add_todo / list_todos / update_todo / remove_todo 四个 LLM 工具，让大模型能操作待办事项；数据持久化到 bbolt。如果系统有日历应用（如 macOS 日历），提供选项帮助用户将待办事项同步到系统日历。
- [ ] FEATURE-94 命令执行审计功能：在执行 execute_command 工具调用时，先将命令发送给 LLM 进行安全风险分析，LLM 判断命令是否存在风险（如删除文件、修改系统配置、网络操作等）。如果存在风险，提示用户确认后才能执行。支持通过 .set audit-enabled 配置、--audit-enabled/--audit-disabled 命令行参数、config.json 控制审计功能的开启/关闭。
- [ ] FEATURE-95 sub-agent 开关：新增 subagent-enabled 配置项，支持通过 .set subagent-enabled 配置、--subagent-enabled/--subagent-disabled 命令行参数、config.json 控制是否允许大模型调用 launch_sub_agent 工具。关闭时，launch_sub_agent 工具不可用，LLM 无法调用。
- [ ] FEATURE-96 checklist 上下文重置：当 create_task_plan 创建新 checklist 或 insert_task_steps/remove_task_steps 更新 checklist 后，将当前 checklist 内容作为新的任务目标注入 LLM 上下文，忽略 checklist 更新前的所有对话记录，确保 LLM 聚焦于当前任务目标。
- [ ] FEATURE-97 对话管理命令：新增 .clear 内置命令，用于清空本次会话中所有历史对话内容（包括系统提示词和用户/助手消息），重置对话上下文，让 LLM 从全新状态开始。支持通过 .clear 命令一键清空，无需重启 co-shell。
- [x] FIX-98 修复可能无限迭代的问题
- [x] FIX-99 context-limit、memory-enabled 在 REPL 中显示的值简化 [BUILD-120]
- [x] ENHANCEMENT-100 优化search_files方法，增加返回内容及长度保护：1、忽略二进制文件；2、开头需要给出有多少匹配的文件，如：”在 agent/ 目录下找到 5 处匹配模式 "fmt.Errorf" 的结果：“; 3、匹配到一个文件，先输出文件名和带上下文的匹配范围，如：“agent/loop.go:40-44:”，然后再输出匹配行及上下文的内容，如："40: 	multimodalMsg, err := a.buildMultimodalMessage(userInput, a.imagePaths)\n41: 	if err != nil {\n42: 		return "", fmt.Errorf("cannot build multimodal message: %w", err)\n43: 	}\n44: 	a.messages = append(a.messages, multimodalMsg)"; 4、内容长度需要有所保护，如果一行的长度超长，需要在首行提示用户，如：“在 agent/ 目录下找到 5 处匹配模式 "fmt.Errorf" 的结果，但有1行内容超长返回被截断（见行尾标注）：”，超长行末尾为：“（...后面被截断128000字符）”；5、如果总内容超过规定的最大字节数，则在开头需要进行提示，如：“在 agent/ 目录下至少找到 5 处匹配模式 "fmt.Errorf" 的结果，由于内容超长，无法全部返回：”，引发超长的最后一行需要被去掉，结尾最后一行参照上述4的处理方法；6、最大行字符长度（默认8192）、最大合计返回字节数（默认65536）、上下文数（默认为5行）可以通过命令行、REPL、和配置文件设置。


## v1.0.0 — 正式版

> **状态**: 🚧 开发中
> **目标日期**: 2026-05-07
> **里程碑**: 稳定可用，可发布

### 功能清单

- [ ] FEATURE-44 Homebrew 安装支持
- [ ] FEATURE-45 自动更新机制
- [ ] FEATURE-48 主题系统
- [ ] ENHANCEMENT-49 性能基准测试
- [ ] FEATURE-50 完整文档站


## v1.1.0 — 增强版

> **状态**: 💡 构想中
> **里程碑**: 生态建设

### 功能清单

- [ ] FEATURE-31 MCP Server 自动重连
- [ ] FEATURE-34 插件系统（WASM 插件支持）
- [ ] FEATURE-35 自定义 Prompt 模板
- [ ] FEATURE-37 输出格式化（JSON/表格/树形）
- [ ] FEATURE-38 命令别名
- [ ] FEATURE-40 管道支持（Pipe）
- [ ] FEATURE-56 MCP Hub 集成（发现和安装 MCP Server）
- [ ] FEATURE-57 社区插件市场
- [ ] FEATURE-58 多 Agent 协作
- [ ] FEATURE-59 可视化工作流编排
- [ ] FEATURE-60 远程执行（SSH）
- [ ] FEATURE-86 支持永久记忆接入数据库

### 优化

- [ ] ENHANCEMENT-41 启动速度优化
- [ ] ENHANCEMENT-42 内存使用优化
- [ ] ENHANCEMENT-43 大模型响应缓存


## 版本发布记录

| 版本 | 日期 | 状态 | 说明 |
|---|---|---|---|
| v0.1.0 | 2026-04-25 | ✅ 已完成 | Alpha 预览版 |
| v0.2.0 | 2026-04-27 | ✅ 已完成 | Beta 测试版 |
| v0.3.0 | 2026-04-29 | 🚧 开发中 | 发布候选版 |
| v1.0.0 | 2026-05-02 | 🚧 开发中 | 正式版 |

### 发布条件

每个版本发布前需满足以下条件：

- [ ] `go build ./...` 编译通过
- [ ] `go vet ./...` 无警告
- [ ] 核心功能手动测试通过
- [ ] USAGE.md 使用文档完整
- [ ] CHANGELOG.md 更新

---

## 版本命名规范

```
v{major}.{minor}.{patch}
  │       │       └── 补丁版本：Bug 修复、小改进
  │       └────────── 次版本：新功能、非破坏性变更
  └────────────────── 主版本：重大变更、不兼容更新
```

### 状态标签

| 标签 | 含义 |
|---|---|
| 💡 构想中 | 初步想法，尚未开始 |
| 📋 规划中 | 已确定计划，待开发 |
| 🚧 开发中 | 正在开发 |
| ✅ 已完成 | 开发完成 |
| 🚀 已发布 | 正式发布 |

### 编号前缀说明

| 前缀 | 含义 |
|---|---|
| FEATURE- | 新特性（New Feature） |
| ENHANCEMENT- | 改进（Enhancement） |
| FIX- | Bug 修复（Bug Fix） |

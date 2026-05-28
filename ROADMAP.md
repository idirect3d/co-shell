# co-shell 版本计划

> 版本号格式：`v{major}.{minor}.{patch}`

---

## 当前版本

> **版本**: v0.5.1
> **BUILD**: 193









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
- [x] FIX-170 修复配置向导添加模型后 LLM 未配置问题：配置向导添加的模型未同步到 ModelManager，导致 Agent 初始化时使用 noopClient [BUILD-170]


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

> **状态**: ✅ 已完成
> **目标日期**: 2026-05-01
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
- [x] FIX-135 修复 wizard 设置后必须重启才能生效的问题。[BUILD-149]
- [x] FEATURE-92 LLM 前端输出模式开关：支持精简（compact）、标准（normal）和调试（debug）三种模式。精简模式：只显示 LLM 返回的内容，隐藏所有工具调用信息；标准模式：在精简基础上，显示工具调用方法名，但不显示调用细节和方法返回结果；调试模式：在标准基础上，显示工具调用输入参数和返回结果。通过 .set output-mode 配置、--output-mode 命令行参数、config.json 持久化。[BUILD-119]
- [x] FEATURE-95 sub-agent 开关：新增 subagent-enabled 配置项，支持通过 .set subagent-enabled 配置、--subagent-enabled/--subagent-disabled 命令行参数、config.json 控制是否允许大模型调用 launch_sub_agent 工具。关闭时，launch_sub_agent 工具不可用，LLM 无法调用。[BUILD-122]
- [x] FEATURE-96 优化memory、checklist和上下文管理：1、get_history_slice改为get_memory_slice；2、新user、assistant消息在加入messages时，content开头增加格式化的日期和时间前缀，如：“2026-05-01 09:51:01 - ”；3、增加一个messages指示器，标记发送LLM时的起始位置标记，当 create_task_plan 创建新 checklist 或 insert_task_steps/remove_task_steps 更新 checklist 后，将当前 checklist 内容作为助手提出的新的任务目标追加到 LLM 上下文messages（但不进入memory），再将message指示器移到最后，相当于忽略 checklist 更新前的所有对话记录，确保 LLM 聚焦于当前任务目标；4、在.session显示messages清单时，指示器对应的那一条在左侧标星；5、在双方向messages插入message时，需要同时插入memory；6、memory_search返回结果中的content，设一个最长限制（M），超长之后的内容为“...“，最长召回记录数为N，其中M默认为512，N默认为100，M和N都可以通过命令行、REPL、和配置文件设置。[BUILD-125]
- [x] FEATURE-97 对话管理命令：新增 .new 内置命令，用于清空本次会话中所有历史对话内容（包括系统提示词和用户/助手消息），重置对话上下文，让 LLM 从全新状态开始。支持通过 .new 命令一键清空，无需重启 co-shell。[BUILD-123]
- [x] FIX-98 修复可能无限迭代的问题
- [x] FIX-99 context-limit、memory-enabled 在 REPL 中显示的值简化 [BUILD-120]
- [x] ENHANCEMENT-100 优化search_files方法，增加返回内容及长度保护：1、忽略二进制文件；2、开头需要给出有多少匹配的文件，如：”在 agent/ 目录下找到 5 处匹配模式 "fmt.Errorf" 的结果：“; 3、匹配到一个文件，先输出文件名和带上下文的匹配范围，如：“agent/loop.go:40-44:”，然后再输出匹配行及上下文的内容，如："40: 	multimodalMsg, err := a.buildMultimodalMessage(userInput, a.imagePaths)\n41: 	if err != nil {\n42: 		return "", fmt.Errorf("cannot build multimodal message: %w", err)\n43: 	}\n44: 	a.messages = append(a.messages, multimodalMsg)"; 4、内容长度需要有所保护，如果一行的长度超长，需要在首行提示用户，如：“在 agent/ 目录下找到 5 处匹配模式 "fmt.Errorf" 的结果，但有1行内容超长返回被截断（见行尾标注）：”，超长行末尾为：“（...后面被截断128000字符）”；5、如果总内容超过规定的最大字节数，则在开头需要进行提示，如：“在 agent/ 目录下至少找到 5 处匹配模式 "fmt.Errorf" 的结果，由于内容超长，无法全部返回：”，引发超长的最后一行需要被去掉，结尾最后一行参照上述4的处理方法；6、最大行字符长度（默认8192）、最大合计返回字节数（默认65536）、上下文数（默认为5行）可以通过命令行、REPL、和配置文件设置。[BUILD-121]
- [x] FEATURE-102 增加对小米最新模型的调用支持。[BUILD-131]
- [x] ENHANCEMENT-103 改进loop.go程序过长的问题，将其中的方法分类整理后，将其拆解为更小的文件。[BUILD-130]
- [x] FEATURE-104 用户确认操作时，可以输入一个数字，表示批准后面多少次执行命令。[BUILD-127]
- [x] FEATURE-107 相同错误反复出现次数限制和提示用户处理，每次任务开始时，构建一个新的错误提示计数器，每次LLM返回错误后，以内容为键值进行计数，当某一个键值的计数超过单个错误最大值（默认为10），或键值数超过错误种类数最大值（默认为100），则提示用户，A：同意所有，C：取消，输入一个数字：自动同意数字所指示的次数，输入一段文字：将文字传给LLM建议下一步处理模式，回车：同意。单个错误最大值和错误种类数最大值可以通过命令行、REPL、和配置文件设置。[BUILD-128]
- [x] FEATURE-108 补充成果样例。[BUILD-129]
- [x] FEATURE-109 增加对GLM（Z.ai）最新模型的调用支持。[BUILD-132]
- [x] ENHANCEMENT-110 大模型设置向导选择大模型类型后，如果选择的是内置支持的几个商用模型，则直接跳过地址设置环节（因为是固定的地址）。[BUILD-133]
- [x] ENHANCEMENT-111 改进命令提示页和错误提示页，增加风险警示。[BUILD-134]
- [x] FEATURE-112 增加thinking开关设置，可以通过命令行、REPL、和配置文件设置。[BUILD-135]
- [x] FEATURE-115 Agent 增加 TokenUsage 和 ResetTokenUsage 方法，用于获取和重置累计 Token 用量统计 [BUILD-135]
- [x] FEATURE-117 改进公文风格 Word 文档生成：优化 DOCX 生成质量，支持更美观的排版样式（字体、字号、行距、标题样式等），支持公文格式规范（红头文件、公文编号、落款等），增强 Markdown 到 DOCX 的转换效果。

## v0.4.0 — RC2

> **状态**: ✅ 已完成
> **目标日期**: 2026-05-05
> **里程碑**: 功能完善，稳定可用

### 功能清单

- [x] FEATURE-131 为LLM提供一个设置系统参数的方法（相当于.set REPL命令），以便可以为用户提供一个更方便的设置界面，提升用户体验。这个方法应该支持所有参数设置，但是，在LLM执行时，需要向用户明确提示，co-shell将自主修改的参数，原来什么值，修改为什么值，为什么修改，声明风险，并征求用户同意。用户可以选择同意、拒绝、或暂停（输入其他文字消息）。[BUILD-143]
- [x] ENHANCEMENT-118 .session 显示消息清单时序号从 0 开始：系统消息序号为 0，用户第 1 条消息序号为 1，以此类推。[BUILD-136]
- [x] ENHANCEMENT-119 优化文件读取、写入工具，以便增强LLM对源码的控制力。[BUILD-141]
- [x] FEATURE-122 新增日志级别控制，可以通过 .set log debug/info/warn/error/off 控制日志输出级别，支持 --log-level 命令行参数和 config.json 持久化。[BUILD-138]
- [x] ENHANCEMENT-124 改进工具调用和执行系统命令的超时时间：用户可设置最低超时时间，LLM 可在工具调用中传入 timeout_seconds 参数自行预判超时时间，执行时取两者最大值。[BUILD-142]
- [x] ENHANCEMENT-126 优化输出模式控制，梳理与LLM迭代相关的输出，合理分类后，进行参数话控制，以便用户能够更清晰的控制：LLM返回的thinking内容（show-llm-thinking，默认开）、LLM返回的主要内容（show-llm-content，默认开）、输出Tool-call名（show-tool，默认开）、输出Tool-call输入参数（show-tool-input，默认关）、输出Tool-call返回数据（show-tool-output，默认关）、输出系统命令行（show-command，默认开）、输出命令返回数据（show-command-output，默认开），删除现有的show-output、output-mode两个快关。[BUILD-139]
- [x] FEATURE-127 增加Agent身份定义的默认值，以便发行版能够具备比较一致的行为，对于身份描述内容需要支持多语言。[BUILD-140]
- [x] ENHANCEMENT-132 通过不同表情符号区分不同角色输出的内容，以便用户能够更好的看清哪些是自己说的([👤]>)，哪些是LLM说的([🐚]>)，哪些是工具调用相关输入([⚙️]<)/输出([⚙️]>)，哪些是命令调用相关输入([🔴]<)/输出([🔴]>)，另外，颜色系统是可以配置是否启用的，默认为开。[BUILD-148]
- [x] ENHANCEMENT-134 模型API设置向导改进：1) 提供商列表增加Ollama选项（倒数第二），默认地址 http://localhost:11434/v1；2) 模型列表增加编号选项，输入数字选择模型，输入非数字则作为前缀匹配；3) 最终检测时列出全部地址和模型参数让用户确认；4) 最终测试成功后提示用户再次修改模型的方法。[BUILD-147]
- [x] FIX-134 修复 .set 显示时 description/principles 空值回退到 i18n 默认值的问题 [BUILD-151]

## v0.5.0 — Beta2

> **状态**: ✅ 已完成
> **目标日期**: 2026-05-12
> **里程碑**: 功能完善，稳定可用

### 功能清单

- [x] FEATURE-103 动态上下文调整，尝试让LLM决定取多少上下文。[BUILD-174]
  - 新增 `context_start_mode` 配置项，支持三种模式：
    - `window`：固定窗口模式，上下文为最后 N 条消息（N=context_limit）
    - `task`：任务模式（默认），上下文指针随任务边界自动移动
    - `smart`：智能模式，LLM 可通过 `adjust_context_start` 工具自行决定上下文起始位置
  - 新增 `adjust_context_start` LLM 工具，允许 LLM 动态调整消息指针位置
  - 新增命令行参数 `--context-start`
  - 新增 `.set context-start` REPL 命令
  - 新增 `list_settings` 工具显示上下文起始模式参数
- [x] FEATURE-105 提供用户选择对话框：新增 ask_followup_question 工具，支持 question + options 格式，用户输入数字序号选择选项。[BUILD-173]
- [x] FEATURE-116 增加显示token数功能，每次任务（checklist）完成时可以统计本次任务所用的token数，另外还有一个从程序启动到现在的总数。[BUILD-162]
- [x] FEATURE-123 新增对接飞书功能，以便用户可以通过即时通讯软件，以跟特定机器人会话对话的方式，与co-shell交互。[BUILD-153]
  - FIX-123a 修复 WebSocket ACK 确认机制：收到事件后立即写回 ACK（`{"id":"...","type":"pong"}`），防止飞书 3 秒超时重推
  - FIX-123b 修复 Ping/Pong 心跳处理：设置 WebSocket 协议层的 PongHandler，自动响应服务端 Ping 帧；设置 ReadDeadline 检测连接状态
  - FIX-123c 优化日志：pong 心跳消息仅在 debug 级别输出，减少日志噪音
- [x] FIX-133 修复 --help 中缺少 --init-capabilities 和 --init-rules 参数说明的问题 [BUILD-161]
- [x] FEATURE-137 新增co-shell之间相互调用能力，以便Agent可以向人和人那样分工和交流，让形成真正的AI团队成为可能。[BUILD-161]
- [x] FEATURE-138 增加删除记忆memory的REPL和tool call方法，可以按last_from和last_to删除一段记忆。[BUILD-154]
- [x] FEATURE-139 launch_sub_agent 工具改为 sub_agent_name 字符串参数，仅调用已存在的 agent workspace，不再自动创建 workspace。[BUILD-155]
- [x] FEATURE-140 新增 ASCII art Logo 显示功能：启动时显示 co-shell 字符 Logo，支持通过 .set show-logo on|off 和 --show-logo 命令行参数控制显示，Logo 文件通过 go:embed 嵌入程序。[BUILD-156]
- [x] ENHANCEMENT-141 工具调用支持检测与配置：在配置模型时自动检测模型是否支持工具调用（function calling），新增 `toolcall-enabled` 配置项，支持通过 .set toolcall-enabled、--toolcall-enabled 命令行参数、config.json 控制。配置向导检测模型时自动测试工具调用能力，不支持则关闭开关，支持则默认开启。当开关关闭时，Agent 不向 LLM 传递工具定义，LLM 仅以纯文本模式工作。[BUILD-157]
- [x] ENHANCEMENT-142 模型最大上下文长度（max-model-len）自动检测：在配置模型时，通过模型列表 API 获取模型的 `max_model_len` 值并自动记录到配置中。新增 `max-model-len` 配置项，在 `.set` 命令和配置显示中展示该值。配置向导选择模型后自动检测并记录模型的最大上下文长度，为未来上下文管理提供依据。[BUILD-159]
- [x] ENHANCEMENT-143 优化模型工具调用自动检测以及手工配置机制：在配置模型检测时，增加工具调用支持检测选项，检测完成后在最终配置摘要中明确显示工具调用状态（支持/不支持、开启/关闭）。wizard 设置向导的最终确认界面（包括预设供应商和 OpenAI 兼容模式）均展示工具调用开关状态，让用户清晰了解当前模型的工具调用能力。[BUILD-160]
- [x] ENHANCEMENT-140 新增 Top-P、Top-K、重复惩罚（repetition-penalty）三个采样参数的完整支持：LLM 客户端接口新增 SetTopP/SetTopK/SetRepetitionPenalty 方法；chatRequestJSON 结构体新增对应字段，Temperature 改为指针类型以支持 -1 不发送；config 新增 top_p/top_k/repetition_penalty 配置项及默认值；CLI 新增 --top-p/--top-k/--repetition-penalty 命令行参数；.set 命令和 LLM 工具支持运行时修改；i18n 中英文翻译支持；帮助信息展示。[BUILD-161]
- [x] ENHANCEMENT-144 新增 token-usage 开关，支持通过命令行（--token-usage）、REPL（.set token-usage）、配置文件（config.json）配置为 on/off/none，分别对应显示 token 用量、不显示、不发送 stream_options.include_usage 参数。[BUILD-163]
- [x] ENHANCEMENT-145 参数配置策略优化：1）能力测试方法将 Temperature 都设置为 0；2）新增 --body-add 命令行参数和 .body-add/.body-remove/.body-display REPL 命令，支持向 LLM 请求体 JSON 中增加/删除/查看自定义属性。[BUILD-164]
- [x] FIX-146 修复 LLM HTTP 报错时问题消息残留导致无限循环的问题：当 role=assistant 的消息导致 HTTP 报错时，将该消息及之后的消息从上下文队列中移除，并将移除内容拼接到错误提示消息中返回给 LLM；当 role=user 的消息导致 HTTP 报错时，立即退出迭代，将错误提示给用户。[BUILD-165]
- [x] FEATURE-147 多模型切换和参数模版管理：支持配置多个模型参数并快速切换，系统内置大模型供应商模板，用户可从模板选择模型类型填入参数，程序根据任务能力自动选择优先级最高的模型。[BUILD-165]
- [x] FEATURE-148 模型参数模板增加可自定义属性：ModelTemplate 新增 DefaultParams 字段，为每个内置模板设置合适的默认参数（如 DeepSeek 的 thinking 配置、Qwen 的 extra_body 等），创建模型时自动继承到 ModelConfig.CustomParams，切换模型时合并到 bodyAdditions 发送给 LLM。支持 "None" 字符串值表示不发送该参数。[BUILD-166]
- [x] FEATURE-149 .model set-param 命令支持设置模型自定义参数：新增 .model set-param <id> <key> <value> 子命令，支持设置 None 表示不发送该属性，支持 JSON 格式的值（自动解析）和纯字符串值。model info 显示自定义参数列表。[BUILD-167]
- [x] ENHANCEMENT-151 改进.model list显示格式：第一行显示 `<No>.[<id>][<provider>][<endpoint>:<model>][<max_model_len>][<capabilities>]`，第二行显示模型参数（temperature/top-k/top-p等），不再显示 name 字段；.model add向导自动从API获取max_model_len并保存到ModelConfig；清理已废弃的wizard包（wizard/wizard.go、wizard/input.go、wizard/terminal.go）。[BUILD-169]
- [x] ENHANCEMENT-152 移除全局LLM配置中的单模型参数：从LLMConfig中移除Provider/APIKey/Endpoint/Model四个字段，将Temperature/MaxTokens等参数改为字符串覆盖类型，所有LLM调用参数统一从ModelConfig获取。[BUILD-168]
- [x] FEATURE-171 模型视觉能力自动同步：新增模型具备视觉能力时自动开启全局 vision_support，移除后无视觉能力模型时自动关闭，修复 API Key 短字符 panic。[BUILD-171]
- [x] ENHANCEMENT-175 优化LLM调用参数的默认值，以便系统具有较高的适应性：temperature 默认值从 0.7 调整为 0.5；top_p/top_k/repetition_penalty 改为缺省为 None（不发送），即设置为 -1。[BUILD-175]
- [x] FEATURE-176 会话持久化功能：实现程序中断重启后对话上下文自动恢复。新增 SessionData 结构和 SaveSession/LoadSession/ClearSession 存储方法；Agent 新增 RestoreSession/PersistSession 方法；RunStream 在请求完成后自动持久化会话；程序启动时自动恢复上次会话。[BUILD-176]
- [x] FIX-179 修复 LLM 输出死循环问题：
   - **流式循环检测**：监控 LLM 流式输出中的重复模式，当相同归一化模式在窗口中出现次数达到阈值（默认5次）时主动停止并向 LLM 发送纠正提示。支持 `loop-detect-enabled`、`loop-detect-threshold`、`loop-detect-max-window` 配置。
   - **消息级去重检测**：新增旁路重复监测机制，在向 session 添加消息前，抽取 20% 随机特征词按顺序匹配历史消息，若特征匹配率 >= 60% 则进行 Jaccard 相似度计算，相似度 >= 85% 判定为重复。连续重复达到 3 次时发送警告提示。支持通过 `dedup-enabled`、`dedup-feature-ratio`、`dedup-match-ratio`、`dedup-similarity-threshold`、`dedup-max-history`、`dedup-repeat-limit` 配置项控制。[BUILD-179]
- [x] ENHANCEMENT-177 在 write_to_file 工具描述中增加提醒，建议 LLM 尽量使用 replace_in_file 而不是重写文件来修正文件错误，以避免重写复杂文件依旧产生新问题。[BUILD-176]
- [x] FEATURE-178 修改对话上下文时间戳前缀格式：从 "2026-05-12 10:15:30 - " 改为 "在 2026-05-12 10:15:30 说："，提升用户可读性。[BUILD-187]
- [x] FIX-97 修正qwen3.6在遇到写大文件时LLM输出无限循环的问题：[BUILD-177]
  - 在流式输出处增加每种event的日志输出（event类型、当前块内容），证明循环不是程序直接造成的
  - 修复 REPL 输出格式：command/tool_call 提示符从行首开始显示
  - 修复 loop.go：带 tool_calls 的 assistant 消息去掉时间戳前缀和索引前缀
- [x] FIX-180 修复 LLM 调用 write_to_file 时缺少 content 参数导致死循环的问题：增强工具参数描述明确性、改进错误反馈格式，帮助 LLM 更快理解并修正参数缺失问题。[BUILD-177]
- [x] FIX-181 修正qwen3.6在遇到写大文件时LLM输出无限循环的问题：[BUILD-181]
  - 在流式输出处增加每种event的日志输出（event类型、当前块内容），证明循环不是程序直接造成的
  - 新增 log.Raw 方法用于 SSE 原始数据流追踪
  - 增强 LoopDetector 单词重复检测模式（同一单词重复、交替两词模式）
  - 完善 streamLLMResponse debug 日志
  - 新增 docs/system-prompt-composition.md system prompt 组成文档
- [x] FEATURE-182 可配置的分模式自定义灵活的toolcall调用机制，先支持当前的标准OpenAI API tool call，增加实现类Cline XML式，并且可由用户通过配置文件自定义其他扩展模式，以便co-shell能够支持更多不同能力和工具调用标准的LLM，同时能够尝试避免一些LLM调用死循环的问题。[BUILD-187]
- [x] FIX-183 修复 .model add/switch 后模型切换不生效的问题：ModelManager 与 cfg.Models 双列表不同步导致 selectModelForCall 仍使用旧模型。在 cmd/model.go 的 saveModel/switchModel/removeModel/enableModel/disableModel/setPriority 方法中增加 modelMgr 同步逻辑。[BUILD-182]
- [x] FEATURE-184 工具调用确认机制扩展：1) 将所有工具调用（不限于 execute_command）都增加用户确认；2) 每个工具增加 confirm-tool 控制参数，可通过命令行、REPL、参数文件设置，默认需要用户确认；3) 用户确认时增加 G 选项（同意，且取消此方法需用户确认标志）；4) 数字计数器改为每个方法独立，当前任务结束后全部清 0。[BUILD-187]
- [x] FEATURE-86 支持永久记忆接入数据库。1）基于PostgreSQL进行设计；2）提供数据库连接配置方法，尽量简化数据库配置，仅提供地址（默认本地）、端口（数据库默认）、库名（默认为coshell_db，或其一个更好的名字）、schema使用默认public；3）如果配置数据库后，默认连接数据库，如果不通，则提示用户后依然使用本地库（不要因为远端数据库无法连接影响整体使用）。[BUILD-183]
- [x] FEATURE-185 程序默认使用当前工作目录名（最后一层）作为 agent name。用户依旧可以通过已经实现的命令行参数、REPL、配置文件配置。[BUILD-184]
- [x] FEATURE-186 数据库配置改为子命令模式：`.set db enabled on`、`.set db host 10.0.0.1` 等，模仿 confirm-tool 方式做成配置子集。DB 配置显示移到「记忆与上下文」分组中。新增交互式配置向导，首次运行 `.set db` 时自动引导用户完成 PostgreSQL 连接配置，支持连接测试和 bbolt 数据迁移。[BUILD-186]
- [x] FEATURE-187 改进 .model 子命令交互体验：switch/remove/enable/disable/info/set-priority/set-param 等需要选择模型的命令，当不提供模型 ID 参数时，先显示模型列表让用户通过编号选择，提升易用性。[BUILD-188]
- [x] ENHANCEMENT-188 XML 模式数组参数统一使用 `<item>` 标签：将 parseXMLChildrenToJSON/buildXMLToolDescription/buildXMLToolPrompt 中的 element 统一改为 item；修复 i18n 系统提示词中反引号导致 raw string literal 提前结束的问题；同步更新中英文示例。[BUILD-190]
- [x] FIX-189 修复 API URL 自动拼写时，如果 base URL 中已经有 /v1 后缀，不再重复添加 /v1。[BUILD-191]


## v0.5.1 — 补丁版

> **状态**: ✅ 已完成
> **发布日期**: 2026-05-28
> **里程碑**: Bug 修复

### 功能清单

- [x] 在 ROADMAP.md 中新建 v0.5.1 版本计划段，更新版本发布记录表
- [x] 更新 main.go 版本常量从 v0.5.0-Beta2 改为 v0.5.1
- [x] FIX-190 增强循环检测能力：新增内容级循环检测（checkContentLoop），在每次收到 LLM 流式输出 chunk 时立即检测 accumulated 内容中是否存在重复文本块（整段重复、URL编码重复字符等），使用滑动窗口自动匹配最佳块大小，复用现有 threshold 参数 [BUILD-192]
- [x] FIX-190 补充：新增 attempt_completion 工具（function calling + XML 模式），修复用户消息模板未应用的问题 [BUILD-193]

## v0.6.0 — Beta3

> **状态**: 🚧 开发中
> **目标日期**: 2026-06-01
> **里程碑**: 功能完善，稳定可用

### 功能清单

- [ ] FEATURE-93 日历与待办事项管理：提供日历功能，支持记录和管理待办事项（todo）。提供 .calendar 内置命令（add/list/remove/update）管理待办事项；提供 add_todo / list_todos / update_todo / remove_todo 四个 LLM 工具，让大模型能操作待办事项；数据持久化到 bbolt。如果系统有日历应用（如 macOS 日历），提供选项帮助用户将待办事项同步到系统日历。
- [ ] FEATURE-94 命令执行审计功能：在执行 execute_command 工具调用时，先将命令发送给 LLM 进行安全风险分析，LLM 判断命令是否存在风险（如删除文件、修改系统配置、网络操作等）。如果存在风险，提示用户确认后才能执行。支持通过 .set audit-enabled 配置、--audit-enabled/--audit-disabled 命令行参数、config.json 控制审计功能的开启/关闭。
- [ ] FEATURE-106 实现history命令翻页。
- [ ] FEATURE-45 自动更新机制（通过github）。
- [ ] ENHANCEMENT-49 性能基准测试。
- [ ] FEATURE-50 完整文档站。
- [ ] FEATURE-120 新增Excel文件编辑工具，以便为LLM增加直接（而不是现写程序）操控Excel的能力。
- [ ] FEATURE-121 新增Word文件编辑工具，以便为LLM增加直接（而不是现写程序）操控Word的能力。
- [ ] FEATURE-125 建立备用异常处理机制，以便在主LLM报错时，有另外独立的诊断渠道，可以通过异常信息库协助处理问题。
- [ ] FEATURE-128 增加移动端APP和co-shell-hub，以便用户能够在手机端安全的操控co-shell。
  - [x] 使用 Flutter 开发 iOS/Android 跨平台应用
  - [x] 所有移动端代码放在 mobile/ 目录下
  - [x] 通信协议：UDP + 首次请求密钥验证（无密钥不回包，降低被扫描风险）
  - [x] 实现 co-shell-hub（多 agent 管理服务端）
    - hub 监听 UDP 端口，处理握手验证
    - 管理多个 co-shell 实例的生命周期
    - 消息路由（根据 agent_id 转发）
    - 会话管理（创建/切换 agent）
  - [x] FEATURE-183 co-shell-hub 改进：简化认证流程，使用昵称+access key 替代公钥签名 [BUILD-180]
    - 新增 --add-client 命令注册移动端客户端
    - 新增 --gen-key 命令生成密钥对
    - 新增 --dev 开发模式（返回错误详情）
    - 新增 --log-dir/--log-level 日志配置
    - 握手协议简化：客户端发送 nickname + access_key
    - 消息自动添加 "<昵称>说：" 前缀
    - 移动端 Flutter 代码同步更新
  - [ ] 移动端支持多 agent 会话列表
  - [ ] 支持功能：聊天界面、语音输入、图片选择、任务计划查看
- [ ] FEATURE-129 增加语音识别模型，以便用户能够与co-shell通过语音进行沟通，计划支持GPU和CPU部署，可以通过co-shell自主安装所需要的模型和服务。
- [ ] FEATURE-136 在Agent策略中，增加让LLM预测用户下一步操作的机制，提供几个选项给用户选择，以便提升人机协同效率和自动化程序


## v1.0.0 — 正式版

> **状态**: 💡 构想中
> **目标日期**: 2026-07-01
> **里程碑**: 稳定可用，可发布


### 功能清单

- [ ] FEATURE-130 为co-shell正式发布中文名，以便中国用户能够记住并且具有亲和力，有利于推广。
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

### 优化

- [ ] ENHANCEMENT-41 启动速度优化
- [ ] ENHANCEMENT-42 内存使用优化
- [ ] ENHANCEMENT-43 大模型响应缓存


## 版本发布记录

| 版本 | 日期 | 状态 | 说明 |
|---|---|---|---|
| v0.1.0 | 2026-04-25 | ✅ 已完成 | Alpha 预览版 |
| v0.2.0 | 2026-04-27 | ✅ 已完成 | Beta 测试版 |
| v0.3.0 | 2026-04-29 | ✅ 已完成 | 发布候选版 |
| v0.4.0 | 2026-05-03 | ✅ 已完成 | 发布候选版 RC2 |
| v0.5.0 | 2026-05-12 | ✅ 已完成 | Beta2 测试版 |
| v0.5.1 | 2026-05-28 | ✅ 已完成 | 补丁版 |
| v0.6.0 | 2026-06-01 | 🚧 开发中 | Beta3 测试版 |
| v1.0.0 | 2026-07-01 | 💡 构想中 | 正式版 |


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

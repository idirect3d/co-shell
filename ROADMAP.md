# co-shell 版本计划

> 版本号格式：`v{major}.{minor}.{patch}`

---

## 当前版本

> **版本**: v0.6.0



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
- [x] FEATURE-11 系统命令直接运行（如果用户直接输入系统命令或执行程序在当前环境下可以直接执行，则直接执行用户的所有内容，而不用通过大模型解释。）[BUILD-1]
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
- [x] FEATURE-48 全局规则（--rule 命令行参数 / .rule 命令管理 / rule.md 自动加载） [BUILD-34]
- [x] ENHANCEMENT-50 配置文件增加注释说明 [BUILD-34]
- [x] FEATURE-51 命令行增强：.list tools/tasks 列出 LLM 工具和任务计划，.list commands 列出内置命令 [BUILD-45]
- [x] FEATURE-25 日志级别可配置：debug / info / warn / error / off [BUILD-36]
- [x] FEATURE-52 XML 输出格式支持（--xml / 模式切换），增强配置向导支持 XML 模式 [BUILD-42]
- [x] FEATURE-26 设置命令增强：新的设置键 confirm-file 等。.settings 子命令支持 Tab 自动补全，子命令统一化 [BUILD-47]
- [x] FEATURE-27 子 Agent 机制（快速执行独立任务而不阻塞主 Agent 循环） [BUILD-48]
- [x] FEATURE-28 图片理解支持，通过 --image 参数传入图片路径来读取图片，支持多图片，支持多模态 VLM [BUILD-49]
- [x] FEATURE-29 工具调用结果模式配置（result-mode，默认 full，支持 minimal） [BUILD-54]
- [x] FEATURE-30 .mcp 子命令增强：restart/remove/rename/add/man [BUILD-60]
- [x] FEATURE-32 多 Agent 支持（--agent 指定名称，配置独立） [BUILD-61]
- [x] FEATURE-33 .model 子命令管理模型列表 [BUILD-62]
- [x] FEATURE-46 输出格式化增强：支持 Markdown 渲染（-format markdown）和表格渲染 [BUILD-63]
- [x] FEATURE-37 配置热重载 [BUILD-65]
- [x] FEATURE-36 上下文管理（.context 命令）[BUILD-66]
- [x] FEATURE-39 REPl 持久化历史记录 [BUILD-67]
- [x] FEATURE-53 Tool/Agent 模式切换 [BUILD-68]
- [x] FEATURE-54 输出增强：代码块 Markdown 渲染 [BUILD-68]
- [x] FEATURE-55 上下文分支管理 [BUILD-69]
- [x] FEATURE-38 文件读取工具增强：支持 read_file start_line/end_line [BUILD-70]
- [x] FEATURE-39 代码搜索工具：search_files [BUILD-71]
- [x] FEATURE-40 基于关键字的记忆系统 [BUILD-72]
- [x] FEATURE-41 记忆自动总结 [BUILD-73]
- [x] FEATURE-42 交互体验优化：LLM 调用阶段显示当前阶段（思考/使用工具）[BUILD-74]
- [x] FEATURE-43 自动上下文清除：[BUILD-79]
- [x] FEATURE-44 增强显示：支持 LLM 思考和推理过程显示 [BUILD-80]
- [x] FEATURE-29 Toolcall 模式扩展 [BUILD-92]

## v0.2.0 — Beta（已完成）

> **状态**: ✅ 已完成
> **发布日期**: 2026-04-27


### 功能清单

- [x] FEATURE-58 REPL 功能增强：支持联想记忆、历史记录 [BUILD-2]
- [x] FEATURE-59 Agent 核心流程优化：支持消息裁剪、上下文窗口管理 [BUILD-2]
- [x] FEATURE-60 工具调用增强：支持文件写入确认、工具调用链 [BUILD-2]
- [x] FEATURE-61 跨平台构建：支持 Linux/macOS/Windows [BUILD-2]
- [x] FEATURE-62 指令模板：支持用户自定义指令模板，.prompt 命令管理，持久化 [BUILD-2]
- [x] FEATURE-63 Shell 集成：co-shell 默认用最后一级目录名为 agent name [BUILD-2]

## v0.3.0 — 发布候选版（已完成）

> **状态**: ✅ 已完成
> **发布日期**: 2026-04-29


### 功能清单

- [x] FEATURE-64 消息管理：消息上下文 Base64 编码存储和读取 [BUILD-3]
- [x] FEATURE-65 工具调用管理：工具调用结果模式改进 [BUILD-3]
- [x] FEATURE-66 Agent 功能增强：
  - 支持多轮工具调用 [BUILD-3]
  - 增加 write_to_file 的写入大小限制 [BUILD-3]
  - 增加 execute_command 的确认机制 [BUILD-3]
- [x] FEATURE-67 系统消息管理：详细的系统提示词，包含当前配置、工具定义、角色设定等 [BUILD-3]
- [x] FEATURE-68 配置管理增强：
  - 断连后自动重连 [BUILD-3]
  - 系统提示词头部信息 [BUILD-3]
  - 最大迭代次数限制 [BUILD-3]
- [x] FEATURE-69 会话管理改进：
  - 会话消息指针移动 [BUILD-3]
  - 无限迭代默认使用 [BUILD-3]
  - 历史记录切换 [BUILD-3]

## v0.4.0 — 发布候选版 RC2（已完成）

> **状态**: ✅ 已完成
> **发布日期**: 2026-05-03


### 功能清单

- [x] FEATURE-33 模型列表管理改进 [BUILD-6]
- [x] FEATURE-70 Agent 功能增强：
  - 对话上下文可视化 [BUILD-4]
  - 无限迭代循环控制 [BUILD-4]
  - 消息指针调整 [BUILD-5]
  - 大量文件写入支持 [BUILD-5]
  - 上下文清理 [BUILD-8]
  - 大小写不敏感的命令匹配 [BUILD-9]
- [x] FEATURE-71 配置管理改进：
  - 配置文件位置改进 [BUILD-4]
  - 配置热加载改进 [BUILD-4]
  - 多个配置合并 [BUILD-6]
- [x] FEATURE-72 REPL 改进：
  - ESC 取消 [BUILD-4]
  - 命令提示符 [BUILD-9]
- [x] FEATURE-73 MCP Server 增强：
  - MCP Server STDIO 模式、超时 [BUILD-4]
  - .mcp man 命令支持 [BUILD-7]
- [x] FEATURE-74 日志改进：
  - 日志级别过滤 [BUILD-4]
  - 结构化日志 [BUILD-6]
  - 错误堆栈追踪 [BUILD-6]
- [x] FEATURE-75 文件工具增强：
  - write_to_file 写入确认 [BUILD-5]
  - 文件搜索改进 [BUILD-5]
  - 文件写入大小验证 [BUILD-6]
  - 大文件写入优化 [BUILD-7]
- [x] FEATURE-76 上下文管理改进：
  - 上下文历史导入 [BUILD-5]
  - 上下文文本查看 [BUILD-5]
  - 自动上下文清理 [BUILD-8]
  - 上下文管理错误处理改进 [BUILD-8]
- [x] FEATURE-77 i18n 改进：
  - 系统提示词同步 [BUILD-6]
  - 用户消息模板 [BUILD-6]
  - 系统提示词优化 [BUILD-7]
  - 错误信息国际化 [BUILD-8]
- [x] FEATURE-78 设置向导改进：
  - 向导退出优化 [BUILD-7]
  - 供应商选择添加 [BUILD-7]
- [x] FEATURE-79 安全改进：
  - 输出安全过滤 [BUILD-8]
  - 文件路径安全验证 [BUILD-8]
- [x] FEATURE-80 循环检测优化：
  - LLM 调用循环自动检测和自修复 [BUILD-9]
  - 循环检测阈值可配置 [BUILD-9]
- [x] FEATURE-81 消息管理改进：
  - 用户消息中添加 {OS}、{SHELL}、{WORKSPACE}、{LOCALE} 模板变量 [BUILD-97]
  - 用户消息中添加 {TOOL_RESULT_MODE} 模板变量 [BUILD-97]
- [x] ENHANCEMENT-83 优化 REPL 交互体验：错误信息红色显示，调整消息裁剪阈值为对话使用量（非总量），限制消息裁剪后仍保留系统提示词 [BUILD-97]
- [x] FEATURE-84 工具调用执行流程改进：Agent 工具执行失败时，向 LLM 返回结构化错误信息，帮助 LLM 理解并修正 [BUILD-97]
- [x] FEATURE-85 新的确认模式和文件写入确认改进：[BUILD-107]
  - 实现三种确认模式：all（全部确认）、custom（选择性确认）、off（不确认）
  - 默认模式为 custom
  - 新的 ESL (Enhanced Selection List) 组件用于交互式选择
  - 文件写入确认对话框
- [x] FEATURE-86 持久的 memory 对比上下文：[BUILD-105]
  - 独立的记忆列表
  - 记忆自动过期
  - 记忆归档
  - 记忆搜索
- [x] FEATURE-87 子 Agent 改进：[BUILD-108]
  - 完整的子 Agent 工具列表
  - 可配超时时间
  - UI 提示
  - 任务编号
  - 日志
  - 支持截图
- [x] FEATURE-88 文件和系统工具的完善：[BUILD-108]
  - 文件追加工具（append_to_file）
  - 目录创建工具（create_directory）
  - 文件移动工具（move_file）
  - 文件复制工具（copy_file）
  - 文件删除工具（delete_file）
  - 文件重命名工具（rename_file）
- [x] FEATURE-89 工具调用返回信息改进：工具定义增加最佳实践说明 [BUILD-108]
- [x] FEATURE-90 子 Agent 新增替代主要 LLM 的功能：可配置子 Agent 使用的模型 [BUILD-110]

## v0.5.0 — Beta2（已完成）

> **状态**: ✅ 已完成
> **发布日期**: 2026-05-12
> **里程碑**: 功能完善，稳定可用


### 功能清单

- [x] FEATURE-91 任务计划（Task Plan / Checklist）系统 [BUILD-108]
- [x] FEATURE-82 对话上下文指系统改进：引入 messagePointer 机制，支持 LLM 在对话结束后能通过上下文指针来回移动，以在大量上下文空间中定位到上次继续的位置 [BUILD-113]
- [x] FEATURE-92 对话记忆保留机制：当 LLM 在迭代过程中自主完成任务时，在退出前的最后一次迭代中，应将 agent 迭代过程中的重要信息主动写入 memory（系统的 message 中会有类似的提示），以便能够在频繁切换对话主题时保持记忆连贯性。[BUILD-115]
- [x] FEATURE-93 工具调用历史追踪：LLM 每轮迭代的思考过程使用数字编号显示 [BUILD-116]
- [x] FEATURE-94 系统提示词 Size 优化：[BUILD-117]
  - 太长的一律放到最后，并且使用分隔符
  - 优化现有 i18n 键
- [x] FEATURE-95 会话记忆策略改进：在 Agent 每次收到 LLM 流式消息时，对工具调用结果消息设置 message_pointer，而用户消息、系统消息和助手消息不设 pointer [BUILD-118]
- [x] FEATURE-96 config.json 数据文件导出和导入 [BUILD-119]
- [x] FEATURE-97 Agent B 作为第二个智能体辅助主 Agent [BUILD-121]
- [x] FEATURE-98 ToolCall 消耗 Token 纳入 Token 统计 [BUILD-122]
- [x] FEATURE-99 新增 .settings_db 子命令管理数据库数据（key/data 增删改查）[BUILD-124]
- [x] FEATURE-100 PostgreSQL 数据库支持：支持持久化到 PostgreSQL，支持 SSL 连接、连接池（PGX 驱动），新增 .set db 配置和 .db 命令，config.json 持久化 [BUILD-125]
- [x] FEATURE-101 飞书集成 — co-shell-feishu-bridge 子项目 [BUILD-130]
  - co-shell-feishu-bridge 启动后自动运行一个 co-shell 子进程作为执行引擎，支持异步任务
  - 支持用户 `.card plan create/update/cancel` 创建/更新/取消任务计划卡片
  - 飞书消息处理集成
- [x] FEATURE-102 飞书被动响应增强：用户 `.deactive` 后 bridge 不再回复，`.active`后恢复正常 [BUILD-131]
- [x] FEATURE-103 飞书任务计划卡片自动更新：当 LLM 完成某个步骤或创建新计划时，卡片内容自动更新 [BUILD-132]
- [x] FEATURE-104 co-shell-feishu-bridge 长期记忆保持：关闭 co-shell-bridge 后恢复时保持先前记忆 [BUILD-149]
- [x] FEATURE-105 飞书用户多轮对话支持：bridge 将同一飞书用户的多次提问串联成同一对话上下文 [BUILD-149]
- [x] FEATURE-106 新工具支持：list_code_definition_names（列出代码定义名称）[BUILD-112]
- [x] FEATURE-107 新工具支持：PDF 文件读取工具 [BUILD-113]
- [x] FEATURE-108 新工具支持：access_mcp_resource [BUILD-119]
- [x] FEATURE-109 新工具支持：use_mcp_tool [BUILD-118]
- [x] FEATURE-110 增强 i18n/memory 回读在 context 中的可读性 [BUILD-119]
- [x] FEATURE-111 i18n Key 管理和翻译规范化 [BUILD-120]
- [x] FEATURE-112 Agent B 子命令增强：自动清理 Agent B 和主 Agent 历史（只在 agent_b 模块中处理 Agent B 的会话消息）；子 Agent 提示词增加 目标任务的描述（来自主 Agent 任务描述）[BUILD-123]
- [x] FEATURE-113 co-shell 设置向导支持 Tab / 数字快速选择供应商 [BUILD-127]
- [x] FEATURE-114 内置命令 Tab 补全增强：Tab 显示全部补全列表 [BUILD-128]
- [x] FEATURE-115 Agent B 快速任务/对话功能增强 [BUILD-133]
- [x] FEATURE-116 co-shell-bridge 启动效率优化：[BUILD-138]
  - 不启动 UserIO（用于 REPL）
  - 不启动 VT（不需要 LP 级别的 Shell 会话支持）
  - 不启动 REPL 输入历史
  - 不启动 Scheduler（不需要定时任务调度）
  - 不启动 Browser（不需要浏览器自动化功能）
  - 不启动自动升级检查
  - LLM 启动方式改为流式输出（但 bridge 模式下不启用增强输入）
  - 日志打印到 log/ 目录
  - 按更合理的初始化步骤启动 engine
  - 系统提示词中删除 Shell Session、Browser、Task Plan 相关的内容
- [x] FEATURE-117 子 Agent 新增 {TASK} 模板变量和指令增强：[BUILD-139]
  - 子 Agent 启动时自动从主 Agent 上下文获取完整消息列表
  - {TASK} 变量自动展开为子 Agent 格式化任务描述
  - 指令提示词统一（含子 Agent 规则、任务描述、系统提示词规则）
  - 工具列表过滤：只保留核心工具
  - 子 Agent 对话上下文格式改进
  - 子 Agent 支持截图
- [x] FEATURE-118 子 Agent 运行时自动清理消息上下文中的 system tool_result 以避免上下文膨胀：在子 Agent 运行开始和结束时，从消息历史中移除 system 角色的 tool_result 消息 [BUILD-139]
- [x] FEATURE-119 子 Agent 运行支持 TaskPlan（任务计划）：主 Agent 通过 `launch_sub_agent_with_plan` 可同时提交子 Agent 任务计划和指令，子 Agent 会自动使用 taskplan 工具管理计划内任务 [BUILD-140]
- [x] FEATURE-120 Agent 流式输出提取并替换 ${...} 变量 [BUILD-141]
- [x] FEATURE-121 Agent 改进：调整 system prompt 顺序、agent_name 作为程序名称传递给子 Agent、子 Agent 在过程中不再使用 agent_b_xxx 工具 [BUILD-141]
- [x] FEATURE-122 子 Agent 改进：限制子 Agent 的 tool_result 消息数（最多 5 个）、确保子 Agent 退出的消息写入主 Agent 记忆 [BUILD-141]
- [x] FEATURE-123 重复工具调用处理：增强消息裁剪合并逻辑，对相邻的同一工具 tool_use+tool_result 合并为一条 assistant 消息 [BUILD-142]
- [x] FEATURE-124 避免无限循环：agent 使用 LLM 工具返回后，进行工具使用结果去重处理：如果当前工具调用结果与前一次完全相同，则记录去重次数（第 N 次相同结果），并截断上下文（删除当前和前一轮迭代的助手消息和工具结果消息），以便 LLM 改变策略 [BUILD-143]
- [x] FEATURE-125 Agent 新增 `GetAgentName()` 方法 [BUILD-146]
- [x] FEATURE-126 .context compact 压缩时移除所有 tool_result 消息 [BUILD-147]
- [x] FEATURE-127 工作空间持久化配置：`.set workspace` 保存工作空间路径、`--workspace` 命令行参数 【BUILD-149】
- [x] FEATURE-128 Agent B 改进：Agent B 执行结果自动添加到主 Agent 内存中，Agent B 不包含工具调用结果的上下文信息 【BUILD-149】
- [x] FEATURE-129 .rule 命令的 [ 和 ] 键上下移动规则，r 重命名规则，w 切换启用/禁用，d 删除 【BUILD-150】
- [x] FEATURE-130 .rule 命令改进：列表左侧显示规则编号，编号按最长数字右对齐，启用/禁用规则后切换到同一页 【BUILD-151】
- [x] FEATURE-131 Taskplan 创建后消息指针自动调整到新计划之后，以便 LLM 能"看到"刚创建的任务计划 【BUILD-152】
- [x] FEATURE-132 `.plan` 命令交互式管理任务计划：支持通过上下键选择步骤、Enter 切换状态、+/- 调整位置等 【BUILD-152】
- [x] FEATURE-133 `.plan` 子命令增强：`.plan create` 交互创建 [BUILD-153]；`.plan remove` 交互式选择要删除的步骤 [BUILD-154]；`.plan insert` 交互式插入步骤 [BUILD-154]
- [x] FEATURE-134 `.plan` 步骤显示编号和控制键改进：步骤按使用顺序排列（步骤编码和子步骤），新增 v 查看已完成步骤，n 显示步骤备注，c 切换步骤完成状态，快捷键摘要显示在列表底部 【BUILD-155】
- [x] FEATURE-135 启动时 co-shell 自动根据当前工作空间通过 `.plan` 命令检查是否有未完成的任务计划 【BUILD-156】
- [x] FEATURE-136 `.plan` 编辑模式增强：新增 e 键编辑模式 —— 在任何步骤上按 e 进入编辑模式，按 a 添加步骤、d 删除、i 插入、e 编辑描述、m 编辑备注、←/→ 或 Ctrl+←/Ctrl+→ 移动步骤，编辑完成后按 ESC 预览并确认、Enter 直接保存 【BUILD-157】
- [x] FEATURE-137 任务计划移除步骤支持范围删除：`.plan remove` 参数支持单个、空格分隔的多个、横线范围、混合格式，如 `1 3-5 7`（删除 1,3,4,5,7）【BUILD-158】
- [x] FEATURE-138 `.plan` 增加 steps 子命令列出详细步骤 【BUILD-159】
- [x] FEATURE-139 启动时如果有未完成的任务计划，co-shell 自动将计划详情发送给 LLM 作为上下文，并提示用户是否继续 【BUILD-160】
- [x] FEATURE-140 Agent 系统提示词 TaskPlan 节增强：增加创建新计划时说明、示例和规则——1) Plan-Act-Reflect 方法论、2) 渐进式计划=简单开始+逐步调整、3) 步骤变动时更新计划  【BUILD-161】
- [x] FEATURE-141 `.plan` 交互界面改进：增加 `m` 键调整步骤顺序，选中步骤后移动到指定位置（手动输入目标位置） 【BUILD-162】
- [x] FEATURE-142 `.plan` 编辑模式增加 `v` 键还原被删除步骤：在 e 编辑模式下，按 v 显示最近一次删除的步骤列表，勾选需要还原的步骤后恢复到原位 【BUILD-163】
- [x] FEATURE-143 `.plan` 编辑模式增加 `i` / `a` 插入步骤时选择插入到选中步骤前/后：在 e 编辑模式下选中某步按 i 后弹窗让选择插入位置（之前/之后） 【BUILD-163】
- [x] FEATURE-144 `.plan` 列表显示所有子命令 【BUILD-163】
- [x] FEATURE-145 `.plan` e 编辑模式增加 `t` 键修改步骤状态：当前状态高亮显示，上下键选择，Enter 确定 【BUILD-164】
- [x] FEATURE-146 Browser Tool 改进：browser_screenshot 参数形式配置、`.browser` 命令改进 【BUILD-165】
- [x] FEATURE-147 子 Agent 执行时长信息反馈：【BUILD-166】
  - Agent 状态显示阶段增加秒数显示（思考 / 使用工具 / 工具已完成）
  - Agent B 执行完成后返回耗时信息
  - 子 Agent 执行完成后返回耗时信息
- [x] FEATURE-148 任务计划调整触发记忆归档：【BUILD-167】
  - 当使用 remove_steps 或 insert_steps 调整任务计划时，如果最终原计划步骤全部被清空，应归档旧计划（作为已取消）并删除空计划，允许 LLM 创建全新计划
  - 当使用 update_step_status 更新所有步骤状态为 completed 时，也应归档当前计划作为已完成，删除空计划，允许创建新计划
  - 归档的记忆内容包含当前进展（进度百分比）
- [x] FEATURE-149 .plan remove 命令增强：如果在交互式删除中删除了所有步骤，整个计划归档到记忆并删除，并提示用户可以建新计划了 【BUILD-168】
- [x] FEATURE-150 新工具：use_subagents（启动多个子 Agent 并行执行）【BUILD-168】
- [x] FEATURE-151 .rule 命令新增 rename 子命令 【BUILD-169】
- [x] FEATURE-152 工具定义名称统一：所有工具保留原始下划线名称（如 execute_command），移除描述中的 `（execute_command）` 后缀和冒号前缀，工具定义聚焦 param 而不是 description 【BUILD-170】
- [x] FEATURE-153 支持的供应商列表中加入 qwen-turbo-2026-01-19 【BUILD-170】
- [x] FEATURE-154 关闭 shell 会话时自动清理 VT 终端窗口内容 【BUILD-171】
- [x] FEATURE-155 消息去重改进：sendConversationUpdate 独立为方法、延迟调用（500ms 防抖）、消息对象含 time+text+isLoading 三字段、isLoading 变化时立即更新、仅 isLoaded 状态才加入 history 【BUILD-171】
- [x] FEATURE-156 任务计划 Step 备注显示：在 `.plan` 列表和 `remove` / `insert` / `view` 命令输出中显示步骤备注 【BUILD-172】
- [x] FEATURE-157 Agent 检查配置文件错误并提示：【BUILD-172】
  - 启动时检查 config.json 中是否包含无法识别的 key，如果有则打印警告并列出
- [x] FEATURE-158 工作空间改进：启动时检查 config.json 中的 workspace 是否变更，如变更则清理旧工作空间的记忆列表并初始化新 workspace 的 DB；`work/` 目录在 workspace 下创建 【BUILD-173】
- [x] FEATURE-159 REPL 增强：支持多行输入（`\` 行续接 + `{ }` 自动续接），LLM 调用按 ESC 后自动暂停并显示暂停后的输出，按任意键继续或 ESC 再次退出 【BUILD-173】
- [x] FEATURE-160 上下文压缩改进（compileContext）：在达到最大 token 数量时，优先压缩 tool_result 工具调用结果消息【BUILD-173】
- [x] FEATURE-161 多行输入时行号提示改变：第一行提示符显示 > ，续行提示符显示 >> 【BUILD-174】
- [x] FEATURE-162 命令执行结果中出现的图片 URL 自动下载到本地：【BUILD-174】
  - 提取命令输出中的图片 URL（如 ![alt](url)、<img src="url"> 或纯 URL）
  - 下载为 PNG/JPEG/GIF/WebP 格式到 workspace/{image-dir} 目录
  - 在命令输出中替换 URL 为本地路径
- [x] FEATURE-163 命令 `execute_command` 输出中的附带文件收集：【BUILD-174】
  - 当执行 shell 命令后，如果输出中包含 `📄` 文件标记，则将对应的文件内容一并返给 LLM
- [x] FEATURE-164 Chrome 直接下载图片 【BUILD-175】
- [x] FEATURE-165 代码搜索工具 search_files 增加 file_pattern 参数，支持限定文件扩展名或文件名模式 【BUILD-175】
- [x] FEATURE-166 上下文管理优化：system prompt 中的 ROADMAP 只保留当前版本的任务列表，已完成和未开始的版本只保留标题，不再展开任务项 【BUILD-176】
- [x] FEATURE-167 工具调用中文件写入类工具（write_to_file / replace_in_file）在返回结果时显示文件大小（bytes），并在系统提示词中指导 LLM 注意监控文件大小，防止产生过大文件 【BUILD-176】
- [x] FEATURE-168 新工具：image_tools（图片分析工具），用于 LLM 分析图片文件，支持 URL 和本地路径，返回 Base64 编码图片数据 【BUILD-176】
- [x] FEATURE-169 新工具：rename_file 工具，用于重命名文件 【BUILD-176】
- [x] FEATURE-170 子 Agent 改进：从主 Agent 继承工具调用模式、confirm-tool 设置等配置参数 【BUILD-176】
- [x] FEATURE-171 任务计划：支持 LLM 在步骤中直接完成（completed）状态标记，并自动归档已完成任务计划到记忆 【BUILD-176】
- [x] FEATURE-172 tool_use 列表去重，仅保留最近一条：当 LLM 连续两次返回完全相同的 tool_use（函数名称和参数完全相同）时，仅保留最近一次调用，删掉前一次 【BUILD-177】
- [x] ENHANCEMENT-173 RUN 流式输出工具调用和结果显示改进：【BUILD-178】
  - LLM 返回 tool_use 时即显示 `[🔧] 工具名称(参数...)` 并立即执行
  - 工具调用执行完成显示 `[🔧] 工具调用结果` + 结果摘要
  - 工具执行过程中无 LLM 思考步骤也无额外空行
  - 所有输出归流式输出的 callback（非工具函数内 print）
  - 首次调用前和末次调用后各输出一个分隔线
- [x] ENHANCEMENT-174 工具调用确认序号改进：序号改为每次任务会话独立，agent 初始化时重置为 0，工具调用确认对话中的序号现在持续增长且不会再滚动回 0 【BUILD-178】
- [x] ENHANCEMENT-175 循环检测优化：消息历史 tokens 整数溢出改为 uint64 防止溢出；新增 content-level 循环检测（checkContentLoop）【BUILD-178】
- [x] ENHANCEMENT-176 系统提示词优化：移除重复的 "Objective" 节，移除 PromptSection 和 WorkMode 内置节中的重复节 【BUILD-178】
- [x] ENHANCEMENT-177 工具调用确认序号显示优化：序号改为：实例上所有 agent 共享的全局序号的最后一个数字。【BUILD-178】
- [x] ENHANCEMENT-178 修改对话上下文时间戳前缀格式：从 "2026-05-12 10:15:30 - " 改为 "在 2026-05-12 10:15:30 说："，提升用户可读性。[BUILD-187]
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
- [x] FIX-191 修复 i18n 中 <task> 闭合标签错误，修复 {TASK} 只含纯指令而非格式化消息的问题（agent.go/run_stream.go/run.go/loop.go + i18n）

## v0.6.0 — Beta3

> **状态**: 🚧 开发中
> **目标日期**: 2026-06-01
> **里程碑**: 功能完善，稳定可用

### 功能清单

- [x] FEATURE-208 外部工具（bin/）梳理与优化：[BUILD-213]
  - [x] 新增 pdf2png 工具：将 PDF 拆分为分页 PNG 图片，支持 LLM 多模态 PDF 内容解析
  - [x] 新增 docx2pdf 工具：将 .docx 转换为 PDF，WPS 优先（Linux wps2pdf / macOS AppleScript / LibreOffice 兜底）
  - [x] 新增 doc2pdf 工具：将老式 .doc 转换为 PDF，WPS 优先，LibreOffice 兜底
  - [x] 新增 wps2pdf 工具：将 .wps（WPS Office Writer）转换为 PDF
  - [x] 为每个外部工具创建同名 .md 参数说明文件
  - [x] 系统提示词中新增 ExternalTools 节（中英文），指导 LLM 调用 bin/ 工具
- [x] FEATURE-192 持续交互 Shell 环境：新增 shell/session.go 包，使用 PTY 维护长期运行的 shell 进程，支持命令发送和输出捕获。提供 shell_start / shell_exec / shell_stop / shell_get_output 四个 LLM 工具，让 LLM 在同一个 shell 进程中连续执行命令（如 cd 保持路径、Python REPL 等），支持超时控制。新增 shell-session-enabled / shell-session-timeout 配置项。[BUILD-196]
- [x] ENHANCEMENT-193 Shell Session 机制优化：shell_send 替代 shell_exec，纯 idle timeout 观察（不再追加 \n 和无 marker），新增 unescapeCommand() 支持控制字符（\n、\x03 等），stripLogANSI 日志和 LLM 输出控制字符剥离，0 工具调用时需 attempt_completion 才退出，XML 解析已知工具优先检查避免 HTML 标签误报，parseXMLChildrenToJSON 不再 trim 保留全部空格，defaultToolModes() 默认工具确认模式配置，.set confirm-tool reset 恢复出厂设置。[BUILD-197]
- [x] FEATURE-194 虚拟终端（Virtual Terminal）功能：为 Shell Session 增加纯 Go 实现的虚拟终端，支持 ANSI 控制序列解析和字符网格渲染，让 LLM 能像人一样查看终端画面。[BUILD-198]
  - 新增 `shell/vt.go`：虚拟终端核心实现，无外部依赖，支持光标移动（CUP/HVP/CHA/CUU/CUD/CUF/CUB）、清屏清行（ED/EL）、字符输入（CR/LF/BS/TAB）、屏幕滚动、SGR 解析
  - 新增 VT 窗口尺寸配置：`shell-vt-rows`（默认 24）、`shell-vt-cols`（默认 80），支持通过 `.set` REPL、命令行参数、配置文件设置
  - `shell_send` 改为返回完整的 VT 窗口文本（rows 行），而非增量文本
  - 新增 `shell_window_content` 工具，返回 VT 窗口当前内容
  - 完善 shell session 相关方法的使用说明和用例
  - `shell-session-enabled` 为 on 时自动屏蔽 `execute_command`；为 off 时自动屏蔽所有 shell session 工具
- [x] FEATURE-195 系统提示词外部化：所有系统提示词节（Identity/ToolUsage/ResultMode/Capabilities/Rules/Environment/Objective 及 XML 模式的 Examples/TaskProgress/EditingFiles）均支持通过外部 Markdown 文件覆盖，文件放在 workspace 根目录，启动时优先读取外部文件，不存在时回退到 i18n 内置资源。[BUILD-199]
- [x] FEATURE-196 工作模式配置系统：
  - 新增 PromptSection 和 WorkMode 数据结构，config.json 持久化
  - 新增 `.section` 命令管理自定义节（add/list/remove）
  - 新增 `.mode` 命令管理工作模式（list/create/edit/switch）
  - 新增 `--mode` 命令行参数和 `.set mode` REPL 命令切换
  - 内置默认工作模式，含所有内置节的默认拼接顺序
  - 修改 `buildSystemPromptWithMode` 按当前工作模式拼装
  - 交互式编辑：+/-上下移动、a/d添加删除、v查看内容、p预览完整提示词 [BUILD-199]
- [x] FEATURE-197 REPL 配置向导模式：新增 .config 内置命令，提供逐级菜单式配置向导，第一层按 .set 命令的配置分类显示，P 返回上一步，Q 完全退出 [BUILD-200]
- [x] FEATURE-198 shell-session-enabled=on 时 REPL 直接命令通过 VT 会话执行：当 shell-session-enabled 为开时，用户输入的 shell 命令（非以 . 开头的输入内容）直接发送到 VT 去执行，并返回执行结果（使用 shell_get_output，获得完整的输出）[BUILD-202]
- [x] FIX-199 修复 .config 设置 shell-session-enabled=on 后未自动初始化 VT session 及 VT 中文显示问题：shell-session-enabled SetValue 缺少 agent.SetShellEnabled() 同步；VT 逐字节处理 UTF-8 导致中文文件名乱码；VT \n 空 lineBuf 时 flushLine 导致 ls 多空行 [BUILD-203]
- [x] FIX-204 修复 execute_command / ExecuteCommandDirectly 执行交互式命令（如 sudo）时 stdin 未连接导致用户无法输入的问题 [BUILD-209]
- [x] FEATURE-205 UserIO 规范合规整改：检查全代码库中所有直接使用 fmt.Print/fmt.Printf/fmt.Println/os.Stdin/bufio.Scanner 进行用户交互的地方，统一替换为 agent.UserIO 接口方法。整改完成：agent/io.go 新增 DefaultUserIO + GetIO + DefaultIO + ErrPrintf；cmd/config.go、cmd/mode.go、cmd/model.go、cmd/settings_db.go、main.go 全部改用 io 方法；删除 cmd/model.go 废弃的 bufio.Scanner 字段。同时修正 i18n/zh_system.go 中 BROWSER USAGE 示例错误（browser_xxx 是工具调用而非系统命令）。[BUILD-210]
- [x] FIX-206 补全浏览器工具 XML 模式逐方法调用说明：在 i18n/keys.go 中新增 11 个浏览器工具键，在 i18n/zh_system.go 和 i18n/en_system.go 中为每个浏览器工具添加完整的 Description/Parameters/Usage XML 说明，在 agent/toolcall_mode.go 的 toolUsageKeyMap 中建立 browser_xxx 到 i18n 键的映射，确保 XML 模式下 LLM 能获取到每个浏览器工具的完整调用格式。[BUILD-211]
- [x] FEATURE-200 CDP 浏览器支持：新增 browser/ 包，通过 Chrome DevTools Protocol (CDP) 直接启动并控制 Chrome 浏览器，提供一组浏览器操作 LLM 工具（browser_navigate、browser_screenshot、browser_click、browser_type、browser_evaluate、browser_get_html、browser_scroll、browser_get_interactive_elements、browser_go_back、browser_go_forward、browser_close），配合截图视觉分析实现 "观察→评估→操作" 的 SREA 闭环。新增 browser-enabled / browser-port / browser-headless 配置项，截图自动注入到多模态上下文供 VLM 分析。[BUILD-211]
- [x] FEATURE-223 browser_get_html 更名为 browser_get_rendered_html：明确名称和文档，强调该工具返回的是经过所有 JS 渲染后的 DOM HTML（来自 Chrome 实时 DOM 树），而非原始静态源码，LLM 无需再单独下载 JS/JSON 等资源。同步更新所有 i18n 系统提示词中的工具描述、SREA 步骤和工具表格。HTML 无论大小始终保存到本地文件以确保数据完整性。新增页面数据收集方式对比章节（截图/交互元素/渲染后 DOM HTML 三种方式优劣分析）。[BUILD-227]
- [x] FIX-224 修复 .simulate 缺少父标签闭合时错误提示混淆：XML 解析器 findAnyCloseTag 回退逻辑会盲目使用不匹配的子标签闭合标签来闭合父标签，导致 LLM 看到"参数缺少闭合标签"的错误提示而非"父标签缺少闭合标签"。修复为检测名称不匹配时直接报清晰错误，错误消息末尾附带正确的方法调用格式示例。[BUILD-228]
- [x] FIX-225 修复当前会话历史记录未出现在上下键导航中的问题：REPL 的 `saveHistory()` 只将输入持久化到数据库，但从未更新内存中的 `r.history` 切片，导致每次 `readLine()` 创建新 `EnhancedInput` 时传入的都是启动时加载的旧历史。修复为在数据库写入后同步更新 `r.history` 和 `r.historyPos`。[BUILD-229]
- [x] FIX-226 修复上下键导航时残留旧行字符的问题：`clearLine()` 使用 `\033[J`（从光标清除到屏幕末尾）只能清除光标之后的字符，当从长命令切换到短命令时，旧行尾部字符残留在光标之前。修复为 `\r\033[2K`（先回车到行首，再擦除整行），确保整行被完全清除。[BUILD-230]
- [x] FEATURE-207 系统提示词规则增强：新增五条系统提示词规则——1) 获取 Web 页面优先使用浏览器工具（ToolUsage 节）；2) curl/wget 下载先保存到本地再用 read_file（ToolUsage + RULES 节）；3) 阶段性任务完成时推荐移动上下文指针（RULES 节）；4) 修改程序优先 replace_in_file 分多次而非重写整个文件（RULES 节）；5) 研究报告用 Markdown 整理后转 Word 并呈现给用户（RULES 节）。五条规则均与现有规则互补不冲突。[BUILD-212]
- [x] FEATURE-202 表达式计算器工具：新增 `evaluate_expression` LLM 工具，提供表达式计算能力。使用递归下降解析器实现，支持四则运算（+、-、*、/、%）、指数运算（^）、三角函数（sin、cos、tan、asin、acos、atan）、对数（log、ln）、开方（sqrt）、绝对值（abs）、取整（ceil、floor、round）以及常数 pi、e。工具接收表达式字符串，解析计算后返回精确数值结果，让 LLM 在进行数学计算时无需依赖 Python 或外部命令。[BUILD-207]
- [x] FEATURE-201 ESC 中断 LLM 输出功能：用户在增强输入模式下按 ESC 键可中断 LLM 流式输出。系统先暂停接收 LLM 返回数据，提示用户确认取消或继续；若确认取消则丢弃不完整消息并返回命令提示符；若选择继续则重新尝试接收 LLM 返回数据，失败时同取消处理。[BUILD-206]
- [ ] FEATURE-93 日历与待办事项管理：提供日历功能，支持记录和管理待办事项（todo）。提供 .calendar 内置命令（add/list/remove/update）管理待办事项；提供 add_todo / list_todos / update_todo / remove_todo 四个 LLM 工具，让大模型能操作待办事项；数据持久化到 bbolt。如果系统有日历应用（如 macOS 日历），提供选项帮助用户将待办事项同步到系统日历。
- [x] FEATURE-203 .config 配置向导增强：1) 补齐 browser-enabled/browser-port/browser-headless 参数到"智能体设置"分组；2) 快捷键改进：B 退回上一步、Q 完全退出，每一步都显示快捷键提示；3) 显示格式改进：所有选项先显示说明再显示当前值，.Xxxx 命令显示"name..."表示进入子配置；4) 选择编号按最长数字右对齐；5) 去掉发送LLM上下文时的序号前缀；6) 运行失败的命令也加入history。[BUILD-208]
- [x] FIX-212 修复浏览器 CDP 功能异常：[BUILD-218]
  - CDP 调用超时保护（ensureTimeoutContext，默认30s超时兜底），防止 context.Background() 无 deadline 导致永久阻塞
  - WaitForPageLoad 轮询 document.readyState=="complete"，解决 Page.navigate 异步返回后页面未加载完成的问题
  - 过滤 Chrome 扩展背景页面（chrome-extension://），优先 type=="page" 的真实标签页；全为扩展时自动新建标签页，解决导航到扩展页而非目标 URL 的问题
  - browserCloseTool 不设 browserEnabled=false，解决第二次调用浏览器工具时 "tool not found" 的问题
  - browserCloseTool 不杀 Chrome 进程，仅断开 WebSocket；下一个工具调用时 EnsurePageConnected 自动创建新标签页重连，不会产生多余的空白窗口
  - 重启时检测并复用已有 Chrome 实例（IsEndpointAvailable+SetStarted），避免重复启动浏览器
  - 浏览器数据目录持久化到 {workspace}/browser-data/，替代 /tmp 临时目录，浏览器状态（Cookie/Session/下载）跨重启保留，可追溯
  - 自闭合标签（<tag />）解析为工具调用：checkSelfClosing 支持 <tag/> 和 <tag /> 两种格式
  - HTML 内容导致 parseXMLChildrenToJSON 递归解析失败时回退纯文本，解决 write_to_file 含 HTML/CSS 内容时解析退出
  - createNewPage 增加 HTTP 状态码检查，返回非 JSON 响应时提供完整错误体诊断
- [x] FIX-209 修复 ESC monitor 与子进程争夺 stdin（sudo 密码输入被拦截）、streamCallback 换行（\r→\n）、命令输出重复、.config 参数不立即生效等问题：Agent 新增 commandRunning 标志 + SetCommandRunning/IsCommandRunning 方法；rawOutputWriter 实时输出 \n→\r\n 转换 + [🔴]> 前缀；syncedOnOffParam 辅助函数使 .config 设置即时同步到 agent；confirm-tool 默认改为 custom 模式。[BUILD-215]
- [x] FIX-210 工作空间默认路径智能检测：双击启动时自动使用可执行文件所在目录作为默认工作空间，终端启动时使用当前工作目录。新增 workspace/detect_common.go / detect_darwin.go / detect_linux.go / detect_windows.go 实现跨平台启动方式检测，main() 在 workspace 初始化后自动 os.Chdir 到工作空间根目录。
- [x] FIX-211 修复 .set description 无法保存生效的问题：i18n SystemPromptIdentity 节中增加 {AGENT_DESCRIPTION} 占位符，新增 KeyAgentDefaultDescription 默认描述键，Agent 构建系统提示词时从 cfg.LLM.AgentDescription 读取并替换占位符。
- [x] FEATURE-215 数据库双写策略重构（memory+history 同时写入 PG 和本地 bbolt，其他数据仅本地 bbolt）：[BUILD-217]
  - [x] 创建 DualStore 包装器，memory 和 history 操作同时写入 bbolt 和 PG，其他操作仅写入 bbolt
  - [x] PGStore 简化为仅 memory 和 history 两张表，移除 context/schedules/taskplans/token_usage/sessions 表
  - [x] 启动时如 PG 可用，自动增量迁移 bbolt 中未同步的 memory 和 history 数据
  - [x] .db 命令显示当前数据库连接状态
  - [x] .db init/migrate/backup/restore 子命令仅处理 memory+history 表
- [x] FIX-216 HasUnfinished 检查不再将 cancelled 视为未完成步骤：\`HasUnfinished()\` 中 `StatusCancelled` 与 `StatusCompleted` 一样被视为"已完成"，允许在所有步骤均为已完成或已取消时创建新计划。[BUILD-220]
- [x] FEATURE-218 模拟 LLM 方法调用命令 `.simulate`：新增 `.simulate` 内置命令，接收模拟的 LLM 返回内容（XML 或 JSON 格式），使用与正常 LLM 调用完全一致的管道（ParseXMLToolCalls / executeToolCall）进行解析和执行测试。结果不加入对话上下文，仅用于调试和测试。同步加入 `.config` 开发者工具分组。[BUILD-221]
- [x] FIX-219 XML 模式 attempt_completion 退出逻辑修复：\`attemptCompAvailable\` 在 XML 模式下因 \`buildTools()\` 返回空列表而被误判为 false，导致 0 toolCall 时直接退出而非要求 LLM 调用 \`attempt_completion\`。修复为改用 \`buildToolsInternal()\` 判断。同步增强 continuePrompt 文本，更明确要求 LLM 必须调用 attempt_completion 并深思熟虑。[BUILD-222]
- [x] FEATURE-220 PostgreSQL 数据库连接超时配置：DBConfig 新增 Timeout 字段（默认 3 秒），DSN 追加 connect_timeout 参数，net.DialTimeout TCP 预检 + goroutine Ping 超时控制。支持 `.set db timeout <秒>` 子命令、`.db` 显示超时值、`.db status` 重新检测连接。所有 db.Close 用 safeCloseDB 超时保护。防止数据库不可达时程序长时间挂起。新增 i18n KeyDBTimeoutLabel/KeyDBStatusCmd 中英文翻译。[BUILD-223]
- [x] FEATURE-221 消息序号 `{MESSAGE_NO}` 注入：在每条用户消息和 XML 工具结果消息的 `<environment_details>` 中注入消息在 `a.messages` 数组中的索引序号，LLM 可直接将该序号作为 `adjust_context_start` 的 `target_index` 参数使用。新增 `formatUserMessage`/`formatXMLToolResult` 的 `messageNo` 参数。中英文 i18n 模板同步更新，OpenAI 模式工具描述和 Rules 节补充使用时说明。新增 RULES 节「指令不明确时搜索记忆」规则。[BUILD-224]
- [x] FIX-222 .model add 新增模型时默认使用最高优先级：`wizardEnterModelParams()` 中优先级默认值从 `template.Priority` 改为 `(len(h.cfg.Models) + 1) * 10`，确保新增模型默认排到最高优先级。[BUILD-225]
- [ ] FEATURE-94 命令执行审计功能：在执行 execute_command 工具调用时，先将命令发送给 LLM 进行安全风险分析，LLM 判断命令是否存在风险（如删除文件、修改系统配置、网络操作等）。如果存在风险，提示用户确认后才能执行。支持通过 .set audit-enabled 配置、--audit-enabled/--audit-disabled 命令行参数、config.json 控制审计功能的开启/关闭。
- [x] FEATURE-106 实现history命令翻页：支持通过上下键浏览、.history last/first 命令查看、编号重新执行历史命令，数据持久化到 bbolt [BUILD-68]
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
|---|---|---|---|---|---|
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
| FEATURE- | 新特性或改进（New Feature/Enhancement） |
| FIX- | Bug 修复（Bug Fix） |
# FEATURE-481 测试用例

## UC-0001: agent 未注入运行时信息时 environment_details 不含 runtime_info

### 测试目标
验证当 agent 未调用 SetRuntimeInfo（默认状态）时，`<environment_details>` 不输出 `<runtime_info>` 块，保持向后兼容。

### 前置条件
- 构造一个未注入运行时信息的 Agent

### 测试步骤
1. 构造 Agent（不调用 SetRuntimeInfo）
2. 调用 buildFullEnvironmentDetails 生成 environment_details
3. 检查输出是否包含 `<runtime_info>`

### 预期结果
- 输出不包含 `<runtime_info>` 块
- 原有字段（time/message_no/cwd 等）正常输出

### 验证方法
- 单元测试通过（agent 包内测试）

## UC-0002: 注入 stdio 模式运行时信息后 environment_details 输出 runtime_info

### 测试目标
验证注入 stdio 服务模式的运行时信息后，`<environment_details>` 正确输出进程号、版本号、build号、服务模式。

### 前置条件
- 构造 Agent
- 调用 SetRuntimeInfo 注入 stdio 模式信息（pid/version/build/serviceMode=stdio）

### 测试步骤
1. 调用 SetRuntimeInfo 注入 stdio 模式运行时信息
2. 调用 buildFullEnvironmentDetails 生成 environment_details
3. 检查输出

### 预期结果
- 输出包含 `<runtime_info>` 块
- 包含 `<pid>`、`<version>`、`<build>`、`<service_mode>stdio</service_mode>`
- stdio 模式不输出端口/白名单/bind（非 serve 模式）

### 验证方法
- 单元测试通过

## UC-0003: 注入 enhanced 模式运行时信息后输出 service_mode=enhanced

### 测试目标
验证 enhanced（REPL 交互）服务模式正确输出。

### 前置条件
- 构造 Agent
- SetRuntimeInfo 注入 serviceMode=enhanced

### 测试步骤
1. 注入 enhanced 模式运行时信息
2. 生成 environment_details
3. 检查输出

### 预期结果
- `<service_mode>enhanced</service_mode>` 正确输出
- 不输出端口/白名单/bind

### 验证方法
- 单元测试通过

## UC-0004: 注入 serve 模式运行时信息后输出端口/白名单/bind

### 测试目标
验证 serve 服务模式额外输出端口号、白名单、bind 地址。

### 前置条件
- 构造 Agent
- SetRuntimeInfo 注入 serviceMode=serve，含 port=8399、bind=127.0.0.1、whitelist

### 测试步骤
1. 注入 serve 模式运行时信息（含端口/白名单/bind）
2. 生成 environment_details
3. 检查输出

### 预期结果
- `<service_mode>serve</service_mode>` 正确输出
- 输出 `<serve_port>8399</serve_port>`、`<serve_bind>127.0.0.1</serve_bind>`
- 有白名单时输出 `<serve_whitelist>...</serve_whitelist>`

### 验证方法
- 单元测试通过

## UC-0005: serve 模式无白名单时不输出 serve_whitelist

### 测试目标
验证 serve 模式白名单为空（无白名单）时不输出 serve_whitelist 字段。

### 前置条件
- 构造 Agent
- SetRuntimeInfo 注入 serviceMode=serve，whitelist 为空

### 测试步骤
1. 注入 serve 模式运行时信息（whitelist 为空）
2. 生成 environment_details
3. 检查输出

### 预期结果
- 输出 `<service_mode>serve</service_mode>`、端口、bind
- 不输出 `<serve_whitelist>`（白名单为空）

### 验证方法
- 单元测试通过

## UC-0006: main.go 启动时注入运行时信息（stdio 单命令模式）

### 测试目标
验证 co-shell 以单命令（stdio）模式启动时，agent 被注入正确的运行时信息（pid/version/build/serviceMode=stdio）。

### 前置条件
- 编译后的 co-shell 可执行文件
- 配置好 API key

### 测试步骤
1. 以单命令模式运行：`co-shell "简单指令"`
2. 观察 agent 收到的 environment_details 是否含 runtime_info

### 预期结果
- environment_details 含 `<runtime_info>`，service_mode=stdio
- pid 为当前进程号，version=0.36.0，build 为当前 build

### 验证方法
- 运行单命令，检查日志/输出中的 environment_details

## UC-0007: main.go 启动时注入运行时信息（serve 模式）

### 测试目标
验证 co-shell 以 serve 模式启动时，agent 被注入 serve 模式的端口/白名单/bind。

### 前置条件
- 编译后的 co-shell 可执行文件
- 配置好 API key

### 测试步骤
1. 以 serve 模式运行：`co-shell --serve --port 8399`
2. 在 Web UI 中发起一个任务
3. 观察 agent 收到的 environment_details

### 预期结果
- environment_details 含 `<runtime_info>`，service_mode=serve
- 输出 serve_port=8399、serve_bind=127.0.0.1
- 无白名单时不输出 serve_whitelist

### 验证方法
- 运行 serve 模式，在 Web UI 发起任务，检查 environment_details
